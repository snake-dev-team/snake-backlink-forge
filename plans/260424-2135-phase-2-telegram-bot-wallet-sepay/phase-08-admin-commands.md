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
- **[F5 Option A — round-3 rework] Admin grant ordering:** avoids BIGSERIAL→UUID type collision from round-2. Ledger row uses `ref_type='user', ref_id=target_user_id (UUID)`; audit row captures `metadata.ledger_id (bigint)` via `currval('ledger_id_seq')` within same session. Both rows land in single `pgx.Tx` — grant fail → audit NOT inserted; audit fail → grant ROLLBACK.
- **[F5] Chain-of-evidence:** `ledger.user_id + ledger.created_at` ↔ `audit_log.user_id + audit_log.created_at + metadata.ledger_id` — forensic JOIN path preserved.
- **[round-3] Auth-fail burst alert:** `auditFailAlertWatcher` goroutine spawned in `main.go` polls `audit_log` for `sepay_auth_fail` every 60s; threshold ≥20-in-15min → admin alert via `adminAlertCh`. Window-bucket dedup prevents duplicate alerts within same 15-min bucket.
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
- **[F5 Option A — round-3] Ordering within single pgx.Tx (avoids BIGSERIAL↔UUID collision):**
  1. `BEGIN` (pgx.Tx, default READ COMMITTED)
  2. `SELECT grant_credits(p_user_id := $target, p_pool := $pool, p_amount := $amount, p_event_type := 'admin_adjust', p_ref_type := 'user', p_ref_id := $target_uuid)` → returns `new_balance` (int)
     - `ref_type='user'`, `ref_id=target_user_id` (UUID — fits `ledger.ref_entity_id UUID`)
  3. `SELECT currval('ledger_id_seq')` → `ledger_id` (bigint) — safe within same session per Postgres spec; captures id of the ledger row just inserted by grant_credits
  4. `INSERT INTO audit_log (user_id, event, metadata) VALUES ($target_user_id, 'admin_grant', jsonb_build_object('ledger_id', $ledger_id::bigint, 'amount', $amount, 'pool', $pool, 'admin_tg_id', $admin_tg_id, 'reason', $reason)) RETURNING id` → `audit_id`
  5. `COMMIT`
  - **Atomicity guarantee:** grant fails → step 2 errors → ROLLBACK drops nothing (no writes yet). Audit INSERT fails → step 4 errors → ROLLBACK drops ledger + wallet delta from step 2. COMMIT success → both rows written.
  - **Chain-of-evidence:** audit → ledger via `audit_log.metadata->>'ledger_id'` (bigint). Ledger → audit via `(ledger.user_id, ledger.created_at)` fuzzy match within ±1s to `(audit_log.user_id, audit_log.created_at)`.
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
- phase 08 admin grant / ban / unban → event=`admin_grant | admin_ban | admin_unban` (note: `admin_grant` is canonical — round-2 used `admin_grant_credits`, renamed to match F5 Option A rework)

### Lookup parse
```go
func parseLookupIdent(s string) (kind, val string) {
    if _, err := strconv.ParseInt(s, 10, 64); err == nil { return "telegram_id", s }
    if strings.HasPrefix(s, "+") { return "phone", s }
    if strings.HasPrefix(s, "sbf_live_") { return "key_prefix", s[:12] }
    return "unknown", s
}
```

