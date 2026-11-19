package ratings

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// GlobalAverageTTL, genel ortalamanın süreç içinde tutulduğu süre.
const GlobalAverageTTL = 60 * time.Second

// GlobalAverage, genel ortalamayı (ağırlıklı puandaki C) süreç içinde kısa süre tutar.
//
// NEDEN (REVIEW_FEEDBACK F26; Faz 16c): C, puana göre her sıralama isteğinden ÖNCE ayrı bir sorguyla
// okunuyordu. 500 bin kitapta bu sorgu 204 ms; canlıda ayrıca veritabanına bir gidiş-dönüş. Oysa C
// bütün oyların ortalamasıdır ve tek bir oy onu fark edilir biçimde değiştirmez: 60 sn bayat bir C
// sıralamayı kaydırmaz.
//
// NEDEN Redis değil süreç içi: tek bir sayı. Redis'e sormak, kaçınmaya çalıştığımız gidiş-dönüşün
// aynısı olurdu. Bedeli: iki API örneği kısa süre farklı C kullanabilir (fark virgülden sonraki
// hanelerde; kabul edildi).
//
// Bu, saklanan türetilmiş veri DEĞİLDİR (şemada kolon yok): kaynağı her 60 sn'de yeniden okunan bir
// önbellektir ve süreç yeniden başlayınca boşalır.
type GlobalAverage struct {
	mu      sync.Mutex
	value   float64
	expires time.Time

	ttl  time.Duration
	now  func() time.Time
	load func(ctx context.Context) (float64, error)
}

// NewGlobalAverage, load ile okunan değeri ttl boyunca tutan bir önbellek kurar.
func NewGlobalAverage(ttl time.Duration, load func(ctx context.Context) (float64, error)) *GlobalAverage {
	return &GlobalAverage{ttl: ttl, now: time.Now, load: load}
}

// Get, taze değeri döner; süresi dolmuşsa kaynağından yeniden okur.
//
// Kilit okuma boyunca tutulur: süre dolduğu anda gelen on istek on ayrı sorgu çalıştırmaz, biri okur
// ve diğerleri onun sonucunu kullanır. Hata önbelleğe alınmaz; bir sonraki çağrı yeniden dener.
func (g *GlobalAverage) Get(ctx context.Context) (float64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.now().Before(g.expires) {
		return g.value, nil
	}
	v, err := g.load(ctx)
	if err != nil {
		return 0, fmt.Errorf("genel ortalama okunamadı: %w", err)
	}
	g.value, g.expires = v, g.now().Add(g.ttl)
	return v, nil
}
