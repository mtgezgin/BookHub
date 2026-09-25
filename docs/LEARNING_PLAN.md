# BookHub — Öğrenme Planı (sonraki oturum buradan başlasın)

> Bu dosya bir **devir belgesi**. 20 Eylül 2026 gecesi, Faz 13 bittikten ve push edildikten
> sonra yazıldı. Amacı: yeni bir oturum açıldığında "nerede kalmıştık, ne öğreneceğiz, nasıl"
> sorularının cevabını sıfırdan türetmeye gerek kalmaması.
> Durum ve mimari için: [STATUS.md](STATUS.md) · [PROJECT_MAP.md](PROJECT_MAP.md)

## 1. Ne oldu, neden bu plan var

13 faz bitti, kod GitHub'da ve çalışıyor. Ama **çalışma modu bozuldu**: anlaşma "sıkıcı kısmı
asistan yazar, anlamlı %20'yi kullanıcı yazar" idi; Faz 12–13'te asistan çoklu ajan
döngüleriyle neredeyse her şeyi yazdı. Ortaya iyi bir kod tabanı çıktı, eksik kalan öğrenme.

Kullanıcının kendi teşhisi (20 Eylül):

> "Belki de benim öğrenme şeklim daha teorik. Handler–repository kullanımı neyi sağlıyor,
> OOP'ye yaklaşmak için mi? Bunu anlasam zaten günümüzde kimse manuel kod yazmıyor.
> En temel fark ve anlamam gereken: JavaScript'teki async/await yerine buradaki concurrency
> yapısı. Anlasam da unutuyorum."

Bu plan o teşhise göre kuruldu: **önce kavram, sonra minik deney, en son üretim kodu.**

## 2. Yeni çalışma anlaşması

1. **Teori önce.** Her konu "bu hangi problemi çözüyor, olmasaydı ne olurdu" ile başlar.
   Kod ikinci sırada ve önce OKUMA olarak gelir — bu repoda o kavramın geçtiği gerçek satır.
2. **Uygulama birimi küçük.** Tam bir özellik yazmak yerine 10–30 satırlık **deney**:
   `/tmp/go-deney/` altında çalıştırılıp atılan programlar. Handler yazmak öğretmiyorsa
   yazdırmıyoruz; goroutine'in davranışını gösteren 15 satır yazdırıyoruz.
3. **Önce tahmin, sonra çalıştır.** Her deneyde çıktı tahmin edilmeden `go run` yok.
   Tahminin tutmaması en öğretici an — not oraya düşer.
4. **Üretim kodunu asistan yazabilir**, ama kullanıcı o kodu okumadan ve "neden böyle"
   sorusunu cevaplayamadan commit edilmez.
5. **Unutmaya karşı tekrar:** her konu 0. gün öğrenilir, **2., 7. ve 21. günlerde** yalnızca
   soruyla yoklanır (aşağıdaki "Yoklama soruları"). Not okumak tekrar değildir; hatırlamaya
   çalışmak tekrardır.
6. Takılınan/öğrenilen her kavram `/not` ile [learning-notes/](learning-notes/)'a düşer.

## 3. Asıl konu: Concurrency (JS async/await → Go)

**Neden en önemlisi:** Go'nun geri kalanı (interface, struct, error) JS/TS bilen biri için
sürpriz değil. Asıl model farkı burada ve yanlış anlaşılırsa üstüne konan her şey sallantıda
kalıyor.

**Tek cümlelik çekirdek fikir:** JavaScript'te eşzamanlılık *dilin sözdizimine* gömülüdür
(`async`/`await`, tek iş parçacığı, olay döngüsü); Go'da *çalışma zamanına* gömülüdür —
kod düz ve bloklayıcı yazılır, paralelliği zamanlayıcı sağlar.

### 3.0 Hazır materyal

| Oturum | Ders | Test |
|---|---|---|
| 1–2 | [learning/01-concurrency-goroutines.md](learning/01-concurrency-goroutines.md) — goroutine, "renkli fonksiyon" problemi, neden bloklamak serbest, bu repodaki 6 dayanak, iki yarış türü, tuzaklar, 5 deney, kaynak listesi | [learning/01-concurrency-test.md](learning/01-concurrency-test.md) — 18 soru + cevap anahtarı + puanlama |
| 3–6 | yazılacak | |

### 3.1 Altı oturumluk sıra

Her oturum ~30–45 dk: kavram → bu repodaki karşılığı → 1–2 minik deney → yoklama sorusu.

