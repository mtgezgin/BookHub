# Makefile — sık kullanılan komutların kısayolları (Faz 8).
#
# Neden: komutları hatırlamak yerine "make test", "make up" yaz. Her hedef tek bir şey yapar
# ve altında GERÇEK komut görünür — sihir yok; öğrenirken komutun kendisini de gör.
# Env gerektirenler .env'i kendileri yükler (set -a; . ./.env; set +a).

SHELL := /bin/bash
ENV := set -a; . ./.env; set +a
GO := go -C backend

.PHONY: help up down app-up app-down logs run test test-db lint fmt sqlc migrate-status admin smoke perf-seed docker-build

help: ## Hedefleri listele
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-16s %s\n", $$1, $$2}'

up: ## Altyapıyı kaldır (Postgres + Redis)
	docker compose up -d

down: ## Tüm konteynerleri durdur (veri volume'da kalır)
	docker compose --profile app down

app-up: ## Tüm uygulamayı konteynerde çalıştır (API imajını da derler)
	docker compose --profile app up -d --build

app-down: ## Yalnız API konteynerini durdur
	docker compose --profile app stop api

logs: ## API konteyner loglarını izle
	docker compose --profile app logs -f api

run: ## API'yi host'ta çalıştır (hızlı geliştirme döngüsü)
	$(ENV); $(GO) run ./cmd/api

test: ## Birim testleri (DB'siz, CI gibi)
	$(GO) test -race ./...

test-db: ## Tüm testler (Postgres + Redis entegrasyonu dahil)
	$(ENV); TEST_DATABASE_URL="$$DATABASE_URL" TEST_REDIS_URL="$$REDIS_URL" $(GO) test -race ./...

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

smoke: ## MVP Definition of Done'ı çalışan API'ye karşı doğrula (API=... ile değiştirilebilir)
	$(ENV); API=$${API:-http://localhost:8080} backend/scripts/mvp_smoke.sh

perf-seed: ## Ayrı bookhub_perf DB'sine 20k kitap / ~200k oy (EXPLAIN ve yük testi için)
	docker exec bookhub-postgres createdb -U bookhub bookhub_perf || true
	goose -dir backend/migrations postgres "postgres://bookhub:bookhub_dev@localhost:5432/bookhub_perf?sslmode=disable" up
	docker exec -i bookhub-postgres psql -U bookhub -d bookhub_perf < backend/scripts/perf_seed.sql

docker-build: ## API imajını derle
	docker build -t bookhub-api:dev backend
