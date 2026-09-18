package database

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

// TestMigrateConcurrentFreshDB: TAZE bir veritabanına aynı anda 4 Migrate çağrısı —
// hepsi hatasız bitmeli ve kilit kilitlenmeye (deadlock/zaman aşımı) yol açmamalı.
//
// DÜRÜST NOT: bu test bir regresyon kanıtı DEĞİL, bir duman testi (smoke test). Kilidi
// kaldırıp 8 kez denedik, kilitsiz sürüm de geçti — yarış penceresi bu migration'larla
// yeniden üretilemeyecek kadar dar. Test, advisory lock'un eşzamanlı açılışı BOZMADIĞINI
// (herkesin sırayla geçtiğini) doğruluyor.
func TestMigrateConcurrentFreshDB(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tanımlı değil → entegrasyon testi atlanıyor")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	admin, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("bağlantı: %v", err)
	}
	// t.Cleanup LIFO çalışır: önce aşağıdaki DROP, SONRA havuzun kapanışı. (defer kullansaydık
	// havuz test fonksiyonu biterken kapanır, DROP'u çalıştıracak bağlantı kalmazdı.)
	t.Cleanup(admin.Close)

	name := fmt.Sprintf("bookhub_migrate_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Skipf("geçici veritabanı açılamadı (yetki?): %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("geçici veritabanı silinemedi: %v", err)
		}
	})

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("DSN: %v", err)
	}
	u.Path = "/" + name
	fresh := u.String()

	const n = 4
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = Migrate(ctx, fresh)
		}()
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("migrate #%d başarısız: %v", i, err)
		}
	}
}
