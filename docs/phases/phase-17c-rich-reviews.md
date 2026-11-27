# Faz 17C — Zengin metinli inceleme ve seçici spoiler

> Durum: ✅ **spec onaylandı** (2026-11-27); uygulama başlamadı ·
> [ROADMAP](../ROADMAP.md) · Backlog: [FEATURE_REQUESTS](../FEATURE_REQUESTS.md) REV-04 ·
> Issue: [#128](https://github.com/mtgezgin/BookHub/issues/128) · İlgili:
> [phase-17](phase-17-reading-reviews.md) (17B: inceleme, sürümlü yazım, gizleme),
> [0018_reviews.sql](../../backend/migrations/0018_reviews.sql)

## 🎯 Amaç

Okur incelemesini biçimlendirebilsin (paragraf, kalın, italik, alıntı, liste) ve **yalnız spoiler olan
kısmı** gizleyebilsin. Okuyan, spoiler olmayan kısmı görsün; her spoiler parçasını ayrı açsın.

Bugün (17B) inceleme düz metin ve spoiler işareti incelemenin tamamını kapatıyor.

## 🔍 Başlangıç durumu (neden şimdi)

- `reviews.body` düz metin (1–5000 karakter), `reviews.has_spoiler` tek bir kutu.
- Canlıda **tek** inceleme var. Saklama biçimini değiştirmenin en ucuz anı bu.
- Kullanıcı kararı (2026-11-27): her şey en basit hâliyle yapılmasın, ama editör sıfırdan da yazılmasın;
  hazır bir çekirdek üstüne kısıtlı şema kurulsun.

## 🧠 Yeni kavramlar

1. **Belge modeli ve şema.** Zengin metin editörleri metni HTML olarak değil, bir düğüm ağacı olarak
   tutar: belge → paragraf → metin parçaları. Kalın ya da spoiler gibi biçimler metin parçasının
   üstündeki *işaretlerdir* (mark). **Şema** hangi düğümün neyin içinde durabileceğini söyler; şemada
   olmayan şey belgeye giremez.
2. **Neden HTML saklamıyoruz.** Kullanıcının yazdığı HTML'i saklayıp sayfaya basmak **kalıcı XSS**
   kapısıdır: `<script>` ya da `onerror=` içeren bir inceleme, onu okuyan herkesin tarayıcısında
   çalışır. HTML temizleyiciler (sanitizer) bu yüzden vardır. Bizim yolumuz temizlemek değil,
   **hiç HTML kabul etmemek**: saklanan şey yalnız türü ve metni olan bir JSON.
3. **İstemciye güvenilmez.** Editör şemaya uysa bile isteği herkes elle gönderebilir. Bu yüzden asıl
   kapı sunucudaki doğrulayıcıdır (izin listesi); editörün şeması yalnız kullanım kolaylığıdır.
4. **Türetilmiş veri.** Belgeden iki şey türetilir ve sunucuda hesaplanır: düz metin (uzunluk sınırı,
   ileride arama) ve "spoiler içeriyor mu". İstemcinin söylediğine bakılmaz.
5. **`contenteditable` neden zor.** Tarayıcının düzenlenebilir alanında imleç, yapıştırma, geri alma
   ve mobil klavyeler ayrı ayrı sorun çıkarır. Editör çekirdekleri bu işleri üstlenir; sıfırdan
   yazmamamızın nedeni bu.

## ⚖️ Editör çekirdeği karşılaştırması

Veriler 2026-11-27'de alındı. Kaynaklar: npm kayıt defteri (sürüm, tarih, lisans, haftalık indirme),
bundlephobia (boyut), GitHub depoları, her projenin kendi dokümanı.

| | Tiptap (ProseMirror üstünde) | Lexical | Slate | Yalın ProseMirror |
|---|---|---|---|---|
| Lisans | MIT | MIT | MIT | MIT |
| Son sürüm | 3.35.0 (2026-11-20, **7 gün**) | 0.53.0 (2026-10-28) | slate 0.147.4 (2026-10-26), slate-react 0.147.6 (2026-11-11) | view 1.44.0 (2026-11-24, **3 gün**) |
| 14 günlük kurala uyan sürüm | **3.34.0** (2026-11-13) | 0.53.0 | 0.147.4 / 0.147.6 | view 1.42.9 (2026-10-31), model 1.25.13, state 1.4.4 |
| Sürüm numarası | 3.x | 0.x | 0.x | 1.x |
| Haftalık indirme | `@tiptap/core` 38,5 milyon | `lexical` 9,5 milyon | `slate-react` 4,4 milyon | `prosemirror-view` 43 milyon |
| Boyut (min + gzip) | çekirdek 37 kB, React bağı 9 kB, altındaki ProseMirror 55 kB; bütün başlangıç seti 108 kB | `lexical` 69 kB; `rich-text` ile 81 kB, liste ayrı | slate 32 kB, slate-react 26 kB | view (model, state, transform dahil) 55 kB |
| React 19 | destekli (`^17 \|\| ^18 \|\| ^19`) | destekli (`>=18`) | destekli (`>=18.2`) | bağ yok; kendimiz yazarız |
| Yapı kuralı | ProseMirror şeması (bildirimsel) | düğüm sınıfları ve dönüşümler | hazır şema yok; kuralı kendimiz yazarız | ProseMirror şeması |
| Satır içi spoiler | bir işaret; kalın ve italikle üst üste biner | sarmalayan satır içi düğüm (bağlantı gibi) | metin parçasında alan | bir işaret |
| Erişilebilirlik | arayüz ve ARIA bizde; resmî rehber ve klavye örneği var | APG klavye modelini izler; `@lexical/a11y` hazır yardımcılar sunar | bizde | bizde |

Boyut notu: bundlephobia her paketi bağımlılıklarıyla ölçer; satırlar birbirinin üstüne biner ve
toplanamaz. Gerçek rakam adım 1'de bizim `npm run bundle` çıktımızdan okunur.

**Öneri: Tiptap 3.34.0.** Gerekçe:

- **Spoiler doğal oturuyor.** ProseMirror'da spoiler bir işarettir; "kalın ve spoiler" aynı metinde
  üst üste biner. Lexical'da biçimler metin düğümünde bayrak, sarmalayıcılar ayrı düğümdür (kendi
  dokümanı: bağlantı ve benzerleri satır içi `ElementNode`); spoiler için daha çok kod gerekir.
