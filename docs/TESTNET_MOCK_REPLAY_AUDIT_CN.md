# 测试网 / Mock / Replay 支持盘点（2026-03-25）

> 状态：盘点完成，能力不完整
> 目标：确认 `nofxmax` 当前是否具备可用于主线交付验证的 testnet / mock / replay 支撑，而不是只停留在零散测试文件层面。

---

## 1. 结论先说

当前仓库 **具备部分 testnet 与 mock 支撑**，但 **尚不具备完整可交付的 replay / paper-trading / 仿真验证体系**。

可以归类为：

- **Testnet：部分支持**
- **Mock：中等支持**
- **Replay：基本缺失（仅研究文档提及，未形成产品/测试闭环）**

因此，这一项不能简单写成“有”或“没有”，更准确的结论是：

> **当前具备开发级测试支撑基础，但还不具备完整的交易保护主线验证环境。**

---

## 2. Testnet 支持现状

### 2.1 已确认存在 testnet 开关 / 代码路径

代码和文档中可以确认的 testnet 支持包括：

- `trader/auto_trader.go`
  - `HyperliquidTestnet`
  - `LighterTestnet`
- `store/exchange.go`
  - 交易所配置持久化中已有 `Testnet bool`
- `trader/hyperliquid/trader.go`
  - 根据 `testnet` 切换 `hyperliquid.TestnetAPIURL`
- `trader/hyperliquid/trader_orders.go`
  - 下单相关路径按 `isTestnet` 切换 API URL
- `trader/lighter/trader.go`
  - 存在 testnet URL 与链 ID 配置
- 文档：
  - `SECURITY.md`
  - `docs/getting-started/README.md`
  - `docs/community/*`
  都明确提到“先在 testnet 测试”

### 2.2 Testnet 支持的现实问题

虽然有 testnet 开关，但当前还不能认为“测试网交付面完整”，原因是：

1. **不是所有交易所能力都明确完成 testnet 验证**
2. **Protection Phase 2 / Phase 3 没有独立 testnet 验证矩阵**
3. **没有统一文档说明每个交易所哪些能力能在 testnet 覆盖**
4. **没有一套标准化 testnet smoke / regression 流程**

结论：

- **有 testnet 入口**
- **但没有 testnet 级别的系统化验证闭环**

---

## 3. Mock 支持现状

### 3.1 已有 mock / httptest 基础

仓库中已有较明显的 mock 测试基础：

- `trader/binance/futures_test.go`
  - 使用 `httptest.Server` 模拟 Binance Futures API
- `trader/gate/trader_test.go`
  - mock server 验证 Gate 交易接口
- `trader/aster/trader_test.go`
  - mock server 验证 Aster 接口
- `trader/lighter/orders_test.go`
  - mock response / mock server
- `telegram/agent/agent_test.go`
  - `mockLLM` + `mockAPIServer`
- `mcp/mock_test.go`
  - MCP 层 mock 支持
- `trader/testutil/test_suite.go`
  - 统一测试套件骨架

### 3.2 Mock 支撑的现实问题

当前 mock 更偏“模块单测 / 接口单测”，还不是“主线交付 mock 环境”：

1. **交易保护链路没有完整 mock 场景矩阵**
   - 开仓成功 + 止损失败
   - 开仓成功 + 止盈失败
   - drawdown partial close
   - break-even 改单失败
   - ladder 多层触发与撤单仲裁
2. **缺少统一 fake trader / fake exchange 能力层**
3. **跨模块集成验证仍依赖真实逻辑拼接，不够隔离**
4. **前端 protection Phase 2 还没有针对复杂策略配置的专项 mock/集成测试**

结论：

- **已有 mock 测试基础**
- **但还不是围绕主线交付目标设计的 mock 验证体系**

---

## 4. Replay / Paper Trading 现状

### 4.1 已发现的内容

目前找到的 replay / simulation 相关内容主要在：

- `docs/research/AI-Trader-Analysis-Report.md`
  - 有 simulation / replay 概念性设计
- 若干测试中存在 “simulate” 字样
  - 但多为单个场景模拟，不是产品能力

### 4.2 缺失点

当前仓库 **没有发现** 下列成熟能力：

1. **统一 replay engine**
2. **基于历史行情驱动的策略回放执行器**
3. **paper trading / simulated execution 模式**
4. **保护链路在 replay 环境下的结果回放分析**
5. **用于验收的 replay 数据集 / fixtures 目录规范**

结论：

- **Replay 在研究文档层面有概念**
- **在工程交付层面基本还没形成**

---

## 5. 对当前主线任务的意义

这次盘点对当前主线的直接意义是：

### 已经可以做的
- 继续推进 Protection Phase 2 / 3 代码落地
- 用现有 Go 单测 + mock server 做模块级验证
- 借已有 testnet 开关做后续真实环境 smoke test 预留

### 还不能假设已经有的
- 不能说项目已有完整 replay 验证闭环
- 不能说 protection 新能力已经有统一 testnet / mock / replay 验证矩阵
- 不能把零散 mock 单测误判为完整交付级仿真能力

---

## 6. 建议的后续动作

### P1：低成本补强
1. 为 `Protection Phase 2` 新增专项测试矩阵文档
2. 为 drawdown / break-even / ladder 增加 fake trader 单测
3. 补一份“交易所 testnet 支持能力表”

### P2：中成本补强
1. 引入统一 fake trader / fake exchange harness
2. 做 protection 生命周期集成测试
3. 前端补 protection phase 2 配置回归测试

### P3：更完整交付面
1. 建立 replay 数据目录规范
2. 建立 paper-trading / simulated execution 模式
3. 用 replay 验证 AI 决策 + protection 执行 + 持仓期保护全链路

---

## 7. 当前判定

对待办项“确认是否存在测试网 / mock / replay 数据支持”的当前结论：

- **测试网：有部分支持**
- **mock：有基础支持**
- **replay：暂无成型支持**
- **综合结论：部分具备，但不完整，不能视作已满足主线交付要求**
