# Phase 05 — Transactions + `/buy` + `/topup` (QR)

## Context Links
- Spec: `docs/MASTER_PROMPT.md` §1.3 packages, §5.1 commands, §5.2 FSM, §5.4 SePay QR
- Research: `./research/research-01-library-and-concurrency-decisions.md` §4 (idempotency), §7 (QR)
- Migration `20260424002_phase2.sql` (phase 02) adds `idx_tx_user_pkg_pending` UNIQUE partial index

## Overview
- **Priority:** P1
- **Status:** pending
- **Description:** `/buy` shows package menu. `/topup <package_code>` creates pending transaction + QR image. Idempotent: spam clicks → single pending row.

## Key Insights
- Packages defined as Go struct constant (not DB table — KISS; changes rare, code review easier). One source of truth in `service/packages.go`.
- **[F2] Order code = 12 uppercase hex chars** from first 6 bytes of UUID (`hex.EncodeToString(uuidBytes[:6])` then `ToUpper`). 48-bit space → collision ~1 in 280T. Stored in `transactions.provider_ref`. SePay webhook content format `SBF TOPUP <12CHAR>`.
- **[F2] Provider_ref UNIQUE is partial** on active states (`pending`, `paid`, `recovered_by_late_payment`) — cancelled/failed/refunded rows can share with new attempts. Migration `20260424004` (phase-04) drops the unconditional UNIQUE and adds partial index.
- QR URL is ephemeral per order; no caching. `tgbotapi.NewPhoto` with URL string — Telegram fetches & caches on their end.
- FSM `topup_waiting` TTL 24h. User can leave app, come back, still see pending status.
- Double-tap prevention: Redis `SET NX topup_lock:<user_id>:<package_code>` TTL 30s + DB unique partial index.
- **[Q2] Cancel-then-pay policy:** user cancels pending → status='cancelled' (via CancelPendingTransaction). If SePay webhook arrives later matching that provider_ref → webhook CAS widens to `status IN ('pending','cancelled')` and flips to `recovered_by_late_payment` with full credits granted. Handled in phase-06.

## Requirements
### Functional
- `/buy` — shows inline keyboard: 2 rows Standard + 2 rows Premium + 1 row Combo. Each button callback `buy:pkg:<code>`.
- Callback `buy:pkg:<code>` → show package summary + Confirm/Cancel.
- Confirm → `TransactionService.CreateTopupIntent(ctx, userID, packageCode)` → returns transaction + QR URL.
- Send QR photo with caption (order code, amount, bank, account, description), inline buttons [Đã thanh toán / Đã chuyển khoản] [Hủy].
- FSM = `topup_waiting`, data = `{tx_id, package_code, expires_at}`.
- `/topup` alias: if state=idle → /buy; if state=topup_waiting → resend last QR.
- "Đã thanh toán" button → poll DB once for status. If still pending → edit caption "Đang chờ... Vui lòng đợi tối đa 5 phút". If paid → success message (most cases webhook already flipped state).
- "Hủy" → UPDATE status='failed' + metadata.cancel_reason=user_cancel, clear FSM.

### Non-functional
- Idempotent `CreateTopupIntent`: on UNIQUE violation (23505) on `idx_tx_user_pkg_pending` → SELECT existing pending row + return its QR.
- QR URL construction pure function: `BuildQRURL(bank, acc, amount, desc) string`.
- Package codes match regex `^(standard|premium)_(starter|basic|pro|max)_(50|100|200|300)$` or `^combo_(p100_s50|p200_s100)$`.
- **[F2]** On `23505` against `idx_tx_provider_ref_active` (astronomically rare at 48-bit): retry up to 3 times with fresh `orderCode`. Log warn. Abort with error after 3 retries.
- **[Q2] CancelPendingTransaction** sets `status='cancelled'` (not `'failed'`). Preserves provider_ref in active-state partial index, allowing recovery if user later pays. Phase-06 recovery branch flips to `recovered_by_late_payment`.

## Architecture

