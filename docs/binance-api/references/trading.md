# Binance-Api - Trading

**Pages:** 18

---

## Smart Order Routing (SOR)

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/faqs/sor_faq

**Contents:**
- Smart Order Routing (SOR)
  - What is Smart Order Routing (SOR)?​
  - What symbols support SOR?​
  - How do I place an order using SOR?​
  - In the API response, there's a field called workingFloor. What does that field mean?​
  - In the API response, fills contain fields matchType and allocId. What do they mean?​
  - What are allocations?​
  - How do I query orders that used SOR?​
  - How do I get details of my fills for orders that used SOR?​

Smart Order Routing (SOR) allows you to potentially get better liquidity by filling an order with liquidity from other order books with the same base asset and interchangeable quote assets. Interchangeable quote assets are quote assets with fixed 1 to 1 exchange rate, such as stablecoins pegged to the same fiat currency.

Note that even though the quote assets are interchangeable, when selling the base asset you will always receive the quote asset of the symbol in your order.

When you place an order using SOR, it goes through the eligible order books, looks for best price levels for each order book in that SOR configuration, and takes from those books if possible.

Note: If the order using SOR cannot fully fill based on the eligible order books' liquidity, LIMIT IOC or MARKET orders will immediately expire, while LIMIT GTC orders will place the remaining quantity on the order book you originally submitted the order to.

Let's consider a SOR configuration containing the symbols BTCUSDT, BTCUSDC and BTCUSDP, and the following ASK (SELL side) order books for those symbols:

If you send a LIMIT GTC BUY order for BTCUSDT with quantity=0.5 and price=31000, you would match with the best SELL price on the BTCUSDT book at 30,500. You would spend 15,250 USDT and receive 0.5 BTC.

If you send a LIMIT GTC BUY order using SOR for BTCUSDT with quantity=0.5 and price=31000, you would match with the best SELL price across all symbols in the SOR, which is BTCUSDC at price 28,000. You would spend 14,000 USDT (not USDC!) and receive 0.5 BTC.

Using the same order book as Example 1:

If you send a LIMIT GTC BUY order for BTCUSDT with quantity=5 and price=31000, you would:

In total, you spend 153,100 USDT and receive 5 BTC.

If you send the same LIMIT GTC BUY order using SOR for BTCUSDT with quantity=5 and price=31000, you would:

In total, you spend 148,000 USDT and receive 5 BTC.

Using the same order book as Example 1 and 2:

If you send a MARKET BUY order for BTCUSDT using SOR with quantity=11, there is only 10 BTC in total available across all eligible order books. Once all the order books in SOR configuration have been exhausted, the remaining quantity of 1 expires.

Let's consider a SOR configuration containing the symbols BTCUSDT, BTCUSDC and BTCUSDP and the following BID (BUY side) order book for those symbols:

If you send a LIMIT GTC SELL order for BTCUSDT with price=29000 and quantity=10, you would sell 5 BTC and receive 147,500 USDT. Since there is no better price available on the BTCUSDT book, the remaining (unfilled) quantity of the order will rest there at the price of 29,000.

If you send a LIMIT GTC SELL order using SOR for BTCUSDT, you would:

In total, you sell 10 BTC and receive 325,000 USDT.

Summary: The goal of SOR is to potentially access better liquidity across order books with interchangeable quote assets. Better liquidity access can fill orders more fully and at better prices during an order's taker phase.

You can find the current SOR configuration in Exchange Information (GET /api/v3/exchangeInfo for Rest, and exchangeInfo on Websocket API).

The sors field is optional. It is omitted in responses if SOR is not available.

On the Rest API, the request is POST /api/v3/sor/order.

On the WebSocket API, the request is sor.order.place.

This is a term used to determine where the order's last activity occurred (filling, expiring, or being placed as new, etc.).

If the workingFloor is SOR, this means your order interacted with other eligible order books in the SOR configuration.

If the workingFloor is EXCHANGE, this means your order interacted on the order book that you sent that order to.

matchType field indicates a non-standard order fill.

When your order is filled by SOR, you will see matchType: ONE_PARTY_TRADE_REPORT, indicating that you did not trade directly on the exchange (tradeId: -1). Instead your order is filled by allocations.

allocId field identifies the allocation so that you can query it later.

An allocation is a transfer of an asset from the exchange to your account. For example, when SOR takes liquidity from eligible order books, your order is filled by allocations. In this case you don't trade directly, but rather receive allocations from SOR corresponding to the trades made by SOR on your behalf.

You can find them the same way you query any other order. The main difference is that in the response for an order that used SOR there are two extra fields: usedSor and workingFloor.

When SOR orders trade against order books other than the symbol submitted with the order, the order is filled with an allocation and not a trade. Orders placed with SOR can potentially have both allocations and trades.

In the API response, you can review the fills fields. Allocations have an allocId and "matchType": "ONE_PARTY_TRADE_REPORT", while trades will have a non-negative tradeId.

Allocations can be queried using GET /api/v3/myAllocations (Rest API) or myAllocations (WebSocket API).

Trades can be queried using GET /api/v3/myTrades (Rest API) or myTrades (WebSocket API).

**Examples:**

Example 1 (text):
```text
BTCUSDT quantity 3 price 30,800BTCUSDT quantity 3 price 30,500BTCUSDC quantity 1 price 30,000BTCUSDC quantity 1 price 28,000BTCUSDP quantity 1 price 35,000BTCUSDP quantity 1 price 29,000
```

Example 2 (text):
```text
BTCUSDT quantity 3 price 30,800BTCUSDT quantity 3 price 30,500BTCUSDC quantity 1 price 30,000BTCUSDC quantity 1 price 28,000BTCUSDP quantity 1 price 35,000BTCUSDP quantity 1 price 29,000
```

Example 3 (unknown):
```unknown
LIMIT GTC BUY
```

Example 4 (unknown):
```unknown
quantity=0.5
```

---

## Filters

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/filters

**Contents:**
- Filters
- Symbol filters​
  - PRICE_FILTER​
  - PERCENT_PRICE​
  - PERCENT_PRICE_BY_SIDE​
  - LOT_SIZE​
  - MIN_NOTIONAL​
  - NOTIONAL​
  - ICEBERG_PARTS​
  - MARKET_LOT_SIZE​

Filters define trading rules on a symbol or an exchange. Filters come in three forms: symbol filters, exchange filters and asset filters.

The PRICE_FILTER defines the price rules for a symbol. There are 3 parts:

Any of the above variables can be set to 0, which disables that rule in the price filter. In order to pass the price filter, the following must be true for price/stopPrice of the enabled rules:

/exchangeInfo format:

The PERCENT_PRICE filter defines the valid range for the price based on the average of the previous trades. avgPriceMins is the number of minutes the average price is calculated over. 0 means the last price is used.

In order to pass the percent price, the following must be true for price:

/exchangeInfo format:

The PERCENT_PRICE_BY_SIDE filter defines the valid range for the price based on the average of the previous trades. avgPriceMins is the number of minutes the average price is calculated over. 0 means the last price is used. There is a different range depending on whether the order is placed on the BUY side or the SELL side.

Buy orders will succeed on this filter if:

Sell orders will succeed on this filter if:

/exchangeInfo format:

The LOT_SIZE filter defines the quantity (aka "lots" in auction terms) rules for a symbol. There are 3 parts:

In order to pass the lot size, the following must be true for quantity/icebergQty:

/exchangeInfo format:

The MIN_NOTIONAL filter defines the minimum notional value allowed for an order on a symbol. An order's notional value is the price * quantity. applyToMarket determines whether or not the MIN_NOTIONAL filter will also be applied to MARKET orders. Since MARKET orders have no price, the average price is used over the last avgPriceMins minutes. avgPriceMins is the number of minutes the average price is calculated over. 0 means the last price is used.

/exchangeInfo format:

The NOTIONAL filter defines the acceptable notional range allowed for an order on a symbol. applyMinToMarket determines whether the minNotional will be applied to MARKET orders. applyMaxToMarket determines whether the maxNotional will be applied to MARKET orders.

In order to pass this filter, the notional (price * quantity) has to pass the following conditions:

For MARKET orders, the average price used over the last avgPriceMins minutes will be used for calculation. If the avgPriceMins is 0, then the last price will be used.

/exchangeInfo format:

The ICEBERG_PARTS filter defines the maximum parts an iceberg order can have. The number of ICEBERG_PARTS is defined as CEIL(qty / icebergQty).

/exchangeInfo format:

The MARKET_LOT_SIZE filter defines the quantity (aka "lots" in auction terms) rules for MARKET orders on a symbol. There are 3 parts:

In order to pass the market lot size, the following must be true for quantity:

/exchangeInfo format:

The MAX_NUM_ORDERS filter defines the maximum number of orders an account is allowed to have open on a symbol. Note that both "algo" orders and normal orders are counted for this filter.

/exchangeInfo format:

The MAX_NUM_ALGO_ORDERS filter defines the maximum number of "algo" orders an account is allowed to have open on a symbol. "Algo" orders are STOP_LOSS, STOP_LOSS_LIMIT, TAKE_PROFIT, and TAKE_PROFIT_LIMIT orders.

/exchangeInfo format:

The MAX_NUM_ICEBERG_ORDERS filter defines the maximum number of ICEBERG orders an account is allowed to have open on a symbol. An ICEBERG order is any order where the icebergQty is > 0.

/exchangeInfo format:

The MAX_POSITION filter defines the allowed maximum position an account can have on the base asset of a symbol. An account's position defined as the sum of the account's:

BUY orders will be rejected if the account's position is greater than the maximum position allowed.

If an order's quantity can cause the position to overflow, this will also fail the MAX_POSITION filter.

/exchangeInfo format:

The TRAILING_DELTA filter defines the minimum and maximum value for the parameter trailingDelta.

In order for a trailing stop order to pass this filter, the following must be true:

For STOP_LOSS BUY, STOP_LOSS_LIMIT_BUY,TAKE_PROFIT SELL and TAKE_PROFIT_LIMIT SELL orders:

For STOP_LOSS SELL, STOP_LOSS_LIMIT SELL, TAKE_PROFIT BUY, and TAKE_PROFIT_LIMIT BUY orders:

/exchangeInfo format:

The MAX_NUM_ORDER_AMENDS filter defines the maximum number of times an order can be amended on the given symbol.

If there are too many order amendments made on a single order, you will receive the -2038 error code.

/exchangeInfo format:

The MAX_NUM_ORDER_LISTS filter defines the maximum number of open order lists an account can have on a symbol. Note that OTOCOs count as one order list.

/exchangeInfo format:

The EXCHANGE_MAX_NUM_ORDERS filter defines the maximum number of orders an account is allowed to have open on the exchange. Note that both "algo" orders and normal orders are counted for this filter.

/exchangeInfo format:

The EXCHANGE_MAX_NUM_ALGO_ORDERS filter defines the maximum number of "algo" orders an account is allowed to have open on the exchange. "Algo" orders are STOP_LOSS, STOP_LOSS_LIMIT, TAKE_PROFIT, and TAKE_PROFIT_LIMIT orders.

/exchangeInfo format:

The EXCHANGE_MAX_NUM_ICEBERG_ORDERS filter defines the maximum number of iceberg orders an account is allowed to have open on the exchange.

