# BookHub — Durum & Devir (STATUS)

> **Yeni bir konuşma bunu İLK okusun.** Nerede olduğumuzu, kararları ve sıradaki adımı özetler.
> Her faz sonunda güncellenir. "Neyin ne olduğu" için: [PROJECT_MAP.md](PROJECT_MAP.md).

**Son güncelleme:** Faz 7 ✅ — **MVP TAMAM** 🎉 (brief §15, `backend/scripts/mvp_smoke.sh` 21/21). Sırada opsiyonel altyapı rayı: **Faz 8 — Docker**.
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
| 4 | Öneri + moderasyon (state machine, audit log) | ✅ |
| 5 | Yorum + puan (ON CONFLICT, tx-içi agregat, eşzamanlılık) | ✅ |
| 6 | Ağırlıklı puan (Bayesian, top-rated) | ✅ |
| 7 | Redis cache + rate limit + yük testi — **MVP sonu** | ✅ |
| 8–12 | Docker imajı · Frontend · CI/CD · NGINX · Kubernetes | ⏭️ sırada |

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

## Faz 4'te ne oldu (özet)

- **Uçlar:** `POST /api/books/suggestions` (USER → PENDING); admin: kuyruk (`GET /api/admin/books?status=`),
  tekil, `PATCH`, `approve|reject|restore`, `DELETE` (= arşiv), `.../audit`.
- **Kurallar:** durum makinesi `moderation.Next` (veri olarak); geçiş = `FOR UPDATE` + güncelle +
  denetim kaydı, tek tx; `audit_logs` DB trigger'ıyla append-only (TRUNCATE dahil).
- **Commit'ler:** `0455a70` çekirdek · `16310ab` uçlar · (docs).

## Faz 5'te ne oldu (özet)

- **Uçlar:** yorumlar (`GET/POST /api/books/{id}/comments`, admin `DELETE /api/admin/comments/{id}`),
  puan (`GET/PUT/DELETE /api/books/{id}/rating`, `GET .../ratings/summary`).
- **Doğruluk:** `(user_id, book_id)` PK + `ON CONFLICT`; agregat aynı tx'te `AVG/COUNT`; kitap
  satırı `FOR UPDATE`. Kilitsiz 25 eşzamanlı oyda sayım 16–19'a düşüyordu.
- **Commit'ler:** `69d44f8` audit refactor · `73bcb97` şema+servis · `cc18495` uçlar · (docs).

## Faz 6'da ne oldu (özet)

- `ratings.WeightedScore` (saf, bayrak tablo testi) + `GET /api/books/top-rated` (query-time SQL; SQL↔Go testli).
- EXPLAIN ANALYZE (`backend/scripts/perf_seed.sql`, ayrı `bookhub_perf` DB): public liste index'i
  düzeltildi (6.5 → 0.06 ms), C hesabı 43 → 5.7 ms. Top-rated sıralaması 14 ms — index'le hızlanamaz.
- **Commit'ler:** `1e4ab05` · `4e3217a` · `06f9ec8` · (docs).

## Faz 7'de ne oldu (özet)

- Redis (compose, **127.0.0.1:6380** — 6379 başka projenin): top-rated cache-aside (60 sn TTL +
  invalidation), login/register rate limit (Redis varsa paylaşılan, yoksa bellek içi).
- Yük testi: 327 → 2892 istek/sn (~8.8×). Rate limit canlı: bellek 10 vs Redis 5 (sınır 5, iki instance).
- **MVP DoD:** `set -a; source .env; set +a; API=http://localhost:8080 backend/scripts/mvp_smoke.sh`
- **Commit'ler:** `5e1a179` · `5a223d8` · `62c7c28` · `3376982` · (docs).

## Sıradaki: altyapı rayı (Faz 8–12, brief §18.2'ye göre opsiyonel)

8. Docker: çok aşamalı `Dockerfile` (distroless/scratch), compose'a `api` servisi, `Makefile`.
9. Frontend: React + Vite (liste, detay, giriş/kayıt, öneri, admin kuyruğu).
10. CI/CD: GitHub Actions (test + imaj build); gerçek bulut deploy'u hesap gerektirir (kullanıcıda).
11. NGINX: reverse proxy + statik frontend, güvenilir proxy başlığı.
12. Kubernetes: manifest'ler (Deployment/Service/ConfigMap/Secret şablonu), kind ile yerel deneme.

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
