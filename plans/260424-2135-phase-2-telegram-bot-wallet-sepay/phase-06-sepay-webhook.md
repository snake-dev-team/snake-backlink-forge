# Phase 06 — SePay Webhook (`POST /webhooks/sepay`)

## Context Links
- Spec: `docs/MASTER_PROMPT.md` §4.2 webhook endpoint, §5.4 webhook flow
- Research: `./research/research-01-library-and-concurrency-decisions.md` §3 (isolation), §5 (Apikey auth)
- SePay docs fetched 2026-04-24 — see research memo
- Dependencies: phase 04 (WalletService), phase 05 (transactions)

## Overview
- **Priority:** P1 (money flow — the single highest-risk endpoint in Phase 2)
- **Status:** pending
- **Description:** Handle SePay bank webhook. Verify `Apikey` auth. Atomic idempotent credit grant. Telegram notify user.

## Key Insights
- **CAS-gated UPDATE is the idempotency boundary.** Second concurrent/retry webhook with same `provider_ref` sees `RowsAffected=0` → returns success no-op. Existing stored proc needs no change.
- **SePay retries up to 7x on non-2xx.** We return `200 {success:false}` on auth fail (prevents retry storm).
- `Authorization: Apikey <token>` (not Bearer) — confirmed from SePay docs.
- Content parse: regex `SBF\s+TOPUP\s+([A-F0-9]{8})` — if no match → log `sepay_unmatched_transfer` to audit_log, return 200 success. Reason: user might send bank transfer without order code (manual top-up request).
- Notify Telegram AFTER commit (goroutine, non-blocking) — webhook latency budget < 500ms p95.

## Requirements
### Functional
- `POST /webhooks/sepay` — accept SePay JSON payload.
- Verify `Authorization: Apikey <token>` via `subtle.ConstantTimeCompare`. On mismatch → return `200 {"success":false,"reason":"invalid_signature"}`, audit_log event=`sepay_auth_fail` with ip_hash + payload hash.
- Parse payload; require `transferType="in"` (ignore outgoing — return 200 noop).
- Extract order code from `content` field via regex.
- If no match → audit `sepay_unmatched_transfer`, return 200 success.
- If match:
  1. BEGIN TXN (pgx.ReadCommitted)
  2. `UPDATE transactions SET status='paid', paid_at=NOW(), metadata = metadata || $payload::jsonb, updated_at=NOW() WHERE provider_ref=$1 AND status='pending' RETURNING id, user_id, premium_granted, standard_granted, amount_vnd`
  3. If `RowsAffected = 0` → already processed OR unknown ref. Check separately: `SELECT status FROM transactions WHERE provider_ref=$1`. If status='paid' → already processed (return 200 success, no log noise). If NULL → unknown ref (audit, return 200 success).
  4. Validate `transferAmount >= amount_vnd` (row snapshot, not registry). If `transferAmount < amount_vnd` → set status='manual_review' + metadata.underpaid → return 200 success (don't credit).
  5. `wallet.Grant(tx, premium, ...)` if premium_granted > 0
  6. `wallet.Grant(tx, standard, ...)` if standard_granted > 0
  7. `wallet.AddVNDSpent(tx, userID, amount_vnd)`
  8. COMMIT
- Post-commit: async goroutine → send Telegram message to user_id, audit log event=`topup_success`.

### Non-functional
- Handler latency < 500ms p95 (webhook client will retry on timeout — bad UX for us).
- Authentication check runs BEFORE body parse (cheap check first).
- Body size limit 64KB.
- Full raw payload stored in `transactions.metadata` for forensics.

## Architecture

### Handler flow
```go
// POST /webhooks/sepay
func handleSePayWebhook(c *fiber.Ctx) error {
    // 1. Auth
    auth := c.Get("Authorization")
    if !strings.HasPrefix(auth, "Apikey ") {
        return ack200(c, false, "invalid_signature")
    }
    token := strings.TrimPrefix(auth, "Apikey ")
    if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.SepayWebhookToken)) != 1 {
        auditFail(c, "sepay_auth_fail")
        return ack200(c, false, "invalid_signature")
    }

    // 2. Parse
    var p sepay.Payload
    if err := c.BodyParser(&p); err != nil {
        return ack200(c, false, "invalid_payload")
    }
    if p.TransferType != "in" { return ack200(c, true, "ignored_outgoing") }

    // 3. Extract order code
    match := orderCodeRe.FindStringSubmatch(p.Content)
    if match == nil {
        audit(c, "sepay_unmatched_transfer", map[string]any{"content": p.Content, "amount": p.TransferAmount})
        return ack200(c, true, "unmatched")
    }
    orderCode := match[1]

    // 4. CAS + grant
    result, err := webhookSvc.ProcessPaidTransaction(c.Context(), orderCode, p)
    if err != nil {
        log.Error("sepay_process_err", zap.Error(err), zap.String("order_code", orderCode))
        return c.Status(500).JSON(fiber.Map{"success": false}) // let SePay retry on unexpected server error
    }
    if result.AlreadyProcessed {
        return ack200(c, true, "idempotent_replay")
    }
    if result.Underpaid {
        return ack200(c, true, "underpaid_flagged")
    }

    // 5. Notify (non-blocking)
    go notifyTelegramSuccess(context.Background(), result.UserID, result.Premium, result.Standard)
    return ack200(c, true, "")
}
```

### WebhookService (`service/webhook_service.go`)
```go
type ProcessResult struct {
    AlreadyProcessed bool
    Underpaid        bool
    UserID           uuid.UUID
    Premium, Standard int
}

func (s *WebhookService) ProcessPaidTransaction(ctx, orderCode string, p sepay.Payload) (ProcessResult, error) {
    tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
    if err != nil { return zero, err }
    defer tx.Rollback(ctx)

    var txRow struct {
        ID uuid.UUID; UserID uuid.UUID; Premium int; Standard int; AmountVND int64; Status string
    }
    payloadJSON, _ := json.Marshal(map[string]any{"sepay_raw": p})
    err = tx.QueryRow(ctx, `
        UPDATE transactions
        SET status='paid', paid_at=NOW(), metadata = metadata || $2::jsonb, updated_at=NOW()
        WHERE provider_ref=$1 AND status='pending'
        RETURNING id, user_id, premium_granted, standard_granted, amount_vnd, status`,
        orderCode, payloadJSON,
    ).Scan(&txRow.ID, &txRow.UserID, &txRow.Premium, &txRow.Standard, &txRow.AmountVND, &txRow.Status)

    if errors.Is(err, pgx.ErrNoRows) {
        // second lookup: was it already paid?
        var st string
        s.pool.QueryRow(ctx, `SELECT status FROM transactions WHERE provider_ref=$1`, orderCode).Scan(&st)
        if st == "paid" { return ProcessResult{AlreadyProcessed: true}, nil }
        // unknown ref — return "unmatched" semantics via audit
        return ProcessResult{AlreadyProcessed: true}, nil // caller logs as idempotent
    }
    if err != nil { return zero, err }

    // Underpayment guard
    if p.TransferAmount < txRow.AmountVND {
        _, _ = tx.Exec(ctx, `
            UPDATE transactions SET status='manual_review',
            metadata = metadata || jsonb_build_object('underpaid_diff', $2::bigint)
            WHERE id=$1`, txRow.ID, txRow.AmountVND - p.TransferAmount)
        if err := tx.Commit(ctx); err != nil { return zero, err }
        return ProcessResult{Underpaid: true, UserID: txRow.UserID}, nil
    }

    // Grant credits
    if txRow.Premium > 0 {
        if _, err := s.wallet.Grant(ctx, tx, wallet.GrantInput{UserID: txRow.UserID, Pool: "premium", Amount: txRow.Premium, EventType: "topup", RefType: "transaction", RefID: txRow.ID}); err != nil { return zero, err }
    }
    if txRow.Standard > 0 {
        if _, err := s.wallet.Grant(ctx, tx, wallet.GrantInput{UserID: txRow.UserID, Pool: "standard", Amount: txRow.Standard, EventType: "topup", RefType: "transaction", RefID: txRow.ID}); err != nil { return zero, err }
    }
    if _, err := tx.Exec(ctx, `UPDATE wallets SET total_vnd_spent = total_vnd_spent + $2 WHERE user_id=$1`, txRow.UserID, txRow.AmountVND); err != nil { return zero, err }

    if err := tx.Commit(ctx); err != nil { return zero, err }
    return ProcessResult{UserID: txRow.UserID, Premium: txRow.Premium, Standard: txRow.Standard}, nil
}
```

### Payload struct (`integration/sepay/payload.go`)
```go
type Payload struct {
    ID              int64  `json:"id"`
    Gateway         string `json:"gateway"`
    TransactionDate string `json:"transactionDate"`
    AccountNumber   string `json:"accountNumber"`
    Code            string `json:"code"`
    Content         string `json:"content"`
    TransferType    string `json:"transferType"`
    TransferAmount  int64  `json:"transferAmount"`
    ReferenceCode   string `json:"referenceCode"`
    Description     string `json:"description"`
    Accumulated     int64  `json:"accumulated"`
    SubAccount      string `json:"subAccount"`
}

var orderCodeRe = regexp.MustCompile(`(?i)SBF\s+TOPUP\s+([A-F0-9]{8})`)
```

## Related Code Files
### Create
- `services/api/internal/api/handlers/webhook.go`
- `services/api/internal/api/handlers/webhook_test.go`
- `services/api/internal/integration/sepay/payload.go`
- `services/api/internal/integration/sepay/verify.go` — Apikey compare helper
- `services/api/internal/service/webhook_service.go`
- `services/api/internal/service/webhook_service_test.go`

### Modify
- `services/api/internal/api/router.go` — add `/webhooks/sepay` route + `BodyLimit(64*1024)`

## Implementation Steps
1. Write `integration/sepay/payload.go` (struct + regex).
2. Write `integration/sepay/verify.go` (`VerifyApikey(auth, expected string) bool`).
3. Write `service/webhook_service.go` — CAS UPDATE + grant pattern.
4. Write `api/handlers/webhook.go` — thin controller, defers to service.
5. Register route with `app.Post("/webhooks/sepay", handlers.SePayWebhook(...))` and `BodyLimit(64*1024)` middleware.
6. Write unit test `verify_test.go` — known apikey matches; wrong returns false; empty returns false; `Bearer foo` prefix returns false.
7. Write integration test `webhook_service_test.go`:
   - Setup: user, pending transaction with provider_ref='ABC12345', amount=329000, standard=200.
   - Call ProcessPaidTransaction with matching payload → wallet.standard_credits=200, tx.status=paid, ledger row inserted.
   - Call again with same order code → AlreadyProcessed=true, wallet still 200 (not 400).
   - 10x concurrent goroutines calling ProcessPaidTransaction with same order → exactly 1 row credited.
   - Underpayment (transferAmount=100000 < 329000) → status=manual_review, wallet still 0.
8. Write handler test `webhook_test.go`:
   - Apikey correct → 200 success.
   - Apikey wrong → 200 success=false (not 401).
   - No Authorization → 200 success=false.
   - Valid auth + unparseable body → 200 success=false reason=invalid_payload.
   - Valid + `transferType=out` → 200 success ignored.
   - Valid + content no match → 200 success + audit row `sepay_unmatched_transfer`.

## Todo List
- [ ] Implement `sepay.Payload` struct
- [ ] Implement `sepay.VerifyApikey` with subtle.ConstantTimeCompare
- [ ] Implement `WebhookService.ProcessPaidTransaction`
- [ ] Implement `/webhooks/sepay` handler
- [ ] Register route in `api/router.go`
- [ ] Unit test verify
- [ ] Integration test happy path
- [ ] Integration test idempotent replay
- [ ] RACE test: 10x concurrent same order code → 1 credit
- [ ] Integration test underpayment
- [ ] Integration test unmatched content
- [ ] Manual test: curl POST with real-shape payload

## Success Criteria
- 10x concurrent `POST /webhooks/sepay` with same `referenceCode` → wallet incremented exactly once
- Auth fail → 200 success=false (no retry storm)
- Underpayment → status=manual_review, no credit granted
- Handler p95 < 500ms in integration test
- `go test -race ./internal/api/handlers/... ./internal/service/webhook*` green

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Attacker replays captured payload with valid signature | High (we can't prevent) | High | CAS-gate on status='pending' → even with valid sig, 2nd call is no-op. Audit log shows replay attempts |
| SePay rotates API key → all webhooks 200 success=false silently | Med | Critical | Alert on 10+ consecutive `sepay_auth_fail` in 5min window (Phase 08 admin stats watches this) |
| Payload schema drift — new field breaks unmarshal | Low | High | Use `json.Decoder.DisallowUnknownFields = FALSE` (default); extra fields ignored. Raw JSON also stored in metadata |
| `transferAmount` is int64 JSON but SePay sends string | Low | High | Payload test with real SePay doc example payload |
| Network partition between grant and notify goroutine loses the Telegram message | Med | Low | Notify is best-effort; user can check `/balance` or `/history`; audit log records the success |
| Forbid `+goose` SQL in webhook handlers — keep logic in Go | Low | Low | Sanity: all SQL in sqlc queries or inline in service; no migrations triggered by webhook |
| Underpaid + duplicate retry → 1st call manual_review, 2nd call AlreadyProcessed | Low | Low | ManualReview status ≠ pending → 2nd call CAS fails → AlreadyProcessed path taken → no double-handle |

## Security Considerations
- Body size limit 64KB.
- Content-Type not enforced (SePay may send application/json or text/json; BodyParser handles both).
- IP allowlist (SePay publishes webhook source IPs) — DEFERRED to Phase 10 deploy (requires knowing prod IP allowlist).
- `SEPAY_WEBHOOK_TOKEN` marked required in config — server refuses to boot if empty in production env.
- Audit log every branch: auth fail, unmatched, underpaid, success, already_processed. Use `audit_log.event` values `sepay_auth_fail | sepay_unmatched_transfer | sepay_underpaid | sepay_success | sepay_replay`.
- `ip_hash = sha256(c.IP())[:16]` — never raw IP.

## Next Steps
- Phase 07 `/history` shows top-up success rows.
- Phase 08 admin `/admin stats` surfaces `manual_review` queue.
- Phase 10 load-test + chaos (same payload replay).
