# Concurrency 1 — Goroutine, bloklamak ve zamanlayıcı (JS async/await'ten gelenler için)

> Öğrenme planının [§3](../LEARNING_PLAN.md) 1. ve 2. oturumu. Sırada: 3) mutex ve yarış,
> 4) channel/`select`, 5) context, 6) eşzamanlılığı test etmek.
> Bitirince: [01-concurrency-test.md](01-concurrency-test.md) (önce dersi oku, teste
> koda bakmadan gir).

## 0. Bu dersin tek cümlesi

**JavaScript'te eşzamanlılık dilin sözdizimindedir; Go'da çalışma zamanındadır.**
JS'te `async`/`await` yazmadan eşzamanlı kod yazamazsın. Go'da tam tersi: kodu düz ve
bloklayıcı yazarsın, eşzamanlılığı zamanlayıcı halleder — hatta hiç `go` yazmasan bile
kodun eşzamanlı çalışabilir. BookHub'da tam olarak bu oldu.

---

## 1. "Renkli fonksiyon" problemi (JS'in asıl bedeli)

JS'te bir fonksiyon ya "normal" ya "async"tir. `await` kullanmak için fonksiyonun `async`
olması gerekir; onu çağıran da `await` etmek isterse **o da** `async` olmak zorundadır. Bu
zincir yukarı doğru yayılır — renk bulaşır:

```js
// JS: bir satır için üç fonksiyon da async olmak ZORUNDA
async function getUser(id) { return (await db.query("...")).rows[0]; }
async function handler(req) { const u = await getUser(req.id); return render(u); }
async function route(req)  { return await handler(req); }
```

`getUser` bir gün DB'ye gitmeye başladıysa, çağrı zincirindeki herkesin imzası değişir.
Bob Nystrom bunu "What Color is Your Function?" yazısında anlattı; JS/Python/C# gibi
`async`/`await` dillerinin ortak bedeli.

Go'da böyle bir bölünme **yok**:

```go
// Go: hiçbir imza "eşzamanlı" olduğunu söylemiyor; hepsi düz fonksiyon
func (r *Repository) UserByID(ctx context.Context, id int64) (db.User, error) { … }
func handleMe(svc authService) http.HandlerFunc { … }
```

`UserByID` içeride ağa gidip 5 ms bekler. Çağıranın imzası değişmez, `await` yoktur,
`Promise` yoktur. **Fonksiyonun rengi yoktur.**

Peki bekleyen kod programı nasıl kilitlemiyor? Cevap 3. bölümde.

---

## 2. Goroutine nedir, ne değildir

```go
go f()   // f'i ayrı bir akışta başlat, SONUCUNU BEKLEMEDEN devam et
```

| | JS `Promise`/task | Go goroutine |
|---|---|---|
| Nerede koşar | Tek OS thread'i, olay döngüsü | Birden çok OS thread'ine dağıtılır |
| Kaç tane olur | On binlerce (hepsi aynı thread) | **Milyonlarca** (başlangıç yığını ~2–8 KB, büyüyebilir) |
| Nasıl sıraya girer | Microtask/macrotask kuyruğu | Go zamanlayıcısı (M:N), tercihli (preemptive) |
| Paralellik | **Yok** (worker thread hariç) | **Var** (`GOMAXPROCS` çekirdek sayısı kadar) |
| Bekleyince | Thread serbest kalır, kuyruk işler | Goroutine **park edilir**, thread başka goroutine'e geçer |
| Sonucu alma | `await promise` | channel, `sync.WaitGroup`, `errgroup` |

Goroutine bir thread **değildir**: thread pahalıdır (1–8 MB yığın, çekirdek zamanlayıcısı),
goroutine ucuzdur ve Go'nun kendi zamanlayıcısı onları thread'lere dağıtır. Buna **M:N
zamanlama** denir: M goroutine, N OS thread'i.

**Concurrency ≠ parallelism** (Rob Pike): eşzamanlılık *aynı anda birden çok işi ele alacak
şekilde yapılandırmak*, paralellik *aynı anda birden çok işi yürütmek*. Tek çekirdekli bir
makinede Go programı yine eşzamanlıdır, ama paralel değildir.

---

## 3. Neden Go'da "bloklamak serbest"

JS'te `fs.readFileSync` ya da 250 ms CPU yakan bir döngü yazarsan **tüm süreç** durur: tek
thread var, olay döngüsü ilerlemez, diğer istekler bekler. Bu yüzden JS'te her I/O
non-blocking API ister ve dil seni `await`e zorlar.

