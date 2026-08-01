# Öğrenme Notları — Database

Postgres, SQL, sqlc, migration, transaction, index kavramları.
Format ve nasıl çalıştığı: [README.md](README.md).

## İçindekiler

1. [goose ve migration kavramı: "şema için git"](#1-goose-ve-migration-kavramı-şema-için-git)
2. [sqlc nedir: SQL'i sen yaz, Go'yu makine yazsın](#2-sqlc-nedir-sqli-sen-yaz-goyu-makine-yazsın)
3. [Natural key vs surrogate key: ISBN neden PK olmaz?](#3-natural-key-vs-surrogate-key-isbn-neden-pk-olmaz)
4. [DSN (Data Source Name) nedir?](#4-dsn-data-source-name-nedir)

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
`-- name: GetBook :one` satırı normal yorum değil, **direktif**: `name:` sonrası üretilecek
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
