package editions_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/editions"
	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

// newBook, varsayılan baskısıyla bir kitap açar (gerçek yol: books.Repository.Create).
func newBook(t *testing.T, pool *pgxpool.Pool, cover string, pages int32) db.BookView {
	t.Helper()
	slug := testdb.Unique("baski")
	b, err := books.NewRepository(pool).Create(context.Background(), books.CreateParams{
		CreateBookParams: db.CreateBookParams{Title: "Suç ve Ceza " + slug, Slug: slug, Author: "Dostoyevski", Language: "tr", Status: books.StatusApproved},
		CoverURL:         cover,
		PageCount:        &pages,
	})
	if err != nil {
		t.Fatalf("kitap açılamadı: %v", err)
	}
	t.Cleanup(func() { testdb.Exec(t, pool, "DELETE FROM books WHERE id = $1", b.ID) })
	return b
}

func ptr[T any](v T) *T { return &v }

func primaryCount(t *testing.T, pool *pgxpool.Pool, bookID int64) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM editions WHERE book_id = $1 AND is_primary`, bookID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestBookCreateOpensPrimaryEdition(t *testing.T) {
	pool := testdb.Pool(t)
	b := newBook(t, pool, "https://example.test/suc.jpg", 687)

	if b.CoverUrl != "https://example.test/suc.jpg" || b.PageCount == nil || *b.PageCount != 687 {
		t.Errorf("kitap kapağı ve sayfası varsayılan baskıdan gelmeli: %q %v", b.CoverUrl, b.PageCount)
	}
	list, err := editions.NewService(pool).ListByBook(context.Background(), b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || !list[0].IsPrimary || list[0].Language != "tr" || list[0].Title != b.Title {
		t.Fatalf("tek varsayılan baskı bekleniyordu (kitabın adı ve diliyle): %+v", list)
	}
}

func TestEditionLifecycle(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	admin := testdb.CreateUser(t, pool, "Admin")
	hookCalls := 0
	svc := editions.NewService(pool, editions.WithPublicChangeHook(func(context.Context) { hookCalls++ }))
	b := newBook(t, pool, "https://example.test/tr.jpg", 687)

	ru, err := svc.Create(ctx, admin, b.ID, editions.CreateParams{Title: "Преступление и наказание", Language: "ru", PublicationYear: ptr(int32(1866))})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ru.IsPrimary {
		t.Error("varsayılanı olan kitaba eklenen baskı varsayılan olmamalı")
	}

	t.Run("sıra: varsayılan önce, sonra dil", func(t *testing.T) {
		list, err := svc.ListByBook(ctx, b.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 2 || !list[0].IsPrimary || list[1].ID != ru.ID {
			t.Fatalf("sıra yanlış: %+v", list)
		}
	})

	t.Run("varsayılan silinemez", func(t *testing.T) {
		list, _ := svc.ListByBook(ctx, b.ID)
		if err := svc.Delete(ctx, admin, list[0].ID); !errors.Is(err, editions.ErrPrimaryEditionDelete) {
			t.Fatalf("ErrPrimaryEditionDelete bekleniyordu, gelen %v", err)
		}
	})

	t.Run("varsayılan yap: tek varsayılan kalır, kitap kapağı değişir", func(t *testing.T) {
		if _, err := svc.Update(ctx, admin, ru.ID, db.UpdateEditionParams{CoverUrl: ptr("https://example.test/ru.jpg"), Translator: ptr("")}); err != nil {
			t.Fatalf("Update: %v", err)
		}
		got, err := svc.SetPrimary(ctx, admin, ru.ID)
		if err != nil {
			t.Fatalf("SetPrimary: %v", err)
		}
		if !got.IsPrimary || primaryCount(t, pool, b.ID) != 1 {
			t.Fatalf("tek varsayılan olmalı: döndü %+v, sayı %d", got, primaryCount(t, pool, b.ID))
		}
		book, err := books.NewRepository(pool).Get(ctx, b.ID)
		if err != nil {
			t.Fatal(err)
		}
		if book.CoverUrl != "https://example.test/ru.jpg" || book.PageCount != nil {
			t.Errorf("kitap görünümü yeni varsayılandan okumalı: %q %v", book.CoverUrl, book.PageCount)
		}
		if _, err := svc.SetPrimary(ctx, admin, ru.ID); err != nil { // zaten varsayılan: değişiklik yok
			t.Fatalf("ikinci SetPrimary: %v", err)
		}
	})

	t.Run("eski varsayılan artık silinebilir", func(t *testing.T) {
		list, _ := svc.ListByBook(ctx, b.ID)
		if err := svc.Delete(ctx, admin, list[1].ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if err := svc.Delete(ctx, admin, list[1].ID); !errors.Is(err, editions.ErrNotFound) {
			t.Fatalf("ikinci silme ErrNotFound olmalı, gelen %v", err)
		}
	})

	t.Run("denetim kaydı kitabın geçmişinde", func(t *testing.T) {
		rows, err := pool.Query(ctx, `SELECT action, details FROM audit_logs WHERE entity_type = 'book' AND entity_id = $1 ORDER BY id`, b.ID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var got []string
		for rows.Next() {
			var action string
			var details json.RawMessage
			if err := rows.Scan(&action, &details); err != nil {
				t.Fatal(err)
			}
			var d struct {
				EditionID int64    `json:"edition_id"`
				Fields    []string `json:"fields"`
			}
			if err := json.Unmarshal(details, &d); err != nil || d.EditionID == 0 {
				t.Errorf("%s: details edition_id taşımalı: %s", action, details)
			}
			if action == editions.ActionUpdate && len(d.Fields) != 2 {
				t.Errorf("update yalnız gönderilen alan adlarını yazmalı: %v", d.Fields)
			}
			got = append(got, action)
		}
		want := []string{editions.ActionCreate, editions.ActionUpdate, editions.ActionSetPrimary, editions.ActionDelete}
		if len(got) != len(want) {
			t.Fatalf("eylemler: istenen %v, gelen %v", want, got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("eylemler: istenen %v, gelen %v", want, got)
			}
		}
	})

	// Yazımlar: Create, Update, SetPrimary (değişen), Delete. Başarısız ve boş (zaten varsayılan) çağrı
	// kancayı tetiklese de zararsız; sayılan en az dört başarılı yazım.
	if hookCalls < 4 {
		t.Errorf("önbellek kancası her başarılı yazımdan sonra çağrılmalı: %d", hookCalls)
	}
}

func TestEditionNotFound(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	admin := testdb.CreateUser(t, pool, "Admin")
	svc := editions.NewService(pool)

	if _, err := svc.Create(ctx, admin, 0, editions.CreateParams{Title: "x", Language: "tr"}); !errors.Is(err, books.ErrNotFound) {
		t.Errorf("olmayan kitap: books.ErrNotFound bekleniyordu, gelen %v", err)
	}
	if _, err := svc.Update(ctx, admin, 0, db.UpdateEditionParams{Title: ptr("x")}); !errors.Is(err, editions.ErrNotFound) {
		t.Errorf("Update: ErrNotFound bekleniyordu, gelen %v", err)
	}
	if _, err := svc.SetPrimary(ctx, admin, 0); !errors.Is(err, editions.ErrNotFound) {
		t.Errorf("SetPrimary: ErrNotFound bekleniyordu, gelen %v", err)
	}
}

// Baskısı olmayan kitap (genişletme yayın aralığında eski kodla eklenmiş): görünümde kapak ” ve
// sayfa NULL; ilk eklenen baskı varsayılan olur; kitap PATCH'i varsayılan baskıyı açar.
func TestBookWithoutEdition(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	admin := testdb.CreateUser(t, pool, "Admin")
	id := testdb.CreateBook(t, pool, "", books.StatusApproved) // doğrudan INSERT: baskı yok

	b, err := books.NewRepository(pool).Get(ctx, id)
	if err != nil {
		t.Fatalf("baskısız kitap da okunmalı (LEFT JOIN): %v", err)
	}
	if b.CoverUrl != "" || b.PageCount != nil {
		t.Errorf("baskısız kitap: kapak '' ve sayfa nil olmalı: %q %v", b.CoverUrl, b.PageCount)
	}
	ed, err := editions.NewService(pool).Create(ctx, admin, id, editions.CreateParams{Title: "İlk", Language: "tr"})
	if err != nil {
		t.Fatal(err)
	}
	if !ed.IsPrimary {
		t.Error("varsayılanı olmayan kitaba eklenen ilk baskı varsayılan olmalı")
	}

	other := testdb.CreateBook(t, pool, "", books.StatusApproved)
	if err := db.New(pool).PatchPrimaryEdition(ctx, db.PatchPrimaryEditionParams{BookID: other, PageCount: ptr(int32(120))}); err != nil {
		t.Fatal(err)
	}
	got, _ := books.NewRepository(pool).Get(ctx, other)
	if got.PageCount == nil || *got.PageCount != 120 || got.CoverUrl != "" || primaryCount(t, pool, other) != 1 {
		t.Errorf("PATCH baskısız kitaba varsayılan baskı açmalı: %+v", got)
	}
	if err := db.New(pool).PatchPrimaryEdition(ctx, db.PatchPrimaryEditionParams{BookID: other, CoverUrl: ptr("https://example.test/c.jpg")}); err != nil {
		t.Fatal(err)
	}
	got, _ = books.NewRepository(pool).Get(ctx, other)
	if got.PageCount == nil || *got.PageCount != 120 || got.CoverUrl != "https://example.test/c.jpg" || primaryCount(t, pool, other) != 1 {
		t.Errorf("ikinci PATCH aynı baskıyı güncellemeli, gönderilmeyen alana dokunmamalı: %+v", got)
	}
}
