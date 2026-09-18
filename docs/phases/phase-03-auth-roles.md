# Faz 3 — Auth + Roller

> Durum: ✅ TAMAM · Önkoşul: Faz 2 ✅
> İlgili: [WHY_GO_VS_TS](../WHY_GO_VS_TS.md) §2/§3 · [GO_CONVENTIONS](../GO_CONVENTIONS.md) §12 (güvenlik) · [PROJECT_BRIEF](../PROJECT_BRIEF.md) §6.1, §10.1

## 🎯 Amaç

Kullanıcı kaydı/girişi, parola güvenliği, JWT ile kimlik doğrulama ve rol bazlı yetkilendirme
(USER/ADMIN). İlk **Service katmanı** burada doğar (gerçek iş mantığı = auth kuralları).

## 🧠 Yeni kavram(lar)

- Parola **hash**'leme (bcrypt/argon2id), JWT üretme/doğrulama, **middleware** ile korunan route,
  yetki bağlamını (`ctx`'e user koyma), admin-only route grupları.

## ⚠️ Önemli noktalar (güvenlik fazı — yavaşla)

- bcrypt (cost ≥ 12) veya argon2id; **asla** SHA/MD5.
- JWT'de **imza algoritmasını açıkça doğrula** (`alg=none` tuzağı); secret env'den, commit'lenmez;
  kısa ömürlü access (~15dk).
- "Logout" = `refresh_tokens` tablosundan token silme (stateless JWT iptal edilemez); kullanımda
  rotate.
- **IDOR**: sadece "giriş yapmış mı" değil, **kaynak sahipliği** kontrolü (servis katmanında).
- Whitelisted DTO (mass-assignment yok); login hatası generic ("invalid credentials").
- Detaylar: [GO_CONVENTIONS.md](../GO_CONVENTIONS.md) §12.

## ✅ Tamamlanma

- [x] Register/login çalışıyor; parola bcrypt (cost 12) ile hash'li saklanıyor (`$2a$12$…`).
- [x] Korunan route'lar JWT olmadan 401; ADMIN-only route'lar USER ile 403 (`TestAdminRoutesAreGuarded`).
- [x] Parola hash, token doğrulama (13 red senaryosu), servis ve yetki davranışı için testler.
- [x] Refresh token rotasyonu + logout; eşzamanlı tüketimde tek kazanan (gerçek DB testi).
- [x] İlk admin: `go run ./cmd/manage create-admin -email …` (parola prompt/stdin'den).

## 🧾 Ne yapıldı (commit'ler)

| Commit | İçerik |
|---|---|
| `a4c9f80` | Migration 0002: `users` (email CHECK lower, role DEFAULT 'USER'), `refresh_tokens` (SHA-256) |
| `ecd9c8c` | bcrypt + HS256 access token, algoritma sabitleme (mutation ile kanıtlı) |
| `6def864` | `auth.Service`: register/login/refresh rotasyonu/logout, zamanlama eşitleme |
| `9c19fd6` | HTTP uçları + `requireAuth` + config (`JWT_SECRET` fail-fast) + 72 bayt login açığı düzeltmesi |
| `3820ae2` | `/api/admin` grubuna `requireAuth` + `requireAdmin` (push öncesi güvenlik bulgusu) |
| `9d9226c` | `cmd/manage create-admin` |

**Bulunan ve kapatılan gerçek açıklar:** (1) `/api/admin` kimliksizdi (güvenlik incelemesi);
(2) bcrypt karşılaştırması 72 baytta kırptığı için "72 baytlık parola + herhangi bir sonek"
girişi açıyordu (deneyerek bulduk).

## 📒 WHY_GO_VS_TS güncellemesi

- **§2** (errors-as-values) → auth hatalarının HTTP status'e tek yerde map'lenmesi.
- **§3** (DI/Service) → ilk gerçek Service katmanı (auth).
