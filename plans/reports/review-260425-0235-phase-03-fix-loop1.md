# Code Review — Phase 03 Key Service (fix loop 1 verify)

## Verdict
**APPROVED_WITH_MINOR**

Code-level fixes for all 7 prior findings are correctly implemented. Concurrent race test passes 10/10 iterations with exactly 1 active row remaining. All regression guards (plaintext/math-rand/singleflight/base58) clean. Build + go vet + full test suite green.

One operational concern flagged (N1): the unique partial index in the dev DB is currently `INVALID` — dev-DB state issue, not a code defect. Production boot on a fresh DB will create a valid index and behavior will match spec.

---

## Prior findings closure

| ID | Status | Where | Evidence |
|---|---|---|---|
| C1 (race) | **CLOSED (code)** / **OPEN (dev-DB state)** | migrations/20260425001_key_unique_active.sql + service/key_service.go:62-97 | Code correct: Serializable iso + 23505/40001 retry + partial UNIQUE migration. Dev DB index is INVALID (see N1). |
| H1 (nil-check) | CLOSED | bot/router.go:79-108 | `guardCallback` runs in `dispatchCallback` before switch; all 4 callback entries guarded uniformly. |
| H2 (INCR) | CLOSED | bot/cmd_regenkey.go:252-282 | `peekRegenRateLimit` (GET-only) separated from `incrRegenRateLimit` (INCR+EXPIRE pipelined). Key renamed `regenkey_rate_limit:<uid>`. INCR fires only post-Issue-success. |
| M1 (4th blocked test) | CLOSED | bot/cmd_regenkey_ratelimit_test.go:39-132 | 3 tests: FourthCallBlocked (miniredis FastForward), PeekDoesNotIncrement, KeyNaming. |
| M2 (base58 entropy) | CLOSED | util/token.go:74-104 + token_test.go:145-160 | MSB-side padding, LSB-only truncation. 50k-sample length regression test passes. |
| M3 (audit key_id) | CLOSED | service/key_service.go:74,167-180 + audit_log query | Raw pgx.Exec with 4 positional params matches schema columns. Verified populated in dev DB (100/100 post-fix rows have key_id). |
| M4 (FSM revalidate) | CLOSED | bot/cmd_regenkey.go:125-135, 224-232 | Both confirm and cancel handlers Load state + reject on mismatch. Clear after success, on cancel, and on error branches. |

---

## New findings (regressions / residual concerns)

### N1 — dev DB has `idx_keys_user_active_unique` in INVALID state (operational — not a code defect)
- **Severity:** MINOR (operational)
- **Where:** Postgres cluster `sbf_dev`, not source code
- **Evidence:**
  - `\d api_keys` shows `"idx_keys_user_active_unique" UNIQUE, btree (user_id) WHERE is_active = true INVALID`
  - `pg_index.indisvalid = f`
  - Empirical INSERT test succeeded in creating 2 active rows for one user via raw SQL (no 23505 raised)
- **Root cause:** `CREATE UNIQUE INDEX CONCURRENTLY` fails silently and leaves INVALID index if duplicate active rows exist at migration time. The implementer noted prior test iterations had left duplicate rows; the migration ran but did not complete the build.
- **Why concurrent test still passes:** Serializable isolation + 40001 retry handles it at application layer. DB-level enforcement is defense-in-depth; currently disabled in dev.
- **Production impact:** None if the cluster starts with clean data (no prior duplicates when migration runs). The planner explicitly accepted this tradeoff.
- **Dev/CI impact:** Future test runs that produce race contention under Read Committed would silently produce duplicate active rows — currently nothing catches this.
- **Recommended ops fix (one-off):**
  ```sql
  -- After ensuring no duplicates exist:
  DROP INDEX IF EXISTS idx_keys_user_active_unique;
  CREATE UNIQUE INDEX CONCURRENTLY idx_keys_user_active_unique
    ON api_keys(user_id) WHERE is_active = TRUE;
  ```
- **Long-term:** Consider whether CONCURRENTLY is worth the failure-mode risk for a small table. A plain `CREATE UNIQUE INDEX` in a transactional migration would fail hard and keep DB consistent — easier to debug. Acceptable either way.

### N2 — Rate-limit ceiling can be exceeded by ±1 under concurrent double-click
- **Severity:** LOW (non-security; rate-limit not money-flow)
- **Where:** bot/cmd_regenkey.go — `handleRegenConfirmCallback` PEEK (146) → Issue (162) → INCR (177)
- **Race trace:** User double-taps Confirm in <10ms:
  1. Callback A: PEEK counter=2 → passes gate
  2. Callback B: PEEK counter=2 → passes gate
  3. A calls Issue → succeeds → INCR → counter=3
  4. B calls Issue → wins race retry → succeeds → INCR → counter=4
- **Impact:** User can rotate at most 4 times in rare double-click case vs spec's 3. Not a security hole — both rotations actually happened (no phantom state).
- **Fix option (if tightening desired):** Use Redis `INCR` as the gate (atomic read-and-increment), rollback via `DECR` on Issue failure. Spec acceptability: fine as-is per user directive "PEEK/INCR separated".

### N3 — Cancel handler silently returns on Redis error
- **Severity:** LOW (UX)
- **Where:** bot/cmd_regenkey.go:226-232
- **Behavior:** If `stateStore.Load` errors (Redis transient failure), the cancel callback returns `nil` without editing the message or sending feedback. User sees the Cancel button answered (spinner clears via line 220) but no visible response.
- **Suggested:** On state load error, still attempt the message edit to "Đã huỷ." — state-loss is acceptable since cancel is idempotent.

