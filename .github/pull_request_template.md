<!--
Süreç: docs/PROCESS.md — bu şablon "bitti tanımı"nın PR'daki hâli.
Başlık Conventional Commits biçiminde: feat(api): …, fix(web): …, docs: …
-->

## Ne ve neden

<!--
Bir-iki cümle. İlgili issue: Refs #…
"Closes/Fixes #…" YAZMA ve issue'yu kenar çubuğundan bağlama: ikisi de issue'yu merge anında
kapatır. Issue, yayından sonraki doğrulamayla (PROCESS.md §6) elle kapatılır.
-->

## Kabul kriterleri → doğrulama

| # | Kabul kriteri (issue'dan) | Nasıl doğrulandı (test adı ya da manuel adım) |
|---|---|---|
| 1 |  |  |

## Doğrulandı / doğrulanamadı

- **Doğrulandı:**
- **Doğrulanamadı** (neden; ne zaman ve nasıl doğrulanacak):

## Veritabanı

- [ ] Migration yok
- [ ] Yalnız **yeni** migration dosyası var; canlıya çıkmış bir dosya değiştirilmedi, silinmedi, numarası yeniden kullanılmadı
- [ ] Migration **şu an canlıda çalışan kodla geriye uyumlu**. Yayın akışı migration'ı yeni kod çıkmadan önce uyguluyor: önce genişlet → yayınla → sonra daralt.
- [ ] Daraltma (kolon/tablo silme, yeniden adlandırma) varsa: önceki genişletme yayını canlıda ve doğrulandı (commit: …)

## Yayın ve geri dönüş

<!-- Risk ne? Yanlış giderse nasıl geri alınır (revert, ayar)? Yayından sonra canlıda neye bakılacak? -->

## Ekran görüntüleri

<!-- Arayüz değiştiyse önce / sonra. -->

## Kontrol listesi

- [ ] CI yeşil (Backend, Frontend, İmaj taraması, E2E)
- [ ] Push kapıları (güvenlik incelemesi, AI review, test coverage) Claude Code üzerinden koştu — koşmadıysa nedeni: …
- [ ] Yeni ya da değişen davranışın testi var
- [ ] Go değiştiyse [GO_CONVENTIONS hızlı kontrol listesi](https://github.com/mtgezgin/BookHub/blob/main/docs/GO_CONVENTIONS.md#quick-checklist-paste-into-each-prcommit-description) gözden geçirildi
- [ ] Sır, `.env`, parola, token ya da kişisel veri yok
- [ ] Commit mesajlarında atlama bayrağı (köşeli parantez içinde skip ci, skip nf vb.) yok (Northflank yayını atlar)
- [ ] Gerekiyorsa dokümanlar güncellendi (DEPLOYMENT, STATUS, öğrenme notu)
