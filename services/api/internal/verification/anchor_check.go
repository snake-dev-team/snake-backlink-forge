package verification

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

// VerifyAnchor fetches the HTML at resultURL and checks whether it contains an
// anchor tag whose href matches moneyURL (case-insensitive) and whose visible
// text contains anchorText (case-insensitive substring match).
//
// Returns (true, nil) if the anchor is found.
// Returns (false, nil) if the page loaded but anchor is absent (not an error — just unverified).
// Returns (false, err) if the page could not be fetched or HTML could not be parsed.
func VerifyAnchor(ctx context.Context, resultURL, moneyURL, anchorText string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resultURL, nil)
	if err != nil {
		return false, fmt.Errorf("anchor_check: build request: %w", err)
	}
	req.Header.Set("User-Agent", verifierUserAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("anchor_check: GET %s: %w", resultURL, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return false, fmt.Errorf("anchor_check: unexpected status %d for %s", resp.StatusCode, resultURL)
	}

	// Limit body read to 2 MB — sufficient for any blog post, prevents memory exhaustion.
	const maxBodyBytes = 2 * 1024 * 1024
	body := io.LimitReader(resp.Body, maxBodyBytes)

	doc, err := html.Parse(body)
	if err != nil {
		// html.Parse is lenient — errors here indicate corrupt/binary content.
		return false, fmt.Errorf("anchor_check: parse HTML from %s: %w", resultURL, err)
	}

	return findAnchorMatch(doc, moneyURL, anchorText), nil
}

// findAnchorMatch recursively walks the HTML tree looking for an <a> element
// whose href matches moneyURL and visible text contains anchorText.
// Case-insensitive for both href and text comparisons.
// Designed to be panic-safe — malformed HTML is handled gracefully by golang.org/x/net/html.
func findAnchorMatch(node *html.Node, moneyURL, anchorText string) bool {
	if node.Type == html.ElementNode && node.Data == "a" {
		var href string
		for _, attr := range node.Attr {
			if attr.Key == "href" {
				href = attr.Val
				break
			}
		}
		text := collectText(node)
		if strings.EqualFold(strings.TrimSpace(href), moneyURL) &&
			strings.Contains(strings.ToLower(text), strings.ToLower(anchorText)) {
			return true
		}
	}
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if findAnchorMatch(c, moneyURL, anchorText) {
			return true
		}
	}
	return false
}

// collectText recursively gathers all visible text content under a node,
// joining child text nodes with a single space. Used to extract anchor link text.
func collectText(node *html.Node) string {
	if node.Type == html.TextNode {
		return node.Data
	}
	var sb strings.Builder
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(collectText(c))
	}
	return sb.String()
}