### N4 — Audit insert race against graceful shutdown
- **Severity:** INFO (not a bug)
- **Where:** service/key_service.go:74 — `go s.insertAuditLog(context.Background(), ...)`
- Uses `context.Background()` intentionally (caller ctx may cancel). On graceful shutdown, goroutine can run after `pool.Close()`, audit insert fails, warning logged and swallowed. No audit integrity violation because the key issuance log at line 69 still fires synchronously before the goroutine is spawned.
- **No action needed** — documented behavior.

---

## Approved items

### Concurrency (C1 code-path)
- Serializable isolation explicit in `pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})` ✓
- Retry loop handles BOTH 23505 AND 40001 ✓
- `maxRetries = 1` per spec ✓
- Fresh plaintext regenerated on retry (confirmed: `util.GenerateAPIKey()` called inside `issueOnce` each attempt) ✓
- Audit goroutine fires only on final success (line 74 inside `err == nil` block, not in retry path) ✓
- Concurrent test passed 10 iterations — exactly 1 active row, ≥3 successes, 0 orphan revoked-with-NULL-revoked_at rows

### H1 (nil-check)
- `guardCallback` defined once in router.go ✓
- Applied centrally in `dispatchCallback` before switch ✓
- All 4 callback data strings covered ✓
- `handleUnknownCallback` retains its own internal guard as secondary defense ✓

### H2 (PEEK/INCR separation)
- `peekRegenRateLimit`: only `GET`, handles `goredis.Nil` → returns not-limited ✓
- `incrRegenRateLimit`: `INCR` + `EXPIRE` pipelined atomically ✓
- `HandleRegenKey` calls PEEK, never INCR ✓
- `handleRegenConfirmCallback`: PEEK pre-Issue, INCR post-Issue-success ✓
- `handleRegenCancelCallback`: no rate-limit ops ✓
- Search `regen_rl:` returns only comment reference (1 match) ✓

### M1 (4th call blocked test)
- `TestRegenRateLimit_FourthCallBlocked`: seeds counter via real INCR → 4th PEEK returns true → TTL verified ~86400s → FastForward 25h → 5th PEEK returns false ✓
- `TestRegenRateLimit_PeekDoesNotIncrement`: 10 peeks create 0 Redis keys ✓
- `TestRegenRateLimit_KeyNaming`: asserts exact key format ✓

### M2 (base58 entropy)
- `math/big.Int` for bigint ops ✓
- MSB-side padding with `alphabet[0]` ✓
- LSB-only truncation when natural encoding >32 digits ✓
- Documented entropy tradeoff (≥181 bits preserved) in code comments ✓
- 50k-sample length test passes ✓
- `math/rand` imported: grep returned 0 matches ✓
- Alphabet excludes `0OIl`: test passes ✓

### M3 (audit key_id)
- `issueOnce` captures `newKey` from `InsertKey` return ✓
- `insertAuditLog(ctx, userID, newKey.ID, prefix)` — keyID plumbed through ✓
- Raw `pool.Exec` with positional params `$1..$4` — no injection ✓
- Empirically verified: 100 recent key_issued audit rows all have key_id populated ✓

### M4 (FSM revalidate)
- Single const `stateAwaitingRegenConfirm` — no magic strings ✓
- Confirm handler: Load → reject if not `awaiting_regen_confirm` → clear on success/error ✓
- Cancel handler: Load → silent skip if not matching → clear if matching ✓

### Security posture
- `math/rand` NEVER in util: 0 matches ✓
- `singleflight` NEVER reintroduced: 0 matches ✓
- `zap.*plaintext` patterns: 0 matches ✓
- Plaintext never logged on retry, error, or success paths ✓
- SQL injection: all queries parameterized ✓

### Build & test
- `go build ./...` clean ✓
- `go vet ./...` clean ✓
- Full `go test ./...` — all packages green ✓
- Concurrent race test: 10 iterations, all pass, success+contention counts sum to 10 ✓
- Token length test: 3×50k iterations, 0 failures ✓

---

## Plan spec alignment
- [x] ErrKeyRaceContention defined + exported + documented
- [x] Issue retries max 1 time
- [x] PEEK/INCR separated
- [x] 4th rate limit test present (+PeekDoesNotIncrement +KeyNaming)
- [x] Base58 entropy preserved ≥181 bits (LSB truncation only; documented)
- [x] audit_log.key_id populated (verified in dev DB)
- [x] FSM revalidate in both confirm and cancel callbacks

---

## Recommended next action

**Ship the code.** The fixes are production-correct. N1 is a dev-DB hygiene issue, not a code defect — it does not block the commit.

**Before merging to main, run one-off ops fix:**
```bash
docker exec sbf_postgres psql -U sbf -d sbf_dev -c "
  DELETE FROM api_keys WHERE is_active=TRUE AND ctid NOT IN (
    SELECT MAX(ctid) FROM api_keys WHERE is_active=TRUE GROUP BY user_id
  );
  DROP INDEX IF EXISTS idx_keys_user_active_unique;
  CREATE UNIQUE INDEX CONCURRENTLY idx_keys_user_active_unique
    ON api_keys(user_id) WHERE is_active = TRUE;
"
```

Or simpler: reset the dev DB (`docker compose down -v && up`) to get a fresh valid index.

**Optional follow-ups** (no blocker):
- N2: tighten rate-limit race if product wants strict 3/day (likely not needed for Phase 2)
- N3: improve cancel UX on Redis transient failure

---

## Unresolved questions

1. Should the migration switch away from `CONCURRENTLY` to avoid INVALID-index failure mode in dev/CI? (Production is fine either way since it runs once on clean schema.)
2. Does the user want N2 tightened, or is ±1 tolerance on the 3/day rate limit acceptable?
