# 模拟盘「候选 vs 对照」晋升报告

研究或因子挖掘得到的有效参数，先跑至少一周模拟盘，再决定是否手动用于正式交易员。报告只读 SQLite，永远不会修改任何交易员配置。

1. 建一对同时开始的模拟交易员，保持初始资金、交易市场、模型、策略、风控及其余配置一致。候选开启待验证参数，对照关闭。人工核对模拟盘身份及配置；脚本不代为核验这些信息。
2. 在 `data/promotion/pairs.json` 填入实验信息，例如：

   ```json
   {
     "pairs": [
       {
         "id": "factor-ensemble",
         "title": "因子 ensemble 参考字段",
         "feature": "enable_factor_ensemble",
         "candidate_trader_id": "候选模拟交易员 id",
         "control_trader_id": "对照模拟交易员 id",
         "live_trader_id": "",
         "started_at": "2026-10-21T00:00:00Z",
         "min_days": 7,
         "min_trades": 20,
         "notes": "写明唯一配置差异及研究依据"
       }
     ]
   }
   ```

   `live_trader_id` 可省略、为 null 或空字符串；填写时额外展示正式交易员同期统计，不参与候选判定。`min_days` 默认 7，不能小于 7；`min_trades` 默认 20，必须为正整数。候选、对照 id 必须不同，实验 id 不可重复。
3. 每日在仓库根运行：

   ```sh
   /Users/zhangyun/workspace/nofx/.venv/bin/python -m scripts.promotion.report
   ```

   默认目录以 `Path(__file__).resolve().parents[2]` 为根：配置及报告目录为 `<repo>/data/promotion`，数据库为 `<repo>/data/data.db`。工作树需要读取主仓库数据时显式指定，例如：

   ```sh
   PROMOTION_DIR=/Users/zhangyun/workspace/nofx/data/promotion \
   NOFX_DB=/Users/zhangyun/workspace/nofx/data/data.db \
   /Users/zhangyun/workspace/nofx/.venv/bin/python -m scripts.promotion.report
   ```

   stdout 显示摘要，`PROMOTION_DIR/report.json` 经同目录临时文件原子替换。缺少 `pairs.json` 或 `pairs` 数组为空时，输出空报告并以 0 退出，无需数据库存在；配置错误或读取失败以 1 退出，保留旧报告。脚本只用 Python 标准库，无网络依赖。
4. 查看 `verdict`、`reasons`、`warnings`：`insufficient` 表示运行时长或任一侧已平仓笔数不足；达到门槛后，候选净盈亏大于 0、期间收益率不低于对照、完整日配对收益差均值非负时为 `ready`，否则为 `hold`。缺少权益或可配对日时不能 `ready`。运行天数按 `started_at` 至报告时刻的实际 UTC 时长计算，不代表系统持续在线，仍需人工核对数据覆盖。
5. 达到条件后，人工核查异常数据、配置差异、资金流和实际运行时间，再手动应用于正式交易员，保持小仓位并准备回滚；报告不会自动上线。

## 统计口径