/exchangeInfo format:

The EXCHANGE_MAX_NUM_ORDERS filter defines the maximum number of order lists an account is allowed to have open on the exchange. Note that OTOCOs count as one order list.

/exchangeInfo format:

The MAX_ASSET filter defines the maximum quantity of an asset that an account is allowed to transact in a single order.

**Examples:**

Example 1 (unknown):
```unknown
symbol filters
```

Example 2 (unknown):
```unknown
exchange filters
```

Example 3 (unknown):
```unknown
asset filters
```

Example 4 (unknown):
```unknown
PRICE_FILTER
```

---

## Order Amend Keep Priority

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/faqs/order_amend_keep_priority

**Contents:**
- Order Amend Keep Priority
- What is Order Amend Keep Priority?​
- How can I amend the quantity of my order?​
- What is the difference between "Cancel an Existing Order and Send a New Order" (cancel-replace) and "Order Amend Keep Priority"?​
- Does Order Amend Keep Priority affect unfilled order count (rate limits)?​
- How do I know if my order has been amended?​
- What happens if my amend request does not succeed?​
- Is it possible to reuse the current clientOrderId for my amended order?​
- Can Iceberg Orders be amended?​
- Can Order lists be amended?​

Order Amend Keep Priority request is used to modify (amend) an existing order without losing order book priority.

The following order modifications are allowed:

Use the following requests:

Cancel an Existing Order and Send a New Order request cancels the old order and places a new order. Time priority is lost. The new order executes after existing orders at the same price.

Order Amend Keep Priority request modifies an existing order in-place. The amended order keeps its time priority among existing orders at the same price.

For example, consider the following order book:

Your order 15 is the second one in the queue based on price and time.

You want to reduce the quantity from 5.50 down to 5.00.

If you use cancel-replace to cancel orderId=15 and place a new order with qty=5.00, the order book will look like this:

Note that the new order gets a new order ID and you lose time priority: order 22 will trade after the order 20.

If instead you use Order Amend Keep Priority to reduce the quantity of orderId=15 down to qty=5.00, the order book will look like this:

Note that the order ID stays the same and the order keeps its priority in the queue. Only the quantity of the order changes.

Currently, Order Amend Keep Priority requests charge 0 for unfilled order count.

If the order was amended successfully, the API response contains your order with the updated quantity.

On User Data Stream, you will receive an "executionReport" event with execution type "x": "REPLACED".

If the amended order belongs to an order list and the client order ID has changed, you will also receive a "listStatus" event with list status type "l": "UPDATED".

You can also use the following requests to query order modification history:

If the request fails for any reason (e.g. fails the filters, permissions, account restrictions, etc), then the order amend request is rejected and the order remains unchanged.

By default, amended orders get a random new client order ID, but you can pass the current client order ID in the newClientOrderId parameter if you wish to keep it.

Note that an iceberg order's visible quantity will only change if newQty is below the pre-amended visible quantity.

Orders in an order list can be amended.

Note that OCO order pairs must have the same quantity, since only one of the orders can ever be executed. This means that amending either order affects both orders.

For OTO orders, the working and pending orders can be amended individually.

This information is available in Exchange Information. Symbols that allow Order Amend Keep Priority requests have amendAllowed set to true.

**Examples:**

Example 1 (unknown):
```unknown
PUT /api/v3/order/amend/keepPriority
```

Example 2 (unknown):
```unknown
order.amend.keepPriority
```

Example 3 (unknown):
```unknown
"executionReport"
```

Example 4 (unknown):
```unknown
"x": "REPLACED"
```

---

## Trading endpoints

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/rest-api/trading-endpoints

**Contents:**
- Trading endpoints
  - New order (TRADE)​
  - Test new order (TRADE)​
  - Cancel order (TRADE)​
  - Cancel All Open Orders on a Symbol (TRADE)​
  - Cancel an Existing Order and Send a New Order (TRADE)​
  - Order Amend Keep Priority (TRADE)​
  - Order lists​
    - New OCO - Deprecated (TRADE)​
    - New Order list - OCO (TRADE)​

This adds 1 order to the EXCHANGE_MAX_ORDERS filter and the MAX_NUM_ORDERS filter.

Unfilled Order Count: 1

Some additional mandatory parameters based on order type:

Notes on using parameters for Pegged Orders:

Any LIMIT or LIMIT_MAKER type order can be made an iceberg order by sending an icebergQty.

Any order with an icebergQty MUST have timeInForce set to GTC.

For STOP_LOSS, STOP_LOSS_LIMIT, TAKE_PROFIT_LIMIT and TAKE_PROFIT orders, trailingDelta can be combined with stopPrice.

MARKET orders using quoteOrderQty will not break LOT_SIZE filter rules; the order will execute a quantity that will have the notional value as close as possible to quoteOrderQty. Trigger order price rules against market price for both MARKET and LIMIT versions:

Price above market price: STOP_LOSS BUY, TAKE_PROFIT SELL

Price below market price: STOP_LOSS SELL, TAKE_PROFIT BUY

Data Source: Matching Engine

Conditional fields in Order Responses

There are fields in the order responses (e.g. order placement, order query, order cancellation) that appear only if certain conditions are met.

These fields can apply to order lists.

The fields are listed below:

Test new order creation and signature/recvWindow long. Creates and validates a new order but does not send it into the matching engine.

In addition to all parameters accepted by POST /api/v3/order, the following optional parameters are also accepted:

Without computeCommissionRates

With computeCommissionRates

Cancel an active order.

Data Source: Matching Engine

Regarding cancelRestrictions

Cancels all active orders on a symbol. This includes orders that are part of an order list.

Data Source: Matching Engine

Cancels an existing order and places a new order on the same symbol.

Filters and Order Count are evaluated before the processing of the cancellation and order placement occurs.

A new order that was not attempted (i.e. when newOrderResult: NOT_ATTEMPTED), will still increase the unfilled order count by 1.

Unfilled Order Count: 1

Similar to POST /api/v3/order, additional mandatory parameters are determined by type.

Response format varies depending on whether the processing of the message succeeded, partially succeeded, or failed.

Data Source: Matching Engine

Response SUCCESS and account has not exceeded the unfilled order count:

Response when Cancel Order Fails with STOP_ON FAILURE and account has not exceeded their unfilled order count:

Response when Cancel Order Succeeds but New Order Placement Fails and account has not exceeded their unfilled order count:

Response when Cancel Order fails with ALLOW_FAILURE and account has not exceeded their unfilled order count:

Response when both Cancel Order and New Order Placement fail using cancelReplaceMode=ALLOW_FAILURE and account has not exceeded their unfilled order count:

Response when using orderRateLimitExceededMode=DO_NOTHING and account's unfilled order count has been exceeded:

Response when using orderRateLimitExceededMode=CANCEL_ONLY and account's unfilled order count has been exceeded:

Reduce the quantity of an existing open order.

This adds 0 orders to the EXCHANGE_MAX_ORDERS filter and the MAX_NUM_ORDERS filter.

Read Order Amend Keep Priority FAQ to learn more.

Unfilled Order Count: 0

Data Source: Matching Engine

Response: Response for a single order:

Response for an order that is part of an Order list:

Note: The payloads above do not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Unfilled Order Count: 2

Data Source: Matching Engine

Send in an one-cancels-the-other (OCO) pair, where activation of one order immediately cancels the other.

Unfilled Order Count: 2

Data Source: Matching Engine

Response format for orderReports is selected using the newOrderRespType parameter. The following example is for the RESULT response type. See POST /api/v3/order for more examples.

Unfilled Order Count: 2

Mandatory parameters based on pendingType or workingType

Depending on the pendingType or workingType, some optional parameters will become mandatory.

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Unfilled Order Count: 3

Mandatory parameters based on pendingAboveType, pendingBelowType or workingType

Depending on the pendingAboveType/pendingBelowType or workingType, some optional parameters will become mandatory.

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Cancel an entire Order list

Data Source: Matching Engine

Places an order using smart order routing (SOR).

This adds 1 order to the EXCHANGE_MAX_ORDERS filter and the MAX_NUM_ORDERS filter.

Read SOR FAQ to learn more.

Unfilled Order Count: 1

Note: POST /api/v3/sor/order only supports LIMIT and MARKET orders. quoteOrderQty is not supported.

Data Source: Matching Engine

Test new order creation and signature/recvWindow using smart order routing (SOR). Creates and validates a new order but does not send it into the matching engine.

In addition to all parameters accepted by POST /api/v3/sor/order, the following optional parameters are also accepted:

Without computeCommissionRates

With computeCommissionRates

**Examples:**

Example 1 (text):
```text
POST /api/v3/order
```

Example 2 (text):
```text
POST /api/v3/order
```

Example 3 (unknown):
```unknown
EXCHANGE_MAX_ORDERS
```

Example 4 (unknown):
```unknown
MAX_NUM_ORDERS
```

---

## Spot Trailing Stop order FAQ

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/faqs/trailing-stop-faq

**Contents:**
- Spot Trailing Stop order FAQ
  - What is a trailing stop order?​
  - What are BIPs?​
  - What order types can be trailing stop orders?​
  - How do I place a trailing stop order?​
  - What kind of price changes will trigger my trailing stop order?​
  - How do I pass the TRAILING_DELTA filter?​
  - Trailing Stop Order Scenarios​
    - Scenario A - Trailing Stop Loss Limit Buy Order​
    - Scenario B - Trailing Stop Loss Limit Sell Order​

Trailing stop is a type of contingent order with a dynamic trigger price influenced by price changes in the market. For the SPOT API, the change required to trigger order entry is specified in the trailingDelta parameter, and is defined in BIPS.

Intuitively, trailing stop orders allow unlimited price movement in a direction that is beneficial for the order, and limited movement in a detrimental direction.

Buy orders: low prices are good. Unlimited price decreases are allowed but the order will trigger after a price increase of the supplied delta, relative to the lowest trade price since submission.

Sell orders: high prices are good. Unlimited price increases are allowed but the order will trigger after a price decrease of the supplied delta, relative to the highest trade price since submission.

Basis Points, also known as BIP or BIPS, are used to indicate a percentage change.

BIPS conversion reference:

For example, a STOP_LOSS SELL order with a trailingDelta of 100 is a trailing stop order which will be triggered after a price decrease of 1% from the highest price after placing the order.

Trailing stop orders are supported for contingent orders such as STOP_LOSS, STOP_LOSS_LIMIT, TAKE_PROFIT, and TAKE_PROFIT_LIMIT.

OCO orders also support trailing stop orders in the contingent leg. In this scenario if the trailing stop condition is triggered, the limit leg of the OCO order will be canceled.

Trailing stop orders are entered the same way as regular STOP_LOSS, STOP_LOSS_LIMIT, TAKE_PROFIT, or TAKE_PROFIT_LIMIT orders, but with an additional trailingDelta parameter. This parameter must be within the range of the TRAILING_DELTA filter for that symbol.

Unlike regular contingent orders, the stopPrice parameter is optional for trailing stop orders. If it is provided then the order will only start tracking price changes after the stopPrice condition is met. If the stopPrice parameter is omitted then the order starts tracking price changes from the next trade.

