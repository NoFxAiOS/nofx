"""Offline, causal factor DSL implemented with an AST allowlist and numpy/pandas.

Budgets count all Python AST nodes, including Expression/operator/context nodes;
the Expression root has depth one. ``max_window`` is the largest individual
window/lag, not the accumulated lookback of nested operators. Percentile ranks
use average rank / sample count.
"""

from __future__ import annotations

import ast
import hashlib
import math
import re

import numpy as np
import pandas as pd
from numpy.lib.stride_tricks import sliding_window_view


class ExprError(ValueError):
    """An invalid DSL expression or input panel."""


FIELDS = ("open", "high", "low", "close", "volume", "vwap", "ret")

# arity is exact; window_arg indicates a literal trailing window, except that
# Quantile's window precedes q. This table is also the parser's call allowlist.
OPERATORS: dict[str, dict] = {
    "Ref": {"arity": 2, "doc": "向过去滞后 n 行（1..120）。", "window_arg": True},
    "Delta": {"arity": 2, "doc": "当前值减去 n 行前的值（1..120）。", "window_arg": True},
    "Mean": {"arity": 2, "doc": "过去 n 行的均值。", "window_arg": True},
    "Sum": {"arity": 2, "doc": "过去 n 行的和。", "window_arg": True},
    "Std": {"arity": 2, "doc": "过去 n 行的样本标准差（ddof=1）。", "window_arg": True},
    "Var": {"arity": 2, "doc": "过去 n 行的样本方差（ddof=1）。", "window_arg": True},
    "Max": {"arity": 2, "doc": "过去 n 行的最大值。", "window_arg": True},
    "Min": {"arity": 2, "doc": "过去 n 行的最小值。", "window_arg": True},
    "Median": {"arity": 2, "doc": "过去 n 行的中位数。", "window_arg": True},
    "Skew": {"arity": 2, "doc": "过去 n 行的无偏样本偏度，至少需 3 个值。", "window_arg": True},
    "Kurt": {"arity": 2, "doc": "过去 n 行的无偏超额峰度，至少需 4 个值。", "window_arg": True},
    "Quantile": {"arity": 3, "doc": "Quantile(x,n,q)，线性插值分位数，q 为 0..1 常量。", "window_arg": True},
    "Rank": {"arity": 2, "doc": "当前值在过去 n 行中的平均秩/n。", "window_arg": True},
    "IdxMax": {"arity": 2, "doc": "距窗口最近一次最高值的行数/n，今日为 0。", "window_arg": True},
    "IdxMin": {"arity": 2, "doc": "距窗口最近一次最低值的行数/n，今日为 0。", "window_arg": True},
    "Corr": {"arity": 3, "doc": "Corr(x,y,n)，过去 n 行的 Pearson 相关系数。", "window_arg": True},
    "Cov": {"arity": 3, "doc": "Cov(x,y,n)，过去 n 行的样本协方差（ddof=1）。", "window_arg": True},
    "Slope": {"arity": 2, "doc": "对时间序号 0..n-1 做带截距线性回归的斜率。", "window_arg": True},
    "Rsquare": {"arity": 2, "doc": "时间线性回归的 R²，常数窗口为 NaN。", "window_arg": True},
    "Resi": {"arity": 2, "doc": "时间线性回归最后一点的残差。", "window_arg": True},
    "EMA": {"arity": 2, "doc": "alpha=2/(n+1)、adjust=False 的 EMA，累计至少 n 个有效值。", "window_arg": True},
    "WMA": {"arity": 2, "doc": "窗口从旧到新使用 1..n 权重的均值。", "window_arg": True},
    "Mad": {"arity": 2, "doc": "窗口内相对均值的平均绝对偏差。", "window_arg": True},
    "CSRank": {"arity": 1, "doc": "逐行跨标的的平均秩/有效标的数。", "window_arg": False},
    "CSZ": {"arity": 1, "doc": "逐行截面 z-score（ddof=1），标准差为零时为 NaN。", "window_arg": False},
    "CSDemean": {"arity": 1, "doc": "逐行减去截面均值。", "window_arg": False},
    "Abs": {"arity": 1, "doc": "逐元素绝对值。", "window_arg": False},
    "Sign": {"arity": 1, "doc": "逐元素符号（-1、0、1）。", "window_arg": False},
    "Log": {"arity": 1, "doc": "自然对数，非正值为 NaN。", "window_arg": False},
    "Sqrt": {"arity": 1, "doc": "平方根，负值为 NaN。", "window_arg": False},
    "Power": {"arity": 2, "doc": "Power(x,p)，p 必须为数值字面量。", "window_arg": False},
    "Clip": {"arity": 3, "doc": "Clip(x,lo,hi)，上下界为数值字面量且 lo<=hi。", "window_arg": False},
    "Greater": {"arity": 2, "doc": "逐元素取两者较大值，NaN 传播。", "window_arg": False},
    "Less": {"arity": 2, "doc": "逐元素取两者较小值，NaN 传播。", "window_arg": False},
    "If": {"arity": 3, "doc": "If(比较式,x,y)，比较为真取 x，否则取 y，NaN 按假处理。", "window_arg": False},
}

