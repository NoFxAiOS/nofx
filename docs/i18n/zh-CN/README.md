<p align="center">
  <img src="../../assets/nofx-banner.svg" alt="nofx — 让智能落地交易。由你自己运行的开放交易系统。" width="100%"/>
</p>

<p align="center">
  <a href="#frontier">Frontier</a> ·
  <a href="#run-locally">开始使用</a> ·
  <a href="#documentation">文档</a> ·
  <a href="#community">社区</a>
</p>

<p align="center">
  <a href="https://github.com/NoFxAiOS/nofx/stargazers"><img src="https://img.shields.io/github/stars/NoFxAiOS/nofx?style=flat&labelColor=192434&color=295BE8" alt="GitHub stars"></a>
  <a href="../../../LICENSE"><img src="https://img.shields.io/badge/license-AGPL--3.0-295BE8?style=flat&labelColor=192434" alt="License: AGPL-3.0"></a>
  <a href="https://t.me/nofx_dev_community"><img src="https://img.shields.io/badge/telegram-community-295BE8?style=flat&labelColor=192434&logo=telegram&logoColor=white" alt="Telegram community"></a>
</p>

<p align="center">
  <a href="../../../README.md">English</a> ·
  <a href="README.md">中文</a> ·
  <a href="../ja/README.md">日本語</a> ·
  <a href="../ko/README.md">한국어</a> ·
  <a href="../ru/README.md">Русский</a> ·
  <a href="../uk/README.md">Українська</a> ·
  <a href="../vi/README.md">Tiếng Việt</a>
</p>

NOFX 是在你自己的电脑上运行的开源交易系统。使用 **Frontier** 与 AI 推敲交易想法，在图表上比较入场方案，再由你批准下单。使用 **Autopilot** 持续运行策略。两者共用你配置的模型和交易所账户。

