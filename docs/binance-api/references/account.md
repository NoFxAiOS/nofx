# Binance-Api - Account

**Pages:** 4

---

## Account requests

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/websocket-api/account-requests

**Contents:**
- Account requests
  - Account information (USER_DATA)​
  - Query order (USER_DATA)​
  - Current open orders (USER_DATA)​
  - Account order history (USER_DATA)​
  - Query Order list (USER_DATA)​
  - Current open Order lists (USER_DATA)​
  - Account order list history (USER_DATA)​
  - Account trade history (USER_DATA)​
  - Unfilled Order Count (USER_DATA)​

Query information about your account.

Data Source: Memory => Database

Check execution status of an order.

If both orderId and origClientOrderId are provided, the orderId is searched first, then the origClientOrderId from that result is checked against that order. If both conditions are not met the request will be rejected.

For some historical orders the cummulativeQuoteQty response field may be negative, meaning the data is not available at this time.

Data Source: Memory => Database

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Query execution status of all open orders.

If you need to continuously monitor order status updates, please consider using WebSocket Streams:

Weight: Adjusted based on the number of requested symbols:

Data Source: Memory => Database

Status reports for open orders are identical to order.status.

Note that some fields are optional and included only for orders that set them.

Open orders are always returned as a flat list. If all symbols are requested, use the symbol field to tell which symbol the orders belong to.

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Query information about all your orders – active, canceled, filled – filtered by time range.

If startTime and/or endTime are specified, orderId is ignored.

Orders are filtered by time of the last execution status update.

If orderId is specified, return orders with order ID >= orderId.

If no condition is specified, the most recent orders are returned.

For some historical orders the cummulativeQuoteQty response field may be negative, meaning the data is not available at this time.

The time between startTime and endTime can't be longer than 24 hours.

Data Source: Database

Status reports for orders are identical to order.status.

Note that some fields are optional and included only for orders that set them.

Check execution status of an Order list.

For execution status of individual orders, use order.status.

origClientOrderId refers to listClientOrderId of the order list itself.

If both origClientOrderId and orderListId parameters are specified, only origClientOrderId is used and orderListId is ignored.

Data Source: Database

Query execution status of all open order lists.

If you need to continuously monitor order status updates, please consider using WebSocket Streams:

Data Source: Database

Query information about all your order lists, filtered by time range.

If startTime and/or endTime are specified, fromId is ignored.

Order lists are filtered by transactionTime of the last order list execution status update.

If fromId is specified, return order lists with order list ID >= fromId.

If no condition is specified, the most recent order lists are returned.

The time between startTime and endTime can't be longer than 24 hours.

Data Source: Database

Status reports for order lists are identical to orderList.status.

Query information about all your trades, filtered by time range.

If fromId is specified, return trades with trade ID >= fromId.

If startTime and/or endTime are specified, trades are filtered by execution time (time).

fromId cannot be used together with startTime and endTime.

If orderId is specified, only trades related to that order are returned.

startTime and endTime cannot be used together with orderId.

If no condition is specified, the most recent trades are returned.

The time between startTime and endTime can't be longer than 24 hours.

Data Source: Memory => Database

Query your current unfilled order count for all intervals.

Displays the list of orders that were expired due to STP.

These are the combinations supported:

Data Source: Database

Retrieves allocations resulting from SOR order placement.

Supported parameter combinations:

Note: The time between startTime and endTime can't be longer than 24 hours.

Data Source: Database

Get current account commission rates.

Data Source: Database

Queries all amendments of a single order.

Data Source: Database

Retrieves the list of filters relevant to an account on a given symbol. This is the only endpoint that shows if an account has MAX_ASSET filters applied to it.

**Examples:**

Example 1 (javascript):
```javascript
{  "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",  "method": "account.status",  "params": {    "apiKey": "vmPUZE6mv9SD5VNHk4HlWFsOr6aKE2zvsw0MuIgwCIPy6utIco14y7Ju91duEh8A",    "signature": "83303b4a136ac1371795f465808367242685a9e3a42b22edb4d977d0696eb45c",    "timestamp": 1660801839480  }}
```

Example 2 (javascript):
```javascript
{  "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",  "method": "account.status",  "params": {    "apiKey": "vmPUZE6mv9SD5VNHk4HlWFsOr6aKE2zvsw0MuIgwCIPy6utIco14y7Ju91duEh8A",    "signature": "83303b4a136ac1371795f465808367242685a9e3a42b22edb4d977d0696eb45c",    "timestamp": 1660801839480  }}
```

