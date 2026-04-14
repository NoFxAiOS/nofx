package gmgn

import (
	"fmt"
	"nofx/logger"
	"nofx/market"
	gmgnprovider "nofx/provider/gmgn"
	"nofx/store"
	"sort"
	"strings"
	"time"
)

func (t *Trader) SyncOrdersFromActivity(traderID, exchangeID, exchangeType string, st *store.Store) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}

	resp, err := t.client.GetWalletActivity(t.chain, t.walletAddress, map[string]any{"limit": 200})
	if err != nil {
		return err
	}

	activities := resp.Activities
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].Timestamp < activities[j].Timestamp
	})

	orderStore := st.Order()
	posBuilder := store.NewPositionBuilder(st.Position())
	synced := 0

	for _, activity := range activities {
		action, side, positionSide, symbol, quantity, price, realizedPnL, fee, ok := t.mapActivity(activity)
		if !ok {
			continue
		}

		if existing, err := orderStore.GetOrderByExchangeID(exchangeID, activity.TxHash); err == nil && existing != nil {
			continue
		}

		tradeTimeMs := normalizeActivityTimestamp(activity.Timestamp)
		orderRecord := &store.TraderOrder{
			TraderID:        traderID,
			ExchangeID:      exchangeID,
			ExchangeType:    exchangeType,
			ExchangeOrderID: activity.TxHash,
			Symbol:          symbol,
			Side:            side,
			PositionSide:    positionSide,
			Type:            "MARKET",
			OrderAction:     action,
			Quantity:        quantity,
			Price:           price,
			Status:          "FILLED",
			FilledQuantity:  quantity,
			AvgFillPrice:    price,
			Commission:      fee,
			CommissionAsset: t.chainConfig.NativeTokenSymbol,
			FilledAt:        tradeTimeMs,
			CreatedAt:       tradeTimeMs,
			UpdatedAt:       tradeTimeMs,
		}
		if err := orderStore.CreateOrder(orderRecord); err != nil {
			logger.Infof("⚠️ GMGN failed to sync order %s: %v", activity.TxHash, err)
			continue
		}

		fill := &store.TraderFill{
			TraderID:        traderID,
			ExchangeID:      exchangeID,
			ExchangeType:    exchangeType,
			OrderID:         orderRecord.ID,
			ExchangeOrderID: activity.TxHash,
			ExchangeTradeID: activity.TxHash,
			Symbol:          symbol,
			Side:            side,
			Price:           price,
			Quantity:        quantity,
			QuoteQuantity:   price * quantity,
			Commission:      fee,
			CommissionAsset: t.chainConfig.NativeTokenSymbol,
			RealizedPnL:     realizedPnL,
			IsMaker:         false,
			CreatedAt:       tradeTimeMs,
		}
		if err := orderStore.CreateFill(fill); err != nil {
			logger.Infof("⚠️ GMGN failed to sync fill %s: %v", activity.TxHash, err)
		}

		if err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, positionSide, action,
			quantity, price, fee, realizedPnL,
			tradeTimeMs, activity.TxHash,
		); err != nil {
			logger.Infof("⚠️ GMGN failed to update position for %s: %v", activity.TxHash, err)
		}

		synced++
	}

	logger.Infof("🔄 GMGN order sync completed: trader=%s synced=%d", traderID, synced)
	return nil
}

func (t *Trader) StartOrderSync(traderID, exchangeID, exchangeType string, st *store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := t.SyncOrdersFromActivity(traderID, exchangeID, exchangeType, st); err != nil {
				logger.Infof("⚠️ GMGN order sync failed: %v", err)
			}
		}
	}()
	logger.Infof("🔄 GMGN order+position sync started (interval: %v)", interval)
}

func (t *Trader) mapActivity(activity gmgnprovider.WalletActivity) (action, side, positionSide, symbol string, quantity, price, realizedPnL, fee float64, ok bool) {
	tokenAddress := strings.TrimSpace(activity.Token.Address)
	if tokenAddress == "" || strings.EqualFold(tokenAddress, t.chainConfig.USDCAddress) {
		return "", "", "", "", 0, 0, 0, 0, false
	}

	symbol = market.Normalize(gmgnprovider.FormatSymbol(t.chain, tokenAddress))
	quantity = gmgnprovider.ParseFloatString(activity.TokenAmount)
	price = gmgnprovider.ParseFloatString(activity.PriceUSD)
	if price <= 0 {
		price = gmgnprovider.ParseFloatString(activity.Price)
	}
	fee = gmgnprovider.ParseFloatString(activity.GasUSD) + gmgnprovider.ParseFloatString(activity.DEXUSD)
	positionSide = "LONG"

	switch strings.ToLower(strings.TrimSpace(activity.EventType)) {
	case "buy", "swap_in", "swap_buy":
		action = "open_long"
		side = "BUY"
	case "sell", "swap_out", "swap_sell":
		action = "close_long"
		side = "SELL"
		realizedPnL = gmgnprovider.ParseFloatString(activity.CostUSD) - gmgnprovider.ParseFloatString(activity.BuyCostUSD)
	default:
		if activity.IsOpenOrClose > 0 {
			action = "open_long"
			side = "BUY"
		} else if activity.IsOpenOrClose < 0 {
			action = "close_long"
			side = "SELL"
			realizedPnL = gmgnprovider.ParseFloatString(activity.CostUSD) - gmgnprovider.ParseFloatString(activity.BuyCostUSD)
		} else {
			return "", "", "", "", 0, 0, 0, 0, false
		}
	}

	if quantity <= 0 || price <= 0 {
		return "", "", "", "", 0, 0, 0, 0, false
	}

	return action, side, positionSide, symbol, quantity, price, realizedPnL, fee, true
}

func normalizeActivityTimestamp(ts int64) int64 {
	if ts > 1_000_000_000_000 {
		return ts
	}
	return ts * 1000
}
