package trader

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"nofx/logger"
	"net/http"
	"strings"
	"sync"
	"time"

	lighterClient "github.com/elliottech/lighter-go/client"
	lighterHTTP "github.com/elliottech/lighter-go/client/http"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// AccountInfo LIGHTER account information
type AccountInfo struct {
	AccountIndex int64  `json:"account_index"`
	L1Address    string `json:"l1_address"`
	// Other fields can be added based on actual API response
}

// LighterTraderV2 New implementation using official lighter-go SDK
type LighterTraderV2 struct {
	ctx        context.Context
	privateKey *ecdsa.PrivateKey // L1 wallet private key (for account identification)
	walletAddr string            // Ethereum wallet address

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
	marketIndexMap map[string]uint8 // symbol -> market_id
	marketMutex    sync.RWMutex
}

// NewLighterTraderV2 Create new LIGHTER trader (using official SDK)
// Parameters:
//   - l1PrivateKeyHex: L1 wallet private key (32 bytes, standard Ethereum private key)
//   - walletAddr: Ethereum wallet address (optional, will be derived from private key if empty)
//   - apiKeyPrivateKeyHex: API Key private key (40 bytes, for signing transactions) - needs generation if empty
//   - testnet: Whether to use testnet
func NewLighterTraderV2(l1PrivateKeyHex, walletAddr, apiKeyPrivateKeyHex string, testnet bool) (*LighterTraderV2, error) {
	// 1. Parse L1 private key
	l1PrivateKeyHex = strings.TrimPrefix(strings.ToLower(l1PrivateKeyHex), "0x")
	l1PrivateKey, err := crypto.HexToECDSA(l1PrivateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid L1 private key: %w", err)
	}

	// 2. If wallet address not provided, derive from private key
	if walletAddr == "" {
		walletAddr = crypto.PubkeyToAddress(*l1PrivateKey.Public().(*ecdsa.PublicKey)).Hex()
		logger.Infof("✓ Derived wallet address from private key: %s", walletAddr)
	}

	// 3. Determine API URL and Chain ID
	baseURL := "https://mainnet.zklighter.elliot.ai"
	chainID := uint32(42766) // Mainnet Chain ID
	if testnet {
		baseURL = "https://testnet.zklighter.elliot.ai"
		chainID = uint32(42069) // Testnet Chain ID
	}

	// 4. Create HTTP client
	httpClient := lighterHTTP.NewClient(baseURL)

	trader := &LighterTraderV2{
		ctx:              context.Background(),
		privateKey:       l1PrivateKey,
		walletAddr:       walletAddr,
		client:           &http.Client{Timeout: 30 * time.Second},
		baseURL:          baseURL,
		testnet:          testnet,
		chainID:          chainID,
		httpClient:       httpClient,
		apiKeyPrivateKey: apiKeyPrivateKeyHex,
		apiKeyIndex:      0, // Default to index 0
		symbolPrecision:  make(map[string]SymbolPrecision),
		marketIndexMap:   make(map[string]uint8),
	}

	// 5. Initialize account (get account index)
	if err := trader.initializeAccount(); err != nil {
		return nil, fmt.Errorf("failed to initialize account: %w", err)
	}

	// 6. If no API Key, prompt user to generate one
	if apiKeyPrivateKeyHex == "" {
		logger.Infof("⚠️  No API Key private key provided, please call GenerateAndRegisterAPIKey() to generate")
		logger.Infof("   Or get an existing API Key from LIGHTER website")
		return trader, nil
	}

	// 7. Create TxClient (for signing transactions)
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

	// 8. Verify API Key is correct
	if err := trader.checkClient(); err != nil {
		logger.Infof("⚠️  API Key verification failed: %v", err)
		logger.Infof("   You may need to regenerate API Key or check configuration")
		return trader, err
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
func (t *LighterTraderV2) getAccountByL1Address() (*AccountInfo, error) {
	endpoint := fmt.Sprintf("%s/api/v1/account?by=address&value=%s", t.baseURL, t.walletAddr)

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
		return nil, fmt.Errorf("failed to get account (status %d): %s", resp.StatusCode, string(body))
	}

	var accountInfo AccountInfo
	if err := json.Unmarshal(body, &accountInfo); err != nil {
		return nil, fmt.Errorf("failed to parse account response: %w", err)
	}

	return &accountInfo, nil
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
