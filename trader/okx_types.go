package trader

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

// OKXBalance OKX账户余额信息
type OKXBalance struct {
	TotalEquity string                 `json:"totalEq"` // 总资产
	IsoEquity   string                 `json:"isoEq"`   // 已用资产
	AdjEquity   string                 `json:"adjEq"`   // 可用资产
	OrdFrozen   string                 `json:"ordFroz"` // 挂单冻结
	MgnRatio    string                 `json:"mgnRatio"` // 保证金率
	UTime       string                 `json:"uTime"`   // 更新时间
	Details     []OKXBalanceDetail     `json:"details"` // 各币种详情
}

// OKXBalanceDetail 各币种余额详情
type OKXBalanceDetail struct {
	CCY       string `json:"ccy"`       // 币种
	Eq        string `json:"eq"`        // 权益
	CashBal   string `json:"cashBal"`   // 现金余额
	UTime     string `json:"uTime"`     // 更新时间
	IsoEq     string `json:"isoEq"`     // 已用权益
	AvailBal  string `json:"availBal"`  // 可用余额
	FrozenBal string `json:"frozenBal"` // 冻结余额
}

// OKXPosition OKX持仓信息
type OKXPosition struct {
	InstID     string `json:"instId"`     // 产品ID
	Pos        string `json:"pos"`        // 持仓数量
	PosSide    string `json:"posSide"`    // 持仓方向
	AvgPx      string `json:"avgPx"`      // 开仓均价
	MgnMode    string `json:"mgnMode"`    // 保证金模式
	MgnRatio   string `json:"mgnRatio"`   // 保证金率
	Lever      string `json:"lever"`      // 杠杆倍数
	LiqPx      string `json:"liqPx"`      // 强平价格
	UPnL       string `json:"upl"`        // 未实现盈亏
	UPnLRatio  string `json:"uplRatio"`   // 未实现盈亏率
	Last       string `json:"last"`       // 最新成交价
	Notional   string `json:"notional"`   // 持仓名义价值
	Ccy        string `json:"ccy"`        // 保证金币种
	UTime      string `json:"uTime"`      // 更新时间
	IMR        string `json:"imr"`        // 初始保证金要求
	MMR        string `json:"mmr"`        // 维持保证金要求
}

// OKXOrder OKX订单信息
type OKXOrder struct {
	InstID     string `json:"instId"`     // 产品ID
	OrdID      string `json:"ordId"`      // 订单ID
	ClOrdID    string `json:"clOrdId"`    // 客户自定义订单ID
	Tag        string `json:"tag"`        // 订单标签
	Px         string `json:"px"`         // 委托价格
	Sz         string `json:"sz"`         // 委托数量
	PxUsd      string `json:"pxUsd"`      // 委托价格(USD)
	PxVol      string `json:"pxVol"`      // 委托价格(波动率)
	OrdType    string `json:"ordType"`    // 订单类型
	Side       string `json:"side"`       // 订单方向
	PosSide    string `json:"posSide"`    // 持仓方向
	TdMode     string `json:"tdMode"`     // 交易模式
	AccFillSz  string `json:"accFillSz"`  // 累计成交数量
	FillPx     string `json:"fillPx"`     // 最新成交价格
	TradeID    string `json:"tradeId"`    // 最新成交ID
	FillSz     string `json:"fillSz"`     // 最新成交数量
	FillPxVol  string `json:"fillPxVol"`  // 最新成交价格(波动率)
	FillTime   string `json:"fillTime"`   // 最新成交时间
	FillPnl    string `json:"fillPnl"`    // 成交收益
	State      string `json:"state"`      // 订单状态
	AvgPx      string `json:"avgPx"`      // 成交均价
	Lever      string `json:"lever"`      // 杠杆倍数
	TPTriggerPx string `json:"tpTriggerPx"` // 止盈触发价格
	TPOrdPx    string `json:"tpOrdPx"`    // 止盈委托价格
	SLTriggerPx string `json:"slTriggerPx"` // 止损触发价格
	SLOrdPx    string `json:"slOrdPx"`    // 止损委托价格
	CTime      string `json:"cTime"`      // 创建时间
	UTime      string `json:"uTime"`      // 更新时间
	Fee        string `json:"fee"`        // 手续费
	Rebate     string `json:"rebate"`     // 返佣
	Pnl        string `json:"pnl"`        // 收益
	Category   string `json:"category"`   // 订单种类
}

// OKXTicker OKX行情数据
type OKXTicker struct {
	InstID  string `json:"instId"`  // 产品ID
	Last    string `json:"last"`    // 最新成交价
	LastSz  string `json:"lastSz"`  // 最新成交数量
	AskPx   string `json:"askPx"`   // 卖一价
	AskSz   string `json:"askSz"`   // 卖一量
	BidPx   string `json:"bidPx"`   // 买一价
	BidSz   string `json:"bidSz"`   // 买一量
	Open24h string `json:"open24h"` // 24小时开盘价
	High24h string `json:"high24h"` // 24小时最高价
	Low24h  string `json:"low24h"`  // 24小时最低价
	Vol24h  string `json:"vol24h"`  // 24小时成交量
	VolCcy24h string `json:"volCcy24h"` // 24小时成交额
	Ts      string `json:"ts"`      // 时间戳
	SodUtc0 string `json:"sodUtc0"` // UTC 0点开盘价
	SodUtc8 string `json:"sodUtc8"` // UTC 8点开盘价
}

// OKXOrderRequest OKX下单请求
type OKXOrderRequest struct {
	InstID      string `json:"instId"`      // 产品ID
	TdMode      string `json:"tdMode"`      // 交易模式
	Side        string `json:"side"`        // 订单方向
	PosSide     string `json:"posSide,omitempty"` // 持仓方向
	OrdType     string `json:"ordType"`     // 订单类型
	Sz          string `json:"sz"`          // 委托数量
	Px          string `json:"px,omitempty"`      // 委托价格
	SlTriggerPx string `json:"slTriggerPx,omitempty"` // 止损触发价格
	SlOrdPx     string `json:"slOrdPx,omitempty"`     // 止损委托价格
	TpTriggerPx string `json:"tpTriggerPx,omitempty"` // 止盈触发价格
	TpOrdPx     string `json:"tpOrdPx,omitempty"`     // 止盈委托价格
	TriggerPx   string `json:"triggerPx,omitempty"`   // 触发价格
	OrderPx     string `json:"orderPx,omitempty"`     // 委托价格
	Tag         string `json:"tag,omitempty"`         // 订单标签
	ClOrdID     string `json:"clOrdId,omitempty"`     // 客户自定义订单ID
}

// OKXOrderResponse OKX下单响应
type OKXOrderResponse struct {
	OrdID   string `json:"ordId"`   // 订单ID
	ClOrdID string `json:"clOrdId"` // 客户自定义订单ID
	Tag     string `json:"tag"`     // 订单标签
	SCode   string `json:"sCode"`   // 事件执行结果的code
	SMsg    string `json:"sMsg"`    // 事件执行结果的msg
}

