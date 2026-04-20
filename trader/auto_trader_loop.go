package trader

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/kernel"
	"nofx/logger"
	"nofx/store"
	"nofx/wallet"
	"strings"
	"time"
)

// runCycle 是自动交易主循环里最关键的一步：
// 它把账户、持仓、行情、策略配置收敛成一个完整决策周期，
// 再把 AI 输出转成排序后的动作并交给执行链处理。
func (at *AutoTrader) runCycle() error {
	at.callCount++

	logger.Info("\n" + strings.Repeat("=", 70) + "\n")
	logger.Infof("⏰ %s - AI decision cycle #%d", time.Now().Format("2006-01-02 15:04:05"), at.callCount)
	logger.Info(strings.Repeat("=", 70))

	// 0. Check if trader is stopped (early exit to prevent trades after Stop() is called)
	at.isRunningMutex.RLock()
	running := at.isRunning
	at.isRunningMutex.RUnlock()
	if !running {
		logger.Infof("⏹ Trader is stopped, aborting cycle #%d", at.callCount)
		return nil
	}

	// Check USDC balance periodically for claw402 users (every 10 cycles)
	if at.callCount%10 == 0 && store.IsClaw402Config(at.config.AIModel) {
		at.checkClaw402Balance()
	}

	// Create decision record
	record := &store.DecisionRecord{
		ExecutionLog:   []string{},
		Success:        true,
		AllowAIClose:   at.GetAllowAIClose(),
		AIDecisionMode: at.GetAIDecisionMode(),
		ReviewContext: map[string]interface{}{
			"safe_mode":        at.safeMode,
			"safe_mode_reason": at.safeModeReason,
			"allow_ai_close":   at.GetAllowAIClose(),
			"ai_decision_mode": at.GetAIDecisionMode(),
		},
	}

	// Populate protection snapshot from strategy config
	if at.config.StrategyConfig != nil {
		protCfg := at.config.StrategyConfig.Protection
		ps := &store.ProtectionSnapshot{}
		hasProtection := false

		if protCfg.FullTPSL.Enabled {
			hasProtection = true
			ps.FullTPSL = &store.ProtectionSnapshotFullTPSL{
				Enabled: true,
				Mode:    string(protCfg.FullTPSL.Mode),
				TakeProfit: store.ProtectionSnapshotValueSource{
					Mode:  string(protCfg.FullTPSL.TakeProfit.Mode),
					Value: protCfg.FullTPSL.TakeProfit.Value,
				},
				StopLoss: store.ProtectionSnapshotValueSource{
					Mode:  string(protCfg.FullTPSL.StopLoss.Mode),
					Value: protCfg.FullTPSL.StopLoss.Value,
				},
				FallbackMaxLoss: store.ProtectionSnapshotValueSource{
					Mode:  string(protCfg.FullTPSL.FallbackMaxLoss.Mode),
					Value: protCfg.FullTPSL.FallbackMaxLoss.Value,
				},
			}
		}

		if protCfg.LadderTPSL.Enabled {
			hasProtection = true
			ladder := &store.ProtectionSnapshotLadder{
				Enabled:           true,
				Mode:              string(protCfg.LadderTPSL.Mode),
				TakeProfitEnabled: protCfg.LadderTPSL.TakeProfitEnabled,
				StopLossEnabled:   protCfg.LadderTPSL.StopLossEnabled,
				TakeProfitPrice: store.ProtectionSnapshotValueSource{
					Mode:  string(protCfg.LadderTPSL.TakeProfitPrice.Mode),
					Value: protCfg.LadderTPSL.TakeProfitPrice.Value,
				},
				TakeProfitSize: store.ProtectionSnapshotValueSource{
					Mode:  string(protCfg.LadderTPSL.TakeProfitSize.Mode),
					Value: protCfg.LadderTPSL.TakeProfitSize.Value,
				},
				StopLossPrice: store.ProtectionSnapshotValueSource{
					Mode:  string(protCfg.LadderTPSL.StopLossPrice.Mode),
					Value: protCfg.LadderTPSL.StopLossPrice.Value,
				},
				StopLossSize: store.ProtectionSnapshotValueSource{
					Mode:  string(protCfg.LadderTPSL.StopLossSize.Mode),
					Value: protCfg.LadderTPSL.StopLossSize.Value,
				},
				FallbackMaxLoss: store.ProtectionSnapshotValueSource{
					Mode:  string(protCfg.LadderTPSL.FallbackMaxLoss.Mode),
					Value: protCfg.LadderTPSL.FallbackMaxLoss.Value,
				},
			}
			for _, r := range protCfg.LadderTPSL.Rules {
				ladder.Rules = append(ladder.Rules, store.ProtectionSnapshotLadderRule{
					TakeProfitPct:           r.TakeProfitPct,
					TakeProfitCloseRatioPct: r.TakeProfitCloseRatioPct,
					StopLossPct:             r.StopLossPct,
					StopLossCloseRatioPct:   r.StopLossCloseRatioPct,
				})
			}
			ps.LadderTPSL = ladder
		}

		if protCfg.DrawdownTakeProfit.Enabled && len(protCfg.DrawdownTakeProfit.Rules) > 0 {
			hasProtection = true
			drawdownSource := "strategy"
			for _, r := range protCfg.DrawdownTakeProfit.Rules {
				ps.Drawdown = append(ps.Drawdown, store.ProtectionSnapshotDrawdown{
					Mode:           string(protCfg.DrawdownTakeProfit.Mode),
					Source:         drawdownSource,
					MinProfitPct:   r.MinProfitPct,
					MaxDrawdownPct: r.MaxDrawdownPct,
					CloseRatioPct:  r.CloseRatioPct,
					PollIntervalS:  r.PollIntervalSeconds,
				})
			}
		}

		if protCfg.BreakEvenStop.Enabled {
			hasProtection = true
			ps.BreakEven = &store.ProtectionSnapshotBreakEven{
				Enabled:      true,
				Source:       at.getBreakEvenConfigSource("", ""),
				TriggerMode:  string(protCfg.BreakEvenStop.TriggerMode),
				TriggerValue: protCfg.BreakEvenStop.TriggerValue,
				OffsetPct:    protCfg.BreakEvenStop.OffsetPct,
			}
		}

		if hasProtection {
			record.ProtectionSnapshot = ps
		}
	}

	// 1. Check if trading needs to be stopped
	if time.Now().Before(at.stopUntil) {
		remaining := at.stopUntil.Sub(time.Now())
		logger.Infof("⏸ Risk control: Trading paused, remaining %.0f minutes", remaining.Minutes())
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("Risk control paused, remaining %.0f minutes", remaining.Minutes())
		at.saveDecision(record)
		return nil
	}

	// 2. Reset daily P&L (reset every day)
	if time.Since(at.lastResetTime) > 24*time.Hour {
		at.dailyPnL = 0
		at.lastResetTime = time.Now()
		logger.Info("📅 Daily P&L reset")
	}

	// 4. Collect trading context
	ctx, err := at.buildTradingContext()
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("Failed to build trading context: %v", err)
		at.saveDecision(record)
		return fmt.Errorf("failed to build trading context: %w", err)
	}

	// Save equity snapshot independently (decoupled from AI decision, used for drawing profit curve)
	// NOTE: Must be called BEFORE candidate coins check to ensure equity is always recorded
	at.saveEquitySnapshot(ctx)
	if record.ReviewContext == nil {
		record.ReviewContext = map[string]interface{}{}
	}
	record.ReviewContext["candidate_count"] = len(ctx.CandidateCoins)
	record.ReviewContext["position_count"] = ctx.Account.PositionCount
	record.ReviewContext["total_equity"] = ctx.Account.TotalEquity
	record.ReviewContext["available_balance"] = ctx.Account.AvailableBalance
	record.ReviewContext["margin_used_pct"] = ctx.Account.MarginUsedPct

	// If no candidate coins available, log but do not error
	if len(ctx.CandidateCoins) == 0 {
		logger.Infof("ℹ️  No candidate coins available, skipping this cycle")
		record.Success = true // Not an error, just no candidate coins
		record.ExecutionLog = append(record.ExecutionLog, "No candidate coins available, cycle skipped")
		record.AccountState = store.AccountSnapshot{
			TotalBalance:          ctx.Account.TotalEquity,
			AvailableBalance:      ctx.Account.AvailableBalance,
			TotalUnrealizedProfit: ctx.Account.UnrealizedPnL,
			PositionCount:         ctx.Account.PositionCount,
			InitialBalance:        at.initialBalance,
		}
		at.saveDecision(record)
		return nil
	}

	logger.Info(strings.Repeat("=", 70))
	for _, coin := range ctx.CandidateCoins {
		record.CandidateCoins = append(record.CandidateCoins, coin.Symbol)
	}

	logger.Infof("📊 Account equity: %.2f USDT | Available: %.2f USDT | Positions: %d",
		ctx.Account.TotalEquity, ctx.Account.AvailableBalance, ctx.Account.PositionCount)

	// 5. Use strategy engine to call AI for decision
	logger.Infof("🤖 Requesting AI analysis and decision... [Strategy Engine]")
	decisionVariant := at.GetAIDecisionMode()
	if !at.GetAllowAIClose() {
		decisionVariant = decisionVariant + "|no_close"
	}
	aiDecision, err := kernel.GetFullDecisionWithStrategy(ctx, at.mcpClient, at.strategyEngine, decisionVariant)

	if aiDecision != nil && aiDecision.AIRequestDurationMs > 0 {
		record.AIRequestDurationMs = aiDecision.AIRequestDurationMs
		logger.Infof("⏱️ AI call duration: %.2f seconds", float64(record.AIRequestDurationMs)/1000)
		record.ExecutionLog = append(record.ExecutionLog,
			fmt.Sprintf("AI call duration: %d ms", record.AIRequestDurationMs))
	}

	// Save chain of thought, decisions, and input prompt even if there's an error (for debugging)
	if aiDecision != nil {
		record.SystemPrompt = aiDecision.SystemPrompt // Save system prompt
		record.InputPrompt = aiDecision.UserPrompt
		record.CoTTrace = aiDecision.CoTTrace
		record.RawResponse = aiDecision.RawResponse // Save raw AI response for debugging
		if aiDecision.ParseFallback {
			fallbackMsg := "AI decision parser used safe fallback"
			if aiDecision.ParseFallbackReason != "" {
				fallbackMsg = fmt.Sprintf("AI decision parser used safe fallback: %s", aiDecision.ParseFallbackReason)
			}
			record.ExecutionLog = append(record.ExecutionLog, fallbackMsg)
			if record.ReviewContext == nil {
				record.ReviewContext = map[string]interface{}{}
			}
			record.ReviewContext["parse_fallback"] = true
			record.ReviewContext["parse_fallback_reason"] = aiDecision.ParseFallbackReason
		}
		if len(aiDecision.Decisions) > 0 {
			decisionJSON, _ := json.MarshalIndent(aiDecision.Decisions, "", "  ")
			record.DecisionJSON = string(decisionJSON)
		}
	}

	// Record AI charge (track cost regardless of decision outcome)
	if aiDecision != nil && at.store != nil {
		if chargeErr := at.store.AICharge().Record(at.id, at.aiModel, at.config.AIModel); chargeErr != nil {
			logger.Warnf("⚠️ Failed to record AI charge: %v", chargeErr)
		}
	}

	if err != nil {
		at.consecutiveAIFailures++
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("Failed to get AI decision: %v", err)

		// Activate safe mode after 3 consecutive failures
		if at.consecutiveAIFailures >= 3 && !at.safeMode {
			at.safeMode = true
			at.safeModeReason = fmt.Sprintf("AI failed %d consecutive times: %v", at.consecutiveAIFailures, err)
			logger.Errorf("🛡️ [%s] SAFE MODE ACTIVATED — AI failed %d times in a row. No new positions will be opened. Existing positions are protected with current stop-loss settings.",
				at.name, at.consecutiveAIFailures)
			logger.Errorf("🛡️ [%s] Reason: %v", at.name, err)
			logger.Errorf("🛡️ [%s] Action: Will keep trying AI each cycle. Safe mode auto-deactivates when AI recovers.", at.name)
		}

		// Print system prompt and AI chain of thought (output even with errors for debugging)
		if aiDecision != nil {
			logger.Info("\n" + strings.Repeat("=", 70) + "\n")
			logger.Infof("📋 System prompt (error case)")
			logger.Info(strings.Repeat("=", 70))
			logger.Info(aiDecision.SystemPrompt)
			logger.Info(strings.Repeat("=", 70))

			if aiDecision.CoTTrace != "" {
				logger.Info("\n" + strings.Repeat("-", 70) + "\n")
				logger.Info("💭 AI chain of thought analysis (error case):")
				logger.Info(strings.Repeat("-", 70))
				logger.Info(aiDecision.CoTTrace)
				logger.Info(strings.Repeat("-", 70))
			}
		}

		at.saveDecision(record)

		// In safe mode, don't return error — keep the loop running to retry next cycle
		if at.safeMode {
			logger.Warnf("🛡️ [%s] Safe mode: skipping this cycle, will retry in %v", at.name, at.config.ScanInterval)
			return nil
		}

		return fmt.Errorf("failed to get AI decision: %w", err)
	}

	// AI succeeded — reset failure counter and deactivate safe mode
	if at.consecutiveAIFailures > 0 {
		logger.Infof("✅ [%s] AI recovered after %d consecutive failures", at.name, at.consecutiveAIFailures)
	}
	at.consecutiveAIFailures = 0
	if at.safeMode {
		logger.Infof("🛡️ [%s] SAFE MODE DEACTIVATED — AI is working again. Resuming normal trading.", at.name)
		at.safeMode = false
		at.safeModeReason = ""
	}

	// // 5. Print system prompt
	// logger.Infof("\n" + strings.Repeat("=", 70))
	// logger.Infof("📋 System prompt [template: %s]", at.systemPromptTemplate)
	// logger.Info(strings.Repeat("=", 70))
	// logger.Info(decision.SystemPrompt)
	// logger.Infof(strings.Repeat("=", 70) + "\n")

	// 6. Print AI chain of thought
	// logger.Infof("\n" + strings.Repeat("-", 70))
	// logger.Info("💭 AI chain of thought analysis:")
	// logger.Info(strings.Repeat("-", 70))
	// logger.Info(decision.CoTTrace)
	// logger.Infof(strings.Repeat("-", 70) + "\n")

	// 7. Print AI decisions
	// logger.Infof("📋 AI decision list (%d items):\n", len(kernel.Decisions))
	// for i, d := range kernel.Decisions {
	//     logger.Infof("  [%d] %s: %s - %s", i+1, d.Symbol, d.Action, d.Reasoning)
	//     if d.Action == "open_long" || d.Action == "open_short" {
	//        logger.Infof("      Leverage: %dx | Position: %.2f USDT | Stop loss: %.4f | Take profit: %.4f",
	//           d.Leverage, d.PositionSizeUSD, d.StopLoss, d.TakeProfit)
	//     }
	// }
	logger.Info()
	logger.Info(strings.Repeat("-", 70))
	// 8. Sort decisions: ensure close positions first, then open positions (prevent position stacking overflow)
	logger.Info(strings.Repeat("-", 70))

	// 8. Sort decisions: ensure close positions first, then open positions (prevent position stacking overflow)
	sortedDecisions := sortDecisionsByPriority(aiDecision.Decisions)

	logger.Info("🔄 Execution order (optimized): Close positions first → Open positions later")
	for i, d := range sortedDecisions {
		logger.Infof("  [%d] %s %s", i+1, d.Symbol, d.Action)
	}
	logger.Info()

	// Check if trader is stopped before executing any decisions (prevent trades after Stop())
	at.isRunningMutex.RLock()
	running = at.isRunning
	at.isRunningMutex.RUnlock()
	if !running {
		logger.Infof("⏹ Trader stopped before decision execution, aborting cycle #%d", at.callCount)
		return nil
	}

	// Safe mode: filter out open positions, only allow close/hold
	if at.safeMode {
		filtered := make([]kernel.Decision, 0)
		for _, d := range sortedDecisions {
			if d.Action == "open_long" || d.Action == "open_short" {
				logger.Warnf("🛡️ [%s] Safe mode: BLOCKED %s %s (no new positions allowed)", at.name, d.Action, d.Symbol)
				continue
			}
			filtered = append(filtered, d)
		}
		sortedDecisions = filtered
		if len(sortedDecisions) == 0 {
			logger.Infof("🛡️ [%s] Safe mode: all decisions were open positions, nothing to execute", at.name)
		}
	}

	// AI close gate: only blocks close_long / close_short generated by the model.
	// Code protection / exchange-native protection remain active and unaffected.
	if !at.GetAllowAIClose() {
		filtered := make([]kernel.Decision, 0, len(sortedDecisions))
		for _, d := range sortedDecisions {
			if d.Action == "close_long" || d.Action == "close_short" {
				logger.Warnf("🚫 [%s] AI close disabled: BLOCKED %s %s", at.name, d.Action, d.Symbol)
				continue
			}
			filtered = append(filtered, d)
		}
		sortedDecisions = filtered
	}

	// Execute decisions and record results
	for _, d := range sortedDecisions {
		// Check if trader is stopped before each decision (allow immediate stop during execution)
		at.isRunningMutex.RLock()
		running = at.isRunning
		at.isRunningMutex.RUnlock()
		if !running {
			logger.Infof("⏹ Trader stopped during decision execution, aborting remaining decisions")
			break
		}

		constraintSnapshot := at.collectExecutionConstraintsSnapshot(d.Symbol)
		policyMode := store.StrategyControlPolicyModeStrict
		if at.config.StrategyConfig != nil {
			policyMode = at.config.StrategyConfig.StrategyControlPolicy.EffectiveMode()
		}
		protectionAlignment := deriveProtectionAlignment(&d, record.ProtectionSnapshot)
		policy := applyRuntimeOpenPolicy(&d, constraintSnapshot, at.getMinRiskRewardRatio(), policyMode, protectionAlignment)
		if policy.Reason != "" {
			appendRuntimePolicyNote(&d, policy.Reason)
		}

		actionRecord := store.DecisionAction{
			Action:     d.Action,
			Symbol:     d.Symbol,
			Quantity:   0,
			Leverage:   d.Leverage,
			Price:      0,
			StopLoss:   d.StopLoss,
			TakeProfit: d.TakeProfit,
			Confidence: d.Confidence,
			Reasoning:  d.Reasoning,
			ReviewContext: buildDecisionActionReviewContext(
				&d,
				at.getMinRiskRewardRatio(),
				record.ProtectionSnapshot,
				constraintSnapshot,
				protectionAlignment,
			),
			Timestamp: time.Now().UTC(),
			Success:   false,
		}

		if actionRecord.ReviewContext != nil {
			actionRecord.ReviewContext.Control = buildRuntimePolicyControlOutcome(policy)
		}

		if policy.Blocked {
			logger.Infof("🚫 %s", policy.Reason)
			actionRecord.Error = policy.Reason
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("🚫 %s %s blocked: %s", d.Symbol, policy.OriginalAction, policy.Reason))
		} else if policy.Decision == "downgraded_to_wait" {
			actionRecord.Success = true
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("⏸ %s %s downgraded to wait: %s", d.Symbol, policy.OriginalAction, policy.Reason))
		} else if err := at.executeDecisionWithRecord(&d, &actionRecord); err != nil {
			logger.Infof("❌ Failed to execute decision (%s %s): %v", d.Symbol, d.Action, err)
			actionRecord.Error = err.Error()
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ %s %s failed: %v", d.Symbol, d.Action, err))
		} else {
			actionRecord.Success = true
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s succeeded", d.Symbol, d.Action))
			// Brief delay after successful execution
			time.Sleep(1 * time.Second)
		}

		record.Decisions = append(record.Decisions, actionRecord)
	}

	// 9. Save decision record
	if err := at.saveDecision(record); err != nil {
		logger.Infof("⚠ Failed to save decision record: %v", err)
	}

	return nil
}

