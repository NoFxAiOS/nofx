package binance

import (
	"fmt"
	"math"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	notify "nofx/telegram/notify"
	"nofx/trader/types"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// syncState stores the last sync time (Unix ms) for incremental sync
var (
	binanceSyncState      = make(map[string]int64) // exchangeID -> lastSyncTimeMs (Unix ms)
	binanceSyncStateMutex sync.RWMutex
	// review 2026-10-07 B2-A: all clients and entry points share one mutex per account.
	binanceSyncAccounts sync.Map // exchangeID -> *sync.Mutex
)

// Request-weight budget (Binance: 2400/min per IP, shared by every client):
// each sync used to spend ~100 weight (two income probes at 30 each + one
// 5-weight userTrades per candidate symbol) every 30 s per trader, even when
// the account was idle. The mechanisms below cut that without losing trade
// attribution. All bookkeeping is PER TRADER (traderID+exchangeID): traders
// sharing one exchange account each keep syncing on their own schedule, so
// one trader's probe or fingerprint state never suppresses another's queries.
//   - probe throttle: the symbol-DISCOVERY income probes (COMMISSION and
//     REALIZED_PNL, 60 weight) run every orderSyncProbeEvery, over a window
//     that starts at the previous probe (with overlap) instead of at the last
//     fill time — which on an idle account reached back hours or days and made
//     every probe return, and every sync re-query, every symbol of that span.
//   - fingerprint skip: a symbol known only because it is held / locally OPEN
//     is not re-queried while its exchange position is unchanged since its
//     last query (a fill always changes the position; net-zero round trips are
//     caught by the next probe run).
//   - carry-over: symbols whose query failed or filled its page are retried
//     next run, so the throttled probe window cannot lose them.
const (
	orderSyncProbeEvery   = 2 * time.Minute
	orderSyncProbeOverlap = 10 * time.Minute // income rows can lag the fill
	orderSyncTradesLimit  = 500
)

// orderSyncAcct is the per-trader sync bookkeeping.
type orderSyncAcct struct {
	lastProbe time.Time         // start time of the last fully successful probe pair
	pending   map[string]bool   // symbols to re-query next run (failed / truncated at the page limit)
	queriedFP map[string]string // symbol -> exchange position fingerprint at its last query
}

var orderSyncAccts = map[string]*orderSyncAcct{} // guarded by binanceSyncStateMutex

func orderSyncAcctLocked(traderID, exchangeID string) *orderSyncAcct {
	key := traderID + "|" + exchangeID
	a := orderSyncAccts[key]
	if a == nil {
		a = &orderSyncAcct{pending: map[string]bool{}, queriedFP: map[string]string{}}
		orderSyncAccts[key] = a
	}
	return a
}

// positionFingerprints returns symbol -> a stable description of the open
// exchange position(s) on that symbol.
func (t *FuturesTrader) positionFingerprints() (map[string]string, error) {
	positions, err := t.GetPositions()
	if err != nil {
		return nil, err
	}
	parts := map[string][]string{}
	for _, pos := range positions {
		sym, _ := pos["symbol"].(string)
		if sym == "" {
			continue
		}
		side, _ := pos["side"].(string)
		amt, _ := pos["positionAmt"].(float64)
		parts[sym] = append(parts[sym], fmt.Sprintf("%s:%g", side, amt))
	}
	out := make(map[string]string, len(parts))
	for sym, ps := range parts {
		sort.Strings(ps)
		out[sym] = strings.Join(ps, "|")
	}
	return out, nil
}

// SyncOrdersFromBinance syncs Binance Futures trade history to local database
// Uses COMMISSION detection + fromId for efficient incremental sync
// Also creates/updates position records to ensure orders/fills/positions data consistency
//
// Direct callers (orphan reconciliation, tests) always run the full discovery
// probes; only the periodic loop (StartOrderSync) throttles them.
func (t *FuturesTrader) SyncOrdersFromBinance(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	return t.syncOrders(traderID, exchangeID, exchangeType, st, true)
}

func (t *FuturesTrader) syncOrders(traderID string, exchangeID string, exchangeType string, st *store.Store, forceProbe bool) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}
	accountLock, _ := binanceSyncAccounts.LoadOrStore(exchangeID, &sync.Mutex{})
	mu := accountLock.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	orderStore := st.Order()

	// Get last sync time (Unix ms) - first try memory, then database, then default
	binanceSyncStateMutex.RLock()
	lastSyncTimeMs, exists := binanceSyncState[exchangeID]
	binanceSyncStateMutex.RUnlock()

	nowMs := time.Now().UTC().UnixMilli()
	if !exists {
		// Try to get last fill time from database (persist across restarts)
		lastFillTimeMs, err := orderStore.GetLastFillTimeByExchange(exchangeID)
		if err == nil && lastFillTimeMs > 0 {
			// If recovered time is in the future, it's clearly wrong - use default
			if lastFillTimeMs > nowMs {
				logger.Infof("⚠️ DB sync time %d is in the future (now: %d), using default",
					lastFillTimeMs, nowMs)
				lastSyncTimeMs = nowMs - 24*60*60*1000 // 24 hours ago
			} else {
				// review 2026-10-07 B2-C: overlap discovery across restart and deduplicate by fill ID.
				lastSyncTimeMs = lastFillTimeMs - (5 * time.Minute).Milliseconds()
				logger.Infof("📅 Recovered last sync time from DB: %s (UTC)",
					time.UnixMilli(lastSyncTimeMs).UTC().Format("2006-01-02 15:04:05"))
			}
		} else {
			// First sync: go back 24 hours
			lastSyncTimeMs = nowMs - 24*60*60*1000
			logger.Infof("📅 First sync, starting from 24 hours ago: %s (UTC)",
				time.UnixMilli(lastSyncTimeMs).UTC().Format("2006-01-02 15:04:05"))
		}
	}

	logger.Infof("🔄 Syncing Binance trades from: %s (UTC) [ms: %d, now: %d]",
		time.UnixMilli(lastSyncTimeMs).UTC().Format("2006-01-02 15:04:05"), lastSyncTimeMs, nowMs)

	// Step 1: Get max trade IDs from local DB for incremental sync
	maxTradeIDs, err := orderStore.GetMaxTradeIDsByExchange(exchangeID)
	if err != nil {
		logger.Infof("  ⚠️ Failed to get max trade IDs: %v, will use time-based query", err)
		maxTradeIDs = make(map[string]int64)
	}

	// Step 2: Detect symbols to sync using multiple methods
	// COMMISSION detection may miss trades (VIP users, BNB discount, 0-fee trades)
	symbolMap := make(map[string]bool)             // every symbol to query this run
	lastSyncTime := time.UnixMilli(lastSyncTimeMs) // Convert to time.Time for API calls

	runStart := time.Now()
	binanceSyncStateMutex.Lock()
	acct := orderSyncAcctLocked(traderID, exchangeID)
	lastProbe := acct.lastProbe
	pendingNow := make([]string, 0, len(acct.pending))
	for s := range acct.pending {
		pendingNow = append(pendingNow, s)
	}
	prevFP := make(map[string]string, len(acct.queriedFP))
	for s, fp := range acct.queriedFP {
		prevFP[s] = fp
	}
	binanceSyncStateMutex.Unlock()

	probeDue := forceProbe || lastProbe.IsZero() || runStart.Sub(lastProbe) >= orderSyncProbeEvery
	// Probe window: from the last fill (as before) but no earlier than the
	// previous successful probe minus an overlap — see the file-level note.
	probeSince := lastSyncTime
	if !lastProbe.IsZero() {
		if ps := lastProbe.Add(-orderSyncProbeOverlap); ps.After(probeSince) {
			probeSince = ps
		}
	}

	for _, s := range pendingNow {
		symbolMap[s] = true
	}
	if len(pendingNow) > 0 {
		logger.Infof("  📋 Pending (retry/continuation) symbols: %d - %v", len(pendingNow), pendingNow)
	}

	probesOK := probeDue
	if probeDue {
		// Method 1: COMMISSION income detection
		commissionSymbols, err := t.GetCommissionSymbols(probeSince)
		if err != nil {
			probesOK = false
			if IsAuthOrIPError(err) {
				return WrapAuthError(fmt.Sprintf("OrderSync commission probe (exchange %s)", exchangeID), err)
			}
			logger.Warnf("  ⚠️ [%s] Failed to get commission symbols: %v", exchangeID, err)
		} else {
			logger.Infof("  📋 COMMISSION symbols found: %d - %v", len(commissionSymbols), commissionSymbols)
			for _, s := range commissionSymbols {
				symbolMap[s] = true
			}
		}
	}

	// Method 2: Always include active positions (catches trades that COMMISSION missed)
	fps, err := t.positionFingerprints()
	if err != nil {
		if IsAuthOrIPError(err) {
			return WrapAuthError(fmt.Sprintf("OrderSync position probe (exchange %s)", exchangeID), err)
		}
		logger.Warnf("  ⚠️ [%s] Failed to get position symbols: %v", exchangeID, err)
		fps = nil
	} else {
		logger.Infof("  📋 Position symbols found: %d", len(fps))
	}
	held := make(map[string]bool, len(fps))
	for s := range fps {
		held[s] = true
	}

	// Method 3: Include symbols from recent fills in DB (in case some were partially synced)
	recentSymbols, _ := orderStore.GetRecentFillSymbolsByExchange(exchangeID, lastSyncTimeMs)
	logger.Infof("  📋 Recent fill symbols found: %d - %v", len(recentSymbols), recentSymbols)
	for _, s := range recentSymbols {
		symbolMap[s] = true
	}

	// review 2026-10-07 B2-C: a locally OPEN position may already be flat on the exchange.
	openSymbols, err := st.Position().GetOpenSymbolsByExchange(exchangeID)
	if err != nil {
		return fmt.Errorf("failed to discover DB open position symbols: %w", err)
	}
	for _, s := range openSymbols {
		held[s] = true
	}

	// Held / locally-open symbols have no activity evidence of their own: on a
	// probe run they are all queried (full coverage every orderSyncProbeEvery);
	// between probes only when their exchange position changed since the last
	// query (a fill always changes it).
	skippedHeld := 0
	for s := range held {
		if symbolMap[s] {
			continue
		}
		cur, open := fps[s]
		if !open {
			cur = "flat"
		}
		if probeDue || fps == nil {
			symbolMap[s] = true
		} else if prev, seen := prevFP[s]; !seen || prev != cur {
			symbolMap[s] = true
		} else {
			skippedHeld++
		}
	}
	if skippedHeld > 0 {
		logger.Infof("  ⏭️ %d held symbols unchanged since last query, skipped", skippedHeld)
	}

	// Method 4: ALWAYS query REALIZED_PNL income to find symbols with closed trades
	// This catches trades that COMMISSION missed (VIP users, BNB fee discount)
	// IMPORTANT: Must run always, not just when symbolMap is empty,
	// because a position might be fully closed (no active position) but have PnL
	// (always = on every probe run; probes are throttled, see orderSyncProbeEvery).
	if probeDue {
		pnlSymbols, err := t.GetPnLSymbols(probeSince)
		if err != nil {
			probesOK = false
			if IsAuthOrIPError(err) {
				return WrapAuthError(fmt.Sprintf("OrderSync PnL probe (exchange %s)", exchangeID), err)
			}
			logger.Warnf("  ⚠️ [%s] Failed to get PnL symbols: %v", exchangeID, err)
		} else {
			logger.Infof("  📋 REALIZED_PNL symbols found: %d - %v", len(pnlSymbols), pnlSymbols)
			for _, s := range pnlSymbols {
				symbolMap[s] = true
			}
		}
	}
	if probeDue && probesOK {
		binanceSyncStateMutex.Lock()
		orderSyncAcctLocked(traderID, exchangeID).lastProbe = runStart
		binanceSyncStateMutex.Unlock()
	}

	var changedSymbols []string
	for s := range symbolMap {
		changedSymbols = append(changedSymbols, s)
	}
	sort.Strings(changedSymbols)

	if len(changedSymbols) == 0 {
		logger.Infof("📭 No symbols with new trades to sync")
		// DON'T update lastSyncTime to current time here!
		// Keep using the last actual trade time from DB to avoid creating gaps
		return nil
	}

	logger.Infof("📊 Found %d symbols with new trades: %v", len(changedSymbols), changedSymbols)

	// Step 3: Query trades for changed symbols using fromId (incremental) or time-based (new symbols)
	var allTrades []types.TradeRecord
	var failedSymbols []string
	apiCalls := 0
	newPending := map[string]bool{}
	queriedOK := map[string]string{} // symbol -> position fingerprint at query time
	for _, symbol := range changedSymbols {
		var trades []types.TradeRecord
		var queryErr error

		if lastID, ok := maxTradeIDs[symbol]; ok && lastID > 0 {
			// Incremental sync: query from last known trade ID
			trades, queryErr = t.GetTradesForSymbolFromID(symbol, lastID+1, orderSyncTradesLimit)
		} else {
			// New symbol or first sync: query by time
			trades, queryErr = t.GetTradesForSymbol(symbol, lastSyncTime, orderSyncTradesLimit)
		}
		apiCalls++

		if queryErr != nil {
			logger.Infof("  ⚠️ Failed to get trades for %s: %v", symbol, queryErr)
			failedSymbols = append(failedSymbols, symbol)
			newPending[symbol] = true // the throttled probe window may no longer reach back to it
			continue
		}
		if len(trades) >= orderSyncTradesLimit {
			newPending[symbol] = true // page full: more fills may follow
		}
		fp, open := fps[symbol]
		if !open {
			fp = "flat"
		}
		queriedOK[symbol] = fp
		allTrades = append(allTrades, trades...)
	}
	binanceSyncStateMutex.Lock()
	acct = orderSyncAcctLocked(traderID, exchangeID)
	acct.pending = newPending
	for s := range newPending {
		delete(acct.queriedFP, s)
	}
	if fps != nil {
		for s, fp := range queriedOK {
			if !newPending[s] {
				acct.queriedFP[s] = fp
			}
		}
	}
	binanceSyncStateMutex.Unlock()

	logger.Infof("📥 Received %d trades from Binance (%d API calls)", len(allTrades), apiCalls)

	if len(allTrades) == 0 {
		// No trades returned, but symbols were detected - might be false positive from COMMISSION/PnL detection
		// Don't update lastSyncTime, keep using DB value
		if len(failedSymbols) > 0 {
			logger.Infof("  ⚠️ %d symbols failed: %v", len(failedSymbols), failedSymbols)
		}
		return nil
	}

	// Sort trades by time ASC (oldest first) for proper position building
	sort.Slice(allTrades, func(i, j int) bool {
		return allTrades[i].Time.UnixMilli() < allTrades[j].Time.UnixMilli()
	})

	// Process trades one by one
	syncedCount := 0

	skippedCount := 0
	exchangeClosedSymbols := make(map[string]bool) // symbols with close trades this round (raw exchange symbol)
	// review 2026-10-07 B2-B: a trade that fails to account blocks only the
	// REST of its own symbol this run (later fills on that symbol depend on
	// it) — other symbols still book, the orphan sweep below still runs, and
	// failedSymbols keeps the cursor from advancing so the failure retries.
	// Returning early here made one poison trade freeze ALL bookkeeping.
	accountingFailed := make(map[string]bool)
	for _, trade := range allTrades {
		if accountingFailed[trade.Symbol] {
			continue
		}
		// Normalize symbol
		symbol := market.Normalize(trade.Symbol)

		// Determine order action based on side and position side
		orderAction := t.determineOrderAction(trade.Side, trade.PositionSide, trade.RealizedPnL)

		// Determine position side for position builder
		positionSide := trade.PositionSide
		if positionSide == "" || positionSide == "BOTH" {
			// Infer from order action
			if strings.Contains(orderAction, "long") {
				positionSide = "LONG"
			} else {
				positionSide = "SHORT"
			}
		}

		// Normalize side
		side := strings.ToUpper(trade.Side)

		// Create order record - use Unix milliseconds UTC
		tradeTimeMs := trade.Time.UTC().UnixMilli()
		orderRecord := &store.TraderOrder{
			TraderID:        traderID,
			ExchangeID:      exchangeID,
			ExchangeType:    exchangeType,
			ExchangeOrderID: trade.TradeID,
			Symbol:          symbol,
			Side:            side,
			PositionSide:    positionSide,
			Type:            "MARKET",
			OrderAction:     orderAction,
			Quantity:        trade.Quantity,
			Price:           trade.Price,
			Status:          "FILLED",
			FilledQuantity:  trade.Quantity,
			AvgFillPrice:    trade.Price,
			Commission:      trade.Fee,
			FilledAt:        tradeTimeMs,
			CreatedAt:       tradeTimeMs,
			UpdatedAt:       tradeTimeMs,
		}

		// Create fill record - use Unix milliseconds UTC
		fillRecord := &store.TraderFill{
			TraderID:        traderID,
			ExchangeID:      exchangeID,
			ExchangeType:    exchangeType,
			OrderID:         orderRecord.ID,
			ExchangeOrderID: trade.PositionOrderID(),
			ExchangeTradeID: trade.TradeID,
			Symbol:          symbol,
			Side:            side,
			Price:           trade.Price,
			Quantity:        trade.Quantity,
			QuoteQuantity:   trade.Price * trade.Quantity,
			Commission:      trade.Fee,
			CommissionAsset: "USDT",
			RealizedPnL:     trade.RealizedPnL,
			IsMaker:         trade.IsMaker,
			CreatedAt:       tradeTimeMs,
		}

		// review 2026-10-07 B2-A/B2-B: commit order, fill and position together.
		// On failure the transaction rolled back (no fill receipt), so the
		// trade is retried next run; the cursor does not advance past it.
		applied, err := orderStore.ApplyTrade(orderRecord, fillRecord)
		if err != nil {
			logger.Errorf("  ❌ Failed to account for trade %s %s (rolled back, retried next sync): %v", trade.Symbol, trade.TradeID, err)
			accountingFailed[trade.Symbol] = true
			failedSymbols = append(failedSymbols, trade.Symbol)
			continue
		}
		if !applied {
			skippedCount++
			continue
		}
		if orderAction == "close_long" || orderAction == "close_short" {
			exchangeClosedSymbols[trade.Symbol] = true
		}

		syncedCount++
		logger.Infof("  ✅ Synced trade: %s %s %s qty=%.6f price=%.6f pnl=%.2f fee=%.6f action=%s time=%s(UTC)",
			trade.TradeID, symbol, side, trade.Quantity, trade.Price, trade.RealizedPnL, trade.Fee, orderAction,
			trade.Time.UTC().Format("01-02 15:04:05"))

		// Notify on every executed fill (covers AI orders, risk closes,
		// exchange-side stop triggers and manual closes alike).
		notifyFill(t.notifyLabel(), traderID, st, symbol, orderAction, trade)
	}

	// Update lastSyncTime to the LATEST trade time (not current time!)
	// This ensures next sync starts from where we left off, not from "now"
	// allTrades is already sorted by time ASC, so last element is the latest
	if len(accountingFailed) > 0 {
		binanceSyncStateMutex.Lock()
		acct = orderSyncAcctLocked(traderID, exchangeID)
		for s := range accountingFailed {
			acct.pending[s] = true
			delete(acct.queriedFP, s)
		}
		binanceSyncStateMutex.Unlock()
	}
	if len(allTrades) > 0 && len(failedSymbols) == 0 {
		latestTradeTimeMs := allTrades[len(allTrades)-1].Time.UTC().UnixMilli()
		binanceSyncStateMutex.Lock()
		binanceSyncState[exchangeID] = latestTradeTimeMs
		binanceSyncStateMutex.Unlock()
		logger.Infof("📅 Updated lastSyncTime to latest trade: %s (UTC)",
			time.UnixMilli(latestTradeTimeMs).UTC().Format("2006-01-02 15:04:05"))
	} else if len(failedSymbols) > 0 {
		logger.Infof("  ⚠️ %d symbols failed, not updating lastSyncTime to retry next time: %v", len(failedSymbols), failedSymbols)
	}

	// Re-read positions for orphan cleanup: a cached flat snapshot can predate
	// a new entry. Only cancel individually identified program protection on
	// flat hedge sides; manual orders and the live opposite side are preserved.
	if len(exchangeClosedSymbols) > 0 {
		t.InvalidatePositionCache()
		if positions, err := t.GetPositions(); err == nil {
			openSides := make(map[string]map[string]bool)
			for _, pos := range positions {
				if s, ok := pos["symbol"].(string); ok {
					if openSides[s] == nil {
						openSides[s] = make(map[string]bool)
					}
					side, _ := pos["side"].(string)
					if side == "long" || side == "short" {
						openSides[s][strings.ToUpper(side)] = true
					} else {
						// An unrecognized position must not authorize cleanup.
						openSides[s]["LONG"], openSides[s]["SHORT"] = true, true
					}
				}
			}
			for symbol := range exchangeClosedSymbols {
				for _, side := range []string{"LONG", "SHORT"} {
					if !openSides[symbol][side] {
						if err := t.CancelProgramProtectiveOrdersForSide(symbol, side); err != nil {
							logger.Warnf("Failed to clean up orphaned program protection for %s %s: %v", symbol, side, err)
						}
					}
				}
			}
		} else {
			logger.Warnf("Skipping orphaned protection cleanup: fresh positions unavailable: %v", err)
		}
	}

	logger.Infof("✅ Binance order sync completed: %d new trades synced, %d skipped (already exist)", syncedCount, skippedCount)
	return nil
}

