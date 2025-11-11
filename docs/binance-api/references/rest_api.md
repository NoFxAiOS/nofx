# Binance-Api - Rest Api

**Pages:** 22

---

## General Information on Endpoints

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/rest-api/general-information-on-endpoints

**Contents:**
- General Information on Endpoints

**Examples:**

Example 1 (unknown):
```unknown
query string
```

Example 2 (unknown):
```unknown
query string
```

Example 3 (unknown):
```unknown
request body
```

Example 4 (unknown):
```unknown
application/x-www-form-urlencoded
```

---

## CHANGELOG for Binance SPOT Testnet

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet

**Contents:**
- CHANGELOG for Binance SPOT Testnet
  - 2025-10-24​
    - SBE​
    - REST and WebSocket API​
  - 2025-10-17​
  - 2025-10-08​
    - FIX API​
  - 2025-10-01​
  - 2025-09-24​
  - 2025-09-18​

Last Updated: 2025-10-24

Note: All features here will only apply to the SPOT Testnet. This is not always synced with the live exchange.

Following the announcement from 2025-04-01, all documentation related with listenKey for use on wss://stream.binance.com has been removed.

Please refer to the list of requests and methods below for more information.

The features will remain available until a future retirement announcement is made.

Notice: The following changes will be enabled at 2025-10-17 07:00 UTC

Notice: The following changes will be enabled at 2025-10-08 07:00 UTC

All data on the Spot Test Network will be deleted today according to the periodic reset procedure. See F.A.Q. for more details.

REST and WebSocket API:

Notice: The following changes will be deployed on 2025-09-24, starting at 7:00 UTC and may take several hours to complete.

All data on the Spot Test Network will be deleted today according to the periodic reset procedure. See F.A.Q. for more details.

Notice: The following will be enabled on 2025-08-08, 07:00 UTC

Notice: The following changes will be deployed on 2025-08-06, starting 7:00 UTC and may take several hours to complete. Please consult the Spot Test Network's homepage to be informed of the release completion.

All data on the Spot Test Network will be deleted today according to the periodic reset procedure. (see F.A.Q. for more details)

All data on the Spot Test Network will be deleted today according to the periodic reset procedure. (see F.A.Q. for more details)

REST and WebSocket API:

Notice: The following changes will happen at 2025-05-21 7:00 UTC.

Notice: The following changes will be deployed tomorrow April 2, 2025 starting at 7:00 UTC and may take several hours to complete. Please consult the Spot Test Network's homepage to be informed of the release completion.

Note: These changes will be deployed live starting 2024-11-28 and may take several hours for all features to work as intended.

New Feature: Microsecond support:

The system now supports microseconds in all related time and/or timestamp fields. Microsecond support is opt-in, by default the requests and responses still use milliseconds. Examples in documentation are also using milliseconds for the foreseeable future.

Fixed a bug that prevented orders from being placed when submitting OCOs on the BUY side without providing a stopPrice.

TAKE_PROFIT and TAKE_PROFIT_LIMIT support has been added for OCOs.

Timestamp parameters now reject values too far into the past or the future. To be specific, the parameter will be rejected if:

If startTime and/or endTime values are outside of range, the values will be adjusted to fit the correct range.

The field for quote order quantity (origQuoteOrderQty) has been added to responses that previously did not have it. Note that for order placement endpoints the field will only appear for requests with newOrderRespType set to RESULT or FULL.

Note: This is in the process of being deployed. Please consult the Spot Test Network's homepage to be informed of the release completion.

Changes to Exchange Information (i.e. GET /api/v3/exchangeInfo from REST and exchangeInfo for WebSocket API).

REST and WebSocket API:

Note: This will be deployed starting around 7am UTC. Please consult the Spot Test Network's homepage to be informed of the release completion.

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

## API Key Types

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/faqs/api_key_types

**Contents:**
- API Key Types
  - Ed25519​
  - HMAC​
  - RSA​

Binance APIs require an API key to access authenticated endpoints for trading, account history, etc.

We support several types of API keys:

This document provides an overview of supported API keys.

We recommend to use Ed25519 API keys as it should provide the best performance and security out of all supported key types.

Read REST API or WebSocket API documentation to learn how to use different API keys.

Ed25519 keys use asymmetric cryptography. You share your public key with Binance and use the private key to sign API requests. Binance API uses the public key to verify your signature.

Ed25519 keys provide security comparable to 3072-bit RSA keys, but with considerably smaller key, smaller signature size, and faster signature computation.

We recommend to use Ed25519 API keys.

Sample Ed25519 signature:

HMAC keys use symmetric cryptography. Binance generates and shares with you a secret key which you use to sign API requests. Binance API uses the same shared secret key to verify your signature.

HMAC signatures are quick to compute and compact. However, the shared secret must be shared between multiple parties which is less secure than asymmetric cryptography used by Ed25519 or RSA keys.

HMAC keys are deprecated. We recommend to migrate to asymmetric API keys, such as Ed25519 or RSA.

Sample HMAC signature:

RSA keys use asymmetric cryptography. You share your public key with Binance and use the private key to sign API requests. Binance API uses the public key to verify your signature.

We support 2048 and 4096 bit RSA keys.

While RSA keys are more secure than HMAC keys, RSA signatures are much larger than HMAC and Ed25519 which can lead to a degradation to performance.

Sample RSA key (2048 bits):

Sample RSA signature (2048 bits):

**Examples:**

Example 1 (text):
```text
-----BEGIN PUBLIC KEY-----MCowBQYDK2VwAyEAgmDRTtj2FA+wzJUIlAL9ly1eovjLBu7uXUFR+jFULmg=-----END PUBLIC KEY-----
```

