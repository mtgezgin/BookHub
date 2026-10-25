# BookHub — Durum & Devir (STATUS)

> **Yeni bir konuşma bunu İLK okusun.** Nerede olduğumuzu, kararları ve sıradaki adımı özetler.
> Her faz sonunda güncellenir. "Neyin ne olduğu" için: [PROJECT_MAP.md](PROJECT_MAP.md).

**Son güncelleme (2026-10-25):** **15 fazın tamamı ✅** (Faz 15 kişisel kitaplık canlıda, #43) · **Şimdi: Faz 16 — eser ve baskı** ([Sıradaki](#sıradaki)) — MVP (Faz 1–7, brief §15;
`backend/scripts/mvp_smoke.sh` 25/25) + altyapı rayı (Docker, React frontend, CI/CD, NGINX,
Kubernetes) + **Faz 13 oturum sertleştirme** (httpOnly refresh çerezi, token aileleri + reuse
tespiti, CSRF koruması, parola değişimi, gerçek tarayıcıda Playwright).

- **Canlıda (2026-10-04):** Northflank (`web` public, `api` private) + Supabase + Upstash —
  https://p01--web--4x2qb6s8vg5j.code.run · [DEPLOYMENT.md](DEPLOYMENT.md).
- **Süreç (2026-10-05, canlıda 2026-10-12):** issue/PR şablonları, testlere bağlı yayın (`release`
  dalı + Deploy iş akışı; main'e her merge = yayın), [CLAUDE.md](../CLAUDE.md) kuralları,
  makinenin denetlediği kurallar (`scripts/checks.sh`: git kancaları + CI'da Kurallar) ·
  [PROCESS.md](PROCESS.md).
- **Yol haritası (2026-10-17):** hedef "üretim kalitesinde demo"; sıra ve gerekçe
  [ROADMAP.md](ROADMAP.md)'de. Faz 15 kişisel kitaplık ✅ → **Şimdi: Faz 16 — eser ve baskı**
  ([#36](https://github.com/mtgezgin/BookHub/issues/36), spec [incelemede](phases/phase-16-works-editions.md)) →
  Faz 17 okuma takibi ve inceleme.
- **Teknik hat:** canlı imajlara haftalık güvenlik taraması ([#48](https://github.com/mtgezgin/BookHub/issues/48)),
  Node 26 (Dependabot #13, #29; LTS'i bekliyor), canlı izleme ve alarm (henüz yok),
  [REVIEW_FEEDBACK.md](REVIEW_FEEDBACK.md)'deki kapanmamış bulgular (ilgili fazın içinde kapanır).

> **Faz 15 — kişisel kitaplık (canlıda 2026-10-25):** dört durumla gizli kitaplık (migration `0010`),
> kitap sayfasında ekleme/durum menüsü, kartlarda durum rozeti (tek toplu istek), süzgeçleri adreste
> duran Kitaplığım sayfası — PR #43 (#42); yayından önce imaj açıkları kapandı (Go 1.27.3, nginx 1.30.6;
> #47, PR #49) · [phase-15](phases/phase-15-personal-library.md).

> **Faz 14 — arayüz yeniden tasarımı (canlıda 2026-10-12):** A tasarım temeli · B ana sayfa +
> detay · C formlar, gezinme, tema, moderasyon diyaloğu · D katalog sıralaması (`?sort=rating`) —
> PR #21, takipleri #25; gerçek kapaklar (Open Library, #34) ·
> [phase-14](phases/phase-14-ui-redesign.md).

> **Ortam notu:** Kabukta `go` bulunamazsa `export PATH="/opt/homebrew/bin:$HOME/go/bin:$PATH"`
> — git hook'ları da `go: command not found` ile bu yüzden patlıyor.

---

> **Öğrenme modu (20 Eylül 2026'da yeniden kuruldu):** teori önce, minik deney, sonra kod —
> ayrıntı ve sonraki oturumun başlangıç noktası: [LEARNING_PLAN.md](LEARNING_PLAN.md).

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
| 8 | Docker imajı + compose (`app` profili) | ✅ |
| 9 | Frontend (React + Vite) | ✅ |
| 10 | CI/CD (servis konteynerleri, E2E, release) | ✅ (canlı: Northflank, 2026-10-04) |
| 11 | NGINX (tek origin, CSP, güvenilen proxy) | ✅ |
| 12 | Kubernetes (kind'da doğrulandı) | ✅ |
| 13 | Oturum sertleştirme (httpOnly çerez, token ailesi + reuse, CSRF, parola değişimi, Playwright) | ✅ |
| 14 | Arayüz yeniden tasarımı (tasarım token'ları, shadcn/ui bileşenleri, erişilebilirlik, katalog sıralaması) | ✅ (canlı, gerçek kapaklar dahil) |
| 15 | Kişisel kitaplık (durum, Kitaplığım sayfası, kart rozetleri; migration 0010) | ✅ (canlı 2026-10-25) |

## Faz 2'de ne oldu (özet)

- **Uçlar:** `GET /api/books` (sayfalı: `?genre=&q=&sort=rating&limit=&offset=` → `{items,total,limit,offset}`),
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

## Faz 8–12'de ne oldu (özet)

- **8 Docker:** distroless imaj 38 MB, uid 65532; slog JSON log + `X-Request-Id`; `run() int` ile temiz çıkış.
- **9 Frontend:** React 19 + Vite; tek uçuşlu token yenileme (refresh token tek kullanımlık olduğu için şart).
- **10 CI/CD:** gerçek Postgres/Redis ile entegrasyon testleri, frontend işi, compose + smoke E2E;
  imaj yayını yalnız `v*` etiketi/elle (`release.yml`; 2026-10-12'de kaldırıldı, Northflank
  imajı kendisi derliyor).
- **11 NGINX:** tek origin, CSP + başlıklar (add_header tuzağı snippet ile aşıldı), `X-Real-IP`'ye
  yalnız güvenilen proxy'den güven; root olmayan NGINX.
- **12 Kubernetes:** kustomize manifestleri, migrate Job, 2 API kopyası, pod güvenliği; kind'da 21/21
  (o günkü smoke; Faz 13'te kontrol sayısı 25'e çıktı).

## Faz 13'te ne oldu (özet)

Tetikleyen: ilk kapsamlı kod/güvenlik incelemesi ([REVIEW_FEEDBACK.md](REVIEW_FEEDBACK.md)) + senin
"httpOnly çerez + token ailesi iptali + Playwright" isteğin. Ayrıntı ve NEDEN'ler:
[phases/phase-13-session-hardening.md](phases/phase-13-session-hardening.md).

- **Token aileleri + reuse tespiti** (migration `0008`): bir giriş = bir aile; kullanılmış token
  silinmez (`used_at`), ikinci kez gelirse **aile** ölür. Mutlak tavan `JWT_SESSION_MAX_AGE`
  (30 gün). Kilit sırası değişmezi `users → families → tokens`; ters sıra gerçek `40P01` üretmişti.
- **Refresh token httpOnly çerezte** (`Path=/api/auth`, `SameSite=Strict`, `Secure`): JSON gövdesinde
  `refresh_token` yok, `localStorage`'da sır yok (yalnız `bookhub.session` ipucu bayrağı).
- **CSRF:** Go 1.25 `http.CrossOriginProtection` (403 `cross_origin_request`) + NGINX `$http_host`
  (Host'u portuyla ilet, yoksa meşru istek 403 alıyordu).
- **Parola değiştirme** (`POST /api/auth/password`, `/account` sayfası): kullanıcının TÜM ailelerini
  aynı transaction'da iptal eder; işlemi yapan cihaz yeni bir aileyle içeride kalır.
- **Playwright** (`make e2e`, `frontend/e2e/`): gerçek chromium'da 11 senaryo — çerez hijyeni,
  reuse, üç sekme/Web Locks, cross-origin, moderasyon. Katalogda bayat cevap hatasını (F10) buldu.
- **İmaj sertleştirme:** slim nginx tabanı (91 → 30 MB), digest pinleri + Dependabot, CI'da Trivy
  işi (`make scan` ile aynı kural), pod'larda `automountServiceAccountToken: false`.
- **Smoke 25/25** (Faz 13 kontrolleri dahil).

## Sıradaki

Sıra artık tek yerde: [ROADMAP.md](ROADMAP.md). Kısaca:

1. **Şimdi — Faz 16, eser ve baskı** (CAT-02, [#36](https://github.com/mtgezgin/BookHub/issues/36)). İlk adım
   [spec](phases/phase-16-works-editions.md)'in incelenmesi ve kararları; ardından iki PR: **genişlet**
   (0011 tablo + taşıma, 0012 demo baskılar) → canlıda doğrula → **daralt** (eski iki kolonu silen
   migration). Daraltma, genişletme yayını canlıda doğrulanmadan birleştirilmez.
2. **Sonra:**
   - Faz 17, okuma takibi ve inceleme.
   - Faz 18, katkı döngüsü ve hesap (gerçek kullanıcı kararıyla öne alınır).
3. **Teknik hat (araya, küçük):**
   - Canlı imajlara haftalık güvenlik taraması ([#48](https://github.com/mtgezgin/BookHub/issues/48)).
   - Node 26.
   - Canlı izleme ([PROCESS.md](PROCESS.md) §7).
   - [REVIEW_FEEDBACK.md](REVIEW_FEEDBACK.md) bulguları: açık olanlar F03, F04, F06–F09, F11–F15, F17
     ve F10/F16'nın kalan yarıları; F05 risk kabul edildi.
   - Bilinen borçlar: [PROJECT_MAP.md](PROJECT_MAP.md) §7.
4. **Kural:** yeni fikir önce [FEATURE_REQUESTS.md](FEATURE_REQUESTS.md)'e yazılır. Issue yalnız
   "Şimdi" fazı için açılır ([ROADMAP § Kurallar](ROADMAP.md#kurallar)).

## Ortam / gerçekler

- **Go toolchain 1.27.1** (Homebrew) · `go.mod`: `go 1.27.3` (Docker derleme imajıyla aynı yama; 1.27.2 CVE-2026-111943 yüzünden bırakıldı, #47) · module `github.com/mtgezgin/bookhub/backend`.
- CLI'lar `~/go/bin`: goose v3.27.2, sqlc v1.31.1. golangci-lint v2.14.0 Homebrew'dan (CI'da da v2.14.0
  pinli); `~/go/bin`'deki eski v1.64.8 PATH'te arkada kalıyor.
- Docker Desktop + `docker compose up -d` → Postgres 16 (`127.0.0.1:5432`) + Redis 7 (`127.0.0.1:6380` — 6379 başka projenin). Veri named volume'da.
- Tüm uygulama: `make app-up` → http://localhost:3000 · Kubernetes: `make k8s-up` / `make k8s-down` (kind).
- Canlı: Northflank, `release` dalından yayınlanıyor; `release`'i yalnız Deploy iş akışı ilerletir — [PROCESS.md](PROCESS.md) §5.
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

- [ROADMAP.md](ROADMAP.md) — "hangi sırayla ve neden?" (Faz 15+ ve teknik hat).
- [PROJECT_MAP.md](PROJECT_MAP.md) — "neyin ne olduğu" (dosya dosya, istek yolculuğu, araçlar).
- [PROJECT_BRIEF.md](PROJECT_BRIEF.md) — ürün + ilk roadmap (§11, Faz 1–12) + kararlar (§18, §19).
- [phases/](phases/) — faz rehberleri · [GO_CONVENTIONS.md](GO_CONVENTIONS.md) · [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md)
- [learning-notes/](learning-notes/) — sorulan kavramların kalıcı notları (`/not` skill'i ekler).
- **Canlı backlog'lar:** [REVIEW_FEEDBACK.md](REVIEW_FEEDBACK.md) — inceleme bulguları + çözüm
  kayıtları (kutuyu ancak kabul kriteri doğrulanınca işaretle) · [FEATURE_REQUESTS.md](FEATURE_REQUESTS.md)
  — ürün/özellik istekleri (senin sahipliğinde; ajanlar buraya kendiliğinden yazmaz).
