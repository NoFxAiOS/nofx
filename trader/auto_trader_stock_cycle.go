package trader

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"nofx/kernel/stockengine"
	"nofx/logger"
	"nofx/market/usstock"
	"nofx/store"
	"nofx/trader/types"

	notify "nofx/telegram/notify"
)

// us_stock (design 2026-10-09 §6): one decision cycle.

// prepareStockRuntime resolves the symbol list and initializes run state;
// live mode also runs the executor preflight and restores resting entries.
func (at *AutoTrader) prepareStockRuntime(ctx context.Context) (*store.StockConfig, error) {
	sc := at.config.StrategyConfig.StockConfig
	if at.store == nil {
		return nil, fmt.Errorf("us_stock strategy needs a store (program-owned positions are tracked in it)")
	}
	symbols, err := at.resolveStockSymbols(ctx, sc)
	if err != nil {
		return nil, err
	}
	cfg := *sc
	cfg.Symbols = symbols
	at.stock.cfg = &cfg
	at.stock.stopTicks = map[string]int{}
	at.stock.lastProtect = map[string]stockProtect{}
	at.stock.pendings = map[string]*stockPending{}
	if sc.IsScanMode() {
		stockWarmScan(stockScanWarmAge)
	}
	if !sc.IsPaper() {
		if at.stockTrader == nil {
			return nil, fmt.Errorf("us_stock executor not linked")
		}
		probe := at.stockPreflightSymbol(ctx, symbols)
		if probe == "" {
			return nil, fmt.Errorf("us_stock preflight: no listed bStock pair available")
		}
		if err := at.stockTrader.Preflight(probe); err != nil {
			logger.Errorf("❌ [%s] us_stock preflight failed on %s: %v", at.name, probe, err)
			stockNotify("ALERT", at.name, fmt.Sprintf("<b>🚫 美股策略未启动 (预检失败)</b>\n<i>%s: %s</i>", notify.Escape(probe), notify.Escape(err.Error())))
			// Entries kept across a restart would now sit unmanaged: cancel them.
			at.cancelOrphanStockPendings("preflight failed")
			return nil, fmt.Errorf("us_stock preflight failed: %w", err)
		}
		at.restoreStockPendings()
	} else {
		// A live instance may have kept resting entries for a restart that came
		// back in paper mode; paper never reconciles exchange orders.
		at.cancelOrphanStockPendings("strategy is now paper trading")
	}
	return &cfg, nil
}

// stockOrphanExecutor builds the temporary spot executor cancelOrphanStockPendings
// uses when the instance has none (paper mode). It is a package-level seam so
// tests can inject a fake.
var stockOrphanExecutor = func(at *AutoTrader) (types.SpotStockTrader, error) {
	if newSpotStockTrader == nil {
		return nil, fmt.Errorf("us_stock executor not linked")
	}
	if at.config.BinanceAPIKey == "" || at.config.BinanceSecretKey == "" {
		return nil, fmt.Errorf("no Binance credentials on this trader")
	}
	return newSpotStockTrader(at.config.BinanceAPIKey, at.config.BinanceSecretKey)
}

// cancelOrphanStockPendings cancels and forgets resting live entries that no
// running live instance will manage (kept across a restart that did not come
// back live). A failed cancel alerts the user to cancel by hand.
func (at *AutoTrader) cancelOrphanStockPendings(reason string) {
	if at.store == nil {
		return
	}
	rows, err := at.store.StockPaper().ListPending(at.id)
	if err != nil || len(rows) == 0 {
		return
	}
	// Paper mode never builds at.stockTrader (newStockExecutor returns nil), so
	// kept live entries could only be alerted. Build a temporary executor from
	// the trader's own credentials instead; alert-only if that is impossible.
	exec := at.stockTrader
	var execErr error
	if exec == nil {
		exec, execErr = stockOrphanExecutor(at)
	}
	var done, failed []string
	for _, r := range rows {
		label := fmt.Sprintf("%s(order %s)", r.Symbol, r.OrderID)
		if exec == nil {
			if execErr != nil {
				failed = append(failed, fmt.Sprintf("%s: no executor (%v)", label, execErr))
			} else {
				failed = append(failed, label+": no executor")
			}
			continue
		}
		if err := exec.CancelOrder(r.Symbol, r.OrderID); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", label, err))
			continue
		}
		done = append(done, label)
		if err := at.store.StockPaper().DeletePending(at.id, r.Symbol); err != nil {
			logger.Warnf("⚠️ [%s] cannot delete resting entry row %s: %v", at.name, r.Symbol, err)
		}
	}
	if len(done) > 0 {
		logger.Infof("📌 [%s] cancelled kept resting entries (%s): %s", at.name, reason, strings.Join(done, ", "))
	}
	if len(failed) > 0 {
		msg := strings.Join(failed, ", ")
		logger.Warnf("⚠️ [%s] could not cancel kept resting entries (%s): %s — cancel manually", at.name, reason, msg)
		stockNotify("ALERT", at.name, fmt.Sprintf("<b>⚠️ 美股限价入场单无人管理</b>\n<i>%s：%s — 请手动撤单，成交后需人工保护</i>", notify.Escape(reason), notify.Escape(msg)))
	}
}