### [F5 Option A] AdminService.Grant pseudocode — round-3 rewrite
```go
func (s *AdminService) Grant(ctx context.Context, adminTGID int64, targetUserID uuid.UUID,
    pool string, amount int, reason string) (newBalance int, err error) {

    tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
    if err != nil { return 0, err }
    defer tx.Rollback(ctx)

    // Step 1: grant_credits with ref_type='user', ref_id=target_user_id (UUID fits UUID)
    if err := tx.QueryRow(ctx,
        `SELECT grant_credits($1, $2, $3, 'admin_adjust', 'user', $1)`,
        targetUserID, pool, amount,
    ).Scan(&newBalance); err != nil {
        return 0, fmt.Errorf("grant_credits: %w", err)
    }

    // Step 2: capture ledger_id via currval (same session — safe per PG spec)
    var ledgerID int64
    if err := tx.QueryRow(ctx, `SELECT currval('ledger_id_seq')`).Scan(&ledgerID); err != nil {
        return 0, fmt.Errorf("currval ledger_id: %w", err)
    }

    // Step 3: audit_log with ledger_id in metadata (bigint → jsonb)
    _, err = tx.Exec(ctx,
        `INSERT INTO audit_log (user_id, event, metadata)
         VALUES ($1, 'admin_grant', jsonb_build_object(
             'ledger_id', $2::bigint,
             'amount', $3::int,
             'pool', $4::text,
             'admin_tg_id', $5::bigint,
             'reason', $6::text))`,
        targetUserID, ledgerID, amount, pool, adminTGID, reason,
    )
    if err != nil {
        return 0, fmt.Errorf("audit_log insert: %w", err)
    }

    if err := tx.Commit(ctx); err != nil { return 0, err }
    return newBalance, nil
}
```

### [round-3] auditFailAlertWatcher goroutine (`bot/admin_alerts.go` — new)
```go
// auditFailAlertWatcher polls audit_log for sepay_auth_fail bursts.
// Spawned in cmd/api/main.go: go auditFailAlertWatcher(rootCtx, pool, adminAlertCh, logger)
// Threshold: 20-in-15min; 60s tick; window-bucket dedup prevents duplicate alerts per 15-min bucket.
func auditFailAlertWatcher(ctx context.Context, db *pgxpool.Pool, alertCh chan<- AdminAlert, log *zap.Logger) {
    ticker := time.NewTicker(60 * time.Second)
    defer ticker.Stop()
    alertedWindows := make(map[int64]struct{}) // bucket -> alerted

    for {
        select {
        case <-ctx.Done():
            return
        case now := <-ticker.C:
            bucket := now.Unix() / (15 * 60) // 15-min window bucket
            if _, already := alertedWindows[bucket]; already {
                continue
            }
            var count int
            err := db.QueryRow(ctx,
                `SELECT COUNT(*) FROM audit_log
                 WHERE event='sepay_auth_fail'
                 AND created_at > NOW() - INTERVAL '15 minutes'`,
            ).Scan(&count)
            if err != nil {
                log.Warn("auth_fail poll failed", zap.Error(err))
                continue
            }
            if count >= 20 {
                select {
                case alertCh <- AdminAlert{
                    Kind: "auth_fail_burst",
                    Err:  fmt.Sprintf("%d SePay webhook auth failures in 15min. Check SEPAY_WEBHOOK_TOKEN rotation or attack.", count),
                    At:   now,
                }:
                    alertedWindows[bucket] = struct{}{}
                    // GC old buckets (keep last 4 to guard against clock skew)
                    for b := range alertedWindows {
                        if b < bucket-4 {
                            delete(alertedWindows, b)
                        }
                    }
                default:
                    log.Warn("adminAlertCh full, dropping auth_fail_burst alert",
                        zap.Int("count", count), zap.Int64("bucket", bucket))
                }
            }
        }
    }
}
```