For STOP_LOSS BUY, STOP_LOSS_LIMIT BUY, TAKE_PROFIT SELL, and TAKE_PROFIT_LIMIT SELL orders:

For STOP_LOSS SELL, STOP_LOSS_LIMIT SELL, TAKE_PROFIT BUY, and TAKE_PROFIT_LIMIT BUY orders:

At 12:01:00 there is a trade at a price of 40,000 and a STOP_LOSS_LIMIT order is placed on the BUY side of the exchange. The order has of a stopPrice of 44,000, a trailingDelta of 500 (5%), and a limit price of 45,000.

Between 12:01:00 and 12:02:00 a series of linear trades lead to a decrease in last price, ending at 37,000. This is a price decrease of 7.5% or 750 BIPS, well exceeding the order's trailingDelta. However since the order has not started price tracking, the price movement is ignored and the order remains contingent.

Between 12:02:00 and 12:03:00 a series of linear trades lead to an increase in last price. When a trade is equal to, or surpasses, the stopPrice the order starts tracking price changes immediately; the first trade that meets this condition sets the "lowest price". In this case, the lowest price is 44,000 and if there is a 500 BIPS increase from 44,000 then the order will trigger. The series of linear trades continue to increase the last price, ending at 45,000.

Between 12:03:00 and 12:04:00 a series of linear trades lead to an increase in last price, ending at 46,000. This is an increase of ~454 BIPS from the order's previously noted lowest price, but it's not large enough to trigger the order.

Between 12:04:00 and 12:05:00 a series of linear trades lead to a decrease in last price, ending at 42,000. This is a decrease from the order's previously noted lowest price. If there is a 500 BIPS increase from 42,000 then the order will trigger.

Between 12:05:00 and 12:05:30 a series of linear trades lead to an increase in last price to 44,100. This trade is equal to, or surpasses, the order's requirement of 500 BIPS, as 44,100 = 42,000 * 1.05. This causes the order to trigger and start working against the order book at its limit price of 45,000.

At 12:01:00 there is a trade at a price of 40,000 and a STOP_LOSS_LIMIT order is placed on the SELL side of the exchange. The order has of a stopPrice of 39,000, a trailingDelta of 1000 (10%), and a limit price of 38,000.

Between 12:01:00 and 12:02:00 a series of linear trades lead to an increase in last price, ending at 41,500.

Between 12:02:00 and 12:03:00 a series of linear trades lead to a decrease in last price. When a trade is equal to, or surpasses, the stopPrice the order starts tracking price changes immediately; the first trade that meets this condition sets the "highest price". In this case, the highest price is 39,000 and if there is a 1000 BIPS decrease from 39,000 then the order will trigger.

Between 12:03:00 and 12:04:00 a series of linear trades lead to a decrease in last price, ending at 37,000. This is a decrease of ~512 BIPS from the order's previously noted highest price, but it's not large enough to trigger the order.

Between 12:04:00 and 12:05:00 a series of linear trades lead to an increase in last price, ending at 41,000. This is an increase from the order's previously noted highest price. If there is a 1000 BIPS decrease from 41,000 then the order will trigger.

Between 12:05:00 and 12:05:30 a series of linear trades lead to a decrease in last price to 36,900. This trade is equal to, or surpasses, the order's requirement of 1000 BIPS, as 36,900 = 41,000 * 0.90. This causes the order to trigger and start working against the order book at its limit price of 38,000.

At 12:01:00 there is a trade at a price of 40,000 and a TAKE_PROFIT_LIMIT order is placed on the BUY side of the exchange. The order has of a stopPrice of 38,000, a trailingDelta of 850 (8.5%), and a limit price of 38,500.

Between 12:01:00 and 12:02:00 a series of linear trades lead to an increase in last price, ending at 42,000.

Between 12:02:00 and 12:03:00 a series of linear trades lead to a decrease in last price. When a trade is equal to, or surpasses, the stopPrice the order starts tracking price changes immediately; the first trade that meets this condition sets the "lowest price". In this case, the lowest price is 38,000 and if there is a 850 BIPS increase from 38,000 then the order will trigger.

The series of linear trades continues to decrease the last price, ending at 37,000. If there is a 850 BIPS increase from 37,000 then the order will trigger.

Between 12:03:00 and 12:04:00 a series of linear trades lead to an increase in last price, ending at 39,000. This is an increase of ~540 BIPS from the order's previously noted lowest price, but it's not large enough to trigger the order.

Between 12:04:00 and 12:05:00 a series of linear trades lead to a decrease in last price, ending at 38,000. It does not surpass the order's previously noted lowest price, resulting in no change to the order's trigger price.

Between 12:05:00 and 12:05:30 a series of linear trades lead to an increase in last price to 40,145. This trade is equal to, or surpasses, the order's requirement of 850 BIPS, as 40,145 = 37,000 * 1.085. This causes the order to trigger and start working against the order book at its limit price of 38,500.

At 12:01:00 there is a trade at a price of 40,000 and a TAKE_PROFIT_LIMIT order is placed on the SELL side of the exchange. The order has of a stopPrice of 42,000, a trailingDelta of 750 (7.5%), and a limit price of 41,000.

Between 12:01:00 and 12:02:00 a series of linear trades lead to an increase in last price, ending at 41,500.

Between 12:02:00 and 12:03:00 a series of linear trades lead to a decrease in last price, ending at 39,000.

Between 12:03:00 and 12:04:00 a series of linear trades lead to an increase in last price. When a trade is equal to, or surpasses, the stopPrice the order starts tracking price changes immediately; the first trade that meets this condition sets the "highest price". In this case, the highest price is 42,000 and if there is a 750 BIPS decrease from 42,000 then the order will trigger.

The series of linear trades continues to increase the last price, ending at 45,000. If there is a 750 BIPS decrease from 45,000 then the order will trigger.

Between 12:04:00 and 12:05:00 a series of linear trades lead to a decrease in last price, ending at 44,000. This is a decrease of ~222 BIPS from the order's previously noted highest price, but it's not large enough to trigger the order.

Between 12:05:00 and 12:06:00 a series of linear trades lead to an increase in last price, ending at 46,500. This is an increase from the order's previously noted highest price. If there is a 750 BIPS decrease from 46,500 then the order will trigger.

Between 12:06:00 and 12:06:50 a series of linear trades lead to a decrease in last price to 43,012.5. This trade is equal to, or surpasses, the order's requirement of 750 BIPS, as 43,012.5 = 46,500 * 0.925. This causes the order to trigger and start working against the order book at its limit price of 41,000.

At 12:01:00 there is a trade at a price of 40,000 and a STOP_LOSS_LIMIT order is placed on the SELL side of the exchange. The order has a trailingDelta of 700 (7%), a limit price of 39,000 and no stopPrice. The order starts tracking price changes once placed. If there is a 700 BIPS decrease from 40,000 then the order will trigger.

Between 12:01:00 and 12:02:00 a series of linear trades lead to an increase in last price, ending at 42,000. This is an increase from the order's previously noted highest price. If there is a 700 BIPS decrease from 42,000 then the order will trigger.

Between 12:02:00 and 12:03:00 a series of linear trades lead to a decrease in last price, ending at 39,500. This is a decrease of ~595 BIPS from the order's previously noted highest price, but it's not large enough to trigger the order.

Between 12:03:00 and 12:04:00 a series of linear trades lead to an increase in last price, ending at 45,500. This is an increase from the order's previously noted highest price. If there is a 700 BIPS decrease from 45,500 then the order will trigger.

Between 12:04:00 and 12:04:45 a series of linear trades lead to a decrease in last price to 42,315. This trade is equal to, or surpasses, the order's requirement of 700 BIPS, as 42,315 = 45,500 * 0.93. This causes the order to trigger and start working against the order book at its limit price of 39,000.

Assuming a last price of 40,000.

Placing a trailing stop STOP_LOSS_LIMIT BUY order, with a price of 42,000.0 and a trailing stop of 5%.

Placing a trailing stop STOP_LOSS_LIMIT SELL order, with a price of 37,500.0 and a trailing stop of 2.5%.

Placing a trailing stop TAKE_PROFIT_LIMIT BUY order, with a price of 38,000.0 and a trailing stop of 5%.

Placing a trailing stop TAKE_PROFIT_LIMIT SELL order, with a price of 41,500.0 and a trailing stop of 1.75%.

**Examples:**

Example 1 (unknown):
```unknown
trailingDelta
```

Example 2 (unknown):
```unknown
trailingDelta
```

Example 3 (unknown):
```unknown
STOP_LOSS_LIMIT
```

Example 4 (unknown):
```unknown
TAKE_PROFIT
```

---

## Commission Rates

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/faqs/commission_faq

**Contents:**
- Commission Rates
  - What are Commission Rates?​
  - What are the different types of rates?​
  - How do I know what the commission rates are?​
  - What is the difference between the response sending a test order with computeCommissionRates vs the response from querying commission rates?​
  - How is the commission calculated?​

These are the rates that determine the commission to be paid on trades when your order fills for any amount.

Standard commission rate may be reduced, depending on promotions for specific trading pairs, applicable discounts, etc.

You can find them using the following requests:

REST API: GET /api/v3/account/commission

WebSocket API: account.commission

You can also find out the commission rates to a trade from an order using the test order requests with computeCommissionRates.

A test order with computeCommissionRates returns detailed commission rates for that specific order:

Note: It does not show buyer/seller commissions separately, as these are already taken into account based on the order side.

In contrast, querying commission rates returns your current commission rates for the symbol on your account.

Using an example commission configuration:

If you placed an order with the following parameters which took immediately and fully filled in a single trade:

Since you sold BTC for USDT, the commission will be paid either in USDT or BNB.

When standard commission is calculated, the received amount is multiplied with the sum of the rates.

Since this order is on the SELL side, the received amount is the notional value. (For orders on the BUY side, the received amount would be quantity.) The order type was MARKET, making this the taker order for the trade.

Tax commission (if applicable) is calculated similarly:

Special commission (if applicable) is calculated as:

If not paying in BNB, the total commission are summed up and deducted from your received amount of USDT.

Since enabledforAccount and enabledForSymbol under discount is set to true, this means the commission will be paid in BNB assuming you have a sufficient balance.

If paying with BNB, then the standard commission will be reduced based on the discount.

First the standard commission and tax commission will be converted into BNB based on the exchange rate. For this example, assume that 1 BNB = 260 USDT.

Note that the discount does not apply to tax commissions or special commissions.

If you do not have enough BNB to pay the discounted commission, the full commission will be taken out of your received amount of USDT instead.

**Examples:**

Example 1 (unknown):
```unknown
standardCommission
```

Example 2 (unknown):
```unknown
taxCommission
```

Example 3 (unknown):
```unknown
specialCommission
```

Example 4 (unknown):
```unknown
GET /api/v3/account/commission
```

---

## Trading requests

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/websocket-api/trading-requests

**Contents:**
- Trading requests
  - Place new order (TRADE)​
  - Test new order (TRADE)​
  - Cancel order (TRADE)​
  - Cancel and replace order (TRADE)​
  - Order Amend Keep Priority (TRADE)​
  - Cancel open orders (TRADE)​
  - Order lists​
    - Place new OCO - Deprecated (TRADE)​
    - Place new Order list - OCO (TRADE)​