// stockPreflightSymbol picks the pair the live preflight probes: the first
// resolved pin, else (scan mode without pins) the first listed market-context
// pair, else the first listed bStock pair. "" when nothing is listed.
func (at *AutoTrader) stockPreflightSymbol(ctx context.Context, pins []string) string {
	if len(pins) > 0 {
		return pins[0]
	}
	for _, s := range stockContextSymbols {
		if _, ok := stockLookupSymbol(ctx, s); ok {
			return s
		}
	}
	if infos, err := stockListUniverse(ctx); err == nil && len(infos) > 0 {
		return infos[0].Symbol
	} else if err != nil {
		logger.Warnf("⚠️ [%s] us_stock preflight: cannot list bStock pairs: %v", at.name, err)
	}
	return ""
}

// stockCycleSymbols builds the symbol list of one decision cycle and the scan
// report that chose it (nil when none is available). Manual mode trades the
// configured symbols. Scan mode trades the held program-owned symbols (the
// validator must know them to accept close/adjust) plus the scanner's top long
// setups minus the exclude list. Hybrid mode also always includes the configured
// symbols, after the held ones and before the scanner's picks.
func (at *AutoTrader) stockCycleSymbols(ctx context.Context, sc *store.StockConfig, state *stockState,
	logf func(string, ...interface{})) ([]string, *stockengine.ScanReport) {
	if !sc.IsScanMode() {
		return sc.Symbols, stockCachedScan()
	}
	report, err := stockGetScanReport(ctx, stockScanMaxAge)
	if err != nil {
		if report == nil {
			logf("scan failed, no report: %v", err)
		} else {
			logf("scan refresh failed (%v) — using the previous report from %s", err, report.GeneratedAt.In(stockET).Format("01-02 15:04 MST"))
		}
	}
	var list []string
	seen := map[string]bool{}
	add := func(sym string) {
		if !seen[sym] {
			seen[sym] = true
			list = append(list, sym)
		}
	}
	held := make([]string, 0, len(state.Holdings))
	for sym := range state.Holdings {
		held = append(held, sym)
	}
	sort.Strings(held)
	for _, sym := range held {
		add(sym)
	}
	if sc.UsesManualSymbols() {
		for _, sym := range sc.Symbols {
			if _, ok := stockLookupSymbol(ctx, sym); ok {
				add(sym)
			}
		}
	}
	nBase := len(list)
	exclude := map[string]bool{}
	for _, sym := range sc.ScanExclude {
		exclude[sym] = true
	}
	for sym := range seen {
		exclude[sym] = true
	}
	var picks []string
	rank := sc.EffectiveScanRank()
	if report != nil {
		want := sc.EffectiveScanTopN()
		ranked := report.TopLong(len(report.Results), exclude)
		if rank == store.StockScanRankMomentum {
			ranked = report.TopMomentum(len(report.Results), exclude)
		}
		for _, r := range ranked {
			if len(picks) == want {
				break
			}
			if _, ok := stockLookupSymbol(ctx, r.Symbol); !ok {
				continue
			}
			add(r.Symbol)
			if rank == store.StockScanRankMomentum {
				picks = append(picks, fmt.Sprintf("%s(mom %+.1f%% %s)", r.Symbol, r.Momentum.Mom12_1Pct, r.Cycle.Phase))
			} else {
				picks = append(picks, fmt.Sprintf("%s(%s %.1f)", r.Symbol, r.Cycle.Phase, r.Cycle.Score))
			}
		}
	}
	if report == nil {
		logf("scan unavailable: %d held/manual symbols this cycle", nBase)
	} else {
		logf("scan top %d rank=%s: %s (report age %s)", sc.EffectiveScanTopN(), rank, strings.Join(picks, " "),
			at.stockClock().Sub(report.GeneratedAt).Round(time.Second))
	}
	return list, report
}

