package library

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/database"
	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// Rafın iş kuralı hataları. "Bulunamadı" için books.ErrNotFound kullanılır (Faz 15 kuralı: yayında
// olmayan kitap ile olmayan kitap ayırt edilmez).
var (
	// ErrVersionConflict: ilerleme düzeltmesi eski bir sürüme dayanıyor; arada başka bir ekran yazmış
	// (spec kavram 4). Handler 409 version_conflict der; istemci rafı yeniden okur.
	ErrVersionConflict = errors.New("okuma başka bir yerde güncellenmiş")
	// ErrInvalidProgress: negatif sayfa, bilinen toplamı aşan sayfa, 0–100 dışı yüzde ya da bilinmeyen birim.
	ErrInvalidProgress = errors.New("ilerleme geçersiz")
	// ErrInvalidDates: bitiş başlangıçtan önce.
	ErrInvalidDates = errors.New("bitiş günü başlangıçtan önce olamaz")
	// ErrInvalidEdition: seçilen baskı bu kitabın değil (ya da yok).
	ErrInvalidEdition = errors.New("baskı bu kitaba ait değil")
)

// Shelf, bir kitabın kullanıcının rafındaki hâli: güncel kayıt ve kapanmış okumaları.
type Shelf struct {
	// BookID, rafın kitabı. Günlük uçları kitabı adresten değil kayıttan bulur; kayıt da kitaplık satırı
	// da kalmamışsa (son geçmiş satırı silindi) cevap yine hangi kitabın rafı olduğunu söyler.
	BookID int64
	// Entry nil = kitaplıkta değil. Okuma alanları yalnız Status READING iken dolu döner (0017'ye kadar
	// başka durumda bayat kalabilirler; spec 17A § 1).
	Entry *db.LibraryEntry
	// Log, kapanmış okumalar, en yeni önce. Kitap kitaplıktan çıkarılsa da kalır.
	Log []db.ReadingLog
}

// CommandInput, bir komutun isteğe bağlı alanları. Yalnız ilgili komut okur: başla baskı, birim ve
// başlangıç gününü; bitir ve yarım bırak bitiş gününü. "Bugün"ü istemci söyler (kavram 6).
type CommandInput struct {
	Command   Command
	Mode      StartMode
	EditionID *int64
	Unit      *string
	StartedOn *time.Time
	EndedOn   *time.Time
}

// ReadingPatch, okurken alan düzeltmesi. nil = değişmez. Version zorunlu: okunan sürüm (kavram 4).
// Birim ya da baskı değişirse ilerleme sıfırlanır (dönüşüm yok; spec § Kararlar).
type ReadingPatch struct {
	Version   int32
	Progress  *int32
	Unit      *string
	EditionID *int64
	StartedOn *time.Time
}

// Service, rafın durum makinesini işletir (Faz 17A-1).
//
// Neden servis (Faz 15'te yalnız repository vardı): artık her yazımın kuralları var — komut durumdan
// etkiye çevrilir (resolve), kitaplık satırı ve günlük AYNI transaction'da yazılır, eşzamanlı komutlar
// sıraya girer, sayfa toplamı baskıdan kopyalanır, ilerleme sürümle korunur. Bunlar ne HTTP'nin ne tek
// bir SQL sorgusunun işi (GO_CONVENTIONS §4).
type Service struct {
	conn db.DBTX
}

// NewService, transaction açabilen bir bağlantıyla (havuz) servisi kurar.
func NewService(conn db.DBTX) *Service {
	return &Service{conn: conn}
}

// Get, kitap sayfasının rafı. Kitap yayında değilse books.ErrNotFound.
func (s *Service) Get(ctx context.Context, userID, bookID int64) (Shelf, error) {
	q := db.New(s.conn)
	if err := requireApproved(ctx, q, bookID); err != nil {
		return Shelf{}, err
	}
	return load(ctx, q, userID, bookID)
}

