package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeAPI, BookHub API'sini taklit eder: gelen isteği kaydeder ve verilen cevabı döner.
// Gerçek API'yi değil MCP katmanını test ediyoruz — API'nin kendi testleri cmd/api'de.
type fakeAPI struct {
	status int
	body   string
	gotReq *http.Request
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.gotReq = r
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(f.status)
	_, _ = w.Write([]byte(f.body))
}

// connect, sunucuyu bellek içi taşımayla gerçek bir MCP istemcisine bağlar: stdio'nun aynısı,
// yalnız boru yerine bellek. initialize el sıkışması da gerçekten yapılır.
func connect(t *testing.T, api *fakeAPI) *mcp.ClientSession {
	t.Helper()
	ts := httptest.NewServer(api)
	t.Cleanup(ts.Close)

	base, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := newServer(newAPIClient(base)).Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func textOf(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("1 içerik bekleniyordu, %d geldi", len(res.Content))
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("metin içeriği bekleniyordu, %T geldi", res.Content[0])
	}
	return tc.Text
}

func TestToolsList(t *testing.T) {
	cs := connect(t, &fakeAPI{status: http.StatusOK, body: "{}"})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]bool{
		"search_books": true, "get_book": true, "top_rated": true,
		"list_genres": true, "book_comments": true, "rating_summary": true,
	}
	if len(res.Tools) != len(want) {
		t.Fatalf("%d araç bekleniyordu, %d geldi", len(want), len(res.Tools))
	}
	for _, tool := range res.Tools {
		if !want[tool.Name] {
			t.Errorf("beklenmeyen araç: %s", tool.Name)
		}
		// Bu fazdaki her araç salt okunur; yazan bir araç eklenirse bu test bilinçli güncellenmeli.
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s salt okunur işaretli değil", tool.Name)
		}
		if tool.Description == "" {
			t.Errorf("%s açıklamasız — model aracı neye göre seçecek?", tool.Name)
		}
	}
}

func TestToolCalls(t *testing.T) {
	tests := []struct {
		name      string
		tool      string
		args      map[string]any
		wantPath  string
		wantQuery url.Values
	}{
		{"arama süzgeçleri sorguya geçer", "search_books",
			map[string]any{"query": "dune", "genre": "bilim-kurgu", "sort": "rating", "limit": 5, "offset": 10},
			"/api/books", url.Values{"q": {"dune"}, "genre": {"bilim-kurgu"}, "sort": {"rating"}, "limit": {"5"}, "offset": {"10"}}},
		{"boş arama sorgusuz gider (API varsayılanları)", "search_books",
			map[string]any{}, "/api/books", url.Values{}},
		{"kitap ayrıntısı id'yi yola yazar", "get_book",
			map[string]any{"id": 7}, "/api/books/7", url.Values{}},
		{"en iyiler limiti", "top_rated",
			map[string]any{"limit": 3}, "/api/books/top-rated", url.Values{"limit": {"3"}}},
		{"türler", "list_genres",
			map[string]any{}, "/api/genres", url.Values{}},
		{"yorumlar sayfalanır", "book_comments",
			map[string]any{"id": 7, "limit": 2}, "/api/books/7/comments", url.Values{"limit": {"2"}}},
		{"puan özeti", "rating_summary",
			map[string]any{"id": 7}, "/api/books/7/ratings/summary", url.Values{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const body = `{"items":[{"id":7,"title":"Dune"}],"total":1}`
			api := &fakeAPI{status: http.StatusOK, body: body}
			cs := connect(t, api)

			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tt.tool, Arguments: tt.args})
			if err != nil {
				t.Fatal(err)
			}
			if res.IsError {
				t.Fatalf("araç hata döndü: %s", textOf(t, res))
			}

			if api.gotReq.Method != http.MethodGet {
				t.Errorf("yöntem = %s, GET bekleniyordu", api.gotReq.Method)
			}
			if api.gotReq.URL.Path != tt.wantPath {
				t.Errorf("yol = %s, beklenen %s", api.gotReq.URL.Path, tt.wantPath)
			}
			if got := api.gotReq.URL.Query(); got.Encode() != tt.wantQuery.Encode() {
				t.Errorf("sorgu = %q, beklenen %q", got.Encode(), tt.wantQuery.Encode())
			}
			// API'nin JSON'u değişmeden modele ulaşmalı (MCP tarafında DTO kopyası yok).
			if !json.Valid([]byte(textOf(t, res))) || !strings.Contains(textOf(t, res), `"title":"Dune"`) {
				t.Errorf("çıktı API gövdesini taşımıyor: %s", textOf(t, res))
			}
		})
	}
}