// buildStockSnapshots fetches series (sequentially: Yahoo throttles) and the
// quote of every configured pair plus the SPY/QQQ market context.
func (at *AutoTrader) buildStockSnapshots(ctx context.Context, sc *store.StockConfig, preset stockengine.Preset, now time.Time,
	report *stockengine.ScanReport, logf func(string, ...interface{})) (all, market []*stockengine.SymbolSnapshot, prices map[string]float64) {
	prices = map[string]float64{}
	tfs := []string{usstock.TF1d}
	for _, tf := range []string{preset.TrendTF, preset.EntryTF} {
		dup := false
		for _, e := range tfs {
			dup = dup || e == tf
		}
		if !dup {
			tfs = append(tfs, tf)
		}
	}
	// Raw candles and RSI in the prompt cover the weekly and 4h timeframes for
	// every symbol; they are optional (only preset-required timeframes warn).
	for _, tf := range []string{usstock.TF1w, usstock.TF4h} {
		dup := false
		for _, e := range tfs {
			dup = dup || e == tf
		}
		if !dup {
			tfs = append(tfs, tf)
		}
	}
	configured := map[string]bool{}
	var order []string
	for _, s := range sc.Symbols {
		configured[s] = true
		order = append(order, s)
	}
	var ctxSyms []string
	for _, s := range stockContextSymbols {
		if !configured[s] {
			if _, ok := stockLookupSymbol(ctx, s); ok {
				order = append(order, s)
			}
		}
		if _, ok := stockLookupSymbol(ctx, s); ok {
			ctxSyms = append(ctxSyms, s)
		}
	}
	byName := map[string]*stockengine.SymbolSnapshot{}
	daily := map[string]*usstock.Series{}
	for _, sym := range order {
		if ctx.Err() != nil {
			break
		}
		info, ok := stockLookupSymbol(ctx, sym)
		if !ok {
			logf("symbol %s not found in the bStock registry — no snapshot", sym)
			continue
		}
		series := map[string]*usstock.Series{}
		for _, tf := range tfs {
			var s *usstock.Series
			var err error
			if sc.YahooFallback() {
				// Prefer the underlying's regular-session bars for every timeframe:
				// bStock trades 24/7 and its night/weekend bars are near-empty, which
				// distorts candles, RSI and trends. bStock is the fallback.
				if s, err = stockGetUnderlyingSeries(ctx, sym, tf, stockSeriesNeeds[tf]); s == nil || len(s.Bars) == 0 {
					s, err = stockGetSeries(ctx, sym, tf, stockSeriesNeeds[tf], true)
				}
			} else {
				s, err = stockGetSeries(ctx, sym, tf, stockSeriesNeeds[tf], false)
			}
			if err != nil && !errors.Is(err, usstock.ErrInsufficientBars) {
				logf("series %s %s: %v", sym, tf, err)
			}
			if s != nil && len(s.Bars) > 0 {
				series[tf] = s // a short series is passed through: the snapshot marks MissingTF
			}
		}
		quote, err := stockGetQuote(ctx, sym)
		if err != nil {
			logf("quote %s: %v", sym, err)
			quote = nil
		}
		snap := stockengine.BuildSnapshot(sym, info.Underlying, series, quote, now)
		byName[sym] = snap
		daily[sym] = series[usstock.TF1d]
		if snap.Price > 0 {
			prices[sym] = snap.Price
		}
		all = append(all, snap)
	}
	for _, s := range ctxSyms {
		if snap := byName[s]; snap != nil {
			market = append(market, snap)
		}
	}
	// Kell price-cycle assessment: the scan report's entry when it has the
	// symbol and is fresh, otherwise classified from this cycle's daily series
	// (second pass so the SPY series is available as the relative-strength
	// benchmark). A stale report (manual mode reads whatever is cached; a failed
	// refresh keeps the previous one) must not feed hours-old phases.
	if report != nil && now.Sub(report.GeneratedAt) > stockScanMaxAge {
		report = nil
	}
	var spyBars []usstock.Bar
	if s := daily[stockBenchmarkSymbol]; s != nil {
		spyBars = s.Bars
	}
	for sym, snap := range byName {
		if r, ok := report.Lookup(sym); ok {
			c, m := r.Cycle, r.Momentum
			snap.Cycle, snap.Momentum = &c, &m
		} else if s := daily[sym]; s != nil && len(s.Bars) > 0 {
			c, m := stockengine.ClassifyCycle(s.Bars, spyBars), stockengine.ClassifyMomentum(s.Bars)
			snap.Cycle, snap.Momentum = &c, &m
		}
	}
	return all, market, prices
}

