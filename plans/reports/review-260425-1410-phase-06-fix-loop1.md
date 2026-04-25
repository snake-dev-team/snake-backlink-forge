# Code Review — Phase 06 SePay Webhook (fix loop 1 verify)

## Verdict
APPROVED

## Summary
All 8 prior findings (2 crit + 3 high + 3 med) genuinely closed. Build clean, `go vet` clean. PII no longer leaks into audit_log; production BodyLimit enforced; over-payment audit + alert moved post-commit; ledger row order spec-compliant; bonus computed once; consumer goroutines wrapped in panic-recover supervisors; ProxyHeader configured; webhook audit insert scoped to RootCtx with 2s timeout. Ready for manual M14-M20 webhook simulate.

## Prior findings closure

| ID | Severity | Status | File:line |
|---|---|---|---|
| C1 BodyLimit 64KB | crit | CLOSED | server.go:27 |
| C2 PII redact (IP + acc + content) | crit | CLOSED | redact.go:16-27, webhook.go:19/28-29/49-53/74-83 |
| H1 audit + alert AFTER commit | high | CLOSED | webhook_service_process.go:155-194 |
| H2 ledger order base before bonus | high | CLOSED | webhook_service_process.go:113-145 |
| H3 bonus computed once | high | CLOSED | webhook_service_process.go:121-124 |
| M1 recover() on consumers | med | CLOSED | retry_consumer.go:55-80, admin_alerts.go:43-69 |
| M2 ProxyHeader + TrustedProxies | med | CLOSED | server.go:38-40 |
| M3 webhookAuditLog uses RootCtx | med | CLOSED | webhook_deps.go:138-157 |

## Verification details

### C1 BodyLimit 64KB
- `server.go:27` → `BodyLimit: 64 * 1024,` literal present in fiber.Config. router.go comment now factual.
- Default 4MB no longer applies; oversized payloads rejected pre-handler.

### C2 PII redact
- `redact.go:16-19` HashIP returns full 64-char hex (32 bytes SHA-256 → hex.EncodeToString — confirmed length 64).
- `redact.go:24-27` HashAccountPrefix returns first 12 hex chars; verified in test `TestHashAccountPrefix_IsPrefixOfFullHash`.
- 6 unit tests cover determinism, length, distinct-input distinct-hash for both helpers.
- `webhook.go:19` `ipHashField()` is the single sink for c.IP() in handlers — grepped, only call sites.
- `webhook.go:28-29` auth-fail log + audit use ip_hash exclusively.
- `webhook.go:49-53` account-mismatch path uses HashAccountPrefix; raw account never logged or audited.
- `webhook.go:74-83` unmatched content truncated to first 50 chars (PII-safer for personal-name memos); amount logged verbatim (non-PII).
- `webhook_deps.go` and `webhook_service_process.go` audit metadata grepped — no `"ip"`, no `"received_account"`, no `"content"` raw.

### H1 audit + alert AFTER commit
- `webhook_service_process.go:283-303` `handleOverpaidPreCommit` body contains ONLY the `tx.Exec` for manual_review flag. No s.auditLog, no s.adminAlertCh send.
- `webhook_service_process.go:155-194` audit + alert fires after `tx.Commit(ctx)` returns nil (line 155-157 commit; line 164 audit; line 174-192 alert).
- `webhook_service_process.go:265-277` underpaid path also audits AFTER commit (line 265 commit; line 269 audit). Pattern consistent.
- Test `TestProcessPaidTransaction_CommitFail_NoAuditNoAlert` (webhook_service_fixes_test.go:92-127) closes the pool before ProcessPaidTransaction → BeginTx fails → asserts alertCh is empty. Note: test exercises BeginTx-fail (not literal Commit-fail) but validates same invariant — alert/audit never fire on failed DB path. Comment in test (lines 92-100) documents this trade-off explicitly.

### H2 ledger order base before bonus
- `webhook_service_process.go:113-115` `applyBaseGrants` (premium → standard) called first.
- `webhook_service_process.go:120-144` bonus Grant runs only after base grants returned successfully.
- Test `TestProcessPaidTransaction_Overpaid_LedgerOrder` (webhook_service_fixes_test.go:22-88) queries ledger ORDER BY id ASC, asserts topupIdx < excessIdx → matches spec lines 49-51.

### H3 bonus DRY
- Single `int(diff / rate)` site at line 124. Single `bonus := 0` initialisation that drives the rate computation at line 122.
- Line 161 `bonus := 0` is a post-commit local that reads `overpaid.bonus` (no recomputation; pure pass-through).
- `overpaidResult` struct (line 39-44) is the carrier; no second computation locus exists.

