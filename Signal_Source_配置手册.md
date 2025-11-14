# 📡 Signal Source Configuration 配置手册
## NOFX AI Trading System - 新手完全指南

---

## 🎯 什么是Signal Source？

**Signal Source（信号源）** 是NOFX AI交易系统的"情报中心"，负责向AI交易员提供实时市场数据和交易信号。

### 📊 信号源类型

NOFX支持**两种主要信号源**：

#### 1. **COIN POOL（币种池）**
- **功能**：提供高潜力币种排行榜
- **数据**：包含币种评分、涨幅、价格变化等
- **用途**：AI根据评分选择优质交易标的

#### 2. **OI TOP（持仓量排行榜）**
- **功能**：显示持仓量增长最快的币种
- **数据**：持仓量变化、资金流向等
- **用途**：识别市场热点和资金动向

---

## 🚀 快速开始配置

### 步骤1：访问配置界面

1. 登录NOFX系统
2. 进入 **AI交易员页面**
3. 点击右上角 **"📡 Signal Source"** 按钮
   ![Signal Source按钮位置](按钮位置示意图)

### 步骤2：配置信号源URL

#### **COIN POOL URL配置**

```
格式示例：
https://api.example.com/coinpool
```

**📝 配置方法**：
1. 在"COIN POOL URL"输入框中粘贴API地址
2. 确保URL以`http://`或`https://`开头
3. URL应返回JSON格式的币种数据

**✅ 正确配置示例**：
```
https://api.coingecko.com/coins/pool
https://api.binance.com/api/v3/coins
```

**❌ 常见错误**：
```
错误示例                   正确示例
---------                 ---------
api.example.com     →     https://api.example.com
localhost:3000      →     http://localhost:3000
/api/coins          →     https://yourdomain.com/api/coins
```

#### **OI TOP URL配置**

```
格式示例：
https://api.example.com/oitop
```

**📝 配置方法**：
1. 在"OI TOP URL"输入框中粘贴API地址
2. 确保URL可访问且返回有效数据
3. URL应返回JSON格式的持仓量数据

**✅ 正确配置示例**：
```
https://api.coinglass.com/openinterest/coin
https://api.binance.com/futures/data/top/long_short_account
```

### 步骤3：保存配置

1. 点击 **"Save"** 按钮
2. 看到成功提示："User signal source configuration saved"
3. 配置自动保存到系统

---

## 🔧 高级配置详解

### Trader配置中的信号源选项

创建或编辑AI交易员时，可以选择使用哪些信号源：

#### **选项1：启用COIN POOL**
- **位置**：Trader配置页面 → "Use COIN POOL Signal" 复选框
- **功能**：启用后，AI将根据币种池数据进行交易决策
- **推荐**：✅ 建议启用，获取优质币种信息

#### **选项2：启用OI TOP**
- **位置**：Trader配置页面 → "Use OI TOP Signal" 复选框
- **功能**：启用后，AI将参考持仓量变化调整策略
- **推荐**：✅ 建议启用，捕捉资金流向

#### **同时使用两个信号源（推荐）**
- 同时勾选两个选项
- AI将综合分析两种数据源
- 提供更全面的市场洞察

---

## 📡 API格式要求

### COIN POOL API响应格式

系统期望的JSON数据结构：

```json
{
  "success": true,
  "data": {
    "coins": [
      {
        "pair": "BTCUSDT",           // 交易对符号
        "score": 85.5,               // 评分（0-100）
        "start_time": 1609459200,    // 开始时间戳
        "start_price": 45000,        // 开始价格
        "last_score": 88.2,          // 最新评分
        "max_score": 92.5,           // 最高评分
        "max_price": 52000,          // 最高价格
        "increase_percent": 15.6     // 涨幅百分比
      }
    ],
    "count": 50
  }
}
```

**⚠️ 重要字段说明**：
- `pair`：必须格式为 `BASE+QUOTE`（如 BTCUSDT）
- `score`：数值越高，AI越倾向交易此币种
- 所有字段可选，但`pair`字段必须提供

### OI TOP API响应格式

系统期望的JSON数据结构：

```json
{
  "success": true,
  "data": {
    "positions": [
      {
        "symbol": "BTCUSDT",         // 交易对符号
        "oi": 1250000,               // 当前持仓量
        "oi_change": 15.8,           // 持仓量变化百分比
        "price": 47500,              // 当前价格
        "change_24h": 3.2            // 24小时价格变化
      }
    ],
    "count": 20
  }
}
```

