package market

import (
	"nofx/safe"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"nofx/hook"
	"strconv"
	"time"
)

const (
	baseURL = "https://fapi.binance.com"
)

// truncateBody returns the first 512 bytes of body for error messages.
func truncateBody(body []byte) string {
	if len(body) > 512 {
		return string(body[:512]) + "..."
	}
	return string(body)
}

type APIClient struct {
	client *http.Client
}

func NewAPIClient() *APIClient {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	hookRes := hook.HookExec[hook.SetHttpClientResult](hook.SET_HTTP_CLIENT, client)
	if hookRes != nil && hookRes.Error() == nil {
		log.Printf("Using HTTP client set by Hook")
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

	body, err := safe.ReadAllLimited(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Binance API error (status %d): %s", resp.StatusCode, truncateBody(body))
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

	body, err := safe.ReadAllLimited(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Binance klines API error (status %d): %s", resp.StatusCode, truncateBody(body))
	}

	var klineResponses []KlineResponse
	err = json.Unmarshal(body, &klineResponses)
	if err != nil {
		log.Printf("Failed to get K-line data, response content: %s", string(body))
		return nil, err
	}

	var klines []Kline
	for _, kr := range klineResponses {
		kline, err := parseKline(kr)
		if err != nil {
			log.Printf("Failed to parse K-line data: %v", err)
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

	// Parse each field with safe type assertions to prevent panics on unexpected API responses
	if v, ok := kr[0].(float64); ok {
		kline.OpenTime = int64(v)
	}
	if v, ok := kr[1].(string); ok {
		kline.Open, _ = strconv.ParseFloat(v, 64)
	}
	if v, ok := kr[2].(string); ok {
		kline.High, _ = strconv.ParseFloat(v, 64)
	}
	if v, ok := kr[3].(string); ok {
		kline.Low, _ = strconv.ParseFloat(v, 64)
	}
	if v, ok := kr[4].(string); ok {
		kline.Close, _ = strconv.ParseFloat(v, 64)
	}
	if v, ok := kr[5].(string); ok {
		kline.Volume, _ = strconv.ParseFloat(v, 64)
	}
	if v, ok := kr[6].(float64); ok {
		kline.CloseTime = int64(v)
	}
	if v, ok := kr[7].(string); ok {
		kline.QuoteVolume, _ = strconv.ParseFloat(v, 64)
	}
	if v, ok := kr[8].(float64); ok {
		kline.Trades = int(v)
	}
	if v, ok := kr[9].(string); ok {
		kline.TakerBuyBaseVolume, _ = strconv.ParseFloat(v, 64)
	}
	if v, ok := kr[10].(string); ok {
		kline.TakerBuyQuoteVolume, _ = strconv.ParseFloat(v, 64)
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

	body, err := safe.ReadAllLimited(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Binance price API error (status %d): %s", resp.StatusCode, truncateBody(body))
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
