---
name: Database layer
phase: 03
status: pending
priority: P0
estimated_effort: 2.5h
blockedBy: [phase-02-go-api-skeleton, phase-04-local-infra]
blocks: [phase-07-docs-housekeeping]
agents: [fullstack-developer, code-reviewer]
---

# Phase 03 — Database Layer

## Context Links

- MASTER_PROMPT §3.1 Full DDL (lines 527–881) — 12 tables, 2 stored procs, 1 view, 1 trigger
- MASTER_PROMPT §3.2 Seed dorks (lines 883–896)
- MASTER_PROMPT §1.4 Backlink types (line 167–174) — drives dork patterns distribution
- Go research §5 goose, §6 sqlc, §11 PG extensions (`plans/reports/researcher-260424-0247-go-stack-versions.md`)
- Prior phase: `phase-02-go-api-skeleton.md` (provides services/api Makefile)

## Overview

**Priority:** P0 · **Status:** pending · **Effort:** 2.5h
Ship full §3.1 schema as goose-embedded migrations + 30 dork pattern seed. Wire `Makefile migrate-*` to goose lib (not CLI) per researcher §5. Create empty sqlc query `.sql` files + `sqlc.yaml` v2. NO business queries yet — Phase 2+ fills them.

## Key Insights

- **Embed migrations** via `//go:embed migrations/*.sql` (researcher §5) — allows one binary to carry schema, easier ops + test.
- **sqlc v1.30 `emit_pointers_for_null_types: true`** required — §3.1 has nullable UUIDs (targets.owner_user_id, users.referred_by, audit_log.user_id/key_id).
- **Trigger `trg_campaign_limit`** enforces max 3 active campaigns at DB layer (belt+suspenders with app layer).
- **Stored procs `consume_credits` + `grant_credits`** — CORE business atomicity; raise `INSUFFICIENT_CREDITS` (ERRCODE P0001) → Go code matches via `pgconn.PgError.Code == "P0001"`.
- **View `v_user_stats`** — convenience for Phase 2 bot `/balance` aggregation, no materialization in Phase 1.
- **Seed 30 dork patterns** — 8 blog_comment + 8 forum_profile + 8 web2_post + 6 directory_listing, mix VN (`"tin tức"`, `"blog công nghệ"`) + EN (`"tech blog"`, `"niche"`) per §1.4 adapter coverage.

## Requirements

**Functional:**
- `20260424_001_init.up.sql` applies all §3.1 DDL clean on empty Postgres 16.
- `20260424_001_init.down.sql` drops all objects reverse order; rerun produces zero residue.
- `20260424_002_seed_dorks.up.sql` inserts exactly 30 rows; `.down.sql` truncates `dork_patterns`.
- `sqlc generate` produces Go code from empty query files (zero compile errors).
- `goose` embedded in `internal/db/migrator/migrator.go` — `Up(ctx)`, `Down(ctx)`, `Status(ctx)`.

**Non-functional:**
- Forward migration idempotent (`CREATE EXTENSION IF NOT EXISTS`, `DROP ... IF EXISTS` in down).
- `consume_credits` atomic under `pool_max_conns=15` concurrency (rely on row-lock via UPDATE WHERE CHECK).

## Architecture

```
services/api/
├── internal/
│   ├── migrations/
│   │   ├── embed.go                 (go:embed migrations/*.sql → FS)
│   │   ├── 20260424_001_init.up.sql       (§3.1 DDL full)
│   │   ├── 20260424_001_init.down.sql     (reverse drops)
│   │   ├── 20260424_002_seed_dorks.up.sql (30 INSERTs)
│   │   └── 20260424_002_seed_dorks.down.sql (TRUNCATE dork_patterns)
│   ├── db/
│   │   ├── db.go                   (existing from Phase 02)
│   │   ├── migrator/
│   │   │   └── migrator.go          (goose Up/Down/Status wrapper)
│   │   ├── queries/                 (empty .sql stubs)
│   │   │   ├── users.sql
│   │   │   ├── keys.sql
│   │   │   ├── wallets.sql
│   │   │   ├── campaigns.sql
│   │   │   ├── targets.sql
│   │   │   ├── jobs.sql
│   │   │   ├── ledger.sql
│   │   │   └── audit.sql
│   │   └── sqlc/                    (generated code COMMITTED — no sqlc CLI needed for fresh clone)
└── sqlc.yaml                        (v2, pgx/v5 engine)
```

## Related Code Files

