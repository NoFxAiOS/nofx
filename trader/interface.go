package trader

import "database/sql"

// CreditReservation 积分预留凭证
// 用于两阶段提交：先预留积分，交易成功后确认消费，失败则释放
type CreditReservation struct {
        ID               string       // 预留ID（通常使用tradeID）
        UserID           string       // 用户ID
        TradeID          string       // 交易ID
        Amount           int          // 预留积分数量
        Tx               *sql.Tx      // 数据库事务（用于确认或释放）
        alreadyProcessed bool         // 是否已处理过（幂等性检查）
        onConfirm        func(symbol, action, traderID string) error // 确认回调
        onRelease        func() error // 释放回调
}

// Confirm 确认积分消费（第二阶段 - 成功）
func (r *CreditReservation) Confirm(symbol, action, traderID string) error {
        if r.alreadyProcessed {
                return nil // 已处理过，直接返回成功
        }
        if r.onConfirm != nil {
                return r.onConfirm(symbol, action, traderID)
        }
        return nil
}

// Release 释放积分预留（第二阶段 - 失败）
func (r *CreditReservation) Release() error {
        if r.alreadyProcessed {
                return nil // 已处理过，直接返回成功
        }
        if r.onRelease != nil {
                return r.onRelease()
        }
        return nil
}

// IsAlreadyProcessed 检查是否已处理过
func (r *CreditReservation) IsAlreadyProcessed() bool {
        return r.alreadyProcessed
}

// CreditConsumer 积分消费者接口
// 定义交易积分消费的统一接口
type CreditConsumer interface {
        // ReserveCredit 预留积分用于交易
        // 返回预留凭证，用于后续确认或释放
        ReserveCredit(userID, tradeID string) (*CreditReservation, error)
}

// Trader 交易器统一接口
// 支持多个交易平台（币安、Hyperliquid等）
type Trader interface {
        // GetBalance 获取账户余额
        GetBalance() (map[string]interface{}, error)

        // GetPositions 获取所有持仓
        GetPositions() ([]map[string]interface{}, error)

        // OpenLong 开多仓
        OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error)

        // OpenShort 开空仓
        OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error)

        // CloseLong 平多仓（quantity=0表示全部平仓）
        CloseLong(symbol string, quantity float64) (map[string]interface{}, error)

        // CloseShort 平空仓（quantity=0表示全部平仓）
        CloseShort(symbol string, quantity float64) (map[string]interface{}, error)

        // SetLeverage 设置杠杆
        SetLeverage(symbol string, leverage int) error

        // SetMarginMode 设置仓位模式 (true=全仓, false=逐仓)
        SetMarginMode(symbol string, isCrossMargin bool) error

        // GetMarketPrice 获取市场价格
        GetMarketPrice(symbol string) (float64, error)

        // SetStopLoss 设置止损单
        SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error

        // SetTakeProfit 设置止盈单
        SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error

        // CancelAllOrders 取消该币种的所有挂单
        CancelAllOrders(symbol string) error

        // FormatQuantity 格式化数量到正确的精度
        FormatQuantity(symbol string, quantity float64) (string, error)
}