// getPositionSymbols returns list of symbols that have active positions
// Used as fallback when COMMISSION detection fails
func (t *FuturesTrader) getPositionSymbols() ([]string, error) {
	positions, err := t.GetPositions()
	if err != nil {
		return nil, err
	}

	var symbols []string
	for _, pos := range positions {
		if symbol, ok := pos["symbol"].(string); ok && symbol != "" {
			symbols = append(symbols, symbol)
		}
	}
	return symbols, nil
}

// determineOrderAction determines the order action based on trade data
func (t *FuturesTrader) determineOrderAction(side, positionSide string, realizedPnL float64) string {
	side = strings.ToUpper(side)
	positionSide = strings.ToUpper(positionSide)

	// review 2026-10-07 B1-7: in hedge mode (positionSide LONG/SHORT)
	// (positionSide, side) fully determine open vs close. Classifying by
	// realizedPnL != 0 mislabeled a breakeven close (PnL exactly 0) as an
	// open and the builder then ADDED quantity to the opposite row.
	switch positionSide {
	case "LONG":
		if side == "BUY" {
			return "open_long"
		}
		return "close_long"
	case "SHORT":
		if side == "SELL" {
			return "open_short"
		}
		return "close_short"
	}

	// One-way mode (positionSide "" or "BOTH"): direction is not encoded, so
	// keep the realized-PnL heuristic — a trade with realized PnL is a close.
	isClose := realizedPnL != 0
	if side == "BUY" {
		if isClose {
			return "close_short" // Buying to close short
		}
		return "open_long"
	}
	if isClose {
		return "close_long" // Selling to close long
	}
	return "open_short"
}

