# BookHub — Durum & Devir (STATUS)

> **Yeni bir konuşma bunu İLK okusun.** Nerede olduğumuzu, kararları ve sıradaki adımı özetler.
> Her oturum sonunda güncellenir. (Kişisel öğrenme notları/memory ayrıca otomatik yüklenir.)

**Son güncelleme bağlamı:** Faz 2 devam ediyor — adım 1–3 tamam (dev Postgres + `DATABASE_URL` config + ilk migration); sırada adım 4 (sqlc).
**Repo:** github.com/mtgezgin/BookHub (private) · **Son push'lu commit:** `cecf2b4` · Faz 2 milestone commit'i lokalde (push bekliyor).

---

## Neredeyiz — Faz 1 ✅ TAMAM
Çalışan, test edilmiş, CI'lı bir Go API iskeleti kuruldu ve push'landı:
- **Kod** (`backend/`): `cmd/api/{main.go, router.go, respond.go}` + testleri; `internal/config/{config.go, config_test.go}`.
  - `/health` endpoint, env'den config + boot'ta fail-fast doğrulama, graceful shutdown + timeouts, Chi router + middleware (RequestID/Logger/Recoverer), tutarlı JSON hata zarfı `{"error":{"code","message"}}`.
  - Testler: `httptest` + table-driven, `go test -race` yeşil.
- **Bağımlılık:** `go-chi/chi/v5 v5.3.0` (ilk ve tek dış paket).
- **CI** (`.github/workflows/ci.yml`): gofmt + vet + build + test-race + **golangci-lint** — **tamamen yeşil**.
- **Katmanlı git hook'ları** (`.githooks/`): pre-commit (gofmt+vet), pre-push (golangci-lint+test-race). `core.hooksPath=.githooks` **yerel git config**'tir → taze klonda bir kez `git config core.hooksPath .githooks` gerekir.

## Faz 2 — devam ediyor 🔄 (adım 3/8 bitti) → [phases/phase-02](phases/phase-02-postgres-books-genres.md)
- ✅ **1. Docker Postgres:** `docker-compose.yml` (postgres:16, named volume `pg_data`, healthcheck, `5432:5432`). `docker compose up -d` ile ayakta, doğrulandı.
- ✅ **2. `DATABASE_URL` config:** zorunlu alan (default yok → fail-fast) — kullanıcının ilk "örnekten uyarla" egzersisiydi; testler 8 senaryoya çıktı. `.env.example` eklendi; lokalde `.env` kopyalandı.
- ✅ **3. goose + ilk migration:** goose v3.27.2 (`go install`, `~/go/bin`). `backend/migrations/0001_books_genres.sql` = `books`/`genres`/`book_genres` (§18.3: identity PK, TEXT+CHECK status **DEFAULT'suz**, timestamptz, slug UNIQUE, partial index `(created_at DESC) WHERE status='APPROVED'`, `book_genres(genre_id)` index'i). **avg_rating/rating_count bilerek yok → Faz 5 migration'ı ekleyecek** (şema evrimi dersi). up/down/up roundtrip doğrulandı; dev DB'de seed: books id=1 "Clean Code".
- ⏭️ **4. sqlc kurulumu + ilk sorgular** (`sqlc.yaml`, `sql/queries/books.sql`) — **ilk sorguları kullanıcı yazacak.**
- Sonra: **5.** pgx pool (`internal/database`, elle DI) → **6.** dikey dilim `GET /api/books/{id}` → **7.** liste/create/genres/m2m → **8.** repository integration testi.

## Ortam / gerçekler
- **Go 1.26.4** (Homebrew ile yönetiliyor). Module: `github.com/mtgezgin/bookhub/backend`.
- **goose v3.27.2** CLI `~/go/bin`'de (taze makinede `go install github.com/pressly/goose/v3/cmd/goose@latest`). pgAdmin 4 kurulu (dev DB'ye `localhost:5432`, bookhub/bookhub_dev).
- **Commit kimliği:** bu repoda **yerel** `user.email=mehmetturgutgezgin@gmail.com` (global iş kimliği değişmedi).
- Bu repo'da **pre-commit gate'leri** (güvenlik + coverage + kod review) ve **pre-push fan-out panel** çalışıyor → kaynak kod commit/push'ları ağır. **Commit'leri milestone'da topla.**

## Kilit kararlar (yeniden tartışma — [PROJECT_BRIEF.md](PROJECT_BRIEF.md) §7 & §18)
- **DB/Redis:** dev'de **lokal Docker** · prod'da **cloud** (Supabase Postgres + Upstash Redis); `DATABASE_URL`/`REDIS_URL` ile geçiş; **dev ≠ prod DB**.
- **sqlc + pgx** (ORM değil — GORM/bun dışlandı).
- **§14.5:** sebepsiz teknoloji ekleme. **MVP = Faz 1–7.** Faz 8–12 opsiyonel altyapı.
- Rating consistency: `UNIQUE(user_id,book_id)` + `ON CONFLICT`, tx-içi agregat; `weighted_score` query-time.

## Çalışma modu (memory'de de var — [bookhub-working-mode])
- **Hedef:** production-ready **demo ürün** + tool'ları *neden/ne zaman* anlamak. Go **syntax ustalığı değil**.
- **Hedef beceri:** oku + anla + gözden geçir + karar ver.
- **Kod yazımı (seçici):** boilerplate'i **ben** yazarım; anlamlı ~%20'yi (bir sorgu, fonksiyon gövdesi) **kullanıcı** yazar/uyarlar.
- **Örnekten uyarla:** daha önce benzerini yaptıysak → "şu örneğe (Y) bakarak sen yaz (X)". İlk kez → ben yazar+açıklarım.
- Derin, başlangıç-seviyesi açıklamalar; her adımda kadran kullanıcıda.

## Açık uçlar (non-blocking)
- ~~[F1] golangci-lint sürüm yorumu~~ ✅ kapandı (yorum "reproducibility için pinlendi; otorite CI" olarak düzeltildi).
- GitHub Actions **Node 20 deprecation** uyarısı (checkout@v4, setup-go@v5, golangci-lint-action@v7) → ileride action sürümlerini bump'la (acil değil).

## Sırada: Faz 2 adım 4 — sqlc
- `sqlc.yaml` (backend kökünde) + `sql/queries/books.sql` — **iki SQL klasörünü karıştırma:** `sql/queries/` = sqlc girdisi, `migrations/` = goose (§18.4).
- Kurulum + yaml boilerplate'i asistan yazar; **`GetBook`/`ListBooks`/`CreateBook` sorgularını kullanıcı yazar** (SQL öğrenimi bu fazın asıl dersi).
- Sonra pgx pool → dikey dilim `GET /api/books/{id}` (seed "Clean Code" ile curl checkpoint'i).

## Doküman haritası
- [PROJECT_BRIEF.md](PROJECT_BRIEF.md) — ürün + roadmap (§11) + §18 addendum (kararlar).
- [phases/](phases/) — faz faz adım adım rehber (README = merdiven).
- [GO_CONVENTIONS.md](GO_CONVENTIONS.md) — Go'yu nasıl yazıyoruz + güvenlik temeli.
- [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) — Go vs TS/JS (Faz 1 dolduruldu).
- [learning-notes/](learning-notes/) — sorulan kavramların kalıcı notları (`/not` skill'i ekler).
