## Why

Users see the error message "⚠️ 支付功能暂时不可用" (Payment feature temporarily unavailable) with the underlying issue: "API Key not configured" in console. The `CROSSMINT_CLIENT_API_KEY` environment variable is properly configured in Vercel, but it's not being exposed to the client-side Vite build. This blocks the entire payment feature.

## Root Cause

**Vite Environment Variable Visibility Problem**:
- Vite automatically filters environment variables at build time
- By default, ONLY variables prefixed with `VITE_` are exposed to client code
- `CROSSMINT_CLIENT_API_KEY` without the `VITE_` prefix is filtered out during build
- Even though the variable exists in Vercel environment variables, `import.meta.env.CROSSMINT_CLIENT_API_KEY` is `undefined` in the browser
- Result: CrossmintService receives empty string and logs warning

**Error Flow**:
1. `CROSSMINT_CLIENT_API_KEY=ck_staging_...` set in Vercel
2. Vite build starts: sees variable without `VITE_` prefix
3. Vite filters it out: security feature to prevent leaking server variables
4. Client code runs: `import.meta.env.CROSSMINT_CLIENT_API_KEY` = `undefined`
5. CrossmintService: `apiKey || undefined || ""` = `""`
6. Warning logged: "API Key not configured"
7. User sees: "Payment feature temporarily unavailable"

## What Changes

This is a Vite build-time environment variable configuration issue that requires:

1. **Rename environment variable** from `CROSSMINT_CLIENT_API_KEY` to `VITE_CROSSMINT_CLIENT_API_KEY`
   - Adds `VITE_` prefix so Vite exposes it to client code
   - Tells Vercel this variable should be available to the browser

2. **Update TypeScript types** in `src/vite-env.d.ts`
   - Add type definition for `VITE_CROSSMINT_CLIENT_API_KEY`
   - Enables TypeScript type checking for the variable

3. **Update all references** in code
   - `CrossmintService.ts`: Read from `VITE_CROSSMINT_CLIENT_API_KEY`
   - `PaymentModal.tsx`: Read from `VITE_CROSSMINT_CLIENT_API_KEY`

4. **Update Vercel environment variables**
   - Change variable name from `CROSSMINT_CLIENT_API_KEY` to `VITE_CROSSMINT_CLIENT_API_KEY`
   - Ensure all environments (production, preview, development) are updated

## Impact

- **Affected specs**: payment-checkout
- **Affected code**:
  - `src/features/payment/services/CrossmintService.ts` - Reads env variable
  - `src/features/payment/components/PaymentModal.tsx` - Reads env variable
  - `src/vite-env.d.ts` - Type definitions
- **Severity**: Critical - Blocks payment feature in production
- **User impact**: Payment button shows error when used
- **Breaking changes**: None - transparently changes how variable is read
- **Risk level**: Very Low - Only fixes environment variable access

## Security Clarification

**Note on Previous Security Fix**:
- Previously changed from `VITE_CROSSMINT_CLIENT_API_KEY` to `CROSSMINT_CLIENT_API_KEY` for "security"
- However, `CROSSMINT_CLIENT_API_KEY` is a **Crossmint Client SDK API Key**, not a server secret
- Client API Keys are by definition public and meant to be used in browsers
- Crossmint architecture requires Client API Key on the client side
- This is different from:
  - Server-side API keys (which should NOT be prefixed with VITE_)
  - Webhook secrets (which should NEVER be in client code)
  - Bearer tokens (which should be server-side only)

**Correct Security Approach**:
- ✅ `VITE_CROSSMINT_CLIENT_API_KEY` - Client API Key (safe to expose)
- ✅ `CROSSMINT_WEBHOOK_SECRET` - Server secret (no VITE_ prefix, server-side only)
- ✅ `VITE_API_URL` - Public API URL (safe to expose)
- ❌ `VITE_DATABASE_PASSWORD` - Server secret (should NOT have VITE_ prefix)

