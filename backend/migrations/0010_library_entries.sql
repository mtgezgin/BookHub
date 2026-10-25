-- 0010_library_entries.sql — kişisel kitaplık (Faz 15).

-- +goose Up

-- Kullanıcı başına kitap başına TEK kayıt: kural uygulamada değil birincil anahtarda.
-- (user_id, book_id) sırası bilinçli: sol önek user_id, "bir kullanıcının kitaplığı" ve
-- "bu kitaplar onun kitaplığında mı" sorgularına index olur; ayrı bir index gerekmez.
-- Kitaplık GİZLİDİR: bu tabloyu okuyan her sorgu user_id = token'daki kullanıcı ile başlar.
CREATE TABLE library_entries (
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    book_id    bigint NOT NULL REFERENCES books (id) ON DELETE CASCADE,
    status     text NOT NULL CHECK (status IN ('WANT_TO_READ', 'READING', 'READ', 'ABANDONED')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, book_id)
);

-- Liste sorgusu başlığı Türkçe alfabeyle sıralar; kural yoksa migration burada durur ve kod çıkmaz.
SELECT 'a' COLLATE "tr-x-icu";

-- +goose Down

DROP TABLE library_entries;
