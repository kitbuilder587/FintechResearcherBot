.PHONY: run build test test-race test-integration lint docker-up docker-down docker-build db-up db-down wait-db migrate up infra-up bot-up observability-up observability-logs logs restart ps health prometheus-health grafana-health searxng-smoke tavily-smoke eval-score

export GOPROXY=https://proxy.golang.org,direct

# Load .env file if exists (- prefix ignores errors if file missing)
-include .env
export

run:
	go run ./cmd/bot

build:
	go build -o bin/bot ./cmd/bot

test:
	go test -v -short ./...

test-race:
	go test -race ./...

test-integration:
	go test -v -run Integration ./test/...

lint:
	golangci-lint run

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-build:
	docker compose build

db-up:
	docker compose up -d db

db-down:
	docker compose stop db

wait-db:
	@echo "Waiting for Postgres..."
	@until docker exec fintech-db pg_isready -U $${DB_USER:-fintech} -d $${DB_NAME:-fintech_bot} >/dev/null 2>&1; do sleep 1; done

migrate: wait-db
	docker exec -i fintech-db psql -U $${DB_USER:-fintech} -d $${DB_NAME:-fintech_bot} < migrations/001_init.up.sql
	docker exec -i fintech-db psql -U $${DB_USER:-fintech} -d $${DB_NAME:-fintech_bot} < migrations/002_world_model.up.sql

infra-up:
	docker compose up -d db searxng

bot-up:
	docker compose up -d --build bot

observability-up:
	docker compose up -d prometheus grafana

observability-logs:
	docker compose logs -f prometheus grafana

up: infra-up migrate bot-up observability-up

restart:
	docker compose down
	$(MAKE) up

logs:
	docker logs -f fintech-bot

ps:
	docker compose ps

health:
	curl -sS http://localhost:$${BOT_HTTP_PORT:-8081}/health

prometheus-health:
	curl -sS http://localhost:$${PROMETHEUS_PORT:-9090}/-/healthy

grafana-health:
	curl -sS http://localhost:$${GRAFANA_PORT:-3000}/api/health

searxng-smoke:
	python3 tests/eval/searxng_smoke.py

tavily-smoke:
	python3 tests/eval/tavily_smoke.py

eval-score:
	python3 tests/eval/score_answers.py