- 数据库以 `sqlite3.connect('file:…?mode=ro', uri=True)` 打开，并设置 `query_only`。整份报告共用一次读事务，适用于运行中的 WAL 数据库；不使用可能漏读 WAL 的 `immutable=1`，不创建索引、不写数据库。
- 成交按 `status='CLOSED'` 和 `exit_time` 选取 `[started_at, 报告时刻]`，按退出时间排序；可以包含实验前入场、实验期间平仓的交易。`store/position_query.go` 明确 `realized_pnl` 为交易所未扣手续费盈亏，因此净盈亏、胜率、R 都扣除 `fee`。胜率分母包括盈亏为零的成交。`open_positions` 为当前 `status='OPEN'` 的持仓数，包含实验前入场的持仓。
- `R = (realized_pnl - fee) / (abs(entry_price - initial_stop_loss) * quantity)`，严格使用 `quantity`。缺失/非正的价格、初始止损或数量，以及止损距离不大于 `max(1e-12, abs(entry_price)*1e-9)`，均排除并计入 `r_excluded`。无效盈亏或风险额同样排除。`|R|>15` 保留在 avgR 中，并在 `r_extremes` 列出成交 id、退出时间、净盈亏、初始风险额及 R，在 warnings 提示，便于排查历史 +34R 等坏数据。
- 权益时间统一解析为 UTC（无时区的历史时间按 UTC）。每个 UTC 日取最后一个有效快照，缺日仅在首末观测日之间向前填充，不向前回填或向后延伸。MaxDD 是日末权益相对此前日末峰值的最大跌幅，可能低于盘中回撤；期间收益率是末个/首个实际快照权益减一，首个权益非正则不可计算。日末曲线包含最新观测日，若是当天则该点仍未完成。
- 日收益为相邻日末权益比值减一（前一日权益须大于 0），两侧按日取交集后计算 `候选 − 对照`；不使用未结束的 UTC 当天，也不使用没有前一日基准的首个观测日。一天只有一个差值，按日聚类 SE 等于差值样本标准差除以 `sqrt(日数)`。完整配对日少于 10 时，均值仍展示，SE 和 CI 置 null。缺日填充会稀释波动，warnings 会标明填充日数。
- `paired_daily.cumulative_return_diff` 是两侧各自同期首末快照累计收益率之差；`aligned_cumulative_return_diff` 是两侧在共同完整日上分别复利累计后的差。它们的时间覆盖可能不同，报告保留完整日序列以便复核。判定使用前者对应的期间收益率比较及日差均值。
- 两侧有效 R 成交笔数均达到 `min_trades` 后，额外估计交易加权 avgR 差（不是各周均值差的等权平均），按退出时间的 UTC 周一划分联合周。某周仅一侧交易也纳入聚类。若均值为 `avg_c`、`avg_b`，笔数为 `N_c`、`N_b`，则每周影响量为 `sum_c(R-avg_c)/N_c - sum_b(R-avg_b)/N_b`，SE 为 `sqrt(G/(G-1) * sum(周影响量²))`。不足 3 个交易周时不给 SE/CI。周统计可含当前未结束的交易周，不参与判定。
- CI 均为近似正态 95% 区间 `差值 ± 1.96*SE`。比例字段以小数存储（0.01 表示 1%）；时间均为 UTC ISO8601。没有成交时笔数与盈亏为 0、胜率和 avgR 为 null；没有有效快照时权益统计为 null。NaN/Inf 序列化为 null；出现无效成交盈亏时净盈亏汇总和胜率均为 null，不将坏数据冒充零盈亏。
- 本报告不调整出入金、杠杆和敞口差异，不证明实验是随机化或配置严格匹配；应在人工控制这些条件后解读结果。

一周样本统计功效有限：ready 只表示未发现比对照差，不是已证明更优；上线前仍需人工确认并保持小仓位/可回滚

验证：

```sh
/Users/zhangyun/workspace/nofx/.venv/bin/python -m unittest discover -s scripts/promotion/tests -t . -q
```

## 一键建对

`scripts/promotion/create_pair.py` 从正式交易员克隆一对独立模拟账户、策略和交易员。默认 **dry-run**：以 SQLite `mode=ro` 打开数据库，只打印六条 INSERT SQL、绑定参数、对象和合并后的 `pairs.json`，不写数据库、配置、备份或目录。源交易所不能是 paper；源策略必须是 JSON 对象；数据库缺少 `exchanges.paper_config` 时会提示「请先用包含 paper 支持的新版后端启动一次完成迁移」，脚本不会代做迁移。

在本工作树根目录执行预览，将 `<source-trader-id>` 换成现有正式交易员 id：