Example 2 (text):
```text
-----BEGIN PUBLIC KEY-----MCowBQYDK2VwAyEAgmDRTtj2FA+wzJUIlAL9ly1eovjLBu7uXUFR+jFULmg=-----END PUBLIC KEY-----
```

Example 3 (text):
```text
E7luAubOlcRxL10iQszvNCff+xJjwJrfajEHj1hOncmsgaSB4NE+A/BbQhCWwit/usNJ32/LeTwDYPoA7Qz4BA==
```

Example 4 (text):
```text
E7luAubOlcRxL10iQszvNCff+xJjwJrfajEHj1hOncmsgaSB4NE+A/BbQhCWwit/usNJ32/LeTwDYPoA7Qz4BA==
```

---

## Simple Binary Encoding (SBE) FAQ

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/faqs/sbe_faq

**Contents:**
- Simple Binary Encoding (SBE) FAQ
  - How to get an SBE response​
    - REST API​
    - WebSocket API​
  - Supported APIs​
  - SBE Schema​
  - Regarding Legacy support​
  - Generate SBE decoders:​
    - Decimal field encoding​
    - Timestamp field encoding​

The goal of this document is to explain:

SBE is a serialization format used for low-latency.

This implementation is based on the FIX SBE specification.

Sample request (REST):

Sample request (WebSocket):

REST API and WebSocket API for SPOT support SBE.

Unlike the FIX SBE specification, decimal fields have their mantissa and exponent fields encoded separately as primitive fields in order to minimize payload size and the number of encoded fields within messages.

Timestamps in SBE responses are in microseconds. This differs from JSON responses, which contain millisecond timestamps by default.

A few field attributes prefixed with mbx: were added to the schema file for documentation purposes:

**Examples:**

Example 1 (unknown):
```unknown
application/sbe
```

Example 2 (unknown):
```unknown
<ID>:<VERSION>
```

Example 3 (text):
```text
curl -sX GET -H "Accept: application/sbe" -H "X-MBX-SBE: 1:0" 'https://api.binance.com/api/v3/exchangeInfo?symbol=BTCUSDT'
```

Example 4 (text):
```text
curl -sX GET -H "Accept: application/sbe" -H "X-MBX-SBE: 1:0" 'https://api.binance.com/api/v3/exchangeInfo?symbol=BTCUSDT'
```

---

## HTTP Return Codes

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/rest-api/http-return-codes

**Contents:**
- HTTP Return Codes

---

## HTTP Return Codes

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/rest-api/http-return-codes

**Contents:**
- HTTP Return Codes

---

## General endpoints

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/rest-api/general-endpoints

**Contents:**
- General endpoints
  - Test connectivity​
  - Check server time​
  - Exchange information​

Test connectivity to the Rest API.

Test connectivity to the Rest API and get the current server time.

Current exchange trading rules and symbol information

Examples of Symbol Permissions Interpretation from the Response:

**Examples:**

Example 1 (text):
```text
GET /api/v3/ping
```

Example 2 (text):
```text
GET /api/v3/ping
```

Example 3 (text):
```text
GET /api/v3/time
```

Example 4 (text):
```text
GET /api/v3/time
```

---

## SPOT Testnet Terms of Use

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/TESTNET-TERMS-OF-USE

**Contents:**
- SPOT Testnet Terms of Use

The Binance Spot Testnet and Futures Testnet are subject to the Testnet Terms of Use. Please read it carefully before proceeding.

---

## FIX API

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/fix-api

**Contents:**
- FIX API
- General API Information​
  - FIX API Order Entry sessions​
  - FIX API Drop Copy sessions​
  - FIX API Market Data sessions​
  - FIX Connection Lifecycle​
  - API Key Permissions​
  - On message processing order​
  - Response Mode​
  - Timing Security​

[!NOTE] This API can only be used with the SPOT Exchange.

FIX sessions only support Ed25519 keys. You can setup and configure your API key permissions on Spot Test Network.

To access the FIX API order entry sessions, your API key must be configured with the FIX_API permission.

To access the FIX Drop Copy sessions, your API key must be configured with either FIX_API_READ_ONLY or FIX_API permission.

To access the FIX Market Data sessions, your API key must be configured with either FIX_API or FIX_API_READ_ONLY permission.

FIX sessions only support Ed25519 keys.

The MessageHandling (25035) field required in the initial Logon<A> message controls whether messages from the client may be reordered before they are processed by the Matching Engine.

In all modes, the client's MsgSeqNum (34) must increase monotonically, with each subsequent message having a sequence number that is exactly 1 greater than the previous message.

[!TIP] UNORDERED(1) should offer better performance when there are multiple messages in flight from the client to the server.

By default, all concurrent order entry sessions receive all of the account's successful ExecutionReport<8> and ListStatus<N> messages, including those in response to orders placed from other FIX sessions and via non-FIX APIs.

Use the ResponseMode (25036) field in the initial Logon<A> message to change this behavior.

The Logon<A> message authenticates your connection to the FIX API. This must be the first message sent by the client.

The signature payload is a text string constructed by concatenating the values of the following fields in this exact order, separated by the SOH character:

Sign the payload using your private key. Encode the signature with base64. The resulting text string is the value of the RawData (96) field.

Here is a sample Python code implementing the signature algorithm:

The values presented below can be used to validate the correctness of the signature computation implementation:

The Ed25519 private key used in the example computation is shown below:

[!CAUTION] The following secret key is provided solely for illustrative purposes. Do not use this key in any real-world application as it is not secure and may compromise your cryptographic implementation. Always generate your own unique and secure keys for actual use.

Resulting Logon <A> message:

