package usstock

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

const chartRows = `[
 [1700000000000,"100.5","110","90","105.25","50",1700000299999,"5262.5",10,"0","0","0"],
 [1700000300000,"105.25","111","95","108","20",1700000599999,"2160",5,"0","0","0"]
]`

func TestChartKlinesParsesFormingBar(t *testing.T) {
	var got http.Request
	setupHTTP(t, time.Unix(1700000400, 0), func(r *http.Request) (*http.Response, error) {
		got = *r
		return response(200, chartRows), nil
	})
	bars, err := ChartKlines(context.Background(), "AAPLBUSDT", "5m", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 2 {
		t.Fatalf("want 2 bars (forming kept), got %d", len(bars))
	}
	want := ChartKline{OpenTime: 1700000000000, CloseTime: 1700000299999, Open: 100.5, High: 110, Low: 90, Close: 105.25, Volume: 50, QuoteVolume: 5262.5}
	if bars[0] != want {
		t.Fatalf("got %+v want %+v", bars[0], want)
	}
	if bars[1].OpenTime <= bars[0].OpenTime || bars[1].QuoteVolume != 2160 {
		t.Fatalf("unexpected second bar %+v", bars[1])
	}
	q := got.URL.Query()
	if got.URL.Path != "/api/v3/klines" || q.Get("symbol") != "AAPLBUSDT" || q.Get("interval") != "5m" || q.Get("limit") != "3" {
		t.Fatalf("unexpected request %s", got.URL)
	}
}

func TestChartKlinesLimitDefaultAndClamp(t *testing.T) {
	var limit string
	setupHTTP(t, time.Now(), func(r *http.Request) (*http.Response, error) {
		limit = r.URL.Query().Get("limit")
		return response(200, `[]`), nil
	})
	for in, want := range map[int]string{0: "500", -5: "500", 1500: "1000", 1000: "1000", 7: "7"} {
		bars, err := ChartKlines(context.Background(), "AAPLBUSDT", "1h", in)
		if err != nil || len(bars) != 0 {
			t.Fatalf("limit %d: bars=%v err=%v", in, bars, err)
		}
		if limit != want {
			t.Fatalf("limit %d: sent %s want %s", in, limit, want)
		}
	}
}

func TestChartKlinesBadInterval(t *testing.T) {
	setupHTTP(t, time.Now(), nil) // any request fails the test
	if _, err := ChartKlines(context.Background(), "AAPLBUSDT", "7m", 10); err == nil {
		t.Fatal("expected error for unsupported interval")
	}
}

func TestChartKlinesHTTPError(t *testing.T) {
	setupHTTP(t, time.Now(), func(r *http.Request) (*http.Response, error) {
		return response(400, `{"code":-1121}`), nil
	})
	_, err := ChartKlines(context.Background(), "NOPEBUSDT", "5m", 10)
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("want HTTP 400 error, got %v", err)
	}
}

func TestChartKlinesMalformedRow(t *testing.T) {
	for name, body := range map[string]string{
		"short":  `[[1700000000000,"1","2","3","4","5"]]`,
		"number": `[[1700000000000,"x","2","3","4","5",1700000299999,"6"]]`,
	} {
		t.Run(name, func(t *testing.T) {
			setupHTTP(t, time.Now(), func(r *http.Request) (*http.Response, error) { return response(200, body), nil })
			if _, err := ChartKlines(context.Background(), "AAPLBUSDT", "5m", 10); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
