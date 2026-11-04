# 📊 项目管理指南

**语言：** [English](PROJECT_MANAGEMENT.md) | [中文](PROJECT_MANAGEMENT.zh-CN.md)

本指南解释了我们如何管理 NOFX 项目、跟踪进度和优先级排序。

---

## 🎯 项目结构

### GitHub Projects

我们使用 **GitHub Projects (Beta)** 和以下看板：

#### 1. **NOFX 开发看板**

**列：**
```
Backlog → Triaged → In Progress → In Review → Done
```

**视图：**
- 📋 **所有 Issue** - 所有工作项的看板视图
- 🏃 **Sprint** - 当前 Sprint 项（2 周 Sprint）
- 🗺️ **路线图** - 按路线图阶段的时间轴视图
- 🏷️ **按区域** - 按区域标签分组
- 🔥 **优先级** - 按优先级排序（critical/high/medium/low）
- 👥 **按分配人** - 按分配的维护者分组

#### 2. **悬赏计划看板**

**列：**
```
Available → Claimed → In Progress → Under Review → Paid
```

---

## 📅 Sprint 计划（双周）

### Sprint 时间表

**Sprint 周期：** 2 周
**Sprint 计划：** 每隔一周的星期一
**Sprint 回顾：** 每隔一周的星期五

### 计划流程

**星期一 - Sprint 计划（1小时）：**

1. **回顾上一个 Sprint**（15分钟）
   - 完成了什么？
   - 什么没有完成？为什么？
   - 指标回顾

2. **优先级排序 Backlog**（20分钟）
   - 审查新的 issue/PR
   - 基于路线图更新优先级
   - 分配标签

3. **计划下一个 Sprint**（25分钟）
   - 选择下一个 Sprint 的项目
   - 分配给维护者
   - 设定清晰的验收标准
   - 估算工作量（S/M/L）

**星期五 - Sprint 回顾（30分钟）：**

1. **演示已完成的工作**（15分钟）
   - 展示已合并的 PR
   - 演示新功能

2. **复盘**（15分钟）
   - 什么做得好？
   - 什么可以改进？
   - 下一个 Sprint 的行动项

---

## 🏷️ Issue 分类流程

### 每日分类（周一至周五，15分钟）

审查新的 issue 和 PR：

1. **验证完整性**
   - 模板是否正确填写？
   - 重现步骤清晰吗（对于 bug）？
   - 使用场景解释清楚吗（对于功能）？

2. **应用标签**
   ```yaml
   优先级：
     - priority: critical  # 安全问题、数据丢失、生产环境宕机
     - priority: high      # 主要 bug、高价值功能
     - priority: medium    # 常规 bug、标准功能
     - priority: low       # 可选功能、次要改进

   类型：
     - type: bug
     - type: feature
     - type: enhancement
     - type: documentation
     - type: security

   区域：
     - area: exchange
     - area: ai
     - area: frontend
     - area: backend
     - area: security
     - area: ui/ux

   路线图：
     - roadmap: phase-1  # 核心基础设施
     - roadmap: phase-2  # 测试与稳定性
     - roadmap: phase-3  # 通用市场
     ```

3. **分配或标记讨论**
   - 可以立即处理？分配给维护者
   - 需要讨论？标记在下次计划会议
   - 需要更多信息？从作者处请求

4. **必要时关闭**
   - 重复？关闭并链接到原始 issue
   - 无效？关闭并说明原因
   - 超出范围？礼貌关闭并说明理由

---

## 🎯 优先级决策矩阵

使用此矩阵决定优先级：

| 影响/紧急程度 | 高紧急 | 中等紧急 | 低紧急 |
|------------------|--------------|----------------|-------------|
| **高影响** | 🔴 Critical | 🔴 Critical | 🟡 High |
| **中等影响** | 🔴 Critical | 🟡 High | 🟢 Medium |
| **低影响** | 🟡 High | 🟢 Medium | ⚪ Low |

**影响：**
- 高：影响核心功能、安全性或许多用户
- 中：影响特定功能或中等数量用户
- 低：可选功能、次要改进

**紧急程度：**
- 高：需要立即关注
- 中：应该尽快处理
- 低：可以等待自然包含

---

## 📊 路线图对齐

所有工作应与我们的[路线图](../roadmap/README.zh-CN.md)对齐：

### Phase 1：核心基础设施（当前重点）

**必须接受：**
- 安全增强
- AI 模型集成
- 交易所集成（OKX、Bybit、Lighter、EdgeX）
- 项目结构重构
- UI/UX 改进

**可以接受：**
- 相关 bug 修复
- 文档改进
- 性能优化

**应该推迟：**
- 通用市场扩展（股票、期货）
- 高级 AI 功能（RL、多智能体）
- 企业功能

### Phase 2-5：未来工作

使用适当的 `roadmap: phase-X` 标签标记并添加到 backlog。

---

## 🎫 Issue 模板

我们有这些 issue 模板：

### 1. Bug 报告
- 用于 bug 和错误
- 必须包含重现步骤
- 标签：`type: bug`

### 2. 功能请求
- 用于新功能
- 必须包含使用场景和好处
- 标签：`type: feature`

