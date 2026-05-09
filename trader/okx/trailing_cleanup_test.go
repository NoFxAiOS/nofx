package okx

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCancelTrailingStopOrdersCancelsAllMoveOrderStopsForInactiveSymbol(t *testing.T) {
	requests := make([]string, 0)
	canceled := make([]string, 0)
	tr := &OKXTrader{
		apiKey:     "test-key",
		secretKey:  "test-secret",
		passphrase: "test-passphrase",
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests = append(requests, req.Method+" "+req.URL.Path+"?"+req.URL.RawQuery)
			body := `{"code":"0","msg":"","data":[]}`
			if req.Method == http.MethodGet && strings.Contains(req.URL.RawQuery, "ordType=move_order_stop") {
				body = `{"code":"0","msg":"","data":[{"algoId":"trail-short","instId":"BTC-USDT-SWAP"},{"algoId":"trail-long","instId":"BTC-USDT-SWAP"}]}`
			}
			if req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/cancel-advance-algos") {
				payload, _ := io.ReadAll(req.Body)
				var items []map[string]string
				if err := json.Unmarshal(payload, &items); err != nil {
					t.Fatalf("parse cancel payload: %v", err)
				}
				for _, item := range items {
					canceled = append(canceled, item["algoId"])
				}
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
		})},
		instrumentsCache: make(map[string]*OKXInstrument),
	}

	if err := tr.CancelTrailingStopOrders("BTCUSDT"); err != nil {
		t.Fatalf("cancel trailing stop orders: %v", err)
	}
	if len(canceled) != 2 {
		t.Fatalf("expected both move_order_stop algos canceled for inactive symbol, got %v (requests=%v)", canceled, requests)
	}
	if canceled[0] != "trail-short" || canceled[1] != "trail-long" {
		t.Fatalf("unexpected canceled algo IDs: %v", canceled)
	}
}

func TestCancelTrailingStopOrdersByIDsCancelsOnlyTargetedMoveOrderStops(t *testing.T) {
	canceled := make([]string, 0)
	tr := &OKXTrader{
		apiKey:     "test-key",
		secretKey:  "test-secret",
		passphrase: "test-passphrase",
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"code":"0","msg":"","data":[]}`
			if req.Method == http.MethodGet && strings.Contains(req.URL.RawQuery, "ordType=move_order_stop") {
				body = `{"code":"0","msg":"","data":[{"algoId":"keep-this","instId":"BTC-USDT-SWAP"},{"algoId":"cancel-this","instId":"BTC-USDT-SWAP"}]}`
			}
			if req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/cancel-advance-algos") {
				payload, _ := io.ReadAll(req.Body)
				var items []map[string]string
				if err := json.Unmarshal(payload, &items); err != nil {
					t.Fatalf("parse cancel payload: %v", err)
				}
				for _, item := range items {
					canceled = append(canceled, item["algoId"])
				}
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
		})},
		instrumentsCache: make(map[string]*OKXInstrument),
	}

	if err := tr.CancelTrailingStopOrdersByIDs("BTCUSDT", []string{"cancel-this"}); err != nil {
		t.Fatalf("cancel targeted trailing stop order: %v", err)
	}
	if len(canceled) != 1 || canceled[0] != "cancel-this" {
		t.Fatalf("expected only targeted algo canceled, got %v", canceled)
	}
}
