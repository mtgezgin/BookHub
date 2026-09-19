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

-- ---------------------------------------------------------------------------------------------
-- Parola değiştirme (Faz 13). Kilit sırası değişmezi burada bir kademe daha yukarıdan başlar:
-- users satırı → refresh_token_families → refresh_tokens. Aşağıdaki iki "kilitle ve oku"
-- sorgusu aynı satıra farklı GÜÇTE kilit alır; farkın nedeni her birinin üstünde.
-- ---------------------------------------------------------------------------------------------

-- Parola değişiminin ilk adımı: kullanıcının satırını kilitle ve hash'ini oku.
-- FOR NO KEY UPDATE, FOR UPDATE DEĞİL. NEDEN: uçuştaki bir rotasyon, ailesinin kilidini
-- tutarken halef token'ı INSERT eder; refresh_tokens.user_id'nin FK'si o an users satırında
-- FOR KEY SHARE alır. FOR UPDATE, KEY SHARE ile ÇAKIŞIR: biz users'ı tutup rotasyonun ailesini
-- beklerken rotasyon da bizim users kilidimizi beklerdi → deadlock (40P01). FOR NO KEY UPDATE
-- KEY SHARE ile çakışmaz (yalnız anahtar olmayan kolonları değiştireceğimizi söyler; password_hash
-- bir anahtar değil) ama FOR SHARE ile çakışır — girişin kontrolü (aşağıda) bizi bekler.
-- name: LockUserPasswordForChange :one
SELECT password_hash FROM users WHERE id = $1 FOR NO KEY UPDATE;

-- Girişin (StartRefreshFamily) tx içindeki kontrolü: bcrypt karşılaştırması tx DIŞINDA eski
-- hash'le yapıldı; aile açılmadan önce hash'in HÂLÂ o olduğunu kilit altında doğruluyoruz.
-- FOR SHARE: yukarıdaki FOR NO KEY UPDATE ile çakışır (parola değişimi ile giriş SERİ olur —
-- değişim önce bittiyse burada yeni hash okunur ve giriş reddedilir), ama rotasyonların
-- FOR KEY SHARE'iyle çakışmaz (girişler rotasyonları bekletmez).
-- name: LockUserPasswordForLogin :one
SELECT password_hash FROM users WHERE id = $1 FOR SHARE;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1;

-- Kullanıcının CANLI ailelerini DETERMİNİSTİK sırayla (id) kilitle. Aşağıdaki toplu UPDATE
-- satırları rastgele sırada kilitlerdi; iki işlem aynı kümeyi ters sırada kilitlerse deadlock.
-- Sonuç kümesi kullanılmaz; amaç yalnızca kilitlerin sırası.
-- name: LockUserLiveRefreshFamilies :many
SELECT id FROM refresh_token_families
WHERE user_id = $1 AND revoked_at IS NULL
ORDER BY id
FOR UPDATE;

