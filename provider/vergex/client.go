package vergex

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"nofx/mcp"
	"nofx/mcp/payment"
	"nofx/provider/hyperliquid"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

const (
	DefaultBaseURL             = "https://claw402.ai"
	DefaultChain               = "mainnet"
	DefaultMarketType          = "hip3_perp"
	MaxDirectionChangeItems    = 30
	DirectionLeaderboardPath   = "/api/v1/vergex/direction-change/leaderboard"
	DirectionCurrentPath       = "/api/v1/vergex/direction-change/current"
	DirectionHistoryPath       = "/api/v1/vergex/direction-change/history"
	CostLiquidationHeatmapPath = "/api/v1/vergex/cost-liquidation-heatmap"
	FlowMarketsPath            = "/api/v1/vergex/flow-markets"
	HolderWinrateMapPath       = "/api/v1/vergex/holder-winrate-map"
	HolderWinrateHoldersPath   = "/api/v1/vergex/holder-winrate-map/holders"
)

type Client struct {
	baseURL    string
	privateKey *ecdsa.PrivateKey
	httpClient *http.Client
	logger     mcp.Logger
}

type Query struct {
	MarketType string
	Symbol     string
	Chain      string
	LiqBand    string
	Category   string
}

// WinrateQuery selects the holder win-rate matrix viewport. WinMin/WinMax are
// win-rate percents (0..100); CostMin/CostMax are percents of the current
// price (upstream defaults 60..140). Each pair must be sent together.
// MinRoundTrips filters addresses by verified closed round trips (default 1).
type WinrateQuery struct {
	MarketType    string
	Symbol        string
	Chain         string
	WinMin        int
	WinMax        int
	CostMin       float64
	CostMax       float64
	MinRoundTrips int
}

// WinrateHoldersQuery pins the drilldown to a matrix snapshot and a rectangle
// of its grid. Row/RowEnd index win-rate bins (0..winBins-1) and
// Column/ColumnEnd index cost slots (0 = below viewport, 1..costBins = inside,
// costBins+1 = above); ends are inclusive.
type WinrateHoldersQuery struct {
	WinrateQuery
	SnapshotID string
	Row        int
	RowEnd     int
	Column     int
	ColumnEnd  int
	Side       string
	Offset     int
	Limit      int
}

type DirectionChangeLeaderboardData struct {
	Band         int                   `json:"band"`
	UniverseSize int                   `json:"universeSize"`
	RankBy       string                `json:"rankBy"`
	AsOf         int64                 `json:"asOf"`
	Raw          json.RawMessage       `json:"raw,omitempty"`
	Items        []DirectionChangeItem `json:"items"`
}

type DirectionChangeItem struct {
	Rank         int             `json:"rank,omitempty"`
	Symbol       string          `json:"symbol"`
	APISymbol    string          `json:"api_symbol,omitempty"`
	MarketType   string          `json:"market_type,omitempty"`
	Bias         string          `json:"bias,omitempty"`
	Score        float64         `json:"score,omitempty"`
	BullishCount int             `json:"bullish_count,omitempty"`
	BearishCount int             `json:"bearish_count,omitempty"`
	NeutralCount int             `json:"neutral_count,omitempty"`
	OIRank       int             `json:"oi_rank,omitempty"`
	MarkPrice    float64         `json:"mark_price,omitempty"`
	Category     string          `json:"category,omitempty"`
	Raw          json.RawMessage `json:"raw,omitempty"`
}

type MarketAnalysis struct {
	Symbol                string               `json:"symbol"`
	QuerySymbol           string               `json:"query_symbol"`
	MarketType            string               `json:"market_type"`
	Ranking               *DirectionChangeItem `json:"ranking,omitempty"`
	DirectionCurrent      json.RawMessage      `json:"direction_current,omitempty"`
	DirectionCurrentError string               `json:"direction_current_error,omitempty"`
	DirectionHistory      json.RawMessage      `json:"direction_history,omitempty"`
	DirectionHistoryError string               `json:"direction_history_error,omitempty"`
	Heatmap               json.RawMessage      `json:"heatmap,omitempty"`
	HeatmapError          string               `json:"heatmap_error,omitempty"`
	Winrate               json.RawMessage      `json:"winrate,omitempty"`
	WinrateError          string               `json:"winrate_error,omitempty"`
}

func NewClient(baseURL, privateKeyHex string, logger mcp.Logger) (*Client, error) {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if privateKeyHex == "" {
		privateKeyHex = os.Getenv("CLAW402_WALLET_KEY")
	}
	if privateKeyHex == "" {
		return nil, fmt.Errorf("claw402 wallet private key not set")
	}
	if logger == nil {
		logger = mcp.NewNoopLogger()
	}

	hexKey := strings.TrimPrefix(strings.TrimSpace(privateKeyHex), "0x")
	pk, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid claw402 private key: %w", err)
	}

	return &Client{
		baseURL:    baseURL,
		privateKey: pk,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}, nil
}

func (c *Client) GetDirectionChangeLeaderboard(ctx context.Context) (*DirectionChangeLeaderboardData, error) {
	body, err := c.doGET(ctx, DirectionLeaderboardPath, nil)
	if err != nil {
		return nil, err
	}
	return ParseDirectionChangeLeaderboard(body)
}

func (c *Client) GetDirectionChangeCurrent(ctx context.Context, symbol string) (json.RawMessage, error) {
	if strings.TrimSpace(symbol) == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	params := url.Values{"symbol": {strings.TrimSpace(symbol)}}
	return c.doGET(ctx, DirectionCurrentPath, params)
}

