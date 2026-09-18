# Faz 2 — Postgres + Kitap/Tür CRUD

> Durum: ✅ TAMAM · Önkoşul: Faz 1 ✅
> İlgili: [WHY_GO_VS_TS](../WHY_GO_VS_TS.md) §6/§7 · [GO_CONVENTIONS](../GO_CONVENTIONS.md) §6/§7 · [PROJECT_BRIEF](../PROJECT_BRIEF.md) §7.1/§7.2/§8/§9, §18.1/§18.3/§18.4

## 🎯 Amaç

API'yi gerçek bir veritabanına bağla. Kitapları ve türleri (genres) kalıcı olarak sakla. Önce
**tek bir dikey dilim** (tek Book'u uçtan uca), sonra türler ve many-to-many ilişki.

## 🧠 Öğreneceğin yeni kavram(lar)

- **Dev'de Docker Postgres** (prod ileride Supabase — aynı kod, farklı `DATABASE_URL`).
- **Migration** (goose) — şemayı versiyonlu, geri alınabilir SQL dosyalarıyla yönetmek.
- **sqlc + pgx** — elle SQL yaz, sqlc onu **tip-güvenli Go**'ya çevirsin (ORM değil).
- **Repository pattern** — SQL'i tek katmanda topla.
- **Dikey dilim** — bir özelliği migration→repo→handler→JSON→test boyunca uçtan uca bitirmek.
- **Many-to-many** ilişki (`book_genres` ara tablosu).

## Neden bu faz burada?

Faz 1'de "boş" bir API'miz var. Gerçek bir uygulama veriyi saklar. Burada SQL, ilişkiler ve
tip-güvenli sorguların temelini atıyoruz — sonraki tüm fazlar (auth, yorum, puan) bu temele
oturacak. Önce tek dilimi uçtan uca yapmak, "tüm akışı bir kez gör" anını veriyor.

## 🪜 Adımlar (step-by-step)

1. **Docker Compose'a dev Postgres ekle.** `docker-compose.yml` ile bir `postgres:16` servisi
   (named volume ile veri kalıcı). API hâlâ `go run` ile host'ta çalışır (hızlı feedback).
   - *Checkpoint:* `docker compose up -d` sonrası Postgres ayakta; `psql`/bir GUI ile bağlanılıyor.
2. **`DATABASE_URL`'i config'e ekle** (Faz 1'in config doğrulamasına). `.env.example`'a yaz.
3. **goose'u kur, ilk migration.** `migrations/0001_books_genres.sql`: `books`, `genres`,
   `book_genres` tabloları — doğru tip ve kısıtlarla (aşağıdaki ⚠️).
   - *Checkpoint:* `goose up` tabloları oluşturuyor; `goose down` geri alıyor.
4. **sqlc'yi kur.** `sqlc.yaml` + `sql/queries/books.sql` (örn. `GetBook`, `ListBooks`,
   `CreateBook`). `sqlc generate` → tip-güvenli Go kodu üretir.
   - *Neden:* SQL'i sen yazıyorsun (join'leri öğreniyorsun) ama `rows.Scan` boilerplate'ini sqlc
     üretiyor — `rows.Err()`/NULL tuzakları yok.
5. **pgx pool bağlantısı.** `internal/database` içinde `pgxpool` ile bağlan; `main.go`'da kur,
   elle DI ile repository'ye geçir.
6. **DİKEY DİLİM: tek Book uçtan uca.** `GET /api/books/{id}`:
   migration → sqlc sorgu → repository → handler → JSON → bir test. Sadece bu.
   - *Checkpoint:* Seed'lediğin bir kitabı `curl` ile çekebiliyorsun.
7. **Yay: liste + create + türler.** `GET /api/books`, `POST /api/admin/books` (şimdilik authsuz),
   `genres` CRUD, ve `book_genres` ile many-to-many (bir kitabın türleri).
8. **Repository integration testi.** Compose Postgres'e karşı (mock değil) gerçek sorguları test
   et — özellikle many-to-many join'i. (İleride testcontainers'a geçebiliriz.)
   - *Checkpoint:* `go test ./...` (DB'li testler dahil) PASS.

## ⚠️ Önemli noktalar

- **İki SQL klasörünü karıştırma:** `sql/queries/` = sqlc girdisi; `migrations/` = goose şeması.
  Farklı işler. (PROJECT_BRIEF §18.4)
- **Tipler & kısıtlar baştan doğru** (PROJECT_BRIEF §18.3): `slug` `UNIQUE`; status
  `TEXT + CHECK (... IN (...))` (native ENUM değil — evrimi kolay); `timestamptz`; PK varsayılan
  `bigint GENERATED ALWAYS AS IDENTITY`.
- **Index'leri migration'la birlikte planla:** her FK'yi index'le (Postgres FK'leri otomatik
  index'lemez), `book_genres` PK `(book_id, genre_id)` + ayrıca `(genre_id)` index'i.
- **Import cycle tuzağı:** domain paketleri birbirini import etmesin; cross-domain referansları
  **ID ile** ver (`BookID`, `UserID`) ya da küçük bir `domain/` paketi kullan. (GO_CONVENTIONS §6)
- **Saf CRUD'da Service katmanı yazma.** Bu fazda gerçek iş mantığı yok → handler doğrudan
  repository'yi çağırabilir. Service'i Faz 3'te (auth) ekleyeceğiz. (GO_CONVENTIONS §4)
- **`ctx`'i sorguya ilet.** Tüm pgx çağrıları `ctx` alır; Faz 1'de başlattığın alışkanlık
  burada işe yarıyor. (WHY_GO_VS_TS §7)
- **Migration'ı dev DB'ye uygula; prod'a (Supabase) asla `goose down` çekme.** (PROJECT_BRIEF §7.2)
- **ORM'e kayma.** GORM/bun yok — SQL'i görmek bu fazın asıl dersi. (PROJECT_BRIEF §7.1)

## ✅ Tamamlanma (Definition of Done)

- [x] `docker compose up -d` ile dev Postgres çalışıyor; API ona bağlanıyor.
- [x] goose migration'ları `up`/`down` çalışıyor; tablolar doğru tip/kısıt/index ile.
- [x] sqlc üretilen kodla kitap & tür CRUD çalışıyor; many-to-many bağ kurulabiliyor.
- [x] Tek Book dikey dilimi uçtan uca çalışıyor ve testi var.
- [x] En az bir repository integration testi PASS; `go test -race ./...` yeşil.

## 📒 Bu fazda WHY_GO_VS_TS güncellemesi

- **§6** (sqlc+pgx vs Prisma/ORM) → `sql/queries/books.sql` + üretilen kod; many-to-many join.
- **§7** (context propagation) → `internal/books/repository.go`'da ctx'li pgx çağrısı.
- §15'e bir girdi: "I/O-bound CRUD'da Node de gayet yeterdi" gibi dürüst bir karşı not.
