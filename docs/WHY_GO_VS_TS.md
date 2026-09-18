# Why Go over TS/JS for the BookWise Backend — A Living, Code-Anchored Comparison

> **Bu doküman hibrittir:** teknik açıklamalar ve kod referansları **İngilizce** (kodla
> birebir eşleşsin diye), "beni şaşırtan / doğrulandı mı çürütüldü mü" **refleksiyonları
> Türkçe** (öğrenme notların senin dilinde olsun diye).

This is **not** a "Go beats Node" essay. It is a judgment-building log tied to the brief's
central question ([PROJECT_BRIEF.md](PROJECT_BRIEF.md) §1, §2, §17):

> _Which problem does each technology solve, and when is it worth adding?_

Every section below is a **hypothesis to confirm or falsify against real BookWise code** —
not a conclusion copied from a blog post. Until the illustrating code exists, a section
holds a placeholder like `[Phase 6 — backend/internal/ratings/score.go — TBD]`. **A
placeholder is a feature, not a gap:** it marks a claim you have not yet earned.

---

## Design principles (read once)

1. **Hypotheses, not verdicts.** A section is "done" only when it cites a real
   `file:symbol` you wrote *and read*, plus a short reflection on whether the code
   **confirmed** or **falsified** the hypothesis.
2. **Anchored to a feature + phase.** No abstract Go-vs-Node prose. If a Go concept has no
   concrete BookWise illustration, it does not get a section.
3. **Calibrated to scale.** Each section answers "does this difference matter *for what I'm
   building*?" For most of the brief's §10 I/O-bound CRUD, the language barely matters — and
   this doc says so plainly.
4. **Mandatory honest caveat.** Every section reserves a _"Where TS/JS is competitive or
   better"_ note. A section that can't name one is treated as evangelism and flagged.
5. **Ordered by when the code gets written.** Phase 1–2 topics first (you can fill them
   soonest); concurrency, perf, deployment footprint come later (Phase 6–12).
6. **Measured, not claimed.** Any performance/footprint claim must hold a real BookWise
   number (`go test -benchmem`, `docker images`, `hey`/`k6`, `/usr/bin/time` RSS) or stay
   tagged `UNVERIFIED-PERF`.
7. **Language differences ≠ library differences.** The low-magic stack (Chi + pgx + sqlc,
   §7.1) is chosen precisely so the comparison shows Go-vs-Node, not Chi-vs-NestJS. Call out
   when a perceived "Go win" is really just "I picked a thinner library."

### Status legend

| Tag | Meaning |
| --- | --- |
| `HYPOTHESIS` | No code yet — placeholder. The honest default at brief stage. |
| `CONFIRMED` | Code written + read; the hypothesis held. |
| `FALSIFIED` | Code written + read; the hypothesis did **not** hold (the most valuable outcome). |
| `PARTIAL` | True in some respects, not others. |
| `UNVERIFIED-PERF` | A perf/footprint claim without a measured BookWise number yet. |

Every section starts at `HYPOTHESIS`. That is correct, not incomplete.

---

## 0. How to use this document

**Status:** `HYPOTHESIS` (meta)

This is the index + the rules. The fill-in protocol and per-phase checklist live at the
[bottom](#living-document-mechanism). The one rule that keeps the whole thing honest:
**every Go section must grow at least one entry in §15 (where TS/JS wins) when you fill it.**

> **Refleksiyon (TR):** Bu dokümanı, her fazın sonunda "bu faz hangi bölümlere gerçek kod
> üretti?" diye sorup ilgili placeholder'ı doldurarak büyüteceksin. Sonuç pekâlâ "bunların
> birçoğunda TS/JS de gayet yeterdi" olabilir — **bu bir başarısızlık değil, tam da
> öğrenmek istediğin yargının kendisi.**

---

## 1. Static typing & compile-time enum safety

**Status:** `PARTIAL` (Faz 4) · **Illustration:** `backend/internal/books/repository.go:ParseStatus` (sınır), `backend/internal/moderation/transitions.go:Next` (durum makinesi)
>
> **🟡 Faz 4:** Sınırda parse gerçekten iş gördü: `?status=approved` ya da `DELETED` sorguya ulaşıp sessizce boş liste döndürmüyor, 400 oluyor. Ama dürüst olmak gerekirse, `type BookStatus string` tanımlamadık — sqlc `db.Book.Status`'u `string` üretiyor ve her sınırda dönüştürmek, bu ölçekte kazandırdığından fazla tören getirirdi. Asıl kazanç tipten değil, **durum makinesini VERİ olarak** yazmaktan geldi (`transitions` map'i): 4×4'lük tablo testi yasak 12 geçişin hepsini kanıtlıyor. TS'te discriminated union + exhaustive `switch` bunu derleme zamanında yapardı — Go'da çalışma zamanı testiyle yapıyoruz (bkz. §15 #18).

- **Hypothesis:** Go's typed constants + a single parse-at-the-edge boundary make an invalid
  `status` *unrepresentable* past the HTTP layer, catching a class of bug earlier than plain
  JS and at least as early as TS.
- **Go concept:** A string-backed defined type (`type BookStatus string`) with a `const`
  block and a `ParseBookStatus(s string) (BookStatus, error)` boundary. Go has no native sum
  type, so the runtime parse boundary is what makes invalid values impossible downstream.
- **TS/JS contrast:** TS string-literal unions (`'PENDING' | 'APPROVED' | ...`) are
  compile-time only and **erased at runtime** — a bad value from JSON flows straight through
  unless you add zod/io-ts. Plain JS gives nothing until it explodes.
- **Where TS/JS is competitive or better:** TS discriminated unions + an exhaustive `switch`
  (with a `never` fallthrough) are arguably *more ergonomic* than Go's const blocks, and the
  compiler can force exhaustiveness — something Go can't do for its fake enums. Go's edge is
  the unavoidable runtime parse boundary, not a richer type system.
- **BookWise anchors:** book status `PENDING/APPROVED/REJECTED/ARCHIVED` (§6.2), comment
  status `VISIBLE/DELETED/HIDDEN` (§6.5), roles `USER/ADMIN/MODERATOR` (§6.1).

> **Refleksiyon (TR — kod yazılınca doldur):** `ParseBookStatus`'u yazınca, geçersiz status'ü
> sınırda (handler) yakalamak gerçekten kolaylaştı mı? TS union'a göre fazladan yazdığın
> "parse boundary" sana ağır mı geldi yoksa güven mi verdi?

