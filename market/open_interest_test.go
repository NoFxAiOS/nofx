package market

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type oiRoundTripper func(*http.Request) (*http.Response, error)

func (f oiRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestFetchOpenInterestData(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
		body   string
		want   float64
		bad    bool
	}{
		{"known positive", 200, `{"openInterest":"123.45"}`, 123.45, false},
		{"measured zero", 200, `{"openInterest":"0"}`, 0, false},
		{"rate limited", 429, `{"code":-1003,"msg":"rate limit"}`, 0, true},
		{"server error with numeric body", 503, `{"openInterest":"123.45"}`, 0, true},
		{"missing", 200, `{}`, 0, true},
		{"API error", 200, `{"code":-1121,"msg":"Invalid symbol"}`, 0, true},
		{"empty", 200, `{"openInterest":""}`, 0, true},
		{"invalid number", 200, `{"openInterest":"unknown"}`, 0, true},
		{"negative", 200, `{"openInterest":"-1"}`, 0, true},
		{"nan", 200, `{"openInterest":"NaN"}`, 0, true},
		{"infinity", 200, `{"openInterest":"+Inf"}`, 0, true},
		{"overflow", 200, `{"openInterest":"1e999"}`, 0, true},
		{"wrong JSON type", 200, `{"openInterest":123}`, 0, true},
		{"malformed JSON", 200, `{`, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{Transport: oiRoundTripper(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host != "fapi.binance.com" || req.URL.Path != "/fapi/v1/openInterest" || req.URL.Query().Get("symbol") != "BTCUSDT" {
					t.Fatalf("unexpected OI request: %s", req.URL)
				}
				return &http.Response{StatusCode: tt.status, Body: io.NopCloser(strings.NewReader(tt.body)), Header: make(http.Header)}, nil
			})}
			got, err := fetchOpenInterestData(client, "BTCUSDT")
			if tt.bad {
				if err == nil || got != nil {
					t.Fatalf("unavailable OI must return nil and an error, got %+v, %v", got, err)
				}
				return
			}
			if err != nil || got == nil || got.Latest != tt.want {
				t.Fatalf("got %+v, %v; want observed OI %v", got, err, tt.want)
			}
		})
	}
}

func TestFetchOpenInterestNetworkFailure(t *testing.T) {
	client := &http.Client{Transport: oiRoundTripper(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("offline")
	})}
	if got, err := fetchOpenInterestData(client, "BTCUSDT"); err == nil || got != nil {
		t.Fatalf("network failure must not become zero OI: %+v, %v", got, err)
	}
}

func TestCandleSnapshotDoesNotInventOpenInterest(t *testing.T) {
	data, err := BuildDataFromKlines("BTCUSDT", generateTestKlines(40), nil)
	if err != nil {
		t.Fatal(err)
	}
	if data.OpenInterest != nil {
		t.Fatal("candles alone must not imply a measured zero open interest")
	}
	if formatted := Format(data); !strings.Contains(formatted, "Open Interest: unavailable") {
		t.Fatalf("missing OI must be explicit in market text: %s", formatted)
	}
}
