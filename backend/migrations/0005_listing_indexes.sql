-- 0005_listing_indexes.sql — EXPLAIN ANALYZE'ın bulduğu index düzeltmesi (Faz 6).
--
-- GÖZLEM (20 bin kitaplık sentetik veri, backend/scripts/perf_seed.sql):
--   public liste  ORDER BY created_at DESC, id DESC LIMIT 20
--   0001'deki books_approved_recent_idx (created_at DESC) KULLANILMIYORDU → Seq Scan + sort, 6.5 ms.
--   Sebep: sorgu kararlı sayfalama için id'yi İKİNCİ sıralama anahtarı yapıyor; index yalnızca
--   created_at içerdiği için sıralamanın tamamını veremiyordu.
--   (created_at DESC, id DESC) ile: Index Scan, 0.095 ms (~68x).
-- Aynı eksik admin'in bekleyen kuyruğu için de geçerli (ListBooksByStatus aynı ORDER BY).
--
-- Ders: index'i sorgunun WHERE'ine göre değil, WHERE + ORDER BY'ın TAMAMINA göre tasarla —
-- ve tahmin etme, EXPLAIN ANALYZE ile ölç.

-- +goose Up
DROP INDEX books_approved_recent_idx;
CREATE INDEX books_approved_recent_idx ON books (created_at DESC, id DESC) WHERE status = 'APPROVED';

DROP INDEX books_pending_idx;
CREATE INDEX books_pending_idx ON books (created_at DESC, id DESC) WHERE status = 'PENDING';

-- +goose Down
DROP INDEX books_pending_idx;
CREATE INDEX books_pending_idx ON books (created_at DESC) WHERE status = 'PENDING';

DROP INDEX books_approved_recent_idx;
CREATE INDEX books_approved_recent_idx ON books (created_at DESC) WHERE status = 'APPROVED';
