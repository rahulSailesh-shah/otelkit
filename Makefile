DB_FILE ?= ./otelkit.db
MIGRATIONS_DIR = internal/store/migrations

# Override at call site: make run-grafana SERVICE=my-api CMD="./my-api --port 8080"
SERVICE ?= demo
CMD     ?= go run ./examples/demo

.PHONY: install build test tidy demo \
	grafana-up grafana-down \
	signoz-up signoz-down \
	run-grafana run-signoz \
	migrate-up migrate-down sqlc-gen

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

# ── Grafana stack: Jaeger + Prometheus + Loki + Grafana ──────────────────────
# UI: Jaeger :16686  Prometheus :9090  Grafana :3000 (admin/admin)
# otelkit flags: --jaeger-addr localhost:14317 --prometheus-addr :9091 --loki-addr http://localhost:3100
grafana-up:
	docker compose --profile grafana -f infra/docker-compose.yml up -d

grafana-down:
	docker compose --profile grafana -f infra/docker-compose.yml down

# ── SigNoz stack: ZooKeeper + ClickHouse + OTel Collector + SigNoz UI ────────
# UI: http://localhost:8080
# otelkit flags: --signoz-addr localhost:24317
# Note: first run downloads histogram-quantile binary (~5s), subsequent runs instant.
signoz-up:
	docker compose -f infra/signoz/docker-compose.yml up -d

signoz-down:
	docker compose -f infra/signoz/docker-compose.yml down

# ── Run otelkit launcher ──────────────────────────────────────────────────────
# Defaults: SERVICE=demo  CMD="go run ./examples/demo"
# Override: make run-grafana SERVICE=my-api CMD="./my-api --port 8080"

run-grafana: build
	./bin/otelkit run \
		--jaeger-addr localhost:14317 \
		--prometheus-addr :9091 \
		--loki-addr http://localhost:3100 \
		--service $(SERVICE) \
		-- $(CMD)

# UI: http://localhost:8080
# Demo app overrides port via APP_PORT (default 8081 to avoid conflict with SigNoz UI on 8080)
APP_PORT ?= 8081
run-signoz: build
	APP_PORT=$(APP_PORT) ./bin/otelkit run \
		--signoz-addr localhost:24317 \
		--service $(SERVICE) \
		-- $(CMD)

# ── DB / codegen ──────────────────────────────────────────────────────────────
migrate-up:
	@echo "Running migrations..."
	@goose -dir $(MIGRATIONS_DIR) sqlite3 $(DB_FILE) up

migrate-down:
	@echo "Rolling back migration..."
	@goose -dir $(MIGRATIONS_DIR) sqlite3 $(DB_FILE) down

sqlc-gen:
	@echo "Generating sqlc..."
	@sqlc generate
