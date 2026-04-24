---
title: "Red-Team Review — Phase 2 Telegram Bot + Wallet + SePay"
role: code-reviewer (red-team adversarial)
date: 2026-04-24
phase: 2
plan_dir: E:\tool_backlink\plans\260424-2135-phase-2-telegram-bot-wallet-sepay
status: FIX_REQUIRED
---

# Red-Team Review — Phase 2 Telegram Bot + Wallet + SePay

## Verdict

**APPROVED_WITH_FIXES_REQUIRED.** Plan's core idempotency design (CAS on `transactions.status='pending' RETURNING`) is mathematically correct and blocks double-credit under concurrent webhook replay — the critical goal is met. However, several high-impact gaps exist that can be exploited:

1. **Order-code parse is ambiguous and 8-hex space is small** — attacker collision / false positive on user-supplied content.
2. **`transactions.provider_ref` is populated at topup-creation time with an 8-hex derived from UUID, but the existing schema has `provider_ref UNIQUE` constraint** — collision between users is rare but not zero (birthday bound ~4B, realistic collision at ~65K pending rows).
3. **No handling for over-payment** (user pays 500K for a 329K package — plan only covers `<`, not `>`).
4. **Trial gate missing FOR UPDATE row lock** → concurrent `/start` from same user races through phone-uniqueness pre-check.
5. **Admin `/admin grant` audit-log chain-of-evidence is broken** — `ref_id=audit_row_id` creates a chicken-and-egg: audit row needs ledger ref, ledger needs audit id.
6. **Suite D race test under-specified** — `FireN` uses `errgroup.Go` without `runtime.Gosched()` jitter or `-cpu=N -count=100` directive.
7. **Webhook retry semantics: SePay fetches 7x on non-2xx, but the plan returns 500 on transient DB errors** — this creates a retry amplification path for any tx-layer hiccup, magnifying lock contention.
8. **Rate limit on `/webhooks/sepay` endpoint is absent** — attacker with valid leaked Apikey can hammer the CAS for known order codes.

Blocker-class fixes must be applied to phase-06 (F1, F2, F3, F4), phase-02 (F5), and phase-10 (F6) before `/ck:cook`.

---

## Critical findings (must fix before /ck:cook)

### [F1] Webhook over-payment path undefined — credit discrepancy or revenue loss
- **File:** `phase-06-sepay-webhook.md:33, 87` + spec `docs/MASTER_PROMPT.md:1284` (§5.4 step 4)
- **Attack:** User pays 500,000đ for `standard_pro_200` (priced 329,000đ). Webhook receives `transferAmount=500000`, plan's check `p.TransferAmount < txRow.AmountVND` is FALSE → webhook CAS succeeds → wallet credited 200 standard (per package row). 171,000đ pocket-differential banked but unaccounted for. Worse case: user disputes with bank, we refund 329K, keep 171K but also keep 200 credits already spent.
- **Impact:** Revenue leak + compliance concern (unearned money held); potential chargeback loss.
- **Fix:** Add symmetric guard for over-payment. In `phase-06-sepay-webhook.md` §Requirements.Functional step 4: "Validate `transferAmount == amount_vnd` (exact match required). If `>` → set status='manual_review' + metadata.overpaid=<diff>, return 200 success. If `<` → already specified manual_review. Never auto-credit on mismatch." Update `ProcessPaidTransaction` code block to check both directions. Document in SECURITY.md that over/under-payment requires admin refund or manual adjust via `/admin grant`.

