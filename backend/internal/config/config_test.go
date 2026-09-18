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