// OKXAPIResponse OKX API通用响应
type OKXAPIResponse struct {
	Code string      `json:"code"` // 响应代码
	Msg  string      `json:"msg"`  // 响应消息
	Data interface{} `json:"data"` // 响应数据
}

// OKXLeverageRequest OKX杠杆设置请求
type OKXLeverageRequest struct {
	InstID  string `json:"instId"`  // 产品ID
	Ccy     string `json:"ccy,omitempty"` // 保证金币种
	Lever   string `json:"lever"`   // 杠杆倍数
	MgnMode string `json:"mgnMode"` // 保证金模式
	PosSide string `json:"posSide,omitempty"` // 持仓方向
}

// OKXMarginModeRequest OKX保证金模式设置请求
type OKXMarginModeRequest struct {
	InstID  string `json:"instId"`  // 产品ID
	MgnMode string `json:"mgnMode"` // 保证金模式
}

// OKXCancelAllRequest OKX取消所有订单请求
type OKXCancelAllRequest struct {
	InstID string `json:"instId,omitempty"` // 产品ID
	OrdType string `json:"ordType,omitempty"` // 订单类型
}

// OKXPublicRequest OKX公共请求参数
type OKXPublicRequest struct {
	InstID string `json:"instId,omitempty"` // 产品ID
}

// OKXAuthHeaders OKX认证头信息
type OKXAuthHeaders struct {
	APIKey     string `json:"OK-ACCESS-KEY"`      // API密钥
	Signature  string `json:"OK-ACCESS-SIGN"`     // 签名
	Timestamp  string `json:"OK-ACCESS-TIMESTAMP"` // 时间戳
	Passphrase string `json:"OK-ACCESS-PASSPHRASE"` // 密码短语
}

// OKXCredentials OKX凭证信息
type OKXCredentials struct {
	APIKey     string `json:"api_key"`     // API密钥
	SecretKey  string `json:"secret_key"`  // 密钥
	Passphrase string `json:"passphrase"`  // 密码短语
	Testnet    bool   `json:"testnet"`     // 是否使用测试网络
}

// OKXConfig OKX配置信息
type OKXConfig struct {
	BaseURL            string        `json:"base_url"`            // 基础URL
	Timeout            time.Duration `json:"timeout"`             // 超时时间
	CacheDuration      time.Duration `json:"cache_duration"`      // 缓存时间
	MaxRetries         int           `json:"max_retries"`         // 最大重试次数
	RateLimitPerSecond int           `json:"rate_limit_per_second"` // 每秒请求限制
	EnableTestnet      bool          `json:"enable_testnet"`      // 启用测试网络
}

// DefaultOKXConfig 默认OKX配置
var DefaultOKXConfig = OKXConfig{
	BaseURL:            "https://www.okx.com",
	Timeout:            30 * time.Second,
	CacheDuration:      15 * time.Second,
	MaxRetries:         3,
	RateLimitPerSecond: 10,
	EnableTestnet:      false,
}

// OKXRateLimitInfo OKX速率限制信息
type OKXRateLimitInfo struct {
	Limit     int           `json:"limit"`     // 限制
	Remaining int           `json:"remaining"` // 剩余
	Reset     time.Duration `json:"reset"`     // 重置时间
}

// OKXMarketInfo OKX市场信息
type OKXMarketInfo struct {
	InstID      string `json:"instId"`      // 产品ID
	BaseCcy     string `json:"baseCcy"`     // 基础货币
	QuoteCcy    string `json:"quoteCcy"`    // 报价货币
	InstType    string `json:"instType"`    // 产品类型
	State       string `json:"state"`       // 产品状态
	LotSz       string `json:"lotSz"`       // 最小下单数量
	MinSz       string `json:"minSz"`       // 最小下单数量
	MaxSz       string `json:"maxSz"`       // 最大下单数量
	TickSz      string `json:"tickSz"`      // 最小价格精度
	PxPrecision string `json:"pxPrecision"` // 价格精度
}

// OKXAccountInfo OKX账户信息
type OKXAccountInfo struct {
	UID        string `json:"uid"`        // 用户ID
	AcctLv     string `json:"acctLv"`     // 账户等级
	PosMode    string `json:"posMode"`    // 持仓模式
	AutoLoan   bool   `json:"autoLoan"`   // 是否自动借币
	GreedyMode bool   `json:"greedyMode"` // 是否贪婪模式
}

// OKXPositionMode OKX持仓模式
type OKXPositionMode struct {
	PosMode string `json:"posMode"` // 持仓模式
	InstID  string `json:"instId"`  // 产品ID
}

// OKXTradeFee OKX交易手续费
type OKXTradeFee struct {
	InstType  string `json:"instType"`  // 产品类型
	InstID    string `json:"instId"`    // 产品ID
	Taker     string `json:"taker"`     // 吃单手续费率
	Maker     string `json:"maker"`     // 挂单手续费率
	TakerUA   string `json:"takerUa"`   // 吃单手续费率(UA)
	MakerUA   string `json:"makerUa"`   // 挂单手续费率(UA)
	Delivery  string `json:"delivery"`  // 交割手续费率
	Exercise  string `json:"exercise"`  // 行权手续费率
	Ts        string `json:"ts"`        // 时间戳
}

// OKXWebSocketMessage OKX WebSocket消息
type OKXWebSocketMessage struct {
	Op   string      `json:"op"`   // 操作类型
	Args []OKXArg    `json:"args"` // 参数列表
}

// OKXArg OKX WebSocket参数
type OKXArg struct {
	Channel string `json:"channel"` // 频道
	InstID  string `json:"instId"`  // 产品ID
}

// OKXWebSocketResponse OKX WebSocket响应
type OKXWebSocketResponse struct {
	Event string          `json:"event"` // 事件类型
	Code  string          `json:"code"`  // 错误码
	Msg   string          `json:"msg"`   // 错误消息
	Arg   *OKXArg         `json:"arg"`   // 参数
	Data  interface{}     `json:"data"`  // 数据
}

// OKXConstants OKX常量定义
type OKXConstants struct {
	MaxLeverage    int           `json:"max_leverage"`    // 最大杠杆
	MinLeverage    int           `json:"min_leverage"`    // 最小杠杆
	MaxOrderSize   float64       `json:"max_order_size"`  // 最大订单数量
	MinOrderSize   float64       `json:"min_order_size"`  // 最小订单数量
	MaxPrice       float64       `json:"max_price"`       // 最大价格
	MinPrice       float64       `json:"min_price"`       // 最小价格
	SupportedPairs []string      `json:"supported_pairs"` // 支持的交易对
	MaxRetries     int           `json:"max_retries"`     // 最大重试次数
	Timeout        time.Duration `json:"timeout"`         // 超时时间
}

