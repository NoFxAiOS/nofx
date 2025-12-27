# 任务清单：修复新Trader启动后无法加载AI学习数据

**提案**: TRADER-NOT-FOUND-FIX
**总工作量**: 预计 4-6 小时
**优先级**: P0 - Critical

---

## Phase 1: 代码修复

### Task 1.1: 修改LoadUserTraders验证逻辑
**状态**: ⏳ Pending
**文件**: `/nofx/manager/trader_manager.go`
**行号**: 829-862

**改动内容**:
- [ ] 移除或注释掉 `if aiModelCfg == nil { continue }` 逻辑
- [ ] 改为 WARN 日志但继续加载
- [ ] 对exchange config做同样处理
- [ ] 对enabled检查做同样处理

**验证**:
- [ ] 代码编译通过
- [ ] 逻辑审查通过

---

### Task 1.2: 修改HandleCreateTrader添加验证
**状态**: ⏳ Pending
**文件**: `/nofx/api/handlers/trader.go`
**行号**: 177-182

**改动内容**:
- [ ] 在LoadUserTraders后添加验证
- [ ] 调用GetTrader(traderID)验证加载成功
- [ ] 如果失败，返回详细错误信息

**代码片段**:
```go
// 验证trader确实被加载
_, err = h.TraderManager.GetTrader(traderID)
if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{
        "error": fmt.Sprintf("交易员已创建但加载失败: %v", err),
    })
    return
}
```

**验证**:
- [ ] 代码编译通过
- [ ] GetTrader调用成功

---

### Task 1.3: 修改HandlePerformance添加重试
**状态**: ⏳ Pending
**文件**: `/nofx/api/handlers/trader.go`
**行号**: 754-765

**改动内容**:
- [ ] 在GetTrader后添加重试逻辑
- [ ] 如果失败，重新LoadUserTraders
- [ ] 再次尝试GetTrader
- [ ] 如果仍失败，返回详细错误

**代码片段**:
```go
trader, err := h.TraderManager.GetTrader(traderID)
if err != nil {
    log.Printf("⏳ Trader在内存中未找到 %s，尝试重新加载...", traderID)
    userID := c.GetString("user_id")
    h.TraderManager.LoadUserTraders(h.Database, userID)

    trader, err = h.TraderManager.GetTrader(traderID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "error": fmt.Sprintf("交易员不存在或配置缺失: %v", err),
        })
        return
    }
}
```

**验证**:
- [ ] 代码编译通过
- [ ] 重试逻辑正确

---

## Phase 2: 单元测试

### Task 2.1: 创建trader_test.go
**状态**: ⏳ Pending
**文件**: `/nofx/api/handlers/trader_test.go` (new)

**测试用例**:

#### Test 1: CreateTrader验证加载
```go
func TestCreateTraderLoadsToMemory(t *testing.T) {
    // 创建trader
    // 验证TraderManager中存在
    // 验证GetTrader不返回错误
}
```

#### Test 2: GetPerformance重试
```go
func TestGetPerformanceRetry(t *testing.T) {
    // 创建trader但不加载到内存
    // 调用GetPerformance
    // 验证重试逻辑被触发
    // 验证返回成功（或空数据）
}
```

#### Test 3: 缺失配置的Graceful处理
```go
func TestLoadTraderWithMissingConfig(t *testing.T) {
    // 创建trader
    // 移除AI模型配置
    // 调用LoadUserTraders
    // 验证trader仍被加载
}
```

**完成标准**:
- [ ] 三个测试都编写完成
- [ ] 所有测试通过
- [ ] 代码覆盖率 ≥ 80%

---

### Task 2.2: 前端集成测试（可选）
**状态**: ⏳ Pending
**文件**: `/nofx/web/src/components/__tests__/AITradersPage.test.tsx` (update)

**改动**:
- [ ] 添加"创建trader后立即加载性能数据"的测试
- [ ] 验证AILearning组件不报错

---

## Phase 3: 验证与文档

### Task 3.1: 手动测试
**状态**: ⏳ Pending

**测试步骤**:
1. [ ] 启动后端服务
2. [ ] 在UI中创建新trader
3. [ ] 验证后端日志显示trader被加载
4. [ ] 启动trader
5. [ ] 打开AILearning组件
6. [ ] 验证不出现"trader ID不存在"错误
7. [ ] 验证性能数据能正常加载（或显示"暂无数据"）

---

### Task 3.2: 更新变更日志
**状态**: ⏳ Pending

**添加**:
- [ ] 在CHANGELOG.md中记录此bug fix
- [ ] 说明问题和解决方案
- [ ] 标记为v[next-version]

---

## Phase 4: 代码提交

### Task 4.1: Git提交
**状态**: ⏳ Pending

**提交内容**:
- [ ] `/nofx/manager/trader_manager.go` - LoadUserTraders修复
- [ ] `/nofx/api/handlers/trader.go` - HandleCreateTrader和HandlePerformance修复
- [ ] `/nofx/api/handlers/trader_test.go` - 单元测试
- [ ] `/nofx/web/openspec/bugs/ai-learning-trader-not-found/` - OpenSpec文档

**提交消息**:
```
fix(trader): fix "trader not found" error when loading AI learning data

- LoadUserTraders: relax AI model/exchange existence checks
- HandleCreateTrader: add verification that trader loads to memory
- HandlePerformance: add retry mechanism when trader not found
- Add comprehensive unit tests

Fixes: New traders couldn't start or load performance data
Closes: TRADER-NOT-FOUND-FIX
```

**验证**:
- [ ] git status 显示干净
- [ ] 代码变更正确
- [ ] 提交信息清晰

---

### Task 4.2: Push到Remote
**状态**: ⏳ Pending

**操作**:
- [ ] `git push origin main`
- [ ] 验证push成功
- [ ] 检查GitHub上的commit

---

## 进度追踪

### Phase 1: 代码修复
- [ ] Task 1.1 - LoadUserTraders
- [ ] Task 1.2 - HandleCreateTrader
- [ ] Task 1.3 - HandlePerformance

### Phase 2: 单元测试
- [ ] Task 2.1 - trader_test.go
- [ ] Task 2.2 - 前端测试 (optional)

### Phase 3: 验证与文档
- [ ] Task 3.1 - 手动测试
- [ ] Task 3.2 - CHANGELOG

### Phase 4: 代码提交
- [ ] Task 4.1 - git commit
- [ ] Task 4.2 - git push

---

## 风险与缓解

### 风险1: LoadUserTraders验证放宽可能导致运行时错误
**缓解**: 在trader.Run()中添加配置检查，失败时返回清晰错误

### 风险2: 重试逻辑可能导致性能下降
**缓解**: 重试仅在首次失败时触发，不会重复调用

### 风险3: 并发创建多个trader时的race condition
**缓解**: TraderManager已有mutex保护，应该没问题

---

## 资源需求

- 开发: 1人
- 代码审查: 1人
- 测试: 可自动化
- 部署: 1人

---

## 参考文件

- Bug分析: `/nofx/web/openspec/bugs/ai-learning-trader-not-found-bug.md`
- OpenSpec提案: `/nofx/web/openspec/bugs/ai-learning-trader-not-found/proposal.md`
- 管理器代码: `/nofx/manager/trader_manager.go:741-872`
- 处理器代码: `/nofx/api/handlers/trader.go:64-192, 753-777`

---

**最后更新**: 2025-12-27
**版本**: 1.0
