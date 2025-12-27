# 规范：Trader加载和性能数据访问

**版本**: 1.0
**更新日期**: 2025-12-27
**相关提案**: TRADER-NOT-FOUND-FIX

---

## 1. 概述

本规范定义了Trader在创建、加载到内存和访问性能数据时的行为，确保新创建的trader能够立即被使用。

---

## 2. Trader生命周期

```
Database              TraderManager Memory      API Access
(Persistent)          (Runtime)                (User-facing)

1. Create
   [DB CREATE]
        ↓
2. Load to Memory
   [DB QUERY] → [Load to tm.traders] → [in memory]
        ↓
3. Access
   [Query from memory] → [GetTrader(ID)]
```

---

## 3. CreateTrader流程

### 3.1 API请求
```
POST /api/traders
{
  "name": "My Trader",
  "ai_model_id": "deepseek",
  "exchange_id": "okx",
  ...
}
```

### 3.2 后端处理流程

#### 步骤1: 验证请求
- 验证必填字段存在
- 验证leverage范围
- 验证trading_symbols格式

#### 步骤2: 生成ID
```go
traderID := fmt.Sprintf("%s_%s_%d",
    req.ExchangeID,      // e.g., "okx"
    req.AIModelID,       // e.g., "deepseek"
    time.Now().Unix()    // timestamp
)
// Result: "okx_deepseek_1766800370"
```

#### 步骤3: 持久化到数据库
```go
trader := &config.TraderRecord{
    ID:           traderID,
    UserID:       userID,        // from auth context
    Name:         req.Name,
    AIModelID:    req.AIModelID,
    ExchangeID:   req.ExchangeID,
    ...
}
err := h.Database.CreateTrader(trader)  // INSERT
```

#### 步骤4: 加载到内存
```go
err := h.TraderManager.LoadUserTraders(h.Database, userID)
```

**规范**:
- ✅ LoadUserTraders必须被调用
- ✅ 失败不应该返回错误给用户（trader已在DB中）
- ✅ 但应该记录详细日志

#### 步骤5: 验证加载成功 ⭐ NEW
```go
_, err := h.TraderManager.GetTrader(traderID)
if err != nil {
    // Trader创建成功但加载失败
    return error: "trader created but failed to load: ..."
}
```

**规范**:
- ✅ 必须验证trader确实被加载
- ✅ 如果加载失败，应该返回500错误，不是201
- ✅ 前端应该根据响应重试或通知用户

#### 步骤6: 返回响应
```json
{
  "trader_id": "okx_deepseek_1766800370",
  "trader_name": "My Trader",
  "ai_model": "deepseek",
  "is_running": false
}
```

---

## 4. LoadUserTraders流程

### 4.1 输入
```go
func LoadUserTraders(database *Database, userID string) error
```

### 4.2 处理流程

#### 步骤1: 获取用户的所有traders
```go
traders, err := database.GetTraders(userID)  // SQL query
```

#### 步骤2: 对每个trader执行加载

**旧行为（有问题）**:
```
for each trader:
    获取AI模型配置
    ❌ if 模型不存在 { SKIP }
    获取交易所配置
    ❌ if 交易所不存在 { SKIP }
    加载trader到内存
```

**新行为（修复）**:
```
for each trader:
    尝试获取AI模型配置
    ⚠️ if 模型不存在 { LOG WARN but CONTINUE }
    尝试获取交易所配置
    ⚠️ if 交易所不存在 { LOG WARN but CONTINUE }
    加载trader到内存（即使config不完整）
    如果加载失败 { LOG ERROR }
```

#### 步骤3: 加载单个trader

```go
func loadSingleTrader(traderCfg, aiModelCfg, exchangeCfg, ...)
    // 创建AutoTrader实例
    // 初始化配置（使用nil-safe处理）
    // 添加到tm.traders[ID]
```

**规范**:
- ✅ 即使aiModelCfg为nil，也应该创建trader
- ✅ 在trader.Run()时做真正的配置检查
- ✅ 添加防御代码处理nil config

### 4.3 返回

```go
return nil  // 总是成功返回
```

**规范**:
- ✅ LoadUserTraders应该 graceful fail
- ✅ 部分trader加载失败不应该中止整个过程
- ✅ 记录所有错误但继续处理其他trader

---

## 5. GetPerformance流程 (带重试)

### 5.1 API请求
```
GET /api/performance?trader_id=okx_deepseek_1766800370
```

### 5.2 后端处理流程

#### 步骤1: 获取trader从查询参数
```go
_, traderID, err := h.getTraderFromQuery(c)
```

#### 步骤2: 从内存获取trader
```go
trader, err := h.TraderManager.GetTrader(traderID)
```

**新行为（添加重试）**:
```go
trader, err := h.TraderManager.GetTrader(traderID)

// ⭐ 如果不存在，尝试重新加载
if err != nil {
    log.Printf("⏳ Trader未在内存中 %s，尝试加载...", traderID)
    userID := c.GetString("user_id")
    h.TraderManager.LoadUserTraders(h.Database, userID)

    // 再试一次
    trader, err = h.TraderManager.GetTrader(traderID)

    if err != nil {
        // 仍然找不到
        return 404: "trader not found or config missing"
    }
}
```

