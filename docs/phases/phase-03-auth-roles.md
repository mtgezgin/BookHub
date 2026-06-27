# Faz 3 — Auth + Roller

> Durum: 📝 taslak (sırası gelince birlikte detaylandıracağız) · Önkoşul: Faz 2 ✅
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

## ✅ Tamamlanma (taslak)

- [ ] Register/login çalışıyor; parola hash'li saklanıyor.
- [ ] Korunan route'lar JWT olmadan 401; ADMIN-only route'lar USER ile 403.
- [ ] Parola hash ve yetki davranışı için testler.

## 📒 WHY_GO_VS_TS güncellemesi

- **§2** (errors-as-values) → auth hatalarının HTTP status'e tek yerde map'lenmesi.
- **§3** (DI/Service) → ilk gerçek Service katmanı (auth).
