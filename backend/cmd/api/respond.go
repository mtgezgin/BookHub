package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// writeJSON, bir değeri verilen HTTP status koduyla JSON olarak cevaba yazar.
// Tüm handler'lar cevabı bunun üzerinden yazsın ki Content-Type/encoding tek yerde olsun.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Buraya genelde sadece istemci bağlantıyı yarıda keserse düşülür.
		slog.Warn("cevap JSON yazılamadı", "err", err, "request_id", w.Header().Get(requestIDHeader))
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

// internalErrorMessage, 500 cevaplarının TEK metni. Detay asla istemciye gitmez;
// operatör için log'a yazılır (bkz. writeInternalError).
const internalErrorMessage = "beklenmeyen bir hata oluştu"

// writeInternalError, beklenmeyen bir hatayı loglar ve istemciye generic 500 döner.
// Her handler'daki "log + generic mesaj" ikilisini tek yere topluyor: kural tek
// yerde yaşarsa bir handler'ın unutup detay sızdırma ihtimali ortadan kalkar.
//
// request_id: requestLogger her cevaba X-Request-Id başlığını koyar; burada onu cevap
// başlığından okuyoruz. Böylece imzaya *http.Request eklemeden (30+ çağrı yeri) hata satırı,
// aynı isteğin erişim log satırıyla eşleşir — "hangi istek patladı?" sorusu cevaplanır.
func writeInternalError(w http.ResponseWriter, op string, err error) {
	slog.Error("istek başarısız", "op", op, "err", err, "request_id", w.Header().Get(requestIDHeader))
	writeError(w, http.StatusInternalServerError, "internal", internalErrorMessage)
}

// requestError, istemcinin DÜZELTEBİLECEĞİ bir istek hatası: HTTP kodu + makine-okur
// kod + insan-okur mesaj. message istemciye gösterilir; cause ise YALNIZCA log içindir.
//
// Neden tipli bir hata: readJSON düz `error` dönseydi çağıran "bu boyut hatası mı
// (413), sözdizimi mi (400), okuma arızası mı?" ayrımını yapamazdı — hatayı
// düzleştirmek, çağıranın doğru HTTP kodunu seçme yeteneğini elinden alır.
type requestError struct {
	status  int
	code    string
	message string
	cause   error // log için; istemciye ASLA yazılmaz
}

func (e *requestError) Error() string { return e.message }
func (e *requestError) Unwrap() error { return e.cause }

func badRequest(code, message string) *requestError {
	return &requestError{status: http.StatusBadRequest, code: code, message: message}
}

// writeRequestError, readJSON/doğrulama hatalarını cevaba çevirir. requestError
// değilse bu bir programlama hatasıdır → 500 (detay log'a).
func writeRequestError(w http.ResponseWriter, err error) {
	var re *requestError
	if !errors.As(err, &re) {
		writeInternalError(w, "istek işleme", err)
		return
	}
	if re.cause != nil {
		slog.Warn("istek hatası", "code", re.code, "err", re.cause, "request_id", w.Header().Get(requestIDHeader))
	}
	writeError(w, re.status, re.code, re.message)
}

// maxRequestBody, kabul ettiğimiz en büyük istek gövdesi.
// NEDEN SINIR: sınırsız gövde bedava bir DoS'tur — tek bir 10 GB'lık POST
// sunucunun belleğini şişirip süreci öldürebilir. Bu API'nin hiçbir ucu 1 MiB'a yaklaşmaz.
const maxRequestBody = 1 << 20 // 1 MiB