func (c *Client) GetDirectionChangeHistory(ctx context.Context, symbol, eventType string, page, pageSize int) (json.RawMessage, error) {
	if strings.TrimSpace(symbol) == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	eventType = strings.ToLower(strings.TrimSpace(eventType))
	if eventType == "" {
		eventType = "all"
	}
	if eventType != "all" && eventType != "reversal" && eventType != "non_reversal" {
		return nil, fmt.Errorf("type must be all, reversal, or non_reversal")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	params := url.Values{
		"symbol":    {strings.TrimSpace(symbol)},
		"type":      {eventType},
		"page":      {fmt.Sprintf("%d", page)},
		"page_size": {fmt.Sprintf("%d", pageSize)},
	}
	return c.doGET(ctx, DirectionHistoryPath, params)
}

func (c *Client) GetCostLiquidationHeatmap(ctx context.Context, q Query) (json.RawMessage, error) {
	if strings.TrimSpace(q.MarketType) == "" || strings.TrimSpace(q.Symbol) == "" {
		return nil, winrateInvalid("marketType and symbol are required")
	}
	q.MarketType = canonicalMarketType(q.MarketType)
	if q.MarketType == "" {
		return nil, winrateInvalid("marketType must be core_perp or hip3_perp")
	}
	params := url.Values{}
	addQueryDefaults(params, q, true)
	return c.doGET(ctx, CostLiquidationHeatmapPath, params)
}

// GetFlowMarkets fetches the Vergex net-flow market ranking via the paid
// claw402 x402 endpoint. Params mirror the public API: chain (e.g. "mainnet"),
// window (e.g. "1h"), and limit. The raw JSON is returned for the caller to
// pass through — the response shape is owned by Vergex.
func (c *Client) GetFlowMarkets(ctx context.Context, chain, window string, limit int) (json.RawMessage, error) {
	params := url.Values{}
	if v := strings.TrimSpace(chain); v != "" {
		params.Set("chain", v)
	}
	if v := strings.TrimSpace(window); v != "" {
		params.Set("window", v)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	return c.doGET(ctx, FlowMarketsPath, params)
}

// GetHolderWinrateMap fetches the holder win-rate matrix via the paid claw402
// x402 endpoint: live positions bucketed by entry-cost slot × historical
// win-rate bin (20 win rows × 18 cost slots, slots 0 and costBins+1 being
// below/above-viewport catch-alls). The raw JSON is returned verbatim.
func (c *Client) GetHolderWinrateMap(ctx context.Context, q WinrateQuery) (json.RawMessage, error) {
	params, err := winrateMapParams(q)
	if err != nil {
		return nil, err
	}
	return c.doGET(ctx, HolderWinrateMapPath, params)
}

// GetHolderWinrateHolders paginates the addresses behind one rectangle of the
// win-rate matrix (paid claw402 x402 endpoint). SnapshotID must come from a
// prior GetHolderWinrateMap response.
func (c *Client) GetHolderWinrateHolders(ctx context.Context, q WinrateHoldersQuery) (json.RawMessage, error) {
	params, err := winrateHoldersParams(q)
	if err != nil {
		return nil, err
	}
	return c.doGET(ctx, HolderWinrateHoldersPath, params)
}

// WinrateValidationError identifies local errors that must not reach payment.
type WinrateValidationError struct{ Message string }

func (e *WinrateValidationError) Error() string { return e.Message }

func winrateInvalid(message string) error { return &WinrateValidationError{Message: message} }

func (q WinrateQuery) Validate() error {
	_, err := winrateMapParams(q)
	return err
}

func (q WinrateHoldersQuery) Validate() error {
	_, err := winrateHoldersParams(q)
	return err
}

func winrateHoldersParams(q WinrateHoldersQuery) (url.Values, error) {
	q.SnapshotID = strings.TrimSpace(q.SnapshotID)
	if !validWinrateSnapshotID(q.SnapshotID) {
		return nil, winrateInvalid("snapshotId must be the snapshot id returned by holder-winrate-map")
	}
	if q.Row < 0 || q.RowEnd < q.Row || q.Column < 0 || q.ColumnEnd < q.Column ||
		q.RowEnd > 19 || q.ColumnEnd > 17 {
		return nil, winrateInvalid("row range must satisfy 0 <= row <= rowEnd <= 19; column range must satisfy 0 <= column <= columnEnd <= 17")
	}
	side := strings.ToLower(strings.TrimSpace(q.Side))
	if side == "" {
		side = "long"
	}
	if side != "long" && side != "short" {
		return nil, winrateInvalid("side must be long or short")
	}
	if q.Offset < 0 || q.Offset > 1000000 {
		return nil, winrateInvalid("offset must be between 0 and 1000000")
	}
	limit := q.Limit
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return nil, winrateInvalid("limit must be between 1 and 100")
	}

	params, err := winrateMapParams(q.WinrateQuery)
	if err != nil {
		return nil, err
	}
	params.Set("snapshotId", q.SnapshotID)
	params.Set("row", fmt.Sprintf("%d", q.Row))
	params.Set("rowEnd", fmt.Sprintf("%d", q.RowEnd))
	params.Set("column", fmt.Sprintf("%d", q.Column))
	params.Set("columnEnd", fmt.Sprintf("%d", q.ColumnEnd))
	params.Set("side", side)
	params.Set("offset", fmt.Sprintf("%d", q.Offset))
	params.Set("limit", fmt.Sprintf("%d", limit))
	return params, nil
}

// winrateMapParams validates the shared matrix viewport and builds the query
// string. Every rule mirrors the claw402 gateway's pre-payment validation so
// a typo fails locally instead of as a paid 400.
func winrateMapParams(q WinrateQuery) (url.Values, error) {
	params := url.Values{}
	marketType := canonicalMarketType(q.MarketType)
	if marketType == "" {
		return nil, winrateInvalid("marketType must be hip3_perp or core_perp")
	}
	symbol := strings.TrimSpace(q.Symbol)
	if symbol == "" {
		return nil, winrateInvalid("marketType and symbol are required")
	}
	symbol = MarketSymbol(marketType, symbol)
	if symbol == "" {
		return nil, winrateInvalid("marketType and symbol are required")
	}
	params.Set("marketType", marketType)
	params.Set("symbol", symbol)
	if q.Chain != "" {
		chain := QueryChain(q.Chain)
		if chain != "mainnet" {
			return nil, winrateInvalid("chain must be mainnet")
		}
		params.Set("chain", chain)
	}

	winSet := q.WinMin != 0 || q.WinMax != 0
	if winSet {
		if q.WinMin < 0 || q.WinMax > 100 || q.WinMin >= q.WinMax {
			return nil, winrateInvalid("win-rate window must satisfy 0 <= winMin < winMax <= 100")
		}
		params.Set("winMin", fmt.Sprintf("%d", q.WinMin))
		params.Set("winMax", fmt.Sprintf("%d", q.WinMax))
	}
	costSet := q.CostMin != 0 || q.CostMax != 0
	if costSet {
		if math.IsNaN(q.CostMin) || math.IsNaN(q.CostMax) || math.IsInf(q.CostMin, 0) || math.IsInf(q.CostMax, 0) {
			return nil, winrateInvalid("cost window must be finite numbers (percent of price)")
		}
		if q.CostMin < 1 || q.CostMax > 10000 || q.CostMin >= q.CostMax || math.Trunc(q.CostMin) != q.CostMin || math.Trunc(q.CostMax) != q.CostMax {
			return nil, winrateInvalid("cost window must satisfy integer 1 <= costMin < costMax <= 10000 (percent of price)")
		}
		params.Set("costMin", trimFloat(q.CostMin, 4))
		params.Set("costMax", trimFloat(q.CostMax, 4))
	}
	if q.MinRoundTrips < 0 || q.MinRoundTrips > 10000 {
		return nil, winrateInvalid("minRoundTrips must be between 1 and 10000")
	}
	if q.MinRoundTrips > 1 {
		params.Set("minRoundTrips", fmt.Sprintf("%d", q.MinRoundTrips))
	}
	return params, nil
}

// canonicalMarketType maps user-friendly spellings onto the two claw402
// market types, defaulting like the rest of the client (hip3_perp).
func canonicalMarketType(marketType string) string {
	switch normalizeMarketType(marketType) {
	case "":
		return DefaultMarketType
	case "coreperp", "core", "crypto", "cryptoperp":
		return "core_perp"
	case "perp":
		// the terminal calls crypto majors "perp"
		return "core_perp"
	case "hip3perp", "hip3":
		return DefaultMarketType
	default:
		return ""
	}
}

// validWinrateSnapshotID checks the base32 snapshot id format issued by the
// winrate-map response (observed as 32 chars from [A-Z2-7]).
func validWinrateSnapshotID(snapshot string) bool {
	snapshot = strings.TrimSpace(snapshot)
	if len(snapshot) < 16 || len(snapshot) > 64 {
		return false
	}
	for _, r := range snapshot {
		if (r >= 'A' && r <= 'Z') || (r >= '2' && r <= '7') {
			continue
		}
		return false
	}
	return true
}

func addQueryDefaults(params url.Values, q Query, includeMarket bool) {
	if includeMarket {
		if q.MarketType != "" {
			params.Set("marketType", q.MarketType)
		}
		if q.Symbol != "" {
			params.Set("symbol", MarketSymbol(q.MarketType, q.Symbol))
		}
	}
	if q.Chain != "" {
		params.Set("chain", QueryChain(q.Chain))
	}
	if q.LiqBand != "" {
		params.Set("liqBand", q.LiqBand)
	}
}

func (c *Client) doGET(ctx context.Context, path string, params url.Values) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("vergex client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	fullURL := c.baseURL + path
	if encoded := params.Encode(); encoded != "" {
		fullURL += "?" + encoded
	}

	buildReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Client-ID", "nofx")
		return req, nil
	}

	body, err := payment.DoX402Request(
		ctx,
		c.httpClient,
		buildReq,
		payment.MakeClaw402SignFunc(c.privateKey),
		"claw402-vergex",
		c.logger,
	)
	if err != nil {
		return nil, fmt.Errorf("vergex request failed (%s): %w", path, err)
	}
	return body, nil
}

