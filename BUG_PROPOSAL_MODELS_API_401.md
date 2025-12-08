# Bug Proposal: /api/models API Returns Wrong User's Model Configs

## Issue ID
BUG-2025-1208-001

## Summary
When user gyc567@gmail.com attempts to start TopTrader from https://www.agentrade.xyz/traders, the frontend fails with "获取模型配置失败" (Failed to load model configs). The root cause involves two issues: JWT token validation fallback logic and PostgreSQL prepared statement caching problems.

## Affected Components
- **Backend**: `api/server.go` - authMiddleware function
- **Backend**: `config/database.go` - PostgreSQL connection configuration
- **Frontend**: https://www.agentrade.xyz/traders
- **API Endpoint**: `GET /api/models`

## Environment
- **Backend URL**: https://nofx-gyc567.replit.app
- **Frontend URL**: https://www.agentrade.xyz
- **Database**: Neon PostgreSQL (serverless)
- **User**: gyc567@gmail.com (ID: 68003b68-2f1d-4618-8124-e93e4a86200a)

## Reproduction Steps
1. Login as gyc567@gmail.com on https://www.agentrade.xyz
2. Navigate to /traders page
3. Click "Start" button on TopTrader
4. Observe error: "获取模型配置失败" (Failed to load model configs)

## Root Cause Analysis

### Issue 1: authMiddleware JWT Fallback Logic (PRIMARY)
**Location**: `api/server.go`, lines 1437-1453

When `admin_mode=true` and JWT token validation fails, the middleware incorrectly falls back to using "admin" user instead of returning a 401 error:

```go
// BEFORE (incorrect behavior)
claims, err := auth.ValidateJWT(tokenParts[1])
if err != nil {
    if isAdminMode {
        // Fallback to admin user - THIS IS THE BUG
        c.Set("user_id", "admin")
        c.Next()
        return
    }
}
```

**Impact**: User requests with expired/invalid tokens are processed as "admin" user, returning admin's model configs instead of user's own configs.

### Issue 2: PostgreSQL Prepared Statement Caching
**Location**: `config/database.go`, lines 25-45

Neon PostgreSQL uses connection pooling similar to PgBouncer. The Go `pq` driver's anonymous prepared statements conflict with this architecture, causing:
```
pq: bind message supplies 4 parameters, but prepared statement "" requires 1
```

## Solution Applied

### Fix 1: Strict JWT Validation
Modified `authMiddleware` to return 401 error when JWT validation fails, even in admin_mode:

```go
// AFTER (correct behavior)
claims, err := auth.ValidateJWT(tokenParts[1])
if err != nil {
    log.Printf("⚠️ JWT验证失败: %v (token前20字符: %s...)", err, tokenParts[1][:min(20, len(tokenParts[1]))])
    // Always return 401 when token is provided but invalid
    c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的token: " + err.Error()})
    c.Abort()
    return
}
```

**Rationale**: 
- If no token is provided and admin_mode=true, fallback to admin user (for testing)
- If token IS provided but invalid, return 401 error (don't silently use wrong user)

### Fix 2: Binary Parameters for PostgreSQL
Added `binary_parameters=yes` to database connection URL:

```go
if strings.Contains(databaseURL, "?") {
    databaseURL += "&binary_parameters=yes"
} else {
    databaseURL += "?binary_parameters=yes"
}
```

**Rationale**: This forces the pq driver to send parameters in binary format and skip the prepare step, avoiding connection pool conflicts.

## Files Changed
1. `api/server.go` - authMiddleware JWT validation logic
2. `config/database.go` - PostgreSQL connection configuration

## Testing
1. Invalid token now returns 401:
```bash
curl -H "Authorization: Bearer invalid-token" "http://localhost:8080/api/models"
# Response: {"error":"无效的token: token is malformed..."}
# Status: 401
```

2. No token in admin_mode still works:
```bash
curl "http://localhost:8080/api/models"
# Response: Admin's model configs (expected for admin_mode testing)
```

3. Database initialization succeeds without prepared statement errors

## Security Considerations
- The fix ensures users cannot accidentally access other users' data due to token validation failures
- Admin mode fallback only applies when NO token is provided (explicit testing scenario)
- Detailed JWT error logging helps diagnose authentication issues

## Related Issues
- Previous fix: BUG_PROPOSAL_CREDITS_API_401.md (context key mismatch)

## Status
- [x] Root cause identified
- [x] Fix implemented
- [x] Backend compiled
- [x] API tested locally
- [ ] Deployed to production
- [ ] Verified with user gyc567@gmail.com

## Date
December 8, 2025

## Author
Replit Agent
