package bybit

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"nofx/logger"
	"strconv"
	"strings"
	"sync"
	"time"

	bybit "github.com/bybit-exchange/bybit.go.api"
)

// BybitTrader Bybit USDT Perpetual Futures Trader
type BybitTrader struct {
	client    *bybit.Client
	apiKey    string
	secretKey string

	// Balance cache
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// Position cache
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// Trading pair precision cache (symbol -> qtyStep)
	qtyStepCache      map[string]float64
	qtyStepCacheMutex sync.RWMutex

	// Cache duration (15 seconds)
	cacheDuration time.Duration
}

// NewBybitTrader creates a Bybit trader
func NewBybitTrader(apiKey, secretKey string) *BybitTrader {
	const src = "Up000938"

	client := bybit.NewBybitHttpClient(apiKey, secretKey, bybit.WithBaseURL(bybit.MAINNET))

	// Set HTTP transport
	if client != nil && client.HTTPClient != nil {
		defaultTransport := client.HTTPClient.Transport
		if defaultTransport == nil {
			defaultTransport = http.DefaultTransport
		}

		client.HTTPClient.Transport = &headerRoundTripper{
			base:      defaultTransport,
			refererID: src,
		}
	}

	trader := &BybitTrader{
		client:        client,
		apiKey:        apiKey,
		secretKey:     secretKey,
		cacheDuration: 15 * time.Second,
		qtyStepCache:  make(map[string]float64),
	}

	logger.Infof("🔵 [Bybit] Trader initialized")

	return trader
}

// headerRoundTripper HTTP RoundTripper for adding custom headers
type headerRoundTripper struct {
	base      http.RoundTripper
	refererID string
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Referer", h.refererID)
	return h.base.RoundTrip(req)
}

// getQtyStep retrieves the quantity step for a trading pair
func (t *BybitTrader) getQtyStep(symbol string) float64 {
	// Check cache first
	t.qtyStepCacheMutex.RLock()
	if step, ok := t.qtyStepCache[symbol]; ok {
		t.qtyStepCacheMutex.RUnlock()
		return step
	}
	t.qtyStepCacheMutex.RUnlock()

	// Call public API directly to get contract information
	url := fmt.Sprintf("https://api.bybit.com/v5/market/instruments-info?category=linear&symbol=%s", symbol)
	resp, err := http.Get(url)
	if err != nil {
		logger.Infof("⚠️ [Bybit] Failed to get precision info for %s: %v", symbol, err)
		return 1 // Default to integer
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 1
	}

	var result struct {
		RetCode int `json:"retCode"`
		Result  struct {
			List []struct {
				LotSizeFilter struct {
					QtyStep string `json:"qtyStep"`
				} `json:"lotSizeFilter"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 1
	}

	if result.RetCode != 0 || len(result.Result.List) == 0 {
		return 1
	}

	qtyStep, _ := strconv.ParseFloat(result.Result.List[0].LotSizeFilter.QtyStep, 64)
	if qtyStep <= 0 {
		qtyStep = 1
	}

	// Cache result
	t.qtyStepCacheMutex.Lock()
	t.qtyStepCache[symbol] = qtyStep
	t.qtyStepCacheMutex.Unlock()

	logger.Infof("🔵 [Bybit] %s qtyStep: %v", symbol, qtyStep)

	return qtyStep
}

// FormatQuantity formats quantity
func (t *BybitTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	// Get qtyStep for this symbol
	qtyStep := t.getQtyStep(symbol)

	// Align quantity according to qtyStep (round down to nearest step)
	alignedQty := math.Floor(quantity/qtyStep) * qtyStep

	// Calculate required decimal places
	decimals := 0
	if qtyStep < 1 {
		stepStr := strconv.FormatFloat(qtyStep, 'f', -1, 64)
		if idx := strings.Index(stepStr, "."); idx >= 0 {
			decimals = len(stepStr) - idx - 1
		}
	}

	// Format
	format := fmt.Sprintf("%%.%df", decimals)
	formatted := fmt.Sprintf(format, alignedQty)

	return formatted, nil
}

// Helper methods

func (t *BybitTrader) clearCache() {
	t.balanceCacheMutex.Lock()
	t.cachedBalance = nil
	t.balanceCacheMutex.Unlock()

	t.positionsCacheMutex.Lock()
	t.cachedPositions = nil
	t.positionsCacheMutex.Unlock()
}

func (t *BybitTrader) parseOrderResult(result *bybit.ServerResponse) (map[string]interface{}, error) {
	if result.RetCode != 0 {
		return nil, fmt.Errorf("order placement failed: %s", result.RetMsg)
	}

	resultData, ok := result.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("return format error")
	}

	orderId, _ := resultData["orderId"].(string)

	return map[string]interface{}{
		"orderId": orderId,
		"status":  "NEW",
	}, nil
}
