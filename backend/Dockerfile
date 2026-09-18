# syntax=docker/dockerfile:1
#
# BookHub API imajı — ÇOK AŞAMALI (multi-stage) derleme (Faz 8).
#
# AŞAMA 1 (build): Go derleyicisi, modül cache'i, kaynak kod — ~300+ MB. Yalnızca derlemek için.
# AŞAMA 2 (run):   yalnızca derlenmiş statik binary'ler + CA sertifikaları — birkaç MB.
# Son imaja derleyici, kaynak kod, shell, paket yöneticisi GİRMEZ: saldırı yüzeyi küçülür,
# imaj hızlı çekilir (WHY_GO_VS_TS §11'in ölçüldüğü yer).

# ---------- Aşama 1: derleme ----------
FROM golang:1.26-alpine AS build
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
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /out/manage /app/
USER nonroot:nonroot
EXPOSE 8080

# Shell/curl yok → binary kendini yoklar (cmd/api main.go: "healthcheck" alt komutu).
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 CMD ["/app/api", "healthcheck"]

ENTRYPOINT ["/app/api"]
