# Phase 07 — History, Support, Download, Ref, Language

## Context Links
- Spec: `docs/MASTER_PROMPT.md` §5.1 commands
- Migration from phase 02 creates `support_tickets` and `referrals` tables

## Overview
- **Priority:** P2 (non-blocking for core money flow)
- **Status:** pending
- **Description:** 5 read-heavy / light-write commands. No complex state, no money flow — fast to implement once earlier services exist.

## Key Insights
- `/history` merges 2 data sources (transactions + ledger of consume events) — need UNION-like presentation, not a single SQL query.
- `/support` stores tickets in DB (no external help-desk integration this phase).
- `/download` — static link to R2 CDN (installer URL from env; Phase 7 actually populates the installer).
- `/ref` — generate unique referral code (6-char base58, uppercase, Redis-checked for collision), store in `referrals`.
- `/language` — toggle VN/EN, persist in `users.language`.

## Requirements

### `/history`
- Show last 5 transactions (status icon + amount + package) + last 5 ledger consume events (job / captcha / finder).
- Inline pagination buttons: `◀ prev | next ▶` (page size 5 each).
- FSM state NOT needed — stateless, page index encoded in callback data `hist:tx:2` or `hist:ledger:3`.

### `/support`
- Show FAQ menu (inline buttons: "Thanh toán", "Key", "Credits", "Kỹ thuật", "Khác"). Each button edits message with canned FAQ text + "Vẫn cần hỗ trợ? Bấm đây" button.
- "Vẫn cần hỗ trợ" → state=`support_describing`, prompt "Mô tả vấn đề của bạn".
- Next text message in this state → **[M5] truncate body to 4096 chars (Telegram max)**, INSERT support_ticket, **[M5] IMMEDIATELY clear state to `idle`** (one-shot consumption — no further messages open new tickets), reply "Đã gửi. Ticket #XXXX. Ad sẽ reply trong 24h".
- Max 3 open tickets per user (cap spam).
- State TTL 30min; `/cancel` also clears.

> **[Q6] Note on `/campaigns`:** deferred to Phase 4+ (requires Campaign API CRUD endpoints not landing until Phase 4). NOT routed in Phase 2. Any `/campaigns` text → falls through to unknown-command handler (already covered by router default).

### `/download`
- Reply: "📥 Tải installer Windows: {url}" with url from env `INSTALLER_URL` (default = `{INSTALLER_CDN_URL}/installer/SnakeBacklinkSetup.exe` — `INSTALLER_CDN_URL` resolved in Phase 7-9 when Cloudflare R2 bucket + public base URL finalized; Phase 2 may set this to a `.fly.dev/static/` stub or leave unset to trigger "installer not yet published" reply).
- Inline button "Hướng dẫn cài đặt" → edit to installation guide text.

### `/ref`
- Lookup `referrals.user_id = user.id`; if missing → generate code + insert.
- Show bot deep-link `t.me/SnakeBacklinkBot?start=ref_XXXXXX`.
- Stats: `SELECT total_referred FROM referrals WHERE user_id=$1`.
- Note: `/start ref_XXXXXX` parsing handled in phase 02 — update `users.referred_by` if valid code; bump `referrals.total_referred`. Deferred to phase 02 amend note.

### `/language`
- Inline keyboard [🇻🇳 Tiếng Việt | 🇬🇧 English].
- Callback `lang:set:vi|en` → `UPDATE users SET language=$1`, reply in new language "Đã đổi / Language changed".

### Non-functional
- `/history` query uses LIMIT 5 OFFSET $n, indexed by (user_id, created_at DESC) already present.
- Pagination over 500 records → UX skip to "Last page" button (simple `COUNT(*)`-based).
- Ref code generation: 3 retries on Redis collision check, then fall back to 8-char code.

## Architecture

