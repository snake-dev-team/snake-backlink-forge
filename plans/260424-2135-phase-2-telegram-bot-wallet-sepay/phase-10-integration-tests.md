# Phase 10 — Integration Tests + Race/Security Stress

## Context Links
- Spec: `docs/MASTER_PROMPT.md` §11.1 backend testing strategy — MANDATORY migration smoke + stress suites

## Overview
- **Priority:** P1 (gate for phase completion; no cook without tests)
- **Status:** pending
- **Description:** testcontainers-go driven end-to-end tests. Full flow: new user → start → balance → buy → webhook → history. Race tests on webhook idempotency. Spoof attempts on auth.

## Key Insights
- testcontainers-go spins real Postgres 16 + Redis 7 per test suite. Slow (~8s setup) but TRUE integration — catches migration bugs (per §11.1 lesson from Phase 1 Block C).
- Bot command tests mock `tgbotapi.BotAPI` via interface wrapper — don't hit real Telegram.
- SePay webhook tested via direct HTTP POST to local server — full Fiber stack, real DB.
- Race tests use `testing/quick` or `golang.org/x/sync/errgroup` with N goroutines firing the same payload.

## Requirements

### Test suites

#### Suite A — Migration smoke (`migrations_test.go`)
- testcontainers-go Postgres
- Run all migrations up → verify all expected tables + indexes + triggers
- Run all migrations down → verify clean drop
- Verify `idx_tx_user_pkg_pending` unique, `idx_users_phone_trial` unique present

#### Suite B — User + Key + Trial flow (`e2e_user_test.go`)
- Start bot in-process with mock tgbotapi
- Simulate Update: `/start` from new tg_id → reply includes "chia sẻ số điện thoại"
- Simulate Contact share → key issued, wallet=0/5, trial_used=TRUE, ledger row
- Repeat `/start` → "already verified" reply, no new credits
- Second tg_id with SAME phone → trial blocked, no grant
- `/regenkey confirm` → old key revoked, new key issued, plaintext returned once
- **[F4] Concurrent phone race** — 2 tg_ids, SAME phone, `VerifyContactAndGrantTrial` fired simultaneously via `FireN` → exactly 1 succeeds, 1 returns `ErrTrialPhoneReused` (partial index fires). Run in Suite D stress matrix too.
- **[H5] /regenkey rate limit** — 4th `/regenkey` in same day → reply `regen_rate_limited`, no new key issued, Redis key `regen_rl:<user_id>` value=4 with TTL ~86400s

#### Suite C — Buy + Topup idempotency (`e2e_topup_test.go`)
- User with key + verified
- `/buy confirm` on `premium_pro_200` → pending tx + QR URL valid
- Rapid-fire 10x same click → exactly 1 pending row (race test)
- `/topup cancel` → status=failed
- New `/buy confirm` after cancel → new pending row OK

