package stockengine

import (
	"fmt"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"

	"nofx/market/usstock"
	"nofx/store"
)

const decisionContract = `[{"symbol":"AAPLBUSDT","action":"open_long","entry_type":"market","limit_price":0,"stop_loss":95,"take_profit":110,"reduce_fraction":0,"confidence":80,"reasoning":"short summary"}]`

// BuildSystemPrompt describes the style and the program-enforced output rules.
func BuildSystemPrompt(cfg *store.StockConfig, preset Preset, lang string) string {
	if preset.Name == "" {
		preset = ResolvePreset(cfg)
	}
	position := preset.Name == store.StockPresetPosition
	horizon, trail := "days to weeks; review after 30 trading days", "daily EMA20 or 2×ATR(1d)"
	zhHorizon, zhTrail := "数天至数周；30 个交易日后复核", "日线 EMA20 或 2×ATR(1d)"
	if position {
		horizon, trail = "weeks to months; no fixed maximum holding days", "daily EMA50 or 3×ATR(1d)"
		zhHorizon, zhTrail = "数周至数月；无固定最长持仓天数", "日线 EMA50 或 3×ATR(1d)"
	}
	divergence := fmt.Sprintf("|divergence| ≤ %.2f%%", effectiveMaxDivergencePct(cfg))
	zhDivergence := fmt.Sprintf("|偏离| ≤ %.2f%%", effectiveMaxDivergencePct(cfg))
	if effectiveMaxDivergencePct(cfg) < 0 {
		divergence = "divergence limit disabled"
		zhDivergence = "偏离阈值已禁用"
	}
	sessions, zhSessions := sessionsText(cfg)
	var enScan, zhScan string
	if cfg.IsScanMode() {
		if cfg.EffectiveSymbolSource() == store.StockSymbolSourceHybrid {
			enScan = "Scan mode: the candidate list is the user's manual picks plus the cycle scanner's picks and held names, and changes between cycles; a name leaving the list is not a bearish signal by itself.\n"
			zhScan = "扫描模式：候选标的由用户手动指定的标的、周期扫描器挑选的标的和持仓标的组成，每个周期都会变化；标的不再出现在列表中本身不是看空信号。\n"
		} else {
			enScan = "Scan mode: the candidate list is the cycle scanner's picks plus held names, and changes between cycles; a name leaving the list is not a bearish signal by itself.\n"
			zhScan = "扫描模式：候选标的由周期扫描器挑选的标的和持仓标的组成，每个周期都会变化；标的不再出现在列表中本身不是看空信号。\n"
		}
	}
	enNever, zhNever := "never open at exhaustion_extension, wedge_drop or downtrend", "exhaustion_extension、wedge_drop、downtrend 阶段绝不开仓"
	if cfg.IsScanMode() && cfg.EffectiveScanRank() == store.StockScanRankMomentum {
		enNever, zhNever = "never open at wedge_drop or downtrend; at exhaustion_extension do not chase", "wedge_drop、downtrend 阶段绝不开仓，exhaustion_extension 阶段不追高"
		enScan += "Scan ranking: candidates were ranked by 12-1 momentum among names above their 200-day SMA (a statistically validated ranking). The Kell cycle phase is entry-timing context, not a filter: prefer entries on pullbacks toward EMA10/EMA20 (ema_crossback, base_forming/base_n_break, uptrend pullbacks); at exhaustion_extension do not chase — prefer a limit entry on a pullback or wait; wedge_drop / downtrend → wait.\n"
		zhScan += "扫描排序：候选标的按「站上 200 日均线的标的中 12-1 动量从高到低」排序（经统计验证的排序）。Kell 周期阶段是入场时机的参考而非过滤条件：优先在回踩 EMA10/EMA20 时入场（ema_crossback、base_forming/base_n_break、上升趋势回踩）；exhaustion_extension 不追高，宁可挂限价等回踩或等待；wedge_drop / downtrend 则等待。\n"
	}
	if strings.EqualFold(lang, "zh") || strings.HasPrefix(strings.ToLower(lang), "zh-") {
		return fmt.Sprintf(`你是美股中长线交易员，在 Binance bStock 现货交易，只做多、无杠杆。预设 %s，持仓周期：%s。趋势周期 %s，入场周期 %s；每个交易日美东时间 %s 决策，避免追逐日内噪声。
程序强制规则：新开仓/加仓允许时段 %s，休市禁止新增敞口；平仓任何时段允许。必需周期数据充分（1d 至少 200 根，EMA200 不足也阻止入场），入场参考报价必须新鲜，%s。止损必须 >0 且低于入场价，距离在 [%.2f, %.2f]×ATR(1d)；限价 >0 且不高于现价 1.01 倍；止盈如设置必须高于入场价。单票上限 %.2f%%，总敞口上限 %.2f%%，最多 %d 只，单笔风险 %.2f%% 权益。程序计算数量和最小名义金额（5 USDT），模型绝不输出 quantity。加仓必须盈利且现价高于初始止损；减仓比例在 (0,1)；止损只能上移收紧。
入场方法（只在趋势周期向上或刚转强时做多，其余情况 wait）：①顺势回踩——趋势周期多头排列，价格回落到上升的 EMA20/EMA50 或前高转支撑附近，入场周期重新转强；②带量突破——日线收盘站上近期整理区上沿，量比明显放大（>1.5），避免在突破当日远离突破位追高；③不在 52 周新低附近、下降趋势中或单日大涨远离均线（>2×ATR）时开仓。同等条件下优先 ETF 与流动性好的大盘股。
价格周期法（Oliver Kell，日线）：按周期阶段判断位置——优先低风险做多入场：wedge_pop（收敛后首次站回 EMA10/20，止损放在突破 K 线/枢轴低点下方）、ema_crossback（首次回踩 EMA10/20 并守住，止损放在 EMA20/回踩低点下方）、base_n_break（紧密整理平台带量突破，止损放在平台下方）；reversal_extension 只是早期信号，宁可等收敛确认；base_forming 仅观察；%s。持仓遇 exhaustion_extension 考虑 reduce_long 或上移止损，遇 wedge_drop/downtrend 平仓或收紧止损。周期早期要耐心持有，周期后段更快兑现利润；价格领先、成交量确认；相对 SPY 的强弱用于同条件下择优。扫描分数只是候选过滤而非信号，上述所有程序规则依然适用，随时可以 wait。
%s大盘背景：SPY/QQQ 日线转弱（跌破 EMA50 或趋势 down）时只保留最强标的，新开仓要求更严；大盘下行趋势中不加仓。临近财报、重大事件时信息不足，宁可等待。
退出纪律：结构止损放在日线确认摆动低点下方并满足 ATR 带，否则等待；盈利达到 1R 后移动至保本，按 %s 跟踪。不得在初始止损以下摊低成本。趋势周期转为 down 或跌破关键支撑时主动减仓或平仓，不要等止损。
输出契约 OUTPUT CONTRACT：先给简短推理摘要，再给一个 JSON 数组；字段只能使用 symbol, action, entry_type, limit_price, stop_loss, take_profit, reduce_fraction, confidence, reasoning，价格用 bStock USDT。每个配置标的恰好一个决策。已持仓只用 hold/add_long/reduce_long/close_long/adjust_stop；未持仓只用 open_long/wait。entry_type 为 market 或 limit；confidence 为 0–100。示例：
%s`, preset.Name, zhHorizon, preset.TrendTF, preset.EntryTF, strings.Join(preset.DecisionTimes, ","), zhSessions, zhDivergence, preset.StopATRMin, preset.StopATRMax, effectiveMaxPositionPct(cfg), effectiveMaxTotalExposurePct(cfg), effectiveMaxPositions(cfg), effectiveRiskPerTradePct(cfg), zhNever, zhScan, zhTrail, decisionContract)
	}
	return fmt.Sprintf(`You are a US-equity mid/long-term trader on Binance bStock spot: long-only, no leverage. Preset %s; horizon: %s. Trend timeframe %s, entry timeframe %s; decide at %s ET on trading days. Avoid chasing intraday noise.
PROGRAM-ENFORCED rules: new exposure (open/add) allowed sessions %s; closed sessions prohibit new exposure; closes are allowed in every session. Required timeframes must be sufficient (1d needs 200 bars; missing EMA200 blocks entries). A fresh reference quote is required; %s. Stop >0 and below entry, distance within [%.2f, %.2f]×ATR(1d); limit >0 and ≤ market price×1.01; take-profit, if set, must exceed entry. Position cap %.2f%%, total exposure cap %.2f%%, maximum %d positions, risk per trade %.2f%% of equity. The program sizes orders and enforces 5 USDT minimum notional; the model never outputs quantity. Adds require profit and price above InitialStop; reduce_fraction is in (0,1); stops may only tighten upward.
Entry methods (go long only when the trend timeframe is up or just turning up; otherwise wait): (1) trend pullback — trend timeframe aligned up, price pulls back to a rising EMA20/EMA50 or former resistance turned support, entry timeframe turning up again; (2) volume breakout — a daily close above a recent base with clearly expanded volume (ratio > 1.5), without chasing far above the breakout level; (3) no entries near 52-week lows, in downtrends, or after a one-day spike far above the averages (> 2×ATR). Prefer ETFs and liquid large caps when setups are equal.
Price-cycle method (Oliver Kell, daily bars): read the cycle phase of each name. Preferred low-risk long entries: wedge_pop (first reclaim of EMA10/20 after tightening; stop below the pivot/pop-bar low), ema_crossback (first pullback to EMA10/20 that holds; stop below EMA20 / the pullback low) and base_n_break (tight base breakout on volume; stop below the base). reversal_extension is only an early signal — prefer waiting for tightening; base_forming = watch only; %s. For holdings: exhaustion_extension → consider reduce_long or tighten the stop; wedge_drop or downtrend → close or tighten. Be patient early in the cycle, take profits faster late in it; price leads, volume confirms; relative strength vs SPY breaks ties. The scanner score is a candidate filter, not a signal: every program rule above still applies and wait is always acceptable.
%sMarket context: when SPY/QQQ daily trend weakens (below EMA50 or trend down), keep only the strongest names and raise the bar for new entries; never add during a market downtrend. Around earnings or major events information is thin — prefer waiting.
Exit discipline: structural stop below the confirmed daily swing low, within the ATR band, otherwise wait. Move to breakeven after 1R; trail by %s. No averaging down below the initial stop. When the trend timeframe turns down or key support breaks, reduce or close proactively instead of waiting for the stop.
OUTPUT CONTRACT: a short reasoning summary followed by a JSON array. Use exactly these JSON field names: symbol, action, entry_type, limit_price, stop_loss, take_profit, reduce_fraction, confidence, reasoning. Prices are bStock USDT units. Every configured symbol gets exactly one decision. Held symbols use hold/add_long/reduce_long/close_long/adjust_stop; non-held symbols use open_long/wait. entry_type is market or limit; confidence is 0–100. Example:
%s`, preset.Name, horizon, preset.TrendTF, preset.EntryTF, strings.Join(preset.DecisionTimes, ","), sessions, divergence, preset.StopATRMin, preset.StopATRMax, effectiveMaxPositionPct(cfg), effectiveMaxTotalExposurePct(cfg), effectiveMaxPositions(cfg), effectiveRiskPerTradePct(cfg), enNever, enScan, trail, decisionContract)
}

