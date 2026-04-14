package trader

import (
	"nofx/kernel"
	"strings"
	"testing"
)

func TestValidateSpotOnlyDecisionsRejectsShorts(t *testing.T) {
	err := validateSpotOnlyDecisions([]kernel.Decision{
		{Symbol: "sol:DezX...", Action: "open_short"},
	})
	if err == nil {
		t.Fatal("expected error for open_short on spot-only exchange")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSpotOnlyDecisionsRejectsLeverageAboveOne(t *testing.T) {
	err := validateSpotOnlyDecisions([]kernel.Decision{
		{Symbol: "sol:DezX...", Action: "open_long", Leverage: 2},
	})
	if err == nil {
		t.Fatal("expected error for leverage > 1 on spot-only exchange")
	}
	if !strings.Contains(err.Error(), "leverage") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSpotOnlyDecisionsAllowsLongLifecycle(t *testing.T) {
	err := validateSpotOnlyDecisions([]kernel.Decision{
		{Symbol: "sol:DezX...", Action: "open_long", Leverage: 1},
		{Symbol: "sol:DezX...", Action: "close_long"},
		{Symbol: "sol:DezX...", Action: "hold"},
		{Symbol: "sol:DezX...", Action: "wait"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
