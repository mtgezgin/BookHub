# BookHub — Durum & Devir (STATUS)

> **Yeni bir konuşma bunu İLK okusun.** Nerede olduğumuzu, kararları ve sıradaki adımı özetler.
> Her faz sonunda güncellenir. "Neyin ne olduğu" için: [PROJECT_MAP.md](PROJECT_MAP.md).

**Son güncelleme:** Faz 3 ✅ TAMAM. Sırada **Faz 4 — Öneri + moderasyon**.
**Hedef (kullanıcı talebi, 2026-09):** projeyi baştan sona bitir — MVP (Faz 1–7) + opsiyonel
altyapı rayı (Faz 8–12) — **her adım ayrı commit**, faz sonlarında push.

> **Ortam notu:** Kabukta `go` bulunamazsa `export PATH="/opt/homebrew/bin:$HOME/go/bin:$PATH"`
> — git hook'ları da `go: command not found` ile bu yüzden patlıyor.

---

## Faz durumu

| Faz | Konu | Durum |
|---|---|---|
| 0 | Repo + git | ✅ |
| 1 | Minimal Go API (/health, config, graceful shutdown, CI, hook'lar) | ✅ |
| 2 | Postgres + kitap/tür CRUD + many-to-many | ✅ |
| 3 | Auth + roller (JWT, bcrypt, refresh token, admin guard) | ✅ |
| 4 | Öneri + moderasyon (state machine, audit log) | ⏭️ sırada |
| 5 | Yorum + puan (ON CONFLICT, tx-içi agregat, eşzamanlılık) | ⏳ |
| 6 | Ağırlıklı puan (Bayesian, top-rated) | ⏳ |
| 7 | Redis cache + rate limit + yük testi — **MVP sonu** | ⏳ |
| 8–12 | Docker imajı · Frontend · CI/CD · NGINX · Kubernetes | ⏳ |

## Faz 2'de ne oldu (özet)

- **Uçlar:** `GET /api/books` (sayfalı: `?genre=&q=&limit=&offset=` → `{items,total,limit,offset}`),
  `GET /api/books/{id}` (türleriyle), `GET /api/genres`; admin: `POST /api/admin/books`,
  `PUT /api/admin/books/{id}/genres`, `POST/PATCH/DELETE /api/admin/genres`.
- **Güvenlik dersleri (canlı):** BOLA/IDOR (görünürlük sınırı SQL'de: `GetApprovedBook`),
  mass-assignment (ayrı request DTO + `DisallowUnknownFields`), hata detayı sızıntısı
  (`readJSON` artık tipli `*requestError`; ham hata asla istemciye gitmez).
- **Doğruluk dersleri:** N+1'den kaçınma (toplu `ANY($1)` + çağrı sayan test), `WithTx` +
  `SELECT ... FOR UPDATE` (kilitsiz eşzamanlı değiştirme iki listenin birleşimini bırakıyordu —
  mutation ile kanıtlandı).
- **Altyapı:** migration'lar binary'ye gömülü (`migrations/embed.go`), `AUTO_MIGRATE=true` ile
  açılışta uygulanır; entegrasyon testleri `internal/testdb` ile kendiliğinden migrate eder.
- **Commit'ler:** `d6a9245` kitap uçları · `0f4da49` gömülü migration · `ee1ae59` test sertleştirme ·
  `159f8c8` türler · `7dd71d7` many-to-many · `2a31a50` sayfalama/arama · (docs kapanışı).

## Faz 3'te ne oldu (özet)

- **Uçlar:** `POST /api/auth/register|login|refresh|logout`, `GET /api/auth/me`.
  `/api/admin/*` artık `requireAuth` (401) + `requireAdmin` (403) arkasında.
- **İlk Service katmanı:** `internal/auth` (password, token, repository, service).
- **İlk admin:** `set -a; source .env; set +a; go -C backend run ./cmd/manage create-admin -email SEN@ornek.com`
  (parola gizli sorulur). Kayıt ucu her zaman USER açar.
- **Bulunan açıklar:** admin grubu kimliksizdi (push kapısı yakaladı — Faz 2 push'u bu yüzden
  Faz 3'le birlikte yapılacak); bcrypt 72 bayt kırpması girişi açıyordu (deneyerek bulundu).
- **Commit'ler:** `a4c9f80` şema · `ecd9c8c` bcrypt+JWT · `6def864` servis · `9c19fd6` uçlar ·
  `3820ae2` admin koruması · `9d9226c` create-admin · (docs).

## Sıradaki: Faz 4 — Öneri + moderasyon (plan)

1. Migration `0003`: `books.status`'a `ARCHIVED`; `submitted_by` (FK users), `rejection_reason`,
   `moderated_by`/`moderated_at`; `audit_logs` (append-only: actor, action, entity, jsonb detay).
2. `POST /api/books/suggestions` (USER) → `PENDING` kitap, türleriyle.
3. `internal/moderation` servisi: geçiş kuralları (PENDING→APPROVED/REJECTED, APPROVED→ARCHIVED…),
   `FOR UPDATE` + audit kaydı **aynı transaction'da**.
4. Admin: `GET /api/admin/books?status=PENDING`, `POST .../{id}/approve|reject`, `PATCH`/`DELETE`
   (arşivle) — admin DTO'su statü/moderasyon alanlarını gösterir; public DTO göstermez.

## Ortam / gerçekler

- **Go toolchain 1.27.0** (Homebrew) · `go.mod` dil sürümü 1.26.4 · module `github.com/mtgezgin/bookhub/backend`.
- CLI'lar `~/go/bin`: goose v3.27.2, sqlc v1.31.1, golangci-lint v2.13 (CI v2.12.2 pinli).
- Docker Desktop + `docker compose up -d` → Postgres 16 (`127.0.0.1:5432`, LAN'a kapalı). Veri named volume'da.
- `.env` = `.env.example` kopyası (dotenv YOK: `set -a; source .env; set +a` ile yükle).
- Testler: `go test -race ./...` (DB'siz, CI gibi) · `TEST_DATABASE_URL="$DATABASE_URL" go test -race ./...` (DB'li).
- Commit kimliği: repo-yerel `user.email=mehmetturgutgezgin@gmail.com`.
- **Git kapıları:** pre-commit hook yalnız gofmt+vet (golangci-lint'i ELLE çalıştır). Push'ta plugin
  kapıları: güvenlik incelemesi + çok ajanlı review paneli + test coverage (SHA'ya bağlı; her yeni
  commit onları sıfırlar → push'u faz sonunda topla).

## Kilit kararlar (yeniden tartışma — [PROJECT_BRIEF.md](PROJECT_BRIEF.md) §7 & §18)

- dev = lokal Docker, prod = Supabase/Upstash; geçiş yalnızca `DATABASE_URL`/`REDIS_URL` ile.
- sqlc + pgx, ORM yok. Identifier'lar İngilizce, yorum/mesajlar Türkçe (GO_CONVENTIONS §9).
- Ratings: `UNIQUE(user_id,book_id)` + `ON CONFLICT`, tx-içi agregat; `weighted_score` query-time.
- Service katmanı yalnızca gerçek iş mantığı olan yerde (auth, moderasyon, puan).

## Doküman haritası

- [PROJECT_MAP.md](PROJECT_MAP.md) — "neyin ne olduğu" (dosya dosya, istek yolculuğu, araçlar).
- [PROJECT_BRIEF.md](PROJECT_BRIEF.md) — ürün + roadmap (§11) + kararlar (§18).
- [phases/](phases/) — faz rehberleri · [GO_CONVENTIONS.md](GO_CONVENTIONS.md) · [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md)
- [learning-notes/](learning-notes/) — sorulan kavramların kalıcı notları (`/not` skill'i ekler).