// freshScanResults returns the report's results for the prompt's universe
// table, or nil when there is no report or it is older than stockScanMaxAge
// (the same freshness rule the Kell/momentum lines apply).
func freshScanResults(report *stockengine.ScanReport, now time.Time) []stockengine.ScanResult {
	if report == nil || now.Sub(report.GeneratedAt) > stockScanMaxAge {
		return nil
	}
	return report.Results
}

// stockWeekRisk tracks the ET week's starting equity and latches the weekly
// drawdown circuit (design §6): drawdown above stockWeeklyDrawdownHaltPct stops
// new exposure until the next ET week; exits keep being managed.
func (at *AutoTrader) stockWeekRisk(now time.Time, equity float64) (halted bool, ddPct float64) {
	weekStart := stockWeekStart(now)
	key := weekStart.Format("2006-01-02")
	if at.stock.weekKey != key {
		base := equity
		if at.store != nil {
			if snaps, err := at.store.Equity().GetByTimeRange(at.id, weekStart.UTC(), now.UTC()); err == nil && len(snaps) > 0 && snaps[0].TotalEquity > 0 {
				base = snaps[0].TotalEquity
			}
		}
		at.stock.weekKey, at.stock.weekStartEquity, at.stock.weekHalted = key, base, false
	}
	base := at.stock.weekStartEquity
	if base > 0 {
		ddPct = (base - equity) / base * 100
	}
	if ddPct > stockWeeklyDrawdownHaltPct && !at.stock.weekHalted {
		at.stock.weekHalted = true
		logger.Warnf("🛑 [%s] us_stock weekly drawdown %.2f%% > %.0f%%: no new exposure until next ET week", at.name, ddPct, stockWeeklyDrawdownHaltPct)
		stockNotify("RISK", at.name, fmt.Sprintf("<b>🛑 美股周回撤熔断</b>\n<i>本周回撤 %.2f%% 超过 %.0f%%，本周不再开新仓（仍管理退出）%s</i>",
			ddPct, stockWeeklyDrawdownHaltPct, stockPrefixSuffix(at.stockPaper())))
	}
	return at.stock.weekHalted, ddPct
}

func stockPrefixSuffix(paper bool) string {
	if paper {
		return " [PAPER]"
	}
	return ""
}

func (at *AutoTrader) saveStockEquity(now time.Time, st *stockState) {
	if at.store == nil || st == nil {
		return
	}
	unrealized := 0.0
	for _, h := range st.Holdings {
		unrealized += (h.Mark - h.Avg) * h.Owned
	}
	pct := 0.0
	if st.Equity > 0 {
		pct = st.Exposure / st.Equity * 100
	}
	err := at.store.Equity().Save(&store.EquitySnapshot{TraderID: at.id, Timestamp: now.UTC(), TotalEquity: st.Equity,
		Balance: st.Equity - unrealized, UnrealizedPnL: unrealized, PositionCount: len(st.Holdings), MarginUsedPct: pct})
	if err != nil {
		logger.Infof("⚠️ Failed to save equity snapshot: %v", err)
	}
}

func isEntryAction(a string) bool {
	return a == stockengine.ActionOpenLong || a == stockengine.ActionAddLong
}

