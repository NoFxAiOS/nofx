# 🔍 维护者 PR 审核指南

**语言：** [English](PR_REVIEW_GUIDE.md) | [中文](PR_REVIEW_GUIDE.zh-CN.md)

本指南适用于审核 pull request 的 NOFX 维护者。

---

## 📋 审核清单

### 1. 初步分类（24小时内）

- [ ] **检查 PR 与路线图的一致性**
  - 是否符合我们当前的优先级？
  - 是否在[路线图](../roadmap/README.zh-CN.md)中？
  - 如果不在，我们是否应该接受它？

- [ ] **验证 PR 完整性**
  - PR 模板的所有部分都已填写？
  - 变更描述清晰？
  - 相关 issue 已链接？
  - UI 变更有截图/演示？

- [ ] **应用适当的标签**
  - 优先级：critical/high/medium/low
  - 类型：bug/feature/enhancement/docs
  - 区域：frontend/backend/exchange/ai/security
  - 状态：needs review/needs changes

- [ ] **分配审核者**
  - 根据专业领域分配
  - 至少需要 1 个维护者审核

### 2. 代码审核

#### A. 功能审核

```markdown
✅ **要问的问题：**

- 是否解决了所述问题？
- 边界情况是否处理？
- 是否会破坏现有功能？
- 方法是否适合我们的架构？
- 是否有更好的替代方案？
```

**测试：**
- [ ] 所有 CI 检查都通过？
- [ ] 贡献者进行了手动测试？
- [ ] 测试覆盖率足够？
- [ ] 测试有意义（不只是为了覆盖率）？

#### B. 代码质量审核

**Go 后端代码：**

```go
// ❌ 差 - 拒绝
func GetData(a, b string) interface{} {
    d := doSomething(a, b)
    return d
}

// ✅ 好 - 批准
func GetAccountBalance(apiKey, secretKey string) (*Balance, error) {
    if apiKey == "" || secretKey == "" {
        return nil, fmt.Errorf("API credentials required")
    }

    balance, err := client.FetchBalance(apiKey, secretKey)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch balance: %w", err)
    }

    return balance, nil
}
```

**检查项：**
- [ ] 有意义的变量/函数名
- [ ] 正确的错误处理（没有忽略错误）
- [ ] 复杂逻辑有注释
- [ ] 没有硬编码值（使用常量/配置）
- [ ] 遵循 Go 习惯用法和约定
- [ ] 没有不必要的复杂性

**TypeScript/React 前端代码：**

```typescript
// ❌ 差 - 拒绝
const getData = (data: any) => {
  return data.map(d => <div>{d.name}</div>)
}

// ✅ 好 - 批准
interface Trader {
  id: string;
  name: string;
  status: 'running' | 'stopped';
}

const TraderList: React.FC<{ traders: Trader[] }> = ({ traders }) => {
  return (
    <div className="trader-list">
      {traders.map(trader => (
        <TraderCard key={trader.id} trader={trader} />
      ))}
    </div>
  );
};
```

**检查项：**
- [ ] 类型安全（除非绝对必要，否则不使用 `any`）
- [ ] 正确的 React 模式（hooks、函数式组件）
- [ ] 组件可重用性
- [ ] 可访问性（a11y）考虑
- [ ] 性能优化（需要时使用 memoization）

#### C. 安全审核

**关键检查：**

```go
// 🚨 拒绝 - 安全问题
func Login(username, password string) {
    query := "SELECT * FROM users WHERE username='" + username + "'"  // SQL 注入！
    db.Query(query)
}

// ✅ 批准 - 安全
func Login(username, password string) error {
    query := "SELECT * FROM users WHERE username = ?"
    row := db.QueryRow(query, username)  // 参数化查询
    // ... 使用 bcrypt 进行正确的密码验证
}
```

- [ ] 没有 SQL 注入漏洞
- [ ] 前端没有 XSS 漏洞
- [ ] API 密钥/密码没有硬编码
- [ ] 用户输入已正确验证
- [ ] 认证/授权正确处理
- [ ] 日志中没有敏感数据
- [ ] 依赖项没有已知漏洞

