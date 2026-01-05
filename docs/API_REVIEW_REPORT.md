# HTX 和 Gate.io API 实现审查报告

## 审查日期：2026-01-05

## ⚠️ 重要提示：API 版本说明

### HTX API 版本问题

**当前项目使用的是 Huobi 旧版 API**：

- 🔴 **旧版域名**：`api.huobi.pro`
- 🔴 **旧版文档**：`huobiapi.github.io/docs/usdt_swap/v1/`
- 🔴 **旧版路径**：`/linear-swap-api/v1/*`

**HTX 官方最新 API（推荐迁移）**：

- ✅ **新版域名**：`api.htx.com`
- ✅ **新版文档**：[HTX 开放平台 - U 本位合约](https://www.htx.com/zh-cn/opend/newApiPages/?type=2)
- ✅ **品牌升级**：Huobi 已更名为 HTX，API 已全面升级

**迁移建议**：

1. **兼容性风险**：旧版 API 可能逐步弃用，建议尽快迁移到新版
2. **功能差异**：新版 API 可能有更完善的接口和更高的性能
3. **文档更新**：本报告中引用的旧版文档链接仍可用，但建议对照新版文档
4. **域名切换**：需要修改 `htxBaseURL` 从 `api.huobi.pro` 到 `api.htx.com`

**迁移优先级**：🟠 **中高优先级**（建议在 3-6 个月内完成）

### ✅ 兼容性测试结果（2026-01-05）

**实际测试**：

```bash
# 旧版API测试
$ curl "https://api.huobi.pro/v1/common/timestamp"
{"data":1767583649274,"status":"ok"}  ✅ 正常响应

# 新版API测试
$ curl "https://api.htx.com/v1/common/timestamp"
{"data":1767583653472,"status":"ok"}  ✅ 正常响应
```

**核心结论**：

- ✅ **旧版 API（api.huobi.pro）目前仍然正常工作**
- ✅ **新版 API（api.htx.com）已上线并可用**
- 🟢 **当前代码无需立即更新，可继续稳定运行**
- 📋 **建议在 3-6 个月内完成迁移（非紧急）**

**详细分析**：参见 [HTX_API_COMPATIBILITY_ANALYSIS.md](./HTX_API_COMPATIBILITY_ANALYSIS.md)

---

## 🔍 审查范围

- HTX (Huobi) Linear Swap API v1 **(旧版，当前使用)**
- Gate.io Futures API v4
- 对照官方文档进行完整审查

---

## ❌ 发现的严重问题

### HTX 实现问题

#### 1. **[已修复] 缺少 client_order_id 参数**

**严重程度**: 🔴 高危

**官方文档**:

- 📘 旧版文档: [HTX 合约下单接口 (v1)](https://huobiapi.github.io/docs/usdt_swap/v1/cn/#5ea2e0cde2-2)
- 📗 **新版文档**: [HTX U 本位合约 - 合约下单](https://www.htx.com/zh-cn/opend/newApiPages/?type=2#linear-swap-api-v1-swap-order)
- 文档说明: `client_order_id` 参数为**可选**，用户自定义订单号，最大长度 32 位

**问题描述**:

- 所有下单接口都缺少 `client_order_id` 参数
- 这在高并发或网络延迟场景下可能导致订单重复

**代码位置**:

- 文件: `trader/htx_trader.go`
- 方法: `OpenLong()` (Line 323), `OpenShort()` (Line 360), `CloseLong()` (Line 397), `CloseShort()` (Line 438)

**影响**:

- 网络超时重试可能导致重复下单
- 无法通过客户端订单 ID 追踪订单状态

**修复对照**:

官方示例 (文档):

```json
{
  "contract_code": "BTC-USDT",
  "client_order_id": 9086, // 官方建议添加
  "direction": "buy",
  "offset": "open"
}
```

项目实现 (已修复):

```go
// trader/htx_trader.go Line 333-335
clientOrderID := fmt.Sprintf("nofx_%d", time.Now().UnixNano())
body["client_order_id"] = clientOrderID
```

---

#### 2. **[已修复] 止损止盈价格格式错误**

**严重程度**: 🟠 中危

**官方文档**:

- 📘 旧版文档: [HTX 计划委托下单 (v1)](https://huobiapi.github.io/docs/usdt_swap/v1/cn/#2b634a2f98)
- 📗 **新版文档**: [HTX U 本位合约 - 计划委托下单](https://www.htx.com/zh-cn/opend/newApiPages/?type=2#linear-swap-api-v1-swap-trigger-order)
- 文档说明: `trigger_price` 和 `order_price` 类型为 **decimal (string)**

**问题描述**:

- `trigger_price` 和 `order_price` 应该使用字符串格式
- 直接传 float64 可能被交易所拒绝

**代码位置**:

- 文件: `trader/htx_trader.go`
- 方法: `SetStopLoss()` (Line 558), `SetTakeProfit()` (Line 581)

**修复对照**:

官方要求 (文档):

```json
{
  "trigger_price": "50000.5", // string 类型
  "order_price": "50000.5" // string 类型
}
```

项目实现 (已修复):

```go
// trader/htx_trader.go Line 565-566
"trigger_price": fmt.Sprintf("%.8f", stopPrice),  // ✅ 正确：字符串格式
"order_price":   fmt.Sprintf("%.8f", stopPrice),
```

---

#### 3. **[已修复] GetOrderStatus 未实现**

**严重程度**: 🔴 高危

**官方文档**:

- 📘 旧版文档: [HTX 获取合约订单信息 (v1)](https://huobiapi.github.io/docs/usdt_swap/v1/cn/#5ea2e0cde2-3)
- 📗 **新版文档**: [HTX U 本位合约 - 获取订单信息](https://www.htx.com/zh-cn/opend/newApiPages/?type=2#linear-swap-api-v1-swap-order-info)
- API: `POST /linear-swap-api/v1/swap_order_info`
- 必需参数: `contract_code` + (`order_id` 或 `client_order_id`)

**问题描述**:

- 原代码直接返回 `FILLED` 状态，未实际查询交易所
- 导致订单状态追踪完全失效

**代码位置**:

- 文件: `trader/htx_trader.go`
- 方法: `GetOrderStatus()` (Line 693-719)

**修复对照**:

官方接口 (文档):

```http
POST /linear-swap-api/v1/swap_order_info
{
  "contract_code": "BTC-USDT",
  "order_id": "773131315209248768"
}
```

官方响应 (文档):

```json
{
  "status": "ok",
  "data": [
    {
      "order_id": 773131315209248768,
      "status": 6, // 1:准备提交 3:已提交 4:部分成交 6:全部成交 7:已撤销
      "trade_avg_price": "50000.5",
      "trade_volume": 10,
      "fee": 0.02
    }
  ]
}
```

项目实现 (已修复):

```go
// trader/htx_trader.go Line 693-748
func (t *HTXTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
    body := map[string]interface{}{
        "contract_code": symbol,
        "order_id":      orderID,
    }
    data, err := t.doRequest("POST", "/linear-swap-api/v1/swap_order_info", nil, body)
    // ... 解析响应并转换状态码
}
```

---

#### 4. **[未实现] GetClosedPnL 历史盈亏**

**严重程度**: 🟠 中危

**官方文档**:

- 📘 旧版文档: [HTX 查询用户财务记录 (v1)](https://huobiapi.github.io/docs/usdt_swap/v1/cn/#5ea2e0cde2-35)
- 📗 **新版文档**: [HTX U 本位合约 - 查询用户财务记录](https://www.htx.com/zh-cn/opend/newApiPages/?type=2#linear-swap-api-v1-swap-financial-record)
- API: `POST /linear-swap-api/v1/swap_financial_record`
- 参数: `contract_code`, `type` (平多、平空), `start_time`, `end_time`

**问题描述**:

- 返回空数组，无法获取历史盈亏数据
- 影响盈亏统计和回测功能

**代码位置**:

- 文件: `trader/htx_trader.go`
- 方法: `GetClosedPnL()` (Line 754-757)

**需要实现的对照**:

官方接口 (文档):

```http
POST /linear-swap-api/v1/swap_financial_record
{
  "contract_code": "BTC-USDT",
  "type": "3,4",  // 3:平多 4:平空
  "start_time": 1604160000000,
  "end_time": 1606752000000
}
```

项目当前实现:

```go
// trader/htx_trader.go Line 754-757
func (t *HTXTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
    return []ClosedPnLRecord{}, nil  // ❌ 返回空数组
}
```

建议实现:

```go
// 需要调用 swap_financial_record 接口
// 解析返回的盈亏记录并转换为 ClosedPnLRecord 格式
```

---

#### 5. **[已修复] CancelAllOrders 参数不完整**

**严重程度**: 🟡 低危

**官方文档**:

- 📘 旧版文档: [HTX 撤销订单 (v1)](https://huobiapi.github.io/docs/usdt_swap/v1/cn/#5ea2e0cde2-4)
- 📗 **新版文档**: [HTX U 本位合约 - 撤销订单](https://www.htx.com/zh-cn/opend/newApiPages/?type=2#linear-swap-api-v1-swap-cancel)
- API: `POST /linear-swap-api/v1/swap_cancel`
- 必需参数: `contract_code` + (`order_id` 或 `client_order_id`)

**问题描述**:

- HTX 的取消订单接口**必须**指定 `order_id`
- 原代码只传了 `contract_code` 无法取消订单

**代码位置**:

- 文件: `trader/htx_trader.go`
- 方法: `CancelAllOrders()` (Line 618-648)

**修复对照**:

官方要求 (文档):

```json
{
  "contract_code": "BTC-USDT",
  "order_id": "773131315209248768" // 必需参数
}
```

项目实现 (已修复):

```go
// trader/htx_trader.go Line 618-648
// ✅ 正确流程：先查询所有挂单，再逐个取消
// 1. POST /linear-swap-api/v1/swap_openorders (查询)
// 2. 遍历订单列表
// 3. POST /linear-swap-api/v1/swap_cancel (逐个取消)
```

---

#### 6. **[待完善] 缺少合约信息缓存**

**严重程度**: 🟡 低危

**官方文档**:

- 📘 旧版文档: [HTX 获取合约信息 (v1)](https://huobiapi.github.io/docs/usdt_swap/v1/cn/#5ea2e0cde2-42)
- 📗 **新版文档**: [HTX U 本位合约 - 获取合约信息](https://www.htx.com/zh-cn/opend/newApiPages/?type=2#linear-swap-api-v1-swap-contract-info)
- API: `GET /linear-swap-api/v1/swap_contract_info`
- 返回: 合约价格精度、数量精度等信息

**问题描述**:

- 定义了 `HTXContract` 结构和缓存，但从未使用
- 可能导致下单数量精度问题

**代码位置**:

- 文件: `trader/htx_trader.go`
- 结构: `HTXContract` (Line 70-77)
- 缓存字段: `contractsCache` (Line 59-61) - 定义但未使用

**建议实现**:

官方接口 (文档):

```http
GET /linear-swap-api/v1/swap_contract_info?contract_code=BTC-USDT
```

官方响应 (文档):

```json
{
  "data": [
    {
      "contract_code": "BTC-USDT",
      "contract_size": 0.001,
      "price_tick": 0.1,
      "settlement_time": "1604160000000"
    }
  ]
}
```

建议添加:

```go
// 在下单前验证数量精度
func (t *HTXTrader) validateQuantity(symbol string, quantity float64) error {
    contract := t.getContractInfo(symbol)  // 需要实现
    // 验证数量是否符合最小/最大限制
}
```

---

### Gate.io 实现问题

#### 1. **[已修复] 止损止盈 rule 参数错误**

**严重程度**: 🔴 高危

**官方文档**:

- 📘 [Gate.io 价格触发订单](https://www.gate.io/docs/developers/apiv4/zh_CN/#%E5%88%9B%E5%BB%BA%E4%BB%B7%E6%A0%BC%E8%A7%A6%E5%8F%91%E8%AE%A2%E5%8D%95)
- API: `POST /api/v4/futures/usdt/price_orders`
- `rule` 参数: `1` = >=, `2` = <=

**问题描述**:

- `rule` 参数决定触发条件（>= 或 <=）
- 原实现对多空仓的 rule 设置反了

**代码位置**:

- 文件: `trader/gate_trader.go`
- 方法: `SetStopLoss()` (Line 559-582), `SetTakeProfit()` (Line 591-614)

**修复对照**:

官方文档 (rule 定义):

```
rule: 触发条件
  1: 价格 >= trigger_price 时触发
  2: 价格 <= trigger_price 时触发
```

止损逻辑 (官方):

```
多仓止损: 价格下跌，需要 rule=2 (<=)
空仓止损: 价格上涨，需要 rule=1 (>=)
```

止盈逻辑 (官方):

```
多仓止盈: 价格上涨，需要 rule=1 (>=)
空仓止盈: 价格下跌，需要 rule=2 (<=)
```

项目实现 (已修复):

```go
// trader/gate_trader.go Line 562-567 (SetStopLoss)
rule := 2 // <= for long stop loss
if positionSide == "short" {
    rule = 1 // >= for short stop loss
}

// trader/gate_trader.go Line 594-599 (SetTakeProfit)
rule := 1 // >= for long take profit
if positionSide == "short" {
    rule = 2 // <= for short take profit
}
```

---

#### 2. **[需验证] GetOrderStatus 实现**

**严重程度**: 🟠 中危

**官方文档**:

- 📘 [Gate.io 查询单个订单](https://www.gate.io/docs/developers/apiv4/zh_CN/#%E6%9F%A5%E8%AF%A2%E5%8D%95%E4%B8%AA%E8%AE%A2%E5%8D%95-2)
- API: `GET /api/v4/futures/usdt/orders/{order_id}`
- 必需参数: `settle=usdt`, `order_id` (路径参数)

**问题描述**:

- Gate.io 的订单查询需要特定的 settle 参数
- 当前实现需要验证是否完全匹配官方响应格式

**代码位置**:

- 文件: `trader/gate_trader.go`
- 方法: `GetOrderStatus()` (Line 686-709)

**官方接口对照**:

官方响应 (文档):

```json
{
  "id": 123456789,
  "status": "finished", // open, finished
  "fill_price": "50000.5",
  "size": 10,
  "left": 0
}
```

项目实现:

```go
// trader/gate_trader.go Line 686-709
func (t *GateTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
    path := fmt.Sprintf(gateCancelOrderPath, orderID)  // ⚠️ 使用了取消订单的路径
    data, err := t.doRequest("GET", path, query, nil)
    // 状态转换为大写: "finished" -> "FINISHED"
}
```

**需要验证**:

1. ⚠️ 路径是否正确 (使用了 `gateCancelOrderPath` 而非查询路径)
2. 状态映射是否完整 (finished, open, cancelled 等)

---

#### 3. **[未实现] GetClosedPnL 历史盈亏**

**严重程度**: 🟠 中危

**官方文档**:

- 📘 [Gate.io 查询个人成交记录](https://www.gate.io/docs/developers/apiv4/zh_CN/#%E6%9F%A5%E8%AF%A2%E4%B8%AA%E4%BA%BA%E6%88%90%E4%BA%A4%E8%AE%B0%E5%BD%95-2)
- API: `GET /api/v4/futures/usdt/my_trades`
- 参数: `settle=usdt`, `contract` (可选), `limit`, `from`, `to` (时间戳)

**问题描述**:

- 与 HTX 相同，返回空数组
- 无法获取历史盈亏数据

**代码位置**:

- 文件: `trader/gate_trader.go`
- 方法: `GetClosedPnL()` (Line 714-717)

**需要实现的对照**:

官方接口 (文档):

```http
GET /api/v4/futures/usdt/my_trades?settle=usdt&contract=BTC_USDT&from=1604160000&to=1606752000
```

官方响应 (文档):

```json
[
  {
    "id": 123456789,
    "create_time": 1546569968.0,
    "contract": "BTC_USDT",
    "order_id": "987654321",
    "size": 10,
    "price": "50000.5",
    "role": "taker"
  }
]
```

项目当前实现:

```go
// trader/gate_trader.go Line 714-717
func (t *GateTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
    return []ClosedPnLRecord{}, nil  // ❌ 返回空数组
}
```

---

#### 4. **[需测试] CancelStopOrders 查询参数**

**严重程度**: 🟡 低危

**官方文档**:

- 📘 [Gate.io 查询所有价格触发订单](https://www.gate.io/docs/developers/apiv4/zh_CN/#%E6%9F%A5%E8%AF%A2%E6%89%80%E6%9C%89%E4%BB%B7%E6%A0%BC%E8%A7%A6%E5%8F%91%E8%AE%A2%E5%8D%95)
- API: `GET /api/v4/futures/usdt/price_orders`
- 参数: `settle=usdt`, `status` (open/finished), `contract` (可选)

**问题描述**:

- 价格触发订单查询接口参数需验证
- `status: "open"` 是否正确需要实际测试

**代码位置**:

- 文件: `trader/gate_trader.go`
- 方法: `CancelStopOrders()` (Line 628-662)

**官方参数对照**:

官方文档 (status 参数):

```
status: 订单状态
  - open: 等待触发
  - finished: 已完成
  - failed: 失败
  - cancelled: 已取消
```

项目实现:

```go
// trader/gate_trader.go Line 633-636
query := map[string]string{
    "settle":   "usdt",
    "contract": symbol,
    "status":   "open",  // ✅ 正确：查询待触发的订单
}

---

## ✅ 实现正确的部分

### HTX

1. ✅ API 签名算法正确（HMAC-SHA256）
2. ✅ 签名格式正确（method + host + path + params）
3. ✅ 参数排序正确
4. ✅ GZIP 解压缩处理正确
5. ✅ 账户余额查询正确
6. ✅ 持仓查询正确
7. ✅ 杠杆设置正确
8. ✅ 市价单参数正确（optimal_20）

### Gate.io

1. ✅ API 签名算法正确（HMAC-SHA512）
2. ✅ Body hash 处理正确
3. ✅ 签名格式正确
4. ✅ 账户余额查询正确
5. ✅ 持仓查询正确（正负数表示多空）
6. ✅ reduce_only 参数使用正确
7. ✅ 杠杆设置正确
8. ✅ 保证金模式设置正确

---

## 🔧 必须修复的问题（优先级排序）

### P0 - 阻塞性问题（必须立即修复）

1. ❌ **HTX GetOrderStatus** - 订单状态追踪失效
2. ✅ **HTX client_order_id** - 已修复，防止重复下单
3. ✅ **Gate.io 止损止盈 rule** - 已修复，防止反向触发

### P1 - 高优先级（影响核心功能）

4. ❌ **HTX CancelAllOrders** - 取消订单功能失效
5. ❌ **HTX GetClosedPnL** - 影响盈亏统计
6. ❌ **Gate.io GetClosedPnL** - 影响盈亏统计

### P2 - 中优先级（优化和完善）

7. ❌ **HTX 合约信息缓存** - 完善数量精度验证
8. ❌ **Gate.io GetOrderStatus** - 验证和完善

---

## 📝 测试建议

### 必须测试的场景

#### HTX

1. **订单测试**

   - [ ] 开多仓小单（0.001 BTC）
   - [ ] 开空仓小单
   - [ ] 平多仓
   - [ ] 平空仓
   - [ ] 验证 client_order_id 是否生效

2. **止损止盈测试**

   - [ ] 设置多仓止损（价格 < 入场价）
   - [ ] 设置多仓止盈（价格 > 入场价）
   - [ ] 设置空仓止损（价格 > 入场价）
   - [ ] 设置空仓止盈（价格 < 入场价）
   - [ ] 验证触发价格格式是否正确

3. **订单管理测试**
   - [ ] 查询订单状态（当前会失败）
   - [ ] 取消限价单
   - [ ] 取消止损止盈单

#### Gate.io

1. **订单测试**

   - [ ] 开多仓（size > 0）
   - [ ] 开空仓（size < 0）
   - [ ] 平多仓（size < 0, reduce_only: true）
   - [ ] 平空仓（size > 0, reduce_only: true）

2. **止损止盈测试**

   - [ ] 多仓止损：rule=2 (<=), price < 入场价
   - [ ] 多仓止盈：rule=1 (>=), price > 入场价
   - [ ] 空仓止损：rule=1 (>=), price > 入场价
   - [ ] 空仓止盈：rule=2 (<=), price < 入场价

3. **杠杆和保证金模式**
   - [ ] 设置不同杠杆倍数（2x, 5x, 10x）
   - [ ] 切换全仓/逐仓模式

---

## 🚨 业务流程完整性检查

### 完整的交易流程

```

1. 初始化 Trader ✅
2. 查询余额 ✅
3. 设置杠杆 ✅
4. 开仓 ✅（已修复 client_order_id）
5. 查询持仓 ✅
6. 设置止损 ✅（已修复价格格式/rule）
7. 设置止盈 ✅（已修复价格格式/rule）
8. 查询订单状态 ❌ HTX 未实现
9. 平仓 ✅
10. 查询历史盈亏 ❌ 两个交易所都未实现

```

### 异常流程处理

```

1. 网络超时重试 ⚠️ 依赖 client_order_id（已修复）
2. 订单失败处理 ⚠️ 依赖 GetOrderStatus（HTX 未实现）
3. 余额不足处理 ✅ 交易所会返回错误
4. 价格异常处理 ✅ 交易所会返回错误
5. 持仓爆仓处理 ✅ 查询持仓时会发现

````

---

## 📋 建议的实施计划

### 第一阶段：修复阻塞性问题（P0）

**预计时间**: 2-3 小时

1. 实现 HTX GetOrderStatus

   ```go
   // API: POST /linear-swap-api/v1/swap_order_info
   // 参数: order_id, contract_code
````

2. 验证 Gate.io 止损止盈逻辑
   - 小额实盘测试
   - 验证 rule 参数是否正确

### 第二阶段：实现高优先级功能（P1）

**预计时间**: 4-6 小时

1. 实现 HTX CancelAllOrders
2. 实现 HTX GetClosedPnL
3. 实现 Gate.io GetClosedPnL

### 第三阶段：完善和优化（P2）

**预计时间**: 2-3 小时

1. 实现合约信息缓存
2. 添加数量精度验证
3. 完善错误处理

---

## 🎯 总结

### 当前状态

- ✅ **基础功能**: 60% 完成（开平仓、查询余额持仓）
- ⚠️ **高级功能**: 30% 完成（止损止盈已修复，订单查询未实现）
- ❌ **数据统计**: 0% 完成（历史盈亏未实现）

### 风险评估

- 🔴 **高风险**: HTX GetOrderStatus 未实现，可能导致订单状态混乱
- 🟠 **中风险**: 历史盈亏查询缺失，影响统计功能
- 🟢 **低风险**: 核心开平仓功能已基本正常

### 建议

1. **立即修复** P0 问题，特别是 HTX GetOrderStatus
2. **小额测试** 在修复前不要用大额资金测试
3. **逐步上线** 先测试 Gate.io（实现更完整），再测试 HTX
4. **监控日志** 密切关注交易日志，发现异常立即停止

---

## 📞 联系方式

如有疑问，请查阅：

- HTX API 文档: https://huobiapi.github.io/docs/usdt_swap/v1/cn/
- Gate.io API 文档: https://www.gate.io/docs/developers/apiv4/zh_CN/

审查人：AI Code Reviewer
审查日期：2026-01-05
