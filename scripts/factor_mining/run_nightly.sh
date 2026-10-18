#!/bin/sh
set -u
FM_SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
FM_REPO_DIR=$(CDPATH= cd -- "$FM_SCRIPT_DIR/../.." && pwd)
AR_PYTHON=${AR_PYTHON:-/Users/zhangyun/workspace/nofx/.venv/bin/python}
FM_ROOT=${FACTOR_MINING_DIR:-/Users/zhangyun/workspace/nofx/data/factor_mining}
mkdir -p "$FM_ROOT"
FM_LOG=$FM_ROOT/nightly.log
if [ -f "$FM_LOG" ] && [ "$(wc -c < "$FM_LOG")" -gt 2097152 ]; then
    : > "$FM_LOG"
fi
cd "$FM_REPO_DIR" || exit 1
PYTHONPATH="$FM_REPO_DIR${PYTHONPATH:+:$PYTHONPATH}"
PYTHONDONTWRITEBYTECODE=1
export PYTHONPATH PYTHONDONTWRITEBYTECODE
for FM_MARKET in crypto us; do
    (
        echo "=== $FM_MARKET $(date '+%Y-%m-%d %H:%M:%S') ==="
        "$AR_PYTHON" -m scripts.factor_mining.run_mining --market "$FM_MARKET" \
            --mutations "${FM_MUTATIONS:-300}" --llm "${FM_LLM:-20}" || \
            echo "$FM_MARKET exited non-zero"
    ) >> "$FM_LOG" 2>&1
done
