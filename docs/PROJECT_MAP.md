# BookHub — Proje Haritası (neyin ne olduğu)

> **Bu dosya "NE" sorusunun cevabı:** hangi dosya ne işe yarar, bir istek nereden nereye gider,
> hangi araç niye var. Uzun aradan sonra dönünce burayı oku.
> **[STATUS.md](STATUS.md)** ise "NEREDEYİZ" sorusunun cevabı. İkisi kardeş: harita + imleç.
> "Hangi sırayla?" sorusunun cevabı: **[ROADMAP.md](ROADMAP.md)**.
>
> Son güncelleme: 2026-11-22 — Faz 1–16d canlıda: arama (16b, migration `0014`), katalog sorguları (16c),
> arayüz veri katmanı (16d, TanStack Query), ikinci incelemenin düzeltmeleri (index'ler `0015`, süre
> bütçeleri, istek ölçümü, hata sınırı), OpenTelemetry.

---

## 1. Otuz saniyelik özet

**BookHub**, okunan kitapların kataloğu + topluluğu. Asıl amaç **öğrenme**: TS geçmişinden gelen
birinin backend teknolojilerini *neden var oldukları* üzerinden Go ile öğrenmesi. Her teknoloji
gerçek bir problem doğunca eklendi; kodun içi Türkçe "neden böyle" yorumlarıyla dolu.

- **Yığın:** Go (chi, pgx, sqlc, goose, slog, OpenTelemetry) · PostgreSQL · Redis · React + Vite · NGINX ·
  Docker · Kubernetes · GitHub Actions · Playwright · yerelde Grafana + Prometheus. **ORM yok.**
