package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	claudeAPIURL          = "https://api.anthropic.com/v1/messages"
	claudeAPIVersion      = "2023-06-01"
	claudeDefaultModel    = "claude-sonnet-4-6"
	claudeMaxTokens       = 4096
	claudeRequestTimeout  = 90 * time.Second
)

// claudeMessage is a single turn in the messages array.
type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// claudeRequest is the POST body for Anthropic Messages API.
type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system"`
	Messages  []claudeMessage `json:"messages"`
}

// claudeResponseContent is one block in the response content array.
type claudeResponseContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// claudeUsage tracks token counts returned by the API.
type claudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// claudeResponse is the top-level response from Anthropic Messages API.
type claudeResponse struct {
	Content []claudeResponseContent `json:"content"`
	Usage   claudeUsage             `json:"usage"`
}

// ClaudeClient calls the Anthropic Messages API to generate content.
// Returns ErrAIUnavailable when APIKey is empty.
type ClaudeClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewClaudeClient constructs a ClaudeClient.
// model defaults to claudeDefaultModel when empty.
// Returns a client even when apiKey is empty; Generate() will return ErrAIUnavailable.
func NewClaudeClient(apiKey, model string) *ClaudeClient {
	if model == "" {
		model = claudeDefaultModel
	}
	return &ClaudeClient{
		apiKey: strings.TrimSpace(apiKey),
		model:  model,
		httpClient: &http.Client{
			Timeout: claudeRequestTimeout,
		},
	}
}

// Generate calls the Anthropic Messages API and returns a parsed ContentResponse.
// Returns ErrAIUnavailable if the API key is not configured.
func (c *ClaudeClient) Generate(ctx context.Context, req ContentRequest) (ContentResponse, error) {
	if c.apiKey == "" {
		return ContentResponse{}, ErrAIUnavailable
	}

	tone := req.TonePreference
	if tone == "" {
		tone = "professional"
	}

	systemPrompt := buildSystemPrompt()
	userPrompt := buildUserPrompt(req, tone)

	payload := claudeRequest{
		Model:     c.model,
		MaxTokens: claudeMaxTokens,
		System:    systemPrompt,
		Messages: []claudeMessage{
			{Role: "user", Content: userPrompt},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ContentResponse{}, fmt.Errorf("claude: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeAPIURL, bytes.NewReader(body))
	if err != nil {
		return ContentResponse{}, fmt.Errorf("claude: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", claudeAPIVersion)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return ContentResponse{}, fmt.Errorf("claude: http: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return ContentResponse{}, fmt.Errorf("claude: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Surface API error message to caller for diagnosis.
		snippet := string(respBytes)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return ContentResponse{}, fmt.Errorf("claude: status %d: %s", resp.StatusCode, snippet)
	}

	var apiResp claudeResponse
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return ContentResponse{}, fmt.Errorf("claude: unmarshal response: %w", err)
	}

	// Extract first text block from content array.
	rawText := ""
	for _, block := range apiResp.Content {
		if block.Type == "text" && block.Text != "" {
			rawText = block.Text
			break
		}
	}
	if rawText == "" {
		return ContentResponse{}, fmt.Errorf("claude: empty text content in response")
	}

	parsed, err := parseArticleJSON(rawText)
	if err != nil {
		return ContentResponse{}, fmt.Errorf("claude: parse article JSON: %w", err)
	}

	parsed.InputTokens = apiResp.Usage.InputTokens
	parsed.OutputTokens = apiResp.Usage.OutputTokens
	parsed.Provider = ProviderClaude
	return parsed, nil
}

// buildSystemPrompt returns the Vietnamese SEO expert system prompt.
func buildSystemPrompt() string {
	return `Bạn là chuyên gia content writer SEO Việt Nam. Viết bài 800-1500 từ tone tự nhiên, qua AI detection 95%, structure: intro hook + 3-5 sections với h2/h3 + conclusion CTA. Embed anchor link tự nhiên không gượng ép. KHÔNG dùng cụm từ AI điển hình ("As an AI", "I cannot", "However, it's important to note", "I'm unable").`
}

// buildUserPrompt builds the user turn with all article parameters.
func buildUserPrompt(req ContentRequest, tone string) string {
	return fmt.Sprintf(
		`Topic: %s. Anchor text: %s. Money URL: %s. Tone: %s. Trả về JSON (không markdown fence): { "title": "...", "slug": "...", "meta_description": "...", "body_html": "<h2>...</h2>...", "anchor_position": "intro|section_2|conclusion" }`,
		req.Topic, req.AnchorText, req.MoneyURL, tone,
	)
}