// DefaultOKXConstants 默认OKX常量
var DefaultOKXConstants = OKXConstants{
	MaxLeverage:    125,
	MinLeverage:    1,
	MaxOrderSize:   1000000,
	MinOrderSize:   0.001,
	MaxPrice:       10000000,
	MinPrice:       0.000001,
	SupportedPairs: []string{
		"BTC-USDT-SWAP",
		"ETH-USDT-SWAP",
		"SOL-USDT-SWAP",
		"BNB-USDT-SWAP",
		"XRP-USDT-SWAP",
		"DOGE-USDT-SWAP",
		"ADA-USDT-SWAP",
		"MATIC-USDT-SWAP",
		"DOT-USDT-SWAP",
		"AVAX-USDT-SWAP",
	},
	MaxRetries: 3,
	Timeout:    30 * time.Second,
}

// OKXOrderState OKX订单状态
type OKXOrderState string

const (
	OKXOrderStateLive   OKXOrderState = "live"   // 待成交
	OKXOrderStateFilled OKXOrderState = "filled" // 完全成交
	OKXOrderStateCanceled OKXOrderState = "canceled" // 已撤销
	OKXOrderStatePartiallyFilled OKXOrderState = "partially_filled" // 部分成交
)

// OKXPositionSide OKX持仓方向
type OKXPositionSide string

const (
	OKXPositionSideLong  OKXPositionSide = "long"  // 多头
	OKXPositionSideShort OKXPositionSide = "short" // 空头
	OKXPositionSideNet   OKXPositionSide = "net"   // 净持仓
)

// OKXMarginMode OKX保证金模式
type OKXMarginMode string

const (
	OKXMarginModeIsolated OKXMarginMode = "isolated" // 逐仓
	OKXMarginModeCross    OKXMarginMode = "cross"    // 全仓
)

// OKXOrderType OKX订单类型
type OKXOrderType string

const (
	OKXOrderTypeMarket     OKXOrderType = "market"     // 市价
	OKXOrderTypeLimit      OKXOrderType = "limit"      // 限价
	OKXOrderTypePostOnly   OKXOrderType = "post_only"  // 只做maker
	OKXOrderTypeIOC        OKXOrderType = "ioc"        // 立即成交或取消
	OKXOrderTypeFOK        OKXOrderType = "fok"        // 全部成交或取消
	OKXOrderTypeConditional OKXOrderType = "conditional" // 条件单
)

// OKXOrderSide OKX订单方向
type OKXOrderSide string

const (
	OKXOrderSideBuy  OKXOrderSide = "buy"  // 买入
	OKXOrderSideSell OKXOrderSide = "sell" // 卖出
)

// OKXInstrumentType OKX产品类型
type OKXInstrumentType string

const (
	OKXInstrumentTypeSpot    OKXInstrumentType = "SPOT"    // 现货
	OKXInstrumentTypeSwap    OKXInstrumentType = "SWAP"    // 永续合约
	OKXInstrumentTypeFutures OKXInstrumentType = "FUTURES" // 交割合约
	OKXInstrumentTypeOption  OKXInstrumentType = "OPTION"  // 期权
)

// OKXPublicChannel OKX公共频道
type OKXPublicChannel string

const (
	OKXPublicChannelTickers   OKXPublicChannel = "tickers"   // 行情频道
	OKXPublicChannelCandle1m  OKXPublicChannel = "candle1m"  // 1分钟K线
	OKXPublicChannelCandle5m  OKXPublicChannel = "candle5m"  // 5分钟K线
	OKXPublicChannelCandle1H  OKXPublicChannel = "candle1H"  // 1小时K线
	OKXPublicChannelCandle1D  OKXPublicChannel = "candle1D"  // 1日K线
	OKXPublicChannelTrades    OKXPublicChannel = "trades"    // 交易频道
	OKXPublicChannelBooks      OKXPublicChannel = "books"     // 深度频道
	OKXPublicChannelBooks5     OKXPublicChannel = "books5"    // 5档深度
	OKXPublicChannelBooks50    OKXPublicChannel = "books50"   // 50档深度
)

// OKXPrivateChannel OKX私有频道
type OKXPrivateChannel string

const (
	OKXPrivateChannelAccount      OKXPrivateChannel = "account"       // 账户频道
	OKXPrivateChannelPositions    OKXPrivateChannel = "positions"     // 持仓频道
	OKXPrivateChannelBalance      OKXPrivateChannel = "balance"       // 余额频道
	OKXPrivateChannelOrders       OKXPrivateChannel = "orders"        // 订单频道
	OKXPrivateChannelOrdersAlgo   OKXPrivateChannel = "orders-algo"   // 策略订单频道
)

// OKXRateLimitType OKX速率限制类型
type OKXRateLimitType string

const (
	OKXRateLimitTypeRequest OKXRateLimitType = "request" // 请求速率限制
	OKXRateLimitTypeOrder   OKXRateLimitType = "order"   // 订单速率限制
)

// OKXTimeInForce OKX有效时间
type OKXTimeInForce string

const (
	OKXTimeInForceGTC OKXTimeInForce = "gtc" // 成交为止
	OKXTimeInForceIOC OKXTimeInForce = "ioc" // 立即成交或取消
	OKXTimeInForceFOK OKXTimeInForce = "fok" // 全部成交或取消
)

// OKXResponseCode OKX响应代码
type OKXResponseCode string

const (
	OKXResponseCodeSuccess      OKXResponseCode = "0" // 成功
	OKXResponseCodeFailure      OKXResponseCode = "1" // 失败
	OKXResponseCodePartialSuccess OKXResponseCode = "2" // 部分成功
)

// GetOrderStateDescription 获取订单状态描述
func GetOrderStateDescription(state OKXOrderState) string {
	descriptions := map[OKXOrderState]string{
		OKXOrderStateLive:             "待成交",
		OKXOrderStateFilled:           "完全成交",
		OKXOrderStateCanceled:         "已撤销",
		OKXOrderStatePartiallyFilled:  "部分成交",
	}

	if desc, exists := descriptions[state]; exists {
		return desc
	}
	return "未知状态"
}

// GetPositionSideDescription 获取持仓方向描述
func GetPositionSideDescription(side OKXPositionSide) string {
	descriptions := map[OKXPositionSide]string{
		OKXPositionSideLong:  "多头",
		OKXPositionSideShort: "空头",
		OKXPositionSideNet:   "净持仓",
	}

	if desc, exists := descriptions[side]; exists {
		return desc
	}
	return "未知方向"
}

// GetMarginModeDescription 获取保证金模式描述
func GetMarginModeDescription(mode OKXMarginMode) string {
	descriptions := map[OKXMarginMode]string{
		OKXMarginModeIsolated: "逐仓",
		OKXMarginModeCross:    "全仓",
	}

	if desc, exists := descriptions[mode]; exists {
		return desc
	}
	return "未知模式"
}

// GetOrderTypeDescription 获取订单类型描述
func GetOrderTypeDescription(orderType OKXOrderType) string {
	descriptions := map[OKXOrderType]string{
		OKXOrderTypeMarket:     "市价",
		OKXOrderTypeLimit:      "限价",
		OKXOrderTypePostOnly:   "只做maker",
		OKXOrderTypeIOC:        "立即成交或取消",
		OKXOrderTypeFOK:        "全部成交或取消",
		OKXOrderTypeConditional: "条件单",
	}

	if desc, exists := descriptions[orderType]; exists {
		return desc
	}
	return "未知类型"
}

