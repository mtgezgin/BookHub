package ratings

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/cache"
)

// TopRatedCacheKey, top-rated listesinin cache anahtarı. "v1": cevap biçimi değişirse
// anahtarı v2 yapmak, eski biçimdeki kayıtları yeni kodun okumasını engeller.
const TopRatedCacheKey = "books:top-rated:v1"

// TopRatedCacheTTL, cache'teki listenin en uzun ömrü. Açık invalidation'a EK bir güvenlik
// ağı: kaçan bir invalidation (ya da aşağıdaki yarış) en çok bu kadar bayatlık üretir.
const TopRatedCacheTTL = 60 * time.Second

// maxCachedTopRated: cache HER ZAMAN en iyi 50'yi tutar; istenen limit bundan dilimlenir.
// Böylece limit=10 ve limit=20 için ayrı anahtarlar (ve ayrı invalidation) gerekmez.
const maxCachedTopRated = 50

// CachedService, Service'i top-rated cache'iyle sarar (cache-aside deseni):
//
//	oku:  cache'e bak → varsa dön; yoksa DB'den hesapla, cache'e yaz, dön
//	yaz:  puan değişince (Rate/Remove) anahtarı SİL → bir sonraki okuma taze hesaplar
//
// NEDEN BU UÇ: EXPLAIN ANALYZE ve yük testi (bkz. WHY_GO_VS_TS §12) — sıralama anahtarı
// global C'ye bağlı olduğu için index'le hızlanamayan tek sorgu bu; ve herkese açık,
// okunması yazılmasından çok daha sık.
//
// BİLİNEN YARIŞ (cache-aside'ın doğası): bir okuyucu DB'den ESKİ listeyi okur, bu arada bir
// puan yazılır ve anahtar silinir, sonra okuyucu eski listeyi cache'e yazar → en çok TTL
// (60 sn) boyunca bayat. Sıralama için kabul edilebilir; kesinlik gerekseydi sürüm numaralı
// anahtar ya da yazımda cache'i yeniden hesaplama gerekirdi.
type CachedService struct {
	*Service
	cache cache.Cache
	ttl   time.Duration
	// load, testlerde DB'siz takılabilsin diye alan; varsayılanı Service.TopRated.
	load func(ctx context.Context, limit int32) (TopRated, error)
}

func NewCached(s *Service, c cache.Cache) *CachedService {
	return &CachedService{Service: s, cache: c, ttl: TopRatedCacheTTL, load: s.TopRated}
}

// TopRated, cache-aside okuma. Cache hatası isteği ASLA düşürmez: loglanır, DB'ye düşülür.
func (s *CachedService) TopRated(ctx context.Context, limit int32) (TopRated, error) {
	if b, ok, err := s.cache.Get(ctx, TopRatedCacheKey); err != nil {
		slog.Warn("top-rated cache okunamadı, DB'ye düşülüyor", "err", err)
	} else if ok {
		var t TopRated
		if err := json.Unmarshal(b, &t); err == nil {
			return first(t, limit), nil
		}
		slog.Warn("top-rated cache kaydı bozuk, yok sayıldı")
	}

	t, err := s.load(ctx, maxCachedTopRated)
	if err != nil {
		return TopRated{}, err
	}
	if b, err := json.Marshal(t); err == nil {
		if err := s.cache.Set(ctx, TopRatedCacheKey, b, s.ttl); err != nil {
			slog.Warn("top-rated cache yazılamadı", "err", err)
		}
	}
	return first(t, limit), nil
}

// Rate / Remove: yazım başarılıysa (commit SONRASI) cache'i geçersiz kıl. Commit'ten önce
// silseydik, commit'i beklemeyen bir okuyucu eski veriyi hemen yeniden cache'leyebilirdi.
func (s *CachedService) Rate(ctx context.Context, userID, bookID int64, score int) (Result, error) {
	res, err := s.Service.Rate(ctx, userID, bookID, score)
	if err == nil {
		s.InvalidateTopRated(ctx)
	}
	return res, err
}

func (s *CachedService) Remove(ctx context.Context, userID, bookID int64) (Stats, error) {
	st, err := s.Service.Remove(ctx, userID, bookID)
	if err == nil {
		s.InvalidateTopRated(ctx)
	}
	return st, err
}

// InvalidateTopRated, listeyi etkileyen başka yazımlar (onay, arşiv, başlık düzeltme) için de
// dışarı açık — moderasyon servisi bunu kanca (hook) olarak çağırır.
func (s *CachedService) InvalidateTopRated(ctx context.Context) {
	if err := s.cache.Delete(ctx, TopRatedCacheKey); err != nil {
		slog.Warn("top-rated cache silinemedi", "err", err, "max_staleness", s.ttl)
	}
}

func first(t TopRated, limit int32) TopRated {
	if int(limit) < len(t.Books) {
		t.Books = t.Books[:limit]
	}
	return t
}
