package books

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

// fixture, entegrasyon testlerinin gerçek DB'ye bağlı repository'si.
type fixture struct {
	repo *Repository
	pool *pgxpool.Pool
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := testdb.Pool(t)
	return fixture{repo: NewRepository(pool), pool: pool}
}

// createBook, gerçek DB'ye kitap ekler ve testin sonunda siler.
func (f fixture) createBook(t *testing.T, status string) db.Book {
	t.Helper()
	book, err := f.repo.Create(context.Background(), db.CreateBookParams{
		Title:    "Test Kitabı " + status,
		Slug:     testdb.Unique("test-" + strings.ToLower(status)),
		Author:   "Test Yazar",
		Language: "tr",
		Status:   status,
	})
	if err != nil {
		t.Fatalf("%s kitabı eklenemedi: %v", status, err)
	}
	t.Cleanup(func() { testdb.Exec(t, f.pool, "DELETE FROM books WHERE id = $1", book.ID) })
	return book
}

// TestRepositoryGetVisibility, GERÇEK Postgres'e karşı Get'in görünürlük sınırını
// doğrular: yalnızca APPROVED döner; PENDING/REJECTED "yok" (ErrNotFound) muamelesi
// görür — onaylanmamış kitabın VARLIĞI bile sızmaz (OWASP BOLA/IDOR).
//
// Handler testleri sahte repository kullandığı için bu sınırı yalnızca burası
// kanıtlar — filtre SQL'den düşerse kırılacak test budur.
func TestRepositoryGetVisibility(t *testing.T) {
	f := newFixture(t)

	ids := map[string]int64{}
	for _, status := range []string{StatusApproved, StatusPending, StatusRejected} {
		ids[status] = f.createBook(t, status).ID
	}

	tests := []struct {
		status    string
		wantFound bool
	}{
		{status: StatusApproved, wantFound: true},
		{status: StatusPending},
		{status: StatusRejected},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got, err := f.repo.Get(context.Background(), ids[tt.status])
			if !tt.wantFound {
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("%s kitap SIZDI: istenen ErrNotFound, gelen (%+v, %v)", tt.status, got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("onaylı kitap okunamadı: %v", err)
			}
			if got.ID != ids[tt.status] || got.Status != StatusApproved {
				t.Errorf("yanlış kitap: istenen id=%d APPROVED, gelen id=%d %s", ids[tt.status], got.ID, got.Status)
			}
		})
	}
}

// TestRepositoryListOrderAndVisibility, List'in yalnızca SQL'de yaşayan iki sözünü
// kanıtlar: (1) yalnızca APPROVED listelenir, (2) en yeni en başta. Liste GLOBAL
// olduğu için mutlak içerik değil, bu testin eklediği satırların GÖRECELİ sırası
// ve varlığı/yokluğu doğrulanır — DB'de başka veri olsa da güvenilir kalır.
func TestRepositoryListOrderAndVisibility(t *testing.T) {
	f := newFixture(t)

	older := f.createBook(t, StatusApproved)
	newer := f.createBook(t, StatusApproved)
	pending := f.createBook(t, StatusPending)

	// created_at'i ELLE veriyoruz: art arda INSERT'lerde now() farkı mikrosaniye
	// mertebesinde kalır ve sıra iddiası kırılgan olur.
	for id, at := range map[int64]time.Time{
		older.ID:   time.Now().Add(-2 * time.Hour),
		newer.ID:   time.Now().Add(-1 * time.Hour),
		pending.ID: time.Now().Add(-90 * time.Minute),
	} {
		testdb.Exec(t, f.pool, "UPDATE books SET created_at = $1 WHERE id = $2", at, id)
	}

	got, _, err := f.repo.List(context.Background(), ListFilter{Limit: 1000})
	if err != nil {
		t.Fatalf("liste okunamadı: %v", err)
	}

	position := make(map[int64]int, len(got))
	for i, b := range got {
		position[b.ID] = i
		if b.Status != StatusApproved {
			t.Errorf("listede APPROVED olmayan kitap var: id=%d statü=%q", b.ID, b.Status)
		}
	}
	if _, leaked := position[pending.ID]; leaked {
		t.Errorf("PENDING kitap listeye SIZDI (id=%d)", pending.ID)
	}
	newerPos, okN := position[newer.ID]
	olderPos, okO := position[older.ID]
	if !okN || !okO {
		t.Fatalf("onaylı kitaplar listede olmalıydı (yeni=%v, eski=%v)", okN, okO)
	}
	if newerPos >= olderPos {
		t.Errorf("sıra yanlış: yeni kitap eskiden önce gelmeliydi (yeni=%d, eski=%d)", newerPos, olderPos)
	}
}

// TestRepositoryCreateDuplicateSlugRealDB, 23505 → ErrDuplicateSlug çevirisini GERÇEK
// Postgres'le kanıtlar. Birim testi PgError'ı elle kurar; bu test Postgres'in gerçekten
// o kodu ürettiğini doğrular — ikisi birlikte zincirin iki ucunu da kapatır.
func TestRepositoryCreateDuplicateSlugRealDB(t *testing.T) {
	f := newFixture(t)
	first := f.createBook(t, StatusApproved)

	_, err := f.repo.Create(context.Background(), db.CreateBookParams{
		Title: "Kopya", Slug: first.Slug, Author: "X", Language: "tr", Status: StatusApproved,
	})
	if !errors.Is(err, ErrDuplicateSlug) {
		t.Fatalf("aynı slug: istenen ErrDuplicateSlug, gelen %v", err)
	}
}