// GetOrderSideDescription 获取订单方向描述
func GetOrderSideDescription(side OKXOrderSide) string {
	descriptions := map[OKXOrderSide]string{
		OKXOrderSideBuy:  "买入",
		OKXOrderSideSell: "卖出",
	}

	if desc, exists := descriptions[side]; exists {
		return desc
	}
	return "未知方向"
}

// GetInstrumentTypeDescription 获取产品类型描述
func GetInstrumentTypeDescription(instType OKXInstrumentType) string {
	descriptions := map[OKXInstrumentType]string{
		OKXInstrumentTypeSpot:    "现货",
		OKXInstrumentTypeSwap:    "永续合约",
		OKXInstrumentTypeFutures: "交割合约",
		OKXInstrumentTypeOption:  "期权",
	}

	if desc, exists := descriptions[instType]; exists {
		return desc
	}
	return "未知类型"
}

// ConvertToStandardPosition 转换为标准持仓格式
func ConvertToStandardPosition(okxPos OKXPosition) map[string]interface{} {
	return map[string]interface{}{
		"symbol":     okxPos.InstID,
		"position":   okxPos.Pos,
		"posSide":    okxPos.PosSide,
		"avgPrice":   okxPos.AvgPx,
		"leverage":   okxPos.Lever,
		"marginMode": okxPos.MgnMode,
		"liquidationPrice": okxPos.LiqPx,
		"unrealizedPnl":    okxPos.UPnL,
		"unrealizedPnlRatio": okxPos.UPnLRatio,
		"lastPrice":        okxPos.Last,
		"notionalValue":    okxPos.Notional,
		"marginCurrency":   okxPos.Ccy,
		"updateTime":       okxPos.UTime,
		"initialMarginRequired": okxPos.IMR,
		"maintenanceMarginRequired": okxPos.MMR,
	}
}

// ConvertToStandardBalance 转换为标准余额格式
func ConvertToStandardBalance(okxBalance OKXBalance) map[string]interface{} {
	return map[string]interface{}{
		"total": okxBalance.TotalEquity,
		"used":  okxBalance.IsoEquity,
		"free":  okxBalance.AdjEquity,
		"updateTime": okxBalance.UTime,
		"details": okxBalance.Details,
	}
}

// ConvertToStandardOrder 转换为标准订单格式
func ConvertToStandardOrder(okxOrder OKXOrder) map[string]interface{} {
	return map[string]interface{}{
		"orderId":     okxOrder.OrdID,
		"clientOrderId": okxOrder.ClOrdID,
		"symbol":      okxOrder.InstID,
		"price":       okxOrder.Px,
		"quantity":    okxOrder.Sz,
		"orderType":   okxOrder.OrdType,
		"side":        okxOrder.Side,
		"positionSide": okxOrder.PosSide,
		"state":       okxOrder.State,
		"filledQuantity": okxOrder.AccFillSz,
		"averagePrice": okxOrder.AvgPx,
		"leverage":    okxOrder.Lever,
		"createTime":  okxOrder.CTime,
		"updateTime":  okxOrder.UTime,
		"fee":         okxOrder.Fee,
		"profitLoss":  okxOrder.Pnl,
	}
}

// ConvertToStandardTicker 转换为标准行情格式
func ConvertToStandardTicker(okxTicker OKXTicker) map[string]interface{} {
	return map[string]interface{}{
		"symbol":    okxTicker.InstID,
		"lastPrice": okxTicker.Last,
		"lastSize":  okxTicker.LastSz,
		"askPrice":  okxTicker.AskPx,
		"askSize":   okxTicker.AskSz,
		"bidPrice":  okxTicker.BidPx,
		"bidSize":   okxTicker.BidSz,
		"open24h":   okxTicker.Open24h,
		"high24h":   okxTicker.High24h,
		"low24h":    okxTicker.Low24h,
		"volume24h": okxTicker.Vol24h,
		"value24h":  okxTicker.VolCcy24h,
		"timestamp": okxTicker.Ts,
	}
}

// IsValidOKXSymbol 验证是否为有效的OKX交易对
func IsValidOKXSymbol(symbol string) bool {
	// 标准格式: BASE-QUOTE-SWAP
	parts := strings.Split(symbol, "-")
	if len(parts) != 3 {
		return false
	}

	base := parts[0]
	quote := parts[1]
	suffix := parts[2]

	// 检查基础货币
	if base == "" || len(base) < 2 || len(base) > 10 {
		return false
	}

	// 检查报价货币
	if quote == "" || len(quote) < 2 || len(quote) > 10 {
		return false
	}

	// 检查后缀
	if suffix != "SWAP" {
		return false
	}

	return true
}

// GetDefaultSymbols 获取默认支持的交易对
func GetDefaultSymbols() []string {
	return []string{
		"BTC-USDT-SWAP",
		"ETH-USDT-SWAP",
		"SOL-USDT-SWAP",
		"BNB-USDT-SWAP",
		"XRP-USDT-SWAP",
		"DOGE-USDT-SWAP",
		"ADA-USDT-SWAP",
		"MATIC-USDT-SWAP",
		"DOT-USDT-SWAP",
		"AVAX-USDT-SWAP",
	}
}

// GetSymbolPrecision 获取交易对精度信息
func GetSymbolPrecision(symbol string) (quantityPrecision int, pricePrecision int) {
	// 默认精度
	quantityPrecision = 3
	pricePrecision = 2

	// 根据交易对设置特定精度
	base := strings.Split(symbol, "-")[0]

	switch base {
	case "BTC":
		quantityPrecision = 3
		pricePrecision = 2
	case "ETH":
		quantityPrecision = 3
		pricePrecision = 2
	case "SOL":
		quantityPrecision = 2
		pricePrecision = 3
	case "BNB":
		quantityPrecision = 2
		pricePrecision = 2
	case "XRP":
		quantityPrecision = 0
		pricePrecision = 4
	case "DOGE":
		quantityPrecision = 0
		pricePrecision = 5
	case "ADA":
		quantityPrecision = 0
		pricePrecision = 4
	case "MATIC":
		quantityPrecision = 1
		pricePrecision = 4
	case "DOT":
		quantityPrecision = 2
		pricePrecision = 3
	case "AVAX":
		quantityPrecision = 2
		pricePrecision = 3
	default:
		// 默认精度
		quantityPrecision = 4
		pricePrecision = 4
	}

	return quantityPrecision, pricePrecision
}

// FormatQuantityWithPrecision 根据精度格式化数量
func FormatQuantityWithPrecision(quantity float64, precision int) string {
	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, quantity)
}

// FormatPriceWithPrecision 根据精度格式化价格
func FormatPriceWithPrecision(price float64, precision int) string {
	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, price)
}

// ParseOKXNumber 解析OKX数字字符串
func ParseOKXNumber(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}

// ParseOKXTime 解析OKX时间字符串
func ParseOKXTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("时间字符串为空")
	}
	// OKX时间格式通常是毫秒时间戳
	timestamp, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	// 转换为秒
	if timestamp > 9999999999 {
		// 毫秒时间戳
		return time.Unix(timestamp/1000, (timestamp%1000)*int64(time.Millisecond)), nil
	}
	// 秒时间戳
	return time.Unix(timestamp, 0), nil
}

