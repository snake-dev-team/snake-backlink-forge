package verification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ─────────────────────────── HTTPHead tests ───────────────────────────────

func TestHTTPHead_Returns200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	code, ct, err := HTTPHead(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("HTTPHead: unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Fatalf("HTTPHead: want 200, got %d", code)
	}
	if !strings.Contains(ct, "text/html") {
		t.Fatalf("HTTPHead: want text/html content-type, got %q", ct)
	}
}

func TestHTTPHead_Returns404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	code, _, err := HTTPHead(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("HTTPHead 404: unexpected error: %v", err)
	}
	if code != http.StatusNotFound {
		t.Fatalf("HTTPHead 404: want 404, got %d", code)
	}
}

func TestHTTPHead_MethodNotAllowedFallsBackToGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	code, ct, err := HTTPHead(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("HTTPHead fallback: unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Fatalf("HTTPHead fallback: want 200, got %d", code)
	}
	if !strings.Contains(ct, "text/html") {
		t.Fatalf("HTTPHead fallback: want text/html, got %q", ct)
	}
}

func TestHTTPHead_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(30 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, _, err := HTTPHead(ctx, srv.URL)
	if err == nil {
		t.Fatal("HTTPHead timeout: expected error for stalled server, got nil")
	}
}

func TestHTTPHead_UserAgentPropagated(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if _, _, err := HTTPHead(context.Background(), srv.URL); err != nil {
		t.Fatalf("HTTPHead UA: %v", err)
	}
	if gotUA != verifierUserAgent {
		t.Fatalf("HTTPHead UA: want %q, got %q", verifierUserAgent, gotUA)
	}
}

// ─────────────────────────── VerifyAnchor tests ───────────────────────────

func serveHTML(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
}

func TestVerifyAnchor_FoundExact(t *testing.T) {
	const moneyURL = "https://my-money-site.com"
	const anchorText = "best SEO tool"
	body := `<!DOCTYPE html><html><body><a href="` + moneyURL + `">` + anchorText + `</a></body></html>`

	srv := serveHTML(body)
	defer srv.Close()

	found, err := VerifyAnchor(context.Background(), srv.URL, moneyURL, anchorText)
	if err != nil {
		t.Fatalf("VerifyAnchor found: %v", err)
	}
	if !found {
		t.Fatal("VerifyAnchor found: expected true, got false")
	}
}

func TestVerifyAnchor_CaseInsensitive(t *testing.T) {
	// Serve lowercase href + text; query with mixed-case.
	const servedHref = "https://my-money-site.com"
	const servedText = "best seo tool"
	body := `<html><body><a href="` + servedHref + `">` + servedText + `</a></body></html>`

	srv := serveHTML(body)
	defer srv.Close()

	found, err := VerifyAnchor(context.Background(), srv.URL, "https://My-Money-Site.COM", "Best SEO Tool")
	if err != nil {
		t.Fatalf("VerifyAnchor case: %v", err)
	}
	if !found {
		t.Fatal("VerifyAnchor case: expected case-insensitive match, got false")
	}
}

func TestVerifyAnchor_AnchorMissing(t *testing.T) {
	srv := serveHTML(`<html><body><p>No anchor here at all</p></body></html>`)
	defer srv.Close()

	found, err := VerifyAnchor(context.Background(), srv.URL, "https://money.example.com", "click here")
	if err != nil {
		t.Fatalf("VerifyAnchor missing: unexpected error: %v", err)
	}
	if found {
		t.Fatal("VerifyAnchor missing: expected false, got true")
	}
}

func TestVerifyAnchor_WrongAnchorText(t *testing.T) {
	const moneyURL = "https://money.example.com"
	body := `<html><body><a href="` + moneyURL + `">completely different</a></body></html>`

	srv := serveHTML(body)
	defer srv.Close()

	found, err := VerifyAnchor(context.Background(), srv.URL, moneyURL, "expected text")
	if err != nil {
		t.Fatalf("VerifyAnchor wrong text: %v", err)
	}
	if found {
		t.Fatal("VerifyAnchor wrong text: expected false, got true")
	}
}

func TestVerifyAnchor_PageReturns404_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	found, err := VerifyAnchor(context.Background(), srv.URL, "https://money.example.com", "anchor")
	if err == nil {
		t.Fatal("VerifyAnchor 404: expected error, got nil")
	}
	if found {
		t.Fatal("VerifyAnchor 404: expected false, got true")
	}
}

func TestVerifyAnchor_MalformedHTML_NoPanic(t *testing.T) {
	// golang.org/x/net/html is lenient — must not panic on broken tags.
	srv := serveHTML(`<html><body><p>unclosed <a href="https://money.example.com">anchor text<p>more</body>`)
	defer srv.Close()

	// Should not panic. Return value may be true or false depending on parser recovery.
	_, err := VerifyAnchor(context.Background(), srv.URL, "https://money.example.com", "anchor text")
	if err != nil {
		t.Fatalf("VerifyAnchor malformed: unexpected error (parser should be lenient): %v", err)
	}
}

func TestVerifyAnchor_AnchorInNestedElement(t *testing.T) {
	// Anchor text split across nested spans — collectText should join them.
	const moneyURL = "https://money.example.com"
	body := `<html><body><a href="` + moneyURL + `"><span>my</span> <em>anchor</em> text</a></body></html>`

	srv := serveHTML(body)
	defer srv.Close()

	found, err := VerifyAnchor(context.Background(), srv.URL, moneyURL, "anchor")
	if err != nil {
		t.Fatalf("VerifyAnchor nested: %v", err)
	}
	if !found {
		t.Fatal("VerifyAnchor nested: expected true for nested text, got false")
	}
}

// ─────────────────────────── boolPtr utility test ─────────────────────────

func TestBoolPtr(t *testing.T) {
	trueVal := boolPtr(true)
	falseVal := boolPtr(false)
	if trueVal == nil || *trueVal != true {
		t.Fatal("boolPtr(true): expected non-nil *true")
	}
	if falseVal == nil || *falseVal != false {
		t.Fatal("boolPtr(false): expected non-nil *false")
	}
	if trueVal == falseVal {
		t.Fatal("boolPtr: same pointer returned for different values")
	}
}
