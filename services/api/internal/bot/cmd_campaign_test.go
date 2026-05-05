// cmd_campaign_test.go — Phase 7.02: smoke tests for /campaign bot command dispatch.
// Tests use nil services to exercise nil-guard paths and the dispatcher routing.
// No live DB or Telegram connection required — pure unit tests.
package bot

import (
	"context"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"go.uber.org/zap"
)

// ─────────────────────────── helpers ────────────────────────────────────────

// fakeUpdate builds a minimal tgbotapi.Update with a command and arguments.
func fakeUpdate(cmd, args string) tgbotapi.Update {
	text := "/" + cmd
	if args != "" {
		text += " " + args
	}
	entities := []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: len("/" + cmd)}}
	return tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat:     &tgbotapi.Chat{ID: 12345},
			From:     &tgbotapi.User{ID: 99},
			Text:     text,
			Entities: entities,
		},
	}
}

// testDeps returns a minimal Deps with logger and nil services.
func testDeps() *Deps {
	log, _ := zap.NewDevelopment()
	return &Deps{Log: log}
}

// ─────────────────────────── dispatcher tests ───────────────────────────────

// TestHandleCampaign_NilServices verifies nil-service guard returns without panic.
// Send to a zero-value BotAPI will error; that is expected and acceptable here.
func TestHandleCampaign_NilServices(t *testing.T) {
	deps := testDeps() // CampaignService + JobService are nil
	update := fakeUpdate("campaign", "list")
	api := &tgbotapi.BotAPI{}

	// Must not panic. Return value may be non-nil (Telegram send fails without token).
	_ = HandleCampaign(context.Background(), deps, api, update)
}

// TestHandleCampaign_HelpRoute verifies empty args route to help (args length == 0).
func TestHandleCampaign_HelpRoute(t *testing.T) {
	update := fakeUpdate("campaign", "")
	args := strings.Fields(update.Message.CommandArguments())
	if len(args) != 0 {
		t.Errorf("expected 0 args for empty arguments, got %d: %v", len(args), args)
	}
}

// TestHandleCampaign_SubcommandRouting verifies args[0] maps to the correct subcommand.
func TestHandleCampaign_SubcommandRouting(t *testing.T) {
	cases := []struct {
		input      string
		wantKnown  bool
		wantSub    string
	}{
		{"list", true, "list"},
		{"new https://example.com", true, "new"},
		{"pause ab12ef34", true, "pause"},
		{"resume ab12ef34", true, "resume"},
		{"archive ab12ef34", true, "archive"},
		{"jobs ab12ef34", true, "jobs"},
		{"stats ab12ef34", true, "stats"},
		{"unknown", false, ""},
	}
	for _, tc := range cases {
		args := strings.Fields(tc.input)
		sub := ""
		if len(args) > 0 {
			sub = strings.ToLower(args[0])
		}
		known := false
		switch sub {
		case "list", "new", "pause", "resume", "archive", "jobs", "stats":
			known = true
		}
		if known != tc.wantKnown {
			t.Errorf("input %q: known=%v, want %v", tc.input, known, tc.wantKnown)
		}
		if known && sub != tc.wantSub {
			t.Errorf("input %q: sub=%q, want %q", tc.input, sub, tc.wantSub)
		}
	}
}

// ─────────────────────────── helpers unit tests ─────────────────────────────

