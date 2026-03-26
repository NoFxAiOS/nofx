# Replay Fixtures 目录规范

本目录用于后续 replay / paper-trading / simulation 验证。

## 建议结构
- `market/`：历史 K 线、funding、OI、价格切片
- `scenarios/`：单场景回放定义
- `expected/`：期望事件、期望保护动作、期望状态

## 单场景最小字段
```json
{
  "name": "btc-long-protection-smoke",
  "symbol": "BTCUSDT",
  "initial_price": 100,
  "prices": [100, 101, 103, 102, 105, 104],
  "actions": [
    {
      "type": "open_long",
      "quantity": 1,
      "leverage": 5
    }
  ],
  "expected": {
    "protection_orders": 2,
    "final_position_count": 1
  }
}
```

## 当前阶段目标
- 先统一格式
- 再接 paper trader
- 最后再做完整 replay runner
