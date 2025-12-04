# Bug Proposal: Credits API 401 Authentication Error

## OpenSpec Format

### Summary
The credits API (`/api/user/credits`) returns 401 "用户未认证" (User not authenticated) error even when the user is properly authenticated through the authMiddleware.

### Root Cause Analysis
**Context Key Mismatch**: The `getUserID()` function in `api/credits/handler.go` looks for a context key `"userID"`, but the `authMiddleware` in `api/server.go` sets the user ID using the key `"user_id"`.

#### Evidence

1. **api/server.go authMiddleware** (lines 1420 and 1473):
   ```go
   c.Set("user_id", "admin")  // Admin mode
   c.Set("user_id", claims.UserID)  // Normal JWT auth
   ```

2. **api/credits/handler.go getUserID** (line 643):
   ```go
   if userID, exists := c.Get("userID"); exists {  // WRONG KEY!
   ```

3. **Result**: `getUserID()` always returns empty string, causing 401 error.

### Affected Endpoints
- `GET /api/user/credits` - User credits balance
- `GET /api/user/credits/transactions` - User transaction history  
- `GET /api/user/credits/summary` - User credit summary

### Fix Applied
Modified `getUserID()` function to check for the correct `"user_id"` key first, with fallback to `"userID"` for compatibility:

```go
func getUserID(c *gin.Context) string {
    // Check for the key used by authMiddleware
    if userID, exists := c.Get("user_id"); exists {
        if id, ok := userID.(string); ok {
            return id
        }
    }
    // Fallback for legacy compatibility
    if userID, exists := c.Get("userID"); exists {
        if id, ok := userID.(string); ok {
            return id
        }
    }
    return ""
}
```

### Testing
1. Build the backend: `go build -o nofx-backend main.go`
2. Start the server
3. Test endpoint: `curl http://localhost:8080/api/user/credits`
4. Expected: Should return user credits or admin user credits (when admin_mode=true)

### Impact
- **Critical**: Blocks all credit-related functionality for authenticated users
- **Production**: https://nofx-gyc567.replit.app affected
- **Frontend**: www.agentrade.xyz credit features broken

### Prevention
- Establish consistent naming convention for context keys (prefer `snake_case`)
- Add integration tests that verify context key consistency across middleware and handlers
- Consider using constants for context keys instead of string literals

### Related Files
- `api/credits/handler.go` - Contains the fixed `getUserID()` function
- `api/server.go` - Contains the `authMiddleware` that sets `"user_id"`

### Status
- [x] Root cause identified
- [x] Fix applied to codebase
- [ ] Backend recompiled
- [ ] Production deployed
- [ ] Verified in production

### Date
December 4, 2025