**CREATE:**
- `services/api/internal/migrations/embed.go`
- `services/api/internal/migrations/20260424_001_init.up.sql` — copy-paste §3.1 lines 530–880 verbatim (extensions → users → api_keys → wallets → ledger_event_type enum → ledger → tx enums → transactions → campaign_status enum → campaigns → trg_campaign_limit → target enums → targets → job_status enum → jobs → domain_cooldown → dork_patterns → audit_log → safeguard_hits → consume_credits → grant_credits → v_user_stats)
- `services/api/internal/migrations/20260424_001_init.down.sql` — reverse order DROPs: view → functions → safeguard_hits → audit_log → dork_patterns → domain_cooldown → jobs → job_status enum → targets → target_* enums → campaigns → campaign_status enum → transactions → transaction_* enums → ledger → ledger_event_type enum → wallets → api_keys → users
- `services/api/internal/migrations/20260424_002_seed_dorks.up.sql` — 30 INSERT rows (template in §3.2, expand to 30)
- `services/api/internal/migrations/20260424_002_seed_dorks.down.sql` — `TRUNCATE dork_patterns RESTART IDENTITY;`
- `services/api/internal/db/migrator/migrator.go` (~50 LOC)
- `services/api/internal/db/queries/{users,keys,wallets,campaigns,targets,jobs,ledger,audit}.sql` — 8 files, each containing header comment only (`-- name: Placeholder :one\nSELECT 1;` so sqlc can parse; actual queries Phase 2+)
- `services/api/sqlc.yaml`
Note: No `.gitkeep` — sqlc generated files (`internal/db/sqlc/*.go`) are COMMITTED per controller decision #3 (predictable onboarding, no sqlc CLI required for fresh clone).

**MODIFY:**
- `services/api/Makefile` — wire `migrate-up`, `migrate-down`, `migrate-status`, `migrate-new name=X`, `sqlc-gen` to goose lib binary `./cmd/migrate` OR direct `go run` of migrator helper.
- `services/api/cmd/migrate/main.go` — NEW small CLI: reads DATABASE_URL, runs `migrator.Up/Down/Status` based on arg.

**DELETE:** none.

## Implementation Steps

1. `go get github.com/pressly/goose/v3@latest` and `github.com/sqlc-dev/sqlc` (dev tool — install CLI `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0`).
2. **[C4]** Create `services/api/internal/migrations/20260424_001_init.up.sql` — begin with `-- +goose Up`. Then wrap EACH of the 4 multi-statement blocks with its own `-- +goose StatementBegin` / `-- +goose StatementEnd` pair (goose v3 requires per-block markers; a single outer wrapper will NOT parse):
   - Block 1: `check_active_campaign_limit()` function (`CREATE OR REPLACE FUNCTION ... $$ ... $$ LANGUAGE plpgsql;`)
   - Block 2: `consume_credits()` function
   - Block 3: `grant_credits()` function
   - Block 4: `trg_campaign_limit` trigger (`CREATE TRIGGER ... EXECUTE FUNCTION ...;`)
   Single-statement DDL (CREATE TABLE, CREATE TYPE, etc.) does NOT need StatementBegin/End. Paste MASTER_PROMPT §3.1 lines 532–877 verbatim for all other DDL.
3. Create `20260424_001_init.down.sql` — `-- +goose Down` then reverse DROP order; use `DROP ... IF EXISTS` + `CASCADE` for enums tied to tables.
4. **[H5]** Create `20260424_002_seed_dorks.up.sql` — `-- +goose Up` then 30 `INSERT INTO dork_patterns (pattern, target_type, expected_platform) VALUES (...)`. Distribution: 8 blog_comment + 8 forum_profile + 8 web2_post + 6 directory_listing. Implementer drafts all 30 patterns based on the 5 examples from MASTER_PROMPT §3.2 + Google dorks best practices. Language mix: 20% VN keyword examples (`"bình luận"`, `"diễn đàn"`, `"tin tức"`, `"blog công nghệ"`, `"đăng ký"`, `"thành viên"`), 80% EN. Inline SQL is NOT committed in this plan file to keep file under 200 lines — code-reviewer validates patterns per checklist below.
5. Create `20260424_002_seed_dorks.down.sql`.
6. Create `internal/migrations/embed.go`:
   ```go
   package migrations
   import "embed"
   //go:embed *.sql
   var FS embed.FS
   ```
7. Create `internal/db/migrator/migrator.go` — funcs `Up(ctx, pool)`, `Down(ctx, pool)`, `Status(ctx, pool)` wrapping goose with `goose.SetBaseFS(migrations.FS)`, `goose.SetDialect("postgres")`, use stdlib `database/sql` adapter via `stdlib.OpenDBFromPool(pool)`.
8. Create `cmd/migrate/main.go` — reads `cfg.DatabaseURL`, `os.Args[1]` ∈ {up, down, status, redo}, dispatches.
9. Wire Makefile:
   ```
   migrate-up: ; go run ./cmd/migrate up
   migrate-down: ; go run ./cmd/migrate down
   migrate-status: ; go run ./cmd/migrate status
   migrate-new: ; @test -n "$(name)" && goose -dir internal/migrations create $(name) sql
   sqlc-gen: ; sqlc generate
   ```
10. Create 8 `internal/db/queries/*.sql` with placeholder: `-- name: PlaceholderSelect :one\nSELECT 1 AS dummy;` (enables sqlc parse; Phase 2 replaces).
11. Create `services/api/sqlc.yaml` v2:
    ```yaml
    version: "2"
    sql:
      - engine: "postgresql"
        queries: "internal/db/queries"
        schema: "internal/migrations/*.up.sql"  # [M6] glob: picks up all future migrations automatically
        gen:
          go:
            package: "sqlcdb"
            sql_package: "pgx/v5"
            out: "internal/db/sqlc"
            emit_pointers_for_null_types: true
            emit_json_tags: true
            emit_prepared_queries: false
    ```