func ParseDirectionChangeLeaderboard(body []byte) (*DirectionChangeLeaderboardData, error) {
	raw := json.RawMessage(append([]byte(nil), body...))
	var meta struct {
		Band         int    `json:"band"`
		UniverseSize int    `json:"universeSize"`
		RankBy       string `json:"rankBy"`
		AsOf         int64  `json:"asOf"`
	}
	_ = json.Unmarshal(body, &meta)
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("failed to parse vergex direction-change leaderboard response: %w", err)
	}

	rows := findObjectArray(decoded)
	items := make([]DirectionChangeItem, 0, len(rows))
	for idx, row := range rows {
		obj, ok := row.(map[string]any)
		if !ok {
			continue
		}
		item, ok := parseRankItem(obj, idx+1)
		if ok {
			items = append(items, item)
		}
	}

	return &DirectionChangeLeaderboardData{Band: meta.Band, UniverseSize: meta.UniverseSize, RankBy: meta.RankBy, AsOf: meta.AsOf, Raw: raw, Items: items}, nil
}

func FilterTradFiItems(items []DirectionChangeItem, marketType string, limit int) []DirectionChangeItem {
	if marketType == "" {
		marketType = DefaultMarketType
	}
	return filterSignalRankingItems(items, marketType, limit, false)
}

func FilterDirectionChangeItems(items []DirectionChangeItem, marketType string, limit int) []DirectionChangeItem {
	return filterSignalRankingItems(items, marketType, limit, true)
}

func filterSignalRankingItems(items []DirectionChangeItem, marketType string, limit int, allowAll bool) []DirectionChangeItem {
	requestedMarketType := marketType
	normalizedMarketType := normalizeMarketType(marketType)
	includeAll := allowAll && isAllMarketType(marketType)
	if limit <= 0 {
		limit = 5
	}
	if limit > MaxDirectionChangeItems {
		limit = MaxDirectionChangeItems
	}

	out := make([]DirectionChangeItem, 0, limit)
	seen := make(map[string]bool)
	for _, item := range items {
		base := QuerySymbol(item.Symbol)
		if base == "" {
			continue
		}
		itemMarket := normalizeMarketType(item.MarketType)
		isXYZ := hyperliquid.IsXYZAsset(item.Symbol) || hyperliquid.IsXYZAsset(base)
		if !includeAll {
			if itemMarket != "" && normalizedMarketType != "" && itemMarket != normalizedMarketType && !isTradeFiMarketType(itemMarket) && !isXYZ {
				continue
			}
			if itemMarket == "" && !isXYZ {
				continue
			}
		}
		item.MarketType = coalesce(item.MarketType, inferRankingMarketType(item.Symbol, base, requestedMarketType))
		tradeSymbol := TradableSymbolForMarket(item.MarketType, item.Symbol)
		if tradeSymbol == "" || seen[tradeSymbol] {
			continue
		}
		item.Symbol = base
		item.Category = rankingCategory(item.MarketType, base)
		out = append(out, item)
		seen[tradeSymbol] = true
		if len(out) >= limit {
			break
		}
	}
	return out
}

func TradableSymbol(symbol string) string {
	return TradableSymbolForMarket(DefaultMarketType, symbol)
}

func TradableSymbolForMarket(marketType, symbol string) string {
	base := QuerySymbol(symbol)
	if base == "" {
		return ""
	}
	if isCoreMarketType(marketType) {
		return base
	}
	if isAllMarketType(marketType) && !hyperliquid.IsXYZAsset(symbol) && !hyperliquid.IsXYZAsset(base) {
		return base
	}
	return hyperliquid.FormatCoinForAPI("xyz:" + base)
}

