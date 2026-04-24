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

