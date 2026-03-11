package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"strings"
	"time"
)

// apiCallTool executes HTTP requests against the NOFX API server.
// This is the only tool available to the agent.
type apiCallTool struct {
	baseURL string
	token   string
	client  *http.Client
}

// apiRequest holds the arguments decoded from the LLM's api_request tool call.
type apiRequest struct {
	Method string         `json:"method"`
	Path   string         `json:"path"`
	Body   map[string]any `json:"body"`
}

func newAPICallTool(port int, token string) *apiCallTool {
	return &apiCallTool{
		baseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		token:   token,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// execute calls the API and returns the response as a string for LLM consumption.
func (t *apiCallTool) execute(req *apiRequest) string {
	if req.Method == "" || req.Path == "" {
		return "error: method and path are required"
	}
	if !strings.HasPrefix(req.Path, "/") {
		req.Path = "/" + req.Path
	}

	var bodyReader io.Reader
	if req.Method != "GET" && len(req.Body) > 0 {
		b, err := json.Marshal(req.Body)
		if err != nil {
			return fmt.Sprintf("error marshaling body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	httpReq, err := http.NewRequest(req.Method, t.baseURL+req.Path, bodyReader)
	if err != nil {
		return fmt.Sprintf("error creating request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+t.token)

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return fmt.Sprintf("API call failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("error reading response: %v", err)
	}

	logger.Infof("Agent api_call: %s %s -> %d", req.Method, req.Path, resp.StatusCode)

	if resp.StatusCode >= 400 {
		return fmt.Sprintf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Pretty-print JSON for better LLM readability
	var v any
	if json.Unmarshal(body, &v) == nil {
		if pretty, err := json.MarshalIndent(v, "", "  "); err == nil {
			return string(pretty)
		}
	}
	return string(body)
}