func MarketSymbol(marketType, symbol string) string {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return ""
	}
	if strings.Contains(symbol, "/") {
		parts := strings.Split(symbol, "/")
		symbol = parts[len(parts)-1]
	}
	if strings.HasPrefix(strings.ToLower(symbol), "xyz:") {
		return "xyz:" + hyperliquid.NormalizeCoinBase(strings.TrimPrefix(strings.ToUpper(symbol), "XYZ:"))
	}
	base := QuerySymbol(symbol)
	if base == "" {
		return ""
	}
	if normalizeMarketType(marketType) == "hip3perp" {
		return "xyz:" + base
	}
	return base
}

func QuerySymbol(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return ""
	}
	symbol = strings.TrimPrefix(strings.ToUpper(symbol), "XYZ:")
	if strings.Contains(symbol, "/") {
		parts := strings.Split(symbol, "/")
		symbol = parts[len(parts)-1]
	}
	return hyperliquid.NormalizeCoinBase(symbol)
}

func QueryChain(chain string) string {
	raw := strings.TrimSpace(chain)
	normalized := strings.ToLower(raw)
	switch normalized {
	case "", "hyperliquid", "hl":
		return DefaultChain
	default:
		return raw
	}
}

func FormatAnalysisForAI(analysis *MarketAnalysis) string {
	if analysis == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### %s (Vergex %s/%s)\n", analysis.Symbol, analysis.MarketType, analysis.QuerySymbol))
	if analysis.Ranking != nil {
		sb.WriteString(fmt.Sprintf("Direction leaderboard: rank=%d bias=%s direction_score=%.0f bulls=%d bears=%d neutral=%d oi_rank=%d mark_price=%s category=%s\n",
			analysis.Ranking.Rank,
			emptyDash(analysis.Ranking.Bias),
			analysis.Ranking.Score,
			analysis.Ranking.BullishCount,
			analysis.Ranking.BearishCount,
			analysis.Ranking.NeutralCount,
			analysis.Ranking.OIRank,
			trimFloat(analysis.Ranking.MarkPrice, 6),
			emptyDash(analysis.Ranking.Category)))
	}
	if len(analysis.DirectionCurrent) > 0 {
		sb.WriteString("#### Current Bull/Bear Direction\n")
		sb.WriteString(fallbackJSONBlock(analysis.DirectionCurrent, 2200))
		sb.WriteString("\n")
	} else if analysis.DirectionCurrentError != "" {
		sb.WriteString("Current direction: unavailable (")
		sb.WriteString(truncateText(analysis.DirectionCurrentError, 360))
		sb.WriteString(")\n")
	}
	if len(analysis.DirectionHistory) > 0 {
		sb.WriteString("#### Bull/Bear Direction History\n")
		sb.WriteString(fallbackJSONBlock(analysis.DirectionHistory, 2600))
		sb.WriteString("\n")
	} else if analysis.DirectionHistoryError != "" {
		sb.WriteString("Direction history: unavailable (")
		sb.WriteString(truncateText(analysis.DirectionHistoryError, 360))
		sb.WriteString(")\n")
	}
	if len(analysis.Heatmap) > 0 {
		sb.WriteString("#### Cost/Liquidation Heatmap\n")
		sb.WriteString(FormatHeatmapMarkdown(analysis.Heatmap))
		sb.WriteString("\n")
	} else if analysis.HeatmapError != "" {
		sb.WriteString("Cost/Liquidation Heatmap: unavailable (")
		sb.WriteString(truncateText(analysis.HeatmapError, 360))
		sb.WriteString(")\n")
	}
	if len(analysis.Winrate) > 0 {
		sb.WriteString("#### Holder Win-Rate Matrix\n")
		sb.WriteString(FormatWinrateMarkdown(analysis.Winrate))
		sb.WriteString("\n")
	} else if analysis.WinrateError != "" {
		sb.WriteString("Holder Win-Rate Matrix: unavailable (")
		sb.WriteString(truncateText(analysis.WinrateError, 360))
		sb.WriteString(")\n")
	}
	return sb.String()
}

