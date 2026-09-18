# BookHub — Durum & Devir (STATUS)

> **Yeni bir konuşma bunu İLK okusun.** Nerede olduğumuzu, kararları ve sıradaki adımı özetler.
> Her faz sonunda güncellenir. "Neyin ne olduğu" için: [PROJECT_MAP.md](PROJECT_MAP.md).

**Son güncelleme:** Faz 2 ✅ TAMAM. Sırada **Faz 3 — Auth + roller**.
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
| 3 | Auth + roller (JWT, bcrypt, refresh token, admin guard) | ⏭️ sırada |
| 4 | Öneri + moderasyon (state machine, audit log) | ⏳ |
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

## Sıradaki: Faz 3 — Auth + roller (plan)

1. Migration `0002`: `users` (email UNIQUE — küçük harfe normalize, `password_hash`, `role` TEXT+CHECK
   USER/ADMIN), `refresh_tokens` (token **hash**'i, `expires_at`, kullanıcıya FK, CASCADE).
2. `internal/auth`: bcrypt (cost 12) · JWT HS256 (`alg` açıkça doğrulanır; `exp/iat/iss/aud/sub/role`,
   access ~15 dk) · opak refresh token (`crypto/rand`, DB'de SHA-256 hash, kullanımda **rotate**).
3. Config: `JWT_SECRET` (zorunlu, ≥32 bayt), `JWT_ACCESS_TTL`, `JWT_REFRESH_TTL`.
4. Uçlar: `POST /api/auth/register|login|refresh|logout`, `GET /api/auth/me`. Login hatası generic.
5. Middleware: `requireAuth` (401), `requireAdmin` (403) → `/api/admin/*` korunur.
6. İlk admin: `cmd/manage create-admin` komutu (parola env'den/stdin'den; koda gömülmez).
7. İlk **Service** katmanı: `auth.Service` (register/login/refresh iş kuralları).

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
