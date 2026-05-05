# Code Review — Phase 09 Message Templates

## Verdict
APPROVED_WITH_FIXES

## Summary
- Refactor scope respected: locked Phase 02-08 service / handler / migration files diff clean.
- Templates package well-structured: 55 keys × 2 langs, sync.Map cache, VN-fallback chain, parse/exec error sentinels.
- 9 tests cover renderer + escape + bundle parity + secret-lint. All pass. Build + vet clean.
- Plaintext key NEVER logged. Plaintext rendered ONCE via `KeyStartVerifiedFirst` / `KeyKeyRegenDone` templates wrapped in backtick code-span (legacy Markdown safe for alphanumeric `sbf_live_` keys).
- FSM transitions preserved verbatim (Save / Load / Clear paths untouched).
- DB calls preserved verbatim (no service-layer touched).
- ParseMode shifted from `ModeHTML` → `"Markdown"` in start, regenkey, key. Bundle markup matches new mode. Pre-existing inconsistencies in topup_qr_caption (asterisks + `<code>` mixed under ModeHTML) are NOT regressions — exact same text shape as Phase 05.
- 2 high-priority regressions found in user-visible message content (resendTopupQR, download empty-URL). Both swap correct semantic copy for inappropriate template reuse.

## Behavior preservation audit
- [x] FSM transitions unchanged (verified cmd_start.go, cmd_regenkey.go, cmd_buy.go, cmd_buy_confirm.go, cmd_topup.go, cmd_topup_check.go, cmd_support.go)
- [x] DB calls unchanged (locked files diff empty)
- [x] Error semantics unchanged (handleTrialError still returns same errors per branch; cmd_regenkey rate-limit/INCR ordering preserved)
- [x] Plaintext key still shown once via template, never logged (grep confirms zero plaintext logging)
- [x] Markdown ParseMode preserved where applicable (HTML→Markdown swap intentional + matches new bundle markup)

## Critical findings
None. No regression on money flow, FSM gating, plaintext leakage, or auth bypass.

## High findings

### H1 — `resendTopupQR` uses wrong template (cmd_topup.go:65-73)
**Regression:** When user runs `/topup` while in `topup_waiting` state, the QR resend caption now uses `tplBuyConfirm` template:
```
📦 *<DisplayVI>*
💳 Số credits: ...
💰 Giá: *...đ*

Xác nhận thanh toán?    ← inappropriate; user already past confirm
```
Original (a385a18:cmd_topup.go:62-65):
```
💳 *Đơn hàng đang chờ thanh toán*
📦 Gói: %s
💰 Số tiền: *%sđ*
Quét QR hoặc chuyển khoản theo thông tin trên ảnh.
```
The new copy contradicts the user's actual state ("Confirm payment?" when they're waiting for SePay to confirm).
**Fix:** Add new template key `KeyTopupResendCaption` with the original "đang chờ thanh toán" copy, OR reuse `KeyTopupQRCaption` (which has bank info + transfer note already).

### H2 — `cmd_download.go:25-30` empty-URL message regression
**Regression:** When `INSTALLER_URL` is empty (installer not yet published), user previously received:
```
Installer chưa publish. Theo dõi announcement nhé.
```
Now receives:
```
⚠️ Đã xảy ra lỗi. Vui lòng thử lại sau.    (KeyErrorGeneric)
```
This misleads the user — it's not an error, the feature is just not yet available. Misclassifies a planned-state UX into a "system fault" frame.
**Fix:** Add dedicated key `KeyDownloadNotReady` ("Installer chưa publish. Theo dõi announcement nhé.") OR keep this branch inline (it is dev/early-state-only).

## Medium findings

### M1 — `HandleSupportCancelCallback` reuses `tplBuyCancelled` (cmd_support.go:153)
Semantically inappropriate cross-domain reuse. Both render to "Đã huỷ." today, but a future copy change to buy-cancel would silently bleed into support-cancel.
**Fix:** Add `KeySupportCancelled` OR keep inline. Low impact — string matches.

