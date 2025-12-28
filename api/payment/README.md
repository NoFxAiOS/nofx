# Crossmint Payment Integration

## 📋 Overview

This package implements Crossmint payment integration for the NOFX trading platform, allowing users to purchase credit packages using cryptocurrency.

## 🏗️ Architecture

### Design Principles

1. **KISS (Keep It Simple, Stupid)**: Clean, straightforward code with minimal complexity
2. **High Cohesion, Low Coupling**: Each layer has clear responsibilities
3. **Transaction Safety**: ACID guarantees with idempotency protection
4. **100% Test Coverage**: Comprehensive unit and integration tests

### Layer Structure

```
┌─────────────────────────────────────────────┐
│          HTTP Layer (handler.go)            │
│  • Request validation                       │
│  • Authentication/Authorization             │
│  • Response formatting                      │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│       Service Layer (service.go)            │
│  • Business logic                          │
│  • Crossmint API integration               │
│  • Webhook processing                      │
│  • Signature verification                  │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│      Data Layer (config/payment.go)         │
│  • Database operations                     │
│  • Transaction management                  │
│  • Data validation                        │
└─────────────────────────────────────────────┘
```

## 📡 API Endpoints

### 1. Create Payment Order

Creates a Crossmint checkout order for credit purchase.

**Endpoint**: `POST /api/payments/crossmint/create-order`

**Authentication**: Required (Bearer Token)

**Rate Limit**: 10 requests/minute per user

**Request**:
```json
{
  "packageId": "pkg_starter"
}
```

**Response** (Success):
```json
{
  "success": true,
  "orderId": "order_abc123...",
  "clientSecret": "secret_xyz789...",
  "amount": 10.00,
  "currency": "USDT",
  "credits": 600,
  "expiresAt": ""
}
```

**Response** (Error):
```json
{
  "success": false,
  "error": "Invalid package ID",
  "code": "INVALID_PACKAGE",
  "details": "..."
}
```

### 2. Get Payment Order

Retrieves a specific payment order.

**Endpoint**: `GET /api/payments/orders/:id`

**Authentication**: Required

**Response**:
```json
{
  "success": true,
  "order": {
    "id": "order_123",
    "userId": "user_xyz",
    "packageId": "pkg_starter",
    "amount": 10.00,
    "currency": "USDT",
    "credits": 600,
    "status": "completed",
    "createdAt": "2025-12-28T10:00:00Z",
    "completedAt": "2025-12-28T10:05:00Z"
  }
}
```

### 3. Get User Payment Orders

Lists all payment orders for the authenticated user.

**Endpoint**: `GET /api/payments/orders?page=1&limit=20`

**Authentication**: Required

**Query Parameters**:
- `page` (optional, default: 1): Page number
- `limit` (optional, default: 20): Results per page

**Response**:
```json
{
  "success": true,
  "orders": [...],
  "total": 10,
  "page": 1,
  "limit": 20
}
```

### 4. Crossmint Webhook Handler

Receives payment notifications from Crossmint.

**Endpoint**: `POST /api/webhooks/crossmint`

**Authentication**: None (protected by signature verification)

**Headers**:
- `X-Crossmint-Signature`: HMAC-SHA256 signature

**Request**:
```json
{
  "type": "order.paid",
  "data": {
    "orderId": "order_abc123",
    "status": "paid",
    "amount": "10.00",
    "currency": "USDT",
    "metadata": {
      "packageId": "pkg_starter",
      "credits": 600,
      "userId": "user_xyz"
    },
    "paidAt": "2025-12-28T13:45:00Z"
  }
}
```

**Response**:
```json
{
  "success": true,
  "received": true
}
```

## 🗄️ Database Schema

### payment_orders Table

```sql
CREATE TABLE payment_orders (
    id TEXT PRIMARY KEY,
    crossmint_order_id TEXT UNIQUE,
    user_id TEXT NOT NULL,
    package_id TEXT NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    currency TEXT NOT NULL DEFAULT 'USDT',
    credits INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    payment_method TEXT,
    crossmint_client_secret TEXT,
    webhook_received_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    failed_reason TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Order Status Flow

```
pending → processing → completed
                   ↘
                    → failed
                    → cancelled
                    → refunded
```

## 🔐 Security

### Webhook Signature Verification

Crossmint webhooks are protected by HMAC-SHA256 signatures:

```go
func VerifyWebhookSignature(signature string, body []byte) bool {
    mac := hmac.New(sha256.New, []byte(webhookSecret))
    mac.Write(body)
    expectedSig := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(signature), []byte(expectedSig))
}
```

### Idempotency Protection

- Each webhook event is processed only once
- Duplicate `order.paid` events for completed orders are safely ignored
- Reference IDs link payments to credit transactions

### Sensitive Data Handling

- `crossmint_client_secret` is never exposed in API responses
- Server API key is stored in environment variables only
- Webhook secret is never logged

## 🧪 Testing

### Unit Tests

**Models** (`config/payment_test.go`):
- ✅ CreatePaymentOrder (valid/invalid inputs)
- ✅ GetPaymentOrderByID
- ✅ GetPaymentOrderByCrossmintID
- ✅ UpdatePaymentOrderStatus
- ✅ UpdatePaymentOrderWithCrossmintID
- ✅ MarkPaymentOrderWebhookReceived
- ✅ GetUserPaymentOrders (pagination)
- ✅ CrossmintWebhookEvent parsing

**Service** (`service/payment/service_test.go`):
- ✅ CreatePaymentOrder
- ✅ GetPaymentOrder
- ✅ GetUserPaymentOrders
- ✅ VerifyWebhookSignature
- ✅ ProcessWebhook (all event types)
- ✅ Idempotency protection
- ✅ Environment configuration

**Handler** (`api/payment/handler_test.go`):
- ✅ CreateOrder (HTTP)
- ✅ GetOrder (HTTP)
- ✅ GetUserOrders (HTTP)
- ✅ HandleWebhook (HTTP)
- ✅ Authentication/Authorization
- ✅ Error handling
- ✅ Rate limiting

### Running Tests

```bash
# Run all payment tests
go test ./api/payment/... -v
go test ./service/payment/... -v
go test ./config/... -run Payment -v

