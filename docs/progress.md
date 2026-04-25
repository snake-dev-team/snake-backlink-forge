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

---

## 2026-04-25 — Phase 2 Foundation Complete (50%)

### Done
- **Phase 01 Bot Skeleton** (`5ab2ab1`) — M smoke pass; long-poll FSM Redis, middleware chain, per-user mutex serialization, graceful shutdown 10s drain
- **Phase 02 User Service** (`581e32b`) — M1+M2 pass; F4 atomic trial gate (SELECT FOR UPDATE + conditional UPDATE + grant_credits in single tx, partial UNIQUE `idx_users_phone_trial` is the gate)
- **Phase 03 Key Service** (`0ac8fa2`) — M3-M6 pass; race-safe rotation (Serializable + 23505/40001 retry), partial UNIQUE `idx_keys_user_active_unique`, H5 rate limit /regenkey 3/day
- **Phase 04 Wallet + Ledger** (`fda0fc1`) — M7 pass; thin service wrapping `grant_credits`/`consume_credits` stored procs, caller-tx composability for Phase 06, /balance VND comma-separator format
- **Phase 05 Transactions + Topup** (`e9aa722` + `015ecb8`) — M8-M13 pass; 10 packages per §1.3, F2 12-hex provider_ref + retry, Q2 cancel preserves provider_ref, idempotency via `idx_tx_user_pkg_pending`, snapshot invariant on amount/credits, QR via SePay/MB/VietQR

### Stats cluster
- **13 manual E2E tests** pass (M1-M13: /start trial → /key /regenkey rotation → /balance → /buy keyboard → /topup QR → idempotency → cancel → re-buy)
- **80+ automated tests** pass (race + integration: Phase 01-05 service+util+bot packages)
- **0 critical findings** outstanding (all F1-F6 + H1-H7 + Q1-Q6 closed via planner round 1-3 + reviewer fix loops)
- **Foundation production-shape** verified — money-flow correctness audit CLEAN across Phase 04 + Phase 05

### Hook bug fixes shipped (bonus)
- `33b8a16` — `vendor/ignore.cjs` shim restored (ClaudeKit bundle missing module)
- `1712573` — Pattern-matcher Windows abs path normalize (drive letter strip + cwd-relative rebase)
- `2671a7e` — `$CLAUDE_PROJECT_DIR` for hook commands (fix recurring `loader:1459` when CWD changes via subprocess)

### Pending — Phase 06-10
- **Phase 06 SePay webhook** — *highest risk*, money-flow critical. Implements F1 over-payment 3-way branch + bonus credits, F3 race-safe CAS, Q2 cancel-then-pay recovery (`recovered_by_late_payment`), Q3 strict account+gateway match, Q4 20/s/IP rate limit, Q5 admin alert goroutine, H6 scoped notify ctx
- **Phase 07** History + Support — `/history` pagination, support ticket FSM, /campaigns scope-cut placeholder
- **Phase 08** Admin Commands — F5 audit-then-grant atomic, M3 self-ban guard, auth-fail burst alert producer
- **Phase 09** Message Templates VN — i18n key map, copywriter polish 5 new templates from round 3
- **Phase 10** Integration Tests + Deploy — Suite D race + Suite F rollback, Fly.io subdomain deploy with SePay verification fallback (CF Tunnel / custom domain)

### Resume instructions (next session)
1. **Verify stack:** `docker ps` (sbf_postgres + sbf_redis healthy) + bot process check (`netstat -ano | grep :8080`)
2. **Read spec:** `plans/260424-2135-phase-2-telegram-bot-wallet-sepay/phase-06-sepay-webhook.md`
3. **Cook Phase 06:** sequential, NO parallel (money flow risky); planner→fullstack-developer→code-reviewer→fix loops
4. **Code review** must reach APPROVED before manual webhook simulate
5. **Manual test Phase 06:** curl simulate SePay payload → assert credit grant + Telegram notify reach user; test recovery using cancelled tx `56D391967CC8` for Q2 late-pay
6. **DB invariants** to preserve: tg_id=8042306755 has active key (`sbf_live_CDv`) + pending tx `D49B493200EB` + cancelled tx `56D391967CC8` (Q2 recovery test target)

