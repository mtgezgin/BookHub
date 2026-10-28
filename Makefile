# Makefile — sık kullanılan komutların kısayolları (Faz 8).
#
# Neden: komutları hatırlamak yerine "make test", "make up" yaz. Her hedef tek bir şey yapar
# ve altında GERÇEK komut görünür — sihir yok; öğrenirken komutun kendisini de gör.
# Env gerektirenler .env'i kendileri yükler (set -a; . ./.env; set +a).

SHELL := /bin/bash
ENV := set -a; . ./.env; set +a
GO := go -C backend

.PHONY: help up down app-up app-down logs run mcp test test-db lint fmt sqlc migrate-status admin demo-seed smoke perf-seed docker-build scan web web-test e2e k8s-up k8s-port k8s-status k8s-down

help: ## Hedefleri listele
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-16s %s\n", $$1, $$2}'

up: ## Altyapıyı kaldır (Postgres + Redis)
	docker compose up -d

down: ## Tüm konteynerleri durdur (veri volume'da kalır)
	docker compose --profile app down

app-up: ## Tüm uygulama konteynerde: http://localhost:3000 (NGINX + frontend + API + DB + Redis)
	docker compose --profile app up -d --build

app-down: ## Yalnız API konteynerini durdur
	docker compose --profile app stop api

logs: ## API konteyner loglarını izle
	docker compose --profile app logs -f api

run: ## API'yi host'ta çalıştır (hızlı geliştirme döngüsü)
	$(ENV); $(GO) run ./cmd/api

