#!/usr/bin/env bash
# setup-github.sh — süreç için BİR KERELİK GitHub kurulumu (docs/PROCESS.md §9).
#
# Ne yapar (ikisi de tekrar çalıştırılabilir):
#   1. Etiketleri oluşturur ya da günceller: tür, öncelik, triage.
#   2. `release` dalı GitHub'da yoksa, ŞU AN CANLIDA OLAN commit'te oluşturur (varsayılan:
#      origin/main). Yeni bir şey push etmeden ve Northflank'ı `release`'e çevirmeden ÖNCE
#      çalıştır; yoksa release canlıdakinden farklı bir commit'te doğar.
#
# Kullanım (repo kökünden):  scripts/setup-github.sh [canlıdaki-commit]
# Gerekenler: gh (giriş yapılmış, repo yetkili), git.
set -euo pipefail

cd "$(dirname "$0")/.."

label() {
	local name="$1" color="$2" description="$3"
	gh label create "$name" --color "$color" --description "$description" --force >/dev/null
	echo "  ✓ etiket: $name"
}

echo "▶ etiketler"
label feature 1d76db "Yeni davranış ya da iyileştirme"
label bug d73a4a "Beklenmeyen davranış"
label needs-triage fbca04 "Karar (verdict) bekliyor"
label incident b60205 "Canlıda olay: postmortem ve önleme kuralı"
label security 5319e7 "Güvenlik işi ya da bulgusu"
label tech-debt c5def5 "Teknik borç"
label research 0e8a16 "Araştırma; çıktısı bir karar"
label P0 b60205 "Olmazsa olmaz / acil"
label P1 d93f0b "Önemli"
label P2 fef2c0 "İyi olur"

echo "▶ release dalı"
# Doğrudan GitHub'a sor (yereldeki origin/release kaydı silinmiş bir dalı hâlâ gösterebilir).
# Tam ref adı: kısa ad "release", "hotfix/release" gibi başka dallarla da eşleşirdi.
remote_sha="$(git ls-remote origin refs/heads/release | cut -f1)"
if [[ -n "$remote_sha" ]]; then
	echo "  ✓ zaten var: release → ${remote_sha:0:7}"
else
	git fetch --quiet origin
	target="${1:-origin/main}"
	sha="$(git rev-parse --verify "$target^{commit}")"
	git push origin "$sha:refs/heads/release"
	echo "  ✓ oluşturuldu: release → $(git rev-parse --short "$sha")"
fi

cat <<'EOF'

Sıradaki elle adımlar (docs/PROCESS.md §9 — sırayla):
  2. Northflank: api ve web servislerinin derlendiği dalı main → release yap (Build options).
  3. Migration'lar için GitHub secret'ı (değer ekranda görünmez):
       security find-generic-password -s bookhub-prod-dsn -w | tr -d '\n' | gh secret set PROD_DATABASE_URL
  4. (İsteğe bağlı) GitHub → Settings → General → Issues: "Auto-close issues with merged linked
     pull requests" kapalı. Northflank: commit mesajı atlama bayrakları kapalı.
  5. Commit + push. CI yeşil → Deploy yeşil → Northflank yayınlar → PROCESS.md §6 doğrulaması.
  6. Pano: GitHub → Projects → New project → Board; Status sütunları PROCESS.md §1'deki
     statüler; Workflows → Auto-add (is:issue is:open) ve "Item added → Status: Backlog".
EOF
