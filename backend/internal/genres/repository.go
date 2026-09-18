// Package genres, tür (genre) domain'inin veri erişim katmanı.
package genres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mtgezgin/bookhub/backend/internal/database"
	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// Domain hataları — handler bunları HTTP koduna çevirir; pgx'i tanımaz.
var (
	ErrNotFound      = errors.New("tür bulunamadı")
	ErrDuplicateName = errors.New("bu tür adı zaten kullanılıyor")
	ErrDuplicateSlug = errors.New("bu tür slug'ı zaten kullanılıyor")
)

// Kısıt adları migration'daki UNIQUE tanımlarından gelir (Postgres "<tablo>_<kolon>_key"
// diye adlandırır). genres tablosunda İKİ benzersiz kolon olduğu için 23505 tek başına
// yetmez: hangi kısıtın çakıştığına bakıp doğru mesajı veriyoruz.
const (
	constraintName = "genres_name_key"
	constraintSlug = "genres_slug_key"
)

// Repository, türlere erişimin tek kapısı (bkz. books.Repository — aynı desen).
type Repository struct {
	q *db.Queries
}

func NewRepository(dbtx db.DBTX) *Repository {
	return &Repository{q: db.New(dbtx)}
}

// List, tüm türleri ada göre sıralı döner (tür sayısı küçük ve sınırlı; sayfalama gereksiz).
func (r *Repository) List(ctx context.Context) ([]db.Genre, error) {
	list, err := r.q.ListGenres(ctx)
	if err != nil {
		return nil, fmt.Errorf("tür listeleme sorgusu: %w", err)
	}
	return list, nil
}

func (r *Repository) Get(ctx context.Context, id int64) (db.Genre, error) {
	g, err := r.q.GetGenre(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Genre{}, ErrNotFound
	}
	if err != nil {
		return db.Genre{}, fmt.Errorf("tür sorgusu (id=%d): %w", id, err)
	}
	return g, nil
}

func (r *Repository) Create(ctx context.Context, name, slug string) (db.Genre, error) {
	g, err := r.q.CreateGenre(ctx, db.CreateGenreParams{Name: name, Slug: slug})
	if err != nil {
		return db.Genre{}, translateWriteErr(err, "tür ekleme sorgusu")
	}
	return g, nil
}

// Update, yalnızca nil OLMAYAN alanları değiştirir (kısmi güncelleme; COALESCE SQL'de).
func (r *Repository) Update(ctx context.Context, id int64, name, slug *string) (db.Genre, error) {
	g, err := r.q.UpdateGenre(ctx, db.UpdateGenreParams{ID: id, Name: name, Slug: slug})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Genre{}, ErrNotFound
	}
	if err != nil {
		return db.Genre{}, translateWriteErr(err, fmt.Sprintf("tür güncelleme sorgusu (id=%d)", id))
	}
	return g, nil
}

// Delete, türü siler; kitaplarla bağları FK CASCADE ile gider. Yoksa ErrNotFound.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	n, err := r.q.DeleteGenre(ctx, id)
	if err != nil {
		return fmt.Errorf("tür silme sorgusu (id=%d): %w", id, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// translateWriteErr, benzersizlik ihlallerini hangi kolonun çakıştığına göre domain
// hatasına çevirir; gerisini bağlamla sarar.
func translateWriteErr(err error, op string) error {
	if c, ok := database.UniqueViolation(err); ok {
		switch c {
		case constraintName:
			return ErrDuplicateName
		case constraintSlug:
			return ErrDuplicateSlug
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}
