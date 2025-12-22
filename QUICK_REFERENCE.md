# 快速参考指南 - 新闻源配置功能

## 一键测试

### 后端测试
```bash
# 运行所有API集成测试
cd /Users/guoyingcheng/dreame/code/nofx
go test ./api -v -run "TestAPI"

# 运行特定测试
go test ./api -v -run "TestAPI_CreateNewsConfig"

# 运行所有后端测试
go test ./...
```

### 前端E2E测试
```bash
# 进入前端目录
cd /Users/guoyingcheng/dreame/code/nofx/web

# 安装依赖（首次）
npm install

# 运行所有E2E测试
npm run test:e2e

# 交互式UI模式
npm run test:e2e:ui

# 调试特定测试
npm run test:e2e:debug

# 运行特定测试文件
npx playwright test tests/news-config.e2e.spec.ts

# 运行特定测试用例
npx playwright test -g "should open news config modal"
```

## 关键文件位置

### 后端
| 功能 | 文件 | 行数 |
|------|------|------|
| API处理器 | `nofx/api/news_config_handler.go` | 350 |
| API测试 | `nofx/api/news_config_integration_test.go` | 320 |
| 数据库 | `nofx/database/user_news_config_repository.go` | 280 |
| 接口定义 | `nofx/database/news_config_repository.go` | 20 |
| 数据库迁移 | `nofx/database/migration.sql` | 创建表 |
| 日志 | `nofx/logger/news_config_logger.go` | 280 |
| 特性标志 | `nofx/config/feature_flags.go` | 350 |

### 前端
| 功能 | 文件 | 行数 |
|------|------|------|
| Modal组件 | `nofx/web/src/components/NewsSourceModal.tsx` | 280 |
| 页面组件 | `nofx/web/src/components/NewsConfigPage.tsx` | 280 |
| E2E测试 | `nofx/web/tests/news-config.e2e.spec.ts` | 500 |
| Playwright配置 | `nofx/web/playwright.config.ts` | 40 |

### 文档
| 文档 | 位置 | 用途 |
|------|------|------|
| 集成指南 | `nofx/web/src/components/INTEGRATION_GUIDE.md` | 前端集成说明 |
| E2E指南 | `nofx/web/tests/E2E_GUIDE.md` | E2E测试运行 |
| Phase 3报告 | `nofx/PHASE_3_COMPLETION.md` | Phase 3总结 |
| 项目总结 | `nofx/NEWS_SOURCE_FEATURE_COMPLETE.md` | 完整总结 |

## API端点

```
GET    /api/user/news-config
   获取当前用户的新闻配置
   Header: Authorization: Bearer <token>
   响应: { code: 200, data: UserNewsConfig }

POST   /api/user/news-config
   创建或更新用户新闻配置
   Header: Authorization: Bearer <token>
   Body: { enabled, news_sources, auto_fetch_interval_minutes, max_articles_per_fetch, sentiment_threshold }
   响应: { code: 201, data: UserNewsConfig }

PUT    /api/user/news-config
   更新用户新闻配置（支持部分更新）
   Header: Authorization: Bearer <token>
   Body: 同POST（可选字段）
   响应: { code: 200, data: UserNewsConfig }

DELETE /api/user/news-config
   删除用户新闻配置
   Header: Authorization: Bearer <token>
   响应: { code: 200, message: "deleted" }

GET    /api/user/news-config/sources
   获取启用的新闻源列表
   Header: Authorization: Bearer <token>
   响应: { code: 200, data: ["mlion", "twitter"] }
```

## 表单验证规则

### 新闻源
- 允许值: `mlion`, `twitter`, `reddit`, `telegram`
- 必须至少选择一个
- 以逗号分隔

### 抓取间隔
- 范围: 1-1440 分钟
- 建议: 5-60 分钟

### 最大文章数
- 范围: 1-100 篇
- 建议: 10-50 篇

### 情绪阈值
- 范围: -1.0 到 1.0
- -1.0: 极度负面
- 0.0: 中立
- 1.0: 极度正面

## 测试场景检查表

### 后端集成测试
- [x] CRUD完整操作
- [x] 新闻源验证
- [x] 抓取间隔验证
- [x] 文章数验证
- [x] 情绪阈值验证
- [x] 权限检查

