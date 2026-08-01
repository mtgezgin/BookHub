# BookHub — Durum & Devir (STATUS)

> **Yeni bir konuşma bunu İLK okusun.** Nerede olduğumuzu, kararları ve sıradaki adımı özetler.
> Her oturum sonunda güncellenir. (Kişisel öğrenme notları/memory ayrıca otomatik yüklenir.)

**Son güncelleme bağlamı:** Faz 2 adım 6/8 bitti — dikey dilim canlı doğrulandı; adım 7 (liste+create) kullanıcı egzersizi olarak YARIM.
**Repo:** github.com/mtgezgin/BookHub (private) · Adım 4–6 (sqlc + pgx havuzu + dikey dilim) **commit'lendi**; push bekliyor. Adım 7 (liste+create) hâlâ yarım — kullanıcı egzersizi.

---

## Neredeyiz — Faz 1 ✅ TAMAM
Çalışan, test edilmiş, CI'lı bir Go API iskeleti kuruldu ve push'landı:
- **Kod** (`backend/`): `cmd/api/{main.go, router.go, respond.go}` + testleri; `internal/config/{config.go, config_test.go}`.
  - `/health` endpoint, env'den config + boot'ta fail-fast doğrulama, graceful shutdown + timeouts, Chi router + middleware (RequestID/Logger/Recoverer), tutarlı JSON hata zarfı `{"error":{"code","message"}}`.
  - Testler: `httptest` + table-driven, `go test -race` yeşil.
- **Bağımlılık:** `go-chi/chi/v5 v5.3.0` (ilk ve tek dış paket).
- **CI** (`.github/workflows/ci.yml`): gofmt + vet + build + test-race + **golangci-lint** — **tamamen yeşil**.
- **Katmanlı git hook'ları** (`.githooks/`): pre-commit (gofmt+vet), pre-push (golangci-lint+test-race). `core.hooksPath=.githooks` **yerel git config**'tir → taze klonda bir kez `git config core.hooksPath .githooks` gerekir.

