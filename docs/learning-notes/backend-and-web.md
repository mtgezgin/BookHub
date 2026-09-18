# Öğrenme Notları — Backend & Web

Dil-bağımsız backend/web kavramları: HTTP, API tasarımı, config/12-factor, server lifecycle
(graceful shutdown), hata yönetimi. Format: [README.md](README.md).

## İçindekiler

1. [Graceful shutdown nedir, neden önemli?](#1-graceful-shutdown-nedir-neden-önemli)
2. [Graceful shutdown Express/Node ile de yapılır mı?](#2-graceful-shutdown-expressnode-ile-de-yapılır-mı)
3. [Environment variable nedir? (os.Getenv, lokal vs prod)](#3-environment-variable-nedir-osgetenv-lokal-vs-prod)
4. [The Twelve-Factor App (12-factor) nedir?](#4-the-twelve-factor-app-12-factor-nedir)
5. [HTTP hata kodları: bu kimin hatası? (400 / 409 / 500)](#5-http-hata-kodları-bu-kimin-hatası-400--409--500)
6. [Middleware ve context: kimlik isteğe nasıl taşınır? (401 vs 403)](#6-middleware-ve-context-kimlik-isteğe-nasıl-taşınır-401-vs-403)
7. [Ağırlıklı (Bayes) puan: neden basit ortalama sıralamaya yetmez?](#7-ağırlıklı-bayes-puan-neden-basit-ortalama-sıralamaya-yetmez)
8. [Cache-aside ve rate limit: neden paylaşılan state (Redis)?](#8-cache-aside-ve-rate-limit-neden-paylaşılan-state-redis)

---

## 1. Graceful shutdown nedir, neden önemli?

**🗓️ Bağlam:** Faz 1'de ilk minimal Go sunucusunu (`/health`) kurmuştuk. Sunucuyu **Ctrl-C**
ile kapatıyordum; ben "şimdilik Ctrl-C biraz *kaba* kapatır, bunu graceful shutdown adımında
düzelteceğiz" deyince "graceful shutdown ne demek?" diye sordum.

**⚡ Kısa cevap:** Programı durdururken **anında öldürmek yerine**, (1) yeni istek almayı kes,
(2) elindeki devam eden istekleri bitir, (3) kaynakları (DB, dosya, log) düzgün kapat, **sonra**
çık.

**📖 Açıklama:**

Analoji — dükkân kapatmak gibi:
- *Nazik:* ön kapıyı kilitle (yeni müşteri yok) → içeride kasada bekleyenleri servisle → ışıkları
  kapat, çık.
- *Kaba:* müşteriler kasada ödeme yaparken elektriği komple kes.

Neden önemli?
1. **Devam eden istek kaybolmasın** — kullanıcı tam kitap önerisi/puan gönderirken deploy
   olursan, kaba kapanmada isteği yarıda ölür (hata + yarım kayıt).
2. **Veri bütünlüğü** — açık transaction'ı, buffer'daki log'u aniden kesmek yerine bilerek bitir.
3. **Deploy/restart sık olur** — production'da uygulamayı sürekli yeniden başlatırsın; her
   restart'ta kullanıcı hata almamalı.
4. **⭐ Asıl büyük sebep: Docker/Kubernetes.** K8s konteyneri durdururken önce **`SIGTERM`**
   ("lütfen dur") yollar ve bir **süre tanır** (ör. 30 sn); dolunca **`SIGKILL`** ("hemen öl",
   yakalanamaz) gönderir. O süreyi nazik kullanırsan rolling deploy'da **tek istek bile düşmez.**

Sinyaller (programa "dur" deme yolları):
- **`SIGINT`** → Ctrl-C.
- **`SIGTERM`** → Docker/K8s'in nazik "dur"u; **yakalanabilir** → burada graceful davranırız.
- **`SIGKILL`** → zorla öldürme; **yakalanamaz** (grace süresi dolunca gelir).

Go'da nasıl yapacağız (sonraki adım):
```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()
// ... server bir goroutine'de ...
<-ctx.Done()                  // sinyal bekle
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
srv.Shutdown(shutdownCtx)     // yeni bağlantı alma + mevcutları (deadline'a kadar) bitir
```
`ctx` üstündeki **timeout**: takılı bir istek için sonsuza kadar bekleme, "en fazla şu kadar
bekle, sonra yine de çık".

**🔗 İlgili:**
- [docs/GO_CONVENTIONS.md](../GO_CONVENTIONS.md) §5 (server lifecycle: timeouts + graceful shutdown)
- [docs/WHY_GO_VS_TS.md](../WHY_GO_VS_TS.md) §8 (graceful shutdown & fail-fast config)
- [docs/phases/phase-01-minimal-go-api.md](../phases/phase-01-minimal-go-api.md) Adım 7
- Bkz. aşağıda [Girdi 2](#2-graceful-shutdown-expressnode-ile-de-yapılır-mı) (Express ile farkı)

---

## 2. Graceful shutdown Express/Node ile de yapılır mı?

**🗓️ Bağlam:** Graceful shutdown'ı öğrendikten sonra, JS/Node geçmişimden gelerek "dur, aklıma
geldi — bu birebir Express ile de yapılabiliyor değil mi?" diye sordum. Bu bir **yargı
sorusu**ydu: "Go'ya özel bir şey mi, yoksa genel bir kavram mı?"

**⚡ Kısa cevap:** **Evet, neredeyse birebir.** Graceful shutdown Go'ya özel bir yetenek değil;
genel bir kavram. Go'nun stdlib'i sadece *biraz daha az uğraşla* bunu default idiom yapıyor.

**📖 Açıklama:**

Express/Node'da:
```js
const server = app.listen(8080);
function shutdown(signal) {
  server.close(() => { /* db.end() vb. */ process.exit(0); }); // yeni bağlantı yok + mevcutları bitir
  setTimeout(() => process.exit(1), 10_000).unref();           // deadline (son çare)
}
process.on('SIGTERM', () => shutdown('SIGTERM'));
process.on('SIGINT',  () => shutdown('SIGINT'));
```

Eşleşme birebir:

| Kavram | Node/Express | Go |
|---|---|---|
| Sinyali yakala | `process.on('SIGTERM')` | `signal.NotifyContext(...)` |
| Nazik kapat | `server.close(cb)` | `srv.Shutdown(ctx)` |
| Deadline | elle `setTimeout` | `ctx`'e gömülü (`WithTimeout`) |

Küçük nüanslar (dürüst olmak için):
1. Go'da deadline `Shutdown(ctx)`'in içinde hazır; Node'da `setTimeout` ile **kendin** kurarsın.
2. **Keep-alive tuzağı:** Node'da `server.close()` tarihsel olarak boştaki keep-alive
   bağlantılarını kapatmaz; çözüm `server.closeAllConnections()` (Node 18.2+) veya
   `http-terminator`. Go'nun `Shutdown`'ı bunu kutudan halleder.

**Çıkarım (önemli — yargı):** Fark **ergonomi**, "Go yapar Node yapamaz" değil. Bu tam da
[WHY_GO_VS_TS.md](../WHY_GO_VS_TS.md) §8'in "honest caveat"ının söylediği şey. Hedefim "Go
kazandı" demek değil; **"bu fark benim için gerçekten önemli mi?"** diye sorabilmek.

**🔗 İlgili:**
- Yukarıda [Girdi 1](#1-graceful-shutdown-nedir-neden-önemli) (graceful shutdown'ın kendisi)
- [docs/WHY_GO_VS_TS.md](../WHY_GO_VS_TS.md) §8 ve §15 ("TS/JS nerede daha iyi/eşit")

---

## 3. Environment variable nedir? (os.Getenv, lokal vs prod)

**🗓️ Bağlam:** Config'te `getenv`/`os.Getenv` satırını görünce "port'u ben nasıl tanımlayacağım,
`os.Getenv` nereden çekiyor, lokalde ve prod'da nasıl bir işlem gerekiyor?" diye sordum.

**⚡ Kısa cevap:** Her process'in OS'tan aldığı `KEY=value` çiftleri = environment.
`os.Getenv("PORT")` **o process'in environment'ından** okur (DOSYA DEĞİL); yoksa `""` döner.
Lokalde shell ile (`export`/inline) veya `.env`+loader ile; prod'da deploy platformu enjekte
eder. Aynı kod her yerde.

**📖 Açıklama:**

`os.Getenv` bir **dosya okumaz**; process'in hafızasındaki env'i okur. Process bu env'i onu
**başlatan şeyden** (shell, Docker, platform) miras alır.

Lokalde tanımlama:
- **Inline** (tek komut): `PORT=9090 go run ./cmd/api`
- **Shell oturumu:** `export PORT=9090` → sonraki komutlar görür (`unset` ile kaldırılır)
- **`.env` dosyası — TUZAK:** OS `.env`'i **otomatik okumaz**; bir loader gerekir (Node'da
  `dotenv`, Go'da `godotenv`). **Bizim kod şu an `.env` okumuyor** → `.env`'e yazmak tek başına
  işe yaramaz.

Prod'da: shell/`.env` yok, secret git'te yok. **Platform enjekte eder:**
- Render/Fly/Vercel → dashboard / secret store
- Docker → `-e KEY=val` / compose `environment:`
- K8s → Pod `env:` / `envFrom:` (ConfigMap + Secret)

Secret vs değil: PORT secret değil (`.env.example`'a yazılabilir); `DATABASE_URL`/`JWT_SECRET`
**secret** → asla git'te, prod'da secret store'da.

Bizde pratik: tek PORT için şimdilik default/inline yeter; `.env`+`godotenv`'i **Faz 2**'de
`DATABASE_URL` gelince (sebepli) ekleriz.

**🔗 İlgili:**
- [backend/internal/config/config.go](../../backend/internal/config/config.go) (`getenv`/`os.Getenv`)
- [docs/PROJECT_BRIEF.md](../PROJECT_BRIEF.md) §7.2/§7.3 (dev-lokal/prod-cloud; DSN'ler secret store'da)
- Aşağıda [Girdi 4](#4-the-twelve-factor-app-12-factor-nedir) (config-in-env = 12-factor III)

---

## 4. The Twelve-Factor App (12-factor) nedir?

**🗓️ Bağlam:** Config'i env'den okurken "12-factor" dememe üzerine "o nedir, onu da bilmiyorum"
dedim.

**⚡ Kısa cevap:** Modern web/SaaS uygulamaları için **12 ilkelik bir rehber** (Heroku, ~2011,
12factor.net): "bulut dostu, kolay deploy edilen/ölçeklenen" uygulama nasıl yazılır — yaygın bir
**ortak dil**.

**📖 Açıklama:**

12 ilke (⭐ = bizde doğrudan karşılığı olan):

| # | İlke | Bizde |
|---|---|---|
| 1 | Codebase (tek repo, çok deploy) | git repo |
| 2 | Dependencies (açık tanım) | `go.mod` |
| 3 | **Config (env'de tut)** ⭐ | PORT env'den (config adımı) |
| 4 | **Backing services (takılı kaynak)** ⭐ | `DATABASE_URL`/`REDIS_URL` swap (dev↔prod) |
| 5 | Build/release/run ayrı | Faz 8 |
| 6 | Stateless processes | state'i DB/cache'te tut |
| 7 | **Port binding** ⭐ | Go server `:8080`'e bağlanır |
| 8 | Concurrency (çok instance) | Faz 12 |
| 9 | **Disposability (graceful shutdown)** ⭐ | shutdown dersi |
| 10 | **Dev/prod parity** ⭐ | aynı binary, env-driven |
| 11 | **Logs → stdout** ⭐ | `log` stdout/stderr'e yazar |
| 12 | **Admin processes** ⭐ | planlanan `bookhub` CLI |

Asıl ders: soyut değil — fazları ilerledikçe bunları **sebebiyle** uyguluyoruz. Config'i
env'den okumak = Factor III; graceful shutdown = Factor IX; "aynı kod, farklı env" = Factor X.

**🔗 İlgili:**
- 12factor.net (kaynak)
- Yukarıda [Girdi 3](#3-environment-variable-nedir-osgetenv-lokal-vs-prod) (config-in-env)
- [docs/PROJECT_BRIEF.md](../PROJECT_BRIEF.md) §7 (dev-lokal/prod-cloud = parity)

---

## 5. HTTP hata kodları: bu kimin hatası? (400 / 409 / 500)

**🗓️ Bağlam:** `POST /api/admin/books` ucunu yazarken üç farklı hata yolunu haritalamak gerekti:
bozuk JSON, eksik alan, aynı slug'ın ikinci kez gönderilmesi ve beklenmeyen DB hatası. Aynı slug
başta **500** dönüyordu; bunun yanlış olduğunu konuştuk ve **409**'a çevirdik.

**⚡ Kısa cevap:** Doğru kodu seçmenin tek sorusu var: **"bu hatayı kim düzeltebilir?"**
İstemci düzeltebiliyorsa `4xx` (ve mesajı GÖSTER), yalnızca sunucu tarafı düzeltebiliyorsa
`5xx` (ve detayı SAKLA, log'a yaz).

**📖 Açıklama:**

Bizim uçtaki tam harita:

| Durum | Kod | `error.code` | Detay istemciye? |
|---|---|---|---|
| Gövde bozuk JSON / bilinmeyen alan / boş | **400** | `invalid_json` | **Evet** — girdiyi o yazdı, düzeltebilsin |
| Zorunlu alan eksik, `page_count: 0` | **400** | `validation_error` | **Evet** — hangi alan olduğunu söyle |
| Aynı slug ikinci kez | **409** | `slug_conflict` | **Evet** — başka slug seçebilir |
| Beklenmeyen DB/sürücü hatası | **500** | `internal` | **HAYIR** — log'a yaz, generic mesaj dön |

**400 vs 409 ayrımı:** 400 "gönderdiğin şey **kendi başına** hatalı" demektir (eksik alan, bozuk
JSON) — istek tek başına bakıldığında bozuktur. 409 ise "gönderdiğin şey geçerli ama **mevcut
durumla çakışıyor**" demektir — aynı gövde beş dakika önce gönderilse çalışırdı, ya da farklı bir
slug'la şimdi çalışır. Çakışma isteğin değil, **sistemin durumunun** fonksiyonudur.

**Neden 500 yanlıştı:** 500 "sunucuda beklenmedik bir şey oldu, senin yapabileceğin bir şey yok"
demektir. Ama aynı slug'ı göndermek **beklenen** bir istemci hatasıdır ve çözümü istemcide.
500 dönmek üç şeyi birden bozar: (1) istemci ne yapacağını bilemez, (2) hata izleme sistemleri
gerçek arıza sanıp alarm üretir, (3) SLA/uptime metrikleri boşuna kirlenir.

**Detay göster/gizle kuralı aynı soruya bağlı.** Aynı projede iki zıt karar var ve ikisi de doğru:

```go
// Girdi hakkında → GÖSTER (yardım eder, sızıntı değil)
writeError(w, 400, "invalid_json", err.Error())   // "bilinmeyen alan: \"status\""

// Sunucunun içi hakkında → SAKLA (şema/sürücü bilgisi taşıyabilir)
log.Printf("CreateBook başarısız: %v", err)
writeError(w, 500, "internal", "beklenmeyen bir hata oluştu")
```

**Sık karışan diğerleri:** 401 (kimliğini bilmiyorum → giriş yap) vs 403 (kim olduğunu biliyorum,
yetkin yok) · 404 (yok) vs 403 — güvenlikte çoğu zaman **404 tercih edilir**, çünkü 403 "burada
bir şey VAR ama göremezsin" bilgisini sızdırır (bkz. `GET /api/books/{id}`, PENDING kitaba 404) ·
422 (sözdizimi doğru ama semantik geçersiz) — 400 ile sınırı bulanık, biz 400'de birleştirdik.

**Dil bağımsız:** Express/NestJS'te de aynı sorular. Fark yalnızca `res.status(409).json(...)`
yazmak; karar mekanizması HTTP'nin kendisine ait.

**🔗 İlgili:**
- [backend/cmd/api/books.go](../../backend/cmd/api/books.go) — `handleCreateBook`'taki `switch` haritalaması
- [backend/internal/books/repository.go](../../backend/internal/books/repository.go) — `ErrDuplicateSlug` (Postgres 23505 → domain hatası)
- [backend/cmd/api/respond.go](../../backend/cmd/api/respond.go) — tutarlı `{"error":{"code","message"}}` zarfı
- [security-and-auth.md](security-and-auth.md) Girdi 2 (404'ün varlık sızdırmama gerekçesi)

---

## 6. Middleware ve context: kimlik isteğe nasıl taşınır? (401 vs 403)

**🗓️ Bağlam:** Faz 3'te `/api/admin` grubunu korurken — push öncesi güvenlik incelemesi bu grubun
kimlik doğrulamasız olduğunu bulgu olarak işaretlemişti.

**⚡ Kısa cevap:** Middleware, handler'ı saran bir handler'dır: isteği önce o görür, uygun değilse
zinciri **keser**. Doğrulanan kimliği `context`'e koyar; handler oradan okur.

**📖 Açıklama:**

```go
func requireAuth(v tokenVerifier) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // token yok/geçersiz → 401 yaz ve DÖN (next çağrılmaz = zincir kesildi)
            next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), p)))
        })
    }
}
```

Express karşılığı: `app.use((req, res, next) => { ...; next() })`. Go'da `req.user = ...` gibi
isteğe alan eklenemez; yerine **context** taşınır.

**Context anahtarı neden özel bir tip?** `context.WithValue(ctx, "user", p)` yazsaydık her paket
aynı `"user"` string'ini kullanıp değeri ezebilir/okuyabilirdi. `type principalKey struct{}`
dışarıya kapalı → bu anahtarı yalnızca bizim paketimiz üretebilir.

**401 vs 403:**

| Kod | Anlamı | Bizde |
|---|---|---|
| 401 Unauthorized | "Seni **tanımıyorum**" — token yok / geçersiz / süresi dolmuş | `requireAuth` (+ `WWW-Authenticate: Bearer` başlığı) |
| 403 Forbidden | "Seni tanıyorum ama buna **yetkin yok**" | `requireAdmin` (USER admin rotasına geldi) |

**Grup seviyesinde koruma:** `r.Route("/api/admin", func(r) { r.Use(requireAuth(..), requireAdmin); ... })`
— yeni bir admin rotası eklemek onu **otomatik** korur. Tek tek eklemek "unutursam açık kalır"
demekti. Test tüm admin rotalarını gezip token'sız 401, USER'la 403 bekliyor; `r.Use` satırı
silinince 5 rotanın 5'i de düşüyor.

**Rol ≠ sahiplik:** middleware kaba rol kontrolü yapar ("admin misin?"). "Bu yorum **senin** mi?"
gibi kaynak sahipliği kontrolü servis katmanında yapılır (IDOR — Faz 5).

**🔗 İlgili:**
- [backend/cmd/api/middleware.go](../../backend/cmd/api/middleware.go)
- [backend/cmd/api/router.go](../../backend/cmd/api/router.go) — `/api/admin` grubu
- [security-and-auth.md](security-and-auth.md) — Girdi 2 (BOLA/IDOR)

---

## 7. Ağırlıklı (Bayes) puan: neden basit ortalama sıralamaya yetmez?

**🗓️ Bağlam:** Faz 6'da "en iyi kitaplar" listesini yazarken. İlk entegrasyon testim yanlış bir
sezgiyi yakaladı — formül değil, ben yanılmıştım.

**⚡ Kısa cevap:** Her kitabı, sanki **m tane "katalog ortalaması" oyla** başlıyormuş gibi düşün.
Az oylu kitabın gerçek oyları bu sanal oyların yanında zayıf kalır → skoru katalog ortalamasına (C)
yakın durur. Çok oylu kitabın sanal oyları kaybolur → kendi ortalamasına yakın durur.

**📖 Açıklama:**

```
WR = (v / (v + m)) · R  +  (m / (v + m)) · C
      ↑ kendi ortalamasının ağırlığı   ↑ katalog ortalamasının ağırlığı
```

| Kitap | R | v | WR (m=10, C=7) |
|---|---|---|---|
| A — 1 kişi 10 verdi | 10 | 1 | 7.27 |
| B — 2000 kişi ortalama 8.5 | 8.5 | 2000 | 8.49 |

Basit ortalamada A birinci olurdu. "Bir kişinin 10 vermesi" zayıf bir kanıt; formül bunu sayıyla ifade ediyor.

**m nasıl seçilir?** m büyüdükçe az oylu kitaplar C'ye daha çok çekilir. Brief'teki m=100, 30 oylu
bir kitabı bile C'ye yapıştırır — küçük katalogda sıralama anlamsızlaşır. Bu yüzden 10 ile başladık
(`RATING_MIN_VOTES`), katalog büyüdükçe artırılır.

**C'nin etkisi — testin öğrettiği:** ilk testte yalnız iki kitap vardı: 30 kişi 9 vermiş, 1 kişi 10
vermiş. C ≈ 9.03 çıktı (oyların çoğu 9'luk kitaptan) ve 1 oylu kitap ÖNE geçti (9.12 > 9.01). Önce
bug sandım; değildi. C = 9 olan bir dünyada 9'luk kitap "ortalamanın altında", tek 10 ise "zayıf ama
olumlu" kanıt — formül tutarlı. Brief'in iddiası C her iki kitabın da altındayken geçerli, yani
gerçekçi bir katalogda. Test artık bunu açıkça kuruyor (40 oyla 5 alan arka plan kitabı → C ≈ 6.9).

**Neden skor saklanmıyor?** C globaldir: herhangi bir kitaba verilen tek oy C'yi, dolayısıyla
BÜTÜN kitapların skorunu değiştirir. Saklanan bir `weighted_score` kolonu her oyda bayatlar.
Sorgu anında hesaplamak ucuz (14 ms/20 bin kitap); yavaşlarsa cache (Faz 7).

**🔗 İlgili:**
- [backend/internal/ratings/score.go](../../backend/internal/ratings/score.go) — saf fonksiyon (spesifikasyon)
- [backend/sql/queries/ratings.sql](../../backend/sql/queries/ratings.sql) — `ListTopRated` (SQL'de aynı formül)
- `TestTopRatedRealDB` — SQL ↔ Go karşılaştırması

---

## 8. Cache-aside ve rate limit: neden paylaşılan state (Redis)?

**🗓️ Bağlam:** Faz 7'de (MVP'nin son fazı) top-rated'ı cache'lerken ve login'e sınır koyarken.
İki API örneğini yan yana çalıştırıp farkı ölçtük.

**⚡ Kısa cevap:** Cache'i ve sınır sayaçlarını **süreç belleğinde** tutarsan her API örneği kendi
kopyasını görür: sınır N değil N × örnek sayısı olur, cache tutarsızlaşır. Redis, bütün örneklerin
**ortak** gördüğü bellek.

**📖 Açıklama:**

**Cache-aside (en yaygın desen):**

```
okuma:  Redis'te var mı? → evet: dön (hit)
                         → hayır: DB'den hesapla → Redis'e yaz (TTL) → dön (miss)
yazma:  DB'ye yaz → COMMIT → Redis'teki anahtarı SİL (bir sonraki okuma taze hesaplar)
```

- **Neden commit'ten SONRA sil:** önce silersen, commit'i beklemeyen bir okuyucu eski veriyi
  hemen yeniden cache'e yazabilir.
- **Yine de kalan yarış:** yavaş bir okuyucu eski listeyi okur → biz sileriz → o eski listeyi yazar.
  Sonuç: en çok TTL (60 sn) bayatlık. Sıralama için kabul edilebilir; bu yüzden TTL = güvenlik ağı.
- **Cache hatası isteği düşürmez:** Redis çökerse miss sayılır, DB cevaplar. Cache bir hızlandırıcı,
  doğruluk kaynağı değil.

Ölçüm (20 bin kitap, 50 eşzamanlı): 327 → 2892 istek/sn; p95 236 → 27 ms.

**Rate limit — canlı deney (sınır 5/dk, 20 deneme iki örneğe sırayla):**

| Sınırlayıcı | Geçen | Neden |
|---|---|---|
| bellek içi | 10 | her örnek kendi 5'ini verdi |
| Redis | 5 | tek, paylaşılan sayaç (`INCR` atomik) |

**İki güvenlik detayı:**
- IP'yi `X-Forwarded-For`'dan OKUMUYORUZ: o başlığı istemci yazabilir; güvenseydik her istekte sahte
  IP yollayarak sınırı atlatırdı. Proxy arkasında (Faz 11) yalnız proxy'nin başlığına güvenilir.
- Redis arızasında sınırlayıcı **izin verir** (fail-open): aksi hâlde Redis kesintisi herkesin
  girişini kilitlerdi. Kaba kuvvete karşı asıl duvar bcrypt'in yavaşlığı; sınır ikinci hat.

**🔗 İlgili:**
- [backend/internal/ratings/cached.go](../../backend/internal/ratings/cached.go)
- [backend/internal/ratelimit/ratelimit.go](../../backend/internal/ratelimit/ratelimit.go)
- [docs/WHY_GO_VS_TS.md](../WHY_GO_VS_TS.md) §12
