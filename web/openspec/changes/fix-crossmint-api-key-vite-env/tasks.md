## 1. Root Cause Analysis & Verification

### 1.1 Verify Vite Environment Variable Behavior
- [ ] 1.1.1 Confirm Vite by default only exposes VITE_* prefixed variables
- [ ] 1.1.2 Check vite.config.ts has no custom env.prefix configuration
- [ ] 1.1.3 Verify current variable CROSSMINT_CLIENT_API_KEY lacks VITE_ prefix
- [ ] 1.1.4 Test that import.meta.env.CROSSMINT_CLIENT_API_KEY returns undefined
- [ ] 1.1.5 Understand why previous security fix removed the VITE_ prefix

### 1.2 Verify Current Code References
- [ ] 1.2.1 Locate all references to CROSSMINT_CLIENT_API_KEY in code
- [ ] 1.2.2 Check CrossmintService.ts line 13 usage
- [ ] 1.2.3 Check PaymentModal.tsx line 79 usage
- [ ] 1.2.4 Search for any other environment variable references
- [ ] 1.2.5 Document all files that need updating

### 1.3 Verify Vercel Configuration
- [ ] 1.3.1 Check Vercel project environment variables settings
- [ ] 1.3.2 Verify CROSSMINT_CLIENT_API_KEY is currently set (without VITE_ prefix)
- [ ] 1.3.3 Confirm whether it's set for production, preview, and development
- [ ] 1.3.4 Note the current API key value location
- [ ] 1.3.5 Plan for updating to VITE_CROSSMINT_CLIENT_API_KEY

### 1.4 Clarify Security Requirements
- [ ] 1.4.1 Confirm Crossmint Client API Key is meant to be public
- [ ] 1.4.2 Understand difference between Client API Key vs Server Secrets
- [ ] 1.4.3 Verify Crossmint documentation on key exposure
- [ ] 1.4.4 Confirm no server secrets are being exposed

---

## 2. Code Implementation

### 2.1 Update Environment Variable References
- [ ] 2.1.1 Open src/features/payment/services/CrossmintService.ts
- [ ] 2.1.2 Change line 13: `import.meta.env.CROSSMINT_CLIENT_API_KEY` → `import.meta.env.VITE_CROSSMINT_CLIENT_API_KEY`
- [ ] 2.1.3 Open src/features/payment/components/PaymentModal.tsx
- [ ] 2.1.4 Change line 79: `import.meta.env.CROSSMINT_CLIENT_API_KEY` → `import.meta.env.VITE_CROSSMINT_CLIENT_API_KEY`
- [ ] 2.1.5 Search entire codebase for remaining references

### 2.2 Update TypeScript Type Definitions
- [ ] 2.2.1 Open src/vite-env.d.ts
- [ ] 2.2.2 Add to ImportMetaEnv interface:
  ```typescript
  readonly VITE_CROSSMINT_CLIENT_API_KEY: string
  ```
- [ ] 2.2.3 Verify proper indentation and formatting
- [ ] 2.2.4 Add explanatory comment about Client API Key
- [ ] 2.2.5 Ensure no duplicate declarations

### 2.3 Update Local Development Environment
- [ ] 2.3.1 Open .env.local
- [ ] 2.3.2 Rename CROSSMINT_CLIENT_API_KEY → VITE_CROSSMINT_CLIENT_API_KEY
- [ ] 2.3.3 Add your test/staging API key value
- [ ] 2.3.4 Save file
- [ ] 2.3.5 Do NOT commit .env.local with actual API key

### 2.4 Code Quality Verification
- [ ] 2.4.1 Ensure no hardcoded API keys in code
- [ ] 2.4.2 Check for console.log statements with API keys
- [ ] 2.4.3 Verify no API key duplication
- [ ] 2.4.4 Ensure consistent naming across all files
- [ ] 2.4.5 Add TODO comments if incomplete integration

---

## 3. Build & Compilation

### 3.1 Local Build Verification
- [ ] 3.1.1 Run: npm run build
- [ ] 3.1.2 Check for TypeScript errors: should be 0
- [ ] 3.1.3 Verify no console warnings during build
- [ ] 3.1.4 Check bundle size (should be minimal increase)
- [ ] 3.1.5 Verify build completes in ~1.5-2 seconds

