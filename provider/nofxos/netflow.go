package nofxos

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// NetFlowPosition represents fund flow data for a single coin
type NetFlowPosition struct {
	Rank   int     `json:"rank"`
	Symbol string  `json:"symbol"`
	Amount float64 `json:"amount"` // Fund flow amount in USDT (positive=inflow, negative=outflow)
	Price  float64 `json:"price"`
}

// NetFlowResponse is the API response structure
type NetFlowResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Netflows  []NetFlowPosition `json:"netflows"`
		Count     int               `json:"count"`
		Type      string            `json:"type"`      // institution or personal
		Trade     string            `json:"trade"`     // 合约 or 现货
		TimeRange string            `json:"time_range"`
		RankType  string            `json:"rank_type"` // top or low
		Limit     int               `json:"limit"`
	} `json:"data"`
}

// NetFlowRankingData contains institution and personal fund flow rankings
type NetFlowRankingData struct {
	Duration             string            `json:"duration"`
	TimeRange            string            `json:"time_range"`
	InstitutionFutureTop []NetFlowPosition `json:"institution_future_top"`
	InstitutionFutureLow []NetFlowPosition `json:"institution_future_low"`
	PersonalFutureTop    []NetFlowPosition `json:"personal_future_top"`
	PersonalFutureLow    []NetFlowPosition `json:"personal_future_low"`
	FetchedAt            time.Time         `json:"fetched_at"`
}

// GetNetFlowRanking retrieves NetFlow ranking data (institution/personal, top/low)
func (c *Client) GetNetFlowRanking(duration string, limit int) (*NetFlowRankingData, error) {
	if duration == "" {
		duration = "1h"
	}
	if limit <= 0 {
		limit = 10
	}

	result := &NetFlowRankingData{
		Duration:  duration,
		FetchedAt: time.Now(),
	}

	// Fetch institution futures top (inflow)
	positions, timeRange, err := c.fetchNetFlowRanking("top", duration, limit, "institution", "future")
	if err != nil {
		log.Printf("⚠️  Failed to fetch institution future inflow ranking: %v", err)
	} else {
		result.InstitutionFutureTop = positions
		result.TimeRange = timeRange
	}

	// Fetch institution futures low (outflow)
	positions, _, err = c.fetchNetFlowRanking("low", duration, limit, "institution", "future")
	if err != nil {
		log.Printf("⚠️  Failed to fetch institution future outflow ranking: %v", err)
	} else {
		result.InstitutionFutureLow = positions
	}

	// Fetch personal futures top (retail inflow)
	positions, _, err = c.fetchNetFlowRanking("top", duration, limit, "personal", "future")
	if err != nil {
		log.Printf("⚠️  Failed to fetch personal future inflow ranking: %v", err)
	} else {
		result.PersonalFutureTop = positions
	}

	// Fetch personal futures low (retail outflow)
	positions, _, err = c.fetchNetFlowRanking("low", duration, limit, "personal", "future")
	if err != nil {
		log.Printf("⚠️  Failed to fetch personal future outflow ranking: %v", err)
	} else {
		result.PersonalFutureLow = positions
	}

	log.Printf("✓ Fetched NetFlow ranking data: inst_in=%d, inst_out=%d, retail_in=%d, retail_out=%d (duration: %s)",
		len(result.InstitutionFutureTop), len(result.InstitutionFutureLow),
		len(result.PersonalFutureTop), len(result.PersonalFutureLow), duration)

	return result, nil
}

func (c *Client) fetchNetFlowRanking(rankType, duration string, limit int, flowType, trade string) ([]NetFlowPosition, string, error) {
	endpoint := fmt.Sprintf("/api/netflow/%s-ranking?limit=%d&duration=%s&type=%s&trade=%s",
		rankType, limit, duration, flowType, trade)

	body, err := c.doRequest(endpoint)
	if err != nil {
		return nil, "", fmt.Errorf("request failed: %w", err)
	}

	var response NetFlowResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, "", fmt.Errorf("JSON parsing failed: %w", err)
	}

	if !response.Success {
		return nil, "", fmt.Errorf("API returned failure status")
	}

	return response.Data.Netflows, response.Data.TimeRange, nil
}

// FormatNetFlowRankingForAI formats NetFlow ranking data for AI consumption
func FormatNetFlowRankingForAI(data *NetFlowRankingData, lang Language) string {
	if data == nil {
		return ""
	}

	if lang == LangChinese {
		return formatNetFlowRankingZH(data)
	}
	return formatNetFlowRankingEN(data)
}

