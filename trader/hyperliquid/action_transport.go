package hyperliquid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// v0.36 executeAction decodes a 200 response into UserState without inspecting
// status. Hyperliquid business rejections are HTTP 200 with status="err".
// Preserve the SDK's signer/nonces while rejecting these envelopes explicitly.
type checkedActionTransport struct{ base http.RoundTripper }

func (t checkedActionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || req.URL.Path != "/exchange" || resp.StatusCode != http.StatusOK {
		return resp, err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if len(body) > 1<<20 {
		return nil, fmt.Errorf("Hyperliquid action response exceeds size limit")
	}
	var envelope struct {
		Status   string          `json:"status"`
		Response json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("invalid Hyperliquid action response: %w", err)
	}
	if envelope.Status != "ok" {
		return nil, fmt.Errorf("Hyperliquid action rejected (status %q): %s", envelope.Status, envelope.Response)
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return resp, nil
}

func checkedActionHTTPClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second, Transport: checkedActionTransport{base: http.DefaultTransport}}
}