# Run with coverage
go test ./api/payment/... -cover
go test ./service/payment/... -cover
go test ./config/... -run Payment -cover

# Generate coverage report
go test ./api/payment/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Test Coverage Goals

- ✅ **100% Function Coverage**: All functions tested
- ✅ **Edge Cases**: Invalid inputs, errors, race conditions
- ✅ **Integration**: End-to-end API flows
- ✅ **Security**: Signature verification, authorization

## 🚀 Deployment

### Environment Variables

```bash
# Required
CROSSMINT_SERVER_API_KEY=sk_staging_YOUR_KEY_HERE
CROSSMINT_WEBHOOK_SECRET=whsec_YOUR_SECRET_HERE

# Optional
CROSSMINT_ENVIRONMENT=staging  # or "production"
CROSSMINT_API_URL=https://staging.crossmint.com/api  # custom API URL
```

### Database Migration

```bash
# Apply migration
psql $DATABASE_URL < database/migrations/20251228_crossmint_payment/001_create_tables.sql

# Rollback (if needed)
psql $DATABASE_URL < database/migrations/20251228_crossmint_payment/002_rollback.sql
```

### Crossmint Console Setup

1. **Create Server API Key**:
   - Visit https://staging.crossmint.com/console
   - Navigate to `Developers` → `API Keys`
   - Create new server-side key with scopes:
     - `orders.create`
     - `orders.read`
     - `orders.update`

2. **Configure Webhook**:
   - Navigate to `Developers` → `Webhooks`
   - URL: `https://your-api.com/api/webhooks/crossmint`
   - Events: `order.paid`, `order.failed`, `order.cancelled`
   - Copy webhook secret to `.env`

## 📊 Monitoring & Logging

### Key Metrics

- Order creation rate
- Payment success rate
- Webhook processing time
- Failed payment reasons

### Log Examples

```
🔄 创建支付订单: userID=user_123, packageID=pkg_starter
✅ 支付订单创建成功: orderID=order_abc, amount=10.00 USDT, credits=600

🔄 调用Crossmint API创建订单: orderID=order_abc, amount=10.00 USDT
✅ Crossmint订单创建成功: crossmintOrderID=crossmint_xyz

📥 收到Crossmint webhook: type=order.paid, orderID=crossmint_xyz, status=paid
🔄 处理支付成功: orderID=order_abc, userID=user_123, credits=600
✅ 支付处理完成: orderID=order_abc, 积分已到账

❌ Crossmint API错误 (状态码 400): Invalid payment amount
⚠️ 订单已处理过，跳过: orderID=order_abc
```

## 🔄 Integration Flow

### Complete Payment Flow

```
1. User clicks "Buy Credits" → Frontend
2. Frontend calls POST /api/payments/crossmint/create-order → Backend
3. Backend creates payment_order record → Database
4. Backend calls Crossmint API → Crossmint
5. Crossmint returns orderId + clientSecret → Backend
6. Backend updates payment_order with crossmintOrderId → Database
7. Backend returns orderId + clientSecret → Frontend
8. Frontend displays Crossmint checkout UI → User
9. User completes payment → Crossmint
10. Crossmint sends webhook order.paid → Backend
11. Backend verifies signature → Security
12. Backend updates payment_order status → Database
13. Backend adds credits to user account → Database
14. Backend returns 200 OK → Crossmint
15. Frontend polls order status → Backend
16. Backend returns completed order → Frontend
17. Frontend shows success message → User
```

## 🛠️ Troubleshooting

### Common Issues

**1. "Payment system not configured" error**
- Check `CROSSMINT_SERVER_API_KEY` environment variable
- Verify key format: `sk_staging_...` or `sk_production_...`

**2. Webhook signature verification failing**
- Check `CROSSMINT_WEBHOOK_SECRET` is set correctly
- Verify webhook secret matches Crossmint Console
- Check for trailing spaces in environment variable

**3. Credits not added after payment**
- Check webhook logs: `grep "Crossmint webhook" /var/log/app.log`
- Verify order status in database: `SELECT * FROM payment_orders WHERE crossmint_order_id = '...'`
- Check credit transactions: `SELECT * FROM credit_transactions WHERE reference_id = '...'`

**4. Crossmint API returning errors**
- Verify API key has correct permissions
- Check API URL matches environment (staging vs production)
- Review Crossmint API logs in their console

## 📚 References

- [Crossmint API Documentation](https://docs.crossmint.com)
- [Crossmint Console](https://staging.crossmint.com/console)
- [Original Specification](/worktrees/cc/openspec/changes/migrate-crossmint-to-sdk/backend-api-spec.md)

## 🤝 Contributing

### Code Style

- Follow existing Go conventions
- Use descriptive variable names
- Add comments for complex logic
- Write tests for new features

### Pull Request Process

1. Update tests to cover new code
2. Run `go test ./...` and ensure all tests pass
3. Run `go fmt ./...` to format code
4. Update documentation if API changes
5. Add changelog entry

---

**Version**: 1.0.0
**Last Updated**: 2025-12-28
**Maintainer**: Backend Team
