## MODIFIED Requirements

### Requirement: PaymentModal Accessibility and Keyboard Navigation
The PaymentModal component SHALL fully implement WCAG 2.1 Level AA accessibility standards with complete keyboard navigation support.

#### Scenario: Modal opens with proper ARIA structure
- **WHEN** PaymentModal opens (isOpen={true})
- **THEN** overlay div has role="presentation" (semantic purpose only)
- **AND** content container has role="dialog"
- **AND** modal has aria-modal="true" attribute
- **AND** modal has aria-labelledby pointing to modal title id
- **AND** modal has aria-describedby pointing to description (if present)

#### Scenario: User closes modal with Escape key
- **WHEN** PaymentModal is open
- **THEN** pressing Escape key closes the modal
- **AND** onClose callback is triggered
- **AND** focus returns to trigger element (CreditsValue)
- **AND** closing by Escape is discoverable (help text shown)

#### Scenario: User closes modal by clicking background
- **WHEN** PaymentModal is open and user clicks outside content area
- **THEN** only clicks on the background div trigger close
- **AND** clicks on modal content are not propagated
- **AND** behavior is documented with visual hint or aria-label

#### Scenario: Focus is properly managed within modal
- **WHEN** PaymentModal opens
- **THEN** focus moves to first focusable element (close button recommended)
- **AND** Tab key cycles through all focusable elements within modal only
- **AND** focus cannot escape to content outside modal (focus trap)
- **AND** when modal closes, focus returns to trigger element

#### Scenario: All buttons have proper ARIA labels
- **WHEN** modal displays action buttons (select, pay, close, retry)
- **THEN** each button has aria-label or visible text describing action
- **AND** buttons show loading state with aria-busy="true" during processing
- **AND** disabled buttons have aria-disabled="true" and tabIndex="-1"

---

### Requirement: PaymentModal CSS and Styling Architecture
The PaymentModal component SHALL use CSS modules instead of inline styles for maintainability, CSP compliance, and performance.

#### Scenario: All inline styles are extracted to CSS module
- **WHEN** PaymentModal renders
- **THEN** no inline style={{...}} objects are used
- **AND** all styling is defined in payment-modal.module.css
- **AND** styles follow BEM naming convention
- **AND** CSS supports dark and light themes via CSS variables

#### Scenario: Modal animations are defined in CSS
- **WHEN** PaymentModal displays loading, success, or error states
- **THEN** animations are defined in CSS @keyframes
- **AND** animations are not injected via <style> tag
- **AND** animation performance follows Chrome DevTools recommendations
- **AND** animations can be disabled via prefers-reduced-motion

#### Scenario: Responsive design is CSS-based
- **WHEN** screen size changes
- **THEN** modal width, padding, and layout adjust via CSS media queries
- **AND** modal scales appropriately on mobile (90vh constraint maintained)
- **AND** touch targets meet WCAG 44x44px minimum on mobile

---

### Requirement: PaymentModal Button State Management
The PaymentModal component SHALL prevent user errors by properly managing button states and preventing duplicate submissions.

#### Scenario: Payment button is disabled during processing
- **WHEN** context.paymentStatus is 'loading' or 'success'
- **THEN** "Continue Payment" button is disabled (disabled={true})
- **AND** button shows loading indicator or changed text ("Processing...")
- **AND** cursor changes to not-allowed when hovering
- **AND** aria-busy="true" is set during loading

#### Scenario: Package selection prevents invalid payments
- **WHEN** no package is selected
- **THEN** "Continue Payment" button is disabled
- **AND** disabled state has visual distinction (grayed out)
- **AND** tooltip shows "Please select a package first"

#### Scenario: Error state shows retry option
- **WHEN** paymentStatus is 'error'
- **THEN** error message displays specific error details
- **AND** "Retry" button is visible and enabled
- **AND** user can immediately retry without re-selecting package
- **AND** error message provides support contact information

#### Scenario: Success completion closes properly
- **WHEN** user clicks "Complete" button after success
- **THEN** PaymentModal closes
- **AND** onSuccess callback receives creditsAdded count
- **AND** context payment state is reset
- **AND** user sees confirmation message briefly before close

---

### Requirement: PaymentModal Configuration Error Handling
The PaymentModal component SHALL validate environment configuration at the appropriate lifecycle point.

#### Scenario: Missing API key shows helpful error
- **WHEN** VITE_CROSSMINT_CLIENT_API_KEY environment variable is missing
- **THEN** user-friendly error message displays: "支付功能配置不完整"
- **AND** error suggests contacting administrator
- **AND** error is shown with clear CTA (close/contact button)
- **AND** logging records this as configuration error (not user error)

#### Scenario: Configuration validated before component load
- **WHEN** Header component renders with simple={false}
- **THEN** API key existence is validated before opening modal
- **AND** missing key prevents modal from opening (early check)
- **AND** error is displayed inline in CreditsDisplay or toast notification

---

## ADDED Requirements

### Requirement: PaymentModal Error Recovery
The PaymentModal component SHALL provide clear error messages and recovery paths for all failure scenarios.

#### Scenario: Network error handling
- **WHEN** payment initiation fails with network error
- **THEN** error message shows: "网络连接失败，请检查网络后重试"
- **AND** error details (error code) are logged for debugging
- **AND** retry button is prominently displayed
- **AND** state allows user to try again without closing modal

#### Scenario: Payment gateway error handling
- **WHEN** Crossmint API returns error (e.g., invalid amount, declined)
- **THEN** specific error message displays based on error code
- **AND** error provides hint for resolution (e.g., "Please try a different payment method")
- **AND** sensitive error details are not shown to user

#### Scenario: Timeout error handling
- **WHEN** payment processing exceeds timeout threshold
- **THEN** user sees message: "处理超时，请检查您的订单状态"
- **AND** user can retry or navigate to order history
- **AND** backend receives notification about timeout for reconciliation

---

### Requirement: PaymentModal Internationalization
The PaymentModal component SHALL support multiple languages using the project's i18n system.

#### Scenario: All text uses i18n translations
- **WHEN** PaymentModal renders any user-facing text
- **THEN** text is retrieved from i18n system (not hardcoded)
- **AND** current language from useLanguage() context is respected
- **AND** switching language updates modal text immediately

#### Scenario: Locale-specific formatting
- **WHEN** displaying prices or credits
- **THEN** formatting respects locale (USD vs CNY, number format)
- **AND** currency symbols are correct for locale
- **AND** number separators follow locale conventions (1,000 vs 1.000)