**规范**:
- ✅ 一次重试（仅一次，避免循环）
- ✅ 记录重试尝试
- ✅ 重试失败时返回详细错误

#### 步骤3: 分析性能
```go
performance, err := trader.GetDecisionLogger().AnalyzePerformance(100)
```

#### 步骤4: 返回结果
```json
{
  "total_trades": 5,
  "winning_trades": 3,
  "losing_trades": 2,
  "win_rate": 0.6,
  ...
}
```

---

## 6. 错误处理

### 6.1 CreateTrader错误

| 错误 | HTTP | 消息 |
|------|------|------|
| 验证失败 | 400 | "Invalid leverage/symbols: ..." |
| DB插入失败 | 500 | "Failed to create trader: ..." |
| 内存加载失败 | 500 | "Trader created but failed to load: ..." |

### 6.2 GetPerformance错误

| 错误 | HTTP | 消息 |
|------|------|------|
| Trader不存在 | 404 | "Trader not found or config missing: ..." |
| 分析失败 | 500 | "Failed to analyze performance: ..." |

---

## 7. 配置不完整时的行为

### 场景：AI模型配置缺失

当创建trader时，选择的AI模型在用户的配置中不存在：

```
1. CreateTrader: 成功创建到DB ✅
2. LoadUserTraders:
   - 获取AI模型配置 → 找不到
   - ⚠️ LOG WARN
   - 继续加载trader ✅
3. trader.Run():
   - 尝试获取AI模型配置
   - ❌ 失败，返回错误
   - Trader停止运行
```

**用户体验**:
1. 创建trader成功
2. Trader可以在UI中看到
3. 尝试启动时失败，错误信息提示检查配置

---

## 8. 并发安全

### TraderManager并发访问

```go
type TraderManager struct {
    traders map[string]*AutoTrader  // protected by mu
    mu      sync.RWMutex
}
```

**规范**:
- ✅ LoadUserTraders持有写锁 (mu.Lock)
- ✅ GetTrader只需读锁 (mu.RLock)
- ✅ 多个GetPerformance可以并发执行

### Database并发访问

```go
// Neon serverless with connection pooling
PostgreSQL (with tx support)
```

**规范**:
- ✅ 数据库连接池管理并发
- ✅ CreateTrader使用tx确保原子性
- ✅ GetTraders使用事务隔离

---

## 9. 日志规范

### LoadUserTraders
```
INFO:  "📋 为用户 {userID} 加载交易员配置: {count} 个"
WARN:  "⚠️ 交易员 {name} 的AI模型 {id} 不存在，继续加载"
WARN:  "⚠️ 交易员 {name} 的AI模型 {id} 未启用，继续加载"
ERROR: "❌ 加载交易员 {name} 失败: {err}"
```

### GetPerformance重试
```
INFO:  "⏳ Trader在内存中未找到 {id}，尝试重新加载..."
INFO:  "✓ Trader {id} 重新加载成功"
ERROR: "❌ Trader {id} 仍未找到: {err}"
```

---

## 10. 测试用例

### UC1: 正常创建和访问
```
1. CreateTrader("My Trader", "deepseek", "okx")
   → 200 OK, trader_id returned
2. GetPerformance(trader_id)
   → 200 OK, performance data or empty
```

### UC2: 缺失AI模型配置
```
1. CreateTrader("Trader", "unknown_model", "okx")
   → 500 "failed to load"
2. 用户修复配置
3. 系统重试或用户重新启动
   → GetPerformance成功
```

### UC3: 并发创建多个traders
```
1. CreateTrader × 5 (concurrently)
   → All succeed
2. LoadUserTraders called automatically
   → All 5 traders in memory
3. GetPerformance × 5 (concurrently)
   → All succeed
```

---

## 11. 性能考虑

### LoadUserTraders性能
- 时间复杂度: O(n) where n = number of traders
- 空间复杂度: O(n) in tm.traders map
- DB查询: 2 queries per trader (models, exchanges)

**优化**:
- ✅ 使用缓存避免重复DB查询
- ✅ 批量加载而不是逐个查询

### GetPerformance重试性能
- 额外DB查询: 1 (only on first miss)
- 额外内存操作: O(1)
- 网络延迟: 1 额外往返

**优化**:
- ✅ 仅重试一次，避免循环
- ✅ 前端应该缓存结果

---

## 12. 未来改进

1. **自动重试策略**
   - 指数退避重试
   - 最大重试次数限制

2. **配置热加载**
   - 不需要重启就能更新配置
   - 通知已加载的traders

3. **健康检查**
   - 定期验证trader的配置
   - 自动修复可修复的问题

4. **监控和告警**
   - 追踪加载失败率
   - 告警关键trader问题

---

**文档完成**: 2025-12-27
