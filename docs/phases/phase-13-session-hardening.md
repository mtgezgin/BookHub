# Faz 13 — Oturum Sertleştirme (httpOnly çerez, token aileleri, gerçek tarayıcı testleri)

> Durum: ✅ TAMAM · Önkoşul: Faz 12 ✅
> İlgili: [WHY_GO_VS_TS](../WHY_GO_VS_TS.md) §7/§10/§15 · [REVIEW_FEEDBACK](../REVIEW_FEEDBACK.md) ·
> [security-and-auth notları](../learning-notes/security-and-auth.md) · [database notları](../learning-notes/database.md) ·
> [tooling-and-workflow notları](../learning-notes/tooling-and-workflow.md) ·
> [infra-and-devops notları](../learning-notes/infra-and-devops.md) · [PROJECT_MAP](../PROJECT_MAP.md)

## 🎯 Amaç

Faz 3'te kurduğumuz oturum modeli "çalışıyordu" ama **çalınmış bir token'ı fark edemiyordu** ve
uzun ömürlü sır tarayıcıda JavaScript'in okuyabildiği bir yerde (`localStorage`) duruyordu. Bu
faz o iki deliği kapatır, üstüne CSRF korumasını ve "parola değişince her yerden çık" davranışını
ekler — ve hepsini **gerçek bir tarayıcıda** (Playwright) doğrular.

## 🧠 Yeni kavram(lar)

- **Refresh token ailesi + reuse tespiti** (RFC 9700 §4.14.2): rotasyon hırsızlığı *kısaltır*,
  tespit *görür*.
- **Mutlak oturum tavanı** (kayan pencere ≠ sonsuz oturum).
- **httpOnly / Secure / SameSite=Strict çerez** ve neyi çözmediği.
- **CSRF** ve Go 1.25'in `http.CrossOriginProtection`'ı (Fetch Metadata → `Sec-Fetch-Site`).
- **Postgres satır kilidi türleri** (`FOR UPDATE` / `FOR NO KEY UPDATE` / `FOR KEY SHARE` /
  `FOR SHARE`) ve **kilit sırası değişmezi**; `SKIP LOCKED`.
- **Gerçek tarayıcı testi** (Playwright) ve onun kendi kısıtı: rate limit bütçesi.

## 🧩 Tehdit → cevap haritası

| Tehdit (somut senaryo) | Bu fazdaki cevap |
|---|---|
| XSS'li bir betik 7 günlük refresh token'ı okuyup kendi sunucusuna kopyalar | Token artık **httpOnly çerezte**; JS ne okur ne yazar (`1d2e92d`) |
| Token çalındı, hırsız yarışı kazandı → zinciri o taşır, kimse fark etmez | **Token ailesi + reuse tespiti**: kullanılmış token ikinci kez gelirse aile ölür (`362dc11`) |
| Hırsız aileyi düzenli yenileyerek sonsuza dek taşır | **Mutlak tavan** `JWT_SESSION_MAX_AGE` (30 gün) (`362dc11`) |
| Kötü bir sayfa, kurbanın çerezleriyle bizim API'mize yazma isteği attırır (CSRF / login-CSRF) | **`http.CrossOriginProtection`** + `SameSite=Strict` (`c1901ff`) |
| "Hesabım ele geçirilmiş olabilir" → parolayı değiştiriyorum, ama diğer cihazlar açık kalıyor | **Parola değişimi TÜM aileleri iptal eder** (`90402d8`) |
| Tarayıcıya özgü davranışlar (çerez, Web Locks, sekmeler) hiç test edilmiyor | **Playwright** ile 11 senaryo, gerçek chromium (`5192b19`) |
| Temel imajlardaki CVE'leri kimse saymıyor | slim taban + digest pin + **Trivy CI işi** + Dependabot (`c79707b`) |

---

## 🪜 Ne yapıldı, neden

### 1. Token aileleri + reuse tespiti (`362dc11`)