// sessionsText renders the sessions in which new exposure is allowed.
func sessionsText(cfg *store.StockConfig) (en, zh string) {
	var e, z []string
	if allowPreMarket(cfg) {
		e, z = append(e, "pre-market 04:00–09:30 ET"), append(z, "盘前（美东 04:00–09:30）")
	}
	if allowRegular(cfg) {
		e, z = append(e, "regular 09:30–16:00 ET"), append(z, "常规时段（美东 09:30–16:00）")
	}
	if allowAfterHours(cfg) {
		e, z = append(e, "after-hours 16:00–20:00 ET"), append(z, "盘后（美东 16:00–20:00）")
	}
	if len(e) == 0 {
		return "none", "无"
	}
	return strings.Join(e, ", "), strings.Join(z, "、")
}

// BuildUserPrompt presents injected ET time, account, positions and compact data.
func BuildUserPrompt(ctx *Context, lang string) string {
	if ctx == nil {
		return "No context / 无上下文"
	}
	zh := strings.HasPrefix(strings.ToLower(lang), "zh")
	label := func(en, cn string) string {
		if zh {
			return cn
		}
		return en
	}
	preset := contextPreset(ctx)
	required := map[string]bool{usstock.TF1d: true, preset.TrendTF: true, preset.EntryTF: true}
	et, _ := time.LoadLocation("America/New_York")
	var b strings.Builder
	fmt.Fprintf(&b, "%s ET | session=%s | %s=%t\n", ctx.Now.In(et).Format("2006-01-02 15:04:05 MST"), ctx.Session, label("new exposure allowed", "允许新增敞口"), sessionAllowed(ctx.Config, ctx.Session))
	exposurePct := 0.0
	if ctx.Account.Equity > 0 {
		exposurePct = ctx.Account.Exposure / ctx.Account.Equity * 100
	}
	fmt.Fprintf(&b, "%s: equity=%.2f USDT available=%.2f exposure=%.2f (%.2f%%)\n", label("Account", "账户"), ctx.Account.Equity, ctx.Account.Available, ctx.Account.Exposure, exposurePct)
	fmt.Fprintf(&b, "%s:\n", label("Positions", "持仓"))
	if len(ctx.Positions) == 0 {
		fmt.Fprintln(&b, label("none", "无"))
	}
	for _, p := range ctx.Positions {
		price := p.Price
		if s := snapshotFor(ctx, p.Symbol); s != nil && positive(s.Price) {
			price = s.Price
		}
		pnl := 0.0
		if p.AvgPrice > 0 {
			pnl = (price/p.AvgPrice - 1) * 100
		}
		r := "n/a"
		if p.InitialStop > 0 && p.AvgPrice > p.InitialStop {
			r = fmt.Sprintf("%.2fR", (price-p.AvgPrice)/(p.AvgPrice-p.InitialStop))
		}
		days := "n/a"
		if !p.OpenedAt.IsZero() {
			d := ctx.Now.Sub(p.OpenedAt).Hours() / 24
			if d < 0 {
				d = 0
			}
			days = fmt.Sprintf("%.1f", d)
		}
		fmt.Fprintf(&b, "%s qty=%.6f avg=%.2f price=%.2f unrealized=%.2f%% R=%s held_days=%s (calendar) initial_stop=%.2f live_stop=%.2f TP=%.2f\n", p.Symbol, p.Quantity, p.AvgPrice, price, pnl, r, days, p.InitialStop, p.StopPrice, p.TakeProfit)
	}
	fmt.Fprintf(&b, "%s:\n", label("Market context SPY/QQQ (reference only — not candidates, output no decisions for them)", "大盘背景 SPY/QQQ（仅供参考，不是候选标的，不要对其输出决策）"))
	for _, s := range ctx.Market {
		writeSnapshot(&b, s, zh, required)
	}
	fmt.Fprintf(&b, "%s:\n", label("Configured symbols", "配置标的"))
	if ctx.Config != nil {
		for _, symbol := range ctx.Config.Symbols {
			s := snapshotFor(ctx, symbol)
			if s == nil {
				fmt.Fprintf(&b, "%s: DATA_INSUFFICIENT / MissingTF 1d and required timeframes\n", symbol)
				continue
			}
			writeSnapshot(&b, s, zh, required)
		}
	}
	writeUniverse(&b, ctx, zh)
	return b.String()
}

