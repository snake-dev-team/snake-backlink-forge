package ai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// articleJSON is the expected JSON structure returned by the AI model.
type articleJSON struct {
	Title          string `json:"title"`
	Slug           string `json:"slug"`
	MetaDescription string `json:"meta_description"`
	BodyHTML       string `json:"body_html"`
	AnchorPosition string `json:"anchor_position"`
}

// htmlTagRe strips HTML tags for word counting. RE2-safe.
var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

// parseArticleJSON extracts the JSON object from the model's raw text output.
// The model is instructed to return raw JSON, but sometimes wraps it in markdown
// fences (```json ... ```) — this handles both cases.
func parseArticleJSON(raw string) (ContentResponse, error) {
	raw = strings.TrimSpace(raw)

	// Strip markdown code fence if present.
	if strings.HasPrefix(raw, "```") {
		// Find the closing fence.
		end := strings.LastIndex(raw, "```")
		if end > 3 {
			// Remove opening fence line and closing fence.
			start := strings.Index(raw, "\n")
			if start != -1 && start < end {
				raw = strings.TrimSpace(raw[start:end])
			}
		}
	}

	// Find outermost JSON object boundaries to tolerate leading/trailing prose.
	first := strings.Index(raw, "{")
	last := strings.LastIndex(raw, "}")
	if first == -1 || last == -1 || last <= first {
		return ContentResponse{}, fmt.Errorf("no JSON object found in model output")
	}
	raw = raw[first : last+1]

	var article articleJSON
	if err := json.Unmarshal([]byte(raw), &article); err != nil {
		return ContentResponse{}, fmt.Errorf("unmarshal article: %w", err)
	}

	if strings.TrimSpace(article.Title) == "" {
		return ContentResponse{}, fmt.Errorf("article title is empty")
	}
	if strings.TrimSpace(article.BodyHTML) == "" {
		return ContentResponse{}, fmt.Errorf("article body_html is empty")
	}

	return ContentResponse{
		Title:          strings.TrimSpace(article.Title),
		Slug:           strings.TrimSpace(article.Slug),
		MetaDescription: strings.TrimSpace(article.MetaDescription),
		BodyHTML:       strings.TrimSpace(article.BodyHTML),
		AnchorPosition: strings.TrimSpace(article.AnchorPosition),
	}, nil
}

// WordCount strips HTML tags from html and counts whitespace-delimited words.
// Used by quality guards in ContentService.
func WordCount(html string) int {
	plain := htmlTagRe.ReplaceAllString(html, " ")
	words := strings.Fields(plain)
	return len(words)
}
