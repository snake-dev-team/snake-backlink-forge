# Phase 09 — Message Templates (VN + EN)

## Context Links
- Spec: `docs/MASTER_PROMPT.md` §5.3 message templates
- Bot copywriter (VN "bắt tai") handled in `/ck:cook` copywriter agent; THIS PHASE defines structure + key inventory + English fallback scaffolding only

## Overview
- **Priority:** P2 (written last — command handlers written against key names, copy filled here)
- **Status:** pending
- **Description:** Central template registry. `map[lang]map[key]string` + `text/template` for interpolation. VN default, EN fallback when VN missing (unlikely but defensive).

## Key Insights
- Templates as Go code (not YAML/JSON): compile-time key safety via constants, grep-ability from handlers.
- `text/template` for interpolation — supports `.Premium`, `.StandardCredits` with zero dependencies beyond stdlib.
- VN tone: "bắt tai", emoji-rich, informal "bạn/ad" — copywriter agent fills in `/ck:cook`.
- EN tone: professional, concise, MinimalInter aesthetic — copywriter agent fills.
- This phase ships: (a) key constants, (b) renderer, (c) VN placeholder strings, (d) EN placeholder strings. Final copy in `/ck:cook copywriter` stage.

## Requirements
### Functional
- `tmpl.Render(lang, key, data) string` — parses template cache on first use, interpolates data.
- Fallback chain: `lang` → `vi` (default) → key-as-literal (with log warn).
- Telegram MarkdownV2 escape helper for user-generated data.

### Non-functional
- Zero deps (stdlib only).
- Template parse happens once per (lang, key) — cached in `sync.Map`.
- Bench target: 100k renders/sec on dev machine.

## Key Inventory (minimum required)

### User-facing (per-lang)
| Key | Purpose | Sample data fields |
|---|---|---|
| `start_welcome` | First /start greeting | `{BotName}` |
| `start_contact_prompt` | Ask for phone share | — |
| `start_verified_first` | Trial granted success | `{Key, StandardCredits, BotName}` |
| `start_verified_repeat` | Already verified greeting | `{KeyPrefixMasked}` |
| `start_trial_blocked_phone_reuse` | Phone already used | — |
| `start_trial_blocked_banned` | Banned account | — |
| `start_contact_rejected` | Contact from wrong user | — |
| `trial_phone_reused` | **[F4]** Friendly error: phone already consumed trial (alias of blocked_phone_reuse surfaced via user_service error mapping) | — |
| `key_show` | `/key` response | `{KeyPrefixMasked, LastUsedAt}` |
| `key_regen_confirm` | Confirm regenkey prompt | — |
| `key_regen_done` | New key issued | `{Key}` |
| `key_regen_cancelled` | User cancelled | — |
| `regen_rate_limited` | **[H5]** /regenkey cap 3/day hit | `{ResetIn}` (optional) |
| `balance` | `/balance` response | `{Premium, Standard, TotalVNDSpent}` |
| `buy_menu_header` | `/buy` menu title | — |
| `buy_package_row` | Single package row button label | `{DisplayName, PriceVND, Credits}` |
| `buy_confirm` | Pre-confirm summary | `{Package.Display, Package.AmountVND}` |
| `buy_cancelled` | User cancelled | — |
| `topup_qr_caption` | QR photo caption | `{OrderCode, AmountVND, BankCode, Acc, DescText}` |
| `topup_waiting_notice` | "Đang chờ SePay xác nhận..." | — |
| `topup_paid_success` | Webhook notified | `{Premium, Standard, Package.Display}` |
| `topup_underpaid` | Underpayment manual review | — |
| `topup_cancelled` | User cancelled pending | — |
| `topup_overpaid_success` | **[Q1]** Overpaid — base + bonus credits granted | `{Premium, Standard, BonusCredits, BonusPool, ExcessVND, Package.Display}` |
| `topup_recovered_late_payment` | **[Q2]** Late payment recovered from cancelled order | `{Premium, Standard, Package.Display}` |
| `history_empty` | No history | — |
| `history_tx_row` | Single tx row | `{Status, Display, AmountVND, CreatedAt}` |
| `history_ledger_row` | Single ledger event row | `{EventType, Pool, Delta, CreatedAt}` |
| `history_page_footer` | Pagination footer | `{PageTx, PageLedger, TotalTx, TotalLedger}` |
| `support_menu` | FAQ menu intro | — |
| `support_faq_payment` | FAQ canned text | — |
| `support_faq_key` | FAQ canned | — |
| `support_faq_credits` | FAQ canned | — |
| `support_faq_technical` | FAQ canned | — |
| `support_describe_prompt` | Ask user to type issue | — |
| `support_ticket_submitted` | Confirmation | `{TicketID}` |
| `support_ticket_cap_reached` | 3-ticket cap | — |
| `download_text` | /download response | `{InstallerURL}` |
| `ref_show` | /ref response | `{Code, DeepLink, TotalReferred}` |
| `language_menu` | /language prompt | — |
| `language_set_vi` | "Đã chuyển sang VN" | — |
| `language_set_en` | "Switched to English" | — |
| `error_generic` | Unexpected error | — |
| `error_account_disabled` | Banned user fallback | — |
| `error_unknown_command` | Unknown /cmd | — |
| `error_insufficient_credits` | Consume failure (phase 3 will also use) | `{Pool, Needed, Have}` |