| # | Konu | Bu repoda nerede |
|---|---|---|
| 1 | **Goroutine nedir, `async` fonksiyondan farkı ne.** "Renkli fonksiyon" sorunu: JS'te `await` kullanmak için fonksiyonun `async` olması gerekir, bu da çağrı zincirinin tamamını boyar; Go'da böyle bir bölünme yoktur. | `backend/cmd/api/main.go` — HTTP sunucusu; **her istek zaten ayrı bir goroutine'de** koşar (`net/http` öyle yapar). Kullanıcı bugüne kadar hiç `go` yazmadı ama kodunun tamamı eşzamanlı çalışıyor. Başlangıç noktası bu fark ediş olmalı. |
| 2 | **Bloklamak neden serbest.** JS'te bloklayan kod tüm sayfayı dondurur; Go'da bir goroutine bloklanınca zamanlayıcı diğerini koşturur (M:N zamanlama). "Bekleyen" kod yazmak Go'da normaldir. | `internal/auth/service.go` — bcrypt karşılaştırması ~250 ms **bloklar**; bu bilinçli (zamanlama saldırısına karşı `dummyHash`). JS'te böyle bir satır felaket olurdu. |
| 3 | **Paylaşılan durum: mutex ve yarış.** `sync.Mutex`, `sync.OnceValue`. **Veri yarışı** (aynı belleğe eşzamanlı yazma) ile **mantıksal yarış** (check-then-act) farkı. | `internal/ratelimit/memory.go` (mutex) · `service.go`'daki `sync.OnceValue(dummyHash)` · CI'da `go test -race`. Mantıksal yarışlar ise veritabanında: `FOR UPDATE`, `ON CONFLICT`, reuse tespiti. İkisi ayrı dünyalar — bu ayrım kritik. |
| 4 | **Channel ve `select`.** Channel'ı "kuyruk" değil "randevu" olarak düşün. `select` = birden çok olaydan ilk geleni al. | `cmd/api/main.go` graceful shutdown (sinyal channel'ı) · `cmd/api/auth.go` içindeki `sleepCtx`: `select { case <-ctx.Done() … case <-t.C … }` — "ya bütçe biter ya süre dolar". |
| 5 | **Context: Go'nun `AbortController`'ı.** İptal ve süre sınırı çağrı zinciri boyunca taşınır. `WithTimeout`, `WithoutCancel`. | `handleLogout`: istemci bağlantıyı koparsa iptal yarıda kalmasın diye `context.WithoutCancel(r.Context())` + 5 sn bütçe. Bu satır, context'in ne işe yaradığını tek başına anlatıyor. |
| 6 | **Eşzamanlılığı test etmek.** Deterministik olmayan şeyi deterministik sınamak. | `internal/auth/repository_integration_test.go`: `pg_blocking_pids` ile "şu işlem gerçekten bloklandı mı" yoklaması, `-race`, 20 goroutine'in aynı token'ı döndürmesi. Sektörde çoğu projede olmayan bir şey; burada var. |

### 3.2 Minik deney fikirleri (her biri < 30 satır)

1. 5 goroutine başlat, hiçbirini bekleme, `main` bitsin → hiçbiri çalışmaz. Sonra
   `sync.WaitGroup` ekle. **Ders:** `main` biterse program biter; JS'te olay döngüsü beklerdi.
2. Bir sayaca 1000 goroutine ile `i++` yaptır → yanlış sonuç. `go run -race` ile çalıştır →
   yarış raporu. Sonra mutex ekle. **Ders:** veri yarışı ve `-race`'in ne gördüğü.
3. `time.Sleep(2*time.Second)` yapan 3 goroutine → toplam 6 sn değil ~2 sn. **Ders:** bloklamak
   goroutine'i durdurur, programı değil.
4. `select` ile bir timer ve bir channel yarıştır; hangisi kazanırsa onu yazdır.
5. `context.WithTimeout(100ms)` ver, 500 ms süren işi iptal ettir; sonra `WithoutCancel` ile
   aynısını yaptır. **Ders:** `handleLogout`'taki kararın birebir küçük hâli.

### 3.3 Yoklama soruları (2., 7., 21. gün — koda bakmadan)

1. JS'te `await` neden fonksiyonu "boyar", Go'da neden böyle bir şey yok?
2. BookHub'da hiç `go` anahtar kelimesi yazmadık; kod nerede eşzamanlı çalışıyor?
3. Veri yarışı ile mantıksal yarış farkı ne? Bu repoda her birine bir örnek.
4. `select` ne zaman gerekir, `if` neden yetmez?
5. `context.WithoutCancel` hangi somut hatayı engelledi?
6. 20 goroutine aynı refresh token'ı döndürmeye çalışırsa ne olur ve **neden**?

