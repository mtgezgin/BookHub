# Faz 16d — Sunucu verisi TanStack Query ile

> Durum: 📝 spec (2026-11-19), adım 1 başlamadı · Issue: [#105](https://github.com/mtgezgin/BookHub/issues/105) —
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

## 🎨 Kararlar (adım 1'de kullanıcıya sorulacak; önerilerle)

| # | Konu | Öneri | Gerekçe |
|---|---|---|---|
| 1 | Sürüm | `@tanstack/react-query` tam sürümle sabitlenir (package.json'daki diğer paketler gibi) | Ölçülen sürüm 5.108.0: ~10,5 KB gzip |
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
- **Sonuç:** paket boyutu önce/sonra ölçümü; tüm testler yeşil.

### Adım 2 — Türler: en basit okuma ve paylaşılan önbellek

- **Amaç:** `Home` ve `Library`'deki iki ayrı `api.genres()` çağrısını tek bir `useGenres()` kancasına çevirmek.
- **Neden:** en basit örnek: parametresi yok, seyrek değişiyor, iki sayfada kullanılıyor. Sorgu anahtarı ve
  önbellek paylaşımı burada görülür.
- **Kendin dene:** Ağ sekmesinde ana sayfadan Kitaplığım'a geçince `/api/genres` isteğinin **ikinci kez
  gitmediğini** görmek.

### Adım 3 — Katalog listesi: parametreli sorgu ve eski cevap koruması

- **Amaç:** `Home`'daki katalog `useEffect`'ini `useQuery` ile değiştirmek; anahtar süzgeçleri içerir
  (`["books", { q, genre, sort, offset }]`). Sayfa değişirken önceki sayfa ekranda kalır (`placeholderData`).
- **Neden:** F10'daki "geç gelen eski cevap yenisinin üstüne yazıyor" hatasının sınıfı burada kapanır: her
  cevap kendi anahtarına yazılır, ekran yalnız güncel anahtarı gösterir. Elle yazılmış bayrak silinir.
- **Sonuç:** eski cevap yarışı için bir test (yavaş ilk istek, hızlı ikinci istek; ekranda ikincisi kalır).
- **Kendin dene:** bir türe girip geri dönünce katalog **anında** görünür (önbellekten), sonra arka planda tazelenir.

### Adım 4 — En sevilenler şeridi

- **Amaç:** `topRated` çağrısını sorguya çevirmek. Küçük adım; adım 3'teki kalıbın tekrarı.

### Adım 5 — Kitap sayfası ve ilk değişiklik (mutation)

- **Amaç:** kitap, puan özeti, benzer kitaplar ve benim puanım sorgu olur. Puan verme ve silme `useMutation`
  olur; başarıdan sonra puan özeti ve benim puanım **geçersiz kılınır**.
- **Neden:** yazma işleminden sonra "neyin yenileneceği" bugün elle yazılıyor. Geçersiz kılmayla bu, verinin
  anahtarına bağlanır. Faz 17'nin inceleme yazma işi bu kalıbı kullanacak.
- **Kendin dene:** puan verince özetin güncellendiğini, DevTools'ta ilgili kaydın "fetching" olup döndüğünü görmek.

### Adım 6 — Yorumlar ve `usePaged`'in yerini almak

- **Amaç:** yorum listesi ve moderasyon kuyruğu sorgu olur; yorum ekleme, silme ve moderasyon işlemleri
  değişiklik olur. `usePaged`'in sayaç ve ref mantığı kalkar; `Pager` bileşeni kalır.
- **Neden:** `usePaged`'in yorumundaki üç kural (boş sayfada geri adım, eski cevap koruması, sayfalayıcının
  görünmesi) sorguyla nasıl karşılanıyor, tek tek gösterilir.

### Adım 7 — Kitaplık ve raf durumları

- **Amaç:** `Library`, `LibraryMenu` ve `useLibraryStatuses` sorgu ve değişikliklere çevrilir.
- **Neden:** bir bileşendeki değişikliğin (rafa ekle) **başka** bileşenlerdeki veriyi (kitaplık listesi, kart
  üstündeki rozet) güncellemesi: geçersiz kılmanın asıl faydası burada görülür.

### Adım 8 — Çıkışta temizlik ve son ölçüm

- **Amaç:** çıkışta önbelleği temizlemek (karar 4), kullanılmayan kodu silmek, paket boyutunu ve test
  sayısını son kez ölçmek, öğrenme notunu yazmak (`/not`).

## ✅ Kabul kriterleri

- Sayfalarda veri çeken `useEffect` kalmaz (`AuthContext` ve tema hariç).
- Mevcut vitest ve Playwright testleri geçer; eski cevap yarışı için bir test vardır.
- Çıkış yapınca kullanıcıya özel veri önbellekte kalmaz (test).
- Ana paket artışı ölçülür ve 12 KB gzip'i geçmez.

## 🚫 Kapsam dışı

- `AuthContext` ve `client.ts`'teki oturum yenileme mantığı.
- İyimser güncelleme (sunucu cevap vermeden ekranı değiştirmek), sonsuz kaydırma, sunucu tarafı render.
- Arka uçta değişiklik.
