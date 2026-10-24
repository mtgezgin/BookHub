# Öğrenme Notları — Database

Postgres, SQL, sqlc, migration, transaction, index kavramları.
Format ve nasıl çalıştığı: [README.md](README.md).

## İçindekiler

1. [goose ve migration kavramı: "şema için git"](#1-goose-ve-migration-kavramı-şema-için-git)
2. [sqlc nedir: SQL'i sen yaz, Go'yu makine yazsın](#2-sqlc-nedir-sqli-sen-yaz-goyu-makine-yazsın)
3. [Natural key vs surrogate key: ISBN neden PK olmaz?](#3-natural-key-vs-surrogate-key-isbn-neden-pk-olmaz)
4. [DSN (Data Source Name) nedir?](#4-dsn-data-source-name-nedir)
5. [sqlc + parametreli INSERT: DB DEFAULT'u neden devreye girmez?](#5-sqlc--parametreli-insert-db-defaultu-neden-devreye-girmez)
6. [Durum makinesi + denetim kaydı: kural nerede yaşamalı?](#6-durum-makinesi--denetim-kaydı-kural-nerede-yaşamalı)
7. [Kayıp güncelleme (lost update): neden kilit, neden türetilmiş veri?](#7-kayıp-güncelleme-lost-update-neden-kilit-neden-türetilmiş-veri)
8. [EXPLAIN ANALYZE nasıl okunur? (index'in neden kullanılmadığını bulmak)](#8-explain-analyze-nasıl-okunur-indexin-neden-kullanılmadığını-bulmak)
9. [Kilit sırası değişmezi: gerçek bir deadlock (40P01) nasıl doğdu](#9-kilit-sırası-değişmezi-gerçek-bir-deadlock-40p01-nasıl-doğdu)
10. [Satır kilidi türleri: FOR UPDATE, FOR NO KEY UPDATE, FOR KEY SHARE, FOR SHARE](#10-satır-kilidi-türleri-for-update-for-no-key-update-for-key-share-for-share)
11. [SELECT ... FOR UPDATE SKIP LOCKED: beklemek yerine atlamak](#11-select--for-update-skip-locked-beklemek-yerine-atlamak)
12. [COMMIT'ten sonra başarısız olabilecek adım bırakma](#12-committen-sonra-başarısız-olabilecek-adım-bırakma)
13. [Genişlet ve daralt: bir kolonu silmek neden iki yayın ister?](#13-genişlet-ve-daralt-bir-kolonu-silmek-neden-iki-yayın-ister)
14. [Eser ve baskı modeli: kısmi benzersiz index ile tek varsayılan](#14-eser-ve-baskı-modeli-kısmi-benzersiz-index-ile-tek-varsayılan)

---

## 1. goose ve migration kavramı: "şema için git"

**🗓️ Bağlam:** Faz 2 adım 3'te ilk migration'ı (`0001_books_genres.sql`) yazıp `goose up` /
`goose down` roundtrip'ini gördükten sonra "goose nedir, anlatabilir misin?" diye sordum.
Aynı konuşmada tablolara pgAdmin4 ile bakmak isteyince "GUI'den şema değiştirme" uyarısı da
gündeme geldi.

**⚡ Kısa cevap:** goose, şema değişikliklerini **numaralı + geri alınabilir SQL dosyaları**
olarak yöneten küçük bir CLI (migration aracı). Zihinsel model: **şema için git** — her dosya
bir commit, `goose_db_version` tablosu HEAD, `up`/`down` ileri/geri gitmek.

**📖 Açıklama:**

Çözdüğü problem — şemayı elle (psql'de `CREATE/ALTER`) yönetirsen: tarih yok (taze
makinede/CI'da/prod'da aynı şemayı birebir kuramazsın), ortamlar sessizce ayrışır (**schema
drift**), "son değişikliği geri al" düğmesi yok.

| git | goose |
|---|---|
| commit | migration dosyası (`0001_books_genres.sql`) |
| commit history | `migrations/` klasörü, numara sırası |
| HEAD (neredeyim?) | DB içindeki `goose_db_version` tablosu |
| checkout ileri/geri | `goose up` / `goose down` |
| `git log` | `goose status` |

Mekanik: dosyaları numara sırasına dizer → `goose_db_version`'a bakıp uygulanmamışları bulur
→ onların `-- +goose Up` bölümünü çalıştırır ve deftere işler. `down` = son uygulananın
`-- +goose Down` bölümü + kaydı düşmek. Tek dosyada iki yön:

```sql
-- +goose Up
CREATE TABLE books (...);
-- +goose Down
DROP TABLE books;
```

Komutlar (hep `backend/` içinden):

```bash
goose -dir migrations postgres "$DATABASE_URL" up      # uygula
goose -dir migrations postgres "$DATABASE_URL" status  # neredeyiz?
goose -dir migrations postgres "$DATABASE_URL" down    # son migration'ı geri al (SADECE dev!)
```

**TS dünyasından karşılığı:** Prisma Migrate / knex / TypeORM migrations. Kritik fark:
Prisma'da `schema.prisma` yazarsın, SQL'i **o üretir**; goose'da SQL'i **sen yazarsın**, goose
sadece sıra + kayıt tutar (bizde bilinçli tercih — SQL'i görmek dersin kendisi,
PROJECT_BRIEF §7.1). GORM `AutoMigrate` tarzı "struct'a bakıp şemayı kendiliğinden değiştiren"
sihirden aynı sebeple uzağız: migration dosyası PR diff'inde görünür, review edilir, prod'a
aynen o gider. (Dürüst not: Prisma'nın ürettiği SQL de migration dosyasına yazılır ve review
edilebilir — fark "kim yazıyor", disiplin değil.)

Altın kurallar:
- **Uygulanmış migration'ı asla düzenleme** — değişiklik = **yeni** dosya (`0002_...`).
  Yoksa defter ile gerçek şema birbirini tutmaz.
- **`down` sadece dev konforu** — prod'da geri alma da ileri giden yeni bir migration'la yapılır.
- **Şema değişikliğinin tek kapısı migration** — pgAdmin/GUI sadece bakmak ve sorgu denemek
  için; sağ-tık ile kolon eklersen migration'lara yansımaz → schema drift'i kendi elinle
  yaratırsın (taze klonda `goose up` çalıştıran senin GUI değişikliğini alamaz).

**🔗 İlgili:**
- [backend/migrations/0001_books_genres.sql](../../backend/migrations/0001_books_genres.sql) — ilk migration (satır satır açıklamalı)
- [docs/phases/phase-02-postgres-books-genres.md](../phases/phase-02-postgres-books-genres.md) adım 3 · [PROJECT_BRIEF](../PROJECT_BRIEF.md) §7.1/§7.2 (goose neden; prod'a down çekme), §18.3 (tip/kısıt/index kararları)
- [infra-and-devops.md](infra-and-devops.md) (gelince: Docker Compose Postgres — migration'ın koştuğu dev DB)

---

## 2. sqlc nedir: SQL'i sen yaz, Go'yu makine yazsın

**🗓️ Bağlam:** Faz 2 adım 4'te sqlc kurulup ilk `GetBook` sorgusu üretildikten sonra "sqlc
nedir, hiç duymadım; benzer tool'lara göre avantajı dezavantajı nedir?" diye sordum. Prisma'yı
da pek kullanmadığım için ORM dünyasıyla kıyası sıfırdan istedim.

**⚡ Kısa cevap:** sqlc bir **kod üreteci** (kütüphane DEĞİL, runtime'da yok): `migrations/`
şeması + `sql/queries/`'deki el yazması SQL'den **tip-güvenli Go fonksiyonları** üretir.
ORM'in tersi — ORM sorguyu çalışma anında üretir, sqlc kodu derlemeden önce üretir; SQL'i
sen yazarsın, `rows.Scan` boilerplate'ini makine yazar.

**📖 Açıklama:**

DB erişim yaklaşımları spektrumu:

| | SQL'i kim yazar? | Hata ne zaman yakalanır? | Runtime sihri | Dinamik sorgu |
|---|---|---|---|---|
| çıplak pgx | sen | **runtime** 💥 | yok | elle |
| ORM (GORM ≈ TypeORM/Sequelize) | kütüphane | çoğu runtime | **çok** (N+1 tuzağı) | güçlü |
| query builder (squirrel ≈ knex) | yarı yarıya | runtime | az | güçlü |
| **sqlc** | **sen** | **generate/derleme** ✅ | **yok** | zayıf |

Akış: `migrations/*.sql` (şema) + `sql/queries/*.sql` (sorgular) → `sqlc generate` →
`internal/db/*.go`. Üretilen kod, elle yazacağın pgx kodunun aynısı — okunabilir, sihirsiz.

Faydalar (canlı gördüklerim):
- **Hatalar üretim anına çekilir:** kolon typo'su → `sqlc generate` hatası; şema değişti →
  regenerate → derleme hatası etkilenen her yeri gösterir. (`SELECT *`'ı bile üretim anında
  açık kolon listesine çevirir.)
- **SQL öğrenirsin** — JOIN/index/`RETURNING` gizlenmez (BookHub'ın varlık sebebi, §7.1).
- **Boilerplate yok:** 12 kolonluk `Scan`'i, `CreateBookParams` struct'ını makine yazar.
- **Injection kapısı yok:** her şey parametreli (`$1`), string birleştirme deseni oluşmuyor.

Dürüst dezavantajlar:
- SQL bilmek **şart** (ORM'in "SQL'siz başla" konforu yok — bizde bu bir özellik).
- **Dinamik/koşullu sorgularda hantal** (5 opsiyonel filtreli arama → sorgu varyantları ya da
  `COALESCE` cambazlığı; ORM/builder'ın gerçekten parladığı yer).
- `sqlc generate` adımını unutmamak gerekir; üretilen dosyalar diff kalabalığı yapar.

JS dünyasında en yakın akraba Prisma değil **pgtyped** (SQL yaz → TS tipi üret); Prisma
şema-DSL'den hem SQL'i hem client'ı kendi üretir — "SQL'i sen yaz" ilkesi orada yok.

**Devam sorusu: hangi sorgunun hangisi olduğunu nasıl anlıyor?** Sorgunun üstündeki
`-- name: GetApprovedBook :one` satırı normal yorum değil, **direktif**: `name:` sonrası üretilecek
Go fonksiyonunun adı, `:one/:many/:exec/:execrows` ise dönüş şekli (`(Book, error)` /
`([]Book, error)` / `error` / `(int64, error)`). Bu satır yoksa sqlc sorguyu derlemez.
Tipleri iki kaynağı çaprazlayarak bulur: **parametreler** ($1'i şemadaki kolonla eşler —
`id bigint` → `int64`; çok parametrede `XxxParams` struct'ı üretir) ve **dönüş kolonları**
(`SELECT *` = tam tablo → model struct'ı `Book`; alt küme/JOIN → sorguya özel `XxxRow`
struct'ı). `-- name:` şekli sen bildirirsin, SQL + şema tipleri belirler; tutarsızlık
`sqlc generate`'te patlar.

**🔗 İlgili:**
- [backend/sqlc.yaml](../../backend/sqlc.yaml) (açıklamalı config) · [backend/sql/queries/books.sql](../../backend/sql/queries/books.sql) (sorgu direktifleri: `:one/:many/:exec`)
- [backend/internal/db/books.sql.go](../../backend/internal/db/books.sql.go) — üretilen Scan kodu (GO_CONVENTIONS §6: "en az bir kez oku")
- [docs/WHY_GO_VS_TS.md](../WHY_GO_VS_TS.md) §6 (sqlc+pgx vs Prisma/ORM — bu fazın sonunda dolacak)
- Yukarıda [Girdi 1](#1-goose-ve-migration-kavramı-şema-için-git) (şemanın tek gerçeği: migrations/)

---

## 3. Natural key vs surrogate key: ISBN neden PK olmaz?

**🗓️ Bağlam:** Faz 2'de üretilen `models.go`'daki `Book` struct'ını incelerken "id olarak
ISBN tutulmalı mıydı? Projede böyle bir şey var mı? ISBN'in özel formatı ekstra işlem
gerektirir mi?" diye sordum. (Brief'te ISBN sadece §12.6'da — MVP-sonrası "Book Import"
özelliğinde — geçiyor; kitap modelinde yok.)

**⚡ Kısa cevap:** Hayır. ISBN bir **natural key** (gerçek dünyadan gelen kimlik); PK olarak
**surrogate key** (DB'nin ürettiği anlamsız `bigint`) kullanılır. Endüstri deseni: yapay PK +
doğal kimlikler ayrı `UNIQUE` kolonlar (bizde `slug`; ISBN gelirse o da öyle olur).

**📖 Açıklama:**

ISBN'i PK yapmak dört yerden batar:

1. **Her kitapta yok** (1970 öncesi, öz-yayın, elle giriş) — PK her satırda var olmak
   zorunda, NULL PK diye bir şey yok.
2. **Yanlış granülerlik:** ISBN **baskıyı** (edition) tanımlar, eseri (work) değil — aynı
   kitabın ciltli/e-kitap baskıları farklı ISBN taşır. Bizim domain eser seviyesinde.
3. **PK asla değişmemeli:** ISBN typo'sunu düzeltmek = PK UPDATE = ona bağlı tüm FK'ların
   (`book_genres`, ileride `comments`/`ratings`) peşinden sürüklenmesi. Yapay `bigint`
   gerçek-dünya hatalarından etkilenmez.
4. **Maliyet:** ISBN sayı değil **yapılı string** (ISBN-10/13 ikiliği, baştaki sıfırlar,
   check digit) — her FK/index'te 8-byte `bigint` yerine 13+ karakter taşırsın.

İleride §12.6 gelirse: yeni migration ile `isbn text UNIQUE` (nullable) eklenir —
**Postgres'te UNIQUE kolonda birden çok NULL yaşayabilir** (NULL ≠ NULL sayılır), yani
"ISBN'siz kitaplar serbest, mükerrer ISBN yasak" bedavaya gelir. Format doğrulaması
(ISBN-13 checksum) ve "tiresiz ISBN-13'e normalize" kuralı uygulama katmanına ait.
(§14.5: ihtiyaç gelmeden ekleme.)

**🔗 İlgili:**
- [backend/migrations/0001_books_genres.sql](../../backend/migrations/0001_books_genres.sql) — `id bigint GENERATED ALWAYS AS IDENTITY` + `slug UNIQUE` (yapay PK + doğal kimlik deseni)
- [docs/PROJECT_BRIEF.md](../PROJECT_BRIEF.md) §6.2 (kitap modeli — ISBN yok), §12.6 (ISBN lookup — MVP sonrası), §14.5/§18.3

---

## 4. DSN (Data Source Name) nedir?

**🗓️ Bağlam:** Faz 2 adım 5'te `internal/database/database.go`'yu okurken yorumlarda geçen
"DSN" terimini görüp "DSN nedir?" diye sordum.

**⚡ Kısa cevap:** **Data Source Name** — bir veritabanına bağlanmak için gereken TÜM
bilgileri (kullanıcı, şifre, host, port, DB adı, seçenekler) tek string'de toplayan format.
"Connection string" ile eş anlamlı; `DATABASE_URL` env değişkeninin *içindeki değer* bir DSN.

**📖 Açıklama:**

```
postgres://bookhub:bookhub_dev@localhost:5432/bookhub?sslmode=disable
└─şema──┘ └─kullanıcı─┴─şifre──┘ └──host──┴port┘└─DB──┘ └───seçenekler───┘
```

- **İki format** (pgx ikisini de kabul eder): URL biçimi (`postgres://...`) ve
  anahtar=değer biçimi (`host=... port=... user=...`). URL biçimi tek parça taşınır ve
  12-factor `DATABASE_URL` konvansiyonunun beklediği biçimdir.
- **Tarihçe:** terim 1990'ların ODBC döneminden; o zaman OS'a kayıtlı isimlerdi, bugün
  kendini tarif eden string'lere dönüştü, isim kaldı.
- **Ayrım:** `DATABASE_URL` = zarfın üstündeki isim (env değişkeni); DSN = zarfın içindeki
  mektup (bağlantı tarifi). `pgxpool.New(ctx, dsn)` bu string'i parse eder —
  "havuz yapılandırılamadı" hatası bu parse'ın patladığı yerdir.
- **JS paraleli birebir:** `pg`, Prisma, TypeORM da aynı `postgres://` DSN'ini alır —
  dil-bağımsız standart.

**🔗 İlgili:**
- [.env.example](../../.env.example) — dev DSN'imiz ve format açıklaması
- [backend/internal/database/database.go](../../backend/internal/database/database.go) — DSN'i tüketen kod (pgxpool + fail-fast Ping)
- [security-and-auth.md](security-and-auth.md) Girdi 1 — dev DSN'i repoya yazmak neden güvenli, prod DSN nerede yaşar

---

## 5. sqlc + parametreli INSERT: DB DEFAULT'u neden devreye girmez?

**🗓️ Bağlam:** `POST /api/admin/books` ucunu ilk kez canlı denerken `language` alanını
göndermedim. Migration'da `language text NOT NULL DEFAULT 'tr'` yazdığı için `"tr"` bekliyordum;
cevapta **`"language":""`** çıktı.

**⚡ Kısa cevap:** DB'nin `DEFAULT` değeri yalnızca kolon INSERT'te **hiç anılmazsa** çalışır.
sqlc her parametreyi **her zaman açıkça** yazdığı için (`INSERT INTO books (..., language, ...)
VALUES (..., $8, ...)`), Go'daki boş string DB'ye gider ve Postgres "değer verilmiş" sayar.
Yani parametreli INSERT'te **varsayılanı uygulama katmanı düşünmek zorundadır.**

**📖 Açıklama:**

Postgres'in kuralı basit ama sezgiye ters geliyor:

```sql
-- DEFAULT ÇALIŞIR: kolon listede yok
INSERT INTO books (title, slug, author, status) VALUES (...);        → language = 'tr'

-- DEFAULT ÇALIŞMAZ: kolon listede var, değeri boş string
INSERT INTO books (title, slug, author, language, status)
VALUES (..., '', ...);                                                → language = ''
```

`''` bir *yokluk* değil, geçerli bir değerdir. `NULL` gönderilse `NOT NULL` reddederdi; boş string
ise sessizce kabul edilir. Bu, aynı ailedeki başka bir tuzağın kardeşi: **`NOT NULL` boş string'i
engellemez** — o yüzden zorunlu alan kontrolünü de uygulama yapıyor.

Neden sqlc böyle davranıyor: üretilen kod statiktir, "bu alan gönderilmedi mi?" diye
çalışma-zamanı dallanması yapmaz — tip güvenliğinin bedeli budur (bkz. [Girdi 2](#2-sqlc-nedir-sqli-sen-yaz-goyu-makine-yazsın)
"dinamik sorgularda hantal").

**Üç çözüm yolu, biz 1'i seçtik:**

| Yol | Nasıl | Değerlendirme |
|---|---|---|
| 1. **Uygulamada varsayılan** | Handler `Language == "" → "tr"` | ✅ Seçtiğimiz. Görünür, test edilebilir, tek yerde (`varsayilanDil`) |
| 2. SQL'de `COALESCE` | `COALESCE(NULLIF($8,''), 'tr')` | Varsayılan SQL'de kalır ama sorgu okunaksızlaşır |
| 3. `sqlc.narg` + `COALESCE` | Parametreyi nullable yap, SQL'de `COALESCE($8, 'tr')` | Varsayılan SQL'de; Go tarafı `*string`. ⚠️ Nullable parametre **tek başına** DEFAULT'u tetiklemez: açıkça gönderilen `NULL` de bir değerdir → `NOT NULL` ihlali. `COALESCE` şart |

**Genel ders:** DB'deki `DEFAULT`, elle yazılmış INSERT'ler ve migration'lar için bir güvenlik ağı;
uygulama yolunda ona güvenme. Varsayılanın **iki yerde** (DB + uygulama) tanımlı olması da bir tür
borç — biri değişip diğeri unutulursa sessizce ayrışırlar. Bizim durumda ikisi de `'tr'`.

**🔗 İlgili:**
- [backend/cmd/api/books.go](../../backend/cmd/api/books.go) — `varsayilanDil` sabiti ve `normalizeAndValidate`
- [backend/migrations/0001_books_genres.sql](../../backend/migrations/0001_books_genres.sql) — `language text NOT NULL DEFAULT 'tr'`
- Yukarıda [Girdi 2](#2-sqlc-nedir-sqli-sen-yaz-goyu-makine-yazsın) (sqlc'nin dinamik sorgu zayıflığı)
- [go-language.md](go-language.md) Girdi 8 (zero value — boş string neden "yokluk" değil)

---

## 6. Durum makinesi + denetim kaydı: kural nerede yaşamalı?

**🗓️ Bağlam:** Faz 4'te öneri/onay akışını kurarken. İki admin'in aynı öneriyi aynı anda
onaylayıp reddetmesini test ettik ve kilidi kaldırınca ne olduğunu ölçtük.

**⚡ Kısa cevap:** Statü geçiş kuralı **veri** olarak (bir tablo) servis katmanında; geçişin
kendisi **kilitli okuma + güncelleme + denetim kaydı** olarak tek transaction'da; denetim
kaydının değiştirilemezliği ise **DB trigger'ında**. Her kural, atlatılması en zor olan katmanda.

**📖 Açıklama:**

| Kural | Nerede | Neden orada |
|---|---|---|
| "PENDING → APPROVED olur, REJECTED → APPROVED olmaz" | `moderation.Next` (Go, saf fonksiyon) | iş kuralı; DB'siz tablo testiyle 16 kombinasyonun hepsi doğrulanır |
| "iki admin aynı anda karar veremez" | `SELECT ... FOR UPDATE` (tx içinde) | yarış ancak DB kilidiyle çözülür; Go'daki bir mutex çok-instance'ta işe yaramaz |
| "karar varsa kaydı da var" | aynı transaction | biri yazılıp öteki yazılamazsa ikisi de geri alınır |
| "kayıt değiştirilemez/silinemez" | trigger (UPDATE/DELETE + TRUNCATE) | uygulamadaki bir bug ya da iz silmeye çalışan biri de DB'den geçmek zorunda |
| "red gerekçesi yalnız REJECTED'da" | `CHECK` kısıtı | tutarsız satır hiçbir kod yolundan yazılamaz |

**Kilidin ölçülen değeri (mutation):** `FOR UPDATE` kaldırıldığında 5 turun 5'inde de iki admin
"kazandı" ve denetim kaydında **çelişen iki karar** kaldı: `book.reject` VE `book.approve`.
Kilitle, ikinci admin birincinin commit'ini bekliyor, kitabı artık `APPROVED` görüyor ve 409 alıyor.

**İki küçük tuzak:**
- Satır trigger'ı (`FOR EACH ROW`) **TRUNCATE'te çalışmaz** → ayrı `FOR EACH STATEMENT` trigger'ı.
- `actor_id`'ye FK + `ON DELETE SET NULL` koymak, append-only kuralıyla çakışır: SET NULL bir
  UPDATE'tir → kullanıcı silmek imkânsızlaşır. Denetim kaydı zaten aktörden uzun yaşamalı; FK yok.

**🔗 İlgili:**
- [backend/migrations/0003_moderation.sql](../../backend/migrations/0003_moderation.sql)
- [backend/internal/moderation/](../../backend/internal/moderation/) — `transitions.go`, `service.go`
- [security-and-auth.md](security-and-auth.md) — Girdi 7 (aynı "check-then-act" yarışının token hâli)

---

## 7. Kayıp güncelleme (lost update): neden kilit, neden türetilmiş veri?

**🗓️ Bağlam:** Faz 5'te puan ortalamasını yazarken — brief'in "#1 doğruluk dersi". Kilidi
kaldırıp 25 eşzamanlı oyla ölçtük.

**⚡ Kısa cevap:** İki transaction aynı anda "oku → hesapla → yaz" yaparsa, ikisi de diğerinin
**henüz commit edilmemiş** değişikliğini göremez; sonra yazan öncekini ezer. Çözüm: kaynağı
(`ratings`) tek doğruluk kaynağı tut, agregatı her seferinde oradan SQL ile türet ve bu işlemi
kitap satırını **kilitleyerek** sıraya sok.

**📖 Açıklama:**

Postgres'in varsayılan izolasyonu **READ COMMITTED**: her deyim, başladığı andaki commit edilmiş
veriyi görür.

```
T1: INSERT oy(Ali, 8)            T2: INSERT oy(Ayşe, 5)
T1: AVG/COUNT → (8, 1)           T2: AVG/COUNT → (5, 1)   ← T1'in oyu commit edilmedi, görünmez
T1: UPDATE books SET (8, 1)      T2: UPDATE books ... (T1'in satır kilidini bekler)
T1: COMMIT                       T2: ... sonra (5, 1) YAZAR  ← T1'in oyu istatistikten silindi
```

Sonuç: `ratings` tablosunda 2 oy var, `books.rating_count` = 1. Ölçtüğümüz: 25 oyda 16–19.

**Düzeltme:** transaction'ın EN BAŞINDA `SELECT ... FROM books WHERE id=$1 FOR UPDATE`. İkinci
işlem kilidi bekler; birinci commit edince, ikincinin SONRAKİ deyimleri yeni bir anlık görüntüyle
çalışır ve birinci oyu görür. Test: 25/25, her seferinde.

**Neden "sayacı +1 yap" değil de "baştan hesapla":** `rating_count = rating_count + 1` tek başına
atomik olsa da ortalama için ayrıca toplam gerekir ve bir hata/kaçan güncelleme sonsuza dek kalır.
Kaynaktan yeniden hesaplamak **kendini düzeltir**: bir sonraki yazım doğru değeri üretir.

**Neden `-race` bunu yakalamadı:** race detector Go belleğindeki paylaşılan değişkenlere bakar.
Burada paylaşılan şey Postgres'teki bir satırdı; iki goroutine hiçbir Go değişkenini paylaşmadı.
Veri yarışını yalnızca eşzamanlı entegrasyon testi (ve kilidi silip düşmesini görmek) kanıtlar.

**Bu projede aynı desenin dört yüzü:**

| Faz | Yarış | Çözüm |
|---|---|---|
| 2 | eşzamanlı tür değiştirme → iki listenin birleşimi | `FOR UPDATE` |
| 3 | aynı refresh token'la iki yenileme | tek atımlık `DELETE ... RETURNING` |
| 4 | iki admin aynı öneriye onay + red | `FOR UPDATE` |
| 5 | eşzamanlı puanlar → kayıp sayım | `FOR UPDATE` + kaynaktan yeniden hesap |

**🔗 İlgili:**
- [backend/sql/queries/ratings.sql](../../backend/sql/queries/ratings.sql) — `LockApprovedBook`, `UpsertRating`, `RefreshBookRatingStats`
- [backend/internal/ratings/service.go](../../backend/internal/ratings/service.go)
- [docs/WHY_GO_VS_TS.md](../WHY_GO_VS_TS.md) §10

---

## 8. EXPLAIN ANALYZE nasıl okunur? (index'in neden kullanılmadığını bulmak)

**🗓️ Bağlam:** Faz 6'da top-rated ucunu ölçerken; 20 bin kitaplık sentetik veriyle
(`backend/scripts/perf_seed.sql`) sorgu planlarına baktık ve Faz 2'den beri var olan bir index'in
hiç kullanılmadığını keşfettik.

**⚡ Kısa cevap:** `EXPLAIN ANALYZE` sorguyu **gerçekten çalıştırır** ve her adımın gerçek süresini,
satır sayısını ve hangi yolu (Seq Scan / Index Scan / Sort) seçtiğini gösterir. "Yavaş" hissine
değil bu çıktıya bakarak karar verirsin.

**📖 Açıklama:**

Okuma sırası: **içten dışa, alttan üste**. En içteki satır veriyi okur, üsttekiler onu işler.

```
Limit (actual time=6.46..6.46 rows=20)
  ->  Sort  Sort Key: created_at DESC, id DESC   Sort Method: top-N heapsort
        ->  Seq Scan on books  (rows=18000)        ← 18 bin satırı baştan sona okudu
Execution Time: 6.511 ms
```

Burada alarm: tabloda `(created_at DESC) WHERE status='APPROVED'` index'i VAR ama plan **Seq Scan**
diyor. Neden? Sorgu `ORDER BY created_at DESC, id DESC` — ikinci anahtar `id` index'te yok, index
sıralamanın tamamını veremiyor. Planlayıcı "zaten hepsini sıralayacaksam index'le uğraşmayayım" dedi.

```
Limit ->  Index Scan using books_approved_recent_idx   (rows=20)   ← yalnız 20 satır
Execution Time: 0.058 ms
```

Index `(created_at DESC, id DESC)` olunca: yalnızca gereken 20 satır, ~100 kat hızlı.

**Sözlük:**

| Terim | Anlamı |
|---|---|
| Seq Scan | tablonun tamamını oku (küçük tabloda ya da satırların çoğu lazımken doğru seçim) |
| Index Scan | index'ten git, yalnız gereken satırları oku |
| top-N heapsort | LIMIT'li sıralama: hepsini değil en iyi N'i tutar (ucuz ama yine tam okuma ister) |
| Hash Join | küçük tarafı belleğe hash'le, büyüğü tara (bizim C sorgusu: 209 bin oy × kitaplar = 43 ms) |
| actual time=a..b | ilk satırın ve son satırın hazır olduğu an (ms) |
| rows / loops | gerçek satır sayısı / o adımın kaç kez çalıştığı (paralel işçilerde loops=2) |

**Üç kural:**
1. Index'i yalnız WHERE'e göre değil, **WHERE + ORDER BY'ın tamamına** göre tasarla.
2. Küçük tabloda Seq Scan normaldir — ölçmek için gerçekçi hacimde veri üret (dev DB'de 3 kitapla
   her plan aynı görünür).
3. Önce index/sorgu, SONRA cache: C'yi 209 bin satır yerine 20 bin kitap agregatından hesaplamak
   43 ms'yi 5.7 ms'ye indirdi — Redis'e hiç gerek kalmadan.

**🔗 İlgili:**
- [backend/migrations/0005_listing_indexes.sql](../../backend/migrations/0005_listing_indexes.sql)
- [backend/scripts/perf_seed.sql](../../backend/scripts/perf_seed.sql)
- [docs/WHY_GO_VS_TS.md](../WHY_GO_VS_TS.md) §12

## 9. Kilit sırası değişmezi: gerçek bir deadlock (40P01) nasıl doğdu

**🗓️ Bağlam:** Faz 13'te token ailelerini yazarken. İlk tasarımda rotasyon önce token satırını
işaretliyor, sonra aileyi kilitliyordu; iptal ise tam tersini yapıyordu. Test gerçek Postgres'te
`40P01` verdi ve "deadlock neden oldu, nasıl kural hâline getiririz?" sorusu buradan çıktı.

**⚡ Kısa cevap:** Deadlock, iki transaction'ın **birbirinin tuttuğu kilidi beklemesidir**. Çaresi
akıllı bir yeniden deneme değil, **tek bir global sıra**: aynı veriye dokunan herkes kilitleri
AYNI sırayla alır. Bizde bu sıra `users → refresh_token_families → refresh_tokens`.

**📖 Açıklama:**

Somut senaryo (kuralı koymadan önceki hâli):

```
T1 (rotasyon)                          T2 (çıkış/iptal)
UPDATE refresh_tokens  … used_at  ⟵ token satırını kilitledi
                                       UPDATE refresh_token_families … revoked_at  ⟵ aileyi kilitledi
SELECT … families … FOR UPDATE  ⟶ T2'yi bekler
                                       DELETE FROM refresh_tokens WHERE family_id=…  ⟶ T1'i bekler
                                       ▶ Postgres birini seçip öldürür: ERROR 40P01 deadlock detected
```

Postgres bu durumu **tespit eder** (`deadlock_timeout`, varsayılan 1 sn sonra grafiğe bakar) ve
kurbanlardan birini `40P01` ile iptal eder. Yani veri bozulmaz — ama kullanıcı rastgele bir hata
alır ve sorunun kaynağı zamanlamaya bağlı olduğu için "bazen oluyor" diye kaydedilir.

**Kural:** *bir ağaç varsa her zaman kökten yaprağa kilitle.* Bizde ağaç
`users → families → tokens`; Faz 5'teki `0007` migration'ı da aynı deseni kullanıyor (trigger
önce kitabı kilitler, sonra puanları). Kuralı koyduktan sonra rotasyon şöyle: aileyi `FOR UPDATE`
kilitle → **sonra** token'ı işaretle. Aynı ailedeki her rotasyon/iptal/çıkış artık o tek satırda
sıraya giriyor; READ COMMITTED'da kilidi bekleyen işlem uyandığında satırın **en güncel** hâlini
okur, yani az önce iptal edilmiş bir aileyi "canlı" sanamaz.

**Bir incelik — `ORDER BY` ile kilitleme.** Toplu bir `UPDATE … WHERE user_id = $1` satırları
**rastgele** sırada kilitler; iki işlem aynı kümeyi ters sırada kilitlerse yine deadlock. Bu
yüzden parola değişimi önce `SELECT id … ORDER BY id FOR UPDATE` yapıyor: sonuç kümesi
kullanılmıyor, **tek amacı kilitlerin sırasını sabitlemek**.

**Nasıl deterministik test edilir (uyku yok):** testin kendi transaction'ı ayrı bir bağlantıda
açılır, üretim kodu bir goroutine'de koşar ve `pg_blocking_pids()` ile "gerçekten bekliyor mu"
diye **yoklanır**; her bekleyiş bir context ile sınırlanır. Sonra test tx'i commit/rollback edilir
ve sonuç doğrulanır. Kilit sırasını bozan mutasyon testi `40P01` ile kırar — kanıt böyle üretilir.

**🔗 İlgili:**
- [#7 Kayıp güncelleme](#7-kayıp-güncelleme-lost-update-neden-kilit-neden-türetilmiş-veri) — kilitle ilk tanışma
- [#10 Satır kilidi türleri](#10-satır-kilidi-türleri-for-update-for-no-key-update-for-key-share-for-share)
- [backend/sql/queries/users.sql](../../backend/sql/queries/users.sql) · [backend/internal/auth/repository_integration_test.go](../../backend/internal/auth/repository_integration_test.go)
- [docs/phases/phase-13-session-hardening.md](../phases/phase-13-session-hardening.md)

## 10. Satır kilidi türleri: FOR UPDATE, FOR NO KEY UPDATE, FOR KEY SHARE, FOR SHARE

**🗓️ Bağlam:** Faz 13'te parola değiştirme akışını yazarken. `users` satırını `FOR UPDATE` ile
kilitlemek "en güvenlisi" gibi görünüyordu ama test `40P01` verdi — üstelik ortada ikinci bir
`users` yazıcısı yoktu. "Kim bu kilidi alıyor?" sorusunun cevabı **foreign key** çıktı.

**⚡ Kısa cevap:** Postgres'te dört farklı satır kilidi var ve **daha zayıf olan daha çok
eşzamanlılık** demek. Bir FK'yi INSERT eden transaction, hedef satırda kendiliğinden
`FOR KEY SHARE` alır; `FOR UPDATE` onunla çakıştığı için deadlock'a davetiye çıkarır,
`FOR NO KEY UPDATE` çakışmaz.

**📖 Açıklama:**

Güçten zayıfa ve çakışma tablosu (Postgres 16, "Row-Level Locks"):

| Kilit | Ne demek | Çakıştıkları |
|---|---|---|
| `FOR UPDATE` | "bu satırı silebilirim / anahtarını değiştirebilirim" | hepsi |
| `FOR NO KEY UPDATE` | "yalnız anahtar OLMAYAN kolonları değiştireceğim" | `FOR UPDATE`, `FOR NO KEY UPDATE`, `FOR SHARE` |
| `FOR SHARE` | "okuyacağım, kimse değiştirmesin" | `FOR UPDATE`, `FOR NO KEY UPDATE` |
| `FOR KEY SHARE` | "anahtarı sabit kalsın" (FK'lerin aldığı kilit) | yalnız `FOR UPDATE` |

BookHub'daki üç somut kullanım:

1. **`LockRefreshFamily` → `FOR UPDATE`.** Aile satırı gerçekten silinebiliyor/iptal edilebiliyor;
   burada en güçlü kilit doğru olanı — aynı ailedeki rotasyon, reuse iptali ve çıkış bu satırda
   sıraya girsin istiyoruz.
2. **`LockUserPasswordForChange` → `FOR NO KEY UPDATE`.** Parola değişimi `users.password_hash`'i
   yazar; `password_hash` bir anahtar değil. Neden `FOR UPDATE` olmasın: uçuştaki bir rotasyon
   ailesinin kilidini tutarken halef token'ı INSERT ediyor ve `refresh_tokens.user_id` FK'si için
   `users` satırında **`FOR KEY SHARE`** alıyor. `FOR UPDATE` bununla çakışır → biz `users`'ı tutup
   ailesini beklerken o bizim `users` kilidimizi bekler → **`40P01`**. `FOR NO KEY UPDATE`
   `FOR KEY SHARE` ile çakışmaz; rotasyon rahatça devam eder.
3. **`LockUserPasswordForLogin` → `FOR SHARE`.** Giriş, `bcrypt` karşılaştırmasını transaction'ın
   **dışında** yapıyor (~250 ms). O aralıkta parola değişirse yeni aile iptalden **sonra** doğar ve
   eski parolayla yaşardı. Bu yüzden aile açılmadan önce hash kilit altında yeniden okunur.
   `FOR SHARE` bilerek seçildi: `FOR NO KEY UPDATE` ile **çakışır** (giriş ile parola değişimi seri
   olur — doğru olan da bu), rotasyonların `FOR KEY SHARE`'iyle **çakışmaz** (girişler rotasyonları
   bekletmez).

**Akılda kalıcı özet:** ihtiyacın olan **en zayıf** kilidi al. "Ne olur ne olmaz `FOR UPDATE`"
demek, görünmez bir FK kilidiyle çakışıp deadlock üretmenin en kolay yoludur.

**🔗 İlgili:**
- [#9 Kilit sırası değişmezi](#9-kilit-sırası-değişmezi-gerçek-bir-deadlock-40p01-nasıl-doğdu)
- [PostgreSQL 16 — Explicit Locking / Row-Level Locks](https://www.postgresql.org/docs/16/explicit-locking.html#LOCKING-ROWS)
- [backend/sql/queries/users.sql](../../backend/sql/queries/users.sql) (üç sorgunun üstündeki NEDEN yorumları)

## 11. SELECT ... FOR UPDATE SKIP LOCKED: beklemek yerine atlamak

**🗓️ Bağlam:** Faz 13'te girişteki temizliği (`CleanupRefreshTokens`) yazarken. "Ölü aileleri
silen bir DELETE, tam o sırada dönen bir oturumu da öldürebilir mi?" diye sorduk — evet,
öldürüyordu.

**⚡ Kısa cevap:** `SKIP LOCKED`, "o an başka bir transaction'ın kilitlediği satırları **atla**"
demektir. Temizlik/kuyruk gibi işlerde beklemek yanlış cevaptır: bekleyen deyim, uyandığında
**bayat** bir anlık görüntüyle karar verir.

**📖 Açıklama:**

Sorun tam olarak şuydu: temizliğin alt sorgusu "bu ailenin canlı token'ı kalmamış" diye karar
verir. Aynı anda bir rotasyon aile kilidini tutuyor ve halef token'ı yazıyor olabilir. `SKIP
LOCKED` olmasaydı temizlik kilidi bekler, rotasyon commit eder, temizlik uyanır ve **kendi eski
anlık görüntüsüne göre** aileyi siler — CASCADE ile az önce kullanıcıya verilmiş halef token da
gider. Sonuç: kullanıcı sebepsiz yere oturumdan düşer.

```sql
DELETE FROM refresh_token_families
WHERE id IN (
    SELECT f.id FROM refresh_token_families f
    WHERE f.user_id = $1 AND (… ölü olma koşulları …)
    FOR UPDATE SKIP LOCKED     -- işlem gören aileye HİÇ dokunma
);
```

Atlanan aile kaybolmaz: bir sonraki girişte yeniden değerlendirilir. **Temizlik işleri için doğru
ödünleşim budur** — "şimdi değilse sonra" kabul edilebilir, "yanlış satırı sil" değil.

Aynı desenin klasik kullanımı **iş kuyruklarıdır**: `SELECT … FOR UPDATE SKIP LOCKED LIMIT 1` ile
N işçi aynı tablodan birbirini beklemeden farklı işler çeker (Postgres'i basit bir kuyruk gibi
kullanmanın standart yolu).

Üç seçeneğin farkı:

| Yazım | Davranış | Nerede doğru |
|---|---|---|
| (düz) `FOR UPDATE` | kilidi **bekler** | tek bir satırı doğru güncellemek (rotasyon, puan) |
| `FOR UPDATE NOWAIT` | kilitliyse **hata** verir | "meşgulse hemen söyle" isteyen etkileşimli işler |
| `FOR UPDATE SKIP LOCKED` | kilitli satırı **atlar** | kuyruk, toplu temizlik, süpürme |

**🔗 İlgili:**
- [#9 Kilit sırası değişmezi](#9-kilit-sırası-değişmezi-gerçek-bir-deadlock-40p01-nasıl-doğdu)
- [backend/sql/queries/users.sql](../../backend/sql/queries/users.sql) (`DeleteDeadRefreshFamilies`, `DeleteExpiredRefreshTokens`)
- [PostgreSQL 16 — SELECT / The Locking Clause](https://www.postgresql.org/docs/16/sql-select.html#SQL-FOR-UPDATE-SHARE)

## 12. COMMIT'ten sonra başarısız olabilecek adım bırakma

**🗓️ Bağlam:** Faz 13'ün inceleme turunda çıkan F02 bulgusu. Refresh rotasyonu transaction'da
bitiyor, kullanıcı satırı **sonra** ayrı bir sorguyla okunuyordu. "Bu niye sorun, sonuçta sadece
bir okuma?" diye sordum.

**⚡ Kısa cevap:** COMMIT, dış dünyaya verilmiş geri alınamaz bir sözdür. Ondan sonra atılan her
adım **başarısız olabilir** ve artık geri alınamaz; istemci ile sunucu farklı gerçekleri doğru
sanır. Kural: bir isteğin başarılı sayılması için gereken her şeyi **aynı transaction'a** al.

**📖 Açıklama:**

Bizdeki zincir şöyle kırılıyordu:

1. Rotasyon: eski token harcanır, halef yazılır → **COMMIT** (geri dönüşü yok).
2. Ayrı sorgu: kullanıcı okunur → **patlarsa** handler 500 döner.
3. Tarayıcıdaki çerez hâlâ **eski** token'ı taşıyor; istemci yeniden dener.
4. Sunucu için bu, harcanmış bir token'ın ikinci kullanımıdır → **reuse** → **tüm aile iptal**.

Yani bir saniyelik bir DB sarsıntısı, kullanıcıyı **her cihazdan** atıyordu. Düzeltme basit:
kullanıcıyı rotasyonun **kendi** transaction'ında düz bir `SELECT` ile oku ve sonuçla birlikte dön
(`Rotation.User`). Hesap silinmişse rotasyon geri alınır — yani "token harcandı ama iş yarım
kaldı" durumu artık mümkün değil. Okuma düz `SELECT` olduğu için kilit almaz, kilit sırası
değişmezini de etkilemez.

**Genelleme (dağıtık sistemlerde "dual write" problemi):**

- COMMIT'ten sonra kaçınılmaz olarak kalan tek şey **cevabı göndermek** olmalı.
- Commit sonrası zorunlu bir yan etki varsa (e-posta, cache temizleme, webhook) onu **idempotent**
  yap ve "olmadıysa zararsız" olacak şekilde tasarla. BookHub'da puan yazımından sonra Redis
  cache'ini silmek böyle: başarısız olursa en kötü ihtimalle 60 sn bayat veri gösterilir.
- Geriye kalan pencereyi **politika olarak yaz**: commit başarılı ama **cevap** kaybolduysa
  (ağ koptu, sekme kapandı) bizde bir sonraki yenileme gerçekten reuse'dur → aile ölür, kullanıcı
  yeniden girer. Alternatifi (harcanmış token'ı kısa süre kabul eden tolerans penceresi) çalınmış
  token'la yarışı kazanan saldırgana fark edilmeden alan açardı. İstemcideki `keepalive: true` bu
  pencereyi daraltıyor.

**🔗 İlgili:**
- [#7 Kayıp güncelleme](#7-kayıp-güncelleme-lost-update-neden-kilit-neden-türetilmiş-veri) — transaction sınırının diğer yüzü
- [docs/REVIEW_FEEDBACK.md](../REVIEW_FEEDBACK.md) F02 · [docs/phases/phase-13-session-hardening.md](../phases/phase-13-session-hardening.md)
- [backend/internal/auth/repository.go](../../backend/internal/auth/repository.go) (`Rotation.User`, `RotateRefreshToken`)

---

## 13. Genişlet ve daralt: bir kolonu silmek neden iki yayın ister?

**🗓️ Bağlam:** 2026-10-17, #36'nın (eser/baskı) tasarımında. Kapak ve sayfa sayısı `books`'tan
yeni `editions` tablosuna taşınacak, eski iki kolon sonra silinecekti. İlk önerim "yeni kod
baskısı olmayan kitapta eski kolona düşsün, değişiklikleri iki yere yazsın" idi. sqlc'nin ürettiği
koda bakınca bunun, kolonları silen migration koştuğu anda canlıyı kıracağı ortaya çıktı;
tasarımı birlikte düzelttik.

**⚡ Kısa cevap:** Deploy önce migration'ı uygular, yeni kod birkaç dakika sonra çıkar. Yani her
migration **o an çalışan eski kodla** uyumlu olmalı. Bir kolonu silmek bu yüzden iki yayındır:
önce kolonun adını hiç anmayan kod canlıya çıkar, sonra ayrı bir yayında kolon silinir. sqlc
`SELECT *`'ı kolon adlarına açtığı için "o alanı kullanmıyorum" demek yetmez.

**📖 Açıklama:**

Zaman çizelgesi (daraltma yayını):

| An | Veritabanı | Çalışan kod |
| --- | --- | --- |
| 1 | kolon var | eski kod (önceki yayın) |
| 2 | migration kolonu siler | **hâlâ eski kod** |
| 3 | kolon yok | yeni kod çıkar |

2. anda eski kodun gönderdiği her sorgu silinen kolonu anıyorsa hata verir. Çözüm "genişlet →
daralt" (expand / contract):

1. **Genişlet:** yeni yapıyı ekle (tablo, kolon), veriyi taşı, kodu yeni yapıya geçir. Eski yapı
   hâlâ duruyor; eski kod da yeni kod da çalışabilir.
2. **Daralt:** genişletme yayını canlıda doğrulandıktan **sonra**, ayrı bir PR'da eski yapıyı sil.

**sqlc tuzağı:** `books.sql`'de `SELECT * FROM books` yazarız, ama üretilen Go kodunda
(`internal/db/books.sql.go`) sorgu metni şöyledir:

```sql
SELECT id, title, slug, author, description, cover_url, publication_year, page_count, ... FROM books
```

Yıldız, **kod üretilirken** açılır ve tek tek ad olarak binary'ye girer. `cover_url`'i hiçbir
handler okumasa bile sorgu onu ister; kolon silinince Postgres `column "cover_url" does not exist`
der. Bu yüzden genişletme PR'ında kitap sorguları `*` yerine açık kolon listesine geçiyor ve silinecek
iki kolon listede yok.

**Kanıtlamak için test:** bir transaction açıp iki kolonu `ALTER TABLE … DROP COLUMN` ile silmek,
kitap okuyan/yazan her repository fonksiyonunu çalıştırmak ve sonunda rollback etmek. Bir sorgu
eski kolonu anıyorsa CI'da kırmızı yanar; hata canlıda değil, PR'da yakalanır. (Postgres'te DDL de
transaction içinde geri alınabilir; MySQL'de bu mümkün değil.)

**Bedeli:** genişletme yayını geri alınırsa eski kod, aradaki düzenlemeleri göremez (eski kolonlar
güncellenmedi). Bu bilinçli kabul edilir; o yüzden daraltma ancak genişletme canlıda doğrulanınca
gelir.

**JS/TS'te de aynı:** Prisma da şemadaki bütün alanları açıkça seçer; kolon silmek orada da iki
adımdır. Sorun dile değil, "migration koddan önce koşar" gerçeğine ait.

**🔗 İlgili:**
- [docs/phases/phase-16-works-editions.md](../phases/phase-16-works-editions.md) — tasarım, PR 1 / PR 2
- [CLAUDE.md](../../CLAUDE.md) "Veritabanı ve migration" — "önce genişlet → yayınla → sonra daralt" kuralı
- [#1 goose ve migration kavramı](#1-goose-ve-migration-kavramı-şema-için-git) · [#2 sqlc nedir](#2-sqlc-nedir-sqli-sen-yaz-goyu-makine-yazsın)
- [infra-and-devops.md](infra-and-devops.md) #12 — Deploy'un migration'ı koddan önce uygulaması

---

## 14. Eser ve baskı modeli: kısmi benzersiz index ile tek varsayılan

**🗓️ Bağlam:** 2026-10-12'de kapakları eklerken "Türkçe baskı mı, özgün baskı mı?" sorusu çıktı;
çeviri eklemek istediğimi söyledim ve #36 açıldı. 2026-10-17'de tasarımında `books` tablosunun aslında
iki şeyi karıştırdığını ve "kitap başına tek varsayılan baskı" kuralını nasıl garanti edeceğimizi
konuştuk.

**⚡ Kısa cevap:** Eser (work) soyut içeriktir, baskı (edition) elde tutulan kitaptır; puan ve
yorum esere, kapak/çevirmen/sayfa baskıya aittir. "Her eserin en çok bir varsayılan baskısı" kuralını
uygulama kodu değil, `UNIQUE (book_id) WHERE is_primary` gibi bir **kısmi benzersiz index**
garanti eder.

**📖 Açıklama:**

| Seviye | Örnek | Ona ait olanlar |
| --- | --- | --- |
| Eser | Suç ve Ceza, Dostoyevski, 1866, Rusça | puan, yorum, tür, `/books/:id` adresi |
| Baskı | İş Bankası, çev. …, 2019, 687 sayfa | dil, çevirmen, yayınevi, yıl, ISBN, sayfa, kapak |

BookHub'da bugün tek `books` tablosu iki seviyeyi karıştırıyor: `publication_year` ve `language`
eserin (ilk yayın, özgün dil), `cover_url` ve `page_count` ise Türkçe baskının. Örnek: 1984'ün dili
`en`, kapağı Türkçe baskının.

**Neden `books` eser olarak kaldı:** puanlar, yorumlar, tür bağları, adresler ve denetim kayıtları
hep `books.id`'ye bağlı. Yeni bir `works` tablosu açıp bunların hepsini taşımak, kazancına göre çok
riskli. Yeni tablo yalnız baskılar için: `editions.book_id → books.id`.

**Kısmi benzersiz index:**

```sql
CREATE UNIQUE INDEX editions_one_primary_idx ON editions (book_id) WHERE is_primary;
```

Benzersizlik yalnız `is_primary = true` satırlara uygulanır: bir kitabın istediği kadar baskısı
olabilir, ama ikinci bir `is_primary = true` satırı reddedilir (`23505`).

**Neden uygulamada kontrol etmek yetmez:** "önce varsayılan var mı bak, yoksa ekle" iki adımdır. Aynı
anda gelen iki istek ikisi de "yok" görüp ikisi de ekleyebilir (bkz. #7 kayıp güncelleme). Index bu
yarışı veritabanında kapatır.

**Sıra tuzağı:** varsayılanı değiştirirken önce eskisini `false` yapmak, sonra yenisini `true`
yapmak gerekir (aynı transaction'da). Tersi sırada bir an iki varsayılan olur ve index reddeder.

**JS/TS'te:** Prisma şemasında kısmi index doğrudan yazılamıyor; elle bir SQL migration eklemek
gerekiyor. Burada SQL'i zaten biz yazdığımız için tek satır.

**🔗 İlgili:**
- [docs/phases/phase-16-works-editions.md](../phases/phase-16-works-editions.md) · issue [#36](https://github.com/mtgezgin/BookHub/issues/36)
- [docs/FEATURE_REQUESTS.md](../FEATURE_REQUESTS.md) CAT-02 — aynı ayrımın ürün tarafı
- [#3 Natural key vs surrogate key](#3-natural-key-vs-surrogate-key-isbn-neden-pk-olmaz) — ISBN'in neden kimlik olmadığı
- [#7 Kayıp güncelleme](#7-kayıp-güncelleme-lost-update-neden-kilit-neden-türetilmiş-veri)