### [F2] Order-code collision + ambiguity — cross-user credit theft via content parsing
- **File:** `phase-05-transactions-topup.md:75` + `phase-06-sepay-webhook.md:18, 173`
- **Attack #1 (birthday collision):** `orderCode = strings.ToUpper(txID.String()[:8])` → 8 hex chars = 32 bits = ~4.3B space. With 65,536 pending transactions in history, a collision is ~50% (birthday paradox). Plan's webhook does `WHERE provider_ref=$1 AND status='pending'` so only concurrent pending rows with identical order code matter. But schema says `provider_ref VARCHAR(255) UNIQUE` (no status filter) — the INSERT in `CreateTopupIntent` will fail with 23505 on UUID prefix collision, bricking the user flow. Only `idx_tx_user_pkg_pending` is the partial index; the base `UNIQUE` constraint on `provider_ref` is unconditional per migration line 97.
- **Attack #2 (content regex false positive):** Regex `(?i)SBF\s+TOPUP\s+([A-F0-9]{8})`. Attacker sends bank transfer with memo "Note SBF TOPUP ABC12345 to John" — matches `ABC12345`. If another user has an unrelated pending order with that exact code, their credits are granted to the wrong user (webhook correlates by provider_ref only, not by sender/amount match to that user).
- **Attack #3 (prefix extract bug):** `strings.ReplaceAll(txID.String()[:8], "-", "")` — but `uuid.New().String()` produces `xxxxxxxx-xxxx-...` where the 8th char is `-`. `String()[:8]` yields `xxxxxxxx` (8 chars, no `-`). The `ReplaceAll("-", "")` is dead code. The real bug: silently fails to disambiguate. More importantly, the first 8 hex of a v4 UUID has only 32 bits — collision probability grows fast.
- **Impact:** Critical. Money-flow routing correctness. Potential cross-user grant + failed user flows due to 23505 at insert time.
- **Fix:** (a) Extend order code to ≥12 hex (48 bits, collision ~1 in 280T) OR use base58 to compress. Update `phase-05` §Architecture.CreateTopupIntent to `orderCode := strings.ToUpper(hex.EncodeToString(uuid.New()[:6]))` → 12 hex chars, and update regex in `phase-06` to `([A-F0-9]{12})`. (b) Add explicit insert-retry loop for 23505 on `provider_ref` UNIQUE constraint (orthogonal from the pending-partial-index handling). (c) Require webhook to also validate `transferAmount == txRow.amount_vnd` and `len(orderCode) == 12` exactly (reject partial-match from user memo pollution).

### [F3] Transient 500 response on DB error causes SePay retry storm + lock amplification
- **File:** `phase-06-sepay-webhook.md:80-82`
- **Attack:** Handler's err path `return c.Status(500)` on any server error triggers SePay's 7x Fibonacci-backoff retry. If the root cause is transient (e.g., connection pool saturated at high load, or `grant_credits` throws P0001 wrongly due to concurrent modification), every retry compounds lock contention on `wallets` row + `transactions` row. Worst case: 7 concurrent retries per failed webhook × 10 users = 70 inflight txns fighting for the same `wallets` lock → pool exhaustion cascade.
- **Impact:** Self-DoS during bank slowness spikes. Legitimate webhooks silently retried while the first attempt is still holding a lock.
- **Fix:** In `phase-06-sepay-webhook.md` §Handler flow step 4: Classify errors. For `context.DeadlineExceeded`, `pgconn.PgError{Code: "40P01"}` (deadlock) → return 500 (transient, retry OK). For unexpected logic errors → return `200 {"success":false,"reason":"internal_error"}` + audit_log event=`sepay_internal_error` + alert burst. Document in risk table. Add test in Suite D: inject fault mid-txn, assert only deadlock returns 500.

### [F4] Trial gate race: concurrent `/start` from same user bypasses phone-uniqueness pre-check
- **File:** `phase-02-user-service.md:60-65` (checkTrialGate) + `phase-02-user-service.md:197` (risk row "Race: 2 updates for same user hit trial grant")
- **Attack:** User A triggers `/start` twice within ~5ms (Telegram update duplication, manual double-tap via desktop+mobile clients). singleflight dedupes by `telegram_id` — but what if attacker uses two Telegram accounts with SAME `phone_e164`? Both `/start` flows enter at different tg_ids. singleflight doesn't collapse them. Both check `COUNT(*) FROM users WHERE phone_e164=$1 AND trial_used=TRUE` → both see 0 → both race to flip `trial_used=TRUE` and call `grant_credits` for 5 standard. The unique partial index `idx_users_phone_trial` will throw 23505 on the loser, but the loser has already called `grant_credits` inside the txn and will roll back on commit. However, if both transactions commit around the same sequence and the index error only fires on the second `UPDATE users SET trial_used=TRUE`, the first user succeeds and the second fails gracefully. THIS IS OK — unique index IS the safety net. BUT plan assumes `COUNT(*)` pre-check is the gate, which is wrong ordering.
- **Separate race:** If singleflight is held, a DIFFERENT user calling with same phone simultaneously still races (singleflight keyed on telegram_id, not phone).
- **Impact:** Medium. Defense-in-depth broken — partial index IS the real gate, but if future refactor removes the index thinking "pre-check is enough", trial abuse returns.
- **Fix:** (a) In `phase-02-user-service.md` §checkTrialGate: add comment/doc "This COUNT(*) is UX-friendly error; the `idx_users_phone_trial` partial unique index is the REAL enforcement. Treat 23505 on the UPDATE/INSERT as `TrialPhoneReused`." (b) Wrap the entire flow in `SELECT ... FROM users WHERE phone_e164=$1 FOR UPDATE` using the same txn to serialize concurrent attempts. (c) Add integration test to phase-10 Suite B: two concurrent `VerifyContactAndGrantTrial` with same phone/different tg_id → exactly 1 succeeds, 1 gets `TrialPhoneReused`.

