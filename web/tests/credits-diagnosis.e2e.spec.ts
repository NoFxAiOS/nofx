import { test, expect, Page } from '@playwright/test';

const FRONTEND_URL = 'https://www.agentrade.xyz';
const API_BASE = 'https://nofx-gyc567.replit.app/api';

test.describe('Credits Display Diagnosis', () => {
  let page: Page;

  test.beforeEach(async ({ browser }) => {
    page = await browser.newPage();

    // 记录所有网络请求
    page.on('response', response => {
      if (response.url().includes('/user/credits')) {
        console.log(`\n📡 API响应: ${response.url()}`);
        console.log(`   状态码: ${response.status()}`);
        response.json().then(data => {
          console.log(`   响应数据:`, JSON.stringify(data, null, 2));
        }).catch(() => {});
      }
    });

    // 记录console日志
    page.on('console', msg => {
      if (msg.text().includes('Token') || msg.text().includes('User') || msg.text().includes('credits')) {
        console.log(`🖥️  Console: ${msg.text()}`);
      }
    });
  });

  test('01: 检查localStorage中的认证数据', async () => {
    await page.goto(FRONTEND_URL);
    await page.waitForTimeout(2000);

    const token = await page.evaluate(() => localStorage.getItem('auth_token'));
    const user = await page.evaluate(() => localStorage.getItem('auth_user'));

    console.log('\n=== localStorage检查 ===');
    console.log(`✓ Token存在: ${token ? '✅' : '❌'}`);
    console.log(`✓ User存在: ${user ? '✅' : '❌'}`);

    if (token) {
      console.log(`✓ Token长度: ${token.length}`);
      console.log(`✓ Token前50字符: ${token.substring(0, 50)}...`);
    }

    if (user) {
      const userData = JSON.parse(user);
      console.log(`✓ User ID: ${userData.id}`);
      console.log(`✓ User Email: ${userData.email}`);
    }

    expect(token).not.toBeNull();
    expect(user).not.toBeNull();
  });

  test('02: 检查API请求是否被发送', async () => {
    console.log('\n=== Network请求检查 ===');

    const requestPromise = page.waitForResponse(
      response => response.url().includes('/api/user/credits'),
      { timeout: 10000 }
    ).catch(() => null);

    await page.goto(FRONTEND_URL);
    const response = await requestPromise;

    if (!response) {
      console.log('❌ 没有看到/api/user/credits请求');
      console.log('   原因: Hook可能没有执行或条件不满足');
    } else {
      console.log(`✓ 请求状态码: ${response.status()}`);
      response.json().then(data => {
        console.log(`✓ 响应数据:`, JSON.stringify(data, null, 2));
      });
    }
  });

  test('03: 检查CreditsDisplay组件是否存在和可见', async () => {
    console.log('\n=== UI组件检查 ===');

    await page.goto(FRONTEND_URL);
    await page.waitForTimeout(3000);

    // 寻找积分显示的元素（多个选择器）
    const selectors = [
      '[data-testid="credits-display"]',
      '[role="status"][aria-label*="credits"]',
      'text=/.*credits.*/',
      '.credits-display',
      '.credits-value'
    ];

    let found = false;
    for (const selector of selectors) {
      try {
        const element = await page.locator(selector).first();
        const visible = await element.isVisible().catch(() => false);
        if (visible || await element.count().catch(() => 0) > 0) {
          console.log(`✓ 找到元素 (${selector})`);
          const text = await element.textContent();
          console.log(`  显示的内容: ${text}`);
          found = true;
          break;
        }
      } catch (e) {
        // 继续尝试下一个选择器
      }
    }

    if (!found) {
      console.log('❌ 没有找到积分显示元素');
      console.log('   可能的原因:');
      console.log('   1. Hook没有执行（token/user不存在）');
      console.log('   2. API请求失败（401/500）');
      console.log('   3. 组件被隐藏或未渲染');
    }

    // 检查页面中是否有error信息
    const errorIndicators = await page.locator('text=/error|Error|失败/i').count();
    if (errorIndicators > 0) {
      console.log(`⚠️  检测到 ${errorIndicators} 个错误指示符`);
    }
  });

  test('04: 完整流程诊断', async () => {
    console.log('\n=== 完整流程诊断 ===');

    // 1. 检查初始状态
    console.log('1️⃣  检查初始localStorage状态...');
    await page.goto(FRONTEND_URL);

    const initialToken = await page.evaluate(() => localStorage.getItem('auth_token'));
    const initialUser = await page.evaluate(() => localStorage.getItem('auth_user'));

    console.log(`   Token: ${initialToken ? '✅ 存在' : '❌ 不存在'}`);
    console.log(`   User: ${initialUser ? '✅ 存在' : '❌ 不存在'}`);

    // 2. 等待API请求
    console.log('2️⃣  等待API请求...');
    let apiCalled = false;
    let apiStatus = null;
    let apiData = null;

    page.on('response', async response => {
      if (response.url().includes('/api/user/credits')) {
        apiCalled = true;
        apiStatus = response.status();
        try {
          apiData = await response.json();
        } catch (e) {
          apiData = { error: 'Failed to parse response' };
        }
      }
    });

    await page.waitForTimeout(5000);

    if (apiCalled) {
      console.log(`   ✅ API被调用`);
      console.log(`   状态码: ${apiStatus}`);
      if (apiData && apiData.data) {
        console.log(`   available_credits: ${apiData.data.available_credits}`);
      }
    } else {
      console.log(`   ❌ API未被调用`);
    }

    // 3. 检查UI
    console.log('3️⃣  检查UI元素...');
    const uiExists = await page.locator('[data-testid="credits-display"]').count().then(c => c > 0).catch(() => false);
    console.log(`   积分显示组件: ${uiExists ? '✅ 存在' : '❌ 不存在'}`);

    // 4. 总结
    console.log('\n📊 诊断总结:');
    const hasToken = !!initialToken;
    const hasUser = !!initialUser;
    const apiWorking = apiCalled && apiStatus === 200;
    const uiWorking = uiExists;

    console.log(`  Token: ${hasToken ? '✅' : '❌'}`);
    console.log(`  User: ${hasUser ? '✅' : '❌'}`);
    console.log(`  API: ${apiWorking ? '✅' : '❌'}`);
    console.log(`  UI: ${uiWorking ? '✅' : '❌'}`);

    if (!hasToken || !hasUser) {
      console.log('\n⚠️  问题: 登录状态丢失 → 需要重新登录或检查登录流程');
    } else if (!apiCalled) {
      console.log('\n⚠️  问题: Hook没有发送API请求 → 检查useUserCredits条件');
    } else if (apiStatus !== 200) {
      console.log(`\n⚠️  问题: API返回${apiStatus} → 检查认证或后端错误`);
    } else if (!uiWorking) {
      console.log('\n⚠️  问题: UI未渲染 → 检查CreditsDisplay组件逻辑');
    } else {
      console.log('\n✅ 一切正常!');
    }
  });

  test.afterEach(async () => {
    await page.close();
  });
});