### 3.2 Development Server Testing
- [ ] 3.2.1 Run: npm run dev
- [ ] 3.2.2 Check browser console for Vite errors
- [ ] 3.2.3 Verify VITE_CROSSMINT_CLIENT_API_KEY is available in dev
- [ ] 3.2.4 Check that import.meta.env returns the variable value
- [ ] 3.2.5 Test on different ports if needed

### 3.3 Type Checking
- [ ] 3.3.1 Ensure TypeScript recognizes VITE_CROSSMINT_CLIENT_API_KEY
- [ ] 3.3.2 Verify IDE autocomplete works for the variable
- [ ] 3.3.3 Check for "Property does not exist" errors
- [ ] 3.3.4 Compile without errors: npm run build
- [ ] 3.3.5 No type warnings in console

---

## 4. Local Testing

### 4.1 CrossmintService Configuration Test
- [ ] 4.1.1 Run dev server: npm run dev
- [ ] 4.1.2 Open browser console
- [ ] 4.1.3 Check that NO warning "API Key not configured" appears
- [ ] 4.1.4 Verify CrossmintService.isConfigured() returns true
- [ ] 4.1.5 Check that API key value is accessible

### 4.2 Payment Modal Test
- [ ] 4.2.1 Navigate to home page
- [ ] 4.2.2 Click "积分套餐" button
- [ ] 4.2.3 PaymentModal should open without error
- [ ] 4.2.4 No "Payment feature temporarily unavailable" message
- [ ] 4.2.5 Check browser console for errors

### 4.3 Environment Variable Visibility
- [ ] 4.3.1 Open browser DevTools console
- [ ] 4.3.2 Type: import.meta.env.VITE_CROSSMINT_CLIENT_API_KEY
- [ ] 4.3.3 Should display the API key value (not undefined or empty)
- [ ] 4.3.4 Repeat test on different pages
- [ ] 4.3.5 Verify persistence across page reloads

### 4.4 Error Scenarios Test
- [ ] 4.4.1 Temporarily remove API key from .env.local
- [ ] 4.4.2 Restart dev server
- [ ] 4.4.3 Verify that warning "API Key not configured" NOW appears
- [ ] 4.4.4 This confirms the fix works (warning appears when key is actually missing)
- [ ] 4.4.5 Restore API key for further testing

---

## 5. Vercel Configuration

### 5.1 Update Vercel Environment Variables
- [ ] 5.1.1 Log into Vercel project: prj_xMoVJ4AGtNNIiX6nN9uCgRop6KsP
- [ ] 5.1.2 Go to Settings → Environment Variables
- [ ] 5.1.3 Delete the old CROSSMINT_CLIENT_API_KEY variable
- [ ] 5.1.4 Add new VITE_CROSSMINT_CLIENT_API_KEY for production
- [ ] 5.1.5 Add VITE_CROSSMINT_CLIENT_API_KEY for preview/staging
- [ ] 5.1.6 Add VITE_CROSSMINT_CLIENT_API_KEY for development (if needed)
- [ ] 5.1.7 Use different API keys for staging vs production
- [ ] 5.1.8 Save all changes

### 5.2 Verify Vercel Configuration
- [ ] 5.2.1 Confirm variable is named VITE_CROSSMINT_CLIENT_API_KEY (with prefix)
- [ ] 5.2.2 Check that value is not empty
- [ ] 5.2.3 Verify it's set for the correct environments
- [ ] 5.2.4 Note: Variable will be accessible during build (not at runtime by Vercel, but by Vite)

---

## 6. Build & Deploy

### 6.1 Git Commit
- [ ] 6.1.1 Stage changes: git add -A
- [ ] 6.1.2 Commit with message: fix: Use VITE_ prefix for Crossmint Client API Key
- [ ] 6.1.3 Commit message should explain Vite environment variable requirement
- [ ] 6.1.4 Verify git status shows all changes committed
- [ ] 6.1.5 Check git log shows the commit

