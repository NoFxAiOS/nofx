## MODIFIED Requirements

### Requirement: CreditsValue Component Keyboard Accessibility
The CreditsValue component SHALL fully support keyboard navigation with proper event handling and ARIA attributes.

#### Scenario: User navigates with keyboard and presses Enter
- **WHEN** user navigates to CreditsValue element via Tab key
- **THEN** element receives focus with visible focus indicator
- **AND** user can press Enter key to trigger payment modal
- **AND** event.preventDefault() is called to prevent default behavior
- **AND** element has aria-label describing "Open payment modal to purchase credits"

#### Scenario: User navigates with keyboard and presses Space
- **WHEN** user has focus on CreditsValue element
- **THEN** user can press Space key to trigger payment modal
- **AND** Space key default behavior is prevented
- **AND** payment modal opens without page scrolling

#### Scenario: Credits value displays with proper formatting and i18n
- **WHEN** component renders the credits display text
- **THEN** hardcoded "(用户积分)" text is replaced with i18n translated value
- **AND** English and Chinese locales both display correctly
- **AND** text follows project i18n conventions using useLanguage context

---

### Requirement: CreditsDisplay Secure Logging Practices
The CreditsDisplay component SHALL NOT expose sensitive user information through browser console in production environments.

#### Scenario: Development environment has detailed logging
- **WHEN** process.env.NODE_ENV is 'development'
- **THEN** console.debug outputs detailed auth state for debugging
- **AND** only development builds include these logs

#### Scenario: Production environment has no sensitive logging
- **WHEN** process.env.NODE_ENV is 'production'
- **THEN** no sensitive information (userId, token, email) is logged
- **AND** critical errors are logged with structured format only
- **AND** error messages don't contain user IDs or auth tokens

---

## ADDED Requirements

### Requirement: CreditsValue Loading State Support
The CreditsValue component SHALL support disabled and loading states to prevent multiple interactions during payment flow.

#### Scenario: Component displays disabled state
- **WHEN** disabled prop is true
- **THEN** element has disabled cursor style (not-allowed)
- **AND** onClick handler is ignored
- **AND** aria-disabled is set to true
- **AND** element is not keyboard-focusable (tabIndex=-1)

#### Scenario: Component displays loading state
- **WHEN** loading prop is true
- **THEN** spinner icon appears next to value
- **AND** clicks are prevented during loading
- **AND** aria-busy is set to true for screen readers
