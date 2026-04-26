package trader

import "testing"

func TestPlanRollingProtectionMigrationAddsNewBeforeRemovingObsoleteAndKeepsBridge(t *testing.T) {
	current := []RollingProtectionTier{
		{Kind: RollingTierDrawdown, Fingerprint: "dd1", StageName: "DD1", Priority: 1, Verified: true},
		{Kind: RollingTierDrawdown, Fingerprint: "dd2", StageName: "DD2", Priority: 2, Verified: true},
	}
	desired := []RollingProtectionTier{
		{Kind: RollingTierDrawdown, Fingerprint: "dd2", StageName: "DD2", Priority: 2},
		{Kind: RollingTierDrawdown, Fingerprint: "dd3", StageName: "DD3", Priority: 3},
	}

	plan := planRollingProtectionMigration(current, desired)
	if len(plan.AddFirst) != 1 || plan.AddFirst[0].Fingerprint != "dd3" {
		t.Fatalf("expected DD3 add-first, got %+v", plan.AddFirst)
	}
	if len(plan.Keep) != 1 || plan.Keep[0].Fingerprint != "dd2" {
		t.Fatalf("expected DD2 bridge keep, got %+v", plan.Keep)
	}
	if len(plan.RemoveAfter) != 1 || plan.RemoveAfter[0].Fingerprint != "dd1" {
		t.Fatalf("expected DD1 remove-after, got %+v", plan.RemoveAfter)
	}
	if !preservesProfitProtectionBridge(plan) {
		t.Fatal("expected plan to preserve profit-protection bridge")
	}
}

func TestFinalizeRollingProtectionMigrationBlocksRemovalWhenNewTierNotVerified(t *testing.T) {
	plan := RollingProtectionPlan{
		AddFirst:    []RollingProtectionTier{{Kind: RollingTierDrawdown, Fingerprint: "dd3", Priority: 3}},
		Keep:        []RollingProtectionTier{{Kind: RollingTierDrawdown, Fingerprint: "dd2", Priority: 2, Verified: true}},
		RemoveAfter: []RollingProtectionTier{{Kind: RollingTierDrawdown, Fingerprint: "dd1", Priority: 1, Verified: true}},
	}

	final := finalizeRollingProtectionMigration(plan, false)
	if !final.Degraded {
		t.Fatal("expected degraded plan when add verification fails")
	}
	if len(final.RemoveAfter) != 0 {
		t.Fatalf("expected removals blocked, got %+v", final.RemoveAfter)
	}
	if len(final.BlockedRemove) != 1 || final.BlockedRemove[0].Fingerprint != "dd1" {
		t.Fatalf("expected DD1 removal blocked, got %+v", final.BlockedRemove)
	}
}

func TestPlanRollingProtectionMigrationAppliesSameBridgeRuleToLadderTP(t *testing.T) {
	current := []RollingProtectionTier{
		{Kind: RollingTierLadderTP, Fingerprint: "tp1", StageName: "TP1", Priority: 1, Verified: true},
		{Kind: RollingTierLadderTP, Fingerprint: "tp2", StageName: "TP2", Priority: 2, Verified: true},
	}
	desired := []RollingProtectionTier{
		{Kind: RollingTierLadderTP, Fingerprint: "tp2", StageName: "TP2", Priority: 2},
		{Kind: RollingTierLadderTP, Fingerprint: "tp3", StageName: "TP3", Priority: 3},
	}

	plan := planRollingProtectionMigration(current, desired)
	if len(plan.AddFirst) != 1 || plan.AddFirst[0].Fingerprint != "tp3" {
		t.Fatalf("expected TP3 add-first, got %+v", plan.AddFirst)
	}
	if len(plan.Keep) != 1 || plan.Keep[0].Fingerprint != "tp2" {
		t.Fatalf("expected TP2 bridge keep, got %+v", plan.Keep)
	}
	if len(plan.RemoveAfter) != 1 || plan.RemoveAfter[0].Fingerprint != "tp1" {
		t.Fatalf("expected TP1 remove-after, got %+v", plan.RemoveAfter)
	}
}
