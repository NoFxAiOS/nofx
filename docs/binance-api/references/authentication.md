# Binance-Api - Authentication

**Pages:** 5

---

## Market Data Only URLs

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/faqs/market_data_only

**Contents:**
- Market Data Only URLs
  - RESTful API​
  - Websocket Streams​

These URLs do not require any authentication (i.e. The API key is not necessary) and serve only public market data.

On the RESTful API, these are the endpoints you can request on data-api.binance.vision:

Public market data can also be retrieved through the websocket market data using the URL data-stream.binance.vision. The streams available through this domain are the same that can be found in the Websocket Market Streams documentation.

Note that User Data Streams cannot be accessed through this URL.

**Examples:**

Example 1 (unknown):
```unknown
data-api.binance.vision
```

Example 2 (text):
```text
curl -sX GET "https://data-api.binance.vision/api/v3/exchangeInfo?symbol=BTCUSDT"
```

Example 3 (text):
```text
curl -sX GET "https://data-api.binance.vision/api/v3/exchangeInfo?symbol=BTCUSDT"
```

Example 4 (unknown):
```unknown
data-stream.binance.vision
```

---

## Authentication requests

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/websocket-api/authentication-requests

**Contents:**
- Authentication requests
  - Log in with API key (SIGNED)​
  - Query session status​
  - Log out of the session​

Note: Only Ed25519 keys are supported for this feature.

Authenticate WebSocket connection using the provided API key.

After calling session.logon, you can omit apiKey and signature parameters for future requests that require them.

Note that only one API key can be authenticated. Calling session.logon multiple times changes the current authenticated API key.

Query the status of the WebSocket connection, inspecting which API key (if any) is used to authorize requests.

Forget the API key previously authenticated. If the connection is not authenticated, this request does nothing.

Note that the WebSocket connection stays open after session.logout request. You can continue using the connection, but now you will have to explicitly provide the apiKey and signature parameters where needed.

**Examples:**

Example 1 (javascript):
```javascript
{  "id": "c174a2b1-3f51-4580-b200-8528bd237cb7",  "method": "session.logon",  "params": {    "apiKey": "vmPUZE6mv9SD5VNHk4HlWFsOr6aKE2zvsw0MuIgwCIPy6utIco14y7Ju91duEh8A",    "signature": "1cf54395b336b0a9727ef27d5d98987962bc47aca6e13fe978612d0adee066ed",    "timestamp": 1649729878532  }}
```

Example 2 (javascript):
```javascript
{  "id": "c174a2b1-3f51-4580-b200-8528bd237cb7",  "method": "session.logon",  "params": {    "apiKey": "vmPUZE6mv9SD5VNHk4HlWFsOr6aKE2zvsw0MuIgwCIPy6utIco14y7Ju91duEh8A",    "signature": "1cf54395b336b0a9727ef27d5d98987962bc47aca6e13fe978612d0adee066ed",    "timestamp": 1649729878532  }}
```

Example 3 (unknown):
```unknown
session.logon
```

Example 4 (unknown):
```unknown
session.logon
```

---

## Authentication requests

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/websocket-api/authentication-requests

**Contents:**
- Authentication requests
  - Log in with API key (SIGNED)​
  - Query session status​
  - Log out of the session​

Note: Only Ed25519 keys are supported for this feature.

Authenticate WebSocket connection using the provided API key.

After calling session.logon, you can omit apiKey and signature parameters for future requests that require them.

Note that only one API key can be authenticated. Calling session.logon multiple times changes the current authenticated API key.

Query the status of the WebSocket connection, inspecting which API key (if any) is used to authorize requests.

Forget the API key previously authenticated. If the connection is not authenticated, this request does nothing.

Note that the WebSocket connection stays open after session.logout request. You can continue using the connection, but now you will have to explicitly provide the apiKey and signature parameters where needed.

**Examples:**