Example 3 (unknown):
```unknown
omitZeroBalances
```

Example 4 (javascript):
```javascript
{  "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",  "status": 200,  "result": {    "makerCommission": 15,    "takerCommission": 15,    "buyerCommission": 0,    "sellerCommission": 0,    "canTrade": true,    "canWithdraw": true,    "canDeposit": true,    "commissionRates": {      "maker": "0.00150000",      "taker": "0.00150000",      "buyer": "0.00000000",      "seller": "0.00000000"    },    "brokered": false,    "requireSelfTradePrevention": false,    "preventSor": false,    "updateTime": 1660801833000,    "accountType": "SPOT",    "balances": [      {        "asset": "BNB",        "free": "0.00000000",        "locked": "0.00000000"      },      {        "asset": "BTC",        "free": "1.3447112",        "locked": "0.08600000"      },      {        "asset": "USDT",        "free": "1021.21000000",        "locked": "0.00000000"      }    ],    "permissions": [      "SPOT"    ],    "uid": 354937868  },  "rateLimits": [    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 20    }  ]}
```

---

## Account requests

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/websocket-api/account-requests

**Contents:**
- Account requests
  - Account information (USER_DATA)​
  - Query order (USER_DATA)​
  - Current open orders (USER_DATA)​
  - Account order history (USER_DATA)​
  - Query Order list (USER_DATA)​
  - Current open order lists (USER_DATA)​
  - Account order list history (USER_DATA)​
  - Account trade history (USER_DATA)​
  - Account unfilled order count (USER_DATA)​

Query information about your account.

Data Source: Memory => Database

Check execution status of an order.

If both orderId and origClientOrderId are provided, the orderId is searched first, then the origClientOrderId from that result is checked against that order. If both conditions are not met the request will be rejected.

For some historical orders the cummulativeQuoteQty response field may be negative, meaning the data is not available at this time.

Data Source: Memory => Database

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Query execution status of all open orders.

If you need to continuously monitor order status updates, please consider using WebSocket Streams:

Weight: Adjusted based on the number of requested symbols:

Data Source: Memory => Database

Status reports for open orders are identical to order.status.

Note that some fields are optional and included only for orders that set them.

Open orders are always returned as a flat list. If all symbols are requested, use the symbol field to tell which symbol the orders belong to.

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Query information about all your orders – active, canceled, filled – filtered by time range.

If startTime and/or endTime are specified, orderId is ignored.

Orders are filtered by time of the last execution status update.

If orderId is specified, return orders with order ID >= orderId.

If no condition is specified, the most recent orders are returned.

For some historical orders the cummulativeQuoteQty response field may be negative, meaning the data is not available at this time.

The time between startTime and endTime can't be longer than 24 hours.

Data Source: Database

Status reports for orders are identical to order.status.

Note that some fields are optional and included only for orders that set them.

Check execution status of an Order list.

For execution status of individual orders, use order.status.

origClientOrderId refers to listClientOrderId of the order list itself.

If both origClientOrderId and orderListId parameters are specified, only origClientOrderId is used and orderListId is ignored.

Data Source: Database

Query execution status of all open order lists.

If you need to continuously monitor order status updates, please consider using WebSocket Streams:

Data Source: Database

Query information about all your order lists, filtered by time range.

If startTime and/or endTime are specified, fromId is ignored.

Order lists are filtered by transactionTime of the last order list execution status update.

If fromId is specified, return order lists with order list ID >= fromId.

If no condition is specified, the most recent order lists are returned.

The time between startTime and endTime can't be longer than 24 hours.

Data Source: Database

Status reports for order lists are identical to orderList.status.

Query information about all your trades, filtered by time range.

If fromId is specified, return trades with trade ID >= fromId.

If startTime and/or endTime are specified, trades are filtered by execution time (time).

fromId cannot be used together with startTime and endTime.

If orderId is specified, only trades related to that order are returned.

startTime and endTime cannot be used together with orderId.

If no condition is specified, the most recent trades are returned.

The time between startTime and endTime can't be longer than 24 hours.

Data Source: Memory => Database

Query your current unfilled order count for all intervals.

Displays the list of orders that were expired due to STP.

These are the combinations supported:

Retrieves allocations resulting from SOR order placement.

Supported parameter combinations:

Note: The time between startTime and endTime can't be longer than 24 hours.

Data Source: Database

Get current account commission rates.

Data Source: Database

Queries all amendments of a single order.

Data Source: Database

