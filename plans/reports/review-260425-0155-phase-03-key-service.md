# Code Review — Phase 03 Key Service

## Verdict
**FIX_REQUIRED**

## Summary
1 critical race (no UNIQUE partial index → concurrent Issue can leave 2 active keys per user). Key gen crypto+base58 correct. Logging clean (no plaintext leak). Rate-limit H5 pipelined correctly but fail-open by design. One medium bug: `key:regen` inline button dispatches to `HandleRegenKey` which calls `updateChatID` on a CallbackQuery whose `.Message` may be nil (stale/inaccessible). One medium: missing /regenkey H5 rate-limit integration test.

---

## Critical findings

### C1 — Concurrent `Issue` can produce 2 active keys per user (race)
- **Where:** `service/key_service.go:46-91` (Issue) + `migrations/20260424001_init.sql:38` (`idx_keys_user` is NOT UNIQUE).
- **Bug:** The "exactly 1 active key" invariant is enforced only in application code via `RevokeActiveKeysForUser` then `InsertKey` inside a tx. There is no DB-level exclusion:
  - `idx_keys_user ON api_keys(user_id) WHERE is_active = TRUE` is a plain index (line 38), not UNIQUE.
  - `key_hash BYTEA NOT NULL UNIQUE` (line 30) protects only against same-hash duplication — concurrent Issue calls generate DIFFERENT random hashes, so this does not apply.
- **Race trace (2 concurrent `Issue(userID)` calls, READ COMMITTED):**
  1. G1 begins tx, G2 begins tx.
  2. G1: `UPDATE ... WHERE user_id=X AND is_active=TRUE` → 0 rows affected (or 1 row, locked by G1).
  3. G2: `UPDATE ... WHERE user_id=X AND is_active=TRUE` → 0 rows affected (empty predicate, no lock) OR blocks on G1's row, then after G1 commit re-evaluates and finds 0 rows (row now inactive).
  4. G1: INSERT new row is_active=TRUE → OK.
  5. G2: INSERT new row is_active=TRUE → OK (distinct hash).
  6. Both commit → **2 active keys for user X**.
- **Why `TestIssue_ConcurrentForSameUser` passes today:** the test uses a fresh user with no prior active key. Both goroutines race at InsertKey speed; in practice on a single pgx pool with 2 goroutines, one tx typically commits before the other's INSERT finishes, leaving only 1 active row. But this is **timing-dependent, not enforced** — under higher load or different scheduling it will fail. The test does not `require.Equal(t, 1, ...)` under adversarial scheduling; it passes by luck.
- **Impact:** Violates spec §1.3 "1 active key per user". Downstream `GetActiveKeyByUser` uses `LIMIT 1` so the "wrong" key can be returned; audit integrity compromised; user can accidentally use a key they thought was revoked.
- **Fix options (pick one):**
  1. **DB-level (preferred):** Add partial UNIQUE index migration:
     ```sql
     CREATE UNIQUE INDEX idx_keys_user_active
       ON api_keys(user_id) WHERE is_active = TRUE;
     -- Drop old non-unique idx_keys_user or keep alongside.
     ```
     Then handle SQLSTATE 23505 in `Issue` → retry once or return a sentinel.
  2. **Application-level:** Row-lock the user row at start of tx: `SELECT 1 FROM users WHERE id=$1 FOR UPDATE` — serialises concurrent Issue per user.
- **Severity:** CRITICAL — correctness + spec violation.

---

## High findings

### H1 — `HandleRegenKey` called from callback path with nil `CallbackQuery.Message` possible
- **Where:** `bot/router.go:81-83` dispatches `"key:regen"` → `HandleRegenKey(ctx, deps, api, update)`.
  `HandleRegenKey` (cmd_regenkey.go:47) calls `updateChatID(update)` which, for CallbackQuery path, reads `u.CallbackQuery.Message.Chat.ID`. `update_helpers.go:64-66` nil-checks `.Message`, but if nil returns 0. Subsequent `tgbotapi.NewMessage(0, ...)` will hit Telegram's Bad Request.
- `HandleRegenKey` does not answer the callback query first, so the user also sees a stuck spinner on stale messages.
- **Impact:** Stale inline-keyboard button click → silent failure or spinner lock, possible Telegram API error noise.
- **Fix:** In router dispatch for `"key:regen"` only, prepend:
  ```go
  if update.CallbackQuery != nil {
      cb := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
      _, _ = api.Request(cb)
      if update.CallbackQuery.Message == nil {
          return nil // stale message — cannot reply
      }
  }
  ```
  Then call HandleRegenKey.

