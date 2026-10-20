package binance

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"nofx/logger"
	"nofx/trader/types"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/adshao/go-binance/v2/common"
	"github.com/adshao/go-binance/v2/futures"
)

// ErrSubmitUnknown means an entry may exist on the exchange, but its state
// could not be established. Callers must not blindly resubmit the entry.
var ErrSubmitUnknown = errors.New("Binance entry submit state is UNKNOWN")

// ClientOrderRefPrefix marks an order reference that is a client order ID
// rather than a numeric exchange order ID. GetOrderStatus and CancelOrder
// accept it so the execution layer can keep tracking an UNKNOWN submit (the
// POST response was lost and the lookup failed) until the exchange answers.
const ClientOrderRefPrefix = "client:"

// SubmitUnknownError is the typed form of ErrSubmitUnknown. It carries the
// symbol and the stable client order ID so callers can keep reconciling the
// same order instead of treating the submit as a plain failure.
type SubmitUnknownError struct {
	Symbol        string
	ClientOrderID string
	submitErr     error
	msg           string
}

func (e *SubmitUnknownError) Error() string { return e.msg }

// Unwrap keeps errors.Is(err, ErrSubmitUnknown) and the original submit error
// (e.g. *common.APIError) both reachable.
func (e *SubmitUnknownError) Unwrap() []error { return []error{ErrSubmitUnknown, e.submitErr} }

var unknownSubmitSleep = func(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func wrapEntrySubmitError(source string, err error) error {
	// WrapAuthError replaces the error chain. A 5xx with an auth-looking
	// body is still ambiguous and must retain both UNKNOWN and its cause.
	if errors.Is(err, ErrSubmitUnknown) {
		return err
	}
	return WrapAuthError(source, err)
}

// The SDK drops the HTTP status when constructing common.APIError. Retain
// 5xx evidence per submission so even a negative code on a 5xx is reconciled.
// Use a private client, preserving the configured transport and timeout,
// rather than mutating the shared client's transport during live trading.
type entrySubmitTransport struct {
	base        http.RoundTripper
	client      *futures.Client
	serverError atomic.Bool
}

func (t *entrySubmitTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(r)
	if resp != nil {
		if resp.StatusCode >= 500 {
			t.serverError.Store(true)
		}
		t.client.UsedWeight.UpdateByHeader(resp.Header)
		t.client.OrderCount.UpdateByHeader(resp.Header)
	}
	return resp, err
}

func (t *FuturesTrader) entryOrderService() (*futures.CreateOrderService, *entrySubmitTransport) {
	hc := *t.client.HTTPClient
	transport := &entrySubmitTransport{base: hc.Transport, client: t.client}
	if transport.base == nil {
		transport.base = http.DefaultTransport
	}
	hc.Transport = transport
	// Copy configuration explicitly: the SDK's weight counters are atomic
	// state and must not be copied while other requests update them.
	client := &futures.Client{
		APIKey: t.client.APIKey, SecretKey: t.client.SecretKey, KeyType: t.client.KeyType,
		BaseURL: t.client.BaseURL, UserAgent: t.client.UserAgent, HTTPClient: &hc,
		Debug: t.client.Debug, Logger: t.client.Logger, TimeOffset: t.client.TimeOffset,
	}
	return client.NewCreateOrderService(), transport
}

// submitEntryOrder submits exactly once. An ambiguous result is reconciled
// with that same client ID; it is never retried with another POST.
func (t *FuturesTrader) submitEntryOrder(service *futures.CreateOrderService, transport *entrySubmitTransport, symbol, clientID string) (*futures.CreateOrderResponse, bool, error) {
	order, submitErr := service.Do(context.Background())
	if submitErr == nil {
		return order, false, nil
	}
	var apiErr *common.APIError
	if !transport.serverError.Load() && errors.As(submitErr, &apiErr) && apiErr.Code < 0 && apiErr.Code != -1007 && apiErr.Code != -1001 {
		return nil, false, submitErr
	}

	// Bound lookup I/O as well as the retry waits. Three immediate/one-second
	// spaced attempts leave the last request within a three-second budget.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	notFound := 0
	var lookupErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			if ctx.Err() != nil {
				lookupErr = ctx.Err()
				break
			}
			if err := unknownSubmitSleep(ctx, time.Second); err != nil {
				lookupErr = err
				break
			}
		}
		found, err := t.client.NewGetOrderService().Symbol(symbol).OrigClientOrderID(clientID).Do(ctx)
		if err != nil {
			lookupErr = err
			var lookupAPIError *common.APIError
			if errors.As(err, &lookupAPIError) && lookupAPIError.Code == -2013 {
				notFound++
			}
			continue
		}
		if found.OrderID <= 0 || found.Symbol != symbol || (found.ClientOrderID != "" && found.ClientOrderID != clientID) {
			lookupErr = fmt.Errorf("lookup returned an invalid order for %s client id %s", symbol, clientID)
			continue
		}
		switch found.Status {
		case futures.OrderStatusTypeRejected, futures.OrderStatusTypeExpired, futures.OrderStatusTypeCanceled:
			executed, err := strconv.ParseFloat(found.ExecutedQuantity, 64)
			if err != nil || math.IsNaN(executed) || math.IsInf(executed, 0) || executed < 0 {
				lookupErr = fmt.Errorf("invalid executedQty %q for terminal order %d", found.ExecutedQuantity, found.OrderID)
				continue
			}
			if executed == 0 {
				return nil, false, submitErr
			}
		}
		logger.Warnf("submit result unknown — order found by client id, adopting: %s clientOrderId=%s orderId=%d status=%s", symbol, clientID, found.OrderID, found.Status)
		return &futures.CreateOrderResponse{
			OrderID: found.OrderID, ClientOrderID: clientID, Symbol: found.Symbol,
			Status: found.Status, Side: found.Side, PositionSide: found.PositionSide,
			Price: found.Price, OrigQuantity: found.OrigQuantity, ExecutedQuantity: found.ExecutedQuantity,
			AvgPrice: found.AvgPrice, UpdateTime: found.UpdateTime,
		}, true, nil
	}
	if notFound == 3 {
		return nil, false, submitErr
	}
	return nil, false, &SubmitUnknownError{
		Symbol: symbol, ClientOrderID: clientID, submitErr: submitErr,
		msg: fmt.Sprintf("%v (%s clientOrderId=%s): submit: %v; lookup: %v", ErrSubmitUnknown, symbol, clientID, submitErr, lookupErr),
	}
}

// OpenLong opens a long position
func (t *FuturesTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// R4 (2026-09-26 review): cancel ONLY this side's leftover protection —
	// CancelAllOrders wiped the opposite side's SL/TP (and any manual
	// positions' protection the hands-off rule will never restore).
	if err := t.CancelProtectiveOrdersForSide(symbol, string(futures.PositionSideTypeLong)); err != nil {
		logger.Infof("  ⚠ Failed to cancel old LONG protection (may not have any): %v", err)
	}

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// Note: Margin mode should be set by the caller (AutoTrader) before opening position via SetMarginMode

	// Format quantity to correct precision
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Check if formatted quantity is 0 (prevent rounding errors)
	quantityFloat, parseErr := strconv.ParseFloat(quantityStr, 64)
	if parseErr != nil || quantityFloat <= 0 {
		return nil, fmt.Errorf("position size too small, rounded to 0 (original: %.8f → formatted: %s). Suggest increasing position amount or selecting a lower-priced coin", quantity, quantityStr)
	}

	// Check minimum notional value (Binance requires at least 10 USDT)
	if err := t.CheckMinNotional(symbol, quantityFloat); err != nil {
		return nil, err
	}

	// Create market buy order (using br ID)
	clientOrderID := getBrOrderID()
	service, transport := t.entryOrderService()
	service = service.
		Symbol(symbol).
		Side(futures.SideTypeBuy).
		PositionSide(futures.PositionSideTypeLong).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		NewClientOrderID(clientOrderID)
	order, recovered, err := t.submitEntryOrder(service, transport, symbol, clientOrderID)

	if err != nil {
		return nil, fmt.Errorf("failed to open long position: %w", wrapEntrySubmitError("OpenLong "+symbol, err))
	}

	logger.Infof("✓ Opened long position successfully: %s quantity: %s", symbol, quantityStr)
	logger.Infof("  Order ID: %d", order.OrderID)

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	result["clientOrderId"] = clientOrderID
	if recovered {
		result["recoveredUnknownSubmit"] = true
	}
	return result, nil
}

// OpenShort opens a short position
func (t *FuturesTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// R4: only this side's leftover protection (see OpenLong).
	if err := t.CancelProtectiveOrdersForSide(symbol, string(futures.PositionSideTypeShort)); err != nil {
		logger.Infof("  ⚠ Failed to cancel old SHORT protection (may not have any): %v", err)
	}

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// Note: Margin mode should be set by the caller (AutoTrader) before opening position via SetMarginMode

	// Format quantity to correct precision
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Check if formatted quantity is 0 (prevent rounding errors)
	quantityFloat, parseErr := strconv.ParseFloat(quantityStr, 64)
	if parseErr != nil || quantityFloat <= 0 {
		return nil, fmt.Errorf("position size too small, rounded to 0 (original: %.8f → formatted: %s). Suggest increasing position amount or selecting a lower-priced coin", quantity, quantityStr)
	}

	// Check minimum notional value (Binance requires at least 10 USDT)
	if err := t.CheckMinNotional(symbol, quantityFloat); err != nil {
		return nil, err
	}

	// Create market sell order (using br ID)
	clientOrderID := getBrOrderID()
	service, transport := t.entryOrderService()
	service = service.
		Symbol(symbol).
		Side(futures.SideTypeSell).
		PositionSide(futures.PositionSideTypeShort).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		NewClientOrderID(clientOrderID)
	order, recovered, err := t.submitEntryOrder(service, transport, symbol, clientOrderID)

	if err != nil {
		return nil, fmt.Errorf("failed to open short position: %w", wrapEntrySubmitError("OpenShort "+symbol, err))
	}

	logger.Infof("✓ Opened short position successfully: %s quantity: %s", symbol, quantityStr)
	logger.Infof("  Order ID: %d", order.OrderID)

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	result["clientOrderId"] = clientOrderID
	if recovered {
		result["recoveredUnknownSubmit"] = true
	}
	return result, nil
}

// CloseLong closes a long position
func (t *FuturesTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// quantity==0 means "close everything". Capture that BEFORE the lookup below
	// overwrites quantity with the live size — otherwise the full-close cancel
	// further down was dead code and the closePosition SL/TP survived the close
	// (2026-10-16 review E).
	fullClose := quantity == 0
	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no long position found for %s", symbol)
		}
	}

	// Format quantity
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Create market sell order (close long, using br ID)
	order, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(futures.SideTypeSell).
		PositionSide(futures.PositionSideTypeLong).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		NewClientOrderID(getBrOrderID()).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to close long position: %w", WrapAuthError("CloseLong "+symbol, err))
	}

	logger.Infof("✓ Closed long position successfully: %s quantity: %s", symbol, quantityStr)

	// Full closes cancel the remaining SL/TP orders; PARTIAL closes (explicit
	// quantity, e.g. the TP-ladder 1/3 trim) must keep them — the remaining
	// position would otherwise ride unprotected.
	if fullClose {
		// R4 (2026-09-26 review): full close cancels THIS side's protection
		// only — CancelAllOrders wiped the opposite side's SL/TP too.
		// 2026-10-20 review area2 #2: a successful POST is not a flat
		// position (a market close can part-fill and expire) — only a fresh
		// snapshot showing this side flat releases its protection. Otherwise
		// the remainder keeps its SL/TP; OrderSync cleans them up once the
		// close trades are accounted and the side is really flat.
		if t.confirmSideFlatAfterClose(symbol, "long") {
			if err := t.CancelProtectiveOrdersForSide(symbol, string(futures.PositionSideTypeLong)); err != nil {
				logger.Infof("  ⚠ Failed to cancel pending orders: %v", err)
			}
		} else {
			logger.Warnf("  ⚠ %s close submitted but the long position is not confirmed flat — keeping its SL/TP", symbol)
		}
	}

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	return result, nil
}

// CloseShort closes a short position
func (t *FuturesTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// quantity==0 means "close everything". Capture that BEFORE the lookup below
	// overwrites quantity with the live size — otherwise the full-close cancel
	// further down was dead code and the closePosition SL/TP survived the close
	// (2026-10-16 review E).
	fullClose := quantity == 0
	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = -pos["positionAmt"].(float64) // Short position quantity is negative, take absolute value
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no short position found for %s", symbol)
		}
	}

	// Format quantity
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Create market buy order (close short, using br ID)
	order, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(futures.SideTypeBuy).
		PositionSide(futures.PositionSideTypeShort).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		NewClientOrderID(getBrOrderID()).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to close short position: %w", WrapAuthError("CloseShort "+symbol, err))
	}

	logger.Infof("✓ Closed short position successfully: %s quantity: %s", symbol, quantityStr)

	// Full closes cancel the remaining SL/TP orders; partial closes keep
	// them (same rule as CloseLong).
	if fullClose {
		// R4 (2026-09-26 review): full close cancels THIS side's protection
		// only — CancelAllOrders wiped the opposite side's SL/TP too.
		// 2026-10-20 review area2 #2: a successful POST is not a flat
		// position (a market close can part-fill and expire) — only a fresh
		// snapshot showing this side flat releases its protection. Otherwise
		// the remainder keeps its SL/TP; OrderSync cleans them up once the
		// close trades are accounted and the side is really flat.
		if t.confirmSideFlatAfterClose(symbol, "short") {
			if err := t.CancelProtectiveOrdersForSide(symbol, string(futures.PositionSideTypeShort)); err != nil {
				logger.Infof("  ⚠ Failed to cancel pending orders: %v", err)
			}
		} else {
			logger.Warnf("  ⚠ %s close submitted but the short position is not confirmed flat — keeping its SL/TP", symbol)
		}
	}

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	return result, nil
}

// closeConfirmSleep spaces the post-close position re-reads (test seam).
var closeConfirmSleep = time.Sleep

// confirmSideFlatAfterClose re-reads the account (cache bypassed) and reports
// whether the given side ("long"/"short") of symbol holds no position. A failed
// read counts as NOT flat: callers then keep protection instead of stripping
// it from a position that may still be open. One retry rides out a short
// read-after-write lag right after the close order.
func (t *FuturesTrader) confirmSideFlatAfterClose(symbol, side string) bool {
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			closeConfirmSleep(300 * time.Millisecond)
		}
		t.InvalidatePositionCache()
		positions, err := t.GetPositions()
		if err != nil {
			continue
		}
		flat := true
		for _, pos := range positions {
			if pos["symbol"] != symbol || pos["side"] != side {
				continue
			}
			if amt, ok := pos["positionAmt"].(float64); !ok || amt != 0 {
				flat = false
				break
			}
		}
		if flat {
			return true
		}
	}
	return false
}

// CancelStopLossOrders cancels only stop-loss orders (doesn't affect take-profit orders)
// Now uses both legacy API and new Algo Order API
// CancelProtectiveOrdersForSide cancels BOTH protective legs (SL + TP) for
// ONE position side — R4 (2026-09-26 review): OpenLong/OpenShort and full
// closes used CancelAllOrders(symbol), which wiped the OPPOSITE side's
// protection (including manually opened positions' stops, which the
// hands-off watchdog will never restore). Only this side's STOP*/TAKE_PROFIT*
// algos and legacy stops are cancelled.
func (t *FuturesTrader) CancelProtectiveOrdersForSide(symbol, positionSide string) error {
	return t.cancelProtectiveOrdersForSide(symbol, positionSide, false)
}

// CancelProgramProtectiveOrdersForSide only cancels this program's protective
// orders for the exact position side. Manual orders (including empty client
// IDs), entry orders and the opposite hedge side are never cancelled here.
func (t *FuturesTrader) CancelProgramProtectiveOrdersForSide(symbol, positionSide string) error {
	if positionSide != string(futures.PositionSideTypeLong) && positionSide != string(futures.PositionSideTypeShort) && positionSide != string(futures.PositionSideTypeBoth) {
		return fmt.Errorf("invalid position side %q", positionSide)
	}
	return t.cancelProtectiveOrdersForSide(symbol, positionSide, true)
}

// ProtectiveOrderSnapshot is the set of program protective order IDs that
// rested on one position side at the moment it was taken.
type ProtectiveOrderSnapshot struct {
	OrderIDs []int64
	AlgoIDs  []int64
}

// Empty reports whether the snapshot holds nothing to cancel.
func (s *ProtectiveOrderSnapshot) Empty() bool {
	return s == nil || (len(s.OrderIDs) == 0 && len(s.AlgoIDs) == 0)
}

// SnapshotProgramProtectiveOrdersForSide lists (without cancelling) this
// program's protective orders for the exact position side. Any listing error
// is returned: an unknown set must never authorize a later cancel.
func (t *FuturesTrader) SnapshotProgramProtectiveOrdersForSide(symbol, positionSide string) (*ProtectiveOrderSnapshot, error) {
	if positionSide != string(futures.PositionSideTypeLong) && positionSide != string(futures.PositionSideTypeShort) && positionSide != string(futures.PositionSideTypeBoth) {
		return nil, fmt.Errorf("invalid position side %q", positionSide)
	}
	snap := &ProtectiveOrderSnapshot{}
	orders, err := t.client.NewListOpenOrdersService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list legacy orders: %w", err)
	}
	for _, order := range orders {
		if isProtectiveOrderType(string(order.Type)) && string(order.PositionSide) == positionSide && strings.HasPrefix(order.ClientOrderID, BrIDPrefix) {
			snap.OrderIDs = append(snap.OrderIDs, order.OrderID)
		}
	}
	algoOrders, err := t.client.NewListOpenAlgoOrdersService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list algo orders: %w", err)
	}
	for _, algo := range algoOrders {
		if isProtectiveOrderType(string(algo.OrderType)) && string(algo.PositionSide) == positionSide && strings.HasPrefix(algo.ClientAlgoId, BrIDPrefix) {
			snap.AlgoIDs = append(snap.AlgoIDs, algo.AlgoId)
		}
	}
	return snap, nil
}

// CancelProtectiveOrderSnapshot cancels exactly the IDs in a snapshot taken
// BEFORE the caller confirmed the side flat. Orders placed afterwards (a new
// position's SL/TP) carry new IDs and are therefore never touched, which the
// re-enumerating CancelProgramProtectiveOrdersForSide could not guarantee
// (2026-10-20 review area2 #4).
func (t *FuturesTrader) CancelProtectiveOrderSnapshot(symbol, positionSide string, snap *ProtectiveOrderSnapshot) error {
	if snap.Empty() {
		return nil
	}
	var cancelErrors []error
	for _, id := range snap.OrderIDs {
		if _, err := t.client.NewCancelOrderService().Symbol(symbol).OrderID(id).Do(context.Background()); err != nil {
			cancelErrors = append(cancelErrors, fmt.Errorf("order %d: %v", id, err))
		} else {
			logger.Infof("  Cancelled orphaned program protective order: %s %s orderId=%d", symbol, positionSide, id)
		}
	}
	for _, id := range snap.AlgoIDs {
		if _, err := t.client.NewCancelAlgoOrderService().AlgoID(id).Do(context.Background()); err != nil {
			cancelErrors = append(cancelErrors, fmt.Errorf("algo %d: %v", id, err))
		} else {
			logger.Infof("  Cancelled orphaned program protective algo: %s %s algoId=%d", symbol, positionSide, id)
		}
	}
	if len(cancelErrors) > 0 {
		return fmt.Errorf("failed to cancel protective orders (%s %s): %v", symbol, positionSide, cancelErrors)
	}
	return nil
}

func isProtectiveOrderType(orderType string) bool {
	orderType = strings.ToUpper(orderType)
	return strings.Contains(orderType, "STOP") || strings.Contains(orderType, "TAKE_PROFIT")
}

