# Bug报告：OKX交易所持仓状态显示错误 (无持仓)

## 📋 基本信息
- **Bug ID**: BUG-2025-1206-001
- **优先级**: P1 (高)
- **影响模块**: 后端 `trader` 模块 (OKX集成)
- **发现时间**: 2025-12-06
- **状态**: 待修复

## 🚨 问题描述

### 用户反馈
在 https://www.agentrade.xyz/dashboard 的"最近决策"中，AI的思维链（Chain of Thought）分析内容显示"账户状态：无持仓"（No position），但实际上账户中存在3个活跃持仓。

### 现象描述
1. 用户在OKX账户中有未平仓的合约订单。
2. 自动交易程序运行日志中显示 "ℹ️ 当前无持仓，跳过止盈止损更新"。
3. AI构建的决策上下文（Context）中，`Positions` 列表为空。
4. 导致AI决策逻辑判断为"无持仓"，可能引发重复开仓或无法执行平仓操作。

## 🔍 技术分析

### 错误定位
**文件**: 
1. `trader/okx_trader.go` (函数: `parsePositions`)
2. `trader/auto_trader.go` (函数: `buildTradingContext`)

**根本原因**: `OKXTrader` 返回的持仓数据结构与 `AutoTrader` 所期望的数据结构不匹配，导致持仓数据被校验逻辑过滤掉。

### 详细分析

#### 1. AutoTrader 的数据校验逻辑
在 `trader/auto_trader.go` 的 `buildTradingContext` 方法中（约第424行），存在如下校验：
```go
// 跳过无效持仓数据
if symbol == "" || side == "" || markPrice == 0 {
        continue
}
```
这意味如果持仓数据中缺少 `markPrice`（标记价格）或者 `side`（方向）为空，该持仓将被直接忽略。

#### 2. OKXTrader 的数据返回格式
在 `trader/okx_trader.go` 的 `parsePositions` 方法中，OKX API返回的数据被映射为：
```go
standardizedPos := map[string]interface{}{
        "symbol":    pos["instId"],
        "position":  pos["pos"],
        "posSide":   pos["posSide"], // 注意：这里用了 "posSide" 而不是 "side"
        "avgPrice":  pos["avgPx"],   // 注意：这里用了 "avgPrice" 且是string类型
        "leverage":  pos["lever"],
        "marginMode": pos["mgnMode"],
        "upl":       pos["upl"],
        "uplRatio":  pos["uplRatio"],
}
```
**关键缺失**: 
- 没有映射 `markPx` 到 `markPrice`。
- 使用了 `posSide` 作为键名，而 `AutoTrader` 期望的是 `side`。
- 字段值多为 `string` 类型（源自JSON），而 `AutoTrader` 期望 `float64`。

#### 3. 字段映射不匹配表

| 字段含义 | AutoTrader 期望键名 (类型) | OKXTrader 提供键名 (类型) | 结果 |
| :--- | :--- | :--- | :--- |
| 交易对 | `symbol` (string) | `symbol` (string) | ✅ 匹配 |
| 方向 | `side` (string) | `posSide` (string) | ❌ **不匹配** (键名不同) |
| 标记价格 | `markPrice` (float64) | **缺失** | ❌ **导致被过滤** |
| 开仓均价 | `entryPrice` (float64) | `avgPrice` (string) | ❌ **不匹配** (键名与类型) |
| 持仓数量 | `positionAmt` (float64) | `position` (string) | ❌ **不匹配** (键名与类型) |
| 未实现盈亏 | `unRealizedProfit` (float64) | `upl` (string) | ❌ **不匹配** (键名与类型) |
| 强平价格 | `liquidationPrice` (float64) | **缺失** | ❌ **缺失** |

### 调用链路
```
AutoTrader.runCycle()
  ↓
AutoTrader.buildTradingContext()
  ↓ 调用
OKXTrader.GetPositions() -> parsePositions() [数据源头不完整]
  ↓ 返回 map列表
AutoTrader 遍历列表
  ↓
if markPrice == 0 { continue } [校验失败，持仓被丢弃]
  ↓
Context.Positions 为空
  ↓
AI Prompt 接收到 "No Position"
```

## 🛠 解决方案

### 方案建议
修改 `trader/okx_trader.go` 中的 `parsePositions` 方法，使其返回符合 `AutoTrader` 期望的标准数据结构。需要处理字段重命名和类型转换（string 转 float64）。

### 建议的数据映射代码（伪代码）

```go
// 需要引入 strconv 包
// 在 parsePositions 中：

// 1. 解析标记价格 (markPx)
markPrice, _ := strconv.ParseFloat(pos["markPx"].(string), 64)

// 2. 解析其他数值字段
entryPrice, _ := strconv.ParseFloat(pos["avgPx"].(string), 64)
quantity, _ := strconv.ParseFloat(pos["pos"].(string), 64)
upl, _ := strconv.ParseFloat(pos["upl"].(string), 64)
liqPx, _ := strconv.ParseFloat(pos["liqPx"].(string), 64)
leverage, _ := strconv.ParseFloat(pos["lever"].(string), 64)

// 3. 构建标准化Map
standardizedPos := map[string]interface{}{
    "symbol":           pos["instId"],
    "side":             pos["posSide"],     // 修正键名为 side
    "markPrice":        markPrice,          // 新增标记价格 (float64)
    "entryPrice":       entryPrice,         // 修正键名为 entryPrice (float64)
    "positionAmt":      quantity,           // 修正键名为 positionAmt (float64)
    "unRealizedProfit": upl,                // 修正键名为 unRealizedProfit (float64)
    "liquidationPrice": liqPx,              // 新增强平价格 (float64)
    "leverage":         leverage,           // 转换为 float64
    // 保留原有字段以防其他模块使用（可选）
    "posSide":          pos["posSide"],
}
```

## ✅ 验证标准

1. **日志验证**: 启动程序后，不再出现 "当前无持仓，跳过止盈止损更新" 的提示（当实际有持仓时）。
2. **Dashboard验证**: 决策日志中的思维链部分，能够正确列出当前的持仓信息（Symbol, Side, PnL等）。
3. **功能验证**: 
    - AI能够识别当前持仓并给出平仓建议（Close Long/Short）。
    - 止盈止损更新逻辑能够正常执行。

## 📊 影响评估

### 影响范围
- **用户**: 使用OKX交易所的所有用户。
- **功能**: 仓位管理、风险控制、AI决策准确性。

### 风险评估
- **高风险**: 目前AI认为自己"空仓"，可能会在已有仓位的情况下继续开仓，导致风险敞口倍增。
- **高风险**: 无法执行针对现有仓位的平仓操作。

### 紧急程度
**P0 - 极高优先级**
- 涉及资金安全和核心交易逻辑，必须立即修复。
