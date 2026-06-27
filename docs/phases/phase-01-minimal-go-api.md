# Faz 1 — Minimal Go API

> Durum: 📖 detaylı · Önkoşul: Faz 0 (repo + git) ✅
> İlgili: [WHY_GO_VS_TS](../WHY_GO_VS_TS.md) §1/§3/§4/§5/§8/§13 · [GO_CONVENTIONS](../GO_CONVENTIONS.md) §1/§2/§5/§8/§10 · [PROJECT_BRIEF](../PROJECT_BRIEF.md) §11 (Phase 1), §18.1

## 🎯 Amaç

Çalışan, **en küçük** Go HTTP sunucusu: `GET /health` çağrısına JSON dönen bir API. Veritabanı
yok, auth yok, Docker yok. Sadece "Go ile bir web sunucusu nasıl ayağa kalkar"ı öğreniyoruz.

## 🧠 Öğreneceğin yeni kavram(lar)

- **Go modülü** (`go.mod`) ve proje klasör yapısı (`cmd/` + `internal/`).
- **HTTP handler** ve `net/http` / Chi router'ın temelleri.
- **JSON response** yazmak (struct → JSON).
- **Config'i env'den** okuyup **boot'ta doğrulamak** (eksikse hemen hata).
- **Graceful shutdown** (Ctrl-C gelince sunucuyu düzgün kapatmak).
- **Hata zarfı** (tutarlı JSON hata formatı) — frontend'in güveneceği kontrat.
- **Table-driven test** — Go'nun standart test biçimi.

## Neden bu faz burada?

Önce "bir şey çalışıyor" hissini almak gerekiyor. Veritabanı/auth eklemeden, sadece istek
gelince cevap veren bir iskelet kurarsak, sonraki her şeyi bunun üstüne koyarız. Go'nun en
temel idiomlarını (ctx, hata, config, kapanma) buraya **bir kez** doğru yerleştirirsek,
sonradan eklemek acı vermez.

## 🪜 Adımlar (step-by-step)

> Her adımı birlikte yapacağız; bu liste yol haritası. Kodu satır satır o an açıklayacağım.

1. **Go kurulu mu, doğrula.** `go version` çalışmalı. Yoksa önce Go kurulumu.
   - *Checkpoint:* `go version` bir sürüm yazdırıyor.
2. **Modülü başlat.** `backend/` içinde `go mod init github.com/mtgezgin/bookhub/backend`.
   - *Neden:* `go.mod` bu projenin "kimliği" + bağımlılık listesi. Modül yolu bir
     **tanımlayıcı**dır; gerçekten internetten çekilebilir olması gerekmez (kişisel proje).
3. **Klasör iskeleti.** `backend/cmd/api/main.go` (uygulamanın giriş noktası) ve birazdan
   `backend/internal/` (kendi paketlerin).
   - *Neden:* `cmd/` = çalıştırılabilir program(lar); `internal/` = sadece bu projenin
     kullanabileceği iç kod. Go'nun standart düzeni.
4. **En küçük sunucu.** `main.go` içinde bir `http.Server`, bir `/health` handler'ı, JSON
   `{"status":"ok"}` dönsün. `go run ./cmd/api` ile çalıştır, tarayıcı/curl ile gör.
   - *Checkpoint:* `curl localhost:8080/health` → `{"status":"ok"}`.
5. **Chi router'ı ekle.** Router'ı Chi'ye taşı (std `http.Handler` uyumlu, sihir yok).
   - *Neden:* İleride middleware ve route grupları (auth, admin) gerekecek; Chi bunu temiz tutar.
6. **Config'i env'den oku + doğrula.** `PORT` gibi değerleri `os.Getenv`'den al, eksik/yanlışsa
   **boot'ta** `log.Fatal`. `.env.example` ekle (`.env` gitignore'da).
   - *Neden:* "Eksik env → request anında gizemli panic" hatasını baştan engelle (12-factor).
7. **Graceful shutdown.** `signal.NotifyContext` ile SIGINT/SIGTERM yakala, `srv.Shutdown(ctx)`.
   - *Neden:* Ctrl-C'de açık istekleri yarıda kesmeden kapan; Docker/K8s fazında bu önemli olacak.
8. **Hata zarfı + JSON helper.** `{"error":{"code","message"}}` formatı ve küçük bir
   `writeJSON` / `writeError` yardımcı fonksiyonu. Bir `/health` yanında bilerek bir hata
   senaryosu (ör. 404) ile dene.
   - *Neden:* Tüm API tek tip hata döndürsün; frontend `error.code`'a göre dallanabilsin.
9. **İlk test.** `/health` handler'ına `httptest` ile, ayrıca küçük bir pure fonksiyona
   **table-driven** test yaz. `go test -race ./...` yeşil olsun.
   - *Checkpoint:* `go test ./...` PASS.

## ⚠️ Önemli noktalar

- **Varsayılan `http.Server`'ı çıplak kullanma.** `ReadTimeout`/`WriteTimeout` ayarla; aksi
  halde yavaş bir istemci bağlantıyı süresiz tutabilir. (GO_CONVENTIONS §5)
- **`http.DefaultServeMux` / global state'ten kaçın.** Router'ı ve bağımlılıkları `main.go`'da
  elle kur (DI). (GO_CONVENTIONS §3)
- **`context.Context`'i baştan alışkanlık yap** — handler'da `r.Context()`, ileride servis/repo
  metodlarının ilk parametresi. Şimdi az iş, sonra çok kazanç. (GO_CONVENTIONS §1)
- **Hata formatını BİR kez seç ve dondur.** Üç kez değiştirip frontend'i kırma. (GO_CONVENTIONS §2)
- **Config'i boot'ta doğrula, fail-fast.** Eksik `PORT`/ileride `DATABASE_URL` → hemen patla,
  request anında değil.
- **Test için pure fonksiyon kullan.** İş mantığını HTTP'den ayır ki table-driven test kolay
  olsun; mock'a boğulma. (GO_CONVENTIONS §8)
- **`gofmt`/`go vet`'i alışkanlık yap** — JS'teki eslint/prettier tartışması Go'da yok; tek komut.

## ✅ Tamamlanma (Definition of Done)

- [ ] `go run ./cmd/api` çalışıyor; `curl localhost:8080/health` → `{"status":"ok"}`.
- [ ] Config env'den okunuyor ve eksikse boot'ta hata veriyor.
- [ ] Ctrl-C'de sunucu graceful kapanıyor.
- [ ] Tutarlı JSON hata zarfı var; bir hata senaryosu onu döndürüyor.
- [ ] En az bir `httptest` ve bir table-driven test var; `go test -race ./...` PASS.
- [ ] `gofmt`/`go vet` temiz.

## 📒 Bu fazda WHY_GO_VS_TS güncellemesi

Faz bitince [WHY_GO_VS_TS.md](../WHY_GO_VS_TS.md)'de şu placeholder'ları gerçek `dosya:Sembol`
referansı + kısa refleksiyonla doldur:

- **§3** (elle DI) → `cmd/api/main.go`
- **§4** (görünür HTTP / Chi) → `internal/server/router.go`
- **§5** (JSON + validation sınırı) → ilk DTO/handler
- **§8** (graceful shutdown + fail-fast config) → `cmd/api/main.go`
- **§13** (toolchain + table-driven test) → ilk `_test.go`

Ve §15'e ("TS/JS nerede kazanır") en az bir dürüst girdi ekle (ör. "bu basit JSON CRUD'da
Express de aynı kolaylıkta olurdu").
