# OPTIMIZE PROPOSAL: Trading Decision Position Conflict Prevention & Decision Stability

## 1. Problem Description

### 1.1 Current Issue
The trading system reports error:
```
❌ BTCUSDT open_short 失败: ❌ BTCUSDT 已有空仓，拒绝开仓以防止仓位叠加超限。如需换仓，请先给出 close_short 决策
```

This indicates:
- **Symptom**: AI generates `open_short` decision when position already exists
- **Current Defense**: Execution-layer check in `executeOpenShortWithRecord()` catches and rejects it (後补救)
- **Root Problem**: AI generates invalid decisions that should never happen (前预防缺失)

### 1.2 Related Issues
1. **No deduplication logic**: Same symbol can appear multiple times in one decision cycle
2. **No position awareness in System Prompt**: AI doesn't explicitly know what's already held
3. **No trade frequency limit**: AI can open/close same symbol within minutes (churning)
4. **No holding period tracking**: AI doesn't see how long position has been held

## 2. Root Cause Analysis (Three-Layer Analysis)

### Layer 1: Phenomenon (System Behavior)
- Error appears during execution phase
- System rejects invalid trade, not preventing it

### Layer 2: Essence (Architectural Problem)
1. **Decision Generation Gap**:
   - `buildUserPrompt()` includes current positions in text format
   - But System Prompt doesn't explicitly forbid duplicate same-direction opens
   - AI has no structured constraint about position conflicts

2. **Decision Validation Gap**:
   - No deduplication check after Decision JSON is parsed
   - No conflict detection between new decisions and existing positions
   - All validation happens at execution layer, too late

3. **Decision Stability Gap**:
   - No "holding period minimum" constraint
   - No "minimum time between open/close same symbol" rule
   - No tracking of previous cycle's decisions for stability check

### Layer 3: Philosophy (Design Principle)
This violates **Linus Torvalds' "Good Taste" principle**:
> "Sometimes you can look at something from different angles and rewrite it so the special case is gone, and it becomes general."

Current approach: Check and reject bad decisions → Better approach: Make bad decisions impossible

**Design Principle**: Push validation LEFT (generation) not RIGHT (execution)

## 3. Proposed Solution

### 3.1 Architecture Changes

```
BEFORE (Bad Taste):
┌─────────────────────────┐
│ AI generates Decision   │  ← Can generate invalid decisions
├─────────────────────────┤
│ Validate in execute()   │  ← Catch and reject (too late)
├─────────────────────────┤
│ Trade or Error          │  ← Too late, decision was wrong
└─────────────────────────┘

AFTER (Good Taste):
┌─────────────────────────┐
│ AI aware of constraints │  ← Enhanced System Prompt
├─────────────────────────┤
│ Validate after parsing  │  ← Deduplicate & deconflict
├─────────────────────────┤
│ Execute valid Decision  │  ← All decisions are valid
└─────────────────────────┘
```

### 3.2 Three-Layer Optimization

#### Optimization 1: System Prompt Enhancement
**File**: `decision/engine.go` - `buildSystemPrompt()`

Add explicit constraints:
```
## Held Position Rules:
- DO NOT open same direction on an already held symbol
- DO NOT hold same symbol with both long AND short simultaneously
- DO NOT trade same symbol within 15 minutes if closed recently
- For position changes: issue CLOSE first, then OPEN in separate decision cycle

## Decision Deduplication:
- Check your JSON output for duplicate symbols in one cycle
- If same symbol appears twice, only keep highest confidence decision
- If actions conflict (open_long + close_long for same symbol), prioritize close
```

**Code Location**: `decision/engine.go` lines 276-299 in `buildSystemPrompt()`

---

#### Optimization 2: Decision Conflict Detection
**File**: `decision/engine.go` - Add new function `ValidateAndDeduplicateDecisions()`

