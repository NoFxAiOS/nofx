"""FROZEN evaluator. Candidates receive declared feature arrays ONLY.
AST permits pointwise numpy expressions, no I/O/reflection/reductions/row loops.
This is defense in depth, not an OS sandbox for an untrusted same-user shell.
Never reads holdout; judge.py is the sole holdout entry point.
"""
import argparse
import ast
import datetime as dt
import fcntl
import hashlib
import json
import math
import multiprocessing as mp
import os
from pathlib import Path
import sys
import types

# Also works when copied to data/autoresearch/work by init_work.py.
REPO = Path('/Users/zhangyun/workspace/nofx/.claude/worktrees/youthful-bassi-0f1445')
if not REPO.exists():  # worktree removed after merge: fall back to the main checkout
    REPO = Path('/Users/zhangyun/workspace/nofx')
source_root = Path(__file__).resolve().parents[2]
if (source_root / 'scripts/autoresearch/evaluate.py').resolve() == Path(__file__).resolve():
    REPO = source_root
# A copied evaluator lives under the main checkout's data/, which may be a
# DIFFERENT repository from this worktree. Never discover code via its ancestors.
sys.path.insert(0, str(REPO))
import numpy as np
import pandas as pd
from scripts.backtest import short_features_study as sf
from scripts.autoresearch.common import RESEARCH, read_json, sha256

NP_FUNCTIONS = set(('abs absolute sign where clip minimum maximum fmin fmax '
                    'nan_to_num isnan isfinite isinf sqrt square power log log1p exp expm1 '
                    'tanh sin cos arctan floor ceil round subtract add multiply divide '
                    'logical_and logical_or logical_not').split())
NP_CONSTANTS = {'nan', 'inf', 'pi', 'float64'}
NODE_TYPES = (ast.Module, ast.Import, ast.alias, ast.Assign, ast.FunctionDef, ast.arguments, ast.arg,
              ast.Return, ast.Expr, ast.Constant, ast.List, ast.Tuple, ast.Dict, ast.Name, ast.Load,
              ast.Store, ast.Subscript, ast.Attribute, ast.Call, ast.keyword, ast.BinOp, ast.UnaryOp,
              ast.Compare, ast.Add, ast.Sub, ast.Mult, ast.Div, ast.Pow, ast.Mod, ast.UAdd, ast.USub,
              ast.BitAnd, ast.BitOr, ast.Invert, ast.Eq, ast.NotEq, ast.Lt, ast.LtE, ast.Gt, ast.GtE)


def validate_candidate(source, feature_names):
    if len(source.encode()) > 64 * 1024:
        raise ValueError('candidate exceeds 64 KiB')
    tree = ast.parse(source)
    features, function = None, None
    imports = 0
    for node in tree.body:
        if isinstance(node, ast.Expr) and isinstance(node.value, ast.Constant) and isinstance(node.value.value, str):
            continue
        if isinstance(node, ast.Import):
            imports += 1
            if len(node.names) != 1 or node.names[0].name != 'numpy' or node.names[0].asname != 'np' or imports > 1:
                raise ValueError('only import numpy as np is allowed')
        elif isinstance(node, ast.Assign) and len(node.targets) == 1 and isinstance(node.targets[0], ast.Name) and node.targets[0].id == 'FEATURES':
            if features is not None:
                raise ValueError('duplicate FEATURES')
            features = ast.literal_eval(node.value)
        elif isinstance(node, ast.FunctionDef) and node.name == 'score' and function is None:
            function = node
        else:
            raise ValueError('module may contain only FEATURES and score')
    if not isinstance(features, list) or not 1 <= len(features) <= 12 or not all(isinstance(f, str) for f in features):
        raise ValueError('FEATURES must list 1..12 feature names')
    if len(set(features)) != len(features) or set(features) - set(feature_names):
        raise ValueError('duplicate or unknown feature')
    if function is None or len(function.args.args) != 1 or function.args.args[0].arg != 'X' or function.decorator_list:
        raise ValueError('expected def score(X)')
    if function.args.defaults or function.args.kw_defaults or function.args.kwonlyargs or function.args.posonlyargs or function.args.vararg or function.args.kwarg:
        raise ValueError('score cannot have defaults/extra arguments')
    local = {'X', 'np', 'dict'}
    for node in ast.walk(function):
        if isinstance(node, ast.Assign):
            for target in node.targets:
                if not isinstance(target, ast.Name) or target.id in ('np', 'X', 'FEATURES', 'dict') or target.id.startswith('_'):
                    raise ValueError('only local variable assignments allowed')
                local.add(target.id)
    for node in ast.walk(tree):
        if not isinstance(node, NODE_TYPES):
            raise ValueError(f'forbidden syntax: {type(node).__name__} (no loops/comprehensions)')
        if isinstance(node, ast.FunctionDef) and node is not function:
            raise ValueError('no nested functions')
        if isinstance(node, ast.Name) and node.id not in local | {'FEATURES', 'score'}:
            raise ValueError(f'forbidden name: {node.id}')
        if isinstance(node, ast.Name) and (node.id.startswith('_') or node.id in {'open', 'exec', 'eval', 'compile', '__import__', 'getattr', 'os', 'sys', 'subprocess', 'socket', 'pathlib', 'importlib'}):
            raise ValueError('private names forbidden')
        if isinstance(node, ast.Attribute):
            if not isinstance(node.value, ast.Name) or node.value.id != 'np' or node.attr not in NP_FUNCTIONS | NP_CONSTANTS:
                raise ValueError('only approved pointwise numpy attributes allowed')
        if isinstance(node, ast.Call):
            if not isinstance(node.func, ast.Attribute) or node.func.attr not in NP_FUNCTIONS:
                raise ValueError('only approved numpy calls allowed')
            if any(k.arg is None or k.arg == 'out' for k in node.keywords):
                raise ValueError('no unpacking or out= mutation')
        if isinstance(node, ast.Subscript):
            if not (isinstance(node.value, ast.Name) and node.value.id == 'X' and
                    isinstance(node.slice, ast.Constant) and node.slice.value in features):
                raise ValueError('only X[declared literal feature] indexing allowed')
    # Remove import: the worker supplies a capability-limited numpy facade.
    tree.body = [n for n in tree.body if not isinstance(n, ast.Import)]
    return tree, features


