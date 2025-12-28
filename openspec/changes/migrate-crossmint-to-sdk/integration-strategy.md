# Crossmint Integration Strategy - Revised

## 问题分析

经过深入研究 Crossmint SDK，发现以下情况：

### Crossmint SDK 的限制

1. **Embedded Checkout** 需要：
   - `collectionLocator` (NFT collection) 或
   - `productLocator` (Product ID) 或
   - `tokenLocator` (Specific NFT token)

2. **我们的需求**：
   - 购买虚拟积分 (Credits)
   - 不是 NFT，不是预定义的 product

### 可行方案

#### 方案 1: 使用 Hosted Checkout（推荐）✅

**特点**:
- 弹出窗口方式
- 不需要 collection/product locator
- 支持自定义 line items
- 使用 `createOrder` API 在后端

**流程**:
```
Frontend → Backend API → Crossmint createOrder API → Return orderId
Frontend → Open CrossmintHostedCheckout → User pays
Crossmint → Webhook → Backend → Update credits
```

#### 方案 2: 在 Crossmint Console 创建 Products

**特点**:
- 在 Crossmint Dashboard 预先创建 3 个 products
- 每个 product 对应一个套餐
- 使用 productLocator

**缺点**:
- 需要手动配置
- 不够灵活
- 套餐变更需要在 Crossmint 手动更新

#### 方案 3: 完全自定义 API 集成

**特点**:
- 使用 Crossmint Payment API (非 checkout)
- 完全控制流程
- 复杂度高

## 推荐实施方案

### 使用 Backend + Hosted Checkout

#### 架构

```
PaymentModal
  ↓ (select package)
PaymentOrchestrator
  ↓ (call backend)
Backend API (/api/payments/create-checkout)
  ↓ (call Crossmint API with server key)
Crossmint createOrder API
  ↓ (return orderId + clientSecret)
Frontend
  ↓ (open hosted checkout)
CrossmintHostedCheckout (popup window)
  ↓ (user completes payment)
Crossmint Webhook → Backend
  ↓ (update credits)
Frontend (poll or webhook notification)
```

#### 优点

1. ✅ **符合 Crossmint 官方推荐**
2. ✅ **安全**: Server key 不暴露给前端
3. ✅ **灵活**: 可以传递任意 metadata
4. ✅ **简单**: SDK 处理所有 UI
5. ✅ **KISS 原则**: 最少的代码，最清晰的职责分离

#### 实施步骤

**Phase 1**: Backend API (需要后端支持)
```typescript
// POST /api/payments/create-crossmint-order
Request: {
  packageId: "starter" | "pro" | "vip"
}

Response: {
  orderId: string
  clientSecret: string
}
```

**Phase 2**: Frontend Integration
```typescript
// 1. Call backend to create order
const { orderId, clientSecret } = await createOrder(packageId)

// 2. Open hosted checkout
<CrossmintHostedCheckout
  orderId={orderId}
  clientSecret={clientSecret}
/>
```

## 决策

**选择方案 1: Hosted Checkout with Backend API**

原因：
- 最符合官方推荐
- 安全性最高
- 实现最简单
- 可扩展性最好

下一步：
1. 确认是否可以添加后端 API
2. 如果可以 → 实施 Backend API + Hosted Checkout
3. 如果不行 → 退回方案 2 (在 Console 创建 Products)
