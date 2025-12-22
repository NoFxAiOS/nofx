package decision

import (
	"testing"
)

func TestNewEmptyNewsContext(t *testing.T) {
	ctx := NewEmptyNewsContext()

	if ctx.Enabled {
		t.Error("Empty context should have Enabled=false")
	}
	if len(ctx.Articles) != 0 {
		t.Error("Empty context should have no articles")
	}
	if ctx.SentimentAvg != 0.0 {
		t.Error("Empty context should have SentimentAvg=0")
	}
}

func TestNewNewsContext(t *testing.T) {
	articles := []Article{
		{ID: 1, Headline: "BTC up", Sentiment: 1, Category: "price"},
		{ID: 2, Headline: "ETH down", Sentiment: -1, Category: "price"},
		{ID: 3, Headline: "New regulation", Sentiment: 0, Category: "regulation"},
	}

	ctx := NewNewsContext(articles)

	if !ctx.Enabled {
		t.Error("News context should be enabled")
	}
	if len(ctx.Articles) != 3 {
		t.Errorf("Expected 3 articles, got %d", len(ctx.Articles))
	}
	if ctx.SentimentAvg != 0.0 {
		t.Errorf("Expected sentiment 0.0, got %f", ctx.SentimentAvg)
	}
	if ctx.FetchedAt == 0 {
		t.Error("FetchedAt should be set")
	}
}

func TestCalculateSentiment(t *testing.T) {
	tests := []struct {
		name     string
		articles []Article
		expected float64
	}{
		{
			name:     "all positive",
			articles: []Article{{Sentiment: 1}, {Sentiment: 1}},
			expected: 1.0,
		},
		{
			name:     "all negative",
			articles: []Article{{Sentiment: -1}, {Sentiment: -1}},
			expected: -1.0,
		},
		{
			name:     "mixed",
			articles: []Article{{Sentiment: 1}, {Sentiment: -1}, {Sentiment: 0}},
			expected: 0.0,
		},
		{
			name:     "empty",
			articles: []Article{},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateSentiment(tt.articles)
			if result != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestHasArticles(t *testing.T) {
	tests := []struct {
		name     string
		ctx      *NewsContext
		expected bool
	}{
		{
			name:     "nil context",
			ctx:      nil,
			expected: false,
		},
		{
			name:     "empty articles",
			ctx:      NewEmptyNewsContext(),
			expected: false,
		},
		{
			name: "with articles",
			ctx: &NewsContext{
				Articles: []Article{{ID: 1}},
				Enabled:  true,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ctx.HasArticles()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetTopArticles(t *testing.T) {
	articles := []Article{
		{ID: 1, Headline: "Article 1"},
		{ID: 2, Headline: "Article 2"},
		{ID: 3, Headline: "Article 3"},
	}

	ctx := NewNewsContext(articles)

	tests := []struct {
		n        int
		expected int
	}{
		{n: 1, expected: 1},
		{n: 2, expected: 2},
		{n: 3, expected: 3},
		{n: 5, expected: 3}, // More than available
		{n: -1, expected: 3}, // Negative = all
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := ctx.GetTopArticles(tt.n)
			if len(result) != tt.expected {
				t.Errorf("Expected %d articles, got %d", tt.expected, len(result))
			}
		})
	}
}

func TestSentimentLabel(t *testing.T) {
	tests := []struct {
		sentiment float64
		expected  string
	}{
		{0.5, "positive"},
		{0.3, "positive"},
		{0.2, "positive"},   // 修复：0.2应该是positive (>= 0.2)
		{0.19, "neutral"},
		{0.1, "neutral"},
		{-0.1, "neutral"},
		{-0.19, "neutral"},
		{-0.2, "negative"},  // 修复：-0.2应该是negative (<= -0.2)
		{-0.3, "negative"},
		{-0.5, "negative"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			ctx := &NewsContext{SentimentAvg: tt.sentiment}
			result := ctx.SentimentLabel()
			if result != tt.expected {
				t.Errorf("Sentiment %f: expected %s, got %s", tt.sentiment, tt.expected, result)
			}
		})
	}
}
