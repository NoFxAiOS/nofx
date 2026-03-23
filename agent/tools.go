package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"nofx/mcp"
)

// cachedTools holds the static tool definitions (built once, reused per message).
var cachedTools = buildAgentTools()

// agentTools returns the tools available to the LLM for autonomous action.
func agentTools() []mcp.Tool { return cachedTools }

func buildAgentTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "search_stock",
				Description: "Search for a stock by name, ticker symbol, or keyword. Searches across A-share (沪深), Hong Kong, and US markets. Returns a list of matching stocks with their codes. Use this when the user asks about a stock not in your known list, or when you need to find the exact code for a stock.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"keyword": map[string]any{
							"type":        "string",
							"description": "Search keyword: stock name (e.g. '宁德时代', '腾讯'), ticker (e.g. 'TSLA', 'AAPL'), or stock code (e.g. '300750')",
						},
					},
					"required": []string{"keyword"},
				},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "execute_trade",
				Description: "Execute a trade order (crypto or US stocks). Use this when the user explicitly asks to open/close a position. For stocks (e.g. AAPL, TSLA), use open_long to buy and close_long to sell. This creates a pending trade that requires user confirmation.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"open_long", "open_short", "close_long", "close_short"},
							"description": "Trade action: open_long (做多/buy), open_short (做空/sell), close_long (平多), close_short (平空)",
						},
						"symbol": map[string]any{
							"type":        "string",
							"description": "Trading symbol. For crypto: BTCUSDT, ETHUSDT. For US stocks: AAPL, TSLA, NVDA (no suffix needed).",
						},
						"quantity": map[string]any{
							"type":        "number",
							"description": "Trade quantity/amount. Required for opening positions. Use 0 to close entire position.",
						},
						"leverage": map[string]any{
							"type":        "number",
							"description": "Leverage multiplier (e.g. 5, 10, 20). Optional, defaults to trader's current setting.",
						},
					},
					"required": []string{"action", "symbol", "quantity"},
				},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "get_positions",
				Description: "Get all current open positions across all traders. Returns symbol, side, size, entry price, mark price, and unrealized PnL.",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "get_balance",
				Description: "Get account balance and equity across all traders.",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "get_market_price",
				Description: "Get the current market price for a crypto or stock symbol.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"symbol": map[string]any{
							"type":        "string",
							"description": "Trading symbol, e.g. BTCUSDT for crypto, AAPL for stocks",
						},
					},
					"required": []string{"symbol"},
				},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "get_trade_history",
				Description: "Get recent closed trade history with PnL. Use when user asks about past trades, performance, or trade results. Returns the most recent closed positions.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"limit": map[string]any{
							"type":        "number",
							"description": "Number of recent trades to return (default 10, max 50)",
						},
					},
				},
			},
		},
	}
}

// handleToolCall processes a single tool call from the LLM and returns the result.
func (a *Agent) handleToolCall(ctx context.Context, userID int64, lang string, tc mcp.ToolCall) string {
	switch tc.Function.Name {
	case "search_stock":
		return a.toolSearchStock(tc.Function.Arguments)
	case "execute_trade":
		return a.toolExecuteTrade(ctx, userID, lang, tc.Function.Arguments)
	case "get_positions":
		return a.toolGetPositions()
	case "get_balance":
		return a.toolGetBalance()
	case "get_market_price":
		return a.toolGetMarketPrice(tc.Function.Arguments)
	case "get_trade_history":
		return a.toolGetTradeHistory(tc.Function.Arguments)
	default:
		return fmt.Sprintf(`{"error": "unknown tool: %s"}`, tc.Function.Name)
	}
}

