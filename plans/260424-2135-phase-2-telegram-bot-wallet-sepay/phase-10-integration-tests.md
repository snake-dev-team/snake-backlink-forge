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

#### Suite C — Buy + Topup idempotency (`e2e_topup_test.go`)
- User with key + verified
- `/buy confirm` on `premium_pro_200` → pending tx + QR URL valid
- Rapid-fire 10x same click → exactly 1 pending row (race test)
- `/topup cancel` → status=failed
- New `/buy confirm` after cancel → new pending row OK

#### Suite D — SePay webhook atomic grant (`webhook_e2e_test.go`) — CRITICAL
- Setup pending tx (user X, order ABC12345, amount 329000, premium 200)
- POST /webhooks/sepay with valid Apikey + payload → 200 success
- Verify: wallet.premium_credits=200, tx.status=paid, ledger row, audit row `sepay_success`
- POST same payload again → 200 success (idempotent_replay), wallet unchanged
- 10 goroutines POST same payload concurrently → wallet.premium_credits EXACTLY 200 (not 400, not 2000)
- POST with wrong Apikey → 200 success=false, wallet unchanged, audit `sepay_auth_fail`
- POST with `Bearer ...` header → 200 success=false (wrong scheme)
- POST with content missing order code → 200 success, audit `sepay_unmatched_transfer`
- POST with transferAmount 100000 < required 329000 → 200 success, tx.status=manual_review, wallet unchanged
- POST with transferType="out" → 200 success ignored

#### Suite E — History + Support (`e2e_history_support_test.go`)
- User with 12 tx + 20 ledger rows → /history pagination across 3 pages, no dup, total matches count
- User creates 3 support tickets → 4th fails with cap message
- User changes language en → /balance in English

#### Suite F — Admin (`e2e_admin_test.go`)
- Non-admin `/admin stats` → no reply
- Admin `/admin stats` → monospace table with correct counts
- Admin `/admin grant 12345 standard 100 reason=support` → wallet +100, audit row with reason
- Admin `/admin ban 12345` → user.is_banned=TRUE, banned user's next `/balance` → "account disabled"
- Admin `/admin lookup 12345` → user summary rendered
- Admin `/admin grant 12345 invalid 100` → error "unknown pool"
- Admin `/admin grant 12345 standard -5` → error "amount must be > 0"

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

### Race test helper (`testutil/concurrent.go`)
```go
func FireN(n int, fn func() error) []error {
    var eg errgroup.Group
    errs := make([]error, n)
    for i := 0; i < n; i++ {
        i := i
        eg.Go(func() error { errs[i] = fn(); return nil })
    }
    _ = eg.Wait()
    return errs
}
```

## Related Code Files
### Create
- `services/api/internal/testutil/harness.go`
- `services/api/internal/testutil/fake_tgbotapi.go`
- `services/api/internal/testutil/concurrent.go`
- `services/api/internal/testutil/fixtures.go` — seed users/tx
- `services/api/internal/migrations/migrations_test.go`
- `services/api/internal/e2e/user_flow_test.go`
- `services/api/internal/e2e/topup_flow_test.go`
- `services/api/internal/e2e/webhook_flow_test.go`
- `services/api/internal/e2e/history_support_test.go`
- `services/api/internal/e2e/admin_flow_test.go`
- `services/api/internal/e2e/security_test.go`

### Modify
- `services/api/go.mod` — add `github.com/testcontainers/testcontainers-go`, `github.com/alicebob/miniredis/v2` (unit), `github.com/stretchr/testify`
- `services/api/Makefile` — add `test-integration` target (runs with `-tags=integration` to gate expensive tests)

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
- [ ] Add test deps to go.mod
- [ ] Implement test harness + fake tgbotapi
- [ ] Refactor bot to accept BotAPI interface
- [ ] Write Suite A: migrations
- [ ] Write Suite D: webhook atomic grant (10x concurrent)
- [ ] Write Suite B: user flow (trial + regen)
- [ ] Write Suite C: topup idempotency
- [ ] Write Suite E: history + support
- [ ] Write Suite F: admin
- [ ] Write Suite G: security fuzz
- [ ] Add `test-integration` Makefile target
- [ ] Update CI workflow for postgres/redis service containers
- [ ] Coverage check: ≥ 80% on service/, bot/, integration/sepay/
- [ ] Local run green
- [ ] CI run green

## Success Criteria
- `make test-integration` completes < 3min on dev machine
- `go test -race` passes across all suites
- Suite D concurrent webhook test: wallet credited EXACTLY ONCE under 10x concurrent
- Coverage ≥ 80% in target packages
- CI workflow green on first push to `dev`

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| testcontainers-go slow to pull image in CI | Med | Low | Cache Docker images in CI (`actions/cache`) |
| Flaky race test — rare schedule lets 2 concurrent win | Low | High | Sleep jitter between goroutines; run 100x in CI with `-count=100` on critical test |
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
- Monitoring / alerting for `sepay_auth_fail` burst — Phase 08+ (deploy).
