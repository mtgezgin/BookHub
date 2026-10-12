# BookHub — Dağıtım (Deploy)

> **Durum (2026-10-05):** Canlıda — https://p01--web--4x2qb6s8vg5j.code.run
> Yayın akışı: `main`'e push → CI → Deploy iş akışı (gerekirse migration + `release` dalı) →
> Northflank. Süreç: [PROCESS.md](PROCESS.md) §5. Mimari kararlar: [PROJECT_BRIEF.md](PROJECT_BRIEF.md) §7, §13.

## 1. Mimari

```
tarayıcı ──HTTPS──▶ Northflank ağ geçidi (TLS burada çözülür)
                        │ HTTP
                        ▼
          web: NGINX + React  :8080  (public)
                        │ HTTP, proje içi özel ağ: api:8080
                        ▼
          api: Go API         :8080  (private; servis adı tam olarak "api")
                 │                      │
     DATABASE_URL (TLS)          REDIS_URL (rediss://)
                 ▼                      ▼
     Supabase Postgres            Upstash Redis
     (Frankfurt, session pooler)  (Frankfurt)
```

- **Northflank** (ücretsiz Developer Sandbox): iki servis, uyumuyor, servisler birbirine
  `[servis-adı]:[port]` ile ulaşıyor. Bu yüzden NGINX'teki `proxy_pass http://api:8080;` satırı
  compose, kind ve canlıda aynı. Bölge: ücretsiz planda tek seçenek olan Europe-West (London);
  veriye (Frankfurt) gidiş-dönüş ~15 ms.
