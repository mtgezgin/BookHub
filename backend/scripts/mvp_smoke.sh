#!/usr/bin/env bash
# mvp_smoke.sh — PROJECT_BRIEF §15 "MVP Definition of Done"un çalıştırılabilir kanıtı.
#
# Çalışan bir API'ye karşı tüm MVP akışını uçtan uca dener ve her DoD maddesini ✓/✗ basar.
# Kendi admin'ini ve kullanıcısını açar (rastgele parola, hiçbir dosyaya yazılmaz) ve sonunda
# oluşturduğu kullanıcıları siler (kitap/tür kayıtları bilinçli kalır — gözle bakabilesin).
#
# Kullanım (repo kökünden, API çalışırken):
#   set -a; source .env; set +a
#   API=http://localhost:8080 backend/scripts/mvp_smoke.sh
#
# Gerekenler: curl, python3 (JSON için), go (create-admin komutu için), docker (temizlik için).
set -euo pipefail

API="${API:-http://localhost:8080}"
: "${DATABASE_URL:?DATABASE_URL gerekli (set -a; source .env; set +a)}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TS="$(date +%s)"
ADMIN_EMAIL="smoke-admin-$TS@example.com"
USER_EMAIL="smoke-user-$TS@example.com"
ADMIN_PW="$(openssl rand -base64 24)"
USER_PW="$(openssl rand -base64 24)"
PASS=0
FAIL=0

ok()   { echo "  ✓ $1"; PASS=$((PASS + 1)); }
bad()  { echo "  ✗ $1"; FAIL=$((FAIL + 1)); }
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print($1)"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (beklenen $3, gelen $2)"; fi; }

# req METHOD PATH [TOKEN] [BODY] → cevap gövdesi $BODY'de, HTTP kodu $CODE'da.
# BİLEREK $(...) içinde çağrılmaz: komut ikamesi bir ALT KABUK açar ve orada atanan
# değişkenler (CODE) çağırana geri dönmez — betiğin ilk sürümündeki hata buydu.
req() {
	local method="$1" path="$2" token="${3:-}" body="${4:-}" out
	out="$(mktemp)"
	CODE="$(curl -s -o "$out" -w '%{http_code}' -X "$method" "$API$path" \
		${token:+-H "Authorization: Bearer $token"} ${body:+-d "$body"})"
	BODY="$(cat "$out")"
	rm -f "$out"
}
field() { printf '%s' "$BODY" | json "$1"; } # son cevaptan alan oku

