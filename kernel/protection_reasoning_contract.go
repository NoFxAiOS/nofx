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
	if config.Protection.DrawdownTakeProfit.Enabled && len(config.Protection.DrawdownTakeProfit.Rules) > 0 {
		if !strings.Contains(reasoning, "drawdown") && !strings.Contains(reasoning, "trailing") && !strings.Contains(reasoning, "profit-protection") {
			return fmt.Errorf("reasoning must acknowledge drawdown/trailing profit protection when drawdown_take_profit is enabled")
		}
	}
	if config.Protection.BreakEvenStop.Enabled {
		if !strings.Contains(reasoning, "break-even") && !strings.Contains(reasoning, "breakeven") && !strings.Contains(reasoning, "stop layer") {
			return fmt.Errorf("reasoning must acknowledge break-even protection when break_even_stop is enabled")
		}
	}
	return nil
}
