-- ratings.sql — puan sorguları (Faz 5).

-- Kitabı KİLİTLE (ve onaylı olduğunu doğrula). Neden şart: READ COMMITTED'da iki eşzamanlı
-- puan yazımı birbirinin commit edilmemiş satırını göremez; kilitsiz, her biri AVG/COUNT'u
-- yalnız kendi oyuyla hesaplar ve sonra yazan öncekinin sayımını EZER (kayıp güncelleme).
-- Kilitle ikinci işlem, birincinin commit'ini bekler; sonraki deyimleri yeni bir anlık
-- görüntüyle (snapshot) çalışır ve birinci oyu görür.
-- name: LockApprovedBook :one
SELECT id FROM books
WHERE id = $1 AND status = 'APPROVED'
FOR UPDATE;

-- "Bir kez puan ver + puanını güncelle" TEK deyimde: satır yoksa ekle, varsa (PK çakışması)
-- güncelle. Uygulamada "var mı bak → ekle/güncelle" yazmak yarış kapısıydı.
-- (xmax = 0): Postgres'in iç sütunu; eklenen satırda 0, güncellenen satırda değil →
-- 201 mi 200 mü döneceğimizi ekstra sorgusuz öğreniriz.
-- name: UpsertRating :one
INSERT INTO ratings (user_id, book_id, score)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, book_id)
DO UPDATE SET score = EXCLUDED.score, updated_at = now()
RETURNING score, (xmax = 0)::boolean AS inserted;

-- name: DeleteRating :execrows
DELETE FROM ratings WHERE user_id = $1 AND book_id = $2;

-- İstatistikleri KAYNAKTAN yeniden türet (aynı transaction'da). Artımlı (+1) güncelleme
-- değil: her seferinde ratings'ten hesaplamak, bir hata ya da kaçan bir güncelleme olsa bile
-- bir sonraki yazımda kendini düzelten (self-healing) bir tasarım.
-- name: RefreshBookRatingStats :one
UPDATE books b
SET average_rating = s.avg,
    rating_count   = s.cnt
FROM (
    SELECT round(avg(score), 2)::numeric(4, 2) AS avg, count(*)::integer AS cnt
    FROM ratings
    WHERE book_id = sqlc.arg('book_id')
) s
WHERE b.id = sqlc.arg('book_id')
RETURNING b.average_rating, b.rating_count;

-- name: GetUserRating :one
SELECT score, created_at, updated_at FROM ratings
WHERE user_id = $1 AND book_id = $2;

-- Dağılım: 1..10 her puandan kaç tane (frontend'de histogram).
-- name: RatingDistribution :many
SELECT score, count(*)::integer AS votes
FROM ratings
WHERE book_id = $1
GROUP BY score
ORDER BY score;

-- Genel ortalama C: onaylı kitaplara verilmiş TÜM oyların ortalaması.
--
-- ratings tablosunu (oy başına bir satır) değil, books'taki türetilmiş agregatları okur:
-- Σ(ortalama × oy sayısı) / Σ(oy sayısı) matematiksel olarak aynı ortalama. EXPLAIN ANALYZE
-- (209 bin oy, 20 bin kitap): ratings JOIN books ile 43 ms → agregatlardan 5.7 ms. Fark
-- yalnızca average_rating'in 2 haneye yuvarlanmasından: 5.998415 vs 5.998631.
-- Hiç oy yoksa NULL → COALESCE ile COLD-START varsayılanı (ölçeğin ortası).
-- name: GlobalAverageRating :one
SELECT COALESCE(
           sum(average_rating * rating_count)::float8 / NULLIF(sum(rating_count), 0),
           sqlc.arg('fallback')::float8
       )::float8 AS c
FROM books
WHERE status = 'APPROVED' AND rating_count > 0;

-- En iyi kitaplar, AĞIRLIKLI skora göre — QUERY-TIME hesap (saklanan kolon YOK).
--
-- Neden saklamıyoruz: C (genel ortalama) GLOBAL. Herhangi bir kitaba verilen tek oy C'yi
-- değiştirir ve dolayısıyla BÜTÜN kitapların doğru skorunu değiştirir; her yazımda tüm
-- tabloyu güncellemek ya da bayat skor tutmak gerekirdi. Hesaplamak ucuz (oylu, onaylı
-- kitaplar üzerinde tek geçiş + LIMIT); yavaşladığı ÖLÇÜLÜRSE cache (Faz 7) ya da
-- materialized view düşünülür — önce değil.
--
-- Formül internal/ratings.WeightedScore ile AYNI (entegrasyon testi ikisini karşılaştırır).
-- Eşitlikte daha çok oylu önce, sonra id: sayfalar arası kararlı sıra.
-- name: ListTopRated :many
SELECT b.*,
       ((b.rating_count::float8 / (b.rating_count + sqlc.arg('m')::float8)) * b.average_rating::float8
        + (sqlc.arg('m')::float8 / (b.rating_count + sqlc.arg('m')::float8)) * sqlc.arg('c')::float8
       )::float8 AS weighted_score
FROM books b
WHERE b.status = 'APPROVED' AND b.rating_count > 0
ORDER BY weighted_score DESC, b.rating_count DESC, b.id DESC
LIMIT sqlc.arg('limit');
