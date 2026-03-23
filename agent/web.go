package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"time"
)

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
		req.UserID = 1
	}
	msg := req.Message
	if req.Lang != "" {
		msg = "[lang:" + req.Lang + "] " + msg
	}

	ctx, cancel := context.WithTimeout(r.Context(), 55*time.Second)
	defer cancel()

	resp, err := w.agent.HandleMessage(ctx, req.UserID, msg)
	if err != nil {
		w.logger.Error("agent HandleMessage failed", "error", err, "user_id", req.UserID)
		writeJSON(rw, 500, map[string]string{"error": "Failed to process message. Please try again."})
		return
	}
	writeJSON(rw, 200, map[string]string{"response": resp})
}

// HandleKlines proxies kline data from Binance.
func (w *WebHandler) HandleKlines(rw http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" { symbol = "BTCUSDT" }
	interval := r.URL.Query().Get("interval")
	if interval == "" { interval = "1h" }

	if !validSymbolRe.MatchString(symbol) {
		writeJSON(rw, 400, map[string]string{"error": "invalid symbol"})
		return
	}
	if !validIntervalRe.MatchString(interval) {
		writeJSON(rw, 400, map[string]string{"error": "invalid interval"})
		return
	}

	proxyBinance(rw, fmt.Sprintf("https://fapi.binance.com/fapi/v1/klines?symbol=%s&interval=%s&limit=300", symbol, interval))
}

// HandleTicker proxies ticker data from Binance.
func (w *WebHandler) HandleTicker(rw http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" { symbol = "BTCUSDT" }

	if !validSymbolRe.MatchString(symbol) {
		writeJSON(rw, 400, map[string]string{"error": "invalid symbol"})
		return
	}

	proxyBinance(rw, fmt.Sprintf("https://fapi.binance.com/fapi/v1/ticker/24hr?symbol=%s", symbol))
}

func proxyBinance(rw http.ResponseWriter, url string) {
	resp, err := binanceClient.Get(url)
	if err != nil {
		writeJSON(rw, 502, map[string]string{"error": "upstream request failed"})
		return
	}
	defer resp.Body.Close()
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	// Limit response body to 2MB to prevent memory exhaustion
	io.Copy(rw, io.LimitReader(resp.Body, 2*1024*1024))
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
