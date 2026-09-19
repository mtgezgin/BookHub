// Package config, uygulamanın ayarlarını ortam değişkenlerinden (env) okur ve
// uygulama BAŞLARKEN doğrular. Amaç: eksik/bozuk ayarı request anında değil,
// boot anında yakalamak (fail-fast).
package config

import (
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config, uygulamanın çalışması için gereken tüm ayarları bir arada tutan TİP.
// (OOP'deki "class"ın yalnızca VERİ kısmı gibi düşün — metotlar ayrı yazılır.)
type Config struct {
	Port        string // HTTP sunucusunun dinleyeceği port (ör. "8080")
	DatabaseURL string // Veritabanı bağlantı dizesi (ör. "postgres://...")
	// AutoMigrate: açılışta gömülü migration'ları uygula. Dev/compose'da kolaylık;
	// prod'da (Supabase pooler arkası) KAPALI kalmalı — migration doğrudan bağlantıyla
	// ve bilinçli bir adım olarak koşar. Bu yüzden varsayılan false.
	AutoMigrate bool

	// JWTSecret: access token'ların HMAC anahtarı. Zorunlu ve en az 32 bayt — koda ya da
	// repoya ASLA yazılmaz; sızarsa herkes istediği rolde token üretebilir.
	JWTSecret       string
	AccessTokenTTL  time.Duration // kısa: çalınan access token'ın ömrü bu kadar
	RefreshTokenTTL time.Duration // uzun: kullanıcı bu süre boyunca tekrar giriş yapmaz
	// SessionMaxAge: bir girişin (refresh token ailesinin) MUTLAK ömrü. RefreshTokenTTL kayan
	// bir penceredir — her rotasyon onu yeniden başlatır, aktif bir oturum hiç bitmezdi.
	// Tavan olmasa, çalınan token'la yarışı önce kazanan saldırgan aileyi sonsuza dek taşırdı;
	// tavanla en geç bu süre sonunda herkes (saldırgan dahil) yeniden parola girmek zorunda.
	SessionMaxAge time.Duration

	// RedisURL: OPSİYONEL. Boşsa cache kapalı (Noop) ve rate limit bellek içi — uygulama
	// Redis'siz de doğru çalışır. Doluysa açılışta PING atılır (fail-fast).
	RedisURL string

	// AuthRateLimit: login ve register için IP başına dakikada en çok istek (ayrı ayrı).
	AuthRateLimit int

	// LogFormat: "text" (dev, okunur) ya da "json" (konteyner/prod, makine-okur).
	LogFormat string

	// TrustedProxies: X-Real-IP başlığına güvenilecek reverse proxy adresleri (CIDR ya da
	// tek IP, virgülle). Boş (varsayılan) → başlık hiçbir zaman dikkate alınmaz.
	TrustedProxies []netip.Prefix

	// RatingMinVotes: ağırlıklı puandaki m ("güvenilir sayılmak için gereken oy sayısı").
	// Veri büyüdükçe artırılır; küçük başlar (bkz. ratings.DefaultMinVotes).
	RatingMinVotes int
}

// minJWTSecretLen, auth.MinSecretLen ile aynı sınır. config'i auth'a bağımlı yapmamak için
// burada tekrar ediliyor (config yaprak paket kalsın); auth da kendi içinde yeniden denetler.
const minJWTSecretLen = 32

// Load, env'den ayarları okur, doğrular ve hazır bir Config döner.
// Go'da özel "constructor" yoktur; konvansiyon: böyle bir Load()/NewXxx()
// fonksiyonu değeri kurup döndürür. (Config, error) = iki dönüş değeri.
func Load() (Config, error) {
	autoMigrate, err := getenvBool("AUTO_MIGRATE", false)
	if err != nil {
		return Config{}, fmt.Errorf("geçersiz config: %w", err)
	}
	accessTTL, err := getenvDuration("JWT_ACCESS_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, fmt.Errorf("geçersiz config: %w", err)
	}
	refreshTTL, err := getenvDuration("JWT_REFRESH_TTL", 7*24*time.Hour)
	if err != nil {
		return Config{}, fmt.Errorf("geçersiz config: %w", err)
	}
	sessionMaxAge, err := getenvDuration("JWT_SESSION_MAX_AGE", 30*24*time.Hour)
	if err != nil {
		return Config{}, fmt.Errorf("geçersiz config: %w", err)
	}
	minVotes, err := getenvInt("RATING_MIN_VOTES", 10)
	if err != nil {
		return Config{}, fmt.Errorf("geçersiz config: %w", err)
	}
	authLimit, err := getenvInt("AUTH_RATE_LIMIT_PER_MIN", 10)
	if err != nil {
		return Config{}, fmt.Errorf("geçersiz config: %w", err)
	}
	proxies, err := parsePrefixes(os.Getenv("TRUSTED_PROXIES"))
	if err != nil {
		return Config{}, fmt.Errorf("geçersiz config: %w", err)
	}

	// Config{...} = "struct literal" (JS'teki nesne literali { } gibi).
	cfg := Config{
		Port:        getenv("PORT", "8080"),     // PORT tanımlı değilse varsayılan 8080
		DatabaseURL: getenv("DATABASE_URL", ""), // DATABASE_URL yoksa boş
		AutoMigrate: autoMigrate,

		JWTSecret:       os.Getenv("JWT_SECRET"), // varsayılan YOK: zayıf bir varsayılan, hiç olmamasından kötüdür
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
		SessionMaxAge:   sessionMaxAge,
		RatingMinVotes:  minVotes,
		RedisURL:        os.Getenv("REDIS_URL"),
		AuthRateLimit:   authLimit,
		LogFormat:       getenv("LOG_FORMAT", "text"),
		TrustedProxies:  proxies,
	}

	if err := cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("geçersiz config: %w", err)
	}
	return cfg, nil
}

