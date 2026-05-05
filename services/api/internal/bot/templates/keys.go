// Package templates provides the central message-template registry for the bot.
// keys.go is the single source of truth for template key names — every Key*
// constant MUST have a corresponding entry in BOTH bundles["vi"] and bundles["en"]
// (enforced by TestRender_AllKeysHaveBothBundles).
//
// Naming convention: snake_case string value, PascalCase Go identifier prefixed
// with "Key" so handlers reference templates as templates.KeyXxx (grep-friendly).
package templates

const (
	// --- Start / contact-share flow ---
	KeyStartWelcome              = "start_welcome"
	KeyStartContactPrompt        = "start_contact_prompt"
	KeyStartVerifiedFirst        = "start_verified_first"
	KeyStartVerifiedRepeat       = "start_verified_repeat"
	KeyStartTrialBlockedReuse    = "start_trial_blocked_phone_reuse"
	KeyStartTrialBlockedBanned   = "start_trial_blocked_banned"
	KeyStartContactRejected      = "start_contact_rejected"
	KeyTrialPhoneReused          = "trial_phone_reused" // [F4]

	// --- /key + /regenkey ---
	KeyKeyShow           = "key_show"
	KeyKeyRegenConfirm   = "key_regen_confirm"
	KeyKeyRegenDone      = "key_regen_done"
	KeyKeyRegenCancelled = "key_regen_cancelled"
	KeyRegenRateLimited  = "regen_rate_limited" // [H5]

	// --- /balance + /buy + /topup ---
	KeyBalance         = "balance"
	KeyBuyMenuHeader   = "buy_menu_header"
	KeyBuyPackageRow   = "buy_package_row"
	KeyBuyConfirm      = "buy_confirm"
	KeyBuyCancelled    = "buy_cancelled"
	KeyTopupQRCaption  = "topup_qr_caption"
	KeyTopupWaiting    = "topup_waiting_notice"
	KeyTopupPaidOK     = "topup_paid_success"
	KeyTopupUnderpaid  = "topup_underpaid"
	KeyTopupCancelled  = "topup_cancelled"
	KeyTopupOverpaidOK = "topup_overpaid_success"        // [Q1]
	KeyTopupRecovered  = "topup_recovered_late_payment"  // [Q2]

	// --- /history ---
	KeyHistoryEmpty      = "history_empty"
	KeyHistoryTxRow      = "history_tx_row"
	KeyHistoryLedgerRow  = "history_ledger_row"
	KeyHistoryPageFooter = "history_page_footer"

	// --- /support ---
	KeySupportMenu             = "support_menu"
	KeySupportFAQPayment       = "support_faq_payment"
	KeySupportFAQKey           = "support_faq_key"
	KeySupportFAQCredits       = "support_faq_credits"
	KeySupportFAQTechnical     = "support_faq_technical"
	KeySupportDescribePrompt   = "support_describe_prompt"
	KeySupportTicketSubmitted  = "support_ticket_submitted"
	KeySupportTicketCapReached = "support_ticket_cap_reached"

	// --- /download + /ref + /language ---
	KeyDownloadText        = "download_text"
	KeyDownloadUnavailable = "download_unavailable"
	KeyRefShow             = "ref_show"
	KeyLanguageMenu        = "language_menu"
	KeyLanguageSetVI       = "language_set_vi"
	KeyLanguageSetEN       = "language_set_en"

	// --- Generic errors ---
	KeyErrorGeneric             = "error_generic"
	KeyErrorAccountDisabled     = "error_account_disabled"
	KeyErrorUnknownCommand      = "error_unknown_command"
	KeyErrorInsufficientCredits = "error_insufficient_credits"

	// --- Admin (Vietnamese-only is acceptable per spec, but EN bundle still required) ---
	KeyAdminMenu             = "admin_menu"
	KeyAdminStats            = "admin_stats"
	KeyAdminGrantOK          = "admin_grant_ok"
	KeyAdminBanOK            = "admin_ban_ok"
	KeyAdminUnbanOK          = "admin_unban_ok"
	KeyAdminLookupResult     = "admin_lookup_result"
	KeyAdminErrorUserMissing = "admin_error_user_not_found"
	KeyAdminSelfBanBlocked   = "admin_self_ban_blocked" // [M3]
	KeyAdminUnknownSubcmd    = "admin_unknown_subcmd"
)

// AllKeys lists every exported template key constant. Tests use this to
// enforce that both vi and en bundles contain an entry for each key.
// Add new keys to BOTH the const block above AND this slice.
var AllKeys = []string{
	KeyStartWelcome,
	KeyStartContactPrompt,
	KeyStartVerifiedFirst,
	KeyStartVerifiedRepeat,
	KeyStartTrialBlockedReuse,
	KeyStartTrialBlockedBanned,
	KeyStartContactRejected,
	KeyTrialPhoneReused,

	KeyKeyShow,
	KeyKeyRegenConfirm,
	KeyKeyRegenDone,
	KeyKeyRegenCancelled,
	KeyRegenRateLimited,

	KeyBalance,
	KeyBuyMenuHeader,
	KeyBuyPackageRow,
	KeyBuyConfirm,
	KeyBuyCancelled,
	KeyTopupQRCaption,
	KeyTopupWaiting,
	KeyTopupPaidOK,
	KeyTopupUnderpaid,
	KeyTopupCancelled,
	KeyTopupOverpaidOK,
	KeyTopupRecovered,

	KeyHistoryEmpty,
	KeyHistoryTxRow,
	KeyHistoryLedgerRow,
	KeyHistoryPageFooter,

	KeySupportMenu,
	KeySupportFAQPayment,
	KeySupportFAQKey,
	KeySupportFAQCredits,
	KeySupportFAQTechnical,
	KeySupportDescribePrompt,
	KeySupportTicketSubmitted,
	KeySupportTicketCapReached,

	KeyDownloadText,
	KeyDownloadUnavailable,
	KeyRefShow,
	KeyLanguageMenu,
	KeyLanguageSetVI,
	KeyLanguageSetEN,

	KeyErrorGeneric,
	KeyErrorAccountDisabled,
	KeyErrorUnknownCommand,
	KeyErrorInsufficientCredits,

	KeyAdminMenu,
	KeyAdminStats,
	KeyAdminGrantOK,
	KeyAdminBanOK,
	KeyAdminUnbanOK,
	KeyAdminLookupResult,
	KeyAdminErrorUserMissing,
	KeyAdminSelfBanBlocked,
	KeyAdminUnknownSubcmd,
}