Retrieves the list of filters relevant to an account on a given symbol. This is the only endpoint that shows if an account has MAX_ASSET filters applied to it.

**Examples:**

Example 1 (javascript):
```javascript
{  "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",  "method": "account.status",  "params": {    "apiKey": "vmPUZE6mv9SD5VNHk4HlWFsOr6aKE2zvsw0MuIgwCIPy6utIco14y7Ju91duEh8A",    "signature": "83303b4a136ac1371795f465808367242685a9e3a42b22edb4d977d0696eb45c",    "timestamp": 1660801839480  }}
```

Example 2 (javascript):
```javascript
{  "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",  "method": "account.status",  "params": {    "apiKey": "vmPUZE6mv9SD5VNHk4HlWFsOr6aKE2zvsw0MuIgwCIPy6utIco14y7Ju91duEh8A",    "signature": "83303b4a136ac1371795f465808367242685a9e3a42b22edb4d977d0696eb45c",    "timestamp": 1660801839480  }}
```

Example 3 (unknown):
```unknown
omitZeroBalances
```

Example 4 (javascript):
```javascript
{  "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",  "status": 200,  "result": {    "makerCommission": 15,    "takerCommission": 15,    "buyerCommission": 0,    "sellerCommission": 0,    "canTrade": true,    "canWithdraw": true,    "canDeposit": true,    "commissionRates": {      "maker": "0.00150000",      "taker": "0.00150000",      "buyer": "0.00000000",      "seller": "0.00000000"    },    "brokered": false,    "requireSelfTradePrevention": false,    "preventSor": false,    "updateTime": 1660801833000,    "accountType": "SPOT",    "balances": [      {        "asset": "BNB",        "free": "0.00000000",        "locked": "0.00000000"      },      {        "asset": "BTC",        "free": "1.3447112",        "locked": "0.08600000"      },      {        "asset": "USDT",        "free": "1021.21000000",        "locked": "0.00000000"      }    ],    "permissions": [      "SPOT"    ],    "uid": 354937868  },  "rateLimits": [    {      "rateLimitType": "REQUEST_WEIGHT",      "interval": "MINUTE",      "intervalNum": 1,      "limit": 6000,      "count": 20    }  ]}
```

---

## Account Endpoints

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/rest-api/account-endpoints

**Contents:**
- Account Endpoints
  - Account information (USER_DATA)​
  - Query order (USER_DATA)​
  - Current open orders (USER_DATA)​
  - All orders (USER_DATA)​
  - Query Order list (USER_DATA)​
  - Query all Order lists (USER_DATA)​
  - Query Open Order lists (USER_DATA)​
  - Account trade list (USER_DATA)​
  - Query Unfilled Order Count (USER_DATA)​

Get current account information.

Data Source: Memory => Database

Check an order's status.

Data Source: Memory => Database

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Get all open orders on a symbol. Careful when accessing this with no symbol.

Weight: 6 for a single symbol; 80 when the symbol parameter is omitted

Data Source: Memory => Database

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Get all account orders; active, canceled, or filled.

Data Source: Database

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Retrieves a specific order list based on provided optional parameters.

Data Source: Database

Retrieves all order lists based on provided optional parameters.

Note that the time between startTime and endTime can't be longer than 24 hours.

Data Source: Database

Data Source: Database

Get trades for a specific account and symbol.

Data Source: Memory => Database

Displays the user's unfilled order count for all intervals.

Displays the list of orders that were expired due to STP.

These are the combinations supported:

Retrieves allocations resulting from SOR order placement.

Supported parameter combinations:

Note: The time between startTime and endTime can't be longer than 24 hours.

Data Source: Database

Get current account commission rates.

Data Source: Database

Queries all amendments of a single order.

Retrieves the list of filters relevant to an account on a given symbol. This is the only endpoint that shows if an account has MAX_ASSET filters applied to it.

**Examples:**

Example 1 (text):
```text
GET /api/v3/account
```

Example 2 (text):
```text
GET /api/v3/account
```

Example 3 (javascript):
```javascript
{  "makerCommission": 15,  "takerCommission": 15,  "buyerCommission": 0,  "sellerCommission": 0,  "commissionRates": {    "maker": "0.00150000",    "taker": "0.00150000",    "buyer": "0.00000000",    "seller": "0.00000000"  },  "canTrade": true,  "canWithdraw": true,  "canDeposit": true,  "brokered": false,  "requireSelfTradePrevention": false,  "preventSor": false,  "updateTime": 123456789,  "accountType": "SPOT",  "balances": [    {      "asset": "BTC",      "free": "4723846.89208129",      "locked": "0.00000000"    },    {      "asset": "LTC",      "free": "4763368.68006011",      "locked": "0.00000000"    }  ],  "permissions": [    "SPOT"  ],  "uid": 354937868}
```

