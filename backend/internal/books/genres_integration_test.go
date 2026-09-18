package books

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

func (f fixture) createGenre(t *testing.T) int64 {
	t.Helper()
	var id int64
	name := testdb.Unique("Tür")
	err := f.pool.QueryRow(context.Background(),
		"INSERT INTO genres (name, slug) VALUES ($1, $2) RETURNING id", name, testdb.Unique("tur")).Scan(&id)
	if err != nil {
		t.Fatalf("tür eklenemedi: %v", err)
	}
	t.Cleanup(func() { testdb.Exec(t, f.pool, "DELETE FROM genres WHERE id = $1", id) })
	return id
}

func genreIDs(list []db.Genre) []int64 {
	ids := make([]int64, 0, len(list))
	for _, g := range list {
		ids = append(ids, g.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func equalIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestSetGenresRealDB, çoka-çok ilişkinin replace semantiğini, atomikliğini ve hata
// çevirilerini gerçek Postgres'le doğrular.
func TestSetGenresRealDB(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	book := f.createBook(t, StatusApproved)
	g1, g2, g3 := f.createGenre(t), f.createGenre(t), f.createGenre(t)

	mustSet := func(ids ...int64) {
		t.Helper()
		if err := f.repo.SetGenres(ctx, book.ID, ids); err != nil {
			t.Fatalf("SetGenres(%v): %v", ids, err)
		}
	}
	current := func() []int64 {
		t.Helper()
		list, err := f.repo.GenresForBook(ctx, book.ID)
		if err != nil {
			t.Fatalf("GenresForBook: %v", err)
		}
		return genreIDs(list)
	}

	mustSet(g1, g2, g2) // tekrar eden id sessizce tekilleşir
	if got := current(); !equalIDs(got, []int64{g1, g2}) {
		t.Fatalf("ilk atama: istenen [%d %d], gelen %v", g1, g2, got)
	}

	mustSet(g3) // REPLACE: eskiler gider, yeni kalır (birleşim değil)
	if got := current(); !equalIDs(got, []int64{g3}) {
		t.Fatalf("değiştirme: istenen [%d], gelen %v", g3, got)
	}

	// Var olmayan tür: FK ihlali → ErrUnknownGenre VE hiçbir şey değişmemeli (rollback).
	err := f.repo.SetGenres(ctx, book.ID, []int64{g1, -42})
	if !errors.Is(err, ErrUnknownGenre) {
		t.Fatalf("olmayan tür: istenen ErrUnknownGenre, gelen %v", err)
	}
	if got := current(); !equalIDs(got, []int64{g3}) {
		t.Errorf("ATOMİKLİK: hata sonrası türler değişmemeliydi (istenen [%d]), gelen %v", g3, got)
	}

	if err := f.repo.SetGenres(ctx, -1, []int64{g1}); !errors.Is(err, ErrNotFound) {
		t.Errorf("olmayan kitap: istenen ErrNotFound, gelen %v", err)
	}

	mustSet() // boş liste = tüm türleri kaldır
	if got := current(); len(got) != 0 {
		t.Errorf("boş liste sonrası tür kalmamalıydı, gelen %v", got)
	}

	// Toplu okuma: iki kitap, tek sorgu, doğru gruplama.
	other := f.createBook(t, StatusApproved)
	mustSet(g1)
	if err := f.repo.SetGenres(ctx, other.ID, []int64{g2, g3}); err != nil {
		t.Fatal(err)
	}
	byBook, err := f.repo.GenresForBooks(ctx, []int64{book.ID, other.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !equalIDs(genreIDs(byBook[book.ID]), []int64{g1}) || !equalIDs(genreIDs(byBook[other.ID]), []int64{g2, g3}) {
		t.Errorf("toplu gruplama yanlış: %v", byBook)
	}
}

// TestSetGenresConcurrentReplace, iki eşzamanlı değiştirmenin sonunda türlerin
// listelerden BİRİ olduğunu (birleşimi DEĞİL) doğrular. LockBook'taki FOR UPDATE
// olmasaydı "sil + ekle" adımları iç içe geçip iki listenin birleşimi kalabilirdi.
func TestSetGenresConcurrentReplace(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	book := f.createBook(t, StatusApproved)
	a := []int64{f.createGenre(t), f.createGenre(t)}
	b := []int64{f.createGenre(t), f.createGenre(t)}
	sort.Slice(a, func(i, j int) bool { return a[i] < a[j] })
	sort.Slice(b, func(i, j int) bool { return b[i] < b[j] })

	for i := 0; i < 20; i++ {
		var wg sync.WaitGroup
		for _, ids := range [][]int64{a, b} {
			wg.Add(1)
			go func(ids []int64) {
				defer wg.Done()
				if err := f.repo.SetGenres(ctx, book.ID, ids); err != nil {
					t.Errorf("SetGenres: %v", err)
				}
			}(ids)
		}
		wg.Wait()

		list, err := f.repo.GenresForBook(ctx, book.ID)
		if err != nil {
			t.Fatal(err)
		}
		got := genreIDs(list)
		if !equalIDs(got, a) && !equalIDs(got, b) {
			t.Fatalf("tur %d: sonuç iki listeden biri olmalıydı (%v veya %v), gelen %v", i, a, b, got)
		}
	}
}

// TestListFiltersRealDB, liste ucunun tür filtresi, metin araması ve sayfalamasını gerçek
// Postgres'le doğrular. Aramalar bu teste özgü benzersiz bir işaretle yapılır ki DB'deki
// başka satırlar sonucu kirletmesin.
func TestListFiltersRealDB(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	marker := testdb.Unique("isaret")

	mk := func(title, author, status string) db.Book {
		t.Helper()
		b, err := f.repo.Create(ctx, db.CreateBookParams{
			Title: title, Slug: testdb.Unique("liste"), Author: author, Language: "tr", Status: status,
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { testdb.Exec(t, f.pool, "DELETE FROM books WHERE id = $1", b.ID) })
		return b
	}
	dune := mk("Dune "+marker, "Frank Herbert", StatusApproved)
	foundation := mk("Vakıf "+marker, "Isaac Asimov", StatusApproved)
	mk("Gizli "+marker, "Frank Herbert", StatusPending) // görünmemeli
	percent := mk("%100 Gerçek "+marker, "Anonim", StatusApproved)

	genre := f.createGenre(t)
	if err := f.repo.SetGenres(ctx, dune.ID, []int64{genre}); err != nil {
		t.Fatal(err)
	}
	var genreSlug string
	if err := f.pool.QueryRow(ctx, "SELECT slug FROM genres WHERE id=$1", genre).Scan(&genreSlug); err != nil {
		t.Fatal(err)
	}

	ids := func(list []db.Book) []int64 {
		out := make([]int64, 0, len(list))
		for _, b := range list {
			out = append(out, b.ID)
		}
		sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
		return out
	}
	sorted := func(v ...int64) []int64 { sort.Slice(v, func(i, j int) bool { return v[i] < v[j] }); return v }

	tests := []struct {
		name      string
		filter    ListFilter
		wantIDs   []int64
		wantTotal int64
	}{
		{"arama: başlıkta işaret → onaylılar (PENDING hariç)", ListFilter{Query: marker, Limit: 50}, sorted(dune.ID, foundation.ID, percent.ID), 3},
		{"arama: yazar adı, büyük/küçük harf duyarsız", ListFilter{Query: "frank herbert", Limit: 50}, nil, -1},
		{"tür filtresi", ListFilter{Genre: genreSlug, Limit: 50}, []int64{dune.ID}, 1},
		{"% joker DEĞİL, düz karakter", ListFilter{Query: "%100 Gerçek " + marker, Limit: 50}, []int64{percent.ID}, 1},
		{"sayfalama: limit 2 → 2 kayıt ama toplam 3", ListFilter{Query: marker, Limit: 2}, nil, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list, total, err := f.repo.List(ctx, tt.filter)
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantIDs != nil && !equalIDs(ids(list), tt.wantIDs) {
				t.Errorf("id'ler: istenen %v, gelen %v", tt.wantIDs, ids(list))
			}
			if tt.wantTotal >= 0 && total != tt.wantTotal {
				t.Errorf("toplam: istenen %d, gelen %d", tt.wantTotal, total)
			}
			if tt.filter.Limit == 2 && len(list) != 2 {
				t.Errorf("limit 2: istenen 2 kayıt, gelen %d", len(list))
			}
			for _, b := range list {
				if b.Status != StatusApproved {
					t.Errorf("onaylanmamış kitap listede: %+v", b)
				}
			}
		})
	}

	// Yazar araması: bizim onaylı Dune'umuz sonuçta olmalı, PENDING kopyası olmamalı.
	list, _, err := f.repo.List(ctx, ListFilter{Query: "frank herbert", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, b := range list {
		found = found || b.ID == dune.ID
	}
	if !found {
		t.Error("yazar araması (küçük harf) Dune'u bulamadı")
	}
}
