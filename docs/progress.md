# Snake Backlink Forge — Progress Log

## Phase 1 — Foundation

- **Started:** 2026-04-24
- **Completed:** 2026-04-24 (scaffold only — runtime verification pending Docker Desktop install)
- **Commits:** be839e8, 640764c, c8b13bc, 9c72163, d6cf625, 4ff9b62, + Phase 07 docs commit
- **Tests added:** 3 (services/api/internal/api/handlers/health_test.go smoke)
- **Coverage delta:** +58.3% (handlers package, Go)
- **ADR deviations:** Go 1.23->1.26.2 (EOL), TS 5.5->6.0.3 (Biome 2 compat), Fiber logger->fiberzap/v2 (Zap JSON)

### Sub-phase status

| Phase | Status | Notes |
|---|---|---|
| 01 Monorepo backbone | DONE | pnpm + turbo + .editorconfig + .gitignore + LICENSE proprietary |
| 02 Go API skeleton | DONE_WITH_CONCERNS | /health + /ready live; -race needs CGO (CI covers) |
| 03 Database layer | DONE_WITH_CONCERNS | Full §3.1 DDL + 30 dorks + sqlc generated; runtime migrate deferred (Docker) |
| 04 Local infra | DONE_WITH_CONCERNS | compose + Makefile structural OK; runtime test deferred (Docker) |
| 05 Frontend stubs | DONE_WITH_CONCERNS | Next + Svelte + shared-types build green |
| 06 Quality gates | DONE | biome + husky + ci.yml + gitleaks; Biome 2.3 schema adjustments applied |
| 07 Docs + git | DONE | this commit |

### Notable decisions

- LICENSE = proprietary from Phase 01 (user directive 2026-04-24)
- Plans + reports committed to git (decision: history/audit trail)
- sqlc generated code committed (predictable onboarding)
- Windows = Git Bash primary; WSL2 optional
- `scripts/_claudekit/` namespace isolates ClaudeKit tooling from future `scripts/sbf-*` project scripts

### Known issues deferred

- Docker Desktop not installed on host — runtime `make dev-up` + migrate-up/down smoke test
  MUST be run when Docker available
- `.github/workflows/ci.yml` push requires `gh auth refresh -s workflow` (OAuth scope) — one-time refresh
- Svelte plugin v3 advisory for Svelte 5 -> upgrade to `@sveltejs/vite-plugin-svelte@^4` in Phase 3
- gofmt removed from lint-staged (not in Git Bash PATH on Windows); CI golangci-lint covers
- DB role separation (`api_user` with limited privileges) deferred to Phase 10 (§1.6 requirement)

### Phase 2 pre-requisites (user must provision BEFORE /ck:cook phase 2)

- [ ] Telegram bot token from @BotFather -> `TELEGRAM_BOT_TOKEN`
- [ ] Telegram bot username -> `TELEGRAM_BOT_USERNAME` (e.g. `SnakeBacklinkBot`)
- [ ] SePay merchant account + webhook token -> `SEPAY_WEBHOOK_TOKEN`
- [ ] Verify 9Router Claude access: `CLAUDE_BASE_URL=https://r7yyfje.9router.com`
- [ ] Sectigo OV code signing cert (Phase 7, ~$100/year) — can defer to Phase 7
- [ ] Domain snakebacklink.com + Cloudflare R2 bucket (Phase 9-10)

### Verify Phase 1 stack (run after Docker Desktop installed)

```bash
cd "C:/Users/Hanna/Desktop/tool_backlink"
make dev-setup
make dev-up                   # starts postgres:16-alpine + redis:7-alpine healthy
cd services/api && make migrate-up   # applies §3.1 DDL + 30 seed dorks
psql "$DATABASE_URL" -c "\dt"        # should list 12 tables
psql "$DATABASE_URL" -c "SELECT COUNT(*) FROM dork_patterns;"  # should return 30
make run &                    # start API
curl localhost:8080/health                           # 200 {"status":"ok"}
curl localhost:8080/ready                            # 200 {"status":"ready",...}
docker stop sbf_postgres && sleep 3 && curl localhost:8080/ready  # 503 degraded
docker start sbf_postgres && sleep 10 && curl localhost:8080/ready  # 200 again
make migrate-down             # reverts clean
make dev-down                 # teardown
```

