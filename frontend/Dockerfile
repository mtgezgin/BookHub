# syntax=docker/dockerfile:1
# BookHub web imajı (Faz 11): frontend'i derle, NGINX ile sun, /api'yi Go API'ye ilet.

# ---------- Aşama 1: derleme ----------
FROM node:24-alpine AS build
WORKDIR /src
# Önce yalnız paket dosyaları: bağımlılık katmanı kod değişiminde cache'ten gelir.
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY . .
RUN npx vite build

# ---------- Aşama 2: sunum ----------
FROM nginx:stable-alpine
COPY nginx/default.conf /etc/nginx/conf.d/default.conf
COPY nginx/security-headers.conf nginx/api-proxy.conf /etc/nginx/snippets/
COPY --from=build /src/dist /usr/share/nginx/html
EXPOSE 80
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD wget -qO- http://127.0.0.1/healthz || exit 1
