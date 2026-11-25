-- 0017_reading_checks.sql — okuma takibinin DARALTMA adımı (Faz 17A-2, #80).
--
-- 0016 okuma kolonlarını kısıtsız ekledi: yayın penceresinde canlıdaki eski kod bir satırı
-- reddettirmesin diye (expand → deploy → contract, CLAUDE.md). 17A-1 canlıda doğrulandı
-- (2026-11-25); artık kurallar veritabanında da zorlanır. Servis aynı kuralları zaten Go'da denetler
-- (internal/library validProgress: istemci 500 değil 400 görsün). Bu kısıtlar, servisi atlayan
-- bir yazımın (elle SQL, ileride yazılacak başka bir kod yolu) kuralı bozmasını engeller.
--
-- TEMİZLİK: yayın penceresinde eski kod okunan bir kitabın durumunu değiştirdiyse okuma alanları
-- satırda bayat kalmış olabilir (spec 17A § 1). Okuma alanlarının anlamı yalnız status = 'READING'
-- iken var; başka durumda boşaltılır. Bu bilgi kaybı değil: yeni kod o alanları zaten okumaz.
--
-- KURAL DIŞI İLERLEME TEMİZLENMEZ: okunan bir kitapta sayfa toplamı aşan ya da 0–100 dışı yüzde
-- olsaydı ADD CONSTRAINT hata verir, transaction geri alınır ve Deploy durur (release ilerlemez).
-- Sessizce silmek yerine durmak bilinçli: yeni kod böyle bir satır yazamaz, eski kod ilerleme hiç
-- yazmazdı; varsa nedenini görmek gerekir. Canlıdaki sayım PR'da.
--
-- SPEC'TEN İKİ FARK (spec 17A § 2'deki taslağa göre):
--   1. İlerleme kısıtı NULL'a dayanıklı yazıldı (aşağıda "NULL TUZAĞI"); taslak kural dışı satırları
--      geçiriyordu.
--   2. Sayfa toplamı biliniyorsa 0'dan büyük olmalı (total_pages > 0). reading_log aynı kuralı 0016'dan
--      beri taşıyor ve servis yalnız pozitif toplamı kopyalıyor (snapshot); editions.page_count
--      kısıtsız olduğu için kuralın veritabanında da durması gerekiyor.
--
-- KİLİT: ADD CONSTRAINT tabloyu ACCESS EXCLUSIVE kilitler ve bütün satırları bir kez denetler.
-- Canlıda tablo küçük (onlarca satır); NOT VALID + VALIDATE ayrımına gerek yok.

-- +goose Up
SET LOCAL lock_timeout = '5s';

UPDATE library_entries
SET edition_id = NULL, unit = NULL, progress = NULL, total_pages = NULL, started_on = NULL
WHERE status <> 'READING'
  AND (edition_id IS NOT NULL OR unit IS NOT NULL OR progress IS NOT NULL
       OR total_pages IS NOT NULL OR started_on IS NOT NULL);

ALTER TABLE library_entries
    -- Okuma alanları yalnız okunan kitapta.
    ADD CONSTRAINT library_reading_only_when_reading CHECK (
        status = 'READING'
        OR (edition_id IS NULL AND unit IS NULL AND progress IS NULL
            AND total_pages IS NULL AND started_on IS NULL)),
    -- İlerleme: hiç girilmemiş (Faz 15'ten kalan okuma), yüzde 0–100 (toplamsız) ya da sayfa
    -- (toplam biliniyorsa onu aşmaz). Go karşılığı: internal/library validProgress.
    --
    -- NULL TUZAĞI: CHECK ifadesi NULL dönerse satır KABUL edilir (yalnız false reddedilir). Spec'teki
    -- "(unit = 'percent' AND progress BETWEEN 0 AND 100 …) OR …" biçiminde birim boş ama ilerleme 5
    -- olan satır, ya da birim 'page' ama ilerleme boş olan satır NULL'a düşüp geçiyordu. Bu yüzden
    -- CASE: her dal önce IS [NOT] NULL ile başlar ve yalnız true/false döner; bilinmeyen birim false.
    ADD CONSTRAINT library_progress_valid CHECK (
        CASE
            WHEN unit IS NULL THEN progress IS NULL AND total_pages IS NULL
            WHEN unit = 'percent' THEN progress IS NOT NULL AND progress BETWEEN 0 AND 100
                                       AND total_pages IS NULL
            WHEN unit = 'page' THEN progress IS NOT NULL AND progress >= 0
                                    AND (total_pages IS NULL OR (total_pages > 0 AND progress <= total_pages))
            ELSE false
        END);

-- +goose Down
SET LOCAL lock_timeout = '5s';

-- Temizlik geri alınmaz: boşaltılan alanlar zaten anlamsızdı.
ALTER TABLE library_entries
    DROP CONSTRAINT library_progress_valid,
    DROP CONSTRAINT library_reading_only_when_reading;
