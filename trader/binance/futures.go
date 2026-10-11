package binance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"nofx/hook"
	"nofx/logger"
	"nofx/market/bnweight"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"math"
	"strconv"
)

// getBrOrderID generates unique order ID (for futures contracts)
// Format: x-{BR_ID}{TIMESTAMP}{RANDOM}
// Futures limit is 32 characters, use this limit consistently
// Uses nanosecond timestamp + random number to ensure global uniqueness (collision probability < 10^-20)
func getBrOrderID() string {
	brID := "KzrpZaP9" // Futures br ID

	// Calculate available space: 32 - len("x-KzrpZaP9") = 32 - 11 = 21 characters
	// Allocation: 13-digit timestamp + 8-digit random = 21 characters (perfect utilization)
	timestamp := time.Now().UnixNano() % 10000000000000 // 13-digit nanosecond timestamp

	// Generate 4-byte random number (8 hex digits)
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)

	// Format: x-KzrpZaP9{13-digit timestamp}{8-digit random}
	// Example: x-KzrpZaP91234567890123abcdef12 (exactly 31 characters)
	orderID := fmt.Sprintf("x-%s%d%s", brID, timestamp, randomHex)

	// Ensure not exceeding 32-character limit (theoretically exactly 31 characters)
	if len(orderID) > 32 {
		orderID = orderID[:32]
	}

	return orderID
}

// BrIDPrefix is the broker referral prefix every client order ID carries —
// order attribution (revival) depends on it, so tagged orders keep it too.
// Exported for the trader's startup reconciliation, which strips it before
// matching entry tags.
const BrIDPrefix = "x-KzrpZaP9"

// getBrOrderIDFor builds a client order ID that keeps the broker referral
// prefix but replaces the timestamp+random tail with a caller tag ("lim-…"
// for AI limit entries, "grid-…" for grid levels) so the order class is
// visible on-exchange. Binance caps client IDs at 32 chars; the prefix is
// 10, so the tag may use at most 22. Truncation keeps the tag's HEAD — the
// lim-<hash> class/ownership marker — and drops tail digits: timestamps can
// lose precision, the marker cannot.
func getBrOrderIDFor(tag string) string {
	const maxTag = 22
	if len(tag) > maxTag {
		tag = tag[:maxTag]
	}
	return BrIDPrefix + tag
}

// FuturesTrader Binance futures trader
type FuturesTrader struct {
	client *futures.Client

	// displayName is the human-readable label (trader name · model) used in
	// Telegram notifications.
	displayName string

	// Balance cache
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// Position cache
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// Cache validity period (15 seconds)
	cacheDuration time.Duration

	// Background order-sync lifecycle. Exchange credentials are immutable on
	// the SDK client, so a config reload must stop the old sync loop before the
	// old FuturesTrader is discarded; otherwise it keeps calling Binance every
	// 30 seconds with stale credentials forever.
	orderSyncMu     sync.Mutex
	orderSyncStop   chan struct{}
	orderSyncDone   chan struct{}
	syncAuthAlerted atomic.Bool
}

// binanceHTTPTimeout bounds every Binance REST call (review 2026-10-07 B1-4).
const binanceHTTPTimeout = 15 * time.Second

// NewFuturesTrader creates futures trader
func NewFuturesTrader(apiKey, secretKey string, userId string) *FuturesTrader {
	client := futures.NewClient(apiKey, secretKey)

	hookRes := hook.HookExec[hook.NewBinanceTraderResult](hook.NEW_BINANCE_TRADER, userId, client)
	if hookRes != nil && hookRes.GetResult() != nil {
		client = hookRes.GetResult()
	}
	// review 2026-10-07 B1-4: go-binance defaults to http.DefaultClient (no
	// timeout) and every call here uses context.Background(). The position /
	// balance caches now hold their write lock across a cache-miss fetch, so
	// one hung connection would freeze every reader (protection monitor
	// included). Bound every request; a hook-supplied client keeps its
	// transport (proxy) and only gains the timeout.
	switch {
	case client.HTTPClient == nil || client.HTTPClient == http.DefaultClient:
		client.HTTPClient = &http.Client{Timeout: binanceHTTPTimeout}
	case client.HTTPClient.Timeout == 0:
		hc := *client.HTTPClient
		hc.Timeout = binanceHTTPTimeout
		client.HTTPClient = &hc
	}
	// Meter every call through the shared Binance weight guard. This client
	// carries order placement/cancel/algo orders and the position/account
	// reads protection depends on, so it is Critical: never delayed, and still
	// sent during a 429/418 cooldown. The existing transport (default / proxy /
	// hook-supplied) is wrapped, not replaced; the client is copied so a
	// hook-supplied shared *http.Client is never mutated.
	metered := *client.HTTPClient
	metered.Transport = bnweight.Wrap(metered.Transport, bnweight.Critical)
	client.HTTPClient = &metered

	// Sync time to avoid "Timestamp ahead" error
	syncBinanceServerTime(client)
	trader := &FuturesTrader{
		client:        client,
		cacheDuration: 15 * time.Second, // 15-second cache
	}

	// Set dual-side position mode (Hedge Mode)
	// This is required because the code uses PositionSide (LONG/SHORT)
	if err := trader.setDualSidePosition(); err != nil {
		logger.Infof("⚠️ Failed to set dual-side position mode: %v (ignore this warning if already in dual-side mode)", err)
	}

	return trader
}

