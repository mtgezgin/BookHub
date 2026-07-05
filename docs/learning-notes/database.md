# Öğrenme Notları — Database

Postgres, SQL, sqlc, migration, transaction, index kavramları.
Format ve nasıl çalıştığı: [README.md](README.md).

## İçindekiler

1. [goose ve migration kavramı: "şema için git"](#1-goose-ve-migration-kavramı-şema-için-git)

---

## 1. goose ve migration kavramı: "şema için git"

**🗓️ Bağlam:** Faz 2 adım 3'te ilk migration'ı (`0001_books_genres.sql`) yazıp `goose up` /
`goose down` roundtrip'ini gördükten sonra "goose nedir, anlatabilir misin?" diye sordum.
Aynı konuşmada tablolara pgAdmin4 ile bakmak isteyince "GUI'den şema değiştirme" uyarısı da
gündeme geldi.

**⚡ Kısa cevap:** goose, şema değişikliklerini **numaralı + geri alınabilir SQL dosyaları**
olarak yöneten küçük bir CLI (migration aracı). Zihinsel model: **şema için git** — her dosya
bir commit, `goose_db_version` tablosu HEAD, `up`/`down` ileri/geri gitmek.

**📖 Açıklama:**

Çözdüğü problem — şemayı elle (psql'de `CREATE/ALTER`) yönetirsen: tarih yok (taze
makinede/CI'da/prod'da aynı şemayı birebir kuramazsın), ortamlar sessizce ayrışır (**schema
drift**), "son değişikliği geri al" düğmesi yok.

| git | goose |
|---|---|
| commit | migration dosyası (`0001_books_genres.sql`) |
| commit history | `migrations/` klasörü, numara sırası |
| HEAD (neredeyim?) | DB içindeki `goose_db_version` tablosu |
| checkout ileri/geri | `goose up` / `goose down` |
| `git log` | `goose status` |

Mekanik: dosyaları numara sırasına dizer → `goose_db_version`'a bakıp uygulanmamışları bulur
→ onların `-- +goose Up` bölümünü çalıştırır ve deftere işler. `down` = son uygulananın
`-- +goose Down` bölümü + kaydı düşmek. Tek dosyada iki yön:

```sql
-- +goose Up
CREATE TABLE books (...);
-- +goose Down
DROP TABLE books;
```

Komutlar (hep `backend/` içinden):

```bash
goose -dir migrations postgres "$DATABASE_URL" up      # uygula
goose -dir migrations postgres "$DATABASE_URL" status  # neredeyiz?
goose -dir migrations postgres "$DATABASE_URL" down    # son migration'ı geri al (SADECE dev!)
```

**TS dünyasından karşılığı:** Prisma Migrate / knex / TypeORM migrations. Kritik fark:
Prisma'da `schema.prisma` yazarsın, SQL'i **o üretir**; goose'da SQL'i **sen yazarsın**, goose
sadece sıra + kayıt tutar (bizde bilinçli tercih — SQL'i görmek dersin kendisi,
PROJECT_BRIEF §7.1). GORM `AutoMigrate` tarzı "struct'a bakıp şemayı kendiliğinden değiştiren"
sihirden aynı sebeple uzağız: migration dosyası PR diff'inde görünür, review edilir, prod'a
aynen o gider. (Dürüst not: Prisma'nın ürettiği SQL de migration dosyasına yazılır ve review
edilebilir — fark "kim yazıyor", disiplin değil.)

Altın kurallar:
- **Uygulanmış migration'ı asla düzenleme** — değişiklik = **yeni** dosya (`0002_...`).
  Yoksa defter ile gerçek şema birbirini tutmaz.
- **`down` sadece dev konforu** — prod'da geri alma da ileri giden yeni bir migration'la yapılır.
- **Şema değişikliğinin tek kapısı migration** — pgAdmin/GUI sadece bakmak ve sorgu denemek
  için; sağ-tık ile kolon eklersen migration'lara yansımaz → schema drift'i kendi elinle
  yaratırsın (taze klonda `goose up` çalıştıran senin GUI değişikliğini alamaz).

**🔗 İlgili:**
- [backend/migrations/0001_books_genres.sql](../../backend/migrations/0001_books_genres.sql) — ilk migration (satır satır açıklamalı)
- [docs/phases/phase-02-postgres-books-genres.md](../phases/phase-02-postgres-books-genres.md) adım 3 · [PROJECT_BRIEF](../PROJECT_BRIEF.md) §7.1/§7.2 (goose neden; prod'a down çekme), §18.3 (tip/kısıt/index kararları)
- [infra-and-devops.md](infra-and-devops.md) (gelince: Docker Compose Postgres — migration'ın koştuğu dev DB)
