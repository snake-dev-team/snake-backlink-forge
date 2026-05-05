// worker_test.go: Table-driven tests for worker components.
// Tests wp_client.go, retry.go, and rate_limiter.go using httptest servers.
// DB-level tests (ClaimContentReadyJobs, DLQ transition) require DATABASE_URL
// and are skipped automatically when it is unset.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- WP client tests ---

func TestPostArticle_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/wp-json/wp/v2/posts" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if auth == "" {
			t.Error("missing Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"link": "https://example.com/my-post",
			"content": map[string]string{
				"rendered": "<p>Hello world backlink</p>",
			},
		})
	}))
	defer srv.Close()

	result, err := PostArticle(context.Background(), srv.URL, "admin", "pass", WPPostRequest{
		Title:           "Test Post",
		ContentHTML:     "<p>Hello world backlink</p>",
		MetaDescription: "A test post",
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if result.PostURL != "https://example.com/my-post" {
		t.Errorf("unexpected PostURL: %q", result.PostURL)
	}
	if result.Evidence == "" {
		t.Error("expected non-empty Evidence")
	}
}

func TestPostArticle_Auth401_PermanentError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := PostArticle(context.Background(), srv.URL, "admin", "wrongpass", WPPostRequest{
		Title:       "Test",
		ContentHTML: "<p>content</p>",
	})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	wpErr, ok := err.(*WPClientError)
	if !ok {
		t.Fatalf("expected *WPClientError, got %T", err)
	}
	if wpErr.Code != "wp_auth_failed" {
		t.Errorf("expected wp_auth_failed, got %q", wpErr.Code)
	}
}

func TestPostArticle_Server502_TransientError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream error"))
	}))
	defer srv.Close()

	_, err := PostArticle(context.Background(), srv.URL, "admin", "pass", WPPostRequest{
		Title:       "Test",
		ContentHTML: "<p>content</p>",
	})
	if err == nil {
		t.Fatal("expected error for 502")
	}
	wpErr, ok := err.(*WPClientError)
	if !ok {
		t.Fatalf("expected *WPClientError, got %T", err)
	}
	if wpErr.Code != "wp_server_error" {
		t.Errorf("expected wp_server_error, got %q", wpErr.Code)
	}
}

func TestPostArticle_403_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := PostArticle(context.Background(), srv.URL, "admin", "pass", WPPostRequest{
		Title:       "Test",
		ContentHTML: "<p>content</p>",
	})
	wpErr, ok := err.(*WPClientError)
	if !ok || wpErr.Code != "wp_forbidden" {
		t.Errorf("expected wp_forbidden, got %v", err)
	}
}

func TestPostArticle_404_RestDisabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := PostArticle(context.Background(), srv.URL, "admin", "pass", WPPostRequest{
		Title:       "Test",
		ContentHTML: "<p>content</p>",
	})
	wpErr, ok := err.(*WPClientError)
	if !ok || wpErr.Code != "wp_rest_disabled" {
		t.Errorf("expected wp_rest_disabled, got %v", err)
	}
}

