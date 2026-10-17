# BookHub — Özellik talepleri ve ürün geliştirme planı

Tarih: **20 Eylül 2026** · Son güncelleme: **17 Ekim 2026** (durum özeti; sıra artık [ROADMAP.md](ROADMAP.md)'de)  
Durum: **Ürün önerisi / önceliklendirilecek backlog**

Bu belge, mevcut kitap kataloğunu kişisel kitaplık, okuma takibi ve topluluk üzerinden keşif ürününe dönüştürmek için önerilen özellikleri somutlaştırır. Özelliklerin uygulanmış olduğunu veya bütün iş kurallarının kesinleştiğini ifade etmez. Kapsam, önceki kod incelemesi ve ürün görüşmesine dayanır; güncel uygulama durumunun yeniden denetimi değildir.

Teknik hata ve güvenlik bulguları ayrı tutulur: [REVIEW_FEEDBACK.md](REVIEW_FEEDBACK.md). Mevcut proje vizyonu: [PROJECT_BRIEF.md](PROJECT_BRIEF.md). Bu doküman, önceki brief'i değiştirmez; çelişen yeni ürün kuralları aşağıda açıkça işaretlenmiştir.

## Durum özeti (17 Ekim 2026)

Kodla karşılaştırma (origin/main, Faz 14 sonrası). 34 maddenin hiçbiri tam değil: **7'si kısmen var,
27'si yok**. Hangi sırayla ele alınacakları: [ROADMAP.md](ROADMAP.md) (Faz 15 = Paket A).

| ID | Durum | Var olan | Eksik |
| --- | --- | --- | --- |
| CAT-01 | kısmen | Detayda kapak (yüklenemezse üretilmiş kapak), yazar, yıl, sayfa, dil, türler, açıklama; kendi puanı topluluk ortalamasından ayrı; benzer kitaplar | Detaydan kitaplığa ekleme (LIB-01), kendi incelemesi (REV-01), yayınevi |
| CAT-02 | karar verildi | Tasarım: [phase-16-works-editions.md](phases/phase-16-works-editions.md), [#36](https://github.com/mtgezgin/BookHub/issues/36) | Uygulama (Faz 16) |
| MOD-02 | kısmen | Backend: tekil kayıt, düzenleme (PATCH), onay/red/arşiv/geri yayın, denetim geçmişi. Arayüz: durum sekmeleri, red diyaloğu | Arayüzde bütün alanlar, öneren ve geçmiş; onay öncesi düzeltme; zorunlu red gerekçesi; eşzamanlı düzenleme uyarısı |
| DISC-01 | kısmen | Başlık/yazar araması, tür filtresi, "En yeni" ve "En yüksek puan" sıralaması; filtre URL'de, bayat yanıt koruması, boş sonuçta "filtreleri temizle" | Dil ve sayfa aralığı filtresi, başlığa göre sıralama, aktif filtre etiketleri |
| DISC-02 | kısmen | Detayda ilk türe göre benzer kitaplar, gerekçe başlıkta | Ortak türlerin birlikte değerlendirilmesi, ağırlıklı puanla sıralama |
| ACC-02 | kısmen | Mevcut parolayla değiştirme; değişince diğer bütün oturumlar kapanır (Faz 13) | "Parolamı unuttum" ve e-postayla sıfırlama |
| UX-01 | kısmen | İskelet yükleme, boş ve hata durumları, bildirimler, çift gönderim koruması, girişten güvenli dönüş | "Yeniden dene", girişten sonra yarım kalan işlemi sürdürme, F10/F11'in kalanları |
| UX-02 | kısmen | "İçeriğe atla", anlamlı etiketler, diyaloglarda odak yönetimi, mobil menü | 320/360/390 px doğrulaması (F13) |

**Yok:** LIB-01–03, READ-01–03, STATS-01–02, CAT-03, REV-01–03, MOD-01, MOD-03, MOD-04, DISC-03,
DISC-04, SOC-01–03, ACC-01, ACC-03, ACC-04 (altyapısı var: Faz 13'ün oturum aileleri), DATA-01,
DATA-02, NOTIF-01.

## 1. Ürün yönü ve temel ihtiyaç

**Ürün sözü:** Okumak istediğin kitapları bul, okumalarını takip et ve düşüncelerini paylaş.

Katalog, puan ve yorum temel etkileşimi sağlar. En büyük ürün boşluğu, kullanıcının zaman içinde değer biriktirdiği ve tekrar ziyaret ettiği kişisel alanın eksikliğidir. İlk hedef yeni ekran sayısını artırmak değil, şu döngüyü tamamlamaktır:

**Keşfet → kitaplığına ekle → okumaya başla → ilerlemeyi kaydet → bitir → değerlendir → yeni kitap keşfet.**

İkinci döngü katkı üzerinedir:

**Kitap öner → durumunu izle → geri bildirim al → düzelt → yayınlandığını gör.**

## 2. Öncelik ve takip kuralları

- **P0 — Temel:** İlk ürün paketinin parçası veya gerçek kullanıcılara açılmadan önce gerekli hesap işlevi.
- **P1 — Sonraki:** Temel kullanımın değerini artırır; P0 akışları tamamlandıktan sonra uygulanır.
- **P2 — Daha sonra:** Kullanım ve veri oluşunca değerlendirilir.
- Her özellik başlangıçta **önerildi** durumundadır. İşaretlenmeyen kutu, mevcut kodda kesinlikle hiç karşılığı olmadığı anlamına gelmez; uygulama öncesinde kapsam farkı kontrol edilir.
- Öncelik, tek başına geliştirme sırası değildir. Bağımlılıklar ve bölüm 15'teki paketler birlikte değerlendirilir.
- Sayısal aralıklar, varsayılanlar ve silme davranışları bu belgede **önerilen ürün kararlarıdır**; implementation öncesinde karara bağlanır.

## 3. Kategori haritası

| Kategori | Özellik ID'leri | Cevaplanan kullanıcı sorusu |
|---|---|---|
| Kişisel kitaplık ve raflar | LIB-01–03 | Kitaplarımı nerede düzenleyeceğim? |
| Okuma oturumları ve ilerleme | READ-01–03 | Ne okuyorum, ne kadar ilerledim? |
| Okuma hedefi ve kişisel özet | STATS-01–02 | Okuma alışkanlığım nasıl değişiyor? |
| Kitap detayı ve katalog kimliği | CAT-01–03 | Bu kitap bana uygun mu, doğru baskı mı? |
| İncelemeler ve değerlendirmeler | REV-01–03 | Okuyanlar ne düşünüyor? |
| Kullanıcı katkısı ve moderasyon | MOD-01–04 | Katkıma ne oldu, hatayı nasıl düzeltebilirim? |
| Arama, keşif ve öneriler | DISC-01–04 | Sırada ne okuyabilirim? |
| Profil, görünürlük ve sosyal akış | SOC-01–03 | Kimlerle neyi paylaşacağım? |
| Hesap erişimi ve oturum kontrolü | ACC-01–04 | Hesabımı nasıl korur ve yönetirim? |
| Veri sahipliği ve taşınabilirlik | DATA-01–02 | Verimi alabilir veya hesabımı silebilir miyim? |
| Uygulama içi bildirimler | NOTIF-01 | Katkımın sonucunu nasıl öğrenirim? |
| Ortak kullanıcı deneyimi | UX-01–02 | İşlemler anlaşılır ve erişilebilir mi? |

## 4. Kişisel kitaplık ve raflar

### LIB-01 — Kitabı kişisel kitaplığa ekleme ve durum seçme

- [ ] **P0 · Önerildi**
- **Kullanıcı ihtiyacı:** Okumak istediğim, okuduğum ve bıraktığım kitapları tek yerde tutmak.
- **Ekran/akış:** Kitap kartı veya detay → Kitaplığıma ekle → durum seç → aynı yerde seçili durumu gör.
- **İlk kapsam:** Okumak istiyorum, Şu an okuyorum, Okudum, Yarım bıraktım. Kitaplığımdan çıkar işlemi.
- **İş kuralları:** Kullanıcı-kitap çifti tek kitaplık kaydıdır. Aynı kitap tek ana okuma durumunda bulunur. Kaydın sahibi oturumdan belirlenir. Tekrarlanan ekleme ikinci kayıt oluşturmaz. Kitaplıktan çıkarma puan ve herkese açık incelemeyi sessizce silmez.
- **Kabul kriterleri:** Durum kart, detay ve kitaplıkta tutarlıdır; sayfa yenilemede korunur. Başkasının kaydı değiştirilemez. Çıkarma öncesinde okuma geçmişine etkisi açıkça gösterilir.
- **Bağımlılıklar:** Mevcut kimlik ve katalog. F10–F11 veri doğruluğu sorunlarıyla birlikte değerlendirilir.
- **İlk sürüm dışında:** Özel raflar, sürükle-bırak, toplu düzenleme.

### LIB-02 — Kitaplığım ekranı ve kişisel filtreleme

- [ ] **P0 · Önerildi**
- **Kullanıcı ihtiyacı:** Biriktirdiğim kitaplar arasında hızlıca doğru kitaba ulaşmak.
- **Ekran/akış:** Ana navigasyon → Kitaplığım → durum sekmesi/filtre → kitap detayı.
- **İlk kapsam:** Başlık/yazar araması; durum, tür ve kendi puanına göre filtreleme; eklenme tarihi ve başlığa göre sıralama; sayfalama. Kendi puanı olmayanlar için ayrı filtre.
- **İş kuralları:** Filtreler yalnız kullanıcının kitaplığını arar. Topluluk puanı ile kişisel puan etiketleri ayrıdır. Filtre ve sayfa URL'de tutulur; filtre değişince ilk sayfaya dönülür.
- **Kabul kriterleri:** Geri/ileri gezinmede filtreler korunur. Boş kitaplıkta keşfe yönlendirme, boş filtre sonucunda filtreyi temizleme eylemi bulunur. Sonuç sayısı geçerli filtreyle eşleşir.
- **Bağımlılık:** LIB-01.

### LIB-03 — Favoriler ve özel raflar

- [ ] **P2 · Önerildi**
- **Kullanıcı ihtiyacı:** Kitapları okuma durumundan bağımsız temalarla düzenlemek.
- **İlk kapsam:** Favori işareti; isimli özel raf oluşturma, yeniden adlandırma, silme; bir kitabı birden fazla özel rafa ekleme.
- **İş kuralları:** Özel raflar ana okuma durumunu değiştirmez. Raf silmek kitaplık kaydını silmez. Başlangıçta raflar özeldir; paylaşım SOC-02'ye bağlıdır.
- **Kabul kriterleri:** Bir kitap hem Okudum hem Favoriler hem de bir özel rafta bulunabilir; birinden çıkarılması diğerlerini etkilemez.
- **Bağımlılıklar:** LIB-01–02. İlk ürün paketine dahil değildir.

## 5. Okuma oturumları ve ilerleme

### READ-01 — Okuma başlatma ve ilerleme güncelleme

- [ ] **P0 · Önerildi**
- **Kullanıcı ihtiyacı:** Kitapta nerede kaldığımı kaydetmek.
- **Ekran/akış:** Kitap detayı/Kitaplığım → Okumaya başla → sayfa veya yüzde gir → ilerlemeyi güncelle.
- **İlk kapsam:** Aktif okuma kaydı, başlama tarihi, ilerleme birimi ve değeri. Toplam sayfa bilinmiyorsa yüzde kullanılabilir.
- **İş kuralları:** Kullanıcı-kitap başına en fazla bir aktif okuma kaydı. Sayfa negatif olamaz; bilinen toplamı aşamaz. Yüzde 0–100 aralığındadır. Yanlış giriş düzeltilebilmeli; ilerlemenin geriye alınması yasaklanmamalı. Başlangıçta tek birim tutulur; dönüşüm için yeterli bilgi yoksa tahmin yapılmaz.
- **Kabul kriterleri:** İlerleme yenilemede korunur; iki aktif kayıt oluşmaz. Aynı kayda eski ekran üzerinden yazım yeni değeri sessizce ezmez. Bilinmeyen toplam, 0 sayfa gibi gösterilmez.
- **Bağımlılıklar:** LIB-01; sayfa sayısı için CAT-02 kararı.

### READ-02 — Bitirme, yarım bırakma ve okuma geçmişi

- [ ] **P0 · Önerildi**
- **Kullanıcı ihtiyacı:** Ne zaman başlayıp bitirdiğimi ve yarım bıraktığımı hatırlamak.
- **İlk kapsam:** Bitirdim/Yarım bıraktım eylemleri; başlangıç/bitiş tarihini düzeltme; kitap bazında geçmiş kaydı. Önceden okunmuş kitap, tarih bilinmeden Okudum olarak eklenebilir.
- **İş kuralları:** Kitaplık durumu ve aktif okuma kaydı tutarlı güncellenir. Bilinen bitiş başlangıçtan önce olamaz. Bilinmeyen tarihler uydurulmaz. Bitirme sonrası puan/inceleme daveti isteğe bağlıdır.
- **Kabul kriterleri:** Kitabı bitirmek aktif kaydı kapatır ve kitaplık durumunu günceller. İşlem kısmen uygulanamaz. Tarihsiz tamamlanmış kitap yıllık istatistiğe rastgele bir tarihle eklenmez.
- **Bağımlılıklar:** READ-01, LIB-01.

### READ-03 — Aynı kitabı yeniden okuma

- [ ] **P1 · Önerildi**
- **Kullanıcı ihtiyacı:** İkinci okumam ilk okuma tarihlerimi silmesin.
- **İlk kapsam:** Tek kitaplık kaydına bağlı birden çok okuma kaydı; geçmiş okumaların listesi ve yeni okuma başlatma.
- **İş kuralları:** Yeni okuma önceki kaydı değiştirmez. Aynı anda en fazla bir aktif kayıt vardır. Kitaba verilen kişisel puan, her okuma için çoğaltılmaz.
- **Kabul kriterleri:** İki tamamlanmış okuma ayrı tarihlerle görünür; kişisel kitaplıkta kitap tek görünür. İstatistiklerde tekrar okuma kuralı açıkça uygulanır.
- **Bağımlılık:** READ-02. Veri modeli ilk sürümde bu genişlemeyi engellememeli.

## 6. Okuma hedefi ve kişisel özet

### STATS-01 — Aylık ve yıllık okuma özeti

- [ ] **P1 · Önerildi**
- **İlk kapsam:** Tamamlanan okuma sayısı, farklı kitap sayısı, aylara göre dağılım. Tarih veya sayfa bilgisi eksik kayıtlar için açıklama.
- **İş kuralları:** Kaynak veri okuma kayıtlarıdır. Tekrar okumalar tamamlanan okuma sayısına dahil edilir; farklı kitap sayısında tek sayılır. İlk sürümde kısmi ilerlemeleri toplayıp kesin bir “okunan sayfa” iddiası üretilmez.
- **Kabul kriterleri:** Bir bitiş tarihi değişince ilgili ay/yıl toplamları değişir. Yarım bırakılan ve tarihi bilinmeyen kayıtlar tamamlanan yıllık okuma sayısına dahil edilmez. Yıl sınırı kullanıcının belirlenen saat dilimine göre hesaplanır.
- **Bağımlılık:** READ-02; tekrar okuma için READ-03.

### STATS-02 — Yıllık kitap hedefi

- [ ] **P2 · Önerildi**
- **İlk kapsam:** Yıla özel pozitif tam sayı hedefi; güncelleme, kaldırma ve gerçekleşme göstergesi.
- **İş kuralları:** Önerilen ölçü tamamlanan okuma sayısıdır; tekrar okumaların dahil olduğu kullanıcıya açıklanır. Hedef değiştirmek geçmiş kayıtları değiştirmez.
- **Kabul kriterleri:** Hedef ve STATS-01 aynı sayıyı kullanır. Hedefe ulaşma yalnız doğru yılın kayıtlarına dayanır. Hedef belirlemeyen kullanıcı eksik profil olarak işaretlenmez.
- **Bağımlılık:** STATS-01. İlk sürüm dışında: seri koruma, rozet ve yarışma.

## 7. Kitap detayı ve katalog kimliği

### CAT-01 — Karar vermeyi destekleyen kitap detayı

- [ ] **P0 · Önerildi**
- **İlk kapsam:** Kapak, başlık, yazar, açıklama, türler, dil, yayın ve sayfa bilgisi; kitaplığa ekleme; kendi puanı/incelemesi; topluluk değerlendirmeleri. Benzer kitaplar DISC-02 hazır olduğunda eklenir.
- **İş kuralları:** Bilinmeyen alanlar yanlış varsayılanlarla doldurulmaz. Kapak yüklenemezse yer tutucu gösterilir. Kullanıcının puanı topluluk ortalamasından ayrılır.
- **Kabul kriterleri:** Kullanıcı detaydan ayrılmadan kitabı kitaplığına ekleyebilir. Boş açıklama veya bozuk kapak sayfayı bozmaz. Mobilde temel eylemler erişilebilir ve kitap kimliği yüklenmeden yazma işlemi yapılamaz.
- **Bağımlılıklar:** LIB-01, REV-01, mevcut katalog; parçalar bağımsız teslim edilebilir.

### CAT-02 — Eser ve baskı kimliği kararı

- [ ] **P0 karar / P1 tam uygulama · Karar verildi (17 Ekim 2026) · Planlandı: Faz 16**
- **Karar kaydı:** [phases/phase-16-works-editions.md](phases/phase-16-works-editions.md) ·
  [#36](https://github.com/mtgezgin/BookHub/issues/36). `books` eser olarak kalır; yeni `editions`
  tablosu, kitap başına bir varsayılan baskı; puan ve yorum eser düzeyinde; genişlet → daralt, iki PR.
  Baskıları ilk sürümde yalnız admin ekler. Okuma kaydının baskıya bağlanması Faz 17'de.
- **İhtiyaç:** Aynı eserin farklı dil, çeviri ve baskılarını yanlışlıkla bağımsız kitaplar gibi çoğaltmamak.
- **Önerilen ayrım:** Eser: ortak başlık/kimlik/yazar ilişkisi. Baskı: ISBN, dil, yayınevi, çevirmen, yayın yılı, kapak ve sayfa sayısı. Bir eserin birden çok baskısı olabilir; ISBN her eserde zorunlu değildir.
- **Önerilen ürün kuralı:** Puan ve inceleme eser düzeyinde; okuma kaydı isteğe bağlı baskıya bağlı. Baskıya özgü eleştiri incelemede bağlam olarak gösterilebilir. Bu kural uygulama öncesi karara bağlanmalıdır.
- **İlk teslimat:** Kısa bir karar kaydı ve mevcut book ID'lerinin eşlenme/migration planı. Tam baskı seçici sonraki pakette olabilir.
- **Kabul kriterleri:** İki baskının farklı sayfa/kapak bilgisi tutulabilir. Mevcut URL, puan ve kitaplık ilişkileri kaybolmaz. Baskı değişimi eski ilerlemeyi sessizce yeni sayfa toplamına dönüştürmez.
- **Bağımlılıklar:** READ-01, CAT-03 ve katalog birleştirme tasarımını etkiler; kapsam büyümeden karar verilmelidir.

### CAT-03 — ISBN ile katalog bilgisi ön doldurma

- [ ] **P2 · Önerildi**
- **İlk kapsam:** ISBN girişi → biçim/checksum doğrulama → önce mevcut katalogda arama → gerekirse dış kaynaktan taslak → kullanıcı/admin kontrolü → normal öneri akışı.
- **İş kuralları:** Dış sonuç doğrudan yayınlanmaz. Bulunan mevcut baskı için tekrar oluşturmak yerine mevcut kayda yönlendirilir. Sağlayıcı kesintisi manuel girişi engellemez.
- **Kabul kriterleri:** Aynı ISBN için mükerrer baskı üretilmez. Eksik veya hatalı metadata düzeltilebilir. Kaynak ve alınma zamanı izlenebilir.
- **Bağımlılıklar:** CAT-02, MOD-01–02. Sağlayıcı seçimi, kullanım şartları, görsel kullanım hakları ve kota ayrıca araştırılır; bu belge sağlayıcı taahhüdü içermez.

## 8. İncelemeler ve değerlendirmeler

### REV-01 — Kendi kitap incelemesini yazma ve yönetme

- [ ] **P0 · Önerildi**
- **Kullanıcı ihtiyacı:** Puanımın nedenini anlatmak ve sonradan düzeltmek.
- **İlk kapsam:** Kitap başına kullanıcıya ait tek inceleme; yazma, düzenleme ve silme; oluşturma/güncelleme tarihi. Mevcut puan aynı kaynaktan gösterilir, ikinci bir puan alanı oluşturulmaz. Puan vermeden inceleme yazmak mümkündür.
- **İş kuralları:** İnceleme uzun değerlendirmedir; mevcut kısa yorumlarla aynı şey değildir. Kullanıcı yalnız kendi incelemesini yönetir. İncelemeyi silmek puanı otomatik silmez; bu açıkça belirtilir. Metin sınırı ve moderasyon görünürlüğü belirlenir.
- **Kabul kriterleri:** Başkasının incelemesi değiştirilemez. İkinci inceleme oluşturmak yerine mevcut inceleme düzenlenir. Düzenlendi bilgisi görünür; moderatörce gizlenmiş içerik kullanıcı düzenlemesiyle kendiliğinden yayınlanmaz.
- **Bağımlılıklar:** Mevcut kimlik/puanlama; MOD-04.
- **Brief ile ilişki:** Eski brief yalnız admin'in yorum silebileceğini söylüyor. Buradaki kullanıcı silme hakkı yeni inceleme nesnesi içindir; mevcut yorum politikası sessizce değiştirilmez. İki nesne birleştirilecekse açık ürün kararı gerekir.

### REV-02 — Spoiler işareti ve kontrollü gösterim

- [ ] **P0 · Önerildi**
- **İlk kapsam:** İnceleme yazarken spoiler işareti; listede ve detayda gizlenmiş metin; Spoiler'ı göster eylemi.
- **İş kuralları:** Gizleme yalnız detayda değil kart, profil ve bildirim önizlemesinde de uygulanır. İşaretli içeriğin metin özeti otomatik açığa çıkarılmaz.
- **Kabul kriterleri:** Kullanıcı açmadan spoiler metnini görmez. Klavye ve ekran okuyucuyla açma kontrolü kullanılabilir. İşaret düzenlenebilir.
- **Bağımlılık:** REV-01. İlk sürümde paragraf bazlı karmaşık editör gerekmez.

### REV-03 — İncelemeyi faydalı bulma ve sıralama

- [ ] **P1 · Önerildi**
- **İlk kapsam:** Faydalı bul/geri al; En yeni ve Faydalı bulunan sıralaması; eşitlikte deterministik tarih/ID sırası.
- **İş kuralları:** Kullanıcı-inceleme başına tek oy; kendi incelemesine oy veremez. İşlem kotası uygulanır. Gizli/silinmiş inceleme listelenmez.
- **Kabul kriterleri:** Tekrarlanan istek sayacı şişirmez. Oy geri alınınca sayı tutarlı azalır. Sayfalama kararlı çalışır.
- **Bağımlılıklar:** REV-01, MOD-04. İlk sürüm dışında: yanıt zincirleri ve özel mesajlar.

## 9. Kullanıcı katkısı ve moderasyon

### MOD-01 — Önerilerim ve düzeltip yeniden gönderme

- [ ] **P0 · Önerildi**
- **İlk kapsam:** Kullanıcının kendi önerileri; bekliyor/onaylandı/reddedildi durumları; tarih ve red gerekçesi; onaylanan kitap bağlantısı. Reddedilen öneriyi düzeltip tekrar incelemeye gönderme.
- **İş kuralları:** Yayınlanmamış öneriyi sahibi ve yetkili moderatör görür. Yeniden gönderim önceki karar ve içeriğin izini silmez; öneri revizyonu ve katalog yayın durumu ayrı ele alınır.
- **Kabul kriterleri:** Başka kullanıcı bekleyen öneriyi okuyamaz. Red gerekçesi görünür. Yeniden gönderilen içerik onaysız yayınlanmaz; önceki karar geçmişte kalır.
- **Bağımlılıklar:** Mevcut öneri sistemi, MOD-02.
- **Karar gerektirir:** Mevcut REJECTED durumunun terminal olması bu akışla değişir. State machine, audit ve kullanıcı izinleri birlikte güncellenmelidir.

### MOD-02 — Tam moderasyon inceleme ekranı

- [ ] **P0 · Önerildi**
- **İlk kapsam:** Önerinin bütün alanları, öneren, revizyon ve işlem geçmişi; onay öncesi düzeltme; gerekçeli red; arşivleme ve geri yayınlama.
- **İş kuralları:** Önerilen yeni politika: red gerekçesi zorunlu. İki admin'in aynı kayıt üzerindeki çakışması sessizce üzerine yazma yerine açık uyarı üretir. Kullanıcıya gerekçe ile sistem içi not aynı alan değildir.
- **Kabul kriterleri:** Yayınlanacak içerik karar öncesi görülebilir. İptal eylemi kayıt değiştirmez. Karar, düzenleme ve audit tutarlıdır. Yanlış kararın düzeltme yolu bellidir.
- **Bağımlılıklar:** F12/F15 ile birlikte ele alınır. Red gerekçesinin zorunluluğu mevcut opsiyonel davranıştan farklı, önerilen bir ürün kararıdır.

### MOD-03 — Kitap bilgisi hatası bildirme

- [ ] **P1 · Önerildi**
- **İlk kapsam:** Kitap detayında Bilgi hatası bildir → alan seçimi → önerilen doğru değer ve isteğe bağlı kaynak → admin kuyruğu → kabul/red sonucu.
- **İş kuralları:** Kullanıcı doğrudan katalog değiştirmez. Bildirim ve uygulanan değişiklik bağlantılı tutulur. Aynı kullanıcının aynı kayıt için açık tekrar bildirimleri sınırlandırılır.
- **Kabul kriterleri:** Admin mevcut ve önerilen değeri karşılaştırabilir. Kabul edilen düzeltme audit üretir; reddedilen bildirim kataloğu değiştirmez.
- **Bağımlılık:** MOD-02. Mükerrer eserleri birleştirme ayrı, daha kapsamlı bir iş olarak planlanır.

### MOD-04 — İçerik şikâyeti ve inceleme kuyruğu

- [ ] **P1; incelemeler geniş kullanıma açılmadan önce · Önerildi**
- **İlk kapsam:** Yorum/inceleme için spam, hakaret, işaretlenmemiş spoiler ve diğer gerekçeleri; açıklama; admin kuyruğu; içeriği gizleme veya şikâyeti kapatma.
- **İş kuralları:** Şikâyetçi kimliği halka açılmaz. Tekrarlı şikâyetler birleştirilir/sınırlanır. Salt şikâyet sayısı otomatik silme sebebi değildir. Kararın aktörü ve gerekçesi kaydedilir.
- **Kabul kriterleri:** Şikâyet sahibinin kimliği içerik sahibine sızmaz. Gizlenen içerik liste, detay ve profilde tutarlı biçimde kaldırılır. Yetkisiz kullanıcı moderasyon işlemi yapamaz.
- **Bağımlılıklar:** Mevcut yorumlar veya REV-01; admin yetkileri ve rate limit.

## 10. Arama, keşif ve öneriler

### DISC-01 — Katalog araması ve kullanılabilir filtreler

- [ ] **P1 · Önerildi**
- **İlk kapsam:** Başlık/yazar araması; tür, dil ve sayfa aralığı; Son eklenen, Başlık ve En yüksek ağırlıklı puan sıralamaları. Aktif filtre etiketleri ve tümünü temizle.
- **İş kuralları:** Sayfa aralığında sayfa sayısı bilinmeyen kayıtlar eşleşmiş sayılmaz. Yalnız yayınlanmış kayıtlar gösterilir. Filtre değişince sayfa sıfırlanır. Türkçe karakter ve büyük/küçük harf davranışı örneklerle tanımlanır.
- **Kabul kriterleri:** Paylaşılan URL aynı filtre durumunu açar. Hızlı arama değişimlerinde eski sonuçlar gelmez. Boş sonuçta filtre temizleme yolu vardır.
- **Bağımlılıklar:** Katalog metadata kalitesi, F10/F14. Arama motoru eklemek ön koşul değildir; önce mevcut sorgular ölçülür.

### DISC-02 — Benzer kitaplar

- [ ] **P1 · Önerildi**
- **İlk kapsam:** Detayda ortak türlere dayalı sınırlı liste; eşitlikte mevcut ağırlıklı puan ve kararlı ID sırası. Öneri gerekçesi: Örneğin “Bilim kurgu türünden”.
- **İş kuralları:** Mevcut kitap ve yayınlanmamış kayıtlar dışlanır. Yeterli benzer kayıt yoksa ilgisiz sonuçlarla doldurulmaz. Yazar benzerliği, katalog kimliği güvenilir olduğunda eklenir.
- **Kabul kriterleri:** Aynı veri için kararlı ve tekrarsız sonuçlar; her sonuç için açıklanabilir eşleşme. Kitap arşivlenince görünürlük kuralları uygulanır.
- **Bağımlılıklar:** Türler, CAT-01. AI veya embedding gerektirmez.

### DISC-03 — Tematik keşif bölümleri

- [ ] **P1 · Önerildi**
- **İlk kapsam:** Son eklenenler, Kısa kitaplar ve Az bilinen, beğenilenler. Her bölümün liste ekranı ve dahil edilme açıklaması.
- **İş kuralları:** Son eklenenler için öneri tarihi yerine yayınlanma/onay zamanı kullanılması önerilir. Kısa kitap eşiği açık gösterilir. Az bilinen bölümünde minimum oy, maksimum oy ve ağırlıklı puan eşiği tanımlanır; tek yüksek oy yeterli olmaz.
- **Kabul kriterleri:** Her bölümün ölçütü belgelenir ve sonuçlar buna uyar. Eşikler uygulama öncesinde örnek veriyle seçilir. Az veri varsa bölüm gizlenir veya dürüst boş durum gösterilir.
- **Bağımlılıklar:** Güvenilir metadata ve puan verisi. Eşikler bu belgede kesinleştirilmemiştir.

### DISC-04 — Tercihlere dayalı kişisel keşif

- [ ] **P2 · Önerildi**
- **İlk kapsam:** İsteğe bağlı favori tür seçimi; seçilen türler ve yüksek kişisel puanlardan türeyen basit öneriler; Neden önerildi açıklaması.
- **İş kuralları:** Okudum/Yarım bıraktım kayıtları varsayılan olarak dışlanır; öneri kuralı belgelenir. Veri yoksa genel keşif gösterilir. Kullanıcı tercihini değiştirebilir veya sıfırlayabilir.
- **Kabul kriterleri:** Yeni kullanıcı boş öneri ekranında kalmaz. Her sonuç anlaşılır gerekçeye sahiptir. Tercih değişimi sonucu etkiler; gizli profil verisi başka kullanıcılara açılmaz.
- **Bağımlılıklar:** LIB-01, kişisel puanlar, DISC-02–03. İlk sürüm dışında: benzer kullanıcı modeli ve AI önerileri.

## 11. Profil, görünürlük ve sosyal akış

### SOC-01 — Profil sayfası ve kişisel ana sayfa

- [ ] **P1 · Önerildi**
- **İlk kapsam:** Görünen ad, kısa biyografi, izin verilen kitaplık ve incelemeler. Kullanıcının kendi ana sayfasında Şu an okuyorum ve hızlı ilerleme güncelleme alanı.
- **İş kuralları:** E-posta profil bilgisi olarak yayınlanmaz. Sahip görünümü ile ziyaretçi görünümü ayrıdır. İlk sürümde avatar yükleme zorunlu değildir.
- **Kabul kriterleri:** Sahip ve ziyaretçi yalnız izin verilen alanları görür. Okunan kitap yoksa kitap seçmeye yönlendirme bulunur.
- **Bağımlılıklar:** ACC-03, LIB-01, READ-01, SOC-02. Kişisel ana sayfa, herkese açık profil hazır olmadan teslim edilebilir.

### SOC-02 — Kitaplık görünürlüğü

- [ ] **P0 varsayılan gizlilik / P1 paylaşım · Önerildi**
- **İlk kapsam:** Kitaplık için Özel/Herkese açık seçimi; varsayılan Özel. İncelemeler yayınlama eylemiyle herkese açılır; kitaplığın gizli olması incelemeyi otomatik gizlemez ve bu fark açıklanır.
- **İş kuralları:** Kontrol yalnız UI'da değil backend'de uygulanır. Gizli kitaplık sayıları, okuma tarihleri ve ilerleme verileri de korunur. Admin erişimi gerekiyorsa açık yetki ve amaçla sınırlanır.
- **Kabul kriterleri:** Özel kitaplığa doğrudan API çağrısı veri döndürmez. Gizliye alınan kitaplık önbellek ve sosyal gösterimlerde görünmez. Kullanıcı hangi alanın paylaşıldığını önizleyebilir.
- **Bağımlılık:** LIB-01. Sosyal akış yayınlanmadan önce tamamlanır.

### SOC-03 — Kullanıcı takip etme ve faaliyet akışı

- [ ] **P2 · Önerildi**
- **İlk kapsam:** Takip et/bırak; takip edilenlerin açık incelemeleri ve izin verdikleri kitap bitirme olayları; tarih sıralı sayfalama.
- **İş kuralları:** Kişi kendini takip edemez; ilişki tekildir. Okuma paylaşımı ayrıca isteğe bağlıdır. Gizli kitaplık faaliyeti yayınlanmaz. Gizlilik değişimi veya içerik silinmesi eski faaliyetlerin görünürlüğüne yansır.
- **Kabul kriterleri:** Tek olay tekrar tekrar görünmez. Takip bırakma sonrası kişinin akışı artık dahil edilmez. Gizli faaliyet veri, sayaç veya önizleme üzerinden sızmaz.
- **Bağımlılıklar:** SOC-01–02, REV-01, READ-02; sosyal yayına çıkmadan engelleme/kötüye kullanım politikası da belirlenir.
- **İlk sürüm dışında:** Mesajlaşma, algoritmik akış, gruplar ve okuma kulüpleri.

## 12. Hesap erişimi ve oturum kontrolü

### ACC-01 — E-posta doğrulama

- [ ] **P0 — genel kullanıma açılmadan önce · Önerildi**
- **İlk kapsam:** Kayıt sonrası doğrulama e-postası, tek kullanımlık süreli bağlantı, yeniden gönderme, başarı/süre doldu ekranları.
- **İş kuralları:** Token özeti saklanır; yeniden gönderim kotası vardır. Önerilen kural: doğrulanmamış hesap kişisel kitaplığını kullanabilir, herkese açık katkı için doğrulama gerekir. Mevcut hesaplara geçiş davranışı açıkça planlanır.
- **Kabul kriterleri:** Süresi geçmiş/kullanılmış bağlantı reddedilir. Tekrarlı gönderim sınırlandırılır. E-posta servisi hatası hesap oluşturuldu bilgisini kaybettirmez ve yeniden deneme yolu sunulur.
- **Bağımlılıklar:** E-posta gönderim altyapısı ve teslimat takibi; sağlayıcı ayrıca seçilir.

### ACC-02 — Parola sıfırlama ve değiştirme

- [ ] **P0 — genel kullanıma açılmadan önce · Önerildi**
- **İlk kapsam:** Parolamı unuttum; süreli tek kullanımlık bağlantı; yeni parola; girişli kullanıcı için mevcut parolayla değiştirme.
- **İş kuralları:** Sıfırlama talebi hesabın varlığını ifşa etmez. Token özeti saklanır ve kullanım parola değişikliğiyle atomik olur. Parola sıfırlama sonrası bütün oturum ailelerinin iptali önerilir; mevcut access token'ların ne kadar yaşayacağı F06 politikasıyla uyumlu olmalıdır.
- **Kabul kriterleri:** Aynı bağlantı ikinci kez kullanılamaz. Başarılı sıfırlama sonrası eski parola ve iptal edilmiş refresh oturumları kullanılamaz. İşlem yarıda kalırsa tutarsız parola/token durumu oluşmaz.
- **Bağımlılıklar:** ACC-01 e-posta altyapısı; oturum iptalinin güvenilir olması.

### ACC-03 — Profil ve hesap ayarları

- [ ] **P0 temel profil / P1 genişletme · Önerildi**
- **İlk kapsam:** Görünen adı düzenleme; profil görünürlüğü ve ileride biyografi. Hesap e-postası görünür ama ilk sürümde değiştirilemeyebilir.
- **İş kuralları:** Rol istemciden değiştirilemez. E-posta değiştirme eklenirse yeni adres doğrulanmadan geçiş tamamlanmaz ve hassas işlem için yeniden doğrulama gerekir.
- **Kabul kriterleri:** Güncellenen ad ilgili ekranlarda tutarlıdır. Yetkisiz alan gönderimi rol değiştiremez. E-posta değişikliği ilk kapsam dışında kalıyorsa UI bunu vaat etmez.
- **Bağımlılıklar:** Mevcut hesap sistemi; görünürlük için SOC-02.

### ACC-04 — Aktif oturumları görme ve sonlandırma

- [ ] **P1; F01–F03 ön koşul · Önerildi**
- **İlk kapsam:** Oturum başlangıcı, son etkinlik ve yaklaşık tarayıcı/cihaz etiketi; Bu oturum işareti; tek oturumu veya diğerlerini kapatma.
- **İş kuralları:** Liste yalnız sahibine açıktır; token değerleri gösterilmez. User-Agent kesin cihaz kimliği veya güven kanıtı olarak kullanılmaz. Access token iptal gecikmesi varsa kullanıcıya doğru anlatılır.
- **Kabul kriterleri:** Seçilen ailenin refresh işlemi reddedilir; diğer aileler korunur. Başka kullanıcının aile ID'siyle sonlandırma yapılamaz. Diğerlerini kapat mevcut oturumu korur.
- **Bağımlılıklar:** Mevcut refresh aileleri, F01–F03/F06 kararları. Yaklaşık konum gösterimi ilk sürüm dışında.

## 13. Veri sahipliği ve taşınabilirlik

### DATA-01 — Kişisel kitaplığı dışa aktarma

- [ ] **P1 · Önerildi**
- **İlk kapsam:** CSV veya JSON olarak kitaplık, okuma durumu/tarihleri, kişisel puan ve kendi incelemeleri. Format sürümü ve dışa aktarım zamanı.
- **İş kuralları:** Yalnız kullanıcının verisi çıkarılır. CSV kullanılıyorsa formül enjeksiyonuna karşı hücreler güvenli üretilir. UTF-8 ve kararlı alan adları kullanılır; token/e-posta gibi gereksiz hesap verisi eklenmez.
- **Kabul kriterleri:** Türkçe karakter, satır sonu ve tırnak içeren içerik bozulmaz. Gizli kitaplık sahibine aktarılabilir; başkasına aktarılamaz. Çıktıdaki kayıt sayısı kapsamla eşleşir.
- **Bağımlılıklar:** LIB/READ/REV alan modelleri. CSV içe aktarma ayrı P2 işidir; ilk sürüme dahil değildir.

### DATA-02 — Hesap silme ve bağlı verilerin akıbeti

- [ ] **P0 — genel kullanıma açılmadan önce · Önerildi**
- **İlk kapsam:** Etkileri gösteren hesap silme ekranı, yakın zamanda kimlik doğrulama, açık onay, işlem sonucu.
- **Karara bağlanacak veri politikası:** Özel kitaplık/okuma kayıtlarının silinmesi; kamusal yorum/incelemelerin silinmesi veya anonimleştirilmesi; puanların kaldırılması; önerilen katalog kayıtlarının korunması; audit saklama kapsamı. Bu belge hukuki saklama süresi belirlemez.
- **İş kuralları:** Oturumlar iptal edilir. Puan kaldırılıyorsa kitap istatistikleri güncellenir. İlişkili kayıtlar üzerinde kısmi silme hatası oluşmaması ve işlemin tekrar denenebilmesi tasarlanır.
- **Kabul kriterleri:** Eski hesap erişim sağlayamaz; özel kayıtlar erişilemez. Kamusal içerik seçilen politikaya uyar. Puan özetleri tutarlıdır; başka kullanıcı veya ortak katalog verisi yanlışlıkla silinmez.
- **Bağımlılıklar:** F01/F06, veri ilişkilerinin envanteri; sonradan eklenen her özellik silme politikasına dahil edilir.

## 14. Bildirimler ve ortak kullanıcı deneyimi

### NOTIF-01 — Öneri ve bildirim sonuçları için uygulama içi gelen kutusu

- [ ] **P1 · Önerildi**
- **İlk kapsam:** Öneri onaylandı/reddedildi ve bilgi düzeltme talebi sonuçlandı bildirimleri; okunmamış sayısı; okundu işareti; ilgili kayda bağlantı.
- **İş kuralları:** Bildirim başarılı karardan sonra oluşur; tekrar işleme aynı olayı çoğaltmaz. Yetki kontrolü hedef ekranda da uygulanır. Şikâyetçi kimliği veya özel moderasyon notu gösterilmez.
- **Kabul kriterleri:** Başarısız moderasyon işlemi başarı bildirimi üretmez. Aynı karar tek bildirim oluşturur. Bildirimi okundu yapmak diğer kullanıcının verisini etkilemez.
- **Bağımlılıklar:** MOD-01–03. İlk sürüm dışında: push ve pazarlama e-postaları. MOD-01 bildirim olmadan da kullanılabilir olmalı.

### UX-01 — İşlem durumu ve giriş sonrası devam etme

- [ ] **P0 · Önerildi**
- **İlk kapsam:** Yükleniyor, boş, hata, başarılı durumları; yeniden dene; işlem sırasında tekrar gönderim koruması; giriş sonrası başlatılan akışa dönüş.
- **İş kuralları:** Dönüş adresi uygulama içi güvenli rotayla sınırlıdır. Başarılı işlem sonrası mesajın süresi ve konumu tutarlıdır. Yeniden deneme kullanıcı metnini kaybettirmez.
- **Kabul kriterleri:** Anonim kullanıcı kitaplığa eklemek için giriş yaptığında aynı kitaba döner ve niyetini tamamlayabilir. Ağ hatası formu temizlemez. Eski istek yeni ekrana veri yazamaz.
- **Bağımlılıklar:** F10/F11/F14; her P0 akışının teslimatına dahildir.

### UX-02 — Mobil ve erişilebilir ortak bileşenler

- [ ] **P0 · Önerildi**
- **İlk kapsam:** Dar ekranda navigasyon, filtreler, kartlar ve form düzeni; anlamlı etiketler; klavye odağı; seçili durum ve işlem sonucunun erişilebilir bildirimi.
- **Kabul kriterleri:** 320/360/390 px genişliklerde yatay taşma yoktur. Temel akışlar klavyeyle tamamlanır. Dialog kapanınca odak başlatan kontrole döner. Puan, raf durumu ve hata yalnız renkle anlatılmaz.
- **Bağımlılık:** F13. Ayrı bir son rötuş olarak ertelenmez; her yeni ekranın kabul koşuludur.

## 15. Teslimat paketleri

> Sıra (17 Ekim 2026, [ROADMAP](ROADMAP.md)): Paket A = Faz 15 (şimdi) · CAT-02 baskılar = Faz 16 ·
> Paket B = Faz 17 · Paket C = Faz 18 · D ve E daha sonra.

### Paket A — Kişisel kitaplığın ilk kullanılabilir sürümü

**Çıktı:** Kullanıcı kitap bulur, kitaplığına ekler ve kendi listesini yönetir.

- LIB-01, LIB-02, CAT-01'in katalog/kitaplık bölümü.
- CAT-02 kimlik kararı; tam baskı sistemi bu pakette zorunlu değil.
- SOC-02 özel kitaplık varsayılanı; UX-01–02.
- Teknik ön koşul: [inceleme belgesindeki](REVIEW_FEEDBACK.md) oturum ve veri doğruluğu bulgularının ilgili akışları etkilememesi.

### Paket B — Okuma takibi ve değerlendirme

**Çıktı:** Kullanıcı okumaya başlar, ilerlemesini kaydeder, bitirir ve değerlendirme yazar.

- READ-01–02, REV-01–02, CAT-01'in değerlendirme bölümü.
- Mevcut yorum/inceleme ayrımı ve silme kuralları karara bağlanır.
- Geniş kullanıma açılış öncesinde MOD-04 koruması tamamlanır.

### Paket C — Katkı döngüsü ve hesap sahipliği

**Çıktı:** Kullanıcı önerisinin sonucunu görür; hesabını kurtarabilir, düzenleyebilir ve silebilir.

- MOD-01–02, ACC-01–03, DATA-02.
- Bunların tamamlanması genel kullanıma açılma kapısıdır; yalnız geliştirme sıralamasında C yazması güvenlik/hak sahipliği özelliklerini erteleme gerekçesi değildir.
- NOTIF-01, MOD-03, ACC-04 ve DATA-01 takip eden küçük teslimatlar olabilir.

### Paket D — Keşif ve okuma özeti

**Çıktı:** Kullanıcı uygun yeni kitap bulur ve geçmiş okumalarını anlamlandırır.

- DISC-01–03, STATS-01, READ-03, REV-03.
- Metadata ve kullanıcı verisi yeterliliği doğrulanır.

### Paket E — Sosyal ve kişiselleştirilmiş deneyim

**Çıktı:** İsteğe bağlı paylaşım ve açıklanabilir kişisel öneriler.

- SOC-01–03'ün kamusal bölümleri, DISC-04, STATS-02, LIB-03, CAT-03.
- Ön koşul: Gizlilik, moderasyon ve hesap silmenin bütün yeni veri türlerini kapsaması.

## 16. Veri modeli ve tasarım sınırları

Bu tablo önerilen kavramsal ayrımdır; migration veya zorunlu tablo adı değildir.

| Kavram | Sorumluluk | Karıştırılmaması gereken |
|---|---|---|
| Katalog eseri | Ortak kitap kimliği | Belirli dil/baskının sayfa sayısı |
| Baskı | ISBN, dil, yayınevi, kapak, sayfa | Kullanıcının okuma ilerlemesi |
| Kitaplık kaydı | Kullanıcı-eser ilişkisi ve güncel durum | Her yeniden okumada yeni katalog/kitaplık kaydı |
| Okuma kaydı | Başlama/bitiş, ilerleme, seçilmiş baskı | Herkesin gördüğü puan |
| Puan | Kullanıcı-eser başına tek değerlendirme sayısı | İnceleme içinde ikinci bağımsız puan |
| İnceleme | Kullanıcının uzun değerlendirmesi ve spoiler durumu | Kısa yorum veya özel kişisel not |
| Öneri revizyonu | İncelemeye sunulan içerik ve karar geçmişi | Yayındaki katalog kaydını doğrudan değiştirme |
| Faaliyet/bildirim | Kaynak olayın kullanıcıya gösterimi | Yetki kontrolünü atlayan kalıcı veri kopyası |

Mevcut Go modüler monolit yapısı sürdürülebilir. Yeni bir özellik; migration, domain kuralları, API ve kullanıcı ekranıyla dikey bir parça olarak teslim edilmeli. Sırf bu backlog büyüdü diye mikroservis, mesaj kuyruğu, ayrı arama motoru veya AI katmanı eklenmemeli.

## 17. Başarı ölçümü

Başlangıçta sayısal başarı hedefleri uydurulmaz; önce küçük bir kullanım grubu üzerinden başlangıç değerleri ölçülür. Ölçüm için özel kitap adı, inceleme metni veya e-posta toplamak gerekmez.

| Amaç | Önerilen ölçüm | Tanım |
|---|---|---|
| İlk değer | Kitaplığa ilk ekleme oranı | Belirlenen yeni kullanıcı grubunda en az bir kitap ekleyenler / gruptaki kullanıcılar |
| Düzenli kullanım | Okuma takibine dönüş | İlk eklemeden sonraki 7 gün içinde ilerleme güncelleyen kullanıcı oranı |
| Akış tamamlama | Başlatma → bitirme | Aynı okuma kaydının tamamlanması; gözlem süresi ve henüz aktif kayıtlar ayrı raporlanır |
| Katkı kalitesi | Moderasyon bekleme süresi | Gönderim ile ilk karar arasındaki süre; yeniden gönderimler ayrıca izlenir |
| Keşif değeri | Öneriden kitaplığa ekleme | Öneri bölümünden açılan kitapların aynı oturumda kitaplığa eklenme oranı |
| Sağlamlık | Başarısız temel işlem oranı | Ekleme/ilerleme/inceleme işlemlerindeki teknik hata oranı; doğrulama hataları ayrı tutulur |

## 18. Şimdilik ertelenecek fikirler

- AI sohbeti ve notlar üzerinde soru-cevap: Önce gerçek not/veri ve kullanıcı ihtiyacı oluşmalı.
- Okuma kulübü, grup sohbeti ve özel mesaj: Moderasyon ve güvenlik kapsamı belirgin şekilde büyür.
- Rozet, seri ve liderlik tablosu: Temel okuma deneyimi yeterli olmadan aktiviteyi yanlış teşvik edebilir.
- Karmaşık makine öğrenmesi önerileri: Az veriyle anlamlı avantajı gösterilmeden eklenmez.
- Tam sosyal ağ, gelişmiş editör ve kapsamlı baskı kataloğunu aynı sürümde kurma: İlk ürün paketini gereksiz büyütür.
- Özel notlar, alıntılar ve CSV içe aktarma: Değerli genişlemelerdir; ayrı gereksinim, gizlilik ve veri eşleme tasarımıyla sonraki backlog'a alınabilir.

## 19. Özellik bazında uygulama kaydı

Bir özellik işe alındığında ilgili bölümün altına şu kayıt eklenebilir:

```text
Durum: Önerildi / Karar bekliyor / Planlandı / Geliştiriliyor / Doğrulanıyor / Tamamlandı / Ertelendi
Kesinleşen kapsam ve ürün kararları:
Bağımlılıklar:
Etkilenen ekranlar ve API sözleşmesi:
Migration / mevcut veri geçişi:
Commit veya PR:
Kabul kriteri doğrulama sonuçları:
Kalan iş ve riskler:
```

Bir özellik yalnız endpoint hazır olduğunda tamamlanmış sayılmaz. İlgili kullanıcı akışı, sahiplik kontrolleri, mobil/erişilebilir durumlar, hata/boş/yükleme davranışları ve mevcut verinin geçişi birlikte doğrulanmalıdır.
