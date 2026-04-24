# Phase 02 — User Service + `/start` Flow

## Context Links
- Spec: `docs/MASTER_PROMPT.md` §5.1 `/start`, §5.5 trial gate, §3.1 users table
- Research: `./research/research-01-library-and-concurrency-decisions.md` §6 (trial gate)
- Existing: `services/api/internal/db/sqlc/users.sql.go` (placeholder)

## Overview
- **Priority:** P1 (gate for key + trial credits)
- **Status:** pending
- **Description:** user_service handles: upsert on Telegram contact, phone verify, trial grant (5 Standard credits). `/start` command runs multi-step FSM: welcome → request_contact → verify+grant.

## Key Insights
- MASTER_PROMPT §5.5 trial gate specifies Telegram account-age check, but Bot API doesn't expose creation date. Replace with phone-uniqueness gate (see research §6).
- Trial grant + `users.trial_used=TRUE` + wallet insert MUST be atomic in single Postgres txn.
- `users.phone_e164` stored normalized (+84..., strip spaces/dashes).
- New users get `wallets` row on `users` INSERT via stored proc `ensure_wallet()` OR inline INSERT in same txn (pick inline — simpler, no new proc).

## Requirements
### Functional
- On `/start` first-time: reply welcome + force `request_contact` keyboard → state=`awaiting_contact`.
- On contact received: validate phone, upsert user (INSERT ... ON CONFLICT (telegram_id) DO UPDATE SET phone_e164, is_verified=TRUE), create wallet if absent, run trial gate, grant 5 Standard credits + flip `trial_used`, state=`idle`.
- On `/start` repeat (already verified): show "already verified" message, remind of `/key` and `/balance`.
- On trial gate fail: reply reason-specific message (phone_reused / banned) — no credit grant, no ledger entry.

### Non-functional
- Entire flow idempotent: double-tap `/start` at awaiting_contact → same reply, no double grant.
- Phone normalized via `util/phone.go` (libphonenumber-style — use `github.com/nyaruka/phonenumbers` if VN default, fallback regex if not available).
- Ledger entry includes `metadata.source="telegram_bot_start"`.

## Architecture

### Data flow
```
/start → loadUser middleware (creates user row stub if missing, wallet stub) →
         UserService.Flow(ctx, user) →
            case user.IsVerified == false:
               state := awaiting_contact; send welcome + contact keyboard
            case user.IsVerified == true:
               clear state; send welcome_back (show key prefix, balance pointer)

[contact share event] → UserService.VerifyContact(ctx, user, phone) →
   tx := BEGIN
      UPDATE users SET phone_e164=$1, is_verified=TRUE WHERE id=$2
      gate := checkTrialGate(tx, user.ID, phone)
      if gate == OK && !user.TrialUsed:
         SELECT grant_credits(user.id, 'standard', 5, 'trial_grant', 'user', user.id)
         UPDATE users SET trial_used=TRUE WHERE id=$1
      COMMIT
   → send success message with key_prefix from api_keys (issued by phase 3 in same call)
```

### Trial gate (`user_service.go`)
```go
func checkTrialGate(tx pgx.Tx, userID UUID, phone string) TrialGateResult {
    // 1. Already used on this user
    if user.TrialUsed { return TrialAlreadyUsed }
    // 2. Banned
    if user.IsBanned { return TrialUserBanned }
    // 3. Phone reuse — UNIQUE partial index will throw; pre-check for friendlier error
    var count int
    tx.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE phone_e164=$1 AND trial_used=TRUE AND id != $2`, phone, userID).Scan(&count)
    if count > 0 { return TrialPhoneReused }
    return TrialOK
}
```

## Related Code Files
### Create
- `services/api/internal/service/user_service.go`
- `services/api/internal/util/phone.go` — normalize E.164 (VN-biased: `0987654321` → `+84987654321`)
- `services/api/internal/bot/commands/start.go`
- `services/api/internal/db/queries/users.sql` — real queries (replace placeholder)
- `services/api/internal/migrations/20260424002_phase2.sql` — adds unique indexes + support_tickets + referrals
- `services/api/internal/service/user_service_test.go`

### Modify
- `services/api/internal/db/sqlc/users.sql.go` — regenerate via `sqlc generate`
- `services/api/internal/bot/router.go` — route `/start` + `update.Message.Contact != nil`

## sqlc queries (new)
```sql
-- services/api/internal/db/queries/users.sql

-- name: GetUserByTelegramID :one
SELECT * FROM users WHERE telegram_id = $1;

-- name: UpsertUserStub :one
INSERT INTO users (telegram_id, telegram_username, language)
VALUES ($1, $2, 'vi')
ON CONFLICT (telegram_id) DO UPDATE SET
    telegram_username = EXCLUDED.telegram_username,
    last_active_at = NOW(),
    updated_at = NOW()
RETURNING *;

-- name: SetPhoneAndVerify :one
UPDATE users
SET phone_e164 = $2, is_verified = TRUE, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: MarkTrialUsed :exec
UPDATE users SET trial_used = TRUE, updated_at = NOW() WHERE id = $1;

