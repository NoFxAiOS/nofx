package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"nofx/logger"
	"nofx/store"
	"nofx/trader"
	"nofx/trader/aster"
	"nofx/trader/binance"
	"nofx/trader/binance_stocks"
	"nofx/trader/bitget"
	"nofx/trader/bybit"
	"nofx/trader/gate"
	hyperliquidtrader "nofx/trader/hyperliquid"
	"nofx/trader/indodax"
	"nofx/trader/kucoin"
	"nofx/trader/lighter"
	"nofx/trader/okx"

	"github.com/gin-gonic/gin"
)

const exchangeAccountStateCacheTTL = 30 * time.Second

const (
	exchangeAccountStatusOK                 = "ok"
	exchangeAccountStatusDisabled           = "disabled"
	exchangeAccountStatusMissingCredentials = "missing_credentials"
	exchangeAccountStatusInvalidCredentials = "invalid_credentials"
	exchangeAccountStatusPermissionDenied   = "permission_denied"
	exchangeAccountStatusUnavailable        = "unavailable"
)

type ExchangeAccountState struct {
	ExchangeID       string    `json:"exchange_id"`
	Status           string    `json:"status"`
	DisplayBalance   string    `json:"display_balance,omitempty"`
	Asset            string    `json:"asset,omitempty"`
	UnrealizedPnL    float64   `json:"unrealized_pnl,omitempty"`
	TotalEquity      float64   `json:"total_equity,omitempty"`
	AvailableBalance float64   `json:"available_balance,omitempty"`
	CheckedAt        time.Time `json:"checked_at"`
	ErrorCode        string    `json:"error_code,omitempty"`
	ErrorMessage     string    `json:"error_message,omitempty"`
}

type cachedExchangeAccountStates struct {
	states   map[string]ExchangeAccountState
	cachedAt time.Time
}

type ExchangeAccountStateCache struct {
	entries map[string]cachedExchangeAccountStates
	mu      sync.RWMutex
}

func NewExchangeAccountStateCache() *ExchangeAccountStateCache {
	return &ExchangeAccountStateCache{
		entries: make(map[string]cachedExchangeAccountStates),
	}
}

func (c *ExchangeAccountStateCache) Get(userID string) (map[string]ExchangeAccountState, bool) {
	c.mu.RLock()
	entry, ok := c.entries[userID]
	c.mu.RUnlock()
	if !ok || time.Since(entry.cachedAt) >= exchangeAccountStateCacheTTL {
		return nil, false
	}
	return cloneExchangeAccountStates(entry.states), true
}

func (c *ExchangeAccountStateCache) Set(userID string, states map[string]ExchangeAccountState) {
	c.mu.Lock()
	c.entries[userID] = cachedExchangeAccountStates{
		states:   cloneExchangeAccountStates(states),
		cachedAt: time.Now(),
	}
	c.mu.Unlock()
}

func (c *ExchangeAccountStateCache) Invalidate(userID string) {
	c.mu.Lock()
	delete(c.entries, userID)
	c.mu.Unlock()
}

func cloneExchangeAccountStates(states map[string]ExchangeAccountState) map[string]ExchangeAccountState {
	cloned := make(map[string]ExchangeAccountState, len(states))
	for id, state := range states {
		cloned[id] = state
	}
	return cloned
}

func (s *Server) handleGetExchangeAccountStates(c *gin.Context) {
	userID := c.GetString("user_id")

	states, err := s.getExchangeAccountStates(userID)
	if err != nil {
		SafeInternalError(c, "Failed to get exchange account states", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"states": states})
}

