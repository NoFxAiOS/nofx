package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// WebHandler provides HTTP endpoints for the NOFXi agent.
// These are registered on the existing NOFX API server.
type WebHandler struct {
	agent  *Agent
	logger *slog.Logger
}

// NewWebHandler creates a new web handler.
func NewWebHandler(agent *Agent, logger *slog.Logger) *WebHandler {
	return &WebHandler{agent: agent, logger: logger}
}

// RegisterRoutes registers agent API routes on an existing mux or gin router.
// For now we use a standalone http server on a separate port.
func (w *WebHandler) StartStandalone(port int) {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("/api/agent/health", func(rw http.ResponseWriter, r *http.Request) {
		writeJSON(rw, 200, map[string]string{"status": "ok", "agent": "NOFXi", "time": time.Now().Format(time.RFC3339)})
	})

	// Chat endpoint
	mux.HandleFunc("/api/agent/chat", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(rw, "method not allowed", 405)
			return
		}
		var req struct {
			Message string `json:"message"`
			UserID  int64  `json:"user_id"`
			Lang    string `json:"lang"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
			writeJSON(rw, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(rw, 200, map[string]string{"response": resp})
	})

	// Kline data
	mux.HandleFunc("/api/agent/klines", handleKlines)

	// Ticker
	mux.HandleFunc("/api/agent/ticker", handleTicker)

	go func() {
		addr := fmt.Sprintf(":%d", port)
		w.logger.Info("NOFXi agent web API starting", "port", port)
		if err := http.ListenAndServe(addr, mux); err != nil {
			w.logger.Error("agent web server error", "error", err)
		}
	}()
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func handleKlines(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" { symbol = "BTCUSDT" }
	interval := r.URL.Query().Get("interval")
	if interval == "" { interval = "1h" }

	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/klines?symbol=%s&interval=%s&limit=300", symbol, interval)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(200)
	buf := make([]byte, 32768)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 { w.Write(buf[:n]) }
		if err != nil { break }
	}
}

func handleTicker(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" { symbol = "BTCUSDT" }

	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/ticker/24hr?symbol=%s", symbol)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(200)
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 { w.Write(buf[:n]) }
		if err != nil { break }
	}
}
