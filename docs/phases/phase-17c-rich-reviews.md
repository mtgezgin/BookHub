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

### Adım 3 — `internal/richtext` (kayıt)

- **Amaç:** belgenin tek kapısı. Bu adımda paketi hiçbir şey çağırmıyor; servis adım 4'te bağlanır.
- **Parçalar:**
  - `Parse(raw)`: ham JSON'u izin listesine göre doğrular. Biçim dosyanın başında yazılı (hangi düğüm
    neyin içinde durabilir). Hata her zaman `ErrInvalid`'i sarar; sınır aşımı ayrıca `ErrLimit`,
    görünen metnin olmaması `ErrEmpty`.
  - `Doc.Text()`: düz metin; her paragraf bir satır. `reviews.body`'ye bu yazılacak.
  - `Doc.HasSpoiler()`: belgede spoiler işareti var mı. `reviews.has_spoiler`'a bu yazılacak.
  - `Doc.JSON()`: kanonik JSON; veritabanına ve cevaba bu gider.
  - `Equal(a, b)`: "değişmedi" karşılaştırması.
  - `FromPlain(body, spoiler)`: düz metinden belge (karar 6 ve 7).
- **Yeni kavramlar:**
  1. **Kanonik hâl.** Aynı içerik birden çok JSON'la yazılabilir (işaret sırası, metnin kaç düğüme
     bölündüğü, boş dizi ya da alanın hiç olmaması). `Parse` hepsini tek biçime indirir ve veritabanına
     o yazılır. İki kazanç: karşılaştırma basitleşir ve **saklanan şey doğrulanan şeyin kendisidir**,
     istemcinin gönderdiği baytlar değil.
  2. **Fuzz testi.** Elle yazılan örnekler aklımıza gelen hataları sınar. Fuzz, girdiyi rastgele
     bozarak milyonlarca kez dener ve bir *sözün* her girdide tuttuğuna bakar. Buradaki sözler: panik
     yok; kabul edilen belgenin kanonik hâli yeniden doğrulanınca aynı belgeyi verir; düz metin
     sınırların içindedir ve denetim karakteri taşımaz.
  3. **Mutasyon denemesi.** Testin işe yaradığını görmenin yolu koda bilerek hata koymaktır: test
     kırmızıya dönmüyorsa o satırı aslında sınamıyordur.
- **Kararlar (bu adımda verildi):**
  - **Sınırlar (karar 4'ün rakamları):** metin 5000 karakter (kod noktası; satır sonları dahil, çünkü
    `body`'nin CHECK'i onları da sayar), ham JSON 128 KiB, 2000 düğüm, 12 kat (bir alıntının içinde
    dört kat iç içe liste). Düzenleyici girintiyi bundan önce durdurmalı (adım 6).
  - **Alan adları harfi harfine.** Go'nun `encoding/json`'ı struct alanlarını büyük/küçük harfe
    bakmadan eşler (`"Type"` kabul edilirdi); düğümler bu yüzden elle açılıyor.
  - **Metin kuralları:** boş metin düğümü yok; sekme dışında denetim karakteri yok. Satır sonu da
    yasak: paragraf içinde satır sonu yoktur, yeni satır yeni paragraftır. NUL ayrıca Postgres'in
    kabul etmediği tek karakter.
  - **Hata mesajı girdiyi geri yazmaz:** yalnız sorunun yeri (`doc.content[2].marks[0]`) ve türü.
  - **Boş dizi hoş görülür:** `"content": []` ile alanın hiç olmaması aynı belgedir (kanonik hâlde
    alan yok). "Bilinmeyen şey → 400" kuralı aynen duruyor.
