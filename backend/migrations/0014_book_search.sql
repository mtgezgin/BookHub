-- 0014_book_search.sql — Türkçe duyarlı, index'li katalog araması (DISC-01), GENİŞLETME adımı.
--
-- SORUN: arama `title ILIKE '%…%' OR author ILIKE '%…%'` idi. İki kusuru var:
--   1) Doğruluk. Büyük/küçük harf eşlemesi veritabanının yerel ayarına bağlı; canlıda "ince memed"
--      araması "İnce Memed"i bulmuyordu. Aksan (kurk → kürk), kelime sırası (atay oğuz) ve iki alana
--      yayılan sorgu (dostoyevski suç) hiç eşleşmiyordu.
--   2) Hız. Baştaki % yüzünden index kullanılamaz; her arama bütün kitapları okur
--      (500 bin kitapta arama başına ~2 sn ölçüldü).
--
-- ÇÖZÜM: her kitap için önceden hazırlanmış bir ARAMA BELGESİ (book_search) ve onun üzerinde GIN index.
--   * search_fold: metni "arama biçimine" indirger (aşağıda). Belge de sorgu da AYNI fonksiyondan
--     geçer; eşleşmenin tek tanımı burasıdır.
--   * Belge kelimelere ayrılır (tsvector, 'simple': kök bulma ve yok sayılan kelime yok; kitap adı ve
--     kişi adı için doğrusu bu). Ağırlıklar: A = başlık, B = yazar, C = baskı adları ve çevirmenler.
--   * Tetikleyiciler belgeyi kitap ve baskı yazımlarında günceller: uygulamanın hangi yoldan yazdığı
--     fark etmez (API, seed, elle SQL).
--
-- GERİYE UYUM: yalnız yeni nesneler eklenir; canlıdaki eski kod bunları anmaz. Eski kodun kitap ve
-- baskı yazımları yeni tetikleyicileri çalıştırır (istenen de bu). Tasarım ve ölçümler:
-- docs/phases/phase-16b-search.md.

-- +goose Up

-- Aşağıdaki CREATE TRIGGER'lar books ve editions üzerinde kilit ister. Kilit 5 sn içinde alınamazsa
-- (uzun süren bir işlem tabloyu tutuyorsa) migration hata verir ve yayın durur; beklerken arkasına
-- bütün sorguları dizmesindense yeniden denemek daha iyidir (REVIEW_FEEDBACK F38). SET LOCAL yalnız bu
-- migration'ın transaction'ında geçerlidir.
SET LOCAL lock_timeout = '5s';

-- lower() aşağıda veritabanının yerel ayarına değil ICU'nun dilden bağımsız kuralına bağlanır.
-- Bu harmanlama yoksa migration burada durur ve kod çıkmaz (0010'daki tr-x-icu denetimiyle aynı fikir).
SELECT 'a' COLLATE "und-x-icu";

-- search_fold: aramada "aynı" sayılacak yazımları tek biçime indirir.
--   1) normalize(NFKD): harfi taban harf + birleşen işaretlere ayırır ("ş" → "s" + çengel,
--      "İ" → "I" + nokta, "â" → "a" + şapka); bitişik ve tam genişlikli biçimleri de açar ("ﬁ" → "fi").
--   2) birleşen işaretler (U+0300–U+036F) atılır → ş, ğ, ü, ö, ç, â, é … taban harfe iner.
--   3) ayrışmayan harfler açık tabloyla: I ve ı → i (Türkçe'nin özel durumu: I, ı, İ, i dördü de "i"
--      olur), ø → o, ł → l, đ → d, ß → s, æ → a, œ → o.
--   4) lower(): kalan büyük harfler (A–Z ve başka alfabeler). I harfi buraya hiç ulaşmaz, bu yüzden
--      sonuç veritabanının Türkçe ya da İngilizce yerel ayarla kurulmuş olmasına bağlı değildir.
--   5) boşluk ve noktalama öbekleri tek boşluğa iner (kod noktası aralıklarıyla; yerel ayardan bağımsız).
-- IMMUTABLE: aynı girdi her zaman aynı çıktıyı verir; planlayıcı sabit girdide bir kez hesaplar.
-- +goose StatementBegin
CREATE FUNCTION search_fold(t text) RETURNS text
LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS $$
  SELECT btrim(regexp_replace(
           lower(translate(regexp_replace(normalize(t, NFKD), '[̀-ͯ]', '', 'g'),
                           'IıØøŁłĐđßÆæŒœ', 'iioollddsaaoo') COLLATE "und-x-icu"),
           '[\s!-/:-@\[-`{-~ -¿ -⁯　-〿]+', ' ', 'g'))
$$;
-- +goose StatementEnd

