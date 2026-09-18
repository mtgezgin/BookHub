package genres

import (
	"context"
	"errors"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

// TestRepositoryRealDB, türlerin CRUD döngüsünü ve İKİ ayrı UNIQUE kısıtın (ad, slug)
// doğru domain hatasına ayrıştığını gerçek Postgres'le doğrular.
func TestRepositoryRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	name, slug := testdb.Unique("Tür"), testdb.Unique("tur")
	g, err := repo.Create(ctx, name, slug)
	if err != nil {
		t.Fatalf("tür eklenemedi: %v", err)
	}
	t.Cleanup(func() { testdb.Exec(t, pool, "DELETE FROM genres WHERE id = $1", g.ID) })

	t.Run("aynı ad → ErrDuplicateName", func(t *testing.T) {
		_, err := repo.Create(ctx, name, testdb.Unique("baska"))
		if !errors.Is(err, ErrDuplicateName) {
			t.Fatalf("istenen ErrDuplicateName, gelen %v", err)
		}
	})
	t.Run("aynı slug → ErrDuplicateSlug", func(t *testing.T) {
		_, err := repo.Create(ctx, testdb.Unique("Başka"), slug)
		if !errors.Is(err, ErrDuplicateSlug) {
			t.Fatalf("istenen ErrDuplicateSlug, gelen %v", err)
		}
	})
	t.Run("kısmi güncelleme: yalnız ad değişir, slug korunur", func(t *testing.T) {
		newName := testdb.Unique("Yeni")
		got, err := repo.Update(ctx, g.ID, &newName, nil)
		if err != nil {
			t.Fatalf("güncellenemedi: %v", err)
		}
		if got.Name != newName || got.Slug != slug {
			t.Errorf("istenen ad=%q slug=%q, gelen %+v", newName, slug, got)
		}
	})
	t.Run("olmayan türü güncelle/sil → ErrNotFound", func(t *testing.T) {
		n := "x"
		if _, err := repo.Update(ctx, -1, &n, nil); !errors.Is(err, ErrNotFound) {
			t.Errorf("Update: istenen ErrNotFound, gelen %v", err)
		}
		if err := repo.Delete(ctx, -1); !errors.Is(err, ErrNotFound) {
			t.Errorf("Delete: istenen ErrNotFound, gelen %v", err)
		}
	})
	t.Run("sil → artık yok", func(t *testing.T) {
		if err := repo.Delete(ctx, g.ID); err != nil {
			t.Fatalf("silinemedi: %v", err)
		}
		if _, err := repo.Get(ctx, g.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("silinen tür hâlâ okunuyor: %v", err)
		}
	})
}
