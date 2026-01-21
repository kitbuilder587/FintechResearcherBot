.PHONY: run build test test-race test-integration lint docker-up docker-down docker-build db-up db-down migrate

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
	docker-compose up -d

docker-down:
	docker-compose down

docker-build:
	docker-compose build

db-up:
	docker-compose up -d db

db-down:
	docker-compose stop db

migrate:
	docker exec -i fintech-db psql -U fintech -d fintech_bot < migrations/001_init.up.sql
