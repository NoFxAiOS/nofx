package backtest

import (
	"testing"
)

func TestCheckReplayOnlyMacroMicro_ReplayOnlyWithMacroMicro_ReturnsError(t *testing.T) {
	err := checkReplayOnlyMacroMicro(true, true)
	if err == nil {
		t.Fatal("expected error when replay_only and macro-micro both true")
	}
	if err.Error() != "replay_only is incompatible with macro-micro flow (no AI cache for multi-turn)" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckReplayOnlyMacroMicro_ReplayOnlyWithoutMacroMicro_NoError(t *testing.T) {
	err := checkReplayOnlyMacroMicro(true, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckReplayOnlyMacroMicro_MacroMicroWithoutReplayOnly_NoError(t *testing.T) {
	err := checkReplayOnlyMacroMicro(false, true)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