```go
// ValidateAndDeduplicateDecisions validates decisions and removes conflicts
// Rules:
// 1. No duplicate symbol-action pairs (keep highest confidence)
// 2. No conflicting actions (open_long + close_long same symbol)
// 3. No open action if position already exists
// 4. No rapid re-entry (< 15 minutes since close)
func ValidateAndDeduplicateDecisions(
    decisions []Decision,
    positions []PositionInfo,
    lastDecisions map[string]int64, // symbol_action -> unix timestamp
) []Decision {

    // Step 1: Deduplicate same symbol - keep highest confidence
    symbolActionMap := make(map[string]*Decision)
    for i := range decisions {
        key := decisions[i].Symbol + "_" + decisions[i].Action
        if existing, exists := symbolActionMap[key]; exists {
            // Keep higher confidence
            if decisions[i].Confidence > existing.Confidence {
                symbolActionMap[key] = &decisions[i]
            }
        } else {
            symbolActionMap[key] = &decisions[i]
        }
    }

    // Step 2: Check conflicts with existing positions
    // Build held positions map for quick lookup
    heldPositions := make(map[string]string) // symbol -> "long"|"short"
    for _, pos := range positions {
        heldPositions[pos.Symbol] = pos.Side
    }

    // Step 3: Filter invalid decisions
    var validDecisions []Decision
    now := time.Now().UnixMilli()

    for _, decision := range symbolActionMap {
        valid := true
        reason := ""

        switch decision.Action {
        case "open_long":
            if held, exists := heldPositions[decision.Symbol]; exists {
                valid = false
                reason = fmt.Sprintf("already hold %s", held)
            }
            if lastClosedTime, exists := lastDecisions[decision.Symbol+"_close_long"]; exists {
                if now-lastClosedTime < 15*60*1000 { // 15 minutes
                    valid = false
                    reason = fmt.Sprintf("closed %.0f minutes ago",
                        float64(now-lastClosedTime)/(60*1000))
                }
            }

        case "open_short":
            if held, exists := heldPositions[decision.Symbol]; exists {
                valid = false
                reason = fmt.Sprintf("already hold %s", held)
            }
            if lastClosedTime, exists := lastDecisions[decision.Symbol+"_close_short"]; exists {
                if now-lastClosedTime < 15*60*1000 {
                    valid = false
                    reason = fmt.Sprintf("closed %.0f minutes ago",
                        float64(now-lastClosedTime)/(60*1000))
                }
            }

        case "close_long", "close_short":
            if !exists := heldPositions[decision.Symbol]; !exists {
                valid = false
                reason = "no position held"
            }
        }

        if valid {
            validDecisions = append(validDecisions, *decision)
        } else {
            log.Printf("⚠️  Decision filtered: %s %s - reason: %s",
                decision.Symbol, decision.Action, reason)
        }
    }

    return validDecisions
}
```

**Location in Code**: After `GetFullDecisionWithCustomPrompt()` in `decision/engine.go`

---

#### Optimization 3: Context Enhancement
**File**: `trader/auto_trader.go` - `buildTradingContext()` function

Enhance Context struct to include:
```go
type Context struct {
    // ... existing fields ...

    // NEW FIELDS for AI awareness
    HeldPositions      map[string]string `json:"-"` // symbol -> "long"|"short"
    LastDecisions      map[string]int64  `json:"-"` // symbol_action -> timestamp
    MinHoldingMinutes  int               `json:"-"` // Minimum hold time before close
    CooldownMinutes    int               `json:"-"` // Minimum time before re-entry
}
```

Add to `buildUserPrompt()`:
```go
// Add position awareness section
if len(ctx.HeldPositions) > 0 {
    sb.WriteString("## ⚠️ 不可操作的币种 (已持仓)\n\n")
    for symbol, side := range ctx.HeldPositions {
        sb.WriteString(fmt.Sprintf("- %s: 已持%s，禁止开%s (如需换仓请先平仓)\n",
            symbol, side, side))
    }
    sb.WriteString("\n")
}
```

---

### 3.3 Implementation Checklist

