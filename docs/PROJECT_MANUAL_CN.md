# NOFX 项目详细说明书（审阅版）

> 文档目的：为项目所有者、后续开发者、维护者、评审者提供一份较完整的项目说明书，覆盖项目定位、架构、模块职责、主要功能、关键链路、风险边界与当前交付状态。
> 适用仓库：`MAX-LIUS/nofxmax`
> 当前参考分支：`fox/project-takeover-baseline`
> 更新时间：2026-03-23

---

# 1. 项目概述

## 1.1 项目名称
- NOFX / nofxmax

## 1.2 项目定位
NOFX 是一个面向交易场景的 **AI Trading OS（AI 交易操作系统）**。它不是单一策略脚本，也不是单一交易所 bot，而是一个围绕“策略配置 → AI 决策 → 风控约束 → 交易执行 → 数据展示 → 持续扩展”构建的平台型系统。

## 1.3 核心价值主张
NOFX 的核心价值不只是“自动下单”，而是提供一套完整的可组合交易平台能力：

1. **多模型接入**
   - 支持多种 LLM / AI Provider
   - 支持传统 API Key 模式
   - 支持 x402 / 钱包支付模式

2. **多交易所接入**
   - 支持中心化交易所（CEX）
   - 支持去中心化永续交易所（Perp-DEX）
   - 支持不同市场基础设施的统一接入

3. **策略驱动型交易**
   - 用户通过策略配置决定数据来源、指标、风险参数、提示词结构
   - AI 在策略约束之下分析市场并输出决策

4. **图形化控制台**
   - 模型配置
   - 交易所配置
   - Trader 管理
   - Dashboard
   - Competition
   - Strategy Studio
   - Strategy Market

5. **可扩展平台能力**
   - Telegram Agent
   - x402 支付
   - 数据与指标扩展
   - 多市场扩展（Crypto / Stocks / Forex / Metals）

## 1.4 当前阶段定位
当前仓库已不处于“原型脚本期”，而是平台化阶段。当前接管阶段的主要目标不是立刻做大规模新功能，而是：

- 清理外部问题
- 建立统一认知
- 稳定测试与构建基线
- 固化接管文档与项目记忆
- 在低风险前提下优化前端交付体验
- 为后续二次开发做好承接基础

---

# 2. 技术栈与运行时组成

## 2.1 后端技术栈
- Go
- Gin（HTTP API）
- Store / Manager / Trader 分层结构
- SQLite / Postgres（按配置）
- JWT 鉴权
- Telegram Bot

## 2.2 前端技术栈
- React
- TypeScript
- Vite
- SWR
- Tailwind CSS
- Zustand（项目内已使用）
- Lazy Loading + Manual Chunks（已做首轮优化）

## 2.3 AI / 模型接入层
- MCP / Provider 架构
- 多模型支持
- OpenAI-compatible 模式
- x402 钱包支付链路

## 2.4 交易层
- 多交易所适配器
- AutoTrader 通用逻辑
- Position / Order / Equity / Decision 等领域数据管理

## 2.5 数据与指标层
- 市场数据装配
- 技术指标开关与配置
- 第三方 provider 数据接入

---

# 3. 顶层架构说明

## 3.1 结构概览
NOFX 可以概括为以下分层：

```text
用户 / 运维 / 管理员
  ↓
React 前端 / Telegram Bot
  ↓
Gin API
  ↓
TraderManager / AutoTrader / Kernel / Service Logic
  ↓
AI Provider / MCP / Market Provider / Exchange Adapter
  ↓
Store（数据库持久层）
```

## 3.2 顶层职责划分
- **前端层**：负责配置、展示、控制、交互
- **API 层**：负责将前端/机器人请求转为系统内部调用
- **Manager 层**：负责 trader 生命周期管理
- **Kernel 层**：负责策略语义、Prompt 构建、分析上下文组织
- **Trader 层**：负责执行交易逻辑、与交易所适配器协作
- **Provider/MCP 层**：负责模型与外部数据源调用
- **Store 层**：负责持久化和领域对象读写

---

# 4. 系统主链路说明

项目的核心不是单一页面，而是几条贯穿式主链路。

## 4.1 配置链
配置链是平台化的起点。

### 涉及对象
- AI 模型配置
- 交易所配置
- 策略配置
- Trader 配置
- Telegram 配置
- 系统加密配置

