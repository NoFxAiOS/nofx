# CryptoMaster API 接口文档

## 概述

### 基础信息
- **Base URL**: `https://nofxos.ai`
- **响应格式**: JSON
- **缓存时间**: 15秒（所有数据接口）
- **限流**: 每个IP每秒最多30次请求

### 认证方式
所有数据接口需要认证，支持两种方式：

#### 方式1: Query参数（推荐）
```
GET /api/ai500/list?auth=your_api_key
```

#### 方式2: Authorization Header
```
GET /api/ai500/list
Authorization: Bearer your_api_key
```

### 响应格式

**成功响应：**
```json
{
  "success": true,
  "data": { ... }
}
```

**错误响应：**
```json
{
  "success": false,
  "error": "错误信息"
}
```

---

## 重要：数值格式说明

### 百分比字段格式

不同接口的百分比字段使用不同的格式，请注意区分：

| 字段名 | 格式 | 示例 | 说明 |
|--------|------|------|------|
| `price_delta` (涨跌幅榜/币种详情) | **小数** | `0.05` = 5% | 需要 ×100 转换为百分比 |
| `oi_delta_percent` | **已×100** | `5.0` = 5% | 直接使用，无需转换 |
| `price_delta_percent` (OI接口) | **已×100** | `5.0` = 5% | 直接使用，无需转换 |
| `increase_percent` (AI500) | **已×100** | `7.14` = 7.14% | 直接使用，无需转换 |

### 金额字段

| 字段名 | 单位 | 说明 |
|--------|------|------|
| `oi_delta_value` | USDT | 持仓价值变化 |
| `amount` / `future_flow` / `spot_flow` | USDT | 资金流量 |
| `price` | USDT | 当前价格 |

### 持仓量字段

| 字段名 | 单位 | 说明 |
|--------|------|------|
| `oi_delta` | 张/个 | 持仓量变化 |
| `current_oi` / `oi` | 张/个 | 当前持仓量 |
| `net_long` / `net_short` | 张/个 | 净多头/空头持仓 |

---

## 时间范围参数说明

所有接口支持的 `duration` 参数值：

| 参数值 | 说明 | 备注 |
|--------|------|------|
| `1m` | 1分钟 | |
| `5m` | 5分钟 | |
| `15m` | 15分钟 | |
| `30m` | 30分钟 | |
| `1h` | 1小时 | 默认值 |
| `4h` | 4小时 | |
| `8h` | 8小时 | |
| `12h` | 12小时 | |
| `24h` / `1d` | 24小时 | 两种写法均可 |
| `2d` | 2天 | |
| `3d` | 3天 | |
| `5d` | 5天 | |
| `7d` | 7天 | |

---

## 1. AI500 智能评分接口

AI500 是基于多维度量化指标的智能评分系统，用于筛选具有上涨潜力的币种。

### 1.1 获取AI500推荐币种列表

获取经过严格筛选的优质币种列表。

**请求**
```
GET /api/ai500/list
```

**过滤条件**
- AI评分 > 70
- 币安OI持仓价值 > 15M USDT
- 现价 > 上榜起始价格（只返回上涨中的币种）
- 资金没有持续流出（1h/4h/12h/24h不能全为负）

**响应示例**
```json
{
  "success": true,
  "data": {
    "count": 5,
    "coins": [
      {
        "pair": "BTCUSDT",
        "score": 85.234,
        "start_time": 1704067200,
        "start_price": 42000.5,
        "last_score": 83.5,
        "max_score": 87.2,
        "max_price": 45000.0,
        "increase_percent": 7.14
      }
    ]
  }
}
```

**字段说明**
| 字段 | 类型 | 说明 |
|------|------|------|
| `pair` | string | 交易对名称，如 BTCUSDT |
| `score` | float | 当前AI评分（0-100） |
| `start_time` | int64 | 上榜时间戳（Unix秒） |
| `start_price` | float | 上榜时价格（USDT） |
| `last_score` | float | 上次记录的评分 |
| `max_score` | float | 在榜期间最高评分 |
| `max_price` | float | 在榜期间最高价格（USDT） |
| `increase_percent` | float | 最大涨幅百分比（**已×100**，7.14 = 7.14%） |