#### D. 性能审核

- [ ] 没有明显的性能问题
- [ ] 数据库查询已优化（索引、没有 N+1 查询）
- [ ] 没有不必要的 API 调用
- [ ] 适当的缓存
- [ ] 没有内存泄漏

### 3. 文档审核

- [ ] 复杂逻辑有代码注释
- [ ] 如果需要，README 已更新
- [ ] API 文档已更新（如有 API 变更）
- [ ] 破坏性变更有迁移指南
- [ ] Changelog 条目（对于重大变更）

### 4. 测试审核

- [ ] 新函数有单元测试
- [ ] 新功能有集成测试
- [ ] 测试确实测试了功能（不只是覆盖率）
- [ ] 测试名称具有描述性
- [ ] 模拟数据真实

---

## 🏷️ 标签管理

### 优先级分配

使用这些标准来分配优先级：

**Critical（严重）：**
- 安全漏洞
- 生产环境破坏性 bug
- 数据丢失问题

**High（高）：**
- 影响许多用户的重大 bug
- 高优先级路线图功能
- 性能问题

**Medium（中）：**
- 常规 bug 修复
- 标准功能请求
- 重构

**Low（低）：**
- 次要改进
- 代码风格变更
- 非紧急文档

### 状态工作流

```
needs review → in review → needs changes → needs review → approved → merged
                       ↓
                   on hold
```

**状态标签：**
- `status: needs review` - 准备初次审核
- `status: in progress` - 正在积极审核
- `status: needs changes` - 审核者请求更改
- `status: on hold` - 等待讨论/决定
- `status: blocked` - 被另一个 PR/issue 阻塞

---

## 💬 提供反馈

### 编写好的审核评论

**❌ 差的评论：**
```
这是错的。
改这个。
你为什么这样做？
```

**✅ 好的评论：**
```
这种方法可能会导致并发请求的问题。
考虑在这里使用互斥锁或原子操作。

建议：将此逻辑提取到单独的函数中以提高可测试性：
```go
func validateTraderConfig(config *TraderConfig) error {
    // 验证逻辑
}
```

问题：你是否考虑过使用现有的 `ExchangeClient` 接口
而不是创建新接口？这将与代码库的其余部分保持一致。
```

### 评论类型

**🔴 阻塞性（必须解决）：**
```markdown
**阻塞性：** 这引入了 SQL 注入漏洞。
请改用参数化查询。
```

**🟡 非阻塞性（建议）：**
```markdown
**建议：** 考虑在这里使用 `strings.Builder` 以提高
连接多个字符串时的性能。
```

**🟢 赞扬（鼓励好的做法）：**
```markdown
**很好！** 很好地使用 context 进行超时处理。这正是
我们想看到的。
```

### 问题 vs 指令

**❌ 指令（可能感觉强硬）：**
```
改用工厂模式。
为这个函数添加测试。
```

**✅ 问题（更协作）：**
```
工厂模式在这里会更合适吗？它可能会使测试更容易。
你能为错误路径添加一个测试用例吗？我想确保我们
优雅地处理失败。
```

---

## ⏱️ 响应时间指南

| PR 类型 | 初次审核 | 后续审核 | 合并决定 |
|---------|----------|----------|----------|
| **严重 Bug** | 4 小时 | 2 小时 | 当天 |
| **悬赏 PR** | 24 小时 | 12 小时 | 2-3 天 |
| **功能** | 2-3 天 | 1-2 天 | 3-5 天 |
| **文档** | 2-3 天 | 1-2 天 | 3-5 天 |
| **大型 PR** | 3-5 天 | 2-3 天 | 5-7 天 |

---

## ✅ 批准标准

PR 应在以下情况下批准：

1. **功能性**
   - ✅ 解决了所述问题
   - ✅ 现有功能没有退化
   - ✅ 边界情况已处理

