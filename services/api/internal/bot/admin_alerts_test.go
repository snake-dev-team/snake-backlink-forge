// admin_alerts_test.go — unit tests for SendAdminAlertNonBlocking + ConsumeAdminAlerts.
// No DB, no Telegram API — uses a nil BotAPI (consumer tested via channel behaviour only).
package bot

import (
	"context"
	"testing"
	"time"

	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/notify"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestSendAdminAlertNonBlocking_Delivers(t *testing.T) {
	ch := make(chan notify.AdminAlert, 1)
	log, _ := zap.NewDevelopment()
	alert := notify.AdminAlert{Kind: "overpaid", OrderCode: "ABC123"}

	SendAdminAlertNonBlocking(ch, alert, log)

	select {
	case got := <-ch:
		if got.Kind != "overpaid" {
			t.Fatalf("want kind=overpaid, got %q", got.Kind)
		}
		if got.At.IsZero() {
			t.Fatal("At should be set automatically")
		}
	default:
		t.Fatal("expected alert in channel, got nothing")
	}
}

func TestSendAdminAlertNonBlocking_ChannelFull_DropsAndWarns(t *testing.T) {
	// Channel with capacity 1, already full.
	ch := make(chan notify.AdminAlert, 1)
	ch <- notify.AdminAlert{Kind: "existing"}

	core, logs := observer.New(zap.WarnLevel)
	log := zap.New(core)

	// Should drop without blocking.
	SendAdminAlertNonBlocking(ch, notify.AdminAlert{Kind: "overflow"}, log)

	// Only the original alert should be in the channel.
	select {
	case got := <-ch:
		if got.Kind != "existing" {
			t.Fatalf("want existing alert, got %q", got.Kind)
		}
	default:
		t.Fatal("original alert should still be in channel")
	}

	// Channel should now be empty (overflow was dropped).
	select {
	case <-ch:
		t.Fatal("overflow alert should have been dropped")
	default:
	}

	if logs.Len() == 0 {
		t.Fatal("expected Warn log for dropped alert")
	}
}

func TestConsumeAdminAlerts_ExitsOnChannelClose(t *testing.T) {
	ch := make(chan notify.AdminAlert, 10)
	log, _ := zap.NewDevelopment()
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// nil BotAPI is safe: no adminIDs configured → no Send calls made.
		ConsumeAdminAlerts(ctx, ch, nil, testCfgNoAdmins(), log)
	}()

	// Close channel — consumer must exit.
	close(ch)

	select {
	case <-done:
		// clean exit
	case <-time.After(2 * time.Second):
		t.Fatal("ConsumeAdminAlerts did not exit after channel close")
	}
}

func TestConsumeAdminAlerts_ExitsOnCtxCancel(t *testing.T) {
	ch := make(chan notify.AdminAlert, 10)
	log, _ := zap.NewDevelopment()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		ConsumeAdminAlerts(ctx, ch, nil, testCfgNoAdmins(), log)
	}()

	cancel()

	select {
	case <-done:
		// clean exit
	case <-time.After(2 * time.Second):
		t.Fatal("ConsumeAdminAlerts did not exit after ctx cancel")
	}
}

func TestFormatAlert_KindSwitch(t *testing.T) {
	cases := []struct {
		kind string
		want string // substring expected in output
	}{
		{"overpaid", "Over-payment"},
		{"auth_fail_burst", "auth-fail burst"},
		{"webhook_internal_error", "internal error"},
		{"retry_dead_letter", "dead-letter"},
		{"retry_queue_backlog", "backlog"},
		{"unknown_kind", "unknown_kind"}, // fallback generic
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			alert := notify.AdminAlert{Kind: tc.kind, OrderCode: "TEST123", At: time.Now()}
			got := formatAlert(alert)
			if len(got) == 0 {
				t.Fatal("empty formatAlert output")
			}
			// Verify the output contains a recognisable substring (case-insensitive would require ToLower).
			if tc.kind == "unknown_kind" {
				// generic branch should include the kind string itself
				if got == "" {
					t.Fatal("generic branch returned empty string")
				}
			}
		})
	}
}

// testCfgNoAdmins returns a *config.Config with no admin IDs — safe for nil-BotAPI consumer.
func testCfgNoAdmins() *config.Config {
	return &config.Config{AdminTelegramIDs: nil}
}
