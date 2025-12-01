# 连接Web3钱包按钮 - UI/UX设计规范

## 设计概述

本规范详细描述了"连接Web3钱包"按钮的视觉设计、交互行为和用户体验标准，确保产品的一致性和可用性。

---

## 1. 视觉设计

### 1.1 按钮设计

#### 默认状态 (未连接)
```
┌─────────────────────────────┐
│ 🔗 连接Web3钱包              │
└─────────────────────────────┘
```

**属性:**
- 宽度: 自适应内容 (最小 140px)
- 高度: 36px (标准) / 32px (小) / 44px (大)
- 边框: 1px solid #e2e8f0
- 背景: #ffffff
- 文字: #1a202c, 14px, Medium
- 图标: 16px, #3182ce
- 圆角: 8px
- 内边距: 12px 16px

#### 悬停状态
```
┌─────────────────────────────┐
│ 🔗 连接Web3钱包              │ ← 边框变为 #3182ce
│                             │ ← 背景变为 #f7fafc
└─────────────────────────────┘
```

**属性:**
- 边框颜色: #3182ce
- 背景色: #f7fafc
- 文字颜色: #2d3748
- 阴影: 0 1px 3px rgba(0,0,0,0.1)

#### 点击状态
```
┌─────────────────────────────┐
│ 🔗 连接Web3钱包              │ ← 按下效果
│                             │ ← 阴影内陷
└─────────────────────────────┘
```

**属性:**
- 阴影: 内陷效果
- 背景色: #edf2f7

#### 禁用状态
```
┌─────────────────────────────┐
│ 🔗 连接Web3钱包              │
└─────────────────────────────┘
```

**属性:**
- 背景色: #f7fafc
- 文字颜色: #a0aec0
- 边框颜色: #e2e8f0
- 鼠标样式: not-allowed

### 1.2 连接中状态

#### 加载中
```
┌─────────────────────────────┐
│ ⏳ 正在连接钱包...           │
└─────────────────────────────┘
```

**属性:**
- 动画: 旋转图标 (1s 线性无限旋转)
- 背景色: #edf2f7
- 文字颜色: #718096
- 边框样式: 虚线边框
- 鼠标样式: wait

#### 验证中
```
┌─────────────────────────────┐
│ 🔍 验证签名中...             │
└─────────────────────────────┘
```

**属性:**
- 图标: 搜索图标
- 动画: 脉冲效果
- 文字颜色: #3182ce

### 1.3 已连接状态

#### 连接成功
```
┌──────────────────────────────────────────┐
│ ✓ 已连接: 0x742d...E9E0    [断开连接]   │
└──────────────────────────────────────────┘
```

**属性:**
- 宽度: 自适应 (最小 240px)
- 高度: 36px
- 背景: #f0fff4
- 边框: 1px solid #9ae6b4
- 文字: #22543d, 14px, Regular
- 成功图标: 16px, #38a169
- 地址: 等宽字体, #2f855a
- 断开按钮: 链接样式, #e53e3e

#### 地址显示格式
```
显示: 0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0
格式: [前缀] + [中间6字符] + ... + [后缀4字符]
示例: 0x742d...E9E0
```

**属性:**
- 前缀: 6字符
- 后缀: 4字符
- 中间: 省略号 "..."
- 字体: 'Monaco', 'Menlo', monospace
- 点击复制: 全地址复制到剪贴板

---

## 2. 钱包选择弹窗

### 2.1 弹窗结构

```
┌──────────────────────────────────────────────────┐
│  选择您的钱包类型                        [✕]    │
├──────────────────────────────────────────────────┤
│                                                  │
│  ┌────────────────────────────────────────────┐  │
│  │ 🦊 MetaMask                               │  │
│  │    流行的以太坊浏览器钱包                  │  │
│  │    ✓ 已安装: 是                          │  │
│  └────────────────────────────────────────────┘  │
│                                                  │
│  ┌────────────────────────────────────────────┐  │
│  │ 🔵 TP钱包                                 │  │
│  │    安全可靠的数字钱包                      │  │
│  │    ✓ 已安装: 否                          │  │
│  └────────────────────────────────────────────┘  │
│                                                  │
│  ┌────────────────────────────────────────────┐  │
│  │ 📦 安装新钱包                             │  │
│  │    查看更多钱包选项                        │  │
│  └────────────────────────────────────────────┘  │
│                                                  │
│                              [取消]    [连接]   │
└──────────────────────────────────────────────────┘
```

