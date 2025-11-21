# Tasks: Fix CORS Policy Error

## Implementation Checklist

### Phase 1: Backend CORS Fix
- [x] Create proposal and identify root cause
- [x] Modify api/server.go CORS middleware
- [x] Fix compilation errors (remove auth.IsAdminMode calls)
- [x] Move test files to avoid main function conflicts
- [x] Verify code compiles successfully
- [x] Commit and push changes to GitHub (commit: 3fc4a09)
- [x] Create deployment scripts and documentation
- [ ] Deploy updated backend to Replit (MANUAL STEP REQUIRED)
- [ ] Verify backend is running with new CORS config (PENDING DEPLOYMENT)

### Phase 2: Frontend Testing
- [ ] Test API endpoints from frontend (browser)
- [ ] Check Network tab for successful requests (200 OK)
- [ ] Verify TopTrader displays real values:
  - [ ] Total Equity: 99.88 USDT (not 0.00)
  - [ ] Available Balance: 99.88 USDT (not 0.00)
  - [ ] Total P&L: -0.12 USDT (not 0.00)
  - [ ] Position Count: 0 (correct)
- [ ] Confirm no CORS errors in browser console

### Phase 3: Validation & Cleanup
- [ ] Remove debug logs from CompetitionPage.tsx
- [ ] Deploy final version without debug code
- [ ] Document final state and results
- [ ] Archive this change proposal

## Implementation Details

### Status: Code Ready, Awaiting Manual Replit Deployment

**Recent Progress**:
- ✅ Fixed compilation errors in api/server.go and main.go
- ✅ Removed undefined auth.IsAdminMode() and auth.SetAdminMode() calls
- ✅ Successfully compiled backend locally
- ✅ Code pushed to GitHub (commit: 3fc4a09)

**Current Issue**: Replit has not automatically deployed despite GitHub push.

**Evidence**:
```bash
$ curl -I -X OPTIONS https://nofx-gyc567.replit.app/api/competition
access-control-allow-headers: Content-Type, Authorization
# Should include: Cache-Control, X-Requested-With, etc.
```

**Required Action**: Manually restart the Replit backend service via web interface.

### Backend Change (api/server.go)
```go
// Line 56 - Update CORS headers
c.Writer.Header().Set("Access-Control-Allow-Headers",
    "Content-Type, Authorization, Cache-Control, X-Requested-With, X-Requested-By, If-Modified-Since, Pragma");
```

### Test Commands
```bash
# Test CORS preflight
curl -X OPTIONS https://nofx-gyc567.replit.app/api/competition \
  -H "Origin: https://web-pink-omega-40.vercel.app" \
  -H "Access-Control-Request-Method: GET" \
  -H "Access-Control-Request-Headers: cache-control"

# Should return Access-Control-Allow-Headers including "cache-control"
```

### Browser Testing Checklist
- [ ] Open https://web-pink-omega-40.vercel.app
- [ ] Open DevTools → Console (no CORS errors)
- [ ] Open DevTools → Network (requests succeed)
- [ ] Check TopTrader values (real numbers not zeros)

## Success Criteria

Final state should show:
- ✅ No CORS errors in console
- ✅ API requests return 200 OK
- ✅ TopTrader displays 99.88 USDT (not 0.00)
- ✅ All dashboard metrics populated correctly
- ✅ Debug logs removed (clean production code)

## Files Modified

1. **api/server.go** - Update CORS middleware headers
2. **web/src/components/CompetitionPage.tsx** - Remove debug logs (later)