### Admin-facing (VN only — admin is the dev team, English comments acceptable but VN UI OK)
| Key | Purpose |
|---|---|
| `admin_menu` | /admin no-subcmd help |
| `admin_stats` | Formatted stats reply | data: full AdminStats struct |
| `admin_grant_ok` | |
| `admin_ban_ok` | |
| `admin_unban_ok` | |
| `admin_lookup_result` | |
| `admin_error_user_not_found` | |
| `admin_self_ban_blocked` | **[M3]** Attempt to ban an admin tg_id → rejected | — |
| `admin_unknown_subcmd` | |

## Architecture

```go
// bot/templates/keys.go
package templates

const (
    KeyStartWelcome         = "start_welcome"
    KeyStartVerifiedFirst   = "start_verified_first"
    // ... all keys as constants

    // Round-2 additions (placeholder copy; copywriter agent fills in /ck:cook)
    KeyTopupOverpaidSuccess       = "topup_overpaid_success"        // [Q1]
    KeyTopupRecoveredLatePayment  = "topup_recovered_late_payment"  // [Q2]
    KeyTrialPhoneReused           = "trial_phone_reused"            // [F4]
    KeyRegenRateLimited           = "regen_rate_limited"            // [H5]
    KeyAdminSelfBanBlocked        = "admin_self_ban_blocked"        // [M3]
)

// Placeholder copy (added to `bundles["vi"]` and `bundles["en"]`):
// topup_overpaid_success:        "TODO: VN copy — overpaid bonus granted"   / "TODO: EN copy — overpaid bonus granted"
// topup_recovered_late_payment:  "TODO: VN copy — late payment recovered"   / "TODO: EN copy — late payment recovered"
// trial_phone_reused:            "TODO: VN copy — phone already used trial" / "TODO: EN copy — phone already used trial"
// regen_rate_limited:            "TODO: VN copy — /regenkey rate limited"   / "TODO: EN copy — /regenkey rate limited"
// admin_self_ban_blocked:        "TODO: VN copy — cannot ban admin"         / "TODO: EN copy — cannot ban admin"

// bot/templates/messages.go
package templates

var bundles = map[string]map[string]string{
    "vi": {
        KeyStartWelcome: `🐍 *Chào mừng đến Snake Backlink Forge!*

Tool automation xây backlink chuyên nghiệp, dùng qua Chrome/Edge extension.

Để bắt đầu, hãy bấm nút chia sẻ số điện thoại bên dưới.`,
        // ... rest filled by copywriter
    },
    "en": {
        KeyStartWelcome: `🐍 *Welcome to Snake Backlink Forge!*

Professional backlink automation via our Chrome/Edge extension.

