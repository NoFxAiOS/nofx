"""Offline seed/mutation/LLM search supervised by a frozen evaluator.

The persisted split envelope uses the shortest horizon; every horizon has its
own h+2 purged masks internally (the file schema has one shared date envelope).
"""
import ast
from collections import Counter, OrderedDict
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import random
import re
import signal
import subprocess
import time
from typing import TypedDict

from . import evaluate, zoo
from .config import CORR_REDUNDANT, MARKETS
from .data_fetch import ROOT
from .ensemble import build_ensemble
from .expr import OPERATORS, canonical, dsl_help, evaluate_expr, expr_hash, validate_expr
from .panel import forward_target, load_panels, make_splits
from . import store


class Candidate(TypedDict):
    expr: str
    source: str
    family: str


def family(expr):
    if re.search(r'\b(Corr|Cov)\(', expr):
        return 'price_volume'
    if '$volume' in expr or re.search(r'\bRank\(', expr):
        return 'volume'
    if re.search(r'\b(Std|Skew|Kurt|Mad)\(', expr):
        return 'volatility'
    if re.search(r'\b(Delta|Ref)\(\$close', expr):
        return 'momentum'
    return 'other'


def mutate(expr, rng, pool) -> list[str]:
    """Make validated local AST edits, wrappers, and pool combinations."""
    original = canonical(expr)
    tree = ast.parse(re.sub(r'\$([a-z]+)', r'FM_\1', expr), mode='eval')
    variants = []
    windows = (3, 5, 7, 10, 14, 20, 30, 40, 60, 90, 120)
    groups = (('Mean', 'EMA', 'WMA', 'Median'), ('Std', 'Mad'), ('Max', 'Min'),
              ('Corr', 'Cov'), ('Skew', 'Kurt'))
    def emit():
        variants.append(re.sub(r'\bFM_([a-z]+)\b', r'$\1', ast.unparse(tree)))
    for node in ast.walk(tree):
        if isinstance(node, ast.Call):
            name = node.func.id
            if OPERATORS[name]['window_arg']:
                pos = 2 if name in ('Corr', 'Cov') else 1
                old = node.args[pos]
                near = sorted(windows, key=lambda w: (abs(w-old.value), w))
                for window in [w for w in near if w != old.value][:2]:
                    node.args[pos] = ast.Constant(window)
                    emit()
                node.args[pos] = old
            for group in groups:
                if name in group:
                    choices = [op for op in group if op != name]
                    rng.shuffle(choices)
                    for op in choices:
                        node.func.id = op
                        emit()
                    node.func.id = name
        elif isinstance(node, ast.Name) and node.id in ('FM_close', 'FM_vwap', 'FM_open'):
            old = node.id
            for field in ('FM_close', 'FM_vwap', 'FM_open'):
                if field != old:
                    node.id = field
                    emit()
            node.id = old
    window = rng.choice(windows)
    variants.extend((f'CSRank({expr})', f'Mean({expr},{window})', f'Delta({expr},{window})'))
    if pool:
        other = rng.choice(pool)
        other = other['expr'] if isinstance(other, dict) else other
        variants.extend(f'CSRank({expr}){op}CSRank({other})' for op in ('+', '-', '*'))
    rng.shuffle(variants)
    valid, seen = [], {original}
    for variant in variants:
        try:
            key = canonical(variant)
        except ValueError:
            continue
        if key not in seen:
            valid.append(key)
            seen.add(key)
    return valid


def model_call(workdir, prompt_path, out_path):
    command = [str(Path.home() / '.local/bin/opencode-task'), str(workdir), str(prompt_path),
               'zhipuai-coding-plan/glm-5.3-flash', str(out_path)]
    process = subprocess.Popen(command, cwd=workdir, stdout=subprocess.PIPE,
                               stderr=subprocess.STDOUT, start_new_session=True)
    captured = b''
    try:
        captured, _ = process.communicate(timeout=15 * 60)
        if process.returncode:
            raise RuntimeError(f'opencode-task exited {process.returncode}')
    finally:
        # Kill the complete process group, including grandchildren after early exit.
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        process.wait()
        if not out_path.exists():
            out_path.write_bytes(captured)