// runStockCycle runs one decision cycle: account + data → engine → execution →
// decision record. Holds the execution mutex (paper: its own mutex).
func (at *AutoTrader) runStockCycle(ctx context.Context) error {
	sc := at.stock.cfg
	paper := sc.IsPaper()
	mu := at.stockMutex()
	mu.Lock()
	defer mu.Unlock()
	if !at.stockRunning() {
		return nil
	}
	now := at.stockClock()
	pfx := stockPrefix(paper)
	rec := &store.DecisionRecord{Timestamp: now.UTC(), CandidateCoins: append([]string(nil), sc.Symbols...)}
	logf := func(format string, args ...interface{}) {
		line := pfx + fmt.Sprintf(format, args...)
		rec.ExecutionLog = append(rec.ExecutionLog, line)
		logger.Infof("🇺🇸 [%s] %s", at.name, line)
	}
	fail := func(err error) error {
		rec.Success = false
		rec.ErrorMessage = pfx + err.Error()
		logf("cycle failed: %v", err)
		at.saveStockRecord(rec)
		return err
	}
	preset := stockengine.ResolvePreset(sc)
	session := usstock.SessionAt(now)
	logf("cycle start %s ET session=%s preset=%s", now.In(stockET).Format("2006-01-02 15:04"), session, preset.Name)

	var state *stockState
	var err error
	var quarantined map[string]string
	if !paper {
		quarantined, err = at.reconcileStockPendings(ctx, now, logf)
		if err != nil {
			return fail(fmt.Errorf("pending reconciliation: %w", err))
		}
	}
	if paper {
		state, err = at.stockGatherPaper(nil)
	} else {
		state, err = at.stockGatherLiveWithQuarantine(ctx, now, true, true, quarantined)
	}
	if err != nil {
		return fail(fmt.Errorf("account state: %w", err))
	}
	for _, n := range state.Notes {
		logf("%s", n)
	}
	for _, n := range state.manualNotes() {
		logf("%s", n)
	}

	// Per-cycle config copy: scan mode swaps in this cycle's symbol list so the
	// snapshots, the prompt and the validator agree; at.stock.cfg is not touched.
	cc := sc
	cycleSyms, report := at.stockCycleSymbols(ctx, sc, state, logf)
	if sc.IsScanMode() {
		cp := *sc
		cp.Symbols = cycleSyms
		cc = &cp
		rec.CandidateCoins = append([]string(nil), cycleSyms...)
		if len(cycleSyms) == 0 {
			msg := "scan unavailable, no candidates"
			if report != nil {
				msg = "scan found no long setups, no candidates"
			}
			logf("%s — no AI call this cycle", msg)
			rec.Success = true
			at.saveStockRecord(rec)
			return nil
		}
	}
	snaps, market, prices := at.buildStockSnapshots(ctx, cc, preset, now, report, logf)
	if ctx.Err() != nil {
		return nil
	}
	for sym, p := range prices {
		at.stock.setPrice(sym, p)
	}
	if paper {
		state.repricePaper(prices)
	}
	halted, dd := at.stockWeekRisk(now, state.Equity)
	at.saveStockEquity(now, state)
	if halted {
		logf("weekly drawdown circuit active (%.2f%%): new exposure blocked this ET week", dd)
	}

	// Carry each open position's thesis/invalidation forward into the prompt. A
	// missing row (position opened before theses existed, or a store error) is fine.
	theses, terr := at.store.StockThesis().List(at.id)
	if terr != nil {
		logf("thesis store read failed: %v", terr)
	}
	positions := state.enginePositions(prices)
	for i := range positions {
		if th, ok := theses[positions[i].Symbol]; ok {
			positions[i].Thesis, positions[i].Invalidation = th.Thesis, th.Invalidation
		}
		if h := state.Holdings[positions[i].Symbol]; h != nil && h.Quarantined {
			// Attach a runtime note to this prompt's position block through the
			// existing text field; never persist it as the position's thesis.
			positions[i].Thesis = "PROTECTION UNVERIFIED — only hold/wait allowed: " + h.QuarantineReason + "\n  " + positions[i].Thesis
		}
	}
	ectx := &stockengine.Context{Now: now, Session: session, Config: cc, Preset: preset,
		Account:   stockengine.Account{Equity: state.Equity, Available: state.Available, Exposure: state.Exposure},
		Positions: positions, Snapshots: snaps, Market: market, Paper: paper,
		Universe: freshScanResults(report, now)}
	result, derr := stockengine.Decide(ectx, at.mcpClient, at.stockLang())
	if result != nil {
		rec.SystemPrompt, rec.InputPrompt, rec.RawResponse, rec.AIRequestDurationMs = result.SystemPrompt, result.UserPrompt, result.RawResponse, result.DurationMs
		rec.CoTTrace = stockCoTTrace(result)
		if result.RepairPrompt != "" {
			rec.RawResponse = result.RawResponse + "\n\n--- repair ---\n" + result.RepairRawResponse
			rec.AIRequestDurationMs += result.RepairDurationMs
			accepted := 0
			for _, v := range result.Verdicts {
				if v.Accepted && isEntryAction(v.Decision.Action) && containsString(result.Repaired, v.Decision.Symbol) {
					accepted++
				}
			}
			logf("repair: resubmitted %v → %d accepted", result.Repaired, accepted)
		}
	}
	if derr != nil {
		return fail(derr)
	}
	at.callCount++
	if !at.stockRunning() || ctx.Err() != nil {
		logf("trader stopped before execution — decisions not executed")
		return nil
	}

	snapBy := map[string]*stockengine.SymbolSnapshot{}
	for _, s := range snaps {
		snapBy[s.Symbol] = s
	}
	sizing := *ectx
	acct := ectx.Account
	var rejected []string
	for _, v := range result.Verdicts {
		d := v.Decision
		if !v.Accepted && isStockContextNoop(d, v.Codes) {
			logf("%s %s ignored: market-context reference, not a candidate", d.Action, d.Symbol)
			continue
		}
		act := store.DecisionAction{Action: d.Action, Symbol: d.Symbol, StopLoss: d.StopLoss, TakeProfit: d.TakeProfit,
			Confidence: d.Confidence, Reasoning: d.Reasoning, Leverage: 1, Timestamp: now.UTC()}
		code := ""
		h := state.Holdings[d.Symbol]
		switch {
		case h != nil && h.Quarantined && d.Action != stockengine.ActionHold && d.Action != stockengine.ActionWait:
			code = "PROTECTION_UNVERIFIED"
		case !v.Accepted:
			code = joinCodes(v.Codes)
		case isEntryAction(d.Action) && halted:
			code = "WEEKLY_DRAWDOWN_HALT"
		case isEntryAction(d.Action) && at.stock.pendings[d.Symbol] != nil:
			code = "PENDING_ENTRY"
		}
		if code != "" {
			act.Error = pfx + "rejected: " + code
			logf("%s %s rejected: %s %s", d.Action, d.Symbol, code, v.Note)
			if isEntryAction(d.Action) {
				rejected = append(rejected, d.Symbol+" "+code)
			}
			rec.Decisions = append(rec.Decisions, act)
			continue
		}
		var out *stockOutcome
		var xerr error
		snap := snapBy[d.Symbol]
		switch d.Action {
		case stockengine.ActionHold, stockengine.ActionWait:
			act.Success = true
			logf("%s %s", d.Action, d.Symbol)
			if d.Action == stockengine.ActionHold {
				at.refineStockThesis(d, theses, now, logf)
			}
		case stockengine.ActionOpenLong, stockengine.ActionAddLong:
			sizing.Account = acct
			var reserved float64
			out, reserved, xerr = at.execStockEntry(ctx, &sizing, d, snap, h, paper, now, logf)
			if xerr == nil {
				acct.Available -= reserved
				acct.Exposure += reserved
			}
		case stockengine.ActionReduceLong, stockengine.ActionCloseLong:
			out, xerr = at.execStockExit(ctx, d, snap, h, paper, now)
		case stockengine.ActionAdjustStop:
			out, xerr = at.execStockAdjust(ctx, d, snap, h, paper, now)
		}
		if d.Action == stockengine.ActionHold || d.Action == stockengine.ActionWait {
			rec.Decisions = append(rec.Decisions, act)
			continue
		}
		if xerr != nil {
			act.Error = pfx + xerr.Error()
			logf("%s %s FAILED: %v", d.Action, d.Symbol, xerr)
			if isEntryAction(d.Action) {
				rejected = append(rejected, d.Symbol+" "+xerr.Error())
			}
			rec.Decisions = append(rec.Decisions, act)
			continue
		}
		act.Success, act.Quantity, act.Price = true, out.Qty, out.Price
		at.persistStockThesis(d, theses, now, logf)
		act.OrderID, act.EntryOrderID = orderIDInt(out.OrderID), out.OrderID
		logf("%s %s ok: qty=%.6f price=%.4f stop=%.4f tp=%.4f%s", d.Action, d.Symbol, out.Qty, out.Price, d.StopLoss, d.TakeProfit, noteSuffix(out.Note))
		at.notifyStockOutcome(paper, d, out)
		rec.Decisions = append(rec.Decisions, act)
	}
	if len(rejected) > 0 {
		stockNotify("ORDER", at.name, fmt.Sprintf("<b>%s🇺🇸 美股开仓被拒绝 %d 项</b>\n<i>%s</i>",
			map[bool]string{true: "[PAPER 模拟] ", false: ""}[paper], len(rejected), notify.Escape(strings.Join(rejected, "; "))))
	}
	rec.Success = true
	at.saveStockRecord(rec)
	if !paper {
		at.syncStockJournal()
	}
	return nil
}

