package nofxos

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// QuantData represents quantitative data for a single coin
type QuantData struct {
	Symbol      string             `json:"symbol"`
	Price       float64            `json:"price"`
	Netflow     *NetflowData       `json:"netflow,omitempty"`
	OI          map[string]*OIData `json:"oi,omitempty"` // keyed by exchange: "binance", "bybit"
	PriceChange map[string]float64 `json:"price_change,omitempty"` // keyed by duration: "1h", "4h", etc.
}

// NetflowData contains fund flow data
type NetflowData struct {
	Institution *FlowTypeData `json:"institution,omitempty"`
	Personal    *FlowTypeData `json:"personal,omitempty"`
}

// FlowTypeData contains flow data by trade type
type FlowTypeData struct {
	Future map[string]float64 `json:"future,omitempty"` // keyed by duration
	Spot   map[string]float64 `json:"spot,omitempty"`   // keyed by duration
}

// OIData contains open interest data for an exchange
type OIData struct {
	CurrentOI float64                 `json:"current_oi"`
	NetLong   float64                 `json:"net_long"`
	NetShort  float64                 `json:"net_short"`
	Delta     map[string]*OIDeltaData `json:"delta,omitempty"` // keyed by duration
}

// OIDeltaData contains OI change data
type OIDeltaData struct {
	OIDelta        float64 `json:"oi_delta"`
	OIDeltaValue   float64 `json:"oi_delta_value"`
	OIDeltaPercent float64 `json:"oi_delta_percent"` // Already x100
}

// CoinResponse is the API response structure for coin details
type CoinResponse struct {
	Success bool       `json:"success"`
	Code    int        `json:"code"`
	Data    *QuantData `json:"data"`
}

// GetCoinData retrieves quantitative data for a single coin
func (c *Client) GetCoinData(symbol string, include string) (*QuantData, error) {
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}

	if include == "" {
		include = "netflow,oi,price"
	}

	// Normalize symbol (remove USDT suffix for API call if needed)
	symbol = strings.TrimSuffix(strings.ToUpper(symbol), "USDT")

	endpoint := fmt.Sprintf("/api/coin/%s?include=%s", symbol, include)

	body, err := c.doRequest(endpoint)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var response CoinResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("JSON parsing failed: %w", err)
	}

	// Check for success (support both success field and code field)
	if !response.Success && response.Code != 0 {
		return nil, fmt.Errorf("API returned error code: %d", response.Code)
	}

	return response.Data, nil
}

// GetCoinDataBatch retrieves quantitative data for multiple coins
func (c *Client) GetCoinDataBatch(symbols []string, include string) map[string]*QuantData {
	result := make(map[string]*QuantData)

	for _, symbol := range symbols {
		data, err := c.GetCoinData(symbol, include)
		if err != nil {
			log.Printf("⚠️  Failed to fetch coin data for %s: %v", symbol, err)
			continue
		}
		if data != nil {
			// Use normalized symbol as key
			normalizedSymbol := NormalizeSymbol(symbol)
			result[normalizedSymbol] = data
		}
	}

	return result
}

// FormatQuantDataForAI formats single coin quant data for AI consumption
func FormatQuantDataForAI(symbol string, data *QuantData, lang Language) string {
	if data == nil {
		return ""
	}

	if lang == LangChinese {
		return formatQuantDataZH(symbol, data)
	}
	return formatQuantDataEN(symbol, data)
}

func formatQuantDataZH(symbol string, data *QuantData) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("### %s 量化数据\n", symbol))
	sb.WriteString(fmt.Sprintf("价格: $%.4f\n\n", data.Price))

	if len(data.PriceChange) > 0 {
		sb.WriteString("**价格变化**:\n")
		durations := []string{"1h", "4h", "8h", "12h", "24h"}
		for _, d := range durations {
			if change, ok := data.PriceChange[d]; ok {
				sb.WriteString(fmt.Sprintf("- %s: %+.2f%%\n", d, change*100))
			}
		}
		sb.WriteString("\n")
	}

	if len(data.OI) > 0 {
		for exchange, oiData := range data.OI {
			if oiData != nil {
				sb.WriteString(fmt.Sprintf("**%s持仓**:\n", strings.ToUpper(exchange)))
				sb.WriteString(fmt.Sprintf("- OI: %.2f\n", oiData.CurrentOI))
				if oiData.NetLong > 0 || oiData.NetShort > 0 {
					sb.WriteString(fmt.Sprintf("- 多头: %.2f, 空头: %.2f\n", oiData.NetLong, oiData.NetShort))
				}
				if oiData.Delta != nil {
					if delta, ok := oiData.Delta["1h"]; ok && delta != nil {
						sb.WriteString(fmt.Sprintf("- 1h变化: %s (%.2f%%)\n",
							formatValue(delta.OIDeltaValue), delta.OIDeltaPercent))
					}
				}
				sb.WriteString("\n")
			}
		}
	}

	if data.Netflow != nil && data.Netflow.Institution != nil && data.Netflow.Institution.Future != nil {
		sb.WriteString("**机构资金流**:\n")
		durations := []string{"1h", "4h", "24h"}
		for _, d := range durations {
			if flow, ok := data.Netflow.Institution.Future[d]; ok {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", d, formatValue(flow)))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatQuantDataEN(symbol string, data *QuantData) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("### %s Quant Data\n", symbol))
	sb.WriteString(fmt.Sprintf("Price: $%.4f\n\n", data.Price))

	if len(data.PriceChange) > 0 {
		sb.WriteString("**Price Change**:\n")
		durations := []string{"1h", "4h", "8h", "12h", "24h"}
		for _, d := range durations {
			if change, ok := data.PriceChange[d]; ok {
				sb.WriteString(fmt.Sprintf("- %s: %+.2f%%\n", d, change*100))
			}
		}
		sb.WriteString("\n")
	}

	if len(data.OI) > 0 {
		for exchange, oiData := range data.OI {
			if oiData != nil {
				sb.WriteString(fmt.Sprintf("**%s OI**:\n", strings.ToUpper(exchange)))
				sb.WriteString(fmt.Sprintf("- Current OI: %.2f\n", oiData.CurrentOI))
				if oiData.NetLong > 0 || oiData.NetShort > 0 {
					sb.WriteString(fmt.Sprintf("- Net Long: %.2f, Net Short: %.2f\n", oiData.NetLong, oiData.NetShort))
				}
				if oiData.Delta != nil {
					if delta, ok := oiData.Delta["1h"]; ok && delta != nil {
						sb.WriteString(fmt.Sprintf("- 1h Change: %s (%.2f%%)\n",
							formatValue(delta.OIDeltaValue), delta.OIDeltaPercent))
					}
				}
				sb.WriteString("\n")
			}
		}
	}

	if data.Netflow != nil && data.Netflow.Institution != nil && data.Netflow.Institution.Future != nil {
		sb.WriteString("**Institution Fund Flow**:\n")
		durations := []string{"1h", "4h", "24h"}
		for _, d := range durations {
			if flow, ok := data.Netflow.Institution.Future[d]; ok {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", d, formatValue(flow)))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
