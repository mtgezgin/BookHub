# syntax=docker/dockerfile:1
# BookHub web imajı (Faz 11): frontend'i derle, NGINX ile sun, /api'yi Go API'ye ilet.

# DIGEST SABİTLEME (tag@sha256): "node:24-alpine" etiketi her hafta başka bir imaja işaret
# edebilir; digest ise tek bir içeriğin parmak izidir. Böylece dünkü build ile bugünkü aynı
# temeli kullanır (tekrarlanabilirlik) ve bir etiketi ele geçiren biri bize başka bir imaj
# yediremez. Bedeli: güvenlik yamaları kendiliğinden gelmez → .github/dependabot.yml onları
# haftalık PR olarak getirir. Etiket, insan okusun diye digest'in yanında duruyor.
#
# ---------- Aşama 1: derleme ----------
# Bu aşamadaki hiçbir şey son imaja GİRMEZ (node, npm, node_modules). Tarayıcılar burada
# npm'in kendi gömülü paketlerinde (tar, undici, ip-address, brace-expansion) açık bildirir
# (2026-09: 4 high, 5 medium); yayınlanan imajda bulunmadıkları için CI'daki Trivy taraması
# yalnız SON imajları tarar. npm@latest bile aynı sürümleri gömüyordu — upstream düzeltmesini
# Dependabot'un getireceği yeni node imajı taşıyacak.
FROM node:24-alpine@sha256:ebfe2f90462722a7a4de65e91990e97fe0d401c70e0e762c5b53302f905ec1c1 AS build
WORKDIR /src
# Önce yalnız paket dosyaları: bağımlılık katmanı kod değişiminde cache'ten gelir.
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY . .
RUN npx vite build

# ---------- Aşama 2: sunum ----------
# nginx-UNPRIVILEGED: süreç root DEĞİL (uid 101) çalışır. Bedeli: 1024 altı portlara bağlanamaz
# → 80 yerine 8080'de dinler (dışarıya hangi porttan açılacağına compose/k8s karar verir).
# API imajındaki "nonroot" kararının web tarafındaki karşılığı; k8s'te runAsNonRoot ile uyumlu.
# -slim: dinamik modülsüz sürüm (geoip, image-filter, njs, xslt ve kütüphaneleri — libxml2,
# libxslt, gd… — yok). Hiçbirini kullanmıyoruz; daha az paket = daha küçük saldırı yüzeyi
# (91 MB → 30 MB, 35 → 21 paket) ve Docker Scout'un standart sürümde bildirdiği "1 high"
# bu modül kütüphanelerindeydi, slim'de yok. Healthcheck'in wget'i busybox'tan gelir, slim'de de var.
FROM nginxinc/nginx-unprivileged:stable-alpine-slim@sha256:3af0c10d960cc2502427fe1219c52989d309e7d65596869c60a34fd2fa2406f0
COPY nginx/default.conf /etc/nginx/conf.d/default.conf
COPY nginx/security-headers.conf nginx/api-proxy.conf /etc/nginx/snippets/
COPY --from=build /src/dist /usr/share/nginx/html
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
