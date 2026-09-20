# Faz 7 — Redis Cache + Rate Limit (MVP'nin son fazı)

> Durum: ✅ TAMAM — **MVP TAMAM** 🎉 · Önkoşul: Faz 6 ✅
> İlgili: [WHY_GO_VS_TS](../WHY_GO_VS_TS.md) §7/§12/§14 · [GO_CONVENTIONS](../GO_CONVENTIONS.md) · [PROJECT_BRIEF](../PROJECT_BRIEF.md) §6.9, §7.3

## 🎯 Amaç

Redis'i **gerçek bir performans/durum problemi çözdüğü için** ekle — havalı olduğu için değil.
Önce ölç, sonra cache'le. Ayrıca login için temel rate limiting.

## 🧠 Yeni kavram(lar)

- Cache key tasarımı, TTL, **cache invalidation** (puan değişince top-rated cache'i temizle).
- `go-redis` ile bağlanma (dev'de Docker Redis, prod'da Upstash).
- Token-bucket **rate limiting** (önce in-memory, sonra Redis).

## ⚠️ Önemli noktalar

- **Sıra önemli:** önce index (EXPLAIN ANALYZE), sonra **yük testi** (k6/hey), **sonra** cache
  kararı. Eksik index'i "Redis lazım" sanma. (WHY_GO_VS_TS §12)
- Cache'i ölçülmüş bir sebeple ekle; sebebi ve "öncesi/sonrası" sayıyı WHY_GO_VS_TS §12'ye yaz.
- Rate limiter'ı **önce in-memory** yaz; çok-instance'ta neden kırıldığını *hissedince* Redis'e
  taşı ("neden paylaşılan state" anı).
- Redis'i Faz 7'den önce ekleme — Upstash'i seçmiş olman onu erken bağlamak demek değil. (§14.5)
- Redis çağrılarına da `ctx` ilet (timeout/iptal). (WHY_GO_VS_TS §7)

## ✅ Tamamlanma

- [x] Top-rated cache'leniyor (`books:top-rated:v1`, TTL 60 sn); puan yazımı ve moderasyon commit'i invalidate ediyor.
- [x] Yük testi öncesi/sonrası: 327 → 2892 istek/sn, p95 236 → 27 ms (WHY §12).
- [x] Login/register rate limit: bellek içi (tek instance) + Redis (paylaşılan) — iki instance'la canlı fark: 10 vs 5.
- [x] **MVP tamam:** `backend/scripts/mvp_smoke.sh` brief §15'in her maddesini uçtan uca doğruluyor
      (o gün 21/21; betiğe sonradan kontroller eklendi — Faz 13'ten beri 25/25).

## 🧾 Ne yapıldı

| Commit | İçerik |
|---|---|
| `5e1a179` | Redis (compose, 127.0.0.1:6380), opsiyonel `REDIS_URL`, `internal/cache` |
| `5a223d8` | Top-rated cache-aside + invalidation (moderasyon kancası) + yük testi |
| `62c7c28` | Rate limit (bellek/Redis), 429 + Retry-After, fail-open |
| `3376982` | MVP DoD smoke betiği |

## 📒 WHY_GO_VS_TS güncellemesi

- **§12** (perf, ölçülmüş) → yük testi öncesi/sonrası sayılar.
- **§7** (ctx) → Redis çağrısında deadline.
- **§14** (background concurrency) → cache invalidation; sadece gerçek bir iş çıkarsa doldur.
