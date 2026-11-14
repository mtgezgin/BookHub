# 📚 BookHub

**Canlı:** https://p01--web--4x2qb6s8vg5j.code.run

> **Demo:** gerçek kullanıcıya açık bir hizmet değil. Kayıt olursan başka bir yerde kullandığın
> parolayı kullanma; e-posta adresi doğrulanmaz ve veriler her an silinebilir.

Okunan kitapların kataloğu ve topluluğu: kullanıcılar kitap önerir, admin onaylar; herkes yorum
yapar ve 1–10 puan verir; "en iyiler" listesi basit ortalamayla değil **ağırlıklı (Bayes) puanla**
sıralanır.

Asıl amaç ürün değil **öğrenme**: TypeScript geçmişinden gelen biri için backend teknolojilerini
*neden var oldukları* üzerinden, Go ile, baştan sona bir üretim yığını kurarak öğrenmek. Kod
içindeki Türkçe yorumlar "neden böyle"yi anlatır; her faz bir kavram öğretir.

## Özellikler

- **Katalog:** arama (başlık/yazar), tür filtresi, sayfalama; kitap detayı, tür etiketleri.
- **Kimlik:** kayıt/giriş, JWT (HS256, algoritma sabitli) + tek kullanımlık, döndürülen refresh
  token **httpOnly çerezte** (`SameSite=Strict`, `Secure`); token **ailesi** + yeniden kullanım
  (reuse) tespiti ve mutlak oturum tavanı; parola değişimi tüm cihazları düşürür; USER/ADMIN
  rolleri; bcrypt (cost 12); zamanlama eşitlemeli giriş.
- **Moderasyon:** öneri → PENDING → onay/red/arşiv (durum makinesi), her karar aynı transaction'da
  append-only denetim kaydına.
- **Topluluk:** yorumlar (yalnız admin siler, yumuşak silme), 1–10 puan (kullanıcı başına bir,
  `ON CONFLICT`), ortalama/sayı her yazımda kaynaktan türetilir.
- **Sıralama:** `WR = v/(v+m)·R + m/(v+m)·C` sorgu anında; Redis cache (~8.8× verim).
- **Koruma:** IP başına rate limit (Redis ile çok kopyada paylaşılan), güvenilen proxy'den gerçek
  IP, cross-origin yazma isteklerinin reddi (CSRF), CSP ve güvenlik başlıkları, root olmayan
  konteynerler, imaj açık taraması (Trivy).

## Yığın

| Katman | Teknoloji |
|---|---|
| API | Go 1.27 · chi · pgx/v5 · sqlc (ORM yok) · goose · slog |
| Veri | PostgreSQL 16 · Redis 7 |
| Web | React 19 · TypeScript · Vite · react-router |
| Altyapı | Docker (distroless) · Docker Compose · NGINX · Kubernetes (kind) · GitHub Actions |

```
tarayıcı ──▶ NGINX (statik frontend, /api proxy, CSP) ──▶ Go API (×2) ──▶ PostgreSQL
                                                              └──────────▶ Redis (cache, rate limit)
```

## Hızlı başlangıç

Gerekenler: Docker Desktop, Go 1.27+, Node 26+ (yalnız frontend geliştirmek için), `make`.

```bash
cp .env.example .env            # dev değerleri; JWT_SECRET'ı prod'da mutlaka değiştir
make app-up                     # her şey konteynerde → http://localhost:3000
make demo-seed                  # örnek katalog (klasikler, puanlar, yorumlar)
make admin EMAIL=sen@ornek.com  # ilk admin (parola gizli sorulur)
make smoke                      # MVP tanımını uçtan uca doğrula (25 kontrol)
```

Geliştirme döngüsü (API host'ta, hızlı):

```bash
make up          # yalnız Postgres + Redis
make run         # API → http://localhost:8080
make web         # frontend → http://localhost:5173 (/api → 8080)
make test-db     # Go testleri (Postgres + Redis entegrasyonu dahil)
make web-test    # frontend tip kontrolü + testler
make e2e         # uçtan uca: gerçek tarayıcı (Playwright) + gerçek yığın
make scan        # yayınlanan imajları bilinen açıklara karşı tara (Trivy)
make help        # tüm hedefler
```

`make e2e` yığını kendisi kaldırır, demo veriyi basar ve bu koşuya özel bir admin açar.
Tarayıcıyı bir kereliğine indir: `cd frontend && npm run e2e:browsers`.

Git kancaları (commit ve push öncesi repo kuralları, bir kez):
`git config core.hooksPath .githooks` ve `brew install gitleaks shellcheck actionlint sqlc golangci-lint`.
gitleaks zorunlu; diğerleri yoksa o kontrol yerelde atlanır, CI'daki **Kurallar** işi yine denetler
([PROCESS](docs/PROCESS.md)).

Kubernetes (yerel kind kümesi): `make k8s-up` → `make k8s-port` → http://localhost:3001.

## API özeti

| Uç | Kim |
|---|---|
| `GET /api/books` · `/api/books/{id}` (baskılarıyla) · `/api/books/top-rated` · `/api/genres` | herkes |
| `GET /api/books/{id}/comments` · `/ratings/summary` | herkes |
| `POST /api/auth/register\|login\|refresh\|logout` · `GET /api/auth/me` | — / giriş |
| `POST /api/auth/password` (parola değiştir; diğer tüm oturumları kapatır) | giriş |
| `POST /api/books/suggestions` · `POST /api/books/{id}/comments` · `GET/PUT/DELETE /api/books/{id}/rating` | giriş |
| `/api/admin/books…` (kuyruk, approve/reject/restore, PATCH, DELETE=arşiv, audit) · `POST /api/admin/books/{id}/editions` · `PATCH/DELETE /api/admin/editions/{id}` · `POST …/editions/{id}/primary` · `/api/admin/genres…` · `DELETE /api/admin/comments/{id}` | admin |

Hata zarfı her yerde aynı: `{"error":{"code":"…","message":"…"}}`; her cevapta `X-Request-Id`.

## Dokümanlar

- [docs/STATUS.md](docs/STATUS.md) — neredeyiz (oturum devri)
- [docs/PROJECT_MAP.md](docs/PROJECT_MAP.md) — neyin ne olduğu (dosya haritası, istek yolculuğu)
- [docs/phases/](docs/phases/) — 13 fazın rehberleri · [docs/learning-notes/](docs/learning-notes/) — kavram notları
- [docs/REVIEW_FEEDBACK.md](docs/REVIEW_FEEDBACK.md) — inceleme bulguları ve çözüm kayıtları (canlı backlog)
- [docs/WHY_GO_VS_TS.md](docs/WHY_GO_VS_TS.md) — Go vs TS: ölçülmüş, dürüst karşılaştırma
- [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) — canlı kurulum (Northflank + Supabase + Upstash) ve yayın akışı
- [docs/PROCESS.md](docs/PROCESS.md) — geliştirme süreci: issue → PR → testlere bağlı yayın → doğrulama
- [CLAUDE.md](CLAUDE.md) — proje kuralları (AI oturumları ve review kapısı için)
- [docs/PROJECT_BRIEF.md](docs/PROJECT_BRIEF.md) — ürün tanımı ve yol haritası

## Lisans

[MIT](LICENSE)