### History render pseudocode
```go
func (h *HistoryCmd) Handle(ctx, user, pageTx, pageLedger int) (Reply, error) {
    txs, err := q.GetTxByUserPage(ctx, user.ID, 5, pageTx*5)
    consumes, err := q.GetLedgerConsumesByUserPage(ctx, user.ID, 5, pageLedger*5)

    body := tmpl.Render(user.Lang, "history_header")
    for _, t := range txs {
        body += tmpl.Render(user.Lang, "history_tx_row", t)
    }
    body += tmpl.Render(user.Lang, "history_ledger_header")
    for _, l := range consumes {
        body += tmpl.Render(user.Lang, "history_ledger_row", l)
    }
    return reply(body, paginationKeyboard(pageTx, pageLedger, totalTx, totalLedger))
}
```

### Ref code generation (`service/referral_service.go`)
```go
func (s *RefService) EnsureCode(ctx, userID UUID) (string, error) {
    existing, err := s.q.GetReferralByUser(ctx, userID)
    if err == nil { return existing.Code, nil }
    if !errors.Is(err, pgx.ErrNoRows) { return "", err }

    for i := 0; i < 3; i++ {
        code := util.GenBase58Upper(6)
        _, err := s.q.InsertReferral(ctx, userID, code)
        if err == nil { return code, nil }
        if pgErr := asPgError(err); pgErr.Code == "23505" { continue } // collision, retry
        return "", err
    }
    code := util.GenBase58Upper(8) // fallback longer
    _, err = s.q.InsertReferral(ctx, userID, code)
    return code, err
}
```

## Related Code Files
### Create
- `services/api/internal/bot/commands/history.go`
- `services/api/internal/bot/commands/support.go`
- `services/api/internal/bot/commands/download.go`
- `services/api/internal/bot/commands/ref.go`
- `services/api/internal/bot/commands/language.go`
- `services/api/internal/service/support_service.go`
- `services/api/internal/service/referral_service.go`
- `services/api/internal/db/queries/support.sql`
- `services/api/internal/db/queries/referrals.sql`

### Modify
- `services/api/internal/db/queries/ledger.sql` — add `GetLedgerConsumesByUserPage` (filters `event_type IN ('consume_backlink','consume_captcha','consume_finder')`)
- `services/api/internal/config/config.go` — add `InstallerURL` env (default `{INSTALLER_CDN_URL}/installer/SnakeBacklinkSetup.exe`; `INSTALLER_CDN_URL` resolved Phase 7-9). If env empty in Phase 2, `/download` replies "installer chưa publish — theo dõi announcement".
- `services/api/internal/bot/router.go` — route new commands + callbacks

## sqlc queries (new)
```sql
-- services/api/internal/db/queries/support.sql

-- name: InsertSupportTicket :one
INSERT INTO support_tickets (user_id, subject, body) VALUES ($1, $2, $3) RETURNING *;

-- name: CountOpenTicketsByUser :one
SELECT COUNT(*) FROM support_tickets WHERE user_id = $1 AND status IN ('open','in_progress');

-- services/api/internal/db/queries/referrals.sql

-- name: GetReferralByUser :one
SELECT * FROM referrals WHERE user_id = $1;

-- name: GetReferralByCode :one
SELECT * FROM referrals WHERE code = $1;

-- name: InsertReferral :one
INSERT INTO referrals (user_id, code) VALUES ($1, $2) RETURNING *;

-- name: IncrementReferralCount :exec
UPDATE referrals SET total_referred = total_referred + 1 WHERE user_id = $1;
```

## Implementation Steps
1. Extend `db/queries/ledger.sql` with `GetLedgerConsumesByUserPage`.
2. Write `db/queries/support.sql` + `referrals.sql`; `sqlc generate`.
3. Write `service/support_service.go` — Create + CountOpen.
4. Write `service/referral_service.go` — EnsureCode + ProcessReferralOnStart(ctx, newUserID, refCode).
5. Write `bot/commands/history.go` — pagination logic, merged render.
6. Write `bot/commands/support.go` — FAQ menu + FSM for describe-ticket.
   - **[M5]** On state=`support_describing`, accept ONE text message: cap body to 4096 chars via `[:min(len, 4096)]`, INSERT ticket, then `state.Clear()` BEFORE sending confirmation reply. Subsequent text from same user → falls through to default handler (no further ticket creation).