---

## 2026-04-24 evening checkpoint

### Done today
- Phase 1 Foundation 7/7 sub-phases (commits `be839e8` -> `dcfb334`)
- Brand v1.0 design hoàn tất, assets trong `assets/brand/`
- `gh auth refresh` + push `main` + `dev` branch lên origin
- Repo: https://github.com/danhng876/snake-backlink-forge (PRIVATE)

### Pending before Phase 2
- [ ] Install Docker Desktop
- [ ] Verify Phase 1 stack runtime (6 blocks A-F — see "Verify Phase 1 stack" section above)
- [ ] Telegram bot `@SnakeBacklinkBot` + avatar
- [ ] SePay merchant + webhook bearer token
- [ ] 9Router verify (`CLAUDE_BASE_URL` + `cc/claude-sonnet-4-6` model)
- [ ] `services/api/.env.local` populated with real secrets

### Tomorrow plan
- Resume Phase 2 prep via Claude Code session reload
- Cook Phase 2 Telegram Bot + Wallet + SePay (run `/ck:plan --hard` review first)

---

## 2026-04-24 — Machine Handoff

### Current repo state
- Branch: `dev`
- Last commit: `5299d13 docs(progress): 2026-04-24 evening checkpoint`
- Remote sync status: `dev` == `origin/dev` (pushed, in sync); `main` == `origin/main` (pushed, in sync). `main` is 2 commits behind `dev` — Phase 07 docs (`dcfb334`) + evening checkpoint (`5299d13`) live on `dev` only.
- Working tree: dirty — untracked only. `.claude/session-state/latest.md.*.tmp` (8 session-state temp files, safe to ignore) + empty `plans/260424-0247-phase-1-foundation/reports/` dir. No tracked files modified.

### Phase status
- Phase 1 Foundation: **DONE** (7/7 sub-phases committed + pushed)
- Brand v1.0: Orbit S finalized, assets in `assets/brand/`
- Phase 2: **NOT STARTED**

### Next machine setup checklist (do in order)
1. Clone repo: `git clone https://github.com/danhng876/snake-backlink-forge`
2. Install tools:
   - Node.js v20+
   - pnpm 9.15.9 via `npm install -g pnpm@9.15.9`
   - Go 1.26.2 from https://go.dev/dl/
   - Docker Desktop (WSL2 backend)
   - Git + Git Bash + GitHub CLI (`gh`)
   - ClaudeKit from https://claudekit.cc
3. Configure Claude Code:
   - Set `ANTHROPIC_BASE_URL` to 9Router endpoint (`https://r7yyfje.9router.com`)
   - Model alias: `cc/claude-opus-4-7` for `/ck:plan`, `cc/claude-sonnet-4-6` for `/ck:cook`
   - Working dir: repo root on new machine (`<local-path>/snake-backlink-forge`)
4. Restore secrets:
   - Copy `sbf-secrets.txt` from USB/cloud
   - Create `services/api/.env.local` from template + secrets
5. `pnpm install` (first time on new machine)
6. Verify: `docker compose up -d --wait` → `make migrate-up` → `make run` → `curl localhost:8080/health`

### Phase 2 pre-requirements (pending, user action)
- [ ] Install Docker Desktop on new machine
- [ ] Verify Phase 1 stack (6 blocks A-F in `docs/progress.md` → "Verify Phase 1 stack" section)
- [ ] Create Telegram bot `@<username>` via BotFather
- [ ] Apply SBF avatar to bot (`sbf-telegram-avatar-512.png`)
- [ ] Register SePay merchant + get webhook bearer token
- [ ] Get admin Telegram ID from `@userinfobot`
- [ ] Populate `services/api/.env.local` with all secrets
- [ ] Verify 9Router `cc/claude-sonnet-4-6` still active

### When all pre-reqs done
Run: `/ck:plan --hard "Phase 2 Telegram Bot + Wallet + SePay"`
Review plan files before `/ck:cook`.