// structuredContent standartta (2025-06-18) bir JSON nesnesi: nesne cevap orada da durur, dizi
// cevap (ör. /api/genres) yalnız metin olarak gider — iki durumda da model gövdeyi aynen okur.
func TestStructuredContentOnlyForObjects(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		wantStructured bool
	}{
		{"nesne cevap structuredContent'te de durur", `{"items":[{"id":7}],"total":1}`, true},
		{"dizi cevap yalnız metin olarak gider", `[{"id":1,"name":"Bilim","slug":"bilim"}]`, false},
		{"baştaki boşluk nesneyi gizlemez", " \n{\"total\":0}", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := connect(t, &fakeAPI{status: http.StatusOK, body: tt.body})
			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_genres", Arguments: map[string]any{}})
			if err != nil {
				t.Fatal(err)
			}
			if res.IsError {
				t.Fatalf("araç hata döndü: %s", textOf(t, res))
			}
			if got := textOf(t, res); got != strings.TrimLeft(tt.body, " \n") && got != tt.body {
				t.Errorf("metin = %q, API gövdesi aynen beklenirdi: %q", got, tt.body)
			}
			if gotStructured := res.StructuredContent != nil; gotStructured != tt.wantStructured {
				t.Errorf("structuredContent var mı = %v, beklenen %v (%v)", gotStructured, tt.wantStructured, res.StructuredContent)
			}
		})
	}
}

func TestToolErrors(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantText string
	}{
		{"hata zarfının mesajı modele ulaşır", http.StatusNotFound,
			`{"error":{"code":"not_found","message":"kitap bulunamadı"}}`, "404 not_found: kitap bulunamadı"},
		{"zarfsız hata durum metnine düşer", http.StatusBadGateway, "<html>", "502: Bad Gateway"},
		{"JSON olmayan başarılı cevap reddedilir", http.StatusOK, "<html>", "geçerli JSON"},
		{"sınırı aşan cevap reddedilir", http.StatusOK, `"` + strings.Repeat("a", maxResponseSize) + `"`, "çok büyük"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := connect(t, &fakeAPI{status: tt.status, body: tt.body})
			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_book", Arguments: map[string]any{"id": 1}})
			// API hatası protokol hatası DEĞİL: istek başarılı, sonuç IsError=true.
			if err != nil {
				t.Fatalf("protokol hatası beklenmiyordu: %v", err)
			}
			if !res.IsError {
				t.Fatal("IsError bekleniyordu")
			}
			if got := textOf(t, res); !strings.Contains(got, tt.wantText) {
				t.Errorf("hata metni = %q, %q içermeli", got, tt.wantText)
			}
		})
	}
}

func TestParseAPIURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{"boşsa yerel varsayılan", "", defaultAPIURL, false},
		{"https kabul", "https://ornek.com", "https://ornek.com", false},
		{"yol öneki korunur", "http://localhost:8080/onek", "http://localhost:8080/onek", false},
		{"şemasız red", "localhost:3000", "", true},
		{"başka şema red", "ftp://ornek.com", "", true},
		{"sorgulu red", "http://localhost:3000?x=1", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := parseAPIURL(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("hata bekleniyordu, %v geldi", u)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if u.String() != tt.want {
				t.Errorf("= %s, beklenen %s", u, tt.want)
			}
		})
	}
}
