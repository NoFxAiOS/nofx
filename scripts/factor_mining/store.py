"""Atomic, finite JSON storage and a process-wide nonblocking advisory lock."""
import datetime as dt
import fcntl
import json
import math
import os
from pathlib import Path
import tempfile

import numpy as np

from .data_fetch import ROOT


class AlreadyRunning(RuntimeError):
    pass


class FileLock:
    def __init__(self, root=ROOT):
        self.path = Path(root) / '.lock'
        self.stream = None

    def __enter__(self):
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.stream = self.path.open('a')
        try:
            fcntl.flock(self.stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as exc:
            self.stream.close()
            self.stream = None
            raise AlreadyRunning(f'factor mining already running: {self.path}') from exc
        return self

    def __exit__(self, *exc):
        if self.stream is not None:
            fcntl.flock(self.stream, fcntl.LOCK_UN)
            self.stream.close()
            self.stream = None


def timestamp():
    return dt.datetime.now(dt.timezone.utc).isoformat(timespec='seconds')


def finite(value):
    if isinstance(value, dict):
        return {str(k): finite(v) for k, v in value.items()}
    if isinstance(value, (list, tuple)):
        return [finite(v) for v in value]
    if isinstance(value, np.generic):
        value = value.item()
    if isinstance(value, float) and not math.isfinite(value):
        return None
    return value


def read_json(path, default=None):
    try:
        return json.loads(Path(path).read_text(encoding='utf-8'))
    except FileNotFoundError:
        return default


def _replace(path, write):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, name = tempfile.mkstemp(prefix='.' + path.name + '.', dir=path.parent)
    try:
        with os.fdopen(fd, 'w', encoding='utf-8') as stream:
            write(stream)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(name, path)
    finally:
        if os.path.exists(name):
            os.unlink(name)


def write_json(path, obj):
    _replace(path, lambda f: json.dump(finite(obj), f, allow_nan=False, ensure_ascii=False,
                                      separators=(',', ':')))


def read_jsonl(path):
    path = Path(path)
    if not path.exists():
        return []
    return [json.loads(line) for line in path.read_text(encoding='utf-8').splitlines() if line.strip()]


def append_jsonl(path, rows):
    path = Path(path)
    rows = list(rows)
    def write(stream):
        if path.exists():
            with path.open(encoding='utf-8') as old:
                for line in old:
                    stream.write(line)
        for row in rows:
            stream.write(json.dumps(finite(row), allow_nan=False, ensure_ascii=False,
                                    separators=(',', ':')) + '\n')
    _replace(path, write)


def read_ledger(market, root=ROOT):
    return read_jsonl(Path(root) / market / 'ledger.jsonl')


def append_ledger(market, rows, root=ROOT):
    append_jsonl(Path(root) / market / 'ledger.jsonl', rows)


def write_zoo(market, zoo, root=ROOT):
    write_json(Path(root) / market / 'zoo.json', zoo)


def write_ensemble(market, ensemble, root=ROOT):
    write_json(Path(root) / market / 'ensemble.json', ensemble)


def write_status(patch, root=ROOT):
    path = Path(root) / 'status.json'
    value = read_json(path, dict(updated_at=None, running=False, current=None,
                                 schedule='每天 02:30', markets={}))
    def merge(base, updates):
        for key, item in updates.items():
            if isinstance(item, dict) and isinstance(base.get(key), dict):
                merge(base[key], item)
            else:
                base[key] = item
    merge(value, patch)
    value['updated_at'] = timestamp()
    write_json(path, value)


def append_run(row, root=ROOT):
    append_jsonl(Path(root) / 'runs.jsonl', [row])
