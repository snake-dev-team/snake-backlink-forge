---
title: "Red-Team Review Round 3 — Phase 2 Plan Patches"
role: code-reviewer (red-team round 3)
date: 2026-04-24
phase: 2
round: 3
plan_dir: E:\tool_backlink\plans\260424-2135-phase-2-telegram-bot-wallet-sepay
commit: 51a901f
status: APPROVED_WITH_MINOR
---

# Red-Team Review Round 3 — Phase 2 Plan

## Verdict

**APPROVED_WITH_MINOR.** F5 type collision and H-Round2-1 auth-fail alert unwired are both genuinely closed. All 4 round-2 low nits are fixed. PART 1 domain deferral, PART 4 retry queue, PART 5 rollback tests are structurally correct and testable. Two new minor findings introduced by round-3 patches (neither is a cook-blocker): (a) **phase-06 has TWO producer snippets for `sepay_retry_queue`** — one old (line 141, raw payload LPush, NO LTRIM, pre-envelope) and one new (line 412, RetryEnvelope + LTRIM). Implementer must delete the old one; cook will need the careful reader to spot the duplicate. (b) **`TestRetryQueueDeadLetter` as drafted cannot fail** — plan says "use UnknownOrder for perma-fail", but `UnknownOrder=true` path returns `nil` error → consumer treats as success and `continue`; dead-letter never fires. The alternative "FK-violating synthetic payload" is mentioned but not specified. Test will pass vacuously unless implementer picks a real failure mode. Both are test/spec-level cook-time corrections; no round 4 needed.

Cook readiness: GREEN on F5, auth-fail alert, PART 1/4/5/6. YELLOW on retry-queue-producer duplicate text and TestRetryQueueDeadLetter scheme — easily resolved during `/ck:cook` by implementer reading both snippets + picking the envelope-wrapping one.

## Round 2 findings closure audit

### F5 — audit→ledger type collision → Option A rewrite
- **Status:** CLOSED
- **Verification:** `phase-08-admin-commands.md:39-47` (requirements prose) + `:121-161` (pseudocode). Single pgx.Tx with explicit order: `grant_credits(ref_type='user', ref_id=target_user_id UUID)` → `SELECT currval('ledger_id_seq')` → `INSERT audit_log(metadata.ledger_id=$bigint)` → COMMIT. Deferred `defer tx.Rollback(ctx)` at line 127 guards all exit paths.
- **Atomicity proof:** Step 1 failure → no writes, rollback no-op. Steps 2-3 failures → rollback drops ledger row + wallet delta from Step 1. COMMIT success → both rows present with cross-referencing `metadata.ledger_id` (bigint in jsonb) matching `ledger.id` (BIGSERIAL).
- **Attack outcomes:**
  - **Type correctness:** `target_user_id uuid.UUID` passed to `grant_credits(p_ref_id UUID)` — fits. `ledger.ref_entity_id UUID` — fits. No collision. Plan pseudocode line 131: `SELECT grant_credits($1, $2, $3, 'admin_adjust', 'user', $1)` — `$1 = targetUserID` reused for both `p_user_id` and `p_ref_id`. pgx supports parameter reuse. ✓
  - **currval in same tx:** `tx.QueryRow(ctx, 'SELECT currval(...)')` — uses `tx` handle, not pool. pgx.Tx reuses single session for all queries within tx boundary → currval reads the same sequence state as `grant_credits` just inserted. ✓
  - **Concurrent admins:** pgx pool gives each tx its own conn. currval is session-scoped → first admin's sequence does not leak to second. ✓
  - **Rollback on audit INSERT failure:** line 154-156 returns error without swallow; `defer tx.Rollback(ctx)` fires. ✓
  - **Metadata contents:** all 5 specified fields present (`ledger_id`, `amount`, `pool`, `admin_tg_id`, `reason`) in line 144-153 pseudocode. ✓
  - **NULL reason behavior:** `jsonb_build_object('reason', NULL)` produces `"reason": null` (PG spec). Safe. ✓
  - **Sequence name:** `ledger_id_seq`. Schema at init.sql:63-64 is `CREATE TABLE ledger (id BIGSERIAL PRIMARY KEY, ...)`. PG default sequence name for BIGSERIAL = `<table>_<col>_seq` = `ledger_id_seq`. ✓
