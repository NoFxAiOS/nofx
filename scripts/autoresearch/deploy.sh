#!/bin/sh
# Sync scripts/backtest + scripts/autoresearch into the stable runtime dir used by launchd.
# Do NOT run while a round is in progress or after init_work (freeze.json hashes the source files):
# a changed frozen dependency makes run_round refuse until the work dir is rebuilt on purpose.
set -eu
SRC=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
DST=${AR_RUNTIME:-/Users/zhangyun/workspace/nofx-autoresearch}
mkdir -p "$DST/scripts"
touch "$DST/scripts/__init__.py"
for d in backtest autoresearch; do
  rsync -a --delete --exclude '__pycache__' --exclude '*.pyc' "$SRC/scripts/$d/" "$DST/scripts/$d/"
done
echo "synced to $DST"
