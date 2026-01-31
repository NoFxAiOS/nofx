package lighter

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/sha3"
)

// SymbolPrecision Symbol precision information
type SymbolPrecision struct {
	PricePrecision    int
	QuantityPrecision int
	TickSize          float64 // Price tick size
	StepSize          float64 // Quantity step size
}

// AccountBalance Account balance information (Lighter)
type AccountBalance struct {
	TotalEquity       float64 `json:"total_equity"`       // Total equity
	AvailableBalance  float64 `json:"available_balance"`  // Available balance
	MarginUsed        float64 `json:"margin_used"`        // Used margin
	UnrealizedPnL     float64 `json:"unrealized_pnl"`     // Unrealized PnL
	MaintenanceMargin float64 `json:"maintenance_margin"` // Maintenance margin
}

// Position Position information (Lighter)
type Position struct {
	Symbol           string  `json:"symbol"`            // Trading pair
	Side             string  `json:"side"`              // "long" or "short"
	Size             float64 `json:"size"`              // Position size
	EntryPrice       float64 `json:"entry_price"`       // Average entry price
	MarkPrice        float64 `json:"mark_price"`        // Mark price
	LiquidationPrice float64 `json:"liquidation_price"` // Liquidation price
	UnrealizedPnL    float64 `json:"unrealized_pnl"`    // Unrealized PnL
	Leverage         float64 `json:"leverage"`          // Leverage multiplier
	MarginUsed       float64 `json:"margin_used"`       // Used margin
}

// CreateOrderRequest Create order request (Lighter)
type CreateOrderRequest struct {
	Symbol      string  `json:"symbol"`        // Trading pair
	Side        string  `json:"side"`          // "buy" or "sell"
	OrderType   string  `json:"order_type"`    // "market" or "limit"
	Quantity    float64 `json:"quantity"`      // Quantity
	Price       float64 `json:"price"`         // Price (required for limit orders)
	ReduceOnly  bool    `json:"reduce_only"`   // Reduce-only flag
	TimeInForce string  `json:"time_in_force"` // "GTC", "IOC", "FOK"
	PostOnly    bool    `json:"post_only"`     // Post-only (maker only)
}

// OrderResponse Order response (Lighter API)
// Field names must match Lighter API response exactly
type OrderResponse struct {
	OrderID             string `json:"order_id"`
	OrderIndex          int64  `json:"order_index"`
	MarketIndex         int    `json:"market_index"`
	Side                string `json:"side"`                  // "bid" or "ask"
	Type                string `json:"type"`                  // "limit", "market", etc.
	IsAsk               bool   `json:"is_ask"`                // true = sell, false = buy
	Price               string `json:"price"`                 // Price as string
	InitialBaseAmount   string `json:"initial_base_amount"`   // Original quantity
	RemainingBaseAmount string `json:"remaining_base_amount"` // Remaining quantity
	FilledBaseAmount    string `json:"filled_base_amount"`    // Filled quantity
	Status              string `json:"status"`                // "open", "filled", "cancelled"
	TriggerPrice        string `json:"trigger_price"`         // For stop orders
	ReduceOnly          bool   `json:"reduce_only"`
	Timestamp           int64  `json:"timestamp"`
	CreatedAt           int64  `json:"created_at"`
}

// LighterTradeResponse represents the response from Lighter trades API
type LighterTradeResponse struct {
	Code       int            `json:"code"`
	NextCursor string         `json:"next_cursor,omitempty"`
	Trades     []LighterTrade `json:"trades"`
}

// LighterTrade represents a single trade from Lighter API
// API docs: https://apidocs.lighter.xyz/reference/trades
type LighterTrade struct {
	TradeID      int64  `json:"trade_id"`
	TxHash       string `json:"tx_hash"`
	Type         string `json:"type"`      // "trade", "liquidation", etc
	MarketID     int    `json:"market_id"` // Need to convert to symbol
	Size         string `json:"size"`
	Price        string `json:"price"`
	UsdAmount    string `json:"usd_amount"`
	AskID        int64  `json:"ask_id"`
	BidID        int64  `json:"bid_id"`
	AskAccountID int64  `json:"ask_account_id"`
	BidAccountID int64  `json:"bid_account_id"`
	IsMakerAsk   bool   `json:"is_maker_ask"`
	BlockHeight  int64  `json:"block_height"`
	Timestamp    int64  `json:"timestamp"`
	TakerFee     int64 `json:"taker_fee,omitempty"`
	MakerFee     int64 `json:"maker_fee,omitempty"`

	// Position change information - critical for determining open/close
	TakerPositionSizeBefore    string `json:"taker_position_size_before"`
	TakerPositionSignChanged   bool   `json:"taker_position_sign_changed"`
	MakerPositionSizeBefore    string `json:"maker_position_size_before"`
	MakerPositionSignChanged   bool   `json:"maker_position_sign_changed,omitempty"`
}

// parseFloat parses a string to float64, returns 0 for empty string
func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// ToChecksumAddress converts an Ethereum address to EIP-55 checksum format
// This is required for Lighter API which is case-sensitive for addresses
func ToChecksumAddress(address string) string {
	// Remove 0x prefix and convert to lowercase
	addr := strings.ToLower(strings.TrimPrefix(address, "0x"))
	if len(addr) != 40 {
		return address // Return original if invalid length
	}

	// Compute Keccak-256 hash of the lowercase address
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write([]byte(addr))
	hash := hasher.Sum(nil)

	// Build checksum address
	var result strings.Builder
	result.WriteString("0x")

	for i, c := range addr {
		// Get the corresponding nibble from the hash
		// Each byte in hash contains 2 nibbles (4 bits each)
		hashByte := hash[i/2]
		var nibble byte
		if i%2 == 0 {
			nibble = hashByte >> 4 // High nibble
		} else {
			nibble = hashByte & 0x0F // Low nibble
		}

		// If nibble >= 8, uppercase the character (if it's a letter)
		if nibble >= 8 && c >= 'a' && c <= 'f' {
			result.WriteByte(byte(c) - 32) // Convert to uppercase
		} else {
			result.WriteByte(byte(c))
		}
	}

	return result.String()
}