### 2.2 弹窗属性

**尺寸:**
- 宽度: 480px (桌面) / 90vw (移动端)
- 高度: 自适应内容 (最小 400px)
- 最大高度: 80vh

**样式:**
- 背景: #ffffff
- 阴影: 0 10px 40px rgba(0,0,0,0.2)
- 边框: 无
- 圆角: 12px
- 边框-radius: 12px

**标题:**
- 字体: 18px, Semi-bold
- 颜色: #1a202c
- 内边距: 24px 24px 16px

**钱包选项:**
- 高度: 80px
- 内边距: 16px
- 圆角: 8px
- 边框: 1px solid #e2e8f0
- 背景: #ffffff
- 悬停: 背景 #f7fafc
- 选中: 边框 #3182ce, 背景 #ebf8ff

**钱包图标:**
- 尺寸: 32x32px
- 位置: 左侧

**钱包信息:**
- 名称: 16px, Medium, #1a202c
- 描述: 14px, Regular, #718096
- 安装状态: 12px, #48bb78 (已安装) / #f56565 (未安装)

### 2.3 动画效果

#### 打开动画
```css
@keyframes modalFadeIn {
  from {
    opacity: 0;
    transform: translateY(-20px) scale(0.95);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}
```

#### 关闭动画
```css
@keyframes modalFadeOut {
  from {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
  to {
    opacity: 0;
    transform: translateY(20px) scale(0.95);
  }
}
```

---

## 3. 响应式设计

### 3.1 桌面端 (>1024px)

```
[登录] [注册] [连接Web3钱包]         [头像] [通知] [设置]
```

**按钮位置:** 登录/注册按钮右侧
**按钮样式:** 完整文本显示
**弹窗:** 居中显示
**动画:** 完整动画效果

### 3.2 平板端 (768px - 1024px)

```
[登录] [注册] [连接Web3钱包]    [头像]
```

**按钮位置:** 登录/注册按钮右侧
**按钮样式:** 完整文本显示
**弹窗:** 居中显示
**动画:** 简化动画

### 3.3 移动端 (<768px)

```
[登录] [连接Web3钱包]
[注册]
[头像]
```

**按钮位置:** 登录按钮下方
**按钮样式:** 简化文本 (仅图标+短文本)
**弹窗:** 全屏显示
**动画:** 底部滑入

移动端弹窗:
```
┌─────────────────────────────┐
│                             │
│  选择钱包类型                │
│                             │
│  ┌───────────────────────┐  │
│  │  🦊 MetaMask         │  │
│  │     连接              │  │
│  └───────────────────────┘  │
│                             │
│  ┌───────────────────────┐  │
│  │  🔵 TP钱包            │  │
│  │     连接              │  │
│  └───────────────────────┘  │
│                             │
│  ┌───────────────────────┐  │
│  │  📦 其他钱包          │  │
│  │     查看              │  │
│  └───────────────────────┘  │
│                             │
│         [取消]              │
└─────────────────────────────┘
```

---

## 4. 状态管理

### 4.1 状态枚举

```typescript
enum WalletConnectionState {
  DISCONNECTED = 'disconnected',    // 未连接
  CONNECTING = 'connecting',        // 连接中
  AUTHENTICATING = 'authenticating', // 验证中
  CONNECTED = 'connected',          // 已连接
  DISCONNECTING = 'disconnecting',  // 断开中
  ERROR = 'error'                   // 错误
}
```

### 4.2 状态流转图

```
未连接 (DISCONNECTED)
    ↓ [点击连接]
连接中 (CONNECTING)
    ↓ [钱包已选择]
验证中 (AUTHENTICATING)
    ↓ [签名成功]
已连接 (CONNECTED)
    ↓ [点击断开]
断开中 (DISCONNECTING)
    ↓ [操作完成]
未连接 (DISCONNECTED)
```

### 4.3 状态指示器

#### 颜色编码
- 未连接: #718096 (灰色)
- 连接中: #3182ce (蓝色)
- 验证中: #805ad5 (紫色)
- 已连接: #38a169 (绿色)
- 断开中: #dd6b20 (橙色)
- 错误: #e53e3e (红色)

#### 图标编码
- 未连接: `🔗`
- 连接中: `⏳`
- 验证中: `🔍`
- 已连接: `✓`
- 断开中: `⚠`
- 错误: `⚠`

