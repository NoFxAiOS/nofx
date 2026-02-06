package lighter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"nofx/logger"
	"strings"
	"sync"
	"time"

	lighterClient "github.com/elliottech/lighter-go/client"
	lighterHTTP "github.com/elliottech/lighter-go/client/http"
	"github.com/ethereum/go-ethereum/common/hexutil"
	tradertypes "nofx/trader/types"
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
	apiKeyValid      bool   // Whether API key has been validated against server

	// Authentication token
	authToken     string
	tokenExpiry   time.Time
	accountMutex  sync.RWMutex

	// Market info cache
	symbolPrecision map[string]SymbolPrecision
	precisionMutex  sync.RWMutex

	// Market index cache
	marketIndexMap      map[string]uint16 // symbol -> market_id
	marketMutex         sync.RWMutex
	marketListCache     []MarketInfo // Cached market list
	marketListCacheTime time.Time    // Time when cache was populated
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
		ctx:        context.Background(),
		walletAddr: walletAddr,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
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
		trader.apiKeyValid = false
		logger.Warnf("⚠️  API Key verification FAILED: %v", err)
		logger.Warnf("⚠️  ❌ The API key stored in NOFX does NOT match the API key registered on Lighter.")
		logger.Warnf("⚠️  ❌ ALL trading operations (open/close positions, cancel orders) WILL FAIL with 'invalid signature' error.")
		logger.Warnf("⚠️  🔧 To fix: Update your Lighter API key in NOFX Exchange settings with the correct key from app.lighter.xyz")
		// Don't fail here, allow trader to continue for read operations (balance, positions)
	} else {
		trader.apiKeyValid = true
	}

	logger.Infof("✓ LIGHTER trader initialized (account=%d, apiKey=%d, testnet=%v, apiKeyValid=%v)",
		trader.accountIndex, trader.apiKeyIndex, testnet, trader.apiKeyValid)

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
	logger.Debugf("LIGHTER account API response: %s", string(body))

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

	// Log account summary
	logger.Infof("Found %d account(s) (main: %d, sub: %d)", len(allAccounts), len(accountResp.Accounts), len(accountResp.SubAccounts))
	for i, acc := range allAccounts {
		logger.Debugf("  Account[%d]: index=%d, collateral=%s", i, acc.AccountIndex, acc.Collateral)
	}

	account := &allAccounts[0]
	// Use index field if account_index is 0
	if account.AccountIndex == 0 && account.Index != 0 {
		account.AccountIndex = account.Index
	}

	return account, nil
}

// ApiKeyResponse API key query response
type ApiKeyResponse struct {
	Code    int `json:"code"`
	ApiKeys []struct {
		AccountIndex int64  `json:"account_index"`
		ApiKeyIndex  uint8  `json:"api_key_index"`
		Nonce        int64  `json:"nonce"`
		PublicKey    string `json:"public_key"`
	} `json:"api_keys"`
}

// getApiKeyFromServer Get API Key public key from Lighter server
// Uses our own HTTP client instead of SDK's global client to avoid connection issues
func (t *LighterTraderV2) getApiKeyFromServer() (string, error) {
	endpoint := fmt.Sprintf("%s/api/v1/apikeys?account_index=%d&api_key_index=%d",
		t.baseURL, t.accountIndex, t.apiKeyIndex)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result ApiKeyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Code != 200 {
		return "", fmt.Errorf("API error (code %d)", result.Code)
	}

	if len(result.ApiKeys) == 0 {
		return "", fmt.Errorf("no API keys found for account %d", t.accountIndex)
	}

	return result.ApiKeys[0].PublicKey, nil
}

