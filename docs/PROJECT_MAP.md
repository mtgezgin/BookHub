# BookHub — Proje Haritası (neyin ne olduğu)

> **Bu dosya "NE" sorusunun cevabı:** hangi dosya ne işe yarar, hangi araç niye var, bir istek
> nereden nereye gider. Kalıcı referans — uzun aradan sonra dönünce burayı oku.
> **[STATUS.md](STATUS.md) ise "NEREDEYİZ" sorusunun cevabı:** o oturum-devir dosyası, her
> oturumda güncellenir. İkisi kardeş: harita + imleç.
>
> Bu haritanın içindeki tüm durum bilgisi **2026-08-01'de kod/komut çalıştırılarak
> doğrulandı** (dosya envanteri, git durumu, testler, canlı ortam).

---

## 1. Otuz saniyelik özet

**BookHub**, okunan kitapların kaydedildiği bir katalog + topluluk API'si. Ama asıl amaç ürün
değil **öğrenme**: JS/TS geçmişinden gelen birinin backend teknolojilerini *neden var
oldukları* üzerinden Go ile öğrenmesi. Bu yüzden her teknoloji, projede gerçek bir problem
doğduğunda ekleniyor (§14.5 "sebepsiz teknoloji yok") ve kodun içi **Türkçe "neden böyle"
yorumlarıyla dolu** — kaynak dosyalar aynı zamanda ders notu.

- **Yığın:** Go 1.26 + Chi (router) + pgx/v5 (Postgres sürücüsü) + sqlc (kod üreteci) + goose
  (migration) + Docker Compose (dev Postgres). **ORM yok** — bilinçli karar.
