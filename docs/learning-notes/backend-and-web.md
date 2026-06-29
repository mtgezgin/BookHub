# Öğrenme Notları — Backend & Web

Dil-bağımsız backend/web kavramları: HTTP, API tasarımı, config/12-factor, server lifecycle
(graceful shutdown), hata yönetimi. Format: [README.md](README.md).

## İçindekiler

1. [Graceful shutdown nedir, neden önemli?](#1-graceful-shutdown-nedir-neden-önemli)
2. [Graceful shutdown Express/Node ile de yapılır mı?](#2-graceful-shutdown-expressnode-ile-de-yapılır-mı)

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
