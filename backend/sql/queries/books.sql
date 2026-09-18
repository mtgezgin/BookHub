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

-- name: CreateBook :one
INSERT INTO books (
    title, slug, author, description, cover_url, publication_year,
    page_count, language, status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING *;