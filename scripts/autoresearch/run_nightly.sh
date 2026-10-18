#!/bin/sh
# Nightly batch: N rounds back to back (default 4 = one pass over short24/long24/short72/long72).
# A failed round is already rolled back and logged by run_round.py; keep going with the next one.
set -u
AR_SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROUNDS=${AR_ROUNDS:-4}
i=1
while [ "$i" -le "$ROUNDS" ]; do
  echo "=== round $i/$ROUNDS $(date '+%Y-%m-%d %H:%M:%S') ==="
  "$AR_SCRIPT_DIR/run_round.sh" || echo "round $i exited non-zero"
  i=$((i + 1))
done
echo "=== done $(date '+%Y-%m-%d %H:%M:%S') ==="
