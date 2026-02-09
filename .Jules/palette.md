## 2025-05-14 - Password Visibility Toggle Accessibility
**Learning:** Icon-only buttons for toggling password visibility are often missing `aria-label` and `aria-pressed` states, making them unusable for screen reader users who need to verify their input.
**Action:** Always include dynamic `aria-label` (Switching between "Show password" and "Hide password") and `aria-pressed` state for password toggles.