_WINDOW_POS = {
    name: (2 if name in {"Corr", "Cov"} else 1)
    for name, spec in OPERATORS.items() if spec["window_arg"]
}
_BINARY = {ast.Add: "+", ast.Sub: "-", ast.Mult: "*", ast.Div: "/"}
_COMPARE = {ast.Gt: ">", ast.Lt: "<", ast.GtE: ">=", ast.LtE: "<="}
_FIELD_RE = re.compile(r"\$([A-Za-z_][A-Za-z_0-9]*)")


def _literal(node: ast.AST) -> float:
    """Read a finite numerical literal, optionally with a single minus sign."""
    sign = 1
    if isinstance(node, ast.UnaryOp) and isinstance(node.op, ast.USub):
        sign, node = -1, node.operand
    if not isinstance(node, ast.Constant) or type(node.value) not in (int, float):
        raise ExprError("参数必须为数值字面量（可带一元负号）")
    try:
        value = sign * float(node.value)
    except (OverflowError, ValueError) as exc:
        raise ExprError("常数必须为有限实数") from exc
    if not math.isfinite(value):
        raise ExprError("常数必须为有限实数")
    return value


def _parse(expr: str) -> tuple[ast.AST, dict]:
    if not isinstance(expr, str) or not expr.strip():
        raise ExprError("表达式必须为非空字符串")
    # Reserve the parser's field prefix so bare Python identifiers cannot
    # masquerade as dollar-prefixed fields.
    if re.search(r"\bF_[A-Za-z_0-9]*\b", expr):
        raise ExprError("字段必须使用 $name 形式")
    try:
        tree = ast.parse(_FIELD_RE.sub(r"F_\1", expr.strip()), mode="eval")
    except (SyntaxError, ValueError, RecursionError, MemoryError) as exc:
        raise ExprError("表达式语法错误或过于复杂") from exc

    nodes = depth = 0
    stack = [(tree, 1)]
    while stack:
        node, level = stack.pop()
        nodes += 1
        depth = max(depth, level)
        if nodes > 40 or depth > 8:
            raise ExprError("表达式限制：节点数 <=40，深度 <=8")
        stack.extend((child, level + 1) for child in ast.iter_child_nodes(node))

    root = tree.body
    max_window = 0
    fields = set()
    stack = [root]
    while stack:
        node = stack.pop()
        children = []
        if isinstance(node, ast.Constant):
            _literal(node)
        elif isinstance(node, ast.Name):
            if not node.id.startswith("F_") or node.id[2:] not in FIELDS:
                raise ExprError(f"未知字段或标识符：{node.id}")
            fields.add(node.id[2:])
            if node.id == "F_ret":
                max_window = max(max_window, 1)
        elif isinstance(node, ast.UnaryOp) and isinstance(node.op, ast.USub):
            children = [node.operand]
        elif isinstance(node, ast.BinOp) and type(node.op) in _BINARY:
            children = [node.left, node.right]
        elif isinstance(node, ast.Compare):
            if len(node.ops) != 1 or type(node.ops[0]) not in _COMPARE:
                raise ExprError("只允许单个 >、<、>=、<= 比较")
            children = [node.left, node.comparators[0]]
        elif isinstance(node, ast.Call):
            if not isinstance(node.func, ast.Name) or node.func.id not in OPERATORS:
                raise ExprError("只允许调用白名单算子")
            name = node.func.id
            if node.keywords or len(node.args) != OPERATORS[name]["arity"]:
                raise ExprError(f"{name} 参数数量错误或使用了关键字参数")
            if name in _WINDOW_POS:
                win = node.args[_WINDOW_POS[name]]
                lower = 1 if name in {"Ref", "Delta"} else 2
                if (not isinstance(win, ast.Constant) or type(win.value) is not int
                        or not lower <= win.value <= 120):
                    raise ExprError(f"{name} 窗口必须为 {lower}..120 的整数字面量")
                max_window = max(max_window, win.value)
            if name == "Quantile" and not 0 <= _literal(node.args[2]) <= 1:
                raise ExprError("Quantile 的 q 必须在 0..1")
            if name == "Power":
                _literal(node.args[1])
            if name == "Clip" and _literal(node.args[1]) > _literal(node.args[2]):
                raise ExprError("Clip 必须满足 lo<=hi")
            if name == "If" and not isinstance(node.args[0], ast.Compare):
                raise ExprError("If 的条件必须为比较表达式")
            children = node.args
        else:
            raise ExprError(f"禁止的语法：{type(node).__name__}")
        stack.extend(children)
    return root, {"nodes": nodes, "depth": depth, "max_window": max_window,
                  "fields": sorted(fields)}