### Package registry (`service/packages.go`)
```go
type Package struct {
    Code            string
    DisplayVI       string
    DisplayEN       string
    AmountVND       int64
    PremiumCredits  int
    StandardCredits int
    Featured        bool // UI decoy
}

var Packages = map[string]Package{
    "standard_starter_50":  {Code: "standard_starter_50", AmountVND: 99_000, StandardCredits: 50, DisplayVI: "Standard Starter — 50cr"},
    "standard_basic_100":   {AmountVND: 179_000, StandardCredits: 100, ...},
    "standard_pro_200":     {AmountVND: 329_000, StandardCredits: 200, Featured: true, ...},
    "standard_max_300":     {AmountVND: 459_000, StandardCredits: 300, ...},
    "premium_starter_50":   {AmountVND: 499_000, PremiumCredits: 50, ...},
    "premium_basic_100":    {AmountVND: 899_000, PremiumCredits: 100, ...},
    "premium_pro_200":      {AmountVND: 1_699_000, PremiumCredits: 200, Featured: true, ...},
    "premium_max_300":      {AmountVND: 2_399_000, PremiumCredits: 300, ...},
    "combo_p100_s50":       {AmountVND: 999_000, PremiumCredits: 100, StandardCredits: 50, ...},
    "combo_p200_s100":      {AmountVND: 1_799_000, PremiumCredits: 200, StandardCredits: 100, ...},
}
```

### CreateTopupIntent (`transaction_service.go`) — [F2] 12-hex order code + provider_ref retry
```go
func (s *TransactionService) CreateTopupIntent(ctx context.Context, userID uuid.UUID, pkgCode string) (Transaction, string, error) {
    pkg, ok := Packages[pkgCode]
    if !ok { return Transaction{}, "", ErrUnknownPackage }

    // Redis lock (UX fast path)
    lockKey := fmt.Sprintf("topup_lock:%s:%s", userID, pkgCode)
    _, _ = s.rdb.SetNX(ctx, lockKey, "1", 30*time.Second).Result()

    // [F2] 12-hex uppercase from first 6 bytes of UUID
    for attempt := 0; attempt < 3; attempt++ {
        u := uuid.New()
        orderCode := strings.ToUpper(hex.EncodeToString(u[:6])) // 12 hex chars, 48-bit entropy

        row := s.pool.QueryRow(ctx, `
            INSERT INTO transactions (id, user_id, provider, provider_ref, package_code, amount_vnd, premium_granted, standard_granted, status)
            VALUES ($1, $2, 'sepay', $3, $4, $5, $6, $7, 'pending')
            RETURNING id, user_id, provider_ref, package_code, amount_vnd, premium_granted, standard_granted, status, created_at`,
            u, userID, orderCode, pkgCode, pkg.AmountVND, pkg.PremiumCredits, pkg.StandardCredits)

        var tx Transaction
        err := row.Scan(&tx.ID, &tx.UserID, &tx.ProviderRef, &tx.PackageCode, &tx.AmountVND, &tx.PremiumGranted, &tx.StandardGranted, &tx.Status, &tx.CreatedAt)
        if err == nil {
            qr := BuildQRURL(s.cfg.SepayBankCode, s.cfg.SepayBankAccount, pkg.AmountVND, fmt.Sprintf("SBF TOPUP %s", orderCode))
            return tx, qr, nil
        }
        var pgErr *pgconn.PgError
        if errors.As(err, &pgErr) && pgErr.Code == "23505" {
            switch pgErr.ConstraintName {
            case "idx_tx_user_pkg_pending":
                existing, qErr := s.q.GetPendingTxByUserPackage(ctx, userID, pkgCode)
                if qErr != nil { return Transaction{}, "", qErr }
                qr := BuildQRURL(s.cfg.SepayBankCode, s.cfg.SepayBankAccount, existing.AmountVND, fmt.Sprintf("SBF TOPUP %s", existing.ProviderRef))
                return existing, qr, nil
            case "idx_tx_provider_ref_active":
                s.log.Warn("provider_ref collision — retrying", zap.String("order_code", orderCode), zap.Int("attempt", attempt))
                continue
            }
        }
        return Transaction{}, "", err
    }
    return Transaction{}, "", fmt.Errorf("provider_ref collision after 3 retries")
}
```

