// Package sepay provides helpers for interacting with the SePay payment platform.
// Phase 05: QR URL builder (pure function, no IO).
// Phase 06: webhook signature verification will live in a separate file.
package sepay

import (
	"net/url"
	"strconv"
)

// BuildQRURL constructs the SePay QR image URL for a given bank transfer.
// Parameters:
//   - bankCode:  SePay bank identifier (e.g. "MBBank")
//   - accNo:     bank account number
//   - amount:    transfer amount in VND
//   - des:       transfer description (e.g. "SBF TOPUP A1B2C3D4E5F6")
//
// Returns a URL string pointing to qr.sepay.vn. The caller (Telegram bot)
// passes this URL to tgbotapi.NewPhoto — Telegram fetches and caches the image.
// Special characters in des are percent-encoded by url.Values.Encode() (spaces → '+').
func BuildQRURL(bankCode, accNo string, amount int64, des string) string {
	u := url.Values{}
	u.Set("acc", accNo)
	u.Set("bank", bankCode)
	u.Set("amount", strconv.FormatInt(amount, 10))
	u.Set("des", des)
	u.Set("template", "compact")
	return "https://qr.sepay.vn/img?" + u.Encode()
}