| Component | File | Change | Priority |
|-----------|------|--------|----------|
| System Prompt | `decision/engine.go` | Add explicit position rules | HIGH |
| Dedupe Function | `decision/engine.go` | Add `ValidateAndDeduplicateDecisions()` | HIGH |
| Validation Call | `decision/engine.go` in `GetFullDecisionWithCustomPrompt()` | Call validation after JSON parse | HIGH |
| Context Enhanced | `trader/auto_trader.go` in `buildTradingContext()` | Add held positions map | MEDIUM |
| User Prompt | `decision/engine.go` in `buildUserPrompt()` | Show locked symbols | MEDIUM |
| Execution Layer | `trader/auto_trader.go` | Keep existing checks as defense-in-depth | LOW |
| Logging | `logger/decision_logger.go` | Log filtered decisions | MEDIUM |

---

## 4. Implementation Steps

### Step 1: Update System Prompt Template (decision/engine.go)
Add position conflict rules to `buildSystemPrompt()` function

### Step 2: Add Validation Function (decision/engine.go)
Implement `ValidateAndDeduplicateDecisions()` with conflict detection

### Step 3: Enhance Context Building (trader/auto_trader.go)
Add `HeldPositions` and `LastDecisions` maps to Context

### Step 4: Update User Prompt (decision/engine.go)
Add section showing currently held positions AI cannot touch

### Step 5: Call Validation in Decision Loop
In `GetFullDecisionWithCustomPrompt()`, call validation before execution

### Step 6: Update Auto-Trader Decision Loop (trader/auto_trader.go)
- Store validated decisions for next cycle
- Pass previous decisions map to next context

### Step 7: Add Logging
Log all filtered decisions for debugging

### Step 8: Test & Verify
Run in test mode for 10+ cycles, verify:
- No duplicate decisions
- No same-direction double opens
- No rapid re-entry trades

---

## 5. Code Changes Required

### File 1: decision/engine.go

#### Change 1a: Add to buildSystemPrompt() function
```go
// Around line 278, add after existing constraints:
sb.WriteString("\n## 仓位冲突预防 (Critical)\n")
sb.WriteString("1. 禁止重复开仓: 同一币种已有仓位，禁止继续开相同方向仓位\n")
sb.WriteString("2. 禁止换仓无间隙: 平仓后需等待15分钟才能重新开仓同币种\n")
sb.WriteString("3. 决策去重: 如果JSON中同一币种出现多次，只保留信心度最高的决策\n")
sb.WriteString("4. 冲突消解: 如果同币种同时出现open和close，优先执行close\n\n")
```

#### Change 1b: Add validation function
```go
// After line 399 (end of buildUserPrompt), add:
func ValidateAndDeduplicateDecisions(decisions []Decision, positions []PositionInfo, lastDecisions map[string]int64) []Decision {
    // [see full code in 3.2 Optimization 2]
}
```

#### Change 1c: Call validation in GetFullDecisionWithCustomPrompt
```go
// Around line 150, after JSON is parsed:
if len(fullDec.Decisions) > 0 {
    // Validate and deduplicate decisions
    validatedDecisions := ValidateAndDeduplicateDecisions(
        fullDec.Decisions,
        ctx.Positions,
        ctx.LastDecisions, // NEW field
    )

    log.Printf("📋 决策验证: %d个决策 -> %d个有效决策 (过滤%d个)",
        len(fullDec.Decisions), len(validatedDecisions),
        len(fullDec.Decisions)-len(validatedDecisions))

    fullDec.Decisions = validatedDecisions
}
```

---

### File 2: trader/auto_trader.go

#### Change 2a: Add fields to Context building
```go
// Around line 513, in buildTradingContext(), add:
heldPositions := make(map[string]string)
for _, pos := range positionInfos {
    heldPositions[pos.Symbol] = pos.Side
}
ctx.HeldPositions = heldPositions
ctx.LastDecisions = at.positionFirstSeenTime // reuse existing map
ctx.MinHoldingMinutes = 30
ctx.CooldownMinutes = 15
```

