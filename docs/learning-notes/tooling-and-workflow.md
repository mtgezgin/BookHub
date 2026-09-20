# Öğrenme Notları — Tooling & Workflow

Geliştirme aracı kavramları: Claude Code (skill / memory / hook), git, pre-commit gate'ler,
commit ritmi. Format: [README.md](README.md).

## İçindekiler

1. [Memory mi, skill mi, hook mu? (kalıcılık & tetiklenme)](#1-memory-mi-skill-mi-hook-mu-kalıcılık--tetiklenme)
2. [Proje-içi skill nasıl yapılır + restart + otomatik çağrılır mı?](#2-proje-içi-skill-nasıl-yapılır--restart--otomatik-çağrılır-mı)
3. [Bir commit'te neden birden çok "gate" çalışıyor?](#3-bir-committe-neden-birden-çok-gate-çalışıyor)
4. [gofmt vs golangci-lint (Prettier vs ESLint)](#4-gofmt-vs-golangci-lint-prettier-vs-eslint)
5. [pre-commit vs pre-push: kontrolleri maliyete göre katmanlama](#5-pre-commit-vs-pre-push-kontrolleri-maliyete-göre-katmanlama)
6. [VS Code eklentisi aktivasyonu ve "command not found" yarışı](#6-vs-code-eklentisi-aktivasyonu-ve-command-not-found-yarışı)
7. [Playwright ile uçtan uca test: gerçek tarayıcı, rate limit bütçesi ve storageState tuzağı](#7-playwright-ile-uçtan-uca-test-gerçek-tarayıcı-rate-limit-bütçesi-ve-storagestate-tuzağı)

---

## 1. Memory mi, skill mi, hook mu? (kalıcılık & tetiklenme)

**🗓️ Bağlam:** `docs/learning-notes`'u oluşturup bir de "bunu hatırla" diye bir **memory** notu
yazmıştım. "Bunun için bir skill yazmaya değer mi, memory'den bu bilgi silinebilir mi?" diye
sordum — çünkü memory'nin kalıcılığından emin değildim.

**⚡ Kısa cevap:** Memory **silinebilir** ve tavsiye niteliğindedir. Tekrar eden, değer üreten
bir davranışı **skill** olarak yazmak daha kalıcıdır (özellikle repo içine koyarsan). Hook ise
otomatiktir ama düz komut çalıştırır; "konuşmayı özetleyip not al" gibi muhakeme isteyen işe
uymaz.

**📖 Açıklama:**

| | Ne yapar | Dayanıklılık | Tetiklenme |
|---|---|---|---|
| **Memory** | "Şunu hatırla" — yumuşak tercih | Silinebilir, repo'da değil, makineye bağlı | Claude uygun görünce (yargı) |
| **Skill** | Çağrılınca tanımlı işi yapar | **Kalıcı dosya**; repo'daki `.claude/skills/`'e koyarsan **git'e girer, version'lanır** | Sen `/komut` yazarsın **veya** Claude proaktif çağırır |
| **Hook** | Olay olunca **deterministik komut** | Kalıcı (settings.json) | Otomatik (ama LLM muhakemesi yapamaz) |

Kritik: bir işi **garanti her seferinde otomatik** yaptırmak ancak hook ile olur; ama hook düz
kabuk komutu çalıştırır. "Son Q&A'i özetleyip dosyaya ekle" Claude'un muhakemesini gerektirdiği
için **saf hook'a uymaz** → bu iş için **skill** (açık çağrı) doğru araç.

**🔗 İlgili:**
- [.claude/skills/not/SKILL.md](../../.claude/skills/not/SKILL.md) (bu öğrenmenin sonucu: `/not` skill'i)
- Aşağıda [Girdi 2](#2-proje-içi-skill-nasıl-yapılır--restart--otomatik-çağrılır-mı)

---

## 2. Proje-içi skill nasıl yapılır + restart + otomatik çağrılır mı?

**🗓️ Bağlam:** `/not` skill'ini **sadece bu projede** istediğimi söyledim ve nasıl yapılacağını,
restart gerekip gerekmediğini, yeni konuşmada Claude'un otomatik çağırıp çağıramayacağını sordum.

**⚡ Kısa cevap:** Skill'i repo içindeki **`.claude/skills/<ad>/SKILL.md`**'e koyarsan **sadece o
projede** görünür ve git'e girer. Yeni skill'in `/komut` olarak tanınması için genelde **restart
gerekir**. Yeni konuşmada Claude varlığını bilir ve **proaktif çağırabilir**, ama bu garanti
değil (yargı); kesinlik için sen `/komut` yazarsın.

**📖 Açıklama:**

- **Kapsam:**
  - `~/.claude/skills/...` → **kullanıcı-seviyesi** (tüm projeler, global).
  - `<repo>/.claude/skills/...` → **proje-seviyesi** (sadece o repo) + git'e girer (kalıcı,
    version'lı). İstediğim buydu.
- **Dosya formatı:** frontmatter (`name`, `description`, `user-invocable: true`) + gövdede
  talimatlar. `user-invocable: true` olmazsa `/komut` ile çağrılamaz.
- **Restart:** skill listesi genelde **oturum açılışında** taranır → yeni skill için restart.
  (Restart yaptım, `/not` geldi ✓.)
- **Tetiklenme garantisi:**
  | Yöntem | Garanti |
  |---|---|
  | Sen `/not` yazarsın | ✅ %100 |
  | Claude proaktif çağırır | 🟡 best-effort (description + memory dürtüyor ama yargı) |
  | Hook | ❌ bu iş için uygun değil |
- **Önemli nüans:** yeni bir konuşmada Claude **sadece o konuşmadaki** içeriği bilir; geçmiş
  konuşmaların içeriğini hatırlamaz (sadece memory dosyaları + repo taşınır). Yani `/not`,
  **içinde bulunulan oturumdaki** kavramı kaydeder.

**🔗 İlgili:**
- [.claude/skills/not/SKILL.md](../../.claude/skills/not/SKILL.md)
- Yukarıda [Girdi 1](#1-memory-mi-skill-mi-hook-mu-kalıcılık--tetiklenme)

---

## 3. Bir commit'te neden birden çok "gate" çalışıyor?

**🗓️ Bağlam:** İlk Go kodunu (`main.go` + testi) commit'lerken önce 1, sonra ardı ardına **3
gate** (güvenlik + test coverage + kod review) devreye girdi. Markdown commit'lerinde ise sadece
1 tanesi çalışıyordu. "Neden bu kadar çok kontrol?" sorusu doğdu.

**⚡ Kısa cevap:** Bu repo'da **pre-commit hook'ları** var: her commit'te güvenlik review'u,
**kaynak kod** değişince ayrıca test coverage ve kod review gate'leri çalışır. Markdown/docs
commit'inde coverage & kod review "kaynak yok" deyip kendiliğinden onaylar; sadece güvenlik
çalışır.

**📖 Açıklama:**

- Gate'ler birer **pre-commit hook**: commit'i bloklar, ilgili skill çalışır, bir "verdict"
  dosyası yazılır, commit o zaman geçer.
- **Docs-only commit** → çoğunlukla sadece güvenlik gate'i (diğerleri kaynak kod arar, bulamayıp
  geçer).
- **Kaynak kod (`.go`) commit'i** → üçü birden: güvenlik + coverage ("testi var mı?") + kod
  review ("bug/kalite var mı?").
- Pratik sonuç (öğrenme temposu için): kod yazmak/çalıştırmak/test etmek gate tetiklemez —
  **sadece `git commit`** tetikler. O yüzden birkaç adımı yapıp **milestone'da tek commit**
  atmak, her micro-adımda 3-gate dansını yaşamaktan iyidir.

**🔗 İlgili:**
- Bu gate'ler `5c8a677` (ilk kod commit'i) sırasında çalıştı.

---

## 4. gofmt vs golangci-lint (Prettier vs ESLint)

**🗓️ Bağlam:** CI'ı kurunca ilk çalışmada **kırmızı** oldu — `gofmt`, `main_test.go`'nun
doc-comment formatını yakaladı. "gofmt'i lokale ekleyelim mi, golangci-lint ile farkı ne?"
diye sordum (lokalde golangci-lint çalıştırmıştım, 0 issue vermişti).

**⚡ Kısa cevap:** İkisi **birbirinin yerine geçmez**:
- **gofmt = formatter** (kodun nasıl *göründüğü*) → JS'teki **Prettier**.
- **golangci-lint = linter** (kodun *doğru/kaliteli* olup olmadığı) → JS'teki **ESLint**.

**📖 Açıklama:**

| | gofmt | golangci-lint |
|---|---|---|
| İş | biçim (girinti, boşluk, import, doc-comment) | bug/kalite (errcheck, unused, staticcheck...) |
| JS karşılığı | **Prettier** | **ESLint** |
| Nereden | Go ile **gelir** (toolchain) | **3. parti** (kurarsın) |
| Config | sıfır (tek kanonik biçim) | `.golangci.yml` (yapılandırılabilir) |
| Kapsam | sadece görünüş | çok daha geniş (doğruluk) |

**Bizim olayın sırrı:** Lokalde **golangci-lint (ESLint)** çalıştırdım → 0 issue (bug yok). Ama
**gofmt (Prettier)**'i çalıştırmadım → biçim sapması kaçtı → **CI'ın ayrı gofmt adımı** yakaladı.
(golangci-lint v2, isteğe bağlı gofmt'i de içine alabilir ama varsayılanda almıyor.)

**🔗 İlgili:**
- [.githooks/pre-commit](../../.githooks/pre-commit) (gofmt+vet), [.githooks/pre-push](../../.githooks/pre-push) (golangci-lint+test)
- [.github/workflows/ci.yml](../../.github/workflows/ci.yml) (ikisi de CI'da)
- [docs/WHY_GO_VS_TS.md](../WHY_GO_VS_TS.md) §13 (zero-config toolchain)

---

## 5. pre-commit vs pre-push: kontrolleri maliyete göre katmanlama

**🗓️ Bağlam:** gofmt/golangci-lint'i lokalde nereye koyalım diye konuşurken **pre-push** hook'unu
öğrendim ("golangci-lint'i pre-push'a koymak ne demek?").

**⚡ Kısa cevap:** git'in birden çok hook türü var. Kontrolleri **maliyetine göre katmanla**:
ucuz olanı sık (pre-commit), pahalı olanı seyrek (pre-push).

**📖 Açıklama:**

- **pre-commit** → her `git commit`'te çalışır (**sık**). Ucuz kontrollere ideal.
- **pre-push** → her `git push`'ta çalışır (**seyrek** — çok commit atarsın, az push'larsın).
  Ağır kontrollere ideal ("kod makineden çıkmadan bir kez tam kontrol").

Bizim kurulum ([core.hooksPath](../../.githooks/) ile aktifleşir):

| Katman | Ne | Ne zaman |
|---|---|---|
| **pre-commit** | gofmt + go vet | her commit (anlık) |
| **pre-push** | golangci-lint + go test -race | her push (ağır) |
| **CI** | hepsi | sunucuda (otoriter backstop) |

Neden pre-push? golangci-lint/test **ağır**laşabilir; her commit'te çalışması sık commit atanı
yavaşlatır. pre-push, onu **her commit yerine her push'ta bir kez** çalıştırır → CI kırmızı
olmadan, sürtünmesiz. (Küçük projede ikisini de pre-commit'e koymak da olur; katmanlama
"büyüyünce" önem kazanır.)

**Kurulum notu:** `core.hooksPath` **yerel git config**'tir (commit'lenmez); hook dosyaları
`.githooks/`'te versiyonlanır ama her makinede bir kez `git config core.hooksPath .githooks`
gerekir.

**🔗 İlgili:**
- [.githooks/pre-commit](../../.githooks/pre-commit), [.githooks/pre-push](../../.githooks/pre-push)
- Yukarıda [Girdi 4](#4-gofmt-vs-golangci-lint-prettier-vs-eslint)

---

## 6. VS Code eklentisi aktivasyonu ve "command not found" yarışı

**🗓️ Bağlam:** `GET /api/books/{id}` testlerini yazarken VS Code'u yeni açmıştım. Sağ alttaki
Go sürüm rozetine (`1.27.0`) tıkladım ve **`command 'go.environment.status' not found`** hatası
aldım. "Go kurulumum mu bozuldu, eklentiyi yeniden mi kurayım?" diye sordum. Cevap: hiçbir şey
bozuk değil; bu, eklentinin **açılış sırasındaki birkaç saniyelik yarış (race)** penceresi.

**⚡ Kısa cevap:** Go eklentisi status bar öğesini `go.environment.status` komutuna bağlayıp
**hemen gösteriyor**, ama komutun kendisini ancak aktivasyonun **sonunda** (gopls başladıktan
sonra) kaydediyor. O aralıkta tıklarsan VS Code komutu registry'de bulamaz. ⚡ ikonu gelince
tıkla → çalışır. Israr ederse `Developer: Reload Window`.

**📖 Açıklama:**

VS Code'da bir eklenti nasıl "canlanır":

- Eklentiler **extension host** denen ayrı bir Node.js sürecinde çalışır. Bir `.go` dosyası
  açılınca (`activationEvents: onLanguage:go`) eklentinin `async activate()` fonksiyonu çağrılır.
- Bir **command** aslında sadece bir **string id** → handler eşlemesidir; `registerCommand("id", fn)`
  ile bir Map'e yazılır. Status bar öğesi de sadece bu id'yi tutar (`item.command = "id"`).
- JS/TS analojisi: `emitter.emit("x")` demeyi `emitter.on("x", …)` satırından **önce** çağırmak.
  Listener henüz yoksa hiçbir şey olmaz; VS Code bunu "command not found" hatasıyla söyler.

`golang.go` 0.56.1 `activate()` sırası (dist/goMain.js, satır 40355–40392):

| Adım | Ne olur | Status bar |
|---|---|---|
| `await updateGoVarsFromConfig()` | `go env` çalışır, callback'te `initGoStatusBar()` | öğe **görünür**, command id atanmış, arka plan **sarı** (gopls yok) |
| `await maybeInstallImportantTools()` | araç kontrolü (yerel `go version` vb.) | tıklanabilir ama komut **kayıtsız** |
| `await startLanguageServer()` | gopls başlar | ⚡ ikonu gelir |
| `registerCommand("go.environment.status")` | komut **ancak şimdi** kayda girer | artık çalışır |

VS Code tarafı: komut registry'de yoksa `onCommand:<id>` activation event'ini bekler; ama bu id
`package.json`'da **ilan edilmemiş** (sadece `go.environment.choose` var), `*` aktivasyonu da
çoktan bitmiş → anında `Error: command 'go.environment.status' not found`.

Bugünkü zaman çizelgesi (loglardan; normalde bu aralık 1–2 s, bugün extension host aşırı yüklü
olduğundan ~10 s):

| Saat | Kaynak | Olay |
|---|---|---|
| 18:43:42.7 | exthost.log | `golang.go` aktivasyonu başladı |
| 18:43:51–53 | renderer.log | extension host "unresponsive" |
| 18:43:53.4 | Go.log | "Try to start language server" |
| 18:43:54.750 | Go.log + state.vscdb | gopls çalışıyor; aynı ms'de komut kaydı yapıldı |

**Nasıl teşhis ettik (öğrenilecek asıl şey):** tahmin yerine log okuduk.
`~/Library/Application Support/Code/logs/<oturum>/window<N>/exthost/exthost.log` (aktivasyon
hataları), aynı klasörde `golang.go/Go.log` (eklentinin kendi "Go" output kanalı) ve
`~/.vscode/extensions/golang.go-*/dist/goMain.js` (eklenti kodu). VS Code içinden:
`Output → Go`, `Output → Log (Extension Host)`, `Developer: Show Running Extensions`.

**Yapma:** eklentiyi kaldırıp kurma, sürüm değiştirme, GOPATH silme, PATH/GOROOT oynama.
Loglar `go env`'in başarılı olduğunu ve gopls'un başladığını gösteriyor; sorun kurulumda değil.

**Upstream durumu:** aynı sıralama 0.56.1 (güncel stable), 0.57.2 (pre-release) ve master'da
da var; hiçbir sürüm düzeltmiyor. Aynı hata için açılmış #3065 teşhis konmadan kapanmış. Kalıcı
çözüm eklenti tarafında: `registerCommand`'ı `updateGoVarsFromConfig`'in üstüne taşımak ya da
`onCommand:go.environment.status` activation event'i eklemek (#457'de `go.locate.tools` için
aynı sınıf sorun böyle çözülmüş).

**🔗 İlgili:**
- [golang/vscode-go#3065](https://github.com/golang/vscode-go/issues/3065) (aynı hata, teşhissiz kapanmış), [#457](https://github.com/golang/vscode-go/issues/457) (aynı sınıf sorunun düzeltilmesi)
- [docs/STATUS.md](../STATUS.md) (o sırada çalıştığımız faz)
- Yukarıda [Girdi 1](#1-memory-mi-skill-mi-hook-mu-kalıcılık--tetiklenme) (VS Code / Claude Code araç katmanları)

## 7. Playwright ile uçtan uca test: gerçek tarayıcı, rate limit bütçesi ve storageState tuzağı

**🗓️ Bağlam:** Faz 13'te, "httpOnly çerez + token ailesi iptali + Playwright" isteğinin son
parçası. O ana kadar "E2E" dediğimiz şey CI'da `curl`'le API'ye atılan isteklerdi; inceleme de
(F16) "gerçek tarayıcı akışları doğrulanmıyor" dedi. "Zaten vitest ve smoke var, Playwright ne
ekliyor?" sorusunun cevabı bu not.

**⚡ Kısa cevap:** vitest jsdom'da koşar ve orada **çerez diye bir şey yoktur**; `curl` ise
tarayıcı değildir. Faz 13'ün konusu tam olarak tarayıcıya ait (httpOnly çerez, `SameSite`,
sekmeler arası Web Locks, `window.prompt`) — onu ancak çerez kavanozu olan gerçek bir tarayıcı
sınayabilir. Ama gerçek yığına karşı koşmanın kendi kısıtları var: rate limit ve oturum durumu.

**📖 Açıklama:**

**Üç test katmanı ne sınar:**

| Katman | Nerede koşar | Ne yakalar | Ne YAKALAMAZ |
|---|---|---|---|
| vitest (jsdom) | `fetch` sahte | istemci mantığı, dallar, hata yolları | çerez, tarayıcı politikaları |
| `mvp_smoke.sh` (curl) | gerçek API | sözleşme, durum kodları | NGINX + tarayıcı davranışı |
| Playwright (chromium) | gerçek yığın | çerez öznitelikleri, rotasyon, sekmeler, dialog | birim seviyesi ayrıntı |

**1) Rate limit bütçesi = SERT KURAL.** Testler gerçek NGINX'ten geçiyor ve NGINX'in
`login|register` kovası **imajın içinde** (yükseltilemez): anında 11 istek, sonra 3 saniyede 1.
Host'tan gelen her istek konteyner için aynı adres. Suite kararlı hâlde **9** harcıyor. Bunu
disiplin hâline getiren dört karar:

- Bütçe **tablosu** `e2e/helpers.ts`'in başında; yeni senaryo eklerken önce tabloya satır eklenir.
- 429 gelince test **adıyla patlar** (sessiz yeniden deneme bütçe aşımını gizlerdi — bu bir
  tripwire).
- Bütçeye girmemesi gereken çağrılar NGINX'i **atlar** (`DIRECT_API_URL`): paylaşılan hesabın
  kaydı, "eski parola reddedilmeli" gibi başarısız olması beklenen kontroller.
- Koşucu (`scripts/e2e.sh`) her koşudan önce web konteynerini **yeniden başlatır**: `limit_req`
  sayacı nginx sürecinin paylaşılan belleğindedir, restart onu sıfırlar → arka arkaya `make e2e`
  koşuları güvenli olur. Garantisi bundan ibaret: **koşunun BAŞINDA dolu kova**. Koşu İÇİNDEKİ
  yeniden denemeler bütçeyi tekrar harcar (`playwright.config.ts` `retries: CI ? 1 : 0`);
  auth-flow'un tek bir tekrarı 9'u **13**'e çıkarır (> 11). CI ayrıca `scripts/e2e.sh`'ı hiç
  koşmaz (doğrudan `npx playwright test`; kovayı orada her job'ın yeniden kurduğu konteynerler
  sıfırlar). 429 bu yüzden "yeniden denenebilir hata" değil, **tripwire**.

**2) `storageState` neden PAYLAŞILMIYOR.** Playwright'in klasik hızlandırması "bir kez giriş yap,
oturum durumunu (`storageState`) tüm testlere ver"dir. Bizde bu **yanlış**: refresh token tek
kullanımlık ve tekrar kullanımı **tüm aileyi** iptal ediyor. Aynı çerezi iki test kullanırsa
ikincisi harcanmış bir token sunar, reuse tespiti devreye girer ve testler birbirini düşürür.
Mutasyonla da doğrulandı: iki test arasında `storageState` paylaşmak suite'i kırmızıya çevirdi.
**Genel ders:** hazır test deseni, uygulamanın oturum modeliyle çelişebilir; deseni değil modeli
esas al.

**3) Bir e2e suite'i de mutasyonla sınanır.** "Yeşil" tek başına bir şey söylemez — suite gerçekten
bir şey mi tutuyor? Üç mutasyon denendi ve her biri bir senaryoyu kırmızıya çevirdi:
NGINX'in `Host` başlığını eski hâline (`$host`) almak, `refresh_token`'ı login cevabının gövdesine
geri koymak, `storageState` paylaşmak. Kırmızıya çevirmeyen bir test, koruduğunu iddia ettiği şeyi
korumuyordur.

**4) Suite gerçek bir hata buldu.** Katalog senaryosu, yerel kalabalık bir veritabanında kırmızıya
döndü: filtre değişince atılan yeni istek ile uçuştaki eski istek yarışıyor ve **filtresiz sorgu
daha yavaş** olduğu için geç cevap ızgaranın üstüne boyuyordu. Gerçekçi veri hacmi olmadan hiç
görünmeyecekti (inceleme bulgusu F10).

**Pratik ayrıntılar:** izler (trace) çerez ve parola içerir → `.gitignore` + `.dockerignore`,
CI'da yalnız hata durumunda ve kısa saklama süresiyle yüklenir. Sır (admin parolası) rastgele
üretilir, tek bir adımın içinde kalır ve `::add-mask::` ile log'da yıldızlanır. vitest'in
`include`'u `src/**`'e daraltıldı ki spec'leri toplamasın.

**🔗 İlgili:**
- [docs/phases/phase-13-session-hardening.md](../phases/phase-13-session-hardening.md)
- [frontend/e2e/helpers.ts](../../frontend/e2e/helpers.ts) (bütçe tablosu) · [frontend/playwright.config.ts](../../frontend/playwright.config.ts) · [scripts/e2e.sh](../../scripts/e2e.sh)
- [#5 pre-commit vs pre-push](#5-pre-commit-vs-pre-push-kontrolleri-maliyete-göre-katmanlama) — kontrolleri maliyete göre katmanlamanın aynı mantığı
- [docs/REVIEW_FEEDBACK.md](../REVIEW_FEEDBACK.md) F16, F10
