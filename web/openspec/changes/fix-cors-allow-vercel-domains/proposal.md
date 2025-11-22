# CORS域名白名单扩展提案

## 📋 提案概述

**提案名称**: 扩展CORS白名单支持Vercel部署域名
**提案类型**: 安全配置修复
**优先级**: P0 (紧急修复)
**影响范围**: 所有Vercel部署的前端应用
**预估工期**: 1小时

## 🚨 问题描述

### 当前状况

**背景**：
在之前的P0修复中（提交f64aa6e），我们实施了CORS域名白名单机制以提升安全性。然而，这一安全措施导致了一个意外问题：

**具体问题**：
1. **CORS白名单不完整**：当前配置仅允许本地开发域名
2. **Vercel部署受阻**：所有 `*.vercel.app` 域名被阻止访问API
3. **数据显示异常**：用户访问部署的网站时无法获取数据
4. **影响用户体验**：多个部署实例受影响

### 影响范围

**受影响的域名**：
- ❌ `https://web-pink-omega-40.vercel.app/competition` - 数据无法加载
- ❌ `https://web-gyc567s-projects.vercel.app` - API请求被拒绝
- ❌ 其他所有新的Vercel部署实例

**可能正常的域名**：
- ✅ `https://web-3c7a7psvt-gyc567s-projects.vercel.app/dashboard` - 使用旧版本或缓存

### 根因分析

当前CORS配置（`api/server.go:55-60`）：
```go
allowedOrigins := []string{
    "http://localhost:3000",
    "http://localhost:5173",
    "http://127.0.0.1:3000",
    "http://127.0.0.1:5173",
}
```

**问题**：
- 缺少所有Vercel部署域名
- 环境变量 `ALLOWED_ORIGINS` 可能未正确设置
- 没有动态域名管理机制

## 💡 解决方案

### 方案选择：更新CORS白名单 + 环境变量配置

**选择理由**：
1. **安全性**：保留域名白名单机制，不使用通配符
2. **灵活性**：支持环境变量动态配置
3. **可维护性**：易于添加新域名
4. **快速实施**：1小时内完成修复

### 实施计划

#### 步骤1: 更新CORS白名单
- 编辑 `api/server.go` 的 `corsMiddleware` 函数
- 添加所有已知的Vercel域名到默认白名单
- 确保开发环境域名正常

#### 步骤2: 配置环境变量
- 在后端部署环境设置 `ALLOWED_ORIGINS`
- 包含所有活跃的Vercel域名
- 支持动态更新

#### 步骤3: 测试验证
- 测试CORS预检请求
- 验证API调用正常
- 确认数据加载

## 🎯 目标

### 主要目标

1. **立即修复**：所有Vercel部署实例能够正常访问API
2. **保持安全**：仅允许指定域名，不使用通配符
3. **易于维护**：支持环境变量动态配置

### 成功标准

- [ ] `https://web-pink-omega-40.vercel.app/competition` 数据正常加载
- [ ] 所有Vercel域名API调用成功（CORS检查通过）
- [ ] 开发环境正常工作
- [ ] 无安全风险（不使用 `*` 通配符）

## 📊 影响评估

### 正面影响

- ✅ 修复数据无法加载问题
- ✅ 恢复所有Vercel部署实例的正常功能
- ✅ 提升用户体验
- ✅ 保持CORS安全机制

### 风险评估

- **安全风险**: 低
  - 仅添加特定域名，非通配符
  - 保留安全白名单机制

- **兼容性风险**: 无
  - 向后兼容现有功能
  - 不影响API逻辑

- **性能影响**: 无
  - 仅增加域名检查（O(n)，n<20）
  - 可忽略不计

## 🛠️ 技术细节

### 修改文件

1. **api/server.go**
   - 函数: `corsMiddleware`
   - 修改: 添加Vercel域名到默认白名单
   - 影响: 提升CORS配置完整性

### 配置变更

**环境变量**:
```bash
ALLOWED_ORIGINS="https://web-3c7a7psvt-gyc567s-projects.vercel.app,https://web-pink-omega-40.vercel.app,https://web-gyc567s-projects.vercel.app"
```

**默认值**:
```go
allowedOrigins := []string{
    "http://localhost:3000",
    "http://localhost:5173",
    "https://web-3c7a7psvt-gyc567s-projects.vercel.app",
    "https://web-pink-omega-40.vercel.app",
    "https://web-gyc567s-projects.vercel.app",
    // 更多域名...
}
```

## 📅 时间表

| 阶段 | 任务 | 工时 | 负责人 |
|------|------|------|--------|
| 1 | 创建OpenSpec提案 | 0.5h | 开发 |
| 2 | 更新CORS白名单 | 0.5h | 后端 |
| 3 | 配置环境变量 | 0.25h | DevOps |
| 4 | 测试验证 | 0.5h | QA |
| 5 | 部署上线 | 0.25h | DevOps |

**总工期**: 2小时
**紧急程度**: P0 - 立即实施

## ✅ 验收标准

### 功能验收

- [ ] CORS预检请求返回正确头
- [ ] 所有Vercel域名API调用成功
- [ ] 前端数据正常加载
- [ ] 开发环境无影响

### 安全验收

- [ ] 无通配符 `*` 使用
- [ ] 仅允许指定域名
- [ ] 环境变量正确配置
- [ ] 通过安全检查

### 测试验收

- [ ] 单元测试通过
- [ ] 集成测试通过
- [ ] 浏览器兼容性测试
- [ ] 回归测试通过

## 🔄 回滚方案

### 回滚条件

- 发现严重安全问题
- CORS配置导致系统不稳定
- 意外拒绝合法请求

### 回滚步骤

1. **立即回滚**
   ```bash
   # 恢复CORS为通配符（仅开发环境）
   c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
   ```

2. **重新部署**
   ```bash
   go build -o nofx-backend ./main.go
   ```

3. **监控验证**
   - 检查错误率
   - 确认API正常

## 📚 相关文档

- [P0认证修复报告](../../fix-p0-auth-issues/P0_AUTH_FIX_SUMMARY.md)
- [CORS配置指南](../fix-cors-policy-error/api-spec.md)
- [Vercel部署指南](../../DEPLOYMENT_SUCCESS.md)

## 👥 参与人员

**提案人**: Claude Code
**审核人**: 技术负责人
**实施人**: 后端开发团队
**测试人**: QA团队
**部署人**: DevOps团队

---

**提案状态**: 待批准
**创建时间**: 2025-11-22
**最后更新**: 2025-11-22
**批准人**: [待填写]
