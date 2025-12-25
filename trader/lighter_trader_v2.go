package trader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"strings"
	"sync"
	"time"

	lighterClient "github.com/elliottech/lighter-go/client"
	lighterHTTP "github.com/elliottech/lighter-go/client/http"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// AccountInfo LIGHTER account information
type AccountInfo struct {
	AccountIndex     int64   `json:"account_index"`
	Index            int64   `json:"index"` // Same as account_index
	L1Address        string  `json:"l1_address"`
	AvailableBalance string  `json:"available_balance"`
	Collateral       string  `json:"collateral"`
	CrossAssetValue  string  `json:"cross_asset_value"`
	TotalEquity      string  `json:"total_equity"`
	UnrealizedPnl    string  `json:"unrealized_pnl"`
	Positions        []LighterPositionInfo `json:"positions"`
}

// LighterPositionInfo Position info from Lighter account API
type LighterPositionInfo struct {
	MarketID              int     `json:"market_id"`
	Symbol                string  `json:"symbol"`
	Sign                  int     `json:"sign"`                    // 1 = long, -1 = short
	Position              string  `json:"position"`                // Position size
	AvgEntryPrice         string  `json:"avg_entry_price"`         // Entry price
	PositionValue         string  `json:"position_value"`          // Position value in USD
	LiquidationPrice      string  `json:"liquidation_price"`
	UnrealizedPnl         string  `json:"unrealized_pnl"`
	RealizedPnl           string  `json:"realized_pnl"`
	InitialMarginFraction string  `json:"initial_margin_fraction"` // e.g. "5.00" means 5% = 20x leverage
	AllocatedMargin       string  `json:"allocated_margin"`
	MarginMode            int     `json:"margin_mode"`             // 0 = cross, 1 = isolated
}

// AccountResponse LIGHTER account API response
// API may return accounts in "accounts" or "sub_accounts" field
type AccountResponse struct {
	Code        int           `json:"code"`
	Message     string        `json:"message"`
	Accounts    []AccountInfo `json:"accounts"`
	SubAccounts []AccountInfo `json:"sub_accounts"` // Sub-accounts field
}

// LighterTraderV2 New implementation using official lighter-go SDK
type LighterTraderV2 struct {
	ctx        context.Context
	walletAddr string // Ethereum wallet address

	client  *http.Client
	baseURL string
	testnet bool
	chainID uint32

	// SDK clients
	httpClient lighterClient.MinimalHTTPClient
	txClient   *lighterClient.TxClient

	// API Key management
	apiKeyPrivateKey string // 40-byte API Key private key (for signing transactions)
	apiKeyIndex      uint8  // API Key index (default 0)
	accountIndex     int64  // Account index

	// Authentication token
	authToken     string
	tokenExpiry   time.Time
	accountMutex  sync.RWMutex

	// Market info cache
	symbolPrecision map[string]SymbolPrecision
	precisionMutex  sync.RWMutex

	// Market index cache
	marketIndexMap map[string]uint16 // symbol -> market_id
	marketMutex    sync.RWMutex
}