// validate, Config'i kontrol eder VE normalize eder.
// Baştaki (c *Config) = POINTER receiver: 'c', çağrılan ORİJİNAL Config'e işaret
// eder, dolayısıyla onu DEĞİŞTİREBİLİR. (Value receiver `(c Config)` olsaydı
// kopyayı değiştirirdi, orijinal etkilenmezdi — bu yüzden burada pointer şart.)
func (c *Config) validate() error {
	n, err := strconv.Atoi(c.Port) // Port sayıya çevrilebiliyor mu?
	if err != nil {
		return fmt.Errorf("PORT bir sayı olmalı, gelen %q", c.Port)
	}
	if n < 1 || n > 65535 {
		return fmt.Errorf("PORT 1-65535 aralığında olmalı, gelen %d", n)
	}
	// Normalize: strconv.Atoi "+8080"/"08080" gibi kanonik-olmayan biçimleri de
	// kabul eder. Kanonik hâli ("8080") geri yazarak addr=":+8080" gibi geçersiz
	// değerlerin sunucuya sızmasını önleriz. (Pointer receiver sayesinde mümkün.)
	c.Port = strconv.Itoa(n)
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL zorunlu - dev için .env.example'daki değeri kullan")
	}
	if len(c.JWTSecret) < minJWTSecretLen {
		return fmt.Errorf("JWT_SECRET zorunlu ve en az %d bayt olmalı (üret: openssl rand -base64 48)", minJWTSecretLen)
	}
	if c.AccessTokenTTL <= 0 || c.RefreshTokenTTL <= 0 {
		return fmt.Errorf("JWT_ACCESS_TTL ve JWT_REFRESH_TTL pozitif olmalı")
	}
	if c.LogFormat != "text" && c.LogFormat != "json" {
		return fmt.Errorf("LOG_FORMAT text ya da json olmalı, gelen %q", c.LogFormat)
	}
	if c.AuthRateLimit < 1 {
		return fmt.Errorf("AUTH_RATE_LIMIT_PER_MIN en az 1 olmalı, gelen %d", c.AuthRateLimit)
	}
	if c.RatingMinVotes < 0 {
		return fmt.Errorf("RATING_MIN_VOTES negatif olamaz, gelen %d", c.RatingMinVotes)
	}
	if c.AccessTokenTTL >= c.RefreshTokenTTL {
		return fmt.Errorf("JWT_ACCESS_TTL (%s), JWT_REFRESH_TTL'den (%s) kısa olmalı", c.AccessTokenTTL, c.RefreshTokenTTL)
	}
	// Tavan pencereden kısa olsaydı JWT_REFRESH_TTL yalan söylerdi: her token tavanda ölür ve
	// ayar etkisiz kalırdı. Yanlış yazılmış bir değeri (ör. 720h yerine 72h) boot'ta yakala.
	// EŞİTLİK serbest: "kayan pencere yok, her giriş tam N gün" de geçerli bir politika.
	if c.SessionMaxAge <= 0 {
		return fmt.Errorf("JWT_SESSION_MAX_AGE pozitif olmalı, gelen %s", c.SessionMaxAge)
	}
	if c.SessionMaxAge < c.RefreshTokenTTL {
		return fmt.Errorf("JWT_SESSION_MAX_AGE (%s), JWT_REFRESH_TTL'den (%s) kısa olamaz", c.SessionMaxAge, c.RefreshTokenTTL)
	}

	return nil
}

// getenv, env değişkenini okur; yoksa (boşsa) verilen varsayılanı döner.
// Küçük harfle başladığı için paket DIŞINDAN görünmez (unexported / private).
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getenvBool, env değişkenini bool olarak okur. Tanımsızsa varsayılanı döner; tanımlı
// ama geçersizse ("evet", "1x") HATA döner — sessizce varsayılana düşmek, yanlış
// yazılmış bir ayarın fark edilmeden etkisiz kalması demektir (fail-fast).
func getenvBool(key string, fallback bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s true/false olmalı, gelen %q", key, v)
	}
	return b, nil
}

// getenvDuration, env değişkenini süre olarak okur ("15m", "168h"). Tanımsızsa varsayılan;
// geçersizse hata (getenvBool ile aynı fail-fast gerekçesi).
func getenvDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s süre olmalı (ör. 15m, 168h), gelen %q", key, v)
	}
	return d, nil
}

// getenvInt, env değişkenini tam sayı olarak okur (tanımsızsa varsayılan, geçersizse hata).
func getenvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s tam sayı olmalı, gelen %q", key, v)
	}
	return n, nil
}

// parsePrefixes, "10.0.0.0/8, 172.28.0.10" gibi bir listeyi ayrıştırır; tek IP /32 (ya da
// /128) olur. Geçersiz bir girdi HATA — yanlış yazılmış bir proxy adresi sessizce yok
// sayılsaydı, NGINX arkasında tüm istemciler tek IP'ye çökerdi ve kimse fark etmezdi.
func parsePrefixes(v string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if p, err := netip.ParsePrefix(part); err == nil {
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES geçersiz girdi %q (CIDR ya da IP olmalı)", part)
		}
		out = append(out, netip.PrefixFrom(a, a.BitLen()))
	}
	return out, nil
}
