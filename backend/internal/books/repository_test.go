package books

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// fakeRow, pgx.Row'un SAHTE (fake) uygulaması: Scan çağrılınca ya söylenen hatayı
// döner ya da söylenen kitabı doldurur. Amaç DB'yi taklit etmek değil —
// Get'in HATA ÇEVİRİSİNİ (sürücü dili → domain dili) DB'siz kanıtlamak.
type fakeRow struct {
	err  error
	book db.Book
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	// Alan sırası sqlc'nin ürettiği Scan sırasıdır (id, title, ...);
	// bu test yalnızca ilk ikisinin geçtiğini doğrulamak için yeterince doldurur.
	if len(dest) > 0 {
		if p, ok := dest[0].(*int64); ok {
			*p = r.book.ID
		}
	}
	if len(dest) > 1 {
		if p, ok := dest[1].(*string); ok {
			*p = r.book.Title
		}
	}
	return nil
}

// fakeDBTX, sqlc'nin ürettiği db.DBTX interface'inin sahtesi. Repository bunu
// kabul ettiği için (bkz. NewRepository) araya gerçek havuz olmadan girebiliyoruz.
type fakeDBTX struct {
	row fakeRow
}

func (f fakeDBTX) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("Exec bu testte kullanılmıyor")
}

func (f fakeDBTX) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("Query bu testte kullanılmıyor")
}

func (f fakeDBTX) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return f.row
}

// TestRepositoryGet_HataCevirisi, Get'in SÜRÜCÜ hatasını domain sınırından
// geçirmediğini doğrular: pgx.ErrNoRows → ErrNotFound (sarılmadan, tanınabilir),
// diğer hatalar ise %w ile bağlam eklenerek sarılır (zincir korunur, ErrNotFound'a
// dönüşmez). Görünürlük sınırının (status='APPROVED') kendisi SQL'de yaşıyor;
// onu repository_integration_test.go gerçek DB'ye karşı kanıtlar.
func TestRepositoryGet_HataCevirisi(t *testing.T) {
	errBaglanti := errors.New("bağlantı koptu: çok gizli sürücü detayı")

	tests := []struct {
		name            string
		row             fakeRow
		wantErrIs       error  // errors.Is ile YAKALANMALI (nil = hata beklenmiyor)
		notErrIs        error  // errors.Is ile YAKALANMAMALI
		wantErrContains string // hata metninde geçmesi beklenen bağlam
		wantID          int64  // başarı durumunda beklenen kitap id'si
	}{
		{
			name:      "satır yok → ErrNotFound (pgx hatası domain'e geçmez)",
			row:       fakeRow{err: pgx.ErrNoRows},
			wantErrIs: ErrNotFound,
			notErrIs:  pgx.ErrNoRows,
		},
		{
			name:            "beklenmeyen hata → %w ile sarılır, ErrNotFound'a dönüşmez",
			row:             fakeRow{err: errBaglanti},
			wantErrIs:       errBaglanti,
			notErrIs:        ErrNotFound,
			wantErrContains: "id=42",
		},
		{
			name:   "satır var → kitap döner",
			row:    fakeRow{book: db.Book{ID: 42, Title: "Clean Code"}},
			wantID: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewRepository(fakeDBTX{row: tt.row})

			got, err := repo.Get(context.Background(), 42)

			if tt.wantErrIs == nil {
				if err != nil {
					t.Fatalf("beklenmeyen hata: %v", err)
				}
				if got.ID != tt.wantID {
					t.Errorf("kitap id: istenen %d, gelen %d", tt.wantID, got.ID)
				}
				return
			}

			if !errors.Is(err, tt.wantErrIs) {
				t.Fatalf("hata zinciri: %v hatasını taşıması isteniyordu, gelen %v", tt.wantErrIs, err)
			}
			if tt.notErrIs != nil && errors.Is(err, tt.notErrIs) {
				t.Errorf("hata %v'a çözülmemeliydi (sınır sızdırıyor), gelen %v", tt.notErrIs, err)
			}
			if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Errorf("hata metni %q içermeliydi, gelen %q", tt.wantErrContains, err.Error())
			}
		})
	}
}
