# Crossmint 支付失败问题分析与解决方案

**日期**: 2025-12-28
**状态**: 🔴 API 端点已弃用
**环境**: Staging (测试环境)

---

## 📊 问题总结

### 用户报告的错误
```
支付失败
支付服务暂时不可用: Failed to initialize Crossmint checkout: Failed to fetch
```

### 根本原因分析

经过深入测试，确认了以下问题：

#### 1. ✅ API Key 配置正确
- **状态**: 已配置
- **格式**: `ck_staging_...` (正确的 staging 环境 key)
- **长度**: 227 字符 (正常)
- **位置**: `.env.local` 文件中的 `VITE_CROSSMINT_CLIENT_API_KEY`

#### 2. ❌ API 端点已弃用
- **当前使用**: `https://api.crossmint.com/2022-06-09/embedded-checkouts`
- **测试结果**: HTTP 404 - DEPLOYMENT_NOT_FOUND
- **错误详情**:
  ```
  x-vercel-error: DEPLOYMENT_NOT_FOUND
  The deployment could not be found on Vercel.
  ```

**结论**: API 版本 `2022-06-09` (3年前) 已经被 Crossmint 弃用或移除。

---

## 🔧 解决方案

### 方案 A: 使用官方 SDK（强烈推荐）⭐

Crossmint 现在推荐使用官方 SDK 而不是直接调用 API。

#### 步骤 1: 安装 SDK

\`\`\`bash
cd web
npm install @crossmint/client-sdk-react-ui
\`\`\`

#### 步骤 2: 更新 CrossmintService

当前实现使用 `fetch` 直接调用 API，需要改为使用 SDK。

**需要修改的文件**:
- `src/features/payment/services/CrossmintService.ts`
- `src/features/payment/components/PaymentModal.tsx`

#### 步骤 3: 实现 SDK 集成

参考官方文档: https://docs.crossmint.com/payments/embedded/quickstart

**注意**: 这需要重构现有代码，预计工作量 2-4 小时。

---

### 方案 B: 联系 Crossmint 支持获取最新 API 端点

#### 行动项

1. **联系 Crossmint 支持**
   - Email: support@crossmint.com
   - Discord: https://discord.gg/crossmint
   - 问题: "请提供 embedded-checkouts API 的最新端点版本"

2. **可能的新端点**（需要验证）
   - `https://api.crossmint.com/api/v1-alpha1/embedded-checkouts`
   - `https://api.crossmint.com/v1/embedded-checkouts`
   - 或使用 SDK（推荐）

3. **测试新端点**

   使用提供的测试脚本:
   \`\`\`bash
   cd web
   ./test-api.sh
   \`\`\`

---

### 方案 C: 临时禁用支付功能（不推荐）

如果需要快速上线其他功能，可以暂时禁用支付：

\`\`\`typescript
// src/features/payment/services/CrossmintService.ts
async initializeCheckout(config: CheckoutConfig): Promise<string> {
  throw new Error("支付功能暂时维护中，请稍后再试")
}
\`\`\`

---

## 📝 当前配置验证

### ✅ 已正确配置

1. **环境变量**: `VITE_CROSSMINT_CLIENT_API_KEY` 已配置
2. **API Key 格式**: `ck_staging_...` (正确)
3. **API Key 长度**: 227 字符
4. **套餐配置**: 与需求一致
   - 初级套餐: 10 USDT → 500 积分
   - 专业套餐: 50 USDT → 3,300 积分 (3000 + 300 bonus)
   - VIP 套餐: 100 USDT → 9,600 积分 (8000 + 1600 bonus)

### ❌ 需要修复

1. **API 端点**: 当前使用的 `2022-06-09` 版本已弃用
2. **实现方式**: 应该使用 SDK 而不是直接 fetch

---

## 🚀 推荐行动计划

### 立即执行 (今天)

1. ✅ **已完成**: 配置 API Key 到 `.env.local`
2. ⏳ **待执行**: 联系 Crossmint 支持
   - 询问最新的 API 端点
   - 或确认必须使用 SDK

### 短期 (1-2天)

3. 根据 Crossmint 回复选择方案:
   - **如果提供新端点**: 更新代码中的 API URL
   - **如果必须用 SDK**: 开始重构使用 SDK

### 中期 (本周)

4. 完整测试支付流程:
   - 创建 checkout session
   - 完成支付
   - 验证积分到账
   - 测试错误处理

---

## 📞 Crossmint 联系方式

- **Console**: https://staging.crossmint.com/console
- **文档**: https://docs.crossmint.com
- **支持**: support@crossmint.com
- **Discord**: https://discord.gg/crossmint

### 建议的支持请求模板

\`\`\`
Subject: Embedded Checkout API Endpoint Question

Hi Crossmint Team,

I'm integrating the embedded checkout feature and currently using:
https://api.crossmint.com/2022-06-09/embedded-checkouts

However, this endpoint returns 404. Could you please provide:
1. The current/correct API endpoint for embedded-checkouts
2. Whether we should use the SDK instead of direct API calls
3. Any migration guide from the 2022-06-09 version

Environment: Staging
API Key Type: Client-side (ck_staging_...)

Thank you!
\`\`\`

---

## 🔍 测试脚本

已创建测试脚本来验证 API 连接:

### 使用方法

\`\`\`bash
cd web

# 测试 API 连接
./test-api.sh

# 或使用 Node.js 测试
node test-crossmint-api.js
\`\`\`

### 预期结果

- ✅ 成功: 返回 session ID
- ❌ 当前: HTTP 404 - DEPLOYMENT_NOT_FOUND

---

## 📚 相关资源

- [Crossmint Embedded Checkout Quickstart](https://docs.crossmint.com/payments/embedded/quickstart)
- [Crossmint SDK GitHub](https://github.com/Crossmint/crossmint-sdk)
- [Embedded Checkout Demo](https://github.com/Crossmint/embedded-checkout-quickstart)

---

## 🎯 下一步

**最紧急**: 联系 Crossmint 支持获取正确的 API 端点或确认 SDK 使用方式

**测试环境准备**:
1. API Key ✅ 已配置
2. 套餐配置 ✅ 已就绪
3. 前端集成 ✅ 已完成
4. API 端点 ❌ 需要更新

一旦获得正确的 API 端点或完成 SDK 集成，支付功能即可正常工作。

---

**报告生成时间**: 2025-12-28 13:21 CST
