# Concurrency 1 — Test

> Dersi okumadan girme: [01-concurrency-goroutines.md](01-concurrency-goroutines.md).
> **Kural: koda, derse ve internete bakmadan cevapla.** Cevap anahtarı en altta — oraya
> bakmadan önce her soruya bir şey yaz, "bilmiyorum" da geçerli bir cevap (nerede boşluk
> olduğunu gösterir, onu gizlemekten iyidir).
>
> 18 soru, ~40 dk. Bölümler: A) kavram · B) çıktı tahmini · C) bu repo · D) tuzak avı.
> Puanlama ve ne yapman gerektiği en altta.

---

## A. Kavram (6 soru)

**A1.** JS'te `await` kullanmak için fonksiyonun `async` olması gerekir ve bu zincir yukarı
doğru yayılır. Go'da bu yayılma neden yok? Cevabında "goroutine" kelimesini kullanmadan
açıklamayı dene.

**A2.** Goroutine'i "hafif thread" diye tanımlamak neden eksik? M:N zamanlama ne demek?

**A3.** "Concurrency, parallelism değildir." Tek çekirdekli bir makinede BookHub'ın API'si
hâlâ eşzamanlı mıdır? Neden?

**A4.** Node'da 250 ms CPU yakan senkron bir fonksiyon neden "tüm sunucuyu durdurur" da,
Go'da aynı fonksiyon yalnız bir isteği yavaşlatır? Zamanlayıcının bu sırada yaptığı şeyi
iki adımda anlat.

**A5.** Şu iki ifade arasındaki farkı bir cümleyle söyle: "bu kod bloklar" ve "bu kod
programı bloklar".

**A6.** `go f()` çağrısı ne **döndürür**? Sonucunu almanın üç yolunu say.

---

## B. Çıktı tahmini (5 soru — çalıştırmadan)

**B1.** Ne yazdırır? (Sıra dahil.)

```go
func main() {
	for i := range 3 {
		go fmt.Println("goroutine", i)
	}
	fmt.Println("main bitti")
}
```

**B2.** Bu programın toplam süresi yaklaşık ne olur — ~1 sn mi, ~3 sn mi? Neden?

```go
func main() {
	start := time.Now()
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() { defer wg.Done(); time.Sleep(time.Second) }()
	}
	wg.Wait()
	fmt.Println(time.Since(start).Round(100 * time.Millisecond))
}
```

**B3.** Aşağıdaki program `go run .` ile ne yazdırır? `go run -race .` ile ek olarak ne olur?

```go
func main() {
	count := 0
	var wg sync.WaitGroup
	for range 1000 {
		wg.Add(1)
		go func() { defer wg.Done(); count++ }()
	}
	wg.Wait()
	fmt.Println(count)
}
```

**B4.** Ne yazdırır ve **neden** — "hazır" mı, "zaman aşımı" mı?

```go
func main() {
	ch := make(chan string)
	go func() { time.Sleep(200 * time.Millisecond); ch <- "hazır" }()
	select {
	case v := <-ch:
		fmt.Println(v)
	case <-time.After(100 * time.Millisecond):
		fmt.Println("zaman aşımı")
	}
}
```

**B5.** Kaç satır yazdırır? Program biter mi, asılı kalır mı?

```go
func main() {
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range 3 {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; fmt.Println("koştu", i) }()
	}
	wg.Wait()
	close(start)
}
```

---

## C. Bu repo (4 soru)

**C1.** BookHub'ın kodunda `go` anahtar kelimesi neredeyse hiç geçmiyor. Buna rağmen
`handleGetBook` aynı anda 50 kez çalışabilir. Bunu kim sağlıyor, ve bu gerçek `deps`
struct'ındaki alanlar (limiter, cache, pool) üzerinde hangi zorunluluğu doğuruyor?

**C2.** `internal/auth/service.go`'da parola karşılaştırması ~250 ms bloklar **ve** kullanıcı
bulunamadığında sahte bir hash'le aynı karşılaştırma yapılır. İkinci kısmın amacı ne, ve bu
"bloklamak serbest" ilkesiyle nasıl birleşiyor?

