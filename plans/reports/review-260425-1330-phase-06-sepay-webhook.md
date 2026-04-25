# Code Review — Phase 06 SePay Webhook

## Verdict
FIX_REQUIRED

## Summary
Strict bar: 0 critical tolerated. Found **2 critical** money/PII bugs + **3 high** correctness/contract violations + **3 medium** + **3 low**. Money-flow CAS gate is correct, race-safety is solid, base grants math is right, but **(a) production body limit not enforced**, **(b) audit_log leaks raw IP and raw account number** (PII), **(c) admin alert and audit row fire BEFORE tx.Commit so commit-fail produces ghost alerts/rows**, **(d) ledger row order inverted** (topup_excess inserted before base topup). Tests pass and confirm credit math; the failing surface is in the production wiring + post-success side-effects, which the test harness does not exercise.

Build/test claims spot-checked: payload regex tests + verify tests + handler tests + integration tests structurally sound. Race-detector skip is documented; -count=5 stress is acceptable substitute.

## Money-flow correctness audit (CRITICAL)
- [x] F1 over-payment 3-way branch correct (underpaid blocks credits, exact==0 falls through, >0 grants bonus)
- [x] perCreditRate pool routing for combos — combo p100_s50: 999000/150=6660 → premium pool. Math sound.
- [x] AddVNDSpent uses transferAmount (full received) — webhook_service_process.go:104. Confirmed.
- [x] Q2 cancel-then-pay → recovered_by_late_payment + full credits — CTE captures pre_status correctly.
- [x] CAS WITH/RETURNING captures pre_status correctly — webhook_service_process.go:50-72
- [x] Underpaid path NEVER credits — handleUnderpaid commits + audits, no Grant call.
- [ ] **Overpaid bonus inserted BEFORE base grants — ledger row order reversed.** Spec (phase-06 lines 49-51) mandates premium → standard → bonus. Code: `handleOverpaid` (with bonus topup_excess Grant) runs inside `case diff > 0` BEFORE `applyBaseGrants`. Functionally idempotent on totals; user /history will show topup_excess preceding topup. **High** (UX/observability bug, not money loss).
- [x] No double grant under concurrent replay — TestProcessPaidTransaction_ConcurrentRace_10x asserts.
- [ ] **Admin alert + sepay_overpaid audit row fire BEFORE tx.Commit.** If commit fails, admin gets a false ALERT and audit_log has an `sepay_overpaid` row for a payment that never landed. webhook_service_process.go:250 (audit) + :269 (alert) inside tx; commit at processTransaction line 109. **CRITICAL.**

## Race + atomicity audit
- [x] CAS gate atomic (single SQL with CTE + FOR UPDATE)
- [x] ReadCommitted sufficient for this CTE pattern (FOR UPDATE inside CTE serialises concurrent webhooks for same provider_ref)
- [x] AlreadyProcessed branch on ErrNoRows (not RowsAffected since QueryRow.Scan)
- [x] Concurrent test 10x → exactly 1 grant — passes per implementer; mechanism sound.
- [x] handleNoRowsCase out-of-band lookup (paid/recovered → AlreadyProcessed; missing → UnknownOrder; manual_review/failed/refunded → AlreadyProcessed)

## Security audit
- [x] Apikey VerifyApikey constant-time via crypto/subtle — verify.go:27
- [x] Empty expected token = false (no auto-allow on misconfig) — verify.go:20-22 + test
- [x] No token in logs — grepped zap.* + Authorization, only header-name reads
- [ ] **PII leak in audit metadata**: webhook.go:24 stores raw IP `c.IP()`, webhook.go:47 stores raw account number `p.AccountNumber`. Schema (audit_log.ip_hash BYTEA + comment "sha256(ip), never store raw IP" — migration 20260424001 line 254 + queries/audit.sql line 3 + threat-model.md) is **violated**. Spec (phase-06 §security line 590 and §architecture line 91 — "received_acc_sha256_prefix") mandates sha256. **CRITICAL.**
- [ ] **Body limit 64KB not enforced in production.** server.go:21-32 omits BodyLimit; default is 4MB. router.go:27 comment "Body size cap: Fiber app-level BodyLimit (64KB) set in server.go covers this route" — **untrue**. Test app sets it locally (webhook_test.go:37,287) but `api.New()` does not. **CRITICAL.**
- [x] CAS gate prevents replay with valid token — IN ('pending','cancelled')

## Critical findings

### C1 — Production body limit missing
- File: `services/api/internal/api/server.go:21-32`
- Issue: `fiber.New(fiber.Config{...})` does NOT set `BodyLimit: 64 * 1024`. Fiber default is 4 MB → spec §non-functional line 59 + plan §security line 583 violated.
- router.go:27 comment is inaccurate.
- Impact: 64x oversized payloads accepted in prod. Memory pressure / DoS amplification. Spec violation on a hard-bound webhook.
- Fix: add `BodyLimit: 64 * 1024,` to fiber.Config in server.go.

