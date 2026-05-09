package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"nofx/market"

	"github.com/gin-gonic/gin"
)

// HotCoinResponse is the API response for hot coins / OI ranking endpoints.
type HotCoinResponse struct {
	Coins     []HotCoinItem `json:"coins"`
	UpdatedAt string        `json:"updated_at"`
	Exchange  string        `json:"exchange"`
}

// HotCoinItem is a single coin in the ranking response.
type HotCoinItem struct {
	Symbol                string                  `json:"symbol"`
	Score                 float64                 `json:"score"`
	TradabilityScore      float64                 `json:"tradability_score,omitempty"`
	Volume24h             float64                 `json:"volume_24h"`
	OI                    float64                 `json:"oi"`
	OIChangePct           float64                 `json:"oi_change_pct,omitempty"`
	OIChangeWindowSeconds int                     `json:"oi_change_window_seconds,omitempty"`
	OISource              string                  `json:"oi_source,omitempty"`
	PriceChange24h        float64                 `json:"price_change_24h"`
	Source                string                  `json:"source,omitempty"`
	Quality               market.CandidateQuality `json:"quality,omitempty"`
}

func toHotCoinItems(coins []market.HotCoin) []HotCoinItem {
	items := make([]HotCoinItem, len(coins))
	for i, c := range coins {
		items[i] = HotCoinItem{
			Symbol:                c.Symbol,
			Score:                 c.HotScore,
			TradabilityScore:      c.Quality.Tradability,
			Volume24h:             c.QuoteVolume24h,
			OI:                    c.OpenInterestUSD,
			OIChangePct:           c.OpenInterestChangePct,
			OIChangeWindowSeconds: c.OpenInterestWindowSec,
			OISource:              c.OpenInterestSource,
			PriceChange24h:        c.PriceChangePct,
			Source:                c.Source,
			Quality:               c.Quality,
		}
	}
	return items
}

// handleHotCoins GET /api/market/hot-coins?limit=20&exchange=binance&excluded=COIN1,COIN2
func (s *Server) handleHotCoins(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	exchange := c.DefaultQuery("exchange", "okx")
	excluded := parseExcluded(c.Query("excluded"))

	coins, err := market.GetHotCoinsWithExchange(limit, excluded, exchange)
	if err != nil {
		SafeInternalError(c, "Get hot coins", err)
		return
	}

	c.JSON(http.StatusOK, HotCoinResponse{
		Coins:     toHotCoinItems(coins),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Exchange:  exchange,
	})
}

// handleOIRanking GET /api/market/oi-ranking?direction=top&limit=20&exchange=okx&excluded=COIN1,COIN2
func (s *Server) handleOIRanking(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	direction := c.DefaultQuery("direction", "top")
	exchange := c.DefaultQuery("exchange", "okx")
	excluded := parseExcluded(c.Query("excluded"))

	var coins []market.HotCoin
	var err error
	if direction == "low" {
		coins, err = market.GetOILowCoinsWithExchange(limit, excluded, exchange)
	} else {
		coins, err = market.GetOITopCoinsWithExchange(limit, excluded, exchange)
	}
	if err != nil {
		SafeInternalError(c, "Get OI ranking", err)
		return
	}

	c.JSON(http.StatusOK, HotCoinResponse{
		Coins:     toHotCoinItems(coins),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Exchange:  exchange,
	})
}

// CoinDataResponse wraps market.Data with JSON tags for the API.
type CoinDataResponse struct {
	Symbol            string                                 `json:"symbol"`
	CurrentPrice      float64                                `json:"current_price"`
	PriceChange1h     float64                                `json:"price_change_1h"`
	PriceChange4h     float64                                `json:"price_change_4h"`
	FundingRate       float64                                `json:"funding_rate"`
	LongShortRatio    *float64                               `json:"long_short_ratio,omitempty"`
	TopTraderRatio    *float64                               `json:"top_trader_ratio,omitempty"`
	TakerBuySellRatio *float64                               `json:"taker_buy_sell_ratio,omitempty"`
	DepthBidTotal     *float64                               `json:"depth_bid_total,omitempty"`
	DepthAskTotal     *float64                               `json:"depth_ask_total,omitempty"`
	DepthImbalance    *float64                               `json:"depth_imbalance,omitempty"`
	FibonacciLevels   *market.FibonacciLevels                `json:"fibonacci_levels,omitempty"`
	StructuralLevels  []market.StructuralLevel               `json:"structural_levels,omitempty"`
	OpenInterest      *market.OIData                         `json:"open_interest,omitempty"`
	TimeframeData     map[string]*market.TimeframeSeriesData `json:"timeframe_data,omitempty"`
}

// handleCoinData GET /api/market/coin-data?symbol=BTCUSDT
func (s *Server) handleCoinData(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		SafeBadRequest(c, "symbol parameter is required")
		return
	}

	timeframes := []string{"5m", "15m", "1h", "4h"}
	exchange := c.Query("exchange")
	if exchange == "" {
		exchange = "okx"
	}
	data, err := market.GetWithTimeframesExchange(symbol, timeframes, "15m", 100, exchange)
	if err != nil {
		SafeInternalError(c, "Get coin data", err)
		return
	}

	resp := CoinDataResponse{
		Symbol:            data.Symbol,
		CurrentPrice:      data.CurrentPrice,
		PriceChange1h:     data.PriceChange1h,
		PriceChange4h:     data.PriceChange4h,
		FundingRate:       data.FundingRate,
		LongShortRatio:    data.LongShortRatio,
		TopTraderRatio:    data.TopTraderRatio,
		TakerBuySellRatio: data.TakerBuySellRatio,
		DepthBidTotal:     data.DepthBidTotal,
		DepthAskTotal:     data.DepthAskTotal,
		DepthImbalance:    data.DepthImbalance,
		FibonacciLevels:   data.FibonacciLevels,
		StructuralLevels:  data.StructuralLevels,
		OpenInterest:      data.OpenInterest,
		TimeframeData:     data.TimeframeData,
	}

	c.JSON(http.StatusOK, resp)
}

// handleCompositeMarket GET /api/market/composite?symbol=BTCUSDT&exchange=okx&timeframes=3m,5m,15m,1h,4h,1d&primary=15m&count=120&ttl=15
func (s *Server) handleCompositeMarket(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		SafeBadRequest(c, "symbol parameter is required")
		return
	}
	exchange := c.DefaultQuery("exchange", "okx")
	primary := c.DefaultQuery("primary", "15m")
	count, _ := strconv.Atoi(c.DefaultQuery("count", "120"))
	if count <= 0 || count > 300 {
		count = 120
	}
	ttlSeconds, _ := strconv.Atoi(c.DefaultQuery("ttl", "180"))
	if ttlSeconds < 30 {
		ttlSeconds = 30
	}
	if ttlSeconds > 600 {
		ttlSeconds = 300
	}
	timeframes := []string{"3m", "5m", "15m", "1h", "4h", "1d"}
	if raw := strings.TrimSpace(c.Query("timeframes")); raw != "" {
		timeframes = parseExcluded(raw)
	}

	snapshot, err := market.BuildCompositeMarketSnapshot(symbol, exchange, timeframes, primary, count, time.Duration(ttlSeconds)*time.Second)
	if err != nil {
		SafeInternalError(c, "Get composite market snapshot", err)
		return
	}
	view := c.DefaultQuery("view", "chart")
	c.JSON(http.StatusOK, market.ProjectCompositeMarketSnapshot(snapshot, view))
}

func parseExcluded(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
