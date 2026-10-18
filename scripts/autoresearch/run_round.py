"""Trusted round supervisor; model process gets a 40min process-group deadline.
The model is never invoked by tests with AR_DRY_RUN=1. No holdout path is read.
Ledger contains valid train rows AND type=round completion markers for rotation.
"""
import argparse
import datetime as dt
import fcntl
import json
import logging
from logging.handlers import RotatingFileHandler
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tempfile

if __package__ in (None, ''):
    sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
from scripts.autoresearch.common import RESEARCH, read_json, sha256
from scripts.autoresearch.evaluate import append_ledger, evaluate, ledger_rows, valid_trials
from scripts.autoresearch.init_work import COMBINATIONS


def verify(work):
    manifest = read_json(work / 'freeze.json')
    for name, digest in manifest['files'].items():
        if sha256(work / name) != digest:
            raise ValueError(f'frozen file changed: {name}')
    for path, digest in manifest['dependencies'].items():
        if sha256(path) != digest:
            raise ValueError(f'frozen dependency changed: {path}')


def model_call(work, prompt, log, trial_start):
    env = dict(os.environ, AR_ROUND_LEDGER_START=str(trial_start))
    env['PATH'] = str(Path(sys.executable).parent) + os.pathsep + env.get('PATH', '')
    command = [str(Path.home() / '.local/bin/opencode-task'), str(work), str(prompt),
               'zhipuai-coding-plan/glm-5.3-flash', str(work / 'model_transcript.log')]
    with log.open('a') as stream:
        process = subprocess.Popen(command, cwd=work, env=env, stdout=stream, stderr=subprocess.STDOUT, start_new_session=True)
        try:
            code = process.wait(timeout=40 * 60)
            if code:
                raise RuntimeError(f'opencode-task exited {code}')
        finally:
            # Also stop remaining grandchildren after an early parent exit.
            try:
                os.killpg(process.pid, signal.SIGTERM)
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    pass
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                process.wait()
            except ProcessLookupError:
                pass


