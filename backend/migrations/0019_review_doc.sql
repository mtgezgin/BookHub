-- 0019_review_doc.sql — incelemenin zengin metin belgesi (Faz 17C, #128; REV-04). GENİŞLETME.
--
-- NE: reviews.body_doc, incelemenin belge hâli (ProseMirror JSON'unun kısıtlı alt kümesi; spec karar
-- 1 ve 2). HTML hiçbir yerde saklanmaz. Belgenin içini (izin verilen düğümler, işaretler, sınırlar)
-- internal/richtext doğrular; veritabanı yalnız "bu bir JSON nesnesi" der.
--
-- BOŞ OLABİLİR: NULL = "belgesi yok, body'den çevrilir" (karar 6). Bugünkü satırlar ve eski kodun
-- yazdıkları NULL kalır. NOT NULL ve kalıcı çeviri daraltma PR'ının işi (adım 8).
--
-- body ve has_spoiler KALIR (karar 5): body belgenin düz metni, has_spoiler "belgede spoiler işareti
-- var" olur; ikisini de servis belgeden türetir. 1–5000 karakter sınırı body'nin CHECK'inde durmaya
-- devam eder.
--
-- JSON null TUZAĞI: jsonb 'null' değeri SQL NULL'u DEĞİLDİR; "belgesi yok" diye okunmaz ama belge de
-- değildir. CHECK onu, diziyi, metni ve sayıyı reddeder: kolonda ya SQL NULL ya bir nesne durur.
--
-- GERİYE UYUM: canlıdaki kod kolonu bilmiyor. sqlc "SELECT *" ve "RETURNING *"ı üretim anında açık
-- kolon listesine çevirdiği için (internal/db/reviews.sql.go) eski kod yeni kolonu okumaz; INSERT'i
-- kolonu saymadığı için NULL yazar. İkisi de 0019 testinde eski kodun sorgu metniyle sınanır.
--
-- KİLİT: varsayılanı olmayan, boş olabilen kolon eklemek yalnız katalog değişikliğidir. CHECK aynı
-- komutta eklendiği için tablo bir kez taranır; reviews küçük (canlıda 1 satır, 2026-11-27).

-- +goose Up
SET LOCAL lock_timeout = '5s';

ALTER TABLE reviews
    ADD COLUMN body_doc jsonb
    CONSTRAINT reviews_body_doc_is_object CHECK (body_doc IS NULL OR jsonb_typeof(body_doc) = 'object');

-- +goose Down
SET LOCAL lock_timeout = '5s';

ALTER TABLE reviews DROP COLUMN body_doc;
