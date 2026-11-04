# 🤝 为 NOFX 做贡献

**语言：** [English](../../../CONTRIBUTING.md) | [中文](CONTRIBUTING.md)

> **语言声明：** 本中文版本文档仅为方便海外华人社区阅读而提供，不代表本软件面向中国大陆、香港、澳门或台湾地区用户开放。如您位于上述地区，请勿使用本软件。

感谢您有兴趣为 NOFX 做贡献！本文档提供了为项目做贡献的指南和工作流程。

---

## 📑 目录

- [行为准则](#行为准则)
- [如何贡献](#如何贡献)
- [开发工作流程](#开发工作流程)
- [PR 提交指南](#pr-提交指南)
- [编码规范](#编码规范)
- [提交信息指南](#提交信息指南)
- [审核流程](#审核流程)
- [悬赏计划](#悬赏计划)

---

## 📜 行为准则

本项目遵守[行为准则](../../../CODE_OF_CONDUCT.md)。参与项目即表示您同意遵守此准则。

---

## 🎯 如何贡献

### 1. 报告 Bug 🐛

- 使用 [Bug 报告模板](../../../.github/ISSUE_TEMPLATE/bug_report.md)
- 检查 bug 是否已被报告
- 包含详细的重现步骤
- 提供环境信息（操作系统、Go 版本等）

### 2. 建议功能 ✨

- 使用[功能请求模板](../../../.github/ISSUE_TEMPLATE/feature_request.md)
- 解释使用场景和好处
- 检查是否与[项目路线图](../../roadmap/README.zh-CN.md)一致

### 3. 提交 Pull Request 🔧

提交 PR 前，请检查以下内容：

#### ✅ **接受的贡献**

**高优先级**（与路线图一致）：
- 🔒 安全增强（加密、认证、RBAC）
- 🧠 AI 模型集成（GPT-4、Claude、Gemini Pro）
- 🔗 交易所集成（OKX、Bybit、Lighter、EdgeX）
- 📊 交易数据 API（AI500、OI 分析、NetFlow）
- 🎨 UI/UX 改进（移动端响应式、图表）
- ⚡ 性能优化
- 🐛 Bug 修复
- 📝 文档改进

**中等优先级：**
- ✅ 测试覆盖率改进
- 🌐 国际化（新语言支持）
- 🔧 构建/部署工具
- 📈 监控和日志增强

#### ❌ **不接受**（未经事先讨论）

- 没有 RFC（征求意见稿）的重大架构变更
- 与项目路线图不一致的功能
- 没有迁移路径的破坏性变更
- 引入新依赖但没有充分理由的代码
- 没有可选标志的实验性功能

**⚠️ 重要：** 对于重大功能，请在开始工作**之前**先开 issue 讨论。

---

## 🛠️ 开发工作流程

### 1. Fork 和 Clone

```bash
# 在 GitHub 上 Fork 仓库
# 然后 clone 你的 fork
git clone https://github.com/YOUR_USERNAME/nofx.git
cd nofx

# 添加 upstream remote
git remote add upstream https://github.com/NoFxAiOS/nofx.git
```

### 2. 创建功能分支

```bash
# 更新你的本地 dev 分支
git checkout dev
git pull upstream dev

# 创建新分支
git checkout -b feature/your-feature-name
# 或
git checkout -b fix/your-bug-fix
```

**分支命名规范：**
- `feature/` - 新功能
- `fix/` - Bug 修复
- `docs/` - 文档更新
- `refactor/` - 代码重构
- `perf/` - 性能改进
- `test/` - 测试更新
- `chore/` - 构建/配置更改

### 3. 设置开发环境

```bash
# 安装 Go 依赖
go mod download

# 安装前端依赖
cd web
npm install
cd ..

# 安装 TA-Lib（必需）
# macOS:
brew install ta-lib

# Ubuntu/Debian:
sudo apt-get install libta-lib0-dev
```

### 4. 进行更改

- 遵循[编码规范](#编码规范)
- 为新功能编写测试
- 根据需要更新文档
- 保持提交专注和原子性

### 5. 测试你的更改

```bash
# 运行后端测试
go test ./...

# 构建后端
go build -o nofx

# 以开发模式运行前端
cd web
npm run dev

# 构建前端
npm run build
```

### 6. 提交你的更改

遵循[提交信息指南](#提交信息指南)：

```bash
git add .
git commit -m "feat: add support for OKX exchange integration"
```

### 7. 推送并创建 PR

```bash
# 推送到你的 fork
git push origin feature/your-feature-name

# 前往 GitHub 创建 Pull Request
# 使用 PR 模板并填写所有部分
```

---

## 📝 PR 提交指南

### 提交前检查

- [ ] 代码成功编译（`go build` 和 `npm run build`）
- [ ] 所有测试通过（`go test ./...`）
- [ ] 没有 linting 错误（`go fmt`、`go vet`）
- [ ] 文档已更新
- [ ] 提交遵循 conventional commits 格式
- [ ] 分支已基于最新的 `dev` rebase

### PR 标题格式

使用 [Conventional Commits](https://www.conventionalcommits.org/) 格式：

```
<type>(<scope>): <subject>

示例：
feat(exchange): add OKX exchange integration
fix(trader): resolve position tracking bug
docs(readme): update installation instructions
perf(ai): optimize prompt generation
refactor(core): extract common exchange interface
```

**类型：**
- `feat` - 新功能
- `fix` - Bug 修复
- `docs` - 文档
- `style` - 代码样式（格式化，无逻辑变更）
- `refactor` - 代码重构
- `perf` - 性能改进
- `test` - 测试更新
- `chore` - 构建/配置更改
- `ci` - CI/CD 更改
- `security` - 安全改进

### PR 描述

使用 [PR 模板](../../../.github/PULL_REQUEST_TEMPLATE.md)并确保：

1. **清晰描述**更改内容和原因
2. **变更类型**已标记
3. **相关 issue** 已链接
4. **测试步骤**已记录
5. UI 更改有**截图**
6. **所有复选框**已完成

### PR 大小

保持 PR 专注且大小合理：

- ✅ **小型 PR**（< 300 行）：理想，审核快速
- ⚠️ **中型 PR**（300-1000 行）：可接受，可能需要更长时间
- ❌ **大型 PR**（> 1000 行）：请拆分为更小的 PR

---

## 💻 编码规范

### Go 代码

```go
// ✅ 好：清晰的命名，正确的错误处理
func ConnectToExchange(apiKey, secret string) (*Exchange, error) {
    if apiKey == "" || secret == "" {
        return nil, fmt.Errorf("API credentials are required")
    }

    client, err := createClient(apiKey, secret)
    if err != nil {
        return nil, fmt.Errorf("failed to create client: %w", err)
    }

    return &Exchange{client: client}, nil
}

// ❌ 差：糟糕的命名，没有错误处理
func ce(a, s string) *Exchange {
    c := createClient(a, s)
    return &Exchange{client: c}
}
```

**最佳实践：**
- 使用有意义的变量名
- 显式处理所有错误
- 为复杂逻辑添加注释
- 遵循 Go 习惯用法和约定
- 提交前运行 `go fmt`
- 使用 `go vet` 和 `golangci-lint`

### TypeScript/React 代码

```typescript
// ✅ 好：类型安全，清晰的命名
interface TraderConfig {
  id: string;
  exchange: 'binance' | 'hyperliquid' | 'aster';
  aiModel: string;
  enabled: boolean;
}

const TraderCard: React.FC<{ trader: TraderConfig }> = ({ trader }) => {
  const [isRunning, setIsRunning] = useState(false);

  const handleStart = async () => {
    try {
      await startTrader(trader.id);
      setIsRunning(true);
    } catch (error) {
      console.error('Failed to start trader:', error);
    }
  };

  return <div>...</div>;
};

// ❌ 差：没有类型，不清晰的命名
const TC = (props) => {
  const [r, setR] = useState(false);
  const h = () => { startTrader(props.t.id); setR(true); };
  return <div>...</div>;
};
```

**最佳实践：**
- 使用 TypeScript 严格模式
- 为所有数据结构定义接口
- 避免使用 `any` 类型
- 使用带 hooks 的函数式组件
- 遵循 React 最佳实践
- 提交前运行 `npm run lint`

### 文件结构

```
NOFX/
├── cmd/               # 主应用程序
├── internal/          # 私有代码
│   ├── exchange/      # 交易所适配器
│   ├── trader/        # 交易逻辑
│   ├── ai/           # AI 集成
│   └── api/          # API 处理器
├── pkg/              # 公共库
├── web/              # 前端
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   ├── hooks/
│   │   └── utils/
│   └── public/
└── docs/             # 文档
```

---

## 📋 提交信息指南

### 格式

```
<type>(<scope>): <subject>

<body>

<footer>
```

### 示例

```
feat(exchange): add OKX futures API integration

- Implement order placement and cancellation
- Add balance and position retrieval
- Support leverage configuration

Closes #123
```

```
fix(trader): prevent duplicate position opening

The trader was opening multiple positions in the same direction
for the same symbol. Added check to prevent this behavior.

Fixes #456
```

```
docs: update Docker deployment guide

- Add troubleshooting section
- Update environment variables
- Add examples for common scenarios
```

### 规则

- 使用现在时（"add" 而非 "added"）
- 使用祈使语气（"move" 而非 "moves"）
- 第一行 ≤ 72 字符
- 引用 issue 和 PR
- 解释"是什么"和"为什么"，而非"如何做"

---

## 🔍 审核流程

### 时间线

- **初次审核：** 2-3 个工作日内
- **后续审核：** 1-2 个工作日内
- **悬赏 PR：** 1 个工作日内优先审核

### 审核标准

审核者将检查：

1. **功能性**
   - 是否按预期工作？
   - 边界情况是否处理？
   - 现有功能没有退化？

2. **代码质量**
   - 遵循编码规范？
   - 结构良好且可读？
   - 正确的错误处理？

3. **测试**
   - 测试覆盖率足够？
   - CI 中测试通过？
   - 手动测试已记录？

4. **文档**
   - 需要的地方有代码注释？
   - README/文档已更新？
   - API 变更已记录？

5. **安全性**
   - 没有硬编码的密钥？
   - 输入验证？
   - 没有已知漏洞？

### 回应反馈

- 处理所有审核评论
- 不清楚时提问
- 标记对话为已解决
- 更改后重新请求审核

### 批准和合并

- 需要维护者 **1 个批准**
- 所有 CI 检查必须通过
- 没有未解决的对话
- 维护者将合并（小型 PR 使用 squash merge，功能使用 merge commit）

---

## 💰 悬赏计划

### 工作方式

1. 查看[悬赏 issue](https://github.com/NoFxAiOS/nofx/labels/bounty)
2. 评论认领（先到先得）
3. 在截止日期前完成工作
4. 提交 PR 并填写悬赏认领部分
5. 合并后获得报酬

### 指南

- 阅读[悬赏指南](../../community/bounty-guide.md)
- 满足所有验收标准
- 包含演示视频/截图
- 遵循所有贡献指南
- 私下讨论付款详情

---

## ❓ 问题？


- **技术问题：** 开启[讨论](https://github.com/NoFxAiOS/nofx/discussions)
- **安全问题：** 查看[安全政策](../../../SECURITY.md)
- **Bug 报告：** 使用 [Bug 报告模板](../../../.github/ISSUE_TEMPLATE/bug_report.md)

---

## 📚 其他资源

- [项目路线图](../../roadmap/README.zh-CN.md)
- [架构文档](../../architecture/README.zh-CN.md)
- [API 文档](../../api/README.md)
- [部署指南](../../getting-started/docker-deploy.zh-CN.md)

---

## 🙏 感谢你！

你的贡献让 NOFX 变得更好。我们感谢你的时间和努力！

**编码愉快！🚀**
