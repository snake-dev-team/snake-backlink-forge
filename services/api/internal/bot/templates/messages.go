// Package templates — message bundles for VN (default) and EN (fallback).
//
// File-size note: this file intentionally exceeds the 200-line modularization
// guideline because content density (40+ keys × 2 languages × multi-line VN/EN
// strings) cannot be split without losing the "single registry" property that
// makes Renderer's lookup trivial. Splitting per-feature would scatter keys
// and require multiple init() merges. Treated as data-only — adding a new key
// is a 2-line addition (one per bundle).
//
// Placeholder copy: tones are functional but NOT polished. Final VN "bắt tai"
// copywriter pass happens in /ck:cook copywriter stage. EN strings are kept
// professional and concise to mirror the structure.
package templates

// bundles maps lang → key → text/template source.
// Renderer.Render falls back to "vi" when the requested lang lacks a key.
var bundles = map[string]map[string]string{
	"vi": viBundle,
	"en": enBundle,
}

// viBundle — Vietnamese (default).
var viBundle = map[string]string{
	// --- Start / contact-share flow ---
	KeyStartWelcome: `🐍 *Chào mừng đến Snake Backlink Forge!*

Tool automation xây backlink chuyên nghiệp, dùng qua Chrome/Edge extension.

Bấm nút bên dưới để chia sẻ số điện thoại và bắt đầu nhé.`,

	KeyStartContactPrompt: `📱 Vui lòng nhấn nút *Chia sẻ số điện thoại* để xác thực tài khoản.`,

	KeyStartVerifiedFirst: `✅ *Xác thực thành công!*

🎁 Bạn đã nhận *{{.StandardCredits}} Standard credits* miễn phí.

🔑 *API Key của bạn:*
` + "`{{.Key}}`" + `

⚠️ *Key chỉ hiển thị MỘT LẦN — hãy lưu lại ngay.*

Dùng /key để xem prefix, /balance để kiểm tra số dư.`,

	KeyStartVerifiedRepeat: `✅ Tài khoản của bạn đã được xác thực.

Dùng /key để xem API key, /balance để kiểm tra số dư.`,

	KeyStartTrialBlockedReuse: `⚠️ Số điện thoại này đã được dùng cho tài khoản khác. Mỗi số chỉ được kích hoạt một lần.`,

	KeyStartTrialBlockedBanned: `🚫 Tài khoản đã bị tạm khoá. Liên hệ @support để được hỗ trợ.`,

	KeyStartContactRejected: `⚠️ Vui lòng chỉ chia sẻ số điện thoại của *chính bạn*.`,

	KeyTrialPhoneReused: `⚠️ Số điện thoại này đã dùng cho tài khoản khác. Mỗi số chỉ được kích hoạt một lần.`,

	// --- /key + /regenkey ---
	KeyKeyShow: `🔑 *API Key của bạn:*
` + "`{{.KeyPrefixMasked}}`" + `

Plaintext không thể lấy lại. Dùng *🔄 Regenerate* để tạo key mới (key cũ sẽ bị thu hồi).`,

	KeyKeyRegenConfirm: `⚠️ Bạn có chắc muốn tạo lại API key?

Key hiện tại sẽ bị *thu hồi ngay* và không thể khôi phục.
Key mới chỉ hiện *một lần duy nhất*.`,

	KeyKeyRegenDone: `✅ *API key mới của bạn:*

` + "`{{.Key}}`" + `

⚠️ *Lưu ngay — bot sẽ không hiện lại.*

Dùng /key để xem prefix.`,

	KeyKeyRegenCancelled: `Đã huỷ.`,

	KeyRegenRateLimited: `⏱ Đã đạt giới hạn 3 lần tạo lại/ngày. Thử lại sau.`,

	// --- /balance + /buy + /topup ---
	KeyBalance: `💰 *Số dư của bạn:*

🔥 Premium: *{{.Premium}}* credit
⚡ Standard: *{{.Standard}}* credit

💸 Đã chi: {{.TotalVNDFormatted}}đ`,

	KeyBuyMenuHeader: `🛒 *Chọn gói credit bạn muốn mua:*

⭐ = gói phổ biến nhất`,

	KeyBuyPackageRow: `{{.DisplayName}} · {{.Credits}} credit · {{.PriceVND}}đ`,

	KeyBuyConfirm: `📦 *{{.Display}}*

💳 Số credits: {{.CreditSummary}}
💰 Giá: *{{.AmountVNDFormatted}}đ*

Xác nhận thanh toán?`,

	KeyBuyCancelled: `Đã huỷ.`,

	KeyTopupQRCaption: `💳 *Thanh toán gói {{.Display}}*

🏦 Ngân hàng: {{.BankCode}}
💰 Số tiền: *{{.AmountVNDFormatted}}đ*
📝 Nội dung CK: ` + "<code>SBF TOPUP {{.OrderCode}}</code>" + `

⏰ QR có hiệu lực 24 giờ.`,

	KeyTopupWaiting: `⏳ Đang chờ SePay xác nhận thanh toán...

Sau khi chuyển khoản, credits sẽ được cộng tự động trong vòng 1 phút.`,

	KeyTopupPaidOK: `✅ *Nạp tiền thành công!*

📦 Gói: {{.Display}}
💳 Credits: {{.CreditSummary}}

Dùng /balance để xem số dư.`,

	KeyTopupUnderpaid: `⚠️ Số tiền chuyển khoản không đủ. Đơn hàng đã chuyển sang *manual review* — admin sẽ kiểm tra trong 24h.`,

	KeyTopupCancelled: `Đã huỷ đơn.`,

	KeyTopupOverpaidOK: `✅ *Nạp tiền thành công + bonus!*

📦 Gói: {{.Display}}
💳 Credits gốc: {{.CreditSummary}}
🎁 Bonus từ phần dư ({{.ExcessVNDFormatted}}đ): *+{{.BonusCredits}}* {{.BonusPool}}

Dùng /balance để xem số dư.`,

	KeyTopupRecovered: `✅ *Đơn cũ đã được thanh toán muộn — credits đã cộng!*

📦 Gói: {{.Display}}
💳 Credits: {{.CreditSummary}}

Dùng /balance để xem số dư.`,

	// --- /history ---
	KeyHistoryEmpty: `Chưa có lịch sử.`,

	KeyHistoryTxRow: `{{.StatusIcon}} {{.Display}} · {{.AmountVNDFormatted}}đ · {{.CreatedAt}}`,

	KeyHistoryLedgerRow: `▪ {{.EventLabel}} · {{.Delta}} {{.Pool}} · {{.CreatedAt}}`,

	KeyHistoryPageFooter: `Tx page {{.PageTx}}/{{.MaxTx}} · Credits page {{.PageLedger}}/{{.MaxLedger}}`,

	// --- /support ---
	KeySupportMenu: `🆘 *Trung tâm hỗ trợ*

Chọn chủ đề bạn cần hỗ trợ:`,

	KeySupportFAQPayment: `*❓ Thanh toán*

Chúng tôi hỗ trợ chuyển khoản qua SePay. Sau khi chuyển khoản, credits được cộng tự động trong vòng 1 phút. Nếu sau 5 phút chưa nhận được, hãy liên hệ support.`,

	KeySupportFAQKey: `*❓ API Key*

Key được cấp một lần duy nhất khi xác thực số điện thoại. Dùng /key để xem prefix. Nếu mất key, dùng /regenkey để tạo key mới (key cũ bị thu hồi).`,

	KeySupportFAQCredits: `*❓ Credits*

Credits dùng để chạy backlink, captcha và finder. Premium credits ưu tiên hơn Standard. Xem số dư bằng /balance.`,

	KeySupportFAQTechnical: `*❓ Kỹ thuật*

Nếu bot không phản hồi, thử gửi /start lại. Nếu lỗi liên tục, hãy mô tả chi tiết và gửi ticket để đội kỹ thuật hỗ trợ.`,

	KeySupportDescribePrompt: `📝 Mô tả vấn đề của bạn (tối đa 4096 ký tự).

Gửi /cancel để huỷ.`,

	KeySupportTicketSubmitted: `✅ Đã gửi ticket #{{.TicketID}}. Admin sẽ reply trong vòng 24h.`,

	KeySupportTicketCapReached: `⚠️ Đã đạt giới hạn 3 ticket mở. Đợi admin reply trước khi gửi ticket mới.`,

	// --- /download + /ref + /language ---
	KeyDownloadText: `📥 Tải installer Windows:
{{.InstallerURL}}`,

	KeyDownloadUnavailable: `⏳ Installer chưa publish. Theo dõi announcement nhé.`,

	KeyRefShow: `🎁 *Mã giới thiệu của bạn:* ` + "`{{.Code}}`" + `

🔗 Link: {{.DeepLink}}

👥 Đã giới thiệu: *{{.TotalReferred}}* người`,

	KeyLanguageMenu: `🌐 Chọn ngôn ngữ / Choose language:`,

	KeyLanguageSetVI: `✅ Đã đổi sang Tiếng Việt.`,

	KeyLanguageSetEN: `✅ Switched to English.`,

	// --- Generic errors ---
	KeyErrorGeneric:             `⚠️ Đã xảy ra lỗi. Vui lòng thử lại sau.`,
	KeyErrorAccountDisabled:     `🚫 Tài khoản đã bị tạm khoá. Liên hệ @support để được hỗ trợ.`,
	KeyErrorUnknownCommand:      `Lệnh không hợp lệ. Dùng /start để bắt đầu.`,
	KeyErrorInsufficientCredits: `⚠️ Không đủ {{.Pool}} credit. Cần *{{.Needed}}*, có *{{.Have}}*. Dùng /buy để nạp thêm.`,

	// --- Admin ---
	KeyAdminMenu: `*/admin* — lệnh quản trị:
` + "`/admin stats`" + ` — thống kê tổng quan
` + "`/admin grant <tg_id> <pool> <amount> [reason]`" + ` — cộng credit
` + "`/admin ban <tg_id> [reason]`" + ` — khóa tài khoản
` + "`/admin unban <tg_id>`" + ` — mở khóa
` + "`/admin lookup <tg_id|+phone|key_prefix>`" + ` — tra cứu user`,

	KeyAdminStats:            `{{.Body}}`,
	KeyAdminGrantOK:          `Đã cộng *{{.Amount}}* {{.Pool}} credit cho tg_id={{.TargetTGID}}.` + "\n" + `Số dư mới: *{{.NewBalance}}*` + "\n" + `Lý do: {{.Reason}}`,
	KeyAdminBanOK:            `Đã ban tg_id={{.TargetTGID}}.`,
	KeyAdminUnbanOK:          `Đã unban tg_id={{.TargetTGID}}.`,
	KeyAdminLookupResult:     `{{.Body}}`,
	KeyAdminErrorUserMissing: `Không tìm thấy user.`,
	KeyAdminSelfBanBlocked:   `Không thể ban tài khoản admin.`,
	KeyAdminUnknownSubcmd:    `Subcommand không hợp lệ. Dùng /admin để xem help.`,
}