### [F5] Admin grant audit → ledger ref_id chicken-and-egg breaks provenance
- **File:** `phase-08-admin-commands.md:33-35`
- **Attack/Bug:** Plan says "`wallet.Grant(tx, ..., event_type='admin_adjust', ref_type='audit_log', ref_id=audit_row_id)` + audit_log insert" — but the ref_id must be known BEFORE the Grant call (ledger row writes balance_after + ref_entity_id atomically). Audit_log row hasn't been inserted yet. To get `audit_row_id` we need to INSERT audit_log first, which means we're audit-logging BEFORE the grant actually succeeds. If grant fails (e.g., invalid pool), we have an orphan audit row claiming an action that didn't occur.
- **Impact:** Provenance broken for admin actions. Postmortems of "did admin grant credits?" show false-positive audit rows.
- **Fix:** Reorder in `phase-08-admin-commands.md`: (a) `BEGIN TXN` → (b) `INSERT audit_log RETURNING id` → (c) `wallet.Grant(... ref_id=auditID)` → (d) `COMMIT`. If grant fails → ROLLBACK drops audit row atomically. OR: use `ref_type='user', ref_id=target_user_id` in the ledger + metadata="admin:<admin_tgid>:reason=<text>", and still insert audit_log as separate side-record with `ref_type='ledger', ref_id=ledger_id RETURNING`. Document the pattern. Reject the "fail-open audit" pattern in `Risk table:Audit log insert fails silently after side-effect commits` — for admin grants, audit is INTEGRAL not best-effort.

### [F6] Suite D concurrent webhook test is under-specified — may pass on single-core, fail in prod
- **File:** `phase-10-integration-tests.md:120-131` (FireN helper) + `phase-10-integration-tests.md:42-52` (Suite D)
- **Attack:** Planner's own red flag #4 said "run Suite D with `-count=100 -cpu=1,2,4,8`" — this directive is NOT in the current plan's Todo List or Success Criteria or Makefile step. Current `FireN` uses `errgroup.Go` which GoSched's on its own cadence; with Go scheduler preferring M:N on multi-core, 10 goroutines may serialize before even hitting the DB under low contention. Race-condition test may pass on dev but fail in prod where CPU is saturated.
- **Impact:** False-pass gate. Race test gives false security signal.
- **Fix:** (a) Add explicit `runtime.Gosched()` between `eg.Go` dispatches in `FireN` OR use `sync.WaitGroup` with `sync.Barrier` to release all goroutines simultaneously via `close(chan)`. (b) Add Makefile target `test-integration-stress: go test -race -count=100 -cpu=1,2,4,8 -run TestWebhookRace ./internal/e2e/...`. (c) Add todo item to phase-10 + CI workflow entry. (d) Success criteria: "Suite D passes 100/100 iterations across `-cpu=1,2,4,8` matrix".

---

## High findings (should fix)

### [H1] SePay `subAccount` field ignored — multi-bank / sub-wallet webhook misrouting
- **File:** `phase-06-sepay-webhook.md:170` (Payload struct)
- **Issue:** SePay supports multi-account setups. If merchant has multiple bank accounts registered, webhook arrives with `subAccount` set. Plan ignores it. Attacker can register their own sub-account under our merchant umbrella and forward webhooks — our handler treats them as legit since Apikey matches the merchant (not per-account).
- **Fix:** In phase-06, assert `p.AccountNumber == cfg.SepayBankAcc` (our expected account number from env) after auth check. Mismatch → 200 success + audit_log event=`sepay_account_mismatch`. Add Suite D test case.

