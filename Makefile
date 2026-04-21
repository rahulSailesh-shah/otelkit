DB_FILE ?= ./otelkit.db
MIGRATIONS_DIR = internal/store/migrations

.PHONY: install build test tidy sample dev migrate-up migrate-down migrate-status sqlc-gen

install:
	go install ./cmd/otelkit

build:
	go build -o ./bin/otelkit ./cmd/otelkit

test:
	go test ./...
	cd examples/todos && go test ./...

tidy:
	go mod tidy
	cd examples/todos && go mod tidy
	go work sync

todos:
	cd examples/todos && go run .

run:
	cd cmd/otelkit && go run . run

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
