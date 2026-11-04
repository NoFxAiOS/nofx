# Security Policy / 安全政策

**Languages:** [English](#english) | [中文](#中文)

---

# English

## 🛡️ Security Overview

NOFX is an AI-powered trading system that handles real funds and API credentials. We take security seriously and appreciate the security community's efforts to responsibly disclose vulnerabilities.

**Critical Areas:**
- 🔑 API key storage and handling
- 💰 Trading execution and fund management
- 🔐 Authentication and authorization
- 🗄️ Database security (SQLite)
- 🌐 Web interface and API endpoints

---

## 📋 Supported Versions

We provide security updates for the following versions:

| Version | Supported          | Notes                |
| ------- | ------------------ | -------------------- |
| 3.x     | ✅ Fully supported | Current stable release |
| 2.x     | ⚠️ Limited support | Security fixes only |
| < 2.0   | ❌ Not supported   | Please upgrade       |

**Recommendation:** Always use the latest stable release (v3.x) for best security.

---

## 🔒 Reporting a Vulnerability

### ⚠️ Please DO NOT Publicly Disclose

If you discover a security vulnerability in NOFX, please **DO NOT**:
- ❌ Open a public GitHub Issue
- ❌ Discuss it on social media (Twitter, Reddit, etc.)
- ❌ Share it in public chat groups
- ❌ Post it on security forums before we've had time to fix it

Public disclosure before a fix is available puts all users at risk.

### ✅ Responsible Disclosure Process

**Step 1: Report Privately**

Contact core team directly:
- **Tinkle:** [@Web3Tinkle on Twitter](https://x.com/Web3Tinkle) (DM)

**Alternative:** Encrypted communication via [Keybase](https://keybase.io/) (if available)

**Step 2: Include These Details**

```markdown
Subject: [SECURITY] Brief description of vulnerability

## Vulnerability Description
Clear explanation of the security issue

## Affected Components
- Which parts of the system are affected?
- Which versions are vulnerable?

## Reproduction Steps
1. Step-by-step instructions
2. Sample code or commands (if applicable)
3. Expected vs actual behavior

## Potential Impact
- Can funds be stolen?
- Can API keys be leaked?
- Can accounts be compromised?
- Rate the severity: Critical / High / Medium / Low

## Suggested Fix (Optional)
If you have ideas for fixing it, please share!

## Your Information
- Name (or pseudonym)
- Contact info for follow-up
- If you want public credit (yes/no)
```

**Step 3: Wait for Our Response**

We will:
- ✅ Acknowledge receipt within **24 hours**
- ✅ Provide initial assessment within **72 hours**
- ✅ Keep you updated on fix progress
- ✅ Notify you before public disclosure

---

## ⏱️ Response Timeline

| Stage | Timeline | Action |
|-------|----------|--------|
| **Acknowledgment** | 24 hours | Confirm we received your report |
| **Initial Assessment** | 72 hours | Verify vulnerability, rate severity |
| **Fix Development** | 7-30 days | Depends on complexity and severity |
| **Testing** | 3-7 days | Verify fix doesn't break functionality |
| **Public Disclosure** | After fix deployed | Publish security advisory |

**Critical vulnerabilities** (fund theft, credential leaks) are prioritized and may be fixed within 48 hours.

---

## 💰 Security Bounty Program (Optional)

We offer rewards for valid security vulnerabilities:

| Severity | Criteria | Reward |
|----------|----------|--------|
| **🔴 Critical** | Fund theft, API key extraction, RCE | **$500-1000 USD** |
| **🟠 High** | Authentication bypass, unauthorized trading | **$200-500 USD** |
| **🟡 Medium** | Information disclosure, XSS, CSRF | **$100-200 USD** |
| **🟢 Low** | Security improvements, minor issues | **$50-100 USD or Recognition** |

**Note:** Bounty amounts are at maintainers' discretion based on:
- Severity and impact
- Quality of report
- Ease of exploitation
- Number of affected users

**Out of Scope (No Bounty):**
- Issues in third-party libraries (report to them directly)
- Social engineering attacks
- DoS/DDoS attacks
- Issues requiring physical access
- Previously known/reported vulnerabilities

---

## 🔐 Security Best Practices (For Users)

To keep your NOFX deployment secure:

### 1. API Key Management
```bash
# ✅ DO: Use environment variables
export BINANCE_API_KEY="your_key"
export BINANCE_SECRET_KEY="your_secret"

# ❌ DON'T: Hardcode in source files
api_key = "abc123..."  # NEVER DO THIS
```

### 2. Database Security
```bash
# ✅ Set proper permissions
chmod 600 nofx.db
chmod 600 config.json

# ❌ DON'T: Leave files world-readable
chmod 777 nofx.db  # NEVER DO THIS
```

### 3. Network Security
```bash
# ✅ Use firewall to restrict API access
# Only allow localhost to access API server
iptables -A INPUT -p tcp --dport 8080 -s 127.0.0.1 -j ACCEPT
iptables -A INPUT -p tcp --dport 8080 -j DROP

# ❌ DON'T: Expose API to public internet without authentication
```

### 4. Use Subaccounts
- Create dedicated Binance subaccount for trading
- Limit maximum balance
- Restrict withdrawal permissions
- Use IP whitelist

### 5. Test on Testnet First
- Hyperliquid: Use testnet mode
- Binance: Use testnet API (https://testnet.binancefuture.com)
- Never test with real funds initially

### 6. Regular Updates
```bash
# Check for updates regularly
git pull origin main
go build -o nofx

# Subscribe to security advisories
# Watch GitHub releases: https://github.com/NoFxAiOS/nofx/releases
```

---

## 🚨 Security Advisories

Past security advisories will be published here:

### 2025-XX-XX: [Title]
- **Severity:** [Critical/High/Medium/Low]
- **Affected Versions:** [x.x.x - x.x.x]
- **Fixed in:** [x.x.x]
- **Description:** [Brief description]
- **Mitigation:** [How to protect yourself]

*No security advisories have been published yet.*

---

## 🙏 Security Researchers Hall of Fame

We thank the following security researchers for responsibly disclosing vulnerabilities:

*No reports have been submitted yet. Be the first!*

---

## 📚 Additional Resources

**Security Documentation:**
- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [CWE Top 25](https://cwe.mitre.org/top25/)
- [Binance API Security Best Practices](https://www.binance.com/en/support/faq/360002502072)

**Audit Reports:**
- No third-party audits completed yet
- Self-audit checklist: [TODO: Add link]

---

## 📞 Contact

**For security issues ONLY:**
- 🐦 **Twitter DM:** [@Web3Tinkle](https://x.com/Web3Tinkle)

**For general questions:**
- See [CONTRIBUTING.md](CONTRIBUTING.md)


---

**Thank you for helping keep NOFX secure!** 🔒

---

# 中文

## 🛡️ 安全概述

NOFX 是一个处理真实资金和 API 凭证的 AI 交易系统。我们非常重视安全，并感谢安全社区负责任地披露漏洞的努力。

**关键领域：**
- 🔑 API 密钥存储和处理
- 💰 交易执行和资金管理
- 🔐 身份验证和授权
- 🗄️ 数据库安全（SQLite）
- 🌐 Web 界面和 API 端点

---

## 📋 支持的版本

我们为以下版本提供安全更新：

| 版本 | 支持状态 | 说明 |
| ------- | ------------------ | -------------------- |
| 3.x     | ✅ 完全支持 | 当前稳定版本 |
| 2.x     | ⚠️ 有限支持 | 仅安全修复 |
| < 2.0   | ❌ 不支持 | 请升级 |

**建议：** 始终使用最新的稳定版本（v3.x）以获得最佳安全性。

---

## 🔒 报告漏洞

### ⚠️ 请勿公开披露

如果您在 NOFX 中发现安全漏洞，请**不要**：
- ❌ 公开创建 GitHub Issue
- ❌ 在社交媒体上讨论（Twitter、Reddit 等）
- ❌ 在公开聊天群组中分享
- ❌ 在我们有时间修复之前发布到安全论坛

在修复可用之前公开披露会使所有用户面临风险。

### ✅ 负责任的披露流程

**步骤 1：私下报告**

直接联系核心团队：
- **Tinkle:** [@Web3Tinkle on Twitter](https://x.com/Web3Tinkle)（私信）

**替代方案：** 通过 [Keybase](https://keybase.io/) 加密通信（如果可用）

**步骤 2：包含这些详细信息**

```markdown
主题：[SECURITY] 漏洞简要描述

## 漏洞描述
清楚解释安全问题

## 受影响的组件
- 系统的哪些部分受到影响？
- 哪些版本存在漏洞？

## 复现步骤
1. 逐步说明
2. 示例代码或命令（如果适用）
3. 预期行为 vs 实际行为

## 潜在影响
- 资金是否可能被盗？
- API 密钥是否可能泄露？
- 账户是否可能被入侵？
- 严重程度评级：严重 / 高 / 中 / 低

## 建议修复（可选）
如果您有修复的想法，请分享！

## 您的信息
- 姓名（或化名）
- 后续联系信息
- 是否希望公开致谢（是/否）
```

**步骤 3：等待我们的回复**

我们将：
- ✅ 在 **24 小时**内确认收到
- ✅ 在 **72 小时**内提供初步评估
- ✅ 告知您修复进展
- ✅ 在公开披露前通知您

---

## ⏱️ 响应时间表

| 阶段 | 时间线 | 行动 |
|-------|----------|--------|
| **确认** | 24 小时 | 确认我们收到了您的报告 |
| **初步评估** | 72 小时 | 验证漏洞，评估严重程度 |
| **修复开发** | 7-30 天 | 取决于复杂性和严重程度 |
| **测试** | 3-7 天 | 验证修复不会破坏功能 |
| **公开披露** | 修复部署后 | 发布安全公告 |

**严重漏洞**（资金盗窃、凭证泄露）会优先处理，可能在 48 小时内修复。

---

## 💰 安全奖励计划（可选）

我们为有效的安全漏洞提供奖励：

| 严重程度 | 标准 | 奖励 |
|----------|----------|--------|
| **🔴 严重** | 资金盗窃、API 密钥提取、RCE | **$500-1000 USD** |
| **🟠 高** | 认证绕过、未授权交易 | **$200-500 USD** |
| **🟡 中** | 信息泄露、XSS、CSRF | **$100-200 USD** |
| **🟢 低** | 安全改进、小问题 | **$50-100 USD 或致谢** |

**注意：** 奖励金额由维护者根据以下因素酌情决定：
- 严重性和影响
- 报告质量
- 利用难易度
- 受影响用户数量

**不在范围内（无奖励）：**
- 第三方库的问题（直接向他们报告）
- 社会工程攻击
- DoS/DDoS 攻击
- 需要物理访问的问题
- 已知/已报告的漏洞

---

## 🔐 安全最佳实践（用户指南）

保护您的 NOFX 部署安全：

### 1. API 密钥管理
```bash
# ✅ 正确：使用环境变量
export BINANCE_API_KEY="your_key"
export BINANCE_SECRET_KEY="your_secret"

# ❌ 错误：在源文件中硬编码
api_key = "abc123..."  # 永远不要这样做
```

### 2. 数据库安全
```bash
# ✅ 设置适当的权限
chmod 600 nofx.db
chmod 600 config.json

# ❌ 不要：让文件全局可读
chmod 777 nofx.db  # 永远不要这样做
```

### 3. 网络安全
```bash
# ✅ 使用防火墙限制 API 访问
# 仅允许本地访问 API 服务器
iptables -A INPUT -p tcp --dport 8080 -s 127.0.0.1 -j ACCEPT
iptables -A INPUT -p tcp --dport 8080 -j DROP

# ❌ 不要：在没有身份验证的情况下将 API 暴露到公共互联网
```

### 4. 使用子账户
- 为交易创建专用的 Binance 子账户
- 限制最大余额
- 限制提现权限
- 使用 IP 白名单

### 5. 先在测试网上测试
- Hyperliquid：使用测试网模式
- Binance：使用测试网 API (https://testnet.binancefuture.com)
- 最初永远不要用真实资金测试

### 6. 定期更新
```bash
# 定期检查更新
git pull origin main
go build -o nofx

# 订阅安全公告
# 关注 GitHub 发布：https://github.com/NoFxAiOS/nofx/releases
```

---

## 🚨 安全公告

过去的安全公告将在此发布：

### 2025-XX-XX: [标题]
- **严重程度：** [严重/高/中/低]
- **受影响版本：** [x.x.x - x.x.x]
- **已修复版本：** [x.x.x]
- **描述：** [简要描述]
- **缓解措施：** [如何保护自己]

*尚未发布任何安全公告。*

---

## 🙏 安全研究员名人堂

我们感谢以下安全研究员负责任地披露漏洞：

*尚未收到任何报告。成为第一个！*

---

## 📞 联系方式

**仅限安全问题：**
- 🐦 **Twitter 私信：** [@Web3Tinkle](https://x.com/Web3Tinkle)

**一般问题：**


---

**感谢您帮助保持 NOFX 的安全！** 🔒