### [H2] No rate limit on `/webhooks/sepay` — brute-force Apikey + enumerate pending orders
- **File:** `phase-06-sepay-webhook.md:242-246` (Security Considerations)
- **Issue:** Plan says "IP allowlist DEFERRED to Phase 10 deploy". No Fiber-level rate limit on the webhook route either. Attacker who leaks Apikey (e.g., from accidentally-committed env file) can hammer the endpoint, enumerate order-code space (2^32 for 8-hex, exponentially faster with known pending patterns).
- **Fix:** (a) Apply Fiber `limiter.New(Max: 60/sec, KeyGenerator: c.IP)` to the webhook route — legit SePay won't burst > 10/sec. (b) `ConstantTimeCompare` on Apikey already; add secondary check: `if p.Gateway != "MBBank" && ... explicitly-listed gateways → 200 unknown_gateway`. (c) Document IP allowlist as BLOCKER for production deploy, not deferred.

### [H3] Order code UPPER(...) case-sensitivity mismatch between insert and regex
- **File:** `phase-05-transactions-topup.md:75` + `phase-06-sepay-webhook.md:173`
- **Issue:** Insert uses `strings.ToUpper(...)` → provider_ref stored UPPERCASE. Regex uses `(?i)` case-insensitive match — extracts captured group in original case (from user memo), then does `WHERE provider_ref=$1`. If user types "sbf topup abc12345" (lowercase), extracted = `abc12345`, query searches `provider_ref='abc12345'` but DB has `ABC12345`. No match.
- **Fix:** In `phase-06-sepay-webhook.md` handler flow step 3: `orderCode := strings.ToUpper(match[1])` before passing to `ProcessPaidTransaction`. Add Suite D test with mixed-case memo.

### [H4] FSM state race: ban check AFTER loadUser middleware but FSM Save can race
- **File:** `phase-01-bot-skeleton.md:50-53`
- **Issue:** Middleware chain: recover → logger → loadUser → banCheck → i18n → route. If admin bans user via `/admin ban` DURING that user's in-flight update, the `loadUser` call happens at update t=0, banCheck sees `is_banned=FALSE`, admin bans at t=100ms, user's handler completes side effects at t=150ms. Small window, but real.
- **Impact:** Low-probability but a banned user can complete 1 final transaction.
- **Fix:** Acceptable window; document in risk table. Mitigation: re-check ban inside transaction for money-flow commands (`/buy confirm`, `/topup`) via `SELECT is_banned FROM users WHERE id=$1` at start of txn. Reject if TRUE.

### [H5] `/regenkey` has no rate limit enforcement in plan despite risk table mentioning it
- **File:** `phase-03-key-service.md:148` (risk row, but no corresponding plan item)
- **Issue:** Risk table says "Rate-limit `/regenkey` in Redis `regen_rl:<user_id>` — 3/day cap". But §Requirements, §Architecture, §Implementation Steps, §Todo List have zero mention. It's mitigation-ghosted.
- **Fix:** Add explicit implementation step + todo item: "Rate-limit `/regenkey` invocation via `INCR regen_rl:<user_id> EXPIRE 86400`. On count > 3 → reply `regen_rate_limited` template." Add phase-10 Suite B test for cap enforcement.

### [H6] Telegram notify-goroutine can race with commit rollback
- **File:** `phase-06-sepay-webhook.md:90-93`
- **Issue:** `go notifyTelegramSuccess(...)` spawned BEFORE `tx.Commit()` completes IF caller puts goroutine inside the handler (plan shows goroutine AFTER return of ProcessPaidTransaction, so post-commit OK). BUT `go notifyTelegramSuccess(context.Background(), ...)` uses `context.Background()` — no cancellation. If process is shutting down (SIGTERM), goroutine outlives the context and can silently fail mid-flight. Worse: if notify fires before DB txn actually flushes to disk (WAL sync), user sees "paid" message but 1ms later DB crashes and loses the commit.
- **Impact:** Low probability but correctness issue at process shutdown.
- **Fix:** Use a goroutine-lifetime context scoped to server's root ctx (passed via Deps). Use `pgx.Pool.Ping()` before notify to confirm WAL flushed. Or accept as inherent eventual consistency — document in SECURITY.md.