12. Run `sqlc generate` — expect clean output, 8 files generated in `internal/db/sqlc/`.
13. **[C1]** Test: Phase 04 must be complete before running this step (Postgres from compose required). `make migrate-up && make migrate-down && make migrate-up` — expect zero errors, `SELECT COUNT(*) FROM dork_patterns` returns 30.
14. Commit generated sqlc output: after `sqlc generate` runs clean, `git add services/api/internal/db/sqlc/*.go` and commit. Generated code is committed (not gitignored) per controller decision #3.

## Todo List

- [ ] Install goose lib + sqlc CLI v1.30.0
- [ ] Write `20260424_001_init.up.sql` (full §3.1 DDL + per-block goose markers per C4)
- [ ] Write `20260424_001_init.down.sql` (reverse drops)
- [ ] Write `20260424_002_seed_dorks.up.sql` (30 patterns, VN+EN, 4 types — draft per H5)
- [ ] Write `20260424_002_seed_dorks.down.sql`
- [ ] Write `internal/migrations/embed.go`
- [ ] Write `internal/db/migrator/migrator.go`
- [ ] Write `cmd/migrate/main.go`
- [ ] Write 8 empty `internal/db/queries/*.sql` placeholders
- [ ] Write `sqlc.yaml` v2 (with glob schema per M6)
- [ ] Wire Makefile migrate-* + sqlc-gen targets
- [ ] Verify: migrate up+down+up cycle clean + sqlc generate clean + 30 dork rows (requires Phase 04 infra per C1)
- [ ] **[Controller #3]** Commit generated sqlc output after `sqlc generate` runs clean

## Success Criteria

- `make migrate-up` → exit 0, `psql -c "\dt"` shows 12 tables (users, api_keys, wallets, ledger, transactions, campaigns, targets, jobs, domain_cooldown, dork_patterns, audit_log, safeguard_hits)
- `psql -c "SELECT proname FROM pg_proc WHERE proname IN ('consume_credits','grant_credits','check_active_campaign_limit')"` returns 3 rows
- `psql -c "SELECT COUNT(*) FROM dork_patterns"` → 30
- `psql -c "SELECT * FROM v_user_stats LIMIT 1"` executes without error
- `make migrate-down` → exit 0, `\dt` returns empty (no residue)
- `sqlc generate` exit 0, `ls internal/db/sqlc/` has `db.go` + 8 query files (committed, not gitignored)
- `go build ./...` still clean after generated code lands

**[H5] Seed dork validation checklist (code-reviewer validates):**
- Exactly 30 rows: 8 blog_comment + 8 forum_profile + 8 web2_post + 6 directory_listing
- Platform coverage: wordpress/disqus/blogger/ghost, discourse/phpbb/vbulletin/flarum, medium/blogger/tumblr/substack, business/niche directories
- Language mix: ~6 VN keyword patterns (20%), ~24 EN patterns (80%)
- No SQL syntax errors in VALUES clauses; single-quote escaping valid
- `{niche}` placeholder present (template literal, NOT SQL interpolation)

**[Controller Decision #6] code-reviewer agent role in Phase 03:**
- Validates DDL matches MASTER_PROMPT §3.1 exactly (no typos in column types, FK, constraints)
- Validates `migrate-down` reverses cleanly with zero residue (drop order respects FK dependencies)
- Validates `consume_credits` / `grant_credits` stored proc logic is atomic (UPDATE WHERE row-lock, RAISE EXCEPTION on insufficient credits)

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Stored proc `$$` block breaks goose parser | High | High | Mandatory `-- +goose StatementBegin/End` wrappers around every `CREATE FUNCTION ... $$...$$` and `CREATE TRIGGER`; test on PG16 before commit |
| Enum drop order causes `cannot drop type ... because table X depends on it` | High | Med | Down migration drops tables first then enums with `DROP TYPE ... CASCADE` |
| sqlc v1.30 emits non-compiling code on placeholder queries | Low | Med | Use `SELECT 1 AS dummy;` syntax (valid pg query) + typed column; test `sqlc generate && go build` |
| Seed patterns embed SQL-injection risk via `{niche}` placeholder | Low | High | `{niche}` is a template literal replaced in Go code via parametrized builder, NOT SQL interpolation — document in `queries/targets.sql` Phase 4 |

## Security Considerations

- DB role used here = superuser/migration role. Production needs a separate `api_user` with `SELECT/INSERT/UPDATE/EXECUTE` only (§1.6 rule 8) — track as TODO in `docs/progress.md`; enforced Phase 10.
- Stored procs run `SECURITY INVOKER` by default (PostgreSQL default) — no privilege escalation.
- `audit_log.ip_hash BYTEA` — never stores raw IP; document hashing (sha256) in `docs/threat-model.md` Phase 7.

## Next Steps

Unblocks Phase 07 (docs mention migration flow). Does NOT block Phase 04, 05, 06 directly. Phase 2 Telegram bot will fill the empty query `.sql` files; this phase leaves them structurally valid.
