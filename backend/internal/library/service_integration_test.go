package library_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/library"
	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

func day(t *testing.T, s string) *time.Time {
	t.Helper()
	v, err := time.Parse(time.DateOnly, s)
	if err != nil {
		t.Fatal(err)
	}
	return &v
}

// createEdition, kitaba bir baskı ekler (testdb.CreateBook baskısız kitap açar).
func createEdition(t *testing.T, pool *pgxpool.Pool, bookID int64, pages *int32, primary bool) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(), `
		INSERT INTO editions (book_id, title, language, page_count, is_primary)
		VALUES ($1, 'Baskı', 'tr', $2, $3) RETURNING id`, bookID, pages, primary).Scan(&id)
	if err != nil {
		t.Fatalf("baskı açılamadı: %v", err)
	}
	return id
}

func logCount(t *testing.T, pool *pgxpool.Pool, userID, bookID int64) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM reading_log WHERE user_id = $1 AND book_id = $2`, userID, bookID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func status(s library.Shelf) string {
	if s.Entry == nil {
		return "yok"
	}
	return s.Entry.Status
}

// fixture: bir okur ve 300 sayfalık varsayılan baskısı olan, yayında bir kitap.
type fixture struct {
	user, book, edition int64
}

func newFixture(t *testing.T, pool *pgxpool.Pool) fixture {
	t.Helper()
	f := fixture{user: testdb.CreateUser(t, pool, "Okur"), book: testdb.CreateBook(t, pool, "", books.StatusApproved)}
	f.edition = createEdition(t, pool, f.book, new(int32(300)), true)
	return f
}

// putState, kaydı doğrudan SQL'le istenen duruma koyar (servisten bağımsız kurulum). Okuyorken sayfa 42;
// yarım bırakılmışsa günlükte o okumanın ABANDONED satırı da vardır ("Devam et" geri açabilsin).
func putState(t *testing.T, pool *pgxpool.Pool, f fixture, st string) {
	t.Helper()
	switch st {
	case "yok":
	case library.StatusReading:
		testdb.Exec(t, pool, `INSERT INTO library_entries (user_id, book_id, status, edition_id, unit, progress, total_pages, started_on)
			VALUES ($1, $2, 'READING', $3, 'page', 42, 300, '2026-11-01')`, f.user, f.book, f.edition)
	case library.StatusAbandoned:
		testdb.Exec(t, pool, `INSERT INTO library_entries (user_id, book_id, status) VALUES ($1, $2, 'ABANDONED')`, f.user, f.book)
		testdb.Exec(t, pool, `INSERT INTO reading_log (user_id, book_id, edition_id, outcome, unit, progress, total_pages, started_on, ended_on)
			VALUES ($1, $2, $3, 'ABANDONED', 'page', 42, 300, '2026-11-01', '2026-11-05')`, f.user, f.book, f.edition)
	default:
		testdb.Exec(t, pool, `INSERT INTO library_entries (user_id, book_id, status) VALUES ($1, $2, $3)`, f.user, f.book, st)
	}
}

// TestShelfEveryPairRealDB: spec Doğrulama 1'in veritabanı yarısı. Her (durum, komut) çifti gerçek
// veritabanında İKİ KEZ gönderilir: ilki beklenen duruma geçer ve beklenen sayıda günlük satırı yazar,
// ikincisi (çift tık, yeniden deneme) hiçbir şeyi değiştirmez. Hiçbir çift hata dönmez.
func TestShelfEveryPairRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	svc := library.NewService(pool)
	ctx := context.Background()

	// Komutun sonundaki durum yalnız komuta bağlı (makinenin "hedef durum" tasarımı).
	target := map[library.Command]string{
		library.CmdWant: library.StatusWantToRead, library.CmdPostpone: library.StatusWantToRead,
		library.CmdStart: library.StatusReading, library.CmdFinish: library.StatusRead,
		library.CmdMarkRead: library.StatusRead, library.CmdAbandon: library.StatusAbandoned, library.CmdRemove: "yok",
	}
	// İlk gönderimin günlükteki etkisi (satır sayısı farkı); listede olmayan çiftlerde 0.
	logDelta := map[string]int{
		"READING/want": 1, "READING/postpone": 1, "READING/finish": 1, "READING/mark-read": 1,
		"READING/abandon": 1, "READING/remove": 1,
		"ABANDONED/start": -1, "ABANDONED/start/resume": -1, "ABANDONED/finish": 1, "ABANDONED/mark-read": 1,
		"yok/finish": 1, "yok/mark-read": 1, "WANT_TO_READ/finish": 1, "WANT_TO_READ/mark-read": 1,
	}
	variants := []struct {
		name string
		in   library.CommandInput
	}{
		{"want", library.CommandInput{Command: library.CmdWant}},
		{"postpone", library.CommandInput{Command: library.CmdPostpone}},
		{"start", library.CommandInput{Command: library.CmdStart, StartedOn: day(t, "2026-11-20")}},
		{"start/resume", library.CommandInput{Command: library.CmdStart, Mode: library.ModeResume}},
		{"start/restart", library.CommandInput{Command: library.CmdStart, Mode: library.ModeRestart}},
		{"finish", library.CommandInput{Command: library.CmdFinish, EndedOn: day(t, "2026-11-23")}},
		{"mark-read", library.CommandInput{Command: library.CmdMarkRead}},
		{"abandon", library.CommandInput{Command: library.CmdAbandon, EndedOn: day(t, "2026-11-23")}},
		{"remove", library.CommandInput{Command: library.CmdRemove}},
	}
	for _, st := range []string{"yok", library.StatusWantToRead, library.StatusReading, library.StatusRead, library.StatusAbandoned} {
		for _, v := range variants {
			t.Run(st+"/"+v.name, func(t *testing.T) {
				f := newFixture(t, pool)
				putState(t, pool, f, st)
				before := logCount(t, pool, f.user, f.book)

				first, err := svc.Do(ctx, f.user, f.book, v.in)
				if err != nil {
					t.Fatalf("ilk gönderim hata dönmemeli: %v", err)
				}
				if got, want := status(first), target[v.in.Command]; got != want {
					t.Errorf("durum = %s, istenen %s", got, want)
				}
				afterFirst := logCount(t, pool, f.user, f.book)
				if d := afterFirst - before; d != logDelta[st+"/"+v.name] {
					t.Errorf("günlük farkı = %d, istenen %d", d, logDelta[st+"/"+v.name])
				}

				second, err := svc.Do(ctx, f.user, f.book, v.in)
				if err != nil {
					t.Fatalf("ikinci gönderim hata dönmemeli: %v", err)
				}
				if status(second) != status(first) {
					t.Errorf("ikinci gönderim durumu değiştirdi: %s → %s", status(first), status(second))
				}
				if n := logCount(t, pool, f.user, f.book); n != afterFirst {
					t.Errorf("ikinci gönderim günlüğe yazdı: %d → %d", afterFirst, n)
				}
				if first.Entry != nil && second.Entry != nil && second.Entry.Version != first.Entry.Version {
					t.Errorf("hiçbir şey yapmayan gönderim sürümü artırmamalı: %d → %d", first.Entry.Version, second.Entry.Version)
				}
			})
		}
	}
}

func TestShelfReadingRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	svc := library.NewService(pool)
	ctx := context.Background()

	t.Run("başla: varsayılan baskı, sayfa birimi, toplamın görüntüsü, 0 ilerleme", func(t *testing.T) {
		f := newFixture(t, pool)
		s, err := svc.Do(ctx, f.user, f.book, library.CommandInput{Command: library.CmdStart, StartedOn: day(t, "2026-11-20")})
		if err != nil {
			t.Fatal(err)
		}
		e := s.Entry
		if *e.EditionID != f.edition || *e.Unit != library.UnitPage || *e.TotalPages != 300 || *e.Progress != 0 ||
			!e.StartedOn.Equal(*day(t, "2026-11-20")) {
			t.Errorf("beklenmeyen okuma: %+v", e)
		}
		// Kavram 7: admin baskının sayfa sayısını düzeltse de okumanın toplamı değişmez.
		testdb.Exec(t, pool, `UPDATE editions SET page_count = 500 WHERE id = $1`, f.edition)
		if s, _ = svc.Get(ctx, f.user, f.book); *s.Entry.TotalPages != 300 {
			t.Errorf("toplam baskıdan sonradan kopyalanmamalı: %d", *s.Entry.TotalPages)
		}
	})

	t.Run("başla: sayfası bilinmeyen baskıda yüzde; başka kitabın baskısı reddedilir", func(t *testing.T) {
		f := newFixture(t, pool)
		unknown := createEdition(t, pool, f.book, nil, false)
		s, err := svc.Do(ctx, f.user, f.book, library.CommandInput{Command: library.CmdStart, EditionID: &unknown})
		if err != nil || *s.Entry.Unit != library.UnitPercent || s.Entry.TotalPages != nil || s.Entry.StartedOn != nil {
			t.Errorf("yüzde ve toplamsız başlamalı (tarih verilmedi → bilinmiyor): %+v %v", s.Entry, err)
		}
		other := newFixture(t, pool)
		_, err = svc.Do(ctx, other.user, other.book, library.CommandInput{Command: library.CmdStart, EditionID: &f.edition})
		if !errors.Is(err, library.ErrInvalidEdition) {
			t.Errorf("ErrInvalidEdition bekleniyordu, gelen %v", err)
		}
		if logCount(t, pool, other.user, other.book) != 0 {
			t.Error("reddedilen komut bir şey yazmamalı")
		}
	})

	t.Run("devam et: sayfa ve başlangıç geri gelir, günlük satırı kalmaz; baştan başla günlüğü korur (Doğrulama 6)", func(t *testing.T) {
		f := newFixture(t, pool)
		putState(t, pool, f, library.StatusAbandoned)
		s, err := svc.Do(ctx, f.user, f.book, library.CommandInput{Command: library.CmdStart})
		if err != nil || *s.Entry.Progress != 42 || !s.Entry.StartedOn.Equal(*day(t, "2026-11-01")) || len(s.Log) != 0 {
			t.Errorf("devam et: %+v, günlük %d, %v", s.Entry, len(s.Log), err)
		}

		g := newFixture(t, pool)
		putState(t, pool, g, library.StatusAbandoned)
		s, err = svc.Do(ctx, g.user, g.book, library.CommandInput{Command: library.CmdStart, Mode: library.ModeRestart})
		if err != nil || *s.Entry.Progress != 0 || len(s.Log) != 1 || s.Log[0].Outcome != library.OutcomeAbandoned {
			t.Errorf("baştan başla: %+v, günlük %+v, %v", s.Entry, s.Log, err)
		}
	})

	t.Run("bitir: günlükte tarih ve ilerleme; bitiş başlangıçtan önce olamaz (Doğrulama 4)", func(t *testing.T) {
		f := newFixture(t, pool)
		putState(t, pool, f, library.StatusReading)
		if _, err := svc.Do(ctx, f.user, f.book, library.CommandInput{Command: library.CmdFinish, EndedOn: day(t, "2026-10-01")}); !errors.Is(err, library.ErrInvalidDates) {
			t.Errorf("ErrInvalidDates bekleniyordu, gelen %v", err)
		}
		s, err := svc.Do(ctx, f.user, f.book, library.CommandInput{Command: library.CmdFinish, EndedOn: day(t, "2026-11-23")})
		if err != nil || status(s) != library.StatusRead || len(s.Log) != 1 {
			t.Fatalf("bitir: %+v %v", s, err)
		}
		l := s.Log[0]
		if l.Outcome != library.OutcomeFinished || *l.Progress != 42 || !l.EndedOn.Equal(*day(t, "2026-11-23")) || !l.StartedOn.Equal(*day(t, "2026-11-01")) {
			t.Errorf("günlük satırı: %+v", l)
		}
		if s.Entry.Progress != nil || s.Entry.StartedOn != nil {
			t.Errorf("okunmuş kitabın okuma alanları boş dönmeli: %+v", s.Entry)
		}
	})

	t.Run("zaten okudum: verilen tarih yok sayılır (bilinmeyen tarih uydurulmaz)", func(t *testing.T) {
		f := newFixture(t, pool)
		s, err := svc.Do(ctx, f.user, f.book, library.CommandInput{Command: library.CmdMarkRead, EndedOn: day(t, "2026-11-23")})
		if err != nil || len(s.Log) != 1 || s.Log[0].EndedOn != nil {
			t.Errorf("tarihsiz FINISHED bekleniyordu: %+v %v", s.Log, err)
		}
	})

	t.Run("ilerleme yokken ertelemek ve çıkarmak günlüğe yazmaz; çıkarılanın geçmişi kalır (Doğrulama 5)", func(t *testing.T) {
		f := newFixture(t, pool)
		for _, c := range []library.Command{library.CmdStart, library.CmdPostpone, library.CmdStart, library.CmdRemove} {
			if _, err := svc.Do(ctx, f.user, f.book, library.CommandInput{Command: c}); err != nil {
				t.Fatal(err)
			}
		}
		if n := logCount(t, pool, f.user, f.book); n != 0 {
			t.Errorf("0 ilerlemeli okuma geçmişe yazılmamalı, satır %d", n)
		}

		g := newFixture(t, pool)
		putState(t, pool, g, library.StatusReading)
		if _, err := svc.Do(ctx, g.user, g.book, library.CommandInput{Command: library.CmdRemove}); err != nil {
			t.Fatal(err)
		}
		s, err := svc.Get(ctx, g.user, g.book)
		if err != nil || s.Entry != nil || len(s.Log) != 1 || *s.Log[0].Progress != 42 {
			t.Errorf("çıkarılan kitabın geçmişi görünmeli: %+v %v", s, err)
		}
	})

	t.Run("yayında olmayan kitap: komutlar 404, çıkarmak çalışır", func(t *testing.T) {
		f := newFixture(t, pool)
		putState(t, pool, f, library.StatusReading)
		testdb.Exec(t, pool, `UPDATE books SET status = 'ARCHIVED' WHERE id = $1`, f.book)
		for _, c := range []library.Command{library.CmdStart, library.CmdFinish, library.CmdWant} {
			if _, err := svc.Do(ctx, f.user, f.book, library.CommandInput{Command: c}); !errors.Is(err, books.ErrNotFound) {
				t.Errorf("%s: ErrNotFound bekleniyordu, gelen %v", c, err)
			}
		}
		if _, err := svc.Get(ctx, f.user, f.book); !errors.Is(err, books.ErrNotFound) {
			t.Errorf("raf okuması: ErrNotFound bekleniyordu, gelen %v", err)
		}
		if s, err := svc.Do(ctx, f.user, f.book, library.CommandInput{Command: library.CmdRemove}); err != nil || s.Entry != nil {
			t.Errorf("çıkarmak çalışmalı: %+v %v", s, err)
		}
	})
}

func TestShelfUpdateReadingRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	svc := library.NewService(pool)
	ctx := context.Background()

	start := func(t *testing.T) (fixture, int32) {
		t.Helper()
		f := newFixture(t, pool)
		s, err := svc.Do(ctx, f.user, f.book, library.CommandInput{Command: library.CmdStart})
		if err != nil {
			t.Fatal(err)
		}
		return f, s.Entry.Version
	}

	t.Run("ilerleme yazılır, sürüm artar; sınır dışı değerler reddedilir (Doğrulama 2)", func(t *testing.T) {
		f, v := start(t)
		for _, bad := range []int32{-1, 301} {
			if _, err := svc.UpdateReading(ctx, f.user, f.book, library.ReadingPatch{Version: v, Progress: &bad}); !errors.Is(err, library.ErrInvalidProgress) {
				t.Errorf("%d: ErrInvalidProgress bekleniyordu, gelen %v", bad, err)
			}
		}
		s, err := svc.UpdateReading(ctx, f.user, f.book, library.ReadingPatch{Version: v, Progress: new(int32(57))})
		if err != nil || *s.Entry.Progress != 57 || s.Entry.Version != v+1 {
			t.Errorf("ilerleme: %+v %v", s.Entry, err)
		}
	})

	t.Run("eski sürüm 409 döner ve satırı değiştirmez (Doğrulama 3)", func(t *testing.T) {
		f, v := start(t)
		if _, err := svc.UpdateReading(ctx, f.user, f.book, library.ReadingPatch{Version: v, Progress: new(int32(57))}); err != nil {
			t.Fatal(err)
		}
		_, err := svc.UpdateReading(ctx, f.user, f.book, library.ReadingPatch{Version: v, Progress: new(int32(12))})
		if !errors.Is(err, library.ErrVersionConflict) {
			t.Errorf("ErrVersionConflict bekleniyordu, gelen %v", err)
		}
		if s, _ := svc.Get(ctx, f.user, f.book); *s.Entry.Progress != 57 {
			t.Errorf("çakışan yazım satırı değiştirmemeli: %d", *s.Entry.Progress)
		}
	})

	t.Run("okumuyorsa 404 (sürüm çakışmasıyla karışmaz)", func(t *testing.T) {
		f := newFixture(t, pool)
		putState(t, pool, f, library.StatusRead)
		if _, err := svc.UpdateReading(ctx, f.user, f.book, library.ReadingPatch{Version: 1, Progress: new(int32(5))}); !errors.Is(err, books.ErrNotFound) {
			t.Errorf("ErrNotFound bekleniyordu, gelen %v", err)
		}
	})

	t.Run("birim ya da baskı değişince ilerleme sıfırlanır, toplam yeni baskıdan", func(t *testing.T) {
		f, v := start(t)
		s, err := svc.UpdateReading(ctx, f.user, f.book, library.ReadingPatch{Version: v, Progress: new(int32(57))})
		if err != nil {
			t.Fatal(err)
		}
		s, err = svc.UpdateReading(ctx, f.user, f.book, library.ReadingPatch{Version: s.Entry.Version, Unit: new(library.UnitPercent)})
		if err != nil || *s.Entry.Unit != library.UnitPercent || *s.Entry.Progress != 0 || s.Entry.TotalPages != nil {
			t.Fatalf("yüzdeye geçiş: %+v %v", s.Entry, err)
		}
		other := createEdition(t, pool, f.book, new(int32(120)), false)
		s, err = svc.UpdateReading(ctx, f.user, f.book, library.ReadingPatch{Version: s.Entry.Version, EditionID: &other, Unit: new(library.UnitPage), Progress: new(int32(30))})
		if err != nil || *s.Entry.EditionID != other || *s.Entry.TotalPages != 120 || *s.Entry.Progress != 30 {
			t.Errorf("baskı değişimi: %+v %v", s.Entry, err)
		}
	})

	t.Run("Faz 15'ten kalan okuma: ilk ilerlemeyle varsayılan baskı ve sayfa birimi (Doğrulama 8)", func(t *testing.T) {
		f := newFixture(t, pool)
		testdb.Exec(t, pool, `INSERT INTO library_entries (user_id, book_id, status) VALUES ($1, $2, 'READING')`, f.user, f.book)
		s, err := svc.UpdateReading(ctx, f.user, f.book, library.ReadingPatch{Version: 1, Progress: new(int32(10))})
		if err != nil || *s.Entry.EditionID != f.edition || *s.Entry.Unit != library.UnitPage || *s.Entry.TotalPages != 300 || *s.Entry.Progress != 10 {
			t.Errorf("ilk ilerleme: %+v %v", s.Entry, err)
		}
	})
}

// TestShelfAtomicityRealDB (Doğrulama 7): transaction ortasında hata olursa ne satır ne günlük değişir.
// Hata gerçek bir veritabanı hatası: yalnız bu testin kullanıcısı için günlük yazımını reddeden geçici bir
// tetikleyici. "Bitirdim" önce satırı READ yapar, sonra günlük satırını yazarken düşer; satır geri alınmalı.
// (0017'ye kadar hata kural dışı bir ilerlemeyle, -5, üretiliyordu; artık veritabanı onu baştan reddediyor.)
func TestShelfAtomicityRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	svc := library.NewService(pool)
	f := newFixture(t, pool)
	putState(t, pool, f, library.StatusReading)
	failLogWrites(t, pool, f.user)

	// Hata satır yazıldıktan SONRA, günlük yazımında gelmeli; yoksa geri alınacak bir şey yoktur.
	if _, err := svc.Do(context.Background(), f.user, f.book, library.CommandInput{Command: library.CmdFinish}); err == nil ||
		!strings.Contains(err.Error(), "(satır değişmişti)") {
		t.Fatalf("günlük yazımı hatası bekleniyordu: %v", err)
	}
	var st string
	var version int
	var progress *int32
	if err := pool.QueryRow(context.Background(), `SELECT status, version, progress FROM library_entries WHERE user_id = $1 AND book_id = $2`,
		f.user, f.book).Scan(&st, &version, &progress); err != nil || st != library.StatusReading || version != 1 || progress == nil || *progress != 42 {
		t.Errorf("satır geri alınmalı: %s v%d %v %v", st, version, progress, err)
	}
	if n := logCount(t, pool, f.user, f.book); n != 0 {
		t.Errorf("günlük boş kalmalı: %d", n)
	}
}

// failLogWrites, bu kullanıcının reading_log'a her yazımını reddeden geçici bir tetikleyici kurar; test
// bitince kaldırılır. Test veritabanı paylaşıldığı için koşul kullanıcıya bağlı (öteki testler etkilenmez).
// Kimlik sayı olduğu için DDL'e biçimlendirilerek girer (DDL parametre almaz).
//
// Hata mesajı, aynı transaction'da kitaplık satırının o an hâlâ "okuyor" olup olmadığını söyler. Böylece
// test, hatanın satır YAZILDIKTAN sonra geldiğini görebilir (yazılmadan gelirse geri alınacak bir şey yok).
func failLogWrites(t *testing.T, pool *pgxpool.Pool, user int64) {
	t.Helper()
	name := fmt.Sprintf("test_fail_log_%d", user)
	testdb.Exec(t, pool, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF EXISTS (SELECT 1 FROM library_entries
			           WHERE user_id = NEW.user_id AND book_id = NEW.book_id AND status = 'READING') THEN
				RAISE EXCEPTION 'deneme: günlük yazımı reddedildi (satır değişmemişti)';
			END IF;
			RAISE EXCEPTION 'deneme: günlük yazımı reddedildi (satır değişmişti)';
		END $$`, name))
	testdb.Exec(t, pool, fmt.Sprintf(`CREATE TRIGGER %[1]s BEFORE INSERT ON reading_log
		FOR EACH ROW WHEN (NEW.user_id = %[2]d) EXECUTE FUNCTION %[1]s()`, name, user))
	t.Cleanup(func() {
		testdb.Exec(t, pool, fmt.Sprintf(`DROP TRIGGER %[1]s ON reading_log; DROP FUNCTION %[1]s()`, name))
	})
}