**C3.** `cmd/api/auth.go`'daki `sleepCtx` neden `time.Sleep` değil de `select` kullanıyor?
`time.Sleep` kullanılsaydı çıkışta hangi somut davranış bozulurdu?

**C4.** Reuse testinde 20 goroutine `<-start` ile bekliyor, sonra `close(start)` çağrılıyor.
`close` yerine `start <- struct{}{}` yazsaydık ne olurdu?

---

## D. Tuzak avı (3 soru)

**D1.** Aşağıdaki kod eski Go blog yazılarında "klasik hata" diye geçer. Go 1.22 sonrasını kullanan
bu repoda hâlâ hata mı? Neden?

```go
for _, id := range ids {
	go func() { process(id) }()
}
```

**D2.** Bu fonksiyonda bir goroutine sızıntısı var. Nerede ve neden?

```go
func first(urls []string) string {
	ch := make(chan string)
	for _, u := range urls {
		go func() { ch <- fetch(u) }()
	}
	return <-ch // ilk cevap yeter
}
```

**D3.** `go test -race ./...` tamamen temiz. Aşağıdakilerden hangileri hâlâ mümkün?
(Birden fazla olabilir; her biri için tek cümle gerekçe.)

a) İki eşzamanlı istek aynı refresh token'ı harcar ve kullanıcı oturumdan düşer
b) Bir map'e eşzamanlı yazma yüzünden program çöker
c) İki admin aynı kitabı aynı anda onaylar ve audit'e iki kayıt düşer
d) Bir goroutine sonsuza dek channel'da bekler

---

---

---

# CEVAP ANAHTARI

> Buraya bakmadan önce her soruya bir şey yazdın, değil mi?

## A

**A1.** Çünkü JS'te bekleme *dilin* bir özelliği: `await` bir sözdizimsel işaret ve yalnız
`async` işaretli bir fonksiyon gövdesinde geçerli; fonksiyon da beklemeyi çağıranına
"Promise döndürerek" devrediyor. Go'da bekleme *çalışma zamanının* işi: fonksiyon sonucunu
doğrudan döndürür, beklerken çalışma zamanı onu kenara alır. İmzada işaretlenecek bir şey
olmadığı için yayılacak bir "renk" de yok.
*(Tam puan: "JS'te bekleme çağıranın sorunu, Go'da çalışma zamanının sorunu" fikri.)*

**A2.** Eksik, çünkü goroutine bir OS thread'i değildir: thread'i çekirdek zamanlar, yığını
megabaytlarla ölçülür; goroutine'i Go'nun kendi zamanlayıcısı zamanlar, yığını kilobaytla
başlar ve gerektikçe büyür. **M:N**: M goroutine'i N OS thread'ine dağıtmak — bu yüzden
yüz binlerce goroutine mümkün, yüz binlerce thread değil.

**A3.** Evet, hâlâ eşzamanlıdır. Eşzamanlılık *yapıyla* ilgilidir: program birden çok işi
birbirinden bağımsız ilerleyecek şekilde kurgulanmıştır. Paralellik *yürütmeyle* ilgilidir ve
donanım ister. Tek çekirdekte goroutine'ler sırayla koşar, ama yine bağımsızdır.

**A4.** Node tek thread'de çalışır ve olay döngüsü bir sonraki işe ancak mevcut senkron blok
bitince geçer; 250 ms CPU o kuyruğun tamamını bekletir. Go'da: (1) zamanlayıcı, uzun süre
koşan goroutine'i zaman aşımıyla **tercihli olarak** araya girip (async preemption) sıradan
alır, ve I/O beklemesinde onu **park eder**; (2) boşalan OS thread'ine koşmaya hazır başka bir
goroutine yerleştirir. Ayrıca `GOMAXPROCS` kadar thread paralel koşar, yani başka çekirdek
zaten başka istekle meşguldür.

**A5.** "Bu kod bloklar" = çağıran goroutine ilerlemez. "Bu kod programı bloklar" = başka
hiçbir iş ilerlemez. Go'da ilki normaldir, ikincisi neredeyse hiç olmaz; JS'te ikisi aynı şeydir.