**⚠️ 重要字段说明**：
- `symbol`：交易对符号（如 BTCUSDT）
- `oi_change`：持仓量增长百分比，正值表示资金流入
- 所有字段可选，但`symbol`字段必须提供

---

## 🎨 界面操作指南

### 主界面说明

```
┌─────────────────────────────────────────┐
│  AI交易员页面                           │
├─────────────────────────────────────────┤
│  📊 我的交易员    [创建交易员] [配置]    │
│                                         │
│  [📡 Signal Source] ← 点击这里配置信号源 │
│                                         │
│  ⚙️ 系统配置                          │
└─────────────────────────────────────────┘
```

### 信号源配置弹窗

```
┌─────────────────────────────────────────┐
│  📡 Signal Source Configuration         │
├─────────────────────────────────────────┤
│                                         │
│  COIN POOL URL                          │
│  ┌─────────────────────────────────────┐ │
│  │ https://api.example.com/coinpool    │ │ ← 输入框
│  └─────────────────────────────────────┘ │
│  📝 提供高潜力币种排行榜数据              │
│                                         │
│  OI TOP URL                             │
│  ┌─────────────────────────────────────┐ │
│  │ https://api.example.com/oitop       │ │ ← 输入框
│  └─────────────────────────────────────┘ │
│  📝 显示持仓量增长Top币种                │
│                                         │
│  ℹ️ 信号源配置信息：                      │
│     • 数据将用于AI交易决策                │
│     • 支持缓存机制提高响应速度           │
│     • 建议使用稳定的API服务              │
│                                         │
│           [Cancel]  [Save]              │
└─────────────────────────────────────────┘
```

### Trader配置中的信号源选项

```
┌─────────────────────────────────────────┐
│  创建交易员                             │
├─────────────────────────────────────────┤
│  基本配置                               │
│  ├─ 交易员名称：My Trader               │
│  ├─ AI模型：DeepSeek                    │
│  └─ 交易所：Binance Futures             │
│                                         │
│  信号源配置                             │
│  ☑️ Use COIN POOL Signal                │
│     └ 启用币种池数据                     │
│  ☑️ Use OI TOP Signal                   │
│     └ 启用持仓量排行榜                   │
│                                         │
│  📊 交易参数                            │
│  ├─ 初始资金：$10,000                   │
│  └─ 扫描间隔：3分钟                     │
│                                         │
│           [Create]  [Cancel]            │
└─────────────────────────────────────────┘
```

---

## 🔍 验证配置

### 方法1：查看日志

系统会记录信号源配置：

```
✓ 用户信号源配置已保存: user=user123, coin_pool=https://api.example.com/coinpool, oi_top=https://api.example.com/oitop
```

### 方法2：测试Trader运行

1. 创建启用信号源的Trader
2. 启动Trader
3. 查看日志确认数据获取：
   ```
   📋 [Trader-1] 使用币种池信号源：获取到50个币种
   📋 [Trader-1] 使用OI Top信号源：获取到20个持仓
   ```

### 方法3：浏览器检查

打开浏览器开发者工具（F12）：
- **Network面板**：查看API请求状态
- **Console面板**：查看数据获取日志

---

## 🛠️ 常见问题解决

### Q1：配置保存失败
**现象**：点击Save后提示错误

**原因**：
- URL格式不正确
- 权限不足
- 网络连接问题

**解决**：
1. ✅ 检查URL是否以`http://`或`https://`开头
2. ✅ 确认URL可访问（用浏览器打开测试）
3. ✅ 重新登录系统
4. ✅ 检查浏览器控制台错误信息

### Q2：Trader无法获取信号
**现象**：Trader启动但日志显示无数据

**原因**：
- API返回格式错误
- API服务不可用
- 信号源未启用

**解决**：
1. ✅ 验证API返回JSON格式正确
2. ✅ 检查API服务状态
3. ✅ 确认Trader配置中启用了信号源选项
4. ✅ 查看日志中的具体错误信息

### Q3：数据显示为空
**现象**：Dashboard显示无数据

**原因**：
- 首次使用无配置（正常现象）
- 信号源URL配置错误
- Trader未启动

**解决**：
1. ✅ 首次使用需先创建Trader（正常）
2. ✅ 确认信号源URL配置正确
3. ✅ 启动Trader才能产生交易数据
4. ✅ 等待3-5分钟让AI收集数据

