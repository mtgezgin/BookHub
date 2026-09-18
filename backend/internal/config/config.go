// Package config, uygulamanın ayarlarını ortam değişkenlerinden (env) okur ve
// uygulama BAŞLARKEN doğrular. Amaç: eksik/bozuk ayarı request anında değil,
// boot anında yakalamak (fail-fast).
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config, uygulamanın çalışması için gereken tüm ayarları bir arada tutan TİP.
// (OOP'deki "class"ın yalnızca VERİ kısmı gibi düşün — metotlar ayrı yazılır.)
// İleride buraya DatabaseURL, RedisURL, JWTSecret gibi alanlar eklenecek.
type Config struct {
	Port        string // HTTP sunucusunun dinleyeceği port (ör. "8080")
	DatabaseURL string // Veritabanı bağlantı dizesi (ör. "postgres://...")
	// AutoMigrate: açılışta gömülü migration'ları uygula. Dev/compose'da kolaylık;
	// prod'da (Supabase pooler arkası) KAPALI kalmalı — migration doğrudan bağlantıyla
	// ve bilinçli bir adım olarak koşar. Bu yüzden varsayılan false.
	AutoMigrate bool
}

// Load, env'den ayarları okur, doğrular ve hazır bir Config döner.
// Go'da özel "constructor" yoktur; konvansiyon: böyle bir Load()/NewXxx()
// fonksiyonu değeri kurup döndürür. (Config, error) = iki dönüş değeri.
func Load() (Config, error) {
	autoMigrate, err := getenvBool("AUTO_MIGRATE", false)
	if err != nil {
		return Config{}, fmt.Errorf("geçersiz config: %w", err)
	}

	// Config{...} = "struct literal" (JS'teki nesne literali { } gibi).
	cfg := Config{
		Port:        getenv("PORT", "8080"),     // PORT tanımlı değilse varsayılan 8080
		DatabaseURL: getenv("DATABASE_URL", ""), // DATABASE_URL yoksa boş
		AutoMigrate: autoMigrate,
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
