package gmgn

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultHost              = "https://openapi.gmgn.ai"
	DefaultUSDCQuoteDecimals = 6
)

type Client struct {
	apiKey     string
	host       string
	httpClient *http.Client
	signer     signer
}

type signer struct {
	algorithm string
	ed25519   ed25519.PrivateKey
	rsa       *rsa.PrivateKey
}

type envelope[T any] struct {
	Code    any    `json:"code"`
	Message string `json:"message"`
	Error   string `json:"error"`
	Reason  string `json:"reason"`
	Data    T      `json:"data"`
}

type UserInfoResponse struct {
	Wallets []WalletEntry `json:"wallets"`
}

type WalletEntry struct {
	Chain    string         `json:"chain"`
	Address  string         `json:"address"`
	Balances []TokenBalance `json:"balances"`
}

type TokenBalance struct {
	Symbol       string `json:"symbol"`
	TokenAddress string `json:"token_address"`
	Balance      string `json:"balance"`
	USDValue     string `json:"usd_value"`
}

type TokenInfo struct {
	Address   string          `json:"address"`
	Symbol    string          `json:"symbol"`
	Name      string          `json:"name"`
	Decimals  int             `json:"decimals"`
	Liquidity string          `json:"liquidity"`
	Price     *TokenPriceInfo `json:"price"`
	Pool      *TokenPoolInfo  `json:"pool"`
}

type TokenPriceInfo struct {
	Price     string `json:"price"`
	Price1m   string `json:"price_1m"`
	Price5m   string `json:"price_5m"`
	Price1h   string `json:"price_1h"`
	Price4h   string `json:"price_4h"`
	Price24h  string `json:"price_24h"`
	Volume1m  string `json:"volume_1m"`
	Volume5m  string `json:"volume_5m"`
	Volume1h  string `json:"volume_1h"`
	Volume4h  string `json:"volume_4h"`
	Volume24h string `json:"volume_24h"`
}

type TokenPoolInfo struct {
	QuoteAddress string `json:"quote_address"`
	QuoteSymbol  string `json:"quote_symbol"`
}

type KlineResponse struct {
	List []KlineItem `json:"list"`
}

type TrendingResponse struct {
	Rank []TrendingRankItem `json:"rank"`
}

type TrendingRankItem struct {
	Address string `json:"address"`
	Symbol  string `json:"symbol"`
	Name    string `json:"name"`
	Chain   string `json:"chain"`
}

type KlineItem struct {
	Time   int64  `json:"time"`
	Open   string `json:"open"`
	High   string `json:"high"`
	Low    string `json:"low"`
	Close  string `json:"close"`
	Volume string `json:"volume"`
	Amount string `json:"amount"`
}

type WalletHoldingsResponse struct {
	List []WalletHoldingItem `json:"list"`
	Next string              `json:"next"`
}

type WalletHoldingItem struct {
	Balance             string           `json:"balance"`
	USDValue            string           `json:"usd_value"`
	AccuAmount          string           `json:"accu_amount"`
	AccuCost            string           `json:"accu_cost"`
	AccuFee             string           `json:"accu_fee"`
	RealizedProfit      string           `json:"realized_profit"`
	UnrealizedProfit    string           `json:"unrealized_profit"`
	TotalProfit         string           `json:"total_profit"`
	HistoryBoughtAmount string           `json:"history_bought_amount"`
	HistoryBoughtCost   string           `json:"history_bought_cost"`
	HistorySoldAmount   string           `json:"history_sold_amount"`
	HistorySoldIncome   string           `json:"history_sold_income"`
	LastActiveTimestamp int64            `json:"last_active_timestamp"`
	StartHoldingAt      int64            `json:"start_holding_at"`
	EndHoldingAt        int64            `json:"end_holding_at"`
	WalletTokenTags     []string         `json:"wallet_token_tags"`
	Token               HoldingTokenInfo `json:"token"`
}

