package news

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewsService_Integration(t *testing.T) {
	// 1. Setup Mock Finnhub Server
	now := time.Now().Unix()
	articles := []Article{
		{ID: 1, Headline: "Fed raises rates by 25bps", Summary: "Inflation concerns remain.", Source: "Reuters", Datetime: now - 100},
		{ID: 2, Headline: "PBOC cuts LPR to boost economy", Summary: "China stimulus continues.", Source: "Xinhua", Datetime: now - 50},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL params if needed
		assert.Equal(t, "general", r.URL.Query().Get("category"))
		json.NewEncoder(w).Encode(articles)
	}))
	defer server.Close()

	// 2. Setup Service
	fetcher := NewFinnhubFetcher("test_key")
	fetcher.SetBaseURL(server.URL)

	notifier := &MockNotifier{}
	store := &MockStateStore{
		Configs: map[string]string{},
	}

	svc := &Service{
		store:          store,
		fetchers:       []Fetcher{fetcher},
		topicRouter:    map[string]int{"Finnhub": 100}, // Topic 100
		notifier:       notifier,
		enabled:        true,
		sentArticleIDs: make(map[string]bool),
	}

	// 3. Execute
	err := svc.ProcessFetcher(fetcher, "general")
	assert.NoError(t, err)

	// 4. Verify
	assert.Equal(t, 2, len(notifier.SentMessages), "Should send exactly 2 messages")
	assert.Equal(t, 100, notifier.LastThreadID, "Should route to topic 100")
}

func TestMlion_Integration(t *testing.T) {
	// 1. Mock Mlion Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "mlion-key", r.Header.Get("X-API-KEY"))
		resp := MlionResponse{
			Code: 200,
			Data: []MlionNewsItem{
				{NewsID: 999, Title: "Mlion News", Content: "Content", CreateTime: "2025-01-01 12:00:00"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	fetcher := NewMlionFetcher("mlion-key")
	fetcher.baseURL = server.URL

	notifier := &MockNotifier{}
	store := &MockStateStore{Configs: map[string]string{}}

	svc := &Service{
		store:          store,
		fetchers:       []Fetcher{fetcher},
		topicRouter:    map[string]int{"Mlion": 17758},
		notifier:       notifier,
		enabled:        true,
		sentArticleIDs: make(map[string]bool),
	}

	err := svc.ProcessFetcher(fetcher, "crypto")
	assert.NoError(t, err)

	assert.Equal(t, 1, len(notifier.SentMessages))
	assert.Equal(t, 17758, notifier.LastThreadID, "Should route to topic 17758")
	assert.Contains(t, notifier.SentMessages[0], "Mlion News")
}
