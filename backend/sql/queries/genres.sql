-- genres.sql — tür (genre) sorguları (sqlc girdisi).

-- name: ListGenres :many
SELECT * FROM genres
ORDER BY name;

-- name: GetGenre :one
SELECT * FROM genres
WHERE id = $1;

-- name: CreateGenre :one
INSERT INTO genres (name, slug)
VALUES ($1, $2)
RETURNING *;

-- KISMİ güncelleme (PATCH): gönderilmeyen alan NULL gelir ve COALESCE mevcut değeri
-- korur. Böylece "önce oku, sonra değiştirip yaz" yarışı olmadan tek statement'ta
-- güncelliyoruz. sqlc.narg = "nullable argüman" → Go tarafında *string.
-- name: UpdateGenre :one
UPDATE genres
SET name = COALESCE(sqlc.narg('name'), name),
    slug = COALESCE(sqlc.narg('slug'), slug)
WHERE id = sqlc.arg('id')
RETURNING *;

-- Silme: book_genres'teki FK "ON DELETE CASCADE" olduğu için bu türün kitaplarla
-- bağları da silinir; kitapların kendisi etkilenmez. :execrows → etkilenen satır sayısı
-- (0 = böyle bir tür yoktu → 404).
-- name: DeleteGenre :execrows
DELETE FROM genres
WHERE id = $1;
