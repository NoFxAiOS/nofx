# Replay Fixtures 目录规范

本目录用于 replay / paper-trading / simulation 验证。

## 当前场景清单

| 文件 | 覆盖内容 |
|------|----------|
| `scenario-btc-long-protection-smoke.json` | BTC 做多 + protection 挂设 + regime filter |
| `scenario-btc-long-open-close-smoke.json` | BTC 做多开平仓 + realized PnL |
| `scenario-eth-short-open-close.json` | ETH 做空开平仓 + short 侧 PnL |
| `scenario-multi-step-progression.json` | 多步价格推进，先多后空，双向 PnL 累计 |
| `scenario-negative-pnl-long.json` | 做多亏损，负收益计算 |
| `scenario-open-with-protection.json` | 开仓持仓 + 保护单挂设验证 |
| `scenario-short-with-protection.json` | 做空持仓 + short 侧保护单验证 |
| `scenario-regime-trend-block.json` | 趋势不对齐 regime filter 阻断 |

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
    },
    {
      "type": "close_long",
      "quantity": 0,
      "price": 104
    }
  ],
  "protection": {
    "mode": "full",
    "take_profit_pct": 5,
    "stop_loss_pct": 2
  },
  "regime_filter": {
    "enabled": true,
    "allowed_regimes": ["standard", "trending"],
    "block_high_funding": true,
    "max_funding_rate_abs": 0.01,
    "require_trend_alignment": false
  },
  "expected": {
    "protection_orders": 2,
    "final_position_count": 0,
    "closed_pnl_count": 1,
    "realized_pnl": 4,
    "blocked": false
  }
}
```

## 支持的 action 类型
- `open_long` / `open_short`：开仓
- `close_long` / `close_short`：平仓
- 每个 action 可选 `price` 字段覆盖当前市场价

## 支持的 expected 校验字段
- `protection_orders`：最终挂单数
- `final_position_count`：最终持仓数
- `closed_pnl_count`：已平仓记录数
- `realized_pnl`：已实现收益总和
- `blocked`：是否被 regime filter 阻断
