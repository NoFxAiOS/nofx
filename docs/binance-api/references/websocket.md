# Binance-Api - Websocket

**Pages:** 24

---

## User Data Streams for Binance Spot TESTNET

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/user-data-stream

**Contents:**
- User Data Streams for Binance Spot TESTNET
- General information​
- User Data Stream Events​
  - Account Update​
  - Balance Update​
  - Order Update​
    - Conditional Fields in Execution Report​
    - Order Reject Reason​
    - Execution types​
  - Event Stream Terminated​

Last Updated: 2025-10-24

outboundAccountPosition is sent any time an account balance has changed and contains the assets that were possibly changed by the event that generated the balance change.

Balance Update occurs during the following:

Orders are updated with the executionReport event.

Note: Average price can be found by doing Z divided by z.

These are fields that appear in the payload only if certain conditions are met.

For additional information on these parameters, please refer to the Spot Glossary.

For additional details, look up the Error Message in the Errors documentation.

If the order is an order list, an event named ListStatus will be sent in addition to the executionReport event.

Check the Enums Documentation for more relevant enum definitions.

eventStreamTerminated is sent when the User Data Stream is stopped. For example, after you send a userDataStream.unsubscribe request, or a session.logout request.

externalLockUpdate is sent when part of your spot wallet balance is locked/unlocked by an external system, for example when used as margin collateral.

**Examples:**

Example 1 (unknown):
```unknown
outboundAccountPosition
```

Example 2 (javascript):
```javascript
{  "subscriptionId": 0,  "event": {    "e": "outboundAccountPosition", // Event type    "E": 1564034571105,             // Event Time    "u": 1564034571073,             // Time of last account update    "B":                            // Balances Array    [      {        "a": "ETH",                 // Asset        "f": "10000.000000",        // Free        "l": "0.000000"             // Locked      }    ]  }}
```

Example 3 (javascript):
```javascript
{  "subscriptionId": 0,  "event": {    "e": "outboundAccountPosition", // Event type    "E": 1564034571105,             // Event Time    "u": 1564034571073,             // Time of last account update    "B":                            // Balances Array    [      {        "a": "ETH",                 // Asset        "f": "10000.000000",        // Free        "l": "0.000000"             // Locked      }    ]  }}
```

Example 4 (javascript):
```javascript
{  "subscriptionId": 0,  "event": {    "e": "balanceUpdate",         // Event Type    "E": 1573200697110,           // Event Time    "a": "BTC",                   // Asset    "d": "100.00000000",          // Balance Delta    "T": 1573200697068            // Clear Time  }}
```

---

## Event format

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/websocket-api/event-format

**Contents:**
- Event format

User Data Stream events for non-SBE sessions are sent as JSON in text frames, one event per frame

Events in SBE sessions will be sent as binary frames.

Please refer to userDataStream.subscribe for details on how to subscribe to User Data Stream in WebSocket API.

**Examples:**

Example 1 (unknown):
```unknown
userDataStream.subscribe
```

Example 2 (javascript):
```javascript
{  "subscriptionId": 0,  "event": {    "e": "outboundAccountPosition",    "E": 1728972148778,    "u": 1728972148778,    "B": [      {        "a": "BTC",        "f": "11818.00000000",        "l": "182.00000000"      },      {        "a": "USDT",        "f": "10580.00000000",        "l": "70.00000000"      }    ]  }}
```

Example 3 (javascript):
```javascript
{  "subscriptionId": 0,  "event": {    "e": "outboundAccountPosition",    "E": 1728972148778,    "u": 1728972148778,    "B": [      {        "a": "BTC",        "f": "11818.00000000",        "l": "182.00000000"      },      {        "a": "USDT",        "f": "10580.00000000",        "l": "70.00000000"      }    ]  }}
```

Example 4 (unknown):
```unknown
subscriptionId
```

---

## Response format

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/websocket-api/response-format

**Contents:**
- Response format
  - Status codes​

Responses are returned as JSON in text frames, one response per frame.

Example of successful response:

Example of failed response:

Status codes in the status field are the same as in HTTP.

Here are some common status codes that you might encounter:

See Error codes for Binance for a list of error codes and messages.

**Examples:**

Example 1 (json):
```json
{  "id": "e2a85d9f-07a5-4f94-8d5f-789dc3deb097",  "status": 200,  "result": {    "symbol": "BTCUSDT",    "orderId": 12510053279,    "orderListId": -1,    "clientOrderId": "a097fe6304b20a7e4fc436",    "transactTime": 1655716096505,    "price": "0.10000000",    "origQty": "10.00000000",    "executedQty": "0.00000000",    "origQuoteOrderQty": "0.000000",    "cummulativeQuoteQty": "0.00000000",    "status": "NEW",    "timeInForce": "GTC",    "type": "LIMIT",    "side": "BUY",    "workingTime": 1655716096505,    "selfTradePreventionMode": "NONE"  },  "rateLimits": [    {      "rateLimitType": "ORDERS",      "interval": "SECOND",      "intervalNum": 10,      "limit": 50,      "count": 12    },    {      "rateLimitType": "ORDERS",      "interval": "DAY",      "intervalNum": 1,      "limit": 160000,      "count": 4043    },    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 321    }  ]}
```

Example 2 (json):
```json
{  "id": "e2a85d9f-07a5-4f94-8d5f-789dc3deb097",  "status": 200,  "result": {    "symbol": "BTCUSDT",    "orderId": 12510053279,    "orderListId": -1,    "clientOrderId": "a097fe6304b20a7e4fc436",    "transactTime": 1655716096505,    "price": "0.10000000",    "origQty": "10.00000000",    "executedQty": "0.00000000",    "origQuoteOrderQty": "0.000000",    "cummulativeQuoteQty": "0.00000000",    "status": "NEW",    "timeInForce": "GTC",    "type": "LIMIT",    "side": "BUY",    "workingTime": 1655716096505,    "selfTradePreventionMode": "NONE"  },  "rateLimits": [    {      "rateLimitType": "ORDERS",      "interval": "SECOND",      "intervalNum": 10,      "limit": 50,      "count": 12    },    {      "rateLimitType": "ORDERS",      "interval": "DAY",      "intervalNum": 1,      "limit": 160000,      "count": 4043    },    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 321    }  ]}
```

Example 3 (json):
```json
{  "id": "e2a85d9f-07a5-4f94-8d5f-789dc3deb097",  "status": 400,  "error": {    "code": -2010,    "msg": "Account has insufficient balance for requested action."  },  "rateLimits": [    {      "rateLimitType": "ORDERS",      "interval": "SECOND",      "intervalNum": 10,      "limit": 50,      "count": 13    },    {      "rateLimitType": "ORDERS",      "interval": "DAY",      "intervalNum": 1,      "limit": 160000,      "count": 4044    },    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 322    }  ]}
```