### [H7] Cancel-then-buy exploits the `status='failed'` gap
- **File:** `phase-05-transactions-topup.md:208` (risk row "Rapid cancel-then-buy creates duplicate keys")
- **Issue:** Risk marked "Low/Low" but the cancel flow allows: (a) user creates pending → (b) cancels → status=failed → (c) partial index releases → (d) user creates new pending same package → (e) SePay webhook arrives late for ORIGINAL failed order → status='failed' NOT 'pending' → CAS fails → audit shows "sepay_replay". User now gets double-credit opportunity: PAY twice, expect both to credit, but only one pending matched. Attacker: pay → cancel via bot → pay again with NEW QR code → both bank transfers hit same merchant account → second one matches via new order code → first transfer's fate? Goes to `sepay_unmatched_transfer` audit with money pocketed but no credits.
- **Impact:** Edge case but real: user reports "I paid twice, got credits once" — support burden + potential refund obligation.
- **Fix:** Either (a) When user clicks cancel, mark tx as `'failed'` BUT KEEP the provider_ref reservation for 24h via metadata.cancel_ack_deadline — don't allow new pending with same amount_vnd within 10 min. OR (b) Document policy: cancelled orders that receive late payment go to `manual_review` for admin refund. Add Suite D test: create → cancel → receive matching webhook → assert audit + manual_review flag. Plan currently has no such test.

---

## Medium findings (nice to fix)

### [M1] Missing /campaigns command — spec §5.1 says Phase 2
- **File:** `docs/MASTER_PROMPT.md:1219` — "/campaigns (Phase 2)" vs plan command inventory
- **Issue:** Plan inventory covers: /start /key /regenkey /balance /buy /topup /history /support /download /ref /language /admin. NOT /campaigns. Spec explicitly tags it Phase 2.
- **Fix:** Either (a) add stub `/campaigns` replying "Campaigns coming in extension; this command will list your campaigns in Phase 3" in phase-07 (no DB read yet). OR (b) document scope-cut in plan.md §Success Criteria with justification (campaigns are tied to extension which lands Phase 3).

### [M2] Package code regex allows invalid combinations (e.g., `premium_starter_300`)
- **File:** `phase-05-transactions-topup.md:35`
- **Issue:** Regex `^(standard|premium)_(starter|basic|pro|max)_(50|100|200|300)$` matches 24 combos; Packages map defines only 8 — e.g., `premium_starter_300` matches regex, isn't a real package → map lookup fails, returns ErrUnknownPackage at runtime. Defense-in-depth is fine but regex is misleading.
- **Fix:** Drop regex; rely solely on `_, ok := Packages[code]`. Or make regex exhaustive-match actual keys. Minor.

### [M3] `/admin ban` self-ban guard missing
- **File:** `phase-08-admin-commands.md:38-44`
- **Issue:** Admin fat-fingers `/admin ban <own_tgid>`. `isAdmin` check happens at command dispatch — but the UPDATE runs regardless. Admin's own user row flips `is_banned=TRUE`. Next `/admin stats` → banCheck middleware short-circuits → admin locked out. Recovery requires direct DB access.
- **Fix:** Add check in `AdminService.Ban`: `if target_user.telegram_id IN cfg.AdminTelegramIDs → return ErrCannotBanAdmin`. Add Suite F test.

### [M4] `ip_hash` on webhook uses truncated SHA256 — collision trivial
- **File:** `phase-06-sepay-webhook.md:247`
- **Issue:** `ip_hash = sha256(c.IP())[:16]` — 16 hex chars = 64 bits. For PII-avoidance this is fine, but for anomaly detection (e.g., "seeing 1000 different IPs") it's vulnerable to birthday collision at ~4B distinct IPs. Negligible in practice, but if the same field is used for per-IP rate limit keys → trivial collision could merge two IPs' counters.
- **Fix:** Use full 32-char hex. Negligible storage cost.

### [M5] Support ticket body size unbounded
- **File:** `phase-07-history-support.md:180` + migration support_tickets.body is TEXT
- **Issue:** Plan doesn't cap ticket body length. User sends 100MB text → Telegram caps at 4096 chars (their limit), but bot receives chunks? Actually tgbotapi receives single message ≤ 4096 chars. But if bot receives 10 sequential messages while in `support_describing` state, each spawns a new ticket (no guard "only first message counts").
- **Fix:** In `phase-07` support.go: on state=`support_describing`, accept ONE message up to 4096 chars, then immediately clear state. Add test: spam 5 messages in describing state → only 1 ticket created.

### [M6] SePay payload `TransferAmount` handling — JSON numeric precision
- **File:** `phase-06-sepay-webhook.md:166`
- **Issue:** `TransferAmount int64` + JSON. Go's `json.Unmarshal` of a JSON number > 2^53 into int64 works via encoding/json's lazy decode, BUT SePay docs may send amount as STRING in some deployments (anecdotal from VN fintech integrations). Risk table mentions this (Low/High) but only one-line mitigation. No explicit test fixture with real payload.
- **Fix:** Add test fixture `testutil/sepay_payload_real.json` copied from SePay doc example. Assert parse succeeds + TransferAmount populated correctly. Add fallback: `json.RawMessage` on TransferAmount, custom Unmarshal trying int64 first then string.

