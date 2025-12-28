#!/bin/bash

API_KEY="ck_staging_A5uKZ7CEniK6h66rJfWmTQsgY815P38779hcw39CGFcidLvbBKZHVNiZDKs8p23eBZ4C38BHD6itjdfrHgEuswMyfZFFLS8HBuXL7DprgVwYJcgnxKvaHC5uzXfL81SGdXt6NThX2bJcXS2LxLU6HQH7wfjRqSGWgXMYS3cBCGG3rnBn9uvYFNSsTypVMqX3C9Vy7nzeo9sCSKBHwduUUQCr"

echo "🔍 Testing Crossmint API Connection"
echo "API Key: ${API_KEY:0:40}..."
echo ""

curl -k -i -X POST "https://api.crossmint.com/2022-06-09/embedded-checkouts" \
  -H "Content-Type: application/json" \
  -H "X-API-KEY: $API_KEY" \
  -d '{"lineItems":[{"price":"10","currency":"USDT","quantity":1}],"payment":{"allowedMethods":["crypto"]}}'

echo ""
echo "---"
echo "测试完成"
