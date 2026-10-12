#!/usr/bin/env bash
# checks.sh — CLAUDE.md'deki kuralların makinenin denetlediği hâli. Git kancaları ve CI (Kurallar
# iş akışı) AYNI betiği çağırır: kural tek yerde yaşar, yerel ile CI ayrışmaz. Her kuralın testi
# scripts/checks_test.sh'ta.
#
# Kullanım (repo kökünden):
#   scripts/checks.sh staged              # pre-commit: commit'e hazırlanan dosyalar
#   scripts/checks.sh commit-msg <dosya>  # commit-msg kancası
#   scripts/checks.sh push                # pre-push: frontend değiştiyse vitest
#   scripts/checks.sh range <base> <head> # CI: bir aralıktaki commit'ler
#
# Araç yoksa: yerelde gitleaks ZORUNLU (sır kuralı), diğerleri uyarıp atlar; CI'da (range) hepsi
# zorunlu — orada kurulu olmaları iş akışının sorumluluğu.
set -euo pipefail

MODE="${1:-}"
ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

ERRORS=0
fail() {
	ERRORS=$((ERRORS + 1))
	echo "✗ $1" >&2
}
warn() { echo "⚠ $1" >&2; }

# need <araç>: araç varsa 0. Yoksa CI'da hata, yerelde uyarı (1 döner, kontrol atlanır).
need() {
	command -v "$1" >/dev/null 2>&1 && return 0
	if [[ "$MODE" == range ]]; then
		fail "$1 kurulu değil (CI'da zorunlu)"
	else
		warn "$1 kurulu değil; bu kontrol atlandı (CI yine denetler). Kur: brew install $1"
	fi
	return 1
}

# Değişen dosyalar "DURUM<TAB>yol[<TAB>yeni-yol]" biçiminde (yeniden adlandırmada iki yol).
changes() {
	case "$MODE" in
	staged) git diff --cached --name-status -M ;;
	# Üç nokta: PR'da base, dal açıldıktan sonra ilerlemiş main olabilir; yalnız dalın KENDİ
	# değişiklikleri (ortak ata → head) denetlenir.
	range) git diff --name-status -M "$BASE...$HEAD" ;;
	esac
}

# file_at <yol>: denetlenen hâl — staged'de index, range'de HEAD, ikisi de yoksa boş.
file_at() {
	case "$MODE" in
	staged) git show ":$1" 2>/dev/null || true ;;
	range) git show "$HEAD:$1" 2>/dev/null || true ;;
	esac
}

# Kural: .env ve supabase/ repoya girmez (CLAUDE.md "Güvenlik" ve "Veritabanı").
check_forbidden_paths() {
	local status path new
	while IFS=$'\t' read -r status path new; do
		[[ "$status" == D* ]] && continue
		[[ -n "${new:-}" ]] && path="$new"
		case "$path" in
		supabase/* | */supabase/*)
			fail "yasak yol: $path — Supabase'in GitHub entegrasyonu supabase/ klasörünü canlı veritabanına uygulayabilir" ;;
		esac
		local base="${path##*/}"
		if [[ "$base" == .env || ("$base" == .env.* && "$base" != .env.example) ]]; then
			fail "yasak yol: $path — sırlar repoya girmez; yalnız .env.example (herkese açık dev değerleri)"
		fi
	done < <(changes)
}

# Kural: canlıya çıkmış (release'teki) migration değişmez, silinmez, yeniden adlandırılmaz.
check_migrations() {
	if ! git rev-parse --verify -q origin/release >/dev/null; then
		if [[ "$MODE" == range ]]; then
			fail "origin/release yok: migration dokunulmazlığı denetlenemedi (CI release dalını çekmeli)"
		else
			warn "origin/release yok; migration dokunulmazlığı atlandı (git fetch origin release)"
		fi
		return
	fi
	local status path new
	while IFS=$'\t' read -r status path new; do
		[[ "$path" == backend/migrations/*.sql ]] || continue
		case "$status" in
		M* | D* | R*)
			if git cat-file -e "origin/release:$path" 2>/dev/null; then
				fail "canlıya çıkmış migration değişmez: $path (${status:0:1}) — düzeltme yeni bir migration'dır"
			fi
			;;
		esac
	done < <(changes)
}

# Kural: commit mesajında atlama bayrağı olmaz; Northflank aynı push'taki tek bir bayrak yüzünden
# bütün yayını atlar. Git'in yorum satırları (#) mesaja girmez.
SKIP_FLAGS='\[(skip ci|ci skip|no ci|skip actions|actions skip|skip nf|nf skip|northflank skip|skip northflank)\]'
check_commit_message() {
	if grep -v '^#' "$1" | grep -qiE "$SKIP_FLAGS"; then
		fail "commit mesajında atlama bayrağı var ($(grep -v '^#' "$1" | grep -oiE "$SKIP_FLAGS" | head -1)) — Northflank yayını atlar"
	fi
}

# Kural: go.mod ile Dockerfile'daki Go sürümü aynı yamadadır (CI ve Deploy go.mod'dan, canlı binary
# imajdan derlenir; ayrışırlarsa test edilen ile çıkan farklı Go olur).
check_go_version() {
	local mod docker
	mod="$(file_at backend/go.mod | sed -nE 's/^go ([0-9.]+)$/\1/p')"
	docker="$(file_at backend/Dockerfile | sed -nE 's/^FROM golang:([0-9.]+)-alpine.*/\1/p' | head -1)"
	[[ -z "$mod" || -z "$docker" ]] && return
	if [[ "$mod" != "$docker" ]]; then
		fail "Go sürümü ayrışmış: go.mod $mod, Dockerfile golang:$docker — ikisini birlikte güncelle"
	fi
}

# Kural: sır commit'lenmez.
check_secrets() {
	if ! command -v gitleaks >/dev/null 2>&1; then
		fail "gitleaks kurulu değil; sır taraması zorunlu. Kur: brew install gitleaks"
		return
	fi
	local out
	case "$MODE" in
	staged) out="$(gitleaks git --pre-commit --staged --redact --no-banner . 2>&1)" || fail "gitleaks sır buldu:"$'\n'"$out" ;;
	range) out="$(gitleaks git --log-opts="$BASE..$HEAD" --redact --no-banner . 2>&1)" || fail "gitleaks sır buldu:"$'\n'"$out" ;;
	esac
}