// Do, bir komutu tek transaction'da uygular ve güncel rafı döner (istemci ayrıca okumaz).
//
// Sıra her komutta aynı: (1) kitap yayında mı ("çıkar" hariç), (2) (kullanıcı, kitap) kilidi,
// (3) satır FOR UPDATE, (4) resolve, (5) yazım, (6) güncel raf. Hata herhangi bir adımda olursa hiçbir
// şey yazılmamış olur (database.WithTx).
func (s *Service) Do(ctx context.Context, userID, bookID int64, in CommandInput) (Shelf, error) {
	if in.Command == CmdMarkRead {
		in.EndedOn = nil // "Zaten okudum": bilinmeyen tarih uydurulmaz (READ-02)
	}
	var out Shelf
	err := database.WithTx(ctx, s.conn, func(tx pgx.Tx) error {
		q := db.New(tx)
		if in.Command != CmdRemove { // çıkarmak her zaman çalışır (Faz 15)
			if err := requireApproved(ctx, q, bookID); err != nil {
				return err
			}
		}
		cur, err := lockShelf(ctx, q, userID, bookID)
		if err != nil {
			return err
		}
		current := StatusNone
		if cur != nil {
			current = cur.Status
		}
		st, err := resolve(current, in.Command, in.Mode)
		if err != nil {
			return err
		}
		w := writer{q: q, userID: userID, bookID: bookID}
		if err := w.apply(ctx, st, cur, in); err != nil {
			return err
		}
		out, err = load(ctx, q, userID, bookID)
		return err
	})
	return out, err
}

// UpdateReading, okurken ilerleme, birim, baskı ya da başlangıç gününü düzeltir. Kitap okunmuyorsa
// books.ErrNotFound (404), sürüm eskiyse ErrVersionConflict (409). İkisini ayırmak için satır
// kilitlenip okunur; tek bir "UPDATE … WHERE version = $n" 0 satır döndüğünde hangisi olduğunu
// söyleyemezdi (spec § Önemli noktalar).
func (s *Service) UpdateReading(ctx context.Context, userID, bookID int64, p ReadingPatch) (Shelf, error) {
	var out Shelf
	err := database.WithTx(ctx, s.conn, func(tx pgx.Tx) error {
		q := db.New(tx)
		if err := requireApproved(ctx, q, bookID); err != nil {
			return err
		}
		cur, err := lockShelf(ctx, q, userID, bookID)
		if err != nil {
			return err
		}
		if cur == nil || cur.Status != StatusReading {
			return books.ErrNotFound
		}
		if cur.Version != p.Version {
			return ErrVersionConflict
		}
		r, err := applyPatch(ctx, q, bookID, readingOf(cur), p)
		if err != nil {
			return err
		}
		if err := validProgress(r); err != nil {
			return err
		}
		w := writer{q: q, userID: userID, bookID: bookID}
		if err := w.save(ctx, StatusReading, r); err != nil {
			return err
		}
		out, err = load(ctx, q, userID, bookID)
		return err
	})
	return out, err
}

// applyPatch, yamayı süren okumaya uygular. Baskı ya da birim değişirse yeni bir anlık görüntü alınır
// (sayfa toplamı o baskıdan, ilerleme sıfırdan); ardından yamadaki ilerleme ve başlangıç günü yazılır.
func applyPatch(ctx context.Context, q *db.Queries, bookID int64, r reading, p ReadingPatch) (reading, error) {
	editionChange := p.EditionID != nil && (r.EditionID == nil || *p.EditionID != *r.EditionID)
	unitChange := p.Unit != nil && (r.Unit == nil || *p.Unit != *r.Unit)
	// Faz 15'ten kalan okuma birimsizdir ("İlerlemeni gir"): ilk ilerlemeyle baskı ve birim seçilir.
	firstEntry := r.Unit == nil && p.Progress != nil
	if editionChange || unitChange || firstEntry {
		in := CommandInput{EditionID: r.EditionID, Unit: r.Unit, StartedOn: r.StartedOn}
		if p.EditionID != nil {
			in.EditionID = p.EditionID
		}
		if p.Unit != nil {
			in.Unit = p.Unit
		}
		var err error
		if r, err = snapshot(ctx, q, bookID, in); err != nil {
			return reading{}, err
		}
	}
	if p.Progress != nil {
		r.Progress = p.Progress
	}
	if p.StartedOn != nil {
		r.StartedOn = p.StartedOn
	}
	return r, nil
}