### [M7] No metrics / alerting defined for webhook failure modes
- **File:** `phase-06-sepay-webhook.md:234` (risk row "Alert on 10+ consecutive sepay_auth_fail")
- **Issue:** Plan says alert on burst; where does the alert go? Phase 08 admin stats SHOWS the count but doesn't ALERT. Telegram DM to admin requires separate notify goroutine with threshold check. Not in plan.
- **Fix:** Add explicit implementation step to phase-08: cron-like goroutine runs every 60s, counts `audit_log WHERE event='sepay_auth_fail' AND created_at > NOW() - INTERVAL '5 minutes'`. If ≥ 10 → DM admin. OR note as deferred to ops phase.

---

## Low / observations

### [L1] Referral code 6-char base58 collision math
- `phase-07-history-support.md:173`: "34^6 = 1.5B". Base58 alphabet has 58 chars, not 34 (excludes 0OIl from 62). 58^6 ≈ 38B. Risk assessment note is correct direction, wrong base. Cosmetic.

### [L2] Key full length mismatch with spec §1.3
- `phase-03-key-service.md:15`: spec says 46 chars total, plan locks 41. Plan's rationale (9 prefix + 32 base58 = 41) is mathematically correct. Document as spec fix in plan.md, not a defect.

### [L3] `AdminTelegramIDs` env type — comma-separated or JSON array?
- `phase-08-admin-commands.md:13`: `cfg.AdminTelegramIDs []int64` — plan doesn't specify parsing format. Fly secrets are strings. Need explicit spec: `ADMIN_TELEGRAM_IDS=123,456,789` parsed via `strings.Split + Atoi`.
- **Fix:** Note in config.go modification: "parse comma-separated string, error on any non-int, log warn on dupes".

### [L4] `payloadJSON` in webhook could exceed metadata JSONB size
- `phase-06-sepay-webhook.md:113`: stores raw webhook payload into `transactions.metadata` (JSONB). Postgres JSONB soft-limit ~1GB; SePay payload ~1KB; no real risk. Observation only.

### [L5] Base58 alphabet excludes `0OIl` — stated correctly but plan uses 58-char string
- `phase-03-key-service.md:40`: `base58Alphabet` contains 58 chars (correct, excludes 0OIl). Naming consistent. OK.

### [L6] `/history` merges tx + ledger but uses independent offsets
- `phase-07-history-support.md:54-68`: `pageTx*5 + pageLedger*5` — independent pagination per kind. Confusing UX: user doesn't know which list they're paging. Observation; UX design call.

---

## Spec-vs-plan divergence

| Spec §          | Spec requirement                                                   | Plan handling                                                          | Status                               |
|------------------|--------------------------------------------------------------------|------------------------------------------------------------------------|--------------------------------------|
| §1.3 key length  | 46 chars total                                                     | 41 chars (9 prefix + 32 random)                                        | Plan overrides — document in plan.md |
| §1.3 packages    | 10 packages as defined                                             | Plan replicates all 10                                                 | OK                                   |
| §1.5 per-domain  | Per-domain rate limit                                              | Out of scope Phase 2 (extension job flow)                              | OK (deferred)                        |
| §1.5 fraud rule  | Same SePay tx ref retry > 2 → manual review                        | Plan gates on status not retry count                                   | **Gap** — need retry counter         |
| §1.6 key rotate  | Redis blocklist 24h on regen                                       | Deferred to Phase 3 per plan                                           | OK (with note)                       |
| §5.1 /campaigns  | Phase 2 command                                                    | Not in plan                                                            | **Gap** — see M1                     |
| §5.4 step 1 auth | "Verify bearer token"                                              | Plan uses Apikey (confirmed correct via SePay docs fetch)              | Plan correct, spec stale             |
| §5.4 step 4      | "Compare `transferAmount >= expected amount_vnd`"                  | Plan implements `<` check only, missing `>` (see F1)                   | **Gap**                              |
| §5.5 trial gate  | Account age < 30d check                                            | Plan uses phone-uniqueness (Bot API limitation justifies override)     | Plan overrides — justified           |
| §1.5 fraud rule  | "Same SePay transaction ref retry > 2 times → flag manual review" | Plan auto-retries via CAS; no explicit counter                         | **Gap** — H7 related                 |