def llm_propose(market, pool_text, failed_text, n, runner, workdir):
    stats = dict(llm_proposed=0, llm_valid=0, llm_invalid=0, llm_error=None)
    workdir = Path(workdir).resolve()
    workdir.mkdir(parents=True, exist_ok=True)
    prompt_path, out_path = workdir / 'prompt.md', workdir / 'out.md'
    description = ('虚拟币日线，点时刻成交额前60，持有1/3/7日。' if market == 'crypto'
                   else '美股日线，SPY基准，持有5/10/20个交易日。')
    prompt = (f'你是量化因子研究员。{description}\n{dsl_help()}\n'
              f'训练集 top-10（仅 train 指标）：\n{pool_text}\n'
              f'最近10个失败表达式：\n{failed_text}\n'
              f'请提出最多{min(max(n, 0), 20)}条新表达式。只输出一个JSON数组 ["expr",...]，'
              '禁止解释，禁止输出代码，禁止读写其它文件。\n')
    prompt_path.write_text(prompt, encoding='utf-8')
    # A repeated, fixed run_id must not accidentally parse an earlier response.
    out_path.unlink(missing_ok=True)
    try:
        (runner or model_call)(workdir, prompt_path, out_path)
        text = out_path.read_text(encoding='utf-8')
        decoder, arrays = json.JSONDecoder(), []
        for match in re.finditer(r'\[', text):
            try:
                value, end = decoder.raw_decode(text, match.start())
                if isinstance(value, list):
                    arrays.append((end, -match.start(), value))
            except ValueError:
                continue
        if not arrays:
            raise ValueError('no JSON array in LLM output')
        proposals = max(arrays, key=lambda x: x[:2])[2][:min(max(n, 0), 20)]
        stats['llm_proposed'] = len(proposals)
        valid = []
        for expr in proposals:
            try:
                validate_expr(expr)
                valid.append(canonical(expr))
                stats['llm_valid'] += 1
            except (TypeError, ValueError):
                stats['llm_invalid'] += 1
        return valid, stats
    except Exception as exc:
        stats['llm_error'] = f'{type(exc).__name__}: {exc}'
        return [], stats


def frozen_hashes():
    return {'evaluator_sha256': hashlib.sha256(Path(evaluate.__file__).read_bytes()).hexdigest(),
            'expr_sha256': hashlib.sha256(Path(__file__).with_name('expr.py').read_bytes()).hexdigest()}