---

## 2. Errors as values vs try/catch

**Status:** `PARTIAL` (Faz 2 + Faz 3) · **Illustration:** `backend/cmd/api/auth.go:handleRegister` (sentinel + tipli hata → HTTP tek `switch`'te), `backend/internal/auth/token.go:Verify` (çoklu `%w`), `[Phase 5 — ratings 409 — TBD]`
>
> **🟡 Faz 3:** Hata-değer modeli iki yerde gerçekten iş gördü. (1) `auth.ValidationError` TİPLİ hata: servis doğrulama problemlerini taşıyor, handler `errors.As` ile yakalayıp 400'e çeviriyor — servis HTTP'yi hiç tanımıyor. (2) `fmt.Errorf("%w: %w", ErrInvalidToken, err)`: istemci tek bir sentinel görüyor (401), log'da kütüphanenin asıl nedeni (süre mi, imza mı) duruyor. Güvenlik açısından önemli: "neden reddedildi" bilgisi hata ZİNCİRİNDE kalıyor, cevaba sızmıyor. Tam `CONFIRMED` için Faz 5'in UNIQUE → 409 örneği bekleniyor.

- **Hypothesis:** Returning errors as values (with `%w` wrapping) and mapping domain
  sentinel errors to HTTP status in **one place** produces a clearer, harder-to-ignore error
  path than JS try/catch + unhandled promise rejections. Realizes brief principle §14.4.
- **Go concept:** `fmt.Errorf("create rating: %w", err)`, sentinel/typed errors
  (`ErrDuplicateRating`, `ErrBookNotFound`), inspected with `errors.Is`/`errors.As`. The
  `if err != nil` line makes the unhandled path a visible, unignorable statement.
- **TS/JS contrast:** `try/catch` where any value can be thrown (`catch (e: unknown)`), and
  the silent-failure mode of a forgotten `await`. TS has no checked exceptions — the compiler
  never forces you to handle a failure path.
- **Where TS/JS is competitive or better:** Go's `if err != nil { return err }` is genuinely
  verbose for deep call stacks; `try/catch` is more compact for "just bubble it up."
  `neverthrow`/Result types give TS a comparable values-based model when wanted. The win here
  is **enforcement**, not elegance.
- **BookWise anchors:** the "rate a book only once" rule (§6.6) — a Postgres
  `UNIQUE(user_id, book_id)` violation surfaced by pgx, mapped to a clean **409**; moderation
  approve/reject (§6.3).

> **Refleksiyon (TR):** UNIQUE ihlalini pgx'ten alıp 409'a tek yerde map'lediğinde, JS'teki
> "hangi catch nerede" dağınıklığına kıyasla daha mı öngörülebilir oldu? `if err != nil`
> tekrarından sıkıldın mı?

---

## 3. Explicit DI & small interfaces vs duck typing / DI containers

**Status:** `CONFIRMED` (Faz 1–3) · **Illustration:** `backend/cmd/api/main.go` (elle DI), `backend/internal/auth/service.go:Service` (ilk servis), `backend/cmd/api/books.go:bookStore` (tüketen tarafta interface), `backend/cmd/api/router.go:deps`.
>
> **✅ Faz 1:** grafı `main.go`'da elle kurmak net ve okunur. Henüz interface yok — gerçek bir fake / 2. implementasyon gerekmedi, o yüzden eklemedik (GO_CONVENTIONS §3). Hipotez bu ölçekte doğrulandı.
>
> **✅ Faz 3:** İlk **Service** katmanı gerçek iş mantığıyla doğdu (`internal/auth/service.go`): parola politikası, zamanlama eşitleme, token üret-sakla-döndür. Kitap/tür CRUD'unda hâlâ servis yok — ayrım işe yaradı. İki DI kazancı somut: (1) aynı `Service`, hem HTTP handler'ı hem `cmd/manage` CLI'ı tarafından kullanılıyor → parola politikası iki kapıda da tek kaynaktan uygulanıyor; (2) `compare` fonksiyonu alan olarak enjekte edildiği için "kullanıcı yokken de bcrypt çalıştı mı" iddiası süre ölçmeden, çağrı sayarak test edilebildi. Ayrıca `auth.User` (PasswordHash alanı YOK) — hash'in cevaba sızması derleyici seviyesinde imkânsız.
>
> **✅ Faz 2:** İlk interface'ler **gerçek bir ihtiyaçla** doğdu: handler'ları DB'siz test etmek. `bookStore`/`genreStore` TÜKETEN tarafta (`cmd/api`) tanımlı; `*books.Repository` onları "implements" demeden sağlıyor. Kazanç somut: `countingBooks` sahtesi çağrıları sayıp liste ucunun N+1 yapmadığını kanıtlıyor — DI container'ında bu kadar ucuz olmazdı. Maliyeti de somut: interface büyüdükçe (Get → List/Create/SetGenres) her sahte tüm metotları uygulamak zorunda; struct embedding (`capturingBooks{fakeBooks}`) bunu hafifletti. Router parametre listesi uzamaya başlayınca `deps` struct'ına geçtik — hâlâ elle DI, ama testler yalnızca ihtiyaç duydukları alanı dolduruyor.

- **Hypothesis:** Wiring dependencies by hand in `main.go` (no framework) keeps the whole
  object graph readable top-to-bottom and teaches DI better than a container that resolves it
  at runtime.
- **Go concept:** Plain constructors (`NewBookRepository(db)`, `NewServer(...)`) assembled in
  `main.go`. Interfaces defined **at the consumer**, only once a test fake or a second
  implementation needs one — otherwise pass the concrete `*Repository`.
- **TS/JS contrast:** NestJS decorator/reflection DI containers; or ad-hoc duck typing. Go
  interfaces are *also* structurally satisfied, so the real contrast is "explicit constructor
  graph you can read" vs "a container resolves your graph via decorators you must trust."
- **Where TS/JS is competitive or better:** For a large app with many cross-cutting deps, a
  DI container removes real wiring boilerplate that hand-wiring accumulates. At BookWise's
  monolith scale, hand-wiring stays readable — so this is a "right size for the project" win,
  not an absolute one.
- **⚠️ Layering caveat (from review):** **Skip the Service layer for pure CRUD** (Phase 2
  books/genres) and introduce it only where real logic lives — auth (Phase 3), moderation
  (Phase 4), weighted score (Phase 6). Anemic pass-through layers are the cargo-cult trap a
  TS/OOP background invites. See [GO_CONVENTIONS.md](GO_CONVENTIONS.md).

> **Refleksiyon (TR):** `main.go`'da grafı elle kurarken "bağımlılık nereden geliyor"
> sorusunu tek dosyada görmek Nest'in decorator sihrine göre daha mı net geldi?

---

## 4. Visible HTTP (Chi / net/http) vs framework abstraction

**Status:** `CONFIRMED` (Faz 1) · **Illustration:** `backend/cmd/api/router.go:newRouter` (Chi, net/http uyumlu; dosya `internal/server` yerine `cmd/api`'de — tek handler için erken paketleme yapmadık).
>
> **✅ Faz 1:** Chi handler imzaları net/http ile birebir aynı; öğrendiğim hiçbir şey çöpe gitmedi. Ama dürüst: gördüğüm "şeffaflık"ın çoğu **library seçimi** (Chi vs Express), dil farkı değil — caveat doğrulandı.

- **Hypothesis:** Chi keeps HTTP mechanics (request, `http.ResponseWriter`, status codes,
  middleware) visible, so nothing learned about `net/http` is throwaway. Matches §7.1's
  stated reason.
- **Go concept:** Chi is std-lib-compatible (`http.Handler`, standard middleware signature).
  Routing and middleware chaining stay explicit.
- **TS/JS contrast:** Express `req`/`res`, or Fastify/Nest lifecycles wrapping raw `node:http`.
- **Where TS/JS is competitive or better — and an honesty check:** This is **largely a
  library choice, not a language one.** You can pick a thin router in TS too (or raw
  `node:http`). Express/Fastify have a far bigger middleware ecosystem and faster route
  prototyping. The "win" here is conceptual transparency, which only matters *because the goal
  is learning*. Do not credit Go for what is really Chi-vs-Express.
- **BookWise anchors:** health check + book-list (§10.2, Phase 1), auth middleware (§6.1,
  Phase 3), request-ID/logging middleware.

> **Refleksiyon (TR):** Bu bölümün tuzağı: "Go HTTP'yi şeffaf yapıyor" diye yazma — asıl fark
> Chi vs Express. Kendine dürüst ol: gördüğün şeffaflık dilden mi, ince library seçiminden mi?

---

## 5. JSON (de)serialization & validation at the boundary

**Status:** `CONFIRMED` (Faz 2) · **Illustration:** `backend/cmd/api/respond.go:readJSON`, `backend/cmd/api/validate.go:validator`, `backend/cmd/api/books.go:createBookRequest`/`bookResponse`.
>
> **◑ Faz 1:** JSON yazma ve tutarlı hata zarfı vardı; doğrulama yoktu.
>
> **✅ Faz 2:** İstek ve cevap DTO'ları DB modelinden (`db.Book`) ayrıldı ve bunun neden şart olduğunu **canlı** gördük: ilk taslak gövdeyi doğrudan `db.CreateBookParams`'a okuyordu ve `{"status":"APPROVED"}` gönderen istemci kitabını kendi yayına aldı (mass-assignment). `DisallowUnknownFields` + ayrı request struct ikinci ve birinci savunma hattı oldu. **Karar:** `go-playground/validator` EKLENMEDİ — 60 satırlık bir `validator` (biriktiren, rune sayan, kalıp/URL denetleyen) ihtiyacı karşıladı; struct tag'lerine gömülü DSL yerine düz Go kodu okunur ve test edilir kaldı (§14.5). **Dürüst not:** zod'un "tek şema → tip + doğrulama" ergonomisi burada gerçekten daha iyi — Go'da request struct'ı, `normalize()` ve `validate()` üç ayrı yerde yaşıyor.

- **Hypothesis:** Go struct tags + `go-playground/validator` on request DTOs give a
  compile-time-known shape with co-located validation rules, vs untyped `JSON.parse` recovered
  by a runtime validator.
- **Go concept:** `json:"..."` tags for (de)serialization; `validate:"required,min=1,max=10"`
  on a DTO struct; validation runs in the handler. Introduces the DTO-vs-domain separation.
- **TS/JS contrast:** `JSON.parse` returns `any`; zod/joi bolted on at runtime recovers a
  type.
- **Where TS/JS is competitive or better:** zod's **schema-first inference** (one declaration
  yields *both* validation and the static type) is cleaner than maintaining a Go struct + tags
  + a separate validated DTO. TS arguably wins on ergonomics here; Go wins only on "no runtime
  type recovery needed for the happy path."
- **BookWise anchors:** suggest-book body (§10.2 `POST /api/books/suggestions`), rating body
  1–10 (§10.5).

> **Refleksiyon (TR):** zod'un "tek şema → hem tip hem doğrulama" akışını özledin mi, yoksa
> struct-tag + validator ayrımı sana daha mı açık geldi?

---

## 6. Typed DB access: sqlc + pgx vs Prisma / Knex / TypeORM

**Status:** `CONFIRMED` (Faz 2) · **Illustration:** `backend/sql/queries/books.sql`, `book_genres.sql`, üretilen `backend/internal/db/*.go`; `[Phase 5–6 — rating aggregation queries — TBD]`
>
> **✅ Faz 2:** Hipotez doğrulandı ve dört somut kanıt birikti. (1) `SELECT *` üretim anında açık 12 kolonluk listeye genişledi — şema değişirse regenerate derleme hatası verir. (2) Çoka-çok JOIN için sqlc sorguya özel `ListGenresForBooksRow` üretti; ORM'in `book.genres` diye sakladığı şeyi SQL olarak yazdık. (3) **N+1'i bilerek önledik:** liste için `book_id = ANY($1)` ile tek sorgu + Go'da gruplama; bir test çağrı sayısını sayıyor. Naif bir ORM döngüsü 20 kitapta 21 sorgu atardı. (4) Opsiyonel filtreler (`genre`, `q`) için `(sqlc.narg IS NULL OR ...)` deseni: sqlc'nin dinamik sorgu zayıflığının standart çözümü — sorgu metni sabit, filtreyi değer açıp kapatıyor. **Gerçek bir tuzak da yaşadık:** parametreli INSERT'te DB `DEFAULT`'u devreye girmez (`language` boş string olarak gitti) → varsayılanı uygulama koymak zorunda (öğrenme notu database #5). **Maliyet:** her SQL değişikliği `sqlc generate` + commit; opsiyonel filtre deseni sorguyu okunaksızlaştırıyor.

- **Hypothesis:** sqlc — which **reads your hand-written SQL and generates type-safe Go** —
  lets you learn real SQL (joins, the many-to-many) while eliminating the `rows.Scan` /
  `rows.Err()` / NULL-handling footguns, giving a cleaner "compile-time-checked queries"
  story than an ORM that hides SQL.
- **Go concept:** Write SQL in `sql/queries/`, run `sqlc generate`, get typed structs +
  methods. **Not an ORM:** no query DSL, no magic. Reviewers recommend committing to sqlc+pgx
  from Phase 2 and **explicitly excluding GORM/bun** (an ORM defeats §2's "learn how
  relationships work" goal).
- **TS/JS contrast:** Prisma (schema-first, generated client, hides SQL, occasional N+1
  surprises), Knex (query builder), TypeORM (decorators/ActiveRecord). Clean contrast: sqlc
  *verifies your SQL against the schema at build time*; Prisma *generates SQL from a schema and
  hides it.*
- **Where TS/JS is competitive or better:** Prisma is a faster path to a working data layer,
  with best-in-class migrations and a great client DX for the I/O-bound CRUD that is most of
  §10. If the goal were shipping rather than learning SQL, Prisma would be defensible. sqlc
  wins **specifically because seeing the SQL is the objective.**
- **BookWise anchors:** `book_genres` many-to-many join (§6.4/§9) — the query that would be an
  N+1 trap in a naive ORM — and the rating `AVG`/`COUNT` aggregation (§6.7). Note the
  directory split: `sql/queries/` (sqlc input) vs `migrations/` (goose).

> **Refleksiyon (TR):** Üretilen Scan kodunu okuyunca "doğru pgx böyle yazılırmış" dersini
> aldın mı? Prisma'nın sihrini özlediğin an oldu mu, yoksa SQL'i görmek mi kazandırdı?

---

## 7. context.Context propagation: cancellation, timeouts, deadlines

**Status:** `PARTIAL` (Faz 2 + Faz 7) · **Illustration:** `backend/cmd/api/books.go` (`r.Context()` → `store.Get(ctx, ...)`), `backend/internal/database/tx.go:WithTx`, `backend/internal/cache/cache.go:NewRedis` (3 sn PING deadline + 500 ms okuma/yazma zaman aşımı)
>
> **🟡 Faz 7:** Redis'te ctx + kısa zaman aşımları işe yaradı: yavaş bir Redis'i beklemek yerine "miss" sayıp DB'ye düşüyoruz; cache hatası isteği asla düşürmüyor (testli). Hâlâ ölçmediğimiz: istemci bağlantıyı kesince uçuştaki bir sorgunun GERÇEKTEN iptal edildiği — bu yüzden PARTIAL kalıyor.
>
> **◑ Faz 2:** `ctx` handler'dan pgx'e kadar kesintisiz iniyor ve transaction'lar da (`WithTx`) aynı ctx'e bağlı. Açılıştaki `Ping` gerçek bir deadline kullanıyor. **Ama dürüst olmak gerekirse** iptal edilen bir isteğin DB sorgusunu iptal ettiğini henüz ÖLÇMEDİK — kısa DB okumalarında bu disiplinin getirisi bu ölçekte görünmüyor. Kısmi: yapı hazır, kanıt Faz 7'de (Redis deadline) aranacak. Yan kazanç: review paneli, `ReadTimeout` dolunca gövde okumasının `i/o timeout` verdiğini ve hatanın iç IP:port'u sızdırdığını bulmuştu — deadline'lar gerçekten tetikleniyor.

- **Hypothesis:** `context.Context` as the first parameter everywhere gives uniform,
  pervasive cancellation/timeouts (a cancelled request cancels its DB query) that Node's
  newer, less-uniform `AbortController` adoption doesn't match.
- **Go concept:** `ctx` flows `r.Context()` → service → repo → every `pgx` call (all pgx
  methods take `ctx` first) → later every Redis call. `slog` can pull request/trace IDs from
  `ctx`, reinforcing the pattern.
- **TS/JS contrast:** `AbortController`/`AbortSignal` — newer, inconsistently threaded through
  DB drivers and libraries.
- **Where TS/JS is competitive or better:** `ctx` as the first arg of *every* function is real
  ceremony that clutters signatures, and most BookWise requests are short DB reads that never
  get cancelled — so the discipline pays off rarely at this scale. Node's single-threaded
  model also sidesteps a class of cancellation complexity.
- **Anchor:** per-request DB timeouts from Phase 2; Redis timeouts in Phase 7.

> **Refleksiyon (TR):** ctx'i her imzaya eklemek külfet mi hissettirdi yoksa "her şey iptal
> edilebilir" güveni mi verdi? Gerçekten iptal edilen bir istek yakaladın mı?

---

## 8. Graceful shutdown & fail-fast config

**Status:** `CONFIRMED` (Faz 1) · **Illustration:** `backend/cmd/api/main.go` (graceful shutdown) + `backend/internal/config/config.go:Load` (fail-fast).
>
> **✅ Faz 1:** SIGTERM gönderince "sunucu temiz kapandı" (exit 0) **canlı** görüldü; `PORT=abc` ile boot'ta durdu (fail-fast). Dürüst: dikkatli bir Node servisi de aynısını yapar; Go'nun stdlib'i sadece `Shutdown(ctx)` ile bunu daha az uğraşla veriyor.

- **Hypothesis:** Go's stdlib makes `http.Server` timeouts + `signal.NotifyContext` +
  `srv.Shutdown(ctx)` the idiomatic default, and fail-fast config validation at boot prevents
  the "missing env var → nil-panic on first request" class of bug.
- **Go concept:** `ReadTimeout`/`WriteTimeout`, graceful shutdown on `SIGINT`/`SIGTERM`,
  config loaded + **validated** at startup (panic early, not at request time). A small but
  real concurrency teaching moment.
- **TS/JS contrast:** `server.close()` + manual `process.on('SIGTERM')`; the common failure of
  starting with a missing env var that only blows up later.
- **Where TS/JS is competitive or better:** This is a **stdlib-and-discipline** difference,
  not a deep language one — a careful Node service does exactly this. For a local-first app you
  `Ctrl-C`, graceful shutdown is educational until the K8s phase (pod `SIGTERM`) makes it bite.
- **Anchor:** `main.go` bootstrap/shutdown; matters under Docker (Phase 8) and K8s (Phase 12).

> **Refleksiyon (TR):** Eksik env değişkeninde boot'ta patlamak, request anında nil-panic'e
> göre hata ayıklamayı kolaylaştırdı mı?

---

## 9. Concurrency: goroutines vs the Node event loop (CPU-bound fan-out)

**Status:** `FALSIFIED-FOR-THIS-PROJECT` (Faz 6) · **Illustration:** yok — bilerek.
>
> **❌ Faz 6 (dürüst not):** CPU-bound bir fan-out işi hiç doğmadı. Ağırlıklı puan query-time SQL'de hesaplanıyor (tek geçiş + LIMIT, 14 ms/20 bin kitap); "paralel recompute worker pool" yazmak hem gereksiz hem de "skor saklanmaz" kararıyla çelişirdi. Goroutine'ler bu projede yalnızca TESTTE eşzamanlı istemci üretmek için kullanıldı (25 eşzamanlı oy, iki admin yarışı) — orada gerçekten rahat. Hipotez bu projenin ölçeğinde test edilemedi; uydurma bir iş yaratmıyoruz.

- **Hypothesis:** Goroutines + `errgroup` / a bounded worker pool can recompute weighted
  scores across all books in **true parallel** across cores — work that Node's single thread
  either blocks on or pushes to `worker_threads`.
- **Go concept:** Keep the weighted-rating math a **pure function** (so it's both
  table-testable *and* safely parallelizable), then fan it out with a bounded pool.
- **TS/JS contrast:** Node's event loop: the same CPU-bound recompute blocks the loop;
  `Promise.all` gives concurrent **I/O**, not parallel **CPU**.
- **Where TS/JS is competitive or better — say it plainly:** BookWise will never have enough
  books for parallel recompute to matter. **Sequential is correct and simpler here.** Node's
  event loop is perfectly adequate (often simpler) for this app's I/O-bound reality. Go's
  parallelism is a real capability this project barely needs. The brief frames the concurrent
  variant as explicitly **optional** (Phase 6).
- **Anchor:** rating recalculation service (§6.7); optional cache warmup of top-rated +
  popular together (§6.9, Phase 7).

> **Refleksiyon (TR):** Paralel sürümü yazsan bile, bu ölçekte sequential'a göre ölçülebilir
> bir fayda gördün mü? "Yapabiliyor olmak" ile "gerekmek" arasındaki farkı burada hissettin mi?

---

## 10. Data-race detection & concurrent correctness (`go test -race` + DB transactions)

**Status:** `CONFIRMED` (Faz 2–5, bir düzeltmeyle) · **Illustration:** `backend/internal/ratings/service.go:Rate`, `backend/internal/ratings/service_integration_test.go:TestConcurrentRatingsRealDB`
>
> **✅ Faz 5 (düzeltmeli):** Hipotezin İKİNCİ yarısı güçlü biçimde doğrulandı: doğru çözüm DB'de. Kitap satırı `FOR UPDATE` ile kilitlenmeden, 25 eşzamanlı oy sonunda `rating_count` 16–19 çıktı (3 deneme) — klasik kayıp güncelleme; kilitle her seferinde 25. Ama hipotezin İLK yarısı ("`-race` gerçek hataları bedavaya yakalar") burada **yanlış çıktı**: bu hata Go belleğinde değil, iki Postgres transaction'ı arasındaydı; race detector hiçbir şey görmedi (paylaşılan Go değişkeni yok). `-race` bellekteki yarışlar içindir; veri yarışlarını ancak **eşzamanlı entegrasyon testi + mutation** yakalar. Aynı desen dört kez tekrarlandı: tür değiştirme (Faz 2), refresh token tüketme (Faz 3), moderasyon kararı (Faz 4), puan agregatı (Faz 5) — hepsinde çözüm dil değil, SQL (kilit ya da tek atımlık deyim).

- **Hypothesis:** `go test -race` surfaces real concurrency bugs for free, while the *correct
  fix* lives in the DB: derive aggregates inside the **same transaction** as the rating write,
  never app-read-then-write a counter. This is the reviewers' #1 correctness lesson.
- **Go concept:** `UNIQUE(user_id, book_id)` + `INSERT ... ON CONFLICT DO UPDATE`; aggregates
  via `UPDATE books SET ... FROM (SELECT AVG/COUNT ...)` in one transaction. The race detector
  catches in-memory shared-state bugs.
- **TS/JS contrast:** The **false sense of safety** from Node's single thread: developers
  assume single-threaded ⇒ race-free, but interleaved `await`s — and especially **multiple
  instances** racing the same row — produce the classic lost update. Single-threaded JS does
  not protect a read-modify-write across `await`s or across processes. There's no standard Node
  equivalent of `-race` for this.
- **Where TS/JS is competitive or better:** The **real** fix is in Postgres (transaction +
  `ON CONFLICT`), which is language-agnostic — a TS service using the same SQL is equally
  correct. `-race` only helps with in-memory shared state, which a stateless handler mostly
  avoids. Go just hands you `-race` for free.
- **Anchor:** `average_rating`/`rating_count` consistency under concurrent writes (§6.6).

> **Refleksiyon (TR):** "Tek thread = yarış yok" yanılgısını kendi kodunda kırabildin mi?
> Asıl çözümün dilde değil, Postgres transaction + ON CONFLICT'te olduğunu görmek ne hissettirdi?

---

## 11. Single static binary & container footprint

**Status:** `PARTIAL-MEASURED` (Faz 8 + Faz 12) · **Illustration:** `backend/Dockerfile` (multi-stage → `distroless/static`, uid 65532), `deploy/k8s/api.yaml`
>
> **🟡 Faz 12:** kind'da API pod'u `readOnlyRootFilesystem: true` ile sorunsuz çalıştı (binary diske hiçbir şey yazmıyor) ve 32 Mi bellek isteğiyle 2 kopya açıldı; Job + rollout ~30 sn. Soğuk başlangıç süresini ya da Node karşılığını ÖLÇMEDİK — iddia hâlâ PARTIAL.
>
> **🟡 Faz 8 — ölçüldü:** imaj **38 MB** (api + manage statik binary'leri 23.6 MB, `-s -w` ile; debug sembollü yerel binary 19.5 MB tek başına). Konteyner açılıştan `healthy`'ye ~5 sn (çoğu migration + Redis/DB bağlantısı; sürecin kendisi ms'ler). Shell ve paket yöneticisi yok, root değil. Dürüst sınır: aynı API'nin Node sürümünü inşa edip ÖLÇMEDİK — "node:22-alpine + node_modules genelde 150 MB+" bir genel bilgi, bu projenin ölçümü değil. Karşılaştırmalı iddia bu yüzden hâlâ PARTIAL.

- **Hypothesis:** A single statically-linked binary on `scratch`/`distroless` yields a much
  smaller image, faster cold start, and lower idle RSS than a Node image carrying
  `node_modules` + the runtime. Reviewers rate this the highest-impact infra section *because
  the numbers are concrete.*
- **Go concept:** Multi-stage Dockerfile → tiny final image, no runtime shipped.
- **TS/JS contrast:** Node image with `node_modules` + Node runtime; process warmup; higher
  baseline heap.
- **Where TS/JS is competitive or better:** Image size rarely changes the bill on free tiers
  (§13), and Node cold-start is fine for an always-on container. This matters most for
  scale-to-zero / many-pod scenarios BookWise won't hit.
- **MEASUREMENT REQUIRED:** record real `docker images` size, cold-start ms, and idle RSS
  (`/usr/bin/time`) for the Go image **and** an equivalent Node image. Until then this stays
  `UNVERIFIED-PERF` — do not generalize from blog numbers.

> **Refleksiyon (TR):** Ölçtüğün gerçek sayıları buraya yapıştır (Go image MB / Node image MB,
> cold-start ms, idle RSS). Fark beklediğin kadar mıydı, yoksa bu ölçekte önemsiz mi kaldı?

---

## 12. Performance & memory under load on read-hot endpoints

**Status:** `CONFIRMED-MEASURED` (Faz 6 + Faz 7) · **Illustration:** `backend/migrations/0005_listing_indexes.sql`, `backend/scripts/perf_seed.sql`, `backend/internal/ratings/cached.go`
>
> **🟡 Faz 6 — EXPLAIN ANALYZE (20 bin kitap, ~209 bin oy):** En büyük kazanımlar dilden değil SORGUDAN geldi: (1) public liste Faz 2'deki index'i hiç kullanmıyordu — `ORDER BY created_at DESC, id DESC`'in ikinci anahtarı index'te yoktu; `(created_at DESC, id DESC)` ile 6.5 ms → 0.06 ms. (2) Genel ortalama C her istekte 209 bin oyu tarıyordu (43 ms); kitap agregatlarından hesaplayınca 5.7 ms. (3) Top-rated sorgusunun kendisi (14 ms) index'le hızlanamaz — sıralama anahtarı global C'ye bağlı. Faz 7'de cache'lenecek olan tam da bu, ve karar ÖLÇÜLDÜ. Go vs Node farkı bu uçta ölçülemez: süre DB'de geçiyor.
>
> **✅ Faz 7 — yük testi (`ab -n 3000 -c 50`, top-rated, 20 bin kitap / 209 bin oy, lokal Docker):**
>
> | | istek/sn | p50 | p95 | p99 |
> |---|---|---|---|---|
> | cache yok | 327 | 138 ms | 236 ms | 332 ms |
> | Redis cache | 2892 | 16 ms | 27 ms | 40 ms |
>
> ~8.8×. Sıra doğruydu: önce EXPLAIN ANALYZE (index + agregat düzeltmeleri, Redis'siz), sonra ölçüm, sonra cache — yalnızca index'le hızlanamayan sorguya. Dil karşılaştırması açısından dürüst sonuç: kazançların hepsi mimari (index, cache); Go'ya özgü bir performans kazancı bu uçlarda ölçülmedi.

- **Hypothesis:** On read-hot paths Go's per-request allocation/GC profile is leaner than an
  equivalent Express handler — but for DB-bound endpoints the **database, not the language,**
  usually dominates.
- **Go concept:** `go test -bench/-benchmem` + a load tool (`hey`/`k6`) on the hot endpoints.
- **TS/JS contrast:** an equivalent Express/Fastify handler under the same load.
- **Where TS/JS is competitive or better — the evangelism trap:** for JSON-shaped, DB-bound
  CRUD (most of §10) the gap is marginal and often dwarfed by query + network time. **Any
  speed claim here must hold a real BookWise number or be deleted.** The honest conclusion will
  often be "indistinguishable at our scale."
- **Anchor:** `GET /api/books/top-rated` and `/popular` (§10.2), the weighted-rating sort
  (§6.7). **Tie measurement to the Phase 7 "why cache" decision:** a missing index (no index
  on `status`/`weighted_score`) must not be mistaken for a language problem.

> **Refleksiyon (TR):** Önce index'i doğru kur, sonra ölç. Yavaşlık dilden mi, eksik index'ten
> mi, sorgudan mı geldi? Redis'i "yavaş çünkü ölçtüm" diye mi ekledin, yoksa "havalı" diye mi?

---

## 13. Standard toolchain & DX: `go test` / `vet` / `-race` / `gofmt` / `build` vs the JS toolchain

**Status:** `CONFIRMED` (Faz 1 + Faz 6 bayrak testi) · **Illustration:** `backend/internal/ratings/score_test.go:TestWeightedScore` (bayrak tablo testi: brief örnekleri + kenarlar + özellik testleri), `backend/internal/config/config_test.go:TestLoad` (table-driven, 7 alt-test) + `.github/workflows/ci.yml` (CI — Faz 10'dan öne çekildi) + `.githooks/` (fmt/vet/lint/test hook'ları).
>
> **✅ Faz 1:** sıfır-config toolchain (`go test`/`vet`/`gofmt`, hepsi kutudan) + table-driven gerçekten rahat — JS'teki jest+eslint+prettier+tsc kurulum yükü yok. Dürüst karşı-nokta: JS'in mock/snapshot/watch ekosistemi daha zengin. (Bayrak weighted-score testi hâlâ Faz 6.)

- **Hypothesis:** One cohesive, zero-config toolchain (`go test`, `go vet`, `gofmt`,
  `go build`, `-race`) plus table-driven tests with the standard `testing` package is a real
  DX win for a solo learner over assembling jest/vitest + eslint + prettier + tsc + ts-node + a
  bundler.
- **Go concept:** Table-driven tests (`[]struct{name,in,want}` + `t.Run`), no third-party
  assertion framework; `gofmt` ends formatting debates; `go vet`/`golangci-lint` as a standard
  quality gate.
- **TS/JS contrast:** the assemble-it-yourself JS stack, each tool configured separately.
- **Where TS/JS is competitive or better:** Vitest/jest with rich mocking, snapshot testing,
  and watch-mode DX are genuinely more featureful than std `testing`, and the JS ecosystem's
  breadth is a real asset. Go's win is "works out of the box with zero decisions," which suits
  a solo learner but trades away ecosystem depth.
- **Anchor:** the weighted-rating **pure function** as the flagship table-driven test (§6.7);
  Phase 10 CI running `go vet` + `gofmt -l` + `go build` + `go test -race` so CI is a real gate.

> **Refleksiyon (TR):** "Sıfır config çalışıyor" rahatlığı, JS'in zengin mock/snapshot
> ekosistemini özlemene değdi mi? gofmt tartışmasız formatlama hayatını kolaylaştırdı mı?

---

## 14. Structured concurrency for background work & lifecycle

**Status:** `NOT-NEEDED` (Faz 7) · **Illustration:** yok — bilerek.
>
> **➖ Faz 7:** Cache invalidation arka plan goroutine'i GEREKMEDİ: yazım commit edilince anahtar senkron olarak siliniyor (tek `DEL`, ~0.1 ms) ve TTL güvenlik ağı. Arka plan işi bir kuyruk, yeniden deneme ve kapanışta boşaltma (graceful drain) gerektirirdi — gerçek bir problem çözmeden. Uydurmuyoruz; gerçek bir arka plan işi çıkarsa (ör. e-posta gönderimi) bu bölüm o zaman doldurulur.

- **Hypothesis:** `errgroup`, bounded worker pools, and graceful-shutdown context
  cancellation tie background work's lifetime to a context cleanly.
- **Go concept:** structured concurrency where background goroutines are bound to a `ctx`.
- **TS/JS contrast:** `setTimeout`/event-loop scheduling and queue libs like BullMQ.
- **Where TS/JS is competitive or better:** for **real** job needs (retries, scheduling,
  persistence, dashboards), Node's BullMQ ecosystem is far more turnkey than rolling Go worker
  pools. BookWise's job needs are tiny, so **neither is justified yet** — adding a queue now
  would violate §14.5. Keep this a placeholder until a real job exists.
- **Anchor:** cache invalidation on rating change (§6.9, Phase 7); email-verify / password-
  reset token flows (§6.9).

> **Refleksiyon (TR):** Gerçek bir arka plan işi çıkana kadar bu bölüm boş kalmalı. Boş
> kalması = §14.5'e sadık kaldığının kanıtı.

---

## 15. Standing section — Where TS/JS is genuinely the better choice

**Status:** `HYPOTHESIS` (grows with every Go section) · **Illustration:** `[Phase 9 — frontend/ ↔ backend/ DTO boundary — TBD]`, `[post-MVP — §12.8 AI — TBD]`

This is the deliberate counterweight that keeps the doc honest (§14.5: reason, not
popularity). **Rule: add at least one concrete entry here every time you fill a Go section.**

Seed entries:
1. **End-to-end type sharing.** Since §7.4 picks React/Next, TS could share request/response
   types front-to-back (zod/tRPC). Go cannot — your `frontend/` and `backend/` DTOs are
   maintained separately.
2. **AI/LLM ecosystem (§12.8).** The SDK/tooling ecosystem is far richer in TS/Python; if you
   build the optional AI features, TS/Python is the path of least resistance.
3. **I/O-bound JSON CRUD (most of §10).** Node's async model is fully adequate; Go's edge is
   marginal and often unmeasurable.
4. **Rapid prototyping / dynamic data shaping.** JS's flexibility wins for throwaway scripts
   and ad-hoc shapes.
5. **Optional-field / JSON-nullability ergonomics** and some generics niceties read cleaner in
   TS than Go's zero-value + pointer-for-nullable dance.

**Faz 1'de eklenen dürüst gözlemler:**
6. **Graceful shutdown (§8):** Express de `server.close()` + sinyal ile aynısını yapar. Go'nun
   `Shutdown(ctx)`'i biraz daha temiz ama bu bir *dil kazanımı değil*, ergonomi.
7. **Görünür HTTP / Chi (§4):** "şeffaflık" büyük ölçüde Chi-vs-Express (**library seçimi**);
   TS'te de ince bir router seçebilirsin. Dile puan yazma.
8. **I/O-bound `/health` (§3–§5):** bu basit JSON CRUD'da Node/Express **birebir yeterdi**;
   Go'nun hız avantajı bu ölçekte ölçülemez.
9. **Test DX (§13):** Go'nun sıfır-config'i rahat, AMA jest/vitest'in mock/snapshot/watch DX'i
   ve JS ekosisteminin genişliği gerçek bir avantaj.
10. **Hata zarfı (§5):** tutarlı `{"error":{"code","message"}}` JS'te de birebir kurulur — dil
    farkı değil, **disiplin**.

**Faz 2'de eklenen dürüst gözlemler:**
11. **Doğrulama ergonomisi (§5):** zod'da tek bir şema hem TS tipini hem doğrulamayı üretir. Go'da
    `createBookRequest` struct'ı, `normalize()` ve `validate()` üç ayrı yerde yaşıyor ve senkron
    tutulmaları elle. Bu noktada TS açıkça daha iyi.
12. **Mass-assignment (§5):** açığı yaratan şey dil değil, "gövdeyi DB modeline decode et"
    kısayoluydu — Express'te `Object.assign(book, req.body)` aynı açıktır, NestJS `whitelist:true`
    ile aynı çözümü sunar. Go burada bir şey kazandırmadı; disiplin kazandırdı.
13. **Opsiyonel filtreler (§6):** `(narg IS NULL OR ...)` deseni çalışıyor ama okunaksız; Knex/
    Prisma'da `where` nesnesini koşullu kurmak çok daha doğal. sqlc'nin bilinen zayıflığı.
14. **Sahte (fake) bakımı (§3):** interface büyüdükçe her sahte tüm metotları uygulamak zorunda.
    jest'in `jest.fn()` ile tek metodu taklit etmesi bu noktada daha az törenli.

**Faz 3'te eklenen dürüst gözlemler:**
15. **JWT algoritma sabitleme (§2/§5):** Node'un `jsonwebtoken`'ı da `verify(token, secret,
    { algorithms: ['HS256'] })` ile aynı korumayı sunar. Açık kütüphane seçeneğinde; Go burada
    bir şey kazandırmadı — "seçeneği bil ve testle" dersi dil-bağımsız.
16. **bcrypt 72 bayt kırpması:** Node `bcrypt` paketi de aynı davranır (algoritma özelliği). Bulan
    şey dil değil, "kenar davranışı teste sor" alışkanlığıydı.
17. **Sahte store maliyeti (§3):** `memStore` ~110 satır (8 metot, mutex, map'ler). jest'te
    `jest.fn().mockResolvedValue(...)` ile yalnızca test edilen metodu taklit etmek çok daha az
    törenli. Karşılığında sahte, gerçek sözleşmeyi (tekil e-posta, tek kullanımlık token) taklit
    ediyor — ama bu disiplin, dil zorunluluğu değil.

**Faz 4'te eklenen dürüst gözlemler:**
18. **Durum makinesi (§1):** TS'te `type Status = 'PENDING' | ...` + exhaustive `switch` +
    `never` ile yeni bir statü eklendiğinde derleyici işlenmeyen her yeri gösterir. Go'nun sahte
    enum'u bunu yapamıyor; biz aynı güvenceyi bir tablo testiyle (4×4) sağlıyoruz. TS kazanır.
19. **Admin/public DTO (§5):** iki ayrı DTO'nun (ve gömülü struct'ın) faydası dil-bağımsız bir
    disiplin; NestJS'te `@Exclude()`/serializer grupları aynısını yapar.

**Faz 5'te eklenen dürüst gözlemler:**
20. **Kayıp güncelleme (§10):** Node'da aynı hata aynı biçimde olur (iki istek, iki transaction);
    çözüm de aynı SQL. Go'nun goroutine'leri testi YAZMAYI kolaylaştırdı (25 eşzamanlı istemci,
    `sync.WaitGroup` + başlangıç kanalı), ama hatayı bulan şey dil değil, test tasarımıydı.

**Faz 6'da eklenen dürüst gözlemler:**
21. **Performans (§12):** ölçülen üç iyileştirmenin üçü de SQL'di (index, agregat, sıralama).
    Aynı API Node'da aynı sorgularla aynı sürede cevap verirdi; darboğaz dil değil DB.
22. **Tablo testi (§13):** `it.each` ile Jest'te aynı test aynı kısalıkta yazılır. Go'nun avantajı
    yalnızca sıfır bağımlılık — ifade gücü değil.

**Faz 7'de eklenen dürüst gözlemler:**
23. **Cache/rate limit (§12):** `ioredis` + `express-rate-limit` + `rate-limit-redis` ile Node'da
    aynı mimari birkaç satırda kurulur ve olgun paketler hazır. Go'da sınırlayıcıyı kendimiz
    yazdık (öğretici ama daha çok kod). Bu uçta ekosistem hazırlığında TS önde.

**Faz 8'de eklenen dürüst gözlemler:**
24. **Konteyner (§11):** Node de `node:alpine` + multi-stage + `npm ci --omit=dev` ile makul
    küçülür; Bun/Deno tek dosya derleme de sunar. Go'nun avantajı "varsayılan olarak statik
    tek binary" — ekstra araç gerekmiyor. Fark gerçek ama "Node imajı şişkindir" kaderi değil.

> **Refleksiyon (TR):** Buraya her Go bölümü doldurduğunda en az bir dürüst "TS/JS kazanırdı"
> girdisi ekle. Bu bölüm büyümüyorsa, doküman Go reklamına kaymış demektir.

---

## Living-document mechanism

**References are added per phase, never up front.** Phase 0 creates the doc with every
section at `HYPOTHESIS`. You earn each reference as the code lands.

### Per-phase fill-in checklist

At the **end of every roadmap phase** (§11), before calling the phase done:

1. **Which sections did this phase produce real code for?** Map:
   - **Phase 1** → §1, §3, §4, §5, §8, §13
   - **Phase 2** → §1, §6, §7
   - **Phase 3** → §2, §3 (auth service)
   - **Phase 4** → §1 (status transitions), §2
   - **Phase 5** → §2, §10
   - **Phase 6** → §1, §9, §13
   - **Phase 7** → §7, §12, §14
   - **Phase 8 / 12** → §11
   - **Phase 9** → §15
   - **Phase 10** → §13
2. **Replace each `[… — TBD]`** with a real reference — prefer a stable
   `path:Symbol` (e.g. `backend/internal/ratings/score.go:WeightedScore`) over a bare line
   number, so it survives refactors — plus a 3–5 line **Türkçe** reflection: did the code
   **confirm** or **falsify** the hypothesis? Update the `Status` tag accordingly.
3. **Add ≥1 entry to §15** (Where TS/JS wins). The doc may not grow a Go section without
   growing its counterweight.
4. **Perf sections (§11, §12):** paste a real measured number or leave `UNVERIFIED-PERF`. No
   blog numbers.

This maps onto §16's per-phase "explain what changed and why" habit — the reflection **is**
that explanation, persisted. Each placeholder→reference edit is its own small commit, so the
git history shows *when and why* each comparison was earned.

### How staleness is prevented

- **Stable anchors:** cite `path:Symbol`, not bare line numbers. Keep the weighted-rating math
  a pure, stable function partly so its citation stays valid.
- **CI guard (Phase 10):** a small script in the GitHub Actions pipeline greps this file for
  every cited path and fails the build if a referenced file no longer exists.
- **Status tags:** a section may never be silently "true" — it always carries its evidence
  state (`HYPOTHESIS` / `CONFIRMED` / `FALSIFIED` / `PARTIAL` / `UNVERIFIED-PERF`).
- **Stop condition:** the **core** is the sections fillable by the MVP (Phases 1–7 = §15
  Definition of Done). Infra sections (§11, §14) may legitimately stay `HYPOTHESIS` if you stop
  at MVP — an unfilled infra section is **success-with-early-stop, not debt.**

---

## Companion docs

- [GO_CONVENTIONS.md](GO_CONVENTIONS.md) — _how_ we write Go (the idioms §1–§8, §13 cite).
- [PROJECT_BRIEF.md](PROJECT_BRIEF.md) — the product/learning brief and phase roadmap.