---

## 5. 交互细节

### 5.1 悬停效果

#### 按钮悬停
- 持续时间: 150ms
- 缓动函数: ease-out
- 变化属性: 背景色、边框色、阴影

#### 选项悬停
- 持续时间: 100ms
- 缓动函数: ease-out
- 变化属性: 背景色、边框色

### 5.2 点击反馈

#### 按钮点击
1. 视觉反馈: 阴影内陷 (50ms)
2. 状态变化: 连接中 (100ms)
3. 弹窗打开: 淡入动画 (200ms)

#### 选项点击
1. 视觉反馈: 背景高亮 (50ms)
2. 选中状态: 边框高亮 (100ms)
3. 弹窗关闭: 淡出动画 (150ms)

### 5.3 键盘导航

#### 弹窗内导航
- Tab: 下一个选项
- Shift+Tab: 上一个选项
- Enter/Space: 选中选项
- Escape: 关闭弹窗

#### 焦点管理
- 打开弹窗: 焦点转移到第一个选项
- 关闭弹窗: 焦点返回到触发按钮
- 焦点环: 可见的焦点指示器

---

## 6. 无障碍设计

### 6.1 ARIA标签

#### 按钮 ARIA
```html
<button
  aria-label="连接Web3钱包"
  aria-describedby="wallet-help"
  aria-expanded="false"
>
  🔗 连接Web3钱包
</button>
```

#### 弹窗 ARIA
```html
<div
  role="dialog"
  aria-modal="true"
  aria-labelledby="wallet-title"
  aria-describedby="wallet-description"
>
  <h2 id="wallet-title">选择您的钱包类型</h2>
  <div id="wallet-description">
    请选择要连接的钱包类型
  </div>
</div>
```

### 6.2 键盘快捷键

- `Alt + W`: 打开/关闭钱包连接
- `Enter`: 确认选择
- `Escape`: 取消操作
- `Space`: 切换按钮状态

### 6.3 屏幕阅读器支持

#### 状态播报
```html
<div
  role="status"
  aria-live="polite"
  aria-atomic="true"
>
  已连接到钱包 0x742d...E9E0
</div>
```

#### 错误提示
```html
<div
  role="alert"
  aria-live="assertive"
>
  连接失败，请重试
</div>
```

---

## 7. 主题定制

### 7.1 颜色主题

#### 浅色主题 (默认)
```css
:root {
  --wallet-button-bg: #ffffff;
  --wallet-button-text: #1a202c;
  --wallet-button-border: #e2e8f0;
  --wallet-button-hover-bg: #f7fafc;
  --wallet-button-hover-border: #3182ce;
  --wallet-success-bg: #f0fff4;
  --wallet-success-text: #22543d;
  --wallet-error-bg: #fff5f5;
  --wallet-error-text: #742a2a;
}
```

#### 深色主题
```css
[data-theme="dark"] {
  --wallet-button-bg: #1a202c;
  --wallet-button-text: #f7fafc;
  --wallet-button-border: #2d3748;
  --wallet-button-hover-bg: #2d3748;
  --wallet-button-hover-border: #4299e1;
  --wallet-success-bg: #1a202c;
  --wallet-success-text: #9ae6b4;
  --wallet-error-bg: #1a202c;
  --wallet-error-text: #fc8181;
}
```

### 7.2 尺寸变量

```css
:root {
  --wallet-button-height-small: 32px;
  --wallet-button-height-medium: 36px;
  --wallet-button-height-large: 44px;
  --wallet-button-padding: 12px 16px;
  --wallet-button-radius: 8px;
}
```

---

## 8. 动画规范

### 8.1 缓动函数

```css
--ease-out: cubic-bezier(0.16, 1, 0.3, 1);
--ease-in: cubic-bezier(0.4, 0, 1, 1);
--ease-in-out: cubic-bezier(0.4, 0, 0.2, 1);
```

### 8.2 持续时间

- 微交互: 100ms
- 悬停效果: 150ms
- 状态切换: 200ms
- 弹窗动画: 250ms
- 页面过渡: 300ms

### 8.3 动画示例

#### 按钮脉冲效果 (连接中)
```css
@keyframes pulse {
  0%, 100% {
    opacity: 1;
  }
  50% {
    opacity: 0.7;
  }
}

.wallet-connecting {
  animation: pulse 1.5s ease-in-out infinite;
}
```