### C2 — Raw IP + raw account number in audit_log metadata (PII leak)
- File: `services/api/internal/api/handlers/webhook.go:23-24, 46-47`
- Issue 1: `webhookAuditLog(deps, "sepay_auth_fail", map[string]any{"ip": c.IP()})` — raw IP stored in JSON metadata. Schema column `audit_log.ip_hash BYTEA` exists for sha256(ip); ignored.
- Issue 2: `webhookAuditLog(deps, "sepay_account_mismatch", map[string]any{"received_account": p.AccountNumber, "gateway": p.Gateway})` — full account number persisted. Plan §architecture line 91 specifies `received_acc_sha256_prefix: sha256Hex(p.AccountNumber)[:12]`.
- Issue 3: `deps.Log.Warn("sepay webhook auth fail", zap.String("ip", c.IP()))` — raw IP in log. Plan §security line 590-591 mandates sha256-hashed identifiers, full 32-char hex.
- Impact: GDPR / Vietnamese Personal Data Protection Decree exposure. Raw IPs persist in audit_log for years; raw account numbers identify the payment counterparty.
- Fix: import crypto/sha256 + encoding/hex, compute `ipHash := sha256.Sum256([]byte(c.IP()))`, store hex(ipHash[:]) in metadata AND populate `audit_log.ip_hash` BYTEA column. Same for account number → `received_acc_sha256_prefix`.

## High findings

### H1 — Admin alert + audit row fire pre-commit on overpaid
- File: `services/api/internal/service/webhook_service_process.go:250-281`
- Issue: `handleOverpaid` calls `s.auditLog(ctx, ...)` (pool-based, persists across rollback) AND `s.adminAlertCh <- AdminAlert{...}` while still inside the active tx. Caller `processTransaction` does the actual `tx.Commit(ctx)` later (line 109). If commit fails (deadlock at commit, infrastructure error), admin gets a false ALERT and `sepay_overpaid` audit row exists for a payment that never granted credits.
- Impact: false admin alerts on commit-fail; audit log inconsistency; admin response wasted.
- Spec: plan line 284-285 explicitly says "alert admin post-commit (defer setting flag; actual send below after Commit succeeds)" — code violates this.
- Fix: defer the alert send + audit log to AFTER `tx.Commit` returns nil. Restructure `handleOverpaid` to compute bonus + flag manual_review only (DB ops inside tx); return bonus + alert payload to caller; caller emits alert post-commit.

### H2 — Ledger row order inverted (bonus before base)
- File: `services/api/internal/service/webhook_service_process.go:88-92, 95-98`
- Issue: For overpaid case, `handleOverpaid` (which inserts `topup_excess` ledger row via wallet.Grant) runs BEFORE `applyBaseGrants` (which inserts `topup` ledger rows). Per spec phase-06 lines 49-51 (steps 5-7), order is premium-base → standard-base → bonus.
- Impact: /history page (Phase 07) and ledger reads will show bonus credit row preceding the base credit row. Confusing for users; reconciliation logic that assumes "base before bonus" will misorder.
- Fix: in `processTransaction`, call `applyBaseGrants` first, then a new `handleOverpaidGrantOnly` that just does the bonus Grant + manual_review flag. Move audit + alert post-commit (combined with H1 fix).

### H3 — Bonus computation duplicated (DRY violation, drift risk)
- File: `services/api/internal/service/webhook_service_process.go:113-120` vs `:215-220`
- Issue: `processTransaction` recomputes `bonus` after commit even though `handleOverpaid` already computed it. Two parallel computations against same inputs — risk of future drift if rate formula changes in only one place.
- Impact: Now: identical results, no bug. Future: maintenance hazard.
- Fix: hoist `bonus` to local var assigned inside the `case diff > 0` branch, returned alongside `overpaidBonus` from handleOverpaid; compute once.

## Medium findings

### M1 — No panic-recover on retry / admin-alert consumer goroutines
- Files: `services/api/internal/service/retry_consumer.go:46-132`, `services/api/internal/bot/admin_alerts.go:42-65`
- Issue: Plan §risk-table-line-580 ("Consumer spawned in cmd/api/main.go bound to rootCtx; on panic, recover() + restart loop") is not implemented. A panic in BRPOP decode / Grant call / DM send crashes the goroutine; the queue stalls until process restart.
- Fix: wrap top-of-loop body with `defer func() { if r := recover(); r != nil { log.Error("retry consumer panic", zap.Any("panic", r)); /* restart loop */ } }()` per iteration, or wrap whole goroutine in a supervisor that restarts on panic.

