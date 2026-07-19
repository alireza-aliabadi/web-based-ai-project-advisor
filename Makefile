.PHONY: up down infra api test lint migrate tidy

up:
	docker compose up -d --build

down:
	docker compose down

infra:
	docker compose up -d postgres redis qdrant

api:
	cd web-advisor-api && go run ./cmd/server

test:
	cd web-advisor-api && go test ./...

lint:
	cd web-advisor-api && go vet ./...
	cd web-advisor-api && gofmt -l .

tidy:
	cd web-advisor-api && go mod tidy

migrate:
	cd web-advisor-api && go run ./cmd/server --migrate-only
