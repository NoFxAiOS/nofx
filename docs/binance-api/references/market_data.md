# Binance-Api - Market Data

**Pages:** 6

---

## SBE Market Data Streams

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/sbe-market-data-streams

**Contents:**
- SBE Market Data Streams
- General Information​
- WebSocket Limits​
- Available Streams​
  - Trades Streams​
  - Best Bid/Ask Streams​
  - Diff. Depth Streams​
  - Partial Book Depth Streams​

Raw trade information, pushed in real-time.

SBE Message Name: TradesStreamEvent

Stream Name: <symbol>@trade

Update Speed: Real time

The best bid and ask price and quantity, pushed in real-time when the order book changes.

[!NOTE] Best bid/ask streams in SBE are the equivalent of bookTicker streams in JSON, except they support auto-culling, and also include the eventTime field.

SBE Message Name: BestBidAskStreamEvent

Stream Name: <symbol>@bestBidAsk

Update Speed: Real time

SBE best bid/ask streams use auto-culling: when our system is under high load, we may drop outdated events instead of queuing all events and delivering them with a delay.

For example, if a best bid/ask event is generated at time T2 when we still have an undelivered event queued at time T1 (where T1 < T2), the event for T1 is dropped, and we will deliver only the event for T2. This is done on a per-symbol basis.

Incremental updates to the order book, pushed at regular intervals. Use this stream to maintain a local order book.

How to manage a local order book.

SBE Message Name: DepthDiffStreamEvent

Stream Name: <symbol>@depth

Snapshots of the top 20 levels of the order book, pushed at regular intervals.

SBE Message Name: DepthSnapshotStreamEvent

Stream Name: <symbol>@depth20

**Examples:**

Example 1 (unknown):
```unknown
X-MBX-APIKEY
```

Example 2 (unknown):
```unknown
pong frames
```

Example 3 (unknown):
```unknown
TradesStreamEvent
```

Example 4 (unknown):
```unknown
BestBidAskStreamEvent
```

---

## Market Data endpoints

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/rest-api/market-data-endpoints

**Contents:**
- Market Data endpoints
  - Order book​
  - Recent trades list​
  - Old trade lookup​
  - Compressed/Aggregate trades list​
  - Kline/Candlestick data​
  - UIKlines​
  - Current average price​
  - 24hr ticker price change statistics​
  - Trading Day Ticker​

Weight: Adjusted based on the limit:

Data Source: Database

Get compressed, aggregate trades. Trades that fill at the time, from the same taker order, with the same price will have the quantity aggregated.

Data Source: Database

Kline/candlestick bars for a symbol. Klines are uniquely identified by their open time.

Supported kline intervals (case-sensitive):

Data Source: Database

The request is similar to klines having the same parameters and response.

uiKlines return modified kline data, optimized for presentation of candlestick charts.

Data Source: Database

Current average price for a symbol.

24 hour rolling window price change statistics. Careful when accessing this with no symbol.

Price change statistics for a trading day.

4 for each requested symbol. The weight for this request will cap at 200 once the number of symbols in the request is more than 50.

Data Source: Database

Latest price for a symbol or symbols.

Best price/qty on the order book for a symbol or symbols.

Note: This endpoint is different from the GET /api/v3/ticker/24hr endpoint.

The window used to compute statistics will be no more than 59999ms from the requested windowSize.

openTime for /api/v3/ticker always starts on a minute, while the closeTime is the current time of the request. As such, the effective window will be up to 59999ms wider than windowSize.

E.g. If the closeTime is 1641287867099 (January 04, 2022 09:17:47:099 UTC) , and the windowSize is 1d. the openTime will be: 1641201420000 (January 3, 2022, 09:17:00)

4 for each requested symbol regardless of windowSize. The weight for this request will cap at 200 once the number of symbols in the request is more than 50.

Data Source: Database

**Examples:**

Example 1 (text):
```text
GET /api/v3/depth
```

Example 2 (text):
```text
GET /api/v3/depth
```

Example 3 (javascript):
```javascript
{  "lastUpdateId": 1027024,  "bids": [    [      "4.00000000",     // PRICE      "431.00000000"    // QTY    ]  ],  "asks": [    [      "4.00000200",      "12.00000000"    ]  ]}
```

Example 4 (javascript):
```javascript
{  "lastUpdateId": 1027024,  "bids": [    [      "4.00000000",     // PRICE      "431.00000000"    // QTY    ]  ],  "asks": [    [      "4.00000200",      "12.00000000"    ]  ]}
```

---

## Market data requests

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/websocket-api/market-data-requests

