// cmd_helpers_template.go — Phase 09: thin facade over templates.Renderer.
//
// All cmd_*.go handlers call renderTpl(deps, lang, key, data) instead of touching
// deps.Templates directly. This:
//   1. Centralizes the deps.Templates == nil fallback (returns inline-equivalent
//      string from a tiny VN-only emergency map so dev-mode without main.go
//      wiring still works).
//   2. Avoids importing the templates package in every cmd_*.go file (only
//      this file depends on the templates symbol; handlers reference key
//      constants via re-exported aliases below).
//
// Adding a new template key: add to templates.AllKeys + bundles, then call
// renderTpl(deps, lang, templates.KeyXxx, data) from the handler.
package bot

import (
	"context"

	"github.com/kekuta/snake-backlink-forge/services/api/internal/bot/templates"
)

// renderTpl is the canonical render entry point used by command handlers.
// When deps.Templates is nil (only happens before main.go finishes wiring,
// or in unit tests that skip templates init), returns "[" + key + "]" so a
// non-empty placeholder is always sent to the user instead of an empty reply.
func renderTpl(deps *Deps, lang, key string, data any) string {
	if deps == nil || deps.Templates == nil {
		return "[" + key + "]"
	}
	return deps.Templates.Render(lang, key, data)
}

// renderTplCtx is a convenience that pulls the language from context.
// Equivalent to renderTpl(deps, LangFromCtx(ctx), key, data).
func renderTplCtx(ctx context.Context, deps *Deps, key string, data any) string {
	return renderTpl(deps, LangFromCtx(ctx), key, data)
}

// Re-exported template key aliases. Avoids forcing every cmd_*.go file to
// import the templates package directly (keeps imports clean + minimizes
// merge churn when keys are renamed).
const (
	tplStartVerifiedRepeat   = templates.KeyStartVerifiedRepeat
	tplStartContactRejected  = templates.KeyStartContactRejected
	tplTrialPhoneReused      = templates.KeyTrialPhoneReused
	tplStartTrialBlockedBan  = templates.KeyStartTrialBlockedBanned
	tplErrorGeneric          = templates.KeyErrorGeneric
	tplErrorAccountDisabled  = templates.KeyErrorAccountDisabled

	tplKeyShow           = templates.KeyKeyShow
	tplKeyRegenConfirm   = templates.KeyKeyRegenConfirm
	tplKeyRegenDone      = templates.KeyKeyRegenDone
	tplKeyRegenCancelled = templates.KeyKeyRegenCancelled
	tplRegenRateLimited  = templates.KeyRegenRateLimited

	tplBalance         = templates.KeyBalance
	tplBuyMenuHeader   = templates.KeyBuyMenuHeader
	tplBuyConfirm      = templates.KeyBuyConfirm
	tplBuyCancelled    = templates.KeyBuyCancelled
	tplTopupQRCaption  = templates.KeyTopupQRCaption
	tplTopupPaidOK     = templates.KeyTopupPaidOK
	tplTopupCancelled  = templates.KeyTopupCancelled

	tplHistoryEmpty = templates.KeyHistoryEmpty

	tplSupportMenu             = templates.KeySupportMenu
	tplSupportFAQPayment       = templates.KeySupportFAQPayment
	tplSupportFAQKey           = templates.KeySupportFAQKey
	tplSupportFAQCredits       = templates.KeySupportFAQCredits
	tplSupportFAQTechnical     = templates.KeySupportFAQTechnical
	tplSupportDescribePrompt   = templates.KeySupportDescribePrompt
	tplSupportTicketSubmitted  = templates.KeySupportTicketSubmitted
	tplSupportTicketCapReached = templates.KeySupportTicketCapReached

	tplDownloadText        = templates.KeyDownloadText
	tplDownloadUnavailable = templates.KeyDownloadUnavailable
	tplRefShow             = templates.KeyRefShow
	tplLanguageMenu  = templates.KeyLanguageMenu
	tplLanguageSetVI = templates.KeyLanguageSetVI
	tplLanguageSetEN = templates.KeyLanguageSetEN

	tplAdminMenu             = templates.KeyAdminMenu
	tplAdminGrantOK          = templates.KeyAdminGrantOK
	tplAdminBanOK            = templates.KeyAdminBanOK
	tplAdminUnbanOK          = templates.KeyAdminUnbanOK
	tplAdminErrorUserMissing = templates.KeyAdminErrorUserMissing
	tplAdminSelfBanBlocked   = templates.KeyAdminSelfBanBlocked
)
