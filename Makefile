.PHONY: help setup build up down logs migrate scrape test fmt vet vuln openapi check fmt-check reset

COMPOSE ?= docker compose
COMPETITION ?= brasileirao-serie-a
YEAR ?= 2026

help:
	@echo "setup     copy .env.example to .env when .env is missing"
	@echo "build     build the API image"
	@echo "up        start Postgres, migrations, API, and worker"
	@echo "down      stop containers and keep the database volume"
	@echo "logs      follow compose logs"
	@echo "migrate   apply SQL migrations"
	@echo "scrape    run one sync (COMPETITION and YEAR)"
	@echo "test      go test -race in the test container"
	@echo "fmt       rewrite Go files with gofmt"
	@echo "vet       go vet"
	@echo "vuln      govulncheck"
	@echo "openapi   parse docs/openapi.yaml"
	@echo "check     format, vet, test, vuln, and OpenAPI"
	@echo "reset     delete volumes; requires CONFIRM=yes"

setup:
	@if [ ! -f .env ]; then cp .env.example .env; echo "created .env"; else echo ".env already exists"; fi

build:
	$(COMPOSE) build api worker migrate

up:
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down --remove-orphans

logs:
	$(COMPOSE) logs -f --tail=200

migrate:
	$(COMPOSE) run --rm --build migrate

scrape:
	$(COMPOSE) run --rm --build worker -once -competition $(COMPETITION) -year $(YEAR)

test:
	$(COMPOSE) up -d postgres
	$(COMPOSE) --profile test run --rm --build test

fmt:
	$(COMPOSE) --profile test run --rm --no-deps --build --entrypoint gofmt test -w .

vet:
	$(COMPOSE) --profile test run --rm --no-deps --build --entrypoint go test vet ./...

vuln:
	$(COMPOSE) --profile test run --rm --no-deps --build --entrypoint sh test -c "go install golang.org/x/vuln/cmd/govulncheck@v1.1.4 && \"$$(go env GOPATH)/bin/govulncheck\" ./..."

openapi:
	$(COMPOSE) --profile test run --rm --no-deps --build --entrypoint go test test ./internal/httpapi -count=1 -run TestOpenAPIDocument

check: fmt-check vet test vuln openapi

fmt-check:
	$(COMPOSE) --profile test run --rm --no-deps --build --entrypoint sh test -c 'test -z "$$(gofmt -l .)"'

reset:
	@if [ "$(CONFIRM)" != "yes" ]; then echo "refusing to delete volumes; re-run with CONFIRM=yes"; exit 1; fi
	$(COMPOSE) down -v --remove-orphans