### M2 — Rate-limit key resolves to LB IP on Fly.io (no ProxyHeader)
- File: `services/api/internal/api/server.go:21-32`
- Issue: `fiber.Config` does not set `ProxyHeader` (e.g. `X-Forwarded-For`). On Fly.io / behind a load balancer, `c.IP()` returns the upstream proxy IP — all webhook requests share the same key → rate limit becomes effectively global, not per-IP.
- Spec acknowledges this as deploy concern (line 586) but ProxyHeader config is missing.
- Fix: add `ProxyHeader: "X-Forwarded-For"` (or `Fly-Client-IP`) and `EnableTrustedProxyCheck: true` + `TrustedProxies: [...]`. Document in Phase 10 deploy checklist.

### M3 — webhookAuditLog uses context.Background, not RootCtx
- File: `services/api/internal/api/handlers/webhook_deps.go:141`
- Issue: `_, _ = deps.Pool.Exec(context.Background(), ...)` for audit row insert. Survives server shutdown but also outlives request cancellation. Acceptable for audit (best-effort), but inconsistent with the broader pattern of scoping side-effects to RootCtx.
- Fix: use `deps.RootCtx` so audit writes drain on shutdown signal alongside other consumers. Low priority.

## Low / nits

### L1 — Bonus rate "premium" label incorrect for combos with PremiumCredits=0
- File: `services/api/internal/service/webhook_service.go:79-83`
- Issue: `if premiumCr > 0 { return "premium", amount/total }` — this is correct, but the `else` returns "standard". For a hypothetical premium-only package where `premiumCr=0` (theoretically impossible, but defensive), the rate goes to standard pool. Currently no such package exists → no immediate bug, but defensive comment would help.

### L2 — No audit_log row on body-parse failure or transferType=out
- File: `services/api/internal/api/handlers/webhook.go:30-39`
- Issue: BodyParser failures and outgoing transfer events produce no audit_log row. Forensics gap if attacker probes with malformed bodies.
- Fix: emit `sepay_invalid_payload` and `sepay_outgoing_transfer` (info-level) audit rows. Optional.

### L3 — TestRetryQueueConsumer_DeadLetter skipped is documented but test harness coupling on miniredis closure
- File: `services/api/internal/service/retry_consumer_test.go:107-115`
- Skip rationale (sentinel const non-hex 18 chars not extractable from regex) is correct. Replacement test ViaMaxAttempts at line 117+ exercises same code path. Acceptable.

## Plan spec alignment

| Item | Status | Note |
|---|---|---|
| Auth via Apikey (not Bearer) | OK | verify.go:23 prefix check |
| subtle.ConstantTimeCompare | OK | verify.go:27 |
| Empty expected → fail-closed | OK | verify.go:20-22 |
| Rate limit 20/s/IP Redis fixed window | OK | rate_limit_webhook.go:58 (caveat: ProxyHeader missing M2) |
| Account match | OK | webhook.go:42-50 |
| Gateway match | OK | webhook.go:52-60 |
| OrderCode regex case-insensitive | OK | payload.go:27 + handler ToUpper at webhook.go:75 |
| CAS widened to ('pending','cancelled') | OK | webhook_service_process.go:54 |
| F1 3-way amount branch | OK | webhook_service_process.go:85-92 |
| Bonus credits in topup_excess ledger event | OK | webhook_service_process.go:242 |
| manual_review flag at >=50k | OK | webhook_service_process.go:223 |
| Admin alert at >=10k | OK (but fired pre-commit — H1) | webhook_service_process.go:262 |
| AddVNDSpent uses transferAmount | OK | webhook_service_process.go:104 |
| Telegram notify async with scoped ctx (H6) | OK (placeholder) | webhook.go:101-107 |
| Body limit 64KB | **MISSING IN PROD** | C1 |
| All audit events present | OK (10/10) | grep confirmed |
| Retry queue producer + consumer + dead-letter | OK | retry_consumer.go full file |
| 5-min bucket dedup on dead-letter alerts | OK | retry_consumer.go:157-170 |
| Backlog alert >400 / reset <200 | OK | retry_consumer.go:188-199 |
| ip_hash sha256 / account sha256_prefix | **MISSING** | C2 |
| Panic recover on consumers | **MISSING** | M1 |

## Test adequacy

