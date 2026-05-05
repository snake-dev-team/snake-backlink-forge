package ai

import (
	"context"
	"errors"
	"fmt"
)

// Router tries the primary Generator first; on error or ErrAIUnavailable it
// falls back to the secondary Generator. If both fail, the combined error is
// returned. Implements Generator itself so callers are routing-unaware.
type Router struct {
	primary   Generator
	secondary Generator
}

// NewRouter constructs a Router. Both primary and secondary must be non-nil.
// Typical wiring: NewRouter(NewClaudeClient(...), NewOpenAIClient(...))
func NewRouter(primary, secondary Generator) *Router {
	return &Router{primary: primary, secondary: secondary}
}

// Generate calls primary first. If primary returns any error (including
// ErrAIUnavailable for unconfigured key), it falls back to secondary.
// Returns ErrAIUnavailable only when both backends are unconfigured.
func (r *Router) Generate(ctx context.Context, req ContentRequest) (ContentResponse, error) {
	resp, primaryErr := r.primary.Generate(ctx, req)
	if primaryErr == nil {
		return resp, nil
	}

	// If context is already cancelled/timed-out, don't attempt fallback.
	if ctx.Err() != nil {
		return ContentResponse{}, primaryErr
	}

	resp, secondaryErr := r.secondary.Generate(ctx, req)
	if secondaryErr == nil {
		return resp, nil
	}

	// Both failed. Surface combined error. If both are unavailable, promote
	// ErrAIUnavailable so callers can detect the unconfigured state cleanly.
	if errors.Is(primaryErr, ErrAIUnavailable) && errors.Is(secondaryErr, ErrAIUnavailable) {
		return ContentResponse{}, ErrAIUnavailable
	}

	return ContentResponse{}, fmt.Errorf("ai router: primary: %w; fallback: %v", primaryErr, secondaryErr)
}
