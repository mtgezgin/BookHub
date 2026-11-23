#!/usr/bin/env bash
# backup.sh — veritabanının ŞİFRELİ dökümü ve geri yüklenmesi (REVIEW_FEEDBACK F23).
#
# NEDEN: Supabase'in ücretsiz planında otomatik yedek yok ve yedek indirilemiyor. Hatalı bir migration
# ya da kod hatası canlı veriyi geri dönülemez biçimde bozabilir.
#
# NEDEN ŞİFRELİ: döküm kullanıcıların e-postalarını ve parola özetlerini içerir; GitHub artefaktı olarak
# saklanır ve repo public. Şifreleme `age` ile, repodaki AÇIK anahtarla (ops/backup/recipients.txt)
# yapılır. Açabilen tek şey GİZLİ anahtardır ve o yalnız kullanıcıdadır: ne repoda ne GitHub'da durur.
#
# NASIL: pg_dump ve age AYNI konteynerde, tek boruyla çalışır. Şifresiz döküm hiçbir anda diske
# yazılmaz, yalnız boruda akar. Bağlantı adresi konteynere ortam değişkeniyle geçer; komut satırında
# ve hata mesajlarında görünmez.
#
# Kullanım:
#   DATABASE_URL=… scripts/backup.sh dump <çıktı.dump.age>
#   TARGET_URL=…   scripts/backup.sh restore <yedek.dump.age> <gizli-anahtar-dosyası>
#   DATABASE_URL=… TARGET_URL=… scripts/backup.sh roundtrip   (CI: geçici anahtarla döküm → geri yükle → karşılaştır)
#
# PG_DOCKER_ARGS: konteynerin ağ ayarı. Varsayılan "--network host" (GitHub runner'ı). Yerelde compose
# ağındaki veritabanı için: PG_DOCKER_ARGS="--network bookhub_default" ve adreslerde host "postgres".
#
# TARGET_URL BOŞ bir veritabanı olmalı; restore onu doldurur. Canlı veritabanına restore YAPILMAZ:
# felakette yeni bir Supabase projesi açılır, oraya yüklenir (docs/DEPLOYMENT.md "Yedek").
set -euo pipefail

# pg_dump 18 eski sunucuları da döker (9.2 ve sonrası); canlıdaki Postgres sürümünden bağımsız çalışır.
# Özet (digest) ile sabit: aynı etiket başka bir imaja taşınamaz.
IMAGE="postgres:18-alpine@sha256:b6972e1115a4ff8eba5d7f7d19184926d1e16d7d1136ae1e1a7ebf286da0f015"
PG_DOCKER_ARGS="${PG_DOCKER_ARGS:---network host}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
RECIPIENTS="${RECIPIENTS:-$ROOT/ops/backup/recipients.txt}"

# hostpath: Docker'a verilecek mutlak dosya yolu. Windows'ta (Git Bash) Docker, /c/… biçimini anlamaz;
# cygpath onu C:/… biçimine çevirir. Linux'ta (CI) yol olduğu gibi kalır.
hostpath() {
	local abs
	abs="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
	if command -v cygpath >/dev/null 2>&1; then cygpath -m "$abs"; else echo "$abs"; fi
}

die() {
	echo "✗ $*" >&2
	exit 1
}

