#!/bin/bash

# Binance Testnet API Test
API_KEY="UDhZMfRIaTRmikFRX19XSmJ43N6iKqY29DAvmD7B6GnEU8FPkQN1S3xMB5nzpRLP"
SECRET_KEY="jblq1A8DVq2kJ63P9QbTMEyKhgui6Sxe1VkWGz0ZSj6WpP5wQTqHBIqZJqEfDGxm"

# Test 1: Public endpoint (no auth)
echo "=== Test 1: Public API (Ping) ==="
curl -s "https://testnet.binance.vision/api/v3/ping"
echo -e "\n"

echo "=== Test 2: Public API (Time) ==="
curl -s "https://testnet.binance.vision/api/v3/time" | python3 -m json.tool
echo -e "\n"

# Test 3: Signed endpoint (Account info)
echo "=== Test 3: Signed API (Account) ==="
timestamp=$(python3 -c "import time; print(int(time.time() * 1000))")
query_string="timestamp=$timestamp"
signature=$(echo -n "$query_string" | openssl dgst -sha256 -hmac "$SECRET_KEY" | awk '{print $2}')

curl -s -H "X-MBX-APIKEY: $API_KEY" \
  "https://testnet.binance.vision/api/v3/account?${query_string}&signature=${signature}" | python3 -m json.tool

echo -e "\n=== Test 4: Futures Account ==="
timestamp=$(python3 -c "import time; print(int(time.time() * 1000))")
query_string="timestamp=$timestamp"
signature=$(echo -n "$query_string" | openssl dgst -sha256 -hmac "$SECRET_KEY" | awk '{print $2}')

curl -s -H "X-MBX-APIKEY: $API_KEY" \
  "https://testnet.binancefuture.com/fapi/v2/account?${query_string}&signature=${signature}" | python3 -m json.tool
