// Package reviews, kitap incelemelerini yönetir (Faz 17B; REV-01, REV-02): okurun kitap başına tek
// incelemesi (yaz, sürümlü düzenle, sil), herkese açık liste ve yöneticinin gerekçeli gizlemesi.
//
// Neden servis (düz CRUD değil, GO_CONVENTIONS §4): ilk yazım ile düzenleme tek uçta (PUT) birleşiyor
// ve sürüm kuralına bağlı; aynı anda iki ilk yazım yarışabiliyor; gizleme ve geri açma audit kaydıyla
// aynı transaction'da ve bir kez yazılmalı.
//
// İnceleme bir BELGEDİR (Faz 17C, internal/richtext): servis yalnız doğrulanmış richtext.Doc alır ve
// üç kolonu ondan yazar: body_doc (kanonik JSON), body (düz metin), has_spoiler (belgede spoiler
// işareti var mı). İstemcinin "spoiler var" demesine bakılmaz. Gerekçe uzunluğu handler'da doğrulanır.
// Veritabanındaki CHECK'ler son güvence: handler'ı atlayan bir yazım reddedilir.
package reviews

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mtgezgin/bookhub/backend/internal/audit"
	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/database"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/richtext"
)

// Sınırlar: 0018'deki CHECK'lerin aynısı (karakter, bayt değil). Metin sınırının sahibi richtext.
const (
	MaxBodyLen   = richtext.MaxTextLen
	MaxReasonLen = 500
)

// Audit eylemleri (audit_logs.action).
const (
	ActionHide   = "review.hide"
	ActionUnhide = "review.unhide"
)

var (
	// ErrNotFound: istenen inceleme yok. Kullanıcı uçlarında "benim incelemem yok", yönetici uçlarında
	// "bu numarada inceleme yok". Başkasının incelemesi de böyle görünür: uçlarda kullanıcı kimliği yok.
	ErrNotFound = errors.New("inceleme bulunamadı")
	// ErrVersionConflict: istemcinin bildiği hâl eskimiş; arada başka bir ekran yazmış ya da silmiş
	// (spec kavram 4). Handler 409 version_conflict der; istemci incelemesini yeniden okur.
	ErrVersionConflict = errors.New("inceleme başka bir yerde değişmiş")
)

// Draft, okurun gönderdiği inceleme.
type Draft struct {
	// Doc, doğrulanmış belge: richtext.Parse'tan geçmiş olmalı (handler'ın işi). Düz metin ve spoiler
	// bayrağı bundan türetilir.
	Doc richtext.Doc
	// Version, istemcinin okuduğu sürüm. nil = "bu kitapta incelemem yok" (ilk yazım). Sürüm her iki
	// yönde de karşılaştırılır: inceleme varken nil gelirse başka bir sekme az önce yazmıştır, yokken
	// sayı gelirse başka bir sekme silmiştir. İkisi de 409; hiçbir yazım haberi olmadan ezilmez.
	Version *int32
}

// Service, incelemelerin iş kurallarını işletir.
type Service struct {
	conn db.DBTX
}

// NewService, transaction açabilen bir bağlantıyla (havuz) servisi kurar.
func NewService(conn db.DBTX) *Service {
	return &Service{conn: conn}
}

