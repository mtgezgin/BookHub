package config

import (
	"strings"
	"testing"
	"time"
)

// testJWTSecret, testlerde geçerli kabul edilen (≥32 bayt) sahte anahtar.
var testJWTSecret = strings.Repeat("s", minJWTSecretLen)

// TestLoad, table-driven (tablo tabanlı) bir testtir: önce bir "tablo" (test
// durumlarının slice'ı) tanımlarız, sonra HEPSİNİ tek bir döngüde çalıştırırız.
// Yeni bir durum eklemek = tabloya yeni bir satır eklemek. (JS'teki test.each gibi,
// ama hiçbir framework gerekmeden, standart "testing" paketiyle.)
func TestLoad(t *testing.T) {
	// Her satır = bir senaryo: girdi (port) + beklenen sonuç (wantPort / wantErr).
	tests := []struct {
		name     string // alt-testin adı (çıktıda görünür)
		port     string // PORT env değeri ("" = boş → varsayılana düşmeli)
		dbURL    string // DATABASE_URL env değeri
		wantPort string // hata yoksa beklenen cfg.Port
		wantErr  bool   // hata bekliyor muyuz?
	}{
		{name: "PORT boş → varsayılan 8080", port: "", wantPort: "8080", dbURL: "postgres://test"},
		{name: "geçerli PORT", port: "9090", wantPort: "9090", dbURL: "postgres://test"},
		{name: "sayı olmayan PORT → hata", port: "abc", wantErr: true, dbURL: "postgres://test"},
		{name: "aralık dışı (büyük) → hata", port: "70000", wantErr: true, dbURL: "postgres://test"},
		{name: "geçersiz 0 → hata", port: "0", wantErr: true, dbURL: "postgres://test"},
		// Normalize: Atoi bunları kabul eder ama biz kanonik "8080" döndürmeliyiz.
		{name: "kanonik olmayan +8080 → normalize 8080", port: "+8080", wantPort: "8080", dbURL: "postgres://test"},
		{name: "baştaki sıfır 08080 → normalize 8080", port: "08080", wantPort: "8080", dbURL: "postgres://test"},
		{name: "boş DATABASE_URL → hata", port: "8080", dbURL: "", wantErr: true},
	}

	for _, tt := range tests {
		// t.Run her durum için AYRI bir alt-test açar: biri patlasa diğerleri devam
		// eder ve çıktıda "TestLoad/geçerli_PORT" gibi ayrı ayrı görünürler.
		t.Run(tt.name, func(t *testing.T) {
			// PORT'u bu test için ayarla; test bitince otomatik eski hâline döner.
			t.Setenv("PORT", tt.port)
			t.Setenv("DATABASE_URL", tt.dbURL)
			t.Setenv("JWT_SECRET", testJWTSecret)

			cfg, err := Load()

			if tt.wantErr {
				if err == nil {
					t.Fatalf("hata bekleniyordu ama gelmedi (PORT=%q)", tt.port)
				}
				return // hata beklenen durumda cfg'ye bakmaya gerek yok
			}

			if err != nil {
				t.Fatalf("beklenmeyen hata: %v", err)
			}
			if cfg.Port != tt.wantPort {
				t.Errorf("Port yanlış: istenen %q, gelen %q", tt.wantPort, cfg.Port)
			}
		})
	}
}

// TestLoadAutoMigrate: AUTO_MIGRATE tanımsızken KAPALI (prod güvenli varsayılan),
// geçerli bool'larda okunur, geçersiz değerde boot durur (sessizce varsayılana düşmez).
func TestLoadAutoMigrate(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    bool
		wantErr bool
	}{
		{name: "tanımsız → false (güvenli varsayılan)", value: "", want: false},
		{name: "true", value: "true", want: true},
		{name: "1 → true", value: "1", want: true},
		{name: "false", value: "false", want: false},
		{name: "geçersiz → hata", value: "evet", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://test")
			t.Setenv("JWT_SECRET", testJWTSecret)
			t.Setenv("AUTO_MIGRATE", tt.value)

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("hata bekleniyordu (AUTO_MIGRATE=%q)", tt.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("beklenmeyen hata: %v", err)
			}
			if cfg.AutoMigrate != tt.want {
				t.Errorf("AutoMigrate: istenen %v, gelen %v", tt.want, cfg.AutoMigrate)
			}
		})
	}
}