Example 4 (json):
```json
{  "id": "e2a85d9f-07a5-4f94-8d5f-789dc3deb097",  "status": 400,  "error": {    "code": -2010,    "msg": "Account has insufficient balance for requested action."  },  "rateLimits": [    {      "rateLimitType": "ORDERS",      "interval": "SECOND",      "intervalNum": 10,      "limit": 50,      "count": 13    },    {      "rateLimitType": "ORDERS",      "interval": "DAY",      "intervalNum": 1,      "limit": 160000,      "count": 4044    },    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 322    }  ]}
```

---

## Data sources

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/websocket-api/data-sources

**Contents:**
- Data sources

The API system is asynchronous. Some delay in the response is normal and expected.

Each method has a data source indicating where the data is coming from, and thus how up-to-date it is.

Some methods have more than one data source (e.g., Memory => Database).

This means that the API will look for the latest data in that order: first in the cache, then in the database.

---

## Request format

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/websocket-api/request-format

**Contents:**
- Request format

Requests must be sent as JSON in text frames, one request per frame.

Request id is truly arbitrary. You can use UUIDs, sequential IDs, current timestamp, etc. The server does not interpret id in any way, simply echoing it back in the response.

You can freely reuse IDs within a session. However, be careful to not send more than one request at a time with the same ID, since otherwise it might be impossible to tell the responses apart.

Request method names may be prefixed with explicit version: e.g., "v3/order.place".

The order of params is not significant.

**Examples:**

Example 1 (json):
```json
{  "id": "e2a85d9f-07a5-4f94-8d5f-789dc3deb097",  "method": "order.place",  "params": {    "symbol": "BTCUSDT",    "side": "BUY",    "type": "LIMIT",    "price": "0.1",    "quantity": "10",    "timeInForce": "GTC",    "timestamp": 1655716096498,    "apiKey": "T59MTDLWlpRW16JVeZ2Nju5A5C98WkMm8CSzWC4oqynUlTm1zXOxyauT8LmwXEv9",    "signature": "5942ad337e6779f2f4c62cd1c26dba71c91514400a24990a3e7f5edec9323f90"  }}
```

Example 2 (json):
```json
{  "id": "e2a85d9f-07a5-4f94-8d5f-789dc3deb097",  "method": "order.place",  "params": {    "symbol": "BTCUSDT",    "side": "BUY",    "type": "LIMIT",    "price": "0.1",    "quantity": "10",    "timeInForce": "GTC",    "timestamp": 1655716096498,    "apiKey": "T59MTDLWlpRW16JVeZ2Nju5A5C98WkMm8CSzWC4oqynUlTm1zXOxyauT8LmwXEv9",    "signature": "5942ad337e6779f2f4c62cd1c26dba71c91514400a24990a3e7f5edec9323f90"  }}
```

Example 3 (unknown):
```unknown
"v3/order.place"
```

---

## User Data Streams for Binance

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/user-data-stream

**Contents:**
- User Data Streams for Binance
- General information​
- User Data Stream Events​
  - Account Update​
  - Balance Update​
  - Order Update​
    - Conditional Fields in Execution Report​
    - Order Reject Reason​
- Event Stream Terminated​
- External Lock Update​

Last Updated: 2025-10-24

outboundAccountPosition is sent any time an account balance has changed and contains the assets that were possibly changed by the event that generated the balance change.

Balance Update occurs during the following:

Orders are updated with the executionReport event.

Note: Average price can be found by doing Z divided by z.

These are fields that appear in the payload only if certain conditions are met.

For additional information on these parameters, please refer to the Spot Glossary.

For additional details, look up the Error Message in the Errors documentation.

If the order is an order list, an event named ListStatus will be sent in addition to the executionReport event.

Check the Enums page for more relevant enum definitions.

eventStreamTerminated is sent when the User Data Stream is stopped. For example, after you send a userDataStream.unsubscribe request, or a session.logout request.

externalLockUpdate is sent when part of your spot wallet balance is locked/unlocked by an external system, for example when used as margin collateral.

**Examples:**

Example 1 (unknown):
```unknown
outboundAccountPosition
```

Example 2 (javascript):
```javascript
{  "subscriptionId": 0,  "event": {    "e": "outboundAccountPosition", // Event type    "E": 1564034571105,             // Event Time    "u": 1564034571073,             // Time of last account update    "B":                            // Balances Array    [      {        "a": "ETH",                 // Asset        "f": "10000.000000",        // Free        "l": "0.000000"             // Locked      }    ]  }}
```

Example 3 (javascript):
```javascript
{  "subscriptionId": 0,  "event": {    "e": "outboundAccountPosition", // Event type    "E": 1564034571105,             // Event Time    "u": 1564034571073,             // Time of last account update    "B":                            // Balances Array    [      {        "a": "ETH",                 // Asset        "f": "10000.000000",        // Free        "l": "0.000000"             // Locked      }    ]  }}
```

Example 4 (javascript):
```javascript
{  "subscriptionId": 0,  "event": {    "e": "balanceUpdate",         // Event Type    "E": 1573200697110,           // Event Time    "a": "BTC",                   // Asset    "d": "100.00000000",          // Balance Delta    "T": 1573200697068            // Clear Time  }}
```

---

## CHANGELOG for Binance's API

**URL:** https://developers.binance.com/docs/binance-spot-api-docs

**Contents:**
- CHANGELOG for Binance's API
  - 2025-10-24​
    - SBE​
    - REST and WebSocket API​
  - 2025-10-21​
  - 2025-10-08​
    - FIX API​
  - 2025-09-29​
  - 2025-09-18​
  - 2025-09-12​

Last Updated: 2025-10-24

Following the announcement from 2025-04-07, all documentation related with listenKey for use on wss://stream.binance.com has been removed.

Please refer to the list of requests and methods below for more information.

The features will remain available until a future retirement announcement is made.

REST and WebSocket API:

Notice: The following changes will be deployed on 2025-09-29, starting at 10:00 UTC and may take several hours to complete.

Notice: The changes in this section will be gradually rolled out, and will take approximately up to two weeks to complete.

The following changes will be available on 2025-08-27 starting at 07:00 UTC:

The following changes will be available on 2025-08-28 starting at 07:00 UTC:

REST and WebSocket API:

Notice: The following changes will happen at 2025-06-06 7:00 UTC.

Clarification on the release of Order Amend Keep Priority and STP Decrement:

Notice: The changes in this section will be gradually rolled out, and will take a week to complete.

Notice: The changes in this section will be gradually rolled out, and will take a week to complete.

Notice: The following changes will occur during April 21, 2025.

The following changes will occur at April 24, 2025, 07:00 UTC:

The system now supports microseconds in all related time and/or timestamp fields. Microsecond support is opt-in, by default the requests and responses still use milliseconds. Examples in documentation are also using milliseconds for the foreseeable future.

Notice: The changes below will be rolled out starting at 2024-12-12 and may take approximately a week to complete.

The following changes will occur between 2024-12-16 to 2024-12-20:

REST and WebSocket API:

Changes to Exchange Information (i.e. GET /api/v3/exchangeInfo from REST and exchangeInfo for WebSocket API).

