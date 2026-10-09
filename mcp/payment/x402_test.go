package payment

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"

	"nofx/mcp"
)

func TestDoX402RequestStreamRetriesInitialServerError(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		if call == 1 {
			http.Error(w, "temporary upstream failure", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: ok\n\n"))
	}))
	defer server.Close()

	resp, err := DoX402RequestStream(
		context.Background(),
		server.Client(),
		func() (*http.Request, error) {
			return http.NewRequest(http.MethodPost, server.URL, nil)
		},
		func(string) (string, error) { return "unused", nil },
		"test-claw402",
		mcp.NewNoopLogger(),
	)
	if err != nil {
		t.Fatalf("DoX402RequestStream returned error: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if got := string(body); got != "data: ok\n\n" {
		t.Fatalf("body = %q, want SSE body", got)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("calls = %d, want 2", got)
	}
}

func TestApplyClawModelControlsDisablesThinkingForDeepSeekV4(t *testing.T) {
	body := map[string]any{"stream": true}
	applyClawModelControls(body, "deepseek-v4-flash")

	thinking, ok := body["thinking"].(map[string]any)
	if !ok {
		t.Fatalf("thinking control = %#v, want object", body["thinking"])
	}
	if got := thinking["type"]; got != "disabled" {
		t.Fatalf("thinking.type = %#v, want disabled", got)
	}
}

func TestApplyClawModelControlsLeavesOtherModelsAlone(t *testing.T) {
	body := map[string]any{"stream": true}
	applyClawModelControls(body, "gpt-5.6-sol")
	if _, exists := body["thinking"]; exists {
		t.Fatalf("unexpected thinking control: %#v", body["thinking"])
	}
}

const claw402TestPrivateKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestClaw402SetAPIKeyInvalidRotationClearsOldWallet(t *testing.T) {
	c := NewClaw402ClientWithOptions().(*Claw402Client)
	c.SetAPIKey(claw402TestPrivateKey, "", "")
	if c.privateKey == nil || c.APIKey == "" {
		t.Fatal("valid key was not configured")
	}

	c.SetAPIKey("not-a-private-key", "", "")
	if c.privateKey != nil || c.APIKey != "" {
		t.Fatal("invalid rotation retained the previous signing wallet")
	}
}

func TestClaw402SetAPIKeyNormalizesWhitespaceAndUppercasePrefix(t *testing.T) {
	c := NewClaw402ClientWithOptions().(*Claw402Client)
	c.SetAPIKey("  0X"+claw402TestPrivateKey+"  ", "", "")
	if c.privateKey == nil {
		t.Fatal("normalized private key was rejected")
	}
	if c.APIKey != claw402TestPrivateKey {
		t.Fatalf("APIKey = %q, want normalized hex", c.APIKey)
	}
}

func TestSignBasePaymentHeaderRejectsUnsafeTerms(t *testing.T) {
	privateKey, err := crypto.HexToECDSA(claw402TestPrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		opt  X402AcceptOption
	}{
		{"wrong network", X402AcceptOption{Scheme: "exact", Network: "eip155:1", Asset: BaseUSDCContract, Amount: "1000", PayTo: "0x1111111111111111111111111111111111111111", MaxTimeoutSeconds: 300}},
		{"wrong asset", X402AcceptOption{Scheme: "exact", Network: BaseNetwork, Asset: "0x1111111111111111111111111111111111111111", Amount: "1000", PayTo: "0x1111111111111111111111111111111111111111", MaxTimeoutSeconds: 300}},
		{"excess amount", X402AcceptOption{Scheme: "exact", Network: BaseNetwork, Asset: BaseUSDCContract, Amount: "1000001", PayTo: "0x1111111111111111111111111111111111111111", MaxTimeoutSeconds: 300}},
		{"domain override", X402AcceptOption{Scheme: "exact", Network: BaseNetwork, Asset: BaseUSDCContract, Amount: "1000", PayTo: "0x1111111111111111111111111111111111111111", MaxTimeoutSeconds: 300, Extra: map[string]string{"name": "Evil Token"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header, err := json.Marshal(X402v2PaymentRequired{X402Version: 2, Accepts: []X402AcceptOption{tt.opt}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := SignBasePaymentHeader(privateKey, base64.RawStdEncoding.EncodeToString(header), "test"); err == nil {
				t.Fatal("unsafe payment terms were accepted")
			}
		})
	}
}