---

### 1.2 获取单个币种AI500信息

**请求**
```
GET /api/ai500/:symbol
```

**路径参数**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `symbol` | string | 是 | 币种符号，支持 `BTCUSDT` 或 `BTC` 格式 |

**示例**
```
GET /api/ai500/BTC
GET /api/ai500/ETHUSDT
```

**响应示例**
```json
{
  "success": true,
  "data": {
    "info": {
      "pair": "BTCUSDT",
      "score": 85.234,
      "start_time": 1704067200,
      "start_price": 42000.5,
      "last_score": 83.5,
      "max_score": 87.2,
      "max_price": 45000.0,
      "increase_percent": 7.14
    },
    "current_price": 44500.0,
    "score": 85.234
  }
}
```

---

### 1.3 获取AI500统计信息

获取AI500整体统计数据。

**请求**
```
GET /api/ai500/stats
```

**响应示例**
```json
{
  "success": true,
  "data": {
    "statistics": {
      "total_count": 50,
      "average_score": 72.5,
      "max_score": 95.2,
      "min_score": 55.3,
      "average_increase": 12.5
    },
    "top_coins": [...],
    "bottom_coins": [...]
  }
}
```

---

## 2. 持仓量(OI)排行接口

监控各币种的合约持仓量变化，用于判断市场资金动向。

### 2.1 获取OI增加排行榜

返回持仓价值增加最多的币种排行。

**请求**
```
GET /api/oi/top-ranking
```

