# CORS白名单扩展 - 任务清单

## 📋 任务概览

**总任务数**: 7
**已完成**: 1
**进行中**: 1
**待开始**: 5

---

## ✅ 任务列表

### 任务 #1: 创建OpenSpec提案
- [x] 创建proposal.md
- [x] 创建specs/cors-config-spec.md
- [x] 创建tasks.md (本文件)
- [x] 提交提案到Git
- [ ] 代码审查并批准

**负责人**: Claude Code
**开始时间**: 2025-11-22 11:55
**预计工时**: 0.5h
**实际工时**: 0.5h
**状态**: ✅ 完成

---

### 任务 #2: 更新后端CORS配置
- [ ] 编辑 `api/server.go`
- [ ] 修改 `corsMiddleware` 函数
- [ ] 添加所有Vercel域名到默认白名单
- [ ] 添加注释说明域名来源
- [ ] 运行Go编译检查
- [ ] Go vet检查

**负责人**: 后端开发
**开始时间**: 待定
**预计工时**: 0.5h
**依赖**: 任务 #1 完成

**详细步骤**:
1. 打开 `api/server.go`
2. 定位 `corsMiddleware` 函数 (52-99行)
3. 更新 `allowedOrigins` 数组：
   ```go
   allowedOrigins := []string{
       // 开发环境
       "http://localhost:3000",
       "http://localhost:5173",

       // Vercel部署域名
       "https://web-3c7a7psvt-gyc567s-projects.vercel.app",
       "https://web-pink-omega-40.vercel.app",
       "https://web-gyc567s-projects.vercel.app",
       "https://web-7jc87z3u4-gyc567s-projects.vercel.app",
       "https://web-gyc567-gyc567s-projects.vercel.app",
       "https://web-fej4rs4y2-gyc567s-projects.vercel.app",
   }
   ```
4. 提交代码
5. 编译验证：
   ```bash
   go build -o nofx-backend ./main.go
   go vet ./...
   ```

---

### 任务 #3: 配置环境变量
- [ ] 准备环境变量配置清单
- [ ] 配置生产环境变量 (Replit)
- [ ] 验证环境变量生效
- [ ] 更新部署文档

**负责人**: DevOps
**开始时间**: 待定
**预计工时**: 0.25h
**依赖**: 任务 #2 完成

**配置内容**:
```bash
# 在 Replit Secrets 中添加
ALLOWED_ORIGINS=https://web-3c7a7psvt-gyc567s-projects.vercel.app,https://web-pink-omega-40.vercel.app,https://web-gyc567s-projects.vercel.app
```

**验证命令**:
```bash
# 检查环境变量
echo $ALLOWED_ORIGINS
```

---

### 任务 #4: 单元测试
- [ ] 编写CORS测试用例
- [ ] 测试允许的域名
- [ ] 测试拒绝的域名
- [ ] 测试环境变量覆盖
- [ ] 运行所有测试

**负责人**: 后端开发
**开始时间**: 待定
**预计工时**: 0.5h
**依赖**: 任务 #3 完成

**测试文件**: `api/cors_test.go`

**测试用例**:
```go
func TestCORSAllowedOrigins(t *testing.T) {
    // 测试本地开发域名
    // 测试Vercel域名
    // 测试拒绝未知域名
}
```

**运行测试**:
```bash
go test ./api/... -v
```

---

### 任务 #5: 集成测试
- [ ] 测试CORS预检请求
- [ ] 测试API实际调用
- [ ] 测试多个Vercel域名
- [ ] 验证数据加载

**负责人**: QA
**开始时间**: 待定
**预计工时**: 0.5h
**依赖**: 任务 #4 完成

**测试步骤**:
```bash
# 1. 测试 web-pink-omega-40.vercel.app
curl -H "Origin: https://web-pink-omega-40.vercel.app" \
     -X OPTIONS https://nofx-gyc567.replit.app/api/competition -I

# 2. 测试实际API调用
curl -H "Origin: https://web-pink-omega-40.vercel.app" \
     https://nofx-gyc567.replit.app/api/competition
```

**浏览器测试**:
- 访问 `https://web-pink-omega-40.vercel.app/competition`
- 打开开发者工具
- 检查Network面板，无CORS错误
- 检查Console面板，无错误信息

---

### 任务 #6: 部署验证
- [ ] 重新构建后端
- [ ] 重启后端服务
- [ ] 验证服务正常运行
- [ ] 监控日志

**负责人**: DevOps
**开始时间**: 待定
**预计工时**: 0.25h
**依赖**: 任务 #5 完成

**部署步骤**:
```bash
# 1. 构建
go build -o nofx-backend ./main.go

# 2. 重启服务 (示例)
pm2 restart nofx-backend
# 或
systemctl restart nofx-backend

# 3. 验证
curl https://nofx-gyc567.replit.app/api/health
```

**监控命令**:
```bash
# 查看CORS相关日志
grep -i cors /var/log/nofx-backend.log

# 查看错误
tail -f /var/log/nofx-backend.log
```