// StartOrderSync starts background order sync task for Binance
func (t *FuturesTrader) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	first := true
	run := func() {
		if first {
			logger.Infof("🔄 [%s] Running initial Binance order sync (exchange %s)...", traderID, exchangeID)
			first = false
		}
		if err := t.syncOrders(traderID, exchangeID, exchangeType, st, false); err != nil {
			logger.Infof("⚠️  [%s] Binance order sync failed (exchange %s): %v", traderID, exchangeID, err)
			if IsAuthOrIPError(err) && !t.syncAuthAlerted.Swap(true) {
				notify.Notify("ALERT", t.notifyLabel(), fmt.Sprintf("<b>🚨 Binance 认证/IP 校验失败（exchange %s），订单同步不可用</b>\n请检查该 API Key 的 IP 白名单与合约权限（当前出口 IP 见日志）；订单同步保留恢复探测，配置更新会重建客户端。", exchangeID))
			}
		} else {
			// Recovered — re-arm the alert for future failures.
			t.syncAuthAlerted.Store(false)
		}
	}
	if t.startOrderSyncLoop(interval, run) {
		logger.Infof("🔄 Binance order sync started (interval: %v)", interval)
	} else {
		logger.Infof("🔄 [%s] Binance order sync already running; duplicate start ignored", traderID)
	}
}