// stockCoTTrace renders the model's portfolio reasoning and market context for
// the decision record ("" for a legacy bare-array response).
func stockCoTTrace(r *stockengine.Result) string {
	var parts []string
	if r.Reasoning != "" {
		parts = append(parts, r.Reasoning)
	}
	if mc := r.MarketContext; mc != nil {
		parts = append(parts, fmt.Sprintf("market_context: %s; factors: %s", mc.OverallSentiment, strings.Join(mc.KeyFactors, "; ")))
	}
	return strings.Join(parts, "\n\n")
}

func containsString(list []string, s string) bool {
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}

// persistStockThesis stores the thesis of a successfully executed decision:
// open_long / add_long (including a resting limit entry) upsert it, hold /
// adjust_stop refine it when the decision carries one, close_long deletes it.
// A stop hit in the protection tick leaves a stale row behind; that is harmless
// because it is not attached to any position and the next open overwrites it.
// Store errors are logged and never fail the cycle.
func (at *AutoTrader) persistStockThesis(d stockengine.Decision, theses map[string]store.StockThesis, now time.Time, logf func(string, ...interface{})) {
	switch d.Action {
	case stockengine.ActionOpenLong, stockengine.ActionAddLong:
		if err := at.store.StockThesis().Upsert(at.id, d.Symbol, d.Thesis, d.Invalidation, now); err != nil {
			logf("thesis save %s failed: %v", d.Symbol, err)
		}
	case stockengine.ActionAdjustStop:
		at.refineStockThesis(d, theses, now, logf)
	case stockengine.ActionCloseLong:
		if err := at.store.StockThesis().Delete(at.id, d.Symbol); err != nil {
			logf("thesis delete %s failed: %v", d.Symbol, err)
		}
	}
}

