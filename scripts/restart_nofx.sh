#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

BIN="$ROOT/nofx"
LOG="/tmp/nofx.log"

# Build first

go build -o "$BIN" .

# Match only the real backend binary process, not vite/esbuild or compiler args.
# Accept either absolute-path execution or ./nofx from the repo root.
old_pid="$(ps -eo pid=,args= | awk -v bin="$BIN" '
  $0 ~ ("^ *[0-9]+ " bin "( |$)") || $0 ~ ("^ *[0-9]+ \\./nofx( |$)") {print $1; exit}
')"
if [[ -n "${old_pid:-}" ]]; then
  echo "old:$old_pid"
  kill -9 "$old_pid"
else
  echo "old:none"
fi

nohup "$BIN" > "$LOG" 2>&1 &
new_pid=$!
echo "new:$new_pid"
sleep 2
ps -eo pid,lstart,args | awk -v bin="$BIN" '
  $0 ~ ("^ *[0-9]+ .*" bin "( |$)") || $0 ~ ("^ *[0-9]+ .*\\./nofx( |$)") {print}
'
