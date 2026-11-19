package ratings

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// TestGlobalAverageCachesForTTL: genel ortalama süre dolana kadar kaynağından YENİDEN okunmaz
// (puana göre her sıralama isteğinde ayrı bir sorgu çalışmasın; Faz 16c). Hata önbelleğe alınmaz.
func TestGlobalAverageCachesForTTL(t *testing.T) {
	now := time.Now()
	loads, value := 0, 6.5
	var loadErr error
	g := NewGlobalAverage(time.Minute, func(context.Context) (float64, error) {
		loads++
		return value, loadErr
	})
	g.now = func() time.Time { return now }
	ctx := context.Background()

	for range 3 {
		if v, err := g.Get(ctx); err != nil || v != 6.5 {
			t.Fatalf("Get: %v %v", v, err)
		}
	}
	if loads != 1 {
		t.Fatalf("süre dolmadan tek okuma bekleniyordu, gelen %d", loads)
	}

	value = 7.0
	now = now.Add(59 * time.Second)
	if v, _ := g.Get(ctx); v != 6.5 || loads != 1 {
		t.Errorf("59. saniyede hâlâ eski değer: %v, %d okuma", v, loads)
	}
	now = now.Add(time.Second)
	if v, _ := g.Get(ctx); v != 7.0 || loads != 2 {
		t.Errorf("süre dolunca yeniden okunmalı: %v, %d okuma", v, loads)
	}

	// Hata: çağırana döner, önbelleğe alınmaz; sonraki çağrı yeniden dener.
	now = now.Add(time.Minute)
	loadErr = errors.New("db")
	if _, err := g.Get(ctx); err == nil {
		t.Error("kaynak hatası çağırana dönmeli")
	}
	loadErr = nil
	if v, err := g.Get(ctx); err != nil || v != 7.0 || loads != 4 {
		t.Errorf("hatadan sonra yeniden denenmeli: %v %v, %d okuma", v, err, loads)
	}
}

// TestGlobalAverageSingleLoadUnderConcurrency: süre dolduğu anda gelen eşzamanlı istekler tek okuma yapar.
func TestGlobalAverageSingleLoadUnderConcurrency(t *testing.T) {
	loads := 0
	g := NewGlobalAverage(time.Minute, func(context.Context) (float64, error) {
		loads++ // kilit altında çağrılır; -race bunu doğrular
		time.Sleep(5 * time.Millisecond)
		return 6.0, nil
	})
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if v, err := g.Get(context.Background()); err != nil || v != 6.0 {
				t.Errorf("Get: %v %v", v, err)
			}
		})
	}
	wg.Wait()
	if loads != 1 {
		t.Errorf("tek okuma bekleniyordu, gelen %d", loads)
	}
}
