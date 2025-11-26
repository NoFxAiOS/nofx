package market

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	okxBaseURL = "https://www.okx.com"
)

type APIClient struct {
	client *http.Client
}

func NewAPIClient() *APIClient {
	return &APIClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type OKXResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func (c *APIClient) GetExchangeInfo() (*ExchangeInfo, error) {
	url := fmt.Sprintf("%s/api/v5/public/instruments?instType=SWAP", okxBaseURL)
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var okxResp OKXResponse
	err = json.Unmarshal(body, &okxResp)
	if err != nil {
		return nil, err
	}

	if okxResp.Code != "0" {
		return nil, fmt.Errorf("OKX API error: %s", okxResp.Msg)
	}

	var instruments []map[string]interface{}
	err = json.Unmarshal(okxResp.Data, &instruments)
	if err != nil {
		return nil, err
	}

	var symbols []SymbolInfo
	for _, inst := range instruments {
		instId, _ := inst["instId"].(string)
		if strings.HasSuffix(instId, "-USDT-SWAP") {
			baseCcy, _ := inst["baseCcy"].(string)
			symbols = append(symbols, SymbolInfo{
				Symbol: baseCcy + "USDT",
				Status: "TRADING",
			})
		}
	}

	return &ExchangeInfo{Symbols: symbols}, nil
}

func symbolToOKXInstId(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.TrimSuffix(symbol, "USDT")
	return symbol + "-USDT-SWAP"
}

func okxBarToInterval(interval string) string {
	switch interval {
	case "1m":
		return "1m"
	case "3m":
		return "3m"
	case "5m":
		return "5m"
	case "15m":
		return "15m"
	case "30m":
		return "30m"
	case "1h":
		return "1H"
	case "4h":
		return "4H"
	case "1d":
		return "1D"
	default:
		return interval
	}
}

func (c *APIClient) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	instId := symbolToOKXInstId(symbol)
	bar := okxBarToInterval(interval)

	url := fmt.Sprintf("%s/api/v5/market/candles?instId=%s&bar=%s&limit=%d",
		okxBaseURL, instId, bar, limit)

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var okxResp OKXResponse
	err = json.Unmarshal(body, &okxResp)
	if err != nil {
		return nil, fmt.Errorf("JSON解析失败: %v, body: %s", err, string(body))
	}

	if okxResp.Code != "0" {
		return nil, fmt.Errorf("OKX API error: %s", okxResp.Msg)
	}

	var rawKlines [][]string
	err = json.Unmarshal(okxResp.Data, &rawKlines)
	if err != nil {
		return nil, fmt.Errorf("解析K线数据失败: %v", err)
	}

	var klines []Kline
	for i := len(rawKlines) - 1; i >= 0; i-- {
		kr := rawKlines[i]
		kline, err := parseOKXKline(kr)
		if err != nil {
			log.Printf("解析K线数据失败: %v", err)
			continue
		}
		klines = append(klines, kline)
	}

	return klines, nil
}

func parseOKXKline(kr []string) (Kline, error) {
	var kline Kline

	if len(kr) < 9 {
		return kline, fmt.Errorf("invalid kline data: length=%d", len(kr))
	}

	openTime, _ := strconv.ParseInt(kr[0], 10, 64)
	kline.OpenTime = openTime
	kline.Open, _ = strconv.ParseFloat(kr[1], 64)
	kline.High, _ = strconv.ParseFloat(kr[2], 64)
	kline.Low, _ = strconv.ParseFloat(kr[3], 64)
	kline.Close, _ = strconv.ParseFloat(kr[4], 64)
	kline.Volume, _ = strconv.ParseFloat(kr[5], 64)
	kline.QuoteVolume, _ = strconv.ParseFloat(kr[7], 64)
	kline.CloseTime = openTime + 180000

	return kline, nil
}

func (c *APIClient) GetCurrentPrice(symbol string) (float64, error) {
	instId := symbolToOKXInstId(symbol)

	url := fmt.Sprintf("%s/api/v5/market/ticker?instId=%s", okxBaseURL, instId)
	resp, err := c.client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var okxResp OKXResponse
	err = json.Unmarshal(body, &okxResp)
	if err != nil {
		return 0, err
	}

	if okxResp.Code != "0" {
		return 0, fmt.Errorf("OKX API error: %s", okxResp.Msg)
	}

	var tickers []map[string]string
	err = json.Unmarshal(okxResp.Data, &tickers)
	if err != nil {
		return 0, err
	}

	if len(tickers) == 0 {
		return 0, fmt.Errorf("no ticker data")
	}

	price, err := strconv.ParseFloat(tickers[0]["last"], 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}
