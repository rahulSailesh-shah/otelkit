.PHONY: install build test tidy sample dev

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

sample:
	cd examples/todos && go run .

dev: install
	cd examples/todos && otelkit run --service todo-api -- go run .