# BookHub — İnceleme ve geri bildirim takibi

İnceleme tarihi: **19 Eylül 2026**  
Takip dosyasının oluşturulması: **20 Eylül 2026**

İkinci inceleme: **17 Kasım 2026**. F20–F40 ve F04, F07, F08, F10'a eklenen kanıtlar [aşağıda](#ikinci-inceleme--17-kasım-2026).

Bu belge kod, güvenlik, yapılandırma, mimari ve UI/UX incelemesinin bulgularını içerir. Bulgular inceleme anındaki çalışma dizinine, commit edilmemiş değişiklikler dahil, dayanır. Sonraki değişiklikler uygulanmadan önce ilgili bulgunun hâlâ geçerli olduğu kontrol edilmelidir. Bu dosyanın oluşturulması herhangi bir bulgunun düzeltildiği anlamına gelmez.

## Kapsam ve doğrulama sınırları

- İnceleme sırasında uygulama kodunda değişiklik yapılmadı.
- Backend `go test ./...` başarılıydı. Ortam değişkenleri tanımlı olmadığı için gerçek PostgreSQL/Redis entegrasyon testleri bu çalıştırmada atlandı.
- Frontend tip kontrolü ve 146 test başarılıydı.
- Çalışan NGINX'in güvenlik başlıkları HTTP üzerinden doğrulandı.
- Tarayıcı bağlantısı bulunmadığından UI/UX değerlendirmesi bileşen ve CSS incelemesine dayanır; görsel doğrulama yapılmadı.
- Güncel bağımlılık CVE taraması ve tam penetrasyon testi yapılmadı.
- Bulguların bir kısmı koddan çıkarılan hata senaryolarıdır; belirtilen kabul testleriyle doğrulanmalıdır. Üretim riskleri, yerel/demo kurulumun aynı şekilde üretime taşınması bağlamında değerlendirilmiştir.

## Genel değerlendirme

Backend'in temeli iyi kurulmuş. Design pattern'lara dikkat edilmiş; temel sorun desen eksikliği değil, bazı sorumlulukların ve hata senaryolarının uçtan uca tamamlanmamış olması. Öncelikli alanlar oturum yönetimi, frontend veri doğruluğu, üretim yapılandırması ve moderasyon deneyimidir.

## Takip listesi

Kutuları yalnız düzeltme uygulanıp kabul kriteri doğrulandığında işaretleyin. Kısmen çözülen bulguların durumu Bulgu sütununda *eğik* yazılır; ayrıntısı bulgunun altındadır. İlgili bulgunun altına çözüm özeti, commit/PR ve doğrulama sonucu ekleyin. Bilinçli olarak ertelenen veya kabul edilen risklerde gerekçe ve yeniden değerlendirme koşulu yazın.