// OKXTimeToUnix 转换OKX时间到Unix时间戳
func OKXTimeToUnix(okxTime string) (int64, error) {
	t, err := ParseOKXTime(okxTime)
	if err != nil {
		return 0, err
	}
	return t.Unix(), nil
}

// GetCurrentOKXTimestamp 获取当前OKX格式时间戳
func GetCurrentOKXTimestamp() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// parseOKXFloat 解析OKX浮点数字符串
func parseOKXFloat(s string) float64 {
	if s == "" {
		return 0
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return val
}

// parseOKXTimestamp 解析OKX时间戳字符串
func parseOKXTimestamp(s string) int64 {
	if s == "" {
		return 0
	}
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return val
}

// parseOKXString 解析OKX字符串值，处理空值情况
func parseOKXString(s interface{}) string {
	if s == nil {
		return ""
	}
	str, ok := s.(string)
	if !ok {
		return ""
	}
	return str
}

// GenerateOKXClOrdID 生成OKX客户订单ID
func GenerateOKXClOrdID() string {
	return fmt.Sprintf("monnaire_%d", time.Now().UnixNano())
}

// ValidateOKXCredentials 验证OKX凭证
func ValidateOKXCredentials(apiKey, secretKey, passphrase string) error {
	if apiKey == "" {
		return fmt.Errorf("API密钥不能为空")
	}
	if secretKey == "" {
		return fmt.Errorf("Secret密钥不能为空")
	}
	if passphrase == "" {
		return fmt.Errorf("Passphrase不能为空")
	}

	// 长度验证
	if len(apiKey) < 10 {
		return fmt.Errorf("API密钥长度不能少于10个字符")
	}
	if len(secretKey) < 20 {
		return fmt.Errorf("Secret密钥长度不能少于20个字符")
	}
	if len(passphrase) < 6 {
		return fmt.Errorf("Passphrase长度不能少于6个字符")
	}

	return nil
}

// OKXConverter OKX数据转换器
type OKXConverter struct{}

// NewOKXConverter 创建OKX数据转换器
func NewOKXConverter() *OKXConverter {
	return &OKXConverter{}
}

// ConvertBalance 转换余额数据
func (c *OKXConverter) ConvertBalance(data interface{}) (map[string]interface{}, error) {
	if data == nil {
		return nil, fmt.Errorf("余额数据为空")
	}

	resp, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无效的余额数据格式")
	}

	code, _ := resp["code"].(string)
	if code != "0" {
		msg, _ := resp["msg"].(string)
		return nil, fmt.Errorf("获取余额失败: %s", msg)
	}

	balanceData, ok := resp["data"].([]interface{})
	if !ok || len(balanceData) == 0 {
		return map[string]interface{}{
			"total": float64(0),
			"used":  float64(0),
			"free":  float64(0),
		}, nil
	}

	balance, ok := balanceData[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无效的余额数据结构")
	}

	result := map[string]interface{}{
		"total": float64(0),
		"used":  float64(0),
		"free":  float64(0),
	}

	// 解析总资产
	if totalEq, ok := balance["totalEq"].(string); ok {
		if total, err := strconv.ParseFloat(totalEq, 64); err == nil {
			result["total"] = total
		}
	}

	// 解析已用资产
	if isoEq, ok := balance["isoEq"].(string); ok {
		if used, err := strconv.ParseFloat(isoEq, 64); err == nil {
			result["used"] = used
		}
	}

	// 解析可用资产
	if adjEq, ok := balance["adjEq"].(string); ok {
		if free, err := strconv.ParseFloat(adjEq, 64); err == nil {
			result["free"] = free
		}
	}

	return result, nil
}

// ConvertPositions 转换持仓数据
func (c *OKXConverter) ConvertPositions(data interface{}) ([]map[string]interface{}, error) {
	if data == nil {
		return nil, fmt.Errorf("持仓数据为空")
	}

	resp, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无效的持仓数据格式")
	}

	code, _ := resp["code"].(string)
	if code != "0" {
		msg, _ := resp["msg"].(string)
		return nil, fmt.Errorf("获取持仓失败: %s", msg)
	}

	positionsData, ok := resp["data"].([]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	var positions []map[string]interface{}
	for _, item := range positionsData {
		pos, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		position := map[string]interface{}{
			"symbol":    getStringValue(pos, "instId"),
			"position":  getStringValue(pos, "pos"),
			"posSide":   getStringValue(pos, "posSide"),
			"avgPrice":  getStringValue(pos, "avgPx"),
			"leverage":  getStringValue(pos, "lever"),
			"marginMode": getStringValue(pos, "mgnMode"),
			"liquidationPrice": getStringValue(pos, "liqPx"),
			"unrealizedPnl":    getStringValue(pos, "upl"),
			"unrealizedPnlRatio": getStringValue(pos, "uplRatio"),
			"lastPrice":        getStringValue(pos, "last"),
			"notionalValue":    getStringValue(pos, "notional"),
			"marginCurrency":   getStringValue(pos, "ccy"),
			"updateTime":       getStringValue(pos, "uTime"),
			"initialMarginRequired": getStringValue(pos, "imr"),
			"maintenanceMarginRequired": getStringValue(pos, "mmr"),
		}

		positions = append(positions, position)
	}

	return positions, nil
}

// ConvertTicker 转换行情数据
func (c *OKXConverter) ConvertTicker(data interface{}) (map[string]interface{}, error) {
	if data == nil {
		return nil, fmt.Errorf("行情数据为空")
	}

	resp, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无效的行情数据格式")
	}

	code, _ := resp["code"].(string)
	if code != "0" {
		msg, _ := resp["msg"].(string)
		return nil, fmt.Errorf("获取行情失败: %s", msg)
	}

	tickerData, ok := resp["data"].([]interface{})
	if !ok || len(tickerData) == 0 {
		return nil, fmt.Errorf("无效的行情数据结构")
	}

	ticker, ok := tickerData[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无效的行情数据结构")
	}

	result := map[string]interface{}{
		"symbol":    getStringValue(ticker, "instId"),
		"lastPrice": getStringValue(ticker, "last"),
		"lastSize":  getStringValue(ticker, "lastSz"),
		"askPrice":  getStringValue(ticker, "askPx"),
		"askSize":   getStringValue(ticker, "askSz"),
		"bidPrice":  getStringValue(ticker, "bidPx"),
		"bidSize":   getStringValue(ticker, "bidSz"),
		"open24h":   getStringValue(ticker, "open24h"),
		"high24h":   getStringValue(ticker, "high24h"),
		"low24h":    getStringValue(ticker, "low24h"),
		"volume24h": getStringValue(ticker, "vol24h"),
		"value24h":  getStringValue(ticker, "volCcy24h"),
		"timestamp": getStringValue(ticker, "ts"),
	}

	return result, nil
}

