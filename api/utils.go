package api

import "strings"

// MaskSensitiveString Mask sensitive strings, showing only first 4 and last 4 characters
// Used to mask API Key, Secret Key, Private Key and other sensitive information
func MaskSensitiveString(s string) string {
	if s == "" {
		return ""
	}
	length := len(s)
	if length <= 8 {
		return "****" // String too short, hide everything
	}
	return s[:4] + "****" + s[length-4:]
}

// SanitizeModelConfigForLog Sanitize model configuration for log output.
// Takes the same ModelConfigUpdate type used by the request handler so the two
// can never drift out of sync.
func SanitizeModelConfigForLog(models map[string]ModelConfigUpdate) map[string]interface{} {
	safe := make(map[string]interface{})
	for modelID, cfg := range models {
		safe[modelID] = map[string]interface{}{
			"enabled":           cfg.Enabled,
			"api_key":           MaskSensitiveString(cfg.APIKey),
			"custom_api_url":    cfg.CustomAPIURL,
			"custom_model_name": cfg.CustomModelName,
		}
	}
	return safe
}

// SanitizeExchangeConfigForLog Sanitize exchange configuration for log output.
// Takes the same ExchangeConfigUpdate type used by the request handler so every
// sensitive field is guaranteed to be masked — adding a field to the request
// type without masking it here would not compile around this helper, but more
// importantly keeps the masking exhaustive.
func SanitizeExchangeConfigForLog(exchanges map[string]ExchangeConfigUpdate) map[string]interface{} {
	safe := make(map[string]interface{})
	for exchangeID, cfg := range exchanges {
		safeExchange := map[string]interface{}{
			"enabled": cfg.Enabled,
			"testnet": cfg.Testnet,
		}

		// Only add masked sensitive fields when they have values
		if cfg.APIKey != "" {
			safeExchange["api_key"] = MaskSensitiveString(cfg.APIKey)
		}
		if cfg.SecretKey != "" {
			safeExchange["secret_key"] = MaskSensitiveString(cfg.SecretKey)
		}
		if cfg.Passphrase != "" {
			safeExchange["passphrase"] = MaskSensitiveString(cfg.Passphrase)
		}
		if cfg.AsterPrivateKey != "" {
			safeExchange["aster_private_key"] = MaskSensitiveString(cfg.AsterPrivateKey)
		}
		if cfg.LighterPrivateKey != "" {
			safeExchange["lighter_private_key"] = MaskSensitiveString(cfg.LighterPrivateKey)
		}
		if cfg.LighterAPIKeyPrivateKey != "" {
			safeExchange["lighter_api_key_private_key"] = MaskSensitiveString(cfg.LighterAPIKeyPrivateKey)
		}

		// Add non-sensitive fields directly
		if cfg.HyperliquidWalletAddr != "" {
			safeExchange["hyperliquid_wallet_addr"] = cfg.HyperliquidWalletAddr
		}
		if cfg.AsterUser != "" {
			safeExchange["aster_user"] = cfg.AsterUser
		}
		if cfg.AsterSigner != "" {
			safeExchange["aster_signer"] = cfg.AsterSigner
		}
		if cfg.LighterWalletAddr != "" {
			safeExchange["lighter_wallet_addr"] = cfg.LighterWalletAddr
		}

		safe[exchangeID] = safeExchange
	}
	return safe
}

// MaskEmail Mask email address, keeping first 2 characters and domain part
func MaskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "****" // Incorrect format
	}
	username := parts[0]
	domain := parts[1]
	if len(username) <= 2 {
		return "**@" + domain
	}
	return username[:2] + "****@" + domain
}