This adds 1 order to the EXCHANGE_MAX_ORDERS filter and the MAX_NUM_ORDERS filter.

Unfilled Order Count: 1

Select response format: ACK, RESULT, FULL.

MARKET and LIMIT orders use FULL by default, other order types default to ACK.

Arbitrary numeric value identifying the order strategy.

Values smaller than 1000000 are reserved and cannot be used.

Certain parameters (*) become mandatory based on the order type:

Supported order types:

Buy or sell quantity at the specified price or better.

LIMIT order that will be rejected if it immediately matches and trades as a taker.

This order type is also known as a POST-ONLY order.

Buy or sell at the best available market price.

MARKET order with quantity parameter specifies the amount of the base asset you want to buy or sell. Actually executed quantity of the quote asset will be determined by available market liquidity.

E.g., a MARKET BUY order on BTCUSDT for "quantity": "0.1000" specifies that you want to buy 0.1 BTC at the best available price. If there is not enough BTC at the best price, keep buying at the next best price, until either your order is filled, or you run out of USDT, or market runs out of BTC.

MARKET order with quoteOrderQty parameter specifies the amount of the quote asset you want to spend (when buying) or receive (when selling). Actually executed quantity of the base asset will be determined by available market liquidity.

E.g., a MARKET BUY on BTCUSDT for "quoteOrderQty": "100.00" specifies that you want to buy as much BTC as you can for 100 USDT at the best available price. Similarly, a SELL order will sell as much available BTC as needed for you to receive 100 USDT (before commission).

Execute a MARKET order for given quantity when specified conditions are met.

I.e., when stopPrice is reached, or when trailingDelta is activated.

Place a LIMIT order with given parameters when specified conditions are met.

Like STOP_LOSS but activates when market price moves in the favorable direction.

Like STOP_LOSS_LIMIT but activates when market price moves in the favorable direction.

Notes on using parameters for Pegged Orders:

Available timeInForce options, setting how long the order should be active before expiration:

newClientOrderId specifies clientOrderId value for the order.

A new order with the same clientOrderId is accepted only when the previous one is filled or expired.

Any LIMIT or LIMIT_MAKER order can be made into an iceberg order by specifying the icebergQty.

An order with an icebergQty must have timeInForce set to GTC.

Trigger order price rules for STOP_LOSS/TAKE_PROFIT orders:

MARKET orders using quoteOrderQty follow LOT_SIZE filter rules.

The order will execute a quantity that has notional value as close as possible to requested quoteOrderQty.

Data Source: Matching Engine

Response format is selected by using the newOrderRespType parameter.

RESULT response type:

Conditional fields in Order Responses

There are fields in the order responses (e.g. order placement, order query, order cancellation) that appear only if certain conditions are met.

These fields can apply to Order lists.

The fields are listed below:

Test order placement.

Validates new order parameters and verifies your signature but does not send the order into the matching engine.

In addition to all parameters accepted by order.place, the following optional parameters are also accepted:

Without computeCommissionRates:

With computeCommissionRates:

Cancel an active order.

If both orderId and origClientOrderId parameters are provided, the orderId is searched first, then the origClientOrderId from that result is checked against that order. If both conditions are not met the request will be rejected.

newClientOrderId will replace clientOrderId of the canceled order, freeing it up for new orders.

If you cancel an order that is a part of an order list, the entire order list is canceled.

The performance for canceling an order (single cancel or as part of a cancel-replace) is always better when only orderId is sent. Sending origClientOrderId or both orderId + origClientOrderId will be slower.

Data Source: Matching Engine

When an individual order is canceled:

When an order list is canceled:

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Regarding cancelRestrictions

Cancel an existing order and immediately place a new order instead of the canceled one.

A new order that was not attempted (i.e. when newOrderResult: NOT_ATTEMPTED), will still increase the unfilled order count by 1.

Unfilled Order Count: 1

Select response format: ACK, RESULT, FULL.

MARKET and LIMIT orders produce FULL response by default, other order types default to ACK.

Arbitrary numeric value identifying the order strategy.

Values smaller than 1000000 are reserved and cannot be used.

The allowed enums is dependent on what is configured on the symbol.

Supported values: STP Modes.

Similar to the order.place request, additional mandatory parameters (*) are determined by the new order type.

Available cancelReplaceMode options:

If both cancelOrderId and cancelOrigClientOrderId parameters are provided, the cancelOrderId is searched first, then the cancelOrigClientOrderId from that result is checked against that order. If both conditions are not met the request will be rejected.

cancelNewClientOrderId will replace clientOrderId of the canceled order, freeing it up for new orders.

newClientOrderId specifies clientOrderId value for the placed order.

A new order with the same clientOrderId is accepted only when the previous one is filled or expired.

The new order can reuse old clientOrderId of the canceled order.

This cancel-replace operation is not transactional.

If one operation succeeds but the other one fails, the successful operation is still executed.

For example, in STOP_ON_FAILURE mode, if the new order placement fails, the old order is still canceled.

Filters and order count limits are evaluated before cancellation and order placement occurs.

If new order placement is not attempted, your order count is still incremented.

Like order.cancel, if you cancel an individual order from an order list, the entire order list is canceled.

The performance for canceling an order (single cancel or as part of a cancel-replace) is always better when only orderId is sent. Sending origClientOrderId or both orderId + origClientOrderId will be slower.

Data Source: Matching Engine

If both cancel and placement succeed, you get the following response with "status": 200:

In STOP_ON_FAILURE mode, failed order cancellation prevents new order from being placed and returns the following response with "status": 400:

If cancel-replace mode allows failure and one of the operations fails, you get a response with "status": 409, and the "data" field detailing which operation succeeded, which failed, and why:

If both operations fail, response will have "status": 400:

If orderRateLimitExceededMode is DO_NOTHING regardless of cancelReplaceMode, and you have exceeded your unfilled order count, you will get status 429 with the following error:

If orderRateLimitExceededMode is CANCEL_ONLY regardless of cancelReplaceMode, and you have exceeded your unfilled order count, you will get status 409 with the following error:

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Reduce the quantity of an existing open order.

This adds 0 orders to the EXCHANGE_MAX_ORDERS filter and the MAX_NUM_ORDERS filter.

Read Order Amend Keep Priority FAQ to learn more.

Unfilled Order Count: 0

Data Source: Matching Engine

Response for a single order:

Response for an order which is part of an Order list:

Note: The payloads above do not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Cancel all open orders on a symbol. This includes orders that are part of an order list.

Data Source: Matching Engine

Cancellation reports for orders and order lists have the same format as in order.cancel.

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Send in a new one-cancels-the-other (OCO) pair: LIMIT_MAKER + STOP_LOSS/STOP_LOSS_LIMIT orders (called legs), where activation of one order immediately cancels the other.

This adds 1 order to EXCHANGE_MAX_ORDERS filter and the MAX_NUM_ORDERS filter

Unfilled Order Count: 1

Arbitrary numeric value identifying the limit order strategy.

Values smaller than 1000000 are reserved and cannot be used.

Arbitrary numeric value identifying the stop order strategy.

Values smaller than 1000000 are reserved and cannot be used.

listClientOrderId parameter specifies listClientOrderId for the OCO pair.

A new OCO with the same listClientOrderId is accepted only when the previous one is filled or completely expired.

listClientOrderId is distinct from clientOrderId of individual orders.

limitClientOrderId and stopClientOrderId specify clientOrderId values for both legs of the OCO.

A new order with the same clientOrderId is accepted only when the previous one is filled or expired.

Price restrictions on the legs:

Both legs have the same quantity.

However, you can set different iceberg quantity for individual legs.

If stopIcebergQty is used, stopLimitTimeInForce must be GTC.

trailingDelta applies only to the STOP_LOSS/STOP_LOSS_LIMIT leg of the OCO.

Data Source: Matching Engine

Response format for orderReports is selected using the newOrderRespType parameter. The following example is for RESULT response type. See order.place for more examples.

Send in an one-cancels-the-other (OCO) pair, where activation of one order immediately cancels the other.

Unfilled Order Count: 2

Data Source: Matching Engine

Response format for orderReports is selected using the newOrderRespType parameter. The following example is for RESULT response type. See order.place for more examples.

Unfilled Order Count: 2

Mandatory parameters based on pendingType or workingType

Depending on the pendingType or workingType, some optional parameters will become mandatory.

Data Source: Matching Engine

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Unfilled Order Count: 3

Mandatory parameters based on pendingAboveType, pendingBelowType or workingType

Depending on the pendingAboveType/pendingBelowType or workingType, some optional parameters will become mandatory.

Data Source: Matching Engine

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Cancel an active order list.

If both orderListId and listClientOrderId parameters are provided, the orderListId is searched first, then the listClientOrderId from that result is checked against that order. If both conditions are not met the request will be rejected.

Canceling an individual order with order.cancel will cancel the entire order list as well.

Data Source: Matching Engine

Places an order using smart order routing (SOR).

This adds 1 order to the EXCHANGE_MAX_ORDERS filter and the MAX_NUM_ORDERS filter.

Read SOR FAQ to learn more.

Unfilled Order Count: 1

Select response format: ACK, RESULT, FULL.

MARKET and LIMIT orders use FULL by default.

Arbitrary numeric value identifying the order strategy.

Values smaller than 1000000 are reserved and cannot be used.

Note: sor.order.place only supports LIMIT and MARKET orders. quoteOrderQty is not supported.

Data Source: Matching Engine

Test new order creation and signature/recvWindow using smart order routing (SOR). Creates and validates a new order but does not send it into the matching engine.

In addition to all parameters accepted by sor.order.place, the following optional parameters are also accepted:

Without computeCommissionRates:

With computeCommissionRates:

**Examples:**

Example 1 (javascript):
```javascript
{  "id": "56374a46-3061-486b-a311-99ee972eb648",  "method": "order.place",  "params": {    "symbol": "BTCUSDT",    "side": "SELL",    "type": "LIMIT",    "timeInForce": "GTC",    "price": "23416.10000000",    "quantity": "0.00847000",    "apiKey": "vmPUZE6mv9SD5VNHk4HlWFsOr6aKE2zvsw0MuIgwCIPy6utIco14y7Ju91duEh8A",    "signature": "15af09e41c36f3cc61378c2fbe2c33719a03dd5eba8d0f9206fbda44de717c88",    "timestamp": 1660801715431  }}
```

Example 2 (javascript):
```javascript
{  "id": "56374a46-3061-486b-a311-99ee972eb648",  "method": "order.place",  "params": {    "symbol": "BTCUSDT",    "side": "SELL",    "type": "LIMIT",    "timeInForce": "GTC",    "price": "23416.10000000",    "quantity": "0.00847000",    "apiKey": "vmPUZE6mv9SD5VNHk4HlWFsOr6aKE2zvsw0MuIgwCIPy6utIco14y7Ju91duEh8A",    "signature": "15af09e41c36f3cc61378c2fbe2c33719a03dd5eba8d0f9206fbda44de717c88",    "timestamp": 1660801715431  }}
```

Example 3 (unknown):
```unknown
EXCHANGE_MAX_ORDERS
```

Example 4 (unknown):
```unknown
MAX_NUM_ORDERS
```

---

## Spot Unfilled Order Count Rules

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/faqs/order_count_decrement

