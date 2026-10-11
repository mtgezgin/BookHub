# BookHub — Geliştirme süreci

> **Amaç:** bir işin fikirden canlıya **kanıtlı** geçmesi. Profesyonel bir ürün ekibinin akışından
> tek kişilik bir projeye uyarlandı: her aşamanın girişi ve çıkışı yazılı, her aşama geride bir
> kanıt bırakıyor (kabul kriteri, test, PR tablosu, yayın kaydı, doğrulama notu).
>
> Kurallar kısa hâliyle [CLAUDE.md](../CLAUDE.md)'de; bu doküman "neden" ve "nasıl" kısmı.

## 1. Bir işin yolculuğu

```
Backlog → Ready → In Progress → In Review → Testing → Released → Done
```

| Statü | Ne demek | Bir sonrakine geçmek için |
|---|---|---|
| **Backlog** | Issue açıldı; yalnız problem yazılı olabilir. Panoya kendiliğinden düşer (§9). | Hazır tanımı (§2) karşılanır, "Hazır mı?" kutuları işaretlenir. |
| **Ready** | Problem, kapsam ve kabul kriterleri net; öncelik etiketi var. | İşe başlanır, dal açılır. |
| **In Progress** | `feat/<issue>-kisa-ad` ya da `fix/<issue>-kisa-ad` dalında kod ve testler yazılıyor. Push'ta kapılar çalışır (aşağıdaki not). | PR açılır (şablon dolu, issue'ya `Refs #…`). |
| **In Review** | CI çalışıyor; AI review bulguları ya çözülüyor ya gerekçesiyle kabul ediliyor. Kullanıcı kodu okuyor ve "neden böyle"yi cevaplayabiliyor. | CI yeşil, bulgular kapandı. |
| **Testing** | Kabul kriterleri doğrulanıyor: otomatik testler, gerekiyorsa lokal tam yığında (`make app-up`) manuel adımlar. Sonuçlar PR'daki tabloya. | Her kriter doğrulandı ya da "doğrulanamadı" diye gerekçesiyle yazıldı → merge. Issue açık kalır. |
| **Released** | main'de CI ve Deploy yeşil bitti; `release` bu commit'te, Northflank yayınladı. | Yayından sonra doğrulama (§6). |
| **Done** | Doğrulandı; sonuç issue'ya yazıldı ve issue elle kapatıldı. | — |

**Push kapıları nerede çalışır:** `.githooks/pre-push` repo kancasıdır (golangci-lint +
`go test -race`; `git config core.hooksPath .githooks` gerekir). Güvenlik incelemesi, AI review
paneli ve test coverage ise Claude Code eklenti kancalarıdır; yalnız Claude Code üzerinden
yapılan push'ta çalışırlar. Terminalden push edildiyse bu kapıların koşmadığı PR'da yazılır.
GitHub'daki asıl güvenlik ağı CI'dır.

İş bir yerde takılırsa issue'ya nedenini yaz ve bulunduğu sütunda bırak; sessizce bekleyen
iş, kaybolmuş iştir.

## 2. Hazır tanımı (Definition of Ready)

Bir iş ancak şunlar yazılıysa **Ready** olur. Özellik şablonunun en altındaki "Hazır mı?"
kutuları bunların listesidir; Ready'ye alırken issue düzenlenip işaretlenir.

- Problem, çözümden bağımsız: kim etkileniyor, bugün ne oluyor, neden önemli.
- **Kapsam içi** ve **kapsam dışı**. Kapsamın kaymasını en iyi "kapsam dışı" önler.
- **Numaralı, test edilebilir kabul kriterleri** ("X yapılınca Y olur"). Belirsiz bir kriter
  ("hızlı olsun") kriter değildir; ölçüsüyle yazılır.
- Bağımlılıklar; açık sorular ya cevaplandı ya kapsam dışına alındı (`[?]` ile işaretli kalmaz).
- Öncelik etiketi: `P0` / `P1` / `P2`.
- Migration gerekiyorsa geriye uyumluluk planı (§5).

## 3. Bitti tanımı (Definition of Done)

