// Package worker implements the embedded background job processor for Phase 7.05.
// wp_client.go: Go port of apps/extension/src/wp/poster.ts — posts articles via WP REST API.
package worker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	wpTimeout        = 60 * time.Second
	wpEvidenceMaxLen = 16_000 // 16 KB — matches extension and job_service.evidenceMaxBytes
)

// WPPostRequest holds the article fields to publish on the target WordPress site.
type WPPostRequest struct {
	Title           string
	ContentHTML     string
	MetaDescription string // mapped to WP post excerpt
}

// WPPublishResult is returned on a successful WP REST API publish.
type WPPublishResult struct {
	PostURL  string // value of the "link" field in WP REST response
	Evidence string // content.rendered trimmed to wpEvidenceMaxLen
}

// WPClientError wraps a failure from the WP REST API with a machine-readable code.
// The codes mirror the extension poster.ts table: wp_auth_failed, wp_forbidden,
// wp_rest_disabled, wp_client_error, wp_server_error, wp_network_error, wp_blocked.
type WPClientError struct {
	Code    string
	Message string
}

func (e *WPClientError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return e.Code
}

// wpPostBody is the JSON body sent to /wp-json/wp/v2/posts.
type wpPostBody struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Status  string `json:"status"`
	Excerpt string `json:"excerpt,omitempty"`
}

// wpPostResponse is the subset of the WP REST API 201 response we parse.
type wpPostResponse struct {
	Link    string `json:"link"`
	Content struct {
		Rendered string `json:"rendered"`
	} `json:"content"`
}

// wpHTTPClient is package-level to allow connection reuse across jobs.
// 60s timeout matches the extension's AbortController timeout.
var wpHTTPClient = &http.Client{Timeout: wpTimeout}

// PostArticle publishes an article to a WordPress site via the WP REST API.
//
// Auth: Authorization: Basic base64(appUsername:appPasswordPlain)
// Endpoint: POST {baseURL}/wp-json/wp/v2/posts
//
// Returns WPPublishResult on success, *WPClientError on any failure.
// Error codes: wp_auth_failed (401), wp_forbidden (403), wp_rest_disabled (404),
// wp_blocked (WAF challenge), wp_client_error (other 4xx), wp_server_error (5xx),
// wp_network_error (timeout / DNS / network).
func PostArticle(ctx context.Context, baseURL, appUsername, appPasswordPlain string, req WPPostRequest) (WPPublishResult, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/wp-json/wp/v2/posts"

	// Basic auth — base64(username:password), identical to extension's btoa().
	creds := base64.StdEncoding.EncodeToString([]byte(appUsername + ":" + appPasswordPlain))

	bodyBytes, err := json.Marshal(wpPostBody{
		Title:   req.Title,
		Content: req.ContentHTML,
		Status:  "publish",
		Excerpt: req.MetaDescription,
	})
	if err != nil {
		return WPPublishResult{}, &WPClientError{Code: "wp_network_error", Message: "marshal body: " + err.Error()}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return WPPublishResult{}, &WPClientError{Code: "wp_network_error", Message: "build request: " + err.Error()}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Basic "+creds)

	resp, err := wpHTTPClient.Do(httpReq)
	if err != nil {
		// Network error, timeout, DNS failure, or context cancellation.
		if ctx.Err() != nil {
			return WPPublishResult{}, &WPClientError{Code: "wp_network_error", Message: "context: " + ctx.Err().Error()}
		}
		return WPPublishResult{}, &WPClientError{Code: "wp_network_error", Message: err.Error()}
	}
	defer resp.Body.Close()

	// Read body for all paths — needed for error messages and success parsing.
	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024)) // 512KB safety cap
	if err != nil {
		return WPPublishResult{}, &WPClientError{Code: "wp_network_error", Message: "read body: " + err.Error()}
	}
	bodyText := string(rawBody)

	switch resp.StatusCode {
	case http.StatusUnauthorized: // 401 — Application Password expired or revoked
		return WPPublishResult{}, &WPClientError{Code: "wp_auth_failed"}
	case http.StatusForbidden: // 403 — insufficient permissions
		return WPPublishResult{}, &WPClientError{Code: "wp_forbidden"}
	case http.StatusNotFound: // 404 — REST API disabled by security plugin
		return WPPublishResult{}, &WPClientError{Code: "wp_rest_disabled"}
	}

	// WAF / Cloudflare challenge heuristic — same as extension poster.ts.
	if lc := strings.ToLower(bodyText); strings.Contains(lc, "cloudflare") ||
		strings.Contains(lc, "challenge") || strings.Contains(lc, "captcha") {
		return WPPublishResult{}, &WPClientError{Code: "wp_blocked", Message: "WAF challenge detected"}
	}

	if resp.StatusCode >= 500 {
		return WPPublishResult{}, &WPClientError{Code: "wp_server_error", Message: truncate(bodyText, 200)}
	}

	if resp.StatusCode >= 400 {
		return WPPublishResult{}, &WPClientError{Code: "wp_client_error", Message: truncate(bodyText, 200)}
	}

	// Parse 201 Created (or 200 OK from some WP setups).
	var parsed wpPostResponse
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return WPPublishResult{}, &WPClientError{Code: "wp_client_error", Message: "non-json response: " + truncate(bodyText, 100)}
	}

	if parsed.Link == "" || parsed.Content.Rendered == "" {
		return WPPublishResult{}, &WPClientError{Code: "wp_client_error", Message: "missing link or content in response"}
	}

	return WPPublishResult{
		PostURL:  parsed.Link,
		Evidence: truncate(parsed.Content.Rendered, wpEvidenceMaxLen),
	}, nil
}

// truncate returns s trimmed to at most n bytes (safe for non-UTF8 strings).
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
