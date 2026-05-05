// Package ai provides AI content generation clients and routing for SBF.
// Supports Anthropic Claude (primary) and OpenAI (fallback).
package ai

import (
	"context"
	"errors"
)

// Sentinel errors returned by AI clients and router.
var (
	// ErrAIUnavailable is returned when an API key is not configured.
	// Callers must handle this gracefully — do not panic.
	ErrAIUnavailable = errors.New("ai_unavailable")

	// ErrAIQuotaExceeded is returned when estimated cost exceeds the per-article cap.
	ErrAIQuotaExceeded = errors.New("ai_quota_exceeded")

	// ErrQualityRejected is returned when generated content fails quality guards
	// after the maximum number of retry attempts.
	ErrQualityRejected = errors.New("ai_quality_rejected")
)

// Provider identifies which AI backend generated a response.
type Provider string

const (
	ProviderClaude Provider = "claude"
	ProviderOpenAI Provider = "openai"
)

// ContentRequest holds the inputs for a single article generation call.
type ContentRequest struct {
	// Topic is the article topic (derived from campaign niche keywords).
	Topic string
	// AnchorText is the exact anchor text to embed naturally in the article.
	AnchorText string
	// AnchorType is one of: branded, naked, generic, exact.
	AnchorType string
	// MoneyURL is the destination URL the anchor must link to.
	MoneyURL string
	// TonePreference is optional; defaults to "professional" when empty.
	TonePreference string
}

// ContentResponse is the structured output returned by a successful generation.
// Body and Title are always non-empty on success.
type ContentResponse struct {
	// Title is the article title (no HTML, plain text).
	Title string
	// Slug is a URL-friendly version of the title.
	Slug string
	// MetaDescription is the SEO meta description (plain text, ≤160 chars).
	MetaDescription string
	// BodyHTML is the full HTML body of the article.
	BodyHTML string
	// AnchorPosition hints where the anchor was embedded (intro|section_2|conclusion).
	AnchorPosition string

	// InputTokens and OutputTokens track usage for cost accounting.
	InputTokens  int
	OutputTokens int
	// Provider records which backend produced this response.
	Provider Provider
}

// Generator is the interface implemented by both Claude and OpenAI clients.
// The router also implements this interface via fallback chaining.
type Generator interface {
	Generate(ctx context.Context, req ContentRequest) (ContentResponse, error)
}
