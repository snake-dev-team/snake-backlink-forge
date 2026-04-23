# Makefile for Snake Backlink Forge — API service (Go)
# Run `make help` to list all targets.

.PHONY: build run test lint vet tidy migrate-up migrate-down migrate-new sqlc-gen docker-build docker-run help

## build: Compile a static Linux binary (cross-compile safe from Windows/macOS).
build:
	CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o bin/api ./cmd/api

## run: Run the server locally (uses .env if present).
run:
	go run ./cmd/api

## test: Run all tests with race detector and coverage report.
test:
	go test ./... -race -cover

## vet: Run go vet on all packages.
vet:
	go vet ./...

## lint: Run golangci-lint (install: https://golangci-lint.run/usage/install/).
lint:
	golangci-lint run

## tidy: Tidy and verify go.mod / go.sum.
tidy:
	go mod tidy

# ── Database migrations (Phase 03 wires goose; targets are placeholders) ──────
# Requires: goose CLI (`go install github.com/pressly/goose/v3/cmd/goose@latest`)
# and DATABASE_URL exported in your shell or .env.

## migrate-up: Apply all pending migrations.
migrate-up:
	goose -dir internal/migrations postgres "$${DATABASE_URL}" up

## migrate-down: Roll back the last applied migration.
migrate-down:
	goose -dir internal/migrations postgres "$${DATABASE_URL}" down

## migrate-new name=<slug>: Create a new timestamped SQL migration file.
migrate-new:
	@test -n "$(name)" || (echo "Usage: make migrate-new name=add_foo" && exit 1)
	goose -dir internal/migrations create $(name) sql

# ── sqlc (Phase 03 fills queries; target is a placeholder) ────────────────────
## sqlc-gen: Regenerate type-safe DB query code from SQL.
sqlc-gen:
	sqlc generate

# ── Docker ────────────────────────────────────────────────────────────────────
## docker-build: Build the multi-stage Docker image.
docker-build:
	docker build -t sbf-api .

## docker-run: Run the image locally (requires .env or explicit env vars).
docker-run:
	docker run --rm -p 8080:8080 --env-file .env sbf-api

## help: Print this help message.
help:
	@grep -E '^## ' Makefile | sed 's/## //'