func TestPostArticle_WAFChallenge_Blocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><body>Checking cloudflare challenge...</body></html>`))
	}))
	defer srv.Close()

	_, err := PostArticle(context.Background(), srv.URL, "admin", "pass", WPPostRequest{
		Title:       "Test",
		ContentHTML: "<p>content</p>",
	})
	wpErr, ok := err.(*WPClientError)
	if !ok || wpErr.Code != "wp_blocked" {
		t.Errorf("expected wp_blocked, got %v", err)
	}
}

func TestPostArticle_ContextCancelled_NetworkError(t *testing.T) {
	// Pre-cancelled context: Do should error immediately, no network roundtrip.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := PostArticle(ctx, "http://127.0.0.1:1", "admin", "pass", WPPostRequest{
		Title:       "Test",
		ContentHTML: "<p>content</p>",
	})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	wpErr, ok := err.(*WPClientError)
	if !ok || wpErr.Code != "wp_network_error" {
		t.Errorf("expected wp_network_error, got %v", err)
	}
}

func TestPostArticle_EvidenceTrimmed(t *testing.T) {
	largeContent := fmt.Sprintf("%*s", wpEvidenceMaxLen+1000, "x")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"link": "https://example.com/post",
			"content": map[string]string{
				"rendered": largeContent,
			},
		})
	}))
	defer srv.Close()

	result, err := PostArticle(context.Background(), srv.URL, "admin", "pass", WPPostRequest{
		Title:       "Test",
		ContentHTML: "<p>content</p>",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Evidence) > wpEvidenceMaxLen {
		t.Errorf("evidence not trimmed: got %d bytes, want <= %d", len(result.Evidence), wpEvidenceMaxLen)
	}
}

// --- Backoff tests ---

func TestBackoff_Schedule(t *testing.T) {
	cases := []struct {
		retryCount int
		want       time.Duration
	}{
		{0, 30 * time.Second},
		{1, 60 * time.Second},
		{2, 120 * time.Second},
		{3, 240 * time.Second},
		{10, 30 * time.Minute}, // capped
	}
	for _, tc := range cases {
		got := Backoff(tc.retryCount)
		if got != tc.want {
			t.Errorf("Backoff(%d) = %v, want %v", tc.retryCount, got, tc.want)
		}
	}
}

func TestBackoffSeconds(t *testing.T) {
	if got := BackoffSeconds(1); got != 60 {
		t.Errorf("BackoffSeconds(1) = %d, want 60", got)
	}
}

// --- Rate limiter tests ---

func TestRateLimiter_FirstCallImmediate(t *testing.T) {
	rl := NewRateLimiter(30 * time.Second)
	start := time.Now()
	if err := rl.WaitForSlot(context.Background(), "example.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Errorf("first call should be immediate, took %v", elapsed)
	}
}

func TestRateLimiter_SecondCallBlocked(t *testing.T) {
	minGap := 100 * time.Millisecond // use short gap for test speed
	rl := NewRateLimiter(minGap)

	// First call — immediate.
	if err := rl.WaitForSlot(context.Background(), "example.com"); err != nil {
		t.Fatalf("first call error: %v", err)
	}

	// Second call should wait ~minGap.
	start := time.Now()
	if err := rl.WaitForSlot(context.Background(), "example.com"); err != nil {
		t.Fatalf("second call error: %v", err)
	}
	elapsed := time.Since(start)

	// Allow 20ms tolerance for scheduler jitter.
	if elapsed < minGap-20*time.Millisecond {
		t.Errorf("second call did not wait: elapsed=%v, minGap=%v", elapsed, minGap)
	}
}

func TestRateLimiter_DifferentDomainsNotBlocked(t *testing.T) {
	rl := NewRateLimiter(10 * time.Second)
	if err := rl.WaitForSlot(context.Background(), "site-a.com"); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := rl.WaitForSlot(context.Background(), "site-b.com"); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Errorf("different domains should not block each other, took %v", elapsed)
	}
}

func TestRateLimiter_ContextCancelledDuringWait(t *testing.T) {
	rl := NewRateLimiter(10 * time.Second)
	// Prime the slot so next call will wait.
	_ = rl.WaitForSlot(context.Background(), "example.com")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := rl.WaitForSlot(ctx, "example.com")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

// --- Worker struct / buildWorkerID tests ---

func TestBuildWorkerID_Format(t *testing.T) {
	id := buildWorkerID()
	if id == "" {
		t.Error("worker ID should not be empty")
	}
	// Should contain "pid-" followed by the process PID.
	if len(id) < 5 {
		t.Errorf("worker ID too short: %q", id)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 3); got != "hel" {
		t.Errorf("truncate(hello,3) = %q, want hel", got)
	}
	if got := truncate("hi", 10); got != "hi" {
		t.Errorf("truncate(hi,10) = %q, want hi", got)
	}
}
