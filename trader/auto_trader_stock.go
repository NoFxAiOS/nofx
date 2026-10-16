package trader

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"nofx/kernel/stockengine"
	"nofx/logger"
	"nofx/market/usstock"
	"nofx/store"
	"nofx/trader/binance_bstock"
	"nofx/trader/types"

	notify "nofx/telegram/notify"
)

// us_stock (design 2026-10-09 §5/§6): a strategy type with its own run cycle.
// Long-only trading of Binance spot bStock pairs, scheduled at fixed US/Eastern
// decision times plus a 60s protection tick. Paper mode (the default) runs the
// full data/decision/risk path against a simulated ledger and places no orders.
//
// Ownership rule (user also trades manually on this Binance account): the
// executor derives positions from spot BALANCES, so the program manages ONLY
// the quantity recorded in its own ai_managed trader_positions rows. Every
// sell passes an explicit quantity (never SellMarket(0)), protection covers the
// owned quantity only, and CancelAllOrders is never called from this path.

// newSpotStockTrader builds the live spot executor; tests inject a fake.
var newSpotStockTrader = func(apiKey, secret string) (types.SpotStockTrader, error) {
	return binance_bstock.NewBStockTrader(apiKey, secret), nil
}

// Market-data seams (tests inject canned data; production uses market/usstock).
var (
	stockLookupSymbol = usstock.LookupSymbol
	stockGetSeries    = usstock.GetSeries
	stockGetQuote     = usstock.GetQuote
	stockListUniverse = usstock.ListSymbols
	// stockGetUnderlyingSeries: Yahoo-only series for prompt context (4h).
	stockGetUnderlyingSeries = usstock.GetUnderlyingSeries
)

// Price-cycle scan seams (scan mode; tests inject canned reports).
var (
	stockGetScanReport = stockengine.GetScanReport
	stockWarmScan      = stockengine.WarmScanReport
	stockCachedScan    = stockengine.CachedScanReport
)

// stockNotify sends Telegram messages (test seam).
var stockNotify = notify.Notify

// Futures-monitor seams so a test can prove the us_stock branch in run() skips
// them. Both are nil in production.
var (
	onFuturesMonitorsStart func(*AutoTrader)
	onStockLoopStart       func(*AutoTrader)
)

const (
	stockProtectInterval = 60 * time.Second
	// stockWeeklyDrawdownHaltPct: design §6 risk circuit — when equity falls
	// more than this percent below the week's starting equity, no new
	// exposure is opened for the rest of the ET week (exits still managed).
	stockWeeklyDrawdownHaltPct = 10.0
	// stockStopLimitBuffer: the stop-limit sits this far below the trigger.
	stockStopLimitBuffer = 0.003
	// stockStopFallbackTicks: consecutive protection ticks with price at/below
	// the stop and the stop order still open before the program sells.
	stockStopFallbackTicks = 2
	stockSnapshotEvery     = 5 // equity snapshot every N protection ticks

	// Scan mode: the 60s tick refreshes the cached scan once it is older than
	// stockScanWarmAge (in the background); a decision cycle accepts a report up
	// to stockScanMaxAge old before it waits for a live rescan.
	stockScanWarmAge = 20 * time.Minute
	stockScanMaxAge  = 45 * time.Minute
)

// stockSeriesNeeds: bars requested per timeframe (design §3.2).
var stockSeriesNeeds = map[string]int{
	usstock.TF1d: 220, usstock.TF1w: 60, usstock.TF1h: 300, usstock.TF4h: 200, usstock.TF15m: 100,
}

// stockContextSymbols: market-background pairs (SPY / QQQ) when listed.
var stockContextSymbols = []string{"SPYBUSDT", "QQQBUSDT"}

// stockBenchmarkSymbol is the relative-strength benchmark of the cycle scan.
const stockBenchmarkSymbol = "SPYBUSDT"

type stockProtect struct {
	Stop, TakeProfit, Qty float64
}

type stockPending struct {
	Symbol, OrderID, Action string
	Qty, FilledQty, Limit   float64
	Stop, TakeProfit        float64
	PlacedAt                time.Time
}

// stockRuntime is the us_stock run state; guarded by mu where noted, the rest
// is only touched by the single loop goroutine (or under the execution mutex).
type stockRuntime struct {
	cfg *store.StockConfig // symbols filtered to the listed bStock pairs

	mu         sync.Mutex // guards lastPrices
	lastPrices map[string]float64

	weekKey         string
	weekStartEquity float64
	weekHalted      bool

	stopTicks   map[string]int
	lastProtect map[string]stockProtect
	pendings    map[string]*stockPending
	// Alert history is loop-owned, like protection reconciliation itself.
	protectionAlerts map[string]map[string]stockProtectionNotice
	tickCount        int
	lastSlot         time.Time
}