func (t *FuturesTrader) cancelProtectiveOrdersForSide(symbol, positionSide string, programOnly bool) error {
	cancelErrors := []error{}

	orders, err := t.client.NewListOpenOrdersService().Symbol(symbol).Do(context.Background())
	if err != nil {
		if programOnly {
			cancelErrors = append(cancelErrors, fmt.Errorf("list legacy orders: %w", err))
		}
	} else {
		for _, order := range orders {
			if !isProtectiveOrderType(string(order.Type)) || string(order.PositionSide) != positionSide {
				continue
			}
			if programOnly && !strings.HasPrefix(order.ClientOrderID, BrIDPrefix) {
				logger.Infof("  Keeping manual protective order: %s %s orderId=%d clientOrderId=%s", symbol, positionSide, order.OrderID, order.ClientOrderID)
				continue
			}
			if _, err := t.client.NewCancelOrderService().Symbol(symbol).OrderID(order.OrderID).Do(context.Background()); err != nil {
				cancelErrors = append(cancelErrors, fmt.Errorf("order %d: %v", order.OrderID, err))
			} else if programOnly {
				logger.Infof("  Cancelled orphaned program protective order: %s %s orderId=%d clientOrderId=%s", symbol, positionSide, order.OrderID, order.ClientOrderID)
			}
		}
	}

	algoOrders, err := t.client.NewListOpenAlgoOrdersService().Symbol(symbol).Do(context.Background())
	if err != nil {
		if programOnly {
			cancelErrors = append(cancelErrors, fmt.Errorf("list algo orders: %w", err))
		}
	} else {
		for _, algo := range algoOrders {
			if !isProtectiveOrderType(string(algo.OrderType)) || string(algo.PositionSide) != positionSide {
				continue
			}
			if programOnly && !strings.HasPrefix(algo.ClientAlgoId, BrIDPrefix) {
				logger.Infof("  Keeping manual protective algo: %s %s algoId=%d clientAlgoId=%s", symbol, positionSide, algo.AlgoId, algo.ClientAlgoId)
				continue
			}
			if _, err := t.client.NewCancelAlgoOrderService().AlgoID(algo.AlgoId).Do(context.Background()); err != nil {
				cancelErrors = append(cancelErrors, fmt.Errorf("algo %d: %v", algo.AlgoId, err))
			} else if programOnly {
				logger.Infof("  Cancelled orphaned program protective algo: %s %s algoId=%d clientAlgoId=%s", symbol, positionSide, algo.AlgoId, algo.ClientAlgoId)
			}
		}
	}

	if len(cancelErrors) > 0 {
		return fmt.Errorf("failed to cancel protective orders (%s %s): %v", symbol, positionSide, cancelErrors)
	}
	return nil
}

// CancelStopLossOrdersForSide cancels the symbol's stop-loss orders for ONE
// position side only — the hedge-mode-safe primitive (2026-09-25 P0: the
// symbol-wide cancel killed the OPPOSITE side's stop too, and the side-blind
// watchdog then judged the naked position protected). Empty positionSide
// cancels all sides (legacy behavior).
func (t *FuturesTrader) CancelStopLossOrdersForSide(symbol, positionSide string) error {
	canceledCount := 0
	var cancelErrors []error
	match := func(ps string) bool {
		return positionSide == "" || ps == "" || ps == positionSide ||
			ps == string(futures.PositionSideTypeBoth)
	}

	// 1. Legacy stop-loss orders
	orders, err := t.client.NewListOpenOrdersService().Symbol(symbol).Do(context.Background())
	if err == nil {
		for _, order := range orders {
			orderType := string(order.Type)
			if (orderType == "STOP_MARKET" || orderType == "STOP") && match(string(order.PositionSide)) {
				if _, err := t.client.NewCancelOrderService().Symbol(symbol).OrderID(order.OrderID).Do(context.Background()); err != nil {
					cancelErrors = append(cancelErrors, fmt.Errorf("order %d: %v", order.OrderID, err))
					continue
				}
				canceledCount++
			}
		}
	}

	// 2. Algo stop-loss orders
	algoOrders, err := t.client.NewListOpenAlgoOrdersService().Symbol(symbol).Do(context.Background())
	if err == nil {
		for _, algoOrder := range algoOrders {
			if (algoOrder.OrderType == futures.AlgoOrderTypeStopMarket || algoOrder.OrderType == futures.AlgoOrderTypeStop) && match(string(algoOrder.PositionSide)) {
				if _, err := t.client.NewCancelAlgoOrderService().AlgoID(algoOrder.AlgoId).Do(context.Background()); err != nil {
					cancelErrors = append(cancelErrors, fmt.Errorf("algo %d: %v", algoOrder.AlgoId, err))
					continue
				}
				canceledCount++
			}
		}
	}

	if len(cancelErrors) > 0 && canceledCount == 0 {
		return fmt.Errorf("failed to cancel stop-loss orders (%s): %v", positionSide, cancelErrors)
	}
	return nil
}

// CancelTakeProfitOrdersForSide cancels the symbol's take-profit orders for
// ONE position side only — the TP twin of CancelStopLossOrdersForSide (the
// -4130 stale-leg replacement needs the same side-scoped primitive; the
// symbol-wide CancelTakeProfitOrders would kill the opposite hedge leg's
// TP, the 09-28 review P3).
func (t *FuturesTrader) CancelTakeProfitOrdersForSide(symbol, positionSide string) error {
	canceledCount := 0
	var cancelErrors []error
	match := func(ps string) bool {
		return positionSide == "" || ps == "" || ps == positionSide ||
			ps == string(futures.PositionSideTypeBoth)
	}

	// 1. Legacy take-profit orders
	orders, err := t.client.NewListOpenOrdersService().Symbol(symbol).Do(context.Background())
	if err == nil {
		for _, order := range orders {
			orderType := string(order.Type)
			if (orderType == "TAKE_PROFIT_MARKET" || orderType == "TAKE_PROFIT") && match(string(order.PositionSide)) {
				if _, err := t.client.NewCancelOrderService().Symbol(symbol).OrderID(order.OrderID).Do(context.Background()); err != nil {
					cancelErrors = append(cancelErrors, fmt.Errorf("order %d: %v", order.OrderID, err))
					continue
				}
				canceledCount++
			}
		}
	}

	// 2. Algo take-profit orders
	algoOrders, err := t.client.NewListOpenAlgoOrdersService().Symbol(symbol).Do(context.Background())
	if err == nil {
		for _, algoOrder := range algoOrders {
			if (algoOrder.OrderType == futures.AlgoOrderTypeTakeProfitMarket || algoOrder.OrderType == futures.AlgoOrderTypeTakeProfit) && match(string(algoOrder.PositionSide)) {
				if _, err := t.client.NewCancelAlgoOrderService().AlgoID(algoOrder.AlgoId).Do(context.Background()); err != nil {
					cancelErrors = append(cancelErrors, fmt.Errorf("algo %d: %v", algoOrder.AlgoId, err))
					continue
				}
				canceledCount++
			}
		}
	}

	if len(cancelErrors) > 0 && canceledCount == 0 {
		return fmt.Errorf("failed to cancel take-profit orders (%s): %v", positionSide, cancelErrors)
	}
	return nil
}

