# scripts/backtest — 规则回测工具

用线上 bot 的真实 AI 开仓信号,回放币安 U 本位合约 1m/5m K 线,比较「入场方式」和「出场规则」改动的效果。纯 stdlib,Python 3.9+(用到 `BooleanOptionalAction`)。

## 数据来源

| 数据 | 来源 | 说明 |
|---|---|---|
| 入场信号 | `data/data.db` 的 `decision_records.decisions`(只读打开) | `open_long/short(_limit)` 且 `success=true` 的决策;`exit_mode` 取自同记录 `decision_json`(缺省 trend),1h ATR% 取自 `input_prompt` 里的 `=== SYM Structured Signal` JSON |
| 限价是否成交 | `trader_positions` | 同 trader/币/方向,`entry_time` 在 [t0, 最后一次决策+45min],`ai_managed=1` 优先,取最近;并要求成交价在该信号任一挂单价的 0.5% 内(`--max-fill-dev-pct`),用来排除手动单 |
| 基线出场参数 | `traders.strategy_id → strategies.config.risk_control` | `exit` 默认由此推导,见下 |
| K 线 | 币安 `fapi/v1/klines` → `data/backtest_cache/{kl1m,kl5m}/{SYMBOL}_{t0ms}.json` | 1m 从信号分钟起 1500 根(约 25h),其后接 5m 1000 根(约 83h);行 `[openMs,o,h,l,c]` |

**信号去重**:同币同向的连续限价决策,相邻间隔 < `--dedupe-min`(默认 60)分钟且期间无成交 → 合并为一个信号(t0/sl0/tp0/exit_mode/ATR 取第一条,最后一条的 sl/tp 记为 `sl_last/tp_last`)。市价开仓各自独立(outcome=`market`)。

## 命令(仓库根目录执行)

```bash
python3 -m scripts.backtest signals --trader mac-nofx --since 2026-09-12 --until 2026-10-12T16:00
python3 -m scripts.backtest fetch                      # 只补抓缺失/不完整的缓存;--dry-run 只统计
python3 -m scripts.backtest entry --horizons 8,24
python3 -m scripts.backtest exit --set mkt --horizon 72 --variants scripts/backtest/variants/exit_grid.json [--csv out.csv]
python3 -m unittest discover -s scripts/backtest/tests -v   # 离线单测
```

通用参数:`--db`(默认 `<repo>/data/data.db`)、`--cache`(默认 `data/backtest_cache`)、`--signals`(默认 `<cache>/signals.json`)、`--trader` / `--trader-id`。`fetch` 走环境变量 `HTTPS_PROXY`,请求间隔 ≥1s,权重头 `X-MBX-USED-WEIGHT-1M`>1000 时退避 30s。

- **signals**:输出 `{meta, signals}` JSON,打印信号数与 outcome 分布(filled/unfilled/market)。
- **entry**:限价实际成交 vs 信号分钟收盘市价入场(+0.03% 滑点,taker 0.05% 进出,限价 maker 0.02% 进),固定 SL/TP 括号,同一根 K 线 SL+TP 都触及算 SL。行:限价实际成交、市价(全部限价信号)、市价(限价成交的)、市价(限价没成交的)、AI 原生市价单、多空拆分。
- **exit**:出场规则模拟。`--set mkt` 每个信号在 t0 市价入场;`--set lim` 只用成交了的限价信号按成交价入场。`--variants` 为 `[[名称, {覆盖参数}], ...]`,第一行恒为 `baseline`。每个变体输出 n、sumR、avgR、胜率、maxDD(按 t0 排序的累计 R)、前后半段 sumR、去掉最大 5 笔后的均值、horizon 末仍持仓数、出场类型计数(sl/be/trail/tp/pd/time/open)。

### 同币同向单仓约束(默认开,`--allow-overlap` 关闭)

线上对已有同币同向仓位的币会拒绝新开仓,所以模拟也不允许重叠:按入场时间顺序遍历信号,为每个 (币, 方向) 记录 busy-until = 已接受交易的出场时间(horizon 到点仍持仓则取 horizon 末);入场时间早于 busy-until 的信号被跳过并计入 `blocked`。该约束**与变体相关**——持仓越久挡掉的后续信号越多,所以不同规则的 n 不同,比较时要同时看 `blocked`。作用范围:`exit`(`mkt`/`lim` 两个集合,输出行与 CSV 都有 `blocked` 列)和 `entry` 的市价行(按固定 SL/TP 括号的出场时间挡,每个 horizon 单独计);`entry` 的「限价实际成交」行是真实成交,不过滤。被挡掉的信号不参与统计,所以带约束的 sumR 与 `--allow-overlap` 不可直接相比。

### 出场模型(R 空间,多空镜像,每根 K 线按此顺序)

1. low ≤ SL → 剩余仓位按 SL 出;2. 触及 TP → 平 `tp_frac_trend`(趋势模式且追踪止损开)或 100%;3. 更新 peak;4. 保本档 `peak ≥ be_arm` → SL=max(SL, be_off);5. 盈利锁 `peak ≥ lock_r` → SL=max(SL, be_off),`lock_trim` 时一次性减 50%;6. R 减仓档 `peak ≥ trim_r` 一次性减 `trim_frac`;7. 追踪止损(仅趋势模式)`peak ≥ 1.5R` → SL=max(SL, high−2×ATR_R);8. 峰值回撤 `peak ≥ pd_arm` 且 low ≤ peak×(1−pd_give) → 按该价出;9. quick 模式持仓 ≥ `quick_stop_h` 且收盘 ≤ 0 → 收盘价出。手续费按整仓折成 R 一次性扣除。

