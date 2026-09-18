# Öğrenme Notları — Infra & DevOps

Docker, Redis hosting, deploy, CI/CD, reverse proxy, Kubernetes.
Format ve nasıl çalıştığı: [README.md](README.md).

## İçindekiler

1. [Multi-stage Docker imajı ve distroless: neden 38 MB?](#1-multi-stage-docker-imajı-ve-distroless-neden-38-mb)
2. [CI'da gerçek veritabanı: servis konteynerleri ve uçtan uca iş](#2-cida-gerçek-veritabanı-servis-konteynerleri-ve-uçtan-uca-iş)
3. [Reverse proxy: tek origin, gerçek istemci IP'si ve add_header tuzağı](#3-reverse-proxy-tek-origin-gerçek-istemci-ipsi-ve-add_header-tuzağı)
4. [Kubernetes'in temel parçaları (ve deploy ederken çıkan iki hata)](#4-kubernetesin-temel-parçaları-ve-deploy-ederken-çıkan-iki-hata)

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

**Sonuç:** smoke 21/21; istekler iki API kopyasına dağıldı (28/30) — oturum JWT'de, sayaçlar
Redis'te olduğu için hangi kopyanın cevap verdiği fark etmiyor.

**🔗 İlgili:** [deploy/k8s/](../../deploy/k8s/) · [Makefile](../../Makefile) (`k8s-*`)