// UpdateLogDates, kapanmış bir okumanın başlangıç ya da bitiş gününü düzeltir (nil = değişmez).
// Başkasının kaydı books.ErrNotFound. Kitap yayında olmasa da çalışır: kullanıcının kendi geçmişi.
func (s *Service) UpdateLogDates(ctx context.Context, userID, logID int64, startedOn, endedOn *time.Time) (Shelf, error) {
	return s.withLog(ctx, userID, logID, func(q *db.Queries, l db.ReadingLog) error {
		if startedOn != nil {
			l.StartedOn = startedOn
		}
		if endedOn != nil {
			l.EndedOn = endedOn
		}
		if err := validDates(l.StartedOn, l.EndedOn); err != nil {
			return err
		}
		_, err := q.UpdateReadingLogDates(ctx, db.UpdateReadingLogDatesParams{
			ID: l.ID, UserID: userID, StartedOn: l.StartedOn, EndedOn: l.EndedOn,
		})
		if err != nil {
			return fmt.Errorf("günlük tarihleri yazılamadı: %w", err)
		}
		return nil
	})
}

// DeleteLog, kapanmış bir okumayı geçmişten siler. Başkasının kaydı books.ErrNotFound.
func (s *Service) DeleteLog(ctx context.Context, userID, logID int64) (Shelf, error) {
	return s.withLog(ctx, userID, logID, func(q *db.Queries, l db.ReadingLog) error {
		if _, err := q.DeleteReadingLog(ctx, db.DeleteReadingLogParams{ID: l.ID, UserID: userID}); err != nil {
			return fmt.Errorf("günlük satırı silinemedi: %w", err)
		}
		return nil
	})
}

// withLog, günlük işlemlerinin ortak iskeleti. Kilit sırası komutlarınkiyle aynı (raf → günlük): önce
// kaydın kitabı kilitsiz okunur, sonra o kitabın raf kilidi alınır, en son satır kilitlenir. Ters sıra
// "Devam et" (raf kilidi elinde, günlük satırını bekler) ile çakışıp kilitlenebilirdi (deadlock).
func (s *Service) withLog(ctx context.Context, userID, logID int64, fn func(*db.Queries, db.ReadingLog) error) (Shelf, error) {
	var out Shelf
	err := database.WithTx(ctx, s.conn, func(tx pgx.Tx) error {
		q := db.New(tx)
		l, err := q.GetReadingLog(ctx, db.GetReadingLogParams{ID: logID, UserID: userID})
		if err != nil {
			return notFound(err, "günlük kaydı okunamadı")
		}
		if err := q.LockShelfKey(ctx, db.LockShelfKeyParams{UserID: userID, BookID: l.BookID}); err != nil {
			return fmt.Errorf("raf kilidi alınamadı: %w", err)
		}
		// Kilidi beklerken kayıt silinmiş olabilir ("Devam et" onu geri açtıysa).
		if l, err = q.LockReadingLog(ctx, db.LockReadingLogParams{ID: logID, UserID: userID}); err != nil {
			return notFound(err, "günlük kaydı kilitlenemedi")
		}
		if err := fn(q, l); err != nil {
			return err
		}
		out, err = load(ctx, q, userID, l.BookID)
		return err
	})
	return out, err
}

// reading, bir okumanın alanları (kitaplık satırında ya da günlükte aynı).
type reading struct {
	EditionID  *int64
	Unit       *string
	Progress   *int32
	TotalPages *int32
	StartedOn  *time.Time
}

func readingOf(e *db.LibraryEntry) reading {
	if e == nil || e.Status != StatusReading {
		return reading{}
	}
	return reading{EditionID: e.EditionID, Unit: e.Unit, Progress: e.Progress, TotalPages: e.TotalPages, StartedOn: e.StartedOn}
}

// hasProgress: ilerleme > 0 mı. Ertelerken ve çıkarırken yalnız o zaman günlüğe "yarım bırakıldı"
// yazılır: yanlışlıkla başlatılan okuma geçmişi kirletmez (spec § Kararlar).
func (r reading) hasProgress() bool { return r.Progress != nil && *r.Progress > 0 }

// writer, bir transaction içindeki yazımlar.
type writer struct {
	q              *db.Queries
	userID, bookID int64
}