7. Write `bot/commands/download.go`.
8. Write `bot/commands/ref.go`.
9. Write `bot/commands/language.go`.
10. Amend phase 02 `bot/commands/start.go`: parse `/start ref_XXXXXX` parameter, call `RefService.ProcessReferralOnStart`.
11. Unit test: ref code uniqueness (1000 samples, 0 collisions in 6-char space is acceptable — collision handled by retry).
12. Integration test: `/history` across 12 tx + 20 ledger rows → pagination correct, no duplication.
13. Integration test: `/support` 3-ticket cap enforced.
14. **[M5]** Integration test: user in `support_describing` state sends 5 text messages rapidly → exactly 1 ticket created, remaining messages hit default handler.
15. **[M5]** Integration test: ticket body length > 4096 → truncated to 4096 in DB.

## Todo List
- [ ] Add `GetLedgerConsumesByUserPage` to ledger queries
- [ ] Write `support.sql` + `referrals.sql`
- [ ] Run `sqlc generate`
- [ ] Implement support + referral services
- [ ] Implement 5 command handlers
- [ ] Amend `start.go` for `/start ref_XXX` parameter
- [ ] Wire router + callbacks
- [ ] Unit test ref code generation
- [ ] Integration test history pagination
- [ ] Integration test support ticket cap
- [ ] **[M5]** Integration test one-shot ticket state + 4096-char body cap
- [ ] Integration test language toggle persists

## Success Criteria
- `/history` pagination stable (no missed/duplicated rows across page flips)
- `/support` caps at 3 open tickets per user
- `/ref` returns same code on repeat calls (idempotent)
- `/language en` → subsequent `/balance` in English
- `go test ./internal/bot/commands/... -race` green

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| History query slow on users with 10k+ rows | Low | Low | Index `ledger(user_id, created_at DESC)` already exists; LIMIT 5 cheap |
| Support FAQ spam clicks fill DB | Low | Low | Only "Vẫn cần hỗ trợ" → DB write; FAQ clicks = message edit only |
| Ref code collision in 6-char base58 (34^6 = 1.5B) | Very Low | Low | 23505 retry loop + 8-char fallback |
| Language toggle race: message in old lang while toggle in-flight | Low | Low | Harmless cosmetic; `singleflight` from phase 01 serializes per-user |
| `/download` URL points to non-existent installer until Phase 7 | Certain | Low | Reply includes "Đang cập nhật Phase 7" banner when env flag set |
| Support `support_describing` state stuck → next unrelated text opens ticket | Low | Med | **[M5]** State cleared BEFORE reply is sent on first accepted message (one-shot). 30min TTL + `/cancel` as backstops. Spam after that → default handler, no extra tickets |
| Support ticket body unbounded size (DDoS via concatenated paste) | Low | Low | **[M5]** Truncate body to 4096 chars at insert time (matches Telegram max) |

## Security Considerations
- **[M5]** Support ticket body truncated to 4096 chars on insert (matches Telegram per-message cap). Stored otherwise AS-IS (user controls). Render via template with HTML escape for future admin dashboard (Phase 8).
- Installer URL content-type not validated this phase — Phase 7 signs installer, user verifies via Windows Authenticode.
- Ref code path: `ProcessReferralOnStart` idempotent — `users.referred_by` write-once (`UPDATE ... WHERE referred_by IS NULL`).

## Next Steps
- Phase 08 admin `/admin stats` reads `support_tickets` queue count.
- Phase 3 (extension) reads `users.language` for UI locale.