// refineStockThesis lets the model update a held position's thesis from a
// hold / adjust_stop decision; a field left empty keeps the stored value.
func (at *AutoTrader) refineStockThesis(d stockengine.Decision, theses map[string]store.StockThesis, now time.Time, logf func(string, ...interface{})) {
	if d.Thesis == "" && d.Invalidation == "" {
		return
	}
	thesis, invalidation := d.Thesis, d.Invalidation
	if old, ok := theses[d.Symbol]; ok {
		if thesis == "" {
			thesis = old.Thesis
		}
		if invalidation == "" {
			invalidation = old.Invalidation
		}
	}
	if err := at.store.StockThesis().Upsert(at.id, d.Symbol, thesis, invalidation, now); err != nil {
		logf("thesis save %s failed: %v", d.Symbol, err)
	}
}

// isStockContextNoop reports a wait/hold the model gave a market-context pair
// (SPY/QQQ) that is not a candidate this cycle. It is not an error — dropping it
// keeps the record from showing a rejected decision (2026-10-15 UI report).
// Any other action on such a pair stays a visible UNKNOWN_SYMBOL rejection.
func isStockContextNoop(d stockengine.Decision, codes []string) bool {
	if d.Action != stockengine.ActionWait && d.Action != stockengine.ActionHold {
		return false
	}
	if len(codes) != 1 || codes[0] != "UNKNOWN_SYMBOL" {
		return false
	}
	for _, s := range stockContextSymbols {
		if s == d.Symbol {
			return true
		}
	}
	return false
}

func noteSuffix(n string) string {
	if n == "" {
		return ""
	}
	return " (" + n + ")"
}

func (at *AutoTrader) saveStockRecord(rec *store.DecisionRecord) {
	if err := at.saveDecision(rec); err != nil {
		logger.Infof("⚠ [%s] Failed to save decision record: %v", at.name, err)
	}
}

func (at *AutoTrader) notifyStockOutcome(paper bool, d stockengine.Decision, out *stockOutcome) {
	switch d.Action {
	case stockengine.ActionOpenLong, stockengine.ActionAddLong:
		title := "开仓"
		if d.Action == stockengine.ActionAddLong {
			title = "加仓"
		}
		extra := fmt.Sprintf("止损 %.4g 止盈 %.4g", d.StopLoss, d.TakeProfit)
		if out.Pending {
			title, extra = "限价入场挂单", extra+"，挂单中"
		}
		at.stockNotifyTrade(paper, title, d.Symbol, out, extra)
	case stockengine.ActionReduceLong:
		at.stockNotifyTrade(paper, "减仓", d.Symbol, out, "")
	case stockengine.ActionCloseLong:
		at.stockNotifyTrade(paper, "平仓", d.Symbol, out, "")
	}
}

