# BookHub — Proje Haritası (neyin ne olduğu)

> **Bu dosya "NE" sorusunun cevabı:** hangi dosya ne işe yarar, bir istek nereden nereye gider,
> hangi araç niye var. Uzun aradan sonra dönünce burayı oku.
> **[STATUS.md](STATUS.md)** ise "NEREDEYİZ" sorusunun cevabı. İkisi kardeş: harita + imleç.
>
> Son güncelleme: 12 fazın tamamı bitti (2026-09-18).

---

## 1. Otuz saniyelik özet

**BookHub**, okunan kitapların kataloğu + topluluğu. Asıl amaç **öğrenme**: TS geçmişinden gelen
birinin backend teknolojilerini *neden var oldukları* üzerinden Go ile öğrenmesi. Her teknoloji
gerçek bir problem doğunca eklendi; kodun içi Türkçe "neden böyle" yorumlarıyla dolu.

- **Yığın:** Go (chi, pgx, sqlc, goose, slog) · PostgreSQL · Redis · React + Vite · NGINX ·
  Docker · Kubernetes · GitHub Actions. **ORM yok.**
- **Kapsam:** MVP (Faz 1–7) + altyapı rayı (Faz 8–12) tamam. Buluta deploy rehberi hazır, deploy
  edilmedi (hesap gerektirir).
- **Boyut:** ~6.5k satır Go (+ ~6k satır test, 151 test fonksiyonu), ~1k satır TypeScript,
  7 migration, 41 öğrenme notu.

---

## 2. Çalıştırma

```bash
cp .env.example .env            # bir kez
make app-up                     # her şey konteynerde → http://localhost:3000
make demo-seed                  # örnek katalog
make admin EMAIL=sen@ornek.com  # ilk admin (parola gizli sorulur)
make smoke                      # MVP tanımını uçtan uca doğrula
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
| `.github/workflows/` | `ci.yml` (backend/frontend/e2e), `release.yml` (yalnız tag/elle imaj yayını) |
| `deploy/k8s/` | Kubernetes manifestleri (kustomize) |

### `backend/cmd/api/` — HTTP katmanı (paket `main`)

| Dosya | Ne |
|---|---|
| `main.go` | `run() int`: config → migrate (ops.) → pgxpool → Redis (ops.) → servisler → router → graceful shutdown; `healthcheck` alt komutu |
| `router.go` | `deps` struct'ı, middleware zinciri, tüm rotalar (public / giriş / admin grupları) |
| `middleware.go` | `requireAuth` (401), `requireAdmin` (403), `rateLimit` (429), `resolveClientIP`, `requestLogger` |
| `respond.go` | `writeJSON`, hata zarfı, `writeInternalError` (request id'li log), `readJSON` (1 MiB, bilinmeyen alan = 400) |
| `validate.go` | `validator` (hataları biriktirir), alan sınırları, slug/dil/URL kuralları |
| `books.go` | public kitap uçları, public DTO, `createBookRequest`, ortak alan kontrolleri (`checkTitle`…) |
| `moderation.go` | öneri ucu + admin kitap uçları, admin DTO, durum geçişleri (method expression) |
| `community.go` | yorum + puan + top-rated uçları |
| `genres.go` | tür uçları |
| `auth.go` | register/login/refresh/logout/me |
| `*_test.go` | sahte (fake) servislerle DB'siz handler testleri; sözleşme (alan listesi) testleri; korelasyon testi |

### `backend/cmd/manage/` — operatör komutları

`migrate` (şemayı uygula) · `create-admin -email E` (parola prompt/stdin'den; argümanla ASLA).

### `backend/internal/` — iş mantığı ve veri

| Paket | Ne |
|---|---|
| `config` | env → `Config`, fail-fast doğrulama (JWT ≥32 bayt, TTL'ler, proxy CIDR'ları…) |
| `database` | pgxpool (`New`), `Migrate` (goose + advisory lock), `WithTx`, Postgres hata kodları |
| `db` | **sqlc'nin ürettiği** kod — elle düzenlenmez (`make sqlc`) |
| `books` | kitap repository'si (public okuma yalnız APPROVED), statüler, `ParseStatus` |
| `genres` | tür repository'si |
| `auth` | bcrypt, JWT (`TokenManager`), refresh token, `Service` (ilk servis katmanı) |
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

`sql/queries/*.sql` = sqlc girdisi (el yazması SQL); `migrations/embed.go` migration'ları binary'ye gömer.

### `backend/scripts/`

`mvp_smoke.sh` (MVP tanımı, 21 kontrol) · `demo_seed.sql` (örnek katalog) · `perf_seed.sql` (20k kitap / 209k oy, ayrı DB).

### `frontend/`

| Dosya | Ne |
|---|---|
| `src/api/client.ts` | fetch sarmalayıcı, token saklama, 401 → **tek uçuşlu** yenileme → tekrar |
| `src/api/types.ts` | API sözleşmesinin TS tipleri |
| `src/auth/AuthContext.tsx` | oturum durumu, açılışta geri yükleme |
| `src/pages/` | Home (katalog + en iyiler), BookDetail, Auth, Suggest, admin/Moderation |
| `nginx/` | `default.conf`, `api-proxy.conf`, `security-headers.conf` |
| `Dockerfile` | node build → nginx-unprivileged (uid 101, :8080) |

---

## 5. Kritik kararlar ve kanıtları

| Karar | Kanıt (test / ölçüm) |
|---|---|
| Görünürlük sınırı SQL'de (`GetApprovedBook`) | öneri onaya kadar public 404 (smoke) |
| Ayrı request/response DTO | `status`/`role` gönderen istek 400; iç alanlar public'e sızmıyor (sözleşme testleri) |
| JWT algoritma sabitleme | sabitleme kaldırılınca HS512 token kabul ediliyor (mutation) |
| bcrypt 72 bayt koruması | koruma kaldırılınca "72 bayt + sonek" giriş yapıyor (regresyon testi) |
| Tek kullanımlık refresh (`DELETE … RETURNING`) | 20 eşzamanlı tüketim → tam 1 kazanan |
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

---

## 7. Bilinen borçlar

- Refresh token `localStorage`'da → httpOnly + `SameSite=Strict` çerez daha güçlü (backend değişikliği ister).
- Token "aile" iptali (eski refresh token tekrar gelirse tüm aileyi düşür) yok.
- Arayüzün giriş gerektiren akışları tarayıcıda otomatik test edilmedi (API seviyesinde smoke ile kapsanıyor).
- Buluta deploy yapılmadı — [DEPLOYMENT.md](DEPLOYMENT.md).
- Küme içi Postgres yalnız demo içindir; prod'da yönetilen veritabanı.
