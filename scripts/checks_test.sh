#!/usr/bin/env bash
# checks_test.sh — scripts/checks.sh'teki kuralların testleri. Her senaryo geçici bir git deposu
# kurar; her kural için "yakalamalı" ve "geçmeli" durumu denenir. CI (Kurallar işi) de koşar.
#
# Kullanım (repo kökünden):  scripts/checks_test.sh
set -euo pipefail

CHECKS="$(cd "$(dirname "$0")" && pwd)/checks.sh"
PASS=0
FAIL=0
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

ok() { PASS=$((PASS + 1)); echo "  ✓ $1"; }
bad() { FAIL=$((FAIL + 1)); echo "  ✗ $1"; }

# expect <pass | fail:<beklenen mesaj parçası>> <ad> <komut...>: komutu çalıştırır. "fail" yalnız sıfır
# olmayan çıkış değil, kuralın kendi mesajını da ister: çöken betik ya da başka bir kural geçmez.
expect() {
	local want="$1" name="$2"
	shift 2
	local out code=0
	out="$("$@" 2>&1)" || code=$?
	if [[ "$want" == pass && "$code" -eq 0 ]] ||
		[[ "$want" == fail:* && "$code" -ne 0 && "$out" == *"${want#fail:}"* ]]; then
		ok "$name"
	else
		bad "$name (beklenen $want, çıkış $code)"
		printf '%s\n' "$out" | sed 's/^/      /'
	fi
}

# Taze bir depo: release'te bir migration, go.mod ile Dockerfile aynı Go sürümünde.
new_repo() {
	local dir="$WORK/$1"
	mkdir -p "$dir"
	cd "$dir"
	git init -q -b main
	git config user.email test@example.com
	git config user.name test
	mkdir -p backend/migrations
	printf 'module x\n\ngo 1.27.2\n' >backend/go.mod
	printf 'FROM golang:1.27.2-alpine@sha256:abc AS build\n' >backend/Dockerfile
	printf -- '-- +goose Up\nCREATE TABLE a (id int);\n' >backend/migrations/0001_a.sql
	printf 'örnek\n' >README.md
	git add -A
	git commit -qm "ilk"
	git update-ref refs/remotes/origin/release HEAD
}

staged() { "$CHECKS" staged; }

echo "▶ yasak yollar"
new_repo forbidden
printf 'JWT_SECRET=x\n' >.env
git add -f .env
expect fail:"yasak yol" ".env commit'e giremez" staged
git rm -q --cached .env
printf 'A=1\n' >.env.local
git add -f .env.local
expect fail:"yasak yol" ".env.local da giremez" staged
git rm -q --cached .env.local
printf 'A=1\n' >.env.example
git add .env.example
expect pass ".env.example girebilir" staged
git reset -q
mkdir -p supabase
printf 'x\n' >supabase/config.toml
git add supabase/config.toml
expect fail:"yasak yol" "supabase/ klasörü giremez" staged

echo "▶ migration dokunulmazlığı"
new_repo migrations
printf 'ALTER TABLE a ADD b int;\n' >>backend/migrations/0001_a.sql
git add backend/migrations/0001_a.sql
expect fail:"canlıya çıkmış migration" "release'teki migration değiştirilemez" staged
git checkout -q -- . && git reset -q --hard HEAD
git rm -q backend/migrations/0001_a.sql
expect fail:"canlıya çıkmış migration" "release'teki migration silinemez" staged
git reset -q --hard HEAD
git mv backend/migrations/0001_a.sql backend/migrations/0001_b.sql
expect fail:"canlıya çıkmış migration" "release'teki migration yeniden adlandırılamaz" staged
git reset -q --hard HEAD
printf -- '-- +goose Up\nCREATE TABLE c (id int);\n' >backend/migrations/0002_c.sql
git add backend/migrations/0002_c.sql
expect pass "yeni migration eklenebilir" staged
git commit -qm "0002"
printf 'CREATE INDEX i ON c (id);\n' >>backend/migrations/0002_c.sql
git add backend/migrations/0002_c.sql
expect pass "henüz release'te olmayan migration düzeltilebilir" staged
git reset -q --hard HEAD
printf -- '-- +goose Up\nCREATE TABLE e (id int);\n' >backend/migrations/0001_e.sql
git add backend/migrations/0001_e.sql
expect fail:"numarası canlıda kullanılmış" "canlıdaki bir migration numarası yeniden kullanılamaz" staged
git rm -q --cached backend/migrations/0001_e.sql && rm backend/migrations/0001_e.sql
printf -- '-- +goose Up\nCREATE TABLE e (id int);\n' >backend/migrations/001_e.sql
git add backend/migrations/001_e.sql
expect fail:"numarası canlıda kullanılmış" "baştaki sıfırlar numarayı değiştirmez (001 = 0001)" staged