-- search_tsquery: katlanmış sorgu metninden arama ifadesi kurar.
--   * Kelimeler AND'lenir: "atay oğuz" her iki kelimeyi de içeren belgeyi bulur, sıra önemsizdir.
--   * SON kelime önek sayılır ("sav" → savaş, savcı): kullanıcı onu hâlâ yazıyor olabilir. Tek
--     kelimelik sorguda önek en az 2 harfte başlar: tek harfin öneki kataloğun büyük bölümüyle
--     eşleşir (500 bin kitapta ortanca 154 ms), tam kelime araması ise 0,1 ms'nin altındadır.
--   * weights boş değilse eşleşme yalnız o ağırlıktaki kelimelerde aranır ('A' = başlık); sıralamada
--     "bütün kelimeler başlıkta mı" sorusunu cevaplamak için kullanılır.
-- Kelimeler tırnak içinde verilir; katlamadan sonra metinde tırnak ve işleç karakteri kalmaz
-- (hepsi noktalama sayılıp boşluğa döner), kullanıcı girdisi ifadenin yapısını değiştiremez.
-- Sorgu boşsa (ya da yalnız noktalamadan oluşuyorsa) NULL döner; NULL ile eşleşme hiçbir satır vermez.
-- +goose StatementBegin
CREATE FUNCTION search_tsquery(folded text, weights text) RETURNS tsquery
LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS $$
  SELECT to_tsquery('simple', string_agg(
           quote_literal(w.tok)
           || CASE WHEN w.ord = w.n AND (w.n > 1 OR char_length(w.tok) >= 2) THEN ':*' || weights
                   WHEN weights <> '' THEN ':' || weights
                   ELSE '' END,
           ' & ' ORDER BY w.ord))
  FROM (SELECT u.tok, u.ord, count(*) OVER () AS n
        FROM unnest(string_to_array(folded, ' ')) WITH ORDINALITY AS u(tok, ord)
        WHERE u.tok <> '') w
$$;
-- +goose StatementEnd