#### Suite D — SePay webhook atomic grant (`webhook_e2e_test.go`) — CRITICAL
- Setup pending tx (user X, order ABCDEF012345, amount 329000, premium 200) — **[F2]** 12-hex order code
- POST /webhooks/sepay with valid Apikey + payload → 200 success
- Verify: wallet.premium_credits=200, tx.status=paid, ledger row, audit row `sepay_success`
- POST same payload again → 200 success (idempotent_replay), wallet unchanged
- 10 goroutines POST same payload concurrently → wallet.premium_credits EXACTLY 200 (not 400, not 2000)
- **[F6] Stress matrix**: `go test -race -count=100 -cpu=1,2,4,8 -run TestWebhookRace` → 100/100 iterations pass. Makefile target + CI step (see below).
- POST with wrong Apikey → 200 success=false, wallet unchanged, audit `sepay_auth_fail`
- POST with `Bearer ...` header → 200 success=false (wrong scheme)
- POST with content missing order code → 200 success, audit `sepay_unmatched_transfer`
- POST with transferAmount 100000 < required 329000 → 200 success, tx.status=manual_review, wallet unchanged
- POST with transferType="out" → 200 success ignored
- **[F1] Over-payment exact** — order `standard_pro_200` (329000đ, 200 std), transferAmount=400000 → wallet.standard=200+43=243, ledger rows `topup` + `topup_excess`, `metadata.manual_review != true` (diff=71000 < 50000 flag threshold)
- **[F1] Over-payment 50k flag** — diff=51000 → `metadata.manual_review=true`, credits granted, audit `sepay_overpaid`
- **[F1] Over-payment admin alert** — diff=15000 (≥10k) → `adminAlertCh` receives alert (assert via test consumer goroutine)
- **[Q2] Cancel-then-pay** — create pending → `CancelPendingTransaction` → POST webhook → tx.status='recovered_by_late_payment', wallet credited full, audit `sepay_success`
- **[Q3] account_mismatch** — payload.accountNumber="999999" → 200 success=false, audit `sepay_account_mismatch`, wallet unchanged
- **[Q3] gateway_mismatch** — payload.gateway="UnknownBank" → 200 success=false, audit `sepay_gateway_mismatch`
- **[Q4] rate_limit** — 21st POST within 1s from same IP → 429 (fake IP via middleware injection)
- **[F2] mixed-case memo** — content="sbf topup abcdef012345" → `ToUpper` → matches DB-stored `ABCDEF012345`, normal paid flow
- **[F3] deadlock_injection** — advisory-lock contention simulated via parallel tx holding lock on `wallets` row → webhook returns 200 queued_for_retry, Redis `sepay_retry_queue` has payload; consumer retry eventually commits once
- **[M6] real_payload_fixture** — `testutil/sepay_payload_real.json` (copy from SePay docs example) → parse succeeds, `TransferAmount` populated as int64
- **[round-3] `TestRetryQueueCapLTRIM`** — LPUSH 501 `RetryEnvelope` payloads manually → read `LLEN sepay_retry_queue` → assert stabilizes at 500 (cap enforced by `LTRIM 0 499`)
- **[round-3] `TestRetryQueueConsumerSuccess`** — LPUSH 1 valid envelope (matches a pending transactions row) → start consumer goroutine → wait 3s → assert `LLEN sepay_retry_queue=0`, wallet credited, ledger row exists, transaction.status='paid'
- **[round-3] `TestRetryQueueDeadLetter`** — LPUSH envelope with `payload.Content="SBF TOPUP DEADLETTER_TRIGGER"` (matches `const DeadLetterSentinel` recognized by consumer) → consumer treats as unrecoverable business error (`ErrDeadLetterSentinel`, not transient) → increments `env.Attempts` each iteration → after 3 failures → assert `LLEN sepay_retry_queue=0`, `LLEN sepay_dead_letter=1`, message NOT re-queued, `adminAlertCh` received `retry_dead_letter` alert exactly ONCE. Fire second identical dead-letter within same 5-min bucket → assert NO duplicate alert (bucket dedup via `now.Unix()/300` map). Advance fake clock by 5min → fire again → assert alert fires (new bucket).
- **[round-3] `TestRetryQueueBacklogAlert`** — LPUSH 401 envelopes → trigger consumer loop iteration → assert `adminAlertCh` received `retry_queue_backlog` alert; pop entries below 200 → re-fire LPUSH above 400 → assert alert fires again (dedup flag reset)

#### Suite E — History + Support (`e2e_history_support_test.go`)
- User with 12 tx + 20 ledger rows → /history pagination across 3 pages, no dup, total matches count
- User creates 3 support tickets → 4th fails with cap message
- User changes language en → /balance in English

