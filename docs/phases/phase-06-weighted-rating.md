# Faz 6 — Ağırlıklı Puan (Weighted Rating)

> Durum: ✅ TAMAM · Önkoşul: Faz 5 ✅
> İlgili: [WHY_GO_VS_TS](../WHY_GO_VS_TS.md) §9/§12/§13 · [GO_CONVENTIONS](../GO_CONVENTIONS.md) §8 · [PROJECT_BRIEF](../PROJECT_BRIEF.md) §6.7

## 🎯 Amaç

Kitapları adil sırala: 1 oyla 10/10 alan kitap, 2000 oyla 8.5 alan kitabın üstüne çıkmasın.
IMDb tarzı Bayesian ağırlıklı puanı uygula ve **table-driven test**'lerle kanıtla.

## 🧠 Yeni kavram(lar)

- Bayesian/ağırlıklı ortalama: `WR = (v/(v+m))*R + (m/(v+m))*C`.
- **Pure fonksiyon** tasarımı (DB'siz) → kolay test edilebilir, kolay güvenli.
- Ağır aggregation SQL; **EXPLAIN ANALYZE** ile sorgu planı okuma.

## ⚠️ Önemli noktalar

- `weighted_score`'u **her yazımda denormalize kolon olarak saklama** — `C` (global ortalama) ve
  `m` global; bir yeni oy tüm kitapların skorunu değiştirir, sakladığın kolon bayatlar.
  **Query-time'da hesapla** (veya ölçülen yavaşlıkta materialized view). (PROJECT_BRIEF §6.7)
- `m`'i **küçük başlat (~10)**, 100 değil (az veride her skor `C`'ye çöker). `C` türetilir
  (`AVG(score)`), cold-start fallback'i ile.
- Ağırlıklı matematik **pure fonksiyon** olsun (`WeightedScore(r, v, m, c)`); HTTP/DB'den ayrı.
- **EXPLAIN ANALYZE alışkanlığı:** sıralama yavaşsa nedeni dil değil, eksik index olabilir;
  önce ölç. (WHY_GO_VS_TS §12)
- "Paralel recompute job" yazma — bu ölçekte sahte; ayrıca query-time kararıyla çelişir.

## ✅ Tamamlanma

- [x] `WeightedScore` pure fonksiyonu + table-driven testler (brief'in örneği: m=100'de A 7.03 < B 8.43).
- [x] `GET /api/books/top-rated` query-time ağırlıklı skorla sıralıyor; SQL skoru Go spesifikasyonuyla birebir (testli).
- [x] `m` config'ten (`RATING_MIN_VOTES`, varsayılan 10); `C` türetiliyor, cold-start 5.5.
- [x] EXPLAIN ANALYZE: public liste index'i düzeltildi (6.5 → 0.06 ms), C hesabı 43 → 5.7 ms.

## 🧾 Ne yapıldı

| Commit | İçerik |
|---|---|
| `1e4ab05` | `WeightedScore` + bayrak tablo testi |
| `4e3217a` | top-rated ucu (query-time), config `RATING_MIN_VOTES` |
| `06f9ec8` | EXPLAIN ANALYZE bulguları: index (0005) + agregattan C; `scripts/perf_seed.sql` |

**Testin öğrettiği:** ilk entegrasyon testinde yalnız iki kitap vardı ve C ≈ 9 çıktı; o dünyada
1 oylu 10'luk kitabın öne geçmesi istatistiksel olarak DOĞRU. Brief'in iddiası C iki kitabın da
altındayken geçerli — gerçekçi bir katalogda. Test artık bu önkoşulu açıkça kuruyor.

## 📒 WHY_GO_VS_TS güncellemesi

- **§13** (table-driven test) → `internal/ratings/score_test.go` bayrak testi.
- **§9** (concurrency) → "sequential yeterliydi" dürüst notu (paralel gerekmedi).
- **§12** (perf) → EXPLAIN ANALYZE'dan gerçek bir gözlem (yoksa `UNVERIFIED-PERF`).
