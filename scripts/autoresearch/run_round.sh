#!/bin/sh
# A single pair per invocation. No GNU timeout dependency (macOS supported).
set -eu
AR_SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
AR_PYTHON=${AR_PYTHON:-/Users/zhangyun/workspace/nofx/.venv/bin/python}
exec "$AR_PYTHON" "$AR_SCRIPT_DIR/run_round.py" "$@"