func (w writer) apply(ctx context.Context, st step, cur *db.LibraryEntry, in CommandInput) error {
	open := readingOf(cur)
	switch st.act {
	case actNone:
		return nil
	case actSetStatus:
		return w.save(ctx, st.status, reading{})
	case actStart:
		return w.start(ctx, in)
	case actResume:
		return w.resume(ctx, in)
	case actClose:
		if err := validDates(open.StartedOn, in.EndedOn); err != nil {
			return err
		}
		if err := w.save(ctx, st.status, reading{}); err != nil {
			return err
		}
		outcome := OutcomeFinished
		if st.status == StatusAbandoned {
			outcome = OutcomeAbandoned
		}
		return w.log(ctx, outcome, open, in.EndedOn)
	case actPostpone:
		if err := w.save(ctx, StatusWantToRead, reading{}); err != nil {
			return err
		}
		if open.hasProgress() {
			return w.log(ctx, OutcomeAbandoned, open, nil)
		}
		return nil
	case actMarkRead:
		if err := w.save(ctx, StatusRead, reading{}); err != nil {
			return err
		}
		return w.log(ctx, OutcomeFinished, reading{}, in.EndedOn)
	case actRemove:
		if open.hasProgress() {
			if err := w.log(ctx, OutcomeAbandoned, open, nil); err != nil {
				return err
			}
		}
		if err := w.q.DeleteLibraryEntry(ctx, db.DeleteLibraryEntryParams{UserID: w.userID, BookID: w.bookID}); err != nil {
			return fmt.Errorf("kitaplık kaydı silinemedi: %w", err)
		}
		return nil
	}
	return fmt.Errorf("bilinmeyen etki: %d", st.act)
}

func (w writer) start(ctx context.Context, in CommandInput) error {
	r, err := snapshot(ctx, w.q, w.bookID, in)
	if err != nil {
		return err
	}
	return w.save(ctx, StatusReading, r)
}

// resume, "Devam et": en son kapanan okuma yarım bırakılmışsa alanları satıra geri döner ve günlük
// satırı silinir (aynı transaction: yarıda kalırsa ikisi de geri alınır). Son kayıt bitirilmiş bir
// okumaysa ya da hiç kayıt yoksa (Faz 15'ten kalan yarım bırakma) baştan başlar.
func (w writer) resume(ctx context.Context, in CommandInput) error {
	last, err := w.q.LatestReadingLog(ctx, db.LatestReadingLogParams{UserID: w.userID, BookID: w.bookID})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && last.Outcome != OutcomeAbandoned) {
		return w.start(ctx, in)
	}
	if err != nil {
		return fmt.Errorf("son okuma okunamadı: %w", err)
	}
	if _, err := w.q.DeleteReadingLog(ctx, db.DeleteReadingLogParams{ID: last.ID, UserID: w.userID}); err != nil {
		return fmt.Errorf("günlük satırı geri alınamadı: %w", err)
	}
	return w.save(ctx, StatusReading, reading{
		EditionID: last.EditionID, Unit: last.Unit, Progress: last.Progress,
		TotalPages: last.TotalPages, StartedOn: last.StartedOn,
	})
}

// save, rafın tek yazma yolu (SaveShelf): version artar. Kitap bu arada yayından kalktıysa 404.
func (w writer) save(ctx context.Context, status Status, r reading) error {
	_, err := w.q.SaveShelf(ctx, db.SaveShelfParams{
		UserID: w.userID, BookID: w.bookID, Status: status,
		EditionID: r.EditionID, Unit: r.Unit, Progress: r.Progress, TotalPages: r.TotalPages, StartedOn: r.StartedOn,
	})
	return notFound(err, "raf yazılamadı")
}

func (w writer) log(ctx context.Context, outcome string, r reading, endedOn *time.Time) error {
	err := w.q.InsertReadingLog(ctx, db.InsertReadingLogParams{
		UserID: w.userID, BookID: w.bookID, EditionID: r.EditionID, Outcome: outcome,
		Unit: r.Unit, Progress: r.Progress, TotalPages: r.TotalPages, StartedOn: r.StartedOn, EndedOn: endedOn,
	})
	if err != nil {
		return fmt.Errorf("günlük satırı yazılamadı: %w", err)
	}
	return nil
}

