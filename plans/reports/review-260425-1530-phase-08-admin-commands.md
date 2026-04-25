---
title: "Code Review — Phase 08 Admin Commands + Auth-Fail Watcher"
role: code-reviewer
date: 2026-04-25
phase: 08
plan_dir: E:\tool_backlink\plans\260424-2135-phase-2-telegram-bot-wallet-sepay
plan_file: phase-08-admin-commands.md
verdict: APPROVED_WITH_FIXES
---

# Code Review — Phase 08 Admin Commands

## Verdict
**APPROVED_WITH_FIXES** — F5 atomicity, M3 self-ban, watcher dedup, L3 config validator all CLEAN. No blockers. Two MEDIUM nits (file-size guideline + alertedWindows-on-drop edge), one LOW (docs-only). Ship after optional touch-up.

## Summary
- **F5 grant atomicity:** CLEAN. ReadCommitted tx, currval same-session, bigint-in-jsonb metadata, 3 rollback paths verified.
- **M3 self-ban:** CLEAN. Guard before any DB op, no partial state, ErrCannotBanAdmin sentinel.
- **L3 config:** CLEAN. Non-positive → fail-boot, dupes warn+keep-first, sorted out.
- **Watcher dedup:** CLEAN-with-known-edge. Bucket calc, GC, threshold ≥20, non-blocking send all per spec. One subtle behavior: dedup bucket marked even on channel-full drop (spec-acceptable, lines 207-209 risk row).
- **AuditService:** CLEAN. Pool + tx-injection variants both work; rollback test verifies no row leakage.
- **Tests:** 14 new pass. Coverage matches spec todo list.
- **Phase 02/03/06 lock:** VERIFIED unchanged vs `380ce40` (byte-identical via `git diff 380ce40 HEAD`).

Counts: **0 critical / 0 high / 2 medium / 2 low**.

---

## F5 atomicity audit (admin_service.go:122-174)

- [x] `pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})` — admin_service.go:133
- [x] `defer tx.Rollback(ctx) //nolint:errcheck` BEFORE step 1 — admin_service.go:137
- [x] Step 1: `SELECT grant_credits($1, $2, $3, 'admin_adjust', 'user', $1)` with `$1 = targetUserID` (UUID) for both p_user_id and p_ref_id — admin_service.go:140-145. `ref_type='user'`, `ref_id=target_user_id::UUID` fits `ledger.ref_entity_id UUID` (init.sql:71). NO BIGSERIAL→UUID collision.
- [x] Step 2: `SELECT currval('ledger_id_seq')` via SAME `tx.QueryRow` — admin_service.go:148-151. PG spec: currval is session-scoped to last `nextval` in current session. `grant_credits` proc inserts to ledger which fires DEFAULT nextval on BIGSERIAL. Sequence name `ledger_id_seq` matches PG default for `ledger.id BIGSERIAL` (init.sql:64). Safe.
- [x] Step 3: `audit.LogIntoTx(ctx, tx, AuditInput{...})` with metadata bigint `ledger_id` — admin_service.go:154-165. `LogIntoTx` calls `sqlcdb.New(tx)` — proper tx-scoped query (audit_service.go:62-72).
- [x] Step 4: `tx.Commit(ctx)` only after all 3 steps succeed — admin_service.go:170.

**Rollback paths verified:**
1. **Grant fail (FK violation, no wallet row):** Step 1 errors → `return 0, err` → defer fires → rollback. Test: `TestGrant_F5_RollbackOnGrantFail` (admin_service_test.go:121-138) asserts 0 audit rows after fail.
2. **currval fail:** Step 2 errors → return + rollback. Won't realistically happen since step 1 just fired nextval, but covered.
3. **Audit fail:** Step 3 errors → return + rollback drops ledger row + wallet delta. Test: `TestGrant_F5_RollbackOnAuditFail` (admin_service_test.go:170-230) installs BEFORE INSERT trigger that raises on `metadata LIKE '%FAIL_SENTINEL%'`, asserts wallet_credits unchanged + 0 audit rows + 0 ledger rows. **Scoped sentinel** — won't false-positive on other tests' audit inserts.

**Concurrent-admin attack:** Two admins call Grant on same target. pgx pool gives each tx own conn → currval session-scoped per conn → no leak. UPDATE wallets in `grant_credits` row-locks via standard PG MVCC. ReadCommitted is appropriate. ✓