func (t *FuturesTrader) CancelStopLossOrders(symbol string) error {
	canceledCount := 0
	var cancelErrors []error

	// 1. Cancel legacy stop-loss orders
	orders, err := t.client.NewListOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err == nil {
		for _, order := range orders {
			orderType := string(order.Type)

			// Only cancel stop-loss orders (don't cancel take-profit orders)
			// Use string comparison since OrderType constants were removed in v2.8.9
			if orderType == "STOP_MARKET" || orderType == "STOP" {
				_, err := t.client.NewCancelOrderService().
					Symbol(symbol).
					OrderID(order.OrderID).
					Do(context.Background())

				if err != nil {
					errMsg := fmt.Sprintf("Order ID %d: %v", order.OrderID, err)
					cancelErrors = append(cancelErrors, fmt.Errorf("%s", errMsg))
					logger.Infof("  ⚠ Failed to cancel legacy stop-loss order: %s", errMsg)
					continue
				}

				canceledCount++
				logger.Infof("  ✓ Canceled legacy stop-loss order (Order ID: %d, Type: %s, Side: %s)", order.OrderID, orderType, order.PositionSide)
			}
		}
	}

	// 2. Cancel Algo stop-loss orders
	algoOrders, err := t.client.NewListOpenAlgoOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err == nil {
		for _, algoOrder := range algoOrders {
			// Only cancel stop-loss orders
			if algoOrder.OrderType == futures.AlgoOrderTypeStopMarket || algoOrder.OrderType == futures.AlgoOrderTypeStop {
				_, err := t.client.NewCancelAlgoOrderService().
					AlgoID(algoOrder.AlgoId).
					Do(context.Background())

				if err != nil {
					errMsg := fmt.Sprintf("Algo ID %d: %v", algoOrder.AlgoId, err)
					cancelErrors = append(cancelErrors, fmt.Errorf("%s", errMsg))
					logger.Infof("  ⚠ Failed to cancel Algo stop-loss order: %s", errMsg)
					continue
				}

				canceledCount++
				logger.Infof("  ✓ Canceled Algo stop-loss order (Algo ID: %d, Type: %s)", algoOrder.AlgoId, algoOrder.OrderType)
			}
		}
	}

	if canceledCount == 0 && len(cancelErrors) == 0 {
		logger.Infof("  ℹ %s has no stop-loss orders to cancel", symbol)
	} else if canceledCount > 0 {
		logger.Infof("  ✓ Canceled %d stop-loss order(s) for %s", canceledCount, symbol)
	}

	// If all cancellations failed, return error
	if len(cancelErrors) > 0 && canceledCount == 0 {
		return fmt.Errorf("failed to cancel stop-loss orders: %v", cancelErrors)
	}

	return nil
}

// CancelTakeProfitOrders cancels only take-profit orders (doesn't affect stop-loss orders)
// Now uses both legacy API and new Algo Order API
func (t *FuturesTrader) CancelTakeProfitOrders(symbol string) error {
	canceledCount := 0
	var cancelErrors []error

	// 1. Cancel legacy take-profit orders
	orders, err := t.client.NewListOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err == nil {
		for _, order := range orders {
			orderType := string(order.Type)

			// Only cancel take-profit orders (don't cancel stop-loss orders)
			// Use string comparison since OrderType constants were removed in v2.8.9
			if orderType == "TAKE_PROFIT_MARKET" || orderType == "TAKE_PROFIT" {
				_, err := t.client.NewCancelOrderService().
					Symbol(symbol).
					OrderID(order.OrderID).
					Do(context.Background())

				if err != nil {
					errMsg := fmt.Sprintf("Order ID %d: %v", order.OrderID, err)
					cancelErrors = append(cancelErrors, fmt.Errorf("%s", errMsg))
					logger.Infof("  ⚠ Failed to cancel legacy take-profit order: %s", errMsg)
					continue
				}

				canceledCount++
				logger.Infof("  ✓ Canceled legacy take-profit order (Order ID: %d, Type: %s, Side: %s)", order.OrderID, orderType, order.PositionSide)
			}
		}
	}

	// 2. Cancel Algo take-profit orders
	algoOrders, err := t.client.NewListOpenAlgoOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err == nil {
		for _, algoOrder := range algoOrders {
			// Only cancel take-profit orders
			if algoOrder.OrderType == futures.AlgoOrderTypeTakeProfitMarket || algoOrder.OrderType == futures.AlgoOrderTypeTakeProfit {
				_, err := t.client.NewCancelAlgoOrderService().
					AlgoID(algoOrder.AlgoId).
					Do(context.Background())

				if err != nil {
					errMsg := fmt.Sprintf("Algo ID %d: %v", algoOrder.AlgoId, err)
					cancelErrors = append(cancelErrors, fmt.Errorf("%s", errMsg))
					logger.Infof("  ⚠ Failed to cancel Algo take-profit order: %s", errMsg)
					continue
				}

				canceledCount++
				logger.Infof("  ✓ Canceled Algo take-profit order (Algo ID: %d, Type: %s)", algoOrder.AlgoId, algoOrder.OrderType)
			}
		}
	}

	if canceledCount == 0 && len(cancelErrors) == 0 {
		logger.Infof("  ℹ %s has no take-profit orders to cancel", symbol)
	} else if canceledCount > 0 {
		logger.Infof("  ✓ Canceled %d take-profit order(s) for %s", canceledCount, symbol)
	}

	// If all cancellations failed, return error
	if len(cancelErrors) > 0 && canceledCount == 0 {
		return fmt.Errorf("failed to cancel take-profit orders: %v", cancelErrors)
	}

	return nil
}

// CancelAllOrders cancels all pending orders for this symbol
// Now uses both legacy API and new Algo Order API
func (t *FuturesTrader) CancelAllOrders(symbol string) error {
	// 1. Cancel all legacy orders
	err := t.client.NewCancelAllOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		logger.Infof("  ⚠ Failed to cancel legacy orders: %v", err)
	} else {
		logger.Infof("  ✓ Canceled all legacy pending orders for %s", symbol)
	}

	// 2. Cancel all Algo orders
	err = t.client.NewCancelAllAlgoOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		// Ignore "no algo orders" error
		if !contains(err.Error(), "no algo") && !contains(err.Error(), "No algo") {
			logger.Infof("  ⚠ Failed to cancel Algo orders: %v", err)
		}
	} else {
		logger.Infof("  ✓ Canceled all Algo orders for %s", symbol)
	}

	return nil
}

