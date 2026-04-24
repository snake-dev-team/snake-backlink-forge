# Phase 08 — Admin Commands (`/admin *`)

## Context Links
- Spec: `docs/MASTER_PROMPT.md` §5.1 commands (admin row), §12.1 `ADMIN_TELEGRAM_IDS` env
- Depends on: phase 02 (users), 04 (wallet), 05 (tx), 07 (support)

## Overview
- **Priority:** P2
- **Status:** pending
- **Description:** Admin-only command namespace. Role check via `config.AdminTelegramIDs` (already present in config.go). Stats, manual credit grant, ban/unban, user lookup. Every admin action audit-logged.

## Key Insights
- Admin role check is middleware-level: `isAdmin(tgID)` via `slices.Contains(cfg.AdminTelegramIDs, tgID)`. No DB role table needed — YAGNI.
- Admin commands use `/admin <subcmd> <args>` pattern, single command router with subcmd switch.
- Every admin write action (grant, ban, unban) MUST write to `audit_log` with `event` like `admin_grant | admin_ban | admin_unban`.
- **[F5] Audit ordering for admin_grant:** INSERT audit_log BEFORE `grant_credits` call, within same pgx.Tx. On grant failure → ROLLBACK drops audit row atomically. No orphan audit rows claiming side effects that didn't happen.
- **[M3] Self-ban guard:** `/admin ban <tg>` rejects if target tg_id ∈ `cfg.AdminTelegramIDs`. Prevents admin lockout.
- **[L3] `ADMIN_TELEGRAM_IDS` env parsing:** comma-separated int64 list — `strings.Split(",") + strconv.ParseInt`. Fail boot on non-int. Log warn on dupes.
- `/admin lookup <tg_id|phone|key_prefix>` — 3-way search: numeric → by telegram_id; starts with `+` → by phone_e164; starts with `sbf_live_` → by key_prefix.

## Requirements

### `/admin stats`
Read-only aggregates:
- Total users / verified / banned / trial_used
- Active keys count
- Transactions: pending count, paid count 24h, manual_review count, total VND 24h
- Credits outstanding: SUM(premium_credits), SUM(standard_credits)
- Open support tickets count
- Last 10 auth_fail events (sepay) in 24h

Reply as monospace table.

### `/admin grant <tg_id> <pool> <amount> [reason]`
- Validate pool ∈ {premium, standard}, amount > 0, amount <= 10000 (sanity cap).
- Resolve user by tg_id. If not found → error.
- **[F5] Ordering within single pgx.Tx:**
  1. `BEGIN`
  2. `INSERT INTO audit_log (user_id, event, metadata) VALUES ($admin_user_id, 'admin_grant_credits', jsonb_build_object('target_user_id', $target, 'pool', $pool, 'amount', $amount, 'reason', $reason)) RETURNING id` → `audit_id`
  3. `SELECT grant_credits($target_user_id, $pool, $amount, 'admin_adjust', 'audit_log', $audit_id)` → `new_balance`
  4. `COMMIT`
  - If step 3 fails (e.g., P0001 INSUFFICIENT_CREDITS for negative adjusts, schema error): `ROLLBACK` drops the audit row. No orphan audit.
- Reply confirmation with new balance + optional reason in audit metadata.

### `/admin ban <tg_id> [reason]`
- **[M3] Self-ban guard:** reject if target `tg_id` ∈ `cfg.AdminTelegramIDs` → return `ErrCannotBanAdmin`, render `admin_self_ban_blocked` template (do NOT leak list of admin IDs in reply).
- `UPDATE users SET is_banned=TRUE WHERE telegram_id=$1`
- Audit log (post-UPDATE but same tx for consistency).
- Send DM to banned user: "Tài khoản bị khoá. Liên hệ @support."
- Their active key not revoked (Phase 3 auth middleware checks `users.is_banned` at request time).

### `/admin unban <tg_id>`
- Inverse.

### `/admin lookup <identifier>`
- Heuristic parse identifier → SELECT ... + render user summary:
  - UUID, telegram_id, telegram_username, phone_hash (masked), created_at, is_verified, is_banned, trial_used, language
  - Wallet: premium / standard / spent_vnd
  - Key: prefix + is_active + last_used_at
  - Last 5 transactions
  - Last 5 ledger events

## Architecture

### Router (`bot/commands/admin.go`)
```go
func (h *AdminCmd) Handle(ctx context.Context, update Update) (Reply, error) {
    if !h.isAdmin(update.From.ID) {
        return Reply{}, nil // silent ignore — do not reveal admin command existence
    }
    args := strings.Fields(update.Message.Text)
    if len(args) < 2 { return replyAdminMenu(), nil }
    switch args[1] {
    case "stats":  return h.stats(ctx)
    case "grant":  return h.grant(ctx, update.From.ID, args[2:])
    case "ban":    return h.ban(ctx, update.From.ID, args[2:])
    case "unban":  return h.unban(ctx, update.From.ID, args[2:])
    case "lookup": return h.lookup(ctx, args[2:])
    default:       return reply(tmpl.Render("admin_unknown_subcmd")), nil
    }
}
```

