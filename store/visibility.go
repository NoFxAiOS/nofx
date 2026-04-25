package store

import "strings"

func IsVisibleAIModel(model *AIModel) bool {
	if model == nil {
		return false
	}
	return model.Enabled ||
		strings.TrimSpace(string(model.APIKey)) != "" ||
		strings.TrimSpace(model.CustomAPIURL) != "" ||
		strings.TrimSpace(model.CustomModelName) != ""
}

func IsVisibleExchange(exchange *Exchange) bool {
	if exchange == nil {
		return false
	}
	return exchange.Enabled ||
		strings.TrimSpace(string(exchange.APIKey)) != "" ||
		strings.TrimSpace(string(exchange.SecretKey)) != "" ||
		strings.TrimSpace(string(exchange.Passphrase)) != "" ||
		strings.TrimSpace(exchange.HyperliquidWalletAddr) != "" ||
		strings.TrimSpace(exchange.AsterUser) != "" ||
		strings.TrimSpace(exchange.AsterSigner) != "" ||
		strings.TrimSpace(string(exchange.AsterPrivateKey)) != "" ||
		strings.TrimSpace(exchange.LighterWalletAddr) != "" ||
		strings.TrimSpace(string(exchange.LighterPrivateKey)) != "" ||
		strings.TrimSpace(string(exchange.LighterAPIKeyPrivateKey)) != "" ||
		exchange.LighterAPIKeyIndex != 0
}

func IsVisibleTrader(trader *Trader) bool {
	if trader == nil {
		return false
	}
	return strings.TrimSpace(trader.Name) != "" &&
		strings.TrimSpace(trader.AIModelID) != "" &&
		strings.TrimSpace(trader.ExchangeID) != ""
}

func IsVisibleStrategy(strategy *Strategy) bool {
	if strategy == nil {
		return false
	}
	return strings.TrimSpace(strategy.Name) != ""
}

