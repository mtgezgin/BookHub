package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/reqstats"
)

// TestPoolSettingsRealDB: havuzun ayarları koddan gelir, pgx'in varsayılanlarından değil
// (REVIEW_FEEDBACK F08). Her bağlantı süre sınırlarıyla açılır; süresini aşan sorgu ve kilit
// beklemesi veritabanında sonlanır ve TimedOut olarak tanınır.
func TestPoolSettingsRealDB(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tanımlı değil → entegrasyon testi atlanıyor")
	}
	ctx := context.Background()

	pool, err := New(ctx, dsn, 3)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(pool.Close)
	if got := pool.Config().MaxConns; got != 3 {
		t.Errorf("MaxConns: istenen 3, gelen %d", got)
	}
	if got := pool.Config().MinConns; got != 1 {
		t.Errorf("MinConns: istenen 1, gelen %d", got)
	}

	def, err := New(ctx, dsn, 0)
	if err != nil {
		t.Fatalf("New (varsayılan): %v", err)
	}
	t.Cleanup(def.Close)
	if got := def.Config().MaxConns; got != DefaultMaxConns {
		t.Errorf("varsayılan MaxConns: istenen %d, gelen %d", DefaultMaxConns, got)
	}

	for name, want := range map[string]string{
		"statement_timeout":                   "10s",
		"lock_timeout":                        "3s",
		"idle_in_transaction_session_timeout": "15s",
	} {
		var got string
		if err := pool.QueryRow(ctx, "SELECT current_setting($1)", name).Scan(&got); err != nil {
			t.Fatalf("%s okunamadı: %v", name, err)
		}
		if got != want {
			t.Errorf("%s: istenen %s, gelen %s", name, want, got)
		}
	}

	t.Run("süresini aşan sorgu veritabanında sonlanır", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		// Testi 10 sn bekletmemek için sınır bu işlemde küçültülür; sınanan şey hata sınıfıdır.
		if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '50ms'"); err != nil {
			t.Fatal(err)
		}
		_, err = tx.Exec(ctx, "SELECT pg_sleep(2)")
		if !TimedOut(err) {
			t.Errorf("TimedOut bekleniyordu, gelen %v", err)
		}
	})

	t.Run("kilit beklemesi lock_timeout ile döner", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS pool_lock_probe (id int PRIMARY KEY)"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP TABLE IF EXISTS pool_lock_probe") })
		if _, err := pool.Exec(ctx, "INSERT INTO pool_lock_probe VALUES (1) ON CONFLICT DO NOTHING"); err != nil {
			t.Fatal(err)
		}
		holder, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = holder.Rollback(ctx) }()
		if _, err := holder.Exec(ctx, "SELECT 1 FROM pool_lock_probe WHERE id = 1 FOR UPDATE"); err != nil {
			t.Fatal(err)
		}
		waiter, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = waiter.Rollback(ctx) }()
		if _, err := waiter.Exec(ctx, "SET LOCAL lock_timeout = '50ms'"); err != nil {
			t.Fatal(err)
		}
		_, err = waiter.Exec(ctx, "SELECT 1 FROM pool_lock_probe WHERE id = 1 FOR UPDATE")
		if !TimedOut(err) {
			t.Errorf("TimedOut bekleniyordu, gelen %v", err)
		}
	})
}

// TestQueryTracerCountsRealDB: isteğin sayacı her sorguyu (BEGIN ve COMMIT dahil) ve süresini görür;
// sayaç taşımayan bir context'te sorgular dallanmadan çalışır (REVIEW_FEEDBACK F32).
func TestQueryTracerCountsRealDB(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tanımlı değil → entegrasyon testi atlanıyor")
	}
	pool, err := New(context.Background(), dsn, 2)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(pool.Close)

	ctx, stats := reqstats.New(context.Background())
	var n int
	if err := pool.QueryRow(ctx, "SELECT 1").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "SELECT pg_sleep(0.02)"); err != nil {
		t.Fatal(err)
	}
	if q, d := stats.DB(); q != 2 || d < 20*time.Millisecond {
		t.Errorf("2 sorgu ve en az 20 ms bekleniyordu: %d sorgu, %v", q, d)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SELECT 1"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if q, _ := stats.DB(); q != 5 {
		t.Errorf("BEGIN + sorgu + COMMIT üç gidiş-dönüş sayılmalı: toplam 5 bekleniyordu, gelen %d", q)
	}

	// Sayaçsız context (açılış, cmd/manage): çalışır, kimseye yazmaz.
	if err := pool.QueryRow(context.Background(), "SELECT 1").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if q, _ := stats.DB(); q != 5 {
		t.Errorf("başka context'in sorgusu bu sayaca yazılmamalı: %d", q)
	}
}