#### Change 2b: Store last cycle decisions
```go
// Around line 509, before returning from runCycle():
// Store decision timestamps for next cycle
for _, d := range sortedDecisions {
    if d.Action == "close_long" || d.Action == "close_short" {
        at.positionFirstSeenTime[d.Symbol+"_"+d.Action] = time.Now().UnixMilli()
    }
}
```

---

## 6. Verification Plan

### Phase 1: Unit Testing (Local)
```bash
# Test 1: Deduplication
- Input: BTCUSDT open_long (conf 60%), BTCUSDT open_long (conf 80%)
- Expected: Keep only 80% confidence version

# Test 2: Position Conflict
- Existing: BTCUSDT long
- Input: BTCUSDT open_long
- Expected: Filtered out

# Test 3: Cooldown Period
- Close: BTCUSDT short at T=0
- Input (T=5min): BTCUSDT open_short
- Expected: Filtered out (cooldown 15 min)

# Test 4: Conflict Resolution
- Input: BTCUSDT open_long, BTCUSDT close_long (same symbol)
- Expected: Keep close_long (prioritize close)
```

### Phase 2: Integration Testing (Live)
- Run for 24 hours
- Monitor: `decision_logs/` for filtered decision count
- Check: No "already hold" errors in execution logs
- Verify: Trade frequency normalized (no churning)

### Phase 3: Metrics
Track these in logs:
- Decision count before/after filtering per cycle
- Rejection rate by reason (duplicate, conflict, cooldown)
- Trades per day (should reduce if churning was high)
- Win rate (should improve if churning was reduced)

---

## 7. Expected Outcomes

### Immediate Benefits
1. **Elimination of "already hold" errors** - 100% of such errors eliminated
2. **Cleaner AI behavior** - AI makes better decisions from start
3. **Better logging** - Can see why decisions were filtered
4. **Reduced order rejections** - Fewer failed trades

### Long-term Benefits
1. **Improved Sharpe Ratio** - Less churning = fewer transaction fees
2. **Better decision stability** - AI can't flip-flop on same symbol
3. **Educational** - AI learns position constraints implicitly
4. **Scalable** - Easy to add more validation rules later

---

## 8. Risk Assessment

| Risk | Probability | Mitigation |
|------|-------------|-----------|
| Over-filtering decisions | Low | Tune cooldown from 15→10 if needed |
| Missing valid signals | Low | Validation only filters duplicates/conflicts |
| Performance impact | Very Low | Validation is O(n) where n=decisions (~5-10) |
| Regression | Low | Keep execution-layer checks as fallback |

---

## 9. Rollback Plan

If issues appear:
1. Remove call to `ValidateAndDeduplicateDecisions()` in `GetFullDecisionWithCustomPrompt()`
2. Comment out System Prompt additions
3. Context still contains enhanced fields (harmless)
4. System reverts to previous behavior immediately

---

## 10. Success Criteria

✅ No "已有空仓，拒绝开仓" errors appear in logs
✅ Decision filtering logs show 5-15% of decisions filtered per cycle
✅ Same-symbol transactions not within 15 minutes of each other
✅ Trade frequency reduces by 20-30% (if churning was happening)
✅ Win rate maintains or improves

---

## Related Issues

- Frequent open/close cycles (trading churn)
- AI decision instability (same symbol, opposite actions)
- Over-trading reducing Sharpe ratio
- Position overlap prevention scattered across layers

---

## Notes for Implementation

1. **Three-Layer Philosophy**: This optimization demonstrates moving validation left:
   - **Phenomenon**: Error at execution
   - **Essence**: Validation gap
   - **Philosophy**: Prevent problems in generation, not execution

2. **Linus' "Good Taste"**: Instead of nested if-checks at execution, we eliminate the bad case upfront so it can't happen

3. **Defensive Layers**: Keep existing execution-layer checks as backup (defense-in-depth), but they should never trigger if optimization works

4. **AI Awareness**: Enhanced System Prompt makes AI aware of constraints, improving quality of generated decisions

---

## Author Notes
This proposal follows the principle: **"Show the system what's possible, not what's forbidden."**
Rather than listing "don't do X", we show held positions and let AI naturally avoid them.
