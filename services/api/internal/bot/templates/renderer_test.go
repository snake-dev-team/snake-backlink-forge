// Package templates — unit tests for Renderer + EscapeMDV2.
package templates

import (
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"
)

// TestRender_KnownKey verifies a known key renders with data interpolated.
func TestRender_KnownKey(t *testing.T) {
	r := NewRenderer(zap.NewNop())
	out := r.Render("vi", KeyBalance, map[string]any{
		"Premium":           100,
		"Standard":          50,
		"TotalVNDFormatted": "329,000",
	})

	if !strings.Contains(out, "Premium") {
		t.Errorf("expected 'Premium' literal, got: %s", out)
	}
	if !strings.Contains(out, "100") {
		t.Errorf("expected interpolated '100', got: %s", out)
	}
	if !strings.Contains(out, "50") {
		t.Errorf("expected interpolated '50', got: %s", out)
	}
	if !strings.Contains(out, "329,000") {
		t.Errorf("expected interpolated '329,000', got: %s", out)
	}
}

// TestRender_UnknownKey verifies the bracket-literal sentinel for missing keys.
func TestRender_UnknownKey(t *testing.T) {
	r := NewRenderer(zap.NewNop())
	out := r.Render("vi", "nonexistent_key_xyz", nil)

	want := "[nonexistent_key_xyz]"
	if out != want {
		t.Errorf("Render(unknown) = %q, want %q", out, want)
	}
}

// TestRender_FallbackVI verifies that when a key is missing in EN bundle,
// Renderer falls back to VI. We seed a temporary key in vi only.
func TestRender_FallbackVI(t *testing.T) {
	const tmpKey = "__test_only_vi__"
	bundles["vi"][tmpKey] = "vi-only content"
	defer delete(bundles["vi"], tmpKey)

	r := NewRenderer(zap.NewNop())
	out := r.Render("en", tmpKey, nil)

	if out != "vi-only content" {
		t.Errorf("expected VI fallback 'vi-only content', got: %q", out)
	}
}

// TestRender_AllKeysHaveBothBundles enforces that every constant in AllKeys
// has an entry in BOTH vi and en bundles, and the entry is non-empty.
func TestRender_AllKeysHaveBothBundles(t *testing.T) {
	for _, k := range AllKeys {
		viVal, viOk := bundles["vi"][k]
		if !viOk {
			t.Errorf("key %q missing in vi bundle", k)
			continue
		}
		if strings.TrimSpace(viVal) == "" {
			t.Errorf("key %q empty in vi bundle", k)
		}

		enVal, enOk := bundles["en"][k]
		if !enOk {
			t.Errorf("key %q missing in en bundle", k)
			continue
		}
		if strings.TrimSpace(enVal) == "" {
			t.Errorf("key %q empty in en bundle", k)
		}
	}
}

// TestRender_DataInterpolation verifies struct-field interpolation works
// for KeyBalance. Uses a typed struct (mirrors real call sites).
func TestRender_DataInterpolation(t *testing.T) {
	type balanceData struct {
		Premium           int
		Standard          int
		TotalVNDFormatted string
	}

	r := NewRenderer(zap.NewNop())
	out := r.Render("vi", KeyBalance, balanceData{
		Premium:           100,
		Standard:          50,
		TotalVNDFormatted: "329,000",
	})

	for _, want := range []string{"100", "50", "329,000"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got: %s", want, out)
		}
	}
}

// TestEscapeMDV2_AllChars verifies every reserved MarkdownV2 char is escaped.
func TestEscapeMDV2_AllChars(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`hello_world`, `hello\_world`},
		{`*bold*`, `\*bold\*`},
		{`[link]`, `\[link\]`},
		{`(group)`, `\(group\)`},
		{`a~b`, `a\~b`},
		{"`code`", "\\`code\\`"},
		{`>quote`, `\>quote`},
		{`#hash`, `\#hash`},
		{`a+b`, `a\+b`},
		{`a-b`, `a\-b`},
		{`a=b`, `a\=b`},
		{`a|b`, `a\|b`},
		{`{block}`, `\{block\}`},
		{`end.`, `end\.`},
		{`hi!`, `hi\!`},
	}
	for _, tc := range cases {
		got := EscapeMDV2(tc.in)
		if got != tc.want {
			t.Errorf("EscapeMDV2(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	// Smoke test: every special char in mdV2Special is present in escaped output.
	mixed := strings.Join(mdV2Special, "")
	got := EscapeMDV2(mixed)
	for _, c := range mdV2Special {
		if !strings.Contains(got, `\`+c) {
			t.Errorf("EscapeMDV2 did not escape %q in mixed input", c)
		}
	}
}

// TestRender_CacheHit exercises the cache path: rendering the same key
// many times concurrently must not race or panic. Loose check — relies
// on -race detector when go test -race is invoked.
func TestRender_CacheHit(t *testing.T) {
	r := NewRenderer(zap.NewNop())
	const N = 1000
	const G = 16

	var wg sync.WaitGroup
	for g := 0; g < G; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < N/G; i++ {
				out := r.Render("vi", KeyBalance, map[string]any{
					"Premium":           i,
					"Standard":          i * 2,
					"TotalVNDFormatted": "0",
				})
				if !strings.Contains(out, "Premium") {
					t.Errorf("cached render missing 'Premium' literal: %s", out)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestRender_NilLogger verifies NewRenderer accepts nil and substitutes nop.
func TestRender_NilLogger(t *testing.T) {
	r := NewRenderer(nil)
	out := r.Render("vi", KeyBuyCancelled, nil)
	if out == "" {
		t.Error("expected non-empty render with nil logger, got empty string")
	}
}
