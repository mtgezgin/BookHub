-- library.sql — kişisel kitaplık sorguları (Faz 15).

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
-- Okuma alanları (Faz 17) kartlardaki ilerleme için; anlamları yalnız status = 'READING' iken var
-- (0017'ye kadar başka durumda bayat kalabilirler, Go tarafı bakmaz).
SELECT sqlc.embed(b), le.status, le.created_at AS added_at, r.score AS my_rating,
       le.unit, le.progress, le.total_pages,
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

-- ── Raf (Faz 17A-1): okuma takibinin durum makinesi (internal/library servisi) ──────────────────
-- Her komut tek transaction: önce LockShelf satırı kilitler (yoksa "kitaplıkta değil"), servis yeni
-- durumu hesaplar, SaveShelf yazar. Okuma alanlarının anlamı yalnız status = 'READING' iken var.

-- Kitap sayfası için okuma (kilitsiz).
-- name: GetShelf :one
SELECT * FROM library_entries WHERE user_id = $1 AND book_id = $2;

-- Aynı satır, transaction sonuna kadar kilitli: aynı kayda gelen ikinci komut ilki bitene kadar
-- bekler, sonra güncel satırı görür (çift tık iki günlük satırı yazamaz).
-- name: LockShelf :one
SELECT * FROM library_entries WHERE user_id = $1 AND book_id = $2 FOR UPDATE;

-- Rafın TEK yazma yolu: durum ve okuma alanları birlikte, version her yazımda artar. Kitap yayında
-- değilse SELECT satır üretmez, hiçbir şey yazılmaz → pgx.ErrNoRows → 404 (Faz 15'in kuralı; o
-- fazın UpsertLibraryEntry'si 17A-1'de kalktı, eski PUT da buradan geçer). Satır yoksa (iki komut
-- aynı anda "kitaplıkta değil" gördüyse) ikinci INSERT çakışır ve ilk transaction'ın bitmesini
-- bekleyip güncellemeye döner.
-- name: SaveShelf :one
INSERT INTO library_entries (user_id, book_id, status, edition_id, unit, progress, total_pages, started_on)
SELECT sqlc.arg('user_id')::bigint, b.id, sqlc.arg('status')::text, sqlc.narg('edition_id')::bigint,
       sqlc.narg('unit')::text, sqlc.narg('progress')::integer, sqlc.narg('total_pages')::integer,
       sqlc.narg('started_on')::date
FROM books b
WHERE b.id = sqlc.arg('book_id') AND b.status = 'APPROVED'
ON CONFLICT (user_id, book_id)
DO UPDATE SET status      = EXCLUDED.status,
              edition_id  = EXCLUDED.edition_id,
              unit        = EXCLUDED.unit,
              progress    = EXCLUDED.progress,
              total_pages = EXCLUDED.total_pages,
              started_on  = EXCLUDED.started_on,
              version     = library_entries.version + 1,
              updated_at  = now()
RETURNING *;

-- Okumanın baskısı ve sayfa toplamının anlık görüntüsü (kavram 7). edition_id verilmişse o kitabın o
-- baskısı (başka kitabın baskısı satır üretmez → servis 400 der), verilmemişse varsayılan baskı
-- (varsayılanı olmayan kitapta satır yok → baskısız, toplamı bilinmeyen okuma).
-- name: GetReadingEdition :one
SELECT id, page_count FROM editions
WHERE book_id = sqlc.arg('book_id')
  AND CASE WHEN sqlc.narg('edition_id')::bigint IS NULL THEN is_primary
           ELSE id = sqlc.narg('edition_id') END;

-- Komutlar yalnız yayındaki kitapta çalışır (Faz 15 kuralı); "kitaplıktan çıkar" hariç. Hiçbir şey
-- yazmayan bir komut (ör. ikinci "Bitirdim") da yayında olmayan kitapta 404 dönsün diye ayrı sorgu:
-- SaveShelf'in kendi denetimi yalnız yazan komutları kapsar.
-- name: BookIsApproved :one
SELECT EXISTS (SELECT 1 FROM books WHERE id = $1 AND status = 'APPROVED');

-- Aynı (kullanıcı, kitap) çiftine gelen komutları SIRAYA sokar; transaction bitince kendiliğinden
-- bırakılır. NEDEN FOR UPDATE yetmiyor: kitap kitaplıkta yokken kilitlenecek satır da yoktur; iki
-- eşzamanlı "Zaten okudum" ikisi de "yok" görüp iki günlük satırı yazardı. Danışma kilidi (advisory
-- lock) bir satıra değil bir SAYIYA konur, satır olmadan da çalışır. Sayı anahtarın özetidir; iki farklı
-- çiftin özeti çakışırsa yalnız birbirini bekler, yanlış sonuç üretmez.
-- name: LockShelfKey :exec
SELECT pg_advisory_xact_lock(hashtextextended(
    'library_entries:' || sqlc.arg('user_id')::bigint || ':' || sqlc.arg('book_id')::bigint, 0));