// enBundle — English fallback. Functional placeholders, professional tone.
var enBundle = map[string]string{
	// --- Start / contact-share flow ---
	KeyStartWelcome: `🐍 *Welcome to Snake Backlink Forge!*

Professional backlink automation via our Chrome/Edge extension.

Tap the button below to share your phone number and get started.`,

	KeyStartContactPrompt: `📱 Please tap *Share phone number* to verify your account.`,

	KeyStartVerifiedFirst: `✅ *Verification successful!*

🎁 You received *{{.StandardCredits}} Standard credits* free.

🔑 *Your API Key:*
` + "`{{.Key}}`" + `

⚠️ *The key is shown ONCE — save it now.*

Use /key to view the prefix, /balance to check your balance.`,

	KeyStartVerifiedRepeat: `✅ Your account is already verified.

Use /key to view your API key, /balance to check your balance.`,

	KeyStartTrialBlockedReuse: `⚠️ This phone number is already linked to another account. Each phone can be activated only once.`,

	KeyStartTrialBlockedBanned: `🚫 This account is suspended. Contact @support for assistance.`,

	KeyStartContactRejected: `⚠️ Please share *your own* phone number only.`,

	KeyTrialPhoneReused: `⚠️ This phone number is already used by another account. Each phone can be activated only once.`,

	// --- /key + /regenkey ---
	KeyKeyShow: `🔑 *Your API Key:*
` + "`{{.KeyPrefixMasked}}`" + `

The plaintext key cannot be retrieved. Use *🔄 Regenerate* to issue a new key (the old one is revoked).`,

	KeyKeyRegenConfirm: `⚠️ Are you sure you want to regenerate your API key?

The current key will be *revoked immediately* and cannot be restored.
The new key will be shown *only once*.`,

	KeyKeyRegenDone: `✅ *Your new API key:*

` + "`{{.Key}}`" + `

⚠️ *Save it now — the bot will not show it again.*

Use /key to view the prefix.`,

	KeyKeyRegenCancelled: `Cancelled.`,

	KeyRegenRateLimited: `⏱ Daily limit reached (3 regenerations/day). Try again later.`,

	// --- /balance + /buy + /topup ---
	KeyBalance: `💰 *Your balance:*

🔥 Premium: *{{.Premium}}* credits
⚡ Standard: *{{.Standard}}* credits

💸 Spent: {{.TotalVNDFormatted}} VND`,

	KeyBuyMenuHeader: `🛒 *Choose a credit package:*

⭐ = most popular`,

	KeyBuyPackageRow: `{{.DisplayName}} · {{.Credits}} credits · {{.PriceVND}} VND`,

	KeyBuyConfirm: `📦 *{{.Display}}*

💳 Credits: {{.CreditSummary}}
💰 Price: *{{.AmountVNDFormatted}} VND*

Confirm payment?`,

	KeyBuyCancelled: `Cancelled.`,

	KeyTopupQRCaption: `💳 *Payment for {{.Display}}*

🏦 Bank: {{.BankCode}}
💰 Amount: *{{.AmountVNDFormatted}} VND*
📝 Transfer note: ` + "<code>SBF TOPUP {{.OrderCode}}</code>" + `

⏰ QR valid for 24 hours.`,

	KeyTopupWaiting: `⏳ Waiting for SePay to confirm payment...

Credits will be added automatically within 1 minute after the transfer.`,

	KeyTopupPaidOK: `✅ *Top-up successful!*

📦 Package: {{.Display}}
💳 Credits: {{.CreditSummary}}

Use /balance to check your balance.`,

	KeyTopupUnderpaid: `⚠️ The transferred amount is insufficient. The order is now in *manual review* — an admin will review it within 24h.`,

	KeyTopupCancelled: `Order cancelled.`,

	KeyTopupOverpaidOK: `✅ *Top-up successful + bonus!*

📦 Package: {{.Display}}
💳 Base credits: {{.CreditSummary}}
🎁 Bonus from excess ({{.ExcessVNDFormatted}} VND): *+{{.BonusCredits}}* {{.BonusPool}}

Use /balance to check your balance.`,

	KeyTopupRecovered: `✅ *Late payment recovered — credits granted!*

📦 Package: {{.Display}}
💳 Credits: {{.CreditSummary}}

Use /balance to check your balance.`,

	// --- /history ---
	KeyHistoryEmpty: `No history yet.`,

	KeyHistoryTxRow: `{{.StatusIcon}} {{.Display}} · {{.AmountVNDFormatted}} VND · {{.CreatedAt}}`,

	KeyHistoryLedgerRow: `▪ {{.EventLabel}} · {{.Delta}} {{.Pool}} · {{.CreatedAt}}`,

	KeyHistoryPageFooter: `Tx page {{.PageTx}}/{{.MaxTx}} · Credits page {{.PageLedger}}/{{.MaxLedger}}`,

	// --- /support ---
	KeySupportMenu: `🆘 *Support center*

Choose a topic:`,

	KeySupportFAQPayment: `*❓ Payment*

We support bank transfers via SePay. After transferring, credits are added automatically within 1 minute. If nothing arrives after 5 minutes, contact support.`,

	KeySupportFAQKey: `*❓ API Key*

Your key is issued once on phone verification. Use /key to view the prefix. If you lose your key, use /regenkey to generate a new one (the old key is revoked).`,

	KeySupportFAQCredits: `*❓ Credits*

Credits power backlink, captcha, and finder runs. Premium credits are spent before Standard. Use /balance to check your balance.`,

	KeySupportFAQTechnical: `*❓ Technical*

If the bot is unresponsive, try /start again. For persistent issues, describe the problem in detail and submit a ticket — our engineers will help.`,

	KeySupportDescribePrompt: `📝 Describe your issue (max 4096 characters).

Send /cancel to abort.`,

	KeySupportTicketSubmitted: `✅ Ticket #{{.TicketID}} submitted. An admin will reply within 24h.`,

	KeySupportTicketCapReached: `⚠️ You have 3 open tickets — wait for an admin reply before submitting another.`,

	// --- /download + /ref + /language ---
	KeyDownloadText: `📥 Download Windows installer:
{{.InstallerURL}}`,

	KeyDownloadUnavailable: `⏳ Installer not yet published. Watch for the announcement.`,

	KeyRefShow: `🎁 *Your referral code:* ` + "`{{.Code}}`" + `

🔗 Link: {{.DeepLink}}

👥 Referred: *{{.TotalReferred}}* users`,

	KeyLanguageMenu: `🌐 Chọn ngôn ngữ / Choose language:`,

	KeyLanguageSetVI: `✅ Đã đổi sang Tiếng Việt.`,

	KeyLanguageSetEN: `✅ Switched to English.`,

	// --- Generic errors ---
	KeyErrorGeneric:             `⚠️ Something went wrong. Please try again later.`,
	KeyErrorAccountDisabled:     `🚫 This account is suspended. Contact @support for assistance.`,
	KeyErrorUnknownCommand:      `Unknown command. Use /start to begin.`,
	KeyErrorInsufficientCredits: `⚠️ Not enough {{.Pool}} credits. Need *{{.Needed}}*, have *{{.Have}}*. Use /buy to top up.`,

	// --- Admin (mirrored from VN; admins are dev team but EN bundle still required) ---
	KeyAdminMenu: `*/admin* — administrative commands:
` + "`/admin stats`" + ` — overall stats
` + "`/admin grant <tg_id> <pool> <amount> [reason]`" + ` — grant credits
` + "`/admin ban <tg_id> [reason]`" + ` — ban account
` + "`/admin unban <tg_id>`" + ` — unban account
` + "`/admin lookup <tg_id|+phone|key_prefix>`" + ` — lookup user`,

	KeyAdminStats:            `{{.Body}}`,
	KeyAdminGrantOK:          `Granted *{{.Amount}}* {{.Pool}} credits to tg_id={{.TargetTGID}}.` + "\n" + `New balance: *{{.NewBalance}}*` + "\n" + `Reason: {{.Reason}}`,
	KeyAdminBanOK:            `Banned tg_id={{.TargetTGID}}.`,
	KeyAdminUnbanOK:          `Unbanned tg_id={{.TargetTGID}}.`,
	KeyAdminLookupResult:     `{{.Body}}`,
	KeyAdminErrorUserMissing: `User not found.`,
	KeyAdminSelfBanBlocked:   `Cannot ban an admin account.`,
	KeyAdminUnknownSubcmd:    `Unknown subcommand. Use /admin for help.`,
}
