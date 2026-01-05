package nofxos

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// PriceRankingItem represents single coin price ranking data
type PriceRankingItem struct {
	Pair         string  `json:"pair"`
	Symbol       string  `json:"symbol"`
	PriceDelta   float64 `json:"price_delta"`    // Decimal format: 0.0723 = 7.23%
	Price        float64 `json:"price"`
	FutureFlow   float64 `json:"future_flow"`
	SpotFlow     float64 `json:"spot_flow"`
	OI           float64 `json:"oi"`
	OIDelta      float64 `json:"oi_delta"`
	OIDeltaValue float64 `json:"oi_delta_value"`
}

// PriceRankingDuration contains top gainers and losers for a single duration
type PriceRankingDuration struct {
	Top []PriceRankingItem `json:"top"`
	Low []PriceRankingItem `json:"low"`
}

// PriceRankingResponse is the API response structure
type PriceRankingResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Durations []string                        `json:"durations"`
		Limit     int                             `json:"limit"`
		Data      map[string]PriceRankingDuration `json:"data"`
	} `json:"data"`
}

// PriceRankingData contains price ranking data for multiple durations
type PriceRankingData struct {
	Durations map[string]*PriceRankingDuration `json:"durations"`
	FetchedAt time.Time                        `json:"fetched_at"`
}

// GetPriceRanking retrieves price ranking data (gainers/losers)
func (c *Client) GetPriceRanking(durations string, limit int) (*PriceRankingData, error) {
	if durations == "" {
		durations = "1h"
	}
	if limit <= 0 {
		limit = 10
	}

	endpoint := fmt.Sprintf("/api/price/ranking?duration=%s&limit=%d", durations, limit)

	body, err := c.doRequest(endpoint)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var response PriceRankingResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("JSON parsing failed: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("API returned failure status")
	}

	result := &PriceRankingData{
		Durations: make(map[string]*PriceRankingDuration),
		FetchedAt: time.Now(),
	}

	for duration, data := range response.Data.Data {
		d := data // Create a copy to avoid pointer issues
		result.Durations[duration] = &d
	}

	log.Printf("✓ Fetched Price ranking data for %d durations", len(result.Durations))

	return result, nil
}

// FormatPriceRankingForAI formats Price ranking data for AI consumption
func FormatPriceRankingForAI(data *PriceRankingData, lang Language) string {
	if data == nil || len(data.Durations) == 0 {
		return ""
	}

	if lang == LangChinese {
		return formatPriceRankingZH(data)
	}
	return formatPriceRankingEN(data)
}

func formatPriceRankingZH(data *PriceRankingData) string {
	var sb strings.Builder

	sb.WriteString("## 涨跌幅排行\n\n")

	durationOrder := []string{"1h", "4h", "24h"}
	for _, duration := range durationOrder {
		durationData, exists := data.Durations[duration]
		if !exists || durationData == nil {
			continue
		}

		sb.WriteString(fmt.Sprintf("### %s 涨跌幅\n\n", duration))

		if len(durationData.Top) > 0 {
			sb.WriteString("**涨幅榜**\n")
			sb.WriteString("| 币种 | 涨幅 | 价格 | 资金流 | OI变化 |\n")
			sb.WriteString("|------|------|------|--------|--------|\n")
			for _, item := range durationData.Top {
				sb.WriteString(fmt.Sprintf("| %s | %+.2f%% | $%.4f | %s | %s |\n",
					item.Symbol, item.PriceDelta*100, item.Price,
					formatValue(item.FutureFlow), formatValue(item.OIDeltaValue)))
			}
			sb.WriteString("\n")
		}

		if len(durationData.Low) > 0 {
			sb.WriteString("**跌幅榜**\n")
			sb.WriteString("| 币种 | 跌幅 | 价格 | 资金流 | OI变化 |\n")
			sb.WriteString("|------|------|------|--------|--------|\n")
			for _, item := range durationData.Low {
				sb.WriteString(fmt.Sprintf("| %s | %.2f%% | $%.4f | %s | %s |\n",
					item.Symbol, item.PriceDelta*100, item.Price,
					formatValue(item.FutureFlow), formatValue(item.OIDeltaValue)))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("**解读**: 涨幅大+资金流入+OI增加=强势上涨 | 跌幅大+资金流出+OI减少=弱势下跌\n\n")
	return sb.String()
}

func formatPriceRankingEN(data *PriceRankingData) string {
	var sb strings.Builder

	sb.WriteString("## Price Gainers/Losers\n\n")

	durationOrder := []string{"1h", "4h", "24h"}
	for _, duration := range durationOrder {
		durationData, exists := data.Durations[duration]
		if !exists || durationData == nil {
			continue
		}

		sb.WriteString(fmt.Sprintf("### %s Price Change\n\n", duration))

		if len(durationData.Top) > 0 {
			sb.WriteString("**Top Gainers**\n")
			sb.WriteString("| Symbol | Change | Price | Fund Flow | OI Change |\n")
			sb.WriteString("|--------|--------|-------|-----------|----------|\n")
			for _, item := range durationData.Top {
				sb.WriteString(fmt.Sprintf("| %s | %+.2f%% | $%.4f | %s | %s |\n",
					item.Symbol, item.PriceDelta*100, item.Price,
					formatValue(item.FutureFlow), formatValue(item.OIDeltaValue)))
			}
			sb.WriteString("\n")
		}

		if len(durationData.Low) > 0 {
			sb.WriteString("**Top Losers**\n")
			sb.WriteString("| Symbol | Change | Price | Fund Flow | OI Change |\n")
			sb.WriteString("|--------|--------|-------|-----------|----------|\n")
			for _, item := range durationData.Low {
				sb.WriteString(fmt.Sprintf("| %s | %.2f%% | $%.4f | %s | %s |\n",
					item.Symbol, item.PriceDelta*100, item.Price,
					formatValue(item.FutureFlow), formatValue(item.OIDeltaValue)))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("**Key**: Big gain + Fund inflow + OI increase = Strong bullish | Big loss + Fund outflow + OI decrease = Strong bearish\n\n")
	return sb.String()
}
