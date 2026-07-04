# Öğrenme Notları — Tooling & Workflow

Geliştirme aracı kavramları: Claude Code (skill / memory / hook), git, pre-commit gate'ler,
commit ritmi. Format: [README.md](README.md).

## İçindekiler

1. [Memory mi, skill mi, hook mu? (kalıcılık & tetiklenme)](#1-memory-mi-skill-mi-hook-mu-kalıcılık--tetiklenme)
2. [Proje-içi skill nasıl yapılır + restart + otomatik çağrılır mı?](#2-proje-içi-skill-nasıl-yapılır--restart--otomatik-çağrılır-mı)
3. [Bir commit'te neden birden çok "gate" çalışıyor?](#3-bir-committe-neden-birden-çok-gate-çalışıyor)
4. [gofmt vs golangci-lint (Prettier vs ESLint)](#4-gofmt-vs-golangci-lint-prettier-vs-eslint)
5. [pre-commit vs pre-push: kontrolleri maliyete göre katmanlama](#5-pre-commit-vs-pre-push-kontrolleri-maliyete-göre-katmanlama)

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
