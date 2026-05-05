// Package verification implements post-publish link verification for SBF.
// After the worker publishes an article, the verifier checks that the result URL
// is reachable and contains the expected anchor link back to the money site.
package verification

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const (
	verifierUserAgent = "SBF-Verifier/1.0 (+https://snake-backlink-forge.fly.dev)"
	httpTimeout       = 10 * time.Second
	maxRedirects      = 3
)

// httpClient is a package-level client reused across all verifications.
// MaxIdleConns=5 avoids keeping excessive idle connections open (constraint from spec).
var httpClient = &http.Client{
	Timeout: httpTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        5,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     30 * time.Second,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("too many redirects (max %d)", maxRedirects)
		}
		// Propagate User-Agent through redirects.
		req.Header.Set("User-Agent", verifierUserAgent)
		return nil
	},
}

// HTTPHead performs a HEAD request (falling back to GET if HEAD returns 405) to check
// whether the given URL is reachable. Returns the HTTP status code and content-type.
// Respects the 10s timeout via ctx. Does not follow more than maxRedirects redirects.
func HTTPHead(ctx context.Context, rawURL string) (statusCode int, contentType string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return 0, "", fmt.Errorf("http_check: build HEAD request: %w", err)
	}
	req.Header.Set("User-Agent", verifierUserAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("http_check: HEAD %s: %w", rawURL, err)
	}
	resp.Body.Close() //nolint:errcheck

	// Some servers return 405 Method Not Allowed for HEAD — fall back to GET.
	if resp.StatusCode == http.StatusMethodNotAllowed {
		getReq, getErr := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if getErr != nil {
			return resp.StatusCode, "", fmt.Errorf("http_check: build GET fallback: %w", getErr)
		}
		getReq.Header.Set("User-Agent", verifierUserAgent)

		getResp, getErr := httpClient.Do(getReq)
		if getErr != nil {
			return resp.StatusCode, "", fmt.Errorf("http_check: GET fallback %s: %w", rawURL, getErr)
		}
		getResp.Body.Close() //nolint:errcheck
		return getResp.StatusCode, getResp.Header.Get("Content-Type"), nil
	}

	return resp.StatusCode, resp.Header.Get("Content-Type"), nil
}
