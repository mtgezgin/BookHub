package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/auth"
	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

func TestReadPasswordFromPipe(t *testing.T) {
	tests := []struct {
		in, want string
		wantErr  bool
	}{
		{in: "gizli-parola\n", want: "gizli-parola"},
		{in: "gizli-parola\r\n", want: "gizli-parola"},
		{in: "sonsatirsiz", want: "sonsatirsiz"},
		{in: "", wantErr: true},
		{in: "\n", wantErr: true},
	}
	for _, tt := range tests {
		got, err := readPassword(strings.NewReader(tt.in), &bytes.Buffer{})
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("%q: istenen (%q, hata=%v), gelen (%q, %v)", tt.in, tt.want, tt.wantErr, got, err)
		}
	}
}

func TestRunArgumentErrors(t *testing.T) {
	env := func(string) string { return "" }
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"komut yok", nil, "kullanım"},
		{"migrate: DB yok", []string{"migrate"}, "DATABASE_URL zorunlu"},
		{"bilinmeyen komut", []string{"drop-db"}, "bilinmeyen komut"},
		{"e-posta yok", []string{"create-admin"}, "-email zorunlu"},
		{"DB yok", []string{"create-admin", "-email", "a@b.co"}, "DATABASE_URL zorunlu"},
		{"bilinmeyen bayrak", []string{"create-admin", "-password", "x"}, "create-admin"},
		{"geçersiz AUTO_MIGRATE", []string{"create-admin", "-email", "a@b.co"}, "AUTO_MIGRATE true/false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := env
			if tt.name == "geçersiz AUTO_MIGRATE" {
				env = func(k string) string {
					return map[string]string{"DATABASE_URL": "postgres://x", "AUTO_MIGRATE": "yes"}[k]
				}
			}
			err := run(context.Background(), tt.args, strings.NewReader("gizli-parola\n"), &bytes.Buffer{}, env)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("%q içeren hata bekleniyordu, gelen %v", tt.want, err)
			}
		})
	}
}

// TestCreateAdminRealDB: komut gerçek DB'de admin açar; ikinci çalıştırma idempotent.
func TestCreateAdminRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	email := testdb.Unique("admin") + "@example.com"
	t.Cleanup(func() { testdb.Exec(t, pool, "DELETE FROM users WHERE email = $1", email) })
	env := func(k string) string {
		if k == "DATABASE_URL" {
			return os.Getenv("TEST_DATABASE_URL")
		}
		return ""
	}

	var out bytes.Buffer
	args := []string{"create-admin", "-email", email, "-name", "Kurucu"}
	if err := run(context.Background(), args, strings.NewReader("yeterince-uzun-parola\n"), &out, env); err != nil {
		t.Fatalf("create-admin: %v", err)
	}
	if !strings.Contains(out.String(), "admin oluşturuldu") {
		t.Errorf("çıktı: %q", out.String())
	}

	var role string
	if err := pool.QueryRow(context.Background(), "SELECT role FROM users WHERE email = $1", email).Scan(&role); err != nil {
		t.Fatalf("kullanıcı okunamadı: %v", err)
	}
	if role != auth.RoleAdmin {
		t.Errorf("rol ADMIN olmalı, gelen %q", role)
	}

	out.Reset()
	if err := run(context.Background(), args, strings.NewReader("baska-bir-parola\n"), &out, env); err != nil {
		t.Fatalf("ikinci çalıştırma: %v", err)
	}
	if !strings.Contains(out.String(), "DEĞİŞMEDİ") {
		t.Errorf("ikinci çalıştırma var olanı yükseltmeli: %q", out.String())
	}
}

// TestCreateAdminAutoMigrate: AUTO_MIGRATE=true iken database.Migrate havuz açılmadan
// ÖNCE çalışır; başarısızlığı "migration başarısız" olarak raporlanır.
func TestCreateAdminAutoMigrate(t *testing.T) {
	t.Run("migrate hatası → migration başarısız (havuza hiç gelinmez)", func(t *testing.T) {
		// Ulaşılamayan hedef: Migrate çalışmasaydı hata database.New'den (farklı metinle) gelirdi.
		env := func(k string) string {
			return map[string]string{
				"DATABASE_URL": "postgres://u:p@127.0.0.1:1/yok?sslmode=disable&connect_timeout=2",
				"AUTO_MIGRATE": "true",
			}[k]
		}
		err := run(context.Background(), []string{"create-admin", "-email", "a@b.co"}, strings.NewReader("yeterince-uzun-parola\n"), &bytes.Buffer{}, env)
		if err == nil || !strings.Contains(err.Error(), "migration başarısız") {
			t.Errorf("\"migration başarısız\" içeren hata bekleniyordu, gelen %v", err)
		}
	})

	t.Run("gerçek DB: migrate + admin oluşturma", func(t *testing.T) {
		pool := testdb.Pool(t)
		email := testdb.Unique("admin") + "@example.com"
		t.Cleanup(func() { testdb.Exec(t, pool, "DELETE FROM users WHERE email = $1", email) })
		env := func(k string) string {
			return map[string]string{"DATABASE_URL": os.Getenv("TEST_DATABASE_URL"), "AUTO_MIGRATE": "true"}[k]
		}
		var out bytes.Buffer
		if err := run(context.Background(), []string{"create-admin", "-email", email}, strings.NewReader("yeterince-uzun-parola\n"), &out, env); err != nil {
			t.Fatalf("create-admin (AUTO_MIGRATE=true): %v", err)
		}
		if !strings.Contains(out.String(), "admin oluşturuldu") {
			t.Errorf("çıktı: %q", out.String())
		}
	})
}

// TestCreateAdminPasswordPolicyRealDB: politikaya uymayan parola → "admin oluşturulamadı"
// ve DB'de kullanıcı AÇILMAZ (politika servis katmanında, CLI de ondan geçer).
func TestCreateAdminPasswordPolicyRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	email := testdb.Unique("admin") + "@example.com"
	t.Cleanup(func() { testdb.Exec(t, pool, "DELETE FROM users WHERE email = $1", email) })
	env := func(k string) string {
		if k == "DATABASE_URL" {
			return os.Getenv("TEST_DATABASE_URL")
		}
		return ""
	}

	err := run(context.Background(), []string{"create-admin", "-email", email}, strings.NewReader("kisa\n"), &bytes.Buffer{}, env)
	if err == nil || !strings.Contains(err.Error(), "admin oluşturulamadı") {
		t.Fatalf("\"admin oluşturulamadı\" içeren hata bekleniyordu, gelen %v", err)
	}

	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM users WHERE email = $1", email).Scan(&n); err != nil {
		t.Fatalf("sayım sorgusu: %v", err)
	}
	if n != 0 {
		t.Errorf("politikaya uymayan parolayla kullanıcı oluşturulmamalıydı, %d kayıt var", n)
	}
}

// TestMigrateCommandRealDB: migrate komutu güncel şemada hatasız ve idempotent.
func TestMigrateCommandRealDB(t *testing.T) {
	_ = testdb.Pool(t) // TEST_DATABASE_URL yoksa atlar; varsa şemayı hazırlar
	env := func(k string) string {
		if k == "DATABASE_URL" {
			return os.Getenv("TEST_DATABASE_URL")
		}
		return ""
	}
	for range 2 {
		var out bytes.Buffer
		if err := run(context.Background(), []string{"migrate"}, strings.NewReader(""), &out, env); err != nil {
			t.Fatalf("migrate: %v", err)
		}
		if !strings.Contains(out.String(), "güncel") {
			t.Errorf("çıktı: %q", out.String())
		}
	}
}