**Contents:**
- Market data requests
  - Order book​
  - Recent trades​
  - Historical trades​
  - Aggregate trades​
  - Klines​
  - UI Klines​
  - Current average price​
  - 24hr ticker price change statistics​
  - Trading Day Ticker​

Get current order book.

Note that this request returns limited market depth.

If you need to continuously monitor order book updates, please consider using WebSocket Streams:

You can use depth request together with <symbol>@depth streams to maintain a local order book.

Weight: Adjusted based on the limit:

If you need access to real-time trading activity, please consider using WebSocket Streams:

Get historical trades.

Data Source: Database

Get aggregate trades.

An aggregate trade (aggtrade) represents one or more individual trades. Trades that fill at the same time, from the same taker order, with the same price – those trades are collected into an aggregate trade with total quantity of the individual trades.

If you need access to real-time trading activity, please consider using WebSocket Streams:

If you need historical aggregate trade data, please consider using data.binance.vision.

If fromId is specified, return aggtrades with aggregate trade ID >= fromId.

Use fromId and limit to page through all aggtrades.

If startTime and/or endTime are specified, aggtrades are filtered by execution time (T).

fromId cannot be used together with startTime and endTime.

If no condition is specified, the most recent aggregate trades are returned.

Data Source: Database

Get klines (candlestick bars).

Klines are uniquely identified by their open & close time.

If you need access to real-time kline updates, please consider using WebSocket Streams:

If you need historical kline data, please consider using data.binance.vision.

Supported kline intervals (case-sensitive):

Data Source: Database

Get klines (candlestick bars) optimized for presentation.

This request is similar to klines, having the same parameters and response. uiKlines return modified kline data, optimized for presentation of candlestick charts.

Data Source: Database

Get current average price for a symbol.

Get 24-hour rolling window price change statistics.

If you need to continuously monitor trading statistics, please consider using WebSocket Streams:

If you need different window sizes, use the ticker request.

Weight: Adjusted based on the number of requested symbols:

symbol and symbols cannot be used together.

If no symbol is specified, returns information about all symbols currently trading on the exchange.

FULL type, for a single symbol:

MINI type, for a single symbol:

If more than one symbol is requested, response returns an array:

Price change statistics for a trading day.

4 for each requested symbol. The weight for this request will cap at 200 once the number of symbols in the request is more than 50.

Data Source: Database

Get rolling window price change statistics with a custom window.

This request is similar to ticker.24hr, but statistics are computed on demand using the arbitrary window you specify.

Note: Window size precision is limited to 1 minute. While the closeTime is the current time of the request, openTime always start on a minute boundary. As such, the effective window might be up to 59999 ms wider than the requested windowSize.

For example, a request for "windowSize": "7d" might result in the following window:

Time of the request – closeTime – is 1660184865291 (August 11, 2022 02:27:45.291). Requested window size should put the openTime 7 days before that – August 4, 02:27:45.291 – but due to limited precision it ends up a bit earlier: 1659580020000 (August 4, 2022 02:27:00), exactly at the start of a minute.

If you need to continuously monitor trading statistics, please consider using WebSocket Streams:

Weight: Adjusted based on the number of requested symbols:

Supported window sizes:

Either symbol or symbols must be specified.

Maximum number of symbols in one request: 200.

Window size units cannot be combined. E.g., 1d 2h is not supported.

Data Source: Database

FULL type, for a single symbol:

MINI type, for a single symbol:

If more than one symbol is requested, response returns an array:

Get the latest market price for a symbol.

If you need access to real-time price updates, please consider using WebSocket Streams:

Weight: Adjusted based on the number of requested symbols:

symbol and symbols cannot be used together.

If no symbol is specified, returns information about all symbols currently trading on the exchange.

If more than one symbol is requested, response returns an array:

Get the current best price and quantity on the order book.

If you need access to real-time order book ticker updates, please consider using WebSocket Streams:

Weight: Adjusted based on the number of requested symbols:

symbol and symbols cannot be used together.

If no symbol is specified, returns information about all symbols currently trading on the exchange.

If more than one symbol is requested, response returns an array:

**Examples:**

Example 1 (javascript):
```javascript
{  "id": "51e2affb-0aba-4821-ba75-f2625006eb43",  "method": "depth",  "params": {    "symbol": "BNBBTC",    "limit": 5  }}
```

Example 2 (javascript):
```javascript
{  "id": "51e2affb-0aba-4821-ba75-f2625006eb43",  "method": "depth",  "params": {    "symbol": "BNBBTC",    "limit": 5  }}
```