// FormatWinrateMarkdown compresses a holder win-rate matrix response into a few
// decision-oriented lines: per-side notional quality (share held by top win-rate
// decile vs bottom, entered above vs below current price) plus the water
// (unrealized PnL) split and the single largest cell. Percentages are of side
// notional; cost bands are expressed as a percent of the current price.
func FormatWinrateMarkdown(raw json.RawMessage) string {
	data, ok := decodeVergexDataObject(raw)
	if !ok {
		return fallbackJSONBlock(raw, 1600)
	}
	// Upstream regenerates snapshots in the background; a "map_building" style
	// error payload should degrade to one short line, not a JSON dump.
	if errObj := objectOf(data, "error"); errObj != nil {
		if msg := firstString(errObj, "message", "code", "reason"); msg != "" {
			return "matrix temporarily unavailable: " + msg
		}
	}
	cells := objectArray(data, "cells")
	if len(cells) == 0 {
		var sb strings.Builder
		sb.WriteString(formatWinrateQuality(data, time.Now()))
		writeScalarSummary(&sb, data, []string{"symbol", "marketType", "markPrice", "includedPositions"})
		return withFallbackIfEmpty(sb.String(), raw)
	}

	winBins := firstInt(data, "winBins")
	costBins := firstInt(data, "costBins")
	viewport := objectOf(data, "viewport")
	lo := firstFloat(viewport, "costMin")
	hi := firstFloat(viewport, "costMax")
	if winBins <= 0 {
		winBins = 20
	}
	if costBins <= 0 {
		costBins = 16
	}
	if hi <= lo {
		lo, hi = 60, 140
	}
	slotWidth := (hi - lo) / float64(costBins)
	// slot t (1..costBins) covers [lo+(t-1)w, lo+t·w) as % of price; slots 0 and
	// costBins+1 are the below/above-viewport catch-alls. The price boundary is
	// slot 9's lower edge (lo+8w == 100 by construction of the near-price band).
	priceSlot := 1
	for t := 1; t <= costBins; t++ {
		if lo+float64(t-1)*slotWidth >= 100-1e-9 {
			priceSlot = t
			break
		}
	}
	topWinFrom := winBins - 2    // top win decile rows (e.g. 90-100%)
	bottomWinTo := winBins/3 - 1 // bottom third (e.g. 0-30%)

	type sideStats struct {
		total     float64
		topWin    float64
		bottomWin float64
		abovePx   float64
		count     int
	}
	sides := map[string]*sideStats{"long": {}, "short": {}}
	type cellRef struct {
		side     string
		row      int
		column   int
		notional float64
		count    int
	}
	var best cellRef
	for _, cell := range cells {
		row := firstInt(cell, "row")
		column := firstInt(cell, "column")
		for _, sideName := range []string{"long", "short"} {
			sideCell := objectOf(cell, sideName)
			n := firstFloat(sideCell, "notional")
			if n <= 0 {
				continue
			}
			st := sides[sideName]
			st.total += n
			st.count += firstInt(sideCell, "count")
			if row >= topWinFrom {
				st.topWin += n
			}
			if row <= bottomWinTo {
				st.bottomWin += n
			}
			if column >= priceSlot {
				st.abovePx += n
			}
			if n > best.notional {
				best = cellRef{side: sideName, row: row, column: column, notional: n, count: firstInt(sideCell, "count")}
			}
		}
	}

	var sb strings.Builder
	sb.WriteString(formatWinrateQuality(data, time.Now()))
	writeScalarSummary(&sb, data, []string{"markPrice"})
	if included := objectOf(data, "included"); included != nil {
		sb.WriteString(fmt.Sprintf("- Included: %s addrs / %s\n",
			intComma(firstInt(included, "count")), formatUSDAmount(firstFloat(included, "notional"))))
	}
	if excluded := objectOf(data, "excluded"); excluded != nil {
		if noSamples := objectOf(excluded, "no_samples"); noSamples != nil {
			sb.WriteString(fmt.Sprintf("- Excluded (no verified round trip): %s addrs / %s\n",
				intComma(firstInt(noSamples, "count")), formatUSDAmount(firstFloat(noSamples, "notional"))))
		}
	}
	winBand := func(row int) string {
		return fmt.Sprintf("%d-%d%%", row*100/winBins, (row+1)*100/winBins)
	}
	for _, sideName := range []string{"long", "short"} {
		st := sides[sideName]
		if st.total <= 0 {
			sb.WriteString(fmt.Sprintf("- %s: no positions in window\n", sideName))
			continue
		}
		line := fmt.Sprintf("- %s %s / %s addrs: %s held by %d-100%% win-rate holders, %s by 0-%d%%; %s entered above current price",
			sideName, formatUSDAmount(st.total), intComma(st.count),
			sharePct(st.topWin, st.total), topWinFrom*100/winBins,
			sharePct(st.bottomWin, st.total), (bottomWinTo+1)*100/winBins,
			sharePct(st.abovePx, st.total))
		if water := objectOf(objectOf(data, "water"), sideName); water != nil {
			line += fmt.Sprintf("; unrealized: in profit %s vs underwater %s",
				formatUSDAmount(firstFloat(objectOf(water, "aboveWater"), "notional")),
				formatUSDAmount(firstFloat(objectOf(water, "belowWater"), "notional")))
		}
		sb.WriteString(line + "\n")
	}
	if best.notional > 0 {
		costBand := "above window"
		lowerPct := lo + float64(best.column-1)*slotWidth
		upperPct := lo + float64(best.column)*slotWidth
		switch {
		case best.column == 0:
			costBand = fmt.Sprintf("<%.0f%% of price", lo)
		case best.column >= costBins+1:
			costBand = fmt.Sprintf("≥%.0f%% of price", hi)
		default:
			costBand = fmt.Sprintf("%.0f-%.0f%% of price", lowerPct, upperPct)
		}
		sb.WriteString(fmt.Sprintf("- Largest cell: %s %s win-rate × entry %s (%s / %s addrs)\n",
			best.side, winBand(best.row), costBand, formatUSDAmount(best.notional), intComma(best.count)))
	}

	// The full-resolution joint view: rows = entry cost as % of current price
	// (1% steps near price, wider bands further out — the raw near-price banding,
	// ** ** marks the at-price row), columns = holder win-rate in 5% bins (the
	// raw 20 bins, NOT the human panel's 10% merge). Cells carry notional and
	// address count ("62.5M/922") so whale clusters read differently from
	// crowds. LLMs parse this joint structure directly; the aggregates above
	// cannot express where quality sits.
	if winBins == 20 && costBins == 16 {
		type cellAgg struct {
			notional float64
			count    int
		}
		agg := map[string]map[int]cellAgg{"long": {}, "short": {}}
		for _, cell := range cells {
			row := firstInt(cell, "row")
			column := firstInt(cell, "column")
			key := row*100 + column
			for _, sideName := range []string{"long", "short"} {
				side := objectOf(cell, sideName)
				n := firstFloat(side, "notional")
				if n > 0 {
					a := agg[sideName][key]
					a.notional += n
					a.count += firstInt(side, "count")
					agg[sideName][key] = a
				}
			}
		}
		groups := winrateDisplayCostGroups(lo, slotWidth)
		var header strings.Builder
		header.WriteString("| cost |")
		for w := 0; w < winBins; w++ {
			header.WriteString(fmt.Sprintf(" %d-%d |", w*100/winBins, (w+1)*100/winBins))
		}
		divider := "|---|" + strings.Repeat("---|", winBins)
		for _, sideName := range []string{"long", "short"} {
			sb.WriteString(fmt.Sprintf("\n%s grid (cells = notional/addrs, rows = entry %% of price ↓ / holder win-rate %% →):\n\n", sideName))
			sb.WriteString(header.String() + "\n" + divider + "\n")
			for _, g := range groups {
				label := g.label
				if g.atPrice {
					label = "**" + label + "**"
				}
				row := strings.Builder{}
				row.WriteString("| " + label + " |")
				for w := 0; w < winBins; w++ {
					var a cellAgg
					ok := false
					for _, s := range g.slots {
						if v, has := agg[sideName][w*100+s]; has {
							a.notional += v.notional
							a.count += v.count
							ok = true
						}
					}
					if ok {
						row.WriteString(" " + compactGridAmount(a.notional) + "/" + intComma(a.count) + " |")
					} else {
						row.WriteString(" 0 |")
					}
				}
				row.WriteString("\n")
				sb.WriteString(row.String())
			}
		}
	}
	return withFallbackIfEmpty(sb.String(), raw)
}

