-- 0018_reviews.sql — kitap incelemeleri (Faz 17B, #80; REV-01, REV-02).
--
-- MODEL (spec § Kararlar): inceleme yorumdan AYRI bir tablo. Yorum kısa ve çok olur, silinince
-- "silinmiş" görünür; inceleme kullanıcı başına kitapta TEK, düzenlenebilir ve yönetici gizleyebilir.
-- Yorumların tablosu ve silme kuralları aynen kalır.
--
-- PUAN: incelemede ayrı puan yok. Liste yazarın puanını ratings'ten okur; inceleme silmek puanı
-- silmez (REV-01).
-- SÜRÜM (kavram 4): version her düzenlemede artar; düzenleme okuduğu sürümü geri gönderir, uyuşmazsa
-- 409 version_conflict (iki sekmede ezilme yok, okuma ilerlemesindeki gibi).
-- GİZLEME: hidden_at dolu = herkese açık listede yok; sahibi kendi incelemesini gerekçesiyle görür.
-- Sahibi düzenleyince gizli kalır (gizleme kolonlarına yalnız yönetici uçları yazar). Gizlemek sürümü
-- artırmaz: sahibinin açık düzenleyicisi yönetici yüzünden 409 almasın.
-- HESAP SİLİNİNCE: inceleme de silinir (CASCADE), kitaplık ve puanla aynı: kullanıcının kendi verisi.
-- Gizleyen yöneticinin hesabı silinirse gizleme kalır, yalnız "kim gizledi" boşalır (SET NULL).
--
-- GERİYE UYUM: yalnız yeni tablo; canlıdaki kod ona hiç dokunmaz. Kısıtlar baştan (reading_log gibi).
--
-- SPEC'TEN FARKLAR (§ 17B adım 1'deki taslağa göre):
--   1. Liste index'i (book_id, created_at DESC, id DESC): sorgu kararlı sayfalama için id'yi ikinci
--      sıralama anahtarı yapıyor; id index'te yoksa sıralamanın tamamını veremez (0005'in dersi).
--   2. Gizleme tutarlılığı iki yönlü: gizliyse gerekçe dolu, görünürse gerekçe boş (geri açınca
--      eski gerekçe kalmasın). Gerekçe en çok 500 karakter (moderasyondaki ret gerekçesiyle aynı sınır).
--
-- KİLİT: CREATE TABLE yeni tablo; users ve books yabancı anahtar için kısa bir an kilitlenir.

-- +goose Up
SET LOCAL lock_timeout = '5s';

CREATE TABLE reviews (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id       bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    book_id       bigint NOT NULL REFERENCES books (id) ON DELETE CASCADE,
    body          text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 5000),
    has_spoiler   boolean NOT NULL DEFAULT false,
    version       integer NOT NULL DEFAULT 1,
    created_at    timestamptz NOT NULL DEFAULT now(),
    edited_at     timestamptz,                        -- NULL = hiç düzenlenmedi
    hidden_at     timestamptz,                        -- NULL = görünür
    hidden_by     bigint REFERENCES users (id) ON DELETE SET NULL,
    hidden_reason text NOT NULL DEFAULT '' CHECK (char_length(hidden_reason) <= 500),
    -- Kullanıcı başına kitapta tek inceleme; ikinci yazım düzenlemedir (Doğrulama 12). Sol öneki
    -- user_id olduğu için reviews.user_id yabancı anahtarının index'i de budur.
    UNIQUE (user_id, book_id),
    CONSTRAINT reviews_hidden_has_reason CHECK ((hidden_at IS NULL) = (hidden_reason = ''))
);

-- Kitap sayfasındaki liste: bir kitabın incelemeleri, en yeni önce. Gizlenenler sorguda süzülür;
-- az olacakları için index tam (kısmi index yabancı anahtar denetimine sayılmaz, 0015).
CREATE INDEX reviews_book_created_idx ON reviews (book_id, created_at DESC, id DESC);
-- Yabancı anahtar index'i (internal/database/schema_integration_test.go); kolon çoğunlukla boş.
CREATE INDEX reviews_hidden_by_idx ON reviews (hidden_by) WHERE hidden_by IS NOT NULL;

-- +goose Down
SET LOCAL lock_timeout = '5s';

DROP TABLE reviews;
