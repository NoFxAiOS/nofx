package interaction

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// WebServer provides a REST API for NOFXi.
type WebServer struct {
	handler  MessageHandler
	port     int
	logger   *slog.Logger
	server   *http.Server
}

// NewWebServer creates a new web API server.
func NewWebServer(port int, handler MessageHandler, logger *slog.Logger) *WebServer {
	return &WebServer{
		handler: handler,
		port:    port,
		logger:  logger,
	}
}

// chatRequest is the API request body.
type chatRequest struct {
	UserID  int64  `json:"user_id"`
	Message string `json:"message"`
}

// chatResponse is the API response body.
type chatAPIResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

// Start begins listening. Blocks until context is cancelled.
func (w *WebServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(rw http.ResponseWriter, r *http.Request) {
		json.NewEncoder(rw).Encode(map[string]string{
			"status": "ok",
			"agent":  "NOFXi",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// Chat endpoint
	mux.HandleFunc("/api/chat", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(rw, http.StatusBadRequest, chatAPIResponse{Error: "invalid request body"})
			return
		}

		if req.Message == "" {
			writeJSON(rw, http.StatusBadRequest, chatAPIResponse{Error: "message is required"})
			return
		}
		if req.UserID == 0 {
			req.UserID = 1 // Default user for API access
		}

		resp, err := w.handler(r.Context(), req.UserID, req.Message)
		if err != nil {
			writeJSON(rw, http.StatusInternalServerError, chatAPIResponse{Error: err.Error()})
			return
		}

		writeJSON(rw, http.StatusOK, chatAPIResponse{Response: resp})
	})

	// OpenAI-compatible endpoint (so other tools can talk to NOFXi)
	mux.HandleFunc("/v1/chat/completions", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid request"})
			return
		}

		// Get the last user message
		userMsg := ""
		for i := len(body.Messages) - 1; i >= 0; i-- {
			if body.Messages[i].Role == "user" {
				userMsg = body.Messages[i].Content
				break
			}
		}
		if userMsg == "" {
			writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "no user message"})
			return
		}

		resp, err := w.handler(r.Context(), 1, userMsg)
		if err != nil {
			writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Return OpenAI-compatible response
		writeJSON(rw, http.StatusOK, map[string]interface{}{
			"id":      fmt.Sprintf("nofxi-%d", time.Now().UnixNano()),
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "nofxi",
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"message":       map[string]string{"role": "assistant", "content": resp},
					"finish_reason": "stop",
				},
			},
		})
	})

	addr := fmt.Sprintf(":%d", w.port)
	w.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	w.logger.Info("web server starting", "addr", addr)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		w.server.Shutdown(shutdownCtx)
	}()

	if err := w.server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Port returns the configured port.
func (w *WebServer) Port() int {
	return w.port
}

// PortStr returns port as string.
func (w *WebServer) PortStr() string {
	return strconv.Itoa(w.port)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