// PlaceLimitOrder places a limit order for grid trading
// This implements the GridTrader interface for FuturesTrader
func (t *FuturesTrader) PlaceLimitOrder(req *types.LimitOrderRequest) (*types.LimitOrderResult, error) {
	// Quantize against the symbol's actual LOT_SIZE and validate before the
	// request reaches Binance. Previously QNT 0.03 became "0.0" and was sent
	// repeatedly, producing -4003 on every otherwise-valid signal.
	rules, err := t.getSymbolOrderRules(req.Symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get quantity rules: %w", err)
	}
	quantityStr, err := formatQuantityWithRules(req.Symbol, req.Quantity, rules)
	if err != nil {
		return nil, fmt.Errorf("failed to format quantity: %w", err)
	}

	// Format price to correct precision
	priceStr, err := t.FormatPrice(req.Symbol, req.Price)
	if err != nil {
		return nil, fmt.Errorf("failed to format price: %w", err)
	}
	quantityValue, _ := strconv.ParseFloat(quantityStr, 64)
	priceValue, _ := strconv.ParseFloat(priceStr, 64)
	if !req.ReduceOnly {
		if err := validateOrderNotional(req.Symbol, quantityValue, priceValue, rules.MinNotional); err != nil {
			return nil, err
		}
	}

	// Set leverage if specified
	if req.Leverage > 0 {
		if err := t.SetLeverage(req.Symbol, req.Leverage); err != nil {
			// review 2026-10-07 B1-3: match market entries; do not submit
			// an order with unconfirmed leverage.
			return nil, err
		}
	}

	// Determine side and position side
	var side futures.SideType
	var positionSide futures.PositionSideType

	if req.Side == "BUY" {
		side = futures.SideTypeBuy
		positionSide = futures.PositionSideTypeLong
	} else {
		side = futures.SideTypeSell
		positionSide = futures.PositionSideTypeShort
	}

	// Client order ID: every order keeps the broker referral prefix (revival
	// attribution, getBrOrderID). A caller-supplied req.ClientID tag is
	// carried after it ("lim…" for AI limit entries, "grid…" for grid levels)
	// so the order class is visible on-exchange — startup reconciliation
	// attributes resting limit entries by this tag and never touches orders
	// without it.
	clientOrderID := getBrOrderID()
	if req.ClientID != "" {
		clientOrderID = getBrOrderIDFor(req.ClientID)
	}
	orderService := t.client.NewCreateOrderService()
	var transport *entrySubmitTransport
	if !req.ReduceOnly {
		orderService, transport = t.entryOrderService()
	}
	orderService = orderService.
		Symbol(req.Symbol).
		Side(side).
		PositionSide(positionSide).
		Type(futures.OrderTypeLimit).
		TimeInForce(futures.TimeInForceTypeGTC).
		Quantity(quantityStr).
		Price(priceStr).
		NewClientOrderID(clientOrderID)
	if req.ReduceOnly {
		// Reduce-only resting limits (1R profit-lock trims) close in the
		// position's own direction and never open new exposure.
		orderService = orderService.ReduceOnly(true)
	}

	// Execute order
	var order *futures.CreateOrderResponse
	if req.ReduceOnly {
		order, err = orderService.Do(context.Background())
	} else {
		order, _, err = t.submitEntryOrder(orderService, transport, req.Symbol, clientOrderID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to place limit order: %w", wrapEntrySubmitError("PlaceLimitOrder "+req.Symbol, err))
	}

	logger.Infof("✓ [Grid] Placed limit order: %s %s %s @ %s, qty=%s, orderID=%d",
		req.Symbol, req.Side, positionSide, priceStr, quantityStr, order.OrderID)

	return &types.LimitOrderResult{
		OrderID:  fmt.Sprintf("%d", order.OrderID),
		ClientID: order.ClientOrderID,

		Symbol:       order.Symbol,
		Side:         string(order.Side),
		PositionSide: string(order.PositionSide),
		Price:        req.Price,
		Quantity:     quantityValue,
		Status:       string(order.Status),
	}, nil
}

func validateOrderNotional(symbol string, quantity, price, minNotional float64) error {
	notional := quantity * price
	if notional < minNotional {
		return fmt.Errorf("MIN_NOTIONAL: %s order amount %.2f USDT is below minimum %.2f USDT (quantity=%.8f, price=%.8f)",
			symbol, notional, minNotional, quantity, price)
	}
	return nil
}

// CancelOrder cancels a specific order by ID
// This implements the GridTrader interface for FuturesTrader
func (t *FuturesTrader) CancelOrder(symbol, orderID string) error {
	cancelService := t.client.NewCancelOrderService().Symbol(symbol)
	if clientID, ok := strings.CutPrefix(orderID, ClientOrderRefPrefix); ok && clientID != "" {
		// UNKNOWN-submit entry still tracked by its client order ID.
		cancelService = cancelService.OrigClientOrderID(clientID)
	} else {
		// Parse order ID to int64
		orderIDInt, err := strconv.ParseInt(orderID, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid order ID: %w", err)
		}
		cancelService = cancelService.OrderID(orderIDInt)
	}
	_, err := cancelService.Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	logger.Infof("✓ [Grid] Cancelled order: %s/%s", symbol, orderID)
	return nil
}

// GetOrderBook gets the order book for a symbol
// This implements the GridTrader interface for FuturesTrader
func (t *FuturesTrader) GetOrderBook(symbol string, depth int) (bids, asks [][]float64, err error) {
	book, err := t.client.NewDepthService().
		Symbol(symbol).
		Limit(depth).
		Do(context.Background())

	if err != nil {
		return nil, nil, fmt.Errorf("failed to get order book: %w", err)
	}

	// Convert bids
	bids = make([][]float64, len(book.Bids))
	for i, bid := range book.Bids {
		price, _ := strconv.ParseFloat(bid.Price, 64)
		qty, _ := strconv.ParseFloat(bid.Quantity, 64)
		bids[i] = []float64{price, qty}
	}

	// Convert asks
	asks = make([][]float64, len(book.Asks))
	for i, ask := range book.Asks {
		price, _ := strconv.ParseFloat(ask.Price, 64)
		qty, _ := strconv.ParseFloat(ask.Quantity, 64)
		asks[i] = []float64{price, qty}
	}

	return bids, asks, nil
}

// CancelStopOrders cancels take-profit/stop-loss orders for this symbol (used to adjust TP/SL positions)
// Now uses both legacy API and new Algo Order API (Binance migrated stop orders to Algo system)
func (t *FuturesTrader) CancelStopOrders(symbol string) error {
	canceledCount := 0

	// 1. Cancel legacy stop orders (for backward compatibility)
	orders, err := t.client.NewListOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err == nil {
		for _, order := range orders {
			orderType := string(order.Type)

			// Only cancel stop-loss and take-profit orders
			// Use string comparison since OrderType constants were removed in v2.8.9
			if orderType == "STOP_MARKET" ||
				orderType == "TAKE_PROFIT_MARKET" ||
				orderType == "STOP" ||
				orderType == "TAKE_PROFIT" {

				_, err := t.client.NewCancelOrderService().
					Symbol(symbol).
					OrderID(order.OrderID).
					Do(context.Background())

				if err != nil {
					logger.Infof("  ⚠ Failed to cancel legacy order %d: %v", order.OrderID, err)
					continue
				}

				canceledCount++
				logger.Infof("  ✓ Canceled legacy stop order for %s (Order ID: %d, Type: %s)",
					symbol, order.OrderID, orderType)
			}
		}
	}

	// 2. Cancel Algo orders (new API)
	err = t.client.NewCancelAllAlgoOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		// Ignore "no algo orders" error
		if !contains(err.Error(), "no algo") && !contains(err.Error(), "No algo") {
			logger.Infof("  ⚠ Failed to cancel Algo orders: %v", err)
		}
	} else {
		logger.Infof("  ✓ Canceled all Algo orders for %s", symbol)
		canceledCount++
	}

	if canceledCount == 0 {
		logger.Infof("  ℹ %s has no take-profit/stop-loss orders to cancel", symbol)
	}

	return nil
}