// startOrderSyncLoop owns the ticker lifecycle. Keeping this helper free of
// exchange calls makes stop/duplicate-start behavior deterministic in tests.
func (t *FuturesTrader) startOrderSyncLoop(interval time.Duration, run func()) bool {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	t.orderSyncMu.Lock()
	defer t.orderSyncMu.Unlock()
	if t.orderSyncStop != nil {
		return false
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	t.orderSyncStop = stop
	t.orderSyncDone = done

	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		types.RunSyncSafely(run) // initial sync immediately
		for {
			select {
			case <-ticker.C:
				types.RunSyncSafely(run)
			case <-stop:
				return
			}
		}
	}()
	return true
}

// StopOrderSync stops and joins the background sync loop. This must run before
// a FuturesTrader is removed during API-key/exchange config reload, otherwise
// the discarded client survives through its ticker and keeps using stale keys.
func (t *FuturesTrader) StopOrderSync() {
	t.orderSyncMu.Lock()
	stop, done := t.orderSyncStop, t.orderSyncDone
	if stop == nil {
		t.orderSyncMu.Unlock()
		return
	}
	close(stop)
	<-done
	t.orderSyncStop = nil
	t.orderSyncDone = nil
	t.orderSyncMu.Unlock()
	logger.Infof("⏹ Binance order sync stopped")
}

