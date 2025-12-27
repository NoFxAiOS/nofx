## Phase 1: Critical Accessibility & Security Fixes

### 1.1 PaymentModal Accessibility Implementation
- [ ] 1.1.1 Add role="dialog" and ARIA attributes to PaymentModal
- [ ] 1.1.2 Implement focus trap using useEffect hook
- [ ] 1.1.3 Add Escape key listener to close modal with proper cleanup
- [ ] 1.1.4 Implement focus restoration when modal closes
- [ ] 1.1.5 Add aria-label and aria-describedby to dialog container
- [ ] 1.1.6 Add aria-label to all action buttons (close, select, pay, retry)
- [ ] 1.1.7 Add aria-busy to buttons during payment processing
- [ ] 1.1.8 Add aria-disabled to disabled buttons

### 1.2 PaymentModal Styling Refactor
- [ ] 1.2.1 Create payment-modal.module.css file
- [ ] 1.2.2 Extract overlay styles to CSS module
- [ ] 1.2.3 Extract content container styles to CSS module
- [ ] 1.2.4 Extract button styles to CSS module
- [ ] 1.2.5 Extract form/input styles to CSS module
- [ ] 1.2.6 Extract state-specific styles (idle, loading, success, error)
- [ ] 1.2.7 Move @keyframes animations to CSS file
- [ ] 1.2.8 Remove all inline style={{}} objects from JSX
- [ ] 1.2.9 Update imports to use CSS module classes

### 1.3 Security: Remove Sensitive Logging
- [ ] 1.3.1 Wrap console.log in CreditsDisplay with process.env.NODE_ENV check
- [ ] 1.3.2 Move sensitive logs to console.debug (only in dev mode)
- [ ] 1.3.3 Remove userId logging
- [ ] 1.3.4 Remove token/auth status logging from production
- [ ] 1.3.5 Use structured logging approach for error cases only

### 1.4 CreditsValue Keyboard & i18n
- [ ] 1.4.1 Add event.preventDefault() in keyboard handler
- [ ] 1.4.2 Add aria-label prop to CreditsValue
- [ ] 1.4.3 Replace hardcoded "(用户积分)" with i18n translation
- [ ] 1.4.4 Use useLanguage hook for dynamic text
- [ ] 1.4.5 Add disabled and loading prop support

### 1.5 Button State Management
- [ ] 1.5.1 Add disabled state to payment submit button
- [ ] 1.5.2 Disable button during loading state (context.paymentStatus === 'loading')
- [ ] 1.5.3 Disable button when no package selected
- [ ] 1.5.4 Disable button during success state until closed
- [ ] 1.5.5 Update button text to show loading state ("Processing...")
- [ ] 1.5.6 Add visual feedback for disabled state

### 1.6 Error Recovery UI
- [ ] 1.6.1 Ensure error messages display in error state
- [ ] 1.6.2 Add retry button in error state (context.resetPayment)
- [ ] 1.6.3 Provide clear action: show error details with context
- [ ] 1.6.4 Add support contact information in error message

---

## Phase 2: UX & Component Improvements

### 2.1 PaymentModal UX Enhancements
- [ ] 2.1.1 Add loading skeleton for package list during fetch
- [ ] 2.1.2 Add visual indication that background click closes modal
- [ ] 2.1.3 Add package selection change feedback (visual highlight)
- [ ] 2.1.4 Prevent scroll of page behind modal
- [ ] 2.1.5 Add smooth transitions between states

### 2.2 CreditsValue Enhancements
- [ ] 2.2.1 Add loading spinner support
- [ ] 2.2.2 Add disabled cursor style
- [ ] 2.2.3 Add hover effect to indicate clickability
- [ ] 2.2.4 Update prop interface with disabled and loading

### 2.3 CreditsDisplay Improvements
- [ ] 2.3.1 Add retry button in error state
- [ ] 2.3.2 Use refetch hook from useUserCredits for retry
- [ ] 2.3.3 Improve loading skeleton appearance (match design)

### 2.4 Header Component Refinement
- [ ] 2.4.1 Add aria-label to language toggle buttons
- [ ] 2.4.2 Add aria-current to active language button
- [ ] 2.4.3 Add loading state feedback on language change

---

## Phase 3: Testing