- **Kapsam:** MVP (Faz 1–7) + altyapı rayı (Faz 8–12) + oturum sertleştirme (Faz 13) + ürün fazları
  14–16d tamam. Canlıda (2026-10-04'ten beri): Northflank + Supabase + Upstash — [DEPLOYMENT.md](DEPLOYMENT.md).
- **Boyut (2026-11-19, `wc -l`):** ~8,7k satır Go (üretilmiş `internal/db` hariç; + ~13,7k satır test,
  303 test fonksiyonu), ~3,4k satır TypeScript (+ ~4,8k vitest, ~1,1k Playwright), 15 migration, ~90
  öğrenme notu.

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
   ─▶ observe (OpenTelemetry iz + süre metriği; adres verilmemişse hiçbir şey yapmaz)
   ─▶ requestLogger (tek JSON satırı + X-Request-Id + db_ms/redis_ms) ─▶ recoverer
   ─▶ requestBudget (süre bütçesi: okuma 5 sn, yazma 9 sn; dolarsa 503 timeout)
   ─▶ crossOriginProtection (Faz 13: başka siteden gelen YAZMA isteği → 403; GET'ler geçer)
   ─▶ requireAuth (JWT: HS256 sabit, iss/aud/exp) → Principal context'e
   ─▶ handleRateBook (community.go): gövde doğrula (1–10), kullanıcı = TOKEN'dan (IDOR yok)
   ─▶ ratings.CachedService.Rate
        └▶ ratings.Service.Rate — TEK transaction:
             1. SELECT … FROM books … FOR UPDATE      (eşzamanlı oylar sıraya girer)
             2. INSERT … ON CONFLICT DO UPDATE        (bir kez oy + güncelleme, tek deyim)
             3. UPDATE books SET avg/count FROM (SELECT AVG/COUNT …)   (kaynaktan türet)
        └▶ "en sevilenler" önbelleği SİLİNMEZ (Faz 16c): 60 sn'de kendiliğinden yenilenir.
           Moderasyon ve baskı değişiklikleri silmeye devam eder (gizlenen kitap hemen kalksın).
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
| `.githooks/` | pre-commit (`scripts/checks.sh staged` + gofmt/vet), commit-msg (atlama bayrağı), pre-push (golangci-lint + test -race + frontend değiştiyse vitest) |
| `.github/workflows/` | `ci.yml` (backend / frontend / **images** = Trivy taraması / **upgrade** = `release`'in şeması + veri üstüne bu commit'in migration'ları / e2e = compose + smoke + Playwright; PR'da yol süzgeci yok), `deploy.yml` (main'de CI yeşil → yeni migration varsa uygula → `release` dalını ilerlet; Northflank yalnız `release`'i yayınlar), `rules.yml` (**Kurallar**: her push/PR'da `scripts/checks.sh` ve testleri), `dependency-review.yml` (PR'ın eklediği paketlerde HIGH/CRITICAL açık → kırmızı), `scan.yml` (haftalık Trivy, `release` üzerinde), `uptime.yml` (günlük erişilebilirlik). `main`'de 8 zorunlu kontrol: [PROCESS.md](PROCESS.md) §1 |
| `.github/ISSUE_TEMPLATE/`, `pull_request_template.md` | özellik / bug / olay şablonları ve PR şablonu — [PROCESS.md](PROCESS.md) §2–§4 |
| `CLAUDE.md` | proje kuralları: her Claude oturumu ve AI review kapısı okur (önleme kuralları burada) |
| `scripts/checks.sh`, `checks_test.sh` | makinenin denetlediği repo kuralları (sır, `.env`/`supabase/`, canlıya çıkmış migration, migration sırası ve numarası, Go sürüm kayması, NGINX log biçimi, telemetri gizliliği, action'ların SHA ile sabitlenmesi, üretilmiş kod, lint, atlama bayrağı) ve testleri; kancalar ve CI'daki **Kurallar** işi aynı betiği çağırır — [PROCESS.md](PROCESS.md) |
| `scripts/upgrade-check.sh` | CI'daki **Yükseltme** işi: `release`'in şemasını ve demo veriyi kurar, üstüne bu commit'in migration'larını uygular (canlıdaki yayın sırasının taklidi; F38) |
| `.trivyignore` | Trivy'nin **süreli** istisnaları (`exp:` tarihi, gerekçe, issue); süre dolunca tarama yeniden kırmızı olur — [PROCESS.md](PROCESS.md) §5 |
| `.sonarcloud.properties` | SonarCloud'un otomatik analizinin ayarı (hariç tutulan dosyalar) |
| `ops/grafana/` | yerel Grafana'nın panosu ve kaynak tanımı (`make obs-up`); canlı Grafana Cloud'a da aynı pano yüklenir (#96) |
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
| `editions.go` | baskılar (Faz 16): `editionService`, `editionResponse`, detay DTO'ları (`bookDetailResponse`, `adminBookDetailResponse`: + `editions`), admin uçları (ekle / PATCH / sil / varsayılan yap), ISBN temizleme |
| `community.go` | yorum + puan + top-rated uçları |
| `library.go` | kişisel kitaplık uçları (`/api/me/library…`, Faz 15): `libraryStore` arayüzü (yalnız okuma), toplu durum (`parseBookIDs`, en çok 100), süzülen liste (`parseLibraryFilter`; okurken `unit`/`progress`/`total_pages`); eski PUT {status} ve DELETE artık raf servisinin komutları (Faz 17A-1) |
| `shelf.go` | raf uçları (Faz 17A-1): `shelfService` arayüzü, `GET/POST /api/me/books/{id}/shelf…` (komutlar tek üreticiden: `handleShelfCommand`), `PATCH …/shelf/reading` (sürümlü, 409), `PATCH/DELETE /api/me/reading-log/{id}`; `writeShelfError` hata haritası, günler `YYYY-AA-GG` |
| `genres.go` | tür uçları |
| `observe.go` | OpenTelemetry ara katmanı: istek başına iz (yalnız method, rota kalıbı, durum kodu) ve süre histogramı; telemetri kapalıyken boş |
| `auth.go` | register/login/refresh/logout/me + `handleChangePassword` (`POST /api/auth/password`); refresh çerezi yardımcıları: `cookieSettings`/`cookieSettingsFrom`, `newRefreshCookie`, `setRefreshCookie`, `clearRefreshCookie`, `readRefreshCookie` (`r.CookiesNamed`: tam 1, ≤256 bayt) |
| `*_test.go` | sahte (fake) servislerle DB'siz handler testleri; sözleşme (alan listesi) testleri; korelasyon testi |

### `backend/cmd/manage/` — operatör komutları

`migrate` (şemayı uygula) · `create-admin -email E` (parola prompt/stdin'den; argümanla ASLA; hesap varsa durur, `-promote` ile devralır).

### `backend/cmd/mcp/` — MCP sunucusu (#52)

Kataloğu bir dil modeline araç olarak açar (stdio, resmî Go SDK). REST API'nin istemcisidir — DB'ye
bağlanmaz, sır kullanmaz; adres `BOOKHUB_API_URL` (varsayılan `http://localhost:3000`). `make mcp`.

| Dosya | Ne |
|---|---|
| `main.go` | `run()`: adres doğrulama (`parseAPIURL`, fail fast) → sunucu → taşıma |
| `client.go` | `apiClient.get`: yalnız GET, 10 sn zaman aşımı, 1 MiB sınır; hata zarfı → modelin göreceği hata |
| `tools.go` | salt okunur araçlar: `search_books`, `get_book`, `top_rated`, `list_genres`, `book_comments`, `rating_summary` |
| `main_test.go` | sahte API (`httptest`) + bellek içi MCP istemcisi |

### `backend/internal/` — iş mantığı ve veri

| Paket | Ne |
|---|---|
| `config` | env → `Config`, fail-fast doğrulama (JWT ≥32 bayt, TTL'ler, proxy CIDR'ları, `JWT_SESSION_MAX_AGE ≥ JWT_REFRESH_TTL`, `COOKIE_NAME` RFC 6265 token + `__Host-`/`__Secure-` önek kuralları) |
| `database` | pgxpool (`New`; `DB_MAX_CONNS`, her bağlantıda `statement_timeout` 10 sn, `lock_timeout` 3 sn, `idle_in_transaction_session_timeout` 15 sn, `plan_cache_mode = force_custom_plan`), `Migrate` (goose + advisory lock), `WithTx`, Postgres hata kodları |
| `db` | **sqlc'nin ürettiği** kod — elle düzenlenmez (`make sqlc`) |
| `books` | kitap repository'si (public okuma yalnız APPROVED), statüler, `ParseStatus`; `CreateParams`/`UpdateParams` (kapak ve sayfa varsayılan baskıya), `Editions` (baskı okuma); `catalog.go` (Faz 16c): tür slug → kimlik eşlemesi ve tür başına toplam sayı süreç içinde 60 sn, sıraya ve türe göre doğru liste sorgusu |
| `editions` | baskı servisi (Faz 16): tek varsayılan kuralı (ilk baskı varsayılan, varsayılan silinemez, önce temizle sonra ata), kitap kilidi, denetim kaydı (`edition.*`), önbellek kancası |
| `genres` | tür repository'si |
| `auth` | bcrypt, JWT (`TokenManager`), refresh token **aileleri**, `Service` (ilk servis katmanı). Repository: `StartRefreshFamily` (girişte `FOR SHARE` hash kontrolü), `RotateRefreshToken` (→ `Rotation{UserID, FamilyID, ExpiresAt, User}`), `RevokeRefreshFamily`, `CleanupRefreshTokens`, `ChangePassword`; hata tipi `ReuseError` (`Unwrap → ErrInvalidToken`), sentinel `ErrWrongPassword` |
| `moderation` | durum makinesi (`Next`, veri olarak), öneri/onay/red/arşiv, `WithPublicChangeHook` |
| `audit` | denetim kaydı yazıcısı (append-only tablo) |
| `comments` | yorum servisi (yumuşak silme + denetim) |
| `ratings` | puan servisi (kilit + upsert + türetme), `WeightedScore` (saf), `CachedService` (Redis; puan yazımında silmez, Faz 16c), `GlobalAverage` (ağırlıklı puanın C'si süreç içinde 60 sn) |
| `library` | kişisel kitaplık ve okuma takibi. `Repository` yalnız okur: `Statuses` (toplu), `List` (süzgeç + `CASE` ile sıralama, başlık `tr-x-icu`, toplam aynı sorguda `count(*) OVER ()`). Yazımlar `Service`'te (Faz 17A-1): saf durum makinesi `resolve` (`commands.go`), komutlar, okurken düzeltme (`version`), geçmiş; (kullanıcı, kitap) danışma kilidi |
| `cache` | `Cache` arayüzü: Redis / Noop |
| `ratelimit` | sabit pencere: Memory / Redis / Unlimited |
| `slug` | NFD ile Türkçe/aksanlı → URL slug'ı, `Truncate` |
| `reqstats` | bir isteğin veritabanında ve Redis'te geçirdiği süreyi toplar; erişim logunda `db_ms`, `redis_ms` (F32) |
| `telemetry` | OpenTelemetry kurulumu (iz + metrik, OTLP); `OTEL_EXPORTER_OTLP_ENDPOINT` boşsa kapalı; veritabanı ve Redis donatması tek yerde, istek verisi taşımaz |
| `testdb` | entegrasyon testlerinin ortak DB kurulumu; `CreateUser` / `CreateBook` test verisi yardımcıları |

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
| `0009` | demo kataloğun gerçek kapakları (veri): slug + yazar eşleşen kitapta boş `cover_url`'i Open Library kapak adresiyle doldurur; Down yok ([phase-14](phases/phase-14-ui-redesign.md)). Yazdığı kolonu `0013` sildi: `make demo-seed` ve ilk kurulum artık çalıştırmaz, kapakları `0012` taşır |
| `0010` | `library_entries` (Faz 15): `PRIMARY KEY (user_id, book_id)`, dört durum `TEXT + CHECK`; Up sonunda `tr-x-icu` koruması (kural yoksa Deploy migration'da durur) ([phase-15](phases/phase-15-personal-library.md)) |
| `0011` | `editions` (Faz 16): kitap başına tek varsayılanı garanti eden kısmi benzersiz index, her kitaba bir varsayılan baskı açan taşıma, **`book_view`** (kitap + varsayılan baskının kapağı ve sayfası; kitap okuyan her sorgu buradan) ([phase-16](phases/phase-16-works-editions.md)) |
| `0012` | demo baskılar (veri): varsayılan baskıların dili / yayınevi / çevirmeni / yılı (Open Library) + 8 özgün dil baskısı; Down yok, `make demo-seed` ve ilk kurulum seed'den sonra yeniden çalıştırır |
| `0013` | daraltma (Faz 16): varsayılan baskısı olmayan kitaba eski kolonlardan baskı açar, sonra `books.cover_url` ve `books.page_count`'u siler; Down kolonları varsayılan baskıdan geri doldurur |
| `0014` | arama (Faz 16b, canlı 2026-11-18): `search_fold` (Türkçe duyarlı katlama) ve `search_tsquery`, kitap başına bir satırlık `book_search` belgesi (GIN index), onu `books` ve `editions` yazımlarında güncelleyen iki tetikleyici, ilk doldurma; Up başında `und-x-icu` koruması ve `lock_timeout` ([phase-16b](phases/phase-16b-search.md)) |
| `0015` | index düzeltmeleri (F28): index'siz üç yabancı anahtara index, liste index'lerinin sadeleştirilmesi; yalnız index değişir. Kural bir testte: `internal/database/schema_integration_test.go` |
| `0016` | okuma takibi, genişletme (Faz 17A-1): `library_entries`'e okuma alanları ve `version`, kapanmış okumalar için `reading_log`; kısıtsız kolonlar, CHECK'ler 0017'de ([phase-17](phases/phase-17-reading-reviews.md)) |

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
| `src/api/types.ts` | API sözleşmesinin TS tipleri (`Session`'da `refresh_token` YOK); raf tipleri ve komutu gövdesiyle birlikte tanımlayan `ShelfCommand` (Faz 17) |
| `src/api/queries.ts` | sunucu verisinin **sorgu ve değişiklik kancaları** (TanStack Query, Faz 16d) ve bütün **sorgu anahtarları** (`queryKeys`) tek yerde; kullanıcıya özel veri `["me", kullanıcı, …]` altında. Ana pakette |
| `src/api/adminQueries.ts`, `src/api/libraryQueries.ts` | yalnız tembel yüklenen parçaların kancaları (Moderasyon; Kitaplığım ve kitaplık menüsü): ayrı dosyada ki ana pakete girmesin. Raf kancaları (`useShelf`, komut, ilerleme, geçmiş; Faz 17) `libraryQueries.ts`'te: cevap rafa doğrudan yazılır, 409/404'te raf yeniden okunur |
| `src/lib/queryClient.ts` | tek `QueryClient` ve varsayılanları: `staleTime` 30 sn, okumada yeniden deneme yalnız 5xx/ağda bir kez, değişiklikte hiç |
| `src/auth/AuthContext.tsx` | oturum durumu, açılışta geri yükleme; oturum kapanınca (çıkış, başka sekme, yenileme reddi) sorgu önbelleğini boşaltır |
| `src/pages/` | Home (katalog + en iyiler), BookDetail, **Library** ("Kitaplığım", Faz 15), Auth, Suggest, **Account** ("Hesabım": parola değiştirme, Faz 13), admin/Moderation |
| `src/styles.css` | Tailwind + **tasarım token'ları** (açık/koyu tema, OKLCH); değeri değişince `node scripts/check-contrast.mjs` — [Faz 14](phases/phase-14-ui-redesign.md) |
| `src/components/ui/` | shadcn/ui yaklaşımıyla repoya kopyalanmış temel bileşenler (Button, Input, Card, Badge, Alert, Skeleton, Label, Dialog) |
| `src/components/` | `BookCover` (üretilmiş kapak, başlıktan sabit renk), `EditionStrip` + `PrimaryEditionNote` (Faz 16: "Diğer baskılar" şeridi, kapak altı baskı notu), `BookCard`, `EmptyState`, `NotFound`, `ErrorBoundary` (yakalanmayan hata tek sayfayı düşürür, uygulamayı değil; F34), `common.tsx` (ErrorBox, Rating, PageTitle), `usePaged.tsx` (`usePageView`: sayfalı sorgunun sonucunu ekrana hazırlar, boş sayfada son dolu sayfaya döner; `Pager`: 10.000 sınırında "Sonraki" kapanır), `LibraryButton` (kitap sayfasındaki kontrol; giriş yoksa bağlantı) + `ShelfControl` (Faz 17, tembel parça: tek raf kontrolü — durumdan yapılabilecek komutlar, onaylar, ilerleme çubuğu) + `ShelfForms` (ilerleme ve "Okumayı düzenle"; 409 akışı) + `ReadingHistory` (kapanmış okumalar, günleri düzelt / sil), `ReadingProgress` (ilerleme çubuğu + metin; raf kontrolü ve Kitaplığım kartları), `LibraryBadge` + `useLibraryStatuses` (kart rozeti, tek toplu istek; bir sorgu, rafa eklemek onu geçersiz kılar), `filters.tsx` (katalog ve Kitaplığım'ın ortak `Chip`, `GRID`, `useParamUpdater`; adresten okunan süzgeçleri sunucunun sınırlarıyla temizleyen `asGenre`/`asQuery`/`asOffset`), `ui/dropdown-menu.tsx` (Radix menüsü), `toast.tsx` (bildirimler), `PasswordField`, `UserMenu`, `ThemeToggle` |
| `src/lib/` | `cn()` (sınıf birleştirme), `format.ts` (Türkçe puan, dil adı, göreli zaman, baş harf), `shelf.ts` (raf metinleri: ilerleme `42 / 310 sayfa`, durum satırları, baskı adı; sayıya ek gelmeyecek biçimde), `day.ts` (takvim günü: yerel "bugün" ve `YYYY-AA-GG` gösterimi; `toISOString()` ve `new Date("…")` UTC tuzakları), `theme.ts` (açık/koyu tema), `chunkReload.ts` (yayından sonra eski parça yüklenemezse bir kez yeniden yükleme; `main.tsx`'teki `vite:preloadError` dinleyicisi) |
| `public/theme-init.js` | temayı ilk boyamadan önce uygular (CSP satır içi betiğe izin vermediği için ayrı dosya) |
| `e2e/` | Playwright senaryoları (`catalog`, `library`, `auth-flow`, `reuse`, `tabs`, `cross-origin`, `rating-comment`, `moderation`, `resilience`, `cache-headers`, `theme`) + `env.ts`, `helpers.ts` (**rate limit bütçesi tablosu**), `global-setup.ts` |
| `playwright.config.ts` | yalnız chromium, `baseURL` = `E2E_BASE_URL`, `trace: retain-on-failure`, CI'da 1 yeniden deneme |
| `nginx/` | `default.conf` (auth + **password** + **catalog** rate limit zone'ları, 429 için hata zarfı, sorgusuz `log_format bookhub`, `$api_host` map'i), `api-proxy.conf` (`Host` = `$http_host` → port korunur), `security-headers.conf` |
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
| **Trivy** | imaj açık taraması; CI'daki `images` işi, haftalık `scan.yml` ve `make scan` aynı kuralı koşar; istisnalar `.trivyignore`'da süreli |
| **Dependabot** | digest/bağımlılık güncellemelerini haftalık PR olarak getirir; güvenlik uyarıları da açık (2026-11-19) |
| **Bağımlılık incelemesi** | `actions/dependency-review-action`: PR'ın eklediği paketi açıklara karşı birleşmeden önce denetler |
| **SonarCloud** | kod kalitesi ve güvenlik analizi; otomatik analiz modunda (test kapsamı yok); zorunlu kontrol |
| **gitleaks** | sır taraması (pre-commit ve Kurallar); zorunlu araç |
| **actionlint**, **shellcheck** | iş akışı ve kabuk betiği denetimi (Kurallar); Windows'ta Docker'la |
| **OpenTelemetry** | API'nin iz ve metrik üretmesi (OTLP); yerelde Grafana + Prometheus (`make obs-up`), canlıda Grafana Cloud hazırlığı (#96) |

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
- [REVIEW_FEEDBACK.md](REVIEW_FEEDBACK.md)'deki açık bulgular: özet tablo güncel (kısmen çözülenler
  orada eğik yazıyla). F05 risk kabul edildi.
- Kullanıcı silme ucu yok (hesap yalnızca veritabanından silinebiliyor).
- Canlı izleme sınırlı: günlük uptime ve haftalık imaj taraması var; Northflank sağlık kontrolü, hata
  izleme ve canlı telemetri (#96) yok — [DEPLOYMENT.md](DEPLOYMENT.md) §5.
- Canlı veritabanının yedeği yok; şifreli yedek PR #101'de, kullanıcının anahtarını bekliyor (F23).
- Küme içi Postgres yalnız demo içindir; prod'da yönetilen veritabanı.
- Ürün tarafındaki istekler ayrı bir listede: [FEATURE_REQUESTS.md](FEATURE_REQUESTS.md); sırası
  [ROADMAP.md](ROADMAP.md)'de.
