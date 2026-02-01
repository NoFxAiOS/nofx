package hyperliquid

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sonirico/go-hyperliquid"
	"nofx/trader/types"
)

// HyperliquidTrader Hyperliquid trader
type HyperliquidTrader struct {
	exchange      *hyperliquid.Exchange
	ctx           context.Context
	walletAddr    string
	meta          *hyperliquid.Meta // Cache meta information (including precision)
	metaMutex     sync.RWMutex      // Protect concurrent access to meta field
	isCrossMargin bool              // Whether to use cross margin mode
	// xyz dex support (stocks, forex, commodities)
	xyzMeta      *xyzDexMeta
	xyzMetaMutex sync.RWMutex
	privateKey   *ecdsa.PrivateKey // For xyz dex signing
	isTestnet    bool
}

// xyzDexMeta represents metadata for xyz dex assets
type xyzDexMeta struct {
	Universe []xyzAssetInfo `json:"universe"`
}

// xyzAssetInfo represents info for a single xyz dex asset
type xyzAssetInfo struct {
	Name        string `json:"name"`
	SzDecimals  int    `json:"szDecimals"`
	MaxLeverage int    `json:"maxLeverage"`
}

// xyz dex assets (stocks, forex, commodities, index)
// Updated based on actual available assets from xyz dex API
var xyzDexAssets = map[string]bool{
	// Stocks (US equities perpetuals)
	"TSLA": true, "NVDA": true, "AAPL": true, "MSFT": true, "META": true,
	"AMZN": true, "GOOGL": true, "AMD": true, "COIN": true, "NFLX": true,
	"PLTR": true, "HOOD": true, "INTC": true, "MSTR": true, "TSM": true,
	"ORCL": true, "MU": true, "RIVN": true, "COST": true, "LLY": true,
	"CRCL": true, "SKHX": true, "SNDK": true,
	// Forex (currency pairs)
	"EUR": true, "JPY": true,
	// Commodities (precious metals)
	"GOLD": true, "SILVER": true,
	// Index
	"XYZ100": true,
}

// isXyzDexAsset checks if a symbol is an xyz dex asset
func isXyzDexAsset(symbol string) bool {
	// Remove common suffixes to get base symbol
	base := strings.ToUpper(symbol) // Convert to uppercase for case-insensitive matching
	for _, suffix := range []string{"USDT", "USD", "-USDC", "-USD"} {
		if strings.HasSuffix(base, suffix) {
			base = strings.TrimSuffix(base, suffix)
			break
		}
	}
	// Remove xyz: prefix if present (case-insensitive)
	base = strings.TrimPrefix(base, "XYZ:")
	base = strings.TrimPrefix(base, "xyz:")
	return xyzDexAssets[base]
}

// NewHyperliquidTrader creates a Hyperliquid trader
func NewHyperliquidTrader(privateKeyHex string, walletAddr string, testnet bool) (*HyperliquidTrader, error) {
	// Remove 0x prefix from private key (if present, case-insensitive)
	privateKeyHex = strings.TrimPrefix(strings.ToLower(privateKeyHex), "0x")

	// Parse private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Select API URL
	apiURL := hyperliquid.MainnetAPIURL
	if testnet {
		apiURL = hyperliquid.TestnetAPIURL
	}

	// Security enhancement: Implement Agent Wallet best practices
	// Reference: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/nonces-and-api-wallets
	agentAddr := crypto.PubkeyToAddress(*privateKey.Public().(*ecdsa.PublicKey)).Hex()

	if walletAddr == "" {
		return nil, fmt.Errorf("❌ Configuration error: Main wallet address (hyperliquid_wallet_addr) not provided\n" +
			"🔐 Correct configuration pattern:\n" +
			"  1. hyperliquid_private_key = Agent Private Key (for signing only, balance should be ~0)\n" +
			"  2. hyperliquid_wallet_addr = Main Wallet Address (holds funds, never expose private key)\n" +
			"💡 Please create an Agent Wallet on Hyperliquid official website and authorize it before configuration:\n" +
			"   https://app.hyperliquid.xyz/ → Settings → API Wallets")
	}

	// Check if user accidentally uses main wallet private key (security risk)
	if strings.EqualFold(walletAddr, agentAddr) {
		logger.Infof("⚠️⚠️⚠️ WARNING: Main wallet address (%s) matches Agent wallet address!", walletAddr)
		logger.Infof("   This indicates you may be using your main wallet private key, which poses extremely high security risks!")
		logger.Infof("   Recommendation: Immediately create a separate Agent Wallet on Hyperliquid official website")
		logger.Infof("   Reference: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/nonces-and-api-wallets")
	} else {
		logger.Infof("✓ Using Agent Wallet mode (secure)")
		logger.Infof("  └─ Agent wallet address: %s (for signing)", agentAddr)
		logger.Infof("  └─ Main wallet address: %s (holds funds)", walletAddr)
	}

	ctx := context.Background()

	// Create Exchange client (Exchange includes Info functionality)
	exchange := hyperliquid.NewExchange(
		ctx,
		privateKey,
		apiURL,
		nil,        // Meta will be fetched automatically
		"",         // vault address (empty for personal account)
		walletAddr, // wallet address
		nil,        // SpotMeta will be fetched automatically
	)

	logger.Infof("✓ Hyperliquid trader initialized successfully (testnet=%v, wallet=%s)", testnet, walletAddr)

	// Get meta information (including precision and other configurations)
	meta, err := exchange.Info().Meta(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get meta information: %w", err)
	}

	// 🔍 Security check: Validate Agent wallet balance (should be close to 0)
	// Only check if using separate Agent wallet (not when main wallet is used as agent)
	if !strings.EqualFold(walletAddr, agentAddr) {
		agentState, err := exchange.Info().UserState(ctx, agentAddr)
		if err == nil && agentState != nil && agentState.CrossMarginSummary.AccountValue != "" {
			// Parse Agent wallet balance
			agentBalance, _ := strconv.ParseFloat(agentState.CrossMarginSummary.AccountValue, 64)

			if agentBalance > 100 {
				// Critical: Agent wallet holds too much funds
				logger.Infof("🚨🚨🚨 CRITICAL SECURITY WARNING 🚨🚨🚨")
				logger.Infof("   Agent wallet balance: %.2f USDC (exceeds safe threshold of 100 USDC)", agentBalance)
				logger.Infof("   Agent wallet address: %s", agentAddr)
				logger.Infof("   ⚠️  Agent wallets should only be used for signing and hold minimal/zero balance")
				logger.Infof("   ⚠️  High balance in Agent wallet poses security risks")
				logger.Infof("   📖 Reference: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/nonces-and-api-wallets")
				logger.Infof("   💡 Recommendation: Transfer funds to main wallet and keep Agent wallet balance near 0")
				return nil, fmt.Errorf("security check failed: Agent wallet balance too high (%.2f USDC), exceeds 100 USDC threshold", agentBalance)
			} else if agentBalance > 10 {
				// Warning: Agent wallet has some balance (acceptable but not ideal)
				logger.Infof("⚠️  Notice: Agent wallet address (%s) has some balance: %.2f USDC", agentAddr, agentBalance)
				logger.Infof("   While not critical, it's recommended to keep Agent wallet balance near 0 for security")
			} else {
				// OK: Agent wallet balance is safe
				logger.Infof("✓ Agent wallet balance is safe: %.2f USDC (near zero as recommended)", agentBalance)
			}
		} else if err != nil {
			// Failed to query agent balance - log warning but don't block initialization
			logger.Infof("⚠️  Could not verify Agent wallet balance (query failed): %v", err)
			logger.Infof("   Proceeding with initialization, but please manually verify Agent wallet balance is near 0")
		}
	}

	return &HyperliquidTrader{
		exchange:      exchange,
		ctx:           ctx,
		walletAddr:    walletAddr,
		meta:          meta,
		isCrossMargin: true, // Use cross margin mode by default
		privateKey:    privateKey,
		isTestnet:     testnet,
	}, nil
}

