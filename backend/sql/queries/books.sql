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

-- Ana liste: sadece onaylılar, en yeniden eskiye. Bilerek parametresiz —
-- migration'daki partial index'in ((created_at DESC) WHERE status='APPROVED')
-- birebir karşılığı. (Admin'in TÜM statüleri listelemesi ileride ayrı sorgu olur.)
-- name: ListBooks :many
SELECT * FROM books
WHERE status = 'APPROVED'
ORDER BY created_at DESC;

-- name: CreateBook :one
INSERT INTO books (
    title, slug, author, description, cover_url, publication_year,
    page_count, language, status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING *;