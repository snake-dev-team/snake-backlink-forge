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
- **[Q2] CAS widens to `status IN ('pending','cancelled')`** — late payment for cancelled order transitions to `recovered_by_late_payment` and grants full credits.
- **SePay retries up to 7x on non-2xx.** We return `200 {success:false}` on auth fail (prevents retry storm).
- `Authorization: Apikey <token>` (not Bearer) — confirmed from SePay docs.
- **[F2] Content parse: regex `(?i)SBF\s+TOPUP\s+([A-F0-9]{12})`** — 12 hex chars (case-insensitive match), then `strings.ToUpper(match[1])` before DB query (DB stores uppercase). Reject partial-match with length != 12.
- If no regex match → log `sepay_unmatched_transfer` to audit_log, return 200 success. Reason: user might send bank transfer without order code (manual top-up request).
- **[Q3] Hard-bind single merchant bank account.** Webhook payload must satisfy `p.AccountNumber == env.SEPAY_BANK_ACCOUNT` AND `p.Gateway == env.SEPAY_BANK_CODE` — otherwise audit + 200 success=false.
- **[Q4] Rate limit 20 req/sec/IP** via Redis INCR fixed-window `webhook_rl:<ip>` TTL 1s. Exceed → 429.
- **[F3] Error classification** — auth/business → 200 success=false (no retry); lock contention → 200 queued to Redis `sepay_retry_queue`; hard infra (pool exhausted) → 503; panic → 500 + admin alert.
- **[F1] Over-payment auto-credit** — if `transferAmount > amount_vnd`, grant base credits + bonus credits at package's per-credit rate (floor). `excess >= 10k` → alert admin via `adminAlertCh`; `excess >= 50k` → additionally set `metadata.manual_review=true` (credits still granted).
- **[Q5] Admin alerts via buffered channel** `adminAlertCh chan AdminAlert` (cap=100) in `main.go`; non-blocking send; consumer goroutine DMs admin.
- **[H6] Notify goroutine context** scoped to server root ctx with 10s timeout, not `context.Background()`.
- Notify Telegram AFTER commit (goroutine, non-blocking) — webhook latency budget < 500ms p95.

