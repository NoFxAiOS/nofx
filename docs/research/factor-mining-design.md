# 自动因子挖掘（美股 + 虚拟币）落地方案

参考 [microsoft/qlib](https://github.com/microsoft/qlib) 的三块能力，**只复刻思路，不引入 qlib 依赖**：

| qlib 能力 | 我们的对应实现 | 为什么不直接用 qlib |
|---|---|---|
| 表达式引擎 + Alpha158/Alpha360 因子库（`Ref/Mean/Std/Rank/Corr/Slope/Rsquare/Resi/Quantile…`） | `scripts/factor_mining/expr.py` 安全表达式 DSL + `zoo.py` 程序化生成的种子因子库 | qlib 需要自有二进制数据格式、对币圈 7×24 小时/新币频繁上下市支持差、依赖重 |
| 因子/模型评估（RankIC、ICIR、多空分层、换手） | `evaluate.py` 冻结评估器（沿用 autoresearch 的 train/val/holdout + 净费用 + 多重检验门槛） | 评估口径必须与 nofx 现有研究一致 |
| RD-Agent(Q)：LLM 自主提出因子→回测→反馈→进化 | `miner.py`：种子库 → 遗传式变异 → GLM-5.3-flash 提案，三路候选走同一评估器 | 直接复用已有 opencode-task / 订阅 GLM |

## 数据

| 市场 | 来源 | 频率 | 历史 | 宇宙 |
|---|---|---|---|---|
| crypto | Binance 永续 `fapi/v1/klines` interval=1d（经 127.0.0.1:7890 代理） | 日线 | 各币上市至今（多数 2–5 年） | `universe_src.json` 的 crypto 列表，剔除 APRUSDT/MANAUSDT/STXUSDT/USELESSUSDT，点时刻按近 20 日成交额取前 N（默认 60） |
| us | Yahoo chart API 日线（调整后 OHLCV，同一代理） | 日线 | 5 年 | 币安 bStock 对应的正股（排除杠杆/反向 ETF：TQQQ/SQQQ/SOXL/SOXS/UVXY 等；SPY 作为基准不入选股池） |

美股用正股的调整后日线而不是 bStock 自身行情，原因是 bStock 上市时间短、历史不足；这与现有 `cmd/cycleic`、`scripts/backtest stocks` 的口径一致，同样存在幸存者偏差（宇宙为当前在售标的），在页面方法说明里写明。

## 目标与评估（冻结，不允许挖掘代码改动）

- 决策时刻 t = 日线收盘；入场价 = t+1 开盘；出场价 = t+1+h 开盘（h 为持有天数）；**目标 = 截面去均值的 h 日收益**（crypto 去全宇宙等权均值，us 去 SPY 同期收益）。
- 周期：crypto h ∈ {1, 3, 7} 天；us h ∈ {5, 10, 20} 天。
- 切分：首次运行时按 60/20/20 确定 train 结束日和 val 结束日，写入 `<root>/<market>/split.json` 后**永久固定**；之后每天新增的数据全部进入 holdout（封存），所以 holdout 会越来越长，train/val 不再随数据漂移。相邻段之间丢弃 h+2 天（purge，只为隔开重叠的前向收益标签；因子回看窗口只用过去数据，不泄漏）。**holdout 只由人工 `judge.py` 打开**，挖掘流程、页面和 JSON 中都没有 holdout 数据。
- 指标（每个因子 × 周期）：
  - 日 RankIC 序列 → 均值、ICIR、t 值（使用每 h 天取一次的非重叠序列，避免重叠收益虚增显著性）；
  - 分 5 层，每 h 天调仓一次的 Q5−Q1 多空：年化收益（净费用）、夏普、换手；crypto 单边费用+滑点 0.08%，us 0.05%；
  - 因子方向：训练集 IC 为负则取反（direction=-1），val 上不再翻转；
  - 冗余度：与已接受因子的日截面平均相关性，≥0.8 标 `redundant`。
- **多重检验门槛**：同一 (market, horizon) 下，ledger 里所有有效 train 试验数 = K，接受阈值 `t_req = max(2, Φ⁻¹(1−0.025/K))`，K 随挖掘不断变大所以门槛单调升高。
- 状态机：
  - `rejected`：train 不满足 t ≥ t_req，或两半 IC 符号不一致；
  - `candidate`：train 达标但 val 未过（val 只看同号且 t ≥ 1.0 的宽松条件，不做精确阈值）；
  - `accepted`：train 达标且 val IC 同号且 val t ≥ 1.65，且 train 与 val 的多空净收益（扣费后）均 > 0；
  - `redundant`：与已 accepted 因子相关性过高。
- 组合（qlib 的 model 层，简化版）：accepted 因子逐截面 z-score 后等权平均，给出组合因子的 train/val IC 和多空净值曲线。

## 挖掘方式（每晚）

1. **更新数据**（增量）；
2. **种子库**：Alpha158 风格程序化因子（约 150 个/周期无关，窗口 5/10/20/30/60）+ nofx 自有的几个经验因子（破位距离、回撤、成交额突变）；首次运行全量评估，之后只评估新增；
3. **变异搜索**：对当前 top 因子随机改窗口、换算子、加/减一项、与另一因子做秩平均，每晚 N=300 个新表达式（去重，按 AST 哈希），**每个都计入 K**；
4. **LLM 提案**（可关）：把 DSL 语法、当前 top-10 / 近期失败样例（只给 train 指标）喂给 `zhipuai-coding-plan/glm-5.3-flash`，要求输出 ≤20 个 JSON 表达式；语法错误计入统计但不计 K；
5. 全部候选统一进冻结评估器，写 ledger、刷新 `zoo.json`/`ensemble.json`/`status.json`/`runs.jsonl`。

## 安全边界

- 表达式只能用白名单 AST：字段 `$open/$high/$low/$close/$volume/$vwap/$ret`、白名单算子、常数、一元/二元算术、比较和 `If`。禁止属性访问、下标、lambda、推导式、导入、任意函数名；窗口 ≤ 120、深度 ≤ 8、节点数 ≤ 40。
- 只读日线缓存，无网络（数据更新是单独脚本）；评估器与 DSL 文件做哈希冻结，LLM 路径只能提交字符串表达式，不能改代码。
- 不触碰线上交易、不接入下单；因子 accepted 只代表“研究层通过 train/val”，不会自动影响任何策略。

## 落盘文件（供 API/前端读取）

根目录 `data/factor_mining/`（运行产物，被 `.gitignore` 的 `data/` 规则忽略；环境变量 `FACTOR_MINING_DIR` 可覆盖）：

- `status.json`：`{updated_at, running, current:{market,stage,started_at}|null, schedule:"每天 02:30", markets:{crypto:{symbols, start, end, bars, universe, data_updated_at}, us:{…}}}`
- `{market}/zoo.json`：
  ```json
  {"market":"crypto","generated_at":"…","split":{"train":["2022-…","2024-…"],"val":[…],"holdout_sealed":true},
   "horizons":{"1":{"trials":1234,"t_req":3.55,"accepted":3,"candidate":9,"rejected":1100,"redundant":4},"3":{…}},
   "factors":[{"id":"f_ab12cd34","expr":"Mean($close,5)/$close-1","source":"seed|mutation|llm","family":"momentum|reversal|volatility|volume|liquidity|price_volume|other",
     "horizon":3,"direction":1,"status":"accepted","first_seen":"2026-10-18","run_id":"…",
     "train":{"ic":0.031,"icir":0.42,"t":3.9,"ls_ret":0.18,"ls_sharpe":1.1,"turnover":0.62,"n":420},
     "val":{"ic":0.02,"icir":0.31,"t":1.9,"ls_ret":0.1,"ls_sharpe":0.8,"turnover":0.6,"n":140},
     "corr_max":0.35,"corr_with":"f_…","ic_monthly":[["2024-01",0.03],…]}]}
  ```
  `factors` 按 status（accepted>candidate>redundant>rejected）再按 train t 降序，rejected 只保留前 200 个。
- `{market}/ensemble.json`：`{"horizons":{"3":{"factor_ids":[…],"train":{ic,icir,ls_sharpe},"val":{…},"curve":{"val":[["2025-06-01",1.0],…]}}}}`
- `{market}/ledger.jsonl`：每个有效试验一行（追加，永不删改）：`{ts,run_id,id,expr,source,horizon,train_t,train_ic,status}`。
- `runs.jsonl`：每次市场运行一行 `{run_id,market,started_at,finished_at,seconds,seed_evaluated,mutation_evaluated,llm_proposed,llm_valid,new_accepted,new_candidate,trials_total,error}`。

## API / 前端

- `GET /api/factor-mining/overview`（鉴权）→ `{status, zoo:{crypto,us}, ensemble:{crypto,us}}`（直接透传上述文件，缺文件返回 null）；
- `GET /api/factor-mining/runs?limit=50` → 最近的 runs；
- 页面 `/factor-mining`（导航「因子挖掘」）：运行状态条、市场/周期切换、四个统计卡（样本覆盖、已试表达式 K 与门槛 t_req、accepted/candidate 数、组合 val IC）、因子榜单（可排序）、点击行展开 IC 月度柱图 + 表达式 + 冗余对象、组合多空净值曲线、运行历史、方法与局限说明。

## 已知局限（写进页面）

日线级别、币圈 2–5 年样本、美股幸存者偏差；变异/LLM 提案共同扩大 K，门槛随之抬高；accepted ≠ 可交易（未建模资金费率、冲击成本、容量）；holdout 未开启前任何指标都只是 train/val 的选择后结果。