**Naming note:** audit event name is canonical `sepay_auth_fail` (pre-existing in plan). User directive also referenced `webhook_auth_fail` as alias — normalized to `sepay_auth_fail` throughout this plan for consistency with phase-06 audit taxonomy.

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
- `services/api/internal/bot/admin_alerts.go` — **[round-3]** add `auditFailAlertWatcher` alongside existing `consumeAdminAlerts` + `sendAdminAlertNonBlocking` (created in phase-06)
- `services/api/cmd/api/main.go` — **[round-3]** spawn `go auditFailAlertWatcher(rootCtx, pool, adminAlertCh, log)` at server init (pool + logger already wired; adminAlertCh created in phase-06)
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
   - **[F5 Option A — round-3]** `Grant(ctx, adminTGID, targetUserID, pool, amt, reason)`: BEGIN pgx.Tx → `SELECT grant_credits(..., ref_type='user', ref_id=target_user_id::UUID)` → `SELECT currval('ledger_id_seq')` → `INSERT audit_log (event='admin_grant', metadata.ledger_id=$ledger_id::bigint, .amount, .pool, .admin_tg_id, .reason)` → COMMIT.
   - **[M3]** `Ban(ctx, adminUserID, targetTGID, reason)` — reject if target tg_id ∈ `cfg.AdminTelegramIDs`; else UPDATE + audit_log in same tx.
   - `Unban`, `Lookup`.
5. Write `bot/commands/admin.go` dispatcher.
6. **[round-3]** Write `bot/admin_alerts.go` → `auditFailAlertWatcher(ctx, pool, adminAlertCh, log)` goroutine: 60s ticker, `COUNT(*) FROM audit_log WHERE event='sepay_auth_fail' AND created_at > NOW() - INTERVAL '15 minutes'` ≥ 20 → `AdminAlert{Kind:"auth_fail_burst"}`. In-memory bucket dedup on `now.Unix() / (15*60)`; GC buckets older than `bucket-4`.
7. **[round-3]** In `cmd/api/main.go`: spawn `go auditFailAlertWatcher(rootCtx, pool, adminAlertCh, log)` alongside existing `consumeAdminAlerts` goroutine. Both bound to rootCtx; exit on shutdown.
8. Wire `AuditService` dep into phase 02/03/06 services. Propagate via Deps struct.
9. Add `isAdmin(tgID)` to middleware — use in `/admin` router short-circuit.
10. Write integration test:
    - Non-admin sends `/admin stats` → bot ignores (no reply).
    - Admin `/admin grant 123 standard 100 reason=test` → wallet +100, audit_log has row with event=`admin_grant` and metadata.ledger_id populated + metadata.reason=test.
    - **[F5]** `TestAdminGrantAtomicRollback` (3 cases — see phase-10 Suite F): (a) force grant_credits to fail (FK violation via non-existent target_user_id) → audit_log + ledger + wallet all unchanged; (b) force audit INSERT to fail (test-only trigger on audit_log) → ledger + wallet all unchanged; (c) happy path → `audit_log.metadata->>'ledger_id'` matches actual ledger.id.
    - Admin `/admin ban 123` → user.is_banned=TRUE, audit row, banned user's subsequent `/balance` → "account disabled".
    - **[M3]** Admin `/admin ban <self_tg_id>` → reply `admin_self_ban_blocked`, user.is_banned UNCHANGED, no audit row.
    - Admin `/admin lookup 123` → returns user summary.
    - **[L3]** Config boot with `ADMIN_TELEGRAM_IDS="abc,123"` → fail startup with parse error.
    - **[round-3]** `TestAuthFailBurstAlert`: seed 20 `audit_log(event='sepay_auth_fail', created_at > NOW()-15min)` rows → start watcher with 1s ticker override → assert alert on channel within 2s; re-fire → assert NO duplicate for same 15-min bucket.

