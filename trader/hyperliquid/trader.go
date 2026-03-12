package hyperliquid

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"nofx/logger"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sonirico/go-hyperliquid"
)

// HyperliquidTrader Hyperliquid trader
type HyperliquidTrader struct {
	exchange         *hyperliquid.Exchange
	ctx              context.Context
	walletAddr       string
	meta             *hyperliquid.Meta // Cache meta information (including precision)
	metaMutex        sync.RWMutex      // Protect concurrent access to meta field
	isCrossMargin    bool              // Whether to use cross margin mode
	isUnifiedAccount bool              // Whether to use Unified Account mode (Spot as collateral for Perps)
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

// defaultBuilder is the builder info for order routing
// Set to nil to avoid requiring builder fee approval
//
//	var defaultBuilder = &hyperliquid.BuilderInfo{
//		Builder: "0x891dc6f05ad47a3c1a05da55e7a7517971faaf0d",
//		Fee:     10,
//	}
var defaultBuilder *hyperliquid.BuilderInfo = nil

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

// absFloat returns absolute value of float
func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// NewHyperliquidTrader creates a Hyperliquid trader
// unifiedAccount: when true, Spot USDC balance is used as collateral for Perp trading
func NewHyperliquidTrader(privateKeyHex string, walletAddr string, testnet bool, unifiedAccount bool) (*HyperliquidTrader, error) {
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

	// Security check: Validate Agent wallet balance (should be close to 0)
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

	if unifiedAccount {
		logger.Infof("✓ Unified Account mode enabled: Spot USDC will be used as collateral for Perp trading")
	}

	return &HyperliquidTrader{
		exchange:         exchange,
		ctx:              ctx,
		walletAddr:       walletAddr,
		meta:             meta,
		isCrossMargin:    true,           // Use cross margin mode by default
		isUnifiedAccount: unifiedAccount, // Unified Account: Spot as Perp collateral
		privateKey:       privateKey,
		isTestnet:        testnet,
	}, nil
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
	// Concurrency safe: Use read lock to protect meta field access
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