// TestLoadJWT: secret'sız ya da kısa secret'la boot DURMALI; süreler okunur ve
// access < refresh kuralı uygulanır.
func TestLoadJWT(t *testing.T) {
	tests := []struct {
		name                 string
		secret               string
		access, refresh      string
		wantAccess, wantRefr time.Duration
		wantErr              bool
	}{
		{name: "varsayılan süreler", secret: testJWTSecret, wantAccess: 15 * time.Minute, wantRefr: 7 * 24 * time.Hour},
		{name: "özel süreler", secret: testJWTSecret, access: "5m", refresh: "24h", wantAccess: 5 * time.Minute, wantRefr: 24 * time.Hour},
		{name: "secret yok → hata", secret: "", wantErr: true},
		{name: "31 baytlık secret → hata", secret: testJWTSecret[1:], wantErr: true},
		{name: "geçersiz süre → hata", secret: testJWTSecret, access: "onbes-dakika", wantErr: true},
		{name: "geçersiz refresh süresi → hata", secret: testJWTSecret, refresh: "bir-hafta", wantErr: true},
		{name: "negatif süre → hata", secret: testJWTSecret, access: "-1m", wantErr: true},
		{name: "access ≥ refresh → hata", secret: testJWTSecret, access: "2h", refresh: "1h", wantErr: true},
		// Sınır: EŞİTLİK de reddedilir (>=) — refresh, access'ten uzun yaşamalı.
		{name: "access = refresh → hata", secret: testJWTSecret, access: "1h", refresh: "1h", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://test")
			t.Setenv("JWT_SECRET", tt.secret)
			t.Setenv("JWT_ACCESS_TTL", tt.access)
			t.Setenv("JWT_REFRESH_TTL", tt.refresh)

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatal("hata bekleniyordu")
				}
				return
			}
			if err != nil {
				t.Fatalf("beklenmeyen hata: %v", err)
			}
			if cfg.AccessTokenTTL != tt.wantAccess || cfg.RefreshTokenTTL != tt.wantRefr {
				t.Errorf("süreler: istenen %s/%s, gelen %s/%s", tt.wantAccess, tt.wantRefr, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
			}
		})
	}
}

func TestLoadRatingMinVotes(t *testing.T) {
	tests := []struct {
		value   string
		want    int
		wantErr bool
	}{
		{"", 10, false},
		{"25", 25, false},
		{"0", 0, false},
		{"-1", 0, true},
		{"on", 0, true},
	}
	for _, tt := range tests {
		t.Run("RATING_MIN_VOTES="+tt.value, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://test")
			t.Setenv("JWT_SECRET", testJWTSecret)
			t.Setenv("RATING_MIN_VOTES", tt.value)
			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("hata beklentisi %v, gelen %v", tt.wantErr, err)
			}
			if err == nil && cfg.RatingMinVotes != tt.want {
				t.Errorf("istenen %d, gelen %d", tt.want, cfg.RatingMinVotes)
			}
		})
	}
}

func TestLoadAuthRateLimit(t *testing.T) {
	for _, tt := range []struct {
		value   string
		want    int
		wantErr bool
	}{{"", 10, false}, {"3", 3, false}, {"0", 0, true}, {"çok", 0, true}} {
		t.Run("AUTH_RATE_LIMIT_PER_MIN="+tt.value, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://test")
			t.Setenv("JWT_SECRET", testJWTSecret)
			t.Setenv("AUTH_RATE_LIMIT_PER_MIN", tt.value)
			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("hata beklentisi %v, gelen %v", tt.wantErr, err)
			}
			if err == nil && cfg.AuthRateLimit != tt.want {
				t.Errorf("istenen %d, gelen %d", tt.want, cfg.AuthRateLimit)
			}
		})
	}
}

func TestLoadLogFormat(t *testing.T) {
	for _, tt := range []struct {
		value, want string
		wantErr     bool
	}{{"", "text", false}, {"json", "json", false}, {"yaml", "", true}} {
		t.Run("LOG_FORMAT="+tt.value, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://test")
			t.Setenv("JWT_SECRET", testJWTSecret)
			t.Setenv("LOG_FORMAT", tt.value)
			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("hata beklentisi %v, gelen %v", tt.wantErr, err)
			}
			if err == nil && cfg.LogFormat != tt.want {
				t.Errorf("istenen %q, gelen %q", tt.want, cfg.LogFormat)
			}
		})
	}
}

func TestParsePrefixes(t *testing.T) {
	got, err := parsePrefixes(" 172.28.0.10 , 10.0.0.0/8,,::1")
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	want := []string{"172.28.0.10/32", "10.0.0.0/8", "::1/128"}
	if len(got) != len(want) {
		t.Fatalf("istenen %v, gelen %v", want, got)
	}
	for i := range want {
		if got[i].String() != want[i] {
			t.Errorf("%d: istenen %s, gelen %s", i, want[i], got[i])
		}
	}
	if _, err := parsePrefixes("nginx"); err == nil {
		t.Error("geçersiz girdi reddedilmeli")
	}
	if got, err := parsePrefixes(""); err != nil || len(got) != 0 {
		t.Errorf("boş liste: %v %v", got, err)
	}
}

func TestLoadTrustedProxies(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("JWT_SECRET", testJWTSecret)

	t.Setenv("TRUSTED_PROXIES", "172.28.0.10, 10.0.0.0/8")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[0].String() != "172.28.0.10/32" || cfg.TrustedProxies[1].String() != "10.0.0.0/8" {
		t.Errorf("TRUSTED_PROXIES ayrıştırılıp cfg'ye kopyalanmalı: %v", cfg.TrustedProxies)
	}

	t.Setenv("TRUSTED_PROXIES", "bir-proxy-degil")
	if _, err := Load(); err == nil {
		t.Error("geçersiz TRUSTED_PROXIES Load'u başarısız kılmalı")
	}
}

func TestLoadRedisURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("JWT_SECRET", testJWTSecret)

	for _, want := range []string{"redis://localhost:6380/0", ""} {
		t.Setenv("REDIS_URL", want)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("beklenmeyen hata: %v", err)
		}
		if cfg.RedisURL != want {
			t.Errorf("REDIS_URL kopyalanmalı (boş = cache kapalı): istenen %q, gelen %q", want, cfg.RedisURL)
		}
	}
}
