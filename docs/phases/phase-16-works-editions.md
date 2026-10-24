# Faz 16 — Eser ve baskı (works / editions)

> Durum: 📝 tasarım incelemede (2026-10-17); uygulama Faz 15'ten (kişisel kitaplık) sonra —
> [ROADMAP](../ROADMAP.md) · Backlog: [CAT-02](../FEATURE_REQUESTS.md) · Issue: [#36](https://github.com/mtgezgin/BookHub/issues/36) ·
> İlgili: [0001_books_genres.sql](../../backend/migrations/0001_books_genres.sql),
> [books.sql](../../backend/sql/queries/books.sql), [BookDetail.tsx](../../frontend/src/pages/BookDetail.tsx)

## 🎯 Amaç

Bir kitabı **eser** (work) ve onun **baskıları** (editions) olarak ayırmak: her baskının kendi dili,
çevirmeni, yayınevi, yılı, ISBN'i, sayfa sayısı ve kapağı olur; puan ve yorumlar eserde kalır. Canlı
veri ve mevcut API bozulmadan, **genişlet → daralt** sırasıyla, iki PR'da.

## 🔍 Başlangıç durumu (neden)

- Tek `books` tablosu iki seviyeyi karıştırıyor: `publication_year` ve `language` eserin (ilk yayın,
  özgün dil — [demo_seed.sql](../../backend/scripts/demo_seed.sql) başlığı); `cover_url` ve
  `page_count` ise çoğu kitapta Türkçe baskının ([0009](../../backend/migrations/0009_demo_book_covers.sql)).
  Örnek: 1984'ün dili `en`, kapağı Türkçe baskının.
- `books.id` her yerde: `/books/:id` adresleri, `/api/books/{id}/*` uçları, `book_genres`, `ratings`,
  `comments` yabancı anahtarları, `audit_logs.entity_id`, en iyi puanlılar önbelleği.
- Çeviri bilgisi tutulamıyor; okur hangi çeviriyi okuyacağını göremiyor.

## 🧠 Yeni kavram(lar)

- **Eser / baskı:** eser soyut içeriktir (Suç ve Ceza, Dostoyevski, 1866, Rusça); baskı elde tutulan
  somut kitaptır (yayınevi, çevirmen, yıl, sayfa, ISBN, kapak). Puan ve yorum esere, kapak ve
  çevirmen baskıya aittir. Open Library de verisini böyle tutar (work → editions).
- **Genişlet → daralt (expand / contract):** çalışan kodu kırmadan şema değiştirmek için iki yayın.
  Önce yeni yapı eklenir ve kod ona geçer; eski yapı, onu anan kod canlıdan **tamamen** çıktıktan
  sonra silinir. Deploy migration'ı yeni koddan önce uyguladığı için, her migration o an çalışan
  (eski) kodla uyumlu olmak zorunda.
- **sqlc'nin `*` açılımı:** sqlc `SELECT *`'ı üretilen kodda kolon kolon yazar
  (`internal/db/books.sql.go`, `getApprovedBook`). "Kolonu kullanmıyorum" demek yetmez: adı üretilen
  sorguda geçiyorsa, kolon silindiği an o sorgu hata verir. Silinecek kolonu anmayan kod, silmeden
  önce canlıda olmalı.
- **Kısmi benzersiz index (partial unique index):** `UNIQUE (book_id) WHERE is_primary` — "kitap
  başına en çok bir varsayılan baskı" kuralını uygulama değil veritabanı garanti eder.

## 🎨 Kararlar

| Konu | Karar | Neden |
| --- | --- | --- |
| Model | `books` = eser; yeni `editions` tablosu | Puan, yorum, tür, URL ve audit `books.id`'ye bağlı; `works` tablosu açıp hepsini taşımak riskli. "Ek baskılar ayrı, varsayılan `books`'ta" seçeneği kapak bilgisini kalıcı olarak iki yerde tutardı. |
| Baskıyı kim ekler | Yalnız admin, API ile; demo verisi migration ile | Bugün admin kitap ekranı yok; kullanıcı önerisi ayrı bir issue. |
| Varsayılan baskı | `is_primary`, kitap başına bir | Ana kapak ve sayfa sayısı ondan gelir. |
| ISBN | Benzersiz değil; tire/boşluk temizlenir, 10 ya da 13 hane; sağlama (checksum) yok | Eski baskılarda çoğu zaman yok; benzersizlik ihlali (23505) bugün `ErrDuplicateSlug`'a eşleniyor. |
| İsteğe bağlı metinler | `translator`, `publisher`, `isbn`: `NOT NULL DEFAULT ''` | Repo kalıbı (`description`, `cover_url`); `''` = bilinmiyor. |
| Herkese açık JSON | Mevcut alanlar aynı; yalnız detay ucuna `editions` | Sözleşme testi ve frontend kırılmaz; liste uçları şişmez. |
| Arayüz | "Diğer baskılar" kapak şeridi + ana kapağın altında varsayılan baskı notu | Kullanıcı seçimi (2026-10-17). |
| Eski kolonlar | PR 1 kodu `books.cover_url` ve `books.page_count` adını hiç anmaz; PR 2 siler | sqlc'nin `*` açılımı (yukarıda). |
| Puan / yorum | Eser düzeyinde, değişmez | Issue kapsamı. |

## 🚫 Kapsam dışı

- Kullanıcının baskı önermesi (moderasyonlu) — ayrı issue.
- Admin baskı ekranı; kitap sayfasında baskı seçip ana kapağı değiştirmek.
- Baskı bazında puan / yorum; Open Library'den toplu içe aktarma.
- ISBN sağlama kontrolü; baskı adıyla arama (arama bugünkü gibi eser adı ve yazarda).

## 🗺️ Bölümler

- **PR 1 — genişlet:** `0010_editions.sql` (tablo + taşıma), `0011_demo_editions.sql` (demo
  baskılar), sorgular, `internal/editions`, admin uçları, detay JSON'u, kapak şeridi, testler.
- **PR 2 — daralt:** sıradaki boş migration numarası (bugün 0012): eksik baskıları tamamla, iki eski
  kolonu sil; `make demo-seed`'den 0009 çıkar. **PR 1 canlıda doğrulanmadan merge edilmez.**

## 🪜 PR 1 — adımlar

### 1. Şema ve taşıma: `0010_editions.sql`

```sql
CREATE TABLE editions (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    book_id          bigint NOT NULL REFERENCES books (id) ON DELETE CASCADE,
    title            text NOT NULL,
    language         text NOT NULL,
    translator       text NOT NULL DEFAULT '',
    publisher        text NOT NULL DEFAULT '',
    publication_year int,
    isbn             text NOT NULL DEFAULT '',
    page_count       int,
    cover_url        text NOT NULL DEFAULT '',
    is_primary       boolean NOT NULL DEFAULT false,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX editions_book_id_idx ON editions (book_id);
CREATE UNIQUE INDEX editions_one_primary_idx ON editions (book_id) WHERE is_primary;

INSERT INTO editions (book_id, title, language, page_count, cover_url, is_primary)
SELECT id, title, language, page_count, cover_url, true FROM books;
```

- Her kitap tam olarak bir varsayılan baskı alır; kapak ve sayfa sayısı birebir kopyalanır.
- Dil `books.language`'den kopyalanır; demo kitaplarda bu özgün dildir ve 0011 düzeltir.
- Down: `DROP TABLE editions;` (0010 psql ile yeniden çalıştırılmaz, Down güvenli).
- Alan doğrulaması `books`'taki gibi uygulamada; tabloya ek CHECK yok.

### 2. Demo baskılar: `0011_demo_editions.sql`

0009'un ilkeleri: eşleşme slug + yazarla, **Down bölümü yok** (`make demo-seed` psql ile yeniden
çalıştırır), tekrar çalıştırılabilir, kitap yoksa hiçbir satır etkilenmez.

