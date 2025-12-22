# Phase 2: 前端NewsSourceModal集成指南

## 概述
实现了两个新的React组件，用于管理用户的新闻源配置：
- `NewsSourceModal.tsx` - 新闻源配置弹窗组件
- `NewsConfigPage.tsx` - 新闻配置管理页面

## 组件说明

### 1. NewsSourceModal Component
**位置**: `web/src/components/NewsSourceModal.tsx`

**功能**:
- 新闻源选择（Mlion, Twitter, Reddit, Telegram）
- 启用/禁用新闻功能
- 配置自动抓取间隔（1-1440分钟）
- 设置每次最多文章数（1-100）
- 情绪阈值调整（-1.0 到 1.0）

**Props**:
```typescript
interface NewsSourceModalProps {
  isOpen: boolean;                    // 是否显示模态框
  onClose: () => void;               // 关闭回调
  onSave?: (data: NewsConfigData) => Promise<void>; // 保存回调
  initialData?: NewsConfigData | null; // 初始数据（编辑模式）
}
```

**使用方式**:
```tsx
import { NewsSourceModal } from './NewsSourceModal';

function MyComponent() {
  const [showModal, setShowModal] = useState(false);

  return (
    <>
      <button onClick={() => setShowModal(true)}>
        配置新闻源
      </button>

      <NewsSourceModal
        isOpen={showModal}
        onClose={() => setShowModal(false)}
        initialData={currentConfig}
        onSave={async (data) => {
          // 保存配置逻辑
        }}
      />
    </>
  );
}
```

### 2. NewsConfigPage Component
**位置**: `web/src/components/NewsConfigPage.tsx`

**功能**:
- 显示当前新闻配置状态
- 编辑/删除配置
- 信息提示和说明
- 响应式设计

**使用方式**:
```tsx
import { NewsConfigPage } from './NewsConfigPage';

// 在路由中添加
<Route path="/news-config" element={<NewsConfigPage />} />
```

## 集成步骤

### 步骤1: 在AITradersPage中添加新闻配置按钮

```tsx
import { NewsSourceModal } from './NewsSourceModal';

export function AITradersPage() {
  const [showNewsConfigModal, setShowNewsConfigModal] = useState(false);

  return (
    <>
      {/* 现有内容 */}
      <div className="flex gap-2 mb-4">
        <button onClick={() => setShowCreateModal(true)}>
          创建交易员
        </button>
        <button onClick={() => setShowNewsConfigModal(true)}>
          📰 配置新闻源
        </button>
      </div>

      {/* 现有modals */}

      {/* 新增 */}
      <NewsSourceModal
        isOpen={showNewsConfigModal}
        onClose={() => setShowNewsConfigModal(false)}
      />
    </>
  );
}
```

### 步骤2: 在路由中添加新闻配置页面

编辑 `web/src/App.tsx` 或路由配置文件：

```tsx
import { NewsConfigPage } from './components/NewsConfigPage';

// 在路由定义中添加
<Route path="/news-config" element={<NewsConfigPage />} />

// 在导航菜单中添加
<li>
  <Link to="/news-config" className="flex items-center gap-2">
    📰 新闻配置
  </Link>
</li>
```

### 步骤3: 在导航菜单中添加链接

编辑 `web/src/components/HeaderBar.tsx` 或导航组件：

```tsx
<Link
  to="/news-config"
  className="flex items-center gap-2 px-4 py-2 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg"
>
  <Newspaper size={20} />
  新闻配置
</Link>
```

## API集成

组件自动调用以下API端点：

```
GET    /api/user/news-config           - 获取用户配置
POST   /api/user/news-config           - 创建配置
PUT    /api/user/news-config           - 更新配置
DELETE /api/user/news-config           - 删除配置
GET    /api/user/news-config/sources   - 获取启用的新闻源
```

## 样式说明

组件使用的CSS类：
- Tailwind CSS 深色模式支持（`dark:` 前缀）
- 响应式布局（`grid-cols-2` 平板及以上）
- 动画支持（加载状态、开关）

## 依赖项

- React Hooks (useState, useEffect)
- 自定义Hooks: `useLanguage`, `useAuth`
- lucide-react icons
- Tailwind CSS

## 特性

✅ 完整的表单验证
✅ 错误处理和显示
✅ 加载状态指示
✅ 成功提示反馈
✅ 深色模式支持
✅ 响应式设计
✅ 国际化支持（准备）

## 后续集成

1. **国际化**: 添加多语言支持
   ```tsx
   const translations = {
     zh: { title: '新闻源配置', ... },
     en: { title: 'News Source Configuration', ... }
   };
   ```

2. **分析跟踪**: 添加用户行为追踪
   ```tsx
   analytics.track('news_config_saved', { sources: [...] });
   ```

3. **权限检查**: 根据用户角色限制访问
   ```tsx
   if (!user.canConfigureNews) return <AccessDenied />;
   ```

4. **实时同步**: 使用WebSocket或Server-Sent Events
   ```tsx
   useEffect(() => {
     const unsubscribe = subscribeToNewsConfigChanges(onConfigChanged);
     return unsubscribe;
   }, []);
   ```

## 测试建议

```tsx
// 单元测试示例
describe('NewsSourceModal', () => {
  it('should validate news sources', () => {
    // 至少选择一个新闻源
  });

  it('should validate fetch interval', () => {
    // 间隔在1-1440范围内
  });

  it('should save configuration', () => {
    // 验证API调用
  });
});
```

## 完成清单

- ✅ NewsSourceModal 组件实现
- ✅ NewsConfigPage 页面实现
- ✅ API 集成
- ✅ 表单验证
- ✅ 错误处理
- ✅ 深色模式支持
- ⬜ 单元测试
- ⬜ E2E 测试
- ⬜ 国际化配置
- ⬜ 集成到 AITradersPage
- ⬜ 路由配置
- ⬜ 菜单链接