| Tamamlandı | ID | Öncelik | Bulgu |
|---|---|---|---|
| [x] | F01 | Yüksek | Başarısız logout sonrasında sunucu oturumu canlı kalabiliyor |
| [x] | F02 | Yüksek | Refresh commit'i sonrası hata yeniden denemede oturumu düşürüyor |
| [ ] | F03 | Orta | Çıkış, devam eden refresh ve kilidi bekliyor |
| [ ] | F04 | Yüksek | Rate limit kapsamı kötüye kullanımı yeterince sınırlamıyor — *parola özeti kuyruğu canlıda (#90)* |
| [ ] | F05 | Yüksek — üretim | API ile migration veritabanı yetkileri ayrılmamış — **risk kabul edildi** |
| [ ] | F06 | Orta | Admin yetkisinin kaldırılması mevcut token'ı hemen etkilemiyor |
| [ ] | F07 | Orta | Readiness gerçek hizmet verebilirliği ölçmüyor |
| [ ] | F08 | Orta | İşlem, sorgu ve kilit bekleme süre bütçeleri eksik — *canlıda; canlı ayar doğrulaması bekliyor (#84)* |
| [ ] | F09 | Orta | Cache ve güvenlik sayaçları aynı eviction politikasını paylaşıyor |
| [x] | F10 | Yüksek | Eski frontend yanıtları yeni ekran verisinin üzerine yazabiliyor |
| [ ] | F11 | Orta | Formlarda tekrar gönderim ve eşzamanlı puan yazımı korunmuyor |
| [ ] | F12 | Yüksek — ürün | Moderasyon ekranı karar için gereken içeriği göstermiyor |
| [ ] | F13 | Orta | Mobil yerleşim ve erişilebilirlik eksikleri var |
| [ ] | F14 | Orta | Yükleme, hata ve kullanıcı geri dönüş akışları eksik |
| [ ] | F15 | Orta | Audit kapsamı değişiklik yollarında tutarlı değil |
| [ ] | F16 | Orta | CI/CD gerçek tarayıcı ve release doğrulamasını tamamlamıyor |
| [ ] | F17 | İyileştirme | Frontend oturum sorumlulukları sadeleştirilmeli |
| [x] | F18 | Düşük | Dokümanlar mevcut uygulamayla eşitlenmeli |
| [ ] | F19 | Düşük | `manage migrate` hangi migration'ları uyguladığını yazmıyor |
| [ ] | F20 | Yüksek | Uygulama ile veri ayrı bölgede; istekler sıralı ve çok sorgulu — *bölge taşıması ertelendi (2026-11-18)* |
| [x] | F21 | Orta | Yarıda kesilen istek veritabanı bağlantısını kapatıyor ve hata olarak loglanıyor |
| [ ] | F22 | Orta | Yayın akışı canlıda hangi sürümün çalıştığını doğrulamıyor |
| [ ] | F23 | Orta | Yedek ve geri yükleme tanımlı değil — *PR #101; anahtar ve sır hazır, ilk döküm bekliyor* |
| [ ] | F24 | Orta — ölçekle | Tür süzgeci kalıbı planlayıcının iyi planı kurmasını engelliyor — *canlıda; nadir türde 36 ms (bilinen sınır)* |
| [ ] | F25 | Orta — ölçekle | Her listede tam sayım, derin sayfa maliyeti ve 10.000 sınırı — *canlıda; sayım önbelleği dakikada bir ıskalar* |
| [ ] | F26 | Orta — ölçekle | Puana göre sıralama ve "en sevilenler" önbelleği — *önbellek yarısı canlıda; saklanan skor ertelendi* |
| [x] | F27 | Düşük | Kitap satırı kilidi gereğinden güçlü; puan istatistiği iki yerde hesaplanıyor |
| [x] | F28 | Düşük | Index'siz yabancı anahtarlar ve küçük index düzeltmeleri |
| [ ] | F29 | Orta | Aynı başlıklı ikinci öneri reddediliyor; yazar düz metin — *öneri yarısı canlıda* |
| [x] | F30 | Orta | Geçersiz bayt içeren metin 500 üretiyor |
| [x] | F31 | Orta | `create-admin` var olan hesabı parolasına dokunmadan admin yapıyor |
| [x] | F32 | Orta | Bir isteğin süresinin nereye gittiği görünmüyor |
| [x] | F33 | Orta | Sunucu verisi her sayfada elle çekiliyor |
| [x] | F34 | Orta | Hata sınırı yok; NGINX'in ürettiği hatalar boş mesajla görünüyor |
| [x] | F35 | Orta | Bulunamayan `/assets/` dosyası bir yıl önbelleklenebilir |
| [x] | F36 | Orta | Açılıştaki geçici hata "çıkış yapılmış" gibi görünüyor |
| [ ] | F37 | Orta | Kapak görselleri denetlenmeyen adreslerden yükleniyor — *öneri yarısı canlıda (#104)* |
| [x] | F38 | Orta | CI taze kurulumu sınıyor, canlı yükseltme yapıyor; migration'da süre sınırı yok |
| [ ] | F39 | Düşük | Sınanan imaj çalışan imaj değil; geri dönüş denenmedi; CI zorunlu kontrol değil — *zorunlu kontroller kuruldu (2026-11-19); geri dönüş provası ve imaj açık* |
| [ ] | F40 | Düşük | Küçük bulgular |

## F01 — Başarısız logout sonrasında sunucu oturumu canlı kalabiliyor

**Öncelik:** Yüksek · Güvenlik

**Kaynaklar:** [backend/cmd/api/auth.go](../backend/cmd/api/auth.go) (`handleLogout`), [frontend/src/api/client.ts](../frontend/src/api/client.ts) (`logout`, `postLogout`, tekrar deneme mekanizması).

Backend, veritabanındaki oturum ailesini iptal etmeden önce refresh çerezini siliyor. İptal işlemi başarısız olup 500 dönerse tarayıcı çerezi yine silmiş oluyor. Frontend tekrar denemeyi planlıyor; ancak sonraki istekte artık token yok. Backend çerezsiz logout'a 204 dönüyor. Frontend tamamlandı sayarken aile veritabanında canlı kalabiliyor.

**Etki:** Önceden kopyalanmış bir token, kullanıcı çıkış yaptığını düşünmesine rağmen geçerliliğini koruyabilir.

**Öneri:** Yerel arayüzün kapanması ile sunucudaki iptal işlemini ayrı durumlar olarak tasarlayın. İptal tamamlanana kadar güvenli yeniden deneme mekanizması oturumun kimliğini korumalı. Token'ı localStorage'a taşımayın.

**Kabul kriteri:** İlk logout'ta DB hatası, ikinci denemede iyileşme simüle edilmeli. İlgili ailenin gerçekten iptal edildiği ve eski token'ın yenileme yapamadığı doğrulanmalı; yalnız arayüzün kapanması yeterli değil.

**Durum:** Tamamlandı (20 Eylül 2026)

**Çözüm özeti:** `handleLogout` artık silme çerezini YALNIZCA iki yerde yazıyor: başarı (204) ve çerezsiz/bozuk çerez (204). Servis hatasında (500) çerez KORUNUYOR — aileyi öldürebilecek tek anahtar o çerez; silinseydi istemcinin yeniden denemesi çerezsiz gidip içi boş bir 204 alır ve aile mutlak tavana kadar yaşardı. Ayrıca sunucu kendi içinde sınırlı yeniden deneme yapıyor (200 ms, 1 sn; `context.WithoutCancel` + 5 sn bütçe) ve hepsi başarısızsa `slog.Error` ile "oturum çıkıştan sağ çıktı" kaydı düşüyor. İstemci tarafında `bookhub.logout_pending` bayrağı + sınırlı geri çekilme (2/10/30 sn) ve açılışta (restore) bir deneme daha var; token localStorage'a TAŞINMADI (öneri buydu), çünkü çerez zaten httpOnly kalmalı.

**Commit:** `1d2e92d` (feat(auth): move the refresh token into an httpOnly cookie)

**Doğrulama ve sonuç:** vitest'te çerez modelli sahte sunucu (`fakeLogoutServer`): ilk denemede 500, ikincide başarı senaryosunda iddia `revokes === 1` ve `hasCookie === false` — yani iptal GERÇEKTEN oldu, yalnız arayüz kapanmadı. Go tarafında "iptal ilk denemede patlar, tutunca 204 + silme çerezi" ve "tüm denemeler patlarsa 500 + Set-Cookie YOK" alt-testleri; ikisi de eski davranışta (çerezi önce sil) kırılıyor.

**Kalan risk:** API'ye hiç ulaşılamayan bir çıkışta tarayıcıda GEÇERLİ bir refresh çerezi kalır (yerel arayüz kapalıdır). Yeniden denemeler ve sonraki açılış bunu kapatır; kapatamazsa aile mutlak tavanda (JWT_SESSION_MAX_AGE) ölür.

## F02 — Refresh commit'i sonrası hata yeniden denemede oturumu düşürüyor

**Öncelik:** Yüksek · Sağlamlık

**Kaynaklar:** [backend/internal/auth/service.go](../backend/internal/auth/service.go) (`Refresh`), [backend/internal/auth/repository.go](../backend/internal/auth/repository.go) (`RotateRefreshToken`).

Refresh rotasyonu transaction içinde tamamlanıyor; kullanıcı ardından ayrı bir `UserByID` sorgusunda okunuyor. Eski token harcanıp yenisi kaydedildikten sonra kullanıcı sorgusu hata verirse istemci yeni token'ı alamıyor. Eski çerezle yeniden deneme reuse olarak değerlendirilip bütün aileyi iptal edebiliyor.

**Etki:** Geçici bir hata kalıcı oturum kaybına dönüşüyor. Handler'ın 500 durumunda çerezi koruması bu senaryoyu çözmüyor.

**Öneri:** Kullanıcı bilgisini okuma işlemini rotasyonun tutarlılık sınırına alın. Commit başarılı fakat HTTP yanıtı kayıp senaryosunu ayrıca tasarlayın. Reuse korumasını genel bir tolerans penceresiyle gevşetmeden yeniden denemenin nasıl tanınacağını belirleyin.

**Kabul kriteri:** Rotasyon sonrası kullanıcı okuma hatası ve başarılı commit sonrası yanıt kaybı ayrı testlerle ele alınmalı. Oturum kaybı/yeniden giriş gerektiren durumlar bilinçli bir politika olarak belgelenmeli.

**Durum:** Tamamlandı (20 Eylül 2026)

**Çözüm özeti:** Kullanıcı satırı artık rotasyonun KENDİ transaction'ında okunuyor (`Rotation.User`); servisteki ayrı `UserByID` çağrısı kaldırıldı. Kural olarak yazıldı: COMMIT'ten sonra başarısız olabilecek adım bırakma. Hesap silinmişse rotasyon `errRefreshNotFound` ile geri alınıyor (eskiden bu kontrol servisteydi). "Rolü taze oku" davranışı korunuyor — okuma aynı tx'te, düz SELECT (kilit almaz, kilit sırası değişmezini etkilemez).

**Commit:** (bu commit) fix(auth): read the user inside the rotation transaction

**Doğrulama ve sonuç:** `TestRefreshDoesNotReadUserAfterCommit` — `UserByID` patlatıldığında yenileme YİNE çalışıyor ve halef token kullanılabilir; okuma commit'ten sonraya alınırsa test kırılıyor (denendi). `TestRotateReadsUserInSameTxRealDB` — gerçek DB'de rotasyon TAZE kullanıcıyı (rolü değişmiş hâliyle) dönüyor, hesap silinmişse `errRefreshNotFound`; rotasyon kullanıcıyı okumayı bırakırsa test kırılıyor (denendi). Eski `TestInfraErrorsAreNotAuthErrors/Refresh: kullanıcı okunamadı` alt-testi artık var olmayan kod yolunu sınadığı için kaldırıldı, yerine `Me` alt-testi kondu.

**Kalan risk (bilinçli politika, kodda yorumlandı):** commit başarılı ama CEVAP istemciye ulaşmazsa (ağ koptu, sekme kapandı) tarayıcıdaki çerez harcanmış token'ı taşır; bir sonraki yenileme reuse sayılır ve aile iptal edilir → kullanıcı yeniden giriş yapar. Alternatifi (harcanmış token'ı kısa süre kabul eden tolerans penceresi) çalınmış token'ı yarışta önde olan saldırganın fark edilmeden kullanmasına izin verirdi. İstemcideki `keepalive` bu pencereyi daraltıyor.

## F03 — Çıkış, devam eden refresh ve kilidi bekliyor

**Öncelik:** Orta · Oturum/UX

**Kaynak:** [frontend/src/api/client.ts](../frontend/src/api/client.ts) (`logout`, `withRefreshLock`).

`logout()` önce `refreshing` Promise'ini, sonra Web Lock'u bekliyor. Yerel oturum ancak bunların ardından temizleniyor. Fetch çağrılarında uygulamaya ait açık süre sınırı yok.

**Etki:** Ağ takılırsa Çıkış düğmesi uzun süre etkisiz görünebilir. Kodun hemen yerel çıkış yapılacağı açıklaması sıralamayla uyuşmuyor.

**Öneri:** Çıkış niyetini ve yerel oturum temizliğini beklemelerden önce kaydedin. Sunucu iptalini sıralı şekilde devam ettirin. Geç refresh yanıtının oturumu diriltmesini engelleyen epoch kontrolünü koruyun.

**Kabul kriteri:** Refresh yanıtı bekletilirken çıkış tıklandığında arayüz hemen anonim duruma geçmeli; geç yanıt oturumu geri açmamalı. Diğer sekmelerdeki davranış da doğrulanmalı.

## F04 — Rate limit kapsamı kötüye kullanımı yeterince sınırlamıyor

**Öncelik:** Yüksek · Kötüye kullanım ve kaynak tüketimi

**Kaynaklar:** [backend/cmd/api/router.go](../backend/cmd/api/router.go), [backend/cmd/api/middleware.go](../backend/cmd/api/middleware.go), [frontend/nginx/default.conf](../frontend/nginx/default.conf).

Uygulama rate limit'i login/register uçlarında var. Yorum, öneri, puan yazımı ve refresh için işlem bazlı kota yok. Tek hesap çok sayıda içerik veya token kaydı üretebilir; puan güncellemeleri kitap kilitleri ve istatistik hesaplaması üzerinde yük oluşturabilir. Login koruması yalnız IP'ye bağlı; Redis hatasında fail-open davranışı var. Bcrypt'in maliyeti saldırganı yavaşlatırken sunucu CPU'sunu da tüketir.

**Öneri:** İşlem bazında kullanıcı/IP kotaları, login için hesap ve IP birleşimi, pahalı parola işlemlerine eşzamanlılık sınırı belirleyin. Redis kesintisinde sınırlı yerel koruma değerlendirin. Tekrar login olmayan kullanıcıların eski refresh kayıtları için periyodik, sınırlı temizlik ihtiyacını da değerlendirin.

**Kabul kriteri:** Kota aşımı 429 ve uygun Retry-After üretmeli. Çok instance ve Redis kesintisi senaryoları doğrulanmalı. Meşru kullanıcı akışları için limitler ölçülerek seçilmeli.

**Ek kanıt (17 Kasım 2026):** Yerel API 0,1 işlemciyle sınırlanarak ölçüldü (bu makinede tek özet 2,2 sn; canlıda belgelenen değer 3,3 sn). Tek giriş denemesi 2,2 sn sürdü; dört eşzamanlı denemenin her biri 8,4–9,0 sn; beş eşzamanlı denemenin beşi de 502 döndü (Go'nun 10 sn'lik yazma süresi doldu, NGINX "upstream prematurely closed connection" yazdı). Üç deneme sürerken katalog isteği 0,1 sn yerine 1,3 sn sürdü. Canlıda eşik 3–4 eşzamanlı giriş: bu kadarı girişi herkes için düşürür ve o sırada bütün API yavaşlar. İki ek gözlem: kayıt iki özet hesaplatıyor (`register`, ardından `login`; [Auth.tsx](../frontend/src/pages/Auth.tsx)) ve kayıtta özet, e-postanın kayıtlı olup olmadığına bakılmadan önce hesaplanıyor ([service.go](../backend/internal/auth/service.go)). **Önerinin somut hâli:** parola özeti çevresinde tek yuvalı bir kuyruk; kuyruk doluysa 503 ve `Retry-After`. `register` oturumu da döndürsün (tek özet).

**İlerleme (17 Kasım 2026, #83, PR #90, canlıda 2026-11-18):** Parola özeti kuyruğu eklendi (`auth.Service.slow`): aynı anda tek özet çalışıyor, sırası 5 sn'de gelmeyen istek 503 `busy` + `Retry-After` alıyor (kayıt, giriş, parola değişimi). Var olmayan hesap da aynı cevabı alıyor. 0,1 işlemcide ölçüm yayından sonra yapılacak; bulgunun rate limit kapsamıyla ilgili diğer maddeleri açık.

## F05 — API ile migration veritabanı yetkileri ayrılmamış

**Öncelik:** Yüksek — aynı yapı üretime taşınırsa · En az yetki

**Kaynaklar:** [docker-compose.yml](../docker-compose.yml), [deploy/k8s/api.yaml](../deploy/k8s/api.yaml), [deploy/k8s/migrate-job.yaml](../deploy/k8s/migrate-job.yaml), [0003_moderation.sql](../backend/migrations/0003_moderation.sql).

Gösterilen kurulumlarda API, veritabanını oluşturan kullanıcıyla bağlanıyor; migration da aynı kullanıcıyı kullanıyor. Audit tablosunun UPDATE/DELETE/TRUNCATE trigger'ları faydalı, fakat tablo sahibi veya superuser olan uygulamaya karşı değiştirilemezlik garantisi vermiyor: Şema ve trigger değiştirilebilir.

**Öneri:** Migration sahibi ile sınırlı runtime rolünü ayırın. API'ye yalnız gereken tablo/sequence yetkileri, audit için gereken INSERT/SELECT yetkileri verilsin. Yerel demo kolaylığı üretim rol tasarımından açıkça ayrılmalı.

**Kabul kriteri:** Runtime rolüyle gerekli API işlemleri çalışmalı; DROP, ALTER, trigger kapatma ve audit kayıtlarını değiştirme girişimleri reddedilmeli.

**Referans:** [PostgreSQL yetki modeli](https://www.postgresql.org/docs/16/ddl-priv.html).

**Durum:** Risk kabul edildi (5 Ekim 2026)

**Gerekçe:** Kullanıcı kararı — canlıda ayrı, kısıtlı bir veritabanı kullanıcısı açılmayacak. BookHub bir öğrenme/demo projesi; canlı API Supabase'e sahip (owner) kullanıcıyla bağlanıyor, migration'lar da (Deploy iş akışı ya da Mac'ten `manage migrate`) aynı kullanıcıyla uygulanıyor.

**Kalan risk:** API ele geçirilirse şema ve audit trigger'ları değiştirilebilir; audit kayıtlarının değiştirilemezliği yalnız uygulama dışından gelen yazmalara karşı geçerli.

**Yeniden değerlendirme koşulu:** Proje gerçek kullanıcı verisi taşıyan bir ürüne dönüşürse ya da audit kayıtları bir uyumluluk gereği hâline gelirse bulgu yeniden açılır. O zamana kadar AI review bu konuyu engelleyici bulgu olarak açmaz ([CLAUDE.md](../CLAUDE.md)).

## F06 — Admin yetkisinin kaldırılması mevcut token'ı hemen etkilemiyor

**Öncelik:** Orta · Yetkilendirme politikası

**Kaynak:** [backend/cmd/api/middleware.go](../backend/cmd/api/middleware.go) (`requireAuth`, `requireAdmin`).

Yetki kararı access token içindeki role dayanıyor. Kullanıcının güncel rolü ve hesap durumu burada okunmuyor. Rol düşürülse bile mevcut token süresi dolana kadar admin yetkisi taşır. Logout da mevcut access token'ı hemen iptal etmiyor.

**Değerlendirme:** Bu stateless JWT'nin bilinçli bir ödünleşimi olabilir; otomatik olarak uygulama hatası değildir. Ancak erişimi hemen kesme ihtiyacını karşılamaz.

**Öneri:** Admin uçlarında güncel yetki kontrolü veya oturum/token sürümü değerlendirin. Normal kullanıcılar için kısa TTL yeterli kabul edilecekse süre ve kabul edilen risk belgelensin.

**Kabul kriteri:** Admin rolü kaldırılmış kullanıcının eski token'ıyla davranış test edilmeli; sonuç kararlaştırılmış iptal süresine uymalı.

## F07 — Readiness gerçek hizmet verebilirliği ölçmüyor

**Öncelik:** Orta · Altyapı

**Kaynaklar:** [backend/cmd/api/router.go](../backend/cmd/api/router.go) (`handleHealth`), [deploy/k8s/api.yaml](../deploy/k8s/api.yaml).

`/health` koşulsuz 200 dönüyor. Readiness ve liveness aynı endpoint'i kullanıyor. Başlangıç DB kontrolünden sonra bağlantı kaybolursa pod hazır kalıp trafik almaya devam ediyor.

**Öneri:** Liveness süreç sağlığını; readiness kısa süre sınırıyla zorunlu bağımlılıklara erişimi ölçsün. İsteğe bağlı Redis'in kesintisi otomatik olarak readiness başarısızlığına dönüştürülmemeli.

**Kabul kriteri:** DB erişimi kesildiğinde readiness başarısız, süreç çalışırken liveness başarılı olmalı. DB geri geldiğinde pod yeniden hazır olmalı.

**Ek kanıt (17 Kasım 2026):** Canlıda Northflank'ta hiç sağlık kontrolü tanımlı değil ([DEPLOYMENT.md](DEPLOYMENT.md) §5). Hazırlık kontrolü olmadan, açılışta düşen bir sürüm (hatalı ortam değişkeni, duraklamış Supabase) başarısız bir yayın yerine kesintiye dönüşebilir; Northflank'ın bu durumdaki davranışı doğrulanmadı. İlk adım kod gerektirmiyor: `api` için `/health`, `web` için `/healthz` kontrolü panelden tanımlanır.

## F08 — İşlem, sorgu ve kilit bekleme süre bütçeleri eksik

**Öncelik:** Orta · Kaynak yönetimi

**Kaynaklar:** [backend/cmd/api/main.go](../backend/cmd/api/main.go), [backend/internal/database/database.go](../backend/internal/database/database.go).

HTTP ReadTimeout/WriteTimeout var; normal handler işlemleri için genel context deadline, sorgu bütçesi veya yapılandırılmış lock_timeout görünmüyor. Bağlantının yazma süresini sınırlamak çalışan iş mantığının aynı anda iptalini garanti etmez.

**Öneri:** İstek, sorgu ve kilit bekleme sürelerini uyumlu bir bütçeyle sınırlayın. Havuz kapasitesini instance sayısı × bağlantı sayısı üzerinden planlayın.

**Kabul kriteri:** Uzun sorgu/kilit beklemesi süre sınırında sonlanmalı; bağlantı havuzu ve goroutine kaynakları serbest kalmalı.

**Referans:** [Go WriteTimeout ve request context davranışı](https://github.com/golang/go/issues/59602).

**Ek kanıt (17 Kasım 2026):** Bağlantı havuzu bütünüyle varsayılan ayarlarla kuruluyor (`pgxpool.New`). En çok bağlantı sayısı `max(4, çekirdek sayısı)`: 0,1 vCPU kotasını değil düğümün çekirdek sayısını izler; Supabase'in oturum havuzu ise varsayılan 15 bağlantı ve fazlası bir dakikaya kadar sıraya girer. En az bağlantı 0 ve boşta kalma süresi 30 dakika olduğu için seyrek trafikte ilk istekler yeni bir TLS bağlantısı kurar. Düzeltirken dikkat: `pool_max_conns`'ı `DATABASE_URL`'e yazmak `manage migrate`'i bozar, çünkü aynı adres `database/sql` üzerinden de açılıyor ve pgx bilinmeyen anahtarı sunucuya iletir (pgx kaynağı okunarak); değer koddan verilmeli. Canlıdaki çekirdek sayısı ve havuz ayarı doğrulanmadı. Migration oturumunun süre sınırları için F38.

**İlerleme (17 Kasım 2026, #84, PR #91, canlıda 2026-11-18; canlı ayar doğrulaması #84'te bekliyor):** Süre bütçeleri: okuma 5 sn, yazma 9 sn (ikisi de WriteTimeout'tan kısa); bütçesi dolan istek 503 `timeout` + `Retry-After` döner. Her veritabanı bağlantısı `statement_timeout` 10 sn, `lock_timeout` 3 sn, `idle_in_transaction_session_timeout` 15 sn ile açılır (`AfterConnect`; migration bağlantısı etkilenmez). Havuz koddan kurulur: en çok bağlantı `DB_MAX_CONNS` (varsayılan 6), en az 1, boşta kalma 5 dk. Açık kalanlar: canlıdaki çekirdek sayısı ve Supabase havuz sınırı panelden doğrulanmadı; `SET` komutlarının Supabase oturum havuzlayıcısında kalıcı olduğu yayından sonra `SHOW statement_timeout` ile doğrulanacak.

## F09 — Cache ve güvenlik sayaçları aynı eviction politikasını paylaşıyor

**Öncelik:** Orta · Yapılandırma

**Kaynaklar:** [docker-compose.yml](../docker-compose.yml), [deploy/k8s/redis.yaml](../deploy/k8s/redis.yaml).

Redis hem cache hem rate limit için kullanılıyor; politika allkeys-lru. Bellek baskısında rate limit anahtarları da atılabilir ve kota erken sıfırlanabilir.

**Öneri:** Üretimde cache ve güvenlik sayaçlarının kaynak/politika ihtiyaçlarını ayırın. Aynı instance içinde farklı Redis DB numaraları bu bellek politikasını ayırmaz. Eviction ve hata metriklerini izleyin.

**Kabul kriteri:** Bellek baskısında güvenlik sayaçlarının davranışı doğrulanmalı; seçilen hata politikasıyla kota koruması öngörülebilir olmalı.

**Referans:** [Redis eviction politikaları](https://redis.io/docs/latest/develop/reference/eviction/).

## F10 — Eski frontend yanıtları yeni ekran verisinin üzerine yazabiliyor

**Öncelik:** Yüksek · İşlevsel doğruluk

**Kaynaklar:** [frontend/src/pages/Home.tsx](../frontend/src/pages/Home.tsx), [frontend/src/pages/BookDetail.tsx](../frontend/src/pages/BookDetail.tsx), [frontend/src/components/usePaged.tsx](../frontend/src/components/usePaged.tsx).

usePaged içinde eski yanıt koruması var; ana katalog ve kitap detayında aynı koruma yok. A filtresinden B'ye geçildiğinde A'nın geç yanıtı B sonuçlarını ezebilir. Kitap detayında route değişince eski book hemen temizlenmiyor; eski içerik ile yeni route ID'si bir süre birlikte bulunabilir.

**Etki:** URL ile gösterilen veri ayrışabilir. Eski kitap görünürken yeni kitap ID'sine puan/yorum gönderilmesi riski oluşur.

**Öneri:** Kaynak anahtarına bağlı yükleme durumu, eski yanıt koruması ve gerekiyorsa AbortController kullanın. Görünen kitap ile işlem hedefi eşleşmeden yazma kontrollerini etkinleştirmeyin.

**Kabul kriteri:** Yanıtlar ters sırayla tamamlandığında yalnız güncel sorgu/kitap gösterilmeli. Route değişimi sırasında eski içeriğin üzerinden yeni kaynağa işlem yapılamamalı.

**Durum:** Kısmen tamamlandı (20 Eylül 2026) — **Home yarısı kapandı, BookDetail açık** (kutu bu yüzden işaretli değil).

**Çözüm özeti:** `Home.tsx`'teki katalog effect'i artık kendi temizliğinde (cleanup) önceki isteği BAYAT olarak işaretliyor ve o isteğin sonucu — başarı da hata da — yok sayılıyor (`usePaged`'de zaten var olan korumanın kataloga taşınmış hâli). BookDetail'deki iki sorun (route değişince eski `book`'un hemen temizlenmemesi, eski içerik görünürken yeni ID'ye yazma) bu commit'in kapsamı dışında; ayrı bir düzeltme olarak açık kalıyor.

**Commit:** `2925550` (fix(web): drop stale catalog responses so a slow query cannot overwrite a new filter) — kısmi.

**Doğrulama ve sonuç:** Hatayı yeni Playwright katalog senaryosu buldu: yerel, kalabalık bir veritabanında filtresiz sorgu (tüm katalog taranır) "Dune" aramasından yavaş dönüyor ve ızgaranın üstüne boyuyordu (47 ms'e karşı 243 ms; URL `?q=Dune` derken liste her kitabı gösteriyordu). Regresyon testi `Home.test.tsx`'te iki isteği elle TERS sırayla çözüyor → düzeltme olmadan zamanlamadan bağımsız olarak kırılıyor.

**Kalan risk / erteleme gerekçesi:** BookDetail'de route değişimi sırasında eski kitap kısa süre görünmeye devam edebilir; o ekrandaki puan/yorum yazma butonları görünen kitapla hedefin eşleştiğini henüz şart koşmuyor.

**İlerleme (17 Kasım 2026):** BookDetail yarısı da kapanmış görünüyor: bileşen `key={id}` ile yeniden kuruluyor ([BookDetail.tsx](../frontend/src/pages/BookDetail.tsx)); regresyon testi "başka kitaba geçince öncekinin içeriği yeni kitap yüklenirken kalmaz" ([BookDetail.test.tsx](../frontend/src/pages/BookDetail.test.tsx)). Test koşusu 17 Kasım'da yeşil (vitest, 374 test); kutu işaretlendi.

## F11 — Formlarda tekrar gönderim ve eşzamanlı puan yazımı korunmuyor

**Öncelik:** Orta · UX ve veri doğruluğu

**Kaynaklar:** [BookDetail.tsx](../frontend/src/pages/BookDetail.tsx), [Auth.tsx](../frontend/src/pages/Auth.tsx), [Suggest.tsx](../frontend/src/pages/Suggest.tsx).

Formlarda submitting durumu yok. Çift tıklama/yavaş ağ tekrar gönderim üretebilir; yorumlarda çift kayıt oluşabilir. Hızlı puan seçimlerinde son tıklanan ile sunucuda son işlenen değer ayrışabilir. Kayıt başarılı fakat otomatik login başarısız olduğunda hesabın oluşturulduğu açıkça anlatılmıyor.

**Öneri:** İşlem sırasında ilgili kontrolleri kilitleyin ve ilerleme durumu gösterin. Puan değişikliklerini sıralayın veya son kullanıcı niyetini koruyun. Kayıt ve otomatik giriş sonuçlarını ayrı yönetin.

**Kabul kriteri:** Çift gönderim tek işlem oluşturmalı. Hızlı puan seçimlerinde ekran ve DB aynı nihai değeri göstermeli. Kayıt başarılı/login başarısız durumunda kullanıcı yeniden kayıt olmaya yönlendirilmemeli.

## F12 — Moderasyon ekranı karar için gereken içeriği göstermiyor

**Öncelik:** Yüksek · Ürün ve moderasyon UX

**Kaynak:** [frontend/src/pages/admin/Moderation.tsx](../frontend/src/pages/admin/Moderation.tsx).

Kuyruk başlık, yazar ve tarih gösteriyor; açıklama, kapak, diğer metadata ve audit geçmişini inceleyecek detay akışı yok. Admin halka açacağı açıklamayı inceleyemeden onay verebiliyor. Backend'deki detay/düzenleme/geçmiş yetenekleri UI'da tamamlanmamış. Red window.prompt ile yapılıyor; reddedilen durumdan çıkış bulunmuyor.

**Öneri:** Tam içerik, düzenleme, geçmiş ve karar alanını içeren inceleme ekranı ekleyin. Red gerekçesi ve kararın sonucunu açık gösterin. Yanlış kararın nasıl düzeltileceğini ürün kuralı olarak belirleyin.

**Kabul kriteri:** Admin yayınlanacak bütün alanları karar öncesi görebilmeli. Red iptali değişiklik yapmamalı; karar düzeltme yolu veya geri alınamazlık politikası açık olmalı.

## F13 — Mobil yerleşim ve erişilebilirlik eksikleri var

**Öncelik:** Orta · UI/UX ve erişilebilirlik

**Kaynaklar:** [frontend/src/styles.css](../frontend/src/styles.css), [Home.tsx](../frontend/src/pages/Home.tsx), [BookDetail.tsx](../frontend/src/pages/BookDetail.tsx), [Moderation.tsx](../frontend/src/pages/admin/Moderation.tsx).

Grid minimum kart genişliği 320 px; toplam yatay padding 48 px. 360 px viewport'ta içerik alanı 312 px kalıyor ve minimum grid genişliği bunu aşıyor. Üst navigasyon, arama satırı ve moderasyon sekmelerinde dar ekran davranışı yetersiz görünüyor. Bu tespit CSS'e dayanır; tarayıcıda doğrulanmalıdır.

Erişilebilirlik eksikleri:

- Arama input'u ve tür select'inin açık etiketi yok.
- Yorum textarea'sı placeholder'a dayanıyor.
- Seçili puan yalnız CSS sınıfıyla belirtiliyor.
- Histogram oy sayıları ağırlıklı olarak title üzerinden sunuluyor.
- Başarı mesajında canlı bildirim semantiği yok.

**Öneri:** Grid minimumunu kullanılabilir alanla sınırlayın. Mobil navigasyon/filtre yerleşimini tasarlayın. Etiketler, uygun radio veya aria-pressed semantiği ve erişilebilir histogram açıklaması ekleyin.

**Kabul kriteri:** 320/360/390 px ekranlarda yatay taşma olmamalı. Klavye ve ekran okuyucuyla alanların adı, seçili puan ve işlem sonucu anlaşılmalı.

## F14 — Yükleme, hata ve kullanıcı geri dönüş akışları eksik

**Öncelik:** Orta · Kullanıcı akışı

**Kaynaklar:** [Home.tsx](../frontend/src/pages/Home.tsx), [BookDetail.tsx](../frontend/src/pages/BookDetail.tsx), [App.tsx](../frontend/src/App.tsx), [Auth.tsx](../frontend/src/pages/Auth.tsx), [Suggest.tsx](../frontend/src/pages/Suggest.tsx).

- Kitap yüklenirken ErrorBox(null) nedeniyle içerik boş kalıyor.
- Home'da eski hata sonraki başarılı aramada temizlenmiyor.
- Arama input'u URL q değişince senkronize edilmiyor; geri/ileri gezinmede ayrışabilir.
- Login sonrası kullanıcı geldiği sayfa yerine ana sayfaya gönderiliyor.
- Kitap detayındaki giriş çağrısı bağlantı değil.
- Öneri sonrası kullanıcının durum takip edeceği ekran yok.

**Öneri:** Her veri bölümü için yükleniyor/başarılı/boş/hatalı durumlarını tasarlayın. Güvenli, uygulama içi dönüş adresini koruyun. Önerilerim akışıyla moderasyon döngüsünü kullanıcı açısından tamamlayın.

**Kabul kriteri:** Yavaş ağda boş ekran olmamalı; başarılı tekrar deneme eski hatayı kaldırmalı. Geri/ileri gezinmede URL/input/sonuç eşleşmeli. Giriş sonrası amaçlanan akış sürmeli; öneri durumu izlenebilmeli.

## F15 — Audit kapsamı değişiklik yollarında tutarlı değil

**Öncelik:** Orta · Mimari ve izlenebilirlik

**Kaynaklar:** [backend/internal/moderation/service.go](../backend/internal/moderation/service.go), [backend/cmd/api/books.go](../backend/cmd/api/books.go) (`handleSetBookGenres`), [backend/cmd/api/genres.go](../backend/cmd/api/genres.go).

Moderasyon geçişleri transaction içinde audit oluşturuyor; kitap-tür ilişkisi doğrudan repository üzerinden değişiyor. Tür yönetiminde aynı merkezi denetim yaklaşımı yok. Kitap düzenleme audit'i yalnız değişen alan adlarını saklıyor; önceki/sonraki değerler tutulmuyor.

**Öneri:** Denetlenecek işlemleri açıkça tanımlayın ve servis sınırından geçirin. Gerekli alanların önce/sonra değerlerini aynı transaction'da kaydedin; hassas veriyi audit'e körlemesine kopyalamayın.

**Kabul kriteri:** Kapsamdaki her başarılı değişiklik doğru aktör ve değişiklik bilgisiyle audit üretmeli. İşlem başarısızsa veri ve audit birlikte geri alınmalı.

## F16 — CI/CD gerçek tarayıcı ve release doğrulamasını tamamlamıyor

**Öncelik:** Orta · Kalite güvencesi

**Kaynaklar:** [.github/workflows/ci.yml](../.github/workflows/ci.yml), `.github/workflows/release.yml` (2026-10-12'de kaldırıldı), [backend/scripts/mvp_smoke.sh](../backend/scripts/mvp_smoke.sh).

CI E2E işi localhost:8080 API'sine doğrudan curl gönderiyor. NGINX, tarayıcı çerezleri, Web Locks, çoklu sekme ve mobil yerleşim bu yolla doğrulanmıyor. Release workflow'u kendi içinde başarılı test sonucuna bağlı olmadan imaj yayınlayabiliyor. Yalnız Kubernetes/Makefile değişiklikleri CI path filtrelerine girmiyor.

**Öneri:** Gerçek web origin'i üzerinden az sayıda kritik tarayıcı testi ekleyin: kayıt/giriş, yenileme, iki sekme, çıkış, yorum ve moderasyon. Release'i aynı commit'in başarılı doğrulamasına bağlayın. Altyapı dosyalarını CI kapsamına alın.

**Kabul kriteri:** Kritik tarayıcı akışları CI'da çalışmalı. Başarısız veya doğrulanmamış commit yayınlanmamalı. Altyapı değişiklikleri ilgili kontrolleri tetiklemeli.

**Durum:** Kısmen tamamlandı (20 Eylül 2026) — **tarayıcı yarısı kapandı, release doğrulaması ve path filtreleri açık** (kutu bu yüzden işaretli değil).

**Çözüm özeti:** CI'nın `e2e` işi artık curl'e ek olarak GERÇEK bir tarayıcıda (chromium) 11 Playwright senaryosu koşuyor: katalog, kayıt → yenileme → ikinci cihaz → parola değişimi, çerez hijyeni + reuse, üç sekme (Web Locks), cross-origin 403 (ve `Origin == Host` → 401 regresyonu), puan + yorum, `window.prompt` ile moderasyon. Hepsi NGINX'in arkasından, demo'nun sunduğu yığına karşı. Ayrıca yeni bir `images` işi iki imajı Trivy ile tarıyor. **Kapsanmayan:** `release.yml` hâlâ aynı commit'in yeşil doğrulamasına bağlı değil ve yalnız Kubernetes/Makefile değişiklikleri CI path filtrelerine girmiyor.

**Commit:** `5192b19` (test(e2e): real-browser Playwright suite and its CI job) — kısmi; `c79707b` (CI'daki imaj taraması).

**Doğrulama ve sonuç:** Üç ardışık yeşil koşu (11 passed, 0 flaky, 429 yok). Üç mutasyon senaryoları kırmızıya çevirdi: NGINX'in `Host` başlığını `$host`'a geri almak, `refresh_token`'ı login gövdesine geri koymak ve iki test arasında `storageState` paylaşmak (reuse tespitine takılır — her testin kendi girişini yapmasının sebebi). Rate limit bütçesi sert kural olarak yazıldı: NGINX'in login|register kovası 11 istek, suite 9 harcıyor (tablo `frontend/e2e/helpers.ts`), 429 gelirse test sessizce yeniden denemek yerine adıyla patlıyor.

**Kalan risk / erteleme gerekçesi:** `release.yml` hâlâ hiçbir CI sonucuna bağlı değil (`needs`/`workflow_run` yok) → doğrulanmamış bir commit'ten imaj yayınlanabilir. Path filtrelerine `scripts/**` eklendi (Makefile zaten vardı) ama `deploy/k8s/**` hâlâ dışarıda: yalnız Kubernetes manifesti değiştiren bir commit CI'yı tetiklemez.

**İlerleme (5 Ekim 2026):** Canlıya giden yol CI'a bağlandı: Northflank yalnız `release` dalını yayınlıyor; o dalı da yalnız `deploy.yml` ilerletiyor — main'de `ci.yml` başarıyla bitince ve yalnız CI'ın test ettiği commit'e (`workflow_run` + fast-forward). `release.yml` artık yayın yolunun parçası değil (ghcr imajı); CI'a bağlanması ya da kaldırılması ve `deploy/k8s/**` path filtresi hâlâ açık.

**İlerleme (12 Ekim 2026):** `release.yml` kaldırıldı (hiç çalışmamıştı; Northflank imajı `release` dalından kendisi derliyor). Açık kalan: `deploy/k8s/**` path filtresi.

## F17 — Frontend oturum sorumlulukları sadeleştirilmeli

**Öncelik:** İyileştirme · Mimari

**Kaynaklar:** [frontend/src/api/client.ts](../frontend/src/api/client.ts), [frontend/src/auth/AuthContext.tsx](../frontend/src/auth/AuthContext.tsx).

HTTP istemcisi, token belleği, sekmeler arası koordinasyon, logout tekrar denemesi ve abonelik tek modülde. AuthContext kullanıcı durumunu ayrıca tutuyor. F01–F03 gibi hatalar bu karmaşıklığın somut sonuçları.

**Öneri:** Yeni framework/desen eklemeden önce oturum geçişlerini açık bir durum modeliyle tanımlayın. Oturum için tek sorumluluk sahibi belirleyin; HTTP taşıma ve sekmeler arası koordinasyon sınırlarını netleştirin. Refactor'ı kanıtlanmış hata senaryolarıyla yönlendirin.

**Kabul kriteri:** Açılış, giriş, yenileme, çıkış, yeniden deneme ve başka sekmedeki değişimler belgelenmiş geçişlere uymalı. F01–F03 testleri geçmeli; React ve istemci durumu ayrışmamalı.

## F18 — Dokümanlar mevcut uygulamayla eşitlenmeli

**Öncelik:** Düşük · Dokümantasyon

**Kaynak:** [docs/DEPLOYMENT.md](DEPLOYMENT.md) ve oturumla ilgili kod yorumları.

DEPLOYMENT.md'nin bilinen sınırlar bölümünde refresh token'ın localStorage'da olduğu yazıyor; mevcut kod httpOnly çereze geçmiş. Bazı açıklamalar eski geliştirme fazlarının davranışını anlatıyor. Yanlış açıklamalar sonraki geliştirmeleri ve işletim kararlarını yönlendirebilir.

**Öneri:** Mevcut davranış, geçmiş karar ve gelecekte yapılacakları ayırın. Oturum, proxy, üretim rolü, test kapsamı ve hata politikası açıklamalarını kodla eşitleyin.

**Kabul kriteri:** Dağıtım rehberi ve kod yorumları güncel oturum saklama/iptal davranışıyla çelişmemeli; tamamlanmış geçişler yapılacak olarak görünmemeli.

**Durum:** Tamamlandı (20 Eylül 2026)

**Çözüm özeti:** İki adımda kapandı. (1) `1d2e92d` ile DEPLOYMENT.md'nin "refresh token localStorage'da" ifadesi ve env tablosu httpOnly çerez geçişine göre güncellendi (`COOKIE_SECURE`, `COOKIE_NAME`, `JWT_SESSION_MAX_AGE` satırları). (2) Bu commit ile kalan eşitleme: yeni faz belgesi [phases/phase-13-session-hardening.md](phases/phase-13-session-hardening.md) (tehdit → cevap, token aileleri, kilit sırası, çerez, CSRF, parola değişimi, Playwright, bilinen sınırlar); [STATUS.md](STATUS.md)'e Faz 13 satırı/özeti + canlı backlog işaretçileri; [PROJECT_MAP.md](PROJECT_MAP.md)'e migration `0008`, yeni auth sorguları/repository metotları, çerez yardımcıları + `handleChangePassword`, Account sayfası + yeniden yazılan istemci durum makinesi, `frontend/e2e/`, `scripts/e2e.sh`, `.github/dependabot.yml`, CI `images` işi; DEPLOYMENT.md'de env tablosunun `internal/config` ile eşitlenmesi ve "prod'da HTTPS ŞART" maddesi; üç öğrenme notu dosyasına Faz 13 girdileri; [learning-notes/infra-and-devops.md](learning-notes/infra-and-devops.md) §5'te "geçiş HENÜZ YAPILMADI / eksik olan asıl kontrol" bulgularının `c79707b`'nin gönderdiği hâle çekilmesi (kriterin "tamamlanmış geçişler yapılacak olarak görünmemeli" yarısı); [WHY_GO_VS_TS.md](WHY_GO_VS_TS.md)'de faz sonu ritüeli (§7 `handleLogout` + `context.WithoutCancel`, §10 `pg_blocking_pids`/`40P01` kilit testleri, §15'e 25. dürüst gözlem — Web Locks + `storage`); `mvp_smoke.sh`'ın 21 → 25 kontrole çıkmasının tarihî kayıtlarda (Faz 7/8/11/12, STATUS, infra notu) işaretlenmesi. Kapanan borçlar PROJECT_MAP §7'de **silinmedi**, üstü çizilip commit'iyle işaretlendi (geçmiş kayıt olarak kalsın). Faz 9 belgesindeki "refresh `localStorage`'da" satırı tarihî karar olarak duruyor ama Faz 13'e yönlendiren bir uyarı aldı.

**Commit:** `1d2e92d` (DEPLOYMENT.md) + (bu commit) docs(phase-13): faz belgesi, STATUS/PROJECT_MAP/DEPLOYMENT eşitlemesi, öğrenme notları, WHY_GO_VS_TS faz ritüeli (bu son dosya tasarımın E bölümünde listelenmiyordu; faz belgesi onu doldurulmuş saydığı için aynı commit'e alındı — commit mesajında da böyle yazıyor)

**Doğrulama ve sonuç:** Dokunulan dosyalardaki her göreli markdown linki bir betikle çözümlendi (hedef dosya + başlık anchor'ları dahil, kırık link yok). `grep` ile bayat iddiaların kalmadığı gösterildi: refresh token'ın `localStorage`'da olduğunu söyleyen tek satır kalmadı (öğrenme notlarındaki tarihî anlatım ve Faz 9'un uyarılı satırı hariç), cevap gövdesinde `refresh_token` alanı iddia eden doküman yok; **güncel** sayıyı iddia eden her yerde smoke 25/25 (README, PROJECT_MAP, STATUS, Faz 13) ve faz DoD'larındaki 21/21 kayıtları (Faz 7, 8, 11, 12, STATUS ve infra-and-devops notu) o günkü tarihî sonuçlar olarak bırakıldı — ama artık hepsi "(o günkü sayı; Faz 13'ten beri 25/25)" notunu taşıyor, işaretsiz tek bir 21/21 kalmadı. `go test ./...` ve `npm test` yeşil — bu commit yalnız `docs/` ve `README.md`'ye dokunuyor.

**Kalan risk:** Dokümanlar koddan ayrı yaşamaya devam ediyor; bunu otomatik doğrulayan bir kapı yok. Faz sonu ritüeli (STATUS + PROJECT_MAP + faz belgesi) tek koruma.

## F19 — `manage migrate` hangi migration'ları uyguladığını yazmıyor

**Öncelik:** Düşük · Altyapı / gözlemlenebilirlik

**Kaynaklar:** [backend/internal/database/migrate.go](../backend/internal/database/migrate.go) (`provider.Up`), [backend/cmd/manage/main.go](../backend/cmd/manage/main.go) (`migrate`), [.github/workflows/deploy.yml](../.github/workflows/deploy.yml) (Migrate production database adımı). Faz 15 yayınında (2026-10-25) bulundu.

`database.Migrate`, goose'un `provider.Up`'ının döndürdüğü uygulanan migration listesini atıyor; `manage migrate` de sıfır ya da N migration uygulasa da yalnız "migration'lar güncel" yazıyor. Faz 15'in Deploy'unda 0010'un uygulandığı ancak dolaylı görülebildi: Plan adımının "Yeni migration'lar" listesi ve `MIGRATE=true` bayrağı. İkisi de **niyeti** gösteriyor, sonucu değil; bir migration uygulanmadan geçilseydi log aynı görünürdü.

**Öneri:** `Up`'ın sonuçlarını yazdır: uygulanan her migration için sürüm, dosya adı ve süre; hiçbiri yoksa "uygulanacak migration yok". İsteğe bağlı: Deploy adımı Plan'ın beklediği listeyle uygulananı karşılaştırıp uyuşmazlıkta durur.

**Kabul kriteri:** Yeni migration'lı bir Deploy logunda uygulanan dosyanın adı ve sürümü görünür; migration'sız koşuda "uygulanacak migration yok" yazar. Başarısızlıkta "güncel" yazılmaz (mevcut `TestMigrateCommandRealDB` sözleşmesi korunur, beklenen çıktı güncellenir).

## İkinci inceleme — 17 Kasım 2026

**Kapsam:** Faz 1–16'nın tamamı (API, veritabanı şeması ve sorgular, arayüz, NGINX, CI/CD ve yayın). `docs/phase-16-close` dalında (`abaa3a4`) yapıldı; inceleme sırasında uygulama kodu değişmedi.

### Yöntem ve doğrulama sınırları

- **Üç bağımsız okuma:** API, arayüz ve altyapı ayrı ayrı, ilgili dosyaların tamamı okunarak incelendi. Bu belgeye giren her bulgu ayrıca kodda, ölçümle ya da yerel yığında doğrulandı. Doğrulanamayan kısım bulgunun içinde yazılı.
- **Veritabanı ölçümleri:** yerel Postgres 16'da üretim şemasının (0001–0013) kopyası ve sentetik veri: 500.033 kitap (460.041'i yayında), 770 bin baskı, 26 tür ve 932 bin tür bağı, 100 bin kullanıcı, 3,1 milyon puan, 2 milyon kitaplık kaydı, 1,05 milyon yorum. Sorgular uygulamanın gönderdiği metinle (sqlc çıktısından alınarak) hazırlanmış deyim olarak, paralel işçi olmadan 8'er kez çalıştırıldı; ilk 3 ölçüm atıldı, kalanların ortancası alındı.
- **Yerel yığın deneyleri:** `make app-up` kurulumu (NGINX → API → Postgres), API bu büyük veritabanına bağlı.
- **Canlı:** yalnız okuma istekleri (yaklaşık 45 GET). Canlıda giriş, yazma ya da yük denemesi yapılmadı.
- **Ölçeğe dair uyarı:** canlıda 54 kitap var ve aşağıdaki sorguların hepsi orada 1 ms'nin altında. "Ölçekle" işaretli bulgular bugünün yavaşlığını değil, katalog büyüdüğünde ilk neyin yavaşlayacağını gösterir.
- **Doğrulanmayanlar:** Northflank ayarları (sağlık kontrolü, derleme önbelleği, düğümdeki çekirdek sayısı, yayın sırasında eski ve yeni örneğin birlikte çalışması), GitHub dal koruması, Supabase'in sürümü, yedekleri ve havuz ayarı, Upstash'in kota aşımındaki davranışı.

### Genel değerlendirme

**Kötü yapılmış bir mimari karar var mı?** Temel kararlar sağlam ve çoğu yazılı gerekçesiyle duruyor. Geri dönülmesi gereken tek karar yerleşim: uygulama Londra'da, veri Frankfurt'ta (F20). Bugün her sayfanın gecikmesini bu belirliyor ve kodla değil taşımayla düzelir. İkinci sırada arayüzün sunucu verisini her sayfada elle yönetmesi var (F33): yanlış değil, ama her yeni ekranda aynı hata sınıfını yeniden üretiyor.

**Performansta ne daha iyi yapılabilirdi?** Bugün: istek başına sorgu sayısı (F20) ve yarıda kesilen isteğin bağlantı kapatması (F21). Katalog büyüdüğünde: tür süzgeci (F24), her istekteki tam sayım (F25) ve puana göre sıralama (F26). Üçü de ölçüldü ve üçünün de ölçülmüş bir çıkışı var.

**Şema ve index kararları yeterince düşünülmüş mü?** Büyük ölçüde evet: kısıtlar veritabanında, türetilmiş veri kaynaktan hesaplanıyor, kilit sırası belgeli, 0005 index'i ölçümle düzeltilmiş. Eksik kalanlar: üç yabancı anahtarın index'i (F28), kitap kilidinin gücü (F27), isteğe bağlı süzgeç kalıbının alt sorgulardaki bedeli (F24) ve kullanılmayan benzersiz slug'ın yol açtığı öneri hatası (F29).

**Güvenlik ve sağlamlık:** yeni bir açık bulunmadı. Düzeltilmesi ucuz dört hata var: geçersiz baytın 500 üretmesi (F30), `create-admin`'in davranışı (F31), bulunamayan `/assets/` dosyasının önbelleklenmesi (F35) ve eşzamanlı girişlerin API'yi kilitlemesi (F04'e eklenen kanıt).

## F20 — Uygulama ile veri ayrı bölgede; istekler sıralı ve çok sorgulu

**Öncelik:** Yüksek · Mimari ve performans

**Kaynaklar:** [DEPLOYMENT.md](DEPLOYMENT.md) §1, [backend/cmd/api/books.go](../backend/cmd/api/books.go) (`handleGetBook`, `handleListBooks`), [backend/internal/books/repository.go](../backend/internal/books/repository.go) (`List`), [backend/cmd/api/community.go](../backend/cmd/api/community.go) (`handleListComments`), [backend/internal/auth/repository.go](../backend/internal/auth/repository.go) (`RotateRefreshToken`), [backend/internal/database/database.go](../backend/internal/database/database.go).

API Londra'da (Northflank'ın ücretsiz plandaki tek bölgesi), Postgres ve Redis Frankfurt'ta. Her sorgu yaklaşık 13 ms ağ bekliyor ve sorgular sırayla gidiyor. Canlı ölçüm (17 Kasım, aynı bağlantı üzerinden): `/health` 74 ms, tek sorgulu `/api/genres` 86 ms, üç sorgulu `/api/books` 112–125 ms. Katalog isteğinin sunucudaki ~40 ms'sinin neredeyse tamamı üç gidiş-dönüş; sorguların kendisi 54 kitapta 1 ms'nin altında.

Uç başına veritabanı gidiş-dönüşü (kod okunarak sayıldı):

| Uç | Gidiş-dönüş | Not |
|---|---|---|
| Katalog listesi | 3 (puan sırasında 4) | liste, sayım, türler ayrı ayrı |
| Kitap detayı | 3 | kitap, türler, baskılar |
| Yorum listesi | 3 | kitap yalnız görünürlük için yeniden okunuyor |
| Puan özeti | 2 | kitap yeniden okunuyor |
| Puan verme | 5 | BEGIN ve COMMIT dahil |
| Oturum yenileme | 7 | giriş yapmış kullanıcının her sayfa açılışında |

Anonim bir kitap sayfası dört API çağrısıyla 11 sorgu çalıştırıyor ve aynı kitap satırını üç kez okuyor. İki ek maliyet: havuz, 1 saniyeden uzun boşta kalmış bağlantıyı kullanmadan önce yokluyor (pgx varsayılanı), yani seyrek trafikte her isteğe bir gidiş-dönüş daha ekleniyor; Redis de Frankfurt'ta olduğu için "en sevilenler" önbelleğinden okumak, yerine geçtiği sorgu kadar ağ bekliyor (F26).

**Etki:** Her sayfa gereğinden 30–150 ms geç geliyor. Yazarken arama gibi sık istek atan özelliklerde fark doğrudan hissedilir.

**Öneri:** Önce veriyi uygulamanın yanına taşıyın: Supabase ve Upstash belgelerinde London (eu-west-2) bölgesi var. 54 kitapla taşıma küçük bir iş: yeni proje, döküm ve geri yükleme, iki sırrın güncellenmesi. Sonra okuma uçlarını tek sorguya indirin; repoda örnekleri var ([library.sql](../backend/sql/queries/library.sql)'deki `count(*) OVER ()`, [comments.sql](../backend/sql/queries/comments.sql)'deki CTE). Sıra önemli: gidiş-dönüş 1–2 ms'ye inerse ikinci adımın getirisi küçülür; hangisinin gerektiğini F32'deki ölçüm gösterir.

**Doğrulanmadı:** London bölgesinin ücretsiz planda seçilebildiği ve taşımadan sonraki gidiş-dönüş süresi.

**Kabul kriteri:** Canlıda `/api/genres` ile `/health` arasındaki fark 5 ms'nin altına iner ya da katalog listesi tek sorguyla cevaplanır. Ölçüm PR'da yazılı.

**Karar (18 Kasım 2026, kullanıcı):** taşıma **ertelendi**. Postgres ve Redis Frankfurt'ta kalır; gerçek kullanıcılar gelip sayıları artınca taşıma (ve gerekirse ücretli plana geçiş) yeniden ele alınır. Belgelerden doğrulandı: Supabase'te bölge değiştirmek yeni proje + döküm/geri yükleme ister (F23'teki yedek betiği bunu da yapar); Londra bölgesi var. Upstash'te ana bölgenin değiştirilip değiştirilemediği ve ücretsiz planda Londra doğrulanmadı.

## F21 — Yarıda kesilen istek veritabanı bağlantısını kapatıyor ve hata olarak loglanıyor

**Öncelik:** Orta · Kaynak yönetimi

**Kaynaklar:** [GO_CONVENTIONS.md](GO_CONVENTIONS.md) §1, [backend/internal/database/database.go](../backend/internal/database/database.go), [backend/cmd/api/respond.go](../backend/cmd/api/respond.go) (`writeInternalError`).

Her sorguya isteğin context'i veriliyor; bu doğru alışkanlık. pgx'in varsayılan davranışında ise context iptal olunca uçuştaki sorgunun bağlantısı kapatılıyor. Yerel yığında NGINX üzerinden ölçüldü: 20 olağan istek tek bağlantıyı yeniden kullandı; yarıda kesilen 20 istek 19 yeni bağlantı açtırdı. Her biri ayrıca ERROR düzeyinde "istek başarısız … context canceled" satırı bıraktı ve handler gitmiş istemciye 500 yazmaya çalıştı.

**Etki:** Sayfadan ayrılma, sekme kapatma ve arayüzün isteği iptal etmesi sunucuda bağlantının yeniden kurulmasına dönüşüyor (canlıda TLS ve kimlik doğrulamayla birkaç gidiş-dönüş). Hata logu ve ileride kurulacak alarm, istemci davranışıyla kirleniyor.

**Öneri:** İki yol var. Kısa okuma sorgularını istemci iptalinden ayırın (`context.WithoutCancel` ve kısa bir zaman aşımı); katalog listesi için bu, arama işiyle birlikte geliyor. Ya da havuzu pgx'in `CancelRequestContextWatcherHandler`'ıyla kurun: ölçümde 20 iptal yeni bağlantı açtırmadı, iptal edilen çağrı yaklaşık 100 ms geç döndü; Supabase'in havuzlayıcısı üzerinden denenmedi. İstemcinin gittiği durumu 500 ve ERROR yerine ayrı bir düzeyde loglayın.

**Kabul kriteri:** Yarıda kesilen 20 istekten sonra havuzun açtığı bağlantı sayısı artmaz ve logda ERROR satırı oluşmaz.

**İlerleme (17 Kasım 2026, [Faz 16b](phases/phase-16b-search.md), PR #89, canlıda 2026-11-18):** katalog listesi ve araması (`GET /api/books`) sorgularını istemci iptalinden ayırıyor (`detached`, 5 sn sınır) ve giden istemciyi 499 olarak logluyor. Yerel yığında 500 bin kitapla yarıda kesilen 40 istek: 4 yeni bağlantı (eşzamanlılık için havuz büyümesi), 0 ERROR. Diğer uçlar ve havuz düzeyindeki çözüm açık.

**Durum (17 Kasım 2026, #84, PR #91, canlıda 2026-11-18; canlı ayar doğrulaması #84'te bekliyor):** Bütün okuma istekleri (GET/HEAD) istemci iptalinden ayrıldı ve 5 sn'lik bütçe aldı (`requestBudget` ara katmanı); katalogdaki özel kod kaldırıldı. Giden istemcinin isteği erişim logunda 499 ve INFO; iptalin bir yazma sorgusuna ulaştığı durum `writeInternalError`'da ERROR yerine INFO ve 499. Yazma istekleri bilerek ayrılmadı (istemcinin vazgeçtiği yazma tamamlanmaz). Yerel ölçüm: yarıda kesilen 40 istekten API'ye ulaşanlar 499/INFO olarak loglandı, yeni veritabanı bağlantısı 0, ERROR 0. Ölçümün sınırı: sorgular 1-4 ms sürdüğü için isteklerin çoğu NGINX'te, API'ye ulaşmadan kesildi; davranış birim testleriyle sabit (`budget_test.go`). Havuz düzeyindeki çözüm (`CancelRequestContextWatcherHandler`) yapılmadı: Supabase havuzlayıcısı üzerinden denenmedi.

## F22 — Yayın akışı canlıda hangi sürümün çalıştığını doğrulamıyor

**Öncelik:** Orta · Yayın

**Kaynaklar:** [.github/workflows/deploy.yml](../.github/workflows/deploy.yml) (`Release`, `Summary`), [backend/cmd/api/router.go](../backend/cmd/api/router.go) (`handleHealth`), [PROCESS.md](PROCESS.md) §6.

Deploy iş akışı `release` dalını ilerletip "Yayına çıktı" yazıyor; Northflank'ın imajı derleyip derlemediğine bakılmıyor. `/health` sürüm bilgisi taşımıyor. "Derlenen commit `release`'teki commit" kontrolü PROCESS §6'da elle yapılan bir madde.

**Etki:** Migration uygulanıp Northflank derlemesi düşerse eski kod yeni şemayla çalışmaya devam eder ve Deploy yeşil görünür. Bir sonraki migration'ın "o an canlıda çalışan kodla uyumu", çalışmayan bir sürüme göre değerlendirilir.

**Öneri:** Commit'i derlemede iki imaja da gömün ve `/health` cevabında döndürün. Deploy'a sır kullanmayan üçüncü bir iş ekleyin: iki servis de bu commit'i bildirene kadar bekler, süre dolarsa kırmızı biter. Northflank'ın commit'i derleme argümanı olarak verip vermediği belgelerinden doğrulanmalı.

**Kabul kriteri:** Derlemesi bilerek bozulmuş bir yayında Deploy kırmızı biter.

**Ek gözlem (18 Kasım 2026, yerel yığın):** NGINX, `proxy_pass http://api:8080;` adresini yalnız AÇILIŞTA çözüyor. API konteyneri yeniden oluşturulup yeni bir adres alınca NGINX eski adrese gitmeye devam etti ve bütün `/api/` istekleri 502 döndü; `web` yeniden başlatılınca düzeldi (compose'daki `depends_on … restart: true` bunu `up --build` akışında kendiliğinden yapıyor, tek servisi yeniden oluşturan komutlarda yapmıyor). Canlıda aynı şeyin olup olmadığı **doğrulanmadı**: Northflank'ta `api` adı büyük olasılıkla değişmeyen bir servis adresine çözülür, ama yalnız `api`'nin yeniden yayınlandığı bir durumda `web`'in 502 verip vermediğine bakılmalı. Yayın doğrulamasının (bu bulgu) kapsamına alınmalı.

## F23 — Yedek ve geri yükleme tanımlı değil

**Öncelik:** Orta · Veri

**Kaynaklar:** [DEPLOYMENT.md](DEPLOYMENT.md), [.github/workflows/deploy.yml](../.github/workflows/deploy.yml) (migration adımı).

Belgelerde yedek, döküm ya da geri yükleme geçmiyor. Kolon silen migration'lar (0013 gibi) canlıya kendiliğinden uygulanıyor.

**Öneri:** Kararı yazın. Ya "veri atılabilir; geri yükleme migration, demo seed ve `create-admin`'dir" denir ya da migration adımından önce şifreli bir döküm alınır (repo public olduğu için düz artefakt olarak yüklenmez). Seçilen yol bir kez denenir. Supabase'in ücretsiz planda ne sakladığı doğrulanmadı.

**Kabul kriteri:** Karar DEPLOYMENT.md'de; seçilen yol bir kez uygulandı ve süresi yazıldı.

**Karar ve ilerleme (18 Kasım 2026, kullanıcı kararı: B2):** otomatik, şifreli döküm. Haftalık (`backup.yml`, `backup` ortamı, yalnız `main`) ve her migration'dan önce (`deploy.yml`, aynı adım). `pg_dump` + `age`, açık anahtar repoda, gizli anahtar yalnız kullanıcıda; artefakt 90 gün. CI her PR'da döküm → şifre → çöz → geri yükle turunu sınıyor (yerelde 13 tablo, satır sayıları aynı). Sırların kapsamı `check_secret_scope` ile denetleniyor. Anahtar çifti ve sır hazır (2026-11-23, DEPLOYMENT §2b). Bekleyen: ilk gerçek döküm ve geri yükleme provası.

## F24 — Tür süzgeci kalıbı planlayıcının iyi planı kurmasını engelliyor

**Öncelik:** Orta — ölçekle · Sorgu

**Kaynaklar:** [backend/sql/queries/books.sql](../backend/sql/queries/books.sql) (`ListBooks`, `ListBooksByRating`, `CountBooks`), [backend/sql/queries/library.sql](../backend/sql/queries/library.sql) (`ListLibrary`).

İsteğe bağlı süzgeç `($1 IS NULL OR EXISTS (…))` biçiminde. Postgres, OR'un altındaki EXISTS'i birleştirmeye çeviremiyor; alt sorgu her istekte o türün bütün kitaplarını okuyup bir özet tablosu kuruyor ("hashed SubPlan"). Sorgu, türün seyrek ya da yaygın olmasına göre farklı bir yol da seçemiyor.

| Tür | Liste (bugün) | Sayım (bugün) | Liste (tür kimliğiyle) |
|---|---|---|---|
| Yaygın (232 bin kitap) | 120 ms | 278 ms | 1,1 ms |
| Seyrek (17 bin) | 11 ms | 132 ms | 1,0 ms |
| Nadir (501) | 3,9 ms | 130 ms | 2,9 ms |
| Çok nadir (21) | 391 ms | 113 ms | 0,7 ms |

Filtresiz liste 0,75 ms. Denenen ve yetmeyen bir yol: EXISTS'i üst düzeye alıp türü yine slug'la birleştirmek çok nadir türde 1.289 ms'ye çıktı, çünkü planlayıcı slug'ın hangi türe denk geldiğini plan anında bilmiyor.

**Öneri:** Tür süzgeci için ayrı bir sorgu yazın; slug önce tür kimliğine çevrilsin (26 satırlık tablo bellekte tutulabilir) ve sorguya kimlik verilsin. Sayım için F25'e bakın: tür kimliğiyle sayım, hazırlanmış deyimin 6. çalıştırmasından sonra genel plana geçip 21 kitaplık tür için 96 ms harcadı.

**Kabul kriteri:** 500 bin kitaplık veride her tür için liste 5 ms'nin altında; `EXPLAIN ANALYZE` çıktısı PR'da.

**Durum (19 Kasım 2026, #98, PR #102, canlıda 2026-11-19):** tür süzgeci ayrı sorgu ve kimlikle (`ListBooksInGenre`); slug → kimlik süreç içinde 60 sn. 500 bin kitapta liste: yaygın 120 → 0,14 ms, çok nadir 391 → 0,12 ms. **Karşılanmayan:** 501 kitaplık nadir türde 36 ms (paralel planın yanlış seçimi; paralel taramayı kapatmak büyük taramaları üçe katladığı için yapılmadı; ayrıntı Faz 16c spec'i).

## F25 — Her listede tam sayım, derin sayfa maliyeti ve 10.000 sınırı

**Öncelik:** Orta — ölçekle · Sorgu ve API

**Kaynaklar:** [backend/internal/books/repository.go](../backend/internal/books/repository.go) (`List`), [backend/sql/queries/books.sql](../backend/sql/queries/books.sql) (`CountBooks`), [backend/cmd/api/books.go](../backend/cmd/api/books.go) (`maxPageOffset`), [frontend/src/components/filters.tsx](../frontend/src/components/filters.tsx) (`asOffset`), [frontend/src/components/usePaged.tsx](../frontend/src/components/usePaged.tsx) (`Pager`), [0011_editions.sql](../backend/migrations/0011_editions.sql) (`book_view`).

- **Sayım:** her liste isteği ayrıca tam sayım çalıştırıyor: filtresiz 50 ms, tür süzgeciyle 113–278 ms. Sayım, liste sorgusunun kendisinden pahalı.
- **Derin sayfa:** `book_view`, LIMIT uygulanmadan önce her satır için varsayılan baskıya birleşiyor. Offset 10.000'de 58 ms; önce sayfanın kimliklerini seçip sonra görünüme birleşmek 1,2 ms; anahtarla sayfalama 0,47 ms.
- **Erişilemeyen sayfalar:** API toplamı olduğu gibi bildiriyor ama offset 10.000'de kesiliyor. Katalog 10.012 kitabı geçince "Sonraki" düğmesi açık kalır; tıklanınca arayüz geçersiz offset'i 0 sayıp ilk sayfaya döner (kod okunarak; denenmedi).

**Öneri:** Toplamı her istekte hesaplamayın: tür başına sayıları kısa süre bellekte tutun ya da `has_more` döndürün. Liste sorgularında önce kimlik, sonra görünüm; varsayılan sıra için anahtarla sayfalama. Sınıra gelindiğinde arayüz bunu söylesin.

**Kabul kriteri:** 500 bin kitapta katalog isteğinin veritabanı süresi 5 ms'nin altında; 10.000 sınırındaki davranış bir testle sabit.

**Durum (19 Kasım 2026, #98, PR #102, canlıda 2026-11-19):** önce kimlik sonra görünüm (offset 10.000: 58 → 1,1 ms); toplam sayı tür başına 60 sn süreç içinde; arayüz 10.000 sınırında "Sonraki"yi kapatıp nedenini yazıyor (`usePaged.test.tsx`). Bağlantılar `plan_cache_mode = force_custom_plan` ile açılıyor: genel planda liste 42–165 ms'ydi. Anahtarla sayfalama yapılmadı.

## F26 — Puana göre sıralama ve "en sevilenler" önbelleği

**Öncelik:** Orta — ölçekle · Sorgu ve önbellek

**Kaynaklar:** [backend/sql/queries/books.sql](../backend/sql/queries/books.sql) (`ListBooksByRating`), [backend/sql/queries/ratings.sql](../backend/sql/queries/ratings.sql) (`GlobalAverageRating`, `ListTopRated`), [backend/internal/ratings/cached.go](../backend/internal/ratings/cached.go), [backend/cmd/api/community.go](../backend/cmd/api/community.go) (`handleTopRated`), [GO_CONVENTIONS.md](GO_CONVENTIONS.md) §7.

Ağırlıklı skor sorgu anında hesaplanıyor. Karar bilinçli ve "yavaşladığı ölçülürse" çıkışı yazılı (§7). Ölçüm o noktayı gösteriyor:

- Puana göre liste 1.125 ms; önündeki genel ortalama sorgusu 204 ms ve her istekte çalışıyor.
- "En sevilenler" önbellek ıskasında 887 + 204 ms.
- Önbellek her puan yazımında siliniyor. Puan gelen bir sitede hep soğuk kalır ve eşzamanlı ıskaların hepsi aynı sıralamayı yeniden çalıştırır.
- İsabette de türler Postgres'ten okunuyor. Önbellek 50 kitabı saklıyor, ana sayfa 5 istiyor.

Ölçülen seçenekler: önce kimlik sonra görünüm 267 ms; saklanan skor ve index'i 0,95 ms (en iyi 50 için 1,2 ms); genel ortalama için kapsayan kısmi index 74 ms.

**Öneri:** Skoru kolon olarak saklayın: kitabın puanı değişince o anki genel ortalamayla yeniden hesaplanır, genel ortalama belirli aralıkla tazelenir (yavaş değişir, sıra kaymaz). Genel ortalamayı ve "en sevilenler" cevabını süreç içinde 60 saniye tutun; puan yazımında silmeyin.

**Kabul kriteri:** 500 bin kitapta puana göre ilk sayfa 5 ms'nin altında; sürekli puan yazılırken "en sevilenler" isteklerinin çoğu sıralamayı yeniden çalıştırmaz.

**Karar (18 Kasım 2026):** saklanan skor **ertelendi** (kullanıcı kararı). Canlıda ölçülmüş bir sorun yok ve ilk önerilen tasarım (elle güncellenen kolon + günlük yenileme) genel ortalamaya bağımlılık yüzünden zayıftı. Gerekince sabit ortalama ve eşikle generated column kullanılacak. Geri dönüş koşulu: canlıda puana göre sıralamanın `db_ms` değeri düzenli olarak 50 ms'yi geçerse. Ayrıntı: [Faz 16c](phases/phase-16c-catalog-queries.md). Önbellek kısmı (genel ortalama ve "en sevilenler" 60 sn) Faz 16c'de.

## F27 — Kitap satırı kilidi gereğinden güçlü; puan istatistiği iki yerde hesaplanıyor

**Öncelik:** Düşük · Eşzamanlılık

**Kaynaklar:** [ratings.sql](../backend/sql/queries/ratings.sql) (`LockApprovedBook`), [books.sql](../backend/sql/queries/books.sql) (`LockBookForModeration`), [book_genres.sql](../backend/sql/queries/book_genres.sql) (`LockBook`), [0006](../backend/migrations/0006_rating_stats_on_delete.sql), [backend/internal/ratings/service.go](../backend/internal/ratings/service.go) (`Remove`).

- `FOR UPDATE`, yabancı anahtar denetiminin aldığı `FOR KEY SHARE` ile çakışır. Deney: kitap satırı `FOR UPDATE` ile tutulurken aynı kitabı kitaplığa ekleme 1 saniyelik kilit zaman aşımına takıldı; `FOR NO KEY UPDATE` ile 5 ms'de tamamlandı. Aynı ayrım [users.sql](../backend/sql/queries/users.sql)'de parola değişimi için zaten yapılmış.
- Puan silmede istatistik iki kez hesaplanıyor: önce silme tetikleyicisi (0006), sonra servis.

**Öneri:** Üç kilit sorgusunda `FOR NO KEY UPDATE` kullanın. İstatistiğin tek bir sahibi olsun (tetikleyici ya da servis).

**Kabul kriteri:** Puan transaction'ı açıkken aynı kitaba kitaplık kaydı ve yorum eklenebilir; puan silme istatistiği bir kez günceller.

**Durum (17 Kasım 2026, #83, PR #90, canlıda 2026-11-18):** Üç kilit sorgusu `FOR NO KEY UPDATE` oldu (`LockApprovedBook`, `LockBookForModeration`, `LockBook`). Puan silmede istatistiği yalnız tetikleyici hesaplıyor; servis sonucu okuyor (`GetBookRatingStats`). Test: kitap kilitliyken aynı kitaba kitaplık kaydı eklenebiliyor, ikinci yazım kilidi bekliyor (`TestBookLockDoesNotBlockReferencingInsertsRealDB`).

## F28 — Index'siz yabancı anahtarlar ve küçük index düzeltmeleri

**Öncelik:** Düşük · Şema

**Kaynaklar:** [0004](../backend/migrations/0004_comments_ratings.sql), [0010](../backend/migrations/0010_library_entries.sql), [0005](../backend/migrations/0005_listing_indexes.sql).

- **Index'siz yabancı anahtarlar:** `comments.deleted_by`, `comments.book_id` (yalnız kısmi index var) ve `library_entries.book_id`. 0001'deki "her FK'ye index" kuralı bu üçünde atlanmış. Ölçüm: kullanıcı silme 171 ms (149'u `deleted_by` taraması), kitap silme 262 ms. Uygulamada kullanıcı ya da kitap silen bir yol yok (testler ve temizlik betikleri siliyor); DATA-02'den önce gerekir.
- **`ratings (book_id)` yerine `(book_id, score)`:** 100 bin oylu kitapta dağılım 29 → 6,2 ms, istatistik yenileme 12,7 → 6,0 ms.
- **Yönetim listesi:** reddedilen ve arşivlenen kitapların listesi 247 ms (index yok). İki kısmi index yerine tek `(status, created_at DESC, id DESC)` bütün durumlara hizmet eder.
- **Kullanıcı silmede puan tetikleyicisi** satır başına çalışıyor ve kitapları rastgele sırada kilitliyor. DATA-02 için deyim düzeyinde, kitapları sırayla kilitleyen bir tetikleyici gerekir.

**Kabul kriteri:** Yabancı anahtar kolonu hiçbir index'in ilk kolonu olmayan kısıt kalmaz (katalog sorgusu bir testte).

**Durum (17 Kasım 2026, #85, PR #92, canlıda 2026-11-18):** Migration `0015`: `comments.deleted_by` (kısmi), `comments (book_id, created_at, id)` (tam; eski kısmi index'in yerine), `library_entries.book_id`, `ratings (book_id, score)` ve `books (status, created_at, id)` (iki kısmi index'in yerine). Son karar ölçüldü (500 bin kitap): tek index dört durumun listesine 0,1–0,5 ms'de hizmet ediyor, herkese açık liste yavaşlamıyor, boyut 24 MB (eski ikisi 30 MB). Kural artık testte: `TestEveryForeignKeyHasIndex` 0014'te üç kolonu yakalıyor, 0015'te temiz. Yapılmayan: kullanıcı silmede deyim düzeyinde puan tetikleyicisi (DATA-02 ile birlikte).

## F29 — Aynı başlıklı ikinci öneri reddediliyor; yazar düz metin

**Öncelik:** Orta · Veri modeli

**Kaynaklar:** [backend/cmd/api/books.go](../backend/cmd/api/books.go) (`normalize`), [0001](../backend/migrations/0001_books_genres.sql) (`slug UNIQUE`), [backend/cmd/api/moderation.go](../backend/cmd/api/moderation.go), [frontend/src/pages/Suggest.tsx](../frontend/src/pages/Suggest.tsx), [backend/internal/moderation/transitions.go](../backend/internal/moderation/transitions.go).

- **Slug:** başlıktan türetiliyor ve bütün durumlarda benzersiz. Öneri formu slug göndermiyor. Aynı başlıklı ikinci kitap (başka bir şairin "Seçme Şiirler"i) ya da reddedilmiş bir başlığın yeniden önerilmesi, kullanıcının hiç yazmadığı bir alan için 409 `slug_conflict` alıyor. Slug hiçbir aramada kullanılmıyor: rotalar kimlikle çalışıyor. `TestSuggestBookDuplicateSlug` bu davranışı sabitliyor (kod okunarak).
- **Yazar:** `books.author` düz metin. Yazar sayfası, birden çok yazar ve yazım farklarını birleştirme bu modelle yapılamaz. Bugün ihtiyaç yok.

**Öneri:** Öneride slug'ı sunucu üretsin ve çakışmada sayısal ek koysun. "Zaten var" uyarısı yalnız başlık ve yazar birlikte eşleşirse verilsin. Yazar varlığı, yazar sayfası backlog'a girdiğinde genişlet/daralt ile gelir.

**Kabul kriteri:** Aynı başlıklı iki öneri de kaydedilir; aynı başlık ve yazarla ikinci öneri anlaşılır bir mesaj alır.

**Durum (17 Kasım 2026, #83, PR #90, canlıda 2026-11-18):** Öneri yarısı yapıldı: öneride slug çakışırsa sayısal ekle yeniden deneniyor (`-2`, `-3`; en çok 20 deneme), admin'in doğrudan eklemesi çakışmayı bildirmeye devam ediyor (`TestSuggestSameTitleTwiceRealDB`). Yazarın düz metin olması açık; ayrı bir tasarım işi.

## F30 — Geçersiz bayt içeren metin 500 üretiyor

**Öncelik:** Orta · Girdi doğrulama

**Kaynaklar:** [backend/cmd/api/books.go](../backend/cmd/api/books.go) (`parseListFilter`), [backend/cmd/api/auth.go](../backend/cmd/api/auth.go), [backend/cmd/api/validate.go](../backend/cmd/api/validate.go).

Yerel yığında `/api/books?q=%FF`, `q=%00` ve `q=a%C3` 500 döndü; logda SQLSTATE 22021. Aynı yol giriş, kayıt, yorum ve öneri gövdelerinde de açık (kod okunarak). Kayıtta 500, parola özeti hesaplandıktan sonra gelir.

**Etki:** Herkes dilediği kadar 500 ve ERROR satırı üretebilir.

**Öneri:** Metin girdilerini tek yerde doğrulayın (`utf8.ValidString`, NUL yok) ve 400 dönün. Yedek olarak SQLSTATE 22 sınıfını 400'e çevirin. Bir fuzz testi ekleyin.

**Kabul kriteri:** Üç örnek istek 400 döner ve ERROR satırı oluşmaz.

**İlerleme (17 Kasım 2026, [Faz 16b](phases/phase-16b-search.md), PR #89, canlıda 2026-11-18):** `q` parametresi katalogda ve Kitaplığım'da denetleniyor (`validator.text`); üç örnek istek yerel yığında 400 döndü. Gövde alanları (giriş, kayıt, yorum, öneri) açık.

**Durum (17 Kasım 2026, #83, PR #90, canlıda 2026-11-18):** Gövdeler de kapandı. `readJSON` çözülen gövdenin bütün metinlerinde NUL arıyor ve 400 dönüyor (kayıtta parola özeti hesaplanmadan önce). Geçersiz UTF-8 gövdede sorun değil: `encoding/json` onu U+FFFD'ye çeviriyor (testle sabit). Yedek: SQLSTATE 22 sınıfı `writeInternalError`'da 400 ve WARN oluyor. Fuzz testi eklenmedi; tablo testi var (`nul_test.go`).

## F31 — `create-admin` var olan hesabı parolasına dokunmadan admin yapıyor

**Öncelik:** Orta · Güvenlik

**Kaynaklar:** [backend/internal/auth/repository.go](../backend/internal/auth/repository.go) (`EnsureAdmin`), [backend/cmd/manage/main.go](../backend/cmd/manage/main.go).

E-posta doğrulaması yok ve repo public. Yöneticinin kullanacağı adresle önceden kayıt olan biri, `create-admin` o adresle çalıştırıldığında kendi parolasıyla admin olur. Araç yöneticinin yazdığı parolayı atar ve tek satırlık bir bilgi yazar.

**Öneri:** Var olan hesapta varsayılan davranış hata olsun. `-promote` verilirse parola da yazılsın ve hesabın bütün oturum aileleri aynı transaction'da iptal edilsin.

**Kabul kriteri:** Var olan adresle `create-admin` hata verir; `-promote` ile eski parola ve eski oturumlar geçersiz olur.

**Durum (17 Kasım 2026, #83, PR #90, canlıda 2026-11-18):** `create-admin` var olan hesapta hata veriyor. `-promote` ile hesap devralınıyor: parola değişiyor, bütün oturum aileleri iptal ediliyor, rol yükseliyor (tek transaction). `scripts/e2e.sh` `-promote` kullanıyor; DEPLOYMENT güncellendi.

## F32 — Bir isteğin süresinin nereye gittiği görünmüyor

**Öncelik:** Orta · Gözlemlenebilirlik

**Kaynaklar:** [backend/cmd/api/middleware.go](../backend/cmd/api/middleware.go) (`requestLogger`), [backend/cmd/api/router.go](../backend/cmd/api/router.go) (`middleware.Recoverer`).

Erişim satırı toplam süreyi yazıyor. Veritabanı ve Redis süresi, sorgu sayısı ve kullanıcı kimliği yok. F20'deki sayılar kod okunarak çıkarıldı; canlıda yavaş bir isteğin ağdan mı, işlemci kısıtından mı, kilit beklemesinden mi kaynaklandığı ayırt edilemiyor. Panik yolunda chi'nin `Recoverer`'ı hata zarfı olmadan boş gövdeli 500 dönüyor (kod okunarak; denenmedi).

**Öneri:** İstek başına sorgu sayısını ve süresini toplayan bir `pgx.QueryTracer`; aynı ölçüm Redis için. Erişim satırına `db_queries`, `db_ms`, `redis_ms` ekleyin. Zarf ve `request_id` yazan küçük bir recoverer yazın. F20'den önce yapılırsa etkisi ölçülebilir.

**Kabul kriteri:** Her erişim satırında sorgu sayısı ve veritabanı süresi var; panik eden bir handler hata zarfı döner.

**Durum (17 Kasım 2026, #84, PR #91, canlıda 2026-11-18; canlı ayar doğrulaması #84'te bekliyor):** Erişim satırında `db_queries`, `db_ms`, `redis_ms` var (`internal/reqstats`; pgx `QueryTracer` ve go-redis hook'u). BEGIN ve COMMIT ayrı gidiş-dönüş sayılır. Sorgu metni ve argümanı kaydedilmez. Panik eden handler hata zarfı ve `X-Request-Id` ile 500 döner (`recoverer`). Havuzdan bağlantı bekleme süresi ve kullanıcı kimliği eklenmedi. Ölçümün ilk faydası aynı gün görüldü: Redis durdurulmuşken `redis_ms` 600 ms gösterdi ve aşağıdaki Redis notuna yol açtı.

**Ek (17 Kasım 2026, #84): Redis kesintisi.** Redis açılışta ulaşılamazsa API artık açılıyor (uyarı loglar; bağlantı sonradan kurulur). Ölçüm sırasında bir sorun bulundu: Redis durdurulmuşken önbellek kullanan tek istek 10 sn'yi aştı, çünkü her komut bağlantı kurmayı baştan deniyordu. Düzeltme: komut başına 600 ms sınır, tek yeniden deneme ve bir devre kesici (başarısız komuttan sonra 5 sn Redis denenmez). Yerel ölçüm: kesinti boyunca 5 sn'de bir tek istek 0,6 sn bekledi, diğerleri 2-4 ms; Redis dönünce önbellek kendiliğinden çalıştı. Kesinti sırasında rate limit açık kalır (uyarı loglanır); NGINX'in sınırı yerinde durur.

## F33 — Sunucu verisi her sayfada elle çekiliyor

**Öncelik:** Orta · Arayüz mimarisi

**Kaynaklar:** [Home.tsx](../frontend/src/pages/Home.tsx), [BookDetail.tsx](../frontend/src/pages/BookDetail.tsx), [Library.tsx](../frontend/src/pages/Library.tsx), [usePaged.tsx](../frontend/src/components/usePaged.tsx), [useLibraryStatuses.tsx](../frontend/src/components/useLibraryStatuses.tsx), [LibraryMenu.tsx](../frontend/src/components/LibraryMenu.tsx), [Moderation.tsx](../frontend/src/pages/admin/Moderation.tsx).

Test dışı kodda 25 `useEffect` var; 11'i veri çekiyor ve ortak bir kuralları yok. Eski cevap koruması kiminde bayrak, kiminde sayaç, kiminde hiç yok. Yeniden yükleme için sayaçlar ve ref'ler kullanılıyor. Önbellek yok: türler Home ve Library'de ayrı ayrı çekiliyor, geri dönüşte katalog yeniden yükleniyor. F10 ve Faz 15'teki yarış düzeltmeleri bu sınıfın örnekleri.

**Öneri:** Tek bir sorgu katmanı: TanStack Query ya da aynı kuralları uygulayan tek bir ortak kanca. Sayfa sayfa geçilebilir; [client.ts](../frontend/src/api/client.ts) altında değişmeden kalır. Bu bir mimari yargıdır; effect sayıları sayıldı, kazanç ölçülmedi.

**Kabul kriteri:** Veri çeken her sayfa aynı katmandan geçer; eski cevap, hata ve yeniden yükleme davranışı tek yerde tanımlı ve testli.

**Durum (22 Kasım 2026, [Faz 16d](phases/phase-16d-server-state.md), #105, PR #112, canlıda 2026-11-22):** Sunucu verisi TanStack Query'ye taşındı. Sayfalarda veri çeken `useEffect` kalmadı (`AuthContext` hariç); sorgu anahtarları ve varsayılanlar tek yerde (`api/queries.ts`, `lib/queryClient.ts`), her değişiklik neyi eskittiğini kendisi söylüyor, çıkışta önbellek boşalıyor. Eski cevap yarışı için testler `usePaged.test.tsx` ve `Home.test.tsx`'te. Ana paket +10,8 KB gzip; adım adım ölçüm spec'te.

## F34 — Hata sınırı yok; NGINX'in ürettiği hatalar boş mesajla görünüyor

**Öncelik:** Orta · Arayüz sağlamlığı

**Kaynaklar:** [frontend/src/main.tsx](../frontend/src/main.tsx), [frontend/src/App.tsx](../frontend/src/App.tsx), [frontend/src/api/client.ts](../frontend/src/api/client.ts) (`toError`), [frontend/src/components/common.tsx](../frontend/src/components/common.tsx) (`ErrorBox`).

- **Hata sınırı:** uygulamada error boundary yok. Render sırasında fırlayan bir hata (iki kez yüklenemeyen tembel parça, tipe uymayan bir cevap) bütün kökü söker ve boş sayfa kalır.
- **Boş mesaj:** `toError`, JSON zarfı olmayan cevapta `statusText`'i mesaj yapıyor. NGINX'in kendi 429, 502 ve 504 cevapları HTML'dir ve HTTP/2'de `statusText` her zaman boştur. Canlıdaki sonuç, içinde yalnız ikon olan kırmızı bir kutu. Yerelde (HTTP/1.1) "Bad Gateway" yazdığı için fark edilmiyor; birim testi de bu metni sabitliyor (kod okunarak; canlıda denenmedi).

**Öneri:** Router'ın içine bir hata sınırı koyun. `toError` duruma göre Türkçe bir mesaj üretsin; boş mesaj genel metne düşsün.

**Kabul kriteri:** Render hatasında başlık ve "yeniden yükle" seçeneği görünür; 429 ve 502'de kullanıcı anlamlı bir mesaj okur.

**İlerleme (17 Kasım 2026, [Faz 16b](phases/phase-16b-search.md), PR #89, canlıda 2026-11-18):** NGINX'in kendi 429'u artık API'nin hata zarfıyla ve `Retry-After` ile dönüyor (giriş, kayıt, parola ve katalog sınırlarında). 502 ve 504 ile hata sınırı açık.

**Durum (17 Kasım 2026, #86, PR #93, canlıda 2026-11-18):** Rota içeriği bir hata sınırıyla sarıldı (`ErrorBoundary`; rota yolu `key`, başka sayfaya geçince sıfırlanır). Zarfsız ya da boş mesajlı hata cevabında mesaj durum kodundan üretiliyor (`messageForStatus`: 429, 413, 502/503/504, diğer 5xx). Uçtan uca test: katalog 502 HTML alınca kullanıcı "Sunucuya şu an ulaşılamıyor" mesajını okuyor (`resilience.spec.ts`). HTTP/2'deki boş durum metni yerelde üretilemedi; testte taklit edildi.

## F35 — Bulunamayan `/assets/` dosyası bir yıl önbelleklenebilir

**Öncelik:** Orta · Önbellek

**Kaynak:** [frontend/nginx/default.conf](../frontend/nginx/default.conf) (`location /assets/`).

`add_header Cache-Control "public, max-age=31536000, immutable" always;` satırındaki `always`, başlığı 404'e de ekliyor. Yerel yığında doğrulandı: var olmayan bir `/assets/…js` isteği 404 ile birlikte bu başlığı döndü.

**Etki:** Yayın sırasında yeni `index.html`'in istediği dosya eski örneğe düşerse, 404 cevabı tarayıcıda bir yıl geçerli kalabilir ve sayfa o tarayıcıda sert yenilemeye kadar açılmaz. Yayın sırasında eski ve yeni örneğin birlikte hizmet verip vermediği doğrulanmadı.

**Öneri:** O satırdan `always`'i kaldırın (güvenlik başlıklarında kalsın). Üç önbellek sözleşmesi için bir uçtan uca test ekleyin: `index.html` önbelleklenmez, var olan dosya kalıcıdır, bulunamayan dosya önbelleklenmez.

**Kabul kriteri:** Bulunamayan `/assets/` dosyasının cevabında `Cache-Control: … immutable` yok.

**Durum (17 Kasım 2026, #83, PR #90, canlıda 2026-11-18):** `/assets/` konumundaki `always` kaldırıldı. Üç önbellek sözleşmesi uçtan uca testte (`frontend/e2e/cache-headers.spec.ts`): index.html `no-cache`, özetli dosya `immutable`, bulunamayan dosya önbelleklenmez.

## F36 — Açılıştaki geçici hata "çıkış yapılmış" gibi görünüyor

**Öncelik:** Orta · Oturum (F17 ile ilgili)

**Kaynaklar:** [frontend/src/api/client.ts](../frontend/src/api/client.ts) (`restore`, yenileme), [frontend/src/auth/AuthContext.tsx](../frontend/src/auth/AuthContext.tsx), [frontend/src/App.tsx](../frontend/src/App.tsx) (`RequireRole`).

- Yenileme geçici bir hatayla (ağ, 429, 502) dönerse istemci oturumu korur ama `restore()` boş döner. Arayüz "Giriş" gösterir, korumalı sayfalar `/login`'e yönlendirir ve yeniden deneyen bir şey yoktur.
- Başarılı yenilemeden sonra `/api/auth/me` bir kez daha çağrılıyor; cevap yenilemede zaten var. Bu çağrı geçici bir hata verirse arayüz geçerli oturumu kapalı gösterir. Her açılışa bir istek ekler.

**Öneri:** Üç durum tanımlayın (anonim, girişli, bilinmiyor) ve "bilinmiyor"da yeniden deneyin. `restore()`, yenilemenin döndürdüğü kullanıcıyı kullansın.

**Kabul kriteri:** Açılışta yenileme 502 dönerse kullanıcı giriş sayfasına atılmaz; ağ düzelince oturum kendiliğinden görünür.

**Durum (17 Kasım 2026, #86, PR #93, canlıda 2026-11-18):** Üçüncü durum eklendi: oturum belirsizken (`api.sessionUncertain()`: HINT var, yenileme geçici hatayla döndü) `ready` false kalıyor, korumalı sayfa "Oturumun doğrulanamadı… Yeniden deneniyor" gösteriyor ve yönlendirmiyor. Yeniden deneme 1, 3, 10, 30 sn sonra ve `online` olayında; sunucu 401 dönerse anonim olarak kesinleşiyor. `restore()` kullanıcıyı yenileme cevabından alıyor; açılıştaki `/api/auth/me` isteği kalktı. Uçtan uca test: yenileme 502 iken `/library`'de kalınıyor, sunucu 401 verince `/login`'e gidiliyor. Bilinen sınır: belirsizken menüde "Giriş" bağlantısı görünmeye devam ediyor.

## F37 — Kapak görselleri denetlenmeyen adreslerden yükleniyor

**Öncelik:** Orta · Güvenlik ve gizlilik

**Kaynaklar:** [backend/cmd/api/validate.go](../backend/cmd/api/validate.go) (`httpURL`), [backend/cmd/api/moderation.go](../backend/cmd/api/moderation.go) (`handleSuggestBook`), [frontend/nginx/security-headers.conf](../frontend/nginx/security-headers.conf) (`img-src`), [Moderation.tsx](../frontend/src/pages/admin/Moderation.tsx).

Öneri formunda kapak alanı yok ama API öneride `cover_url` kabul ediyor ve her http(s) adresi geçerli. Moderasyon listesi kapağı göstermiyor (F12). Onaylanan kitabın kapağı, her ziyaretçinin tarayıcısından o sunucuya istek attırır (IP ve tarayıcı bilgisi gider) ve sunucu görseli onaydan sonra değiştirebilir.

**Öneri:** Kapak sunucularını izin listesine alın (bugün yalnız `covers.openlibrary.org`) ve CSP'yi aynı listeye daraltın. Moderasyonda kapağı gösterin. İleride kapakları kendi origin'inizden sunun.

**Kabul kriteri:** İzin listesi dışındaki bir kapak adresi 400 alır; CSP yalnız listedeki sunuculara izin verir.

**Ek gözlem (18 Kasım 2026, canlıda tarayıcı testi):** kapaklar `covers.openlibrary.org`'dan istenip `archive.org`'a **yönlendirilerek** yükleniyor (en yavaş üç ağ isteği kapaklardı: 470–878 ms; açılıştan ~3 sn sonra şeritteki kapak kutuları hâlâ boştu). CSP'nin `img-src` kuralı yönlendirmenin HEDEFİNE de uygulanır: izin listesi yalnız `covers.openlibrary.org` olursa kapaklar kırılır; liste yönlendirme hedeflerini de içermeli (hangi alan adları olduğu ağ kayıtlarından çıkarılacak; doğrulanmadı). Gecikme, önerideki "kapakları kendi origin'inizden sunun" maddesinin gerekçesini güçlendiriyor.

**Karar ve ilerleme (19 Kasım 2026):** kullanıcı A'yı seçti: önerilerde kapak adresi kabul edilmez (#103). Öneri `cover_url` taşırsa 400 alır; boş ya da hiç gönderilmemişse bugünkü gibi 201. Admin uçları değişmedi; kapak adresini yalnız admin girer (bugün arayüzde bunun bir alanı yok, kitap ya da baskı PATCH'iyle API'den yapılır; canlıdaki kapaklar demo migration'larından gelir: 0009, 0012). İzin listesi, CSP daraltması ve kapakları kendi origin'imizden sunmak açık kalıyor; madde bu yüzden kapanmadı.

## F38 — CI taze kurulumu sınıyor, canlı yükseltme yapıyor; migration'da süre sınırı yok

**Öncelik:** Orta · CI/CD

**Kaynaklar:** [.github/workflows/ci.yml](../.github/workflows/ci.yml), [.github/workflows/deploy.yml](../.github/workflows/deploy.yml), [scripts/checks.sh](../scripts/checks.sh) (`check_migration_number`), [backend/internal/database/migrate.go](../backend/internal/database/migrate.go).

- **Taze kurulum ve yükseltme:** CI her koşuda boş bir veritabanı kuruyor; canlıda migration dolu tabloya ve eski kod çalışırken uygulanıyor. Dolu veride düşen bir migration ancak yayında görülür. 0011 ve 0013 için yazılmış yükseltme testleri iyi bir örnek ama kural değil.
- **Numara sırası:** migration numarası yalnız tekrar için denetleniyor. İki dalda 0015 önce yayınlanıp 0014 sonra birleştirilirse goose sırasız migration'ı reddeder ve yayın durur (goose kaynağı okunarak).
- **Süre sınırı:** canlı migration oturumunda `lock_timeout` ve `statement_timeout` yok; Deploy işlerinde `timeout-minutes` yok. Uzun süren bir kilidin arkasında bekleyen `ALTER TABLE`, o tabloya gelen bütün sorguları sıraya sokar.

**Öneri:** CI'a "`release` şeması ve demo veri üzerine HEAD migration'ları" işini ekleyin. Yeni migration numarası `main` ve `release`'teki en büyük numaradan büyük olsun (`checks.sh`). Migration oturumuna `lock_timeout`, işlere süre sınırı koyun.

**Kabul kriteri:** Dolu veride düşen bir migration PR'da kırmızı olur; sıra dışı numara Kurallar'da yakalanır.

**Durum (17 Kasım 2026, #88, PR #95, canlıda 2026-11-18):** CI'a "Yükseltme" işi eklendi (`scripts/upgrade-check.sh`): `release`'in şeması ve demo verisi üzerine bu commit'in migration'ları uygulanır; şema sürümü ve kitap sayısı denetlenir. Yerelde çalıştırıldı: şema 13 → 15, 54 kitap yerinde (0014 ve 0015 dolu veride geçiyor). `checks.sh`: yeni migration numarası `main` ve `release`'teki en büyük numaradan büyük olmalı (`check_migration_order`). Migration oturumunda `lock_timeout` 5 sn (`migrate.go`); Deploy işlerinde `timeout-minutes` (15 ve 5). Doğrulanmadı: iş akışları GitHub'da henüz koşmadı; `SET lock_timeout` Supabase havuzlayıcısında denenmedi.

## F39 — Sınanan imaj çalışan imaj değil; geri dönüş denenmedi; CI zorunlu kontrol değil

**Öncelik:** Düşük · CI/CD

**Kaynaklar:** [.github/workflows/ci.yml](../.github/workflows/ci.yml), [.github/workflows/scan.yml](../.github/workflows/scan.yml), [PROCESS.md](PROCESS.md) §1 ve §5, [backend/internal/testdb/testdb.go](../backend/internal/testdb/testdb.go).

- CI imajları iki kez, Northflank üçüncü kez derliyor. Trivy'nin taradığı imaj çalışan imaj değil ve geri alınacak hazır bir imaj yok.
- Geri dönüş yolu (`git revert`, PR, CI, Deploy) hiç denenmedi; ilgisiz bir güvenlik açığı bulgusu bu yolu kapatabilir.
- Zorunlu kontrol yalnız Kurallar: CI kırmızıyken PR birleştirilebilir (belgeye dayanıyor; GitHub ayarı görülmedi).
- `TEST_DATABASE_URL` yoksa entegrasyon testleri sessizce atlanıyor. CI'da değişken adı değişirse testler koşmadan yeşil kalır.
- `deploy.yml`'de action'lar etiketle sabit; `ci.yml`'de üçüncü taraf action'lar SHA ile.

**Öneri:** Geri dönüşü bir kez deneyip adımlarını yazın. CI'da `REQUIRE_INTEGRATION=1` ile atlamayı hataya çevirin. Her zaman çalışan tek bir "kapı" işini zorunlu kontrol yapın. `deploy.yml`'deki action'ları SHA ile sabitleyin.

**Kabul kriteri:** Geri dönüş adımları PROCESS.md'de ve bir kez uygulandı; entegrasyon testlerinin atlandığı bir CI koşusu kırmızı biter.

**İlerleme (17 Kasım 2026, #88, PR #95, canlıda 2026-11-18):** CI'da `REQUIRE_INTEGRATION=1`: entegrasyon ortamı eksikse `internal/testdb` testi kırmızı biter. `deploy.yml`'deki action'lar commit ile sabitlendi. Açık kalanlar: geri dönüş provası (canlıda, adım listesiyle), zorunlu kontrol ayarı (GitHub paneli), sınanan imajın yayınlanan imaj olması.

## F40 — Küçük bulgular

**Öncelik:** Düşük

Kod okunarak; "(ölçüldü)" yazanlar dışında ayrıca denenmedi.

- **Liste cevabı detay boyutunda:** her kitap açıklamasını (5.000 karaktere kadar) taşıyor; kartlar kullanmıyor. Liste için dar bir cevap biçimi yeter.
- **NGINX:** API'ye her istekte yeni bağlantı açıyor (upstream `keepalive` yok). Çekirdek sayısı kadar işçi başlatıyor (yerelde 16, ölçüldü); 0,1 vCPU'da tek işçi yeter.
- **Redis istemcisi:** `MaxRetries` varsayılanı 3, yani yavaşlamış bir Redis çağrı başına 2 saniyeye kadar bekletir; kod yorumu 500 ms varsayıyor. API açılışta Redis'e ulaşamazsa başlamıyor, çalışırken ulaşamazsa devam ediyor.
- **Arayüzde lint yok** (hooks ve erişilebilirlik kuralları); karşıtlık betiği CI'da değil.
- **Kaydırma ve odak:** sayfa ve rota değişiminde yönetilmiyor; "Sonraki"den sonra görünüm sayfanın altında kalıyor.
- **BookDetail:** "puanım" effect'i `user` nesnesine bağlı olduğu için her yenilemede yeniden istek atıyor; puan kartı özet gelene kadar "Henüz puan yok" gösteriyor (kitap cevabında puan var).
- **Yorum silme** onaysız. `/books/abc`, "bulunamadı" yerine 400 hata kutusu gösteriyor.
- **API tutarsızlıkları:** bazı uçlar düz dizi, bazıları sayfa zarfı dönüyor; token cevaplarında `Cache-Control: no-store` yok; doğrulama hataları tek metin olarak geliyor.
- **Tek giriş parçası:** 347 kB, canlıda brotli ile 112 kB (ölçüldü). Her yayında React ve router da yeniden iniyor; sabit bir vendor parçası bunu önler.
- **Yazı tipleri** (yaklaşık 204 kB) önceden yüklenmiyor.

**Bakıldı, sorun değil:** canlıda ağ geçidi statik dosyaları ve JSON'u brotli ile sıkıştırıyor ve HSTS başlığını ekliyor (ölçüldü). NGINX'te sıkıştırma olmaması yalnız compose ve Kubernetes kurulumunu etkiler.

### İkinci incelemenin önerdiği sıra

1. **Küçük ve etkisi hemen görülenler:** F35 (tek kelime), F30, F31, F29'un öneri yarısı ve F04'e eklenen kuyruk önerisi.
2. **Gecikme:** F32 (ölçüm) ile F20 (taşıma) birlikte; ardından F21 ve F36.
3. **Katalog büyümeden önce:** F24, F25, F26 ve F28.
4. **Yayın güveni:** F22, F23 ve F38.
5. **Arayüz:** F33 ve F34; F40'taki maddeler ilgili ekrana dokunulduğunda.

## Korunması gereken iyi tasarım kararları

| Desen / yaklaşım | Değerlendirme |
|---|---|
| Handler → Service → Repository | İş mantığı olan alanlarda iyi ayrım. Basit CRUD'a zorla servis eklenmemesi makul. |
| Elle dependency injection | Bağımlılıkların main.go'da kurulması ve tüketen tarafta interface tanımlanması uygun. |
| State machine | Moderasyon geçişlerinin merkezi ve kilitle birlikte uygulanması doğru. |
| Transaction sınırları | Moderasyon + audit ve puan + istatistik bütünlüğü iyi düşünülmüş. |
| Decorator / cache-aside | CachedService temel servisi anlaşılır şekilde sarıyor. |
| Functional options | Moderasyonun cache'i doğrudan tanımadan hook ile haber vermesi uygun. |
| Kimlik ve veri koruması | Sabit JWT algoritması, refresh hash saklama, httpOnly çerez, merkezi admin rota koruması ve parametreli SQL iyi temeller. |
| Görünürlük SQL'de | Yayında olmayan kitabın sızmaması sorgunun kendisinde; public ve admin sorguları ayrı. |
| Kişisel veri katalog cevabında yok | Katalog ve "en sevilenler" herkes için aynı; kitaplık durumu ayrı uçtan gelir. Önbelleğe ve arama işine zemin hazırlıyor. |
| Genişlet → yayınla → daralt | 0011 ve 0013 yükseltme testleriyle birlikte; kuralın makineyle denetlenen kısmı `checks.sh`'ta. |
| Kurallar test edilmiş betikte | Aynı betik kancalarda ve CI'da çalışıyor; yeni kural testiyle geliyor. |
| Yayın yetkilerinin ayrılması | Sırrı taşıyan iş salt okunur; dalı ilerleten iş repo kodu çalıştırmıyor. |

## Önerilen uygulama sırası

1. F01–F04 ve F10: oturum, kötüye kullanım ve veri doğruluğu. F17'yi bu düzeltmelerin gerektirdiği ölçüde ele alın.
2. F05–F09: üretim yetkileri, sağlık kontrolleri ve kaynak sınırları.
3. F11–F15: kullanıcı akışları, moderasyon, mobil/erişilebilirlik ve audit kapsamı.
4. F16, F18 ve F19: CI/CD, yayın gözlemlenebilirliği ve dokümantasyon. Yeni regresyon testlerini ilgili düzeltmeyle birlikte ekleyin; son aşamaya bırakmayın.

## Çözüm kaydı şablonu

Her bulgunun altına gerektiğinde şu bilgileri ekleyin:

```text
Durum: Açık / Çalışılıyor / Doğrulama bekliyor / Tamamlandı / Risk kabul edildi
Çözüm özeti:
Commit / PR:
Doğrulama ve sonuç:
Kalan risk veya erteleme gerekçesi:
```
