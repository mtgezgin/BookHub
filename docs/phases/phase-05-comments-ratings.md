# Faz 5 — Yorum + Puan

> Durum: 📝 taslak · Önkoşul: Faz 4 ✅
> İlgili: [WHY_GO_VS_TS](../WHY_GO_VS_TS.md) §2/§10 · [GO_CONVENTIONS](../GO_CONVENTIONS.md) §7 · [PROJECT_BRIEF](../PROJECT_BRIEF.md) §6.5, §6.6

## 🎯 Amaç

Kullanıcılar onaylı kitaplara yorum yapsın (silmeyi sadece admin yapar) ve 1–10 puan versin
(kullanıcı başına bir kez, güncellenebilir). Kitabın ortalama/sayım istatistikleri **doğru**
güncellensin.

## 🧠 Yeni kavram(lar)

- One-to-many ilişkiler; **soft-delete** (`deleted_at IS NULL` tek doğruluk kaynağı).
- **Eşzamanlılık doğruluğu**: `UNIQUE(user_id, book_id)` + `INSERT ... ON CONFLICT DO UPDATE`;
  agregatları **aynı transaction'da** SQL `AVG`/`COUNT` ile türetmek.

## ⚠️ Önemli noktalar (projenin #1 doğruluk dersi)

- `ratings` **tek doğruluk kaynağı**. `average_rating`/`rating_count`'u **asla** app-read-then-write
  yapma (iki eşzamanlı oy = lost update). Türet. (GO_CONVENTIONS §7, WHY_GO_VS_TS §10)
- "Bir kez oy" + "oyu güncelle" = tek statement: `ON CONFLICT (user_id, book_id) DO UPDATE`.
- Yorum silmeyi sadece admin yapar; soft-delete + `deleted_by`.
- Tüm yorum okumaları `WHERE deleted_at IS NULL` (bir view ile standartlaştır).
- `go test -race` ile eşzamanlı oy senaryosunu dene — yarış dedektörü burada işe yarar.

## ✅ Tamamlanma (taslak)

- [ ] Yorum oluşturma + admin silme (soft-delete) çalışıyor.
- [ ] Puan oluşturma/güncelleme; `UNIQUE` + `ON CONFLICT` ile bir-kez kuralı DB seviyesinde.
- [ ] average_rating/rating_count türetiliyor (tx içinde), denormalize-yazma yok.
- [ ] Eşzamanlılık testi `-race` ile yeşil.

## 📒 WHY_GO_VS_TS güncellemesi

- **§10** (race detector + DB transaction) → `internal/ratings/service_test.go` (-race).
- **§2** (UNIQUE ihlali → 409) → rating handler hata map'i.
