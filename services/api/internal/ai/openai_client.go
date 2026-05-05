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
	openAIAPIURL         = "https://api.openai.com/v1/chat/completions"
	openAIDefaultModel   = "gpt-4o-mini"
	openAIRequestTimeout = 90 * time.Second
)

// openAIMessage is a single turn in the messages array.
type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIRequest is the POST body for OpenAI Chat Completions API.
type openAIRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []openAIMessage `json:"messages"`
}

// openAIChoice is one completion choice in the response.
type openAIChoice struct {
	Message openAIMessage `json:"message"`
}

// openAIUsage tracks token counts returned by the API.
type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// openAIResponse is the top-level response from OpenAI Chat Completions.
type openAIResponse struct {
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
}

// OpenAIClient calls the OpenAI Chat Completions API to generate content.
// Used as the fallback when ClaudeClient fails or is unavailable.
// Returns ErrAIUnavailable when APIKey is empty.
type OpenAIClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewOpenAIClient constructs an OpenAIClient.
// model defaults to openAIDefaultModel when empty.
// Returns a client even when apiKey is empty; Generate() will return ErrAIUnavailable.
func NewOpenAIClient(apiKey, model string) *OpenAIClient {
	if model == "" {
		model = openAIDefaultModel
	}
	return &OpenAIClient{
		apiKey: strings.TrimSpace(apiKey),
		model:  model,
		httpClient: &http.Client{
			Timeout: openAIRequestTimeout,
		},
	}
}

// Generate calls the OpenAI Chat Completions API and returns a parsed ContentResponse.
// Returns ErrAIUnavailable if the API key is not configured.
func (c *OpenAIClient) Generate(ctx context.Context, req ContentRequest) (ContentResponse, error) {
	if c.apiKey == "" {
		return ContentResponse{}, ErrAIUnavailable
	}

	tone := req.TonePreference
	if tone == "" {
		tone = "professional"
	}

	systemPrompt := buildSystemPrompt()
	userPrompt := buildUserPrompt(req, tone)

	payload := openAIRequest{
		Model:     c.model,
		MaxTokens: claudeMaxTokens, // same cap
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ContentResponse{}, fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIAPIURL, bytes.NewReader(body))
	if err != nil {
		return ContentResponse{}, fmt.Errorf("openai: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return ContentResponse{}, fmt.Errorf("openai: http: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return ContentResponse{}, fmt.Errorf("openai: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		snippet := string(respBytes)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return ContentResponse{}, fmt.Errorf("openai: status %d: %s", resp.StatusCode, snippet)
	}

	var apiResp openAIResponse
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return ContentResponse{}, fmt.Errorf("openai: unmarshal response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return ContentResponse{}, fmt.Errorf("openai: no choices in response")
	}

	rawText := apiResp.Choices[0].Message.Content
	if rawText == "" {
		return ContentResponse{}, fmt.Errorf("openai: empty message content")
	}

	parsed, err := parseArticleJSON(rawText)
	if err != nil {
		return ContentResponse{}, fmt.Errorf("openai: parse article JSON: %w", err)
	}

	parsed.InputTokens = apiResp.Usage.PromptTokens
	parsed.OutputTokens = apiResp.Usage.CompletionTokens
	parsed.Provider = ProviderOpenAI
	return parsed, nil
}