### Resume key facts
- **Bot running:** PID 4672 (port 8080) at break time. Kill via `taskkill //PID 4672 //F` if mày muốn fresh restart, hoặc keep running để resume nhanh
- **DB test user:** tg_id=8042306755, phone +84706706468, wallet 5/0 std/prem, key sbf_live_CDv active, 2 transactions (1 cancelled Q2-recovery-ready, 1 pending)
- **Bank env:** `SEPAY_BANK_CODE=MB` + `SEPAY_BANK_ACCOUNT=060320070000` configured in `services/api/.env`
- **Deploy URL:** still TBD — verify SePay accepts `.fly.dev` post-deploy (Outcome A); fallback Cloudflare Tunnel (B) or custom domain (C) per phase-10 checklist

### Estimated remaining
- **Phase 06:** ~2.5h (cook + review + fix loops + manual webhook simulate)
- **Phase 07-10:** ~7-10h combined
- **Total Phase 2 wrap:** ~10-13h, chia 2-3 ngày next sessions

---

## 2026-04-25 — Phase 06 Complete (60%) — money-flow shipped

### Done
- **Phase 06 SePay Webhook** (`7d6bde5`) — M14-M20 ALL PASS via curl simulate
  - F1 over-payment 3-way verified: under (no credit, manual_review) / exact (base only) / over (base+bonus)
  - F2 12-hex regex strict: `[A-F0-9]{12}` rejects non-hex (e.g. `M18EXACT12HX`), accepts `AAAAAAAAAA18`
  - F3 retry semantics: auth/business→200 success=false; lock→200 queued+LPUSH; pool→503; panic→500+alert (NEVER 500 for normal paths)
  - Q2 cancel-then-pay verified live: tx `56D391967CC8` (cancelled at 06:32) → webhook payload → flipped to `recovered_by_late_payment` + full credit grant + cancel metadata preserved
  - H1 audit + admin alert POST tx.Commit (no ghost rows on commit fail)
  - H2 ledger order: base premium/standard FIRST → topup_excess bonus AFTER (verified M20 balance_after sequence: 1005 then 1048)
  - H3 bonus DRY: single `int(diff/rate)` site, `overpaidResult` struct
  - C1 BodyLimit 64KB, C2 PII redact (HashIP 32-char, HashAccountPrefix 12-char), M1 recover() supervisors, M2 ProxyHeader X-Forwarded-For, M3 RootCtx audit
  - Idempotency 7× replay → 1 grant (CAS gate proven)
  - Admin alert ≥10K threshold (M20 71K excess fired alert), manual_review ≥50K (M20 metadata.manual_review=true)

### Phase 06 cluster commits
- `7d6bde5 feat(webhook): phase 2.06 sepay webhook + retry queue + admin alerts`
- `dcc0ad8 docs(reports): code review phase 06 fix loop 1 — APPROVED`
- `e9fb7cb docs(reports): code review phase 06 sepay webhook — fix-required`

### Test data state (preserved for Phase 07/08 testing)
- **user_id:** `24b84987-6ed8-403c-a656-751b7638f058`
- **tg_id:** 8042306755
- **wallet:** 1048 std / 0 prem / 1,717,000đ vnd_spent
- **transactions:** 5 rows
  - `D49B493200EB` paid (M14 happy path, 329K)
  - `56D391967CC8` recovered_by_late_payment (M17 Q2 late-pay, 329K)
  - `AAAAAAAAAA18` paid (M18 exact, 329K)
  - `BBBBBBBBBB19` paid overpaid=true (M19 small excess, 330K, bonus=0 floor)
  - `CCCCCCCCCC20` paid + manual_review=true (M20 large excess, 400K diff=71K bonus=43)
- **ledger:** ~10 entries (1 trial + 4 base topup + 1 topup_excess)
- **audit_log:** clean trail of `sepay_success` / `sepay_overpaid` / `sepay_replay` / `sepay_auth_fail` / `trial_granted` / `key_issued`

### Phase 2 progress: 6/10 phases complete (60%)
- Phase 01-05: foundation done (bot, user, key, wallet, tx)
- **Phase 06: webhook done** ← highest-risk money-flow phase shipped
- Phase 07-09: pending (read paths + admin + i18n)
- Phase 10: pending (deploy + production verify)

