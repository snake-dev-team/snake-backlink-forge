// cmd_admin_render.go — Phase 08: text rendering helpers for /admin replies.
// Kept separate from cmd_admin.go to stay under 200 lines per file.
package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
)

// renderAdminStats formats an AdminStats into a monospace Markdown table for Telegram.
func renderAdminStats(st service.AdminStats) string {
	var b strings.Builder
	b.WriteString("```\n")
	b.WriteString("=== Admin Dashboard ===\n\n")

	// Users section
	b.WriteString("-- Users --\n")
	b.WriteString(fmt.Sprintf("Total:       %d\n", st.Users))
	b.WriteString(fmt.Sprintf("Verified:    %d\n", st.UsersVerified))
	b.WriteString(fmt.Sprintf("Banned:      %d\n", st.UsersBanned))
	b.WriteString(fmt.Sprintf("Trial used:  %d\n", st.UsersTrialUsed))
	b.WriteString(fmt.Sprintf("Active keys: %d\n\n", st.ActiveKeys))

	// Transactions section
	b.WriteString("-- Transactions (24h) --\n")
	b.WriteString(fmt.Sprintf("Paid:         %d\n", st.PaidTx24h))
	b.WriteString(fmt.Sprintf("Revenue:      %s VND\n", formatAdminVND(st.Revenue24h)))
	b.WriteString(fmt.Sprintf("Pending:      %d\n", st.PendingTx))
	b.WriteString(fmt.Sprintf("Manual rev.:  %d\n\n", st.ManualReviewTx))

	// Credits section
	b.WriteString("-- Credits Outstanding --\n")
	b.WriteString(fmt.Sprintf("Premium:  %d\n", st.OutstandingPremium))
	b.WriteString(fmt.Sprintf("Standard: %d\n\n", st.OutstandingStandard))

	// Support section
	b.WriteString("-- Support --\n")
	b.WriteString(fmt.Sprintf("Open tickets: %d\n\n", st.OpenTicketsCount))

	// Auth fails
	b.WriteString(fmt.Sprintf("-- SePay Auth Fails (24h): %d --\n", len(st.LastAuthFails)))
	for i, af := range st.LastAuthFails {
		if i >= 5 {
			b.WriteString(fmt.Sprintf("  ... and %d more\n", len(st.LastAuthFails)-5))
			break
		}
		b.WriteString(fmt.Sprintf("  [%s]\n", af.CreatedAt.Format("15:04:05")))
	}

	b.WriteString("```")
	return b.String()
}

// renderAdminLookup formats a LookupResult into a readable Markdown summary.
func renderAdminLookup(r service.LookupResult) string {
	u := r.User
	var b strings.Builder

	b.WriteString("*User Summary*\n")
	b.WriteString(fmt.Sprintf("ID: `%s`\n", u.ID))
	b.WriteString(fmt.Sprintf("TG ID: `%d`\n", u.TelegramID))
	if u.TelegramUsername != nil && *u.TelegramUsername != "" {
		b.WriteString(fmt.Sprintf("Username: @%s\n", *u.TelegramUsername))
	}
	if r.MaskedPhone != "" {
		b.WriteString(fmt.Sprintf("Phone: `%s`\n", r.MaskedPhone))
	}
	b.WriteString(fmt.Sprintf("Verified: %v | Banned: %v | Trial: %v\n",
		u.IsVerified, u.IsBanned, u.TrialUsed))
	b.WriteString(fmt.Sprintf("Lang: %s | Joined: %s\n",
		u.Language, u.CreatedAt.Format("2006-01-02")))

	b.WriteString("\n*Wallet*\n")
	b.WriteString(fmt.Sprintf("Premium: %d | Standard: %d | VND spent: %s\n",
		r.Wallet.PremiumCredits, r.Wallet.StandardCredits,
		formatAdminVND(r.Wallet.TotalVndSpent)))

	if r.ActiveKey != nil {
		b.WriteString("\n*Active Key*\n")
		b.WriteString(fmt.Sprintf("Prefix: `%s` | Last used: %s\n",
			r.ActiveKey.KeyPrefix, formatOptTime(r.ActiveKey.LastUsedAt.Time)))
	}

	if len(r.LastTx) > 0 {
		b.WriteString("\n*Last Transactions*\n")
		for _, tx := range r.LastTx {
			b.WriteString(fmt.Sprintf("  [%s] %s %s VND\n",
				tx.CreatedAt.Format("01-02 15:04"),
				string(tx.Status),
				formatAdminVND(tx.AmountVnd),
			))
		}
	}

	if len(r.LastLedger) > 0 {
		b.WriteString("\n*Last Ledger Events*\n")
		for _, l := range r.LastLedger {
			sign := "+"
			if l.DeltaCredits < 0 {
				sign = ""
			}
			b.WriteString(fmt.Sprintf("  [%s] %s %s%d %s → bal %d\n",
				l.CreatedAt.Format("01-02 15:04"),
				string(l.EventType),
				sign, l.DeltaCredits,
				l.Pool,
				l.BalanceAfter,
			))
		}
	}

	return b.String()
}

// formatAdminVND formats int64 as comma-separated VND (e.g. 1329000 → "1,329,000").
// Named formatAdminVND to avoid collision with cmd_balance.go's formatVND.
func formatAdminVND(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	var out []byte
	for i, c := range s {
		pos := len(s) - i
		if i > 0 && pos%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// formatOptTime formats a time.Time as "2006-01-02 15:04" or "never" if zero.
func formatOptTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Format("2006-01-02 15:04")
}
