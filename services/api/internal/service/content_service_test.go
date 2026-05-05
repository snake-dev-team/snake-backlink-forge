package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kekuta/snake-backlink-forge/services/api/internal/ai"
)

// mockGenerator is a test double for ai.Generator.
type mockGenerator struct {
	resp ai.ContentResponse
	err  error
	// callCount tracks how many times Generate was called.
	callCount int
	// responses allows sequencing different responses per call.
	responses []ai.ContentResponse
	errs      []error
}

func (m *mockGenerator) Generate(_ context.Context, _ ai.ContentRequest) (ai.ContentResponse, error) {
	idx := m.callCount
	m.callCount++

	if len(m.responses) > 0 && idx < len(m.responses) {
		return m.responses[idx], m.errs[idx]
	}
	return m.resp, m.err
}

// buildValidResponse returns a ContentResponse that passes all quality guards.
// word count is ~830 words, contains anchor and money URL.
func buildValidResponse(anchorText, moneyURL string) ai.ContentResponse {
	// Build body with enough words (800+) containing anchor + money URL.
	var sb strings.Builder
	sb.WriteString("<h2>Giới thiệu</h2><p>")
	// Embed anchor link + money URL to satisfy guards.
	sb.WriteString("Bài viết này giới thiệu về dịch vụ tuyệt vời tại ")
	sb.WriteString(`<a href="` + moneyURL + `">` + anchorText + `</a>. `)
	// Pad to 800-1500 words with Vietnamese filler text.
	// filler ≈ 18 words; 50 iterations ≈ 900 words including h2/p/anchor scaffold.
	filler := "Đây là nội dung mẫu để kiểm tra chất lượng bài viết được tạo bởi hệ thống AI. "
	for i := 0; i < 50; i++ {
		sb.WriteString(filler)
	}
	sb.WriteString("</p><h2>Kết luận</h2><p>Hãy truy cập ngay hôm nay.</p>")
	return ai.ContentResponse{
		Title:           "Bài Viết Test",
		Slug:            "bai-viet-test",
		MetaDescription: "Mô tả meta cho bài viết test",
		BodyHTML:        sb.String(),
		AnchorPosition:  "intro",
		InputTokens:     1000,
		OutputTokens:    2000,
		Provider:        ai.ProviderClaude,
	}
}

// --- validateContent tests ---

