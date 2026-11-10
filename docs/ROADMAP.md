# BookHub — Yol haritası (ROADMAP)

> Son güncelleme: **2026-10-25**. Bu dosya tek bir soruyu cevaplar: **hangi sırayla ve neden?**
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

## Ürün hattı

| Sıra | Faz | Kapsam (backlog kimlikleri) | Neden bu sırada | Durum |
| --- | --- | --- | --- | --- |
| ✅ | 15 — Kişisel kitaplık | Paket A: LIB-01, LIB-02, CAT-01'in kitaplık bölümü, SOC-02'nin "varsayılan gizli" kuralı; UX-01/02 kabul koşulu | En büyük ürün boşluğu: kullanıcının geri döneceği alan. Baskılara bağımlı değil (kitaplık kaydı kullanıcı–eser ilişkisi). | [canlıda](phases/phase-15-personal-library.md) (2026-10-25; [#42](https://github.com/mtgezgin/BookHub/issues/42), PR [#43](https://github.com/mtgezgin/BookHub/pull/43)) |
| **Şimdi** | 16 — Eser ve baskı | CAT-02 ([#36](https://github.com/mtgezgin/BookHub/issues/36)) | Okuma kaydının sayfa toplamı baskıdan gelecek (READ-01'in bağımlılığı). | [spec](phases/phase-16-works-editions.md) incelemede |
| Sonra | 17 — Okuma takibi ve inceleme | Paket B: READ-01, READ-02, REV-01, REV-02 | Döngüyü kapatır. İnceleme geniş kullanıma açılmadan MOD-04 gerekir; demo hedefinde Faz 18'e kalır. | — |
| Daha sonra | 18 — Katkı döngüsü ve hesap | Paket C: MOD-01, MOD-02, ACC-01–03, DATA-02 | Gerçek kullanıcının kapısı; bir e-posta sağlayıcısı gerektirir. | — |
| Daha sonra | — | Paket D (keşif: DISC-01–03, STATS-01, READ-03, REV-03) · Paket E (sosyal) | Veri ve kullanım oluştukça. | — |

Faz 16, Paket A'nın "CAT-02 kimlik kararı" maddesini zaten karşılıyor: tasarım yapıldı, uygulaması
okuma takibinden hemen önceye kondu.

## Teknik hat

Ürün hedefini değiştirmez; küçük ve ayrı PR'lar.

| İş | Ne zaman | Not |
| --- | --- | --- |
| Node 26 (Dependabot #13, #29) | Ekim sonu, LTS çıkınca | |
| Canlı imajlara haftalık güvenlik taraması (Trivy; [#48](https://github.com/mtgezgin/BookHub/issues/48)) | 2026-11 | `scan.yml`: pazartesi 06:00 UTC ve elle, `release` üzerinde; iki imaj her zaman taranır (ci.yml'de de). #47'nin açıkları canlıda 6 gün fark edilmemişti; canlı izlemenin ilk parçası |
| Canlı izleme ve alarm (sağlık kontrolü, uptime, hata izleme) | Uptime 2026-11 ([#63](https://github.com/mtgezgin/BookHub/issues/63)); hata izleme zamanı açık, gerçek kullanıcı kararından önce şart | `uptime.yml`: günde bir üç uç (NGINX, API, Postgres). [PROCESS §7](PROCESS.md) · [DEPLOYMENT §5](DEPLOYMENT.md) |
| NGINX erişim logunda sorgu dizesi ve referrer | İzleme işiyle birlikte | [CLAUDE.md](../CLAUDE.md) "bilinen açık" |
| REVIEW_FEEDBACK bulguları | İlgili fazın içinde | F10, F11 → Faz 15–17 (UX-01) · F13 → her fazın UX-02 kabulü · F12, F15 → Faz 18 (MOD-02) · F04 → Faz 18'den önce · F03, F06–F09, F16, F17 → ihtiyaç oldukça ayrı iş |
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
