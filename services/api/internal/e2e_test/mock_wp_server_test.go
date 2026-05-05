//go:build e2e

package e2e_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
)

// MockWPResponse is what our mock WP REST API returns when a post is created.
type MockWPResponse struct {
	ID      int    `json:"id"`
	Link    string `json:"link"`
	Content struct {
		Rendered string `json:"rendered"`
	} `json:"content"`
}

// NewMockWPServer creates a test HTTP server that mimics WordPress REST API /wp-json/wp/v2/posts endpoint.
// Each POST request returns 201 Created with a valid response.
func NewMockWPServer(baseURLStr string) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only handle POST /wp-json/wp/v2/posts
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/wp-json/wp/v2/posts" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Simulate successful post creation.
		postID := 12345 // dummy ID
		postURL := fmt.Sprintf("%s/article-test-%d/", baseURLStr, postID)

		// The response must include the anchor link as evidence.
		htmlContent := fmt.Sprintf(
			`<h2>Test Article %d</h2>
<p>Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.</p>
<a href="https://test-money-site.example.com">broker uy tín</a>
<p>Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.</p>`,
			postID,
		)

		resp := MockWPResponse{
			ID:   postID,
			Link: postURL,
			Content: struct {
				Rendered string `json:"rendered"`
			}{
				Rendered: htmlContent,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	})

	return httptest.NewServer(handler)
}
