.PHONY: help run build test vet fmt fmt-check tidy check shell clean

COMPOSE := docker compose
GO_RUN := $(COMPOSE) run --rm --user $(shell id -u):$(shell id -g) go

help:
	@printf '%s\n' 'Targets:' '  run        Generate JSON Lines (ARGS="-start ... -days ...")' '  build      Build Linux binary in bin/' '  test       Run Go tests' '  vet        Run go vet' '  fmt        Format Go code' '  fmt-check  Check Go formatting' '  tidy       Update module metadata' '  check      Run formatting, tests, vet, build' '  shell      Open shell in Go container' '  clean      Remove generated artifacts'

run:
	@$(GO_RUN) go run ./cmd/otlpgen $(ARGS)

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
	$(GO_RUN) sh -c 'test -z "$$(gofmt -l cmd internal)"'

tidy:
	$(GO_RUN) go mod tidy

check: fmt-check test vet build

shell:
	$(GO_RUN) sh

clean:
	rm -rf bin .cache