---

## Test plan gaps

### Suite D missing cases
1. **Over-payment** (`transferAmount > amount_vnd`) — test case not present; see F1.
2. **Content memo pollution** — user-supplied memo like "tôi chuyển SBF TOPUP ABC12345 hihi" should extract correctly OR be rejected cleanly. Plan has "unmatched content" test only.
3. **Mixed-case order code in memo** — see H3. Test with "sbf topup abc12345" lowercase.
4. **AccountNumber mismatch** — see H1. Test with `accountNumber="999999"` (wrong) + valid auth + valid order → 200 success + audit `sepay_account_mismatch`, no credit grant.
5. **Gateway unknown** — `gateway="UnknownBank"` → how is it treated? Plan doesn't say.
6. **Transient DB error injection** — simulate deadlock on grant_credits; assert 500 returned and retry-idempotent; assert no partial credit granted.
7. **Order code collision** — pre-seed 2 pending transactions with identical `provider_ref` (force via direct SQL → should fail at insert due to UNIQUE), document expected behavior on collision at `CreateTopupIntent`.
8. **Cancel-then-webhook** — see H7. Cancel pending → webhook arrives for canceled order → assert manual_review path.

### Suite B (User/Trial) missing cases
1. **Concurrent phone race** — 2 tg_ids with same phone starting trial at same moment. Exactly 1 wins, other gets `TrialPhoneReused`.
2. **Banned admin self-rescue** — impossible by design once locked; doc expected recovery procedure.
3. **Contact share without prior `/start`** — FSM guard test.
4. **`/start ref_INVALID`** — referral code doesn't exist → trial still granted, ref not linked, no error.

### Suite F (Admin) missing cases
1. **Admin self-ban guard** — see M3.
2. **`/admin grant amount=0`** — should reject with "amount must be > 0".
3. **`/admin grant amount=10001`** — boundary at cap 10000 → reject.
4. **`/admin lookup +84999999999`** (non-existent phone) — returns "user not found" without leaking existence of other users.
5. **Banned user's `/admin stats`** — should banCheck short-circuit even for admin? Edge case: admin can't self-ban but can admin-A ban admin-B. Behavior undefined.

### Suite G (Security) missing cases
1. **Redis eviction mid-FSM** — flush Redis between `/buy` and confirm → user gets fresh idle state, no crash.
2. **Webhook body with deeply-nested JSON** — 100-level JSON object → BodyParser doesn't stack-overflow.
3. **Webhook content with Unicode/emoji** — "SBF TOPUP 🎉 ABC12345" → regex handling.
4. **Key plaintext in error message** — deliberately trigger error during `Issue`, grep stderr for "sbf_live_" → assert absent.

### Stress specs missing
- `-count=100 -cpu=1,2,4,8` directive absent from Makefile and CI. See F6.
- Load test (k6) punted to later phase — acceptable for Phase 2 scope.

---

## Approved items (planner did RIGHT)

1. **CAS pattern as idempotency gate is architecturally correct.** `UPDATE transactions WHERE status='pending' RETURNING` with row lock under READ COMMITTED gives exactly-once semantics without stored proc change. Mathematically clean.
2. **Package price snapshot into `transactions` row at creation.** Prevents mid-pending price change from corrupting grant amount. Good use of immutable data.
3. **Redis lock + DB unique partial index for topup idempotency (belt + suspenders).** Redis handles 99% traffic, DB catches the 1% race. Correct.
4. **Apikey (not Bearer) + `subtle.ConstantTimeCompare` + return 200-on-fail.** Correctly reconciled against SePay docs; avoids retry storm on auth probe.
5. **Phone-uniqueness trial gate replacing account-age (Bot API limitation).** Documented override, pragmatic choice.
6. **Audit log coverage across all branches.** `sepay_success | sepay_auth_fail | sepay_unmatched_transfer | sepay_underpaid | sepay_replay` is thorough.
7. **Body size limit 64KB on webhook.** Prevents slow-loris / memory exhaustion.
8. **Silent-ignore for non-admin `/admin *`** — correct info-leak prevention.
9. **Templates as Go code + compile-time key constants** — prevents typo-at-runtime bugs.
10. **Stored proc signature preservation** — avoids coordinated migration risk; adds only indexes.
11. **Notify goroutine post-commit** — correct ordering (not pre-commit).
12. **`/regenkey` FSM confirmation** — prevents accidental revocation via slash-spam.
13. **Masked key display `sbf_live_Zk3p...`** — prefix 12 chars visible, acceptable UX/security tradeoff.