// This is a conservative interpretation warning, not an upstream freshness SLA.
// Keep the 15-minute threshold aligned with the terminal quality indicator.
func formatWinrateQuality(data map[string]any, now time.Time) string {
	var sb strings.Builder
	writeScalarSummary(&sb, data, []string{"snapshotId", "coverage", "asOf", "positionsAsOf", "priceAsOf", "historyMode", "metricVersion", "minRoundTrips", "staleHistoryCount", "oldestHistory", "newestHistory"})
	var warnings []string
	coverage := firstString(data, "coverage")
	if coverage != "complete" {
		if coverage == "" {
			coverage = "unknown"
		}
		warnings = append(warnings, "coverage="+coverage+"; aggregates may not represent all holders")
	}
	for _, key := range []string{"asOf", "positionsAsOf"} {
		stamp, err := time.Parse(time.RFC3339Nano, firstString(data, key))
		switch {
		case err != nil:
			warnings = append(warnings, key+" is unknown")
		case now.Sub(stamp) > 15*time.Minute:
			warnings = append(warnings, key+" is older than 15 minutes")
		case stamp.Sub(now) > time.Minute:
			warnings = append(warnings, key+" is in the future")
		}
	}
	if value, ok := data["staleHistoryCount"]; !ok || value == nil {
		warnings = append(warnings, "history freshness is unknown")
	} else if n := firstInt(data, "staleHistoryCount"); n > 0 {
		warnings = append(warnings, fmt.Sprintf("%d stale holder histories", n))
	}
	if len(warnings) > 0 {
		sb.WriteString("- DATA QUALITY WARNING: " + strings.Join(warnings, "; ") + ". Do not treat these aggregates as complete, current market evidence.\n")
	}
	sb.WriteString("- Historical holder win rate is not a forecast or an independent trading signal; position and history timestamps may differ.\n")
	return sb.String()
}

// winrateDisplayGroup is one merged display row of the matrix grid.
type winrateDisplayGroup struct {
	label   string
	slots   []int
	atPrice bool
}

// winrateDisplayCostGroups mirrors the upstream panel's near-price banding:
// 1,1,2,4 doubling away from the at-price slot (bin 9), with the two
// below/above-viewport catch-all rows.
func winrateDisplayCostGroups(lo, slotWidth float64) []winrateDisplayGroup {
	pct := func(t int) float64 { return lo + float64(t-1)*slotWidth }
	label := func(a, b float64) string {
		return fmt.Sprintf("%.0f-%.0f", a, b)
	}
	return []winrateDisplayGroup{
		{label: fmt.Sprintf("≥%.0f", pct(17)), slots: []int{17}},
		{label: label(pct(13), pct(17)), slots: []int{13, 14, 15, 16}},
		{label: label(pct(11), pct(13)), slots: []int{11, 12}},
		{label: label(pct(10), pct(11)), slots: []int{10}},
		{label: label(pct(9), pct(10)), slots: []int{9}, atPrice: true},
		{label: label(pct(8), pct(9)), slots: []int{8}},
		{label: label(pct(7), pct(8)), slots: []int{7}},
		{label: label(pct(5), pct(7)), slots: []int{5, 6}},
		{label: label(pct(1), pct(5)), slots: []int{1, 2, 3, 4}},
		{label: fmt.Sprintf("<%.0f", pct(1)), slots: []int{0}},
	}
}

// compactGridAmount renders a cell notional for the AI grid: "1.2M", "345K",
// "0" — no currency symbol, minimal tokens.
func compactGridAmount(n float64) string {
	switch {
	case n >= 1e9:
		return trimFloat(n/1e9, 1) + "B"
	case n >= 1e6:
		return trimFloat(n/1e6, 1) + "M"
	case n >= 1e3:
		return trimFloat(n/1e3, 0) + "K"
	case n > 0:
		return trimFloat(n, 0)
	default:
		return "0"
	}
}

func objectOf(obj map[string]any, key string) map[string]any {
	if obj == nil {
		return nil
	}
	val, ok := lookupNormalized(obj, key)
	if !ok {
		return nil
	}
	nested, ok := val.(map[string]any)
	if !ok {
		return nil
	}
	return nested
}

func sharePct(part, total float64) string {
	if total <= 0 {
		return "0%"
	}
	return fmt.Sprintf("%.0f%%", part/total*100)
}

func intComma(v int) string {
	return fmt.Sprintf("%d", v)
}

func FormatHeatmapMarkdown(raw json.RawMessage) string {
	data, ok := decodeVergexDataObject(raw)
	if !ok {
		return fallbackJSONBlock(raw, 2600)
	}

	bins := objectArray(data, "bins")
	if len(bins) == 0 {
		var sb strings.Builder
		writeScalarSummary(&sb, data, []string{"symbol", "marketType", "band", "liqBand", "currentPrice", "price", "binStep"})
		return withFallbackIfEmpty(sb.String(), raw)
	}

	zones := make([]heatmapZone, 0, len(bins))
	var totalLongCost, totalShortCost, totalLongLiq, totalShortLiq float64
	for _, bin := range bins {
		zone := heatmapZone{
			Start:     firstFloat(bin, "bucketStartPrice", "start", "startPrice"),
			End:       firstFloat(bin, "bucketEndPrice", "end", "endPrice"),
			PX:        firstFloat(bin, "px", "price"),
			LongCost:  firstFloat(bin, "longCost"),
			ShortCost: firstFloat(bin, "shortCost"),
			LongLiq:   firstFloat(bin, "longLiq", "longLiquidation"),
			ShortLiq:  firstFloat(bin, "shortLiq", "shortLiquidation"),
		}
		totalLongCost += zone.LongCost
		totalShortCost += zone.ShortCost
		totalLongLiq += zone.LongLiq
		totalShortLiq += zone.ShortLiq
		zone.Score = maxFloat(zone.LongCost, zone.ShortCost, zone.LongLiq, zone.ShortLiq)
		if zone.Score > 0 {
			zones = append(zones, zone)
		}
	}
	sortHeatmapZones(zones)

	var sb strings.Builder
	writeScalarSummary(&sb, data, []string{"symbol", "marketType", "band", "liqBand", "currentPrice", "price", "binStep"})
	sb.WriteString(fmt.Sprintf("- Total cost: long %s / short %s\n", formatUSDAmount(totalLongCost), formatUSDAmount(totalShortCost)))
	sb.WriteString(fmt.Sprintf("- Total liquidation: long %s / short %s\n", formatUSDAmount(totalLongLiq), formatUSDAmount(totalShortLiq)))
	sb.WriteString("| Price zone | Long cost | Short cost | Long liq | Short liq | Main cluster |\n")
	sb.WriteString("| --- | ---: | ---: | ---: | ---: | --- |\n")
	limit := minInt(len(zones), 10)
	for _, zone := range zones[:limit] {
		sb.WriteString("| ")
		sb.WriteString(markdownCell(formatPriceZone(zone)))
		sb.WriteString(" | ")
		sb.WriteString(markdownCell(formatUSDAmount(zone.LongCost)))
		sb.WriteString(" | ")
		sb.WriteString(markdownCell(formatUSDAmount(zone.ShortCost)))
		sb.WriteString(" | ")
		sb.WriteString(markdownCell(formatUSDAmount(zone.LongLiq)))
		sb.WriteString(" | ")
		sb.WriteString(markdownCell(formatUSDAmount(zone.ShortLiq)))
		sb.WriteString(" | ")
		sb.WriteString(markdownCell(zone.MainCluster()))
		sb.WriteString(" |\n")
	}
	if len(zones) > limit {
		sb.WriteString(fmt.Sprintf("- Additional heatmap bins omitted: %d\n", len(zones)-limit))
	}
	return withFallbackIfEmpty(sb.String(), raw)
}

