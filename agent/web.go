package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"nofx/safe"
	"regexp"
	"time"
)

type storeUserIDContextKey struct{}

// WithStoreUserID annotates an HTTP request context with the authenticated store user ID.
func WithStoreUserID(ctx context.Context, storeUserID string) context.Context {
	return context.WithValue(ctx, storeUserIDContextKey{}, storeUserID)
}

func storeUserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(storeUserIDContextKey{}).(string); ok && v != "" {
		return v
	}
	return "default"
}

// validSymbolRe matches only alphanumeric trading symbols (e.g. BTCUSDT, ETH-USD).
var validSymbolRe = regexp.MustCompile(`^[A-Za-z0-9\-_]{1,20}$`)

// validIntervalRe matches only valid kline intervals (e.g. 1m, 5m, 1h, 4h, 1d, 1w).
var validIntervalRe = regexp.MustCompile(`^[0-9]{1,2}[mhHdDwWM]$`)

// binanceClient is a shared HTTP client for proxying Binance API requests.
// Reused across requests to benefit from connection pooling.
var binanceClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
}

// WebHandler provides HTTP endpoints for the NOFXi agent.
type WebHandler struct {
	agent  *Agent
	logger *slog.Logger
}

func NewWebHandler(agent *Agent, logger *slog.Logger) *WebHandler {
	return &WebHandler{agent: agent, logger: logger}
}

// HandleHealth handles GET /api/agent/health.
func (w *WebHandler) HandleHealth(rw http.ResponseWriter, r *http.Request) {
	writeJSON(rw, 200, map[string]string{"status": "ok", "agent": "NOFXi", "time": time.Now().Format(time.RFC3339)})
}

// HandleChat handles POST /api/agent/chat.
func (w *WebHandler) HandleChat(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", 405)
		return
	}
	var req struct {
		Message string `json:"message"`
		UserID  int64  `json:"user_id"`
		UserKey string `json:"user_key"`
		Lang    string `json:"lang"`
	}
	// Limit request body to 64KB to prevent abuse
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&req); err != nil {
		writeJSON(rw, 400, map[string]string{"error": "invalid request"})
		return
	}
	if req.Message == "" {
		writeJSON(rw, 400, map[string]string{"error": "message required"})
		return
	}
	if req.UserID == 0 {
		req.UserID = SessionUserIDFromKey(req.UserKey)
	}
	msg := req.Message
	if req.Lang != "" {
		msg = "[lang:" + req.Lang + "] " + msg
	}

	ctx, cancel := context.WithTimeout(r.Context(), 55*time.Second)
	defer cancel()

	resp, err := w.agent.HandleMessageForStoreUser(ctx, storeUserIDFromContext(r.Context()), req.UserID, msg)
	if err != nil {
		w.logger.Error("agent HandleMessage failed", "error", err, "user_id", req.UserID)
		writeJSON(rw, 500, map[string]string{"error": "Failed to process message. Please try again."})
		return
	}
	writeJSON(rw, 200, map[string]string{"response": resp})
}