---

## Residual risks accepted

1. **Phone recycling by VN carriers** — genuine user impact but low frequency; `phone_verified_at` re-verify deferred is acceptable.
2. **Best-effort Telegram notify** — user can self-serve `/balance` to see credits. No SLA claim.
3. **Package price change mid-pending** — snapshot approach is correct; document in SECURITY.md. Inconsistency with new pricing is accepted.
4. **Admin TG ID leak via env** — standard Fly secret hygiene covers it.
5. **tgbotapi v5 staleness** — pinned to stable lib; upgrade friction acceptable.
6. **Shared process bot+API** — single point of fate accepted for Phase 2 scale.

---

## Recommended pre-cook actions

1. **Amend phase-06-sepay-webhook.md** to add:
   - F1 fix: symmetric over/under-payment → `manual_review` on any `!=` mismatch.
   - F2 fix: extend order code to 12 hex + explicit case-upper normalize at regex extraction.
   - F3 fix: classify error types (deadlock → 500 retry; logic errors → 200 success=false with audit).
   - H1 fix: assert `AccountNumber` matches env.
   - H2 fix: Fiber rate limiter + gateway allowlist.
   - H3 fix: `strings.ToUpper(match[1])` before query.
   - H6 note: goroutine context lifecycle.
2. **Amend phase-05-transactions-topup.md** to change `orderCode = ToUpper(hex(uuid[:6]))` → 12 hex chars + test for 23505 on provider_ref UNIQUE.
3. **Amend phase-02-user-service.md**:
   - F4 fix: add `SELECT FOR UPDATE` on users row in VerifyContactAndGrantTrial; treat 23505 on index as PhoneReused.
4. **Amend phase-08-admin-commands.md**:
   - F5 fix: audit + grant atomically (insert audit → grant referring audit id → commit).
   - M3 fix: reject ban on admin tg_id.
5. **Amend phase-10-integration-tests.md**:
   - F6 fix: add `-count=100 -cpu=1,2,4,8` make target + CI step.
   - Add missing Suite D tests listed above.
   - Add missing Suite B concurrent phone race test.
6. **Amend phase-03-key-service.md**:
   - H5 fix: add explicit `/regenkey` rate-limit implementation step + test.
7. **Amend phase-07-history-support.md**:
   - M1 decision: add `/campaigns` stub OR document scope cut in plan.md.
   - M5 fix: cap support body to 4096 chars + one-shot state clear.
8. **Amend plan.md**:
   - Record key-length-41 spec override.
   - Record `/campaigns` scope decision.
   - Add SECURITY.md backlog entry for over-payment policy + refund path.
9. **Add docs/code-standards.md rule** (per planner's red flag #1): "Any call to `grant_credits` MUST be inside a pgx.Tx that also contains a status-gating UPDATE. Violations must be caught by `code-reviewer` via grep on `grant_credits(` → assert surrounding `WHERE status=` pattern within ±20 lines."
10. **Pre-cook smoke**: SePay sandbox POST with real payload shape to verify parse.

---

## Unresolved questions

1. **Over-payment refund policy**: Who handles refunds when user pays more than package price? Automated credit-excess OR manual admin refund? Product decision needed.
2. **Cancel-then-pay recovery path**: If user cancels a pending tx and pays anyway (late webhook), should system auto-refund or park in `manual_review`? See H7.
3. **SePay `subAccount` usage**: Is merchant account single or multi-bank? Affects H1 handling.
4. **SePay webhook max QPS**: Known from SePay? Needed to size rate limit in H2.
5. **Admin DM alerting mechanism**: Does Fly.io have a cron / scheduler we can use, or do we spawn a goroutine in main.go? See M7.
6. **`/campaigns` scope**: Confirm whether it ships as stub or fully scope-cut.

---

**Status:** DONE_WITH_CONCERNS
**Summary:** 6 critical / 7 high / 7 medium / 6 low findings. Verdict: FIX_REQUIRED — plan is architecturally sound (CAS idempotency is correct), but has 6 exploitable gaps that must be closed before cook.
**Report:** E:\tool_backlink\plans\reports\redteam-260424-2135-phase-2.md
**Top 3 blockers:**
1. F1 — over-payment path undefined (revenue leak)
2. F2 — 8-hex order code collision + regex case mismatch (cross-user credit risk)
3. F6 — Suite D race test under-specified (false-pass gate)
