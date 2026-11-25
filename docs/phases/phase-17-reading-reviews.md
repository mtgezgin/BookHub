# Faz 17 — Okuma takibi ve inceleme

> Durum: 📝 spec onaylandı (2026-11-16), uygulama başlamadı (ürün kararları ve 17A'nın "tek durum
> makinesi" modeli aynı gün, § Kararlar). **Koda göre gözden geçirildi (2026-11-23):** Faz 16d'nin
> veri katmanı, 0015'in yabancı anahtar index kuralı, 409 cevabı ve Kitaplığım kartları — § Kararlar'ın
> son dört satırı —
> [ROADMAP](../ROADMAP.md) · Backlog: [FEATURE_REQUESTS](../FEATURE_REQUESTS.md) Paket B — READ-01,
> READ-02, REV-01, REV-02 · Issue: [#80](https://github.com/mtgezgin/BookHub/issues/80) · İlgili: [phase-15](phase-15-personal-library.md)
> (kitaplık), [phase-16](phase-16-works-editions.md) (baskılar), [phase-05](phase-05-comments-ratings.md)
> (yorum ve puan), [phase-04](phase-04-suggestions-moderation.md) (durum makinesi)

> **Migration numaraları (2026-11-17):** bu belgedeki numaralar iki kez kaydı (`0016`–`0018`). `0014` aramaya
> ([Faz 16b](phase-16b-search.md)), `0015` index düzeltmelerine (REVIEW_FEEDBACK F28) verildi; goose sırasız
> migration'ı reddettiği için numara yayın sırasını izler.

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
| Kısıt zamanı | Kolonlar 0016'da kısıtsız; CHECK'ler 0017'de, 17A canlıda doğrulandıktan sonra | Kullanıcı kararı (2026-11-16): yayın penceresinde eski kod hata vermesin (kavram 5). |
| Eski `PUT /api/me/library/{id}` | Kalır, aynı servise komut olarak çevrilir | Yayın penceresindeki eski arayüz ve tek yazma yolu. |
| Geçmiş ve kitaplık | Kitaplıktan çıkarmak geçmişi silmez | Faz 15'teki "çıkarmak puanı silmez" ile tutarlı. |
| İnceleme ↔ yorum | Ayrı `reviews` tablosu; yorumlar ve silme kuralları aynen kalır | Kullanıcı kararı (2026-11-16); REV-01 "brief ile ilişki". |
| İnceleme moderasyonu | Hemen yayınlanır; admin gerekçeyle gizler / geri açar. Şikâyet kuyruğu (MOD-04) Faz 18'de | Kullanıcı kararı (2026-11-16). |
| İnceleme metni | 1–5000 karakter; düzenlenince `edited_at` dolar ve "düzenlendi" görünür | REV-01. |
| Puan | İncelemede ayrı puan yok; `ratings`'ten okunur. İnceleme silmek puanı silmez ve arayüz bunu söyler | REV-01. |
| Spoiler | Kayıt başına bayrak (`has_spoiler`); API metni döner, gizleme arayüzde ve metin DOM'a girmez | REV-02 ilk sürüm. |
| Hesap silinince | Okuma geçmişi ve incelemeler `ON DELETE CASCADE` | Kitaplık ve puanla aynı: kullanıcının kendi verisi. |
| 409 cevabı (2026-11-23) | Gövde her hatadaki zarf: `{"error":{"code":"version_conflict","message"}}`, güncel raf **gövdede yok**. İstemci 409'da rafı bir kez yeniden okur (raf sorgusunu eskitir) ve mesajı o veriden kurar | CLAUDE.md "hata zarfı her yerde"; arayüzün `ApiError`'ı yalnız kod ve mesaj taşıyor. İlk spec gövdede raf öneriyordu; tek hata biçimi nadir bir durumdaki bir istek fazlasından değerli. Asistan kararı, kullanıcı "projeye en uygun" dedi. |
| Kitaplığım kartları (2026-11-23) | 17A-1'de kartlarda yalnız **ilerleme** (`42 / 310 sayfa`, `%35`); komutlar kitap sayfasında. Kart menüsü ayrı küçük iş (§ Kapsam dışı) | Spec "karttaki menü" diyordu ama kartlarda bugün menü yok (yalnız katalogdan kalkmış kitapta "Çıkar"); menüyü sıfırdan kurmak 17A-1'i büyütürdü. Asistan kararı, kullanıcı "projeye en uygun" dedi. |
| Veri katmanı (2026-11-23) | Raf sorgusu `["me", kullanıcı, "library", "shelf", kitap]`. Komutun döndüğü raf önbelleğe doğrudan yazılır; kitaplık listesi ve rozetler eskitilir | Faz 16d kuralları: anahtar `["me", kullanıcı, "library"]` önekinin altında olduğu için çıkışta temizlik ve "kitaplığı eskit" onu kendiliğinden kapsar; "tek kayıt, değeri biliniyor" → doğrudan yazılır. |
| İstemcinin "bugün"ü (2026-11-23) | Yerel tarih (`getFullYear/getMonth/getDate`), testli tek yardımcıdan; `toISOString()` değil | `toISOString()` UTC günüdür: Türkiye'de 00:00–03:00 arasında bitirilen kitap dün bitmiş görünürdü (kavram 6). |

## 🚫 Kapsam dışı

- Şikâyet ve moderasyon kuyruğu (MOD-04) — Faz 18.
- Kitaplığın herkese açılması, profil sayfası, okuma istatistikleri (STATS-01), yıllık hedef.
- Sayfa ↔ yüzde dönüşümü; okuma notları / alıntılar; ilerleme günlüğü (her güncellemenin ayrı
  satırı) — yalnız son değer tutulur.
- İncelemeye beğeni, yanıt; inceleme listesinde sıralama seçenekleri (en yeni önce, sabit).
- Mevcut yorumlarda değişiklik (düzenleme, sahibinin silmesi, spoiler).
- Admin için gizlenmiş incelemeler listesi ekranı (geri açma ucu var, ekranı yok).
- Kitaplığım kartlarında komut menüsü (2026-11-23 kararı): kartlar 17A-1'de yalnız ilerleme gösterir;
  menü sonra, aynı komutlarla ayrı bir iş.

## 🗺️ Bölümler

- **17A-1 — okuma takibi:** migration `0016` (kolonlar + `reading_log`, kısıtsız), `internal/library`
  servisi, raf uçları, kitap sayfasında tek raf kontrolü, Kitaplığım'da ilerleme.
- **17A-2 — kısıt:** migration `0017` (temizlik + CHECK'ler). 17A-1 canlıda doğrulandıktan sonra.
- **17B — inceleme:** migration `0018_reviews`, `internal/reviews`, inceleme uçları ve admin gizleme,
  kitap sayfasında İncelemeler bölümü, spoiler gösterimi, bitirince inceleme daveti.

## 🧭 17A-1 nasıl takip edilecek

Faz 16d'deki gibi: asistan bir adımı yazar, **commit etmez**; o adımın kaydını aşağıya yazar (amaç,
sonuç, bakılacak yerler). Testleri asistan koşturur, CI her PR'da yeniden koşar; kullanıcıdan elle
deneme beklenmez (2026-11-24). Kullanıcı VS Code'da okur (*Source Control* → dosyaya tıkla:
önce/sonra yan yana); "tamam" deyince adım commit edilir. Bütün adımlar `feat/80-okuma-takibi` dalında
birikir ve tek PR olur. Her adım testleri yeşil bırakır.

| Adım | İçerik | Durum |
| --- | --- | --- |
| 0 | Spec'in koda göre gözden geçirilmesi | ✅ PR #120 (2026-11-24) |
| 1 | `0016_reading.sql` + sqlc sorguları + migration testi | ✅ commit (2026-11-24) |
| 2 | `internal/library` servisi: durum makinesi, tablo testleri | ✅ commit (2026-11-24) |
| 3 | API: komut uçları, reading PATCH, günlük uçları; eski `PUT {status}` servise | ✅ commit (2026-11-24) |
| 4 | Arayüz veri katmanı: tipler, raf kancaları, yerel tarih yardımcısı | ✅ commit (2026-11-24) |
| 5 | Arayüz: tek raf kontrolü, onaylar, 409 akışı, geçmiş | ✅ commit (2026-11-25) |
| 6 | Kitaplığım kartlarında ilerleme | ✅ commit (2026-11-25) |
| 7 | Playwright (iki sekme 409 dahil), dokümanlar, PR | ✅ PR [#121](https://github.com/mtgezgin/BookHub/pull/121) birleşti, canlıda doğrulandı (2026-11-25) |

### Adım 1 — Şema ve sorgular

- **Amaç:** okuma takibinin veritabanı tarafı: kitaplık satırına okuma alanları, kapanmış okumalar için
  `reading_log`, servisin (adım 2) kullanacağı sorgular. Bu adımda hiçbir uç ve ekran değişmez.
- **Neden ayrı bir adım:** migration canlıya kod çıkmadan **önce** uygulanır ve o an çalışan eski kodla
  uyumlu olmalı. Bunu tek başına sınamak, "şema eski kodu bozdu mu" sorusunu ayrı cevaplar.
- **Sonuç (2026-11-24):**
  - `TestMigration0016AddsReadingColumns` (yeni): Faz 15'ten kalan "Şu an okuyorum" kaydı 0016'dan sonra
    yerinde, okuma alanları boş, `version` 1; **eski kodun yazımı** (sqlc'nin ürettiği
    `UpsertLibraryEntry`) hâlâ çalışıyor; `reading_log` bitişi başlangıçtan önce olan ve
    FINISHED/ABANDONED dışı sonucu reddediyor; Down kolonları ve tabloyu kaldırıp kayıtları koruyor.
  - `TestEveryForeignKeyHasIndex` yeşil. Kuralın gerçekten yakaladığı da denendi: `reading_log_book_idx`
    geçici olarak silinince test `reading_log.book_id` için kırmızı oldu, dosya geri kondu.
  - Yerel yükseltme sınaması (`scripts/upgrade-check.sh`, `release`'in şeması ve demo verisi üstüne):
    0016 dolu tablolara uygulandı (şema sürümü 16). Betik commit'lenmemiş dosyayı "beklenmeyen" saydığı
    için son satırda durdu; asıl sınama PR'da CI'ın **Yükseltme** işi.
  - Bütün backend testleri (veritabanı ve Redis dahil), `go vet`, golangci-lint (0 sorun) temiz.
  - sqlc'nin `RETURNING *`'ı üretim anında açık kolon listesine çevirdiği `internal/db/library.sql.go`'da
    görüldü: eski kodun yeni kolonları hiç istememesinin nedeni bu (migration'daki not).
  - pgx'in `date` ↔ `time.Time` dönüşümü kaynağında doğrulandı (`pgtype/date.go`: yazarken yalnız
    yıl/ay/gün, okurken gece yarısı UTC); `sqlc.yaml`'a boş olabilen `date` → `*time.Time` kuralı eklendi.
- **Bakılacak yerler (bu sırayla):**
  1. [`backend/migrations/0016_reading.sql`](../../backend/migrations/0016_reading.sql): başlıktaki geriye
     uyum, tarih ve sürüm notları; yabancı anahtar index'leri neden kısmi.
  2. [`backend/sql/queries/library.sql`](../../backend/sql/queries/library.sql) sonundaki "Raf" bölümü:
     `LockShelf` neden `FOR UPDATE`, `SaveShelf` neden tek yazma yolu ve kitap yayında değilse ne olur,
     `GetReadingEdition` baskıyı nasıl seçer. `ListLibrary`'ye üç kolon eklendi.
  3. [`backend/sql/queries/reading_log.sql`](../../backend/sql/queries/reading_log.sql): her sorgu neden
     `user_id` ile süzülüyor (IDOR), "Devam et" en son kaydı neden kilitliyor.
  4. [`backend/sqlc.yaml`](../../backend/sqlc.yaml): `date` kuralı.
  5. [`backend/internal/database/migrate_0016_integration_test.go`](../../backend/internal/database/migrate_0016_integration_test.go).
  6. `backend/internal/db/*.go`: sqlc'nin ürettiği kod; okumana gerek yok.
- **Elle deneme yok (kullanıcı kararı, 2026-11-24):** bu testler CI'ın Backend işinde gerçek bir
  Postgres'le her PR'da koşar (`ci.yml`, `TEST_DATABASE_URL`); asistan da her adımda yerelde koşturup
  sonucu buraya yazar. Kullanıcı yalnız okur.

### Adım 2 — Durum makinesi servisi

- **Amaç:** kullanıcının niyet komutlarını (başla, bitir, yarım bırak, ertele, zaten okudum, çıkar) ve
  okurken düzeltmeyi (ilerleme, birim, baskı, başlangıç günü) tek transaction'da uygulayan servis; geçmişte
  tarih düzeltme ve silme. Bu adımda uç yok: servisi 3. adımdaki handler'lar çağıracak.
- **İki parça:**
  - `commands.go`: **saf** durum makinesi (`resolve`). Veritabanını bilmez; (durum, komut) → etki. Faz 4'ün
    `moderation.Next`'iyle aynı fikir: kural tek yerde, bütün tablo testle gezilir. Dokuz komut × beş
    durum, sekiz etkiye iner.
  - `service.go`: etkiyi veritabanında uygular. Sıra her komutta aynı: kitap yayında mı → (kullanıcı,
    kitap) kilidi → satır `FOR UPDATE` → `resolve` → yazım → güncel raf.
- **Sonuç (2026-11-24):**
  - `TestResolveExhaustive`: 45 çiftin tamamı beklenen etkide, hiçbiri hata dönmüyor.
  - `TestShelfEveryPairRealDB` (Doğrulama 1): 45 çiftin her biri gerçek veritabanında **iki kez**
    gönderiliyor. İlki beklenen duruma geçip beklenen sayıda günlük satırı yazıyor; ikincisi ne durumu ne
    günlüğü ne sürümü değiştiriyor.
  - Gerçek veritabanında ayrıca: başlarken toplamın görüntüsü (admin sayfa sayısını değiştirse de
    okuma 300'de kalıyor), devam et / baştan başla (Doğrulama 6), bitiş günü kuralı (4), ertele ve
    çıkarmada günlük (5), yayında olmayan kitapta 404 ama çıkarmak çalışıyor, sınır dışı ilerleme (2), eski
    sürümle 409 (3'ün servis yarısı), Faz 15'ten kalan okumanın ilk ilerlemesi (8), başkasının günlük kaydı
    404 (9), transaction ortasındaki hatada hiçbir şeyin değişmemesi (7).
  - **Testlerin gerçekten yakaladığı denendi** (bilinçli bozma, sonra geri alındı): danışma kilidi
    kaldırılınca eşzamanlı iki "Zaten okudum" günlüğe 2 satır yazdı ve test kırmızı oldu; "Devam et"in
    günlük satırını silmesi kaldırılınca üç test kırmızı oldu.
  - Bütün backend testleri, `go vet`, golangci-lint (0 sorun), sqlc kontrolü temiz.
  - Uygulamada netleşen çiftler ve kilit kararı yukarıda, § 3'te.
- **Bakılacak yerler (bu sırayla):**
  1. [`backend/internal/library/commands.go`](../../backend/internal/library/commands.go) `resolve`: makinenin
     tamamı tek fonksiyonda; üstündeki çizim.
  2. [`backend/internal/library/commands_test.go`](../../backend/internal/library/commands_test.go)
     `TestResolveExhaustive`: beklenen tablo, spec'in tablosunun kod hâli.
  3. [`backend/internal/library/service.go`](../../backend/internal/library/service.go): `Do` (sıra),
     `writer.apply` (her etki ne yazar), `resume`, `snapshot` (baskı ve birim seçimi), `UpdateReading`
     (404 ile 409'u neden kilitleyip ayırıyor), `withLog` (kilit sırası).
  4. [`backend/sql/queries/library.sql`](../../backend/sql/queries/library.sql) sonu: `BookIsApproved` ve
     `LockShelfKey` (danışma kilidi neden).
  5. [`backend/internal/library/service_integration_test.go`](../../backend/internal/library/service_integration_test.go):
     senaryolar, Doğrulama numaralarıyla.

### Adım 3 — API uçları

- **Amaç:** servisi HTTP'ye bağlamak: raf okuma, yedi komut, okurken düzeltme, geçmiş düzeltme ve silme
  (§ 4). Eski `PUT {status}` ve `DELETE` kendi yazımlarını bırakıp aynı servise geçer; böylece rafa
  yazan **tek yol** kalır. Arayüz henüz değişmez (4. adım).
- **Parçalar:**
  - `cmd/api/shelf.go` (yeni): `shelfService` arayüzü, cevap ve gövde DTO'ları, `writeShelfError`
    (hata → kod), yedi komutun tek üreticisi `handleShelfCommand` (§ Önemli noktalar "Sonar tekrar
    kapısı").
  - `cmd/api/library.go`: eski PUT durumu bir komuta çevirir (`library.CommandForStatus`), DELETE
    "çıkar" komutudur; liste okurken ilerlemeyi de döner.
  - `internal/library`: `Repository`'den `Put`/`Remove`, sorgulardan `UpsertLibraryEntry` kalktı;
    `Shelf`'e `BookID` eklendi (geçmiş uçları kitabı adresten değil kayıttan bilir).
- **Bu adımda verilen kararlar (§ 4'e de yazıldı):** çıkarma için güncel rafı döndüren
  `POST …/shelf/remove` eklendi, eski `DELETE` 204 olarak kaldı. Geçmiş kaydını silmek 204 değil
  200 + raf döner. Komut gövdesi isteğe bağlı; varsa yalnız o komutun alanları kabul edilir.
- **Sonuç (2026-11-24):**
  - `TestShelfEndpointsRealDB` (yeni, gerçek Postgres): yönlendirici, handler ve servis birlikte
    deneniyor. Kapsadığı senaryolar:
    - iki sekme: eski sürüm 409 `version_conflict` dönüyor, satır değişmiyor (Doğrulama 3'ün API
      yarısı);
    - eski `PUT READ` süren okumayı tarihsiz kapatıyor;
    - başkasının geçmiş kaydı her uçta 404 (9);
    - tarih ekleme çalışıyor, ters günler 400 `invalid_dates` dönüyor;
    - `DELETE` kitabı kitaplıktan çıkarıyor, geçmiş kalıyor (5);
    - geçmişten silme çalışıyor, ikinci silme 404;
    - yayında olmayan kitap 404, geçersiz ilerleme 400.
  - Birim testleri (sahte servisle):
    - her komutun rotası; gövdesiz istek varsayılanlarla çalışıyor;
    - her komut yalnız kendi alanlarını kabul ediyor (`bitir`'e `mode` 400). Takvimde olmayan gün
      (`2026-02-30`) ve saatli gün de 400;
    - `version` zorunlu;
    - servisin yedi hata türü × beş uç doğru koda düşüyor ve ham hata metni cevaba sızmıyor;
    - on bir rotanın hepsi token'sız 401;
    - cevap şekli sözleşme testiyle sabit: kullanıcı kimliği yok, günler `YYYY-AA-GG`.
  - Eski PUT'un eşlemesi (`TestCommandForStatus`): dört durum, beş başlangıç durumunun her birinden
    hedefe varıyor (Doğrulama 9).
  - **Testlerin gerçekten yakaladığı denendi** (bilinçli bozma, sonra geri alındı; dördü de kırmızı):
    - liste bayat ilerlemeyi de dönünce;
    - 409 yerine 400 dönünce;
    - eski `PUT READ` `finish`'e çevrilince;
    - `bitir` başla'nın gövdesini kabul edince.
  - **Yan bulgu, düzeltildi:** `ratings` testi bir koşuda "duplicate key users_email_key" ile kırmızı
    oldu, yeniden koşuda yeşildi. Nedeni şuydu: `testdb.Unique` zaman ve süreç içi sayaçtan üretiliyor,
    paketler ise ayrı süreçlerde paralel koşuyor. Bu makinede saat çok kaba (sıkı döngüde 100.000
    çağrının 99.996'sı aynı değer), iki paketin ilk kullanıcısı aynı e-postayı alabiliyordu. Değere
    süreç kimliği eklendi. CI Linux'ta koştuğu için orada görünmüyordu.
  - Bütün backend testleri iki kez arka arkaya, `go vet`, golangci-lint (0 sorun), sqlc kontrolü temiz.
  - `PROJECT_MAP` (`shelf.go`, `library`, `0016`) ve README'deki uç tablosu güncellendi; kalan
    dokümanlar 7. adımda.
- **Bakılacak yerler (bu sırayla):**
  1. [`backend/cmd/api/router.go`](../../backend/cmd/api/router.go) kitaplık grubu: yeni rotalar; yedi
     komut tek döngüden.
  2. [`backend/cmd/api/shelf.go`](../../backend/cmd/api/shelf.go): `handleShelfCommand` ve
     `readCommandInput` (hangi komut hangi alanı kabul eder), `writeShelfError` (hata haritası),
     `shelfResponse` (cevabın şekli).
  3. [`backend/cmd/api/library.go`](../../backend/cmd/api/library.go): `handlePutLibraryEntry`,
     `handleRemoveLibraryEntry` (eski uçlar servise), listede `if row.Status == library.StatusReading`.
  4. [`backend/internal/library/commands.go`](../../backend/internal/library/commands.go) `CommandForStatus`.
  5. [`backend/cmd/api/shelf_integration_test.go`](../../backend/cmd/api/shelf_integration_test.go):
     uçtan uca senaryo; [`shelf_test.go`](../../backend/cmd/api/shelf_test.go) birim testleri.

### Adım 4 — Arayüz veri katmanı

- **Amaç:** arayüzün rafla konuşacağı katman: tipler, API çağrıları, sorgu anahtarı, kancalar ve "bugün"
  yardımcısı. Ekranda hiçbir şey değişmez: yeni kancaları 5. adımdaki raf kontrolü kullanacak; o gelene
  kadar kitap sayfasındaki eski menü eski kancalarla çalışmaya devam ediyor.
- **Parçalar:**
  - `src/api/types.ts`: `Shelf`, `ShelfEntry`, `Reading`, `ReadingLogEntry`, `ReadingPatch`,
    `ReadingLogDates`; `LibraryItem`'a `unit`, `progress`, `total_pages`. `ShelfCommand` komutu ve
    gövdesini birlikte tanımlar: sunucunun "yalnız o komutun alanları" kuralı (adım 3) derleme anında da
    geçerli (`finish`'e `mode` yazmak tip hatası).
  - `src/api/client.ts`: beş çağrı (`shelf`, `shelfCommand`, `updateReading`, `updateReadingLog`,
    `deleteReadingLog`). Gövdesiz komut ağa da gövdesiz gider.
  - `src/api/queries.ts`: `queryKeys.shelf` = `["me", kullanıcı, "library", "shelf", kitap]`.
  - `src/api/libraryQueries.ts`: `useShelf` ve dört değişiklik kancası; ortak kurallar tek yerde
    (`useShelfMutation`).
  - `src/lib/day.ts` (yeni): `today()` ve `formatDay()`.
- **Kurallar (§ Kararlar "Veri katmanı" ve "409 cevabı"nın kodu):**
  - Başarıda cevap rafa **doğrudan yazılır**; önce o rafın süren okuması iptal edilir ki komuttan önce
    başlamış bir GET sonra gelip eski rafı yazmasın.
  - Neyin eskidiği kancaya göre: komutlar Kitaplığım listesini ve rozetleri, ilerleme düzeltmesi yalnız
    listeyi (durum değişmez) eskitir; geçmiş değişiklikleri hiçbir şeyi (kitaplık satırına dokunmazlar).
    Başka kitapların rafları hiç eskimez.
  - Hatada yalnız 409 `version_conflict` ve 404 rafı yeniden okutur, ve hata **raf geldikten sonra**
    yayılır: 5. adımdaki "İlerlemen başka bir yerde güncellenmiş: sayfa 57" mesajı yeni veriden kurulur.
    400 ve 500 hiçbir şeyi eskitmez.
- **Bu adımda verilen kararlar (asistan):**
  - 404 de rafı yeniden okutur (spec yalnız 409'u söylüyordu): okuma ya da geçmiş kaydı başka bir
    sekmede kapanmış/silinmişse ekrandaki raf eskidir; aynı gerekçe.
  - Öteki görünümler eskitilir ama **beklenmez** (Faz 15 kancaları bekliyordu): ekrandaki raf zaten
    güncel, kontrol listenin yeniden gelmesini beklemeden açılır.
  - `formatDay` da bu adımda: `new Date("2026-11-12")` UTC gece yarısıdır ve batıdaki saat dilimlerinde
    "11 Kasım" yazardı; "bugün"ün ikiz tuzağı, aynı dosyada.
- **Sonuç (2026-11-24):**
  - `day.test.ts` (yeni): testler saat dilimini **değiştirerek** koşuyor ve her biri önce değişimin
    gerçekten uygulandığını doğruluyor (yoksa makinenin kendi diliminde tuzağı görmeden yeşil kalırdı).
    İstanbul'da 01:00'de `today()` o günü veriyor, `toISOString()` bir önceki günü (Doğrulama 4'ün
    birim testi); New York'ta `formatDay("2026-11-12")` "12 Kasım 2026".
  - `libraryQueries.test.tsx`'e raf testleri: girişsiz sorgu yok; her kancanın neyi yazıp neyi eskittiği;
    cevap, cevabın söylediği kitabın rafına; 409 ve 404'te raf yeniden okunup hata ancak sonra yayılıyor;
    400 ve 500 hiçbir şeyi eskitmiyor; komuttan önce başlamış GET komutun cevabını ezmiyor.
  - `client.test.ts` uç tablosuna yedi satır (yöntem, yol, gövde) ve tip testi (yanlış gövde
    `npm run typecheck`te kırmızı).
  - **Testlerin gerçekten yakaladığı denendi** (bilinçli bozma, sonra geri alındı; yedisi de kırmızı):
    iptal kaldırılınca, 409'da raf beklenmeyince, bütün kitaplık öneki eskitilince, `today()` UTC günü
    verince, `formatDay` metni UTC okuyunca, `finish` `mode` kabul edince, gövdesiz komut `{}`
    gönderince. İlk denemede "409'da raf beklenmeyince" **kaçtı**: sahte GET hemen çözüldüğü için
    beklenmese de hata yayılmadan bitiyordu. Sahte GET bir zamanlayıcı turu geciktirildi, sonra yakaladı.
  - Bütün frontend testleri (431), `typecheck` ve `bundle` temiz.
- **Bakılacak yerler (bu sırayla):**
  1. [`frontend/src/api/libraryQueries.ts`](../../frontend/src/api/libraryQueries.ts) dosya sonu:
     `useShelfMutation`'ın yorumu ve `onSuccess` / `onError`; ardından dört kanca (neyi eskittikleri).
  2. [`frontend/src/api/types.ts`](../../frontend/src/api/types.ts) `ShelfCommand` ve raf tipleri.
  3. [`frontend/src/lib/day.ts`](../../frontend/src/lib/day.ts) ve
     [`day.test.ts`](../../frontend/src/lib/day.test.ts).
  4. [`frontend/src/api/libraryQueries.test.tsx`](../../frontend/src/api/libraryQueries.test.tsx)
     "raf" bölümü; özellikle son iki test (409 sırası ve yarış).

### Adım 5 — Tek raf kontrolü

- **Amaç:** kitap sayfasında Faz 15'in durum menüsü yerine raf kontrolü (§ 5): bulunduğun durum, oradan
  yapılabilecek komutlar, okurken ilerleme, yan etkili geçişlerde onay, 409 akışı ve okuma geçmişi.
- **Parçalar:**
  - `components/ShelfControl.tsx` (yeni, tembel parça): durum → ana eylem + "Diğer seçenekler" menüsü
    (dosyanın başındaki tablo § 5'tekiyle aynı), durum satırı, ilerleme çubuğu, onay diyaloğu, "Puan
    ver" daveti.
  - `components/ShelfForms.tsx` (yeni): ilerleme formu ve "Okumayı düzenle"; ikisinin ortak 409 akışı
    `useReadingSave`'de.
  - `components/ReadingHistory.tsx` (yeni): geçmiş satırları, günleri düzelt, sil (onaylı).
  - `lib/shelf.ts` (yeni): bütün metinler saf fonksiyon (ilerleme, durum satırları, gün aralığı, baskı
    adı, bildirim); adım 6'daki kartlar da aynı ilerleme metnini kullanacak.
  - `pages/BookDetail.tsx`: kontrol kendi ızgara alanında; "Puan ver" puan seçiciye götürür.
  - Kaldırılanlar: `LibraryMenu.tsx`, Faz 15'in durum kancaları (`useLibraryStatus`,
    `useSetLibraryStatus`), istemcideki `setLibraryStatus` ve `LibraryEntry` tipi. Sunucudaki eski
    `PUT {status}` **kalıyor** (yayın penceresindeki eski arayüz için, § Kararlar).
  - `e2e/library.spec.ts`: yeni kontrole göre (durum değişikliği artık "Zaten okudum" komutu).
- **Kararlar:** § 5'in sonundaki "Adım 5'te verilen kararlar" listesinde. Biri daha: ilerleme formunda
  tarayıcının kendi doğrulaması kapalı (`noValidate`). Tarayıcı `max=310`'u aşan değerde formu bizim
  denetimimizden önce durduruyor, kendi dilinde bir balon gösteriyordu; sınırı artık kod denetliyor,
  mesaj Türkçe ve `role="alert"` ile ekran okuyucuya ulaşıyor (test bunu yakaladı).
- **Sonuç (2026-11-25):**
  - `LibraryButton.test.tsx` baştan yazıldı (29 test). Her test bir raftan başlıyor; hangi komutun
    hangi gövdeyle gittiğine ve ekranda ne kaldığına bakıyor:
    - beş durumun her birinde menü ve ana eylem; "Okumaya başla" ve "Bitirdim" bugünün yerel gününü,
      "Zaten okudum" tarihsiz, "Devam et" günsüz (sunucu eskisini getirir) gönderiyor;
    - Faz 15'ten kalan birimsiz okuma ve toplamı bilinmeyen 0 sayfa: çubuk ve "0 sayfa" yok,
      "İlerlemeni gir" var (Doğrulama 8);
    - ilerleme varken Ertele ve Çıkar sorar, yokken Ertele sormaz; çıkarılan kitabın geçmişi görünür
      kalıyor (5);
    - **409:** öteki sekmenin değeri (80) görünüyor, girilen (57) korunuyor, "Benimkini kaydet" yeni
      sürümle gönderiyor; Vazgeç öteki sekmenin değerini bırakıyor (3'ün arayüz yarısı);
    - "Okumayı düzenle" yalnız değişeni gönderiyor, baskı değişince sıfırlanacağını söylüyor;
    - geçmiş: satır metni, ters günlerde istek gitmiyor, silme onaylı;
    - "Puan ver" yalnız bitirince; komut sürerken her şey pasif; raf okunamazsa tekrar dene;
      komuttan sonra aynı sayfadaki rozet de güncelleniyor.
  - `lib/shelf.test.ts` (yeni, 31 test): metinlerin tablosu.
  - **Testlerin gerçekten yakaladığı denendi** (bilinçli bozma, sonra geri alındı; onu da kırmızı):
    Devam et gün gönderince, ilerleme varken Ertele sormayınca, çıkarma onayı ilerlemeyi söylemeyince,
    davet her Okudum'da çıkınca, komut sürerken kontroller açık kalınca, 409 mesajı gösterilmeyince,
    düzenle formu değişmeyeni de gönderince, ters günler sunucuya gidince, toplamsız 0 sayfa
    yazılınca, ilerleme toplamı aşabilince.
  - Bütün frontend testleri, `typecheck` ve `bundle` temiz. Raf kontrolü ayrı parçada kaldı
    (`ShelfControl` 22,8 kB gzip; ana paket 121,6 → 121,8 kB).
  - Playwright yerel yığında (`make e2e`, canlı değil): 20 senaryonun 20'si yeşil; güncellenen
    `library.spec.ts` dahil, 360 px genişlikte kitap sayfası ve Kitaplığım yatay taşmıyor.
- **Bakılacak yerler (bu sırayla):**
  1. [`frontend/src/components/ShelfControl.tsx`](../../frontend/src/components/ShelfControl.tsx):
     baştaki tablo, sonra `switch (entry?.status)` (hangi durumda hangi komut) ve onay diyaloğu.
  2. [`frontend/src/components/ShelfForms.tsx`](../../frontend/src/components/ShelfForms.tsx)
     `useReadingSave` (409 akışı).
  3. [`frontend/src/lib/shelf.ts`](../../frontend/src/lib/shelf.ts): ekranda yazan her cümle.
  4. [`frontend/src/components/ReadingHistory.tsx`](../../frontend/src/components/ReadingHistory.tsx).
  5. [`frontend/src/components/LibraryButton.test.tsx`](../../frontend/src/components/LibraryButton.test.tsx)
     "ilerleme ve 409" bölümü.

### Adım 6 — Kitaplığım kartlarında ilerleme

- **Amaç:** "Şu an okuyorum" kartlarında ilerleme (§ 5 "Kitaplığım"; yalnız gösterim, komutlar kitap
  sayfasında, § Kararlar "Kitaplığım kartları").
- **Parçalar:**
  - `components/ReadingProgress.tsx` (yeni): çubuk + metin. Kitap sayfasındaki raf kontrolü de artık
    bunu kullanıyor (adım 5'teki `ProgressLine` buraya taşındı): iki yerde aynı biçim, aynı metin
    (`lib/shelf.ts`).
  - `pages/Library.tsx`: kartta yalnız `status === "READING"` ise ilerleme. Çıkarma onayında
    "okuma geçmişin silinmez" de yazıyor (katalogdan kalkmış ama okunan bir kitap çıkarılırken günlüğe
    yazılabilir).
- **Bu adımda verilen kararlar (asistan):**
  - Kartta ilerleme girilmemişse **hiçbir şey** yazmaz (kitap sayfası "İlerlemeni henüz girmedin" der):
    kartlar yalnız gösterim, yapılacak bir şey önermeyen bir satır ızgarada gürültü olurdu.
  - Her kartta bir çubuk olduğu için çubuğun erişilebilir adı kitabı da söylüyor ("Dune: okuma
    ilerlemen"); yoksa ekran okuyucu on iki aynı ad okurdu.
  - İstemci de durumu denetliyor: sunucu alanları yalnız READING'de doldurur (adım 3), gelse bile başka
    durumda gösterilmez.
- **Sonuç (2026-11-25):**
  - `Library.test.tsx`'e altı test: sayfa (toplamlı), yüzde, toplamsız sayfa (çubuk yok); girilmemiş
    ilerleme, toplamsız 0 sayfa ve okunmayan kitapta kartta ilerleme yok (Doğrulama 8).
  - **Testlerin gerçekten yakaladığı denendi:** durum koşulu kaldırılınca ve çubuğun adından kitap
    çıkarılınca kırmızı (ikisi de, sonra geri alındı).
  - Bütün frontend testleri (485), `typecheck` temiz.
- **Bakılacak yerler:**
  1. [`frontend/src/components/ReadingProgress.tsx`](../../frontend/src/components/ReadingProgress.tsx).
  2. [`frontend/src/pages/Library.tsx`](../../frontend/src/pages/Library.tsx) kartın `extra`'sı.

### Adım 7 — Uçtan uca test, dokümanlar, PR

- **Amaç:** okuma akışının gerçek yığında (NGINX + API + Postgres + Redis, tarayıcı) sınanması, iki sekme
  senaryosu dahil; dokümanlar; tek PR.
- **Parçalar:**
  - `e2e/library.spec.ts`: ikinci senaryo **okuma** — başla → ilerle → **iki sekmede 409** → yarım bırak
    → devam et → Kitaplığım kartında ilerleme → bitir → "Puan ver" puan seçiciye odaklanıyor → çıkar
    (geçmiş kalır). Okurken 360 px'te kitap sayfası (çubuk, açık form, geçmiş) yatay taşmıyor.
  - İki sekme: A ilerleme formunu açıp 50 yazar; B aynı bağlamda (aynı oturum) 57 kaydeder; A
    gönderince 409. A'da öteki sekmenin değeri ("57 / 704 sayfa") görünür, A'nın 50'si alanda kalır; B
    yenilenince 57 durur (409 satırı değiştirmedi); A "Benimkini kaydet" deyince ikisinde de 50.
  - `e2e/helpers.ts` bütçe tablosu: `library.spec` yine **1 giriş**.
  - STATUS, ROADMAP, PROJECT_MAP: 17A-1 PR'da. FEATURE_REQUESTS ve "canlıda" ibareleri yayından ve
    canlı doğrulamadan sonra (§ Tamamlanma).
- **Bu adımda verilen karar (asistan):** okuma senaryosu **ayrı dosya değil**, `library.spec.ts`'te
  ikinci senaryo; ikisi `describe.serial` + `beforeAll`'da tek girişle aynı sayfayı paylaşıyor. Neden:
  NGINX'in giriş kovası koşu başına 11 istek; bugün 10 kullanılıyor ve kural "11'in altında tut"
  (helpers.ts). Ayrı dosya kendi girişiyle tavana dayardı. İkinci sekme aynı tarayıcı bağlamında
  açıldığı için oturum çerezle geliyor, giriş harcamıyor.
- **Sonuç (2026-11-25):**
  - Playwright yerel yığında (`make e2e`, canlı değil): **21 senaryonun 21'i** yeşil. İlk koşuda okuma
    senaryosu kırmızıydı: `browser.newPage()` ile açılan sayfanın bağlamına ikinci sekme eklenemiyor;
    bağlam açıkça açıldı.
  - Zamanlama kırılganlığına karşı kitaplık + okuma dosyası **beş kez arka arkaya** (tek işçiyle): 10'un 10'u yeşil. Paralel tekrar (`--repeat-each` varsayılanı) anlamsız kırmızı verdi: beş kopya aynı okur hesabıyla aynı kitabı aynı anda değiştiriyordu; normal pakette bu kitaba ve hesaba dokunan başka senaryo yok.
    (409'u kaçırtabilecek tek şey A sekmesinin kendiliğinden tazelenmesi; önbelleğin 30 sn'lik bayatlama
    süresi senaryonun süresinden uzun, pencereye dönünce tazeleme bu sürede olmaz.)
  - Bütün backend testleri (gerçek Postgres + Redis) yeşil; frontend 485, `typecheck`, `bundle`
    temiz; `scripts/checks.sh` yalnız yerelde kurulu olmayan sqlc'yi söylüyor (CI denetler).
- **Bakılacak yerler:** [`frontend/e2e/library.spec.ts`](../../frontend/e2e/library.spec.ts) dosyanın
  başındaki açıklama ve "Okuma (Faz 17A-1)" bölümü.
- **PR'dan sonra, birleşmeden önce: SonarCloud'un 27 bulgusu (2026-11-25).** Kalite kapısı geçti;
  bulgular kontrol kaydının GitHub notlarından okundu (Sonar'ın anonim API'si PR analizini vermiyor).
  - **Düzeltilen 15:**
    - 11 bileşende prop'lar `Readonly<…>` (S6759; örnek `BookDetail.tsx` `StarPicker`);
    - `ReadingEditForm`'da "Baskı" ile `<select>` arasındaki belirsiz boşluk (S6772);
    - ilerleme çubuğu elle ARIA'lı `<div role="progressbar">` yerine yerleşik `<progress>` (S6819). Rol,
      değer ve en büyük değer tarayıcıdan gelir. Görünüm motorların sözde öğeleriyle; Chromium'da eskisiyle
      aynı olduğu ekran görüntüsüyle karşılaştırıldı (%0, %14, %100). Firefox'un `::-moz-progress-bar`'ı
      yerelde denenmedi, çünkü Playwright'ın yalnız Chromium'u kurulu;
    - Go'da bilişsel karmaşıklık (S3776):
      - `resolve` komut başına bir yardımcıya bölündü (17 → eşiğin altı);
      - `UpdateReading`'den yamanın okumaya uygulanması `applyPatch`'e alındı (23 → eşiğin altı).
      - Davranış değişmedi: aynı testler yeşil.
  - **Testlerin kırıldığı denendi:** çubuğun değeri ya da metni bozulunca arayüz testleri kırmızı;
    `applyPatch`'te ilk giriş ya da `resolveAbandon`'da bir dal bozulunca servis testleri kırmızı.
  - **Kodla düzeltilmeyen 12:**
    - 5 tanesi `library.sql`'de "tekrarlanan metni sabite çevir" (S1192). Tekrarlanan metinler sqlc
      parametre adları (`'user_id'`, `'status'`) ve `'APPROVED'`; SQL sorgu dosyasında sabit yok;
    - 6 tanesi test fonksiyonlarında karmaşıklık. Bunlar senaryo testleri: alt testler sırayla aynı
      durumu paylaşıyor. Bölmek okunurluğu artırmaz;
    - 1 tanesi raf kontrolünün `role="group"`'u (fieldset önerisi). Grubun içinde form denetimleri
      değil, durum satırı, çubuk, düğmeler ve paneller var. Uygulamadaki dokuz grup da aynı biçimde.
  - `.sonarcloud.properties` bulgu yoksaymayı (`sonar.issue.ignore.*`) desteklemiyor. Bu 12 bulgu Sonar
    panelinde gerekçesiyle "Accepted" işaretlenir (kullanıcı kararı; henüz yapılmadı).
  - Not: `e799e7f`'in commit mesajı "14 düzeltildi, 13 kaldı" diyor. Doğrusu 15 ve 12: `library.sql`'deki
    bulgular 6 değil 5. Düzeltmeden sonraki Sonar analizi açıkta tam 12 bulgu gösterdi. Push edilmiş
    commit değiştirilmedi.

## 🌐 17A-1 yayın ve canlı doğrulama (2026-11-25)

- **Birleşme:** PR [#121](https://github.com/mtgezgin/BookHub/pull/121) (`3accf7d`), sekiz kontrolün
  sekizi yeşil.
- **Deploy** ([koşu 45561911429](https://github.com/mtgezgin/BookHub/actions/runs/45561911429)):
  - Plan adımı yeni migration olarak yalnız `0016_reading.sql`'i buldu.
  - Migration'dan **önce** şifreli döküm alındı: `bookhub-migration-oncesi-3accf7d.dump.age`, 57 143 bayt.
  - Döküm `bookhub-yedek-migration-oncesi-45561911429` artefaktı olarak yüklendi; 90 gün, 2027-02-23'e
    kadar saklanıyor.
  - Bu, yedek düzeneğinin gerçek bir migration öncesindeki ilk koşusu
    ([#100](https://github.com/mtgezgin/BookHub/issues/100) kabul kriteri 3).
  - Sonra `manage migrate` "migration'lar güncel" dedi ve `release` `3accf7d`'ye ilerledi.
  - Yayın kontrolü:
    - canlı site raf kontrolünü içeren yeni paketi sunuyor;
    - `GET /api/me/books/1/shelf` girişsiz 401 dönüyor, yani uç var (eski sürümde 404 olurdu).
- **Canlıda tarayıcıyla (spec Doğrulama 10):** Chrome'da kullanıcının kendi hesabıyla, kitaplığı boşken,
  Martin Eden (480 sayfa) üzerinde. Demo hesaplarıyla giriş yapılamıyor: tohum verisi onları parolasız
  açıyor (`password_hash '!'`).

  | Adım | Sonuç |
  |---|---|
  | Okumaya başla | ✅ "Şu an okuyorum", çubuk "0 / 480 sayfa" |
  | İlerleme 120 | ✅ "120 / 480 sayfa", çubuk %25; sayfa yenilenince korunuyor |
  | Yarım bıraktım | ✅ "Yarım bıraktın · 120 / 480 sayfa"; geçmişte "25 Kasım 2026 – 25 Kasım 2026" (yerel gün) |
  | Devam et | ✅ yine 120. sayfa, geçmiş satırı kalktı |
  | Kitaplığım | ✅ kartta "Şu an okuyorum" rozeti ve "120 / 480 sayfa" çubuğu |
  | Bitirdim | ✅ "Okudum · Bugün bitirdin", "Puan ver" daveti, geçmişte "Bitirdin · 25 Kasım 2026" |
  | Temizlik | ✅ geçmiş satırı silindi (`DELETE …/reading-log/2` 200), kitap çıkarıldı (`POST …/shelf/remove` 200); hesapta iz yok |

- **Doğrulanmayan:**
  - Ağ izlemesi yalnız temizlik adımlarında açıktı: başla, ilerle, yarım bırak, devam et ve bitir
    adımlarının durum kodları kaydedilmedi. Her adımda arayüz başarı bildirimi gösterdi ve sunucunun
    cevabını çizdi.
  - API loglarında 5xx'e bakılmadı.
  - İki sekme 409 senaryosu canlıda denenmedi; yerelde ve CI'da Playwright'la sınanıyor.
  - Firefox'ta `<progress>` görünümü.
- **Sırada:** 17A-2 (`0017`, kısıtlar; öncesinde canlıda kural dışı satır sayımı), sonra 17B. #80 açık
  kalır.

## 🧭 17A-2 — kısıtlar (`0017`)

17A-1 gibi: asistan yazar, kullanıcı okur, "tamam" deyince commit; dal `feat/80-okuma-kisitlari`, ayrı PR
(CLAUDE.md: daraltma, genişletme yayını canlıda doğrulandıktan sonra ayrı PR'dır).

| Adım | İçerik | Durum |
| --- | --- | --- |
| 1 | `0017_reading_checks.sql` + migration testi; kısıtla çakışan iki servis testi | ✅ commit (2026-11-25) |
| 2 | Canlıda sayım, dokümanlar, PR | ✅ PR [#123](https://github.com/mtgezgin/BookHub/pull/123) birleşti, canlıda doğrulandı (2026-11-25) |

**Canlıda sayım (2026-11-25, kullanıcı Supabase SQL Editor'da; yalnız okuyan, yalnız sayı döndüren
sorgu, metni PR'da):** `library_entries` toplam **0** satır, temizlenecek **0**, kural dışı ilerleme
**0**. Beklenen: demo tohum verisi kitaplık kaydı oluşturmuyor (yalnız puan yazıyor) ve 17A-1'in canlı
doğrulamasında kullanılan kayıt sonunda çıkarıldı. `0017` canlıda hiçbir satırı değiştirmeyecek;
kısıtlar boş tabloya eklenecek.

### 17A-2 adım 1 — Migration ve testler

- **Amaç:** okuma kuralları veritabanında da zorlansın (spec Doğrulama 11). Servis aynı kuralları Go'da
  zaten denetliyor (`validProgress`, okumuyorken boş alanlar); kısıt, servisi atlayan bir yazımı
  (elle SQL, ileride başka bir kod yolu) durdurur.
- **Parçalar:**
  - `backend/migrations/0017_reading_checks.sql`:
    - önce temizlik: okunmayan kitapta kalmış okuma alanları boşaltılır (yayın penceresinin bayat
      alanları, § 1);
    - sonra iki kısıt: `library_reading_only_when_reading` ve `library_progress_valid`.
    - Kural dışı **ilerleme** temizlenmez: varsa `ADD CONSTRAINT` düşer, transaction geri alınır ve
      Deploy durur (`release` ilerlemez). Sessizce silmek yerine nedenini görmek için; yeni kod böyle
      satır yazamaz, eski kod ilerleme hiç yazmazdı. Canlı sayım adım 2'de.
    - `Down` kısıtları kaldırır; temizlik geri gelmez (boşaltılan alanlar anlamsızdı).
  - `internal/database/migrate_0017_integration_test.go`: veritabanı `0016`'ya kadar kurulur, bayat bir
    "okudum" satırı ve üç geçerli okuma yazılır. `0017`'den sonra:
    - bayat alanlar temizlenmiş, geçerli okumalar olduğu gibi;
    - 5 geçerli yazım kabul, 11 kural dışı yazım ret;
    - `Down`'dan sonra kısıt yok.
  - Servis testleri, 17A-1'in bütün senaryoları kısıtlar altında yeniden koştu. İkisi `0017`'nin
    yasakladığı satırı elle yazıyordu ve kırıldı. Kısıt tam yapması gerekeni yaptı:
    - "bayat okuma alanları gösterilmez (0017'den önce)" kaldırıldı. O satır artık yazılamıyor;
      temizliği ve reddi migration testi sınıyor.
    - `TestShelfAtomicityRealDB` transaction ortasındaki hatayı kural dışı bir ilerlemeyle (−5)
      üretiyordu. Artık yalnız o testin kullanıcısının günlük yazımını reddeden geçici bir tetikleyiciyle
      üretiyor. Tetikleyici, aynı transaction'da satırın o an değişip değişmediğini hata mesajına yazıyor.
      Test, hatanın satır yazıldıktan **sonra** geldiğini ve satırın geri alındığını görüyor.
  - Yorumlar: "0017'ye kadar bayat kalabilir" diyen dört yer (servis, API, sorgu, sqlc'nin ürettiği
    dosya) güncellendi.
- **Bu adımda verilen kararlar (asistan):**
  - **Spec'teki taslak düzeltildi** (§ 2 "Uygulamada değişen"): NULL tuzağı ve `total_pages > 0`.
  - **Kural dışı ilerleme temizlenmiyor, Deploy duruyor:** yukarıda; sayım bunu önceden gösterecek.
  - `NOT VALID` + `VALIDATE` ayrımı yok: tablo küçük, kısa bir kilit yeterli (`lock_timeout` 5 sn, 0016
    gibi).
- **Sonuç (2026-11-25):**
  - Bütün backend testleri gerçek Postgres + Redis'le yeşil; golangci-lint 0 sorun; sqlc `diff` temiz.
  - Testlerin gerçekten yakaladığı bilinçli bozmayla denendi:
    - kısıt spec'teki taslağa çevrilince 4 kural dışı yazım geçiyor (birimsiz ilerleme, ilerlemesiz
      sayfa, ilerlemesiz yüzde, sıfır sayfa toplamı) → test kırmızı;
    - yalnız `total_pages > 0` kaldırılınca 1 → kırmızı;
    - temizlik `UPDATE`'i kaldırılınca migration bayat satırda düşüyor → kırmızı;
    - atomiklik testinde tetikleyici kaldırılınca → kırmızı;
    - "Bitirdim" önce günlüğü sonra satırı yazacak biçimde çevrilince → kırmızı. Bu bozmayı ilk
      tetikleyici yakalayamamıştı (hata satırdan önce geldiği için geri alınacak bir şey yoktu); hata
      mesajına satırın durumu bu yüzden eklendi.
- **Bakılacak yerler:**
  - [`0017_reading_checks.sql`](../../backend/migrations/0017_reading_checks.sql): baştaki açıklama ve
    "NULL TUZAĞI";
  - [`migrate_0017_integration_test.go`](../../backend/internal/database/migrate_0017_integration_test.go):
    durum tablosu;
  - `service_integration_test.go`: `TestShelfAtomicityRealDB` ve `failLogWrites`.
- **PR #123'te SonarCloud (2026-11-25):** tek bulgu, migration testinin bilişsel karmaşıklığı (19 > 15).
  Düzeltildi:
  - yardımcılar (`readingDB`: `must`, `read`, `tryEntry`) paket düzeyine alındı;
  - dört satır kontrolü tek bir beklenti tablosu oldu;
  - kabul/ret tek koşul oldu.

  Sınanan şeyler aynı. Spec taslağına ve temizliksiz migration'a karşı bozmalar yine kırmızı. Kural
  (kullanıcı, 2026-11-25): Sonar bulguları her zaman düzeltilir; düzeltilmeyecekse gerekçesi yazılır.

## 🌐 17A-2 yayın ve canlı doğrulama (2026-11-25)

- **Birleşme:** PR [#123](https://github.com/mtgezgin/BookHub/pull/123) (`8f71e63`). Sekiz kontrol yeşil;
  Sonar'ın tek bulgusu birleşmeden önce düzeltildi (yukarıda).
- **Deploy** ([koşu 45604676755](https://github.com/mtgezgin/BookHub/actions/runs/45604676755)):
  - plan adımı yeni migration olarak yalnız `0017_reading_checks.sql`'i buldu;
  - migration'dan **önce** şifreli döküm alındı: `bookhub-migration-oncesi-8f71e63.dump.age`, 62 968 bayt;
    artefakt `bookhub-yedek-migration-oncesi-45604676755`, 90 gün;
  - "migration'lar güncel"; `release` `8f71e63`'e ilerledi.
- **Canlıda tarayıcıyla (spec Doğrulama 11'in canlı yarısı):**
  - Kim yaptı: kullanıcının tarayıcısındaki Claude, asistanın yazdığı istemle. Asistanın Chrome
    bağlantısı bu sefer sayfayı açamadı (üç denemede tarayıcı hata sayfası). Aynı adresler curl'le 200
    döndü, yani sorun siteden değil Chrome tarafındaydı.
  - Hesap: kullanıcının kendi hesabı, kitaplığı başta boş. Kitap: Martin Eden (480 sayfa).

  | Adım | Sonuç |
  |---|---|
  | Okumaya başla | ✅ "Şu an okuyorum", "0 / 480 sayfa" |
  | İlerleme 120 | ✅ "120 / 480 sayfa" |
  | İlerleme 500 | ✅ form göndermedi: "0 ile 480 arasında bir tam sayı gir."; değer 120 kaldı |
  | Bitirdim | ✅ "Okudum · Bugün bitirdin"; geçmişte "Bitirdin · 25 Kasım 2026" |
  | Temizlik | ✅ geçmiş satırı silindi, kitap çıkarıldı; kitaplık yine boş |

  - **Ağ:** `/api/me/` içeren 12 isteğin hepsi 200 döndü (`shelf/start`, `shelf/reading` PATCH,
    `shelf/finish`, `reading-log/3` DELETE, `shelf/remove` dahil). Kırmızı bildirim ya da konsol hatası yok.
    Konsol izlemesi bitirme adımından sonra başladı.
- **Doğrulanmayan:**
  - Kural dışı değerin (500 sayfa) **sunucuda** reddedilmesi canlıda sınanmadı: istemci isteği hiç
    göndermedi. Sunucuda iki katman var: servis 400 döndürüyor (`TestShelfEndpointsRealDB`, −1 → 400) ve
    veritabanı kısıtı reddediyor (`TestMigration0017ReadingChecks`).
  - Kısıtların canlı veritabanında var olduğu doğrudan görülmedi (asistanın canlı veritabanına erişimi
    yok). Migration adımının hatasız bitmesi kısıtların eklendiğini gösteriyor.
- **17A bitti.** Sırada 17B (inceleme, `0018`). #80 açık kalıyor.

## 🪜 17A — adımlar

### 1. Şema: `0016_reading.sql` (genişletme)

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
-- Yabancı anahtar index'leri (0015'ten beri kural; schema_integration_test denetler).
CREATE INDEX library_entries_edition_idx ON library_entries (edition_id) WHERE edition_id IS NOT NULL;
CREATE INDEX reading_log_book_idx ON reading_log (book_id);
CREATE INDEX reading_log_edition_idx ON reading_log (edition_id) WHERE edition_id IS NOT NULL;
```

- **Yabancı anahtar index'i (2026-11-23 eklendi):** her FK kolonu bir index'in ilk kolonu olmalı;
  kitap ya da baskı silinince Postgres bağlı satırları bununla arar, index yoksa bütün tabloyu tarar
  (`TestEveryForeignKeyHasIndex`). Kısmi index yalnız `WHERE <kolon> IS NOT NULL` ise sayılır (denetim
  NULL aramaz); `edition_id` çoğunlukla boş olacağı için ikisi kısmi. `user_id`'ler birincil anahtar ve
  `reading_log_user_book_idx` ile zaten karşılanıyor.
- `library_entries`'e eklenen her kolon boş olabilir ya da varsayılanlıdır: canlıdaki (eski) kod
  bunları hiç anmadan çalışmaya devam eder. `reading_log` yeni tablo olduğu için kısıtları baştan taşır.
- Eski kod yayın penceresinde okuyan bir satırın durumunu değiştirirse okuma alanları satırda **bayat**
  kalabilir. Yeni kod bu alanları yalnız `status = 'READING'` iken okur ve her "başla"da üzerine yazar;
  bayat alanlar 0017'de temizlenir.

### 2. Şema: `0017_reading_checks.sql` (daraltma, ayrı PR)

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
        CASE
            WHEN unit IS NULL THEN progress IS NULL AND total_pages IS NULL
            WHEN unit = 'percent' THEN progress IS NOT NULL AND progress BETWEEN 0 AND 100
                                       AND total_pages IS NULL
            WHEN unit = 'page' THEN progress IS NOT NULL AND progress >= 0
                                    AND (total_pages IS NULL OR (total_pages > 0 AND progress <= total_pages))
            ELSE false
        END);
```

- Önce canlıda sayım: temizlenecek satır var mı, kural dışı ilerleme var mı (PR'da yazılır).
- **Uygulamada değişen (2026-11-25, 17A-2):** ilk taslak `(unit = 'percent' AND …) OR (unit = 'page' AND …)`
  biçimindeydi. CHECK ifadesi NULL dönerse Postgres satırı **kabul eder**. Bu yüzden taslak, birimi boş
  ama ilerlemesi 5 olan satırı ya da birimi `page` ama ilerlemesi boş olan satırı geçiriyordu. `CASE`'in
  her dalı yalnız true/false döner. Ayrıca bilinen sayfa toplamı 0'dan büyük olmalı (`reading_log` ile
  aynı kural; `editions.page_count` kısıtsız).

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
- **Uygulamada netleşen çiftler (2026-11-24, adım 2):** okumuyorken `finish` = `mark-read` (verilen bitiş
  günüyle); READ ya da ABANDONED'dan `want`/`postpone` = yalnız durum (okuma zaten kapanmış); okumuyorken
  `abandon` = yalnız durum (eski `PUT`'un davranışı); READ'den `start` = yeniden okuma. `mark-read`
  gelen tarihi yok sayar. Tam tablo: `internal/library/commands_test.go` `TestResolveExhaustive`.
- **Kilit (2026-11-24):** satır `FOR UPDATE` yetmiyor, çünkü kitap kitaplıkta yokken kilitlenecek satır
  yok: iki eşzamanlı "Zaten okudum" ikisi de "yok" görüp günlüğe iki satır yazardı (testte görüldü).
  Her komut önce (kullanıcı, kitap) için bir **danışma kilidi** (`pg_advisory_xact_lock`) alır; kilit
  bir satıra değil bir sayıya konur, transaction bitince bırakılır. Günlük işlemleri de aynı sırayla
  kilitler (raf → günlük), yoksa "Devam et" ile "geçmişten sil" birbirini bekleyip kilitlenirdi.
- Yeni hata kodu: `invalid_edition` (400) — seçilen baskı bu kitabın değil.
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
POST   /api/me/books/{id}/shelf/remove      çıkar, güncel rafı döner (adım 3)
PATCH  /api/me/books/{id}/shelf/reading     {version, progress?, started_on?, unit?, edition_id?}
DELETE /api/me/library/{id}                 çıkar (mevcut uç, artık servisten)
PATCH  /api/me/reading-log/{id}             {started_on?, ended_on?}
DELETE /api/me/reading-log/{id}
```

- Her komut güncel rafı döner (istemci ayrıca GET yapmaz; önbelleğe doğrudan yazar, § Kararlar).
- Gövdeler ayrı DTO + `DisallowUnknownFields`; başkasının günlük kaydı 404.
- Hatalar: `version_conflict` (409, yalnız PATCH reading; gövde standart hata zarfı, güncel raf için
  istemci GET yapar), `invalid_progress` (400), `invalid_dates` (400), `invalid_edition` (400).
- **Cevap şekli (2026-11-24, adım 3):** `{book_id, entry, log}`. `entry` null = kitaplıkta değil;
  `entry.reading` yalnız READING'de dolu, içinde `unit` null = ilerleme henüz girilmedi; `log` en yeni
  önce, kitaplıkta değilken de dolu olabilir. Günler `"YYYY-AA-GG"` (saatsiz).
- **Gövdeler (adım 3):** komut gövdesi isteğe bağlı (tek tık = varsayılanlar); varsa yalnız o komutun
  alanlarını kabul eder: `start` → `mode, edition_id, unit, started_on`; `finish`/`abandon` →
  `ended_on`; diğerleri yalnız `{}`. Takvimde olmayan ya da saatli gün 400 `validation_error`.
- **`POST …/shelf/remove` (adım 3, asistan kararı):** ilk listede çıkarma yalnız eski `DELETE`'ti. O uç
  Faz 15'in gövdesiz 204'üyle kalır (yayın penceresindeki eski arayüz onu bekliyor), yeni arayüz güncel
  rafı döndüren komutu kullanır ("her komut güncel rafı döner": çıkarma geçmişe satır yazabilir).
  Aynı gerekçeyle `DELETE /api/me/reading-log/{id}` de 204 değil 200 + raf döner.
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
- **409 `version_conflict`:** raf yeniden okunur; "İlerlemen başka bir yerde güncellenmiş: sayfa 57"
  (yeni veriden) + kullanıcının girdiği değer korunur; yeniden gönder (yeni sürümle) ya da vazgeç.
  Sessiz yeniden deneme yok.
- **Geçmiş:** kontrolün altında kapanmış okumalar (tarih, baskı, sonuç; tarihi düzelt / sil).
- **Kitaplığım:** "Şu an okuyorum" kartlarında ilerleme (yalnız gösterim; menü kapsam dışı).
- Kart rozetleri (`/api/me/library/statuses`) değişmez; kitap sayfası artık rozet ucunu değil raf
  ucunu okur. Raf kontrolü bugünkü `LibraryMenu`'nün yerini alır (aynı tembel yükleme).
- **Adım 5'te verilen kararlar (asistan, 2026-11-24):**
  - Kontrol kitap sayfasında **kendi alanında** (`shelf`): ilerleme, formlar ve geçmişle başlığa
    sığmıyor. Dar ekranda kapak ve başlığın altında tam genişlik, genişte başlığın altında.
  - Menüdeki "Baskı/birim değiştir" → **"Okumayı düzenle"**: baskı, birim ve başlangıç günü tek
    formda (üçü de aynı PATCH).
  - Metinler sayıya ek getirmez ("Sayfa 42'de", "2025'te" sayının okunuşuna göre değişir):
    "Bitirdin · 12 Kasım 2026", "Yarım bıraktın · 42 / 310 sayfa", onayda "Okuman (42 / 310 sayfa)
    geçmişine … kaydedilecek".
  - Çıkarma, ilerleme olmasa da **her zaman sorar** (Faz 15'teki gibi; yalnız metni değişir).
  - "Puan ver" daveti yalnız bu ekranda bitirilince ve puan yoksa; tıklanınca puan seçiciye gider.
  - 409 akışı "Okumayı düzenle"de de aynı ("Yine de kaydet").

## 🧭 17B nasıl takip edilecek

17A-1 gibi: asistan bir adımı yazar, kullanıcı okur, "tamam" deyince commit; dal `feat/80-inceleme`, tek PR.
`0018` yalnız yeni tablo eklediği için genişletme/daraltma ayrımı yok: canlıdaki kod tabloya hiç dokunmaz.
Kural (2026-11-25): Sonar'ın her bulgusu birleşmeden önce düzeltilir ya da gerekçesi yazılır; testler
baştan düşük karmaşıklıkla yazılır.

| Adım | İçerik | Durum |
| --- | --- | --- |
| 1 | `0018_reviews.sql` + sqlc sorguları + migration testi | ✅ commit (2026-11-25) |
| 2 | `internal/reviews` servisi: yaz/düzenle (sürümlü), sil, gizle/geri aç (audit) | ✅ commit (2026-11-25) |
| 3 | API: inceleme uçları, admin uçları, `review` rate limit | ✅ commit (2026-11-25) |
| 4 | Arayüz veri katmanı: tipler, istemci, kancalar | ✅ commit (2026-11-25) |
| 5 | Arayüz: İncelemeler bölümü, kendi incelemem, spoiler, admin gizleme | ✅ commit (2026-11-25) |
| 6 | Playwright (axe dahil), dokümanlar, PR | — |

### 17B adım 1 — Şema ve sorgular

- **Amaç:** incelemenin veritabanı tarafı. Bu adımda uç ve ekran değişmez.
- **Parçalar:**
  - `backend/migrations/0018_reviews.sql`: `reviews` tablosu (§ 1). Taslaktan farklar § 1'de, gerekçeleriyle
    dosyanın başında.
  - `backend/sql/queries/reviews.sql`, servisin (adım 2) kullanacağı sorgular:
    - herkese açık liste ve sayısı: gizlenenler yok; yazarın adı `users`'tan, puanı `ratings`'ten;
    - kendi incelemem ve kilitli hâli;
    - ilk yazım: yalnız yayındaki kitaba; aynı anda iki ilk yazımda ikincisi satır döndürmez, servis
      kilitleyip sürüm kuralına göre karar verir;
    - düzenleme: `edited_at` dolar, `version` artar, gizleme kolonlarına dokunmaz;
    - silme;
    - yönetici için kilitli okuma, gizleme ve geri açma.
  - `internal/db`: sqlc'nin ürettiği kod.
  - `internal/database/migrate_0018_integration_test.go`: veritabanı `0017`'ye kadar kurulur, `0018`
    uygulanır. Sınananlar:
    - kullanıcı başına kitapta tek inceleme;
    - metin 1–5000 karakter, gerekçe en çok 500;
    - gizleme kuralı iki yönlü;
    - silmeler: puan silinince inceleme kalır, yönetici silinince gizleme kalır ve gizleyen boşalır,
      kitap ve yazar silinince inceleme gider;
    - `Down` tabloyu kaldırır.
  - `migrate_integration_test.go`: migration testlerinin ortak yardımcısı `schemaDB` (`must`, `try`,
    `count`). 0017 testinin `must`'ı buraya taşındı; Sonar tekrar saymasın.
- **Bu adımda verilen kararlar (asistan):**
  - Gizlemek `version`'ı artırmıyor: sahibinin açık düzenleyicisi yönetici yüzünden 409 almasın.
  - Sorgular bu adımda yalnız sqlc'nin şemaya karşı doğrulamasından geçti (`generate`, `diff`).
    Davranışları servis testleriyle adım 2'de sınanacak (17A-1 adım 1'deki gibi).
- **Sonuç (2026-11-25):**
  - Bütün backend testleri gerçek Postgres + Redis'le yeşil; golangci-lint 0 sorun; sqlc `diff` temiz.
  - Yabancı anahtar index denetimi (`TestEveryForeignKeyHasIndex`) `reviews`'i de kapsıyor ve geçiyor.
  - Testlerin gerçekten yakaladığı bilinçli bozmayla denendi; beşinin beşi kırmızı:
    - `UNIQUE` kaldırılınca;
    - gizleme kuralı spec taslağındaki tek yönlü hâline çevrilince;
    - `hidden_by`'ın `ON DELETE SET NULL`'ı kaldırılınca;
    - `hidden_by` index'i kaldırılınca (FK denetimi);
    - 500 sınırı kaldırılınca.
- **Bakılacak yerler:**
  - [`0018_reviews.sql`](../../backend/migrations/0018_reviews.sql): baştaki açıklama;
  - [`reviews.sql`](../../backend/sql/queries/reviews.sql): her sorgunun üstündeki not;
  - [`migrate_0018_integration_test.go`](../../backend/internal/database/migrate_0018_integration_test.go).

### 17B adım 2 — Servis

- **Amaç:** incelemenin iş kuralları tek yerde. Bu adımda uç ve ekran değişmez.
- **Parçalar:**
  - `backend/internal/reviews/service.go`:
    - `List`, `Mine`: kitap yayında değilse "kitap yok".
    - `Save`: yaz ya da düzenle, tek transaction. Kitap yayında mı → incelemem kilitli okunur → yoksa
      ilk yazım, varsa düzenleme.
    - `Delete`: puanı silmez.
    - `Hide`, `Unhide`: satır kilitli okunur; yazım ve audit aynı transaction'da.
  - `internal/audit`: yeni varlık tipi `review`.
  - `backend/internal/reviews/service_integration_test.go`: gerçek Postgres'le yedi test.
- **Bu adımda verilen kararlar (asistan):**
  - **Sürüm iki yönlü karşılaştırılır.** İnceleme varken sürümsüz yazım 409: başka bir sekme az önce
    yazmıştır. İnceleme yokken sürümlü yazım da 409: başka bir sekme silmiştir. Böylece hiçbir yazım
    haberi olmadan ezilmez ve silinen inceleme sessizce geri gelmez.
  - **Aynı anda iki ilk yazım:** ikincisi `UNIQUE`'te birincinin bitmesini bekler, satır döndürmez;
    servis satırı yeniden okur ve 409 verir. Birincinin metni kalır.
  - **Değişiklik yoksa yazılmaz:** metin ve spoiler bayrağı aynıysa sürüm artmaz, "düzenlendi"
    görünmez. Kaydet'e iki kez basmak düzenleme değildir.
  - **Gizleme ve geri açma tekrarlanınca hiçbir şey yazılmaz:** zaten gizli incelemenin gerekçesi
    değişmez, audit'e ikinci satır düşmez. İki yönetici aynı anda gizlerse de tek satır (kilit).
  - **Audit gerekçeyi saklar:** gizlemede verilen gerekçe; geri açmada satırdan silinen gerekçe. Geri
    açılan incelemenin neden gizlendiğinin tek izi audit.
  - **Silmek için kitabın yayında olması aranmaz:** okur kendi verisini her zaman silebilir (Faz 15'te
    kitaplıktan çıkarmak gibi). Yazmak, düzenlemek, okumak ve listelemek yayında olmayan kitapta 404.
  - **Yönetici gizlemesi kitabın durumuna bakmaz:** yayından kalkmış kitaptaki inceleme de gizlenebilir.
  - **Uzunluk handler'da doğrulanır** (adım 3; yorumlardaki gibi): metin 1–5000, gerekçe 1–500
    karakter. Sınırlar serviste sabit (`MaxBodyLen`, `MaxReasonLen`); veritabanı CHECK'leri son güvence.
- **Sonuç (2026-11-25):**
  - Yedi test, gerçek Postgres'le yeşil:
    - yazım ve düzenleme adımları (Doğrulama 12);
    - yayında olmayan kitap;
    - silme (Doğrulama 13, 14);
    - liste;
    - gizleme (Doğrulama 15);
    - olmayan incelemeyi gizleme;
    - aynı anda iki ilk yazım: birinci yazım commit edilmeden ikincinin onu beklediği
      `pg_blocking_pids` ile görülür, sonra commit edilir.
  - Bütün backend testleri yeşil; golangci-lint 0 sorun.
  - Bilinçli bozmayla denendi; on birin on biri kırmızı (derlenen hâlleriyle; derlenmeyen iki ilk deneme
    sayılmadı):
    - düzenlemede ya da ilk yazımda sürüm denetimi kaldırılınca;
    - yarışı kaybeden ilk yazım 404 alınca;
    - değişmeyen kayıt da yazılınca;
    - gizleme tekrarı yeniden yazınca;
    - geri açma audit'ine boş gerekçe yazılınca;
    - audit hiç yazılmayınca;
    - silmede 0 satır başarı sayılınca;
    - `Save`, `Mine` ya da `List` kitabın yayında olduğuna bakmayınca.
  - **Testsiz kalan tek dal:** ilk yazımla "kitap yayında mı" denetimi arasında kitap yayından kalkarsa
    `Save` "kitap yok" döner. Bu yarışı testte kurmanın güvenilir bir yolu yok (iki sorgu arasındaki
    anlık durum); dal üç satır ve gerekçesi kodda yazılı.
- **Bakılacak yerler:**
  - [`service.go`](../../backend/internal/reviews/service.go): `Save` → `create` / `edit`, `lostInsert`,
    `moderate`;
  - [`service_integration_test.go`](../../backend/internal/reviews/service_integration_test.go).

### 17B adım 3 — API

- **Amaç:** servisi HTTP'ye açmak. Ekran bu adımda değişmez.
- **Uçlar** (§ 2'deki gibi):

  ```text
  GET    /api/books/{id}/reviews          herkese açık, sayfalı (limit, offset)
  GET    /api/books/{id}/review           benim incelemem; yoksa 404 no_review
  PUT    /api/books/{id}/review           {body, has_spoiler, version?}  yeni 201, düzenleme 200
  DELETE /api/books/{id}/review           204
  POST   /api/admin/reviews/{id}/hide     {reason}  200
  POST   /api/admin/reviews/{id}/unhide   gövdesiz  200
  ```

- **Parçalar:**
  - `backend/cmd/api/reviews.go`: handler'lar, DTO'lar, hata haritası.
  - `router.go`: rotalar; `main.go`: servis bağlandı; `middleware_test.go`: iki yeni admin rotası
    korunan rotalar listesinde (401/403 testi).
  - `reviews_test.go`: sahte servisle uç testleri; `reviews_integration_test.go`: gerçek servisle
    uçtan uca.
  - `AUTH_RATE_LIMIT_PER_MIN`'in açıklaması: `config.go`, `.env.example`, DEPLOYMENT.md.
- **Bu adımda verilen kararlar (asistan):**
  - **Cevaplar:**
    - Herkese açık liste: `id`, `author` (`id`, `display_name`), `author_score` (puan yoksa null),
      `body`, `has_spoiler`, `created_at`, `edited_at`. Sürüm ve gizleme alanları yok; e-posta yok.
      `author` yorumlardakiyle aynı biçim.
    - Benim incelemem (yönetici uçları da bunu döner): `id`, `book_id`, `body`, `has_spoiler`,
      `version`, `created_at`, `edited_at`, `hidden` (görünürse null, gizliyse `at` ve `reason`).
      **Gizleyen yöneticinin kimliği cevapta yok**; yalnız veritabanında ve audit'te.
  - **Hata kodları:** incelemem yoksa `404 no_review` (kitap yoksa `404 not_found`; arayüz ikisini
    ayırıp boş düzenleyici gösterir, puandaki `not_rated` gibi). Sürüm uyuşmazsa `409 version_conflict`.
    Yönetici uçlarında olmayan inceleme `404 not_found`.
  - **PUT'un durum kodu istekten bilinir:** sürümsüz istek ancak ilk yazımsa başarılı olur (servis
    inceleme varken sürümsüz yazımı 409 yapar). Bu yüzden sürümsüz başarı 201, sürümlü başarı 200.
  - **Doğrulama:** metin ve gerekçe kırpılır; metin 1–5000, gerekçe 1–500 karakter; sürüm gönderilirse
    en az 1. NUL ve bilinmeyen alan (`user_id` gibi) `readJSON`'da `400 invalid_json`.
  - **İstek sınırı:** yalnız PUT, IP başına ayrı `review` kovası, önce kimlik sonra sınır (parola
    değiştirmedeki sıra). Sınır değeri yeni bir ayar değil, mevcut `AUTH_RATE_LIMIT_PER_MIN` (varsayılan
    dakikada 10); parola kovası da onu kullanıyordu. Adı tarihî kaldı, açıklaması üç yerde güncellendi.
    NGINX'e ayrı bölge eklenmedi: uç zaten kimlik istiyor ve API'deki sınır Redis'te bütün örnekler
    arasında paylaşılıyor.
- **Sonuç (2026-11-25):**
  - Uç testleri yeşil: PUT için 13 senaryo, okuma ve silme 10, yönetici 9; servise gidenler (kullanıcı
    token'dan, kitap adresten, kırpılmış metin); cevap sözleşmeleri; istek sınırı (kimliksiz istek
    kotayı yemez, okuma ve silme sınırsız).
  - Uçtan uca test gerçek Postgres'le yeşil: yaz → sürümsüz ikinci yazım 409 → düzenle → yönetici gizler
    (listede yok) → sahibi düzenler (gizli kalır) → geri açılır (listede) → başkasının silmesi Alice'in
    incelemesine dokunmaz.
  - Bütün backend testleri yeşil; golangci-lint 0 sorun; `scripts/checks.sh range` yalnız yerelde
    `sqlc` kurulu olmadığını söylüyor (CI'da var; bu adımda sorgu değişmedi).
  - Bilinçli bozmayla denendi; on ikinin on ikisi kırmızı (hepsi derlendi):
    - metin kırpılmayınca; metin ya da gerekçe uzunluğu denetlenmeyince; sürüm alt sınırı kalkınca;
      gerekçe zorunlu olmayınca;
    - düzenleme de 201 dönünce; 409 eşlemesi kalkınca; `no_review` yerine `not_found` dönünce;
    - gizleme bilgisi cevaptan düşünce;
    - PUT'tan istek sınırı kalkınca; sınır bütün gruba (okuma, silme) konunca;
    - gizleme rotası admin grubunun dışına çıkınca.
  - Ara düzeltme: handler'a önce `v.text` (NUL denetimi) koymuştum; `readJSON` bunu zaten bütün metin
    alanları için yapıyor, kaldırıldı.
- **Bakılacak yerler:**
  - [`reviews.go`](../../backend/cmd/api/reviews.go): DTO'lar ve `handleSaveReview`;
  - [`router.go`](../../backend/cmd/api/router.go): "İncelemeler (Faz 17B)" bloğu ve admin rotaları;
  - [`reviews_test.go`](../../backend/cmd/api/reviews_test.go),
    [`reviews_integration_test.go`](../../backend/cmd/api/reviews_integration_test.go).

### 17B adım 4 — Arayüz veri katmanı

- **Amaç:** arayüzün incelemeleri okuyup yazabilmesi. Ekran bu adımda değişmez.
- **Parçalar:**
  - `api/types.ts`: `PublicReview` (liste), `Review` (benim incelemem; yönetici uçları da bunu döner),
    `ReviewDraft` (PUT gövdesi; ilk yazımda `version` yok).
  - `api/client.ts`: `reviews`, `myReview`, `saveReview`, `deleteReview`, `hideReview`, `unhideReview`.
  - `api/queries.ts`: iki anahtar: liste `["reviews", kitap, offset]` (yorumlar gibi kendi kökünde) ve
    benim incelemem `["me", kullanıcı, "review", kitap]` (kullanıcıya özel; çıkışta önbelleğin tamamı
    zaten siliniyor).
  - `api/reviewQueries.ts` (yeni): `useReviews`, `useMyReview`, `useSaveReview`, `useDeleteReview`,
    `useModerateReview`. Ayrı dosyada: İncelemeler bölümü tembel yüklenirse kancalar ana pakete girmez.
- **Bu adımda verilen kararlar (asistan):**
  - **"İncelemen yok" bir cevap, hata değil:** `no_review` → `null` (puandaki `not_rated` gibi). Kitap
    yoksa (`not_found`) hata kalır.
  - **Yazımda** cevap benim incelememe doğrudan yazılır, bu kitabın bütün liste sayfaları eskitilir;
    başka kitabın listesi ve puan özeti dokunulmaz. Raf kancalarındaki gibi önce süren okuma iptal
    edilir (eski bir GET yeni hâli ezmesin).
  - **409'da** benim incelemem bir kez yeniden okunur ve hata ancak sonra yayılır: arayüz mesajı güncel
    hâlden kurar. Sessiz yeniden deneme yok: kullanıcının metni ötekini ezmesin. 400 ve 404 hiçbir şeyi
    eskitmez.
  - **Silmede** `no_review` (başka sekme zaten silmiş) başarıyla aynı sonucu verir: benim incelemem
    `null`, liste eskir; hata yine yayılır.
  - **Yönetici işleminde** liste hatada da eskitilir (yorum silmedeki gibi, hayalet satır kalmasın).
    Yönetici kendi incelemesini gizlemiş olabilir: onun "benim incelemem"i de eskitilir.
  - **Kabul edilen gecikme:** listedeki yazar puanı `ratings`'ten gelir; okur puanını değiştirince liste
    hemen eskitilmez (puan kancası `["reviews"]`'a dokunmuyor), 30 sn içinde ya da sayfa yeniden
    açılınca tazelenir. Puan vermek her seferinde incelemeleri yeniden çekmesin diye.
- **Sonuç (2026-11-25):**
  - Kanca testleri (12) ve istemci adres testleri (8 yeni satır) yeşil; bütün arayüz testleri 505/505;
    `typecheck` ve `vite build` temiz.
  - Ara düzeltme: 409 testi ilk hâlinde benim incelememi izleyen bir bileşen kurmuyordu; gözlemcisiz
    kayıt eskitilince yeniden çekilmediği için test bir şey kanıtlamıyordu. Düzenleyicideki gibi kanca
    gözlemciyle kuruldu.
  - Bilinçli bozmayla denendi; on ikinin on ikisi kırmızı:
    - `no_review` `null`'a çevrilmeyince; girişsizde de istek gidince;
    - yazımda benim incelemem yazılmayınca; liste eskimeyince; bütün kitapların listesi eskiyince;
    - 409'da yeniden okunmayınca; her hatada yeniden okununca;
    - silmede `no_review` hata gibi işlenince; her hata "silinmiş" sayılınca;
    - yönetici işleminde hatada liste eskimeyince; yöneticinin kendi incelemesi eskimeyince;
    - `saveReview` PUT yerine POST gönderince.
- **Bakılacak yerler:**
  - [`reviewQueries.ts`](../../frontend/src/api/reviewQueries.ts): kancaların başındaki notlar;
  - [`reviewQueries.test.tsx`](../../frontend/src/api/reviewQueries.test.tsx);
  - [`types.ts`](../../frontend/src/api/types.ts): `PublicReview`, `Review`, `ReviewDraft`.

### 17B adım 5 — Arayüz

- **Amaç:** kitap sayfasında incelemeler (§ 3).
- **Parçalar:**
  - `components/ReviewSection.tsx` (yeni): bölüm, liste ve sayfalama, inceleme kartı (spoiler düğmesi),
    yöneticinin gizleme diyaloğu ve "Geri aç".
  - `components/MyReview.tsx` (yeni): okurun kendi incelemesi; yazma ve düzenleme formu, 409 notu,
    silme onayı.
  - `pages/BookDetail.tsx`: bölüm yorumların üstünde, tembel yüklenir.
  - `ReviewSection.test.tsx` (yeni, 14 test); `BookDetail.test.tsx`: sahte API'ye inceleme uçları eklendi
    (yoksa bölüm hata kutusu çiziyor ve sayfanın "hata yok" testleri kırılıyordu).
- **Ekranda:**
  - Başlık "İncelemeler (toplam)". Girişsiz ziyaretçiye "İnceleme yazmak için giriş yap".
  - Okurun incelemesi yoksa "İnceleme yaz"; varsa "Senin incelemen" kutusu: metin, tarih,
    "düzenlendi", spoiler işareti, Düzenle ve Sil. Gizlenmişse kutuda "Bu inceleme yönetici tarafından
    gizlendi: <gerekçe>. Başkaları göremiyor."
  - Form: metin alanı, karakter sayacı (`/5000`), "Spoiler içeriyor" onay kutusu, Kaydet ve Vazgeç.
  - Silme onayı: "İncelemen silinsin mi? Puanın silinmez."
  - Listedeki kart: yazar, puanı (varsa), zaman, "düzenlendi". Spoiler'lı kartta "Spoiler içeriyor" ve
    `Spoiler'ı göster` düğmesi.
  - Yöneticiye her kartta "gizle"; diyalogda zorunlu gerekçe (`/500`).
- **Bu adımda verilen kararlar (asistan):**
  - **Kendi incelemen listede ikinci kez görünmez:** üstteki kutuda durur. Başlıktaki toplam sunucunun
    sayısıdır, onu da sayar.
  - **Spoiler (kavram 8):** metin açılana kadar DOM'da hiç yok. Düğme `aria-expanded` taşır;
    `aria-controls` yalnız metin varken (olmayan bir kimliği göstermesin). Kendi incelemende metin hep
    açık; yalnız işaretli olduğu yazılır.
  - **409 akışı (raf formlarıyla aynı):** form açık kalır, yazdığın metin korunur. Üstte ne olduğu yazar:
    başka yerde değişmişse şimdiki metin, silinmişse "silinmiş". Düğme "Benimkini kaydet" olur ve güncel
    sürümle (silinmişse sürümsüz, yeni inceleme olarak) gönderir.
  - **Yöneticinin geri açması:** gizlenenler ekranı kapsam dışı (spec § Kapsam dışı). Yine de yanlış
    gizleme aynı ziyarette geri alınabilsin diye gizledikten sonra listenin üstünde "<yazar> tarafından
    yazılan inceleme gizlendi · Geri aç" kalır. Canlı doğrulamanın (Doğrulama 17) "geri aç" adımı da
    böylece arayüzden yapılabilir.
  - **Tembel yükleme:** bölüm ayrı bir parça (`ReviewSection`, 10 kB). Rollup ana paketle bu parçanın
    ortak modüllerini ayrı bir parçaya (`toast-…js`, 87,5 kB) aldı; ana paket 387 kB'tan 300 kB'a indi.
    Toplam aynı, açılışta ikisi de yükleniyor; fark bir istek fazlası.
  - Metin alanının sınırı tarayıcının `maxLength`'i (UTF-16 birimi); sunucu karakter sayar. Emojili
    metinde arayüz sınırı biraz erken keser, tersi olmaz.
- **Sonuç (2026-11-25):**
  - Bileşen testleri 14/14; bütün arayüz testleri 519/519; `typecheck` ve `vite build` temiz.
  - Bilinçli bozmayla denendi; on beşin on beşi kırmızı:
    - spoiler metni hep DOM'da; `aria-expanded` yok; `aria-controls` hep var;
    - kendi incelemen listede de; herkes "gizle" görür; girişsizde de "benim incelemem" sorulur;
    - gerekçe kırpılmaz; gerekçesiz gizlenir; gizledikten sonra "Geri aç" notu çıkmaz;
    - gizleme gerekçesi kutuda gösterilmez; metin kırpılmaz; boş metin gönderilir; ilk yazımda sürüm
      gider; 409 notunda şimdiki metin yok; silme onayı puanı söylemez.
  - Erişilebilirlik (axe) ve klavyeyle kullanım Playwright'ta (adım 6) sınanacak.
- **Bakılacak yerler:**
  - [`ReviewSection.tsx`](../../frontend/src/components/ReviewSection.tsx): `ReviewCard` ve `HideDialog`;
  - [`MyReview.tsx`](../../frontend/src/components/MyReview.tsx): `ReviewEditor` ve 409 notu;
  - [`ReviewSection.test.tsx`](../../frontend/src/components/ReviewSection.test.tsx).

## 🪜 17B — adımlar

### 1. Şema: `0018_reviews.sql`

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
    hidden_reason text NOT NULL DEFAULT '' CHECK (char_length(hidden_reason) <= 500),
    UNIQUE (user_id, book_id),
    CONSTRAINT reviews_hidden_has_reason CHECK ((hidden_at IS NULL) = (hidden_reason = ''))
);
CREATE INDEX reviews_book_created_idx ON reviews (book_id, created_at DESC, id DESC);
CREATE INDEX reviews_hidden_by_idx ON reviews (hidden_by) WHERE hidden_by IS NOT NULL;
```

- **Uygulamada değişen (2026-11-25, 17B adım 1):**
  - Liste index'ine `id DESC` eklendi. Liste kararlı sayfalama için `id`'yi ikinci sıralama anahtarı
    yapıyor; index'te yoksa sıralamanın tamamını veremez (0005'in dersi).
  - Gizleme kuralı iki yönlü oldu: gizliyse gerekçe dolu, görünürse boş. Geri açılan incelemede eski
    gerekçe kalmasın.
  - Gerekçe en çok 500 karakter; moderasyondaki ret gerekçesiyle aynı sınır.

- **2026-11-23 düzeltmesi:** ilk taslakta liste index'i kısmiydi (`WHERE hidden_at IS NULL`). Böyle bir
  index yabancı anahtar denetimine sayılmaz (yorumlarda 0015'te düzeltilen sorunun aynısı), bu yüzden
  tam; gizlenen inceleme az olacağı için liste sorgusu da onu kullanır. `user_id` `UNIQUE (user_id,
  book_id)` ile karşılanıyor.

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
  arayüzden durumu değiştirilen bir okumanın ilerlemesi günlüğe yazılmaz (bayat alanlar 0017'de silinir).
- **0017 ancak 17A-1 doğrulandıktan sonra** merge edilir (CLAUDE.md: daraltma ayrı PR).
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
   "Bugün" yerel tarihtir: İstanbul'da 01:00'de (UTC'de önceki gün) yardımcı o günü verir (birim testi).
5. İlerleme varken ertelemek ve çıkarmak günlüğe "yarım bırakıldı" yazar; ilerleme yokken yazmaz.
   Çıkarılan kitabın geçmişi yeniden eklenince görünür.
6. Yarım bırakılandan "Devam et" sayfayı ve başlangıç tarihini geri getirir, günlük satırı kalmaz;
   "Baştan başla" günlük satırını korur.
7. Transaction ortasında hata olursa ne satır ne günlük değişir (entegrasyon testi).
8. Toplam bilinmeyen okumada "0 sayfa" görünmez; Faz 15'ten kalan okuyorum kaydı "İlerlemeni gir" gösterir.
9. Başkasının günlük kaydına her uç 404; eski `PUT {status}` aynı sonuçları üretir.
10. Canlı (17A-1): demo hesabıyla başla → ilerle → ertele → devam et → bitir; kitap sayfası ve
    Kitaplığım tutarlı. Yayında Deploy'un migration adımı `0016`'dan **önce** şifreli döküm aldı
    (`bookhub-yedek-migration-oncesi-…` artefaktı): bu, ilk gerçek migration öncesi yedek
    ([#100](https://github.com/mtgezgin/BookHub/issues/100) kabul kriteri 3).
11. Canlı (17A-2): 0017 öncesi sayım PR'da; sonrasında kural dışı yazım veritabanında reddedilir
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
