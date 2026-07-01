package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// writeJSON, bir değeri verilen HTTP status koduyla JSON olarak cevaba yazar.
// Tüm handler'lar cevabı bunun üzerinden yazsın ki Content-Type/encoding tek yerde olsun.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Buraya genelde sadece istemci bağlantıyı yarıda keserse düşülür.
		log.Printf("cevap JSON yazılamadı: %v", err)
	}
}

// errorBody, TÜM hataların döneceği tutarlı zarf: {"error":{"code","message"}}.
// Frontend, sabit `code` alanına göre dallanır (değişebilen `message` metnine değil).
type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// writeError, tutarlı hata zarfını verilen status + makine-okur code + insan-okur message ile yazar.
func writeError(w http.ResponseWriter, status int, code, message string) {
	var body errorBody
	body.Error.Code = code
	body.Error.Message = message
	writeJSON(w, status, body)
}