- **Sonuç (2026-11-27):**
  - 76 alt test yeşil: 6 geçerli örnek (`testdata/vectors.json`), 42 ret, sınırların tam üstü, kanonik
    hâl, `FromPlain`. Ret listesinde adım 1'in varsayılan çıktısı da var (`attrs`'lı sıralı liste).
  - Fuzz: `FuzzParse` 45 sn'de 8,4 milyon, `FuzzFromPlain` 30 sn'de 142 bin deneme; söz bozulmadı.
    Normal `go test` yalnız tohumları koşturur.
  - Mutasyon: bilerek konmuş 11 hatanın 8'i ilk seferde yakalandı. **İki boşluk bulundu ve
    kapatıldı:** derinlik testi sınırın iki üstünü deniyordu (bir üstü kaçıyordu); boyut testi aslında
    metin sınırına takılıyordu. Yakalanmayan sonuncusu eşdeğer bir değişiklik (JSON null zaten bir
    sonraki satırda reddediliyor).
  - Ölçüm (`BenchmarkParse`, bu makinede): 5000 karakterlik biçimli bir inceleme 1,3 ms ve 0,5 MB
    geçici bellek; düğüm sınırına dayanan en kötü belge 9,1 ms ve 4,2 MB. Yazma ucu için kabul
    edilebilir; kod sadelik için her katı yeniden çözüyor, hız gerekirse yeri belli.
  - Bütün backend testleri gerçek Postgres ve Redis'le yeşil; `go vet` temiz; golangci-lint 0 sorun.
- **Doğrulanamadı / sonraya kalan:**
  - Arayüzdeki sayaç aynı kuralla saymalı (kod noktası, paragraf başına bir satır sonu). Adım 5'te
    TypeScript tarafı `vectors.json`'ın aynısıyla sınanacak; o zamana kadar iki taraf eşlenmiş değil.
  - Fuzz yalnız yerelde ve kısa koştu; CI'da sürekli fuzz yok.
- **Bakılacak yerler:**
  1. [`richtext.go`](../../backend/internal/richtext/richtext.go): baştaki açıklama (biçim), sınırlar,
     `Parse`, `checkContent`.
  2. [`derive.go`](../../backend/internal/richtext/derive.go): `Text`, `HasSpoiler`, `FromPlain`.
  3. [`richtext_test.go`](../../backend/internal/richtext/richtext_test.go): `TestParseRejects` tablosu.
  4. [`fuzz_test.go`](../../backend/internal/richtext/fuzz_test.go): `FuzzParse`'ın sözleri.

### Adım 4 — Servis ve API (kayıt)

- **Amaç:** belgeyi yazma ve okuma yoluna bağlamak; bugünkü arayüz hiçbir şey fark etmemeli.
- **Parçalar:**
  - `internal/reviews`: `Draft` artık metin ve bayrak değil, doğrulanmış **belge** taşıyor. Servis üç
    kolonu aynı belgeden yazıyor: `body_doc` (kanonik JSON), `body` (düz metin), `has_spoiler`
    (belgede spoiler işareti var mı). "Değişmedi" karşılaştırması belge üstünden (`unchanged`).
  - `PUT /api/books/{id}/review` iki gövde kabul ediyor (karar 7):
    yeni `{"doc": {...}, "version": n}`, eski `{"body": "...", "has_spoiler": b, "version": n}`.
    İkisi birlikte gelirse 400. Eski gövde bugünkü kurallarıyla denetlenip belgeye çevriliyor.
  - Cevaplar (liste ve "benim incelemem") `doc` alanını da taşıyor; **hiçbir zaman null değil**:
    belgesi olmayan eski inceleme düz metninden çevriliyor (karar 6). `body` ve `has_spoiler` duruyor.
  - `readJSON`'daki NUL taraması bayt dilimlerini atlıyor (belgeyi kendi doğrulayıcısı denetliyor).
- **Adım 2'ye göre değişen:** orada "eski gövdeyle gelen istek belgeyi boş bırakır" yazıyordu. Yeni kodda
  eski gövde de belgeye çevrilip yazılıyor; belgesiz satır yalnız **eski kodun** yazdığıdır (bugünkü
  canlı satır ve yayın sırasında eski sürümün yazdıkları). Böylece daraltma PR'ına az iş kalır.