**A6.** **Hiçbir şey döndürmez** — `go f()` bir ifade değil, deyimdir; değeri yoktur (bu
yüzden `x := go f()` derlenmez). Sonucu almanın yolları: (1) channel'a yazdırmak, (2)
`sync.WaitGroup` ile bitmesini bekleyip paylaşılan bir değişkeni okumak (mutex ya da
"bitmiştir, artık tek okuyucu var" garantisiyle), (3) `golang.org/x/sync/errgroup` gibi bir
sarmalayıcı. *(Bonus: `context` iptal için, sonuç için değil.)*

## B

**B1.** Neredeyse kesin olarak **yalnız `main bitti`**. `main` dönünce program biter ve
goroutine'lerin koşma şansı olmaz. Bazı koşularda bir-iki satır sızabilir — çıktı belirsizdir,
garanti edilen tek şey `main bitti`. *(Bu belirsizlik doğru cevabın parçası.)*

**B2.** **~1 sn.** Üç `Sleep` paralel ilerler; `Sleep` goroutine'i park eder, thread'i meşgul
etmez. Toplam ~3 sn olması için işlerin sırayla koşması gerekirdi.

**B3.** `go run .`: 1000'den **küçük** ve her koşuda değişen bir sayı (ör. 947, 983); `count++`
atomik değil — oku-artır-yaz üçlüsü iç içe geçer. `-race` ile: aynı bozuk sayı **artı** bir
`WARNING: DATA RACE` raporu (hangi goroutine'ler, hangi satırlar) ve sıfır olmayan çıkış kodu.

**B4.** **`zaman aşımı`**. `select` ilk hazır olanı alır: 100 ms'lik timer, 200 ms'lik
goroutine'den önce ateşlenir. *(Yan etki: o goroutine `ch <- "hazır"` satırında sonsuza dek
bekler — okuyan kimse kalmadı. Bu bir sızıntı; D2'nin aynısı.)*

**B5.** **Hiçbir şey yazdırmaz ve program KİLİTLENİR** (deadlock). `wg.Wait()` goroutine'lerin
bitmesini bekler, goroutine'ler `<-start`ta bekler, `close(start)` ise `Wait`ten **sonra**
yazıldığı için hiç çalışmaz. Go çalışma zamanı tüm goroutine'ler uykudaysa bunu fark edip
`fatal error: all goroutines are asleep - deadlock!` ile çöker. Doğru sıra: `close(start)`
önce, `wg.Wait()` sonra. *(Repodaki testte sıra doğru: `close(start)` döngüden hemen sonra.)*

## C

**C1.** `net/http`'nin sunucusu: her bağlantı için `go c.serve(...)`. Doğurduğu zorunluluk:
`deps` içindeki her şey **birden çok goroutine tarafından aynı anda kullanılabilir olmalı**
(goroutine-safe). `pgxpool` havuzu, Redis istemcisi ve bellek içi limiter (mutex'li) bu yüzden
öyle seçildi/yazıldı; handler'lar paylaşılan durumu isteğe özel değişkenlerde değil, bu
goroutine-safe nesnelerde tutuyor.

**C2.** Amaç **zamanlama saldırısını** (timing-based user enumeration) kapatmak: kullanıcı
yoksa hemen dönseydik cevap ~1 ms'de gelir, varsa ~250 ms; saldırgan yalnız süreye bakarak
hangi e-postaların kayıtlı olduğunu çıkarırdı. İki yolu da aynı maliyete eşitliyoruz.
"Bloklamak serbest" ilkesiyle birleşmesi: bu 250 ms'yi *bilerek* harcayabiliyoruz, çünkü
yalnız o isteğin goroutine'ini park ediyor — diğer istekler etkilenmiyor. Node'da aynı tercih
tüm sunucuyu kilitlerdi.

**C3.** Çünkü beklerken **iptal edilebilir** olması gerekiyor: `select` hem timer'ı hem
`ctx.Done()`'ı dinler. `time.Sleep` kullanılsaydı, çıkışın 5 saniyelik bütçesi dolduktan sonra
bile bekleme sürer, yeniden denemeler bütçeyi aşar ve handler cevabı gecikirdi — yani
"istemciyi 5 saniyeden fazla bekletme" garantisi kırılırdı.

**C4.** `start <- struct{}{}` **yalnız bir** goroutine'i serbest bırakır (channel'a bir değer
yazılır, onu bir okuyan alır). Diğer 19'u beklemeye devam eder; testin amacı olan "hepsi aynı
anda" kurgusu çöker ve test `wg.Wait()`/timeout'ta asılır. `close` ise tüm bekleyenleri aynı
anda uyandırır — kapalı channel'dan okuma hemen sıfır değerle döner. Bu yüzden "yayın"
(broadcast) için channel kapatılır, değer yazılmaz.

## D

**D1.** **Artık hata değil.** Go 1.22'den beri `for` döngüsünün değişkeni her iterasyonda
yeniden tanımlanıyor, yani her goroutine kendi `id` kopyasını görür. Bu repo 1.22'den yeni bir Go kullanıyor
(`backend/go.mod`), dolayısıyla güvenli. Go 1.21 ve öncesinde tüm goroutine'ler
aynı değişkeni paylaşır ve çoğu "son eleman"ı görürdü. *(Eski blog yazılarını okurken bu
tuzağın tarihe karıştığını bil.)*

**D2.** Sızıntı `ch <- fetch(u)` satırında: `first`, **tek** bir değer okuyup dönüyor;
channel buffer'sız olduğu için geri kalan N−1 goroutine yazmayı bekleyerek sonsuza dek
parkta kalır (`fetch` bitse bile). Çözümler: `ch` buffer'lı yapmak (`make(chan string, len(urls))`),
ya da `context` ile iptal edip yazmayı `select` altında yapmak. Kural: *nasıl duracağını
bilmeden goroutine başlatma.*

**D3.** **a, c, d mümkün; b değil.**
- **a) mümkün** — bu bir *mantıksal* yarıştır, bellekte değil veritabanında olur; `-race`
  görmez. BookHub bunu SQL ile çözüyor (`used_at`, `FOR UPDATE`, reuse tespiti).