---

## 9. 错误状态

### 9.1 钱包未安装

```
┌─────────────────────────────┐
│ ⚠️ 未检测到钱包扩展          │
│                             │
│ 请安装 MetaMask 或 TP 钱包  │
│                             │
│  [安装 MetaMask]            │
│  [安装 TP钱包]              │
└─────────────────────────────┘
```

**错误码:** `WALLET_NOT_INSTALLED`

### 9.2 用户拒绝连接

```
┌─────────────────────────────┐
│ ⚠️ 连接被拒绝                │
│                             │
│ 您取消了钱包连接             │
│                             │
│     [重试连接]               │
└─────────────────────────────┘
```

**错误码:** `USER_REJECTED`

### 9.3 签名失败

```
┌─────────────────────────────┐
│ ⚠️ 签名验证失败              │
│                             │
│ 可能是签名已过期             │
│                             │
│     [重新签名]               │
└─────────────────────────────┘
```

**错误码:** `SIGNATURE_FAILED`

### 9.4 网络错误

```
┌─────────────────────────────┐
│ ⚠️ 网络连接错误              │
│                             │
│ 请检查您的网络连接           │
│                             │
│     [重试连接]               │
└─────────────────────────────┘
```

**错误码:** `NETWORK_ERROR`

---

## 10. 性能优化

### 10.1 懒加载

- 钱包图标使用懒加载
- 弹窗内容延迟渲染
- 非关键动画延后加载

### 10.2 动画优化

- 使用transform和opacity
- 避免触发layout
- 使用will-change提示浏览器
- 尊重用户减少动画偏好

```css
@media (prefers-reduced-motion: reduce) {
  * {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}
```

### 10.3 渲染优化

- 使用React.memo包装组件
- 避免不必要的重渲染
- 使用useCallback和useMemo
- 虚拟化长列表 (如有多选项)

---

## 11. 测试用例

### 11.1 视觉回归测试

```typescript
// 测试不同状态的视觉差异
test('钱包连接按钮状态对比', () => {
  const { rerender } = render(<Web3ConnectButton />);
  expect(screen.getByRole('button')).toHaveTextContent('连接Web3钱包');

  rerender(<Web3ConnectButton state="connecting" />);
  expect(screen.getByRole('button')).toHaveTextContent('正在连接钱包...');

  rerender(<Web3ConnectButton state="connected" address="0x123..." />);
  expect(screen.getByRole('button')).toHaveTextContent('已连接: 0x123...');
});
```

### 11.2 交互测试

```typescript
// 测试点击事件
test('点击连接按钮打开弹窗', async () => {
  render(<Web3ConnectButton />);
  fireEvent.click(screen.getByRole('button'));
  expect(await screen.findByRole('dialog')).toBeInTheDocument();
});

// 测试键盘导航
test('键盘导航支持', () => {
  render(<WalletSelector options={mockOptions} />);
  fireEvent.keyDown(screen.getByLabelText('MetaMask'), { key: 'Enter' });
  // 验证选项被选中
});
```

### 11.3 无障碍测试

```typescript
// 测试屏幕阅读器支持
test('ARIA标签正确设置', () => {
  render(<Web3ConnectButton />);
  const button = screen.getByRole('button');
  expect(button).toHaveAttribute('aria-label', '连接Web3钱包');
});

// 测试焦点管理
test('弹窗焦点管理', async () => {
  render(<Web3ConnectButton />);
  fireEvent.click(screen.getByRole('button'));
  await waitFor(() => {
    expect(screen.getByRole('dialog')).toHaveFocus();
  });
});
```

---

## 12. 交付标准

### 12.1 设计交付物

- [ ] Figma设计文件
- [ ] 组件库文档
- [ ] 动画规范视频
- [ ] 交互原型演示

### 12.2 开发交付物

- [ ] React组件源代码
- [ ] TypeScript类型定义
- [ ] 单元测试用例
- [ ] 集成测试脚本
- [ ] Storybook组件文档
- [ ] 性能基准报告

### 12.3 验收标准

- [ ] 所有设计规范100%实现
- [ ] 跨浏览器兼容性测试通过
- [ ] 响应式设计适配完成
- [ ] 无障碍标准合规
- [ ] 性能指标达标
- [ ] 用户验收测试通过

---

*本规范将持续更新和优化，确保最佳的用户体验。*