// GetOpenOrders gets all open/pending orders for a symbol
func (t *FuturesTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	var result []types.OpenOrder

	// 1. Get legacy open orders
	orders, err := t.client.NewListOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to get open orders: %w", err)
	}

	for _, order := range orders {
		price, _ := strconv.ParseFloat(order.Price, 64)
		stopPrice, _ := strconv.ParseFloat(order.StopPrice, 64)
		quantity, _ := strconv.ParseFloat(order.OrigQuantity, 64)

		result = append(result, types.OpenOrder{
			OrderID:       fmt.Sprintf("%d", order.OrderID),
			Symbol:        order.Symbol,
			Side:          string(order.Side),
			PositionSide:  string(order.PositionSide),
			Type:          string(order.Type),
			Price:         price,
			StopPrice:     stopPrice,
			Quantity:      quantity,
			Status:        string(order.Status),
			ClientID:      order.ClientOrderID,
			ClosePosition: order.ClosePosition, ReduceOnly: order.ReduceOnly,
		})
	}

	// 2. Get Algo orders (new API for stop-loss/take-profit)
	algoOrders, err := t.client.NewListOpenAlgoOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("algo protection query failed: %w", err)
	}
	{
		for _, algoOrder := range algoOrders {
			triggerPrice, _ := strconv.ParseFloat(algoOrder.TriggerPrice, 64)
			quantity, _ := strconv.ParseFloat(algoOrder.Quantity, 64)

			result = append(result, types.OpenOrder{
				OrderID:       fmt.Sprintf("%d", algoOrder.AlgoId),
				Symbol:        algoOrder.Symbol,
				Side:          string(algoOrder.Side),
				PositionSide:  string(algoOrder.PositionSide),
				Type:          string(algoOrder.OrderType),
				Price:         0, // Algo orders use stop price
				StopPrice:     triggerPrice,
				Quantity:      quantity,
				Status:        "NEW",
				ClosePosition: algoOrder.ClosePosition, ReduceOnly: algoOrder.ReduceOnly, Algo: true,
			})
		}
	}

	return result, nil
}

// SetStopLoss sets stop-loss order using new Algo Order API
// Binance has migrated stop orders to Algo Order system (error -4120 STOP_ORDER_SWITCH_ALGO)
func (t *FuturesTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	var side futures.SideType
	var posSide futures.PositionSideType

	if positionSide == "LONG" {
		side = futures.SideTypeSell
		posSide = futures.PositionSideTypeLong
	} else {
		side = futures.SideTypeBuy
		posSide = futures.PositionSideTypeShort
	}

	// Use new Algo Order API
	//
	// Trigger price MUST be tick-rounded — a raw %.8f triggers -1111
	// "Precision is over the maximum defined for this asset" on coarse-tick
	// symbols (UNIUSDT tick 0.001 vs stop 8.86573 → the position ran
	// unprotected, 2026-09-20). Quantized to a real tick MULTIPLE (not just
	// decimal places — a 0.025 tick breaks the decimals-only shortcut),
	// falling back to decimal formatting when the tick size is unavailable.
	priceStr, ferr := t.formatAlgoTriggerPrice(symbol, stopPrice)
	if ferr != nil {
		return fmt.Errorf("failed to set stop-loss: %w", ferr)
	}
	_, err := t.client.NewCreateAlgoOrderService().
		Symbol(symbol).
		Side(side).
		PositionSide(posSide).
		Type(futures.AlgoOrderTypeStopMarket).
		TriggerPrice(priceStr).
		WorkingType(futures.WorkingTypeContractPrice).
		ClosePosition(true).
		ClientAlgoId(getBrOrderID()).
		Do(context.Background())

	if err != nil && isStaleProtectiveOrder(err) {
		// -4130: an open GTE+closePosition algo in this direction already
		// exists (MUBARAKUSDT 09-29: the watchdog's naked-position fallback
		// fired before the pending-fill path placed the plan stop). The old
		// leg carries a STALE trigger — cancel this side's SL legs once and
		// re-place at the new price; if the cancel itself fails, surface the
		// ORIGINAL error (the position stays protected at the old level).
		previous, readErr := t.GetOpenOrders(symbol)
		if readErr != nil {
			return fmt.Errorf("cannot snapshot original stop: %w", readErr)
		}

		originals := []types.OpenOrder{}
		for _, old := range previous {
			if (old.Type == "STOP_MARKET" || old.Type == "STOP") && strings.EqualFold(old.PositionSide, string(posSide)) && old.OrderID != "" && old.StopPrice > 0 {
				originals = append(originals, old)
			}
		}
		if len(originals) == 0 {
			return fmt.Errorf("stale stop replacement: original protection unknown: %w", err)
		}
		for _, old := range originals {
			if cerr := t.CancelProtectiveOrder(symbol, old); cerr != nil {
				return fmt.Errorf("stale stop %s cancel failed: %w", old.OrderID, cerr)
			}
		}
		if _, err2 := t.client.NewCreateAlgoOrderService().
			Symbol(symbol).
			Side(side).
			PositionSide(posSide).
			Type(futures.AlgoOrderTypeStopMarket).
			TriggerPrice(priceStr).
			WorkingType(futures.WorkingTypeContractPrice).
			ClosePosition(true).
			ClientAlgoId(getBrOrderID()).
			Do(context.Background()); err2 != nil {
			for _, old := range originals {
				_, restoreErr := t.client.NewCreateAlgoOrderService().Symbol(symbol).Side(side).PositionSide(posSide).Type(futures.AlgoOrderTypeStopMarket).TriggerPrice(strconv.FormatFloat(old.StopPrice, 'f', -1, 64)).WorkingType(futures.WorkingTypeContractPrice).ClosePosition(true).ClientAlgoId(getBrOrderID()).Do(context.Background())
				if restoreErr != nil {
					return fmt.Errorf("replacement rejected (%v), original stop restore failed: %w", err2, restoreErr)
				}
				restored, verifyErr := t.GetOpenOrders(symbol)
				confirmed := false
				for _, o := range restored {
					if o.ClosePosition && (o.Type == "STOP_MARKET" || o.Type == "STOP") && strings.EqualFold(o.PositionSide, string(posSide)) && math.Abs(o.StopPrice-old.StopPrice)/old.StopPrice < 0.001 {
						confirmed = true
					}
				}
				if verifyErr != nil || !confirmed {
					return fmt.Errorf("replacement rejected (%v), original stop restoration unconfirmed: %v", err2, verifyErr)
				}
				break
			}
			return fmt.Errorf("failed to set stop-loss: stale algo leg cancelled but re-place failed: %w", err2)
		}
		logger.Infof("  ♻️ Stop-loss replaced: stale -4130 leg cancelled, new trigger %.4f (Algo Order)", stopPrice)
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to set stop-loss: %w", err)
	}

	logger.Infof("  Stop-loss price set (Algo Order): %.4f", stopPrice)
	return nil
}