### Audit service (`service/audit_service.go`)
Thin wrapper around `INSERT INTO audit_log`. Called from every admin write action + every security event.

```go
type AuditService struct { pool *pgxpool.Pool; log *zap.Logger }

func (s *AuditService) Log(ctx, in AuditInput) error {
    return s.pool.QueryRow(ctx, `
        INSERT INTO audit_log (user_id, key_id, event, ip_hash, country, metadata)
        VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
        in.UserID, in.KeyID, in.Event, in.IPHash, in.Country, in.Metadata,
    ).Scan(&in.ID)
}
```

Called from:
- phase 02 trial grant → event=`trial_granted`
- phase 03 key issue / revoke → event=`key_issued | key_revoked`
- phase 06 sepay webhook all branches → event=`sepay_success | sepay_auth_fail | sepay_unmatched_transfer | sepay_underpaid | sepay_replay`
- phase 08 admin grant / ban / unban → event=`admin_grant | admin_ban | admin_unban`

### Lookup parse
```go
func parseLookupIdent(s string) (kind, val string) {
    if _, err := strconv.ParseInt(s, 10, 64); err == nil { return "telegram_id", s }
    if strings.HasPrefix(s, "+") { return "phone", s }
    if strings.HasPrefix(s, "sbf_live_") { return "key_prefix", s[:12] }
    return "unknown", s
}
```

## Related Code Files
### Create
- `services/api/internal/bot/commands/admin.go`
- `services/api/internal/service/audit_service.go`
- `services/api/internal/service/audit_service_test.go`
- `services/api/internal/service/admin_service.go` — stats aggregator (reused by Phase 08 itself; future admin dashboard Phase 2-extended)
- `services/api/internal/db/queries/audit.sql`
- `services/api/internal/db/queries/admin_stats.sql`

### Modify
- `services/api/internal/bot/middleware.go` — add `isAdmin` helper (reads cfg.AdminTelegramIDs)
- `services/api/internal/bot/router.go` — route `/admin` command
- Services in phases 02/03/06 — inject `*AuditService` dep, call `Log` at audit points

## sqlc queries (new)
```sql
-- services/api/internal/db/queries/audit.sql