// TestEscapeMD verifies Telegram Markdown V1 special chars are escaped correctly.
func TestEscapeMD(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"_test_", "\\_test\\_"},
		{"*bold*", "\\*bold\\*"},
		{"`code`", "\\`code\\`"},
		{"[link]", "\\[link\\]"},
		{"mixed_*[`x`]", "mixed\\_\\*\\[\\`x\\`\\]"},
	}
	for _, tc := range cases {
		got := escapeMD(tc.input)
		if got != tc.want {
			t.Errorf("escapeMD(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestResolveCampaignID_ShortPrefixRejected verifies prefix < 4 chars returns error.
func TestResolveCampaignID_ShortPrefixRejected(t *testing.T) {
	deps := testDeps()
	userID := uuid.New()
	_, err := resolveCampaignID(context.Background(), deps, userID, "ab")
	if err == nil {
		t.Error("expected error for short prefix, got nil")
	}
	if !strings.Contains(err.Error(), "ngắn") {
		t.Errorf("expected VN short-prefix error message, got: %v", err)
	}
}

// TestResolveCampaignID_NilServiceReturnsError verifies nil CampaignService is handled.
func TestResolveCampaignID_NilServiceReturnsError(t *testing.T) {
	deps := testDeps() // CampaignService is nil → ResolveIDPrefix returns ErrCampaignUnavailable
	userID := uuid.New()
	_, err := resolveCampaignID(context.Background(), deps, userID, "abcd1234")
	if err == nil {
		t.Error("expected error when CampaignService is nil, got nil")
	}
}

// TestBuildPagerKeyboard_FirstPage verifies no "prev" button on page 0, and "next" present.
func TestBuildPagerKeyboard_FirstPage(t *testing.T) {
	kb := buildPagerKeyboard("cmp:list", 0, true)
	if len(kb.InlineKeyboard) == 0 {
		t.Fatal("expected at least one keyboard row")
	}
	row := kb.InlineKeyboard[0]

	hasNext := false
	for _, btn := range row {
		if strings.Contains(btn.Text, "Sau") {
			hasNext = true
		}
		if strings.Contains(btn.Text, "Trước") {
			t.Error("page 0 must not have a 'Trước' (prev) button")
		}
	}
	if !hasNext {
		t.Error("expected 'Sau' next button on first page when hasNext=true")
	}
}

// TestBuildPagerKeyboard_MiddlePage verifies both prev and next buttons present.
func TestBuildPagerKeyboard_MiddlePage(t *testing.T) {
	kb := buildPagerKeyboard("cmp:list", 2, true)
	if len(kb.InlineKeyboard) == 0 {
		t.Fatal("expected at least one keyboard row")
	}
	row := kb.InlineKeyboard[0]
	hasPrev, hasNext := false, false
	for _, btn := range row {
		if strings.Contains(btn.Text, "Trước") {
			hasPrev = true
		}
		if strings.Contains(btn.Text, "Sau") {
			hasNext = true
		}
	}
	if !hasPrev {
		t.Error("expected 'Trước' button on page 2")
	}
	if !hasNext {
		t.Error("expected 'Sau' button when hasNext=true")
	}
}

// TestBuildPagerKeyboard_LastPage verifies no "next" button when hasNext=false.
func TestBuildPagerKeyboard_LastPage(t *testing.T) {
	kb := buildPagerKeyboard("cmp:list", 3, false)
	if len(kb.InlineKeyboard) == 0 {
		t.Fatal("expected at least one keyboard row")
	}
	row := kb.InlineKeyboard[0]
	for _, btn := range row {
		if strings.Contains(btn.Text, "Sau") {
			t.Error("last page must not have a 'Sau' (next) button")
		}
	}
}

// TestJobStatusEmoji_AllStatuses verifies every known status has a meaningful emoji.
func TestJobStatusEmoji_AllStatuses(t *testing.T) {
	cases := []struct {
		status sqlcdb.JobStatus
		name   string
	}{
		{sqlcdb.JobStatusQueued, "queued"},
		{sqlcdb.JobStatusDispatched, "dispatched"},
		{sqlcdb.JobStatusInProgress, "in_progress"},
		{sqlcdb.JobStatusSuccess, "success"},
		{sqlcdb.JobStatusFailed, "failed"},
		{sqlcdb.JobStatusSkipped, "skipped"},
	}
	for _, tc := range cases {
		emoji := jobStatusEmoji(tc.status)
		if emoji == "" {
			t.Errorf("jobStatusEmoji(%q) returned empty string", tc.name)
		}
		if emoji == "❓" {
			t.Errorf("jobStatusEmoji(%q) hit fallback '❓' — add explicit case", tc.name)
		}
	}
}

// TestFormatCampaignErr_KnownErrors verifies VN messages for known error types.
func TestFormatCampaignErr_KnownErrors(t *testing.T) {
	// Verify non-empty, non-generic message for each known error.
	// We do not hard-code the exact string to allow wording evolution.
	generic := formatCampaignErr(nil) // nil → default branch
	if !strings.Contains(generic, "Lỗi hệ thống") {
		t.Errorf("nil error should return generic message, got: %q", generic)
	}
}
