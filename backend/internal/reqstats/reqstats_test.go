package reqstats

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestStats(t *testing.T) {
	// Sayaçsız context: From nil döner ve nil alıcıdaki bütün metotlar güvenlidir.
	none := From(context.Background())
	if none != nil {
		t.Fatal("sayaç bağlanmamış context'te From nil dönmeli")
	}
	none.AddDB(time.Second)
	none.AddRedis(time.Second)
	if q, d := none.DB(); q != 0 || d != 0 {
		t.Errorf("nil sayaç sıfır dönmeli: %d %v", q, d)
	}
	if c, d := none.Redis(); c != 0 || d != 0 {
		t.Errorf("nil sayaç sıfır dönmeli: %d %v", c, d)
	}

	ctx, s := New(context.Background())
	// İptalden ayrılmış context sayacı korur (okuma istekleri böyle çalışır).
	detached := context.WithoutCancel(ctx)
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			From(detached).AddDB(time.Millisecond)
			From(ctx).AddRedis(2 * time.Millisecond)
		})
	}
	wg.Wait()
	if q, d := s.DB(); q != 50 || d != 50*time.Millisecond {
		t.Errorf("veritabanı: %d sorgu, %v", q, d)
	}
	if c, d := s.Redis(); c != 50 || d != 100*time.Millisecond {
		t.Errorf("redis: %d komut, %v", c, d)
	}
}
