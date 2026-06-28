# Öğrenme Notları — Sorduğum Kavramlar ve Cevapları

Bu dosya **kişisel hatırlama günlüğüm**. Proje boyunca sorduğum, iyi bilmediğim kavramları
burada topluyoruz. Amaç: sonradan dönüp okuyunca **hem cevabı hem de o soruyu neden sorduğumu
(bağlamı)** hatırlayabilmek.

Her girdi şu yapıda:

- **🗓️ Bağlam** — bunu neyi yaparken/neden sordum (gelecekteki ben için).
- **⚡ Kısa cevap** — tek-iki cümlede özü.
- **📖 Açıklama** — daha ayrıntılı, sade anlatım.
- **🔗 İlgili** — projedeki diğer dosyalardan ilgili kısımlar.

> Bu doküman yaşıyor: yeni bir şey sorduğumda buraya bir girdi daha ekleniyor.
> En yeni girdi en altta.

## İçindekiler

1. [Graceful shutdown nedir, neden önemli?](#1-graceful-shutdown-nedir-neden-önemli)
2. [stdlib (standart kütüphane) nedir?](#2-stdlib-standart-kütüphane-nedir)
3. [Graceful shutdown Express/Node ile de yapılır mı?](#3-graceful-shutdown-expressnode-ile-de-yapılır-mı)
4. [Testleri ayrı bir klasörde tutmak daha doğru olmaz mı?](#4-testleri-ayrı-bir-klasörde-tutmak-daha-doğru-olmaz-mı)

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
- [docs/GO_CONVENTIONS.md](GO_CONVENTIONS.md) §5 (server lifecycle: timeouts + graceful shutdown)
- [docs/WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) §8 (graceful shutdown & fail-fast config)
- [docs/phases/phase-01-minimal-go-api.md](phases/phase-01-minimal-go-api.md) Adım 7
- Bkz. aşağıda [Girdi 3](#3-graceful-shutdown-expressnode-ile-de-yapılır-mı) (Express ile farkı)

---

## 2. stdlib (standart kütüphane) nedir?

**🗓️ Bağlam:** Graceful shutdown'ı ve ilk sunucu kodunu anlatırken sürekli "standart
kütüphane" / "stdlib" diyordum (`net/http`, `encoding/json`, `log` hepsi oradan). Terimi tekrar
kullanınca "stdlib nedir, onu da öğrenebilir miyim?" diye sordum.

**⚡ Kısa cevap:** Dilin kendisiyle **hazır gelen**, kurulum gerektirmeyen paketler topluluğu.
Go'yu kurduğun an yüzlerce hazır paket de elinde.

**📖 Açıklama:**

İlk [main.go](../backend/cmd/api/main.go)'daki üç `import` (`net/http`, `encoding/json`, `log`)
**stdlib**. Bu yüzden [go.mod](../backend/go.mod)'da **hiç dış bağımlılık yok** — bir web
sunucusunu sıfır paket indirerek yazdık.

JS/Node ile fark:
- Node'un da stdlib'i var (`node:http`, `node:fs`...) ama görece **küçük**; pratikte hemen
  `npm install express/axios` dersin → `node_modules` şişer.
- **Go'nun stdlib'i "batteries included"** (her şey dahil): HTTP, JSON, kripto, test aracı, SQL
  arayüzü... kutudan çıkar, production kalitesinde.

| | stdlib | dış paket |
|---|---|---|
| Kurulum | gerekmez | `go get ...` |
| `go.mod`'da görünür mü | **hayır** | **evet** (go.mod + go.sum) |
| Örnek | `net/http`, `encoding/json`, `log` | `chi`, `pgx`, `sqlc` |

Bu projede sık göreceğim faydalı stdlib paketleri: `fmt`, `errors`, `context`, `net/http`,
`encoding/json`, `database/sql`, `crypto/*`, `log/slog`, `testing`, `time`, `os`, `io`, `signal`.

Keşfetmek için (kurulum yok):
```bash
go doc net/http            # paket özetini terminalde gösterir
go doc net/http.ServeMux   # belli bir tipin dökümanı
```
Web: **pkg.go.dev/std** — tüm stdlib listesi.

**🔗 İlgili:**
- [docs/WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) §4 (görünür HTTP), §13 (toolchain & DX)
- "Sebepsiz bağımlılık ekleme": dış paketi (Chi/pgx) gerçek ihtiyaç çıkınca ekleyeceğiz —
  [PROJECT_BRIEF.md](PROJECT_BRIEF.md) §14.5

---

## 3. Graceful shutdown Express/Node ile de yapılır mı?

**🗓️ Bağlam:** Graceful shutdown'ı öğrendikten sonra, JS/Node geçmişimden gelerek "dur, aklıma
geldi — bu birebir Express ile de yapılabiliyor değil mi?" diye sordum. Bu aslında bir **yargı
sorusu**ydu: "Go'ya özel bir şey mi, yoksa genel bir kavram mı?"

**⚡ Kısa cevap:** **Evet, neredeyse birebir.** Graceful shutdown Go'ya özel bir yetenek değil;
genel bir kavram. Express/Node de aynı doğrulukta yapar. Go'nun stdlib'i sadece *biraz daha az
uğraşla* bunu default idiom yapıyor.

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
[WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) §8'in "honest caveat"ının söylediği şey: *"a careful Node
service does exactly this."* Hedefim "Go kazandı" demek değil; **"bu fark benim için gerçekten
önemli mi?"** diye sorabilmek.

**🔗 İlgili:**
- [Girdi 1](#1-graceful-shutdown-nedir-neden-önemli) (graceful shutdown'ın kendisi)
- [docs/WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) §8 ve §15 ("TS/JS nerede daha iyi/eşit")

---

## 4. Testleri ayrı bir klasörde tutmak daha doğru olmaz mı?

**🗓️ Bağlam:** İlk testimizi (`main_test.go`) yazmıştık ve `main.go`'nun **tam yanında**
duruyordu. JS/TS'teki "testleri `tests/` veya `__tests__/` altında topla" alışkanlığımdan dolayı
"testleri ayrı bir yerde klasörlemek daha doğru bir yaklaşım olmaz mı?" diye sordum.

**⚡ Kısa cevap:** Go'da **hayır — tam tersi idiomatik.** Test dosyası, test ettiği kodun **tam
yanında** durur; aynı klasör, `xxx_test.go` adıyla. Bu dağınıklık değil, Go'nun standart kuralı.

**📖 Açıklama:**

```
backend/internal/books/
  books.go
  books_test.go        ← test, kodun yanında
  repository.go
  repository_test.go
```

Go neden böyle yapıyor? (4 sebep)
1. **Toolchain bunu bekler.** `go test ./...` her paketteki `_test.go`'ları bulup o paketle
   derler. Kural dile gömülü — config tartışması yok.
2. **İç (unexported) koda erişim.** `_test.go` aynı pakette olduğu için küçük harfle başlayan
   (dışa kapalı) fonksiyonları test edebilir — bizim `handleHealth` gibi. Ayrı klasörde sadece
   **public** API'yi test edebilirdin.
3. **Build'e karışmaz.** `_test.go` son eki özeldir: `go build` onları **otomatik dışlar**,
   sadece `go test` derler. Yan yana durması binary'yi şişirmez.
4. **Yakınlık.** Kodun yanında testi görürsün; senkronda kalır, "kod yazarken test" kolaylaşır.

JS ile fark: orada test runner (jest/vitest) yapılandırılabilir olduğu için testi istediğin yere
koyabilirsin (`__tests__/`, ayrı `test/`...). İki stil de yaşar. Go'nun tek kararlı cevabı:
**yanına koy, `_test.go`.**

Go'da ayrı klasör ne zaman? (nüans)
- **Unit testler** → her zaman kodun yanında.
- **Integration/e2e testler** → bazen üst seviye `test/` klasöründe.
- **`testdata/`** → test fixture dosyaları; Go aracı bu isimli klasörü derlemede **yok sayar**.

Aynı klasörde iki test-paketi seçeneği:
- `package main` (bizimki) → **white-box**, içeriye erişir (`handleHealth`'i çağırdık).
- `package main_test` → **black-box**, sadece public API'yi test eder. İkisi de aynı dizinde.

**🔗 İlgili:**
- [backend/cmd/api/main_test.go](../backend/cmd/api/main_test.go) (ilk testimiz)
- [docs/GO_CONVENTIONS.md](GO_CONVENTIONS.md) §8 (test: table-driven, std `testing`, `-race`)
- [docs/WHY_GO_VS_TS.md](WHY_GO_VS_TS.md) §13 (zero-config toolchain)
