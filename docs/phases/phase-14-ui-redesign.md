# Faz 14 — Arayüz yeniden tasarımı (Tailwind + shadcn/ui)

> Durum: ✅ Faz 14 tamam — A–C ve sıralama canlıda (PR #21, takipler #25), gerçek kapaklar
> [#34](https://github.com/mtgezgin/BookHub/issues/34) ile (2026-10-12) · İlgili: [frontend/src](../../frontend/src),
> [styles.css](../../frontend/src/styles.css)

## 🎯 Amaç

Arayüzü "çalışan bir ödev" görünümünden, birine gösterildiğinde gerçek bir ürün gibi duran bir
hâle getirmek. **İşlev aynı kalır:** yeni özellik yok, API değişmez (Faz D hariç).

## 🔍 Başlangıç durumu (neden)

Canlı sitenin ekran görüntülerinden ve koddan (2026-10-10):

- Görsel kimlik yok: 53 kitabın hepsinde aynı 📖 yer tutucusu (demo veride `cover_url` boş).
- Hiyerarşi yok: başlık, bağlantı ve düğme aynı mor; tek font; bütün stil 61 satırlık tek CSS.
- Ara durumlar yok: yüklenirken boş ekran, "Sonuç yok." + iki pasif düğme, tek satırlık 404.
- Geliştirici dili kullanıcıya sızıyor ("C = 8.12, m = 10", "durum: PENDING", `window.prompt`).
- Bitmemişlik: favicon yok (`/favicon.ico` index.html dönüyor), ondalık nokta ("9.13"), dil
  kodu ham ("en").

## 🧠 Yeni kavram(lar)

- **Utility-first CSS (Tailwind):** stil, bileşenin yanında küçük sınıflarla yazılır; ayrı bir
  CSS dosyasında sınıf adı uydurmak gerekmez. Kullanılmayan sınıf pakete girmez.
- **Tasarım token'ları:** renk, boşluk, köşe yuvarlaklığı gibi kararlar tek yerde CSS
  değişkeni olarak durur; bileşenler `bg-primary` gibi **anlamlı** adlar kullanır, ham renk
  değil. Tema (açık/koyu) yalnız token değerlerini değiştirir.
- **shadcn/ui yaklaşımı:** bir paket değil, repoya **kopyalanan** bileşen kodu. Erişilebilirlik
  gerektiren parçalar (dialog, menü) Radix'in davranış katmanının üstünde; görünüm bizim.
- **Varyant (cva):** bir bileşenin görünüş seçenekleri (birincil / ikincil / tehlike) tek bir
  tabloda tanımlanır; sayfalar `variant="destructive"` der, sınıf listesi tekrarlanmaz.

## 🎨 Kararlar

| Konu | Karar | Neden |
|---|---|---|
| Stil yaklaşımı | Tailwind CSS v4 (`@tailwindcss/vite`) + shadcn/ui yaklaşımı (Radix) + `lucide-react` ikonları | Kullanıcı seçimi (2026-10-10). Hızlı ve tutarlı görünüm, erişilebilir hazır davranışlar, endüstride yaygın yığın. `@tailwindcss/vite` Vite 8'i destekliyor (peer: `^5.2 \|\| … \|\| ^8`). |
| Görsel yön | **"Sıcak kütüphane":** kâğıt tonunda arka plan, mürekkep rengi metin, başlıklarda serif, vurgu rengi koyu "kütüphane yeşili", puan yıldızları kehribar | Kitap sitesine kişilik verir; mevcut krem arka planla süreklilik. Token'lar tek dosyada → beğenilmezse değiştirmek ucuz. Faz A sonunda ekran görüntüleriyle gözden geçirilir. |
| Fontlar | Başlık: **Fraunces** (serif), metin: **Inter** — `@fontsource-variable/*` ile pakete girer, kendi sunucumuzdan | CSP'de `font-src` yok → `default-src 'self'` geçerli, Google Fonts CDN engellenir. Türkçe harfler (ğ, ş, ı, İ) `latin-ext` alt kümesinde; `unicode-range` sayesinde tarayıcı yalnız gerekeni indirir. |
| Tema | Açık + koyu, **sistem tercihine göre** (`prefers-color-scheme`) | Elle seçim düğmesi Faz C'de. |
| Kontrast | WCAG AA: metin ≥ 4.5:1, büyük metin ve arayüz öğeleri ≥ 3:1 | Token çiftleri bir betikle ölçülür (aşağıda). |
| Kapak | `cover_url` varsa resim (yüklenemezse üretilmiş kapağa düşer); yoksa **üretilmiş kapak**: başlık + yazardan türetilen sabit renk, çerçeve, serif başlık; 2:3 oran | Backend'e dokunmadan görsel kimlik. Gerçek kapaklar Faz D. Kapak dekoratiftir (`aria-hidden`); başlık metni kartta zaten var. |
| Bileşen yeri | Kit: `src/components/ui/` (shadcn geleneği) · `cn()`: `src/lib/utils.ts` · mevcut `components/ui.tsx` → `components/common.tsx` | `ui.tsx` dosyası ile `ui/` klasörü yan yana olunca hangisinin içe aktarıldığı belirsizleşir. |
| İçe aktarma | `@/` takma adı eklenmiyor; göreli import | Repo göreli import kullanıyor. shadcn CLI ve `components.json` Faz C'de, Dialog/Menu/Toast gerektiğinde. |
| Geçiş | Eski CSS **tamamen** kaldırılır; bütün sayfalar Faz A'da kite taşınır (yerleşimleri aynı kalır) | Eski ve yeni CSS bir arada yaşarsa eski `button {…}` kuralları yeni düğmelere sızar. Sayfalar küçük (~800 satır TSX). |
| Testler | Sınıf seçicileri (`.grid`, `.success`, `nav.topbar`, `.score-picker` …) yerine rol ve etiket | Görünüm değişince kırılmazlar; erişilebilirliği de zorlarlar. |

## 🗺️ Faz 14'ün bölümleri

| Bölüm | Kapsam | Durum |
|---|---|---|
| **A — Temel** | Token'lar, tipografi, bileşen kiti, üretilmiş kapak + yeni kitap kartı, yükleme / boş / 404 durumları, marka (logo, favicon); bütün sayfalar yeni kitle, yerleşim aynı | ✅ canlıda (PR #21) |
| **B — Ana sayfa + detay** | Karşılama alanı, tür çipleri, kapaklı "en sevilenler" şeridi, yazarken arama; detayda iki sütun (kapak + puan kartı), yıldızlı puan verme, yorum deneyimi, "aynı türden" şeridi | ✅ canlıda (PR #21) |
| **C — Formlar, gezinme, admin, erişilebilirlik** | Parola göster/gizle, girişten sonra geldiği sayfaya dönüş, kullanıcı menüsü + mobil menü, moderasyonda `window.prompt` yerine Dialog, toast'lar, sayfa başlıkları, tema düğmesi, erişilebilirlik turu | ✅ canlıda (PR #21; moderasyon koruması #25) |
| **D — Backend isteyenler** | Katalog sıralaması (bugün yalnız en yeni: `books.sql:47`), gerçek kapaklar (ISBN + Open Library; CSP `img-src https:` izin veriyor) | ✅ sıralama ve gerçek kapaklar ([#34](https://github.com/mtgezgin/BookHub/issues/34)) |

## 🪜 Faz A — adımlar

1. **Kurulum:** bağımlılıklar (`tailwindcss`, `@tailwindcss/vite`, `class-variance-authority`,
   `clsx`, `tailwind-merge`, `radix-ui`, `lucide-react`, iki font paketi); Vite eklentisi;
   `styles.css` → Tailwind + token'lar (açık/koyu); `cn()`.
2. **Kit:** Button, Input, Textarea, Label, Badge, Card, Skeleton, Alert (+ yerel `<select>`
   için aynı görünüm).
3. **Kapak + kart + puan:** `BookCover` (renk paleti, resim yüklenemezse üretilmiş kapak),
   `BookCard` (dikey; bütün kart tıklanır, tür rozetleri ayrı bağlantı), `Rating` (kehribar
   yıldız, Türkçe biçim: "8,1").
4. **Kabuk:** başlık çubuğu (marka işareti, aktif sayfa vurgusu), SVG favicon, meta açıklama,
   `theme-color`.
5. **Durumlar:** iskelet kart ızgarası, `EmptyState`, `NotFound` (bilinmeyen rota + bulunamayan
   kitap), detay sayfası yükleme iskeleti.
6. **Sayfalar kite taşınır:** Home, BookDetail, Auth, Suggest, Account, Moderation — davranış ve
   yerleşim aynı. Dil kodu görünen ada çevrilir (`Intl.DisplayNames`: "en" → "İngilizce").
7. **Testler:** birim testleri yeni yapıya (rol/etiket); yeni testler (kapak rengi sabit mi,
   Türkçe puan biçimi, boş ve 404 durumları); e2e seçicileri.
8. **Doğrulama:** aşağıdaki liste.

## 🪜 Faz B — adımlar (ana sayfa + detay)

**Ana sayfa**

1. **Karşılama alanı:** görünür başlık ("Bir sonraki kitabını bul"), tek cümlelik tanıtım, büyük
   arama kutusu.
2. **Yazarken arama:** yazmayı 300 ms bırakınca URL'deki `q` güncellenir; Enter ve "Ara" hemen
   arar. URL güncellemesi fonksiyon biçiminde (`setParams(prev => …)`): gecikmeli güncelleme
   arada seçilen türü eski parametrelerle ezmesin.
3. **Tür çipleri:** açılır liste yerine "Tümü" + her tür için basılı/basılı değil düğme
   (`aria-pressed`); mobilde yatay kaydırma.
4. **"Okurların en sevdikleri" şeridi:** kapaklı, sıralı 5 kitap; yan sütun kalkar, şerit mobilde
   de en üstte görünür. Yalnız filtresiz ilk sayfada çizilir (aramada gürültü). Bayes açıklaması
   "Nasıl sıralanıyor?" açılır kutusunda (`<details>`, JS'siz erişilebilir).
5. Izgara tam genişlik: 2 / 3 / 4 / 6 sütun (12'lik sayfa her satırı doldurur).

**Kitap detayı**

1. **İki sütun (md+):** solda büyük kapak ve altında puan kartı, sağda başlık, açıklama ve
   yorumlar. Mobilde sıra: küçük kapak + başlık, puan kartı, açıklama, yorumlar (`grid-template-areas`).
2. **Yıldızlı puan verme:** 10 yıldız düğmesi; üstüne gelince/odaklanınca önizleme, seçili olan
   `aria-pressed`. Düğmenin erişilebilir adı puanın kendisi (ekran okuyucuya özel metin) → e2e
   ve birim testleri aynı adla bulur.
3. **Yorumlar:** baş harf avatarı, göreli zaman ("3 gün önce", üzerine gelince tam tarih),
   karakter sayacı, gönderilirken düğme kilidi (çift gönderim yok).
4. **"Aynı türden" şeridi:** ilk türden en fazla 6 kitap (mevcut kitap hariç). Bu istek
   başarısız olursa sayfa hata göstermez; şerit çizilmez (yardımcı içerik).

## 🪜 Faz C — adımlar (formlar, gezinme, admin, erişilebilirlik)

1. **Bildirimler (toast):** küçük bir sağlayıcı + `useToast()`; puan, yorum ve moderasyon
   işlemlerinden sonra kısa onay. Bölge `aria-live="polite"` ama `role="status"` DEĞİL: e2e'nin
   `getByRole("status")` sorgusu (başarı mesajları) iki öğe bulup patlamasın. Sağlayıcı yoksa
   `useToast()` hiçbir şey yapmaz → sayfa testleri sağlayıcı kurmak zorunda kalmaz.
2. **Sayfa başlıkları:** "Huzur — BookHub". `document.title` ile; React 19'un `<title>` taşıması
   işe yaramaz, çünkü index.html'deki `<title>` önce gelir ve tarayıcı ilkini okur.
3. **Formlar:** parola göster/gizle (`aria-pressed`; düğme etiketin İÇİNDE değil, çünkü `<label>`
   ikinci bir etiketlenebilir öğe içeremez), gönderilirken düğme kilidi ve "…yapılıyor" metni.
4. **Girişten sonra geldiği sayfaya dönüş:** korumalı sayfa ve "giriş yap" bağlantısı geldiği
   adresi router `state`'iyle taşır; giriş sonrası oraya gidilir (yalnız uygulama içi yol).
5. **Gezinme:** kullanıcı menüsü (açılır panel, "disclosure" kalıbı: site gezintisi için ARIA
   `menu` rolünden daha doğru) — Hesabım, Çıkış; dar ekranda Kitap öner ve Moderasyon da menüde.
   Esc ve dışarı tıklama kapatır.
6. **Tema düğmesi:** açık/koyu. Seçim `localStorage`'da; ilk boyamadan ÖNCE `public/theme-init.js`
   uygular (CSP satır içi betiğe izin vermiyor → harici, eşzamanlı küçük dosya). Seçim yoksa
   sistem tercihi. Koyu token'lar tek blokta: `:root[data-theme="dark"]`.
7. **Moderasyon:** `window.prompt` yerine erişilebilir Dialog (Radix): odak hapsi, Esc, başlık.
8. **Erişilebilirlik turu:** "İçeriğe atla" bağlantısı; `prefers-reduced-motion`'da kart
   hareketleri kapalı.

## 🪜 Faz D — adımlar (backend isteyenler)

**Sıralama (yapıldı):** `GET /api/books?sort=rating` — "En yüksek puan", top-rated ile AYNI
ağırlıklı (Bayes) skor. Basit ortalama olsaydı 1 oyla 10 alan kitap, 40 oyla 9 alanın önüne
geçerdi; Faz 6'nın çözdüğü sorun kataloğa geri gelirdi.

- Ayrı sorgu (`ListBooksByRating`), `ORDER BY CASE …` değil: varsayılan "en yeni" sırası
  `created_at` indeksini kullanmaya devam etsin.
- C (genel ortalama) ayrı bir sorgudan (`GlobalAverageRating`) gelir ve `ListBooksByRating`'e
  parametre olarak verilir: top-rated ile aynı formül tek yerde (PR #25; ilk sürümde sorgunun
  içinde bir kopyası vardı). m config'ten (`RATING_MIN_VOTES`), oy yokken C'nin yerine geçen değer
  `ratings.ColdStartMean`. İkisini HTTP katmanı verir: `books`, `ratings`'i içe aktaramaz (ratings
  books'u aktarıyor).
- Oysuz kitap `NULL` → en sonda. `CASE`, `RATING_MIN_VOTES=0` iken oysuz kitapta 0/0
  bölmesini de önler.
- **A–Z sırası bilerek yok:** Türkçe doğru sıra (Ç, Ö, Ş yerinde) ICU harmanlaması
  (`COLLATE "tr-x-icu"`) ister; canlı Supabase'de varlığını doğrulayamadım, yoksa istek düşerdi.
- Arayüz: sonuç başlığında "En yeni / En yüksek puan" anahtarı (`aria-pressed`); puan
  sırasında "en sevilenler" şeridi gizli (listenin başıyla aynı olurdu).

**Gerçek kapaklar (yapıldı, [#34](https://github.com/mtgezgin/BookHub/issues/34)):** kod tarafı
zaten hazırdı (`cover_url` doluysa resim, yüklenemezse üretilmiş kapak). Kararlar (kullanıcı,
2026-10-12):

1. **Kaynak: Open Library'ye bağlantı**, kopya tutmuyoruz. Kapak telifli bir eser; bağlantıyla
   gösterirken kopyalayıp yeniden dağıtmış olmayız. Kapak *kimliğiyle* adresler
   (`covers.openlibrary.org/b/id/<id>-L.jpg?default=false`) hız sınırına girmez; `default=false`
   sayesinde kalkmış bir kapak boş yer tutucu yerine 404 döner ve üretilmiş kapak görünür. Open
   Library'nin arşiv sunucusu ara sıra yavaş: o sırada kart bir süre boş zeminle bekler, hata dönerse
   üretilmiş kapağa düşer. Resim isteği ziyaretçinin tarayıcısından Open Library'ye gider (IP'si
   onlara görünür); hangi sayfada olunduğu gitmez (`referrerPolicy="no-referrer"`).
2. **Baskı: Türkçe baskı varsa onun kapağı, yoksa özgün baskı.** Demo seed'in 54 kitabından 53'ü
   (moderasyonda bekleyen *Simyacı* dahil) gözle doğrulandı: arşiv taramasından gelen bazı "kapaklar"
   iç sayfaydı (künye, önsöz) ya da serinin başka kitabıydı; beşi elle değiştirildi. *Tehlikeli
   Oyunlar*'ın Open Library'de kapağı yok, üretilmiş kapak kalıyor.

Veri `0009_demo_book_covers.sql` ile gelir: slug + yazar eşleşen kitapta yalnız boş `cover_url`'i
doldurur (admin'in koyduğu kapak ezilmez), canlıya Deploy uygular; yerelde `make demo-seed`, ilk
kurulumda [DEPLOYMENT.md](../DEPLOYMENT.md) aynı dosyayı seed'den sonra yeniden çalıştırır. Bu yüzden
dosyada Down bölümü yok. `migrations_test.go` her satırın seed'deki bir kitaba (slug + yazar)
denk geldiğini, seed'deki her kitabın ya kapağı ya da gerekçeli bir istisnası olduğunu ve adreslerin
biçimini denetler. Yayından sonra "en sevilenler" önbelleği (~60 sn) dolana kadar o şeritte eski
(kapaksız) kayıtlar görünebilir. Sıradaki adım: eser/baskı ayrımı ve baskı başına kapak
([#36](https://github.com/mtgezgin/BookHub/issues/36)).

## ⚠️ Önemli noktalar

- **CSP** ([security-headers.conf](../../frontend/nginx/security-headers.conf)):
  `style-src 'unsafe-inline'` olduğu için Radix'in satır içi stilleri çalışır; font CDN'i yasak;
  üretilmiş kapak resim değil CSS olduğu için `img-src`'ye takılmaz.
- **Bütün kartı tıklanır yapmak:** başlık bağlantısı kartı kaplayan görünmez bir katman
  ("stretched link") açar; içerideki tür bağlantıları bu katmanın **üstüne** (`relative z-10`)
  alınmazsa tıklanamaz.
- **Türkçe büyük harf:** `uppercase` CSS'i `<html lang="tr">` sayesinde "i" → "İ" yapar; `lang`
  kaldırılırsa "Iletişim" gibi hatalar başlar.
- **jsdom CSS işlemez:** birim testleri görünümü doğrulamaz; görünüm ekran görüntüsüyle
  doğrulanır.
- **Ondalık:** sayılar `Intl.NumberFormat("tr-TR")` ile; `toFixed` nokta kullanır.
- **Türkçe heceleme yok:** `hyphens: auto` Chrome'da Türkçe için çalışmadı; kapakta uzun bir
  kelime ("Otostopçunun", "Tutunamayanlar") çerçeveden taşıp kırpılıyor ya da ortadan
  bölünüyordu. Çözüm: punto, en uzun kelime yazı alanına sığacak kadar küçülür
  (`60 / (harf sayısı × 0,58)` cqw, en çok 10) + son çare `overflow-wrap: anywhere` (BookCover.tsx).
- **Gecikmeli arama ve URL:** 300 ms sonra çalışan güncelleme `setParams(prev => …)` ile
  kurulur; render anındaki parametrelerle kurulsaydı arada seçilen türü ezerdi (testte kilitli).
  `replace: true`: her duraklama tarayıcı geçmişine ayrı kayıt eklemesin.
- **Parola alanındaki düğme ve e2e:** "Parolayı göster" düğmesinin adı da "Parola" ile başlıyor;
  Playwright'in `getByLabel("Parola")` sorgusu (alt dize eşleşmesi) iki öğe bulup patlıyordu →
  e2e'de `exact: true`.
- **Moderasyon ayrı parça (lazy):** Radix Dialog ana paketi ~13 kB (gzip) büyütüyordu; yalnız
  admin sayfası kullandığı için `React.lazy` ile ayrıldı → ziyaretçi o kodu indirmez.
- **Fontlar ilk ziyarette ~200 kB:** iki ailenin latin + latin-ext alt kümeleri (Türkçe harfler
  latin-ext'te). Kiril/Yunan dosyaları imajda durur ama `unicode-range` yüzünden indirilmez.
  Sonrasında `/assets/` bir yıl önbellekte (immutable).

## 🧪 Doğrulama (başarı ölçütleri)

- `npm run typecheck`, `npm test`, `npm run build` temiz; paket boyutu önce/sonra yazılı.
- `make e2e` yeşil (gerçek tarayıcı, gerçek NGINX + API).
- Ekran görüntüleri: ana sayfa, detay, giriş, 404 — masaüstü ve mobil, açık ve koyu tema; önce
  (canlı, 2026-10-10) / sonra.
- Token kontrast betiği: bütün metin çiftleri AA'yı geçiyor.
- Lighthouse erişilebilirlik puanı ≥ 95 (ana sayfa, detay, giriş).

## 📏 Ölçümler (Faz A, 2026-10-10)

| Ölçüm | Önce | Sonra |
|---|---|---|
| Lighthouse erişilebilirlik — ana sayfa | 87 (yetersiz kontrast, etiketsiz `<select>`) | **100** |
| Lighthouse erişilebilirlik — giriş | 92 (yalnız renkle ayırt edilen bağlantı) | **100** |
| Lighthouse erişilebilirlik — detay / 404 | — | **100** / **100** |
| Token kontrastı (`node scripts/check-contrast.mjs`) | — | 44/44 çift AA (açık + koyu tema + 8 kapak rengi) |
| Birim testleri (vitest) | 176 | **202** yeşil |
| E2E (`make e2e`, gerçek chromium + NGINX + API) | 11 | **11** yeşil (seçiciler role taşındı) |
| JS paketi (gzip) | 88,6 kB | 104,5 kB |
| CSS (gzip) | 1,4 kB | 6,8 kB |

"Önce" değerleri canlı siteden (aynı arayüz), "sonra" değerleri lokal yığından (`make app-up`).
JS'teki +16 kB'ın çoğu büyük ihtimalle `tailwind-merge`'ten (çıkarım; paket ayrıntılı ölçülmedi).
Ekran görüntüleri sürüm kontrolünde değil (masaüstü/mobil, açık/koyu, girişli/girişsiz
çekildi).

### Faz B (2026-10-10)

| Ölçüm | Sonuç |
|---|---|
| Lighthouse erişilebilirlik — ana sayfa / tür süzülmüş / detay | **100** / **100** / **100** |
| Birim testleri (vitest) | **221** yeşil (Faz A sonunda 202) |
| E2E (`make e2e`) | **11** yeşil |
| JS paketi (gzip) | 105,7 kB (Faz A: 104,5 kB) |

Girişli detay sayfası (yıldızlar, yorum formu) Lighthouse'la ölçülmedi (giriş ister); birim ve
e2e testleriyle doğrulandı.

### Faz C (2026-10-11)

| Ölçüm | Sonuç |
|---|---|
| Lighthouse erişilebilirlik — ana sayfa / detay / giriş / kayıt | **100** / **100** / **100** / **100** |
| Birim testleri (vitest) | **245** yeşil (Faz B sonunda 221) |
| E2E (`make e2e`) | **11** yeşil (çıkış ve "Hesabım" artık kullanıcı menüsünden, red diyalogdan) |
| JS paketi (gzip) | 107,4 kB ana + 14,1 kB moderasyon parçası (lazy olmasaydı 120,4 kB) |
| Tema | Seçim yeniden yüklemede korunuyor (`data-theme`), CSP ihlali yok (tarayıcı konsolu) |

### Faz D — sıralama (2026-10-11)

| Ölçüm | Sonuç |
|---|---|
| Backend (`make test-db`, `-race`) | bütün paketler yeşil; yeni entegrasyon testi gerçek Postgres'te: 40 oy/9 > 1 oy/10, oysuz en sonda, PENDING sızmıyor |
| Birim testleri (vitest) | **248** yeşil |
| E2E (`make e2e`) | **11** yeşil |
| Canlı (lokal) yoklama | `?sort=rating` sırası "en sevilenler" ile aynı; `?sort=title` → 400 |
| Lighthouse erişilebilirlik — `/?sort=rating` | **100** |

## ✅ Tamamlanma (Faz A)

- [x] Eski CSS kaldırıldı; bütün sayfalar kitle çiziliyor.
- [x] Üretilmiş kapak + yeni kart; Türkçe sayı ve dil adları.
- [x] Yükleme, boş ve 404 durumları; favicon.
- [x] Testler rol/etiket tabanlı; vitest + e2e yeşil.
- [x] Doğrulama listesindeki ölçümler yazıldı (yukarıda).
- [x] Kullanıcı incelemesi ve commit — PR #21 (2026-10-12, canlıda), [PROCESS.md](../PROCESS.md).

## ✅ Tamamlanma (Faz B)

- [x] Karşılama alanı, tür çipleri, yazarken arama, "en sevilenler" şeridi; yan sütun kalktı.
- [x] Detay iki sütun; yıldızlı puan verme; yorumlarda avatar, göreli zaman, sayaç, gönderim kilidi.
- [x] "Aynı türden" şeridi (başarısız olursa sessiz).
- [x] vitest + e2e yeşil; Lighthouse 100.
- [x] Kullanıcı incelemesi ve commit — PR #21 (2026-10-12, canlıda).

## ✅ Tamamlanma (Faz C)

- [x] Bildirimler, sayfa başlıkları, parola göster/gizle, gönderim kilitleri.
- [x] Girişten sonra geldiği sayfaya dönüş (yalnız uygulama içi yol).
- [x] Kullanıcı menüsü (mobilde gezinme de içinde), tema düğmesi, "İçeriğe atla", azaltılmış hareket.
- [x] Moderasyonda red diyaloğu; moderasyon sayfası ayrı parça.
- [x] vitest + e2e yeşil; Lighthouse 100.

## ✅ Tamamlanma (Faz D)

- [x] Katalog sıralaması: en yeni / en yüksek puan (ağırlıklı), backend + arayüz + testler.
- [x] Gerçek kapaklar — Open Library bağlantısı, Türkçe baskı varsa o; demo seed'in 54 kitabından 53'ü, gözle doğrulandı (#34).
