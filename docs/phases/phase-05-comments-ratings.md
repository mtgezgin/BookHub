# Faz 5 — Yorum + Puan

> Durum: ✅ TAMAM · Önkoşul: Faz 4 ✅
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

## ✅ Tamamlanma

- [x] Yorum oluşturma (yalnız onaylı kitaba) + admin silme (soft-delete + denetim kaydı); okumalar `visible_comments` VIEW'undan.
- [x] Puan oluşturma/güncelleme: `(user_id, book_id)` PK + `ON CONFLICT DO UPDATE`; `PUT /api/books/{id}/rating`.
- [x] `average_rating`/`rating_count` her yazımda aynı tx'te SQL `AVG/COUNT` ile türetiliyor.
- [x] Eşzamanlılık testi (25 kullanıcı) yeşil; kilit kaldırılınca sayım 16–19'a düşüyor (mutation).

## 🧾 Ne yapıldı

| Commit | İçerik |
|---|---|
| `69d44f8` | `internal/audit` ortak yazıcı (refactor) |
| `73bcb97` | Migration 0004 + `ratings`/`comments` servisleri |
| `cc18495` | 7 HTTP ucu, public DTO'ya `average_rating`/`rating_count` |

**Bilinen sınır:** kullanıcı hesabı silinirse `ratings` CASCADE ile gider ama kitap agregatı
yeniden hesaplanmaz (bir sonraki puan yazımında kendini düzeltir). Şu an kullanıcı silme ucu
yok; eklenirse servis etkilenen kitapları yeniden hesaplamalı.

## 📒 WHY_GO_VS_TS güncellemesi

- **§10** (race detector + DB transaction) → `internal/ratings/service_test.go` (-race).
- **§2** (UNIQUE ihlali → 409) → rating handler hata map'i.