// fillNotified suppresses duplicate fill alerts until a successful sync.
var fillNotified atomic.Bool

// commaFormat renders a number with thousands separators in the integer part,
// keeping the value's natural decimals (1425 → 1,425; 0.003289 unchanged).
func commaFormat(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = strings.TrimPrefix(s, "-")
	}
	intPart, decPart := s, ""
	if dot := strings.Index(s, "."); dot >= 0 {
		intPart, decPart = s[:dot], s[dot:]
	}
	var out []byte
	for i, ch := range []byte(intPart) {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, ch)
	}
	if neg {
		return "-" + string(out) + decPart
	}
	return string(out) + decPart
}

// notifyFill pushes a trade fill (open/close) to Telegram. It fires once per
// new synced trade — the sync dedup skips already-known trades, so repeats
// only happen on genuine new fills. Body is HTML (the notify layer renders it
// with parse_mode=HTML). label is the display name the message goes out under;
// traderID and st locate the position a close fill belongs to.
func notifyFill(label, traderID string, st *store.Store, symbol, orderAction string, trade types.TradeRecord) {
	switch orderAction {
	case "open_long", "open_short":
		if trade.RealizedPnL != 0 {
			return
		}
	case "close_long", "close_short":
	default:
		return
	}

	sym := notify.Escape(symbol)
	var title string
	var body strings.Builder
	switch orderAction {
	case "open_long":
		title = "🟢 开多 " + sym
	case "open_short":
		title = "🔻 开空 " + sym
	case "close_long":
		title = "🔴 平多 " + sym
	case "close_short":
		title = "🔵 平空 " + sym
	default:
		return
	}

	// Layout: headline / blank / trade numbers / blank / result block —
	// cramped walls of text bury the PnL on a phone screen.
	body.WriteString(fmt.Sprintf("数量  <code>%s</code>\n", commaFormat(trade.Quantity)))
	body.WriteString(fmt.Sprintf("价格  <code>%s</code>\n", commaFormat(trade.Price)))
	body.WriteString(fmt.Sprintf("价值  <code>%s USDT</code>\n", commaFormat(trade.Price*trade.Quantity)))
	body.WriteString(fmt.Sprintf("手续费  <code>%s USDT</code>", commaFormat(trade.Fee)))
	if orderAction == "close_long" || orderAction == "close_short" {
		body.WriteString("\n\n━━━━━━━━━━\n\n")
		body.WriteString(closeResultBlock(st, traderID, symbol, orderAction, trade))
		body.WriteString(fmt.Sprintf("\n\n<i>成交 %s (UTC)</i>", trade.Time.UTC().Format("01-02 15:04:05")))
	}
	notify.Notify("ORDER", label, "<b>"+title+"</b>\n\n"+body.String())
}

