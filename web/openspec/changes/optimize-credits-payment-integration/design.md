## Context

The initial payment modal integration had several issues identified through comprehensive code audit:

1. **Accessibility**: No ARIA attributes, missing keyboard navigation (ESC key), no focus management
2. **Security**: Sensitive user data logged to console in production, inline styles violate CSP
3. **Performance**: 40+ inline style definitions, inefficient state comparisons, no memoization
4. **UX**: Payment button not disabled during processing (double-submit risk), no error recovery UI
5. **Maintainability**: Complex nested components, excessive inline styles, no tests

This change addresses critical and high-priority issues to meet WCAG 2.1 AA compliance and improve production readiness.

---

## Goals

### Primary Goals
1. Achieve WCAG 2.1 Level AA accessibility compliance for PaymentModal
2. Remove all sensitive data logging from production code
3. Extract inline styles to CSS modules for CSP compliance
4. Implement proper button state management to prevent user errors
5. Add error recovery mechanisms with retry capability

### Secondary Goals
1. Improve user experience with better feedback and state indicators
2. Refactor component structure for better testability and maintainability
3. Ensure comprehensive test coverage for payment flows
4. Support internationalization throughout payment flow

### Non-Goals
1. Component refactoring into separate files (Phase 2 work)
2. Performance optimization beyond removing inline styles
3. Complete redesign of payment flow
4. Integration with additional payment providers

---

## Decisions

### Decision 1: CSS Modules Instead of Inline Styles
**What**: Move all inline `style={{}}` objects to a dedicated CSS module file
**Why**:
- CSP compliance - inline styles violate Content Security Policy
- Maintainability - easier to understand and modify styling
- Performance - CSS is parsed once, not recreated per render
- Reusability - shared styles across components
- Browser DevTools - easier to debug styles

**Alternatives Considered**:
- Tailwind CSS: Not applicable for complex modal styling; inline styles would still be used
- Styled-components: Additional dependency; overkill for this use case
- CSS-in-JS: Same CSP issues as inline styles

**Rationale**: CSS Modules provide best balance of maintainability, performance, and compliance.

---

### Decision 2: Focus Trap Using useEffect
**What**: Implement keyboard focus management with focus trap in modal
**Why**:
- WCAG 2.1 AA requirement for modal dialogs
- Prevents accidental interaction with content behind modal
- Improves accessibility for keyboard and screen reader users
- Standard web practice for modals

**Implementation Approach**:
```typescript
useEffect(() => {
  if (!isOpen) return;

  // Focus first focusable element
  const focusableElements = contentRef.current?.querySelectorAll(
    'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
  );
  if (focusableElements?.length) {
    focusableElements[0].focus();
  }

  // Trap focus within modal
  const handleKeyDown = (e) => {
    if (e.key !== 'Tab') return;

    const first = focusableElements[0];
    const last = focusableElements[focusableElements.length - 1];

    if (e.shiftKey && document.activeElement === first) {
      last.focus();
      e.preventDefault();
    } else if (!e.shiftKey && document.activeElement === last) {
      first.focus();
      e.preventDefault();
    }
  };

  document.addEventListener('keydown', handleKeyDown);
  return () => document.removeEventListener('keydown', handleKeyDown);
}, [isOpen]);
```

**Alternatives**:
- Use aria-modal="true" only: Not sufficient per WCAG - actual focus management required
- Use third-party library (react-aria, headless-ui): Increases bundle size

**Rationale**: Manual implementation is appropriate for this use case; library would be overkill.

---

### Decision 3: Disable Payment Button During Processing
**What**: Set disabled={true} on payment submission buttons during loading/success states
**Why**:
- Prevents double-submission of payment requests
- Common payment gateway best practice
- Protects against user impatience leading to duplicate charges
- Provides visual feedback that action is in progress

**State Logic**:
```typescript
const isPaymentDisabled =
  !context.selectedPackage ||
  context.paymentStatus !== 'idle' ||
  context.paymentStatus !== 'error';

<button
  disabled={isPaymentDisabled}
  aria-busy={context.paymentStatus === 'loading'}
  onClick={handlePayment}
>
  {context.paymentStatus === 'loading' ? '处理中...' : '继续支付'}
</button>
```

**Rationale**: Critical for payment system reliability.

---

### Decision 4: Environment Variable Check Location
**What**: Keep API key validation at component render level, with early return
**Why**:
- Catches missing configuration immediately when modal opens
- Provides user-friendly error message
- Doesn't break payment context initialization
- Can be enhanced with top-level check in Phase 2

**Current Implementation**:
```typescript
const apiKey = import.meta.env.VITE_CROSSMINT_CLIENT_API_KEY;
if (!isOpen || !apiKey) return null;

// Only render modal if both conditions are met
```

**Future Enhancement** (Phase 2):
- Add top-level check in App.tsx or Header.tsx
- Show toast notification instead of silent failure
- Log configuration error for debugging

**Rationale**: Minimal change for Phase 1; prevents breaking existing functionality.

---

### Decision 5: Hardcoded Text to i18n Translation
**What**: Replace "(用户积分)" and other hardcoded Chinese text with i18n translations
**Why**:
- Application already uses i18n system (useLanguage, translations.ts)
- Supports international users
- Maintains consistency with rest of application
- Easier to maintain translations in centralized file