## Todo List
- [ ] Write `audit.sql` + `admin_stats.sql` queries
- [ ] Run `sqlc generate`
- [ ] Implement `AuditService`
- [ ] **[L3]** Implement `parseAdminTelegramIDs` in `config.go` with dupe warning + non-int fail
- [ ] **[F5 Option A — round-3]** Implement `AdminService.Grant` — single pgx.Tx: grant_credits(ref_type='user', ref_id=target_uuid) → currval(ledger_id_seq) → INSERT audit_log with metadata.ledger_id → COMMIT
- [ ] **[M3]** Implement `AdminService.Ban` self-ban guard
- [ ] Implement `AdminService` stats/unban/lookup
- [ ] Implement `bot/commands/admin.go` router
- [ ] **[round-3]** Implement `auditFailAlertWatcher` goroutine in `bot/admin_alerts.go` (60s ticker, 15min window, threshold 20, in-memory bucket dedup)
- [ ] **[round-3]** Spawn `auditFailAlertWatcher` in `cmd/api/main.go` alongside `consumeAdminAlerts`
- [ ] Inject AuditService into phases 02/03/06 services
- [ ] Call `Audit.Log` at all audit points
- [ ] Unit test audit insert
- [ ] Integration test admin grant flow
- [ ] **[F5 Option A — round-3]** Integration test `TestAdminGrantAtomicRollback` — 3 cases (grant fail, audit fail, happy path with ledger_id match)
- [ ] **[round-3]** Integration test `TestAuthFailBurstAlert` — seed 20 sepay_auth_fail rows → alert fires once, dedup prevents duplicate
- [ ] **[M3]** Integration test: /admin ban on admin's own tg_id rejected, users row unchanged
- [ ] Integration test non-admin silent ignore
- [ ] Manual smoke: real admin TG ID → `/admin stats` works

## Success Criteria
- Admin role check zero-cost (env-driven int slice check)
- Every admin write action produces audit_log row
- Non-admin users see no acknowledgement of `/admin` (no info leak)
- `/admin stats` responds < 1s on 10k user DB
- Banned user cannot invoke write commands
- **[F5 Option A — round-3]** `/admin grant` atomicity: BEGIN → grant_credits(ref_type='user', ref_id=target_user_id UUID) → currval(ledger_id_seq) → INSERT audit_log(metadata.ledger_id=$bigint) → COMMIT. Inject grant failure → both rows absent. Inject audit failure → both rows absent. Happy path → `audit_log.metadata->>'ledger_id'` = actual ledger.id.
- **[round-3] `auditFailAlertWatcher`** fires alert exactly once per 15-min window on threshold breach (20 sepay_auth_fail in 15min); in-memory bucket map prevents duplicate alerts for same bucket.

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Admin accidentally grants to wrong tg_id | Med | Med | `/admin grant` replies with username + current balance; admin eyeballs before next action |
| Admin list leak (env misconfig) | Low | Critical | `ADMIN_TELEGRAM_IDS` treated as secret, Fly secret not env file. Document in deploy checklist |
| Audit log insert fails silently after side-effect commits | Low | Med | **[F5 Option A — round-3]** For `admin_grant`: single pgx.Tx → grant_credits(ref_type='user', ref_id=target_uuid) first, capture ledger_id via `currval('ledger_id_seq')`, then INSERT audit_log with ledger_id in metadata (bigint in jsonb — no type collision), COMMIT. Grant fail → tx rollback, no writes. Audit fail → tx rollback, ledger + wallet delta dropped. Post-commit audit events (trial_granted, sepay_success) remain best-effort with warn-log on failure |
| SePay token rotation → silent payment loss (webhooks 200 success=false unnoticed) | Low | Critical | **[round-3]** `auditFailAlertWatcher` goroutine polls audit_log every 60s; if ≥20 `sepay_auth_fail` rows in last 15min → admin alert via `adminAlertCh`. Bucket-dedup via `now.Unix() / (15*60)` prevents alert spam |
| `currval('ledger_id_seq')` returns wrong id if grant_credits triggers indirect inserts | Very Low | Med | Postgres `currval` is session-scoped to the sequence's last `nextval` call in the current session. `grant_credits` stored proc does `INSERT INTO ledger ...` which increments ledger_id_seq via DEFAULT — guaranteed to be the last `nextval` in this session before the `SELECT currval`. Safe per PG spec |
| Watcher fires during clock skew / NTP sync | Very Low | Low | Window bucket = `now.Unix() / 900`; GC retains buckets within `[bucket-4, bucket]` — tolerates ±1hr clock jitter before accidental re-alert |
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