// getStringValue 安全获取字符串值
func getStringValue(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// ConvertOrderResponse 转换订单响应
func (c *OKXConverter) ConvertOrderResponse(data interface{}) (map[string]interface{}, error) {
	if data == nil {
		return nil, fmt.Errorf("订单响应数据为空")
	}

	resp, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无效的订单响应格式")
	}

	code, _ := resp["code"].(string)
	if code != "0" {
		msg, _ := resp["msg"].(string)
		return nil, fmt.Errorf("下单失败: %s", msg)
	}

	orderData, ok := resp["data"].([]interface{})
	if !ok || len(orderData) == 0 {
		return nil, fmt.Errorf("无效的订单响应数据")
	}

	order, ok := orderData[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无效的订单数据结构")
	}

	result := map[string]interface{}{
		"orderId":     getStringValue(order, "ordId"),
		"clientOrderId": getStringValue(order, "clOrdId"),
		"tag":         getStringValue(order, "tag"),
		"successCode": getStringValue(order, "sCode"),
		"successMsg":  getStringValue(order, "sMsg"),
	}

	return result, nil
}

// ConvertLeverageResponse 转换杠杆设置响应
func (c *OKXConverter) ConvertLeverageResponse(data interface{}) error {
	if data == nil {
		return fmt.Errorf("杠杆响应数据为空")
	}

	resp, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("无效的杠杆响应格式")
	}

	code, _ := resp["code"].(string)
	if code != "0" {
		msg, _ := resp["msg"].(string)
		return fmt.Errorf("设置杠杆失败: %s", msg)
	}

	return nil
}

// ConvertCancelAllResponse 转换取消所有订单响应
func (c *OKXConverter) ConvertCancelAllResponse(data interface{}) error {
	if data == nil {
		return fmt.Errorf("取消订单响应数据为空")
	}

	resp, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("无效的取消订单响应格式")
	}

	code, _ := resp["code"].(string)
	if code != "0" {
		msg, _ := resp["msg"].(string)
		return fmt.Errorf("取消订单失败: %s", msg)
	}

	return nil
}

// OKXWebSocketSubscribeRequest OKX WebSocket订阅请求
type OKXWebSocketSubscribeRequest struct {
	Op   string        `json:"op"`   // 操作类型: subscribe/unsubscribe
	Args []WebSocketArg `json:"args"` // 参数列表
}

// WebSocketArg WebSocket参数
type WebSocketArg struct {
	Channel string `json:"channel"` // 频道
	InstID  string `json:"instId"`  // 产品ID
}

// OKXWebSocketEvent OKX WebSocket事件
type OKXWebSocketEvent struct {
	Event string      `json:"event"` // 事件类型: subscribe/unsubscribe/error
	Code  string      `json:"code"`  // 错误码
	Msg   string      `json:"msg"`   // 错误消息
	Arg   *WebSocketArg `json:"arg"`   // 参数
}

// OKXWebSocketTickerData OKX WebSocket行情数据
type OKXWebSocketTickerData struct {
	InstID  string `json:"instId"`  // 产品ID
	Last    string `json:"last"`    // 最新成交价
	LastSz  string `json:"lastSz"`  // 最新成交数量
	AskPx   string `json:"askPx"`   // 卖一价
	AskSz   string `json:"askSz"`   // 卖一量
	BidPx   string `json:"bidPx"`   // 买一价
	BidSz   string `json:"bidSz"`   // 买一量
	Open24h string `json:"open24h"` // 24小时开盘价
	High24h string `json:"high24h"` // 24小时最高价
	Low24h  string `json:"low24h"`  // 24小时最低价
	Vol24h  string `json:"vol24h"`  // 24小时成交量
	VolCcy24h string `json:"volCcy24h"` // 24小时成交额
	Ts      string `json:"ts"`      // 时间戳
}

// OKXWebSocketPositionData OKX WebSocket持仓数据
type OKXWebSocketPositionData struct {
	InstID    string `json:"instId"`    // 产品ID
	Pos       string `json:"pos"`       // 持仓数量
	PosSide   string `json:"posSide"`   // 持仓方向
	AvgPx     string `json:"avgPx"`     // 开仓均价
	MgnMode   string `json:"mgnMode"`   // 保证金模式
	MgnRatio  string `json:"mgnRatio"`  // 保证金率
	Lever     string `json:"lever"`     // 杠杆倍数
	LiqPx     string `json:"liqPx"`     // 强平价格
	UPnL      string `json:"upl"`       // 未实现盈亏
	UPnLRatio string `json:"uplRatio"`  // 未实现盈亏率
	Last      string `json:"last"`      // 最新成交价
	Notional  string `json:"notional"`  // 持仓名义价值
	Ccy       string `json:"ccy"`       // 保证金币种
	Ts        string `json:"ts"`        // 时间戳
}

// OKXWebSocketOrderData OKX WebSocket订单数据
type OKXWebSocketOrderData struct {
	InstID    string `json:"instId"`    // 产品ID
	OrdID     string `json:"ordId"`     // 订单ID
	ClOrdID   string `json:"clOrdId"`   // 客户自定义订单ID
	Tag       string `json:"tag"`       // 订单标签
	Px        string `json:"px"`        // 委托价格
	Sz        string `json:"sz"`        // 委托数量
	OrdType   string `json:"ordType"`   // 订单类型
	Side      string `json:"side"`      // 订单方向
	PosSide   string `json:"posSide"`   // 持仓方向
	State     string `json:"state"`     // 订单状态
	AvgPx     string `json:"avgPx"`     // 成交均价
	AccFillSz string `json:"accFillSz"` // 累计成交数量
	FillPx    string `json:"fillPx"`    // 最新成交价格
	FillSz    string `json:"fillSz"`    // 最新成交数量
	TradeID   string `json:"tradeId"`   // 最新成交ID
	FillTime  string `json:"fillTime"`  // 最新成交时间
	UTime     string `json:"uTime"`     // 更新时间
	Fee       string `json:"fee"`       // 手续费
	Rebate    string `json:"rebate"`    // 返佣
	Pnl       string `json:"pnl"`       // 收益
	Category  string `json:"category"`  // 订单种类
}

// OKXWebSocketBalanceData OKX WebSocket余额数据
type OKXWebSocketBalanceData struct {
	Ccy       string `json:"ccy"`       // 币种
	CashBal   string `json:"cashBal"`   // 现金余额
	UTime     string `json:"uTime"`     // 更新时间
	AvailBal  string `json:"availBal"`  // 可用余额
	FrozenBal string `json:"frozenBal"` // 冻结余额
	Eq        string `json:"eq"`        // 权益
	IsoEq     string `json:"isoEq"`     // 已用权益
}

// OKXRestResponse OKX REST API响应包装
type OKXRestResponse struct {
	Code string          `json:"code"` // 响应码
	Msg  string          `json:"msg"`  // 响应消息
	Data json.RawMessage `json:"data"` // 响应数据
}

// OKXRestError OKX REST API错误
type OKXRestError struct {
	Code string `json:"code"` // 错误码
	Msg  string `json:"msg"`  // 错误消息
}

