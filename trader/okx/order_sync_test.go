package okx

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"nofx/store"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newOKXSyncTestTrader(t *testing.T, fillsJSON string) *OKXTrader {
	t.Helper()
	return &OKXTrader{
		apiKey:     "test-key",
		secretKey:  "test-secret",
		passphrase: "test-passphrase",
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"code":"0","msg":"","data":[]}`
			switch {
			case strings.HasPrefix(req.URL.Path, "/api/v5/trade/fills-history"):
				body = fmt.Sprintf(`{"code":"0","msg":"","data":%s}`, fillsJSON)
			case strings.HasPrefix(req.URL.Path, "/api/v5/public/instruments"):
				body = `{"code":"0","msg":"","data":[{"instId":"BTC-USDT-SWAP","ctVal":"0.0001","ctMult":"1","lotSz":"1","minSz":"1","maxMktSz":"1000000","tickSz":"0.1","ctType":"linear"}]}`
			default:
				t.Fatalf("unexpected OKX request path: %s", req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    req,
			}, nil
		})},
		instrumentsCache: make(map[string]*OKXInstrument),
	}
}

func newOKXSyncTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "order-sync-test.db"))
	if err != nil {
		t.Fatalf("create test store: %v", err)
	}
	return st
}

func TestSyncOrdersFromOKXWithFullCloseHandler_InvokesCallbackOnFullClose(t *testing.T) {
	fills := `[
		{"instId":"BTC-USDT-SWAP","tradeId":"trade-open","ordId":"order-open","billId":"bill-open","side":"sell","posSide":"short","fillPx":"78867.8","fillSz":"2","fee":"-0.01","feeCcy":"USDT","ts":"1714260000000","execType":"T","tag":"entry"},
		{"instId":"BTC-USDT-SWAP","tradeId":"trade-close","ordId":"order-close","billId":"bill-close","side":"buy","posSide":"short","fillPx":"77904.4","fillSz":"2","fee":"-0.01","feeCcy":"USDT","ts":"1714260060000","execType":"T","tag":"native_trailing"}
	]`
	tr := newOKXSyncTestTrader(t, fills)
	st := newOKXSyncTestStore(t)

	var callbacks []string
	err := tr.SyncOrdersFromOKXWithFullCloseHandler("trader-1", "exchange-1", "okx", st, func(symbol, side string) {
		callbacks = append(callbacks, symbol+":"+side)
	})
	if err != nil {
		t.Fatalf("sync orders: %v", err)
	}
	if len(callbacks) != 1 || callbacks[0] != "BTCUSDT:SHORT" {
		t.Fatalf("expected one full-close callback for BTCUSDT SHORT, got %v", callbacks)
	}
	openPositions, err := st.Position().GetOpenPositions("trader-1")
	if err != nil {
		t.Fatalf("get open positions: %v", err)
	}
	if len(openPositions) != 0 {
		t.Fatalf("expected no open positions after full close sync, got %d", len(openPositions))
	}
	orders, err := st.Order().GetTraderOrders("trader-1", 10)
	if err != nil {
		t.Fatalf("get trader orders: %v", err)
	}
	if len(orders) != 2 {
		t.Fatalf("expected 2 synced orders, got %d", len(orders))
	}
	var openPositionID, closePositionID int64
	for _, order := range orders {
		switch order.ExchangeOrderID {
		case "trade-open":
			openPositionID = order.RelatedPositionID
		case "trade-close":
			closePositionID = order.RelatedPositionID
			if order.OrderAction != "native_trailing" {
				t.Fatalf("expected close order action to preserve native_trailing source, got %q", order.OrderAction)
			}
		}
	}
	if openPositionID == 0 || closePositionID == 0 {
		t.Fatalf("expected synced open/close orders attached to position, got open=%d close=%d", openPositionID, closePositionID)
	}
	if openPositionID != closePositionID {
		t.Fatalf("expected open and close orders attached to same position, got open=%d close=%d", openPositionID, closePositionID)
	}
}

func TestSyncOrdersFromOKXWithFullCloseHandler_DoesNotInvokeCallbackOnPartialClose(t *testing.T) {
	fills := `[
		{"instId":"BTC-USDT-SWAP","tradeId":"trade-open","ordId":"order-open","billId":"bill-open","side":"sell","posSide":"short","fillPx":"78867.8","fillSz":"3","fee":"-0.01","feeCcy":"USDT","ts":"1714260000000","execType":"T","tag":"entry"},
		{"instId":"BTC-USDT-SWAP","tradeId":"trade-close","ordId":"order-close","billId":"bill-close","side":"buy","posSide":"short","fillPx":"77904.4","fillSz":"1","fee":"-0.01","feeCcy":"USDT","ts":"1714260060000","execType":"T","tag":"native_trailing"}
	]`
	tr := newOKXSyncTestTrader(t, fills)
	st := newOKXSyncTestStore(t)

	callbackCount := 0
	err := tr.SyncOrdersFromOKXWithFullCloseHandler("trader-1", "exchange-1", "okx", st, func(symbol, side string) {
		callbackCount++
	})
	if err != nil {
		t.Fatalf("sync orders: %v", err)
	}
	if callbackCount != 0 {
		t.Fatalf("expected no full-close callback on partial close, got %d", callbackCount)
	}
	openPositions, err := st.Position().GetOpenPositions("trader-1")
	if err != nil {
		t.Fatalf("get open positions: %v", err)
	}
	if len(openPositions) != 1 {
		t.Fatalf("expected one remaining open position after partial close, got %d", len(openPositions))
	}
	if got := openPositions[0].Quantity; got < 0.00019 || got > 0.00021 {
		t.Fatalf("expected remaining quantity about 0.0002, got %.8f", got)
	}
}
