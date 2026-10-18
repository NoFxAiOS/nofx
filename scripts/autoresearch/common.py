"""Shared paths and durable writes; no network, database or process discovery."""
import hashlib
import json
import os
from pathlib import Path
import tempfile

DATA = Path('/Users/zhangyun/workspace/nofx/data')
ALT = DATA / 'alt_data'
RESEARCH = DATA / 'autoresearch'
EXCLUDED = {'APRUSDT', 'MANAUSDT', 'STXUSDT', 'USELESSUSDT'}


def read_json(path, default=None):
    try:
        return json.loads(Path(path).read_text(encoding='utf-8'))
    except FileNotFoundError:
        return default


def atomic_json(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, tmp = tempfile.mkstemp(prefix=path.name + '.', suffix='.tmp', dir=path.parent)
    try:
        with os.fdopen(fd, 'w', encoding='utf-8') as stream:
            json.dump(value, stream, ensure_ascii=False, allow_nan=False)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(tmp, path)
    finally:
        if os.path.exists(tmp):
            os.unlink(tmp)


def sha256(path):
    h = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def universe(path):
    names = read_json(path)['crypto']
    if not isinstance(names, list) or not all(isinstance(s, str) and s.isalnum() for s in names):
        raise ValueError('crypto must be a list of alphanumeric symbols')
    return [s for s in dict.fromkeys(names) if s not in EXCLUDED]