### 3.1 Unit Tests - PaymentModal
- [ ] 3.1.1 Test ARIA attributes are present
- [ ] 3.1.2 Test Escape key closes modal
- [ ] 3.1.3 Test background click closes modal
- [ ] 3.1.4 Test focus trap implementation
- [ ] 3.1.5 Test button disabled states
- [ ] 3.1.6 Test error state and retry flow
- [ ] 3.1.7 Test success state and completion

### 3.2 Unit Tests - CreditsValue
- [ ] 3.2.1 Test keyboard Enter key functionality
- [ ] 3.2.2 Test keyboard Space key functionality
- [ ] 3.2.3 Test preventDefault is called
- [ ] 3.2.4 Test disabled state prevents interaction
- [ ] 3.2.5 Test loading state appearance
- [ ] 3.2.6 Test aria-label attribute

### 3.3 Unit Tests - CreditsDisplay
- [ ] 3.3.1 Test loading state rendering
- [ ] 3.3.2 Test error state with retry button
- [ ] 3.3.3 Test no console logging in production
- [ ] 3.3.4 Test onOpenPayment callback is called

### 3.4 Unit Tests - Header
- [ ] 3.4.1 Test PaymentModal integration
- [ ] 3.4.2 Test language switching
- [ ] 3.4.3 Test responsive layout

### 3.5 Integration Tests
- [ ] 3.5.1 Test complete payment flow: click credits → modal opens → select → pay
- [ ] 3.5.2 Test keyboard navigation flow
- [ ] 3.5.3 Test error and retry flow
- [ ] 3.5.4 Test accessibility with screen reader (axe-core)
- [ ] 3.5.5 Test on mobile and desktop screen sizes

### 3.6 Accessibility Testing
- [ ] 3.6.1 Run axe-core accessibility scanner
- [ ] 3.6.2 Test with keyboard-only navigation
- [ ] 3.6.3 Test with screen reader (NVDA/JAWS simulation)
- [ ] 3.6.4 Test focus indicators visibility
- [ ] 3.6.5 Test color contrast ratios

---

## Phase 4: Performance & Build

### 4.1 CSS Module Creation
- [ ] 4.1.1 Create src/features/payment/styles/payment-modal.module.css
- [ ] 4.1.2 Verify CSS module imports work
- [ ] 4.1.3 Test CSS is bundled correctly

### 4.2 Build & Validation
- [ ] 4.2.1 Run tsc to check TypeScript compilation
- [ ] 4.2.2 Run npm run build to build production bundle
- [ ] 4.2.3 Check for any console warnings or errors
- [ ] 4.2.4 Verify bundle size doesn't increase significantly

### 4.3 Environment Variable Validation
- [ ] 4.3.1 Verify VITE_CROSSMINT_CLIENT_API_KEY is configured
- [ ] 4.3.2 Test error path when API key is missing
- [ ] 4.3.3 Verify environment variables are loaded correctly

---

## Phase 5: Deployment & Verification

### 5.1 Pre-Deployment
- [ ] 5.1.1 All tests passing locally
- [ ] 5.1.2 No TypeScript errors
- [ ] 5.1.3 No console warnings in production build
- [ ] 5.1.4 No accessibility violations (axe-core)

### 5.2 Vercel Deployment
- [ ] 5.2.1 Deploy to Vercel staging
- [ ] 5.2.2 Test on staging environment
- [ ] 5.2.3 Verify all features work on staging
- [ ] 5.2.4 Deploy to production

### 5.3 Post-Deployment Testing
- [ ] 5.3.1 Test complete payment flow on production
- [ ] 5.3.2 Verify keyboard navigation works
- [ ] 5.3.3 Verify error handling works
- [ ] 5.3.4 Check browser console for errors
- [ ] 5.3.5 Verify no sensitive data in logs

### 5.4 Monitoring
- [ ] 5.4.1 Monitor error tracking for payment issues
- [ ] 5.4.2 Collect user feedback on new UX
- [ ] 5.4.3 Track payment conversion metrics

---

## Summary

**Total Tasks**: 73 tasks across 5 phases
**Critical Priority**: 26 tasks (Phases 1-2)
**Testing**: 20 tasks (Phase 3)
**Performance**: 10 tasks (Phase 4)
**Deployment**: 17 tasks (Phase 5)

**Estimated Implementation Order**:
1. Complete Phase 1 tasks first (accessibility, security, styling)
2. Complete Phase 3 tasks (testing - can run in parallel)
3. Complete Phase 2 tasks (UX improvements)
4. Complete Phase 4 & 5 (build and deployment)
