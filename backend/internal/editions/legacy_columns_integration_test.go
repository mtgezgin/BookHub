package editions_test

import (
	"context"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/editions"
	"github.com/mtgezgin/bookhub/backend/internal/library"
	"github.com/mtgezgin/bookhub/backend/internal/moderation"
	"github.com/mtgezgin/bookhub/backend/internal/ratings"
	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

// TestCodeWorksWithoutLegacyBookColumns: "eski kolonlar yokmuş gibi" testi (Faz 16, Doğrulama 2).
//
// Genişletme PR'ının kodu books.cover_url ve books.page_count adını HİÇ anmamalı: daraltma PR'ı
// bu iki kolonu siler ve onları anan bir sorgu o an canlıda hata verir. sqlc `SELECT *`'ı üretilen
// kodda kolon kolon yazdığı için "kolonu kullanmıyorum" demek yetmez; kanıt şu: bir
// transaction'da iki kolon silinir, kitap okuyan ve yazan her fonksiyon o transaction'da çalışır,
// sonra her şey geri alınır. Bir sorgu eski kolonu anıyorsa burada kırmızı.
//
// Yeni bir kitap sorgusu eklenince buraya da bir çağrı eklenir.
func TestCodeWorksWithoutLegacyBookColumns(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	admin := testdb.CreateUser(t, pool, "Admin")
	reader := testdb.CreateUser(t, pool, "Okur")

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // kolonlar ve test verisi geri gelir / gider

	// Kilit beklemesi sınırlı: paralel koşan başka bir paketin testi books'u tutuyorsa sonsuza dek
	// beklemek yerine anlaşılır bir hatayla düşer.
	if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '10s'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `ALTER TABLE books DROP COLUMN cover_url, DROP COLUMN page_count`); err != nil {
		t.Fatalf("eski kolonlar silinemedi (görünüm onlara bağımlı olmamalı): %v", err)
	}

	step := func(name string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s eski kolonlar olmadan çalışmalı: %v", name, err)
		}
	}

	bookRepo := books.NewRepository(tx)
	mod := moderation.NewService(tx)
	ed := editions.NewService(tx)
	rat := ratings.NewService(tx, 1)
	lib := library.NewRepository(tx)

	slug := testdb.Unique("eski-kolon")
	pages := int32(300)
	created, err := mod.CreateApproved(ctx, admin, books.CreateParams{
		CreateBookParams: db.CreateBookParams{Title: "Eski Kolon " + slug, Slug: slug, Author: "Yazar", Language: "tr"},
		CoverURL:         "https://example.test/k.jpg",
		PageCount:        &pages,
	})
	step("moderation.CreateApproved (CreateBook + CreatePrimaryEdition + GetBook)", err)
	if created.CoverUrl != "https://example.test/k.jpg" || created.PageCount == nil || *created.PageCount != 300 {
		t.Errorf("kapak ve sayfa varsayılan baskıdan gelmeli: %+v", created)
	}

	suggested, err := mod.Suggest(ctx, reader, books.CreateParams{
		CreateBookParams: db.CreateBookParams{Title: "Öneri", Slug: testdb.Unique("oneri"), Author: "Yazar", Language: "tr"},
	})
	step("moderation.Suggest", err)
	_, err = mod.Approve(ctx, admin, suggested.ID)
	step("moderation.Approve (LockBookForModeration + SetBookStatus)", err)
	_, err = mod.Archive(ctx, admin, suggested.ID)
	step("moderation.Archive", err)
	_, err = mod.Restore(ctx, admin, suggested.ID)
	step("moderation.Restore", err)
	newCover := "https://example.test/yeni.jpg"
	_, err = mod.Update(ctx, admin, created.ID, books.UpdateParams{
		UpdateBookDetailsParams: db.UpdateBookDetailsParams{Title: &created.Title},
		CoverURL:                &newCover,
		PageCount:               &pages,
	})
	step("moderation.Update (UpdateBookDetails + PatchPrimaryEdition)", err)
	_, err = mod.Get(ctx, created.ID)
	step("moderation.Get (GetBook)", err)
	_, _, err = mod.List(ctx, books.StatusApproved, 5, 0)
	step("moderation.List (ListBooksByStatus)", err)
	_, err = mod.History(ctx, created.ID)
	step("moderation.History", err)

	_, err = bookRepo.Get(ctx, created.ID)
	step("books.Get (GetApprovedBook)", err)
	_, _, err = bookRepo.List(ctx, books.ListFilter{Query: "Eski Kolon", Limit: 5})
	step("books.List (ListBooks + CountBooks)", err)
	_, _, err = bookRepo.List(ctx, books.ListFilter{Sort: books.SortRating, MinVotes: 1, ColdStart: 5.5, Limit: 5})
	step("books.List sıralı (GlobalAverageRating + ListBooksByRating)", err)
	_, err = bookRepo.GenresForBook(ctx, created.ID)
	step("books.GenresForBook", err)
	err = bookRepo.SetGenres(ctx, created.ID, nil)
	step("books.SetGenres", err)

	_, err = rat.Rate(ctx, reader, created.ID, 8)
	step("ratings.Rate (LockApprovedBook + RefreshBookRatingStats)", err)
	_, err = rat.TopRated(ctx, 5)
	step("ratings.TopRated (ListTopRated)", err)
	_, err = rat.Summary(ctx, created.ID)
	step("ratings.Summary", err)
	_, err = rat.Remove(ctx, reader, created.ID)
	step("ratings.Remove", err)

	_, err = lib.Put(ctx, reader, created.ID, library.StatusRead)
	step("library.Put (UpsertLibraryEntry)", err)
	_, _, err = lib.List(ctx, reader, library.ListFilter{Limit: 5})
	step("library.List (ListLibrary)", err)

	e, err := ed.Create(ctx, admin, created.ID, editions.CreateParams{Title: "Original", Language: "en"})
	step("editions.Create", err)
	_, err = ed.Update(ctx, admin, e.ID, db.UpdateEditionParams{Publisher: &slug})
	step("editions.Update", err)
	_, err = ed.SetPrimary(ctx, admin, e.ID)
	step("editions.SetPrimary", err)
	_, err = ed.ListByBook(ctx, created.ID)
	step("editions.ListByBook", err)
	list, _ := ed.ListByBook(ctx, created.ID)
	err = ed.Delete(ctx, admin, list[len(list)-1].ID)
	step("editions.Delete", err)
}
