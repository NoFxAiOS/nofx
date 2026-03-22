package execution

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// TraderFactory creates a NOFX Trader by exchange name.
// Injected from main.go where nofx exchange packages are available.
type TraderFactory func(exchange, apiKey, apiSecret, passphrase string, testnet bool) (NofxTrader, error)

// NofxTrader mirrors nofx/trader/types.Trader interface.
// We redefine it here to avoid a direct import cycle with the parent module.
type NofxTrader interface {
	GetBalance() (map[string]interface{}, error)
	GetPositions() ([]map[string]interface{}, error)
	OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error)
	OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error)
	CloseLong(symbol string, quantity float64) (map[string]interface{}, error)
	CloseShort(symbol string, quantity float64) (map[string]interface{}, error)
	SetLeverage(symbol string, leverage int) error
	GetMarketPrice(symbol string) (float64, error)
	SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error
	SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error
	CancelAllOrders(symbol string) error
}

// Bridge connects NOFXi to NOFX trading engine.
type Bridge struct {
	mu      sync.RWMutex
	traders map[string]NofxTrader // exchange name → trader
	factory TraderFactory
	logger  *slog.Logger
}

// NewBridge creates a new execution bridge.
func NewBridge(factory TraderFactory, logger *slog.Logger) *Bridge {
	return &Bridge{
		traders: make(map[string]NofxTrader),
		factory: factory,
		logger:  logger,
	}
}

// RegisterTrader registers a pre-configured trader for an exchange.
func (b *Bridge) RegisterTrader(exchange string, trader NofxTrader) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.traders[strings.ToLower(exchange)] = trader
	b.logger.Info("trader registered", "exchange", exchange)
}

// getTrader returns the trader for the given exchange.
func (b *Bridge) getTrader(exchange string) (NofxTrader, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	t, ok := b.traders[strings.ToLower(exchange)]
	if !ok {
		return nil, fmt.Errorf("exchange %q not configured", exchange)
	}
	return t, nil
}

// PlaceOrder executes a trade via the NOFX trader.
func (b *Bridge) PlaceOrder(exchange, symbol, side string, quantity float64, leverage int) (map[string]interface{}, error) {
	trader, err := b.getTrader(exchange)
	if err != nil {
		return nil, err
	}

	// Set leverage first
	if leverage > 0 {
		if err := trader.SetLeverage(symbol, leverage); err != nil {
			b.logger.Warn("set leverage failed", "error", err)
		}
	}

	side = strings.ToUpper(side)
	switch side {
	case "BUY", "LONG", "OPEN_LONG":
		b.logger.Info("opening long", "exchange", exchange, "symbol", symbol, "qty", quantity, "leverage", leverage)
		return trader.OpenLong(symbol, quantity, leverage)
	case "SELL", "SHORT", "OPEN_SHORT":
		b.logger.Info("opening short", "exchange", exchange, "symbol", symbol, "qty", quantity, "leverage", leverage)
		return trader.OpenShort(symbol, quantity, leverage)
	case "CLOSE_LONG":
		b.logger.Info("closing long", "exchange", exchange, "symbol", symbol, "qty", quantity)
		return trader.CloseLong(symbol, quantity)
	case "CLOSE_SHORT":
		b.logger.Info("closing short", "exchange", exchange, "symbol", symbol, "qty", quantity)
		return trader.CloseShort(symbol, quantity)
	default:
		return nil, fmt.Errorf("unknown side: %s", side)
	}
}

// GetPositions returns all open positions from an exchange.
func (b *Bridge) GetPositions(exchange string) ([]Position, error) {
	trader, err := b.getTrader(exchange)
	if err != nil {
		return nil, err
	}

	raw, err := trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("get positions: %w", err)
	}

	var positions []Position
	for _, p := range raw {
		pos := Position{
			Exchange: exchange,
			Symbol:   fmt.Sprint(p["symbol"]),
			Side:     fmt.Sprint(p["side"]),
		}
		if v, ok := p["size"].(float64); ok {
			pos.Size = v
		}
		if v, ok := p["entryPrice"].(float64); ok {
			pos.EntryPrice = v
		}
		if v, ok := p["markPrice"].(float64); ok {
			pos.MarkPrice = v
		}
		if v, ok := p["unrealizedPnl"].(float64); ok {
			pos.PnL = v
		}
		if v, ok := p["leverage"].(float64); ok {
			pos.Leverage = v
		}
		// Skip empty positions
		if pos.Size != 0 {
			positions = append(positions, pos)
		}
	}
	return positions, nil
}

// GetBalance returns account balance from an exchange.
func (b *Bridge) GetBalance(exchange string) (*Balance, error) {
	trader, err := b.getTrader(exchange)
	if err != nil {
		return nil, err
	}

	raw, err := trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}

	bal := &Balance{
		Exchange: exchange,
		Currency: "USDT",
	}
	if v, ok := raw["totalBalance"].(float64); ok {
		bal.Total = v
	}
	if v, ok := raw["availableBalance"].(float64); ok {
		bal.Available = v
	}
	bal.InPosition = bal.Total - bal.Available
	return bal, nil
}

// GetPrice returns the current market price for a symbol.
func (b *Bridge) GetPrice(exchange, symbol string) (float64, error) {
	trader, err := b.getTrader(exchange)
	if err != nil {
		return 0, err
	}
	return trader.GetMarketPrice(symbol)
}

// SetStopLoss sets a stop-loss order.
func (b *Bridge) SetStopLoss(exchange, symbol, positionSide string, quantity, price float64) error {
	trader, err := b.getTrader(exchange)
	if err != nil {
		return err
	}
	return trader.SetStopLoss(symbol, positionSide, quantity, price)
}

// SetTakeProfit sets a take-profit order.
func (b *Bridge) SetTakeProfit(exchange, symbol, positionSide string, quantity, price float64) error {
	trader, err := b.getTrader(exchange)
	if err != nil {
		return err
	}
	return trader.SetTakeProfit(symbol, positionSide, quantity, price)
}