#### Suite F — Admin (`e2e_admin_test.go`)
- Non-admin `/admin stats` → no reply
- Admin `/admin stats` → monospace table with correct counts
- Admin `/admin grant 12345 standard 100 reason=support` → wallet +100, ledger row (ref_type='user', ref_id=target_user_uuid), audit row (event='admin_grant', `metadata->>'ledger_id'` matches ledger.id, metadata.reason=support)
- Admin `/admin ban 12345` → user.is_banned=TRUE, banned user's next `/balance` → "account disabled"
- Admin `/admin lookup 12345` → user summary rendered
- Admin `/admin grant 12345 invalid 100` → error "unknown pool"
- Admin `/admin grant 12345 standard -5` → error "amount must be > 0"
- Admin `/admin grant 12345 standard 10001` → error "amount exceeds cap 10000"
- **[F5 Option A — round-3] `TestAdminGrantAtomicRollback`** — 3 cases:
  1. **Grant failure mid-tx**: invoke `AdminService.Grant` with non-existent `targetUserID` (random UUID not in users) → FK violation on ledger INSERT fires; assert `audit_log WHERE event='admin_grant' AND metadata->>'admin_tg_id'=$test_admin` = 0 rows; `ledger WHERE user_id=$target` = 0 rows; wallet for target unchanged.
  2. **Audit INSERT failure mid-tx**: install test-only trigger on audit_log `CREATE OR REPLACE FUNCTION test_audit_fail() RETURNS TRIGGER AS $$ BEGIN IF NEW.event='admin_grant' AND NEW.metadata->>'reason'='__test_fail_marker__' THEN RAISE EXCEPTION 'test induced failure'; END IF; RETURN NEW; END; $$ LANGUAGE plpgsql; CREATE TRIGGER t BEFORE INSERT ON audit_log ...`. Call Grant with reason `__test_fail_marker__` → audit INSERT raises; assert ledger rows for target + matching test marker = 0; wallet unchanged.
  3. **Happy path baseline**: normal Grant call → both rows committed → `audit_log.metadata->>'ledger_id' IS NOT NULL` AND equals actual ledger.id; wallet balance increased by `amount`.
  - Run: `go test -race -count=100 -run TestAdminGrantAtomicRollback ./internal/e2e/...`
- **[round-3] `TestAuthFailBurstAlert`** — seed 20 rows of `audit_log(event='sepay_auth_fail', created_at=NOW()-INTERVAL '1 minute')` → start `auditFailAlertWatcher` with test-only 1s ticker override → assert `AdminAlert{Kind:"auth_fail_burst"}` received on channel within 2s. Re-fire (same 15-min bucket) → assert NO duplicate alert (bucket dedup).
- **[M3]** Admin `/admin ban <self_tg_id>` → reply `admin_self_ban_blocked`, target (self) `is_banned` UNCHANGED, no audit row
- **[L3]** Boot config with `ADMIN_TELEGRAM_IDS="abc,123"` → fail with parse error
- **[L3]** Boot config with `ADMIN_TELEGRAM_IDS="123,123,456"` → boot succeeds, warn log "duplicate admin tg_id", final list `[123, 456]`

#### Suite G — Security fuzz (`security_test.go`)
- SQL injection attempt in `/start` contact phone → sanitized, no SQL error
- MarkdownV2 special chars in support ticket body → rendered safely via EscapeMDV2
- XL body POST on /webhooks/sepay (200KB) → rejected via BodyLimit
- 100 requests/sec on /webhooks/sepay with wrong auth → not OOM'd, still 200
- Malformed JSON → 200 success=false invalid_payload
- Replay attack: 10 distinct POSTs with valid auth but invalid order codes → all 200 success, zero wallet movement

### Non-functional
- `go test -race -timeout 5m ./...` passes
- Coverage ≥ 80% in `service/`, `bot/`, `integration/sepay/`
- Webhook p95 < 500ms measured in Suite D

## Architecture

