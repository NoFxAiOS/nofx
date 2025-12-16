package news

import (
	"net/http"
	"net/http/httptest"
	"net/url" // Import for parsing URL
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

		// Verify Query Params (All new and old)
		params := r.URL.Query()
		if params.Get("language") != "cn" {
			t.Errorf("Expected language=cn, got %s", params.Get("language"))
		}
		if params.Get("time_zone") != "Asia/Shanghai" {
			t.Errorf("Expected time_zone=Asia/Shanghai, got %s", params.Get("time_zone"))
		}
		if params.Get("num") != "100" {
			t.Errorf("Expected num=100, got %s", params.Get("num"))
		}
		if params.Get("page") != "1" {
			t.Errorf("Expected page=1, got %s", params.Get("page"))
		}
		if params.Get("client") != "mlion" {
			t.Errorf("Expected client=mlion, got %s", params.Get("client"))
		}
		if params.Get("is_hot") != "Y" {
			t.Errorf("Expected is_hot=Y, got %s", params.Get("is_hot"))
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
	// Manually ensure the base URL for the test matches the production expectation of having all query params.
	// We use url.Parse and Add to properly handle parameters when overriding base.
	u, _ := url.Parse(ts.URL)
	q := u.Query()
	q.Set("language", "cn")
	q.Set("time_zone", "Asia/Shanghai")
	q.Set("num", "100")
	q.Set("page", "1")
	q.Set("client", "mlion")
	q.Set("is_hot", "Y")
	u.RawQuery = q.Encode()
	fetcher.baseURL = u.String()

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

	// Verify Time Parsing
	loc, _ := time.LoadLocation("Asia/Shanghai")
	expectedTime, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-12-15 12:00:00", loc)
	if a.Datetime != expectedTime.Unix() {
		t.Errorf("Expected timestamp %d, got %d", expectedTime.Unix(), a.Datetime)
	}
}

func TestMlionFetcher_Constant(t *testing.T) {
	f := NewMlionFetcher("key")

	expectedParams := map[string]string{
		"language":  "cn",
		"time_zone": "Asia/Shanghai",
		"num":       "100",
		"page":      "1",
		"client":    "mlion",
		"is_hot":    "Y",
	}

	u, err := url.Parse(f.baseURL)
	if err != nil {
		t.Fatalf("Failed to parse baseURL: %v", err)
	}
	params := u.Query()

	for key, expectedValue := range expectedParams {
		if params.Get(key) != expectedValue {
			t.Errorf("baseURL missing or incorrect param: %s. Expected '%s', got '%s'", key, expectedValue, params.Get(key))
		}
	}
}
