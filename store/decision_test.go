package store

import (
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openDecisionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "decision-test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	return db
}

func TestDecisionStore_LogDecisionProtectionSnapshotRoundTrip(t *testing.T) {
	db := openDecisionTestDB(t)
	store := NewDecisionStore(db)
	if err := store.initTables(); err != nil {
		t.Fatalf("initTables failed: %v", err)
	}

	ts := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	record := &DecisionRecord{
		TraderID:       "trader-1",
		CycleNumber:    7,
		Timestamp:      ts,
		CandidateCoins: []string{"BTC", "ETH"},
		ExecutionLog:   []string{"built context", "saved decision"},
		Decisions: []DecisionAction{{
			Action:    "open_long",
			Symbol:    "BTCUSDT",
			Quantity:  0.1,
			Leverage:  5,
			Price:     65000,
			Timestamp: ts,
			Success:   true,
		}},
		ProtectionSnapshot: &ProtectionSnapshot{
			FullTPSL: &ProtectionSnapshotFullTPSL{
				Enabled: true,
				Mode:    "full",
				TakeProfit: ProtectionSnapshotValueSource{
					Mode:  "manual",
					Value: 12.5,
				},
				StopLoss: ProtectionSnapshotValueSource{
					Mode:  "manual",
					Value: 4.5,
				},
			},
			LadderTPSL: &ProtectionSnapshotLadder{
				Enabled:           true,
				Mode:              "ladder",
				TakeProfitEnabled: true,
				StopLossEnabled:   true,
				Rules: []ProtectionSnapshotLadderRule{{
					TakeProfitPct:           10,
					TakeProfitCloseRatioPct: 50,
					StopLossPct:             5,
					StopLossCloseRatioPct:   100,
				}},
			},
			Drawdown: []ProtectionSnapshotDrawdown{{
				MinProfitPct:   8,
				MaxDrawdownPct: 3,
				CloseRatioPct:  50,
				PollIntervalS:  30,
			}},
			BreakEven: &ProtectionSnapshotBreakEven{
				Enabled:      true,
				TriggerMode:  "profit_pct",
				TriggerValue: 6,
				OffsetPct:    0.2,
			},
		},
		Success: true,
	}

	if err := store.LogDecision(record); err != nil {
		t.Fatalf("LogDecision failed: %v", err)
	}
	if record.ID == 0 {
		t.Fatal("expected record ID to be assigned")
	}

	records, err := store.GetLatestRecords("trader-1", 1)
	if err != nil {
		t.Fatalf("GetLatestRecords failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	got := records[0]
	if got.ProtectionSnapshot == nil {
		t.Fatal("expected protection snapshot to round-trip")
	}
	if got.ProtectionSnapshot.FullTPSL == nil || got.ProtectionSnapshot.FullTPSL.TakeProfit.Value != 12.5 {
		t.Fatalf("unexpected full_tp_sl snapshot: %+v", got.ProtectionSnapshot.FullTPSL)
	}
	if got.ProtectionSnapshot.LadderTPSL == nil || len(got.ProtectionSnapshot.LadderTPSL.Rules) != 1 {
		t.Fatalf("unexpected ladder snapshot: %+v", got.ProtectionSnapshot.LadderTPSL)
	}
	if len(got.ProtectionSnapshot.Drawdown) != 1 || got.ProtectionSnapshot.Drawdown[0].PollIntervalS != 30 {
		t.Fatalf("unexpected drawdown snapshot: %+v", got.ProtectionSnapshot.Drawdown)
	}
	if got.ProtectionSnapshot.BreakEven == nil || got.ProtectionSnapshot.BreakEven.TriggerMode != "profit_pct" {
		t.Fatalf("unexpected break-even snapshot: %+v", got.ProtectionSnapshot.BreakEven)
	}
}

func TestDecisionRecordDB_ToRecordWithoutProtectionSnapshot(t *testing.T) {
	dbRecord := &DecisionRecordDB{
		ID:                 1,
		TraderID:           "trader-2",
		CycleNumber:        3,
		Timestamp:          time.Date(2026, 4, 10, 13, 0, 0, 0, time.UTC),
		CandidateCoins:     `[]`,
		ExecutionLog:       `[]`,
		Decisions:          `[]`,
		ProtectionSnapshot: "",
		Success:            true,
	}

	record := dbRecord.toRecord()
	if record == nil {
		t.Fatal("record should not be nil")
	}
	if record.ProtectionSnapshot != nil {
		t.Fatalf("expected nil protection snapshot, got %+v", record.ProtectionSnapshot)
	}
}