echo "▶ commit mesajı atlama bayrakları"
new_repo commitmsg
msg="$WORK/msg"
printf 'fix: bir şey [skip ci]\n' >"$msg"
expect fail:"atlama bayrağı" "[skip ci] reddedilir" "$CHECKS" commit-msg "$msg"
printf 'fix: bir şey\n\nAyrıntı [Skip NF]\n' >"$msg"
expect fail:"atlama bayrağı" "gövdedeki [Skip NF] büyük/küçük harf fark etmeden reddedilir" "$CHECKS" commit-msg "$msg"
printf 'docs: skip ci bayrağını anlat (köşeli parantezsiz)\n' >"$msg"
expect pass "köşeli parantezsiz söz geçer" "$CHECKS" commit-msg "$msg"
printf 'fix: bir şey\n# [skip ci] yorum satırı, mesaja girmez\n' >"$msg"
expect pass "git yorum satırları sayılmaz" "$CHECKS" commit-msg "$msg"
printf 'docs: kural notu\n# ------------------------ >8 ------------------------\n+bir satır [skip ci]\n' >"$msg"
expect pass "git commit -v: makas çizgisinin altındaki diff sayılmaz" "$CHECKS" commit-msg "$msg"
printf 'fix: bir şey\n\n\nskip-checks: true\n' >"$msg"
expect fail:"atlama bayrağı" "GitHub'ın skip-checks satırı reddedilir" "$CHECKS" commit-msg "$msg"
{ printf 'fix: bir şey [skip ci]\n'; head -c 200000 /dev/zero | tr '\0' 'a' | fold -w 80; } >"$msg"
expect fail:"atlama bayrağı" "uzun mesajda da baştaki bayrak bulunur (SIGPIPE yok)" "$CHECKS" commit-msg "$msg"

echo "▶ Go sürüm kayması"
new_repo goversion
expect pass "go.mod ile Dockerfile aynı yama" staged
printf 'module x\n\ngo 1.27.3\n' >backend/go.mod
git add backend/go.mod
expect fail:"Go sürümü ayrışmış" "go.mod ile Dockerfile ayrışırsa durur" staged
printf 'FROM golang:1.27.3-alpine@sha256:def AS build\n' >backend/Dockerfile
git add backend/Dockerfile
expect pass "ikisi birlikte yükselince geçer" staged
printf 'FROM --platform=linux/amd64 golang:1.27.3-bookworm AS build\n' >backend/Dockerfile
git add backend/Dockerfile
expect pass "farklı imaj türü ve --platform da okunur" staged
printf 'FROM golang:latest AS build\n' >backend/Dockerfile
git add backend/Dockerfile
expect fail:"golang:<sürüm> satırı bulunamadı" "sürüm okunamazsa kontrol sessizce kapanmaz" staged

echo "▶ sır taraması"
if command -v gitleaks >/dev/null 2>&1; then
	new_repo secrets
	# Gerçek görünümlü ama sahte bir GitHub token'ı (her koşuda rastgele).
	printf 'token = "ghp_%s"\n' "$(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 36)" >config.txt
	git add config.txt
	expect fail:"gitleaks sır buldu" "sır içeren dosya commit'e giremez" staged
	git reset -q && rm config.txt
	printf 'merhaba\n' >notes.txt
	git add notes.txt
	expect pass "sırsız dosya geçer" staged
else
	bad "gitleaks kurulu değil: sır taraması test edilemedi (kur: brew install gitleaks)"
fi

echo "▶ silinen betik"
new_repo deleted
mkdir -p scripts
printf '#!/usr/bin/env bash\necho eski\n' >scripts/old.sh
git add scripts/old.sh && git commit -qm "betik"
git rm -q scripts/old.sh
expect pass "silinen .sh dosyası commit'i durdurmaz" staged
git reset -q --hard HEAD
git mv scripts/old.sh scripts/old.txt
expect pass "adı .sh olmaktan çıkan dosya commit'i durdurmaz" staged

echo "▶ aralık (CI) modu"
new_repo range
base="$(git rev-parse HEAD)"
printf 'a\n' >a.txt && git add a.txt && git commit -qm "feat: a [ci skip]"
expect fail:"atlama bayrağı" "aralıktaki bir commit mesajında bayrak varsa durur" "$CHECKS" range "$base" HEAD
new_repo range2
base="$(git rev-parse HEAD)"
printf 'ALTER TABLE a ADD b int;\n' >>backend/migrations/0001_a.sql
git add -A && git commit -qm "fix: migration'ı düzelt"
expect fail:"canlıya çıkmış migration" "aralıkta release'teki migration değişmişse durur" "$CHECKS" range "$base" HEAD
new_repo range3
base="$(git rev-parse HEAD)"
printf -- '-- +goose Up\nCREATE TABLE d (id int);\n' >backend/migrations/0002_d.sql
git add -A && git commit -qm "feat: yeni tablo"
expect pass "temiz bir aralık geçer" "$CHECKS" range "$base" HEAD

echo
echo "$PASS geçti, $FAIL kaldı"
[[ "$FAIL" -eq 0 ]]