# in_container: imajda (age kurularak) bir kabuk komutu çalıştırır. Ek docker argümanları "--"dan önce.
in_container() {
	local args=()
	while [[ $# -gt 0 && "$1" != "--" ]]; do
		args+=("$1")
		shift
	done
	shift
	# shellcheck disable=SC2086 # PG_DOCKER_ARGS bilerek kelimelere bölünür
	docker run --rm -i $PG_DOCKER_ARGS "${args[@]}" "$IMAGE" sh -euc "apk add -q --no-progress age >/dev/null; $1"
}

# Döküm biçimi: özel biçim (-Fc, sıkıştırılmış, pg_restore ile seçmeli geri yüklenebilir), yalnız
# uygulamanın şeması (public). Sahip ve yetki bilgisi yok: yeni bir projede kullanıcı adları farklıdır.
# shellcheck disable=SC2016 # $DATABASE_URL konteynerin içinde açılır, burada değil
DUMP_CMD='pg_dump --format=custom --schema=public --no-owner --no-privileges "$DATABASE_URL"'

# Geri yükleme: döküm SQL'e çevrilip psql ile, ilk hatada durarak uygulanır (yarım yüklenmiş bir yedek
# "başarılı" görünmesin). İki ayrıntı:
#   - public şeması hedefte zaten var; dökümdeki "CREATE SCHEMA public" listeden çıkarılır.
#   - pg_restore 18, "SET transaction_timeout" yazar; Postgres 17'den eski bir sunucu bu ayarı tanımaz
#     (yerelde ve CI'da 16 var). Satır atılır: yalnız oturum ayarıdır, veriye dokunmaz.
# shellcheck disable=SC2016 # $TARGET_URL konteynerin içinde açılır
RESTORE_CMD='pg_restore --list /tmp/db.dump | grep -v " SCHEMA - public " >/tmp/db.list
pg_restore --no-owner --no-privileges --use-list=/tmp/db.list --file=- /tmp/db.dump   | sed "/^SET transaction_timeout/d"   | psql "$TARGET_URL" --quiet --no-psqlrc -v ON_ERROR_STOP=1 >/dev/null'

# Her tablonun satır sayısı ("tablo sayı" satırları): geri yüklemenin tam olduğunu göstermek için.
COUNTS_SQL="SELECT table_name || ' ' || (xpath('/row/c/text()', query_to_xml(format('SELECT count(*) AS c FROM public.%I', table_name), false, true, '')))[1]::text FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE' ORDER BY 1"

counts() { # counts <ORTAM_DEĞİŞKENİ_ADI>
	in_container -e "URL=${!1}" -- "psql \"\$URL\" -qtA -v ON_ERROR_STOP=1 -c \"$COUNTS_SQL\""
}

cmd_dump() {
	local out="${1:?çıktı dosyası gerekli}"
	[[ -n "${DATABASE_URL:-}" ]] || die "DATABASE_URL tanımlı değil"
	[[ -s "$RECIPIENTS" ]] || die "açık anahtar dosyası yok ya da boş: $RECIPIENTS"
	mkdir -p "$(dirname "$out")"
	# Hata çıktısı bilerek yutulur: pg_dump'ın mesajı sunucu adını ve kullanıcı adını içerebilir ve CI
	# logları herkese açık. Yerinde yalnız ne olduğunu söyleyen bir satır kalır.
	if ! in_container -e DATABASE_URL -v "$(hostpath "$RECIPIENTS"):/recipients.txt:ro" -- \
		"$DUMP_CMD 2>/tmp/err | age --encrypt --recipients-file /recipients.txt; test ! -s /tmp/err || { echo 'pg_dump uyarı ya da hata verdi' >&2; exit 1; }" \
		>"$out"; then
		rm -f "$out"
		die "döküm alınamadı (bağlantı ya da yetki; ayrıntı güvenlik için yazılmadı)"
	fi
	[[ -s "$out" ]] || die "döküm boş çıktı"
	echo "✓ şifreli döküm: $out ($(wc -c <"$out") bayt)"
}

cmd_restore() {
	local enc="${1:?şifreli yedek dosyası gerekli}" key="${2:?gizli anahtar dosyası gerekli}"
	[[ -n "${TARGET_URL:-}" ]] || die "TARGET_URL tanımlı değil (BOŞ bir veritabanı)"
	in_container -e TARGET_URL -v "$(hostpath "$enc"):/in.age:ro" \
		-v "$(hostpath "$key"):/key.txt:ro" -- \
		"age --decrypt --identity /key.txt /in.age >/tmp/db.dump; $RESTORE_CMD"
	echo "✓ geri yüklendi; tablolar ve satır sayıları:"
	counts TARGET_URL
}

cmd_roundtrip() {
	[[ -n "${DATABASE_URL:-}" && -n "${TARGET_URL:-}" ]] || die "DATABASE_URL ve TARGET_URL gerekli"
	local work
	work="$(mktemp -d)"
	trap 'rm -rf "$work"' RETURN
	# Geçici anahtar: yalnız bu sınama için üretilir ve iş bitince silinir.
	in_container -- "age-keygen 2>/dev/null" >"$work/key.txt"
	grep -o 'age1[0-9a-z]*' "$work/key.txt" >"$work/recipients.txt"
	RECIPIENTS="$work/recipients.txt" cmd_dump "$work/db.dump.age"
	if grep -aq 'CREATE TABLE' "$work/db.dump.age"; then die "döküm şifresiz görünüyor"; fi
	cmd_restore "$work/db.dump.age" "$work/key.txt" >/dev/null
	local before after
	before="$(counts DATABASE_URL)"
	after="$(counts TARGET_URL)"
	if [[ "$before" != "$after" ]]; then
		diff <(echo "$before") <(echo "$after") >&2 || true
		die "geri yüklenen veritabanı kaynakla aynı değil"
	fi
	echo "✓ döküm → şifre → çöz → geri yükle: $(wc -l <<<"$before") tablo, satır sayıları aynı"
	while IFS= read -r line; do echo "    $line"; done <<<"$before"
}

case "${1:-}" in
dump) cmd_dump "${2:-}" ;;
restore) cmd_restore "${2:-}" "${3:-}" ;;
roundtrip) cmd_roundtrip ;;
*)
	echo "kullanım: scripts/backup.sh dump <çıktı> | restore <yedek> <anahtar> | roundtrip" >&2
	exit 2
	;;
esac
