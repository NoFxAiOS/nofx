# Tasks: Migrate Crossmint to SDK

**Status**: 🟡 In Progress
**Started**: 2025-12-28

---

## ✅ Completed Tasks

- [x] Install `@crossmint/client-sdk-react-ui` dependency
- [x] Create OpenSpec proposal document

---

## 🚧 In Progress

- [ ] Phase 1: Create SDK Adapter
- [ ] Phase 2: Write comprehensive tests
- [ ] Phase 3: Integration updates
- [ ] Phase 4: Validation

---

## 📋 Detailed Task Breakdown

### Phase 1: SDK Adapter Implementation

#### Task 1.1: Define Interface
- [ ] Create `ICrossmintService` interface
- [ ] Document interface methods
- [ ] Add TypeScript type definitions

#### Task 1.2: Implement Adapter
- [ ] Create `CrossmintSDKAdapter.ts`
- [ ] Implement `initializeCheckout()` method
- [ ] Implement error handling
- [ ] Add configuration validation

#### Task 1.3: Type Safety
- [ ] Import SDK types
- [ ] Create type adapters for domain models
- [ ] Add JSDoc documentation

---

### Phase 2: Testing (100% Coverage)

#### Task 2.1: Unit Tests
- [ ] Test SDK initialization
- [ ] Test checkout creation success path
- [ ] Test checkout creation error path
- [ ] Test configuration validation
- [ ] Test error message translation
- [ ] Mock SDK dependencies

#### Task 2.2: Integration Tests
- [ ] Update payment flow integration test
- [ ] Test adapter integration with orchestrator
- [ ] Test error propagation
- [ ] Verify backward compatibility

---

### Phase 3: Service Integration

#### Task 3.1: Update Orchestrator
- [ ] Accept interface instead of concrete class
- [ ] Update dependency injection
- [ ] Maintain public API compatibility

#### Task 3.2: Update Provider
- [ ] Instantiate adapter instead of old service
- [ ] Pass API key to adapter
- [ ] Verify context unchanged

---

### Phase 4: Validation & Documentation

#### Task 4.1: Verification
- [ ] Run unit tests → 100% pass
- [ ] Run integration tests → 100% pass
- [ ] TypeScript compilation → No errors
- [ ] Linting → No errors
- [ ] Manual dev testing → Payment works

#### Task 4.2: Documentation
- [ ] Update INTEGRATION_GUIDE.md
- [ ] Add migration notes
- [ ] Document adapter usage

---

## 🎯 Acceptance Criteria

- ✅ Payment modal opens without errors
- ✅ Checkout session created successfully
- ✅ Session ID returned and displayed
- ✅ All tests pass (100% coverage)
- ✅ No TypeScript errors
- ✅ No impact on other features

---

## 📝 Notes

- SDK installed: `@crossmint/client-sdk-react-ui`
- API Key configured: `VITE_CROSSMINT_CLIENT_API_KEY`
- Test environment: Staging
