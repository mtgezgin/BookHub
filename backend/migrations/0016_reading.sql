-- 0016_reading.sql — okuma takibi (Faz 17A-1, #80), GENİŞLETME adımı.
--
-- MODEL (spec § Kararlar, kavram 1): aktif okuma ayrı bir tabloda değil, kitaplık satırının İÇİNDE.
-- "Okuyor mu" bilgisi tek yerde (status) durur; "en fazla bir aktif okuma" birincil anahtardan gelir.
-- Kapanmış okumalar (bitirilen, yarım bırakılan) reading_log'a yazılır. Durum günlükten türetilmez.
--
-- GERİYE UYUM (kavram 5): library_entries'e eklenen her kolon boş olabilir ya da varsayılanlıdır;
-- canlıdaki eski kod bunları hiç anmadan çalışmaya devam eder: INSERT'i kolon listesiyle yazıyor ve
-- sqlc sorgudaki `RETURNING *`'ı üretim anında açık kolon listesine çevirdiği için eski kod yeni
-- kolonları hiç istemez (internal/db/library.sql.go; CI'daki Yükseltme işi eski şema + veriyle
-- sınar). CHECK kısıtları yok: eski kodun yayın penceresinde yazabildiği bir
-- satırı reddetmesinler diye ayrı bir migration'da (0017, daraltma) gelir. reading_log yeni tablo:
-- eski kod ona hiç yazmaz, bu yüzden kısıtlarını baştan taşır.
--
-- TARİH (kavram 6): başlama ve bitiş bir takvim günü → date. "Bugün"ü istemci söyler.
-- ANLIK GÖRÜNTÜ (kavram 7): total_pages başlarken baskıdan kopyalanır; admin baskının sayfa sayısını
-- sonradan düzeltirse ilerleme sessizce başka bir orana dönüşmez.
-- SÜRÜM (kavram 4): version her yazımda artar; ilerleme düzeltmesi okuduğu sürümü geri gönderir,
-- uyuşmazsa 409 version_conflict (iyimser eşzamanlılık).
--
-- KİLİT: ADD COLUMN boş ya da sabit varsayılanlı olduğu için tablo yeniden yazılmaz (yalnız katalog
-- değişir); library_entries kısa bir an ACCESS EXCLUSIVE kilitlenir. Canlıda tablo küçük.

-- +goose Up
SET LOCAL lock_timeout = '5s';

ALTER TABLE library_entries
    -- Baskı silinirse okuma kalır, yalnız baskı bağı kopar (sayfa toplamı zaten satırda).
    ADD COLUMN edition_id  bigint REFERENCES editions (id) ON DELETE SET NULL,
    ADD COLUMN unit        text,       -- 'page' | 'percent'; NULL = ilerleme henüz girilmedi
    ADD COLUMN progress    integer,
    ADD COLUMN total_pages integer,    -- başlarken baskıdan kopyalanır; NULL = bilinmiyor
    ADD COLUMN started_on  date,       -- NULL = bilinmiyor
    ADD COLUMN version     integer NOT NULL DEFAULT 1;

CREATE TABLE reading_log (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id     bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    book_id     bigint NOT NULL REFERENCES books (id) ON DELETE CASCADE,
    edition_id  bigint REFERENCES editions (id) ON DELETE SET NULL,
    outcome     text NOT NULL CHECK (outcome IN ('FINISHED', 'ABANDONED')),
    unit        text CHECK (unit IN ('page', 'percent')),
    progress    integer CHECK (progress >= 0),
    total_pages integer CHECK (total_pages > 0),
    started_on  date,
    ended_on    date,             -- NULL = bilinmiyor ("Zaten okudum")
    created_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (ended_on IS NULL OR started_on IS NULL OR ended_on >= started_on)
);

-- Kitap sayfasındaki geçmiş ve "son yarım kalan okuma": hep bir kullanıcının bir kitabı, en yeni önce.
-- Sol öneki user_id olduğu için reading_log.user_id yabancı anahtarının index'i de budur.
CREATE INDEX reading_log_user_book_idx ON reading_log (user_id, book_id, created_at DESC);

-- Yabancı anahtar index'leri (0015'ten beri kural; internal/database/schema_integration_test.go).
-- Kitap ya da baskı silinirken Postgres bağlı satırları bunlarla arar. edition_id kolonları
-- çoğunlukla boş olacağı için kısmi; "kolon IS NOT NULL" yüklemli kısmi index denetime sayılır
-- (denetim NULL aramaz). library_entries.user_id birincil anahtarın ilk kolonu.
CREATE INDEX library_entries_edition_idx ON library_entries (edition_id) WHERE edition_id IS NOT NULL;
CREATE INDEX reading_log_book_idx ON reading_log (book_id);
CREATE INDEX reading_log_edition_idx ON reading_log (edition_id) WHERE edition_id IS NOT NULL;

-- +goose Down
SET LOCAL lock_timeout = '5s';

DROP TABLE reading_log;

-- Index kolonla birlikte düşer; ayrıca DROP INDEX gerekmez.
ALTER TABLE library_entries
    DROP COLUMN version,
    DROP COLUMN started_on,
    DROP COLUMN total_pages,
    DROP COLUMN progress,
    DROP COLUMN unit,
    DROP COLUMN edition_id;