Client messages that contain syntax errors, missing required fields, or refer to unknown symbols will be rejected by the server with a Reject <3> message.

If a valid message cannot be processed and is rejected, an appropriate reject response will be sent. Please refer to the individual message documentation for possible responses.

Please refer to the Text (58) and ErrorCode (25016) fields in responses for the reject reason.

The list of error codes can be found on the Error codes page.

Only printable ASCII characters and SOH are supported.

Supported UTCTIMESTAMP formats:

Client order ID fields must conform to the regex ^[a-zA-Z0-9-_]{1,36}$:

[!NOTE] In example messages, the | character is used to represent SOH character:

Appears at the start of every message.

Appears at the end of every message.

Sent by the server if there is no outgoing traffic during the heartbeat interval (HeartBtInt (108) in Logon<A>).

Sent by the client to indicate that the session is healthy.

Sent by the client or the server in response to a TestRequest<1> message.

Sent by the server if there is no incoming traffic during the heartbeat interval (HeartBtInt (108) in Logon<A>).

Sent by the client to request a Heartbeat<0> response.

[!NOTE] If the client does not respond to TestRequest<1> with Heartbeat<0> with a correct TestReqID (112) within timeout, the connection will be dropped.

Sent by the server in response to an invalid message that cannot be processed.

Sent by the server if a new connection cannot be accepted. Please refer to Connection Limits.

Please refer to the Text (58) and ErrorCode (25016) fields for the reject reason.

Sent by the client to authenticate the connection. Logon<A> must be the first message sent by the client.

Sent by the server in response to a successful logon.

[!NOTE] Logon<A> can only be sent once for the entirety of the session.

Sent to initiate the process of closing the connection, and also when responding to Logout.

When the server enters maintenance, a News message will be sent to clients every 10 seconds for 10 minutes. After this period, clients will be logged out and their sessions will be closed.

Upon receiving this message, clients are expected to establish a new session and close the old one.

The countdown message sent will be:

When there are 10 seconds remaining, the following message will be sent:

If the client does not close the old session within 10 seconds of receiving the above message, the server will log it out and close the session.

Resend requests are currently not supported.

[!NOTE] The messages below can only be used for the FIX Order Entry and FIX Drop Copy Sessions.

Sent by the client to submit a new order for execution.

This adds 1 order to the EXCHANGE_MAX_ORDERS filter and the MAX_NUM_ORDERS filter.

Unfilled Order Count: 1

Please refer to Supported Order Types for supported field combinations.

[!NOTE] Many fields become required based on the order type. Please refer to Supported Order Types.

Required fields based on Binance OrderType:

Sent by the server whenever an order state changes.

Sent by the client to cancel an order or an order list.

If the canceled order is part of an order list, the entire list will be canceled.

Sent by the server when OrderCancelRequest<F> has failed.

Sent by the client to cancel an order and submit a new one for execution.

Filters and Order Count are evaluated before the processing of the cancellation and order placement occurs.

A new order that was not attempted (i.e. when newOrderResult: NOT_ATTEMPTED), will still increase the unfilled order count by 1.

Unfilled Order Count: 1

Please refer to Supported Order Types for supported field combinations when describing the new order.

[!NOTE] Cancel is always processed first. Then immediately after that the new order is submitted.

Sent by the client to cancel all open orders on a symbol.

[!NOTE] All orders of the account will be canceled, including those placed in different connections.

Sent by the server in response to OrderMassCancelRequest<q>.

Sent by the client to submit a list of orders for execution.

Unfilled Order Count:

Orders in an order list are contingent on one another. Please refer to Supported Order List Types for supported order types and triggering instructions.

[!NOTE] Orders must be specified in the sequence indicated in the Order Names column in the table below.

Sent by the server whenever an order list state changes.

[!NOTE] By default, ListStatus<N> is sent for all order lists of an account, including those submitted in different connections. Please see Response Mode for other behavior options.

Sent by the client to reduce the original quantity of their order.

This adds 0 orders to the EXCHANGE_MAX_ORDERS filter and the MAX_NUM_ORDERS filter.

Unfilled Order Count: 0

Read Order Amend Keep Priority FAQ to learn more.

Sent by the server when the OrderAmendKeepPriorityRequest <XAK> has failed.

Sent by the client to query current limits.

Sent by the server in response to LimitQuery<XLQ>.

[!NOTE] The messages below can only be used for the FIX Market Data.

Sent by the client to query information about active instruments (i.e., those that have the TRADING status). If used for an inactive instrument, it will be responded to with a Reject<3>.

Sent by the server in a response to the InstrumentListRequest<x>.

[!NOTE] More detailed symbol information is available through the exchangeInfo endpoint.

Sent by the client to subscribe to or unsubscribe from market data stream.

The Trade Streams push raw trade information; each trade has a unique buyer and seller.

Fields required to subscribe:

Update Speed: Real-time

Individual Symbol Book Ticker Stream

Pushes any update to the best bid or offers price or quantity in real-time for a specified symbol.

Fields required to subscribe:

Update Speed: Real-time

[!NOTE] In the Individual Symbol Book Ticker Stream, when MDUpdateAction is set to CHANGE(1) in a MarketDataIncrementalRefresh<X> message sent from the server, it replaces the previous best quote.

Order book price and quantity depth updates used to locally manage an order book.

Fields required to subscribe:

[!NOTE] Since the MarketDataSnapshot<W> have a limit on the number of price levels (5000 on each side maximum), you won't learn the quantities for the levels outside of the initial snapshot unless they change. So be careful when using the information for those levels, since they might not reflect the full view of the order book. However, for most use cases, seeing 5000 levels on each side is enough to understand the market and trade effectively.