- **Davranış farkları (bilerek):**
  - Eski gövdede satır sonları `\n`'e iner, sekme dışındaki denetim karakterleri atılır (belgede
    yasaklar). Bugünkü arayüz bunları zaten üretmiyor.
  - Eski gövdeyle 1000'den çok satırlı bir inceleme artık 400 alır (düğüm sınırı: satır başına iki
    düğüm). 5000 karakterde 1000 satır gerçek bir inceleme değil.
  - Belge yolunda metin kırpılmaz (baştaki boş paragraf metne `\n` olarak girer); "yalnız boşluk" yine
    400.
  - Kaydedilmiş bir belgenin `doc` alanı veritabanının yazdığı hâliyle döner (`jsonb` anahtar sırasını
    ve boşlukları değiştirir). İçerik kanoniktir, baytlar değil.
- **Sonuç (2026-11-27):**
  - Servis (gerçek Postgres): `TestDocRealDB` yeşil. Üç kolon belgeden türüyor; aynı belgenin başka
    yazımı (işaret sırası, bölünmüş metin) düzenleme sayılmıyor; yalnız biçim değişirse düzenleme;
    belgesiz eski satırın aynısı yazılmıyor, düzenlenince belgesini alıyor. Var olan yedi servis testi
    belgeye çevrilmiş taslaklarla yeşil (sürüm, 409, yarış, gizleme aynen).
  - Uçlar: 15 yeni PUT senaryosu (geçerli belge, bilinmeyen düğüm, bağlantı işareti, `attrs`, boş,
    5001 karakter, NUL, HTML metni, iki gövde birlikte, `doc: null`, ikisi de yok, çok satırlı eski
    gövde). Hata mesajı girdiyi geri yazmıyor. Sözleşme testi iki cevapta da `doc`'u arıyor.
  - Bütün backend testleri gerçek Postgres ve Redis'le yeşil (22 paket); `go vet` temiz; golangci-lint
    0 sorun; sqlc `diff` temiz.
  - **Bugünkü arayüz, yeni API:** `scripts/e2e.sh e2e/rating-comment.spec.ts` 2/2 yeşil (inceleme
    senaryosu 3,7 sn: yaz, düzenle, iki sekmede 409, gizlenince gerekçe, klavyeyle spoiler, axe, sil).
    Yığın bu dalın API'siyle kuruldu (yerel şema sürümü 19); arayüz değişmedi, yani eski gövdeyi
    gönderdi. Karar 7'nin "açık duran eski sekme bozulmaz" sözünün yereldeki kanıtı bu.