// buildTradingContext builds trading context
func (at *AutoTrader) buildTradingContext() (*kernel.Context, error) {
	// 1. Get account information
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("failed to get account balance: %w", err)
	}

	// Get account fields
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0
	totalEquity := 0.0

	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Use totalEquity directly if provided by trader (more accurate)
	if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
		totalEquity = eq
	} else {
		// Fallback: Total Equity = Wallet balance + Unrealized profit
		totalEquity = totalWalletBalance + totalUnrealizedProfit
	}

	// 2. Get position information
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var positionInfos []kernel.PositionInfo
	totalMarginUsed := 0.0

	// Current position key set (for cleaning up closed position records)
	currentPositionKeys := make(map[string]bool)

	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity // Short position quantity is negative, convert to positive
		}

		// Skip closed positions (quantity = 0), prevent "ghost positions" from being passed to AI
		if quantity == 0 {
			continue
		}

		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)

		// Calculate margin used (estimated)
		leverage := 10 // Default value, should actually be fetched from position info
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed

		// Calculate P&L percentage (based on margin, considering leverage)
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		// Get position open time from exchange (preferred) or fallback to local tracking
		posKey := symbol + "_" + side
		currentPositionKeys[posKey] = true

		var updateTime int64
		// Priority 1: Get from database (trader_positions table) - most accurate
		if at.store != nil {
			if dbPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, side); err == nil && dbPos != nil {
				if dbPos.EntryTime > 0 {
					updateTime = dbPos.EntryTime
				}
			}
		}
		// Priority 2: Get from exchange API (Bybit: createdTime, OKX: createdTime)
		if updateTime == 0 {
			if createdTime, ok := pos["createdTime"].(int64); ok && createdTime > 0 {
				updateTime = createdTime
			}
		}
		// Priority 3: Fallback to local tracking
		if updateTime == 0 {
			if _, exists := at.positionFirstSeenTime[posKey]; !exists {
				at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()
			}
			updateTime = at.positionFirstSeenTime[posKey]
		}

		// Get peak profit rate for this position
		at.peakPnLCacheMutex.RLock()
		peakPnlPct := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		positionInfos = append(positionInfos, kernel.PositionInfo{
			Symbol:           symbol,
			Side:             side,
			EntryPrice:       entryPrice,
			MarkPrice:        markPrice,
			Quantity:         quantity,
			Leverage:         leverage,
			UnrealizedPnL:    unrealizedPnl,
			UnrealizedPnLPct: pnlPct,
			PeakPnLPct:       peakPnlPct,
			LiquidationPrice: liquidationPrice,
			MarginUsed:       marginUsed,
			UpdateTime:       updateTime,
		})
	}

	// Clean up closed position records
	for key := range at.positionFirstSeenTime {
		if !currentPositionKeys[key] {
			delete(at.positionFirstSeenTime, key)
		}
	}

	// 3. Use strategy engine to get candidate coins (must have strategy engine)
	var candidateCoins []kernel.CandidateCoin
	if at.strategyEngine == nil {
		logger.Infof("⚠️ [%s] No strategy engine configured, skipping candidate coins", at.name)
	} else {
		coins, err := at.strategyEngine.GetCandidateCoins()
		if err != nil {
			// Log warning but don't fail - equity snapshot should still be saved
			logger.Infof("⚠️ [%s] Failed to get candidate coins: %v (will use empty list)", at.name, err)
		} else {
			candidateCoins = coins
			logger.Infof("📋 [%s] Strategy engine fetched candidate coins: %d", at.name, len(candidateCoins))
		}
	}

	// 4. Calculate total P&L
	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	// 5. Get leverage from strategy config
	strategyConfig := at.strategyEngine.GetConfig()
	btcEthLeverage := strategyConfig.RiskControl.BTCETHMaxLeverage
	altcoinLeverage := strategyConfig.RiskControl.AltcoinMaxLeverage
	logger.Infof("📋 [%s] Strategy leverage config: BTC/ETH=%dx, Altcoin=%dx", at.name, btcEthLeverage, altcoinLeverage)

	// 6. Build context
	ctx := &kernel.Context{
		CurrentTime:     time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		RuntimeMinutes:  int(time.Since(at.startTime).Minutes()),
		CallCount:       at.callCount,
		BTCETHLeverage:  btcEthLeverage,
		AltcoinLeverage: altcoinLeverage,
		Account: kernel.AccountInfo{
			TotalEquity:      totalEquity,
			AvailableBalance: availableBalance,
			UnrealizedPnL:    totalUnrealizedProfit,
			TotalPnL:         totalPnL,
			TotalPnLPct:      totalPnLPct,
			MarginUsed:       totalMarginUsed,
			MarginUsedPct:    marginUsedPct,
			PositionCount:    len(positionInfos),
		},
		Positions:      positionInfos,
		CandidateCoins: candidateCoins,
	}

	// 7. Add recent closed trades (if store is available)
	if at.store != nil {
		// Get recent 10 closed trades for AI context
		recentTrades, err := at.store.Position().GetRecentTrades(at.id, 10)
		if err != nil {
			logger.Infof("⚠️ [%s] Failed to get recent trades: %v", at.name, err)
		} else {
			logger.Infof("📊 [%s] Found %d recent closed trades for AI context", at.name, len(recentTrades))
			for _, trade := range recentTrades {
				// Convert Unix timestamps to formatted strings for AI readability
				entryTimeStr := ""
				if trade.EntryTime > 0 {
					entryTimeStr = time.Unix(trade.EntryTime, 0).UTC().Format("01-02 15:04 UTC")
				}
				exitTimeStr := ""
				if trade.ExitTime > 0 {
					exitTimeStr = time.Unix(trade.ExitTime, 0).UTC().Format("01-02 15:04 UTC")
				}

				ctx.RecentOrders = append(ctx.RecentOrders, kernel.RecentOrder{
					Symbol:       trade.Symbol,
					Side:         trade.Side,
					EntryPrice:   trade.EntryPrice,
					ExitPrice:    trade.ExitPrice,
					RealizedPnL:  trade.RealizedPnL,
					PnLPct:       trade.PnLPct,
					EntryTime:    entryTimeStr,
					ExitTime:     exitTimeStr,
					HoldDuration: trade.HoldDuration,
				})
			}
		}
		// Get trading statistics for AI context
		stats, err := at.store.Position().GetFullStats(at.id)
		if err != nil {
			logger.Infof("⚠️ [%s] Failed to get trading stats: %v", at.name, err)
		} else if stats == nil {
			logger.Infof("⚠️ [%s] GetFullStats returned nil", at.name)
		} else if stats.TotalTrades == 0 {
			logger.Infof("⚠️ [%s] GetFullStats returned 0 trades (traderID=%s)", at.name, at.id)
		} else {
			ctx.TradingStats = &kernel.TradingStats{
				TotalTrades:    stats.TotalTrades,
				WinRate:        stats.WinRate,
				ProfitFactor:   stats.ProfitFactor,
				SharpeRatio:    stats.SharpeRatio,
				TotalPnL:       stats.TotalPnL,
				AvgWin:         stats.AvgWin,
				AvgLoss:        stats.AvgLoss,
				MaxDrawdownPct: stats.MaxDrawdownPct,
			}
			logger.Infof("📈 [%s] Trading stats: %d trades, %.1f%% win rate, PF=%.2f, Sharpe=%.2f, DD=%.1f%%",
				at.name, stats.TotalTrades, stats.WinRate, stats.ProfitFactor, stats.SharpeRatio, stats.MaxDrawdownPct)
		}
	} else {
		logger.Infof("⚠️ [%s] Store is nil, cannot get recent trades", at.name)
	}

	// 8. Get quantitative data (if enabled in strategy config)
	if strategyConfig.Indicators.EnableQuantData {
		// Collect symbols to query (candidate coins + position coins)
		symbolsToQuery := make(map[string]bool)
		for _, coin := range candidateCoins {
			symbolsToQuery[coin.Symbol] = true
		}
		for _, pos := range positionInfos {
			symbolsToQuery[pos.Symbol] = true
		}

		symbols := make([]string, 0, len(symbolsToQuery))
		for sym := range symbolsToQuery {
			symbols = append(symbols, sym)
		}

		logger.Infof("📊 [%s] Fetching quantitative data for %d symbols...", at.name, len(symbols))
		ctx.QuantDataMap = at.strategyEngine.FetchQuantDataBatch(symbols)
		logger.Infof("📊 [%s] Successfully fetched quantitative data for %d symbols", at.name, len(ctx.QuantDataMap))
	}

	// 9. Get OI ranking data (market-wide position changes)
	if strategyConfig.Indicators.EnableOIRanking {
		logger.Infof("📊 [%s] Fetching OI ranking data...", at.name)
		ctx.OIRankingData = at.strategyEngine.FetchOIRankingData()
		if ctx.OIRankingData != nil {
			logger.Infof("📊 [%s] OI ranking data ready: %d top, %d low positions",
				at.name, len(ctx.OIRankingData.TopPositions), len(ctx.OIRankingData.LowPositions))
		}
	}

	// 10. Get NetFlow ranking data (market-wide fund flow)
	if strategyConfig.Indicators.EnableNetFlowRanking {
		logger.Infof("💰 [%s] Fetching NetFlow ranking data...", at.name)
		ctx.NetFlowRankingData = at.strategyEngine.FetchNetFlowRankingData()
		if ctx.NetFlowRankingData != nil {
			logger.Infof("💰 [%s] NetFlow ranking data ready: inst_in=%d, inst_out=%d",
				at.name, len(ctx.NetFlowRankingData.InstitutionFutureTop), len(ctx.NetFlowRankingData.InstitutionFutureLow))
		}
	}

	// 11. Get Price ranking data (market-wide gainers/losers)
	if strategyConfig.Indicators.EnablePriceRanking {
		logger.Infof("📈 [%s] Fetching Price ranking data...", at.name)
		ctx.PriceRankingData = at.strategyEngine.FetchPriceRankingData()
		if ctx.PriceRankingData != nil {
			logger.Infof("📈 [%s] Price ranking data ready for %d durations",
				at.name, len(ctx.PriceRankingData.Durations))
		}
	}

	return ctx, nil
}