Go'da `time.Sleep`, `db.Query`, `io.Read` bloklar — **ama yalnız o goroutine'i.** Zamanlayıcı
şunu yapar:

1. Goroutine bir syscall'a ya da channel beklemesine girer.
2. Zamanlayıcı onu **park eder** ve o OS thread'ine **başka** bir goroutine koyar.
3. Sonuç geldiğinde goroutine yeniden koşmaya hazır kuyruğuna alınır.

Yani "bekleyen" kod yazmanın maliyeti bir thread değil, bir goroutine. Bu yüzden Go'da
`readFileSync` diye ayrı bir API yok — tek bir `os.ReadFile` var ve o bloklar.

### BookHub'da bunun bilinçli bir örneği

`backend/internal/auth/service.go` — bcrypt karşılaştırması (`bcryptCost = 12`) **~250 ms
CPU bloklar** ve bu kasıtlı: yavaşlık, parola denemesini pahalı kılan güvenlik özelliğinin
kendisi. Üstelik kullanıcı bulunamadığında bile sahte bir hash'le aynı karşılaştırma yapılır
(`dummyHash`), ki cevap süresi "bu e-posta kayıtlı mı" bilgisini sızdırmasın.

Aynı satır Node'da tek thread'i 250 ms kitler; o yüzden Node'da bcrypt'in async sürümü
(libuv thread pool'una atan) kullanılır. Go'da böyle bir ayrım gerekmez.

---

## 4. BookHub'da eşzamanlılık nerede — hiç `go` yazmadan

Bu bölüm dersin kalbi: **kodun zaten eşzamanlı.**

### 4.1 Her istek kendi goroutine'inde

`net/http`'nin sunucusu her bağlantı için `go c.serve(...)` çağırır. Yani
`backend/cmd/api/` altındaki **her handler eşzamanlı çalışır** ve sen bunu hiç yazmadın.
Sonuçları:

- Handler'ların paylaştığı her şey (limiter, cache, `*pgxpool.Pool`, `deps`) **goroutine-safe
  olmak zorunda.** Bunlar öyle tasarlandı.
- `w http.ResponseWriter` yalnız o isteğe aittir; handler döndükten sonra kullanılamaz —
  `handleLogout`'ta iptali `context.WithoutCancel` ile sürdürürken cevaba dokunmamamızın
  nedeni bu.
- Testlerin `-race` ile koşması bu yüzden anlamlı: CI'da `go test -race` var.

### 4.2 Paylaşılan durum: mutex

`backend/internal/ratelimit/ratelimit.go:41` — bellek içi limiter bir `sync.Mutex` taşıyor;
`Allow` metodu sayaca dokunurken kilitler (`:59-60`). Neden: aynı anda gelen 50 istek = 50
goroutine = aynı map'e eşzamanlı yazma. Mutex olmasaydı bu bir **veri yarışı** olurdu ve Go
çalışma zamanı map'te eşzamanlı yazma gördüğünde programı doğrudan çökertir
(`concurrent map writes`).

### 4.3 Süreç başına bir kez: `sync.OnceValue`

`internal/auth/service.go:152` — `dummyHash`, `sync.OnceValue` ile sarılı. Onlarca goroutine
aynı anda çağırsa bile bcrypt **bir kez** koşar, gerisi hazır sonucu alır. JS'te bu "modül
seviyesinde bir promise'i paylaş" deseninin karşılığı.

### 4.4 Channel + `select`: kapanış

`backend/cmd/api/main.go`:

```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM) // :52
serveErr := make(chan error, 1)                                                          // :135
go func() { serveErr <- srv.ListenAndServe() }()                                         // sunucu ayrı akışta
select {                                                                                  // :145
case <-ctx.Done():          // Ctrl+C / SIGTERM geldi → düzgün kapan
case err := <-serveErr:     // sunucu hiç açılamadı (port dolu) → hatayı bildir
}
```

`select` = "hangisi önce gelirse onu al". `if` yetmez çünkü **ikisi de gelecekte** ve hangisinin
önce geleceği bilinmiyor. JS karşılığı `Promise.race([...])`.

### 4.5 `select` + timer: `sleepCtx`

`backend/cmd/api/auth.go:311` — çıkışta iptal denemeleri arasında beklerken:

```go
select {
case <-ctx.Done(): return false   // 5 sn'lik bütçe doldu: beklemenin anlamı yok
case <-t.C:        return true    // süre doldu: yeniden dene
}
```

Düz `time.Sleep` bütçe bittikten sonra da beklemeye devam ederdi. Bu, `select`in en yaygın
gerçek kullanımı: **bir işi bir iptal sinyaliyle yarıştırmak.**

### 4.6 Aynı anda başlatma hilesi (testte)

`internal/auth/repository_integration_test.go:496` — 20 goroutine aynı refresh token'ı
döndürmeye çalışıyor. Hepsinin gerçekten aynı anda başlaması için:

```go
start := make(chan struct{})
for i := range calls {
    calls[i] = goAsync(func(ctx context.Context) error {
        <-start                        // kapalı kapıda bekle
        _, err := s.Refresh(ctx, tok)
        return err
    })
}
close(start)                           // kapıyı aç: 20'si birden koşar
```

`close(channel)` **tüm** bekleyenleri aynı anda serbest bırakır (kapalı channel'dan okuma
hemen döner). Bu, yarış testlerinin klasik "yayın" (broadcast) deseni.

---

## 5. İki ayrı "yarış" — karıştırılması en kolay şey

| | Veri yarışı (data race) | Mantıksal yarış (race condition) |
|---|---|---|
| Ne | İki goroutine aynı belleğe senkronizasyonsuz erişir | İki işlem birbirinin kararını geçersizleştirir |
| Nerede | Aynı süreç içinde | Genelde veritabanında, farklı süreçler arasında bile |
| Yakalayan | `go test -race`, çalışma zamanı | Hiçbir araç otomatik yakalamaz — tasarım işi |
| BookHub örneği | Limiter'ın map'i (mutex ile çözüldü) | İki eşzamanlı `/refresh` aynı token'ı harcamaya çalışır → `FOR UPDATE`, `used_at`, reuse tespiti ile çözüldü |

`-race` temiz demek "yarış yok" demek **değildir**; yalnız "bellek düzeyinde yarış yok" demek.
BookHub'ın Faz 5 ve Faz 13'te uğraştığı yarışların neredeyse tamamı ikinci türden.

---

## 6. Klasik tuzaklar

1. **`main` biterse program biter.** Beklenmeyen goroutine'ler öldürülür — JS'te olay döngüsü
   bekler, Go'da beklemez. `sync.WaitGroup` ya da channel ile beklemek gerekir.
2. **Goroutine sızıntısı.** Kimsenin okumadığı bir channel'a yazan goroutine sonsuza dek
   parkta kalır. Dave Cheney'nin kuralı: *"Nasıl duracağını bilmeden goroutine başlatma."*
   BookHub'da `sleepCtx`'in bütçesi ve `lockWait` timeout'ları bu yüzden var.
3. **Döngü değişkeni tuzağı — artık YOK.** Go 1.22'den beri `for` değişkeni her iterasyonda
   yeni; eski `for i := range x { go func(){ use(i) }() }` hatası tarihe karıştı (bu repo
   Go 1.26). Eski blog yazılarında bu tuzağı görürsen: geçersiz.
4. **`defer` goroutine'e değil fonksiyona bağlıdır** — `go` ile başlatılan fonksiyonun kendi
   `defer`'i vardır.
5. **Panik yayılmaz.** Bir goroutine'deki panik tüm programı çökertir; recover o goroutine'in
   içinde olmalı (chi'nin `Recoverer` middleware'i bunu her istek goroutine'inde yapar).

---

## 7. Kendi yazacağın deneyler (asistan yazmayacak)

`/tmp/go-deney/` altında `go mod init deney` ile başla. **Her deneyde çıktıyı çalıştırmadan
tahmin et**, sonra koş.

1. **`main` beklemiyor:** 5 goroutine başlat, her biri bir şey yazdırsın, `main` hemen bitsin.
   Sonra `sync.WaitGroup` ekle. → Tuzak 1.
2. **Veri yarışı:** bir `counter`'ı 1000 goroutine ile artır, `sync.WaitGroup` ile bekle,
   sonucu yazdır. `go run .` ve `go run -race .` çıktılarını karşılaştır. Sonra `sync.Mutex`
   ekle. → 4.2 ve 5. bölüm.
3. **Bloklamak ucuz:** 3 goroutine, her biri `time.Sleep(2*time.Second)`; toplam süreyi ölç
   (`time.Since`). 6 sn mi 2 sn mi? Sonra `time.Sleep` yerine `for {}` (CPU yakan döngü) koy
   ve farkı gör. → 3. bölüm.
4. **`select` yarışı:** bir `time.After(100*time.Millisecond)` ile bir channel'ı yarıştır;
   channel'a 50 ms sonra yazan bir goroutine ekle. Kim kazanır? Süreleri ters çevir.
5. **Kapıyı aç:** 5 goroutine `<-start` ile beklesin, `close(start)` ile hepsi birden koşsun;
   her biri kendi numarasını yazdırsın. Çıktı sırası her koşuda aynı mı? → 4.6 ve
   "zamanlayıcı sıra garantisi vermez".

---

## 8. Kaynaklar (sıralı, süre tahminleriyle)

### Önce bunlar (temel, ~2 saat)

1. **A Tour of Go — Concurrency** (go.dev/tour/concurrency/1) · ~40 dk · tarayıcıda çalıştırılabilir.
   11 adım: goroutine, channel, buffered channel, `select`, mutex.
2. **Rob Pike — "Concurrency is not Parallelism"** (video, ~30 dk) · YouTube'da bu başlıkla ara;
   Go blog'undaki `go.dev/blog/waza` yazısı slaytlara bağlantı verir. Go'nun eşzamanlılık
   felsefesini anlatan tek en iyi kaynak.
3. **Effective Go — Concurrency** (go.dev/doc/effective_go#concurrency) · ~30 dk ·
   "Belleği paylaşarak iletişim kurma; iletişim kurarak belleği paylaş" cümlesinin kaynağı.

### Sonra bunlar (derinleşme)

4. **Bob Nystrom — "What Color is Your Function?"** (journal.stuffwithstuff.com) · ~20 dk ·
   JS'ten gelen biri için 1. bölümün uzun hâli. Go'yu değil, `async`/`await`in bedelini anlatır.
5. **Go Blog — "Go Concurrency Patterns: Context"** (go.dev/blog/context) · 5. oturumun hazırlığı.
6. **Go Blog — "Go Concurrency Patterns: Pipelines and cancellation"** (go.dev/blog/pipelines) ·
   channel'ların gerçek kullanımı; uzun ama değerli.
7. **Go Blog — "Introducing the Go Race Detector"** (go.dev/blog/race-detector) · ~15 dk ·
   `-race` ne görür, ne görmez.
8. **Dave Cheney — "Never start a goroutine without knowing how it will stop"**
   (dave.cheney.net) · ~10 dk · sızıntıların tek maddelik özeti.
9. **William Kennedy — "Scheduling in Go" 1–3** (ardanlabs.com/blog) · ~1 saat · M:N
   zamanlayıcının içi: P/M/G, work stealing. "Bloklamak neden serbest" sorusunun donanım
   seviyesindeki cevabı.

### Kitaplar

- **Katherine Cox-Buday, "Concurrency in Go"** (O'Reilly) — konunun standart kitabı. Generics
  öncesi yazıldı, desenler hâlâ geçerli. Kısa (~240 sayfa).
- **Jon Bodner, "Learning Go"** (O'Reilly, 2. baskı) — eşzamanlılık bölümü JS'ten gelen biri
  için en anlaşılır girişlerden biri; kitabın geri kalanı da bu projeye birebir denk düşüyor.

### Referans (okumak için değil, bakmak için)

- **Go Memory Model** (go.dev/ref/mem) — "happens before" tanımları. Şimdi okuma; 3. oturumda
  "mutex neden yeterli" sorusuna cevap ararken bak.
- **`sync` paket dokümantasyonu** (`go doc sync`) — `Mutex`, `RWMutex`, `WaitGroup`, `Once`,
  `OnceValue`.

## 9. Bu dersi bitirdiğinde cevaplayabilmen gerekenler

- Bir Go fonksiyonuna bakıp "bu eşzamanlı mı" diye soramamanın nedeni ne, ve bu iyi bir şey mi?
- BookHub'da hiç `go` yazmadık; eşzamanlılık nereden geliyor ve bu hangi tasarım kısıtını doğurdu?
- 250 ms bloklayan bcrypt neden Go'da sorun değil, Node'da sorun?
- `select` yerine `if` neden kullanılamaz?
- `-race` temiz çıktı: hangi yarışlar hâlâ mümkün?

Cevapların testi: [01-concurrency-test.md](01-concurrency-test.md).