// NewLighterTraderV2 Create new LIGHTER trader (using official SDK)
// Parameters:
//   - walletAddr: Ethereum wallet address (required)
//   - apiKeyPrivateKeyHex: API Key private key (40 bytes, for signing transactions)
//   - apiKeyIndex: API Key index (0-255)
//   - testnet: Whether to use testnet
func NewLighterTraderV2(walletAddr, apiKeyPrivateKeyHex string, apiKeyIndex int, testnet bool) (*LighterTraderV2, error) {
	// 1. Validate wallet address
	if walletAddr == "" {
		return nil, fmt.Errorf("wallet address is required")
	}

	// Convert to checksum address (Lighter API is case-sensitive)
	walletAddr = ToChecksumAddress(walletAddr)
	logger.Infof("Using checksum address: %s", walletAddr)

	// 2. Validate API Key
	if apiKeyPrivateKeyHex == "" {
		return nil, fmt.Errorf("API Key private key is required")
	}

	// 3. Determine API URL and Chain ID
	// Note: Python SDK uses 304 for mainnet, 300 for testnet (not the L1 chain IDs)
	baseURL := "https://mainnet.zklighter.elliot.ai"
	chainID := uint32(304) // Mainnet Lighter Chain ID (from Python SDK)
	if testnet {
		baseURL = "https://testnet.zklighter.elliot.ai"
		chainID = uint32(300) // Testnet Lighter Chain ID (from Python SDK)
	}

	// 4. Create HTTP client
	httpClient := lighterHTTP.NewClient(baseURL)

	trader := &LighterTraderV2{
		ctx:              context.Background(),
		walletAddr:       walletAddr,
		client:           &http.Client{Timeout: 30 * time.Second},
		baseURL:          baseURL,
		testnet:          testnet,
		chainID:          chainID,
		httpClient:       httpClient,
		apiKeyPrivateKey: apiKeyPrivateKeyHex,
		apiKeyIndex:      uint8(apiKeyIndex),
		symbolPrecision:  make(map[string]SymbolPrecision),
		marketIndexMap:   make(map[string]uint16),
	}

	// 5. Initialize account (get account index)
	if err := trader.initializeAccount(); err != nil {
		return nil, fmt.Errorf("failed to initialize account: %w", err)
	}

	// 6. Create TxClient (for signing transactions)
	txClient, err := lighterClient.NewTxClient(
		httpClient,
		apiKeyPrivateKeyHex,
		trader.accountIndex,
		trader.apiKeyIndex,
		trader.chainID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create TxClient: %w", err)
	}

	trader.txClient = txClient

	// 7. Verify API Key is correct
	if err := trader.checkClient(); err != nil {
		logger.Warnf("⚠️  API Key verification failed: %v", err)
		// Don't fail here, allow trader to continue (may work with some operations)
	}

	logger.Infof("✓ LIGHTER trader initialized successfully (account=%d, apiKey=%d, testnet=%v)",
		trader.accountIndex, trader.apiKeyIndex, testnet)

	return trader, nil
}

// initializeAccount Initialize account information (get account index)
func (t *LighterTraderV2) initializeAccount() error {
	// Get account info by L1 address
	accountInfo, err := t.getAccountByL1Address()
	if err != nil {
		return fmt.Errorf("failed to get account info: %w", err)
	}

	t.accountMutex.Lock()
	t.accountIndex = accountInfo.AccountIndex
	t.accountMutex.Unlock()

	logger.Infof("✓ Account index: %d", t.accountIndex)
	return nil
}

// getAccountByL1Address Get LIGHTER account info by L1 wallet address
// Supports both main accounts and sub-accounts
func (t *LighterTraderV2) getAccountByL1Address() (*AccountInfo, error) {
	endpoint := fmt.Sprintf("%s/api/v1/account?by=l1_address&value=%s", t.baseURL, t.walletAddr)

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

	// Log raw response for debugging
	logger.Infof("LIGHTER account API response: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get account (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response - Lighter may return accounts in "accounts" or "sub_accounts"
	var accountResp AccountResponse
	if err := json.Unmarshal(body, &accountResp); err != nil {
		return nil, fmt.Errorf("failed to parse account response: %w", err)
	}

	// Check for API error
	if accountResp.Code != 0 && accountResp.Code != 200 {
		return nil, fmt.Errorf("Lighter API error (code %d): %s", accountResp.Code, accountResp.Message)
	}

	// Try accounts first, then sub_accounts
	var allAccounts []AccountInfo
	allAccounts = append(allAccounts, accountResp.Accounts...)
	allAccounts = append(allAccounts, accountResp.SubAccounts...)

	if len(allAccounts) == 0 {
		return nil, fmt.Errorf("no account found for wallet address: %s (try depositing funds first at app.lighter.xyz)", t.walletAddr)
	}

	// Log all found accounts
	logger.Infof("Found %d accounts (main: %d, sub: %d)", len(allAccounts), len(accountResp.Accounts), len(accountResp.SubAccounts))
	for i, acc := range allAccounts {
		logger.Infof("  Account[%d]: index=%d, collateral=%s", i, acc.AccountIndex, acc.Collateral)
	}

	account := &allAccounts[0]
	// Use index field if account_index is 0
	if account.AccountIndex == 0 && account.Index != 0 {
		account.AccountIndex = account.Index
	}

	return account, nil
}

// checkClient Verify if API Key is correct
func (t *LighterTraderV2) checkClient() error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	// Get API Key public key registered on server
	publicKey, err := t.httpClient.GetApiKey(t.accountIndex, t.apiKeyIndex)
	if err != nil {
		return fmt.Errorf("failed to get API Key: %w", err)
	}

	// Get local API Key public key
	pubKeyBytes := t.txClient.GetKeyManager().PubKeyBytes()
	localPubKey := hexutil.Encode(pubKeyBytes[:])
	localPubKey = strings.Replace(localPubKey, "0x", "", 1)

	// Compare public keys
	if publicKey != localPubKey {
		return fmt.Errorf("API Key mismatch: local=%s, server=%s", localPubKey, publicKey)
	}

	logger.Infof("✓ API Key verification passed")
	return nil
}