Sent by the server in a response to an invalid MarketDataRequest <V>.

Sent by the server in response to a MarketDataRequest<V>, activating Individual Symbol Book Ticker Stream or Diff. Depth Stream subscriptions.

Sent by the server when there is a change in a subscribed stream.

Sample fragmented messages:

[!NOTE] Below are example messages, with NoMDEntry limited to 2, In the real streams, the NoMDEntry is limited to 10000.

**Examples:**

Example 1 (unknown):
```unknown
tcp+tls://fix-oe.testnet.binance.vision:9000
```

Example 2 (unknown):
```unknown
tcp+tls://fix-dc.testnet.binance.vision:9000
```

Example 3 (unknown):
```unknown
FIX_API_READ_ONLY
```

Example 4 (unknown):
```unknown
tcp+tls://fix-md.testnet.binance.vision:9000
```

---

## Data Sources

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/rest-api/data-sources

**Contents:**
- Data Sources

These are the three sources, ordered by least to most potential for delays in data updates.

Some endpoints can have more than 1 data source. (e.g. Memory => Database) This means that the endpoint will check the first Data Source, and if it cannot find the value it's looking for it will check the next one.

---

## Request Security

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/rest-api/request-security

**Contents:**
- Request Security
  - SIGNED Endpoint security​
  - Timing security​
  - SIGNED Endpoint Examples for POST /api/v3/order​
    - HMAC Keys​
    - RSA Keys​
    - Ed25519 Keys​

Serious trading is about timing. Networks can be unstable and unreliable, which can lead to requests taking varying amounts of time to reach the servers. With recvWindow, you can specify that the request must be processed within a certain number of milliseconds or be rejected by the server.

It is recommended to use a small recvWindow of 5000 or less!

Here is a step-by-step example of how to send a valid signed payload from the Linux command line using echo, openssl, and curl.

Example 1: As a request body

requestBody: symbol=LTCBTC&side=BUY&type=LIMIT&timeInForce=GTC&quantity=1&price=0.1&recvWindow=5000&timestamp=1499827319559

HMAC SHA256 signature:

Example 2: As a query string

queryString: symbol=LTCBTC&side=BUY&type=LIMIT&timeInForce=GTC&quantity=1&price=0.1&recvWindow=5000&timestamp=1499827319559

HMAC SHA256 signature:

Example 3: Mixed query string and request body

queryString: symbol=LTCBTC&side=BUY&type=LIMIT&timeInForce=GTC

requestBody: quantity=1&price=0.1&recvWindow=5000&timestamp=1499827319559

HMAC SHA256 signature:

Note that the signature is different in example 3. There is no & between "GTC" and "quantity=1".

This will be a step by step process how to create the signature payload to send a valid signed payload.

We support PKCS#8 currently.

To get your API key, you need to upload your RSA Public Key to your account and a corresponding API key will be provided for you.

For this example, the private key will be referenced as ./test-prv-key.pem

Step 1: Construct the payload

Arrange the list of parameters into a string. Separate each parameter with a &.

For the parameters above, the signature payload would look like this:

Step 2: Compute the signature:

A sample Bash script below does the similar steps said above.

Note: It is highly recommended to use Ed25519 API keys as it should provide the best performance and security out of all supported key types.

This is a sample code in Python to show how to sign the payload with an Ed25519 key.

**Examples:**

Example 1 (unknown):
```unknown
USER_STREAM
```

Example 2 (unknown):
```unknown
query string
```

Example 3 (unknown):
```unknown
request body
```

Example 4 (javascript):
```javascript
serverTime = getCurrentTime()if (timestamp < (serverTime + 1 second) && (serverTime - timestamp) <= recvWindow) {  // begin processing request  serverTime = getCurrentTime()  if (serverTime - timestamp) <= recvWindow {    // forward request to Matching Engine  } else {    // reject request  }  // finish processing request} else {  // reject request}
```

---

## CHANGELOG for Binance SPOT Testnet

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/

**Contents:**
- CHANGELOG for Binance SPOT Testnet
  - 2025-10-24​
    - SBE​
    - REST and WebSocket API​
  - 2025-10-17​
  - 2025-10-08​
    - FIX API​
  - 2025-10-01​
  - 2025-09-24​
  - 2025-09-18​

Last Updated: 2025-10-24

Note: All features here will only apply to the SPOT Testnet. This is not always synced with the live exchange.

Following the announcement from 2025-04-01, all documentation related with listenKey for use on wss://stream.binance.com has been removed.

Please refer to the list of requests and methods below for more information.

The features will remain available until a future retirement announcement is made.

Notice: The following changes will be enabled at 2025-10-17 07:00 UTC

Notice: The following changes will be enabled at 2025-10-08 07:00 UTC

All data on the Spot Test Network will be deleted today according to the periodic reset procedure. See F.A.Q. for more details.

REST and WebSocket API:

Notice: The following changes will be deployed on 2025-09-24, starting at 7:00 UTC and may take several hours to complete.

All data on the Spot Test Network will be deleted today according to the periodic reset procedure. See F.A.Q. for more details.

Notice: The following will be enabled on 2025-08-08, 07:00 UTC

Notice: The following changes will be deployed on 2025-08-06, starting 7:00 UTC and may take several hours to complete. Please consult the Spot Test Network's homepage to be informed of the release completion.

All data on the Spot Test Network will be deleted today according to the periodic reset procedure. (see F.A.Q. for more details)

All data on the Spot Test Network will be deleted today according to the periodic reset procedure. (see F.A.Q. for more details)

REST and WebSocket API:

Notice: The following changes will happen at 2025-05-21 7:00 UTC.