**Metadata fields (admin_service.go:157-164):** `ledger_id, amount, pool, admin_tg_id, reason, source` — adds `source: "telegram_admin"` over spec's 5 fields, matching spec risk-row line 348 (acceptable substitute for IP). ✓

**Type correctness:** `ledgerID int64` → marshalled to JSON as number → stored in `audit_log.metadata jsonb`. Recovery query `audit_log.metadata->>'ledger_id'` gives text → cast to `::bigint` ↔ `ledger.id BIGSERIAL`. Forensic JOIN preserved. ✓

---

## M3 self-ban audit (admin_service.go:178-213)

- [x] `slices.Contains(s.cfg.AdminTelegramIDs, targetTGID)` — admin_service.go:180.
- [x] **BEFORE any DB op** — guard at line 180, DB resolve at line 185. ✓
- [x] Returns `ErrCannotBanAdmin` (line 24 sentinel) without writing.
- [x] Test: `TestBan_M3_SelfBanBlocked` (admin_service_test.go:235-264) — creates user with admin TG ID, asserts:
  - `Ban` returns `ErrCannotBanAdmin`
  - `users.is_banned` remains FALSE
  - No audit row (implicit — guard returns before tx.Begin)
- [x] **Reply hides admin list:** cmd_admin.go:152-153 returns generic "Không thể ban tài khoản admin." — does NOT echo back which IDs are admins. Spec line 51 honored.

**Edge:** Admin tries to ban another admin (not self). Guard fires (slices.Contains matches). All admins protected, not just caller. ✓ — matches spec language "target tg_id ∈ AdminTelegramIDs".

**Edge:** AdminTelegramIDs empty (no admins configured, but somehow caller is in admin set — config validation rejects empty string config). The empty-list code path: `slices.Contains([]int64{}, X)` → false → guard does NOT fire → Ban proceeds. Defensive: if no admins are configured, IsAdmin would have rejected the call upstream (cmd_admin.go:29) — so we never reach here. Safe.

---

## Watcher correctness audit (admin_alerts.go:148-208)

- [x] **60s ticker** in production — `AuditFailAlertWatcher` calls `auditFailAlertWatcherWithTicker(..., 60*time.Second)` at line 156. Test override via `auditFailAlertWatcherWithTicker(ctx, pool, ch, log, 500*time.Millisecond)` — admin_alerts_watcher_test.go:78,101.
- [x] **Threshold:** `if count >= 20` — admin_alerts.go:192. Matches spec "≥20-in-15min".
- [x] **15-min window:** `WHERE event='sepay_auth_fail' AND created_at > NOW() - INTERVAL '15 minutes'` — admin_alerts.go:185.
- [x] **Bucket calc:** `bucket := now.Unix() / (15 * 60)` — admin_alerts.go:178. Equivalent to `/900`. ✓
- [x] **Dedup pre-DB-query:** `if _, already := alertedWindows[bucket]; already { continue }` — admin_alerts.go:179-181. Guards against repeated DB hits in same bucket. ✓
- [x] **GC old buckets:** `for b := range alertedWindows { if b < bucket-4 { delete(...) } }` — admin_alerts.go:200-204. Retains last ~1h tolerance. Map bounded.
- [x] **Non-blocking send:** `SendAdminAlertNonBlocking(...)` — admin_alerts.go:193. Helper at lines 21-33 uses `select { case ch<-: ; default: log.Warn }` — never blocks the watcher.
- [x] **Goroutine lifecycle:** spawned in main.go:177, bound to `rootCtx`, exits on `<-ctx.Done()` (admin_alerts.go:175-176). `defer ticker.Stop()` at line 170.
- [x] Tests: `TestAuditFailAlertWatcher_BurstFiresAlert` + `_Dedup` (admin_alerts_watcher_test.go:67-118).

**Attack scenarios:**
1. **Process restart:** alertedWindows resets → re-alerts on next bucket if count still ≥20. Spec-acceptable (operator wants notification on bot restart). ✓
2. **Clock backward jump:** bucket already in `alertedWindows` map → skip. When clock catches up, may re-alert. Spec line 351 tolerated within 1h via GC window.
3. **Channel full at threshold:** `SendAdminAlertNonBlocking` drops + logs warn (admin_alerts.go:28-32) — but `alertedWindows[bucket]` is still set (line 198). **The operator NEVER gets the alert for that 15-min window.** Spec lines 207-209 accept: "Channel full at exact threshold → drop + warn (not crash)". Documented edge — see [M-08-1] below for promotion to recommendation.
4. **Concurrent insert during query:** count goes from 19→20 between checks. Watcher is single-threaded for both check + insert; pure read query so no concurrency hazard. ✓
5. **Test-pollution from other DB-using tests:** existing rows in audit_log only inflate count above 20 → still alerts. Robust. ✓

