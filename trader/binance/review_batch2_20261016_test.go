package binance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adshao/go-binance/v2/common"
	"github.com/adshao/go-binance/v2/futures"
	"nofx/trader/types"
)

func reviewKInstantRecovery(t *testing.T) {
	t.Helper()
	previous := unknownSubmitSleep
	unknownSubmitSleep = func(context.Context, time.Duration) error { return nil }
	t.Cleanup(func() { unknownSubmitSleep = previous })
}

// Exercise the SDK's real HTTP request/response handling without a listening
// socket, so these httptest fixtures also work in network-restricted sandboxes.
func reviewKHTTPClient(handler http.Handler) *http.Client {
	return &http.Client{Transport: batch1Transport(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Result(), nil
	})}
}

func reviewKCancelID(r *http.Request, key string) string {
	if id := r.URL.Query().Get(key); id != "" {
		return id
	}
	body, _ := io.ReadAll(r.Body)
	params, _ := url.ParseQuery(string(body))
	return params.Get(key)
}

func reviewKEntryTrader(t *testing.T, post, lookup http.HandlerFunc) *FuturesTrader {
	t.Helper()
	hc := reviewKHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/exchangeInfo"):
			_, _ = io.WriteString(w, `{"symbols":[{"symbol":"XUSDT","pricePrecision":2,"quantityPrecision":3,"filters":[{"filterType":"LOT_SIZE","stepSize":"0.001","minQty":"0.001","maxQty":"1000"},{"filterType":"PRICE_FILTER","tickSize":"0.01"},{"filterType":"MIN_NOTIONAL","notional":"5"}]}]}`)
		case strings.Contains(r.URL.Path, "positionRisk"):
			_, _ = io.WriteString(w, batch1PositionsJSON) // existing leverage=2, no cooldown
		case strings.HasSuffix(r.URL.Path, "/ticker/price"):
			_, _ = io.WriteString(w, `[{"symbol":"XUSDT","price":"100"}]`)
		case strings.HasSuffix(r.URL.Path, "/openOrders"), strings.HasSuffix(r.URL.Path, "/openAlgoOrders"):
			_, _ = io.WriteString(w, `[]`)
		case r.URL.Path == "/fapi/v1/order" && r.Method == http.MethodPost:
			post(w, r)
		case r.URL.Path == "/fapi/v1/order" && r.Method == http.MethodGet:
			lookup(w, r)
		default:
			t.Errorf("unexpected endpoint %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	client := futures.NewClient("key", "secret")
	client.BaseURL = "http://binance.test"
	client.HTTPClient = hc
	return &FuturesTrader{client: client, cacheDuration: time.Minute}
}

func TestReviewKMarketEntryReconciliation(t *testing.T) {
	reviewKInstantRecovery(t)
	for _, direction := range []string{"long", "short"} {
		for _, tc := range []struct {
			name, body, lookupMode, status, executed string
			httpStatus                               int
			submitTimeout                            bool
			lookups                                  int
			recovered, unknown                       bool
		}{
			{name: "plain_503_found", httpStatus: 503, body: "Unknown error", lookups: 1, recovered: true},
			{name: "503_negative_code_found", httpStatus: 503, body: `{"code":-1000,"msg":"internal error"}`, lookups: 1, recovered: true},
			{name: "1007_found", httpStatus: 400, body: `{"code":-1007,"msg":"execution status unknown"}`, lookups: 1, recovered: true},
			{name: "1001_found", httpStatus: 400, body: `{"code":-1001,"msg":"disconnected"}`, lookups: 1, recovered: true},
			{name: "timeout_found", submitTimeout: true, lookups: 1, recovered: true},
			{name: "missing_three_times", httpStatus: 503, body: `{"code":-1007,"msg":"original submit failure"}`, lookupMode: "missing", lookups: 3},
			{name: "definitive_margin", httpStatus: 400, body: `{"code":-2019,"msg":"Margin is insufficient."}`},
			{name: "definitive_precision", httpStatus: 400, body: `{"code":-1111,"msg":"Precision is over the maximum."}`},
			{name: "definitive_notional", httpStatus: 400, body: `{"code":-4164,"msg":"min notional"}`},
			{name: "lookup_network_failure", httpStatus: 503, body: `{"code":-1007,"msg":"original submit failure"}`, lookupMode: "network", lookups: 3, unknown: true},
			{name: "503_auth_code_lookup_network_failure", httpStatus: 503, body: `{"code":-2015,"msg":"original submit failure"}`, lookupMode: "network", lookups: 3, unknown: true},
			{name: "mixed_missing_network", httpStatus: 503, body: `{"code":-1007,"msg":"original submit failure"}`, lookupMode: "mixed", lookups: 3, unknown: true},
			{name: "eventually_found", httpStatus: 503, body: `{"code":-1007,"msg":"original submit failure"}`, lookupMode: "eventual", lookups: 3, recovered: true},
			{name: "cancelled_zero", httpStatus: 503, body: `{"code":-1007,"msg":"original submit failure"}`, status: "CANCELED", executed: "0", lookups: 1},
			{name: "expired_zero", httpStatus: 503, body: `{"code":-1007,"msg":"original submit failure"}`, status: "EXPIRED", executed: "0", lookups: 1},
			{name: "rejected_zero", httpStatus: 503, body: `{"code":-1007,"msg":"original submit failure"}`, status: "REJECTED", executed: "0", lookups: 1},
			{name: "cancelled_partial_fill", httpStatus: 503, body: `{"code":-1007,"msg":"original submit failure"}`, status: "CANCELED", executed: "0.25", lookups: 1, recovered: true},
			{name: "resting_new", httpStatus: 503, body: `{"code":-1007,"msg":"original submit failure"}`, status: "NEW", executed: "0", lookups: 1, recovered: true},
		} {
			t.Run(direction+"/"+tc.name, func(t *testing.T) {
				var mu sync.Mutex
				var submittedID string
				var lookupIDs []string
				posts := 0
				ft := reviewKEntryTrader(t, func(w http.ResponseWriter, r *http.Request) {
					if err := r.ParseForm(); err != nil {
						t.Error(err)
					}
					mu.Lock()
					submittedID = r.Form.Get("newClientOrderId")
					posts++
					mu.Unlock()
					w.WriteHeader(tc.httpStatus)
					_, _ = io.WriteString(w, tc.body)
				}, func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					defer mu.Unlock()
					lookupIDs = append(lookupIDs, r.URL.Query().Get("origClientOrderId"))
					if r.URL.Query().Get("symbol") != "XUSDT" || r.URL.Query().Get("orderId") != "" {
						t.Errorf("lookup must use symbol and original client id: %s", r.URL.RawQuery)
					}
					if tc.lookupMode == "missing" || tc.lookupMode == "mixed" || (tc.lookupMode == "eventual" && len(lookupIDs) < 3) {
						w.WriteHeader(http.StatusBadRequest)
						_, _ = io.WriteString(w, `{"code":-2013,"msg":"Order does not exist."}`)
						return
					}
					status, qty := tc.status, tc.executed
					if status == "" {
						status, qty = "FILLED", "1"
					}
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"orderId": int64(123), "symbol": "XUSDT", "clientOrderId": submittedID,
						"status": status, "executedQty": qty,
					})
				})
				base := ft.client.HTTPClient.Transport
				if base == nil {
					base = http.DefaultTransport
				}
				ft.client.HTTPClient.Transport = batch1Transport(func(r *http.Request) (*http.Response, error) {
					if tc.submitTimeout && r.Method == http.MethodPost && r.URL.Path == "/fapi/v1/order" {
						body, _ := io.ReadAll(r.Body)
						_ = r.Body.Close()
						form, _ := url.ParseQuery(string(body))
						mu.Lock()
						submittedID, posts = form.Get("newClientOrderId"), posts+1
						mu.Unlock()
						return nil, context.DeadlineExceeded
					}
					if r.Method == http.MethodGet && r.URL.Path == "/fapi/v1/order" && (tc.lookupMode == "network" || tc.lookupMode == "mixed") {
						mu.Lock()
						fail := tc.lookupMode == "network" || len(lookupIDs) == 0
						if fail {
							lookupIDs = append(lookupIDs, r.URL.Query().Get("origClientOrderId"))
						}
						mu.Unlock()
						if fail {
							return nil, errors.New("lookup connection lost")
						}
					}
					return base.RoundTrip(r)
				})

				var result map[string]interface{}
				var err error
				if direction == "long" {
					result, err = ft.OpenLong("XUSDT", 1, 2)
				} else {
					result, err = ft.OpenShort("XUSDT", 1, 2)
				}
				mu.Lock()
				defer mu.Unlock()
				if posts != 1 || len(lookupIDs) != tc.lookups || !strings.HasPrefix(submittedID, BrIDPrefix) {
					t.Fatalf("posts=%d lookupIDs=%v submittedID=%q", posts, lookupIDs, submittedID)
				}
				for _, id := range lookupIDs {
					if id != submittedID {
						t.Fatalf("lookup client id %q differs from submitted %q", id, submittedID)
					}
				}
				if tc.recovered {
					if err != nil || result["recoveredUnknownSubmit"] != true || result["orderId"] != int64(123) || result["clientOrderId"] != submittedID || result["symbol"] != "XUSDT" {
						t.Fatalf("recovery result=%v err=%v", result, err)
					}
					wantStatus := tc.status
					if wantStatus == "" {
						wantStatus = "FILLED"
					}
					if result["status"] != futures.OrderStatusType(wantStatus) {
						t.Fatalf("status type/value changed: %T %v", result["status"], result["status"])
					}
					return
				}
				if err == nil || result != nil || errors.Is(err, ErrSubmitUnknown) != tc.unknown {
					t.Fatalf("result=%v err=%v wantUnknown=%v", result, err, tc.unknown)
				}
				var original *common.APIError
				if !errors.As(err, &original) || !strings.Contains(tc.body, fmt.Sprint(original.Code)) || !strings.Contains(tc.body, original.Message) {
					t.Fatalf("original submit API error lost: %v", err)
				}
			})
		}
	}
}

