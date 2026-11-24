-- reading_log.sql — kapanmış okumalar (Faz 17A-1): bitirilen ve yarım bırakılan okumaların günlüğü.
-- Durum günlükten türetilmez (kavram 3); günlüğü yalnız internal/library servisi yazar.
-- Her sorgu user_id ile süzülür: başkasının kaydı "yok" görünür → 404 (IDOR yok).

-- name: InsertReadingLog :exec
INSERT INTO reading_log (user_id, book_id, edition_id, outcome, unit, progress, total_pages, started_on, ended_on)
VALUES (sqlc.arg('user_id'), sqlc.arg('book_id'), sqlc.narg('edition_id'), sqlc.arg('outcome'),
        sqlc.narg('unit'), sqlc.narg('progress'), sqlc.narg('total_pages'), sqlc.narg('started_on'),
        sqlc.narg('ended_on'));

-- Kitap sayfasındaki geçmiş, en yeni önce. LIMIT bir emniyet: bir kitabın yüz kapanmış okuması
-- gerçekçi değil; sayfalama gerekirse ayrı karar.
-- name: ListReadingLog :many
SELECT * FROM reading_log
WHERE user_id = $1 AND book_id = $2
ORDER BY created_at DESC, id DESC
LIMIT 100;

-- "Devam et" (resume): en son kapanan okuma. Servis sonucu ABANDONED ise geri açar ve satırı
-- siler; son kayıt FINISHED ise ya da hiç yoksa (Faz 15'ten kalan yarım bırakma) baştan başlatır.
-- name: LatestReadingLog :one
SELECT * FROM reading_log
WHERE user_id = $1 AND book_id = $2
ORDER BY created_at DESC, id DESC
LIMIT 1
FOR UPDATE;

-- Günlük kaydı hangi kitabın? Servis önce bunu kilitsiz okur, sonra o kitabın raf kilidini alır,
-- en son satırı kilitler: komutlarla AYNI kilit sırası (raf → günlük), yoksa "Devam et" ile
-- "geçmişten sil" birbirini bekleyip kilitlenebilirdi (deadlock).
-- name: GetReadingLog :one
SELECT * FROM reading_log WHERE id = $1 AND user_id = $2;

-- Tarih düzeltmesi: servis kilitli satırın üstüne yalnız gönderilen alanları uygular.
-- name: LockReadingLog :one
SELECT * FROM reading_log WHERE id = $1 AND user_id = $2 FOR UPDATE;

-- Bitiş başlangıçtan önceyse tablonun CHECK'i reddeder (servis önce kendisi 400 der).
-- name: UpdateReadingLogDates :one
UPDATE reading_log
SET started_on = sqlc.narg('started_on'), ended_on = sqlc.narg('ended_on')
WHERE id = sqlc.arg('id') AND user_id = sqlc.arg('user_id')
RETURNING *;

-- 0 satır = yok ya da başkasının → 404.
-- name: DeleteReadingLog :execrows
DELETE FROM reading_log WHERE id = $1 AND user_id = $2;
