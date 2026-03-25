// Package nofxos provides data access to the NofxOS API (https://nofxos.ai)
// for quantitative trading data including AI500 scores, OI rankings,
// fund flow (NetFlow), price rankings, and coin details.
package nofxos

import (
	"io/ioutil"
	"net/http"
	"nofx/security"
	"strings"
	"sync"
	"time"
)

// Default configuration
const (
	DefaultBaseURL = "https://nofxos.ai"
	DefaultTimeout = 30 * time.Second
	DefaultAuthKey = "cm_568c67eae410d912c54c"
)

// Client is the NofxOS API client
type Client struct {
	BaseURL string
	AuthKey string
	Timeout time.Duration
	mu      sync.RWMutex
	claw402 *Claw402DataClient // If set, routes requests through claw402
}

var (
	defaultClient *Client
	clientOnce    sync.Once
)

// DefaultClient returns the singleton default client
func DefaultClient() *Client {
	clientOnce.Do(func() {
		defaultClient = &Client{
			BaseURL: DefaultBaseURL,
			AuthKey: DefaultAuthKey,
			Timeout: DefaultTimeout,
		}
	})
	return defaultClient
}

// NewClient creates a new NofxOS API client
func NewClient(baseURL, authKey string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if authKey == "" {
		authKey = DefaultAuthKey
	}
	return &Client{
		BaseURL: baseURL,
		AuthKey: authKey,
		Timeout: DefaultTimeout,
	}
}

// SetClaw402 enables routing requests through claw402 payment gateway.
func (c *Client) SetClaw402(claw402Client *Claw402DataClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.claw402 = claw402Client
}

// SetConfig updates client configuration
func (c *Client) SetConfig(baseURL, authKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if baseURL != "" {
		c.BaseURL = baseURL
	}
	if authKey != "" {
		c.AuthKey = authKey
	}
}

// GetBaseURL returns the current base URL
func (c *Client) GetBaseURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.BaseURL
}

// GetAuthKey returns the current auth key
func (c *Client) GetAuthKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.AuthKey
}

// doRequest performs an HTTP GET request with authentication.
// If claw402 client is configured, routes through claw402 payment gateway instead.
func (c *Client) doRequest(endpoint string) ([]byte, error) {
	c.mu.RLock()
	claw402Client := c.claw402
	baseURL := c.BaseURL
	authKey := c.AuthKey
	timeout := c.Timeout
	c.mu.RUnlock()

	// Route through claw402 if configured
	if claw402Client != nil {
		return claw402Client.DoRequest(endpoint)
	}

	url := baseURL + endpoint
	if !strings.Contains(url, "auth=") {
		if strings.Contains(url, "?") {
			url += "&auth=" + authKey
		} else {
			url += "?auth=" + authKey
		}
	}

	resp, err := security.SafeGet(url, timeout)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return body, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	return body, nil
}

// APIError represents an API error response
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return e.Message
}

// ExtractAuthKey extracts auth key from a URL string
func ExtractAuthKey(url string) string {
	if idx := strings.Index(url, "auth="); idx != -1 {
		authKey := url[idx+5:]
		if ampIdx := strings.Index(authKey, "&"); ampIdx != -1 {
			authKey = authKey[:ampIdx]
		}
		return authKey
	}
	return ""
}