### QR URL builder (`integration/sepay/qr.go`)
```go
func BuildQRURL(bankCode, accNo string, amount int64, des string) string {
    u := url.Values{}
    u.Set("acc", accNo)
    u.Set("bank", bankCode)
    u.Set("amount", strconv.FormatInt(amount, 10))
    u.Set("des", des)
    u.Set("template", "compact")
    return "https://qr.sepay.vn/img?" + u.Encode()
}
```

## Related Code Files
### Create
- `services/api/internal/service/packages.go` — static Package map
- `services/api/internal/service/transaction_service.go`
- `services/api/internal/service/transaction_service_test.go`
- `services/api/internal/bot/commands/buy.go`
- `services/api/internal/bot/commands/topup.go`
- `services/api/internal/bot/keyboards/buy.go` — inline keyboard builders
- `services/api/internal/integration/sepay/qr.go`
- `services/api/internal/integration/sepay/qr_test.go`
- `services/api/internal/db/queries/transactions.sql`

### Modify
- `services/api/internal/config/config.go` — add `SepayBankCode` (env `SEPAY_BANK_CODE`) + `SepayBankAccount` (env `SEPAY_BANK_ACCOUNT`) — both used by QR builder AND webhook match (Q3)
- `services/api/internal/bot/router.go` — route `/buy`, `/topup`, callbacks `buy:pkg:*`, `buy:confirm:*`, `buy:cancel`, `topup:check`, `topup:cancel`

## sqlc queries
```sql
-- services/api/internal/db/queries/transactions.sql

-- name: InsertPendingTransaction :one
INSERT INTO transactions (user_id, provider, provider_ref, package_code, amount_vnd, premium_granted, standard_granted)
VALUES ($1, 'sepay', $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetPendingTxByUserPackage :one
SELECT * FROM transactions
WHERE user_id = $1 AND package_code = $2 AND status = 'pending'
ORDER BY created_at DESC LIMIT 1;

-- name: GetTxByProviderRef :one
SELECT * FROM transactions WHERE provider = 'sepay' AND provider_ref = $1 LIMIT 1;

-- name: CancelPendingTransaction :exec
-- [Q2] cancelled (not failed) — keeps provider_ref in active-state partial index for late-payment recovery
UPDATE transactions
SET status = 'cancelled', updated_at = NOW(), metadata = metadata || jsonb_build_object('cancel_reason', $2::text, 'cancelled_at', to_char(NOW(), 'YYYY-MM-DD"T"HH24:MI:SSOF'))
WHERE id = $1 AND status = 'pending';

-- name: GetTxByUserPage :many
SELECT * FROM transactions WHERE user_id = $1
ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: CountTxByUser :one
SELECT COUNT(*) FROM transactions WHERE user_id = $1;
```

## Implementation Steps
1. Add env `SEPAY_BANK_CODE` + `SEPAY_BANK_ACCOUNT` to config (Q3 rename from `SEPAY_BANK_ACC` — used by QR builder + webhook match).
2. Write `service/packages.go` with full package registry.
3. Write `db/queries/transactions.sql`; run `sqlc generate`.
4. Write `integration/sepay/qr.go` + pure-function test (URL encoding assertions).
5. **[F2]** Write `service/transaction_service.go` with `CreateTopupIntent` (12-hex uppercase via `hex.EncodeToString(uuid[:6])` + up to 3 retries on `idx_tx_provider_ref_active` 23505), `CancelPendingTransaction` (status='cancelled' per Q2), `GetTxByRef`.
6. Handle `23505 unique_violation` by constraint name:
   - `idx_tx_user_pkg_pending` → fetch existing pending, return same QR (idempotent UX).
   - `idx_tx_provider_ref_active` → retry with fresh order code (up to 3x).
7. Write `bot/keyboards/buy.go` helpers: `PackageMenu(lang) InlineKeyboardMarkup`, `ConfirmCancel(pkgCode) InlineKeyboardMarkup`, `TopupActions(txID) InlineKeyboardMarkup`.
8. Write `bot/commands/buy.go`:
   - `/buy` → send package menu, state=`buy_selecting_package`.
   - Callback `buy:pkg:<code>` → show summary, state=`buy_confirming`, data={pkg_code}.
   - Callback `buy:confirm:<code>` → CreateTopupIntent → send photo QR + caption.
   - Callback `buy:cancel` → clear state, edit message "đã huỷ".
