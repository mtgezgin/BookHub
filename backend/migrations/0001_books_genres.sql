-- 0001_books_genres.sql — İLK migration: books, genres, book_genres (Faz 2).
--
-- MIGRATION NEDİR: Şemayı "elle psql'de CREATE TABLE çalıştırarak" değil, versiyonlu ve
-- geri alınabilir SQL dosyalarıyla yönetmek. goose bu dosyaları SIRAYLA uygular ve hangi
-- sürümde olduğunu DB içindeki kendi tablosunda (goose_db_version) takip eder.
-- "git ama şema için" gibi düşün: her dosya bir commit, up=apply, down=revert.
--
-- Kurallar (PROJECT_BRIEF §18.3):
--   * PK = bigint GENERATED ALWAYS AS IDENTITY (SERIAL'in modern standardı)
--   * status = TEXT + CHECK (native ENUM değil — yeni değer eklemek ALTER TYPE istemez)
--   * tüm zamanlar timestamptz (timezone'lu; "timestamp" tuzağına düşme)
--   * slug UNIQUE; her FK'ye index (Postgres FK'yi OTOMATİK index'lemez!)
--
-- NOT: avg_rating / rating_count alanları BİLEREK yok — Faz 5'in migration'ı ekleyecek
-- (şema evrimini migration'la yaşamak dersin parçası; §14.5 "ihtiyaç gelince ekle").

-- +goose Up
-- ↑ Bu işaret goose'a "buradan aşağısı UP yönü" der (yorum değil, direktif).

CREATE TABLE books (
    -- GENERATED ALWAYS AS IDENTITY: DB üretir, elle değer YAZILAMAZ (ALWAYS bunu zorlar).
    -- Eski usül SERIAL'den farkı: standart SQL + yanlışlıkla id vermeyi engeller.
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title            text NOT NULL,
    -- slug: URL'de kullanılan kimlik ("clean-code" gibi). UNIQUE = ayrıca index demek.
    slug             text NOT NULL UNIQUE,
    author           text NOT NULL,
    -- Opsiyonel STRING'lerde tercih: NULL yerine NOT NULL DEFAULT '' — Go tarafında
    -- *string/pgtype cambazlığı çıkarmaz. (Opsiyonel SAYILARDA ise NULL kullanıyoruz,
    -- aşağıda — çünkü 0 "bilinmiyor" demek değildir. İki yaklaşımı da bilerek görüyorsun.)
    description      text NOT NULL DEFAULT '',
    cover_url        text NOT NULL DEFAULT '',
    publication_year integer,          -- NULL = bilinmiyor (0 yanıltıcı olurdu)
    page_count       integer,          -- NULL = bilinmiyor
    language         text NOT NULL DEFAULT 'tr',
    -- TEXT + CHECK: ENUM gibi davranır ama evrimi kolay (yeni değer = CHECK'i güncelle).
    -- DEFAULT bilerek YOK: uygulama statüyü AÇIKÇA yazmak zorunda — buglu bir insert'in
    -- kitabı sessizce yayına almasını (APPROVED) istemeyiz.
    status           text NOT NULL CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED')),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

-- PARTIAL INDEX (§18.3): ana liste sorgusu hep "APPROVED'ları en yeniden eskiye" olacak.
-- WHERE'li index SADECE o satırları içerir → küçük kalır, tam da o sorguya hizmet eder.
CREATE INDEX books_approved_recent_idx ON books (created_at DESC) WHERE status = 'APPROVED';

CREATE TABLE genres (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       text NOT NULL UNIQUE,   -- görünen ad ("Bilim Kurgu")
    slug       text NOT NULL UNIQUE,   -- URL hâli ("bilim-kurgu")
    created_at timestamptz NOT NULL DEFAULT now()
);

-- MANY-TO-MANY ara tablosu: bir kitabın N türü, bir türün N kitabı olabilir.
-- İlişkinin kendisi ayrı bir tablo satırı olarak yaşar — JS/ORM dünyasında gizlenen şey.
CREATE TABLE book_genres (
    -- REFERENCES = foreign key. ON DELETE CASCADE: kitap silinirse bağları da silinsin
    -- (bağın kitapsız anlamı yok). Kitabı "yanlışlıkla" silmeye karşı koruma FK'nin işi
    -- değil — o, soft-delete/yetki katmanının işi (Faz 4-5).
    book_id  bigint NOT NULL REFERENCES books (id) ON DELETE CASCADE,
    genre_id bigint NOT NULL REFERENCES genres (id) ON DELETE CASCADE,
    -- Bileşik PK: aynı (kitap,tür) çifti iki kez eklenemez + book_id'den başlayan
    -- aramalar için index görevi görür ("bu kitabın türleri" ← PK prefix'i yeter).
    PRIMARY KEY (book_id, genre_id)
);

-- Ters yön için AYRICA index şart: "bu türdeki kitaplar" genre_id ile arar; bileşik
-- PK (book_id, genre_id) sıralamasında genre_id İKİNCİ sırada → PK bu aramaya yaramaz.
-- (Postgres FK'leri otomatik index'lemez — §18.3'ün tekrar eden uyarısı.)
CREATE INDEX book_genres_genre_id_idx ON book_genres (genre_id);

-- +goose Down
-- ↓ Geri alma: oluşturmanın TERS SIRASINDA sil. books'u önce silemezsin —
-- book_genres'in FK'si ona bağımlı, Postgres izin vermez (dependency koruması).

DROP TABLE book_genres;
DROP TABLE genres;
DROP TABLE books;
