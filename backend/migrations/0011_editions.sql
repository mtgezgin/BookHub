-- 0011_editions.sql — eser ve baskı (Faz 16, #36), GENİŞLETME adımı.
--
-- books = ESER (puan, yorum, tür, URL, özgün dil); editions = BASKI (dil, çevirmen, yayınevi, yıl,
-- ISBN, sayfa, kapak). Bu migration eski kodla uyumlu: books'tan hiçbir şey silinmez. Eski kod
-- books.cover_url / books.page_count okumaya ve yazmaya devam eder; yeni kod bu iki kolonu ANMAZ,
-- değerleri varsayılan baskıdan (book_view) okur. Kolonlar daraltma PR'ında silinir
-- (docs/phases/phase-16-works-editions.md).

-- +goose Up

CREATE TABLE editions (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    book_id          bigint NOT NULL REFERENCES books (id) ON DELETE CASCADE,
    title            text NOT NULL,
    language         text NOT NULL,
    -- İsteğe bağlı metinler books'taki kalıpla: NOT NULL DEFAULT '' ('' = bilinmiyor).
    translator       text NOT NULL DEFAULT '',
    publisher        text NOT NULL DEFAULT '',
    publication_year integer,          -- NULL = bilinmiyor
    -- ISBN benzersiz DEĞİL: eski baskılarda çoğu zaman yok, aynı ISBN'li iki kayıt da olabilir.
    isbn             text NOT NULL DEFAULT '',
    page_count       integer,          -- NULL = bilinmiyor
    cover_url        text NOT NULL DEFAULT '',
    is_primary       boolean NOT NULL DEFAULT false,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX editions_book_id_idx ON editions (book_id);

-- KISMİ BENZERSİZ INDEX: "kitap başına en çok bir varsayılan baskı" kuralını uygulama değil
-- veritabanı garanti eder. Yalnız is_primary = true satırlar index'e girer; varsayılan olmayan
-- baskılar istediği kadar olabilir.
CREATE UNIQUE INDEX editions_one_primary_idx ON editions (book_id) WHERE is_primary;

-- Taşıma: her kitaba bugünkü kapak ve sayfa sayısıyla TEK varsayılan baskı. Dil eserin dilidir;
-- demo kitaplarda 0012 bunu baskının gerçek diline düzeltir.
INSERT INTO editions (book_id, title, language, page_count, cover_url, is_primary)
SELECT id, title, language, page_count, cover_url, true FROM books;

-- book_view: kitabın (eserin) satırı + varsayılan baskısının kapağı ve sayfa sayısı. Kitap okuyan
-- her sorgu bu görünümden okur; JOIN tek yerde tanımlı. Kolonlar books'taki sırayla ve adlarıyla:
-- sqlc'nin ürettiği db.BookView, db.Book'la aynı alanlara sahip olur.
-- Kolonlar AÇIKÇA yazılı (b.* değil): görünüm books.cover_url ve books.page_count'a bağımlı
-- olmamalı, yoksa Postgres daraltmada o kolonları silmeyi reddeder. Bedeli: books'a ileride
-- eklenen bir kolon görünüme kendiliğinden girmez, yeni bir migration görünümü yeniden tanımlar.
-- Baskısız kitap (yayın aralığında eski kodla eklenen) LEFT JOIN sayesinde yine görünür: kapak ''
-- (arayüz üretilmiş kapak çizer), sayfa NULL.
CREATE VIEW book_view AS
SELECT b.id,
       b.title,
       b.slug,
       b.author,
       b.description,
       COALESCE(pe.cover_url, '') AS cover_url,
       b.publication_year,
       pe.page_count,
       b.language,
       b.status,
       b.created_at,
       b.updated_at,
       b.submitted_by,
       b.rejection_reason,
       b.moderated_by,
       b.moderated_at,
       b.average_rating,
       b.rating_count
FROM books b
LEFT JOIN editions pe ON pe.book_id = b.id AND pe.is_primary;

-- +goose Down

DROP VIEW book_view;
DROP TABLE editions;
