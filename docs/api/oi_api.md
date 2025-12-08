# OI 持仓数据接口文档

## 接口概述

该接口提供币安交易所的合约持仓量（Open Interest）排行数据，支持查询持仓增加和减少排行榜。

## 接口列表

| 接口 | 说明 |
|-----|------|
| `/api/oi/top` | 持仓增加排行 Top20（固定参数，向后兼容） |
| `/api/oi/top-ranking` | 持仓增加排行（支持自定义参数） |
| `/api/oi/low-ranking` | 持仓减少排行（支持自定义参数） |

---

## 1. 持仓增加排行 Top20

### 请求

```
GET /api/oi/top
```

### 完整示例

```
http://nofxaios.com:30006/api/oi/top?auth=cm_568c67eae410d912c54c
```

### 参数

| 参数 | 类型 | 必填 | 说明 |
|-----|------|-----|------|
| auth | string | 是 | 认证密钥 |

### 说明

固定返回 1 小时内持仓价值增加最多的前 20 个币种，向后兼容接口。

---

## 2. 持仓增加排行（自定义参数）

### 请求

```
GET /api/oi/top-ranking
```

### 完整示例

```
http://nofxaios.com:30006/api/oi/top-ranking?limit=50&duration=4h&auth=cm_568c67eae410d912c54c
```

### 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|-----|------|-----|-------|------|
| limit | int | 否 | 20 | 获取数量，范围 1-100 |
| duration | string | 否 | 1h | 时间范围 |
| auth | string | 是 | - | 认证密钥 |

---

## 3. 持仓减少排行

### 请求

```
GET /api/oi/low-ranking
```

### 完整示例

```
http://nofxaios.com:30006/api/oi/low-ranking?limit=30&duration=24h&auth=cm_568c67eae410d912c54c
```

### 参数

同持仓增加排行接口。

---

## duration 时间范围参数

| 值 | 说明 |
|---|------|
| 1m | 1 分钟 |
| 5m | 5 分钟 |
| 15m | 15 分钟 |
| 30m | 30 分钟 |
| 1h | 1 小时（默认） |
| 4h | 4 小时 |
| 8h | 8 小时 |
| 12h | 12 小时 |
| 24h | 24 小时 |
| 1d | 1 天（同 24h） |
| 2d | 2 天 |
| 3d | 3 天 |

---

## 返回数据

### 响应示例

```json
{
  "code": 0,
  "data": {
    "count": 20,
    "exchange": "binance",
    "time_range": "4小时",
    "time_range_param": "4h",
    "rank_type": "top",
    "limit": 20,
    "positions": [
      {
        "rank": 1,
        "symbol": "BTCUSDT",
        "oi_delta": 1500.5,
        "oi_delta_value": 145500000,
        "oi_delta_percent": 3.52,
        "current_oi": 44000,
        "price_delta_percent": 2.15,
        "net_long": 26000,
        "net_short": 18000
      },
      {
        "rank": 2,
        "symbol": "ETHUSDT",
        "oi_delta": 25000,
        "oi_delta_value": 87500000,
        "oi_delta_percent": 2.85,
        "current_oi": 900000,
        "price_delta_percent": 1.80,
        "net_long": 520000,
        "net_short": 380000
      }
    ]
  }
}
```

### 字段说明

#### 外层字段

| 字段 | 类型 | 说明 |
|-----|------|------|
| count | int | 返回的币种数量 |
| exchange | string | 交易所，固定为 `binance` |
| time_range | string | 时间范围显示名称 |
| time_range_param | string | 时间范围参数值 |
| rank_type | string | 排行类型：`top` 增加 / `low` 减少 |
| limit | int | 请求的数量限制 |
| positions | array | 持仓数据列表 |

#### positions 数组字段

| 字段 | 类型 | 说明 |
|-----|------|------|
| rank | int | 排名 |
| symbol | string | 币种交易对，如 `BTCUSDT` |
| oi_delta | float | 持仓量变化（单位：币） |
| oi_delta_value | float | 持仓价值变化（单位：USDT），**排序依据** |
| oi_delta_percent | float | 持仓量变化百分比（%） |
| current_oi | float | 当前持仓量（单位：币） |
| price_delta_percent | float | 价格变化百分比（%） |
| net_long | float | 净多头持仓量 |
| net_short | float | 净空头持仓量 |

---

## 数据解读

### 持仓量与价格的关系

| 持仓变化 | 价格变化 | 市场含义 |
|---------|---------|---------|
| 增加 | 上涨 | 多头主导，上涨趋势可能延续 |
| 增加 | 下跌 | 空头主导，下跌趋势可能延续 |
| 减少 | 上涨 | 空头平仓，可能是反弹 |
| 减少 | 下跌 | 多头平仓，可能是回调 |

### 多空比例

- `net_long > net_short`：市场整体偏多
- `net_long < net_short`：市场整体偏空

---

## 调用示例

### cURL

```bash
curl -X GET "http://nofxaios.com:30006/api/oi/top-ranking?limit=50&duration=4h&auth=cm_568c67eae410d912c54c"
```

### Python

```python
import requests

url = "http://nofxaios.com:30006/api/oi/top-ranking"
params = {
    "limit": 50,
    "duration": "4h",
    "auth": "cm_568c67eae410d912c54c"
}

response = requests.get(url, params=params)
data = response.json()

for pos in data['data']['positions']:
    print(f"#{pos['rank']} {pos['symbol']}: 持仓价值变化 ${pos['oi_delta_value']:,.0f}")
```

### JavaScript

```javascript
const url = 'http://nofxaios.com:30006/api/oi/top-ranking?limit=50&duration=4h&auth=cm_568c67eae410d912c54c';

fetch(url)
  .then(response => response.json())
  .then(data => {
    data.data.positions.forEach(pos => {
      console.log(`#${pos.rank} ${pos.symbol}: 持仓价值变化 $${pos.oi_delta_value.toLocaleString()}`);
    });
  });
```

---

## 错误响应

| code | 说明 |
|------|------|
| 0 | 成功 |
| 401 | 认证失败（auth 无效） |
| 500 | 服务器内部错误 |

---

## 注意事项

1. 数据来源为币安交易所
2. 排行依据为 `oi_delta_value`（持仓价值变化），非持仓量变化
3. 数据缓存 2 秒，高频请求会命中缓存
4. `limit` 最大值为 100
