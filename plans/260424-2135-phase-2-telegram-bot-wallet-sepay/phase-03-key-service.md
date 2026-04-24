# Phase 03 — Key Service + `/key` + `/regenkey`

## Context Links
- Spec: `docs/MASTER_PROMPT.md` §1.3 Key model, §1.6 key rotation, §5.1 commands
- Existing: `services/api/internal/db/sqlc/keys.sql.go` (placeholder)

## Overview
- **Priority:** P1
- **Status:** pending
- **Description:** Generate `sbf_live_<32-char-base58>` keys. Store SHA256 hash. Plaintext shown once on issue or regen. `/regenkey` revokes current + issues new atomically.

## Key Insights
- 32-char base58 encoding → ~187 bits entropy. Use `crypto/rand` directly, not math/rand.
- Full key length: `sbf_live_` (9) + 32 = 41 chars (NOT 46 as spec §1.3 says — 46 would need 37 random chars; master prompt is inconsistent. Lock 41 as our standard, document in code comment).
- `key_prefix` stored = first 12 chars of plaintext (e.g., `sbf_live_Zk3`) for display masking.
- Base58 alphabet excludes `0OIl` to prevent eye-confusion.
- Key rotation: `UPDATE api_keys SET is_active=FALSE, revoked_at=NOW() WHERE user_id=$1 AND is_active=TRUE` → `INSERT new`. Both in single txn. After commit: Redis SET `key_revoked:<old_hash_hex>` TTL 24h to drain pending requests (phase 3 extension).

