# BookHub — Yol haritası (ROADMAP)

> Son güncelleme: **2026-11-23**. Bu dosya tek bir soruyu cevaplar: **hangi sırayla ve neden?**
> "Ne yapılabilir?" → [FEATURE_REQUESTS.md](FEATURE_REQUESTS.md) (ürün) ve
> [REVIEW_FEEDBACK.md](REVIEW_FEEDBACK.md) (teknik bulgular). "Neredeyiz?" → [STATUS.md](STATUS.md).

## Hedef

**Üretim kalitesinde demo** (kullanıcı kararı, 2026-10-17; repo public olduktan sonra 2026-11-09'da
yinelendi — kayıt ekranında ve README'de demo uyarısı var, [#64](https://github.com/mtgezgin/BookHub/issues/64)).
Site gerçek kullanıcılara açılmıyor; amaç
ürün döngüsünü uçtan uca, canlıda ve üretim standartlarında göstermek:

**Keşfet → kitaplığına ekle → okumaya başla → bitir → değerlendir.**

Gerçek kullanıcıya açılma kararı verilirse Faz 18 (katkı ve hesap) ile canlı izleme öne alınır.

## Neredeyiz

- **Platform hazır:** Faz 1–16d bitti — API, veritabanı, auth, moderasyon, puan, önbellek, Docker,
  React, CI/CD, NGINX, Kubernetes, oturum sertleştirme, arayüz, kişisel kitaplık, eser ve baskı, Türkçe
  arama, büyük katalog için sorgular, arayüz veri katmanı ([STATUS](STATUS.md)).
- **Faz 17'den önce (kullanıcı kararı, 2026-11-17/18):** ikinci incelemenin düzeltmeleri. Partiler
  A, B, index'ler, arayüz sağlamlığı ve yayın güveni canlıda (#90–#95); arayüz veri katmanı Faz 16d
  olarak canlıda ([#105](https://github.com/mtgezgin/BookHub/issues/105), 2026-11-22); şifreli yedek
  canlıda ([#100](https://github.com/mtgezgin/BookHub/issues/100), 2026-11-23).
- **Ürün döngüsünün ilk adımı canlıda (2026-10-25):** Faz 15 ile "kitaplığına ekle" tamam — LIB-01 ve
  LIB-02, SOC-02'nin varsayılan gizlilik yarısı. 17 Ekim'deki durum: 34 maddenin 7'si kısmen vardı, 27'si
  yoktu ([FEATURE_REQUESTS § Durum özeti](FEATURE_REQUESTS.md#durum-özeti-17-ekim-2026)).
- **Kubernetes öğrenimi ertelendi (kullanıcı kararı, 2026-11-09):** sıra önce özelliklerde (Faz 16).
  Manifest'ler ve kind kurulumu yerinde; dönüldüğünde ilk adım `make k8s-up` (Windows düzeltmesi
  [#67](https://github.com/mtgezgin/BookHub/issues/67)).

## Ürün hattı

| Sıra | Faz | Kapsam (backlog kimlikleri) | Neden bu sırada | Durum |
| --- | --- | --- | --- | --- |
| ✅ | 15 — Kişisel kitaplık | Paket A: LIB-01, LIB-02, CAT-01'in kitaplık bölümü, SOC-02'nin "varsayılan gizli" kuralı; UX-01/02 kabul koşulu | En büyük ürün boşluğu: kullanıcının geri döneceği alan. Baskılara bağımlı değil (kitaplık kaydı kullanıcı–eser ilişkisi). | [canlıda](phases/phase-15-personal-library.md) (2026-10-25; [#42](https://github.com/mtgezgin/BookHub/issues/42), PR [#43](https://github.com/mtgezgin/BookHub/pull/43)) |
| ✅ | 16 — Eser ve baskı | CAT-02 ([#36](https://github.com/mtgezgin/BookHub/issues/36)) | Okuma kaydının sayfa toplamı baskıdan gelecek (READ-01'in bağımlılığı). | [canlıda](phases/phase-16-works-editions.md) (2026-11-15; PR [#73](https://github.com/mtgezgin/BookHub/pull/73), [#74](https://github.com/mtgezgin/BookHub/pull/74)) |
| ✅ | 16b — Arama | [DISC-01](FEATURE_REQUESTS.md)'in arama yarısı: Türkçe duyarlı, index'li arama; yazarken arama kuralları | Canlıda arama Türkçe başlıkları bulamıyordu ("ince memed" sıfır sonuç) ve yazarken arama istek sayısını artırıyordu. | [canlıda](phases/phase-16b-search.md) (2026-11-18; [#82](https://github.com/mtgezgin/BookHub/issues/82), PR [#89](https://github.com/mtgezgin/BookHub/pull/89)) |
| ✅ | 16c — Katalog sorguları | REVIEW_FEEDBACK F24–F26: tür süzgeci, sayım, derin sayfa | "Yüksek trafik ve çok kayıt varmış gibi" (kullanıcı kararı, 2026-11-18); 500 bin kitapta ölçüldü. | [canlıda](phases/phase-16c-catalog-queries.md) (2026-11-19; [#98](https://github.com/mtgezgin/BookHub/issues/98), PR [#102](https://github.com/mtgezgin/BookHub/pull/102)) |
| ✅ | 16d — Arayüz veri katmanı | REVIEW_FEEDBACK F33: sunucu verisi TanStack Query ile | Faz 17 yeni yazma işlemleri getiriyor; yazmadan sonra hangi verinin tazeleneceği her sayfada elleydi. Kullanıcı her adımı okuyarak ilerledi. | [canlıda](phases/phase-16d-server-state.md) (2026-11-22; [#105](https://github.com/mtgezgin/BookHub/issues/105), PR [#112](https://github.com/mtgezgin/BookHub/pull/112)) |
| Şimdi | 17 — Okuma takibi ve inceleme | Paket B: READ-01, READ-02, REV-01, REV-02 | Döngüyü kapatır. İnceleme geniş kullanıma açılmadan MOD-04 gerekir; demo hedefinde Faz 18'e kalır. | [#80](https://github.com/mtgezgin/BookHub/issues/80), [spec](phases/phase-17-reading-reviews.md) onaylandı, koda göre gözden geçirildi (2026-11-23); 17A canlıda (PR #121 ve #123, 2026-11-25); sırada 17B |
| Sonra (1) | Okuma özeti ve kişisel ana sayfa | STATS-01; SOC-01'in kişisel ana sayfa yarısı ("Şu an okuyorum", hızlı ilerleme) | Faz 17'nin `reading_log`'u tam bu iş için kuruluyor; biriken veri kullanıcıya geri gösterilmezse kayıt tutmak angarya olur. Demo döngüsü "okudum → özetimi gördüm" diye kapanır. Yeni tablo gerekmez. | — |
| Sonra (2) | Yazarın diğer kitapları | DISC-05 | Küçük; keşfin en doğal yolu. `search_fold` ve 16c'nin ölçüm yöntemi hazır. | — |
| Sonra (3) | Bağlantı önizlemesi | SOC-04 | Paylaşılan kitap bağlantısı bugün boş kart gösteriyor (tek sayfa uygulaması, `og:` etiketi yok). NGINX, API ve kaçışlama üzerine iyi bir öğrenme işi. | — |
| Sonra (4) | Notlar ve alıntılar | READ-04 | Kişiye özel, moderasyon gerektirmez; kitaplığa bağlılığı artırır. | — |
| Gerçek kullanıcı kararıyla | 18 — Katkı döngüsü ve hesap | Paket C: MOD-01, MOD-02, ACC-01–03, DATA-02; kullanıcının baskı önermesi ([#79](https://github.com/mtgezgin/BookHub/issues/79)) | Gerçek kullanıcının kapısı; bir e-posta sağlayıcısı gerektirir. Demo hedefinde yukarıdakilerden sonra. | — |
| Gerçek kullanıcı kararıyla | — | CAT-04 (seriler), DATA-03 (içe aktarma), UX-03 (kurulabilir site) | Gerçek kullanıcı kazanmaya dönük (içe aktarma bir numaralı yol); demoda değeri sınırlı. | — |
| Daha sonra | — | Paket D'nin kalanı (DISC-01–03, READ-03, REV-03) · Paket E (sosyal) | Veri ve kullanım oluştukça. | — |

Faz 16, Paket A'nın "CAT-02 kimlik kararı" maddesini zaten karşılıyor: tasarım yapıldı, uygulaması
okuma takibinden hemen önceye kondu.

## Teknik hat

Ürün hedefini değiştirmez; küçük ve ayrı PR'lar.

| İş | Ne zaman | Not |
| --- | --- | --- |
| Node 26 LTS ([#71](https://github.com/mtgezgin/BookHub/issues/71); Dependabot #13, #46) | 2026-11-11 (LTS: 2026-10-28) | Derleme imajı, CI ve `@types/node` birlikte; derleme aşaması son imaja girmez |
| Canlı imajlara haftalık güvenlik taraması (Trivy; [#48](https://github.com/mtgezgin/BookHub/issues/48)) | 2026-11 | `scan.yml`: pazartesi 06:00 UTC ve elle, `release` üzerinde; iki imaj her zaman taranır (ci.yml'de de). #47'nin açıkları canlıda 6 gün fark edilmemişti; canlı izlemenin ilk parçası |
| Canlı izleme ve alarm (sağlık kontrolü, uptime, hata izleme) | Uptime 2026-11 ([#63](https://github.com/mtgezgin/BookHub/issues/63)); Northflank sağlık kontrolleri panelde yapılacak (adımlar verildi, 2026-11-19); hata izleme zamanı açık, gerçek kullanıcı kararından önce şart | `uptime.yml`: günde bir üç uç (NGINX, API, Postgres). [PROCESS §7](PROCESS.md) · [DEPLOYMENT §5](DEPLOYMENT.md) |
| Salt okunur MCP sunucusu ([#52](https://github.com/mtgezgin/BookHub/issues/52)) | 2026-11-11 (PR #53) | `backend/cmd/mcp`: kataloğu bir modele altı okuma aracıyla açar; API istemcisi, imaja girmez. Yazma araçları ayrı karar |
| Gözlemlenebilirlik: OpenTelemetry, Prometheus, Grafana ([#87](https://github.com/mtgezgin/BookHub/issues/87), [#96](https://github.com/mtgezgin/BookHub/issues/96)) | 1. adım canlıda kodda (2026-11-18, PR #94; telemetri adres verilmedikçe kapalı). Canlıya açmak #96: hazırlık PR #97'de, kullanıcının Grafana Cloud hesabı ve birlikte yürüyüş bekliyor | `make obs-up`: Grafana :3002, Prometheus :9090. İzler ve metrikler OTLP ile; adres boşsa kapalı. Canlıda collector çalıştırılamaz (iki servis sınırı): API doğrudan Grafana Cloud'a gönderir. Sıra: yerel → metrik/log → canlı → pano ve alarm |
| `PROD_DATABASE_URL`'i bir GitHub Environment'a taşımak | Zamanı açık | Repo public olunca Environments ücretsiz: sır yalnız `main`'den koşan Deploy'a açılabilir ([DEPLOYMENT §2](DEPLOYMENT.md)) |
| NGINX erişim logunda sorgu dizesi ve referrer | Faz 16b ile (canlıda 2026-11-18) | `log_format bookhub` ([default.conf](../frontend/nginx/default.conf)); kural `scripts/checks.sh`'ta |
| REVIEW_FEEDBACK bulguları | İlgili fazın içinde | F10, F11 → Faz 15–17 (UX-01) · F13 → her fazın UX-02 kabulü · F12, F15 → Faz 18 (MOD-02) · F04 → Faz 18'den önce · F03, F06–F09, F16, F17 → ihtiyaç oldukça ayrı iş. İkinci inceleme (2026-11-17): F20–F40; önerilen sıra [REVIEW_FEEDBACK](REVIEW_FEEDBACK.md#ikinci-incelemenin-önerdiği-sıra)'te |
| Canlı veritabanının şifreli yedeği ([#100](https://github.com/mtgezgin/BookHub/issues/100), F23) | Canlıda 2026-11-23 (PR #101) | Haftalık ve her migration'dan önce `age` ile şifreli döküm; CI her PR'da döküm → geri yükleme turunu sınar. İlk döküm ve yerelde geri yükleme provası 2026-11-23; zamanlanmış ilk koşu 2026-11-30, migration öncesi döküm Faz 17'nin `0016`'sında sınanır |
| Fonksiyonlarda sabit `search_path` (Supabase Security Advisor, 7 WARN) | Zamanı açık | `search_fold`, `search_tsquery` ve beş tetikleyici fonksiyonu. Risk düşük (tek uygulama kullanıcısı, Data API kapalı). Tek satır değil: `search_fold` index ifadelerinde; SQL fonksiyonuna `SET search_path` eklemek inlining'i kapatır, Faz 16b'nin arama planları önce ve sonra ölçülmeli |
| CI: zorunlu kontroller, bağımlılık incelemesi, süre sınırları, action SHA kuralı ([#108](https://github.com/mtgezgin/BookHub/issues/108)) | 2026-11-19 (PR #99, #109) | `main`'de 8 zorunlu kontrol (Trivy ve SonarCloud dahil); Dependabot güvenlik uyarıları açık. Kalan öğrenme adımları: CodeQL, SonarCloud'da test kapsamı, "bir kez derle, aynısını yayınla" (F39) |
| Web imajında süreli Trivy istisnası ([#106](https://github.com/mtgezgin/BookHub/issues/106)) | 2026-12-19'a kadar | nginx 1.30.6'nın 6 açığının koşulu bizde yok; düzeltilmiş temel imaj gelince istisna silinir. Süre dolarsa tarama yeniden kırmızı olur (bilerek) |
| Dependabot, Sonar ve güvenlik bulguları | Geldikçe | Ürün fazını durdurmaz; güvenlik engelleyicisi hariç |

## Kurallar

Bu kurallar rastgeleliği önlemek için var:

1. **Yeni fikir önce backlog'a.** Ürün fikri FEATURE_REQUESTS'e ("Önerildi"), teknik bulgu
   REVIEW_FEEDBACK'e yazılır; doğrudan issue açılmaz.
2. **Issue yalnız "Şimdi" fazı için açılır** ve backlog kimliğini taşır (ör. "LIB-01: …"). Teknik hattaki
   işler de issue'yla başlar ve bu tabloda yeri vardır.
3. **Aynı anda tek ürün fazı.** Teknik işler araya girebilir ama küçük kalır.
4. **Faz bitince:** bu dosyada "Şimdi" ilerler; FEATURE_REQUESTS'te maddelerin durumu işaretlenir;
   STATUS güncellenir.
5. **Sıra değişikliği bilinçli bir karardır:** tarihiyle aşağıdaki "Kararlar"a yazılır.

## Kararlar

- **2026-10-17:**
  - Hedef "üretim kalitesinde demo" olarak belirlendi.
  - Sıra Paket A → baskılar → Paket B oldu (kullanıcı kararı).
  - #36'nın tasarımı yapıldı; uygulaması Faz 16'ya kaydı. Faz numaraları buna göre: 15 kişisel
    kitaplık, 16 eser ve baskı, 17 okuma takibi ve inceleme.
- **2026-11-17:**
  - Arama (DISC-01'in arama yarısı) Faz 17'den önce, **Faz 16b** olarak yapıldı (kullanıcı kararı). Neden: canlıda arama Türkçe başlıkları bulamıyor ve yazarken arama isteniyor.
    Faz numaraları değişmedi; Faz 17 "Şimdi" olarak kalıyor.
  - Migration `0014` aramaya verildi; Faz 17'nin migration'ları `0015`–`0017` oldu (goose sırasız
    migration'ı reddeder; [spec](phases/phase-17-reading-reviews.md) güncellendi).
  - İkinci kapsamlı inceleme yapıldı: bulgular [REVIEW_FEEDBACK](REVIEW_FEEDBACK.md) F20–F40.
  - Gözlemlenebilirlik (OpenTelemetry, Prometheus, Grafana) teknik hatta eklendi (kullanıcı kararı; öğrenme
    hedefi). Ücretsiz sınırlar resmi sayfalardan doğrulandı; ilk adım yalnız yerel ([#87](https://github.com/mtgezgin/BookHub/issues/87)).
  - İnceleme düzeltmeleri Faz 17'den önce, partiler hâlinde yapılıyor (kullanıcı kararı): A (#83), B (#84),
    index'ler (#85). Migration `0015` index düzeltmelerine verildi; Faz 17'nin migration'ları `0016`–`0018` oldu.
- **2026-11-18 (kullanıcı):**
  - Yedek: otomatik şifreli döküm, haftalık ve her migration öncesi ([#100](https://github.com/mtgezgin/BookHub/issues/100)).
  - Veritabanı ve Redis Frankfurt'ta kalır; bölge taşıması (ve ücretli plana geçiş) gerçek kullanıcılar
    gelince (F20).
  - Faz 16c'nin okuma yolu da yapılır: "yüksek trafik ve çok kayıt varmış gibi" ilerlenir
    ([#98](https://github.com/mtgezgin/BookHub/issues/98)). Saklanan puan skoru ertelendi (F26).
  - "En sevilenler" önbelleği puan yazımında silinmez, 60 sn'de bir yenilenir; moderasyon silmeye devam eder.
  - SonarCloud zorunlu kontrol olur; birleşen dallar otomatik silinir.
- **2026-11-19 (kullanıcı):**
  - Arayüz veri katmanı (F33) Faz 17'den önce, **TanStack Query** ile (Faz 16d). Kod asistandan; kullanıcı
    her adımı commit'ten önce editörde okur.
  - Kapak (F37) için A: önerilerde kapak adresi kabul edilmez (#104). İzin listesi ve CSP daraltması açık.
  - Trivy de zorunlu kontrol olur. Aynı gün nginx açıkları için süreli istisna kabul edildi (#106, #107).
  - CI iyileştirmeleri (bağımlılık incelemesi, süre sınırları, eski koşu iptali, SHA kuralı) yapıldı (#108).
- **2026-11-23 (kullanıcı):**
  - Faz 17 spec'i koda göre gözden geçirildi. 409 cevabı standart hata zarfında kalır (istemci rafı
    yeniden okur); Kitaplığım kartlarında 17A-1'de yalnız ilerleme, kart menüsü sonra. Kullanıcı
    "projeye en uygun olan" dedi; öneriler asistandan ([spec](phases/phase-17-reading-reviews.md) § Kararlar).
  - Altı yeni backlog maddesi (READ-04, CAT-04, DISC-05, SOC-04, DATA-03, UX-03) ve Faz 17'den sonraki
    sıra: okuma özeti ve kişisel ana sayfa → DISC-05 → SOC-04 → READ-04; Paket C ile CAT-04, DATA-03,
    UX-03 gerçek kullanıcı kararıyla. Öneri asistandan, kullanıcı "hepsini istiyorum" dedi.
  - Faz numarası iş "Şimdi" olunca verilir; Faz 18 adı Paket C'de kalır (dokümanlardaki "MOD-04 Faz
    18'de" gibi atıflar değişmesin). Araya girenler numarasını başlarken alır.
