---
name: Local infra (docker-compose + root Makefile)
phase: 04
status: pending
priority: P1
estimated_effort: 1h
blockedBy: [phase-01-monorepo-backbone, phase-02-go-api-skeleton]
blocks: [phase-07-docs-housekeeping]
agents: [fullstack-developer]
---

# Phase 04 — Local Infra

## Context Links

- MASTER_PROMPT §1.2 ADR (Postgres 16, Redis 7 locked)
- MASTER_PROMPT §2 Repository Structure (`ops/docker-compose.yml` line 505)
- Go research §11 PG extensions, §12 Redis compose (`plans/reports/researcher-260424-0247-go-stack-versions.md`)
- Prior phase: `phase-02-go-api-skeleton.md` (provides `/ready` endpoint that pings both)
- Prior phase: `phase-03-database-layer.md` (provides `make migrate-up` to run after compose up)

## Overview

**Priority:** P1 · **Status:** pending · **Effort:** 1h
Create `ops/docker-compose.yml` (Postgres 16-alpine + Redis 7-alpine, named volumes, healthchecks) + root `Makefile` orchestrating `dev-up` / `dev-down` / `migrate` / `run-api`. No Grafana/Prometheus (Phase 10). Service ports exposed to host for local dev.

## Key Insights

- **Postgres 16-alpine** has `uuid-ossp`, `pgcrypto`, `citext`, `pg_trgm` out-of-box (researcher §11) — no custom image build needed.
- **Redis 7-alpine** requires `--appendonly yes --appendfsync everysec` to persist locally (researcher §12); Fly prod 256MB does NOT persist (accepted).
- **Healthchecks** mandatory: `pg_isready -U postgres` (PG), `redis-cli ping` (Redis) — prevents API from racing start.
- **Named volumes `sbf_pgdata` / `sbf_redisdata`** scope to project (prefix avoids collision with other local stacks).
- **Compose `.env` file** shared with API via `env_file: ../services/api/.env` (dev convenience; not used in CI/prod).

## Requirements

**Functional:**
- `docker compose -f ops/docker-compose.yml up -d` starts PG + Redis, both healthy within 10s.
- `docker compose down` stops cleanly; `docker compose down -v` wipes volumes.
- PG reachable on `localhost:5432`, credentials match `services/api/.env` (default `postgres://postgres:postgres@localhost:5432/sbf?sslmode=disable`).
- Redis reachable on `localhost:6379`, no auth (local dev).
- Root `Makefile`: `make dev` boots stack → migrate → run API; `make dev-down` tears down.

**Non-functional:**
- Compose file < 60 LOC.
- Postgres data persists across restarts (but `dev-reset` target wipes).
- First-run pull + start + migrate completes < 90s on 10Mbps.

## Architecture

```
ops/
└── docker-compose.yml
     ├── service: postgres
     │   image: postgres:16-alpine
     │   ports: 5432:5432
     │   env: POSTGRES_USER/PASSWORD/DB
     │   volume: sbf_pgdata:/var/lib/postgresql/data
     │   healthcheck: pg_isready
     └── service: redis
         image: redis:7-alpine
         command: redis-server --appendonly yes --appendfsync everysec
         ports: 6379:6379
         volume: sbf_redisdata:/data
         healthcheck: redis-cli ping

Makefile (root)
├── SHELL := bash                                            [H1] explicit bash shell
├── dev-up        → docker compose -f ops/docker-compose.yml up -d --wait   [H2] --wait replaces sleep 5
├── dev-down      → docker compose -f ops/docker-compose.yml down
├── dev-reset     → docker compose -f ops/docker-compose.yml down -v
├── dev-logs      → docker compose -f ops/docker-compose.yml logs -f
├── dev-setup     → cp services/api/.env.example services/api/.env           [C3] first-run env setup
├── migrate       → cd services/api && make migrate-up
├── run-api       → cd services/api && make run
└── dev           → dev-up && migrate && run-api   [H2] sleep 5 removed; --wait handles readiness
```

## Related Code Files

**CREATE:**
- `C:\Users\Hanna\Desktop\tool_backlink\ops\docker-compose.yml`
- `C:\Users\Hanna\Desktop\tool_backlink\ops\.gitkeep` (if `ops/` empty aside from compose)
- `C:\Users\Hanna\Desktop\tool_backlink\Makefile` (root)
- `C:\Users\Hanna\Desktop\tool_backlink\.env` placeholder for compose (gitignored; reads POSTGRES_* defaults)

**MODIFY:** none (first creation of `ops/` and root Makefile).

**DELETE:** none.

## Implementation Steps