// setDualSidePosition ensures dual-side position mode (called during initialization).
// Queries the current mode first: when already in Hedge Mode (the normal case for
// an account with open orders/positions) no change request is sent — Binance
// rejects mode changes with -4067 while open orders exist.
func (t *FuturesTrader) setDualSidePosition() error {
	mode, err := t.client.NewGetPositionModeService().Do(context.Background())
	if err == nil && mode != nil && mode.DualSidePosition {
		logger.Infof("  ✓ Account is already in dual-side position mode (Hedge Mode)")
		return nil
	}
	// Query failed (non-fatal) or account is in one-way mode — attempt the switch.
	if err != nil {
		logger.Infof("  ⚠️ Could not query position mode (%v), attempting to set dual-side mode", err)
	}

	err = t.client.NewChangePositionModeService().
		DualSide(true). // true = dual-side position (Hedge Mode)
		Do(context.Background())

	if err != nil {
		// If error message contains "No need to change", it means already in dual-side position mode
		if strings.Contains(err.Error(), "No need to change position side") {
			logger.Infof("  ✓ Account is already in dual-side position mode (Hedge Mode)")
			return nil
		}
		// Other errors are returned (but won't interrupt initialization in the caller)
		return err
	}

	logger.Infof("  ✓ Account has been switched to dual-side position mode (Hedge Mode)")
	logger.Infof("  ℹ️  Dual-side position mode allows holding both long and short positions simultaneously")
	return nil
}

// syncBinanceServerTime syncs Binance server time to ensure request timestamps are valid
func syncBinanceServerTime(client *futures.Client) {
	serverTime, err := client.NewServerTimeService().Do(context.Background())
	if err != nil {
		logger.Infof("⚠️ Failed to sync Binance server time: %v", err)
		return
	}

	now := time.Now().UnixMilli()
	offset := now - serverTime
	client.TimeOffset = offset
	logger.Infof("⏱ Binance server time synced, offset %dms", offset)
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// calculatePrecision calculates precision from stepSize
func calculatePrecision(stepSize string) int {
	// Remove trailing zeros
	stepSize = trimTrailingZeros(stepSize)

	// Find decimal point
	dotIndex := -1
	for i := 0; i < len(stepSize); i++ {
		if stepSize[i] == '.' {
			dotIndex = i
			break
		}
	}

	// If no decimal point or decimal point is at the end, precision is 0
	if dotIndex == -1 || dotIndex == len(stepSize)-1 {
		return 0
	}

	// Return number of digits after decimal point
	return len(stepSize) - dotIndex - 1
}

// trimTrailingZeros removes trailing zeros
func trimTrailingZeros(s string) string {
	// If no decimal point, return directly
	if !stringContains(s, ".") {
		return s
	}

	// Iterate backwards to remove trailing zeros
	for len(s) > 0 && s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}

	// If last character is decimal point, remove it too
	if len(s) > 0 && s[len(s)-1] == '.' {
		s = s[:len(s)-1]
	}

	return s
}

// SetDisplayName sets the label used in Telegram notifications.
func (t *FuturesTrader) SetDisplayName(name string) {
	t.displayName = name
}

// notifyLabel returns the display name, falling back to the internal ID.
func (t *FuturesTrader) notifyLabel() string {
	if t.displayName != "" {
		return t.displayName
	}
	return "Binance"
}

// formatTriggerPrice renders an algo-order trigger price at the symbol's
// tick precision (decimals derived from PRICE_FILTER.tickSize). Refuses to
// emit a price that rounding moved by more than 1%: a mis-rounded stop
// trigger sits at the wrong level — instant or never — and "never" is how a
// position runs naked while everyone believes it is protected.
func formatTriggerPrice(price float64, precision int) (string, error) {
	if precision < 0 || precision > 8 {
		precision = 8
	}
	out := fmt.Sprintf("%.*f", precision, price)
	if rp, err := strconv.ParseFloat(out, 64); err == nil && price > 0 &&
		math.Abs(rp-price)/price > 0.01 {
		return "", fmt.Errorf("tick rounding moved the trigger %.2f%% (%.8f -> %s) — refusing to place", math.Abs(rp-price)/price*100, price, out)
	}
	return out, nil
}

// quantizeToTick renders a trigger price ON the symbol's tick grid:
// quantize to the nearest tickSize multiple, then format at the tick's
// decimal count. Decimal-place formatting alone (formatTriggerPrice) is only
// equivalent when the tick is a power of ten — on a 0.025 tick, 3dp rounding
// can emit 1.234, which is NOT a multiple of 0.025 and Binance rejects with
// -1111 (round-4 review R4-15). The same >1% drift guard applies.
func quantizeToTick(price, tickSize float64, tickDecimals int) (string, error) {
	if tickSize <= 0 || tickDecimals < 0 || tickDecimals > 8 {
		return formatTriggerPrice(price, 8)
	}
	q := math.Round(price/tickSize) * tickSize
	out := fmt.Sprintf("%.*f", tickDecimals, q)
	if rp, err := strconv.ParseFloat(out, 64); err == nil && price > 0 &&
		math.Abs(rp-price)/price > 0.01 {
		return "", fmt.Errorf("tick rounding moved the trigger %.2f%% (%.8f -> %s) — refusing to place", math.Abs(rp-price)/price*100, price, out)
	}
	return out, nil
}

func (t *FuturesTrader) InvalidateAccountCache() {
	t.balanceCacheMutex.Lock()
	t.cachedBalance = nil
	t.balanceCacheMutex.Unlock()
	t.positionsCacheMutex.Lock()
	t.cachedPositions = nil
	t.positionsCacheMutex.Unlock()
}
