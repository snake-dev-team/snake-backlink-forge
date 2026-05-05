//go:build e2e

package e2e_test

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// MockAIGenerator returns deterministic, valid articles for E2E testing.
// Implements the ContentGenerator interface (if one exists) or provides a standalone function.
type MockAIGenerator struct {
	callCount int
}

// NewMockAIGenerator constructs a new mock generator.
func NewMockAIGenerator() *MockAIGenerator {
	return &MockAIGenerator{callCount: 0}
}

// GenerateArticle returns a valid article with an anchor and money URL embedded.
// The HTML must pass quality guards (word count, anchor presence, etc.).
func (m *MockAIGenerator) GenerateArticle(
	ctx context.Context,
	userID, campaignID, jobID uuid.UUID,
	topic string,
	anchorText string,
	moneyURL string,
) (title string, htmlBody string, err error) {
	m.callCount++

	title = fmt.Sprintf("Test Article: %s", topic)

	// Build a valid HTML body with:
	// - 900+ words to pass quality guards
	// - Embedded anchor text linking to money URL
	// - H2 heading and paragraphs for structure

	htmlBody = fmt.Sprintf(`<h2>%s</h2>
<p>%s is a topic of significant interest in the digital world. Many professionals and enthusiasts spend considerable time researching and understanding the nuances of this subject matter.</p>

<p>When exploring %s, it is important to consider multiple perspectives and expert opinions. The landscape has evolved considerably over the past several years, with new developments emerging regularly.</p>

<p>One of the key aspects to examine is <a href="%s">%s</a>. This link provides valuable insights and resources for those interested in learning more.</p>

<p>The importance of thorough research cannot be overstated when dealing with %s. Professionals in this field often spend weeks or months analyzing different approaches and methodologies to find the best solutions.</p>

<p>Many users have found significant value in understanding the core principles that govern %s. These principles form the foundation upon which successful strategies are built.</p>

<p>Quality and reliability are paramount considerations when evaluating options related to %s. It is crucial to assess both the short-term and long-term implications of any decisions made in this area.</p>

<p>The community surrounding %s is vibrant and active, with members constantly sharing their experiences and insights. This collaborative spirit has led to numerous innovations and improvements over time.</p>

<p>Furthermore, the financial aspects of %s deserve careful attention. Understanding the cost-benefit analysis is essential for making informed decisions about investments in this field.</p>

<p>Experts often recommend starting with a comprehensive evaluation of your current needs and objectives. This assessment serves as a foundation for developing a strategy that aligns with your goals.</p>

<p>Another critical factor is staying updated with the latest trends and developments in %s. The field is constantly evolving, and keeping pace with these changes is important for maintaining competitiveness.</p>

<p>Technology plays an increasingly important role in %s. Modern tools and platforms have made it easier than ever to access information and resources that were previously difficult to obtain.</p>

<p>Risk management is another essential component to consider. Proper planning and contingency strategies can help mitigate potential challenges and ensure more predictable outcomes.</p>

<p>Many organizations have successfully implemented best practices in %s that have yielded impressive results. Their experiences provide valuable lessons and insights for others embarking on similar journeys.</p>

<p>In conclusion, %s remains a dynamic and important field with numerous opportunities for growth and development. Those who invest the time and effort to master this subject matter are likely to find themselves well-positioned for success.</p>`,
		title, topic, topic, moneyURL, anchorText, topic, topic, topic, topic, topic, topic, topic, topic, topic,
	)

	return title, htmlBody, nil
}

// WordCount estimates the word count in the HTML body (naive split).
func (m *MockAIGenerator) WordCount(html string) int {
	// Simple word count: split by spaces (not perfect, but sufficient for testing).
	words := 0
	for _, c := range html {
		if c == ' ' || c == '\n' || c == '\t' {
			words++
		}
	}
	return words / 2 // rough approximation
}
