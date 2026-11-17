# BookHub — Yol haritası (ROADMAP)

> Son güncelleme: **2026-11-17**. Bu dosya tek bir soruyu cevaplar: **hangi sırayla ve neden?**
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

- **Platform hazır:** Faz 1–14 bitti — API, veritabanı, auth, moderasyon, puan, önbellek, Docker,
  React, CI/CD, NGINX, Kubernetes, oturum sertleştirme, arayüz ([STATUS](STATUS.md)).
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
| Onay bekliyor | 16b — Arama | [DISC-01](FEATURE_REQUESTS.md)'in arama yarısı: Türkçe duyarlı, index'li arama; yazarken arama kuralları | Canlıda arama Türkçe başlıkları bulamıyor ("ince memed" sıfır sonuç) ve yazarken arama istek sayısını artırıyor. Faz 17'den bağımsız; şeması yalnız genişletme. | [#82](https://github.com/mtgezgin/BookHub/issues/82), [spec](phases/phase-16b-search.md); kod yazıldı ve yerelde doğrulandı (2026-11-17), yayın onay bekliyor |
| **Şimdi** | 17 — Okuma takibi ve inceleme | Paket B: READ-01, READ-02, REV-01, REV-02 | Döngüyü kapatır. İnceleme geniş kullanıma açılmadan MOD-04 gerekir; demo hedefinde Faz 18'e kalır. | [#80](https://github.com/mtgezgin/BookHub/issues/80), [spec](phases/phase-17-reading-reviews.md) onaylandı |
| Sonra | 18 — Katkı döngüsü ve hesap | Paket C: MOD-01, MOD-02, ACC-01–03, DATA-02; kullanıcının baskı önermesi ([#79](https://github.com/mtgezgin/BookHub/issues/79)) | Gerçek kullanıcının kapısı; bir e-posta sağlayıcısı gerektirir. | — |
| Daha sonra | — | Paket D (keşif: DISC-01–03, STATS-01, READ-03, REV-03) · Paket E (sosyal) | Veri ve kullanım oluştukça. | — |

Faz 16, Paket A'nın "CAT-02 kimlik kararı" maddesini zaten karşılıyor: tasarım yapıldı, uygulaması
okuma takibinden hemen önceye kondu.

## Teknik hat

Ürün hedefini değiştirmez; küçük ve ayrı PR'lar.

| İş | Ne zaman | Not |
| --- | --- | --- |
| Node 26 LTS ([#71](https://github.com/mtgezgin/BookHub/issues/71); Dependabot #13, #46) | 2026-11-11 (LTS: 2026-10-28) | Derleme imajı, CI ve `@types/node` birlikte; derleme aşaması son imaja girmez |
| Canlı imajlara haftalık güvenlik taraması (Trivy; [#48](https://github.com/mtgezgin/BookHub/issues/48)) | 2026-11 | `scan.yml`: pazartesi 06:00 UTC ve elle, `release` üzerinde; iki imaj her zaman taranır (ci.yml'de de). #47'nin açıkları canlıda 6 gün fark edilmemişti; canlı izlemenin ilk parçası |
| Canlı izleme ve alarm (sağlık kontrolü, uptime, hata izleme) | Uptime 2026-11 ([#63](https://github.com/mtgezgin/BookHub/issues/63)); hata izleme zamanı açık, gerçek kullanıcı kararından önce şart | `uptime.yml`: günde bir üç uç (NGINX, API, Postgres). [PROCESS §7](PROCESS.md) · [DEPLOYMENT §5](DEPLOYMENT.md) |
| Salt okunur MCP sunucusu ([#52](https://github.com/mtgezgin/BookHub/issues/52)) | 2026-11-11 (PR #53) | `backend/cmd/mcp`: kataloğu bir modele altı okuma aracıyla açar; API istemcisi, imaja girmez. Yazma araçları ayrı karar |
| `PROD_DATABASE_URL`'i bir GitHub Environment'a taşımak | Zamanı açık | Repo public olunca Environments ücretsiz: sır yalnız `main`'den koşan Deploy'a açılabilir ([DEPLOYMENT §2](DEPLOYMENT.md)) |
| NGINX erişim logunda sorgu dizesi ve referrer | Faz 16b ile (yayın bekliyor) | `log_format bookhub` ([default.conf](../frontend/nginx/default.conf)); kural `scripts/checks.sh`'ta |
| REVIEW_FEEDBACK bulguları | İlgili fazın içinde | F10, F11 → Faz 15–17 (UX-01) · F13 → her fazın UX-02 kabulü · F12, F15 → Faz 18 (MOD-02) · F04 → Faz 18'den önce · F03, F06–F09, F16, F17 → ihtiyaç oldukça ayrı iş. İkinci inceleme (2026-11-17): F20–F40; önerilen sıra [REVIEW_FEEDBACK](REVIEW_FEEDBACK.md#ikinci-incelemenin-önerdiği-sıra)'te |
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
  - İnceleme düzeltmeleri Faz 17'den önce, partiler hâlinde yapılıyor (kullanıcı kararı): A (#83), B (#84),
    index'ler (#85). Migration `0015` index düzeltmelerine verildi; Faz 17'nin migration'ları `0016`–`0018` oldu.