### M2 — Bundle has unused keys (10+ orphans)
Defined but never referenced by any handler:
- `KeyStartContactPrompt` — orphan (no handler)
- `KeyBuyPackageRow` — deferred (keyboard render uses inline)
- `KeyTopupWaiting` / `KeyTopupUnderpaid` — Q2 deferred (Phase 06 webhook would emit)
- `KeyTopupOverpaidOK` / `KeyTopupRecovered` — Q2 deferred
- `KeyHistoryTxRow` / `KeyHistoryLedgerRow` / `KeyHistoryPageFooter` — Q3 deferred
- `KeyErrorUnknownCommand` / `KeyErrorInsufficientCredits` — orphan
- `KeyAdminStats` / `KeyAdminLookupResult` — passthrough `{{.Body}}` (render funcs build body inline; templates effectively unused)
- `KeyAdminUnknownSubcmd` — orphan (handler uses `tplAdminMenu` for unknown subcmd)

Q2/Q3 entries are acceptable scope deferrals. The remaining ~5 are dead bundle entries — keep for the spec'd 55-key contract OR add deletion to a follow-up. No behavior impact.

### M3 — `KeyTrialPhoneReused` and `KeyStartTrialBlockedReuse` are duplicates
Both exist in bundles + AllKeys with near-identical Vietnamese copy:
- `KeyTrialPhoneReused` (used by `handleTrialError` for `ErrTrialPhoneReused`)
- `KeyStartTrialBlockedReuse` (orphan)

DRY violation. Pick one (likely `KeyTrialPhoneReused` since it has a wired call site) and remove the other.

### M4 — `EscapeMDV2` defined and tested but never called at any site
The escape helper covers all 18 MarkdownV2 special chars and is correctly tested, but no call site uses it. All current `ParseMode` usage is legacy `"Markdown"` or `"ModeHTML"`, not `"MarkdownV2"`. Either:
- Add MarkdownV2 migration in a follow-up phase (then escape becomes useful), OR
- Remove the helper as YAGNI.
Acceptable to keep for forward planning. Document the intent in escape.go.

## Low / nits

### L1 — `KeyKeyShow` template lost the "Prefix được hiển thị để xác nhận key — plaintext không thể lấy lại." explanation
Old (cmd_key.go @ a385a18):
```
Prefix được hiển thị để xác nhận key — plaintext không thể lấy lại.
Dùng <b>🔄 Regenerate</b> để tạo key mới (key cũ sẽ bị thu hồi).
```
New (`KeyKeyShow`):
```
Plaintext không thể lấy lại. Dùng *🔄 Regenerate* để tạo key mới (key cũ sẽ bị thu hồi).
```
Slightly less informative but still factually correct. Acceptable copy compression.

### L2 — `cmd_history.go` deferred refactor is OK but bundle entries `KeyHistoryTxRow / KeyHistoryLedgerRow / KeyHistoryPageFooter` use `{{.StatusIcon}}` / `{{.Display}}` template fields that would need handlers' format funcs to be rewritten as struct field providers. Q3 flagged — acceptable.

### L3 — Comment on cmd_admin.go:226-228 claims `adminHelpText` was removed but the function never existed in the diff (only the constant). Minor doc cleanup if desired.

### L4 — `messages.go` exceeds 200-line cap (412 lines). Implementer documented justification (single-registry property, content density). Acceptable per the file-comment rationale. Treat as data file.

## Plan spec alignment

Per phase-09-message-templates.md spec mandate:
- [x] Strict scope = swap inline strings for `tmpl.Render(lang, key, data)` — mostly held; 23 inline strings remaining are justified (admin dev-CLI, dev stubs, fallback paths).
- [x] No new behavior, no new logic — held EXCEPT for H1/H2 above (template-substitution chose semantically-wrong template, not a true new behavior but a copy mismatch).
- [x] FSM transitions / DB calls / error returns / ParseMode untouched (modulo intentional HTML→Markdown swap with matching markup).
- [x] Plaintext key shown ONCE via template; never logged.
- [x] Bundle parity test enforces VN+EN entries for all 55 keys.

## Test adequacy

