package hyperliquid

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestAgentAuthorizationFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		ok         bool
	}{
		{"approved", `[{"address":"0xAbC","validUntil":2000}]`, 200, true},
		{"other agent", `[{"address":"0xdef","validUntil":2000}]`, 200, false},
		{"expired", `[{"address":"0xabc","validUntil":1000}]`, 200, false},
		{"missing", `[]`, 200, false},
		{"malformed", `{}`, 200, false},
		{"offline", `unavailable`, 503, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request map[string]string
				json.NewDecoder(r.Body).Decode(&request)
				if r.URL.Path != "/info" || request["type"] != "extraAgents" || request["user"] != "owner" {
					t.Error("not an authorization read")
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer s.Close()
			err := checkAgentAuthorization(context.Background(), s.Client(), s.URL, "owner", "0xabc", time.UnixMilli(1000))
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v, want success=%v", err, tc.ok)
			}
		})
	}
}

func TestXYZLeverageUsesDexMetadataAndBlocksOrderOnFailure(t *testing.T) {
	// No regression may escape the local fake exchange, even if an order
	// accidentally gets past the leverage guard. No funded key is used.
	originalTransport := http.DefaultTransport
	http.DefaultTransport = loopbackOnlyTransport{base: originalTransport}
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	for _, direction := range []string{"long", "short"} {
		t.Run(direction, func(t *testing.T) {
			key, _ := crypto.GenerateKey()
			agent := crypto.PubkeyToAddress(key.PublicKey).Hex()
			leverageCalls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request map[string]any
				json.NewDecoder(r.Body).Decode(&request)
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/exchange" {
					action := request["action"].(map[string]any)
					if action["type"] != "updateLeverage" {
						t.Errorf("unexpected mutation: %v", action["type"])
					}
					if action["asset"] != float64(110000) || action["leverage"] != float64(5) {
						t.Errorf("wrong leverage asset: %v", action)
					}
					leverageCalls++
					fmt.Fprint(w, `{"status":"err","response":"leverage denied"}`)
					return
				}
				switch request["type"] {
				case "extraAgents":
					fmt.Fprintf(w, `[{"address":%q,"validUntil":%d}]`, agent, time.Now().Add(time.Hour).UnixMilli())
				case "meta":
					if request["dex"] != "xyz" {
						t.Error("missing xyz dex")
					}
					fmt.Fprint(w, `{"universe":[{"name":"xyz:SP500","szDecimals":3,"maxLeverage":20}],"marginTables":[]}`)
				case "perpDexs":
					fmt.Fprint(w, `[null,{"name":"xyz"}]`)
				default:
					t.Errorf("unexpected request after failed leverage: %v", request["type"])
					fmt.Fprint(w, `{}`)
				}
			}))
			defer s.Close()
			tr := &HyperliquidTrader{ctx: context.Background(), apiURL: s.URL, privateKey: key, walletAddr: "owner"}
			var err error
			if direction == "long" {
				_, err = tr.OpenLong("xyz:SP500", 1, 5)
			} else {
				_, err = tr.OpenShort("xyz:SP500", 1, 5)
			}
			if err == nil || !strings.Contains(err.Error(), "leverage denied") || leverageCalls != 1 {
				t.Fatalf("calls=%d err=%v", leverageCalls, err)
			}
		})
	}
}

type loopbackOnlyTransport struct{ base http.RoundTripper }

func (t loopbackOnlyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Hostname() != "127.0.0.1" {
		return nil, fmt.Errorf("test refuses non-local request")
	}
	return t.base.RoundTrip(r)
}

func TestActionTransportRequiresExplicitSuccess(t *testing.T) {
	for _, tc := range []struct {
		body string
		ok   bool
	}{
		{`{"status":"ok","response":{"type":"default"}}`, true},
		{`{"status":"err","response":"permission denied"}`, false},
		{`{"response":{}}`, false},
		{`<html>unavailable</html>`, false},
	} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.body) }))
		resp, err := checkedActionHTTPClient().Post(s.URL+"/exchange", "application/json", strings.NewReader(`{}`))
		if resp != nil {
			resp.Body.Close()
		}
		s.Close()
		if (err == nil) != tc.ok {
			t.Errorf("body=%s err=%v", tc.body, err)
		}
	}
}
