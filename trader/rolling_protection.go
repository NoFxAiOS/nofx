package trader

import "sort"

type RollingProtectionTierKind string

const (
	RollingTierDrawdown RollingProtectionTierKind = "drawdown"
	RollingTierLadderTP RollingProtectionTierKind = "ladder_tp"
	RollingTierLadderSL RollingProtectionTierKind = "ladder_sl"
)

type RollingProtectionTier struct {
	Kind             RollingProtectionTierKind
	Fingerprint      string
	StageName        string
	Priority         int
	ActivationPct    float64
	CloseRatioPct    float64
	Verified         bool
	Source           string
	StructuralAnchor string
}

type RollingProtectionPlan struct {
	AddFirst      []RollingProtectionTier
	Keep          []RollingProtectionTier
	RemoveAfter   []RollingProtectionTier
	BlockedRemove []RollingProtectionTier
	Degraded      bool
	Reasons       []string
}

func planRollingProtectionMigration(current, desired []RollingProtectionTier) RollingProtectionPlan {
	plan := RollingProtectionPlan{}
	currentByID := map[string]RollingProtectionTier{}
	desiredByID := map[string]RollingProtectionTier{}

	for _, tier := range current {
		if tier.Fingerprint == "" {
			continue
		}
		currentByID[tier.Fingerprint] = tier
	}
	for _, tier := range desired {
		if tier.Fingerprint == "" {
			continue
		}
		desiredByID[tier.Fingerprint] = tier
	}

	for id, tier := range desiredByID {
		if cur, ok := currentByID[id]; ok {
			plan.Keep = append(plan.Keep, cur)
			continue
		}
		plan.AddFirst = append(plan.AddFirst, tier)
	}
	for id, tier := range currentByID {
		if _, ok := desiredByID[id]; ok {
			continue
		}
		plan.RemoveAfter = append(plan.RemoveAfter, tier)
	}

	sortRollingTiers(plan.AddFirst)
	sortRollingTiers(plan.Keep)
	sortRollingTiers(plan.RemoveAfter)
	return plan
}

func finalizeRollingProtectionMigration(plan RollingProtectionPlan, addVerified bool) RollingProtectionPlan {
	if len(plan.AddFirst) == 0 {
		return plan
	}
	if addVerified {
		return plan
	}
	plan.Degraded = true
	plan.Reasons = append(plan.Reasons, "new_tiers_not_verified_keep_existing")
	plan.BlockedRemove = append(plan.BlockedRemove, plan.RemoveAfter...)
	plan.RemoveAfter = nil
	return plan
}

func preservesProfitProtectionBridge(plan RollingProtectionPlan) bool {
	for _, tier := range plan.Keep {
		if tier.Kind == RollingTierDrawdown || tier.Kind == RollingTierLadderTP {
			return true
		}
	}
	return len(plan.AddFirst) > 0 && len(plan.RemoveAfter) == 0
}

func sortRollingTiers(tiers []RollingProtectionTier) {
	sort.SliceStable(tiers, func(i, j int) bool {
		if tiers[i].Priority == tiers[j].Priority {
			return tiers[i].Fingerprint < tiers[j].Fingerprint
		}
		return tiers[i].Priority < tiers[j].Priority
	})
}
