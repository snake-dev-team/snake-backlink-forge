// Package templates — lint test ensuring no secrets leak into bundle strings.
package templates

import (
	"strings"
	"testing"
)

// TestNoSecretsInBundles scans every bundle value for tokens/keywords that
// must never appear in a committed message string. A real key prefix or
// connection string in the bundle would expose internal credentials to every
// user who triggers that template.
//
// This is a defense-in-depth check — secret review primarily belongs in code
// review, but a programmatic test catches regressions when copy is updated.
func TestNoSecretsInBundles(t *testing.T) {
	// Each forbidden marker is a substring search (case-insensitive).
	// Patterns are intentionally narrow to avoid false positives on the
	// word "key" in user-facing copy ("API Key", "key prefix" are fine).
	forbidden := []string{
		"sbf_live_",      // real plaintext API key prefix
		"DATABASE_URL",   // env var name (would only appear in a config dump)
		"postgres://",    // connection string scheme
		"redis://",       // connection string scheme
		"telegram_bot_token",
		"sepay_apikey",
		"webhook_secret",
		"bearer ",
	}

	for lang, bundle := range bundles {
		for key, val := range bundle {
			lower := strings.ToLower(val)
			for _, marker := range forbidden {
				if strings.Contains(lower, strings.ToLower(marker)) {
					t.Errorf("bundle[%q][%q] contains forbidden marker %q", lang, key, marker)
				}
			}
		}
	}
}

// TestAllKeysListMatchesBundles ensures the AllKeys slice is consistent with
// what the bundles actually define — adding a key to the const block but
// forgetting AllKeys would silently skip the both-bundles enforcement test.
func TestAllKeysListMatchesBundles(t *testing.T) {
	allSet := make(map[string]bool, len(AllKeys))
	for _, k := range AllKeys {
		if allSet[k] {
			t.Errorf("AllKeys has duplicate entry: %q", k)
		}
		allSet[k] = true
	}

	// Cross-check: every vi bundle key should be in AllKeys (otherwise
	// TestRender_AllKeysHaveBothBundles can't enforce parity for it).
	for k := range bundles["vi"] {
		if strings.HasPrefix(k, "__test_only") {
			continue // dynamically seeded by TestRender_FallbackVI
		}
		if !allSet[k] {
			t.Errorf("vi bundle has key %q not in AllKeys — add it for parity check", k)
		}
	}
}