def run(work_dir=RESEARCH / 'work'):
    work = Path(work_dir).resolve()
    # Nonblocking lock prevents cron overlap without accumulating waiting jobs.
    with (work / '.round.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        verify(work)
        return _run_locked(work)


def _run_locked(work):
    ledger = work / 'ledger.jsonl'
    history = ledger_rows(ledger)
    rounds = sum(row.get('type') == 'round' for row in history)
    side, horizon = COMBINATIONS[rounds % len(COMBINATIONS)]
    candidate = work / f'candidate_{side}_{horizon}.py'
    best = work / 'best' / candidate.name
    started = dt.datetime.now(dt.timezone.utc)
    old, val, new, error, promoted_path = None, {}, None, None, None
    before = valid_trials(ledger, side, horizon)
    # Transaction snapshots are outside the model cwd. Always restore on failure.
    # Same-user shell instructions are an operational boundary, not a sandbox.
    handler = RotatingFileHandler(work / 'round.log', maxBytes=1024 * 1024, backupCount=3)
    logger = logging.getLogger('autoresearch.round')
    logger.setLevel(logging.INFO)
    logger.addHandler(handler)
    with tempfile.TemporaryDirectory(prefix='round-rollback-', dir=work.parent) as tmp:
        backup = Path(tmp)
        # Mutable candidates + small frozen files. Data files are immutable and
        # hash-checked; never duplicate the large dataset on every round.
        protected = [p for p in work.iterdir() if p.is_file() and
                     (p.name.startswith('candidate_') or p.name in ('evaluate.py', 'meta.json', 'program.md', 'freeze.json'))]
        for path in protected:
            shutil.copy2(path, backup / path.name)
        shutil.copyfile(candidate, best)
        try:
            old = evaluate(candidate, side, horizon, 'train', work, ledger)
            if not old['valid']:
                raise ValueError('baseline invalid: ' + old['reason'])
            baseline_ledger = ledger.read_bytes()
            summary = [{k: row[k] for k in ('fitness', 'pass') if k in row} for row in history[-10:]]
            text = (work / 'program.md').read_text() + '\n\n本轮组合：' + side + '/' + str(horizon)
            text += '\n工作目录：' + str(work) + '\n上轮摘要（仅 fitness / pass）：' + json.dumps(summary)
            prompt = work / '.round_prompt.md'
            prompt.write_text(text, encoding='utf-8')
            trial_start = valid_trials(ledger, side, horizon)
            if os.environ.get('AR_DRY_RUN') != '1':
                model_call(work, prompt, work / 'round.log', trial_start)
            verify(work)
            if not ledger.read_bytes().startswith(baseline_ledger):
                raise ValueError('ledger history modified')
            for path in protected:
                if path.name.startswith('candidate_') and path != candidate and sha256(path) != sha256(backup / path.name):
                    raise ValueError('model changed a different combination')
            if sha256(best) != sha256(backup / candidate.name):
                raise ValueError('model changed best backup')
            # Supervisor uses the original evaluator, not an import from model cwd.
            new = evaluate(candidate, side, horizon, 'train', work, ledger)
            val = evaluate(candidate, side, horizon, 'val', work, ledger)
            if not new['valid'] or not val['valid']:
                raise ValueError('candidate invalid: ' + str(new.get('reason', val.get('reason'))))
            proposal = candidate.read_bytes()
            if new['fitness'] <= old['fitness']:
                shutil.copyfile(backup / candidate.name, candidate)
            # Pass refers to the evaluated proposal, including a train-rejected
            # proposal. Save exactly those evaluated bytes, never a restored file.
            if val['pass']:
                name = f'{side}_{horizon}_{started.strftime("%Y%m%dT%H%M%S%fZ")}.py'
                promoted_path = work / 'promoted' / name
                promoted_path.write_bytes(proposal)
                append_ledger(work / 'promoted.log', dict(ts=started.isoformat(), side=side, horizon=horizon,
                              candidate_sha256=new['candidate_sha256'], file=name, **{'pass': True}))
        except BaseException as exc:
            error = f'{type(exc).__name__}: {exc}'
            logger.exception('round %s/%s failed', side, horizon)
            for path in protected:
                if path.is_symlink():
                    path.unlink()
                elif path.exists():
                    path.chmod(0o644)
                shutil.copy2(backup / path.name, path)
            shutil.copyfile(backup / candidate.name, best)
            if promoted_path is not None and promoted_path.exists():
                promoted_path.unlink()
            # Keep all evaluator-written train attempts for honest K accounting.
        finally:
            if (work / '.round_prompt.md').exists():
                (work / '.round_prompt.md').unlink()
            count = valid_trials(ledger, side, horizon) - before
            row = dict(type='round', ts=started.isoformat(), side=side, horizon=horizon, split='round',
                       trials=count, fitness=new.get('fitness') if new else None,
                       previous_fitness=old.get('fitness') if old else None, **{'pass': val.get('pass', False)},
                       bucket=val.get('val_ic_t_bucket'), error=error)
            append_ledger(ledger, row)
            with (work / 'round_log.md').open('a') as stream:
                stream.write(f'- {row["ts"]} {side}/{horizon}: trials={count}, fitness={row["previous_fitness"]} -> '
                             f'{row["fitness"]}, val_pass={row["pass"]}, bucket={row["bucket"]}, error={error}\n')
            logger.info('%s', json.dumps(row, ensure_ascii=False))
            logger.removeHandler(handler)
            handler.close()
    return row


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--work-dir', type=Path, default=RESEARCH / 'work')
    args = ap.parse_args()
    row = run(args.work_dir)
    print(json.dumps(row, ensure_ascii=False))
    return 1 if row['error'] else 0


if __name__ == '__main__':
    raise SystemExit(main())