## Requirements
### Functional
- `KeyService.Generate() (plaintext, hash, prefix)` — crypto-secure random.
- `KeyService.Issue(ctx, userID) (plaintext, KeyRow, error)` — revokes old active + inserts new, single txn.
- `KeyService.GetActiveMask(ctx, userID) (maskedDisplay, error)` — shows `sbf_live_Zk3p•••••dVo5` (prefix + last 4 hash chars — last 4 not of plaintext since we don't store it).
- `/key` — show masked + inline buttons Copy (copies key_prefix only — plaintext not retrievable) and Regenerate (triggers /regenkey).
- `/regenkey` — FSM: idle → buy_confirming-like confirmation keyboard Yes/Cancel → on Yes: `Issue()` → display plaintext ONCE with warning.
- **[H5] Rate limit `/regenkey`** to 3/day per user via Redis `INCR regen_rl:<user_id>` with `EXPIRE 86400`. On count > 3 → reply `regen_rate_limited` template (do NOT call `Issue`). Check runs at command dispatch BEFORE FSM confirmation prompt.

### Non-functional
- Hash via `crypto/sha256`, NOT `bcrypt` (we need O(1) lookup by hash, not verification — keys are high-entropy already).
- Lookup index `idx_keys_hash` already exists on `api_keys(key_hash) WHERE is_active=TRUE`.
- Plaintext NEVER stored, NEVER logged, NEVER returned via GET.

## Architecture

### Key generation (`util/token.go`)
```go
const (
    KeyPrefix = "sbf_live_"
    KeyRandomLen = 32 // base58 chars
    base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
)

func GenerateAPIKey() (plaintext string, hashBytes []byte, prefix string, err error) {
    buf := make([]byte, 24) // 24 bytes → 32 base58 chars avg
    if _, err := rand.Read(buf); err != nil { return }
    encoded := base58Encode(buf, KeyRandomLen)
    plaintext = KeyPrefix + encoded
    sum := sha256.Sum256([]byte(plaintext))
    return plaintext, sum[:], plaintext[:12], nil
}
```

### Issue flow (`key_service.go`)
```go
func (s *KeyService) Issue(ctx, userID UUID) (plaintext string, k KeyRow, err error) {
    plaintext, hash, prefix, err := util.GenerateAPIKey()
    if err != nil { return }
    tx, err := s.pool.Begin(ctx); defer tx.Rollback(ctx)
    // revoke any active
    _, err = tx.Exec(ctx, `UPDATE api_keys SET is_active=FALSE, revoked_at=NOW() WHERE user_id=$1 AND is_active=TRUE`, userID)
    if err != nil { return }
    // insert new
    row := tx.QueryRow(ctx, `
        INSERT INTO api_keys (user_id, key_hash, key_prefix, is_active, name)
        VALUES ($1, $2, $3, TRUE, 'primary') RETURNING *`, userID, hash, prefix)
    // scan into k ...
    err = tx.Commit(ctx)
    // audit log (post-commit, best effort)
    go s.audit.Log(ctx, userID, "key_issued", map[string]any{"key_prefix": prefix})
    return
}
```

## Related Code Files
### Create
- `services/api/internal/util/token.go` — key gen + base58 encode
- `services/api/internal/util/token_test.go`
- `services/api/internal/service/key_service.go`
- `services/api/internal/service/key_service_test.go`
- `services/api/internal/bot/commands/key.go`
- `services/api/internal/bot/commands/regenkey.go`
- `services/api/internal/db/queries/keys.sql`

### Modify
- `services/api/internal/db/sqlc/keys.sql.go` (regenerated)
- `services/api/internal/bot/router.go` — route `/key`, `/regenkey`, callback `key:regen:confirm|cancel`

## sqlc queries
```sql
-- services/api/internal/db/queries/keys.sql

-- name: GetActiveKeyByUser :one
SELECT * FROM api_keys WHERE user_id = $1 AND is_active = TRUE LIMIT 1;

-- name: RevokeActiveKeysForUser :exec
UPDATE api_keys SET is_active = FALSE, revoked_at = NOW()
WHERE user_id = $1 AND is_active = TRUE;

-- name: InsertKey :one
INSERT INTO api_keys (user_id, key_hash, key_prefix, name, is_active)
VALUES ($1, $2, $3, $4, TRUE)
RETURNING *;

-- name: GetKeyByHash :one
SELECT * FROM api_keys WHERE key_hash = $1 AND is_active = TRUE LIMIT 1;
```

## Implementation Steps
1. Write `util/token.go` with base58 encode + key gen. Test entropy + uniqueness across 10k samples.
2. Write `db/queries/keys.sql`; run `sqlc generate`.
3. Write `service/key_service.go` with `Issue`, `GetActiveMasked`, `VerifyAndResolveUser(plaintextHeader)`.
4. Write `bot/commands/key.go`:
   - Query `GetActiveKeyByUser`. If none → call `Issue` (covers edge case where /start trial granted but key gen failed mid-flow).
   - Reply: masked prefix + inline keyboard [Copy prefix] [Regenerate].
5. Write `bot/commands/regenkey.go`:
   - **[H5]** At command entry: `INCR regen_rl:<user_id>` pipelined with `EXPIRE 86400 NX`. If value > 3 → reply template `regen_rate_limited`, return early (no FSM transition, no Issue call).
   - On `/regenkey` command (within rate): reply confirm prompt with callback `key:regen:confirm|cancel`.
   - On callback `confirm`: call `Issue`, reply plaintext with bold warning "Save this NOW — won't show again".
   - On callback `cancel`: edit message to "cancelled".
6. Hook phase 02 trial-grant flow to call `key_service.Issue` AFTER the trial-grant txn commits (not inside — keys are not financial state).
7. Write unit test for base58 encoding + key uniqueness.
8. Write integration test: issue key → revoke old → old hash no longer `is_active`.
9. **[H5]** Integration test: 4th `/regenkey` in same day → rate-limited reply, 3rd-key active row unchanged, Redis key expires after 86400s.

## Todo List
- [ ] Implement `util/token.go` + tests (entropy, base58 correctness)
- [ ] Write `db/queries/keys.sql`
- [ ] Run `sqlc generate`
- [ ] Implement `key_service.go` with Issue/Resolve
- [ ] Implement `bot/commands/key.go`
- [ ] **[H5]** Implement `bot/commands/regenkey.go` with Redis rate-limit (`regen_rl:<user_id>` INCR + EXPIRE 86400, cap 3/day) + confirmation FSM
- [ ] Wire router + callbacks
- [ ] Unit tests for key gen
- [ ] Integration test for rotation (old revoked, new active)
- [ ] **[H5]** Integration test: 4th /regenkey rate-limited reply
- [ ] Verify no plaintext in logs (grep zap output)

## Success Criteria
- `/key` returns masked display, never plaintext
- `/regenkey confirm` returns plaintext, old key `is_active=FALSE revoked_at!=NULL`
- SHA256(plaintext) matches stored `key_hash`
- 10k generated keys have 0 collisions in test
- No plaintext in log output

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| `rand.Read` fails on broken entropy source | Very Low | Critical | Propagate error, refuse issue; log + alert |
| Base58 encoder off-by-one | Low | High | Unit test round-trip on 1000 samples |
| Old key still valid in cached request after rotation | Med | Med | Phase 3 will add Redis blocklist `key_revoked:<hash>` TTL 24h (deferred scope note here) |
| Plaintext leaks via zap structured field | Med | Critical | Lint rule: forbid `zap.String("plaintext_key"…)`; use only `zap.String("key_prefix", k.KeyPrefix)` |
| Regen spammed → DB bloat in audit | Low | Low | **[H5] Implemented:** Rate-limit `/regenkey` in Redis `regen_rl:<user_id>` — 3/day cap, short-circuit before FSM |

## Security Considerations
- `crypto/rand` (not math/rand).
- `crypto/subtle.ConstantTimeCompare` used when comparing key-derived secrets (Phase 3).
- Audit log `key_issued` + `key_revoked` — immutable record for compliance.
- `/regenkey` requires FSM confirmation to prevent accidental revocation.
- `Copy` button in Telegram copies only `key_prefix`, not full plaintext — full plaintext already shown in initial reply; reclick = intentional re-view.

## Next Steps
- Phase 02 calls `Issue` after trial grant.
- Phase 3 (extension integration) adds auth middleware using `GetKeyByHash`.