- Her kabul kriteri bir teste ya da yazılı bir manuel adıma eşlendi (PR'daki tablo).
- CI yeşil (backend, frontend, imaj taraması, E2E). Push kapıları koştu ya da koşmadıkları PR'da
  yazılı (§1'deki not).
- Doğrulanamayan her şey PR'da yazılı: neden, ne zaman, nasıl doğrulanacak.
- Yayınlandı ve yayından sonra doğrulandı (§6).
- Gerekirse DEPLOYMENT, STATUS ve öğrenme notları güncel.

## 4. Bug triage

Bug şablonuyla açılan her kayıt `needs-triage` etiketiyle gelir. Triage'da kayda **tek bir
karar** yorumu yazılır ve etiket kaldırılır:

| Karar | Yorumda ne olmalı |
|---|---|
| **Doğrulandı** | Öncelik (değişmese bile) ve hangi eksene dayandığı: kullanıcıya etkisi, sıklığı, ciddiyeti. |
| **Tekrar üretilemedi** | Kaç deneme, hangi koşullar değiştirildi, bildirenden ne gerekiyor. Kayıt açık kalır. |
| **Tasarım gereği** | Dayandığı spesifikasyon. Spesifikasyon şüpheliyse ayrı bir özellik isteği açılır. |
| **Yapılandırma** | Hangi ayar, doğru değeri ne. |
| **Aslında özellik isteği** | Özellik şablonuyla yeniden açılır, bu kayıt kapanır. |
| **Tekrar / zaten düzeltildi** | Kapsayan kayıt ya da düzelten commit ve yayın. |

Her kararda ortam ve commit, atılan adımlar, gözlenen ile beklenen ve **en az bir kanıt**
bulunur: ekran görüntüsü, log satırı ya da `X-Request-Id`. Kanıtsız "bug değil" deyip kapatılmaz.

Bir düzeltme PR'ı şu sırayla anlatılır: **kök neden** (mekanizma + kanıt) → **düzeltme** →
**doğrulandı** → **doğrulanamadı** → **yayından sonra takip** (ör. geçici çözümün kaldırılması).

## 5. Yayın akışı

```
dal → PR → CI (PR) → merge → main → CI (main) ──yeşil──▶ Deploy iş akışı
                                                         ├─ plan: yeni migration varsa
                                                         │    manage migrate (canlı veritabanı)
                                                         └─ release: release dalını bu commit'e ilerlet
                                                                    │
                                                                    ▼
                                                   Northflank: release'i derle ve yayınla
```

**Neden böyle:** "main'e push = canlı" olsaydı testleri kırmızı bir commit de canlıya çıkardı.
Northflank artık yalnız `release` dalını izliyor; o dalı da yalnız Deploy iş akışı
([.github/workflows/deploy.yml](../.github/workflows/deploy.yml)) ve yalnız CI'ın test ettiği
commit'e ilerletiyor. Sırla çalışan migration işi salt-okunur, dalı ilerleten iş ise repo kodu
çalıştırmıyor.

- **Migration önce gelir.** Yeni şema, eski kod hâlâ canlıdayken uygulanır. Bu yüzden her
  migration o an canlıda çalışan kodla **geriye uyumlu** olmalı: önce genişlet (yeni kolon,
  varsayılanıyla) → yayınla (kod yeni kolonu kullanır) → sonra daralt (eski kolon ayrı bir PR'da,
  genişletme yayını canlıda doğrulandıktan sonra silinir). Migration başarısız olursa `release`
  ilerlemez.
- **Canlıya çıkmış migration dosyası değişmez.** goose sürümü numarasıyla takip eder; uygulanmış
  bir dosyanın yeni hâli canlıya hiç gitmez. Deploy, değişen/silinen bir migration dosyası
  görürse durur. Düzeltme her zaman yeni bir migration'dır. Deploy'un **uygulamaya çalıştığı**
  her migration canlıya çıkmış sayılır, release adımı düşmüş olsa bile: goose onu (ya da bir
  kısmını) uygulamış olabilir, ama `release` ilerlemediği için Deploy'un koruması o dosyayı
  hâlâ "yeni" görür.
- **`release`'e elle push edilmez.** Özel repoda branch koruması ücretli (GitHub Pro ya da public
  repo); şimdilik bu bir kural.
- Bekleyen yayınlar iptal edilmez, sırayla koşar. Release'in zaten kapsadığı bir commit'in
  koşusu "zaten yayında" diye atlanır.
- Yalnız doküman değişen commit'ler CI'ı tetiklemez; bir sonraki yeşil CI'ın commit'iyle
  birlikte `release`'e girerler.
- Dependabot PR'ı merge edilince de aynı yol işler: CI → Deploy → canlı.
- **Bir koşu başarısız olursa:** sebep CI'daysa (flaky test) Deploy'u değil **CI'ı** yeniden
  koştur; CI yeşil bitince Deploy kendiliğinden tetiklenir. Deploy'un kendisi düştüyse
  Actions → Deploy → **Re-run jobs**.
- **"main'de daha yeni bir iş akışı değişikliği var" notu:** GitHub, iş akışı dosyası değiştiren
  bir commit'i Actions token'ıyla `release`'e taşımayı main'in ucu farklıysa reddediyor. Deploy
  bu durumu Plan'da (main o arada ilerlediyse Release adımında) yakalar ve o commit'i atlar;
  main'in ucundaki commit'in kendi Deploy'u onu da kapsayarak yayına çıkarır. Yeniden denemek
  işe yaramaz, gerek de yoktur. Ucun CI'ı kırmızıysa yayın, main'deki bir sonraki yeşil CI'a
  kadar bekler.
- **CI, değişiklikle ilgisiz bir güvenlik açığı yüzünden kırmızıysa** (ör. temel imajda yeni bir
  CVE): yayın kapısı bunun için var, `release`'e elle push edilmez. Temel imaj digest'ini
  güncelleyen bir PR aç (`make scan` ile doğrula). Upstream'de düzeltilmiş imaj yoksa CVE
  kimliği, issue bağlantısı ve bitiş tarihi yazılı, süreli bir `.trivyignore` girdisi PR'la
  eklenir (ci.yml'deki Trivy adımlarının bu dosyayı okuduğunu da doğrula).
- **Geri dönüş:** bozuk commit `git revert` ile geri alınır; revert de aynı yoldan geçer.
  Commit bir migration içeriyorsa migration dosyası **korunur**, şema gerekiyorsa yeni bir ileri
  migration'la düzeltilir:
  ```bash
  git revert --no-commit <commit> && git checkout HEAD -- backend/migrations && git commit
  # PR bir merge commit'iyle girdiyse ana hattı seç (-m 1):
  git revert --no-commit -m 1 <merge-commit> && git checkout HEAD -- backend/migrations && git commit
  ```
  Acil durumda Northflank panelinde servisi önceki bir build'e döndürme seçeneklerine bak;
  kalıcı düzeltme yine revert'le yapılır.

## 6. Yayından sonra doğrulama

Yalnız okuma yapan kontroller. `make smoke` ve `make e2e` canlıya karşı **çalıştırılmaz**,
çünkü veritabanına test verisi bırakırlar.

- [ ] Northflank'ta `api` ve `web` **Running**, yeniden başlatma yok; derlenen commit
      `release`'teki commit.
- [ ] `https://<site>/health` → `{"status":"ok"}`
- [ ] Ana sayfa açılıyor; katalog ve "En iyiler" geliyor.
- [ ] Değişen akış canlıda bir kez denendi (ör. giriş + F5'te oturumun kalması).
- [ ] `api` loglarında yayından sonra yeni `ERROR` satırı yok (Northflank → api → Logs).
- [ ] Sonuç issue'ya yazıldı (doğrulanamayan bir şey varsa nedeni ve takibiyle) ve issue elle
      kapatıldı → pano **Done**.

## 7. Olay (incident) döngüsü

1. **Etkiyi durdur:** revert ya da geçici çözüm. Sır sızıntısı ya da oturum çalınması
   şüphesinde: sırrı döndür ve oturumları iptal et ([DEPLOYMENT.md](DEPLOYMENT.md) §2,
   "Herkesi anında çıkarmak").
2. Olay şablonuyla bir issue: etki, zaman çizelgesi, nasıl fark edildiği, kök neden.
3. Her aksiyon ayrı bir issue.
4. **Önleme kuralı:** bu hata *sınıfını* bir daha ne yakalayacak? Yeni bir test, bir alarm ya da
   [CLAUDE.md](../CLAUDE.md)'ye eklenen bir review kuralı. Önleme kuralı yazılmadan olay
   kapanmaz.

Hedef, olayları kullanıcıdan önce kendi izlememizin bulması. Canlıda henüz izleme ve alarm yok;
sıradaki adım bu.

## 8. Etiketler

| Etiket | Ne zaman |
|---|---|
| `feature` | Yeni davranış ya da iyileştirme |
| `bug` + `needs-triage` | Beklenmeyen davranış; triage kararı bekliyor |
| `incident` | Canlıda olay |
| `security` | Güvenlik işi ya da bulgusu |
| `tech-debt` | Teknik borç |
| `research` | Araştırma; çıktısı bir karar |
| `P0` / `P1` / `P2` | Öncelik: olmazsa olmaz / önemli / iyi olur |

## 9. Bir kerelik kurulum

Sıra önemli: `release` dalı, Northflank ona çevrilmeden ve yeni bir şey push edilmeden önce
**şu an canlıda olan commit'te** oluşmalı. Northflank `release`'e çevrildikten sonraki ilk push,
artık doğrudan değil Deploy üzerinden yayına çıkar.

0. **Ön koşul — CI yeşil olmalı.** Push edilecek değişiklik CI'dan geçmezse Deploy hiç çalışmaz.
   (Kurulum sırasında temel NGINX imajındaki bir CVE Trivy işini kırmızıya çevirmişti; aynı
   değişiklik setindeki digest güncellemesi bunu kapatıyor, `make scan` ile doğrulandı.)
1. `scripts/setup-github.sh` — etiketleri ve `release` dalını oluşturur.
2. Northflank: `api` ve `web` servislerinin derlendiği dalı `main` → `release` yap (Build options).
   İstersen aynı yerde commit mesajı atlama bayraklarını kapat.
3. Migration'lar için GitHub secret'ı (değer ekranda görünmez):
   ```bash
   security find-generic-password -s bookhub-prod-dsn -w | tr -d '\n' | gh secret set PROD_DATABASE_URL
   ```
4. (İsteğe bağlı, çift sigorta) GitHub → Settings → General → Issues: "Auto-close issues with
   merged linked pull requests" kapalı.
5. Değişiklikleri commit'le ve push et. CI yeşil → Deploy yeşil → Northflank yayınlar → §6.
6. Pano: GitHub → Projects → New project → **Board**. Status sütunlarını §1'deki statüler yap.
   Workflows: **Auto-add to project** (repo BookHub, filtre `is:issue is:open`) ve
   **Item added to project → Status: Backlog**. Mevcut açık issue'lar bir kez elle eklenir.
   (`gh` ile yapmak için önce `gh auth refresh -s project` gerekir.)