### 流程
用户在前端配置 → API 接口写入 store → 系统运行期读取这些配置 → Manager / Trader / Kernel 使用这些配置驱动行为

### 价值
配置链决定平台是否“可组合”。如果配置链不稳定，后续所有能力都会变得不可信。

## 4.2 决策链
决策链是 AI 交易系统的核心。

### 流程
市场数据 + 策略配置 → 组装分析上下文 → Prompt 构建 → 调用模型 → 解析响应 → 形成决策 → 决策落库 → 后续执行

### 决策输出内容可能包括
- 是否开仓
- 多空方向
- 建议仓位
- 风险判断
- 市场状态判断
- 继续持有/等待/平仓等动作

### 当前结论
决策链认知骨架已建立，但仍然是后续必须持续深化审查的高风险主链之一。

## 4.3 交易链
交易链是“把决策落地成真实市场动作”的链路。

### 流程
AI 决策 → Trader 执行器 → 交易所适配器 → 下单/平仓/同步 → 持仓/订单/权益更新 → 数据写入 store → 前端展示

### 高风险点
- 交易所 API 差异
- 状态同步延迟或不一致
- 下单结果与本地状态不一致
- 持仓重建与历史统计偏差

## 4.4 风控链
风控链不是单一模块，而是横切系统的约束链。

### 涉及内容
- 杠杆限制
- 最大仓位
- 保证金使用率
- 网格风险
- 强平相关风险
- 策略层风险参数
- 行为边界（哪些操作允许自动做，哪些不应该默认自动做）

### 当前结论
风控链是后续“系统可信性结项”的关键部分，目前仍属于重点未完全收口区。

## 4.5 展示链
展示链负责把 store 中的数据和运行状态转成前端可见信息。

### 主要展示面
- Dashboard
- Positions
- Equity 曲线
- Position History
- Competition
- Strategy Market
- Settings / Traders / Strategy Studio

### 当前结论
前端展示链已经形成较完整产品面，并做过首轮懒加载与共享包优化。

---

# 5. 主要目录与模块职责说明

以下按仓库主要目录逐项说明。

## 5.1 `main.go`
系统启动总入口。

### 主要职责
- 加载环境变量
- 初始化 logger
- 初始化全局配置
- 初始化加密服务
- 初始化数据库
- 初始化 installation id / telemetry
- 设置 JWT
- 创建 `TraderManager`
- 从数据库加载 trader 并决定是否启动
- 创建 API server
- 启动 Telegram bot
- 处理优雅退出

### 重要性
这是整个系统的装配根节点。后续中文注释首轮，优先级极高。

## 5.2 `api/`
HTTP API 层。

### 主要职责
- 路由注册
- 鉴权和会话处理
- 对外暴露配置、查询、控制接口
- 面向前端与部分集成能力提供统一入口

### 典型能力
- 模型配置管理
- 交易所配置管理
- Trader CRUD / start / stop
- 策略 CRUD / activate / duplicate
- Dashboard 数据查询
- Competition 数据查询
- Telegram 配置管理
- 加密 / 公钥 / 解密相关接口
- Wallet 验证与生成

### 特点
`route_registry.go` 说明了项目有“路由文档化”的意识，这对后续 LLM / 文档生成 / 接管都很重要。

## 5.3 `config/`
全局配置读取层。

### 职责
- 从环境变量读取配置
- 为数据库、端口、JWT、Telegram 等提供统一配置来源

### 风险点
文档与实际环境变量命名存在偏差的可能性，需要持续核验。

## 5.4 `crypto/`
敏感数据加密能力。

### 职责
- 初始化加密服务
- 敏感字段的存储与读取支持
- 与前端的 transport encryption 配合

### 价值
是 API key / secret / 私钥等敏感信息保护的关键能力。

## 5.5 `manager/`
Trader 生命周期管理中心。

### 核心角色
`TraderManager`

### 主要职责
- 加载 trader 到内存
- 启动/停止 trader
- 管理运行状态
- 在系统启动时恢复运行中 trader

## 5.6 `kernel/`
策略/分析/Prompt 内核。

### 职责推断
- 组装策略配置
- 构建 prompt
- 整理分析上下文
- 定义 AI 输入输出 schema
- 驱动分析引擎与网格相关逻辑

### 重要性
Kernel 决定 AI 是否能在受控上下文内工作，是决策链的核心枢纽之一。