- **Şema bildirimsel.** İzin listemizle editörün şeması aynı biçimde yazılır; yapıştırılan HTML şemanın
  ayrıştırma kurallarından geçer.
- **Sürüm numarası.** Tiptap 3.x ve ProseMirror 1.x; Lexical ve Slate hâlâ 0.x.
- **Yalın ProseMirror'a göre** React bağı, komutlar ve klavye kısayolları hazır. Altında aynı çekirdek
  olduğu için ileride Tiptap'ten vazgeçersek belge biçimi değişmez.

Eksi yanlar (bilerek kabul):

- Erişilebilirlik hazır gelmiyor: araç çubuğu, ARIA rolleri ve klavye akışı bizim işimiz (adım 6,
  axe ve klavye testiyle). Lexical bu konuda daha çok hazır parça veriyor.
- Tiptap sık sürüm çıkarıyor (son dört sürüm 15 günde). Dependabot gürültüsü artar; 14 gün kuralı
  her güncellemede uygulanır.
- Bağımlılık sayısı artar (`@tiptap/pm` ProseMirror paketlerini getirir). Yalnız kullandığımız
  uzantılar kurulur; `starter-kit` kurulmaz.

Doğrulanamadı (adım 1 deneyinde bakılacak):

- gerçek paket boyutumuz;
- Tiptap'in sayfaya eklediği stilin CSP'mizle (`style-src 'self' 'unsafe-inline'`) davranışı;
- sıralı listenin JSON'unda öznitelik üretip üretmediği (karar 3'ü etkiler);
- ekran okuyucuyla gerçek kullanım. Yalnız axe ve klavye otomatik sınanabilir; ekran okuyucu elle
  denenmeden "çalışıyor" yazılmaz.