-- name: InsertAuditLog :one
INSERT INTO audit_log (user_id, key_id, event, ip_hash, country, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAuditLogByEventSince :many
SELECT * FROM audit_log WHERE event = $1 AND created_at > $2
ORDER BY created_at DESC LIMIT $3;

-- services/api/internal/db/queries/admin_stats.sql

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: CountUsersBanned :one
SELECT COUNT(*) FROM users WHERE is_banned = TRUE;

-- name: CountUsersVerified :one
SELECT COUNT(*) FROM users WHERE is_verified = TRUE;

-- name: CountUsersTrialUsed :one
SELECT COUNT(*) FROM users WHERE trial_used = TRUE;

-- name: CountActiveKeys :one
SELECT COUNT(*) FROM api_keys WHERE is_active = TRUE;

-- name: TxStats24h :one
SELECT
  COALESCE(SUM(amount_vnd) FILTER (WHERE status='paid' AND paid_at > NOW() - INTERVAL '24 hours'), 0)::bigint AS revenue_24h,
  COUNT(*) FILTER (WHERE status='paid' AND paid_at > NOW() - INTERVAL '24 hours') AS paid_24h,
  COUNT(*) FILTER (WHERE status='pending') AS pending,
  COUNT(*) FILTER (WHERE status='manual_review') AS manual_review
FROM transactions;

-- name: CreditsOutstanding :one
SELECT COALESCE(SUM(premium_credits),0)::bigint AS premium, COALESCE(SUM(standard_credits),0)::bigint AS standard FROM wallets;

-- name: GetUserByPhone :one
SELECT * FROM users WHERE phone_e164 = $1 LIMIT 1;

-- name: GetUserByKeyPrefix :one
SELECT u.* FROM users u JOIN api_keys k ON k.user_id = u.id
WHERE k.key_prefix = $1 AND k.is_active = TRUE LIMIT 1;

-- name: BanUser :exec
UPDATE users SET is_banned = TRUE, updated_at = NOW() WHERE telegram_id = $1;

-- name: UnbanUser :exec
UPDATE users SET is_banned = FALSE, updated_at = NOW() WHERE telegram_id = $1;
```

## Implementation Steps
1. Write `db/queries/audit.sql` + `admin_stats.sql`; `sqlc generate`.
2. Write `service/audit_service.go`.
3. **[L3]** In `config/config.go`: implement `parseAdminTelegramIDs(raw string) ([]int64, error)` — `strings.Split(",") + strings.TrimSpace + strconv.ParseInt(base=10, bitSize=64)`. Error on any non-int. Use `map[int64]struct{}` to detect dupes → log warn, keep first occurrence.
4. Write `service/admin_service.go` with:
   - `Stats(ctx)`
   - **[F5]** `Grant(ctx, adminUserID, targetTGID, pool, amt, reason)` — audit_log INSERT then grant_credits, both in single pgx.Tx; return (newBalance, err).
   - **[M3]** `Ban(ctx, adminUserID, targetTGID, reason)` — reject if target tg_id ∈ `cfg.AdminTelegramIDs`; else UPDATE + audit_log in same tx.
   - `Unban`, `Lookup`.
5. Write `bot/commands/admin.go` dispatcher.
6. Wire `AuditService` dep into phase 02/03/06 services. Propagate via Deps struct.
7. Add `isAdmin(tgID)` to middleware — use in `/admin` router short-circuit.
8. Write integration test:
   - Non-admin sends `/admin stats` → bot ignores (no reply).
   - Admin `/admin grant 123 standard 100 reason=test` → wallet +100, audit_log has row with event=`admin_grant_credits` and metadata.reason=test.
   - **[F5]** Force grant_credits failure (mock `pgconn.PgError{Code:"P0001"}` via stored-proc monkey-patch OR deliberately invalid pool through a raw SQL path) → audit_log row ABSENT after rollback.
   - Admin `/admin ban 123` → user.is_banned=TRUE, audit row, banned user's subsequent `/balance` → "account disabled".
   - **[M3]** Admin `/admin ban <self_tg_id>` → reply `admin_self_ban_blocked`, user.is_banned UNCHANGED, no audit row.
   - Admin `/admin lookup 123` → returns user summary.
   - **[L3]** Config boot with `ADMIN_TELEGRAM_IDS="abc,123"` → fail startup with parse error.

## Todo List
- [ ] Write `audit.sql` + `admin_stats.sql` queries
- [ ] Run `sqlc generate`
- [ ] Implement `AuditService`
- [ ] **[L3]** Implement `parseAdminTelegramIDs` in `config.go` with dupe warning + non-int fail
- [ ] **[F5]** Implement `AdminService.Grant` with audit-then-grant ordering in single pgx.Tx
- [ ] **[M3]** Implement `AdminService.Ban` self-ban guard
- [ ] Implement `AdminService` stats/unban/lookup
- [ ] Implement `bot/commands/admin.go` router
- [ ] Inject AuditService into phases 02/03/06 services
- [ ] Call `Audit.Log` at all audit points
- [ ] Unit test audit insert
- [ ] Integration test admin grant flow
- [ ] **[F5]** Integration test: grant_credits failure → audit row NOT present
- [ ] **[M3]** Integration test: /admin ban on admin's own tg_id rejected, users row unchanged
- [ ] Integration test non-admin silent ignore
- [ ] Manual smoke: real admin TG ID → `/admin stats` works

## Success Criteria
- Admin role check zero-cost (env-driven int slice check)
- Every admin write action produces audit_log row
- Non-admin users see no acknowledgement of `/admin` (no info leak)
- `/admin stats` responds < 1s on 10k user DB
- Banned user cannot invoke write commands

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Admin accidentally grants to wrong tg_id | Med | Med | `/admin grant` replies with username + current balance; admin eyeballs before next action |
| Admin list leak (env misconfig) | Low | Critical | `ADMIN_TELEGRAM_IDS` treated as secret, Fly secret not env file. Document in deploy checklist |
| Audit log insert fails silently after side-effect commits | Low | Med | **[F5]** For `admin_grant_credits`: audit INSERT happens BEFORE grant_credits in SAME tx; on grant failure, rollback drops audit row atomically. Post-commit audit events (trial_granted, sepay_success) remain best-effort with warn-log on failure |
| Admin bans own tg_id → locked out | Low | Critical | **[M3]** `AdminService.Ban` rejects if `target.telegram_id IN cfg.AdminTelegramIDs` → `ErrCannotBanAdmin` template reply |
| Malformed `ADMIN_TELEGRAM_IDS` env crashes boot | Low | High | **[L3]** Explicit parse in `config.go` with validation; fail-fast on non-int; log warn + dedupe on dupes |
| Lookup leaks phone PII to admin chat | Low | Low | Masked display: `phone: +84***54321`; raw phone only on explicit `lookup +84...` match |
| `/admin grant amount=10000000000` overflows int32 | Low | Med | Cap at 10_000 per invocation; docstring on service method |
| Stats query scans whole wallets table on big DB | Low | Low | SUM acceptable at 10k users; Phase 08+ introduce `daily_stats` materialized view |

## Security Considerations
- Silent ignore for non-admin `/admin *` — doesn't reveal existence of admin namespace.
- Audit log is immutable by convention (no UPDATE/DELETE queries). Consider `REVOKE UPDATE, DELETE ON audit_log FROM api_user` in Phase 10 DB role setup.
- Admin grant bypasses normal credit flow → MUST audit with reason + metadata to justify in postmortems.
- IP hash of admin NOT captured (bot long-poll has no IP); metadata field `source="telegram_admin"` instead.

## Next Steps
- Phase 10 tests include admin role coverage.
- Post-Phase-2: admin web dashboard (out of scope).
