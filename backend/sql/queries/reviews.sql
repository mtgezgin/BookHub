-- reviews.sql — kitap incelemeleri (Faz 17B). Yazımları yalnız internal/reviews servisi yapar.
-- Kullanıcının kendi incelemesine dokunan her sorgu user_id ile süzülür: başkasının incelemesi "yok"
-- görünür → 404 (IDOR yok; uçlarda kullanıcı kimliği yok, Doğrulama 13).

-- Kitap sayfasındaki herkese açık liste, en yeni önce. Gizlenenler yok (Doğrulama 15). Yazarın adı
-- users'tan (hesap silinince inceleme de silindiği için JOIN), puanı ratings'ten (LEFT JOIN: puansız
-- inceleme de olur; inceleme silmek puanı silmez, puan silmek incelemeyi silmez).
-- name: ListBookReviews :many
SELECT r.id, r.user_id, u.display_name AS author_name, rt.score AS author_score,
       r.body, r.has_spoiler, r.created_at, r.edited_at
FROM reviews r
JOIN users u ON u.id = r.user_id
LEFT JOIN ratings rt ON rt.user_id = r.user_id AND rt.book_id = r.book_id
WHERE r.book_id = sqlc.arg('book_id') AND r.hidden_at IS NULL
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountBookReviews :one
SELECT count(*) FROM reviews WHERE book_id = $1 AND hidden_at IS NULL;

-- Benim incelemem (gizlenmişse gerekçesiyle; sahibi görür).
-- name: GetMyReview :one
SELECT * FROM reviews WHERE user_id = $1 AND book_id = $2;

-- Düzenleme ve silme: satır transaction sonuna kadar kilitli. Sürüm servis tarafından karşılaştırılır;
-- tek bir "UPDATE … WHERE version = $n" 0 satır döndüğünde "inceleme yok" (404) ile "sürüm eski"
-- (409) ayrılamazdı (okuma ilerlemesindeki UpdateReading'in aynısı).
-- name: LockMyReview :one
SELECT * FROM reviews WHERE user_id = $1 AND book_id = $2 FOR UPDATE;

-- İlk yazım: yalnız yayındaki kitaba (değilse satır üretmez → pgx.ErrNoRows → 404). Aynı anda iki ilk
-- yazım gelirse ikincisi UNIQUE'e çarpar ve ON CONFLICT DO NOTHING yüzünden yine satır döndürmez;
-- servis o durumda satırı kilitleyip sürüm kuralına göre karar verir.
-- name: InsertReview :one
INSERT INTO reviews (user_id, book_id, body, has_spoiler)
SELECT sqlc.arg('user_id'), b.id, sqlc.arg('body'), sqlc.arg('has_spoiler')
FROM books b
WHERE b.id = sqlc.arg('book_id') AND b.status = 'APPROVED'
ON CONFLICT (user_id, book_id) DO NOTHING
RETURNING *;

-- Düzenleme: metin ve spoiler bayrağı; edited_at dolar, version artar. Gizleme kolonlarına dokunmaz:
-- gizlenmiş inceleme düzenlenince gizli kalır (REV-01).
-- name: UpdateReview :one
UPDATE reviews
SET body = sqlc.arg('body'), has_spoiler = sqlc.arg('has_spoiler'),
    edited_at = now(), version = version + 1
WHERE id = sqlc.arg('id')
RETURNING *;

-- Silme puanı silmez (ratings ayrı tablo). Satır yoksa 0 → 404.
-- name: DeleteMyReview :execrows
DELETE FROM reviews WHERE user_id = $1 AND book_id = $2;

-- Yönetici: gizleme ve geri açma, audit kaydıyla aynı transaction'da. Satır kilitlenir ki iki yönetici
-- aynı anda gizleyince audit'e iki kez "gizledi" yazılmasın (servis önceki durumu kilitli satırdan okur).
-- name: LockReview :one
SELECT * FROM reviews WHERE id = $1 FOR UPDATE;

-- name: HideReview :one
UPDATE reviews
SET hidden_at = now(), hidden_by = sqlc.arg('hidden_by'), hidden_reason = sqlc.arg('reason')
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: UnhideReview :one
UPDATE reviews
SET hidden_at = NULL, hidden_by = NULL, hidden_reason = ''
WHERE id = sqlc.arg('id')
RETURNING *;