## Requirements
### Functional
- `POST /webhooks/sepay` — accept SePay JSON payload.
- **[Q4] Rate limit 20 req/sec/IP** — Fiber middleware on the route: Redis `INCR webhook_rl:<ip>` then `EXPIRE 1 NX`; if count > 20 → `return 429 Too Many Requests`. Runs BEFORE auth check (cheapest gate).
- Verify `Authorization: Apikey <token>` via `subtle.ConstantTimeCompare`. On mismatch → return `200 {"success":false,"reason":"invalid_signature"}`, audit_log event=`sepay_auth_fail` with ip_hash + payload hash.
- Parse payload; require `transferType="in"` (ignore outgoing — return 200 noop).
- **[Q3] Account match:** if `p.AccountNumber != cfg.SepayBankAccount` → audit `sepay_account_mismatch`, return 200 success=false.
- **[Q3] Gateway match:** if `p.Gateway != cfg.SepayBankCode` → audit `sepay_gateway_mismatch`, return 200 success=false.
- Extract order code from `content` field via regex `(?i)SBF\s+TOPUP\s+([A-F0-9]{12})`.
- **[F2]** `orderCode := strings.ToUpper(match[1])` — normalize to uppercase before DB query.
- If no match → audit `sepay_unmatched_transfer`, return 200 success.
- If match:
  1. BEGIN TXN (pgx.ReadCommitted)
  2. **[Q2]** `UPDATE transactions SET status = CASE WHEN status='pending' THEN 'paid' ELSE 'recovered_by_late_payment' END, paid_at=NOW(), metadata = metadata || $payload::jsonb, updated_at=NOW() WHERE provider_ref=$1 AND status IN ('pending','cancelled') RETURNING id, user_id, premium_granted, standard_granted, amount_vnd, status AS post_status, (status='cancelled') AS was_cancelled`
     (Alternative cleaner pattern: capture `was_cancelled` via a subselect; see service pseudocode below.)
  3. If `RowsAffected = 0` → already processed OR unknown ref. Check separately: `SELECT status FROM transactions WHERE provider_ref=$1`. If status IN ('paid','recovered_by_late_payment') → already processed (audit `sepay_replay`, return 200 success). Else → unknown ref (audit `sepay_unknown_order`, return 200 success).
  4. **[F1] 3-way amount branch:**
     - `transferAmount < amount_vnd` → set status='manual_review' + metadata.underpaid_diff → audit `sepay_underpaid` → return 200 success (don't credit).
     - `transferAmount == amount_vnd` → normal paid flow.
     - `transferAmount > amount_vnd` → compute `bonus_credits = excess_vnd / package_price_per_credit` (floor); base credits from row + bonus grant via second `grant_credits(... 'topup_excess' ...)`; if `excess >= 10_000` → non-blocking send `adminAlertCh <- AdminAlert{Kind: "overpaid", ...}`; if `excess >= 50_000` → also set `metadata.manual_review=true` (credits still granted); audit `sepay_overpaid` with delta.
  5. `wallet.Grant(tx, premium, ...)` if premium_granted > 0 (event_type='topup')
  6. `wallet.Grant(tx, standard, ...)` if standard_granted > 0 (event_type='topup')
  7. **[F1]** If over-paid: `wallet.Grant(tx, <package_pool>, bonus_credits, 'topup_excess', 'transaction', txID)` for the package's primary pool
  8. `wallet.AddVNDSpent(tx, userID, amount_vnd)`
  9. COMMIT
- Post-commit: async goroutine (ctx scoped to server root ctx + 10s timeout — **[H6]**) → send Telegram message to user_id. If was_cancelled → use `topup_recovered_late_payment` template; if overpaid bonus granted → `topup_overpaid_success` template; else `topup_paid_success`. Audit log event=`sepay_success`.

### Non-functional
- Handler latency < 500ms p95 (webhook client will retry on timeout — bad UX for us).
- Authentication check runs BEFORE body parse (cheap check first).
- Body size limit 64KB.
- Full raw payload stored in `transactions.metadata` for forensics.

## Architecture

### Handler flow (with [Q3] [Q4] [F2] [F3] [H6])
```go
// POST /webhooks/sepay — registered with rateLimitMiddleware(rdb, 20) first
func handleSePayWebhook(c *fiber.Ctx) error {
    // 1. [Q4] Rate limit enforced by Fiber middleware BEFORE this handler.
    //        Middleware: Redis INCR webhook_rl:<ip> with EXPIRE 1 NX; if >20 → return 429.

    // 2. Auth
    auth := c.Get("Authorization")
    if !strings.HasPrefix(auth, "Apikey ") {
        return ack200(c, false, "invalid_signature")
    }
    token := strings.TrimPrefix(auth, "Apikey ")
    if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.SepayWebhookToken)) != 1 {
        auditFail(c, "sepay_auth_fail")
        return ack200(c, false, "invalid_signature")
    }

    // 3. Parse
    var p sepay.Payload
    if err := c.BodyParser(&p); err != nil {
        return ack200(c, false, "invalid_payload")
    }
    if p.TransferType != "in" { return ack200(c, true, "ignored_outgoing") }

    // 4. [Q3] Account + gateway match — reject early if this webhook is for a different merchant/bank binding.
    if p.AccountNumber != deps.Cfg.SepayBankAccount {
        audit(c, "sepay_account_mismatch", map[string]any{"received_acc_sha256_prefix": sha256Hex(p.AccountNumber)[:12]})
        return ack200(c, false, "account_mismatch")
    }
    if p.Gateway != deps.Cfg.SepayBankCode {
        audit(c, "sepay_gateway_mismatch", map[string]any{"received_gateway": p.Gateway})
        return ack200(c, false, "gateway_mismatch")
    }

    // 5. [F2] Extract order code + uppercase normalize
    match := orderCodeRe.FindStringSubmatch(p.Content)
    if match == nil {
        audit(c, "sepay_unmatched_transfer", map[string]any{"content": p.Content, "amount": p.TransferAmount})
        return ack200(c, true, "unmatched")
    }
    orderCode := strings.ToUpper(match[1]) // DB stores uppercase

    // 6. CAS + grant with [F3] error classifier
    result, err := webhookSvc.ProcessPaidTransaction(c.Context(), orderCode, p)
    if err != nil {
        return classifyWebhookError(c, deps, err, orderCode, p)
    }
    switch {
    case result.Underpaid:
        return ack200(c, true, "underpaid_flagged")
    case result.AlreadyProcessed:
        return ack200(c, true, "idempotent_replay")
    case result.UnknownOrder:
        return ack200(c, true, "unknown_order")
    }

    // 7. [H6] Notify (non-blocking) with scoped context
    notifyCtx, notifyCancel := context.WithTimeout(deps.RootCtx, 10*time.Second)
    go func() {
        defer notifyCancel()
        notifyTelegramSuccess(notifyCtx, deps, result)
    }()
    return ack200(c, true, "")
}
```

### [F3] Error classifier
```go
// classifyWebhookError maps Go errors → HTTP response per F3 policy.
func classifyWebhookError(c *fiber.Ctx, deps *Deps, err error, orderCode string, p sepay.Payload) error {
    var pgErr *pgconn.PgError
    switch {
    // Lock contention / transient concurrency — queue for async retry, return 200 to prevent SePay retry storm
    case errors.As(err, &pgErr) && (pgErr.Code == pgerrcode.DeadlockDetected || pgErr.Code == pgerrcode.SerializationFailure),
         errors.Is(err, context.DeadlineExceeded):
        env := RetryEnvelope{Payload: p, Attempts: 0, OriginalTS: time.Now().Unix()}
        data, _ := json.Marshal(env)
        _ = deps.Rdb.LPush(context.Background(), "sepay_retry_queue", data).Err()
        _ = deps.Rdb.LTrim(context.Background(), "sepay_retry_queue", 0, 499).Err()
        deps.Log.Warn("sepay webhook queued for retry (lock contention)",
            zap.String("order_code", orderCode), zap.Error(err))
        return ack200(c, true, "queued_for_retry")

    // Hard infra — pool exhausted, Redis down, etc. Let SePay retry (short-lived).
    case errors.Is(err, pgx.ErrAcquireTimeout):
        audit(c, "sepay_infra_error", map[string]any{"err": err.Error()})
        return c.Status(503).JSON(fiber.Map{"success": false, "reason": "service_unavailable"})

    // Unknown/panic — audit + alert admin, return 500 (rare retry expected)
    default:
        deps.Log.Error("sepay webhook unknown error", zap.Error(err), zap.String("order_code", orderCode))
        audit(c, "sepay_internal_error", map[string]any{"err": err.Error()})
        sendAdminAlertNonBlocking(deps.AdminAlertCh, AdminAlert{Kind: "webhook_internal_error", Err: err.Error(), OrderCode: orderCode})
        return c.Status(500).JSON(fiber.Map{"success": false})
    }
}
```

### Error-class response matrix

| Error class | Example | Response | Rationale |
|---|---|---|---|
| Auth fail | Apikey mismatch | 200 success=false | No retry (SePay spec), audit `sepay_auth_fail` |
| Rate limit | 20/s/IP exceeded | 429 | SePay retries safely on 429 |
| Business logic | account mismatch, gateway mismatch, duplicate paid, unknown order | 200 success=false | No retry, audit |
| Lock contention | `40P01` DeadlockDetected, `40001` SerializationFailure, `context.DeadlineExceeded` | 200 queued_for_retry (enqueue Redis `sepay_retry_queue`; consumer retries up to 3x) | Prevents SePay retry storm |
| Hard infra | `pgx.ErrAcquireTimeout`, pool exhausted, Redis down | 503 | SePay retries OK, short-lived |
| Unknown/panic | catch-all | 500 + audit `sepay_internal_error` + admin alert | Rare |

### WebhookService (`service/webhook_service.go`) — [F1] 3-way + [Q2] cancel recovery
```go
type ProcessResult struct {
    AlreadyProcessed  bool
    Underpaid         bool
    UnknownOrder      bool
    Overpaid          bool
    BonusCredits      int
    WasCancelled      bool   // true when recovered_by_late_payment
    UserID            uuid.UUID
    PackageCode       string
    Premium, Standard int    // base credits granted
}

// Per-credit rate (VND) for bonus calculation — derived from snapshot row.
// For mixed combos, the excess is credited to the PREMIUM pool at the premium rate
// (combos always include premium). For single-pool packages, excess credits the same pool.
func perCreditRate(premiumCr, standardCr int, amount int64) (pool string, rate int64) {
    total := int64(premiumCr + standardCr)
    if total == 0 { return "standard", 0 }
    if premiumCr > 0 {
        return "premium", amount / int64(premiumCr+standardCr) // simplified; combos priced by weighted avg ≈ amount/credits
    }
    return "standard", amount / total
}

func (s *WebhookService) ProcessPaidTransaction(ctx context.Context, orderCode string, p sepay.Payload) (ProcessResult, error) {
    tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
    if err != nil { return ProcessResult{}, err }
    defer tx.Rollback(ctx)

    var txRow struct {
        ID uuid.UUID; UserID uuid.UUID; Premium int; Standard int; AmountVND int64
        PkgCode string; PreStatus string
    }
    payloadJSON, _ := json.Marshal(map[string]any{"sepay_raw": p})

    // [Q2] CAS widens to ('pending','cancelled'); capture pre-state for branch
    err = tx.QueryRow(ctx, `
        WITH target AS (
          SELECT id, status AS pre_status FROM transactions
          WHERE provider_ref = $1 AND status IN ('pending','cancelled')
          FOR UPDATE
        )
        UPDATE transactions t
        SET status = CASE target.pre_status WHEN 'pending' THEN 'paid'::transaction_status ELSE 'recovered_by_late_payment'::transaction_status END,
            paid_at = NOW(),
            metadata = t.metadata || $2::jsonb,
            updated_at = NOW()
        FROM target
        WHERE t.id = target.id
        RETURNING t.id, t.user_id, t.premium_granted, t.standard_granted, t.amount_vnd, t.package_code, target.pre_status`,
        orderCode, payloadJSON,
    ).Scan(&txRow.ID, &txRow.UserID, &txRow.Premium, &txRow.Standard, &txRow.AmountVND, &txRow.PkgCode, &txRow.PreStatus)

    if errors.Is(err, pgx.ErrNoRows) {
        // Out-of-band lookup — was it already paid/recovered? vs unknown ref?
        var st string
        qErr := s.pool.QueryRow(ctx, `SELECT status FROM transactions WHERE provider_ref=$1 LIMIT 1`, orderCode).Scan(&st)
        if errors.Is(qErr, pgx.ErrNoRows) {
            s.audit.Log(ctx, AuditInput{Event: "sepay_unknown_order", Metadata: map[string]any{"order_code": orderCode}})
            return ProcessResult{UnknownOrder: true}, nil
        }
        if st == "paid" || st == "recovered_by_late_payment" {
            s.audit.Log(ctx, AuditInput{Event: "sepay_replay", Metadata: map[string]any{"order_code": orderCode, "status": st}})
            return ProcessResult{AlreadyProcessed: true}, nil
        }
        // status='manual_review' or 'failed' or 'refunded' — treat as already-handled
        return ProcessResult{AlreadyProcessed: true}, nil
    }
    if err != nil { return ProcessResult{}, err }

    wasCancelled := txRow.PreStatus == "cancelled"

    // [F1] 3-way amount branch — hoist `bonus` to outer scope so return block can use it
    diff := p.TransferAmount - txRow.AmountVND
    bonus := 0
    switch {
    case diff < 0:
        // underpaid
        _, _ = tx.Exec(ctx, `
            UPDATE transactions SET status='manual_review',
              metadata = metadata || jsonb_build_object('underpaid_diff', $2::bigint)
            WHERE id=$1`, txRow.ID, -diff)
        if cErr := tx.Commit(ctx); cErr != nil { return ProcessResult{}, cErr }
        s.audit.Log(ctx, AuditInput{Event: "sepay_underpaid", UserID: &txRow.UserID, Metadata: map[string]any{"diff": -diff, "order_code": orderCode}})
        return ProcessResult{Underpaid: true, UserID: txRow.UserID, WasCancelled: wasCancelled}, nil

    case diff == 0:
        // exact match — normal paid flow (fall through to base grants below)

    case diff > 0:
        // overpaid: base grants + bonus grant using package per-credit rate
        pool, rate := perCreditRate(txRow.Premium, txRow.Standard, txRow.AmountVND)
        if rate > 0 {
            bonus = int(diff / rate) // floor
        }
        if diff >= 50_000 {
            _, _ = tx.Exec(ctx,
                `UPDATE transactions SET metadata = metadata || jsonb_build_object('manual_review', true, 'overpaid_diff', $2::bigint) WHERE id=$1`,
                txRow.ID, diff)
        }
        if bonus > 0 {
            if _, gErr := s.wallet.Grant(ctx, tx, wallet.GrantInput{
                UserID: txRow.UserID, Pool: pool, Amount: bonus,
                EventType: "topup_excess", RefType: "transaction", RefID: txRow.ID,
            }); gErr != nil {
                return ProcessResult{}, gErr
            }
        }
        // alert admin post-commit (defer setting flag; actual send below after Commit succeeds)
        s.audit.Log(ctx, AuditInput{Event: "sepay_overpaid", UserID: &txRow.UserID, Metadata: map[string]any{"diff": diff, "bonus_credits": bonus, "order_code": orderCode}})
        if diff >= 10_000 {
            sendAdminAlertNonBlocking(s.adminAlertCh, AdminAlert{
                Kind: "overpaid", OrderCode: orderCode, UserID: txRow.UserID.String(),
                ExcessVND: diff, BonusCredits: bonus,
            })
        }
        // defer to base-grant block for premium/standard
        // mark Overpaid for notify template selection
        // (we overwrite ProcessResult fields after COMMIT below)
    }

    // Base grants (premium / standard)
    if txRow.Premium > 0 {
        if _, err := s.wallet.Grant(ctx, tx, wallet.GrantInput{UserID: txRow.UserID, Pool: "premium", Amount: txRow.Premium, EventType: "topup", RefType: "transaction", RefID: txRow.ID}); err != nil { return ProcessResult{}, err }
    }
    if txRow.Standard > 0 {
        if _, err := s.wallet.Grant(ctx, tx, wallet.GrantInput{UserID: txRow.UserID, Pool: "standard", Amount: txRow.Standard, EventType: "topup", RefType: "transaction", RefID: txRow.ID}); err != nil { return ProcessResult{}, err }
    }
    if _, err := tx.Exec(ctx, `UPDATE wallets SET total_vnd_spent = total_vnd_spent + $2 WHERE user_id=$1`, txRow.UserID, p.TransferAmount); err != nil {
        return ProcessResult{}, err
    }

    if err := tx.Commit(ctx); err != nil { return ProcessResult{}, err }

    return ProcessResult{
        UserID: txRow.UserID, PackageCode: txRow.PkgCode,
        Premium: txRow.Premium, Standard: txRow.Standard,
        WasCancelled: wasCancelled,
        Overpaid: diff > 0, BonusCredits: bonus,
    }, nil
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

// [F2] 12 hex chars, case-insensitive capture; callers MUST ToUpper(match[1]) before DB query
var orderCodeRe = regexp.MustCompile(`(?i)SBF\s+TOPUP\s+([A-F0-9]{12})`)
```

### [round-3] Retry queue + consumer (`sepay_retry_queue`)
- **Producer** (from `classifyWebhookError` lock-contention branch): `LPUSH sepay_retry_queue <payload_envelope_json>` then `LTRIM sepay_retry_queue 0 499` → keep newest 500 entries (bounded). Payload envelope: `{"payload": <raw>, "attempts": 0, "original_ts": <unix>}`.
- **Consumer goroutine** (spawned in `cmd/api/main.go`): `BRPOP sepay_retry_queue 0` (blocking, no timeout) → decode envelope → backoff sleep `min(60s, 2^attempts * 1s)` if `attempts > 0` → replay `ProcessPaidTransaction` logic using the stored raw payload.
- **Max retries per message:** 3. On 4th attempt failure → `LPUSH sepay_dead_letter <envelope>` (uncapped list for now) + log + `sendAdminAlertNonBlocking(AdminAlert{Kind:"retry_dead_letter"})` (deduped via 5-min bucket map: `now.Unix()/300` — one alert per 5-min window regardless of dead-letter volume, prevents admin-DM flood during retry storm).
- **Dead-letter test sentinel:** `provider_ref="DEADLETTER_TRIGGER"` (const `DeadLetterSentinel`) — consumer short-circuits with `ErrDeadLetterSentinel` (not transient); increments attempts normally; after 3 retries lands in dead-letter. Used by `TestRetryQueueDeadLetter` without DB fault injection.
- **Backlog alert:** every tick of consumer loop, check `LLEN sepay_retry_queue > 400` (80% full) → `sendAdminAlertNonBlocking(AdminAlert{Kind:"retry_queue_backlog"})`. One alert per backlog event (deduped via in-memory flag until LLEN drops below 200).
- **Re-enqueue on transient failure:** increment envelope.attempts → `LPUSH sepay_retry_queue <envelope>` → respect the 500 cap via `LTRIM 0 499` after each LPUSH.

```go
// services/api/internal/service/retry_consumer.go (new)
type RetryEnvelope struct {
    Payload    sepay.Payload `json:"payload"`
    Attempts   int           `json:"attempts"`
    OriginalTS int64         `json:"original_ts"`
}

// DeadLetterSentinel — reserved provider_ref value used by integration tests to force
// unrecoverable business-error on every ProcessPaidTransaction call. Consumer recognizes
// it, treats all attempts as failures, exercises dead-letter flow without DB fault injection.
const DeadLetterSentinel = "DEADLETTER_TRIGGER"

func retryQueueConsumer(ctx context.Context, rdb *redis.Client, svc *WebhookService,
    alertCh chan<- AdminAlert, log *zap.Logger) {
    backlogAlertedHigh := false                // dedup flag: true after >400 alert, reset when <200
    deadLetterAlerted := make(map[int64]bool)  // 5-min bucket dedup for retry_dead_letter alerts
    for {
        // BRPOP blocks until item available or ctx cancelled
        res, err := rdb.BRPop(ctx, 0, "sepay_retry_queue").Result()
        if err != nil {
            if errors.Is(err, context.Canceled) { return }
            log.Warn("BRPOP failed", zap.Error(err)); continue
        }
        var env RetryEnvelope
        if err := json.Unmarshal([]byte(res[1]), &env); err != nil {
            log.Warn("retry envelope decode failed", zap.Error(err)); continue
        }
        if env.Attempts > 0 {
            backoff := time.Duration(min64(60, 1<<env.Attempts)) * time.Second
            select { case <-time.After(backoff): case <-ctx.Done(): return }
        }
        // Extract orderCode from payload.Content and replay
        match := orderCodeRe.FindStringSubmatch(env.Payload.Content)
        if match == nil { continue } // unrecoverable; drop
        orderCode := strings.ToUpper(match[1])

        // Test sentinel: force unrecoverable business error without DB fault injection
        var procErr error
        if orderCode == DeadLetterSentinel {
            procErr = ErrDeadLetterSentinel // defined in webhook_service.go, !errors.Is transient
        } else {
            _, procErr = svc.ProcessPaidTransaction(ctx, orderCode, env.Payload)
        }
        if procErr == nil { continue } // success

        env.Attempts++
        if env.Attempts >= 3 {
            data, _ := json.Marshal(env)
            _ = rdb.LPush(ctx, "sepay_dead_letter", data).Err()

            // 5-min window dedup: prevents N-per-batch flood when retry storm hits dead-letter
            bucket := time.Now().Unix() / 300
            if !deadLetterAlerted[bucket] {
                sendAdminAlertNonBlocking(alertCh, AdminAlert{
                    Kind: "retry_dead_letter", OrderCode: orderCode, Err: procErr.Error(),
                })
                deadLetterAlerted[bucket] = true
                for b := range deadLetterAlerted { // GC old buckets (keep last ~1h)
                    if b < bucket-12 { delete(deadLetterAlerted, b) }
                }
            }
            log.Error("retry exhausted → dead-letter",
                zap.String("order_code", orderCode), zap.Error(procErr))
            continue
        }
        // Re-enqueue with incremented attempts
        data, _ := json.Marshal(env)
        _ = rdb.LPush(ctx, "sepay_retry_queue", data).Err()
        _ = rdb.LTrim(ctx, "sepay_retry_queue", 0, 499).Err()

        // Backlog alert check
        llen, _ := rdb.LLen(ctx, "sepay_retry_queue").Result()
        if llen > 400 && !backlogAlertedHigh {
            sendAdminAlertNonBlocking(alertCh, AdminAlert{
                Kind: "retry_queue_backlog", Err: fmt.Sprintf("queue len=%d (>80%% of 500 cap)", llen),
            })
            backlogAlertedHigh = true
        } else if llen < 200 && backlogAlertedHigh {
            backlogAlertedHigh = false
        }
    }
}
```

Producer: inlined in `classifyWebhookError` lock-contention branch (see `### [F3] Error classifier` above) — single source of truth. Envelope shape: `RetryEnvelope{Payload, Attempts, OriginalTS}` → `LPUSH sepay_retry_queue` → `LTRIM 0 499`. No separate producer snippet to avoid drift.

### [Q5] Admin alert channel wiring (`cmd/api/main.go` — phase-01 patched)
```go
// In main.go during server init:
adminAlertCh := make(chan AdminAlert, 100)

// Single consumer goroutine — spawned once, bound to root ctx
go consumeAdminAlerts(rootCtx, adminAlertCh, bot, cfg)

// On shutdown (after app.Shutdown returns):
close(adminAlertCh)
// consumeAdminAlerts drains remaining alerts, then exits when channel closed.

// Producer pattern (in webhook handler + audit thresholds):
func sendAdminAlertNonBlocking(ch chan<- AdminAlert, alert AdminAlert) {
    select {
    case ch <- alert:
    default:
        // Drop + warn; prevents handler blocking under flood
        zap.L().Warn("admin alert channel full, dropping", zap.Any("alert", alert))
    }
}
```

`AdminAlert` struct:
```go
type AdminAlert struct {
    Kind         string    // "overpaid" | "auth_fail_burst" | "webhook_internal_error" | ...
    OrderCode    string
    UserID       string
    ExcessVND    int64
    BonusCredits int
    Err          string
    At           time.Time
}
```

`consumeAdminAlerts` template: on each alert, `bot.Send(tgbotapi.NewMessage(adminID, formatAlert(alert)))` for each configured `cfg.AdminTelegramIDs`. Swallow send errors; log only.

## Related Code Files
### Create
- `services/api/internal/api/handlers/webhook.go`
- `services/api/internal/api/handlers/webhook_test.go`
- `services/api/internal/api/middleware/rate_limit_webhook.go` — **[Q4]** Fiber middleware: Redis INCR + EXPIRE 1 NX, 20 req/sec/IP
- `services/api/internal/integration/sepay/payload.go`
- `services/api/internal/integration/sepay/verify.go` — Apikey compare helper
- `services/api/internal/service/webhook_service.go`
- `services/api/internal/service/webhook_service_test.go`
- `services/api/internal/bot/admin_alerts.go` — **[Q5]** `AdminAlert` struct + `consumeAdminAlerts` consumer goroutine + `sendAdminAlertNonBlocking` helper
- `services/api/internal/service/retry_consumer.go` — **[round-3]** `retryQueueConsumer` goroutine + `RetryEnvelope` struct; BRPOP-based; max 3 retries + dead-letter + backlog alert

### Modify
- `services/api/internal/api/router.go` — add `/webhooks/sepay` route + `BodyLimit(64*1024)` + `rateLimitWebhook(20/sec/IP)` middleware
- `services/api/cmd/api/main.go` — **[Q5]** create `adminAlertCh := make(chan AdminAlert, 100)`; spawn consumer goroutine; wire into Deps; close on shutdown. **[round-3]** also spawn `go retryQueueConsumer(rootCtx, rdb, webhookSvc, adminAlertCh, log)`
- `services/api/internal/config/config.go` — rename `SepayBankAcc` → `SepayBankAccount` (env `SEPAY_BANK_ACCOUNT`) — consistent w/ phase-05

## Implementation Steps
1. Write `integration/sepay/payload.go` (struct + regex — **[F2]** 12-hex `{12}`).
2. Write `integration/sepay/verify.go` (`VerifyApikey(auth, expected string) bool`).
3. **[Q4]** Write `api/middleware/rate_limit_webhook.go` — Fiber middleware: `INCR webhook_rl:<c.IP()>` then `EXPIRE 1 NX`. If `>20` → `c.SendStatus(429)`. Use Redis pipeline for atomicity.
4. **[Q5]** Write `bot/admin_alerts.go` — `AdminAlert`, `sendAdminAlertNonBlocking`, `consumeAdminAlerts(ctx, ch, bot, cfg)`. In `main.go`: create buffered channel cap=100, spawn single consumer goroutine, pass ch via Deps to webhook handler; close channel on shutdown.
5. Write `service/webhook_service.go` — **[F1]** 3-way branch (underpaid / exact / overpaid w/ bonus + threshold flags), **[Q2]** CAS widened to `('pending','cancelled')` with pre-state captured.
6. Write `api/handlers/webhook.go` — thin controller with **[Q3]** account + gateway match, **[F2]** `ToUpper` order code normalize, **[F3]** error classifier, **[H6]** notify goroutine with `WithTimeout(deps.RootCtx, 10s)`.
7. Register route: `app.Post("/webhooks/sepay", rateLimitWebhook(rdb, 20), bodyLimit(64*1024), handlers.SePayWebhook(deps))`.
7a. **[round-3]** Write `service/retry_consumer.go` — `retryQueueConsumer(ctx, rdb, webhookSvc, alertCh, log)` goroutine: BRPOP blocking; backoff `min(60s, 2^attempts * 1s)`; 3-retry dead-letter; 80%-backlog alert with LLEN<200 reset.
7b. **[round-3]** In `cmd/api/main.go`: spawn `go retryQueueConsumer(rootCtx, rdb, webhookSvc, adminAlertCh, log)` alongside `consumeAdminAlerts`.
7c. **[round-3]** Producer in `classifyWebhookError`: wrap payload in `RetryEnvelope{Payload, Attempts: 0, OriginalTS}` → `LPUSH sepay_retry_queue` → `LTRIM 0 499`.
8. Write unit test `verify_test.go` — known apikey matches; wrong returns false; empty returns false; `Bearer foo` prefix returns false.
9. Write integration test `webhook_service_test.go`:
   - Setup: user, pending transaction with provider_ref='ABCDEF012345' (12-hex), amount=329000, standard=200.
   - Call ProcessPaidTransaction with matching payload → wallet.standard_credits=200, tx.status=paid, ledger row inserted.
   - Call again with same order code → AlreadyProcessed=true, wallet still 200 (not 400).
   - 10x concurrent goroutines calling ProcessPaidTransaction with same order → exactly 1 row credited.
   - Underpayment (transferAmount=100000 < 329000) → status=manual_review, wallet still 0.
   - **[F1]** Over-payment exact (transferAmount=400000, amount=329000, package='standard_pro_200') → base 200 + bonus 43 standard granted; ledger has rows `topup` + `topup_excess`; `adminAlertCh` receives `overpaid` alert (diff=71000 ≥ 10k threshold).
   - **[F1]** Over-payment (transferAmount=380000, diff=51000 → ≥50k threshold) → base + bonus granted; `metadata.manual_review=true` set; credits still granted.
   - **[Q2]** Cancel-then-pay: create pending → CancelPendingTransaction → ProcessPaidTransaction → status='recovered_by_late_payment', wallet credited full amount.
   - **[Q3]** account_mismatch: payload.accountNumber != cfg.SepayBankAccount → 200 success=false, wallet untouched, audit `sepay_account_mismatch`.
   - **[Q3]** gateway_mismatch: payload.gateway='UnknownBank' → 200 success=false, audit `sepay_gateway_mismatch`.
   - **[F2]** Mixed-case memo "sbf topup abcdef012345" → `ToUpper` yields `ABCDEF012345`, CAS matches.
   - **[F3]** Deadlock injection (use testcontainers' multi-connection pressure + advisory lock) → 200 queued_for_retry + Redis `sepay_retry_queue` has payload; wallet untouched.
10. Write handler test `webhook_test.go`:
    - Apikey correct → 200 success.
    - Apikey wrong → 200 success=false (not 401).
    - No Authorization → 200 success=false.
    - Valid auth + unparseable body → 200 success=false reason=invalid_payload.
    - Valid + `transferType=out` → 200 success ignored.
    - Valid + content no match → 200 success + audit row `sepay_unmatched_transfer`.
    - **[Q4]** 21st request within 1s from same IP → 429.
    - **[H6]** SIGTERM during in-flight notify → goroutine exits via ctx cancel within 10s.

## Todo List
- [ ] Implement `sepay.Payload` struct + **[F2]** 12-hex regex
- [ ] Implement `sepay.VerifyApikey` with subtle.ConstantTimeCompare
- [ ] **[Q4]** Implement `rateLimitWebhook` middleware (Redis INCR + EXPIRE 1 NX, 20/s/IP → 429)
- [ ] **[Q5]** Implement `AdminAlert` + buffered channel cap=100 + `consumeAdminAlerts` consumer + `sendAdminAlertNonBlocking`
- [ ] **[Q5]** Wire `adminAlertCh` into `main.go`; close on shutdown
- [ ] Implement `WebhookService.ProcessPaidTransaction` with **[F1]** 3-way branch + **[Q2]** CAS widened
- [ ] **[Q3]** Implement account + gateway match in handler
- [ ] **[F2]** Implement `ToUpper(match[1])` normalization
- [ ] **[F3]** Implement `classifyWebhookError` (lock → 200 queued + Redis list + LTRIM 0 499; infra → 503; panic → 500 + alert)
- [ ] **[round-3]** Implement `RetryEnvelope` + `retryQueueConsumer` goroutine (BRPOP, backoff, 3-retry dead-letter, 80% backlog alert)
- [ ] **[round-3]** Spawn retry consumer in `cmd/api/main.go`
- [ ] **[H6]** Implement notify goroutine with `WithTimeout(RootCtx, 10s)` + defer cancel
- [ ] Register route in `api/router.go` with rate-limit + body-limit
- [ ] Unit test verify
- [ ] Integration test happy path
- [ ] Integration test idempotent replay
- [ ] RACE test: 10x concurrent same order code → 1 credit
- [ ] Integration test underpayment
- [ ] **[F1]** Integration test over-payment exact (standard_pro_200 + 400k → +243 std credits) + over-payment 50k flag + admin alert channel receives
- [ ] **[Q2]** Integration test cancel-then-pay recovery path (status → recovered_by_late_payment, full credits)
- [ ] **[Q3]** Integration test account_mismatch + gateway_mismatch
- [ ] **[Q4]** Integration test rate-limit 21st req in 1s → 429
- [ ] **[F3]** Integration test deadlock injection → 200 queued + retry queue populated
- [ ] Integration test unmatched content
- [ ] Manual test: curl POST with real-shape payload

## Success Criteria
- 10x concurrent `POST /webhooks/sepay` with same `referenceCode` → wallet incremented exactly once
- Auth fail → 200 success=false (no retry storm)
- Underpayment → status=manual_review, no credit granted
- **[F1] Over-payment** → base credits + bonus credits granted; ledger has `topup_excess` row; admin alert received if diff≥10k; `metadata.manual_review=true` if diff≥50k
- **[Q2] Cancel-then-pay** → status=`recovered_by_late_payment`, full credits granted, user DM'd via `topup_recovered_late_payment` template
- **[Q3]** account/gateway mismatch → 200 success=false, audit row, wallet untouched
- **[Q4]** 21st request in 1s/IP → 429
- **[F3]** Deadlock injection → 200 queued_for_retry; Redis `sepay_retry_queue` length +1; eventual retry grants credits exactly once
- Handler p95 < 500ms in integration test
- `go test -race ./internal/api/handlers/... ./internal/service/webhook*` green

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Attacker replays captured payload with valid signature | High (we can't prevent) | High | CAS-gate on status IN ('pending','cancelled') → replay on paid/recovered row is no-op. Audit log shows replay attempts |
| SePay rotates API key → all webhooks 200 success=false silently | Med | Critical | **[Q5 + round-3]** `auditFailAlertWatcher` goroutine (phase-08) polls `audit_log` every 60s; if ≥20 `sepay_auth_fail` rows in last 15min → `adminAlertCh <- AdminAlert{Kind:"auth_fail_burst"}`. Window-bucket dedup (`now.Unix()/900`) prevents duplicate alerts per bucket |
| Payload schema drift — new field breaks unmarshal | Low | High | Use `json.Decoder.DisallowUnknownFields = FALSE` (default); extra fields ignored. Raw JSON also stored in metadata |
| `transferAmount` is int64 JSON but SePay sends string | Low | High | Payload test with real SePay doc example payload (**[M6]** `testutil/sepay_payload_real.json` in phase-10). Current `int64` tag works per docs; if real payload reveals string-typed number, add custom `UnmarshalJSON` |
| Network partition between grant and notify goroutine loses the Telegram message | Med | Low | Notify is best-effort; **[H6]** ctx scoped to `RootCtx` + 10s timeout; user can self-serve `/balance` or `/history`; audit log records the success |
| Over-payment bonus calc off by rounding | Low | Med | **[F1]** explicit `int(diff/rate)` floor; integration test asserts 400000-329000=71000 → 71000/1645=43 (not 44) |
| [Q2] Cancelled + legit late payment + separate new pending for same pkg | Med | Low | User might click /buy again after cancel; new pending row uses DIFFERENT provider_ref; webhook matches by provider_ref (not user+pkg), so each gets credited independently. Audit logs differentiate |
| [Q4] Rate limit false-positive at legit burst | Low | Low | 20/s/IP is ~10x above SePay's observed max (~2/s). If legit load exceeds, raise cap via env var (`WEBHOOK_RATE_LIMIT_PER_SEC`) |
| [Q5] Admin alert channel full (100 alerts backlog) | Low | Low | Non-blocking send with drop + warn; prevents cascading block |
| Forbid `+goose` SQL in webhook handlers — keep logic in Go | Low | Low | Sanity: all SQL in sqlc queries or inline in service; no migrations triggered by webhook |
| Underpaid + duplicate retry → 1st call manual_review, 2nd call AlreadyProcessed | Low | Low | ManualReview status ∉ ('pending','cancelled') → 2nd call CAS fails → AlreadyProcessed path taken → no double-handle |
| **[round-3]** Retry consumer poisoning — permanently-failing payload loops | Low | Med | Max 3 retries per envelope; 4th attempt → `sepay_dead_letter` Redis list + `retry_dead_letter` admin alert (5-min bucket dedup). Log + manual inspection queue |
| **[round-3]** Retry queue backlog during lock storm | Low | Med | `LTRIM 0 499` caps at 500. Backlog > 400 → `retry_queue_backlog` admin alert (dedup until LLEN < 200). Old entries evicted LIFO via LTRIM |
| **[round-3]** Retry consumer goroutine crash → queue stalls | Low | High | Consumer spawned in `cmd/api/main.go` bound to rootCtx; on panic, `recover()` + restart loop. BRPOP is blocking — no CPU burn when empty |

## Security Considerations
- Body size limit 64KB.
- Content-Type not enforced (SePay may send application/json or text/json; BodyParser handles both).
- **[Q4] Rate limit 20 req/sec/IP** via Redis fixed-window — hard gate before auth.
- **IP allowlist — pre-deploy blocker for phase-10:** MUST research SePay source IPs at https://docs.sepay.vn. If docs do not publish static IPs, fall back to rate limit only. Document in phase-10 deploy checklist.
- `SEPAY_WEBHOOK_TOKEN` marked required in config — server refuses to boot if empty in production env.
- **[Q3]** `SEPAY_BANK_ACCOUNT` + `SEPAY_BANK_CODE` required; server refuses to boot if unset.
- Audit log every branch: `sepay_auth_fail | sepay_account_mismatch | sepay_gateway_mismatch | sepay_unmatched_transfer | sepay_unknown_order | sepay_underpaid | sepay_overpaid | sepay_success | sepay_replay | sepay_internal_error`.
- **[M4]** `ip_hash = sha256(c.IP())` — store full 32-char hex (not truncated to 16) — trivial storage cost, avoids birthday-collision merging distinct IPs into one rate-limit key.

### [round-3] Deploy target
- **Production:** `https://snake-backlink-api.fly.dev/webhooks/sepay` (Fly auto-assigned subdomain — no custom domain in Phase 2). Fallback: Cloudflare Tunnel (`*.trycloudflare.com`) if SePay rejects `.fly.dev`.
- **Local dev:** expose `localhost:8080/webhooks/sepay` via `ngrok http 8080` or equivalent (e.g. `cloudflared tunnel`); configure SePay sandbox webhook URL with the public tunnel URL.

## Next Steps
- Phase 07 `/history` shows top-up success rows.
- Phase 08 admin `/admin stats` surfaces `manual_review` queue.
- Phase 10 load-test + chaos (same payload replay).
