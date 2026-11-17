# Faz 16b — Arama: Türkçe duyarlı, index'li; yazarken arama

> Durum: 📝 kod yazıldı, yerelde doğrulandı (2026-11-17); yayın kullanıcı onayını bekliyor · Issue: [#82](https://github.com/mtgezgin/BookHub/issues/82) —
> [ROADMAP](../ROADMAP.md) · Backlog: [DISC-01](../FEATURE_REQUESTS.md)'in arama yarısı ·
> İlgili: [0014_book_search.sql](../../backend/migrations/0014_book_search.sql),
> [books.sql](../../backend/sql/queries/books.sql) (`SearchBooks`), [Home.tsx](../../frontend/src/pages/Home.tsx),
> [default.conf](../../frontend/nginx/default.conf) · Bulgular: [REVIEW_FEEDBACK](../REVIEW_FEEDBACK.md) F20, F21, F30, F34

## 🎯 Amaç

Katalog aramasını **doğru** (Türkçe harf, aksan, kelime sırası), **hızlı** (yüz binlerce kitapta da
milisaniyeler) ve **yazarken kullanılabilir** (her tuşta istek patlamadan) yapmak. Tek PR; migration
yalnız genişletme.

## 🔍 Başlangıç durumu (neden)

- Arama `title ILIKE '%…%' OR author ILIKE '%…%'` idi. Canlıda (17 Kasım) "ince memed", "TANPINAR",
  "kurk", "oguz atay", "atay oğuz" ve "dostoyevski suç" **sıfır sonuç** veriyordu: büyük/küçük harf
  eşlemesi veritabanının yerel ayarına bağlı, aksan ve kelime sırası hiç eşleşmiyor, sorgu iki alana
  yayılamıyor.
- Baştaki `%` yüzünden index kullanılamıyor: 500 bin kitaplık deneyde arama başına yaklaşık 2 sn.
- Katalog sayfası zaten yazarken arıyor (300 ms bekleme, eski cevap yok sayılıyor) ama en az harf
  sınırı yok; her adım sunucuda üç ayrı sorgu çalıştırıyor (canlıda sorgu başına ~13 ms ağ).
- İstemci isteği yarıda bırakınca API'nin veritabanı bağlantısı kapanıyordu (F21): yazarken aramada
  bu sık olur.

## 🧠 Yeni kavram(lar)

- **Katlama (folding):** aramada "aynı" sayılacak yazımları tek biçime indirmek. `search_fold`: Unicode
  ayrıştırma (NFKD) → birleşen işaretleri at → `I` ve `ı`'yı `i` yap → küçük harf → noktalamayı boşluk
  yap. Belge de sorgu da aynı fonksiyondan geçer. `lower()` tek başına yetmez: `I`'nın küçüğü dile
  göre `i` ya da `ı`'dır ve veritabanının yerel ayarı bunu sessizce belirler.
- **Tam metin araması ve GIN:** belge kelimelere ayrılır (`tsvector`); GIN index'i "bu kelimeyi içeren
  belgeler" sorusunu tablonun tamamını okumadan cevaplar. `'sav':*` biçimindeki **önek** araması aynı
  index'te o önekle başlayan bütün kelimeleri gezer: önek kısaldıkça pahalılaşır.
- **Türetilmiş arama belgesi:** kitap başına bir satırlık dar tablo (`book_search`), tetikleyicilerle
  güncel tutulur. Kaynak yine `books` ve `editions`; belge her zaman onlardan yeniden kurulabilir.
- **Hazırlanmış deyim ve plan önbelleği:** Postgres bir deyimi birkaç kez çalıştırdıktan sonra
  parametreden bağımsız bir plana geçebilir. O planda sorgu metnini işleyen fonksiyonlar satır başına
  çalışır; tek satırlık `MATERIALIZED` bir ara sorgu onları bir kez çalıştırır.
- **Yazarken arama:** bekleme süresi (debounce) isteği geciktirir ama yavaş yazan birinde her tuş yine
  bir istek olur; en pahalı istekleri **en az harf** kuralı eler.
- **İptalden ayrılmış sorgu:** istemcinin gitmesi, milisaniyeler süren bir okumayı yarıda kesmeye
  değmez; kesmek bağlantıyı da götürür.

## 📏 Ölçümler (500 bin kitap, yerel Postgres 16, hazırlanmış deyim, ortanca)

| Sorgu | Önce (ILIKE) | Sonra |
| --- | --- | --- |
| İki kelime ("ince memed") | ~2 sn | 1,0 ms |
| Üç harfli önek ("sav", 3.518 eşleşme) | ~2 sn | 6,6 ms |
| Yaygın kelime ("ahmet", 22 bin eşleşme), en yeni / puana göre | ~2 sn | 27 / 30 ms |
| İki harfli önek ("sa", 49 bin eşleşme) | ~2 sn | 82 ms |
| Migration: ilk doldurma + index | | 22 sn + 2 sn |

Yazarken arama benzetimi (168.960 kişilik yazma hızı verisi; arama başına ortalama): her tuşta istek
13,1 istek ve 105 ms veritabanı süresi; 300 ms bekleme 4,2 ve 28 ms; buna en az 3 harf ve "yeni
kelimenin ilk harfinde gönderme" eklenince 3,3 istek ve ~9 ms.

Yarıda kesilen istekler (yerel yığın, NGINX üzerinden): önce 20 istekte 19 yeni veritabanı bağlantısı
ve 20 ERROR satırı; sonra 40 istekte 4 bağlantı (yalnız eşzamanlılık için havuz büyümesi), 0 ERROR,
logda 499.

## 🎨 Kararlar

| Konu | Karar | Neden |
| --- | --- | --- |
| Eşleşme | Kelimeler VE'lenir, sıra önemsiz; **son kelime önek**; tek kelimelik sorguda önek en az 2 harfte | Kullanıcı son kelimeyi hâlâ yazıyor olabilir. Tek harfin öneki kataloğun büyük bölümüyle eşleşir (ortanca 154 ms), tam kelime araması 0,1 ms. |
| Belgenin içeriği | Başlık (A), yazar (B), baskı adları ve çevirmenler (C). Yayınevi ve ISBN yok | Yayınevi kelimeleri belgelerin yarısından çoğunda geçiyor; ISBN her baskıda ayrı bir kelime olup index'i üç kat büyütüyor. |
| Belgenin yeri | Ayrı, dar tablo (`book_search`); `books`'ta kolon değil | Geniş sorgularda 2,5–4 kat hızlı (eşleşme başına geniş `books` sayfası okunmuyor). Baskı adları `books`'ta üretilmiş kolon olamaz. |
| Güncel tutma | `books` ve `editions` tetikleyicileri | Hangi yoldan yazılırsa yazılsın (API, seed, elle SQL) belge güncel kalır; eski kod da yayın penceresinde belgeyi bozmaz. |
| Sıra | Varsayılan **ilgi sırası**: başlık aynı > başlıkla başlıyor > kademe (başlıkta / başlık + yazarda / baskıda) > oy sayısı. `sort=newest` ve `sort=rating` de geçerli | `ts_rank` denendi: kelime tekrarını ödüllendiriyor, başka baskıdaki başlık tekrarı çok okunan kitabın önüne geçiyor. |
| Cevap | Liste ile aynı biçim; sayfa, toplam ve türler **tek sorguda** | Yazarken arama bu ucu sık çağırıyor; canlıda her sorgu ayrı bir gidiş-dönüş (F20). |
| Küçük harf | ICU'nun dilden bağımsız kuralı (`und-x-icu`) | Sonuç veritabanının yerel ayarına bağlı olmasın. Harmanlama yoksa migration durur, kod çıkmaz. |
| Yazarken arama | 300 ms bekleme aynı; tek kelimede en az **3 harf**, çok kelimede son kelime en az 2 harf; Enter her uzunlukta arar; kısa metinde ipucu satırı | En pahalı aşama 2 harfli önek (ortanca 29 ms, en kötü 180 ms; 3 harf 7,6 ms). Open Library ve Discourse da 3 harf kullanıyor. |
| İstek iptali | Sunucu liste ve arama sorgularını istemci iptalinden ayırır (5 sn sınır); giden istemciye gövde yazılmaz, loga 499 düşer | F21. Arayüze `AbortController` eklenmedi: eski cevap zaten yok sayılıyor ve sunucu iptali dinlemediği için kazancı yok. |
| Hız sınırı | NGINX'te `GET /api/books` için ziyaretçi başına 10 istek/sn, anında 30; anahtar `X-Envoy-External-Address` | Redis'e bağlanmadı: Upstash ücretsiz planı ayda 500 bin komut ve bugünkü sınırlayıcı istek başına en az 3 komut harcıyor. Başlık yoksa (yerel) sınır uygulanmaz. |
| 429 gövdesi | NGINX'in kendi 429'u API'nin hata zarfıyla döner (üç sınırlı uçta) | HTML gövdede ve HTTP/2'de boş durum metninde arayüz boş bir hata kutusu gösteriyordu (F34). |
| Erişim logu | NGINX yolu sorgusuz ve referrer'sız yazar (`log_format bookhub`); kural `checks.sh`'ta | Yazarken aramada yazılan her önek loglanırdı. CLAUDE.md'deki "bilinen açık" kapanır. |
| Geçersiz metin | `q` geçerli UTF-8 değilse ya da NUL içeriyorsa 400 | Veritabanına ulaşınca 500 ve ERROR satırı üretiyordu (F30; bu iş yalnız `q`'yu kapatır). |
| Kitaplığım araması | Katalogla aynı kural (`book_search`) | Aynı metin iki ekranda farklı sonuç vermesin. Kitaplıkta arama Enter ile; yazarken arama yok. |

## 🚫 Kapsam dışı

- **Yazım hatası toleransı** ("dostoyevsky" → Dostoyevski): `pg_trgm` ile ölçüldü (6–25 ms), ama uzantının
  canlıda kurulabildiği doğrulanmadı. Ayrı iş.
- **Üst çubukta öneri listesi** (her sayfada açılan, klavyeyle gezilen liste): aynı sunucu ucunu kullanır;
  ayrı iş.
- **Önbellek katmanları:** `Cache-Control`, NGINX ortak önbelleği ve 1–3 harflik önekler için hazır tablo
  (58 sn'de kuruluyor, okuma 0,08 ms). Bugünkü trafikte gereksiz; ihtiyaç ölçülünce.
- **ISBN ile arama** ([CAT-03](../FEATURE_REQUESTS.md)).
- **Tür süzgecinin planı ve her istekteki sayım** (F24, F25): arama yolunu etkilemiyor; ayrı iş.
- **Diğer metin girdilerinde geçersiz bayt denetimi** (F30'un kalanı) ve genel iptal davranışı (F21'in
  kalanı).

## 🪜 Adımlar

1. **`0014_book_search.sql` (genişletme):** `search_fold`, `search_tsquery`, `book_search` tablosu,
   `refresh_book_search`, iki tetikleyici, ilk doldurma, GIN index. Önce tetikleyiciler sonra doldurma:
   arada kaçan yazım olmaz. `SET LOCAL lock_timeout = '5s'` (F38). Down bütün nesneleri kaldırır.
2. **Sorgular:** `SearchBooks` (yeni); `ListBooks`, `ListBooksByRating`, `CountBooks` artık metin
   aramaz; `ListLibrary` aramayı `book_search` üzerinden yapar. `sqlc generate`.
3. **Go:** `books.Repository.Search`; liste ucu `?q=` varsa aramaya gider; `detached` (iptalden ayrılmış
   context); `validator.text`; `EscapeLike` silindi.
4. **Arayüz:** `autoSearchable` ([filters.tsx](../../frontend/src/components/filters.tsx)), ipucu satırı,
   aramada "En ilgili / En yeni / En yüksek puan".
5. **NGINX:** `zone=catalog`, `@rate_limited`, `log_format bookhub`; kural ve testi `scripts/checks.sh`'ta.

## ✅ Doğrulama (kabul kriteri → kanıt)

| # | Kriter | Kanıt |
| --- | --- | --- |
| 1 | "ince memed", "İNCE MEMED", "INCE MEMED", "kurk mantolu", "atay oğuz", "dostoyevski suç" doğru kitabı bulur | `TestSearchMatchingRealDB`, `TestMigration0014BuildsSearchDocuments`; yerel yığında uçtan uca |
| 2 | Yayında olmayan kitap aramada dönmez; onaylanınca döner, arşivlenince kaybolur; başlık düzeltmesi eski adı aramadan çıkarır | `TestSearchFollowsWritesRealDB` |
| 3 | Varsayılan sıra ilgi sırasıdır ve kademeler oy sayısından önce gelir; `newest` ve `rating` çalışır | `TestSearchOrderRealDB` |
| 4 | Arama cevabı listeyle aynı biçimdedir, türleri taşır, tek sorguda gelir | `TestListBooksSearchBranch`, `TestRepositorySearchArguments` |
| 5 | Toplam sayfadan bağımsızdır; sonun ötesindeki sayfada da doğrudur | `TestSearchOrderRealDB` (sayfalama) |
| 6 | Kitap ve baskı yazımları belgeye yansır; dolu veritabanında 0013 → 0014 çalışır; Down temizler | `TestMigration0014BuildsSearchDocuments` |
| 7 | Geçersiz UTF-8 ya da NUL içeren `q` 400 alır, sorguya ulaşmaz | `TestListBooksRejectsInvalidTextBeforeQuery`, `TestListLibrary` |
| 8 | İstemci giderse sorgu iptal edilmez; gövde yazılmaz, 499 loglanır | `TestListBooksQueryOutlivesClient`; yerel yığın ölçümü (yukarıda) |
| 9 | Tek kelimede 3 harften kısa metin kendiliğinden aranmaz ve ipucu görünür; Enter arar; çok kelimede yeni kelimenin ilk harfinde aranmaz | `Home.test.tsx` "yazarken arama kuralları", `filters.test.ts` |
| 10 | Aramada sıra seçenekleri: En ilgili (varsayılan), En yeni, En yüksek puan | `Home.test.tsx` "aramada sıra" |
| 11 | Katalog ucu ziyaretçi başına sınırlıdır; 429 hata zarfıyla ve `Retry-After` ile döner; başlık yoksa sınır yoktur | Yerel yığın: aynı ziyaretçiden 60 istekte 39 × 200 ve 21 × 429; başlıksız 60 × 200. Otomatik test yok |
| 12 | NGINX erişim logunda sorgu dizesi ve referrer yoktur | `scripts/checks_test.sh` "NGINX erişim logu"; yerel yığında sorgulu isteklerin logunda sorgu yok |

Koşulanlar (17 Kasım, yerel): `go test -race ./...` gerçek Postgres ve Redis ile bütün paketler yeşil;
`golangci-lint` 0 bulgu; `tsc` ve vitest (374 test) yeşil; Playwright 15/15; `checks_test.sh` 49/49.

## 🚀 Yayın ve yayından sonra doğrulama

- Migration koddan önce uygulanır ve eski kodla uyumludur: eski kod yeni nesneleri anmaz, yazımları
  tetikleyicileri çalıştırır. Geri dönüş yalnız kodun geri alınmasıdır; tablo zararsız kalır.
- Canlıda bakılacaklar (yalnız okuma):
  1. Deploy logunda 0014 uygulandı; `und-x-icu` denetimi geçti.
  2. "ince memed", "kurk", "atay oğuz" aramaları sonuç veriyor.
  3. Aynı adresten art arda 45 `GET /api/books` isteğinde 429 görülüyor (başlık geliyor mu?) ve başka
     bir ağdan gelen istek etkilenmiyor.
  4. NGINX logunda `?q=` yok; API logunda yarıda kesilen isteklerde ERROR yok.

## ⚠️ Doğrulanmayanlar

- **Canlı veritabanında `und-x-icu`:** 0010'daki `tr-x-icu` denetimi canlıda geçti, yani ICU var; bu
  harmanlamanın adıyla bulunduğu canlıda denenmedi. Yoksa migration durur ve kod çıkmaz.
- **`X-Envoy-External-Address`:** Northflank belgesine dayanıyor. Başlık gelmezse sınır hiç uygulanmaz
  (bugünkü durum); gelirse ziyaretçi başına çalışır.
- **Ölçüm verisi sentetik** (Türkçe sözlük ve nadir kelime kuyruğu); gerçek katalogda dağılım farklı olabilir.
- **500 bin kitapta ilk doldurma** 22 sn boyunca kitap ve baskı yazımlarını bekletir; canlıdaki 54 kitapta
  milisaniyeler. O ölçekte doldurma ayrı ve parçalı yapılmalı.