func (r *stockRuntime) price(symbol string) float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastPrices[symbol]
}

func (r *stockRuntime) setPrice(symbol string, p float64) {
	if p <= 0 {
		return
	}
	r.mu.Lock()
	if r.lastPrices == nil {
		r.lastPrices = map[string]float64{}
	}
	r.lastPrices[symbol] = p
	r.mu.Unlock()
}

// IsStockStrategy reports whether this trader runs the us_stock strategy.
func (at *AutoTrader) IsStockStrategy() bool {
	sc := at.config.StrategyConfig
	return sc != nil && sc.StrategyType == store.StrategyTypeUSStock && sc.StockConfig != nil
}

func (at *AutoTrader) stockClock() time.Time {
	if at.stockNow != nil {
		return at.stockNow()
	}
	return time.Now()
}

func (at *AutoTrader) stockPaper() bool {
	return at.config.StrategyConfig.StockConfig.IsPaper()
}

func (at *AutoTrader) stockLang() string {
	if at.config.StrategyConfig != nil && at.config.StrategyConfig.Language != "" {
		return at.config.StrategyConfig.Language
	}
	return "zh"
}

// stockMutex: live cycles serialize with the other traders of the same Binance
// account; paper cycles never touch the account, so they only serialize with
// themselves.
func (at *AutoTrader) stockMutex() *sync.Mutex {
	if at.stockPaper() {
		return &at.executionStateMu
	}
	return at.executionMutex()
}

func (at *AutoTrader) stockRunning() bool {
	at.isRunningMutex.RLock()
	defer at.isRunningMutex.RUnlock()
	return at.isRunning
}

// newStockExecutor validates the exchange and builds the live executor
// (nil in paper mode: paper places no orders and needs no credentials).
func newStockExecutor(config *AutoTraderConfig) (types.SpotStockTrader, error) {
	if config.Exchange != "binance" {
		return nil, fmt.Errorf("us_stock strategy requires a Binance exchange account (got %q)", config.Exchange)
	}
	if config.StrategyConfig.StockConfig.IsPaper() {
		return nil, nil
	}
	if newSpotStockTrader == nil {
		return nil, fmt.Errorf("us_stock executor not linked")
	}
	return newSpotStockTrader(config.BinanceAPIKey, config.BinanceSecretKey)
}

// resolveStockInitialBalance returns the P&L baseline of a us_stock trader:
// the spot account equity (live) or the paper ledger's starting cash (paper).
func resolveStockInitialBalance(config *AutoTraderConfig, st *store.Store, userID string, exec types.SpotStockTrader) (float64, error) {
	if config.StrategyConfig.StockConfig.IsPaper() {
		if st == nil {
			if config.InitialBalance > 0 {
				return config.InitialBalance, nil
			}
			return store.DefaultStockPaperCash, nil
		}
		acct, err := st.StockPaper().EnsureAccount(config.ID, config.InitialBalance)
		if err != nil {
			return 0, fmt.Errorf("paper ledger init failed: %w", err)
		}
		if config.InitialBalance > 0 {
			return config.InitialBalance, nil
		}
		if err := st.Trader().UpdateInitialBalance(userID, config.ID, acct.InitialCash); err != nil {
			logger.Infof("⚠️  [%s] Failed to save paper initial balance: %v", config.Name, err)
		}
		return acct.InitialCash, nil
	}
	if config.InitialBalance > 0 {
		return config.InitialBalance, nil
	}
	bal, err := exec.GetBalance()
	if err != nil {
		return 0, fmt.Errorf("initial balance not set and unable to fetch spot balance: %w", err)
	}
	eq := firstPositive(bal, "totalEquity", "total_equity", "totalWalletBalance", "balance")
	if eq <= 0 {
		return 0, fmt.Errorf("initial balance must be greater than 0, please set InitialBalance in config or ensure the spot account has USDT")
	}
	if st != nil {
		if err := st.Trader().UpdateInitialBalance(userID, config.ID, eq); err != nil {
			logger.Infof("⚠️  [%s] Failed to save initial balance to database: %v", config.Name, err)
		}
	}
	logger.Infof("✓ [%s] Auto-fetched initial balance: %.2f USDT", config.Name, eq)
	return eq, nil
}

func firstPositive(m map[string]interface{}, keys ...string) float64 {
	for _, k := range keys {
		if v, err := SafeFloat64(m, k); err == nil && v > 0 {
			return v
		}
	}
	return 0
}