- **Risk row:** phase-08:350 explicitly documents currval session-scope + `grant_credits` nextval guarantee. ✓

### H-Round2-1 — Auth-fail alert producer
- **Status:** CLOSED
- **Verification:** phase-08-admin-commands.md:164-213 (full goroutine implementation) + `:231` modify main.go + `:299-300` impl steps 6-7 + `:322-323` todo + `:329` integration test + `:341` success criteria + `:349` risk row. phase-10:90 test + phase-10:247 todo. phase-06:549 risk row updated to reference the new watcher.
- **Event name normalization:** canonical = `sepay_auth_fail` throughout. Explicit note at phase-08:216: "normalized to `sepay_auth_fail` throughout this plan for consistency with phase-06 audit taxonomy." Grep confirms zero occurrences of `webhook_auth_fail` outside the naming note. ✓
- **Dedup correctness:**
  - Bucket calc: `bucket := now.Unix() / (15 * 60)` = 15-min int-div windows. ✓
  - Map short-circuits at line 179 before DB query. ✓
  - GC at line 200-205: `b < bucket-4` deletes buckets older than ~1h. Bounded map. ✓
  - Clock skew: backward skew could re-alert for a previously-GC'd bucket. Acceptable edge-case; operator notified on re-alert is benign.
  - Process restart: map resets; first tick post-restart may re-alert if count still ≥20. Acceptable.
- **Non-blocking send:** line 193-209 uses `select { case alertCh <-: ...; default: log.Warn }`. ✓
- **Goroutine lifecycle:** spawned in main.go (plan phase-08:231 + step 7); `<-ctx.Done()` exits (line 175); `defer ticker.Stop()` at line 170. ✓
- **Threshold:** `count >= 20` (line 192) — matches directive. ✓
- **Query scope:** event + 15min INTERVAL window; uses idx_audit_user partial coverage but scans time range. Planner new-risk #4 documents this as acceptable cost.
- **Test (`TestAuthFailBurstAlert`):** phase-10:90 explicitly seeds 20 rows with created_at=NOW()-1min, uses 1s ticker override, asserts alert within 2s, re-fires and asserts no duplicate. Ticker-override mechanism not spelled out but implementer-obvious (constructor param or test-only constant).

### Low nits (3+1 items from round 2)
- **uuid.uid → uuid.UUID typo** phase-06:181 → FIXED. Grep confirms zero `uuid.uid` occurrences. ✓
- **max0/maxRate undefined helpers** phase-06:312 → FIXED. Return uses hoisted `bonus` local var (line 248, 312). Grep confirms zero `max0`/`maxRate` occurrences. ✓
- **isPoolExhausted undefined** phase-06:147 → FIXED. Case reduced to `errors.Is(err, pgx.ErrAcquireTimeout)` only. Grep confirms zero `isPoolExhausted` occurrences. ✓
- **Migration 004 "cannot be reversed" rephrase** → FIXED. plan.md:87 now says "Effectively forward-only in this deployment. Enum value removal is technically possible via type recreate + data migration but invasive". phase-04:138-143 same prose. Accurate. ✓

## PART 1 — Domain deferral verification

- **plan.md ADR row:** lines 114-123 "ADR — Deployment domain deferral" explicit: Fly.io subdomain Phase 2-9, custom domain post-Phase-9, Cloudflare Tunnel fallback, `ngrok http 8080` for local dev. ✓
- **phase-06 §Deploy target:** lines 573-575 — production URL `https://snake-backlink-api.fly.dev/webhooks/sepay` + CF Tunnel fallback + local dev ngrok. ✓
- **phase-10 deploy checklist:** lines 287-301 — smoke test commands, SePay dashboard verification item, CF Tunnel fallback path, `fly secrets set` admonition, migration ordering during low-traffic window. ✓
- **No stale hardcoded domain refs:** grep for `snakebacklink.com` → 2 hits in phase-07 (`InstallerURL` default `https://cdn.snakebacklink.com/installer/SnakeBacklinkSetup.exe`). NOT cook-blocking because (a) that's the installer CDN, not the SePay webhook, and (b) it's a Phase 3+ concern. Note for consistency later.