### Q4：API请求超时
**现象**：日志显示请求超时

**原因**：
- API服务器响应慢
- 网络连接不稳定
- URL配置错误

**解决**：
1. ✅ 使用更快的API服务
2. ✅ 检查网络连接
3. ✅ 确认URL正确且服务正常
4. ✅ 系统有30秒超时保护

---

## 💡 最佳实践

### ✅ 推荐配置流程

1. **准备API服务**
   - 选择稳定的API提供商
   - 测试API响应速度和稳定性
   - 确认返回数据格式符合要求

2. **配置信号源**
   - 先配置COIN POOL（主要数据源）
   - 再配置OI TOP（辅助数据源）
   - 保存后测试连通性

3. **创建Trader**
   - 同时启用两个信号源
   - 设置合理的扫描间隔（建议3-5分钟）
   - 分配足够的初始资金

4. **监控运行**
   - 查看Trader日志确认数据获取
   - 检查Dashboard数据更新
   - 调整策略参数优化表现

### 📋 配置检查清单

**配置前检查**：
- [ ] API服务已准备并测试
- [ ] URL格式正确（http/https）
- [ ] 返回JSON格式数据
- [ ] 包含必需字段（pair/symbol）

**配置后验证**：
- [ ] 配置保存成功
- [ ] Trader能获取到数据
- [ ] 日志显示正常
- [ ] Dashboard显示数据

### ⚠️ 注意事项

1. **数据格式严格**
   - 字段名必须完全匹配
   - 数据类型必须正确
   - 缺失字段将导致数据丢失

2. **API稳定性**
   - 选择可靠的API提供商
   - 监控API服务状态
   - 备用多个API源

3. **性能考虑**
   - API请求有30秒超时
   - 系统有缓存机制避免重复请求
   - 合理设置扫描间隔

---

## 📚 参考资料

### 推荐API提供商

**免费选项**：
- CoinGecko API (https://www.coingecko.com/en/api)
- Binance API (https://binance-docs.github.io/apidocs/)
- CoinMarketCap API (https://coinmarketcap.com/api/)

**付费选项**：
- Alpha Vantage
- Polygon.io
- Quandl

### 示例API地址

```
COIN POOL示例：
https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&order=market_cap_desc&per_page=50&page=1&sparkline=false

OI TOP示例：
https://fapi.binance.com/futures/data/globalLongShortAccountRatio?symbol=BTCUSDT&period=1h
```

### 更多帮助

- **系统日志**：查看Trader运行日志了解详细信息
- **技术支持**：访问项目GitHub提交Issue
- **社区讨论**：加入用户群组交流经验

---

## 📖 附录

### 字段详细说明

#### COIN POOL字段
| 字段名 | 类型 | 说明 | 示例 |
|--------|------|------|------|
| pair | string | 交易对符号 | "BTCUSDT" |
| score | float | 评分（0-100） | 85.5 |
| increase_percent | float | 涨幅百分比 | 15.6 |
| max_score | float | 最高评分 | 92.5 |
| start_time | int64 | 开始时间戳 | 1609459200 |

#### OI TOP字段
| 字段名 | 类型 | 说明 | 示例 |
|--------|------|------|------|
| symbol | string | 交易对符号 | "BTCUSDT" |
| oi | float | 当前持仓量 | 1250000 |
| oi_change | float | 持仓量变化% | 15.8 |
| price | float | 当前价格 | 47500 |

### 错误码说明

| 错误信息 | 含义 | 解决方案 |
|----------|------|----------|
| URL格式错误 | URL不以http/https开头 | 添加协议前缀 |
| 保存失败 | 后端错误 | 重试或联系技术支持 |
| API响应错误 | 数据格式不匹配 | 检查JSON结构 |
| 超时 | 请求时间过长 | 使用更快的API |

---

**📅 文档版本**：v1.0
**🎯 最后更新**：2025-11-14
**👨‍💻 适用版本**：NOFX AI Trading System v1.0+

---

**🎓 恭喜！您已掌握Signal Source配置的全部知识！**

现在您可以：
- ✅ 配置COIN POOL和OI TOP信号源
- ✅ 创建并启用信号的AI交易员
- ✅ 监控和优化交易表现
- ✅ 解决常见配置问题

开始您的AI交易之旅吧！ 🚀
