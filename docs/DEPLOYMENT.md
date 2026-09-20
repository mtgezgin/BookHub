# BookHub — Dağıtım (Deploy) Rehberi

> **Durum:** CI (test + E2E) ve imaj yayınlama (`release.yml`) hazır. **Gerçek bir buluta
> deploy EDİLMEDİ** — bunun için senin hesapların (ve kararın) gerekiyor. Bu rehber, hesapları
> açtığında izleyeceğin adımlar. Mimari kararlar: [PROJECT_BRIEF.md](PROJECT_BRIEF.md) §7, §13.

## 1. Hedef mimari (ücretsiz/düşük maliyetli katmanlar)

```
                ┌──────────────── tarayıcı ────────────────┐
                │                                           │
        statik frontend (dist/)                     /api/* istekleri
                │                                           │
     ┌──────────▼──────────┐                    ┌───────────▼───────────┐
     │  NGINX konteyneri   │── /api/ proxy ───▶ │ API konteyneri        │
     │  (Faz 11) ya da     │                    │ ghcr.io/…/bookhub-api │
     │  Cloudflare Pages   │                    └──────┬─────────┬──────┘
     └─────────────────────┘                           │         │
                                              DATABASE_URL   REDIS_URL
                                                       │         │
                                              ┌────────▼──┐  ┌───▼──────┐
                                              │ Supabase  │  │ Upstash  │
                                              │ Postgres  │  │ Redis    │
                                              └───────────┘  └──────────┘
```

Kod, dev ile prod arasında **yalnızca env değişkenleriyle** geçiş yapar (12-factor III):
lokal Docker Postgres ↔ Supabase, lokal Redis ↔ Upstash.

## 2. Gerekli env değişkenleri (prod)

| Değişken | Prod değeri | Not |
|---|---|---|
| `DATABASE_URL` | Supabase **session pooler** ya da doğrudan bağlantı DSN'i, `sslmode=require` | Platformun secret store'unda; repoya asla |
| `JWT_SECRET` | `openssl rand -base64 48` çıktısı | `.env.example`'daki dev değerini ASLA kullanma |
| `REDIS_URL` | Upstash `rediss://…` (TLS) | Opsiyonel; boşsa cache kapalı, rate limit bellek içi (tek instance) |
| `AUTO_MIGRATE` | `false` | Migration ayrı, bilinçli bir adım (aşağıda) |
| `LOG_FORMAT` | `json` | Platform log toplayıcısı alanları ayrıştırsın |
| `PORT` | platformun verdiği | Çoğu platform `PORT`'u kendisi enjekte eder |
| `COOKIE_SECURE` | `true` (varsayılan) | Refresh çerezi yalnız HTTPS'te gider → **prod HTTPS ŞART**. `false` yalnız lokal denemeler için |
| `COOKIE_NAME` | `bookhub_refresh` (varsayılan) | Çerezler porta göre değil **host'a** göre kapsanır: aynı host'ta birden çok yığın varsa her birine ayrı ad ver (kind: `bookhub_refresh_k8s`). Boot'ta doğrulanır (RFC 6265 token; `__Host-` öneki `Path=/` istediği için kullanılamaz) |
| `TRUSTED_PROXIES` | Proxy'nin IP'si ya da CIDR'ı (boş = hiç kimseye güvenme) | `X-Real-IP`/`X-Forwarded-For` yalnız bu adreslerden gelirse dikkate alınır (Faz 11); yanlış değer = rate limit'i atlatan sahte IP |
| `JWT_ACCESS_TTL` | `15m` (varsayılan) | İptal gecikmesinin üst sınırı: çıkış/parola/rol değişimi bu süre kadar access token'ı etkilemez |
| `JWT_REFRESH_TTL` | `168h` = 7 gün (varsayılan) | Hareketsizlik penceresi; her rotasyon yeniden başlatır |
| `JWT_SESSION_MAX_AGE` | `720h` = 30 gün (varsayılan) | Bir girişin (token ailesinin) **mutlak** ömrü; `JWT_REFRESH_TTL`'den kısa olamaz (boot'ta doğrulanır) |
| `RATING_MIN_VOTES`, `AUTH_RATE_LIMIT_PER_MIN` | varsayılanlar makul | Ağırlıklı puanın `m` değeri · IP başına dakikalık kimlik isteği kotası (login/register/parola) |