### Resume instructions next session
1. Verify stack: `docker ps` (sbf_postgres + sbf_redis healthy) + bot process check
2. Bot decision at break: PID 14928 (running) — kill/keep per user choice (see Live state)
3. Read specs: `plans/260424-2135-phase-2-telegram-bot-wallet-sepay/phase-07-history-support.md`, `phase-08-admin-commands.md`, `phase-09-message-templates.md`
4. **Cook Phase 07 + 08 + 09 PARALLEL OK** — no shared file overlap (Phase 09 templates touched by all but additive only). However sequential is also fine if cautious — money-flow guards already past
5. After 07/08/09 → cook Phase 10 sequential (deploy + production verify per plan-10 checklist)

### Resume key facts
- **Bot PID 14928** at break time. State: running on port 8080 with full Phase 01-06 code. `/health` 200, `/ready` db+redis ok, webhook route registered (returns 405 on GET, accepts POST per spec)
- **Test data preserved:** rich tx history for Phase 07 `/history` testing without re-onboarding
- **SePay webhook live:** tested via curl 7 scenarios; retry queue + admin alert + auth-fail watcher (Phase 08) wired
- **Deploy:** TBD Phase 10 — `.fly.dev` acceptance still to verify post-deploy (Outcome A/B/C in plan-10 checklist)

### Estimated remaining Phase 2
- **Phase 07-09:** ~4-5h combined (read paths + admin + templates — all lower risk than 06)
- **Phase 10:** ~2-3h (deploy + production verify + SePay sandbox dashboard binding)
- **Total Phase 2 wrap:** ~6-8h, 1-2 ngày next sessions

---

## 2026-04-25 (continued) — Phase 07-09 Complete (90%)

### Done (additional)
- **Phase 07** (`a385a18`) — `/history` paginated tx + ledger; `/support` FAQ + describe-ticket FSM (M5 one-shot, 4096 byte cap, 3-ticket cap); `/download` with InstallerURL + guide; `/ref` 6-char base58 + 23505 retry + 8-char fallback + idempotent EnsureCode + ProcessReferralOnStart atomic; `/language` VN/EN toggle. **H1+H2 fix loop** (Opus review caught Sonnet impl race bugs): IncrementReferralCount now uses tx.WithTx; EnsureCode distinguishes referrals_user_id_key vs code_key on 23505.
- **Phase 08** (`a385a18`) — `/admin` namespace (silent ignore for non-admin) with 5 subcmds: stats / grant (F5 Option A atomic) / ban (M3 self-ban guard) / unban / lookup (3-way heuristic by tg_id|phone|key_prefix); AuditService for atomic audit-with-grant; auditFailAlertWatcher goroutine (60s tick, 15min window, ≥20 threshold, bucket dedup); L3 config validator for ADMIN_TELEGRAM_IDS.
- **Phase 09** (`11f51e0`) — Central template registry: 56 Key constants × 2 langs = 112 bundle entries; sync.Map cached Renderer with VN fallback chain; MarkdownV2 escape helper; renderTpl/renderTplCtx facade with nil-safety; 14 cmd_*.go handlers + middleware refactored. **H1+H2 fix loop** (Opus): resendTopupQR template swap (KeyTopupQRCaption not KeyBuyConfirm), KeyDownloadUnavailable added for empty-URL fail-safe. NoSecretsInBundles + AllKeysListMatchesBundles tests enforce parity.

### Cluster commits Phase 07-09
- `11f51e0 feat(bot): phase 2.09 message templates registry + 14 handlers refactored`
- `774f3a0 docs(reports): code review phase 09 templates — APPROVED_WITH_FIXES`
- `a385a18 feat(bot): phase 2.07+08 history/support/admin/ref/language + auth-fail watcher`
- `4e09286 docs(reports): code review phase 07 history-support — APPROVED_WITH_FIXES`
- `b1998bd docs(reports): code review phase 08 admin commands — APPROVED_WITH_FIXES`

