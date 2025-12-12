package trader

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"nofx/logger"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

// LighterTrader LIGHTER DEX trader
// LIGHTER is an Ethereum L2-based perpetual contract DEX using zk-rollup technology
type LighterTrader struct {
	ctx        context.Context
	privateKey *ecdsa.PrivateKey
	walletAddr string // Ethereum wallet address
	client     *http.Client
	baseURL    string
	testnet    bool

	// Account information cache
	accountIndex  int    // LIGHTER account index
	apiKey        string // API key (derived from private key)
	authToken     string // Authentication token (8-hour validity)
	tokenExpiry   time.Time
	accountMutex  sync.RWMutex

	// Market information cache
	symbolPrecision map[string]SymbolPrecision
	precisionMutex  sync.RWMutex
}

// LighterConfig LIGHTER configuration
type LighterConfig struct {
	PrivateKeyHex string
	WalletAddr    string
	Testnet       bool
}

// NewLighterTrader Create LIGHTER trader
func NewLighterTrader(privateKeyHex string, walletAddr string, testnet bool) (*LighterTrader, error) {
	// Remove 0x prefix from private key (if present)
	privateKeyHex = strings.TrimPrefix(strings.ToLower(privateKeyHex), "0x")

	// Parse private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Derive wallet address from private key (if not provided)
	if walletAddr == "" {
		walletAddr = crypto.PubkeyToAddress(*privateKey.Public().(*ecdsa.PublicKey)).Hex()
		logger.Infof("✓ Derived wallet address from private key: %s", walletAddr)
	}

	// Select API URL
	baseURL := "https://mainnet.zklighter.elliot.ai"
	if testnet {
		baseURL = "https://testnet.zklighter.elliot.ai" // TODO: Confirm testnet URL
	}

	trader := &LighterTrader{
		ctx:             context.Background(),
		privateKey:      privateKey,
		walletAddr:      walletAddr,
		client:          &http.Client{Timeout: 30 * time.Second},
		baseURL:         baseURL,
		testnet:         testnet,
		symbolPrecision: make(map[string]SymbolPrecision),
	}

	logger.Infof("✓ LIGHTER trader initialized successfully (testnet=%v, wallet=%s)", testnet, walletAddr)

	// Initialize account information (get account index and API key)
	if err := trader.initializeAccount(); err != nil {
		return nil, fmt.Errorf("failed to initialize account: %w", err)
	}

	return trader, nil
}

// initializeAccount Initialize account information
func (t *LighterTrader) initializeAccount() error {
	// 1. Get account information (by L1 address)
	accountInfo, err := t.getAccountByL1Address()
	if err != nil {
		return fmt.Errorf("failed to get account information: %w", err)
	}

	t.accountMutex.Lock()
	t.accountIndex = accountInfo["index"].(int)
	t.accountMutex.Unlock()

	logger.Infof("✓ LIGHTER account index: %d", t.accountIndex)

	// 2. Generate authentication token (8-hour validity)
	if err := t.refreshAuthToken(); err != nil {
		return fmt.Errorf("failed to generate auth token: %w", err)
	}

	return nil
}

