# Öğrenme Notları — Backend & Web

Dil-bağımsız backend/web kavramları: HTTP, API tasarımı, config/12-factor, server lifecycle
(graceful shutdown), hata yönetimi. Format: [README.md](README.md).

## İçindekiler

1. [Graceful shutdown nedir, neden önemli?](#1-graceful-shutdown-nedir-neden-önemli)
2. [Graceful shutdown Express/Node ile de yapılır mı?](#2-graceful-shutdown-expressnode-ile-de-yapılır-mı)
3. [Environment variable nedir? (os.Getenv, lokal vs prod)](#3-environment-variable-nedir-osgetenv-lokal-vs-prod)
4. [The Twelve-Factor App (12-factor) nedir?](#4-the-twelve-factor-app-12-factor-nedir)

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