**Contents:**
- Spot Unfilled Order Count Rules
  - What are the current rate limits?​
  - How does the unfilled ORDERS rate limit work?​
  - Is the unfilled order count tracked by IP address?​
  - How do filled orders affect the unfilled order count?​
  - How do canceled or expired orders affect the unfilled order count?​
  - Which time zone does "interval":"DAY" use?​
  - What happens if I placed an order yesterday but it is filled the next day?​

To ensure a fair and orderly Spot market, we limit the rate at which new orders may be placed.

The rate limit applies to the number of new, unfilled orders placed within a time interval. That is, orders which are partially or fully filled do not count against the rate limit.

[!NOTE] Unfilled order rate limit rewards efficient traders.

So long as your orders trade, you can keep trading.

More information: How do filled orders affect the rate limit?

You can query current rate limits using the "exchange information" request.

The "rateLimitType": "ORDERS" indicates the current unfilled order rate limit.

Please refer to the API documentation:

[!IMPORTANT] Order placement requests are also affected by the general request rate limits on REST and WebSocket API and the message limits on FIX API.

If you send too many requests at a high rate, you will be blocked by the API.

Every successful request to place an order adds to the unfilled order count for the current time interval. If too many unfilled orders accumulate during the interval, subsequent requests will be rejected.

For example, if the unfilled order rate limit is 100 per 10 seconds:

then you can place at most 100 new orders between 12:34:00 and 12:34:10, then 100 more from 12:34:10 to 12:34:20, and so on.

[!TIP] If the newly placed orders receive fills, your unfilled order count decreases and you may place more orders during the time interval.

More information: How do filled orders affect the rate limit?

When an order is rejected by the system due to the unfilled order rate limit, the HTTP status code is set to 429 Too Many Requests and the error code is -1015 "Too many new orders".

If you encounter these errors, please stop sending orders until the affected rate limit interval expires.

Please refer to the API documentation:

Unfilled order count is tracked by (sub)account.

Unfilled order count is shared across all IP addresses, all API keys, and all APIs.

When an order is filled for the first time (partially or fully), your unfilled order count is decremented by one order for all intervals of the ORDERS rate limit. Effectively, orders that trade do not count towards the rate limit, allowing efficient traders to keep placing new orders.

Certain orders provide additional incentive:

In these cases the unfilled order count may be decremented by more than one order for each order that starts trading.

Note how for every taker order that immediately trades, the unfilled order count is decremented later, allowing you to keep placing orders.

Note how for every maker order that is filled later, the unfilled order count is decremented by a higher amount, allowing you to place more orders.

Canceling an order does not change the unfilled order count.

Expired orders also do not change the unfilled order count.

New order fills decrease your current unfilled order count regardless of when the orders were placed.

Note: You do not get credit for order fills. That is, once the unfilled order count is down to zero, additional fills will not decrease it further. New orders will increase the count as usual.

**Examples:**

Example 1 (unknown):
```unknown
"rateLimitType": "ORDERS"
```

Example 2 (unknown):
```unknown
GET /api/v3/exchangeInfo
```

Example 3 (unknown):
```unknown
exchangeInfo
```

Example 4 (javascript):
```javascript
{  "rateLimitType": "ORDERS",  "interval": "SECOND",  "intervalNum": 10,  "limit": 100}
```

---

## Trading endpoints

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/rest-api/trading-endpoints

**Contents:**
- Trading endpoints
  - New order (TRADE)​
  - Test new order (TRADE)​
  - Query order (USER_DATA)​
  - Cancel order (TRADE)​
  - Cancel All Open Orders on a Symbol (TRADE)​
  - Cancel an Existing Order and Send a New Order (TRADE)​
  - Order Amend Keep Priority (TRADE)​
  - Order lists​
    - New Order list - OCO (TRADE)​

This adds 1 order to the EXCHANGE_MAX_ORDERS filter and the MAX_NUM_ORDERS filter.

Unfilled Order Count: 1

Some additional mandatory parameters based on order type:

Notes on using parameters for Pegged Orders:

Any LIMIT or LIMIT_MAKER type order can be made an iceberg order by sending an icebergQty.

Any order with an icebergQty MUST have timeInForce set to GTC.

For STOP_LOSS, STOP_LOSS_LIMIT, TAKE_PROFIT_LIMIT and TAKE_PROFIT orders, trailingDelta can be combined with stopPrice.

MARKET orders using quoteOrderQty will not break LOT_SIZE filter rules; the order will execute a quantity that will have the notional value as close as possible to quoteOrderQty. Trigger order price rules against market price for both MARKET and LIMIT versions:

Price above market price: STOP_LOSS BUY, TAKE_PROFIT SELL

Price below market price: STOP_LOSS SELL, TAKE_PROFIT BUY

Data Source: Matching Engine

Conditional fields in Order Responses

There are fields in the order responses (e.g. order placement, order query, order cancellation) that appear only if certain conditions are met.

These fields can apply to order lists.

The fields are listed below:

Test new order creation and signature/recvWindow long. Creates and validates a new order but does not send it into the matching engine.

In addition to all parameters accepted by POST /api/v3/order, the following optional parameters are also accepted:

Without computeCommissionRates

With computeCommissionRates

Check an order's status.

Data Source: Memory => Database

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Cancel an active order.

Data Source: Matching Engine

Regarding cancelRestrictions

Cancels all active orders on a symbol. This includes orders that are part of an order list.

Data Source: Matching Engine

Cancels an existing order and places a new order on the same symbol.

Filters and Order Count are evaluated before the processing of the cancellation and order placement occurs.

A new order that was not attempted (i.e. when newOrderResult: NOT_ATTEMPTED), will still increase the unfilled order count by 1.

Unfilled Order Count: 1

Similar to POST /api/v3/order, additional mandatory parameters are determined by type.

Response format varies depending on whether the processing of the message succeeded, partially succeeded, or failed.

Data Source: Matching Engine

Response SUCCESS and account has not exceeded the unfilled order count:

Response when Cancel Order Fails with STOP_ON FAILURE and account has not exceeded their unfilled order count:

Response when Cancel Order Succeeds but New Order Placement Fails and account has not exceeded their unfilled order count:

Response when Cancel Order fails with ALLOW_FAILURE and account has not exceeded their unfilled order count:

Response when both Cancel Order and New Order Placement fail using cancelReplaceMode=ALLOW_FAILURE and account has not exceeded their unfilled order count:

Response when using orderRateLimitExceededMode=DO_NOTHING and account's unfilled order count has been exceeded:

Response when using orderRateLimitExceededMode=CANCEL_ONLY and account's unfilled order count has been exceeded:

Reduce the quantity of an existing open order.

This adds 0 orders to the EXCHANGE_MAX_ORDERS filter and the MAX_NUM_ORDERS filter.

Read Order Amend Keep Priority FAQ to learn more.

Unfilled Order Count: 0

Data Source: Matching Engine

Response: Response for a single order:

Response for an order that is part of an Order list:

Note: The payloads above do not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Send in an one-cancels-the-other (OCO) pair, where activation of one order immediately cancels the other.

Unfilled Order Count: 2

Data Source: Matching Engine

Response format for orderReports is selected using the newOrderRespType parameter. The following example is for the RESULT response type. See POST /api/v3/order for more examples.

Unfilled Order Count: 2

Mandatory parameters based on pendingType or workingType

Depending on the pendingType or workingType, some optional parameters will become mandatory.

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Unfilled Order Count: 3

Mandatory parameters based on pendingAboveType, pendingBelowType or workingType

Depending on the pendingAboveType/pendingBelowType or workingType, some optional parameters will become mandatory.

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Cancel an entire Order list

Data Source: Matching Engine

Places an order using smart order routing (SOR).

This adds 1 order to the EXCHANGE_MAX_ORDERS filter and the MAX_NUM_ORDERS filter.

Read SOR FAQ to learn more.

Unfilled Order Count: 1

Note: POST /api/v3/sor/order only supports LIMIT and MARKET orders. quoteOrderQty is not supported.

Data Source: Matching Engine

Test new order creation and signature/recvWindow using smart order routing (SOR). Creates and validates a new order but does not send it into the matching engine.

In addition to all parameters accepted by POST /api/v3/sor/order, the following optional parameters are also accepted:

Without computeCommissionRates

With computeCommissionRates

**Examples:**

Example 1 (text):
```text
POST /api/v3/order
```

Example 2 (text):
```text
POST /api/v3/order
```

Example 3 (unknown):
```unknown
EXCHANGE_MAX_ORDERS
```

Example 4 (unknown):
```unknown
MAX_NUM_ORDERS
```

---

## WebSocket Streams for Binance (2025-01-28)

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/web-socket-streams

**Contents:**
- WebSocket Streams for Binance (2025-01-28)
- General WSS information​
- WebSocket Limits​
- Live Subscribing/Unsubscribing to streams​
  - Subscribe to a stream​
  - Unsubscribe to a stream​
  - Listing Subscriptions​
  - Setting Properties​
  - Retrieving Properties​
  - Error Messages​

Currently, the only property that can be set is whether combined stream payloads are enabled or not. The combined property is set to false when connecting using /ws/ ("raw streams") and true when connecting using /stream/.

The Aggregate Trade Streams push trade information that is aggregated for a single taker order.

Stream Name: <symbol>@aggTrade

Update Speed: Real-time

The Trade Streams push raw trade information; each trade has a unique buyer and seller.

Stream Name: <symbol>@trade

Update Speed: Real-time

The Kline/Candlestick Stream push updates to the current klines/candlestick every second in UTC+0 timezone

Kline/Candlestick chart intervals:

s-> seconds; m -> minutes; h -> hours; d -> days; w -> weeks; M -> months

Stream Name: <symbol>@kline_<interval>

Update Speed: 1000ms for 1s, 2000ms for the other intervals

The Kline/Candlestick Stream push updates to the current klines/candlestick every second in UTC+8 timezone

Kline/Candlestick chart intervals:

Supported intervals: See Kline/Candlestick chart intervals

UTC+8 timezone offset:

Stream Name: <symbol>@kline_<interval>@+08:00

Update Speed: 1000ms for 1s, 2000ms for the other intervals

24hr rolling window mini-ticker statistics. These are NOT the statistics of the UTC day, but a 24hr rolling window for the previous 24hrs.

Stream Name: <symbol>@miniTicker

24hr rolling window mini-ticker statistics for all symbols that changed in an array. These are NOT the statistics of the UTC day, but a 24hr rolling window for the previous 24hrs. Note that only tickers that have changed will be present in the array.

Stream Name: !miniTicker@arr

24hr rolling window ticker statistics for a single symbol. These are NOT the statistics of the UTC day, but a 24hr rolling window for the previous 24hrs.

Stream Name: <symbol>@ticker

24hr rolling window ticker statistics for all symbols that changed in an array. These are NOT the statistics of the UTC day, but a 24hr rolling window for the previous 24hrs. Note that only tickers that have changed will be present in the array.

Stream Name: !ticker@arr

Rolling window ticker statistics for a single symbol, computed over multiple windows.

Stream Name: <symbol>@ticker_<window_size>

Window Sizes: 1h,4h,1d

Note: This stream is different from the <symbol>@ticker stream. The open time "O" always starts on a minute, while the closing time "C" is the current time of the update. As such, the effective window might be up to 59999ms wider than <window_size>.