1. **Varsayılan baskıların meta verisi (53 kitap):** dil, yayınevi, çevirmen ve baskı yılı. Kaynak:
   0009'daki kapak kimliği → Open Library kapak kaydı → baskı kaydı. Bulunamayan alan olduğu gibi
   kalır. Kapak alanına 0009'daki adresin aynısı yazılır: canlıda 0010 onu zaten kopyaladığı için
   değer değişmez, taze kurulumda boş kapak dolar. **Sayfa sayısına dokunulmaz.** Canlı doğrulama
   kapak ve sayfayı birebir karşılaştırır. Koruma: yalnız kapağı hâlâ `''` ya da 0009'daki adres
   olan varsayılan baskı güncellenir — admin'in sonradan değiştirdiği baskıya dokunulmaz.
2. **Sekiz özgün dil baskısı:** Suç ve Ceza (ru), Dönüşüm (de), Küçük Prens (fr), Yüzyıllık
   Yalnızlık (es), Simyacı (pt), Gülün Adı (it), 1984 (en), Dune (en). Aynı kitapta aynı dil ve adla
   bir baskı varsa eklenmez. Bu kitaplardan birinin varsayılan baskısı zaten özgün dildeyse, o kitap
   başka bir çeviri demo kitapla değiştirilir (her seçili kitapta iki farklı dil olmalı).