- 7 regex tests cover case sensitivity, length boundaries, embedded content. ✓
- 6 verify tests cover correct token, mismatch, no-prefix, empty-expected, same-length-wrong. ✓
- 11 handler tests cover auth pass/fail, transferType=out, account/gateway mismatch, unmatched, nil svc, invalid body, rate-limit, body-limit. ✓
- 10 service integration tests cover happy, idempotent replay, concurrent race 10x, unknown order, underpaid, overpaid (small + large), Q2 cancel recovery, mixed-case memo. ✓
- 4 retry consumer tests cover success drain, dead-letter via max attempts, backlog alert; sentinel test skipped with documented rationale. ✓
- Admin alert tests cover delivery, channel-full drop, ctx cancel, channel close, format-alert kinds. ✓

**Coverage gaps:**
- No test verifies that on `tx.Commit` failure, admin alert is NOT sent (would catch H1 today).
- No test verifies ledger row order between `topup` and `topup_excess` (would catch H2).
- No test exercises `processTransaction` with a forced commit error (e.g. close pool between Grant and Commit).
- No production-mode test for BodyLimit (test app sets explicit limit; would not catch C1).

## Approved items

- Money math: bonus rate computation correct for all 10 packages including combos.
- CAS gate atomicity: WITH/RETURNING + FOR UPDATE pattern is sound; race test confirms 1-of-10 wins.
- Q2 cancel→recovered_by_late_payment transition: correct via CTE pre_status capture.
- Idempotency: ErrNoRows path → handleNoRowsCase → AlreadyProcessed for paid/recovered/manual_review/failed/refunded.
- Apikey verification: constant-time, fail-closed on empty, prefix-checked.
- Rate-limit middleware design: pipelined INCR+EXPIRE, fail-open on Redis error (documented).
- Retry queue: bounded LTRIM 0 499, BRPOP blocking, max-3 dead-letter, backlog alert with high/low watermarks, 5-min bucket dedup on dead-letter alerts.
- Admin alert channel: notify/ package breaks service↔bot import cycle cleanly.
- F3 error classifier: pgErr 40P01/40001 + DeadlineExceeded → retry queue; pool exhaust → 503; default → 500 + alert.
- ProcessResult struct surfaces all branch flags (Underpaid, AlreadyProcessed, UnknownOrder, Overpaid, WasCancelled, BonusCredits) for handler templating.

## Recommended actions (priority order)

1. **C1** add `BodyLimit: 64 * 1024` to fiber.Config in `server.go`.
2. **C2** sha256-hash IP and account number; store ip_hash in `audit_log.ip_hash` BYTEA column; replace raw `ip` and `received_account` in metadata with `ip_sha256_hex` and `received_acc_sha256_prefix`. Add corresponding sanitisation in zap.Warn log line.
3. **H1** restructure `handleOverpaid` to defer audit + admin alert until AFTER `tx.Commit` returns nil. Return alert payload as ProcessResult side-channel; emit in caller post-commit (or via deferred closure with a "committed" flag).
4. **H2** invert order: call `applyBaseGrants` first, then bonus Grant. Or split `handleOverpaid` into `handleOverpaid_PreGrant` (manual_review flag) + `handleOverpaid_Bonus` (Grant) + `handleOverpaid_PostCommit` (alert + audit).
5. **H3** dedupe bonus computation — single source of truth in `handleOverpaid`, returned via `(bonus int)` to processTransaction.
6. **M1** wrap `RetryQueueConsumer` and `ConsumeAdminAlerts` body in a panic-recover supervisor that restarts the loop on panic with a log line.
7. **M2** add `ProxyHeader: "X-Forwarded-For"` (or `Fly-Client-IP`) + `TrustedProxies` to fiber.Config. Document in Phase 10 deploy checklist.
8. **M3** thread RootCtx into `WebhookDeps.AuditCtx` for audit writes. Optional.
9. **L1/L2/L3** acceptable as-is; track for future hardening.

Add tests:
- Force `tx.Commit` failure → assert no `sepay_overpaid` audit row and no admin alert.
- Assert ledger query `ORDER BY created_at` returns `topup` rows before `topup_excess`.
- Production-mode test of BodyLimit using `api.New()` directly.

## Unresolved questions

- Q1: Does Fly.io webhook ingress provide `Fly-Client-IP` header reliably? If yes, prefer it over `X-Forwarded-For` to avoid spoofing.
- Q2: Should `webhookAuditLog` use the request user_id when known (e.g. on overpaid where we DO know user_id post-CAS)? Current handler always passes user_id=NULL; service-layer auditLog includes it for overpaid/underpaid/success. Inconsistent — by design or oversight?
- Q3: Is `audit_log.metadata` size-bounded? `sepay_raw` payload is currently stored both in `transactions.metadata` AND would be stored verbatim in audit metadata if we expanded it. Confirm 8KB cap (jsonb soft limit) sufficient.
- Q4: Implementer flagged `notifyTelegramSuccess` as Phase 07 placeholder. Confirm Phase 07 plan picks up this thread; otherwise webhook reports "success" but user gets no DM.