// GenerateAndRegisterAPIKey Generate new API Key and register to LIGHTER
// Note: This requires L1 private key signature, so must be called with L1 private key available
func (t *LighterTraderV2) GenerateAndRegisterAPIKey(seed string) (privateKey, publicKey string, err error) {
	// This function needs to call the official SDK's GenerateAPIKey function
	// But this is a CGO function in sharedlib, cannot be called directly in pure Go code
	//
	// Solutions:
	// 1. Let users generate API Key from LIGHTER website
	// 2. Or we can implement a simple API Key generation wrapper

	return "", "", fmt.Errorf("GenerateAndRegisterAPIKey feature not implemented yet, please generate API Key from LIGHTER website")
}

// refreshAuthToken Refresh authentication token (using official SDK)
func (t *LighterTraderV2) refreshAuthToken() error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized, please set API Key first")
	}

	// Generate auth token using official SDK (valid for 7 hours)
	deadline := time.Now().Add(7 * time.Hour)
	authToken, err := t.txClient.GetAuthToken(deadline)
	if err != nil {
		return fmt.Errorf("failed to generate auth token: %w", err)
	}

	t.accountMutex.Lock()
	t.authToken = authToken
	t.tokenExpiry = deadline
	t.accountMutex.Unlock()

	logger.Infof("✓ Auth token generated (valid until: %s)", t.tokenExpiry.Format(time.RFC3339))
	return nil
}

// ensureAuthToken Ensure authentication token is valid
func (t *LighterTraderV2) ensureAuthToken() error {
	t.accountMutex.RLock()
	expired := time.Now().After(t.tokenExpiry.Add(-30 * time.Minute)) // Refresh 30 minutes early
	t.accountMutex.RUnlock()

	if expired {
		logger.Info("🔄 Auth token about to expire, refreshing...")
		return t.refreshAuthToken()
	}

	return nil
}

// GetExchangeType Get exchange type
func (t *LighterTraderV2) GetExchangeType() string {
	return "lighter"
}

// Cleanup Clean up resources
func (t *LighterTraderV2) Cleanup() error {
	logger.Info("⏹  LIGHTER trader cleanup completed")
	return nil
}

// GetClosedPnL gets closed position PnL records from exchange
// LIGHTER does not have a direct closed PnL API, returns empty slice
func (t *LighterTraderV2) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
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

		side := "long"
		if trade.Side == "SELL" || trade.Side == "Sell" {
			side = "long"
		} else {
			side = "short"
		}

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

// GetTrades retrieves trade history from Lighter
func (t *LighterTraderV2) GetTrades(startTime time.Time, limit int) ([]TradeRecord, error) {
	// Ensure we have account index
	if t.accountIndex == 0 {
		if err := t.initializeAccount(); err != nil {
			return nil, fmt.Errorf("failed to get account index: %w", err)
		}
	}

	// Build request URL (use Unix timestamp in seconds, not milliseconds)
	startTimeSec := startTime.Unix()
	endpoint := fmt.Sprintf("%s/api/v1/trades?account_index=%d&start_time=%d",
		t.baseURL, t.accountIndex, startTimeSec)
	if limit > 0 {
		endpoint = fmt.Sprintf("%s&limit=%d", endpoint, limit)
	}

	logger.Infof("🔍 Calling Lighter GetTrades API: %s", endpoint)

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
		return []TradeRecord{}, nil
	}

	var response LighterTradeResponse
	if err := json.Unmarshal(body, &response); err != nil {
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
			PositionSide: "BOTH",
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