### E2E Test Results (M21-M39 hybrid PART A auto + PART B manual)
- **PART A auto: 12/12 PASS** — A1+A2 /history pagination integrity, A3+A4 support cap (3 max), A5 ref idempotent (UNIQUE on user_id), A6 admin stats 12 metrics, **A7 F5 grant atomic** (ledger ref_type=user UUID + audit metadata.ledger_id bigint in jsonb, single tx commit), A8 M3 self-ban guard (TestBan_M3 PASS), A9 lookup 3 patterns return same UUID, A10 watcher 41 audit rows seeded + dedup verified (single DM per 15-min bucket per spec), A11 webhook notify (placeholder per Phase 06 deferred design), A12 VN error template parity.
- **PART B manual: 7/7 PASS** — B1 /support menu, B2 /download fail-safe (KeyDownloadUnavailable rendered = H2 fix verified), B3 /language toggle EN→VN (DB UPDATE + render flip), B4 /admin menu (admin role recognized), **B5 /start fresh + VN tone APPROVED** (screenshot reviewed: pronoun consistent, bold emphasis, plaintext key code-block, warning UPPERCASE, CTA actionable), **B6 /buy keyboard VN** (5×2 layout, 10 packages, ⭐ markers, compact prices match §1.3), B7 EN full sweep (6 commands EN + revert clean).
- **Total: 19/19 PASS, 0 residual bugs.** All 4 Opus-caught HIGH bugs (Phase 07 H1 atomicity, H2 23505 user_id race, Phase 09 H1 template swap, H2 download fail-safe) verified fixed live.

### Final state pre-deploy
- **Local stack**: postgres + redis healthy (uptime 6+ hours), bot PID 4288 (current at break) running full Phase 01-09 code
- **DB test data**: clean fresh-onboarded user `tg_id=8042306755` post-cleanup with `sbf_live_DW9` active key + 5 std credits + language=vi (ready for Phase 10 production smoke)
- **Templates registry**: 56 keys × 2 langs loaded, sync.Map cache primed
- **Commands registered**: 21 total — `start`, `ping`, `key`, `regenkey`, `balance`, `buy`, `topup`, `history`, `support`, `download`, `ref`, `language`, `admin {stats,grant,ban,unban,lookup}`, plus 8+ inline keyboard callbacks

### Phase 2 progress: **9/10 (90%)** — final phase pending
| Phase | Status |
|---|---|
| 01-09 | ✅ COMPLETE locally |
| 10 Deploy | 🔜 PENDING (production verify) |

### Phase 10 pre-deploy checklist (mày làm TRƯỚC resume)
- [ ] Verify SePay merchant dashboard access (login OK, can create webhooks)
- [ ] Verify Fly.io CLI authenticated: `flyctl auth whoami`
- [ ] Verify Fly.io payment method active (Hobby tier $5 credit)
- [ ] Reserve app name: `flyctl apps create snake-backlink-api` (or alternative if taken)
- [ ] Check `fly.toml` exists in `services/api/` (Phase 10 cook will create if missing)
- [ ] Save 99K từ 1 account khác sẵn sàng test transfer (Standard Starter 50 = 99,000đ minimum)

### Resume instructions (next session)
1. Verify stack: `docker ps` + bot still running PID 4288 OR restart
2. Verify pre-deploy checklist all green
3. Read `plans/260424-2135-phase-2-telegram-bot-wallet-sepay/phase-10-integration-tests.md`
4. **Cook Phase 10 sequential** (deploy operations don't parallelize):
   - Step 1: Provision Fly.io infrastructure (app + postgres + redis OR external managed)
   - Step 2: `fly secrets set` ALL secrets from `.env`
   - Step 3: `fly deploy services/api`
   - Step 4: Verify `https://snake-backlink-api.fly.dev/health` → 200
   - Step 5: Create SePay webhook with `.fly.dev` URL → verify acceptance (Outcome A/B/C)
   - Step 6: REAL 99K transfer test → verify webhook fires + credits granted
   - Step 7: CI integration test suite full run với Linux `-race -count=100`
   - Step 8: Tag `v0.1.0-beta` release
5. Sau deploy verify → **Phase 2 100% COMPLETE** → product MVP launchable

### Estimated remaining
- **Phase 10 deploy:** 2-3h cook + setup
- **Pre-deploy blocker resolution** (if any): 30-60 min
- **Real money flow E2E test**: 30 min (transfer 99K + đợi webhook)
- **Total Phase 10 wrap**: ~3-4h next session