- **Supabase:** Data API kapalı, otomatik RLS kapalı. Uygulama **Session pooler** adresine
  bağlanıyor (IPv4; prepared statement ve oturum kilidi destekli — migration'ların advisory
  lock'u için gerekli). Doğrudan bağlantı yalnız IPv6; transaction pooler prepared statement
  desteklemiyor.
- **Upstash:** TCP adresi (`rediss://…:6379`), eviction açık. HTTP/REST adresi go-redis ile
  çalışmaz.
- Neden Northflank: Fly.io'nun yeni hesaplara ücretsiz katmanı yok; Render'ın ücretsiz
  servisleri 15 dk'da uyuyor ve özel ağdan istek alamıyor; Cloudflare Workers Go/Docker
  çalıştırmıyor ([öğrenme notu #7](learning-notes/infra-and-devops.md)). Koyeb'in ücretsiz
  katmanında tek servis var (bize iki lazım); Oracle Always Free VM boşta kalan makineyi geri
  alabiliyor ve sunucu bakımını (SSH, güvenlik duvarı, TLS) bize bırakıyor.
- Upstash'te neden TCP (`rediss://`) adresi: [öğrenme notu #6](learning-notes/infra-and-devops.md).

## 2. Env değişkenleri (canlı)

Northflank → `api` → Environment → **Runtime variables** (Build arguments DEĞİL: oradakiler
imaja girer). `web` servisinin env değişkeni yok.

| Değişken | Canlı değeri | Not |
|---|---|---|
| `DATABASE_URL` | Supabase Session pooler adresi + `?sslmode=require` | Keychain'de `bookhub-prod-dsn`; panele `security … \| tr -d '\n' \| pbcopy` ile |
| `REDIS_URL` | Upstash `rediss://…` | Boş olsaydı cache kapalı, rate limit bellek içi olurdu |
| `JWT_SECRET` | `openssl rand -base64 48` çıktısı | Başka yerde saklanmaz. Yalnız access token'ları imzalar: değişince verilmiş access token'lar geçersizleşir ama oturumlar refresh çereziyle kendiliğinden yenilenir (refresh token'lar secret'tan bağımsız). Herkesi çıkarmak: aşağıdaki not |
| `AUTO_MIGRATE` | `false` | Migration'ı Deploy iş akışı uygular; elle yalnız acil durumda ([CLAUDE.md](../CLAUDE.md)) |
| `LOG_FORMAT` | `json` | Northflank loglarında alan alan okunur |
| `PORT` | `8080` | Servisin port ayarıyla aynı |
| `COOKIE_SECURE` | (varsayılan `true`) | Refresh çerezi yalnız HTTPS'te gider → canlıda HTTPS şart |
| `COOKIE_NAME` | (varsayılan `bookhub_refresh`) | Aynı host'ta birden çok yığın varsa her birine ayrı ad |
| `TRUSTED_PROXIES` | (boş) | Bilinen sınır: aşağıda §5 |
| `JWT_ACCESS_TTL` · `JWT_REFRESH_TTL` · `JWT_SESSION_MAX_AGE` | varsayılanlar (15m · 168h · 720h) | `JWT_SESSION_MAX_AGE`, `JWT_REFRESH_TTL`'den kısa olamaz (açılışta doğrulanır) |
| `RATING_MIN_VOTES` · `AUTH_RATE_LIMIT_PER_MIN` | varsayılanlar | Ağırlıklı puanın `m`'i · IP başına dakikalık kimlik isteği kotası |

Deploy iş akışı için GitHub secret'ı: `PROD_DATABASE_URL` (yukarıdaki `DATABASE_URL` ile aynı
değer). Yalnız yeni bir migration dosyası geldiğinde kullanılır. Repo seviyesinde bir sır
(ücretsiz özel repoda Environments yok): yazma yetkisi olan herkes, herhangi bir dalda bu sırrı
referans eden bir iş akışıyla okuyabilir. Bu yüzden yalnız deploy.yml'in migration adımında
kullanılır ([CLAUDE.md](../CLAUDE.md)).

**Herkesi anında çıkarmak** (ör. bir sızıntıda) iki adımdır: aileleri iptal et ve `JWT_SECRET`'ı
döndür. Yalnız aileleri iptal etmek verilmiş access token'ları ≤ 15 dk daha yaşatır; yalnız
secret'ı döndürmek ise kimseyi çıkarmaz.

```sql
UPDATE refresh_token_families SET revoked_at = now() WHERE revoked_at IS NULL;
```

## 3. Yayın akışı (her değişiklikte)

1. Dal → PR → CI yeşil → `main`'e merge.
2. `main`'de CI yeşil bitince **Deploy** iş akışı
   ([.github/workflows/deploy.yml](../.github/workflows/deploy.yml)):
   - `plan` işi (salt-okunur): canlıya çıkmış bir migration dosyası değişmiş ya da silinmişse
     durur; **yeni** migration dosyası varsa `go -C backend run ./cmd/manage migrate` ile canlı
     veritabanına uygular (yeni kod çıkmadan **önce** → migration'lar geriye uyumlu olmalı),
   - `release` işi (yazma yetkili, repo kodu çalıştırmaz): `release` dalını CI'ın test ettiği
     commit'e ilerletir (yalnız fast-forward).
3. Northflank `release`'i derler ve yayınlar (CI ve CD açık).
4. Yayından sonra doğrulama: [PROCESS.md](PROCESS.md) §6.

Geri dönüş ve yeniden deneme: [PROCESS.md](PROCESS.md) §5.

## 4. Kurulum kaydı (bir kez yapıldı)

- **Supabase:** proje (Central EU — Frankfurt), harf+rakam parola; Data API ve otomatik RLS
  kapalı. GitHub entegrasyonu bağlı bırakıldı → repoya **asla `supabase/` klasörü eklenmez**.
- **Upstash:** Redis (Frankfurt), eviction açık, Free plan.
- **Northflank:** proje `bookhub` (Europe-West), iki Combined service:

  | Servis | Dockerfile / context | Port | Compute |
  |---|---|---|---|
  | `api` | `/backend/Dockerfile` · `/backend` | 8080 HTTP, private | nf-compute-10 |
  | `web` | `/frontend/Dockerfile` · `/frontend` | 8080 HTTP, public | nf-compute-10 |

- **İlk veritabanı kurulumu** (Mac'ten, repo kökünde):
  ```bash
  export DATABASE_URL="$(security find-generic-password -s bookhub-prod-dsn -w)"
  go -C backend run ./cmd/manage migrate
  go -C backend run ./cmd/manage create-admin -email <e-posta>     # parola gizli sorulur
  docker run --rm -i postgres:16 psql "$DATABASE_URL" < backend/scripts/demo_seed.sql
  unset DATABASE_URL
  ```
  Bugün elle migration gerekirse (yalnız acil durumda, [CLAUDE.md](../CLAUDE.md)): önce
  `git fetch origin && git switch --detach origin/release`, sonra yalnız `manage migrate` satırı.
- **Süreç kurulumu** (`release` dalı, etiketler, secret, pano): [PROCESS.md](PROCESS.md) §9.

Northflank imajı repodan kendisi derliyor; ayrı bir imaj kayıt defteri (ghcr) kullanılmıyor.
Eski `release.yml` (etiketle ghcr'ye imaj yayını) hiç çalışmadığı için 2026-10-12'de kaldırıldı.

## 5. Bilinen sınırlar

- **HTTPS şart.** Refresh token `HttpOnly` + `Secure` + `SameSite=Strict` bir çerezte; Northflank
  HTTPS'i sağlıyor. `COOKIE_SECURE=false` yalnız lokal denemeler içindir.
- **IP başına sınır herkes için ortak.** `TRUSTED_PROXIES` boş: API, NGINX'in iç adresini
  istemci sanıyor; NGINX de Northflank ağ geçidinin adresini görüyor. Giriş denemesi sınırı tüm
  ziyaretçiler arasında paylaşılıyor ([REVIEW_FEEDBACK](REVIEW_FEEDBACK.md) F04 ile ilgili).
- **Supabase ücretsiz projesi** bir hafta hareketsiz kalınca duruyor ve kendiliğinden uyanmıyor;
  panelden *Restore*.
- **Küçük makine** (`nf-compute-10`, 0.1 vCPU): giriş ve kayıt 3–4 sn sürüyor (2026-10-12 ölçümü:
  ağ ~0,45 sn + bcrypt cost 12 ~3,3 sn). Kodla düzelmez: cost 12'nin altına inilmez (CLAUDE.md).
  Tek çare `api` servisinin planı: yerel ölçümle `nf-compute-20` (0.2 vCPU) ≈ 2 sn, `nf-compute-50`
  (0.5 vCPU) ≈ 1 sn. Şimdilik plan değişmiyor (kullanıcı kararı, #32).
- **API veritabanına sahip (owner) kullanıcıyla bağlanıyor** — kabul edilmiş risk
  ([REVIEW_FEEDBACK](REVIEW_FEEDBACK.md) F05).
- **Veritabanı TLS bağlantısı sunucu kimliğini doğrulamıyor.** `sslmode=require` şifreler ama
  sertifikayı ve host adını doğrulamaz (pgx). Sertleştirme: Supabase'in CA sertifikasını repoya
  ve imaja koyup `PGSSLROOTCERT` ile vermek, sonra DSN'i `sslmode=verify-full` yapmak
  ([öğrenme notu #6](learning-notes/infra-and-devops.md)).
- **İzleme yok:** Northflank'ta sağlık kontrolü, uptime izleyici ve hata izleme (Sentry) henüz
  kurulmadı; sıradaki adım.
- Access token'lar stateless: çıkış, parola ve rol değişikliği refresh tarafını anında keser ama
  verilmiş access token'lar `JWT_ACCESS_TTL` (15 dk) dolana kadar geçerli kalır.
