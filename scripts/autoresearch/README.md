# 自进化打分研究（离线构建、冻结评价）

仅依赖指定 Python 环境已有的 numpy、pandas 和标准库。未新增网络依赖。直接复用 `scripts.backtest.short_features_study` 的 `load_aux`、`price_features`、`market_features`、`forward_return`、`decision_times`、`causal_check`、`quintiles`、`ic_series`、`spearman`、`cluster_estimate`；5m 载入/重采样复用 `ict_fetch.load` 和 `breakdown_study.Context`。不调用原 `build_samples` 的随机抽样与前置筛选，不调用回测引擎，不访问数据库。

文件：`alt_collect.py` 采集；`build_dataset.py` 构建；`evaluate.py` 冻结评估；`judge.py` 人工留出裁判；`init_work.py` 初始化；`run_round.sh`/`run_round.py` 单轮监督；`program.md` 中文模型规则；`common.py` 路径/持久化；`tests/` 离线测试。

## 本机命令

以下采集、模型调用示例供人工在本机执行；测试不会真实联网。所有默认运行数据均在 `/Users/zhangyun/workspace/nofx/data/`，没有从 worktree 相对路径推导数据目录。

```sh
cd /Users/zhangyun/workspace/nofx/.claude/worktrees/youthful-bassi-0f1445
export AR_PYTHON=/Users/zhangyun/workspace/nofx/.venv/bin/python
export PYTHONDONTWRITEBYTECODE=1

# universe_src.json 必须已有 {"crypto": ["BTCUSDT", ...]}。
# 默认剔除 APRUSDT/MANAUSDT/STXUSDT/USELESSUSDT；所有未剔除币都保留。
"$AR_PYTHON" scripts/autoresearch/alt_collect.py --backfill
"$AR_PYTHON" scripts/autoresearch/alt_collect.py --update
"$AR_PYTHON" scripts/autoresearch/alt_collect.py --update --series oi glsr taker --symbols BTCUSDT ETHUSDT --out-root /Users/zhangyun/workspace/nofx/data/alt_data

# 仅读取本地缓存。5m 在 --cache/ict5m，1h/fund 在 --aux-cache 下；不传 aux 默认同 cache。
"$AR_PYTHON" scripts/autoresearch/build_dataset.py
# 调试：只取 universe 前 N 币；正式实验不要用调试数据替代完整 universe。
"$AR_PYTHON" scripts/autoresearch/build_dataset.py --sample 8 --out-root /Users/zhangyun/workspace/nofx/data/autoresearch/debug
# 可加 --cache DIR --aux-cache DIR --alt-root DIR --universe-json FILE。

"$AR_PYTHON" scripts/autoresearch/init_work.py
# 从另一份 visible/meta 初始化新目录：
# "$AR_PYTHON" scripts/autoresearch/init_work.py --dataset-work DIR --work-dir DIR

"$AR_PYTHON" scripts/autoresearch/evaluate.py --candidate /Users/zhangyun/workspace/nofx/data/autoresearch/work/candidate_long_24.py --side long --horizon 24 --split train
"$AR_PYTHON" scripts/autoresearch/evaluate.py --candidate /Users/zhangyun/workspace/nofx/data/autoresearch/work/candidate_long_24.py --side long --horizon 24 --split val
# 可加 --work-dir DIR --ledger FILE；默认 work/ledger.jsonl。

# 单轮离线检查，不启动 opencode，不联网。
AR_DRY_RUN=1 scripts/autoresearch/run_round.sh
# 真实单轮：仅在本机人工执行；调用 ~/.local/bin/opencode-task。
scripts/autoresearch/run_round.sh
# 可加 --work-dir DIR。每次处理一个组合，40分钟进程组超时。

# 只在正式选定候选、准备消耗 holdout 时人工执行：
"$AR_PYTHON" scripts/autoresearch/judge.py --candidate /Users/zhangyun/workspace/nofx/data/autoresearch/work/candidate_long_24.py --side long --horizon 24 --i-understand-this-spends-the-holdout
# 可加 --private-dir DIR。外部 round/init/evaluate 均不会调用 judge。

# 完整离线验收（mock HTTP + 自动清理的临时合成文件）
"$AR_PYTHON" -m unittest discover -s scripts/autoresearch/tests -v
# 只跑 init_work -> 复制后evaluate -> run_round dry-run CLI 冒烟：
"$AR_PYTHON" -m unittest scripts.autoresearch.tests.test_round.RoundTests.test_init_evaluate_and_dry_run_rotation_cli -v
```

