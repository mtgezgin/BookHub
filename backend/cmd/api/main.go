// Package main, BookHub API sunucusunun giriş noktasıdır.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/config"
)

func main() {
	// Ayarları env'den yükle ve doğrula (fail-fast).
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config yüklenemedi: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)

	// http.Server'ı ELLE kuruyoruz (ListenAndServe kısayolu yerine), çünkü:
	//  - timeout'ları ayarlamak,
	//  - Shutdown() metoduna erişmek istiyoruz.
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,  // bir isteğin tamamını okumak için üst sınır
		WriteTimeout: 10 * time.Second, // cevabı yazmak için üst sınır
	}

	// [context + defer] SIGINT (Ctrl-C) veya SIGTERM (Docker/K8s "dur") gelince
	// ctx İPTAL olur. stop(), iş bitince sinyal aboneliğini geri bırakır.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// [goroutine] Sunucuyu AYRI bir akışta çalıştır ki main bloklanmasın ve
	// aşağıda sinyali bekleyebilelim. ListenAndServe normalde geri dönmez;
	// graceful kapanışta http.ErrServerClosed döner — bu BEKLENEN bir durumdur,
	// o yüzden onu hata saymıyoruz.
	go func() {
		log.Printf("BookHub API %s adresinde dinliyor", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("sunucu başlatılamadı: %v", err)
		}
	}()

	// [channel] ctx.Done() bir KANALDIR; sinyal gelene kadar TAM BURADA bekleriz.
	<-ctx.Done()
	log.Println("kapanış sinyali alındı, nazikçe kapanıyor...")

	// [context + defer] Kapanışa bir DEADLINE ver: devam eden istekler en fazla
	// 10 sn'de bitsin; bitmezse yine de çık (sonsuza kadar bekleme).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown: yeni bağlantı almayı KES, devam edenleri (deadline'a kadar) bitir,
	// sonra dön. Kaba Ctrl-C'nin aksine açık istekler yarıda kesilmez.
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("graceful shutdown başarısız: %v", err)
	}
	log.Println("sunucu temiz kapandı")
}

// handleHealth, servisin ayakta olup olmadığını söyleyen basit bir uç nokta.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