func (a *Agent) toolSearchStock(argsJSON string) string {
	var args struct {
		Keyword string `json:"keyword"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf(`{"error": "invalid arguments: %s"}`, err)
	}

	if args.Keyword == "" {
		return `{"error": "keyword is required"}`
	}

	results, err := searchStock(args.Keyword)
	if err != nil {
		return fmt.Sprintf(`{"error": "search failed: %s"}`, err)
	}

	if len(results) == 0 {
		return fmt.Sprintf(`{"results": [], "message": "no stocks found for '%s'"}`, args.Keyword)
	}

	// Limit to top 10 results
	if len(results) > 10 {
		results = results[:10]
	}

	// Also fetch real-time quotes for the top results (up to 3)
	type enrichedResult struct {
		Name   string      `json:"name"`
		Code   string      `json:"code"`
		Market string      `json:"market"`
		Quote  *StockQuote `json:"quote,omitempty"`
	}

	var enriched []enrichedResult
	for i, r := range results {
		er := enrichedResult{Name: r.Name, Code: r.Code, Market: r.Market}
		if i < 3 {
			q, qErr := fetchStockQuote(r.Code)
			if qErr == nil && q.Price > 0 {
				er.Quote = q
			}
		}
		enriched = append(enriched, er)
	}

	result, _ := json.Marshal(map[string]any{
		"keyword": args.Keyword,
		"count":   len(enriched),
		"results": enriched,
	})
	return string(result)
}

func (a *Agent) toolExecuteTrade(_ context.Context, userID int64, lang, argsJSON string) string {
	var args struct {
		Action   string  `json:"action"`
		Symbol   string  `json:"symbol"`
		Quantity float64 `json:"quantity"`
		Leverage int     `json:"leverage"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf(`{"error": "invalid arguments: %s"}`, err)
	}

	// Normalize symbol
	sym := strings.ToUpper(args.Symbol)
	// Only append USDT for crypto symbols; stock tickers (e.g. AAPL, TSLA) stay as-is
	if !isStockSymbol(sym) && !strings.HasSuffix(sym, "USDT") {
		sym += "USDT"
	}

	// Validate action
	validActions := map[string]bool{
		"open_long": true, "open_short": true,
		"close_long": true, "close_short": true,
	}
	if !validActions[args.Action] {
		return fmt.Sprintf(`{"error": "invalid action: %s"}`, args.Action)
	}

	// For open actions, quantity must be > 0
	if (args.Action == "open_long" || args.Action == "open_short") && args.Quantity <= 0 {
		return `{"error": "quantity must be > 0 for opening positions"}`
	}

	// For stock symbols, check market hours and warn if closed
	var marketWarning string
	if isStockSymbol(sym) && a.traderManager != nil {
		for _, t := range a.traderManager.GetAllTraders() {
			if t.GetExchange() == "alpaca" {
				ut := t.GetUnderlyingTrader()
				if ut == nil {
					continue
				}
				type marketChecker interface {
					IsMarketOpen() (bool, string, error)
				}
				if mc, ok := ut.(marketChecker); ok {
					isOpen, status, err := mc.IsMarketOpen()
					if err == nil && !isOpen {
						marketWarning = fmt.Sprintf("⚠️ US market is currently %s. Order will be queued for next market open.", status)
					}
				}
				break
			}
		}
	}

	// Create pending trade — requires user confirmation
	trade := &TradeAction{
		ID:        fmt.Sprintf("trade_%d", time.Now().UnixNano()),
		Action:    args.Action,
		Symbol:    sym,
		Quantity:  args.Quantity,
		Leverage:  args.Leverage,
		Status:    "pending_confirmation",
		CreatedAt: time.Now().Unix(),
	}

	a.pending.Add(trade)
	a.pending.CleanExpired()

	// Return confirmation info to LLM so it can present it to the user
	resultMap := map[string]any{
		"status":   "pending_confirmation",
		"trade_id": trade.ID,
		"action":   trade.Action,
		"symbol":   trade.Symbol,
		"quantity": trade.Quantity,
		"leverage": trade.Leverage,
		"message":  fmt.Sprintf("Trade created. User must confirm with: 确认 %s (or: confirm %s)", trade.ID, trade.ID),
		"expires":  "5 minutes",
	}
	if marketWarning != "" {
		resultMap["market_warning"] = marketWarning
	}
	result, _ := json.Marshal(resultMap)
	return string(result)
}

func (a *Agent) toolGetPositions() string {
	if a.traderManager == nil {
		return `{"error": "no trader manager configured"}`
	}

	var positions []map[string]any
	for id, t := range a.traderManager.GetAllTraders() {
		pos, err := t.GetPositions()
		if err != nil {
			continue
		}
		for _, p := range pos {
			size := toFloat(p["size"])
			if size == 0 {
				continue
			}
			tid := id
			if len(tid) > 8 {
				tid = tid[:8]
			}
			positions = append(positions, map[string]any{
				"trader":          tid,
				"exchange":        t.GetExchange(),
				"symbol":          p["symbol"],
				"side":            p["side"],
				"size":            size,
				"entry_price":     toFloat(p["entryPrice"]),
				"mark_price":      toFloat(p["markPrice"]),
				"unrealized_pnl":  toFloat(p["unrealizedPnl"]),
				"leverage":        p["leverage"],
			})
		}
	}

	if len(positions) == 0 {
		return `{"positions": [], "message": "no open positions"}`
	}

	result, _ := json.Marshal(map[string]any{"positions": positions})
	return string(result)
}

