# Öğrenme Notları — Infra & DevOps

Docker, Redis hosting, deploy, CI/CD, reverse proxy, Kubernetes.
Format ve nasıl çalıştığı: [README.md](README.md).

## İçindekiler

1. [Multi-stage Docker imajı ve distroless: neden 38 MB?](#1-multi-stage-docker-imajı-ve-distroless-neden-38-mb)

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
