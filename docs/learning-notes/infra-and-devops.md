# Öğrenme Notları — Infra & DevOps

Docker, Redis hosting, deploy, CI/CD, reverse proxy, Kubernetes.
Format ve nasıl çalıştığı: [README.md](README.md).

## İçindekiler

1. [Multi-stage Docker imajı ve distroless: neden 38 MB?](#1-multi-stage-docker-imajı-ve-distroless-neden-38-mb)
2. [CI'da gerçek veritabanı: servis konteynerleri ve uçtan uca iş](#2-cida-gerçek-veritabanı-servis-konteynerleri-ve-uçtan-uca-iş)
3. [Reverse proxy: tek origin, gerçek istemci IP'si ve add_header tuzağı](#3-reverse-proxy-tek-origin-gerçek-istemci-ipsi-ve-add_header-tuzağı)
4. [Kubernetes'in temel parçaları (ve deploy ederken çıkan iki hata)](#4-kubernetesin-temel-parçaları-ve-deploy-ederken-çıkan-iki-hata)
5. [İmaj güvenlik açığı taraması: Trivy nedir, Docker Scout'tan neden farklı sayar?](#5-i̇maj-güvenlik-açığı-taraması-trivy-nedir-docker-scouttan-neden-farklı-sayar)
6. [TCP, HTTP ve TLS: Upstash Redis'e neden rediss:// ile bağlanıyoruz?](#6-tcp-http-ve-tls-upstash-redise-neden-rediss-ile-bağlanıyoruz)
7. [Sürekli çalışan sunucu vs edge fonksiyonu: BookHub neden Cloudflare'ın ücretsiz katmanına sığmaz?](#7-sürekli-çalışan-sunucu-vs-edge-fonksiyonu-bookhub-neden-cloudflareın-ücretsiz-katmanına-sığmaz)
8. [NGINX'in görevleri ve bir isteğin yolu: location seçimi, SPA fallback ve rate limit](#8-nginxin-görevleri-ve-bir-isteğin-yolu-location-seçimi-spa-fallback-ve-rate-limit)
9. [proxy_pass ve tek origin: NGINX isteği API'ye nasıl iletir?](#9-proxy_pass-ve-tek-origin-nginx-isteği-apiye-nasıl-iletir)
10. [Docker ağında sabit IP ve güvenilen proxy: 172.28.0.10 nereden geliyor?](#10-docker-ağında-sabit-ip-ve-güvenilen-proxy-17228010-nereden-geliyor)
11. [NGINX'i canlıda gözlemlemek: access log, API logu ve istek kimliği](#11-nginxi-canlıda-gözlemlemek-access-log-api-logu-ve-istek-kimliği)

---

## 1. Multi-stage Docker imajı ve distroless: neden 38 MB?

**🗓️ Bağlam:** Faz 8'de API'yi konteynere alırken.

**⚡ Kısa cevap:** İmajı iki aşamada kurarız: **derleme aşaması** (Go derleyicisi, kaynak kod —
yüzlerce MB) yalnızca binary üretir; **çalışma aşaması** yalnızca o binary'yi taşır. Go varsayılan
olarak tek **statik** binary ürettiği için çalışma imajında işletim sistemi araçlarına bile gerek
kalmaz (`distroless/static`: shell yok, paket yöneticisi yok).

**📖 Açıklama:**

```dockerfile
FROM golang:1.26-alpine AS build     # ① derleyici burada
COPY go.mod go.sum ./                 #   önce yalnız bağımlılık listesi →
RUN go mod download                   #   bu katman kod değişse de cache'ten gelir
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot   # ② yalnız binary + CA sertifikaları
COPY --from=build /out/api /app/
USER nonroot:nonroot
```

| Seçim | Neden |
|---|---|
| `CGO_ENABLED=0` | C kütüphanesine (libc) bağımlılık yok → libc'siz imajda çalışır |
| `-ldflags="-s -w"` | sembol/debug bilgisi atılır → daha küçük binary |
| `-trimpath` | derleyen makinenin dosya yolları binary'ye gömülmez |
| `:nonroot` | süreç uid 65532 ile çalışır; konteynerden kaçan saldırgan root bulmaz |
| shell yok | saldırganın içeride çalıştırabileceği araç yok (bedeli: `docker exec sh` de yok) |

**Healthcheck'i shell'siz yapmak:** `curl` yok, bu yüzden binary kendini yoklar:
`/app/api healthcheck` → `/health`'e GET → 0/1 çıkış kodu.

**Compose'da servis DNS'i:** konteyner içinden `localhost` konteynerin KENDİSİ. API, DB'ye
`postgres:5432` diye (servis adı = DNS adı) ve Redis'e **konteyner portuyla** (`redis:6379`) gider —
host'ta 6380'e eşlediğimiz port yalnız dışarıdan erişim için.

**Profil:** `api` servisi `profiles: ["app"]` altında. Düz `docker compose up -d` yalnız altyapıyı
kaldırır (API'yi host'ta `go run` ile hızlı geliştirirsin); `--profile app` her şeyi kaldırır.

**🔗 İlgili:**
- [backend/Dockerfile](../../backend/Dockerfile) · [docker-compose.yml](../../docker-compose.yml) · [Makefile](../../Makefile)
- [docs/WHY_GO_VS_TS.md](../WHY_GO_VS_TS.md) §11

---

## 2. CI'da gerçek veritabanı: servis konteynerleri ve uçtan uca iş

**🗓️ Bağlam:** Faz 10'da CI'ı genişletirken. Entegrasyon testlerimiz `TEST_DATABASE_URL` yoksa
atlanıyordu — yani CI şimdiye kadar eşzamanlılık testlerinin HİÇBİRİNİ koşmuyordu.

**⚡ Kısa cevap:** GitHub Actions'ta bir işe `services:` ile gerçek Postgres/Redis konteynerleri
eklenir; test adımı onlara `localhost` üzerinden bağlanır. Uçtan uca iş ise lokalde kullandığımız
**aynı** `docker-compose.yml` ile tüm stack'i kaldırıp smoke testini koşar.

**📖 Açıklama:**

| İş | Ne kanıtlar |
|---|---|
| backend | birim + gerçek DB entegrasyonu (FOR UPDATE yarışları, tek kullanımlık token, 25 eşzamanlı oy) + lint |
| frontend | tip kontrolü, vitest, üretim paketi derleniyor |
| e2e | imajlar derleniyor, konteynerler sağlıklı açılıyor, MVP tanımı uçtan uca geçiyor |

- Sır YOK: JWT secret her koşuda `openssl rand` ile üretilip `$GITHUB_ENV`'e yazılıyor.
- Yayın ayrı: `release.yml` yalnız `v*` etiketi ya da elle çalışır. İmaj yayınlamak dışarıya
  dönük bir eylem — her push'ta otomatik olmamalı.

**🔗 İlgili:** [.github/workflows/ci.yml](../../.github/workflows/ci.yml) · [release.yml](../../.github/workflows/release.yml)

---

## 3. Reverse proxy: tek origin, gerçek istemci IP'si ve add_header tuzağı

**🗓️ Bağlam:** Faz 11'de NGINX'i API'nin önüne koyarken. Rate limit'in proxy arkasında TÜM
kullanıcıları tek kovaya atacağını fark ettik.

**⚡ Kısa cevap:** NGINX hem statik frontend'i sunar hem `/api`'yi Go'ya iletir → tarayıcı için tek
origin (CORS yok). Proxy arkasında `RemoteAddr` hep proxy'dir; gerçek IP'yi proxy'nin koyduğu
başlıktan okursun — ama YALNIZCA bağlantı gerçekten o proxy'den geliyorsa.

**📖 Açıklama:**

```
istemci ──(X-Real-IP: 6.6.6.6 sahte)──▶ NGINX ──(X-Real-IP: $remote_addr ile EZİLDİ)──▶ API
istemci ──(X-Real-IP: 6.6.6.6 sahte)──────────────────────────────────────────────────▶ API
                                  API: bağlantı güvenilen proxy'den değil → başlığı yok say
```

Canlı doğrulama: sahte başlık iki yoldan da API loglarına düşmedi.

**`add_header` tuzağı (denendi):** bir `location` içinde tek bir `add_header` bile yazarsan, `server`
seviyesindeki TÜM `add_header` satırları o location için kaybolur. Deneme: üst seviyede
`X-Frame-Options`, location'da `Cache-Control` → cevapta `X-Frame-Options` sayısı 1'den 0'a düştü.
Çözüm: başlıkları bir snippet'e koyup her bloğa `include` etmek.

**Cache stratejisi:** `index.html` → `no-cache` (yeni sürüm hemen görünsün); `/assets/*` → 1 yıl,
`immutable` (dosya adı içeriğin hash'i: içerik değişince ad da değişir).

**🔗 İlgili:** [frontend/nginx/](../../frontend/nginx/) · [backend/cmd/api/middleware.go](../../backend/cmd/api/middleware.go) (`resolveClientIP`)
· Devamı (canlıya çıkıştan sonra): [#8 görevler ve location](#8-nginxin-görevleri-ve-bir-isteğin-yolu-location-seçimi-spa-fallback-ve-rate-limit),
[#9 proxy_pass](#9-proxy_pass-ve-tek-origin-nginx-isteği-apiye-nasıl-iletir),
[#10 sabit IP](#10-docker-ağında-sabit-ip-ve-güvenilen-proxy-17228010-nereden-geliyor),
[#11 loglar](#11-nginxi-canlıda-gözlemlemek-access-log-api-logu-ve-istek-kimliği)

---

## 4. Kubernetes'in temel parçaları (ve deploy ederken çıkan iki hata)

**🗓️ Bağlam:** Faz 12'de uygulamayı yerel bir kind kümesine kurarken.

**⚡ Kısa cevap:** Kubernetes'e "ne istediğini" yazarsın (manifest), o gerçeği ona benzetir.
Durumsuz API → **Deployment** (kopya sayısı serbest); kendi diski olan Postgres → **StatefulSet**;
bir kez çalışıp biten migration → **Job**; hepsinin önünde sabit DNS adı → **Service**.

**📖 Açıklama:**

| Parça | Bizde | Neden |
|---|---|---|
| Deployment | api (2 kopya), web, redis | kopyalar birbirinin yerine geçebilir |
| StatefulSet + PVC | postgres | pod yeniden doğsa da aynı diske bağlanır |
| Job | migrate | şema, kopyalar açılmadan ÖNCE tek bir kez değişir; başarısızsa deploy durur |
| Service | api, web, postgres, redis | küme içi DNS: NGINX `api:8080`'e gider |
| ConfigMap / Secret | ayarlar / JWT + DB parolası | sır dosyada değil — `make k8s-up` rastgele üretir |
| readiness / liveness | `/health` | "trafik alabilir mi?" / "kilitlendi mi, yeniden başlat" |

**Deploy ederken çıkan iki hata (gerçek):**
1. `runAsNonRoot: true` + imajda `USER nonroot` (AD) → kubelet: *"image has non-numeric user,
   cannot verify user is non-root"*, pod başlamadı. Çözüm: `USER 65532:65532` (sayısal).
2. `kind load docker-image` → *"content digest not found"*: Docker Desktop'ın containerd deposunda
   çok platformlu imajların yalnız bu makinenin platformu var. Çözüm: `docker save --platform …`
   ile tek platformlu arşiv, `kind load image-archive`.

**Sonuç:** smoke 21/21 (o günkü sayı; Faz 13'ten beri 25/25); istekler iki API kopyasına dağıldı
(28/30) — oturum JWT'de, sayaçlar Redis'te olduğu için hangi kopyanın cevap verdiği fark etmiyor.

**🔗 İlgili:** [deploy/k8s/](../../deploy/k8s/) · [Makefile](../../Makefile) (`k8s-*`)

---

## 5. İmaj güvenlik açığı taraması: Trivy nedir, Docker Scout'tan neden farklı sayar?

**🗓️ Bağlam:** Faz 13'te VS Code'daki Docker DX eklentisi `frontend/Dockerfile`'ın `FROM`
satırlarında *"The image contains 1 high vulnerability"* ve *"0C 4H 5M 0L"* uyarıları gösterdi.
"Bunlar neden giderilmedi?" diye sordum — cevap: proje imajları hiç taramamıştı (ne CI'da ne
yerelde; push kapısındaki güvenlik incelemesi kod diff'ine bakıyor, temel imajlara değil).
Asistan Trivy ile inceleyince "trivy nedir, ne işe yarıyor?" diye sordum.

**⚡ Kısa cevap:** Trivy, açık kaynaklı bir **güvenlik açığı tarayıcısı**: bir imajın (ya da dosya
sisteminin/repo'nun) içindeki her yazılım parçasını listeler ve sürümlerini bilinen açık
veritabanlarıyla karşılaştırır. `npm audit`'in "imajın içindeki HER ŞEY için" hâli.

**📖 Açıklama:**

**`npm audit` benzetmesi** — `npm audit`, `package-lock.json`'daki paketleri GitHub'ın açık
veritabanıyla karşılaştırır. Trivy aynı işi tüm katmanlarda yapar:

| Katman | `npm audit` | Trivy |
|---|---|---|
| npm paketleri | ✅ | ✅ (`node_modules`, `package-lock.json`) |
| İşletim sistemi paketleri (Alpine `apk`, Debian `dpkg`) | ❌ | ✅ |
| Go binary'sine gömülü modül listesi | ❌ | ✅ (`app/api`, `app/manage` satırları buradan) |
| Python/Ruby/Java/Rust bağımlılıkları | ❌ | ✅ |
| K8s/Terraform/Dockerfile yanlış yapılandırması | ❌ | ✅ (ayrı tarayıcı modu) |
| Sızmış sır (API anahtarı vb.) | ❌ | ✅ (`--scanners secret`) |

**Nasıl çalışır:**
1. **Envanter (SBOM)** — imajın katmanlarını açar; `/lib/apk/db/installed`, `package.json`'lar,
   Go binary'lerine gömülü modül bilgisi → "içindekiler listesi" (Software Bill of Materials).
2. **Açık veritabanı** — ilk çalıştırmada indirilir (`~/.cache/trivy`): Alpine/Debian'ın kendi
   güvenlik veritabanları, GitHub Advisory Database, NVD, Go vulnerability DB…
3. **Eşleştirme** — "kurulu sürüm, açığın etkilediği aralıkta mı?" → **CVE** numarası, **CVSS**
   şiddeti (LOW/MEDIUM/HIGH/CRITICAL), varsa **düzeltilmiş sürüm**.
   Örnek satır: `HIGH CVE-2026-73566 tar 7.5.19 -> 7.5.21 [fixed]`.

**Ne YAPMAZ (önemli):**
- **Senin kodundaki hatayı bulmaz** — yalnız "bilinen açığı olan bağımlılık var mı?" sorusuna bakar.
  Kendi kodun için `golangci-lint`/`gosec`/code review.
- **Açığın gerçekten ulaşılabilir olduğunu bilmez** — kütüphanede açık olabilir ama sen o
  fonksiyonu hiç çağırmıyor olabilirsin. Go için bunu `govulncheck` yapar (çağrı grafiğine bakar).
- **Veritabanı kadar iyidir** — bugün gördüğümüz fark: Trivy, Alpine'ın kendi güvenlik
  veritabanına göre nginx imajını temiz saydı; **Docker Scout** (Docker DX'in kullandığı) NVD
  eşleşmelerini de sayıp "1 high" dedi. Aynı imaja iki tarayıcı iki farklı sayı verebilir; ikisi
  de yalan söylemiyor, farklı kaynaklara güveniyorlar.

**Bugün kullanılan komutlar:**
```bash
trivy image --scanners vuln --severity MEDIUM,HIGH,CRITICAL bookhub-web:dev   # yerel imaj
trivy image --image-src remote --platform linux/arm64 node:24-alpine           # Docker'a ÇEKMEDEN
trivy image --ignore-unfixed --exit-code 1 --severity HIGH,CRITICAL bookhub-api:dev  # CI ayarı
```
`--ignore-unfixed` neden: dağıtımın henüz yama yayınlamadığı bir açık için yapacak bir şey yok;
CI'ı onunla kırmak yalnızca gürültü.

**Bugünkü bulgular (ve ne yapıldı):**
- `node:24-alpine` (**derleme aşaması**): 4H 5M = 9 CVE, hepsi npm'in **kendi içine gömdüğü**
  paketlerde (`tar`, `undici`, `ip-address`, `brace-expansion`). Multi-stage build'in sonunda
  node/npm yok → **yayınlanan imaja girmiyor**. `npm@latest` bile aynı sürümleri gömüyor →
  uygulanabilir upstream düzeltmesi henüz yok; lock dosyası + bütünlük hash'leriyle kurulum
  riski düşük tutuyor.
- `nginx-unprivileged:stable-alpine` (**sunum**): Trivy 0, Scout 1H. `stable-alpine-slim`
  hiç kullanmadığımız dinamik modülleri (geoip, image-filter, njs, xslt) ve kütüphanelerini
  içermiyor: Scout 0H, 91 MB → 30 MB, 35 → 21 paket. Bizim NGINX ayarıyla denendi: `nginx -t`,
  `/healthz` 200, healthcheck'in `wget`'i mevcut → **geçiş `c79707b` ile yapıldı**:
  `frontend/Dockerfile` artık `stable-alpine-slim`'i, üstelik **digest'e sabitlenmiş** hâlde
  kullanıyor (`@sha256:c2c3905b…`). İmaj değişikliği, oturum/çerez işiyle karışmasın diye ayrı bir
  `build(docker):` commit'inde tutuldu.
- Yayınlanan `bookhub-api:dev` ve `bookhub-web:dev`: Trivy 0.
- O gün eksik olan kontroller **aynı commit'te (`c79707b`) kapandı**: CI'daki `images` işi iki
  imajı da derleyip `trivy-action` (commit SHA'sıyla pinli) ile tarıyor — *düzeltmesi olan*
  HIGH/CRITICAL bir açıkta iş kırmızı; aynı kural yerelde `make scan`; temel imajlar digest'e
  sabitlendi ve `.github/dependabot.yml` (docker, gomod, npm, github-actions; haftalık)
  sabitlemenin "güncelleme" yarısını üstleniyor.

**🔗 İlgili:**
- [#1 Multi-stage imaj](#1-multi-stage-docker-imajı-ve-distroless-neden-38-mb) — derleme aşaması
  neden yayınlanmıyor
- [docs/phases/phase-13-session-hardening.md](../phases/phase-13-session-hardening.md) — Faz 13'ün
  imaj sertleştirme özeti
- [frontend/Dockerfile](../../frontend/Dockerfile) · [.github/workflows/ci.yml](../../.github/workflows/ci.yml)
- [.github/dependabot.yml](../../.github/dependabot.yml) · [Makefile](../../Makefile) (`make scan`)
- [security-and-auth.md](security-and-auth.md) — kod tarafındaki güvenlik notları

---

## 6. TCP, HTTP ve TLS: Upstash Redis'e neden rediss:// ile bağlanıyoruz?

**🗓️ Bağlam:** Canlıya çıkarken (Fly.io + Supabase + Upstash) Upstash'te Redis veritabanını
açtım; panelde iki ayrı bağlantı adresi vardı: **TCP** ve **HTTP**. Asistan "TLS'li adresi
(`rediss://`) ver" demişti, ben de TLS'i üçüncü bir seçenek sandım ve "ikisi de uygun mu, yoksa
TLS olanı mı vermeliyim?" diye sordum. Hemen öncesinde Supabase adresinin sonuna
`?sslmode=require` eklemiştim — aynı fikrin Postgres'teki hâli olduğunu burada fark ettim.

**⚡ Kısa cevap:** TCP olanı (`rediss://…upstash.io:6379`). TLS ayrı bir seçenek değil, bir
bağlantıyı **şifreleyen katman**: `rediss`'teki ikinci **s**, `https`'teki **s** ile aynı anlama
gelir. HTTP adresi Upstash'in REST arayüzü; bizim Go kodu (go-redis) Redis'in kendi protokolünü
konuşuyor, HTTP değil.

**📖 Açıklama:**

İkisi de altta TCP kullanır; fark, üstünde **hangi dilin** konuşulduğu:

```
      Upstash "TCP"                        Upstash "HTTP" (REST)
┌─────────────────────────────┐    ┌──────────────────────────────────┐
│ Redis protokolü (RESP)      │    │ HTTP: her komut bir istek        │
│ SET k v · GET k · INCR say  │    │ POST / gövde: ["SET","k","v"]    │
├─────────────────────────────┤    ├──────────────────────────────────┤
│ TLS  → adres: rediss://     │    │ TLS  → adres: https://           │
├─────────────────────────────┤    ├──────────────────────────────────┤
│ TCP  (kalıcı bağlantı)      │    │ TCP                              │
└─────────────────────────────┘    └──────────────────────────────────┘
```

| | TCP (`rediss://`) | HTTP (`https://` + token) |
|---|---|---|
| Konuşulan | Redis protokolü | Redis komutları HTTP isteği olarak |
| Bağlantı | Kalıcı, havuzdan tekrar kullanılır → düşük gecikme | Komut başına bir HTTP isteği |
| Kimin için | Sürekli çalışan sunucu (bizim API'miz) | Ham TCP soketi açamayan ya da bağlantı tutamayan ortamlar (bazı edge runtime'lar, her istekte doğup ölen serverless fonksiyonlar) |
| İstemci örneği | `go-redis`, `redis-cli`, `ioredis` | `@upstash/redis`, düz `fetch` |

**TLS bir katman, seçenek değil.** Aynı şifreleme katmanı HTTP'nin üstüne gelince `https`,
Redis protokolünün üstüne gelince `rediss` olur; Postgres'te bunu `sslmode=require` söyler.
Lokalde şifresiz (`redis://localhost:6380`, `sslmode=disable`) çünkü veri makineden çıkmıyor;
canlıda parola ve veri internetten geçiyor.

TLS iki iş yapar: **şifreleme** ve (istemci isterse) sunucunun **kimliğini sertifikayla
doğrulama**. `rediss://` ile go-redis ikisini de yapar (sertifikayı imajdaki CA'larla doğrular).
Postgres'teki `sslmode=require` ise yalnız şifrelemeyi zorunlu kılar; sunucu kimliğini de
doğrulamak `sslmode=verify-full` + Supabase'in CA dosyasını ister — ileride bir sertleştirme adımı.

**Bizim koddaki karşılığı:** `cache.NewRedis` → `redis.ParseURL`, yalnız `redis://` ve `rediss://`
şemalarını tanır. `REDIS_URL` doluysa ve ayrıştırılamıyor ya da PING'e cevap vermiyorsa API
**açılmaz** (`main.go`: "redis hazır değil" → çıkış kodu 1). Yani HTTP adresini (`https://…`) ya
da Upstash'e tek **s**'li `redis://` verseydik, Fly'da deploy sağlık kontrolünde düşerdi;
sebebi `fly logs`'ta görünürdü. Kod hiç değişmiyor, ortama göre yalnız `REDIS_URL` değişiyor
(12-factor).

**JS/TS'ten bakınca:** `ioredis` ↔ go-redis (TCP), `@upstash/redis` ↔ HTTP. Next.js'i Vercel Edge
gibi ham TCP açamayan bir runtime'da çalıştırsaydık HTTP'yi seçmek zorunda kalırdık. Bizim API
uzun ömürlü bir süreç olduğu için TCP doğal seçim — Node ile bir sunucu yazsaydık da aynısını
seçerdik. Yani bu bir Go avantajı değil, **çalışma ortamı** farkı.

**🔗 İlgili:**
- [backend/internal/cache/cache.go](../../backend/internal/cache/cache.go) (`NewRedis`) ·
  [backend/cmd/api/main.go](../../backend/cmd/api/main.go) (Redis yoksa / bozuksa ne olur)
- [docker-compose.yml](../../docker-compose.yml) — lokal Redis: şifresiz, yalnız `127.0.0.1`
- [docs/DEPLOYMENT.md](../DEPLOYMENT.md) — `REDIS_URL` (Upstash, `rediss://`) ve `DATABASE_URL` (`sslmode=require`)

---

## 7. Sürekli çalışan sunucu vs edge fonksiyonu: BookHub neden Cloudflare'ın ücretsiz katmanına sığmaz?

**🗓️ Bağlam:** Canlıya çıkış için ücretsiz platform ararken: Fly.io'nun yeni hesaplara ücretsiz
katmanı olmadığı çıktı, Render'ın ücretsiz servisleri 15 dk boşta kalınca uyuyor, asistan
Northflank sandbox'ı önerdi. Seçenekler çoğalınca "bu tarz basit projeleri Cloudflare'da yayına
alsak olmuyor mu?" diye sordum — statik siteleri ve küçük projeleri orada bedavaya
yayınlayanları görmüştüm.

**⚡ Kısa cevap:** Projenin yarısı uyar, yarısı uymaz. React frontend (derlenmiş statik
dosyalar) Cloudflare'a mükemmel uyar. Go API uymaz: Cloudflare'ın ücretsiz Workers'ı **istek
başına kısa süre çalışan JS/TS/WASM fonksiyonları** çalıştırır; bizim API ise **sürekli çalışan
bir sunucu süreci**. Docker imajını olduğu gibi çalıştırmak Cloudflare Containers ister — o da
ücretli plan (Workers Paid, en az 5 $/ay) ve o da boşta uykuya geçer.

**📖 Açıklama:**

| | Sürekli çalışan sunucu (BookHub API) | Edge fonksiyonu (Cloudflare Workers) |
|---|---|---|
| Ne çalışır | Herhangi bir program (Docker imajı → Go binary) | JS/TS ya da WebAssembly fonksiyonu |
| Ömrü | Açılır, günlerce yaşar | İstek gelince kısa süre; istekler arası durum tutmaz |
| Bellekte tuttuğu | Postgres bağlantı havuzu (`pgxpool`), Redis istemcisi | Kalıcı hiçbir şey; her istek sıfırdan |
| Nerede | Tek bölgede (Frankfurt), verinin yanında | Dünyadaki yüzlerce noktada, kullanıcının yanında |
| İşlemci sınırı | Makine ne veriyorsa | Ücretsiz planda **istek başına 10 ms CPU** (günde 100k istek) |

**Somut kırılma noktası — bcrypt:** Girişte parolayı bcrypt (cost 12) ile kontrol ediyoruz; kodun
kendi yorumuna göre ≈ 250 ms/hash. Bilerek yavaş: kaba kuvvet denemelerini pahalı kılmak için.
10 ms'lik sınırda istek bitmeden kesilir. Yani API'yi Workers için baştan yazsak bile ücretsiz
katmanda giriş çalışmazdı. (Ağı/veritabanını beklemek CPU sayılmaz; bcrypt ise saf CPU.)

**Cloudflare'ın iyi olduğu yer:** statik dosyalar — istekleri ücretsiz ve sınırsız, uyumaz, CDN
sayesinde her yerde hızlı. Frontend'imiz tam bu.

**Karma çözüm neden zahmetli:** frontend Cloudflare'da, API başka yerde olursa tarayıcı için iki
ayrı origin olur. Refresh çerezimiz `SameSite=Strict` ve API `CrossOriginProtection` ile başka
origin'den gelen yazma isteklerini reddediyor. Çözüm: `/api/*`'yi API'ye ileten küçük bir proxy
Worker + NGINX'teki CSP/güvenlik başlıklarının `_headers` dosyasıyla kopyası. Mümkün; ama uyumayan
bir platformda (Northflank) iki konteyneri yan yana koymak hem daha basit hem kod değişikliği
istemiyor. Cloudflare'ın ileride doğal yeri: kendi alan adını alınca sitenin önünde DNS + önbellek
+ saldırı koruması (ücretsiz).

**TS/JS açısından dürüst not:** Bu bir Go dezavantajı değil, **çalışma modeli** farkı. Baştan
Workers için yazılmış bir TS API (ör. Hono + Cloudflare D1) orada doğal çalışırdı. Bizim seçtiğimiz
model — uzun ömürlü süreç, bellekte tutulan bağlantı havuzu, istek başına yüzlerce ms CPU isteyen
bcrypt — hangi dille yazılsa yazılsın Workers'ın ücretsiz modeline aynı sebeplerle sığmazdı.

**🔗 İlgili:**
- [#6 TCP, HTTP ve TLS](#6-tcp-http-ve-tls-upstash-redise-neden-rediss-ile-bağlanıyoruz) — Upstash'in
  HTTP arayüzü de aynı sebepten var: bazı edge ortamları ham TCP bağlantısı açamıyor
- [#3 Reverse proxy](#3-reverse-proxy-tek-origin-gerçek-istemci-ipsi-ve-add_header-tuzağı) — tek origin
  ve güvenlik başlıkları NGINX'te · [frontend/nginx/](../../frontend/nginx/)
- [backend/internal/auth/password.go](../../backend/internal/auth/password.go) — `bcryptCost = 12`
- [security-and-auth.md](security-and-auth.md) — refresh çerezi ve CSRF koruması
- Cloudflare: [Workers pricing](https://developers.cloudflare.com/workers/platform/pricing/) ·
  [Containers pricing](https://developers.cloudflare.com/containers/pricing/)

---

## 8. NGINX'in görevleri ve bir isteğin yolu: location seçimi, SPA fallback ve rate limit

**🗓️ Bağlam:** 2026-10-04'te BookHub'ı Northflank'ta canlıya aldıktan hemen sonra "NGINX ne
işe yarıyor, panelde ona istek gelince ne olduğunu görebiliyor muyum?" diye sordum. Faz 11'de
NGINX'i kurmuştuk ([#3](#3-reverse-proxy-tek-origin-gerçek-istemci-ipsi-ve-add_header-tuzağı)),
ama gerçek trafik alan bir sunucuda neyin nereye gittiğini tam oturtamamıştım. Bu not "hangi
istek nereye gider" kısmı; iletmenin mekaniği [#9](#9-proxy_pass-ve-tek-origin-nginx-isteği-apiye-nasıl-iletir),
adres seçimi [#10](#10-docker-ağında-sabit-ip-ve-güvenilen-proxy-17228010-nereden-geliyor),
gözlem [#11](#11-nginxi-canlıda-gözlemlemek-access-log-api-logu-ve-istek-kimliği).

**⚡ Kısa cevap:** NGINX, `web` konteynerinde siteye gelen her isteği ilk karşılayan
"resepsiyon": yola bakıp `location` kurallarından birini seçer; dosyaysa diskten verir, `/api`
ise API'ye iletir, giriş isteklerinde önce hız sınırı uygular.

**📖 Açıklama:**

**Altı görev:**

| Görev | Ayardaki karşılığı | Ne sağlıyor |
|---|---|---|
| Statik dosya sunmak | `root /usr/share/nginx/html` | React'in derlenmiş hâli |
| `/api`'yi API'ye iletmek | `include …/api-proxy.conf` → `proxy_pass` | Tek origin, API internete kapalı (#9) |
| Güvenlik başlıkları | `security-headers.conf` | CSP, `X-Frame-Options`… (#3) |
| Giriş denemesini sınırlamak | `limit_req` | Kaba kuvvete API'den önceki ilk duvar |
| Önbellek talimatı | `/assets/` 1 yıl, `index.html` hiç | Yeni sürüm hemen görünür (#3) |
| SPA fallback | `try_files $uri /index.html` | `/books/42` gibi yolları React Router çözer |

**Ayar imaja nasıl giriyor** ([frontend/Dockerfile](../../frontend/Dockerfile)):
```dockerfile
COPY nginx/default.conf /etc/nginx/conf.d/default.conf                   # kurallar
COPY nginx/security-headers.conf nginx/api-proxy.conf /etc/nginx/snippets/ # tekrar kullanılan parçalar
COPY --from=build /src/dist /usr/share/nginx/html                          # derlenmiş React
```
İmajın kendi `nginx.conf`'u `http { include /etc/nginx/conf.d/*.conf; }` dediği için
`default.conf` açılışta kendiliğinden okunur. `snippets/` bizim adlandırmamız: birden çok
`location`'a `include` edilen parçalar — kod tekrarını önleyen bir fonksiyon gibi.

**İskelet** ([frontend/nginx/default.conf](../../frontend/nginx/default.conf), yorumsuz):
```nginx
server {
    listen 8080;                    # root olmayan kullanıcı 1024 altındaki portlara bağlanamaz
    root /usr/share/nginx/html;

    location = /healthz { return 200 "ok\n"; }                  # NGINX'in kendi sağlığı
    location = /health  { include …/api-proxy.conf; }           # arkadaki API'nin sağlığı
    location ~ ^/api/auth/(login|register)$ { limit_req zone=auth …;     include …/api-proxy.conf; }
    location = /api/auth/password           { limit_req zone=password …; include …/api-proxy.conf; }
    location /api/    { include …/api-proxy.conf; }
    location /assets/ { … 1 yıllık önbellek … }
    location /        { try_files $uri /index.html; }
}
```

**Location seçim sırası** — yukarıdan aşağı bir `if/else` DEĞİL, kendi kuralı var:
1. `=` ile **birebir eşleşme** varsa → hemen o (arama biter).
2. Yoksa önek kurallarından **en uzun eşleşen** bulunur ve akılda tutulur.
3. `~` **regex kuralları dosyadaki sırayla** denenir → **ilk eşleşen kazanır**.
4. Hiçbir regex eşleşmezse → 2. adımda akılda tutulan önek.

| İstek | Seçilen | Neden |
|---|---|---|
| `GET /` | `/` | tek eşleşen önek |
| `GET /assets/index-Cl0fhZJt.js` | `/assets/` | en uzun önek, regex eşleşmedi |
| `GET /books/42` | `/` | `try_files`: böyle bir dosya yok → `index.html` |
| `GET /api/books?limit=12` | `/api/` | regex (`login\|register`) eşleşmedi → önek |
| `POST /api/auth/login` | regex | regex, akılda tutulan önekten (`/api/`) önce gelir |
| `POST /api/auth/password` | `= /api/auth/password` | birebir eşleşme her şeyden önce |

`location` yalnız **yola** bakar; `?limit=12` gibi sorgu kısmı eşleşmeye katılmaz.

**SPA fallback:** `try_files $uri /index.html` = "`root + $uri` diye bir dosya varsa onu ver,
yoksa `index.html`'i ver." `/books/42` diskte bir dosya değil; `index.html` gelir, React Router
adres çubuğuna bakıp kitap sayfasını çizer. Bu satır olmasaydı kitap sayfasında F5'e basınca
NGINX 404 verirdi.

**Rate limit (iki ayrı kova):**
```nginx
limit_req_zone $binary_remote_addr zone=auth:10m rate=20r/m;   # IP başına dakikada 20 = 3 sn'de 1
limit_req zone=auth burst=10 nodelay;
limit_req_status 429;
```
- `zone=auth:10m` — IP sayaçlarının tutulduğu 10 MB'lık ortak bellek (~160 bin IP).
- `rate=20r/m` — kova 3 saniyede bir hak kazanır.
- `burst=10` — anlık 10 fazlalığa izin verir (toplam 11 hak); `nodelay` → bu fazlalıklar
  bekletilmeden hemen işlenir.
- Hak kalmadıysa → `429 Too Many Requests`; istek API'ye hiç ulaşmaz.
- Parola değiştirme **ayrı kova** (`password`, dakikada 10): saldırgan girişin kotasını yiyip
  kurbanı dışarıda bırakamasın, normal giriş trafiği de parola değiştirmeyi kilitlemesin.
- API'nin **kendi** sınırı ikinci katman (Redis'te, `AUTH_RATE_LIMIT_PER_MIN`).
- ⚠️ Kova IP'ye göre. Bir yük dengeleyicinin arkasında herkes aynı adresten geliyormuş gibi
  görünür → kova herkes için ortak olur (canlıdaki durum, #10).

**`/healthz` ≠ `/health`:** `/healthz` NGINX'in kendisidir, API'ye gitmez (konteynerin sağlık
kontrolü); `/health` ise NGINX'in arkadaki API'ye ilettiği istektir. Biri "kapı açık mı?",
öteki "içeride biri var mı?" sorusu.

**🔗 İlgili:**
- [#3 Reverse proxy](#3-reverse-proxy-tek-origin-gerçek-istemci-ipsi-ve-add_header-tuzağı) — güvenlik
  başlıkları, `add_header` tuzağı, önbellek stratejisi
- [frontend/nginx/default.conf](../../frontend/nginx/default.conf) ·
  [frontend/Dockerfile](../../frontend/Dockerfile) · [docs/phases/phase-11-nginx.md](../phases/phase-11-nginx.md)
- [scripts/e2e.sh](../../scripts/e2e.sh) — e2e koşusu NGINX'in auth kovasını neden her seferinde
  sıfırlıyor (11 hak, 3 sn'de 1)

---

## 9. proxy_pass ve tek origin: NGINX isteği API'ye nasıl iletir?

**🗓️ Bağlam:** [#8](#8-nginxin-görevleri-ve-bir-isteğin-yolu-location-seçimi-spa-fallback-ve-rate-limit)'deki
görev tablosunda "`/api` isteklerini API'ye iletmek → API internete kapalı kalıyor, tek origin
olduğu için CORS gerekmiyor ve `SameSite=Strict` çerez çalışıyor" satırını görünce "NGINX bunu
nasıl yapıyor? Kodda bunu anlatan bir dosya var mı? Tam nasıl çalıştığını anlamak istiyorum"
diye sordum — sihir gibi geliyordu.

**⚡ Kısa cevap:** `proxy_pass http://api:8080;` satırı NGINX'i bir **istemciye** çevirir:
tarayıcıdan aldığı isteğin aynısını özel ağdaki API'ye yollar, cevabı alıp tarayıcıya geri
verir. Tarayıcı API'yi hiç görmez; her şeyi tek bir adresten (origin) aldığını sanır.

**📖 Açıklama:**

**Problem — NGINX olmasaydı:** frontend bir adreste, API başka bir adreste dursaydı tarayıcı için
iki ayrı **origin** olurdu:
- **CORS** gerekirdi: API her cevabında "şu siteye izin veriyorum" demek zorunda kalırdı.
- `SameSite=Strict` refresh çerezi başka siteye giden isteğe **eklenmezdi** → her F5'te oturum düşerdi.
- API'nin internete açık olması gerekirdi.

**Mekanik** ([frontend/nginx/api-proxy.conf](../../frontend/nginx/api-proxy.conf)):
```nginx
proxy_pass http://api:8080;
```
1. NGINX, `api` adlı makinenin 8080 portuna bağlantı açar.
2. Gelen isteğin **aynısını** (method, yol, sorgu, gövde) oraya yollar.
3. Cevabı alır ve tarayıcıya geri verir (dönüşte güvenlik başlıklarını ekler).

**`api` adı nasıl çözülüyor:** NGINX, `proxy_pass`'teki adı **açılışta bir kez** çözer.
- Lokalde Docker'ın iç DNS'i `api` → API konteynerinin IP'si. API konteyneri yeniden yaratılınca
  IP değişir ve NGINX eski adreste kalırdı → compose'da web'in `depends_on: api: restart: true`
  satırı bu yüzden var ([docker-compose.yml](../../docker-compose.yml)).
- Kubernetes (kind) ve Northflank'ta `api` bir **servis adı** (`[servis-adı]:[port]`), adresi
  sabit. Northflank'ta API servisine tam olarak `api` adını vermemizin sebebi bu satır: böylece
  ayar hiç değişmeden canlıda da çalıştı.

**Sondaki `/` tuzağı** (tek karakter):

| Yazım | `/api/books` nereye gider | Sonuç |
|---|---|---|
| `proxy_pass http://api:8080;` | `/api/books` (olduğu gibi) | ✅ |
| `proxy_pass http://api:8080/;` | `/books` (eşleşen `/api/` kısmı silinir) | ❌ API'nin rotaları `/api` altında → 404 |

Kural: adres:port'tan sonra **yol yoksa** istek yolu aynen gider; **yol varsa** location'ın
eşleşen kısmı o yolla değiştirilir.

**Bu repoda** sonuç 404 bile olmaz: `api-proxy.conf` bir regex location'da da include ediliyor
ve NGINX regex location'da yollu `proxy_pass`'e izin vermiyor. `nginx -t`:
`"proxy_pass" cannot have URI part in location given by regular expression` → web servisi hiç
açılmaz ([CLAUDE.md](../../CLAUDE.md), Altyapı).

**İstekle birlikte gidenler:**

| Satır | Neden |
|---|---|
| `proxy_http_version 1.1` | NGINX arkadaki sunucuyla varsayılan olarak HTTP/1.0 konuşur |
| `proxy_set_header Host $api_host` | Tarayıcının Host'u **port'uyla** (`localhost:3000`). API'nin CSRF kontrolü `Origin`'i Host ile karşılaştırıyor; `$host` port'u düşürür → meşru istek 403 alırdı (denendi) |
| `proxy_set_header X-Real-IP $remote_addr` | İstemcinin adresi; istemcinin gönderdiği sahte değeri **ezer** (#10) |
| `proxy_set_header X-Forwarded-Proto $scheme` | Orijinal isteğin http mi https mi olduğu |
| `proxy_read_timeout 15s` | API'yi en fazla 15 sn bekle |
| `client_max_body_size 1m` | 1 MB'tan büyük gövde API'ye hiç ulaşmasın |

`proxy_set_header` ile değiştirilmeyen başlıklar (`Cookie`, `Authorization`, `Content-Type`…)
olduğu gibi geçer.

**Frontend tarafı:** [client.ts](../../frontend/src/api/client.ts) adresleri **göreli** yazıyor:
`fetch("/api/auth/refresh", { credentials: "same-origin" })`. Başında `https://…` yok → tarayıcı
isteği sayfayı aldığı adrese, yani NGINX'e yollar; aynı origin olduğu için çerez de eklenir.
Geliştirmede (`make web`, port 5173) aynı işi Vite'ın proxy'si yapar
([vite.config.ts](../../frontend/vite.config.ts): `/api` → `localhost:8080`) — araç farklı,
fikir aynı.

**Canlıda bir isteğin yolu:**
```
tarayıcı ──HTTPS──▶ Northflank ağ geçidi (şifreleme burada çözülür)
                        │ HTTP
                        ▼
            web: NGINX :8080 ── location /api/ ── api-proxy.conf
                        │ HTTP, özel ağ: api:8080
                        ▼
            api: Go API :8080 ──▶ Supabase / Upstash (Frankfurt)
```

**Kendin dene** (lokal yığın açıkken; çalıştırmadan önce tahmin et — hangisinde `Server:` ve
`Content-Security-Policy` başlıkları var, `/books/1` iki yerde ne döner?):
```bash
curl -s -o /dev/null -D - http://localhost:3000/api/genres   # NGINX üzerinden
curl -s -o /dev/null -D - http://localhost:8080/api/genres   # doğrudan API
curl -s -o /dev/null -w '%{http_code} %{content_type}\n' http://localhost:3000/books/1
curl -s -o /dev/null -w '%{http_code} %{content_type}\n' http://localhost:8080/books/1
```

<details>
<summary>Sonuç (2026-10-04) — tahmin etmeden açma</summary>

- NGINX üzerinden: `Server: nginx`, `Content-Security-Policy: …` ve `X-Frame-Options: DENY`
  **var**; doğrudan API'de bu üçü **yok** → bu başlıkları NGINX ekliyor.
- İkisinde de `X-Request-Id` var ve sayılar ardışık (`…-001427`, `…-001428`) → kimliği API
  üretti, NGINX dokunmadan geçirdi.
- `/books/1`: NGINX → `200 text/html` (SPA fallback, `index.html`); API → `404 application/json`
  (API'de böyle bir rota yok).
</details>

**🔗 İlgili:**
- [#8 Location seçimi](#8-nginxin-görevleri-ve-bir-isteğin-yolu-location-seçimi-spa-fallback-ve-rate-limit) ·
  [#10 Sabit IP ve güvenilen proxy](#10-docker-ağında-sabit-ip-ve-güvenilen-proxy-17228010-nereden-geliyor)
- [frontend/nginx/api-proxy.conf](../../frontend/nginx/api-proxy.conf) ·
  [frontend/src/api/client.ts](../../frontend/src/api/client.ts) · [frontend/vite.config.ts](../../frontend/vite.config.ts)
- [security-and-auth.md](security-and-auth.md) — refresh çerezi (`SameSite=Strict`) ve CSRF koruması
- [docs/PROJECT_MAP.md §3](../PROJECT_MAP.md) — isteğin API içindeki devamı (middleware zinciri)

---

## 10. Docker ağında sabit IP ve güvenilen proxy: 172.28.0.10 nereden geliyor?

**🗓️ Bağlam:** NGINX'in isteği API'ye nasıl ilettiğini ([#9](#9-proxy_pass-ve-tek-origin-nginx-isteği-apiye-nasıl-iletir))
incelerken `docker-compose.yml`'de web servisinin altındaki `ipv4_address: 172.28.0.10` satırını
seçip "buradaki IP adresi neye göre belirlendi?" diye sordum.

**⚡ Kısa cevap:** Bir standarttan gelmiyor, bilerek seçilmiş sabit bir adres. API, `X-Real-IP`
başlığına yalnız güvendiği proxy'den gelirse inanıyor (`TRUSTED_PROXIES: 172.28.0.10`); bunu
yazabilmek için NGINX'in adresi değişmemeli. Ağ aralığı (`172.28.0.0/16`) Docker'ın kendi
verdiği aralıklarla çakışmasın, `.10` da diğer konteynerlerin sırayla aldığı adreslerden uzak
dursun diye seçilmiş.

**📖 Açıklama:**

**Neden sabit:** Docker bir konteynere her oluşturulduğunda sıradaki boş adresi verir. NGINX'in
adresi değişseydi API'nin `TRUSTED_PROXIES` ayarı eskirdi: ya gerçek istemci IP'si okunamaz
(IP başına sınır herkes için ortak olur) ya da yanlış bir adrese güvenilirdi.

Bu makinedeki gerçek durum (`docker network inspect bookhub_default`):
```
subnet=172.28.0.0/16  gateway=172.28.0.1
bookhub-redis     172.28.0.2   ┐
bookhub-postgres  172.28.0.3   ├ Docker sırayla verdi
bookhub-api       172.28.0.4   ┘
bookhub-web       172.28.0.10  ← sabit (ipv4_address)
```

**`172.28.0.0/16` — ağın aralığı:**
- `172.16.0.0 – 172.31.255.255` internette yönlendirilmeyen **özel** adres aralıklarından biri
  (RFC 1918; diğerleri `10.0.0.0/8` ve `192.168.0.0/16` — ev modemleri genelde sonuncuyu kullanır).
- Docker yeni ağlara bu aralıktan **sırayla** adres verir. Bu makinede: `bridge` 172.17, `kind`
  172.18, `test-cases` 172.19, `redis-local` 172.20, `falcon` 172.21. 172.28 bu sıradan uzak;
  çakışsaydı compose ağı kuramazdı (*"Pool overlaps with other one on this address space"*).
- Sabit adres verebilmek için ağın aralığının **açıkça** yazılması gerekiyor (Docker kuralı) →
  dosyanın sonundaki `networks.default.ipam.config.subnet` bloğu.
- `/16` = ilk iki sayı sabit, son ikisi serbest (~65 bin adres); fazlası zararsız.
- Kurumsal VPN'ler de bazen 172.16/12'den adres kullanır; çakışırsa Docker ağı ile VPN rotaları
  birbirini bozabilir — aralık seçerken bilinen bir tuzak.

**`.10` — NGINX'in adresi:**
- `.0` ağın kendi adresi, `.1` gateway → kullanılamaz.
- Diğer konteynerler sırayla `.2`, `.3`, `.4` alıyor → `.10` bu sıradan uzak, bilinçli bir boşluk.
- İncelik: 8'den fazla konteyner olursa sıralı tahsis `.10`'a ulaşıp onu NGINX'ten önce kapabilir.
  Daha sağlam yol, sırayla verilen adresleri ayrı bir aralığa sıkıştırmak (`ipam.config.ip_range`);
  4 konteynerlik bu yığında gerek yok.

**Ortama göre "güvenilen proxy":**

| Ortam | `TRUSTED_PROXIES` | Neden |
|---|---|---|
| Compose | `172.28.0.10` | NGINX'in sabit adresi |
| Kubernetes (kind) | `10.244.0.0/16` | Pod'lar her başlangıçta yeni IP alır → tek adres yazılamaz, tüm pod aralığına güvenilir |
| Northflank (canlı) | boş = kimseye güvenme | API, NGINX'in iç adresini istemci sanır → IP başına sınır **herkes için ortak** (bilinen kısıt) |

`resolveClientIP` ([backend/cmd/api/middleware.go](../../backend/cmd/api/middleware.go)):
bağlantı güvenilen bir adresten geliyorsa `X-Real-IP`'yi, gelmiyorsa bağlantının kendi adresini
(`RemoteAddr`) kullanır. NGINX ise istemcinin gönderdiği `X-Real-IP`'yi kendi gördüğü adresle
ezer — sahte başlık iki yoldan da geçemez (#3).

**🔗 İlgili:**
- [docker-compose.yml](../../docker-compose.yml) (`ipv4_address`, `networks.default.ipam`,
  `TRUSTED_PROXIES`) · [deploy/k8s/config.yaml](../../deploy/k8s/config.yaml)
- [#3 Reverse proxy — gerçek istemci IP'si](#3-reverse-proxy-tek-origin-gerçek-istemci-ipsi-ve-add_header-tuzağı) ·
  [#11 Loglarda IP alanları](#11-nginxi-canlıda-gözlemlemek-access-log-api-logu-ve-istek-kimliği)

---

## 11. NGINX'i canlıda gözlemlemek: access log, API logu ve istek kimliği

**🗓️ Bağlam:** Canlıya çıktıktan sonra "panelde NGINX'e istek geldiğinde ne olduğunu görebiliyor
muyum artık?" diye sordum — trafiği gösteren bir yönetim paneli bekliyordum.

**⚡ Kısa cevap:** Açık kaynak NGINX'in paneli yok; penceresi **loglar**. İmajda `access.log`
konteynerin standart çıktısına bağlı, Northflank da bu çıktıyı gösteriyor: *Services → web →
Logs* (her istek bir satır) ve *Services → api → Logs* (yalnız `/api` istekleri, JSON).

**📖 Açıklama:**

**Satırlar oraya nasıl düşüyor:** imajda şu sembolik bağlar var:
```
/var/log/nginx/access.log -> /dev/stdout
/var/log/nginx/error.log  -> /dev/stderr
```
NGINX dosyaya yazdığını sanıyor, yazdığı konteynerin çıktısına gidiyor; Docker
(`docker logs bookhub-web`) ve Northflank bu çıktıyı topluyor (12-factor XI: log bir olay akışı).
`/healthz` isteklerinde `access_log off` → sağlık kontrolleri logu kirletmiyor.

**NGINX satırı** (imajın `log_format main`'i):
```
10.x.x.x - - [04/Oct/2026:13:20:01 +0000] "GET /api/books?limit=12 HTTP/1.1" 200 5123 "https://p01--web--….code.run/" "Mozilla/5.0 …" "85.x.x.x"
```
Sırasıyla: bağlantıyı açan adres · zaman · istek satırı (sorgu dahil) · durum · cevap boyutu ·
referrer · tarayıcı · `X-Forwarded-For`.

**API satırı** ([middleware.go](../../backend/cmd/api/middleware.go) `requestLogger`, `LOG_FORMAT=json`):
```json
{"msg":"http","method":"GET","path":"/api/books","status":200,"bytes":5123,"duration_ms":41.3,"remote":"10.x.x.x","request_id":"…"}
```
- `path` **sorgusuz** — arama terimleri kişisel veri olabilir diye bilerek.
- `duration_ms` — Londra (Northflank) → Frankfurt (Supabase) gecikmesi burada görünür.
- `status >= 500` → `ERROR` seviyesi: "5xx'leri göster" sorgusu metin ayrıştırmadan yapılır.

**Deney:** web ve api loglarını iki sekmede yan yana aç, siteyi yenile. Önce `/`,
`/assets/*.js`, `/assets/*.css` yalnız NGINX logunda; ardından `/api/books`,
`/api/books/top-rated`, `/api/genres` iki logda birden görünür.

**Okurken dikkat:**
- **IP'ler:** NGINX satırının başındaki adres Northflank'ın ağ geçidi; ziyaretçinin gerçek IP'si
  büyük ihtimalle sondaki `X-Forwarded-For`'da. API'nin `remote`'u NGINX'in iç adresi → IP başına
  sınırın canlıda ortak olmasının görünür kanıtı (#10).
- **Gizlilik tutarsızlığı:** API sorguyu bilerek yazmıyor, ama NGINX'in varsayılan biçimi
  (`"$request"`) `?q=…`'yu olduğu gibi yazıyor → karar bu katmanda delinmiş. Referrer da aynı
  sorguyu taşıyabilir (aynı origin'e giden isteklerde tarayıcı sayfanın tam adresini yollar).
  Çözüm (henüz yapılmadı): yolu sorgusuz yazan özel bir `log_format`. `$uri` işe yaramaz:
  `try_files`'tan sonra SPA sayfalarında `/index.html` olur (denendi: `/books/42?q=…` →
  `/index.html`). Doğrusu sorguyu `$request_uri`'den bir `map` ile atmak:
  `map $request_uri $path_only { ~^(?<p>[^?]*) $p; }` → biçimde `"$request_method $path_only"`.
- **İstek kimliği:** API her cevaba `X-Request-Id` koyuyor ama NGINX satırında yok → iki logu
  eşleştirmek için zamana bakmak gerekiyor. Ek bulgu: chi'nin `RequestID` middleware'i **gelen**
  `X-Request-Id`'yi aynen kullanıyor ve NGINX bu başlığı ezmiyor —
  `curl -H 'X-Request-Id: benim-uydurdugum-id' http://localhost:3000/api/genres` ile denendi,
  cevap uydurulan kimliği taşıdı. Öneri (henüz yapılmadı): NGINX'te
  `proxy_set_header X-Request-Id $request_id;` + `log_format`'a `$request_id` → iki log aynı
  kimlikle eşleşir, istemci de kimlik uyduramaz.

Northflank'ın *Metrics* sekmesi makine düzeyindeki kaynakları (CPU, bellek, ağ) gösterir; tek
tek istekleri görmek için loglara bakılır.

**🔗 İlgili:**
- [#8 Görevler ve location](#8-nginxin-görevleri-ve-bir-isteğin-yolu-location-seçimi-spa-fallback-ve-rate-limit) ·
  [#9 proxy_pass](#9-proxy_pass-ve-tek-origin-nginx-isteği-apiye-nasıl-iletir) ·
  [#10 Sabit IP](#10-docker-ağında-sabit-ip-ve-güvenilen-proxy-17228010-nereden-geliyor)
- [backend/cmd/api/middleware.go](../../backend/cmd/api/middleware.go) (`requestLogger`) ·
  [backend/cmd/api/router.go](../../backend/cmd/api/router.go) (`middleware.RequestID`)
- [frontend/nginx/default.conf](../../frontend/nginx/default.conf) (`access_log off`)
