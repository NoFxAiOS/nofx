# Protection Delivery Discipline

## Mandatory execution rule
For any protection-system change that affects runtime behavior, the work is NOT considered delivered until all of the following are done by the agent itself:

1. code change completed
2. relevant tests/build completed
3. service/backend reloaded or restarted if required
4. health check passes after reload
5. runtime log evidence is collected from the new process
6. when the change targets exchange-native protection behavior, delivery requires real exchange/open-order evidence whenever a live position exists

## What is not acceptable
- stopping after code edits without reload
- claiming success from unit tests alone for runtime protection behavior
- relying on stale logs from an older process
- treating local fallback as success when native exchange support exists and is expected to be primary

## Current protection goal
For Binance / Bitget / OKX:
- exchange-native protection orders are the primary target
- local monitoring is secondary/fallback only
- reconciler must continuously verify and repair target exchange protection orders