**查询参数**
| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | int | 20 | 返回数量，最大100 |
| `duration` | string | `1h` | 时间范围，见[时间范围参数](#时间范围参数说明) |

**示例**
```
GET /api/oi/top-ranking?limit=50&duration=4h
```

**响应示例**
```json
{
  "success": true,
  "data": {
    "count": 20,
    "exchange": "binance",
    "time_range": "4小时",
    "time_range_param": "4h",
    "rank_type": "top",
    "limit": 50,
    "positions": [
      {
        "rank": 1,
        "symbol": "BTCUSDT",
        "price": 44500.0,
        "oi_delta": 1500.5,
        "oi_delta_value": 65000000,
        "oi_delta_percent": 2.5,
        "current_oi": 62000,
        "price_delta_percent": 1.2,
        "net_long": 35000,
        "net_short": 27000
      }
    ]
  }
}
```

**字段说明**
| 字段 | 类型 | 格式 | 说明 |
|------|------|------|------|
| `rank` | int | - | 排名 |
| `symbol` | string | - | 交易对名称 |
| `price` | float | USDT | 当前价格 |
| `oi_delta` | float | 张/个 | 持仓量变化 |
| `oi_delta_value` | float | USDT | 持仓价值变化（**排序依据**） |
| `oi_delta_percent` | float | **已×100** | 持仓量变化百分比，2.5 = 2.5% |
| `current_oi` | float | 张/个 | 当前持仓量 |
| `price_delta_percent` | float | **已×100** | 价格变化百分比，1.2 = 1.2% |
| `net_long` | float | 张/个 | 净多头持仓 |
| `net_short` | float | 张/个 | 净空头持仓 |

---

### 2.2 获取OI减少排行榜

返回持仓价值减少最多的币种排行。

**请求**
```
GET /api/oi/low-ranking
```

**查询参数**
同 [OI增加排行榜](#21-获取oi增加排行榜)

**示例**
```
GET /api/oi/low-ranking?limit=30&duration=24h
```

---

### 2.3 获取OI Top20（向后兼容）

**请求**
```
GET /api/oi/top
```

固定返回1小时内OI增加最多的Top20，用于向后兼容。

---

## 3. 资金流量(NetFlow)排行接口

监控机构和散户的资金流向。

### 3.1 获取资金流入排行榜

**请求**
```
GET /api/netflow/top-ranking
```

**查询参数**
| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | int | 20 | 返回数量，最大100 |
| `duration` | string | `1h` | 时间范围，见[时间范围参数](#时间范围参数说明) |
| `type` | string | `institution` | 资金类型：`institution`(机构), `personal`(散户) |
| `trade` | string | `future` | 交易类型：`future`(合约), `spot`(现货) |

**示例**
```
GET /api/netflow/top-ranking?limit=30&duration=4h&type=institution&trade=future
```

**响应示例**
```json
{
  "success": true,
  "data": {
    "count": 30,
    "type": "institution",
    "trade": "合约",
    "time_range": "4h",
    "rank_type": "top",
    "limit": 30,
    "netflows": [
      {
        "rank": 1,
        "symbol": "BTCUSDT",
        "amount": 15000000.5,
        "price": 44500.0
      }
    ]
  }
}
```

**字段说明**
| 字段 | 类型 | 格式 | 说明 |
|------|------|------|------|
| `rank` | int | - | 排名 |
| `symbol` | string | - | 交易对名称 |
| `amount` | float | USDT | 资金流量，**正数=流入，负数=流出** |
| `price` | float | USDT | 当前价格 |

---

### 3.2 获取资金流出排行榜

**请求**
```
GET /api/netflow/low-ranking
```

**查询参数**
同 [资金流入排行榜](#31-获取资金流入排行榜)

**示例**
```
GET /api/netflow/low-ranking?limit=20&duration=1h&type=personal&trade=spot
```

---

### 3.3 获取资金流入Top20（向后兼容）

**请求**
```
GET /api/netflow/top
```

固定返回1小时内机构合约资金流入最多的Top20。

---

## 4. 涨跌幅榜接口

### 4.1 获取涨跌幅榜

同时返回涨幅榜(top)和跌幅榜(low)，支持多个时间周期同时查询。

**请求**
```
GET /api/price/ranking
```

**查询参数**
| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `duration` | string | `1h` | 时间范围，可多选逗号分隔：`1h,4h,24h` |
| `limit` | int | 20 | 每个榜单返回数量，最大100 |
| `exchange` | string | `binance` | 交易所 |

**示例**
```
GET /api/price/ranking?duration=1h,4h,24h&limit=20
```

**响应示例**
```json
{
  "success": true,
  "data": {
    "durations": ["1h", "4h", "24h"],
    "limit": 20,
    "data": {
      "1h": {
        "top": [
          {
            "pair": "MOGUSDT",
            "symbol": "MOG",
            "price_delta": 0.0723,
            "price": 0.00123,
            "future_flow": 201500,
            "spot_flow": 0,
            "oi": 15000000,
            "oi_delta": 500000,
            "oi_delta_value": 615
          }
        ],
        "low": [
          {
            "pair": "XYZUSDT",
            "symbol": "XYZ",
            "price_delta": -0.0512,
            "price": 1.234,
            "future_flow": -50000,
            "spot_flow": -10000,
            "oi": 8000000,
            "oi_delta": -200000,
            "oi_delta_value": -246800
          }
        ]
      },
      "4h": { ... },
      "24h": { ... }
    }
  }
}
```

**字段说明**
| 字段 | 类型 | 格式 | 说明 |
|------|------|------|------|
| `pair` | string | - | 完整交易对名称，如 BTCUSDT |
| `symbol` | string | - | 币种符号（去除USDT），如 BTC |
| `price_delta` | float | **小数** | 价格变动比例，**0.0723 = 7.23%**（需×100显示） |
| `price` | float | USDT | 当前价格 |
| `future_flow` | float | USDT | 合约资金流量，正数=流入 |
| `spot_flow` | float | USDT | 现货资金流量，正数=流入 |
| `oi` | float | 张/个 | 当前持仓量 |
| `oi_delta` | float | 张/个 | 持仓变化量 |
| `oi_delta_value` | float | USDT | 持仓变化价值 |

> **注意**：`price_delta` 使用小数格式，与 OI 接口的 `price_delta_percent` 不同！

---

## 5. 币种详情接口

### 5.1 获取单币种完整数据

获取指定币种的所有统计信息，一次调用获取全部数据。

**请求**
```
GET /api/coin/:symbol
```

**路径参数**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `symbol` | string | 是 | 币种符号，支持 `BTC` 或 `BTCUSDT` 格式 |

**查询参数**
| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `include` | string | `netflow,oi,price,ai500` | 包含的数据类型，逗号分隔 |

**include 参数选项**
| 值 | 说明 |
|------|------|
| `netflow` | 资金流量数据（机构/散户，合约/现货） |
| `oi` | 持仓量数据（币安/Bybit） |
| `price` | 价格变化数据 |
| `ai500` | AI500评分 |

**示例**
```
GET /api/coin/BTC?include=netflow,oi,price,ai500
GET /api/coin/ETHUSDT?include=netflow,oi
```

**响应示例**
```json
{
  "success": true,
  "data": {
    "symbol": "BTCUSDT",
    "price": 44500.0,
    "ai500": {
      "score": 85.234,
      "is_active": true,
      "start_time": 1704067200,
      "start_price": 42000.5,
      "increase_percent": 5.95
    },
    "netflow": {
      "institution": {
        "future": {
          "1m": 50000,
          "5m": 200000,
          "15m": 500000,
          "30m": 800000,
          "1h": 1500000,
          "4h": 5000000,
          "8h": 8000000,
          "12h": 10000000,
          "24h": 15000000,
          "2d": 25000000,
          "3d": 35000000,
          "5d": 50000000,
          "7d": 75000000
        },
        "spot": { ... }
      },
      "personal": {
        "future": { ... },
        "spot": { ... }
      }
    },
    "oi": {
      "binance": {
        "current_oi": 62000,
        "net_long": 35000,
        "net_short": 27000,
        "delta": {
          "1m": {
            "oi_delta": 50,
            "oi_delta_value": 2225000,
            "oi_delta_percent": 0.08
          },
          "5m": { ... },
          "1h": { ... },
          "4h": { ... },
          "24h": { ... }
        }
      },
      "bybit": { ... }
    },
    "price_change": {
      "1m": 0.001,
      "5m": 0.005,
      "15m": 0.008,
      "30m": 0.012,
      "1h": 0.015,
      "4h": 0.025,
      "8h": 0.035,
      "12h": 0.042,
      "24h": 0.055,
      "2d": 0.08,
      "3d": 0.12,
      "5d": 0.18,
      "7d": 0.25
    }
  }
}
```

**字段说明**

**price_change 对象**
| 字段 | 类型 | 格式 | 说明 |
|------|------|------|------|
| `{duration}` | float | **小数** | 价格变化比例，**0.015 = 1.5%**（需×100显示） |

**netflow 对象**
| 路径 | 类型 | 格式 | 说明 |
|------|------|------|------|
| `institution.future.{duration}` | float | USDT | 机构合约资金流量 |
| `institution.spot.{duration}` | float | USDT | 机构现货资金流量 |
| `personal.future.{duration}` | float | USDT | 散户合约资金流量 |
| `personal.spot.{duration}` | float | USDT | 散户现货资金流量 |

**oi 对象**
| 路径 | 类型 | 格式 | 说明 |
|------|------|------|------|
| `binance.current_oi` | float | 张/个 | 币安当前持仓量 |
| `binance.net_long` | float | 张/个 | 币安净多头 |
| `binance.net_short` | float | 张/个 | 币安净空头 |
| `binance.delta.{duration}.oi_delta` | float | 张/个 | 持仓量变化 |
| `binance.delta.{duration}.oi_delta_value` | float | USDT | 持仓价值变化 |
| `binance.delta.{duration}.oi_delta_percent` | float | **已×100** | 持仓变化百分比，0.08 = 0.08% |
| `bybit.*` | - | - | Bybit数据，结构同上 |

**ai500 对象**
| 字段 | 类型 | 格式 | 说明 |
|------|------|------|------|
| `score` | float | 0-100 | AI综合评分 |
| `is_active` | bool | - | 是否为活跃高分币种 |
| `start_time` | int64 | Unix秒 | 上榜时间 |
| `start_price` | float | USDT | 上榜时价格 |
| `increase_percent` | float | **已×100** | 最大涨幅，5.95 = 5.95% |

---

## 错误码说明

| HTTP状态码 | 说明 | 常见原因 |
|------------|------|----------|
| 200 | 成功 | - |
| 400 | 请求参数错误 | 参数格式不正确、缺少必填参数 |
| 401 | 未授权 | 缺少认证信息或API Key无效 |
| 404 | 资源不存在 | 币种不存在或未被追踪 |
| 429 | 请求过于频繁 | 超过限流阈值（30次/秒） |
| 500 | 服务器内部错误 | 服务端异常 |

**错误响应示例**
```json
{
  "success": false,
  "error": "unauthorized"
}
```

---

## 使用示例

### cURL 示例

```bash
# 方式1: Query参数认证
curl "https://nofxos.ai/api/ai500/list?auth=your_api_key"

# 方式2: Header认证
curl "https://nofxos.ai/api/ai500/list" \
  -H "Authorization: Bearer your_api_key"

# 获取1小时涨跌幅榜
curl "https://nofxos.ai/api/price/ranking?duration=1h&limit=20&auth=your_api_key"

# 获取多个时间周期涨跌幅榜
curl "https://nofxos.ai/api/price/ranking?duration=1h,4h,24h&limit=10&auth=your_api_key"

# 获取BTC详细数据
curl "https://nofxos.ai/api/coin/BTC?auth=your_api_key"

# 只获取BTC的资金流和OI数据
curl "https://nofxos.ai/api/coin/BTC?include=netflow,oi&auth=your_api_key"

# 获取4小时OI增加排行Top50
curl "https://nofxos.ai/api/oi/top-ranking?duration=4h&limit=50&auth=your_api_key"

# 获取24小时OI减少排行Top30
curl "https://nofxos.ai/api/oi/low-ranking?duration=24h&limit=30&auth=your_api_key"

# 获取机构合约资金流入排行
curl "https://nofxos.ai/api/netflow/top-ranking?type=institution&trade=future&duration=1h&auth=your_api_key"

# 获取散户现货资金流出排行
curl "https://nofxos.ai/api/netflow/low-ranking?type=personal&trade=spot&duration=4h&auth=your_api_key"
```

### Python 示例

```python
import requests

BASE_URL = "https://nofxos.ai"
API_KEY = "your_api_key"

# 方式1: Query参数认证
def get_with_query_auth(endpoint, params=None):
    if params is None:
        params = {}
    params["auth"] = API_KEY
    response = requests.get(f"{BASE_URL}{endpoint}", params=params)
    return response.json()

# 方式2: Header认证
def get_with_header_auth(endpoint, params=None):
    headers = {"Authorization": f"Bearer {API_KEY}"}
    response = requests.get(f"{BASE_URL}{endpoint}", params=params, headers=headers)
    return response.json()

# 获取AI500列表
def get_ai500_list():
    return get_with_query_auth("/api/ai500/list")

# 获取涨跌幅榜
def get_price_ranking(durations="1h,4h,24h", limit=20):
    return get_with_query_auth("/api/price/ranking", {
        "duration": durations,
        "limit": limit
    })

# 获取币种详情
def get_coin_stats(symbol, include="netflow,oi,price,ai500"):
    return get_with_query_auth(f"/api/coin/{symbol}", {
        "include": include
    })

# 获取OI排行
def get_oi_ranking(rank_type="top", duration="1h", limit=20):
    endpoint = f"/api/oi/{rank_type}-ranking"
    return get_with_query_auth(endpoint, {
        "duration": duration,
        "limit": limit
    })

# 获取资金流排行
def get_netflow_ranking(rank_type="top", duration="1h", limit=20,
                        flow_type="institution", trade="future"):
    endpoint = f"/api/netflow/{rank_type}-ranking"
    return get_with_query_auth(endpoint, {
        "duration": duration,
        "limit": limit,
        "type": flow_type,
        "trade": trade
    })

# 使用示例
if __name__ == "__main__":
    # 获取AI500推荐币种
    ai500 = get_ai500_list()
    print(f"AI500推荐币种数量: {ai500['data']['count']}")

    # 获取1小时涨幅榜前10
    ranking = get_price_ranking("1h", 10)
    for coin in ranking['data']['data']['1h']['top'][:3]:
        # 注意: price_delta 是小数，需要×100
        pct = coin['price_delta'] * 100
        print(f"{coin['symbol']}: {pct:.2f}%")

    # 获取BTC详情
    btc = get_coin_stats("BTC")
    # 注意: price_change 是小数
    print(f"BTC 1小时涨跌: {btc['data']['price_change']['1h'] * 100:.2f}%")

    # 获取4小时OI增加Top20
    oi = get_oi_ranking("top", "4h", 20)
    for pos in oi['data']['positions'][:3]:
        # 注意: oi_delta_percent 已×100
        print(f"{pos['symbol']}: OI变化 {pos['oi_delta_percent']:.2f}%")
```

### JavaScript/TypeScript 示例

```typescript
const BASE_URL = "https://nofxos.ai";
const API_KEY = "your_api_key";

// 通用请求函数
async function apiRequest<T>(endpoint: string, params: Record<string, any> = {}): Promise<T> {
    const url = new URL(`${BASE_URL}${endpoint}`);
    params.auth = API_KEY;
    Object.entries(params).forEach(([key, value]) => {
        url.searchParams.append(key, String(value));
    });

    const response = await fetch(url.toString());
    return response.json();
}

// 获取涨跌幅榜
interface PriceRankingItem {
    pair: string;
    symbol: string;
    price_delta: number;  // 小数格式，0.05 = 5%
    price: number;
    future_flow: number;
    spot_flow: number;
}

async function getPriceRanking(durations = "1h", limit = 20) {
    const data = await apiRequest<any>("/api/price/ranking", { duration: durations, limit });
    return data;
}

// 使用示例
async function main() {
    const ranking = await getPriceRanking("1h,4h", 10);

    for (const coin of ranking.data.data["1h"].top) {
        // 转换为百分比显示
        const pctChange = (coin.price_delta * 100).toFixed(2);
        console.log(`${coin.symbol}: ${pctChange}%`);
    }
}
```

---

## 常见问题

### Q: 为什么有些百分比字段格式不同？

A: 这是历史原因造成的：
- **OI接口**的 `oi_delta_percent` 和 `price_delta_percent` 是**已乘100**的格式（5.0 = 5%）
- **涨跌幅榜和币种详情**的 `price_delta` / `price_change` 是**小数**格式（0.05 = 5%）

建议在前端显示时统一处理。

### Q: duration 参数支持哪些值？

A: 支持以下值：`1m`, `5m`, `15m`, `30m`, `1h`, `4h`, `8h`, `12h`, `24h`(或`1d`), `2d`, `3d`, `5d`, `7d`

### Q: 如何判断资金是流入还是流出？

A: `amount`、`future_flow`、`spot_flow` 等字段：
- **正数** = 资金流入
- **负数** = 资金流出

### Q: API缓存时间是多久？

A: 所有数据接口缓存15秒，相同请求在15秒内返回缓存数据。

### Q: 限流规则是什么？

A: 每个IP每秒最多30次请求，超过会返回 429 错误。