Notice: The following changes will be deployed tomorrow April 2, 2025 starting at 7:00 UTC and may take several hours to complete. Please consult the Spot Test Network's homepage to be informed of the release completion.

Note: These changes will be deployed live starting 2024-11-28 and may take several hours for all features to work as intended.

New Feature: Microsecond support:

The system now supports microseconds in all related time and/or timestamp fields. Microsecond support is opt-in, by default the requests and responses still use milliseconds. Examples in documentation are also using milliseconds for the foreseeable future.

Fixed a bug that prevented orders from being placed when submitting OCOs on the BUY side without providing a stopPrice.

TAKE_PROFIT and TAKE_PROFIT_LIMIT support has been added for OCOs.

Timestamp parameters now reject values too far into the past or the future. To be specific, the parameter will be rejected if:

If startTime and/or endTime values are outside of range, the values will be adjusted to fit the correct range.

The field for quote order quantity (origQuoteOrderQty) has been added to responses that previously did not have it. Note that for order placement endpoints the field will only appear for requests with newOrderRespType set to RESULT or FULL.

Note: This is in the process of being deployed. Please consult the Spot Test Network's homepage to be informed of the release completion.

Changes to Exchange Information (i.e. GET /api/v3/exchangeInfo from REST and exchangeInfo for WebSocket API).

REST and WebSocket API:

Note: This will be deployed starting around 7am UTC. Please consult the Spot Test Network's homepage to be informed of the release completion.

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

## General API Information

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/rest-api/general-api-information

**Contents:**
- General API Information

**Examples:**

Example 1 (unknown):
```unknown
X-MBX-TIME-UNIT:MICROSECOND
```

Example 2 (unknown):
```unknown
X-MBX-TIME-UNIT:microsecond
```

---

## General API Information

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/rest-api/general-api-information

**Contents:**
- General API Information

**Examples:**

Example 1 (unknown):
```unknown
X-MBX-TIME-UNIT:MICROSECOND
```

Example 2 (unknown):
```unknown
X-MBX-TIME-UNIT:microsecond
```

---

## Request Security

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/rest-api/request-security

**Contents:**
- Request Security
  - SIGNED Endpoint security​
  - Timing security​
  - SIGNED Endpoint Examples for POST /api/v3/order​
    - HMAC Keys​
    - RSA Keys​
    - Ed25519 Keys​

Serious trading is about timing. Networks can be unstable and unreliable, which can lead to requests taking varying amounts of time to reach the servers. With recvWindow, you can specify that the request must be processed within a certain number of milliseconds or be rejected by the server.

It is recommended to use a small recvWindow of 5000 or less! The max cannot go beyond 60,000!

Here is a step-by-step example of how to send a valid signed payload from the Linux command line using echo, openssl, and curl.

Example 1: As a request body

requestBody: symbol=LTCBTC&side=BUY&type=LIMIT&timeInForce=GTC&quantity=1&price=0.1&recvWindow=5000&timestamp=1499827319559

HMAC SHA256 signature:

Example 2: As a query string

queryString: symbol=LTCBTC&side=BUY&type=LIMIT&timeInForce=GTC&quantity=1&price=0.1&recvWindow=5000&timestamp=1499827319559

HMAC SHA256 signature:

Example 3: Mixed query string and request body

queryString: symbol=LTCBTC&side=BUY&type=LIMIT&timeInForce=GTC

requestBody: quantity=1&price=0.1&recvWindow=5000&timestamp=1499827319559

HMAC SHA256 signature:

Note that the signature is different in example 3. There is no & between "GTC" and "quantity=1".

This will be a step by step process how to create the signature payload to send a valid signed payload.

We support PKCS#8 currently.

To get your API key, you need to upload your RSA Public Key to your account and a corresponding API key will be provided for you.

For this example, the private key will be referenced as ./test-prv-key.pem

Step 1: Construct the payload

Arrange the list of parameters into a string. Separate each parameter with a &.

For the parameters above, the signature payload would look like this:

Step 2: Compute the signature:

A sample Bash script below does the similar steps said above.

Note: It is highly recommended to use Ed25519 API keys as it should provide the best performance and security out of all supported key types.

This is a sample code in Python to show how to sign the payload with an Ed25519 key.

**Examples:**

Example 1 (unknown):
```unknown
USER_STREAM
```

Example 2 (unknown):
```unknown
query string
```

Example 3 (unknown):
```unknown
request body
```

Example 4 (javascript):
```javascript
serverTime = getCurrentTime()if (timestamp < (serverTime + 1 second) && (serverTime - timestamp) <= recvWindow) {  // begin processing request  serverTime = getCurrentTime()  if (serverTime - timestamp) <= recvWindow {    // forward request to Matching Engine  } else {    // reject request  }  // finish processing request} else {  // reject request}
```

---

## SPOT Exchange Terms of Use

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/PROD-TERMS-OF-USE

**Contents:**
- SPOT Exchange Terms of Use

Binance products and services are subject to the Product Terms of Use. Please read it carefully before proceeding.

---

## FIX API

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/fix-api

**Contents:**
- FIX API
- General API Information​
  - FIX API Order Entry sessions​
  - FIX API Drop Copy sessions​
  - FIX API Market Data sessions​
  - FIX Connection Lifecycle​
  - API Key Permissions​
  - On message processing order​
  - Response Mode​
  - Timing Security​

[!NOTE] This API can only be used with the SPOT Exchange.

FIX sessions only support Ed25519 keys.

Please refer to this tutorial on how to set up an Ed25519 key pair.

To access the FIX API order entry sessions, your API key must be configured with the FIX_API permission.

To access the FIX Drop Copy sessions, your API key must be configured with either FIX_API_READ_ONLY or FIX_API permission.

