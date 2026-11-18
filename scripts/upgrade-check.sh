#!/usr/bin/env bash
# upgrade-check.sh — canlıdaki ŞEMA ve VERİ üzerine bu commit'in migration'larını uygular
# (REVIEW_FEEDBACK F38).
#
# NEDEN: testler her koşuda BOŞ bir veritabanı kurar; canlıda ise migration DOLU tabloya uygulanır.
# Boş tabloda geçen bir migration dolu tabloda düşebilir (NOT NULL eklenen kolon, benzersizlik ihlali,
# veri taşıyan UPDATE). Bu betik yayındaki sırayı taklit eder:
#   1. release dalının (canlıdaki kodun) migration'ları   → canlının şeması
#   2. release dalının demo verisi                        → dolu tablolar
#   3. bu commit'in migration'ları                        → yayında olacak olan
# 3. adım düşerse PR kırmızı olur; hata yayında değil burada görülür.
#
# Kullanım: DATABASE_URL=postgres://… scripts/upgrade-check.sh
#   Veritabanı BOŞ olmalı ve betiğe ayrılmalı (CI'da işin kendi Postgres'i). Canlıya karşı ÇALIŞTIRILMAZ.
#   PSQL: psql komutu (varsayılan "psql"); yerelde ör. PSQL="docker exec -i bookhub-postgres psql".
set -euo pipefail

: "${DATABASE_URL:?DATABASE_URL gerekli (boş, bu betiğe ayrılmış bir veritabanı)}"
PSQL="${PSQL:-psql}"
root="$(git rev-parse --show-toplevel)"
cd "$root"

if ! git rev-parse --verify -q origin/release >/dev/null; then
	echo "origin/release yok: yükseltme denenemedi (git fetch origin release)" >&2
	exit 1
fi

sql() { $PSQL "$DATABASE_URL" -v ON_ERROR_STOP=1 -qtA "$@"; }

if [[ "$(sql -c "SELECT count(*) FROM pg_tables WHERE schemaname = 'public'")" != "0" ]]; then
	echo "veritabanı boş değil: bu betik yalnız kendisine ayrılmış boş bir veritabanında çalışır" >&2
	exit 1
fi

work="$(mktemp -d)"
cleanup() {
	git worktree remove --force "$work/release" >/dev/null 2>&1 || true
	rm -rf "$work"
}
trap cleanup EXIT

echo "▶ 1/3 canlının şeması (origin/release @ $(git rev-parse --short origin/release))"
git worktree add --detach "$work/release" origin/release >/dev/null
go -C "$work/release/backend" run ./cmd/manage migrate
released="$(sql -c "SELECT max(version_id) FROM goose_db_version")"

echo "▶ 2/3 demo veri (release'teki seed)"
sql -f - <"$work/release/backend/scripts/demo_seed.sql" >/dev/null
books_before="$(sql -c "SELECT count(*) FROM books")"
if [[ "$books_before" == "0" ]]; then
	echo "demo veri yüklenmedi: dolu tablo olmadan yükseltme sınanmış olmaz" >&2
	exit 1
fi

echo "▶ 3/3 bu commit'in migration'ları (HEAD @ $(git rev-parse --short HEAD))"
go -C backend run ./cmd/manage migrate
current="$(sql -c "SELECT max(version_id) FROM goose_db_version")"
books_after="$(sql -c "SELECT count(*) FROM books")"

# Bu commit'teki en büyük migration numarası uygulanmış olmalı (goose sessizce atlamış olmasın).
want="$(git ls-files 'backend/migrations/*.sql' | sed -nE 's|^backend/migrations/0*([0-9]+)_.*$|\1|p' | sort -n | tail -n 1)"
if [[ "$current" != "$want" ]]; then
	echo "şema sürümü $current, beklenen $want: migration'ların tamamı uygulanmadı" >&2
	exit 1
fi
if [[ "$books_after" != "$books_before" ]]; then
	echo "kitap sayısı değişti ($books_before → $books_after): migration veri kaybettirdi ya da çoğalttı" >&2
	exit 1
fi
echo "✓ yükseltme: şema $released → $current, $books_after kitap yerinde"