type HoldingTokenInfo struct {
	TokenAddress string `json:"token_address"`
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
	Decimals     int    `json:"decimals"`
	Price        string `json:"price"`
	Liquidity    string `json:"liquidity"`
}

type WalletActivityResponse struct {
	Activities []WalletActivity `json:"activities"`
	Next       string           `json:"next"`
}

type WalletActivity struct {
	Wallet        string            `json:"wallet"`
	Chain         string            `json:"chain"`
	TxHash        string            `json:"tx_hash"`
	Timestamp     int64             `json:"timestamp"`
	EventType     string            `json:"event_type"`
	TokenAmount   string            `json:"token_amount"`
	QuoteAmount   string            `json:"quote_amount"`
	CostUSD       string            `json:"cost_usd"`
	BuyCostUSD    string            `json:"buy_cost_usd"`
	PriceUSD      string            `json:"price_usd"`
	Price         string            `json:"price"`
	IsOpenOrClose int               `json:"is_open_or_close"`
	QuoteAddress  string            `json:"quote_address"`
	FromAddress   string            `json:"from_address"`
	ToAddress     string            `json:"to_address"`
	GasNative     string            `json:"gas_native"`
	GasUSD        string            `json:"gas_usd"`
	DEXUSD        string            `json:"dex_usd"`
	PriorityFee   string            `json:"priority_fee"`
	TipFee        string            `json:"tip_fee"`
	Token         ActivityTokenInfo `json:"token"`
	QuoteToken    QuoteTokenInfo    `json:"quote_token"`
}

type ActivityTokenInfo struct {
	Address string `json:"address"`
	Symbol  string `json:"symbol"`
}

type QuoteTokenInfo struct {
	TokenAddress string `json:"token_address"`
	Symbol       string `json:"symbol"`
	Decimals     int    `json:"decimals"`
}

type WalletStats struct {
	WalletAddress     string `json:"wallet_address"`
	NativeBalance     string `json:"native_balance"`
	RealizedProfit    string `json:"realized_profit"`
	RealizedProfitPnl string `json:"realized_profit_pnl"`
}

type OrderResponse struct {
	Status          string       `json:"status"`
	Hash            string       `json:"hash"`
	OrderID         string       `json:"order_id"`
	ErrorCode       string       `json:"error_code"`
	ErrorStatus     string       `json:"error_status"`
	StrategyOrderID string       `json:"strategy_order_id"`
	Report          *OrderReport `json:"report"`
}

type OrderReport struct {
	InputToken          string `json:"input_token"`
	InputTokenDecimals  int    `json:"input_token_decimals"`
	InputAmount         string `json:"input_amount"`
	OutputToken         string `json:"output_token"`
	OutputTokenDecimals int    `json:"output_token_decimals"`
	OutputAmount        string `json:"output_amount"`
	QuoteToken          string `json:"quote_token"`
	QuoteDecimals       int    `json:"quote_decimals"`
	QuoteAmount         string `json:"quote_amount"`
	BaseToken           string `json:"base_token"`
	BaseDecimals        int    `json:"base_decimals"`
	BaseAmount          string `json:"base_amount"`
	Price               string `json:"price"`
	PriceUSD            string `json:"price_usd"`
	GasNative           string `json:"gas_native"`
	GasUSD              string `json:"gas_usd"`
}

type StrategyOrdersResponse struct {
	NextPageToken string                   `json:"next_page_token"`
	Total         int                      `json:"total"`
	List          []map[string]interface{} `json:"list"`
}

type SwapParams struct {
	Chain           string                   `json:"chain"`
	FromAddress     string                   `json:"from_address"`
	InputToken      string                   `json:"input_token"`
	OutputToken     string                   `json:"output_token"`
	InputAmount     string                   `json:"input_amount"`
	InputAmountBPS  string                   `json:"input_amount_bps,omitempty"`
	Slippage        float64                  `json:"slippage,omitempty"`
	AutoSlippage    bool                     `json:"auto_slippage,omitempty"`
	PriorityFee     string                   `json:"priority_fee,omitempty"`
	TipFee          string                   `json:"tip_fee,omitempty"`
	GasPrice        string                   `json:"gas_price,omitempty"`
	ConditionOrders []StrategyConditionOrder `json:"condition_orders,omitempty"`
	SellRatioType   string                   `json:"sell_ratio_type,omitempty"`
}