### 前端E2E测试
- [x] 模态框打开/关闭
- [x] 新闻源选择
- [x] 参数输入
- [x] 表单验证
- [x] 成功保存
- [x] 错误提示
- [x] 暗模式支持
- [x] 多浏览器兼容
- [x] 配置页面显示

## 常见命令

### 开发流程
```bash
# 启动后端开发服务
cd /Users/guoyingcheng/dreame/code/nofx
go run main.go

# 启动前端开发服务
cd /Users/guoyingcheng/dreame/code/nofx/web
npm run dev

# 构建前端
npm run build

# 运行测试
npm test              # 单元测试
npm run test:e2e     # E2E测试
npm run test:e2e:ui  # UI模式

# 查看E2E报告
npx playwright show-report
```

### 数据库操作
```bash
# 应用迁移
psql -f nofx/database/migration.sql

# 查看表结构
psql -c "\d user_news_config"

# 查询配置
psql -c "SELECT * FROM user_news_config WHERE user_id = 'test-user';"
```

### Git操作
```bash
# 查看当前状态
git status

# 查看最近提交
git log --oneline -5

# 提交更改
git add .
git commit -m "feat(news): complete phase 3 integration and e2e tests"

# 推送
git push origin main
```

## 故障排除

### 后端测试失败
```bash
# 清理编译缓存
go clean -testcache
go test ./api -v

# 检查数据库连接
go test ./database -v

# 查看详细错误
go test ./api -v -count=1
```

### 前端E2E失败
```bash
# 清理依赖
rm -rf node_modules package-lock.json
npm install

# 调试特定测试
npx playwright test -g "test name" --debug

# 查看失败的视频
npx playwright show-report

# 检查浏览器安装
npx playwright install
```

### 构建失败
```bash
# 检查Go版本
go version  # 需要 >=1.19

# 检查Node版本
node -v     # 需要 >=16

# 检查npm依赖
npm list

# 清理npm缓存
npm cache clean --force
```

## 性能指标

### 后端
- API响应时间: <50ms（不含网络延迟）
- 数据库查询: <10ms
- 日志写入: <5ms

### 前端
- 页面加载: <2s
- Modal打开: <500ms
- 表单验证: <50ms
- API调用: 依赖网络

## 监控要点

### 日志文件位置
```
/var/log/nofx/news_config.log
```

### 关键指标
```
- 新配置创建数
- 配置更新频率
- 验证错误率
- API错误率
- 平均响应时间
```

### 特性标志检查
```go
// 检查新闻自动抓取是否启用
if featureFlagManager.IsEnabled("news.auto_fetch_enabled") {
    // 启用自动抓取
}

// 检查特定用户是否启用注入防护
if featureFlagManager.IsEnabledForUser("news.prompt_injection_protection", userID) {
    // 执行注入防护
}
```

## 部署清单

- [ ] 数据库迁移已执行
- [ ] 环境变量已配置
- [ ] API密钥已设置
- [ ] 日志目录已创建
- [ ] 权限已配置
- [ ] 测试全部通过
- [ ] 文档已更新
- [ ] 备份已完成
- [ ] 监控已部署
- [ ] 告警已配置

## 技术栈总览

### 后端
- 语言: Go 1.19+
- Web框架: Gin
- 数据库: PostgreSQL
- 测试: Go Testing
- 日志: JSON structured logging

### 前端
- 框架: React 18+
- 语言: TypeScript
- 样式: Tailwind CSS
- 测试: Playwright
- 打包: Vite

### 基础设施
- 容器: Docker（可选）
- CI/CD: GitHub Actions（可选）
- 监控: 结构化日志
- 部署: 标准Go/Node部署

## 联系与支持

### 文档
- API文档: `nofx/api/README.md`
- 前端指南: `nofx/web/INTEGRATION_GUIDE.md`
- E2E指南: `nofx/web/tests/E2E_GUIDE.md`

### 常见问题
1. **Q: 如何添加新的新闻源？**
   A: 修改AVAILABLE_SOURCES常量，更新验证规则，添加测试

2. **Q: 如何禁用某个功能？**
   A: 使用特性标志或修改代码逻辑

3. **Q: 如何处理性能问题？**
   A: 检查数据库索引，启用缓存，增加并发限制

4. **Q: 如何扩展功能？**
   A: 查看PHASE_3_COMPLETION.md的"未来增强"部分

---

最后更新: 2025年12月21日
版本: 1.0 (Production Ready)
