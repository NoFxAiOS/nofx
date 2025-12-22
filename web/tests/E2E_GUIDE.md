# News Config E2E测试指南

## 测试覆盖范围

本E2E测试套件覆盖了以下场景：

### 1. 创建新闻配置
- **测试**: `should open news config modal and create configuration`
- **步骤**:
  - 打开新闻源配置模态框
  - 选择Mlion和Twitter新闻源
  - 设置抓取间隔为10分钟
  - 设置最大文章数为25
  - 调整情绪阈值为0.5
  - 保存配置
- **验证**: 显示成功提示，模态框关闭

### 2. 表单验证 - 未选择新闻源
- **测试**: `should display validation errors for invalid input`
- **验证**: 显示"必须至少选择一个新闻源"错误

### 3. 表单验证 - 无效抓取间隔
- **测试**: `should validate fetch interval range`
- **步骤**: 设置抓取间隔为0
- **验证**: 显示"抓取间隔必须在1-1440分钟之间"错误

### 4. 表单验证 - 无效文章数
- **测试**: `should validate max articles count range`
- **步骤**: 设置最大文章数为150
- **验证**: 显示"每次抓取的最大文章数必须在1-100之间"错误

### 5. 情绪阈值范围验证
- **测试**: `should validate sentiment threshold range`
- **验证**: 验证Slider的min/max属性为-1到1

### 6. 启用/禁用切换
- **测试**: `should toggle news function on/off`
- **验证**: 切换开关状态改变视觉样式

### 7. 配置页面显示
- **测试**: `should display current configuration on news config page`
- **验证**: 配置页面显示当前配置信息

### 8. 模态框关闭 - 取消按钮
- **测试**: `should close modal on cancel button`
- **验证**: 点击取消后模态框关闭

### 9. 模态框关闭 - X按钮
- **测试**: `should close modal on X button`
- **验证**: 点击X后模态框关闭

### 10. 暗模式支持
- **测试**: `should support dark mode`
- **验证**: 模态框包含dark模式类

### 11. 多新闻源选择
- **测试**: `should handle multiple news sources selection`
- **步骤**: 选择所有4个新闻源（Mlion, Twitter, Reddit, Telegram）
- **验证**: 所有源都被选中，保存成功

## 运行测试

### 前置条件
1. 确保后端API服务运行在端口上（通常是http://localhost:8080）
2. 前端应用在开发模式运行（端口5000）
3. 数据库已初始化
4. 用户已认证（或跳过认证检查）

### 安装依赖
```bash
cd web
npm install
```

### 运行所有E2E测试
```bash
npm run test:e2e
```

### 使用UI模式运行测试（交互式）
```bash
npm run test:e2e:ui
```

### 调试单个测试
```bash
npm run test:e2e:debug
```

### 运行特定测试文件
```bash
npx playwright test tests/news-config.e2e.spec.ts
```

### 运行特定测试用例
```bash
npx playwright test -g "should open news config modal and create configuration"
```

## 测试配置

Playwright配置文件：`playwright.config.ts`

关键配置：
- **baseURL**: http://localhost:5000
- **浏览器**: Chromium, Firefox, WebKit
- **超时**: 30秒（默认）
- **重试**: 开发环境0次，CI环境2次
- **报告**: HTML报告输出到`playwright-report/`
- **截图**: 仅在测试失败时保存
- **视频**: 仅在测试失败时保存

## 测试输出

成功运行后会生成：
- `playwright-report/` - HTML测试报告
- 失败时会包含截图和视频

## 已知问题和注意事项

1. **身份认证**: 当前测试假设用户已经认证。如果需要测试完整的身份流程，需要修改beforeEach钩子。

2. **按钮选择器**: 测试使用文本匹配来定位元素。如果UI文本改变，选择器需要更新。

3. **异步操作**: 某些异步操作可能需要更长的等待时间，可以通过调整timeout参数来处理。

4. **暗模式检查**: 当前只检查类名是否包含"dark:"，可以增强为检查实际的CSS应用。

## 扩展测试

可以添加的额外测试：
1. 编辑现有配置
2. 删除配置
3. 不同浏览器的兼容性测试
4. 响应式设计测试（移动、平板）
5. 长配置名称处理
6. 并发操作（多个用户同时配置）
7. API错误处理（网络故障、服务器错误）
8. 性能测试（加载时间、渲染性能）

## 持续集成集成

在CI/CD流程中运行E2E测试：

```yaml
# GitHub Actions example
- name: Run E2E tests
  run: |
    npm install
    npm run test:e2e
  env:
    CI: true
```

注意：在CI环境中测试会使用headless浏览器并启用重试。