**Problem.** Rotasyon (her `/refresh`'te yeni token, eskisi ölür) çalınan token'ın ömrünü kısaltır
ama hırsızlığı **fark etmez**: hırsız gerçek sahipten önce kullanırsa yeni zinciri o taşır, sahibin
elindeki token "bilinmiyor" der, kullanıcı yeniden girer ve hırsızın zinciri yaşamaya devam eder.

**Çözüm.** Bir giriş = bir **aile** (`refresh_token_families`). Kullanılan token **silinmez**,
`used_at` ile işaretlenir. Aynı token ikinci kez gelirse iki taraftan biri eski bir kopya
kullanıyordur — hangisi olduğunu bilemeyiz, bu yüzden **ailenin tamamı** ölür.

Migration [`0008_refresh_token_families.sql`](../../backend/migrations/0008_refresh_token_families.sql):

```sql
TRUNCATE refresh_tokens;                      -- (1) eski token'ların ailesi yok; herkes bir kez yeniden girer
CREATE TABLE refresh_token_families (         -- (2) id, user_id, created_at, absolute_expires_at, revoked_at
    ...
);
ALTER TABLE refresh_tokens                    -- (3) family_id NOT NULL (CASCADE) + used_at
    ADD COLUMN family_id ..., ADD COLUMN used_at timestamptz;
```

**Tuzak 1 — `WithTx` COMMIT tuzağı (en öğretici satır).** Reuse görüldüğünde aileyi iptal edip
hatayı closure'dan dönmek **yanlış**: `database.WithTx`, closure hata dönerse ROLLBACK eder — yani
"tespit ettik" dediğimiz iptal geri alınır ve aile yaşamaya devam ederdi. Doğrusu: iptali yap,
sonucu dıştaki değişkene yaz, closure'dan **`nil` dön** (COMMIT), hatayı `WithTx`'ten **sonra**
üret ([`repository.go`](../../backend/internal/auth/repository.go), `RotateRefreshToken` adım c).

**Tuzak 2 — kilit sırası değişmezi.** Bir aileye dokunan **her** transaction önce
`refresh_token_families` satırını, **sonra** o ailenin `refresh_tokens` satırlarını kilitler
(Faz 5/0007'deki "önce ebeveyn satırı" deseninin devamı). İlk tasarım rotasyonda tersini yapıyordu:
rotasyon token'ı tutup aileyi, iptal aileyi tutup token'ı bekledi → **gerçek deadlock, `40P01`**.
Bugün bunu `TestRotationWaitsForRevocationHoldingFamilyRealDB` deterministik olarak koruyor
(`isDeadlock` yardımcısı `pgErr.Code == "40P01"`e bakar).

**Ayırt edilemeyen cevap.** `ReuseError` bir **güvenlik olayı** (WARN log: `user_id`, `family_id`,
`request_id` — token ASLA), ama `Unwrap() → ErrInvalidToken` sayesinde istemciye giden cevap düz
"geçersiz token" 401'iyle **birebir aynı**. Farklı bir kod/mesaj, saldırgana "bu token'ı biri
zaten kullanmış" diyen bir kâhin (oracle) olurdu.

### 2. Mutlak oturum tavanı

Rotasyon her seferinde pencereyi yeniden 7 gün açar (kayan pencere). Tavan yoksa aileyi ele geçiren
oturumu **sonsuza dek** taşır. `JWT_SESSION_MAX_AGE` (varsayılan `720h` = 30 gün) ailenin
`absolute_expires_at`'i olur; her token `min(now + JWT_REFRESH_TTL, tavan)` ile kırpılır ve
`refresh_expires_at` cevapta **gerçek** bitişi söyler. Config, `JWT_SESSION_MAX_AGE >=
JWT_REFRESH_TTL` olmasını boot'ta zorlar — aksi hâlde `JWT_REFRESH_TTL` yalan söylerdi.

### 3. CSRF: `http.CrossOriginProtection` + NGINX `$http_host` (`c1901ff`)

**Neden bu commit çerezden ÖNCE geldi.** Refresh token çereze taşındığı an, login cevabı oturumu
tarayıcıya **kendiliğinden** yazar: başka bir sayfa kurbanın tarayıcısında *saldırganın* hesabıyla
oturum açtırabilir (login-CSRF) ya da kurbanın çereziyle `/refresh`–`/logout`'a gidebilirdi. Kapıyı
çerez gelmeden önce kapattık.

Go 1.25'in `http.NewCrossOriginProtection()`'ı zincire `RequestID` + `requestLogger`'dan **sonra**
takılı (ret de loglansın ve `X-Request-Id` taşısın), `SetDenyHandler` ile bizim JSON zarfımızı
döner: `403 cross_origin_request`.

| Girdi | Karar |
|---|---|
| `GET/HEAD/OPTIONS` | her zaman geçer → hiçbir GET ucu durum değiştirmemeli |
| `Sec-Fetch-Site: same-origin` / `none` | geçer |
| `Sec-Fetch-Site: cross-site` **veya** `same-site` | **403** (kardeş alt alan aynı *site*'tır ama aynı *origin* değildir; `SameSite` çerezi oraya YİNE gider) |
| Başlık yok → `Origin` host:port == `Host` | geçer, farklıysa 403 |
| Ne `Sec-Fetch-Site` ne `Origin` (curl, smoke) | geçer — CSRF bir **tarayıcı** saldırısıdır |

**NGINX düzeltmesi.** Tarayıcılar Fetch Metadata başlıklarını **yalnız güvenli bağlama** (HTTPS
veya localhost) yollar; düz HTTP'deki bir kurulumda asıl savunma `Origin == Host` kontrolüdür ve
**port da karşılaştırılır**. `proxy_set_header Host $host` portu düşürüyordu → `"localhost:3000"
≠ "localhost"` → meşru istek 403 (denendi). Artık `$api_host` map'i `$http_host`'u iletiyor
(istemci hiç `Host` göndermediyse `$host`'a düşer). Regresyonu `e2e/cross-origin.spec.ts`'in
"Origin == Host → **401**, 403 DEĞİL" vakası tutuyor.

### 4. Refresh token httpOnly çerezte (`1d2e92d`)

Çerez: `Name=COOKIE_NAME` (varsayılan `bookhub_refresh`), `Path=/api/auth`, `HttpOnly`,
`SameSite=Strict`, `Secure=COOKIE_SECURE` (varsayılan **true**), `Expires` = tavanlı gerçek bitiş.
`sessionResponse`'tan `refresh_token` **çıkarıldı** (gövdede de dönseydi çerez anlamsız olurdu);
`refresh_expires_at` kaldı — o bir sır değil.

- **`Path=/api/auth`**: uzun ömürlü sır `/api/books`'a giden her istekte ağda dolaşmasın.
- **`COOKIE_NAME` neden ayarlı**: çerezler **port'a göre değil host'a göre** kapsanır (RFC 6265
  §8.5) — compose (`:3000`), kind (`:3001`) ve vite (`:5173`) aynı çerezi ezerdi. kind kendi adını
  kullanıyor (`COOKIE_NAME: bookhub_refresh_k8s`). Web Locks ve localStorage ise origin (port dahil)
  başınadır; ikisi aynı şeyi kapsamıyor.
- **Çerez okuma `r.CookiesNamed`** ile: tam 1 tane, boş değil, ≤ 256 bayt. Aynı adla iki çerez =
  **gölgeleme (cookie shadowing)** girişimi; istek başlığında Path/Domain bilgisi olmadığı için
  hangisinin gerçek olduğu ayırt edilemez → tahmin etmek yerine reddediyoruz (WARN).
- **Çıkışta çerez politikası (inceleme bulgusu F01).** Silme çerezi YALNIZ iki yerde yazılır:
  başarı (204) ve çerezsiz/bozuk çerez (204). **Servis hatasında (500) çerez KORUNUR** — aileyi
  öldürebilecek tek anahtar odur; silseydik istemcinin `bookhub.logout_pending` yeniden denemesi
  çerezsiz gider, sunucu hiçbir şeyi iptal etmeden 204 der ve aile mutlak tavana kadar yaşardı.
  Sunucu ayrıca `context.WithoutCancel` + 5 sn bütçe içinde kendi kendine 200 ms / 1 sn aralıklı
  iki deneme daha yapar (sekme kapanırken kopan bağlantı iptali yarıda bırakmasın).
- **HINT bayrağı sır değil.** `localStorage`'da yalnızca `bookhub.session="1"` var: "bu tarayıcıda
  bir oturum açık olmalı". Bilinmesi kimseye bir şey kazandırmaz ama üç iş görür: (1) anonim
  ziyaretçi her açılışta boşuna `/refresh` atmasın, (2) bir sekmedeki çıkış diğerlerine `storage`
  olayıyla duyurulsun, (3) uçuştaki bir yenileme arada çıkış yapıldığını anlayıp oturumu
  **diriltmesin**. Faz 13 öncesinden kalan `bookhub.refresh` anahtarı açılışta silinir.

**Kalan XSS riski (dürüst olalım).** httpOnly XSS'i **çözmez**. Enjekte edilmiş bir betik hâlâ
`/api/auth/refresh`'e POST atıp 15 dakikalık bir access token alabilir (çerezi göremese de tarayıcı
onu ekler). Çerezin kazandırdığı tek şey şu: **uzun ömürlü sır saldırganın sunucusuna
kopyalanamaz** — saldırı, kurbanın tarayıcısı açık kaldığı sürece yaşar. En sıkı model BFF'tir
(token tarayıcıya hiç gelmez); bizimki "yarı-BFF".

### 5. Parola değiştirme + tüm ailelerin iptali (`90402d8`)

`POST /api/auth/password` (`requireAuth` + `rateLimit(limiter, "password")`), gövde
`{current_password, new_password}`. Başarıda 200 + yeni oturum + **yeni çerez** (bu cihazın eski
ailesi de öldü; çerez yazılmasa tarayıcıda ölü bir çerez kalırdı).

- **Yanlış mevcut parola 403, 401 DEĞİL.** İstemci 401'i "access token bitti" sayıp sessizce
  yenileyip isteği tekrarlardı; kullanıcı hatasını hiç görmezdi. 403 = "kimliğin belli ama bu
  isteğe iznin yok".
- **Kilit sırası bir kademe yukarıdan**: `users` → `refresh_token_families` → `refresh_tokens`.
- **`FOR NO KEY UPDATE`, `FOR UPDATE` DEĞİL** (`LockUserPasswordForChange`): uçuştaki bir rotasyon
  ailesinin kilidini tutarken halef token'ı INSERT eder ve `refresh_tokens.user_id` FK'si için
  `users` satırında `FOR KEY SHARE` alır. `FOR UPDATE` bununla **çakışır** → karşılıklı bekleme →
  `40P01`. `FOR NO KEY UPDATE` çakışmaz ("yalnız anahtar olmayan kolonu değiştireceğim";
  `password_hash` anahtar değil). `TestChangePasswordWaitsForInFlightRotationRealDB` bu mutasyonda
  deterministik kırılır.
- **Giriş yarışı `FOR SHARE` ile kapandı.** bcrypt karşılaştırması transaction **dışında** ve eski
  hash ile yapılıyor; o ~250 ms içinde parola değişirse yeni aile **iptalden sonra** doğar ve eski
  parolayla yaşardı. `StartRefreshFamily` artık tx içinde `LockUserPasswordForLogin`
  (`SELECT password_hash ... FOR SHARE`) ile hash'in hâlâ aynı olduğunu doğrular; farklıysa
  `ErrInvalidCredentials` + rollback. `FOR SHARE`, `FOR NO KEY UPDATE` ile çakışır (giriş ile
  değişim seri olur) ama rotasyonların `FOR KEY SHARE`'iyle çakışmaz (girişler rotasyonları
  bekletmez).
- **Rate limit ayrı kovada**: NGINX'te `zone=password` (10r/m, burst 5) ve `location =
  /api/auth/password`; API'de `"password"` anahtarı. Çalınmış bir access token'la mevcut parolayı
  kaba kuvvetle deneyen biri kurbanın **giriş** kotasını yemesin.

Arayüz: `/account` ("Hesabım", `RequireRole` arkasında) — mevcut / yeni / yeni (tekrar). Tekrar
alanı sunucuya **gitmez**; uyuşmazlık istemcide yakalanır (yanlış yazılmış bir parola kaydedilip
kullanıcıyı her cihazdan düşürmesin). Başarı mesajı: "Parolan değişti. Diğer tüm cihazlardaki
oturumlar kapatıldı."

### 6. Tolerans penceresi YOK — ödünleşim ve bedeli

Auth0/Okta gibi sağlayıcılar harcanmış bir token'ı birkaç saniye daha kabul eden bir **grace
window** sunar. Biz **0** seçtik: pencere, yarışı birkaç saniye önce kazanan hırsızın fark
edilmemesine izin verirdi. Bedeli gerçek ve bilinçli:

| Bedel | Nasıl hafifletildi |
|---|---|
| Aynı tarayıcıdaki eşzamanlı yenilemeler birbirini "reuse" sanabilir | **Web Locks** (`bookhub-refresh`) çerez yazan tüm istekleri origin başına sıraya sokar |
| Web Locks olmayan/güvenli olmayan bağlam | Kilit yoksa istemci doğrudan çalışır: nadiren gereksiz bir "oturum düştü" |
| Sayfa kapanırken cevabın kaybolması → tarayıcıda harcanmış çerez | `keepalive: true` ile istek iptal edilmez; pencere daralır |
| Commit başarılı ama **cevap** kayıp (F02) | Politika olarak kabul: bir sonraki yenileme gerçek bir reuse'dur, aile ölür, kullanıcı yeniden girer |
| Aynı host'taki farklı portlar aynı çerezi paylaşır | `COOKIE_NAME` (kind `bookhub_refresh_k8s`) |

### 7. F02 — commit'ten sonra başarısız olabilecek adım bırakma (`e865548`)

Rotasyon commit ediliyor, kullanıcı satırı **sonra** ayrı bir sorguyla okunuyordu. O sorgu
patlarsa: halef token DB'de yazılı, istemcide yok → istemci eski çerezle tekrar dener → **reuse** →
tüm aile ölür. Bir saniyelik DB sarsıntısı kullanıcıyı her cihazdan atıyordu. Artık kullanıcı
**aynı transaction'da** düz bir `SELECT` ile okunuyor (`Rotation.User`; kilit almaz, kilit sırasını
etkilemez) ve hesap silinmişse rotasyon geri alınıyor. Kural: **COMMIT'ten sonra başarısız
olabilecek adım bırakma.**

### 8. Playwright: gerçek tarayıcı (`5192b19`)

**Neden curl yetmiyordu.** CI'daki E2E işi `localhost:8080`'e doğrudan curl atıyordu; vitest ise
jsdom'da koşar ve orada **çerez diye bir şey yoktur**. Faz 13'ün konusu tam olarak tarayıcıya ait:
httpOnly + Secure + `SameSite=Strict` bir çerez, rotasyon, reuse, sekmeler arası Web Locks,
`window.prompt`, gerçek form. 11 senaryo, yalnız chromium, `frontend/e2e/`.

**Rate limit bütçesi = sert kural.** NGINX'te login ve register **tek** kovayı paylaşır
(`zone=auth`, 20r/m burst 10 nodelay) ve host'tan gelen her istek konteyner için aynı adrestir:
**anında 11 istek**, sonra 3 saniyede 1. Kararlı hâlde suite **9** kullanıyor; tablo
`e2e/helpers.ts`'in başında ve yeni senaryo eklerken önce o tabloya satır eklenir. 429 gelirse test
**adıyla patlar** (sessiz yeniden deneme bütçe aşımını gizlerdi). `scripts/e2e.sh` her koşudan önce
`bookhub-web`'i yeniden başlatır: `limit_req` sayacı nginx sürecinin paylaşılan belleğindedir, yani
restart onu sıfırlar ve arka arkaya koşular güvenli olur.

**`storageState` PAYLAŞILMAZ.** Playwright'in klasik hızlandırması "bir kez giriş yap, oturum
durumunu tüm testlere ver"dir — bizde bu **yanlış**: aynı refresh çerezini iki test kullanırsa
ikincisi harcanmış token sunar, reuse tespiti aileyi iptal eder ve testler birbirini düşürür. Her
test kendi girişini yapar; paylaşılan tek şey `global-setup`'ın NGINX'i atlayarak (bütçeye
girmeden) açtığı "okur" hesabıdır.

**Suite bir uygulama hatası buldu (`2925550`).** Katalog senaryosu yerel, kalabalık bir veritabanında
kırmızıya döndü: filtre değişince yeni istek atılıyor ama **filtresiz sorgu daha yavaş** (tüm
katalog taranır) — "Dune" araması 47 ms'de dönerken 243 ms'lik filtresiz cevap ızgaranın üstüne
boyuyordu. URL `?q=Dune` derken liste her kitabı gösteriyordu (inceleme bulgusu F10'un Home yarısı).
`usePaged`'de bu koruma vardı, katalogda yoktu.

### 9. İmaj sertleştirme (`c79707b`, `ec2da77`)

Push kapıları kod diff'ini inceliyor, **temel imajların CVE envanterini değil**. Docker DX
eklentisinin uyarısı buna bakma vesilesi oldu:

- `node:24-alpine` (derleme aşaması): 9 CVE, hepsi npm'in gömülü paketlerinde (tar, undici,
  ip-address, brace-expansion). Derleme aşaması **son imaja girmez** ve npm@latest aynı sürümleri
  gömüyor → uygulanacak bir upstream düzeltmesi yok; Dockerfile'da yazılı.
- `nginx-unprivileged:stable-alpine` → **`-slim`**: hiç kullanmadığımız dinamik modüller (geoip,
  image-filter, njs, xslt) ve kütüphaneleri düşer: **91 MB → 30 MB, 35 → 21 paket**; Scout'un
  bildirdiği "1 high" o kütüphanelerdeydi.
- Tabanlar **digest ile sabitlendi** (tekrarlanabilir build; taşınan bir etiket altımızdan imaj
  değiştiremez). Bedeli: yamalar kendiliğinden gelmez → **`.github/dependabot.yml`** (docker, gomod,
  npm, github-actions; haftalık) pinlemenin diğer yarısıdır.
- CI'ya **`images` işi**: iki imajı derler ve *düzeltmesi olan* HIGH/CRITICAL bir açıkta kırmızı
  yanar (`trivy-action` commit SHA'sıyla pinli). Aynı kural lokalde `make scan`.
- Kubernetes tarafında: hiçbir pod Kubernetes API'siyle konuşmuyor → `automountServiceAccountToken:
  false` (en az yetki).

---

## ⚠️ Önemli noktalar (tuzak listesi)

1. **`WithTx` + iptal**: iptali COMMIT ettirmek için closure'dan `nil` dön, hatayı dışarıda üret.
2. **Kilit sırası her yerde aynı olmalı**: `users` → `families` → `tokens`. Tek bir sorgu bile ters
   sıraya girerse `40P01` gerçek olur.
3. **Kilit gücünü ihtiyaca göre seç**: FK'nin aldığı `FOR KEY SHARE` ile çakışmayan en zayıf kilit
   `FOR NO KEY UPDATE`.
4. **`SKIP LOCKED`**: girişteki temizlik, o an bir rotasyonun kilitlediği aileyi **atlar** —
   beklerse bayat anlık görüntüyle yeni verilmiş halefi silerdi.
5. **Silme çerezi, set çerezinin `Name`/`Path`'ini birebir taşımalı**: çerez kimliği **ad + domain
   + path**'tir (RFC 6265 §5.3). Yanlış `Path`'li bir silme çerezi tarayıcıda ikinci bir kayıt
   açar, asıl çerez yaşamaya devam eder. `Secure`/`SameSite` kimliğe dahil DEĞİL — ama `Secure`
   yine de tekrarlanmalı: `__Secure-` önekli bir adda (`COOKIE_NAME` buna izin veriyor) `Secure`
   içermeyen `Set-Cookie` tarayıcıca **tümden reddedilir** (silme sessizce başarısız olur) ve
   güvensiz bir origin zaten `Secure` bir çerezin üzerine yazamaz. İkisini de tek yerde
   (`newRefreshCookie`) üretmek bu sınıfı kapatır.
6. **`Sec-Fetch-Site` yalnız güvenli bağlamda gelir** → `Origin == Host` yedeği gerçek savunmadır →
   proxy `Host`'u **portuyla** iletmeli.
7. **Hiçbir GET ucu durum değiştirmemeli**: CSRF koruması güvenli method'ları her zaman geçirir.
8. **Token asla loglanmaz**: `user_id`, `family_id`, `request_id` yeterli.

## 🧪 Nasıl doğrulandı

- **Servis testleri** (fake clock + `memStore`): rotasyon, reuse, mutlak tavan, çıkış, temizlik,
  parola değişimi.
- **Gerçek DB entegrasyonu** (`repository_integration_test.go`) — `time.Sleep` **yok**: bekleme
  `pg_blocking_pids` ile PID bazlı yoklanır, her bekleyiş context ile sınırlıdır, testin kendi
  transaction'ı ayrı bir bağlantıdadır. Her test bir **mutasyonu** korur: aile `FOR UPDATE`'ini
  kaldır, token'ları aile UPDATE'inden önce sil, kilit sırasını ters çevir (gerçek `40P01`),
  reuse hatasını tx içinden dön, `SKIP LOCKED`'ı düşür, `users` kilidini `FOR UPDATE` yap,
  `FOR SHARE` kontrolünü kaldır — hepsi bir testi kırmızıya çevirir.
- **20 goroutine aynı token'la yenileme**: tam 1 kazanan, ≥1 `ReuseError`, kalanlar
  `ErrInvalidToken`; sonunda aile iptal ve 0 token.
- **Handler + vitest**: çerez öznitelikleri, `refresh_token`'ın gövdede olmaması, çoklu çerez,
  çıkış çerez politikası, epoch/HINT kontrolleri, Web Locks sıralaması.
- **Playwright** (11 senaryo, 9 sınırlı istek): üç ardışık yeşil koşu; üç mutasyon senaryoları
  kırmızıya çevirdi — NGINX `Host` başlığını geri almak, `refresh_token`'ı login gövdesine geri
  koymak, iki test arasında `storageState` paylaşmak.
- **Smoke 25/25** (Faz 13 kontrolleri dahil: yanlış mevcut parola → 403, değişim → 200, eski
  parolayla giriş → 401, yenisiyle → 200).

## 🚧 Bilinen sınırlar

- **Access token'lar stateless.** İptal (çıkış/reuse), parola değişimi ve rol değişikliği
  **refresh** tarafını anında keser; o ana kadar verilmiş access token'lar süreleri dolana kadar
  (**≤ `JWT_ACCESS_TTL`, varsayılan 15 dk**) geçerli kalır. Çözümü (token'a
  `password_changed_at`/`token_version` iddiası + hassas uçlarda DB kontrolü) bu fazda yok —
  [REVIEW_FEEDBACK F06](../REVIEW_FEEDBACK.md#f06--admin-yetkisinin-kaldırılması-mevcut-tokenı-hemen-etkilemiyor).
- **Kayıp cevap politikası (F02).** Rotasyon COMMIT oldu ama cevap istemciye ulaşmadıysa (ağ koptu,
  sekme kapandı) tarayıcıdaki çerez harcanmış token'ı taşır; bir sonraki yenileme **gerçekten**
  reuse'dur → aile iptal edilir, kullanıcı yeniden girer. Alternatifi (tolerans penceresi) hırsıza
  fark edilmeden çalışma alanı açardı. `keepalive` bu pencereyi daraltır.
- **DB çökmüşken yapılan çıkış** o tarayıcıda hâlâ geçerli bir refresh çerezi bırakır (yerel arayüz
  kapalıdır). İstemcinin yeniden denemeleri ve açılıştaki `restore` bunu kapatır; kapatamazsa aile
  mutlak tavanda ölür.
- **Migration 0008 çok kopyalı rollout'ta kesintisiz değil**: `TRUNCATE`'ten sonra hâlâ eski kodla
  çalışan pod'lar `family_id`'yi bilmediği için kısa süre giriş hatası verir (demo için kabul;
  sıfır kesinti iki aşamalı bir geçiş isterdi).
- **E2E rate limit bütçesinde yeniden deneme marjı yok.** Kararlı hâl 9/11; ama CI'da
  `retries: 1` düşen HER testi baştan koşar ve auth-flow'un tek bir tekrarı toplamı **13**'e
  çıkarır (> 11) → asıl hata bir 429'un altında kaybolabilir. `scripts/e2e.sh`'ın restart'ı
  yalnız **koşu başında** dolu kovayı garantiler ve CI zaten o betiği koşmuyor.
- **Açık inceleme bulguları** ([REVIEW_FEEDBACK.md](../REVIEW_FEEDBACK.md)): F10'un BookDetail
  yarısı, F16'nın release-gating yarısı ve F03–F09, F11–F15, F17 hâlâ açık.

## ✅ Tamamlanma (DoD)

- [x] Refresh token yalnız httpOnly + Secure + `SameSite=Strict` çerezte; JSON gövdesinde
      `refresh_token` yok; `localStorage`'da sır yok (gerçek tarayıcıda doğrulandı).
- [x] Reuse tespiti: harcanmış token tekrar gelince aile ölür; **halefi de** ölür (e2e'deki ayırt
      edici iddia).
- [x] Mutlak tavan (`JWT_SESSION_MAX_AGE`) rotasyonla aşılamıyor.
- [x] Cross-origin yazma istekleri 403 `cross_origin_request`; `Origin == Host` meşru isteği geçiyor.
- [x] Parola değişimi kullanıcının tüm ailelerini iptal ediyor, işlemi yapan cihaz içeride kalıyor;
      eski parola ölü.
- [x] Deadlock yok: kilit sırası ve kilit güçleri gerçek DB testleriyle (mutasyonlu) korunuyor.
- [x] `make e2e` → 11 senaryo yeşil, 9 sınırlı istek (bütçe 11); `make smoke` 25/25.
- [x] İmajlar digest pinli + slim taban; CI'da Trivy işi; Dependabot açık.

## 🧾 Ne yapıldı (commit'ler)

| Commit | İçerik |
|---|---|
| `ec2da77` | k8s: hiçbir pod'a service account token mount edilmiyor (en az yetki) |
| `36af4ed` | Öğrenme notu: refresh token yaşam döngüsü sektörde nasıl kurgulanıyor (kararların kaynağı) |
| `362dc11` | Migration 0008 + token aileleri, reuse tespiti, mutlak tavan, `SKIP LOCKED` temizliği |
| `c1901ff` | `http.CrossOriginProtection` + NGINX `$http_host` (port'u koru) |
| `1d2e92d` | Refresh token httpOnly çereze; istemcinin oturum durum makinesi yeniden yazıldı |
| `c79707b` | slim nginx tabanı, digest pinleri, CI'da Trivy, Dependabot |
| `1a88cee` | Öğrenme notu: Trivy nedir, Scout neden farklı sayar |
| `90402d8` | Parola değiştirme + tüm ailelerin iptali (`/account` sayfası dahil) |
| `e865548` | F02: kullanıcı rotasyonun kendi transaction'ında okunuyor |
| `2925550` | F10 (Home): bayat katalog cevabı yeni filtrenin üstüne yazamıyor |
| `5192b19` | Playwright suite'i + CI işi + `scripts/e2e.sh` |

## 📒 WHY_GO_VS_TS güncellemesi

- **§7** (context) → `handleLogout`'ta `context.WithoutCancel` + bağımsız 5 sn bütçe: isteğin
  context'i ölse de iptal tamamlanır.
- **§10** (race detector + DB transaction) → `repository_integration_test.go`: `pg_blocking_pids`
  ile deterministik kilit testleri ve gerçek `40P01`.
- **§15** (TS/JS'in daha iyi olduğu yerler) → Web Locks + `storage` olayı: sekmeler arası
  koordinasyon tarayıcı platformunun kendi gücü; sunucu tarafında karşılığı yok.