// closeFillResult is the after-commission result a close notice reports.
type closeFillResult struct {
	Net           float64 // realized PnL after commissions
	Gross         float64 // realized PnL as the exchange reports it (commissions excluded)
	Fee           float64 // commissions netted out of Gross, positive = cost
	Partial       bool    // the position is still open after this fill
	EntryFeeKnown bool    // false: the entry commission could not be attributed
}

// netCloseFill nets a close fill's commissions out of its realized PnL.
//   - Closed position (this fill closed it): the row holds the whole position —
//     every leg's PnL and every commission, entry included — so net is
//     RealizedPnL − Fee, the figure Binance's position history shows.
//   - Open position (partial close): the row's Fee less closeFeesSoFar (the
//     commissions of its close legs, this fill included) is the entry
//     commission; this fill carries its pro-rata share of it.
//   - No position: the fill's own commission only, entry commission not counted.
func netCloseFill(trade types.TradeRecord, pos *store.TraderPosition, closeFeesSoFar float64) closeFillResult {
	fillOnly := closeFillResult{Net: trade.RealizedPnL - trade.Fee, Gross: trade.RealizedPnL, Fee: trade.Fee}
	switch {
	case pos == nil:
		return fillOnly
	case pos.Status == "CLOSED":
		return closeFillResult{Net: pos.RealizedPnL - pos.Fee, Gross: pos.RealizedPnL, Fee: pos.Fee, EntryFeeKnown: true}
	case pos.Status != "OPEN":
		return fillOnly
	}
	entryFee := pos.Fee - closeFeesSoFar
	if pos.EntryQuantity <= 0 || entryFee < -1e-9 {
		return fillOnly
	}
	entryFee = math.Max(entryFee, 0)
	fee := trade.Fee + entryFee*trade.Quantity/pos.EntryQuantity
	return closeFillResult{Net: trade.RealizedPnL - fee, Gross: trade.RealizedPnL, Fee: fee, Partial: true, EntryFeeKnown: true}
}