**Implementation**:
```typescript
// In i18n/translations.ts
export const translations = {
  zh: {
    userCredits: '用户积分',
    selectPackage: '选择你想要购买的积分套餐',
    // ... other translations
  },
  en: {
    userCredits: 'Credits',
    selectPackage: 'Select the package you want to purchase',
    // ... other translations
  }
};

// In component
const { language } = useLanguage();
const userCreditsLabel = translations[language].userCredits;
```

**Rationale**: Aligns with existing application architecture.

---

### Decision 6: No Sensitive Data in Console Logs
**What**: Remove userId, token, email from console.log statements in production
**Why**:
- Security best practice - prevent accidental exposure of auth tokens/IDs
- Compliance - follows OWASP guidelines on logging sensitive data
- Privacy - respects user data minimization principle
- Browser DevTools - anyone can open console and see logs

**Implementation**:
```typescript
// ❌ BEFORE (Security risk in production)
console.log('[CreditsDisplay] Auth state:', {
  userId: user?.id,      // Sensitive!
  hasToken: !!token,     // Sensitive!
  authLoading,
  credits,
  loading,
  error: error?.message
});

// ✅ AFTER (Secure)
if (process.env.NODE_ENV === 'development') {
  console.debug('[CreditsDisplay] Auth state:', { /* ... */ });
}
```

**Alternatives**:
- Use structured logging library (winston): Over-engineering for this case
- Disable all logging: Makes debugging harder
- Keep logs but mask sensitive data: Still risky

**Rationale**: Simple, effective, follows industry best practices.

---

### Decision 7: Focus Restoration After Modal Close
**What**: Restore focus to CreditsValue element when modal closes
**Why**:
- Keyboard navigation continuity
- Screen reader users expect focus to return to trigger
- WCAG 2.1 requirement for dialogs
- Improves navigation flow

**Implementation**:
```typescript
// Store reference to trigger element when modal opens
useEffect(() => {
  if (isOpen) {
    triggerRef.current = document.activeElement;
  }
}, [isOpen]);

// Restore focus when modal closes
const handleClose = useCallback(() => {
  onClose();
  // Restore focus to trigger element
  setTimeout(() => {
    triggerRef.current?.focus();
  }, 0);
}, [onClose]);
```

**Rationale**: Critical for keyboard navigation accessibility.

---

## Risks / Trade-offs

### Risk 1: CSS Module Bundle Size
**Risk**: Adding 2-3KB CSS module could increase bundle size
**Mitigation**: Removal of inline styles creates similar or smaller net size
**Impact**: Low
**Likelihood**: Low (net positive or neutral)

### Risk 2: Browser Compatibility for CSS Features
**Risk**: CSS Grid, Flexbox, CSS Variables may not work in very old browsers
**Mitigation**: Project already uses modern CSS features; no new constraints
**Impact**: Low
**Likelihood**: Low

### Risk 3: Focus Trap Implementation Complexity
**Risk**: Focus trap logic could have edge cases (Shadow DOM, portals)
**Mitigation**: Start with simple implementation; can enhance in Phase 2
**Impact**: Medium
**Likelihood**: Low (project doesn't use Shadow DOM)

### Risk 4: Payment Flow Regression
**Risk**: Changes to button states could break payment flow
**Mitigation**: Comprehensive test coverage; test before deployment
**Impact**: Critical
**Likelihood**: Very Low (button state changes are isolated)

### Risk 5: Internationalization Inconsistency
**Risk**: Missing translations for new strings
**Mitigation**: Add all translation keys upfront; test in both languages
**Impact**: Medium
**Likelihood**: Low (can be caught in testing)

---

## Migration Plan

### Phase 1: Implementation (1-2 days)
1. Create CSS module file with all payment modal styles
2. Update PaymentModal component to use CSS modules
3. Add focus trap implementation
4. Add Escape key listener
5. Implement button disabled states
6. Remove sensitive logging
7. Add i18n translations
8. Update CreditsValue keyboard handling

### Phase 2: Testing (1 day)
1. Unit tests for each component
2. Integration tests for payment flow
3. Accessibility tests with axe-core
4. Manual testing on mobile/desktop
5. Manual keyboard navigation testing

### Phase 3: Review & Deployment (0.5 day)
1. Code review
2. Staging deployment
3. Verification testing
4. Production deployment

### Rollback Plan
If critical issue discovered post-deployment:
1. Revert to previous commit
2. Investigate issue
3. Re-deploy after fix

---

## Open Questions

1. **API Key Security**: Is `VITE_CROSSMINT_CLIENT_API_KEY` definitely a public Client Key? Should verify no Secret Key is exposed.

2. **Error Messages**: Should error messages be translated or show error codes for debugging?

3. **Modal Stacking**: Should PaymentModal support multiple stacked modals in future?

4. **Payment Confirmation**: Should there be confirmation dialog before processing payment?

5. **Loading Timeout**: What's the timeout threshold for showing "Processing timeout" error?

6. **Retry Limits**: Should there be a limit on number of retry attempts?

7. **Analytics**: Should payment flow events be tracked (modal open, package select, payment success/failure)?

---

## Implementation Notes

- Use TypeScript strict mode throughout
- Add JSDoc comments for complex logic
- Test with both keyboard and mouse interactions
- Verify with screen reader (NVDA for Windows, VoiceOver for Mac)
- Test on Chrome, Firefox, Safari, Edge
- Test on iOS Safari and Android Chrome
- Use Lighthouse for performance and accessibility audit
