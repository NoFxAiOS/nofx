# 币种综合数据接口文档

## 接口概述

该接口提供单个币种的综合数据查询，一次请求即可获取资金净流入、持仓变化、价格变化等多维度数据。

## 请求信息

### 接口地址

```
GET /api/coin/{symbol}
```

### 完整示例

```
http://nofxaios.com:30006/api/coin/PIPPINUSDT?include=netflow,oi,price&auth=cm_568c67eae410d912c54c
```

### 请求参数

| 参数 | 位置 | 类型 | 必填 | 说明 |
|-----|------|------|-----|------|
| symbol | path | string | 是 | 币种符号，如 `PIPPINUSDT`、`ETH`（会自动补全USDT后缀） |
| include | query | string | 否 | 返回数据类型，逗号分隔。可选值：`netflow,oi,price`。默认返回全部 |
| auth | query | string | 是 | 认证密钥 |

### include 参数说明

| 值 | 说明 |
|---|------|
| netflow | 资金净流入数据（机构/散户、合约/现货） |
| oi | 持仓数据（币安、Bybit） |
| price | 价格变化百分比 |

---

## 返回数据

### 完整响应示例

```json
{
  "code": 0,
  "data": {
    "symbol": "PIPPINUSDT",
    "price": 0.085,
    "netflow": {
      "institution": {
        "future": {
          "1m": 120000,
          "5m": 580000,
          "15m": 1200000,
          "30m": 2500000,
          "1h": 5800000,
          "4h": 12000000,
          "8h": 25000000,
          "12h": 38000000,
          "24h": 65000000,
          "2d": 120000000,
          "3d": 180000000
        },
        "spot": {
          "1m": 50000,
          "5m": 280000,
          "15m": 600000,
          "30m": 1200000,
          "1h": 2800000,
          "4h": 6000000,
          "8h": 12000000,
          "12h": 18000000,
          "24h": 32000000,
          "2d": 60000000,
          "3d": 90000000
        }
      },
      "personal": {
        "future": {
          "1m": -80000,
          "5m": -350000,
          "15m": -800000,
          "30m": -1500000,
          "1h": -3200000,
          "4h": -8000000,
          "8h": -15000000,
          "12h": -22000000,
          "24h": -40000000,
          "2d": -75000000,
          "3d": -110000000
        },
        "spot": {
          "1m": -30000,
          "5m": -150000,
          "15m": -400000,
          "30m": -800000,
          "1h": -1800000,
          "4h": -4000000,
          "8h": -8000000,
          "12h": -12000000,
          "24h": -22000000,
          "2d": -40000000,
          "3d": -60000000
        }
      }
    },
    "oi": {
      "binance": {
        "current_oi": 85000,
        "net_long": 48000,
        "net_short": 37000,
        "delta": {
          "1m": {
            "oi_delta": 150,
            "oi_delta_value": 14550000,
            "oi_delta_percent": 0.18
          },
          "5m": {
            "oi_delta": 680,
            "oi_delta_value": 65960000,
            "oi_delta_percent": 0.8
          },
          "1h": {
            "oi_delta": 2500,
            "oi_delta_value": 242500000,
            "oi_delta_percent": 2.94
          },
          "4h": {
            "oi_delta": 5200,
            "oi_delta_value": 504400000,
            "oi_delta_percent": 6.12
          },
          "24h": {
            "oi_delta": 8500,
            "oi_delta_value": 824500000,
            "oi_delta_percent": 10.0
          }
        }
      },
      "bybit": {
        "current_oi": 42000,
        "net_long": 24000,
        "net_short": 18000,
        "delta": {
          "1h": {
            "oi_delta": 1200,
            "oi_delta_value": 116400000,
            "oi_delta_percent": 2.86
          }
        }
      }
    },
    "price_change": {
      "1m": 0.05,
      "5m": 0.18,
      "15m": 0.35,
      "30m": 0.62,
      "1h": 1.25,
      "4h": 2.80,
      "8h": 3.50,
      "12h": 2.95,
      "24h": 4.80,
      "2d": 6.50,
      "3d": 8.20
    }
  }
}
```

---

## 字段详细说明

### 基础字段

| 字段 | 类型 | 说明 |
|-----|------|------|
| symbol | string | 币种交易对，如 `PIPPINUSDT` |
| price | float | 当前期货价格（单位：USDT） |

---

### netflow - 资金净流入

资金净流入数据，**正数表示资金流入，负数表示资金流出**，单位为 USDT。

#### 数据结构

```
netflow
├── institution          # 机构资金
│   ├── future          # 合约市场
│   └── spot            # 现货市场
└── personal            # 散户资金
    ├── future          # 合约市场
    └── spot            # 现货市场
```

#### 分类说明