// closeResultBlock renders the net-result block of a close notice. It looks up
// the position the fill belongs to in st; when that fails it falls back to the
// fill's own commission and the notice says the entry commission is missing.
func closeResultBlock(st *store.Store, traderID, symbol, orderAction string, trade types.TradeRecord) string {
	side, closeSide := "LONG", "SELL"
	if orderAction == "close_short" {
		side, closeSide = "SHORT", "BUY"
	}
	pos, err := st.Position().GetPositionForFill(traderID, symbol, side, trade.Time.UTC().UnixMilli())
	if err != nil {
		logger.Warnf("  ⚠️ Net PnL: position lookup failed for %s %s, showing fill-only result: %v", symbol, side, err)
		return formatCloseResult(netCloseFill(trade, nil, 0))
	}
	var closeFees float64
	if pos != nil && pos.Status == "OPEN" {
		if closeFees, err = st.Order().SumCloseCommission(traderID, symbol, closeSide, pos.EntryTime); err != nil {
			logger.Warnf("  ⚠️ Net PnL: close commission sum failed for %s %s, showing fill-only result: %v", symbol, side, err)
			return formatCloseResult(netCloseFill(trade, nil, 0))
		}
	}
	return formatCloseResult(netCloseFill(trade, pos, closeFees))
}