To begin, tap the button below to share your phone number.`,
        // ... rest filled by copywriter
    },
}

// bot/templates/renderer.go
package templates

type Renderer struct {
    log   *zap.Logger
    cache sync.Map // key: lang+"/"+tmplKey → *template.Template
}

func (r *Renderer) Render(lang, key string, data any) string {
    cacheKey := lang + "/" + key
    if v, ok := r.cache.Load(cacheKey); ok {
        return execTemplate(v.(*template.Template), data)
    }
    src, ok := bundles[lang][key]
    if !ok {
        // fallback to vi
        src, ok = bundles["vi"][key]
        if !ok {
            r.log.Warn("missing template key", zap.String("key", key))
            return "[" + key + "]"
        }
    }
    t, err := template.New(key).Parse(src)
    if err != nil {
        r.log.Error("template parse fail", zap.String("key", key), zap.Error(err))
        return "[parse_error:" + key + "]"
    }
    r.cache.Store(cacheKey, t)
    return execTemplate(t, data)
}

func execTemplate(t *template.Template, data any) string {
    var sb strings.Builder
    if err := t.Execute(&sb, data); err != nil {
        return "[exec_error]"
    }
    return sb.String()
}

// bot/templates/escape.go — Telegram MarkdownV2 escape
var mdV2Special = []string{`_`, `*`, `[`, `]`, `(`, `)`, `~`, "`", `>`, `#`, `+`, `-`, `=`, `|`, `{`, `}`, `.`, `!`}

func EscapeMDV2(s string) string {
    for _, c := range mdV2Special { s = strings.ReplaceAll(s, c, `\`+c) }
    return s
}
```

## Related Code Files
### Create
- `services/api/internal/bot/templates/keys.go`
- `services/api/internal/bot/templates/messages.go`
- `services/api/internal/bot/templates/renderer.go`
- `services/api/internal/bot/templates/escape.go`
- `services/api/internal/bot/templates/renderer_test.go`

### Modify
- All `bot/commands/*.go` files — replace inline strings with `tmpl.Render(lang, templates.KeyXxx, data)` calls

## Implementation Steps
1. Write `keys.go` constants — single source of truth for key names.
2. Write `messages.go` with placeholder copy for both VN and EN (copywriter agent refines in `/ck:cook`).
3. Write `renderer.go` with sync.Map cache + fallback chain.
4. Write `escape.go` for MarkdownV2 user-data escape.
5. Inject `*Renderer` into `Deps` struct from phase 01.
6. Refactor all command handlers to use `Render` calls.
7. Unit test renderer: known key renders, unknown key returns bracket-literal, fallback chain lang=en key-missing→vi, data interpolation works.
8. Lint rule via test: for every constant in `keys.go`, both `vi` and `en` bundles must have an entry (test iterates constants, asserts presence).

## Todo List
- [ ] Write `keys.go` with all 40+ constants
- [ ] Write `messages.go` with placeholder VN + EN
- [ ] Write `renderer.go` with cache
- [ ] Write `escape.go`
- [ ] Unit test renderer
- [ ] Unit test: every key has VN + EN entry
- [ ] Inject into Deps
- [ ] Refactor command handlers to use Render
- [ ] Verify MarkdownV2 escape tested against Telegram sandbox manually

## Success Criteria
- All handlers reference templates via constants (no inline strings except fmt.Sprintf for numbers)
- Test: every key const has VN + EN entry
- Benchmark: 100k renders/sec (sanity)
- `go vet ./internal/bot/templates/...` clean

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Copywriter adds keys not in `keys.go` | Med | Low | Compile fails because handlers reference const; copywriter edits both files |
| MarkdownV2 escape missed → user input breaks message parse | Med | Med | Test all fields that embed user input (phone, ticket body, usernames) pass through EscapeMDV2 |
| Language fallback infinite loop if VN also missing key | Very Low | Low | Second fallback returns bracket-literal, no recursion |
| Template cache unbounded growth | Very Low | Low | Total keys ~50 × 2 langs = 100 — trivial memory |
| Copy change requires code redeploy | Certain | Low | Accepted — copy is product, code as source is OK for 1k users scale |

## Security Considerations
- `EscapeMDV2` MUST be called on any user input embedded in message (phone, username, ticket subject/body, ref code display).
- Templates NEVER include raw SQL, keys, or tokens. Linter check via test: scan bundles for `sbf_live_`, `Apikey`, `DATABASE_URL` → fail.
- Bundle strings committed to repo; no secrets.

## Next Steps
- Copywriter agent refines VN "bắt tai" copy in `/ck:cook`.
- Phase 10 tests assert key messages appear in integration flow.