// execStockEntry sizes and places an accepted open/add.
func (at *AutoTrader) execStockEntry(ctx context.Context, sizing *stockengine.Context, d stockengine.Decision, snap *stockengine.SymbolSnapshot,
	h *stockHolding, paper bool, now time.Time, logf func(string, ...interface{})) (*stockOutcome, float64, error) {
	if snap == nil || snap.Price <= 0 {
		return nil, 0, fmt.Errorf("DATA_INSUFFICIENT: no snapshot price")
	}
	size, err := stockengine.SizeOrder(sizing, d)
	if err != nil {
		return nil, 0, fmt.Errorf("sizing: %w", err)
	}
	info := stockSymbolInfo(ctx, d.Symbol)
	entry := snap.Price
	if d.EntryType == stockengine.EntryLimit {
		entry = floorStep(d.LimitPrice, info.TickSize)
	}
	qty := floorStep(size.Quantity, info.StepSize)
	if qty <= 0 || qty < info.MinQty || qty*entry < minNotionalOf(info) {
		return nil, 0, fmt.Errorf("MIN_NOTIONAL after lot rounding (qty %.6f x %.4f)", qty, entry)
	}
	logf("%s %s sized: qty=%.6f notional=%.2f risk=%.2f limited_by=%s", d.Action, d.Symbol, qty, qty*entry, size.RiskUSDT, size.LimitedBy)
	if paper {
		out, err := at.stockOpenPaper(d, qty, snap.Price, h, info, now)
		if err != nil {
			return nil, 0, err
		}
		logf("PAPER fill %s qty=%.6f @ %.4f (no exchange order)", d.Symbol, out.Qty, out.Price)
		return out, out.Qty * out.Price, nil
	}
	out, err := at.stockOpenLive(ctx, d, qty, entry, h, info, now, logf)
	if err != nil {
		return nil, 0, err
	}
	return out, qty * entry, nil
}

// execStockExit handles reduce_long / close_long on the program-owned qty.
func (at *AutoTrader) execStockExit(ctx context.Context, d stockengine.Decision, snap *stockengine.SymbolSnapshot, h *stockHolding,
	paper bool, now time.Time) (*stockOutcome, error) {
	if h == nil || h.Owned <= 0 {
		return nil, fmt.Errorf("NOT_HELD: no program-owned position")
	}
	info := stockSymbolInfo(ctx, d.Symbol)
	price := h.Mark
	if snap != nil && snap.Price > 0 {
		price = snap.Price
	}
	full := d.Action == stockengine.ActionCloseLong
	qty := h.Owned
	reason := "ai_close"
	if !full {
		reason = "ai_reduce"
		qty = floorStep(h.Owned*d.ReduceFraction, info.StepSize)
		if qty <= 0 || h.Owned-qty < math.Max(info.MinQty, info.StepSize) || qty*price < minNotionalOf(info) {
			return nil, fmt.Errorf("REDUCE_TOO_SMALL: qty %.6f of %.6f", qty, h.Owned)
		}
	}
	if paper {
		sellQty := qty
		if full {
			sellQty = 0
		}
		out, err := at.stockSellPaper(d.Symbol, sellQty, price, reason, now)
		if out != nil && full {
			out.Qty = h.Owned
		}
		return out, err
	}
	return at.stockSellLive(ctx, h, qty, full, reason, now)
}

// execStockAdjust moves the stop (and optionally the take-profit) of an owned position.
func (at *AutoTrader) execStockAdjust(ctx context.Context, d stockengine.Decision, snap *stockengine.SymbolSnapshot, h *stockHolding,
	paper bool, now time.Time) (*stockOutcome, error) {
	if h == nil || h.Owned <= 0 {
		return nil, fmt.Errorf("NOT_HELD: no program-owned position")
	}
	info := stockSymbolInfo(ctx, d.Symbol)
	stop := floorStep(d.StopLoss, info.TickSize)
	tp := d.TakeProfit
	if tp <= 0 {
		tp = h.TakeProfit
	}
	price := h.Mark
	if snap != nil && snap.Price > 0 {
		price = snap.Price
	}
	if paper {
		if err := at.store.StockPaper().SetProtection(at.id, d.Symbol, stop, tp, now); err != nil {
			return nil, err
		}
		return &stockOutcome{Qty: h.Owned, Price: stop, Note: "stop moved"}, nil
	}
	if err := at.stockPlaceProtection(ctx, d.Symbol, h.Owned, stop, tp, price); err != nil {
		return nil, err
	}
	return &stockOutcome{Qty: h.Owned, Price: stop, Note: "stop moved"}, nil
}
