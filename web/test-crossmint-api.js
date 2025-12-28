#!/usr/bin/env node

/**
 * Test Crossmint API Connection
 * Verifies that the API key is valid and can create checkout sessions
 */

const API_KEY = process.env.VITE_CROSSMINT_CLIENT_API_KEY || 'ck_staging_A5uKZ7CEniK6h66rJfWmTQsgY815P38779hcw39CGFcidLvbBKZHVNiZDKs8p23eBZ4C38BHD6itjdfrHgEuswMyfZFFLS8HBuXL7DprgVwYJcgnxKvaHC5uzXfL81SGdXt6NThX2bJcXS2LxLU6HQH7wfjRqSGWgXMYS3cBCGG3rnBn9uvYFNSsTypVMqX3C9Vy7nzeo9sCSKBHwduUUQCr';

console.log('\n🔍 Crossmint API 连接测试\n');
console.log('API Key (前40字符):', API_KEY.substring(0, 40) + '...');
console.log('API Key 长度:', API_KEY.length);
console.log('API Key 格式:', API_KEY.startsWith('ck_staging_') ? '✅ 正确 (staging)' : API_KEY.startsWith('ck_production_') ? '✅ 正确 (production)' : '❌ 格式错误');
console.log('\n测试套餐: 初级套餐 (10 USDT → 500 积分)\n');

const testPayload = {
  lineItems: [{
    price: "10",
    currency: "USDT",
    quantity: 1,
    metadata: {
      packageId: "starter",
      credits: 500,
      bonusMultiplier: 1.0
    }
  }],
  payment: {
    allowedMethods: ["crypto"]
  },
  preferredChains: ["polygon", "base", "arbitrum"],
  locale: "en-US"
};

console.log('📤 发送请求到 Crossmint API...\n');

fetch("https://api.crossmint.com/2022-06-09/embedded-checkouts", {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    "X-API-KEY": API_KEY,
  },
  body: JSON.stringify(testPayload),
})
.then(async (response) => {
  console.log('📥 响应状态:', response.status, response.statusText);

  if (!response.ok) {
    const errorText = await response.text();
    console.error('\n❌ API 错误:');
    console.error('状态码:', response.status);
    console.error('错误详情:', errorText);

    if (response.status === 401) {
      console.error('\n💡 解决方案: API Key 无效或未授权');
      console.error('   - 检查 API Key 是否正确');
      console.error('   - 确认 Key 的 scopes 包含 orders.create');
    } else if (response.status === 403) {
      console.error('\n💡 解决方案: 权限不足或域名未授权');
      console.error('   - 在 Crossmint Console 中添加域名到白名单');
    } else if (response.status === 404) {
      console.error('\n💡 解决方案: API 端点不存在');
      console.error('   - API 版本 2022-06-09 可能已弃用');
      console.error('   - 尝试联系 Crossmint 支持获取最新 API 版本');
    }

    process.exit(1);
  }

  return response.json();
})
.then((data) => {
  console.log('\n✅ API 连接成功!\n');
  console.log('Session ID:', data.id || data.sessionId || '(未找到)');
  console.log('完整响应:', JSON.stringify(data, null, 2));

  if (data.id || data.sessionId) {
    const sessionId = data.id || data.sessionId;
    console.log('\n🔗 测试 Checkout URL:');
    console.log(`https://embedded-checkout.crossmint.com?sessionId=${sessionId}`);
    console.log('\n✨ 配置验证成功！支付功能应该可以正常工作。');
  }
})
.catch((error) => {
  console.error('\n❌ 网络错误或 CORS 问题:');
  console.error('错误信息:', error.message);

  if (error.message.includes('fetch')) {
    console.error('\n💡 可能的原因:');
    console.error('   1. 网络连接问题');
    console.error('   2. CORS 跨域限制 (需要在 Crossmint Console 配置)');
    console.error('   3. API 端点不存在或已变更');
  }

  process.exit(1);
});
