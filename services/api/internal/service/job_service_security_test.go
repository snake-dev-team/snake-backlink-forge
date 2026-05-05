package service

import "testing"

func TestValidateResultURLRequiresSameTargetHost(t *testing.T) {
	got, err := validateResultURL("https://target.example/path?ok=1", "https://target.example/form")
	if err != nil {
		t.Fatalf("validateResultURL returned error for matching host: %v", err)
	}
	if got == nil || *got != "https://target.example/path?ok=1" {
		t.Fatalf("unexpected sanitized result: %#v", got)
	}

	if _, err := validateResultURL("https://attacker.example/fake", "https://target.example/form"); err != ErrJobInvalid {
		t.Fatalf("expected ErrJobInvalid for foreign host, got %v", err)
	}
}

func TestValidateResultURLRejectsNonHTTPAndEmpty(t *testing.T) {
	cases := []string{"", "javascript:alert(1)", "ftp://target.example/file", "not a url"}
	for _, tc := range cases {
		if _, err := validateResultURL(tc, "https://target.example/form"); err != ErrJobInvalid {
			t.Fatalf("expected ErrJobInvalid for %q, got %v", tc, err)
		}
	}
}

func TestValidateBacklinkEvidenceRequiresMoneyURLAndAnchor(t *testing.T) {
	moneyURL := "https://money.example/service"
	anchor := "SEO Services"
	valid := `<a href="https://money.example/service">SEO Services</a>`
	if err := validateBacklinkEvidence(valid, moneyURL, anchor); err != nil {
		t.Fatalf("expected valid evidence, got %v", err)
	}

	cases := []struct {
		name     string
		evidence string
	}{
		{name: "empty", evidence: ""},
		{name: "missing money URL", evidence: `<a href="https://other.example/service">SEO Services</a>`},
		{name: "missing anchor", evidence: `<a href="https://money.example/service">click here</a>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateBacklinkEvidence(tc.evidence, moneyURL, anchor); err != ErrJobInvalid {
				t.Fatalf("expected ErrJobInvalid, got %v", err)
			}
		})
	}
}

func TestNormalizeReportStatus(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{name: "success", input: "success", want: "success", wantOK: true},
		{name: "success trim upper", input: " Success ", want: "success", wantOK: true},
		{name: "failed", input: "failed", want: "failed", wantOK: true},
		{name: "skipped", input: "skipped", want: "skipped", wantOK: true},
		{name: "invalid", input: "queued", wantOK: false},
		{name: "empty", input: " ", wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := normalizeReportStatus(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("normalizeReportStatus(%q) ok=%v want %v", tc.input, ok, tc.wantOK)
			}
			if got != tc.want {
				t.Fatalf("normalizeReportStatus(%q)=%q want %q", tc.input, got, tc.want)
			}
		})
	}
}