## Faz 2 — devam ediyor 🔄 (adım 6/8 bitti) → [phases/phase-02](phases/phase-02-postgres-books-genres.md)
- ✅ **1. Docker Postgres:** `docker-compose.yml` (postgres:16, named volume `pg_data`, healthcheck, `5432:5432`). `docker compose up -d` ile ayakta, doğrulandı.
- ✅ **2. `DATABASE_URL` config:** zorunlu alan (default yok → fail-fast) — kullanıcının ilk "örnekten uyarla" egzersisiydi; testler 8 senaryoya çıktı. `.env.example` eklendi; lokalde `.env` kopyalandı.
- ✅ **3. goose + ilk migration:** goose v3.27.2 (`go install`, `~/go/bin`). `backend/migrations/0001_books_genres.sql` = `books`/`genres`/`book_genres` (§18.3: identity PK, TEXT+CHECK status **DEFAULT'suz**, timestamptz, slug UNIQUE, partial index `(created_at DESC) WHERE status='APPROVED'`, `book_genres(genre_id)` index'i). **avg_rating/rating_count bilerek yok → Faz 5 migration'ı ekleyecek** (şema evrimi dersi). up/down/up roundtrip doğrulandı; dev DB'de seed: books id=1 "Clean Code".
- ✅ **4. sqlc:** v1.31.1 (`go install`) + `sqlc.yaml` (açıklamalı; pgx/v5, timestamptz→time.Time, nullable int→*int32 override'ları) + `sql/queries/books.sql`: `GetBook` (asistan, örnek) + `ListBooks`/`CreateBook` (**kullanıcı yazdı**, ilk denemede doğru). Üretilen kod `internal/db/` (commit'lenir). pgx/v5 v5.10.0 go.mod'da.
- ✅ **5. pgx pool:** `internal/database/database.go` — pgxpool + fail-fast Ping (5sn timeout). main.go'da elle DI: pool → repository → router.
- ✅ **6. Dikey dilim `GET /api/books/{id}`:** `internal/books/repository.go` (ErrNoRows→ErrNotFound çevirisi, DBTX ile tx-hazır) + `cmd/api/books.go` (tüketici-tarafı `bookGetter` interface, closure DI, 400/404/500 haritalama) + fake'li table-driven handler testi. **Canlı doğrulandı** (5 Tem): seed id=1 → 200, 999 → 404, abc → 400.
- 🔶 **7. Liste + create — kullanıcı egzersizi YARIM:**
  - ✅ `Repository.List`/`Create` yazıldı (review'daki hata-yolunda-`nil` düzeltmesi uygulandı).
  - ⏳ Kalan: `List`/`Create` doc comment + `gofmt -w internal/books/repository.go`; `handleListBooks` (**nil slice → `[]` normalize et — JSON null tuzağı!**) + `handleCreateBook` (ayrı request struct — istemciden `status` ALINMAZ, handler `APPROVED` yazar; decode şablonu konuşma geçmişinde/aşağıda); router'a `GET /api/books` + `POST /api/admin/books`; `fakeBooks`'a `List`/`Create` (interface 3 metoda çıkınca derlenmesi için şart) + liste testi.
  - Stretch: slug UNIQUE ihlali (pgconn.PgError 23505) → domain hatası → **409** (şu an 500 döner).
  - Sonra: genres CRUD + book_genres m2m.
- ⏭️ **8. Repository integration testi** (compose Postgres'e karşı, mock'suz) → sonra milestone commit + push.

## Ortam / gerçekler
- **Go 1.26.4** (Homebrew ile yönetiliyor). Module: `github.com/mtgezgin/bookhub/backend`.
- **goose v3.27.2** ve **sqlc v1.31.1** CLI'ları `~/go/bin`'de (taze makinede `go install` ile). pgAdmin 4 kurulu (dev DB'ye `localhost:5432`, bookhub/bookhub_dev).
- **Compose portu artık `127.0.0.1:5432:5432`** (LAN'a kapalı — güvenlik sohbetinin çıktısı; commit'siz değişikliklerde). Oturum başında `docker compose up -d` gerekebilir (yeniden başlatmada container kalkmaz, veri named volume'da durur).
- Dev DB'de seed: books id=1 "Clean Code" (dikey dilimin curl hedefi).
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

## Sırada: adım 7'yi bitir (kullanıcı egzersizi — detay yukarıda 🔶)
1. Repository kozmetiği (doc comment ×2 + gofmt) → 2. iki handler + iki rota → 3. fakeBooks + liste testi → 4. canlı curl doğrulaması → 5. genres/m2m → 6. adım 8 integration testi → **milestone commit + push.**

`handleCreateBook` için hazır decode şablonu (istemciden `status` alınmaz — mass-assignment):
```go
var req createBookRequest // ayrı struct: Title/Slug/Author/... (*int32'ler pointer)
if err := json.NewDecoder(r.Body).Decode(&req); err != nil { /* 400 invalid_json */ }
// zorunlu alanlar (title/slug/author) boşsa 400 → sonra elle eşle:
// db.CreateBookParams{ ..., Status: "APPROVED" } → 201 + writeJSON
```

## Doküman haritası
- [PROJECT_MAP.md](PROJECT_MAP.md) — **"neyin ne olduğu" haritası**: dosya dosya ne işe yarar, istek yolculuğu, araç zinciri, kavramlar, bakım borcu. (Bu dosya "neredeyiz", o "ne var".) Uzun aradan sonra ikisini birlikte oku.
- [PROJECT_BRIEF.md](PROJECT_BRIEF.md) — ürün + roadmap (§11) + §18 addendum (kararlar).
- [phases/](phases/) — faz faz adım adım rehber (README = merdiven).
- [GO_CONVENTIONS.md](GO_CONVENTIONS.md) — Go'yu nasıl yazıyoruz + güvenlik temeli.
- [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) — Go vs TS/JS (Faz 1 dolduruldu).
- [learning-notes/](learning-notes/) — sorulan kavramların kalıcı notları (`/not` skill'i ekler).
