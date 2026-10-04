package trader

import (
	"errors"
	"nofx/store"
	"testing"
)

type rejectedAuthorizationTrader struct{ Trader }

func (*rejectedAuthorizationTrader) CheckTradingAuthorization() error {
	return errors.New("agent mismatch")
}

func TestCycleChecksAuthorizationBeforeFetchingDataOrCallingAI(t *testing.T) {
	// Nil data/model dependencies would panic if the paid context path ran.
	at := &AutoTrader{isRunning: true, trader: &rejectedAuthorizationTrader{}}
	if err := at.runCycle(); err == nil {
		t.Fatal("expected authorization block")
	}
	status := at.GetStatus()
	if status["trading_blocked"] != true || status["trading_error"] != "agent mismatch" {
		t.Fatalf("missing runtime block: %v", status)
	}
}

func TestDecisionExecutionOutcome(t *testing.T) {
	for _, tc := range []struct {
		name          string
		actions       []store.DecisionAction
		initial, want bool
	}{
		{"hold only", []store.DecisionAction{{Action: "hold", Success: true}}, true, true},
		{"all failed with hold", []store.DecisionAction{{Action: "open_long"}, {Action: "close_long"}, {Action: "hold", Success: true}}, true, false},
		{"partial failure", []store.DecisionAction{{Action: "open_long", Success: true}, {Action: "close_long"}}, true, false},
		{"filled", []store.DecisionAction{{Action: "open_long", Success: true}}, true, true},
		{"preserve upstream fault", nil, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := &store.DecisionRecord{Success: tc.initial, Decisions: tc.actions}
			summarizeDecisionExecution(record)
			if record.Success != tc.want {
				t.Fatalf("record=%+v", record)
			}
			if tc.initial && !tc.want && record.ErrorMessage == "" {
				t.Fatal("missing execution failure summary")
			}
		})
	}
}