-- Kullanıcının TÜM canlı ailelerini öldür (parola değişti: her cihaz yeniden giriş yapar).
-- :execrows → iptal edilen aile sayısı (operatör log'u için).
-- name: RevokeUserRefreshFamilies :execrows
UPDATE refresh_token_families
SET revoked_at = sqlc.arg('now')::timestamptz
WHERE user_id = sqlc.arg('user_id') AND revoked_at IS NULL;

-- Aileler kilitliyken (kilit sırası) kullanıcının tüm token'larını sil. Yukarıdaki kilit
-- beklemesi sayesinde anlık görüntü, o an commit etmiş bir rotasyonun halefini de görür.
-- name: DeleteUserRefreshTokens :exec
DELETE FROM refresh_tokens WHERE user_id = $1;

-- ---------------------------------------------------------------------------------------------
-- Refresh token AİLELERİ (Faz 13, migration 0008). Bir giriş = bir aile; rotasyonda kullanılan
-- token SİLİNMEZ, used_at ile işaretlenir ki ikinci kullanımı (reuse = çalınma belirtisi)
-- tanınabilsin. Zaman ($now) hep Go'dan parametre olarak gelir, SQL'de now() YOK: süre kararı
-- tek saatle verilir (servisin saati) ve testler saati ileri sarabilir.
--
-- KİLİT SIRASI DEĞİŞMEZİ: bir aileye dokunan HER transaction ÖNCE refresh_token_families
-- satırını, SONRA o ailenin refresh_tokens satırlarını kilitler (0007'deki "önce ebeveyn
-- satırı" deseni). Ters sıra — rotasyon önce token'ı işaretleyip sonra aileyi beklerken
-- iptal aileyi tutup token'ı beklerse — birbirini bekleyen iki işlem = deadlock (40P01).
-- ---------------------------------------------------------------------------------------------

-- name: CreateRefreshFamily :one
INSERT INTO refresh_token_families (user_id, absolute_expires_at)
VALUES ($1, $2)
RETURNING id;

-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4);

-- Token'ın ailesini KİLİTSİZ bul: family_id bir token'ın ömrü boyunca asla değişmez, bu yüzden
-- kilitsiz okumak güvenli. Kilit bir SONRAKİ adımda ailede alınır (kilit sırası).
-- name: GetRefreshTokenFamily :one
SELECT family_id FROM refresh_tokens WHERE token_hash = $1;

-- Aileyi KİLİTLE: aynı ailede rotasyon, reuse iptali ve çıkış bu satırda sıraya girer. Kilidi
-- bekleyen işlem, sahibi commit edince satırın EN GÜNCEL hâlini okur (READ COMMITTED) — yani
-- az önce iptal edilmiş bir aileyi "canlı" sanamaz.
-- name: LockRefreshFamily :one
SELECT user_id, revoked_at, absolute_expires_at
FROM refresh_token_families
WHERE id = $1
FOR UPDATE;

-- Token'ı HARCA: yalnız henüz kullanılmamışsa. 0 satır = ya yok ya da ZATEN kullanılmış
-- (hangisi olduğunu GetRefreshTokenUsedAt ayırır).
-- name: MarkRefreshTokenUsed :one
UPDATE refresh_tokens
SET used_at = sqlc.arg('now')::timestamptz
WHERE token_hash = sqlc.arg('token_hash') AND used_at IS NULL
RETURNING user_id, expires_at;

-- name: GetRefreshTokenUsedAt :one
SELECT used_at FROM refresh_tokens WHERE token_hash = $1;

-- Aileyi öldür (reuse ya da çıkış). "revoked_at IS NULL": ilk iptalin zamanı korunur.
-- name: RevokeRefreshFamily :exec
UPDATE refresh_token_families
SET revoked_at = sqlc.arg('now')::timestamptz
WHERE id = sqlc.arg('id') AND revoked_at IS NULL;

-- İptal edilen ailenin TÜM token'larını sil. Aile satırı kilitliyken koşar: bu DELETE'in anlık
-- görüntüsü, kilidi bizden önce tutan bir rotasyonun commit ettiği halef token'ı da görür.
-- name: DeleteRefreshFamilyTokens :exec
DELETE FROM refresh_tokens WHERE family_id = $1;

-- Girişteki temizlik (1/2): kullanıcının ÖLÜ ailelerini sil (token'ları CASCADE ile gider).
-- Ölü = iptal edilmiş, mutlak tavanı geçmiş ya da hiç canlı (kullanılmamış + süresi dolmamış)
-- token'ı kalmamış.
-- FOR UPDATE SKIP LOCKED: o an bir rotasyonun (ya da iptalin) kilitlediği aile ATLANIR. Kilidi
-- bekleseydik, sahibi commit ettikten sonra satırı bu deyimin BAYAT anlık görüntüsüne göre
-- silerdik — rotasyonun az önce verdiği halef token'ı da CASCADE ile giderdi (kullanıcı
-- sebepsiz oturumdan düşer). Atlanan aile bir sonraki girişte temizlenir.
-- name: DeleteDeadRefreshFamilies :exec
DELETE FROM refresh_token_families
WHERE id IN (
    SELECT f.id
    FROM refresh_token_families f
    WHERE f.user_id = sqlc.arg('user_id')
      AND (f.revoked_at IS NOT NULL
           OR f.absolute_expires_at <= sqlc.arg('now')
           OR NOT EXISTS (
               SELECT 1 FROM refresh_tokens t
               WHERE t.family_id = f.id
                 AND t.used_at IS NULL
                 AND t.expires_at > sqlc.arg('now')))
    FOR UPDATE SKIP LOCKED
);

-- Girişteki temizlik (2/2): CANLI ailelerdeki süresi dolmuş (pratikte: harcanmış) token'lar.
-- Tespit değerleri kalmadı (süresi dolmuş token zaten reddedilir); tablo sınırsız büyümesin.
-- Aile alt sorgusu kilit sırası değişmezi için: token'lara dokunmadan ÖNCE aileleri kilitler
-- ve kilitli olanı ATLAR. Böylece bu deyim, aynı ailenin token'larını başka sırayla silen bir
-- iptalle (reuse/çıkış) birbirini beklemez — kilitlenme (deadlock) kapısı açılmaz.
-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens t
WHERE t.user_id = sqlc.arg('user_id')
  AND t.expires_at <= sqlc.arg('now')
  AND t.family_id IN (
      SELECT f.id
      FROM refresh_token_families f
      WHERE f.user_id = sqlc.arg('user_id')
      FOR UPDATE SKIP LOCKED
  );
