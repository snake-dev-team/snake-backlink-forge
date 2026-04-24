# 2026-04-24 — Machine Handoff + Phase 1 Verify Block A→G

Session goal: set up new machine from clean state, execute Phase 1 verify checklist, then relocate the repo from `C:\Users\ACER\OneDrive\Desktop\tool_backlink` to `E:\tool_backlink`.

## Starting state
- New machine (Acer Nitro ANV15-51, i9-13900H, RTX 4060, Windows 11)
- Repo cloned fresh via `git clone https://github.com/danhng876/snake-backlink-forge` → `C:\Users\ACER\OneDrive\Desktop\tool_backlink`
- Branch: `dev` at `53f74a5` (machine handoff commit)
- Existing tools: Node 24.14.1, Git 2.53, gh 2.89, Docker Desktop 29.3.1 (daemon off), make/Go/pnpm all missing

## Tools installed
| Tool | Version | Method |
|---|---|---|
| pnpm | 9.15.9 | `npm install -g pnpm@9.15.9` |
| Go | 1.26.2 | MSI via PowerShell `Start-Process -Verb RunAs` (winget silent install failed — UAC couldn't elevate from non-interactive subshell, had to kill stuck winget processes holding installer mutex) |
| make | 4.4.1 | `winget install ezwinports.make` |
| goose CLI | 3.27.0 | `go install github.com/pressly/goose/v3/cmd/goose@latest` |

## Secrets configured
`services/api/.env.local` (later renamed to `.env` since Go API uses godotenv default load):
- `DATABASE_URL`, `REDIS_URL`: local Docker Compose defaults
- `CLAUDE_BASE_URL=https://r7yyfje.9router.com`, `CLAUDE_MODEL=cc/claude-sonnet-4-6`
- `TELEGRAM_BOT_TOKEN`: live (`8492920644:...`)
- `SEPAY_WEBHOOK_TOKEN`: pre-generated via `openssl rand -hex 32`, prefixed `sbf_sepay_`. TODO comment: bind to SePay dashboard after Fly.io deploy + snakebacklink.com domain purchase
- `ADMIN_TELEGRAM_IDS=8042306755`
- `JWT_SECRET`: random 64-hex
- Phase 4-6 API keys (SERPAPI/MOZ/2captcha/CAPSOLVER/RESEND): empty (not needed until those phases)

## 9Router revival
Public endpoint `r7yyfje.9router.com` returned HTTP 530 (Cloudflare "Origin DNS error") at session start — trycloudflare tunnel was dead from machine handoff.

Diagnosis: 9Router is a local npm-global app at `E:\npm-global\9router` that spawns cloudflared via `C:\Users\ACER\.9router\bin\cloudflared.exe`. Fix: `9router --tray --skip-update -n` — spawned cloudflared, tunnel URL rotated to `yarn-academics-enquiries-listprice.trycloudflare.com`, DNS manager re-routes `r7yyfje.9router.com` to it. Verified with `POST /v1/messages` returning proper Anthropic 401 error shape (proxy forwarding correctly).

Saved to memory: `reference_9router.md`.

## Migration bugs fixed (2 PRs)

### PR #1 — `343698d` fix(db): unique migration versions — collapse date-seq underscore

**Layer 1 bug:** Migration files named `20260424_001_init.up.sql` etc. Goose parses version as prefix-before-first-underscore → all 4 files parsed as version=`20260424` → duplicate panic.

Rename: `YYYYMMDD_NNN_name.sql` → `YYYYMMDDNNN_name.sql` → versions become `20260424001` and `20260424002`, unique.

### PR #2 — `da20dba` fix(db): consolidate split up/down files to goose single-file format

**Layer 2 bug (exposed after layer 1 fix):** Goose v3 regex `^(\d+)_(.+)\.sql$` doesn't differentiate `.up.sql` from `.down.sql`. Both files with same prefix collide → duplicate panic persisted.

Root cause: Phase 03 mixed conventions — golang-migrate split-file naming (`.up.sql`/`.down.sql`) with goose single-file directives (`-- +goose Up`/`-- +goose Down` inside files). Only single-file is supported.

Consolidation: merged 4 files → 2 files:
- `20260424001_init.sql` (Up + Down sections, all `StatementBegin/End` markers preserved for 3 plpgsql functions + 1 trigger)
- `20260424002_seed_dorks.sql`

Also added to `docs/MASTER_PROMPT.md` §11.1: migration smoke test is MANDATORY for any DB schema phase going forward — testcontainers-go, no more deferred runtime tests.

**Smoke test before commit:** 2 full UP/DOWN cycles — 12 base tables + 30 dorks on UP, zero residue on DOWN, idempotent across cycles.

## Phase 1 verify results

All 7 blocks PASS on `dev` @ `da20dba`:

| Block | Command | Result |
|---|---|---|
| A | `make dev-setup` | `.env` templates at root + services/api |
| B | `make dev-up` | sbf_postgres + sbf_redis healthy (5432/6379) |
| C | `make migrate-up` | 12 base tables + 30 dork_patterns (after 2 PR fixes) |
| D | `make run` + curl `/health` `/ready` | Both HTTP 200 JSON, DB+Redis `ok` |
| E | docker stop/start postgres | 200 → 503 (`degraded`, `db=err`) → 200 recovery |
| F | `make migrate-down` ×2 | Zero residue: 0 base tables / 0 enums / 0 user functions / 0 views / 0 triggers |
| G | kill API + `make dev-down` | Port free, containers removed, volumes preserved |

## Repo relocated to E:
Moved `C:\Users\ACER\OneDrive\Desktop\tool_backlink` → `E:\tool_backlink` via `robocopy /MOVE /E` (641 MB / 25,707 files / 2m30s).

Post-move issues + fixes:
- 3 tracked files in `packages/shared-types/` missed (809 dir failures likely junction-related) → restored via `git restore packages/shared-types/`
- pnpm junction symlinks flattened to regular dirs → `biome` binary couldn't load → `rm -rf node_modules && pnpm install --frozen-lockfile` (5m3s) → verified `pnpm exec biome --version` returns 2.3.0

Working tree clean, git identity set globally (`Bfat-6z` / `minhhieu2007a@gmail.com`).

## Ready for Phase 2
All `progress.md` Phase 2 pre-requirements ✓:
- [x] Docker Desktop running
- [x] Phase 1 stack verified end-to-end
- [x] Telegram bot `@SnakeBacklinkBot` + avatar
- [x] SePay webhook token pre-generated (bind to dashboard post-deploy)
- [x] Admin Telegram ID captured
- [x] 9Router `cc/claude-sonnet-4-6` verified active
- [x] `services/api/.env` populated

Next: `/ck:plan --hard "Phase 2 Telegram Bot + Wallet + SePay"`.
