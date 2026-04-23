SHELL := bash
.PHONY: dev-setup dev-up dev-down dev-reset dev-logs dev migrate migrate-down run-api stop test lint clean

## dev-setup: Copy .env templates on first clone (idempotent).
dev-setup:
	@test -f .env || cp .env.example .env
	@test -f services/api/.env || cp services/api/.env.example services/api/.env
	@echo "[ok] .env templates in place. Edit credentials as needed."

## dev-up: Start Postgres + Redis; block until both healthchecks pass.
dev-up:
	docker compose -f ops/docker-compose.yml up -d --wait

## dev-down: Stop infra containers (volumes preserved).
dev-down:
	docker compose -f ops/docker-compose.yml down

## dev-reset: Stop infra AND wipe volumes (full clean slate).
dev-reset: dev-down
	docker volume rm sbf_pgdata sbf_redisdata 2>/dev/null || true

## dev-logs: Tail infra logs.
dev-logs:
	docker compose -f ops/docker-compose.yml logs -f

## dev: Full local dev loop — setup + infra up (healthy) + migrate + run API.
dev: dev-setup dev-up migrate run-api

## migrate: Apply all pending DB migrations via services/api Makefile.
migrate:
	cd services/api && $(MAKE) migrate-up

## migrate-down: Roll back last migration via services/api Makefile.
migrate-down:
	cd services/api && $(MAKE) migrate-down

## run-api: Start the Go API server (foreground).
run-api:
	cd services/api && $(MAKE) run

## stop: Alias for dev-down.
stop: dev-down

## test: Run Go unit tests + any JS/TS package tests.
test:
	cd services/api && $(MAKE) test
	pnpm -r test --if-present

## lint: Lint Go code + all JS/TS packages.
lint:
	cd services/api && $(MAKE) lint
	pnpm -r lint

## clean: Stop infra (with volumes) and clean Go build artifacts.
clean:
	cd services/api && $(MAKE) clean 2>/dev/null || true
	docker compose -f ops/docker-compose.yml down -v 2>/dev/null || true

## help: Print this help message.
help:
	@grep -E '^## ' Makefile | sed 's/## //'