Rolling window ticker statistics for all market symbols, computed over multiple windows. Note that only tickers that have changed will be present in the array.

Stream Name: !ticker_<window-size>@arr

Window Size: 1h,4h,1d

Pushes any update to the best bid or ask's price or quantity in real-time for a specified symbol. Multiple <symbol>@bookTicker streams can be subscribed to over one connection.

Stream Name: <symbol>@bookTicker

Update Speed: Real-time

Average price streams push changes in the average price over a fixed time interval.

Stream Name: <symbol>@avgPrice

Top <levels> bids and asks, pushed every second. Valid <levels> are 5, 10, or 20.

Stream Names: <symbol>@depth<levels> OR <symbol>@depth<levels>@100ms

Update Speed: 1000ms or 100ms

Order book price and quantity depth updates used to locally manage an order book.

Stream Name: <symbol>@depth OR <symbol>@depth@100ms

Update Speed: 1000ms or 100ms

To apply an event to your local order book, follow this update procedure:

[!NOTE] Since depth snapshots retrieved from the API have a limit on the number of price levels (5000 on each side maximum), you won't learn the quantities for the levels outside of the initial snapshot unless they change. So be careful when using the information for those levels, since they might not reflect the full view of the order book. However, for most use cases, seeing 5000 levels on each side is enough to understand the market and trade effectively.

**Examples:**

Example 1 (unknown):
```unknown
pong frames
```

Example 2 (unknown):
```unknown
timeUnit=MICROSECOND or timeUnit=microsecond
```

Example 3 (unknown):
```unknown
/stream?streams=btcusdt@trade&timeUnit=MICROSECOND
```

Example 4 (javascript):
```javascript
{  "method": "SUBSCRIBE",  "params": [    "btcusdt@aggTrade",    "btcusdt@depth"  ],  "id": 1}
```

---

## Trading requests

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/websocket-api/trading-requests

**Contents:**
- Trading requests
  - Place new order (TRADE)​
  - Test new order (TRADE)​
  - Cancel order (TRADE)​
  - Cancel and replace order (TRADE)​
  - Order Amend Keep Priority (TRADE)​
  - Cancel open orders (TRADE)​
  - Order lists​
    - Place new Order list - OCO (TRADE)​
    - Place new Order list - OTO (TRADE)​

This adds 1 order to the EXCHANGE_MAX_ORDERS filter and the MAX_NUM_ORDERS filter.

Unfilled Order Count: 1

Select response format: ACK, RESULT, FULL.

MARKET and LIMIT orders use FULL by default, other order types default to ACK.

Arbitrary numeric value identifying the order strategy.

Values smaller than 1000000 are reserved and cannot be used.

Certain parameters (*) become mandatory based on the order type:

Supported order types:

Buy or sell quantity at the specified price or better.

LIMIT order that will be rejected if it immediately matches and trades as a taker.

This order type is also known as a POST-ONLY order.

Buy or sell at the best available market price.

MARKET order with quantity parameter specifies the amount of the base asset you want to buy or sell. Actually executed quantity of the quote asset will be determined by available market liquidity.

E.g., a MARKET BUY order on BTCUSDT for "quantity": "0.1000" specifies that you want to buy 0.1 BTC at the best available price. If there is not enough BTC at the best price, keep buying at the next best price, until either your order is filled, or you run out of USDT, or market runs out of BTC.

MARKET order with quoteOrderQty parameter specifies the amount of the quote asset you want to spend (when buying) or receive (when selling). Actually executed quantity of the base asset will be determined by available market liquidity.

E.g., a MARKET BUY on BTCUSDT for "quoteOrderQty": "100.00" specifies that you want to buy as much BTC as you can for 100 USDT at the best available price. Similarly, a SELL order will sell as much available BTC as needed for you to receive 100 USDT (before commission).

Execute a MARKET order for given quantity when specified conditions are met.

I.e., when stopPrice is reached, or when trailingDelta is activated.

Place a LIMIT order with given parameters when specified conditions are met.

Like STOP_LOSS but activates when market price moves in the favorable direction.

Like STOP_LOSS_LIMIT but activates when market price moves in the favorable direction.

Notes on using parameters for Pegged Orders:

Available timeInForce options, setting how long the order should be active before expiration:

newClientOrderId specifies clientOrderId value for the order.

A new order with the same clientOrderId is accepted only when the previous one is filled or expired.

Any LIMIT or LIMIT_MAKER order can be made into an iceberg order by specifying the icebergQty.

An order with an icebergQty must have timeInForce set to GTC.

Trigger order price rules for STOP_LOSS/TAKE_PROFIT orders:

MARKET orders using quoteOrderQty follow LOT_SIZE filter rules.

The order will execute a quantity that has notional value as close as possible to requested quoteOrderQty.

Data Source: Matching Engine

Response format is selected by using the newOrderRespType parameter.

RESULT response type:

Conditional fields in Order Responses

There are fields in the order responses (e.g. order placement, order query, order cancellation) that appear only if certain conditions are met.

These fields can apply to Order lists.

The fields are listed below:

Test order placement.

Validates new order parameters and verifies your signature but does not send the order into the matching engine.

In addition to all parameters accepted by order.place, the following optional parameters are also accepted:

Without computeCommissionRates:

With computeCommissionRates:

Cancel an active order.

If both orderId and origClientOrderId parameters are provided, the orderId is searched first, then the origClientOrderId from that result is checked against that order. If both conditions are not met the request will be rejected.

newClientOrderId will replace clientOrderId of the canceled order, freeing it up for new orders.

If you cancel an order that is a part of an order list, the entire order list is canceled.

The performance for canceling an order (single cancel or as part of a cancel-replace) is always better when only orderId is sent. Sending origClientOrderId or both orderId + origClientOrderId will be slower.

Data Source: Matching Engine

When an individual order is canceled:

When an order list is canceled:

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Regarding cancelRestrictions

Cancel an existing order and immediately place a new order instead of the canceled one.

A new order that was not attempted (i.e. when newOrderResult: NOT_ATTEMPTED), will still increase the unfilled order count by 1.

Unfilled Order Count: 1

Select response format: ACK, RESULT, FULL.

MARKET and LIMIT orders produce FULL response by default, other order types default to ACK.

Arbitrary numeric value identifying the order strategy.

Values smaller than 1000000 are reserved and cannot be used.

The allowed enums is dependent on what is configured on the symbol.

Supported values: STP Modes.

Similar to the order.place request, additional mandatory parameters (*) are determined by the new order type.

Available cancelReplaceMode options:

If both cancelOrderId and cancelOrigClientOrderId parameters are provided, the cancelOrderId is searched first, then the cancelOrigClientOrderId from that result is checked against that order. If both conditions are not met the request will be rejected.

cancelNewClientOrderId will replace clientOrderId of the canceled order, freeing it up for new orders.

newClientOrderId specifies clientOrderId value for the placed order.

A new order with the same clientOrderId is accepted only when the previous one is filled or expired.

The new order can reuse old clientOrderId of the canceled order.

This cancel-replace operation is not transactional.

If one operation succeeds but the other one fails, the successful operation is still executed.

For example, in STOP_ON_FAILURE mode, if the new order placement fails, the old order is still canceled.

Filters and order count limits are evaluated before cancellation and order placement occurs.

If new order placement is not attempted, your order count is still incremented.

Like order.cancel, if you cancel an individual order from an order list, the entire order list is canceled.

The performance for canceling an order (single cancel or as part of a cancel-replace) is always better when only orderId is sent. Sending origClientOrderId or both orderId + origClientOrderId will be slower.

Data Source: Matching Engine

If both cancel and placement succeed, you get the following response with "status": 200:

In STOP_ON_FAILURE mode, failed order cancellation prevents new order from being placed and returns the following response with "status": 400:

If cancel-replace mode allows failure and one of the operations fails, you get a response with "status": 409, and the "data" field detailing which operation succeeded, which failed, and why:

If both operations fail, response will have "status": 400:

If orderRateLimitExceededMode is DO_NOTHING regardless of cancelReplaceMode, and you have exceeded your unfilled order count, you will get status 429 with the following error:

If orderRateLimitExceededMode is CANCEL_ONLY regardless of cancelReplaceMode, and you have exceeded your unfilled order count, you will get status 409 with the following error:

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Reduce the quantity of an existing open order.

This adds 0 orders to the EXCHANGE_MAX_ORDERS filter and the MAX_NUM_ORDERS filter.

Read Order Amend Keep Priority FAQ to learn more.

Unfilled Order Count: 0

Data Source: Matching Engine

Response for a single order:

Response for an order which is part of an Order list:

Note: The payloads above do not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Cancel all open orders on a symbol. This includes orders that are part of an order list.

Data Source: Matching Engine

Cancellation reports for orders and order lists have the same format as in order.cancel.

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Send in an one-cancels-the-other (OCO) pair, where activation of one order immediately cancels the other.

Unfilled Order Count: 2

Data Source: Matching Engine

Response format for orderReports is selected using the newOrderRespType parameter. The following example is for RESULT response type. See order.place for more examples.

Unfilled Order Count: 2

Mandatory parameters based on pendingType or workingType

Depending on the pendingType or workingType, some optional parameters will become mandatory.

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Unfilled Order Count: 3

Mandatory parameters based on pendingAboveType, pendingBelowType or workingType

Depending on the pendingAboveType/pendingBelowType or workingType, some optional parameters will become mandatory.

Data Source: Matching Engine

Note: The payload above does not show all fields that can appear. Please refer to Conditional fields in Order Responses.

Cancel an active order list.

If both orderListId and listClientOrderId parameters are provided, the orderListId is searched first, then the listClientOrderId from that result is checked against that order. If both conditions are not met the request will be rejected.

Canceling an individual order with order.cancel will cancel the entire order list as well.

Data Source: Matching Engine

Places an order using smart order routing (SOR).

This adds 1 order to the EXCHANGE_MAX_ORDERS filter and the MAX_NUM_ORDERS filter.

Read SOR FAQ to learn more.

Unfilled Order Count: 1

Select response format: ACK, RESULT, FULL.

MARKET and LIMIT orders use FULL by default.

Arbitrary numeric value identifying the order strategy.

Values smaller than 1000000 are reserved and cannot be used.

Note: sor.order.place only supports LIMIT and MARKET orders. quoteOrderQty is not supported.

Data Source: Matching Engine

Test new order creation and signature/recvWindow using smart order routing (SOR). Creates and validates a new order but does not send it into the matching engine.

In addition to all parameters accepted by sor.order.place, the following optional parameters are also accepted:

Without computeCommissionRates:

With computeCommissionRates:

**Examples:**

Example 1 (javascript):
```javascript
{  "id": "56374a46-3061-486b-a311-99ee972eb648",  "method": "order.place",  "params": {    "symbol": "BTCUSDT",    "side": "SELL",    "type": "LIMIT",    "timeInForce": "GTC",    "price": "23416.10000000",    "quantity": "0.00847000",    "apiKey": "vmPUZE6mv9SD5VNHk4HlWFsOr6aKE2zvsw0MuIgwCIPy6utIco14y7Ju91duEh8A",    "signature": "15af09e41c36f3cc61378c2fbe2c33719a03dd5eba8d0f9206fbda44de717c88",    "timestamp": 1660801715431  }}
```