// readJSON, istek gövdesini JSON olarak okuyup v'ye doldurur — writeJSON'ın ayna eşi.
// Dönen hata her zaman *requestError'dır; handler'lar writeRequestError ile yazar.
//
// Üç savunma katmanı:
//  1. MaxBytesReader → boyut sınırı (aşılırsa 413).
//  2. DisallowUnknownFields → tanımadığımız alan gelirse 400. Hem istemcinin
//     typo'sunu ("titel") sessizce yutmaz, hem de MASS-ASSIGNMENT'a karşı ikinci
//     hattır (ilk hat: request struct'ında o alanın hiç olmaması).
//  3. Tek JSON değeri → gövdede `{...}{...}` varsa reddet.
//
// NOT (imza neden w alıyor): MaxBytesReader sınır aşılınca bağlantıyı da düzgün
// kapatabilmek için ResponseWriter'a ihtiyaç duyar.
func readJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(v); err != nil {
		return decodeError(err)
	}

	// İkinci okuma io.EOF vermeli: gövdede TEK bir JSON değeri var demektir.
	switch err := dec.Decode(&struct{}{}); {
	case errors.Is(err, io.EOF):
		return nil
	case isMaxBytes(err):
		return decodeError(err)
	default:
		return badRequest("invalid_json", "gövde tek bir JSON nesnesi içermeli")
	}
}

// requestBody, okunduktan sonra temizlenip (normalize) doğrulanan istek gövdeleri.
type requestBody interface {
	normalize()
	validate() error
}

// readBody, yazım uçlarının ortak gövde adımı: oku (readJSON) → temizle → doğrula. Hata varsa
// cevabı yazar ve ok=false döner. Tip parametresi T gövdenin kendisi; PT onun işaretçisi, çünkü
// normalize alanları DEĞİŞTİRİR (pointer receiver). Kitap ve baskı uçları aynı adımı kopyalamasın.
func readBody[T any, PT interface {
	*T
	requestBody
}](w http.ResponseWriter, r *http.Request) (T, bool) {
	var req T
	if err := readJSON(w, r, &req); err != nil {
		writeRequestError(w, err)
		return req, false
	}
	p := PT(&req)
	p.normalize()
	if err := p.validate(); err != nil {
		writeRequestError(w, err)
		return req, false
	}
	return req, true
}

func isMaxBytes(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

// decodeError, encoding/json'ın teknik hatalarını istemcinin düzeltebileceği
// mesajlara çevirir. Ham hata metni ASLA istemciye gitmez: tanıyamadığımız hata
// (ör. gövde okunurken ağ zaman aşımı — metni sunucunun iç IP:port'unu içerir)
// generic bir mesaja dönüşür, asıl hata cause olarak log'a düşer.
func decodeError(err error) *requestError {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	var maxErr *http.MaxBytesError

	switch {
	case errors.As(err, &syntaxErr):
		return badRequest("invalid_json", fmt.Sprintf("gövde %d. baytta bozuk JSON içeriyor", syntaxErr.Offset))
	case errors.Is(err, io.ErrUnexpectedEOF):
		return badRequest("invalid_json", "gövde eksik/yarım JSON içeriyor")
	case errors.As(err, &typeErr):
		if typeErr.Field != "" {
			return badRequest("invalid_json", fmt.Sprintf("%q alanı yanlış tipte", typeErr.Field))
		}
		return badRequest("invalid_json", "gövde beklenen JSON tipinde değil")
	// Bilinmeyen alan için stdlib TİPLİ bir hata sunmuyor; yalnızca bu metni üretiyor.
	// Kırılgan ama standart çözüm.
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		field := strings.TrimPrefix(err.Error(), "json: unknown field ")
		return badRequest("invalid_json", "bilinmeyen alan: "+field)
	case errors.Is(err, io.EOF):
		return badRequest("invalid_json", "gövde boş olamaz")
	case errors.As(err, &maxErr):
		// 400 değil 413: "gönderdiğin şey bozuk" değil "çok büyük" — istemci farklı düzeltir.
		return &requestError{
			status:  http.StatusRequestEntityTooLarge,
			code:    "body_too_large",
			message: fmt.Sprintf("gövde çok büyük (en fazla %d bayt)", maxErr.Limit),
		}
	default:
		// Tanımadığımız hata = çoğunlukla taşıma katmanı (timeout, reset, bozuk chunk).
		// Metni sunucu içi bilgi taşıyabilir → istemciye generic mesaj, detay log'a.
		return &requestError{
			status:  http.StatusBadRequest,
			code:    "invalid_request",
			message: "istek gövdesi okunamadı",
			cause:   err,
		}
	}
}
