package auth

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// TestHashQueueRunsOneAtATime: parola özetleri sıraya girer; aynı anda yalnız biri çalışır
// (REVIEW_FEEDBACK F04). Sırası bekleme süresinde gelmeyen istek ErrBusy alır ve işi hiç başlatmaz.
func TestHashQueueRunsOneAtATime(t *testing.T) {
	s, _, _ := newTestService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword); err != nil {
		t.Fatalf("Register: %v", err)
	}

	entered, release := make(chan struct{}), make(chan struct{})
	var running, peak, calls atomic.Int32
	s.compare = func(_, _ []byte) error {
		calls.Add(1)
		if n := running.Add(1); n > peak.Load() {
			peak.Store(n)
		}
		defer running.Add(-1)
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		return nil
	}

	first := make(chan error, 1)
	go func() { _, err := s.Login(ctx, "ali@example.com", goodPassword); first <- err }()
	<-entered // ilk giriş yuvayı tutuyor

	// Yuva doluyken gelen istek: bekleme süresi dolunca ErrBusy; karşılaştırma hiç çalışmaz.
	s.hashWait = 20 * time.Millisecond
	if _, err := s.Login(ctx, "ali@example.com", goodPassword); !errors.Is(err, ErrBusy) {
		t.Errorf("ErrBusy bekleniyordu, gelen %v", err)
	}
	// Var olmayan hesap da AYNI cevabı alır: "meşgul" hesabın varlığını sızdırmaz.
	if _, err := s.Login(ctx, "yok@example.com", goodPassword); !errors.Is(err, ErrBusy) {
		t.Errorf("var olmayan hesapta da ErrBusy bekleniyordu, gelen %v", err)
	}
	// Kayıt da aynı sırayı bekler.
	if _, err := s.Register(ctx, "veli@example.com", "Veli", goodPassword); !errors.Is(err, ErrBusy) {
		t.Errorf("kayıtta ErrBusy bekleniyordu, gelen %v", err)
	}
	// İstemci vazgeçerse beklemeyi bırakır.
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	s.hashWait = time.Minute
	if _, err := s.Login(cctx, "ali@example.com", goodPassword); !errors.Is(err, context.Canceled) {
		t.Errorf("context.Canceled bekleniyordu, gelen %v", err)
	}

	close(release)
	if err := <-first; err != nil {
		t.Errorf("yuvayı tutan giriş başarılı bitmeliydi: %v", err)
	}
	if calls.Load() != 1 || peak.Load() != 1 {
		t.Errorf("yalnız bir karşılaştırma çalışmalıydı: %d çağrı, aynı anda en çok %d", calls.Load(), peak.Load())
	}
	// Yuva boşaldı: sıradaki istek çalışır.
	if _, err := s.Login(ctx, "ali@example.com", goodPassword); err != nil {
		t.Errorf("yuva boşalınca giriş çalışmalı: %v", err)
	}
}
