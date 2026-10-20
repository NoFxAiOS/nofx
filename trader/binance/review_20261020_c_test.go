package binance

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

// 2026-10-20 review area2 #2: a successful full-close POST must not strip the
// side's protection unless a fresh snapshot confirms the position is flat.
func TestReview1020FullCloseKeepsProtectionWhileRemainderOpen(t *testing.T) {
	previous := closeConfirmSleep
	closeConfirmSleep = func(time.Duration) {}
	t.Cleanup(func() { closeConfirmSleep = previous })
	for _, remainder := range []string{"flat", "partial", "snapshot_error"} {
		for _, side := range []string{"long", "short"} {
			t.Run(side+"/"+remainder, func(t *testing.T) {
				var mu sync.Mutex
				posted := false
				var cancelled []string
				ps, amt, remainAmt := "LONG", "1", "0.6"
				if side == "short" {
					ps, amt, remainAmt = "SHORT", "-1", "-0.6"
				}
				hc := reviewKHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					defer mu.Unlock()
					w.Header().Set("Content-Type", "application/json")
					switch {
					case strings.HasSuffix(r.URL.Path, "/exchangeInfo"):
						_, _ = io.WriteString(w, `{"symbols":[{"symbol":"XUSDT","pricePrecision":2,"quantityPrecision":3,"filters":[{"filterType":"LOT_SIZE","stepSize":"0.001","minQty":"0.001","maxQty":"1000"},{"filterType":"MIN_NOTIONAL","notional":"5"}]}]}`)
					case strings.Contains(r.URL.Path, "positionRisk"):
						switch {
						case !posted:
							_ = json.NewEncoder(w).Encode([]map[string]string{{"symbol": "XUSDT", "positionAmt": amt, "positionSide": ps, "entryPrice": "100", "markPrice": "100", "leverage": "2"}})
						case remainder == "flat":
							_, _ = io.WriteString(w, `[]`)
						case remainder == "partial":
							_ = json.NewEncoder(w).Encode([]map[string]string{{"symbol": "XUSDT", "positionAmt": remainAmt, "positionSide": ps, "entryPrice": "100", "markPrice": "100", "leverage": "2"}})
						default:
							w.WriteHeader(http.StatusServiceUnavailable)
							_, _ = io.WriteString(w, `{"code":-1001,"msg":"disconnected"}`)
						}
					case strings.HasSuffix(r.URL.Path, "/openOrders"):
						_, _ = io.WriteString(w, `[]`)
					case strings.HasSuffix(r.URL.Path, "/openAlgoOrders"):
						_ = json.NewEncoder(w).Encode([]map[string]interface{}{
							{"algoId": 11, "symbol": "XUSDT", "orderType": "STOP_MARKET", "positionSide": ps, "triggerPrice": "95", "closePosition": true},
						})
					case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/algoOrder"):
						cancelled = append(cancelled, reviewKCancelID(r, "algoId"))
						_, _ = io.WriteString(w, `{"code":"200"}`)
					case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/order"):
						posted = true
						_, _ = io.WriteString(w, `{"orderId":1,"symbol":"XUSDT","status":"NEW"}`)
					default:
						t.Errorf("unexpected endpoint %s %s", r.Method, r.URL.Path)
						http.Error(w, "unexpected", 404)
					}
				}))
				client := futures.NewClient("key", "secret")
				client.BaseURL, client.HTTPClient = "http://binance.test", hc
				ft := &FuturesTrader{client: client}
				var err error
				if side == "long" {
					_, err = ft.CloseLong("XUSDT", 0)
				} else {
					_, err = ft.CloseShort("XUSDT", 0)
				}
				if err != nil {
					t.Fatalf("close failed: %v", err)
				}
				mu.Lock()
				defer mu.Unlock()
				if remainder == "flat" {
					if !reflect.DeepEqual(cancelled, []string{"11"}) {
						t.Fatalf("confirmed flat must release own-side SL, cancelled=%v", cancelled)
					}
				} else if len(cancelled) != 0 {
					t.Fatalf("remainder/unknown must keep protection, cancelled=%v", cancelled)
				}
			})
		}
	}
}

