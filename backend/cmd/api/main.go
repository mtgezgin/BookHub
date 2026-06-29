// Package main, BookHub API sunucusunun giriş noktasıdır.
package main

import (
	"encoding/json"
	"log"
	"net/http"

	// İç paketimiz. Boş satır, stdlib import'larını kendi paketlerimizden
	// ayırır (goimports konvansiyonu).
	"github.com/mtgezgin/bookhub/backend/internal/config"
)

func main() {
	// Ayarları env'den yükle ve DOĞRULA. Eksik/yanlışsa burada — boot anında —
	// dur (fail-fast); request anında gizemli hata değil.
	// cfg, err := ... → config.Load'ın iki dönüş değerini iki değişkene al.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config yüklenemedi: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)

	// Port artık sabit değil; config'ten (env'den) geliyor.
	addr := ":" + cfg.Port
	log.Printf("BookHub API %s adresinde dinliyor", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("sunucu durdu: %v", err)
	}
}

// handleHealth, servisin ayakta olup olmadığını söyleyen basit bir uç nokta.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
