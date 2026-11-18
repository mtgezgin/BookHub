-- library.sql — kişisel kitaplık sorguları (Faz 15).

-- Ekle ya da durumu değiştir, TEK deyimde. Kitap yayında değilse SELECT satır üretmez,
-- hiçbir şey yazılmaz ve RETURNING boş döner → 404 (kitabın varlığı da sızmaz).
-- Arşivlenmiş bir kitabın kaydı bu yüzden güncellenemez; yalnız silinebilir.
-- name: UpsertLibraryEntry :one
INSERT INTO library_entries (user_id, book_id, status)
SELECT sqlc.arg('user_id')::bigint, b.id, sqlc.arg('status')::text
FROM books b
WHERE b.id = sqlc.arg('book_id') AND b.status = 'APPROVED'
ON CONFLICT (user_id, book_id)
DO UPDATE SET status = EXCLUDED.status, updated_at = now()
RETURNING *;

-- Silmek idempotent: kayıt yoksa da hata değil.
-- name: DeleteLibraryEntry :exec
DELETE FROM library_entries WHERE user_id = $1 AND book_id = $2;

-- Kitaplığım listesi. Filtre deseni ListBooks'un aynısı (`param IS NULL OR …`: boş filtre
-- devre dışı). Kitap yayından kalkmış olsa da kayıt listede kalır; in_catalog bunu söyler.
-- sqlc ORDER BY'ı parametre olarak alamaz (sıralama kolonu bir değer değil, sorgunun yapısı):
-- üç sıralama CASE ile seçilir, seçilmeyen CASE NULL üretir ve sırayı etkilemez. En sondaki
-- book_id eşitlikleri kırar → sayfalar arasında kitap kaybolmaz ya da tekrarlanmaz.
-- total: pencere fonksiyonu, LIMIT'ten ÖNCE süzülmüş satırları sayar → ayrı bir sayma sorgusu (ve aynı
-- WHERE'in ikinci kopyası) gerekmez. Sayfa sonun ötesindeyse satır, dolayısıyla toplam da dönmez;
-- repository o durumda ilk satırı ister.
-- r.score dönüşümsüz kalır: LEFT JOIN'in "boş olabilir" bilgisi korunur (sqlc.yaml → *int16, nil = puanım yok).
-- Arama katalogla aynı kuralla eşleşir (search_fold ve book_search; 0014_book_search.sql): Türkçe
-- harf, aksan ve kelime sırası fark etmez, son kelime önek sayılır. q tek satırlık ve MATERIALIZED:
-- arama ifadesi bir kez kurulur, kitaplıktaki her kayıt için yeniden hesaplanmaz.
-- name: ListLibrary :many
WITH q AS MATERIALIZED (
    SELECT search_tsquery(search_fold(sqlc.narg('query')::text), '') AS tsq
)
SELECT sqlc.embed(b), le.status, le.created_at AS added_at, r.score AS my_rating,
       (b.status = 'APPROVED')::boolean AS in_catalog,
       count(*) OVER () AS total
FROM library_entries le
CROSS JOIN q
JOIN book_view b ON b.id = le.book_id
LEFT JOIN ratings r ON r.user_id = le.user_id AND r.book_id = le.book_id
WHERE le.user_id = sqlc.arg('user_id')
  AND (sqlc.narg('status')::text IS NULL OR le.status = sqlc.narg('status'))
  AND (sqlc.narg('genre')::text IS NULL OR EXISTS (
        SELECT 1 FROM book_genres bg
        JOIN genres g ON g.id = bg.genre_id
        WHERE bg.book_id = b.id AND g.slug = sqlc.narg('genre')))
  AND (sqlc.narg('query')::text IS NULL OR EXISTS (
        SELECT 1 FROM book_search s
        WHERE s.book_id = le.book_id AND s.tsv @@ q.tsq))
  AND (sqlc.narg('rated')::text IS NULL OR (sqlc.narg('rated') = 'yes') = (r.user_id IS NOT NULL))
ORDER BY
  CASE WHEN sqlc.arg('sort')::text = 'title' THEN b.title COLLATE "tr-x-icu" END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'my_rating' THEN r.score END DESC NULLS LAST,
  le.created_at DESC, le.book_id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- Kartlardaki rozetler için TOPLU okuma: sayfadaki kitap kimlikleri tek sorguda (N+1 yok).
-- name: ListLibraryStatuses :many
SELECT book_id, status FROM library_entries
WHERE user_id = sqlc.arg('user_id') AND book_id = ANY(sqlc.arg('book_ids')::bigint[]);
