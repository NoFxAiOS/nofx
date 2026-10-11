package binance

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
)

// review 2026-10-11: inspect both actual SDK submissions through an in-memory
// transport, including the quantized partial size and full-close fallback.
func TestReview11TakeProfitStaleRetryPreservesShape(t *testing.T) {
	for _, side := range []string{"LONG", "SHORT"} {
		for _, tc := range []struct {
			name      string
			quantity  float64
			wantQty   string
			wantClose string
		}{
			{"partial", 0.57, "0.5", ""},
			{"full", 0, "", "true"},
			{"underflow_full", 0.01, "", "true"},
		} {
			t.Run(side+"/"+tc.name, func(t *testing.T) {
				var posts []url.Values
				cancels := 0
				tr := batch1FuturesTrader(func(r *http.Request) (*http.Response, error) {
					switch r.URL.Path {
					case "/fapi/v1/exchangeInfo":
						return batch1Response(r, http.StatusOK, `{"symbols":[{"symbol":"XUSDT","pricePrecision":2,"filters":[{"filterType":"PRICE_FILTER","tickSize":"0.01"},{"filterType":"LOT_SIZE","stepSize":"0.1","minQty":"0.1","maxQty":"100"}]}]}`), nil
					case "/fapi/v1/openOrders":
						return batch1Response(r, http.StatusOK, `[]`), nil
					case "/fapi/v1/openAlgoOrders":
						return batch1Response(r, http.StatusOK, fmt.Sprintf(`[{"algoId":7,"symbol":"XUSDT","orderType":"TAKE_PROFIT_MARKET","positionSide":%q,"closePosition":true}]`, side)), nil
					case "/fapi/v1/algoOrder":
						body, err := io.ReadAll(r.Body)
						if err != nil {
							return nil, err
						}
						form, err := url.ParseQuery(string(body))
						if err != nil {
							return nil, err
						}
						if r.Method == http.MethodDelete {
							cancels++
							if form.Get("algoId") != "7" {
								t.Errorf("wrong stale leg canceled: %v", form)
							}
							return batch1Response(r, http.StatusOK, `{"code":"200"}`), nil
						}
						posts = append(posts, form)
						if len(posts) == 1 {
							return batch1Response(r, http.StatusBadRequest, `{"code":-4130,"msg":"stale protective leg"}`), nil
						}
						return batch1Response(r, http.StatusOK, `{"algoId":8,"symbol":"XUSDT"}`), nil
					default:
						return nil, fmt.Errorf("unexpected endpoint: %s", r.URL.Path)
					}
				})
				if err := tr.SetTakeProfit("XUSDT", side, tc.quantity, 110); err != nil {
					t.Fatal(err)
				}
				if len(posts) != 2 || cancels != 1 {
					t.Fatalf("submissions=%d cancels=%d, want 2/1", len(posts), cancels)
				}
				orderSide := "SELL"
				if side == "SHORT" {
					orderSide = "BUY"
				}
				for i, form := range posts {
					if form.Get("quantity") != tc.wantQty || form.Get("closePosition") != tc.wantClose || form.Get("reduceOnly") != "" || form.Get("positionSide") != side || form.Get("side") != orderSide {
						t.Fatalf("submission %d changed close shape: %v", i+1, form)
					}
				}
				for _, field := range []string{"symbol", "type", "algoType", "workingType", "triggerPrice"} {
					if posts[0].Get(field) != posts[1].Get(field) {
						t.Fatalf("retry changed %s: %q -> %q", field, posts[0].Get(field), posts[1].Get(field))
					}
				}
			})
		}
	}
}