-- ARAMA BELGESİ: kitap başına bir satır. Dar ve kendine yeten bir tablo: süzmek (status) ve sıralamak
-- (rating_count, average_rating, created_at) için gereken her şey aynı satırdadır; sayfanın 12 kitabı
-- belli olmadan geniş books satırlarına hiç gidilmez (ölçüm: geniş sorgularda 2,5–4 kat hızlı).
--   * title_fold: "başlık sorguyla aynı / sorguyla başlıyor" sıralama ölçütleri için.
--   * tsv: aranacak kelimeler (A başlık, B yazar, C baskı adları + çevirmenler). Yayınevi ve ISBN
--     belgede YOK: yayınevi kelimeleri belgelerin yarısından çoğunda geçer (eşleşmeyi anlamsızlaştırır),
--     ISBN her baskıda ayrı bir kelimedir (index'i üç kat büyütür).
--   * fillfactor 90: puan yazımları yalnız rating_count ve average_rating'i değiştirir; sayfada boş yer
--     kalırsa yeni satır sürümü aynı sayfaya yazılır ve index'lere dokunulmaz (HOT). Bu iki kolona
--     bilerek index konmadı: konsaydı her puan yazımı GIN index'ine de yazardı.
CREATE TABLE book_search (
    book_id        bigint PRIMARY KEY REFERENCES books (id) ON DELETE CASCADE,
    status         text NOT NULL,
    rating_count   integer NOT NULL,
    average_rating numeric(4, 2),
    created_at     timestamptz NOT NULL,
    title_fold     text NOT NULL,
    tsv            tsvector NOT NULL
) WITH (fillfactor = 90);

-- Bir kitabın belgesini kaynaktan (books + editions) baştan kurar; satır yoksa ekler, varsa değiştirir.
-- Kitap yoksa (silinmekteyse) hiçbir şey yazmaz.
-- +goose StatementBegin
CREATE FUNCTION refresh_book_search(p_book_id bigint) RETURNS void
LANGUAGE sql AS $$
  INSERT INTO book_search (book_id, status, rating_count, average_rating, created_at, title_fold, tsv)
  SELECT b.id, b.status, b.rating_count, b.average_rating, b.created_at,
         search_fold(b.title),
         setweight(to_tsvector('simple', search_fold(b.title)), 'A') ||
         setweight(to_tsvector('simple', search_fold(b.author)), 'B') ||
         setweight(to_tsvector('simple', search_fold(coalesce(
           (SELECT string_agg(concat_ws(' ', e.title, nullif(e.translator, '')), ' ')
            FROM editions e WHERE e.book_id = b.id), ''))), 'C')
  FROM books b
  WHERE b.id = p_book_id
  ON CONFLICT (book_id) DO UPDATE
    SET status         = EXCLUDED.status,
        rating_count   = EXCLUDED.rating_count,
        average_rating = EXCLUDED.average_rating,
        title_fold     = EXCLUDED.title_fold,
        tsv            = EXCLUDED.tsv
$$;
-- +goose StatementEnd

-- Kitap yazımı: metin (başlık, yazar) değiştiyse belge baştan kurulur; yalnız durum ya da puan
-- değiştiyse dar bir güncelleme yeter (puan yazımlarının sık yolu budur).
-- +goose StatementBegin
CREATE FUNCTION books_search_sync() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' OR NEW.title IS DISTINCT FROM OLD.title OR NEW.author IS DISTINCT FROM OLD.author THEN
        PERFORM refresh_book_search(NEW.id);
    ELSIF NEW.status IS DISTINCT FROM OLD.status
       OR NEW.rating_count IS DISTINCT FROM OLD.rating_count
       OR NEW.average_rating IS DISTINCT FROM OLD.average_rating THEN
        UPDATE book_search
        SET status = NEW.status, rating_count = NEW.rating_count, average_rating = NEW.average_rating
        WHERE book_id = NEW.id;
    END IF;
    RETURN NULL; -- AFTER tetikleyici: dönüş değeri kullanılmaz
END;
$$;
-- +goose StatementEnd

-- Baskı yazımı: baskının adı ya da çevirmeni belgenin C bölümüdür. Baskı başka bir kitaba taşınırsa
-- iki kitabın belgesi de yenilenir.
-- +goose StatementBegin
CREATE FUNCTION editions_search_sync() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM refresh_book_search(OLD.book_id);
    ELSIF TG_OP = 'INSERT' THEN
        PERFORM refresh_book_search(NEW.book_id);
    ELSIF NEW.book_id IS DISTINCT FROM OLD.book_id
       OR NEW.title IS DISTINCT FROM OLD.title
       OR NEW.translator IS DISTINCT FROM OLD.translator THEN
        PERFORM refresh_book_search(NEW.book_id);
        IF NEW.book_id IS DISTINCT FROM OLD.book_id THEN
            PERFORM refresh_book_search(OLD.book_id);
        END IF;
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- SIRA ÖNEMLİ: önce tetikleyiciler, sonra ilk doldurma. CREATE TRIGGER tabloya yazımı bu transaction
-- bitene kadar bekletir (SHARE ROW EXCLUSIVE kilidi); böylece doldurma ile tetikleyicinin devreye
-- girmesi arasında kaçan bir yazım olamaz. Bedeli: migration sürerken kitap ve baskı yazımları
-- bekler (canlıdaki katalogla milisaniyeler; 500 bin kitapta doldurma 22 sn + index 2 sn ölçüldü,
-- o ölçekte doldurma ayrı ve parçalı yapılmalı).
CREATE TRIGGER books_search_sync
    AFTER INSERT OR UPDATE OF title, author, status, rating_count, average_rating ON books
    FOR EACH ROW EXECUTE FUNCTION books_search_sync();

CREATE TRIGGER editions_search_sync
    AFTER INSERT OR DELETE OR UPDATE OF book_id, title, translator ON editions
    FOR EACH ROW EXECUTE FUNCTION editions_search_sync();

-- İlk doldurma: var olan bütün kitaplar (her durumda; görünürlük sorguda status ile süzülür).
INSERT INTO book_search (book_id, status, rating_count, average_rating, created_at, title_fold, tsv)
SELECT b.id, b.status, b.rating_count, b.average_rating, b.created_at,
       search_fold(b.title),
       setweight(to_tsvector('simple', search_fold(b.title)), 'A') ||
       setweight(to_tsvector('simple', search_fold(b.author)), 'B') ||
       setweight(to_tsvector('simple', search_fold(coalesce(e.doc, ''))), 'C')
FROM books b
LEFT JOIN (
    SELECT ed.book_id, string_agg(concat_ws(' ', ed.title, nullif(ed.translator, '')), ' ') AS doc
    FROM editions ed
    GROUP BY ed.book_id
) e ON e.book_id = b.id;

-- GIN: "bu kelimeyi (ya da bu önekle başlayan kelimeleri) içeren belgeler" sorusunun index'i.
-- Doldurmadan SONRA kurulur: dolu tabloda tek seferde kurmak, satır satır eklemekten hızlıdır.
CREATE INDEX book_search_tsv_idx ON book_search USING gin (tsv);

-- Planlayıcı istatistikleri hemen olsun (otomatik ANALYZE'ı beklemeden).
ANALYZE book_search;

-- +goose Down

DROP TRIGGER editions_search_sync ON editions;
DROP TRIGGER books_search_sync ON books;
DROP FUNCTION editions_search_sync();
DROP FUNCTION books_search_sync();
DROP FUNCTION refresh_book_search(bigint);
DROP TABLE book_search;
DROP FUNCTION search_tsquery(text, text);
DROP FUNCTION search_fold(text);
