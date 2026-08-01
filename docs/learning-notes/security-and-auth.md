# Öğrenme Notları — Security & Auth

Auth, JWT, parola hash, IDOR, rate limit, sır (secret) yönetimi kavramları.
Format ve nasıl çalıştığı: [README.md](README.md).

## İçindekiler

1. [Sır (secret) nedir — dev credential'ı repoya yazmak neden güvenli olabilir?](#1-sır-secret-nedir--dev-credentialı-repoya-yazmak-neden-güvenli-olabilir)
2. [OWASP nedir? (ve BOLA/IDOR — bizim ilk gerçek bulgumuz)](#2-owasp-nedir-ve-bolaidor--bizim-ilk-gerçek-bulgumuz)

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
