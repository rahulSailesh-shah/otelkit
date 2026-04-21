DB_FILE ?= ./otelkit.db
MIGRATIONS_DIR = internal/store/migrations

.PHONY: install build test tidy dev demo infra-up infra-down migrate-up migrate-down migrate-status sqlc-gen

install:
	go install ./cmd/otelkit

build:
	go build -o ./bin/otelkit ./cmd/otelkit

test:
	go test ./...
	cd examples/demo && go test ./...

tidy:
	go mod tidy
	cd examples/demo && go mod tidy
	go work sync

demo:
	cd examples/demo && go run .

run:
	cd cmd/otelkit && go run . run

infra-up:
	docker compose -f infra/docker-compose.yml up -d

infra-down:
	docker compose -f infra/docker-compose.yml down

migrate-up:
	@echo "Running migrations..."
	@goose -dir $(MIGRATIONS_DIR) sqlite3 $(DB_FILE) up

migrate-down:
	@echo "Rolling back migration..."
	@goose -dir $(MIGRATIONS_DIR) sqlite3 $(DB_FILE) down

migrate-status:
	@echo "Migration status..."
	@goose -dir $(MIGRATIONS_DIR) sqlite3 $(DB_FILE) status

sqlc-gen:
	@echo "Generating sqlc..."
	@sqlc generate