## 3. Adımlar

1. **Veritabanı (Supabase):** proje aç → Settings → Database → bağlantı dizesini al.
2. **Migration'ı uygula** (bir kez ve her şema değişikliğinde, deploy'dan ÖNCE):
   ```bash
   goose -dir backend/migrations postgres "$PROD_DIRECT_DSN" up
   ```
   Transaction-mode pooler (6543) arkasında değil, **doğrudan bağlantıyla** (5432) çalıştır —
   migration'lar oturum seviyesinde kilit (advisory lock) kullanıyor.
3. **Redis (Upstash):** veritabanı aç → `rediss://` URL'ini al.
4. **İmaj:** `git tag v1.0.0` ve etiketi GitHub'a gönder → `release.yml` imajı
   `ghcr.io/mtgezgin/bookhub-api:1.0.0` olarak yayınlar (Actions sekmesinden elle de tetiklenir).
   Repo private olduğu için paket de private; platformun ghcr'den çekebilmesi için bir
   "read:packages" token'ı gerekir.
5. **API konteyneri:** Render / Fly.io / Koyeb gibi bir platformda imajı çalıştır, 2. bölümdeki
   env'leri secret olarak gir. Sağlık kontrolü yolu: `/health`.
6. **İlk admin:** platformun "one-off command" özelliğiyle:
   ```bash
   /app/manage create-admin -email sen@ornek.com   # parola gizli sorulur
   ```
   (Etkileşimsiz ortamda parolayı stdin'den ver; komut satırı argümanı olarak ASLA.)
7. **Frontend:** `cd frontend && npm ci && npx vite build` → `dist/`'i NGINX konteynerinde
   (Faz 11) ya da Cloudflare Pages'te yayınla. `/api/*` isteklerinin API'ye yönlenmesi şart
   (aynı origin → CORS gerekmez).
8. **Doğrula:** `API=https://senin-alanin mvp_smoke.sh` — ⚠️ smoke betiği admin açıp test
   verisi bırakır; prod'da değil, bir staging ortamında çalıştır.

## 4. Bilinen sınırlar / yapılacaklar

- **HTTPS ŞART.** Refresh token httpOnly + `Secure` + `SameSite=Strict` bir çerezte taşınıyor
  (Faz 13): düz HTTP'de tarayıcı o çerezi ne yazar ne geri gönderir → kullanıcı her sayfa
  yenilemesinde düşer. `COOKIE_SECURE=false` yalnızca lokal denemeler içindir, prod'da ASLA.
  Ayrıca tarayıcılar `Sec-Fetch-Site` başlığını yalnız güvenli bağlama yollar; düz HTTP'de
  CSRF korumasının tek dayanağı `Origin == Host` kontrolü kalır (proxy `Host`'u **portuyla**
  iletmeli — bkz. `frontend/nginx/api-proxy.conf`).
- Access token'lar stateless: çıkış, reuse iptali, parola değişimi ve rol değişikliği refresh
  tarafını anında keser, ama verilmiş access token'lar süreleri dolana kadar (`JWT_ACCESS_TTL`,
  varsayılan 15 dk) geçerli kalır.
- Tek API instance'ı varsayımıyla bellek içi rate limit; birden çok instance → `REDIS_URL` şart.
- `X-Forwarded-For` güveni: NGINX arkasında gerçek istemci IP'si için Faz 11 yapılandırması.
- Yedekleme: Supabase günlük yedek alır; point-in-time recovery ücretli katmanda.
