// Package databento provides access to the Databento historical and live
// market data APIs. Initial focus: CME GLBX.MDP3 OHLCV for index futures (NQ, MNQ).
//
// Auth model: Databento uses HTTP Basic with the API key as the username
// and an empty password. Reference: https://databento.com/docs/api-reference-historical/basics/authentication
package databento

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	DefaultHistoricalBaseURL = "https://hist.databento.com/v0"
	DefaultTimeout           = 30 * time.Second
	DefaultDataset           = "GLBX.MDP3"
)

type Client struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration

	mu   sync.RWMutex
	http *http.Client
}

var (
	defaultClient *Client
	clientOnce    sync.Once
)

func DefaultClient() *Client {
	clientOnce.Do(func() {
		defaultClient = NewClient("", "")
	})
	return defaultClient
}

func NewClient(baseURL, apiKey string) *Client {
	if baseURL == "" {
		baseURL = DefaultHistoricalBaseURL
	}
	if apiKey == "" {
		apiKey = os.Getenv("DATABENTO_API_KEY")
	}
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Timeout: DefaultTimeout,
		http:    &http.Client{Timeout: DefaultTimeout},
	}
}

func (c *Client) SetAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.APIKey = key
}

type APIError struct {
	StatusCode int
	Body       string
	Endpoint   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("databento %s: HTTP %d: %s", e.Endpoint, e.StatusCode, e.Body)
}

func (c *Client) doRequest(path string, params url.Values) ([]byte, error) {
	c.mu.RLock()
	baseURL := c.BaseURL
	apiKey := c.APIKey
	httpClient := c.http
	c.mu.RUnlock()

	if apiKey == "" {
		return nil, fmt.Errorf("databento: missing API key (set DATABENTO_API_KEY env var)")
	}

	endpoint := baseURL + path
	if params != nil && len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("databento: build request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(apiKey, ""))
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("databento: HTTP error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("databento: read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return body, &APIError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(body)),
			Endpoint:   path,
		}
	}

	return body, nil
}

func basicAuth(user, pass string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
}
