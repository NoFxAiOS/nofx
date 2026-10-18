#!/bin/sh
# Sync scripts/factor_mining into the stable runtime dir used by launchd (same dir as autoresearch).
# Safe to run any time: nothing here is hash-frozen (the evaluator's sha256 is only recorded per run).
set -eu
SRC=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
DST=${AR_RUNTIME:-/Users/zhangyun/workspace/nofx-autoresearch}
mkdir -p "$DST/scripts"
touch "$DST/scripts/__init__.py"
rsync -a --delete --exclude '__pycache__' --exclude '*.pyc' --exclude 'tests' "$SRC/scripts/factor_mining/" "$DST/scripts/factor_mining/"
echo "synced to $DST/scripts/factor_mining"
