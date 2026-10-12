# BookHub — proje kuralları

Bu dosya her Claude oturumunda ve AI review kapısında okunur. Kısa tutulur; ayrıntılar
bağlantılardaki dokümanlarda. Bir kural bir olaydan ya da incelemeden doğduysa yanına kaynağı
yazılır — kuralın "neden"i silinmesin.

## Proje

- Öğrenme projesi: Go API (chi, pgx, sqlc, goose, slog) + React/Vite + NGINX; Postgres + Redis.
- Canlı: Northflank'ta iki servis — `web` (NGINX + React, public) ve `api` (Go, **private**,
  port 8080). Veri: Supabase Postgres (session pooler) ve Upstash Redis. Ayrıntı:
  [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md).
- Önce okunacaklar: [STATUS](docs/STATUS.md) (neredeyiz), [PROJECT_MAP](docs/PROJECT_MAP.md)
  (neyin ne olduğu), [GO_CONVENTIONS](docs/GO_CONVENTIONS.md) (nasıl yazıyoruz),
  [PROCESS](docs/PROCESS.md) (bir iş nasıl ilerliyor).

## Kullanıcıyla çalışma biçimi

- Kullanıcı öğreniyor: önce kavram ("hangi problemi çözüyor"), sonra küçük deney, en son
  üretim kodu — [LEARNING_PLAN §2](docs/LEARNING_PLAN.md).
- Üretim kodunu asistan yazabilir; kullanıcı okuyup onaylamadan **commit edilmez**, kullanıcı
  istemeden **push edilmez**.
- Uygulama kodu için çoklu ajan iş akışı kullanılmaz; inceleme ve araştırma için serbest.
- Kullanıcı bir kavram sorduğunda açıklamadan sonra `/not` ile öğrenme notuna eklemeyi öner.
- Sırlar asistana verilmez: canlı değerleri kullanıcı Keychain'den doğrudan panele/secret'a
  aktarır (`security find-generic-password … | pbcopy` ya da `| gh secret set …`).
- Commit kimliği kişisel hesaptır (repo-yerel `user.email`); iş hesabı kullanılmaz.

## Kod kuralları

- Identifier'lar İngilizce; yorum, log ve hata mesajları, test senaryosu adları Türkçe
  ([GO_CONVENTIONS §9](docs/GO_CONVENTIONS.md)).
- Katmanlar: handler → service → repository. Service yalnız gerçek iş mantığı olan yerde var
  (auth, moderasyon, puan); düz CRUD'a boş bir service katmanı eklenmez (§4).
- SQL yalnız sqlc ile (`backend/sql/queries` → `internal/db`); ORM yok, sorgu metni string
  birleştirmeyle kurulmaz (§6).
- Config yalnız env'den (12-factor), açılışta doğrulanır; dotenv paketi yok. Yeni değişken:
  `internal/config` + `.env.example` + DEPLOYMENT.md'deki tablo, üçü birlikte.
- Hata zarfı her yerde `{"error":{"code","message"}}`; ham hata istemciye gitmez.

## Güvenlik — review'da engelleyici

- Sır commit'lenmez. `.env` git dışı; `.env.example`'da yalnız herkese açık dev değerleri.
- Refresh çerezi `HttpOnly` + `Secure` + `SameSite=Strict` + `Path=/api/auth`; gevşetilmez.
- CSRF koruması (`http.CrossOriginProtection`) kaldırılmaz ya da atlanmaz. NGINX, API'ye Host'u
  portuyla iletir: `proxy_set_header Host $api_host;` (`$api_host` bir map; varsayılanı
  `$http_host`, yalnız Host hiç gelmezse `$host` — default.conf). Varsayılan dal `$host` yapılmaz.
- `X-Real-IP`'ye yalnız `TRUSTED_PROXIES`'ten gelen bağlantıda güvenilir.
- Loglar: parola, token, çerez, istek gövdesi ve e-posta hiçbir katmanda loglanmaz. Uygulama
  logu (`requestLogger`) yolu **sorgusuz** yazar. Ziyaretçi IP'si kötüye kullanım incelemesi
  için bilerek loglanır; canlıda yalnız NGINX logunun `X-Forwarded-For` alanında durur (API'nin
  `remote`'u orada NGINX'in iç adresi, çünkü `TRUSTED_PROXIES` boş —
  [DEPLOYMENT §5](docs/DEPLOYMENT.md)). **Bilinen açık:** NGINX'in varsayılan erişim logu sorgu
  dizesini ve referrer'ı yazıyor; özel bir `log_format` gelene kadar açık
  ([öğrenme notu #11](docs/learning-notes/infra-and-devops.md)).
- İstek gövdeleri ayrı DTO + `DisallowUnknownFields` ile okunur (mass assignment yok); kaynak
  sahipliği servis ya da SQL katmanında kontrol edilir (IDOR yok).
- bcrypt cost ≥ 12; JWT algoritması sabit (HS256), `iss`/`aud`/`exp` doğrulanır.
- CI/CD: `secrets.PROD_DATABASE_URL` yalnız deploy.yml'in migration adımında kullanılır; başka
  bir iş akışına ya da işe eklenmez. deploy.yml'in `if:` koşulu (başarılı + push + main + ci.yml +
  aynı depo) ve `permissions`'ı gevşetilmez; `pull_request_target` eklenmez; olay verisi `run:` içine
  `${{ }}` ile yazılmaz. CI işlerini atlatan bir değişiklik (`if: false`, `continue-on-error`)
  engelleyicidir.

## Veritabanı ve migration

- Migration'lar `backend/migrations` altında (goose, binary'ye gömülü). Canlıda
  `AUTO_MIGRATE=false`.
