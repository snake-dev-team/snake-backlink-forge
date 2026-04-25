// config_test.go — unit tests for validateAdminTelegramIDs (L3).
// No database required — pure unit tests on the config package.
package config

import (
	"testing"
)

// TestValidateAdminTelegramIDs_NoDupes: clean list passes through sorted, unchanged.
func TestValidateAdminTelegramIDs_NoDupes(t *testing.T) {
	input := []int64{300, 100, 200}
	out, err := validateAdminTelegramIDs(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(out))
	}
	// Must be sorted ascending.
	if out[0] != 100 || out[1] != 200 || out[2] != 300 {
		t.Fatalf("expected [100 200 300], got %v", out)
	}
}

// TestValidateAdminTelegramIDs_WithDupes: duplicates are deduplicated (warn, no error).
func TestValidateAdminTelegramIDs_WithDupes(t *testing.T) {
	input := []int64{100, 200, 100, 300, 200}
	out, err := validateAdminTelegramIDs(input)
	if err != nil {
		t.Fatalf("unexpected error on dupes: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 unique entries, got %d: %v", len(out), out)
	}
	// Must be sorted: [100, 200, 300].
	if out[0] != 100 || out[1] != 200 || out[2] != 300 {
		t.Fatalf("expected [100 200 300], got %v", out)
	}
}

// TestValidateAdminTelegramIDs_NonPositive: zero or negative value → error (fail boot).
func TestValidateAdminTelegramIDs_NonPositive(t *testing.T) {
	cases := []struct {
		name  string
		input []int64
	}{
		{"zero", []int64{0}},
		{"negative", []int64{-1}},
		{"mixed_valid_invalid", []int64{100, -5, 200}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateAdminTelegramIDs(tc.input)
			if err == nil {
				t.Fatalf("expected error for input %v, got nil", tc.input)
			}
		})
	}
}

// TestValidateAdminTelegramIDs_Empty: empty slice → empty result, no error.
func TestValidateAdminTelegramIDs_Empty(t *testing.T) {
	out, err := validateAdminTelegramIDs([]int64{})
	if err != nil {
		t.Fatalf("unexpected error on empty input: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty result, got %v", out)
	}
}
