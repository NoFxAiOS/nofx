# scripts/backtest — Backtrader 回测适配

当前系统有回测/研究能力，但此前没有引入第三方回测平台：

| 入口 | 能力与定位 |
|---|---|
| `scripts/backtest entry / exit` | 现已使用 **Backtrader 1.9.78.123**；原生 `Cerebro`、`BackBroker`、Market/Limit/Stop、OCO、手续费、滑点和账户盈亏 |
| `scripts/backtest ml-dataset` | 两套出场策略的收益标签均通过同一个 Backtrader 适配器生成 |
| `scripts/backtest ml-eval` | 信号特征的统计检验，不执行订单；拒绝未标记为 Backtrader 的旧标签 |
| `market/breakout/backtest.go`，`GET /api/breakout/params` | 突破信号的 1h/4h/24h 前瞻收益研究，`evaluation_scope=signal_forward_returns_research_only`；没有订单、账户或 SL/TP 模拟，不属于交易执行回测 |
| `GET /api/entry-quality-stats` | 实盘决策质量与已发生交易结果的聚合，不是历史回测执行入口 |

选择 [Backtrader](https://github.com/mementum/backtrader) 是因为现有信号提取、缓存和 ML 工具都是 Python，能直接复用数据。Backtrader 免费开源，采用 GPL-3.0 许可证；不需要平台注册或付费服务。项目只实现数据和策略适配，**不再维护自写撮合、仓位账务或盈亏模拟器**，缺少依赖时明确退出，不回退到旧逻辑。

本工具用于独立信号的出场/入场规则研究，每笔等权归一到初始风险 R；不是一个多资产、共享资金、合约保证金的组合回测。没有新增 Web 回测页面。

## 安装与执行

在仓库根目录执行。本次验证环境为 Python 3.12。

```bash
uv venv .venv
uv pip install --python .venv/bin/python -r scripts/backtest/requirements.txt

# 只读提取已有 AI 决策；日期参数为 UTC
.venv/bin/python -m scripts.backtest signals \
  --trader mac-nofx --since 2026-09-12 --until 2026-10-12T16:00

# 查看缺失的历史 K 线；去掉 --dry-run 才下载
.venv/bin/python -m scripts.backtest fetch --dry-run

# 两种入场方式均使用 Backtrader 撮合
.venv/bin/python -m scripts.backtest entry --horizons 8,24

# 基线为数据库里当前策略的 risk_control，不是历史配置快照
.venv/bin/python -m scripts.backtest exit --set mkt --horizon 72 \
  --csv data/backtest_cache/backtrader_mkt_72h.csv
.venv/bin/python -m scripts.backtest exit --set lim --horizon 72 \
  --csv data/backtest_cache/backtrader_lim_72h.csv

# 扫描现有出场规则变体
.venv/bin/python -m scripts.backtest exit --set mkt --horizon 72 \
  --variants scripts/backtest/variants/exit_grid.json --csv data/backtest_cache/variants.csv

.venv/bin/python -m unittest discover -s scripts/backtest/tests -v
```

`signals` / `fetch` 的数据工具和命令帮助仍可用不含 Backtrader 的 Python 环境执行。`entry` / `exit` / `ml-dataset` 必须使用安装了依赖的环境。

通用参数：`--db` 默认 `<repo>/data/data.db`，`--cache` 默认 `data/backtest_cache`，`--signals` 默认 `<cache>/signals.json`，`--trader` / `--trader-id` 可选择策略。回测命令的 `--engine` 只接受 `backtrader`。

成交参数：`--commission 0.0005` 表示每次成交收取 0.05%；默认对所有订单使用同一费率，包括限价单，不区分 maker/taker。`--slippage 0.0003` 表示市价/止损默认滑点 0.03%；限价单不额外滑点。`--limit-valid-min 45` 为从原始信号时间起算的限价有效期。这些都使用原生 broker 配置；费用根据实际成交价格和实际数量收取。

## 数据来源

| 数据 | 来源与用途 |
|---|---|
| 入场信号 | `data/data.db` 的 `decision_records.decisions`，只读打开；成功的 `open_long/short(_limit)` 决策 |
| 初始 SL/TP/限价 | `sl0` / `tp0` / `px0`，只使用该信号首次决策当时的参数 |
| 出场模式与 ATR | 首次记录的 `decision_json.exit_mode` 和 Structured Signal 的 1h ATR% |
| 历史成交标签 | `trader_positions` 的匹配结果，仅作实盘标签分组；不决定模拟限价单是否成交或成交价格 |
| 基线出场参数 | `traders.strategy_id → strategies.config.risk_control`，每次运行读取当前配置；CSV 保存实际参数 |
| K 线 | 币安 `fapi/v1/klines` → `<cache>/{kl1m,kl5m}/{SYMBOL}_{t0ms}.json`；每行 `[openMs,o,h,l,c]` |

信号去重沿用原来的方法：同币同向连续限价决策间隔小于 `--dedupe-min`（默认 60 分钟），且期间无实盘成交，合并为一个信号。模拟只提交首次决策的订单，不使用 `sl_last` / `tp_last` / 实际 `fill_px` 来重建未来状态。合并后的撤单/改价过程没有重放。

缓存先用 1m，再接不重叠的 5m 数据。适配器把时间戳转换为 **K 线收盘时间**（分别加 1/5 分钟），所有时间为 UTC；找不到信号分钟、OHLC 不合法或数据重叠会跳过。`fetch` 复用 `HTTPS_PROXY`，请求间隔至少 1 秒，权重高时退避。

## Backtrader 执行口径

- 不启用 cheat-on-close/open。信号分钟收盘后提交开仓订单，市价单最早在下一根开盘成交；限价单由 broker 按 OHLC 和有效期撮合，实盘未成交的信号也可能在模拟中成交。
- 开仓成交的通知到达后，策略根据实际成交价和原始止损计算初始风险，创建保护订单。保护订单从后续 K 线开始执行；开仓成交所在 K 线内的 SL/TP 触发无法回溯执行。
- 原始止损使用原生 Stop，止盈/减仓使用原生 Limit，关联为 OCO。减仓成交后，用剩余数量重建保护订单，防止旧订单超额平仓形成反向仓位。同根触及多档时先撮合近档，其余档最早在后续 K 线上重挂。
- 保本、盈利锁、ATR 追踪和峰值回撤根据已经收盘的 K 线更新 Stop 价格；新价从下一根生效。不会在同一根先观察最高价，再假定已在该根低价路径上成交。跳空成交、同根已有订单的处理顺序均由 Backtrader 决定。
- quick 时间止损和 horizon 到点后提交原生 Market 平仓，最早在下一根可用 K 线开盘成交。数据结束仍有持仓则标记 `open`，用 broker 账户价值盯市，只收取已发生成交的费用，不伪造最后一笔平仓。
- 每笔研究账户使用足够的虚拟现金和 1 单位资产，不引入实际资金约束。收益 `R = broker 净盈亏 / 实际初始止损风险`，包括真实发生的部分减仓费用。原生 TradeAnalyzer/DrawDown 可在 `backtrader_adapter.run_trade` 的结果中查看。

默认按 (币, 方向) 过滤持仓重叠，`--allow-overlap` 可关闭，`entry` 的市价和限价两组分别过滤。持仓时间取框架执行结果；数据不足而持仓未结束时，保守占用到覆盖末尾或预定 horizon 中较晚的时间。该过滤只是独立信号研究的样本筛选，不是一个共享账户的并发撮合模型。

### 报告

`exit` 输出 n、sumR、avgR、胜率、`maxDD_R`、前后半段 sumR、去掉最大 5 笔后的均值、blocked、unfilled、open 和出场类型。`maxDD_R` 为按信号时间排序的等权累计 R 回撤，**不是组合权益回撤百分比**。CSV 同时保存 engine、engine_version、手续费、滑点和每个变体的完整出场参数。

`entry` 的 A 行为框架模拟限价入场，B 行为下一根开盘市价入场；B1/B2 仍按实盘 filled/unfilled 标签做描述性分组。改变回测规则后，各组实际接纳交易的数量可能不同。

### 策略参数

`params.py` 从线上配置推导基线：保本 `breakeven_arm_r`，锁盈 `profit_lock_at_r` / `profit_lock_be_offset_r` / `tp_trim_yields_to_lock`，减仓 `tp_trim_at_r`，止盈上限 `tp_max_r`，趋势止盈比例 `tp_close_fraction`，ATR 追踪 `trailing_stop_enabled`，峰值回撤 `peak_drawdown_arm_r` / `peak_drawdown_giveback_r`，quick 时间止损 `time_stop_hours`。旧 ROE 减仓/回撤档未建模时会提示。追踪默认 1.5R 启动、2×开仓时 1h ATR。

变体 JSON 为 `[[名称, {覆盖参数}], ...]`，第一行始终是 baseline。支持键：`be_arm be_off lock_r lock_trim trim_r trim_frac tp_cap tp_frac_trend trail trail_arm trail_atr pd_arm pd_give quick_stop_h force_mode use_tp horizon_h`。未知键和不合法减仓比例会报错。

## 信号过滤器检验

```bash
.venv/bin/python -m scripts.backtest ml-dataset --csv data/backtest_cache/ml_rows.csv
uv run --no-project --with numpy --with scikit-learn python -m scripts.backtest ml-eval \
  --label both --perms 400 --folds 5 --min-train 0.4 --min-coverage 0.8 --seed 7
```

`ml-dataset` 用同一个 Backtrader 适配器生成 `r_base` / `r_plain`，不做单仓样本过滤，保存 engine 和费率。`mfe` 是框架持仓期间已收盘 K 线的最大有利 R。特征提取和信号与决策记录的 ±10 分钟匹配沿用原工具；匹配是研究近似，不能保证所有特征严格来自开仓前。plain 关闭保本/盈利锁/减仓/追踪/峰值回撤/止盈上限。

`ml-eval` 的 expanding train、模型、置换检验和嵌套筛选保持原来的统计流程：只有嵌套检验与同一个模型在 `r_base` 和 `r_plain` 两个标签上都通过 Bonferroni 阈值，才输出 `PASS`。仅跑单标签不能通过。4 个模型、2 个标签共 18 项检验，400 次置换满足 p 值精度要求。

**旧模拟器的收益标签、CSV 和 2026-10-13 的检验结果不适用于新执行口径。** 使用新的 `ml-dataset` 重建标签后再评估，不能将旧数值与 Backtrader 数值直接混用。

## 数据与模型边界

历史 AI 决策是已记录的信号，回测不会重新调用 AI 生成历史决策。当前策略配置的基线只是对历史信号重放当前规则，不代表当时实际采用的配置。

OHLC 无法确定 K 线内部事件顺序，混合 1m/5m 精度也不同。缓存不含成交量，因此未启用成交量约束或部分成交模型；这里只模拟订单规则中的主动部分减仓。未建模资金费率、合约保证金/清算、交易所最小数量和精度、AI 主动平仓、组合最大仓位数、共享资金或历史改价链。结果应作为规则研究，不能当作实盘账户收益。

Backtrader 参考：[订单执行](https://www.backtrader.com/docu/order-creation-execution/order-creation-execution/)、[OCO](https://www.backtrader.com/docu/order-creation-execution/oco/oco/)、[手续费](https://www.backtrader.com/docu/commission-schemes/commission-schemes/)、[滑点](https://www.backtrader.com/docu/slippage/slippage/)。
