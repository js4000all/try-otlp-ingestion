.PHONY: help run build test vet fmt fmt-check tidy check shell clean

COMPOSE := docker compose
GO_RUN := $(COMPOSE) run --rm --user $(shell id -u):$(shell id -g) go

help:
	@printf '%s\n' 'Targets:' '  run        Run the placeholder program' '  build      Build a Linux binary in bin/' '  test       Run Go tests' '  vet        Run go vet' '  fmt        Format Go source code' '  fmt-check  Verify Go formatting' '  tidy       Update Go module metadata' '  check      Run formatting, tests, vet, and build' '  shell      Open a shell in the Go container' '  clean      Remove generated files and caches'

run:
	$(GO_RUN) go run ./cmd/otlpgen

build:
	mkdir -p bin
	$(GO_RUN) go build -o bin/otlpgen ./cmd/otlpgen

test:
	$(GO_RUN) go test ./...

vet:
	$(GO_RUN) go vet ./...

fmt:
	$(GO_RUN) go fmt ./...

fmt-check:
	$(GO_RUN) sh -c 'test -z "$$(gofmt -l cmd)"'

tidy:
	$(GO_RUN) go mod tidy

check: fmt-check test vet build

shell:
	$(GO_RUN) sh

clean:
	rm -rf bin .cache