To access the FIX Market Data sessions, your API key must be configured with either FIX_API or FIX_API_READ_ONLY permission.

FIX sessions only support Ed25519 keys.

Please refer to this tutorial on how to set up an Ed25519 key pair.

The MessageHandling (25035) field required in the initial Logon<A> message controls whether messages from the client may be reordered before they are processed by the Matching Engine.

In all modes, the client's MsgSeqNum (34) must increase monotonically, with each subsequent message having a sequence number that is exactly 1 greater than the previous message.

[!TIP] UNORDERED(1) should offer better performance when there are multiple messages in flight from the client to the server.

By default, all concurrent order entry sessions receive all of the account's successful ExecutionReport<8> and ListStatus<N> messages, including those in response to orders placed from other FIX sessions and via non-FIX APIs.

Use the ResponseMode (25036) field in the initial Logon<A> message to change this behavior.

The Logon<A> message authenticates your connection to the FIX API. This must be the first message sent by the client.

The signature payload is a text string constructed by concatenating the values of the following fields in this exact order, separated by the SOH character:

Sign the payload using your private key. Encode the signature with base64. The resulting text string is the value of the RawData (96) field.

Here is a sample Python code implementing the signature algorithm:

The values presented below can be used to validate the correctness of the signature computation implementation:

The Ed25519 private key used in the example computation is shown below:

[!CAUTION] The following secret key is provided solely for illustrative purposes. Do not use this key in any real-world application as it is not secure and may compromise your cryptographic implementation. Always generate your own unique and secure keys for actual use.

Resulting Logon <A> message:

Client messages that contain syntax errors, missing required fields, or refer to unknown symbols will be rejected by the server with a Reject <3> message.

If a valid message cannot be processed and is rejected, an appropriate reject response will be sent. Please refer to the individual message documentation for possible responses.

Please refer to the Text (58) and ErrorCode (25016) fields in responses for the reject reason.

The list of error codes can be found on the Error codes page.

Only printable ASCII characters and SOH are supported.

Supported UTCTIMESTAMP formats:

Client order ID fields must conform to the regex ^[a-zA-Z0-9-_]{1,36}$:

[!NOTE] In example messages, the | character is used to represent SOH character:

Appears at the start of every message.

Appears at the end of every message.

Sent by the server if there is no outgoing traffic during the heartbeat interval (HeartBtInt (108) in Logon<A>).

Sent by the client to indicate that the session is healthy.

Sent by the client or the server in response to a TestRequest<1> message.

Sent by the server if there is no incoming traffic during the heartbeat interval (HeartBtInt (108) in Logon<A>).

Sent by the client to request a Heartbeat<0> response.

[!NOTE] If the client does not respond to TestRequest<1> with Heartbeat<0> with a correct TestReqID (112) within timeout, the connection will be dropped.

Sent by the server in response to an invalid message that cannot be processed.

Sent by the server if a new connection cannot be accepted. Please refer to Connection Limits.

Please refer to the Text (58) and ErrorCode (25016) fields for the reject reason.

Sent by the client to authenticate the connection. Logon<A> must be the first message sent by the client.

Sent by the server in response to a successful logon.

[!NOTE] Logon<A> can only be sent once for the entirety of the session.

Sent to initiate the process of closing the connection, and also when responding to Logout.

When the server enters maintenance, a News message will be sent to clients every 10 seconds for 10 minutes. After this period, clients will be logged out and their sessions will be closed.

Upon receiving this message, clients are expected to establish a new session and close the old one.

The countdown message sent will be:

When there are 10 seconds remaining, the following message will be sent:

If the client does not close the old session within 10 seconds of receiving the above message, the server will log it out and close the session.

Resend requests are currently not supported.

[!NOTE] The messages below can only be used for the FIX Order Entry and FIX Drop Copy Sessions.

Sent by the client to submit a new order for execution.

This adds 1 order to the EXCHANGE_MAX_ORDERS filter and the MAX_NUM_ORDERS filter.

Unfilled Order Count: 1

Please refer to Supported Order Types for supported field combinations.

[!NOTE] Many fields become required based on the order type. Please refer to Supported Order Types.

Required fields based on Binance OrderType:

Sent by the server whenever an order state changes.

Sent by the client to cancel an order or an order list.

If the canceled order is part of an order list, the entire list will be canceled.

Sent by the server when OrderCancelRequest<F> has failed.

Sent by the client to cancel an order and submit a new one for execution.

Filters and Order Count are evaluated before the processing of the cancellation and order placement occurs.

A new order that was not attempted (i.e. when newOrderResult: NOT_ATTEMPTED), will still increase the unfilled order count by 1.

Unfilled Order Count: 1

Please refer to Supported Order Types for supported field combinations when describing the new order.

[!NOTE] Cancel is always processed first. Then immediately after that the new order is submitted.

Sent by the client to cancel all open orders on a symbol.

[!NOTE] All orders of the account will be canceled, including those placed in different connections.

Sent by the server in response to OrderMassCancelRequest<q>.

Sent by the client to submit a list of orders for execution.

Unfilled Order Count:

Orders in an order list are contingent on one another. Please refer to Supported Order List Types for supported order types and triggering instructions.

[!NOTE] Orders must be specified in the sequence indicated in the Order Names column in the table below.

Sent by the server whenever an order list state changes.

[!NOTE] By default, ListStatus<N> is sent for all order lists of an account, including those submitted in different connections. Please see Response Mode for other behavior options.

Sent by the client to reduce the original quantity of their order.

This adds 0 orders to the EXCHANGE_MAX_ORDERS filter and the MAX_NUM_ORDERS filter.

