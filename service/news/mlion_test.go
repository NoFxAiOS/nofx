package news

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMlionFetcher_FetchNews(t *testing.T) {
	// Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Header
		if r.Header.Get("X-API-KEY") != "test-key" {
			t.Errorf("Expected X-API-KEY header")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Verify Query Param (New)
		if r.URL.Query().Get("is_hot") != "Y" {
			t.Errorf("Expected query param is_hot=Y, got %s", r.URL.Query().Get("is_hot"))
		}

		// Return Mock JSON
		response := `{
			"code": 200,
			"data": [
				{
					"news_id": 12345,
					"title": "Test News",
					"content": "Test Content",
					"url": "http://test.com",
					"createTime": "2025-12-15 12:00:00",
					"sentiment": 0,
					"symbol": "BTC"
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer ts.Close()

	// Init Fetcher
	fetcher := NewMlionFetcher("test-key")
	fetcher.baseURL = ts.URL // Override URL

	// Test Fetch
	articles, err := fetcher.FetchNews("crypto")
	if err != nil {
		t.Fatalf("FetchNews failed: %v", err)
	}

	if len(articles) != 1 {
		t.Fatalf("Expected 1 article, got %d", len(articles))
	}

	a := articles[0]
	if a.ID != 12345 {
		t.Errorf("Expected ID 12345, got %d", a.ID)
	}
	if a.Headline != "Test News" {
		t.Errorf("Expected headline 'Test News', got '%s'", a.Headline)
	}
	if a.Source != "Mlion" {
		t.Errorf("Expected source 'Mlion', got '%s'", a.Source)
	}
	
	// Verify Time Parsing
	loc, _ := time.LoadLocation("Asia/Shanghai")
	expectedTime, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-12-15 12:00:00", loc)
	if a.Datetime != expectedTime.Unix() {
		t.Errorf("Expected timestamp %d, got %d", expectedTime.Unix(), a.Datetime)
	}
}