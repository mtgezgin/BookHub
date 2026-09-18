# Faz 8 — Docker imajı + Compose

> Durum: ✅ TAMAM · Önkoşul: MVP (Faz 1–7) ✅
> İlgili: [WHY_GO_VS_TS](../WHY_GO_VS_TS.md) §11 · [infra-and-devops notları](../learning-notes/infra-and-devops.md)

## 🎯 Amaç

API'yi bir konteyner imajına paketle ve tüm uygulamayı (API + Postgres + Redis) tek komutla
konteynerde çalıştır — prod'a giden yolun ilk adımı.

## 🧠 Yeni kavram(lar)

- **Multi-stage build**, `distroless` çalışma imajı, root olmayan kullanıcı.
- Compose **servis DNS**'i (`postgres:5432`), **profil**ler, healthcheck'e dayalı `depends_on`.
- Yapılandırılmış (JSON) log ve request id korelasyonu — konteynerde log'un tek çıkışı stdout.

## 🪜 Ne yapıldı

| Commit | İçerik |
|---|---|
| `refactor(api): structured logging…` | slog + `X-Request-Id`, `run() int` (defer'lar her çıkışta çalışır), `IdleTimeout`/`ReadHeaderTimeout`, `api healthcheck` |
| `build(docker): …` | `backend/Dockerfile`, `.dockerignore`, compose `api` servisi (`app` profili), `Makefile` |

Çalıştırma:

```bash
make up          # yalnız altyapı (Postgres + Redis) — API'yi `make run` ile host'ta geliştir
make app-up      # her şey konteynerde (imajı da derler) → http://localhost:8080
make smoke       # MVP DoD'u çalışan API'ye karşı doğrula
```

## ⚠️ Önemli noktalar

- Konteyner İÇİNDEN `localhost` konteynerin kendisidir; DB'ye **servis adıyla** gidilir.
- Sır (`JWT_SECRET`) compose dosyasında DEĞİL; compose onu git-dışı `.env`'den okur.
- `distroless`'ta shell yok → `docker exec -it bookhub-api sh` çalışmaz (bilerek); teşhis log'larla.

## ✅ Tamamlanma

- [x] İmaj 38 MB, root değil, healthcheck'li.
- [x] `docker compose --profile app up -d --build` → api `healthy`.
- [x] `mvp_smoke.sh` konteynere karşı 21/21.
