# BookHub — Proje Haritası (neyin ne olduğu)

> **Bu dosya "NE" sorusunun cevabı:** hangi dosya ne işe yarar, bir istek nereden nereye gider,
> hangi araç niye var. Uzun aradan sonra dönünce burayı oku.
> **[STATUS.md](STATUS.md)** ise "NEREDEYİZ" sorusunun cevabı. İkisi kardeş: harita + imleç.
>
> Son güncelleme: 2026-10-05 — canlıya çıkış (2026-10-04) ve yayın süreci. 13 fazın tamamı
> 2026-09-20'de bitti (Faz 13: oturum sertleştirme).

---

## 1. Otuz saniyelik özet

**BookHub**, okunan kitapların kataloğu + topluluğu. Asıl amaç **öğrenme**: TS geçmişinden gelen
birinin backend teknolojilerini *neden var oldukları* üzerinden Go ile öğrenmesi. Her teknoloji
gerçek bir problem doğunca eklendi; kodun içi Türkçe "neden böyle" yorumlarıyla dolu.

- **Yığın:** Go (chi, pgx, sqlc, goose, slog) · PostgreSQL · Redis · React + Vite · NGINX ·
  Docker · Kubernetes · GitHub Actions · Playwright. **ORM yok.**
- **Kapsam:** MVP (Faz 1–7) + altyapı rayı (Faz 8–12) + oturum sertleştirme (Faz 13) tamam.
  Canlıda (2026-10-04): Northflank + Supabase + Upstash — [DEPLOYMENT.md](DEPLOYMENT.md).
- **Boyut:** ~7.7k satır Go (+ ~9.3k satır test, 226 test fonksiyonu), ~1.2k satır TypeScript
  (+ ~2.4k vitest, ~0.9k Playwright), 8 migration, 57 öğrenme notu.

---

## 2. Çalıştırma

```bash
cp .env.example .env            # bir kez
make app-up                     # her şey konteynerde → http://localhost:3000
make demo-seed                  # örnek katalog
make admin EMAIL=sen@ornek.com  # ilk admin (parola gizli sorulur)
make smoke                      # MVP tanımını uçtan uca doğrula (25 kontrol)
make e2e                        # gerçek tarayıcı (Playwright): yığını kendisi kaldırır
make scan                       # yayınlanan imajları Trivy ile tara
make help                       # tüm hedefler
```