// List, kitabın herkese açık incelemeleri (gizlenenler yok), en yeni önce, ve toplam sayısı. Kitap
// yayında değilse books.ErrNotFound.
func (s *Service) List(ctx context.Context, bookID int64, limit, offset int32) ([]db.ListBookReviewsRow, int64, error) {
	q := db.New(s.conn)
	if err := requireApproved(ctx, q, bookID); err != nil {
		return nil, 0, err
	}
	rows, err := q.ListBookReviews(ctx, db.ListBookReviewsParams{BookID: bookID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, fmt.Errorf("incelemeler okunamadı (kitap=%d): %w", bookID, err)
	}
	total, err := q.CountBookReviews(ctx, bookID)
	if err != nil {
		return nil, 0, fmt.Errorf("incelemeler sayılamadı (kitap=%d): %w", bookID, err)
	}
	return rows, total, nil
}

// Mine, okurun bu kitaptaki incelemesi; gizlenmişse gerekçesiyle (sahibi görür). Kitap yayında değilse
// books.ErrNotFound, inceleme yoksa ErrNotFound.
func (s *Service) Mine(ctx context.Context, userID, bookID int64) (db.Review, error) {
	q := db.New(s.conn)
	if err := requireApproved(ctx, q, bookID); err != nil {
		return db.Review{}, err
	}
	r, err := q.GetMyReview(ctx, db.GetMyReviewParams{UserID: userID, BookID: bookID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Review{}, ErrNotFound
	}
	if err != nil {
		return db.Review{}, fmt.Errorf("inceleme okunamadı (kitap=%d): %w", bookID, err)
	}
	return r, nil
}

// Save, incelemeyi yazar ya da düzenler (tek uç, PUT). Kitap yayında değilse books.ErrNotFound,
// sürüm uyuşmazsa ErrVersionConflict. Gizlenmiş inceleme düzenlenince gizli kalır (REV-01).
func (s *Service) Save(ctx context.Context, userID, bookID int64, d Draft) (db.Review, error) {
	var out db.Review
	err := database.WithTx(ctx, s.conn, func(tx pgx.Tx) error {
		q := db.New(tx)
		if err := requireApproved(ctx, q, bookID); err != nil {
			return err
		}
		cur, err := lockMine(ctx, q, userID, bookID)
		if err != nil {
			return err
		}
		if cur == nil {
			out, err = create(ctx, q, userID, bookID, d)
		} else {
			out, err = edit(ctx, q, *cur, d)
		}
		return err
	})
	return out, err
}

// Delete, okurun incelemesini siler; puanı silmez (ratings ayrı tablo, Doğrulama 14). Kitabın yayında
// olması aranmaz: okur kendi verisini her zaman silebilir (kitaplıktan çıkarmak gibi, Faz 15).
func (s *Service) Delete(ctx context.Context, userID, bookID int64) error {
	n, err := db.New(s.conn).DeleteMyReview(ctx, db.DeleteMyReviewParams{UserID: userID, BookID: bookID})
	if err != nil {
		return fmt.Errorf("inceleme silinemedi (kitap=%d): %w", bookID, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Hide, incelemeyi yönetici adına gerekçesiyle gizler; audit aynı transaction'da. Zaten gizliyse
// hiçbir şey yazmaz (gerekçe de değişmez) ve güncel hâli döner.
func (s *Service) Hide(ctx context.Context, adminID, reviewID int64, reason string) (db.Review, error) {
	return s.moderate(ctx, adminID, reviewID, true, func(q *db.Queries) (db.Review, error) {
		return q.HideReview(ctx, db.HideReviewParams{ID: reviewID, HiddenBy: &adminID, Reason: reason})
	})
}

// Unhide, gizlemeyi kaldırır; gerekçe de silinir, audit'te kalır. Zaten görünürse hiçbir şey yazmaz.
func (s *Service) Unhide(ctx context.Context, adminID, reviewID int64) (db.Review, error) {
	return s.moderate(ctx, adminID, reviewID, false, func(q *db.Queries) (db.Review, error) {
		return q.UnhideReview(ctx, reviewID)
	})
}

// moderate, gizleme ve geri açmanın ortak yolu. Satır kilitlenir, önceki durum kilitli satırdan okunur:
// iki yönetici aynı anda gizlerse ikincisi zaten gizli satırı görür ve audit'e ikinci satır yazılmaz.
// Audit'teki gerekçe: gizlemede verilen, geri açmada kaldırılan (satırdan silindiği için tek iz orası).
func (s *Service) moderate(ctx context.Context, adminID, reviewID int64, hide bool, write func(*db.Queries) (db.Review, error)) (db.Review, error) {
	var out db.Review
	err := database.WithTx(ctx, s.conn, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := q.LockReview(ctx, reviewID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("inceleme okunamadı (id=%d): %w", reviewID, err)
		}
		if (cur.HiddenAt != nil) == hide {
			out = cur
			return nil
		}
		if out, err = write(q); err != nil {
			return fmt.Errorf("inceleme gizleme durumu yazılamadı (id=%d): %w", reviewID, err)
		}
		action, reason := ActionUnhide, cur.HiddenReason
		if hide {
			action, reason = ActionHide, out.HiddenReason
		}
		return audit.Write(ctx, q, adminID, action, audit.EntityReview, out.ID,
			map[string]any{"book_id": out.BookID, "reason": reason})
	})
	return out, err
}

// create, ilk yazım. İstemci sürüm gönderdiyse incelemesi olduğunu sanıyordu: arada silinmiş → 409.
func create(ctx context.Context, q *db.Queries, userID, bookID int64, d Draft) (db.Review, error) {
	if d.Version != nil {
		return db.Review{}, ErrVersionConflict
	}
	r, err := q.InsertReview(ctx, db.InsertReviewParams{
		UserID: userID, BookID: bookID, Body: d.Doc.Text(), HasSpoiler: d.Doc.HasSpoiler(), BodyDoc: d.Doc.JSON(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Review{}, lostInsert(ctx, q, userID, bookID)
	}
	if err != nil {
		return db.Review{}, fmt.Errorf("inceleme yazılamadı (kitap=%d): %w", bookID, err)
	}
	return r, nil
}

// lostInsert, InsertReview satır döndürmediğinde nedenini ayırır. Ya aynı anda başka bir istek ilk
// incelemeyi yazdı (UNIQUE → DO NOTHING; o istek bitene kadar beklendi, satır artık görünür) ya da kitap
// tam bu arada yayından kalktı. İlkinde bu istek sürüm göndermemişti: o yazımı ezmemek için 409.
func lostInsert(ctx context.Context, q *db.Queries, userID, bookID int64) error {
	cur, err := lockMine(ctx, q, userID, bookID)
	switch {
	case err != nil:
		return err
	case cur != nil:
		return ErrVersionConflict
	default:
		return books.ErrNotFound
	}
}

// edit, düzenleme. Sürüm uyuşmalı; belge aynıysa hiçbir şey yazılmaz (sürüm artmaz, "düzenlendi"
// görünmez: kaydet'e iki kez basmak düzenleme değildir).
func edit(ctx context.Context, q *db.Queries, cur db.Review, d Draft) (db.Review, error) {
	if d.Version == nil || *d.Version != cur.Version {
		return db.Review{}, ErrVersionConflict
	}
	if unchanged(cur, d.Doc) {
		return cur, nil
	}
	r, err := q.UpdateReview(ctx, db.UpdateReviewParams{
		ID: cur.ID, Body: d.Doc.Text(), HasSpoiler: d.Doc.HasSpoiler(), BodyDoc: d.Doc.JSON(),
	})
	if err != nil {
		return db.Review{}, fmt.Errorf("inceleme düzenlenemedi (id=%d): %w", cur.ID, err)
	}
	return r, nil
}

// unchanged: yeni belge saklananla aynı mı. Belgesi olmayan eski satır (0019'dan önce ya da eski kodla
// yazılmış) düz metninden çevrilerek karşılaştırılır; aynıysa yazılmaz ve satır belgesiz kalır (kalıcı
// çeviri daraltma PR'ının işi). Saklanan belge okunamıyorsa "değişti" sayılır: yeni yazım onu onarır.
func unchanged(cur db.Review, doc richtext.Doc) bool {
	if cur.BodyDoc == nil {
		return richtext.Equal(richtext.FromPlain(cur.Body, cur.HasSpoiler), doc)
	}
	stored, err := richtext.Parse(cur.BodyDoc)
	return err == nil && richtext.Equal(stored, doc)
}

// DocJSON, bir incelemenin cevaba yazılacak belgesi: saklanan belge varsa o, yoksa düz metinden
// çevrilmişi (spec karar 6). Saklanan belge yalnız Parse'tan geçmiş yazımlardan gelir; okurken yeniden
// doğrulanmaz.
func DocJSON(body string, hasSpoiler bool, stored json.RawMessage) json.RawMessage {
	if stored != nil {
		return stored
	}
	return richtext.FromPlain(body, hasSpoiler).JSON()
}

// lockMine, okurun bu kitaptaki incelemesini transaction sonuna kadar kilitler. nil = inceleme yok.
func lockMine(ctx context.Context, q *db.Queries, userID, bookID int64) (*db.Review, error) {
	r, err := q.LockMyReview(ctx, db.LockMyReviewParams{UserID: userID, BookID: bookID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inceleme kilitlenemedi (kitap=%d): %w", bookID, err)
	}
	return &r, nil
}

// requireApproved: kitap yayında değilse books.ErrNotFound (Faz 15: yayında olmayan kitap ile olmayan
// kitap ayırt edilmez).
func requireApproved(ctx context.Context, q *db.Queries, bookID int64) error {
	ok, err := q.BookIsApproved(ctx, bookID)
	if err != nil {
		return fmt.Errorf("kitap durumu okunamadı (kitap=%d): %w", bookID, err)
	}
	if !ok {
		return books.ErrNotFound
	}
	return nil
}