// formatCloseResult renders the result block of a close notice: the headline is
// the net figure, the italic line the gross, the commissions netted out, and
// what the figure does not cover.
func formatCloseResult(r closeFillResult) string {
	pnl := signedAmount(r.Net) + " USDT"
	if r.Net >= 0 {
		pnl = "✅ 盈利 <b>" + pnl + "</b>"
	} else {
		pnl = "❌ 亏损 <b>" + pnl + "</b>"
	}
	var notes []string
	if r.Partial {
		notes = append(notes, "部分平仓")
	}
	notes = append(notes, "毛利 "+signedAmount(r.Gross), "手续费 "+signedAmount(-r.Fee))
	if !r.EntryFeeKnown {
		notes = append(notes, "开仓手续费未计入")
	}
	notes = append(notes, "未含资金费")
	return "已实现盈亏（扣手续费）\n" + pnl + "\n<i>" + strings.Join(notes, " · ") + "</i>"
}

// signedAmount formats a USDT figure with its sign: 4 decimals below 0.1 in
// magnitude so a small result never prints as +0.00, 2 decimals otherwise.
// The sign comes from the number alone, so callers must not add one.
func signedAmount(v float64) string {
	if v == 0 {
		v = 0 // drop the sign of -0, which %+f would print as -0.0000
	}
	if math.Abs(v) < 0.1 {
		return fmt.Sprintf("%+.4f", v)
	}
	return fmt.Sprintf("%+.2f", v)
}