- **b) mümkün değil** — eşzamanlı map yazımı bellek düzeyinde bir yarıştır; `-race` onu
  yakalar (ayrıca çalışma zamanı `concurrent map writes` ile çöker). Temiz `-race` bunu dışlar.
  *(Tam dürüst cevap: `-race` yalnız o koşuda **gerçekleşen** erişimleri görür; hiç tetiklenmeyen
  bir yol raporlanmaz. Yani "dışlar" derken "o kod yolu testte koştuysa" koşulu var.)*
- **c) mümkün** — iki transaction'ın sırası; `-race`'in alanı dışında. Repo bunu
  `FOR UPDATE` + durum makinesiyle çözüyor.
- **d) mümkün** — sızıntı bir yarış değil; `-race` sızıntı aramaz.

---

## Puanlama ve sonrası

Her soru 1 puan (D3 için 4 şıktan en az 3'ü doğruysa 1 puan).

| Puan | Ne demek | Ne yap |
|---|---|---|
| 15–18 | Model oturmuş | 3. oturuma geç (mutex ve yarış), §7 deneylerinden 2 ve 5'i yine yap |
| 10–14 | Temel var, kenarlar eksik | Yanlış cevapların bölümünü tekrar oku, §7'deki ilgili deneyi **yaz** |
| 5–9 | Parçalar bağlanmamış | Tour of Go concurrency (11 adım) + Pike'ın videosu, sonra testi tekrar |
| 0–4 | Sıfırdan | Yalnız 1–3. bölümleri oku, §7'deki 1–3. deneyleri yaz, sonra yalnız A ve B bölümlerini çöz |

Hangi soruda yanıldıysan, o kavramı `/not` ile öğrenme günlüğüne ekle — özellikle "yanlış
tahmin ettiğim şey" kısmını yaz. Unutmaya karşı asıl işe yarayan not o.

**Tekrar takvimi:** bu testi 2 gün, 7 gün ve 21 gün sonra **yalnız A ve D bölümlerinden**
yeniden çöz (B'yi hatırlamak kolaydır, A ve D anlamayı ölçer).