### Test harness (`testutil/harness.go`)
```go
type Harness struct {
    PG     *pgxpool.Pool
    Redis  *redis.Client
    App    *fiber.App
    Bot    *bot.Bot         // in-process bot with fake tgbotapi
    Tmpl   *templates.Renderer
    Wallet *service.WalletService
    Webhook *service.WebhookService
    // ...
}

func NewHarness(t *testing.T) *Harness {
    ctx := context.Background()
    pgC, pgDSN := startPostgres(ctx, t)
    rdC, rdURL := startRedis(ctx, t)
    runMigrations(t, pgDSN)
    // ... build full deps, return *Harness
    t.Cleanup(func() { pgC.Terminate(ctx); rdC.Terminate(ctx) })
}
```

### Fake tgbotapi (`testutil/fake_tgbotapi.go`)
```go
// implements the interface subset bot package uses
type FakeBot struct {
    Sent []tgbotapi.Chattable
    mu   sync.Mutex
}
func (f *FakeBot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
    f.mu.Lock(); defer f.mu.Unlock()
    f.Sent = append(f.Sent, c)
    return tgbotapi.Message{}, nil
}
```

### Race test helper (`testutil/concurrent.go`) — [F6] true-simultaneous fire
```go
// FireN fires N concurrent executions of fn with a release barrier so all
// goroutines park at the start, then race simultaneously on a single close().
// This eliminates sequential dispatch jitter from errgroup.Go's scheduler
// preference on multi-core, making race conditions deterministically exposable.
func FireN(n int, fn func() error) []error {
    start := make(chan struct{})
    var wg sync.WaitGroup
    errs := make([]error, n)
    for i := 0; i < n; i++ {
        i := i
        wg.Add(1)
        go func() {
            defer wg.Done()
            <-start // park here until release
            errs[i] = fn()
        }()
    }
    // Brief park to ensure all goroutines reach the barrier.
    // (Runtime-dependent; 2ms adequate on modern schedulers.)
    time.Sleep(2 * time.Millisecond)
    close(start) // simultaneous release
    wg.Wait()
    return errs
}
```

### [F6] Stress matrix — Makefile + CI workflow
Makefile target (new in `services/api/Makefile`):
```make
test-integration-stress:
	go test -race -count=100 -cpu=1,2,4,8 -run 'TestWebhookRace|TestTrialRace|TestTopupIdempotency' ./internal/e2e/...
```

CI workflow step (new in `.github/workflows/ci.yml`):
```yaml
- name: Stress race tests
  run: make -C services/api test-integration-stress
```

Success criteria: **100/100 iterations pass across the -cpu=1,2,4,8 matrix** for each target test.

## Related Code Files
### Create
- `services/api/internal/testutil/harness.go`
- `services/api/internal/testutil/fake_tgbotapi.go`
- `services/api/internal/testutil/concurrent.go` — **[F6]** barrier-release `FireN`
- `services/api/internal/testutil/fixtures.go` — seed users/tx
- `services/api/internal/testutil/sepay_payload_real.json` — **[M6]** real-shape SePay payload fixture copied from docs
- `services/api/internal/migrations/migrations_test.go`
- `services/api/internal/e2e/user_flow_test.go`
- `services/api/internal/e2e/topup_flow_test.go`
- `services/api/internal/e2e/webhook_flow_test.go`
- `services/api/internal/e2e/history_support_test.go`
- `services/api/internal/e2e/admin_flow_test.go`
- `services/api/internal/e2e/security_test.go`

### Modify
- `services/api/go.mod` — add `github.com/testcontainers/testcontainers-go`, `github.com/alicebob/miniredis/v2` (unit), `github.com/stretchr/testify`, `github.com/jackc/pgerrcode`
- `services/api/Makefile` — add `test-integration` target (runs with `-tags=integration` to gate expensive tests) AND **[F6]** `test-integration-stress` target (100-iter matrix)
- `.github/workflows/ci.yml` — add `Stress race tests` step running `test-integration-stress`

