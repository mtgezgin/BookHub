# Öğrenme Notları — Sorduğum Kavramlar ve Cevapları

Bu klasör **kişisel hatırlama günlüğüm**. Proje boyunca sorduğum, iyi bilmediğim kavramları
buraya kaydediyoruz — ama tek dosyada değil, **konuya göre kategorilere** bölünmüş hâlde.

Amaç: sonradan dönüp okuyunca **hem cevabı hem de o soruyu neden sorduğumu (bağlamı)**
hatırlayabilmek.

## Nasıl çalışıyor

Bir kavram öğrendiğimde, **ilgili kategori dosyasına** yeni bir girdi ekleniyor (en yeni en
altta). Girdi formatı:

- **🗓️ Bağlam** — bunu neyi yaparken / neden sordum (gelecekteki ben için).
- **⚡ Kısa cevap** — tek-iki cümlede özü.
- **📖 Açıklama** — sade, beginner-dostu anlatım (gerekirse kod/tablo).
- **🔗 İlgili** — diğer dosyalardan ilgili kısımlara linkler.

> Yeni girdi eklemek için: `/not` komutu (ya da bana "nota ekle" demen) — ilgili kategoriyi
> kendisi seçer. Dil: hibrit (açıklama Türkçe, teknik terim/kod İngilizce).

## Kategoriler

| Dosya | Kapsam | Durum |
| --- | --- | --- |
| [go-language.md](go-language.md) | Go dilinin kendisi: stdlib, idiomlar, test konvansiyonları, syntax, generics | aktif |
| [backend-and-web.md](backend-and-web.md) | Dil-bağımsız backend/web: HTTP, API, config/12-factor, server lifecycle (graceful shutdown), hata yönetimi | aktif |
| [tooling-and-workflow.md](tooling-and-workflow.md) | Claude Code (skill/memory/hook), git, pre-commit gate'ler, commit ritmi, geliştirme aracı kavramları | aktif |
| [database.md](database.md) | Postgres, SQL, sqlc, migration, transaction, index | aktif |
| [security-and-auth.md](security-and-auth.md) | auth, JWT, parola hash, IDOR, rate limit, sır yönetimi | aktif |
| [infra-and-devops.md](infra-and-devops.md) | Docker, Redis hosting, deploy, CI/CD, K8s | aktif |

> "Sırası gelince" kategoriler **boş oluşturulmaz** — ilgili ilk girdi gelince dosyası açılır
> (boş scaffold yok). Yeni bir konu mevcut hiçbir kategoriye uymuyorsa yeni bir dosya açıp bu
> tabloya eklenir.

## İlgili öğrenme dosyaları

- [docs/phases/](../phases/) — faz faz adım adım yapım rehberi
- [docs/WHY_GO_VS_TS.md](../WHY_GO_VS_TS.md) — Go vs TS/JS yaşayan karşılaştırma
- [docs/GO_CONVENTIONS.md](../GO_CONVENTIONS.md) — Go'yu nasıl yazıyoruz