func (s *Server) getExchangeAccountStates(userID string) (map[string]ExchangeAccountState, error) {
	if cached, ok := s.exchangeAccountStateCache.Get(userID); ok {
		return cached, nil
	}

	exchanges, err := s.store.Exchange().List(userID)
	if err != nil {
		return nil, err
	}

	states := make(map[string]ExchangeAccountState, len(exchanges))
	if len(exchanges) == 0 {
		return states, nil
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, exchangeCfg := range exchanges {
		exchangeCfg := exchangeCfg
		wg.Add(1)
		go func() {
			defer wg.Done()
			state := probeExchangeAccountState(exchangeCfg, userID, s.store)
			mu.Lock()
			states[exchangeCfg.ID] = state
			mu.Unlock()
		}()
	}

	wg.Wait()
	s.exchangeAccountStateCache.Set(userID, states)

	return cloneExchangeAccountStates(states), nil
}

func probeExchangeAccountState(exchangeCfg *store.Exchange, userID string, st *store.Store) ExchangeAccountState {
	state := ExchangeAccountState{
		ExchangeID: exchangeCfg.ID,
		CheckedAt:  time.Now().UTC(),
		Asset:      accountAssetForExchange(exchangeCfg.ExchangeType),
	}

	if !exchangeCfg.Enabled {
		state.Status = exchangeAccountStatusDisabled
		state.ErrorCode = "EXCHANGE_DISABLED"
		state.ErrorMessage = "Exchange account is disabled"
		return state
	}

	if exchangeCfg.ExchangeType == "paper" {
		equity, available, unrealized, err := paperAccountBalance(st, exchangeCfg)
		if err != nil {
			state.Status = exchangeAccountStatusUnavailable
			state.ErrorCode = "PAPER_LEDGER_UNAVAILABLE"
			state.ErrorMessage = "无法读取模拟盘账本"
			return state
		}
		state.Status = exchangeAccountStatusOK
		state.TotalEquity, state.AvailableBalance, state.UnrealizedPnL = equity, available, unrealized
		state.DisplayBalance = formatDisplayBalance(equity, state.Asset)
		return state
	}

	if status, code, message, missing := missingExchangeCredentials(exchangeCfg); missing {
		state.Status = status
		state.ErrorCode = code
		state.ErrorMessage = message
		return state
	}

	tempTrader, err := buildExchangeProbeTrader(exchangeCfg, userID)
	if err != nil {
		status, code, message := classifyExchangeProbeError(err)
		state.Status = status
		state.ErrorCode = code
		state.ErrorMessage = message
		return state
	}

	balanceInfo, err := tempTrader.GetBalance()
	if err != nil {
		status, code, message := classifyExchangeProbeError(err)
		state.Status = status
		state.ErrorCode = code
		state.ErrorMessage = message
		logger.Infof("⚠️ Failed to probe exchange account %s (%s): %v", exchangeCfg.ID, exchangeCfg.ExchangeType, err)
		return state
	}

	totalEquity, totalFound := extractFirstNumeric(balanceInfo,
		"total_equity", "totalEquity", "totalWalletBalance", "wallet_balance", "totalEq", "balance")
	availableBalance, availableFound := extractFirstNumeric(balanceInfo,
		"available_balance", "availableBalance", "available")

	if !totalFound && availableFound {
		totalEquity = availableBalance
		totalFound = true
	}

	if !availableFound && totalFound {
		availableBalance = totalEquity
		availableFound = true
	}

	if !totalFound && !availableFound {
		state.Status = exchangeAccountStatusUnavailable
		state.ErrorCode = "BALANCE_NOT_FOUND"
		state.ErrorMessage = "Connected but no balance fields were returned"
		return state
	}

	state.Status = exchangeAccountStatusOK
	if totalFound {
		state.TotalEquity = totalEquity
		state.DisplayBalance = formatDisplayBalance(totalEquity, state.Asset)
	}
	if availableFound {
		state.AvailableBalance = availableBalance
		if state.DisplayBalance == "" {
			state.DisplayBalance = formatDisplayBalance(availableBalance, state.Asset)
		}
	}

	return state
}

func buildExchangeProbeTrader(exchangeCfg *store.Exchange, userID string) (trader.Trader, error) {
	if err := store.ValidateExchangeEnvironment(exchangeCfg.ExchangeType, exchangeCfg.Testnet); err != nil {
		return nil, err
	}
	switch exchangeCfg.ExchangeType {
	case "binance_stocks":
		return binance_stocks.NewStocksTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey)), nil
	case "binance":
		return binance.NewFuturesTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), userID), nil
	case "bybit":
		return bybit.NewBybitTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey)), nil
	case "okx":
		return okx.NewOKXTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), string(exchangeCfg.Passphrase)), nil
	case "bitget":
		return bitget.NewBitgetTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), string(exchangeCfg.Passphrase)), nil
	case "gate":
		return gate.NewGateTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey)), nil
	case "kucoin":
		return kucoin.NewKuCoinTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), string(exchangeCfg.Passphrase)), nil
	case "indodax":
		return indodax.NewIndodaxTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey)), nil
	case "hyperliquid":
		return hyperliquidtrader.NewHyperliquidTrader(
			string(exchangeCfg.APIKey),
			exchangeCfg.HyperliquidWalletAddr,
			exchangeCfg.Testnet,
			exchangeCfg.HyperliquidUnifiedAcct,
		)
	case "aster":
		return aster.NewAsterTrader(
			exchangeCfg.AsterUser,
			exchangeCfg.AsterSigner,
			string(exchangeCfg.AsterPrivateKey),
		)
	case "lighter":
		return lighter.NewLighterTraderV2(
			exchangeCfg.LighterWalletAddr,
			string(exchangeCfg.LighterAPIKeyPrivateKey),
			exchangeCfg.LighterAPIKeyIndex,
			exchangeCfg.Testnet,
		)
	default:
		return nil, fmt.Errorf("unsupported exchange type: %s", exchangeCfg.ExchangeType)
	}
}