## Implementation Steps
1. Add test deps to go.mod (`testcontainers-go`, `miniredis`, `testify`).
2. Write `testutil/harness.go` with shared setup/teardown.
3. Write `testutil/fake_tgbotapi.go` — subset interface for bot code.
4. Refactor `bot/bot.go` to accept a `BotAPI` interface (for test injection).
5. Write Suite A (migrations).
6. Write Suite D (webhook) FIRST — highest-value, catches most bugs.
7. Write Suites B, C, E, F, G in parallel once harness stable.
8. Add Makefile target `test-integration: go test -race -tags=integration ./...`.
9. Verify CI workflow `.github/workflows/ci.yml` runs integration suite (may need service containers: postgres + redis in CI runner).
10. Run local: `make test-integration` → all green.
11. Run load-test manually (k6 script) — DEFERRED to Phase 10 of master plan, not in scope here. Note in success criteria.

## Todo List
- [ ] Add test deps to go.mod (testcontainers-go, miniredis, testify, pgerrcode)
- [ ] Implement test harness + fake tgbotapi
- [ ] **[F6]** Implement barrier-release `FireN` in `testutil/concurrent.go`
- [ ] **[M6]** Copy real SePay payload example → `testutil/sepay_payload_real.json`
- [ ] Refactor bot to accept BotAPI interface
- [ ] Write Suite A: migrations (include 20260424003 + 20260424004)
- [ ] Write Suite D: webhook atomic grant (10x concurrent)
- [ ] **[F1]** Suite D: over-payment exact + 50k flag + admin alert channel receives
- [ ] **[Q2]** Suite D: cancel-then-pay recovery
- [ ] **[Q3]** Suite D: account_mismatch + gateway_mismatch
- [ ] **[Q4]** Suite D: rate_limit 21st/s → 429
- [ ] **[F2]** Suite D: mixed-case memo
- [ ] **[F3]** Suite D: deadlock_injection → 200 queued + retry queue
- [ ] **[M6]** Suite D: real_payload_fixture parse
- [ ] **[round-3]** Suite D: `TestRetryQueueCapLTRIM` — LPUSH 501 → LLEN stable at 500
- [ ] **[round-3]** Suite D: `TestRetryQueueConsumerSuccess` — LPUSH 1 valid → drained + wallet credited
- [ ] **[round-3]** Suite D: `TestRetryQueueDeadLetter` — perma-fail payload → 3 retries → dead-letter list + admin alert
- [ ] **[round-3]** Suite D: `TestRetryQueueBacklogAlert` — LPUSH 401 → `retry_queue_backlog` alert fires (dedup reset below 200)
- [ ] Write Suite B: user flow (trial + regen)
- [ ] **[F4]** Suite B: concurrent phone race (2 tg_ids, same phone)
- [ ] **[H5]** Suite B: /regenkey rate limit 4th call
- [ ] Write Suite C: topup idempotency
- [ ] Write Suite E: history + support (+ **[M5]** body cap + one-shot state)
- [ ] Write Suite F: admin (+ **[F5 Option A — round-3]** `TestAdminGrantAtomicRollback` 3 cases + **[round-3]** `TestAuthFailBurstAlert` dedup + **[M3]** self-ban + **[L3]** config parse)
- [ ] Write Suite G: security fuzz
- [ ] Add `test-integration` Makefile target
- [ ] **[F6]** Add `test-integration-stress` Makefile target (`-race -count=100 -cpu=1,2,4,8`)
- [ ] Update CI workflow for postgres/redis service containers
- [ ] **[F6]** Add CI step `Stress race tests`
- [ ] Coverage check: ≥ 80% on service/, bot/, integration/sepay/
- [ ] Local run green
- [ ] **[F6]** Stress run 100/100 pass across -cpu matrix
- [ ] CI run green

