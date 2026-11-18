-- books.sql — kitap sorguları (sqlc girdisi).
--
-- SÖZDİZİMİ: her sorgunun üstündeki "-- name: X :tip" satırı sqlc DİREKTİFİDİR:
--   X    = üretilecek Go fonksiyonunun adı
--   :one = tek satır döner  → (Book, error)
--   :many= çok satır döner  → ([]Book, error)
--   :exec= satır döndürmez  → error          (UPDATE/DELETE için)
-- $1, $2... = Postgres parametreleri → Go fonksiyonunun argümanları olur.
-- (JS karşılığı: prepared statement placeholder'ları — string birleştirme YOK,
--  SQL injection bu kapıdan giremez.)

-- HERKESE AÇIK tekil okuma. status filtresi ŞART: onu unutursak sıralı id'ler
-- (bigint IDENTITY) üzerinden yürüyen herkes onaylanmamış kitapları okuyabilir —
-- OWASP API Top 10'daki BOLA/IDOR kalıbı. Sınır burada, SQL'de yaşasın ki
-- handler'daki bir "if" hatasıyla açılamasın. Bulunmayan satır = 404: onaylanmamış
-- bir kitabın VARLIĞINI bile sızdırmıyoruz.
-- (Admin'in tüm statüleri görmesi ileride AYRI bir sorgu + /api/admin rotası olur.)
-- Faz 16: kitap okuyan sorgular books'tan değil book_view'dan okur (kapak ve sayfa sayısı
-- varsayılan baskıdan gelir; tanım 0011_editions.sql'de).
-- name: GetApprovedBook :one
SELECT * FROM book_view
WHERE id = $1
  AND status = 'APPROVED';

-- Ana liste: sadece onaylılar, en yeniden eskiye; opsiyonel TÜR filtresi ve SAYFALAMA ile.
-- Metin araması bu sorguda DEĞİL: ?q= geldiğinde SearchBooks (aşağıda) çalışır.
--
-- Opsiyonel filtre deseni: `(param IS NULL OR koşul)`. Parametre gönderilmezse (NULL)
-- koşul devre dışı kalır. sqlc'nin "dinamik sorgularda hantal" dediğimiz zayıflığının
-- standart çözümü bu: sorgu metni sabit kalır, filtreyi DEĞER açıp kapatır — string
-- birleştirerek SQL kurmaya (injection kapısı) hiç gerek kalmaz.
--
-- Sıralamada id ikincil anahtar: aynı created_at'e sahip iki kitap sayfalar arasında
-- yer değiştirmesin (kararlı sayfalama). Partial index (created_at DESC) WHERE
-- status='APPROVED' bu sorguya hizmet ediyor.
-- name: ListBooks :many
SELECT b.* FROM book_view b
WHERE b.status = 'APPROVED'
  AND (sqlc.narg('genre')::text IS NULL OR EXISTS (
        SELECT 1 FROM book_genres bg
        JOIN genres g ON g.id = bg.genre_id
        WHERE bg.book_id = b.id AND g.slug = sqlc.narg('genre')))
ORDER BY b.created_at DESC, b.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- Katalogda "en yüksek puan" sırası: ListTopRated ile AYNI ağırlıklı (Bayes) skor. Basit
-- ortalama olsaydı 1 oyla 10 alan kitap, 40 oyla 9 alanın önüne geçerdi.
-- C (genel ortalama) parametre: tek tanımı ratings.sql'deki GlobalAverageRating; repository önce onu
-- okur. Burada ikinci bir kopyası olsaydı şerit ile katalog sırası sessizce ayrışabilirdi.
-- Oysuz kitap NULL → en sonda (kendi aralarında en yeni önce). CASE, m = 0 iken oysuz
-- kitapta 0/0 bölmesini de önler (RATING_MIN_VOTES 0 olabilir).
-- Tür filtresi ListBooks ile birebir aynı; birini değiştiren ikisini de değiştirir. Aynı skor
-- SearchBooks'un rating sırasında da var.
-- name: ListBooksByRating :many
SELECT b.* FROM book_view b
WHERE b.status = 'APPROVED'
  AND (sqlc.narg('genre')::text IS NULL OR EXISTS (
        SELECT 1 FROM book_genres bg
        JOIN genres g ON g.id = bg.genre_id
        WHERE bg.book_id = b.id AND g.slug = sqlc.narg('genre')))
ORDER BY
  CASE WHEN b.rating_count = 0 THEN NULL ELSE
       (b.rating_count::float8 / (b.rating_count + sqlc.arg('m')::float8)) * b.average_rating::float8
     + (sqlc.arg('m')::float8 / (b.rating_count + sqlc.arg('m')::float8)) * sqlc.arg('c')::float8
  END DESC NULLS LAST,
  b.rating_count DESC, b.created_at DESC, b.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- Aynı filtreyle toplam sayı (sayfalama arayüzü "57 sonuçtan 1-20" diyebilsin).
-- name: CountBooks :one
SELECT count(*) FROM books b
WHERE b.status = 'APPROVED'
  AND (sqlc.narg('genre')::text IS NULL OR EXISTS (
        SELECT 1 FROM book_genres bg
        JOIN genres g ON g.id = bg.genre_id
        WHERE bg.book_id = b.id AND g.slug = sqlc.narg('genre')));

-- Katalog araması (?q=): Türkçe duyarlı ve index'li (0014_book_search.sql). Sayfa, toplam ve türler
-- TEK sorguda döner: canlıda her sorgu ayrı bir ağ gidiş-dönüşüdür (REVIEW_FEEDBACK F20).
--
-- q (tek satır, MATERIALIZED): sorgu metni BİR kez katlanır ve arama ifadeleri bir kez kurulur.
--   Bu adım olmasaydı, hazırlanmış deyim genel plana geçtiğinde fonksiyonlar eşleşen her satır için
--   yeniden çalışırdı (ölçüm: 10 ms'lik sorgu 42–181 ms'ye çıkıyordu).
-- page: eşleşen belgeler (GIN index), sıralama anahtarları, toplam ve sayfa. Dar book_search
--   tablosunda kalır; books ve editions'a yalnız sayfadaki kitaplar için gidilir.
--   total: count(*) OVER (), LIMIT'ten ÖNCE süzülmüş satırları sayar. Sayfa sonun ötesindeyse
--   satır da toplam da dönmez; repository o durumda ilk satırı ister (ListLibrary ile aynı).
--
-- Sıra (sort):
--   relevance: başlık sorguyla aynı > başlık sorguyla başlıyor > kademe (2: bütün kelimeler
--     başlıkta, 1: başlık + yazarda, 0: baskı adı ya da çevirmende) > oy sayısı.
--     ts_rank kullanılmaz: kelime tekrarını ödüllendirir, başka baskıdaki başlık tekrarı çok
--     okunan kitabın önüne geçer.
--   newest: en yeni önce.   rating: ListBooksByRating ile aynı ağırlıklı skor (m ve c parametre).
--   Seçilmeyen sıranın CASE'i NULL üretir ve sırayı etkilemez (ListLibrary'deki desen).
-- LIKE q.f || '%': katlanmış metinde % ve _ kalmaz (noktalama boşluğa döner), joker olamaz.
-- Son JOIN'deki status koşulu ikinci hat: belge ile kitap ayrışırsa (olmamalı) yayında olmayan
-- kitap yine de dönmez.
-- name: SearchBooks :many
WITH q AS MATERIALIZED (
    SELECT x.f,
           search_tsquery(x.f, '')   AS tsq,
           search_tsquery(x.f, 'A')  AS tsq_title,
           search_tsquery(x.f, 'AB') AS tsq_title_author
    FROM (SELECT search_fold(sqlc.arg('query')::text) AS f) x
), page AS (
    SELECT s.book_id, s.created_at,
           CASE WHEN sqlc.arg('sort')::text = 'relevance' THEN s.title_fold = q.f END AS k_exact,
           CASE WHEN sqlc.arg('sort')::text = 'relevance' THEN s.title_fold LIKE q.f || '%' END AS k_prefix,
           CASE WHEN sqlc.arg('sort')::text = 'relevance' THEN
                CASE WHEN s.tsv @@ q.tsq_title THEN 2 WHEN s.tsv @@ q.tsq_title_author THEN 1 ELSE 0 END
           END AS k_tier,
           CASE WHEN sqlc.arg('sort')::text = 'rating' AND s.rating_count > 0 THEN
                (s.rating_count::float8 / (s.rating_count + sqlc.arg('m')::float8)) * s.average_rating::float8
              + (sqlc.arg('m')::float8 / (s.rating_count + sqlc.arg('m')::float8)) * sqlc.arg('c')::float8
           END AS k_score,
           CASE WHEN sqlc.arg('sort')::text <> 'newest' THEN s.rating_count END AS k_votes,
           count(*) OVER () AS total
    FROM q
    JOIN book_search s ON s.tsv @@ q.tsq
    WHERE s.status = 'APPROVED'
      AND (sqlc.narg('genre')::text IS NULL OR EXISTS (
            SELECT 1 FROM book_genres bg
            JOIN genres g ON g.id = bg.genre_id
            WHERE bg.book_id = s.book_id AND g.slug = sqlc.narg('genre')))
    ORDER BY k_exact DESC NULLS LAST, k_prefix DESC NULLS LAST, k_tier DESC NULLS LAST,
             k_score DESC NULLS LAST, k_votes DESC NULLS LAST, s.created_at DESC, s.book_id DESC
    LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset')
)
SELECT sqlc.embed(b), p.total,
       COALESCE((SELECT jsonb_agg(jsonb_build_object('id', g.id, 'name', g.name, 'slug', g.slug) ORDER BY g.name)
                 FROM book_genres bg
                 JOIN genres g ON g.id = bg.genre_id
                 WHERE bg.book_id = p.book_id), '[]'::jsonb)::jsonb AS genres
FROM page p
JOIN book_view b ON b.id = p.book_id AND b.status = 'APPROVED'
ORDER BY p.k_exact DESC NULLS LAST, p.k_prefix DESC NULLS LAST, p.k_tier DESC NULLS LAST,
         p.k_score DESC NULLS LAST, p.k_votes DESC NULLS LAST, p.created_at DESC, p.book_id DESC;

-- submitted_by: kitabı kim getirdi (öneren kullanıcı ya da doğrudan ekleyen admin).
-- Kapak ve sayfa sayısı kitaba değil varsayılan baskıya yazılır (editions.sql → CreatePrimaryEdition,
-- aynı transaction'da). Faz 16'dan beri kitap yazımları yalnız id döner; kitabın tam hâli
-- book_view'dan okunur. RETURNING * books.cover_url'ü anardı, daraltmadan sonra kırılırdı.
-- name: CreateBook :one
INSERT INTO books (
    title, slug, author, description, publication_year, language, status, submitted_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING id;

-- ADMIN tekil okuma: statüden bağımsız. Public GetApprovedBook'un ayrı kalması bilinçli —
-- görünürlük kuralı bir parametreye ("includeHidden bool") indirgenirse bir handler'ın
-- yanlış değeri geçmesi BOLA'yı geri getirir. İki sorgu, iki ayrı niyet.
-- name: GetBook :one
SELECT * FROM book_view WHERE id = $1;

-- Moderasyon için kilitli okuma: statü kontrolü + güncelleme arasına başka bir admin
-- giremesin (iki admin aynı anda approve/reject → ikisi de "PENDING" görüp ikisi de yazardı).
-- Kilit book_view'da değil books'ta: Postgres, LEFT JOIN'in boş olabilen tarafını (baskı)
-- kilitleyemez. Yalnız statü döner. Baskı servisi de aynı kilidi alır (kitabın baskıları
-- arasındaki "tek varsayılan" kuralı için).
-- name: LockBookForModeration :one
SELECT status FROM books WHERE id = $1 FOR UPDATE;

-- name: SetBookStatus :exec
UPDATE books
SET status           = sqlc.arg('status'),
    rejection_reason = sqlc.narg('rejection_reason'),
    moderated_by     = sqlc.arg('moderated_by'),
    moderated_at     = now(),
    updated_at       = now()
WHERE id = sqlc.arg('id');

-- Kısmi güncelleme (PATCH): gönderilmeyen alan (NULL) mevcut değeri korur.
-- Sınır: bu desenle bir alanı NULL'A ÇEKMEK mümkün değil (publication_year'ı "bilinmiyor"
-- yapmak gibi) — NULL "dokunma" anlamına geliyor. Gerekirse ayrı bir "clear" bayrağı gerekir.
-- Kapak ve sayfa sayısı varsayılan baskıdadır (editions.sql → PatchPrimaryEdition).
-- name: UpdateBookDetails :one
UPDATE books
SET title            = COALESCE(sqlc.narg('title'), title),
    slug             = COALESCE(sqlc.narg('slug'), slug),
    author           = COALESCE(sqlc.narg('author'), author),
    description      = COALESCE(sqlc.narg('description'), description),
    publication_year = COALESCE(sqlc.narg('publication_year'), publication_year),
    language         = COALESCE(sqlc.narg('language'), language),
    updated_at       = now()
WHERE id = sqlc.arg('id')
RETURNING id;

-- Admin listesi: tek statüye göre (bekleyen kuyruğu, arşiv, reddedilenler...).
-- name: ListBooksByStatus :many
SELECT * FROM book_view
WHERE status = sqlc.arg('status')
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountBooksByStatus :one
SELECT count(*) FROM books WHERE status = $1;