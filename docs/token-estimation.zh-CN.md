# 📊 Token 估算分析与候选币种上限指南

> 版本：v1.0 | 更新：2026-03-27
> 适用：策略配置 · 模型选择 · 候选币种数量决策

---

## 目录

- [Token 估算公式](#-token-估算公式)
- [系统提示词的准确性分析](#-系统提示词的准确性分析)
- [典型配置下的安全币种数量](#-典型配置下的安全币种数量)
- [模型上限参考](#-模型上限参考)
- [MaxCandidateCoins 常量说明](#-maxcandidatecoins-常量说明)

---

## 📐 Token 估算公式

代码入口：`store/strategy.go` → `EstimateTokens()`

整体结构：

```
total = (staticTokens + N × perCoinTokens) × 1.15
```

其中 `1.15` 为 15% 安全边际。

### 静态部分（与候选币数量无关）

```
SystemPrompt  = baseChars / 2（zh）或 / 4（en）
                baseChars ≈ 3000（zh）/ 4000（en）+ 自定义提示段落长度

FixedOverhead = 200 tokens（时间戳、账户信息、章节标题）

RankingData   = (OILimit × 60 + NetFlowLimit × 80 + PriceLimit × durations × 40) / 4

staticTokens  = SystemPrompt + FixedOverhead + RankingData
              ≈ 1500 + 200 + 650 = 2350 tokens（默认中文配置）
```

### 每枚币的 Token 开销

```
# 每行指标额外字符数（I）
I = EnableEMA×20 + EnableMACD×30 + EnableRSI×15
  + EnableATR×15 + EnableBOLL×25 + EnableVolume×10

# 每枚币的市场数据 token
marketPerCoin = (T × K × (80 + I) + 100) / 4
                ↑ T=时间框架数  K=每TF K线数
                ↑ 100 = OI + 资金费率固定开销

# 每枚币的量化数据 token
quantPerCoin  = (EnableQuantOI×300 + EnableQuantNetflow×300) / 4

perCoinTokens = marketPerCoin + quantPerCoin
```

### 反向公式：最大安全币数

```
budget       = modelContextLimit × 0.80 / 1.15
maxSafeCoins = floor((budget - staticTokens) / perCoinTokens)
```

---

## 📊 典型配置下的安全币种数量

**基准：131K 模型（DeepSeek / Grok / Qwen）**，80% 警戒线

### 三种配置的 perCoinTokens

| 配置                                       | T   | K   | I   | quantPerCoin | perCoinTokens |
| ------------------------------------------ | --- | --- | --- | ------------ | ------------- |
| **最小**（单TF，无指标，无量化）           | 1   | 10  | 0   | 0            | **225**       |
| **默认**（3TF，仅Volume，QuantOI+Netflow） | 3   | 20  | 10  | 600          | **1525**      |
| **最大**（4TF，全部指标，全量化）          | 4   | 30  | 115 | 600          | **6025**      |

### 各模型下的最大安全币数

| 模型上限                       | 最小配置    | 默认配置        | 最大配置    |
| ------------------------------ | ----------- | --------------- | ----------- |
| 131K（DeepSeek / Grok / Qwen） | ≥50（封顶） | **58**          | **14**      |
| 128K（OpenAI GPT-4）           | ≥50（封顶） | **57**          | **14**      |
| 200K（Claude）                 | ≥50（封顶） | **89 → 封顶50** | **22**      |
| 1M（Gemini / Minimax）         | ≥50（封顶） | ≥50（封顶）     | ≥50（封顶） |

---

## 🤖 模型上限参考

来源：`store/strategy.go` → `ModelContextLimits`

| 模型     | Context 上限 | 80% 警戒线 |
| -------- | ------------ | ---------- |
| deepseek | 131,072      | 104,858    |
| openai   | 128,000      | 102,400    |
| claude   | 200,000      | 160,000    |
| qwen     | 131,072      | 104,858    |
| gemini   | 1,000,000    | 800,000    |
| grok     | 131,072      | 104,858    |
| kimi     | 131,072      | 104,858    |
| minimax  | 1,000,000    | 800,000    |

---

## 🔒 MaxCandidateCoins 常量说明

来源：`store/strategy.go` 第 14-20 行

```go
const (
    MaxCandidateCoins = 50   // UI 硬限制：用户最多设定的候选币数量
    MaxPositions      = 3    // 最大同时持仓数
    MaxTimeframes     = 4    // 最大时间框架数
    MinKlineCount     = 10   // 最少 K 线数
    MaxKlineCount     = 30   // 最多 K 线数
)
```

### 为什么 MaxCandidateCoins = 50？

- **默认配置**下 50 枚币约用 **~8,000 tokens**（~6% of 131K），完全安全
- **极端配置**（4TF + 全指标）50 枚币会超过 131K 限制，但 **runtime token-blocking** 会在分析前拦截并报错
- 因此 50 是合理的 UI 上限：一方面给用户足够灵活性，另一方面依赖运行时保护防止真正的溢出

### 建议使用范围

| 用户类型            | 建议配置                | 最大建议币数 |
| ------------------- | ----------------------- | ------------ |
| 新手 / 使用默认配置 | 3TF, K=20, 仅 Volume    | 10-20 枚     |
| 进阶 / 启用部分指标 | 3TF, K=20, EMA+MACD+RSI | 10-15 枚     |
| 高级 / 全部指标     | 3-4TF, K=20-30, 全指标  | 5-10 枚      |