type StrategyConditionOrder struct {
	OrderType    string `json:"order_type"`
	Side         string `json:"side"`
	PriceScale   string `json:"price_scale,omitempty"`
	SellRatio    string `json:"sell_ratio"`
	DrawdownRate string `json:"drawdown_rate,omitempty"`
}

type StrategyCreateParams struct {
	Chain           string  `json:"chain"`
	FromAddress     string  `json:"from_address"`
	BaseToken       string  `json:"base_token"`
	QuoteToken      string  `json:"quote_token"`
	OrderType       string  `json:"order_type"`
	SubOrderType    string  `json:"sub_order_type"`
	CheckPrice      string  `json:"check_price"`
	AmountIn        string  `json:"amount_in,omitempty"`
	AmountInPercent string  `json:"amount_in_percent,omitempty"`
	Slippage        float64 `json:"slippage,omitempty"`
	AutoSlippage    bool    `json:"auto_slippage,omitempty"`
	PriorityFee     string  `json:"priority_fee,omitempty"`
	TipFee          string  `json:"tip_fee,omitempty"`
	GasPrice        string  `json:"gas_price,omitempty"`
}

type StrategyCancelParams struct {
	Chain         string `json:"chain"`
	FromAddress   string `json:"from_address"`
	OrderID       string `json:"order_id"`
	CloseSellMode string `json:"close_sell_model,omitempty"`
}