cleanup() {
	docker exec bookhub-postgres psql -U bookhub -d "$(basename "${DATABASE_URL%%\?*}")" -qtAc \
		"DELETE FROM users WHERE email IN ('$ADMIN_EMAIL', '$USER_EMAIL')" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "BookHub MVP smoke testi → $API"

echo "1) Kullanıcılar kayıt olup giriş yapabilir"
req POST /api/auth/register "" "{\"email\":\"$USER_EMAIL\",\"display_name\":\"Smoke Okur\",\"password\":\"$USER_PW\"}"
check "register → 201" "$CODE" 201
req POST /api/auth/login "" "{\"email\":\"$USER_EMAIL\",\"password\":\"$USER_PW\"}"
USER_TOKEN="$(field 'd["access_token"]')"
check "login → 200" "$CODE" 200

echo "2) Admin var (cmd/manage create-admin)"
printf '%s\n' "$ADMIN_PW" | go -C "$ROOT" run ./cmd/manage create-admin -email "$ADMIN_EMAIL" -name "Smoke Admin" >/dev/null
req POST /api/auth/login "" "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PW\"}"
ADMIN_TOKEN="$(field 'd["access_token"]')"
check "admin login → 200" "$CODE" 200
req GET /api/auth/me "$ADMIN_TOKEN"
ROLE="$(field 'd["role"]')"
check "admin rolü" "$ROLE" ADMIN

echo "3) Admin tür oluşturabilir"
req POST /api/admin/genres "$ADMIN_TOKEN" "{\"name\":\"Smoke Türü $TS\"}"
GENRE_ID="$(field 'd["id"]')"
check "tür → 201" "$CODE" 201
req POST /api/admin/genres "$USER_TOKEN" '{"name":"Yetkisiz"}'
check "USER tür açamaz → 403" "$CODE" 403

echo "4) Kullanıcılar kitap önerebilir"
req POST /api/books/suggestions "$USER_TOKEN" "{\"title\":\"Smoke Kitabı $TS\",\"author\":\"Smoke Yazar\"}"
BOOK_ID="$(field 'd["id"]')"
check "öneri → 201" "$CODE" 201
req GET "/api/books/$BOOK_ID"
check "onay öncesi public'te yok → 404" "$CODE" 404

echo "5-6) Admin onaylar/reddeder; onaylı kitap herkese açık"
req POST "/api/admin/books/$BOOK_ID/approve" "$ADMIN_TOKEN"
check "onay → 200" "$CODE" 200
req PUT "/api/admin/books/$BOOK_ID/genres" "$ADMIN_TOKEN" "{\"genre_ids\":[$GENRE_ID]}"
check "tür bağlama → 200" "$CODE" 200
req GET "/api/books/$BOOK_ID"
check "onay sonrası public → 200" "$CODE" 200
req POST /api/books/suggestions "$USER_TOKEN" "{\"title\":\"Reddedilecek $TS\",\"author\":\"X\"}"
REJ_ID="$(field 'd["id"]')"
req POST "/api/admin/books/$REJ_ID/reject" "$ADMIN_TOKEN" '{"reason":"smoke testi"}'
REJ="$(field 'd["status"]')"
check "red → REJECTED" "$REJ" REJECTED

echo "7-8) Kullanıcılar yorum yapar; yorumu yalnız admin siler"
req POST "/api/books/$BOOK_ID/comments" "$USER_TOKEN" '{"content":"Smoke yorumu"}'
COMMENT_ID="$(field 'd["id"]')"
check "yorum → 201" "$CODE" 201
req DELETE "/api/admin/comments/$COMMENT_ID" "$USER_TOKEN"
check "USER yorum silemez → 403" "$CODE" 403
req DELETE "/api/admin/comments/$COMMENT_ID" "$ADMIN_TOKEN"
check "admin siler → 204" "$CODE" 204
req GET "/api/books/$BOOK_ID/comments"
TOTAL="$(field 'd["total"]')"
check "silinen yorum görünmez" "$TOTAL" 0

echo "9-10) 1-10 puan; ortalama ve sayı hesaplanır"
req PUT "/api/books/$BOOK_ID/rating" "$USER_TOKEN" '{"score":11}'
check "puan 11 → 400" "$CODE" 400
req PUT "/api/books/$BOOK_ID/rating" "$USER_TOKEN" '{"score":9}'
check "ilk puan → 201" "$CODE" 201
req PUT "/api/books/$BOOK_ID/rating" "$ADMIN_TOKEN" '{"score":6}'
req GET "/api/books/$BOOK_ID/ratings/summary"
STATS="$(field 'str(d["average_rating"]) + "/" + str(d["rating_count"])')"
check "ortalama/sayı" "$STATS" "7.5/2"

echo "11-12) Ağırlıklı skor hesaplanır; top-rated onu kullanır"
# Farklı ortalamalı ikinci bir kitap: katalogda oylu TEK kitap varsa C = o kitabın
# ortalaması olur ve ağırlıklı skor basit ortalamaya eşit çıkar (Faz 6 dersi) — ayırt edilemez.
req POST /api/admin/books "$ADMIN_TOKEN" "{\"title\":\"Smoke Karşılaştırma $TS\",\"author\":\"Y\"}"
OTHER_ID="$(field 'd["id"]')"
req PUT "/api/books/$OTHER_ID/rating" "$USER_TOKEN" '{"score":3}'
req GET "/api/books/top-rated?limit=50"
TOP="$(printf '%s' "$BODY" | python3 -c "
import json,sys
d=json.load(sys.stdin)
m,c=d['m'],d['c']
it=[i for i in d['items'] if i['id']==$BOOK_ID]
if not it: print('yok'); sys.exit()
b=it[0]; v=b['rating_count']; r=b['average_rating']
want=round((v/(v+m))*r+(m/(v+m))*c,4)
print('ok' if abs(want-b['weighted_score'])<0.001 and b['weighted_score']!=r else f'{want}!={b[\"weighted_score\"]}')")"
check "weighted_score = Bayes formülü (basit ortalama değil)" "$TOP" ok

echo "13-15) PostgreSQL + Go + lokal"
req GET /health
H="$(field 'd["status"]')"
check "/health" "$H" ok

echo
echo "Sonuç: $PASS geçti, $FAIL başarısız"
[ "$FAIL" -eq 0 ]
