# Fix Binance Exchange Input Fields Not Displaying

## Why
When users select Binance as their exchange in the AI Traders page, the API Key and Secret Key input fields do not appear, preventing users from configuring their Binance exchange connection.

Backend returns `type: "binance"` for Binance exchange, but frontend conditional rendering checks for `type === "cex"`, causing the input fields to never display.

## What Changes
- Fix conditional rendering logic in `ExchangeConfigModal` component (web/src/components/AITradersPage.tsx:1371)
- Update condition from `selectedExchange.type === 'cex'` to include Binance: `(selectedExchange.id === 'binance' || selectedExchange.type === 'cex')`
- Align actual rendering logic with existing debug logging logic (line 1254)

## Impact
- **Affected code**: `web/src/components/AITradersPage.tsx` (ExchangeConfigModal component, line 1371)
- **Affected specs**: `exchange-configuration` (CEX exchange input field rendering)
- **User impact**: Users can now successfully add Binance exchange credentials
- **Breaking changes**: None - this restores intended behavior

## Root Cause Analysis
- Backend API `/api/supported-exchanges` returns Binance with `type: "binance"`
- Frontend has correct logic in debug logs (line 1254) but incorrect logic in actual rendering (line 1371)
- Debug code: `(selectedExchange?.id === 'binance' || selectedExchange?.type === 'cex')`
- Rendering code: `(selectedExchange.type === 'cex')` ❌
- This discrepancy indicates the debug code was added to troubleshoot similar issues but the fix was not applied to the actual rendering logic

## Testing
- Manual verification: Select Binance exchange and confirm API Key and Secret Key fields appear
- Verify other CEX exchanges (OKX) still display correctly
- Verify Hyperliquid and Aster continue to show their specialized fields