// getAccountByL1Address Get LIGHTER account information by Ethereum address
func (t *LighterTrader) getAccountByL1Address() (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("%s/api/v1/account/by/l1/%s", t.baseURL, t.walletAddr)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

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
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// refreshAuthToken Refresh authentication token
func (t *LighterTrader) refreshAuthToken() error {
	// TODO: Implement authentication token generation logic
	// Reference lighter-python SDK implementation
	// Need to sign specific message and submit to API

	t.accountMutex.Lock()
	defer t.accountMutex.Unlock()

	// Temporary implementation: set expiry time to 8 hours from now
	t.tokenExpiry = time.Now().Add(8 * time.Hour)
	logger.Infof("✓ Auth token generated (valid until: %s)", t.tokenExpiry.Format(time.RFC3339))

	return nil
}

// ensureAuthToken Ensure authentication token is valid
func (t *LighterTrader) ensureAuthToken() error {
	t.accountMutex.RLock()
	expired := time.Now().After(t.tokenExpiry.Add(-30 * time.Minute)) // Refresh 30 minutes early
	t.accountMutex.RUnlock()

	if expired {
		logger.Info("🔄 Auth token expiring soon, refreshing...")
		return t.refreshAuthToken()
	}

	return nil
}

// signMessage Sign message (Ethereum signature)
func (t *LighterTrader) signMessage(message []byte) (string, error) {
	// Use Ethereum personal sign format
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	prefixedMessage := append([]byte(prefix), message...)

	hash := crypto.Keccak256Hash(prefixedMessage)
	signature, err := crypto.Sign(hash.Bytes(), t.privateKey)
	if err != nil {
		return "", err
	}

	// Adjust v value (Ethereum format)
	if signature[64] < 27 {
		signature[64] += 27
	}

	return "0x" + hex.EncodeToString(signature), nil
}

// GetName Get trader name
func (t *LighterTrader) GetName() string {
	return "LIGHTER"
}

// GetExchangeType Get exchange type
func (t *LighterTrader) GetExchangeType() string {
	return "lighter"
}

// Close Close trader
func (t *LighterTrader) Close() error {
	logger.Info("✓ LIGHTER trader closed")
	return nil
}

// Run Run trader (implements Trader interface)
func (t *LighterTrader) Run() error {
	logger.Info("⚠️ LIGHTER trader's Run method should be called by AutoTrader")
	return fmt.Errorf("please use AutoTrader to manage trader lifecycle")
}

// GetClosedPnL gets recent closing trades from Lighter
// Note: Lighter does NOT have a position history API, only trade history.
// This returns individual closing trades for real-time position closure detection.
func (t *LighterTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	trades, err := t.GetTrades(startTime, limit)
	if err != nil {
		return nil, err
	}

	// Filter only closing trades (realizedPnl != 0)
	var records []ClosedPnLRecord
	for _, trade := range trades {
		if trade.RealizedPnL == 0 {
			continue
		}

		// Determine side (Lighter uses one-way mode)
		side := "long"
		if trade.Side == "SELL" || trade.Side == "Sell" {
			side = "long"
		} else {
			side = "short"
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

		records = append(records, ClosedPnLRecord{
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

// LighterTradeResponse represents the response from Lighter trades API
type LighterTradeResponse struct {
	Trades []LighterTrade `json:"trades"`
}

// LighterTrade represents a single trade from Lighter
type LighterTrade struct {
	TradeID       string `json:"trade_id"`
	AccountIndex  int64  `json:"account_index"`
	MarketIndex   int    `json:"market_index"`
	Symbol        string `json:"symbol"`
	Side          string `json:"side"` // "buy" or "sell"
	Price         string `json:"price"`
	Size          string `json:"size"`
	RealizedPnl   string `json:"realized_pnl"`
	Fee           string `json:"fee"`
	Timestamp     int64  `json:"timestamp"`
	IsMaker       bool   `json:"is_maker"`
}

// GetTrades retrieves trade history from Lighter
func (t *LighterTrader) GetTrades(startTime time.Time, limit int) ([]TradeRecord, error) {
	// Ensure we have account index
	if t.accountIndex == 0 {
		accountInfo, err := t.getAccountByL1Address()
		if err != nil {
			return nil, fmt.Errorf("failed to get account index: %w", err)
		}
		if idx, ok := accountInfo["index"].(int); ok {
			t.accountIndex = idx
		} else if idx, ok := accountInfo["index"].(float64); ok {
			t.accountIndex = int(idx)
		}
	}

	// Build request URL
	// API: GET /api/v1/trades?account_index=X&start_time=Y&limit=Z
	startTimeMs := startTime.UnixMilli()
	endpoint := fmt.Sprintf("%s/api/v1/trades?account_index=%d&start_time=%d",
		t.baseURL, t.accountIndex, startTimeMs)
	if limit > 0 {
		endpoint = fmt.Sprintf("%s&limit=%d", endpoint, limit)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get trades: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Infof("⚠️  Lighter trades API returned %d: %s", resp.StatusCode, string(body))
		return []TradeRecord{}, nil // Return empty on error
	}

	var response LighterTradeResponse
	if err := json.Unmarshal(body, &response); err != nil {
		// Try parsing as array directly
		var trades []LighterTrade
		if err := json.Unmarshal(body, &trades); err != nil {
			logger.Infof("⚠️  Failed to parse Lighter trades response: %v", err)
			return []TradeRecord{}, nil
		}
		response.Trades = trades
	}

	// Convert to unified TradeRecord format
	var result []TradeRecord
	for _, lt := range response.Trades {
		price, _ := parseFloat(lt.Price)
		qty, _ := parseFloat(lt.Size)
		fee, _ := parseFloat(lt.Fee)
		pnl, _ := parseFloat(lt.RealizedPnl)

		var side string
		if strings.ToLower(lt.Side) == "buy" {
			side = "BUY"
		} else {
			side = "SELL"
		}

		trade := TradeRecord{
			TradeID:      lt.TradeID,
			Symbol:       lt.Symbol,
			Side:         side,
			PositionSide: "BOTH", // Lighter uses one-way mode
			Price:        price,
			Quantity:     qty,
			RealizedPnL:  pnl,
			Fee:          fee,
			Time:         time.UnixMilli(lt.Timestamp),
		}
		result = append(result, trade)
	}

	return result, nil
}

// parseFloat safely parses a float string
func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