def validate_expr(expr: str) -> dict:
    """Validate syntax, literal parameters and resource budgets; return metadata."""
    return _parse(expr)[1]


def _number(value: float) -> str:
    return str(int(value)) if value.is_integer() else repr(value)


def _canonical(node: ast.AST) -> str:
    if isinstance(node, ast.Constant):
        return _number(_literal(node))
    if isinstance(node, ast.Name):
        return "$" + node.id[2:]
    if isinstance(node, ast.UnaryOp):
        if isinstance(node.operand, ast.Constant):
            return _number(_literal(node))
        return "(-" + _canonical(node.operand) + ")"
    if isinstance(node, ast.BinOp):
        left, right = _canonical(node.left), _canonical(node.right)
        if isinstance(node.op, (ast.Add, ast.Mult)):
            left, right = sorted((left, right))
        return f"({left}{_BINARY[type(node.op)]}{right})"
    if isinstance(node, ast.Compare):
        return f"({_canonical(node.left)}{_COMPARE[type(node.ops[0])]}{_canonical(node.comparators[0])})"
    return f"{node.func.id}({','.join(_canonical(arg) for arg in node.args)})"


def canonical(expr: str) -> str:
    """Canonical formatting; sort the two operands of +/* without reassociating."""
    return _canonical(_parse(expr)[0])


def expr_hash(expr: str) -> str:
    """Stable short identifier derived solely from the canonical DSL."""
    return "f_" + hashlib.sha1(canonical(expr).encode("utf-8")).hexdigest()[:8]


def _finite(value):
    return np.where(np.isfinite(value), value, np.nan)


def _panels(panels: dict) -> tuple[dict, pd.DataFrame]:
    if not isinstance(panels, dict) or any(name not in panels for name in FIELDS[:-1]):
        raise ExprError("panels 必须包含 open/high/low/close/volume/vwap 面板")
    base = panels["close"]
    if (not isinstance(base, pd.DataFrame) or not isinstance(base.index, pd.DatetimeIndex)
            or not base.index.is_monotonic_increasing or base.index.hasnans):
        raise ExprError("面板必须为 DataFrame，日期索引必须为升序 DatetimeIndex")
    arrays = {}
    for name in FIELDS[:-1]:
        panel = panels[name]
        if (not isinstance(panel, pd.DataFrame) or not panel.index.equals(base.index)
                or not panel.columns.equals(base.columns)):
            raise ExprError("所有面板必须具有相同的 index/columns")
        try:
            arrays[name] = _finite(panel.to_numpy(dtype=float, na_value=np.nan))
        except (TypeError, ValueError, OverflowError) as exc:
            raise ExprError("面板值必须为数值") from exc
    # Always derive ret; a caller-supplied ret must not override this definition.
    lag = np.full(base.shape, np.nan)
    lag[1:] = arrays["close"][:-1]
    with np.errstate(all="ignore"):
        arrays["ret"] = _finite(arrays["close"] / lag - 1)
    return arrays, base