// SetTakeProfit sets take-profit order using new Algo Order API
// Binance has migrated stop orders to Algo Order system (error -4120 STOP_ORDER_SWITCH_ALGO)
func (t *FuturesTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	var side futures.SideType
	var posSide futures.PositionSideType

	if positionSide == "LONG" {
		side = futures.SideTypeSell
		posSide = futures.PositionSideTypeLong
	} else {
		side = futures.SideTypeBuy
		posSide = futures.PositionSideTypeShort
	}

	// Use new Algo Order API — tick-rounded trigger, same -1111 rationale
	// as SetStopLoss above.
	//
	// quantity > 0 = PARTIAL TP (tp_close_fraction < 1): only that slice
	// closes at the trigger, the remainder stays on as a trend-runner under
	// the trailing stop (09-21 user experiment — a resting full-size TP
	// structurally sold every spike top: BTCUSDT TP filled 83000, price
	// printed 84275 in the same minute). quantity <= 0 keeps the legacy
	// closePosition mode. If the fraction underflows the symbol's lot step
	// the order falls back to full-close rather than placing a rejected
	// dust order.
	priceStr, ferr := t.formatAlgoTriggerPrice(symbol, takeProfitPrice)
	if ferr != nil {
		return fmt.Errorf("failed to set take-profit: %w", ferr)
	}
	algo := t.client.NewCreateAlgoOrderService().
		Symbol(symbol).
		Side(side).
		PositionSide(posSide).
		Type(futures.AlgoOrderTypeTakeProfitMarket).
		TriggerPrice(priceStr).
		WorkingType(futures.WorkingTypeContractPrice)
	if quantity > 0 {
		qtyStr, qerr := t.FormatQuantity(symbol, quantity)
		qty, _ := strconv.ParseFloat(qtyStr, 64)
		if qerr == nil && qty > 0 {
			// NO ReduceOnly here: with positionSide LONG/SHORT Binance
			// rejects the redundant param (-1106 "Parameter 'reduceonly'
			// sent when not required") and the whole TP leg fails to place
			// (AKEUSDT/PHAUSDT 09-22 — positions ran with SL only). The
			// quantity alone bounds the close; positionSide implies the
			// reduce direction.
			algo = algo.Quantity(qtyStr)
		} else {
			// Fraction underflowed the lot step — full close instead of a
			// rejected order (which would leave the TP leg missing).
			algo = algo.ClosePosition(true)
		}
	} else {
		algo = algo.ClosePosition(true)
	}
	_, err := algo.ClientAlgoId(getBrOrderID()).
		Do(context.Background())

	// -4130 on a TP: an open TAKE_PROFIT* leg exists in this direction. For
	// TPs the -4130 block is unconditional (any GTE+closePosition TP), so
	// retrying without cancelling can never succeed — mirror the SL leg's
	// cancel-and-replace. review 2026-10-11: reuse the first request's shape
	// so a quantity-limited TP cannot silently become a full-position close.
	// Only the client ID changes; quantity/closePosition and side stay intact.
	if err != nil && isStaleProtectiveOrder(err) {
		if cerr := t.CancelTakeProfitOrdersForSide(symbol, string(posSide)); cerr != nil {
			return fmt.Errorf("failed to set take-profit: stale algo leg replacement failed at cancel: %w", cerr)
		}
		if _, err2 := algo.ClientAlgoId(getBrOrderID()).
			Do(context.Background()); err2 != nil {
			return fmt.Errorf("failed to set take-profit: stale algo leg cancelled but re-place failed: %w", err2)
		}
		logger.Infof("  ♻️ Take-profit replaced: stale -4130 leg cancelled, new trigger %.4f (Algo Order, original close shape)", takeProfitPrice)
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to set take-profit: %w", err)
	}

	logger.Infof("  Take-profit price set (Algo Order): %.4f", takeProfitPrice)
	return nil
}

// GetOrderStatus gets order status
func (t *FuturesTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	getService := t.client.NewGetOrderService().Symbol(symbol)
	if clientID, ok := strings.CutPrefix(orderID, ClientOrderRefPrefix); ok && clientID != "" {
		// UNKNOWN-submit entry still tracked by its client order ID: the
		// result carries the real exchange orderId once Binance knows it.
		getService = getService.OrigClientOrderID(clientID)
	} else {
		// Convert orderID to int64
		orderIDInt, err := strconv.ParseInt(orderID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid order ID: %s", orderID)
		}
		getService = getService.OrderID(orderIDInt)
	}
	order, err := getService.Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	// Parse execution price
	avgPrice, _ := strconv.ParseFloat(order.AvgPrice, 64)
	executedQty, _ := strconv.ParseFloat(order.ExecutedQuantity, 64)

	result := map[string]interface{}{
		"orderId":     order.OrderID,
		"symbol":      order.Symbol,
		"status":      string(order.Status),
		"avgPrice":    avgPrice,
		"executedQty": executedQty,
		"side":        string(order.Side),
		"type":        string(order.Type),
		"time":        order.Time,
		"updateTime":  order.UpdateTime,
	}

	// Binance futures commission fee needs to be obtained through GetUserTrades, not retrieved here for now
	// Can be obtained later through WebSocket or separate query
	result["commission"] = 0.0

	return result, nil
}

// DebugListOpenAlgoOrders returns one summary line per open algo order on
// the symbol (maintenance-CLI helper, cmd/listalgos).
func (t *FuturesTrader) DebugListOpenAlgoOrders(symbol string) []string {
	orders, err := t.client.NewListOpenAlgoOrdersService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return []string{fmt.Sprintf("list failed: %v", err)}
	}
	out := make([]string, 0, len(orders))
	for _, a := range orders {
		out = append(out, fmt.Sprintf("id=%v type=%s side=%s posSide=%s trigger=%s qty=%s status=%s",
			a.AlgoId, a.OrderType, a.Side, a.PositionSide, a.TriggerPrice, a.Quantity, a.AlgoStatus))
	}
	return out
}

func (t *FuturesTrader) CancelProtectiveOrder(symbol string, o types.OpenOrder) error {
	id, err := strconv.ParseInt(o.OrderID, 10, 64)
	if err != nil {
		return err
	}
	if o.Algo {
		_, err = t.client.NewCancelAlgoOrderService().AlgoID(id).Do(context.Background())
		return err
	}
	return t.CancelOrder(symbol, o.OrderID)
}