// HandleChatStream handles POST /api/agent/chat/stream — SSE streaming chat.
// Sends server-sent events with types including planning, plan, step_start,
// step_complete, replan, tool, delta, done, error.
func (w *WebHandler) HandleChatStream(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", 405)
		return
	}
	var req struct {
		Message string `json:"message"`
		UserID  int64  `json:"user_id"`
		UserKey string `json:"user_key"`
		Lang    string `json:"lang"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&req); err != nil {
		writeJSON(rw, 400, map[string]string{"error": "invalid request"})
		return
	}
	if req.Message == "" {
		writeJSON(rw, 400, map[string]string{"error": "message required"})
		return
	}
	if req.UserID == 0 {
		req.UserID = SessionUserIDFromKey(req.UserKey)
	}
	msg := req.Message
	if req.Lang != "" {
		msg = "[lang:" + req.Lang + "] " + msg
	}

	// Set SSE headers
	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")
	rw.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
	rw.WriteHeader(200)

	flusher, ok := rw.(http.Flusher)
	if !ok {
		writeSSE(rw, nil, "error", "streaming not supported")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	resp, err := w.agent.HandleMessageStreamForStoreUser(ctx, storeUserIDFromContext(r.Context()), req.UserID, msg, func(event, data string) {
		writeSSE(rw, flusher, event, data)
	})
	if err != nil {
		w.logger.Error("agent HandleMessageStream failed", "error", err, "user_id", req.UserID)
		writeSSE(rw, flusher, "error", "Failed to process message. Please try again.")
		return
	}
	// Send final done event with complete response
	writeSSE(rw, flusher, "done", resp)
}

// writeSSE writes a single SSE event.
func writeSSE(w http.ResponseWriter, flusher http.Flusher, event, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, sseEscape(data))
	if flusher != nil {
		flusher.Flush()
	}
}

// sseEscape escapes newlines in SSE data (each line needs a "data: " prefix).
func sseEscape(s string) string {
	// SSE spec: multi-line data uses multiple "data:" lines
	// But we use JSON encoding to avoid this complexity
	b, _ := json.Marshal(s)
	return string(b)
}

// HandleKlines proxies kline data from Binance.
func (w *WebHandler) HandleKlines(rw http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "1h"
	}

	if !validSymbolRe.MatchString(symbol) {
		writeJSON(rw, 400, map[string]string{"error": "invalid symbol"})
		return
	}
	if !validIntervalRe.MatchString(interval) {
		writeJSON(rw, 400, map[string]string{"error": "invalid interval"})
		return
	}

	proxyBinance(rw, r.Context(), fmt.Sprintf("https://fapi.binance.com/fapi/v1/klines?symbol=%s&interval=%s&limit=300", symbol, interval))
}

// HandleTicker proxies ticker data from Binance.
func (w *WebHandler) HandleTicker(rw http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTCUSDT"
	}

	if !validSymbolRe.MatchString(symbol) {
		writeJSON(rw, 400, map[string]string{"error": "invalid symbol"})
		return
	}

	proxyBinance(rw, r.Context(), fmt.Sprintf("https://fapi.binance.com/fapi/v1/ticker/24hr?symbol=%s", symbol))
}

// HandleTickers handles GET /api/agent/tickers?symbols=BTCUSDT,ETHUSDT,SOLUSDT
// Batch endpoint: fetches multiple tickers concurrently, returns array.
func (w *WebHandler) HandleTickers(rw http.ResponseWriter, r *http.Request) {
	symbolsParam := r.URL.Query().Get("symbols")
	if symbolsParam == "" {
		symbolsParam = "BTCUSDT,ETHUSDT,SOLUSDT"
	}

	// Validate symbols
	var symbols []string
	for _, s := range splitComma(symbolsParam) {
		if validSymbolRe.MatchString(s) {
			symbols = append(symbols, s)
		}
	}
	if len(symbols) == 0 {
		writeJSON(rw, 400, map[string]string{"error": "no valid symbols"})
		return
	}
	if len(symbols) > 20 {
		writeJSON(rw, 400, map[string]string{"error": "max 20 symbols"})
		return
	}

	// Fetch all tickers concurrently with context propagation
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	type result struct {
		idx  int
		data json.RawMessage
	}
	results := make(chan result, len(symbols))
	for i, sym := range symbols {
		idx, s := i, sym
		safe.GoNamed("ticker-fetch-"+s, func() {
			req, err := http.NewRequestWithContext(ctx, "GET",
				fmt.Sprintf("https://fapi.binance.com/fapi/v1/ticker/24hr?symbol=%s", s), nil)
			if err != nil {
				results <- result{idx: idx}
				return
			}
			resp, err := binanceClient.Do(req)
			if err != nil {
				results <- result{idx: idx}
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				results <- result{idx: idx}
				return
			}
			body, err := safe.ReadAllLimited(resp.Body, 16*1024)
			if err != nil {
				results <- result{idx: idx}
				return
			}
			results <- result{idx: idx, data: body}
		})
	}

	// Collect results in order
	ordered := make([]json.RawMessage, len(symbols))
	for range symbols {
		r := <-results
		if r.data != nil {
			ordered[r.idx] = r.data
		}
	}

	// Filter out nil entries and write response
	out := make([]json.RawMessage, 0, len(ordered))
	for _, d := range ordered {
		if d != nil {
			out = append(out, d)
		}
	}
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(out)
}

// commaRe is pre-compiled for splitComma — avoids recompiling on every call.
var commaRe = regexp.MustCompile(`\s*,\s*`)

// splitComma splits a comma-separated string, trims whitespace, skips empty.
func splitComma(s string) []string {
	var parts []string
	for _, p := range commaRe.Split(s, -1) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func proxyBinance(rw http.ResponseWriter, ctx context.Context, url string) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		writeJSON(rw, 500, map[string]string{"error": "failed to create request"})
		return
	}
	resp, err := binanceClient.Do(req)
	if err != nil {
		// Distinguish client cancellation from upstream failures
		if ctx.Err() != nil {
			return // Client disconnected, no point writing response
		}
		writeJSON(rw, 502, map[string]string{"error": "upstream request failed"})
		return
	}
	defer resp.Body.Close()

	// Forward upstream error status codes instead of silently proxying bad data
	if resp.StatusCode != http.StatusOK {
		writeJSON(rw, 502, map[string]string{"error": fmt.Sprintf("upstream returned status %d", resp.StatusCode)})
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	// CORS is handled by the gin middleware — no need to set it here
	// Limit response body to 2MB to prevent memory exhaustion
	io.Copy(rw, io.LimitReader(resp.Body, 2*1024*1024))
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	// CORS is handled by the gin middleware — no need to set it here
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
