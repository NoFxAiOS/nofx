package store

import "testing"

func TestMigrateLegacyAutopilotRiskDefaults(t *testing.T) {
	for _, ratio := range []float64{5, 10} {
		cfg := GetDefaultStrategyConfig("en")
		cfg.RiskControl.MaxPositions = 2
		cfg.RiskControl.BTCETHMaxPositionValueRatio = ratio
		cfg.RiskControl.AltcoinMaxPositionValueRatio = ratio

		if !MigrateLegacyAutopilotRiskDefaults(&cfg) {
			t.Fatalf("expected legacy %.1fx config to migrate", ratio)
		}
		if cfg.RiskControl.MaxPositions != AutopilotDefaultMaxPositions {
			t.Fatalf("max positions = %d, want %d", cfg.RiskControl.MaxPositions, AutopilotDefaultMaxPositions)
		}
		if cfg.RiskControl.BTCETHMaxPositionValueRatio != AutopilotMaxPositionValueRatio ||
			cfg.RiskControl.AltcoinMaxPositionValueRatio != AutopilotMaxPositionValueRatio {
			t.Fatalf("position ratios were not migrated: %+v", cfg.RiskControl)
		}
	}

	cfg := GetDefaultStrategyConfig("en")
	cfg.RiskControl.MaxPositions = 4
	cfg.RiskControl.BTCETHMaxPositionValueRatio = legacyAutopilotAllocatedPositionRatio
	cfg.RiskControl.AltcoinMaxPositionValueRatio = legacyAutopilotAllocatedPositionRatio
	if !MigrateLegacyAutopilotRiskDefaults(&cfg) {
		t.Fatal("expected legacy four-position allocation to migrate")
	}
	if cfg.RiskControl.MaxPositions != AutopilotDefaultMaxPositions ||
		cfg.RiskControl.BTCETHMaxPositionValueRatio != AutopilotMaxPositionValueRatio ||
		cfg.RiskControl.AltcoinMaxPositionValueRatio != AutopilotMaxPositionValueRatio {
		t.Fatalf("four-position config was not migrated: %+v", cfg.RiskControl)
	}
}

func TestMigrateLegacyAutopilotRiskDefaultsPreservesCustomBooks(t *testing.T) {
	cfg := GetDefaultStrategyConfig("en")
	cfg.RiskControl.MaxPositions = 3
	cfg.RiskControl.BTCETHMaxPositionValueRatio = 3
	cfg.RiskControl.AltcoinMaxPositionValueRatio = 3

	if MigrateLegacyAutopilotRiskDefaults(&cfg) {
		t.Fatal("custom Autopilot config should not migrate")
	}
	if cfg.RiskControl.MaxPositions != 3 || cfg.RiskControl.AltcoinMaxPositionValueRatio != 3 {
		t.Fatalf("custom config changed unexpectedly: %+v", cfg.RiskControl)
	}

	cfg.CoinSource.SourceType = "static"
	cfg.RiskControl.MaxPositions = 2
	if MigrateLegacyAutopilotRiskDefaults(&cfg) {
		t.Fatal("non-Autopilot config should not migrate")
	}
}

func TestClampLimitsEnforcesAutopilotPositionHardCap(t *testing.T) {
	cfg := GetDefaultStrategyConfig("en")
	cfg.RiskControl.BTCETHMaxPositionValueRatio = 10
	cfg.RiskControl.AltcoinMaxPositionValueRatio = 8

	cfg.ClampLimits()

	if cfg.RiskControl.BTCETHMaxPositionValueRatio != AutopilotMaxPositionValueRatio ||
		cfg.RiskControl.AltcoinMaxPositionValueRatio != AutopilotMaxPositionValueRatio {
		t.Fatalf("Autopilot position hard cap was not enforced: %+v", cfg.RiskControl)
	}
}
