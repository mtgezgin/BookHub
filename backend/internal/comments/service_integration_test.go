package comments

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

func createUser(t *testing.T, pool *pgxpool.Pool, name string) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (email, display_name, password_hash) VALUES ($1, $2, 'x') RETURNING id`,
		testdb.Unique("commenter")+"@example.com", name).Scan(&id)
	if err != nil {
		t.Fatalf("kullanıcı açılamadı: %v", err)
	}
	t.Cleanup(func() { testdb.Exec(t, pool, "DELETE FROM users WHERE id = $1", id) })
	return id
}

func createBook(t *testing.T, pool *pgxpool.Pool, status string) int64 {
	t.Helper()
	slug := testdb.Unique("commented")
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO books (title, slug, author, status) VALUES ($1, $1, 'Yazar', $2) RETURNING id`, slug, status).Scan(&id)
	if err != nil {
		t.Fatalf("kitap açılamadı: %v", err)
	}
	t.Cleanup(func() { testdb.Exec(t, pool, "DELETE FROM books WHERE id = $1", id) })
	return id
}

func TestCommentsRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	s := NewService(pool)
	ctx := context.Background()
	book := createBook(t, pool, books.StatusApproved)
	user, admin := createUser(t, pool, "Ayşe"), createUser(t, pool, "Admin")

	first, err := s.Create(ctx, user, book, "Harika bir kitap")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := s.Create(ctx, user, book, "<script>alert(1)</script>"); err != nil {
		t.Fatalf("Create (ham içerik): %v", err)
	}

	list, total, err := s.List(ctx, book, 20, 0)
	if err != nil || total != 2 || len(list) != 2 {
		t.Fatalf("List: %d/%d %v", len(list), total, err)
	}
	// En yeni önce; içerik HAM saklanır (kaçışlama gösterimde, React'te).
	if list[0].Content != "<script>alert(1)</script>" || list[1].AuthorName == nil || *list[1].AuthorName != "Ayşe" {
		t.Errorf("sıra/yazar/içerik: %+v", list)
	}

	if err := s.Delete(ctx, admin, first.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, total, err = s.List(ctx, book, 20, 0)
	if err != nil || total != 1 {
		t.Errorf("silinen yorum görünmemeli: total=%d %v", total, err)
	}
	// Satır DURUYOR (yumuşak silme) ve kimin sildiği kayıtlı.
	var deletedBy *int64
	if err := pool.QueryRow(ctx, `SELECT deleted_by FROM comments WHERE id = $1`, first.ID).Scan(&deletedBy); err != nil || deletedBy == nil || *deletedBy != admin {
		t.Errorf("yumuşak silme deleted_by yazmalı: %v %v", deletedBy, err)
	}
	if err := s.Delete(ctx, admin, first.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("ikinci silme ErrNotFound vermeli: %v", err)
	}

	var auditRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE entity_type = 'comment' AND entity_id = $1 AND action = 'comment.delete'`, first.ID).Scan(&auditRows); err != nil || auditRows != 1 {
		t.Errorf("silme denetim kaydı bekleniyordu: %d %v", auditRows, err)
	}
}

func TestCommentOnlyOnApprovedRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	s := NewService(pool)
	user := createUser(t, pool, "Ali")
	for _, st := range []string{books.StatusPending, books.StatusRejected, books.StatusArchived} {
		if _, err := s.Create(context.Background(), user, createBook(t, pool, st), "merhaba"); !errors.Is(err, books.ErrNotFound) {
			t.Errorf("%s kitaba yorum ErrNotFound vermeli: %v", st, err)
		}
	}
}