### H2 — Double-count risk via `/key` Regenerate button (confirmed, low user impact but counts)
- Deviation note #3 from implementer: `/key`'s "🔄 Regenerate" inline button calls `HandleRegenKey`, which re-runs rate-limit INCR.
- **Trace:** User types `/regenkey` (counter = 1) → clicks Cancel (counter stays 1). User opens `/key` → clicks "🔄 Regenerate" (counter = 2) → clicks Cancel again (stays 2). User repeats once more → counter = 3. Now user's actual successful regen count is ZERO, but next attempt is blocked.
- **Attack variant:** Attacker rapid-clicks the "🔄 Regenerate" inline button 4x without ever confirming → counter=4, legitimate /regenkey subsequently blocked.
- **Verdict:** Intended but leaky semantics per implementer. Acceptable as a conservative rate-limit, but not ideal. Document in user-facing help text, or increment counter only at CONFIRM click (after Issue succeeds). Latter is cleaner.
- **Suggested fix:** move INCR from HandleRegenKey → handleRegenConfirmCallback, after successful `deps.KeyService.Issue` returns nil err. Rate-limit then tracks actual rotations, not click-intent.

---

## Medium findings

### M1 — Missing H5 rate-limit integration test
- Plan §Implementation Steps item 9 explicitly requires: "Integration test: 4th /regenkey in same day → rate-limited reply, 3rd-key active row unchanged, Redis key expires after 86400s."
- Neither `cmd_regenkey_test.go` nor any other file implements this.
- Only unit coverage of `checkRegenRateLimit` would suffice; current coverage = 0.

### M2 — 33-digit base58 encoding truncation drops MSD in ~96% of keys
- `util/token.go:91-93`: when natural encoding of 24 random bytes produces 33 chars (happens whenever value ≥ 58^32 ≈ 2^187.47, i.e. ~96% of cases), the code takes the rightmost 32 chars, dropping the most-significant digit.
- Net entropy: still ≥ 187 bits — cryptographically safe, no exploit.
- But it's a **distributional flaw**: the leading base58 digit of each plaintext is skewed because we silently collapse ~2^192 values into 58^32 buckets via modulo-like truncation. Not a real attack, but the code ignores 4.5 bits of entropy pointlessly.
- **Cleaner fix:** make `randByteLen` smaller (e.g., 23 bytes = 184 bits; always fits in 32 base58 digits without truncation), OR use 24 bytes and keep all 33 digits (expand key to 33 chars).
- **Severity:** MEDIUM cosmetic; not blocking.

### M3 — Audit row has `key_id = NULL`
- `key_service.go:118-121` inserts into `audit_log` with only `user_id`, `event`, `metadata`. `InsertKey` returns the row with ID available — populate `key_id` for forensic traceability.
- **Fix:** change Issue to capture row from InsertKey (already does: ignored at line 67 `_, insertErr := ...`) and pass `row.ID` to `insertAuditLog`.

### M4 — `handleRegenConfirmCallback` does not revalidate FSM state
- `cmd_regenkey.go:117-122` clears FSM state after checking user exists, but never VERIFIES the FSM was in `stateAwaitingRegenConfirm`. A stale callback (e.g. bot restarted, state expired) could still trigger Issue with no confirmation path.
- **Impact:** low — the callback inherently requires a past `/regenkey` click, which shows the confirmation UI. But an attacker with harvested callback data could trigger regen without the FSM ever having been set.
- **Fix:** Load state via `stateStore.Load`; if state.Name != stateAwaitingRegenConfirm → reply "Phiên xác nhận đã hết hạn" and bail.

---

## Low / nits

### L1 — `masked` rune length test in `TestGetActiveMasked_Exists`
- `key_service_test.go:216`: `if len([]rune(masked)) != 21` — correct (12 prefix + 5 bullets + 4 suffix). Good.

### L2 — `GenerateAPIKey` named returns shadowed
- `token.go:48`: `func GenerateAPIKey() (plaintext string, hashBytes []byte, prefix string, err error)`. Line 50: `if _, err = rand.Read(buf)` uses named `err`; line 60: `prefix = plaintext[:KeyPrefixLen]` uses named `prefix`. Named returns OK here — minor readability; could inline returns.