3. Sekiz yeni kapak #34'teki gibi gözle doğrulanır: kontrol sayfası hazırlanır, kullanıcı onaylar.
   Varsayılan baskıların meta verisi örneklemeyle kontrol edilir.

Taze kurulum:

- `demo_seed.sql`, kitap INSERT'inden sonra baskısı olmayan her kitaba kitabın adı, dili ve sayfa
  sayısıyla bir varsayılan baskı açar (`INSERT … SELECT … WHERE NOT EXISTS`; kapak `''`, 0011
  doldurur). Migration'lar seed'den önce koştuğu için 0010 boş tabloyu taşımıştır.
- `make demo-seed` sırası: seed → 0009 → 0011. DEPLOYMENT.md'deki ilk kurulum adımı da 0011'i ekler.
- `perf_seed.sql` ve `mvp_smoke.sh` değişmez: baskısız kitabı `LEFT JOIN` taşır; smoke kitapları API
  ile ekler ve servis varsayılan baskıyı açar.

### 3. Sorgular (sqlc)

- Kitap satırı döndüren her sorgu (`books.sql`: `GetApprovedBook`, `GetBook`, `ListBooks`,
  `ListBooksByRating`, `ListBooksByStatus`, `RETURNING` olan yazımlar; `ratings.sql`: `ListTopRated`)
  `*` yerine **açık kolon listesine** geçer; `books.cover_url` ve `books.page_count` listede yoktur.
  Kapak ve sayfa `LEFT JOIN editions pe ON pe.book_id = b.id AND pe.is_primary` ile gelir;
  baskısız kitapta kapak `COALESCE(pe.cover_url, '')`, sayfa `NULL`.
- `CreateBook` ve `UpdateBookDetails` bu iki kolonu yazmaz; değerler varsayılan baskıya gider.
- Yeni `editions.sql`: kitabın baskılarını listele, getir, ekle, güncelle (`COALESCE` yaması), sil,
  varsayılanı temizle / ata, kilitle.