- Canlıya migration'ı yalnız Deploy iş akışı ([deploy.yml](.github/workflows/deploy.yml))
  uygular, yeni kod çıkmadan **önce**. Elle `manage migrate` yalnız acil durumda ve yalnız
  `main`'e girmiş, CI'ı yeşil bir commit'ten (normalde `origin/release`) çalıştırılır;
  merge edilmemiş bir daldan asla.
- Her migration **o an canlıda çalışan kodla geriye uyumlu** olmalı: önce genişlet → yayınla →
  sonra daralt. Daraltma (kolon/tablo silme, yeniden adlandırma) ayrı bir PR'dır ve ancak önceki
  genişletme yayını canlıda doğrulandıktan sonra merge edilir. Uyumsuz migration engelleyicidir.
- Canlıya çıkmış bir migration dosyası **değiştirilmez, silinmez, numarası yeniden
  kullanılmaz**; düzeltme yeni bir migration'dır. Neden: goose sürümü numarasıyla takip eder ve
  uygulanmış dosyanın yeni hâlini canlıya hiç uygulamaz (deploy.yml bu durumda durur).
- Repoya `supabase/` klasörü eklenmez: Supabase'in GitHub entegrasyonu açık ve o klasörü
  canlı veritabanına uygulayabilir. Şemanın tek kaynağı goose migration'larıdır.
- **Kabul edilmiş risk:** canlı API Supabase'e sahip (owner) kullanıcıyla bağlanıyor; ayrı,
  kısıtlı bir kullanıcı açılmayacak (kullanıcı kararı, 2026-10-05 —
  [REVIEW_FEEDBACK F05](docs/REVIEW_FEEDBACK.md)). Review'da yeniden engelleyici açılmaz.

## Altyapı

- `frontend/nginx/api-proxy.conf`'ta `proxy_pass http://api:8080;` sonuna `/` eklenmez: dosya
  regex bir location'da da include edildiği için NGINX hiç açılmaz. Northflank'ta API servisinin
  adı tam olarak `api`.
- `make smoke` ve `make e2e` veritabanına test verisi bırakır; **canlıya karşı çalıştırılmaz**.
- Lokal: `make app-up` (http://localhost:3000), kapatmak `make down`. Testler: `make test`,
  `make test-db`, `make web-test`, `make e2e`.

## Süreç

Ayrıntı: [docs/PROCESS.md](docs/PROCESS.md).

- Her iş bir issue ile başlar (şablon: problem; Ready'den önce kapsam içi/dışı ve numaralı
  kabul kriterleri).
- Issue açarken: `gh issue create` YAML issue form'larını görmez (`-T` "no templates found"
  verir; cli/cli#5865). Kullanıcı açacaksa
  `https://github.com/mtgezgin/BookHub/issues/new?template=feature.yml` (ya da `bug.yml` /
  `incident.yml`) bağlantısını ver. Asistan açacaksa gövdeyi şablondaki başlıklarla yaz
  (`### Bağlam / problem`, `### Kapsam içi`, `### Kapsam dışı`, `### Kabul kriterleri` …) ve
  şablonun etiketlerini elle ekle: özellik `--label feature --label <P0|P1|P2>`, bug
  `--label bug --label needs-triage`, olay `--label incident`.
- Dal → PR (şablon) → CI yeşil → `main`. PR issue'ya `Refs #…` ile bağlanır; `Closes/Fixes`
  kullanılmaz (issue merge anında kapanırdı). Issue, yayından sonraki doğrulamayla elle kapanır.
- `main`'de CI yeşil bitince Deploy iş akışı (gerekirse migration uygular ve) `release` dalını
  o commit'e ilerletir; Northflank yalnız `release`'i yayınlar. `release`'e elle push edilmez.
- Commit mesajlarında `[skip ci]`, `[ci skip]`, `[no ci]`, `[skip nf]` gibi atlama bayrakları
  kullanılmaz: Northflank, aynı push'taki tek bir bayraklı commit yüzünden bütün yayını atlar.
- Makinenin denetleyebildiği kurallar `scripts/checks.sh`'ta (git kancaları + CI'da **Kurallar** işi,
  `rules.yml`). Bu dosyaya denetlenebilir bir kural eklenince betiğe ve `scripts/checks_test.sh`'a da
  eklenir; Kurallar kırmızıyken PR birleştirilmez.
- Yayından sonra doğrulama yapılır; doğrulanamayan her şey PR'da ya da issue'da açıkça yazılır.
- Canlıdaki bir olay "önleme kuralı" bulunmadan kapanmaz: bir test, bir alarm ya da bu dosyaya
  eklenen bir kural.
