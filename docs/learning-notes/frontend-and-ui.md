# Öğrenme Notları — Ön yüz ve Arayüz

Arayüz kavramları: CSS ve tasarım sistemi, React bileşen kalıpları, erişilebilirlik ve
arayüz testleri. Format: [README.md](README.md).

## İçindekiler

1. [Tasarım token'ları ve utility-first CSS (Tailwind)](#1-tasarım-tokenları-ve-utility-first-css-tailwind)
2. [shadcn/ui: paket değil, repoya kopyalanan bileşen](#2-shadcnui-paket-değil-repoya-kopyalanan-bileşen)
3. [Erişilebilirlik ve rol tabanlı test (bütün kartı tıklanır yapmak dahil)](#3-erişilebilirlik-ve-rol-tabanlı-test-bütün-kartı-tıklanır-yapmak-dahil)
4. [?? ve ??= ile ||: boş değer mi, yanlış değer mi?](#4--ve--ile--boş-değer-mi-yanlış-değer-mi)
5. [Dış kaynaktan kapak: lisans, bağlantı ve referrerPolicy](#5-dış-kaynaktan-kapak-lisans-bağlantı-ve-referrerpolicy)

---

## 1. Tasarım token'ları ve utility-first CSS (Tailwind)

**🗓️ Bağlam:** 2026-10-10'da canlı siteye bakıp arayüzü "çok amatör" buldum. Teşhis: her şey aynı
mor (`--accent: #7c3aed`), tek font, 61 satırlık tek CSS dosyası, ondan bundan sınıf adları
(`.book-card`, `.score-picker`). İki yol konuştuk — Tailwind + shadcn/ui (A) ya da düz CSS +
token'lar (B) — A'yı seçtim. Faz A'da ilk iş renkleri "token" yapmaktı.

**⚡ Kısa cevap:** Token, bir tasarım kararının **adıdır** (`primary`, `muted-foreground`);
değeri tek yerde durur. Bileşen ham renk değil adı kullanır (`bg-primary`). Tema değiştirmek =
yalnız token değerlerini değiştirmek. Tailwind bu adlardan küçük sınıflar üretir ve stil
bileşenin yanında yazılır.

**📖 Açıklama:**

Üç katman var: ham değer → anlamlı ad → sınıf.

```css
:root { --primary: oklch(0.43 0.075 162); }                 /* ham değer, tek yer */
@media (prefers-color-scheme: dark) {
  :root { --primary: oklch(0.74 0.1 162); }                 /* koyu tema: yalnız değer değişir */
}
@theme inline { --color-primary: var(--primary); }          /* Tailwind'e "bg-primary" üret */
```

```tsx
<Button className="bg-primary hover:bg-primary/90 sm:px-3">…</Button>
```

- **Utility-first:** `.book-card { … }` diye ayrı dosyada sınıf uydurmak yerine stil, kullanıldığı
  yerde küçük sınıflarla yazılır. Tailwind kaynak dosyaları tarar, **yalnız geçen** sınıfların
  CSS'ini üretir. Durumlar da aynı dilde: `hover:`, `sm:` (ekran genişliği), `dark:`.
  TS/JS'teki karşılığı: satır içi `style={{…}}` gibi ama tasarım sisteminin sınırları içinde.
- **OKLCH:** ilk sayı **algılanan parlaklık** (0–1). "Bunu biraz koyulaştır" = ilk sayıyı düşür.
  HEX'te üç kanalı tahmin etmek gerekirdi; OKLCH'de ton (son sayı) sabit kalır.
- **Kontrast gözle değil ölçüyle:** WCAG AA → metin ≥ 4.5:1, ikon/arayüz öğesi ≥ 3:1.
  [check-contrast.mjs](../../frontend/scripts/check-contrast.mjs) token'ları iki temada ölçer.
  İlk koşuda açık temadaki yıldız rengi **2,51:1** çıktı (göze "güzel kehribar" görünüyordu) →
  koyulaştırıldı, 3,44.
- **Fontlar kendi sunucumuzdan:** CSP'de `font-src` yok → `default-src 'self'` geçerli → Google
  Fonts CDN'i engellenir. `@fontsource-variable/*` fontu Vite paketine koyar. `unicode-range`
  sayesinde tarayıcı yalnız gereken alt kümeyi indirir; Türkçe harfler (ğ, ş, ı, İ) `latin-ext`'te.

**Dürüst not:** Tailwind'in bedeli uzun sınıf listeleri ve bir öğrenme eğrisi. Token mantığı
Tailwind'e özgü değil; B seçeneği (düz CSS + CSS değişkenleri) aynı tasarım sistemine götürürdü.
A, hız ve shadcn/ui ekosistemi yüzünden seçildi.

**🔗 İlgili:**
- [docs/phases/phase-14-ui-redesign.md](../phases/phase-14-ui-redesign.md) (kararlar ve ölçümler)
- [frontend/src/styles.css](../../frontend/src/styles.css) · [frontend/nginx/security-headers.conf](../../frontend/nginx/security-headers.conf) (CSP)
- [#2 shadcn/ui](#2-shadcnui-paket-değil-repoya-kopyalanan-bileşen)

---

## 2. shadcn/ui: paket değil, repoya kopyalanan bileşen

**🗓️ Bağlam:** A seçeneğinin ikinci yarısı. "Bileşen kütüphanesi" deyince npm'den kurulan, içine
karışılmayan bir paket (Material UI gibi) bekliyordum; shadcn/ui'nin farkını ve kodda gördüğüm
`cva`, `cn()`, `asChild` gibi şeylerin ne işe yaradığını anlamak istedim.

**⚡ Kısa cevap:** shadcn/ui bileşenleri **kurulmaz, kopyalanır**: `Button`'ın kaynak kodu
[components/ui/button.tsx](../../frontend/src/components/ui/button.tsx)'te durur ve bizim kodumuz
olur. Zor davranışlar (erişilebilir dialog, menü) Radix'ten, görünüm Tailwind sınıflarından gelir.

**📖 Açıklama:**

- **Neden kopya:** kütüphane güncellemesi görünümü habersiz bozamaz, istediğimiz gibi
  değiştiririz. Bedeli: güncellemeleri biz takip ederiz.
- **`cva` — varyant tablosu:** bir bileşenin görünüş seçenekleri tek nesnede.
  ```ts
  const buttonVariants = cva("inline-flex … rounded-md", {
    variants: { variant: { default: "bg-primary …", destructive: "bg-destructive …", outline: "border …" } },
  });
  ```
  Sayfa `<Button variant="destructive">` der, sınıf listesini tekrar etmez. TS/JS gözüyle:
  "variant adı → sınıf metni" sözlüğü.
- **`cn()` = `clsx` + `tailwind-merge`:** `clsx` koşullu birleştirir (`isActive && "bg-accent"`).
  `twMerge` **çakışmada sonuncuyu** tutar: bileşenin `px-4`'ü ile çağıranın `px-2`'si birleşince
  `px-2` kalır. İkisi birden kalsaydı hangisinin kazanacağı CSS dosyasındaki sıraya kalırdı —
  sürpriz.
- **`asChild` / `Slot`:** `<Button asChild><Link to="/register">Kayıt ol</Link></Button>` tek bir
  `<a>` çizer, düğme görünümünde. Alternatifi `<a><button>` iç içe — geçersiz HTML; klavye ve
  ekran okuyucu için iki ayrı odak noktası.

**🔗 İlgili:**
- [frontend/src/components/ui/](../../frontend/src/components/ui/) · [frontend/src/lib/utils.ts](../../frontend/src/lib/utils.ts)
- [#1 Tasarım token'ları](#1-tasarım-tokenları-ve-utility-first-css-tailwind)

---

## 3. Erişilebilirlik ve rol tabanlı test (bütün kartı tıklanır yapmak dahil)

**🗓️ Bağlam:** Faz A'da görünüm baştan değişecekti. Birim testlerinde 71 `querySelector`, e2e'de
`.grid`, `nav.topbar`, `.score-picker button.active` gibi **sınıf** seçicileri vardı; yeni
tasarımla hepsi kırılacaktı. Aynı sırada Lighthouse canlı sitede ana sayfaya 87, giriş sayfasına
92 verdi.

**⚡ Kısa cevap:** Test, kullanıcının ve ekran okuyucunun gördüğü şeye (**rol + ad**)
bağlanırsa görünüm değişince kırılmaz. Aynı zamanda arayüzü erişilebilir olmaya zorlar: ekran
okuyucunun bulamadığını test de bulamaz.

**📖 Açıklama:**

| Önce (sınıf) | Sonra (rol / öznitelik) | Arayüzdeki karşılığı |
|---|---|---|
| `nav.topbar` | `getByRole("navigation", { name: "Ana menü" })` | `<nav aria-label="Ana menü">` |
| `.grid` | `getByRole("list", { name: "Kitaplar" })` | `<ul aria-label="Kitaplar">` |
| `button.active` | `button[aria-pressed=true]` | `aria-pressed={mine === s}` |
| `.success` | `getByRole("status")` | `<Alert role="status">` |
| `form.comment-form` | `getByRole("form", { name: "Yorum yaz" })` | form **adı varsa** "form" rolü alır |

- **`aria-pressed`:** "seçili" bilgisini yalnız renkle (`class="active"`) vermek ekran
  okuyucuya hiçbir şey söylemez; `aria-pressed` "basılı" der.
- **`role="alert"` / `role="status"`:** alert okumayı hemen keser (hatalar), status sırası
  gelince okunur (başarı, "yükleniyor").
- **Bütün kart tıklanır ("stretched link"):** kartı `<a>` ile sarmak, içindeki tür rozeti
  bağlantılarıyla iç içe `<a>` yaratırdı. Onun yerine başlık bağlantısının `::after`'ı kartı
  kaplar (`article: relative`, `after:absolute after:inset-0`). Rozetler `relative z-10` ile o
  katmanın üstüne alınır, yoksa tıklanamazlar.
- **Sonuç:** Lighthouse erişilebilirlik ana sayfa 87 → 100, giriş 92 → 100. Eksikler etiketsiz
  `<select>`, yetersiz kontrast ve "yalnız renkle ayırt edilen bağlantı"ydı.

**Dürüst not:** Lighthouse otomatik bir kontrol listesi. Klavyeyle gezinmeyi ve ekran okuyucuyu
yine elle denemek gerekir. Testing Library'nin `getByRole`'u aynı felsefeden gelir; bu projede o
kütüphane yok, birim testlerindeki düz DOM sorguları rol ve `aria-*` özniteliklerine bakıyor.

**🔗 İlgili:**
- [frontend/e2e/helpers.ts](../../frontend/e2e/helpers.ts) (`topbar`) · [frontend/src/components/BookCard.tsx](../../frontend/src/components/BookCard.tsx) (stretched link)
- [tooling-and-workflow.md #7 Playwright](tooling-and-workflow.md#7-playwright-ile-uçtan-uca-test-gerçek-tarayıcı-rate-limit-bütçesi-ve-storagestate-tuzağı)
- [docs/phases/phase-14-ui-redesign.md](../phases/phase-14-ui-redesign.md) (ölçümler)

---

## 4. ?? ve ??= ile ||: boş değer mi, yanlış değer mi?

**🗓️ Bağlam:** 2026-10-13. SonarCloud `client.ts`'teki token yenileme kodunda `typescript:S6606`
("Nullish coalescing should be preferred") bulgusu verdi; "yalnız bunu düzelt" dedim. Düzeltmede
`??=`'nin ne yaptığını anlatan iki satırlık bir yorum da eklenmişti; "mantıksızsa yapmayalım" diye
sordum ve yorumu kaldırdık.

**⚡ Kısa cevap:** `||` sol taraf **herhangi bir yanlış değer** (`0`, `""`, `false`, `NaN`, `null`,
`undefined`) olduğunda sağa geçer. `??` yalnız **`null` ya da `undefined`** olduğunda geçer. `a ??= b`
ise "a boşsa b'yi ata; doluysa b'yi hiç çalıştırma" demektir.

**📖 Açıklama:**

| İfade | `x = 0` | `x = ""` | `x = null` |
| --- | --- | --- | --- |
| `x \|\| 5` | 5 | 5 | 5 |
| `x ?? 5` | 0 | "" | 5 |

Fark, `0` ya da boş metin gibi **geçerli ama yanlış (falsy)** değerlerde ortaya çıkar. Sayfa
sayısı `0`, puan `0`, arama metni `""` gibi değerlerde `||` sessizce varsayılana kayar; çoğu zaman
istenen bu değildir.

Bizim kodumuz (tek uçuşlu yenileme):

```ts
// önce
if (!refreshing) {
  refreshing = doRefresh().finally(() => { refreshing = null; });
}
// sonra
refreshing ??= doRefresh().finally(() => { refreshing = null; });
```

`refreshing` ya bir `Promise` ya da `null` olduğu için davranış aynı; `??=` yalnız niyeti daha açık
söylüyor: "yalnız boşken başlat". Önemli ayrıntı: sağ taraf **kısa devre** ile çalışır; doluyken
`doRefresh()` hiç çağrılmaz, ikinci bir yenileme başlamaz.

**Yorum neden kalktı:** yorum, operatörün kendisinin söylediğini tekrar ediyordu. Kod kuralımız:
yorum yalnız kodun gösteremediği bir kısıt için yazılır. "Neden tek uçuş" bilgisi zaten fonksiyonun
üstünde ve learning-notes'ta var.

**Go'da:** `??` yok. Her tipin bir **zero value**'su var (`0`, `""`, `nil`) ve "yokluk" çoğu zaman
açık bir `if v == nil` ya da `v, ok := m[k]` ile kontrol edilir. Daha uzun, ama falsy karmaşası da yok.

**🔗 İlgili:**
- [frontend/src/api/client.ts](../../frontend/src/api/client.ts) (`refreshSession`) · commit'ler `ef5a14b`, `8d4bc88`
- [go-language.md #8 Zero value](go-language.md#8-zero-value-nedir-nasıl-belirlenir)
- [tooling-and-workflow.md #8 SonarCloud](tooling-and-workflow.md#8-sonarcloud-bulgu-quality-gate-accepted-ve-mcp)

---

## 5. Dış kaynaktan kapak: lisans, bağlantı ve referrerPolicy

**🗓️ Bağlam:** 2026-10-12, Faz 14'ün son adımı: demo kitaplara gerçek kapak. Kaynak olarak
"kopyasını repoya koymak" ile "Open Library'ye bağlantı" seçenekleri konuşulurken "lisans riski"
dendi; "Ben burada kitap değerlendirme platformu yapmaya çalışıyorum, neden lisans riski var?" diye
sordum. Sonra Tutunamayanlar'ın kapağının yanlış (iç sayfa) olduğunu ekran görüntüsüyle yakaladım.

**⚡ Kısa cevap:** Kitap kapağı kendi başına telifli bir eserdir (tasarımcı/yayınevi). Bir
değerlendirme sitesinde kapağı "bu yorum bu kitapla ilgili" demek için **göstermek** yaygın ve
düşük risklidir. Risk, kapağı **kopyalayıp** kendi repomuzdan ya da imajımızdan yeniden dağıtmaktır.
Biz bağlantı veriyoruz: kapak Open Library'de duruyor, biz yalnız `<img src>` ile gösteriyoruz.
(Hukuki görüş değil; genel tablo.)

**📖 Açıklama:**

| | Bağlantı (seçtiğimiz) | Kopya saklamak |
| --- | --- | --- |
| Görseli kim tutuyor | Open Library (kapak servisini tam bu iş için sunuyor) | Biz; repo ve imajla dağıtıyoruz |
| Servis düşerse | Üretilmiş kapak gösterilir | Etkilenmez |
| Ek sorun | — | Herkese açık repo, görselleri klonlayan herkese dağıtır |

Bağlantının küçük ayrıntıları ([BookCover.tsx](../../frontend/src/components/BookCover.tsx)):

- **Kapak kimliğiyle adres** (`/b/id/<id>-L.jpg`): ISBN ya da başlıkla aramak yerine sabit kimlik;
  Open Library'nin hız sınırına girmiyor.
- **`?default=false`:** kimlik bir gün kalkarsa Open Library boş bir yer tutucu değil **404** döner;
  `<img>`'nin `onError`'u tetiklenir ve arayüz kendi üretilmiş kapağına düşer.
- **`referrerPolicy="no-referrer"`:** tarayıcı görseli isterken normalde `Referer` başlığında hangi
  sayfadan geldiğini (kitap adresimizi) gönderir. Bu ayarla göndermez: üçüncü tarafa ziyaretçinin
  hangi kitaba baktığını sızdırmayız.
- **Gözle doğrulama şart:** Open Library'deki taramalar bazen kapak değil iç sayfa ya da başka bir
  kitap. 53 kapağın hepsi bir kontrol sayfasında tek tek bakılarak seçildi; 5'i değiştirildi.

**🔗 İlgili:**
- [backend/migrations/0009_demo_book_covers.sql](../../backend/migrations/0009_demo_book_covers.sql) — kural: Türkçe baskı varsa o, yoksa özgün
- [database.md #14 Eser ve baskı modeli](database.md#14-eser-ve-baskı-modeli-kısmi-benzersiz-index-ile-tek-varsayılan) — bu sorudan doğan #36
- [frontend/nginx/security-headers.conf](../../frontend/nginx/security-headers.conf) — CSP `img-src 'self' https: data:`: HTTPS kapak adresleri ek ayar gerektirmedi
