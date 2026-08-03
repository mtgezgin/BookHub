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
// row  → QueryRow ile giden sorgular (:one → Get, Create)
// rows → Query ile giden sorgular (:many → List); yalnızca hata yolu için.
type fakeDBTX struct {
	row      fakeRow
	queryErr error
}

func (f fakeDBTX) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("Exec bu testte kullanılmıyor")
}

func (f fakeDBTX) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	if f.queryErr != nil {
		return nil, f.queryErr
	}
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

// TestRepositoryList_HataSarmalama, List'in hata yolunu doğrular. Get'ten
// KASITLI farkı: burada ErrNotFound çevirisi YOKTUR — ":many" sorgusu satır
// bulamazsa bu bir hata değil, boş listedir. Dolayısıyla List'ten dönen her hata
// GERÇEK bir hatadır ve sarılarak (%w) yukarı verilir; ErrNotFound'a dönüşmez.
// Ayrıca hata yolunda slice'ın nil dönmesi beklenir (zero value; çağıran zaten
// önce err'e bakar — boş slice döndürmek "sonuç var ama boş" yanılgısı yaratırdı).
func TestRepositoryList_HataSarmalama(t *testing.T) {
	errBaglanti := errors.New("bağlantı koptu: çok gizli sürücü detayı")

	repo := NewRepository(fakeDBTX{queryErr: errBaglanti})

	got, err := repo.List(context.Background())

	if err == nil {
		t.Fatalf("hata bekleniyordu ama gelmedi (gelen liste: %+v)", got)
	}
	if !errors.Is(err, errBaglanti) {
		t.Errorf("hata zinciri kopmuş: alttaki sürücü hatası taşınmalıydı, gelen %v", err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Errorf("List hatası ErrNotFound'a çözülmemeli (boş sonuç ≠ hata), gelen %v", err)
	}
	if !strings.Contains(err.Error(), "kitap listeleme sorgusu") {
		t.Errorf("hata metni bağlam içermeliydi (\"kitap listeleme sorgusu\"), gelen %q", err.Error())
	}
	if got != nil {
		t.Errorf("hata yolunda nil slice dönmeliydi, gelen %+v", got)
	}
}

// TestRepositoryCreate_HataSarmalama, Create'in hata yolunu doğrular.
// En önemli iddia sondaki senaryo: pgx.ErrNoRows Create'te ErrNotFound'a
// ÇEVRİLMEZ. Çeviri Get'e özgü bir domain kararıdır ("aradığın kayıt yok");
// bir INSERT'ten satır dönmemesi ise beklenmeyen bir durumdur ve öyle raporlanmalı.
func TestRepositoryCreate_HataSarmalama(t *testing.T) {
	errKisit := errors.New("duplicate key value violates unique constraint \"books_slug_key\"")

	tests := []struct {
		name      string
		rowErr    error
		wantErrIs error // errors.Is ile YAKALANMALI
	}{
		{
			name:      "sürücü hatası → %w ile sarılır, zincir korunur",
			rowErr:    errKisit,
			wantErrIs: errKisit,
		},
		{
			name:      "pgx.ErrNoRows → ErrNotFound'a DÖNÜŞMEZ (çeviri Get'e özgü)",
			rowErr:    pgx.ErrNoRows,
			wantErrIs: pgx.ErrNoRows,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewRepository(fakeDBTX{row: fakeRow{err: tt.rowErr}})

			got, err := repo.Create(context.Background(), db.CreateBookParams{
				Title:  "Yeni Kitap",
				Slug:   "yeni-kitap",
				Author: "Yazar",
				Status: "APPROVED",
			})

			if err == nil {
				t.Fatalf("hata bekleniyordu ama gelmedi (gelen kitap: %+v)", got)
			}
			if !errors.Is(err, tt.wantErrIs) {
				t.Errorf("hata zinciri: %v taşınmalıydı, gelen %v", tt.wantErrIs, err)
			}
			if errors.Is(err, ErrNotFound) {
				t.Errorf("Create hatası ErrNotFound'a çözülmemeli, gelen %v", err)
			}
			if !strings.Contains(err.Error(), "kitap ekleme sorgusu") {
				t.Errorf("hata metni bağlam içermeliydi (\"kitap ekleme sorgusu\"), gelen %q", err.Error())
			}
			if got != (db.Book{}) {
				t.Errorf("hata yolunda zero-value Book dönmeliydi, gelen %+v", got)
			}
		})
	}
}
