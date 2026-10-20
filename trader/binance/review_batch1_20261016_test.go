package binance

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/adshao/go-binance/v2/futures"
)

// review 2026-10-16 E: CloseLong/CloseShort(symbol, 0) is a FULL close; the
// side-scoped protection cancel used to be dead code because quantity was
// overwritten with the live size before the `quantity == 0` check. A full
// close must cancel exactly this side's SL/TP legs; a partial close must not.
func TestBinanceFullCloseCancelsOwnSideProtection(t *testing.T) {
	for _, tc := range []struct {
		name, side, positionAmt string
		call                    func(*FuturesTrader, float64) error
	}{
		{"long", "LONG", "2", func(ft *FuturesTrader, q float64) error { _, err := ft.CloseLong("XUSDT", q); return err }},
		{"short", "SHORT", "-2", func(ft *FuturesTrader, q float64) error { _, err := ft.CloseShort("XUSDT", q); return err }},
	} {
		for _, full := range []bool{true, false} {
			name := tc.name + "_partial"
			if full {
				name = tc.name + "_full"
			}
			t.Run(name, func(t *testing.T) {
				var mu sync.Mutex
				var cancelledAlgo []string
				orderPosts := 0
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					defer mu.Unlock()
					w.Header().Set("Content-Type", "application/json")
					switch {
					case strings.HasSuffix(r.URL.Path, "/exchangeInfo"):
						_ = json.NewEncoder(w).Encode(map[string]interface{}{"symbols": []map[string]interface{}{{
							"symbol": "XUSDT", "pricePrecision": 1, "quantityPrecision": 3,
							"filters": []map[string]string{
								{"filterType": "LOT_SIZE", "stepSize": "0.001", "minQty": "0.001", "maxQty": "1000"},
								{"filterType": "MIN_NOTIONAL", "notional": "5"},
							},
						}}})
					case strings.Contains(r.URL.Path, "positionRisk"):
						if orderPosts > 0 {
							// 2026-10-20 area2 #2: the close filled — the fresh
							// snapshot confirms flat, which releases protection.
							_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
							return
						}
						_ = json.NewEncoder(w).Encode([]map[string]interface{}{{
							"symbol": "XUSDT", "positionAmt": tc.positionAmt, "entryPrice": "100",
							"markPrice": "100", "unRealizedProfit": "0", "leverage": "2", "liquidationPrice": "0",
						}})
					case strings.HasSuffix(r.URL.Path, "/openOrders"):
						_ = json.NewEncoder(w).Encode([]interface{}{})
					case strings.HasSuffix(r.URL.Path, "/openAlgoOrders"):
						// Own-side SL + opposite-side TP (a manual hedge leg).
						other := "SHORT"
						if tc.side == "SHORT" {
							other = "LONG"
						}
						_ = json.NewEncoder(w).Encode([]map[string]interface{}{
							{"algoId": 11, "symbol": "XUSDT", "orderType": "STOP_MARKET", "positionSide": tc.side, "triggerPrice": "95", "closePosition": true},
							{"algoId": 22, "symbol": "XUSDT", "orderType": "TAKE_PROFIT_MARKET", "positionSide": other, "triggerPrice": "120", "closePosition": true},
						})
					case strings.HasSuffix(r.URL.Path, "/algoOrder") && r.Method == http.MethodDelete:
						body, _ := io.ReadAll(r.Body)
						form, _ := url.ParseQuery(string(body))
						id := form.Get("algoId")
						if id == "" {
							id = r.URL.Query().Get("algoId")
						}
						cancelledAlgo = append(cancelledAlgo, id)
						_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": "200"})
					case strings.HasSuffix(r.URL.Path, "/order") && r.Method == http.MethodPost:
						orderPosts++
						_ = json.NewEncoder(w).Encode(map[string]interface{}{"orderId": 1, "symbol": "XUSDT", "status": "NEW"})
					default:
						t.Errorf("unexpected endpoint %s %s", r.Method, r.URL.Path)
						http.Error(w, "unexpected", 404)
					}
				}))
				defer srv.Close()
				client := futures.NewClient("key", "secret")
				client.BaseURL = srv.URL
				client.HTTPClient = srv.Client()
				ft := &FuturesTrader{client: client}

				qty := 0.0
				if !full {
					qty = 1
				}
				if err := tc.call(ft, qty); err != nil {
					t.Fatalf("close failed: %v", err)
				}
				mu.Lock()
				defer mu.Unlock()
				if orderPosts != 1 {
					t.Fatalf("close orders posted=%d, want 1", orderPosts)
				}
				if full {
					if len(cancelledAlgo) != 1 || cancelledAlgo[0] != "11" {
						t.Fatalf("full close must cancel only own-side SL (algo 11), cancelled=%v", cancelledAlgo)
					}
				} else if len(cancelledAlgo) != 0 {
					t.Fatalf("partial close must keep protection, cancelled=%v", cancelledAlgo)
				}
			})
		}
	}
}