Example 2 (javascript):
```javascript
{  "id": "56374a46-3061-486b-a311-99ee972eb648",  "method": "order.place",  "params": {    "symbol": "BTCUSDT",    "side": "SELL",    "type": "LIMIT",    "timeInForce": "GTC",    "price": "23416.10000000",    "quantity": "0.00847000",    "apiKey": "vmPUZE6mv9SD5VNHk4HlWFsOr6aKE2zvsw0MuIgwCIPy6utIco14y7Ju91duEh8A",    "signature": "15af09e41c36f3cc61378c2fbe2c33719a03dd5eba8d0f9206fbda44de717c88",    "timestamp": 1660801715431  }}
```

Example 3 (unknown):
```unknown
EXCHANGE_MAX_ORDERS
```

Example 4 (unknown):
```unknown
MAX_NUM_ORDERS
```

---

## ENUM Definitions

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/enums

**Contents:**
- ENUM Definitions
- Symbol status (status):​
- Account and Symbol Permissions (permissions)​
- Order status (status)​
- Order List Status (listStatusType)​
- Order List Order Status (listOrderStatus)​
- ContingencyType​
- AllocationType​
- Order types (orderTypes, type)​
- Order Response Type (newOrderRespType)​

This will apply for both REST API and WebSocket API.

This sets how long an order will be active before expiration.

Read Self Trade Prevention (STP) FAQ to learn more.

**Examples:**

Example 1 (unknown):
```unknown
PENDING_NEW
```

Example 2 (unknown):
```unknown
PARTIALLY_FILLED
```

Example 3 (unknown):
```unknown
PENDING_CANCEL
```

Example 4 (unknown):
```unknown
EXPIRED_IN_MATCH
```

---

## WebSocket Streams for Binance SPOT Testnet

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/web-socket-streams

**Contents:**
- WebSocket Streams for Binance SPOT Testnet
- General WSS information​
- WebSocket Limits​
- Live Subscribing/Unsubscribing to streams​
  - Subscribe to a stream​
  - Unsubscribe to a stream​
  - Listing Subscriptions​
  - Setting Properties​
  - Retrieving Properties​
  - Error Messages​

Last Updated: 2025-04-01

Currently, the only property that can be set is whether combined stream payloads are enabled or not. The combined property is set to false when connecting using /ws/ ("raw streams") and true when connecting using /stream/.

The Aggregate Trade Streams push trade information that is aggregated for a single taker order.

Stream Name: <symbol>@aggTrade

Update Speed: Real-time

The Trade Streams push raw trade information; each trade has a unique buyer and seller.

Stream Name: <symbol>@trade

Update Speed: Real-time

The Kline/Candlestick Stream push updates to the current klines/candlestick every second in UTC+0 timezone.

Kline/Candlestick chart intervals:

s-> seconds; m -> minutes; h -> hours; d -> days; w -> weeks; M -> months

Stream Name: <symbol>@kline_<interval>

Update Speed: 1000ms for 1s, 2000ms for the other intervals

The Kline/Candlestick Stream push updates to the current klines/candlestick every second in UTC+8 timezone.

Kline/Candlestick chart intervals: Supported intervals: See Kline/Candlestick chart intervals

UTC+8 timezone offset:

Stream Name: <symbol>@kline_<interval>@+08:00

Update Speed: 1000ms for 1s, 2000ms for the other intervals

24hr rolling window mini-ticker statistics. These are NOT the statistics of the UTC day, but a 24hr rolling window for the previous 24hrs.

Stream Name: <symbol>@miniTicker

24hr rolling window mini-ticker statistics for all symbols that changed in an array. These are NOT the statistics of the UTC day, but a 24hr rolling window for the previous 24hrs. Note that only tickers that have changed will be present in the array.

Stream Name: !miniTicker@arr

24hr rolling window ticker statistics for a single symbol. These are NOT the statistics of the UTC day, but a 24hr rolling window for the previous 24hrs.

Stream Name: <symbol>@ticker

24hr rolling window ticker statistics for all symbols that changed in an array. These are NOT the statistics of the UTC day, but a 24hr rolling window for the previous 24hrs. Note that only tickers that have changed will be present in the array.

Stream Name: !ticker@arr

Rolling window ticker statistics for a single symbol, computed over multiple windows.

Stream Name: <symbol>@ticker_<window_size>

Window Sizes: 1h,4h,1d

Note: This stream is different from the <symbol>@ticker stream. The open time O always starts on a minute, while the closing time C is the current time of the update. As such, the effective window might be up to 59999ms wider that <window_size>.

Rolling window ticker statistics for all market symbols, computed over multiple windows. Note that only tickers that have changed will be present in the array.

Stream Name: !ticker_<window-size>@arr

Window Size: 1h,4h,1d

Pushes any update to the best bid or ask's price or quantity in real-time for a specified symbol. Multiple <symbol>@bookTicker streams can be subscribed to over one connection.

Stream Name: <symbol>@bookTicker

Update Speed: Real-time

Average price streams push changes in the average price over a fixed time interval.

Stream Name: <symbol>@avgPrice

Top <levels> bids and asks, pushed every second. Valid <levels> are 5, 10, or 20.

Stream Names: <symbol>@depth<levels> OR <symbol>@depth<levels>@100ms

Update Speed: 1000ms or 100ms

Order book price and quantity depth updates used to locally manage an order book.

Stream Name: <symbol>@depth OR <symbol>@depth@100ms

Update Speed: 1000ms or 100ms

To apply an event to your local order book, follow this update procedure:

[!NOTE] Since depth snapshots retrieved from the API have a limit on the number of price levels (5000 on each side maximum), you won't learn the quantities for the levels outside of the initial snapshot unless they change. So be careful when using the information for those levels, since they might not reflect the full view of the order book. However, for most use cases, seeing 5000 levels on each side is enough to understand the market and trade effectively.

**Examples:**

Example 1 (unknown):
```unknown
timeUnit=MICROSECOND
```

Example 2 (unknown):
```unknown
timeUnit=microsecond
```

Example 3 (unknown):
```unknown
/stream?streams=btcusdt@trade&timeUnit=MICROSECOND
```

Example 4 (unknown):
```unknown
pong frames
```

---

## SPOT API Glossary

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/faqs/spot_glossary

**Contents:**
- SPOT API Glossary
  - A​
  - B​
  - C​
  - D​
  - E​
  - F​
  - G​
  - H​
  - I​

Disclaimer: This glossary refers only to the SPOT API Implementation. The definition for these terms may differ with regards to Futures, Options, and other APIs by Binance.

aggTrade/Aggregate trade

baseCommissionPrecision

GTC/ Good Til Canceled

IOC / Immediate or Canceled

Last Prevented Quantity

Order Amend Keep Priority

Prevented execution price

Prevented execution quantity

Prevented execution quote quantity

quoteCommissionPrecision

Self Trade Prevention (STP)

selfTradePreventionMode

Smart Order Routing (SOR)

specialCommissionForOrder/specialCommission

standardCommissionForOrder/standardCommission

taxCommissionForOrder/taxCommission

**Examples:**

Example 1 (unknown):
```unknown
newOrderRespType
```

Example 2 (unknown):
```unknown
orderListId
```

Example 3 (unknown):
```unknown
clientOrderId
```

Example 4 (unknown):
```unknown
transactTime
```

---

## ENUM Definitions

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/enums

**Contents:**
- ENUM Definitions
- Symbol status (status)​
- Account and Symbol Permissions (permissions)​
- Order status (status)​
- Order List Status (listStatusType)​
- Order List Order Status (listOrderStatus)​
- ContingencyType​
- AllocationType​
- Order types (orderTypes, type)​
- Order Response Type (newOrderRespType)​

This will apply for both REST API and WebSocket API.

This sets how long an order will be active before expiration.

Read Self Trade Prevention (STP) FAQ to learn more.

**Examples:**

Example 1 (unknown):
```unknown
TRD_GRP_002
```

Example 2 (unknown):
```unknown
TRD_GRP_003
```

Example 3 (unknown):
```unknown
TRD_GRP_004
```

Example 4 (unknown):
```unknown
TRD_GRP_005
```

---

## Pegged orders

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/faqs/pegged_orders

**Contents:**
- Pegged orders
- What are pegged orders?​
- How can I send a pegged order?​
- What order types support pegged orders?​
  - Limit orders​
  - Stop-limit orders​
  - OCO​
  - OTO and OTOCO​
- Which symbols allow pegged orders?​
- Which Filters are applicable to pegged orders?​

Pegged orders are essentially limit orders with the price derived from the order book.

For example, instead of using a specific price (e.g. SELL 1 BTC for at least 100,000 USDC) you can send orders like “SELL 1 BTC at the best asking price” to queue your order after the orders on the book at the highest price, or “BUY 1 BTC for 100,000 USDT or best offer, IOC” to cherry-pick the sellers at the lowest price, and only that price.

Pegged orders offer a way for market makers to match the best price with minimal latency, while retail users can get quick fills at the best price with minimal slippage.

Pegged orders are also known as “best bid-offer” or BBO orders.

Please refer to the following table:

pegOffsetType and pegOffsetValue PRICE_LEVEL — offset by existing price levels, deeper into the order book

For order lists: (Please see the API documentation for more details.)

Currently, Smart Order Routing (SOR) does not support pegged orders.

This sample REST API response shows that for pegged orders, peggedPrice reflects the selected price, while price is the original order price (zero if not set).

All order types, with the exception of MARKET orders, are supported by this feature.

Since both STOP_LOSS and TAKE_PROFIT orders place a MARKET order once the stop condition is met, these order types cannot be pegged.

Pegged limit orders immediately enter the market at the current best price:

Pegged stop-limit orders enter the market at the best price when price movement triggers the stop order (via stop price or trailing stop):

That is, stop orders use the best price at the time when they are triggered, which is different from the price when the stop order is placed. Only the limit price can be pegged, not the stop price.

OCO order lists may use peg instructions.

OTO order lists may use peg instructions as well.

OTOCO order lists may contain pegged orders as well, similar to OTO and OCO.

Please refer to Exchange Information requests and look for the field pegInstructionsAllowed. If set to true, pegged orders can be used with the symbol.

Pegged orders are required to pass all applicable filters with the selected price:

If a pegged order specifies price, it must pass validation at both price and peggedPrice.

Contingent pegged orders as well as pegged pending orders of OTO order lists are (re)validated at the trigger time and may be rejected later.

**Examples:**

Example 1 (unknown):
```unknown
POST /api/v3/order
```

Example 2 (unknown):
```unknown
pegPriceType
```

Example 3 (unknown):
```unknown
pegOffsetType
```

Example 4 (unknown):
```unknown
pegOffsetValue PRICE_LEVEL
```

---

## Self Trade Prevention (STP) FAQ

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/faqs/stp_faq

**Contents:**
- Self Trade Prevention (STP) FAQ
  - What is Self Trade Prevention?​
  - What defines a self-trade?​
  - What happens when STP is triggered?​
  - What is a Trade Group Id?​
  - What is a Prevented Match?​
  - What is "prevented quantity?"​
  - How do I know which symbol uses STP?​
  - How do I know if an order expired due to STP?​
  - STP Examples​