## 5.7 `market/`
市场数据与指标装配。

### 职责
- 拉取行情或组织行情
- 指标所需数据准备
- 与策略/图表/分析链协同

## 5.8 `mcp/`
模型客户端与协议适配层。

### 职责
- 抽象不同模型调用方式
- 挂载 provider
- 支持多模型与多付费模式

## 5.9 `provider/`
外部数据和市场提供方。

### 已见 provider
- `alpaca`
- `coinank`
- `hyperliquid`
- `nofxos`
- `twelvedata`

### 作用
给 AI 与交易系统提供行情、排名、量化辅助信息等。

## 5.10 `trader/`
自动交易核心与交易所适配器总目录。

### 职责
- 自动交易主逻辑
- 交易所适配器封装
- 仓位重建/快照
- 网格相关逻辑
- 统一行为接口

### 子目录（已见）
- `binance`
- `bybit`
- `okx`
- `gate`
- `kucoin`
- `bitget`
- `hyperliquid`
- `aster`
- `lighter`
- `indodax`

### 风险点
这是系统最需要长期审计的一层，因为不同交易所适配器的一致性、异常恢复、状态同步都容易成为隐患。

## 5.11 `store/`
数据库与领域持久化层。

### 可能覆盖的实体
- trader
- strategy
- exchange
- order
- position
- position_history
- equity
- ai_charge
- decision
- telegram_config
- system config

### 作用
这是平台持久化的事实来源，前后端很多可视化都依赖它。

## 5.12 `telegram/`
Telegram Bot / Agent 相关能力。

### 职责
- Telegram bot 启停
- Telegram 配置与绑定
- 会话状态 / agent 交互

### 当前定位
属于平台延伸面的一部分，但“完整可信交付范围”还需继续定义。

## 5.13 `web/`
前端控制台。

### 页面层（`web/src/pages/`）
- `LandingPage`：落地页 / 介绍页
- `SettingsPage`：账号、模型、交易所、Telegram 等设置入口
- `TraderDashboardPage`：交易员仪表盘主页面
- `StrategyStudioPage`：策略编辑/设计页面
- `StrategyMarketPage`：公开策略市场页面
- `DataPage`：数据展示页
- `FAQPage`：常见问题页面

### 组件层（`web/src/components/`）
主要分为：
- `auth`：登录/注册/重置密码/权限提示
- `charts`：图表、权益曲线、K 线、对比图
- `common`：通用组件与辅助 UI
- `landing`：落地页内容块
- `strategy`：策略相关编辑器
- `trader`：交易员相关组件、配置弹窗、榜单等
- `modals`：配置弹窗
- `faq`：FAQ 页面组件
- `ui`：通用基础 UI

### 前端当前状态
- 已做页面级懒加载
- 已做重图表和共享 vendor 拆分
- 当前主入口更健康，适合作为继续二开的基线

---

# 6. 项目所有主要功能清单（说明书式）

以下按功能域列出当前项目的主要能力。

## 6.1 系统接入与部署能力
- Docker 部署
- Railway 部署
- 本地源码运行
- Linux/macOS/Windows 指导
- HTTP/HTTPS 部署指导

## 6.2 模型接入能力
- 支持多种 AI 模型 provider
- 支持 API Key 配置
- 支持自定义 API Base URL / model name
- 支持钱包支付 / x402 模式
- 支持 Claw402 / BlockRun / ClawRouter 等相关路线

## 6.3 交易所接入能力
- 支持多交易所配置
- 支持 CEX 和 Perp-DEX
- 支持交易所账号增删改
- 支持敏感字段加密传输与存储
- 支持白名单 IP / 钱包地址 / API key 校验相关辅助能力

## 6.4 策略能力
- Strategy Studio 可视化编辑
- 币种来源配置
- 技术指标开关
- 风险参数设置
- Prompt sections 自定义
- 预览 prompt
- 测试运行策略
- 复制 / 激活 / 删除策略
- 策略市场展示公开策略

## 6.5 Trader 能力
- 创建 Trader
- 绑定 AI 模型 + 交易所 + 策略
- 启动 Trader
- 停止 Trader
- 更新 Trader 配置
- 查看 Trader 状态与配置
- 展示多个 Trader 列表