func TestValidateContent_HappyPath(t *testing.T) {
	anchor := "dịch vụ SEO"
	moneyURL := "https://example.com"
	resp := buildValidResponse(anchor, moneyURL)
	if err := validateContent(resp.BodyHTML, anchor, moneyURL); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateContent_TooFewWords(t *testing.T) {
	body := `<p>Quá ngắn.</p>`
	err := validateContent(body, "anchor", "https://example.com")
	if err == nil {
		t.Fatal("expected error for too-few words")
	}
	if !strings.Contains(err.Error(), "word count") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateContent_AnchorMissing(t *testing.T) {
	anchor := "missing anchor"
	moneyURL := "https://example.com"
	resp := buildValidResponse("different anchor text here", moneyURL)
	err := validateContent(resp.BodyHTML, anchor, moneyURL)
	if err == nil {
		t.Fatal("expected error when anchor text missing")
	}
	if !strings.Contains(err.Error(), "anchor text") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateContent_MoneyURLMissing(t *testing.T) {
	anchor := "dịch vụ SEO"
	resp := buildValidResponse(anchor, "https://example.com")
	// Validate with a different money URL than what's embedded.
	err := validateContent(resp.BodyHTML, anchor, "https://other-site.com")
	if err == nil {
		t.Fatal("expected error when money URL missing")
	}
	if !strings.Contains(err.Error(), "money URL") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateContent_AIPhrase_Rejected(t *testing.T) {
	anchor := "test anchor"
	moneyURL := "https://example.com"
	resp := buildValidResponse(anchor, moneyURL)
	// Inject AI filler phrase into body.
	resp.BodyHTML = resp.BodyHTML + " As an AI language model, I should note this."
	err := validateContent(resp.BodyHTML, anchor, moneyURL)
	if err == nil {
		t.Fatal("expected error for AI filler phrase")
	}
	if !strings.Contains(err.Error(), "AI filler phrase") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// --- calcCostUSD tests ---

func TestCalcCostUSD_ZeroTokens(t *testing.T) {
	cost := calcCostUSD(0, 0)
	if cost != 0 {
		t.Errorf("expected 0 cost for zero tokens, got %f", cost)
	}
}

func TestCalcCostUSD_TypicalUsage(t *testing.T) {
	// 1000 input + 2000 output
	// = (1000/1M)*3 + (2000/1M)*15 = 0.003 + 0.030 = 0.033
	cost := calcCostUSD(1000, 2000)
	expected := 0.033
	if cost < expected-0.0001 || cost > expected+0.0001 {
		t.Errorf("expected ~%f, got %f", expected, cost)
	}
}

func TestCalcCostUSD_ExceedsCap(t *testing.T) {
	// Massive token usage should exceed the cap.
	// 20K input + 10K output = 0.06 + 0.15 = 0.21 > $0.15 cap
	cost := calcCostUSD(20_000, 10_000)
	if cost <= maxCostPerArticleUSD {
		t.Errorf("expected cost > %f, got %f", maxCostPerArticleUSD, cost)
	}
}

// --- generateWithQualityGuard tests ---

func TestGenerateWithQualityGuard_HappyPath(t *testing.T) {
	anchor := "dịch vụ SEO"
	moneyURL := "https://example.com"
	mock := &mockGenerator{resp: buildValidResponse(anchor, moneyURL)}
	svc := NewContentService(nil, nil, mock, nil)

	req := ai.ContentRequest{AnchorText: anchor, MoneyURL: moneyURL}
	resp, cost, err := svc.generateWithQualityGuard(context.Background(), req, anchor, moneyURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Title == "" {
		t.Error("expected non-empty title")
	}
	if cost <= 0 {
		t.Error("expected positive cost")
	}
	if mock.callCount != 1 {
		t.Errorf("expected 1 Generate call, got %d", mock.callCount)
	}
}

func TestGenerateWithQualityGuard_QualityFailThenPass(t *testing.T) {
	anchor := "dịch vụ SEO"
	moneyURL := "https://example.com"

	// First response fails quality (too short), second passes.
	badResp := ai.ContentResponse{
		Title:    "Too Short",
		BodyHTML: "<p>Short.</p>",
		Provider: ai.ProviderClaude,
	}
	goodResp := buildValidResponse(anchor, moneyURL)

	mock := &mockGenerator{
		responses: []ai.ContentResponse{badResp, goodResp},
		errs:      []error{nil, nil},
	}
	svc := NewContentService(nil, nil, mock, nil)

	req := ai.ContentRequest{AnchorText: anchor, MoneyURL: moneyURL}
	resp, _, err := svc.generateWithQualityGuard(context.Background(), req, anchor, moneyURL)
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if resp.Title != goodResp.Title {
		t.Errorf("expected good response title, got %q", resp.Title)
	}
	if mock.callCount != 2 {
		t.Errorf("expected 2 Generate calls (1 retry), got %d", mock.callCount)
	}
}

func TestGenerateWithQualityGuard_QualityFailBothAttempts(t *testing.T) {
	badResp := ai.ContentResponse{
		Title:    "Too Short",
		BodyHTML: "<p>Short.</p>",
		Provider: ai.ProviderClaude,
	}
	mock := &mockGenerator{
		responses: []ai.ContentResponse{badResp, badResp},
		errs:      []error{nil, nil},
	}
	svc := NewContentService(nil, nil, mock, nil)

	req := ai.ContentRequest{AnchorText: "anchor", MoneyURL: "https://example.com"}
	_, _, err := svc.generateWithQualityGuard(context.Background(), req, "anchor", "https://example.com")
	if err == nil {
		t.Fatal("expected error when both attempts fail quality guard")
	}
	if !errors.Is(err, ai.ErrQualityRejected) {
		t.Errorf("expected ErrQualityRejected, got: %v", err)
	}
	// Should have attempted maxQualityRetries+1 = 2 times.
	if mock.callCount != maxQualityRetries+1 {
		t.Errorf("expected %d Generate calls, got %d", maxQualityRetries+1, mock.callCount)
	}
}

func TestGenerateWithQualityGuard_CostCapExceeded(t *testing.T) {
	anchor := "dịch vụ SEO"
	moneyURL := "https://example.com"
	resp := buildValidResponse(anchor, moneyURL)
	// Set tokens that exceed $0.15 cap: 20K input + 10K output = $0.21.
	resp.InputTokens = 20_000
	resp.OutputTokens = 10_000

	mock := &mockGenerator{resp: resp}
	svc := NewContentService(nil, nil, mock, nil)

	req := ai.ContentRequest{AnchorText: anchor, MoneyURL: moneyURL}
	_, _, err := svc.generateWithQualityGuard(context.Background(), req, anchor, moneyURL)
	if err == nil {
		t.Fatal("expected ErrAIQuotaExceeded")
	}
	if !errors.Is(err, ai.ErrAIQuotaExceeded) {
		t.Errorf("expected ErrAIQuotaExceeded, got: %v", err)
	}
}

func TestGenerateWithQualityGuard_AIUnavailable(t *testing.T) {
	mock := &mockGenerator{err: ai.ErrAIUnavailable}
	svc := NewContentService(nil, nil, mock, nil)

	req := ai.ContentRequest{AnchorText: "anchor", MoneyURL: "https://example.com"}
	_, _, err := svc.generateWithQualityGuard(context.Background(), req, "anchor", "https://example.com")
	if err == nil {
		t.Fatal("expected error when AI unavailable")
	}
	if !errors.Is(err, ai.ErrAIUnavailable) {
		t.Errorf("expected ErrAIUnavailable, got: %v", err)
	}
}