- Üretilen satır tipleri değişir: `toBookResponse` yeni tipleri alır; `rowcopy_test.go` yeni tiplere
  genişler. En iyi puanlılar önbelleğinin anahtarı `books:top-rated:v2` olur (saklanan JSON'un şekli
  değişiyor; eski anahtar 60 sn'de kendiliğinden düşer).

### 4. `internal/editions` paketi (repository + service)

Service var, çünkü gerçek iş kuralları var:

- **Ekle:** kitap satırı kilitlenir; kitabın varsayılan baskısı yoksa yeni baskı varsayılan olur,
  varsa olmaz (varsayılan değiştirmek ayrı uç).
- **Güncelle:** `COALESCE` yaması; `books`'taki gibi bir alan `NULL`'a geri çekilemez (bilinen sınır).
- **Sil:** varsayılan baskı silinemez → `ErrPrimaryEditionDelete`.
- **Varsayılan yap:** tek transaction'da kitap kilitlenir, eski varsayılan temizlenir, yenisi atanır.
  Sıra önemli: önce temizle, sonra ata; tersi kısmi benzersiz index'e takılır.
- **Her yazım** denetim kaydıyla aynı transaction'da: `action` = `edition.create` / `edition.update` /
  `edition.delete` / `edition.set_primary`, `entity_type` = `book`, `entity_id` = kitap kimliği,
  `details` = `{edition_id, …}`. Böylece kitabın geçmişi (`/api/admin/books/{id}/audit`) baskı
  değişikliklerini de gösterir.
- Commit'ten sonra moderasyondaki gibi public-change kancası çağrılır (önbellek temizliği).
- Bu paketteki 23505 hataları `ErrDuplicateSlug`'a eşlenmez.

Mevcut moderasyon servisi:

- **Kitap oluşturma / öneri:** kitap ve varsayılan baskısı aynı transaction'da açılır. İstekteki
  `cover_url`, `page_count` baskıya; `language` hem kitaba hem baskıya yazılır.
- **Kitap PATCH'i:** `cover_url` ve `page_count` varsayılan baskıya yazılır (yoksa açılır);
  `language` yalnız kitabın (eserin) dilidir.

### 5. API

- `GET /api/books/{id}` ve `GET /api/admin/books/{id}`: mevcut yanıt + `editions` dizisi. Sıra:
  varsayılan önce, sonra dil, sonra yıl (boşlar sonda), sonra `id`. Her öğe: `id`, `title`,
  `language`, `translator`, `publisher`, `publication_year`, `isbn`, `page_count`, `cover_url`,
  `is_primary`. Liste uçları değişmez.
- Admin uçları (giriş + admin rolü):

  | Uç | Başarı |
  | --- | --- |
  | `POST /api/admin/books/{id}/editions` | 201 |
  | `PATCH /api/admin/editions/{id}` | 200 |
  | `DELETE /api/admin/editions/{id}` | 204 |
  | `POST /api/admin/editions/{id}/primary` | 200 |

- İstek gövdesi ayrı DTO + `DisallowUnknownFields`. Doğrulama mevcut yardımcılarla: `checkTitle`,
  `checkLanguage` (eklerken zorunlu), `checkPublicationYear`, `checkPageCount`, `checkCoverURL`;
  `translator` ve `publisher` en çok 200 karakter (`maxAuthorLen`); ISBN temizlendikten sonra
  `^[0-9]{9}[0-9X]$` ya da `^[0-9]{13}$`.
- Hatalar mevcut zarfla: `not_found` (404), `validation_error` (400), `primary_edition_delete` (409),
  `invalid_id` (400). Ham hata istemciye gitmez; istek gövdesi loglanmaz.
- Sözleşme testi (`books_contract_test.go`): detay yanıtının ve baskı öğesinin izin listeleri.

### 6. Arayüz

- `api/types.ts`: `Edition`, `BookDetail = Book & { editions: Edition[] }`; `client.ts`'teki
  `getBook` `BookDetail` döndürür.
- `EditionStrip.tsx`: kitabın 2+ baskısı varsa açıklamanın altında, yorumlardan önce "Diğer
  baskılar"; varsayılanı içermez. Her öğe `BookCover` (baskının kapağı ya da üretilmiş kapak) ve
  altında dil (`languageName`), yıl, varsa "çev. …". Yatay kaydırılır; `<ul aria-label="Diğer
  baskılar">`, kapakların alt metninde ad, dil ve yıl.
- Ana kapağın altında, yalnız 2+ baskıda: "Türkçe baskı · İş Bankası" biçiminde tek satır (dil +
  varsa yayınevi). Meta satırı (yazar · ilk yayın yılı · sayfa · özgün dil) değişmez.
- Kitap kartları ve ana sayfa değişmez.

## 🪜 PR 2 — adımlar (daraltma)

1. Migration (sıradaki boş numara): baskısı olmayan kitaplara eski kolonlardan varsayılan baskı açılır
   (PR 1 yayın aralığında eski kodla eklenenler), sonra `books.cover_url` ve `books.page_count`
   silinir. Down: kolonları geri ekler, değerleri varsayılan baskılardan kopyalar.
2. `demo_seed.sql`: kitap INSERT'inden `page_count` çıkar; sayfa sayıları varsayılan baskıyı açan
   INSERT'e taşınır (kolon silinince eski hâli taze kurulumu kırar).
3. sqlc yeniden üretilir; `make demo-seed`'den 0009 çıkar (yazdığı kolon yok; bilgisi 0011'de);
   DEPLOYMENT.md ilk kurulum, PROJECT_MAP ve STATUS güncellenir.