与 [Vergex](https://vergex.trade) 共同构建，通过 [Claw402](https://claw402.ai) 接入 AI 和数据。

<a id="run-locally"></a>

## 本地运行

**一行安装 — Linux / macOS**

安装并启动带有 Compose 的 Docker，然后运行：

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

在桌面浏览器中打开 **http://127.0.0.1:3000**。

安装脚本会生成缺失的环境密钥，并使用已发布的 Docker 镜像启动 NOFX，无需安装 Go 或 Node.js。接下来按照[首次使用](#your-first-session)完成配置，或查看[源码构建及其他安装方式](#source-builds-and-other-options)。

## Frontier

一张图表、一段对话，以及下单前能看明白的交易方案。

1. **选择市场。** 浏览高成交量标的、搜索交易所原生代码，或使用币安 AI500 列表。交易所支持时，还可选择美股等股票挂钩标的。
2. **比较入场方式。** 生成市价、限价、加权阶梯和 TWAP 方案，同时查看入场价、止损、目标、杠杆、仓位价值及预估保证金。
3. **审核并下单。** 选择方案，让图表聚焦该方案。核对当前账户与订单详情，再明确批准执行。
4. **管理持仓。** 在图表下方查看实时持仓和交易所订单，也可手动开仓或平仓。

Frontier 支持 **Binance USDT 永续、Hyperliquid、OKX 和 Bitget**。连接交易账户前即可浏览公开行情；实际交易需要对应交易所的账户。生成方案不会提交订单。

| 入场方式 | 执行方式 |
| :--- | :--- |
| **市价** | 批准后按可成交的市价入场；界面显示的入场价为估算值。 |
| **限价** | 在指定价格提交入场订单。 |
| **阶梯** | 按权重将仓位分配到多个价格档位。 |
| **TWAP** | 在批准的价格范围内，按时间依次释放有限数量的子单。 |

限价单和定时子单可能无法成交。参见[执行细节与恢复流程](../../features/trade-planning.md)。

## Autopilot

如果需要持续运行策略，可以为交易实例配置模型、交易所账户和策略。Autopilot 读取行情、作出决策，再通过 NOFX 的交易所执行模块下单。Dashboard 展示持仓、表现和决策记录。

**Frontier 的每笔交易都需要你批准。Autopilot 在运行期间可以自动下单。** 启动或停止 Autopilot 是独立操作。

| 工作区 | 用途 |
| :--- | :--- |
| **Config** | 接入模型、为 AI 钱包充值、管理交易账户。 |
| **Frontier** | 浏览市场、比较入场方案、审核并下单。 |
| **Dashboard** | 查看运行中的交易实例、持仓和决策记录。 |
| **Strategy** | 配置选币、指标、仓位规模和交易指令。 |
| **Leaderboard** | 比较公开的交易表现。 |

<a id="your-first-session"></a>

## 首次使用

1. **创建本地账户。** 登录刚启动的 NOFX 实例。
2. **在 Config 中配置 AI。** 选择模型。使用 Claw402 时，创建或导入 AI 费用钱包，保存私钥，并向钱包地址转入 **Base 网络的 USDC**。该余额用于支付模型和付费数据请求。
3. **添加交易账户。** 连接要使用的交易所，检查账户权益和可用余额。交易资金与 AI 费用钱包分开管理。
4. **打开 Frontier。** 选择交易所、账户和标的，生成方案并在图表上比较，准备好后审核订单。

如果需要持续自动交易，请另外配置并启动 Autopilot。

### 更新通过一行命令安装的实例

先停止运行中的 Autopilot，并检查正在执行的 Frontier 交易，再在安装目录（默认为 `~/nofx`）中运行：

```bash
docker compose pull
docker compose up -d
```

升级时保留 `.env` 和 `data/` 目录，其中包含配置、加密材料及已保存的应用状态。

重启后，在交易所核对持仓和订单，处理所有标记为需要检查的执行记录，再进行下一笔交易。停止进程不会自动平仓。

<a id="source-builds-and-other-options"></a>

## 源码构建及其他安装方式

<details>
<summary>开发者：从源码构建和更新</summary>

开发 NOFX，或需要在镜像发布前测试源码改动时，使用此方式。安装 Git 和带有 Compose 的 Docker，并启动 Docker：

```bash
git clone --recurse-submodules https://github.com/NoFxAiOS/nofx.git
cd nofx
./start.sh start --build
```

启动脚本会在需要时创建 `.env`，并生成缺失的加密密钥。如果克隆时没有拉取子模块，请先运行 `git submodule update --init web/vendor/charting` 再构建。TradingView 图表资源受其适用许可证约束，参见[图表集成说明](../../features/trade-planning.md#harness-chart-and-plugins)。

更新源码安装的实例时，先停止运行中的 Autopilot，并检查正在执行的 Frontier 交易，然后运行：

```bash
git pull --ff-only
git submodule update --init web/vendor/charting
./start.sh start --build
```

</details>

<details>
<summary>Windows、Railway 和远程部署</summary>

**Windows**

使用 Docker Desktop 和 WSL2 等 Bash 环境，再运行上方的一行安装命令。

**Railway**

[![部署到 Railway](https://railway.com/button.svg)](https://railway.com/deploy/nofx?referralCode=nofx)

**远程部署**

安装脚本使用生产环境 Compose 配置，默认在主机的所有网络接口上开放端口。源码 Compose 配置默认将前端和 API 绑定到 `127.0.0.1`。对外开放远程实例前，请配置监听地址，以及带身份验证的 HTTPS 代理。

</details>

<details>
<summary>不使用 Docker 的开发方式</summary>

使用 [go.mod](../../../go.mod) 指定的 Go 版本、与前端 Docker 构建一致的 Node.js 24，以及 TA-Lib。启动前端前先初始化图表子模块，并根据 [.env.example](../../../.env.example) 配置环境。

```bash
# 后端
go build -o nofx .
./nofx

# 前端，在另一个终端中运行
cd web
npm ci
npm run dev
```

</details>

## 模型与数据

Frontier 和 Autopilot 共用在 NOFX 中配置的模型。[Claw402](https://claw402.ai) 提供多种模型，使用 Base 网络的 USDC 按用量结算。你可以在 Config 中选择模型，再在工作区切换。

行情和 K 线来自所选交易所。AI500 及受支持的持仓地址分析通过 Claw402 付费接口获取。数据是否可用取决于市场和服务商：Hyperliquid 的持仓地址数据不能代替 Binance、OKX 或 Bitget 的数据。活动流会展示数据读取和规划进度。

AI 费用、付费数据费用与交易所资金彼此独立。即使没有下单，模型请求也可能产生费用。

## 交易所

Frontier 当前提供 Binance、Hyperliquid、OKX 和 Bitget 接入。NOFX 的其他交易执行模块还包括 Bybit、Gate、KuCoin、Aster 和 Lighter。可用产品、订单类型和账户权限取决于交易所。

以下为支持项目开发的合作注册链接，手续费优惠由各交易所制定。

| 交易所 | 状态 | 合作注册链接 |
| :--- | :---: | :--- |
| <img src="../../../web/public/exchange-icons/binance.jpg" width="20" height="20" style="vertical-align: middle;"/> **Binance**       |   ✅   | [注册](https://www.binance.com/join?ref=NOFXENG)                                |
| <img src="../../../web/public/exchange-icons/bybit.png" width="20" height="20" style="vertical-align: middle;"/> **Bybit**           |   ✅   | [注册](https://partner.bybit.com/b/83856)                                       |
| <img src="../../../web/public/exchange-icons/okx.svg" width="20" height="20" style="vertical-align: middle;"/> **OKX**               |   ✅   | [注册](https://www.okx.com/join/1865360)                                        |
| <img src="../../../web/public/exchange-icons/hyperliquid.png" width="20" height="20" style="vertical-align: middle;"/> **Hyperliquid** |   ✅   | [注册](https://app.hyperliquid.xyz/join/AITRADING)                              |
| <img src="../../../web/public/exchange-icons/bitget.svg" width="20" height="20" style="vertical-align: middle;"/> **Bitget**         |   ✅   | [注册](https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172) |
| <img src="../../../web/public/exchange-icons/kucoin.svg" width="20" height="20" style="vertical-align: middle;"/> **KuCoin**         |   ✅   | [注册](https://www.kucoin.com/r/broker/CXEV7XKK)                                |
| <img src="../../../web/public/exchange-icons/gate.svg" width="20" height="20" style="vertical-align: middle;"/> **Gate**             |   ✅   | [注册](https://www.gatenode.xyz/share/VQBGUAxY)                                 |
| <img src="../../../web/public/exchange-icons/aster.svg" width="20" height="20" style="vertical-align: middle;"/> **Aster**           |   ✅   | [注册](https://www.asterdex.com/en/referral/fdfc0e)                             |
| <img src="../../../web/public/exchange-icons/lighter.png" width="20" height="20" style="vertical-align: middle;"/> **Lighter**       |   ✅   | [注册](https://app.lighter.xyz/?referral=68151432)                              |

## 订单如何执行

交易方案和策略决策使用 NOFX 现有的交易所适配器。模型提示词不会包含钱包私钥或交易所凭证。

- **下单前可审核。** Frontier 在批准前展示账户、仓位规模及入场和退出详情。Autopilot 遵循所配置的策略及运行限制。
- **按账户资金计算仓位。** Frontier 使用所选杠杆和当前可用保证金，并预留执行成本。数量精度、价格步长及交易所限制仍然适用。
- **交易所侧止盈止损。** 执行模块观察到成交后，会提交受支持的止损和止盈订单。入场和保护单并非原子操作；保护单失败时，请到交易所检查。
- **核对订单状态。** 提交结果不确定时，先向交易所核实，不盲目重复下单。未解决的执行状态可能暂停后续入场。
- **本地配置。** NOFX 服务端加密保存交易所凭证。你的部署也持有签名材料，请保护好相关文件和备份。

## 架构

```text
桌面浏览器
  Frontier · Config · Dashboard · Strategy
    → NOFX API
      → Frontier：规划、审核与批准
      → Autopilot：策略循环
    → 共用执行模块 → 交易所适配器 → 你的交易账户

模型与付费数据：NOFX 服务商客户端 · Claw402
存储：本地配置、方案和执行记录
```

<details>
<summary>旧版界面演示</summary>

此视频展示的是较早版本的界面，导航和视觉设计现已更新。

https://github.com/user-attachments/assets/3310f495-14c5-4586-a1cc-3d32e44aa505

</details>

## 品牌素材

- [标志 — SVG](../../../web/public/icons/nofx-flow.svg)
- [GitHub 头像 — PNG，512 × 512](../../../web/public/brand/nofx-avatar-512.png)
- [GitHub 头像 — SVG 源文件](../../../web/public/brand/nofx-avatar.svg)
- [README 横幅 — SVG](../../assets/nofx-banner.svg)

<a id="documentation"></a>

## 文档

| 指南 | 内容 |
| :--- | :--- |
| [Frontier](../../features/trade-planning.md) | 规划、实盘执行、数据和恢复流程 |
| [快速开始](../../getting-started/README.zh-CN.md) | 部署与交易所 API 指南 |
| [架构](../../architecture/README.md) | 系统设计与模块索引 |
| [策略模块](../../architecture/STRATEGY_MODULE.md) | 选币、AI 提示词与执行 |
| [常见问题](../../guides/faq.zh-CN.md) | 常见疑问解答 |
| [故障排查](../../guides/TROUBLESHOOTING.zh-CN.md) | 常见问题诊断 |

<a id="community"></a>

## 社区

[Telegram](https://t.me/nofx_dev_community) · [Twitter/X](https://x.com/vergex_ai) · [Issues](https://github.com/NoFxAiOS/nofx/issues) · [vergex.trade](https://vergex.trade) · [实时仪表板](https://vergex.trade/explore)

## 贡献

代码、文档、翻译和 Bug 报告都欢迎——参见[贡献指南](../../../CONTRIBUTING.md)、[行为准则](../../../CODE_OF_CONDUCT.md)和[安全政策](../../../SECURITY.md)。

NOFX 会记录有价值的贡献，并计划随着生态发展回馈贡献者。优先级 Issue 拥有更高权重。

| 贡献类型      | 权重 |
| :---------------- | :----: |
| 置顶 Issue 的 PR  | ★★★★★★ |
| 代码（已合并 PR） | ★★★★★  |
| Bug 修复         |  ★★★★  |
| 功能建议     |  ★★★   |
| Bug 报告       |   ★★   |
| 文档     |   ★★   |

<a href="https://github.com/NoFxAiOS/nofx/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=NoFxAiOS/nofx" alt="Contributors"/>
</a>

## 赞助者

<a href="https://github.com/pjl914335852-ux"><img src="https://github.com/pjl914335852-ux.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/cat9999aaa"><img src="https://github.com/cat9999aaa.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/1733055465"><img src="https://github.com/1733055465.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/kolal2020"><img src="https://github.com/kolal2020.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/CyberFFarm"><img src="https://github.com/CyberFFarm.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/vip3001003"><img src="https://github.com/vip3001003.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/mrtluh"><img src="https://github.com/mrtluh.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/cpcp1117-source"><img src="https://github.com/cpcp1117-source.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/match-007"><img src="https://github.com/match-007.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/leiwuhen1715"><img src="https://github.com/leiwuhen1715.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/SHAOXIA1991"><img src="https://github.com/SHAOXIA1991.png" width="50" height="50" style="border-radius:50%"/></a>

[成为赞助者](https://github.com/sponsors/NoFxAiOS)

<br/>

如果 NOFX 对你有帮助，点个 Star 能让更多交易者发现它。

[![Star History Chart](https://star-history.dera.page/svg?repos=NoFxAiOS/nofx&type=Date)](https://star-history.dera.page/#NoFxAiOS/nofx&Date)

## 许可证

[AGPL-3.0](../../../LICENSE)

<sub>自动化交易存在重大风险。AI 驱动的策略尚处实验阶段，可能造成亏损。请合理控制仓位规模，充分了解每个交易场所，切勿投入无法承受损失的资金。完整[免责声明](../../../DISCLAIMER.md)。</sub>
