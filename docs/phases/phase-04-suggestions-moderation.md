# Faz 4 — Öneri + Moderasyon

> Durum: 📝 taslak · Önkoşul: Faz 3 ✅
> İlgili: [WHY_GO_VS_TS](../WHY_GO_VS_TS.md) §1/§2 · [GO_CONVENTIONS](../GO_CONVENTIONS.md) §4 · [PROJECT_BRIEF](../PROJECT_BRIEF.md) §6.3, §10.2

## 🎯 Amaç

Kullanıcılar kitap önersin (status = `PENDING`); admin onaylasın/reddetsin. Onaylanınca kitap
herkese görünür olur. Admin aksiyonları **audit log**'a yazılsın.

## 🧠 Yeni kavram(lar)

- **Status state machine** (PENDING→APPROVED/REJECTED/ARCHIVED) ve izinli geçiş kuralları.
- Admin moderasyon akışı; **append-only `audit_logs`** tablosu (kim, ne, ne zaman).

## ⚠️ Önemli noktalar

- Status geçişlerini **servis katmanında** doğrula (geçersiz geçişi reddet) — gerçek iş mantığı,
  Service katmanının yeri.
- Audit kaydını moderasyon kararıyla **aynı transaction'da** yaz (drift olmasın).
- Status'u `TEXT + CHECK` ile tut (Faz 2 kararı); `ParseStatus` sınır fonksiyonuyla geçersiz
  değeri kenarda yakala. (WHY_GO_VS_TS §1)
- Yalnızca onaylı kitaplar public listede (status filtreli sorgu + partial index).

## ✅ Tamamlanma (taslak)

- [ ] Öneri → PENDING; admin approve/reject çalışıyor; reject opsiyonel sebep.
- [ ] Onaylı kitaplar public, diğerleri değil.
- [ ] Her admin aksiyonu audit_logs'a (aynı tx'te) yazılıyor.

## 📒 WHY_GO_VS_TS güncellemesi

- **§1** (typed status / state machine) → ikinci, daha zengin örnek (geçiş doğrulama).
- **§2** (errors-as-values) → geçersiz geçiş / yetki hatası → temiz HTTP status.