| Ne | Nerede |
|---|---|
| Uygulama (NGINX → frontend + API) | http://localhost:3000 |
| API (doğrudan) | http://localhost:8080 |
| Frontend geliştirme (`make web`) | http://localhost:5173 |
| Kubernetes (`make k8s-up && make k8s-port`) | http://localhost:3001 |
| Postgres / Redis (host'tan) | 127.0.0.1:5432 / 127.0.0.1:6380 |

> ⚠️ **Redis host portu 6380** — bu makinede 6379'u başka bir projenin Redis'i kullanıyor.
> ⚠️ Uygulama `.env`'i kendisi okumaz: host'ta `set -a; source .env; set +a` (Makefile bunu yapıyor).

---

## 3. Bir isteğin yolculuğu (`PUT /api/books/42/rating`)

```
tarayıcı ─▶ NGINX (Faz 11)          CSP/başlıklar · /api/auth/ için ikinci rate limit
               │ X-Real-IP = gördüğü adres (istemcinin gönderdiğini EZER)
               ▼
Go API ─ chi middleware zinciri (router.go)
   RequestID ─▶ resolveClientIP (yalnız güvenilen proxy'nin başlığına inan)
   ─▶ requestLogger (tek JSON satırı + X-Request-Id) ─▶ Recoverer
   ─▶ crossOriginProtection (Faz 13: başka siteden gelen YAZMA isteği → 403; GET'ler geçer)
   ─▶ requireAuth (JWT: HS256 sabit, iss/aud/exp) → Principal context'e
   ─▶ handleRateBook (community.go): gövde doğrula (1–10), kullanıcı = TOKEN'dan (IDOR yok)
   ─▶ ratings.CachedService.Rate
        └▶ ratings.Service.Rate — TEK transaction:
             1. SELECT … FROM books … FOR UPDATE      (eşzamanlı oylar sıraya girer)
             2. INSERT … ON CONFLICT DO UPDATE        (bir kez oy + güncelleme, tek deyim)
             3. UPDATE books SET avg/count FROM (SELECT AVG/COUNT …)   (kaynaktan türet)
        └▶ commit SONRASI: Redis'ten books:top-rated:v1 sil
   ◀─ 201 (yeni) / 200 (güncelleme) + {score, average_rating, rating_count}
```

Çekirdek fikir: **her katman bir sonrakinin sürücüsünü bilmez** — handler pgx'i tanımaz,
repository/servis HTTP'yi tanımaz; hatalar domain hatasına çevrilip HTTP'ye tek yerde eşlenir.

---

## 4. Dosya haritası

### Kök

| Dosya | Ne |
|---|---|
| `Makefile` | tüm komutların kısayolları (`make help`) |
| `docker-compose.yml` | Postgres + Redis (her zaman) · api + web (`app` profili) · sabit alt ağ |
| `.env.example` | env şablonu (dev değerleri, açıklamalı); gerçek `.env` git dışı |
| `.githooks/` | pre-commit (gofmt + vet), pre-push (golangci-lint + test -race) |
| `.github/workflows/` | `ci.yml` (backend / frontend / **images** = Trivy taraması / e2e = compose + smoke + Playwright), `deploy.yml` (main'de CI yeşil → yeni migration varsa uygula → `release` dalını ilerlet; Northflank yalnız `release`'i yayınlar), `release.yml` (yalnız tag/elle ghcr imaj yayını; yayın yolunun parçası değil) |
| `.github/ISSUE_TEMPLATE/`, `pull_request_template.md` | özellik / bug / olay şablonları ve PR şablonu — [PROCESS.md](PROCESS.md) §2–§4 |
| `CLAUDE.md` | proje kuralları: her Claude oturumu ve AI review kapısı okur (önleme kuralları burada) |
| `scripts/setup-github.sh` | bir kerelik: etiketler + `release` dalı ([PROCESS.md](PROCESS.md) §9) |
| `.github/dependabot.yml` | haftalık bağımlılık PR'ları: docker digest'leri, Go modülleri, npm, actions — digest pinlemenin diğer yarısı (Faz 13) |
| `scripts/e2e.sh` | `make e2e`: yığını kaldırır, NGINX auth kovasını sıfırlar (web restart), demo veriyi basar, koşuya özel rastgele parolalı admin açar, Playwright'i koşar, çıkarken bu koşunun kullanıcılarını siler |
| `deploy/k8s/` | Kubernetes manifestleri (kustomize) |

### `backend/cmd/api/` — HTTP katmanı (paket `main`)

| Dosya | Ne |
|---|---|
| `main.go` | `run() int`: config → migrate (ops.) → pgxpool → Redis (ops.) → servisler → router → graceful shutdown; `healthcheck` alt komutu |
| `router.go` | `deps` struct'ı, middleware zinciri, tüm rotalar (public / giriş / admin grupları); `crossOriginProtection()` = Go 1.25 `http.CrossOriginProtection` + JSON 403 `cross_origin_request` (Faz 13) |
| `middleware.go` | `requireAuth` (401), `requireAdmin` (403), `rateLimit` (429), `resolveClientIP`, `requestLogger` |
| `respond.go` | `writeJSON`, hata zarfı, `writeInternalError` (request id'li log), `readJSON` (1 MiB, bilinmeyen alan = 400) |
| `validate.go` | `validator` (hataları biriktirir), alan sınırları, slug/dil/URL kuralları |
| `books.go` | public kitap uçları, public DTO, `createBookRequest`, ortak alan kontrolleri (`checkTitle`…) |
| `moderation.go` | öneri ucu + admin kitap uçları, admin DTO, durum geçişleri (method expression) |
| `community.go` | yorum + puan + top-rated uçları |
| `genres.go` | tür uçları |
| `auth.go` | register/login/refresh/logout/me + `handleChangePassword` (`POST /api/auth/password`); refresh çerezi yardımcıları: `cookieSettings`/`cookieSettingsFrom`, `newRefreshCookie`, `setRefreshCookie`, `clearRefreshCookie`, `readRefreshCookie` (`r.CookiesNamed`: tam 1, ≤256 bayt) |
| `*_test.go` | sahte (fake) servislerle DB'siz handler testleri; sözleşme (alan listesi) testleri; korelasyon testi |

### `backend/cmd/manage/` — operatör komutları

`migrate` (şemayı uygula) · `create-admin -email E` (parola prompt/stdin'den; argümanla ASLA).

### `backend/internal/` — iş mantığı ve veri

| Paket | Ne |
|---|---|
| `config` | env → `Config`, fail-fast doğrulama (JWT ≥32 bayt, TTL'ler, proxy CIDR'ları, `JWT_SESSION_MAX_AGE ≥ JWT_REFRESH_TTL`, `COOKIE_NAME` RFC 6265 token + `__Host-`/`__Secure-` önek kuralları) |
| `database` | pgxpool (`New`), `Migrate` (goose + advisory lock), `WithTx`, Postgres hata kodları |
| `db` | **sqlc'nin ürettiği** kod — elle düzenlenmez (`make sqlc`) |
| `books` | kitap repository'si (public okuma yalnız APPROVED), statüler, `ParseStatus` |
| `genres` | tür repository'si |
| `auth` | bcrypt, JWT (`TokenManager`), refresh token **aileleri**, `Service` (ilk servis katmanı). Repository: `StartRefreshFamily` (girişte `FOR SHARE` hash kontrolü), `RotateRefreshToken` (→ `Rotation{UserID, FamilyID, ExpiresAt, User}`), `RevokeRefreshFamily`, `CleanupRefreshTokens`, `ChangePassword`; hata tipi `ReuseError` (`Unwrap → ErrInvalidToken`), sentinel `ErrWrongPassword` |
| `moderation` | durum makinesi (`Next`, veri olarak), öneri/onay/red/arşiv, `WithPublicChangeHook` |
| `audit` | denetim kaydı yazıcısı (append-only tablo) |
| `comments` | yorum servisi (yumuşak silme + denetim) |
| `ratings` | puan servisi (kilit + upsert + türetme), `WeightedScore` (saf), `CachedService` (Redis) |
| `cache` | `Cache` arayüzü: Redis / Noop |
| `ratelimit` | sabit pencere: Memory / Redis / Unlimited |
| `slug` | NFD ile Türkçe/aksanlı → URL slug'ı, `Truncate` |
| `testdb` | entegrasyon testlerinin ortak DB kurulumu |

### `backend/migrations/` ve `backend/sql/queries/`

| Migration | İçerik |
|---|---|
| `0001` | books, genres, book_genres |
| `0002` | users (email CHECK lower, role DEFAULT USER), refresh_tokens (SHA-256) |
| `0003` | ARCHIVED, moderasyon kolonları, `audit_logs` (trigger ile append-only) |
| `0004` | ratings (`(user_id, book_id)` PK), comments (+ `visible_comments` VIEW), avg/count kolonları |
| `0005` | liste index'leri `(created_at DESC, id DESC)` — EXPLAIN ANALYZE bulgusu |
| `0006` | puan silinince (CASCADE dahil) kitap istatistiklerini yenileyen trigger |
| `0007` | o trigger önce kitabı kilitlesin (eşzamanlı oyla kayıp güncelleme olmasın) |
| `0008` | refresh token **aileleri**: `refresh_token_families` (`id`, `user_id`, `created_at`, `absolute_expires_at`, `revoked_at`) + `refresh_tokens.family_id` (NOT NULL, CASCADE) ve `refresh_tokens.used_at`; `TRUNCATE refresh_tokens` (eski token'ların ailesi yok) |

`sql/queries/*.sql` = sqlc girdisi (el yazması SQL); `migrations/embed.go` migration'ları binary'ye gömer.

`users.sql`'deki Faz 13 sorguları (hepsinin üstünde NEDEN yorumu var):
`CreateRefreshFamily` · `CreateRefreshToken` · `GetRefreshTokenFamily` (kilitsiz) ·
`LockRefreshFamily` (`FOR UPDATE` — kilit sırasının ilk adımı) · `MarkRefreshTokenUsed` ·
`GetRefreshTokenUsedAt` (reuse ayrımı) · `RevokeRefreshFamily` · `DeleteRefreshFamilyTokens` ·
`DeleteDeadRefreshFamilies` / `DeleteExpiredRefreshTokens` (girişteki temizlik, `FOR UPDATE SKIP
LOCKED`) · `LockUserPasswordForChange` (`FOR NO KEY UPDATE`) · `LockUserPasswordForLogin`
(`FOR SHARE`) · `UpdateUserPassword` · `LockUserLiveRefreshFamilies` · `RevokeUserRefreshFamilies` ·
`DeleteUserRefreshTokens`.

> **Kilit sırası değişmezi (Faz 13):** bir aileye dokunan her transaction `users` →
> `refresh_token_families` → `refresh_tokens` sırasıyla kilitler. Ters sıra gerçek bir deadlock
> (`40P01`) üretti; bugün gerçek DB testleri bunu koruyor.

### `backend/scripts/`

`mvp_smoke.sh` (MVP tanımı, 25 kontrol) · `demo_seed.sql` (örnek katalog) · `perf_seed.sql` (20k kitap / 209k oy, ayrı DB).

### `frontend/`

| Dosya | Ne |
|---|---|
| `src/api/client.ts` | fetch sarmalayıcı + **oturum durum makinesi** (Faz 13'te yeniden yazıldı): access token yalnız bellekte, refresh token httpOnly çerezte (bu dosya onu hiç görmez), `localStorage`'da yalnız sır olmayan `bookhub.session` ipucu ve `bookhub.logout_pending` bayrağı; çerez yazan tüm istekler (login / refresh / logout / password) **Web Locks** (`bookhub-refresh`) ile origin başına seri; `sessionEpoch` geç gelen cevabın oturumu diriltmesini engeller |
| `src/api/types.ts` | API sözleşmesinin TS tipleri (`Session`'da `refresh_token` YOK) |
| `src/auth/AuthContext.tsx` | oturum durumu, açılışta geri yükleme |
| `src/pages/` | Home (katalog + en iyiler), BookDetail, Auth, Suggest, **Account** ("Hesabım": parola değiştirme, Faz 13), admin/Moderation |
| `e2e/` | Playwright senaryoları (`catalog`, `auth-flow`, `reuse`, `tabs`, `cross-origin`, `rating-comment`, `moderation`) + `env.ts`, `helpers.ts` (**rate limit bütçesi tablosu**), `global-setup.ts` |
| `playwright.config.ts` | yalnız chromium, `baseURL` = `E2E_BASE_URL`, `trace: retain-on-failure`, CI'da 1 yeniden deneme |
| `nginx/` | `default.conf` (auth + **password** rate limit zone'ları, `$api_host` map'i), `api-proxy.conf` (`Host` = `$http_host` → port korunur), `security-headers.conf` |
| `Dockerfile` | node build → nginx-unprivileged **stable-alpine-slim** (uid 101, :8080), tabanlar digest ile pinli |

---

## 5. Kritik kararlar ve kanıtları

| Karar | Kanıt (test / ölçüm) |
|---|---|
| Görünürlük sınırı SQL'de (`GetApprovedBook`) | öneri onaya kadar public 404 (smoke) |
| Ayrı request/response DTO | `status`/`role` gönderen istek 400; iç alanlar public'e sızmıyor (sözleşme testleri) |
| JWT algoritma sabitleme | sabitleme kaldırılınca HS512 token kabul ediliyor (mutation) |
| bcrypt 72 bayt koruması | koruma kaldırılınca "72 bayt + sonek" giriş yapıyor (regresyon testi) |
| Tek kullanımlık refresh (Faz 13'ten beri `used_at` ile işaretlenir, silinmez) | 20 eşzamanlı yenileme → tam 1 kazanan, ≥1 `ReuseError`, aile iptal |
| Reuse tespiti = aile iptali | harcanmış token tekrar gelince **halefi de** 401 (Playwright `reuse.spec.ts`) |
| Kilit sırası `users → families → tokens` | ters sıra gerçek `40P01`; kilit testleri mutasyonda kırılıyor |
| `FOR NO KEY UPDATE` (parola) vs `FOR KEY SHARE` (FK) | `FOR UPDATE`'e çevirince deadlock (mutation testi) |
| Refresh token httpOnly çerezte | gövdede `refresh_token` yok, `localStorage`'da yalnız `{bookhub.session:"1"}` (gerçek tarayıcı testi) |
| Cross-origin yazma reddi | `Sec-Fetch-Site: cross-site` → 403; `Origin == Host` → geçer (NGINX regresyonu) |
| Moderasyonda `FOR UPDATE` | kilitsiz: iki admin de "kazanıyor", denetimde çelişen iki karar |
| Puanda kilit + kaynaktan türetme | kilitsiz: 25 oydan sayım 16–19 (kayıp güncelleme) |
| Ağırlıklı skor query-time | SQL skoru = Go spesifikasyonu (1e-9) |
| Index `(created_at, id)` | 6.5 ms → 0.06 ms (EXPLAIN ANALYZE) |
| Redis cache (top-rated) | 327 → 2892 istek/sn, p95 236 → 27 ms |
| Paylaşılan rate limit | iki instance: bellek 10, Redis 5 (sınır 5) |
| Güvenilen proxy | sahte X-Real-IP iki yoldan da yok sayılıyor |

---

## 6. Araçlar — ne işe yarar

| Araç | Rol |
|---|---|
| **sqlc** | SQL yaz → tip güvenli Go üret (`make sqlc`); şemaya karşı derleme anında doğrular |
| **goose** | versiyonlu migration (up/down); binary'ye gömülü, advisory lock'lu |
| **golangci-lint** | ESLint'in Go karşılığı (pre-push + CI) |
| **Docker Compose** | dev altyapısı; `app` profiliyle tüm uygulama |
| **kind** | Docker içinde Kubernetes (yerel küme) |
| **ab** (ApacheBench) | yük testi (Faz 7 ölçümleri) |
| **Playwright** | gerçek tarayıcıda (chromium) uçtan uca testler: çerez, sekmeler, dialog (`make e2e`) |
| **Trivy** | imaj açık taraması; CI'daki `images` işi ve `make scan` aynı kuralı koşar |
| **Dependabot** | digest/bağımlılık güncellemelerini haftalık PR olarak getirir |

---

## 7. Bilinen borçlar

**Faz 13'te KAPANANLAR** (kayıt kalsın diye siliniyor değil, üstü çiziliyor):

- ~~Refresh token `localStorage`'da → httpOnly + `SameSite=Strict` çerez daha güçlü.~~
  ✅ Kapandı (`1d2e92d`): `Path=/api/auth`, `HttpOnly`, `SameSite=Strict`, `Secure`; JSON gövdesinde
  `refresh_token` yok. `localStorage`'da yalnız sır olmayan `bookhub.session` ipucu var.
- ~~Token "aile" iptali (eski refresh token tekrar gelirse tüm aileyi düşür) yok.~~
  ✅ Kapandı (`362dc11`): aile + `used_at` + reuse tespiti + mutlak tavan.
- ~~Arayüzün giriş gerektiren akışları tarayıcıda otomatik test edilmedi.~~
  ✅ Kapandı (`5192b19`): gerçek chromium'da 11 Playwright senaryosu (`make e2e`), CI'da da koşuyor.

**Açık kalanlar:**

- Access token'lar stateless: çıkış / reuse iptali / parola değişimi / rol değişikliği **refresh**
  tarafını anında keser ama verilmiş access token'lar ≤ `JWT_ACCESS_TTL` (15 dk) yaşar
  ([REVIEW_FEEDBACK F06](REVIEW_FEEDBACK.md)).
- [REVIEW_FEEDBACK.md](REVIEW_FEEDBACK.md)'deki açık bulgular: F03, F04, F06–F09, F11–F15, F17 +
  F10'un BookDetail yarısı, F16'nın kalan yarısı (`release.yml` ve `deploy/k8s/**` path filtresi).
  F05 risk kabul edildi.
- Kullanıcı silme ucu yok (hesap yalnızca veritabanından silinebiliyor).
- Canlıda izleme ve alarm yok (sağlık kontrolü, uptime, hata izleme) — [DEPLOYMENT.md](DEPLOYMENT.md) §5.
- Küme içi Postgres yalnız demo içindir; prod'da yönetilen veritabanı.
- Ürün tarafındaki istekler ayrı bir listede: [FEATURE_REQUESTS.md](FEATURE_REQUESTS.md).
