---
name: Go API skeleton
phase: 02
status: pending
priority: P0
estimated_effort: 3.5h
blockedBy: [phase-01-monorepo-backbone]
blocks: [phase-03-database-layer, phase-06-quality-gates]
agents: [fullstack-developer]
---

# Phase 02 — Go API Skeleton

## Context Links

- MASTER_PROMPT §1.2 ADR (lines 94–117) — Fiber v2, pgx, Zap, goose, sqlc locked
- MASTER_PROMPT §2 Repository Structure (lines 363–468) — `services/api/` tree
- MASTER_PROMPT §12.1 env vars (lines 2147–2168) — 16 vars
- MASTER_PROMPT §12.3 Dockerfile (lines 2203–2219) — multi-stage distroless
- Scout report Priority 2 (§7 items 6–16)
- Go stack research: `plans/reports/researcher-260424-0247-go-stack-versions.md` (§1–9)

## Overview

**Priority:** P0 · **Status:** pending · **Effort:** 3.5h
Create `services/api/` Go module. Fiber app boots on `:8080`, graceful shutdown, fiberzap logger, config parse, pgxpool+redis clients initialized, `/health` + `/ready` routes return JSON, Dockerfile multi-stage distroless. NO handlers beyond health. NO auth/hmac middleware (Phase 2-3).

## Key Insights

- **Deviation from ADR**: Go 1.26.2 (not 1.23 — EOL April 2026). Update `go.mod` toolchain + Dockerfile `FROM golang:1.26-alpine`.
- **Fiber built-in logger ≠ Zap**: must use `github.com/gofiber/contrib/fiberzap/v2` (researcher R1 §2, §8).
- **pgx v5 SSL**: must include `sslmode=require` in connection string for Fly Postgres; local dev uses `sslmode=disable` from `.env`.
- **pgxpool config**: MaxConns 15, MinConns 3, MaxConnLifetime 30m, MaxConnIdleTime 5m (researcher §3 for Fly shared-cpu-1x 1GB).
- **Fly proxy IP**: prefer `Fly-Client-IP` header; fallback `c.IP()`.
- **Distroless has no shell**: ensure pure-Go deps only (pgx v5 ✓, redis/go-redis ✓).

## Requirements

**Functional:**
- `go run ./cmd/api` boots Fiber on `$PORT` (default 8080).
- `GET /health` → `{"status":"ok","version":"0.1.0"}` 200 (no DB dependency).
- `GET /ready` → 200 if PG ping + Redis ping succeed; 503 with `{"status":"degraded","postgres":"fail"|"ok","redis":"fail"|"ok"}` otherwise.
- Structured JSON logs to stdout via Zap via fiberzap.
- Graceful shutdown on SIGTERM/SIGINT, drains Fiber within 10s then closes pools.

**Non-functional:**
- `CGO_ENABLED=0` build succeeds.
- Distroless image ≤ 25MB (distroless/static ~2.5MB + Go binary ~15-20MB; 25MB is safety margin). **[M2]**
- `go vet ./...` + `golangci-lint run` clean on initial commit.

## Architecture

```
main.go
  → config.Load()                (envconfig parse + validate)
  → logger.New(cfg)              (zap prod JSON)
  → db.New(cfg) → *pgxpool.Pool  (ping on init, return err if fail)
  → redis.New(cfg) → *redis.Client
  → api.NewServer(cfg, logger, db, redis)
     ├── middleware/logger.go   (fiberzap)
     ├── middleware/recover.go  (Fiber recover + zap)
     ├── router.go → handlers/health.go
  → app.ListenAndServe(":8080")
  ← SIGTERM → shutdown(ctx=10s) → db.Close() + redis.Close()
```

## Related Code Files