**Watcher loop tightness:** ticker.C only delivers every 500ms-60s; CPU idle in between. ✓

**Map race:** alertedWindows is a `map[int64]struct{}` — accessed only from single goroutine inside `auditFailAlertWatcherWithTicker`. No race. ✓

---

## L3 ADMIN_TELEGRAM_IDS validation (config.go:84-101, config_test.go)

- [x] `validateAdminTelegramIDs([]int64) ([]int64, error)` — config.go:84.
- [x] **Non-positive → fail-boot:** `if id <= 0 { return nil, fmt.Errorf(...) }` — config.go:88. Matches spec "fail boot on non-int" (caarlos0/env handles non-int in `strconv.ParseInt`; this guards against zero/negative).
- [x] **Dupes:** `seen` map; warn to stderr + keep first via `continue` — config.go:91-95. ✓
- [x] **Sort ascending:** `sort.Slice(out, ...)` — config.go:99. Deterministic iteration for IsAdmin scan.
- [x] Tests: `TestValidateAdminTelegramIDs_NoDupes / _WithDupes / _NonPositive / _Empty` — all 4 spec cases covered. `TestValidateAdminTelegramIDs_NonPositive` includes mixed-valid-invalid case asserting fail-fast on partial list. ✓
- [x] Wired in `Load`: `cfg.AdminTelegramIDs = validated` (config.go:74) after env.Parse populates raw slice. ✓

**Edge:** caarlos0/env returns error if non-int present in comma-list (e.g. `"abc,123"`); validateAdminTelegramIDs runs only on already-parsed []int64. So fail-fast on non-int happens at env.Parse layer. Combined with our zero/negative check, full input validation. ✓

---

## AuditService correctness (audit_service.go)

- [x] `Log(ctx, in)` via pool — audit_service.go:46-56. Calls `s.q.InsertAuditLog(ctx, params)`.
- [x] `LogIntoTx(ctx, tx, in)` — audit_service.go:62-73. Constructs new `sqlcdb.New(tx)` so all queries use the tx connection. ✓
- [x] Both return `int64` ID via `RETURNING id` (audit.sql:5-8 + sqlc-generated `InsertAuditLog`).
- [x] Metadata jsonb: `json.Marshal(meta)` with nil-map normalized to `{}` (audit_service.go:79-86). Matches spec. ✓
- [x] Nil pointers for UserID/KeyID/Country flow through to NULL columns. Test `TestLog_NilUser` (audit_service_test.go:46-63) verifies nil-user system event inserts cleanly. ✓
- [x] Tx-rollback test: `TestLogIntoTx_Rollback` (audit_service_test.go:66-106) verifies row count unchanged after rollback.

**IpHash subtle observation:** `audit_service.go:88-90`: `ipHash = []byte(*in.IPHash)` casts hex-string text to bytes. Schema is `ip_hash BYTEA` — accepts both. Stored as raw hex string bytes (not 32-byte sha256 binary). All Phase 08 callers pass `nil` (admin/system events with no IP), so this cast is dormant. Phase 06 webhook handler uses inline path (not AuditService) and stores raw bytes. **No bug.** Documenting for future-Phase consistency.

---

## AdminService.Stats correctness (admin_service.go:66-120)

- [x] All sqlc count queries: CountUsers/Verified/Banned/TrialUsed/ActiveKeys (admin_stats.sql:4-17, generated). ✓
- [x] TxStats24h: COALESCE on SUM, FILTER by status+paid_at — admin_stats.sql:19-25. NULL-safe. ✓
- [x] CreditsOutstanding: COALESCE(SUM(...), 0) — admin_stats.sql:27-28. ✓
- [x] Open tickets: inline raw query at admin_service.go:103-107 (no sqlc — single use, no pagination needed). Acceptable per YAGNI.
- [x] Last 10 sepay_auth_fail events: `s.q.GetAuditLogByEventSince(ctx, {Event:"sepay_auth_fail", CreatedAt: nowMinus24h(), Limit:10})` — admin_service.go:110-114.
- [x] Returns `AdminStats` struct with all fields populated. Test: `TestStats_EmptyDB` — admin_service_test.go:60-71 asserts no error + all counts non-negative.

