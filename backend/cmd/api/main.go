// Package main, BookHub API sunucusunun giriş noktasıdır.
package main

import (
	"context"
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

	// http.Server'ı elle kuruyoruz: timeout'lar + Shutdown() erişimi.
	// Handler artık Chi router'ı (bkz. router.go).
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      newRouter(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// [context + defer] SIGINT/SIGTERM gelince ctx iptal olur.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// [goroutine] Sunucuyu ayrı akışta çalıştır; graceful kapanışta gelen
	// http.ErrServerClosed beklenen durumdur.
	go func() {
		log.Printf("BookHub API %s adresinde dinliyor", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("sunucu başlatılamadı: %v", err)
		}
	}()

	// [channel] Sinyal gelene kadar burada bekle.
	<-ctx.Done()
	log.Println("kapanış sinyali alındı, nazikçe kapanıyor...")

	// [context + defer] Kapanışa 10sn deadline; devam eden istekleri bitir.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("graceful shutdown başarısız: %v", err)
	}
	log.Println("sunucu temiz kapandı")
}
