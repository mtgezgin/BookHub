---
name: not
description: BookHub öğrenme günlüğüne (docs/learning-notes/ klasörü) yeni bir kavram girdisi ekler. Kullanıcı bir kavramı öğrendikten sonra "/not" yazınca veya bir konuyu kaydetmek istediğinde çağrılır. Son konuşulan kavramı, NEDEN sorulduğunun bağlamıyla birlikte DOĞRU KATEGORİ dosyasına yapılandırılmış bir girdi olarak ekler.
user-invocable: true
---

# /not — Öğrenme günlüğüne girdi ekle

Bu skill, `docs/learning-notes/` klasörüne **kategorize edilmiş** bir öğrenme notu ekler. Amaç:
kullanıcının (JS/TS geçmişinden Go öğrenen biri) sorduğu kavramları, **neden sorduğunun
bağlamıyla** kalıcı olarak kaydetmek; böylece sonradan dönüp hatırlayabilir.

## Ne zaman çalışır

- `/not` (argümansız) → **bu konuşmada en son sorulan/öğrenilen kavramı** al.
- `/not <konu>` → o konuyu işle (gerekirse konuşmadan ilgili açıklamayı bul).

## Adımlar

1. **Konuyu belirle.**
   - Argüman verildiyse onu kullan.
   - Verilmediyse, bu konuşmada kullanıcının en son sorduğu "bu nedir / nasıl çalışır / şu daha
     mı iyi" türü kavramı ve senin verdiğin cevabı temel al.

2. **Doğru kategori dosyasını seç.** `docs/learning-notes/README.md`'deki kategori tablosuna bak.
   Mevcut kategoriler (içerik geldikçe açılır):
   - `go-language.md` — Go dilinin kendisi: stdlib, idiomlar, test konvansiyonları, syntax, generics
   - `backend-and-web.md` — dil-bağımsız backend/web: HTTP, API, config/12-factor, server lifecycle, hata yönetimi
   - `tooling-and-workflow.md` — Claude Code (skill/memory/hook), git, pre-commit gate'ler, commit ritmi
   - `database.md` — Postgres, SQL, sqlc, migration, transaction, index
   - `security-and-auth.md` — auth, JWT, parola hash, IDOR, rate limit
   - `infra-and-devops.md` — Docker, Redis hosting, deploy, CI/CD, K8s

   Kavram hangi kategoriye giriyorsa o dosyayı kullan. **Dosya henüz yoksa oluştur** (diğer
   kategori dosyalarının başlık+İçindekiler yapısını taklit et) ve `README.md`'deki tabloda
   durumunu "aktif" yap. Hiçbir kategoriye uymuyorsa README'ye yeni bir kategori satırı ekleyip
   yeni dosya aç.

3. **Kategori dosyasını oku**, mevcut `## N. ...` girdilerini say, sonraki numarayı (`N+1`) bul.

4. **Yeni girdiyi EN ALTA ekle**, diğer girdilerle birebir aynı formatta:

   ```markdown
   ## N. <Açık, anlaşılır başlık>

   **🗓️ Bağlam:** Bunu neyi yaparken / neden sordum. (Hangi adımda, hangi koddan yola çıkarak,
   JS/TS'teki hangi alışkanlıktan dolayı sordum.)

   **⚡ Kısa cevap:** Bir-iki cümlede özü.

   **📖 Açıklama:** Sade, beginner-dostu; gerekirse küçük kod örneği veya tablo.

   **🔗 İlgili:**
   - <docs/WHY_GO_VS_TS.md §X, docs/GO_CONVENTIONS.md §Y, docs/phases/..., backend/... linkleri>
   ```

5. **O dosyanın İçindekiler (TOC)** listesini yeni girdiyle, doğru anchor linkiyle güncelle.
   (Anchor: başlığı küçük harf yap, boşlukları `-`, Türkçe karakterleri olduğu gibi bırak — mevcut
   girdilerin stiline bak.)

6. **Bittiğinde** 1-2 cümleyle ne eklediğini ve hangi kategoriye koyduğunu söyle; dosyayı commit
   etmek isteyip istemediğini sor. **Otomatik commit etme** (kullanıcı söylemeden).

## Kurallar

- **Dil:** hibrit — açıklamalar Türkçe, teknik terimler ve kod İngilizce.
- **Bağlam şart:** "🗓️ Bağlam" alanı asla boş/geçiştirme olmasın; skill'in tüm değeri burada.
- **Dürüstlük:** bir kavramda TS/JS eşit/daha iyiyse onu da yaz (WHY_GO_VS_TS felsefesi).
- **Kısa ve net:** öğretici olsun ama şişmesin.
- Sadece `docs/learning-notes/` altındaki dosyalara dokun; başka yere karışma.