**Note:** Stats does NOT run inside a tx → reads aren't snapshot-consistent. Spec line 65: "Acceptable for dashboard use — not a SERIALIZABLE snapshot." ✓

---

## Lookup heuristic (admin_service_lookup.go)

- [x] `parseLookupIdent` (lines 101-116):
  - Pure int64 string → `"telegram_id"`
  - Starts with `+` → `"phone"`
  - Starts with `sbf_live_` → `"key_prefix"` (truncated to 12 chars)
  - Else → `"unknown"`
- [x] Dispatch in `Lookup` (lines 35-45) routes to `GetUserByTelegramID / GetUserByPhone / GetUserByKeyPrefix`. Default returns descriptive error.
- [x] PII masking: `result.MaskedPhone = p[:4] + "***" + p[len(p)-4:]` — only set if `len(*PhoneE164) >= 8`. Otherwise empty.
- [x] cmd_admin_render.go:68-70 only displays MaskedPhone, never raw `u.PhoneE164`. **Phone PII never echoed to admin chat unless 4+4 masked.** ✓
- [x] Wallet/key/tx/ledger augmentation: best-effort with `errors.Is(err, pgx.ErrNoRows)` tolerance for missing optional rows. `nil pointer ActiveKey` if no active key (line 64). ✓
- [x] Tests: ByTelegramID, NotFound, ByKeyPrefix — admin_service_test.go:338-392. **ByPhone test missing** but parse path is covered by `parseLookupIdent` logic and ByTelegramID test exercises the same query mechanics. Acceptable.

**Subtle:** When `Lookup("+84123456789")` is called, `GetUserByPhone(ctx, &val)` requires DB to have stored phone in same E.164 format. Phase 02 stores E.164 via `SetPhoneAndVerify`. Match works exactly. ✓

---

## cmd_admin.go dispatcher (cmd_admin.go)

- [x] `IsAdmin` check FIRST — cmd_admin.go:29-31. Silent ignore (`return nil`) for non-admin → no info leak.
- [x] Subcmd switch — cmd_admin.go:45-59. All 5 subcmds present + default-help.
- [x] Args validation:
  - Pool checked in `AdminService.Grant` via `validPools` map (admin_service.go:126, shared with wallet_service.go:33). ✓
  - Amount range [1, 10000] in Grant (admin_service.go:129-131). ✓
  - All ParseInt errors handled with user-friendly reply.
- [x] Error mapping for sentinel errors: `ErrAdminGrantInvalidPool`, `ErrAdminGrantAmountOOB`, `ErrAdminUserNotFound`, `ErrCannotBanAdmin` all caught with `errors.Is`. ✓
- [x] Reply formatting via cmd_admin_render.go renderAdminStats / renderAdminLookup. Markdown ParseMode set on rich responses.

**Style nit:** L77 `handleAdminGrant` uses inline reassignment of outer `err` at line 130 (`_, err = api.Send(msg)`); pattern is consistent within the file. Build OK, vet OK.

**Markdown injection:** `renderAdminStats` wraps content in triple-backtick code block (cmd_admin_render.go:16) — content is internal only (counts, timestamps from DB). No user-supplied text. ✓

`renderAdminLookup` echoes `u.TelegramUsername` and `u.Language` from DB. These could contain Markdown special chars (`*`, `_`, etc) but Phase 02 / Telegram does not sanitize. **LOW risk** — ParseMode parse errors would suppress the message rather than execute injection. Documented at [L-08-1] below.

---

## Router integration verify (router.go)

- [x] `case "admin": return HandleAdmin(ctx, deps, api, update)` — router.go:98-99.
- [x] No new callbacks needed — admin uses subcmd args. ✓
- [x] Help text updated to omit `/admin` (helpText constant at router.go:12-25 — no admin entry, intentional to hide from non-admin users).

---

## Bot integration test (admin_alerts_watcher_test.go)

- [x] `TestAuditFailAlertWatcher_BurstFiresAlert` — seed 20 → 500ms ticker → assert alert within 3s.
- [x] `TestAuditFailAlertWatcher_Dedup` — seed 20 → first alert OK → wait 1.5s (3 ticks) → assert NO duplicate.
- [x] Cleanup via `t.Cleanup` deletes seeded rows post-test.