Self Trade Prevention (or STP) prevents orders of users, or the user's tradeGroupId to match against their own.

A self-trade can occur in either scenario:

There are five possible modes for what the system does when an order would create a self-trade.

NONE - This mode exempts the order from self-trade prevention. Accounts or Trade group IDs will not be compared, no orders will be expired, and the trade will occur.

EXPIRE_TAKER - This mode prevents a trade by immediately expiring the taker order's remaining quantity.

EXPIRE_MAKER - This mode prevents a trade by immediately expiring the potential maker order's remaining quantity.

EXPIRE_BOTH - This mode prevents a trade by immediately expiring both the taker and the potential maker orders' remaining quantities.

DECREMENT - This mode increases the prevented quantity of both orders by the amount of the prevented match. The smaller of the two orders will expire, or both if they have the same quantity.

The STP event will occur depending on the STP mode of the taker order. Thus, the STP mode of an order that goes on the book is no longer relevant and will be ignored for all future order processing.

Different accounts with the same tradeGroupId are considered part of the same "trade group". Orders submitted by members of a trade group are eligible for STP according to the taker-order's STP mode.

A user can confirm if their accounts are under the same tradeGroupId from the API either from GET /api/v3/account (REST API) or account.status (WebSocket API) for each account.

The field is also present in the response for GET /api/v3/preventedMatches (REST API) or myPreventedMatches (WebSocket API).

If the value is -1, then the tradeGroupId has not been set for that account, so the STP may only take place between orders of the same account.

When a self-trade is prevented, a prevented match is created. The orders in the prevented match have their prevented quantities increased and one or more orders expire.

This is not to be confused with a trade, as no orders will match.

This is a record of what orders could have self-traded.

This can be queried through the endpoint GET /api/v3/preventedMatches on the REST API or myPreventedMatches on the WebSocket API.

This is a sample of the output request for reference:

STP events expire quantity from open orders. The STP modes EXPIRE_TAKER, EXPIRE_MAKER, and EXPIRE_BOTH expire all remaining quantity on the affected orders, resulting in the entire open order being expired.

Prevented quantity is the amount of quantity that is expired due to STP events for a particular order. User stream execution reports for orders involved in STP may have these fields:

B is present for execution type TRADE_PREVENTION, and is the quantity expired due to that individual STP event.

A is the cumulative quantity expired due to STP over the lifetime of the order. For EXPIRE_TAKER, EXPIRE_MAKER, and EXPIRE_BOTH modes this will always be the same value as B.

API responses for orders which expired due to STP will also have a preventedQuantity field, indicating the cumulative quantity expired due to STP over the lifetime of the order.

While an order is open, the following equation holds true:

When an order's available quantity goes to zero, the order will be removed from the order book and the status will be one of EXPIRED_IN_MATCH, FILLED, or EXPIRED.

Symbols may be configured to allow different sets of STP modes and take different default STP modes.

defaultSelfTradePreventionMode - Orders will use this STP mode if the user does not provide one on order placement.

allowedSelfTradePreventionModes - Defines the allowed set of STP modes for order placement on that symbol.

For example, if a symbol has the following configuration:

Then that means if a user sends an order with no selfTradePreventionMode provided, then the order sent will have the value of NONE.

If a user wants to explicitly specify the mode they can pass the enum NONE, EXPIRE_TAKER, or EXPIRE_BOTH.

If a user tries to specify EXPIRE_MAKER for orders on this symbol, they will receive an error:

The order will have the status EXPIRED_IN_MATCH.

For all these cases, assume that all orders for these examples are made on the same account.

Scenario A- A user sends a new order with selfTradePreventionMode:NONE that will match with another order of theirs that is already on the book.

Result: No STP is triggered and the orders will match.

Order Status of the Maker Order

Order Status of the Taker Order

Scenario B- A user sends an order with EXPIRE_MAKER that would match with their orders that are already on the book.

Result: The orders that were on the book will expire due to STP, and the taker order will go on the book.

Output of the Taker Order

Scenario C - A user sends an order with EXPIRE_TAKER that would match with their orders already on the book.

Result: The orders already on the book will remain, while the taker order will expire.

Output of the Taker order

Scenario D- A user has an order on the book, and then sends an order with EXPIRE_BOTH that would match with the existing order.

Result: Both orders will expire.

Scenario E - A user has an order on the book with EXPIRE_MAKER, and then sends a new order with EXPIRE_TAKER which would match with the existing order.

Result: The taker order's STP mode will be used, so the taker order will be expired.

Scenario F - A user sends a market order with EXPIRE_MAKER which would match with an existing order.

Result: The existing order expires with the status EXPIRED_IN_MATCH, due to STP. The new order also expires but with status EXPIRED, due to low liquidity on the order book.

Scenario G- A user sends a limit order with DECREMENT which would match with an existing order.

Result: Both orders have a preventedQuantity of 2. Since this is the taker order’s full quantity, it expires due to STP.

**Examples:**

Example 1 (unknown):
```unknown
tradeGroupId
```

Example 2 (unknown):
```unknown
tradeGroupId
```

Example 3 (unknown):
```unknown
EXPIRE_TAKER
```

Example 4 (unknown):
```unknown
EXPIRE_MAKER
```

---

## Filters

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/filters

**Contents:**
- Filters
- Symbol filters​
  - PRICE_FILTER​
  - PERCENT_PRICE​
  - PERCENT_PRICE_BY_SIDE​
  - LOT_SIZE​
  - MIN_NOTIONAL​
  - NOTIONAL​
  - ICEBERG_PARTS​
  - MARKET_LOT_SIZE​

Filters define trading rules on a symbol or an exchange. Filters come in three forms: symbol filters, exchange filters and asset filters.

The PRICE_FILTER defines the price rules for a symbol. There are 3 parts:

Any of the above variables can be set to 0, which disables that rule in the price filter. In order to pass the price filter, the following must be true for price/stopPrice of the enabled rules:

/exchangeInfo format:

The PERCENT_PRICE filter defines the valid range for the price based on the average of the previous trades. avgPriceMins is the number of minutes the average price is calculated over. 0 means the last price is used.

In order to pass the percent price, the following must be true for price:

/exchangeInfo format:

The PERCENT_PRICE_BY_SIDE filter defines the valid range for the price based on the average of the previous trades. avgPriceMins is the number of minutes the average price is calculated over. 0 means the last price is used. There is a different range depending on whether the order is placed on the BUY side or the SELL side.

Buy orders will succeed on this filter if:

Sell orders will succeed on this filter if:

/exchangeInfo format:

The LOT_SIZE filter defines the quantity (aka "lots" in auction terms) rules for a symbol. There are 3 parts:

In order to pass the lot size, the following must be true for quantity/icebergQty:

/exchangeInfo format:

The MIN_NOTIONAL filter defines the minimum notional value allowed for an order on a symbol. An order's notional value is the price * quantity. applyToMarket determines whether or not the MIN_NOTIONAL filter will also be applied to MARKET orders. Since MARKET orders have no price, the average price is used over the last avgPriceMins minutes. avgPriceMins is the number of minutes the average price is calculated over. 0 means the last price is used.

/exchangeInfo format:

The NOTIONAL filter defines the acceptable notional range allowed for an order on a symbol. applyMinToMarket determines whether the minNotional will be applied to MARKET orders. applyMaxToMarket determines whether the maxNotional will be applied to MARKET orders.

In order to pass this filter, the notional (price * quantity) has to pass the following conditions:

For MARKET orders, the average price used over the last avgPriceMins minutes will be used for calculation. If the avgPriceMins is 0, then the last price will be used.

/exchangeInfo format:

The ICEBERG_PARTS filter defines the maximum parts an iceberg order can have. The number of ICEBERG_PARTS is defined as CEIL(qty / icebergQty).

/exchangeInfo format:

The MARKET_LOT_SIZE filter defines the quantity (aka "lots" in auction terms) rules for MARKET orders on a symbol. There are 3 parts:

In order to pass the market lot size, the following must be true for quantity:

/exchangeInfo format:

The MAX_NUM_ORDERS filter defines the maximum number of orders an account is allowed to have open on a symbol. Note that both "algo" orders and normal orders are counted for this filter.

/exchangeInfo format:

The MAX_NUM_ALGO_ORDERS filter defines the maximum number of "algo" orders an account is allowed to have open on a symbol. "Algo" orders are STOP_LOSS, STOP_LOSS_LIMIT, TAKE_PROFIT, and TAKE_PROFIT_LIMIT orders.

/exchangeInfo format:

The MAX_NUM_ICEBERG_ORDERS filter defines the maximum number of ICEBERG orders an account is allowed to have open on a symbol. An ICEBERG order is any order where the icebergQty is > 0.

/exchangeInfo format:

The MAX_POSITION filter defines the allowed maximum position an account can have on the base asset of a symbol. An account's position defined as the sum of the account's:

BUY orders will be rejected if the account's position is greater than the maximum position allowed.

If an order's quantity can cause the position to overflow, this will also fail the MAX_POSITION filter.

/exchangeInfo format:

The TRAILING_DELTA filter defines the minimum and maximum value for the parameter trailingDelta.

In order for a trailing stop order to pass this filter, the following must be true:

For STOP_LOSS BUY, STOP_LOSS_LIMIT_BUY,TAKE_PROFIT SELL and TAKE_PROFIT_LIMIT SELL orders:

For STOP_LOSS SELL, STOP_LOSS_LIMIT SELL, TAKE_PROFIT BUY, and TAKE_PROFIT_LIMIT BUY orders:

/exchangeInfo format:

The MAX_NUM_ORDER_AMENDS filter defines the maximum number of times an order can be amended on the given symbol.

If there are too many order amendments made on a single order, you will receive the -2038 error code.

/exchangeInfo format:

The MAX_NUM_ORDER_LISTS filter defines the maximum number of open order lists an account can have on a symbol. Note that OTOCOs count as one order list.

/exchangeInfo format:

The EXCHANGE_MAX_NUM_ORDERS filter defines the maximum number of orders an account is allowed to have open on the exchange. Note that both "algo" orders and normal orders are counted for this filter.

/exchangeInfo format:

The EXCHANGE_MAX_NUM_ALGO_ORDERS filter defines the maximum number of "algo" orders an account is allowed to have open on the exchange. "Algo" orders are STOP_LOSS, STOP_LOSS_LIMIT, TAKE_PROFIT, and TAKE_PROFIT_LIMIT orders.

/exchangeInfo format:

The EXCHANGE_MAX_NUM_ICEBERG_ORDERS filter defines the maximum number of iceberg orders an account is allowed to have open on the exchange.

/exchangeInfo format:

The EXCHANGE_MAX_NUM_ORDERS filter defines the maximum number of order lists an account is allowed to have open on the exchange. Note that OTOCOs count as one order list.

/exchangeInfo format:

The MAX_ASSET filter defines the maximum quantity of an asset that an account is allowed to transact in a single order.

**Examples:**

Example 1 (unknown):
```unknown
symbol filters
```

Example 2 (unknown):
```unknown
exchange filters
```

Example 3 (unknown):
```unknown
asset filters
```

Example 4 (unknown):
```unknown
PRICE_FILTER
```

---
