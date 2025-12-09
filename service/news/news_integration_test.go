package news

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewsService_Integration(t *testing.T) {
    // 1. Setup Mock Finnhub Server
    // 注意：为了测试排序，我们给时间稍微错开一点
    now := time.Now().Unix()
    articles := []Article{
        {ID: 1, Headline: "Fed raises rates by 25bps", Summary: "Inflation concerns remain.", Source: "Reuters", Datetime: now - 100},
        {ID: 2, Headline: "PBOC cuts LPR to boost economy", Summary: "China stimulus continues.", Source: "Xinhua", Datetime: now - 50},
        {ID: 3, Headline: "Random Blog Post about Fed", Summary: "Speculation.", Source: "MyBlog", Datetime: now}, // Invalid Source
        {ID: 4, Headline: "World Cup Final Score", Summary: "France vs Argentina.", Source: "Reuters", Datetime: now}, // Excluded Keyword
        {ID: 5, Headline: "Just a normal day", Summary: "Nothing happening.", Source: "Bloomberg", Datetime: now}, // No Policy Keyword
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
        fetcher:        fetcher,
        notifier:       notifier,
        enabled:        true,
        sentArticleIDs: make(map[int64]bool),
    }
    
    // 3. Execute
    err := svc.ProcessCategory("general")
    assert.NoError(t, err)
    
    // 4. Verify
    // Should keep ID 1 (Valid US) and ID 2 (Valid China). Others rejected.
    assert.Equal(t, 2, len(notifier.SentMessages), "Should send exactly 2 messages")
    
    combinedMsg := strings.Join(notifier.SentMessages, "|||")
    
    // Check Content and Tags
    // Note: Fetcher adds tags like 【美国】, 【中国】 to Headline.
    // Service formats message using Headline.
    
    assert.Contains(t, combinedMsg, "【美国】", "Should contain US tag")
    assert.Contains(t, combinedMsg, "【中国】", "Should contain China tag")
    assert.Contains(t, combinedMsg, "Fed raises rates", "Should contain Fed article")
    assert.Contains(t, combinedMsg, "PBOC cuts LPR", "Should contain PBOC article")
    
    // Check Filters
    assert.NotContains(t, combinedMsg, "Random Blog Post", "Should NOT contain invalid source")
    assert.NotContains(t, combinedMsg, "World Cup", "Should NOT contain excluded keyword")
    assert.NotContains(t, combinedMsg, "Just a normal day", "Should NOT contain irrelevant news")
}
