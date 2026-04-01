package api

import (
	"fmt"

	"nofx/store"
	"nofx/trader"
	"nofx/trader/aster"
	"nofx/trader/binance"
	"nofx/trader/bitget"
	"nofx/trader/bybit"
	"nofx/trader/gate"
	hyperliquidtrader "nofx/trader/hyperliquid"
	"nofx/trader/kucoin"
	"nofx/trader/lighter"
	"nofx/trader/okx"
)

func buildBalanceQueryTrader(exchangeCfg *store.Exchange, userID string) (trader.Trader, error) {
	switch exchangeCfg.ExchangeType {
	case "binance":
		return binance.NewFuturesTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), userID), nil
	case "hyperliquid":
		return hyperliquidtrader.NewHyperliquidTrader(
			string(exchangeCfg.APIKey),
			exchangeCfg.HyperliquidWalletAddr,
			exchangeCfg.Testnet,
			exchangeCfg.HyperliquidUnifiedAcct,
		)
	case "aster":
		return aster.NewAsterTrader(
			exchangeCfg.AsterUser,
			exchangeCfg.AsterSigner,
			string(exchangeCfg.AsterPrivateKey),
		)
	case "bybit":
		return bybit.NewBybitTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
		), nil
	case "okx":
		return okx.NewOKXTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
			string(exchangeCfg.Passphrase),
		), nil
	case "bitget":
		return bitget.NewBitgetTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
			string(exchangeCfg.Passphrase),
		), nil
	case "gate":
		return gate.NewGateTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
		), nil
	case "kucoin":
		return kucoin.NewKuCoinTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
			string(exchangeCfg.Passphrase),
		), nil
	case "lighter":
		if exchangeCfg.LighterWalletAddr != "" && string(exchangeCfg.LighterAPIKeyPrivateKey) != "" {
			return lighter.NewLighterTraderV2(
				exchangeCfg.LighterWalletAddr,
				string(exchangeCfg.LighterAPIKeyPrivateKey),
				exchangeCfg.LighterAPIKeyIndex,
				false,
			)
		}
		return nil, fmt.Errorf("Lighter requires wallet address and API Key private key")
	default:
		return nil, fmt.Errorf("unsupported exchange type: %s", exchangeCfg.ExchangeType)
	}
}

func extractExchangeEquity(balanceInfo map[string]interface{}) float64 {
	balanceKeys := []string{"total_equity", "totalWalletBalance", "wallet_balance", "totalEq", "balance"}
	for _, key := range balanceKeys {
		if balance, ok := balanceInfo[key].(float64); ok && balance > 0 {
			return balance
		}
	}
	return 0
}

func queryExchangeEquity(exchangeCfg *store.Exchange, userID string) (float64, error) {
	if exchangeCfg == nil {
		return 0, fmt.Errorf("exchange config is nil")
	}

	tempTrader, err := buildBalanceQueryTrader(exchangeCfg, userID)
	if err != nil {
		return 0, err
	}

	balanceInfo, err := tempTrader.GetBalance()
	if err != nil {
		return 0, err
	}

	actualBalance := extractExchangeEquity(balanceInfo)
	if actualBalance <= 0 {
		return 0, fmt.Errorf("unable to get total equity")
	}

	return actualBalance, nil
}