mcp: ## MCP sunucusu (stdio; salt okunur araçlar) — API=... ile adres değişir, varsayılan app-up'ın NGINX'i
	BOOKHUB_API_URL=$${API:-http://localhost:3000} $(GO) run ./cmd/mcp

test: ## Birim testleri (DB'siz, CI gibi)
	$(GO) test -race ./...

# Testler dev veritabanına DEĞİL, ayrı bir bookhub_test'e koşar (CI'daki gibi): dev verisini
# kirletmesinler, dev'deki yüksek puanlı kitaplar top-rated testlerini bozmasın.
TEST_DB_URL := postgres://bookhub:bookhub_dev@localhost:5432/bookhub_test?sslmode=disable

test-db: ## Tüm testler (ayrı bookhub_test veritabanı + Redis entegrasyonu dahil)
	docker exec bookhub-postgres createdb -U bookhub bookhub_test 2>/dev/null || true
	$(ENV); TEST_DATABASE_URL="$(TEST_DB_URL)" TEST_REDIS_URL="$$REDIS_URL" $(GO) test -race ./...

lint: ## gofmt + vet + golangci-lint
	cd backend && test -z "$$(gofmt -l .)" && go vet ./... && golangci-lint run ./...

fmt: ## Kodu biçimlendir
	cd backend && gofmt -w .

sqlc: ## sql/queries → internal/db kod üretimi
	cd backend && sqlc generate

migrate-status: ## Migration durumu
	$(ENV); goose -dir backend/migrations postgres "$$DATABASE_URL" status

admin: ## İlk admin'i oluştur: make admin EMAIL=sen@ornek.com (parola gizli sorulur)
	$(ENV); $(GO) run ./cmd/manage create-admin -email "$(EMAIL)"

demo-seed: ## Arayüz için örnek katalog (türler, klasikler, puanlar, yorumlar) — tekrar çalıştırılabilir
	docker exec -i bookhub-postgres psql -U bookhub -d bookhub < backend/scripts/demo_seed.sql
	@# Kapaklar canlıda migration olarak gelir; yerelde seed migration'lardan SONRA koştuğu için aynı
	@# dosyayı burada yeniden uygula (tek kaynak, yalnız boş cover_url'leri doldurur). Kapak için yeni
	@# bir migration gelirse o da buraya eklenir.
	docker exec -i bookhub-postgres psql -U bookhub -d bookhub -v ON_ERROR_STOP=1 -q < backend/migrations/0009_demo_book_covers.sql
	@# Seed uygulamayı atlayıp DB'ye doğrudan yazdı → top-rated cache'i bayat; sil (TTL'i beklemeden).
	@# Desenle siliyoruz (ratings.TopRatedCacheKey'in sürüm eki v1→v2 olsa da çalışsın).
	@# "|| true": Redis kapalıysa seed yine de tamamlansın (cache 60 sn'de kendiliğinden tazelenir).
	docker exec bookhub-redis sh -c "redis-cli --scan --pattern 'books:top-rated:*' | xargs -r redis-cli del" >/dev/null || true

smoke: ## MVP Definition of Done'ı çalışan API'ye karşı doğrula (API=... ile değiştirilebilir)
	$(ENV); API=$${API:-http://localhost:8080} backend/scripts/mvp_smoke.sh

perf-seed: ## Ayrı bookhub_perf DB'sine 20k kitap / ~200k oy (EXPLAIN ve yük testi için)
	docker exec bookhub-postgres createdb -U bookhub bookhub_perf || true
	goose -dir backend/migrations postgres "postgres://bookhub:bookhub_dev@localhost:5432/bookhub_perf?sslmode=disable" up
	docker exec -i bookhub-postgres psql -U bookhub -d bookhub_perf < backend/scripts/perf_seed.sql

docker-build: ## API imajını derle
	docker build -t bookhub-api:dev backend

# Yayınlanan İKİ imajı bilinen açıklara karşı tara (CI'daki "images" işiyle aynı kural):
# HIGH/CRITICAL ve düzeltmesi OLAN bir açık varsa başarısız. --ignore-unfixed: dağıtımın henüz
# yama yayınlamadığı açık için yapacak bir şey yok, onunla kırılmak yalnızca gürültü.
# trivy kurulumu: brew install trivy (ya da go install github.com/aquasecurity/trivy/cmd/trivy@latest)
scan: ## İmajları Trivy ile tara (önce make app-up ya da docker compose build)
	@for img in bookhub-api:dev bookhub-web:dev; do \
		echo "▶ trivy $$img"; \
		trivy image --scanners vuln --ignore-unfixed --severity HIGH,CRITICAL --exit-code 1 "$$img" || exit 1; \
	done

web: ## Frontend geliştirme sunucusu (http://localhost:5173; /api → localhost:8080)
	cd frontend && npm install && npm run dev

web-test: ## Frontend tip kontrolü + testler
	cd frontend && npm run typecheck && npm test

# Uçtan uca: GERÇEK tarayıcı (chromium), gerçek NGINX + API + Postgres + Redis. Betik
# yığını kendisi kaldırır, demo veriyi basar, bu koşuya özel bir admin açar ve çıkarken
# açtığı kullanıcıları siler. Tarayıcı bir kereliğine:
#   cd frontend && npm run e2e:browsers
e2e: ## Playwright uçtan uca testleri (yığını kendisi kaldırır)
	scripts/e2e.sh

# ---- Kubernetes (Faz 12): yerel kind kümesi ----
# Her kubectl çağrısı AÇIKÇA --context kind-bookhub: makinede başka (iş) kümeleri tanımlıysa
# yanlışlıkla onlara bir şey uygulanamaz.
KIND_CLUSTER := bookhub
KCTL := kubectl --context kind-$(KIND_CLUSTER)

k8s-up: ## Yerel kind kümesinde tüm uygulama (küme yoksa oluşturur)
	kind get clusters | grep -qx $(KIND_CLUSTER) || kind create cluster --name $(KIND_CLUSTER)
	docker build -t bookhub-api:dev backend
	docker build -t bookhub-web:dev frontend
	docker pull -q postgres:16 && docker pull -q redis:7-alpine
	@# "kind load docker-image" Docker Desktop'ın containerd deposundaki ÇOK PLATFORMLU imajlarda
	@# "content digest not found" ile düşüyor (yerelde yalnız bu makinenin platformu var). Çözüm:
	@# imajları tek platformlu bir arşive kaydedip arşivi yüklemek.
	docker save --platform linux/$$(docker version -f '{{.Server.Arch}}') -o /tmp/bookhub-images.tar \
		bookhub-api:dev bookhub-web:dev postgres:16 redis:7-alpine
	kind load image-archive --name $(KIND_CLUSTER) /tmp/bookhub-images.tar
	rm -f /tmp/bookhub-images.tar
	$(KCTL) apply -f deploy/k8s/namespace.yaml
	@# Sırlar dosyada değil: ilk kurulumda rastgele üretilir, sonra korunur.
	$(KCTL) -n bookhub get secret bookhub-secrets >/dev/null 2>&1 || $(KCTL) -n bookhub create secret generic bookhub-secrets \
		--from-literal=POSTGRES_PASSWORD="$$(openssl rand -hex 24)" --from-literal=JWT_SECRET="$$(openssl rand -base64 48)"
	@# Tekrar çalıştırmada iki sessiz tuzak (review bulgusu F2):
	@#  1) Job şablonu DEĞİŞTİRİLEMEZ ve tamamlanmış Job 10 dk yaşar → apply hiçbir şey yapmaz,
	@#     wait eski Job'ın "complete"ine takılır, YENİ migration'lar koşmaz. → önce sil.
	@#  2) İmaj etiketi hep ":dev" → pod şablonu bayt bayt aynı → apply rollout TETİKLEMEZ,
	@#     eski binary çalışmaya devam eder. → rollout restart ile yeni imajı zorla.
	$(KCTL) -n bookhub delete job migrate --ignore-not-found
	$(KCTL) apply -k deploy/k8s
	$(KCTL) -n bookhub wait --for=condition=complete job/migrate --timeout=240s
	$(KCTL) -n bookhub rollout restart deploy/api deploy/web
	$(KCTL) -n bookhub rollout status deploy/api --timeout=180s
	$(KCTL) -n bookhub rollout status deploy/web --timeout=180s

k8s-port: ## Kümedeki uygulamayı http://localhost:3001'e aç (Ctrl+C ile kapat)
	$(KCTL) -n bookhub port-forward svc/web 3001:80

k8s-status: ## Pod/servis durumu
	$(KCTL) -n bookhub get pods,svc,jobs,pvc

k8s-down: ## kind kümesini tamamen sil
	kind delete cluster --name $(KIND_CLUSTER)
