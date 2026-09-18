-- perf_seed.sql — EXPLAIN ANALYZE ve yük testi (Faz 6-7) için SENTETİK veri.
--
-- ⚠️ YALNIZCA AYRI, ATILABİLİR bir veritabanında çalıştır (ör. bookhub_perf) — dev/test
-- veritabanına ~200 bin satır ekler. Kullanım (repo kökünden):
--   docker exec bookhub-postgres createdb -U bookhub bookhub_perf
--   goose -dir backend/migrations postgres "postgres://bookhub:bookhub_dev@localhost:5432/bookhub_perf?sslmode=disable" up
--   docker exec -i bookhub-postgres psql -U bookhub -d bookhub_perf < backend/scripts/perf_seed.sql
--
-- Dağılım gerçekçi olsun diye ÇARPIK: popülerlik random()^3 ile — az sayıda kitap çok oy
-- alır, çoğu az (gerçek kataloglardaki "uzun kuyruk"). Aynı (kullanıcı, kitap) çifti
-- tekrar üretilirse ON CONFLICT DO NOTHING atlar (PK kuralı burada da geçerli).

BEGIN;

INSERT INTO users (email, display_name, password_hash)
SELECT 'perf' || g || '@example.com', 'Okur ' || g, 'x'
FROM generate_series(1, 2000) g;

INSERT INTO books (title, slug, author, status, language)
SELECT 'Kitap ' || g, 'perf-kitap-' || g, 'Yazar ' || (g % 500),
       CASE WHEN g % 10 = 0 THEN 'PENDING' ELSE 'APPROVED' END, 'tr'
FROM generate_series(1, 20000) g;

INSERT INTO ratings (user_id, book_id, score)
SELECT u0.min_id + (g % 2000),
       b0.min_id + floor(20000 * power(random(), 3))::int,
       -- her kitabın kendi "kalitesi" (id'ye bağlı) + gürültü
       greatest(1, least(10, round(4 + ((b0.min_id + g) % 5) + random() * 4 - 2)))::smallint
FROM generate_series(1, 220000) g,
     (SELECT min(id) AS min_id FROM users) u0,
     (SELECT min(id) AS min_id FROM books) b0
ON CONFLICT DO NOTHING;

-- Türetilmiş istatistikleri kaynaktan bir kerede hesapla (uygulamanın her yazımda yaptığının toptan hâli).
UPDATE books b
SET average_rating = s.avg, rating_count = s.cnt
FROM (SELECT book_id, round(avg(score), 2) AS avg, count(*)::int AS cnt FROM ratings GROUP BY book_id) s
WHERE b.id = s.book_id;

COMMIT;

ANALYZE;

SELECT (SELECT count(*) FROM books) AS books,
       (SELECT count(*) FROM books WHERE rating_count > 0) AS rated_books,
       (SELECT count(*) FROM ratings) AS ratings,
       (SELECT max(rating_count) FROM books) AS max_votes_one_book;
