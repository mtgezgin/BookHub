# Faz 16c — Katalog sorguları: tür süzgeci, sayım, derin sayfa, puan sırası

> Durum: 📝 **taslak spec; onay bekliyor** (2026-11-17). Kod yazılmadı · Issue: açılmadı (onaydan sonra) —
> [ROADMAP](../ROADMAP.md) · Bulgular: [REVIEW_FEEDBACK](../REVIEW_FEEDBACK.md) F24, F25, F26 ·
> İlgili: [books.sql](../../backend/sql/queries/books.sql), [ratings.sql](../../backend/sql/queries/ratings.sql),
> [repository.go](../../backend/internal/books/repository.go), [cached.go](../../backend/internal/ratings/cached.go)

## 🎯 Amaç

Katalog listesi 500 bin kitapta da milisaniyeler içinde dönsün. Bugün canlıdaki katalog küçük olduğu için
sorun görünmüyor; bulgular "katalog büyümeden önce" sınıfında. Üç bulgu aynı üç sorguya dokunduğu için
tek iş olarak ele alınıyor.

## 🔍 Başlangıç durumu (ölçümler: 500 bin kitap, yerel Postgres 16; kaynak REVIEW_FEEDBACK)

| Yol | Bugün | Neden |
|---|---|---|
| Tür süzgeciyle liste | 4–391 ms | `($1 IS NULL OR EXISTS …)`: planlayıcı EXISTS'i birleştirmeye çeviremiyor |
| Sayım (her liste isteğinde) | 50–278 ms | Tam sayım listenin kendisinden pahalı |
| Offset 10.000 | 58 ms | Görünüm, LIMIT'ten önce her satır için baskıya birleşiyor |
| Puana göre liste | 1.125 ms + 204 ms | Skor sorgu anında hesaplanıyor; genel ortalama her istekte |
| "En sevilenler" (önbellek ıskası) | 887 + 204 ms | Aynı; önbellek her puan yazımında siliniyor |

Ölçülmüş çözümler: tür **kimliğiyle** liste 0,7–2,9 ms; önce kimlik sonra görünüm 1,2 ms (offset 10.000);
saklanan skor + index 0,95 ms.

## 🎨 Kararlar (önerilen; ⚠️ işaretliler senin kararın)

| # | Konu | Öneri | Gerekçe |
|---|---|---|---|
| 1 | Tür süzgeci | Ayrı sorgu (`ListBooksByGenre`); slug önce tür kimliğine çevrilir, sorguya kimlik verilir | Ölçüldü: 0,7–2,9 ms. Slug'la birleştirmek denendi, yetmedi (çok nadir türde 1.289 ms) |
| 2 | Slug → kimlik | Tür tablosu (26 satır) süreç içinde 60 sn tutulur | Ayrı bir sorgu canlıda ~13 ms'lik bir gidiş-dönüş demek |
| 3 | Liste sorguları | Önce sayfanın kimlikleri (`books`), sonra `book_view` | Ölçüldü: 58 → 1,2 ms |
| 4 ⚠️ | Toplam sayı | **A (önerilen):** tür başına sayılar süreç içinde 60 sn tutulur; cevap biçimi değişmez, toplam en çok 60 sn bayat olabilir. **B:** `total` yerine `has_more`; arayüzde "57 sonuçtan 1–20" yazısı ve sayfa numaraları kalkar | A arayüzü değiştirmez. B en ucuzu ama görünür bir ürün değişikliği |
| 5 ⚠️ | Puan sırası | Skor kolon olarak saklanır (`books.weighted_score`), puan değişince o kitabın skoru güncellenir; genel ortalama süreç içinde 60 sn tutulur ve skorlar o değerle hesaplanır | Ölçüldü: 0,95 ms. Bedeli: genel ortalama değiştikçe eski skorlar hafif bayat kalır; sıra pratikte kaymaz, ama **GO_CONVENTIONS §7'deki "türetilmiş veri kaynaktan hesaplanır" kararının bilinçli bir istisnasıdır** |
| 6 | Bayat skorlar | Günde bir kez tüm skorları yeniden hesaplayan `manage refresh-scores` komutu (elle ya da zamanlanmış iş) | Northflank ücretsiz planında zamanlanmış iş olup olmadığı **doğrulanmadı** |
| 7 ⚠️ | "En sevilenler" önbelleği | Puan yazımında silinmez; 60 sn'lik süreyle dolar | Bugün puan gelen bir sitede hep soğuk. Bedeli: puan veren kullanıcı şeritteki değişikliği 60 sn'ye kadar görmez (kitap sayfasındaki özet anında güncellenmeye devam eder) |
| 8 | 10.000 sınırı | Sınıra gelince arayüz "daha fazlası için aramayı daralt" der; "Sonraki" kapanır | Bugün düğme açık kalıp ilk sayfaya dönüyor (kod okunarak) |

## 🚫 Kapsam dışı

- Anahtarla sayfalama (keyset): URL biçimini ve arayüzü değiştirir; karar 3 ve 8'den sonra ihtiyaç kalırsa.
- Liste cevabının daraltılması (açıklama alanı; F40).
- Arama sorgusu (`SearchBooks`): Faz 16b'de yapıldı.

## 🪜 Adımlar (her biri ayrı PR olabilir)

1. **Okuma yolu (şema değişmez):** karar 1, 2, 3, 4 ve 8. Geri alınması kolay.
2. **Saklanan skor, genişletme:** migration (kolon + index + doldurma), puan yazımında güncelleme; sorgular
   henüz eski yolu kullanır. Canlıda doğrulanır.
3. **Saklanan skor, kullanım:** sorgular yeni kolonu okur; karar 6 ve 7.

## ✅ Kabul kriterleri

1. 500 bin kitapta her tür için liste isteğinin veritabanı süresi 5 ms'nin altında (`db_ms`; `EXPLAIN ANALYZE` PR'da).
2. Puana göre ilk sayfa 5 ms'nin altında.
3. Sürekli puan yazılırken "en sevilenler" isteklerinin çoğu sıralamayı yeniden çalıştırmaz.
4. 10.000 sınırındaki davranış bir testle sabit.
5. Hazırlanmış deyimin 6. çalıştırmasından sonra da (genel plan) süreler korunur.

## ⚠️ Doğrulanmayanlar

- Ölçümler yerel Postgres'te; canlıda her sorguya ~13 ms ağ gidiş-dönüşü eklenir (bölge taşınmadıkça).
- Karar 6'daki zamanlanmış iş seçeneği.
- Süreç içi önbellek iki API örneği arasında paylaşılmaz (yayın sırasında iki örnek kısa süre farklı
  toplam gösterebilir); kabul edilebilir sayıldı, denenmedi.
