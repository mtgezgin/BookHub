#!/usr/bin/env bash
# e2e.sh — Playwright uçtan uca testlerini GERÇEK yığına karşı koşturur (Faz 13).
#
# Ne yapar: yığını kaldırır (NGINX + API + Postgres + Redis), demo kataloğu basar,
# bu koşuya özel rastgele parolalı bir admin açar, testleri koşar ve ÇIKARKEN bu koşunun
# açtığı kullanıcıları siler.
#
# Kullanım (repo kökünden):  make e2e   ya da   scripts/e2e.sh [playwright argümanları]
#   scripts/e2e.sh --headed              # tarayıcıyı gör (TAM chromium ister — aşağıya bak)
#   scripts/e2e.sh e2e/reuse.spec.ts     # tek dosya
#
# Gerekenler: docker, make, node/npx, openssl. Tarayıcı bir kereliğine:
#   cd frontend && npx playwright install --only-shell chromium   # headless koşu (CI de bunu kurar)
# --headed YALNIZ tam derlemeyle çalışır (headless-shell'de "Executable doesn't exist" verir):
#   cd frontend && npx playwright install chromium                # ~100 MB fazladan, sadece hata ayıklamak için
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# .env: JWT_SECRET ve DATABASE_URL buradan gelir (compose de aynı dosyayı okur).
set -a
# shellcheck disable=SC1091
. ./.env
set +a

# .env'den SONRA export ediliyor: dosyada bir değer olsa bile e2e kotasını BİZ belirleriz.
# NEDEN 1000: API'nin kendi IP başına auth sınırı varsayılan 10/dk ve bu koşu tek IP'den
# (host → 172.28.0.1) bir dizi giriş yapıyor. NGINX'in kovası (login|register, 20r/m
# burst 10) imajın içinde olduğu için yükseltilemez — asıl bütçe odur ve testler 11
# isteğin altında kalacak şekilde yazıldı (frontend/e2e/helpers.ts'teki tablo).
export AUTH_RATE_LIMIT_PER_MIN=1000

# Koşu kimliği: bu koşunun ürettiği her kullanıcı/kitap adının içinde yer alır.
E2E_RUN_ID="$(date +%s)"
export E2E_RUN_ID

# Admin parolası rastgele ve YALNIZ ortam değişkeninde: hiçbir dosyaya yazılmaz, komut
# satırına (ps çıktısı, shell geçmişi) geçmez — manage'e stdin'den verilir.
export E2E_ADMIN_EMAIL="e2e-admin-$E2E_RUN_ID@example.com"
E2E_ADMIN_PASSWORD="$(openssl rand -base64 24)"
export E2E_ADMIN_PASSWORD

# DATABASE_URL'in içindeki veritabanı adı (temizlik psql'i için).
DB_NAME="$(basename "${DATABASE_URL%%\?*}")"

# cleanup, ÇIKIŞ KODUNU DEĞİŞTİRMEZ: "exit" çağırmıyor ve son komutu "|| true" ile
# yutuyor → testler kırmızıysa betik de kırmızı çıkar (CI bunu görebilsin).
# Yalnız KULLANICILAR siliniyor; önerilen kitaplar bilinçli kalıyor (submitted_by
# ON DELETE SET NULL) — arayüzde gözle bakılabilsin, mvp_smoke.sh ile aynı tercih.
cleanup() {
	docker exec bookhub-postgres psql -U bookhub -d "$DB_NAME" -qtAc \
		"DELETE FROM users WHERE email LIKE 'e2e-%-$E2E_RUN_ID@example.com'" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "▶ yığın ayağa kalkıyor (AUTH_RATE_LIMIT_PER_MIN=$AUTH_RATE_LIMIT_PER_MIN)"
docker compose --profile app up -d --build --wait

# NGINX'in login|register kovası (zone=auth) konteynerin PAYLAŞILAN BELLEĞİNDE durur ve
# yeniden başlatınca sıfırlanır. NEDEN her koşuda: bir koşu ~9 kredi yiyor, kova 11 ve
# 3 sn'de 1 doluyor; arka arkaya iki "make e2e" arasında yalnızca ~7 sn geçtiği için
# ikinci koşu deterministik olarak 429 alırdı — üstelik CI'daki yeniden deneme de aynı
# boş kovaya çarpıp gerçek hatayı bir kota mesajının altında gizlerdi.
echo "▶ NGINX auth kovası sıfırlanıyor (bookhub-web restart)"
docker restart bookhub-web >/dev/null
# Bekleyiş SINIRLI: sağlıksız bir konteynerde sonsuz döngü, testlerin düşmesi yerine
# koşuyu (ve CI job'ını) asardı — hatayı sessizce beklemektense yüksek sesle öl.
for _ in $(seq 60); do
	[ "$(docker inspect -f '{{.State.Health.Status}}' bookhub-web 2>/dev/null)" = healthy ] && break
	sleep 1
done
[ "$(docker inspect -f '{{.State.Health.Status}}' bookhub-web 2>/dev/null)" = healthy ] || {
	echo "bookhub-web yeniden başlatmadan sonra 60 sn içinde sağlıklı olmadı" >&2
	exit 1
}

echo "▶ demo katalog"
make demo-seed

echo "▶ e2e admin: $E2E_ADMIN_EMAIL"
printf '%s\n' "$E2E_ADMIN_PASSWORD" |
	docker exec -i bookhub-api /app/manage create-admin -email "$E2E_ADMIN_EMAIL" -name "E2E Admin"

# "exec npx ..." DEĞİL: exec bu kabuğu değiştirirdi ve trap hiç çalışmaz, e2e kullanıcıları
# veritabanında kalırdı.
echo "▶ playwright"
(cd frontend && npx playwright test "$@")

# NOT: dev yığını bir sonraki düz "make app-up" (AUTH_RATE_LIMIT_PER_MIN tanımsız) ile
# varsayılan 10'a döner; o zamana kadar yükseltilmiş sınırla çalışır. Bu betik artık her
# koşuda NGINX'i yeniden başlattığı için arka arkaya "make e2e" güvenlidir; ama e2e'den
# HEMEN SONRA aynı dev yığınında (make app-up) elle giriş denemeleri yaparsan kovanın
# kalanıyla karşılaşırsın (3 sn'de 1 dolar).
#
# NOT 2: API'nin sayacı REDIS'te ve 60 sn'lik SABİT pencereli (internal/ratelimit: INCR +
# EXPIRE NX) — konteyner yeniden yaratılsa da sıfırlanmaz. Yani bu koşu (sınır 1000) sayacı
# 10'un üstüne çıkarır; hemen ardından sınırı varsayılana (10) düşürüp "make smoke" koşarsan
# pencere dolana kadar 429 alırsın. Bir dakika bekle ya da smoke'u e2e'den önce koş.
