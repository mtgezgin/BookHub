-- 0004_comments_ratings.sql — yorumlar ve puanlar (Faz 5).

-- +goose Up

-- Kitabın puan istatistikleri. KAYNAK DEĞİL, TÜRETİLMİŞ veri: tek doğruluk kaynağı
-- ratings tablosu; bu iki kolon her puan yazımında AYNI transaction'da SQL'in AVG/COUNT'u
-- ile yeniden hesaplanır (asla "oku, Go'da +1 yap, geri yaz" — eşzamanlı iki oyda biri kaybolur).
--   average_rating: NUMERIC (FLOAT değil — ikili kayan nokta 7.3'ü tam temsil edemez,
--                   sıralamada ufak kaymalar yapar). NULL = henüz puan yok (0 "kötü" demek olurdu).
ALTER TABLE books
    ADD COLUMN average_rating numeric(4, 2) CHECK (average_rating BETWEEN 1 AND 10),
    ADD COLUMN rating_count   integer NOT NULL DEFAULT 0 CHECK (rating_count >= 0);

-- "Bir kullanıcı bir kitaba bir kez puan verir" kuralı DB'de: (user_id, book_id) PRIMARY KEY.
-- Uygulamadaki bir "önce var mı bak" kontrolü eşzamanlı iki istekte ikisini de geçirirdi;
-- PK'yi ise iki istek aynı anda ihlal edemez. Yapay bir id'ye gerek yok: satırın kimliği
-- zaten bu çift.
CREATE TABLE ratings (
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    book_id    bigint NOT NULL REFERENCES books (id) ON DELETE CASCADE,
    score      smallint NOT NULL CHECK (score BETWEEN 1 AND 10),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, book_id)
);

-- PK (user_id, book_id) "bu kitabın puanları" aramasına yaramaz (book_id ikinci sırada).
CREATE INDEX ratings_book_id_idx ON ratings (book_id);

CREATE TABLE comments (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    book_id    bigint NOT NULL REFERENCES books (id) ON DELETE CASCADE,
    -- Yazar hesabını silerse yorum kalır, "silinmiş kullanıcı" olarak görünür.
    user_id    bigint REFERENCES users (id) ON DELETE SET NULL,
    content    text NOT NULL CHECK (char_length(content) BETWEEN 1 AND 2000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    -- YUMUŞAK SİLME: satır kalır, deleted_at dolar. Tek doğruluk kaynağı deleted_at
    -- (ayrıca bir "status = DELETED" tutmak, ikisinin çelişebileceği bir durum yaratırdı).
    deleted_at timestamptz,
    deleted_by bigint REFERENCES users (id) ON DELETE SET NULL
);

-- "Bu kitabın görünür yorumları, en yeniden eskiye" — partial index yalnız görünürleri içerir.
CREATE INDEX comments_book_visible_idx ON comments (book_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX comments_user_id_idx ON comments (user_id);

-- Okuma yolları bu VIEW'u kullanır: "deleted_at IS NULL" şartını her sorguda hatırlamak
-- yerine tek yerde tanımlıyoruz. Silinmiş yorumu yanlışlıkla göstermek artık bir sorguda
-- WHERE unutmakla değil, bilerek "comments" tablosuna gitmekle mümkün.
CREATE VIEW visible_comments AS
SELECT * FROM comments WHERE deleted_at IS NULL;

-- +goose Down

DROP VIEW visible_comments;
DROP TABLE comments;
DROP TABLE ratings;
ALTER TABLE books
    DROP COLUMN rating_count,
    DROP COLUMN average_rating;