Example 1 (javascript):
```javascript
{  "id": "c174a2b1-3f51-4580-b200-8528bd237cb7",  "method": "session.logon",  "params": {    "apiKey": "vmPUZE6mv9SD5VNHk4HlWFsOr6aKE2zvsw0MuIgwCIPy6utIco14y7Ju91duEh8A",    "signature": "1cf54395b336b0a9727ef27d5d98987962bc47aca6e13fe978612d0adee066ed",    "timestamp": 1649729878532  }}
```

Example 2 (javascript):
```javascript
{  "id": "c174a2b1-3f51-4580-b200-8528bd237cb7",  "method": "session.logon",  "params": {    "apiKey": "vmPUZE6mv9SD5VNHk4HlWFsOr6aKE2zvsw0MuIgwCIPy6utIco14y7Ju91duEh8A",    "signature": "1cf54395b336b0a9727ef27d5d98987962bc47aca6e13fe978612d0adee066ed",    "timestamp": 1649729878532  }}
```

Example 3 (unknown):
```unknown
session.logon
```

Example 4 (unknown):
```unknown
session.logon
```

---

## Session Authentication

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/websocket-api/session-authentication

**Contents:**
- Session Authentication
  - Authenticate after connection​
  - Authorize ad hoc requests​

Note: Only Ed25519 keys are supported for this feature.

If you do not want to specify apiKey and signature in each individual request, you can authenticate your API key for the active WebSocket session.

Once authenticated, you no longer have to specify apiKey and signature for those requests that need them. Requests will be performed on behalf of the account owning the authenticated API key.

Note: You still have to specify the timestamp parameter for SIGNED requests.

You can authenticate an already established connection using session authentication requests:

Regarding API key revocation:

If during an active session the API key becomes invalid for any reason (e.g. IP address is not whitelisted, API key was deleted, API key doesn't have correct permissions, etc), after the next request the session will be revoked with the following error message:

Only one API key can be authenticated with the WebSocket connection. The authenticated API key is used by default for requests that require an apiKey parameter. However, you can always specify the apiKey and signature explicitly for individual requests, overriding the authenticated API key and using a different one to authorize a specific request.

For example, you might want to authenticate your USER_DATA key to be used by default, but specify the TRADE key with an explicit signature when placing orders.

**Examples:**

Example 1 (unknown):
```unknown
session.logon
```

Example 2 (unknown):
```unknown
session.status
```

Example 3 (unknown):
```unknown
session.logout
```

Example 4 (javascript):
```javascript
{  "id": null,  "status": 401,  "error": {    "code": -2015,    "msg": "Invalid API-key, IP, or permissions for action."  }}
```

---

## Session Authentication

**URL:** https://developers.binance.com/docs/binance-spot-api-docs/testnet/websocket-api/session-authentication

**Contents:**
- Session Authentication
  - Authenticate after connection​
  - Authorize ad hoc requests​

Note: Only Ed25519 keys are supported for this feature.

If you do not want to specify apiKey and signature in each individual request, you can authenticate your API key for the active WebSocket session.

Once authenticated, you no longer have to specify apiKey and signature for those requests that need them. Requests will be performed on behalf of the account owning the authenticated API key.

Note: You still have to specify the timestamp parameter for SIGNED requests.

You can authenticate an already established connection using session authentication requests:

Regarding API key revocation:

If during an active session the API key becomes invalid for any reason (e.g. IP address is not whitelisted, API key was deleted, API key doesn't have correct permissions, etc), after the next request the session will be revoked with the following error message:

Only one API key can be authenticated with the WebSocket connection. The authenticated API key is used by default for requests that require an apiKey parameter. However, you can always specify the apiKey and signature explicitly for individual requests, overriding the authenticated API key and using a different one to authorize a specific request.

For example, you might want to authenticate your USER_DATA key to be used by default, but specify the TRADE key with an explicit signature when placing orders.

**Examples:**

Example 1 (unknown):
```unknown
session.logon
```

Example 2 (unknown):
```unknown
session.status
```

Example 3 (unknown):
```unknown
session.logout
```

Example 4 (javascript):
```javascript
{  "id": null,  "status": 401,  "error": {    "code": -2015,    "msg": "Invalid API-key, IP, or permissions for action."  }}
```

---
