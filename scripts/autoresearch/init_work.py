"""Initialize/resume a work directory without resetting trials or existing candidates."""
import argparse
import json
from pathlib import Path
import shutil
import sys

if __package__ in (None, ''):
    sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
from scripts.autoresearch.common import RESEARCH, atomic_json, sha256

SOURCE = Path(__file__).resolve().parent
COMBINATIONS = [('short', 24), ('long', 24), ('short', 72), ('long', 72)]


def initialize(dataset_work=RESEARCH / 'work', work_dir=None):
    source, work = Path(dataset_work).resolve(), Path(work_dir or dataset_work).resolve()
    # Validate before mutating anything. No private/holdout paths accessed here.
    meta = json.loads((source / 'meta.json').read_text())
    if sha256(source / 'visible.npz') != meta['data_sha256']['visible']:
        raise ValueError('visible dataset hash mismatch')
    work.mkdir(parents=True, exist_ok=True)
    freeze = work / 'freeze.json'
    if freeze.exists():
        manifest = json.loads(freeze.read_text())
        for name, digest in manifest['files'].items():
            if sha256(work / name) != digest:
                raise ValueError(f'frozen file changed: {name}; use a new work directory')
        return work
    for src, name in [(source / 'meta.json', 'meta.json'), (source / 'visible.npz', 'visible.npz'),
                      (SOURCE / 'evaluate.py', 'evaluate.py'), (SOURCE / 'program.md', 'program.md')]:
        dest = work / name
        if src.resolve() != dest.resolve():
            shutil.copyfile(src, dest)
        dest.chmod(dest.stat().st_mode & ~0o222)
    for directory in ('best', 'promoted'):
        (work / directory).mkdir(exist_ok=True)
    for side, horizon in COMBINATIONS:
        name = f'candidate_{side}_{horizon}.py'
        path = work / name
        if not path.exists():
            sign = '-' if side == 'short' else '+'
            path.write_text('"""BASELINE / 基线：24h 动量，未经验证。"""\nimport numpy as np\n'
                            'FEATURES = ["ret_24h_xs"]\ndef score(X):\n'
                            f'    return {sign}X["ret_24h_xs"]\n', encoding='utf-8')
        if not (work / 'best' / name).exists():
            shutil.copyfile(path, work / 'best' / name)
    (work / 'ledger.jsonl').touch(exist_ok=True)
    (work / 'notes.md').touch(exist_ok=True)
    dependencies = [SOURCE / 'common.py', SOURCE / 'evaluate.py', SOURCE / 'run_round.py', SOURCE / 'init_work.py',
                    SOURCE.parent / 'backtest/short_features_study.py', SOURCE.parent / 'backtest/breakdown_study.py']
    atomic_json(freeze, dict(files={name: sha256(work / name) for name in ('evaluate.py', 'meta.json', 'visible.npz', 'program.md')},
                            dependencies={str(p): sha256(p) for p in dependencies}))
    freeze.chmod(0o444)
    return work


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--dataset-work', type=Path, default=RESEARCH / 'work')
    ap.add_argument('--work-dir', type=Path, help='default: same as --dataset-work')
    args = ap.parse_args()
    print(initialize(args.dataset_work, args.work_dir))


if __name__ == '__main__':
    main()