### L3 — Defer rollback errcheck suppression
- `key_service.go:56`: `defer tx.Rollback(ctx) //nolint:errcheck` — acceptable pattern. OK.

### L4 — `insertAuditLog` goroutine leaks on shutdown
- Launched via `go s.insertAuditLog(context.Background(), ...)` from Issue. Uses Background ctx (deliberate; survives handler ctx cancellation). But if pool closes before goroutine reaches Exec → pgx returns `pool closed` error, logged as Warn. Non-critical.

### L5 — `handleKeyCopyPrefixCallback` `prefix` extraction loop is O(n)
- `cmd_key.go:97-104`: loops over runes to find '•'. Masked string is 21 chars — trivial. The prefix is also stored on the DB row; could pass it directly instead of re-parsing the masked display. Nit.

---

## Key correctness audit

- [x] **crypto/rand usage** — `token.go:50` uses `crypto/rand.Read`. 0 matches for `math/rand` in util.
- [x] **Base58 alphabet + length** — 58 chars, excludes `0OIl`, verified by `TestBase58_AlphabetExcludes0OIl`. Alphabet string at `token.go:30` counts to 58.
- [x] **Plaintext 41 chars** — asserted by `TestGenerateAPIKey_Shape` (line 23-25), `TestGenerateAPIKey_RandomPartLength` (line 135).
- [x] **Hash computation** — `sha256.Sum256([]byte(plaintext))` at `token.go:57`; verified by `TestGenerateAPIKey_HashCorrectness` (comparing to re-computed SHA-256).
- [x] **Prefix 12 chars** — `plaintext[:KeyPrefixLen]` where `KeyPrefixLen=12`; test at `token_test.go:29-31, 101`.
- [x] **No plaintext in logs/audit** — grep `zap.*plaintext` finds only comments. Audit metadata = `{"key_prefix": ...}` only (key_service.go:117).

---

## H5 rate limit audit

- [x] **INCR + EXPIRE pipelined atomic** — `cmd_regenkey.go:193-197`. Redis pipeline is atomic on a single connection.
- [x] **3/day cap enforced** — `regenRateLimitPerDay = 3`; check `count > regenRateLimitPerDay` at line 207.
- [x] **Short-circuit before FSM** — lines 56-68 run rate-limit check first; FSM save at line 71-79 is BELOW.
- [~] **Double-count risk via /key Regenerate button** — CONFIRMED, see H2. Counter increments on every HandleRegenKey entry, not only on confirmed rotation.
- [-] **Fail mode:** Redis unavailable → `deps.Rdb != nil` guard skips the whole block → fails OPEN. If Rdb is non-nil but returns pipeline error → `limited=false` → fails OPEN too (line 60-62). This matches "lenient boot" policy but means an attacker who DoS Redis can bypass rate limit. Document this is intentional.

---

## Issue atomicity audit

- [x] **RevokeActiveKeysForUser + InsertKey same tx** — `key_service.go:52-79`. Both run through `qtx := sqlcdb.New(tx)` and commit together.
- [ ] **Concurrent Issue → exactly 1 active** — FAIL (C1). No DB-level exclusion; application-only invariant is race-vulnerable.
- [x] **revoke_at timestamp correct** — `UPDATE ... SET is_active=FALSE, revoked_at=NOW()` (queries/keys.sql:9-10). Test `TestIssue_RevokesOldActive` asserts `revoked_at IS NOT NULL` (key_service_test.go:143).

---

## Plan spec alignment

| Spec | Status | Note |
|------|--------|------|
| `sbf_live_<32-char-base58>` | ✓ | 41 char total locked. Comment at token.go:6-9 documents deviation from MASTER_PROMPT §1.3's 46. |
| crypto-secure random | ✓ | crypto/rand only. |
| base58 excludes 0OIl | ✓ | Alphabet at token.go:30. |
| Issue revokes old + inserts new, single tx | ✓ (logically) | See C1 for concurrency gap. |
| GetActiveMask shows prefix + ••••• + 4 last chars | ✓ | Uses last 4 hex of hash[:4], not plaintext — matches spec line 23 "4 hash chars since we don't store plaintext". |
| `/key` masked + [Copy prefix / Regenerate] inline | ✓ | cmd_key.go:52-57. |
| `/regenkey` FSM confirm → Issue → plaintext ONCE | ✓ | cmd_regenkey.go. FSM state not revalidated on confirm callback — see M4. |
| [H5] 3/day via Redis INCR+EXPIRE 86400 | ✓ | cmd_regenkey.go:189-208. |
| Phase 02 → Phase 03 wire real KeyService | ✓ | main.go:68-83 wires keySvc → UserService and bot.Deps. NoopKeyIssuer retired. |
| Plaintext never logged | ✓ | 0 matches in grep. |
| UNIQUE constraint on (user_id, is_active=TRUE) | ✗ | Missing. C1. |