def _score_worker(connection, tree, arrays):
    try:
        facade = types.SimpleNamespace(**{k: getattr(np, k) for k in NP_FUNCTIONS | NP_CONSTANTS})
        env = {'__builtins__': {}, 'np': facade, 'dict': dict}
        for value in arrays.values():
            value.flags.writeable = False
        exec(compile(tree, '<validated-candidate>', 'exec'), env)
        result = np.asarray(env['score'](arrays), dtype=float)
        if result.shape != (len(next(iter(arrays.values()))),):
            raise ValueError('score must return shape (N,)')
        connection.send((True, result))
    except BaseException as exc:
        connection.send((False, f'{type(exc).__name__}: {str(exc)[:200]}'))
    finally:
        connection.close()


def score_candidate(source, feature_names, X, timeout=60):
    tree, features = validate_candidate(source, feature_names)
    arrays = {f: np.asarray(X[:, feature_names.index(f)], dtype=float).copy() for f in features}
    # Fork only a worker executing validated expressions; no model code imported.
    ctx = mp.get_context('fork')
    receive, send = ctx.Pipe(duplex=False)
    process = ctx.Process(target=_score_worker, args=(send, tree, arrays), daemon=True)
    process.start()
    send.close()
    try:
        if not receive.poll(timeout):
            raise ValueError('score timeout (60s) or worker failure')
        ok, result = receive.recv()
        if not ok:
            raise ValueError(result)
    finally:
        if process.is_alive():
            process.terminate()
        process.join()
        receive.close()
    if result.shape != (len(X),):
        raise ValueError('score must return shape (N,)')
    finite = result[np.isfinite(result)]
    if len(finite) < 6 or np.unique(finite).size < 2 or np.ptp(finite) <= 1e-10 * max(1., float(np.max(np.abs(finite)))):
        raise ValueError('approximately constant or insufficient finite scores')
    return result, features