// sortDecisionsByPriority sorts decisions: close positions first, then open positions, finally hold/wait
// This avoids position stacking overflow when changing positions

func sortDecisionsByPriority(decisions []kernel.Decision) []kernel.Decision {
	if len(decisions) <= 1 {
		return decisions
	}

	// Define priority
	getActionPriority := func(action string) int {
		switch action {
		case "close_long", "close_short":
			return 1 // Highest priority: close positions first
		case "open_long", "open_short":
			return 2 // Second priority: open positions later
		case "hold", "wait":
			return 3 // Lowest priority: wait
		default:
			return 999 // Unknown actions at the end
		}
	}

	// Copy decision list
	sorted := make([]kernel.Decision, len(decisions))
	copy(sorted, decisions)

	// Sort by priority
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if getActionPriority(sorted[i].Action) > getActionPriority(sorted[j].Action) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// checkClaw402Balance checks USDC balance and logs warnings if low
func (at *AutoTrader) checkClaw402Balance() {
	scanMinutes := int(at.config.ScanInterval.Minutes())
	if scanMinutes <= 0 {
		scanMinutes = 3
	}
	dailyCost, _ := store.EstimateRunway(1.0, at.config.CustomModelName, scanMinutes)
	logger.Infof("💰 [%s] Estimated daily AI cost: ~$%.2f (model: %s, interval: %dm)",
		at.name, dailyCost, at.config.CustomModelName, scanMinutes)

	if at.claw402WalletAddr != "" {
		balance, err := wallet.QueryUSDCBalance(at.claw402WalletAddr)
		if err != nil {
			logger.Warnf("⚠️ [%s] Failed to query USDC balance: %v", at.name, err)
			return
		}

		if balance < 1.0 {
			logger.Warnf("⚠️ [%s] Low USDC balance: $%.2f — AI may stop soon!", at.name, balance)
		}
		if balance <= 0 {
			logger.Errorf("🚨 [%s] USDC balance is ZERO — AI calls will fail!", at.name)
		}

		runway := float64(0)
		if dailyCost > 0 {
			runway = balance / dailyCost
		}
		logger.Infof("💰 [%s] USDC Balance: $%.2f | Daily AI cost: ~$%.2f | Runway: ~%.1f days",
			at.name, balance, dailyCost, runway)
	}
}

func (at *AutoTrader) getMinRiskRewardRatio() float64 {
	if at != nil && at.config.StrategyConfig != nil {
		if v := at.config.StrategyConfig.RiskControl.MinRiskRewardRatio; v > 0 {
			return v
		}
	}
	return 0
}

func buildDecisionActionReviewContext(decision *kernel.Decision, minRR float64, snapshot *store.ProtectionSnapshot, executionSnapshot ...interface{}) *store.DecisionActionReviewContext {
	ctx := &store.DecisionActionReviewContext{}
	var protectionOverride *store.DecisionActionProtectionAlignment
	if len(executionSnapshot) > 0 {
		if snap, ok := executionSnapshot[0].(*ExecutionConstraintsSnapshot); ok {
			ctx.ExecutionConstraints = mapExecutionConstraintsToActionReview(snap)
		}
	}
	if len(executionSnapshot) > 1 {
		if alignment, ok := executionSnapshot[1].(*store.DecisionActionProtectionAlignment); ok {
			protectionOverride = alignment
		}
	}
	if minRR > 0 {
		ctx.MinRiskReward = minRR
	}
	if decision == nil {
		if reviewContextIsEmpty(ctx) {
			return nil
		}
		return ctx
	}
	if decision.EntryProtection != nil {
		ep := decision.EntryProtection
		if ep.TimeframeContext.Primary != "" {
			ctx.PrimaryTimeframe = ep.TimeframeContext.Primary
		}
		rr := ep.RiskReward
		if rr.Entry > 0 || rr.Invalidation > 0 || rr.FirstTarget > 0 || rr.GrossEstimatedRR > 0 || rr.NetEstimatedRR > 0 || rr.Passed {
			riskReward := &store.DecisionActionRiskRewardSummary{
				Entry:            rr.Entry,
				Invalidation:     rr.Invalidation,
				FirstTarget:      rr.FirstTarget,
				GrossEstimatedRR: rr.GrossEstimatedRR,
				NetEstimatedRR:   rr.NetEstimatedRR,
				Passed:           rr.Passed,
			}
			if !riskReward.Passed && minRR > 0 {
				effectiveRR := rr.GrossEstimatedRR
				if rr.NetEstimatedRR > 0 {
					effectiveRR = rr.NetEstimatedRR
				}
				if effectiveRR > 0 && effectiveRR >= minRR {
					riskReward.Passed = true
				}
			}
			ctx.RiskReward = riskReward
		}
		if len(ep.KeyLevels.Support) > 0 || len(ep.KeyLevels.Resistance) > 0 {
			ctx.KeyLevels = &store.DecisionActionKeyLevels{
				Support:    compactLevelList(ep.KeyLevels.Support),
				Resistance: compactLevelList(ep.KeyLevels.Resistance),
			}
			if len(ctx.KeyLevels.Support) == 0 && len(ctx.KeyLevels.Resistance) == 0 {
				ctx.KeyLevels = nil
			}
		}
		if len(ep.Anchors) > 0 {
			ctx.Anchors = compactReasonAnchors(ep.Anchors)
		}
		if protectionOverride != nil {
			ctx.Protection = protectionOverride
		} else {
			ctx.Protection = deriveProtectionAlignment(decision, snapshot)
		}
	}
	if reviewContextIsEmpty(ctx) {
		return nil
	}
	return ctx
}

func buildRuntimePolicyControlOutcome(policy runtimePolicyResult) *store.DecisionActionControlOutcome {
	if !policy.Blocked && policy.Reason == "" && !policy.ConstraintsMerged && !policy.RRRecomputed && policy.AIGrossRR == 0 && policy.AINetRR == 0 && policy.RuntimeGrossRR == 0 && policy.RuntimeNetRR == 0 && policy.EffectiveRR == 0 && len(policy.ConstraintsSources) == 0 && policy.OriginalAction == "" && policy.FinalAction == "" {
		return nil
	}
	out := &store.DecisionActionControlOutcome{
		Decision:                   "accepted",
		OriginalAction:             policy.OriginalAction,
		FinalAction:                policy.FinalAction,
		ConstraintsMerged:          policy.ConstraintsMerged,
		RuntimeRRRecomputed:        policy.RRRecomputed,
		AIGrossRR:                  policy.AIGrossRR,
		AINetRR:                    policy.AINetRR,
		RuntimeGrossRR:             policy.RuntimeGrossRR,
		RuntimeNetRR:               policy.RuntimeNetRR,
		EffectiveRR:                policy.EffectiveRR,
		EffectiveRRSource:          policy.EffectiveRRSource,
		ExecutionConstraintSources: policy.ConstraintsSources,
	}
	if policy.Decision != "" {
		out.Decision = policy.Decision
	}
	if policy.Blocked || policy.Decision == "downgraded_to_wait" {
		out.NoOrderPlaced = true
	}
	if policy.Reason != "" {
		out.Reasons = []string{policy.Reason}
	}
	if policy.ReasonCode != "" {
		out.FailedChecks = []string{policy.ReasonCode}
	}
	return out
}

func compactLevelList(levels []float64) []float64 {
	compact := make([]float64, 0, 2)
	for _, level := range levels {
		if !isFinitePositive(level) {
			continue
		}
		compact = append(compact, level)
		if len(compact) >= 2 {
			break
		}
	}
	return compact
}

func compactReasonAnchors(anchors []kernel.AIEntryProtectionAnchor) []store.DecisionActionReasonAnchor {
	compact := make([]store.DecisionActionReasonAnchor, 0, minInt(len(anchors), 3))
	for _, anchor := range anchors {
		if anchor.Type == "" && anchor.Timeframe == "" && anchor.Price <= 0 && anchor.Reason == "" {
			continue
		}
		compact = append(compact, store.DecisionActionReasonAnchor{
			Type:      anchor.Type,
			Timeframe: anchor.Timeframe,
			Price:     anchor.Price,
			Reason:    anchor.Reason,
		})
		if len(compact) >= 3 {
			break
		}
	}
	return compact
}

func deriveProtectionAlignment(decision *kernel.Decision, snapshot *store.ProtectionSnapshot) *store.DecisionActionProtectionAlignment {
	if decision == nil || decision.EntryProtection == nil {
		return nil
	}
	rr := decision.EntryProtection.RiskReward
	if rr.Entry <= 0 || rr.Invalidation <= 0 || rr.FirstTarget <= 0 {
		policyStatus, policyOverride, policyRejected, policyReasons := deriveProtectionPolicyTransparency(nil)
		if len(decision.EntryProtection.AlignmentNotes) == 0 && policyStatus == "" {
			return nil
		}
		return &store.DecisionActionProtectionAlignment{
			PolicyStatus:   policyStatus,
			PolicyOverride: policyOverride,
			PolicyRejected: policyRejected,
			PolicyReasons:  policyReasons,
			Notes:          compactNotes(decision.EntryProtection.AlignmentNotes, 3),
		}
	}
	isLong := decision.Action == "open_long"
	isShort := decision.Action == "open_short"
	alignment := &store.DecisionActionProtectionAlignment{
		Notes: compactNotes(decision.EntryProtection.AlignmentNotes, 3),
	}
	hasSignal := len(alignment.Notes) > 0
	if snapshot != nil {
		if stopPrice, ok := deriveProtectionStopPrice(snapshot, rr.Entry, isLong, isShort); ok {
			alignment.StopBeyondInvalidation = (isLong && stopPrice <= rr.Invalidation) || (isShort && stopPrice >= rr.Invalidation)
			hasSignal = true
		}
		if targetPrice, ok := deriveProtectionTargetPrice(snapshot, rr.Entry, isLong, isShort); ok {
			alignment.TargetAligned = (isLong && targetPrice >= rr.FirstTarget) || (isShort && targetPrice <= rr.FirstTarget)
			hasSignal = true
		}
		if snapshot.BreakEven != nil && snapshot.BreakEven.Enabled {
			triggerPrice, ok := deriveBreakEvenTriggerPrice(snapshot.BreakEven, rr.Entry, isLong, isShort)
			if ok {
				alignment.BreakEvenBeforeTarget = (isLong && triggerPrice <= rr.FirstTarget) || (isShort && triggerPrice >= rr.FirstTarget)
				hasSignal = true
			}
		}
		if fallbackPrice, ok := deriveFallbackMaxLossPrice(snapshot, rr.Entry, isLong, isShort); ok {
			alignment.FallbackWithinEnvelope = (isLong && fallbackPrice <= rr.Invalidation) || (isShort && fallbackPrice >= rr.Invalidation)
			hasSignal = true
		}
	}
	alignment.PolicyStatus, alignment.PolicyOverride, alignment.PolicyRejected, alignment.PolicyReasons = deriveProtectionPolicyTransparency(alignment)
	if alignment.PolicyStatus != "" {
		hasSignal = true
	}
	if !hasSignal {
		return nil
	}
	return alignment
}

func deriveProtectionPolicyTransparency(alignment *store.DecisionActionProtectionAlignment) (status string, override bool, rejected bool, reasons []string) {
	if alignment == nil {
		return "", false, false, nil
	}
	reasons = make([]string, 0, 4)
	if !alignment.StopBeyondInvalidation {
		reasons = append(reasons, "stop_inside_invalidation")
	}
	if !alignment.TargetAligned {
		reasons = append(reasons, "target_before_first_target")
	}
	if !alignment.BreakEvenBeforeTarget {
		reasons = append(reasons, "break_even_after_target")
	}
	if !alignment.FallbackWithinEnvelope {
		reasons = append(reasons, "fallback_inside_invalidation")
	}

	switch len(reasons) {
	case 0:
		return "aligned", false, false, nil
	case 1:
		return "recomputed", true, false, reasons
	default:
		return "rejected", true, true, reasons
	}
}

func deriveProtectionStopPrice(snapshot *store.ProtectionSnapshot, entry float64, isLong, isShort bool) (float64, bool) {
	if snapshot == nil || entry <= 0 {
		return 0, false
	}
	if snapshot.FullTPSL != nil {
		if price, ok := valueSourceToAbsolutePrice(snapshot.FullTPSL.StopLoss, entry, isLong, isShort, false); ok {
			return price, true
		}
	}
	if snapshot.LadderTPSL != nil {
		if price, ok := valueSourceToAbsolutePrice(snapshot.LadderTPSL.StopLossPrice, entry, isLong, isShort, false); ok {
			return price, true
		}
		for _, rule := range snapshot.LadderTPSL.Rules {
			if price, ok := pctOffsetPrice(entry, rule.StopLossPct, isLong, isShort, false); ok {
				return price, true
			}
		}
	}
	return 0, false
}

func deriveProtectionTargetPrice(snapshot *store.ProtectionSnapshot, entry float64, isLong, isShort bool) (float64, bool) {
	if snapshot == nil || entry <= 0 {
		return 0, false
	}
	if snapshot.FullTPSL != nil {
		if price, ok := valueSourceToAbsolutePrice(snapshot.FullTPSL.TakeProfit, entry, isLong, isShort, true); ok {
			return price, true
		}
	}
	if snapshot.LadderTPSL != nil {
		if price, ok := valueSourceToAbsolutePrice(snapshot.LadderTPSL.TakeProfitPrice, entry, isLong, isShort, true); ok {
			return price, true
		}
		for _, rule := range snapshot.LadderTPSL.Rules {
			if price, ok := pctOffsetPrice(entry, rule.TakeProfitPct, isLong, isShort, true); ok {
				return price, true
			}
		}
	}
	return 0, false
}

func deriveFallbackMaxLossPrice(snapshot *store.ProtectionSnapshot, entry float64, isLong, isShort bool) (float64, bool) {
	if snapshot == nil || entry <= 0 {
		return 0, false
	}
	if snapshot.FullTPSL != nil {
		if price, ok := valueSourceToAbsolutePrice(snapshot.FullTPSL.FallbackMaxLoss, entry, isLong, isShort, false); ok {
			return price, true
		}
	}
	if snapshot.LadderTPSL != nil {
		if price, ok := valueSourceToAbsolutePrice(snapshot.LadderTPSL.FallbackMaxLoss, entry, isLong, isShort, false); ok {
			return price, true
		}
	}
	return 0, false
}

func deriveBreakEvenTriggerPrice(be *store.ProtectionSnapshotBreakEven, entry float64, isLong, isShort bool) (float64, bool) {
	if be == nil || !be.Enabled || entry <= 0 || be.TriggerValue <= 0 {
		return 0, false
	}
	switch be.TriggerMode {
	case "profit_pct", "pnl_pct", "percent", "pct":
		return pctOffsetPrice(entry, be.TriggerValue, isLong, isShort, true)
	case "price":
		return be.TriggerValue, be.TriggerValue > 0
	default:
		return 0, false
	}
}

func valueSourceToAbsolutePrice(src store.ProtectionSnapshotValueSource, entry float64, isLong, isShort bool, favorable bool) (float64, bool) {
	if entry <= 0 {
		return 0, false
	}
	mode := src.Mode
	if mode == "" {
		mode = "price"
	}
	switch mode {
	case "price", "absolute":
		return src.Value, src.Value > 0
	case "percent", "pct", "profit_pct", "loss_pct", "offset_pct":
		return pctOffsetPrice(entry, src.Value, isLong, isShort, favorable)
	default:
		return 0, false
	}
}

func pctOffsetPrice(entry, pct float64, isLong, isShort bool, favorable bool) (float64, bool) {
	if entry <= 0 || pct <= 0 {
		return 0, false
	}
	multiplier := pct / 100.0
	switch {
	case isLong && favorable:
		return entry * (1 + multiplier), true
	case isLong && !favorable:
		return entry * (1 - multiplier), true
	case isShort && favorable:
		return entry * (1 - multiplier), true
	case isShort && !favorable:
		return entry * (1 + multiplier), true
	default:
		return 0, false
	}
}

func compactNotes(notes []string, limit int) []string {
	compact := make([]string, 0, minInt(len(notes), limit))
	for _, note := range notes {
		note = strings.TrimSpace(note)
		if note == "" {
			continue
		}
		compact = append(compact, note)
		if len(compact) >= limit {
			break
		}
	}
	return compact
}

func reviewContextIsEmpty(ctx *store.DecisionActionReviewContext) bool {
	if ctx == nil {
		return true
	}
	return ctx.PrimaryTimeframe == "" &&
		ctx.MinRiskReward == 0 &&
		ctx.RiskReward == nil &&
		ctx.KeyLevels == nil &&
		len(ctx.Anchors) == 0 &&
		ctx.Protection == nil &&
		ctx.ExecutionConstraints == nil
}

func isFinitePositive(v float64) bool {
	return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