9. Write `bot/commands/topup.go`:
   - `/topup` → resend last pending QR (from FSM data); if no pending, redirect to `/buy`.
   - Callback `topup:check` → SELECT status; if paid/recovered_by_late_payment → success message + clear FSM; else reply "still waiting".
   - Callback `topup:cancel` → CancelPendingTransaction (flips to `cancelled`), clear FSM.
10. Integration test:
    - **[F2]** CreateTopupIntent → row inserted, `provider_ref` matches regex `^[A-F0-9]{12}$`, QR URL well-formed (host=qr.sepay.vn, correct params).
    - Second CreateTopupIntent same user+package → returns SAME tx (idempotent via `idx_tx_user_pkg_pending`).
    - Concurrent 10x CreateTopupIntent → exactly 1 tx row created.
    - **[Q2]** CancelPendingTransaction → status='cancelled' (not 'failed'), provider_ref retained in `idx_tx_provider_ref_active`.
    - `/buy` callback flow in bot sim test (mock tgbotapi) — state transitions correct.

## Todo List
- [ ] Add `SEPAY_BANK_CODE` + `SEPAY_BANK_ACCOUNT` to config (Q3 env names)
- [ ] Write `service/packages.go`
- [ ] Write `db/queries/transactions.sql`
- [ ] Run `sqlc generate`
- [ ] Implement QR builder + tests
- [ ] **[F2]** Implement `TransactionService.CreateTopupIntent` — 12-hex uppercase order code + retry-on-provider_ref-collision (3x)
- [ ] Implement keyboards (menu / confirm / topup actions)
- [ ] Implement `bot/commands/buy.go`
- [ ] **[Q2]** Implement `bot/commands/topup.go` — cancel flips status to `cancelled` (not `failed`)
- [ ] Wire router + callbacks
- [ ] Integration test for idempotent create
- [ ] Race test: 10x concurrent CreateTopupIntent → 1 row
- [ ] **[F2]** Test provider_ref format `^[A-F0-9]{12}$` via 1000 generated samples
- [ ] **[Q2]** Test cancelled tx preserves provider_ref in active partial index

## Success Criteria
- Spam clicking `/buy confirm` → exactly 1 pending row
- QR URL renders correctly in Telegram (visual check)
- `topup:cancel` flips row to `failed` with metadata audit trail
- `go test -race` passes

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| 23505 constraint name change breaks idempotent handling | Low | Med | Use explicit constraint name in error check (`idx_tx_user_pkg_pending` + `idx_tx_provider_ref_active`); covered by migration test |
| FSM `topup_waiting` TTL expires while user paying | Med | Med | 24h TTL; after expiry user can re-/buy and old pending auto-expires via janitor (deferred Phase 8) |
| Package registry drift between bot menu & webhook grant | Low | High | Single source `Packages` map used by BOTH menu render and webhook grant (phase 06) |
| Package price change mid-pending → user paid old amount → webhook grants new amount | Med | Med | Snapshot amount_vnd/premium/standard INTO `transactions` row on create; webhook reads from row, not registry |
| Cancel-then-pay: late webhook matches cancelled tx | Med | Med | **[Q2]** status='cancelled' retains provider_ref in partial index; webhook flips to `recovered_by_late_payment` + full credits (phase-06) |
| Provider_ref 12-hex collision | Extremely Low | High | **[F2]** 48-bit space; retry up to 3x on `idx_tx_provider_ref_active` 23505 |
| QR description param URL-encoded breaks SePay parse | Med | High | Description is ASCII + spaces; URL.Values.Encode() produces `SBF+TOPUP+...`, SePay docs accept both `+` and `%20`. Test with real SePay sandbox in phase 10 |

## Security Considerations
- `package_code` parameter validated via regex + map lookup BEFORE any DB insert; rejects SQLi attempts via callback.
- `transactions.amount_vnd` FROZEN at insert-time (read from package registry snapshot); webhook compares against this row, not live registry.
- `provider_ref` unique by partial constraint inside `transactions` — no collision even across users (UUID source).
- Bank credentials env-only, never in repo, logged masked.

## Next Steps
- Phase 06 webhook consumes pending transactions created here.
- Phase 07 `/history` reads from `transactions` + `ledger`.
- Phase 08 admin can force-cancel stale pending.
