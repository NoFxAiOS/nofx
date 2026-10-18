# 自动评分研究规则（冻结）

你只负责提出简单、有金融含义的打分假设，寻找“分数越高，该方向未来扣费收益越高、胜率越高”。每次由外部脚本指定唯一 side/horizon 组合。只可编辑当轮 candidate_{side}_{horizon}.py 和 notes.md，不能编辑其他组合。

只可读取当前工作目录内 visible.npz、meta.json、evaluate.py、program.md、自己的 candidate 和 notes.md，以及外部提供的历史摘要。visible.npz 的 split 标记包含 train/val；禁止直接查看 val 的目标列、手算 val 指标或通过 notes 存储 val 反馈细节；只有冻结评估器可为外部脚本读取 val 标签。禁止读取或列出 data/autoresearch/private；禁止读取 visible.npz 之外的任何数据路径或缓存。禁止联网、访问进程、数据库、启动子代理。禁止修改/删除/替换/重置 evaluate.py、meta.json、visible.npz、ledger.jsonl、freeze.json、best/、promoted/ 和日志。不要读取 ledger 或轮次内部文件；历史摘要已由外部去掉精确 val 指标。

每轮最多 8 次修改与 train 评估；每次修改后运行：

    python evaluate.py --candidate candidate_SIDE_H.py --side SIDE --horizon H --split train --work-dir .

把 SIDE/H 换成本轮参数。只能运行 train，禁止自行查询 val 或 judge；val 由外部脚本每轮查询一次。无效语法不计入统计有效试验，但仍占你的迭代次数。每次都在 notes.md 写假设、选用特征、train 结果与解释，多次无改进就停止并写结论。不能通过重置记账、复制评估器、增加重复试验或读未来数据绕过预算。试验越多，val 门槛越高。

候选格式：

    import numpy as np
    FEATURES = ["ret_24h_xs"]
    def score(X):
        return -X["ret_24h_xs"]

FEATURES 为 1..12 个 meta 中的特征；只接收所列特征的 N 长度 float 数组，不能接收目标、时间、币名或 split。score 返回 N 个分数，越高越该做本轮方向。只允许逐元素 numpy 运算（如 np.where、np.clip、np.nan_to_num、np.tanh、np.log1p）。禁止循环、逐行遍历、函数嵌套、反射、文件操作、导入其他库、数组索引或切片、全局归约和跨时刻估计。只能 X["已声明特征"] 访问数组，不可用近似常数得分。缺失值可用简单且有含义的中性值处理，不能按未来标签是否存在筛选。_xs 是同一决策时刻全币种分位（含并列平均秩）；全市场常数特征的 _xs 不代表截面区分力。

优先动量/反转、拥挤度、基差、情绪、波动状态等解释清楚的假设。先尝试 1~3 个特征，勿为提高 train 指标堆特征、搜大量阈值或拟合特定币/日期。低于30%覆盖的特征在 meta 标记 immature，尤其新的 OI 数据可能只覆盖近30天；不能把这个窄窗口的高分当长期证据。

train 输出 IC、周聚类不确定性、五分位收益/胜率、前后半和BTC状态稳定性。fitness 只在正IC t、正Q5-Q1且两半同号时为正。val 只披露 pass 和 t 桶；不要推测或追逐精确 val 数字。真实评价还需人工消耗 holdout，一旦看过就不能再当独立样本。

这一年是单一市场阶段，固定事后币种列表也有成分偏差。不要追求高 train 分数，稳定性比数值更重要；没有改进也是合理结论。完成后保留你认为最合理的本轮候选及 notes.md。
