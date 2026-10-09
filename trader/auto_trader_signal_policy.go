package trader

import (
	"fmt"
	"strings"

	"nofx/kernel"
	"nofx/logger"
	"nofx/trader/types"
)

type positionCacheInvalidator interface {
	InvalidatePositionCache()
}

func (at *AutoTrader) usesVergexSignalPolicy() bool {
	return at != nil &&
		at.config.StrategyConfig != nil &&
		strings.EqualFold(strings.TrimSpace(at.config.StrategyConfig.CoinSource.SourceType), "vergex_signal")
}

// usesSignalManagedExit keeps ordinary profit-taking under the direction
// state machine. A protective stop remains on the exchange for hard-risk
// containment, but an unchanged board signal must not be closed by a fixed TP.
func (at *AutoTrader) usesSignalManagedExit() bool {
	return at.usesVergexSignalPolicy()
}

func validateProtectionPrices(action string, marketPrice, stopLoss, takeProfit float64, signalManaged bool) error {
	if marketPrice <= 0 || stopLoss <= 0 {
		return fmt.Errorf("market price and stop loss must be positive")
	}
	switch action {
	case "open_long":
		if stopLoss >= marketPrice {
			return fmt.Errorf("long stop loss %.8f must be below market price %.8f", stopLoss, marketPrice)
		}
		if !signalManaged && takeProfit <= marketPrice {
			return fmt.Errorf("long take profit %.8f must be above market price %.8f", takeProfit, marketPrice)
		}
	case "open_short":
		if stopLoss <= marketPrice {
			return fmt.Errorf("short stop loss %.8f must be above market price %.8f", stopLoss, marketPrice)
		}
		if !signalManaged && (takeProfit <= 0 || takeProfit >= marketPrice) {
			return fmt.Errorf("short take profit %.8f must be positive and below market price %.8f", takeProfit, marketPrice)
		}
	default:
		return fmt.Errorf("unsupported open action %q", action)
	}
	if signalManaged && takeProfit != 0 {
		return fmt.Errorf("signal-managed take profit must be 0")
	}
	return nil
}

func (at *AutoTrader) closeUnprotectedPosition(symbol, side string, quantity float64, protectionErr error) error {
	logger.Infof("  🚨 %v; emergency-closing %s %s", protectionErr, symbol, side)
	if closeErr := at.emergencyClosePositionAndVerify(symbol, side, quantity); closeErr != nil {
		return fmt.Errorf("%w; emergency close failed: %v", protectionErr, closeErr)
	}
	return fmt.Errorf("%w; opened position was emergency-closed", protectionErr)
}

func (at *AutoTrader) emergencyClosePositionAndVerify(symbol, side string, quantity float64) error {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		var err error
		if side == "long" {
			_, err = at.trader.CloseLong(symbol, quantity)
		} else if side == "short" {
			_, err = at.trader.CloseShort(symbol, quantity)
		} else {
			return fmt.Errorf("unknown position direction: %s", side)
		}
		if err != nil {
			lastErr = err
		} else {
			positions, err := getFreshPositions(at.trader)
			if err != nil {
				lastErr = fmt.Errorf("verify flat position: %w", err)
			} else {
				residual := false
				for _, pos := range positions {
					if universeBaseKey(fmt.Sprint(pos["symbol"])) == universeBaseKey(symbol) && strings.EqualFold(fmt.Sprint(pos["side"]), side) {
						residual = true
						break
					}
				}
				if !residual {
					if err := at.trader.CancelAllOrders(symbol); err != nil {
						return fmt.Errorf("position is flat but protection-order cleanup failed: %w", err)
					}
					openOrders, err := at.trader.GetOpenOrders(symbol)
					if err != nil {
						return fmt.Errorf("position is flat but protection-order verification failed: %w", err)
					}
					if len(openOrders) != 0 {
						return fmt.Errorf("position is flat but %d open orders remain for %s", len(openOrders), symbol)
					}
					return nil
				}
				lastErr = fmt.Errorf("residual %s position remains after close attempt %d", side, attempt)
			}
		}
	}
	return lastErr
}

func getFreshPositions(tr types.Trader) ([]map[string]interface{}, error) {
	if invalidator, ok := tr.(positionCacheInvalidator); ok {
		invalidator.InvalidatePositionCache()
	}
	return tr.GetPositions()
}

// enforceVergexSignalPolicy turns the current direction board into a strict
// position state machine. Detail data can explain a signal, but cannot reverse
// or prematurely exit it.
func (at *AutoTrader) enforceVergexSignalPolicy(decisions []kernel.Decision, ctx *kernel.Context) []kernel.Decision {
	if !at.usesVergexSignalPolicy() || at.strategyEngine == nil || ctx == nil || !at.strategyEngine.HasVergexSignalSnapshot() {
		return decisions
	}

	filtered, blocked := applyVergexSignalPolicy(
		decisions,
		ctx.Positions,
		at.strategyEngine.VergexSignalBias,
	)
	for _, decision := range blocked {
		at.logWarnf("🧭 Blocked %s %s: action conflicts with the current Claw402 direction signal", decision.Symbol, decision.Action)
	}
	return filtered
}

func applyVergexSignalPolicy(
	decisions []kernel.Decision,
	positions []kernel.PositionInfo,
	biasFor func(string) (string, bool),
) (filtered []kernel.Decision, blocked []kernel.Decision) {
	positionActions := make(map[string]string, len(positions))
	filtered = make([]kernel.Decision, 0, len(decisions)+len(positions))

	// Existing positions are managed only by the current board direction.
	// A matching signal always holds; a changed, neutral, or absent signal exits.
	for _, position := range positions {
		base := universeBaseKey(position.Symbol)
		bias, present := biasFor(position.Symbol)
		side := strings.ToLower(strings.TrimSpace(position.Side))
		matches := (side == "long" && present && bias == "bullish") ||
			(side == "short" && present && bias == "bearish")

		action := "hold"
		reasoning := fmt.Sprintf("Claw402 direction remains %s; hold the existing %s position", bias, side)
		if !matches {
			switch side {
			case "long":
				action = "close_long"
			case "short":
				action = "close_short"
			}
			if !present {
				reasoning = "Symbol is no longer present on the current Claw402 direction board"
			} else {
				reasoning = fmt.Sprintf("Claw402 direction changed to %s", bias)
			}
		}
		if base != "" {
			positionActions[base] = action
		}
		filtered = append(filtered, kernel.Decision{
			Symbol:     position.Symbol,
			Action:     action,
			Confidence: 100,
			Reasoning:  reasoning,
		})
	}

	// Flat symbols may only open in the exact direction advertised by the board.
	for _, decision := range decisions {
		base := universeBaseKey(decision.Symbol)
		if positionAction, exists := positionActions[base]; base != "" && exists {
			if strings.ToLower(strings.TrimSpace(decision.Action)) != positionAction {
				blocked = append(blocked, decision)
			}
			continue
		}
		if !isOpenAction(decision.Action) {
			filtered = append(filtered, decision)
			continue
		}

		bias, present := biasFor(decision.Symbol)
		action := strings.ToLower(strings.TrimSpace(decision.Action))
		allowed := present &&
			((action == "open_long" && bias == "bullish") ||
				(action == "open_short" && bias == "bearish"))
		if allowed {
			filtered = append(filtered, decision)
		} else {
			blocked = append(blocked, decision)
		}
	}
	return filtered, blocked
}
