# Faz 16d — Sunucu verisi TanStack Query ile

> Durum: 🔍 adım 1–8 tamam (kullanıcı okudu, 2026-11-21); birleşme ve yayından sonra doğrulama sırada · Issue: [#105](https://github.com/mtgezgin/BookHub/issues/105) —
> [ROADMAP](../ROADMAP.md) · Bulgu: [REVIEW_FEEDBACK](../REVIEW_FEEDBACK.md) F33 ·
> İlgili: [client.ts](../../frontend/src/api/client.ts), [usePaged.tsx](../../frontend/src/components/usePaged.tsx),
> [Home.tsx](../../frontend/src/pages/Home.tsx), [BookDetail.tsx](../../frontend/src/pages/BookDetail.tsx)

## 🎯 Amaç

Arayüzün sunucudan veri çekme biçimini tek bir kurala bağlamak. Bugün her sayfa bunu kendi `useEffect`'iyle
yapıyor; yükleniyor/hata durumu, eski cevap koruması ve yeniden yükleme her yerde farklı. TanStack Query bu
işleri tek yerde ve sınanmış bir şekilde yapar.

Bu faz bir **öğrenme fazı**dır: kodu asistan yazar, kullanıcı her adımı okuyup anlayarak ilerler. Bu yüzden
adımlar küçük ve her birinin aşağıda "amaç / neden / sonuç" bölümü var.

## 🧭 Nasıl takip edilecek

1. Asistan bir adımı yazar ama **commit etmez**. Değişiklik çalışma dizininde durur.
2. Asistan bu dosyada o adımın bölümünü doldurur: ne yapıldı, neden, sonuç (ölçüm ya da test), hangi
   dosyanın hangi satırına bakılmalı.
3. Kullanıcı editörde okur:
   - **Ne değişti:** VS Code'da *Source Control* panelinde dosyaya tıklamak, önce/sonra hâlini yan yana gösterir
     (terminalde: `git diff`).
   - **Neden:** kodun içindeki `NEDEN:` yorumları ve bu dosyadaki adım bölümü.
   - **Kendin dene:** her adımda tarayıcıda bakılacak bir şey yazılı (`make dev` ile).
4. Kullanıcı "tamam" deyince adım commit edilir ve sıradakine geçilir. Soru varsa adım commit edilmeden
   konuşulur.

Bütün adımlar aynı dalda (`feat/105-tanstack-query`) birikir ve tek PR olarak birleşir. Her adım kendi başına
çalışan bir uygulama bırakır: testler her adımda yeşildir.

## 🔍 Başlangıç durumu

Test dışı kodda veri çeken yerler:

| Yer | Ne çekiyor | Eski cevap koruması | Önbellek |
|---|---|---|---|
| `Home.tsx` | türler, en sevilenler, katalog sayfası | katalogda var (bayrak) | yok |
| `Library.tsx` | türler, kitaplık sayfası | var | yok (türler Home'dakinden ayrı çekilir) |
| `BookDetail.tsx` | kitap, puan özeti, benzer kitaplar, benim puanım, yorumlar | kısmen | yok |
| `admin/Moderation.tsx` | bekleyen öneriler | `usePaged` (sayaç) | yok |
| `LibraryMenu.tsx`, `useLibraryStatuses.tsx` | kitapların raf durumu | var | yok |
| `usePaged.tsx` | sayfalı liste (yorumlar, moderasyon) | sayaç + ref | yok |

Yazma işlemleri (puan ver, yorum yaz, rafa ekle, onayla…) bittikten sonra ilgili veriyi her sayfa elle yeniden
yüklüyor. Hangi verinin yenileneceğini her seferinde yazan kişi hatırlamak zorunda.

## 📚 Kavramlar (adımlarda ilk geçtikleri yerde ayrıca açıklanacak)

| Kavram | Bir cümleyle |
|---|---|
| **Sorgu (query)** | Sunucudan **okunan** bir veri. `useQuery({ queryKey, queryFn })` ile istenir |
| **Sorgu anahtarı (queryKey)** | Verinin kimliği, ör. `["books", { genre, offset }]`. Aynı anahtar = aynı önbellek kaydı = tek istek |
| **queryFn** | Veriyi gerçekten getiren fonksiyon; bizde `api.books(...)` gibi mevcut çağrılar |
| **Bayat (stale)** | Önbellekteki veri gösterilir ama arka planda tazelenmeye adaydır. `staleTime` bu süreyi belirler |
| **gcTime** | Hiçbir bileşen kullanmayan önbellek kaydının ne kadar süre tutulacağı |
| **Değişiklik (mutation)** | Sunucuda bir şeyi **değiştiren** işlem. `useMutation` ile yapılır |
| **Geçersiz kılma (invalidate)** | "Bu anahtarlarla başlayan veriler artık eski" demek; ekranda olanlar hemen yeniden çekilir |
| **QueryClient** | Önbelleğin kendisi; uygulamada bir tane |

Varsayılan değerler (TanStack belgesine göre; adım 1'de kurulan paketin kaynağından doğrulanacak):
`staleTime` 0, `gcTime` 5 dk, hata alan sorgu 3 kez yeniden denenir, pencereye dönünce bayat sorgular tazelenir.

## 🎨 Kararlar

**Durum (2026-11-19):** kullanıcı "Devam" dedi; adım 1 önerilerle yazıldı. Okurken değiştirmek
istediğin bir karar olursa adım commit edilmeden değiştirilir.

| # | Konu | Öneri | Gerekçe |
|---|---|---|---|
| 1 | Sürüm | `@tanstack/react-query` ve devtools **5.106.0**, tam sürümle sabit | 5.108.0 iki günlüktü; "en az iki haftalık sürüm" kuralıyla 4 Kasım'daki 5.106.0 seçildi |
| 2 | `staleTime` | 30 sn (genel); türler gibi seyrek değişen veride daha uzun | 0 olursa sayfalar arası geçişte her şey yeniden çekilir; önbelleğin faydası azalır. Sunucudaki önbellekler zaten 60 sn |
| 3 | Yeniden deneme | 4xx'te hiç, ağ ve 5xx hatasında 1 kez | 404'ü üç kez denemek anlamsız; `client.ts` 401'de oturumu zaten kendisi yeniliyor |
| 4 | Çıkış | Çıkışta `queryClient.clear()` | Aynı tarayıcıda sonraki kişi öncekinin kitaplığını önbellekten görmesin (gizlilik) |
| 5 | Geliştirici aracı | `@tanstack/react-query-devtools` yalnız `make dev`'de | Önbelleği canlı görmek öğrenmenin en hızlı yolu; üretim paketine girmez (adım 1'de ölçülerek doğrulanır) |
| 6 | İstek iptali | `client.ts`'in `request`'ine isteğe bağlı `AbortSignal` | Sayfadan çıkılınca ya da süzgeç değişince eski istek tarayıcıda da iptal olur. Şart değil; ilk adımlardan sonra karar verilebilir |

## 🪜 Adımlar

Her adım: **amaç**, **neden**, **sonuç** (yazıldıkça doldurulur), **bakılacak yerler**, **kendin dene**.

### Adım 1 — Kurulum, davranış değişmez

- **Amaç:** paketi eklemek, `QueryClient`'ı kararlardaki ayarlarla kurmak, `main.tsx`'te uygulamayı
  `QueryClientProvider` ile sarmak, testler için bir yardımcı (`renderWithQuery`) eklemek.
- **Neden:** sonraki her adım bunlara dayanır. Bu adımda hiçbir sayfa değişmez, böylece "kurulum bir şey
  bozdu mu" sorusu tek başına cevaplanır.
- **Sonuç (2026-11-19):**
  - Varsayılanlar paketin kaynağında doğrulandı (`query-core/build/modern`): `retry` tarayıcıda 3
    (`retryer.js`), `staleTime` 0 (`query.js` `isStaleByTime`), `gcTime` 5 dk (`removable.js`), pencere
    odağında tazeleme açık (`queryObserver.js` `shouldFetchOn`: seçenek `false` değilse ve veri bayatsa).
  - Ana paket: **110,7 → 118,1 KB gzip (+7,4 KB)**. Önceki 10,5 KB'lık ölçüm paketin tamamıydı;
    Vite yalnız kullanılan parçaları alıyor. Sonraki adımlarda kanca kullanıldıkça biraz büyüyebilir;
    sınır 12 KB.
  - Geliştirici aracı üretim paketinde **yok** (`dist/assets` içinde arandı).
  - vitest 385 → 388 (yeni 3 test), `tsc -b` temiz. Davranış değişmedi: hiçbir sayfa henüz sorgu kullanmıyor.
- **Bakılacak yerler (bu sırayla):**
  1. [`frontend/src/lib/queryClient.ts`](../../frontend/src/lib/queryClient.ts): önbelleğin ayarları.
     İki `NEDEN` yorumu: neden 30 sn, neden 4xx'te yeniden deneme yok.
  2. [`frontend/src/main.tsx`](../../frontend/src/main.tsx): `QueryClientProvider` neden en dışta, önbellek
     neden bileşenin dışında kuruluyor, geliştirici aracı neden üretime girmiyor.
  3. [`frontend/src/lib/queryClient.test.ts`](../../frontend/src/lib/queryClient.test.ts): kuralların testi.
  4. [`frontend/src/test/render.tsx`](../../frontend/src/test/render.tsx) `withQuery`: testlerde neden her
     seferinde yeni bir önbellek.
  5. `package.json`: iki yeni satır. `package-lock.json`'daki ~480 satır paketlerin kendi bağımlılık
    kayıtları; okumana gerek yok.
- **Kendin dene:** `make dev` (ya da `cd frontend && npm run dev`), sayfanın sağ altında küçük bir TanStack
  simgesi çıkar. Tıklayınca panel açılır; şimdilik boştur (henüz hiçbir sayfa sorgu kullanmıyor). Adım 2'den
  sonra orada `["genres"]` kaydını göreceksin.

### Adım 2 — Türler: en basit okuma ve paylaşılan önbellek

- **Amaç:** `Home` ve `Library`'deki iki ayrı `api.genres()` çağrısını tek bir `useGenres()` kancasına çevirmek.
- **Neden:** en basit örnek: parametresi yok, seyrek değişiyor, iki sayfada kullanılıyor. Sorgu anahtarı ve
  önbellek paylaşımı burada görülür.
- **Kendin dene:** Ağ sekmesinde ana sayfadan Kitaplığım'a geçince `/api/genres` isteğinin **ikinci kez
  gitmediğini** görmek.
- **Ne yapıldı (2026-11-19, adım 2–4 tek okuma turu: üçü de ana sayfayı ilgilendiriyor):**
  - Yeni dosya [`frontend/src/api/queries.ts`](../../frontend/src/api/queries.ts): bütün okuma kancaları ve
    sorgu anahtarları tek yerde (`queryKeys`). Anahtarlar hiyerarşik: `["books", …]` ile başlayan her şey
    kitap listesidir; adım 6'da moderasyon onayından sonra tek satırla hepsi tazelenecek.
  - `useGenres()`: `staleTime` 5 dk (genel 30 sn yerine; türleri yalnız admin, nadiren değiştirir).
  - `Library.tsx`'te `useState` + `useEffect` yerine tek satır: `useGenres().data ?? []`.
- **Neden anahtarlar ayrı bir nesnede:** bir veri bir yerde okunur, başka bir yerde "geçersiz" ilan edilir. İki
  yer anahtarı ayrı ayrı yazarsa, biri değişince öteki hiçbir hata vermeden boşa düşer.

### Adım 3 — Katalog listesi: parametreli sorgu ve eski cevap koruması

- **Amaç:** `Home`'daki katalog `useEffect`'ini `useQuery` ile değiştirmek; anahtar süzgeçleri içerir
  (`["books", { q, genre, sort, offset }]`). Sayfa değişirken önceki sayfa ekranda kalır (`placeholderData`).
- **Neden:** F10'daki "geç gelen eski cevap yenisinin üstüne yazıyor" hatasının sınıfı burada kapanır: her
  cevap kendi anahtarına yazılır, ekran yalnız güncel anahtarı gösterir. Elle yazılmış bayrak silinir.
- **Sonuç:** eski cevap yarışı için bir test (yavaş ilk istek, hızlı ikinci istek; ekranda ikincisi kalır).
- **Kendin dene:** bir türe girip geri dönünce katalog **anında** görünür (önbellekten), sonra arka planda tazelenir.
- **Ne yapıldı:** `Home.tsx`'teki üç `useState` (sayfa, hata, yükleniyor) ve `current` bayraklı `useEffect`
  gitti; yerine `useBooks({ q, genre, limit, offset, sort })`.
  - `listError` artık `books.error`: hata anahtara bağlı olduğu için filtre değişince eski hata
    kendiliğinden kalkar (eskiden `setListError(null)` ile elle siliniyordu).
  - `loading` artık `books.isPlaceholderData`: yeni anahtarın cevabı beklenirken önceki sayfa soluk
    görünür. Bir fark var: aynı anahtarın arka planda tazelenmesi (pencereye dönünce) listeyi
    **soluklaştırmaz**. Eskiden her istek soluklaştırıyordu; bu bilinçli bir iyileştirme.
- **Sonuç:** spec'in istediği "eski cevap yarışı testi" zaten vardı ve yeni kod da ondan geçiyor:
  `Home.test.tsx` "geç gelen ESKİ liste cevabı yeni filtrenin üstüne YAZMAZ" ve "geç gelen ESKİ isteğin
  HATASI da yutulur". İki testin ikisi de elle yazılmış bayrağı sınıyordu; şimdi aynı davranışı
  TanStack'in anahtar mantığı sağlıyor. Testin kendisi değişmedi.
- **Test altyapısında öğrenilen bir şey:** testler ilk denemede 22 yerde kırıldı. TanStack, önbellek
  değişikliğini bileşenlere `setTimeout(0)` ile bildiriyor (`query-core/notifyManager.js`); testlerin
  `act(async () => {})` beklemesi zamanlayıcıyı beklemiyor. Cevap önbelleğe geliyor, ekrana gelmiyordu.
  Tek tek koşunca geçmesi bu yüzden yanıltıcıydı. Çözüm `test/render.tsx`'te tek satır:
  `notifyManager.setScheduler(queueMicrotask)`. Uygulamanın davranışı aynı, yalnız testteki zamanlama
  değişiyor.

### Adım 4 — En sevilenler şeridi

- **Amaç:** `topRated` çağrısını sorguya çevirmek. Küçük adım; adım 3'teki kalıbın tekrarı.
- **Ne yapıldı:** `useTopRated(5)`. Anahtar `["books", "top-rated", 5]`: şerit de bir kitap listesi,
  `["books"]` geçersiz kılınınca o da tazelenir. Şerit ve tür hatası eskisi gibi ortak kutuda (`error`).
- **Sonuç (adım 2–4 birlikte):**
  - vitest 388/388 (yeni test yok: davranış değişmedi; mevcut 51 Home testi aynen geçiyor).
  - Playwright 20/20 (yerel, gerçek tarayıcı ve gerçek API; katalog, arama ve kitaplık senaryoları dahil).
  - Ana paket: 118,1 → **120,7 KB gzip** (başlangıçtan +10,0 KB; sınır 12 KB). Kalan adımlarda
    `useMutation` eklenecek; sınıra yaklaşırsa ölçüp söyleyeceğim.
  - `Home.tsx`: −2 `useEffect`, −5 `useState`.
- **Bakılacak yerler (bu sırayla):**
  1. [`frontend/src/api/queries.ts`](../../frontend/src/api/queries.ts): yeni dosyanın tamamı; üç kanca ve yorumları.
  2. `frontend/src/pages/Home.tsx`: Source Control'de sol (eski) ve sağ (yeni) hâli yan yana. 34–68. satırların
     yerine gelen blok asıl değişiklik.
  3. `frontend/src/pages/Library.tsx`: türler kısmı (küçük).
  4. `frontend/src/test/render.tsx`: `setScheduler` satırı ve yorumu.
  5. `Home.test.tsx`, `Library.test.tsx`, `App.test.tsx`: yalnız bileşen `withQuery(…)` ile sarıldı.
- **Kendin dene:** `make dev`, ana sayfa. Sağ alttaki TanStack panelinde üç kayıt: `["genres"]`,
  `["books","top-rated",5]`, `["books","list",{…}]`. Bir türe tıkla: yeni bir `list` kaydı açılır, eskisi
  durur. "Tümü"ye dön: liste **beklemeden** gelir (önbellekten). Kitaplığım'a geç: Ağ sekmesinde
  `/api/genres` isteği gitmez.

### Adım 5 — Kitap sayfası ve ilk değişiklik (mutation)

- **Amaç:** kitap, puan özeti, benzer kitaplar ve benim puanım sorgu olur. Puan verme ve silme `useMutation`
  olur; başarıdan sonra puan özeti ve benim puanım **geçersiz kılınır**.
- **Neden:** yazma işleminden sonra "neyin yenileneceği" bugün elle yazılıyor. Geçersiz kılmayla bu, verinin
  anahtarına bağlanır. Faz 17'nin inceleme yazma işi bu kalıbı kullanacak.
- **Kendin dene:** puan verince özetin güncellendiğini, DevTools'ta ilgili kaydın "fetching" olup döndüğünü görmek.
- **Ne yapıldı (2026-11-19):**
  - `api/queries.ts`'e dört okuma: `useBook`, `useRatingSummary`, `useMyRating` ve benzer kitaplar için
    mevcut `useBooks` (yeni `enabled` parametresiyle: tür bilinmeden çalışmaz).
  - İlk **değişiklik**: `useRateBook`. `mutate(7)` puan verir, `mutate(null)` kaldırır.
  - `BookDetail.tsx`: dört `useState`, iki `useEffect`, `loadBook` ve iki ayrı `rate`/`unrate` fonksiyonu
    gitti; yerine dört sorgu ve tek `rate(score | null)`.
- **Yeni kavram 1 — puan yoksa hata değil, cevap:** sunucu "bu kitaba puan vermedin"i 404 `not_rated` ile
  söylüyor. `useMyRating`'in `queryFn`'i bunu `null`'a çeviriyor: ekranda hata kutusu çıkmıyor, önbellekte
  "puanı yok" bilgisi de saklanıyor. Eskiden aynı ayrımı sayfa `catch` içinde yapıyordu.
- **Yeni kavram 2 — kullanıcıya özel anahtar:** "benim puanım" `["me", kullanıcıKimliği, "rating", kitapKimliği]`.
  Aynı tarayıcıda başka biri giriş yaparsa anahtar farklı olduğu için öncekinin puanı görünmez. `"me"` öneki
  adım 8'de işe yarayacak: çıkışta bu önekle başlayan her şey tek satırla silinecek.
- **Yeni kavram 3 — değişiklikten sonra ne olur (`onSuccess`):**
  1. "Benim puanım" **doğrudan yazılır** (`setQueryData`): yeni değeri zaten biliyoruz.
  2. Puan özeti **geçersiz kılınır** (`invalidateQueries`): ekranda olduğu için hemen yeniden çekilir.
  3. Bütün kitap listeleri **bayat işaretlenir ama şimdi çekilmez** (`refetchType: "none"`): kartlardaki
     ortalama değişti, ama o listeler şu an ekranda değil. Kataloğa dönünce tazelenir. Bu satır önbelleğin
     getirdiği bir sorumluluk: eski kod kataloğu her açılışta yeniden çektiği için kart hiç eskimiyordu; önbellekle
     bu satır olmasaydı puan verip 30 sn içinde kataloğa dönen kişi kartta eski ortalamayı görürdü.
- **Neden kurallar kancada, sayfada değil:** "puan değişince neyin tazeleneceği" bilgisi verinin yanında durunca,
  puanın verildiği her yer aynı kuralı kendiliğinden uygular. Faz 17'de kitaplık satırından da puan verilecek.
- **Bilerek eklenen bir davranış (F11'in puan yarısı):** kayıt sürerken yıldızlar ve "kaldır" kilitli
  (`rating.isPending`). Eskiden hızlı iki tık iki istek gönderiyordu ve sunucuya hangisinin önce varacağı belli
  değildi. Yeni test: "puan kaydedilirken yıldızlar ve 'kaldır' kilitli…".
- **Sonuç:** vitest 389/389 (mevcut 38 kitap sayfası testi değişmeden geçiyor; özetin yeniden çekildiğini sayan
  iki test dahil), ana paket 121,3 KB gzip (başlangıçtan +10,6 KB; `useMutation` +0,6 KB getirdi).
  Playwright 20/20 (yerel; puan ve yorum senaryosu dahil).
- **Bakılacak yerler (bu sırayla):**
  1. `frontend/src/api/queries.ts`: dosyanın sonundaki dört yeni kanca; özellikle `useRateBook`'un yorumu.
  2. `frontend/src/pages/BookDetail.tsx`: 97–127. satırlar (sorgular ve tek `rate`), hata kutusu (`shownError`),
     yıldızlardaki `disabled`.
  3. `frontend/src/pages/BookDetail.test.tsx`: yeni kilit testi ("puan kaydedilirken…"); geri kalanı yalnız
     `withQuery` sarmalı.
- **Kendin dene:** `make dev`, giriş yap, bir kitap aç. Bir yıldıza tıkla: DevTools panelinde `["ratings","summary",…]`
  bir an "fetching" olur; `["me",…,"rating",…]` hiç istek atmadan yeni değeri alır. Sonra kataloğa dön: liste
  kayıtları "stale" (bayat) işaretli görünür ve ekrana gelenler tazelenir.

### Adım 6 — Yorumlar ve `usePaged`'in yerini almak

- **Amaç:** yorum listesi ve moderasyon kuyruğu sorgu olur; yorum ekleme, silme ve moderasyon işlemleri
  değişiklik olur. `usePaged`'in sayaç ve ref mantığı kalkar; `Pager` bileşeni kalır.
- **Neden:** `usePaged`'in yorumundaki üç kural (boş sayfada geri adım, eski cevap koruması, sayfalayıcının
  görünmesi) sorguyla nasıl karşılanıyor, tek tek gösterilir.
- **Ne yapıldı (2026-11-21):**
  - `api/queries.ts`: `useComments`, `useAddComment`, `useDeleteComment`.
  - Yeni `api/adminQueries.ts`: `useAdminBooks`, `useModerateBook` (onay, red, arşiv, geri yayın tek kancada),
    `useCreateGenre`.
  - `components/usePaged.tsx`: `usePaged` gitti, yerine veri çekmeyen küçük bir `usePageView` geldi; `Pager` aynı.
  - `BookDetail.tsx`: yorumların `useCallback`'i, `posting` durumu ve yerel `error` durumu gitti.
  - `Moderation.tsx`: `reloadRef`, onu güncelleyen etki, `busy` durumu ve sekme sıfırlama etkisi gitti.
- **`usePaged`'in üç kuralı ne oldu:**
  1. **Boş sayfada geri adım — kaldı.** Sunucu "bu sayfa boş" diyebilir; sorgu bunu bilemez. Artık tek bir
     etki: veri boş, offset > 0 ve veri gerçek (yer tutucu değil) ise bir sayfa geri. Boş sayfa bu arada
     çizilmez; "liste boş" ekranı yanıp sönmez.
  2. **Eski cevap koruması — sorguya geçti.** Her sayfa kendi anahtarında (`["comments", kitap, offset]`),
     ekran yalnız güncel anahtarı gösterir. Sıra numarası (`seq` ref'i) gitti. Bunu gösteren test gerçek
     bir `useQuery` ile yazıldı: "geç gelen ESKİ sayfa cevabı yenisinin üzerine yazmaz".
  3. **Sayfalayıcı offset > 0 iken görünür — aynı hesap,** `usePageView`'da.
  - `usePaged`'in ayrıca bir `onError` ref'i (sonsuz istek döngüsüne karşı) ve `resetKey` etkisi vardı. Hata
    artık sorgunun `error`'ında; sekme değişince offset'i sıfırlamak sekmeyi değiştiren tıklamanın işi.
- **Yeni kavram 1 — geçersiz kılma anahtara göre, closure'a göre değil:** eski Moderasyon "tıklandığı
  andaki `reload`" fonksiyonunu tutuyordu; işlem sürerken sekme değişirse ESKİ sekmeyi çekip yenisinin üstüne
  yazıyordu. Bunu bir ref önlüyordu. `invalidateQueries({ queryKey: ["admin", "books"] })` ise "bu önekle
  başlayan hangi kayıt o an ekrandaysa onu yeniden çek" der: sorun kendiliğinden yok.
- **Yeni kavram 2 — `onSuccess` mı `onSettled` mı:**
  - Yorum yazma `onSuccess`'te geçersiz kılar: yazılamadıysa liste değişmemiştir.
  - Yorum silme `onSettled`'da, yani hatada da: "bulunamadı" büyük olasılıkla başka bir admin'in onu zaten
    sildiği anlamına gelir; ekrandaki hayalet satır gitsin. Eski kod da böyleydi.
- **Yeni kavram 3 — bir değişiklik başka sayfaların verisini de eskitir:**
  - Moderasyon işlemi bütün moderasyon sekmelerini VE bütün katalog kayıtlarını (`["books"]`) geçersiz kılar:
    onaylanan kitap kataloğa girer, arşivlenen çıkar.
  - Yeni tür `["genres"]`'i geçersiz kılar. Adım 2'de türlere 5 dk'lık `staleTime` vermiştik; bu satır
    olmasaydı yeni tür katalog filtresinde 5 dk görünmezdi.
  - Bu kurallar ekranda görünmediği için ayrı test dosyasında sınanıyor (`adminQueries.test.tsx`).
- **Neden admin kancaları ayrı dosyada:** Moderasyon tembel yüklenen bir parça. `queries.ts`'i katalog da
  içe aktardığı için oradaki her şey ana pakete girer, yani her ziyaretçiye gider. Ayrı dosyayı yalnız
  Moderasyon içe aktarıyor.
- **Bilerek korunan iki şey:**
  - Moderasyondaki `inFlight` ref'i KALDI. `isPending` bir sonraki çizimde görünür; aynı anda gelen iki
    tıklamanın ikincisi düğme henüz pasif değilken çalışabilir. İki mevcut test (çift onay, çift tür gönderimi)
    ref geçici olarak silinince kırıldı: ikinci istek de gitti.
  - "İşlenmiş satır" işaretleri (`settled`) kaldı: sunucu verisi değil, ekranın durumu.
- **Üç küçük davranış değişikliği:**
  - Moderasyonda sekme değişirken yeni liste gelene kadar önceki sekmenin satırları görünür ama düğmeleri
    kilitli. Eskiden de görünüyordu, ama tıklanabiliyordu.
  - Kitap sayfasında yeni bir işlem başlarken öteki işlemlerin eski hatası silinir. Bu eski davranışın
    aynısı: tek bir `error` durumu yerine her işlemin kendi hatası var, `clearActionErrors` hatalı olanları
    `reset()` eder. Süren bir işlemi sıfırlamaz; yoksa onun kilidi de kalkardı.
  - Bildirim ("“…” yayına alındı") artık liste yenilendikten sonra çıkar; eskiden yenilemeden hemen önce
    çıkıyordu. Gecikme, listenin yeniden çekilme süresi kadar.
- **Sonuç:**
  - vitest 389 → 397. Eski kancanın 7 testi gitti, yerine `usePageView`'ın 6 testi geldi. 9 yeni test:
    4 geçersiz kılma kuralı, Moderasyonda 2 (yer tutucu satırlar kilitli; son kitap onaylanınca geri adım),
    kitap sayfasında 3 (yeni işlem eski hatayı temizler, yorum yükleme hatası, son yorum silinince geri
    adım). Mevcut Moderasyon ve kitap sayfası testleri yalnız `withQuery` sarmalıyla, değişmeden geçiyor.
  - Her yeni testin bir şey yakaladığı denendi: koruduğu satır geçici olarak silinince test kırıldı.
  - Playwright 20/20 (yerelde; moderasyon ve yorum akışları dahil).
  - Ana paket: bkz. aşağıdaki ölçüm notu.
- **Ölçüm notu — sınıra yaklaştık:** ilk açılışta indirilen JS, Vite'ın gzip rakamıyla 121,3 → 122,6 KB
  (başlangıç 110,7; artış **+11,9 KB**, sınır 12). Bunun ~0,4 KB'ı içerikten değil: Vite (8.3.8) bu adımda
  ana paketten React'i ve ortak parçaları içeren ikinci bir dosya (`toast-….js`) ayırdı; iki dosya ayrı
  sıkıştırılınca toplam biraz büyüyor. İki dosya aynı anda indiriliyor (`modulepreload`), yükleme süresine
  etkisi yok. Adım 7'den sonra yeniden ölçülecek.
- **Sonradan (2026-11-21): 12 KB'lık sınır kaldırıldı.** Kullanıcı sınırın nereden geldiğini sordu. Cevap:
  asistan spec'i yazarken (commit `74b6a57`) uydurmuştu, yani kütüphanenin tamamının o gün ölçülen boyutu
  (~10,5 KB) ve gerekçesi yazılmamış ~1,5 KB'lık bir pay. Bir performans hedefine bağlı değildi; varsayımı
  da adım 1'de yanlış çıktı (Vite kütüphanenin tamamını değil, kullanılan parçaları alıyor). Kabul
  kriteri asıl amacına göre yeniden yazıldı (aşağıda). Sayfaları tembel yüklemek (ölçüldü: −2,0 KB) bu
  fazın işi değil; istenirse ayrı bir iş.
- **Bakılacak yerler (bu sırayla):**
  1. `frontend/src/components/usePaged.tsx`: yeni `usePageView` ve yorumundaki "üç kural ne oldu".
  2. `frontend/src/api/queries.ts`: dosyanın sonundaki üç yorum kancası; `onSuccess` ile `onSettled` farkı.
  3. `frontend/src/api/adminQueries.ts`: `useModerateBook`'un yorumu (neden iki şey geçersiz oluyor).
  4. `frontend/src/pages/admin/Moderation.tsx`: `switchTab`, `rowsLocked`, kalan `inFlight` ref'i.
  5. `frontend/src/pages/BookDetail.tsx`: `clearActionErrors`, `post`, `remove`, `shownError`.
  6. Testler: `api/adminQueries.test.tsx` (yeni), `components/usePaged.test.tsx`, iki sayfa testindeki yeni
     testler.
- **Kendin dene:** `make dev`, admin olarak giriş yap.
  - Moderasyonda bir kitabı onayla: DevTools'ta `["admin","books","PENDING",0]` "fetching" olur; öteki
    sekmeler ve `["books",…]` kayıtları bayat (sarı) görünür.
  - Bir kitap sayfasında yorum yaz: `["comments",…,0]` yeniden çekilir.
  - "Eskiler →" sonra "← Yeniler": ilk sayfa istek atmadan önbellekten gelir.

### Adım 7 — Kitaplık ve raf durumları

- **Amaç:** `Library`, `LibraryMenu` ve `useLibraryStatuses` sorgu ve değişikliklere çevrilir.
- **Neden:** bir bileşendeki değişikliğin (rafa ekle) **başka** bileşenlerdeki veriyi (kitaplık listesi, kart
  üstündeki rozet) güncellemesi: geçersiz kılmanın asıl faydası burada görülür.
- **Ne yapıldı (2026-11-21):**
  - `api/queries.ts`: iki yeni anahtar (`library`, `libraryStatuses`), `keepPreviousOwnData` yardımcısı;
    `useRateBook` artık Kitaplığım listelerini de bayat işaretliyor.
  - Yeni `api/libraryQueries.ts`: `useLibrary`, `useLibraryStatus`, `useSetLibraryStatus`,
    `useRemoveFromLibrary`. Ayrı dosya, çünkü yalnız tembel yüklenen iki parça kullanıyor (Kitaplığım ve
    menü); ana pakete girmedi (derlemede ayrı, 0,34 KB'lık ortak bir parça).
  - `useLibraryStatuses.tsx`: etki, geç cevap bayrağı ve "kime ait" kaydı gitti; tek bir `useQuery`.
  - `LibraryMenu.tsx`: dört durum (`status`, `loaded`, `busy`, `failed`), "tekrar dene" sayacı ve etki gitti.
  - `Library.tsx`: etki, `page`/`error`/`loading`/`busy` durumları ve `reload` sayacı gitti; boş sayfadan
    geri adım artık `usePageView`'da.
  - `usePaged.tsx`: `usePageView` boş sayfada "bir sayfa geri" değil, son DOLU sayfaya dönüyor (aşağıda).
- **Yeni kavram 1 — ortak nokta yalnız anahtar:** menü, katalog rozetleri ve Kitaplığım listesi birbirini
  tanımıyor. Hepsi `["me", kullanıcı, "library", …]` altında; rafa ekleme bu öneki geçersiz kılıyor:
  - ekrandakiler hemen yeniden çekilir (kitap sayfasındaki düğme, açıksa Kitaplığım listesi);
  - ötekiler açılınca (katalog rozetleri, öteki filtrelerin listeleri).
  Bunu gösteren test: `LibraryButton.test.tsx`, "menüden eklenen kitabın rozeti aynı sayfadaki başka bir
  bileşende de güncellenir".
- **Yeni kavram 2 — neden cevabı önbelleğe yazmıyoruz:** puanda yeni değeri `setQueryData` ile yazmıştık
  (adım 5), çünkü "benim puanım" tek bir kayıttı. Bir kitabın kitaplık durumu ise ekranda görünen her kimlik
  listesinin kaydında var ("3,4,9", "12", …). Hepsini bulup düzeltmek yerine sunucuya yeniden soruyoruz:
  bir küçük istek, karşılığında tek bir kural. Bedeli: menüdeki yeni durum yazma + okuma kadar sonra görünür.
- **Yeni kavram 3 — kullanıcıya özel yer tutucu:** düz `keepPreviousData` anahtar değişince önceki kaydı
  gösterir, kimin olduğuna bakmaz. Kullanıcı değişirse öncekinin rozetleri bir an görünürdü. Eski kancada
  bunu bir "kime ait" alanı önlüyordu; şimdi `keepPreviousOwnData` önceki kaydın anahtarındaki kullanıcıya
  bakıyor. Kaynakta bulunan bir ayrıntı: TanStack yer tutucu fonksiyonu öncekiyle AYNI nesneyse onu yeniden
  çağırmaz (`queryObserver.js`); bu yüzden fonksiyon her çizimde yeniden kuruluyor.
- **Yeni kavram 4 — bir veri başka bir verinin içinde de görünür:** Kitaplığım her kartta "Puanım"ı
  gösteriyor ve puana göre süzüp sıralıyor. Liste artık önbellekte durduğu için puan vermek onu da bayat
  işaretlemeli; yoksa puan verip 30 sn içinde Kitaplığım'a geçen okur eski puanı görürdü. Eskiden sayfa her
  açılışta yeniden çektiği için bu sorun yoktu: önbellek, eskiden kendiliğinden doğru olan şeyi açıkça
  söylemeyi gerektiriyor.
- **Bilerek korunan:** durum okunamazsa menü "Okumak istiyorum" göstermiyor, "tekrar dene" gösteriyor
  (PUT kayıtlı bir "Okudum"un üstüne yazardı). Artık değişiklikten SONRAKİ yeniden okuma başarısız olursa da
  aynısı: ekrandaki durum doğru olmayabilir.
- **Davranış değişiklikleri:**
  - Menüde yeni durum ve bildirim, durum sunucudan yeniden okununca görünür (eskiden PUT cevabıyla hemen).
  - Kitaplığım'da çıkarma onayı, liste yenilenene kadar açık kalır (düğme pasif); eskiden önce kapanıp
    sonra liste soluk hâlde yenileniyordu.
  - Son sayfadaki son kitap çıkınca önceki sayfaya geçiş geçmişe kayıt eklemiyor (`replace`); geri tuşu
    boş sayfaya düşmüyor. Arada kısa bir iskelet görünebilir (adım 6'daki "boş sayfa çizilmez" kuralı).
  - Boş sayfadan geri adım yorumlarda ve moderasyonda da artık son dolu sayfaya; normal durumda (son
    sayfadaki son öğe gitti) sonuç aynı, yalnız elle büyütülmüş bir adreste fark var.
  - Rozet okuması 5xx'te bir kez yeniden deneniyor (genel kural); eskiden denenmiyordu.
- **Sonuç:**
  - vitest 397 → 404. Yeni: `api/libraryQueries.test.tsx` (4: ekleme ve çıkarma neyi eskitir, başarısız
    değişiklik hiçbir şeyi, puan Kitaplığım'ı), `LibraryButton.test.tsx`'te 2 (rozet başka bileşende
    güncellenir; yeniden okuma başarısızsa tekrar dene), `usePaged.test.tsx`'te 1 (tek adımda son dolu sayfa).
  - Mevcut testler: `LibraryButton` ve `useLibraryStatuses` testleri önbellek sarmalıyla, `Library` testleri
    oturum taklidiyle çalışıyor. Eski testlerin beklentileri değişmedi; değişen sahte uçların kurulumu:
    menü durumu artık sunucudan yeniden okuduğu için sahte uçlar sabit cevap yerine yazılanı hatırlıyor
    (`shelf`), "son kitap çıkınca önceki sayfa" testinde de sahte sunucu çıkarılan kitabı listeden düşürüyor.
    Eklenen beklentiler: "istek sürerken menü pasif" sonunda düğmenin "Kitaplığında" olduğu, "son kitap
    çıkınca" sonunda ilk sayfanın çizildiği. Bir rozet testine sorgunun bildirimi için bir `act` daha eklendi.
  - Her yeni korumanın bir testi kırdığı denendi (koruduğu satır geçici olarak bozuldu): yer tutucu
    kullanıcı kontrolü, ekleme ve çıkarma sonrası geçersiz kılma, yeniden okuma hatasında tekrar dene,
    puan → Kitaplığım, son dolu sayfaya tek adım. Altısı da en az bir testi kırdı.
  - Playwright 20/20 (yerelde; kitaplık akışını uçtan uca sınayan test dahil). `tsc -b` temiz.
  - Sayfalarda veri çeken `useEffect` kalmadı (kabul kriteri 1); kalan tek yer `AuthContext`, kapsam dışı.
  - Ana paket: Vite'ın gzip rakamıyla **122,6 → 121,5 KB** (başlangıçtan +10,8 KB). Adım 6'daki ikinci
    dosya (`toast-….js`) bu derlemede yok, ana paket yine tek dosya; nedenini incelemedim, ölçülen bu.
- **Bakılacak yerler (bu sırayla):**
  1. `frontend/src/api/libraryQueries.ts`: değişikliklerin üstündeki yorum (neden önek, neden yazmıyoruz).
  2. `frontend/src/api/queries.ts`: iki yeni anahtar, `keepPreviousOwnData`, `useRateBook`'taki 4. madde.
  3. `frontend/src/components/useLibraryStatuses.tsx`: eski üç işin nereye gittiği.
  4. `frontend/src/components/LibraryMenu.tsx`: `change`, `remove`, `disabled`, tekrar dene düğmesi.
  5. `frontend/src/pages/Library.tsx`: `useLibrary`, `toOffset`, `remove`.
  6. `frontend/src/components/usePaged.tsx`: `lastPageOffset`.
  7. Testler: `api/libraryQueries.test.tsx` (yeni), `LibraryButton.test.tsx`'in son iki testi ve sahte
     sunucusu.
- **Kendin dene:** `make dev`, giriş yap.
  - Kataloğu aç, bir kitaba gir, "Okumak istiyorum"a bas. DevTools'ta `["me",…,"library","statuses","…"]`
    kayıtları bayat olur, kitabınki yeniden çekilir. Kataloğa dön: rozet yeni durumla, bir istekle gelir.
  - Kitaplığım'ı aç, bir kitaba puan ver, geri dön: liste yeniden çekilir ve yeni puanı gösterir.
  - Kitaplığım'da filtreler arasında gidip gel: 30 sn içinde yeniden açılan filtre istek atmadan önbellekten
    gelir.

### Adım 8 — Çıkışta temizlik ve son ölçüm

- **Amaç:** çıkışta önbelleği temizlemek (karar 4), kullanılmayan kodu silmek, paket boyutunu ve test
  sayısını son kez ölçmek, öğrenme notunu yazmak (`/not`).
- **Ne yapıldı (2026-11-21):**
  - `AuthContext.tsx`: oturum kapanınca `queryClient.clear()`.
  - Dokümanlar: PROJECT_MAP'in arayüz satırları (yeni `api/*Queries.ts` ve `lib/queryClient.ts` satırları,
    `usePaged` → `usePageView`), phases/README'deki 16d bağlantısı göreli, STATUS ve ROADMAP'te durum.
  - Öğrenme notu: [frontend-and-ui.md #7](../learning-notes/frontend-and-ui.md).
- **Önbellek NEREDE temizleniyor:** oturumun kapandığı her yol (çıkış düğmesi, başka sekmede çıkış,
  yenilemenin reddi) `client.ts`'te aynı bildirimden (`onSessionChange(null)`) geçiyor. Temizlik
  `AuthContext`'in bu bildirime aboneliğinde; `logout` da ayrıca temizliyor (istemci bildirimi bir gün
  atlarsa diye). Oturum yenilemesi de aynı bildirimden geçer ama kullanıcıyla geldiği için önbelleğe
  dokunmaz; bunun da testi var.
- **Neden hepsi, yalnız "me" değil (karar 4'ün kendisi):** ileride `["me", …]` dışına yazılan kullanıcıya
  özel bir anahtar da (moderasyon kuyruğu zaten `["admin", …]`) kendiliğinden kapsansın.
- **Deneyle görülen bedel:** `clear()` ekrandaki sorguları da siler. Bileşen yeniden çizilince sorgusunu
  baştan kurar ve veriyi yeniden çeker (önce `pending`, iskelet). Çıkış anında açık sayfanın herkese açık
  verisi (katalog, kitap) bu yüzden bir kez yeniden yüklenir. Çıkış seyrek; kural basitliği buna değer.
  Bedel ağırlaşırsa seçenek: `removeQueries({ queryKey: ["me"] })` ve `["admin"]`.
- **Kullanılmayan kod:** `frontend/src` altında dışa aktarılıp başka hiçbir üretim dosyasında kullanılmayan
  tanımlar tarandı. Çıkanların hepsi testler için dışa aktarılıyor ya da dosya içinde kullanılıyor
  (`shouldRetry`, `RESTORE_RETRY_DELAYS_MS`, `__resetForTests` …); fazdan kalan ölü kod yok. Kullanılmayan
  yerel değişkeni ve parametreyi zaten `tsc` yakalıyor (`noUnusedLocals`, `noUnusedParameters`).
- **Bilerek bırakılan:** Hesabım (parola değiştirme), kayıt ve öneri formları `api.*`'yi doğrudan
  çağırıyor. Bunlar etki değil, form gönderimi; başka bir ekranın verisini eskitmiyorlar. `useMutation`'a
  çevirmek bir şey kazandırmaz; kabul kriteri 1'in de dışındalar.
- **Sonuç (fazın tamamı):**
  - vitest 385 → **407**, `tsc -b` temiz, Playwright 20/20 (yerelde).
  - Ana paket (Vite'ın gzip rakamı): **110,7 → 121,5 KB (+10,8 KB)**. Adım adım: 1 +7,4 · 2–4 +2,6 · 5 +0,6
    · 6 +1,3 · 7 −1,1 · 8 ±0. Adım 1'deki +7,4 kütüphanenin çekirdeği (kurulum kodumuz birkaç satır);
    sonraki adımlar kütüphanenin yeni kullanılan parçaları (ör. `useMutation`) ve kancalarımız. Adım 7'deki
    düşüş, adım 6'da ayrılan ikinci dosyanın (`toast-….js`) kalkması; nedenini incelemedim. Beklenmedik
    büyüme yok: geliştirici aracı üretim paketinde değil; moderasyon ve kitaplık değişikliklerinin kodu
    ana pakette değil, derleme çıktısında `Moderation-….js` ve `libraryQueries-….js` parçalarında (ana
    pakette yalnız anahtarlar ve kancaların ortak kısmı).
  - Yeni testler: oturum kapanınca önbellek (3: çıkış, dışarıdan düşme, yenilemede KALIR). Temizlik
    satırları geçici olarak silinince ilgili test kırıldı.
- **Bakılacak yerler:**
  1. `frontend/src/auth/AuthContext.tsx`: üstteki karar 4 yorumu, abonelikteki `queryClient.clear()`, `logout`.
  2. `frontend/src/auth/AuthContext.test.tsx`: en alttaki "önbellek" testleri.
  3. `docs/learning-notes/frontend-and-ui.md` #7.
  4. PROJECT_MAP, STATUS, ROADMAP, phases/README'deki 16d satırları.
- **Kendin dene:** `make dev`, giriş yap, Kitaplığım'ı ve bir kitabı aç. DevTools'ta `["me",…]` kayıtları
  görünür. Çıkış yap: önbellek boşalır, açık sayfanın herkese açık verisi bir kez yeniden çekilir.

## ✅ Kabul kriterleri

- ✅ Sayfalarda veri çeken `useEffect` kalmaz (`AuthContext` ve tema hariç). Adım 7'de doğrulandı.
- ✅ Mevcut vitest ve Playwright testleri geçer; eski cevap yarışı için bir test vardır
  (`usePaged.test.tsx`, "geç gelen ESKİ sayfa cevabı yenisinin üzerine yazmaz"; katalog için Home testleri).
- ✅ Çıkış yapınca kullanıcıya özel veri önbellekte kalmaz (test: `AuthContext.test.tsx`, "önbellek").
- ✅ Ana paket artışı ölçülür ve PR'da adım adım yazılır. Kabul koşulu: artışın kaynağı açıklanabilir olmalı
  (kütüphanenin kendisi ve bizim kancalarımız); beklenmedik bir büyüme (ör. geliştirici aracının üretim
  paketine girmesi, admin kodunun ana pakete kayması) varsa düzeltilir.
  - Bu kriter önce "12 KB gzip'i geçmez" idi. O sayı gerekçesizdi (bkz. adım 6'daki "Sonradan" notu);
    kullanıcıyla konuşulup 2026-11-21'de bu hâle getirildi. Gerçek bir boyut bütçesi (ör. ilk yüklenen
    toplam JS için bir üst sınır ve CI denetimi) istenirse kendi gerekçesiyle ayrı bir iş olarak açılır.

## 🚫 Kapsam dışı

- `AuthContext` ve `client.ts`'teki oturum yenileme mantığı.
- İyimser güncelleme (sunucu cevap vermeden ekranı değiştirmek), sonsuz kaydırma, sunucu tarafı render.
- Arka uçta değişiklik.
