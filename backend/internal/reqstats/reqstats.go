// Package reqstats, tek bir HTTP isteğinin dış kaynaklarda (veritabanı, Redis) geçirdiği süreyi toplar.
//
// NEDEN: erişim satırı yalnız toplam süreyi yazıyordu. Yavaş bir isteğin süresi ağda mı, sorguda mı,
// işlemci kısıtında mı geçti, ayırt edilemiyordu (REVIEW_FEEDBACK F32). Sayaç isteğin context'inde
// taşınır: veritabanı ve Redis katmanları HTTP'yi tanımadan ona yazar, erişim logu okur.
//
// Domain bilgisi içermez; yalnız standart kütüphaneye bağlıdır.
package reqstats

import (
	"context"
	"sync/atomic"
	"time"
)

// Stats, bir isteğin sayaçları. Alanlar atomik: bir istek birden çok goroutine'den sorgu atabilir.
type Stats struct {
	dbQueries  atomic.Int64
	dbNanos    atomic.Int64
	redisCmds  atomic.Int64
	redisNanos atomic.Int64
}

type ctxKey struct{}

// New, ctx'e boş bir sayaç bağlar. context.WithoutCancel değerleri koruduğu için sayaç, istemci
// iptalinden ayrılmış okuma sorgularına da ulaşır.
func New(ctx context.Context) (context.Context, *Stats) {
	s := &Stats{}
	return context.WithValue(ctx, ctxKey{}, s), s
}

// From, ctx'teki sayacı döner; yoksa nil. Stats'ın metotları nil alıcıda güvenlidir: bir HTTP isteğine
// bağlı olmayan sorgular (açılış, cmd/manage, testler) ayrıca dallanmadan çalışır.
func From(ctx context.Context) *Stats {
	s, _ := ctx.Value(ctxKey{}).(*Stats)
	return s
}

// AddDB, bir veritabanı sorgusunu ve süresini ekler.
func (s *Stats) AddDB(d time.Duration) {
	if s == nil {
		return
	}
	s.dbQueries.Add(1)
	s.dbNanos.Add(int64(d))
}

// AddRedis, bir Redis komutunu (ya da pipeline'ını) ve süresini ekler.
func (s *Stats) AddRedis(d time.Duration) {
	if s == nil {
		return
	}
	s.redisCmds.Add(1)
	s.redisNanos.Add(int64(d))
}

// DB, sorgu sayısını ve toplam süresini döner. Süre, sorguların duvar saati toplamıdır: sunucuda
// çalışma + ağ gidiş-dönüşü. Havuzdan bağlantı bekleme süresi dahil değildir.
func (s *Stats) DB() (queries int64, total time.Duration) {
	if s == nil {
		return 0, 0
	}
	return s.dbQueries.Load(), time.Duration(s.dbNanos.Load())
}

// Redis, komut sayısını ve toplam süresini döner.
func (s *Stats) Redis() (cmds int64, total time.Duration) {
	if s == nil {
		return 0, 0
	}
	return s.redisCmds.Load(), time.Duration(s.redisNanos.Load())
}
