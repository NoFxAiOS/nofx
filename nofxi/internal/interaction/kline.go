package interaction

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// KlineBar represents a single candlestick.
type KlineBar struct {
	Time   int64   `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

// fetchKlines gets kline data from Binance public API (no auth needed).
func fetchKlines(symbol, interval string, limit int) ([]KlineBar, error) {
	if limit <= 0 {
		limit = 200
	}
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/klines?symbol=%s&interval=%s&limit=%d",
		symbol, interval, limit)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch klines: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var raw [][]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse klines: %w", err)
	}

	bars := make([]KlineBar, 0, len(raw))
	for _, k := range raw {
		if len(k) < 6 {
			continue
		}
		bar := KlineBar{
			Time: int64(k[0].(float64)) / 1000, // ms → seconds for lightweight-charts
		}
		bar.Open, _ = strconv.ParseFloat(k[1].(string), 64)
		bar.High, _ = strconv.ParseFloat(k[2].(string), 64)
		bar.Low, _ = strconv.ParseFloat(k[3].(string), 64)
		bar.Close, _ = strconv.ParseFloat(k[4].(string), 64)
		bar.Volume, _ = strconv.ParseFloat(k[5].(string), 64)
		bars = append(bars, bar)
	}
	return bars, nil
}

// RegisterKlineRoutes adds kline API endpoints to the mux.
func RegisterKlineRoutes(mux *http.ServeMux) {
	// GET /api/klines?symbol=BTCUSDT&interval=1h&limit=200
	mux.HandleFunc("/api/klines", func(w http.ResponseWriter, r *http.Request) {
		symbol := r.URL.Query().Get("symbol")
		if symbol == "" {
			symbol = "BTCUSDT"
		}
		interval := r.URL.Query().Get("interval")
		if interval == "" {
			interval = "1h"
		}
		limitStr := r.URL.Query().Get("limit")
		limit := 200
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
		if limit > 1000 {
			limit = 1000
		}

		bars, err := fetchKlines(symbol, interval, limit)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(bars)
	})

	// GET /api/ticker?symbol=BTCUSDT — current price + 24h change
	mux.HandleFunc("/api/ticker", func(w http.ResponseWriter, r *http.Request) {
		symbol := r.URL.Query().Get("symbol")
		if symbol == "" {
			symbol = "BTCUSDT"
		}

		url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/ticker/24hr?symbol=%s", symbol)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(body)
	})
}
