package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Sınırlar: model bir aracı döngüde çağırabilir; yanıt vermeyen ya da devasa yanıt dönen bir
// API, istemciyi kilitlememeli. Liste uçları en fazla 100 kitap döndürür — 1 MiB rahat yeter.
const (
	requestTimeout  = 10 * time.Second
	maxResponseSize = 1 << 20
)

// apiClient, BookHub REST API'sinin İNCE istemcisi: yalnız GET, yalnız herkese açık uçlar.
type apiClient struct {
	base *url.URL
	http *http.Client
}

func newAPIClient(base *url.URL) *apiClient {
	return &apiClient{base: base, http: &http.Client{Timeout: requestTimeout}}
}

// apiError, API'nin hata zarfı: {"error":{"code","message"}} (CLAUDE.md — her yerde aynı).
type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// get, path'e GET atar ve gövdeyi DOKUNMADAN döner. json.RawMessage: cevabı Go struct'ına
// çözüp yeniden serileştirmiyoruz — API'nin DTO'su zaten sözleşmedir, burada kopyası olursa
// API'ye eklenen bir alan MCP'de sessizce kaybolurdu.
func (c *apiClient) get(ctx context.Context, path string, q url.Values) (json.RawMessage, error) {
	u := c.base.JoinPath(path)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("istek kurulamadı: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		// Ham hata modele gider; adres yerel olduğu için sır taşımaz ama "API kapalı mı?"
		// ipucunu verir (en sık sebep: make app-up çalışmıyor).
		return nil, fmt.Errorf("BookHub API'ye ulaşılamadı (%s): %w", c.base.Redacted(), err)
	}
	// Okuma bittiyse kapatma hatası cevabı değiştirmez; bilerek yok sayılır (cmd/api healthcheck ile aynı desen).
	defer func() { _ = resp.Body.Close() }()

	// +1: sınırı aşan gövdeyi "tam sığdı" sanmamak için bir bayt fazla oku.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("yanıt okunamadı: %w", err)
	}
	if len(body) > maxResponseSize {
		return nil, errors.New("yanıt çok büyük")
	}

	if resp.StatusCode >= 400 {
		return nil, toAPIError(resp.StatusCode, body)
	}
	if !json.Valid(body) {
		return nil, errors.New("API geçerli JSON döndürmedi")
	}
	return body, nil
}

// toAPIError, hata zarfındaki mesajı modele aktarır: "kitap bulunamadı" ya da "limit 1-100
// arasında" gibi bir mesajı okuyan model, girdisini düzeltip yeniden deneyebilir.
func toAPIError(status int, body []byte) error {
	var e apiError
	if err := json.Unmarshal(body, &e); err == nil && e.Error.Message != "" {
		return fmt.Errorf("BookHub API %d %s: %s", status, e.Error.Code, e.Error.Message)
	}
	return fmt.Errorf("BookHub API %d: %s", status, strings.TrimSpace(http.StatusText(status)))
}