func NewClient(apiKey, privateKeyPEM string) (*Client, error) {
	c := &Client{
		apiKey: apiKey,
		host:   DefaultHost,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
	if strings.TrimSpace(privateKeyPEM) != "" {
		s, err := newSigner(privateKeyPEM)
		if err != nil {
			return nil, err
		}
		c.signer = *s
	}
	return c, nil
}

func (c *Client) SetHost(host string) {
	if strings.TrimSpace(host) == "" {
		return
	}
	c.host = strings.TrimRight(host, "/")
}

func (c *Client) GetUserInfo() (*UserInfoResponse, error) {
	return request[UserInfoResponse](c, http.MethodGet, "/v1/user/info", nil, nil, false)
}

func (c *Client) GetTokenInfo(chain, address string) (*TokenInfo, error) {
	return request[TokenInfo](c, http.MethodGet, "/v1/token/info", map[string]any{
		"chain":   chain,
		"address": address,
	}, nil, false)
}

func (c *Client) GetTokenKline(chain, address, resolution string, fromMs, toMs int64) (*KlineResponse, error) {
	query := map[string]any{
		"chain":      chain,
		"address":    address,
		"resolution": resolution,
	}
	if fromMs > 0 {
		query["from"] = fromMs
	}
	if toMs > 0 {
		query["to"] = toMs
	}
	return request[KlineResponse](c, http.MethodGet, "/v1/market/token_kline", query, nil, false)
}

func (c *Client) GetTrending(chain, interval string, extra map[string]any) (*TrendingResponse, error) {
	query := map[string]any{
		"chain":    chain,
		"interval": interval,
	}
	for k, v := range extra {
		query[k] = v
	}
	return request[TrendingResponse](c, http.MethodGet, "/v1/market/rank", query, nil, false)
}

func (c *Client) GetWalletHoldings(chain, wallet string, extra map[string]any) (*WalletHoldingsResponse, error) {
	query := map[string]any{
		"chain":          chain,
		"wallet_address": wallet,
	}
	for k, v := range extra {
		query[k] = v
	}
	return request[WalletHoldingsResponse](c, http.MethodGet, "/v1/user/wallet_holdings", query, nil, false)
}

func (c *Client) GetWalletActivity(chain, wallet string, extra map[string]any) (*WalletActivityResponse, error) {
	query := map[string]any{
		"chain":          chain,
		"wallet_address": wallet,
	}
	for k, v := range extra {
		query[k] = v
	}
	return request[WalletActivityResponse](c, http.MethodGet, "/v1/user/wallet_activity", query, nil, false)
}

func (c *Client) GetWalletStats(chain string, wallets []string, period string) ([]WalletStats, error) {
	query := map[string]any{
		"chain":          chain,
		"wallet_address": wallets,
		"period":         period,
	}
	resp, err := request[[]WalletStats](c, http.MethodGet, "/v1/user/wallet_stats", query, nil, false)
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

func (c *Client) GetWalletTokenBalance(chain, wallet, token string) (map[string]interface{}, error) {
	resp, err := request[map[string]interface{}](c, http.MethodGet, "/v1/user/wallet_token_balance", map[string]any{
		"chain":          chain,
		"wallet_address": wallet,
		"token_address":  token,
	}, nil, false)
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

func (c *Client) QuoteOrder(chain, wallet, inputToken, outputToken, inputAmount string, slippage float64) (*OrderReport, error) {
	resp, err := request[OrderReport](c, http.MethodGet, "/v1/trade/quote", map[string]any{
		"chain":        chain,
		"from_address": wallet,
		"input_token":  inputToken,
		"output_token": outputToken,
		"input_amount": inputAmount,
		"slippage":     slippage,
	}, nil, true)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) Swap(params SwapParams) (*OrderResponse, error) {
	return request[OrderResponse](c, http.MethodPost, "/v1/trade/swap", nil, params, true)
}

func (c *Client) QueryOrder(chain, orderID string) (*OrderResponse, error) {
	return request[OrderResponse](c, http.MethodGet, "/v1/trade/query_order", map[string]any{
		"chain":    chain,
		"order_id": orderID,
	}, nil, true)
}

func (c *Client) CreateStrategyOrder(params StrategyCreateParams) (*OrderResponse, error) {
	return request[OrderResponse](c, http.MethodPost, "/v1/trade/strategy/create", nil, params, true)
}

func (c *Client) GetStrategyOrders(chain string, extra map[string]any) (*StrategyOrdersResponse, error) {
	query := map[string]any{"chain": chain}
	for k, v := range extra {
		query[k] = v
	}
	return request[StrategyOrdersResponse](c, http.MethodGet, "/v1/trade/strategy/orders", query, nil, true)
}

func (c *Client) CancelStrategyOrder(params StrategyCancelParams) (*OrderResponse, error) {
	return request[OrderResponse](c, http.MethodPost, "/v1/trade/strategy/cancel", nil, params, true)
}

func request[T any](c *Client, method, path string, query map[string]any, body any, critical bool) (*T, error) {
	if c == nil {
		return nil, fmt.Errorf("gmgn client is nil")
	}
	if critical && c.signer.algorithm == "" {
		return nil, fmt.Errorf("gmgn private key is required for critical request %s", path)
	}

	queryParams := cloneQuery(query)
	timestamp := time.Now().Unix()
	queryParams["timestamp"] = timestamp
	queryParams["client_id"] = uuid.NewString()

	bodyBytes, err := marshalBody(body)
	if err != nil {
		return nil, err
	}
	bodyString := string(bodyBytes)

	u := c.host + path + "?" + encodeQuery(queryParams)
	req, err := http.NewRequest(method, u, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-APIKEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	if critical {
		signature, err := c.sign(path, queryParams, bodyString, timestamp)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Signature", signature)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gmgn request failed (%s %s): http %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	var env envelope[T]
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, fmt.Errorf("gmgn decode failed (%s %s): %w", method, path, err)
	}
	if !isSuccessCode(env.Code) {
		msg := strings.TrimSpace(env.Message)
		if msg == "" {
			msg = strings.TrimSpace(env.Error)
		}
		if msg == "" {
			msg = strings.TrimSpace(env.Reason)
		}
		if msg == "" {
			msg = fmt.Sprintf("gmgn api error code=%v", env.Code)
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return &env.Data, nil
}

func cloneQuery(query map[string]any) map[string]any {
	out := make(map[string]any, len(query)+2)
	for k, v := range query {
		out[k] = v
	}
	return out
}

func marshalBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	switch v := body.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return json.Marshal(v)
	}
}

func isSuccessCode(code any) bool {
	switch v := code.(type) {
	case float64:
		return int(v) == 0
	case int:
		return v == 0
	case int64:
		return v == 0
	case string:
		return v == "0"
	default:
		return false
	}
}

func encodeQuery(query map[string]any) string {
	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	values := url.Values{}
	for _, key := range keys {
		switch v := query[key].(type) {
		case []string:
			for _, item := range v {
				values.Add(key, item)
			}
		case []any:
			for _, item := range v {
				values.Add(key, fmt.Sprint(item))
			}
		default:
			values.Add(key, fmt.Sprint(v))
		}
	}
	return values.Encode()
}

func (c *Client) sign(path string, query map[string]any, body string, timestamp int64) (string, error) {
	message := buildMessage(path, query, body, timestamp)
	switch c.signer.algorithm {
	case "ed25519":
		return base64.StdEncoding.EncodeToString(ed25519.Sign(c.signer.ed25519, []byte(message))), nil
	case "rsa":
		sum := sha256.Sum256([]byte(message))
		signature, err := rsa.SignPSS(rand.Reader, c.signer.rsa, crypto.SHA256, sum[:], &rsa.PSSOptions{SaltLength: 32})
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(signature), nil
	default:
		return "", fmt.Errorf("unsupported gmgn signer algorithm")
	}
}

func buildMessage(path string, query map[string]any, body string, timestamp int64) string {
	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		switch v := query[key].(type) {
		case []string:
			for _, item := range v {
				parts = append(parts, key+"="+item)
			}
		default:
			parts = append(parts, key+"="+fmt.Sprint(v))
		}
	}
	return fmt.Sprintf("%s:%s:%s:%d", path, strings.Join(parts, "&"), body, timestamp)
}

func newSigner(privateKeyPEM string) (*signer, error) {
	pemBytes := []byte(strings.ReplaceAll(strings.TrimSpace(privateKeyPEM), `\n`, "\n"))
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode gmgn private key PEM")
	}

	if pkcs8Key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		switch key := pkcs8Key.(type) {
		case ed25519.PrivateKey:
			return &signer{algorithm: "ed25519", ed25519: key}, nil
		case *rsa.PrivateKey:
			return &signer{algorithm: "rsa", rsa: key}, nil
		default:
			return nil, fmt.Errorf("unsupported gmgn private key type %T", key)
		}
	}

	if rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return &signer{algorithm: "rsa", rsa: rsaKey}, nil
	}

	if edKey, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if parsed, ok := edKey.(ed25519.PrivateKey); ok {
			return &signer{algorithm: "ed25519", ed25519: parsed}, nil
		}
	}

	return nil, fmt.Errorf("failed to parse gmgn private key")
}

func ParseFloatString(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed
}

func ParseIntString(value string) int {
	parsed, _ := strconv.Atoi(strings.TrimSpace(value))
	return parsed
}

func RawAmountFromDecimal(amount float64, decimals int) string {
	if decimals < 0 {
		decimals = 0
	}
	scale := pow10(decimals)
	raw := amount * scale
	if raw < 0 {
		raw = 0
	}
	return strconv.FormatInt(int64(raw+0.0000001), 10)
}

func DecimalAmountFromRaw(raw string, decimals int) float64 {
	if decimals < 0 {
		decimals = 0
	}
	value := ParseFloatString(raw)
	scale := pow10(decimals)
	if scale == 0 {
		return value
	}
	return value / scale
}

func pow10(decimals int) float64 {
	result := 1.0
	for i := 0; i < decimals; i++ {
		result *= 10
	}
	return result
}
