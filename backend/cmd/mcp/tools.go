package main

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Girdi struct'ları SDK tarafından JSON Schema'ya çevrilir ve tools/list'te modele gösterilir:
// `jsonschema` etiketi alanın AÇIKLAMASIDIR. omitempty olmayan alan şemada zorunlu sayılır.
// Doğrulamayı (limit aralığı, slug biçimi) burada tekrarlamıyoruz — API yapar ve hata
// mesajı modele geri döner.

type searchBooksInput struct {
	Query  string `json:"query,omitempty" jsonschema:"başlıkta ya da yazarda aranacak metin"`
	Genre  string `json:"genre,omitempty" jsonschema:"tür slug'ı, ör. bilim-kurgu (list_genres ile öğrenilir)"`
	Sort   string `json:"sort,omitempty" jsonschema:"newest (varsayılan) ya da rating (ağırlıklı puan)"`
	Limit  int    `json:"limit,omitempty" jsonschema:"sayfa boyu, 1-100 (varsayılan 20)"`
	Offset int    `json:"offset,omitempty" jsonschema:"atlanacak kayıt sayısı (sayfalama)"`
}

type bookIDInput struct {
	ID int64 `json:"id" jsonschema:"kitabın sayısal id'si (search_books sonucundaki id)"`
}

type commentsInput struct {
	ID     int64 `json:"id" jsonschema:"kitabın sayısal id'si"`
	Limit  int   `json:"limit,omitempty" jsonschema:"sayfa boyu, 1-100 (varsayılan 20)"`
	Offset int   `json:"offset,omitempty" jsonschema:"atlanacak yorum sayısı"`
}

type topRatedInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"kaç kitap, 1-50 (varsayılan 10)"`
}

// readOnly: istemciye "bu araç hiçbir şeyi değiştirmez" ipucu. İstemci buna göre onay
// sormadan çalıştırmayı seçebilir; yazan araçlar geldiğinde bu ipucu onlarda OLMAYACAK.
var readOnly = &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}

func registerTools(s *mcp.Server, api *apiClient) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_books",
		Description: "BookHub kataloğunda onaylı kitapları arar ve sayfalı listeler; türe göre süzer, en yeniye ya da ağırlıklı puana göre sıralar.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in searchBooksInput) (*mcp.CallToolResult, any, error) {
		q := url.Values{}
		setIf(q, "q", in.Query)
		setIf(q, "genre", in.Genre)
		setIf(q, "sort", in.Sort)
		setPage(q, in.Limit, in.Offset)
		return call(ctx, api, "/api/books", q)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_book",
		Description: "Tek bir kitabın ayrıntısını döner: yazar, açıklama, yıl, sayfa, türler, ortalama puan.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in bookIDInput) (*mcp.CallToolResult, any, error) {
		return call(ctx, api, bookPath(in.ID), nil)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "top_rated",
		// Açıklama modele "neden basit ortalama değil" bilgisini de verir: kullanıcıya sıralamayı
		// açıklarken yanlış bir şey söylemesin.
		Description: "En iyi kitapları ağırlıklı (Bayes) puana göre listeler; az oy alan kitaplar genel ortalamaya çekilir. Yanıtta m (asgari oy) ve c (genel ortalama) da var.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in topRatedInput) (*mcp.CallToolResult, any, error) {
		q := url.Values{}
		setPage(q, in.Limit, 0)
		return call(ctx, api, "/api/books/top-rated", q)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_genres",
		Description: "Tüm türleri (ad ve slug) listeler; search_books'un genre süzgecinde slug kullanılır.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		return call(ctx, api, "/api/genres", nil)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "book_comments",
		Description: "Bir kitabın yorumlarını sayfalı listeler.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in commentsInput) (*mcp.CallToolResult, any, error) {
		q := url.Values{}
		setPage(q, in.Limit, in.Offset)
		return call(ctx, api, bookPath(in.ID)+"/comments", q)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "rating_summary",
		Description: "Bir kitabın puan özetini döner: ortalama, oy sayısı ve 1-10 arası her puanın oy dağılımı.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in bookIDInput) (*mcp.CallToolResult, any, error) {
		return call(ctx, api, bookPath(in.ID)+"/ratings/summary", nil)
	})
}

// call, API cevabını aracın çıktısı yapar. Out = any olduğu için SDK çıktı şeması üretmez ve
// ham JSON'u hem structuredContent'e hem metin içeriğine koyar. Dönen error ise protokol
// hatası OLMAZ: SDK onu IsError=true bir sonuca çevirir, model mesajı görür.
func call(ctx context.Context, api *apiClient, path string, q url.Values) (*mcp.CallToolResult, any, error) {
	body, err := api.get(ctx, path, q)
	if err != nil {
		return nil, nil, err
	}
	return nil, body, nil
}

// bookPath: id sayı olduğu için yola güvenle yazılır (string olsaydı PathEscape gerekirdi).
func bookPath(id int64) string { return fmt.Sprintf("/api/books/%d", id) }

func setIf(q url.Values, key, val string) {
	if val != "" {
		q.Set(key, val)
	}
}

// setPage: 0 = "gönderilmedi" → API kendi varsayılanını uygular. Negatif değer gönderilir ki
// API doğrulama mesajıyla reddetsin (sessizce yutmak modele yanlış sonuç gösterirdi).
func setPage(q url.Values, limit, offset int) {
	if limit != 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset != 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
}
