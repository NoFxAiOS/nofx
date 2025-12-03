package api

import "strings"

// MaskSensitiveString 脱敏敏感字符串，只显示前4位和后4位
// 用于脱敏 API Key、Secret Key、Private Key 等敏感信息
func MaskSensitiveString(s string) string {
	if s == "" {
		return ""
	}
	length := len(s)
	if length <= 8 {
		return "****" // 字符串太短，全部隐藏
	}
	return s[:4] + "****" + s[length-4:]
}

// SanitizeModelConfigForLog 脱敏模型配置用于日志输出
func SanitizeModelConfigForLog(models map[string]struct {
	Enabled         bool   `json:"enabled"`
	APIKey          string `json:"api_key"`
	CustomAPIURL    string `json:"custom_api_url"`
	CustomModelName string `json:"custom_model_name"`
}) map[string]interface{} {
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

// SanitizeExchangeConfigForLog 脱敏交易所配置用于日志输出
func SanitizeExchangeConfigForLog(exchanges map[string]struct {
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
}) map[string]interface{} {
	safe := make(map[string]interface{})
	for exchangeID, cfg := range exchanges {
		safeExchange := map[string]interface{}{
			"enabled": cfg.Enabled,
			"testnet": cfg.Testnet,
		}

		// 只在有值时才添加脱敏后的敏感字段
		if cfg.APIKey != "" {
			safeExchange["api_key"] = MaskSensitiveString(cfg.APIKey)
		}
		if cfg.SecretKey != "" {
			safeExchange["secret_key"] = MaskSensitiveString(cfg.SecretKey)
		}
		if cfg.AsterPrivateKey != "" {
			safeExchange["aster_private_key"] = MaskSensitiveString(cfg.AsterPrivateKey)
		}
		if cfg.LighterPrivateKey != "" {
			safeExchange["lighter_private_key"] = MaskSensitiveString(cfg.LighterPrivateKey)
		}

		// 非敏感字段直接添加
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

// MaskEmail 脱敏邮箱地址，保留前2位和@后部分
func MaskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "****" // 格式不正确
	}
	username := parts[0]
	domain := parts[1]
	if len(username) <= 2 {
		return "**@" + domain
	}
	return username[:2] + "****@" + domain
}
