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
9. [httpOnly çerez mi localStorage mı: neyi çözer, neyi ÇÖZMEZ](#9-httponly-çerez-mi-localstorage-mı-neyi-çözer-neyi-çözmez)
10. [CSRF katmanları: SameSite=Strict, CrossOriginProtection ve login-CSRF](#10-csrf-katmanları-samesitestrict-crossoriginprotection-ve-login-csrf)
11. [Parola değiştirmek neden "her yerden çıkış" demektir](#11-parola-değiştirmek-neden-her-yerden-çıkış-demektir)
12. [Git'e sızan sır: saklamak değil değiştirmek (rotate)](#12-gite-sızan-sır-saklamak-değil-değiştirmek-rotate)

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

## 9. httpOnly çerez mi localStorage mı: neyi çözer, neyi ÇÖZMEZ

**🗓️ Bağlam:** Faz 13'te "httpOnly çerez + token ailesi iptali + Playwright" isteğini uygularken.
`client.ts` refresh token'ı `localStorage`'da tutuyordu; incelemede de "token'ı localStorage'a
taşımayın" notu geçti. Ben de "çereze geçince XSS'i çözmüş mü oluyoruz?" diye sordum — TS/JS
tarafında refleks, token'ı `localStorage`'a koyup `Authorization` başlığıyla göndermektir.

**⚡ Kısa cevap:** httpOnly çerez, **uzun ömürlü sırrın saldırganın sunucusuna kopyalanmasını**
engeller; XSS'i **çözmez**. Enjekte edilmiş bir betik hâlâ `/api/auth/refresh`'e POST atıp
15 dakikalık bir access token alabilir — ama o saldırı kurbanın tarayıcısı açıkken yaşar, çalınan
7 günlük token gibi başka bir makineye taşınamaz.

**📖 Açıklama:**

| | `localStorage` | httpOnly çerez |
|---|---|---|
| JS okuyabilir mi | **evet** (`localStorage.getItem`) | **hayır** (`document.cookie` görmez) |
| XSS sızdırabilir mi | evet, değeri dışarı POST eder | hayır, ama **kullanabilir** (tarayıcı çerezi kendiliğinden ekler) |
| CSRF riski | yok (tarayıcı kendiliğinden göndermez) | **var** → `SameSite` + cross-origin koruması şart (bkz. #10) |
| Kapsam | origin (port dahil) başına | **host** başına, port'a bakmaz (RFC 6265 §8.5) |
| Sunucu ayarı gerekir mi | hayır | evet (`Set-Cookie` öznitelikleri) |

BookHub'ın bugünkü hâli (`backend/cmd/api/auth.go`):

```
Set-Cookie: bookhub_refresh=…; Path=/api/auth; HttpOnly; Secure; SameSite=Strict; Expires=…
```

- **`Path=/api/auth`** — sır yalnızca ona ihtiyaç duyan uçlara gider; `/api/books`'a giden her
  istekte ağda dolaşmaz (ara loglar, proxy'ler, hata ayıklama çıktıları).
- **`Secure`** — düz HTTP'de yazılmaz/gönderilmez. Bu yüzden prod'da **HTTPS şart**; `localhost`
  tarayıcılarca "güvenli bağlam" sayıldığı için lokalde de çalışır.
- **`Expires`** = oturumun **gerçek** (mutlak tavana kırpılmış) bitişi.
- **Ad ayarlanabilir (`COOKIE_NAME`)** — çerezler **host** başına kapsandığı için
  `localhost:3000` (compose) ve `localhost:3001` (kind) aynı çerezi ezerdi. Web Locks ve
  `localStorage` ise origin (port dahil) başınadır; ikisi aynı şeyi kapsamıyor.

**"Peki `localStorage` tamamen boş mu?"** Hayır ama **sır yok**: yalnız `bookhub.session="1"`
ipucu ("bu tarayıcıda bir oturum açık olmalı") ve `bookhub.logout_pending` bayrağı var. İpucu sır
değil — bilinmesi kimseye bir şey kazandırmaz — ama üç iş görür: (1) anonim ziyaretçi her açılışta
boşuna `/refresh` atmaz, (2) bir sekmedeki çıkış diğer sekmelere `storage` olayıyla duyurulur,
(3) uçuştaki bir yenileme, arada çıkış yapıldığını anlayıp oturumu **diriltmez**. Faz 13 öncesinden
kalan `bookhub.refresh` anahtarı açılışta siliniyor.

**Sunucuda çerez okumanın tuzağı:** `r.Cookie` aynı addaki **ilk** çerezi döndürüp gerisini yutar.
Aynı adla birden fazla çerez gelmesi normal değildir; tipik nedeni **gölgeleme (cookie
shadowing)**: saldırgan bir alt alan adı ya da farklı bir `Path` üzerinden ikinci bir çerez
yazdırır, tarayıcı ikisini de gönderir ve istek başlığında `Path`/`Domain` bilgisi olmadığı için
sunucu hangisinin gerçek olduğunu ayırt edemez. Bu yüzden `r.CookiesNamed` ile okuyup "tam 1 tane,
boş değil, ≤ 256 bayt" şartını koyuyoruz; aksi hâlde tahmin etmiyoruz, reddediyoruz.

**Dürüst not:** en sıkı tarayıcı modeli **BFF** (Backend-for-Frontend) — token tarayıcıya hiç
gelmez, yalnız oturum çerezi taşınır. Bizimki "yarı-BFF": access token hâlâ JS belleğinde.

**🔗 İlgili:**
- [#8 Refresh token yaşam döngüsü](#8-refresh-token-yaşam-döngüsü-sektör-nasıl-kurguluyor-bookhub-nerede) — bu kararın çıktığı yer
- [#10 CSRF katmanları](#10-csrf-katmanları-samesitestrict-crossoriginprotection-ve-login-csrf) — çereze geçince açılan yeni kapı
- [docs/phases/phase-13-session-hardening.md](../phases/phase-13-session-hardening.md)
- [backend/cmd/api/auth.go](../../backend/cmd/api/auth.go) (`newRefreshCookie`, `readRefreshCookie`) · [frontend/src/api/client.ts](../../frontend/src/api/client.ts)

## 10. CSRF katmanları: SameSite=Strict, CrossOriginProtection ve login-CSRF

**🗓️ Bağlam:** Faz 13'te refresh token'ı çereze taşımadan hemen önce. "Çerez gelince CSRF diye
bir şey doğuyor mu, `SameSite=Strict` yetmez mi?" diye sordum — `Authorization: Bearer` ile
çalışırken CSRF'i hiç düşünmemiştim, çünkü tarayıcı o başlığı kendiliğinden eklemez.

**⚡ Kısa cevap:** Çerez, tarayıcının isteğe **kendiliğinden eklediği** bir kimliktir; CSRF tam
olarak bunu sömürür. `SameSite=Strict` güçlü ama tek başına yeterli sayılmaz (kardeş alt alan,
eski tarayıcı, yanlış yapılandırma) — bu yüzden ikinci bir katman olarak Go 1.25'in
`http.CrossOriginProtection`'ı zincire takıldı.

**📖 Açıklama:**

**CSRF nedir:** kötü niyetli bir sayfa (evil.example), ziyaretçinin tarayıcısına **bizim**
API'mize bir istek attırır. Tarayıcı bizim sitemize ait çerezleri ekler → istek kullanıcının
kimliğiyle çalışır, kullanıcı habersizdir. Saldırgan cevabı **okuyamaz** (same-origin politikası)
ama yazma isteğinin **yan etkisi** olmuştur bile.

**login-CSRF** ters yönde çalışır ve daha az bilinir: saldırgan kurbanın tarayıcısında **kendi**
hesabıyla oturum açtırır; kurban farkında olmadan saldırganın hesabına yorum yazar, puan verir,
kart bilgisi girer. Çerez tabanlı oturumun doğal riski budur — bu yüzden koruma, çerez gelmeden
**bir commit önce** eklendi.

**İki katman:**

| Katman | Ne yapar | Boşluğu |
|---|---|---|
| `SameSite=Strict` | Başka bir siteden **başlatılan** hiçbir istekte çerez gönderilmez | Kardeş alt alan (`blog.bookhub.example`) "same-site" sayılır → çerez YİNE gider |
| `http.CrossOriginProtection` | `Sec-Fetch-Site` `cross-site` **veya** `same-site` ise yazma isteğini 403'ler | Başlığı hiç göndermeyen istemcide `Origin == Host` yedeğine düşer |

`Sec-Fetch-Site` başlığını **tarayıcının kendisi** yazar; sayfadaki JS onu ne ekleyebilir ne
değiştirebilir (yasaklı başlık). Ama kritik bir ayrıntı var: tarayıcılar Fetch Metadata
başlıklarını **yalnız güvenli bağlama** (HTTPS veya `localhost`) yollar. Düz HTTP'de yayınlanmış
bir kurulumda (ör. `http://192.168.1.5:3000`) bu başlık hiç gelmez ve asıl savunma
`Origin` host:port == `Host` karşılaştırması olur. **Port da karşılaştırılır** — bizim NGINX
`proxy_set_header Host $host` kullanırken portu düşürüyordu (`"localhost:3000" ≠ "localhost"`) ve
tamamen meşru istekler 403 alıyordu; `$http_host`'a geçmek bunu çözdü.

Karar tablosu (`go doc net/http.CrossOriginProtection`):

| Girdi | Sonuç |
|---|---|
| `GET` / `HEAD` / `OPTIONS` | **her zaman geçer** → hiçbir GET ucu durum değiştirmemeli |
| `Sec-Fetch-Site: same-origin` \| `none` | geçer (adres çubuğu, yer imi "none"dur) |
| `Sec-Fetch-Site: cross-site` \| `same-site` | **403** |
| Başlık yok, `Origin` host:port == `Host` | geçer |
| Başlık yok, `Origin` farklı | **403** |
| Ne `Sec-Fetch-Site` ne `Origin` | geçer — curl/smoke gibi istemciler; CSRF bir **tarayıcı** saldırısıdır |

**Neden klasik CSRF token'ı (gizli form alanı / double-submit çerez) değil:** sunucuda durum ya da
istemcide ek bir başlık/çerez dansı ister. `Sec-Fetch-Site` aynı güvenceyi **durumsuz** ve tek
yerde verir. Not: `readJSON` `Content-Type`'a bakmaz — `text/plain` gövdeli bir form CORS
ön kontrolüne (preflight) takılmadan handler'a ulaşabilir, yani asıl kapı gerçekten burasıdır.

**🔗 İlgili:**
- [#9 httpOnly çerez vs localStorage](#9-httponly-çerez-mi-localstorage-mı-neyi-çözer-neyi-çözmez)
- [docs/phases/phase-13-session-hardening.md](../phases/phase-13-session-hardening.md)
- [backend/cmd/api/router.go](../../backend/cmd/api/router.go) (`crossOriginProtection`) · [frontend/nginx/api-proxy.conf](../../frontend/nginx/api-proxy.conf)

## 11. Parola değiştirmek neden "her yerden çıkış" demektir

**🗓️ Bağlam:** Faz 13'te, #8'deki "iptal olayları" listesini konuşurken "parola değişince tüm
aileleri iptal edelim mi?" diye sordun; karar evet oldu ve `POST /api/auth/password` o kararla
yazıldı.

**⚡ Kısa cevap:** İnsanlar parolayı çoğunlukla "hesabıma biri girmiş olabilir" diye değiştirir.
Parolayı değiştirip eski oturumları ayakta bırakmak, kilidi değiştirip hırsızın elindeki anahtarı
çalışır durumda bırakmaktır. Bu yüzden değişim, kullanıcının **tüm** token ailelerini aynı
transaction'da öldürür; işlemi yapan cihaz yeni bir aileyle içeride kalır.

**📖 Açıklama:**

Tek transaction'da olan bitenler (`auth.Repository.ChangePassword`):

1. `users` satırını kilitle, hash'in servisin doğruladığı hash olduğunu teyit et (araya giren
   eşzamanlı bir değişim varsa geri al).
2. Yeni hash'i yaz.
3. Kullanıcının canlı ailelerini deterministik sırayla kilitle → hepsini iptal et → tüm
   token'larını sil.
4. **Bu cihaz için** yeni bir aile + token aç.

Neden tek transaction: yarım kalmış bir durum ("parola değişti ama oturumlar duruyor" ya da
"herkes düştü, parola değişmedi") kullanıcıyı yanlış bir güvenlik hissine sokardı.

**Üç incelik:**

- **Yanlış mevcut parola → 403, 401 değil.** İstemci 401'i "access token bitti" sayıp sessizce
  yenileme yapıp isteği tekrarlar; kullanıcı hatasını hiç göremez, üstüne boşuna bir rotasyon
  harcanır. 403 = "kimliğin belli, ama bu isteğe iznin yok".
- **Kendi rate limit kovası.** Çalınmış bir access token'la mevcut parolayı kaba kuvvetle deneyen
  biri, kurbanın **giriş** kotasını da yiyip onu dışarıda bırakabilirdi. NGINX'te ayrı bir zone
  (`password`, 10r/m) ve API'de ayrı bir anahtar var.
- **`bcrypt`in 72 bayt tuzağı burada da geçerli** (bkz. #4): mevcut parola 72 baytı aşıyorsa
  doğrudan "hatalı" denir.

**Bilinen sınır — iptal ANINDA değil.** Access token stateless bir JWT: parola değişiminden önce
verilmiş access token'lar süreleri dolana kadar (`JWT_ACCESS_TTL`, varsayılan **15 dk**) geçerli
kalır. Yani "her yerden çıkış" refresh tarafında anında, access tarafında 15 dakikaya kadar
gecikmeli. Gerçek "anında iptal" istiyorsan token'a bir `password_changed_at` / `token_version`
iddiası koyup hassas uçlarda DB'den doğrulaman gerekir — bedeli, her istekte bir okuma (yani
stateless olmanın faydasından vazgeçmek).

**Bir de yarış vardı.** `bcrypt` karşılaştırması transaction'ın **dışında** yapılıyor (~250 ms).
O aralıkta parola değişirse, girişin açacağı aile **iptalden sonra** doğar ve eski parolayla
yaşardı. Çözüm: giriş, aileyi açmadan önce hash'i kilit altında yeniden okuyup karşılaştırıyor —
ayrıntısı ve kilit türlerinin neden özellikle seçildiği [database.md #10](database.md#10-satır-kilidi-türleri-for-update-for-no-key-update-for-key-share-for-share)'da.

**🔗 İlgili:**
- [#8 Refresh token yaşam döngüsü](#8-refresh-token-yaşam-döngüsü-sektör-nasıl-kurguluyor-bookhub-nerede) — "iptal olayları" maddesi
- [#4 Parola hash'leme](#4-parola-hashleme-neden-bcrypt-cost-ne-demek-72-bayt-tuzağı) — 72 bayt tuzağı
- [docs/phases/phase-13-session-hardening.md](../phases/phase-13-session-hardening.md)
- [backend/internal/auth/repository.go](../../backend/internal/auth/repository.go) (`ChangePassword`) · [frontend/src/pages/Account.tsx](../../frontend/src/pages/Account.tsx)

---

## 12. Git'e sızan sır: saklamak değil değiştirmek (rotate)

**🗓️ Bağlam:** 2026-10-12'de PR #18'de SonarCloud "C" (main'de "E") güvenlik notu verdi; bulgular
arasında "hard-coded password" görünce "ilk commit'lerde secret'ları açık etmiş miyiz; öyleyse
değiştirmek mi, saklamak mı gerekir?" diye sordum. Tarama 135 commit'te canlı sır bulmadı;
Sonar'ın işaretledikleri [#1](#1-sır-secret-nedir--dev-credentialı-repoya-yazmak-neden-güvenli-olabilir)'deki
dev DSN'leriydi.

**⚡ Kısa cevap:** Git'e (hele push'a) girmiş gerçek bir sırrın çaresi onu **değiştirmektir**:
sağlayıcıda eskisini iptal et, yenisini üret. Geçmişten silmek ikincil ve isteğe bağlı; tek başına
bir şey çözmez, çünkü sır push edildiği an kopyalanmıştır.

**📖 Açıklama:**

Neden "saklamak" yetmez: push edilen bir commit'in kopyaları.

| Kopya | Geçmişi yeniden yazınca (filter-repo + force push) gider mi? |
|---|---|
| Diğer klonlar (başka makinen, CI runner'ları) | Hayır |
| Fork'lar | Hayır |
| CI logları, PR diff'leri, bildirim e-postaları | Çoğu hayır |
| GitHub'ın önbelleği: eski commit SHA'sıyla hâlâ açılabilir | Destek talebi açılana kadar hayır |

Private repo riski azaltır (erişimi olan az kişi ve araç var) ama sıfırlamaz: erişimi olan her
araç ve kişi bir kopya demek.

**Doğru sıra:**

1. **Değiştir (rotate):** sağlayıcının panelinde eski sırrı iptal et, yenisini üret (Supabase
   parola sıfırlama, Upstash token yenileme, JWT secret değiştirme).
2. **Yerleştir:** yenisini Keychain'den doğrudan panele/secret'a aktar
   (`security find-generic-password … | pbcopy` ya da `| gh secret set …`). Sohbete, dosyaya,
   asistana uğramasın.
3. **İzlere bak:** eski sırla yapılmış erişim var mı (sağlayıcının logları).
4. **Geçmiş temizliği, isteğe bağlı ve en son:** değiştirilmiş bir sır artık hiçbir kilidi
   açmadığı için çoğu zaman gerekmez. Bedeli büyük: herkesin geçmişi değişir, açık PR'lar bozulur
   ve bizim süreçte force push zaten yasak.

**Sonar'ın bulgusu gerçek bir sızıntı demek değil.** Araç bir dizenin neyin kilidini açtığını
bilemez; `postgres://bookhub:…@localhost` gördüğü her yeri işaretler. #1'deki ölçüt geçerli: dev
parolası yalnız senin makinendeki ya da CI'daki geçici bir konteyneri açıyor. Böyle bir bulgu
Sonar'da gerekçesiyle "Safe" işaretlenir; kod değişmez.

**Sızdı mı, nasıl anlaşılır:**

```bash
# Tüm dallar, tüm commit'ler; değerler maskeli (--redact)
gitleaks git --redact --log-opts="--all" .

# Canlı sistemlerin izi (yalnız dosya adları basılır)
git log --all -G 'supabase\.co|upstash\.io' --name-only --format=''
```

Bir de aracı sına: boş bir test reposuna sahte bir token koyup yakaladığını gör. "0 bulgu" ancak
araç gerçekten çalışıyorsa bir şey söyler.

Bu konu dilden bağımsız; TS/JS projelerinde de birebir aynı.

**🔗 İlgili:**
- [#1 Sır (secret) nedir](#1-sır-secret-nedir--dev-credentialı-repoya-yazmak-neden-güvenli-olabilir) — dev değeri ile gerçek sırrın farkı
- [CLAUDE.md](../../CLAUDE.md) "Güvenlik" bölümü — "Sır commit'lenmez", sırların Keychain'den doğrudan aktarılması
- [.env.example](../../.env.example) · [docker-compose.yml](../../docker-compose.yml) — Sonar'ın işaretlediği dev DSN'leri