每小时更新的 crontab 示例（仅示例，不自动安装）：

```cron
5 * * * * PYTHONDONTWRITEBYTECODE=1 /Users/zhangyun/workspace/nofx/.venv/bin/python /Users/zhangyun/workspace/nofx/.claude/worktrees/youthful-bassi-0f1445/scripts/autoresearch/alt_collect.py --update
```

不要并发跑两个采集任务；单轮研究有非阻塞文件锁，cron 重叠时立即拒绝。采集日志轮转 1 MiB × 3；每个成功页面即原子保存，单个币/端点失败不会终止后续采集。原始数据缺口保持缺失，不编造历史。增量从最后保存的已发布源桶之后继续。显式 urllib HTTP/HTTPS 代理为 `127.0.0.1:7890`，请求起点至少间隔 0.65 秒，429/418 指数退避并尊重数值 Retry-After。

## 数据与因果定义

`oi/glsr/toplsp/topls_acct/taker/premium/{SYM}.json` 是按 `timestamp` 去重升序的 JSON 对象数组。timestamp 表示可用时间（UTC 毫秒）；保留 `source_timestamp`。premium 使用 `closeTime+1`；taker 聚合桶保守延迟一小时，避免未闭合桶。OI/账户比按接口快照 timestamp；FNG 的秒时间戳归到 UTC 当日 00:00。allForceOrders 已下线，不采集强平。

公开 futures/data 历史最多约30天，不可能补齐去年 OI 等数据；premium 从 2025-10-01 分页，FNG 请求全历史。新特征覆盖率低是预期结果，少于30%标记 `immature`，仍保留该列。覆盖率只统计 train+val，避免以 holdout 成熟程度选特征。

OI `qty/value_chg_4h/24h` 为数量/名义值比例变化；`oi_price_sign_*` 为数量变化和价格收益的符号乘积（-1/0/+1）。`glsr_change_24h` 为比值差；大户相对全局也是比值差。taker `change` 为端点比值变化率，`mean` 为逐小时比值平均。premium 当前/24h均值/7日分位使用 close；分位为 `(低于当前数+0.5×等于当前数)/窗口数`。小时滚动窗口须完整且连续，不补空桶；端点 as-of 查询超过3h为 NaN。FNG 最晚值不晚于 t，超过36h为缺失，7日变化为点数差。

所有数值特征都有 `_xs`：同一时刻全 universe 有限值的平均并列秩/有效币数，范围0..1，缺失仍 NaN。BTC状态对应原始 `btc_bear/btc_chop/btc_bull`，评估时 chop 显示为 `chop`。BTC5m是唯一必须存在的参考文件；其他币缺失价格、aux或alt文件保留整行/对应 NaN。未来标签缺失不会用于筛选特征或分位。保留价格特征中的 `gate` 作为输入列，绝不把它作为样本筛选器。

收益完全复用 forward_return：t 时刻闭合5m之后的下一根开盘（其 open timestamp 正是 t）入场，t+h 的 close 出场；必须有连续5m。long_ret 为小数价格收益、short_ret为相反数。net 每次减0.0016。long_exc=long_ret-BTC_ret，short_exc=-long_exc；这是毛超额，不额外扣对冲腿费用，也不包含资金费现金流。

按去重决策时刻原始数量划60/20/20；从 train/val 尾部丢弃距下段边界不足或等于76h的时刻，使相邻保留段严格相隔>76h。任何同一时刻的币不会分入不同段。三处代表时刻重建5m/aux/alt/FNG前缀并核对所有特征与截面分位。

