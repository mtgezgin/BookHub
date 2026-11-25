package reviews

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

// fixture, her testin ortak kurulumu: yayında bir kitap, iki okur ve bir yönetici.
type fixture struct {
	t                    *testing.T
	ctx                  context.Context
	pool                 *pgxpool.Pool
	s                    *Service
	book                 int64
	reader, other, admin int64
}

func setup(t *testing.T) fixture {
	pool := testdb.Pool(t)
	return fixture{
		t: t, ctx: context.Background(), pool: pool, s: NewService(pool),
		book:   testdb.CreateBook(t, pool, "", books.StatusApproved),
		reader: testdb.CreateUser(t, pool, "Okur"),
		other:  testdb.CreateUser(t, pool, "İkinci Okur"),
		admin:  testdb.CreateUser(t, pool, "Yönetici"),
	}
}

// Testlerde birden çok yerde geçen metinler.
const (
	hideReason = "Kişisel bilgi içeriyor"
	editedBody = "Düzenlenmiş metin"
)

func ver(v int32) *int32 { return &v }

// mustSave, beklenen başarılı yazım; hata testi düşürür.
func (f fixture) mustSave(user int64, body string, version *int32) db.Review {
	f.t.Helper()
	r, err := f.s.Save(f.ctx, user, f.book, Draft{Body: body, Version: version})
	if err != nil {
		f.t.Fatalf("Save(%q): %v", body, err)
	}
	return r
}

// expectErr, err'in want olmasını ister (want nil = hata yok).
func expectErr(t *testing.T, what string, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Errorf("%s: hata %v, beklenen %v", what, err, want)
	}
}

