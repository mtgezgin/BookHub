# Faz 16c — Katalog sorguları: tür süzgeci, sayım, derin sayfa, puan sırası

> Durum: ✅ canlıda (PR [#102](https://github.com/mtgezgin/BookHub/pull/102), 2026-11-19; canlı ölçüm #98'de) · Issue: [#98](https://github.com/mtgezgin/BookHub/issues/98) —
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

## 🎨 Kararlar (hepsi verildi; 5 ertelendi)

| # | Konu | Öneri | Gerekçe |
|---|---|---|---|
| 1 | Tür süzgeci | Ayrı sorgu (`ListBooksByGenre`); slug önce tür kimliğine çevrilir, sorguya kimlik verilir | Ölçüldü: 0,7–2,9 ms. Slug'la birleştirmek denendi, yetmedi (çok nadir türde 1.289 ms) |
| 2 | Slug → kimlik | Tür tablosu (26 satır) süreç içinde 60 sn tutulur | Ayrı bir sorgu canlıda ~13 ms'lik bir gidiş-dönüş demek |
| 3 | Liste sorguları | Önce sayfanın kimlikleri (`books`), sonra `book_view` | Ölçüldü: 58 → 1,2 ms |
| 4 | Toplam sayı | **Karar (kullanıcı, 2026-11-18):** tür başına sayılar süreç içinde 60 sn tutulur; cevap biçimi değişmez | Arayüz değişmez; toplam en çok 60 sn bayat olabilir. Yeni onaylanan kitap listede hemen görünür, yalnız "N sonuç" sayısı gecikebilir |
| 5 | Puan sırası | **Ertelendi** (kullanıcı kararı, 2026-11-18). Ayrıntı ve geri dönüş koşulu aşağıda: "Ertelenen karar" | Canlıda ölçülmüş bir sorun yok; ilk önerilen tasarımın zayıf yanı vardı |
| 7 | "En sevilenler" önbelleği | **Karar (2026-11-18; kullanıcı "UX açısından doğru olan" dedi):** puan yazımında silinmez, 60 sn'de bir yenilenir. **Moderasyon ve baskı değişikliklerinde silinmeye devam eder** | Kullanıcı açısından: puan veren kişi o an kitap sayfasındadır ve oradaki özet anında güncellenir (kendi yazdığını görür). Şerit beş kitaplık bir sıralamadır; tek bir oy onu nadiren değiştirir ve 60 sn'lik gecikme fark edilmez. Buna karşılık bugünkü davranış herkesi etkiler: puan gelen bir sitede şerit hep soğuktur ve ana sayfa yavaş açılır. Gizlenen ya da arşivlenen bir kitabın şeritten HEMEN kalkması ise doğruluk meselesidir; o yüzden moderasyon silmeye devam eder |
| 8 | 10.000 sınırı | Sınıra gelince arayüz "daha fazlası için aramayı daralt" der; "Sonraki" kapanır | Bugün düğme açık kalıp ilk sayfaya dönüyor (kod okunarak) |

## ⏸️ Ertelenen karar: puana göre sıralamanın skoru

**Karar (2026-11-18):** skor şimdilik saklanmaz; sorgu anında hesaplanmaya devam eder.

**Neden ertelendi:**

1. **Canlıda sorun ölçülmedi.** 1.125 ms ölçümü 500 bin kitaplık deney verisinden. Canlıdaki katalog
   küçük; aynı sorgu orada milisaniyeler sürer. Ölçülmemiş bir sorun için şema değiştirmek, saklanan
   türetilmiş veri için aranan "ölçümle gerekçeli" koşulunu karşılamaz.
2. **İlk önerilen tasarım zayıftı.** Skor yalnız kitabın kendi verisine değil **genel ortalamaya** da
   bağlı: A kitabına puan gelince B kitabının saklanan skoru da eskir. İlk öneri bunu elle güncellenen
   bir kolon ve günlük bir yenileme komutuyla yamıyordu: bakımı gereken, sessizce kayabilen bir yapı
   ve [GO_CONVENTIONS §7](../GO_CONVENTIONS.md)'nin bilinçli bir istisnası.

**Gerekince nasıl yapılacak:** genel ortalama (C) ve eşik (m) **sabit** alınır. Skor o zaman yalnız
satırın `average_rating` ve `rating_count` değerlerinin fonksiyonu olur ve Postgres'in **generated
column**'ı (ya da expression index) ile kurulur: değeri veritabanı hesaplar, kaynakla tutarsız olamaz,
tetikleyici ve yenileme komutu gerekmez. Kuralın istisnası da olmaz.

Bedeli (o gün yeniden tartılacak): gerçek ortalama zamanla kayarsa sıra çok hafif sapar ve düzeltmek
yeni bir migration ister; `RATING_MIN_VOTES` ortam değişkeni olmaktan çıkıp şemada sabitlenir. Bu yol
ayrıca **ölçülmedi**; index yapısı ölçülen saklanan skorla aynı olduğu için benzer sonuç beklenir,
uygulamadan önce ölçülecek.

**Geri dönüş koşulu:** canlıda `GET /api/books?sort=rating` ya da `/api/books/top-rated` isteklerinin
veritabanı süresi (erişim logundaki `db_ms`; ileride Grafana'da rotaya göre p95) düzenli olarak
**50 ms'yi** geçerse bu karar yeniden açılır.

**Şimdi yapılan ucuz kısım:** genel ortalama süreç içinde 60 sn tutulur (her istekte 204 ms'lik sorgu
çalışmaz) ve karar 7. İkisi de şemaya dokunmaz.

## 🚫 Kapsam dışı

- Anahtarla sayfalama (keyset): URL biçimini ve arayüzü değiştirir; karar 3 ve 8'den sonra ihtiyaç kalırsa.
- Liste cevabının daraltılması (açıklama alanı; F40).
- Arama sorgusu (`SearchBooks`): Faz 16b'de yapıldı.

## 🪜 Adımlar (her biri ayrı PR olabilir)

1. **Okuma yolu (şema değişmez):** karar 1, 2, 3, 4 ve 8. Geri alınması kolay.
2. **Önbellek:** genel ortalama süreç içinde 60 sn; karar 7 ("en sevilenler" puan yazımında silinmez).
3. ~~Saklanan skor~~: ertelendi (yukarıda).

## 📏 Sonuç ölçümleri (2026-11-19; 500 bin kitap, yerel Postgres 16, 7 çalıştırmanın ortancası, ms)

| Durum | Önce (eski sorgu) | Yeni sorgu, özel plan | Yeni sorgu, genel plan |
|---|---|---|---|
| Liste, türsüz, offset 0 | 0,75 | 0,10 | 155 |
| Liste, türsüz, offset 10.000 | 58 | 1,1 | 156 |
| Liste, yaygın tür (232 bin) | 120 | 0,14 | 87 |
| Liste, seyrek tür (17 bin) | 11 | 0,70 | 44 |
| Liste, nadir tür (501) | 3,9 | **36** | 42 |
| Liste, çok nadir tür (21) | 391 | 0,12 | 42 |
| Liste, yaygın tür, offset 10.000 | — | 27 | 107 |
| Puan sırası, yaygın tür | 1.125 + 204 | 138 | 152 |
| Sayım, yaygın tür (önbellekte 60 sn) | 278 | 72 | 80 |

**Genel plan sorunu ve çözümü:** pgx sorguları hazırlanmış deyim olarak çalıştırır; Postgres birkaç
çalıştırmadan sonra parametrelerin değerini bilmeden planlanan "genel plana" geçebilir ve LIMIT/OFFSET
parametre olunca o plan kötüdür (son sütun). Uygulamanın her bağlantısı `plan_cache_mode =
force_custom_plan` ile açılır (`internal/database`): her çalıştırma gerçek değerlerle planlanır; bedeli
sorgu başına ~0,1–0,6 ms planlama.

**Bilinen sınır, nadir tür (36 ms):** paralel tarama açıkken planlayıcı, 501 kitaplık türde "en yeniler"
index'inde yürümeyi seçiyor (33 bin satır). Paralel taramayı kapatmak bunu 0,9 ms'ye indiriyor ama büyük
taramaları üçe katlıyor (yaygın türde puan sırası 138 → 393 ms, sayım 72 → 241 ms); bu yüzden yapılmadı.
Canlıdaki Supabase'in paralel çalışan sayısı doğrulanmadı.

**Puan sırası (138 ms):** geç birleştirme 1.125 ms'yi 138 ms'ye indirdi; kalan maliyet skorun sorgu anında
hesaplanması (ertelenen karar, yukarıda).

## ✅ Kabul kriterleri

1. 500 bin kitapta her tür için liste isteğinin veritabanı süresi 5 ms'nin altında (`db_ms`; `EXPLAIN ANALYZE` PR'da).
2. ~~Puana göre ilk sayfa 5 ms'nin altında~~ (ertelenen karara bağlı). Yerine: puana göre liste isteğinde genel ortalama sorgusu çalışmaz (`db_queries` bir azalır).
3. Sürekli puan yazılırken "en sevilenler" isteklerinin çoğu sıralamayı yeniden çalıştırmaz.
4. 10.000 sınırındaki davranış bir testle sabit.
5. Hazırlanmış deyimin 6. çalıştırmasından sonra da (genel plan) süreler korunur.

## ⚠️ Doğrulanmayanlar

- Ölçümler yerel Postgres'te; canlıda her sorguya ~13 ms ağ gidiş-dönüşü eklenir (bölge taşınmadıkça).
- Süreç içi önbellek iki API örneği arasında paylaşılmaz (yayın sırasında iki örnek kısa süre farklı
  toplam gösterebilir); kabul edilebilir sayıldı, denenmedi.
