// Command mcp, BookHub kataloğunu bir MCP (Model Context Protocol) sunucusu olarak açar.
//
// MCP NEDİR, HANGİ PROBLEMİ ÇÖZER? Bir dil modeli (Claude gibi) kendi başına BookHub'ın
// veritabanını göremez. Her yapay zekâ istemcisine ayrı bir entegrasyon yazmak yerine MCP tek
// bir sözleşme tanımlar: sunucu "şu araçlarım var, şu girdileri alır" der (tools/list), istemci
// modelin seçtiği aracı çağırır (tools/call). REST'in tarayıcı için yaptığını MCP model için
// yapar — fark, araçların kendini TANITMASI: ad, açıklama ve girdi şeması modelin okuyacağı
// dokümandır.
//
// NEDEN API'Yİ ÇAĞIRIYOR, DB'Yİ DEĞİL? Bu sunucu REST API'nin bir istemcisidir, tıpkı
// frontend gibi. Böylece görünürlük kuralı (yalnız APPROVED), rate limit ve doğrulama tek
// yerde (API'de) kalır; bu binary'nin DATABASE_URL'e, yani hiçbir sırra ihtiyacı yoktur.
//
// Taşıma: stdio. İstemci (ör. Claude Code) bu programı alt süreç olarak başlatır ve
// stdin/stdout üzerinden JSON-RPC konuşur. Bu yüzden stdout PROTOKOLE ayrılmıştır — tek bir
// fmt.Println bile akışı bozar; loglar stderr'e gider.
//
// Kullanım:
//
//	BOOKHUB_API_URL=http://localhost:3000 go run ./cmd/mcp
//	claude mcp add bookhub -e BOOKHUB_API_URL=http://localhost:3000 -- go run ./cmd/mcp
//
// Bu fazda yalnız OKUYAN araçlar var: giriş gerektiren (puan verme, kitaplık) ve admin
// araçları bilerek yok — bir modele yazma yetkisi vermek ayrı bir karardır.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// defaultAPIURL, `make app-up`'taki NGINX: /api oradan API'ye geçer (canlıdaki yolun aynısı).
const defaultAPIURL = "http://localhost:3000"

// version, istemciye initialize'da bildirilen sunucu sürümü.
const version = "0.1.0"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(ctx, os.Getenv, &mcp.StdioTransport{}); err != nil {
		logger.Error("mcp sunucusu durdu", "err", err)
		stop()
		os.Exit(1)
	}
}

// run, main'in test edilebilir hâli: env ve taşıma dışarıdan gelir (testte bellek içi taşıma).
func run(ctx context.Context, getenv func(string) string, t mcp.Transport) error {
	base, err := parseAPIURL(getenv("BOOKHUB_API_URL"))
	if err != nil {
		return err
	}
	return newServer(newAPIClient(base)).Run(ctx, t)
}

// parseAPIURL, açılışta doğrular (fail fast, GO_CONVENTIONS §5): yanlış adres ilk araç
// çağrısında değil, sunucu başlarken hata versin. Boşsa yerel varsayılan kullanılır.
func parseAPIURL(raw string) (*url.URL, error) {
	if raw == "" {
		raw = defaultAPIURL
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("BOOKHUB_API_URL geçersiz: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("BOOKHUB_API_URL http(s)://host[:port] biçiminde olmalı")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("BOOKHUB_API_URL sorgu ya da fragment içeremez")
	}
	return u, nil
}

func newServer(api *apiClient) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "bookhub", Title: "BookHub", Version: version}, nil)
	registerTools(s, api)
	return s
}
