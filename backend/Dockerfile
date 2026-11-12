# syntax=docker/dockerfile:1
#
# BookHub API imajı — ÇOK AŞAMALI (multi-stage) derleme (Faz 8).
#
# AŞAMA 1 (build): Go derleyicisi, modül cache'i, kaynak kod — ~300+ MB. Yalnızca derlemek için.
# AŞAMA 2 (run):   yalnızca derlenmiş statik binary'ler + CA sertifikaları — birkaç MB.
# Son imaja derleyici, kaynak kod, shell, paket yöneticisi GİRMEZ: saldırı yüzeyi küçülür,
# imaj hızlı çekilir (WHY_GO_VS_TS §11'in ölçüldüğü yer).

# ---------- Aşama 1: derleme ----------
# Digest sabitleme: bkz. frontend/Dockerfile'daki açıklama (Dependabot haftalık günceller).
# Etikette yama sürümü yazılı: backend/go.mod'daki `go` satırıyla aynı olmalı (CI ve Deploy'un
# migrate adımı sürümü oradan alır). scripts/checks.sh ikisinin ayrışmasını durdurur; Dependabot
# bu imajı yükseltince aynı PR'da go.mod da güncellenir.
FROM golang:1.27.3-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18 AS build
WORKDIR /src

# Önce YALNIZCA go.mod/go.sum: bağımlılıklar kodun her değişiminde değil, yalnız bunlar
# değişince yeniden indirilsin (Docker katman cache'i). Kaynak değişti → bu katman cache'ten.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0: C kütüphanesine bağımlı olmayan TAM STATİK binary → libc'siz imajda çalışır.
# -trimpath: binary'ye derleyen makinenin dosya yolları gömülmesin (tekrarlanabilir derleme).
# -ldflags "-s -w": sembol tablosu ve debug bilgisi atılır → daha küçük binary.
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && go build -trimpath -ldflags="-s -w" -o /out/manage ./cmd/manage

# ---------- Aşama 2: çalışma ----------
# distroless/static: yalnız CA sertifikaları, tzdata ve /etc/passwd — shell bile yok.
# ":nonroot" etiketi süreci root OLMAYAN kullanıcıyla (uid 65532) çalıştırır: container'dan
# kaçan bir saldırgan root yetkisi bulmaz.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/api /out/manage /app/
# SAYISAL uid (65532 = distroless'ın "nonroot" kullanıcısı): Kubernetes'in runAsNonRoot kontrolü
# kullanıcıyı ADLA ("nonroot") görürse root olmadığını doğrulayamaz ve pod'u başlatmaz
# ("image has non-numeric user") — kind'da tam olarak bu hatayı aldık.
USER 65532:65532
EXPOSE 8080

# Shell/curl yok → binary kendini yoklar (cmd/api main.go: "healthcheck" alt komutu).
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 CMD ["/app/api", "healthcheck"]

ENTRYPOINT ["/app/api"]