2. **质量**
   - ✅ 遵循代码标准
   - ✅ 结构良好且可读
   - ✅ 测试覆盖率足够

3. **安全性**
   - ✅ 没有安全漏洞
   - ✅ 输入已验证
   - ✅ 密钥管理正确

4. **文档**
   - ✅ 需要的地方有代码注释
   - ✅ 文档已更新（如适用）

5. **流程**
   - ✅ 所有 CI 检查通过
   - ✅ 所有审核评论已处理
   - ✅ 已基于最新 dev 分支 rebase

---

## 🚫 拒绝标准

在以下情况下拒绝 PR：

**立即拒绝：**
- 🔴 引入安全漏洞
- 🔴 包含恶意代码
- 🔴 违反行为准则
- 🔴 包含抄袭代码
- 🔴 硬编码 API 密钥或密码

**请求更改：**
- 🟡 代码质量差（反馈被忽略后）
- 🟡 新功能没有测试
- 🟡 没有迁移路径的破坏性变更
- 🟡 与路线图不一致（未经事先讨论）
- 🟡 不完整（缺少关键部分）

**关闭并说明：**
- 🟠 重复功能
- 🟠 超出项目范围
- 🟠 已存在更好的替代方案
- 🟠 贡献者 >2 周无响应

---

## 🎯 特殊情况审核

### 悬赏 PR

需要额外注意：

- [ ] 所有验收标准都满足？
- [ ] 提供了演示视频/截图？
- [ ] 按悬赏 issue 中的规定工作？
- [ ] 私下讨论了付款信息？
- [ ] 优先审核（24小时周转）

### 破坏性变更

- [ ] 提供了迁移指南？
- [ ] 添加了弃用警告？
- [ ] 计划了版本升级？
- [ ] 考虑了向后兼容性？
- [ ] 为重大变更创建了 RFC？

### 安全 PR

- [ ] 由专注于安全的审核者验证？
- [ ] 没有公开披露漏洞？
- [ ] 如需要，协调披露？
- [ ] 准备了安全公告？
- [ ] 计划了补丁发布？

---

## 🔄 合并指南

### 何时合并

满足以下条件时合并：
- ✅ 至少 1 个维护者批准
- ✅ 所有 CI 检查通过
- ✅ 所有对话已解决
- ✅ 没有待处理的请求更改
- ✅ 已基于最新目标分支 rebase

### 合并策略

**Squash Merge**（大多数 PR 的默认策略）：
- 小型 bug 修复
- 单功能 PR
- 文档更新
- 保持 git 历史清洁

**Merge Commit**（复杂 PR）：
- 具有逻辑提交的多提交功能
- 保留提交历史
- 具有原子提交的大型重构

**Rebase and Merge**（很少使用）：
- 线性历史很重要时
- 提交已经结构良好

### 合并提交信息

格式：
```
<type>(<scope>): <PR 标题> (#123)

变更的简要描述。

- 关键变更 1
- 关键变更 2

Co-authored-by: 贡献者姓名 <email@example.com>
```

---

## 📊 要跟踪的审核指标

每月监控这些指标：

- 平均首次审核时间
- 平均合并时间
- PR 接受率
- 按类型分类的 PR 数量（bug/feature/docs）
- 按区域分类的 PR 数量（frontend/backend/exchange）
- 贡献者留存率

---

## 🙋 问题？

如果对 PR 不确定：

1. **询问其他维护者**在私人频道
2. **向贡献者请求更多上下文**
3. **标记为"on hold"**并添加到下次维护者同步
4. **如有疑问，保守一点** - 问比批准有风险的东西更好

---

## 🔗 相关资源

- [贡献指南](../../CONTRIBUTING.md)
- [行为准则](../../CODE_OF_CONDUCT.md)
- [安全政策](../../SECURITY.md)
- [项目路线图](../roadmap/README.zh-CN.md)

---

**记住：** 审核应该是**尊重的**、**建设性的**和**教育性的**。
我们在构建社区，而不仅仅是代码。🚀