changed_matching() { changes | cut -f2- | tr '\t' '\n' | grep -E "$1" || true; }

# sqlc'nin ürettiği kod sorgularla, go.mod/go.sum bağımlılıklarla uyumlu mu.
check_generated() {
	if [[ -f backend/sqlc.yaml && -n "$(changed_matching '^backend/(sql/|migrations/|internal/db/|sqlc\.yaml)')" ]]; then
		if need sqlc; then (cd backend && sqlc diff) >/dev/null 2>&1 || fail "sqlc çıktısı güncel değil — backend'de 'sqlc generate' çalıştır"; fi
	fi
	if [[ -f backend/go.sum && -n "$(changed_matching '^backend/go\.(mod|sum)$')" ]]; then
		if need go; then (cd backend && go mod tidy -diff) >/dev/null 2>&1 || fail "go.mod/go.sum düzenli değil — backend'de 'go mod tidy' çalıştır"; fi
	fi
}

# İş akışı ve shell betiği lint'i.
check_lint() {
	if [[ -n "$(changed_matching '^\.github/(workflows/.*\.ya?ml|actionlint\.yaml)$')" ]]; then
		if need actionlint; then actionlint || fail "actionlint bulgu verdi"; fi
	fi
	local scripts
	scripts="$(changed_matching '(\.sh$|^\.githooks/)' | while read -r f; do [[ -f "$f" ]] && echo "$f"; done)"
	if [[ -n "$scripts" ]]; then
		# shellcheck disable=SC2086 # dosya adları boşluksuz
		if need shellcheck; then shellcheck $scripts || fail "shellcheck bulgu verdi"; fi
	fi
}

# Frontend tip kontrolü (commit'te; testler pre-push'ta ve CI'da).
check_frontend_types() {
	[[ -f frontend/package.json && -n "$(changed_matching '^frontend/')" ]] || return 0
	if [[ ! -d frontend/node_modules ]]; then
		warn "frontend/node_modules yok; tip kontrolü atlandı (cd frontend && npm ci)"
		return
	fi
	npm --prefix frontend run -s typecheck || fail "frontend tip kontrolü (tsc) başarısız"
}

case "$MODE" in
staged)
	check_forbidden_paths
	check_migrations
	check_go_version
	check_secrets
	check_generated
	check_lint
	check_frontend_types
	;;
commit-msg)
	check_commit_message "${2:?commit mesajı dosyası gerekli}"
	;;
push)
	base="$(git merge-base HEAD origin/main 2>/dev/null || true)"
	if [[ -n "$base" && -f frontend/package.json && -n "$(git diff --name-only "$base" HEAD -- frontend/)" ]]; then
		if [[ -d frontend/node_modules ]]; then
			npm --prefix frontend test --silent || fail "frontend testleri (vitest) başarısız"
		else
			warn "frontend/node_modules yok; vitest atlandı (cd frontend && npm ci)"
		fi
	fi
	;;
range)
	BASE="${2:?base gerekli}"
	HEAD="${3:?head gerekli}"
	msgs="$(mktemp)"
	git log --format=%B "$BASE..$HEAD" >"$msgs"
	check_commit_message "$msgs"
	rm -f "$msgs"
	check_forbidden_paths
	check_migrations
	check_go_version
	check_secrets
	check_generated
	check_lint
	;;
*)
	echo "kullanım: scripts/checks.sh staged | commit-msg <dosya> | push | range <base> <head>" >&2
	exit 2
	;;
esac

if [[ "$ERRORS" -gt 0 ]]; then
	echo "$ERRORS kural ihlali. Ayrıntı: scripts/checks.sh ve CLAUDE.md" >&2
	exit 1
fi