Unfilled Order Count: 0

Read Order Amend Keep Priority FAQ to learn more.

Sent by the server when the OrderAmendKeepPriorityRequest <XAK> has failed.

Sent by the client to query current limits.

Sent by the server in response to LimitQuery<XLQ>.

[!NOTE] The messages below can only be used for the FIX Market Data.

Sent by the client to query information about active instruments (i.e., those that have the TRADING status). If used for an inactive instrument, it will be responded to with a Reject<3>.

Sent by the server in a response to the InstrumentListRequest<x>.

[!NOTE] More detailed symbol information is available through the exchangeInfo endpoint.

Sent by the client to subscribe to or unsubscribe from market data stream.

The Trade Streams push raw trade information; each trade has a unique buyer and seller.

Fields required to subscribe:

Update Speed: Real-time

Individual Symbol Book Ticker Stream

Pushes any update to the best bid or offers price or quantity in real-time for a specified symbol.

Fields required to subscribe:

Update Speed: Real-time

[!NOTE] In the Individual Symbol Book Ticker Stream, when MDUpdateAction is set to CHANGE(1) in a MarketDataIncrementalRefresh<X> message sent from the server, it replaces the previous best quote.

Order book price and quantity depth updates used to locally manage an order book.

Fields required to subscribe:

[!NOTE] Since the MarketDataSnapshot<W> have a limit on the number of price levels (5000 on each side maximum), you won't learn the quantities for the levels outside of the initial snapshot unless they change. So be careful when using the information for those levels, since they might not reflect the full view of the order book. However, for most use cases, seeing 5000 levels on each side is enough to understand the market and trade effectively.

Sent by the server in a response to an invalid MarketDataRequest <V>.

Sent by the server in response to a MarketDataRequest<V>, activating Individual Symbol Book Ticker Stream or Diff. Depth Stream subscriptions.

Sent by the server when there is a change in a subscribed stream.

Sample fragmented messages:

[!NOTE] Below are example messages, with NoMDEntry limited to 2, In the real streams, the NoMDEntry is limited to 10000.

**Examples:**

Example 1 (unknown):
```unknown
tcp+tls://fix-oe.binance.com:9000
```

Example 2 (unknown):
```unknown
tcp+tls://fix-dc.binance.com:9000
```

Example 3 (unknown):
```unknown
FIX_API_READ_ONLY
```

Example 4 (unknown):
```unknown
tcp+tls://fix-md.binance.com:9000
```

---

## General API Information

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/rest-api

**Contents:**
- General API Information

**Examples:**

Example 1 (unknown):
```unknown
X-MBX-TIME-UNIT:MICROSECOND
```

Example 2 (unknown):
```unknown
X-MBX-TIME-UNIT:microsecond
```

---

## General endpoints

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/rest-api/general-endpoints

**Contents:**
- General endpoints
  - Test connectivity​
  - Check server time​
  - Exchange information​

Test connectivity to the Rest API.

Test connectivity to the Rest API and get the current server time.

Current exchange trading rules and symbol information

Examples of Symbol Permissions Interpretation from the Response:

**Examples:**

Example 1 (text):
```text
GET /api/v3/ping
```

Example 2 (text):
```text
GET /api/v3/ping
```

Example 3 (text):
```text
GET /api/v3/time
```

Example 4 (text):
```text
GET /api/v3/time
```

---

## General Information on Endpoints

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/rest-api/general-information-on-endpoints

**Contents:**
- General Information on Endpoints

**Examples:**

Example 1 (unknown):
```unknown
query string
```

Example 2 (unknown):
```unknown
query string
```

Example 3 (unknown):
```unknown
request body
```

Example 4 (unknown):
```unknown
application/x-www-form-urlencoded
```

---

## Data Sources

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/rest-api/data-sources

**Contents:**
- Data Sources

These are the three sources, ordered by least to most potential for delays in data updates.

Some endpoints can have more than 1 data source. (e.g. Memory => Database) This means that the endpoint will check the first Data Source, and if it cannot find the value it's looking for it will check the next one.

---

## Testnet

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/faqs/testnet

**Contents:**
- Testnet
- F.A.Q.
  - How can I use the Spot Test Network? ​
  - Can I use the /sapi endpoints on the Spot Test Network? ​
  - How to get funds in/out of the Spot Test Network? ​
  - What are the restrictions on the Spot Test Network? ​
  - All my data has disappeared! What happened? ​
  - What is the difference between klines and uiKlines? ​
  - What are RSA API Keys? ​
  - What type of RSA keys are supported? ​

Step 1: Log in on this website, and generate an API Key.

Step 2: Follow the official documentation of the Spot API, replacing the URLs of the endpoints with the following values:

No, only the /api endpoints are available on the Spot Test Network:

All users registering on the Spot Test Network automatically receive a balance in many different assets. Please note that these are not real assets and can be used only on the Spot Test Network itself.

All funds on the Spot Test Network are virtual, and can not be transferred in/out of the Spot Test Network.

IP Limits, Order Rate Limits, Exchange Filters and Symbol Filters on the Spot Test Network are generally the same as on the Spot API.

All users are encouraged to regularly query the API to get the most up-to-date rate limits & filters, for example by doing:

The Spot Test Network is periodically reset to a blank state. That includes all pending and executed orders. During that reset procedure, all users automatically receive a fresh allowance of all assets.

These resets happen approximately once per month, and we do not offer prior notification for them.

Starting from August 2020, API Keys are preserved during resets. Users no longer need to re-register new API Keys after a reset.

On the Spot Test Network, these 2 requests always return the same data.

RSA API Keys are an alternative to the typical HMAC-SHA-256 API Keys that are used to authenticate your requests on the Spot API.

