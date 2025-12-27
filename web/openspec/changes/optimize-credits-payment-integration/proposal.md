## Why

Code audit of the credits payment integration revealed critical accessibility, security, and performance issues:

1. **Accessibility (WCAG 2.1 AA violations)**
   - PaymentModal missing ARIA roles and keyboard navigation (ESC key)
   - No focus management or focus trap implementation
   - CreditsValue keyboard navigation incomplete (missing preventDefault)
   - Language buttons missing aria-label and aria-current

2. **Security Issues**
   - Sensitive user data (userId, token) logged to browser console in production
   - PaymentModal inline styles violate Content Security Policy (CSP)
   - Hardcoded API key exposure risk (needs verification)

3. **Performance & Code Quality**
   - Excessive inline styles (40+ definitions in PaymentModal)
   - No component memoization for frequently re-rendering items
   - Inefficient state comparisons in package selection loop
   - Hardcoded Chinese text breaks internationalization

4. **User Experience**
   - Payment button not disabled during processing (allows double-submit)
   - No error recovery mechanism (missing retry UI)
   - Lack of loading state feedback for critical actions
   - Modal close behavior not discoverable (no hint text)

5. **Testing & Maintainability**
   - No unit tests for payment flow
   - Complex nested component structure difficult to test
   - Responsibility overload in single components
   - No component documentation or Storybook entries

## What Changes

### Phase 1: Critical Fixes (This Change)
- Implement complete ARIA accessibility for PaymentModal
- Add keyboard navigation (ESC to close, ENTER to submit)
- Implement focus management and focus trap
- Remove all sensitive logging from production code
- Extract inline styles to CSS modules
- Add button disabled states and loading indicators
- Fix hardcoded text to use i18n
- Add comprehensive error handling with retry

### Phase 2: High Priority Improvements (Follow-up)
- Refactor PaymentModal into smaller components (container/presentation pattern)
- Refactor CreditsDisplay complex logic into custom hook
- Add comprehensive unit and integration tests
- Implement Component composition patterns

### Phase 3: Medium Priority Enhancements (Future)
- Performance optimization (memoization, render optimization)
- Design system integration (CSS variables, theming)
- Props design expansion for flexibility
- Storybook documentation

## Impact

- **Affected specs**:
  - credits-display (CreditsValue keyboard interaction, logging)
  - payment-modal (accessibility, security, UX)
  - payment (component architecture)

- **Affected code**:
  - `src/components/CreditsDisplay/CreditsValue.tsx`
  - `src/components/CreditsDisplay/CreditsDisplay.tsx`
  - `src/components/Header.tsx`
  - `src/features/payment/components/PaymentModal.tsx`
  - `src/features/payment/styles/` (new CSS modules)

- **Breaking changes**: None - all changes are backward compatible

- **Migration**: None required - existing integrations work unchanged

- **Risk assessment**: Low - changes are localized and tested independently
