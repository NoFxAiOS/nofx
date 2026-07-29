package api

import (
	"strings"
	"testing"
)

func usdClassTransferAction(overrides map[string]any) map[string]any {
	action := map[string]any{
		"type":             "usdClassTransfer",
		"signatureChainId": "0x66eee",
		"hyperliquidChain": "Mainnet",
		"amount":           "21.5",
		"toPerp":           true,
		"nonce":            float64(1784900000000),
	}
	for k, v := range overrides {
		if v == nil {
			delete(action, k)
			continue
		}
		action[k] = v
	}
	return action
}

func TestValidateUsdClassTransferActionAcceptsValidTransfer(t *testing.T) {
	if err := validateUsdClassTransferAction(usdClassTransferAction(nil)); err != nil {
		t.Fatalf("expected valid usdClassTransfer to pass, got %v", err)
	}
}

func TestValidateUsdClassTransferActionRejectsBadAmounts(t *testing.T) {
	cases := map[string]any{
		"zero":       "0",
		"negative":   "-5",
		"not number": "abc",
		"empty":      "",
		// The SDK's subaccount suffix must not be relayable from the browser.
		"subaccount": "21.5 subaccount:0x1234",
	}
	for name, amount := range cases {
		err := validateUsdClassTransferAction(usdClassTransferAction(map[string]any{"amount": amount}))
		if err == nil {
			t.Fatalf("%s: expected amount %q to be rejected", name, amount)
		}
	}
}

func TestValidateUsdClassTransferActionRequiresBooleanToPerp(t *testing.T) {
	if err := validateUsdClassTransferAction(usdClassTransferAction(map[string]any{"toPerp": nil})); err == nil {
		t.Fatal("expected missing toPerp to be rejected")
	}
	if err := validateUsdClassTransferAction(usdClassTransferAction(map[string]any{"toPerp": "true"})); err == nil {
		t.Fatal("expected non-boolean toPerp to be rejected")
	}
	if err := validateUsdClassTransferAction(usdClassTransferAction(map[string]any{"toPerp": false})); err != nil {
		t.Fatalf("expected toPerp=false (perp->spot) to be valid, got %v", err)
	}
}

func TestValidateUsdClassTransferActionRequiresMainnetChain(t *testing.T) {
	err := validateUsdClassTransferAction(usdClassTransferAction(map[string]any{"hyperliquidChain": "Testnet"}))
	if err == nil || !strings.Contains(err.Error(), "hyperliquidChain") {
		t.Fatalf("expected Testnet chain to be rejected, got %v", err)
	}
}
