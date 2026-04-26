package trader

func (at *AutoTrader) hasConfiguredProtectionOwner() bool {
	if at == nil || at.config.StrategyConfig == nil {
		return false
	}
	p := at.config.StrategyConfig.Protection
	return p.FullTPSL.Enabled || p.LadderTPSL.Enabled || p.DrawdownTakeProfit.Enabled || p.BreakEvenStop.Enabled
}