- **Doğrulanamadı / sonraya kalan:** yeni gövdenin tarayıcıdan gönderilmesi (arayüz adım 5 ve 6);
  yayın sırasında eski ve yeni sürümün aynı anda yazması (canlıda görülür; kural 0019 testinde ve
  `unchanged`'de sınandı).
- **Bakılacak yerler:**
  1. [`service.go`](../../backend/internal/reviews/service.go): `Draft`, `create`, `edit`, `unchanged`,
     `DocJSON`.
  2. [`reviews.go`](../../backend/cmd/api/reviews.go): `saveReviewRequest` ve `document()`.
  3. [`service_integration_test.go`](../../backend/internal/reviews/service_integration_test.go):
     `TestDocRealDB`.
  4. [`reviews_test.go`](../../backend/cmd/api/reviews_test.go): `TestSaveReviewRequests`'in "belge:"
     satırları.

### Adım 5 — Arayüz veri katmanı ve `ReviewDoc` (kayıt)

- **Amaç:** belgeyi okuma tarafında çizmek. Yazma tarafı (düzenleyici) bu adımda değişmedi: hâlâ düz
  metin kutusu ve eski gövde.
- **Parçalar:**
  - `lib/reviewDoc.ts`: belge tipi ve saf fonksiyonlar. `docText` ve `docLength` sunucudaki `Text` ile
    aynı kuralı uygular (paragraf başına bir satır, uzunluk **kod noktası**); `plainToDoc` sunucudaki
    `FromPlain`'in aynısı; `docOf` incelemenin belgesini verir.
  - `components/ReviewDoc.tsx`: ağacı React öğelerine çevirir. `dangerouslySetInnerHTML` yok; metin her
    zaman metin olarak basılır. Tanınmayan düğüm çizilmez.
  - `api/types.ts`: cevaplarda `doc`; `ReviewDraft` iki biçimli (belge **ya da** düz metin), ikisini
    birlikte göndermek derleme hatası.
  - `ReviewSection` ve `MyReview` metni `ReviewDoc` ile çiziyor.
- **Kararlar (bu adımda verildi):**
  - **Parça kuralı (karar 9):** bir paragraftaki art arda spoiler metinleri tek parça; aradaki kalın
    onları bölmez. Parçanın düğmesi yerinde kalır (açınca odak kaybolmaz), aynı düğme kapatır.
  - **Tamamı spoiler olan inceleme tek düğmeyle açılır.** Eski "spoiler içeriyor" kutusuyla yazılmış
    incelemeler böyle; kural olmasa beş paragraflı bir incelemede beş düğme çıkardı. Görünüm ve düğme
    metni 17B'dekiyle aynı.
  - **Kendi incelemem:** spoiler açık görünür ve işaretlidir (`<mark>`); okur neyi gizlediğini görsün.
  - **`doc` alanı tipte isteğe bağlı.** Sunucu her zaman gönderir, ama yayın sırasında yeni arayüz
    birkaç dakika eski API ile konuşabilir; o zaman `docOf` düz metinden çevirir.
- **Sonuç (2026-11-27):**
  - vitest 519 → 547, hepsi yeşil; `typecheck` temiz.
  - **Ortak örnekler:** `reviewDoc.test.ts`, sunucunun `testdata/vectors.json` dosyasını okuyor; altı
    örnekte düz metin, spoiler ve uzunluk iki tarafta aynı. Adım 3'te "eşlenmiş değil" diye yazılan
    açık kapandı.
  - `ReviewDoc.test.tsx`: yapı (paragraf, kalın, italik, alıntı, iç içe liste), boş paragraf, HTML'in
    metin kalması (`<img onerror>` ve `<script>` öğeye dönüşmüyor), tanınmayan düğüm, parça kuralı,
    parçaların ayrı ayrı açılması, tamamı spoiler, yazar kipi.
  - Mutasyon: spoiler metnini DOM'dan çıkarmak yerine `hidden` ile gizleyince 3 test, parçaları
    birleştirmeyince 2 test kırmızı.
  - Paket: `ReviewSection` parçası gzip 3,45 → 4,23 kB; ana paket aynı (94,89 kB).
  - **Yeni arayüz, yeni API:** `scripts/e2e.sh e2e/rating-comment.spec.ts` 2/2 yeşil, senaryo
    değişmeden (yaz, düzenle, iki sekmede 409, gizleme, klavyeyle spoiler, axe, sil). Okuma artık
    `ReviewDoc`'tan geçiyor; yazma hâlâ eski gövdeyle.
- **Yayın sırası riski (adım 7'de karar):** web ve API ayrı servisler ve aynı commit'ten ayrı ayrı
  yayınlanıyor. Okuma tarafı iki sırada da çalışır (`docOf`). Yazma tarafı adım 6'da belge göndermeye
  başlayınca, yeni arayüz **eski** API'ye denk gelirse kaydetme 400 alır (eski API `doc` alanını
  tanımıyor). Seçenekler: API'yi ayrı PR'la önce yayınlamak ya da birkaç dakikalık pencereyi kabul etmek.
- **Doğrulanamadı / sonraya kalan:** parça parça spoiler'ın tarayıcıda klavye ve axe ile denenmesi
  (adım 7; belgeyi gönderecek düzenleyici adım 6'da geliyor); ekran okuyucu.
- **Bakılacak yerler:**
  1. [`reviewDoc.ts`](../../frontend/src/lib/reviewDoc.ts): `docText`, `docLength`, `isAllSpoiler`,
     `plainToDoc`.
  2. [`ReviewDoc.tsx`](../../frontend/src/components/ReviewDoc.tsx): `Block`, `inline`, `SpoilerPiece`.
  3. [`ReviewDoc.test.tsx`](../../frontend/src/components/ReviewDoc.test.tsx): "spoiler parçaları".
  4. [`types.ts`](../../frontend/src/api/types.ts): `ReviewDraft`.

### Adım 6 — Düzenleyici (kayıt)

- **Amaç:** okur incelemesini biçimli yazsın ve yalnız spoiler olan kısmı işaretlesin; gönderilen şey
  belge olsun.
- **Parçalar:**
  - `frontend/package.json`: Tiptap 3.34.0 (11 paket). Kurulum `--before 2026-11-13T23:59:59Z` ile;
    kilit dosyasına 32 paket girdi (ikisi isteğe bağlı, pakete girmiyor), hepsi MIT, var olan hiçbir
    paketin sürümü değişmedi.
  - `components/review-editor/extensions.ts`: şema. Sunucudaki izin listesinin aynısı; spoiler işareti,
    özniteliksiz sıralı liste, Tab bağlanmamış liste maddesi.
  - `components/review-editor/ReviewRichEditor.tsx`: editör ve araç çubuğu (ayrı, tembel parça).
  - `components/review-editor/editor.css`: Tiptap'in eklediği `<style>` etiketinin yerine geçen birkaç
    satır ve içerik görünümü.
  - `MyReview.tsx`: metin kutusu ve "Spoiler içeriyor" kutusu gitti; form belge gönderiyor. Sayaç
    `docLength` (kod noktası), kaydetmeden önce `docProblem` (boş, uzun, fazla iç içe).
- **Kararlar (bu adımda verildi):**
  - **Tab hiçbir zaman girinti yapmaz, hep odağı ilerletir.** Adım 1'de ölçülen varsayılan davranışta
    Tab liste maddesinde girinti yapıyordu; o zaman klavyeyle gezen okur listeden çıkmak için içeriği
    değiştirmek zorunda kalıyor. Kriter 7 ("klavye tuzağı yok") için Tab ve Shift+Tab bağlanmadı.
    **Bedeli:** düzenleyicide iç içe liste yazılamaz. Yapıştırılan iç içe liste korunur, okuma tarafı
    çizer. Adım 3'teki "düzenleyici girintiyi durdurmalı" maddesi böylece gereksizleşti. İstenirse
    araç çubuğuna "girintiyi artır / azalt" düğmeleri sonra eklenebilir.
  - **Araç çubuğu düğmeleri yazılı** (Kalın, İtalik, Spoiler, Alıntı, Madde listesi, Numaralı liste):
    görünen ad ile erişilebilir ad aynı; simge ezberletmiyor.
  - **Klavyeyle basılan düğme odağı editöre geri verir**; okur yazmaya devam eder. Çubuk kaldığı
    düğmeyi hatırlar (Shift+Tab ile dönünce aynı düğme).
  - **`aria-invalid` editörün seçeneklerinden yazılır**, bir etkiden DOM'a değil (aşağıdaki hata).
- **Bulunan hata (Playwright buldu, birim testi bulamadı):** ilk sürümde `aria-invalid` bir `useEffect`
  içinde `editor.view.dom`'a yazılıyordu. jsdom'daki 549 test yeşildi; derlenmiş pakette düzenleyici
  açılır açılmaz çöktü ("The editor view is not available": görünüm o anda henüz bağlanmamış). Adım
  1'in "editör davranışı Playwright ile sınanır" notunun somut karşılığı.
- **Sonuç (2026-11-27):**
  - vitest 547 → 554, `typecheck` temiz. Yeni birim testleri: belge gönderimi ve seçili metne spoiler;
    araç çubuğu (tek Tab durağı, ok tuşları, Home/End, `aria-pressed`, `aria-controls`); özniteliksiz
    sıralı liste; kod noktası sayacı ve 5001 karakterde gönderilmeme; `docProblem` sınırları (5000/5001
    karakter, 12/13 kat, 2000/2001 düğüm: sunucununkilerle aynı).
  - **Playwright (gerçek chromium, NGINX ve CSP arkasında):** inceleme senaryosu art arda 4 koşuda
    yeşil (6 sn). Sınadıkları:
    - **yalnız klavyeyle yazma:** metin, Shift+Tab ile araç çubuğu, ok tuşlarıyla "Spoiler", Enter;
      odak editöre döner; ikinci Shift+Tab aynı düğmeye gelir; Tab editörden "Vazgeç"e, oradan
      "Kaydet"e geçer (tuzak yok);
    - **Markdown kısayolu:** `**kalın**` yazınca kalına dönüyor;
    - **axe:** düzenleyici açıkken, spoiler parçası kapalı ve açıkken ihlal yok;
    - **CSP:** `style-src` ihlali yok, sayfada Tiptap'in `<style>` etiketi yok;
    - **parça parça spoiler (kriter 2):** öteki okurun incelemesinde açık metin ve liste okunuyor,
      spoiler parçası açılana kadar sayfanın kaynağında yok, Enter ile açılıyor, odak düğmede kalıyor;
    - **yapıştırma:** başlık, bağlantı, resim ve `<script>` içeren HTML'den yalnız metin kalıyor,
      işleyici çalışmıyor;
    - düzenleme belgeyi biçimiyle açıyor; iki sekmede 409, gizliyken düzenleme ve silme aynen.
    Kitap sayfasına ve parça yüklemeye dokunan öteki dosyalar (`cache-headers`, `resilience`,
    `library`) 7/7 yeşil. Tam paket yerelde koşmadı; CI'da koşar.
  - **Paket:** düzenleyici parçası 342 kB (gzip **104,9 kB**) + 1,1 kB stil; yalnız "İnceleme yaz" ya
    da "Düzenle" denince iner. İlk açılışta inen toplam değişmedi: önce `index` + `toast` gzip 123,6 kB,
    şimdi `index` + `utils` 122,1 kB (derleyici ortak kodu yeniden böldü). İncelemeler parçası gzip
    4,23 → 4,70 kB.
  - Dependabot: npm güncellemeleri küçük ve yama sürümlerde zaten gruplu; Tiptap paketleri (birbirini
    tam sürümle istiyor) birlikte güncellenir.
- **Doğrulanamadı / sonraya kalan:**
  - Ekran okuyucuyla kullanım (axe ve klavye otomatik sınandı; ekran okuyucu denenmedi).
  - Firefox, Safari ve mobil klavyeler: yalnız chromium koştu.
  - Araç çubuğundaki ipuçları kısayolu "Ctrl+…" diye yazıyor; macOS'te tuş Cmd.
  - Düzenleyici parçası yüklenemezse (ağ koptu) ne göründüğü ayrıca denenmedi; uygulamanın genel
    parça yenileme ve hata sınırı devrede.
- **Bakılacak yerler:**
  1. [`extensions.ts`](../../frontend/src/components/review-editor/extensions.ts): şema ve üç uyarlama.
  2. [`ReviewRichEditor.tsx`](../../frontend/src/components/review-editor/ReviewRichEditor.tsx):
     `useEditor` seçenekleri ve `Toolbar`.
  3. [`MyReview.tsx`](../../frontend/src/components/MyReview.tsx): `ReviewEditor`.
  4. [`rating-comment.spec.ts`](../../frontend/e2e/rating-comment.spec.ts): `writeAndEdit`,
     `spoilerPieceOpensAlone`, `pasteHostileHtml`.
  5. [`editor.css`](../../frontend/src/components/review-editor/editor.css).

### Adım 7 — Uçtan uca test, dokümanlar, PR (kayıt)

- **Parçalar:**
  - `rating-comment.spec.ts`: adım 6'daki senaryo iki kriter için genişledi. Kriter 1: italik, liste
    ve alıntı da klavyeyle yazılıyor, kayıttan **ve sayfa yenilendikten** sonra aynı biçim aranıyor.
    Kriter 8: düzenleyici parçası "İnceleme yaz"dan önce hiç istenmemiş, sonra bir kez istenmiş.
  - PROJECT_MAP (`reviews.go`, `reviews`, yeni `richtext`, `0019`, `ReviewDoc`, `review-editor/`,
    `lib/reviewDoc.ts`), ROADMAP ve STATUS.
  - Aşağıdaki [kriter tablosu](#-doğrulama-başarı-ölçütleri).
- **Sonuç (2026-11-27):** inceleme senaryosu genişlemiş hâliyle yeşil (7,2 sn). Giriş bütçesi
  değişmedi (yeni giriş yok). Tam Playwright paketi yerelde koşmadı; PR'da CI koşar.
- **Yayın sırası (2026-11-28):** iki PR. Önce backend (adım 2–4: `0019`, `internal/richtext`, uçlar),
  canlıda doğrulanır; sonra arayüz (adım 5–7). Neden: web ve API aynı commit'ten ayrı ayrı
  yayınlanıyor; tek PR'da yeni arayüz eski API'ye denk gelirse kaydetme 400 alırdı. "Nasıl takip
  edilecek" bölümündeki "tek özellik PR'ı" cümlesi bununla değişti. Kullanıcı PR'ın açılmasını ve
  merge'ü onayladı; sırayı ayrıca seçmedi, önerilen yol uygulandı.

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

| # | Kriter | Nerede sınanıyor |
|---|---|---|
| 1 | Biçimli inceleme, yenilemeden sonra aynı biçimle | Playwright `writeAndEdit`: kalın, italik, liste, alıntı, spoiler klavyeyle; kayıttan ve yenilemeden sonra `formatted()` |
| 2 | Açılmamış spoiler parçası DOM'da yok; parçalar ayrı açılır | Playwright `spoilerPieceOpensAlone` (sayfa kaynağında yok); `ReviewDoc.test.tsx` "spoiler parçaları" |
| 3 | İzin listesi dışı → 400, hiçbir şey yazılmaz | `richtext` `TestParseRejects` (42 satır) ve fuzz; `TestSaveReviewRequests` "belge:" satırları (servis çağrılmıyor) |
| 4 | Uzunluk görünen metne göre | `richtext` `TestVectors` (madde imi sayılmaz), 5000/5001 testleri; arayüzde `reviewDoc.test.ts` aynı örnekler |
| 5 | 17B incelemesi eskisi gibi; spoiler'lıysa tek parça | `TestFromPlain`, `TestReviewContracts` (belgesiz inceleme), `ReviewDoc.test.tsx` "tamamı spoiler", Playwright `spoilerIsKeyboardAccessible` (eski gövdeyle yazılmış inceleme) |
| 6 | 409, silme, gizleme, geri açma aynen | `internal/reviews` entegrasyon testleri (değişmeden yeşil) ve `TestDocRealDB`; Playwright `twoTabConflict`, `hiddenReviewShowsReason`, silme |
| 7 | Yalnız klavye, tuzak yok, axe temiz | Playwright `writeAndEdit` (Shift+Tab, ok tuşları, Enter, Tab ile çıkış) ve üç axe taraması; birim testi "araç çubuğu" |
| 8 | Editör yazmayana inmez | Playwright: `ReviewRichEditor` parçası "İnceleme yaz"dan önce 0, sonra 1 istek; `npm run bundle` (ayrı parça, gzip 104,9 kB) |
| 9 | Eski sekme kaydedebilir, kısmi spoiler'ı açıkta göstermez | `TestSaveReviewRequests` (eski gövde 201); `TestDocRealDB` (kısmi spoiler'da `has_spoiler` true: eski arayüz bütün metni kapatır); adım 4'te bugünkü arayüzle E2E |

## ✅ Tamamlanma

—