func TestReviewKLimitEntryUnknownSubmit(t *testing.T) {
	reviewKInstantRecovery(t)
	for _, side := range []string{"BUY", "SELL"} {
		for _, reduceOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reduceOnly=%v", side, reduceOnly), func(t *testing.T) {
				var submittedID string
				posts, lookups := 0, 0
				ft := reviewKEntryTrader(t, func(w http.ResponseWriter, r *http.Request) {
					_ = r.ParseForm()
					submittedID = r.Form.Get("newClientOrderId")
					posts++
					w.WriteHeader(http.StatusServiceUnavailable)
					_, _ = io.WriteString(w, `{"code":-1007,"msg":"execution status unknown"}`)
				}, func(w http.ResponseWriter, r *http.Request) {
					lookups++
					if r.URL.Query().Get("origClientOrderId") != submittedID || r.URL.Query().Get("symbol") != "XUSDT" {
						t.Errorf("wrong lookup: %s", r.URL.RawQuery)
					}
					ps := "LONG"
					if side == "SELL" {
						ps = "SHORT"
					}
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"orderId": 456, "symbol": "XUSDT", "clientOrderId": submittedID,
						"status": "NEW", "side": side, "positionSide": ps, "executedQty": "0",
					})
				})
				result, err := ft.PlaceLimitOrder(&types.LimitOrderRequest{
					Symbol: "XUSDT", Side: side, Price: 100, Quantity: 1.2345, Leverage: 2,
					ClientID: "lim-k-recovery", ReduceOnly: reduceOnly,
				})
				if posts != 1 || submittedID != getBrOrderIDFor("lim-k-recovery") {
					t.Fatalf("posts=%d id=%q", posts, submittedID)
				}
				if reduceOnly {
					if err == nil || result != nil || lookups != 0 {
						t.Fatalf("close path changed: result=%+v err=%v lookups=%d", result, err, lookups)
					}
					return
				}
				if err != nil || result == nil || result.OrderID != "456" || result.ClientID != submittedID || result.Status != "NEW" || result.Symbol != "XUSDT" || result.Side != side || result.Price != 100 || result.Quantity != 1.234 || lookups != 1 {
					t.Fatalf("result=%+v err=%v lookups=%d", result, err, lookups)
				}
			})
		}
	}
}