---

## Test adequacy

### Present
- 7 util/token tests: shape, uniqueness (10k), hash correctness, entropy, prefix, alphabet, random-part length.
- 6 service/key_service tests: NewKey, RevokesOldActive, HashMatchesPlaintext, GetActiveMasked_Exists, GetActiveMasked_NotExists, ConcurrentForSameUser.

### Gaps
- **M1 — no H5 rate-limit integration test.** Plan item 9 explicitly required.
- **TestIssue_ConcurrentForSameUser is flimsy** (passes by timing luck with 2 goroutines). Should ramp to 10 goroutines + retry loop + assert either strict-1-active OR expected error. Currently masks C1.
- **No test for GetActiveMasked on user with revoked key** — edge case where revoked row exists; query filters correctly by `is_active=TRUE` but not exercised.
- **No test for handleRegenConfirmCallback** end-to-end (FSM state, Issue, reply).

---

## Approved items
- `util/token.go` key generation: math/crypto/entropy/encoding are correct and well-documented.
- Plaintext leak hygiene: zero zap/audit leaks confirmed by grep and code audit.
- Transactional revoke+insert pattern: sound application logic, just missing DB-level backstop.
- FSM TTL + regen confirmation ergonomics.
- Router callback dispatch table is clean; `handleUnknownCallback` safely ignores stray data.
- `GetActiveMasked` masked format: 12 + 5 bullets + 4 hex = 21 runes, no plaintext entropy leak.
- Deviation note #2 (KeyService as concrete `*KeyService`) is structurally fine — `*KeyService` still satisfies `KeyIssuer` interface for UserService.
- Best-effort audit log on post-commit goroutine with Background ctx is correct.

---

## Recommended actions

1. **[C1 BLOCKING]** Add migration `20260424004_keys_unique_active.sql`:
   ```sql
   CREATE UNIQUE INDEX IF NOT EXISTS idx_keys_user_active
       ON api_keys(user_id) WHERE is_active = TRUE;
   ```
   Update `Issue` to catch SQLSTATE 23505 on this new index → retry once (the race is already a user-visible edge case so retry is legitimate) or return `ErrKeyIssueRaced` sentinel.
2. **[H1]** Answer callback + nil-check CallbackQuery.Message before dispatching `"key:regen"` in router.
3. **[H2]** Move rate-limit INCR from `HandleRegenKey` to `handleRegenConfirmCallback` after successful Issue — counter tracks actual rotations, not intent.
4. **[M1]** Add H5 rate-limit integration test: 4 consecutive HandleRegenKey calls, 4th returns rate-limited reply, verify no new api_key row inserted.
5. **[M3]** Populate audit_log.key_id with `row.ID` from InsertKey.
6. **[M4]** Revalidate FSM state in `handleRegenConfirmCallback` before issuing.
7. **[M2]** (non-blocking) Either reduce `randByteLen` to 23 or expand output to 33 chars to avoid MSD truncation.

---

## Status block
```
**Status:** DONE
**Verdict:** FIX_REQUIRED
**Summary:** 1 crit / 2 high / 4 med / 5 low
**Report:** E:\tool_backlink\plans\reports\review-260425-0155-phase-03-key-service.md
**Commit sha:** [pending commit]
**Next:** fix-loop
```

---

## Unresolved questions
1. Is the partial-UNIQUE index the right fix, or does the team prefer `SELECT ... FOR UPDATE` on users row? DB-level is simpler but couples migrations to code.
2. Should the H5 rate-limit be "click-intent" (current) or "successful-rotation" (H2 suggestion)? Semantic choice — please confirm before changing.
3. Is the MASTER_PROMPT §1.3 46-char spec going to be updated to match the code's 41-char, or is this drift left as-is with the comment block?