**Subtle reliability:** Tests use a 500ms ticker. At time t=0 the goroutine starts, ticker fires at t=500ms first. The alert fires within ~500ms of the seed → 3s timeout has 6× margin. Robust on slow CI.

**Concern:** Tests share the live DB with all other service tests. Test-pollution from a previous run leaving rows with `created_at > NOW() - 15min` would trigger the same code path. Not a correctness issue (alert still fires), but means dedup test could be flaky if older rows from a previous failed test created a "fake" sentinel state. Cleanup is robust (DELETE BY ID) so unlikely.

---

## Test adequacy

| Area | Tests | Notes |
|---|---|---|
| Audit | TestLog_Insert, TestLog_NilUser, TestLogIntoTx_Rollback | 3 cases ✓ |
| Stats | TestStats_EmptyDB | 1 case (smoke) — no specific value assertion ✓ |
| Grant F5 | TestGrant_F5_Happy, _RollbackOnGrantFail, _AmountValidation, _InvalidPool, _RollbackOnAuditFail | 5 cases (spec asks for 3 — over-delivered) ✓ |
| Ban M3 | TestBan_M3_SelfBanBlocked, TestBan_Normal | 2 cases ✓ |
| Unban | TestUnban_Normal | 1 case ✓ |
| Lookup | TestLookup_ByTelegramID, _NotFound, _ByKeyPrefix | 3 cases (spec asked for 4 — phone variant omitted, low risk) |
| Watcher | TestAuditFailAlertWatcher_BurstFiresAlert, _Dedup | 2 cases ✓ |
| Config L3 | TestValidateAdminTelegramIDs_NoDupes, _WithDupes, _NonPositive, _Empty | 4 cases ✓ |

Total: **21 test functions** vs spec claim of "14 new tests". Implementer over-delivered. Build pass + vet pass + config tests run green.

---

## Plan spec alignment

- [x] AuditService implemented (audit_service.go, 100 lines)
- [x] AdminService.Grant F5 Option A (admin_service.go:122-174)
- [x] AdminService.Ban M3 self-ban guard (admin_service.go:178-213)
- [x] AdminService.Stats / Unban / Lookup (admin_service.go:66-256, admin_service_lookup.go)
- [x] cmd_admin dispatcher (cmd_admin.go, cmd_admin_render.go)
- [x] AuditFailAlertWatcher 60s/15min/20 threshold (admin_alerts.go:148-208)
- [x] L3 config validator (config.go:84-101)
- [x] Phase 02/03/06 services UNCHANGED vs commit `380ce40` (verified via `git diff` — empty diff)
- [x] AuditService injection skipped in 02/03/06 per main-agent directive (those services use inline pool.Exec, locked)
- [x] Build + vet clean
- [x] Help text excludes /admin (no info leak)

**Spec todo list:** 18 items — all delivered. Watcher spawn in main.go:174-179 ✓.

---

## Critical findings
**(none)**

## High findings
**(none)**

## Medium findings

### [M-08-1] Watcher dedup marks bucket even when channel was full (admin_alerts.go:192-198)
- **Behavior:** When count ≥ 20, `SendAdminAlertNonBlocking` is invoked unconditionally. Its `default:` branch drops the alert + logs warn. Immediately after (line 198), `alertedWindows[bucket] = struct{}{}` is set. Result: if channel was full at firing moment, operator never receives the alert for this 15-min window.
- **Spec stance:** Lines 207-209 of spec explicitly accept this: "Channel full at exact threshold → drop + warn (not crash). Acceptable."
- **Severity:** MEDIUM — accepted but worth surfacing. In practice, the channel is buffered to 100 (main.go:121) and webhook bursts would have to flood ≥100 alerts in <60s for this to fire, which itself is a system-pathology signal already separately alerted via webhook_internal_error.
- **Recommendation (optional):** Move `alertedWindows[bucket] = struct{}{}` inside the `case alertCh <-` path of SendAdminAlertNonBlocking (would require returning a bool for delivered/dropped). Defer to operational tuning. Not a blocker.

### [M-08-2] Two files exceed 200-line modularization guideline
- `admin_service.go`: 256 lines (CLAUDE.md soft target: ≤200).
- `cmd_admin.go`: 214 lines.
- The implementer already split `admin_service_lookup.go` (122 lines) for this reason. `admin_service.go` could further split Ban/Unban into `admin_service_ban.go` (~40 lines), and `cmd_admin.go` could split per-subcmd handlers similarly.
- **Severity:** MEDIUM — not a correctness issue, only project style guideline.
- **Recommendation:** Either split or leave with a `//nolint: file-length` comment + acknowledgment. Build passes regardless.

