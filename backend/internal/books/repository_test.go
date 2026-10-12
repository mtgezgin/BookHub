package books

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mtgezgin/bookhub/backend/internal/database"
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

// TestRepositoryGetErrorTranslation, Get'in SÜRÜCÜ hatasını domain sınırından
// geçirmediğini doğrular: pgx.ErrNoRows → ErrNotFound (sarılmadan, tanınabilir),
// diğer hatalar ise %w ile bağlam eklenerek sarılır (zincir korunur, ErrNotFound'a
// dönüşmez). Görünürlük sınırının (status='APPROVED') kendisi SQL'de yaşıyor;
// onu repository_integration_test.go gerçek DB'ye karşı kanıtlar.
func TestRepositoryGetErrorTranslation(t *testing.T) {
	errConn := errors.New("bağlantı koptu: çok gizli sürücü detayı")

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
			row:             fakeRow{err: errConn},
			wantErrIs:       errConn,
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

// TestRepositoryListErrorWrapping, List'in hata yolunu doğrular. Get'ten
// KASITLI farkı: burada ErrNotFound çevirisi YOKTUR — ":many" sorgusu satır
// bulamazsa bu bir hata değil, boş listedir. Dolayısıyla List'ten dönen her hata
// GERÇEK bir hatadır ve sarılarak (%w) yukarı verilir; ErrNotFound'a dönüşmez.
// Ayrıca hata yolunda slice'ın nil dönmesi beklenir (zero value; çağıran zaten
// önce err'e bakar — boş slice döndürmek "sonuç var ama boş" yanılgısı yaratırdı).
func TestRepositoryListErrorWrapping(t *testing.T) {
	errConn := errors.New("bağlantı koptu: çok gizli sürücü detayı")

	repo := NewRepository(fakeDBTX{queryErr: errConn})

	got, _, err := repo.List(context.Background(), ListFilter{Limit: 20})

	if err == nil {
		t.Fatalf("hata bekleniyordu ama gelmedi (gelen liste: %+v)", got)
	}
	if !errors.Is(err, errConn) {
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

// TestRepositoryCreateErrors, Create'in hata çevirisini doğrular. Kritik iki iddia:
//
//  1. Postgres 23505 (unique_violation) → ErrDuplicateSlug, SARILMADAN. Handler bunu
//     409'a çevirir; bu çeviri kırılırsa istemci hatası 500 olarak görünür.
//     DİKKAT: düz bir errors.New("duplicate key ...") bu dalı TETİKLEMEZ — repository
//     errors.As ile *pgconn.PgError arar. Gerçek sürücü hatası tipini kurmak şart;
//     aksi halde test kapsıyormuş gibi görünür ama eski dalı çalıştırır.
//  2. pgx.ErrNoRows Create'te ErrNotFound'a ÇEVRİLMEZ: o çeviri Get'e özgü bir domain
//     kararıdır ("aradığın yok"); bir INSERT'ten satır dönmemesi beklenmeyen bir durumdur.
func TestRepositoryCreateErrors(t *testing.T) {
	errConn := errors.New("bağlantı koptu")

	tests := []struct {
		name          string
		rowErr        error
		wantErrIs     error
		wantDuplicate bool
		wantWrapped   bool // "kitap ekleme sorgusu" bağlamıyla sarılmış olmalı
	}{
		{
			name:          "gerçek 23505 PgError → ErrDuplicateSlug (sarılmadan)",
			rowErr:        &pgconn.PgError{Code: "23505", ConstraintName: "books_slug_key"},
			wantErrIs:     ErrDuplicateSlug,
			wantDuplicate: true,
		},
		{
			name:        "başka bir PgError (23503 FK ihlali) → çakışma DEĞİL, sarılır",
			rowErr:      &pgconn.PgError{Code: "23503"},
			wantWrapped: true,
		},
		{
			name:        "sürücü hatası → %w ile sarılır, zincir korunur",
			rowErr:      errConn,
			wantErrIs:   errConn,
			wantWrapped: true,
		},
		{
			name:        "pgx.ErrNoRows → ErrNotFound'a DÖNÜŞMEZ",
			rowErr:      pgx.ErrNoRows,
			wantErrIs:   pgx.ErrNoRows,
			wantWrapped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewRepository(fakeDBTX{row: fakeRow{err: tt.rowErr}})

			got, err := repo.Create(context.Background(), db.CreateBookParams{
				Title: "Yeni Kitap", Slug: "yeni-kitap", Author: "Yazar", Status: StatusApproved,
			})

			if err == nil {
				t.Fatalf("hata bekleniyordu ama gelmedi (gelen kitap: %+v)", got)
			}
			if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Errorf("hata zinciri: %v taşınmalıydı, gelen %v", tt.wantErrIs, err)
			}
			if errors.Is(err, ErrDuplicateSlug) != tt.wantDuplicate {
				t.Errorf("ErrDuplicateSlug: istenen %v, gelen hata %v", tt.wantDuplicate, err)
			}
			if errors.Is(err, ErrNotFound) {
				t.Errorf("Create hatası ErrNotFound'a çözülmemeli, gelen %v", err)
			}
			if strings.Contains(err.Error(), "kitap ekleme sorgusu") != tt.wantWrapped {
				t.Errorf("bağlam sarması: istenen %v, gelen %q", tt.wantWrapped, err.Error())
			}
			if got != (db.Book{}) {
				t.Errorf("hata yolunda zero-value Book dönmeliydi, gelen %+v", got)
			}
		})
	}
}

// TestRepositorySetGenresNoTx: fakeDBTX Begin sağlamaz → SetGenres, WithTx'in ErrNoTx'ini
// döner; türler transaction'sız yazılmaya çalışılmaz.
func TestRepositorySetGenresNoTx(t *testing.T) {
	err := NewRepository(fakeDBTX{}).SetGenres(context.Background(), 1, []int64{2})
	if !errors.Is(err, database.ErrNoTx) {
		t.Errorf("database.ErrNoTx bekleniyordu, gelen %v", err)
	}
}

// TestRepositoryGenresForBooksEmpty: boş id listesi SORGU ATMADAN boş, nil olmayan bir
// map döner. fakeDBTX'in Query'si hata dönecek şekilde kurulu: sorgu atılsaydı hata görünürdü.
func TestRepositoryGenresForBooksEmpty(t *testing.T) {
	repo := NewRepository(fakeDBTX{queryErr: errors.New("sorgu atılmamalıydı")})

	for _, ids := range [][]int64{nil, {}} {
		got, err := repo.GenresForBooks(context.Background(), ids)
		if err != nil {
			t.Fatalf("boş liste için sorgu atılmamalıydı, gelen hata: %v", err)
		}
		if got == nil {
			t.Error("nil değil, boş map dönmeliydi")
		}
		if len(got) != 0 {
			t.Errorf("boş map bekleniyordu, gelen %v", got)
		}
	}
}

// emptyRows, sıfır satırlık bir pgx.Rows: ListBooks başarılı (boş) dönsün ki hata
// ikinci sorguda (CountBooks) tetiklenebilsin.
type emptyRows struct{}

func (emptyRows) Close()                                       {}
func (emptyRows) Err() error                                   { return nil }
func (emptyRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (emptyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (emptyRows) Next() bool                                   { return false }
func (emptyRows) Scan(...any) error                            { return errors.New("satır yok") }
func (emptyRows) Values() ([]any, error)                       { return nil, nil }
func (emptyRows) RawValues() [][]byte                          { return nil }
func (emptyRows) Conn() *pgx.Conn                              { return nil }

// countFailDBTX: liste sorgusu boş döner, sayım sorgusu (QueryRow) row.err ile patlar.
type countFailDBTX struct{ fakeDBTX }

func (countFailDBTX) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return emptyRows{}, nil
}

// TestRepositoryListCountErrorWrapping: ikinci sorgu (CountBooks) hata verirse List
// sayım bağlamıyla sarılı hata döner ve liste de nil olur.
func TestRepositoryListCountErrorWrapping(t *testing.T) {
	errConn := errors.New("bağlantı koptu")
	repo := NewRepository(countFailDBTX{fakeDBTX{row: fakeRow{err: errConn}}})

	got, total, err := repo.List(context.Background(), ListFilter{Limit: 20})
	if !errors.Is(err, errConn) {
		t.Fatalf("sarılı sürücü hatası bekleniyordu, gelen %v", err)
	}
	if !strings.Contains(err.Error(), "kitap sayma sorgusu") {
		t.Errorf("hata metni \"kitap sayma sorgusu\" bağlamını içermeli, gelen %q", err.Error())
	}
	if got != nil || total != 0 {
		t.Errorf("hata yolunda nil liste ve 0 toplam bekleniyordu, gelen %+v, %d", got, total)
	}
}

// TestRepositoryListRatingAverageError: puana göre sıralamada önce genel ortalama okunur;
// o sorgu (QueryRow) patlarsa List liste sorgusuna hiç geçmeden, "genel ortalama sorgusu"
// bağlamıyla sarılı hatayı döner. Query ayrı bir hata döndürecek şekilde kurulu: liste
// sorgusu atılsaydı gelen hata errConn değil o olurdu.
func TestRepositoryListRatingAverageError(t *testing.T) {
	errConn := errors.New("bağlantı koptu")
	errList := errors.New("liste sorgusu atılmamalıydı")
	repo := NewRepository(fakeDBTX{row: fakeRow{err: errConn}, queryErr: errList})

	got, total, err := repo.List(context.Background(), ListFilter{Sort: SortRating, MinVotes: 5, ColdStart: 3, Limit: 20})
	if !errors.Is(err, errConn) {
		t.Fatalf("genel ortalama sorgusunun sarılı hatası bekleniyordu, gelen %v", err)
	}
	if errors.Is(err, errList) {
		t.Errorf("ortalama hatasından sonra liste sorgusu atılmamalıydı, gelen %v", err)
	}
	if !strings.Contains(err.Error(), "genel ortalama sorgusu") {
		t.Errorf("hata metni \"genel ortalama sorgusu\" bağlamını içermeli, gelen %q", err.Error())
	}
	if got != nil || total != 0 {
		t.Errorf("hata yolunda nil liste ve 0 toplam bekleniyordu, gelen %+v, %d", got, total)
	}
}