// ---------------------------------------------------------------- scheduler

var stockET = func() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}
	return loc
}()

// nextStockDecisionTime returns the first decision instant strictly after
// `after`: the preset's "HH:MM" ET slots on NYSE trading days. A half day's
// 15:30 slot still fires (it falls in after-hours; the engine just rejects
// opens). Zero when none is found within three weeks.
func nextStockDecisionTime(after time.Time, slots []string) time.Time {
	type hm struct{ h, m int }
	var parsed []hm
	for _, s := range slots {
		var h, m int
		if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err == nil && h >= 0 && h < 24 && m >= 0 && m < 60 {
			parsed = append(parsed, hm{h, m})
		}
	}
	sort.Slice(parsed, func(i, j int) bool { return parsed[i].h*60+parsed[i].m < parsed[j].h*60+parsed[j].m })
	day := after.In(stockET)
	for i := 0; i < 21 && len(parsed) > 0; i++ {
		d := time.Date(day.Year(), day.Month(), day.Day()+i, 12, 0, 0, 0, stockET)
		if !usstock.IsTradingDay(d) {
			continue
		}
		for _, p := range parsed {
			t := time.Date(d.Year(), d.Month(), d.Day(), p.h, p.m, 0, 0, stockET)
			if t.After(after) {
				return t
			}
		}
	}
	return time.Time{}
}

// stockWeekStart is Monday 00:00 ET of the week containing t.
func stockWeekStart(t time.Time) time.Time {
	e := t.In(stockET)
	back := (int(e.Weekday()) + 6) % 7
	return time.Date(e.Year(), e.Month(), e.Day()-back, 0, 0, 0, 0, stockET)
}

// ------------------------------------------------------------------- loop

// resolveStockSymbols keeps the configured pairs that are currently listed
// bStock pairs; unknown ones are dropped with a warning. Scan mode ignores the
// configured list entirely. In hybrid mode an empty result is fine: the scanner
// still supplies candidates.
func (at *AutoTrader) resolveStockSymbols(ctx context.Context, sc *store.StockConfig) ([]string, error) {
	if sc.EffectiveSymbolSource() == store.StockSymbolSourceScan {
		return nil, nil
	}
	var ok []string
	for _, sym := range sc.Symbols {
		if _, found := stockLookupSymbol(ctx, sym); found {
			ok = append(ok, sym)
		} else {
			logger.Warnf("⚠️ [%s] us_stock symbol %s is not a listed bStock pair — dropped", at.name, sym)
		}
	}
	if len(ok) == 0 && !sc.IsScanMode() {
		return nil, fmt.Errorf("none of the configured us_stock symbols is a listed bStock pair")
	}
	return ok, nil
}

// runStockLoop is the whole run of a us_stock trader (called from run() in
// place of the futures monitors and the scan-interval loop).
func (at *AutoTrader) runStockLoop() error {
	if onStockLoopStart != nil {
		onStockLoopStart(at)
	}
	at.isRunningMutex.RLock()
	stop := at.stopMonitorCh
	at.isRunningMutex.RUnlock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-stop:
			cancel()
		case <-ctx.Done():
		}
	}()

	sc := at.config.StrategyConfig.StockConfig
	paper := sc.IsPaper()
	cfg, err := at.prepareStockRuntime(ctx)
	if err != nil {
		return err
	}
	symbols := cfg.Symbols
	preset := stockengine.ResolvePreset(cfg)
	logger.Infof("🇺🇸 [%s] us_stock started: %d symbols (source=%s), preset %s, decision times %v ET, paper=%v",
		at.name, len(symbols), cfg.EffectiveSymbolSource(), preset.Name, preset.DecisionTimes, paper)

	// First cycle immediately only when a session is open (pre/regular/after).
	now := at.stockClock()
	if usstock.SessionAt(now) != usstock.SessionClosed {
		at.stock.lastSlot = now
		at.runStockCycleSafe(ctx)
	}
	next := nextStockDecisionTime(maxTime(at.stockClock(), at.stock.lastSlot), preset.DecisionTimes)
	logStockNext(at.name, next)

	timer := time.NewTimer(stockWait(next, at.stockClock()))
	defer timer.Stop()
	ticker := time.NewTicker(stockProtectInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			logger.Infof("[%s] ⏹ us_stock loop stopped", at.name)
			return nil
		case <-ticker.C:
			at.runStockTickSafe(ctx)
		case <-timer.C:
			if !next.IsZero() && !at.stockClock().Before(next) {
				at.stock.lastSlot = next
				at.runStockCycleSafe(ctx)
			}
			next = nextStockDecisionTime(maxTime(at.stockClock(), at.stock.lastSlot), preset.DecisionTimes)
			logStockNext(at.name, next)
			timer.Reset(stockWait(next, at.stockClock()))
		}
	}
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func stockWait(next, now time.Time) time.Duration {
	if next.IsZero() {
		return 6 * time.Hour
	}
	d := next.Sub(now)
	if d < time.Second {
		d = time.Second
	}
	if d > 6*time.Hour {
		d = 6 * time.Hour // re-evaluate across long weekends / clock changes
	}
	return d
}

