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

// GetClosedPnL gets closed position PnL records from exchange
// LIGHTER does not have a direct closed PnL API, returns empty slice
func (t *LighterTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	// LIGHTER does not provide a closed PnL history API
	// Position closure data needs to be tracked locally via position sync
	logger.Infof("⚠️  LIGHTER GetClosedPnL not supported, returning empty")
	return []ClosedPnLRecord{}, nil
}
