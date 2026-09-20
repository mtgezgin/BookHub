# Faz 11 — NGINX

> Durum: ✅ TAMAM · İlgili: [frontend/nginx/](../../frontend/nginx/)

## 🎯 Amaç
Tek kapı: statik frontend + `/api` reverse proxy, güvenlik başlıkları, gerçek istemci IP'si.

## 🧠 Yeni kavram(lar)
- Reverse proxy, SPA fallback (`try_files`), içerik hash'li varlıklar için uzun cache.
- **Güvenilen proxy**: `X-Real-IP` yalnız NGINX'in adresinden gelince dikkate alınır.
- CSP ve NGINX'in `add_header` miras tuzağı (denendi: kendi `add_header`'ı olan location üst seviyedekileri düşürür).

## ✅ Tamamlanma
- [x] `http://localhost:3000` → tüm uygulama; sahte `X-Real-IP` iki yoldan da yok sayılıyor; smoke 21/21
      (o günkü sayı; Faz 13'ten beri 25/25).
- [x] NGINX root olmayan kullanıcıyla (uid 101, port 8080).