| Test | Coverage |
|------|----------|
| TestRender_KnownKey | Pass — KeyBalance interpolates correctly |
| TestRender_UnknownKey | Pass — bracket sentinel `[<key>]` |
| TestRender_FallbackVI | Pass — EN-miss falls to VI |
| TestRender_AllKeysHaveBothBundles | Pass — every AllKeys entry has VN+EN |
| TestRender_DataInterpolation | Pass — struct-field interpolation works |
| TestEscapeMDV2_AllChars | Pass — all 18 chars escaped |
| TestRender_CacheHit | Pass — 16 goroutines × 62 renders no panic |
| TestRender_NilLogger | Pass — nil → nop |
| TestNoSecretsInBundles | Pass — no `sbf_live_`, conn strings, secrets |
| TestAllKeysListMatchesBundles | Pass — AllKeys ↔ vi bundle bijective |

Race detector skipped (Windows no GCC) — sync.Map is stdlib-documented goroutine-safe; loose check via concurrent test is reasonable.

Coverage gaps:
- No test for `tplBuyConfirm` interpolation (would have caught H1 if asserting "Xác nhận thanh toán?" appears only in confirm flow not resend).
- No integration test for `renderTplCtx(deps=nil)` returning `[<key>]` literal — covered by unit tests of helpers facade.
- No test that locked Phase 02-08 service file imports remain unchanged.

## Approved items

- Renderer correctness: cache → bundle → VI fallback → bracket sentinel chain implemented per spec.
- Bundle integrity: 55 keys × 2 langs, all `text/template` syntactically valid (vet + tests pass).
- Concurrency: sync.Map + parsed `*template.Template` shared safely.
- Markdown V2 escape helper: complete coverage of 18 reserved chars (currently unused — see M4).
- Helpers facade `cmd_helpers_template.go`: nil-guard returns `[<key>]` placeholder; LangFromCtx pulls user language correctly.
- Locked file changes: ZERO diff in service / handler / migration files. Refactor scope respected.
- Plaintext security: zero plaintext-logging surfaces (grep clean across all bot/*.go and templates/*.go).
- Test infrastructure: parity test, secret-lint test, AllKeys list cross-check — all enforce regression boundary.

## Recommended actions

1. **[H1] Fix `resendTopupQR` caption** — add `KeyTopupResendCaption` with original "Đơn hàng đang chờ thanh toán" copy OR reuse `KeyTopupQRCaption` (preferred — already has bank/order details).
2. **[H2] Fix `cmd_download.go` empty-URL** — add `KeyDownloadNotReady` ("Installer chưa publish. Theo dõi announcement nhé.") OR revert that branch to inline string.
3. **[M3] Resolve duplicate** — pick `KeyTrialPhoneReused` (wired) or `KeyStartTrialBlockedReuse` (unwired); remove the orphan from keys.go + AllKeys + both bundles.
4. **[M1] Optional:** Add `KeySupportCancelled` for clean ownership; defer if scope-tight.
5. **[M4] Decide:** Keep `EscapeMDV2` for forward MarkdownV2 migration (document intent) OR remove as YAGNI.
6. **[M2] Optional:** Delete dead bundle entries (`KeyStartContactPrompt`, `KeyErrorUnknownCommand`, `KeyErrorInsufficientCredits`, `KeyAdminUnknownSubcmd`) OR keep for spec contract continuity. No behavior impact either way.

## Unresolved questions

- Q1: Confirm the `KeyStartVerifiedFirst` removed-prefix-snippet behavior change is product-approved (implementer flagged as intentional — needs PM/copy nod).
- Q2: Should H1+H2 be addressed in this phase (1 fix-loop) or deferred to a Phase 09.1 patch? Phase scope is refactor-only, but H1/H2 are user-visible regressions in copy semantics — recommend fix-in-phase.
- Q3: Phase 06 webhook still has inline `"sepay webhook: payment success — notify placeholder"`. Implementer flagged for future amend. Track as Phase 06.1 follow-up.
- Q4: `messages.go` at 412 lines: keep as data-file exception, or split per-feature in a follow-up (would defeat single-registry property)?