// checkClient Verify if API Key is correct
func (t *LighterTraderV2) checkClient() error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	// Get API Key public key registered on server (using our own HTTP client)
	serverPubKey, err := t.getApiKeyFromServer()
	if err != nil {
		return fmt.Errorf("failed to get API Key: %w", err)
	}

	// Get local API Key public key from SDK
	pubKeyBytes := t.txClient.GetKeyManager().PubKeyBytes()
	localPubKey := hexutil.Encode(pubKeyBytes[:])
	localPubKey = strings.TrimPrefix(localPubKey, "0x")

	// Compare public keys
	if serverPubKey != localPubKey {
		return fmt.Errorf("API Key mismatch: local=%s, server=%s", localPubKey, serverPubKey)
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
func (t *LighterTraderV2) GetClosedPnL(startTime time.Time, limit int) ([]tradertypes.ClosedPnLRecord, error) {
	trades, err := t.GetTrades(startTime, limit)
	if err != nil {
		return nil, err
	}

	// Filter only closing trades (realizedPnl != 0)
	var records []tradertypes.ClosedPnLRecord
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

		records = append(records, tradertypes.ClosedPnLRecord{
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
func (t *LighterTraderV2) GetTrades(startTime time.Time, limit int) ([]tradertypes.TradeRecord, error) {
	// Ensure we have account index
	if t.accountIndex == 0 {
		if err := t.initializeAccount(); err != nil {
			return nil, fmt.Errorf("failed to get account index: %w", err)
		}
	}

	// Build request URL with correct parameters
	// Required: sort_by, limit
	// Optional: account_index, from (timestamp in milliseconds, -1 for no filter)
	// Note: OpenAPI spec uses "from" not "var_from"
	// Authentication: Use "auth" query parameter (not Authorization header)
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("failed to get auth token: %w", err)
	}

	// URL encode auth token (contains colons that need encoding)
	encodedAuth := url.QueryEscape(t.authToken)
	// Build endpoint - use from=-1 to get all trades (no time filter)
	endpoint := fmt.Sprintf("%s/api/v1/trades?account_index=%d&sort_by=timestamp&sort_dir=desc&limit=%d&auth=%s",
		t.baseURL, t.accountIndex, limit, encodedAuth)

	logger.Infof("🔍 Calling Lighter GetTrades API: %s", endpoint[:min(len(endpoint), 150)]+"...")

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
		return []tradertypes.TradeRecord{}, nil
	}

	// Debug: log raw response
	logger.Debugf("Lighter trades API response: %s", string(body))

	var response LighterTradeResponse
	if err := json.Unmarshal(body, &response); err != nil {
		logger.Infof("⚠️  Failed to parse trades response as object: %v", err)
		var trades []LighterTrade
		if err := json.Unmarshal(body, &trades); err != nil {
			logger.Infof("⚠️  Failed to parse trades response as array: %v", err)
			return []tradertypes.TradeRecord{}, nil
		}
		response.Trades = trades
	}

	if response.Code != 200 && response.Code != 0 {
		logger.Infof("⚠️  Trades API returned non-success code: %d", response.Code)
		return []tradertypes.TradeRecord{}, nil
	}

	// Build market_id -> symbol map
	marketMap := make(map[int]string)
	markets, err := t.fetchMarketList()
	if err != nil {
		logger.Infof("⚠️  Failed to fetch market list: %v, using fallback", err)
		// Fallback market IDs (common ones)
		marketMap[0] = "BTC"
		marketMap[1] = "ETH"
		marketMap[2] = "SOL"
	} else {
		for _, m := range markets {
			marketMap[int(m.MarketID)] = m.Symbol
		}
	}

	// Convert to unified TradeRecord format
	var result []tradertypes.TradeRecord
	for _, lt := range response.Trades {
		price, _ := parseFloat(lt.Price)
		qty, _ := parseFloat(lt.Size)

		// Calculate fee from taker_fee or maker_fee (they are int64, need conversion)
		var fee float64
		if lt.TakerFee > 0 {
			fee = float64(lt.TakerFee) / 1e6 // Convert from smallest units (6 decimals for USDT)
		} else if lt.MakerFee > 0 {
			fee = float64(lt.MakerFee) / 1e6
		}

		// Get symbol from market_id
		symbol := marketMap[lt.MarketID]
		if symbol == "" {
			symbol = fmt.Sprintf("MARKET%d", lt.MarketID)
		}

		// Determine side based on our account being bid (buyer) or ask (seller)
		// IsMakerAsk: true = ask (seller) is maker, false = bid (buyer) is maker
		var side string
		var isTaker bool
		if lt.BidAccountID == t.accountIndex {
			side = "BUY"
			isTaker = lt.IsMakerAsk // If maker is ask, then we (bid) are taker
		} else if lt.AskAccountID == t.accountIndex {
			side = "SELL"
			isTaker = !lt.IsMakerAsk // If maker is NOT ask, then we (ask) are taker
		} else {
			// Neither bid nor ask is our account - skip this trade
			continue
		}

		// Determine position side and action from position change
		var positionSide, orderAction string
		var posBefore float64
		var signChanged bool

		if isTaker {
			posBefore, _ = parseFloat(lt.TakerPositionSizeBefore)
			signChanged = lt.TakerPositionSignChanged
		} else {
			posBefore, _ = parseFloat(lt.MakerPositionSizeBefore)
			signChanged = lt.MakerPositionSignChanged
		}

		// Determine order action based on:
		// 1. posBefore: position BEFORE this trade (positive=LONG, negative=SHORT, 0=no position)
		// 2. side: BUY or SELL
		// 3. signChanged: whether position flipped direction
		//
		// Logic:
		// - BUY when no position (posBefore ≈ 0): open_long
		// - SELL when no position (posBefore ≈ 0): open_short
		// - BUY when LONG (posBefore > 0): open_long (adding to long)
		// - SELL when LONG (posBefore > 0): close_long (reducing long)
		// - BUY when SHORT (posBefore < 0): close_short (reducing short)
		// - SELL when SHORT (posBefore < 0): open_short (adding to short)
		// - signChanged with position flip: split into close + open

		const EPSILON = 0.0001
		tradeTime := time.UnixMilli(lt.Timestamp).UTC()

		// Calculate position after trade
		var posAfter float64
		if side == "SELL" {
			posAfter = posBefore - qty
		} else {
			posAfter = posBefore + qty
		}

		// Check for position flip (signChanged AND both before/after have meaningful size)
		if signChanged && math.Abs(posBefore) > EPSILON && math.Abs(posAfter) > EPSILON {
			// Position FLIPPED - split into close + open
			closeQty := math.Abs(posBefore)
			openQty := math.Abs(posAfter)

			var closeAction, closeSide, openAction, openSide string
			if posBefore > 0 {
				closeSide, closeAction = "LONG", "close_long"
				openSide, openAction = "SHORT", "open_short"
			} else {
				closeSide, closeAction = "SHORT", "close_short"
				openSide, openAction = "LONG", "open_long"
			}

			closeTrade := tradertypes.TradeRecord{
				TradeID:      fmt.Sprintf("%d_close", lt.TradeID),
				Symbol:       symbol,
				Side:         side,
				PositionSide: closeSide,
				OrderAction:  closeAction,
				Price:        price,
				Quantity:     closeQty,
				RealizedPnL:  0,
				Fee:          fee * (closeQty / qty),
				Time:         tradeTime.Add(-time.Millisecond),
			}
			result = append(result, closeTrade)

			openTrade := tradertypes.TradeRecord{
				TradeID:      fmt.Sprintf("%d_open", lt.TradeID),
				Symbol:       symbol,
				Side:         side,
				PositionSide: openSide,
				OrderAction:  openAction,
				Price:        price,
				Quantity:     openQty,
				RealizedPnL:  0,
				Fee:          fee * (openQty / qty),
				Time:         tradeTime,
			}
			result = append(result, openTrade)

			logger.Infof("  🔄 Flip: %s %.4f → %s %.4f", closeSide, closeQty, openSide, openQty)
			continue
		}

		// Determine action based on position direction and trade side
		if math.Abs(posBefore) < EPSILON {
			// No position before → opening new position
			if side == "BUY" {
				positionSide, orderAction = "LONG", "open_long"
			} else {
				positionSide, orderAction = "SHORT", "open_short"
			}
		} else if posBefore > 0 {
			// Was LONG
			if side == "BUY" {
				positionSide, orderAction = "LONG", "open_long" // Adding to long
			} else {
				positionSide, orderAction = "LONG", "close_long" // Reducing long
			}
		} else {
			// Was SHORT (posBefore < 0)
			if side == "BUY" {
				positionSide, orderAction = "SHORT", "close_short" // Reducing short
			} else {
				positionSide, orderAction = "SHORT", "open_short" // Adding to short
			}
		}

		trade := tradertypes.TradeRecord{
			TradeID:      fmt.Sprintf("%d", lt.TradeID),
			Symbol:       symbol,
			Side:         side,
			PositionSide: positionSide,
			OrderAction:  orderAction,
			Price:        price,
			Quantity:     qty,
			RealizedPnL:  0, // Not available in API
			Fee:          fee,
			Time:         time.UnixMilli(lt.Timestamp).UTC(),
		}
		result = append(result, trade)
	}

	return result, nil
}