func TestReviewKSyncCleansOnlyProgramProtectionOnFreshFlatSide(t *testing.T) {
	for _, mode := range []string{"long_flat", "long_reopened", "fresh_read_failed"} {
		t.Run(mode, func(t *testing.T) {
			st := batch2SyncStore(t)
			exchangeID := t.Name()
			t.Cleanup(func() {
				binanceSyncStateMutex.Lock()
				delete(binanceSyncState, exchangeID)
				delete(orderSyncAccts, "k|"+exchangeID)
				binanceSyncStateMutex.Unlock()
				binanceSyncAccounts.Delete(exchangeID)
			})
			var mu sync.Mutex
			var cancelled []string
			freshReads, allCancels := 0, 0
			tradeTime := time.Now().Add(-time.Minute).UnixMilli()
			hc := reviewKHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasSuffix(r.URL.Path, "/income"):
					_ = json.NewEncoder(w).Encode([]map[string]interface{}{{"symbol": "XUSDT", "income": "1", "time": tradeTime}})
				case strings.HasSuffix(r.URL.Path, "/userTrades"):
					if r.URL.Query().Get("symbol") != "XUSDT" {
						_, _ = io.WriteString(w, `[]`)
						return
					}
					_ = json.NewEncoder(w).Encode([]map[string]interface{}{
						batch2SyncTrade(1, "XUSDT", "BUY", "LONG", 1, 100, 0, 0.1, tradeTime),
						batch2SyncTrade(2, "XUSDT", "SELL", "LONG", 1, 101, 1, 0.1, tradeTime+1),
					})
				case strings.Contains(r.URL.Path, "positionRisk"):
					freshReads++
					if mode == "fresh_read_failed" {
						w.WriteHeader(http.StatusServiceUnavailable)
						_, _ = io.WriteString(w, `{"code":-1001,"msg":"disconnected"}`)
						return
					}
					rows := []map[string]string{
						{"symbol": "XUSDT", "positionAmt": "0", "positionSide": "LONG"},
						{"symbol": "XUSDT", "positionAmt": "-1", "positionSide": "SHORT"},
					}
					if mode == "long_reopened" {
						rows[0]["positionAmt"] = "1"
					}
					_ = json.NewEncoder(w).Encode(rows)
				case strings.HasSuffix(r.URL.Path, "/openOrders"):
					_ = json.NewEncoder(w).Encode([]map[string]interface{}{
						{"orderId": 11, "symbol": "XUSDT", "type": "STOP_MARKET", "positionSide": "LONG", "clientOrderId": BrIDPrefix + "sl"},
						{"orderId": 12, "symbol": "XUSDT", "type": "TAKE_PROFIT_MARKET", "positionSide": "LONG", "clientOrderId": "manual-tp"},
						{"orderId": 13, "symbol": "XUSDT", "type": "STOP_MARKET", "positionSide": "SHORT", "clientOrderId": BrIDPrefix + "short-sl"},
						{"orderId": 14, "symbol": "XUSDT", "type": "LIMIT", "positionSide": "LONG", "clientOrderId": BrIDPrefix + "entry"},
						{"orderId": 15, "symbol": "XUSDT", "type": "STOP", "positionSide": "LONG"},
					})
				case strings.HasSuffix(r.URL.Path, "/openAlgoOrders"):
					_ = json.NewEncoder(w).Encode([]map[string]interface{}{
						{"algoId": 21, "symbol": "XUSDT", "orderType": "STOP_MARKET", "positionSide": "LONG", "clientAlgoId": BrIDPrefix + "algo-sl"},
						{"algoId": 22, "symbol": "XUSDT", "orderType": "TAKE_PROFIT_MARKET", "positionSide": "LONG", "clientAlgoId": "manual-algo-tp"},
						{"algoId": 23, "symbol": "XUSDT", "orderType": "STOP_MARKET", "positionSide": "SHORT", "clientAlgoId": BrIDPrefix + "algo-short-sl"},
						{"algoId": 24, "symbol": "XUSDT", "orderType": "LIMIT", "positionSide": "LONG", "clientAlgoId": BrIDPrefix + "algo-entry"},
						{"algoId": 25, "symbol": "XUSDT", "orderType": "STOP", "positionSide": "LONG"},
					})
				case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/order"):
					cancelled = append(cancelled, "order:"+reviewKCancelID(r, "orderId"))
					_, _ = io.WriteString(w, `{}`)
				case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/algoOrder"):
					cancelled = append(cancelled, "algo:"+reviewKCancelID(r, "algoId"))
					_, _ = io.WriteString(w, `{}`)
				case r.Method == http.MethodDelete:
					allCancels++
					t.Errorf("unexpected bulk cancel %s", r.URL.Path)
					_, _ = io.WriteString(w, `{}`)
				default:
					t.Errorf("unexpected endpoint %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected", 404)
				}
			}))
			client := futures.NewClient("key", "secret")
			client.BaseURL, client.HTTPClient = "http://binance.test", hc
			ft := &FuturesTrader{
				client: client, cacheDuration: time.Hour, positionsCacheTime: time.Now(),
				// Deliberately stale: missing XUSDT despite its live SHORT (or reopened LONG).
				cachedPositions: []map[string]interface{}{{"symbol": "YUSDT", "side": "long", "positionAmt": float64(1)}},
			}
			if err := ft.SyncOrdersFromBinance("k", exchangeID, "binance", st); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			defer mu.Unlock()
			if freshReads != 1 || allCancels != 0 {
				t.Fatalf("fresh reads=%d bulk cancels=%d", freshReads, allCancels)
			}
			if mode == "long_flat" {
				if !reflect.DeepEqual(cancelled, []string{"order:11", "algo:21"}) {
					t.Fatalf("must cancel only program LONG protection: %v", cancelled)
				}
			} else if len(cancelled) != 0 {
				t.Fatalf("live or unknown position must keep protection: %v", cancelled)
			}
		})
	}
}