def _window_op(name: str, x: np.ndarray, n: int) -> np.ndarray:
    """Vectorized complete-window statistics, including OLS with an intercept."""
    out = np.full(x.shape, np.nan)
    if (len(x) < n or (name == "Skew" and n < 3)
            or (name == "Kurt" and n < 4)):
        return out
    windows = sliding_window_view(x, n, axis=0)
    if name in {"Slope", "Rsquare", "Resi", "Skew", "Kurt"}:
        # Subtract the first observation before centering for stable statistics
        # on high price levels, and exactly zero variance on constant windows.
        centered = windows - windows[..., :1]
        centered -= centered.mean(axis=-1, keepdims=True)
        if name in {"Skew", "Kurt"}:
            # Window-local moments are essential: pandas rolling skew/kurt can
            # change rounding when future rows change its numerical centering.
            ss_y = np.einsum("tcn,tcn->tc", centered, centered)
            if name == "Skew":
                third = np.einsum("tcn,tcn,tcn->tc", centered, centered, centered)
                values = n * np.sqrt(n - 1) / (n - 2) * third / ss_y ** 1.5
                values = np.where(ss_y == 0, 0., values)
            else:
                fourth = np.einsum("tcn,tcn,tcn,tcn->tc", centered, centered, centered, centered)
                values = ((n - 1) / ((n - 2) * (n - 3))
                          * ((n + 1) * (n * fourth / ss_y ** 2 - 3) + 6))
                # Match pandas rolling.kurt's convention on constant windows.
                values = np.where(ss_y == 0, -3., values)
        else:
            time = np.arange(n, dtype=float) - (n - 1) / 2
            ss_time = np.dot(time, time)
            cross = np.einsum("tcn,n->tc", centered, time)
            slope = cross / ss_time
            if name == "Slope":
                values = slope
            elif name == "Resi":
                values = centered[..., -1] - slope * time[-1]
            else:
                ss_y = np.einsum("tcn,tcn->tc", centered, centered)
                values = np.clip(cross ** 2 / (ss_time * ss_y), 0, 1)
    elif name in {"IdxMax", "IdxMin"}:
        extreme = np.argmax if name == "IdxMax" else np.argmin
        values = extreme(windows[..., ::-1], axis=-1) / n
        values = np.where(np.isfinite(windows).all(axis=-1), values, np.nan)
    elif name == "WMA":
        weights = np.arange(1, n + 1, dtype=float)
        values = np.einsum("tcn,n->tc", windows, weights) / weights.sum()
    else:  # Mad: mean absolute deviation about the window mean.
        values = np.abs(windows - windows.mean(axis=-1, keepdims=True)).mean(axis=-1)
    out[n - 1:] = values
    return out


_ROLLING = {"Mean": "mean", "Sum": "sum", "Std": "std", "Var": "var",
            "Max": "max", "Min": "min", "Median": "median", "Rank": "rank"}