```sh
PROMOTION_DIR=/Users/zhangyun/workspace/nofx/.claude/worktrees/youthful-bassi-0f1445/data/promotion \
/Users/zhangyun/workspace/nofx/.venv/bin/python -m scripts.promotion.create_pair \
  --db /Users/zhangyun/workspace/nofx/data/data.db \
  --source-trader '<source-trader-id>' \
  --feature-path indicators.enable_factor_ensemble --feature-value true \
  --pair-id factor-ensemble-v1 --title '因子 ensemble 验证' \
  --initial-balance 1000 --taker-bps 5 --maker-bps 2 --slippage-bps 3 --leverage 5 \
  --min-days 7 --min-trades 20 \
  --backup-dir /Users/zhangyun/workspace/nofx/.claude/worktrees/youthful-bassi-0f1445/data/promotion/backups
```

核对后，**在同一命令末尾追加 `--apply`** 才会创建。`--db` 优先于 `NOFX_DB`，二者都未指定时使用主仓库 `data/data.db`（通过工作树 `.git/commondir` 定位）；`PROMOTION_DIR` 默认是脚本所在仓库根的 `data/promotion`，与 report.py 一致。`--backup-dir` 默认是 `PROMOTION_DIR/backups`。上例中的数值也是各参数默认值；费用和滑点须为 0–100 bps，模拟账户默认杠杆须为 1–125 的整数，最短验证期不能低于 7 天，最少成交笔数必须为正整数。

候选和对照各有一个新 paper exchange（UUID），绝不共用虚拟账户；交易所名称均为 `Paper Futures (模拟盘)`，账户名为 `<pair-id>-cand` / `<pair-id>-ctrl`。`--leverage` 写入 paper_config 的 `default_leverage`；交易员原有杠杆、策略风控及其余交易设置均从源复制。策略及交易员名称是 `<title>-候选` / `<title>-对照`；交易员 id 沿用后端的「交易所 UUID 前 8 位 + 模型 id + UUID」风格。两侧均不运行、不参与竞赛，run_version 为 0，策略的 is_active/is_default/is_public 为 0。脚本不自动启动交易员。

`--feature-value` 是 JSON 值（例如 `true`、`0.5`、`'"text"'`）。对照策略 config 与源字符串逐字节相同；候选只替换指定点路径的 JSON 值，其余字节不动。路径中间节点缺失时补建对象，遇到数组、标量或 null 则拒绝；重复 JSON 键也拒绝。候选/对照是否对应「开启/关闭」取决于所给新值与源值，预览时应核对。策略和交易员的其他列完整克隆，包括未来新增列。

apply 先取得写事务并使用 SQLite backup API 生成含已提交 WAL 内容的独立备份，再以单个事务插入 2 个交易所、2 个策略、2 个交易员。备份失败立即中止。`pairs.json` 经同目录临时文件、fsync、原子替换合并写入，保留原有 pair 和额外元数据，拒绝重复 pair-id；插入或配置文件写入失败会回滚全部新行，提交失败会恢复原配置文件。SQLite 与 JSON 文件不能组成跨文件的崩溃原子事务，进程或机器骤停后应对照备份与最终清单核对。

新 pair 的 `live_trader_id` 指向源交易员，`started_at` 为 null，并按约定保留 `note: "首次启动后用 report.py --set-started 回填"`。当前 report.py 没有 `--set-started`，实际应先在界面手动启动两个新交易员，再在首次启动当天执行：

```sh
PROMOTION_DIR=/Users/zhangyun/workspace/nofx/.claude/worktrees/youthful-bassi-0f1445/data/promotion \
/Users/zhangyun/workspace/nofx/.venv/bin/python -m scripts.promotion.create_pair start factor-ensemble-v1
```

`start` 是显式写入 `pairs.json` 的子命令，无需 `--apply`：仅将 started_at 记录为当前 UTC 并更新 note，不访问数据库、不启动交易员；已有时间会拒绝重置，避免缩短统计窗口。回填前不要运行该目录的 report.py（其要求每个 pair 都有有效 started_at）。之后以相同 `PROMOTION_DIR` 和所用数据库的 `NOFX_DB` 运行 report.py，至少验证一周再人工评估。
