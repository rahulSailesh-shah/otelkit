.PHONY: install build test tidy sample dev

install:
	go install ./cmd/otelkit

build:
	go build -o ./bin/otelkit ./cmd/otelkit

test:
	go test ./...
	cd examples/sampleapi && go test ./...

tidy:
	go mod tidy
	cd examples/sampleapi && go mod tidy
	go work sync

sample:
	cd examples/sampleapi && go run .

dev: install
	cd examples/sampleapi && otelkit run --service sampleapi -- go run .