## PART 4 — Retry queue verification (LPUSH/LTRIM, dead-letter, backlog alert)

### Correctness checks
- **LTRIM 0 499 semantics:** keeps indices 0-499 inclusive = 500 items. After LPUSH (new to head) + LTRIM drops the oldest (tail). ✓ correct for "keep newest 500".
- **BRPOP sepay_retry_queue 0:** 0 = block forever. LPUSH head + BRPOP tail = FIFO. ✓
- **RetryEnvelope struct:** phase-06:347-351 defines `{Payload, Attempts, OriginalTS}` with explicit Go types. ✓
- **Exponential backoff:** `min(60, 1<<env.Attempts) * time.Second` at line 368. At attempts=1 → 2s; 2 → 4s; cap at 60s. ✓
- **Max retries = 3, then dead-letter:** lines 378-388. Counter increment at 378, `env.Attempts >= 3` guard at 379, LPush to `sepay_dead_letter` + admin alert. ✓
- **Dead-letter uncapped:** planner disclosed in new-risk notes. Acceptable for Phase 2 scale.
- **Backlog alert:** lines 395-403. `llen > 400 && !backlogAlertedHigh` → alert; `llen < 200 && backlogAlertedHigh` → reset flag. ✓
- **Alert when queue fills via producer (not via consumer):** backlog check occurs ONLY after consumer re-enqueue branch. If 500 items arrive from producers and no processing failures, consumer drains them via BRPOP + success → `continue` before ever reaching backlog check. Operator will learn about backlog only if failures are happening. Minor gap but acceptable — lock storm producing backlog implies failures too.

### Attack outcomes
- **LPUSH + LTRIM race (2 producers):** Planner new-risk #2 disclosed. Worst case: transient 501-502 between P1-LPUSH and P1-LTRIM → P2-LPUSH. LTRIM on next call stabilizes at 500. No correctness issue since eviction policy is LIFO from tail (oldest first). Producer P1's payload is never evicted by P2's LTRIM unless queue was already at capacity — in which case the OLDEST payload drops, not P1. ✓
- **Dead-letter flood:** 1 LPush per failure → 1 admin alert per LPush. If 1000 perma-failing payloads, 1000 admin DM alerts. Risk: alert spam. Planner did not dedup this. LOW severity because dead-letter triggers require 3 prior retries each, capped at queue size 500 = ~1500 retries to fill dead-letter to 500. Operator gets overwhelmed, but not a correctness bug. Consider adding dedup in cook.

### Integration test coverage
- **`TestRetryQueueCapLTRIM`** (phase-10:66): LPUSH 501 → LLEN=500. Correct assertion. ✓
- **`TestRetryQueueConsumerSuccess`** (phase-10:67): LPUSH 1 valid → consumer drains → wallet credited. ✓ (timing-sensitive "wait 3s" may flake on slow CI — minor.)
- **`TestRetryQueueDeadLetter`** (phase-10:68): **SPEC BUG.** Plan says "order_code that doesn't map to any transactions row (perma-fail: UnknownOrder loops indefinitely — OR construct failure via FK-violating synthetic payload)." `UnknownOrder=true` returns `(ProcessResult{UnknownOrder: true}, nil)` at phase-06:233. Consumer treats nil error as success (line 376: `if procErr == nil { continue }`). Dead-letter NEVER triggers via UnknownOrder. Alternative "FK-violating synthetic payload" is not concretely specified. **Impact:** test will either (a) fail in implementation because dead-letter never fills, OR (b) pass vacuously if assertion is wrong. Implementer must pick a real failure mode (e.g., testcontainers-installed test-only trigger on `transactions` or `ledger` that raises on sentinel marker, similar to F5 audit fail test). NOT cook-blocking, but must be resolved at implementation time. See findings below.
- **`TestRetryQueueBacklogAlert`** (phase-10:69): LPUSH 401 → alert fires once → pop to <200 → LPush >400 → alert fires again. Valid test for dedup behavior. ✓

