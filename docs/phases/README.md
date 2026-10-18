# BookHub — Faz Faz Öğrenme Klasörü

Bu klasör, projeyi **adım adım, öğrenerek** inşa etmek için bir rehberdir. Her faz ayrı bir
dosyadır. Amaç hız değil; her fazda **tek bir yeni kavramı** gerçekten anlamak.

> Bu dosyalar hibrittir: açıklamalar **Türkçe**, kod ve teknik terimler **İngilizce** (kodla
> eşleşsin diye). Asıl roadmap [PROJECT_BRIEF.md](../PROJECT_BRIEF.md) §11'de; burası onun
> başlangıç dostu, adım adım hâli.

## Nasıl çalışıyoruz (her küçük adımda)

1. **Önce anlatırım** — bu nedir, neden var (jargonsuz).
2. **En küçük çalışan parçayı kurarız.**
3. **Ne yaptığımızı satır satır açıklarım.**
4. **Sen çalıştırıp gözünle görürsün.**
5. **"Netleşti mi?"** — netleşmeden sonraki adıma geçmeyiz.

Tek seferde bir kavram. **Hiçbir soru fazla basit değil.** Takıldığın an "dur" diyebilirsin.

## Öğrenme merdiveni

**MVP = Faz 1–7.** Asıl kazanma koşulu burası. Faz 8–12 "bonus altyapı" rayı, opsiyonel;
Faz 13 ise ilk kapsamlı incelemeden doğan "sertleştirme" fazı.

| Faz | Ne kuracağız | Yeni kavram | Durum |
| --- | --- | --- | --- |
| 0 | Repo + git | versiyon kontrolü, .gitignore | ✅ bitti |
| [1](phase-01-minimal-go-api.md) | Minimal Go API (`/health`) | Go yapısı, HTTP handler, JSON, config, test | ✅ bitti |
| [2](phase-02-postgres-books-genres.md) | Postgres + kitap/tür CRUD | SQL şema, migration, sqlc, repository | ✅ bitti |
| [3](phase-03-auth-roles.md) | Auth + roller | parola hash, JWT, middleware | ✅ bitti |
| [4](phase-04-suggestions-moderation.md) | Öneri + moderasyon | status state machine, admin aksiyon, audit | ✅ bitti |
| [5](phase-05-comments-ratings.md) | Yorum + puan | ilişkiler, UNIQUE+ON CONFLICT, soft-delete | ✅ bitti |
| [6](phase-06-weighted-rating.md) | Ağırlıklı puan | aggregation, pure fonksiyon, EXPLAIN ANALYZE | ✅ bitti |
| [7](phase-07-redis-cache.md) | Redis cache + rate limit | cache/TTL, "önce ölç sonra ekle" | ✅ bitti — **MVP** |
| [8](phase-08-docker.md) | Docker imajı + compose | multi-stage, distroless, servis DNS | ✅ bitti |
| [9](phase-09-frontend.md) | Frontend (React + Vite) | SPA, token yönetimi, proxy | ✅ bitti |
| [10](phase-10-cicd.md) | CI/CD | servis konteynerleri, E2E, release | ✅ bitti (deploy hesap ister) |
| [11](phase-11-nginx.md) | NGINX | reverse proxy, güvenilir IP başlığı, CSP | ✅ bitti |
| [12](phase-12-kubernetes.md) | Kubernetes | Deployment/Job/probe, pod güvenliği | ✅ bitti |
| [13](phase-13-session-hardening.md) | Oturum sertleştirme | httpOnly çerez, token ailesi + reuse, CSRF, kilit sırası, Playwright | ✅ bitti |
| [14](phase-14-ui-redesign.md) | Arayüz yeniden tasarımı | Tailwind, tasarım token'ları, shadcn/ui bileşenleri, erişilebilirlik | ✅ bitti (gerçek kapaklar dahil) |
| [15](phase-15-personal-library.md) | Kişisel kitaplık (Paket A: LIB-01, LIB-02, #42) | kullanıcıya ait gizli kaynak, bileşik anahtarın sol öneki, toplu okuma (N+1'den kaçınmak) | 📖 tasarım onaylandı, plan yazılıyor |
| [16](phase-16-works-editions.md) | Eser ve baskı (CAT-02, #36) | genişlet → daralt migration, sqlc'nin `*` açılımı, kısmi benzersiz index | 📝 tasarım incelemede |
| 17 | Okuma takibi ve inceleme (Paket B) | — | ⏭️ ileride |

Sıra ve gerekçesi: [ROADMAP.md](../ROADMAP.md).

**Durum işaretleri:** ✅ bitti · 📖 detaylı (hazır) · 📝 taslak (sırası gelince birlikte
detaylandırılacak) · ⏭️ ileride.

> Neden 3–7 taslak? Çünkü onların adımları, 1–2'de yazacağın koddan doğacak. Önceden ayrıntı
> yazmak, [WHY_GO_VS_TS.md](../WHY_GO_VS_TS.md)'deki "yazmadığın şeyi önceden doldurma"
> ilkesine aykırı olur. Her faza geldiğimizde o dosyayı birlikte detaya çıkaracağız.

## Her faz dosyasının yapısı

- 🎯 **Amaç** — ne kuracağız (tek cümle)
- 🧠 **Yeni kavram(lar)** — bu fazda öğrendiğin şey
- 🪜 **Adımlar** — küçük, sıralı, her biri "ne + neden"
- ⚠️ **Önemli noktalar** — tuzaklar ve dikkat edilecekler (ayrı blok)
- ✅ **Tamamlanma (DoD)** — faz ne zaman bitti sayılır
- 📒 **WHY_GO_VS_TS güncellemesi** — bu fazın doldurduğu placeholder'lar

## Faz bittiğinde (her faz için ritüel)

1. ✅ DoD maddelerini işaretle.
2. 📒 [WHY_GO_VS_TS.md](../WHY_GO_VS_TS.md)'de ilgili `[Faz X — … — TBD]` placeholder'ını
   gerçek `dosya:Sembol` + 3–5 satır Türkçe refleksiyonla (doğrulandı/çürütüldü) doldur.
3. 🔒 `gofmt` + `go vet` + `go test -race` yeşil; commit'le.
