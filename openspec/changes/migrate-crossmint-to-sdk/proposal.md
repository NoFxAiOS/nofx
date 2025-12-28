# Proposal: Migrate Crossmint Integration from Legacy API to Official SDK

**Date**: 2025-12-28
**Status**: 🟢 Approved for Implementation
**Priority**: P0 (Critical - Payment feature is broken)
**Type**: Bug Fix + Refactoring

---

## 📋 Problem Statement

### Current Issue
Users cannot complete credit purchases. The payment modal shows:
```
支付失败
支付服务暂时不可用: Failed to initialize Crossmint checkout: Failed to fetch
```

### Root Cause
1. **Legacy API endpoint is deprecated**: `https://api.crossmint.com/2022-06-09/embedded-checkouts` returns HTTP 404
2. **Direct API calls are not recommended**: Crossmint now recommends using their official SDK (`@crossmint/client-sdk-react-ui`)

### Verification
```bash
# API test returned:
HTTP/2 404
x-vercel-error: DEPLOYMENT_NOT_FOUND
The deployment could not be found on Vercel.
```

---

## 🎯 Objectives

1. **Primary**: Fix payment functionality by migrating to Crossmint SDK
2. **Secondary**: Improve code maintainability and follow official best practices
3. **Tertiary**: Maintain 100% test coverage

---

## 🏗️ Architecture Design

### Design Principles (KISS + High Cohesion, Low Coupling)

#### Before (Current - Tightly Coupled)
```
CrossmintService (Direct fetch API)
    ↓
Crossmint REST API (Deprecated)
    ↓
Error: 404 Not Found
```

#### After (Proposed - Loosely Coupled with Adapter Pattern)
```
CrossmintService (Abstract Interface)
    ↓
CrossmintSDKAdapter (Implements Interface)
    ↓
@crossmint/client-sdk-react-ui (Official SDK)
    ↓
Crossmint API (Latest Version)
```

### Key Design Patterns

1. **Adapter Pattern**: Wrap SDK in adapter to maintain interface compatibility
2. **Dependency Injection**: Inject adapter into orchestrator
3. **Interface Segregation**: Keep existing public API unchanged

---

## 📐 Implementation Strategy

### Phase 1: Create SDK Adapter (High Cohesion)

**New File**: `src/features/payment/services/CrossmintSDKAdapter.ts`

**Responsibilities** (Single Responsibility Principle):
- Wrap Crossmint SDK
- Translate between our domain model and SDK model
- Handle SDK-specific error handling

**Interface Compatibility**:
```typescript
interface ICrossmintService {
  isConfigured(): boolean
  initializeCheckout(config: CheckoutConfig): Promise<string>
  // ... other methods
}
```

### Phase 2: Update Service Integration (Low Coupling)

**Modified File**: `src/features/payment/services/PaymentOrchestrator.ts`

**Changes**:
- Accept `ICrossmintService` interface instead of concrete class
- No changes to public API
- Dependency injection maintained

### Phase 3: Update Provider (Minimal Changes)

**Modified File**: `src/features/payment/contexts/PaymentProvider.tsx`

**Changes**:
- Instantiate `CrossmintSDKAdapter` instead of old service
- No changes to component interface

### Phase 4: Deprecate Legacy Service

**File**: `src/features/payment/services/CrossmintService.ts`

**Action**: Mark as deprecated, keep for backward compatibility

---

## 🧪 Testing Strategy (100% Coverage Required)

### Unit Tests

**New Test File**: `src/features/payment/services/__tests__/CrossmintSDKAdapter.test.ts`

Tests:
1. ✅ SDK initialization with valid API key
2. ✅ SDK initialization with invalid API key
3. ✅ Create checkout session success
4. ✅ Create checkout session failure
5. ✅ Error message translation
6. ✅ Configuration validation
7. ✅ Interface compatibility

### Integration Tests

**Modified Test File**: `src/features/payment/__tests__/payment-flow.integration.test.ts`

Tests:
1. ✅ Complete payment flow with SDK
2. ✅ Error handling with SDK
3. ✅ Session creation and retrieval
4. ✅ Backward compatibility check

### E2E Tests (Optional)

**New Test File**: `tests/e2e/payment-sdk.spec.ts`

Tests:
1. ✅ Open payment modal
2. ✅ Select package
3. ✅ Initialize checkout (mocked SDK)
4. ✅ Verify session ID displayed

---

## 📝 Implementation Checklist

### Dependencies
- [x] Install `@crossmint/client-sdk-react-ui`

### Code Changes
- [ ] Create `CrossmintSDKAdapter.ts` with interface implementation
- [ ] Create unit tests for adapter (100% coverage)
- [ ] Update `PaymentOrchestrator.ts` to use interface
- [ ] Update `PaymentProvider.tsx` to instantiate adapter
- [ ] Deprecate `CrossmintService.ts` (mark with JSDoc)
- [ ] Update integration tests
- [ ] Add type definitions if needed

### Documentation
- [ ] Update `INTEGRATION_GUIDE.md` with SDK instructions
- [ ] Add migration notes
- [ ] Update API documentation

### Validation
- [ ] All unit tests pass (100% coverage)
- [ ] All integration tests pass
- [ ] TypeScript compilation succeeds
- [ ] No regression in other features
- [ ] Manual testing in dev environment

---

## 🔒 Risk Mitigation

### Risks

1. **SDK Breaking Changes**: Official SDK might have different API
   - **Mitigation**: Adapter pattern isolates changes

2. **TypeScript Type Issues**: SDK types might conflict
   - **Mitigation**: Create type adapters if needed

3. **Test Coverage Gaps**: New code might miss edge cases
   - **Mitigation**: TDD approach, write tests first

4. **Backward Compatibility**: Existing code might break
   - **Mitigation**: Keep old service, extensive integration tests

### Rollback Plan

If issues arise:
1. Revert to previous version (old service)
2. Feature flag to toggle between old/new implementation
3. Gradual rollout (staging → production)

---

## 📊 Success Metrics

1. ✅ Payment initialization succeeds (no 404 errors)
2. ✅ Checkout session created successfully
3. ✅ 100% test coverage maintained
4. ✅ No impact on other features (regression tests pass)
5. ✅ TypeScript compilation with no errors
6. ✅ Code maintainability improved (clean architecture)

---

## 🚀 Rollout Plan

### Stage 1: Development (Today)
- Implement SDK adapter
- Write comprehensive tests
- Local validation

### Stage 2: Staging (Today)
- Deploy to staging environment
- Manual testing with real API key
- Verify payment flow end-to-end

### Stage 3: Production (After Validation)
- Deploy to production
- Monitor error rates
- Verify user payments working

---

## 📚 References

- [Crossmint SDK Documentation](https://docs.crossmint.com/payments/embedded/quickstart)
- [Crossmint SDK GitHub](https://github.com/Crossmint/crossmint-sdk)
- [Adapter Pattern](https://refactoring.guru/design-patterns/adapter)
- [SOLID Principles](https://en.wikipedia.org/wiki/SOLID)

---

## ✅ Approval

**Proposed By**: Claude Code Agent
**Reviewed By**: User
**Approved By**: User
**Implementation Start**: 2025-12-28

---

**Next Steps**: Proceed with Phase 1 - Create SDK Adapter
