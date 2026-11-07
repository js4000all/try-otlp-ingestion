.PHONY: help run run-otlp up down ps logs build test vet fmt fmt-check tidy check shell clean

COMPOSE := docker compose
GO_RUN := $(COMPOSE) run --rm --user $(shell id -u):$(shell id -g) go
OTLP_ENDPOINT ?= http://otel-collector:4318/v1/metrics

help:
	@printf '%s\n' 'Targets:' '  run         Generate JSON Lines (ARGS="-start ... -days ...")' '  run-otlp    Send OTLP/HTTP to Collector (ARGS="-count ...")' '  up          Start Collector and VictoriaMetrics' '  down        Stop Collector and VictoriaMetrics' '  ps          Show Compose services' '  logs        Follow Collector and VictoriaMetrics logs' '  build       Build Linux binary in bin/' '  test        Run Go tests' '  vet         Run go vet' '  fmt         Format Go code' '  fmt-check   Check Go formatting' '  tidy        Update module metadata' '  check       Run formatting, tests, vet, build' '  shell       Open shell in Go container' '  clean       Remove generated artifacts'

run:
	@$(GO_RUN) go run ./cmd/otlpgen $(ARGS)

run-otlp:
	@$(GO_RUN) go run ./cmd/otlpgen -output otlp -otlp-endpoint $(OTLP_ENDPOINT) $(ARGS)

up:
	$(COMPOSE) up -d otel-collector victoriametrics

down:
	$(COMPOSE) down

ps:
	$(COMPOSE) ps

logs:
	$(COMPOSE) logs -f otel-collector victoriametrics

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
