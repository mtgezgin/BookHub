package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/mtgezgin/bookhub/backend/migrations"
)

// TestMigrateConcurrentFreshDB: TAZE bir veritabanına aynı anda 4 Migrate çağrısı —
// hepsi hatasız bitmeli ve kilit kilitlenmeye (deadlock/zaman aşımı) yol açmamalı.
//
// DÜRÜST NOT: bu test bir regresyon kanıtı DEĞİL, bir duman testi (smoke test). Kilidi
// kaldırıp 8 kez denedik, kilitsiz sürüm de geçti — yarış penceresi bu migration'larla
// yeniden üretilemeyecek kadar dar. Test, advisory lock'un eşzamanlı açılışı BOZMADIĞINI
// (herkesin sırayla geçtiğini) doğruluyor.
func TestMigrateConcurrentFreshDB(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	fresh := freshDatabase(ctx, t, "bookhub_migrate")

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

// freshDatabase, TEST_DATABASE_URL'in sunucusunda boş, geçici bir veritabanı açar ve onun
// DSN'ini döner; test bitince veritabanı silinir. Değişken yoksa ya da veritabanı açma yetkisi
// yoksa testi ATLAR. Migration testleri ortak havuzu (testdb) kullanamaz: şemayı sıfırdan
// kurmaları gerekir.
func freshDatabase(ctx context.Context, t *testing.T, prefix string) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tanımlı değil → entegrasyon testi atlanıyor")
	}
	admin, err := New(ctx, dsn, 0)
	if err != nil {
		t.Fatalf("bağlantı: %v", err)
	}
	// t.Cleanup LIFO çalışır: önce aşağıdaki DROP, SONRA havuzun kapanışı. (defer kullansaydık
	// havuz bu fonksiyon biterken kapanır, DROP'u çalıştıracak bağlantı kalmazdı.)
	t.Cleanup(admin.Close)

	name := fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
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
	return u.String()
}

// schemaDB, migration testlerinin ortak SQL yardımcıları: kurulum (must), kısıt denemesi (try) ve sayım
// (count). Test fonksiyonu yalnız senaryoyu anlatsın; her kontrol tek satırda kalsın.
type schemaDB struct {
	ctx  context.Context
	t    *testing.T
	conn *sql.DB
}

// must, kurulum sorgusu; hata testi durdurur.
func (d schemaDB) must(q string, args ...any) {
	d.t.Helper()
	if _, err := d.conn.ExecContext(d.ctx, q, args...); err != nil {
		d.t.Fatalf("%s: %v", q, err)
	}
}

// try, kısıtın kabul ya da reddettiği bir yazımı dener; hatayı döndürür.
func (d schemaDB) try(q string, args ...any) error {
	_, err := d.conn.ExecContext(d.ctx, q, args...)
	return err
}

// count, tek sayı döndüren sorgu.
func (d schemaDB) count(q string, args ...any) int {
	d.t.Helper()
	var n int
	if err := d.conn.QueryRowContext(d.ctx, q, args...).Scan(&n); err != nil {
		d.t.Fatalf("%s: %v", q, err)
	}
	return n
}

// migratedTo, geçici bir veritabanını (freshDatabase) verilen sürüme kadar kurar; tek bir
// migration'ın taşımasını sınayan testler eski şemaya veri yazıp sonrakini provider'la uygular.
func migratedTo(ctx context.Context, t *testing.T, prefix string, version int64) (*sql.DB, *goose.Provider) {
	t.Helper()
	conn, err := sql.Open("pgx", freshDatabase(ctx, t, prefix))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, conn, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, version); err != nil {
		t.Fatalf("%d. sürüme kadar: %v", version, err)
	}
	return conn, provider
}
