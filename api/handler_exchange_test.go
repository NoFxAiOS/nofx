package api

import (
	"testing"

	"nofx/crypto"
	"nofx/store"
)

func TestSafeExchangeConfigFromStoreIncludesCredentialPresenceFlags(t *testing.T) {
	cfg := &store.Exchange{
		ID:                      "ex-1",
		ExchangeType:            "okx",
		AccountName:             "OKX Main",
		Name:                    "OKX Main",
		Type:                    "cex",
		Enabled:                 true,
		APIKey:                  crypto.EncryptedString("api-test-123"),
		SecretKey:               crypto.EncryptedString("secret-test-123"),
		Passphrase:              crypto.EncryptedString("passphrase-test-123"),
		HyperliquidUnifiedAcct:  true,
		AsterPrivateKey:         crypto.EncryptedString("aster-private-key"),
		LighterPrivateKey:       crypto.EncryptedString("lighter-private-key"),
		LighterAPIKeyPrivateKey: crypto.EncryptedString("lighter-api-key-private-key"),
	}

	safe := safeExchangeConfigFromStore(cfg)
	if !safe.HasAPIKey {
		t.Fatalf("expected has_api_key to be true")
	}
	if !safe.HasSecretKey {
		t.Fatalf("expected has_secret_key to be true")
	}
	if !safe.HasPassphrase {
		t.Fatalf("expected has_passphrase to be true")
	}
	if !safe.HasAsterPrivateKey {
		t.Fatalf("expected has_aster_private_key to be true")
	}
	if !safe.HasLighterPrivateKey {
		t.Fatalf("expected has_lighter_private_key to be true")
	}
	if !safe.HasLighterAPIKey {
		t.Fatalf("expected has_lighter_api_key_private_key to be true")
	}
	if !safe.HyperliquidUnifiedAcct {
		t.Fatalf("expected hyperliquid unified account to be exposed")
	}
}

func TestSafeExchangeConfigFromStoreExposesDerivedHyperliquidAgentAddress(t *testing.T) {
	const key = "0000000000000000000000000000000000000000000000000000000000000001"
	const want = "0x7e5f4552091a69125d5dfcb7b8c2659029395bdf"
	for _, prefix := range []string{"", "0x", "0X"} {
		t.Run("prefix_"+prefix, func(t *testing.T) {
			cfg := &store.Exchange{
				ExchangeType: "hyperliquid",
				APIKey:       crypto.EncryptedString(prefix + key),
			}
			safe := safeExchangeConfigFromStore(cfg)
			if safe.HyperliquidAgentAddress != want {
				t.Fatalf("derived agent address = %q, want %q", safe.HyperliquidAgentAddress, want)
			}
		})
	}
}

func TestEffectiveHyperliquidUnifiedAccountDefaultsAndPreserves(t *testing.T) {
	if !effectiveHyperliquidUnifiedAccount("hyperliquid", nil) {
		t.Fatalf("expected new hyperliquid accounts to default unified account on")
	}
	if effectiveHyperliquidUnifiedAccount("binance", nil) {
		t.Fatalf("expected non-hyperliquid accounts to default unified account off")
	}
	fallbackFalse := effectiveHyperliquidUnifiedAccount("hyperliquid", nil, false)
	if fallbackFalse {
		t.Fatalf("expected omitted update field to preserve existing false value")
	}
	requestedTrue := true
	if !effectiveHyperliquidUnifiedAccount("hyperliquid", &requestedTrue, false) {
		t.Fatalf("expected explicit true to override existing false value")
	}
}
