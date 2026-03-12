package aster

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"nofx/trader/types"
	"strconv"
	"time"
)

// GetBalance Get account balance
func (t *AsterTrader) GetBalance() (map[string]interface{}, error) {
	params := make(map[string]interface{})
	body, err := t.request("GET", "/fapi/v3/balance", params)
	if err != nil {
		return nil, err
	}

	var balances []map[string]interface{}
	if err := json.Unmarshal(body, &balances); err != nil {
		return nil, err
	}

	// Find USDT balance
	availableBalance := 0.0
	crossUnPnl := 0.0
	crossWalletBalance := 0.0
	foundUSDT := false

	for _, bal := range balances {
		if asset, ok := bal["asset"].(string); ok && asset == "USDT" {
			foundUSDT = true

			// Parse Aster fields (reference: https://github.com/asterdex/api-docs)
			if avail, ok := bal["availableBalance"].(string); ok {
				availableBalance, _ = strconv.ParseFloat(avail, 64)
			}
			if unpnl, ok := bal["crossUnPnl"].(string); ok {
				crossUnPnl, _ = strconv.ParseFloat(unpnl, 64)
			}
			if cwb, ok := bal["crossWalletBalance"].(string); ok {
				crossWalletBalance, _ = strconv.ParseFloat(cwb, 64)
			}
			break
		}
	}

	if !foundUSDT {
		logger.Infof("⚠️  USDT asset record not found!")
	}

	// Get positions to calculate margin used and real unrealized PnL
	positions, err := t.GetPositions()
	if err != nil {
		logger.Infof("⚠️  Failed to get position information: %v", err)
		// fallback: use simple calculation when unable to get positions
		return map[string]interface{}{
			"totalWalletBalance":    crossWalletBalance,
			"availableBalance":      availableBalance,
			"totalUnrealizedProfit": crossUnPnl,
		}, nil
	}

	// Critical fix: accumulate real unrealized PnL from positions
	// Aster's crossUnPnl field is inaccurate, need to recalculate from position data
	totalMarginUsed := 0.0
	realUnrealizedPnl := 0.0
	for _, pos := range positions {
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		realUnrealizedPnl += unrealizedPnl

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed
	}

	// Aster correct calculation method:
	// Total equity = available balance + margin used
	// Wallet balance = total equity - unrealized PnL
	// Unrealized PnL = calculated from accumulated positions (don't use API's crossUnPnl)
	totalEquity := availableBalance + totalMarginUsed
	totalWalletBalance := totalEquity - realUnrealizedPnl

	return map[string]interface{}{
		"totalWalletBalance":    totalWalletBalance, // Wallet balance (excluding unrealized PnL)
		"availableBalance":      availableBalance,   // Available balance
		"totalUnrealizedProfit": realUnrealizedPnl,  // Unrealized PnL (accumulated from positions)
	}, nil
}

// GetMarketPrice Get market price
func (t *AsterTrader) GetMarketPrice(symbol string) (float64, error) {
	// Use ticker interface to get current price
	resp, err := t.client.Get(fmt.Sprintf("%s/fapi/v3/ticker/price?symbol=%s", t.baseURL, symbol))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	priceStr, ok := result["price"].(string)
	if !ok {
		return 0, errors.New("unable to get price")
	}

	return strconv.ParseFloat(priceStr, 64)
}

// GetClosedPnL gets recent closing trades from Aster
// Note: Aster does NOT have a position history API, only trade history.
// This returns individual closing trades for real-time position closure detection.
func (t *AsterTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	trades, err := t.GetTrades(startTime, limit)
	if err != nil {
		return nil, err
	}

	// Filter only closing trades (realizedPnl != 0)
	var records []types.ClosedPnLRecord
	for _, trade := range trades {
		if trade.RealizedPnL == 0 {
			continue
		}

		// Determine side from PositionSide or trade direction
		side := "long"
		if trade.PositionSide == "SHORT" || trade.PositionSide == "short" {
			side = "short"
		} else if trade.PositionSide == "BOTH" || trade.PositionSide == "" {
			if trade.Side == "SELL" || trade.Side == "Sell" {
				side = "long"
			} else {
				side = "short"
			}
		}

		// Calculate entry price from PnL
		var entryPrice float64
		if trade.Quantity > 0 {
			if side == "long" {
				entryPrice = trade.Price - trade.RealizedPnL/trade.Quantity
			} else {
				entryPrice = trade.Price + trade.RealizedPnL/trade.Quantity
			}
		}

		records = append(records, types.ClosedPnLRecord{
			Symbol:      trade.Symbol,
			Side:        side,
			EntryPrice:  entryPrice,
			ExitPrice:   trade.Price,
			Quantity:    trade.Quantity,
			RealizedPnL: trade.RealizedPnL,
			Fee:         trade.Fee,
			ExitTime:    trade.Time,
			EntryTime:   trade.Time,
			OrderID:     trade.TradeID,
			ExchangeID:  trade.TradeID,
			CloseType:   "unknown",
		})
	}

	return records, nil
}

