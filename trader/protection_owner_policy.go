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
	if protection.LadderTPSL.Enabled && protection.LadderTPSL.StopLossEnabled {
		policy.StopOwner = "ladder"
		policy.UseLadderStops = true
	} else if protection.FullTPSL.Enabled && protection.FullTPSL.StopLoss.Mode != store.ProtectionValueModeDisabled {
		policy.StopOwner = "full"
	} else if protection.BreakEvenStop.Enabled {
		policy.StopOwner = "break_even_dynamic"
	}

	if protection.DrawdownTakeProfit.Enabled {
		policy.ProfitOwner = "drawdown"
		policy.UseDrawdownTP = true
		policy.SuppressStaticTP = true
	} else if protection.LadderTPSL.Enabled && protection.LadderTPSL.TakeProfitEnabled {
		policy.ProfitOwner = "ladder"
	} else if protection.FullTPSL.Enabled && protection.FullTPSL.TakeProfit.Mode != store.ProtectionValueModeDisabled {
		policy.ProfitOwner = "full"
	}
	return policy
}