// Error 实现error接口
func (e *OKXRestError) Error() string {
	return fmt.Sprintf("OKX API Error [%s]: %s", e.Code, e.Msg)
}

// IsRateLimitError 判断是否为速率限制错误
func (e *OKXRestError) IsRateLimitError() bool {
	return e.Code == "50011" || e.Code == "50061"
}

// IsAuthenticationError 判断是否为认证错误
func (e *OKXRestError) IsAuthenticationError() bool {
	authCodes := []string{"50001", "50002", "50003", "50004", "50005", "50006", "50007", "50008", "50013", "50029"}
	for _, code := range authCodes {
		if e.Code == code {
			return true
		}
	}
	return false
}

// IsTradingError 判断是否为交易错误
func (e *OKXRestError) IsTradingError() bool {
	tradingCodes := []string{"50044", "50055", "50056", "50057", "50058", "58100", "58101", "58102", "58103", "58104", "58105", "58106", "58107", "58108", "58109"}
	for _, code := range tradingCodes {
		if e.Code == code {
			return true
		}
	}
	return false
}

// ShouldRetry 判断是否应该重试
func (e *OKXRestError) ShouldRetry() bool {
	return e.Code == "50011" || e.Code == "50061" || e.Code == "58200"
}

// GetRetryDelay 获取重试延迟
func (e *OKXRestError) GetRetryDelay() time.Duration {
	if e.IsRateLimitError() {
		return 1 * time.Second // 速率限制等待1秒
	}
	return 500 * time.Millisecond // 其他错误等待500毫秒
}

// OKXConfigValidator OKX配置验证器
type OKXConfigValidator struct {
	MaxAPIKeyLength     int
	MinAPIKeyLength     int
	MaxSecretKeyLength  int
	MinSecretKeyLength  int
	MaxPassphraseLength int
	MinPassphraseLength int
}

// NewOKXConfigValidator 创建OKX配置验证器
func NewOKXConfigValidator() *OKXConfigValidator {
	return &OKXConfigValidator{
		MaxAPIKeyLength:     50,
		MinAPIKeyLength:     10,
		MaxSecretKeyLength:  100,
		MinSecretKeyLength:  20,
		MaxPassphraseLength: 50,
		MinPassphraseLength: 6,
	}
}

// ValidateCredentials 验证凭证
func (v *OKXConfigValidator) ValidateCredentials(apiKey, secretKey, passphrase string) error {
	if apiKey == "" {
		return fmt.Errorf("API密钥不能为空")
	}
	if len(apiKey) < v.MinAPIKeyLength {
		return fmt.Errorf("API密钥长度不能少于%d个字符", v.MinAPIKeyLength)
	}
	if len(apiKey) > v.MaxAPIKeyLength {
		return fmt.Errorf("API密钥长度不能超过%d个字符", v.MaxAPIKeyLength)
	}

	if secretKey == "" {
		return fmt.Errorf("Secret密钥不能为空")
	}
	if len(secretKey) < v.MinSecretKeyLength {
		return fmt.Errorf("Secret密钥长度不能少于%d个字符", v.MinSecretKeyLength)
	}
	if len(secretKey) > v.MaxSecretKeyLength {
		return fmt.Errorf("Secret密钥长度不能超过%d个字符", v.MaxSecretKeyLength)
	}

	if passphrase == "" {
		return fmt.Errorf("Passphrase不能为空")
	}
	if len(passphrase) < v.MinPassphraseLength {
		return fmt.Errorf("Passphrase长度不能少于%d个字符", v.MinPassphraseLength)
	}
	if len(passphrase) > v.MaxPassphraseLength {
		return fmt.Errorf("Passphrase长度不能超过%d个字符", v.MaxPassphraseLength)
	}

	return nil
}

// ValidateSymbol 验证交易对
func (v *OKXConfigValidator) ValidateSymbol(symbol string) error {
	return ValidateSymbol(symbol)
}

// ValidateQuantity 验证数量
func (v *OKXConfigValidator) ValidateQuantity(quantity float64) error {
	return ValidateQuantity(quantity)
}

// ValidatePrice 验证价格
func (v *OKXConfigValidator) ValidatePrice(price float64) error {
	return ValidatePrice(price)
}

// ValidateLeverage 验证杠杆
func (v *OKXConfigValidator) ValidateLeverage(leverage int) error {
	return ValidateLeverage(leverage)
}

// OKXTradingLimits OKX交易限制
type OKXTradingLimits struct {
	MaxOpenOrders     int     `json:"max_open_orders"`     // 最大开仓订单数
	MaxAlgoOrders     int     `json:"max_algo_orders"`     // 最大策略订单数
	MaxPositionSize   float64 `json:"max_position_size"`   // 最大持仓数量
	MinOrderSize      float64 `json:"min_order_size"`      // 最小订单数量
	MaxOrderSize      float64 `json:"max_order_size"`      // 最大订单数量
	MaxLeverage       int     `json:"max_leverage"`        // 最大杠杆倍数
	MinLeverage       int     `json:"min_leverage"`        // 最小杠杆倍数
	MaxNotionalValue  float64 `json:"max_notional_value"`  // 最大名义价值
	MinNotionalValue  float64 `json:"min_notional_value"`  // 最小名义价值
}

// DefaultOKXTradingLimits 默认OKX交易限制
var DefaultOKXTradingLimits = OKXTradingLimits{
	MaxOpenOrders:    100,
	MaxAlgoOrders:    100,
	MaxPositionSize:  10000,
	MinOrderSize:     0.001,
	MaxOrderSize:     1000,
	MaxLeverage:      125,
	MinLeverage:      1,
	MaxNotionalValue: 10000000,
	MinNotionalValue: 1,
}

// ValidateTradingLimits 验证交易限制
func ValidateTradingLimits(limits OKXTradingLimits) error {
	if limits.MaxOpenOrders <= 0 {
		return fmt.Errorf("最大开仓订单数必须大于0")
	}
	if limits.MaxAlgoOrders <= 0 {
		return fmt.Errorf("最大策略订单数必须大于0")
	}
	if limits.MaxPositionSize <= 0 {
		return fmt.Errorf("最大持仓数量必须大于0")
	}
	if limits.MinOrderSize <= 0 {
		return fmt.Errorf("最小订单数量必须大于0")
	}
	if limits.MaxOrderSize <= limits.MinOrderSize {
		return fmt.Errorf("最大订单数量必须大于最小订单数量")
	}
	if limits.MaxLeverage <= 0 {
		return fmt.Errorf("最大杠杆倍数必须大于0")
	}
	if limits.MinLeverage <= 0 {
		return fmt.Errorf("最小杠杆倍数必须大于0")
	}
	if limits.MaxLeverage <= limits.MinLeverage {
		return fmt.Errorf("最大杠杆倍数必须大于最小杠杆倍数")
	}
	if limits.MaxNotionalValue <= 0 {
		return fmt.Errorf("最大名义价值必须大于0")
	}
	if limits.MinNotionalValue <= 0 {
		return fmt.Errorf("最小名义价值必须大于0")
	}
	if limits.MaxNotionalValue <= limits.MinNotionalValue {
		return fmt.Errorf("最大名义价值必须大于最小名义价值")
	}
	return nil
}