func formatNetFlowRankingZH(data *NetFlowRankingData) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## 资金流向排行 (%s)\n\n", data.Duration))

	// Institution inflow
	if len(data.InstitutionFutureTop) > 0 {
		sb.WriteString("### 机构资金流入榜\n")
		sb.WriteString("Smart Money买入信号:\n\n")
		sb.WriteString("| 排名 | 币种 | 流入金额(USDT) | 价格 |\n")
		sb.WriteString("|------|------|----------------|------|\n")
		for _, pos := range data.InstitutionFutureTop {
			sb.WriteString(fmt.Sprintf("| %d | %s | %s | $%.4f |\n",
				pos.Rank, pos.Symbol, formatValue(pos.Amount), pos.Price))
		}
		sb.WriteString("\n")
	}

	// Institution outflow
	if len(data.InstitutionFutureLow) > 0 {
		sb.WriteString("### 机构资金流出榜\n")
		sb.WriteString("Smart Money卖出信号:\n\n")
		sb.WriteString("| 排名 | 币种 | 流出金额(USDT) | 价格 |\n")
		sb.WriteString("|------|------|----------------|------|\n")
		for _, pos := range data.InstitutionFutureLow {
			sb.WriteString(fmt.Sprintf("| %d | %s | %s | $%.4f |\n",
				pos.Rank, pos.Symbol, formatValue(pos.Amount), pos.Price))
		}
		sb.WriteString("\n")
	}

	// Retail flow summary
	if len(data.PersonalFutureTop) > 0 || len(data.PersonalFutureLow) > 0 {
		sb.WriteString("### 散户资金动向\n")
		if len(data.PersonalFutureTop) > 0 {
			sb.WriteString("散户买入: ")
			for i, pos := range data.PersonalFutureTop {
				if i >= 3 {
					break
				}
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("%s(%s)", pos.Symbol, formatValue(pos.Amount)))
			}
			sb.WriteString("\n")
		}
		if len(data.PersonalFutureLow) > 0 {
			sb.WriteString("散户卖出: ")
			for i, pos := range data.PersonalFutureLow {
				if i >= 3 {
					break
				}
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("%s(%s)", pos.Symbol, formatValue(pos.Amount)))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("**解读**: 机构买入+散户卖出=强烈看多 | 机构卖出+散户买入=强烈看空\n\n")
	return sb.String()
}

func formatNetFlowRankingEN(data *NetFlowRankingData) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Fund Flow Ranking (%s)\n\n", data.Duration))

	// Institution inflow
	if len(data.InstitutionFutureTop) > 0 {
		sb.WriteString("### Institution Inflow\n")
		sb.WriteString("Smart Money buying signals:\n\n")
		sb.WriteString("| Rank | Symbol | Inflow (USDT) | Price |\n")
		sb.WriteString("|------|--------|---------------|-------|\n")
		for _, pos := range data.InstitutionFutureTop {
			sb.WriteString(fmt.Sprintf("| %d | %s | %s | $%.4f |\n",
				pos.Rank, pos.Symbol, formatValue(pos.Amount), pos.Price))
		}
		sb.WriteString("\n")
	}

	// Institution outflow
	if len(data.InstitutionFutureLow) > 0 {
		sb.WriteString("### Institution Outflow\n")
		sb.WriteString("Smart Money selling signals:\n\n")
		sb.WriteString("| Rank | Symbol | Outflow (USDT) | Price |\n")
		sb.WriteString("|------|--------|----------------|-------|\n")
		for _, pos := range data.InstitutionFutureLow {
			sb.WriteString(fmt.Sprintf("| %d | %s | %s | $%.4f |\n",
				pos.Rank, pos.Symbol, formatValue(pos.Amount), pos.Price))
		}
		sb.WriteString("\n")
	}

	// Retail flow summary
	if len(data.PersonalFutureTop) > 0 || len(data.PersonalFutureLow) > 0 {
		sb.WriteString("### Retail Flow\n")
		if len(data.PersonalFutureTop) > 0 {
			sb.WriteString("Retail buying: ")
			for i, pos := range data.PersonalFutureTop {
				if i >= 3 {
					break
				}
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("%s(%s)", pos.Symbol, formatValue(pos.Amount)))
			}
			sb.WriteString("\n")
		}
		if len(data.PersonalFutureLow) > 0 {
			sb.WriteString("Retail selling: ")
			for i, pos := range data.PersonalFutureLow {
				if i >= 3 {
					break
				}
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("%s(%s)", pos.Symbol, formatValue(pos.Amount)))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("**Key**: Institution buy + Retail sell = Strong bullish | Institution sell + Retail buy = Strong bearish\n\n")
	return sb.String()
}