### Producer duplication (NEW finding — see [H-Round3-1] below)
Two producer snippets co-exist:
- phase-06:141 (original F3 classifier): raw `json.Marshal(p)` + `LPush` — no envelope, no LTRIM.
- phase-06:412 (round-3 producer snippet): `RetryEnvelope{...}` wrapper + LPush + LTRIM 0 499.
Step 7c (phase-06:481) says to wrap in envelope — but the classifier code block in line 131-159 was not textually updated. Implementer sees both. Consumer expects RetryEnvelope → if implementer picks old shape, unmarshal fails silently and every lock-contention webhook is lost.

## PART 5 — Rollback tests verification

`TestAdminGrantAtomicRollback` (phase-10:85-89).

- **Case 1 (grant fail):** plan says "invoke with non-existent targetUserID (random UUID) → FK violation on ledger INSERT". The `grant_credits` stored proc does `UPDATE wallets WHERE user_id = p_user_id RETURNING ... INTO new_balance`. If no wallet row exists for the UUID, `new_balance` is NULL → the subsequent `INSERT INTO ledger (..., balance_after, ref_entity_type, ref_entity_id)` with NULL balance_after violates NOT NULL. Or the FK on `ledger.user_id REFERENCES users(id)` fails because the UUID doesn't exist. Either mechanism raises and rolls back. ✓ Testable.
- **Case 2 (audit fail):** plan specifies a test-only trigger installed in `SetUp` + dropped in `TearDown`. Trigger raises exception on `reason='__test_fail_marker__'`. This is practical with testcontainers (fresh DB per suite). Planner new-risk #5 documents trigger leak mitigation. ✓
- **Case 3 (happy path):** assert `audit_log.metadata->>'ledger_id'` matches `ledger.id`. ✓
- **`-race -count=100`:** plan at phase-10:89 says "go test -race -count=100 -run TestAdminGrantAtomicRollback". No `-cpu` matrix — rollback tests are serial tx semantics, matrix overkill. Accepted.

## New findings introduced by round 3 (if any)

### [H-Round3-1] phase-06 has two conflicting producer snippets for sepay_retry_queue
- **Location:** phase-06:141 (old, classifier inline) + phase-06:408-414 (new, round-3 producer block).
- **Impact:** If implementer codes up the F3 `classifyWebhookError` function directly from the `### [F3] Error classifier` code block (line 131-159), they'll write raw `json.Marshal(p)` + `LPush` — no envelope, no LTRIM. Consumer at line 357-405 expects `RetryEnvelope` — JSON unmarshal fails at line 363, logs "retry envelope decode failed", `continue` — silent drop. Every lock-contention webhook would be lost with no retry.
- **Fix:** cook-time delete old producer line 140-141 from classifier block and replace with: `env := RetryEnvelope{Payload: p, Attempts: 0, OriginalTS: time.Now().Unix()}; data, _ := json.Marshal(env); _ = deps.Rdb.LPush(context.Background(), "sepay_retry_queue", data).Err(); _ = deps.Rdb.LTrim(...0, 499).Err();`. Same logic as round-3 producer snippet.
- **Severity:** HIGH if ignored; LOW if implementer reads both and picks the new one. Not a blocker because the spec intent is clear (step 7c + new-producer block), just duplicated.

### [L-Round3-1] TestRetryQueueDeadLetter perma-fail mechanism underspecified
- **Location:** phase-10:68.
- **Impact:** `UnknownOrder` path returns nil error → consumer's `if procErr == nil { continue }` treats it as success. Dead-letter will not populate. Test as drafted cannot verify 3-retry → dead-letter behavior.
- **Fix:** cook-time pick a concrete failure mode:
  - (a) Install test-only trigger on `ledger` INSERT that raises on sentinel marker in payload metadata.
  - (b) Use a second test DB with FK constraint already violated (e.g., wallets row deleted post-tx-start).
  - (c) Use mock pg pool that errors on every ProcessPaidTransaction call (simplest, breaks integration purity).