// GetBalance gets account balance
func (t *HyperliquidTrader) GetBalance() (map[string]interface{}, error) {
	logger.Infof("🔄 Calling Hyperliquid API to get account balance...")

	// ✅ Step 1: Query Spot account balance
	spotState, err := t.exchange.Info().SpotUserState(t.ctx, t.walletAddr)
	var spotUSDCBalance float64 = 0.0
	if err != nil {
		logger.Infof("⚠️ Failed to query Spot balance (may have no spot assets): %v", err)
	} else if spotState != nil && len(spotState.Balances) > 0 {
		for _, balance := range spotState.Balances {
			if balance.Coin == "USDC" {
				spotUSDCBalance, _ = strconv.ParseFloat(balance.Total, 64)
				logger.Infof("✓ Found Spot balance: %.2f USDC", spotUSDCBalance)
				break
			}
		}
	}

	// ✅ Step 2: Query Perpetuals contract account status
	accountState, err := t.exchange.Info().UserState(t.ctx, t.walletAddr)
	if err != nil {
		logger.Infof("❌ Hyperliquid Perpetuals API call failed: %v", err)
		return nil, fmt.Errorf("failed to get account information: %w", err)
	}

	// Parse balance information (MarginSummary fields are all strings)
	result := make(map[string]interface{})

	// ✅ Step 3: Dynamically select correct summary based on margin mode (CrossMarginSummary or MarginSummary)
	var accountValue, totalMarginUsed float64
	var summaryType string
	var summary interface{}

	if t.isCrossMargin {
		// Cross margin mode: use CrossMarginSummary
		accountValue, _ = strconv.ParseFloat(accountState.CrossMarginSummary.AccountValue, 64)
		totalMarginUsed, _ = strconv.ParseFloat(accountState.CrossMarginSummary.TotalMarginUsed, 64)
		summaryType = "CrossMarginSummary (cross margin)"
		summary = accountState.CrossMarginSummary
	} else {
		// Isolated margin mode: use MarginSummary
		accountValue, _ = strconv.ParseFloat(accountState.MarginSummary.AccountValue, 64)
		totalMarginUsed, _ = strconv.ParseFloat(accountState.MarginSummary.TotalMarginUsed, 64)
		summaryType = "MarginSummary (isolated margin)"
		summary = accountState.MarginSummary
	}

	// 🔍 Debug: Print complete summary structure returned by API
	summaryJSON, _ := json.MarshalIndent(summary, "  ", "  ")
	logger.Infof("🔍 [DEBUG] Hyperliquid API %s complete data:", summaryType)
	logger.Infof("%s", string(summaryJSON))

	// ⚠️ Critical fix: Accumulate actual unrealized PnL from all positions
	totalUnrealizedPnl := 0.0
	for _, assetPos := range accountState.AssetPositions {
		unrealizedPnl, _ := strconv.ParseFloat(assetPos.Position.UnrealizedPnl, 64)
		totalUnrealizedPnl += unrealizedPnl
	}

	// ✅ Correctly understand Hyperliquid fields:
	// AccountValue = Total account equity (includes idle funds + position value + unrealized PnL)
	// TotalMarginUsed = Margin used by positions (included in AccountValue, for display only)
	//
	// To be compatible with auto_types.go calculation logic (totalEquity = totalWalletBalance + totalUnrealizedProfit)
	// Need to return "wallet balance without unrealized PnL"
	walletBalanceWithoutUnrealized := accountValue - totalUnrealizedPnl

	// ✅ Step 4: Use Withdrawable field (PR #443)
	// Withdrawable is the official real withdrawable balance, more reliable than simple calculation
	availableBalance := 0.0
	if accountState.Withdrawable != "" {
		withdrawable, err := strconv.ParseFloat(accountState.Withdrawable, 64)
		if err == nil && withdrawable > 0 {
			availableBalance = withdrawable
			logger.Infof("✓ Using Withdrawable as available balance: %.2f", availableBalance)
		}
	}

	// Fallback: If no Withdrawable, use simple calculation
	if availableBalance == 0 && accountState.Withdrawable == "" {
		availableBalance = accountValue - totalMarginUsed
		if availableBalance < 0 {
			logger.Infof("⚠️ Calculated available balance is negative (%.2f), reset to 0", availableBalance)
			availableBalance = 0
		}
	}

	// ✅ Step 5: Query xyz dex balance (stock perps, forex, commodities)
	var xyzAccountValue, xyzUnrealizedPnl float64
	var xyzPositions []xyzAssetPosition
	xyzAccountValue, xyzUnrealizedPnl, xyzPositions, err = t.getXYZDexBalance()
	if err != nil {
		// xyz dex query failed - log warning but don't fail the entire balance query
		logger.Infof("⚠️ Failed to query xyz dex balance: %v", err)
	}
	// Always log xyz dex state for debugging
	logger.Infof("🔍 xyz dex state: accountValue=%.4f, unrealizedPnl=%.4f, positions=%d",
		xyzAccountValue, xyzUnrealizedPnl, len(xyzPositions))
	for _, pos := range xyzPositions {
		entryPx := "nil"
		if pos.Position.EntryPx != nil {
			entryPx = *pos.Position.EntryPx
		}
		logger.Infof("   └─ %s: size=%s, entryPx=%s, posValue=%s, pnl=%s",
			pos.Position.Coin, pos.Position.Szi, entryPx, pos.Position.PositionValue, pos.Position.UnrealizedPnl)
	}
	xyzWalletBalance := xyzAccountValue - xyzUnrealizedPnl

	// ✅ Step 6: Correctly handle Spot + Perpetuals + xyz dex balance
	// Important: Each account is independent, manual transfers required
	totalWalletBalance := walletBalanceWithoutUnrealized + spotUSDCBalance + xyzWalletBalance
	totalUnrealizedPnlAll := totalUnrealizedPnl + xyzUnrealizedPnl

	// Calculate total equity properly: perpAccountValue + spotUSDCBalance + xyzAccountValue
	// Note: totalWalletBalance + totalUnrealizedPnlAll should equal this
	totalEquityCalculated := accountValue + spotUSDCBalance + xyzAccountValue

	result["totalWalletBalance"] = totalWalletBalance       // Total assets (Perp + Spot + xyz) - unrealized
	result["totalEquity"] = totalEquityCalculated           // Total equity = Perp AV + Spot + xyz AV
	result["availableBalance"] = availableBalance           // Available balance (Perpetuals only)
	result["totalUnrealizedProfit"] = totalUnrealizedPnlAll // Unrealized PnL (Perpetuals + xyz)
	result["spotBalance"] = spotUSDCBalance                 // Spot balance
	result["xyzDexBalance"] = xyzAccountValue               // xyz dex equity (stock perps, forex, commodities)
	result["xyzDexUnrealizedPnl"] = xyzUnrealizedPnl        // xyz dex unrealized PnL
	result["perpAccountValue"] = accountValue               // Perp account value for debugging

	logger.Infof("✓ Hyperliquid complete account:")
	logger.Infof("  • Spot balance: %.2f USDC", spotUSDCBalance)
	logger.Infof("  • Perpetuals equity: %.2f USDC (wallet %.2f + unrealized %.2f)",
		accountValue,
		walletBalanceWithoutUnrealized,
		totalUnrealizedPnl)
	logger.Infof("  • Perpetuals available balance: %.2f USDC", availableBalance)
	logger.Infof("  • Margin used: %.2f USDC", totalMarginUsed)
	logger.Infof("  • xyz dex equity: %.2f USDC (wallet %.2f + unrealized %.2f)",
		xyzAccountValue,
		xyzWalletBalance,
		xyzUnrealizedPnl)
	logger.Infof("  • Total assets (Perp+Spot+xyz): %.2f USDC", totalWalletBalance)
	logger.Infof("  ⭐ Total: %.2f USDC | Perp: %.2f | Spot: %.2f | xyz: %.2f",
		totalWalletBalance, availableBalance, spotUSDCBalance, xyzAccountValue)

	return result, nil
}

// xyzDexState represents the clearinghouse state for xyz dex
type xyzDexState struct {
	MarginSummary      *xyzMarginSummary  `json:"marginSummary,omitempty"`
	CrossMarginSummary *xyzMarginSummary  `json:"crossMarginSummary,omitempty"`
	Withdrawable       string             `json:"withdrawable,omitempty"`
	AssetPositions     []xyzAssetPosition `json:"assetPositions,omitempty"`
}

type xyzMarginSummary struct {
	AccountValue    string `json:"accountValue"`
	TotalMarginUsed string `json:"totalMarginUsed"`
}

type xyzAssetPosition struct {
	Position struct {
		Coin          string  `json:"coin"`
		Szi           string  `json:"szi"`
		EntryPx       *string `json:"entryPx"`
		PositionValue string  `json:"positionValue"`
		UnrealizedPnl string  `json:"unrealizedPnl"`
		LiquidationPx *string `json:"liquidationPx"`
		Leverage      struct {
			Type  string `json:"type"`
			Value int    `json:"value"`
		} `json:"leverage"`
	} `json:"position"`
}

