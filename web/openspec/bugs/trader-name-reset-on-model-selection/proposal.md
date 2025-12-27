# Bug Proposal: Trader Name Reset on AI Model Selection

## Bug ID
`trader-name-reset-on-model-selection`

## Severity
**HIGH** - Data Loss, UX Breaking

## Summary
When creating a new trader, if the user enters a trader name and then selects a different AI model from the "AI Model" dropdown, the trader name input field is immediately cleared/reset to empty. This forces the user to re-enter the name, breaking the creation workflow.

## Steps to Reproduce
1. Click "创建交易员" (Create Trader) button
2. Enter trader name in the "交易员名称" (Trader Name) field
3. Open the "AI模型" (AI Model) dropdown
4. Select a different AI model
5. **Expected**: Trader name remains in the input field
6. **Actual**: Trader name is cleared/reset to empty string

## Root Cause Analysis

### Three Identified Root Causes:

#### Cause 1: Unstable Props References in useEffect Dependencies
**File**: `web/src/components/TraderConfigModal.tsx:73-106`

The useEffect hook depends on `availableModels` and `availableExchanges` which are array objects passed from parent component. When parent updates model configuration:
1. `AITradersPage:398-399` calls `api.updateModelConfigs()` then refetches models
2. `setAllModels(refreshedModels)` creates a new array reference
3. `enabledModels` is recomputed (line 115): `allModels?.filter(m => m.enabled && m.apiKey) || []`
4. Parent component passes new `availableModels` reference to TraderConfigModal
5. useEffect triggers due to dependency change
6. Code path `else if (!isEditMode)` (line 81) executes, resetting `trader_name` to empty string

**Impact**: Every time user selects an AI model (which causes parent to refetch models), the form reinitializes.

---

#### Cause 2: Lack of Input Protection in Initialization Logic
**File**: `web/src/components/TraderConfigModal.tsx:81-98`

The initialization code unconditionally resets all formData fields when dependencies change, without checking if user has already provided input:

```typescript
} else if (!isEditMode) {
  setFormData({
    trader_name: '',  // Blindly resets to empty
    ai_model: availableModels[0]?.id || '',
    exchange_id: availableExchanges[0]?.id || '',
    // ... other fields
  });
}
```

There's no protection like checking `if (formData.trader_name === '')` before resetting.

**Impact**: User input is destructively overwritten whenever dependencies change.

---

#### Cause 3: Multiple useEffect Hooks Modifying Same State
**File**: `web/src/components/TraderConfigModal.tsx:144-148`

Multiple useEffect hooks are updating the same `formData` state:
- useEffect at line 73 (depends on traderData, isEditMode, availableModels, availableExchanges)
- useEffect at line 145 (depends on selectedCoins)

When model selection triggers the first useEffect to reset formData, the second useEffect might also run, causing state race conditions and unpredictable behavior.

**Impact**: State synchronization becomes fragile; hard to predict which update wins.

---

## Solution Strategy

### Primary Fix: Remove Unstable Props from Dependencies
Remove `availableModels` and `availableExchanges` from the useEffect dependency array. These should only initialize the form once when modal opens, not every time parent props change.

### Secondary Fix: Initialize with Stability
- Use `isOpen` as explicit dependency instead of relying on parent's dynamic arrays
- Move first available model/exchange selection logic into a separate, stable reference
- Only initialize form when `traderData` is null AND `isOpen` is true

### Tertiary Fix: Consolidate State Updates
Ensure all form state updates go through a single initialization point during modal open.

## Files to Modify
- `web/src/components/TraderConfigModal.tsx` - Fix useEffect dependencies and initialization logic

## Testing Requirements
1. Open "Create Trader" modal
2. Enter trader name
3. Select different AI model from dropdown
4. Verify trader name is preserved (not cleared)
5. Verify AI model selection still works correctly
6. Verify form initializes correctly when opening modal fresh

## Impact Assessment
- **Breaking Changes**: None
- **API Changes**: None
- **Data Model Changes**: None
- **Performance Impact**: Positive (fewer unnecessary re-initializations)