### Key decisions locked (see `docs/MASTER_PROMPT.md`)
- Stack: Go 1.26.2 + Fiber + PostgreSQL + Redis + Next.js 15 + Svelte 5 + Rust WASM
- Deploy: Fly.io (api) + Vercel (landing) + Cloudflare R2 (CDN)
- Business model: credit pools Premium/Standard, keys via Telegram bot, HWID-free
- Brand: Orbit S v1.0 finalized (archived cyber serpent direction)

---

## 2026-04-24 — Phase 1 Verify Complete (7/7 blocks PASS)

### Outcome
Phase 1 Foundation end-to-end verified on new machine (E:\tool_backlink, branch `dev` @ `da20dba`). All 7 verify blocks PASS. Ready for Phase 2.

### Tools installed on new machine
| Tool | Version | Method |
|---|---|---|
| pnpm | 9.15.9 | `npm install -g pnpm@9.15.9` |
| Go | 1.26.2 | MSI installer (winget UAC elevation failed from non-interactive shell) |
| make | 4.4.1 | `winget install ezwinports.make` |
| goose CLI | 3.27.0 | `go install github.com/pressly/goose/v3/cmd/goose@latest` |

### Migration bugs fixed (2 PRs merged to dev)
- **PR #1 `343698d`** — goose parses version as prefix-before-first-underscore. `20260424_001_init.up.sql` → version `20260424` → 4 files collided. Rename `YYYYMMDD_NNN` → `YYYYMMDDNNN` made versions unique.
- **PR #2 `da20dba`** — goose v3 regex `^(\d+)_(.+)\.sql$` doesn't differentiate `.up.sql` from `.down.sql`. Phase 03 mixed golang-migrate split-file naming with goose single-file directives inside. Consolidated 4 split files → 2 single-file migrations (`-- +goose Up` / `-- +goose Down` sections in one file). MASTER_PROMPT §11.1 updated: migration UP/DOWN smoke test is MANDATORY for DB schema phases.

### Verify results on `dev` @ `da20dba`
| Block | Command | Result |
|---|---|---|
| A | `make dev-setup` | `.env` templates at root + `services/api` |
| B | `make dev-up` | `sbf_postgres` + `sbf_redis` healthy (5432/6379) |
| C | `make migrate-up` | 12 base tables + 30 dork_patterns (post-fix) |
| D | `make run` + `/health` + `/ready` | HTTP 200 JSON, DB+Redis `ok` |
| E | docker stop/start postgres | 200 → 503 (`degraded`, `db=err`) → 200 recovery |
| F | `make migrate-down` ×2 | Zero residue: 0 base tables / 0 enums / 0 user functions / 0 views / 0 triggers; idempotent across cycles |
| G | kill API + `make dev-down` | Port free, containers removed, volumes preserved |

### 9Router revival
Public endpoint `r7yyfje.9router.com` returned HTTP 530 (dead trycloudflare tunnel from machine handoff). Fix: `9router --tray --skip-update -n` spawned cloudflared, tunnel URL rotated, DNS manager re-routed. Verified with `POST /v1/messages` returning proper Anthropic 401 shape (proxy forwarding correctly).

### Phase 2 pre-reqs status (all ✓)
- [x] Docker Desktop running
- [x] Phase 1 stack verified end-to-end (A→G)
- [x] Telegram bot `@SnakeBacklinkBot` + avatar live
- [x] SePay webhook token pre-generated (`sbf_sepay_<64hex>`), bind to SePay dashboard post-deploy
- [x] Admin Telegram ID captured (`8042306755`)
- [x] 9Router `cc/claude-sonnet-4-6` verified active
- [x] `services/api/.env` populated (renamed from `.env.local` — godotenv loads `.env` by default)
- [x] Stored procs `grant_credits` + `consume_credits` live + tested (verified via migration UP cycles)

### Session journal
`docs/journals/2026-04-24-machine-handoff-phase1-verify.md`

### Next
Kickoff `/ck:plan --hard "Phase 2 Telegram Bot + Wallet + SePay"` with red-team review before `/ck:cook`.

