package util

import (
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	t.Parallel()

	type tc struct {
		name    string
		input   string
		want    string // empty means expect error
		wantErr bool
	}

	cases := []tc{
		// ── Standard 10-digit VN numbers ─────────────────────────────────────
		{name: "local_0_prefix", input: "0987654321", want: "+84987654321"},
		{name: "local_0_prefix_alt", input: "0912345678", want: "+84912345678"},
		{name: "local_0_prefix_032", input: "0321234567", want: "+84321234567"},
		{name: "local_0_prefix_035", input: "0351234567", want: "+84351234567"},
		{name: "local_0_prefix_039", input: "0391234567", want: "+84391234567"},

		// ── Already has country code ──────────────────────────────────────────
		{name: "plus84", input: "+84987654321", want: "+84987654321"},
		{name: "plus84_9digit_base", input: "+84912345678", want: "+84912345678"},

		// ── Bare 84 prefix (no +) ────────────────────────────────────────────
		{name: "bare_84", input: "84987654321", want: "+84987654321"},
		{name: "bare_84_alt", input: "84912345678", want: "+84912345678"},

		// ── Formatted with separators ─────────────────────────────────────────
		{name: "dashes", input: "098-765-4321", want: "+84987654321"},
		{name: "spaces", input: "098 765 4321", want: "+84987654321"},
		{name: "84_dashes", input: "84-987-654-321", want: "+84987654321"},
		{name: "plus84_spaces", input: "+84 987 654 321", want: "+84987654321"},
		{name: "mixed_format", input: "+84(987)654-321", want: "+84987654321"},

		// ── International (non-VN) ────────────────────────────────────────────
		{name: "us_number", input: "+12125551234", want: "+12125551234"},

		// ── Invalid / garbage ─────────────────────────────────────────────────
		{name: "empty", input: "", wantErr: true},
		{name: "garbage", input: "not-a-phone", wantErr: true},
		{name: "too_short", input: "0123", wantErr: true},
		{name: "letters_mixed", input: "098ABC4321", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizePhone(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("NormalizePhone(%q) = %q, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("NormalizePhone(%q) unexpected error: %v", tc.input, err)
				return
			}
			if got != tc.want {
				t.Errorf("NormalizePhone(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