Example 3 (unknown):
```unknown
<symbol>@depth<levels>
```

Example 4 (unknown):
```unknown
<symbol>@depth
```

---

## SBE Market Data Streams

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/sbe-market-data-streams

**Contents:**
- SBE Market Data Streams
- General Information​
- WebSocket Limits​
- Available Streams​
  - Trades Streams​
  - Best Bid/Ask Streams​
  - Diff. Depth Streams​
  - Partial Book Depth Streams​

Raw trade information, pushed in real-time.

SBE Message Name: TradesStreamEvent

Stream Name: <symbol>@trade

Update Speed: Real time

The best bid and ask price and quantity, pushed in real-time when the order book changes.

[!NOTE] Best bid/ask streams in SBE are the equivalent of bookTicker streams in JSON, except they support auto-culling, and also include the eventTime field.

SBE Message Name: BestBidAskStreamEvent

Stream Name: <symbol>@bestBidAsk

Update Speed: Real time

SBE best bid/ask streams use auto-culling: when our system is under high load, we may drop outdated events instead of queuing all events and delivering them with a delay.

For example, if a best bid/ask event is generated at time T2 when we still have an undelivered event queued at time T1 (where T1 < T2), the event for T1 is dropped, and we will deliver only the event for T2. This is done on a per-symbol basis.

Incremental updates to the order book, pushed at regular intervals. Use this stream to maintain a local order book.

How to manage a local order book.

SBE Message Name: DepthDiffStreamEvent

Stream Name: <symbol>@depth

Snapshots of the top 20 levels of the order book, pushed at regular intervals.

SBE Message Name: DepthSnapshotStreamEvent

Stream Name: <symbol>@depth20

**Examples:**

Example 1 (unknown):
```unknown
X-MBX-APIKEY
```

Example 2 (unknown):
```unknown
pong frames
```

Example 3 (unknown):
```unknown
TradesStreamEvent
```

Example 4 (unknown):
```unknown
BestBidAskStreamEvent
```

---

## Market data requests

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/websocket-api/market-data-requests

**Contents:**
- Market data requests
  - Order book​
  - Recent trades​
  - Historical trades​
  - Aggregate trades​
  - Klines​
  - UI Klines​
  - Current average price​
  - 24hr ticker price change statistics​
  - Trading Day Ticker​

Get current order book.

Note that this request returns limited market depth.

If you need to continuously monitor order book updates, please consider using WebSocket Streams:

You can use depth request together with <symbol>@depth streams to maintain a local order book.

Weight: Adjusted based on the limit:

If you need access to real-time trading activity, please consider using WebSocket Streams:

Get historical trades.

Data Source: Database

Get aggregate trades.

An aggregate trade (aggtrade) represents one or more individual trades. Trades that fill at the same time, from the same taker order, with the same price – those trades are collected into an aggregate trade with total quantity of the individual trades.

If you need access to real-time trading activity, please consider using WebSocket Streams:

If you need historical aggregate trade data, please consider using data.binance.vision.

If fromId is specified, return aggtrades with aggregate trade ID >= fromId.

Use fromId and limit to page through all aggtrades.

If startTime and/or endTime are specified, aggtrades are filtered by execution time (T).

fromId cannot be used together with startTime and endTime.

If no condition is specified, the most recent aggregate trades are returned.

Data Source: Database

Get klines (candlestick bars).

Klines are uniquely identified by their open & close time.

If you need access to real-time kline updates, please consider using WebSocket Streams:

If you need historical kline data, please consider using data.binance.vision.

Supported kline intervals (case-sensitive):

Data Source: Database

Get klines (candlestick bars) optimized for presentation.

This request is similar to klines, having the same parameters and response. uiKlines return modified kline data, optimized for presentation of candlestick charts.

Data Source: Database

Get current average price for a symbol.

Get 24-hour rolling window price change statistics.

If you need to continuously monitor trading statistics, please consider using WebSocket Streams:

If you need different window sizes, use the ticker request.

Weight: Adjusted based on the number of requested symbols:

symbol and symbols cannot be used together.

If no symbol is specified, returns information about all symbols currently trading on the exchange.

FULL type, for a single symbol:

MINI type, for a single symbol:

If more than one symbol is requested, response returns an array:

Price change statistics for a trading day.

4 for each requested symbol. The weight for this request will cap at 200 once the number of symbols in the request is more than 50.

Data Source: Database

Get rolling window price change statistics with a custom window.

This request is similar to ticker.24hr, but statistics are computed on demand using the arbitrary window you specify.

