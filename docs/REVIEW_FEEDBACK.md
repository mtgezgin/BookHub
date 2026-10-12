# BookHub — İnceleme ve geri bildirim takibi

İnceleme tarihi: **19 Eylül 2026**  
Takip dosyasının oluşturulması: **20 Eylül 2026**

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

Kutuları yalnız düzeltme uygulanıp kabul kriteri doğrulandığında işaretleyin. İlgili bulgunun altına çözüm özeti, commit/PR ve doğrulama sonucu ekleyin. Bilinçli olarak ertelenen veya kabul edilen risklerde gerekçe ve yeniden değerlendirme koşulu yazın.

| Tamamlandı | ID | Öncelik | Bulgu |
|---|---|---|---|
| [x] | F01 | Yüksek | Başarısız logout sonrasında sunucu oturumu canlı kalabiliyor |
| [x] | F02 | Yüksek | Refresh commit'i sonrası hata yeniden denemede oturumu düşürüyor |
| [ ] | F03 | Orta | Çıkış, devam eden refresh ve kilidi bekliyor |
| [ ] | F04 | Yüksek | Rate limit kapsamı kötüye kullanımı yeterince sınırlamıyor |
| [ ] | F05 | Yüksek — üretim | API ile migration veritabanı yetkileri ayrılmamış — **risk kabul edildi** |
| [ ] | F06 | Orta | Admin yetkisinin kaldırılması mevcut token'ı hemen etkilemiyor |
| [ ] | F07 | Orta | Readiness gerçek hizmet verebilirliği ölçmüyor |
| [ ] | F08 | Orta | İşlem, sorgu ve kilit bekleme süre bütçeleri eksik |
| [ ] | F09 | Orta | Cache ve güvenlik sayaçları aynı eviction politikasını paylaşıyor |
| [ ] | F10 | Yüksek | Eski frontend yanıtları yeni ekran verisinin üzerine yazabiliyor |
| [ ] | F11 | Orta | Formlarda tekrar gönderim ve eşzamanlı puan yazımı korunmuyor |
| [ ] | F12 | Yüksek — ürün | Moderasyon ekranı karar için gereken içeriği göstermiyor |
| [ ] | F13 | Orta | Mobil yerleşim ve erişilebilirlik eksikleri var |
| [ ] | F14 | Orta | Yükleme, hata ve kullanıcı geri dönüş akışları eksik |
| [ ] | F15 | Orta | Audit kapsamı değişiklik yollarında tutarlı değil |
| [ ] | F16 | Orta | CI/CD gerçek tarayıcı ve release doğrulamasını tamamlamıyor |
| [ ] | F17 | İyileştirme | Frontend oturum sorumlulukları sadeleştirilmeli |
| [x] | F18 | Düşük | Dokümanlar mevcut uygulamayla eşitlenmeli |

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

## F08 — İşlem, sorgu ve kilit bekleme süre bütçeleri eksik

**Öncelik:** Orta · Kaynak yönetimi

**Kaynaklar:** [backend/cmd/api/main.go](../backend/cmd/api/main.go), [backend/internal/database/database.go](../backend/internal/database/database.go).

HTTP ReadTimeout/WriteTimeout var; normal handler işlemleri için genel context deadline, sorgu bütçesi veya yapılandırılmış lock_timeout görünmüyor. Bağlantının yazma süresini sınırlamak çalışan iş mantığının aynı anda iptalini garanti etmez.

**Öneri:** İstek, sorgu ve kilit bekleme sürelerini uyumlu bir bütçeyle sınırlayın. Havuz kapasitesini instance sayısı × bağlantı sayısı üzerinden planlayın.

**Kabul kriteri:** Uzun sorgu/kilit beklemesi süre sınırında sonlanmalı; bağlantı havuzu ve goroutine kaynakları serbest kalmalı.

**Referans:** [Go WriteTimeout ve request context davranışı](https://github.com/golang/go/issues/59602).

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

**Kaynaklar:** [.github/workflows/ci.yml](../.github/workflows/ci.yml), [.github/workflows/release.yml](../.github/workflows/release.yml), [backend/scripts/mvp_smoke.sh](../backend/scripts/mvp_smoke.sh).

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

## Önerilen uygulama sırası

1. F01–F04 ve F10: oturum, kötüye kullanım ve veri doğruluğu. F17'yi bu düzeltmelerin gerektirdiği ölçüde ele alın.
2. F05–F09: üretim yetkileri, sağlık kontrolleri ve kaynak sınırları.
3. F11–F15: kullanıcı akışları, moderasyon, mobil/erişilebilirlik ve audit kapsamı.
4. F16 ve F18: CI/CD ve dokümantasyon. Yeni regresyon testlerini ilgili düzeltmeyle birlikte ekleyin; son aşamaya bırakmayın.

## Çözüm kaydı şablonu

Her bulgunun altına gerektiğinde şu bilgileri ekleyin:

```text
Durum: Açık / Çalışılıyor / Doğrulama bekliyor / Tamamlandı / Risk kabul edildi
Çözüm özeti:
Commit / PR:
Doğrulama ve sonuç:
Kalan risk veya erteleme gerekçesi:
```
