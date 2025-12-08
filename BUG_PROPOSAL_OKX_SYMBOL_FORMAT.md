# Bug Proposal: OKX Symbol Format Mismatch Causes 4h K-line Fetch Failure

## Issue ID
BUG-2025-1208-002

## Summary
When TopTrader attempts to execute a `close_short` decision for BTC, the operation fails with error:
```
获取4小时K线失败: 获取4h分钟K线失败: OKX API error: Instrument ID, Instrument ID code, or Spread ID doesn't exist.
```

The root cause is a symbol format mismatch between the OKX positions API response and the internal market data system.

## Affected Components
- **trader/okx_trader.go**: `GetPositions()` returns OKX-format symbols
- **market/data.go**: `Normalize()` function doesn't handle OKX format
- **market/api_client.go**: `symbolToOKXInstId()` doesn't handle already-OKX-format symbols

## Environment
- **Backend URL**: https://nofx-gyc567.replit.app
- **User**: gyc567@gmail.com
- **Trader**: TopTrader

## Root Cause Analysis

### Root Cause 1: OKXTrader.GetPositions() Returns OKX Format (PRIMARY)
**Location**: `trader/okx_trader.go`, line 201

The `GetPositions()` function returns positions with `symbol` field in OKX format (e.g., "BTC-USDT-SWAP") instead of internal format (e.g., "BTCUSDT"):

```go
// BEFORE (incorrect)
"symbol": pos["instId"],  // Returns "BTC-USDT-SWAP"
```

When AutoTrader uses this symbol for market data lookup:
1. `market.Get("BTC-USDT-SWAP")` is called
2. `Normalize("BTC-USDT-SWAP")` returns "BTC-USDT-SWAPUSDT" (incorrect!)
3. K-line lookup fails because "BTC-USDT-SWAPUSDT" doesn't exist in cache
4. Falls back to API call with malformed instrument ID
5. OKX returns "Instrument ID doesn't exist"

### Root Cause 2: market.Normalize() Doesn't Handle OKX Format
**Location**: `market/data.go`, line 454-460

The original `Normalize()` function only checks for "USDT" suffix:

```go
// BEFORE (incorrect)
func Normalize(symbol string) string {
    symbol = strings.ToUpper(symbol)
    if strings.HasSuffix(symbol, "USDT") {
        return symbol
    }
    return symbol + "USDT"
}
```

"BTC-USDT-SWAP" doesn't end with "USDT", so it becomes "BTC-USDT-SWAPUSDT".

### Root Cause 3: symbolToOKXInstId() Doesn't Handle OKX Format
**Location**: `market/api_client.go`, line 80-84

The original function blindly strips "USDT" suffix:

```go
// BEFORE (incorrect)
func symbolToOKXInstId(symbol string) string {
    symbol = strings.ToUpper(symbol)
    symbol = strings.TrimSuffix(symbol, "USDT")
    return symbol + "-USDT-SWAP"
}
```

This produces invalid results for already-OKX-format symbols.

## Solution Applied

### Fix 1: OKXTrader.GetPositions() - Convert to Internal Format (PRIMARY)
**File**: `trader/okx_trader.go`

```go
// AFTER (correct)
okxInstId, _ := pos["instId"].(string)
internalSymbol := convertFromOKXSymbol(okxInstId)  // BTC-USDT-SWAP -> BTCUSDT

standardizedPos := map[string]interface{}{
    "symbol":    internalSymbol,  // Now returns "BTCUSDT"
    "okxInstId": okxInstId,       // Preserve original for debugging
    // ... other fields
}
```

**Downstream Compatibility**: The `CloseLong`/`CloseShort` functions use:
```go
if (posSymbol == okxSymbol || convertToOKXSymbol(posSymbol) == okxSymbol) && pos["posSide"] == "long"
```
This comparison handles both formats:
- `posSymbol` = "BTCUSDT" (from GetPositions)
- `okxSymbol` = "BTC-USDT-SWAP" (from convertToOKXSymbol(decision.Symbol))
- `convertToOKXSymbol("BTCUSDT")` = "BTC-USDT-SWAP" = `okxSymbol` ✓

### Fix 2: market.Normalize() - Handle OKX Format (Fallback)
**File**: `market/data.go`

Only handles `-USDT-SWAP` suffix specifically to avoid mangling other symbols like `BTC-USDC`:

```go
// AFTER (correct)
func Normalize(symbol string) string {
    symbol = strings.ToUpper(symbol)

    // Only handle OKX USDT perpetual format
    if strings.HasSuffix(symbol, "-USDT-SWAP") {
        symbol = strings.TrimSuffix(symbol, "-USDT-SWAP")
        return symbol + "USDT"
    }

    if strings.HasSuffix(symbol, "USDT") {
        return symbol
    }
    return symbol + "USDT"
}
```

### Fix 3: symbolToOKXInstId() - Handle Already-OKX-Format (Fallback)
**File**: `market/api_client.go`

Only skips conversion if already in `-USDT-SWAP` format:

```go
// AFTER (correct)
func symbolToOKXInstId(symbol string) string {
    symbol = strings.ToUpper(symbol)

    // If already OKX perpetual format, return as-is
    if strings.HasSuffix(symbol, "-USDT-SWAP") {
        return symbol
    }

    // Convert from Binance format
    symbol = strings.TrimSuffix(symbol, "USDT")
    return symbol + "-USDT-SWAP"
}
```

## Files Changed
1. `trader/okx_trader.go` - GetPositions() now returns internal symbol format
2. `market/data.go` - Normalize() handles OKX format as fallback
3. `market/api_client.go` - symbolToOKXInstId() handles already-OKX-format symbols

## Data Flow After Fix

```
1. OKX API returns: instId = "BTC-USDT-SWAP"
2. GetPositions() converts: symbol = "BTCUSDT" (internal format)
3. AutoTrader receives: decision.Symbol = "BTCUSDT"
4. market.Get("BTCUSDT") finds cached K-line data
5. Trading decision executes successfully
```

## Testing Checklist
- [x] Backend compiles successfully
- [x] Backend starts without errors
- [x] K-line data loads for all symbols (BTCUSDT, ETHUSDT, etc.)
- [ ] close_short decision executes without K-line fetch errors
- [ ] Deployed to production

## Security Considerations
- No security issues introduced
- Symbol format conversion is deterministic and reversible

## Related Issues
- BUG_PROPOSAL_MODELS_API_401.md - Previous authentication fix

## Status
- [x] Root causes identified (3 issues)
- [x] All fixes implemented
- [x] Backend compiled
- [x] Backend tested locally
- [ ] Deployed to production
- [ ] Verified with user gyc567@gmail.com

## Date
December 8, 2025

## Author
Replit Agent
