# Go Conventions — How We Write Go in BookWise

> **Hibrit doküman:** kurallar ve kod **İngilizce** (kodla eşleşsin), kısa gerekçe notları
> **Türkçe**. Bu doküman _nasıl_ yazdığımızı tanımlar; [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) _neden_
> (TS/JS'e kıyasla) sorusunu cevaplar; ikisi birbirini tamamlar.

These conventions exist so the project teaches **idiomatic Go**, not "JS written in Go." They
are cheap to adopt on day one and expensive to retrofit later. The Codex/agent working
instructions (PROJECT_BRIEF.md §16) must follow these from Phase 1.

> **Neden gün 1'de?** Özellikle `context.Context` ve hata sarmalama, sonradan her imzaya
> eklemeye kalkınca acı verir. Baştan koy.

---

## 1. `context.Context` is the first parameter — everywhere

```go
func (s *Service) CreateRating(ctx context.Context, in CreateRatingInput) (Rating, error)
func (r *Repository) InsertRating(ctx context.Context, arg InsertRatingParams) (Rating, error)
```

- `ctx` originates from `r.Context()` in the handler and is threaded into **every** service →
  repo → `pgx` (and later Redis) call. All `pgx` methods take `ctx` first.
- Use it for per-request timeouts/deadlines and cancellation.
- Never store a `ctx` in a struct; always pass it.

> **TR:** Bu, [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) §7'nin kanıt bölgesi. İptal edilebilirlik ve
> timeout'lar buradan gelir.

---

## 2. Errors are values — wrap with `%w`, define sentinels, map in ONE place

```go
// per-domain sentinel errors
var (
    ErrBookNotFound    = errors.New("book not found")
    ErrDuplicateRating = errors.New("user already rated this book")
)

// wrap with context as it bubbles up
if err != nil {
    return Rating{}, fmt.Errorf("insert rating: %w", err)
}

// inspect with errors.Is / errors.As
if errors.Is(err, ErrDuplicateRating) { /* ... */ }
```

- **Map domain errors → HTTP status in exactly one place** (the handler layer / a shared
  `httperr` helper). Handlers translate; services/repos never write HTTP status.
- `409` for `ErrDuplicateRating`, `404` for `ErrBookNotFound`, `422` for validation, etc.
- Avoid stringly-typed errors (`errors.New(fmt.Sprintf(...))` for control flow).

> **TR:** §14.4 "explicit errors" ilkesinin mekanizması bu. [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) §2.

---

## 3. Dependency injection by hand in `main.go` — no framework

```go
db := mustConnect(ctx, cfg.DatabaseURL)
bookRepo := books.NewRepository(db)
bookSvc  := books.NewService(bookRepo)        // Service only where logic exists (see §4)
srv      := server.New(cfg, bookSvc, /* ... */)
```

- No `fx`/`wire`/`dig`. Hand-wiring **is** the DI lesson — you read the whole graph in one
  file.
- Define an **interface at the consumer**, and only when a test fake or a second
  implementation actually needs it. Otherwise pass the concrete `*Repository`.

> **TR:** TS/OOP geçmişinin getirdiği "her struct'a interface" refleksinden kaçın.
> [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) §3.

---

## 4. Layering is conditional, not mandatory

`Handler → Service → Repository` is the **target**, not a day-one rule.

- **Pure CRUD (Phase 2 books/genres):** handler may call the repository directly. A Service
  that only forwards calls is an **anemic layer** — don't write it.
- **Add a Service layer where real logic lives:** auth (Phase 3), moderation/status
  transitions (Phase 4), weighted-score recalculation (Phase 6).
- Collapse layers until a decision needs a home.

> **TR:** Boş geçiş borusu katman = cargo-cult. Mantık çıkınca katmanı ekle.

---

## 5. Server lifecycle: timeouts + graceful shutdown + fail-fast config

```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()

srv := &http.Server{
    Addr:         cfg.Addr,
    Handler:      router,
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 10 * time.Second,
}
// ... run in a goroutine, then on <-ctx.Done(): srv.Shutdown(shutdownCtx)
```

- **Config:** load from env, **validate at boot**, and `log.Fatal` on anything missing or
  malformed. Fail fast — never nil-panic at request time. (12-factor: config lives in the
  environment.)
- `.env` is gitignored; only `.env.example` is committed.

> **TR:** [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) §8. K8s fazında `SIGTERM` ile bunun değerini görürsün.

---

## 6. Database: sqlc + pgx, no ORM

- **Committed choice:** `sqlc` (input) + `pgx/v5` (driver). **GORM/bun are out** — an ORM
  defeats §2's goal of learning how relationships actually work.
- Directory split (also note it in §8 of the brief):
  - `backend/sql/queries/*.sql` — hand-written SQL, the **sqlc input**.
  - `backend/migrations/*.sql` — goose migrations (schema), separate from queries.
  - `backend/sqlc.yaml` — sqlc config at the backend root.
- **Avoid import cycles between domain packages.** A comment references a user and a book; a
  rating references both. Reference cross-domain entities **by ID** (`BookID`, `UserID`), or
  put shared entities in a small `domain/` package. Domain packages depend on shared types,
  **not on each other.**
- Read the generated Scan code at least once — it shows what correct `pgx` looks like.

> **TR:** Import cycle, ilk haftanın klasik Go duvarıdır; ID ile referans vererek baştan engelle.
> [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) §6.

---

## 7. Data consistency for derived values (ratings)

The single most important correctness rule in this app:

- **`ratings` is the source of truth.** Never app-read-then-write `average_rating` /
  `rating_count` (lost-update race).
- Enforce one-rating-per-user at the DB: `UNIQUE(user_id, book_id)` +
  `INSERT ... ON CONFLICT (user_id, book_id) DO UPDATE` (satisfies both "rate once" and
  "update rating").
- Recompute aggregates **inside the same transaction** as the rating write, via SQL
  `AVG`/`COUNT` — or compute at read time.
- **`weighted_score` is not a write-denormalized column.** Because `C` (global average) and
  `m` are global, any new rating anywhere invalidates every book's score. Compute it at query
  time with parameterized `C`/`m`, or via a periodically-refreshed materialized view when you
  can **measure** the sort is slow. Start `m` small (~10), and derive `C` from `AVG(score)`
  with a sane cold-start fallback.

> **TR:** [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) §10. "Transaction neden var" dersinin tam yeri.

---

## 8. Tests: table-driven, standard `testing`, `-race`

```go
func TestWeightedScore(t *testing.T) {
    tests := []struct {
        name           string
        r              float64
        v, m           int
        c, want        float64
    }{
        {"few ratings pulled toward C", 10, 1, 100, 7.0, 7.03},
        {"many ratings near own mean", 8.5, 2000, 100, 7.0, 8.43},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := WeightedScore(tt.r, tt.v, tt.m, tt.c)
            // assert with a tolerance ...
        })
    }
}
```

- Std `testing` + `t.Run` subtests; no testify-heavy assertion frameworks to start.
- Keep the weighted-rating math a **pure function** (no DB) so it's trivially unit-testable.
- Run with `-race` early to surface concurrency bugs.
- Integration/repository tests run against the Docker Compose Postgres (or `testcontainers-go`)
  — don't mock `pgx`; mocking the DB teaches little.

> **TR:** [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) §13. Ağırlıklı puanlama bayrak testi (§6.7).

---

## 9. Tooling & formatting

- `gofmt` (no formatting debates), `go vet`, and `golangci-lint` are the standard quality
  gate. Run them locally and in CI (Phase 10).
- For a JS background: the **zero-config** formatting/vetting culture is itself a learning
  point — there is no eslint/prettier config to bikeshed.

---

## 10. Logging: `slog`, context-aware

- Use stdlib `slog` (Go 1.21+). It integrates with `context` — a handler/`slog` handler can
  pull a request/trace ID from `ctx`, reinforcing the §1 convention.
- `zerolog` is an optional later experiment; its perf edge is irrelevant at this scale and it
  adds a third-party API to learn.

---

## 11. DTOs vs domain models, validation at the boundary

- Request/response **DTOs** are separate from domain models. Bind JSON into a DTO, validate
  it (`go-playground/validator`) in the **handler**, then map to the domain type.
- This also closes a security hole — see §12 (mass assignment).

---

## 12. Security baseline (must learn correctly the first time)

> This section fills the gap left by the brief and is summarized into PROJECT_BRIEF.md §6.1.
> Slow down on **Phase 3** — it is the security-sensitive surface.

- **Password hashing:** `bcrypt` (cost ≥ 12) or `argon2id`. **Never** SHA/MD5.
- **JWT:**
  - Short-lived access tokens (~15 min).
  - **Explicitly verify the signing algorithm** when parsing (reject `alg=none` and
    algorithm-confusion). Pin to your expected method (e.g. HS256).
  - Secret comes from env, never committed. Set `exp`/`iat`/`iss`/`aud`.
- **"Logout" with stateless JWT:** you cannot truly revoke a JWT. Use the DB-backed
  `refresh_tokens` table (§9): rotate the refresh token on use; **logout = delete the refresh
  token**. Don't reach for a Redis blacklist just to log out.
- **Authorization in the service layer + resource ownership (IDOR):**
  `PATCH`/`DELETE /books/{id}/ratings` must verify the rating **belongs to the requesting
  user** — not merely that they are authenticated. Coarse role gating (admin) can live in
  middleware; ownership checks live in the service.
- **Mass assignment:** when an admin edits a suggested book before approval (§6.3), bind to a
  **whitelisted DTO** — never bind raw JSON into the domain struct (or a user could set
  `status=APPROVED` or `role`).
- **Injection:** always use parameterized queries via `pgx`/`sqlc`. Never string-concatenate
  the `ILIKE` search (§6.8).
- **Stored XSS:** comments are user content rendered by the React frontend. Store raw, escape
  on render (React escapes by default — beware `dangerouslySetInnerHTML` / markdown
  rendering). Sanitize server-side if you ever render HTML.
- **Rate limiting:** protect `login`/`register` from brute force (§6.9). In-memory limiter is
  fine early; Redis counters in Phase 7.
- **Tokens (email-verify / password-reset):** single-use, expiring, generated with
  `crypto/rand`, stored **hashed**.
- **Auth error hygiene:** generic message ("invalid credentials") — don't reveal whether an
  email exists. **CORS:** explicit origin allowlist (no wildcard with credentials).
- **Secrets:** `.env` gitignored; only `.env.example` committed. A committed secret is
  compromised even after deletion — rotate it.

> **TR:** [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) §2 (UNIQUE ihlali → 409) ve §10 (eşzamanlılık) bu
> kararlarla kesişir. Faz 3'te yavaşla.

---

## Quick checklist (paste into each PR/commit description)

- [ ] `ctx context.Context` first param on new service/repo methods
- [ ] errors wrapped with `%w`; domain sentinels mapped to HTTP status in one place
- [ ] no anemic Service layer for pure CRUD
- [ ] config validated at boot; `.env` not committed
- [ ] SQL parameterized; no ORM; sqlc input under `sql/queries/`
- [ ] table-driven tests for new logic; `go test -race` green
- [ ] `gofmt` + `go vet` + `golangci-lint` clean
- [ ] security baseline (§12) considered for any auth/input/render surface
- [ ] relevant [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) placeholder updated if this phase produced illustrating code
