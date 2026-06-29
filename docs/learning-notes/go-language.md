# Öğrenme Notları — Go Dili

Go dilinin kendisiyle ilgili kavramlar: stdlib, idiomlar, test konvansiyonları, syntax.
Format ve nasıl çalıştığı: [README.md](README.md).

## İçindekiler

1. [stdlib (standart kütüphane) nedir?](#1-stdlib-standart-kütüphane-nedir)
2. [Testler neden ayrı klasörde değil? (`_test.go` konvansiyonu)](#2-testler-neden-ayrı-klasörde-değil-_testgo-konvansiyonu)
3. [Go paketleri JS/TS modüllerine nasıl denk gelir?](#3-go-paketleri-jsts-modüllerine-nasıl-denk-gelir)

---

## 1. stdlib (standart kütüphane) nedir?

**🗓️ Bağlam:** Faz 1'de ilk sunucu kodunu ve graceful shutdown'ı anlatırken sürekli "standart
kütüphane" / "stdlib" diyordum (`net/http`, `encoding/json`, `log` hepsi oradan). Terimi tekrar
kullanınca "stdlib nedir, onu da öğrenebilir miyim?" diye sordum.

**⚡ Kısa cevap:** Dilin kendisiyle **hazır gelen**, kurulum gerektirmeyen paketler topluluğu.
Go'yu kurduğun an yüzlerce hazır paket de elinde.

**📖 Açıklama:**

İlk [main.go](../../backend/cmd/api/main.go)'daki üç `import` (`net/http`, `encoding/json`,
`log`) **stdlib**. Bu yüzden [go.mod](../../backend/go.mod)'da **hiç dış bağımlılık yok** — bir
web sunucusunu sıfır paket indirerek yazdık.

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

Sık göreceğim faydalı stdlib paketleri: `fmt`, `errors`, `context`, `net/http`,
`encoding/json`, `database/sql`, `crypto/*`, `log/slog`, `testing`, `time`, `os`, `io`, `signal`.

Keşfetmek için (kurulum yok):
```bash
go doc net/http            # paket özetini terminalde gösterir
go doc net/http.ServeMux   # belli bir tipin dökümanı
```
Web: **pkg.go.dev/std** — tüm stdlib listesi.

**🔗 İlgili:**
- [docs/WHY_GO_VS_TS.md](../WHY_GO_VS_TS.md) §4 (görünür HTTP), §13 (toolchain & DX)
- "Sebepsiz bağımlılık ekleme": dış paketi (Chi/pgx) gerçek ihtiyaç çıkınca ekleyeceğiz —
  [PROJECT_BRIEF.md](../PROJECT_BRIEF.md) §14.5

---

## 2. Testler neden ayrı klasörde değil? (`_test.go` konvansiyonu)

**🗓️ Bağlam:** İlk testimizi (`main_test.go`) yazmıştık ve `main.go`'nun **tam yanında**
duruyordu. JS/TS'teki "testleri `tests/` veya `__tests__/` altında topla" alışkanlığımdan dolayı
"testleri ayrı bir yerde klasörlemek daha doğru bir yaklaşım olmaz mı?" diye sordum.

**⚡ Kısa cevap:** Go'da **hayır — tam tersi idiomatik.** Test dosyası, test ettiği kodun **tam
yanında** durur; aynı klasör, `xxx_test.go` adıyla.

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
koyabilirsin. Go'nun tek kararlı cevabı: **yanına koy, `_test.go`.**

Go'da ayrı klasör ne zaman? (nüans)
- **Unit testler** → her zaman kodun yanında.
- **Integration/e2e testler** → bazen üst seviye `test/` klasöründe.
- **`testdata/`** → test fixture dosyaları; Go aracı bu isimli klasörü derlemede **yok sayar**.

İki test-paketi seçeneği (aynı klasörde):
- `package main` (bizimki) → **white-box**, içeriye erişir (`handleHealth`'i çağırdık).
- `package main_test` → **black-box**, sadece public API'yi test eder.

**🔗 İlgili:**
- [backend/cmd/api/main_test.go](../../backend/cmd/api/main_test.go) (ilk testimiz)
- [docs/GO_CONVENTIONS.md](../GO_CONVENTIONS.md) §8 (test: table-driven, std `testing`, `-race`)
- [docs/WHY_GO_VS_TS.md](../WHY_GO_VS_TS.md) §13 (zero-config toolchain)

---

## 3. Go paketleri JS/TS modüllerine nasıl denk gelir?

**🗓️ Bağlam:** Config'i ayrı bir `internal/config` paketine mi yazsak diye konuşurken "Go'da
paket yazmak neye göre karar verilir, paketsiz kullanılamaz mı?" diye sordum; sonra JS modül
alışkanlığımdan "JS/TS'de neye denk geliyor? import alias (tsconfig) var mı? döngüsel import
nasıl?" diye devam ettim.

**⚡ Kısa cevap:** Go paketi ≈ JS/TS modülü — **ama** JS modülü = 1 **dosya**, Go paketi = 1
**klasör**. Terminoloji ters: JS'teki "package" (npm) → Go'da **module** (`go.mod`); JS'teki
"module" (dosya) → Go'da **package** (klasör). Export = **harf büyüklüğü** (`Config` public,
`getenv` private). tsconfig-style path alias **yok**. Döngüsel import **yasak** (derleme hatası).

**📖 Açıklama:**

Terminoloji tersliği (asıl kafa karıştıran):

| Birim | JS/TS adı | Go adı |
|---|---|---|
| En küçük import birimi | module (1 dosya) | **package** (1 klasör) |
| Bağımlılık / dağıtım birimi | package (`package.json`, npm) | **module** (`go.mod`) |

Eşleştirme:

| Konu | JS/TS | Go |
|---|---|---|
| Public yapma | `export` | **Büyük harfle başlat** (`Config`, `Load`) |
| Private | export etmemek | küçük harf (`getenv`) |
| Import | `import { x } from './f'` (cherry-pick) | `import "yol"` → `paket.X` (tüm namespace; `import * as` gibi) |
| Birim sınırı | 1 dosya = 1 modül | 1 klasör = 1 paket |
| `internal` zorlanır mı | hayır (convention) | **evet** (derleyici; sadece bu modül import eder) |
| Döngüsel import | tolere (dağınık) | **yasak** (derleme hatası) |

**Import path alias (tsconfig `paths` gibi) var mı? → Hayır.**
- Yol kanonik: `<module-path>` + `<klasör>`. Bir config dosyasında kısaltma tanımlanamaz.
- `import cfg "uzun/yol"` ile paketin **yerel adını** değiştirebilirsin (ama yolu kısaltmaz).
- Pratikte yolu elle yazmazsın; `gopls`/`goimports` otomatik ekler.
- Kısa import istersen kaldıraç = `go.mod`'da kısa module path (ama `go get`'lenemez olur).
- Tasarım: yollar küresel benzersiz + kendini-açıklayan → sıfır-config çözümleme.

**Döngüsel import — örnek** (`Book` bir kullanıcıya ait; `User`'ın da kitapları var):

❌ Yanlış (cycle): `books` paketi `users`'ı, `users` paketi `books`'u import eder →
`import cycle not allowed` (derleme hatası).

✅ Doğru yollar:
1. **ID ile referans:** `Book{ OwnerID int64 }`, `User{ ID int64 }` — paketler birbirini import etmez.
2. **Ortak `internal/domain` paketi:** ikisi de `domain`'i import eder, `domain` kimseyi → diamond (döngü değil).
3. **Tüketicide interface:** `books` küçük bir interface tanımlar, somut `users`'ı `main` enjekte eder.

Neden yasak? Döngüsüz grafik → her paket bir kez, bağımlılık sırasına göre derlenir (hızlı,
basit); temiz katmanlaşmayı zorlar. (Not: döngü yasağı **paketler arası**; aynı paketteki
dosyalar birbirine serbest erişir.)

**🔗 İlgili:**
- [docs/GO_CONVENTIONS.md](../GO_CONVENTIONS.md) §6 (import-cycle tuzağı: ID ile bağla / shared domain)
- [docs/WHY_GO_VS_TS.md](../WHY_GO_VS_TS.md) §4 (görünür yapı)
- [backend/go.mod](../../backend/go.mod) (module path = import prefix); `internal/config` (gelince)
