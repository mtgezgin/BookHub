// Command manage, operatör komutlarını çalıştırır (API'den ayrı bir binary).
//
// İlk admin nasıl doğar? Kayıt ucu (/api/auth/register) her zaman USER açar — bilerek.
// "İlk kaydolan admin olur" gibi bir kural, yayına alınan her kurulumda bir yarış
// (saldırgan senden önce kaydolursa) demek. Admin'i ancak sunucuya/DB'ye erişimi olan
// operatör, bu komutla oluşturur.
//
// Kullanım:
//
//	go run ./cmd/manage create-admin -email admin@example.com [-name "Admin"]
//
// Parola ASLA argüman olarak verilmez (shell geçmişine ve `ps` çıktısına düşerdi):
// terminalde gizli sorulur; betiklerde stdin'den okunur:
//
//	printf '%s\n' "$PAROLA" | go run ./cmd/manage create-admin -email admin@example.com
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/mtgezgin/bookhub/backend/internal/auth"
	"github.com/mtgezgin/bookhub/backend/internal/database"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, "hata:", err)
		stop()
		os.Exit(1)
	}
}

const usage = `kullanım: manage <komut> [bayraklar]

komutlar:
  migrate                           bekleyen migration'ları uygular (DATABASE_URL)
  create-admin -email E [-name N]   admin oluşturur ya da var olan hesabı admin yapar`

// run, main'in test edilebilir hâli: argümanlar, girdi/çıktı ve env dışarıdan gelir.
func run(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, getenv func(string) string) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	switch args[0] {
	case "migrate":
		return migrate(ctx, stdout, getenv)
	case "create-admin":
		return createAdmin(ctx, args[1:], stdin, stdout, getenv)
	default:
		return fmt.Errorf("bilinmeyen komut %q\n%s", args[0], usage)
	}
}

// migrate, migration'ları AYRI ve BİLİNÇLİ bir adım olarak uygular (Kubernetes'te bir Job,
// prod'da deploy öncesi). API'nin AUTO_MIGRATE'i dev kolaylığıdır; birden çok API kopyası
// aynı anda açılırken şemayı değiştirmek yerine önce bu komut koşar, sonra kopyalar açılır.
// Idempotent: şema güncelse hiçbir şey yapmaz; eşzamanlı çalıştırmaya advisory lock korur.
func migrate(ctx context.Context, stdout io.Writer, getenv func(string) string) error {
	dsn := getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL zorunlu (set -a; source .env; set +a)")
	}
	if err := database.Migrate(ctx, dsn); err != nil {
		return fmt.Errorf("migration başarısız: %w", err)
	}
	_, err := fmt.Fprintln(stdout, "migration'lar güncel")
	return err
}

func createAdmin(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, getenv func(string) string) error {
	fs := flag.NewFlagSet("create-admin", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	email := fs.String("email", "", "admin e-postası (zorunlu)")
	name := fs.String("name", "Admin", "görünen ad")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("create-admin: %w", err)
	}
	if *email == "" {
		return errors.New("create-admin: -email zorunlu")
	}

	dsn := getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL zorunlu (set -a; source .env; set +a)")
	}

	password, err := readPassword(stdin, stdout)
	if err != nil {
		return err
	}

	// API ile aynı kural: AUTO_MIGRATE=true ise şemayı güncelle (taze DB'de de çalışsın).
	// Geçersiz değer ("yes", "on") HATA: sessizce false saymak migration'ı atlatır ve
	// operatör anlamsız bir "relation users does not exist" görür (config.getenvBool ile aynı).
	migrate := false
	if raw := getenv("AUTO_MIGRATE"); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("AUTO_MIGRATE true/false olmalı, gelen %q", raw)
		}
		migrate = b
	}
	if migrate {
		if err := database.Migrate(ctx, dsn); err != nil {
			return fmt.Errorf("migration başarısız: %w", err)
		}
	}
	pool, err := database.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Token yöneticisi ve süreler gerekmiyor (EnsureAdmin oturum açmaz) → nil / sıfır.
	svc := auth.NewService(auth.NewRepository(pool), nil, 0, 0)
	u, created, err := svc.EnsureAdmin(ctx, *email, *name, password)
	if err != nil {
		return fmt.Errorf("admin oluşturulamadı: %w", err)
	}

	if created {
		_, err = fmt.Fprintf(stdout, "admin oluşturuldu: id=%d email=%s\n", u.ID, u.Email)
	} else {
		_, err = fmt.Fprintf(stdout, "var olan kullanıcı admin yapıldı (parolası DEĞİŞMEDİ): id=%d email=%s\n", u.ID, u.Email)
	}
	return err
}

// readPassword: stdin bir terminalse gizli (ekrana basmadan) iki kez sorar; değilse
// (pipe) ilk satırı okur.
func readPassword(stdin io.Reader, stdout io.Writer) (string, error) {
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		first, err := prompt(f, stdout, "Parola: ")
		if err != nil {
			return "", err
		}
		second, err := prompt(f, stdout, "Parola (tekrar): ")
		if err != nil {
			return "", err
		}
		if first != second {
			return "", errors.New("parolalar eşleşmiyor")
		}
		return first, nil
	}

	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("parola okunamadı: %w", err)
	}
	pw := strings.TrimRight(line, "\r\n")
	if pw == "" {
		return "", errors.New("parola boş (stdin'den bir satır bekleniyordu)")
	}
	return pw, nil
}

func prompt(f *os.File, out io.Writer, label string) (string, error) {
	if _, err := fmt.Fprint(out, label); err != nil {
		return "", err
	}
	b, err := term.ReadPassword(int(f.Fd()))
	if _, perr := fmt.Fprintln(out); perr != nil && err == nil {
		err = perr
	}
	if err != nil {
		return "", fmt.Errorf("parola okunamadı: %w", err)
	}
	return string(b), nil
}