| 字段 | 说明 |
|-----|------|
| institution.future | 机构在合约市场的资金净流入 |
| institution.spot | 机构在现货市场的资金净流入 |
| personal.future | 散户在合约市场的资金净流入 |
| personal.spot | 散户在现货市场的资金净流入 |

#### 时间周期

| 字段 | 说明 |
|-----|------|
| 1m | 最近 1 分钟 |
| 5m | 最近 5 分钟 |
| 15m | 最近 15 分钟 |
| 30m | 最近 30 分钟 |
| 1h | 最近 1 小时 |
| 4h | 最近 4 小时 |
| 8h | 最近 8 小时 |
| 12h | 最近 12 小时 |
| 24h | 最近 24 小时 |
| 2d | 最近 2 天 |
| 3d | 最近 3 天 |

#### 使用建议

- **机构资金流入 + 散户资金流出** = 典型的主力吸筹信号
- **机构资金流出 + 散户资金流入** = 典型的主力出货信号
- 关注 **合约与现货的资金流向是否一致**，判断市场情绪

---

### oi - 持仓数据

持仓量（Open Interest）数据，来源于币安和 Bybit 交易所。

#### 字段说明

| 字段 | 类型 | 说明 |
|-----|------|------|
| current_oi | float | 当前总持仓量（单位：币） |
| net_long | float | 净多头持仓量 |
| net_short | float | 净空头持仓量 |
| delta | object | 各时间周期的持仓变化 |

#### delta 子字段

| 字段 | 类型 | 说明 |
|-----|------|------|
| oi_delta | float | 持仓量变化（单位：币） |
| oi_delta_value | float | 持仓价值变化（单位：USDT） |
| oi_delta_percent | float | 持仓量变化百分比（%） |

#### 使用建议

- **持仓量增加 + 价格上涨** = 多头主导，趋势可能延续
- **持仓量增加 + 价格下跌** = 空头主导，下跌趋势可能延续
- **持仓量减少 + 价格变化** = 平仓为主，趋势可能反转
- **net_long > net_short** = 市场整体偏多

---

### price_change - 价格变化

各时间周期的价格涨跌幅，**单位为百分比（%）**，正数表示上涨，负数表示下跌。

| 字段 | 说明 |
|-----|------|
| 1m | 最近 1 分钟涨跌幅 |
| 5m | 最近 5 分钟涨跌幅 |
| 15m | 最近 15 分钟涨跌幅 |
| 30m | 最近 30 分钟涨跌幅 |
| 1h | 最近 1 小时涨跌幅 |
| 4h | 最近 4 小时涨跌幅 |
| 8h | 最近 8 小时涨跌幅 |
| 12h | 最近 12 小时涨跌幅 |
| 24h | 最近 24 小时涨跌幅 |
| 2d | 最近 2 天涨跌幅 |
| 3d | 最近 3 天涨跌幅 |

---

## 错误响应

| code | 说明 |
|------|------|
| 0 | 成功 |
| 400 | 参数错误（如缺少 symbol） |
| 401 | 认证失败（auth 无效） |
| 500 | 服务器内部错误 |

错误响应示例：

```json
{
  "code": 400,
  "message": "symbol parameter is required"
}
```

---

## 调用示例

### cURL

```bash
curl -X GET "http://nofxaios.com:30006/api/coin/PIPPINUSDT?include=netflow,oi,price&auth=cm_568c67eae410d912c54c"
```

### Python

```python
import requests

url = "http://nofxaios.com:30006/api/coin/PIPPINUSDT"
params = {
    "include": "netflow,oi,price",
    "auth": "cm_568c67eae410d912c54c"
}

response = requests.get(url, params=params)
data = response.json()

print(f"当前价格: {data['data']['price']}")
print(f"1小时机构合约净流入: {data['data']['netflow']['institution']['future']['1h']}")
print(f"24小时价格涨跌幅: {data['data']['price_change']['24h']}%")
```

### JavaScript

```javascript
const url = 'http://nofxaios.com:30006/api/coin/PIPPINUSDT?include=netflow,oi,price&auth=cm_568c67eae410d912c54c';

fetch(url)
  .then(response => response.json())
  .then(data => {
    console.log('当前价格:', data.data.price);
    console.log('1小时机构合约净流入:', data.data.netflow.institution.future['1h']);
    console.log('24小时价格涨跌幅:', data.data.price_change['24h'], '%');
  });
```

---

## 注意事项

1. **symbol 参数**：支持带或不带 `USDT` 后缀，如 `PIPPIN` 和 `PIPPINUSDT` 等效
2. **include 参数**：可按需选择返回数据，减少不必要的数据传输
3. **数据更新频率**：数据实时更新，建议轮询间隔不低于 1 秒
4. **资金流向解读**：机构与散户的资金流向通常呈相反趋势，可作为市场情绪判断依据
