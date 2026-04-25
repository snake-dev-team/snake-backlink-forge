// Package notify holds shared alert types used by both the bot and service layers.
// It has NO imports from bot or service — exists solely to break the import cycle:
//   service/webhook_service.go → notify.AdminAlert
//   bot/admin_alerts.go        → notify.AdminAlert
// Neither service nor bot imports the other.
package notify

import "time"

// AdminAlert is a structured alert DM'd to configured admin Telegram IDs.
// Produced by: webhook_service (overpaid, internal_error) and retry_consumer (dead_letter, backlog).
// Consumed by: bot.ConsumeAdminAlerts goroutine spawned in main.go.
type AdminAlert struct {
	Kind         string    // "overpaid" | "auth_fail_burst" | "webhook_internal_error" | "retry_dead_letter" | "retry_queue_backlog"
	OrderCode    string
	UserID       string
	ExcessVND    int64
	BonusCredits int
	Err          string
	At           time.Time
}
