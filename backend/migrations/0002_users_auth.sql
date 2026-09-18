-- 0002_users_auth.sql — kullanıcılar ve refresh token'lar (Faz 3).
--
-- Şema evrimi burada somutlaşıyor: 0001'e DOKUNMUYORUZ. Uygulanmış bir migration'ı
-- düzenlemek, onu zaten çalıştırmış her ortamı (senin laptop'un, CI, prod) sessizce
-- ayrıştırır. Değişiklik = YENİ dosya.

-- +goose Up

CREATE TABLE users (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- E-posta KÜÇÜK HARFLE saklanır ve DB bunu zorlar (CHECK): "Ali@x.com" ile "ali@x.com"
    -- iki ayrı hesap olamasın. Normalizasyon uygulamada yapılır; CHECK, onu unutan bir
    -- kod yolunun (ör. ileride bir import betiği) sessizce ikiz hesap açmasını engeller.
    email         text NOT NULL UNIQUE CHECK (email = lower(email)),
    display_name  text NOT NULL,
    -- Parolanın KENDİSİ asla saklanmaz; bcrypt çıktısı saklanır (algoritma + cost + salt
    -- + hash tek string'de: "$2a$12$..."). DB sızsa bile parola geri çevrilemez.
    password_hash text NOT NULL,
    -- books.status'un tersine burada DEFAULT VAR, bilerek: en az yetkili rol (USER)
    -- güvenli varsayılandır. CreateUser sorgusu role kolonunu HİÇ yazmaz → yeni hesap
    -- her zaman USER doğar; ADMIN'lik yalnızca ayrı, açık bir UPDATE ile verilir.
    -- (status'ta DEFAULT olsaydı yanlış varsayılan 'APPROVED' = yetkisiz yayın olurdu;
    -- burada yanlış varsayılan 'ADMIN' olurdu. Güvenli varsayılan = en dar yetki.)
    role          text NOT NULL DEFAULT 'USER' CHECK (role IN ('USER', 'ADMIN')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- Refresh token'lar: stateless JWT iptal edilemez (imzası geçerli olduğu sürece geçerli).
-- "Çıkış yap" ve "oturumu uzat" ancak SUNUCUDA tutulan bir kayıtla mümkün — o kayıt bu tablo.
CREATE TABLE refresh_tokens (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- Token'ın KENDİSİ değil SHA-256 özeti saklanır (parolada bcrypt'in yaptığının hafif
    -- hâli): DB yedeği sızsa bile içindeki değerlerle oturum çalınamaz. Token 256 bit
    -- rastgele olduğu için yavaş hash'e (bcrypt) gerek yok — tahmin edilemez zaten.
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- FK'ye index (Postgres otomatik yapmaz): "bu kullanıcının süresi dolmuş token'larını sil"
-- ve kullanıcı silinince CASCADE bu kolonla arar.
CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens (user_id);

-- +goose Down

DROP TABLE refresh_tokens;
DROP TABLE users;