func logStockNext(name string, next time.Time) {
	if next.IsZero() {
		logger.Warnf("⚠️ [%s] us_stock: no decision slot found in the next 3 weeks (calendar ends?)", name)
		return
	}
	logger.Infof("🕒 [%s] us_stock next decision: %s", name, next.In(stockET).Format("2006-01-02 15:04 MST"))
}

func (at *AutoTrader) runStockCycleSafe(ctx context.Context) {
	defer func() {
		if p := recover(); p != nil {
			logger.Errorf("🚨 [%s] us_stock cycle panic: %v", at.name, p)
		}
	}()
	if err := at.runStockCycle(ctx); err != nil {
		logger.Infof("❌ [%s] us_stock cycle failed: %v", at.name, err)
	}
}

func (at *AutoTrader) runStockTickSafe(ctx context.Context) {
	defer func() {
		if p := recover(); p != nil {
			logger.Errorf("🚨 [%s] us_stock protection tick panic: %v", at.name, p)
		}
	}()
	if at.stock.cfg.IsScanMode() {
		stockWarmScan(stockScanWarmAge) // non-blocking: keeps the ranking fresh ahead of decision times
	}
	at.stockProtectionTick(ctx)
}

// cleanupStockOnStop handles the program's resting limit entries when the
// trader stops. A plain Stop (user stop / delete) cancels them: an entry that
// fills while nobody runs would sit unprotected. StopForRestart keeps them —
// the trader comes straight back, restores the rows and reconciles them.
// Called from Stop() under the execution mutex.
func (at *AutoTrader) cleanupStockOnStop() {
	if !at.IsStockStrategy() || at.stockPaper() || at.store == nil {
		return
	}
	rows, err := at.store.StockPaper().ListPending(at.id)
	if err != nil || len(rows) == 0 {
		return
	}
	var syms []string
	for _, r := range rows {
		syms = append(syms, fmt.Sprintf("%s(order %s)", r.Symbol, r.OrderID))
	}
	if at.keepRestingOnStop.Load() {
		logger.Infof("📌 [%s] us_stock stopping for restart — kept resting limit entries: %s", at.name, strings.Join(syms, ", "))
		return
	}
	// Stopping the strategy must not leave unmanaged entries that could fill
	// with nobody placing a stop: cancel the program's own resting entries.
	var failed []string
	for _, r := range rows {
		if at.stockTrader == nil {
			failed = append(failed, fmt.Sprintf("%s(order %s)", r.Symbol, r.OrderID))
			continue
		}
		if err := at.stockTrader.CancelOrder(r.Symbol, r.OrderID); err != nil {
			failed = append(failed, fmt.Sprintf("%s(order %s): %v", r.Symbol, r.OrderID, err))
		}
	}
	msg := strings.Join(syms, ", ")
	if len(failed) == 0 {
		logger.Infof("📌 [%s] us_stock stopped — cancelled resting limit entries: %s", at.name, msg)
		return
	}
	fmsg := strings.Join(failed, ", ")
	logger.Warnf("⚠️ [%s] us_stock stopped; could not cancel resting limit entries: %s — cancel manually", at.name, fmsg)
	stockNotify("ALERT", at.name, fmt.Sprintf("<b>⚠️ 美股策略已停止，限价入场单撤销失败</b>\n<i>%s — 请手动撤单，成交后需人工保护</i>", notify.Escape(fmsg)))
}

// ---------------------------------------------------------- number helpers

func stockRound(v float64) float64 { return math.Round(v*1e10) / 1e10 }

// floorStep rounds v down to a multiple of step (tolerating float noise).
func floorStep(v, step float64) float64 {
	if step <= 0 {
		return v
	}
	return stockRound(math.Floor(v/step+1e-9) * step)
}

func nearestStep(v, step float64) float64 {
	if step <= 0 {
		return v
	}
	return stockRound(math.Round(v/step) * step)
}

func stockSymbolInfo(ctx context.Context, symbol string) usstock.SymbolInfo {
	info, _ := stockLookupSymbol(ctx, symbol)
	return info
}