Notice: The changes below are being rolled out gradually, and may take approximately a week to complete.

This will be available by June 6, 11:59 UTC.

The following changes have been postponed to take effect on April 25, 05:00 UTC

Notice: The changes below are being rolled out gradually, and will take approximately a week to complete.

The following will take effect approximately a week after the release date:

This will take effect on March 5, 2024.

Simple Binary Encoding (SBE) will be added to the live exchange, both for the Rest API and WebSocket API.

For more information on SBE, please refer to the FAQ

The SPOT WebSocket API can now support SBE on SPOT Testnet.

The SBE schema has been updated with WebSocket API metadata without incrementing either schemaId or version.

Users using SBE only on the REST API may continue to use the SBE schema with git commit hash 128b94b2591944a536ae427626b795000100cf1d or update to the newly-published SBE schema.

Users who want to use SBE on the WebSocket API must use the newly-published SBE schema.

The FAQ for SBE has been updated.

Simple Binary Encoding (SBE) has been added to SPOT Testnet.

This will be added to the live exchange at a later date.

For more information on what SBE is, please refer to the FAQ

Notice: The changes below are being rolled out gradually, and will take approximately a week to complete.

The following will take effect approximately a week after the release date:

Effective on 2023-10-19 00:00 UTC

The following changes will be effective from 2023-08-25 at UTC 00:00.

Please refer to the table for more details:

Smart Order Routing (SOR) has been added to the APIs. For more information please refer to our FAQ. Please wait for future announcements on when the feature will be enabled.

Notice: The change below are being rolled out, and will take approximately a week to complete.

The following changes will take effect approximately a week from the release date::

Notice: The change below are being rolled out, and will take approximately a week to complete.

Notice: All changes are being rolled out gradually to all our servers, and may take a week to complete.

The following changes will take effect approximately a week from the release date, but the rest of the documentation has been updated to reflect the future changes:

Changes to Websocket Limits

The WS-API and Websocket Stream now only allows 300 connections requests every 5 minutes.

This limit is per IP address.

Please be careful when trying to open multiple connections or reconnecting to the Websocket API.

As per the announcement, Self Trade Prevention will be enabled at 2023-01-26 08:00 UTC.

Please refer to GET /api/v3/exchangeInfo from the Rest API or exchangeInfo from the Websocket API on the default and allowed modes.

New API cluster has been added. Note that all endpoints are functionally equal, but may vary in performance.

ACTUAL RELEASE DATE TBD

New Feature: Self-Trade Prevention (aka STP) will be added to the system at a later date. This will prevent orders from matching with orders from the same account, or accounts under the same tradeGroupId.

Please refer to GET /api/v3/exchangeInfo from the Rest API or exchangeInfo from the Websocket API on the status.

Additional details on the functionality of STP is explained in the STP FAQ document.

WEBSOCKET API WILL BE AVAILABLE ON THE LIVE EXCHANGE AT A LATER DATE.

Some error messages on error code -1003 have changed.

Notice: These changes are being rolled out gradually to all our servers, and will take approximately a week to complete.

Fixed a bug where symbol + orderId combination would return all trades even if the number of trades went beyond the 500 default limit.

Previous behavior: The API would send specific error messages depending on the combination of parameters sent. E.g:

New behavior: If the combinations of optional parameters to the endpoint were not supported, then the endpoint will respond with the generic error:

Added a new combination of supported parameters: symbol + orderId + fromId.

The following combinations of parameters were previously supported but no longer accepted, as these combinations were only taking fromId into consideration, ignoring startTime and endTime:

Thus, these are the supported combinations of parameters:

Note: These new fields will appear approximately a week from the release date.

Scheduled changes to the removal of !bookTicker around November 2022.

Note that these are rolling changes, so it may take a few days for it to rollout to all our servers.

Note that these are rolling changes, so it may take a few days for it to rollout to all our servers.

Changes to GET /api/v3/ticker

Note: The update is being rolled out over the next few days, so these changes may not be visible right away.

Changes to Order Book Depth Levels

What does this affect?

Updates to MAX_POSITION

Note: The changes are being rolled out during the next few days, so these will not appear right away.

On April 28, 2021 00:00 UTC the weights to the following endpoints will be adjusted:

New API clusters have been added in order to improve performance.

Users can access any of the following API clusters, in addition to api.binance.com

If there are any performance issues with accessing api.binance.com please try any of the following instead:

This filter defines the allowed maximum position an account can have on the base asset of a symbol. An account's position defined as the sum of the account's:

BUY orders will be rejected if the account's position is greater than the maximum position allowed.

Deprecation of v1 endpoints:

By end of Q1 2020, the following endpoints will be removed from the API. The documentation has been updated to use the v3 versions of these endpoints.

These endpoints however, will NOT be migrated to v3. Please use the following endpoints instead moving forward.

Changes toexecutionReport event

balanceUpdate event type added

In Q4 2017, the following endpoints were deprecated and removed from the API documentation. They have been permanently removed from the API as of this version. We apologize for the omission from the original changelog:

Streams, endpoints, parameters, payloads, etc. described in the documents in this repository are considered official and supported. The use of any other streams, endpoints, parameters, or payloads, etc. is not supported; use them at your own risk and with no guarantees.

New order type: OCO ("One Cancels the Other")

An OCO has 2 orders: (also known as legs in financial terms)

Quantity Restrictions:

recvWindow cannot exceed 60000.

New intervalLetter values for headers:

New Headers X-MBX-USED-WEIGHT-(intervalNum)(intervalLetter) will give your current used request weight for the (intervalNum)(intervalLetter) rate limiter. For example, if there is a one minute request rate weight limiter set, you will get a X-MBX-USED-WEIGHT-1M header in the response. The legacy header X-MBX-USED-WEIGHT will still be returned and will represent the current used weight for the one minute request rate weight limit.

New Header X-MBX-ORDER-COUNT-(intervalNum)(intervalLetter)that is updated on any valid order placement and tracks your current order count for the interval; rejected/unsuccessful orders are not guaranteed to have X-MBX-ORDER-COUNT-** headers in the response.

GET api/v1/depth now supports limit 5000 and 10000; weights are 50 and 100 respectively.

GET api/v1/exchangeInfo has a new parameter ocoAllowed.

(qty * price) of all trades / sum of qty of all trades over previous 5 minutes.

If there is no trade in the last 5 minutes, it takes the first trade that happened outside of the 5min window. For example if the last trade was 20 minutes ago, that trade's price is the 5 min average.

If there is no trade on the symbol, there is no average price and market orders cannot be placed. On a new symbol with applyToMarket enabled on the MIN_NOTIONAL filter, market orders cannot be placed until there is at least 1 trade.

The current average price can be checked here: https://api.binance.com/api/v3/avgPrice?symbol=<symbol> For example: https://api.binance.com/api/v3/avgPrice?symbol=BNBUSDT

**Examples:**

Example 1 (unknown):
```unknown
wss://stream.binance.com
```