1. Create `ops/docker-compose.yml` top-level `services:` map:
   - `postgres`: image `postgres:16-alpine`, env `POSTGRES_USER=${POSTGRES_USER:-postgres}`, `POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-postgres}`, `POSTGRES_DB=${POSTGRES_DB:-sbf}`, ports `"5432:5432"`, volume `sbf_pgdata:/var/lib/postgresql/data`, healthcheck `test: ["CMD-SHELL", "pg_isready -U $$POSTGRES_USER -d $$POSTGRES_DB"]`, interval `5s`, timeout `3s`, retries `10`.
   - `redis`: image `redis:7-alpine`, command `["redis-server", "--appendonly", "yes", "--appendfsync", "everysec"]`, ports `"6379:6379"`, volume `sbf_redisdata:/data`, healthcheck `test: ["CMD", "redis-cli", "ping"]`, interval `5s`, timeout `3s`, retries `10`.
2. Add top-level `volumes:` block: `sbf_pgdata: {}`, `sbf_redisdata: {}`.
3. Add top-level `networks:` block (optional): `sbf_default: {}` attached to both services.
4. **[H1]** Create root `Makefile` with `SHELL := bash` as the first non-comment line. Then add targets per architecture above; `.PHONY` all targets; use `@echo` for user feedback. `SHELL := bash` prevents silent `cmd.exe` execution on Windows when `make` is invoked from non-Git-Bash terminals.
5. Add `dev-reset: dev-down` then `rm -rf services/api/tmp` (if any) + `docker volume rm sbf_pgdata sbf_redisdata 2>/dev/null || true`.
   Add `dev-up` target using `docker compose -f ops/docker-compose.yml up -d --wait` — the `--wait` flag blocks until all healthchecks pass, replacing the racy `sleep 5` approach. **[H2]**
   Add `dev-setup` target: `cp services/api/.env.example services/api/.env` — safe first-run setup; skips if `.env` already exists (add `[ -f services/api/.env ] || cp ...`). **[C3]**
6. Ensure `services/api/.env.example` documents `DATABASE_URL=postgres://postgres:postgres@localhost:5432/sbf?sslmode=disable` and `REDIS_URL=redis://localhost:6379/0` matching compose defaults.
7. Smoke test: `make dev-up && docker compose -f ops/docker-compose.yml ps` → both services `Up (healthy)`. No `sleep 5` — `--wait` ensures healthchecks passed before prompt returns. **[H2]**
8. **[C3]** On fresh clone, run `make dev-setup` to copy `.env` template before `make dev`. Document in README quickstart.
9. Validate `/ready` handler (from Phase 02) returns 200 after `make migrate && make run-api`. **[C2]** This is where the PG-dependent 503 test lives (moved from Phase 02 Success Criteria).

## Todo List

- [ ] Write `ops/docker-compose.yml` (postgres:16-alpine + redis:7-alpine)
- [ ] Add named volumes + healthchecks
- [ ] **[H1]** Write root `Makefile` with `SHELL := bash` first line (dev-up --wait/down/reset/logs/migrate/dev-setup/run-api/dev)
- [ ] **[H2]** `dev-up` uses `docker compose up -d --wait` (no sleep 5)
- [ ] **[C3]** Add `dev-setup` target (`cp .env.example .env` first-run)
- [ ] Document `.env` defaults matching compose creds
- [ ] Smoke test: full `make dev` flow end-to-end

## Success Criteria

- `docker compose -f ops/docker-compose.yml config` validates syntax exit 0
- `make dev-up && docker compose ps --format json | jq '.Health' | grep -c healthy` → 2
- `psql postgres://postgres:postgres@localhost:5432/sbf -c "SELECT 1"` returns `1`
- `redis-cli -h localhost ping` returns `PONG`
- `make migrate` applies clean (from Phase 03)
- `curl -s localhost:8080/ready | jq -r .status` returns `ok` after Phase 02 server up with PG+Redis healthy
- **[C2]** Kill PG container → `curl -s localhost:8080/ready` returns 503 with `{"status":"degraded","postgres":"fail"}` (PG-dependent test moved here from Phase 02 where no infra existed)
- `make dev-down` stops containers; `make dev-reset` additionally removes volumes
- `make dev-setup` creates `services/api/.env` from `.env.example` on first run **[C3]**

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Port 5432 / 6379 already in use on dev machine | Med | Med | Document in README override: `POSTGRES_PORT=5433 make dev-up`; allow env var substitution in compose |
| Windows Docker Desktop volume perms cause PG init fail | Low | High | Use named volume (not bind mount) — Docker Desktop handles perms; avoid `- ./data:/var/lib/postgresql/data` |
| `sleep 5` race: migration runs before PG fully ready | Med | Med | Use `--wait` flag: `docker compose up -d --wait` which blocks until healthchecks pass |

## Security Considerations

- Default creds `postgres/postgres` — LOCAL DEV ONLY. `.env.example` explicitly warns; Fly prod uses distinct role.
- Redis exposed on `localhost:6379` no-auth — acceptable for dev; Fly prod uses `requirepass` + TLS.
- Named volumes stored in Docker Desktop data dir — fine for dev; ensure not backed up to cloud sync.
- `.env` (runtime) gitignored; `.env.example` shipped.

## Next Steps

Unblocks Phase 07 (README quickstart references `make dev`). Enables end-to-end smoke test of Phase 02 + 03 on local machine. Phase 2 bot service will mount same compose stack (no changes needed).