// AsterTradeRecord represents a trade from Aster API
type AsterTradeRecord struct {
	ID           int64  `json:"id"`
	Symbol       string `json:"symbol"`
	OrderID      int64  `json:"orderId"`
	Side         string `json:"side"`         // BUY or SELL
	PositionSide string `json:"positionSide"` // LONG or SHORT
	Price        string `json:"price"`
	Qty          string `json:"qty"`
	RealizedPnl  string `json:"realizedPnl"`
	Commission   string `json:"commission"`
	Time         int64  `json:"time"`
	Buyer        bool   `json:"buyer"`
	Maker        bool   `json:"maker"`
}

// GetTrades retrieves trade history from Aster
func (t *AsterTrader) GetTrades(startTime time.Time, limit int) ([]types.TradeRecord, error) {
	if limit <= 0 {
		limit = 500
	}

	// Build request params
	params := map[string]interface{}{
		"startTime": startTime.UnixMilli(),
		"limit":     limit,
	}

	// Use existing request method with signing
	body, err := t.request("GET", "/fapi/v3/userTrades", params)
	if err != nil {
		logger.Infof("⚠️  Aster userTrades API error: %v", err)
		return []types.TradeRecord{}, nil
	}

	var asterTrades []AsterTradeRecord
	if err := json.Unmarshal(body, &asterTrades); err != nil {
		logger.Infof("⚠️  Failed to parse Aster trades response: %v", err)
		return []types.TradeRecord{}, nil
	}

	// Convert to unified TradeRecord format
	var result []types.TradeRecord
	for _, at := range asterTrades {
		price, _ := strconv.ParseFloat(at.Price, 64)
		qty, _ := strconv.ParseFloat(at.Qty, 64)
		fee, _ := strconv.ParseFloat(at.Commission, 64)
		pnl, _ := strconv.ParseFloat(at.RealizedPnl, 64)

		trade := types.TradeRecord{
			TradeID:      strconv.FormatInt(at.ID, 10),
			Symbol:       at.Symbol,
			Side:         at.Side,
			PositionSide: at.PositionSide,
			Price:        price,
			Quantity:     qty,
			RealizedPnL:  pnl,
			Fee:          fee,
			Time:         time.UnixMilli(at.Time).UTC(),
		}
		result = append(result, trade)
	}

	return result, nil
}

// GetOrderBook gets the order book for a symbol
func (t *AsterTrader) GetOrderBook(symbol string, depth int) (bids, asks [][]float64, err error) {
	if depth <= 0 {
		depth = 20
	}

	// Aster uses public endpoint (no signature required)
	resp, err := t.client.Get(fmt.Sprintf("%s/fapi/v3/depth?symbol=%s&limit=%d", t.baseURL, symbol, depth))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch order book: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Bids [][]string `json:"bids"` // [[price, qty], ...]
		Asks [][]string `json:"asks"` // [[price, qty], ...]
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, nil, fmt.Errorf("failed to parse order book: %w", err)
	}

	// Convert string arrays to float64 arrays
	bids = make([][]float64, len(result.Bids))
	for i, bid := range result.Bids {
		if len(bid) >= 2 {
			price, _ := strconv.ParseFloat(bid[0], 64)
			qty, _ := strconv.ParseFloat(bid[1], 64)
			bids[i] = []float64{price, qty}
		}
	}

	asks = make([][]float64, len(result.Asks))
	for i, ask := range result.Asks {
		if len(ask) >= 2 {
			price, _ := strconv.ParseFloat(ask[0], 64)
			qty, _ := strconv.ParseFloat(ask[1], 64)
			asks[i] = []float64{price, qty}
		}
	}

	return bids, asks, nil
}