Example 2 (unknown):
```unknown
POST /api/v3/userDataStream
```

Example 3 (unknown):
```unknown
PUT /api/v3/userDataStream
```

Example 4 (unknown):
```unknown
DELETE /api/v3/userDataStream
```

---

## General requests

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/websocket-api/general-requests

**Contents:**
- General requests
  - Test connectivity​
  - Check server time​
  - Exchange information​

Test connectivity to the WebSocket API.

Note: You can use regular WebSocket ping frames to test connectivity as well, WebSocket API will respond with pong frames as soon as possible. ping request along with time is a safe way to test request-response handling in your application.

Test connectivity to the WebSocket API and get the current server time.

Query current exchange trading rules, rate limits, and symbol information.

Only one of symbol, symbols, permissions parameters can be specified.

Without parameters, exchangeInfo displays all symbols with ["SPOT, "MARGIN", "LEVERAGED"] permissions.

permissions accepts either a list of permissions, or a single permission name. E.g. "SPOT".

Available Permissions

Examples of Symbol Permissions Interpretation from the Response:

**Examples:**

Example 1 (javascript):
```javascript
{  "id": "922bcc6e-9de8-440d-9e84-7c80933a8d0d",  "method": "ping"}
```

Example 2 (javascript):
```javascript
{  "id": "922bcc6e-9de8-440d-9e84-7c80933a8d0d",  "method": "ping"}
```

Example 3 (javascript):
```javascript
{  "id": "922bcc6e-9de8-440d-9e84-7c80933a8d0d",  "status": 200,  "result": {},  "rateLimits": [    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 1    }  ]}
```

Example 4 (javascript):
```javascript
{  "id": "922bcc6e-9de8-440d-9e84-7c80933a8d0d",  "status": 200,  "result": {},  "rateLimits": [    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 1    }  ]}
```

---

## Data sources

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/websocket-api/data-sources

**Contents:**
- Data sources

The API system is asynchronous. Some delay in the response is normal and expected.

Each method has a data source indicating where the data is coming from, and thus how up-to-date it is.

Some methods have more than one data source (e.g., Memory => Database).

This means that the API will look for the latest data in that order: first in the cache, then in the database.

---

## Rate limits

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/websocket-api/rate-limits

**Contents:**
- Rate limits
  - Connection limits​
  - General information on rate limits​
    - How to interpret rate limits​
    - How to show/hide rate limit information​
  - IP limits​
  - Unfilled Order Count​

There is a limit of 300 connections per attempt every 5 minutes.

The connection is per IP address.

A response with rate limit status may look like this:

The rateLimits array describes all currently active rate limits affected by the request.

Rate limits are accounted by intervals.

For example, a 1 MINUTE interval starts every minute. Request submitted at 00:01:23.456 counts towards the 00:01:00 minute's limit. Once the 00:02:00 minute starts, the count will reset to zero again.

Other intervals behave in a similar manner. For example, 1 DAY rate limit resets at 00:00 UTC every day, and 10 SECOND interval resets at 00, 10, 20... seconds of each minute.

APIs have multiple rate-limiting intervals. If you exhaust a shorter interval but the longer interval still allows requests, you will have to wait for the shorter interval to expire and reset. If you exhaust a longer interval, you will have to wait for that interval to reset, even if shorter rate limit count is zero.

rateLimits field is included with every response by default.

However, rate limit information can be quite bulky. If you are not interested in detailed rate limit status of every request, the rateLimits field can be omitted from responses to reduce their size.

Optional returnRateLimits boolean parameter in request.

Use returnRateLimits parameter to control whether to include rateLimits fields in response to individual requests.

Default request and response:

Request and response without rate limit status:

Optional returnRateLimits boolean parameter in connection URL.

If you wish to omit rateLimits from all responses by default, use returnRateLimits parameter in the query string instead:

This will make all requests made through this connection behave as if you have passed "returnRateLimits": false.

If you want to see rate limits for a particular request, you need to explicitly pass the "returnRateLimits": true parameter.

Note: Your requests are still rate limited if you hide the rateLimits field in responses.

Successful response indicating that in 1 minute you have used 70 weight out of your 6000 limit:

Failed response indicating that you are banned and the ban will last until epoch 1659146400000:

Successful response indicating that you have placed 12 orders in 10 seconds, and 4043 orders in the past 24 hours:

**Examples:**

Example 1 (unknown):
```unknown
exchangeInfo
```

Example 2 (json):
```json
{  "id": "7069b743-f477-4ae3-81db-db9b8df085d2",  "status": 200,  "result": {    "serverTime": 1656400526260  },  "rateLimits": [    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 70    }  ]}
```

Example 3 (json):
```json
{  "id": "7069b743-f477-4ae3-81db-db9b8df085d2",  "status": 200,  "result": {    "serverTime": 1656400526260  },  "rateLimits": [    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 70    }  ]}
```

Example 4 (unknown):
```unknown
rateLimitType
```

---

## General API Information

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/websocket-api/general-api-information

**Contents:**
- General API Information

**Examples:**

Example 1 (unknown):
```unknown
wss://ws-api.binance.com:443/ws-api/v3
```

Example 2 (unknown):
```unknown
wss://ws-api.testnet.binance.vision/ws-api/v3
```

Example 3 (unknown):
```unknown
pong frames
```

Example 4 (unknown):
```unknown
timeUnit=MICROSECOND
```

---

## Request format

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/websocket-api/request-format

**Contents:**
- Request format

Requests must be sent as JSON in text frames, one request per frame.

Request id is truly arbitrary. You can use UUIDs, sequential IDs, current timestamp, etc. The server does not interpret id in any way, simply echoing it back in the response.

You can freely reuse IDs within a session. However, be careful to not send more than one request at a time with the same ID, since otherwise it might be impossible to tell the responses apart.

Request method names may be prefixed with explicit version: e.g., "v3/order.place".

The order of params is not significant.

**Examples:**

Example 1 (json):
```json
{  "id": "e2a85d9f-07a5-4f94-8d5f-789dc3deb097",  "method": "order.place",  "params": {    "symbol": "BTCUSDT",    "side": "BUY",    "type": "LIMIT",    "price": "0.1",    "quantity": "10",    "timeInForce": "GTC",    "timestamp": 1655716096498,    "apiKey": "T59MTDLWlpRW16JVeZ2Nju5A5C98WkMm8CSzWC4oqynUlTm1zXOxyauT8LmwXEv9",    "signature": "5942ad337e6779f2f4c62cd1c26dba71c91514400a24990a3e7f5edec9323f90"  }}
```

Example 2 (json):
```json
{  "id": "e2a85d9f-07a5-4f94-8d5f-789dc3deb097",  "method": "order.place",  "params": {    "symbol": "BTCUSDT",    "side": "BUY",    "type": "LIMIT",    "price": "0.1",    "quantity": "10",    "timeInForce": "GTC",    "timestamp": 1655716096498,    "apiKey": "T59MTDLWlpRW16JVeZ2Nju5A5C98WkMm8CSzWC4oqynUlTm1zXOxyauT8LmwXEv9",    "signature": "5942ad337e6779f2f4c62cd1c26dba71c91514400a24990a3e7f5edec9323f90"  }}
```

Example 3 (unknown):
```unknown
"v3/order.place"
```

---

## Request security

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/websocket-api/request-security

**Contents:**
- Request security
  - SIGNED request security​
  - Timing security​
  - SIGNED request example (HMAC)​
  - SIGNED request example (RSA)​
  - SIGNED Request Example (Ed25519)​

Serious trading is about timing. Networks can be unstable and unreliable, which can lead to requests taking varying amounts of time to reach the servers. With recvWindow, you can specify that the request must be processed within a certain number of milliseconds or be rejected by the server.

It is recommended to use a small recvWindow of 5000 or less!

Here is a step-by-step guide on how to sign requests using HMAC secret key.

Example API key and secret key:

WARNING: DO NOT SHARE YOUR API KEY AND SECRET KEY WITH ANYONE.

The example keys are provided here only for illustrative purposes.

As you can see, the signature parameter is currently missing.

Step 1. Construct the signature payload

Take all request params except for the signature, sort them by name in alphabetical order:

Format parameters as parameter=value pairs separated by &.

Resulting signature payload:

Step 2. Compute the signature

Note that apiKey, secretKey, and the payload are case-sensitive, while resulting signature value is case-insensitive.

You can cross-check your signature algorithm implementation with OpenSSL:

Step 3. Add signature to request params

Finally, complete the request by adding the signature parameter with the signature string.

Here is a step-by-step guide on how to sign requests using your RSA private key.

In this example, we assume the private key is stored in the test-prv-key.pem file.

WARNING: DO NOT SHARE YOUR API KEY AND PRIVATE KEY WITH ANYONE.

The example keys are provided here only for illustrative purposes.

Step 1. Construct the signature payload

Take all request params except for the signature, sort them by name in alphabetical order:

Format parameters as parameter=value pairs separated by &.

Resulting signature payload:

Step 2. Compute the signature

Note that apiKey, the payload, and the resulting signature are case-sensitive.

You can cross-check your signature algorithm implementation with OpenSSL:

Step 3. Add signature to request params

Finally, complete the request by adding the signature parameter with the signature string.

Note: It is highly recommended to use Ed25519 API keys as it should provide the best performance and security out of all supported key types.

This is a sample code in Python to show how to sign the payload with an Ed25519 key.

**Examples:**

Example 1 (unknown):
```unknown
USER_STREAM
```

Example 2 (javascript):
```javascript
serverTime = getCurrentTime()if (timestamp < (serverTime + 1 second) && (serverTime - timestamp) <= recvWindow) {  // begin processing request  serverTime = getCurrentTime()  if (serverTime - timestamp) <= recvWindow {    // forward request to Matching Engine  } else {    // reject request  }  // finish processing request} else {  // reject request}
```

Example 3 (javascript):
```javascript
serverTime = getCurrentTime()if (timestamp < (serverTime + 1 second) && (serverTime - timestamp) <= recvWindow) {  // begin processing request  serverTime = getCurrentTime()  if (serverTime - timestamp) <= recvWindow {    // forward request to Matching Engine  } else {    // reject request  }  // finish processing request} else {  // reject request}
```

Example 4 (unknown):
```unknown
vmPUZE6mv9SD5VNHk4HlWFsOr6aKE2zvsw0MuIgwCIPy6utIco14y7Ju91duEh8A
```

---

## General API Information

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/websocket-api

**Contents:**
- General API Information

**Examples:**

Example 1 (unknown):
```unknown
wss://ws-api.binance.com:443/ws-api/v3
```

Example 2 (unknown):
```unknown
wss://ws-api.testnet.binance.vision/ws-api/v3
```

Example 3 (unknown):
```unknown
pong frames
```

Example 4 (unknown):
```unknown
timeUnit=MICROSECOND
```

---

## General requests

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/websocket-api/general-requests

**Contents:**
- General requests
  - Test connectivity​
  - Check server time​
  - Exchange information​

Test connectivity to the WebSocket API.

Note: You can use regular WebSocket ping frames to test connectivity as well, WebSocket API will respond with pong frames as soon as possible. ping request along with time is a safe way to test request-response handling in your application.

Test connectivity to the WebSocket API and get the current server time.

Query current exchange trading rules, rate limits, and symbol information.

Only one of symbol, symbols, permissions parameters can be specified.

Without parameters, exchangeInfo displays all symbols with ["SPOT, "MARGIN", "LEVERAGED"] permissions.

permissions accepts either a list of permissions, or a single permission name. E.g. "SPOT".

Available Permissions

Examples of Symbol Permissions Interpretation from the Response:

**Examples:**

Example 1 (javascript):
```javascript
{  "id": "922bcc6e-9de8-440d-9e84-7c80933a8d0d",  "method": "ping"}
```

Example 2 (javascript):
```javascript
{  "id": "922bcc6e-9de8-440d-9e84-7c80933a8d0d",  "method": "ping"}
```

Example 3 (javascript):
```javascript
{  "id": "922bcc6e-9de8-440d-9e84-7c80933a8d0d",  "status": 200,  "result": {},  "rateLimits": [    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 1    }  ]}
```

Example 4 (javascript):
```javascript
{  "id": "922bcc6e-9de8-440d-9e84-7c80933a8d0d",  "status": 200,  "result": {},  "rateLimits": [    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 1    }  ]}
```

---

## Response format

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/websocket-api/response-format

**Contents:**
- Response format
  - Status codes​

Responses are returned as JSON in text frames, one response per frame.

Example of successful response:

Example of failed response:

Status codes in the status field are the same as in HTTP.

Here are some common status codes that you might encounter:

See Error codes for Binance for a list of error codes and messages.

**Examples:**

Example 1 (json):
```json
{  "id": "e2a85d9f-07a5-4f94-8d5f-789dc3deb097",  "status": 200,  "result": {    "symbol": "BTCUSDT",    "orderId": 12510053279,    "orderListId": -1,    "clientOrderId": "a097fe6304b20a7e4fc436",    "transactTime": 1655716096505,    "price": "0.10000000",    "origQty": "10.00000000",    "executedQty": "0.00000000",    "origQuoteOrderQty": "0.000000",    "cummulativeQuoteQty": "0.00000000",    "status": "NEW",    "timeInForce": "GTC",    "type": "LIMIT",    "side": "BUY",    "workingTime": 1655716096505,    "selfTradePreventionMode": "NONE"  },  "rateLimits": [    {      "rateLimitType": "ORDERS",      "interval": "SECOND",      "intervalNum": 10,      "limit": 50,      "count": 12    },    {      "rateLimitType": "ORDERS",      "interval": "DAY",      "intervalNum": 1,      "limit": 160000,      "count": 4043    },    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 321    }  ]}
```

Example 2 (json):
```json
{  "id": "e2a85d9f-07a5-4f94-8d5f-789dc3deb097",  "status": 200,  "result": {    "symbol": "BTCUSDT",    "orderId": 12510053279,    "orderListId": -1,    "clientOrderId": "a097fe6304b20a7e4fc436",    "transactTime": 1655716096505,    "price": "0.10000000",    "origQty": "10.00000000",    "executedQty": "0.00000000",    "origQuoteOrderQty": "0.000000",    "cummulativeQuoteQty": "0.00000000",    "status": "NEW",    "timeInForce": "GTC",    "type": "LIMIT",    "side": "BUY",    "workingTime": 1655716096505,    "selfTradePreventionMode": "NONE"  },  "rateLimits": [    {      "rateLimitType": "ORDERS",      "interval": "SECOND",      "intervalNum": 10,      "limit": 50,      "count": 12    },    {      "rateLimitType": "ORDERS",      "interval": "DAY",      "intervalNum": 1,      "limit": 160000,      "count": 4043    },    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 321    }  ]}
```

Example 3 (json):
```json
{  "id": "e2a85d9f-07a5-4f94-8d5f-789dc3deb097",  "status": 400,  "error": {    "code": -2010,    "msg": "Account has insufficient balance for requested action."  },  "rateLimits": [    {      "rateLimitType": "ORDERS",      "interval": "SECOND",      "intervalNum": 10,      "limit": 50,      "count": 13    },    {      "rateLimitType": "ORDERS",      "interval": "DAY",      "intervalNum": 1,      "limit": 160000,      "count": 4044    },    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 322    }  ]}
```

Example 4 (json):
```json
{  "id": "e2a85d9f-07a5-4f94-8d5f-789dc3deb097",  "status": 400,  "error": {    "code": -2010,    "msg": "Account has insufficient balance for requested action."  },  "rateLimits": [    {      "rateLimitType": "ORDERS",      "interval": "SECOND",      "intervalNum": 10,      "limit": 50,      "count": 13    },    {      "rateLimitType": "ORDERS",      "interval": "DAY",      "intervalNum": 1,      "limit": 160000,      "count": 4044    },    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 322    }  ]}
```

---

## Event format

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/websocket-api/event-format

**Contents:**
- Event format

User Data Stream events for non-SBE sessions are sent as JSON in text frames, one event per frame.

Events in SBE sessions will be sent as binary frames.

Please refer to userDataStream.subscribe for details on how to subscribe to User Data Stream in WebSocket API.

**Examples:**

Example 1 (unknown):
```unknown
userDataStream.subscribe
```

Example 2 (javascript):
```javascript
{  "subscriptionId": 0,  "event": {    "e": "outboundAccountPosition",    "E": 1728972148778,    "u": 1728972148778,    "B": [      {        "a": "BTC",        "f": "11818.00000000",        "l": "182.00000000"      },      {        "a": "USDT",        "f": "10580.00000000",        "l": "70.00000000"      }    ]  }}
```

Example 3 (javascript):
```javascript
{  "subscriptionId": 0,  "event": {    "e": "outboundAccountPosition",    "E": 1728972148778,    "u": 1728972148778,    "B": [      {        "a": "BTC",        "f": "11818.00000000",        "l": "182.00000000"      },      {        "a": "USDT",        "f": "10580.00000000",        "l": "70.00000000"      }    ]  }}
```

Example 4 (unknown):
```unknown
subscriptionId
```

---

## General API Information

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/websocket-api/general-api-information

**Contents:**
- General API Information

**Examples:**

Example 1 (unknown):
```unknown
wss://ws-api.testnet.binance.vision/ws-api/v3
```

Example 2 (unknown):
```unknown
pong frames
```

Example 3 (unknown):
```unknown
timeUnit=MICROSECOND
```

Example 4 (unknown):
```unknown
timeUnit=microsecond
```

---

## Request security

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/websocket-api/request-security

**Contents:**
- Request security
  - SIGNED request security​
  - Timing security​
  - SIGNED request example (HMAC)​
  - SIGNED request example (RSA)​
  - SIGNED Request Example (Ed25519)​

Serious trading is about timing. Networks can be unstable and unreliable, which can lead to requests taking varying amounts of time to reach the servers. With recvWindow, you can specify that the request must be processed within a certain number of milliseconds or be rejected by the server.

It is recommended to use a small recvWindow of 5000 or less!

Here is a step-by-step guide on how to sign requests using HMAC secret key.

Example API key and secret key:

WARNING: DO NOT SHARE YOUR API KEY AND SECRET KEY WITH ANYONE.

The example keys are provided here only for illustrative purposes.

As you can see, the signature parameter is currently missing.

Step 1. Construct the signature payload

Take all request params except for the signature, sort them by name in alphabetical order:

Format parameters as parameter=value pairs separated by &.

Resulting signature payload:

Step 2. Compute the signature

Note that apiKey, secretKey, and the payload are case-sensitive, while resulting signature value is case-insensitive.

You can cross-check your signature algorithm implementation with OpenSSL:

Step 3. Add signature to request params

Finally, complete the request by adding the signature parameter with the signature string.

Here is a step-by-step guide on how to sign requests using your RSA private key.

In this example, we assume the private key is stored in the test-prv-key.pem file.

WARNING: DO NOT SHARE YOUR API KEY AND PRIVATE KEY WITH ANYONE.

The example keys are provided here only for illustrative purposes.

Step 1. Construct the signature payload

Take all request params except for the signature, sort them by name in alphabetical order:

Format parameters as parameter=value pairs separated by &.

Resulting signature payload:

Step 2. Compute the signature

Note that apiKey, the payload, and the resulting signature are case-sensitive.

You can cross-check your signature algorithm implementation with OpenSSL:

Step 3. Add signature to request params

Finally, complete the request by adding the signature parameter with the signature string.

Note: It is highly recommended to use Ed25519 API keys as it should provide the best performance and security out of all supported key types.

This is a sample code in Python to show how to sign the payload with an Ed25519 key.

**Examples:**

Example 1 (unknown):
```unknown
USER_STREAM
```

Example 2 (javascript):
```javascript
serverTime = getCurrentTime()if (timestamp < (serverTime + 1 second) && (serverTime - timestamp) <= recvWindow) {  // begin processing request  serverTime = getCurrentTime()  if (serverTime - timestamp) <= recvWindow {    // forward request to Matching Engine  } else {    // reject request  }  // finish processing request} else {  // reject request}
```

Example 3 (javascript):
```javascript
serverTime = getCurrentTime()if (timestamp < (serverTime + 1 second) && (serverTime - timestamp) <= recvWindow) {  // begin processing request  serverTime = getCurrentTime()  if (serverTime - timestamp) <= recvWindow {    // forward request to Matching Engine  } else {    // reject request  }  // finish processing request} else {  // reject request}
```

Example 4 (unknown):
```unknown
vmPUZE6mv9SD5VNHk4HlWFsOr6aKE2zvsw0MuIgwCIPy6utIco14y7Ju91duEh8A
```

---

## Rate limits

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/websocket-api/rate-limits

**Contents:**
- Rate limits
  - Connection limits​
  - General information on rate limits​
    - How to interpret rate limits​
    - How to show/hide rate limit information​
  - IP limits​
  - Unfilled Order Count​

There is a limit of 300 connections per attempt every 5 minutes.

The connection is per IP address.

A response with rate limit status may look like this:

The rateLimits array describes all currently active rate limits affected by the request.

Rate limits are accounted by intervals.

For example, a 1 MINUTE interval starts every minute. Request submitted at 00:01:23.456 counts towards the 00:01:00 minute's limit. Once the 00:02:00 minute starts, the count will reset to zero again.

Other intervals behave in a similar manner. For example, 1 DAY rate limit resets at 00:00 UTC every day, and 10 SECOND interval resets at 00, 10, 20... seconds of each minute.

APIs have multiple rate-limiting intervals. If you exhaust a shorter interval but the longer interval still allows requests, you will have to wait for the shorter interval to expire and reset. If you exhaust a longer interval, you will have to wait for that interval to reset, even if shorter rate limit count is zero.

rateLimits field is included with every response by default.

However, rate limit information can be quite bulky. If you are not interested in detailed rate limit status of every request, the rateLimits field can be omitted from responses to reduce their size.

Optional returnRateLimits boolean parameter in request.

Use returnRateLimits parameter to control whether to include rateLimits fields in response to individual requests.

Default request and response:

Request and response without rate limit status:

Optional returnRateLimits boolean parameter in connection URL.

If you wish to omit rateLimits from all responses by default, use returnRateLimits parameter in the query string instead:

This will make all requests made through this connection behave as if you have passed "returnRateLimits": false.

If you want to see rate limits for a particular request, you need to explicitly pass the "returnRateLimits": true parameter.

Note: Your requests are still rate limited if you hide the rateLimits field in responses.

Successful response indicating that in 1 minute you have used 70 weight out of your 6000 limit:

Failed response indicating that you are banned and the ban will last until epoch 1659146400000:

Successful response indicating that you have placed 12 orders in 10 seconds, and 4043 orders in the past 24 hours:

**Examples:**

Example 1 (unknown):
```unknown
exchangeInfo
```

Example 2 (json):
```json
{  "id": "7069b743-f477-4ae3-81db-db9b8df085d2",  "status": 200,  "result": {    "serverTime": 1656400526260  },  "rateLimits": [    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 70    }  ]}
```

Example 3 (json):
```json
{  "id": "7069b743-f477-4ae3-81db-db9b8df085d2",  "status": 200,  "result": {    "serverTime": 1656400526260  },  "rateLimits": [    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 70    }  ]}
```

Example 4 (unknown):
```unknown
rateLimitType
```

---

## User Data Stream requests

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/websocket-api/user-data-stream-requests

**Contents:**
- User Data Stream requests
  - User Data Stream subscription​
    - Subscribe to User Data Stream (USER_STREAM)​
    - Unsubscribe from User Data Stream​
    - Listing all subscriptions​
    - Subscribe to User Data Stream through signature subscription (USER_STREAM)​

Subscribe to the User Data Stream in the current WebSocket connection.

Stop listening to the User Data Stream in the current WebSocket connection.

Note that session.logout will only close the subscription created with userdataStream.subscribe but not subscriptions opened with userDataStream.subscribe.signature.

**Examples:**

Example 1 (unknown):
```unknown
userDataStream.subscribe
```

Example 2 (unknown):
```unknown
userdataStream.subscribe.signature
```

Example 3 (unknown):
```unknown
subscriptionId
```

Example 4 (unknown):
```unknown
subscriptionId
```

---

## Official Documentation for the Binance APIs and Streams.

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/README

**Contents:**
- Official Documentation for the Binance APIs and Streams.
  - FAQ​
  - Change log​
  - Useful Resources​
  - Contact Us​

Please refer to CHANGELOG for latest changes on our APIs and Streamers.

---

## User Data Stream requests

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/websocket-api/user-data-stream-requests

**Contents:**
- User Data Stream requests
  - User Data Stream subscription​
    - Subscribe to User Data Stream (USER_STREAM)​
    - Unsubscribe from User Data Stream​
    - Listing all subscriptions​
    - Subscribe to User Data Stream through signature subscription (USER_STREAM)​

Subscribe to the User Data Stream in the current WebSocket connection.

Stop listening to the User Data Stream in the current WebSocket connection.

Note that session.logout will only close the subscription created with userdataStream.subscribe but not subscriptions opened with userDataStream.subscribe.signature.

**Examples:**

Example 1 (unknown):
```unknown
userDataStream.subscribe
```

Example 2 (unknown):
```unknown
userdataStream.subscribe.signature
```

Example 3 (unknown):
```unknown
subscriptionId
```

Example 4 (unknown):
```unknown
subscriptionId
```

---

## CHANGELOG for Binance's API

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/CHANGELOG

**Contents:**
- CHANGELOG for Binance's API
  - 2025-10-24​
    - SBE​
    - REST and WebSocket API​
  - 2025-10-21​
  - 2025-10-08​
    - FIX API​
  - 2025-09-29​
  - 2025-09-18​
  - 2025-09-12​

Last Updated: 2025-10-24

Following the announcement from 2025-04-07, all documentation related with listenKey for use on wss://stream.binance.com has been removed.

Please refer to the list of requests and methods below for more information.

The features will remain available until a future retirement announcement is made.

REST and WebSocket API:

Notice: The following changes will be deployed on 2025-09-29, starting at 10:00 UTC and may take several hours to complete.

Notice: The changes in this section will be gradually rolled out, and will take approximately up to two weeks to complete.

The following changes will be available on 2025-08-27 starting at 07:00 UTC:

The following changes will be available on 2025-08-28 starting at 07:00 UTC:

REST and WebSocket API:

Notice: The following changes will happen at 2025-06-06 7:00 UTC.

Clarification on the release of Order Amend Keep Priority and STP Decrement:

Notice: The changes in this section will be gradually rolled out, and will take a week to complete.

Notice: The changes in this section will be gradually rolled out, and will take a week to complete.

Notice: The following changes will occur during April 21, 2025.

The following changes will occur at April 24, 2025, 07:00 UTC:

The system now supports microseconds in all related time and/or timestamp fields. Microsecond support is opt-in, by default the requests and responses still use milliseconds. Examples in documentation are also using milliseconds for the foreseeable future.

Notice: The changes below will be rolled out starting at 2024-12-12 and may take approximately a week to complete.

The following changes will occur between 2024-12-16 to 2024-12-20:

REST and WebSocket API:

Changes to Exchange Information (i.e. GET /api/v3/exchangeInfo from REST and exchangeInfo for WebSocket API).

Notice: The changes below are being rolled out gradually, and may take approximately a week to complete.

This will be available by June 6, 11:59 UTC.

The following changes have been postponed to take effect on April 25, 05:00 UTC

Notice: The changes below are being rolled out gradually, and will take approximately a week to complete.

The following will take effect approximately a week after the release date:

This will take effect on March 5, 2024.

Simple Binary Encoding (SBE) will be added to the live exchange, both for the Rest API and WebSocket API.

For more information on SBE, please refer to the FAQ

The SPOT WebSocket API can now support SBE on SPOT Testnet.

The SBE schema has been updated with WebSocket API metadata without incrementing either schemaId or version.

Users using SBE only on the REST API may continue to use the SBE schema with git commit hash 128b94b2591944a536ae427626b795000100cf1d or update to the newly-published SBE schema.

Users who want to use SBE on the WebSocket API must use the newly-published SBE schema.

The FAQ for SBE has been updated.

Simple Binary Encoding (SBE) has been added to SPOT Testnet.

This will be added to the live exchange at a later date.

For more information on what SBE is, please refer to the FAQ

Notice: The changes below are being rolled out gradually, and will take approximately a week to complete.

The following will take effect approximately a week after the release date:

Effective on 2023-10-19 00:00 UTC

The following changes will be effective from 2023-08-25 at UTC 00:00.

Please refer to the table for more details:

Smart Order Routing (SOR) has been added to the APIs. For more information please refer to our FAQ. Please wait for future announcements on when the feature will be enabled.

Notice: The change below are being rolled out, and will take approximately a week to complete.

The following changes will take effect approximately a week from the release date::

Notice: The change below are being rolled out, and will take approximately a week to complete.

Notice: All changes are being rolled out gradually to all our servers, and may take a week to complete.

The following changes will take effect approximately a week from the release date, but the rest of the documentation has been updated to reflect the future changes:

Changes to Websocket Limits

The WS-API and Websocket Stream now only allows 300 connections requests every 5 minutes.

This limit is per IP address.

Please be careful when trying to open multiple connections or reconnecting to the Websocket API.

As per the announcement, Self Trade Prevention will be enabled at 2023-01-26 08:00 UTC.

Please refer to GET /api/v3/exchangeInfo from the Rest API or exchangeInfo from the Websocket API on the default and allowed modes.

New API cluster has been added. Note that all endpoints are functionally equal, but may vary in performance.

ACTUAL RELEASE DATE TBD

New Feature: Self-Trade Prevention (aka STP) will be added to the system at a later date. This will prevent orders from matching with orders from the same account, or accounts under the same tradeGroupId.

Please refer to GET /api/v3/exchangeInfo from the Rest API or exchangeInfo from the Websocket API on the status.

Additional details on the functionality of STP is explained in the STP FAQ document.

WEBSOCKET API WILL BE AVAILABLE ON THE LIVE EXCHANGE AT A LATER DATE.

Some error messages on error code -1003 have changed.

Notice: These changes are being rolled out gradually to all our servers, and will take approximately a week to complete.

Fixed a bug where symbol + orderId combination would return all trades even if the number of trades went beyond the 500 default limit.

Previous behavior: The API would send specific error messages depending on the combination of parameters sent. E.g:

New behavior: If the combinations of optional parameters to the endpoint were not supported, then the endpoint will respond with the generic error:

Added a new combination of supported parameters: symbol + orderId + fromId.

The following combinations of parameters were previously supported but no longer accepted, as these combinations were only taking fromId into consideration, ignoring startTime and endTime:

Thus, these are the supported combinations of parameters:

Note: These new fields will appear approximately a week from the release date.

Scheduled changes to the removal of !bookTicker around November 2022.

Note that these are rolling changes, so it may take a few days for it to rollout to all our servers.

Note that these are rolling changes, so it may take a few days for it to rollout to all our servers.

Changes to GET /api/v3/ticker

Note: The update is being rolled out over the next few days, so these changes may not be visible right away.

Changes to Order Book Depth Levels

What does this affect?

Updates to MAX_POSITION

Note: The changes are being rolled out during the next few days, so these will not appear right away.

On April 28, 2021 00:00 UTC the weights to the following endpoints will be adjusted:

New API clusters have been added in order to improve performance.

Users can access any of the following API clusters, in addition to api.binance.com

If there are any performance issues with accessing api.binance.com please try any of the following instead:

This filter defines the allowed maximum position an account can have on the base asset of a symbol. An account's position defined as the sum of the account's:

BUY orders will be rejected if the account's position is greater than the maximum position allowed.

Deprecation of v1 endpoints:

By end of Q1 2020, the following endpoints will be removed from the API. The documentation has been updated to use the v3 versions of these endpoints.

These endpoints however, will NOT be migrated to v3. Please use the following endpoints instead moving forward.

Changes toexecutionReport event

balanceUpdate event type added

In Q4 2017, the following endpoints were deprecated and removed from the API documentation. They have been permanently removed from the API as of this version. We apologize for the omission from the original changelog:

Streams, endpoints, parameters, payloads, etc. described in the documents in this repository are considered official and supported. The use of any other streams, endpoints, parameters, or payloads, etc. is not supported; use them at your own risk and with no guarantees.

New order type: OCO ("One Cancels the Other")

An OCO has 2 orders: (also known as legs in financial terms)

Quantity Restrictions:

recvWindow cannot exceed 60000.

New intervalLetter values for headers:

New Headers X-MBX-USED-WEIGHT-(intervalNum)(intervalLetter) will give your current used request weight for the (intervalNum)(intervalLetter) rate limiter. For example, if there is a one minute request rate weight limiter set, you will get a X-MBX-USED-WEIGHT-1M header in the response. The legacy header X-MBX-USED-WEIGHT will still be returned and will represent the current used weight for the one minute request rate weight limit.

New Header X-MBX-ORDER-COUNT-(intervalNum)(intervalLetter)that is updated on any valid order placement and tracks your current order count for the interval; rejected/unsuccessful orders are not guaranteed to have X-MBX-ORDER-COUNT-** headers in the response.

GET api/v1/depth now supports limit 5000 and 10000; weights are 50 and 100 respectively.

GET api/v1/exchangeInfo has a new parameter ocoAllowed.

(qty * price) of all trades / sum of qty of all trades over previous 5 minutes.

If there is no trade in the last 5 minutes, it takes the first trade that happened outside of the 5min window. For example if the last trade was 20 minutes ago, that trade's price is the 5 min average.

If there is no trade on the symbol, there is no average price and market orders cannot be placed. On a new symbol with applyToMarket enabled on the MIN_NOTIONAL filter, market orders cannot be placed until there is at least 1 trade.

The current average price can be checked here: https://api.binance.com/api/v3/avgPrice?symbol=<symbol> For example: https://api.binance.com/api/v3/avgPrice?symbol=BNBUSDT

**Examples:**

Example 1 (unknown):
```unknown
wss://stream.binance.com
```

Example 2 (unknown):
```unknown
POST /api/v3/userDataStream
```

Example 3 (unknown):
```unknown
PUT /api/v3/userDataStream
```

Example 4 (unknown):
```unknown
DELETE /api/v3/userDataStream
```

---
