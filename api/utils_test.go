package api

import (
	"testing"
)

func TestMaskSensitiveString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Short string (8 characters or less)",
			input:    "short",
			expected: "****",
		},
		{
			name:     "Normal API key",
			input:    "sk-1234567890abcdefghijklmnopqrstuvwxyz",
			expected: "sk-1****wxyz",
		},
		{
			name:     "Normal private key",
			input:    "0x1234567890abcdef1234567890abcdef12345678",
			expected: "0x12****5678",
		},
		{
			name:     "Exactly 9 characters",
			input:    "123456789",
			expected: "1234****6789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSensitiveString(tt.input)
			if result != tt.expected {
				t.Errorf("MaskSensitiveString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeModelConfigForLog(t *testing.T) {
	models := map[string]struct {
		Enabled         bool   `json:"enabled"`
		APIKey          string `json:"api_key"`
		CustomAPIURL    string `json:"custom_api_url"`
		CustomModelName string `json:"custom_model_name"`
	}{
		"deepseek": {
			Enabled:         true,
			APIKey:          "sk-1234567890abcdefghijklmnopqrstuvwxyz",
			CustomAPIURL:    "https://api.deepseek.com",
			CustomModelName: "deepseek-chat",
		},
	}

	result := SanitizeModelConfigForLog(models)

	deepseekConfig, ok := result["deepseek"].(map[string]interface{})
	if !ok {
		t.Fatal("deepseek config not found or wrong type")
	}

	if deepseekConfig["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", deepseekConfig["enabled"])
	}

	maskedKey, ok := deepseekConfig["api_key"].(string)
	if !ok {
		t.Fatal("api_key not found or wrong type")
	}

	if maskedKey != "sk-1****wxyz" {
		t.Errorf("expected masked api_key='sk-1****wxyz', got %q", maskedKey)
	}

	if deepseekConfig["custom_api_url"] != "https://api.deepseek.com" {
		t.Errorf("custom_api_url should not be masked")
	}
}

func TestSanitizeExchangeConfigForLog(t *testing.T) {
	exchanges := map[string]struct {
		Enabled               bool   `json:"enabled"`
		APIKey                string `json:"api_key"`
		SecretKey             string `json:"secret_key"`
		Testnet               bool   `json:"testnet"`
		HyperliquidWalletAddr string `json:"hyperliquid_wallet_addr"`
		AsterUser             string `json:"aster_user"`
		AsterSigner           string `json:"aster_signer"`
		AsterPrivateKey       string `json:"aster_private_key"`
		LighterWalletAddr     string `json:"lighter_wallet_addr"`
		LighterPrivateKey     string `json:"lighter_private_key"`
	}{
		"binance": {
			Enabled:   true,
			APIKey:    "binance_api_key_1234567890abcdef",
			SecretKey: "binance_secret_key_1234567890abcdef",
			Testnet:   false,
			LighterWalletAddr:   "",
			LighterPrivateKey:   "",
		},
		"hyperliquid": {
			Enabled:               true,
			HyperliquidWalletAddr: "0x1234567890abcdef1234567890abcdef12345678",
			Testnet:               false,
			LighterWalletAddr:     "",
			LighterPrivateKey:     "",
		},
	}

	result := SanitizeExchangeConfigForLog(exchanges)

	// Check Binance configuration
	binanceConfig, ok := result["binance"].(map[string]interface{})
	if !ok {
		t.Fatal("binance config not found or wrong type")
	}

	maskedAPIKey, ok := binanceConfig["api_key"].(string)
	if !ok {
		t.Fatal("binance api_key not found or wrong type")
	}

	if maskedAPIKey != "bina****cdef" {
		t.Errorf("expected masked api_key='bina****cdef', got %q", maskedAPIKey)
	}

	maskedSecretKey, ok := binanceConfig["secret_key"].(string)
	if !ok {
		t.Fatal("binance secret_key not found or wrong type")
	}

	if maskedSecretKey != "bina****cdef" {
		t.Errorf("expected masked secret_key='bina****cdef', got %q", maskedSecretKey)
	}

	// Check Hyperliquid configuration
	hlConfig, ok := result["hyperliquid"].(map[string]interface{})
	if !ok {
		t.Fatal("hyperliquid config not found or wrong type")
	}

	walletAddr, ok := hlConfig["hyperliquid_wallet_addr"].(string)
	if !ok {
		t.Fatal("hyperliquid_wallet_addr not found or wrong type")
	}

	// Wallet address should not be masked
	if walletAddr != "0x1234567890abcdef1234567890abcdef12345678" {
		t.Errorf("wallet address should not be masked, got %q", walletAddr)
	}
}

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty email",
			input:    "",
			expected: "",
		},
		{
			name:     "Invalid format",
			input:    "notanemail",
			expected: "****",
		},
		{
			name:     "Normal email",
			input:    "user@example.com",
			expected: "us****@example.com",
		},
		{
			name:     "Short username",
			input:    "a@example.com",
			expected: "**@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskEmail(tt.input)
			if result != tt.expected {
				t.Errorf("MaskEmail(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