### 3. 悬赏认领
- 认领悬赏时使用
- 必须引用悬赏 issue
- 标签：`bounty: claimed`

### 4. 安全漏洞
- 用于安全问题（私密）
- 遵循负责任的披露
- 标签：`type: security`

**缺少模板？**
- 使用空白 issue
- 维护者将转换为适当的模板

---

## 📈 我们跟踪的指标

### 每周指标

- **PR 指标：**
  - 打开的 PR 数量
  - 合并的 PR 数量
  - 平均首次审核时间
  - 平均合并时间

- **Issue 指标：**
  - 打开的 issue 数量
  - 关闭的 issue 数量
  - Issue backlog 大小
  - 按优先级/类型/区域分类的 issue

- **社区指标：**
  - 新贡献者
  - 活跃贡献者
  - 社区参与度（评论、反应）

### 每月指标

- **路线图进度：**
  - 每个阶段的完成百分比
  - 已完成 vs 计划项目
  - 阻塞因素和风险

- **代码质量：**
  - 测试覆盖率
  - 每个 PR 的代码审核评论数
  - Bug 修复 vs 功能比率

- **悬赏计划：**
  - 创建的悬赏
  - 认领的悬赏
  - 支付的悬赏
  - 平均完成时间

---

## 🤖 自动化

我们使用 GitHub Actions 进行自动化：

### PR 自动化

- **基于文件变更的自动标签**
- **PR 大小标签**（small/medium/large）
- **CI 检查**（测试、linting、构建）
- **安全扫描**（Trivy、Gitleaks）
- **Conventional commit 验证**

### Issue 自动化

- **过期 issue 检测**（30天不活动后关闭）
- **使用 "bounty" 关键字时自动悬赏标签**
- **使用 issue 相似性的重复检测**

### 发布自动化

- **从 conventional commits 生成 Changelog**
- **基于 commit 类型的版本升级**
- **自动生成发布说明**
- **部署到 staging/production**

---

## 🔄 定期任务

### 每日
- ✅ 分类新的 issue/PR
- ✅ 审查紧急 PR
- ✅ 回应社区问题

### 每周
- ✅ Sprint 计划（星期一）
- ✅ Sprint 回顾（星期五）
- ✅ 审查指标仪表板
- ✅ 更新项目看板

### 每月
- ✅ 路线图进度回顾
- ✅ 社区更新帖子
- ✅ 悬赏计划回顾
- ✅ 依赖更新
- ✅ 安全审计

### 每季度
- ✅ 路线图更新
- ✅ 主要版本规划
- ✅ 贡献者表彰
- ✅ 文档审计

---

## 📞 沟通渠道

### 内部（维护者）

- **GitHub Discussions：** 架构决策、RFC
- **私人频道：** 敏感讨论、悬赏支付
- **每周同步：** Sprint 计划和回顾

### 外部（社区）

- **Telegram：** [@nofx_dev_community](https://t.me/nofx_dev_community)
- **GitHub Issues：** Bug 报告、功能请求
- **GitHub Discussions：** 一般问题、想法
- **Twitter：** [@nofx_official](https://x.com/nofx_official) - 公告

---

## 🎓 新维护者入职

### 新维护者检查清单

- [ ] 添加到 GitHub 组织
- [ ] 授予仓库写入权限
- [ ] 添加到私人维护者频道
- [ ] 介绍给团队
- [ ] 阅读 `/docs/maintainers/` 中的所有文档
- [ ] 跟随有经验的维护者 1 个 Sprint
- [ ] 首次单独 PR 审核（有备份审核者）
- [ ] 首次单独 issue 分类
- [ ] 首次参与 Sprint 计划

### 期望

**时间投入：**
- 每周约 5-10 小时
- 参与 Sprint 计划/回顾
- 在 SLA 内回应分配的 issue/PR

**职责：**
- 代码审核
- Issue 分类
- 社区支持
- 文档维护

---

## 🏆 贡献者表彰

### 每月表彰

**在社区更新中聚焦：**
- 顶级贡献者
- 本月最佳 PR
- 最有帮助的社区成员

### 每季度表彰

**贡献者等级系统：**
- 🥇 **核心贡献者** - 20+ 个已合并 PR
- 🥈 **活跃贡献者** - 10+ 个已合并 PR
- 🥉 **贡献者** - 5+ 个已合并 PR
- ⭐ **首次贡献者** - 1+ 个已合并 PR

**福利：**
- 在 README 中表彰
- 邀请加入私人 Discord
- 早期访问功能
- 周边商品（核心贡献者）

---

## 📚 资源

### 内部文档
- [PR 审核指南](PR_REVIEW_GUIDE.zh-CN.md)
- [安全政策](../../SECURITY.md)
- [行为准则](../../CODE_OF_CONDUCT.md)

### 外部资源
- [GitHub 项目管理](https://docs.github.com/en/issues/planning-and-tracking-with-projects)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [语义化版本](https://semver.org/)

---

## 🤔 问题？

在维护者频道联系我们或开启讨论。

**让我们一起构建令人惊叹的产品！🚀**
