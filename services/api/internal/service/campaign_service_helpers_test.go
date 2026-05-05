// campaign_service_helpers_test.go — pure-function unit tests for unexported
// helpers in campaign_service.go. No DB required.
//
// Package: service (internal) so unexported functions are visible.
// Covers: isPrivateHost, validateCampaignInput, compactStrings (F13 guard).
package service

import (
	"strings"
	"testing"
)

// ─────────────────────────── TestIsPrivateHost ─────────────────────────────

func TestIsPrivateHost(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"foo.localhost", true},
		{"127.0.0.1", true},
		{"127.0.0.1:8080", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true}, // AWS/GCP metadata — F13
		{"::1", true},
		{"example.com", false},
		{"public.io", false},
		{"google.com", false},
		{"8.8.8.8", false},
		{"192.167.1.1", false}, // not in RFC1918 /16 range — public
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.host, func(t *testing.T) {
			if got := isPrivateHost(tc.host); got != tc.want {
				t.Fatalf("isPrivateHost(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}

// ─────────────────────────── TestValidateCampaignInput ────────────────────

func TestValidateCampaignInput(t *testing.T) {
	base := CampaignInput{
		Name:             "valid name",
		MoneySiteURL:     "https://example.com",
		AnchorTexts:      []AnchorTextInput{{Text: "anchor", Type: "branded", Weight: 1}},
		Pool:             "standard",
		SourceMode:       "prebuilt",
		DailyLimit:       5,
		CreditsAllocated: 10,
	}
	cases := []struct {
		name    string
		mutate  func(*CampaignInput)
		wantErr bool
	}{
		{"happy path", func(*CampaignInput) {}, false},
		{"name too short", func(c *CampaignInput) { c.Name = "ab" }, true},
		{"name too long", func(c *CampaignInput) { c.Name = strings.Repeat("x", 129) }, true},
		{"name exactly 3", func(c *CampaignInput) { c.Name = "abc" }, false},
		{"name exactly 128", func(c *CampaignInput) { c.Name = strings.Repeat("x", 128) }, false},
		{"url missing scheme", func(c *CampaignInput) { c.MoneySiteURL = "example.com" }, true},
		{"url javascript", func(c *CampaignInput) { c.MoneySiteURL = "javascript:alert(1)" }, true},
		{"url ftp", func(c *CampaignInput) { c.MoneySiteURL = "ftp://example.com" }, true},
		{"url localhost SSRF", func(c *CampaignInput) { c.MoneySiteURL = "http://localhost:6379" }, true},
		{"url metadata SSRF", func(c *CampaignInput) { c.MoneySiteURL = "http://169.254.169.254/" }, true},
		{"url 10.0.0.1 SSRF", func(c *CampaignInput) { c.MoneySiteURL = "http://10.0.0.1/" }, true},
		{"url 127.0.0.1 SSRF", func(c *CampaignInput) { c.MoneySiteURL = "http://127.0.0.1/" }, true},
		{"pool invalid", func(c *CampaignInput) { c.Pool = "gold" }, true},
		{"pool premium ok", func(c *CampaignInput) { c.Pool = "premium" }, false},
		{"source_mode invalid", func(c *CampaignInput) { c.SourceMode = "unknown" }, true},
		{"source_mode autofind ok", func(c *CampaignInput) { c.SourceMode = "autofind" }, false},
		{"source_mode custom ok", func(c *CampaignInput) { c.SourceMode = "custom" }, false},
		{"source_mode mixed ok", func(c *CampaignInput) { c.SourceMode = "mixed" }, false},
		{"daily_limit too low", func(c *CampaignInput) { c.DailyLimit = 4 }, true},
		{"daily_limit too high", func(c *CampaignInput) { c.DailyLimit = 51 }, true},
		{"daily_limit min ok", func(c *CampaignInput) { c.DailyLimit = 5 }, false},
		{"daily_limit max ok", func(c *CampaignInput) { c.DailyLimit = 50 }, false},
		{"no anchors", func(c *CampaignInput) { c.AnchorTexts = nil }, true},
		{"anchor text empty", func(c *CampaignInput) {
			c.AnchorTexts = []AnchorTextInput{{Text: "", Type: "branded", Weight: 1}}
		}, true},
		{"anchor weight zero", func(c *CampaignInput) {
			c.AnchorTexts = []AnchorTextInput{{Text: "x", Type: "branded", Weight: 0}}
		}, true},
		{"anchor weight 101", func(c *CampaignInput) {
			c.AnchorTexts = []AnchorTextInput{{Text: "x", Type: "branded", Weight: 101}}
		}, true},
		{"anchor weight 100 ok", func(c *CampaignInput) {
			c.AnchorTexts = []AnchorTextInput{{Text: "x", Type: "branded", Weight: 100}}
		}, false},
		{"anchor type invalid", func(c *CampaignInput) {
			c.AnchorTexts = []AnchorTextInput{{Text: "x", Type: "weird", Weight: 1}}
		}, true},
		{"anchor type naked ok", func(c *CampaignInput) {
			c.AnchorTexts = []AnchorTextInput{{Text: "x", Type: "naked", Weight: 1}}
		}, false},
		{"anchor type generic ok", func(c *CampaignInput) {
			c.AnchorTexts = []AnchorTextInput{{Text: "x", Type: "generic", Weight: 1}}
		}, false},
		{"anchor type exact ok", func(c *CampaignInput) {
			c.AnchorTexts = []AnchorTextInput{{Text: "x", Type: "exact", Weight: 1}}
		}, false},
		{"21 anchors", func(c *CampaignInput) {
			anchors := make([]AnchorTextInput, 21)
			for i := range anchors {
				anchors[i] = AnchorTextInput{Text: "anchor", Type: "branded", Weight: 1}
			}
			c.AnchorTexts = anchors
		}, true},
		{"20 anchors ok", func(c *CampaignInput) {
			anchors := make([]AnchorTextInput, 20)
			for i := range anchors {
				anchors[i] = AnchorTextInput{Text: "anchor", Type: "branded", Weight: 1}
			}
			c.AnchorTexts = anchors
		}, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			in := base
			tc.mutate(&in)
			err := validateCampaignInput(in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateCampaignInput() err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

// ─────────────────────────── TestCompactStrings ───────────────────────────

func TestCompactStrings(t *testing.T) {
	in := []string{"  foo ", "FOO", "bar", "", "bar", "baz"}
	got := compactStrings(in, 10)
	want := []string{"foo", "bar", "baz"}
	if len(got) != len(want) {
		t.Fatalf("compactStrings() len=%d, want %d; got %v", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("compactStrings()[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

func TestCompactStrings_MaxBound(t *testing.T) {
	in := make([]string, 30)
	for i := range in {
		in[i] = "v" + string(rune('a'+i%26)) + strings.Repeat("x", i)
	}
	got := compactStrings(in, 5)
	if len(got) != 5 {
		t.Fatalf("compactStrings max bound: got %d items, want 5", len(got))
	}
}

func TestCompactStrings_EmptyInput(t *testing.T) {
	got := compactStrings(nil, 10)
	if len(got) != 0 {
		t.Fatalf("expected empty slice for nil input, got %v", got)
	}
}

func TestCompactStrings_DedupCaseInsensitive(t *testing.T) {
	in := []string{"SEO", "seo", "Seo"}
	got := compactStrings(in, 10)
	if len(got) != 1 {
		t.Fatalf("expected 1 deduped entry, got %d: %v", len(got), got)
	}
	// First occurrence (trimmed) is kept.
	if got[0] != "SEO" {
		t.Fatalf("expected first occurrence kept, got %q", got[0])
	}
}