-- name: CountOtherUsersWithPhoneTrialUsed :one
SELECT COUNT(*) FROM users
WHERE phone_e164 = $1 AND trial_used = TRUE AND id != $2;

-- name: EnsureWallet :exec
INSERT INTO wallets (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING;

-- name: SetLanguage :exec
UPDATE users SET language = $2, updated_at = NOW() WHERE id = $1;
```

## Migration `20260424002_phase2.sql` (shared with other phases — full file written once)
```sql
-- +goose Up
-- +goose NO TRANSACTION

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_trial
  ON users(phone_e164) WHERE trial_used = TRUE AND phone_e164 IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tx_user_pkg_pending
  ON transactions(user_id, package_code) WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS support_tickets (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject       VARCHAR(128),
    body          TEXT NOT NULL,
    status        VARCHAR(16) NOT NULL DEFAULT 'open' CHECK (status IN ('open','in_progress','resolved','closed')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_support_open ON support_tickets(status, created_at DESC) WHERE status IN ('open','in_progress');

CREATE TABLE IF NOT EXISTS referrals (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    code            VARCHAR(16) NOT NULL UNIQUE,
    total_referred  INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS referrals;
DROP TABLE IF EXISTS support_tickets;
DROP INDEX IF EXISTS idx_tx_user_pkg_pending;
DROP INDEX IF EXISTS idx_users_phone_trial;
```

## Implementation Steps
1. Write migration file `20260424002_phase2.sql` with both Up and Down sections.
2. Apply migration locally via `make migrate-up` and verify schema.
3. Write `db/queries/users.sql` queries; run `sqlc generate`.
4. Implement `util/phone.go` normalizer with VN default country; unit tests for common formats.
5. Implement `service/user_service.go`:
   - `EnsureStub(ctx, tgID, tgUsername) (User, error)` — called by loadUser middleware.
   - `Flow(ctx, user) (Reply, error)` — state machine for `/start`.
   - `VerifyContactAndGrantTrial(ctx, userID, rawPhone) (TrialResult, error)` — atomic txn.
6. Implement `bot/commands/start.go`:
   - Handle `/start` command and contact-update event.
   - Set/clear Redis FSM states via `bot.state.Store`.
   - Call `key_service.EnsureActiveKey()` after trial grant — but plaintext returned ONLY on first creation (phase 03 concern).
7. Write integration test `user_service_test.go` with testcontainers-go:
   - New user → `Flow` returns welcome + awaiting_contact.
   - Contact received → trial granted, wallet.standard_credits=5, ledger row inserted, trial_used=TRUE.
   - Second user with SAME phone → returns TrialPhoneReused, wallet stays 0.
   - Double-tap same trial → second call returns TrialAlreadyUsed.

## Todo List
- [ ] Write migration `20260424002_phase2.sql`
- [ ] Apply migration + verify indexes (`psql \d users`, `\d transactions`)
- [ ] Write `db/queries/users.sql`
- [ ] Run `sqlc generate`
- [ ] Implement `util/phone.go` + tests
- [ ] Implement `service/user_service.go`
- [ ] Implement `bot/commands/start.go`
- [ ] Wire router: `/start` + contact event
- [ ] Integration tests (testcontainers)
- [ ] Manual smoke: real Telegram → `/start` → share contact → credits granted

## Success Criteria
- `SELECT COUNT(*) FROM wallets WHERE user_id=$1` = 1 after `/start` completion
- `SELECT standard_credits FROM wallets WHERE user_id=$1` = 5 after trial
- `SELECT trial_used FROM users WHERE id=$1` = TRUE
- Duplicate phone → second user blocked, wallet untouched
- `go test -race` passes

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Phone normalizer eats valid phones | Med | High | Unit-test 15+ VN formats; fallback to rawPhone if unparseable, log warning |
| Partial txn commit (user verified but trial not granted) | Low | High | Whole flow in single `pgx.Tx` — no partial commits possible |
| Race: 2 updates for same user hit trial grant | Med | High | `singleflight` in phase 01 + `trial_used` write-once check INSIDE txn |
| Phone reuse index throws `23505` at runtime | Low | Med | Pre-check via `CountOtherUsersWithPhoneTrialUsed` + treat 23505 as reuse (defense in depth) |
| Contact event fires without `/start` precedent | Low | Low | FSM state guard: accept contact ONLY if state=awaiting_contact, else ignore |

## Security Considerations
- `phone_e164` is PII — DO NOT log full phone; log hash prefix `sha256(phone)[:8]`.
- Reject contact shares not matching `from.id == contact.user_id` (user spoofing someone else's contact).
- Normalize phone BEFORE DB insert to avoid `0987654321` and `+84987654321` as different rows.
- Audit log every trial grant: event=`trial_granted`, metadata={source, phone_hash, ip_hash_null_for_bot}.

## Next Steps
- Phase 03 Key Service: issue plaintext key + store hash, called from within trial-grant txn.
- Phase 04 Wallet Service: `/balance` command reads the wallet row populated here.