func writeSnapshot(b *strings.Builder, s *SymbolSnapshot, zh bool, required map[string]bool) {
	if s == nil {
		return
	}
	fmt.Fprintf(b, "%s (%s) price=%.2f trend[d/w/entry]=%s/%s/%s EMA20/50/200=%.2f/%.2f/%.2f ATR1d=%.2f (%.2f%%) 52w[low/high]=%.2f/%.2f from_high=%.2f%% vol20/50=%.2f ret20d=%.2f%% swing[low/high]=%.2f/%.2f\n", s.Symbol, s.Underlying, s.Price, s.TrendDaily, s.TrendWeekly, s.TrendEntry, s.EMA20, s.EMA50, s.EMA200, s.ATR1d, s.ATR1dPct, s.Low52w, s.High52w, s.PctFrom52wHigh, s.VolumeRatio20_50, s.Return20dPct, s.SwingLow, s.SwingHigh)
	if c := s.Cycle; c != nil {
		writeCycleLine(b, c, zh)
	}
	if m := s.Momentum; m != nil && m.Valid {
		fmt.Fprintf(b, "mom12_1=%+.1f%% above_sma200=%t\n", m.Mom12_1Pct, m.AboveSMA200)
	}
	fmt.Fprint(b, "sources: ")
	for _, tf := range []string{usstock.TF15m, usstock.TF1h, usstock.TF4h, usstock.TF1d, usstock.TF1w} {
		source, ok := s.Sources[tf]
		if !ok {
			continue
		}
		if source == usstock.SourceYahoo {
			if zh {
				source = "Yahoo 正股 " + s.Underlying
			} else {
				source = "Yahoo underlying " + s.Underlying
			}
		} else if source == usstock.SourceBStock {
			source = "bStock"
		}
		fmt.Fprintf(b, "%s: %s; ", tf, source)
	}
	fmt.Fprintln(b)
	// Only timeframes the preset actually needs are worth a warning: optional
	// timeframes the run cycle never fetched (e.g. 15m/4h for swing) must not
	// read as a data problem and talk the model out of a valid entry.
	var missing []string
	for _, tf := range s.MissingTF {
		if required[tf] {
			missing = append(missing, tf)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(b, "MissingTF WARNING: %s (required TF missing blocks entry)", strings.Join(missing, ","))
		for _, tf := range missing {
			if tf == usstock.TF1d {
				fmt.Fprint(b, "; 1d<200: EMA200 unavailable; EMA20/50 only when available")
				break
			}
		}
		fmt.Fprintln(b)
	}
	if q := s.Quote; q != nil {
		fmt.Fprintf(b, "quote: bStock=%.2f ref=%.2f divergence=%.2f%% fresh=%t ref_time=%s session=%s\n", q.BStockPrice, q.RefPrice, q.DivergencePct, q.RefFresh, q.RefTime.Format(time.RFC3339), q.Session)
	} else {
		fmt.Fprintln(b, "quote: missing / BSTOCK_REF_STALE")
	}
	writeCandles(b, s, zh)
}

// candleOrder is the print order of the raw-candle blocks.
var candleOrder = []string{usstock.TF1w, usstock.TF1d, usstock.TF4h}

// writeCandles prints RSI-14 per timeframe and the raw OHLCV candles the
// snapshot carries, oldest first. 1w/1d bars are dated by their UTC open date,
// 4h bars by their ET open time. Timeframes without data are skipped.
func writeCandles(b *strings.Builder, s *SymbolSnapshot, zh bool) {
	var rsis []string
	for _, tf := range []string{usstock.TF4h, usstock.TF1d, usstock.TF1w} {
		if v, ok := s.RSI[tf]; ok {
			rsis = append(rsis, fmt.Sprintf("%s=%.1f", tf, v))
		}
	}
	if len(rsis) > 0 {
		fmt.Fprintf(b, "rsi14: %s\n", strings.Join(rsis, " "))
	}
	et, _ := time.LoadLocation("America/New_York")
	for _, tf := range candleOrder {
		bars := s.Candles[tf]
		if len(bars) == 0 {
			continue
		}
		layout, extra := "2006-01-02", ""
		if tf == usstock.TF4h {
			layout = "2006-01-02 15:04"
			if zh {
				extra = "；美东开盘时间"
			} else {
				extra = "; ET bar start"
			}
		}
		if zh {
			fmt.Fprintf(b, "K线 %s (%d%s；旧→新；date,o,h,l,c,v):\n", tf, len(bars), extra)
		} else {
			fmt.Fprintf(b, "candles %s (%d%s, oldest→newest; date,o,h,l,c,v):\n", tf, len(bars), extra)
		}
		for _, bar := range bars {
			t := bar.OpenTime.UTC()
			if tf == usstock.TF4h {
				t = bar.OpenTime.In(et)
			}
			fmt.Fprintf(b, "%s,%s,%s,%s,%s,%s\n", t.Format(layout), fmtPx(bar.Open), fmtPx(bar.High), fmtPx(bar.Low), fmtPx(bar.Close), fmtVol(bar.Volume))
		}
	}
}

// writeUniverse prints the one-row-per-pair universe table of the fresh scan
// report (ctx.Universe), sorted by symbol. Candidates (this cycle's list) are
// marked Y and scan-excluded pairs x. Omitted when there are no rows.
func writeUniverse(b *strings.Builder, ctx *Context, zh bool) {
	if ctx == nil || len(ctx.Universe) == 0 {
		return
	}
	candidates, excluded := map[string]bool{}, map[string]bool{}
	if ctx.Config != nil {
		for _, sym := range ctx.Config.Symbols {
			candidates[sym] = true
		}
		for _, sym := range ctx.Config.ScanExclude {
			excluded[sym] = true
		}
	}
	rows := make([]ScanResult, 0, len(ctx.Universe))
	for _, r := range ctx.Universe {
		if r.Universe.Price > 0 {
			rows = append(rows, r)
		}
	}
	if len(rows) == 0 {
		return
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Symbol < rows[j].Symbol })
	if zh {
		fmt.Fprintf(b, "全市场概览（所有 bStock；仅供参考——只能对上面的候选标的开仓/加仓）:\n")
		fmt.Fprintln(b, "列：symbol|class|sector|candidate(Y=本周期候选, x=已排除)|price|r1d%|r10d%|r30d%|vol30%(年化)|rsi14(日线)|from30dHigh%|adv20$(20日均成交额)|earn_days(距财报天数,空=未知)|mom12_1%|>sma200(Y/N)|phase(Kell)")
	} else {
		fmt.Fprintf(b, "Universe (all listed bStock pairs; reference only — open/add only symbols in the candidate list above):\n")
		fmt.Fprintln(b, "columns: symbol|class|sector|candidate(Y=in this cycle's list, x=scan-excluded)|price|r1d%|r10d%|r30d%|vol30%(annualized)|rsi14(daily)|from30dHigh%|adv20$(20d avg dollar volume)|earn_days(to next earnings, blank=unknown)|mom12_1%|>sma200(Y/N)|phase(Kell)")
	}
	for _, r := range rows {
		u := r.Universe
		mark := ""
		if candidates[r.Symbol] {
			mark = "Y"
		} else if excluded[r.Symbol] {
			mark = "x"
		}
		earn := ""
		if u.DaysToEarnings >= 0 {
			earn = fmt.Sprint(u.DaysToEarnings)
		}
		mom, above := "", ""
		if r.Momentum.Valid {
			mom = fmt.Sprintf("%.1f", r.Momentum.Mom12_1Pct)
			above = "N"
			if r.Momentum.AboveSMA200 {
				above = "Y"
			}
		}
		fmt.Fprintf(b, "%s|%s|%s|%s|%s|%.1f|%.1f|%.1f|%.1f|%.1f|%.1f|$%s|%s|%s|%s|%s\n", r.Symbol, u.Class, strings.ReplaceAll(u.Sector, "|", "/"), mark,
			fmtPx(u.Price), u.Ret1dPct, u.Ret10dPct, u.Ret30dPct, u.Vol30Pct, u.RSI14, u.DistFrom30dHighPct, fmtVol(u.AvgDollarVol20), earn, mom, above, r.Cycle.Phase)
	}
}

// writeCycleLine prints the Kell price-cycle assessment of one snapshot.
func writeCycleLine(b *strings.Builder, c *CycleAssessment, zh bool) {
	head := "cycle(Kell, daily)"
	if zh {
		head = "价格周期(Kell, 日线)"
	}
	fmt.Fprintf(b, "%s: phase=%s long_setup=%t score=%.1f ext10=%+.1fATR dist20=%+.1fATR rs63_vs_SPY=%+.1f%% vol=%.2fx", head, c.Phase, c.LongSetup, c.Score, c.Ext10ATR, c.Dist20ATR, c.RS63Pct, c.VolRatio)
	if len(c.Notes) > 0 {
		fmt.Fprintf(b, " %s", strings.Join(c.Notes, "; "))
	}
	fmt.Fprintln(b)
}
