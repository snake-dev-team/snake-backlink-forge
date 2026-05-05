// config_test.go — unit tests for validateAdminTelegramIDs (L3).
// No database required — pure unit tests on the config package.
package config

import (
	"testing"
)

// TestValidateAdminTelegramIDs_NoDupes: clean list passes through sorted, unchanged.
func TestValidateSePayWebhookConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{name: "valid", cfg: Config{SepayWebhookToken: "12345678901234567890123456789012", SepayBankAccount: "123456789"}},
		{name: "empty token", cfg: Config{SepayBankAccount: "123456789"}, wantErr: true},
		{name: "short token", cfg: Config{SepayWebhookToken: "short", SepayBankAccount: "123456789"}, wantErr: true},
		{name: "missing bank account", cfg: Config{SepayWebhookToken: "12345678901234567890123456789012"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSePayWebhookConfig(&tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

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

func TestValidateWorkerRuntime(t *testing.T) {
	tests := []struct {
		name       string
		cfg        Config
		wantErr    bool
		wantModel  string
	}{
		{
			name:      "valid explicit model",
			cfg:       Config{WorkerLeaseSec: 300, WorkerRetryLimit: 3, WorkerModel: "worker-model", ClaudeModel: "claude-model"},
			wantModel: "worker-model",
		},
		{
			name:      "fallback to claude model",
			cfg:       Config{WorkerLeaseSec: 300, WorkerRetryLimit: 3, ClaudeModel: "claude-model"},
			wantModel: "claude-model",
		},
		{
			name:    "reject non-positive lease",
			cfg:     Config{WorkerLeaseSec: 0, WorkerRetryLimit: 3},
			wantErr: true,
		},
		{
			name:    "reject negative retry limit",
			cfg:     Config{WorkerLeaseSec: 300, WorkerRetryLimit: -1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			err := validateWorkerRuntime(&cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && cfg.WorkerModel != tt.wantModel {
				t.Fatalf("worker model = %q, want %q", cfg.WorkerModel, tt.wantModel)
			}
		})
	}
}
