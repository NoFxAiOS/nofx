import { test, expect } from '@playwright/test';

test.describe('News Config E2E Tests', () => {
  test.beforeEach(async ({ page }) => {
    // 访问应用
    await page.goto('/');

    // 等待应用加载（假设有登录屏幕，我们需要登录）
    // 这里假设已经通过身份验证，可能需要修改
    await page.waitForLoadState('networkidle');
  });

  test('should open news config modal and create configuration', async ({ page }) => {
    // 点击"配置新闻源"按钮
    const newsConfigButton = page.locator('button:has-text("配置新闻源")').first();
    await newsConfigButton.click();

    // 验证模态框打开
    const modal = page.locator('[class*="fixed"][class*="inset-0"]');
    await expect(modal).toBeVisible();

    // 验证标题
    await expect(page.locator('h2:has-text("新闻源配置")')).toBeVisible();

    // 选择 Mlion 复选框
    const mlionCheckbox = page.locator('input[type="checkbox"]').first();
    await mlionCheckbox.check();
    await expect(mlionCheckbox).toBeChecked();

    // 选择 Twitter
    const twitterCheckbox = page.locator('input[type="checkbox"]').nth(1);
    await twitterCheckbox.check();
    await expect(twitterCheckbox).toBeChecked();

    // 设置抓取间隔
    const intervalInput = page.locator('input[type="number"][min="1"][max="1440"]').first();
    await intervalInput.fill('10');
    await expect(intervalInput).toHaveValue('10');

    // 设置每次最大文章数
    const articlesInput = page.locator('input[type="number"][min="1"][max="100"]');
    await articlesInput.fill('25');
    await expect(articlesInput).toHaveValue('25');

    // 调整情绪阈值
    const sentimentSlider = page.locator('input[type="range"]');
    await sentimentSlider.fill('0.5');

    // 保存配置
    const saveButton = page.locator('button:has-text("保存配置")');
    await saveButton.click();

    // 验证成功提示
    const successMessage = page.locator('text=保存成功');
    await expect(successMessage).toBeVisible();

    // 等待模态框关闭
    await expect(modal).not.toBeVisible({ timeout: 2000 });
  });

  test('should display validation errors for invalid input', async ({ page }) => {
    // 打开模态框
    const newsConfigButton = page.locator('button:has-text("配置新闻源")').first();
    await newsConfigButton.click();

    // 不选择任何新闻源，直接尝试保存
    const saveButton = page.locator('button:has-text("保存配置")');
    await saveButton.click();

    // 验证错误信息
    const errorMessage = page.locator('text=必须至少选择一个新闻源');
    await expect(errorMessage).toBeVisible();
  });

  test('should validate fetch interval range', async ({ page }) => {
    // 打开模态框
    const newsConfigButton = page.locator('button:has-text("配置新闻源")').first();
    await newsConfigButton.click();

    // 选择一个新闻源
    const mlionCheckbox = page.locator('input[type="checkbox"]').first();
    await mlionCheckbox.check();

    // 设置无效的抓取间隔（0）
    const intervalInput = page.locator('input[type="number"][min="1"][max="1440"]').first();
    await intervalInput.fill('0');

    // 尝试保存
    const saveButton = page.locator('button:has-text("保存配置")');
    await saveButton.click();

    // 验证错误信息
    const errorMessage = page.locator('text=抓取间隔必须在1-1440分钟之间');
    await expect(errorMessage).toBeVisible();
  });

  test('should validate max articles count range', async ({ page }) => {
    // 打开模态框
    const newsConfigButton = page.locator('button:has-text("配置新闻源")').first();
    await newsConfigButton.click();

    // 选择一个新闻源
    const mlionCheckbox = page.locator('input[type="checkbox"]').first();
    await mlionCheckbox.check();

    // 设置无效的文章数（>100）
    const articlesInput = page.locator('input[type="number"][min="1"][max="100"]');
    await articlesInput.fill('150');

    // 尝试保存
    const saveButton = page.locator('button:has-text("保存配置")');
    await saveButton.click();

    // 验证错误信息
    const errorMessage = page.locator('text=每次抓取的最大文章数必须在1-100之间');
    await expect(errorMessage).toBeVisible();
  });

  test('should validate sentiment threshold range', async ({ page }) => {
    // 打开模态框
    const newsConfigButton = page.locator('button:has-text("配置新闻源")').first();
    await newsConfigButton.click();

    // 选择一个新闻源
    const mlionCheckbox = page.locator('input[type="checkbox"]').first();
    await mlionCheckbox.check();

    // 情绪阈值范围是-1.0到1.0，range input自动限制，所以这个测试是为了验证UI元素存在
    const sentimentSlider = page.locator('input[type="range"]');
    await expect(sentimentSlider).toHaveAttribute('min', '-1');
    await expect(sentimentSlider).toHaveAttribute('max', '1');
  });

  test('should toggle news function on/off', async ({ page }) => {
    // 打开模态框
    const newsConfigButton = page.locator('button:has-text("配置新闻源")').first();
    await newsConfigButton.click();

    // 找到启用开关
    const enableToggle = page.locator('button[class*="bg-blue-500"], button[class*="bg-gray-300"]').first();

    // 验证初始状态（应该是启用的）
    const initialClass = await enableToggle.getAttribute('class');
    expect(initialClass).toContain('bg-blue-500');

    // 点击切换
    await enableToggle.click();

    // 验证状态改变
    const newClass = await enableToggle.getAttribute('class');
    expect(newClass).toContain('bg-gray-300');
  });

  test('should display current configuration on news config page', async ({ page }) => {
    // 假设有一个导航到news config页面的链接
    const newsConfigLink = page.locator('a:has-text("新闻配置")');

    // 如果存在导航链接，点击它
    if (await newsConfigLink.isVisible()) {
      await newsConfigLink.click();

      // 验证页面加载
      await expect(page.locator('h1:has-text("新闻源配置")')).toBeVisible();

      // 验证配置卡片存在
      const configCard = page.locator('[class*="bg-white"][class*="dark:bg-gray-800"]').first();
      await expect(configCard).toBeVisible();
    }
  });

  test('should close modal on cancel button', async ({ page }) => {
    // 打开模态框
    const newsConfigButton = page.locator('button:has-text("配置新闻源")').first();
    await newsConfigButton.click();

    // 验证模态框打开
    const modal = page.locator('[class*="fixed"][class*="inset-0"]');
    await expect(modal).toBeVisible();

    // 点击取消按钮
    const cancelButton = page.locator('button:has-text("取消")');
    await cancelButton.click();

    // 验证模态框关闭
    await expect(modal).not.toBeVisible({ timeout: 1000 });
  });

  test('should close modal on X button', async ({ page }) => {
    // 打开模态框
    const newsConfigButton = page.locator('button:has-text("配置新闻源")').first();
    await newsConfigButton.click();

    // 验证模态框打开
    const modal = page.locator('[class*="fixed"][class*="inset-0"]');
    await expect(modal).toBeVisible();

    // 点击关闭按钮（X）
    const closeButton = page.locator('button[class*="hover:bg-gray-100"]');
    await closeButton.click();

    // 验证模态框关闭
    await expect(modal).not.toBeVisible({ timeout: 1000 });
  });

  test('should support dark mode', async ({ page }) => {
    // 打开模态框
    const newsConfigButton = page.locator('button:has-text("配置新闻源")').first();
    await newsConfigButton.click();

    // 检查暗模式类是否应用
    const modal = page.locator('[class*="bg-white"]').first();

    // 验证modal包含dark模式支持的类
    const modalClass = await modal.getAttribute('class');
    expect(modalClass).toContain('dark:');
  });

  test('should handle multiple news sources selection', async ({ page }) => {
    // 打开模态框
    const newsConfigButton = page.locator('button:has-text("配置新闻源")').first();
    await newsConfigButton.click();

    // 选择所有可用的新闻源
    const checkboxes = page.locator('input[type="checkbox"]');
    const count = await checkboxes.count();

    // 选择前4个复选框（对应Mlion, Twitter, Reddit, Telegram）
    for (let i = 0; i < Math.min(4, count); i++) {
      await checkboxes.nth(i).check();
    }

    // 验证所有都被选中
    for (let i = 0; i < Math.min(4, count); i++) {
      await expect(checkboxes.nth(i)).toBeChecked();
    }

    // 保存配置
    const saveButton = page.locator('button:has-text("保存配置")');
    await saveButton.click();

    // 验证成功
    const successMessage = page.locator('text=保存成功');
    await expect(successMessage).toBeVisible();
  });
});