// OKXMarketData OKX市场数据
type OKXMarketData struct {
	Symbol      string  `json:"symbol"`       // 交易对
	LastPrice   float64 `json:"last_price"`   // 最新价格
	BidPrice    float64 `json:"bid_price"`    // 买一价
	AskPrice    float64 `json:"ask_price"`    // 卖一价
	BidSize     float64 `json:"bid_size"`     // 买一量
	AskSize     float64 `json:"ask_size"`     // 卖一量
	Volume24h   float64 `json:"volume_24h"`   // 24小时成交量
	Value24h    float64 `json:"value_24h"`    // 24小时成交额
	High24h     float64 `json:"high_24h"`     // 24小时最高价
	Low24h      float64 `json:"low_24h"`      // 24小时最低价
	Open24h     float64 `json:"open_24h"`     // 24小时开盘价
	Timestamp   int64   `json:"timestamp"`    // 时间戳
}

// OKXAccountSettings OKX账户设置信息
type OKXAccountSettings struct {
	UserID      string  `json:"user_id"`       // 用户ID
	AccountLevel string `json:"account_level"` // 账户等级
	PositionMode string `json:"position_mode"` // 持仓模式
	AutoLoan    bool    `json:"auto_loan"`     // 自动借币
	GreedyMode  bool    `json:"greedy_mode"`   // 贪婪模式
}

// OKXOrderBook OKX订单簿
type OKXOrderBook struct {
	Symbol    string              `json:"symbol"`     // 交易对
	Bids      []OKXOrderBookItem  `json:"bids"`       // 买单
	Asks      []OKXOrderBookItem  `json:"asks"`       // 卖单
	Timestamp int64               `json:"timestamp"`  // 时间戳
}

// OKXOrderBookItem OKX订单簿项
type OKXOrderBookItem struct {
	Price  float64 `json:"price"`  // 价格
	Size   float64 `json:"size"`   // 数量
	Orders int     `json:"orders"` // 订单数
}

// OKXRecentTrades OKX最近成交
type OKXRecentTrades struct {
	Symbol    string  `json:"symbol"`     // 交易对
	Trades    []OKXTrade `json:"trades"`  // 成交记录
	Timestamp int64   `json:"timestamp"`  // 时间戳
}

// OKXTrade OKX成交记录
type OKXTrade struct {
	TradeID   string  `json:"trade_id"`   // 成交ID
	Price     float64 `json:"price"`      // 成交价格
	Size      float64 `json:"size"`       // 成交数量
	Side      string  `json:"side"`       // 成交方向
	Timestamp int64   `json:"timestamp"`  // 成交时间
}

// OKXKlineData OKX K线数据
type OKXKlineData struct {
	Symbol    string  `json:"symbol"`     // 交易对
	Interval  string  `json:"interval"`   // 时间间隔
	OpenTime  int64   `json:"open_time"`  // 开盘时间
	CloseTime int64   `json:"close_time"` // 收盘时间
	Open      float64 `json:"open"`       // 开盘价
	High      float64 `json:"high"`       // 最高价
	Low       float64 `json:"low"`        // 最低价
	Close     float64 `json:"close"`      // 收盘价
	Volume    float64 `json:"volume"`     // 成交量
	Value     float64 `json:"value"`      // 成交额
}

// OKXFundingRate OKX资金费率
type OKXFundingRate struct {
	Symbol       string  `json:"symbol"`        // 交易对
	FundingRate  float64 `json:"funding_rate"`  // 资金费率
	FundingTime  int64   `json:"funding_time"`  // 资金费率时间
	NextFundingTime int64 `json:"next_funding_time"` // 下次资金费率时间
}

// OKXOpenInterest OKX持仓量
type OKXOpenInterest struct {
	Symbol       string  `json:"symbol"`        // 交易对
	OpenInterest float64 `json:"open_interest"` // 持仓量
	Timestamp    int64   `json:"timestamp"`     // 时间戳
}

// OKXInsuranceFund OKX保险基金
type OKXInsuranceFund struct {
	Symbol      string  `json:"symbol"`       // 交易对
	InsuranceFund float64 `json:"insurance_fund"` // 保险基金余额
	Timestamp   int64   `json:"timestamp"`    // 时间戳
}

// OKXDeliveryExerciseHistory OKX交割行权历史
type OKXDeliveryExerciseHistory struct {
	Symbol      string  `json:"symbol"`       // 交易对
	Type        string  `json:"type"`         // 类型: delivery/exercise
	Timestamp   int64   `json:"timestamp"`    // 时间戳
	Price       float64 `json:"price"`        // 交割/行权价格
	Volume      float64 `json:"volume"`       // 交割/行权数量
}

// OKXAPIResponseWrapper OKX API响应包装器
type OKXAPIResponseWrapper struct {
	Success bool        `json:"success"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	Error   error       `json:"error,omitempty"`
	Time    int64       `json:"time"`
}

// NewSuccessResponse 创建成功响应
func NewSuccessResponse(data interface{}) *OKXAPIResponseWrapper {
	return &OKXAPIResponseWrapper{
		Success: true,
		Code:    "0",
		Message: "success",
		Data:    data,
		Time:    time.Now().Unix(),
	}
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(code, message string, err error) *OKXAPIResponseWrapper {
	return &OKXAPIResponseWrapper{
		Success: false,
		Code:    code,
		Message: message,
		Error:   err,
		Time:    time.Now().Unix(),
	}
}

// OKXErrorHandler OKX错误处理器
type OKXErrorHandler struct {
	RetryStrategy RetryStrategy
	Logger        *log.Logger
}

// NewOKXErrorHandler 创建OKX错误处理器
func NewOKXErrorHandler() *OKXErrorHandler {
	return &OKXErrorHandler{
		RetryStrategy: DefaultRetryStrategy,
		Logger:        log.New(log.Writer(), "[OKX_ERROR] ", log.LstdFlags),
	}
}

// HandleError 处理错误
func (h *OKXErrorHandler) HandleError(err error, operation string) error {
	if err == nil {
		return nil
	}

	h.Logger.Printf("操作 %s 发生错误: %v", operation, err)

	// 标准化错误
	standardizedErr := StandardizeError(err)

	// 记录安全事件
	if IsAuthenticationError(err.Error()) {
		LogSecurityEvent("认证失败", operation)
	}

	return standardizedErr
}

// ShouldRetry 判断是否应该重试
func (h *OKXErrorHandler) ShouldRetry(err error, attempt int) bool {
	return h.RetryStrategy.ShouldRetry(err, attempt)
}

// CalculateDelay 计算重试延迟
func (h *OKXErrorHandler) CalculateDelay(attempt int) time.Duration {
	return h.RetryStrategy.CalculateDelay(attempt)
}

// LogRetry 记录重试信息
func (h *OKXErrorHandler) LogRetry(operation string, attempt int, delay time.Duration) {
	h.Logger.Printf("重试操作 %s: 第 %d 次尝试，延迟 %v", operation, attempt, delay)
}