## 🎨 Kararlar (onaylandı, 2026-11-27)

| # | Karar | Gerekçe |
|---|---|---|
| 1 | Saklama: `reviews.body_doc jsonb`. HTML hiçbir yerde saklanmaz ve basılmaz. | Kavram 2. |
| 2 | Belge biçimi ProseMirror JSON'unun **alt kümesi**: `doc`, `paragraph`, `blockquote`, `bulletList`, `orderedList`, `listItem`, `text`; işaretler `bold`, `italic`, `spoiler`. | Editörün ürettiğiyle aynı; çeviri katmanı yok. Biçimi sunucudaki doğrulayıcı tanımlar, editör değil. |
| 3 | Sunucu doğrulayıcısı (`internal/richtext`) izin listesidir: bilinmeyen tür, işaret, öznitelik ya da alan → 400 `validation_error`. Sessizce ayıklama yok. | Reddetmek, "neyi sildim" belirsizliğinden iyidir; `DisallowUnknownFields` ile aynı tutum. |
| 4 | Sınırlar: görünen metin 1–5000 karakter (bugünkü sınır); iç içe derinlik, düğüm sayısı ve gövde boyutu sınırlı (rakamlar adım 3'te ölçülerek). | Derin ya da dev belge çizimi ve doğrulamayı yorar. |
| 5 | `body` kolonu kalır ve belgenin **düz metni** olur; `has_spoiler` kalır ve "belgede spoiler işareti var" anlamına gelir. İkisini de sunucu belgeden türetir. | Geriye uyum (eski arayüz sekmesi, geri dönüş) ve ileride arama. |
| 6 | Eski incelemeler okunurken çevrilir: `body_doc` boşsa `body` paragraflara bölünür; `has_spoiler` ise bütün metin spoiler işareti alır. Kalıcı çeviri daraltma PR'ında. | Genişlet → yayınla → daralt. |
| 7 | API: `PUT` hem yeni (`doc`) hem eski (`body` + `has_spoiler`) gövdeyi kabul eder; cevaplar `doc` ile birlikte eski alanları da taşır. Eski gövde desteği daraltma PR'ında kalkar. | Yayın anında açık duran eski sekme bozulmasın (issue kriter 9). |
| 8 | Okuma tarafı belgeyi kendi bileşeniyle (`ReviewDoc`) çizer; Tiptap yalnız düzenleyici açılınca yüklenir (ayrı parça). | Ziyaretçiye editör inmez. |
| 9 | Spoiler parçası: art arda gelen spoiler işaretli metinler tek parça; açılana kadar metin DOM'da yok; düğme `aria-expanded` taşır. | 17B'nin kuralı, parça başına. |
| 10 | Sürümlü yazım (409), silme, gizleme ve audit aynen. "Değişmedi" karşılaştırması belge üstünden yapılır. | 17B davranışı korunur. |

İki küçük karar: düğme metni "Spoiler". Spoiler kısayolu önce `Ctrl+Shift+S` olarak onaylandı;
adım 1 deneyi Firefox ve Edge'de belgeli bir çakışma bulduğu için **kısayolsuz** başlanıyor
([adım 1 kaydı](#adım-1--deney-kayıt), 2026-11-27).

## 🚫 Kapsam dışı

Başlık, resim, bağlantı, tablo, kod bloğu, renk, hizalama; yorumlarda zengin metin; saklama ya da
aktarma biçimi olarak Markdown (yazarken `**kalın**` gibi kısayollar açık, adım 1 kaydı); ortak
düzenleme; taslak kaydetme; sürüm geçmişi; sıfırdan editör.

## 🧭 Nasıl takip edilecek

17B ile aynı: asistan bir adımı yazar ve bu dosyada kaydını doldurur, commit etmez; kullanıcı okur;
"tamam" ile commit edilir. Testleri asistan koşturur. Dal: `feat/128-zengin-inceleme`. Tek özellik
PR'ı; daraltma ayrı PR.

## 🪜 Adımlar

1. **Deney (üretim kodu değil).** Ayrı bir klasörde en küçük Tiptap editörü: şema, spoiler işareti,
   JSON çıktısı. Ölçülecekler: paket boyutu, CSP, sıralı liste öznitelikleri, yapıştırma davranışı,
   klavye akışı, `Ctrl+Shift+S`'in tarayıcı kısayollarıyla çakışması. Sonuç bu dosyaya yazılır; öneri değişirse burada değişir.
2. **Şema: `0019_review_doc.sql` (genişletme).** `body_doc jsonb` (boş bırakılabilir) ve sqlc
   sorguları. Canlıdaki kod kolona dokunmaz.
3. **`internal/richtext`.** Doğrulayıcı, düz metin çıkarımı, "spoiler var mı", düz metinden belge.
   Tablo testleri ve fuzz (doğrulayıcı hiçbir girdide panik atmaz; kabul ettiği her belge yeniden
   doğrulanınca yine kabul edilir).
4. **Servis ve API.** `internal/reviews` belgeyle yazar; uçlar karar 7'deki iki gövdeyi okur.
5. **Arayüz veri katmanı ve `ReviewDoc`.** Tipler, kancalar, okuma bileşeni, parça başına spoiler.
6. **Düzenleyici.** Tembel yüklenen Tiptap, araç çubuğu (`role="toolbar"`, ok tuşlarıyla gezinme,
   `aria-pressed`), sayaç, 409 notu.
7. **Playwright, axe, dokümanlar, PR.**
8. **Ayrı PR, yayın doğrulandıktan sonra (daraltma):** kalan satırların kalıcı çevirisi,
   `body_doc NOT NULL`, eski gövde desteğinin kaldırılması.

### Adım 1 — Deney (kayıt)

- **Amaç:** önerilen çekirdeği üretim koduna dokunmadan denemek ve "doğrulanamadı" listesini kapatmak.
- **Nasıl:** depo dışında, geçici bir klasörde küçük bir Vite + React uygulaması; projeyle aynı React
  (19.3.1), Vite (8.3.8), vitest (5.1.0), jsdom (31.0.1) ve Playwright (1.65.0) sürümleri. Tiptap
  3.34.0 `--before 2026-11-13T13:30Z` ile kuruldu. Derlenen sayfa, `security-headers.conf`'taki CSP
  başlığıyla küçük bir Node sunucusundan sunuldu ve gerçek chromium'da klavye ve yapıştırma olaylarıyla
  denendi. Depoya kod ve bağımlılık girmedi.
- **Sonuç (2026-11-27):**
  - **Paket boyutu.** Düzenleyici ayrı parça olarak 335,6 kB (gzip **102,7 kB**). Karşılaştırma: bugünkü
    ana paketimiz gzip 94,9 kB. Yani düzenleyiciyi açan okur uygulamanın kendisinden büyük bir parça
    indirir; açmayan hiçbir şey indirmez (karar 8). Dağılım (her grup ayrı sıkıştırılınca; toplamları bu
    yüzden 103'ü aşar): ProseMirror 66 kB, Tiptap çekirdeği 27 kB, React bağı ve iki yardımcısı 9 kB,
    uzantılarımız 9 kB. Liste ve geri alma
    uzantıları alt yoldan içe aktarılınca (`@tiptap/extension-list/bullet-list` gibi) 110,6 → 102,7 kB.
  - **CSP.** Bugünkü başlıkla ihlal yok. Tiptap sayfaya bir `<style>` etiketi ekliyor; bu yalnız
    `'unsafe-inline'` sayesinde çalışıyor (`style-src 'self'` ile denendi: etiket engellendi, editör
    çalıştı ama boşluk kuralları uygulanmadı). `injectCSS: false` ile etiket hiç eklenmiyor ve iki
    başlıkta da ihlal yok.
  - **Sıralı liste öznitelikleri.** Varsayılan `orderedList` JSON'a `attrs: {start: 1, type: null}`
    yazıyor. `OrderedList.extend({ addAttributes: () => ({}) })` ile öznitelik üretmiyor; `5. ` yazım
    kuralı ve `<ol start="7" type="a">` yapıştırması hata vermeden özniteliksiz listeye dönüyor.
  - **Belge biçimi.** Spoiler işareti kalınla üst üste biniyor:
    `{"type":"text","marks":[{"type":"bold"},{"type":"spoiler"}],"text":"uşak"}`. Üç ayrıntı
    doğrulayıcıyı ve `ReviewDoc`'u etkiler: (1) kalın, spoiler'ın ortasında başlarsa spoiler **üç ayrı
    metin düğümüne** bölünür (karar 9'un "art arda gelenler tek parça" kuralı gerekli); (2) boş
    paragrafın `content` alanı **yok** (`{"type":"paragraph"}`), boş belge tek boş paragraftır;
    (3) işaretlerin sırası şemadaki sıradır, istemci elle başka sırada gönderebilir.
  - **Yapıştırma.** Başlık, bağlantı, üstü çizili, kod, renkli yazı, tablo, resim, `<script>`, `<pre>`,
    `<hr>` içeren HTML yapıştırıldı: şemada olmayan her şey düştü, metinleri paragraf olarak kaldı;
    kalın, italik, liste ve alıntı korundu. Word biçimli HTML temiz kalın ve italiğe döndü. Düz metinde
    her satır bir paragraf oldu. `onerror`, `onload`, `<script>` ve `iframe srcdoc` CSP **olmadan** da
    çalışmadı; resim isteği de gitmedi (içerik ayrı, etkisiz bir belgede ayrıştırılıyor). CSP açıkken
    konsola iki "inline event handler" ihlal satırı düşüyor; çalışan bir şey yok.
  - **Markdown benzeri kurallar varsayılan olarak açık.** `**kalın**`, `*italik*`, `> `, `- ` ve `1. `
    yazınca biçime dönüyor; düz metin olarak yapıştırılan `**kalın**` da kalın oluyor (yapıştırılan
    `1. bir` listeye dönmüyor). `enableInputRules` ve `enablePasteRules` ile kapatılabilir.
  - **Klavye.** `Ctrl+B`, `Ctrl+I`, `Ctrl+Shift+7/8` (liste), `Ctrl+Shift+B` (alıntı), `Ctrl+Z`,
    `Ctrl+Y` çalışıyor. Enter iki kez listeden çıkarıyor. **Tab:** paragrafta odağı editörden çıkarıyor
    (klavye tuzağı yok), liste maddesinde girinti yapıyor, girinti yapılamayan ilk maddede yine odağı
    çıkarıyor. `Shift+Enter` hiçbir şey yapmıyor (şemada satır sonu düğümü yok).
  - **İç içe liste sınırsız.** Sekiz Tab'la derinliği 21 olan bir belge üretildi. Karar 4'ün derinlik
    sınırı yalnız sunucuda kalırsa okur 400 alır; düzenleyici de aynı sınırda girintiyi durdurmalı.
  - **Erişilebilirlik öznitelikleri.** `role="textbox"`, `aria-multiline` ve `aria-label`
    `editorProps.attributes` ile verilebiliyor; editör `tabindex="0"` alıyor.
  - **jsdom.** Çekirdek editör jsdom'da kuruluyor; komutlar, `setContent` ve `getJSON` çalışıyor, React
    bileşeni çiziliyor (vitest 2/2). Yazma ve seçim jsdom'da denenmedi; onlar Playwright'ta kalır.
  - **Bağımlılıklar.** Çalışma zamanı için 30 yeni paket (11 Tiptap, 13 ProseMirror, 6 küçük yardımcı),
    hepsi MIT. `@tiptap/react` iki isteğe bağlı paket daha istiyor (`extension-bubble-menu`,
    `extension-floating-menu`; `@floating-ui` ile 5 paket): kilit dosyasına girer, pakete girmez.
    `--omit=optional` kullanılamaz, çünkü Vite'ın Windows'a özel yerel paketi de isteğe bağlı.
  - **`Ctrl+Shift+S`.** Chromium'da sayfa tuşu yakalıyor (`defaultPrevented`). Ama Firefox'un ve
    Edge'in kendi dokümanları aynı tuşu ekran görüntüsü aracına veriyor
    ([Firefox](https://support.mozilla.org/en-US/kb/take-screenshots-firefox),
    [Edge](https://learn.microsoft.com/en-us/deployedge/edge-learnmore-configurable-edge-commands)).
    Bu iki tarayıcıda hangisinin kazandığı sınanamadı: otomasyonun gönderdiği tuşlar tarayıcının kendi
    kısayollarını tetiklemez.
- **Deneyden çıkan kararlar (kullanıcı onayladı, 2026-11-27):**
  1. Sıralı liste özniteliksiz kurulur; belgede hiçbir düğümün `attrs` alanı olmaz. Karar 3 sadeleşir:
     `attrs` görülürse 400.
  2. `injectCSS: false`; gereken birkaç satır kendi stil dosyamıza yazılır. Böylece düzenleyici
     `'unsafe-inline'`'a yaslanmaz.
  3. Markdown benzeri yazım ve yapıştırma kuralları **açık kalır**: bilen okur `**kalın**`, `> `,
     `- `, `1. ` yazar, bilmeyen düğmeyi kullanır. Saklama biçimi JSON kalır; Markdown saklanmaz.
     Neden saklama biçimi değil: her metin geçerli Markdown olduğu için izin listesi (karar 3)
     uygulanamaz; Markdown içine HTML yazılabilir; spoiler Markdown'da tanımlı değil; sunucu (Go) ve
     tarayıcı ayrı ayrıştırıcılarla aynı metni farklı okuyabilir. İlk öneri "kapatılsın" idi;
     kullanıcının sorusu üzerine değişti.
  4. `Shift+Enter` için şemaya satır sonu eklenmez.
  5. Doğrulayıcı işaret sırasına bakmaz, aynı işaretin tekrarını reddeder; "değişmedi" karşılaştırması
     sıradan bağımsız yapılır.
  6. Spoiler kısayolu: **kısayolsuz başlanır**; düğme araç çubuğunda klavyeyle erişilir (kriter 7).
     Önce onaylanan `Ctrl+Shift+S` bırakıldı, çünkü çakışma belgeli ve sınanamıyor.
  7. Çekirdek değişmiyor: Tiptap 3.34.0. Bilerek kabul edilen bedel 103 kB'lık tembel parça.
- **Doğrulanamadı:** ekran okuyucuyla kullanım; Firefox, Safari ve mobil klavyelerde davranış (yalnız
  chromium denendi); `Ctrl+Shift+S`'in Firefox ve Edge'deki gerçek davranışı.
- **Deneydeki spoiler işareti** (adım 6'nın başlangıç noktası):

  ```ts
  export const Spoiler = Mark.create({
    name: "spoiler",
    parseHTML: () => [{ tag: "span[data-spoiler]" }],
    renderHTML: ({ HTMLAttributes }) => ["span", mergeAttributes(HTMLAttributes, { "data-spoiler": "" }), 0],
    addCommands() {
      return { toggleSpoiler: () => ({ commands }) => commands.toggleMark(this.name) };
    },
  });
  ```

### Adım 2 — Şema ve sorgular (kayıt)

- **Amaç:** belgenin duracağı kolonu, canlıdaki kodu bozmadan eklemek (genişletme).
- **Parçalar:**
  - `0019_review_doc.sql`: `reviews.body_doc jsonb`, boş olabilir. Tek kısıt: doluysa JSON **nesnesi**
    olmalı. Belgenin içini veritabanı değil `internal/richtext` doğrular (adım 3).
  - `reviews.sql`: liste `body_doc`'u da döndürüyor; yazım ve düzenleme `body_doc` parametresi alıyor.
    Parametre verilmezse NULL yazılır; düzenleme belgeyi her seferinde yeniden yazar (verilmezse
    boşaltır), yoksa eski belge yeni metnin yanında bayat kalırdı.
  - `sqlc.yaml`: boş olabilen `jsonb` → `json.RawMessage` (nil = SQL NULL).
  - Servis ve uçlar **değişmedi**: `internal/reviews` belgeyi henüz vermiyor, her yazım NULL bırakıyor.
    Davranış bugünküyle aynı; belge adım 4'te bağlanır.
- **Yeni kavram: JSON null, SQL NULL değildir.** `jsonb` kolonuna `'null'` yazılabilir; bu "değer yok"
  demek değildir, `IS NULL` onu yakalamaz. "Belgesi yok" durumunu tek anlama indirmek için kısıt JSON
  null'ı da reddediyor: kolonda ya SQL NULL ya bir nesne durur.
- **Sonuç (2026-11-27):**
  - `TestMigration0019ReviewDoc` yeşil. Sınadıkları: var olan inceleme yerinde ve belgesiz;
    **canlıdaki kodun sorgu metniyle** (`origin/main`'deki `reviews.sql.go`'dan aynen) yazım ve
    düzenleme 0019'dan sonra da çalışıyor ve belgeyi boş bırakıyor; nesne kabul, JSON null, dizi, metin
    ve sayı ret; Down kolonu kaldırıyor, incelemeler kalıyor.
  - Servisin yedi entegrasyon testi değişmeden yeşil: Go'daki nil belge kısıtın altında SQL NULL olarak
    yazılıyor (JSON null olsaydı kısıt reddederdi).
  - Bütün backend testleri gerçek Postgres ve Redis'le yeşil; `go vet` temiz; golangci-lint 0 sorun;
    sqlc `generate` ve `diff` temiz (Docker'daki `sqlc/sqlc:1.31.1` ile; yerelde sqlc kurulu değil).
- **Kilit:** varsayılanı olmayan, boş olabilen kolon eklemek katalog değişikliğidir; kısıt aynı komutta
  eklendiği için tablo bir kez taranır. Canlıda `reviews` 1 satır; `lock_timeout` 5 sn.
- **Doğrulanamadı:** canlıdaki kilit süresi (yayında görülür); CI'daki Yükseltme işi (`release` şeması
  ve verisi üstüne 0019) PR açılınca koşar.
- **Bakılacak yerler:**
  1. [`0019_review_doc.sql`](../../backend/migrations/0019_review_doc.sql): baştaki açıklama ve tek
     `ALTER TABLE`.
  2. [`reviews.sql`](../../backend/sql/queries/reviews.sql): `ListBookReviews`, `InsertReview`,
     `UpdateReview`.
  3. [`migrate_0019_integration_test.go`](../../backend/internal/database/migrate_0019_integration_test.go).
  4. [`sqlc.yaml`](../../backend/sqlc.yaml): yeni `jsonb` kuralı.
  5. `backend/internal/db/*.go`: sqlc'nin ürettiği kod; okumana gerek yok.

## ⚠️ Önemli noktalar

- Editör davranışı bileşen testiyle değil Playwright ile sınanır (gerçek tarayıcı); birim testleri
  saf fonksiyonlarda (belge ↔ metin, sayaç) kalır. jsdom'un `contenteditable` desteğinin yeterli olup
  olmadığına adım 1'de bakılır.
- Doğrulayıcı tek kapıdır: `ReviewDoc` bilinmeyen bir düğüm görürse çizmez (çökmez), ama güvenlik
  buna dayanmaz.
- Log kuralı aynen: belge ve metni loglanmaz.
- Yeni bağımlılıklar kurulurken kurulum tarihi sabitlenir (17B'deki `@axe-core/playwright` gibi);
  14 günden genç geçişli sürüm kilit dosyasına girmez.

## 🧪 Doğrulama (başarı ölçütleri)

Issue [#128](https://github.com/mtgezgin/BookHub/issues/128)'in dokuz kabul kriteri; her biri PR'da
bir teste eşlenir. Yayından sonra canlıda: biçimli inceleme yaz → kısmi spoiler aç → düzenle →
yönetici gizle ve geri aç.

## ✅ Tamamlanma

—