type heatmapZone struct {
	Start     float64
	End       float64
	PX        float64
	LongCost  float64
	ShortCost float64
	LongLiq   float64
	ShortLiq  float64
	Score     float64
}

func (z heatmapZone) MainCluster() string {
	maxVal := maxFloat(z.LongCost, z.ShortCost, z.LongLiq, z.ShortLiq)
	switch maxVal {
	case z.LongCost:
		return "long cost"
	case z.ShortCost:
		return "short cost"
	case z.LongLiq:
		return "long liquidation"
	case z.ShortLiq:
		return "short liquidation"
	default:
		return "-"
	}
}

func sortHeatmapZones(zones []heatmapZone) {
	sort.SliceStable(zones, func(i, j int) bool {
		return zones[i].Score > zones[j].Score
	})
}

func decodeVergexDataObject(raw json.RawMessage) (map[string]any, bool) {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, false
	}
	obj, ok := decoded.(map[string]any)
	if !ok {
		return nil, false
	}
	if data, ok := lookupNormalized(obj, "data"); ok {
		if dataObj, ok := data.(map[string]any); ok {
			return dataObj, true
		}
	}
	return obj, true
}

func writeScalarSummary(sb *strings.Builder, obj map[string]any, keys []string) {
	wrote := false
	for _, key := range keys {
		value, ok := lookupNormalized(obj, key)
		if !ok {
			continue
		}
		text := formatScalarValue(value)
		if text == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("- %s: %s\n", titleKey(key), text))
		wrote = true
	}
	if wrote {
		sb.WriteString("\n")
	}
}

func objectArray(obj map[string]any, key string) []map[string]any {
	val, ok := lookupNormalized(obj, key)
	if !ok {
		return nil
	}
	rows, ok := val.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if rowObj, ok := row.(map[string]any); ok {
			out = append(out, rowObj)
		}
	}
	return out
}

func formatOptionalFloat(obj map[string]any, key string) string {
	val, ok := lookupNormalized(obj, key)
	if !ok {
		return "-"
	}
	num, ok := anyFloat(val)
	if !ok {
		return formatScalarValue(val)
	}
	return trimFloat(num, 1)
}

func anyFloat(val any) (float64, bool) {
	switch t := val.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		var f float64
		if _, err := fmt.Sscanf(strings.TrimSpace(t), "%f", &f); err == nil {
			return f, true
		}
	}
	return 0, false
}

func formatScalarValue(val any) string {
	switch t := val.(type) {
	case string:
		return strings.TrimSpace(t)
	case bool:
		return fmt.Sprintf("%t", t)
	case float64:
		return trimFloat(t, 4)
	case json.Number:
		f, err := t.Float64()
		if err == nil {
			return trimFloat(f, 4)
		}
		return t.String()
	default:
		if f, ok := anyFloat(val); ok {
			return trimFloat(f, 4)
		}
		return ""
	}
}

func formatPriceZone(z heatmapZone) string {
	if z.Start != 0 || z.End != 0 {
		return fmt.Sprintf("%s-%s", trimFloat(z.Start, 4), trimFloat(z.End, 4))
	}
	if z.PX != 0 {
		return trimFloat(z.PX, 4)
	}
	return "-"
}

func formatUSDAmount(v float64) string {
	abs := math.Abs(v)
	sign := ""
	if v < 0 {
		sign = "-"
	}
	switch {
	case abs >= 1_000_000_000:
		return fmt.Sprintf("%s$%.2fB", sign, abs/1_000_000_000)
	case abs >= 1_000_000:
		return fmt.Sprintf("%s$%.2fM", sign, abs/1_000_000)
	case abs >= 1_000:
		return fmt.Sprintf("%s$%.2fK", sign, abs/1_000)
	default:
		return fmt.Sprintf("%s$%.2f", sign, abs)
	}
}

func trimFloat(v float64, precision int) string {
	text := fmt.Sprintf("%.*f", precision, v)
	text = strings.TrimRight(text, "0")
	text = strings.TrimRight(text, ".")
	if text == "-0" {
		return "0"
	}
	return text
}

func markdownCell(text string) string {
	text = strings.ReplaceAll(strings.TrimSpace(text), "\n", " ")
	text = strings.ReplaceAll(text, "|", "\\|")
	if text == "" {
		return "-"
	}
	return text
}

func titleKey(key string) string {
	switch key {
	case "marketType":
		return "Market type"
	case "liqBand":
		return "Liquidation band"
	case "currentPrice":
		return "Current price"
	case "markPrice":
		return "Mark price"
	case "binStep":
		return "Bin step"
	case "compositeZ":
		return "Composite Z"
	default:
		if key == "" {
			return ""
		}
		return strings.ToUpper(key[:1]) + key[1:]
	}
}

func withFallbackIfEmpty(text string, raw json.RawMessage) string {
	if strings.TrimSpace(text) == "" {
		return fallbackJSONBlock(raw, 2200)
	}
	return text
}

func fallbackJSONBlock(raw json.RawMessage, maxBytes int) string {
	return "```json\n" + CompactJSON(raw, maxBytes) + "\n```\n"
}