// 2026-10-20 review area2 #4: cleanup cancels only protection IDs that were
// listed BEFORE the flat read; a new position's SL/TP appearing afterwards
// (new IDs) must survive.
func TestReview1020SyncCleanupIgnoresProtectionPlacedAfterSnapshot(t *testing.T) {
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
	positionRead := false
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
			// Flat at read time; the NEW position opens right after this read.
			positionRead = true
			_, _ = io.WriteString(w, `[]`)
		case strings.HasSuffix(r.URL.Path, "/openOrders"):
			rows := []map[string]interface{}{
				{"orderId": 11, "symbol": "XUSDT", "type": "STOP_MARKET", "positionSide": "LONG", "clientOrderId": BrIDPrefix + "old-sl"},
			}
			if positionRead {
				rows = append(rows, map[string]interface{}{"orderId": 91, "symbol": "XUSDT", "type": "STOP_MARKET", "positionSide": "LONG", "clientOrderId": BrIDPrefix + "new-sl"})
			}
			_ = json.NewEncoder(w).Encode(rows)
		case strings.HasSuffix(r.URL.Path, "/openAlgoOrders"):
			rows := []map[string]interface{}{
				{"algoId": 21, "symbol": "XUSDT", "orderType": "STOP_MARKET", "positionSide": "LONG", "clientAlgoId": BrIDPrefix + "old-algo"},
			}
			if positionRead {
				rows = append(rows, map[string]interface{}{"algoId": 92, "symbol": "XUSDT", "orderType": "TAKE_PROFIT_MARKET", "positionSide": "LONG", "clientAlgoId": BrIDPrefix + "new-tp"})
			}
			_ = json.NewEncoder(w).Encode(rows)
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/order"):
			cancelled = append(cancelled, "order:"+reviewKCancelID(r, "orderId"))
			_, _ = io.WriteString(w, `{}`)
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/algoOrder"):
			cancelled = append(cancelled, "algo:"+reviewKCancelID(r, "algoId"))
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
		// Pre-warmed cache: the only positionRisk REST read is the FRESH one.
		cachedPositions: []map[string]interface{}{{"symbol": "YUSDT", "side": "long", "positionAmt": float64(1)}},
	}
	if err := ft.SyncOrdersFromBinance("k", exchangeID, "binance", st); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(cancelled, []string{"order:11", "algo:21"}) {
		t.Fatalf("must cancel only the pre-snapshot protection IDs, got %v", cancelled)
	}
}

// 2026-10-20 review area2 #1: UNKNOWN submits expose the client order ID, and
// the status/cancel calls accept it until the exchange yields a numeric ID.
func TestReview1020UnknownSubmitCarriesClientIDAndIsQueryable(t *testing.T) {
	reviewKInstantRecovery(t)
	var mu sync.Mutex
	var submittedID string
	lookupDown := true
	ft := reviewKEntryTrader(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		submittedID = r.Form.Get("newClientOrderId")
		mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"code":-1007,"msg":"execution status unknown"}`)
	}, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Query().Get("orderId") != "" {
			t.Errorf("client-ref lookup must not send orderId: %s", r.URL.RawQuery)
		}
		if lookupDown {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"code":-1001,"msg":"disconnected"}`)
			return
		}
		if r.URL.Query().Get("origClientOrderId") != submittedID {
			t.Errorf("wrong client id: %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"orderId": int64(777), "symbol": "XUSDT", "clientOrderId": submittedID,
			"status": "FILLED", "executedQty": "1", "avgPrice": "100",
		})
	})
	_, err := ft.OpenLong("XUSDT", 1, 2)
	var unknown *SubmitUnknownError
	if !errors.Is(err, ErrSubmitUnknown) || !errors.As(err, &unknown) {
		t.Fatalf("want typed UNKNOWN error, got %v", err)
	}
	mu.Lock()
	id := submittedID
	lookupDown = false
	mu.Unlock()
	if unknown.ClientOrderID != id || unknown.Symbol != "XUSDT" || !strings.HasPrefix(id, BrIDPrefix) {
		t.Fatalf("typed error lost identity: %+v submitted=%q", unknown, id)
	}
	status, err := ft.GetOrderStatus("XUSDT", ClientOrderRefPrefix+unknown.ClientOrderID)
	if err != nil || status["orderId"] != int64(777) || status["status"] != "FILLED" {
		t.Fatalf("status by client ref: %v err=%v", status, err)
	}
}
