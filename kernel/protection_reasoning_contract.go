package kernel

import (
	"fmt"
	"strings"

	"nofx/store"
)

func ValidateProtectionReasoningContract(cotTrace string, config *store.StrategyConfig) error {
	if config == nil {
		return nil
	}
	reasoning := strings.ToLower(strings.TrimSpace(cotTrace))
	if reasoning == "" {
		return nil
	}
	if config.Protection.DrawdownTakeProfit.Enabled {
		if !strings.Contains(reasoning, "drawdown") && !strings.Contains(reasoning, "trailing") && !strings.Contains(reasoning, "profit-protection") {
			return fmt.Errorf("reasoning must acknowledge drawdown/trailing profit protection when drawdown_take_profit is enabled")
		}
		if config.Protection.DrawdownTakeProfit.Mode == store.ProtectionModeAI {
			hasTimeframe := strings.Contains(reasoning, "timeframe") || strings.Contains(reasoning, "周期") || strings.Contains(reasoning, "15m") || strings.Contains(reasoning, "5m") || strings.Contains(reasoning, "1h")
			hasStructure := strings.Contains(reasoning, "resistance") || strings.Contains(reasoning, "support") || strings.Contains(reasoning, "阻力") || strings.Contains(reasoning, "支撑") || strings.Contains(reasoning, "fibonacci") || strings.Contains(reasoning, "fib") || strings.Contains(reasoning, "波动") || strings.Contains(reasoning, "volatility")
			if !hasTimeframe || !hasStructure {
				return fmt.Errorf("drawdown ai reasoning must reference primary/adjacent timeframes and structural anchors like support/resistance/fibonacci/volatility")
			}
		}
	}
	if config.Protection.BreakEvenStop.Enabled {
		if !strings.Contains(reasoning, "break-even") && !strings.Contains(reasoning, "breakeven") && !strings.Contains(reasoning, "stop layer") {
			return fmt.Errorf("reasoning must acknowledge break-even protection when break_even_stop is enabled")
		}
	}
	return nil
}
