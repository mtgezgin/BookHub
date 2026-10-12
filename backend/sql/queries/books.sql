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
-- name: GetApprovedBook :one
SELECT * FROM books
WHERE id = $1
  AND status = 'APPROVED';

-- Ana liste: sadece onaylılar, en yeniden eskiye; opsiyonel TÜR filtresi, METİN araması
-- ve SAYFALAMA ile.
--
-- Opsiyonel filtre deseni: `(param IS NULL OR koşul)`. Parametre gönderilmezse (NULL)
-- koşul devre dışı kalır. sqlc'nin "dinamik sorgularda hantal" dediğimiz zayıflığının
-- standart çözümü bu: sorgu metni sabit kalır, filtreyi DEĞER açıp kapatır — string
-- birleştirerek SQL kurmaya (injection kapısı) hiç gerek kalmaz.
--
-- Arama ILIKE ile (büyük/küçük harf duyarsız). `%` ve `_` istemci girdisinde joker
-- olmasın diye Go tarafında kaçışlanıyor (bkz. books.escapeLike).
--
-- Sıralamada id ikincil anahtar: aynı created_at'e sahip iki kitap sayfalar arasında
-- yer değiştirmesin (kararlı sayfalama). Partial index (created_at DESC) WHERE
-- status='APPROVED' bu sorguya hizmet ediyor.
-- name: ListBooks :many
SELECT b.* FROM books b
WHERE b.status = 'APPROVED'
  AND (sqlc.narg('genre')::text IS NULL OR EXISTS (
        SELECT 1 FROM book_genres bg
        JOIN genres g ON g.id = bg.genre_id
        WHERE bg.book_id = b.id AND g.slug = sqlc.narg('genre')))
  AND (sqlc.narg('query')::text IS NULL
       OR b.title  ILIKE '%' || sqlc.narg('query') || '%'
       OR b.author ILIKE '%' || sqlc.narg('query') || '%')
ORDER BY b.created_at DESC, b.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- Katalogda "en yüksek puan" sırası: ListTopRated ile AYNI ağırlıklı (Bayes) skor. Basit
-- ortalama olsaydı 1 oyla 10 alan kitap, 40 oyla 9 alanın önüne geçerdi.
-- C (genel ortalama) bağımsız bir alt sorgu: Postgres onu bir kez hesaplar (InitPlan).
-- Oysuz kitap NULL → en sonda (kendi aralarında en yeni önce). CASE, m = 0 iken oysuz
-- kitapta 0/0 bölmesini de önler (RATING_MIN_VOTES 0 olabilir).
-- Filtreler ListBooks ile birebir aynı; birini değiştiren ikisini de değiştirir.
-- name: ListBooksByRating :many
SELECT b.* FROM books b
WHERE b.status = 'APPROVED'
  AND (sqlc.narg('genre')::text IS NULL OR EXISTS (
        SELECT 1 FROM book_genres bg
        JOIN genres g ON g.id = bg.genre_id
        WHERE bg.book_id = b.id AND g.slug = sqlc.narg('genre')))
  AND (sqlc.narg('query')::text IS NULL
       OR b.title  ILIKE '%' || sqlc.narg('query') || '%'
       OR b.author ILIKE '%' || sqlc.narg('query') || '%')
ORDER BY
  CASE WHEN b.rating_count = 0 THEN NULL ELSE
       (b.rating_count::float8 / (b.rating_count + sqlc.arg('m')::float8)) * b.average_rating::float8
     + (sqlc.arg('m')::float8 / (b.rating_count + sqlc.arg('m')::float8)) * (
         SELECT COALESCE(
                    sum(o.average_rating * o.rating_count)::float8 / NULLIF(sum(o.rating_count), 0),
                    sqlc.arg('fallback')::float8)
         FROM books o
         WHERE o.status = 'APPROVED' AND o.rating_count > 0)
  END DESC NULLS LAST,
  b.rating_count DESC, b.created_at DESC, b.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- Aynı filtrelerle toplam sayı (sayfalama arayüzü "57 sonuçtan 1-20" diyebilsin).
-- name: CountBooks :one
SELECT count(*) FROM books b
WHERE b.status = 'APPROVED'
  AND (sqlc.narg('genre')::text IS NULL OR EXISTS (
        SELECT 1 FROM book_genres bg
        JOIN genres g ON g.id = bg.genre_id
        WHERE bg.book_id = b.id AND g.slug = sqlc.narg('genre')))
  AND (sqlc.narg('query')::text IS NULL
       OR b.title  ILIKE '%' || sqlc.narg('query') || '%'
       OR b.author ILIKE '%' || sqlc.narg('query') || '%');

-- submitted_by: kitabı kim getirdi (öneren kullanıcı ya da doğrudan ekleyen admin).
-- name: CreateBook :one
INSERT INTO books (
    title, slug, author, description, cover_url, publication_year,
    page_count, language, status, submitted_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- ADMIN tekil okuma: statüden bağımsız. Public GetApprovedBook'un ayrı kalması bilinçli —
-- görünürlük kuralı bir parametreye ("includeHidden bool") indirgenirse bir handler'ın
-- yanlış değeri geçmesi BOLA'yı geri getirir. İki sorgu, iki ayrı niyet.
-- name: GetBook :one
SELECT * FROM books WHERE id = $1;

-- Moderasyon için kilitli okuma: statü kontrolü + güncelleme arasına başka bir admin
-- giremesin (iki admin aynı anda approve/reject → ikisi de "PENDING" görüp ikisi de yazardı).
-- name: LockBookForModeration :one
SELECT * FROM books WHERE id = $1 FOR UPDATE;

-- name: SetBookStatus :one
UPDATE books
SET status           = sqlc.arg('status'),
    rejection_reason = sqlc.narg('rejection_reason'),
    moderated_by     = sqlc.arg('moderated_by'),
    moderated_at     = now(),
    updated_at       = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- Kısmi güncelleme (PATCH): gönderilmeyen alan (NULL) mevcut değeri korur.
-- Sınır: bu desenle bir alanı NULL'A ÇEKMEK mümkün değil (publication_year'ı "bilinmiyor"
-- yapmak gibi) — NULL "dokunma" anlamına geliyor. Gerekirse ayrı bir "clear" bayrağı gerekir.
-- name: UpdateBookDetails :one
UPDATE books
SET title            = COALESCE(sqlc.narg('title'), title),
    slug             = COALESCE(sqlc.narg('slug'), slug),
    author           = COALESCE(sqlc.narg('author'), author),
    description      = COALESCE(sqlc.narg('description'), description),
    cover_url        = COALESCE(sqlc.narg('cover_url'), cover_url),
    publication_year = COALESCE(sqlc.narg('publication_year'), publication_year),
    page_count       = COALESCE(sqlc.narg('page_count'), page_count),
    language         = COALESCE(sqlc.narg('language'), language),
    updated_at       = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- Admin listesi: tek statüye göre (bekleyen kuyruğu, arşiv, reddedilenler...).
-- name: ListBooksByStatus :many
SELECT * FROM books
WHERE status = sqlc.arg('status')
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountBooksByStatus :one
SELECT count(*) FROM books WHERE status = $1;