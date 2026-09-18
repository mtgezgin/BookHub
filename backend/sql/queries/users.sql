-- users.sql — kullanıcı ve refresh token sorguları (Faz 3).

-- role kolonu BİLEREK yok: DB varsayılanı ('USER') devreye girsin. Parametreli
-- INSERT'te DEFAULT ancak kolon listede HİÇ yoksa uygulanır (bkz. books.language
-- dersi) — burada bu davranışı tam tersine, güvenlik için kullanıyoruz.
-- name: CreateUser :one
INSERT INTO users (email, display_name, password_hash)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- Rol değişikliği ayrı ve açık bir sorgu: register akışından ASLA çağrılmaz.
-- name: SetUserRole :execrows
UPDATE users SET role = $2, updated_at = now() WHERE id = $1;

-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3);

-- Token'ı TEK ATIMDA kullan: sil ve sahibini döndür. İki eşzamanlı /refresh aynı token'la
-- gelirse satırı yalnızca BİRİ silebilir; diğeri "satır yok" alır. SELECT-sonra-DELETE
-- yazsaydık ikisi de SELECT'i geçip iki yeni oturum açabilirdi (check-then-act yarışı).
-- Süresi dolmuş token da silinir (temizlik); süre kontrolü Go tarafında.
-- name: ConsumeRefreshToken :one
DELETE FROM refresh_tokens
WHERE token_hash = $1
RETURNING user_id, expires_at;

-- name: DeleteRefreshToken :exec
DELETE FROM refresh_tokens WHERE token_hash = $1;

-- Girişte kullanıcının süresi dolmuş token'larını süpür: tablo sınırsız büyümesin.
-- (Ayrı bir zamanlanmış iş yerine bu bağlamda sınırlı, ucuz bir temizlik.)
-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens WHERE user_id = $1 AND expires_at <= now();
