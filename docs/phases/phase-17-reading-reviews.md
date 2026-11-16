# Faz 17 — Okuma takibi ve inceleme

> Durum: 📝 spec onaylandı (2026-11-16), uygulama başlamadı (ürün kararları ve 17A'nın "tek durum
> makinesi" modeli aynı gün, § Kararlar) —
> [ROADMAP](../ROADMAP.md) · Backlog: [FEATURE_REQUESTS](../FEATURE_REQUESTS.md) Paket B — READ-01,
> READ-02, REV-01, REV-02 · Issue: [#80](https://github.com/mtgezgin/BookHub/issues/80) · İlgili: [phase-15](phase-15-personal-library.md)
> (kitaplık), [phase-16](phase-16-works-editions.md) (baskılar), [phase-05](phase-05-comments-ratings.md)
> (yorum ve puan), [phase-04](phase-04-suggestions-moderation.md) (durum makinesi)

## 🎯 Amaç

Kullanıcı bir kitabı okumaya başlar, nerede kaldığını kaydeder, bitirir ya da yarım bırakır ve
geçmişini görür (**17A**). Sonra kitap hakkında tek, düzenlenebilir, gerekirse spoiler işaretli bir
inceleme yazar (**17B**). Tek spec; 17A iki PR (özellik + kısıt), 17B bir PR. Her PR ayrı yayınlanıp
doğrulanır.

## 🔍 Başlangıç durumu (neden)

- Kitaplık (Faz 15) yalnız **durumu** tutuyor (`library_entries.status`): "Şu an okuyorum" diyen
  kullanıcının nerede kaldığı, ne zaman başlayıp bitirdiği hiçbir yerde yok.
- Arayüz bir **durum seçici**: kullanıcı dört durumdan birini seçer (`PUT {status}`). Okuma takibi
  gelince bu yetmez: "Okudum" hem "şimdi bitirdim" hem "yıllar önce okumuştum" demek olabilir ve
  ikisinin tarihi farklıdır.
- Sayfa toplamı artık baskıda (Faz 16, `editions.page_count`); okuma kaydının buna bağlanması o fazda
  bu faza bırakıldı.
- Kitapta yalnız kısa yorumlar var (`comments`, 2000 karakter, yalnız admin siler, düzenlenemez).
  Kullanıcı puanının nedenini anlatamıyor ve yazdığını düzeltemiyor.
- Projede **eşzamanlı yazım koruması hiç yok**: iki sekmede açık bir kayıtta sonra yazan, öncekini
  sessizce ezer. Tek değerli durum ve puan için kabul edilebilirdi; ilerleme ve uzun metin için değil
  (READ-01 kabul kriteri).

## 🧠 Yeni kavram(lar)

1. **Tek doğruluk kaynağı; geçersiz durumu yazılamaz kılmak.** İlk taslak aktif okumayı ayrı bir
   tabloda tutuyordu: "bu kitabı okuyor mu" bilgisi iki yerde (kitaplık durumu + aktif kayıt) olunca
   ikisini eşitlemek gerekiyor, eşitlenemeyen her köşe de bir hata koduna (409) ya da tutarsız veriye
   dönüşüyordu. Çözüm eşitlemeyi iyileştirmek değil, ikinci kopyayı kaldırmak: aktif okumanın alanları
   kitaplık satırının **içinde** durur. "Aktif okuma var ama durum başka" artık yazılabilir bir şey
   değildir; "en fazla bir aktif okuma" da birincil anahtardan bedavaya gelir.
2. **Durum makinesi ve niyet komutları (Faz 4'ün deseni, bu kez kullanıcı tarafında).** İstemci
   "durumu X yap" demez; niyetini söyler: başla, bitir, yarım bırak, ertele. Her komut her durumda
   tanımlıdır (ya geçiş yapar ya da zararsızca hiçbir şey yapmaz), bu yüzden kullanıcı bir niyeti için
   hata görmez. Arayüz de durum listesi değil, bulunduğun durumdan yapılabilecekleri gösterir.
3. **Durum ile günlük (state / log) ayrımı.** `library_entries` şimdiki durumu tutar; `reading_log`
   kapanmış okumaların kaydıdır. Durum günlükten **türetilmez**: günlük silinse de durum doğrudur.
4. **İyimser eşzamanlılık (optimistic concurrency) ve kayıp güncelleme.** İki ekran aynı ilerlemeyi
   okur, ikisi de yazar; ikinci yazım birincinin yaptığını haberi olmadan siler (*lost update*). Kilit
   yerine satırda bir `version` sayacı: istemci okuduğu sürümü geri gönderir, yazım
   `WHERE … AND version = $n` ile yapılır. Satır güncellenmediyse arada başkası yazmıştır: **409**,
   istemci güncel hâli gösterip kullanıcıya sorar. Bu fazdaki tek 409 budur.
5. **Kısıt da bir daraltmadır.** Bir CHECK eklemek, o an canlıda çalışan kodun yazabildiği bir şeyi
   yasaklayabilir. Bu yüzden kolonlar önce kısıtsız eklenir (genişlet), yeni kod yayınlanır, kısıt
   ayrı bir migration'la sonra gelir (daralt). Faz 16'da kolon silmek için yaptığımızın aynısı.
6. **`date` ile `timestamptz` farkı.** Başlama ve bitiş bir **takvim günü**dür ("12 Kasım'da
   bitirdim"), bir an değil. `timestamptz` saat dilimi dönüşümünde günü kaydırabilir (İstanbul'da gece
   01:00, UTC'de bir önceki gün). Bu yüzden `date`; "bugün"ü sunucu değil istemci söyler.
7. **Anlık görüntü (*snapshot*).** Okuma, başladığı andaki sayfa toplamını kendinde saklar
   (`total_pages`). Admin baskının sayfa sayısını sonradan düzeltirse ilerleme sessizce başka bir orana
   dönüşmez (CAT-02 kabul kriteri).
8. **Erişilebilir gizli içerik (17B).** Spoiler metni kullanıcı açana kadar DOM'da **hiç yoktur**
   (CSS ile gizlemek ekran okuyucuya ve sayfa içi aramaya metni sızdırır). Açma kontrolü gerçek bir
   `<button aria-expanded>`'dır.

## 🎨 Kararlar

| Konu | Karar | Neden |
| --- | --- | --- |
| Bölme | 17A okuma (READ-01, READ-02): özellik PR'ı + kısıt PR'ı · 17B inceleme (REV-01, REV-02) | Kullanıcı kararı (2026-11-16); kısıt ayrı çünkü daraltma (kavram 5). |
| Model | Aktif okuma `library_entries` satırında; kapanmış okumalar `reading_log`'da | Kullanıcı kararı (2026-11-16), kavram 1. İlk taslaktaki ayrı `reading_sessions` + eşitleme bırakıldı. |
| Yazma yolu | Tek servis, niyet komutları; her komut her durumda tanımlı, niyet için 409 yok | Kavram 2. |
| "Bitirdim" / "Zaten okudum" | İki ayrı komut: `finish` (okurken; tarih = istemcinin bugünü, değiştirilebilir) ve `mark-read` (geçmişte okunmuş; tarih boş, sonradan eklenebilir) | Tek "Okudum" seçeneği iki farklı niyeti karıştırıyordu; "bilinmeyen tarih uydurulmaz" (READ-02). |
| Başlama | Tek tık: varsayılan baskı, bugünün tarihi; birim sayfa (toplam biliniyorsa) yoksa yüzde. Hepsi sonradan düzeltilebilir | Diyalog yerine makul varsayılan + sonradan düzeltme. |
| Yarım bırakılanı sürdürme | Kullanıcı seçer: **Devam et** (son yarım kalan kayıt geri açılır, sayfa ve başlangıç tarihi korunur) ya da **Baştan başla** (yeni okuma; eski kayıt geçmişte kalır) | Kullanıcı kararı (2026-11-16). |
| Erteleme / çıkarma (okurken) | İlerleme > 0 ise geçmişe "yarım bırakıldı" yazılır; ilerleme yoksa hiçbir şey yazılmaz. Arayüz tek cümleyle onay ister | Veri kaybolmaz; yanlışlıkla başlatılan okuma geçmişi kirletmez. |
| İlerleme birimi | Tek birim: `page` ya da `percent`; dönüşüm yok. Birim ya da baskı değişirse ilerleme sıfırlanır (arayüz uyarır) | READ-01 "tahmin yapılmaz"; CAT-02 "baskı değişimi ilerlemeyi dönüştürmez". |
| Faz 15'ten kalan "Şu an okuyorum" | Geçerli durum: alanlar boş = "ilerleme henüz girilmedi" | Veri taşıma gerekmez. |
| Eşzamanlılık | `version`; yalnız ilerleme/alan düzeltmesinde zorunlu, uyuşmazsa 409 `version_conflict` | Kavram 4. Komutlar (başla, bitir…) hedef duruma idempotent, sürüm istemez. |
| Kısıt zamanı | Kolonlar 0014'te kısıtsız; CHECK'ler 0015'te, 17A canlıda doğrulandıktan sonra | Kullanıcı kararı (2026-11-16): yayın penceresinde eski kod hata vermesin (kavram 5). |
| Eski `PUT /api/me/library/{id}` | Kalır, aynı servise komut olarak çevrilir | Yayın penceresindeki eski arayüz ve tek yazma yolu. |
| Geçmiş ve kitaplık | Kitaplıktan çıkarmak geçmişi silmez | Faz 15'teki "çıkarmak puanı silmez" ile tutarlı. |
| İnceleme ↔ yorum | Ayrı `reviews` tablosu; yorumlar ve silme kuralları aynen kalır | Kullanıcı kararı (2026-11-16); REV-01 "brief ile ilişki". |
| İnceleme moderasyonu | Hemen yayınlanır; admin gerekçeyle gizler / geri açar. Şikâyet kuyruğu (MOD-04) Faz 18'de | Kullanıcı kararı (2026-11-16). |
| İnceleme metni | 1–5000 karakter; düzenlenince `edited_at` dolar ve "düzenlendi" görünür | REV-01. |
| Puan | İncelemede ayrı puan yok; `ratings`'ten okunur. İnceleme silmek puanı silmez ve arayüz bunu söyler | REV-01. |
| Spoiler | Kayıt başına bayrak (`has_spoiler`); API metni döner, gizleme arayüzde ve metin DOM'a girmez | REV-02 ilk sürüm. |
| Hesap silinince | Okuma geçmişi ve incelemeler `ON DELETE CASCADE` | Kitaplık ve puanla aynı: kullanıcının kendi verisi. |

## 🚫 Kapsam dışı

- Şikâyet ve moderasyon kuyruğu (MOD-04) — Faz 18.
- Kitaplığın herkese açılması, profil sayfası, okuma istatistikleri (STATS-01), yıllık hedef.
- Sayfa ↔ yüzde dönüşümü; okuma notları / alıntılar; ilerleme günlüğü (her güncellemenin ayrı
  satırı) — yalnız son değer tutulur.
- İncelemeye beğeni, yanıt; inceleme listesinde sıralama seçenekleri (en yeni önce, sabit).
- Mevcut yorumlarda değişiklik (düzenleme, sahibinin silmesi, spoiler).
- Admin için gizlenmiş incelemeler listesi ekranı (geri açma ucu var, ekranı yok).

## 🗺️ Bölümler

- **17A-1 — okuma takibi:** migration `0014` (kolonlar + `reading_log`, kısıtsız), `internal/library`
  servisi, raf uçları, kitap sayfasında tek raf kontrolü, Kitaplığım'da ilerleme.
- **17A-2 — kısıt:** migration `0015` (temizlik + CHECK'ler). 17A-1 canlıda doğrulandıktan sonra.
- **17B — inceleme:** migration `0016_reviews`, `internal/reviews`, inceleme uçları ve admin gizleme,
  kitap sayfasında İncelemeler bölümü, spoiler gösterimi, bitirince inceleme daveti.

## 🪜 17A — adımlar

### 1. Şema: `0014_reading.sql` (genişletme)

```sql
ALTER TABLE library_entries
    ADD COLUMN edition_id  bigint REFERENCES editions (id) ON DELETE SET NULL,
    ADD COLUMN unit        text,       -- 'page' | 'percent'; NULL = ilerleme henüz girilmedi
    ADD COLUMN progress    integer,
    ADD COLUMN total_pages integer,    -- başlarken baskıdan kopyalanır; NULL = bilinmiyor
    ADD COLUMN started_on  date,       -- NULL = bilinmiyor
    ADD COLUMN version     integer NOT NULL DEFAULT 1;

CREATE TABLE reading_log (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id     bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    book_id     bigint NOT NULL REFERENCES books (id) ON DELETE CASCADE,
    edition_id  bigint REFERENCES editions (id) ON DELETE SET NULL,
    outcome     text NOT NULL CHECK (outcome IN ('FINISHED', 'ABANDONED')),
    unit        text CHECK (unit IN ('page', 'percent')),
    progress    integer CHECK (progress >= 0),
    total_pages integer CHECK (total_pages > 0),
    started_on  date,
    ended_on    date,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (ended_on IS NULL OR started_on IS NULL OR ended_on >= started_on)
);
CREATE INDEX reading_log_user_book_idx ON reading_log (user_id, book_id, created_at DESC);
```

- `library_entries`'e eklenen her kolon boş olabilir ya da varsayılanlıdır: canlıdaki (eski) kod
  bunları hiç anmadan çalışmaya devam eder. `reading_log` yeni tablo olduğu için kısıtları baştan taşır.
- Eski kod yayın penceresinde okuyan bir satırın durumunu değiştirirse okuma alanları satırda **bayat**
  kalabilir. Yeni kod bu alanları yalnız `status = 'READING'` iken okur ve her "başla"da üzerine yazar;
  bayat alanlar 0015'te temizlenir.

### 2. Şema: `0015_reading_checks.sql` (daraltma, ayrı PR)

```sql
UPDATE library_entries
SET edition_id = NULL, unit = NULL, progress = NULL, total_pages = NULL, started_on = NULL
WHERE status <> 'READING' AND (unit IS NOT NULL OR progress IS NOT NULL OR started_on IS NOT NULL
                               OR total_pages IS NOT NULL OR edition_id IS NOT NULL);

ALTER TABLE library_entries
    ADD CONSTRAINT library_reading_only_when_reading CHECK (
        status = 'READING' OR (edition_id IS NULL AND unit IS NULL AND progress IS NULL
                               AND total_pages IS NULL AND started_on IS NULL)),
    ADD CONSTRAINT library_progress_valid CHECK (
        (unit IS NULL AND progress IS NULL AND total_pages IS NULL)
        OR (unit = 'percent' AND progress BETWEEN 0 AND 100 AND total_pages IS NULL)
        OR (unit = 'page' AND progress >= 0 AND (total_pages IS NULL OR progress <= total_pages)));
```

- Önce canlıda sayım: temizlenecek satır var mı, kural dışı ilerleme var mı (PR'da yazılır).

### 3. Servis: `internal/library` (durum makinesi)

Her komut tek transaction; satır `FOR UPDATE` ile kilitlenir, `version` her yazımda artar.

| Komut | Hangi durumdan | Sonuç | Günlük |
| --- | --- | --- | --- |
| `want` | yok | WANT_TO_READ | — |
| `start` | yok, WANT_TO_READ, READ | READING; varsayılan baskı, `started_on`, birim | — |
| `start {mode: resume}` | ABANDONED | READING; son ABANDONED günlük satırı geri alınır (alanlar satıra, günlük satırı silinir) | −1 |
| `start {mode: restart}` | ABANDONED | READING; yeni okuma | — |
| `finish {ended_on?}` | READING | READ | FINISHED |
| `abandon {ended_on?}` | READING | ABANDONED | ABANDONED |
| `postpone` | READING | WANT_TO_READ | ilerleme > 0 ise ABANDONED |
| `mark-read` | yok, WANT_TO_READ, ABANDONED | READ | FINISHED, tarihsiz |
| `remove` | hepsi | satır silinir | READING ve ilerleme > 0 ise ABANDONED |

- Tabloda olmayan (komut, durum) çiftleri hedef durumdaysa **hiçbir şey yapmaz** (çift tık, yeniden
  deneme), değilse en yakın tanımlı komuta çevrilir: READING'de `mark-read` = tarihsiz `finish`,
  READING'de `want` = `postpone`. Hiçbiri hata değildir.
- `resume`'da geri alınacak kayıt yoksa (Faz 15'ten kalan ABANDONED) `restart` gibi davranır.
- Kitap yayında değilse ekleyen/başlatan komutlar 404 (Faz 15 kuralı); `remove` her zaman çalışır.
- Eski `PUT {status}`: WANT_TO_READ → `want`, READING → `start` (ABANDONED'dan `resume`), READ →
  `mark-read`, ABANDONED → `abandon` (okumuyorsa yalnız durum).

### 4. API (`requireAuth` grubunda, adreste kullanıcı yok)

```text
GET    /api/me/books/{id}/shelf             durum + okuma alanları + version + geçmiş
POST   /api/me/books/{id}/shelf/want
POST   /api/me/books/{id}/shelf/start       {mode?, edition_id?, unit?, started_on?}
POST   /api/me/books/{id}/shelf/finish      {ended_on?}
POST   /api/me/books/{id}/shelf/abandon     {ended_on?}
POST   /api/me/books/{id}/shelf/postpone
POST   /api/me/books/{id}/shelf/mark-read
PATCH  /api/me/books/{id}/shelf/reading     {version, progress?, started_on?, unit?, edition_id?}
DELETE /api/me/library/{id}                 çıkar (mevcut uç, artık servisten)
PATCH  /api/me/reading-log/{id}             {started_on?, ended_on?}
DELETE /api/me/reading-log/{id}
```

- Her komut güncel rafı döner (istemci ayrıca GET yapmaz).
- Gövdeler ayrı DTO + `DisallowUnknownFields`; başkasının günlük kaydı 404.
- Hatalar: `version_conflict` (409, gövdede güncel raf; yalnız PATCH reading), `invalid_progress`
  (400), `invalid_dates` (400).
- `GET /api/me/library` kartlar için `unit`, `progress`, `total_pages` de döner (aynı satır, ek JOIN yok).

### 5. Arayüz: tek raf kontrolü

Kitap sayfasındaki durum menüsü ve (ilk taslaktaki) ayrı okuma paneli birleşir. Kontrol bulunduğun
durumu ve oradan yapılabilecekleri gösterir:

| Durum | Görünen | Ana eylem | Menü |
| --- | --- | --- | --- |
| Kitaplıkta değil | — | Okumak istiyorum | Okumaya başla · Zaten okudum |
| Okumak istiyorum | rozet | Okumaya başla | Zaten okudum · Kitaplıktan çıkar |
| Okuyorum | ilerleme çubuğu (`42 / 310 sayfa`, `%35`, toplam yoksa `42. sayfa`; girilmemişse "İlerlemeni gir") | İlerlemeyi güncelle · Bitirdim | Yarım bıraktım · Ertele · Baskı/birim değiştir · Kitaplıktan çıkar |
| Okudum | "12 Kasım'da bitirdin" ya da "Okudun · tarih ekle" | Yeniden oku | Kitaplıktan çıkar |
| Yarım bıraktım | "Sayfa 42'de bıraktın" | Devam et | Baştan başla · Zaten okudum · Kitaplıktan çıkar |

- **Onay yalnız yan etkili geçişlerde:** ilerleme varken "Ertele" ve "Kitaplıktan çıkar" ("Sayfa
  42'deki okuman geçmişe kaydedilecek"); birim/baskı değişiminde "İlerlemen sıfırlanacak".
- **Bitirince:** tarih satırı ("Bugün bitirdin · tarihi değiştir") ve isteğe bağlı davet: "Puan ver"
  (17B'de "İnceleme yaz" eklenir).
- **409 `version_conflict`:** "İlerlemen başka bir yerde güncellenmiş: sayfa 57" + kullanıcının
  girdiği değer; yeniden gönder ya da vazgeç. Sessiz yeniden deneme yok.
- **Geçmiş:** kontrolün altında kapanmış okumalar (tarih, baskı, sonuç; tarihi düzelt / sil).
- **Kitaplığım:** "Şu an okuyorum" kartlarında ilerleme; karttaki menü aynı komutları kullanır.
- Kart rozetleri (`/api/me/library/statuses`) değişmez.

## 🪜 17B — adımlar

### 1. Şema: `0016_reviews.sql`

```sql
CREATE TABLE reviews (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id       bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    book_id       bigint NOT NULL REFERENCES books (id) ON DELETE CASCADE,
    body          text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 5000),
    has_spoiler   boolean NOT NULL DEFAULT false,
    version       integer NOT NULL DEFAULT 1,
    created_at    timestamptz NOT NULL DEFAULT now(),
    edited_at     timestamptz,                        -- NULL = hiç düzenlenmedi
    hidden_at     timestamptz,                        -- NULL = görünür
    hidden_by     bigint REFERENCES users (id) ON DELETE SET NULL,
    hidden_reason text NOT NULL DEFAULT '',
    UNIQUE (user_id, book_id),
    CHECK (hidden_at IS NULL OR hidden_reason <> '')
);
CREATE INDEX reviews_book_created_idx ON reviews (book_id, created_at DESC) WHERE hidden_at IS NULL;
```

### 2. API

```text
GET    /api/books/{id}/reviews                herkese açık, sayfalı; gizlenenler yok; yazarın puanı ratings'ten
GET    /api/books/{id}/review                 benim incelemem (gizlenmişse gerekçesiyle)
PUT    /api/books/{id}/review                 yaz / düzenle  {body, has_spoiler, version?}
DELETE /api/books/{id}/review                 sil (puan kalır)
POST   /api/admin/reviews/{id}/hide           {reason}   — audit ile aynı transaction
POST   /api/admin/reviews/{id}/unhide                     — audit ile aynı transaction
```

- PUT: kayıt yoksa oluşturur; varsa `version` zorunlu (kavram 4). Gizlenmiş inceleme düzenlenince
  gizli kalır (REV-01 kabul kriteri).
- PUT'a rate limit (yeni anahtar `review`). Kitap yayında değilse yazım 404.

### 3. Arayüz

- Kitap sayfasında yorumların üstünde **İncelemeler**: yazar, puanı (varsa), tarih, "düzenlendi".
- Kendi incelemem: düzenleyici (karakter sayacı, spoiler onay kutusu), sil onayında "Puanın silinmez".
- Gizlenmiş incelemem: "Bu inceleme yönetici tarafından gizlendi: <gerekçe>".
- Spoiler: kartta "Spoiler içeriyor" + `Spoiler'ı göster` düğmesi (`aria-expanded`); metin açılınca
  DOM'a girer (kavram 8).
- Admin: her incelemede "Gizle" (gerekçe zorunlu diyalog).

## ⚠️ Önemli noktalar

- **Yayın penceresi (17A-1):** migration'dan sonra eski kod birkaç dakika çalışır. Eski kod yeni
  kolonları bilmez; kısıt henüz olmadığı için hiçbir yazımı reddedilmez. Bedeli: o pencerede eski
  arayüzden durumu değiştirilen bir okumanın ilerlemesi günlüğe yazılmaz (bayat alanlar 0015'te silinir).
- **0015 ancak 17A-1 doğrulandıktan sonra** merge edilir (CLAUDE.md: daraltma ayrı PR).
- **404 ile 409'u karıştırmamak:** PATCH reading'de güncellenmeyen satır "kitaplıkta yok / okumuyor"
  (404) da olabilir, "sürüm eski" (409) de. Servis satırı kilitleyip ayırır.
- **`resume` bir satırı günlükten siler:** aynı transaction'da; yarıda kalırsa ikisi de geri alınır.
- **Sonar tekrar kapısı:** komut handler'ları birbirine benzer; tek bir `handleShelfCommand(cmd)`
  üreticisi (Faz 4'teki `handleTransition` gibi) ve ortak test kurulumu.
- **Spoiler metni API'de açık.** İlk sürümde gizleme arayüz kararı; bildirim ve önizleme gibi yeni
  yüzeyler gelirse (REV-02 "kart, profil ve bildirim") yeniden değerlendirilir.

## 🧪 Doğrulama (başarı ölçütleri)

17A:
1. Her (durum, komut) çifti için tablo testi: sonuç durumu, okuma alanları ve günlük satırı beklenen
   gibi; hiçbir çift 4xx/5xx dönmez. Aynı komutu iki kez göndermek ikinci günlük satırı yazmaz.
2. Sayfa ilerlemesi negatif olamaz, bilinen toplamı aşamaz; yüzde 0–100. Yenilemede korunur.
3. Eski sürümle PATCH 409 döner ve satırı değiştirmez (iki sekme senaryosu, Playwright).
4. "Bitirdim" bugünün tarihiyle, "Zaten okudum" tarihsiz kaydeder; bitiş başlangıçtan önce olamaz.
5. İlerleme varken ertelemek ve çıkarmak günlüğe "yarım bırakıldı" yazar; ilerleme yokken yazmaz.
   Çıkarılan kitabın geçmişi yeniden eklenince görünür.
6. Yarım bırakılandan "Devam et" sayfayı ve başlangıç tarihini geri getirir, günlük satırı kalmaz;
   "Baştan başla" günlük satırını korur.
7. Transaction ortasında hata olursa ne satır ne günlük değişir (entegrasyon testi).
8. Toplam bilinmeyen okumada "0 sayfa" görünmez; Faz 15'ten kalan okuyorum kaydı "İlerlemeni gir" gösterir.
9. Başkasının günlük kaydına her uç 404; eski `PUT {status}` aynı sonuçları üretir.
10. Canlı (17A-1): demo hesabıyla başla → ilerle → ertele → devam et → bitir; kitap sayfası ve
    Kitaplığım tutarlı.
11. Canlı (17A-2): 0015 öncesi sayım PR'da; sonrasında kural dışı yazım veritabanında reddedilir
    (entegrasyon testi), canlı akış yine çalışır.

17B:
12. Kullanıcı başına kitapta tek inceleme; ikinci PUT düzenler.
13. Başkasının incelemesi değiştirilemez / silinemez (uçlarda kullanıcı kimliği yok).
14. İnceleme silmek puanı silmez.
15. Gizlenen inceleme herkese açık listede yok; sahibi düzenleyince gizli kalır; gizleme ve geri açma
    audit'te.
16. Spoiler metni açılmadan DOM'da yok; düğme klavye ve ekran okuyucuyla çalışır (axe + Playwright).
17. Canlı: inceleme yaz → düzenle → admin gizle → geri aç.

## ✅ Tamamlanma

- 17A-1 canlıda, Doğrulama 1–10 yeşil ve PR'da yazılı; 17A-2 canlıda, Doğrulama 11 yeşil.
- 17B canlıda, Doğrulama 12–17 yeşil.
- STATUS, ROADMAP, PROJECT_MAP, FEATURE_REQUESTS ve bu dosya güncel; issue doğrulamadan sonra elle kapanır.