func (a *Agent) toolGetBalance() string {
	if a.traderManager == nil {
		return `{"error": "no trader manager configured"}`
	}

	var balances []map[string]any
	for id, t := range a.traderManager.GetAllTraders() {
		info, err := t.GetAccountInfo()
		if err != nil {
			continue
		}
		tid := id
		if len(tid) > 8 {
			tid = tid[:8]
		}
		balances = append(balances, map[string]any{
			"trader":       tid,
			"name":         t.GetName(),
			"exchange":     t.GetExchange(),
			"total_equity": toFloat(info["total_equity"]),
			"available":    toFloat(info["available_balance"]),
			"used_margin":  toFloat(info["used_margin"]),
		})
	}

	result, _ := json.Marshal(map[string]any{"balances": balances})
	return string(result)
}

func (a *Agent) toolGetMarketPrice(argsJSON string) string {
	var args struct {
		Symbol string `json:"symbol"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf(`{"error": "invalid arguments: %s"}`, err)
	}

	sym := strings.ToUpper(args.Symbol)
	if !isStockSymbol(sym) && !strings.HasSuffix(sym, "USDT") {
		sym += "USDT"
	}

	if a.traderManager == nil {
		return `{"error": "no trader manager configured"}`
	}

	wantStock := isStockSymbol(sym)
	for _, t := range a.traderManager.GetAllTraders() {
		underlying := t.GetUnderlyingTrader()
		if underlying == nil {
			continue
		}
		// Route to correct exchange type (stock vs crypto)
		isAlpaca := t.GetExchange() == "alpaca"
		if wantStock && !isAlpaca {
			continue
		}
		if !wantStock && isAlpaca {
			continue
		}
		price, err := underlying.GetMarketPrice(sym)
		if err == nil && price > 0 {
			priceResult := map[string]any{
				"symbol": sym,
				"price":  price,
			}
			// For stocks, include market status
			if wantStock && isAlpaca {
				type marketChecker interface {
					IsMarketOpen() (bool, string, error)
				}
				if mc, ok := underlying.(marketChecker); ok {
					isOpen, status, mErr := mc.IsMarketOpen()
					if mErr == nil {
						priceResult["market_open"] = isOpen
						priceResult["market_status"] = status
					}
				}
			}
			result, _ := json.Marshal(priceResult)
			return string(result)
		}
	}

	return fmt.Sprintf(`{"error": "could not get price for %s"}`, sym)
}

func (a *Agent) toolGetTradeHistory(argsJSON string) string {
	if a.store == nil {
		return `{"error": "store not available"}`
	}

	var args struct {
		Limit int `json:"limit"`
	}
	if argsJSON != "" {
		_ = json.Unmarshal([]byte(argsJSON), &args)
	}
	if args.Limit <= 0 {
		args.Limit = 10
	}
	if args.Limit > 50 {
		args.Limit = 50
	}

	if a.traderManager == nil {
		return `{"error": "no trader manager configured"}`
	}

	var trades []map[string]any
	var totalPnL float64
	var wins, losses int

	for id, t := range a.traderManager.GetAllTraders() {
		positions, err := a.store.Position().GetClosedPositions(id, args.Limit)
		if err != nil {
			continue
		}
		tid := id
		if len(tid) > 8 {
			tid = tid[:8]
		}
		for _, pos := range positions {
			pnl := pos.RealizedPnL
			totalPnL += pnl
			if pnl >= 0 {
				wins++
			} else {
				losses++
			}

			entryTime := ""
			if pos.EntryTime > 0 {
				entryTime = time.Unix(pos.EntryTime/1000, 0).Format("2006-01-02 15:04")
			}
			exitTime := ""
			if pos.ExitTime > 0 {
				exitTime = time.Unix(pos.ExitTime/1000, 0).Format("2006-01-02 15:04")
			}

			trades = append(trades, map[string]any{
				"trader":      t.GetName(),
				"trader_id":   tid,
				"symbol":      pos.Symbol,
				"side":        pos.Side,
				"entry_price": pos.EntryPrice,
				"exit_price":  pos.ExitPrice,
				"quantity":    pos.Quantity,
				"leverage":    pos.Leverage,
				"pnl":         pnl,
				"entry_time":  entryTime,
				"exit_time":   exitTime,
			})
		}
	}

	if len(trades) == 0 {
		return `{"trades": [], "message": "no closed trades found"}`
	}

	// Sort trades by exit time (most recent first) for consistent ordering across traders
	sort.Slice(trades, func(i, j int) bool {
		ti, _ := trades[i]["exit_time"].(string)
		tj, _ := trades[j]["exit_time"].(string)
		return ti > tj // reverse chronological
	})

	// Only return up to the limit
	if len(trades) > args.Limit {
		trades = trades[:args.Limit]
	}

	winRate := 0.0
	total := wins + losses
	if total > 0 {
		winRate = float64(wins) / float64(total) * 100
	}

	result, _ := json.Marshal(map[string]any{
		"trades":    trades,
		"summary": map[string]any{
			"total_trades": total,
			"wins":         wins,
			"losses":       losses,
			"win_rate":     fmt.Sprintf("%.1f%%", winRate),
			"total_pnl":    totalPnL,
		},
	})
	return string(result)
}

// knownCryptoSymbols is a set of well-known cryptocurrency base symbols.
// Without this, isStockSymbol("BTC") would incorrectly return true because
// "BTC" is 3 uppercase letters and the suffix check only catches "BTCUSDT"-style pairs.
var knownCryptoSymbols = map[string]bool{
	"BTC": true, "ETH": true, "SOL": true, "BNB": true, "XRP": true,
	"DOGE": true, "ADA": true, "AVAX": true, "DOT": true, "LINK": true,
	"PEPE": true, "SHIB": true, "ARB": true, "OP": true, "SUI": true,
	"APT": true, "SEI": true, "TIA": true, "JUP": true, "WIF": true,
	"NEAR": true, "ATOM": true, "FTM": true, "MATIC": true, "INJ": true,
	"RENDER": true, "FET": true, "TAO": true, "WLD": true, "USDT": true,
	"USDC": true, "BUSD": true, "DAI": true, "UNI": true, "AAVE": true,
	"LDO": true, "MKR": true, "CRV": true, "PENDLE": true, "ENA": true,
	"ONDO": true, "TRUMP": true, "TON": true, "TRX": true, "LTC": true,
	"BCH": true, "ETC": true, "FIL": true, "ICP": true, "HBAR": true,
	"VET": true, "ALGO": true, "SAND": true, "MANA": true, "AXS": true,
	"GMT": true, "APE": true, "GALA": true, "IMX": true, "BLUR": true,
	"STRK": true, "ZK": true, "W": true, "IO": true, "ZRO": true,
	"BONK": true, "FLOKI": true, "ORDI": true, "STX": true, "RUNE": true,
}

// isStockSymbol heuristically determines if a symbol is a stock ticker (not crypto).
// Stock tickers are 1-5 uppercase letters without numeric suffixes like "USDT".
// Known crypto base symbols (BTC, ETH, SOL etc.) are excluded.
func isStockSymbol(sym string) bool {
	sym = strings.ToUpper(sym)

	// Check known crypto base symbols first (critical: "BTC", "ETH" etc. are NOT stocks)
	if knownCryptoSymbols[sym] {
		return false
	}

	// If it already has a crypto quote suffix, it's crypto
	cryptoSuffixes := []string{"USDT", "BUSD", "USDC", "BTC", "ETH", "BNB"}
	for _, suffix := range cryptoSuffixes {
		if strings.HasSuffix(sym, suffix) && len(sym) > len(suffix) {
			return false
		}
	}
	// Pure uppercase letters, 1-5 chars = likely a stock ticker
	if len(sym) >= 1 && len(sym) <= 5 {
		allLetters := true
		for _, c := range sym {
			if c < 'A' || c > 'Z' {
				allLetters = false
				break
			}
		}
		if allLetters {
			return true
		}
	}
	return false
}
