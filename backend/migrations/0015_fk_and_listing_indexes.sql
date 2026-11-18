-- 0015_fk_and_listing_indexes.sql — index'siz yabancı anahtarlar ve liste index'lerinin sadeleştirilmesi
-- (REVIEW_FEEDBACK F28). Yalnız index değişir: tablo, kolon ve kısıtlara dokunulmaz.
--
-- 1) INDEX'SİZ YABANCI ANAHTARLAR. Bir kullanıcı ya da kitap silinirken Postgres, ona işaret eden her
--    tabloda "bu satıra bağlı kayıt var mı?" diye bakar. O kolonun index'i yoksa tablonun TAMAMINI
--    okur (ölçüm, 500 bin kitap: kullanıcı silme 171 ms, 149'u comments.deleted_by taraması; kitap
--    silme 262 ms). 0001'deki "her yabancı anahtara index" kuralı üç kolonda atlanmıştı:
--      * comments.deleted_by   → kısmi index (yalnız silinmiş yorumlar; kolon çoğunlukla NULL).
--      * comments.book_id      → aşağıdaki (2) ile birlikte.
--      * library_entries.book_id (birincil anahtar (user_id, book_id): book_id ilk kolon değil).
--    Kural bundan sonra bir testte: internal/database/schema_integration_test.go.
--
-- 2) YORUM LİSTESİ. comments_book_visible_idx kısmiydi (WHERE deleted_at IS NULL): yorum listesine
--    hizmet eder ama yabancı anahtar denetimi onu KULLANAMAZ (denetim silinmiş yorumları da arar).
--    İkinci bir index eklemek yerine index tam hâle getirildi ve sıralamanın ikinci anahtarı (id)
--    eklendi. Silinmiş yorumlar azınlıkta; listenin okuduğu fazladan satır ihmal edilebilir.
--
-- 3) PUANLAR. (book_id) yerine (book_id, score): dağılım ve ortalama yalnız index'ten okunur
--    (100 bin oylu kitapta dağılım 29 → 6,2 ms, istatistik yenileme 12,7 → 6,0 ms).
--
-- 4) KİTAP LİSTELERİ. İki kısmi index (APPROVED ve PENDING) yerine tek (status, created_at, id).
--    Reddedilen ve arşivlenen kitapların yönetim listesi index'sizdi (247 ms). Ölçüm (500 bin kitap):
--    tek index dört durumun listesine de hizmet ediyor (0,1–0,5 ms), herkese açık liste yavaşlamıyor
--    ve tek index (24 MB) iki kısmi index'in toplamından (30 MB) küçük.
--
-- KİLİT: CREATE INDEX tablonun yazımlarını index kurulana kadar bekletir (okumalar sürer). Canlıdaki
-- tablolar küçük; kurulum milisaniyeler sürer. CONCURRENTLY kullanılmadı: goose migration'ı tek
-- transaction'da çalıştırır ve CONCURRENTLY transaction içinde çalışmaz. Tablolar büyüdüğünde index
-- eklemek ayrı, transaction'sız bir migration ister (goose'un "NO TRANSACTION" yönergesi; yönerge
-- burada bilerek tam yazılmadı: goose yorum satırındaki yönergeyi de komut sayar).
--
-- GERİYE UYUM: index adlarını kod anmaz; canlıdaki eski kod bu migration'dan önce de sonra da çalışır.

-- +goose Up
SET LOCAL lock_timeout = '5s';

CREATE INDEX comments_deleted_by_idx ON comments (deleted_by) WHERE deleted_by IS NOT NULL;

CREATE INDEX comments_book_recent_idx ON comments (book_id, created_at DESC, id DESC);
DROP INDEX comments_book_visible_idx;

CREATE INDEX library_entries_book_id_idx ON library_entries (book_id);

CREATE INDEX ratings_book_score_idx ON ratings (book_id, score);
DROP INDEX ratings_book_id_idx;

CREATE INDEX books_status_recent_idx ON books (status, created_at DESC, id DESC);
DROP INDEX books_approved_recent_idx;
DROP INDEX books_pending_idx;

-- +goose Down
SET LOCAL lock_timeout = '5s';

CREATE INDEX books_pending_idx ON books (created_at DESC, id DESC) WHERE status = 'PENDING';
CREATE INDEX books_approved_recent_idx ON books (created_at DESC, id DESC) WHERE status = 'APPROVED';
DROP INDEX books_status_recent_idx;

CREATE INDEX ratings_book_id_idx ON ratings (book_id);
DROP INDEX ratings_book_score_idx;

DROP INDEX library_entries_book_id_idx;

CREATE INDEX comments_book_visible_idx ON comments (book_id, created_at DESC) WHERE deleted_at IS NULL;
DROP INDEX comments_book_recent_idx;

DROP INDEX comments_deleted_by_idx;