// getXYZDexBalance queries the xyz dex balance (stock perps, forex, commodities)
func (t *HyperliquidTrader) getXYZDexBalance() (accountValue float64, unrealizedPnl float64, positions []xyzAssetPosition, err error) {
	// Build request for xyz dex clearinghouse state
	reqBody := map[string]interface{}{
		"type": "clearinghouseState",
		"user": t.walletAddr,
		"dex":  "xyz",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Determine API URL
	apiURL := "https://api.hyperliquid.xyz/info"
	// Note: xyz dex may not be available on testnet

	req, err := http.NewRequestWithContext(t.ctx, "POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, 0, nil, fmt.Errorf("xyz dex API error (status %d): %s", resp.StatusCode, string(body))
	}

	var state xyzDexState
	if err := json.Unmarshal(body, &state); err != nil {
		return 0, 0, nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Parse account value - xyz dex uses MarginSummary for isolated margin mode
	// CrossMarginSummary may exist but with 0 values, so check MarginSummary first
	if state.MarginSummary != nil && state.MarginSummary.AccountValue != "" {
		av, _ := strconv.ParseFloat(state.MarginSummary.AccountValue, 64)
		if av > 0 {
			accountValue = av
		}
	}
	// Fallback to CrossMarginSummary if MarginSummary is 0
	if accountValue == 0 && state.CrossMarginSummary != nil && state.CrossMarginSummary.AccountValue != "" {
		accountValue, _ = strconv.ParseFloat(state.CrossMarginSummary.AccountValue, 64)
	}

	// Calculate total unrealized PnL from positions
	for _, pos := range state.AssetPositions {
		pnl, _ := strconv.ParseFloat(pos.Position.UnrealizedPnl, 64)
		unrealizedPnl += pnl
	}

	return accountValue, unrealizedPnl, state.AssetPositions, nil
}

// fetchXyzMeta fetches metadata for xyz dex assets (stocks, forex, commodities)
func (t *HyperliquidTrader) fetchXyzMeta() error {
	// Build request for xyz dex meta
	reqBody := map[string]string{
		"type": "meta",
		"dex":  "xyz",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := "https://api.hyperliquid.xyz/info"

	req, err := http.NewRequestWithContext(t.ctx, "POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("xyz dex meta API error (status %d): %s", resp.StatusCode, string(body))
	}

	var meta xyzDexMeta
	if err := json.Unmarshal(body, &meta); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	t.xyzMetaMutex.Lock()
	t.xyzMeta = &meta
	t.xyzMetaMutex.Unlock()

	logger.Infof("✅ xyz dex meta fetched, contains %d assets", len(meta.Universe))
	return nil
}

// getXyzSzDecimals gets quantity precision for xyz dex asset
func (t *HyperliquidTrader) getXyzSzDecimals(coin string) int {
	t.xyzMetaMutex.RLock()
	defer t.xyzMetaMutex.RUnlock()

	if t.xyzMeta == nil {
		logger.Infof("⚠️  xyz meta information is empty, using default precision 2")
		return 2 // Default precision for stocks/forex
	}

	// The meta API returns names with xyz: prefix, so ensure we match correctly
	lookupName := coin
	if !strings.HasPrefix(lookupName, "xyz:") {
		lookupName = "xyz:" + lookupName
	}

	// Find corresponding asset in xyzMeta.Universe
	for _, asset := range t.xyzMeta.Universe {
		if asset.Name == lookupName {
			return asset.SzDecimals
		}
	}

	logger.Infof("⚠️  Precision information not found for %s, using default precision 2", lookupName)
	return 2 // Default precision for stocks/forex
}

// GetPositions gets all positions (including xyz dex positions)
func (t *HyperliquidTrader) GetPositions() ([]map[string]interface{}, error) {
	// Get account status
	accountState, err := t.exchange.Info().UserState(t.ctx, t.walletAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var result []map[string]interface{}

	// Iterate through all perp positions
	for _, assetPos := range accountState.AssetPositions {
		position := assetPos.Position

		// Position amount (string type)
		posAmt, _ := strconv.ParseFloat(position.Szi, 64)

		if posAmt == 0 {
			continue // Skip positions with zero amount
		}

		posMap := make(map[string]interface{})

		// Normalize symbol format (Hyperliquid uses "BTC", we convert to "BTCUSDT")
		symbol := position.Coin + "USDT"
		posMap["symbol"] = symbol

		// Position amount and direction
		if posAmt > 0 {
			posMap["side"] = "long"
			posMap["positionAmt"] = posAmt
		} else {
			posMap["side"] = "short"
			posMap["positionAmt"] = -posAmt // Convert to positive number
		}

		// Price information (EntryPx and LiquidationPx are pointer types)
		var entryPrice, liquidationPx float64
		if position.EntryPx != nil {
			entryPrice, _ = strconv.ParseFloat(*position.EntryPx, 64)
		}
		if position.LiquidationPx != nil {
			liquidationPx, _ = strconv.ParseFloat(*position.LiquidationPx, 64)
		}

		positionValue, _ := strconv.ParseFloat(position.PositionValue, 64)
		unrealizedPnl, _ := strconv.ParseFloat(position.UnrealizedPnl, 64)

		// Calculate mark price (positionValue / abs(posAmt))
		var markPrice float64
		if posAmt != 0 {
			markPrice = positionValue / absFloat(posAmt)
		}

		posMap["entryPrice"] = entryPrice
		posMap["markPrice"] = markPrice
		posMap["unRealizedProfit"] = unrealizedPnl
		posMap["leverage"] = float64(position.Leverage.Value)
		posMap["liquidationPrice"] = liquidationPx

		result = append(result, posMap)
	}

	// Also get xyz dex positions (stocks, forex, commodities)
	_, _, xyzPositions, err := t.getXYZDexBalance()
	if err != nil {
		// xyz dex query failed - log warning but don't fail
		logger.Infof("⚠️  Failed to get xyz dex positions: %v", err)
	} else {
		for _, pos := range xyzPositions {
			posAmt, _ := strconv.ParseFloat(pos.Position.Szi, 64)
			if posAmt == 0 {
				continue
			}

			posMap := make(map[string]interface{})

			// xyz dex positions - the API returns coin names with xyz: prefix (e.g., "xyz:SILVER")
			// Only add prefix if not already present
			symbol := pos.Position.Coin
			if !strings.HasPrefix(symbol, "xyz:") {
				symbol = "xyz:" + symbol
			}
			posMap["symbol"] = symbol

			if posAmt > 0 {
				posMap["side"] = "long"
				posMap["positionAmt"] = posAmt
			} else {
				posMap["side"] = "short"
				posMap["positionAmt"] = -posAmt
			}

			// Parse price information
			var entryPrice, liquidationPx float64
			if pos.Position.EntryPx != nil {
				entryPrice, _ = strconv.ParseFloat(*pos.Position.EntryPx, 64)
			}
			if pos.Position.LiquidationPx != nil {
				liquidationPx, _ = strconv.ParseFloat(*pos.Position.LiquidationPx, 64)
			}

			positionValue, _ := strconv.ParseFloat(pos.Position.PositionValue, 64)
			unrealizedPnl, _ := strconv.ParseFloat(pos.Position.UnrealizedPnl, 64)

			// Calculate mark price from position value
			var markPrice float64
			if posAmt != 0 {
				markPrice = positionValue / absFloat(posAmt)
			}

			// Get leverage (default to 1 if not available)
			leverage := float64(pos.Position.Leverage.Value)
			if leverage == 0 {
				leverage = 1.0
			}

			posMap["entryPrice"] = entryPrice
			posMap["markPrice"] = markPrice
			posMap["unRealizedProfit"] = unrealizedPnl
			posMap["leverage"] = leverage
			posMap["liquidationPrice"] = liquidationPx
			posMap["isXyzDex"] = true // Mark as xyz dex position

			result = append(result, posMap)
		}
	}

	return result, nil
}

// SetMarginMode sets margin mode (set together with SetLeverage)
func (t *HyperliquidTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// Hyperliquid's margin mode is set in SetLeverage, only record here
	t.isCrossMargin = isCrossMargin
	marginModeStr := "cross margin"
	if !isCrossMargin {
		marginModeStr = "isolated margin"
	}
	logger.Infof("  ✓ %s will use %s mode", symbol, marginModeStr)
	return nil
}

// SetLeverage sets leverage
func (t *HyperliquidTrader) SetLeverage(symbol string, leverage int) error {
	// Hyperliquid symbol format (remove USDT suffix)
	coin := convertSymbolToHyperliquid(symbol)

	// Call UpdateLeverage (leverage int, name string, isCross bool)
	// Third parameter: true=cross margin mode, false=isolated margin mode
	_, err := t.exchange.UpdateLeverage(t.ctx, leverage, coin, t.isCrossMargin)
	if err != nil {
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	logger.Infof("  ✓ %s leverage switched to %dx", symbol, leverage)
	return nil
}

// refreshMetaIfNeeded refreshes meta information when invalid (triggered when Asset ID is 0)
func (t *HyperliquidTrader) refreshMetaIfNeeded(coin string) error {
	assetID := t.exchange.Info().NameToAsset(coin)
	if assetID != 0 {
		return nil // Meta is normal, no refresh needed
	}

	logger.Infof("⚠️  Asset ID for %s is 0, attempting to refresh Meta information...", coin)

	// Refresh Meta information
	meta, err := t.exchange.Info().Meta(t.ctx)
	if err != nil {
		return fmt.Errorf("failed to refresh Meta information: %w", err)
	}

	// ✅ Concurrency safe: Use write lock to protect meta field update
	t.metaMutex.Lock()
	t.meta = meta
	t.metaMutex.Unlock()

	logger.Infof("✅ Meta information refreshed, contains %d assets", len(meta.Universe))

	// Verify Asset ID after refresh
	assetID = t.exchange.Info().NameToAsset(coin)
	if assetID == 0 {
		return fmt.Errorf("❌ Even after refreshing Meta, Asset ID for %s is still 0. Possible reasons:\n"+
			"  1. This coin is not listed on Hyperliquid\n"+
			"  2. Coin name is incorrect (should be BTC not BTCUSDT)\n"+
			"  3. API connection issue", coin)
	}

	logger.Infof("✅ Asset ID check passed after refresh: %s -> %d", coin, assetID)
	return nil
}

// OpenLong opens a long position (supports both crypto and xyz dex)
func (t *HyperliquidTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// First cancel all pending orders for this coin
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel old pending orders: %v", err)
	}

	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)

	// Check if this is an xyz dex asset
	isXyz := strings.HasPrefix(coin, "xyz:")

	// Set leverage (skip for xyz dex as it may not support leverage adjustment)
	if !isXyz {
		if err := t.SetLeverage(symbol, leverage); err != nil {
			return nil, err
		}
	} else {
		logger.Infof("  ℹ xyz dex asset %s - using default leverage", coin)
	}

	// Get current price (for market order)
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ Critical: Price needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 1.01)
	logger.Infof("  💰 Price precision handling: %.8f -> %.8f (5 significant figures)", price*1.01, aggressivePrice)

	// Handle xyz dex assets differently
	if isXyz {
		// xyz dex order
		if err := t.placeXyzOrder(coin, true, quantity, aggressivePrice, false); err != nil {
			return nil, fmt.Errorf("failed to open long position on xyz dex: %w", err)
		}
	} else {
		// Standard crypto order
		roundedQuantity := t.roundToSzDecimals(coin, quantity)
		logger.Infof("  📏 Quantity precision handling: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

		order := hyperliquid.CreateOrderRequest{
			Coin:  coin,
			IsBuy: true,
			Size:  roundedQuantity,
			Price: aggressivePrice,
			OrderType: hyperliquid.OrderType{
				Limit: &hyperliquid.LimitOrderType{
					Tif: hyperliquid.TifIoc,
				},
			},
			ReduceOnly: false,
		}

		_, err = t.exchange.Order(t.ctx, order, defaultBuilder)
		if err != nil {
			return nil, fmt.Errorf("failed to open long position: %w", err)
		}
	}

	logger.Infof("✓ Long position opened successfully: %s quantity: %.4f", symbol, quantity)

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// OpenShort opens a short position (supports both crypto and xyz dex)
func (t *HyperliquidTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// First cancel all pending orders for this coin
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel old pending orders: %v", err)
	}

	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)

	// Check if this is an xyz dex asset
	isXyz := strings.HasPrefix(coin, "xyz:")

	// Set leverage (skip for xyz dex)
	if !isXyz {
		if err := t.SetLeverage(symbol, leverage); err != nil {
			return nil, err
		}
	} else {
		logger.Infof("  ℹ xyz dex asset %s - using default leverage", coin)
	}

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ Critical: Price needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 0.99)
	logger.Infof("  💰 Price precision handling: %.8f -> %.8f (5 significant figures)", price*0.99, aggressivePrice)

	// Handle xyz dex assets differently
	if isXyz {
		// xyz dex order
		if err := t.placeXyzOrder(coin, false, quantity, aggressivePrice, false); err != nil {
			return nil, fmt.Errorf("failed to open short position on xyz dex: %w", err)
		}
	} else {
		// Standard crypto order
		roundedQuantity := t.roundToSzDecimals(coin, quantity)
		logger.Infof("  📏 Quantity precision handling: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

		order := hyperliquid.CreateOrderRequest{
			Coin:  coin,
			IsBuy: false,
			Size:  roundedQuantity,
			Price: aggressivePrice,
			OrderType: hyperliquid.OrderType{
				Limit: &hyperliquid.LimitOrderType{
					Tif: hyperliquid.TifIoc,
				},
			},
			ReduceOnly: false,
		}

		_, err = t.exchange.Order(t.ctx, order, defaultBuilder)
		if err != nil {
			return nil, fmt.Errorf("failed to open short position: %w", err)
		}
	}

	logger.Infof("✓ Short position opened successfully: %s quantity: %.4f", symbol, quantity)

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CloseLong closes a long position (supports both crypto and xyz dex)
func (t *HyperliquidTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)
	isXyz := strings.HasPrefix(coin, "xyz:")

	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		// For xyz dex, also check xyz: prefixed symbols
		searchSymbol := symbol
		if isXyz {
			searchSymbol = coin // Use xyz:SYMBOL format for comparison
		}

		for _, pos := range positions {
			posSymbol := pos["symbol"].(string)
			if (posSymbol == symbol || posSymbol == searchSymbol) && pos["side"] == "long" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no long position found for %s", symbol)
		}
	}

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ Critical: Price needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 0.99)
	logger.Infof("  💰 Price precision handling: %.8f -> %.8f (5 significant figures)", price*0.99, aggressivePrice)

	// Handle xyz dex assets differently
	if isXyz {
		// xyz dex close order
		if err := t.placeXyzOrder(coin, false, quantity, aggressivePrice, true); err != nil {
			return nil, fmt.Errorf("failed to close long position on xyz dex: %w", err)
		}
	} else {
		// Standard crypto close order
		roundedQuantity := t.roundToSzDecimals(coin, quantity)
		logger.Infof("  📏 Quantity precision handling: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

		order := hyperliquid.CreateOrderRequest{
			Coin:  coin,
			IsBuy: false,
			Size:  roundedQuantity,
			Price: aggressivePrice,
			OrderType: hyperliquid.OrderType{
				Limit: &hyperliquid.LimitOrderType{
					Tif: hyperliquid.TifIoc,
				},
			},
			ReduceOnly: true,
		}

		_, err = t.exchange.Order(t.ctx, order, defaultBuilder)
		if err != nil {
			return nil, fmt.Errorf("failed to close long position: %w", err)
		}
	}

	logger.Infof("✓ Long position closed successfully: %s quantity: %.4f", symbol, quantity)

	// Cancel all pending orders for this coin after closing position
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel pending orders: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CloseShort closes a short position (supports both crypto and xyz dex)
func (t *HyperliquidTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)
	isXyz := strings.HasPrefix(coin, "xyz:")

	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		// For xyz dex, also check xyz: prefixed symbols
		searchSymbol := symbol
		if isXyz {
			searchSymbol = coin
		}

		for _, pos := range positions {
			posSymbol := pos["symbol"].(string)
			if (posSymbol == symbol || posSymbol == searchSymbol) && pos["side"] == "short" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no short position found for %s", symbol)
		}
	}

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ Critical: Price needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 1.01)
	logger.Infof("  💰 Price precision handling: %.8f -> %.8f (5 significant figures)", price*1.01, aggressivePrice)

	// Handle xyz dex assets differently
	if isXyz {
		// xyz dex close order
		if err := t.placeXyzOrder(coin, true, quantity, aggressivePrice, true); err != nil {
			return nil, fmt.Errorf("failed to close short position on xyz dex: %w", err)
		}
	} else {
		// Standard crypto close order
		roundedQuantity := t.roundToSzDecimals(coin, quantity)
		logger.Infof("  📏 Quantity precision handling: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

		order := hyperliquid.CreateOrderRequest{
			Coin:  coin,
			IsBuy: true,
			Size:  roundedQuantity,
			Price: aggressivePrice,
			OrderType: hyperliquid.OrderType{
				Limit: &hyperliquid.LimitOrderType{
					Tif: hyperliquid.TifIoc,
				},
			},
			ReduceOnly: true,
		}

		_, err = t.exchange.Order(t.ctx, order, defaultBuilder)
		if err != nil {
			return nil, fmt.Errorf("failed to close short position: %w", err)
		}
	}

	logger.Infof("✓ Short position closed successfully: %s quantity: %.4f", symbol, quantity)

	// Cancel all pending orders for this coin after closing position
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel pending orders: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CancelStopLossOrders only cancels stop loss orders (Hyperliquid cannot distinguish stop loss and take profit, cancel all)
func (t *HyperliquidTrader) CancelStopLossOrders(symbol string) error {
	// Hyperliquid SDK's OpenOrder structure does not expose trigger field
	// Cannot distinguish stop loss and take profit orders, so cancel all pending orders for this coin
	logger.Infof("  ⚠️ Hyperliquid cannot distinguish stop loss/take profit orders, will cancel all pending orders")
	return t.CancelStopOrders(symbol)
}

// CancelTakeProfitOrders only cancels take profit orders (Hyperliquid cannot distinguish stop loss and take profit, cancel all)
func (t *HyperliquidTrader) CancelTakeProfitOrders(symbol string) error {
	// Hyperliquid SDK's OpenOrder structure does not expose trigger field
	// Cannot distinguish stop loss and take profit orders, so cancel all pending orders for this coin
	logger.Infof("  ⚠️ Hyperliquid cannot distinguish stop loss/take profit orders, will cancel all pending orders")
	return t.CancelStopOrders(symbol)
}

// CancelAllOrders cancels all pending orders for this coin
func (t *HyperliquidTrader) CancelAllOrders(symbol string) error {
	coin := convertSymbolToHyperliquid(symbol)

	// Check if this is an xyz dex asset
	isXyz := strings.HasPrefix(coin, "xyz:")

	if isXyz {
		// xyz dex orders - use direct API call
		return t.cancelXyzOrders(coin)
	}

	// Standard crypto orders
	openOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
	if err != nil {
		return fmt.Errorf("failed to get pending orders: %w", err)
	}

	// Cancel all pending orders for this coin
	for _, order := range openOrders {
		if order.Coin == coin {
			_, err := t.exchange.Cancel(t.ctx, coin, order.Oid)
			if err != nil {
				logger.Infof("  ⚠ Failed to cancel order (oid=%d): %v", order.Oid, err)
			}
		}
	}

	logger.Infof("  ✓ Cancelled all pending orders for %s", symbol)
	return nil
}

// CancelStopOrders cancels take profit/stop loss orders for this coin (used to adjust TP/SL positions)
func (t *HyperliquidTrader) CancelStopOrders(symbol string) error {
	coin := convertSymbolToHyperliquid(symbol)

	// Check if this is an xyz dex asset
	isXyz := strings.HasPrefix(coin, "xyz:")

	if isXyz {
		// xyz dex orders - use direct API call
		return t.cancelXyzOrders(coin)
	}

	// Get all pending orders for standard crypto
	openOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
	if err != nil {
		return fmt.Errorf("failed to get pending orders: %w", err)
	}

	// Note: Hyperliquid SDK's OpenOrder structure does not expose trigger field
	// Therefore temporarily cancel all pending orders for this coin (including TP/SL orders)
	// This is safe because all old orders should be cleaned up before setting new TP/SL
	canceledCount := 0
	for _, order := range openOrders {
		if order.Coin == coin {
			_, err := t.exchange.Cancel(t.ctx, coin, order.Oid)
			if err != nil {
				logger.Infof("  ⚠ Failed to cancel order (oid=%d): %v", order.Oid, err)
				continue
			}
			canceledCount++
		}
	}

	if canceledCount == 0 {
		logger.Infof("  ℹ No pending orders to cancel for %s", symbol)
	} else {
		logger.Infof("  ✓ Cancelled %d pending orders for %s (including TP/SL orders)", canceledCount, symbol)
	}

	return nil
}

// cancelXyzOrders cancels all pending orders for xyz dex assets (stocks, forex, commodities)
func (t *HyperliquidTrader) cancelXyzOrders(coin string) error {
	// Query xyz dex open orders
	reqBody := map[string]interface{}{
		"type": "openOrders",
		"user": t.walletAddr,
		"dex":  "xyz",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := "https://api.hyperliquid.xyz/info"

	req, err := http.NewRequestWithContext(t.ctx, "POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("xyz dex openOrders API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse open orders
	var openOrders []struct {
		Coin string `json:"coin"`
		Oid  int64  `json:"oid"`
	}
	if err := json.Unmarshal(body, &openOrders); err != nil {
		return fmt.Errorf("failed to parse open orders: %w", err)
	}

	// Filter orders for this coin and cancel them
	canceledCount := 0
	for _, order := range openOrders {
		if order.Coin == coin {
			if err := t.cancelXyzOrder(order.Oid); err != nil {
				logger.Infof("  ⚠ Failed to cancel xyz dex order (oid=%d): %v", order.Oid, err)
				continue
			}
			canceledCount++
		}
	}

	if canceledCount == 0 {
		logger.Infof("  ℹ No pending xyz dex orders to cancel for %s", coin)
	} else {
		logger.Infof("  ✓ Cancelled %d xyz dex orders for %s", canceledCount, coin)
	}

	return nil
}

// cancelXyzOrder cancels a single xyz dex order by oid
func (t *HyperliquidTrader) cancelXyzOrder(oid int64) error {
	// Get asset index for this order (we need it for cancel action)
	// For cancel, we construct a cancel action with the oid

	action := map[string]interface{}{
		"type": "cancel",
		"cancels": []map[string]interface{}{
			{
				"a": oid, // asset index not needed for cancel by oid in xyz dex
				"o": oid,
			},
		},
	}

	// Sign the action
	nonce := time.Now().UnixMilli()
	isMainnet := !t.isTestnet
	vaultAddress := ""

	sig, err := hyperliquid.SignL1Action(t.privateKey, action, vaultAddress, nonce, nil, isMainnet)
	if err != nil {
		return fmt.Errorf("failed to sign cancel action: %w", err)
	}

	payload := map[string]any{
		"action":    action,
		"nonce":     nonce,
		"signature": sig,
	}

	apiURL := hyperliquid.MainnetAPIURL
	if t.isTestnet {
		apiURL = hyperliquid.TestnetAPIURL
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(t.ctx, http.MethodPost, apiURL+"/exchange", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check response
	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != "ok" {
		return fmt.Errorf("cancel failed: %s", string(body))
	}

	return nil
}

// GetMarketPrice gets market price (supports both crypto and xyz dex assets)
func (t *HyperliquidTrader) GetMarketPrice(symbol string) (float64, error) {
	coin := convertSymbolToHyperliquid(symbol)

	// Check if this is an xyz dex asset
	if strings.HasPrefix(coin, "xyz:") {
		return t.getXyzMarketPrice(coin)
	}

	// Get all market prices for crypto
	allMids, err := t.exchange.Info().AllMids(t.ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get price: %w", err)
	}

	// Find price for corresponding coin (allMids is map[string]string)
	if priceStr, ok := allMids[coin]; ok {
		priceFloat, err := strconv.ParseFloat(priceStr, 64)
		if err == nil {
			return priceFloat, nil
		}
		return 0, fmt.Errorf("price format error: %v", err)
	}

	return 0, fmt.Errorf("price not found for %s", symbol)
}

// getXyzMarketPrice gets market price for xyz dex assets
func (t *HyperliquidTrader) getXyzMarketPrice(coin string) (float64, error) {
	// Build request for xyz dex allMids
	reqBody := map[string]string{
		"type": "allMids",
		"dex":  "xyz",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := "https://api.hyperliquid.xyz/info"

	req, err := http.NewRequestWithContext(t.ctx, "POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("xyz dex allMids API error (status %d): %s", resp.StatusCode, string(body))
	}

	var mids map[string]string
	if err := json.Unmarshal(body, &mids); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	// The API returns keys with xyz: prefix, so ensure the coin has it
	lookupKey := coin
	if !strings.HasPrefix(lookupKey, "xyz:") {
		lookupKey = "xyz:" + lookupKey
	}

	if priceStr, ok := mids[lookupKey]; ok {
		priceFloat, err := strconv.ParseFloat(priceStr, 64)
		if err == nil {
			return priceFloat, nil
		}
		return 0, fmt.Errorf("price format error: %v", err)
	}

	return 0, fmt.Errorf("xyz dex price not found for %s (lookup key: %s)", coin, lookupKey)
}

// floatToWireStr converts a float to wire format string (8 decimal places, trimmed zeros)
// This matches the SDK's floatToWire function
func floatToWireStr(x float64) string {
	// Format to 8 decimal places
	result := fmt.Sprintf("%.8f", x)
	// Remove trailing zeros
	result = strings.TrimRight(result, "0")
	// Remove trailing decimal point if no decimals left
	result = strings.TrimRight(result, ".")
	return result
}

// placeXyzOrder places an order on the xyz dex (stocks, forex, commodities)
// Note: xyz dex orders use builder-deployed perpetuals and require different handling
// xyz dex asset indices start from 10000 (10000 + meta_index)
// This implementation bypasses the SDK's NameToAsset lookup and directly constructs the order
func (t *HyperliquidTrader) placeXyzOrder(coin string, isBuy bool, size float64, price float64, reduceOnly bool) error {
	// Fetch xyz meta if not cached
	t.xyzMetaMutex.RLock()
	hasMeta := t.xyzMeta != nil
	t.xyzMetaMutex.RUnlock()

	if !hasMeta {
		if err := t.fetchXyzMeta(); err != nil {
			return fmt.Errorf("failed to fetch xyz meta: %w", err)
		}
	}

	// Get asset index from xyz meta (returns 0-based index)
	metaIndex := t.getXyzAssetIndex(coin)
	if metaIndex < 0 {
		return fmt.Errorf("xyz asset %s not found in meta", coin)
	}

	// HIP-3 perp dex asset index formula: 100000 + perp_dex_index * 10000 + index_in_meta
	// xyz dex is at perp_dex_index = 1 (verified from perpDexs API: [null, {name:"xyz",...}])
	// So xyz asset index = 100000 + 1 * 10000 + metaIndex = 110000 + metaIndex
	const xyzPerpDexIndex = 1
	assetIndex := 100000 + xyzPerpDexIndex*10000 + metaIndex

	// Round size to correct precision
	szDecimals := t.getXyzSzDecimals(coin)
	multiplier := 1.0
	for i := 0; i < szDecimals; i++ {
		multiplier *= 10.0
	}
	roundedSize := float64(int(size*multiplier+0.5)) / multiplier

	// Round price to 5 significant figures
	roundedPrice := t.roundPriceToSigfigs(price)

	logger.Infof("📝 Placing xyz dex order (direct): %s %s size=%.4f price=%.4f metaIndex=%d assetIndex=%d (formula: 100000 + 1*10000 + %d) reduceOnly=%v",
		map[bool]string{true: "BUY", false: "SELL"}[isBuy],
		coin, roundedSize, roundedPrice, metaIndex, assetIndex, metaIndex, reduceOnly)

	// Construct OrderWire directly with correct asset index (bypassing SDK's NameToAsset)
	orderWire := hyperliquid.OrderWire{
		Asset:      assetIndex,
		IsBuy:      isBuy,
		LimitPx:    floatToWireStr(roundedPrice),
		Size:       floatToWireStr(roundedSize),
		ReduceOnly: reduceOnly,
		OrderType: hyperliquid.OrderWireType{
			Limit: &hyperliquid.OrderWireTypeLimit{
				Tif: hyperliquid.TifIoc,
			},
		},
	}

	// Create OrderAction (no builder to avoid requiring builder fee approval)
	action := hyperliquid.OrderAction{
		Type:     "order",
		Orders:   []hyperliquid.OrderWire{orderWire},
		Grouping: "na",
		Builder:  nil,
	}

	// Sign the action
	nonce := time.Now().UnixMilli()
	isMainnet := !t.isTestnet
	vaultAddress := "" // No vault for personal account

	sig, err := hyperliquid.SignL1Action(t.privateKey, action, vaultAddress, nonce, nil, isMainnet)
	if err != nil {
		return fmt.Errorf("failed to sign xyz dex order: %w", err)
	}

	// Construct payload for /exchange endpoint
	payload := map[string]any{
		"action":    action,
		"nonce":     nonce,
		"signature": sig,
	}

	// Determine API URL
	apiURL := hyperliquid.MainnetAPIURL
	if t.isTestnet {
		apiURL = hyperliquid.TestnetAPIURL
	}

	// POST to /exchange
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	logger.Infof("📤 Sending xyz dex order to %s/exchange", apiURL)

	req, err := http.NewRequestWithContext(t.ctx, http.MethodPost, apiURL+"/exchange", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response
	var result struct {
		Status   string `json:"status"`
		Response struct {
			Type string `json:"type"`
			Data struct {
				Statuses []struct {
					Resting *struct {
						Oid int64 `json:"oid"`
					} `json:"resting,omitempty"`
					Filled *struct {
						TotalSz string `json:"totalSz"`
						AvgPx   string `json:"avgPx"`
						Oid     int    `json:"oid"`
					} `json:"filled,omitempty"`
					Error *string `json:"error,omitempty"`
				} `json:"statuses"`
			} `json:"data"`
		} `json:"response"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		// Try to parse as error response
		logger.Infof("⚠️  Failed to parse response as success, raw body: %s", string(body))
		return fmt.Errorf("xyz dex order failed, status=%d, body=%s", resp.StatusCode, string(body))
	}

	// Check for errors in response
	if result.Status != "ok" {
		return fmt.Errorf("xyz dex order failed: status=%s, body=%s", result.Status, string(body))
	}

	// Check order statuses
	if len(result.Response.Data.Statuses) > 0 {
		status := result.Response.Data.Statuses[0]
		if status.Error != nil {
			return fmt.Errorf("xyz dex order error (coin=%s, assetIndex=%d, size=%.4f, price=%.4f): %s", coin, assetIndex, roundedSize, roundedPrice, *status.Error)
		}
		if status.Filled != nil {
			logger.Infof("✅ xyz dex order filled: totalSz=%s avgPx=%s oid=%d",
				status.Filled.TotalSz, status.Filled.AvgPx, status.Filled.Oid)
		} else if status.Resting != nil {
			logger.Infof("✅ xyz dex order resting: oid=%d", status.Resting.Oid)
		}
	}

	logger.Infof("✅ xyz dex order placed successfully: %s (response: %s)", coin, string(body))
	return nil
}

// getXyzAssetIndex gets the asset index for an xyz dex asset
func (t *HyperliquidTrader) getXyzAssetIndex(baseCoin string) int {
	t.xyzMetaMutex.RLock()
	defer t.xyzMetaMutex.RUnlock()

	if t.xyzMeta == nil {
		return -1
	}

	// The meta API returns names with xyz: prefix, so ensure we match correctly
	lookupName := baseCoin
	if !strings.HasPrefix(lookupName, "xyz:") {
		lookupName = "xyz:" + lookupName
	}

	for i, asset := range t.xyzMeta.Universe {
		if asset.Name == lookupName {
			return i
		}
	}
	return -1
}

// placeXyzTriggerOrder places a trigger order (stop loss / take profit) on the xyz dex
// tpsl: "sl" for stop loss, "tp" for take profit
func (t *HyperliquidTrader) placeXyzTriggerOrder(coin string, isBuy bool, size float64, triggerPrice float64, tpsl string) error {
	// Fetch xyz meta if not cached
	t.xyzMetaMutex.RLock()
	hasMeta := t.xyzMeta != nil
	t.xyzMetaMutex.RUnlock()

	if !hasMeta {
		if err := t.fetchXyzMeta(); err != nil {
			return fmt.Errorf("failed to fetch xyz meta: %w", err)
		}
	}

	// Get asset index from xyz meta (returns 0-based index)
	metaIndex := t.getXyzAssetIndex(coin)
	if metaIndex < 0 {
		return fmt.Errorf("xyz asset %s not found in meta", coin)
	}

	// HIP-3 perp dex asset index formula: 100000 + perp_dex_index * 10000 + index_in_meta
	// xyz dex is at perp_dex_index = 1
	const xyzPerpDexIndex = 1
	assetIndex := 100000 + xyzPerpDexIndex*10000 + metaIndex

	// Round size to correct precision
	szDecimals := t.getXyzSzDecimals(coin)
	multiplier := 1.0
	for i := 0; i < szDecimals; i++ {
		multiplier *= 10.0
	}
	roundedSize := float64(int(size*multiplier+0.5)) / multiplier

	// Round price to 5 significant figures
	roundedPrice := t.roundPriceToSigfigs(triggerPrice)

	logger.Infof("📝 Placing xyz dex %s order: %s %s size=%.4f triggerPrice=%.4f assetIndex=%d",
		tpsl,
		map[bool]string{true: "BUY", false: "SELL"}[isBuy],
		coin, roundedSize, roundedPrice, assetIndex)

	// Construct OrderWire with trigger type for stop loss / take profit
	orderWire := hyperliquid.OrderWire{
		Asset:      assetIndex,
		IsBuy:      isBuy,
		LimitPx:    floatToWireStr(roundedPrice),
		Size:       floatToWireStr(roundedSize),
		ReduceOnly: true, // TP/SL orders are always reduce-only
		OrderType: hyperliquid.OrderWireType{
			Trigger: &hyperliquid.OrderWireTypeTrigger{
				TriggerPx: floatToWireStr(roundedPrice),
				IsMarket:  true,
				Tpsl:      hyperliquid.Tpsl(tpsl), // "sl" or "tp" - convert string to Tpsl type
			},
		},
	}

	// Create OrderAction (no builder to avoid requiring builder fee approval)
	action := hyperliquid.OrderAction{
		Type:     "order",
		Orders:   []hyperliquid.OrderWire{orderWire},
		Grouping: "na",
		Builder:  nil,
	}

	// Sign the action
	nonce := time.Now().UnixMilli()
	isMainnet := !t.isTestnet
	vaultAddress := ""

	sig, err := hyperliquid.SignL1Action(t.privateKey, action, vaultAddress, nonce, nil, isMainnet)
	if err != nil {
		return fmt.Errorf("failed to sign xyz dex trigger order: %w", err)
	}

	// Construct payload for /exchange endpoint
	payload := map[string]any{
		"action":    action,
		"nonce":     nonce,
		"signature": sig,
	}

	// Determine API URL
	apiURL := hyperliquid.MainnetAPIURL
	if t.isTestnet {
		apiURL = hyperliquid.TestnetAPIURL
	}

	// POST to /exchange
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	logger.Infof("📤 Sending xyz dex %s order to %s/exchange", tpsl, apiURL)

	req, err := http.NewRequestWithContext(t.ctx, http.MethodPost, apiURL+"/exchange", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response
	var result struct {
		Status   string `json:"status"`
		Response struct {
			Type string `json:"type"`
			Data struct {
				Statuses []struct {
					Resting *struct {
						Oid int64 `json:"oid"`
					} `json:"resting,omitempty"`
					Error *string `json:"error,omitempty"`
				} `json:"statuses"`
			} `json:"data"`
		} `json:"response"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		logger.Infof("⚠️  Failed to parse response, raw body: %s", string(body))
		return fmt.Errorf("xyz dex %s order failed, status=%d, body=%s", tpsl, resp.StatusCode, string(body))
	}

	// Check for errors in response
	if result.Status != "ok" {
		return fmt.Errorf("xyz dex %s order failed: status=%s, body=%s", tpsl, result.Status, string(body))
	}

	// Check order statuses
	if len(result.Response.Data.Statuses) > 0 {
		status := result.Response.Data.Statuses[0]
		if status.Error != nil {
			return fmt.Errorf("xyz dex %s order error: %s", tpsl, *status.Error)
		}
		if status.Resting != nil {
			logger.Infof("✅ xyz dex %s order placed: oid=%d", tpsl, status.Resting.Oid)
		}
	}

	logger.Infof("✅ xyz dex %s order placed successfully: %s", tpsl, coin)
	return nil
}

// SetStopLoss sets stop loss order
func (t *HyperliquidTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	coin := convertSymbolToHyperliquid(symbol)

	isBuy := positionSide == "SHORT" // Short position stop loss = buy, long position stop loss = sell

	// ⚠️ Critical: Price needs to be processed to 5 significant figures
	roundedStopPrice := t.roundPriceToSigfigs(stopPrice)

	// Check if this is an xyz dex asset (stocks, forex, commodities)
	isXyz := strings.HasPrefix(coin, "xyz:")

	if isXyz {
		// xyz dex stop loss order - use direct API call similar to placeXyzOrder
		if err := t.placeXyzTriggerOrder(coin, isBuy, quantity, roundedStopPrice, "sl"); err != nil {
			return fmt.Errorf("failed to set xyz dex stop loss: %w", err)
		}
	} else {
		// Standard crypto stop loss order
		// ⚠️ Critical: Round quantity according to coin precision requirements
		roundedQuantity := t.roundToSzDecimals(coin, quantity)

		// Create stop loss order (Trigger Order)
		order := hyperliquid.CreateOrderRequest{
			Coin:  coin,
			IsBuy: isBuy,
			Size:  roundedQuantity,  // Use rounded quantity
			Price: roundedStopPrice, // Use processed price
			OrderType: hyperliquid.OrderType{
				Trigger: &hyperliquid.TriggerOrderType{
					TriggerPx: roundedStopPrice,
					IsMarket:  true,
					Tpsl:      "sl", // stop loss
				},
			},
			ReduceOnly: true,
		}

		_, err := t.exchange.Order(t.ctx, order, defaultBuilder)
		if err != nil {
			return fmt.Errorf("failed to set stop loss: %w", err)
		}
	}

	logger.Infof("  Stop loss price set: %.4f", roundedStopPrice)
	return nil
}

// SetTakeProfit sets take profit order
func (t *HyperliquidTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	coin := convertSymbolToHyperliquid(symbol)

	isBuy := positionSide == "SHORT" // Short position take profit = buy, long position take profit = sell

	// ⚠️ Critical: Price needs to be processed to 5 significant figures
	roundedTakeProfitPrice := t.roundPriceToSigfigs(takeProfitPrice)

	// Check if this is an xyz dex asset (stocks, forex, commodities)
	isXyz := strings.HasPrefix(coin, "xyz:")

	if isXyz {
		// xyz dex take profit order - use direct API call similar to placeXyzOrder
		if err := t.placeXyzTriggerOrder(coin, isBuy, quantity, roundedTakeProfitPrice, "tp"); err != nil {
			return fmt.Errorf("failed to set xyz dex take profit: %w", err)
		}
	} else {
		// Standard crypto take profit order
		// ⚠️ Critical: Round quantity according to coin precision requirements
		roundedQuantity := t.roundToSzDecimals(coin, quantity)

		// Create take profit order (Trigger Order)
		order := hyperliquid.CreateOrderRequest{
			Coin:  coin,
			IsBuy: isBuy,
			Size:  roundedQuantity,        // Use rounded quantity
			Price: roundedTakeProfitPrice, // Use processed price
			OrderType: hyperliquid.OrderType{
				Trigger: &hyperliquid.TriggerOrderType{
					TriggerPx: roundedTakeProfitPrice,
					IsMarket:  true,
					Tpsl:      "tp", // take profit
				},
			},
			ReduceOnly: true,
		}

		_, err := t.exchange.Order(t.ctx, order, defaultBuilder)
		if err != nil {
			return fmt.Errorf("failed to set take profit: %w", err)
		}
	}

	logger.Infof("  Take profit price set: %.4f", roundedTakeProfitPrice)
	return nil
}

// FormatQuantity formats quantity to correct precision
func (t *HyperliquidTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	coin := convertSymbolToHyperliquid(symbol)
	szDecimals := t.getSzDecimals(coin)

	// Format quantity using szDecimals
	formatStr := fmt.Sprintf("%%.%df", szDecimals)
	return fmt.Sprintf(formatStr, quantity), nil
}

// getSzDecimals gets quantity precision for coin
func (t *HyperliquidTrader) getSzDecimals(coin string) int {
	// ✅ Concurrency safe: Use read lock to protect meta field access
	t.metaMutex.RLock()
	defer t.metaMutex.RUnlock()

	if t.meta == nil {
		logger.Infof("⚠️  meta information is empty, using default precision 4")
		return 4 // Default precision
	}

	// Find corresponding coin in meta.Universe
	for _, asset := range t.meta.Universe {
		if asset.Name == coin {
			return asset.SzDecimals
		}
	}

	logger.Infof("⚠️  Precision information not found for %s, using default precision 4", coin)
	return 4 // Default precision
}

// roundToSzDecimals rounds quantity to correct precision
func (t *HyperliquidTrader) roundToSzDecimals(coin string, quantity float64) float64 {
	szDecimals := t.getSzDecimals(coin)

	// Calculate multiplier (10^szDecimals)
	multiplier := 1.0
	for i := 0; i < szDecimals; i++ {
		multiplier *= 10.0
	}

	// Round
	return float64(int(quantity*multiplier+0.5)) / multiplier
}

// roundPriceToSigfigs rounds price to 5 significant figures
// Hyperliquid requires prices to use 5 significant figures
func (t *HyperliquidTrader) roundPriceToSigfigs(price float64) float64 {
	if price == 0 {
		return 0
	}

	const sigfigs = 5 // Hyperliquid standard: 5 significant figures

	// Calculate price magnitude
	var magnitude float64
	if price < 0 {
		magnitude = -price
	} else {
		magnitude = price
	}

	// Calculate required multiplier
	multiplier := 1.0
	for magnitude >= 10 {
		magnitude /= 10
		multiplier /= 10
	}
	for magnitude < 1 {
		magnitude *= 10
		multiplier *= 10
	}

	// Apply significant figures precision
	for i := 0; i < sigfigs-1; i++ {
		multiplier *= 10
	}

	// Round
	rounded := float64(int(price*multiplier+0.5)) / multiplier
	return rounded
}

// convertSymbolToHyperliquid converts standard symbol to Hyperliquid format
// Example: "BTCUSDT" -> "BTC", "TSLA" -> "xyz:TSLA", "silver" -> "xyz:SILVER"
func convertSymbolToHyperliquid(symbol string) string {
	// Convert to uppercase for consistent handling
	base := strings.ToUpper(symbol)

	// Remove common suffixes to get base symbol
	for _, suffix := range []string{"USDT", "USD", "-USDC", "-USD"} {
		if strings.HasSuffix(base, suffix) {
			base = strings.TrimSuffix(base, suffix)
			break
		}
	}
	// Remove xyz: prefix if present (case-insensitive, will be re-added if needed)
	if strings.HasPrefix(strings.ToLower(base), "xyz:") {
		base = base[4:] // Remove first 4 characters
	}

	// Check if this is an xyz dex asset (stocks, forex, commodities)
	if isXyzDexAsset(base) {
		return "xyz:" + base
	}
	return base
}

// GetOrderStatus gets order status
// Hyperliquid uses IOC orders, usually filled or cancelled immediately
// For completed orders, need to query historical records
func (t *HyperliquidTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	// Hyperliquid's IOC orders are completed almost immediately
	// If order was placed through this system, returned status will be FILLED
	// Try to query open orders to determine if still pending
	coin := convertSymbolToHyperliquid(symbol)

	// First check if in open orders
	openOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
	if err != nil {
		// If query fails, assume order is completed
		return map[string]interface{}{
			"orderId":     orderID,
			"status":      "FILLED",
			"avgPrice":    0.0,
			"executedQty": 0.0,
			"commission":  0.0,
		}, nil
	}

	// Check if order is in open orders list
	for _, order := range openOrders {
		if order.Coin == coin && fmt.Sprintf("%d", order.Oid) == orderID {
			// Order is still pending
			return map[string]interface{}{
				"orderId":     orderID,
				"status":      "NEW",
				"avgPrice":    0.0,
				"executedQty": 0.0,
				"commission":  0.0,
			}, nil
		}
	}

	// Order not in open list, meaning completed or cancelled
	// Hyperliquid IOC orders not in open list are usually filled
	return map[string]interface{}{
		"orderId":     orderID,
		"status":      "FILLED",
		"avgPrice":    0.0, // Hyperliquid does not directly return execution price, need to get from position info
		"executedQty": 0.0,
		"commission":  0.0,
	}, nil
}

// absFloat returns absolute value of float
func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// GetClosedPnL gets recent closing trades from Hyperliquid
// Note: Hyperliquid does NOT have a position history API, only fill history.
// This returns individual closing trades for real-time position closure detection.
func (t *HyperliquidTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
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

		// Determine side (Hyperliquid uses one-way mode)
		side := "long"
		if trade.Side == "SELL" || trade.Side == "Sell" {
			side = "long" // Selling closes long
		} else {
			side = "short" // Buying closes short
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

// GetTrades retrieves trade history from Hyperliquid
func (t *HyperliquidTrader) GetTrades(startTime time.Time, limit int) ([]types.TradeRecord, error) {
	// Use UserFillsByTime API
	startTimeMs := startTime.UnixMilli()
	fills, err := t.exchange.Info().UserFillsByTime(t.ctx, t.walletAddr, startTimeMs, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get user fills: %w", err)
	}

	var trades []types.TradeRecord
	for _, fill := range fills {
		price, _ := strconv.ParseFloat(fill.Price, 64)
		qty, _ := strconv.ParseFloat(fill.Size, 64)
		fee, _ := strconv.ParseFloat(fill.Fee, 64)
		pnl, _ := strconv.ParseFloat(fill.ClosedPnl, 64)

		// Determine side: "B" = Buy, "S" = Sell (or "A" = Ask, "B" = Bid)
		var side string
		if fill.Side == "B" || fill.Side == "Buy" || fill.Side == "bid" {
			side = "BUY"
		} else {
			side = "SELL"
		}

		// Parse Dir field to get order action
		// Hyperliquid Dir values: "Open Long", "Open Short", "Close Long", "Close Short"
		var orderAction string
		switch strings.ToLower(fill.Dir) {
		case "open long":
			orderAction = "open_long"
		case "open short":
			orderAction = "open_short"
		case "close long":
			orderAction = "close_long"
		case "close short":
			orderAction = "close_short"
		default:
			// Fallback: use RealizedPnL if Dir is missing/unknown
			if pnl != 0 {
				if side == "BUY" {
					orderAction = "close_short"
				} else {
					orderAction = "close_long"
				}
			} else {
				if side == "BUY" {
					orderAction = "open_long"
				} else {
					orderAction = "open_short"
				}
			}
		}

		// Hyperliquid uses one-way mode, so PositionSide is "BOTH"
		trade := types.TradeRecord{
			TradeID:      strconv.FormatInt(fill.Tid, 10),
			Symbol:       fill.Coin,
			Side:         side,
			PositionSide: "BOTH", // Hyperliquid doesn't have hedge mode
			OrderAction:  orderAction,
			Price:        price,
			Quantity:     qty,
			RealizedPnL:  pnl,
			Fee:          fee,
			Time:         time.UnixMilli(fill.Time).UTC(),
		}
		trades = append(trades, trade)
	}

	return trades, nil
}

// defaultBuilder is the builder info for order routing
// Set to nil to avoid requiring builder fee approval
//
//	var defaultBuilder = &hyperliquid.BuilderInfo{
//		Builder: "0x891dc6f05ad47a3c1a05da55e7a7517971faaf0d",
//		Fee:     10,
//	}
var defaultBuilder *hyperliquid.BuilderInfo = nil

// GetOpenOrders gets all open/pending orders for a symbol
func (t *HyperliquidTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	openOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get open orders: %w", err)
	}

	var result []types.OpenOrder
	for _, order := range openOrders {
		if order.Coin != symbol {
			continue
		}

		side := "BUY"
		if order.Side == "A" {
			side = "SELL"
		}

		result = append(result, types.OpenOrder{
			OrderID:      fmt.Sprintf("%d", order.Oid),
			Symbol:       order.Coin,
			Side:         side,
			PositionSide: "",
			Type:         "LIMIT",
			Price:        order.LimitPx,
			StopPrice:    0,
			Quantity:     order.Size,
			Status:       "NEW",
		})
	}

	return result, nil
}

// PlaceLimitOrder places a limit order for grid trading
// Implements GridTrader interface
func (t *HyperliquidTrader) PlaceLimitOrder(req *types.LimitOrderRequest) (*types.LimitOrderResult, error) {
	coin := convertSymbolToHyperliquid(req.Symbol)

	// Set leverage if specified and not xyz dex
	isXyz := strings.HasPrefix(coin, "xyz:")
	if req.Leverage > 0 && !isXyz {
		if err := t.SetLeverage(req.Symbol, req.Leverage); err != nil {
			logger.Warnf("[Hyperliquid] Failed to set leverage: %v", err)
		}
	}

	// Round quantity to allowed decimals
	roundedQuantity := t.roundToSzDecimals(coin, req.Quantity)

	// Round price to 5 significant figures
	roundedPrice := t.roundPriceToSigfigs(req.Price)

	// Determine if buy or sell
	isBuy := req.Side == "BUY"

	logger.Infof("[Hyperliquid] PlaceLimitOrder: %s %s @ %.4f, qty=%.4f", coin, req.Side, roundedPrice, roundedQuantity)

	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: isBuy,
		Size:  roundedQuantity,
		Price: roundedPrice,
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifGtc, // Good Till Cancel for grid orders
			},
		},
		ReduceOnly: req.ReduceOnly,
	}

	_, err := t.exchange.Order(t.ctx, order, defaultBuilder)
	if err != nil {
		return nil, fmt.Errorf("failed to place limit order: %w", err)
	}

	// Note: Hyperliquid's Order response doesn't return the order ID directly
	// We would need to query open orders to get it, but for grid trading
	// we can track orders by price level instead
	orderID := fmt.Sprintf("%d", time.Now().UnixNano())

	logger.Infof("✓ [Hyperliquid] Limit order placed: %s %s @ %.4f",
		coin, req.Side, roundedPrice)

	return &types.LimitOrderResult{
		OrderID:      orderID,
		ClientID:     req.ClientID,
		Symbol:       req.Symbol,
		Side:         req.Side,
		PositionSide: req.PositionSide,
		Price:        roundedPrice,
		Quantity:     roundedQuantity,
		Status:       "NEW",
	}, nil
}

// CancelOrder cancels a specific order by ID
// Implements GridTrader interface
func (t *HyperliquidTrader) CancelOrder(symbol, orderID string) error {
	coin := convertSymbolToHyperliquid(symbol)

	// Parse order ID
	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid order ID: %w", err)
	}

	_, err = t.exchange.Cancel(t.ctx, coin, oid)
	if err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	logger.Infof("✓ [Hyperliquid] Order cancelled: %s %s", symbol, orderID)
	return nil
}

// GetOrderBook gets the order book for a symbol
// Implements GridTrader interface
func (t *HyperliquidTrader) GetOrderBook(symbol string, depth int) (bids, asks [][]float64, err error) {
	coin := convertSymbolToHyperliquid(symbol)

	l2Book, err := t.exchange.Info().L2Snapshot(t.ctx, coin)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get order book: %w", err)
	}

	if l2Book == nil || len(l2Book.Levels) < 2 {
		return nil, nil, fmt.Errorf("invalid order book data")
	}

	// Parse bids (first level array)
	for i, level := range l2Book.Levels[0] {
		if i >= depth {
			break
		}
		bids = append(bids, []float64{level.Px, level.Sz})
	}

	// Parse asks (second level array)
	for i, level := range l2Book.Levels[1] {
		if i >= depth {
			break
		}
		asks = append(asks, []float64{level.Px, level.Sz})
	}

	return bids, asks, nil
}