func maxFloat(values ...float64) float64 {
	max := 0.0
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func CompactJSON(raw json.RawMessage, maxBytes int) string {
	if len(raw) == 0 {
		return "{}"
	}
	var buf any
	if err := json.Unmarshal(raw, &buf); err == nil {
		if compact, err := json.Marshal(buf); err == nil {
			raw = compact
		}
	}
	text := string(raw)
	if maxBytes > 0 && len(text) > maxBytes {
		return text[:maxBytes] + "...<truncated>"
	}
	return text
}

func truncateText(text string, maxBytes int) string {
	text = strings.TrimSpace(text)
	if maxBytes <= 0 || len(text) <= maxBytes {
		return text
	}
	return text[:maxBytes] + "...<truncated>"
}

func parseRankItem(obj map[string]any, fallbackRank int) (DirectionChangeItem, bool) {
	symbol := firstString(obj, "symbol", "ticker", "base", "coin", "asset", "market", "name")
	if symbol == "" {
		symbol = nestedMarketString(obj, "symbol", "ticker", "base", "coin", "asset", "name")
	}
	if symbol == "" {
		return DirectionChangeItem{}, false
	}
	raw, _ := json.Marshal(obj)
	rank := firstInt(obj, "rank", "ranking", "position")
	if rank <= 0 {
		rank = fallbackRank
	}
	score := firstFloat(obj, "directionScore", "direction_score", "score")
	marketType := firstString(obj, "marketType", "market_type", "venue")
	if marketType == "" {
		marketType = nestedMarketString(obj, "marketType", "market_type", "venue", "type")
	}
	item := DirectionChangeItem{
		Rank:         rank,
		Symbol:       QuerySymbol(symbol),
		APISymbol:    symbol,
		MarketType:   marketType,
		Bias:         firstString(obj, "bias", "direction", "side", "signal"),
		Score:        score,
		BullishCount: firstInt(obj, "bullishCount"),
		BearishCount: firstInt(obj, "bearishCount"),
		NeutralCount: firstInt(obj, "neutralCount"),
		OIRank:       firstInt(obj, "oiRank"),
		MarkPrice:    firstFloat(obj, "markPrice"),
		Raw:          raw,
	}
	if item.Symbol != "" {
		item.Category = hyperliquid.XYZCategory(item.Symbol)
	}
	return item, item.Symbol != ""
}

func nestedMarketString(obj map[string]any, keys ...string) string {
	val, ok := lookupNormalized(obj, "market")
	if !ok {
		return ""
	}
	nested, ok := val.(map[string]any)
	if !ok {
		return ""
	}
	return firstString(nested, keys...)
}

func findObjectArray(v any) []any {
	switch t := v.(type) {
	case []any:
		if arrayLooksLikeRows(t) {
			return t
		}
		for _, item := range t {
			if rows := findObjectArray(item); len(rows) > 0 {
				return rows
			}
		}
	case map[string]any:
		for _, key := range []string{"data", "items", "results", "ranking", "rankings", "rows", "markets", "signals"} {
			if val, ok := lookupNormalized(t, key); ok {
				if rows := findObjectArray(val); len(rows) > 0 {
					return rows
				}
			}
		}
		for _, val := range t {
			if rows := findObjectArray(val); len(rows) > 0 {
				return rows
			}
		}
	}
	return nil
}

func arrayLooksLikeRows(rows []any) bool {
	for _, row := range rows {
		obj, ok := row.(map[string]any)
		if !ok {
			continue
		}
		if firstString(obj, "symbol", "ticker", "base", "coin", "asset", "market", "name") != "" {
			return true
		}
	}
	return false
}

func firstString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		val, ok := lookupNormalized(obj, key)
		if !ok {
			continue
		}
		switch t := val.(type) {
		case string:
			if strings.TrimSpace(t) != "" {
				return strings.TrimSpace(t)
			}
		case fmt.Stringer:
			if strings.TrimSpace(t.String()) != "" {
				return strings.TrimSpace(t.String())
			}
		}
	}
	return ""
}

func firstFloat(obj map[string]any, keys ...string) float64 {
	for _, key := range keys {
		val, ok := lookupNormalized(obj, key)
		if !ok {
			continue
		}
		switch t := val.(type) {
		case float64:
			return t
		case int:
			return float64(t)
		case json.Number:
			f, _ := t.Float64()
			return f
		case string:
			var f float64
			if _, err := fmt.Sscanf(strings.TrimSpace(t), "%f", &f); err == nil {
				return f
			}
		}
	}
	return 0
}

func firstInt(obj map[string]any, keys ...string) int {
	for _, key := range keys {
		val, ok := lookupNormalized(obj, key)
		if !ok {
			continue
		}
		switch t := val.(type) {
		case float64:
			return int(t)
		case int:
			return t
		case json.Number:
			i, _ := t.Int64()
			return int(i)
		case string:
			var i int
			if _, err := fmt.Sscanf(strings.TrimSpace(t), "%d", &i); err == nil {
				return i
			}
		}
	}
	return 0
}

func lookupNormalized(obj map[string]any, key string) (any, bool) {
	want := normalizeKey(key)
	for k, v := range obj {
		if normalizeKey(k) == want {
			return v, true
		}
	}
	return nil, false
}

func normalizeKey(key string) string {
	replacer := strings.NewReplacer("_", "", "-", "", " ", "", ".", "")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(key)))
}

func normalizeMarketType(marketType string) string {
	replacer := strings.NewReplacer("_", "", "-", "", " ", "", ".", "", "/", "")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(marketType)))
}

func isTradeFiMarketType(marketType string) bool {
	switch normalizeMarketType(marketType) {
	case "hip3perp", "hip3", "xyz", "xyzperp", "tradefi", "tradfi",
		"stock", "stocks", "equity", "equities", "usequity", "usequities", "usstock", "usstocks",
		"commodity", "commodities", "forex", "fx", "index", "indices", "preipo":
		return true
	default:
		return false
	}
}

func isAllMarketType(marketType string) bool {
	switch normalizeMarketType(marketType) {
	case "", "all", "any", "ranking", "signalranking", "claw402", "vergex":
		return true
	default:
		return false
	}
}

func isCoreMarketType(marketType string) bool {
	switch normalizeMarketType(marketType) {
	case "coreperp", "core", "crypto", "cryptoperp":
		return true
	default:
		return false
	}
}

func inferRankingMarketType(symbol, base, fallback string) string {
	if !isAllMarketType(fallback) && strings.TrimSpace(fallback) != "" {
		return fallback
	}
	if hyperliquid.IsXYZAsset(symbol) || hyperliquid.IsXYZAsset(base) {
		return DefaultMarketType
	}
	return "core_perp"
}

func rankingCategory(marketType, base string) string {
	if isCoreMarketType(marketType) {
		return "crypto"
	}
	return hyperliquid.XYZCategory(base)
}

func coalesce(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
