package trader

import "nofx/store"

type ProtectionOwnerPolicy struct {
	StopOwner        string
	ProfitOwner      string
	UseLadderStops   bool
	UseDrawdownTP    bool
	SuppressStaticTP bool
}

func evaluateProtectionOwnerPolicy(protection store.ProtectionConfig) ProtectionOwnerPolicy {
	policy := ProtectionOwnerPolicy{}
	if protection.LadderTPSL.Enabled && protectionFeatureUsesManual(protection.LadderTPSL.Mode) && protection.LadderTPSL.StopLossEnabled {
		policy.StopOwner = "ladder"
		policy.UseLadderStops = true
	} else if protection.FullTPSL.Enabled && protectionFeatureUsesManual(protection.FullTPSL.Mode) && protection.FullTPSL.StopLoss.Mode != store.ProtectionValueModeDisabled {
		policy.StopOwner = "full"
	} else if protection.BreakEvenStop.Enabled {
		policy.StopOwner = "break_even_dynamic"
	}

	if protection.DrawdownTakeProfit.Enabled {
		policy.ProfitOwner = "drawdown"
		policy.UseDrawdownTP = true
		policy.SuppressStaticTP = true
	} else if protection.LadderTPSL.Enabled && protectionFeatureUsesManual(protection.LadderTPSL.Mode) && protection.LadderTPSL.TakeProfitEnabled {
		policy.ProfitOwner = "ladder"
	} else if protection.FullTPSL.Enabled && protectionFeatureUsesManual(protection.FullTPSL.Mode) && protection.FullTPSL.TakeProfit.Mode != store.ProtectionValueModeDisabled {
		policy.ProfitOwner = "full"
	}
	return policy
}
