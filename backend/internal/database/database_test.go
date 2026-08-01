package database

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestNew_HataYollari, New'in FAIL-FAST sözünü canlı DB'ye İHTİYAÇ DUYMADAN
// doğrular: bozuk DSN havuz kurulmadan, ulaşılamayan hedef ise Ping'de hata
// döner. Her iki yolda da çağırana nil havuz döner — yani ping başarısızken
// açık kalmış bir havuz (pool.Close() yapılmış olan) dışarı sızmaz.
// Mutlu yol (canlı havuz) entegrasyon testinde: internal/books/repository_integration_test.go.
func TestNew_HataYollari(t *testing.T) {
	tests := []struct {
		name            string
		dsn             string
		iptalliCtx      bool   // context'i çağrıdan ÖNCE iptal et
		wantErrContains string // hata metninde geçmesi beklenen bağlam
	}{
		{
			name:            "bozuk DSN → havuz yapılandırılamaz (sorguya hiç gidilmez)",
			dsn:             "postgres://kullanici:sifre@sunucu:port-degil/bookhub",
			wantErrContains: "havuz yapılandırılamadı",
		},
		{
			name:            "iptal edilmiş context → Ping fail-fast",
			dsn:             "postgres://kullanici:sifre@127.0.0.1:1/bookhub?sslmode=disable",
			iptalliCtx:      true,
			wantErrContains: "veritabanına ulaşılamadı",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.iptalliCtx {
				cancel()
			}

			basla := time.Now()
			pool, err := New(ctx, tt.dsn)
			gecen := time.Since(basla)

			if err == nil {
				if pool != nil {
					pool.Close()
				}
				t.Fatalf("hata bekleniyordu ama gelmedi (dsn=%q)", tt.dsn)
			}
			if pool != nil {
				pool.Close()
				t.Errorf("hata yolunda havuz nil dönmeliydi (yarım havuz sızıyor)")
			}
			if !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Errorf("hata metni %q içermeliydi, gelen %q", tt.wantErrContains, err.Error())
			}
			// Fail-fast: 5 sn'lik Ping timeout'unu sonuna kadar BEKLEMEDEN dönmeli.
			if gecen >= 5*time.Second {
				t.Errorf("fail-fast değil: hata %v sonra döndü", gecen)
			}
		})
	}
}
