package trader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// AccountBalance Account balance information
type AccountBalance struct {
	TotalEquity       float64 `json:"total_equity"`        // Total equity
	AvailableBalance  float64 `json:"available_balance"`   // Available balance
	MarginUsed        float64 `json:"margin_used"`         // Used margin
	UnrealizedPnL     float64 `json:"unrealized_pnl"`      // Unrealized PnL
	MaintenanceMargin float64 `json:"maintenance_margin"`  // Maintenance margin
}

// Position Position information
type Position struct {
	Symbol           string  `json:"symbol"`             // Trading pair
	Side             string  `json:"side"`               // "long" or "short"
	Size             float64 `json:"size"`               // Position size
	EntryPrice       float64 `json:"entry_price"`        // Average entry price
	MarkPrice        float64 `json:"mark_price"`         // Mark price
	LiquidationPrice float64 `json:"liquidation_price"`  // Liquidation price
	UnrealizedPnL    float64 `json:"unrealized_pnl"`     // Unrealized PnL
	Leverage         float64 `json:"leverage"`           // Leverage multiplier
	MarginUsed       float64 `json:"margin_used"`        // Used margin
}

// GetBalance Get account balance (implements Trader interface)
func (t *LighterTrader) GetBalance() (map[string]interface{}, error) {
	balance, err := t.GetAccountBalance()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_equity":       balance.TotalEquity,
		"available_balance":  balance.AvailableBalance,
		"margin_used":        balance.MarginUsed,
		"unrealized_pnl":     balance.UnrealizedPnL,
		"maintenance_margin": balance.MaintenanceMargin,
	}, nil
}

// GetAccountBalance Get detailed account balance information
func (t *LighterTrader) GetAccountBalance() (*AccountBalance, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("invalid auth token: %w", err)
	}

	t.accountMutex.RLock()
	accountIndex := t.accountIndex
	t.accountMutex.RUnlock()

	endpoint := fmt.Sprintf("%s/api/v1/account/%d/balance", t.baseURL, accountIndex)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Add auth header
	t.accountMutex.RLock()
	req.Header.Set("Authorization", t.authToken)
	t.accountMutex.RUnlock()

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get balance (status %d): %s", resp.StatusCode, string(body))
	}

	var balance AccountBalance
	if err := json.Unmarshal(body, &balance); err != nil {
		return nil, fmt.Errorf("failed to parse balance response: %w", err)
	}

	return &balance, nil
}

// GetPositionsRaw Get all positions (returns raw type)
func (t *LighterTrader) GetPositionsRaw(symbol string) ([]Position, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("invalid auth token: %w", err)
	}

	t.accountMutex.RLock()
	accountIndex := t.accountIndex
	t.accountMutex.RUnlock()

	endpoint := fmt.Sprintf("%s/api/v1/account/%d/positions", t.baseURL, accountIndex)
	if symbol != "" {
		endpoint += fmt.Sprintf("?symbol=%s", symbol)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Add auth header
	t.accountMutex.RLock()
	req.Header.Set("Authorization", t.authToken)
	t.accountMutex.RUnlock()

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get positions (status %d): %s", resp.StatusCode, string(body))
	}

	var positions []Position
	if err := json.Unmarshal(body, &positions); err != nil {
		return nil, fmt.Errorf("failed to parse positions response: %w", err)
	}

	return positions, nil
}

// GetPositions Get all positions (implements Trader interface)
func (t *LighterTrader) GetPositions() ([]map[string]interface{}, error) {
	positions, err := t.GetPositionsRaw("")
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(positions))
	for _, pos := range positions {
		result = append(result, map[string]interface{}{
			"symbol":             pos.Symbol,
			"side":               pos.Side,
			"size":               pos.Size,
			"entry_price":        pos.EntryPrice,
			"mark_price":         pos.MarkPrice,
			"liquidation_price":  pos.LiquidationPrice,
			"unrealized_pnl":     pos.UnrealizedPnL,
			"leverage":           pos.Leverage,
			"margin_used":        pos.MarginUsed,
		})
	}

	return result, nil
}

// GetPosition Get position for specified symbol
func (t *LighterTrader) GetPosition(symbol string) (*Position, error) {
	positions, err := t.GetPositionsRaw(symbol)
	if err != nil {
		return nil, err
	}

	// Find position for specified symbol
	for _, pos := range positions {
		if pos.Symbol == symbol && pos.Size > 0 {
			return &pos, nil
		}
	}

	// No position
	return nil, nil
}

// GetMarketPrice Get market price
func (t *LighterTrader) GetMarketPrice(symbol string) (float64, error) {
	endpoint := fmt.Sprintf("%s/api/v1/market/ticker?symbol=%s", t.baseURL, symbol)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return 0, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get market price (status %d): %s", resp.StatusCode, string(body))
	}

	var ticker map[string]interface{}
	if err := json.Unmarshal(body, &ticker); err != nil {
		return 0, fmt.Errorf("failed to parse price response: %w", err)
	}

	// Extract latest price
	price, err := SafeFloat64(ticker, "last_price")
	if err != nil {
		return 0, fmt.Errorf("failed to get price: %w", err)
	}

	return price, nil
}

// GetAccountInfo Get complete account information (for AutoTrader)
func (t *LighterTrader) GetAccountInfo() (map[string]interface{}, error) {
	balance, err := t.GetAccountBalance()
	if err != nil {
		return nil, err
	}

	positions, err := t.GetPositionsRaw("")
	if err != nil {
		return nil, err
	}

	// Build return information
	info := map[string]interface{}{
		"total_equity":       balance.TotalEquity,
		"available_balance":  balance.AvailableBalance,
		"margin_used":        balance.MarginUsed,
		"unrealized_pnl":     balance.UnrealizedPnL,
		"maintenance_margin": balance.MaintenanceMargin,
		"positions":          positions,
		"position_count":     len(positions),
	}

	return info, nil
}

// SetLeverage Set leverage multiplier
func (t *LighterTrader) SetLeverage(symbol string, leverage int) error {
	if err := t.ensureAuthToken(); err != nil {
		return fmt.Errorf("invalid auth token: %w", err)
	}

	// TODO: Implement set leverage API call
	// LIGHTER may require signed transaction to set leverage

	return fmt.Errorf("SetLeverage not implemented")
}

// GetMaxLeverage Get maximum leverage multiplier
func (t *LighterTrader) GetMaxLeverage(symbol string) (int, error) {
	// LIGHTER supports up to 50x leverage for BTC/ETH
	// TODO: Get actual limits from API

	if symbol == "BTC-PERP" || symbol == "ETH-PERP" {
		return 50, nil
	}

	// Default 20x for other symbols
	return 20, nil
}