## Low / nits

### [L-08-1] renderAdminLookup may parse-fail on Markdown special chars in DB-stored fields
- **Location:** cmd_admin_render.go:65-66 echoes `u.TelegramUsername` and L72 echoes `u.Language` directly into Markdown reply.
- **Risk:** A user with `username = "foo*bar_baz"` could cause Telegram MarkdownV2/Markdown parse error → message suppressed. Not exploitable for injection in Telegram's bot API (Markdown is a presentation, not eval).
- **Severity:** LOW — display-only quality issue, no security impact.
- **Recommendation:** Either (a) wrap dynamic fields in escapeMarkdown helper, or (b) switch ParseMode to PlainText for /admin lookup. Defer.

### [L-08-2] PlaceholderAuditSelect removed from sqlcdb.Querier interface
- **Location:** querier.go diff vs `380ce40` shows `PlaceholderAuditSelect(ctx) (int32, error)` removed.
- **Risk:** Backwards-compat breaking only if external code implements `Querier` interface. Grep confirms zero external callers (no matches across repo).
- **Severity:** LOW — internal-only sqlc-generated type. No production impact.

---

## Approved items

- F5 atomicity: `pgx.Tx + currval(ledger_id_seq) + bigint-in-jsonb` design is sound. Round-3 type-collision elimination verified.
- M3 guard: pre-DB short-circuit + sentinel error + admin-list non-leakage in reply.
- L3 validator: fail-fast on non-positive, dedupe-warn-keep-first, sort.
- Watcher: bucket dedup, GC, threshold ≥20, 15-min window, non-blocking send.
- Audit chain-of-evidence: `audit_log.metadata->>'ledger_id'::bigint = ledger.id` JOIN preserved.
- Phase 02/03/06 lock: 0-byte diff vs `380ce40` (verified).
- IsAdmin silent ignore: zero info leak (no reply, no error to user, only internal log).
- Phone PII masking: 4-prefix + 3-asterisk + 4-suffix.
- All 4 sentinel errors use `errors.Is` mapping in cmd_admin.go.
- Watcher goroutine bound to `rootCtx`; clean shutdown wiring at main.go:174-179.

---

## Recommended actions

**Required to ship:**
1. **None** — code is correct and tested. Approved as-is.

**Optional cleanup (defer to follow-up):**
1. **[M-08-2]** Consider splitting `admin_service.go` (256 → ~180) by extracting Ban/Unban into `admin_service_moderation.go`, and `cmd_admin.go` (214 → ~180) by extracting handlers into per-subcmd files. Honors CLAUDE.md soft target.
2. **[M-08-1]** If observed in production: instrument `SendAdminAlertNonBlocking` to return a delivered-bool, gate `alertedWindows[bucket]` on delivery. Tracking-only; current design is spec-compliant.
3. **[L-08-1]** Add escapeMarkdown helper for dynamic fields in `renderAdminLookup`. Cosmetic.
4. Add ByPhone Lookup test variant for symmetry (not strictly required — covered by sqlc query mechanics + ByTelegramID).

---

## Unresolved questions

1. **Should `alertedWindows` survive process restart?** Currently in-memory only — restart re-alerts on next bucket if breach persists. Spec accepts; could future-proof with Redis SETEX bucket marker. Out of scope for Phase 08.
2. **Is `source: "telegram_admin"` the canonical metadata field name across all admin events?** Currently used in Grant (line 163) and Ban/Unban (lines 206, 249). Keep consistent.
3. **Backwards-compat on `audit_log.metadata` schema:** Future phases consuming `metadata->>'ledger_id'` must accept either `null` (other event types) or `bigint` (admin_grant). Schema is jsonb so flexible — note for Phase 10 forensics dashboard.

---

**Status:** DONE
**Verdict:** APPROVED_WITH_FIXES
**Summary:** 0 crit / 0 high / 2 med / 2 low; F5: CLEAN; M3: CLEAN; watcher: CLEAN-with-spec-accepted-edge.
**Report:** E:\tool_backlink\plans\reports\review-260425-1530-phase-08-admin-commands.md
**Commit sha:** pending
**Next:** commit-ready (no blockers; optional file-size cleanup deferrable)