4. 0009 dosyasının kendisi değiştirilmez ve silinmez (canlıya çıkmış migration).

## ⚠️ Önemli noktalar

- **PR 1 kodunda iki eski kolonun adı geçmez.** Kanıtı "eski kolonlar yokmuş gibi" testi (Doğrulama 2).
- **Yayın aralığı:** 0010 ile yeni kodun çıkışı arasındaki dakikalarda eski kodla eklenen kitabın
  baskısı olmaz; yeni kod onu üretilmiş kapakla gösterir, PR 2 eski kolonlardan tamamlar. Arayüzdeki
  öneri formunda kapak alanı yok; pratik etkisi yok.
- **PR 1 geri alınırsa** eski kod, PR 1'den sonra yapılan kapak / sayfa düzenlemelerini görmez
  (kabul edilmiş bedel). PR 2'den sonra geri dönüş yalnız yeni bir migration'la olur.
- **Migration numaraları:** 0010 ve 0011'in `origin/main` ve `origin/release`'te boş olduğu PR
  açılırken kontrol edilir (`scripts/checks.sh` da denetler).
- **1450 kuralı** baskı yılına da uygulanır; eski eserlerin (Devlet) baskıları modern baskıdır.
- Canlı veritabanına doğrudan erişim yok; canlı doğrulama API ve arayüzden yapılır.

## 🧪 Doğrulama (başarı ölçütleri)

1. **Go:** baskı servisi (tek varsayılan, varsayılan silinemez, varsayılan yap tek transaction,
   denetim kaydı, önbellek kancası); handler'lar (bilinmeyen alan, doğrulama, 404, admin dışına
   401/403); sözleşme testi; `rowcopy_test.go`.
2. **"Eski kolonlar yokmuş gibi" testi:** bir transaction'da `books.cover_url` ve `books.page_count`
   silinir, kitap okuyan ve yazan her repository fonksiyonu çalıştırılır, sonra geri alınır. Bir
   sorgu eski kolonu anıyorsa PR 1'de kırmızı.
3. **0010 taşıma testi:** N kitap → N varsayılan baskı; kapak ve sayfa birebir aynı; puan
   istatistikleri değişmez.
4. **`TestDemoEditions`** (`migrations_test.go`): seed ile slug + yazar eşleşmesi iki yönde, URL
   deseni 0009'daki ile aynı, dil deseni, Down bölümü yok, her ek baskı bir demo kitaba bağlı.
5. **Frontend:** `EditionStrip` testi (tek baskıda görünmez; dil adı, çevirmen, üretilmiş kapak).
   **e2e:** Suç ve Ceza sayfasında "Diğer baskılar" ve "Rusça" (CI'daki e2e işi `make demo-seed`
   çalıştırıyor).
6. **Canlı, PR 1:** yayından önce ve sonra API'den 53 kitabın `id → cover_url, page_count,
   rating_count, average_rating` anlık görüntüsü birebir aynı (kabul kriteri 2 ve 3); sekiz kitapta
   şerit gözle kontrol.
7. **Canlı, PR 2:** aynı anlık görüntü yine birebir aynı.

## ✅ Tamamlanma

- PR 1 canlıda; Doğrulama 1–6 yeşil ve PR'da yazılı.
- PR 2 canlıda; Doğrulama 7 yeşil.
- STATUS, PROJECT_MAP ve bu dosya güncel; #36 doğrulamadan sonra elle kapanır.
- Kullanıcının baskı önermesi için ayrı issue açılmış.
