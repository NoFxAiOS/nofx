# 修复方案：OKX持仓数据解析修复

## 🎯 目标
修复 `OKXTrader` 中持仓数据解析逻辑，使其输出的数据结构与 `AutoTrader` 的校验逻辑完全兼容，确保AI能够正确获取并识别账户持仓状态。

## 📝 变更内容

### 修改文件: `trader/okx_trader.go`

#### 1. 引入依赖
确保引入了 `strconv` 包，用于字符串到数字的转换。

#### 2. 修改 `parsePositions` 函数

**当前逻辑**:
```go
standardizedPos := map[string]interface{}{
    "symbol":    pos["instId"],
    "position":  pos["pos"],
    "posSide":   pos["posSide"],
    "avgPrice":  pos["avgPx"],
    "leverage":  pos["lever"],
    "marginMode": pos["mgnMode"],
    "upl":       pos["upl"],
    "uplRatio":  pos["uplRatio"],
}
```

**建议修改逻辑**:
```go
func (t *OKXTrader) parsePositions(resp map[string]interface{}) []map[string]interface{} {
    var positions []map[string]interface{}

    if data, ok := resp["data"].([]interface{}); ok {
        for _, item := range data {
            if pos, ok := item.(map[string]interface{}); ok {
                // 辅助函数：安全解析float字符串
                parseFloat := func(key string) float64 {
                    if valStr, ok := pos[key].(string); ok && valStr != "" {
                        if val, err := strconv.ParseFloat(valStr, 64); err == nil {
                            return val
                        }
                    }
                    return 0.0
                }

                // 解析关键数值字段
                markPrice := parseFloat("markPx")
                entryPrice := parseFloat("avgPx")
                quantity := parseFloat("pos")
                upl := parseFloat("upl")
                liqPx := parseFloat("liqPx")
                leverage := parseFloat("lever")

                // 标准化持仓数据格式 (适配 AutoTrader 要求)
                standardizedPos := map[string]interface{}{
                    // 核心字段 (AutoTrader 必需)
                    "symbol":           pos["instId"],
                    "side":             pos["posSide"],     // AutoTrader期望 key="side"
                    "markPrice":        markPrice,          // AutoTrader期望 key="markPrice" (float64)
                    "entryPrice":       entryPrice,         // AutoTrader期望 key="entryPrice" (float64)
                    "positionAmt":      quantity,           // AutoTrader期望 key="positionAmt" (float64)
                    "unRealizedProfit": upl,                // AutoTrader期望 key="unRealizedProfit" (float64)
                    "leverage":         leverage,           // AutoTrader期望 key="leverage" (float64)
                    "liquidationPrice": liqPx,              // AutoTrader期望 key="liquidationPrice" (float64)

                    // 兼容性/原始字段
                    "posSide":          pos["posSide"],
                    "marginMode":       pos["mgnMode"],
                    "uplRatio":         pos["uplRatio"],
                }
                positions = append(positions, standardizedPos)
            }
        }
    }

    return positions
}
```

## 🧪 验证计划

由于无法直接连接生产环境API，验证将主要依赖代码审查和部署后的日志观察。

1.  **代码编译检查**: 确保修改后的代码无编译错误。
2.  **部署观察**:
    -   部署更新后的代码。
    -   观察日志输出，确认 `trader/auto_trader.go` 中的 `buildTradingContext` 是否成功获取到持仓（日志中应该不再显示 "当前无持仓" 的提示，或者在 `GetAccountInfo` 的日志中能看到持仓详情）。
    -   检查 Dashboard 的 Chain of Thought 是否正确显示持仓列表。

## ⚠️ 注意事项
- OKX API 返回的数值通常都是字符串类型，必须使用 `strconv.ParseFloat` 进行转换。
- `markPx` 是必需字段，如果 API 偶尔不返回该字段，可能会导致持仓依然被忽略。建议添加 fallback 逻辑（例如如果 `markPx` 为0，尝试使用 `market.Get(symbol)` 获取当前价格作为替补），但在本次修复中先优先处理字段映射问题。