func extractExchangeTotalEquity(balanceInfo map[string]interface{}) (float64, bool) {
	return extractFirstNumeric(balanceInfo,
		"total_equity", "totalEquity", "totalWalletBalance", "wallet_balance", "totalEq", "balance")
}

func extractFirstNumeric(values map[string]interface{}, keys ...string) (float64, bool) {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok {
			continue
		}

		switch v := raw.(type) {
		case float64:
			return v, true
		case float32:
			return float64(v), true
		case int:
			return float64(v), true
		case int64:
			return float64(v), true
		case int32:
			return float64(v), true
		case string:
			parsed, err := strconv.ParseFloat(v, 64)
			if err == nil {
				return parsed, true
			}
		}
	}

	return 0, false
}

func formatDisplayBalance(value float64, asset string) string {
	formatted := strconv.FormatFloat(value, 'f', 4, 64)
	formatted = strings.TrimRight(strings.TrimRight(formatted, "0"), ".")
	if formatted == "" {
		formatted = "0"
	}
	if asset == "" {
		return formatted
	}
	return fmt.Sprintf("%s %s", formatted, asset)
}

func accountAssetForExchange(exchangeType string) string {
	switch exchangeType {
	case "hyperliquid", "aster", "lighter":
		return "USDC"
	default:
		return "USDT"
	}
}

func missingExchangeCredentials(exchangeCfg *store.Exchange) (status string, code string, message string, missing bool) {
	switch exchangeCfg.ExchangeType {
	case "paper":
		return "", "", "", false
	case "binance", "bybit", "gate", "indodax":
		if exchangeCfg.APIKey == "" || exchangeCfg.SecretKey == "" {
			return exchangeAccountStatusMissingCredentials, "MISSING_REQUIRED_FIELDS", "API key and secret key are required", true
		}
	case "okx", "bitget", "kucoin":
		if exchangeCfg.APIKey == "" || exchangeCfg.SecretKey == "" || exchangeCfg.Passphrase == "" {
			return exchangeAccountStatusMissingCredentials, "MISSING_REQUIRED_FIELDS", "API key, secret key, and passphrase are required", true
		}
	case "hyperliquid":
		if exchangeCfg.APIKey == "" || exchangeCfg.HyperliquidWalletAddr == "" {
			return exchangeAccountStatusMissingCredentials, "MISSING_REQUIRED_FIELDS", "Private key and wallet address are required", true
		}
	case "aster":
		if exchangeCfg.AsterUser == "" || exchangeCfg.AsterSigner == "" || exchangeCfg.AsterPrivateKey == "" {
			return exchangeAccountStatusMissingCredentials, "MISSING_REQUIRED_FIELDS", "Aster user, signer, and private key are required", true
		}
	case "lighter":
		if exchangeCfg.LighterWalletAddr == "" || exchangeCfg.LighterAPIKeyPrivateKey == "" {
			return exchangeAccountStatusMissingCredentials, "MISSING_REQUIRED_FIELDS", "Wallet address and API key private key are required", true
		}
	default:
		return exchangeAccountStatusUnavailable, "UNSUPPORTED_EXCHANGE", "Unsupported exchange type", true
	}

	return "", "", "", false
}

