# BookHub — Durum & Devir (STATUS)

> **Yeni bir konuşma bunu İLK okusun.** Nerede olduğumuzu, kararları ve sıradaki adımı özetler.
> Her oturum sonunda güncellenir. (Kişisel öğrenme notları/memory ayrıca otomatik yüklenir.)

**Son güncelleme bağlamı:** Faz 1 tamamlandı; Faz 2'ye (Postgres) başlamak üzereyiz.
**Repo:** github.com/mtgezgin/BookHub (private) · **Son push'lu commit:** `cecf2b4` (docs: Phase 1 wrap).

---

## Neredeyiz — Faz 1 ✅ TAMAM
Çalışan, test edilmiş, CI'lı bir Go API iskeleti kuruldu ve push'landı:
- **Kod** (`backend/`): `cmd/api/{main.go, router.go, respond.go}` + testleri; `internal/config/{config.go, config_test.go}`.
  - `/health` endpoint, env'den config + boot'ta fail-fast doğrulama, graceful shutdown + timeouts, Chi router + middleware (RequestID/Logger/Recoverer), tutarlı JSON hata zarfı `{"error":{"code","message"}}`.
  - Testler: `httptest` + table-driven, `go test -race` yeşil.
- **Bağımlılık:** `go-chi/chi/v5 v5.3.0` (ilk ve tek dış paket).
- **CI** (`.github/workflows/ci.yml`): gofmt + vet + build + test-race + **golangci-lint** — **tamamen yeşil**.
- **Katmanlı git hook'ları** (`.githooks/`): pre-commit (gofmt+vet), pre-push (golangci-lint+test-race). `core.hooksPath=.githooks` **yerel git config**'tir → taze klonda bir kez `git config core.hooksPath .githooks` gerekir.

## Ortam / gerçekler
- **Go 1.26.4** (Homebrew ile yönetiliyor). Module: `github.com/mtgezgin/bookhub/backend`.
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

## Açık uçlar (non-blocking — sıradaki commit'e yedirilecek)
- **[F1]** `ci.yml`'deki golangci-lint sürüm yorumu "lokalle eşleşir" diyor ama lokal **2.12.1** vs CI **2.12.2**; ayrıca pre-push hook sürüm pinlemiyor. → yorumu düzelt ("reproducibility için pinlendi") ya da lokali 2.12.2'ye bump'la. **Faz 2'nin ilk commit'ine yedir.**
- GitHub Actions **Node 20 deprecation** uyarısı (checkout@v4, setup-go@v5, golangci-lint-action@v7) → ileride action sürümlerini bump'la (acil değil).

## Sırada: Faz 2 — Postgres → [phases/phase-02-postgres-books-genres.md](phases/phase-02-postgres-books-genres.md)
İlk adımlar (çoğu **yeni pattern** → ben yazıp açıklarım):
1. `docker-compose.yml` → dev Postgres (Docker'ın "ne/neden"i ile).
2. Config'e `DatabaseURL` alanı ekle → **ilk "örnekten uyarla": `config.go`'daki `Port`'a bakarak kullanıcı ekler.**
3. İlk goose migration (`books`, `genres`, `book_genres` — tip/kısıt/index'ler §18.3).
4. sqlc kurulumu + ilk sorgu.
5. **İlk dikey dilim:** tek `Book` uçtan uca (migration→repo→handler→JSON→test).

## Doküman haritası
- [PROJECT_BRIEF.md](PROJECT_BRIEF.md) — ürün + roadmap (§11) + §18 addendum (kararlar).
- [phases/](phases/) — faz faz adım adım rehber (README = merdiven).
- [GO_CONVENTIONS.md](GO_CONVENTIONS.md) — Go'yu nasıl yazıyoruz + güvenlik temeli.
- [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) — Go vs TS/JS (Faz 1 dolduruldu).
- [learning-notes/](learning-notes/) — sorulan kavramların kalıcı notları (`/not` skill'i ekler).
