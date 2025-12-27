import { test, expect, Page, Browser } from '@playwright/test';

test.describe('Credits Display with Login', () => {
  let page: Page;

  test.beforeEach(async ({ browser }) => {
    page = await browser.newPage();

    // 记录所有network请求
    page.on('response', response => {
      if (response.url().includes('/api/')) {
        console.log(`\n📡 API: ${response.url()}`);
        console.log(`   状态码: ${response.status()}`);
      }
    });

    // 记录console日志
    page.on('console', msg => {
      console.log(`🖥️  Console: ${msg.text()}`);
    });

    // 记录错误
    page.on('error', error => {
      console.error(`❌ Page错误: ${error}`);
    });
  });

  test('01: 检查登录页面是否能访问', async () => {
    console.log('\n=== 01: 检查登录页面 ===');

    await page.goto('https://www.agentrade.xyz');
    await page.waitForTimeout(2000);

    // 检查是否有登录相关的元素
    const loginButton = await page.locator('text=/Login|登录|sign in/i').first();
    const emailInput = await page.locator('input[type="email"], input[name="email"]').first();

    const hasLoginButton = await loginButton.isVisible().catch(() => false);
    const hasEmailInput = await emailInput.isVisible().catch(() => false);

    console.log(`登录按钮存在: ${hasLoginButton ? '✅' : '❌'}`);
    console.log(`邮箱输入框存在: ${hasEmailInput ? '✅' : '❌'}`);

    // 检查页面的title或url
    const url = page.url();
    const title = await page.title();
    console.log(`页面URL: ${url}`);
    console.log(`页面标题: ${title}`);
  });

  test('02: 尝试登录流程', async () => {
    console.log('\n=== 02: 尝试登录流程 ===');

    await page.goto('https://www.agentrade.xyz');
    await page.waitForTimeout(2000);

    // 寻找登录表单
    const emailInputs = await page.locator('input[type="email"], input[name="email"], input[placeholder*="email"], input[placeholder*="邮箱"]').all();

    if (emailInputs.length === 0) {
      console.log('❌ 找不到邮箱输入框');
      console.log('可能原因: 页面已经在登录状态或登录表单结构不同');

      // 检查是否已经登录
      const token = await page.evaluate(() => localStorage.getItem('auth_token'));
      const user = await page.evaluate(() => localStorage.getItem('auth_user'));
      console.log(`Token存在: ${token ? '✅' : '❌'}`);
      console.log(`User存在: ${user ? '✅' : '❌'}`);
      return;
    }

    // 如果找到了邮箱输入框，尝试填充
    const emailInput = emailInputs[0];
    console.log('找到邮箱输入框，填充测试邮箱...');

    await emailInput.fill('gyc567@gmail.com');
    await page.waitForTimeout(500);

    // 寻找密码输入框
    const passwordInputs = await page.locator('input[type="password"], input[name="password"]').all();
    if (passwordInputs.length > 0) {
      const passwordInput = passwordInputs[0];
      console.log('找到密码输入框，填充测试密码...');
      // 注意：这里使用一个测试密码，实际应该从环境变量或配置获取
      await passwordInput.fill('TestPassword123');
      await page.waitForTimeout(500);

      // 寻找提交按钮
      const submitButton = await page.locator('button:has-text("Login"), button:has-text("登录"), button[type="submit"]').first();
      if (await submitButton.isVisible()) {
        console.log('点击登录按钮...');
        await submitButton.click();

        // 等待登录完成
        await page.waitForTimeout(3000);

        // 检查token是否被保存
        const token = await page.evaluate(() => localStorage.getItem('auth_token'));
        const user = await page.evaluate(() => localStorage.getItem('auth_user'));

        console.log(`登录后Token: ${token ? '✅ 存在' : '❌ 不存在'}`);
        console.log(`登录后User: ${user ? '✅ 存在' : '❌ 不存在'}`);

        if (token && user) {
          const userData = JSON.parse(user);
          console.log(`User Email: ${userData.email}`);
          console.log(`User ID: ${userData.id}`);
        }
      }
    }
  });

  test('03: 直接设置localStorage并检查积分显示', async () => {
    console.log('\n=== 03: 模拟已登录状态检查积分 ===');

    // 创建一个模拟的token（真实的JWT）
    // 注意：这是一个示例token，实际应该从登录响应获取
    const mockToken = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c';

    const mockUser = {
      id: 'test-user-id-12345',
      email: 'gyc567@gmail.com',
      invite_code: 'TEST123'
    };

    // 访问网站前设置localStorage
    await page.goto('https://www.agentrade.xyz');

    // 设置localStorage
    await page.evaluate(({ token, user }) => {
      localStorage.setItem('auth_token', token);
      localStorage.setItem('auth_user', JSON.stringify(user));
    }, { token: mockToken, user: mockUser });

    // 刷新页面使设置生效
    await page.reload();
    await page.waitForTimeout(3000);

    // 检查token和user是否被设置
    const token = await page.evaluate(() => localStorage.getItem('auth_token'));
    const user = await page.evaluate(() => localStorage.getItem('auth_user'));

    console.log(`设置后Token: ${token ? '✅ 存在' : '❌ 不存在'}`);
    console.log(`设置后User: ${user ? '✅ 存在' : '❌ 不存在'}`);

    // 等待API请求
    console.log('等待API请求...');
    await page.waitForTimeout(5000);

    // 检查API是否被调用
    const requests = await page.context().storageState();
    console.log(`存储状态:`, requests);

    // 检查UI
    const creditsElement = await page.locator('[data-testid="credits-display"]').first();
    const exists = await creditsElement.count().then(c => c > 0).catch(() => false);
    console.log(`积分显示组件存在: ${exists ? '✅' : '❌'}`);

    if (exists) {
      const text = await creditsElement.textContent().catch(() => null);
      console.log(`显示内容: ${text}`);
    }
  });

  test('04: 分析问题根源', async () => {
    console.log('\n=== 04: 根本问题分析 ===');

    // 问题分析
    console.log(`
诊断发现：
1. localStorage中没有auth_token和auth_user
   → 原因: 在新浏览器会话中没有登录状态

2. 如果token/user不存在，useUserCredits Hook会直接返回
   → Hook代码: if (!user?.id || !token) { return; }

3. 这不是API问题，而是：
   a) 登录流程可能没有正确保存token到localStorage
   b) 或者user信息没有正确保存
   c) 或者token过期了

建议修复:
1. 检查登录流程是否正确（src/contexts/AuthContext.tsx）
2. 确认token和user被正确保存到localStorage
3. 确认useUserCredits Hook的条件检查是否正确
4. 如果token存在但API仍返回0，需要检查数据库中的user_credits记录
    `);
  });

  test.afterEach(async () => {
    await page.close();
  });
});