### 6.2 Vercel Deployment
- [ ] 6.2.1 Run: vercel --prod
- [ ] 6.2.2 Verify build succeeds on Vercel
- [ ] 6.2.3 Check build logs for any environment variable warnings
- [ ] 6.2.4 Wait for deployment to complete
- [ ] 6.2.5 Verify alias to https://www.agentrade.xyz

### 6.3 Post-Deployment Verification
- [ ] 6.3.1 Deployment status: SUCCESS
- [ ] 6.3.2 URL: https://www.agentrade.xyz
- [ ] 6.3.3 Build time: ~35 seconds (typical for this project)
- [ ] 6.3.4 No build errors or warnings
- [ ] 6.3.5 Vercel shows "Ready" status

---

## 7. Production Verification

### 7.1 Production Site Smoke Test
- [ ] 7.1.1 Visit https://www.agentrade.xyz
- [ ] 7.1.2 Page loads successfully
- [ ] 7.1.3 Open browser DevTools → Console
- [ ] 7.1.4 Check for error: "API Key not configured" - should NOT appear
- [ ] 7.1.5 Check for any payment-related errors

### 7.2 Payment Feature Test
- [ ] 7.2.1 Click "积分套餐" button in header
- [ ] 7.2.2 PaymentModal should open without errors
- [ ] 7.2.3 No error message: "⚠️ 支付功能暂时不可用"
- [ ] 7.2.4 Verify modal displays package options
- [ ] 7.2.5 Test modal close functionality (should work)

### 7.3 Browser Compatibility Test
- [ ] 7.3.1 Test in Chrome
- [ ] 7.3.2 Test in Firefox
- [ ] 7.3.3 Test in Safari
- [ ] 7.3.4 Test in Edge
- [ ] 7.3.5 All should work without API key errors

### 7.4 Mobile Testing
- [ ] 7.4.1 Test on mobile Safari (iOS)
- [ ] 7.4.2 Test on Chrome mobile (Android)
- [ ] 7.4.3 Payment button should work on mobile
- [ ] 7.4.4 Modal should open properly
- [ ] 7.4.5 No console errors on mobile

### 7.5 Multiple Environment Testing
- [ ] 7.5.1 Test if staging/preview deployment exists
- [ ] 7.5.2 Verify staging uses staging API key
- [ ] 7.5.3 Verify production uses production API key
- [ ] 7.5.4 Ensure no API key leakage between environments
- [ ] 7.5.5 Confirm correct environment-specific behavior

---

## 8. Monitoring & Documentation

### 8.1 Error Tracking Verification
- [ ] 8.1.1 Set up Sentry or error tracking (if available)
- [ ] 8.1.2 Monitor for "API Key not configured" errors
- [ ] 8.1.3 Should see ZERO occurrences after fix
- [ ] 8.1.4 Monitor for any payment-related errors
- [ ] 8.1.5 Document baseline metrics

### 8.2 Documentation Updates
- [ ] 8.2.1 Document environment variable naming in README
- [ ] 8.2.2 Add comment in vite-env.d.ts explaining VITE_CROSSMINT_CLIENT_API_KEY
- [ ] 8.2.3 Update .env.local.example with correct variable name
- [ ] 8.2.4 Document Vercel configuration steps
- [ ] 8.2.5 Create setup guide for new developers

### 8.3 Knowledge Base
- [ ] 8.3.1 Document why VITE_ prefix is required for Vite
- [ ] 8.3.2 Clarify difference between client vs server API keys
- [ ] 8.3.3 Explain Crossmint Client API Key security model
- [ ] 8.3.4 Add troubleshooting guide for similar issues
- [ ] 8.3.5 Create checklist for environment variable setup

---

## Summary

**Total Tasks**: 120+ verification and implementation tasks
**Priority**: Critical (Blocks payment feature)
**Complexity**: Low (Environment variable configuration)
**Risk**: Very Low (No code logic changes, only configuration)
**Estimated Implementation Time**: 15-20 minutes (code changes + testing)
**Estimated Deployment Time**: 10-15 minutes (Vercel build + verification)