基线参数(`--from-strategy`,默认开;`--no-from-strategy` 用内置兜底)的推导规则:`breakeven_arm_r≤0` 关;`profit_lock_at_r` <0 关/0→1.0;`profit_lock_be_offset_r` 0→0.2、<0→0(保本档与盈利锁共用);`tp_trim_at_r>0` → 1/3 仓位减在该 R,否则关(ROE 旧档未建模,会告警);锁减仓 50% 仅当 `tp_trim_yields_to_lock` 为 null/true;`tp_max_r<1` 不截断;`tp_close_fraction` 0→0.5、<0→1.0、追踪关时恒 1.0;`peak_drawdown_arm_r>0` 为 R 模式(giveback 0→0.5),否则按关闭并告警;`time_stop_hours` 0→4、<0 关。追踪起点 1.5R、2×ATR(1h) 是代码常量。运行时会打印推导出的参数。

变体可覆盖的键见 `params.py`:`be_arm be_off lock_r lock_trim trim_r trim_frac tp_cap tp_frac_trend trail trail_arm trail_atr pd_arm pd_give quick_stop_h force_mode use_tp horizon_h`。`variants/exit_grid.json` 是单因素扫描 + 在 plain 上逐项加回的组合;「关闭 X」类变体显式写出各项(如 `no BE tier (lock still on)` 只关保本档,盈利锁仍开)。

## 信号过滤器检验(ml-dataset / ml-eval)

问题:能否用开仓时刻的 Structured Signal 特征,统计地筛掉一部分信号来提高期望?此检验要在信号累积到 1000+ 后重跑。

```bash
python3 -m scripts.backtest ml-dataset [--csv rows.csv]     # 纯 stdlib;输出 <cache>/ml_rows.json
uv run --no-project --with numpy --with scikit-learn python -m scripts.backtest ml-eval \
    [--label both|r_base|r_plain] [--perms 400] [--folds 5] [--min-train 0.4] [--min-coverage 0.8] [--seed 7]
```

- **ml-dataset**:对每个信号在 ±10 分钟内找同 action+币的决策记录,解析 Structured Signal 与 AI 决策,算特征(`mlfeatures.py`,凡「方向」特征都以「与交易方向一致」为正,做空取反;缺失为 None),标签为 t0 市价入场、72h、不做单仓过滤:`r_base`(线上基线出场参数)、`r_plain`(无保本/锁盈/减仓/追踪/峰值回撤/止盈上限)、`mfe`。打印行数、缺失数和覆盖率低于 80% 的特征。
- **ml-eval**(`mleval.py`,需 numpy+scikit-learn,缺失时会提示 uv 命令):只用覆盖率 ≥ `--min-coverage` 的特征。(a) 单变量 Spearman 在前后半段的稳定性(描述性,附纯噪声期望命中数,不计入检验数);(b) 走步(expanding train、`--folds` 折、起始训练占比 `--min-train`)的 logit-L2、logit-L1、ridge、HGB(depth2),报告 OOS AUC/rho、保留 top70% 与 top50% 的 R 相对「随机保留同比例」的提升及置换检验 p 值(训练标签打乱、对真实结果评估);(c) 嵌套流程——每折只用过去数据按半段稳定性挑一个特征、以训练集中位数为阈值,与随机保留同样数量的子集比较(5000 次抽样)。
- **裁决规则**:检验总数 = 所有 label 上 模型数×2 个阈值 + 1 个嵌套;Bonferroni 阈值 = 0.05 / 检验数。只有「嵌套检验」和「同一个模型」在 **r_base 和 r_plain 两个 label 上都** ≤ 阈值才输出 `PASS: candidate filter X`,否则输出 `NO DEPLOYABLE FILTER`。默认 `--label both`;只跑单个 label 时永远不会 PASS。置换 p 值下限为 1/(perms+1),所以 4 模型×2 label 共 18 个检验时 `--perms` 至少要 360(默认 400;程序会在不足时提示)。
- **2026-10-13 结果(n=420,67 个特征覆盖率 ≥80%)**:无可部署过滤器。r_base 走步 logit-L2 的 AUC≈0.53,保留 top50% 比随机多 +23.4R(单模型 p≈0.01),看上去不错,但嵌套检验 p≈0.5(r_plain 同样 ≈0.6);事后看 `btc_beta`(越低越好)两个半段都稳定,但按「每折只用过去数据选特征」的嵌套流程选不出它、也过不了检验——事后挑出来的特征不算数。下次重跑的门槛就是上面的 Bonferroni 裁决。

## 假设与局限

- 同币同向单仓约束下,出场时间取触发出场的 K 线开盘时间(1m/5m),下一笔入场不早于它即放行。
- 同一根 K 线内 SL 与 TP/其他触发同时出现,按 SL 优先(悲观);K 线内部顺序未知。
- AI 主动平仓、最大持仓数(槽位)限制、资金/仓位大小、加减仓后的手续费差异均未建模;每个信号独立、等权 1R。
- horizon 到点仍持仓的,按 horizon **内最后一根** K 线收盘盯市(`open` 计数);数据不足 horizon 时同样按最后一根盯市。
- 1m 数据只覆盖约 25h,之后换成 5m(精度下降,同一根 5m 内触发顺序更不确定)。
- 信号全部来自 `decision_records`,手动下单(用户在 bot 账户上手工交易)不在其中;限价成交匹配可能把手动单误判(已用价格偏离过滤,不保证完全准确)。
- 与按日志重建的限价链相比,按时间间隔合并(默认 60 分钟)会把「挂单过期/撤单后重新挂」的相邻信号合并成一个,信号数与成交数会有出入;结果对信号集合很敏感(单笔 σ≈1R,几百笔的 sumR 标准误约 20R),比较不同规则时请固定同一份 signals.json。
- 最近的信号 K 线不足 1500/1000 根时,`fetch` 会在下次运行时重抓补全。
