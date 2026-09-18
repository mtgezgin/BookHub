-- book_genres.sql — kitap ↔ tür ÇOKA-ÇOK ilişkisi (ara tablo: book_genres).
--
-- İlişkinin kendisi bir tablo satırıdır: (book_id, genre_id). ORM'lerin "book.genres"
-- diye gizlediği şey tam olarak bu JOIN'dir.

-- Kitabın türlerini DEĞİŞTİRMEDEN önce kitap satırını kilitle (FOR UPDATE). Neden:
-- iki admin aynı anda türleri değiştirirse, kilitsiz iki transaction'ın "sil + ekle"
-- adımları iç içe geçip İKİ listenin birleşimini bırakabilirdi. Kilit, aynı kitap için
-- yazımları sıraya sokar. Satır yoksa → kitap yok (404).
-- name: LockBook :one
SELECT id FROM books
WHERE id = $1
FOR UPDATE;

-- name: ClearBookGenres :exec
DELETE FROM book_genres
WHERE book_id = $1;

-- Toplu ekleme: bir dizi tür id'sini TEK statement'ta ekle (unnest dizi → satırlar).
-- Var olmayan bir tür id'si FK ihlali (23503) üretir → "bilinmeyen tür".
-- name: AddBookGenres :exec
INSERT INTO book_genres (book_id, genre_id)
SELECT sqlc.arg('book_id')::bigint, unnest(sqlc.arg('genre_ids')::bigint[]);

-- Tek kitabın türleri (kitap detayı için).
-- name: ListGenresForBook :many
SELECT g.* FROM genres g
JOIN book_genres bg ON bg.genre_id = g.id
WHERE bg.book_id = $1
ORDER BY g.name;

-- BİRDEN ÇOK kitabın türleri TEK sorguda (liste sayfası için).
--
-- N+1 TUZAĞI: 20 kitaplık bir listede her kitap için ayrı "türlerini getir" sorgusu
-- atmak 1 (liste) + 20 (tür) = 21 sorgu demektir; ORM'lerin en ünlü sessiz performans
-- hatası. Burada id'leri dizi olarak verip tek sorguyla çekiyor, gruplamayı Go'da
-- yapıyoruz: kitap sayısı ne olursa olsun toplam 2 sorgu.
-- name: ListGenresForBooks :many
SELECT bg.book_id, g.id, g.name, g.slug
FROM book_genres bg
JOIN genres g ON g.id = bg.genre_id
WHERE bg.book_id = ANY(sqlc.arg('book_ids')::bigint[])
ORDER BY bg.book_id, g.name;