- **Nerede:** Faz 2 (Postgres) — 8 adımın 6'sı bitti, 7. adım yarım (kullanıcı egzersizi).
- **Çalışan:** `GET /health` ve `GET /api/books/{id}` (uçtan uca, gerçek DB'den, testli).
- **Öğrenme günlüğü:** [learning-notes/](learning-notes/) altında **22 kayıtlı kavram**.

---

## 2. Başlarken — komut zinciri (sırayla)

Ortam her oturumda sıfırdan kaldırılır; container yeniden başlatmalarda ayakta kalmaz.

```bash
cd ~/Desktop/BookHub

# 1) Dev Postgres'i kaldır (veri named volume'da, kaybolmaz)
docker compose up -d
docker compose ps            # "healthy" görene kadar bekle

# 2) Env'i shell'e yükle  ⚠️ ZORUNLU — aşağıdaki tuzağa bak
set -a; source .env; set +a

# 3) Şema güncel mi?
cd backend
goose -dir migrations postgres "$DATABASE_URL" status

# 4) API'yi çalıştır
go run ./cmd/api            # kapatırken Ctrl+C — unutma!

# 5) Doğrula (başka bir terminalde)
curl localhost:8080/health          # {"status":"ok"}
curl localhost:8080/api/books/1     # seed: "Clean Code"
```

> ⚠️ **`.env` TUZAĞI:** Uygulama `.env` dosyasını **okumaz** (godotenv bilinçli olarak
> eklenmedi). Dosya sadece bir şablon; değerleri shell'e **elle yüklemen** gerekir (adım 2).
> Yüklemezsen `config.Load()` "DATABASE_URL zorunlu" diyerek boot'ta ölür — bu bir bug değil,
> tasarım (fail-fast).
>
> ⚠️ **`go run` unutulmuş süreç:** İki kez başımıza geldi (biri 14 gün ayakta kaldı). Port
> doluysa: `lsof -ti :8080 | xargs kill`.

Diğer sık komutlar:

| Komut | Ne yapar |
|---|---|
| `go test -race ./...` | Tüm testler (yarış dedektörüyle) |
| `gofmt -l .` | Formatı bozuk dosyaları listeler (`-w` düzeltir) |
| `sqlc generate` | `sql/queries/*.sql` → `internal/db/*.go` (SQL değişince şart) |
| `goose ... up` / `down` | Migration uygula / son migration'ı geri al (**down sadece dev**) |
| `docker compose down` | DB'yi durdur (veri kalır) · `down -v` → **veriyi de sil** |
| `docker exec -it bookhub-postgres psql -U bookhub` | DB'ye elle bak |

---

## 3. Şu an ne çalışıyor, ne çalışmıyor

| | Durum |
|---|---|
| `GET /health` | ✅ çalışıyor |
| `GET /api/books/{id}` | ✅ çalışıyor — DB'den gerçek veri, 400/404/500 haritalaması testli |
| `GET /api/books` (liste) | ❌ **rota yok** — repository hazır, handler yazılmadı |
| `POST /api/admin/books` | ❌ **rota yok** — repository hazır, handler yazılmadı |
| Testler | ✅ yeşil: 4 test fonksiyonu / 13 alt-senaryo |
| Test boşluğu | `internal/books`, `internal/database`, `internal/db` → **hiç test yok** |
| Kod hijyeni | `go vet` temiz · `gofmt -l` → `internal/books/repository.go` (dosya sonu newline eksik) |

`Repository.List` ve `Repository.Create` yazılmış **ama HTTP'den erişilemiyor** → şu an teknik
olarak ölü kod. 7. adım tam olarak bunu bağlıyor.

---

## 4. Git ritmi: neyin commit'li, neyin beklediği

Bu repoda **commit'ler milestone'da toplanır** — çünkü her `git commit` ağır kapılar tetikler
(güvenlik review'u + test coverage + kod review) ve `git push` ayrıca çok ajanlı bir review
paneli çalıştırır. Bu yüzden "her küçük adımda commit" ritmi burada pahalıdır.

Faz 2'nin commit haritası:

| Commit | İçerik |
|---|---|
| `f68e107` | Adım 1–3: docker-compose Postgres, zorunlu `DATABASE_URL`, ilk migration |
| *(bu commit)* | Adım 4–6: sqlc kurulumu + sorgular + üretilen kod, pgx havuzu, repository, dikey dilim `GET /api/books/{id}` + testi, compose port sıkılaştırması, öğrenme notları, PROJECT_MAP |
| *(sırada)* | Adım 7–8: liste + create uçları, genres/m2m, integration testi |

> ⚠️ **Commit ≠ push.** Bir şey commit'lendi diye GitHub'da değildir. Taze klonun çalışması ve
> CI'ın kodu görmesi için `git push` gerekir — commit'ten sonra push edilmediyse
> `origin/main` bayat kalır. Durumu `git status -sb` ilk satırındaki `ahead N` ile gör.

---

## 5. Bir isteğin yolculuğu (`GET /api/books/1`)

```
① BOOT      main.go: config.Load() → env doğrula (fail-fast)
            signal.NotifyContext → iptal edilebilir kök ctx
            database.New() → pgxpool + 5sn timeout'lu Ping (bağlanamazsa BURADA ölür)
② DI        books.NewRepository(pool) → newRouter(bookRepo)   ← graf elle, 2 satır
────────────────────────────────────────────────────────────── istek gelir ──
③ ROUTER    cmd/api/router.go · Chi: RequestID → Logger → Recoverer
④ HANDLER   cmd/api/books.go · {id}'yi ParseInt ile doğrula
              └─ geçersizse DB'ye HİÇ GİTMEDEN 400 invalid_id
⑤ REPO      internal/books/repository.go · repo.Get(r.Context(), id)
              └─ ctx iletilir: istemci kopunca sorgu da iptal olur
⑥ SQLC      internal/db/books.sql.go · QueryRow + 12 kolonluk row.Scan
              └─ $1 prepared statement — string birleştirme yok
⑦ PGX       DBTX interface'i → *pgxpool.Pool → Postgres (havuzdan bağlantı al, iade et)
⑧ DÖNÜŞ     satır yok → pgx.ErrNoRows → books.ErrNotFound → 404
            beklenmeyen hata → log'a yaz, istemciye "internal" (detay SIZMAZ) → 500
            başarı → writeJSON(200, book)
```

Öğretici çekirdek: **her katman bir sonrakinin sürücüsünü bilmez.** Handler pgx'i tanımaz
(repository `ErrNoRows`'u domain hatasına çevirir); repository HTTP'yi tanımaz (status kodu
handler'ın işi).

---

## 6. Dosya haritası

### `backend/cmd/api/` — çalıştırılabilir uygulama (paket `main`)

| Dosya | Ne yapar |
|---|---|
| `main.go` | Giriş noktası + **tüm bağımlılık grafiği**: config → pgxpool → repository → router → http.Server (timeout'lu) → goroutine'de dinle → sinyalde 10sn'lik graceful shutdown |
| `router.go` | Chi router'ı, middleware zinciri, rota kayıtları, JSON 404/405 zarfları, `handleHealth` |
| `books.go` | `bookGetter` interface'i (**tüketen tarafta** tanımlı) + `handleGetBook`: girdi doğrulama, ctx iletimi, hata→HTTP haritalaması |
| `respond.go` | Tüm cevap yazımının tek kapısı: `writeJSON` + `{"error":{"code","message"}}` zarfı. *Frontend değişebilen `message`'a değil sabit `code`'a bakar.* |
| `main_test.go` | Faz 1'in ilk testi — `handleHealth`'i router'sız çağırır |
| `router_test.go` | Bilinmeyen yolda JSON hata zarfı dönüyor mu (`newRouter(nil)` ile) |
| `books_test.go` | `fakeBooks` sahtesiyle **DB'siz** handler testi, 5 senaryo (200/404/400×2/500) |

### `backend/internal/` — dışa kapalı paketler (derleyici zorlar)

| Paket / dosya | Ne yapar |
|---|---|
| `config/config.go` | Env'den `Port` + `DatabaseURL`; boot'ta doğrular **ve normalize eder** (pointer receiver). `DATABASE_URL` için varsayılan **yok** |
| `config/config_test.go` | Table-driven, `t.Setenv` ile izole, 8 senaryo |
| `database/database.go` | Tek işi: `pgxpool` aç + **5sn timeout'lu Ping** (pgxpool tembel olduğu için fail-fast bunu gerektirir) |
| `books/repository.go` | Kitap veri erişimi. `pgx.ErrNoRows` → `books.ErrNotFound` çevirisi; `db.DBTX` aldığı için ileride **transaction içinde de** çalışır. Metotlar: `Get`, `List`, `Create` |
| `db/` (**sqlc üretimi, elle düzenleme**) | `db.go` (`DBTX`/`Queries`/`WithTx`), `models.go` (`Book`/`Genre`/`BookGenre` struct'ları), `books.sql.go` (`GetBook`/`ListBooks`/`CreateBook` + `Scan` boilerplate'i) |

### `backend/` — SQL ve yapılandırma

| Yol | Rolü — **ikisini karıştırma** |
|---|---|
| `migrations/0001_books_genres.sql` | **goose'un şema tarihi.** `books`/`genres`/`book_genres`. Kararlar: identity PK, `TEXT + CHECK` status (DEFAULT'suz!), `timestamptz`, `slug UNIQUE`, partial index (`created_at DESC WHERE status='APPROVED'`), `book_genres` bileşik PK + ayrı `(genre_id)` index'i |
| `sql/queries/books.sql` | **sqlc'nin girdisi.** `GetBook :one`, `ListBooks :many`, `CreateBook :one` |
| `sqlc.yaml` | Şemayı `migrations/`'tan okur (şema iki kez tanımlanmaz). pgx/v5, JSON tag'leri, override'lar: `timestamptz→time.Time`, nullable int → `*int32` |
| `go.mod` | **Sadece 2 doğrudan bağımlılık:** chi v5.3.0, pgx/v5 v5.10.0. goose ve sqlc CLI'dır — burada yoklar |

### Kök ve `docs/`

| Yol | Ne |
|---|---|
| `docker-compose.yml` | Dev Postgres 16 (named volume `pg_data`, healthcheck, `127.0.0.1:5432` — LAN'a kapalı) |
| `.env.example` / `.env` | `PORT` + `DATABASE_URL`. İkisi şu an **birebir aynı** (dev'de gerçek sır yok); `.env` gitignore'lu |
| `.githooks/pre-commit` | Hızlı kapı: `gofmt -l` + `go vet` (sadece backend değiştiyse) |
| `.githooks/pre-push` | Ağır kapı: `golangci-lint` + `go test -race`. ⚠️ `core.hooksPath` **yerel** config — taze klonda bir kez `git config core.hooksPath .githooks` |
| `.github/workflows/ci.yml` | gofmt → vet → build → test -race → golangci-lint (otoriter backstop) |
| `docs/STATUS.md` | **Oturum devri** — yeni konuşmanın ilk okuduğu dosya (kök `README.md` boş!) |
| `docs/PROJECT_BRIEF.md` | Ürün + roadmap (§11) + kararlar (§18 addendum). *Not: içinde proje adı "BookWise" geçiyor* |
| `docs/GO_CONVENTIONS.md` | Nasıl yazıyoruz — 12 bölüm + PR checklist |
| `docs/WHY_GO_VS_TS.md` | "Neden Go" yargı günlüğü; her bölüm kanıt gelene kadar HYPOTHESIS |
| `docs/phases/` | Faz faz rehber (README + phase-01..07) |
| `docs/learning-notes/` | Sorulan kavramların kalıcı notları (`/not` skill'i ekler) |

---

## 7. Araç zinciri — hangi araç neden var

| Araç | Ne çözüyor | Nerede |
|---|---|---|
| **Chi** | net/http uyumlu router + hazır middleware (RequestID/Logger/Recoverer) | go.mod |
| **pgx/v5 + pgxpool** | Postgres sürücüsü + bağlantı **havuzu** (her istekte TCP+auth el sıkışması pahalı) | go.mod |
| **sqlc** v1.31.1 | El yazması SQL → tip-güvenli Go. **Runtime'da yok**, kod üreteci. Hataları derleme anına çeker | `~/go/bin` |
| **goose** v3.27.2 | Şemayı versiyonlu + geri alınabilir SQL dosyalarıyla yönetir ("şema için git") | `~/go/bin` |
| **Docker Compose** | Dev Postgres'i tek komutla, izole ve silinebilir tutar. Prod'da yok (Supabase) | kök |
| **golangci-lint** | Kalite/bug linter'ı (ESLint karşılığı; gofmt formatter = Prettier karşılığı) | `~/go/bin` + CI |

---

## 8. Faz merdiveni

| Faz | Konu | Durum |
|---|---|---|
| 0 | Repo + git + .gitignore | ✅ |
| 1 | Minimal Go API (/health, config, graceful shutdown, Chi, testler, CI, hook'lar) | ✅ push'lu |
| **2** | **Postgres + Kitap/Tür CRUD** | **🔄 6/8 adım — buradasın** |
| 3 | Auth + roller (JWT, bcrypt/argon2id) — *ilk Service katmanı burada doğar* | 📝 taslak |
| 4 | Öneri + moderasyon (status state machine, audit log) | 📝 taslak |
| 5 | Yorum + puan — *projenin #1 doğruluk dersi:* `UNIQUE` + `ON CONFLICT`, agregatı asla app'te oku-yaz etme | 📝 taslak |
| 6 | Ağırlıklı puan (Bayesian `WR = (v/(v+m))·R + (m/(v+m))·C`), saf fonksiyon + tablo testleri | 📝 taslak |
| 7 | Redis cache + rate limit — **MVP burada biter** (önce ölç, sonra cache'le) | 📝 taslak |
| 8–12 | Docker imajı · Frontend · CI/CD deploy · NGINX · Kubernetes | ⏭️ opsiyonel |

Faz 2'nin 8 adımı: ①compose Postgres ✅ ②DATABASE_URL config ✅ ③goose + migration ✅
④sqlc ✅ ⑤pgx pool ✅ ⑥dikey dilim `GET /api/books/{id}` ✅ **⑦liste+create+genres/m2m 🔶**
⑧repository integration testi ⏳

> Faz 3–7 bilerek "taslak": adımları, 1–2'de yazılan koddan doğacak. Boşluk değil, prensip.

---

## 9. Öğrenme notları indeksi (22 girdi)

Her girdi sabit formatta: **🗓️ Bağlam** (bunu neden sordum) → **⚡ Kısa cevap** → **📖 Açıklama**
→ **🔗 İlgili**. Ayırt edici değeri "Bağlam" alanı: hangi hatayı alırken, hangi dosyaya
bakarken sorulduğunu kaydeder.

| Dosya | Girdiler |
|---|---|
| [go-language.md](learning-notes/go-language.md) (8) | stdlib · `_test.go` konvansiyonu · paket↔modül · goroutine/channel/context/defer · sync-by-default · if-init + tek harfli isimler · toolchain komutları (`./...`, gofmt) · **zero value** |
| [tooling-and-workflow.md](learning-notes/tooling-and-workflow.md) (5) | memory/skill/hook · proje-içi skill · commit gate'leri · gofmt vs golangci-lint · pre-commit vs pre-push |
| [backend-and-web.md](learning-notes/backend-and-web.md) (4) | graceful shutdown · Node'da da olur mu · environment variable · 12-factor |
| [database.md](learning-notes/database.md) (4) | goose & migration · **sqlc** · natural vs surrogate key (ISBN) · DSN |
| [security-and-auth.md](learning-notes/security-and-auth.md) (1) | sır (secret) nedir — dev credential'ı repoya yazmak |

---

## 10. Konvansiyonlar (kodda görünen kurallar)

1. **`ctx` her yerde ilk parametre** — `r.Context()`'ten doğar, pgx'e kadar kesintisiz iner.
2. **Hatalar değerdir:** `%w` ile sar, domain sentinel'i tanımla (`ErrNotFound`), HTTP'ye
   **tek yerde** (handler) map'le.
3. **Elle DI** — framework yok; graf `main.go`'da okunur. Interface'i **tüketen** tarafta tanımla.
4. **Saf CRUD'da Service katmanı yazma** — anemik katman cargo-cult'tur; Faz 3'te (auth) doğar.
5. **Fail-fast config + timeout + graceful shutdown.**
6. **sqlc + pgx, ORM yok.** `sql/queries/` ≠ `migrations/`.
7. **Testler:** standart `testing`, table-driven, `-race`; assertion framework'ü yok.

### Bilinen bakım borcu (aradan sonra tuzak olabilecekler)

- **Kod yorumları "GO_CONVENTIONS §5" diyor ama elle DI kuralı §3'te** (§5 sunucu yaşam
  döngüsü). 3 dosyada numara kayması var.
- **§10 `slog` diyor, kod stdlib `log` kullanıyor** (8 yerde). §11 (DTO + validator) hiç
  uygulanmadı — `db.Book` doğrudan API cevabı olarak serileşiyor (`status`, `created_at` sızıyor).
- **WHY_GO_VS_TS doldurma borcu:** Faz 2 kod üretti ama **§6 (sqlc+pgx) ve §7 (ctx)** hâlâ
  `[TBD]` placeholder'ında. §13'teki "7 alt-test" bayat (artık 8). §3'teki "henüz interface yok"
  da bayat (`bookGetter` var).
- **golangci-lint sürüm çatlağı:** lokal **v1.64.8** ↔ CI **v2.12.2** (major fark, `.golangci.yml`
  yok) → pre-push'ta yeşil olan CI'da kırmızı olabilir.
- **Kırık link:** `database.md` → `infra-and-devops.md` (dosya henüz yok).
- **Şema drift'i:** brief'te `ARCHIVED` statüsü var, migration'daki `CHECK` sadece 3 değere
  izin veriyor → ileride migration gerekecek.
- **Küçük teknik:** `main.go`'da `ListenAndServe` goroutine'i içindeki `log.Fatalf`,
  `defer pool.Close()`'u atlar; `http.Server`'da `IdleTimeout` set edilmemiş.
- Kök `README.md` **boş** (0 byte); `docs/decisions/` (ADR) klasörü hiç açılmamış;
  `frontend/`, `scripts/` boş.
- Slug UNIQUE ihlali (Postgres `23505`) domain hatasına çevrilmiyor → create handler'ı
  yazılınca **409 yerine 500** dönecek.

---

## 11. Sıradaki adım — Faz 2 / adım 7 (yarım kalan egzersiz)

1. `internal/books/repository.go`: `List`/`Create`'e doc comment + `gofmt -w`.
2. `handleListBooks` — `handleGetBook` şablonundan. ⚠️ Repo `nil` slice dönebilir; `writeJSON`
   öncesi `[]db.Book{}`'a normalize et, yoksa boş listede JSON `null` döner.
3. `handleCreateBook` — **ayrı** `createBookRequest` struct'ı (istemciden `status` alınmaz →
   mass-assignment koruması), zorunlu alan kontrolü → 400, başarı → **201**.
4. `bookGetter` 3 metoda çıkar → `books_test.go`'daki `fakeBooks` da 3 metot almazsa
   **test paketi derlenmez** (bu iki değişiklik atomik).
5. Router: `GET /api/books` + `POST /api/admin/books`.
6. Sonra: genres CRUD + `book_genres` many-to-many → **adım 8** (Compose Postgres'e karşı
   integration testi) → milestone commit + push.