Note: Window size precision is limited to 1 minute. While the closeTime is the current time of the request, openTime always start on a minute boundary. As such, the effective window might be up to 59999 ms wider than the requested windowSize.

For example, a request for "windowSize": "7d" might result in the following window:

Time of the request – closeTime – is 1660184865291 (August 11, 2022 02:27:45.291). Requested window size should put the openTime 7 days before that – August 4, 02:27:45.291 – but due to limited precision it ends up a bit earlier: 1659580020000 (August 4, 2022 02:27:00), exactly at the start of a minute.

If you need to continuously monitor trading statistics, please consider using WebSocket Streams:

Weight: Adjusted based on the number of requested symbols:

Supported window sizes:

Either symbol or symbols must be specified.

Maximum number of symbols in one request: 200.

Window size units cannot be combined. E.g., 1d 2h is not supported.

Data Source: Database

FULL type, for a single symbol:

MINI type, for a single symbol:

If more than one symbol is requested, response returns an array:

Get the latest market price for a symbol.

If you need access to real-time price updates, please consider using WebSocket Streams:

Weight: Adjusted based on the number of requested symbols:

symbol and symbols cannot be used together.

If no symbol is specified, returns information about all symbols currently trading on the exchange.

If more than one symbol is requested, response returns an array:

Get the current best price and quantity on the order book.

If you need access to real-time order book ticker updates, please consider using WebSocket Streams:

Weight: Adjusted based on the number of requested symbols:

symbol and symbols cannot be used together.

If no symbol is specified, returns information about all symbols currently trading on the exchange.

If more than one symbol is requested, response returns an array:

**Examples:**

Example 1 (javascript):
```javascript
{  "id": "51e2affb-0aba-4821-ba75-f2625006eb43",  "method": "depth",  "params": {    "symbol": "BNBBTC",    "limit": 5  }}
```

Example 2 (javascript):
```javascript
{  "id": "51e2affb-0aba-4821-ba75-f2625006eb43",  "method": "depth",  "params": {    "symbol": "BNBBTC",    "limit": 5  }}
```

Example 3 (unknown):
```unknown
<symbol>@depth<levels>
```

Example 4 (unknown):
```unknown
<symbol>@depth
```

---

## Market Data endpoints

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/rest-api/market-data-endpoints

**Contents:**
- Market Data endpoints
  - Order book​
  - Recent trades list​
  - Old trade lookup​
  - Compressed/Aggregate trades list​
  - Kline/Candlestick data​
  - UIKlines​
  - Current average price​
  - 24hr ticker price change statistics​
  - Trading Day Ticker​

Weight: Adjusted based on the limit:

Data Source: Database

Get compressed, aggregate trades. Trades that fill at the time, from the same taker order, with the same price will have the quantity aggregated.

Data Source: Database

Kline/candlestick bars for a symbol. Klines are uniquely identified by their open time.

Supported kline intervals (case-sensitive):

Data Source: Database

The request is similar to klines having the same parameters and response.

uiKlines return modified kline data, optimized for presentation of candlestick charts.

Data Source: Database

Current average price for a symbol.

24 hour rolling window price change statistics. Careful when accessing this with no symbol.

Price change statistics for a trading day.

4 for each requested symbol. The weight for this request will cap at 200 once the number of symbols in the request is more than 50.

Data Source: Database

Latest price for a symbol or symbols.

Best price/qty on the order book for a symbol or symbols.

Note: This endpoint is different from the GET /api/v3/ticker/24hr endpoint.

The window used to compute statistics will be no more than 59999ms from the requested windowSize.

openTime for /api/v3/ticker always starts on a minute, while the closeTime is the current time of the request. As such, the effective window will be up to 59999ms wider than windowSize.

E.g. If the closeTime is 1641287867099 (January 04, 2022 09:17:47:099 UTC) , and the windowSize is 1d. the openTime will be: 1641201420000 (January 3, 2022, 09:17:00)

4 for each requested symbol regardless of windowSize. The weight for this request will cap at 200 once the number of symbols in the request is more than 50.

Data Source: Database

**Examples:**

Example 1 (text):
```text
GET /api/v3/depth
```

Example 2 (text):
```text
GET /api/v3/depth
```

Example 3 (unknown):
```unknown
tradingStatus
```

Example 4 (javascript):
```javascript
{  "lastUpdateId": 1027024,  "bids": [    [      "4.00000000",     // PRICE      "431.00000000"    // QTY    ]  ],  "asks": [    [      "4.00000200",      "12.00000000"    ]  ]}
```

---