NPZ 无 pickle/object 数组：`X` (N,F)、`y` (N,18)、`t_ms` int64、`sym` Unicode、`split` Unicode、`feature_names`、`target_names`。work/visible.npz 只有 train/val；private/holdout.npz 只有 holdout。两处各有 meta.json（仅时间范围、数量、覆盖、哈希与因果审计，不含留出样本值）。

## 冻结、计数和报告

初始化不重置已有 ledger 或候选；已冻结的数据目录拒绝重新写入数据集，重建必须显式使用新的 --out-root。evaluate/program/meta/visible/freeze 设置不可写；freeze.json 记录工作文件和复用源码依赖哈希。轮次前后检查哈希，评估器也校验数据与冻结依赖；改变依赖需明确重建实验目录，而不是静默继续。复制后的 evaluate 仍需原仓库及指定Python可访问。

候选仅允许一个 score 函数、1..12个声明特征、numpy 的逐元素安全白名单；禁止循环、推导、私有属性、反射、动态导入、任意函数调用、数组切片与全局归约。子进程只传声明特征数组，60秒超时；不导入候选模块。近似常数、无有效截面、非法代码不入有效试验 ledger。

每个截面至少6个有效币。IC 使用平均并列秩；分位复用原研究，**并列分数不拆组**，空组为 NaN。Q5-Q1 按各时刻的等权分位收益差进行UTC周聚类，95%CI=均值±1.96SE；不是独立同分布置信区间。胜率提升相对整个 split 有限目标的基准，不让候选通过弃分人为降低基准胜率。

fitness 正向条件为 IC t>0、QS均值>0、两半IC非零同号；通过则取两种 t 值最小值，否则负惩罚。缺失不确定性按无证据惩罚；严格JSON用±1e6表示无限t。返回完整训练指标；val仅返回 `valid/pass/val_ic_t_bucket`，不含精确val指标或阈值。K统计该ledger相同side/horizon的全部有效train行（**重复候选也计数**），阈值 `max(2, NormalInv(1-.025/max(K,1)))` 用 erfc 二分反解。

ledger 还包含 `type=round, split=round` 完成标记（失败也记），仅用来轮换：short24 → long24 → short72 → long72。该标记不计入K。提示词只带最后10行的fitness/pass字段；不暴露val精确数值。模型最多8次train迭代，另有监督器轮前基准/轮后候选两次train评估，**这些也计入K**。默认环境变量限制模型评估预算；程序规则同时限制修改次数。环境变量不构成对恶意shell的硬性预算隔离。

监督器将原候选备份best，轮后train没有严格改善就恢复原候选；轮后提案若val pass，其**实际评估的原字节**保存promoted，即使工作候选随后因train无改善而恢复。promoted.log 记录候选哈希；所有轮次写round_log.md及round.log。超时/非法候选/模型失败会回滚候选及小型冻结文件，保留已经发生的有效train试验。大型visible文件仅校验、不按轮复制；若被蓄意破坏需从原数据重建。judge需要保护参数，读取前记录每次已确认尝试（包括失败）的时间/组合/候选哈希。

## 实验边界与未验证部分

按任务约定，visible.npz 包含 val 目标；摘要输出不能阻止同用户权限的模型直接读取这些标签。program.md 禁止读取val目标、越界数据和自行查询val，但这不是OS访问控制。只读权限/哈希/AST保护针对正常合作模型和候选表达式；不能隔离拥有同用户shell权限的恶意模型。严格防窥探需独立用户/OS沙箱、train-only视图及受控val接口，且private应对模型用户不可读；本实现保持要求的文件契约。不要因失败清空ledger或反复重置work规避试验惩罚。

30天公开接口无法追溯更久数据；真实API可达性、代理、API保留区间、字段发布延迟、接口限速和模型调用均未通过联网验证。没有运行真实100币全历史构建或读取真实holdout；测试完全使用mock与自动清理的临时合成数据。生产数据目录写入需本机权限。固定universe存在成分偏差；单一年度市场状态、重叠目标和多轮val反馈仍会过拟合，Bonferroni式试验门槛不能消除所有选择偏差；周聚类正态近似在周数很少时不可靠。这些是研究评分，不是交易执行或收益承诺。
