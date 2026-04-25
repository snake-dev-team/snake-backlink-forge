// webhook.go — Phase 06: POST /webhooks/sepay thin controller.
// Auth [Q4 rate-limit enforced by middleware] → verify Apikey → parse → [Q3] account/gateway
// match → [F2] regex extract → service call → [F3] error classify → [H6] notify goroutine.
package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/integration/sepay"
	"go.uber.org/zap"
)

// ipHashField returns the SHA-256 hex of c.IP() for use in audit metadata.
// Raw IPs are never stored — spec §security line 590 + GDPR obligation.
func ipHashField(c *fiber.Ctx) string { return sepay.HashIP(c.IP()) }

// SePayWebhook returns the Fiber handler for POST /webhooks/sepay.
// deps must be non-nil; individual fields (WebhookSvc, Rdb) may be nil in tests.
func SePayWebhook(deps *WebhookDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Apikey authentication — constant-time compare; fail-closed. [Q4 rate-limit already ran]
		if !sepay.VerifyApikey(c.Get("Authorization"), deps.Cfg.SepayWebhookToken) {
			// [C2] Hash IP before logging/auditing — never store raw IP (GDPR / PDPD).
			deps.Log.Warn("sepay webhook auth fail", zap.String("ip_hash", ipHashField(c)))
			webhookAuditLog(deps, "sepay_auth_fail", map[string]any{"ip_hash": ipHashField(c)})
			return ack200(c, false, "invalid_signature")
		}

		// 2. Parse body — SePay sends application/json.
		var p sepay.Payload
		if err := c.BodyParser(&p); err != nil {
			deps.Log.Warn("sepay webhook: body parse error", zap.Error(err))
			return ack200(c, false, "invalid_payload")
		}

		// 3. Ignore outgoing transfers (transferType != "in"). Return 200 success=true (not false)
		//    so SePay does not retry outgoing-transfer events.
		if p.TransferType != "in" {
			return ack200(c, true, "ignored_outgoing")
		}

		// 4. [Q3] Hard-bind to single merchant bank account — reject cross-merchant webhooks.
		if p.AccountNumber != deps.Cfg.SepayBankAccount {
			// [C2] Never log or audit raw account number — spec §architecture line 91.
			deps.Log.Warn("sepay webhook: account mismatch",
				zap.String("received_acc_sha256_prefix", sepay.HashAccountPrefix(p.AccountNumber)),
			)
			webhookAuditLog(deps, "sepay_account_mismatch", map[string]any{
				"received_acc_sha256_prefix": sepay.HashAccountPrefix(p.AccountNumber),
				"gateway":                    p.Gateway,
			})
			return ack200(c, false, "account_mismatch")
		}
		if p.Gateway != deps.Cfg.SepayBankCode {
			deps.Log.Warn("sepay webhook: gateway mismatch",
				zap.String("received", p.Gateway),
			)
			webhookAuditLog(deps, "sepay_gateway_mismatch", map[string]any{
				"received_gateway": p.Gateway,
			})
			return ack200(c, false, "gateway_mismatch")
		}

		// 5. [F2] Extract 12-hex order code from content field; normalize to uppercase before DB query.
		match := sepay.OrderCodeRe.FindStringSubmatch(p.Content)
		if match == nil {
			// [C2] Truncate content to first 50 chars — full content may contain PII
			// (e.g. personal name in bank memo). Amount is safe to log verbatim.
			contentSnippet := p.Content
			if len(contentSnippet) > 50 {
				contentSnippet = contentSnippet[:50]
			}
			deps.Log.Info("sepay webhook: unmatched transfer content",
				zap.String("content_snippet", contentSnippet),
				zap.Int64("amount", p.TransferAmount),
			)
			webhookAuditLog(deps, "sepay_unmatched_transfer", map[string]any{
				"content_snippet": contentSnippet,
				"amount":          p.TransferAmount,
			})
			return ack200(c, true, "unmatched")
		}
		orderCode := strings.ToUpper(match[1]) // DB stores uppercase always

		// 6. Service call — CAS-gated atomic grant.
		if deps.WebhookSvc == nil {
			deps.Log.Error("sepay webhook: WebhookSvc not wired")
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"success": false, "reason": "service_unavailable",
			})
		}
		result, err := deps.WebhookSvc.ProcessPaidTransaction(c.Context(), orderCode, p)
		if err != nil {
			return classifyWebhookError(c, deps, err, orderCode, p)
		}

		// 7. Business-logic result branches — all return 200 (no SePay retry on business errors).
		switch {
		case result.Underpaid:
			return ack200(c, true, "underpaid_flagged")
		case result.AlreadyProcessed:
			return ack200(c, true, "idempotent_replay")
		case result.UnknownOrder:
			return ack200(c, true, "unknown_order")
		}

		// 8. [H6] Post-commit Telegram notify — non-blocking goroutine scoped to RootCtx + 5s.
		//    Webhook handler returns immediately; user gets DM asynchronously.
		//    See webhook_notify.go for the impl (loadUserNotifyInfo + pickTemplate + Send).
		if deps.RootCtx != nil {
			notifyTelegramSuccess(deps, result)
		}

		return ack200(c, true, "")
	}
}