class Miner:
    def __init__(self, root=ROOT):
        self.root = Path(root)
        self.fetch_error = None
        self.fetch_seconds = 0.

    def run_market(self, market, *, n_mutations, n_llm, budget_s, seed, use_llm, runner=None, now=None):
        started = now() if callable(now) else now
        started = started or dt.datetime.now(dt.timezone.utc)
        run_id = f'{market}-{started.strftime("%Y%m%dT%H%M%S%fZ")}-{seed}'
        stamp = started.isoformat()
        clock = time.monotonic()
        hashes = frozen_hashes()
        row = dict(run_id=run_id, market=market, started_at=stamp, finished_at=None,
                   seconds=0., seed_evaluated=0, mutation_evaluated=0, llm_proposed=0,
                   llm_valid=0, llm_invalid=0, llm_evaluated=0, invalid=0, new_accepted=0,
                   new_candidate=0, trials_total=0, error=self.fetch_error,
                   seed=seed, budget_exhausted=False, **hashes)
        def stage(name):
            store.write_status(dict(running=True, current=dict(market=market, stage=name, started_at=stamp),
                                    **hashes), self.root)
        try:
            stage('seed')
            cfg = MARKETS[market]
            panels = load_panels(market, self.root)
            store.write_status({'markets': {market: panels.meta}}, self.root)
            splits = {h: make_splits(panels.mask.index, h, panels.split_bounds) for h in cfg['horizons']}
            targets = {h: forward_target(panels, h) for h in cfg['horizons']}
            ledger = store.read_ledger(market, self.root)
            seen = {(r['horizon'], r['id']) for r in ledger}
            K = Counter(r['horizon'] for r in ledger)
            row['trials_total'] = sum(K.values())
            old_zoo = store.read_json(self.root / market / 'zoo.json', {'factors': []})
            factors = {(f['horizon'], f['id']): dict(f) for f in old_zoo['factors']}
            old_status = {key: f['status'] for key, f in factors.items()}
            seeds = [dict(expr=s['expr'], source='seed', family=s['family']) for s in zoo.seed_factors()]
            rng = random.Random(seed)
            pool = []
            for h in cfg['horizons']:
                pool.extend(sorted((f for f in factors.values() if f['horizon'] == h),
                                   key=lambda f: (-f['train']['t'], f['id']))[:20])
            pool.extend(rng.sample(seeds, min(20, len(seeds))))
            candidates, candidate_ids = [], set()
            def add(expr, source, category=None):
                try:
                    key = expr_hash(expr)
                except ValueError:
                    row['invalid'] += 1
                    return False
                if key in candidate_ids or all((h, key) in seen for h in cfg['horizons']):
                    return False
                candidate_ids.add(key)
                candidates.append(dict(expr=expr, source=source, family=category or family(expr)))
                return True
            for s in seeds:
                add(s['expr'], 'seed', s['family'])
            stage('mutation')
            made = 0
            for _ in range(max(0, n_mutations) * 10):
                if made >= n_mutations or not pool:
                    break
                for expr in mutate(rng.choice(pool)['expr'], rng, pool):
                    made += add(expr, 'mutation')
                    if made >= n_mutations:
                        break
            if use_llm and n_llm > 0:
                stage('llm')
                top = sorted(factors.values(), key=lambda f: (-f['train']['t'], f['id']))[:10]
                # Construct the model view explicitly; never serialize zoo/ledger rows.
                pool_text = json.dumps([dict(expr=f['expr'], horizon=f['horizon'], train_t=f['train']['t'])
                                        for f in top], ensure_ascii=False)
                failures = [r['expr'] for r in ledger if r['status'] == 'rejected'][-10:]
                exprs, stats = llm_propose(market, pool_text, json.dumps(failures), n_llm, runner,
                                           self.root / 'llm_work' / run_id)
                row.update(stats)
                for expr in exprs:
                    add(expr, 'llm')
            store.append_ledger(market, [], self.root)
            for i, candidate in enumerate(candidates):
                if i % 20 == 0 and time.monotonic() - clock >= budget_s:
                    row['budget_exhausted'] = True
                    break
                if i == 0 or candidate['source'] != candidates[i-1]['source']:
                    stage(candidate['source'])
                try:
                    values = evaluate_expr(candidate['expr'], panels.fields)
                except Exception:
                    row['invalid'] += 1
                    continue
                key = expr_hash(candidate['expr'])
                valid_candidate = False
                for h in cfg['horizons']:
                    if (h, key) in seen:
                        continue
                    metrics = evaluate.evaluate_factor(values, *targets[h], panels.mask, splits[h], h, cfg)
                    if metrics is None:
                        row['invalid'] += 1
                        continue
                    K[h] += 1
                    seen.add((h, key))
                    state = evaluate.classify(metrics['train'], metrics['val'], K[h], evaluate.t_required(K[h]))
                    factors[h, key] = dict(id=key, **candidate, horizon=h, first_seen=started.date().isoformat(),
                                           run_id=run_id, status=state, corr_max=0., corr_with=None, **metrics)
                    trial = dict(ts=stamp, run_id=run_id, id=key, expr=candidate['expr'],
                                       source=candidate['source'], horizon=h, train_t=metrics['train']['t'],
                                       train_ic=metrics['train']['ic'], status=state)
                    # Persist each valid trial before another horizon can fail.
                    store.append_ledger(market, [trial], self.root)
                    ledger.append(trial)
                    row['trials_total'] = sum(K.values())
                    if not valid_candidate:
                        row[candidate['source'] + '_evaluated'] += 1
                    valid_candidate = True
            row['trials_total'] = sum(K.values())
            self._classify_and_correlate(factors, K, panels, splits, cfg)
            for key, factor in factors.items():
                previous = old_status.get(key)
                if previous and previous != factor['status']:
                    history = list(factor.get('status_history', []))
                    history.append(dict(ts=stamp, run_id=run_id, **{'from': previous, 'to': factor['status']}))
                    factor['status_history'] = history[-5:]
                if previous != factor['status'] and factor['status'] in ('accepted', 'candidate'):
                    row['new_' + factor['status']] += 1
            horizons = {}
            for h in cfg['horizons']:
                counts = Counter(f['status'] for f in factors.values() if f['horizon'] == h)
                # Pruned rejected rows still count toward both trials and rejections.
                counts['rejected'] += K[h] - sum(counts.values())
                horizons[str(h)] = dict(trials=K[h], t_req=evaluate.t_required(K[h]),
                                        **{s: counts[s] for s in ('accepted', 'candidate', 'rejected', 'redundant')})
            priority = {'accepted': 0, 'candidate': 1, 'redundant': 2, 'rejected': 3}
            ordered = sorted(factors.values(), key=lambda f: (priority[f['status']], -f['train']['t'], f['id'], f['horizon']))
            rejected = 0
            retained = []
            for f in ordered:
                if f['status'] == 'rejected':
                    rejected += 1
                    if rejected > 200:
                        continue
                retained.append(f)
            shortest = min(cfg['horizons'])
            split = {s: splits[shortest]['ranges'][s] for s in ('train', 'val')}
            split['holdout_sealed'] = True
            output = dict(market=market, generated_at=stamp, split=split, horizons=horizons, factors=retained)
            # Preserve measured train/val metrics even if ensemble construction fails.
            store.write_zoo(market, output, self.root)
            stage('ensemble')
            ensemble = build_ensemble(market, retained, panels, targets, splits, cfg)
            store.write_ensemble(market, ensemble, self.root)
            return output
        except BaseException as exc:
            error = f'{type(exc).__name__}: {exc}'
            row['error'] = '; '.join(filter(None, (row['error'], error)))
            raise
        finally:
            row['finished_at'] = store.timestamp()
            row['seconds'] = round(time.monotonic() - clock + self.fetch_seconds, 6)
            try:
                store.write_status(dict(running=False, current=None, **hashes), self.root)
            finally:
                store.append_run(row, self.root)

    @staticmethod
    def _classify_and_correlate(factors, K, panels, splits, cfg):
        cache = OrderedDict()
        def ranks(f, dates):
            key = (f['id'], f['horizon'], f['direction'])
            if key not in cache:
                values = evaluate_expr(f['expr'], panels.fields).loc[dates] * f['direction']
                cache[key] = values.where(panels.mask.loc[dates]).rank(axis=1, method='average')
                if len(cache) > 60:
                    cache.popitem(last=False)
            cache.move_to_end(key)
            return cache[key]
        for h in cfg['horizons']:
            visible = splits[h]['train'] | splits[h]['val']
            dates = panels.mask.index[visible][::5]
            kept = []
            current = sorted((f for f in factors.values() if f['horizon'] == h),
                             key=lambda f: (-f['train']['t'], f['first_seen'], f['run_id'], f['id']))
            for f in current:
                f['status'] = evaluate.classify(f['train'], f['val'], K[h], evaluate.t_required(K[h]))
                f['corr_max'], f['corr_with'] = 0., None
                if f['status'] not in ('accepted', 'candidate'):
                    continue
                rank = ranks(f, dates)
                for other in kept:
                    corr = evaluate.cross_corr(rank, ranks(other, dates), dates)
                    if corr > f['corr_max']:
                        f['corr_max'], f['corr_with'] = corr, other['id']
                if f['corr_max'] >= CORR_REDUNDANT:
                    f['status'] = 'redundant'
                elif f['status'] == 'accepted':
                    kept.append(f)