**CREATE (all paths `C:\Users\Hanna\Desktop\tool_backlink\`):**
- `services/api/go.mod` (module `github.com/kekuta/snake-backlink-forge/services/api`, Go 1.26)
- `services/api/go.sum` (generated)
- `services/api/cmd/api/main.go` — Fiber boot + shutdown
- `services/api/internal/config/config.go` — envconfig struct + Load()
- `services/api/internal/db/db.go` — pgxpool init + Ping()
- `services/api/internal/redis/redis.go` — go-redis v9 client + Ping()
- `services/api/internal/middleware/logger.go` — fiberzap adapter
- `services/api/internal/middleware/recover.go` — panic → zap.Error
- `services/api/internal/api/server.go` — Fiber app config, middleware mount
- `services/api/internal/api/router.go` — RegisterRoutes()
- `services/api/internal/api/handlers/health.go` — /health + /ready
- `services/api/internal/api/handlers/health_test.go` — **[C5]** smoke test: Health returns 200 + JSON `{"status":"ok"}`
- `services/api/internal/util/logger.go` — zap.NewProduction builder
- `services/api/.env.example` — 16 vars §12.1
- `services/api/.golangci.yml` — vet, errcheck, staticcheck, gosec, ineffassign
- `services/api/Makefile` — build/run/vet/lint/test/migrate-*/sqlc-gen
- `services/api/Dockerfile` — multi-stage, Go 1.26-alpine → distroless/static-debian12

**MODIFY:** none (first time creation).

## Implementation Steps

1. `cd services/api && go mod init github.com/kekuta/snake-backlink-forge/services/api` then add `go 1.26` + `toolchain go1.26.2`.
2. `go get` deps: `github.com/gofiber/fiber/v2@v2.52.6`, `github.com/gofiber/contrib/fiberzap/v2@latest`, `github.com/jackc/pgx/v5@v5.9.1`, `github.com/jackc/pgx/v5/pgxpool`, `github.com/redis/go-redis/v9@v9.18.0`, `go.uber.org/zap@v1.27.0`, `github.com/kelseyhightower/envconfig@latest`, `github.com/google/uuid@latest`.
3. Write `internal/config/config.go` — struct mirrors §12.1 (DatabaseURL, RedisURL, ClaudeBaseURL, ClaudeModel, TelegramBotToken, SepayWebhookToken, SerpapiKey, MozAccessID, MozSecret, TwocaptchaMasterKey, CapsolverMasterKey, ResendKey, JWTSecret, AdminTelegramIDs `[]int64`, LogLevel, Port int, Env string). `Load()` via envconfig + validate required; allow empty for Phase 2+ secrets but require DATABASE_URL, REDIS_URL, PORT, ENV.
4. Write `internal/util/logger.go` — `zap.NewProductionConfig()` with `Encoding=json`, `OutputPaths=["stdout"]`, level from `cfg.LogLevel`.
5. Write `internal/db/db.go` — parse `cfg.DatabaseURL` via `pgxpool.ParseConfig`, set MaxConns=15, MinConns=3, MaxConnLifetime=30m, MaxConnIdleTime=5m; `New(ctx, cfg)` returns pool + ping error.
6. Write `internal/redis/redis.go` — `redis.NewClient` with PoolSize=5, MaxRetries=3; ping test.
7. Write `internal/middleware/logger.go` — use `fiberzap.New(fiberzap.Config{Logger: zapLogger, Fields: ["ip","latency","status","method","url"]})`.
8. Write `internal/middleware/recover.go` — wrap `recover.New(recover.Config{EnableStackTrace:true, StackTraceHandler: func... log.Error})`.
9. Write `internal/api/handlers/health.go` — `Health(c) → 200 {"status":"ok","version":"0.1.0"}`; `Ready(c) → ping PG + Redis with 2s ctx each; 200 if both ok else 503`.
10. Write `internal/api/router.go` — `RegisterRoutes(app *fiber.App, h *Handlers)` mounts `GET /health`, `GET /ready`.
11. Write `internal/api/server.go` — `NewServer(cfg, log, db, rdb) (*fiber.App, error)` — `fiber.New(Config{DisableStartupMessage:false, ReadTimeout:15s, WriteTimeout:15s, IdleTimeout:60s})`, mount middleware logger+recover, register routes.
12. Write `cmd/api/main.go` — `Load config → logger → db → redis → server → go app.Listen(":"+port) → wait SIGTERM → app.ShutdownWithTimeout(10s) → db.Close() → rdb.Close()`.
13. Write `.env.example` copying §12.1 verbatim (empty values with inline comments).
14. Write `.golangci.yml` with linters: govet, errcheck, staticcheck, gosec, ineffassign, unused, gocritic. Timeout 3m.
15. Write `Makefile` targets: `build` (`CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o bin/api ./cmd/api`), `run`, `vet`, `lint` (golangci-lint), `test` (`go test ./... -race -cover`), `migrate-up`, `migrate-down`, `migrate-new name=X`, `sqlc-gen` (Phase 03 fills), `docker-build`, `docker-run`. **[C6]** Explicit `CGO_ENABLED=0 GOOS=linux` on Makefile `build` target ensures Windows local builds produce Linux-compatible binary matching Docker image (prevents silent GOOS=windows default).
16. Write `Dockerfile` — stage 1 `golang:1.26-alpine` + `apk add git build-base` + `go mod download` + `CGO_ENABLED=0 go build -ldflags='-s -w' -o /out/api ./cmd/api`; stage 2 `gcr.io/distroless/static-debian12:nonroot` + `COPY --from=builder /out/api /api` + `USER nonroot:nonroot` + `EXPOSE 8080` + `ENTRYPOINT ["/api"]`.
17. `go mod tidy && go vet ./... && go build ./...` — expect zero errors.

## Todo List

- [ ] `go mod init` + dep install (Fiber, fiberzap, pgx, go-redis, zap, envconfig)
- [ ] `internal/config/config.go` (16 §12.1 vars via envconfig)
- [ ] `internal/util/logger.go` (zap prod JSON)
- [ ] `internal/db/db.go` (pgxpool, Ping)
- [ ] `internal/redis/redis.go` (go-redis v9)
- [ ] `internal/middleware/{logger,recover}.go`
- [ ] `internal/api/handlers/health.go` (/health + /ready)
- [ ] **[C5]** `internal/api/handlers/health_test.go` (smoke test: Health → 200 + JSON `{status:ok}`)
- [ ] `internal/api/{router,server}.go`
- [ ] `cmd/api/main.go` (boot + graceful shutdown)
- [ ] `.env.example`, `.golangci.yml`
- [ ] `Makefile` (build/run/vet/lint/test/migrate/sqlc)
- [ ] `Dockerfile` (Go 1.26-alpine → distroless)
- [ ] `go mod tidy` + `go vet` + `go build` clean

## Success Criteria

- `cd services/api && go vet ./...` exit 0
- `go build -o /tmp/api ./cmd/api` exit 0, binary runs
- `PORT=8080 DATABASE_URL=postgres://test@localhost/x?sslmode=disable REDIS_URL=redis://localhost:6379 ENV=dev go run ./cmd/api` prints structured JSON log lines to stdout
- `curl -s localhost:8080/health | jq -r .status` → `ok`
- `curl -s localhost:8080/ready` returns either 200 or 503 — both acceptable here (PG/Redis may not be running pre-Phase-04). **[C2]** PG-dependent 503 test moved to `phase-04-local-infra.md` Success Criteria where infra exists.
- `docker build -t sbf-api .` succeeds; image ≤ 25MB via `docker images sbf-api` **[M2]**
- `SIGTERM` to running process → shutdown completes < 12s, no goroutine leak
- `go test ./... -race` exit 0 (health_test.go grounds CI race detector claim) **[C5]**

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| fiberzap version drift (contrib repo moves fast) | Med | Low | Pin exact version in `go.mod`; CI catches breakage |
| Distroless nonroot lacks `/tmp` for pprof | Low | Med | Acceptable Phase 1; add volume or switch to `distroless/base` if pprof needed Phase 10 |
| pgx SSL default causes local dev hang | High | Med | `.env.example` documents `sslmode=disable` for local; `sslmode=require` for prod |
| envconfig silently swallows missing required vars | Med | High | Explicit `Required:"true"` struct tags + explicit validation pass after Load() |
| Windows `go build` defaults to GOOS=windows | Med | Med | **[C6]** Makefile `build` enforces `CGO_ENABLED=0 GOOS=linux`; Docker build runs inside Linux container so is always correct; local builds without Makefile will cross-compile |

## Security Considerations

- `.env.example` values are placeholders only — no real secrets; ensure file ends `.example` so `.gitignore` excludes actual `.env`.
- Fiber `DisableHeaderNormalizing` left default (off) — good.
- No auth middleware yet — `/health` + `/ready` are public by design; all future endpoints must explicit `.Public()` (§20.2 rule).
- Distroless `nonroot` user (uid 65532) — no privileged operations needed.

## Next Steps

Unblocks Phase 03 (migrations need `services/api/internal/migrations/` path + goose lib), Phase 06 (CI needs `services/api/go.mod` + `.golangci.yml`). Provides `Makefile migrate-*` targets (Phase 03 wires goose lib behind them).