---

### 任务 #7: 前端验证
- [ ] 访问所有Vercel部署实例
- [ ] 验证数据显示正常
- [ ] 测试关键功能
- [ ] 生成验证报告

**负责人**: QA
**开始时间**: 待定
**预计工时**: 0.5h
**依赖**: 任务 #6 完成

**验证清单**:

| 域名 | 页面 | 状态 | 备注 |
|------|------|------|------|
| `web-3c7a7psvt-gyc567s-projects.vercel.app` | `/dashboard` | 待测试 | - |
| `web-pink-omega-40.vercel.app` | `/competition` | 待测试 | 主要问题域名 |
| `web-gyc567s-projects.vercel.app` | `/` | 待测试 | - |

**测试步骤**:
1. 打开浏览器 (Chrome/Firefox)
2. 访问每个域名
3. 检查页面加载
4. 检查数据加载
5. 测试关键功能 (登录、注册、数据刷新)
6. 记录测试结果

---

## 📊 进度追踪

### 每日进度

| 日期 | 完成任务 | 完成率 | 阻塞项 |
|------|----------|--------|--------|
| 2025-11-22 | 任务 #1 | 14% | 无 |

### 里程碑

- [ ] **M1: 代码修改完成** (任务 #2 完成)
  - 目标时间: 2025-11-22 12:30
  - 状态: 未开始

- [ ] **M2: 测试通过** (任务 #4, #5 完成)
  - 目标时间: 2025-11-22 13:30
  - 状态: 未开始

- [ ] **M3: 部署完成** (任务 #6, #7 完成)
  - 目标时间: 2025-11-22 14:00
  - 状态: 未开始

---

## 🚨 风险与阻塞

### 当前风险

| 风险项 | 影响 | 概率 | 应对措施 |
|--------|------|------|----------|
| 环境变量配置错误 | 高 | 中 | 双人验证配置 |
| 测试遗漏域名 | 中 | 低 | 完整域名清单 |
| 部署失败 | 高 | 低 | 准备回滚方案 |

### 应对措施

1. **配置验证**:
   - 使用脚本验证环境变量
   - 两人检查机制

2. **全面测试**:
   - 自动化测试覆盖
   - 手动测试关键场景

3. **快速回滚**:
   - 回滚方案准备就绪
   - 5分钟内可回滚

---

## 📝 代码变更记录

### 文件变更

1. **api/server.go**
   - 函数: `corsMiddleware`
   - 新增: +20行 (Vercel域名列表)
   - 修改: 默认白名单配置

2. **新增文件: api/cors_test.go**
   - CORS单元测试
   - 测试用例: 5个

---

## ✅ 验收标准

### 必选项

- [ ] 所有Vercel域名CORS检查通过
- [ ] `web-pink-omega-40.vercel.app` 数据正常加载
- [ ] 开发环境无影响
- [ ] 编译无警告
- [ ] 单元测试100%通过

### 加分项

- [ ] 自动化测试覆盖 > 90%
- [ ] 性能测试 (CORS检查 < 1ms)
- [ ] 文档更新完整
- [ ] 监控配置完善

---

## 🔍 测试用例详细

### 后端测试

```bash
# 测试1: 允许的域名
curl -H "Origin: https://web-pink-omega-40.vercel.app" \
     -X OPTIONS https://nofx-gyc567.replit.app/api/competition -v
# 预期: 200 OK + Access-Control-Allow-Origin

# 测试2: 拒绝的域名
curl -H "Origin: https://evil.com" \
     -X OPTIONS https://nofx-gyc567.replit.app/api/competition -v
# 预期: 200 OK 但无 Access-Control-Allow-Origin

# 测试3: 实际API调用
curl -H "Origin: https://web-pink-omega-40.vercel.app" \
     -H "Authorization: Bearer <token>" \
     https://nofx-gyc567.replit.app/api/competition
# 预期: 200 OK + JSON数据
```

### 前端测试

```
访问: https://web-pink-omega-40.vercel.app/competition

Expected Results:
✅ 页面正常加载
✅ 数据正常显示
✅ 无CORS错误
✅ Network面板所有请求成功
✅ Console无错误信息
```

---

## 📞 联系信息

**项目负责人**: 开发团队
**技术负责人**: [待指派]
**QA负责人**: [待指派]
**DevOps负责人**: [待指派]

**紧急联系**: Slack #cors-fix频道
**状态会议**: 完成后总结会议

---

## 🔗 相关链接

- [原始提案](./proposal.md)
- [技术规范](./specs/cors-config-spec.md)
- [P0认证修复报告](../../fix-p0-auth-issues/P0_AUTH_FIX_SUMMARY.md)
- [CORS修复指南](../fix-cors-policy-error/api-spec.md)

---

**文档创建**: 2025-11-22 11:55
**最后更新**: 2025-11-22 11:55
**维护者**: Claude Code
**审核状态**: 待审核