- **Severity:** LOW (test-only; doesn't block production paths).

### [L-Round3-2] Dead-letter alert dedup missing
- **Location:** phase-06:382-384.
- **Impact:** Each dead-letter LPush fires 1 admin alert. 100 perma-fail payloads → 100 admin alerts. Alert fatigue.
- **Fix:** cook-time add per-order-code dedup or rate limit (1 alert per 5min window).
- **Severity:** LOW (accepted residual; operational pain, not correctness bug).

### [L-Round3-3] Old phase-07 installer URL references `snakebacklink.com` domain
- **Location:** phase-07:36, 108.
- **Impact:** Future inconsistency — Phase 3+ when installer URL is wired, the domain may not match. Not in Phase 2 scope.
- **Fix:** Leave as-is for Phase 2. Flag for Phase 3 consistency pass.
- **Severity:** LOW (out-of-scope).

## Net verdict for /ck:cook readiness

**GREEN** on all round-2 blockers:
- F5 genuinely closed with Option A (same-tx currval + audit metadata).
- H-Round2-1 auth-fail alert fully wired in main.go spawn + goroutine + test + risk row.
- 4 low nits all cleaned.

**YELLOW** (non-blocking, cook-time fixes):
- [H-Round3-1] delete old classifier producer; use round-3 envelope producer.
- [L-Round3-1] pick concrete failure mode for TestRetryQueueDeadLetter.

Both are spec-hygiene issues the implementer can resolve in-flight. No round 4 required.

## Recommended pre-cook actions (delta from round 2)

1. **[H] In classifyWebhookError code block (phase-06:140-144)**: replace 2-line producer with the RetryEnvelope + LTRIM variant from phase-06:409-413. Delete old `payloadJSON, _ := json.Marshal(p)` + `LPush` lines. Consumer expects envelope; sending raw payload = silent drop.
2. **[L] In TestRetryQueueDeadLetter (phase-10:68)**: pick a concrete failure mode. Recommended: test-only trigger on `ledger` INSERT that raises on sentinel metadata marker. Document in test prelude.
3. **[L, optional] Consider dead-letter alert dedup** (rate limit 1 alert per 5min per order_code) to prevent alert fatigue during mass-failure scenarios. Not critical for Phase 2.

## Residual risks accepted (updated cumulative list)

1. **Combo bonus blended per-credit rate** — approximation; accepted.
2. **Migration 004 NO TRANSACTION DROP→CREATE gap** — ~100ms window; deploy during downtime (phase-10 checklist).
3. **Retry queue LTRIM race** — transient 501-502 briefly possible; LTRIM stabilizes; LIFO eviction from tail; accepted.
4. **1đ over-pay log noise** — benign.
5. **Down-migration of enum values** — effectively forward-only; accepted.
6. **Admin chat has no IP context** — metadata `source="telegram_admin"` substitute.
7. **Retry consumer backlog alert only fires on re-enqueue path** — acceptable; lock storm implies failures, failures flow through re-enqueue.
8. **Dead-letter alert per LPush (no dedup)** — cook-stage improvement candidate.
9. **auditFailAlertWatcher clock-skew re-alert** — benign, accepted.
10. **SePay `.fly.dev` webhook URL acceptance** — pre-deploy verify (plan-level Risk row + phase-10 checklist).
11. **phase-07 installer URL still references `snakebacklink.com`** — Phase 3+ consistency concern.

## Unresolved questions (if any)

1. **Dead-letter alert dedup strategy** — per-order-code? fixed window? first-N-suppress-rest? Planner/user decide at cook time.
2. **Ticker-override mechanism for TestAuthFailBurstAlert** — pass ticker interval as constructor param to watcher? Or package-level var for test-only access? Either works; implementer picks.
3. **phase-07 installer URL domain** — leave as `snakebacklink.com` now and update in Phase 3, or replace with `snake-backlink-installer.fly.dev` today? Low priority.

---

**Status:** DONE
**Verdict:** APPROVED_WITH_MINOR
**Summary:** F5 closure verified (currval in same pgx.Tx, proper type fits, atomicity preserved). Auth-fail alert genuinely wired (main.go spawn + goroutine + test + risk row). 4 low nits all cleaned. PART 1/4/5/6 structurally sound. 1 new HIGH finding (duplicate retry-queue producer text in phase-06 — implementer must delete the old producer, not cook-blocking since intent is clear) + 2 LOW (TestRetryQueueDeadLetter underspecified perma-fail mechanism; dead-letter alert no dedup).
**Report:** E:\tool_backlink\plans\reports\redteam-260424-2235-phase-2-round-3.md
**Commit sha:** pending
**Cook readiness:** GREEN (with two minor cook-time corrections queued; no round 4 needed)
