package execution

import (
	"fmt"
	"strings"

	"nofx/trader/binance"
	"nofx/trader/bitget"
	"nofx/trader/bybit"
	"nofx/trader/gate"
	"nofx/trader/kucoin"
	"nofx/trader/okx"
)

// ExchangeConfig holds credentials for creating a trader.
type ExchangeConfig struct {
	Name       string
	APIKey     string
	APISecret  string
	Passphrase string
	Testnet    bool
}

// CreateTrader creates a NofxTrader for the given exchange.
func CreateTrader(cfg ExchangeConfig) (NofxTrader, error) {
	switch strings.ToLower(cfg.Name) {
	case "binance":
		return binance.NewFuturesTrader(cfg.APIKey, cfg.APISecret, "nofxi"), nil

	case "okx":
		return okx.NewOKXTrader(cfg.APIKey, cfg.APISecret, cfg.Passphrase), nil

	case "bybit":
		return bybit.NewBybitTrader(cfg.APIKey, cfg.APISecret), nil

	case "bitget":
		return bitget.NewBitgetTrader(cfg.APIKey, cfg.APISecret, cfg.Passphrase), nil

	case "kucoin":
		return kucoin.NewKuCoinTrader(cfg.APIKey, cfg.APISecret, cfg.Passphrase), nil

	case "gate":
		return gate.NewGateTrader(cfg.APIKey, cfg.APISecret), nil

	// Hyperliquid needs private key, not API key/secret
	// case "hyperliquid":
	//   return hyperliquid.NewHyperliquidTrader(cfg.APIKey, "", cfg.Testnet)

	default:
		return nil, fmt.Errorf("unsupported exchange: %s", cfg.Name)
	}
}
