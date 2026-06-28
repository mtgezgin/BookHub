// Package main, BookHub API sunucusunun giriş noktasıdır.
// Bir Go programı çalıştırılabilir olacaksa paketi "main" olmalı ve bir
// main() fonksiyonu bulunmalıdır — program buradan başlar.
package main

import (
	"encoding/json" // struct/map -> JSON çevirimi
	"log"           // basit loglama
	"net/http"      // HTTP sunucusu, standart kütüphaneden
)

func main() {
	// ServeMux = "router". Gelen isteğin yoluna (path) bakıp doğru
	// fonksiyona yönlendirir.
	mux := http.NewServeMux()

	// "GET /health" deseni Go 1.22+ ile gelir: method + path birlikte.
	// Yani SADECE GET isteği ve SADECE /health yolu handleHealth'e gider.
	mux.HandleFunc("GET /health", handleHealth)

	addr := ":8080" // tüm arayüzlerde 8080 portu
	log.Printf("BookHub API %s adresinde dinliyor", addr)

	// Sunucuyu başlat ve istek beklemeye geç. ListenAndServe normalde
	// "asla geri dönmez"; geri dönerse bir hata olmuştur (ör. port dolu),
	// o yüzden log.Fatal ile programı durduruyoruz.
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("sunucu durdu: %v", err)
	}
}

// handleHealth, servisin ayakta olup olmadığını söyleyen basit bir uç nokta.
// Her HTTP handler aynı imzaya sahiptir:
//   - w: cevabı YAZDIĞIMIZ yer (ResponseWriter)
//   - r: gelen isteğin bilgileri (Request)
func handleHealth(w http.ResponseWriter, r *http.Request) {
	// Cevabın JSON olduğunu istemciye söyle.
	w.Header().Set("Content-Type", "application/json")
	// HTTP durum kodu 200 (OK).
	w.WriteHeader(http.StatusOK)
	// {"status":"ok"} JSON'unu cevap gövdesine yaz.
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