## 6.6 Dashboard 能力
- 展示权益
- 展示收益/亏损
- 展示当前持仓
- 展示决策记录
- 展示仓位历史
- 展示图表（Equity / Kline）
- 展示网格风险信息（grid risk）
- 支持 trader 切换

## 6.7 Competition 能力
- 多 Trader 排行榜
- 收益率比较
- 多条曲线对比
- 榜单项查看配置详情

## 6.8 Strategy Market 能力
- 展示公开策略
- 搜索策略
- 复制策略配置
- 作为“策略模板广场”的基础能力

## 6.9 认证与账户能力
- 登录
- 注册
- 重置密码
- 修改密码
- 登录态恢复
- 401 统一处理

## 6.10 Telegram 能力
- Telegram 配置
- Bot token 管理
- 模型选择
- 绑定/解绑 chat
- Telegram Agent 相关运行基础

## 6.11 加密与安全能力
- 传输加密配置查询
- 公钥获取
- 前端加密敏感字段
- 后端解密敏感载荷
- 敏感信息不应明文回显

## 6.12 Wallet / x402 能力
- 钱包生成
- 钱包校验
- x402 付费模式支撑
- 钱包驱动的模型调用身份

## 6.13 数据与图表能力
- Kline 数据
- Equity history
- 批量 equity history
- symbols 列表
- positions / account / statistics 等查询
- 高级图表与对比图

## 6.14 多市场扩展能力
项目产品层面对外宣称支持：
- Crypto
- US Stocks
- Forex
- Metals

但不同市场能力的成熟度不一定一致，后续仍需继续核对交付边界。

---

# 7. 当前已完成的接管与优化工作（面向审阅）

## 7.1 接管与治理资产
已建立并落仓：
- 项目总览
- 架构说明
- 模块索引
- 开发日志
- 决策记录
- 测试计划
- 验收标准
- 变更影响模板
- 修复候选清单
- 伏羲执行工作流
- 接管结项文档
- 项目记忆归档总表

## 7.2 外部问题清理
已完成首轮：
- 清理 `/api/admin-login` 残留
- 清理 `/api/prompt-templates` 残留
- 修复 public trader config 路径不一致
- 清理 admin mode 误导注释

## 7.3 前端交付优化
已完成首轮：
- 页面懒加载
- Dashboard 重块拆分
- vendor chunk 拆分
- KaTeX 按需加载
- Recharts 入口按需加载
- 部分核心 API 请求统一到 shared layer

---

# 8. 当前风险与未完全收口项

## 8.1 交易系统可信性未完全结项
虽然技术基线已经稳定，但以下仍需要后续专项收口：
- 决策链正确性
- 执行链一致性
- 风控链有效性
- PnL / 统计口径统一性
- 交易所适配器行为一致性
- 异常恢复与幂等

## 8.2 中文代码注释未完成
中文文档体系已有基础，但关键代码入口的中文注释首轮尚未系统补齐。

## 8.3 局部历史残留文件风险
个别文件在历史上可能出现过残留拼接/污染情况，因此后续修改必须继续坚持：
- 小步推进
- 改完即测
- 红灯先恢复稳定版本

## 8.4 部分能力需继续定义交付边界
例如：
- Telegram Agent 的完整交付边界
- Strategy Market 的正式产品边界
- 多市场能力的成熟度说明

---

# 9. 当前可交付状态说明

## 9.1 从代码版本角度
当前版本已经处于**阶段性可交付**状态：
- 测试通过
- 构建通过
- 当前分支可继续承接开发
- 当前仓库记忆与接管资产已固化

## 9.2 从整体接管角度
当前仍属于：
- 代码交付层基本成立
- 接管工程层仍在收口
- 交易系统可信性层仍需专项补充

换句话说：
**“版本可交付”已经成立，但“项目整体接管结项”尚未完全完成。**

---

# 10. 建议审阅重点
如果你准备审阅这份项目说明书，建议优先看：

1. 项目定位是否准确
2. 模块职责划分是否符合你对仓库的认知
3. 功能清单是否有遗漏/表述不当
4. 当前风险和未结项项是否足够诚实
5. 结项判断是否符合你的预期

---

# 11. 下一步建议
如果这份说明书认可，建议后续工作顺序为：

1. 完善这份说明书为正式长期文档
2. 补关键入口中文注释首轮
3. 输出交易系统可信性与风险边界说明
4. 再开始下一阶段二次开发
