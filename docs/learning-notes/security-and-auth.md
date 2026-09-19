# Öğrenme Notları — Security & Auth

Auth, JWT, parola hash, IDOR, rate limit, sır (secret) yönetimi kavramları.
Format ve nasıl çalıştığı: [README.md](README.md).

## İçindekiler

1. [Sır (secret) nedir — dev credential'ı repoya yazmak neden güvenli olabilir?](#1-sır-secret-nedir--dev-credentialı-repoya-yazmak-neden-güvenli-olabilir)
2. [OWASP nedir? (ve BOLA/IDOR — bizim ilk gerçek bulgumuz)](#2-owasp-nedir-ve-bolaidor--bizim-ilk-gerçek-bulgumuz)
3. [Mass-assignment: request DTO neden DB modelinden ayrı?](#3-mass-assignment-request-dto-neden-db-modelinden-ayrı)
4. [Parola hash'leme: neden bcrypt, cost ne demek, 72 bayt tuzağı](#4-parola-hashleme-neden-bcrypt-cost-ne-demek-72-bayt-tuzağı)
5. [JWT nedir ve "algoritma sabitleme" neden en önemli satır?](#5-jwt-nedir-ve-algoritma-sabitleme-neden-en-önemli-satır)
6. [User enumeration: generic mesaj YETMEZ, süre de eşit olmalı](#6-user-enumeration-generic-mesaj-yetmez-süre-de-eşit-olmalı)
7. [Refresh token, rotasyon ve "çıkış yap" (stateless JWT iptal edilemez)](#7-refresh-token-rotasyon-ve-çıkış-yap-stateless-jwt-iptal-edilemez)
8. [Refresh token yaşam döngüsü: sektör nasıl kurguluyor, BookHub nerede?](#8-refresh-token-yaşam-döngüsü-sektör-nasıl-kurguluyor-bookhub-nerede)

---

## 1. Sır (secret) nedir — dev credential'ı repoya yazmak neden güvenli olabilir?

**🗓️ Bağlam:** Faz 2 milestone push'u öncesinde `.env.example`'daki
`postgres://bookhub:bookhub_dev@localhost:5432/bookhub` değerini görüp "lokalde bile olsa
username-password paylaşmak mantıklı mı?" diye sordum.

**⚡ Kısa cevap:** **Bir değeri gizli yapan şey, neyin kilidini açtığıdır.** `bookhub_dev`
sadece kendi makinemdeki, çöp veri taşıyan, silinip yeniden kurulabilen bir container'ın
kilidini açıyor — ve zaten aynı repodaki `docker-compose.yml`'de yazmak zorunda. Kilit ve
anahtar aynı kutuda geliyorsa anahtar sır değildir. **Gerçek sır** (prod DSN, API key) ise
repoya asla girmez.

**📖 Açıklama:**

Dosya rolleri:

| Dosya | Commit'lenir mi | İçeriği |
|---|---|---|
| `.env.example` | **evet** — şablon olmak için var | placeholder veya dev-only değer; **asla gerçek değer** |
| `.env` | **hayır** (gitignore) | senin gerçek lokal değerlerin; gün gelir gerçek sır da barındırır |
| `docker-compose.yml` | evet | dev container'ın kurulum değerleri (şifre burada olmak *zorunda*) |
| Deploy platformu secret store | (repo dışı) | prod DSN, JWT secret, API key — gerçek sırların tek evi |

"Şifre repoda" kulağa yanlış geliyor ama asıl iki gerçek risk başka yerde:

1. **Şifre yeniden kullanımı.** `bookhub_dev`'i günün birinde Supabase'de/başka gerçek bir
   serviste kullanırsam, o an repodan sızmış gerçek bir sır olur. Dev şifresi tek kullanımlık
   uydurma kalmalı. (Ve bir sır git history'ye bir kez girdiyse, dosyayı silmek onu geri
   almaz — rotate etmek gerekir.)
2. **Ağ maruziyeti.** Docker'da `"5432:5432"` portu `0.0.0.0`'a bağlar → aynı Wi-Fi/LAN'daki
   herkes, şifresi repoda yazan dev DB'ye teorik olarak erişebilir. Çözüm (uygulandı):
   `"127.0.0.1:5432:5432"` — sadece kendi makinen.

TS/JS dünyasından fark yok — `.env.example` + gitignore'lu `.env` deseni birebir aynı
(dotenv ekosistemi). Bu dil değil, **12-factor config** meselesi.

Karşı desen (bazı ekipler): `.env.example`'a değer değil placeholder yazmak
(`postgres://USER:PASS@localhost:5432/DB`). Daha "temiz" bir duruş ama lokal dev container
için sıfır güvenlik kazancı + yeni gelenin compose ile elle senkron tutma zahmeti. Localhost
dev değerlerinde gerçek değeri yazmak endüstri standardı.

**🔗 İlgili:**
- [.env.example](../../.env.example) ve [docker-compose.yml](../../docker-compose.yml) — soruyu doğuran dosyalar (127.0.0.1 hardening dahil)
- [docs/PROJECT_BRIEF.md](../PROJECT_BRIEF.md) §7.2 — dev-lokal / prod-Supabase ayrımı, prod DSN'in secret store'da yaşaması
- [backend-and-web.md](backend-and-web.md) — config/12-factor kavramları (env'den config okuma)

---

## 2. OWASP nedir? (ve BOLA/IDOR — bizim ilk gerçek bulgumuz)

**🗓️ Bağlam:** Faz 2'nin push'u öncesinde güvenlik incelemesi bir bulgu çıkardı: `GetBook`
sorgusunda `status = 'APPROVED'` filtresi yoktu, yani `GET /api/books/{id}` ucu onaylanmamış
kitapları da döndürebiliyordu. Açıklarken "OWASP API Top 10'un 1 numaralı maddesi" deyince
"OWASP nedir bilmiyorum" diye sordum.

**⚡ Kısa cevap:** **O**pen **W**orldwide **A**pplication **S**ecurity **P**roject — 2001'de
kurulmuş, kâr amacı gütmeyen, satıcı-bağımsız topluluk; ücretsiz güvenlik referansları üretir.
En bilineni **Top 10** listeleri: sektörün ortak dili. Benim atıf yaptığım madde **BOLA/IDOR**:
*istemci nesneyi id ile ister, sunucu "bu kişi **bu** nesneyi görebilir mi?" sorusunu sormaz.*

**📖 Açıklama:**

Bizi ilgilendiren iki liste:

| Liste | Kapsam |
|---|---|
| **OWASP Top 10** | Genel web uygulamaları (bozuk erişim kontrolü, injection, auth hataları…) |
| **OWASP API Security Top 10** | Sadece API'ler — bizim gibi backend'ler için daha isabetli |

API listesinin 1. maddesi: **BOLA** (Broken Object Level Authorization), eski/yaygın adıyla
**IDOR** (Insecure Direct Object Reference). Bizim koddaki hâli:

```
GET /api/books/1  → 200
GET /api/books/2  → 200   ← PENDING bir kitap olabilir!
GET /api/books/3  → 200
```

Sunucu "id geçerli mi?" diye soruyordu ama "bu kitabı herkes görebilir mi?" diye sormuyordu.
Sıralı `bigint IDENTITY` id'ler bunu kolaylaştırır — tahmin edilecek bir şey yok, 1'den başlayıp
saymak yeter. (UUID kullanmak bunu *zorlaştırır* ama **çözmez**: gerçek çözüm yetki kontrolü.)

Düzeltmede aldığımız tasarım kararı: **sınırı SQL'e göm** (`GetApprovedBook`), handler'daki bir
`if`'e değil. Sebebi fail-safe olmak: yanlış yapılandırılmış bir middleware bile o sorguya
onaylanmamış satır döndürtemez. Ayrıca bulunamayan satır 404 döner — onaylanmamış kitabın
*varlığını* bile sızdırmayız.

Kural (aklımda kalsın): ***"Kim görebilir?" cevabı role göre değişiyorsa ROTAYI ayır; sadece
"ne yapabilir?" değişiyorsa rota aynı kalsın, METODU ayır.*** Aynı ucu rol'e göre içeriden
dallandırmak (`if user.IsAdmin`) güvenlik kararını routing tablosundan handler gövdesine
taşır — denetlenemez ve fail-open olur.

OWASP sadece liste değil: **Cheat Sheet Series** (parola saklama, JWT, oturum yönetimi),
**ASVS** (doğrulama standardı) ve **ZAP** (tarayıcı) da onların. Faz 3'te (auth) bunlar birincil
kaynağımız olacak — `PROJECT_BRIEF §6.1`'deki güvenlik taban çizgisi zaten bu külliyattan
türetilmiş. Dil bağımsız: aynı liste Express/NestJS için de birebir geçerli.

**🔗 İlgili:**
- [backend/sql/queries/books.sql](../../backend/sql/queries/books.sql) — `GetApprovedBook`: sınırın yaşadığı yer
- [backend/internal/books/repository.go](../../backend/internal/books/repository.go) — `Get`: neden "yok" muamelesi yapıyor
- [docs/PROJECT_BRIEF.md](../PROJECT_BRIEF.md) §6.1 (güvenlik taban çizgisi) · [GO_CONVENTIONS.md](../GO_CONVENTIONS.md) §12
- Yukarıda [Girdi 1](#1-sır-secret-nedir--dev-credentialı-repoya-yazmak-neden-güvenli-olabilir) (sır yönetimi)

---

## 3. Mass-assignment: request DTO neden DB modelinden ayrı?

**🗓️ Bağlam:** Faz 2'nin son adımında `POST /api/admin/books` ucunu yazarken, ilk taslakta gövdeyi
doğrudan sqlc'nin ürettiği `db.CreateBookParams`'a okumuştum. Kısa ve temiz görünüyordu. Açığı
**curl ile canlı gösterdik**, sonra kapattık.

**⚡ Kısa cevap:** İstemcinin gövdesini doğrudan DB/domain modeline decode etmek
**mass-assignment** açığıdır: model, istemcinin doldurmaması gereken alanları da taşır
(`status`, `role`, `is_admin`, `user_id`…) ve istemci onları set eder. Çözüm: **kabul edilen
alanları ayrı bir request struct'ında sen tanımla.**

**📖 Açıklama:**

Açığın canlı hâli (kapatmadan önce ölçtük):

```bash
POST /api/admin/books {"title":"Sızma","slug":"sizma","author":"X","status":"APPROVED"}
  → 201  {"id":41, ..., "status":"APPROVED"}
GET /api/books  → "Sızma" listede
```

Saldırgan kitabını gönderdi ve **aynı saniyede yayına aldı** — Faz 4'te yazacağımız admin onay
akışının tamamı anlamsızlaşırdı. Kod tarafında sebep tek satır: `db.CreateBookParams` içinde
`Status` alanı var ve JSON decode onu memnuniyetle doldurur.

Kapattıktan sonra aynı istek:

```
→ 400 {"error":{"code":"invalid_json","message":"bilinmeyen alan: \"status\""}}
```

**İki savunma hattı üst üste** (biri yeterli değil, ikisi birlikte sağlam):

| Hat | Ne yapar | Nerede |
|---|---|---|
| 1. Ayrı request struct | Kabul edilen yüzeyi SEN tanımlarsın; `status` alanı hiç yok | `createBookRequest` (books.go) |
| 2. `DisallowUnknownFields` | Tanımadığı alan gelirse isteği reddeder — sessizce yutmaz | `readJSON` (respond.go) |

Sadece 1 olsaydı: `status` sessizce yok sayılırdı (güvenli ama istemci neden çalışmadığını
bilemezdi). Sadece 2 olsaydı: `CreateBookParams`'a decode ederken `status` "bilinen alan"
olduğu için geçerdi — yani asıl koruma **1. hat**.

**Elle eşlemenin görünmeyen faydası:** alan alan `params := db.CreateBookParams{Title: req.Title, ...}`
yazmak angarya gibi durur, ama şema büyüdüğünde koruma tam da budur — yeni bir DB kolonu
eklendiğinde **otomatik olarak istemciye açılmaz**, buraya bilinçli bir satır eklemek gerekir.

**JS/TS paraleli birebir:** Bu, Rails'in `strong parameters`'ı, Laravel'in `$fillable`'ı,
NestJS'te `class-validator` + `whitelist: true` ile çözdüğü problemin aynısı. Node'da
`Object.assign(user, req.body)` yazmak da tam olarak bu açıktır.

**🔗 İlgili:**
- [backend/cmd/api/books.go](../../backend/cmd/api/books.go) — `createBookRequest` + elle eşleme
- [backend/cmd/api/respond.go](../../backend/cmd/api/respond.go) — `readJSON`, `DisallowUnknownFields`
- Yukarıda [Girdi 2](#2-owasp-nedir-ve-bolaidor--bizim-ilk-gerçek-bulgumuz) — kardeş açık (BOLA/IDOR)
- [docs/GO_CONVENTIONS.md](../GO_CONVENTIONS.md) §11 (DTO ↔ domain ayrımı), §12 (güvenlik temeli)

---

## 4. Parola hash'leme: neden bcrypt, cost ne demek, 72 bayt tuzağı

**🗓️ Bağlam:** Faz 3'te `users` tablosunu ve kayıt akışını kurarken. Login'i bağlarken bcrypt'in
72 bayt davranışını **deneyerek** kontrol ettik ve gerçek bir açık bulduk.

**⚡ Kısa cevap:** Parolayı değil, parolanın **yavaş, tuzlu (salted) hash'ini** saklarsın. bcrypt
bilerek yavaştır (cost 12 ≈ 250 ms) — DB sızsa bile saldırganın deneme hızını milyarlardan
binlere düşürür. Tuzak: bcrypt yalnızca ilk **72 baytı** işler.

**📖 Açıklama:**

| Yöntem | Neden olmaz / olur |
|---|---|
| Düz metin | DB sızınca tüm parolalar okunur. |
| SHA-256 / MD5 | Hızlı olmak için tasarlandı: GPU saniyede **milyarlarca** deneme yapar. Parolada hız düşmandır. |
| bcrypt / argon2id | Bilerek yavaş + her hash'e rastgele **salt** gömülü → aynı parola iki kez farklı hash verir, hazır tablolar (rainbow table) işe yaramaz. |

bcrypt çıktısı her şeyi tek string'de taşır: `$2a$12$<22 karakter salt><31 karakter hash>` —
`12` = **cost**. Cost her +1'de işi ikiye katlar; donanım hızlandıkça artırılır.

**72 bayt tuzağı (canlı bulduğumuz açık):**

```
kayıt parolası : "aaaa…a" (tam 72 bayt)
giriş denemesi : "aaaa…a" + "FAZLADAN"  → bcrypt.CompareHashAndPassword → nil (EŞLEŞTİ!)
```

`GenerateFromPassword` >72 baytta artık hata döner, ama `CompareHashAndPassword` **sessizce
kırpar**. Yani 72 baytlık bir parolanın sonuna ne eklersen ekle giriş açılıyordu. Düzeltme:
kayıtta >72'yi reddet (zaten yapıyorduk), **girişte de** >72'yi doğrudan "hatalı" say. Karakter
değil bayt: "ş" UTF-8'de 2 bayt, yani 37 "ş" = 74 bayt.

**Ders:** güvenlik kütüphanelerinin kenar davranışını dokümana değil **teste** sor. Node'un
`bcrypt` paketi de aynı şekilde 72'de kırpar — bu dil değil, algoritma özelliği.

**🔗 İlgili:**
- [backend/internal/auth/password.go](../../backend/internal/auth/password.go) — `bcryptCost`, `MaxPasswordBytes`
- [backend/internal/auth/service.go](../../backend/internal/auth/service.go) — `Login`'deki 72 bayt koruması
- `TestLoginRejectsTruncatedPassword` — koruma silinince düşen regresyon testi

---

## 5. JWT nedir ve "algoritma sabitleme" neden en önemli satır?

**🗓️ Bağlam:** Faz 3'te access token'ları üretirken (`internal/auth/token.go`).

**⚡ Kısa cevap:** JWT = `başlık.gövde.imza` biçiminde, **sunucunun imzaladığı** bir kimlik kartı.
Sunucu her istekte DB'ye gitmeden "bu kartı ben mi imzaladım, süresi geçmiş mi?" diye bakar.
Doğrularken **algoritmayı token'ın başlığından değil, koddan** almalısın.

**📖 Açıklama:**

```
eyJhbGciOiJIUzI1NiJ9 . eyJzdWIiOiI3Iiwicm9sZSI6IlVTRVIiLCJleHAiOjE3…} . 8f3k…
   başlık (base64)           gövde / claims (base64 — ŞİFRELİ DEĞİL!)        imza
{"alg":"HS256"}        {"sub":"7","role":"USER","exp":…,"iss":…,"aud":…}   HMAC-SHA256(secret)
```

- Gövde **şifreli değil**, sadece base64: herkes okuyabilir. İçine parola/gizli veri koyma.
- Güvenlik imzadan gelir: gövdeyi değiştirirsen (`"role":"ADMIN"`) imza tutmaz.
- `exp` (bitiş), `iss` (kim üretti), `aud` (kimin için) — üçünü de doğruluyoruz.

**Başlığa güvenmenin iki klasik saldırısı:**

| Saldırı | Ne olur | Bizde |
|---|---|---|
| `alg: none` | "imzasız token" — naif doğrulayıcı imzayı hiç kontrol etmez | kütüphane varsayılan olarak reddediyor + testimiz var |
| Algoritma karışıklığı | HS256 bekleyen yere HS512 (ya da RS256) ile oynanmış token | `WithValidMethods([]string{"HS256"})` |

**Mutation ile kanıt:** `WithValidMethods` satırını sildiğimizde, **aynı secret'la HS512 imzalı**
token doğrulamadan geçti. Yani o satır süs değil, taşıyıcı duvar.

**Neden kısa ömür (15 dk)?** Stateless JWT iptal edilemez: çalınırsa süresi dolana kadar
geçerlidir. Süreyi kısa tutarak zarar penceresini daraltırız; uzun oturumu refresh token sağlar
([Girdi 7](#7-refresh-token-rotasyon-ve-çıkış-yap-stateless-jwt-iptal-edilemez)).

**Secret:** HS256'nın gücü secret'ın entropisi kadardır → en az 32 bayt, `openssl rand -base64 48`,
env'den; repoya asla. Sızarsa herkes istediği rolde token imzalar.

**🔗 İlgili:**
- [backend/internal/auth/token.go](../../backend/internal/auth/token.go) — `Issue`, `Verify`
- [backend/internal/auth/token_test.go](../../backend/internal/auth/token_test.go) — 13 red senaryosu
- [docs/GO_CONVENTIONS.md](../GO_CONVENTIONS.md) §12

---

## 6. User enumeration: generic mesaj YETMEZ, süre de eşit olmalı

**🗓️ Bağlam:** Faz 3'te login'i yazarken; canlı olarak yanıt sürelerini ölçtük.

**⚡ Kısa cevap:** "Bu e-posta kayıtlı mı?" sorusunun cevabını sızdırmamak için login hem **aynı
mesajı** hem de **aynı sürede** cevap vermeli. Kullanıcı yoksa da sahte bir bcrypt karşılaştırması
yaparız.

**📖 Açıklama:**

Naif login:

```
e-posta yok   → hemen "hatalı"            (~1 ms)
e-posta var   → bcrypt karşılaştır → "hatalı"  (~250 ms)
```

Mesajlar aynı olsa bile saldırgan **kronometreyle** hangi e-postaların kayıtlı olduğunu çıkarır
(timing attack). Bizim çözüm: kullanıcı yoksa `dummyHash`'e karşı bir karşılaştırma yap.
Canlı ölçüm (5'er istek):

```
e-posta var : 0.231 0.217 0.218 0.217 0.216 sn
e-posta yok : 0.215 0.220 0.217 0.220 0.218 sn   ← ayırt edilemiyor
```

Süreyi testte ölçmek kırılgan olurdu; bunun yerine karşılaştırma fonksiyonunu enjekte edip
**çağrıldığını sayıyoruz** (`TestLoginUnknownEmailStillCompares`).

**Register'daki bilinçli ödünleşim:** `/register` kayıtlı e-postaya **409** dönüyor — bu da bir
sızıntı. Ama aksi hâlde kullanıcı neden kayıt olamadığını anlayamaz. Tam çözüm "e-postana bağlantı
gönderdik" akışıdır (e-posta doğrulama); o gelene kadar toplu sorgulamaya karşı savunma **rate
limit** (Faz 7).

**🔗 İlgili:**
- [backend/internal/auth/service.go](../../backend/internal/auth/service.go) — `dummyHash`, `Login`
- [backend/cmd/api/auth.go](../../backend/cmd/api/auth.go) — `handleRegister`'daki 409 yorumu

---

## 7. Refresh token, rotasyon ve "çıkış yap" (stateless JWT iptal edilemez)

**🗓️ Bağlam:** Faz 3'te oturum yenileme ve logout uçlarını tasarlarken.

**⚡ Kısa cevap:** Kısa ömürlü access token (15 dk) + uzun ömürlü, **DB'de tutulan** refresh
token (7 gün). Refresh token **tek kullanımlıktır**: her kullanımda silinir, yenisi verilir
(rotasyon). "Çıkış yap" = refresh token'ı DB'den silmek.

**📖 Açıklama:**

```
login   → access (15 dk, JWT, DB'de YOK) + refresh (7 gün, DB'de SHA-256'sı var)
istek   → Authorization: Bearer <access>          (DB'ye gidilmez — stateless)
15 dk   → access öldü → POST /refresh {refresh}   → eski refresh SİLİNİR, yeni çift gelir
logout  → POST /logout {refresh}                  → refresh silinir; access en geç 15 dk'da ölür
```

**Neden DB'de hash?** Token 128 bit rastgele (`crypto/rand.Text`) — tahmin edilemez, bu yüzden
bcrypt gibi yavaş hash gerekmez; SHA-256 yeter. DB yedeği sızsa bile içindeki değerlerle oturum
çalınamaz.

**Rotasyonun güvenlik değeri:** çalınmış bir refresh token, gerçek sahibi bir sonraki yenilemeyi
yaptığı anda ölür.

**Yarış koşulu (Faz 2'deki FOR UPDATE dersinin kardeşi):** aynı token'la iki eşzamanlı `/refresh`
gelirse ikisi de başarılı olmamalı. `SELECT` sonra `DELETE` yazsaydık ikisi de SELECT'i geçebilirdi
(check-then-act). Tek atımlık `DELETE ... RETURNING` ile satırı yalnızca biri silebilir — testte
20 goroutine, **tam 1 kazanan**.

**Hata sırası bilinçli:** önce eskiyi tüket, sonra yenisini kaydet. Kayıt başarısız olursa kullanıcı
yeniden giriş yapar (**kapalı başarısızlık**). Ters sıra başarısızlıkta iki geçerli token bırakırdı.

**Bilinen sınır:** "token ailesi" (reuse detection — eski bir token tekrar gelirse o ailenin tüm
token'larını iptal et) yok; öğrenme projesi için bilinçli olarak dışarıda.
*(Güncelleme — Faz 13: token ailesi, reuse tespiti ve mutlak oturum tavanı eklendi; bkz.
[#8](#8-refresh-token-yaşam-döngüsü-sektör-nasıl-kurguluyor-bookhub-nerede).)*

**🔗 İlgili:**
- [backend/migrations/0002_users_auth.sql](../../backend/migrations/0002_users_auth.sql) — `refresh_tokens`
- [backend/sql/queries/users.sql](../../backend/sql/queries/users.sql) — `ConsumeRefreshToken`
- `TestConsumeRefreshTokenConcurrent` — [repository_integration_test.go](../../backend/internal/auth/repository_integration_test.go)
- [database.md](database.md) — transaction / kilit notları

---

## 8. Refresh token yaşam döngüsü: sektör nasıl kurguluyor, BookHub nerede?

**🗓️ Bağlam:** Faz 13'te refresh token'ı localStorage'dan httpOnly çereze taşıyıp token ailesi +
reuse tespiti eklerken "bu genel-geçer bir kullanım mı, gerçek dünyadaki yazılımlarda refresh token
yaşam döngüsü nasıl kurgulanıyor?" diye sordum. Cevabın sonunda "parola değişince tüm oturumları
kapat" eksikliğini görüp onu da Faz 13'e ekletmeye karar verdim.

**⚡ Kısa cevap:** Evet, standart model bu: **kısa ömürlü access token + her kullanımda değişen
(rotasyonlu) refresh token + eski bir token tekrar gelirse o oturum soyunun (ailenin) tamamını
iptal.** IETF'in OAuth güvenlik rehberi (RFC 9700, Ocak 2025) tarayıcı/mobil gibi sır
saklayamayan istemciler için rotasyonu ya da token'ı cihaza bağlamayı (DPoP) şart koşuyor.
Ama tek doğru değil: tek origin'li bir SPA + tek API için klasik sunucu oturumu çoğu zaman daha basit.

**📖 Açıklama:**

**Neden iki token?** Hızlı doğrulama ile iptal edebilmeyi aynı anda almak için:

| | Access token | Refresh token |
|---|---|---|
| Ne zaman gider? | her API isteğinde | yalnız `/refresh` ucuna |
| Nasıl doğrulanır? | imzası kontrol edilir, DB yok (**stateless**) | sunucudaki kayda bakılır (**stateful**) |
| İptal edilebilir mi? | **hayır** — süresi dolana kadar geçerli | **evet** — kaydı silinir |
| Tipik ömür | 5–15 dk | günler–haftalar |

İptal edilemeyenin ömrü kısa tutulur; uzun yaşayan iptal edilebilir yapılır.

**Yaşam döngüsü:**

```
Giriş ──► yeni aile F, T1 verilir
            │  (15 dk sonra access biter)
            ▼
T1 ile /refresh ──► T1 "kullanıldı", T2 verilir      ← rotasyon
            ▼
T2 ile /refresh ──► T3 …
            │
            ├─ çıkış ───────────────────► F iptal (ailedeki her token ölür)
            ├─ 7 gün hiç kullanılmadı ──► son token'ın süresi doldu (hareketsizlik)
            ├─ girişten 30 gün geçti ────► mutlak tavan → yeniden giriş
            ├─ parola değişti ───────────► kullanıcının TÜM aileleri iptal
            └─ ESKİ bir T (T1) geri geldi ► biri çalmış demek → F iptal
```

Sektördeki 7 aşama:

1. **Üretim** — kriptografik rastgele ve **opak** (JWT değil; içinde bilgi taşımaya gerek yok).
   DB'de kendisi değil **hash'i** durur (bkz. #7).
2. **İstemcide saklama** — tarayıcıda `HttpOnly` + `Secure` + `SameSite` çerez (JS okuyamaz);
   mobilde iOS Keychain / Android Keystore. **localStorage önerilmez**: XSS'te script okuyup
   dışarı taşır.
3. **Rotasyon** — her yenilemede yenisi, eskisi "kullanıldı".
4. **Reuse tespiti** — token çalındıysa hırsız da gerçek sahip de AYNI token'ı kullanacak.
   İkinci kullanımı gören sunucu kimin hırsız olduğunu bilemez ama "sızıntı var" der ve aileyi
   öldürür: ikisi de düşer, hırsızın elindeki yeni token da ölür (RFC 9700 §4.14.2). **Aile**,
   kullanıcının tüm oturumları değil: tümünü kapatmak, eline eski bir token geçen herkese
   kullanıcıyı her cihazdan tekrar tekrar atma gücü verirdi.
5. **İki süre** — *hareketsizlik* (kayan pencere, kullandıkça uzar) + *mutlak* (ne olursa olsun
   biter). Mutlak tavan yoksa aileyi ele geçiren, düzenli yenileyerek oturumu sonsuza dek taşır.
6. **İptal olayları** — çıkış, reuse, **parola değişimi**, admin kilidi, "tüm cihazlardan çık".
7. **Temizlik** — süresi dolmuş kayıtlar silinir, tablo sınırsız büyümez.

**Büyük sağlayıcılar** (varsayılanlar sürüme/ayara göre değişebilir):

| Sağlayıcı | Yaklaşım |
|---|---|
| Auth0 | rotasyon + otomatik reuse tespiti (aile iptali); mutlak ömür (30 gün) + hareketsizlik (15 gün); isteğe bağlı tolerans penceresi ("reuse interval") |
| Okta | SPA'da rotasyon; eski token için tolerans penceresi ("grace period", varsayılan 30 sn) |
| Microsoft Entra | SPA refresh token'ı 24 saat yaşar, her kullanımda döner |
| Google | genelde rotasyon yok; uzun ömürlü, uzun süre kullanılmazsa ya da bazı durumlarda parola değişince ölür |
| Keycloak | refresh token sunucudaki oturuma bağlı (boşta / en fazla süre); rotasyon isteğe bağlı ayar |

**BookHub bu yelpazede nerede?**

- **Auth0 modeline çok yakın:** rotasyon + aile iptali + 7 gün kayan + 30 gün mutlak.
- **Tolerans penceresi 0 (bilinçli fark):** pencere açmak, yarışı birkaç saniye önce kazanan
  hırsızın fark edilmemesine izin verir. Aynı tarayıcıdaki eşzamanlı yenilemeyi Web Locks
  zaten sıraya koyuyor. Bedeli: nadiren gereksiz bir "oturum düştü".
- **Kalan zayıflık:** access token hâlâ JS belleğinde → XSS onu en çok 15 dk kullanabilir (ama
  uzun ömürlü sırrı — refresh token'ı — artık çalamaz). En sıkı tarayıcı modeli **BFF**
  (Backend-for-Frontend): token'lar tarayıcıya hiç gelmez, yalnız oturum çerezi taşınır. Bizimki
  "yarı-BFF".
- **İleri seviye, yapılmadı:** **DPoP** (RFC 9449) — token'ı tarayıcıdaki dışarı çıkarılamayan bir
  anahtara bağlar; çalınsa bile başka yerde işe yaramaz (RFC 9700'ün rotasyona alternatifi).
- **Parola değişince tüm ailelerin iptali:** bu soru üzerine Faz 13'e eklendi (işlemi yapan cihaz
  yeni bir aileyle girişli kalır, diğer her cihaz düşer).

**Dürüst not — her zaman bu model mi?** Hayır. Tek origin'deki SPA + tek API için **klasik sunucu
oturumu** (rastgele session id httpOnly çerezde, oturum Redis/DB'de — Rails/Django/Laravel
varsayılanı) çoğu zaman daha basit ve aynı derecede güvenli: iptal anında, rotasyon/aile mantığı
gerekmez. JWT + refresh modeli asıl değerini birden çok servis, mobil istemci ya da harici kimlik
sağlayıcısı (OAuth) olduğunda gösterir. BookHub'da brief ve öğrenme amacıyla bu modeldeyiz.

**🔗 İlgili:**
- [#7 Refresh token, rotasyon ve "çıkış yap"](#7-refresh-token-rotasyon-ve-çıkış-yap-stateless-jwt-iptal-edilemez) — Faz 3'teki ilk hâli
- [#5 JWT](#5-jwt-nedir-ve-algoritma-sabitleme-neden-en-önemli-satır) — access token'ın kendisi
- [database.md](database.md) — kilit sırası / transaction notları (aile iptalinin eşzamanlılık tarafı)
- RFC 9700 §4.14 (refresh token koruması), RFC 9449 (DPoP), IETF taslağı "OAuth 2.0 for Browser-Based Applications" (BFF)