// snapshot, yeni bir okumanın alanları: baskı (verilen ya da varsayılan), birim (verilen ya da sayfa
// toplamı biliniyorsa sayfa, yoksa yüzde), sayfa toplamının görüntüsü (kavram 7) ve 0 ilerleme.
// Varsayılan baskısı olmayan kitapta okuma baskısız ve toplamı bilinmeyen başlar.
func snapshot(ctx context.Context, q *db.Queries, bookID int64, in CommandInput) (reading, error) {
	r := reading{StartedOn: in.StartedOn}
	var pages *int32
	ed, err := q.GetReadingEdition(ctx, db.GetReadingEditionParams{BookID: bookID, EditionID: in.EditionID})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if in.EditionID != nil {
			return reading{}, ErrInvalidEdition
		}
	case err != nil:
		return reading{}, fmt.Errorf("baskı okunamadı: %w", err)
	default:
		r.EditionID = &ed.ID
		if ed.PageCount != nil && *ed.PageCount > 0 {
			pages = ed.PageCount
		}
	}
	unit := UnitPercent
	if pages != nil {
		unit = UnitPage
	}
	if in.Unit != nil {
		unit = *in.Unit
	}
	if unit != UnitPage && unit != UnitPercent {
		return reading{}, fmt.Errorf("%w: birim %q", ErrInvalidProgress, unit)
	}
	zero := int32(0)
	r.Unit, r.Progress = &unit, &zero
	if unit == UnitPage {
		r.TotalPages = pages // yüzdede toplam tutulmaz (0017'nin kuralı)
	}
	return r, nil
}

// validProgress, 0017'nin library_progress_valid kısıtının Go karşılığı: istemci 500 değil 400 görsün.
func validProgress(r reading) error {
	switch {
	case r.Unit == nil:
		if r.Progress != nil || r.TotalPages != nil {
			return fmt.Errorf("%w: birimsiz ilerleme", ErrInvalidProgress)
		}
	case *r.Unit == UnitPercent:
		if r.Progress == nil || *r.Progress < 0 || *r.Progress > 100 || r.TotalPages != nil {
			return fmt.Errorf("%w: yüzde 0–100 olmalı", ErrInvalidProgress)
		}
	case *r.Unit == UnitPage:
		if r.Progress == nil || *r.Progress < 0 || (r.TotalPages != nil && *r.Progress > *r.TotalPages) {
			return fmt.Errorf("%w: sayfa 0 ile toplam arasında olmalı", ErrInvalidProgress)
		}
	default:
		return fmt.Errorf("%w: birim %q", ErrInvalidProgress, *r.Unit)
	}
	return nil
}

// validDates: ikisi de biliniyorsa bitiş başlangıçtan önce olamaz (reading_log'daki CHECK'in Go karşılığı).
func validDates(startedOn, endedOn *time.Time) error {
	if startedOn != nil && endedOn != nil && endedOn.Before(*startedOn) {
		return ErrInvalidDates
	}
	return nil
}

func requireApproved(ctx context.Context, q *db.Queries, bookID int64) error {
	ok, err := q.BookIsApproved(ctx, bookID)
	if err != nil {
		return fmt.Errorf("kitap durumu okunamadı: %w", err)
	}
	if !ok {
		return books.ErrNotFound
	}
	return nil
}

// lockShelf: (kullanıcı, kitap) kilidi, sonra satır FOR UPDATE. nil = kitaplıkta değil.
func lockShelf(ctx context.Context, q *db.Queries, userID, bookID int64) (*db.LibraryEntry, error) {
	if err := q.LockShelfKey(ctx, db.LockShelfKeyParams{UserID: userID, BookID: bookID}); err != nil {
		return nil, fmt.Errorf("raf kilidi alınamadı: %w", err)
	}
	e, err := q.LockShelf(ctx, db.LockShelfParams{UserID: userID, BookID: bookID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("raf okunamadı: %w", err)
	}
	return &e, nil
}

// load, güncel raf: kayıt (okuma alanları yalnız READING iken) ve geçmiş.
func load(ctx context.Context, q *db.Queries, userID, bookID int64) (Shelf, error) {
	out := Shelf{BookID: bookID}
	e, err := q.GetShelf(ctx, db.GetShelfParams{UserID: userID, BookID: bookID})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return Shelf{}, fmt.Errorf("raf okunamadı: %w", err)
	default:
		if e.Status != StatusReading {
			e.EditionID, e.Unit, e.Progress, e.TotalPages, e.StartedOn = nil, nil, nil, nil, nil
		}
		out.Entry = &e
	}
	if out.Log, err = q.ListReadingLog(ctx, db.ListReadingLogParams{UserID: userID, BookID: bookID}); err != nil {
		return Shelf{}, fmt.Errorf("okuma geçmişi okunamadı: %w", err)
	}
	return out, nil
}

// notFound: pgx.ErrNoRows → books.ErrNotFound; başka hata bağlamıyla sarılır; nil → nil.
func notFound(err error, what string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, pgx.ErrNoRows):
		return books.ErrNotFound
	}
	return fmt.Errorf("%s: %w", what, err)
}
