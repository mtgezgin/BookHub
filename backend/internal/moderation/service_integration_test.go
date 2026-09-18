package moderation

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

// createUser, test için bir kullanıcı açar (kitapların submitted_by/moderated_by FK'leri).
func createUser(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	email := testdb.Unique("mod") + "@example.com"
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (email, display_name, password_hash) VALUES ($1, 'Test', 'x') RETURNING id`, email).Scan(&id)
	if err != nil {
		t.Fatalf("kullanıcı açılamadı: %v", err)
	}
	t.Cleanup(func() { testdb.Exec(t, pool, "DELETE FROM users WHERE id = $1", id) })
	return id
}

func params(t *testing.T) db.CreateBookParams {
	t.Helper()
	slug := testdb.Unique("mod")
	return db.CreateBookParams{Title: "Moderasyon " + slug, Slug: slug, Author: "Yazar", Language: "tr"}
}

// cleanupBook: kitap silinir; audit satırları kalır (append-only — silinemez, bilerek).
func cleanupBook(t *testing.T, pool *pgxpool.Pool, id int64) {
	t.Cleanup(func() { testdb.Exec(t, pool, "DELETE FROM books WHERE id = $1", id) })
}

func actions(t *testing.T, s *Service, id int64) []string {
	t.Helper()
	logs, err := s.History(context.Background(), id)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	var out []string
	for i := len(logs) - 1; i >= 0; i-- { // eskiden yeniye
		out = append(out, logs[i].Action)
	}
	return out
}

func TestSuggestApproveFlowRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	s := NewService(pool)
	ctx := context.Background()
	user, admin := createUser(t, pool), createUser(t, pool)

	b, err := s.Suggest(ctx, user, params(t))
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	cleanupBook(t, pool, b.ID)
	if b.Status != books.StatusPending || b.SubmittedBy == nil || *b.SubmittedBy != user {
		t.Fatalf("öneri PENDING ve önerene bağlı olmalı: %+v", b)
	}

	// Public okuma onay öncesi kitabı GÖRMEMELİ.
	if _, err := books.NewRepository(pool).Get(ctx, b.ID); !errors.Is(err, books.ErrNotFound) {
		t.Errorf("onaylanmamış öneri public görünmemeli, gelen %v", err)
	}

	approved, err := s.Approve(ctx, admin, b.ID)
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if approved.Status != books.StatusApproved || approved.ModeratedBy == nil || *approved.ModeratedBy != admin || approved.ModeratedAt == nil {
		t.Errorf("onay moderatörü ve zamanı yazmalı: %+v", approved)
	}
	if _, err := books.NewRepository(pool).Get(ctx, b.ID); err != nil {
		t.Errorf("onaylı kitap public görünmeli: %v", err)
	}

	// Onaylıyı tekrar onaylamak / reddetmek yasak.
	if _, err := s.Reject(ctx, admin, b.ID, "geç kaldın"); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("APPROVED → reject yasak olmalı, gelen %v", err)
	}

	if _, err := s.Archive(ctx, admin, b.ID); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if _, err := s.Restore(ctx, admin, b.ID); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	want := []string{"book.suggest", "book.approve", "book.archive", "book.restore"}
	if got := actions(t, s, b.ID); len(got) != len(want) || got[0] != want[0] || got[3] != want[3] {
		t.Errorf("denetim kaydı: istenen %v, gelen %v (başarısız reject kayıt BIRAKMAMALI)", want, got)
	}
}

func TestRejectStoresReasonRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	s := NewService(pool)
	ctx := context.Background()
	user, admin := createUser(t, pool), createUser(t, pool)

	b, err := s.Suggest(ctx, user, params(t))
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	cleanupBook(t, pool, b.ID)

	rejected, err := s.Reject(ctx, admin, b.ID, "kaynak eksik")
	if err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if rejected.Status != books.StatusRejected || rejected.RejectionReason == nil || *rejected.RejectionReason != "kaynak eksik" {
		t.Errorf("red gerekçesi saklanmalı: %+v", rejected)
	}

	logs, err := s.History(ctx, b.ID)
	if err != nil || len(logs) == 0 {
		t.Fatalf("History: %v (%d kayıt)", err, len(logs))
	}
	var d map[string]string
	if err := json.Unmarshal(logs[0].Details, &d); err != nil {
		t.Fatalf("details JSON değil: %v", err)
	}
	if d["from"] != books.StatusPending || d["to"] != books.StatusRejected || d["reason"] != "kaynak eksik" {
		t.Errorf("denetim detayı: %v", d)
	}
	if logs[0].ActorID == nil || *logs[0].ActorID != admin {
		t.Errorf("aktör admin olmalı: %v", logs[0].ActorID)
	}
}

// TestConcurrentModerationRealDB: iki admin AYNI PENDING kitaba aynı anda biri onay biri
// red gönderir. FOR UPDATE sayesinde tam olarak BİRİ kazanmalı, diğeri geçersiz geçiş
// almalı; denetim kaydında tek bir moderasyon kararı olmalı.
func TestConcurrentModerationRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	s := NewService(pool)
	ctx := context.Background()
	user, a1, a2 := createUser(t, pool), createUser(t, pool), createUser(t, pool)

	for round := range 5 { // yarış olasılığını artırmak için birkaç tur
		b, err := s.Suggest(ctx, user, params(t))
		if err != nil {
			t.Fatalf("Suggest: %v", err)
		}
		cleanupBook(t, pool, b.ID)

		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make([]error, 2)
		wg.Add(2)
		go func() { defer wg.Done(); <-start; _, errs[0] = s.Approve(ctx, a1, b.ID) }()
		go func() { defer wg.Done(); <-start; _, errs[1] = s.Reject(ctx, a2, b.ID, "yarış") }()
		close(start)
		wg.Wait()

		wins, losses := 0, 0
		for _, err := range errs {
			switch {
			case err == nil:
				wins++
			case errors.Is(err, ErrInvalidTransition):
				losses++
			default:
				t.Fatalf("tur %d: beklenmeyen hata %v", round, err)
			}
		}
		if wins != 1 || losses != 1 {
			t.Errorf("tur %d: tek kazanan bekleniyordu (kazanan=%d, kaybeden=%d)", round, wins, losses)
		}
		if got := actions(t, s, b.ID); len(got) != 2 {
			t.Errorf("tur %d: öneri + TEK karar bekleniyordu, gelen %v", round, got)
		}
	}
}

func TestCreateApprovedAndUpdateRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	s := NewService(pool)
	ctx := context.Background()
	admin := createUser(t, pool)

	b, err := s.CreateApproved(ctx, admin, params(t))
	if err != nil {
		t.Fatalf("CreateApproved: %v", err)
	}
	cleanupBook(t, pool, b.ID)
	if b.Status != books.StatusApproved {
		t.Errorf("admin eklemesi APPROVED başlamalı: %s", b.Status)
	}

	newTitle := "Yeni Başlık"
	updated, err := s.Update(ctx, admin, b.ID, db.UpdateBookDetailsParams{Title: &newTitle})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != newTitle || updated.Author != b.Author || updated.Slug != b.Slug {
		t.Errorf("yalnızca başlık değişmeli: %+v", updated)
	}

	other, err := s.CreateApproved(ctx, admin, params(t))
	if err != nil {
		t.Fatalf("CreateApproved: %v", err)
	}
	cleanupBook(t, pool, other.ID)
	if _, err := s.Update(ctx, admin, other.ID, db.UpdateBookDetailsParams{Slug: &b.Slug}); !errors.Is(err, books.ErrDuplicateSlug) {
		t.Errorf("çakışan slug ErrDuplicateSlug vermeli, gelen %v", err)
	}
	if _, err := s.Update(ctx, admin, 0, db.UpdateBookDetailsParams{Title: &newTitle}); !errors.Is(err, books.ErrNotFound) {
		t.Errorf("olmayan kitap ErrNotFound vermeli, gelen %v", err)
	}

	logs, err := s.History(ctx, b.ID)
	if err != nil || len(logs) != 2 || logs[0].Action != "book.update" {
		t.Fatalf("create + update kaydı bekleniyordu: %v %v", logs, err)
	}
	var d map[string][]string
	if err := json.Unmarshal(logs[0].Details, &d); err != nil || len(d["fields"]) != 1 || d["fields"][0] != "title" {
		t.Errorf("güncellenen alanlar kaydedilmeli: %s (%v)", logs[0].Details, err)
	}
}

func TestNotFoundRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	s := NewService(pool)
	ctx := context.Background()
	admin := createUser(t, pool)

	if _, err := s.Approve(ctx, admin, 0); !errors.Is(err, books.ErrNotFound) {
		t.Errorf("Approve: ErrNotFound bekleniyordu, gelen %v", err)
	}
	if _, err := s.History(ctx, 0); !errors.Is(err, books.ErrNotFound) {
		t.Errorf("History: ErrNotFound bekleniyordu, gelen %v", err)
	}
}

// TestPublicChangeHookRealDB: kanca yalnız BAŞARILI geçiş/düzenlemeden sonra çağrılır.
func TestPublicChangeHookRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	calls := 0
	s := NewService(pool, WithPublicChangeHook(func(context.Context) { calls++ }))
	ctx := context.Background()
	user, admin := createUser(t, pool), createUser(t, pool)

	b, err := s.Suggest(ctx, user, params(t))
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	cleanupBook(t, pool, b.ID)
	if calls != 0 {
		t.Errorf("öneri public görünümü değiştirmez, kanca çağrılmamalı: %d", calls)
	}
	if _, err := s.Approve(ctx, admin, b.ID); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if _, err := s.Approve(ctx, admin, b.ID); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("ikinci onay reddedilmeli: %v", err)
	}
	title := "Düzeltilmiş"
	if _, err := s.Update(ctx, admin, b.ID, db.UpdateBookDetailsParams{Title: &title}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if calls != 2 {
		t.Errorf("onay + düzenleme = 2 çağrı bekleniyordu (başarısız onay sayılmaz), gelen %d", calls)
	}
}

// TestListByStatusRealDB: List yalnız istenen statüdeki kitapları döner, limit'e uyar ve
// CountBooksByStatus toplamını verir. Tek bir REPEATABLE READ işlemi içinde koşar: paralel
// koşan diğer paketlerin yazdığı kitaplar liste ile sayım arasındaki anlık görüntüyü bozmasın;
// sonda geri alınır (temizlik gerekmez).
func TestListByStatusRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })

	insert := func(status string) int64 {
		slug := testdb.Unique("modlist")
		var id int64
		if err := tx.QueryRow(ctx, `INSERT INTO books (title, slug, author, status) VALUES ($1, $1, 'Yazar', $2) RETURNING id`, slug, status).Scan(&id); err != nil {
			t.Fatalf("kitap açılamadı: %v", err)
		}
		return id
	}
	for range 3 {
		insert(books.StatusPending)
	}
	approvedID := insert(books.StatusApproved)

	s := NewService(tx)
	list, total, err := s.List(ctx, books.StatusPending, 2, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("limit 2'ye uyulmalı, gelen %d", len(list))
	}
	want, err := db.New(tx).CountBooksByStatus(ctx, books.StatusPending)
	if err != nil {
		t.Fatalf("CountBooksByStatus: %v", err)
	}
	if total != want || total < 3 {
		t.Errorf("toplam CountBooksByStatus ile aynı olmalı: istenen %d, gelen %d", want, total)
	}

	all, _, err := s.List(ctx, books.StatusPending, int32(total), 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, b := range all {
		if b.Status != books.StatusPending || b.ID == approvedID {
			t.Errorf("PENDING listesinde başka statü: id=%d statü=%s", b.ID, b.Status)
		}
	}
}

// TestCreateDuplicateSlugRollsBackRealDB: create içindeki books.Repository.Create hatası
// (burada slug çakışması) ErrDuplicateSlug olarak dönmeli ve transaction geri alınmalı —
// ikinci bir kitap satırı da, başarısız denemeye ait bir denetim kaydı da kalmamalı.
func TestCreateDuplicateSlugRollsBackRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	s := NewService(pool)
	ctx := context.Background()
	admin := createUser(t, pool)

	p := params(t)
	first, err := s.CreateApproved(ctx, admin, p)
	if err != nil {
		t.Fatalf("CreateApproved: %v", err)
	}
	cleanupBook(t, pool, first.ID)

	dup := p
	dup.Title = "Başka Başlık " + p.Slug
	got, err := s.CreateApproved(ctx, admin, dup)
	if !errors.Is(err, books.ErrDuplicateSlug) {
		t.Fatalf("aynı slug: istenen ErrDuplicateSlug, gelen %v", err)
	}
	if got.ID != 0 {
		t.Errorf("hata durumunda boş kitap dönmeli: %+v", got)
	}

	var bookRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM books WHERE slug = $1`, p.Slug).Scan(&bookRows); err != nil {
		t.Fatalf("kitap sayımı: %v", err)
	}
	if bookRows != 1 {
		t.Errorf("ikinci kitap satırı kalmamalı: %d satır", bookRows)
	}

	var auditRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE actor_id = $1`, admin).Scan(&auditRows); err != nil {
		t.Fatalf("denetim sayımı: %v", err)
	}
	if auditRows != 1 {
		t.Errorf("yalnız ilk eklemenin denetim kaydı olmalı: %d kayıt", auditRows)
	}
}
