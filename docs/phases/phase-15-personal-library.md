# Faz 15 — Kişisel kitaplık

> Durum: 🧪 uygulandı, yayın ve canlı doğrulama bekliyor (2026-10-18) · Issue: [#42](https://github.com/mtgezgin/BookHub/issues/42) ·
> [ROADMAP](../ROADMAP.md): **Şimdi** · Backlog:
> [FEATURE_REQUESTS](../FEATURE_REQUESTS.md) Paket A — LIB-01, LIB-02, CAT-01'in kitaplık bölümü,
> SOC-02'nin "varsayılan gizli" kuralı · İlgili: [ratings.sql](../../backend/sql/queries/ratings.sql),
> [BookDetail.tsx](../../frontend/src/pages/BookDetail.tsx), [Home.tsx](../../frontend/src/pages/Home.tsx)

## 🎯 Amaç

Kullanıcının kitaplarını tek yerde tuttuğu, gizli bir kişisel kitaplık: kitabı bir durumla eklemek,
durumunu değiştirmek, çıkarmak ve "Kitaplığım" sayfasında aramak, süzmek, sıralamak. Ürün döngüsünün
ilk halkası: **keşfet → kitaplığına ekle**.

## 🔍 Başlangıç durumu (neden)

- Kullanıcının biriktirdiği ve geri döndüğü bir alan yok; giriş yapmanın tek faydası puan, yorum ve
  öneri ([FEATURE_REQUESTS § Durum özeti](../FEATURE_REQUESTS.md#durum-özeti-17-ekim-2026): LIB-01
  ve LIB-02 yok).
- Kullanıcıya ait veri için kalıp hazır: puanlar `ratings (user_id, book_id)` anahtarıyla tutuluyor
  ve uçlarda kullanıcı adreste değil, token'da (`/api/books/{id}/rating`).
- Katalogda arama, tür filtresi, adreste tutulan filtre durumu, sayfalama ve bayat yanıt koruması
  var ([Home.tsx](../../frontend/src/pages/Home.tsx)); Kitaplığım bunları uyarlar.

## 🧠 Yeni kavram(lar)

- **Kullanıcıya ait, gizli kaynak:** puan herkese açık bir istatistiğe katılır, kitaplık katılmaz.
  Gizlilik arayüzde değil API'de uygulanır: başkasının kitaplığını döndüren bir uç **yok**, var olan
  her sorgu `user_id = token'daki kullanıcı` ile başlar.
- **Bileşik anahtarın sol öneki:** `PRIMARY KEY (user_id, book_id)` hem "kullanıcı başına kitap
  başına tek kayıt" kuralını garanti eder hem de `WHERE user_id = $1` sorgularına index olur; ayrı
  bir index gerekmez.
- **Toplu okuma (N+1'den kaçınmak):** kartlardaki rozetler için kitap başına bir istek yerine tek
  istek: `book_id = ANY($ids)`. Türleri toplu çeken mevcut kalıbın aynısı.
- **Herkese açık ile kişisel veriyi ayırmak:** kişisel durum katalog yanıtına gömülmez; katalog uçları
  ve ortak önbellek (en sevilenler) herkes için aynı kalır, kişisel veri ayrı bir uçtan gelir.

## 🎨 Kararlar

| Konu | Karar | Neden |
| --- | --- | --- |
| Durumlar | `WANT_TO_READ`, `READING`, `READ`, `ABANDONED`; arayüzde Okumak istiyorum, Şu an okuyorum, Okudum, Yarım bıraktım | LIB-01'in ilk kapsamı; `TEXT + CHECK` (§18.3) |
| Puanla ilişki | Bağımsız: puan vermek kitaplığa eklemez, kitaplıktan çıkarmak puanı ve yorumu silmez | Kullanıcı kararı (2026-10-17); mevcut puanlar için veri taşıma gerekmez |
| Kart rozeti | Var; ayrı toplu istekle (`/api/me/library/statuses`) | LIB-01 kabul kriteri "kart, detay ve kitaplıkta tutarlı"; katalog uçları ve ortak önbellek değişmez |
| Kitaplığım kapsamı | Backlog'un tamamı: durum sekmeleri, arama, tür, "Puanım", üç sıralama, sayfalama, hepsi adreste | Kullanıcı kararı; çoğu katalogdan uyarlanıyor |
| Detay kontrolü | Bölünmüş düğme, başlık ve türlerin altında | Kullanıcı seçimi; tek tıkla ekleme, mobilde ilk ekranda |
| Yayından kalkan kitap | Kayıt silinmez; Kitaplığım'da soluk, "Katalogdan kaldırıldı", bağlantısız; yalnız çıkarılabilir | Kullanıcının verisi; kitaplığa yalnız yayındaki kitap eklenir |
| Katman | Service yok; repository (`internal/library`) + handler (`cmd/api/library.go`) | Düz CRUD; kurallar SQL'de (GO_CONVENTIONS §4) |
| Paylaşım | Yok; kitaplığı yalnız sahibi görür | SOC-02'nin P0 yarısı; herkese açık kitaplık P1 |

## 🚫 Kapsam dışı

- Okuma ilerlemesi, tarih, yeniden okuma (Faz 17: READ-01–03). "Şu an okuyorum" bu fazda yalnız bir durum.
- Favoriler ve özel raflar (LIB-03); kitaplık paylaşımı (SOC-02'nin P1 yarısı); dışa aktarma (DATA-01).
- Durum başına sayaçlar (sekmelerde "Okudum (12)"); toplu düzenleme.
- Kitaplık yazımlarına işlem kotası (REVIEW_FEEDBACK F04, Faz 18'den önce).
- Aramada Türkçe büyük/küçük harf davranışı: katalogdaki `ILIKE` ile aynı (DISC-01'in açık maddesi).
- Demo verisi: demo okur hesaplarıyla giriş yapılamadığı için kitaplık tohumlanmaz.

## 🗺️ Bölümler

Tek PR, tek migration (yalnız yeni tablo — genişletme, daraltma yok). Aynı PR'da Faz 16 spec'indeki
migration numaraları 0010/0011 → 0011/0012 olarak güncellenir.

## 🪜 Adımlar

### 1. Şema: `0010_library_entries.sql`

```sql
CREATE TABLE library_entries (
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    book_id    bigint NOT NULL REFERENCES books (id) ON DELETE CASCADE,
    status     text NOT NULL CHECK (status IN ('WANT_TO_READ', 'READING', 'READ', 'ABANDONED')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, book_id)
);
```

- Ek index yok (sol önek yetiyor); ölçüm gerekirse `perf_seed.sql` ile sonra.
- Down: `DROP TABLE library_entries;`.
- Çalışan kodla uyumlu: eski kod tabloyu bilmez.

### 2. Sorgular: `backend/sql/queries/library.sql`

- **Ekle / güncelle:**
  `INSERT … SELECT $user, b.id, $status FROM books b WHERE b.id = $book AND b.status = 'APPROVED'`
  `ON CONFLICT (user_id, book_id) DO UPDATE SET status = EXCLUDED.status, updated_at = now()`.
  Satır dönmezse kitap yok ya da yayında değil → 404. Sonuç: arşivlenmiş bir kitabın durumu
  değiştirilemez, yalnız çıkarılabilir.
- **Çıkar:** `DELETE … WHERE user_id = $1 AND book_id = $2`; etkilenen satır sayısına bakılmaz (idempotent).
- **Toplu durum:** `SELECT book_id, status … WHERE user_id = $1 AND book_id = ANY($2::bigint[])`.
- **Liste ve sayı:** `library_entries` + `books` (+ `LEFT JOIN ratings` aynı kullanıcı ve kitap için),
  `books` kolonları `sqlc.embed(b)` ile (üretilen kodda tek tek ad olarak açılır; sonuç `db.Book`,
  mevcut `toBookResponse` aynen kullanılır). Filtreler: durum, `q` (başlık/yazar `ILIKE`,
  `books.EscapeLike`), tür (`EXISTS` + slug), `rated` (puan var mı yok mu). Sıralama tek
  sorguda `CASE` ile:
  - `added`: `le.created_at DESC`
  - `title`: `b.title COLLATE "tr-x-icu"` artan
  - `my_rating`: `r.score DESC NULLS LAST`, sonra `le.created_at DESC`
  - her durumda son anahtar `le.book_id` (kararlı sayfalama).
  - Toplam aynı sorguda: `count(*) OVER ()` (ayrı bir sayma sorgusu, dolayısıyla `WHERE`'in ikinci kopyası
    yok). Sayfa sonun ötesindeyse satır dönmez; repository o zaman toplamı ilk satırdan okur.
- Liste öğelerinin türleri mevcut toplu tür sorgusuyla tek seferde gelir.

### 3. API: `backend/cmd/api/library.go` (`requireAuth` grubunda)

| Uç | Gövde / parametre | Başarı |
| --- | --- | --- |
| `PUT /api/me/library/{bookId}` | `{"status": "READ"}` (ayrı DTO, `DisallowUnknownFields`) | 200 `{book_id, status, added_at, updated_at}` |
| `DELETE /api/me/library/{bookId}` | — | 204 |
| `GET /api/me/library/statuses` | `?book_ids=54,53` (en çok 100, tekrarlar ayıklanır; boşsa `{}`) | 200 `{"statuses": {"54": "READ"}}` |
| `GET /api/me/library` | `?status=&q=&genre=&rated=yes\|no&sort=added\|title\|my_rating&limit=&offset=` | 200 `{items, total, limit, offset}` |

- Liste öğesi: `{book, status, added_at, my_rating, in_catalog}`. `book` mevcut `bookResponse`
  (türleriyle), `my_rating` sayı ya da `null`, `in_catalog` kitap APPROVED değilse `false`. Yeni bir
  sözleşme testi alan izin listesini sabitler.
- Sayfalama katalogdaki `parsePage` ile (aynı varsayılan ve tavan); tür filtresi katalogdakiyle aynı
  davranır.
- Hatalar mevcut zarfla: `404 not_found` (kitap yok ya da yayında değil — ikisi ayırt edilmez),
  `400 validation_error` (durum, `rated`, `sort`, `limit`), `400 invalid_id` (bozuk kimlik, 100'den
  fazla kimlik), `401` (giriş yok).
- Önbellek yok; mevcut önbelleklere dokunulmaz.

### 4. Arayüz

- **Türler ve istemci:** `api/types.ts`'e `LibraryStatus`, `LibraryItem`, `LibraryPage`;
  `client.ts`'e dört uç. Durum etiketleri tek bir eşlemede (`lib/format.ts`).
- **`ui/dropdown-menu.tsx`:** shadcn kalıbında, mevcut `radix-ui` paketinden (yeni bağımlılık yok).
- **`LibraryButton` (kitap sayfası, başlık ve türlerin altında):**
  - kitaplıkta yoksa "+ Okumak istiyorum" ve ok;
  - varsa "✓ <durum>" ve ok;
  - menüde dört durum ve "Kitaplığımdan çıkar".
  - Çıkarmadan önce onay diyaloğu: "Kitaplığından çıkarılsın mı? Puanın ve yorumların silinmez."
  - Giriş yoksa "Kitaplığa eklemek için giriş yap" bağlantısı; girişten sonra aynı kitaba dönülür
    (`state.from`).
  - İstek sürerken düğme pasif (çift gönderim koruması).
- **Kart rozeti:**
  - `BookCard`'a isteğe bağlı `status`; kapağın köşesinde metinli küçük etiket (yalnız renk değil).
  - `useLibraryStatuses(ids)` kancası giriş yapılmışsa tek toplu istek atar. Kullanan yerler:
    katalog, "en sevilenler" şeridi, benzer kitaplar.
  - Kitap sayfasındaki düğme de aynı uçtan tek kimlikle okur.
- **`Library.tsx` (`/library`, giriş ister; üst menüde ve mobil menüde "Kitaplığım"):**
  - Durum sekmeleri (Tümü + dört durum), arama, tür çipleri, "Puanım" (hepsi / puanladıklarım /
    puanlamadıklarım), sıralama, "N kitap" sayısı, sayfalama.
  - Hepsi adreste; filtre değişince ilk sayfa; bayat yanıt koruması.
  - Boş kitaplıkta "Kataloğa göz at", boş sonuçta "Filtreleri temizle".
  - Kartlarda durum ve varsa "Puanım: 8"; `in_catalog: false` olan kart soluk, bağlantısız,
    "Katalogdan kaldırıldı" yazar, yanında "Çıkar".

## ⚠️ Önemli noktalar

- **Sahiplik yapısal:** hiçbir uç başka bir kullanıcının kimliğini almaz; testler iki kullanıcıyla
  bunu kanıtlar.
- **Türkçe sıralama:** `tr-x-icu` collation'ının yerel Postgres 16'da, CI'da ve Supabase'de olduğu
  plan aşamasında doğrulanır (Supabase sorgusunu kullanıcı çalıştırır:
  `SELECT collname FROM pg_collation WHERE collname = 'tr-x-icu'`). Yoksa plan, sıralama kararını
  yeniden açar; sessizce ikili (byte) sıraya düşülmez.
- **Faz 16 ile etkileşim:** liste sorgusu `sqlc.embed(b)` ile `books.cover_url` ve
  `books.page_count`'u da okuyor; Faz 16'nın genişletme PR'ı bu sorguyu da varsayılan baskıya geçirecek ("eski kolonlar
  yokmuş gibi" testi onu da kapsar).
- **Migration numarası:** `0010`; PR açılırken `origin/main` ve `origin/release`'te boş olduğu
  kontrol edilir (`scripts/checks.sh` da denetler).
- **Loglar:** istek gövdesi loglanmaz; `requestLogger` yolu sorgusuz yazar (arama metni loglanmaz).

## 🧪 Doğrulama (başarı ölçütleri)

1. **Go, veritabanıyla:**
   - B kullanıcısı A'nın kayıtlarını listede ve toplu durumda görmez, değiştiremez.
   - Yayında olmayan kitap → 404. İki aynı PUT → tek kayıt. DELETE kayıt yoksa da 204.
   - Arşivlenen kitap listede `in_catalog: false`; durumu değiştirilemez, çıkarılabilir.
   - Bütün filtreler ve üç sıralama; başlık sırası Türkçe örneklerle (Cesur < Çalıkuşu < Dune;
     Ilık < İnce; Sapiens < Şato).
2. **Go, handler:** bilinmeyen alan, geçersiz durum / `rated` / `sort`, 100'den fazla kimlik, girişsiz
   istek (401); sözleşme testi.
3. **Frontend (vitest):**
   - `LibraryButton`'un üç hâli (yok, var, giriş yok) ve çıkarma onayı.
   - Rozetin metni.
   - Kitaplığım'da filtrelerin adresle eşleşmesi, geri/ileri gezinmede korunması, boş durumlar.
4. **e2e:**
   - Giriş → kitabı ekle → durumu değiştir → katalogda rozet → Kitaplığım'da süz → çıkar.
   - 360 px genişlikte yatay taşma yok (UX-02, F13).
5. **Canlı:**
   - Yayından sonra herkese açık sayfalar ve uçlar aynı; girişsiz `GET /api/me/library` → 401.
   - Kitap ekleme ve Kitaplığım akışını kullanıcı canlıda dener (veri yazan adım).

## 📏 Uygulama notları (2026-10-18)

- **`my_rating`:** `r.score::bigint` dönüşümü LEFT JOIN'in "boş olabilir" bilgisini kaybettiriyordu (sqlc
  `int64` üretti; puansız satır okunurken patlardı). Dönüşüm kalktı; `sqlc.yaml`'a boş olabilen
  `smallint` → `*int16` kuralı eklendi.
- **Kitaplıktaki kitapta düğme:** bölünmüş değil tek bir menü düğmesi ("Kitaplığında: Okudum"); ana
  kısmın kitap eklendikten sonra yapacağı anlamlı bir iş yok.
- **Kitaplığım araması:** Enter / "Ara" ile. Katalogdaki yazarken-arama ince yarış korumaları istiyor;
  küçük bir kişisel liste için gerekmedi.
- **e2e'nin bulduğu, faz öncesinden kalan hata:** `useEffect(() => window.scrollTo(0, 0), [])`.
  Chromium 156'da `scrollTo` Promise döndürüyor; React onu temizlik fonksiyonu sanıp kitap sayfasından
  ayrılırken çağırdı ve uygulama boş ekranla çöktü. Etki artık hiçbir şey döndürmüyor; regresyon testi var.
- **360 px:** "Puanım" düğme grubu 1 px taşıyordu; gruplar kendi içinde kaydırılıyor.

Son incelemenin (taze bir inceleyici, bütün dal) bulduğu ve kapatılanlar:

- **İstek sürerken menü gerçekten açılıyordu.** Düğmenin `disabled`'ı yetmiyor: Radix menüyü
  `pointerdown` ile açıyor ve yalnız tetikleyicinin kendi `disabled` prop'una bakıyor; Chromium pasif
  düğmede de `pointerdown` tetikliyor. İki PUT yarışıp ekranda son seçim olmayan durum kalabiliyordu.
  Tetikleyiciler artık `disabled` alıyor; test menüyü gerçekten açmayı deniyor.
- **Etkiler kullanıcı kimliğine bağlı** (`user?.id`): her sessiz token yenilemesi yeni bir kullanıcı
  nesnesi getiriyor; nesneye bağlı etki durumu yeniden okuyup yeniden denenen PUT'la yarışıyordu.
- **Boşalmış sayfa:** son kitap kendi sayfasından çıkarılıp "geri" ile dönülünce Kitaplığım boş bir
  sayfada "Kitaplığın boş" diyordu. Artık son gerçek sayfaya gidiliyor; boş kitaplık ekranı toplama bakıyor.
- **Paket boyutu:** Radix menüsü ve diyaloğu ana pakete girmişti (Faz 14 Moderasyon'u tam bunu önlemek
  için tembel yüklemişti). Ana JS gzip: Faz 15 öncesi 107,9 kB → tembel yüklemeden 139,4 kB →
  **108,8 kB**. Giriş yapmış okura ait menü `LibraryMenu` (18,9 kB gzip) ve Kitaplığım sayfası (2,9 kB)
  ayrı parçalar; Radix diyaloğu paylaşılan `dialog` parçasında (12,6 kB).
- **Yazma sahipliği testi:** bob'un PUT/DELETE'i alice'in kaydına dokunmuyor; silme sorgusu `user_id`'yi
  yok sayacak şekilde bozulunca test kırmızı yandı (mutasyonla doğrulandı).

PR #43'teki code-review tavsiyeleri ve SonarCloud kalite kapısı (tekrarlanan satır %3,6 > %3) sonrası:

- **Durum okunamazsa ekleme yok:** okuma başarısızken "Okumak istiyorum" gösterilseydi tek tıklama kayıtlı
  bir durumun üstüne yazardı (PUT, `ON CONFLICT` ile değiştirir). Artık "Kitaplık durumu yüklenemedi, tekrar
  dene" düğmesi çıkıyor.
- **Eski parça:** yayın eski `assets/*` dosyalarını silince, önceden açık sekmede tembel parça yüklenemiyor
  ve uygulama boşalıyordu. `vite:preloadError` olayında sayfa bir kez yeniden yükleniyor (10 sn'de en çok
  bir kez; `lib/chunkReload.ts`). Vite 8 olayı parçanın kendisi düştüğünde de yayıyor (paket incelendi).
- **Menü + diyalog iptali:** "Vazgeç"ten sonra sayfanın tıklanabilir kaldığı e2e'de doğrulandı; kurulu
  Radix sürümünde kilitlenme yok.
- **Tekrar azaltma:** `createUser`/`createBook` test yardımcıları `testdb`'ye taşındı (library, ratings,
  comments ortak kullanıyor); sayma sorgusu kalktı; `Chip`, `GRID` ve adres güncelleyici
  `components/filters.tsx`'te.

Takip push'unun (`232c611`) incelemesi engelleyici bulmadı; 10 tavsiyeden birleşmeden önce kapatılanlar
(2026-10-24):

- **Elle değiştirilmiş adres (N1):** Kitaplığım yalnız `status`/`rated`/`sort`'u süzüyordu; `?genre=Bilim Kurgu`,
  100 karakterden uzun `q` ya da `offset` > 10 000 API'den 400 alıp hata kutusu gösteriyordu. Katalog da aynı
  durumdaydı. Süzgeçler sunucunun sınırlarıyla `filters.tsx`'te (`asGenre`, `asQuery`, `asOffset`) ve iki
  sayfa da onları kullanıyor.
- **Bayat rozet (A4):** yeni kimliklerin okuması başarısız olunca önceki rozetler kalıyordu; kullanıcı
  değişince de önceki kullanıcının durumları yeni cevap gelene kadar görünüyordu. Durumlar artık sahibiyle
  saklanıyor, başarısız okuma onları temizliyor. Kimlik değişiminde sıfırlanmıyor: katalogda anahtar iki kez
  değiştiği (N2) için rozetler yanıp sönerdi.
- **Yorumlar (R1, R4):** `LibraryMenu`'deki tetikleyici yorumu kendi JSX'inin üstüne taşındı; `main.tsx`'teki
  "preventDefault hatayı yutar" düzeltildi (Vite import'u `undefined` ile çözüyor, ekran bir an boş kalıyor).

Ertelenenler (davranış doğru, maliyet küçük): katalogda iki durum isteği (N2) ve kitap sayfasında iki istek
(N9); `handleLibraryStatuses`'taki gereksiz harita kopyası (A8); `optional`'ın `books`'takiyle aynı olması (N5);
`List`'in `Limit` 0'da toplamı 0 vermesi, canlıda `parsePage` yüzünden olamaz (R2); site verisi engelliyken
`sessionStorage` okumasının yakalanmamış hata atması, kullanıcı sonucu aynı (R3).

## ✅ Tamamlanma

- Faz 15 issue'su açılmış (Ready'den önce kapsam ve kabul kriterleri), PR'ı `Refs` ile bağlı.
- PR canlıda; Doğrulama 1–5 yeşil ve PR'da yazılı.
- FEATURE_REQUESTS'te LIB-01, LIB-02 ve SOC-02'nin gizlilik yarısı işaretli; ROADMAP'te "Şimdi"
  Faz 16'ya geçmiş; STATUS ve PROJECT_MAP güncel; faz listesinde bu dosyaya bağlantı var.