## 4. İkinci konu: "Handler → Service → Repository neyi sağlıyor?"

Kullanıcının açık sorusu; kısa cevap ve işlenecek başlıklar:

**Kısa cevap: OOP'ye yaklaşmak için değil.** Amaç *bağımlılık yönü* ve *değiştirilebilirlik*:
HTTP katmanı SQL bilmez, SQL katmanı HTTP bilmez. Faydası somut olarak şurada görülür —
`cmd/manage create-admin` komutu HTTP'den geçmeden aynı servisi çağırır ve **aynı parola
politikasını** uygular. Katman olmasaydı politika handler'da kalırdı ve ikinci giriş noktası
onu atlardı.

Bu başlıkla birlikte işlenecek "neden böyle" soruları (hepsi bu repoda karşılığı olan):

- Interface'i neden **kullanan taraf** tanımlıyor (Go'da `implements` yok)?
- Basit CRUD'da (kitap/tür) neden servis katmanı **yok**, auth'ta **var**?
- Framework'süz dependency injection: `deps` struct'ı ve `main.go` — neden bir DI kütüphanesi yok?
- `CachedService` neden decorator? Cache'i servise gömsek ne kaybederdik?
- Moderasyon durum makinesini **veri** olarak tutmak (`moderation.Next`) neye yaradı?
- Repository'nin domain hatası döndürmesi (`books.ErrNotFound`) — pgx hatasını yukarı sızdırmak
  neyi bozardı?

## 5. Açık işler (öğrenmeden bağımsız, sırada duruyor)

1. **Deploy kararları** — [DEPLOYMENT.md](DEPLOYMENT.md) hazır. Kullanıcının vermesi gereken
   üç karar: tek adres mi (önerilen: evet, `SameSite=Strict` çerez yüzünden), hangi platform
   (öneri: Fly.io), Redis şart mı (iki API kopyası çalıştıracaksa evet). Hesaplar: Supabase +
   platform (+ Upstash). **Sır asistana verilmez**, platformun secret alanına kullanıcı girer.
2. **Code review panelinin 4 tavsiyesi** (engelleyici değil, hepsi gerçek):
   harcanmış token'ların geri kazanılmaması · localStorage yazamayan tarayıcıda oturumun
   kullanılamaz hâle gelmesi · çıkış yeniden deneme zamanlayıcısının sızması · parola
   değişiminin kilit sırası değişmezini kırması (`LockUserLiveRefreshFamilies`'teki
   `revoked_at IS NULL`).
3. **11 açık Dependabot PR'ı.** Dikkat isteyenler: TypeScript 5.9 → 7.0 (ana sürüm),
   node 24-alpine → 25-alpine (Scout'ta 25 daha fazla açık gösteriyordu, birleştirmeden önce
   `make scan`), @types/node 26 (çalışma zamanından ileri gitmemeli).
4. **İki backlog:** [REVIEW_FEEDBACK.md](REVIEW_FEEDBACK.md) (15 açık bulgu) ve
   [FEATURE_REQUESTS.md](FEATURE_REQUESTS.md) (ürün backlog'u; asistanın değerlendirmesi
   sohbette verildi — özeti: ACC-04 oturum listesi ve MOD-01 "Önerilerim" öne çekilmeli;
   REV-03, NOTIF-01, SOC-03, MOD-03/04, LIB-03, ACC-01 kesilmeli; CAT-02 tek sayfalık bir
   karar kaydına indirilmeli).
5. **Bu dosya henüz push edilmedi** (push üç kapıdan geçiyor; bir sonraki iş partisiyle gider).

## 6. Sonraki oturumun ilk 5 dakikası

1. Bu dosyayı oku.
2. Kullanıcıya sor: bugün **öğrenme oturumu** mu (§3'ten sıradaki konu), **deploy** mu (§5.1),
   yoksa **düzeltme** mi (§5.2)?
3. Öğrenme oturumuysa: kavramı anlat → repodaki satırı birlikte oku → deneyi **kullanıcı**
   yazsın (asistan yazmaz) → yoklama sorusunu sor → `/not` ile kaydet.
4. Asistan uygulama kodunu yazmaya başlarsa kullanıcı "dur" desin; bu dosyanın §2'si o yüzden var.