// TestShelfConcurrentRealDB: aynı anda gelen iki "Zaten okudum" (iki sekme, çift tık) — kitap kitaplıkta
// yokken kilitlenecek satır da yoktur; danışma kilidi ikisini sıraya sokar, günlüğe tek satır yazılır.
func TestShelfConcurrentRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	svc := library.NewService(pool)
	for range 5 { // yarış her denemede çıkmayabilir; birkaç tur
		f := newFixture(t, pool)
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i := range errs {
			wg.Go(func() {
				_, errs[i] = svc.Do(context.Background(), f.user, f.book, library.CommandInput{Command: library.CmdMarkRead})
			})
		}
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				t.Fatalf("eşzamanlı komut hata dönmemeli: %v", err)
			}
		}
		if n := logCount(t, pool, f.user, f.book); n != 1 {
			t.Fatalf("günlükte tek satır olmalı, %d var", n)
		}
	}
}

func TestShelfLogRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	svc := library.NewService(pool)
	ctx := context.Background()

	f := newFixture(t, pool)
	putState(t, pool, f, library.StatusReading)
	s, err := svc.Do(ctx, f.user, f.book, library.CommandInput{Command: library.CmdFinish, EndedOn: day(t, "2026-11-23")})
	if err != nil {
		t.Fatal(err)
	}
	logID := s.Log[0].ID
	bob := testdb.CreateUser(t, pool, "Başkası")

	t.Run("başkasının kaydı 404 (Doğrulama 9)", func(t *testing.T) {
		if _, err := svc.UpdateLogDates(ctx, bob, logID, nil, day(t, "2026-11-24")); !errors.Is(err, books.ErrNotFound) {
			t.Errorf("düzeltme: ErrNotFound bekleniyordu, gelen %v", err)
		}
		if _, err := svc.DeleteLog(ctx, bob, logID); !errors.Is(err, books.ErrNotFound) {
			t.Errorf("silme: ErrNotFound bekleniyordu, gelen %v", err)
		}
	})

	t.Run("tarih düzeltilir; bitiş başlangıçtan önce olamaz", func(t *testing.T) {
		if _, err := svc.UpdateLogDates(ctx, f.user, logID, day(t, "2026-11-30"), nil); !errors.Is(err, library.ErrInvalidDates) {
			t.Errorf("ErrInvalidDates bekleniyordu, gelen %v", err)
		}
		s, err := svc.UpdateLogDates(ctx, f.user, logID, nil, day(t, "2026-11-22"))
		if err != nil || !s.Log[0].EndedOn.Equal(*day(t, "2026-11-22")) || !s.Log[0].StartedOn.Equal(*day(t, "2026-11-01")) {
			t.Errorf("düzeltme: %+v %v", s.Log, err)
		}
	})

	t.Run("silinir; ikinci silme 404", func(t *testing.T) {
		s, err := svc.DeleteLog(ctx, f.user, logID)
		if err != nil || len(s.Log) != 0 || status(s) != library.StatusRead {
			t.Errorf("silme: %+v %v", s, err)
		}
		if _, err := svc.DeleteLog(ctx, f.user, logID); !errors.Is(err, books.ErrNotFound) {
			t.Errorf("ikinci silme: ErrNotFound bekleniyordu, gelen %v", err)
		}
	})
}
