-- library.sql — kişisel kitaplık sorguları (Faz 15).

-- Ekle ya da durumu değiştir, TEK deyimde. Kitap yayında değilse SELECT satır üretmez,
-- hiçbir şey yazılmaz ve RETURNING boş döner → 404 (kitabın varlığı da sızmaz).
-- Arşivlenmiş bir kitabın kaydı bu yüzden güncellenemez; yalnız silinebilir.
-- name: UpsertLibraryEntry :one
INSERT INTO library_entries (user_id, book_id, status)
SELECT sqlc.arg('user_id')::bigint, b.id, sqlc.arg('status')::text
FROM books b
WHERE b.id = sqlc.arg('book_id') AND b.status = 'APPROVED'
ON CONFLICT (user_id, book_id)
DO UPDATE SET status = EXCLUDED.status, updated_at = now()
RETURNING *;

-- Silmek idempotent: kayıt yoksa da hata değil.
-- name: DeleteLibraryEntry :exec
DELETE FROM library_entries WHERE user_id = $1 AND book_id = $2;

-- Kartlardaki rozetler için TOPLU okuma: sayfadaki kitap kimlikleri tek sorguda (N+1 yok).
-- name: ListLibraryStatuses :many
SELECT book_id, status FROM library_entries
WHERE user_id = sqlc.arg('user_id') AND book_id = ANY(sqlc.arg('book_ids')::bigint[]);
