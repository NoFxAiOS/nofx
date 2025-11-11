package market

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"nofx/hook"
	"strconv"
	"time"
)

const (
	baseURL = "https://fapi.binance.com"
)

type APIClient struct {
	client *http.Client
}

func NewAPIClient() *APIClient {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	hookRes := hook.HookExec[hook.SetHttpClientResult](hook.SET_HTTP_CLIENT, client)
	if hookRes != nil && hookRes.Error() == nil {
		log.Printf("使用Hook设置的HTTP客户端")
		client = hookRes.GetResult()
	}

	return &APIClient{
		client: client,
	}
}

func (c *APIClient) GetExchangeInfo() (*ExchangeInfo, error) {
	url := fmt.Sprintf("%s/fapi/v1/exchangeInfo", baseURL)
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var exchangeInfo ExchangeInfo
	err = json.Unmarshal(body, &exchangeInfo)
	if err != nil {
		return nil, err
	}

	return &exchangeInfo, nil
}

func (c *APIClient) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	url := fmt.Sprintf("%s/fapi/v1/klines", baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("symbol", symbol)
	q.Add("interval", interval)
	q.Add("limit", strconv.Itoa(limit))
	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var klineResponses []KlineResponse
	err = json.Unmarshal(body, &klineResponses)
	if err != nil {
		log.Printf("获取K线数据失败,响应内容: %s", string(body))
		return nil, err
	}

	var klines []Kline
	for _, kr := range klineResponses {
		kline, err := parseKline(kr)
		if err != nil {
			log.Printf("解析K线数据失败: %v", err)
			continue
		}
		klines = append(klines, kline)
	}

	return klines, nil
}

func parseKline(kr KlineResponse) (Kline, error) {
	var kline Kline

	if len(kr) < 11 {
		return kline, fmt.Errorf("invalid kline data")
	}

	// 🔒 安全的类型断言，防止 panic
	// 解析各个字段
	openTime, ok := kr[0].(float64)
	if !ok {
		return kline, fmt.Errorf("invalid OpenTime data type")
	}
	kline.OpenTime = int64(openTime)

	// 🔒 安全的字符串类型断言
	if openStr, ok := kr[1].(string); ok {
		kline.Open, _ = strconv.ParseFloat(openStr, 64)
	}
	if highStr, ok := kr[2].(string); ok {
		kline.High, _ = strconv.ParseFloat(highStr, 64)
	}
	if lowStr, ok := kr[3].(string); ok {
		kline.Low, _ = strconv.ParseFloat(lowStr, 64)
	}
	if closeStr, ok := kr[4].(string); ok {
		kline.Close, _ = strconv.ParseFloat(closeStr, 64)
	}
	if volStr, ok := kr[5].(string); ok {
		kline.Volume, _ = strconv.ParseFloat(volStr, 64)
	}

	closeTime, ok := kr[6].(float64)
	if !ok {
		return kline, fmt.Errorf("invalid CloseTime data type")
	}
	kline.CloseTime = int64(closeTime)

	if quoteVolStr, ok := kr[7].(string); ok {
		kline.QuoteVolume, _ = strconv.ParseFloat(quoteVolStr, 64)
	}

	trades, ok := kr[8].(float64)
	if !ok {
		return kline, fmt.Errorf("invalid Trades data type")
	}
	kline.Trades = int(trades)

	if takerBuyBaseStr, ok := kr[9].(string); ok {
		kline.TakerBuyBaseVolume, _ = strconv.ParseFloat(takerBuyBaseStr, 64)
	}
	if takerBuyQuoteStr, ok := kr[10].(string); ok {
		kline.TakerBuyQuoteVolume, _ = strconv.ParseFloat(takerBuyQuoteStr, 64)
	}

	return kline, nil
}

func (c *APIClient) GetCurrentPrice(symbol string) (float64, error) {
	url := fmt.Sprintf("%s/fapi/v1/ticker/price", baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	q := req.URL.Query()
	q.Add("symbol", symbol)
	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var ticker PriceTicker
	err = json.Unmarshal(body, &ticker)
	if err != nil {
		return 0, err
	}

	price, err := strconv.ParseFloat(ticker.Price, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}