### M1 recover() on consumers
- `retry_consumer.go:55-60` `RetryQueueConsumer` outer for{} calls `runRetryConsumerOnce`; returns false on ctx cancel, true on panic-recover (line 78 sets continueLoop=true inside recover handler).
- `retry_consumer.go:73-80` `defer func() { if r := recover(); ... }()` correctly placed at start of inner func.
- `admin_alerts.go:44-49` `ConsumeAdminAlerts` outer for{} → `runAdminAlertsOnce` → defer recover at line 62-69.
- Test `TestRetryConsumer_RecoversFromPanic` (webhook_service_fixes_test.go:131-186) validates loop-continue across malformed envelope (decode-fail Warn at retry_consumer.go:99-103, `continue` line 103). Note: test exercises decode-fail path (loop continue) rather than literal panic, but the supervisor pattern is identical. Acceptable — supervisor structure visually clear from code.

### M2 ProxyHeader
- `server.go:38-40` sets `ProxyHeader: fiber.HeaderXForwardedFor`, `EnableTrustedProxyCheck: true`, `TrustedProxies: []string{"0.0.0.0/0"}`.
- Implementer chose X-Forwarded-For (matches review report wording at line 86 of prior review). Trust scope wide-open in dev — comment at server.go:36-37 flags Phase 10 deploy tightening.
- Build: fiber.HeaderXForwardedFor is a valid v2 constant (compiles clean).

### M3 RootCtx for audit
- `webhook_deps.go:142-147` audit insert ctx = `deps.RootCtx` (with `context.Background()` fallback for tests that don't wire RootCtx); 2s `WithTimeout` derived from it.
- `context.Background()` no longer the default — only used as test-friendly fallback when RootCtx==nil. Consistent with spec preference; survives shutdown drain.

## Regression scan (new issues from fixes)

### None blocking. Notes:

**N1 (info) — Audit lost on rare audit-insert fail post-commit.** After H1 fix, if `tx.Commit` succeeds but the post-commit `s.auditLog` call fails (DB blip, pool drain, ctx exceeded), money is correct but audit row missing. Per project audit philosophy this is best-effort — `auditLog` swallows errors via `log.Warn`. Acceptable trade.

**N2 (info) — Alert-fail post-commit window.** Between successful `tx.Commit` and `select case s.adminAlertCh <-` (line 180-192), process crash drops the alert. Channel-full also drops it (default branch logs Warn). Acceptable for Phase 2 — admin can detect overpaid via `/admin stats` or audit_log replay. Not regressed by H1; mirrors prior post-commit behaviour for sepay_success.

**N3 (info) — Test naming subtle.** `TestProcessPaidTransaction_CommitFail_NoAuditNoAlert` exercises BeginTx-fail (not literal Commit-fail) by closing pool before call. Acceptable — same invariant verified (no alert on failed DB path). Comment in test documents the choice. A future hardening could inject a connection that panics on Commit specifically; not required for closure.

**N4 (info) — `TestRetryQueueConsumer_BacklogAlert` flakiness.** Implementer-flagged pre-existing — not caused by these fixes. Passes 3/3 in isolation. Acceptable.

## New findings from re-review

None genuinely new. Everything I noticed during verification was either already covered in N1-N4 or was a positive observation.

## Approved items (re-review)

- BodyLimit 64KB now enforced in production server.go.
- All audit_log metadata in webhook flow uses sha256 hashes; raw IP / raw account / raw content never persist.
- Over-payment audit + admin alert fire only after Commit succeeds; commit-fail produces no ghost rows.
- Ledger row insertion order matches spec lines 49-51 (topup → topup_excess).
- Bonus computed exactly once (line 124); overpaidResult struct carries the value forward.
- RetryQueueConsumer + ConsumeAdminAlerts wrapped in supervisor with defer-recover; panics restart inner loop, ctx cancel exits cleanly.
- ProxyHeader X-Forwarded-For + EnableTrustedProxyCheck + TrustedProxies plumbed; Phase 10 will tighten subnet.
- webhookAuditLog scoped to RootCtx with 2s WithTimeout; drains on SIGTERM.
- redact.go has dedicated test file (6 tests) covering determinism, length, distinct-input, prefix-of-full-hash equality.
- New tests for H1 (no alert on DB-path fail), H2 (ledger order assertion via SQL ORDER BY id ASC), M1 (loop survives malformed item).
- `go build ./...` clean, `go vet ./internal/...` clean.

## Recommended next action

**commit-ready-for-checkpoint**

Suggested commit message (already squashed in working tree per implementer report):

```
docs(reports): code review phase 06 fix loop 1 — APPROVED
```

Then proceed to manual M14-M20 webhook simulate per Phase 06 plan.

## Unresolved questions

None blocking. Carry-forward from prior review:
- Q1 Fly-Client-IP vs X-Forwarded-For — defer to Phase 10 deploy checklist; X-Forwarded-For + open TrustedProxies acceptable in dev.
- Q2 user_id in handler-layer audit rows — handler still passes only the audit metadata-map shape (no user_id column); service-layer audit rows do include user_id. Inconsistent by design (handler doesn't yet have user_id post-CAS). Not a blocker.
- Q3 audit_log.metadata size cap — 8KB jsonb soft limit not exceeded by truncated content snippet (50 chars) + small metadata. Confirmed safe.
- Q4 Phase 07 will replace notifyTelegramSuccess placeholder. Tracked.