def metrics(t, scores, net, excess, states, n_features):
    # All cross-sectional ranks are formed before filtering outcomes, so future
    # target availability cannot change a coin's membership in Q1/Q5.
    d = pd.DataFrame(dict(t_ms=t, score=scores, net=net, excess=excess, regime=states))
    d = d.replace([np.inf, -np.inf], np.nan)
    d['q'] = sf.quintiles(d, d.score)
    counts = d.groupby('t_ms').score.transform('count')
    d = d[counts >= 6]
    series = sf.ic_series(d, 'score', 'net')
    ic = sf.cluster_estimate(series.t_ms, series.ic)
    if not len(series):
        raise ValueError('no nonconstant cross sections with >=6 paired observations')
    groups = d.groupby(['t_ms', 'q'])[['net', 'excess']].mean()
    wide = groups.net.unstack('q').reindex(columns=[1., 2., 3., 4., 5.])
    qmeans = wide.mean().to_numpy()
    qs = sf.cluster_estimate(wide.index.to_numpy(), (wide[5.] - wide[1.]).to_numpy())
    top = d[d.q == 5].dropna(subset=['net'])
    all_net = d.dropna(subset=['net'])
    # Compare Q5 against the FULL universe, including rows the candidate abstains
    # from scoring. Missing predictions cannot manufacture an easier baseline.
    baseline = pd.DataFrame(dict(t_ms=t, net=net)).replace([np.inf, -np.inf], np.nan).dropna()
    top_win = float((top.net > 0).mean()) if len(top) else np.nan
    baseline_win = float((baseline.net > 0).mean()) if len(baseline) else np.nan
    times = np.unique(t)
    cut = times[len(times) // 2]
    scopes = {'first_half': series[series.t_ms < cut], 'second_half': series[series.t_ms >= cut]}
    scopes.update({s: series[series.regime == s] for s in ('btc_bear', 'chop', 'btc_bull')})
    stability = {k: sf.cluster_estimate(v.t_ms, v.ic) for k, v in scopes.items()}
    for stat in stability.values():
        stat['ic_sign'] = int(np.sign(stat['mean'])) if np.isfinite(stat['mean']) else None
    a, b = stability['first_half']['mean'], stability['second_half']['mean']
    same = bool(np.isfinite([a, b]).all() and a * b > 0)
    # Missing uncertainty is no evidence; cap infinities from exact synthetic
    # relationships at a finite sentinel for strict JSON and ledger ordering.
    def bounded(v):
        return float(np.clip(v, -1e6, 1e6)) if not np.isnan(v) else -1.
    it, qt = bounded(ic['t']), bounded(qs['t'])
    good = it > 0 and qs['mean'] > 0 and same
    fitness = min(it, qt) if good else -max(.001, abs(min(it, qt, 0.)))
    finite_scores = scores[np.isfinite(scores)]
    return dict(valid=True, fitness=float(fitness), ic=ic, qs=qs,
                quintile_net_means=qmeans.tolist(), q5_net=float(qmeans[-1]), q1_net=float(qmeans[0]),
                monotonicity=sf.spearman(np.arange(1, 6), qmeans),
                q5_win_rate=top_win, baseline_win_rate=baseline_win, win_uplift=top_win - baseline_win,
                q5_excess=float(top.excess.mean()), baseline_excess=float(all_net.excess.mean()),
                excess_ic=sf.cluster_estimate(*(lambda s: (s.t_ms, s.ic))(sf.ic_series(d, 'score', 'excess'))),
                stability=stability, halves_same_sign=same,
                complexity=dict(n_features=n_features, unique_scores=int(np.unique(finite_scores).size)),
                score_coverage=float(np.isfinite(scores).mean()), rows=len(d))


def t_required(k):
    """Normal upper-tail inverse using erfc directly (no 1-p cancellation)."""
    tail = .025 / max(1, k)
    lo, hi = 0., 40.
    for _ in range(100):
        mid = (lo + hi) / 2
        if .5 * math.erfc(mid / math.sqrt(2)) > tail:
            lo = mid
        else:
            hi = mid
    return max(2., (lo + hi) / 2)


def ledger_rows(path):
    path = Path(path)
    if not path.exists():
        return []
    with path.open() as stream:
        fcntl.flock(stream, fcntl.LOCK_SH)
        return [json.loads(line) for line in stream if line.strip()]


def valid_trials(path, side, horizon):
    return sum(r.get('side') == side and r.get('horizon') == horizon and r.get('split') == 'train'
               for r in ledger_rows(path))


def clean(value):
    if isinstance(value, dict):
        return {k: clean(v) for k, v in value.items()}
    if isinstance(value, (list, tuple)):
        return [clean(v) for v in value]
    if isinstance(value, (np.integer, np.bool_)):
        return value.item()
    if isinstance(value, (float, np.floating)):
        return float(value) if np.isfinite(value) else (1e6 if value > 0 else -1e6 if value < 0 else None)
    return value


def append_ledger(path, row):
    with Path(path).open('a') as stream:
        fcntl.flock(stream, fcntl.LOCK_EX)
        stream.write(json.dumps(clean(row), allow_nan=False) + '\n')
        stream.flush()
        os.fsync(stream.fileno())


def evaluate_arrays(candidate, dataset, meta, side, horizon, split):
    names = meta['features']
    if list(dataset['feature_names']) != names or list(dataset['target_names']) != meta['targets']:
        raise ValueError('dataset schema mismatch')
    mask = dataset['split'] == split
    X, y, t = dataset['X'][mask], dataset['y'][mask], dataset['t_ms'][mask]
    if not len(t):
        raise ValueError('empty split')
    source = Path(candidate).read_bytes().decode('utf-8')
    scores, features = score_candidate(source, names, X)
    states = np.full(len(t), 'unknown', dtype='<U8')
    for feature, label in [('btc_bear', 'btc_bear'), ('btc_chop', 'chop'), ('btc_bull', 'btc_bull')]:
        if feature in names:
            states[X[:, names.index(feature)] == 1] = label
    targets = meta['targets']
    result = metrics(t, scores, y[:, targets.index(f'{side}_net_{horizon}')],
                     y[:, targets.index(f'{side}_exc_{horizon}')], states, len(features))
    result['candidate_sha256'] = hashlib.sha256(source.encode()).hexdigest()
    return result


def evaluate(candidate, side, horizon, split, work_dir=RESEARCH / 'work', ledger=None):
    work = Path(work_dir)
    ledger = Path(ledger) if ledger else work / 'ledger.jsonl'
    try:
        if split not in ('train', 'val'):
            raise ValueError('only train/val allowed')
        meta = read_json(work / 'meta.json')
        manifest = read_json(work / 'freeze.json', {})
        for name, digest in manifest.get('files', {}).items():
            if name != 'visible.npz' and sha256(work / name) != digest:
                raise ValueError(f'frozen file changed: {name}')
        for path, digest in manifest.get('dependencies', {}).items():
            if sha256(path) != digest:
                raise ValueError('frozen evaluator dependency changed')
        expected = meta['data_sha256']['visible']
        if sha256(work / 'visible.npz') != expected:
            raise ValueError('visible.npz sha256 mismatch')
        if split == 'train' and 'AR_ROUND_LEDGER_START' in os.environ:
            if valid_trials(ledger, side, horizon) - int(os.environ['AR_ROUND_LEDGER_START']) >= 8:
                raise ValueError('round train budget exhausted (8)')
        with np.load(work / 'visible.npz', allow_pickle=False) as dataset:
            if not set(dataset['split']).issubset({'train', 'val'}):
                raise ValueError('visible dataset contains forbidden split')
            result = evaluate_arrays(candidate, dataset, meta, side, horizon, split)
        if split == 'train':
            append_ledger(ledger, dict(ts=dt.datetime.now(dt.timezone.utc).isoformat(), side=side, horizon=horizon,
                                      split=split, candidate_sha256=result['candidate_sha256'], fitness=result['fitness'],
                                      ic_t=result['ic']['t'], qs=result['qs']['mean'], n_features=result['complexity']['n_features']))
            return clean(result)
        k = valid_trials(ledger, side, horizon)
        ic_t = result['ic']['t']
        bucket = '<0' if np.isnan(ic_t) or ic_t < 0 else '0-1' if ic_t < 1 else '1-2' if ic_t <= 2 else '>2'
        passed = result['ic']['mean'] > 0 and result['qs']['mean'] > 0 and result['win_uplift'] > 0 and ic_t > t_required(k)
        return dict(valid=True, **{'pass': bool(passed)}, val_ic_t_bucket=bucket)
    except Exception as exc:
        # No data-dependent metrics or tracebacks in val errors.
        return dict(valid=False, reason=f'{type(exc).__name__}: {str(exc)[:240]}')


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--candidate', type=Path, required=True)
    ap.add_argument('--side', choices=['long', 'short'], required=True)
    ap.add_argument('--horizon', choices=[12, 24, 72], type=int, required=True)
    ap.add_argument('--split', choices=['train', 'val'], required=True)
    ap.add_argument('--ledger', type=Path)
    ap.add_argument('--work-dir', type=Path, default=RESEARCH / 'work')
    args = ap.parse_args()
    result = evaluate(args.candidate, args.side, args.horizon, args.split, args.work_dir, args.ledger)
    print(json.dumps(result, ensure_ascii=False, allow_nan=False))
    return 0 if result['valid'] else 2


if __name__ == '__main__':
    raise SystemExit(main())
