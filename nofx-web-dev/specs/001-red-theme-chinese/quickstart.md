# Quick Start: Red Theme and Chinese Language

**Feature**: 001-red-theme-chinese  
**Last Updated**: 2025-11-11

## 快速概览

将 NOFX Web Dashboard 从黑色背景+黄色主题改为浅灰色背景+红色主题，并强制所有用户使用中文界面。

## 核心修改

### 1. 主题颜色 (`src/index.css`)

**修改前**:
```css
:root {
  --brand-yellow: #f0b90b;
  --background: #000000;
  --foreground: #eaecef;
}
```

**修改后**:
```css
:root {
  --brand-red: #E50012;
  --background: #FAFAFA;
  --foreground: #1A1A1A;
}
```

### 2. 语言设置 (`src/contexts/LanguageContext.tsx`)

**修改前**:
```typescript
const [language, setLanguage] = useState<Language>(() => {
  const saved = localStorage.getItem('language')
  return saved === 'en' || saved === 'zh' ? saved : 'en'
})
```

**修改后**:
```typescript
const [language] = useState<Language>('zh') // Fixed to Chinese
// Remove localStorage logic
```

## 实施步骤

### Phase 1: CSS 变量重构

1. **备份现有配置**
   ```bash
   cp src/index.css src/index.css.backup
   ```

2. **修改背景颜色**
   - `--background`: `#000000` → `#FAFAFA`
   - `--panel-bg`: `#0a0a0a` → `#FFFFFF`
   - `--foreground`: `#eaecef` → `#1A1A1A`

3. **修改强调色**
   - `--brand-yellow`: `#f0b90b` → `--brand-red`: `#E50012`
   - `--binance-yellow-*`: 改为 `--brand-red-*` 系列

4. **更新文本颜色**
   - `--text-primary`: `#eaecef` → `#1A1A1A`
   - `--text-secondary`: `#848e9c` → `#616161`
   - `--text-tertiary`: `#5e6673` → `#9E9E9E`

5. **调整阴影**
   - 从深色阴影改为浅色阴影
   - `box-shadow: 0 2px 4px rgba(0,0,0,0.3)` → `rgba(0,0,0,0.1)`

### Phase 2: 语言上下文修改

1. **修改 `src/contexts/LanguageContext.tsx`**
   ```typescript
   export function LanguageProvider({ children }: { children: ReactNode }) {
     // 强制使用中文
     const [language] = useState<Language>('zh')
   
     return (
       <LanguageContext.Provider value={{ language, setLanguage: () => {} }}>
         {children}
       </LanguageContext.Provider>
     )
   }
   ```

2. **移除语言选择器 UI** (如果存在)
   - 检查 `src/components/Header.tsx`
   - 移除语言切换下拉菜单或按钮

### Phase 3: 验证

1. **运行开发服务器**
   ```bash
   npm run dev
   ```

2. **访问 http://localhost:3000**
   - 验证背景为浅灰色
   - 验证按钮、链接为红色
   - 验证所有文本为中文

3. **对比度检查**
   - 使用 Chrome DevTools > Accessibility
   - 确保所有文本对比度 >= 4.5:1

4. **跨浏览器测试**
   - Chrome
   - Firefox
   - Safari
   - Edge

## 预期效果

### 视觉变化

**Before (Black + Yellow)**:
- ⬛ 黑色背景 (#000000)
- 🟨 黄色按钮和链接 (#f0b90b)
- 🌑 深色卡片和面板
- 🇬🇧 英文界面

**After (Light Gray + Red)**:
- ⬜ 浅灰色背景 (#FAFAFA)
- 🟥 红色按钮和链接 (#E50012)
- ☁️ 白色卡片和面板
- 🇨🇳 中文界面

### 用户体验

- ✅ 长时间使用更舒适（浅色背景减少眼睛疲劳）
- ✅ 红色强调色更醒目，品牌识别度更高
- ✅ 所有用户统一使用中文，减少支持成本
- ✅ 符合 WCAG 2.1 AA 无障碍标准

## 常见问题

### Q: 如何回滚到旧主题？

A: 使用 Git 恢复文件：
```bash
git checkout HEAD -- src/index.css src/contexts/LanguageContext.tsx
```

### Q: 某些第三方组件颜色不对？

A: 检查是否有 inline styles 或硬编码颜色，手动覆盖：
```css
.third-party-component {
  background: var(--panel-bg) !important;
  color: var(--text-primary) !important;
}
```

### Q: 如何临时切换回英文（开发调试）？

A: 在 `LanguageContext.tsx` 中临时修改：
```typescript
const [language] = useState<Language>('en') // Temporary for debugging
```

### Q: 对比度检查工具推荐？

A: 
- WebAIM Contrast Checker: https://webaim.org/resources/contrastchecker/
- Chrome DevTools > Lighthouse > Accessibility
- axe DevTools (浏览器扩展)

## 开发工具

### 实时预览 CSS 变量

在浏览器 DevTools Console 中：
```javascript
// 查看当前主题色
getComputedStyle(document.documentElement).getPropertyValue('--brand-red')

// 实时修改主题色
document.documentElement.style.setProperty('--brand-red', '#FF0000')
```

### 批量查找颜色引用

```bash
# 查找所有 yellow 引用
grep -rn "yellow" src/

# 查找所有黑色背景引用
grep -rn "#000000\|#0a0a0a" src/

# 查找 inline styles
grep -rn "style={{" src/
```

## 性能优化

### CSS 变量性能

- ✅ CSS 变量通过 CSS OM 直接应用，性能优于 JS 操作
- ✅ 浏览器缓存 CSS 文件，首次加载后无额外开销
- ✅ 主题切换无需重新渲染 React 组件

### 语言加载优化

- ✅ 翻译字典在构建时打包，无运行时加载
- ✅ Tree-shaking 移除未使用的翻译 key
- ✅ 无 localStorage 读写开销

## 相关文件

- 📄 `src/index.css` - 主题颜色配置
- 📄 `src/contexts/LanguageContext.tsx` - 语言上下文
- 📄 `src/i18n/translations.ts` - 翻译字典
- 📄 `tailwind.config.js` - Tailwind 配置 (可能需要同步)
- 📁 `src/components/` - 所有组件 (自动应用新主题)

## 下一步

完成主题和语言修改后，继续：
1. `/speckit.tasks` - 生成详细任务列表
2. `/speckit.implement` - 执行实际代码修改
3. 手动测试所有页面和组件
4. 使用 Playwright 进行视觉回归测试