// auditCount, incelemenin verilen eylemle kaç audit satırı olduğu.
func (f fixture) auditCount(reviewID int64, action string) int {
	f.t.Helper()
	var n int
	err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM audit_logs
		WHERE entity_type = 'review' AND entity_id = $1 AND action = $2`, reviewID, action).Scan(&n)
	if err != nil {
		f.t.Fatalf("audit sayılamadı: %v", err)
	}
	return n
}

// TestSaveRealDB: tek uçta yazım ve düzenleme, sürüm kuralı (Doğrulama 12). Adımlar sırayla aynı
// incelemeye uygulanır; her adım beklenen hatayı ya da sonuçtaki sürümü ve "düzenlendi"yi sınar.
func TestSaveRealDB(t *testing.T) {
	f := setup(t)
	for _, c := range []struct {
		name       string
		draft      Draft
		wantErr    error
		wantVer    int32
		wantEdited bool
	}{
		{"ilk yazım", Draft{Body: "İlk"}, nil, 1, false},
		{"sürümsüz ikinci yazım (başka sekme az önce yazdı)", Draft{Body: "Ezme"}, ErrVersionConflict, 0, false},
		{"eski sürüm", Draft{Body: "Ezme", Version: ver(0)}, ErrVersionConflict, 0, false},
		{"aynı metin ve bayrak yazılmaz", Draft{Body: "İlk", Version: ver(1)}, nil, 1, false},
		{"metin düzenleme", Draft{Body: editedBody, Version: ver(1)}, nil, 2, true},
		{"yalnız spoiler bayrağı", Draft{Body: editedBody, HasSpoiler: true, Version: ver(2)}, nil, 3, true},
	} {
		r, err := f.s.Save(f.ctx, f.reader, f.book, c.draft)
		expectErr(t, c.name, err, c.wantErr)
		if err == nil && (r.Version != c.wantVer || (r.EditedAt != nil) != c.wantEdited) {
			t.Errorf("%s: sürüm %d düzenlendi=%v, beklenen %d %v", c.name, r.Version, r.EditedAt != nil, c.wantVer, c.wantEdited)
		}
	}

	mine, err := f.s.Mine(f.ctx, f.reader, f.book)
	if err != nil || mine.Body != editedBody || !mine.HasSpoiler {
		t.Errorf("409'lar incelemeyi değiştirmemeli: %+v %v", mine, err)
	}
	if r := f.mustSave(f.other, "Bence de", nil); r.Version != 1 {
		t.Errorf("başka okurun aynı kitaba ilk incelemesi sürüm 1 olmalı: %d", r.Version)
	}
	_, err = f.s.Mine(f.ctx, f.admin, f.book)
	expectErr(t, "incelemesi olmayanın Mine'ı", err, ErrNotFound)
}

// TestUnpublishedBookRealDB: yayında olmayan kitapta liste, okuma ve yazım "kitap yok".
func TestUnpublishedBookRealDB(t *testing.T) {
	f := setup(t)
	for _, st := range []string{books.StatusPending, books.StatusRejected, books.StatusArchived} {
		book := testdb.CreateBook(t, f.pool, "", st)
		_, err := f.s.Save(f.ctx, f.reader, book, Draft{Body: "Merhaba"})
		expectErr(t, st+" kitaba yazım", err, books.ErrNotFound)
		_, err = f.s.Mine(f.ctx, f.reader, book)
		expectErr(t, st+" kitapta Mine", err, books.ErrNotFound)
		_, _, err = f.s.List(f.ctx, book, 20, 0)
		expectErr(t, st+" kitapta List", err, books.ErrNotFound)
	}
}

// TestDeleteRealDB: silme yalnız okurun kendi incelemesini siler, puanı silmez (Doğrulama 13, 14);
// silinmiş incelemeye sürümlü yazım 409; yayından kalkmış kitapta da silinebilir.
func TestDeleteRealDB(t *testing.T) {
	f := setup(t)
	testdb.Exec(t, f.pool, `INSERT INTO ratings (user_id, book_id, score) VALUES ($1, $2, 8)`, f.reader, f.book)
	f.mustSave(f.reader, "Silinecek", nil)
	f.mustSave(f.other, "Kalacak", nil)

	expectErr(t, "silme", f.s.Delete(f.ctx, f.reader, f.book), nil)
	expectErr(t, "ikinci silme", f.s.Delete(f.ctx, f.reader, f.book), ErrNotFound)
	_, err := f.s.Save(f.ctx, f.reader, f.book, Draft{Body: "Eski sekme", Version: ver(1)})
	expectErr(t, "silinmiş incelemeye sürümlü yazım (başka sekme sildi)", err, ErrVersionConflict)

	again := f.mustSave(f.reader, "Yeniden", nil)
	testdb.Exec(t, f.pool, `UPDATE books SET status = 'ARCHIVED' WHERE id = $1`, f.book)
	_, err = f.s.Save(f.ctx, f.reader, f.book, Draft{Body: "Düzenleme", Version: ver(again.Version)})
	expectErr(t, "yayından kalkmış kitapta düzenleme", err, books.ErrNotFound)
	expectErr(t, "yayından kalkmış kitapta silme", f.s.Delete(f.ctx, f.reader, f.book), nil)

	var reviews, ratings int
	err = f.pool.QueryRow(f.ctx, `SELECT (SELECT count(*) FROM reviews WHERE book_id = $1),
		(SELECT count(*) FROM ratings WHERE book_id = $1)`, f.book).Scan(&reviews, &ratings)
	if err != nil || reviews != 1 || ratings != 1 {
		t.Errorf("başkasının incelemesi ve okurun puanı kalmalı: inceleme %d, puan %d, %v", reviews, ratings, err)
	}
}

// TestListRealDB: herkese açık liste en yeni önce; yazarın adı ve (varsa) puanı.
func TestListRealDB(t *testing.T) {
	f := setup(t)
	testdb.Exec(t, f.pool, `INSERT INTO ratings (user_id, book_id, score) VALUES ($1, $2, 8)`, f.reader, f.book)
	f.mustSave(f.reader, "Önce yazılan", nil)
	f.mustSave(f.other, "Sonra yazılan", nil)

	list, total, err := f.s.List(f.ctx, f.book, 20, 0)
	if err != nil || total != 2 || len(list) != 2 {
		t.Fatalf("List: %d/%d %v", len(list), total, err)
	}
	newest, oldest := list[0], list[1]
	if newest.Body != "Sonra yazılan" || newest.AuthorName != "İkinci Okur" || newest.AuthorScore != nil {
		t.Errorf("en yeni önce, puansız yazar: %+v", newest)
	}
	if oldest.AuthorName != "Okur" || oldest.AuthorScore == nil || *oldest.AuthorScore != 8 {
		t.Errorf("yazarın puanı ratings'ten: %+v", oldest)
	}
	if page, _, _ := f.s.List(f.ctx, f.book, 1, 1); len(page) != 1 || page[0].ID != oldest.ID {
		t.Errorf("ikinci sayfa en eski incelemeyi vermeli: %+v", page)
	}
}

// TestHideRealDB: gizlenen inceleme listede yok, sahibi gerekçesiyle görür, düzenleyince gizli kalır;
// gizleme ve geri açma audit'te, tekrarı ikinci audit satırı yazmaz (Doğrulama 15).
func TestHideRealDB(t *testing.T) {
	f := setup(t)
	r := f.mustSave(f.reader, "Gizlenecek", nil)
	f.mustSave(f.other, "Görünür", nil)

	hidden, err := f.s.Hide(f.ctx, f.admin, r.ID, hideReason)
	if err != nil || hidden.HiddenAt == nil || hidden.HiddenBy == nil || *hidden.HiddenBy != f.admin || hidden.Version != r.Version {
		t.Fatalf("Hide (gizleme sürümü artırmaz): %+v %v", hidden, err)
	}
	f.expectListed(1)
	if again, _ := f.s.Hide(f.ctx, f.admin, r.ID, "Başka gerekçe"); again.HiddenReason != hideReason {
		t.Errorf("zaten gizli incelemenin gerekçesi değişmemeli: %q", again.HiddenReason)
	}
	edited := f.mustSave(f.reader, "Düzelttim", ver(r.Version))
	if edited.HiddenAt == nil || edited.HiddenReason != hideReason {
		t.Errorf("sahibi düzenleyince gizli kalmalı: %+v", edited)
	}

	shown, err := f.s.Unhide(f.ctx, f.admin, r.ID)
	if err != nil || shown.HiddenAt != nil || shown.HiddenBy != nil || shown.HiddenReason != "" {
		t.Fatalf("Unhide: %+v %v", shown, err)
	}
	f.expectListed(2)
	if _, err := f.s.Unhide(f.ctx, f.admin, r.ID); err != nil {
		t.Errorf("görünür incelemeyi geri açmak zararsız olmalı: %v", err)
	}
	if h, u := f.auditCount(r.ID, ActionHide), f.auditCount(r.ID, ActionUnhide); h != 1 || u != 1 {
		t.Errorf("audit: gizleme %d, geri açma %d; ikisi de 1 olmalı", h, u)
	}
	f.expectAuditReason(r.ID, ActionUnhide, hideReason)
}

// TestModerateMissingRealDB: olmayan incelemeyi gizlemek ve geri açmak ErrNotFound.
func TestModerateMissingRealDB(t *testing.T) {
	f := setup(t)
	_, err := f.s.Hide(f.ctx, f.admin, -1, "Gerekçe")
	expectErr(t, "olmayan incelemeyi gizleme", err, ErrNotFound)
	_, err = f.s.Unhide(f.ctx, f.admin, -1)
	expectErr(t, "olmayan incelemeyi geri açma", err, ErrNotFound)
}

func (f fixture) expectListed(want int64) {
	f.t.Helper()
	if _, total, err := f.s.List(f.ctx, f.book, 20, 0); err != nil || total != want {
		f.t.Errorf("listede %d inceleme, beklenen %d (%v)", total, want, err)
	}
}

// expectAuditReason: geri açmada satırdan silinen gerekçenin tek izi audit'tir.
func (f fixture) expectAuditReason(reviewID int64, action, want string) {
	f.t.Helper()
	var got string
	err := f.pool.QueryRow(f.ctx, `SELECT details->>'reason' FROM audit_logs
		WHERE entity_type = 'review' AND entity_id = $1 AND action = $2`, reviewID, action).Scan(&got)
	if err != nil || got != want {
		f.t.Errorf("%s audit gerekçesi %q, beklenen %q (%v)", action, got, want, err)
	}
}

// TestConcurrentFirstWriteRealDB: iki sekme aynı anda ilk incelemeyi yazar. Birinci (holder) satırı
// eklemiş ama henüz commit etmemişken ikinci Save UNIQUE'te bekler; birinci commit edince ikincisi
// 409 alır, birincinin metni ezilmez.
func TestConcurrentFirstWriteRealDB(t *testing.T) {
	f := setup(t)
	ctx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
	defer cancel()

	conn, err := f.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	holder, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(context.Background()) }()
	if _, err := holder.Exec(ctx, `INSERT INTO reviews (user_id, book_id, body) VALUES ($1, $2, 'Birinci sekme')`, f.reader, f.book); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := f.s.Save(ctx, f.reader, f.book, Draft{Body: "İkinci sekme"})
		done <- err
	}()
	waitBlocked(ctx, t, f.pool, conn.Conn().PgConn().PID())
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	expectErr(t, "aynı anda ikinci ilk yazım", <-done, ErrVersionConflict)
	if mine, err := f.s.Mine(f.ctx, f.reader, f.book); err != nil || mine.Body != "Birinci sekme" {
		t.Errorf("birinci sekmenin metni kalmalı: %+v %v", mine, err)
	}
}

// waitBlocked, holder'ın kilidini bekleyen bir backend görünene kadar yoklar (pg_blocking_pids).
// Görmeden commit edilirse senaryo kurulmamış olur: Save satırı hazır bulup başka yoldan 409 alırdı.
func waitBlocked(ctx context.Context, t *testing.T, pool *pgxpool.Pool, holder uint32) {
	t.Helper()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		var pid int32
		err := pool.QueryRow(ctx, `SELECT pid FROM pg_stat_activity WHERE $1::integer = ANY(pg_blocking_pids(pid)) LIMIT 1`,
			int32(holder)).Scan(&pid)
		if err == nil {
			return
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("bekleyen işlem yoklanamadı: %v", err)
		}
		select {
		case <-ctx.Done():
			t.Fatal("ikinci yazım birincinin kilidini beklemedi")
		case <-tick.C:
		}
	}
}
