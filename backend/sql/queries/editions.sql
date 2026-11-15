-- editions.sql — kitap baskıları (Faz 16, #36).
--
-- Bir kitabın (eserin) birden çok baskısı olur; tam olarak biri varsayılandır (is_primary).
-- "En çok bir varsayılan" kuralını kısmi benzersiz index (editions_one_primary_idx) garanti eder;
-- "en az bir" kuralını servis uygular (varsayılan silinemez, ilk baskı varsayılan olur).

-- Detay sayfasındaki sıra: varsayılan önce, sonra dil, sonra yıl (bilinmeyen sonda), sonra id.
-- name: ListEditionsByBook :many
SELECT * FROM editions
WHERE book_id = $1
ORDER BY is_primary DESC, language, publication_year NULLS LAST, id;

-- name: GetEdition :one
SELECT * FROM editions WHERE id = $1;

-- Kitap ekleme ve öneriyle aynı transaction'da: yeni kitabın varsayılan baskısı.
-- name: CreatePrimaryEdition :exec
INSERT INTO editions (book_id, title, language, cover_url, page_count, is_primary)
VALUES (sqlc.arg('book_id'), sqlc.arg('title'), sqlc.arg('language'), sqlc.arg('cover_url'),
        sqlc.narg('page_count'), true);

-- Kitap PATCH'indeki kapak / sayfa sayısı varsayılan baskıya yazılır. Kitabın varsayılan baskısı
-- yoksa (genişletme yayın aralığında eski kodla eklenmiş kitap) kitabın adı ve diliyle açılır.
-- ON CONFLICT hedefi kısmi index'in kendisi: (book_id) WHERE is_primary.
-- name: PatchPrimaryEdition :exec
INSERT INTO editions (book_id, title, language, cover_url, page_count, is_primary)
SELECT b.id, b.title, b.language, COALESCE(sqlc.narg('cover_url'), '') AS cover_url,
       sqlc.narg('page_count')::integer AS page_count, true AS is_primary
FROM books b
WHERE b.id = sqlc.arg('book_id')
ON CONFLICT (book_id) WHERE is_primary
DO UPDATE SET cover_url  = COALESCE(sqlc.narg('cover_url'), editions.cover_url),
              page_count = COALESCE(sqlc.narg('page_count'), editions.page_count),
              updated_at = now();

-- name: CreateEdition :one
INSERT INTO editions (
    book_id, title, language, translator, publisher, publication_year, isbn, page_count,
    cover_url, is_primary
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- Kısmi güncelleme: NULL = dokunma (books'taki UpdateBookDetails ile aynı desen ve aynı sınır:
-- bir alan NULL'a geri çekilemez). is_primary burada değişmez; ayrı uç (SetPrimaryEdition).
-- name: UpdateEdition :one
UPDATE editions
SET title            = COALESCE(sqlc.narg('title'), title),
    language         = COALESCE(sqlc.narg('language'), language),
    translator       = COALESCE(sqlc.narg('translator'), translator),
    publisher        = COALESCE(sqlc.narg('publisher'), publisher),
    publication_year = COALESCE(sqlc.narg('publication_year'), publication_year),
    isbn             = COALESCE(sqlc.narg('isbn'), isbn),
    page_count       = COALESCE(sqlc.narg('page_count'), page_count),
    cover_url        = COALESCE(sqlc.narg('cover_url'), cover_url),
    updated_at       = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteEdition :exec
DELETE FROM editions WHERE id = $1;

-- name: HasPrimaryEdition :one
SELECT EXISTS (SELECT 1 FROM editions WHERE book_id = $1 AND is_primary)::boolean;

-- Varsayılanı değiştirmek iki adım ve SIRA önemli: önce eskisi temizlenir, sonra yenisi atanır.
-- Tersi, bir an için iki varsayılan demektir ve kısmi benzersiz index'e takılır.
-- name: ClearPrimaryEdition :exec
UPDATE editions SET is_primary = false, updated_at = now()
WHERE book_id = $1 AND is_primary;

-- name: MarkPrimaryEdition :exec
UPDATE editions SET is_primary = true, updated_at = now()
WHERE id = $1;
