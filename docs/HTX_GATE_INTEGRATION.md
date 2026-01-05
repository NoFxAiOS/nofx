# HTX 和 Gate.io 交易所集成

本次更新为 NOFX 系统添加了 HTX (火币) 和 Gate.io 两个交易所的完整支持。

## 📋 更新内容

### 后端实现

#### 1. 新增交易所实现文件

- **`trader/htx_trader.go`** - HTX (火币) 合约交易实现

  - 完整实现 `Trader` 接口
  - 支持开多、开空、平仓操作
  - 支持止损止盈设置
  - 包含账户余额和持仓查询
  - 内置请求缓存机制（15 秒）
  - 支持 GZIP 压缩响应

- **`trader/gate_trader.go`** - Gate.io 合约交易实现
  - 完整实现 `Trader` 接口
  - 支持全仓/逐仓模式切换
  - 支持动态杠杆设置
  - 完整的订单管理（开仓、平仓、止损、止盈）
  - SHA512 签名算法实现

#### 2. 核心模块集成

**`trader/auto_trader.go`**

```go
// 新增配置字段
HTXAPIKey    string
HTXSecretKey string
GateAPIKey    string
GateSecretKey string

// 新增交易所初始化逻辑
case "htx":
    trader = NewHTXTrader(config.HTXAPIKey, config.HTXSecretKey)
case "gate":
    trader = NewGateTrader(config.GateAPIKey, config.GateSecretKey)
```

**`api/server.go`**

- 在 Trader 创建 API 中添加 HTX 和 Gate.io 的初始化逻辑
- 支持自动查询交易所真实余额

**`manager/trader_manager.go`**

- 在 Trader 加载逻辑中添加 HTX 和 Gate.io 的 API 密钥配置

### 前端实现

#### 1. 交易所图标支持

**`web/src/components/ExchangeIcons.tsx`**

```typescript
const ICON_PATHS: Record<string, string> = {
  // ...
  htx: "/exchange-icons/htx.svg",
  gate: "/exchange-icons/gate.svg",
  // ...
};
```

#### 2. 交易所配置界面

**`web/src/components/traders/ExchangeConfigModal.tsx`**

```typescript
const SUPPORTED_EXCHANGE_TEMPLATES = [
  // ...
  { exchange_type: "htx", name: "HTX (Huobi) Futures", type: "cex" },
  { exchange_type: "gate", name: "Gate.io Futures", type: "cex" },
  // ...
];

const exchangeRegistrationLinks = {
  // ...
  htx: {
    url: "https://www.htx.com/invite/en-us/1f?invite_code=6xyq8223",
    hasReferral: true,
  },
  gate: {
    url: "https://www.gate.io/signup/AgBFAApb?ref_type=103",
    hasReferral: true,
  },
  // ...
};
```

## 🔧 技术实现细节

### HTX (火币) 特点

1. **API 签名算法**
   - HMAC-SHA256
   - 签名格式：`method + "\n" + host + "\n" + path + "\n" + sortedParams`
   - 参数需按字母顺序排序
2. **响应处理**
   - 支持 GZIP 压缩
   - 统一的错误处理机制
3. **订单类型**
   - 使用 `optimal_20` 作为市价单类型
   - 支持多空双向持仓

### Gate.io 特点

1. **API 签名算法**

   - HMAC-SHA512
   - 需要对 body 进行 SHA512 哈希
   - 完整签名格式：`method + "\n" + path + "\n" + query + "\n" + bodyHash + "\n" + timestamp`

2. **持仓模式**

   - 使用正数表示多仓，负数表示空仓
   - 支持 `reduce_only` 标记平仓操作

3. **订单管理**
   - 支持价格触发订单（止损/止盈）
   - 订单状态查询返回详细信息

## 📊 支持的功能

### 交易功能

- ✅ 开多仓（OpenLong）
- ✅ 开空仓（OpenShort）
- ✅ 平多仓（CloseLong）
- ✅ 平空仓（CloseShort）
- ✅ 设置杠杆（SetLeverage）
- ✅ 设置保证金模式（SetMarginMode）
- ✅ 止损设置（SetStopLoss）
- ✅ 止盈设置（SetTakeProfit）

### 查询功能

- ✅ 账户余额查询（GetBalance）
- ✅ 持仓信息查询（GetPositions）
- ✅ 市场价格查询（GetMarketPrice）
- ✅ 订单状态查询（GetOrderStatus）

### 订单管理

- ✅ 取消所有订单（CancelAllOrders）
- ✅ 取消止损订单（CancelStopLossOrders）
- ✅ 取消止盈订单（CancelTakeProfitOrders）
- ✅ 取消条件单（CancelStopOrders）

## 🚀 使用方法

### 1. 添加交易所账户

在配置页面：

1. 点击"添加交易所"
2. 选择"HTX (Huobi) Futures"或"Gate.io Futures"
3. 输入 API Key 和 Secret Key
4. 保存配置

### 2. 创建 Trader

在 Traders 页面：

1. 点击"创建 Trader"
2. 选择刚添加的 HTX 或 Gate.io 账户
3. 配置 AI 模型和策略
4. 启动交易

### 3. 获取 API 密钥

**HTX (火币)**

1. 访问 https://www.htx.com
2. 进入 API 管理页面
3. 创建新的 API Key
4. 权限设置：启用"合约交易"权限
5. 绑定 IP 白名单（推荐）

**Gate.io**

1. 访问 https://www.gate.io
2. 进入 API 管理页面
3. 创建新的 API Key
4. 权限设置：启用"合约交易"权限
5. 绑定 IP 白名单（推荐）

## ⚠️ 注意事项

1. **API 权限**

   - 只需开启"合约交易"权限
   - 不要开启"提币"权限
   - 建议绑定服务器 IP 白名单

2. **测试建议**

   - 首次使用建议小额测试
   - 确认 API 配置正确后再加大资金
   - 监控初期交易情况

3. **风险提示**
   - 合约交易有爆仓风险
   - AI 交易不保证盈利
   - 请谨慎设置杠杆倍数
   - 建议设置止损保护

## 🔄 后续计划

- [ ] 添加更详细的交易所配置指南
- [ ] 优化错误处理和重试机制
- [ ] 添加 WebSocket 实时行情支持
- [ ] 完善 GetClosedPnL 历史盈亏查询
- [ ] 添加交易所特定的风控参数

## 📝 版本信息

- **添加日期**: 2026-01-05
- **支持版本**: NOFX v1.0+
- **API 版本**:
  - HTX: Linear Swap API v1
  - Gate.io: Futures API v4

## 🤝 贡献

如果在使用过程中遇到问题或有改进建议，欢迎提交 Issue 或 PR。

---

**注意**: 本次集成已完成核心交易功能，可以立即用于实盘交易。部分高级功能（如历史盈亏详情）将在后续版本中完善。
