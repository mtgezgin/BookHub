# BookHub — Dağıtım (Deploy)

> **Durum (2026-11-19):** Canlıda (2026-10-05'ten beri) — https://p01--web--4x2qb6s8vg5j.code.run
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
  desteklemiyor. Panelde bilinen iki gürültü (2026-11-23'te loglardan bakıldı): ana sayfadaki Postgres
  kartı saatte ~110 ERROR gösterir, hepsi `schema "pg_pgrst_no_exposed_schemas" does not exist` —
  Data API kapalıyken Supabase'in kendi PostgREST'i yazıyor, uygulamadan değil. Security Advisor'da 7
  WARN "Function Search Path Mutable" bizim fonksiyonlarımız ([ROADMAP](ROADMAP.md) teknik hat).
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
| `DATABASE_URL` | Supabase Session pooler adresi + `?sslmode=require` | Okunabilir kopyası Bitwarden'da (`BookHub canlı veritabanı adresi`); panelde göz simgesiyle de görülür |
| `REDIS_URL` | Upstash `rediss://…` | Boş olsaydı cache kapalı, rate limit bellek içi olurdu. Redis'e ulaşılamazsa API yine açılır ve çalışır (uyarı loglar; cache'siz, API'nin rate limit'i açık); bağlantı geri gelince kendiliğinden kullanılır |
| `JWT_SECRET` | `openssl rand -base64 48` çıktısı | Başka yerde saklanmaz. Yalnız access token'ları imzalar: değişince verilmiş access token'lar geçersizleşir ama oturumlar refresh çereziyle kendiliğinden yenilenir (refresh token'lar secret'tan bağımsız). Herkesi çıkarmak: aşağıdaki not |
| `AUTO_MIGRATE` | `false` | Migration'ı Deploy iş akışı uygular; elle yalnız acil durumda ([CLAUDE.md](../CLAUDE.md)) |
| `LOG_FORMAT` | `json` | Northflank loglarında alan alan okunur |
| `PORT` | `8080` | Servisin port ayarıyla aynı |
| `COOKIE_SECURE` | (varsayılan `true`) | Refresh çerezi yalnız HTTPS'te gider → canlıda HTTPS şart |
| `COOKIE_NAME` | (varsayılan `bookhub_refresh`) | Aynı host'ta birden çok yığın varsa her birine ayrı ad |
| `TRUSTED_PROXIES` | (boş) | Bilinen sınır: aşağıda §5 |
| `JWT_ACCESS_TTL` · `JWT_REFRESH_TTL` · `JWT_SESSION_MAX_AGE` | varsayılanlar (15m · 168h · 720h) | `JWT_SESSION_MAX_AGE`, `JWT_REFRESH_TTL`'den kısa olamaz (açılışta doğrulanır) |
| `DB_MAX_CONNS` | (varsayılan `6`) | Havuzun en çok bağlantısı. 2 × değer + 1 (yayın sırasında iki örnek + migration), Supabase oturum havuzunun sınırını (varsayılan 15) aşmamalı. Adrese `pool_max_conns` yazılmaz |
| `OTEL_EXPORTER_OTLP_ENDPOINT` · `OTEL_EXPORTER_OTLP_HEADERS` (**sır**) · `OTEL_EXPORTER_OTLP_PROTOCOL` · `OTEL_TRACES_SAMPLER` · `OTEL_TRACES_SAMPLER_ARG` · `OTEL_RESOURCE_ATTRIBUTES` | Kurulum: aşağıda §2a. Hiçbiri yokken telemetri **kapalı** | OpenTelemetry iz ve metrikleri Grafana Cloud'a gider ([#96](https://github.com/mtgezgin/BookHub/issues/96)). İlk değişkeni uygulama da doğrular; diğerlerini SDK okur |
| `RATING_MIN_VOTES` · `AUTH_RATE_LIMIT_PER_MIN` | varsayılanlar | Ağırlıklı puanın `m`'i · IP başına dakikalık kimlik isteği kotası |

Deploy iş akışı için GitHub secret'ı: `PROD_DATABASE_URL` (yukarıdaki `DATABASE_URL` ile aynı
değer). Yalnız yeni bir migration dosyası geldiğinde kullanılır. Repo seviyesinde bir sır: yazma
yetkisi olan herkes, herhangi bir dalda bu sırrı referans eden bir iş akışıyla okuyabilir (fork'tan
gelen PR'lar repo sırlarını görmez). Bu yüzden yalnız deploy.yml'in migration adımında kullanılır
([CLAUDE.md](../CLAUDE.md)). Repo 2026-11-09'da public olduğundan GitHub **Environments** artık
ücretsiz: sırrı yalnız `main`'den koşan işlere açan bir `production` ortamına taşımak mümkün
(henüz yapılmadı; [ROADMAP](ROADMAP.md) teknik hat).

**Herkesi anında çıkarmak** (ör. bir sızıntıda) iki adımdır: aileleri iptal et ve `JWT_SECRET`'ı
döndür. Yalnız aileleri iptal etmek verilmiş access token'ları ≤ 15 dk daha yaşatır; yalnız
secret'ı döndürmek ise kimseyi çıkarmaz.

```sql
UPDATE refresh_token_families SET revoked_at = now() WHERE revoked_at IS NULL;
```

### 2a. Telemetri: Grafana Cloud'a doğrudan OTLP ([#96](https://github.com/mtgezgin/BookHub/issues/96))

Canlıda collector çalıştırılamaz (iki servis sınırı); API veriyi OTLP/HTTP ile doğrudan Grafana Cloud'un
ağ geçidine gönderir. Grafana bu yolu üretim dışı kullanım için destekler (yeniden deneme ve zenginleştirme
sınırlı); öğrenme projesi için kabul edildi.

**Bir kez (kullanıcı; sırlar asistana verilmez):**

1. grafana.com'da ücretsiz hesap aç; yığın (stack) bölgesi olarak API'ye yakın bir AB bölgesi seç. Kart
   istenmez. Hesap 14 günlük denemeyle açılır, sonra ücretsiz plana geçer (10 bin seri, ayda 50 GB iz,
   14 gün saklama).
2. Grafana Cloud portalında yığının **OpenTelemetry** kutusunda *Configure*: bir erişim token'ı üret.
   Sayfa üç değer verir: `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL` ve
   `OTEL_EXPORTER_OTLP_HEADERS`. (Panelin adları değişmiş olabilir; aranan şey bu üç değişkendir.)
3. Northflank → `api` servisi → ortam değişkenleri. **Yalnız `api`'ye**; `web`'e eklenmez:

   | Değişken | Değer |
   |---|---|
   | `OTEL_EXPORTER_OTLP_ENDPOINT` | Grafana'nın verdiği adres (`https://otlp-gateway-….grafana.net/otlp`) |
   | `OTEL_EXPORTER_OTLP_PROTOCOL` | Grafana'nın verdiği değer |
   | `OTEL_EXPORTER_OTLP_HEADERS` | Grafana'nın verdiği değer (`Authorization=Basic …`). **Gizli değişken** olarak gir |
   | `OTEL_TRACES_SAMPLER` | `parentbased_traceidratio` |
   | `OTEL_TRACES_SAMPLER_ARG` | `1.0` (trafik az: bütün istekler izlenir; trafik artınca `0.1`) |
   | `OTEL_RESOURCE_ATTRIBUTES` | `deployment.environment=production` |

   Kaydedince Northflank `api`'yi yeniden başlatır. Loglarda `telemetri açık` satırı görünmeli.
4. Grafana Cloud → Dashboards → *New* → *Import* → [bookhub-api.json](../ops/grafana/dashboards/bookhub-api.json)
   dosyasını yükle; "Veri kaynağı" olarak yığının Prometheus'unu (`grafanacloud-…-prom`) seç.

**Doğrulama:** sitede birkaç sayfa gez → Grafana → Explore → Tempo → `service.name = bookhub-api` izleri;
pano veri gösteriyor; bir izin özniteliklerinde sorgu dizesi, e-posta, token yok. Northflank'ta `api`'nin
bellek ve işlemci grafiğine önce/sonra bak (issue'ya yaz).

**Kapatmak:** `OTEL_EXPORTER_OTLP_ENDPOINT`'i sil; `api` yeniden başlar ve hiçbir şey gönderilmez. Kod
değişikliği gerekmez.

**Sürüm etiketi:** imajda git bilgisi olmadığı için `service.version` "bilinmiyor" görünür. Northflank'ın
commit'i bir değişken olarak verip vermediği **doğrulanmadı**; verirse `OTEL_RESOURCE_ATTRIBUTES`'a eklenir.

### 2b. Yedek ([#100](https://github.com/mtgezgin/BookHub/issues/100); REVIEW_FEEDBACK F23)

Supabase'in ücretsiz planında otomatik yedek yok ve yedek indirilemiyor. Yedeği biz alırız:

| Ne zaman | Nerede | Sır |
|---|---|---|
| Her pazartesi 03:17 UTC ve elle (Actions → Yedek → *Run workflow*) | [backup.yml](../.github/workflows/backup.yml) | `BACKUP_DATABASE_URL` (`backup` ortamının sırrı; ortam yalnız `main`'e açık) |
| Her migration'dan hemen önce | [deploy.yml](../.github/workflows/deploy.yml) migration adımı | `PROD_DATABASE_URL` (aynı adım) |

Döküm `pg_dump` ile alınır (yalnız `public` şeması: uygulamanın bütün tabloları ve `goose_db_version`),
aynı boruda [age](https://github.com/FiloSottile/age) ile **repodaki açık anahtara** şifrelenir
([ops/backup/recipients.txt](../ops/backup/recipients.txt)) ve artefakt olarak **90 gün** saklanır.
Şifresiz döküm hiçbir anda diske ya da loga yazılmaz. Dökümü açan **gizli anahtar** yalnız proje
sahibinde durur. Migration öncesi döküm alınamazsa migration uygulanmaz ve yayın durur.

Her PR'da CI, yükseltilmiş deney veritabanını geçici bir anahtarla döker, çözer, boş bir veritabanına
geri yükler ve her tablonun satır sayısını karşılaştırır (*Yükseltme* işi). Yani yedeğin **geri
yüklenebildiği** her değişiklikte sınanır.

**Bir kez kurulum (kullanıcı; sırlar ve gizli anahtar asistana verilmez):**

1. `age`'i kur: macOS'ta `brew install age`, Windows'ta `winget install FiloSottile.age` (paket
   2026-11-19'da doğrulandı: sürüm 1.3.2; yedek yol [sürüm sayfası](https://github.com/FiloSottile/age/releases)).
2. Anahtar çifti üret: `age-keygen -o bookhub-yedek.key`. Komut açık anahtarı ekrana yazar
   (`Public key: age1…`).
3. `bookhub-yedek.key` dosyasının bütün içeriğini parola yöneticisine güvenli not olarak kaydet
   (Bitwarden → New → Note; Not Defteri'nden Ctrl+A, Ctrl+C), dosyayı sil, panoyu ve Win+V geçmişini
   temizle. **Bu anahtar kaybolursa yedekler açılamaz.**
4. Açık anahtarı (`age1…` satırı; gizli değildir) `ops/backup/recipients.txt`'ye yazdır (asistana
   verebilirsin).
5. Sırrı ortama gir: `gh secret set BACKUP_DATABASE_URL --env backup --repo mtgezgin/BookHub` ve değer
   olarak `PROD_DATABASE_URL` ile **aynı** session pooler adresini yapıştır (Bitwarden'dan ya da
   Northflank → `api` → Environment → `DATABASE_URL`).
6. PR birleştikten sonra Actions → **Yedek** → *Run workflow*; iş yeşil bitmeli ve bir
   `bookhub-yedek-…` artefaktı görünmeli.

Adım 1–5 2026-11-23'te yapıldı (Windows; gizli anahtar ve veritabanı adresi kullanıcının Bitwarden
kasasında). Açık anahtar, onunla şifrelenen bir deneme metninin aynı dosyadaki gizli anahtarla açılmasıyla
doğrulandı.

**Geri yükleme provası (kurulumdan sonra bir kez, yerelde):**

```bash
# Artefaktı indir ve aç (zip içinde .dump.age dosyası):
gh run download <run-id> -n bookhub-yedek-<run-id> -D /tmp/yedek
# Gizli anahtarı parola yöneticisinden geçici bir dosyaya koy (iş bitince sil):
#   /tmp/yedek.key
# Boş bir yerel veritabanına geri yükle ve satır sayılarına bak:
docker exec bookhub-postgres psql -U bookhub -d postgres -c "CREATE DATABASE bookhub_restore_check"
TARGET_URL=postgres://bookhub:bookhub_dev@postgres:5432/bookhub_restore_check \
  PG_DOCKER_ARGS="--network bookhub_default" MSYS_NO_PATHCONV=1 \
  scripts/backup.sh restore /tmp/yedek/bookhub-….dump.age /tmp/yedek.key
rm /tmp/yedek.key
```

**Felakette:** canlı veritabanının üzerine yüklenmez. Yeni bir Supabase projesi açılır, yedek oraya
yüklenir (`TARGET_URL` = yeni projenin session pooler adresi), sonra Northflank'taki `DATABASE_URL`,
GitHub'daki `PROD_DATABASE_URL` ve `BACKUP_DATABASE_URL` yeni adrese çevrilir. Redis'te kalıcı veri yok.

**Neler yedekte yok:** Supabase'in kendi şemaları (auth, storage; kullanılmıyor). Oturumlar yedekte
vardır ama geri yükleme sonrası herkesin yeniden girmesi beklenir.

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

- **İlk veritabanı kurulumu** (repo kökünde, Git Bash; adres Bitwarden'dan yapıştırılır, ekranda görünmez):
  ```bash
  read -rs DATABASE_URL && export DATABASE_URL
  go -C backend run ./cmd/manage migrate
  go -C backend run ./cmd/manage create-admin -email <e-posta>     # parola gizli sorulur
  # Bu adresle bir hesap ZATEN varsa komut durur (o hesabı açan kişi kendi parolasıyla admin olmasın
  # diye). Hesabı bilerek devralmak için -promote ekle: parolası girdiğin parola olur, açık
  # oturumları kapanır, rolü ADMIN olur.
  docker run --rm -i postgres:16 psql "$DATABASE_URL" < backend/scripts/demo_seed.sql
  # Kapaklar ve baskılar (Faz 16): migration 0012 seed'den ÖNCE boş tabloya uygulandı; seed'den sonra
  # yeniden çalıştır (seed varsayılan baskıları açar, 0012 kapak ve bilgileriyle doldurur). 0009 artık
  # çalıştırılmaz: yazdığı books.cover_url kolonunu 0013 sildi.
  docker run --rm -i postgres:16 psql "$DATABASE_URL" -v ON_ERROR_STOP=1 < backend/migrations/0012_demo_editions.sql
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
  Katalog listesi ve araması (`GET /api/books`) ayrıdır: NGINX orada ağ geçidinin yazdığı
  `X-Envoy-External-Address` başlığıyla **ziyaretçi başına** sınırlar (10 istek/sn, anında 30). Başlığın
  canlıda geldiği yayından sonra doğrulanacak ([Faz 16b](phases/phase-16b-search.md)).
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
- **İzleme sınırlı:** günde bir dışarıdan erişilebilirlik kontrolü var
  ([uptime.yml](../.github/workflows/uptime.yml), #63: `/healthz`, `/health`, `/api/books`) ve
  haftalık imaj taraması ([scan.yml](../.github/workflows/scan.yml), #48); ikisi de kırmızı
  koşuda GitHub e-postasıyla bildirir. Northflank'ta sağlık kontrolü ve hata izleme henüz yok
  (sağlık kontrolü panelde kurulacak: `api` için `/health`, `web` için `/healthz`, ikisi de port 8080).
  Uptime'ın `/api/books` isteği Supabase'in hareketsizlik duraklamasını da önlemeli.
- **Yedek yok.** Supabase'in ücretsiz planında otomatik yedek yok. Şifreli yedek (haftalık ve her
  migration'dan önce) PR #101'de, kullanıcının anahtarını bekliyor ([#100](https://github.com/mtgezgin/BookHub/issues/100)).
- **Web imajında süreli Trivy istisnası:** temel imaj nginx 1.30.6'yı sabitliyor; altı açığın koşulu
  (mail proxy, HTTP/2, HTTP/3, `listen … ssl`, değişkenli proxy adı + SSI/mirror) bizim yapılandırmada
  yok. 2026-12-19'a kadar; düzeltilmiş temel imaj gelince silinir ([#106](https://github.com/mtgezgin/BookHub/issues/106)).
- Access token'lar stateless: çıkış, parola ve rol değişikliği refresh tarafını anında keser ama
  verilmiş access token'lar `JWT_ACCESS_TTL` (15 dk) dolana kadar geçerli kalır.
