package config

import "testing"

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