## Success Criteria
- `make test-integration` completes < 3min on dev machine
- **[F6]** `make test-integration-stress` passes 100/100 iterations across `-cpu=1,2,4,8` matrix for `TestWebhookRace|TestTrialRace|TestTopupIdempotency`
- `go test -race` passes across all suites
- Suite D concurrent webhook test: wallet credited EXACTLY ONCE under 10x concurrent
- Coverage ≥ 80% in target packages
- CI workflow green on first push to `dev`

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| testcontainers-go slow to pull image in CI | Med | Low | Cache Docker images in CI (`actions/cache`) |
| Flaky race test — rare schedule lets 2 concurrent win | Low | High | **[F6]** Barrier-release `FireN` + `test-integration-stress` Makefile target runs `-race -count=100 -cpu=1,2,4,8` matrix in CI |
| Fake tgbotapi drifts from real API shape | Med | Med | Interface kept minimal; upgrade tgbotapi in lockstep |
| Windows Docker Desktop incompatibility with testcontainers | Med | Med | Doc alt: run via WSL2; CI on ubuntu-latest primary gate |
| Test harness DB state leaks between tests | Med | High | Each test gets fresh DB container (slow but isolated); for fast tests use `TRUNCATE` between tests in same container |
| Admin role check test requires hardcoded tg_id | Low | Low | Harness provides `harness.SetAdminTGID(int64)` helper; test-only config override |

## Security Considerations
- Tests never hit real Telegram / real SePay — all mocked/localhost.
- Test secrets (webhook token) loaded from test fixture, never from `.env.local`.
- Harness tears down containers on `t.Cleanup` — no residual DB state cross-run.

## Next Steps
- On green: commit + push + create PR if applicable.
- Load test (k6) and chaos test deferred to Phase 10 of master plan (ops).
- Monitoring / alerting for `sepay_auth_fail` burst — wired via `adminAlertCh` in phase-06 (Q5) + `auditFailAlertWatcher` goroutine in phase-08 (round-3).
- **Pre-deploy blocker (Q4):** research SePay source IPs at https://docs.sepay.vn. If docs publish static IPs → add IP allowlist at Fly firewall level. If not published → fall back to rate limit (20/s/IP) as sole gate. Document outcome before prod deploy.

## [round-3] Deploy target

- **Phase 2-9 production target:** `https://snake-backlink-api.fly.dev` (Fly.io auto-assigned subdomain — custom domain deferred per plan.md ADR).
- **Integration tests run locally via Docker Compose** (`docker compose up`) — NOT against Fly.
- **End-of-Phase-2 smoke test (manual):**
  1. `cd services/api && fly deploy` — deploy to Fly staging
  2. `curl https://snake-backlink-api.fly.dev/health` → 200 OK
  3. `curl https://snake-backlink-api.fly.dev/ready` → 200 OK (DB + Redis reachable)
  4. POST sample SePay payload (from `testutil/sepay_payload_real.json`) to `https://snake-backlink-api.fly.dev/webhooks/sepay` with test Apikey → 200 success OR 200 success=false expected branch
- **Pre-deploy checklist additions:**
  - [ ] Verify SePay dashboard accepts `https://snake-backlink-api.fly.dev/webhooks/sepay` as webhook URL
  - [ ] If SePay rejects `.fly.dev` → configure Cloudflare Tunnel: `cloudflared tunnel create sbf-webhook` → get `*.trycloudflare.com` subdomain → point at Fly app IPv4 → update SePay dashboard
  - [ ] If SePay rejects both `.fly.dev` AND `trycloudflare.com` → purchase `snakebacklink.com` (~$10 Namecheap), configure DNS A record to Fly app IPv4, wait 10-30min propagate, re-verify SePay dashboard. Sets `api.snakebacklink.com/webhooks/sepay` as production webhook URL.
  - [ ] `SEPAY_BANK_ACCOUNT` + `SEPAY_BANK_CODE` + `SEPAY_WEBHOOK_TOKEN` + `ADMIN_TELEGRAM_IDS` set via `fly secrets set`
  - [ ] Run migration `20260424003` + `20260424004` via `fly ssh console -C 'make migrate-up'` during low-traffic window (migration 004 NO TRANSACTION gap)
- **Local dev (PART 1):** `ngrok http 8080` → public `*.ngrok.io` URL → configure SePay sandbox webhook with tunnel URL for round-trip testing.