def evaluate_expr(expr: str, panels: dict) -> pd.DataFrame:
    """Interpret a validated AST, returning a finite-or-NaN panel of equal shape."""
    root, _ = _parse(expr)
    arrays, base = _panels(panels)

    def as_array(value):
        return np.broadcast_to(value, base.shape)

    def run(node):
        if isinstance(node, ast.Constant):
            return float(node.value)
        if isinstance(node, ast.Name):
            return arrays[node.id[2:]]
        if isinstance(node, ast.UnaryOp):
            return -run(node.operand)
        if isinstance(node, ast.BinOp):
            left, right = run(node.left), run(node.right)
            fn = {ast.Add: np.add, ast.Sub: np.subtract,
                  ast.Mult: np.multiply, ast.Div: np.divide}[type(node.op)]
            return _finite(fn(left, right))
        if isinstance(node, ast.Compare):
            left, right = run(node.left), run(node.comparators[0])
            fn = {ast.Gt: np.greater, ast.Lt: np.less,
                  ast.GtE: np.greater_equal, ast.LtE: np.less_equal}[type(node.ops[0])]
            return np.where(np.isnan(left) | np.isnan(right), np.nan,
                            fn(left, right).astype(float))

        name = node.func.id
        args = [run(arg) for arg in node.args]
        x = args[0]
        if name in _WINDOW_POS:
            n = int(args[_WINDOW_POS[name]])
            x = as_array(x)
            if name in {"Ref", "Delta"}:
                lag = np.full(base.shape, np.nan)
                lag[n:] = x[:-n]
                return lag if name == "Ref" else _finite(x - lag)
            if name in {"Slope", "Rsquare", "Resi", "IdxMax", "IdxMin", "WMA", "Mad", "Skew", "Kurt"}:
                return _window_op(name, x, n)
            frame = pd.DataFrame(x)
            if name == "EMA":
                return frame.ewm(span=n, adjust=False, min_periods=n).mean().to_numpy()
            rolling = frame.rolling(n, min_periods=n)
            if name in {"Corr", "Cov"}:
                return getattr(rolling, name.lower())(pd.DataFrame(as_array(args[1]))).to_numpy()
            if name == "Quantile":
                return rolling.quantile(args[2], interpolation="linear").to_numpy()
            if name == "Rank":
                return rolling.rank(method="average", pct=True).to_numpy()
            return getattr(rolling, _ROLLING[name])().to_numpy()
        if name in {"CSRank", "CSZ", "CSDemean"}:
            frame = pd.DataFrame(as_array(x))
            if name == "CSRank":
                return frame.rank(axis=1, method="average", pct=True).to_numpy()
            centered = frame.sub(frame.mean(axis=1), axis=0)
            if name == "CSDemean":
                return centered.to_numpy()
            return centered.div(frame.std(axis=1, ddof=1).replace(0, np.nan), axis=0).to_numpy()
        if name == "If":
            return np.where(np.isfinite(x) & (x != 0), args[1], args[2])
        if name == "Abs":
            return np.abs(x)
        if name == "Sign":
            return np.sign(x)
        if name == "Log":
            return np.where(np.asarray(x) > 0, np.log(x), np.nan)
        if name == "Sqrt":
            return np.where(np.asarray(x) >= 0, np.sqrt(x), np.nan)
        if name == "Power":
            return _finite(np.power(x, args[1]))
        if name == "Clip":
            return np.clip(x, args[1], args[2])
        if name == "Greater":
            return np.maximum(x, args[1])
        if name == "Less":
            return np.minimum(x, args[1])
        raise ExprError(f"未实现的算子：{name}")

    with np.errstate(all="ignore"):
        result = _finite(as_array(run(root)))
    return pd.DataFrame(result, index=base.index, columns=base.columns)


def dsl_help() -> str:
    """Generate the Chinese DSL reference for factor proposal prompts."""
    lines = [
        "因子表达式 DSL（纯离线、只使用当日及过去数据）",
        "字段：" + "、".join("$" + field for field in FIELDS),
        "$ret 自动派生为 $close/Ref($close,1)-1。",
        "允许 int/float 有限常数、括号、+ - * /、一元负号及 > < >= <= 单比较；比较返回 1.0/0.0，缺失值返回 NaN。",
        "除零、无穷值和非法对数/平方根返回 NaN；If 的 NaN 条件按假处理。",
        "窗口是 2..120 的整数字面量（Ref/Delta 为 1..120），包含当日且 min_periods=n；EMA 为累计有效值预热。",
        "Quantile 的 q、Power 的 p、Clip 的 lo/hi 必须为数值字面量，可带负号；q 在 0..1、lo<=hi。",
        "百分位为平均并列秩/有效样本数，范围 0..1；IdxMax/IdxMin 并列取最近一次，距今日行数/n。",
        "最多 40 个 Python AST 节点、深度 8（Expression 根为 1，计入函数名、运算符及上下文节点）；max_window 为最大单个窗口/滞后，不累加。",
        "禁止属性、下标、字符串、lambda、推导式、关键字参数及任意非白名单调用。",
        "算子表（名称 / 参数个数 / 是否有窗口 / 说明）：",
    ]
    lines.extend(f"{name} / {spec['arity']} / {'是' if spec['window_arg'] else '否'} / {spec['doc']}"
                 for name, spec in OPERATORS.items())
    lines.extend(["例子：Mean($close,20)/$close-1", "例子：Corr($close,Log($volume+1),20)",
                  "例子：If($close>Ref($close,1),$ret,0)", "例子：CSRank(Std($ret,20))"])
    return "\n".join(lines)