Example 4 (javascript):
```javascript
{  "makerCommission": 15,  "takerCommission": 15,  "buyerCommission": 0,  "sellerCommission": 0,  "commissionRates": {    "maker": "0.00150000",    "taker": "0.00150000",    "buyer": "0.00000000",    "seller": "0.00000000"  },  "canTrade": true,  "canWithdraw": true,  "canDeposit": true,  "brokered": false,  "requireSelfTradePrevention": false,  "preventSor": false,  "updateTime": 123456789,  "accountType": "SPOT",  "balances": [    {      "asset": "BTC",      "free": "4723846.89208129",      "locked": "0.00000000"    },    {      "asset": "LTC",      "free": "4763368.68006011",      "locked": "0.00000000"    }  ],  "permissions": [    "SPOT"  ],  "uid": 354937868}
```

---

## Account Endpoints

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/rest-api/account-endpoints

**Contents:**
- Account Endpoints
  - Account information (USER_DATA)​
  - Current open orders (USER_DATA)​
  - All orders (USER_DATA)​
  - Query Order list (USER_DATA)​
  - Query all Order lists (USER_DATA)​
  - Query Open Order lists (USER_DATA)​
  - Account trade list (USER_DATA)​
  - Query Unfilled Order Count (USER_DATA)​
  - Query Prevented Matches (USER_DATA)​

Get current account information.

Data Source: Memory => Database

Get all open orders on a symbol. Careful when accessing this with no symbol.

Weight: 6 for a single symbol; 80 when the symbol parameter is omitted

Data Source: Memory => Database

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Get all account orders; active, canceled, or filled.

Data Source: Database

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Retrieves a specific order list based on provided optional parameters.

Data Source: Database

Retrieves all order lists based on provided optional parameters

Note that the time between startTime and endTime can't be longer than 24 hours.

Data Source: Database

Data Source: Database

Get trades for a specific account and symbol.

Data Source: Memory => Database

Displays the user's unfilled order count for all intervals.

Displays the list of orders that were expired due to STP.

These are the combinations supported:

Retrieves allocations resulting from SOR order placement.

Supported parameter combinations:

Note: The time between startTime and endTime can't be longer than 24 hours.

Data Source: Database

Get current account commission rates.

Data Source: Database

Queries all amendments of a single order.

Retrieves the list of filters relevant to an account on a given symbol. This is the only endpoint that shows if an account has MAX_ASSET filters applied to it.

**Examples:**

Example 1 (text):
```text
GET /api/v3/account
```

Example 2 (text):
```text
GET /api/v3/account
```

Example 3 (javascript):
```javascript
{  "makerCommission": 15,  "takerCommission": 15,  "buyerCommission": 0,  "sellerCommission": 0,  "commissionRates": {    "maker": "0.00150000",    "taker": "0.00150000",    "buyer": "0.00000000",    "seller": "0.00000000"  },  "canTrade": true,  "canWithdraw": true,  "canDeposit": true,  "brokered": false,  "requireSelfTradePrevention": false,  "preventSor": false,  "updateTime": 123456789,  "accountType": "SPOT",  "balances": [    {      "asset": "BTC",      "free": "4723846.89208129",      "locked": "0.00000000"    },    {      "asset": "LTC",      "free": "4763368.68006011",      "locked": "0.00000000"    }  ],  "permissions": [    "SPOT"  ],  "uid": 354937868}
```

Example 4 (javascript):
```javascript
{  "makerCommission": 15,  "takerCommission": 15,  "buyerCommission": 0,  "sellerCommission": 0,  "commissionRates": {    "maker": "0.00150000",    "taker": "0.00150000",    "buyer": "0.00000000",    "seller": "0.00000000"  },  "canTrade": true,  "canWithdraw": true,  "canDeposit": true,  "brokered": false,  "requireSelfTradePrevention": false,  "preventSor": false,  "updateTime": 123456789,  "accountType": "SPOT",  "balances": [    {      "asset": "BTC",      "free": "4723846.89208129",      "locked": "0.00000000"    },    {      "asset": "LTC",      "free": "4763368.68006011",      "locked": "0.00000000"    }  ],  "permissions": [    "SPOT"  ],  "uid": 354937868}
```

---