func classifyExchangeProbeError(err error) (status string, code string, message string) {
	if err == nil {
		return exchangeAccountStatusOK, "", ""
	}

	rawMessage := err.Error()
	msg := strings.ToLower(rawMessage)

	switch {
	case strings.Contains(msg, "unsupported exchange type"):
		return exchangeAccountStatusUnavailable, "UNSUPPORTED_EXCHANGE", "Unsupported exchange type"
	case strings.Contains(msg, "requires ") || strings.Contains(msg, "missing") || strings.Contains(msg, "empty"):
		return exchangeAccountStatusMissingCredentials, "MISSING_REQUIRED_FIELDS", "Exchange credentials are incomplete"
	case strings.Contains(msg, "permission") || strings.Contains(msg, "forbidden") || strings.Contains(msg, "no authority") || strings.Contains(msg, "not allowed"):
		return exchangeAccountStatusPermissionDenied, "PERMISSION_DENIED", "Exchange account has no permission to read balances"
	case strings.Contains(msg, "invalid") || strings.Contains(msg, "signature") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "api key") || strings.Contains(msg, "api-key") || strings.Contains(msg, "auth"):
		return exchangeAccountStatusInvalidCredentials, "INVALID_CREDENTIALS", "Exchange credentials are invalid"
	default:
		return exchangeAccountStatusUnavailable, "EXCHANGE_UNAVAILABLE", limitErrorMessage(rawMessage)
	}
}

func limitErrorMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "Unable to fetch exchange balance right now"
	}
	if len(message) <= 160 {
		return message
	}
	return message[:157] + "..."
}

// paperAccountBalance reads the persisted snapshot without advancing matching or
// constructing another executor. A never-started account displays its seed cash.
func paperAccountBalance(st *store.Store, ex *store.Exchange) (equity, available, unrealized float64, err error) {
	cfg, err := ex.ParsePaperConfig()
	if err != nil {
		return 0, 0, 0, err
	}
	ledger, err := st.PaperLedger().Get(ex.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return cfg.InitialBalance, cfg.InitialBalance, 0, nil
	}
	if err != nil {
		return 0, 0, 0, err
	}
	var snapshot struct {
		Marks     map[string]float64
		Positions map[string]struct {
			Symbol, Side         string
			Quantity, EntryPrice float64
			Leverage             int
		}
	}
	if err := json.Unmarshal([]byte(ledger.State), &snapshot); err != nil {
		return 0, 0, 0, err
	}
	margin := 0.0
	for _, pos := range snapshot.Positions {
		mark := snapshot.Marks[pos.Symbol]
		if pos.Leverage <= 0 || mark <= 0 {
			return 0, 0, 0, fmt.Errorf("模拟盘账本持仓数据无效")
		}
		direction := 1.0
		if pos.Side == "SHORT" {
			direction = -1
		}
		unrealized += direction * (mark - pos.EntryPrice) * pos.Quantity
		margin += mark * pos.Quantity / float64(pos.Leverage)
	}
	equity, available = ledger.WalletBalance+unrealized, ledger.WalletBalance-margin
	for _, value := range []float64{equity, available, unrealized} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, 0, 0, fmt.Errorf("模拟盘账本余额无效")
		}
	}
	return equity, available, unrealized, nil
}
