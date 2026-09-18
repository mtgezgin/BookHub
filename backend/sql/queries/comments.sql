-- comments.sql — yorum sorguları (Faz 5). Okumalar visible_comments VIEW'undan.

-- Yalnızca ONAYLI kitaba yorum: kontrol ve ekleme TEK deyimde (INSERT ... SELECT ... WHERE
-- EXISTS). Kitap onaylı değilse hiç satır eklenmez → ErrNoRows → 404.
-- name: CreateComment :one
INSERT INTO comments (book_id, user_id, content)
SELECT sqlc.arg('book_id'), sqlc.arg('user_id'), sqlc.arg('content')
WHERE EXISTS (SELECT 1 FROM books WHERE id = sqlc.arg('book_id') AND status = 'APPROVED')
RETURNING *;

-- Yazarın görünen adıyla birlikte (LEFT JOIN: yazarı silinmiş yorum da gelsin, adı NULL).
-- name: ListComments :many
SELECT c.id, c.book_id, c.user_id, u.display_name AS author_name, c.content, c.created_at
FROM visible_comments c
LEFT JOIN users u ON u.id = c.user_id
WHERE c.book_id = sqlc.arg('book_id')
ORDER BY c.created_at DESC, c.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountComments :one
SELECT count(*) FROM visible_comments WHERE book_id = $1;

-- Yumuşak silme; zaten silinmişse satır dönmez (idempotent değil, 404: "silinecek görünür
-- yorum yok").
-- name: SoftDeleteComment :one
UPDATE comments
SET deleted_at = now(), deleted_by = sqlc.arg('deleted_by'), updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, book_id;
