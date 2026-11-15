-- 0013_drop_book_cover_pages.sql — eser ve baskı (Faz 16, #36), DARALTMA adımı.
--
-- 0011 (genişletme) her kitaba varsayılan baskı açtı ve kod kapak ile sayfa sayısını o günden beri
-- varsayılan baskıdan (book_view) okuyor; books.cover_url ve books.page_count'u hiçbir sorgu anmıyor
-- (kanıtı internal/editions/legacy_columns_integration_test.go). Bu migration iki eski kolonu siler.
-- Kural gereği ayrı bir PR'da ve 0011 + yeni kod canlıda doğrulandıktan sonra geldi.

-- +goose Up

-- Önce eksik baskıları tamamla: 0011'in uygulanmasıyla yeni kodun yayına çıkması arasındaki
-- dakikalarda ESKİ kodla eklenen bir kitabın varsayılan baskısı olmaz (eski kod editions'ı tanımaz).
-- O kitabın kapağı ve sayfa sayısı bugün yalnız eski kolonlarda; silmeden önce baskıya taşınır.
-- Varsayılan baskısı olan kitaba dokunulmaz.
INSERT INTO editions (book_id, title, language, page_count, cover_url, is_primary)
SELECT b.id, b.title, b.language, b.page_count, b.cover_url, true
FROM books b
WHERE NOT EXISTS (SELECT 1 FROM editions e WHERE e.book_id = b.id AND e.is_primary);

-- book_view bu kolonlara bağlı değil (0011'de kolonlar açıkça yazıldı), Postgres silmeye izin verir.
ALTER TABLE books
    DROP COLUMN cover_url,
    DROP COLUMN page_count;

-- +goose Down

-- Kolonlar 0001'deki tanımlarıyla geri eklenir ve değerler varsayılan baskıdan kopyalanır; böylece
-- 0012'ye kadarki kod (eski kolonları anmasa da) ve daha eski kod yine çalışır. Kolon SIRASI
-- 0001'dekinden farklı olur (sona eklenir); hiçbir sorgu `books.*` sırasına dayanmıyor.
ALTER TABLE books
    ADD COLUMN cover_url  text NOT NULL DEFAULT '',
    ADD COLUMN page_count integer;

UPDATE books b
SET cover_url = e.cover_url, page_count = e.page_count
FROM editions e
WHERE e.book_id = b.id AND e.is_primary;