Unlike HMAC-SHA-256 API Keys where we generate the secret signing key for you, with RSA API Keys, *you* generate a pair of public+private RSA keys, send us the public key, and sign your requests with your private key.

We support RSA keys of any length from 2048 bits up to 4096 bits. We recommend 2048 bits keys as a good balance between security and signature speed.

When generating the RSA signature, use the PKCS#1 v1.5 signature scheme. This is the default when using OpenSSL. We currently do not support the PSS signature scheme.

Step 1: Generate the private key test-prv-key.pem. Do not share this file with anyone!

Step 2: Generate the public key test-pub-key.pem from the private key.

The public key should look something like this:

Step 3: Register your public key on the Spot Test Network.

During registration, we will generate an API Key for you that you will have to put in the X-MBX-APIKEY header of your requests, exactly the same way as you would do for HMAC-SHA-256 API Keys.

Step 4: When you send a request to the Spot Test Network, sign the payload using your private key.

Here is an example Bash script to post a new order and sign the request using OpenSSL. You can adapt it to your favorite programming language:

Ed25519 API keys are an alternative to RSA API keys, using asymmetric cryptography to authenticate your requests on the Spot API.

Like RSA API keys, Ed25519 keys are asymmetric: you generate a keypair, share the public key with Binance, and use your private key to sign requests.

Ed25519 digital signature scheme provides security comparable to 3072-bit RSA keys, while having much smaller signatures that are faster to compute:

Step 1: Generate the private key test-prv-key.pem. Do not share this file with anyone!

Step 2: Compute the public key test-pub-key.pem from the private key.

The public key should look something like this:

Step 3: Register your public key on the Spot Test Network.

During registration, we will generate an API key for you. Please put it in the X-MBX-APIKEY header of your requests, exactly the same way as with other API key types.

Step 4: When you send a request to the Spot Test Network, sign the payload using your private key.

Here is an example in Python that posts a new order signed with Ed25519 key. You can adapt it to your favorite programming language.

**Examples:**

Example 1 (unknown):
```unknown
test-prv-key.pem
```

Example 2 (unknown):
```unknown
test-pub-key.pem
```

Example 3 (text):
```text
-----BEGIN PUBLIC KEY-----bL4DUXwR3ijFSXzcecQtVFU1zVWcSQd0Meztl3DLX42l/8EALJx3LSz9YKS0PMQWMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAv9ij99RAJM4JLl8Rg47bdJXMrv84WL1OK/gid4hCnxo083LYLXUpIqMmL+O6fmXAvsvkyMyT520Cw0ZNCrUkWoCjGE4JZZGF4wOkWdF37JFWbDnE/GF5mAykKj+OMaECBlZ207KleQqgVzHjKuCbhPMuBVVD3IhjBfIc7EEM438LbtayMDx4dviPWwm127jwn8qd9H3kv5JBoDfsdYMB3k39r724CljqlAfX33GpbV2LvEkL6Da3OFk+grfN98X2pCBRz5+1N95I2cRD7o+jwtCr+65E+Gqjo4OI60F9Gq5GDcrnudnUw13a4zwlU6W+Cy8gJ4R0CcKTc4+VhYVX5wW2tzLVnDqvjIN8hjhgtmUv8hr19Wn+42ev+5sNtO5QAS6sJMJG5D+cpxCNhei1Xm+1zXliaA1fvVYRqon2MdHcedFeAjzVtX38+Xweytowydcq2V/9pUUNZIzUqX7tZr3F+Ao3QOb/CuWbUBpUcbXfGv7AI1ozP8LRByyu6O8Z1dZNdkdjWVt83maUrIJHjjc7jlZY9JbH6EyYV5TenjJaupvdlx72vA7Fcgevx87seog2JALAJqZQNT+t9/tmrTUSEp3t4aINKUC1QC0CYKECAwEAAQ==-----END PUBLIC KEY-----
```

Example 4 (text):
```text
-----BEGIN PUBLIC KEY-----bL4DUXwR3ijFSXzcecQtVFU1zVWcSQd0Meztl3DLX42l/8EALJx3LSz9YKS0PMQWMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAv9ij99RAJM4JLl8Rg47bdJXMrv84WL1OK/gid4hCnxo083LYLXUpIqMmL+O6fmXAvsvkyMyT520Cw0ZNCrUkWoCjGE4JZZGF4wOkWdF37JFWbDnE/GF5mAykKj+OMaECBlZ207KleQqgVzHjKuCbhPMuBVVD3IhjBfIc7EEM438LbtayMDx4dviPWwm127jwn8qd9H3kv5JBoDfsdYMB3k39r724CljqlAfX33GpbV2LvEkL6Da3OFk+grfN98X2pCBRz5+1N95I2cRD7o+jwtCr+65E+Gqjo4OI60F9Gq5GDcrnudnUw13a4zwlU6W+Cy8gJ4R0CcKTc4+VhYVX5wW2tzLVnDqvjIN8hjhgtmUv8hr19Wn+42ev+5sNtO5QAS6sJMJG5D+cpxCNhei1Xm+1zXliaA1fvVYRqon2MdHcedFeAjzVtX38+Xweytowydcq2V/9pUUNZIzUqX7tZr3F+Ao3QOb/CuWbUBpUcbXfGv7AI1ozP8LRByyu6O8Z1dZNdkdjWVt83maUrIJHjjc7jlZY9JbH6EyYV5TenjJaupvdlx72vA7Fcgevx87seog2JALAJqZQNT+t9/tmrTUSEp3t4aINKUC1QC0CYKECAwEAAQ==-----END PUBLIC KEY-----
```

---
