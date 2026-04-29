package store

import (
	"strings"
	"testing"
	"time"
)

func TestFindEntryDecisionCycleForPositionMatchesSymbolAndSideBeforeEntry(t *testing.T) {
	s := newTestPositionStore(t)
	traderID := "trader-1"
	entryTime := time.UnixMilli(2000).UTC()
	records := []DecisionRecordDB{
		{TraderID: traderID, CycleNumber: 10, Timestamp: time.UnixMilli(1000).UTC(), CreatedAt: time.UnixMilli(1000).UTC(), Success: true, Decisions: `[{"symbol":"ETHUSDT","action":"open_long"}]`},
		{TraderID: traderID, CycleNumber: 11, Timestamp: time.UnixMilli(1500).UTC(), CreatedAt: time.UnixMilli(1500).UTC(), Success: true, Decisions: `[{"symbol":"BTCUSDT","action":"open_short"}]`},
		{TraderID: traderID, CycleNumber: 12, Timestamp: time.UnixMilli(1800).UTC(), CreatedAt: time.UnixMilli(1800).UTC(), Success: true, Decisions: `[{"symbol":"BTCUSDT","action":"open_long"}]`},
		{TraderID: traderID, CycleNumber: 13, Timestamp: time.UnixMilli(2500).UTC(), CreatedAt: time.UnixMilli(2500).UTC(), Success: true, Decisions: `[{"symbol":"BTCUSDT","action":"open_long"}]`},
	}
	for _, record := range records {
		if err := s.db.Create(&record).Error; err != nil {
			t.Fatalf("create decision record: %v", err)
		}
	}

	got := s.FindEntryDecisionCycleForPosition(traderID, "BTCUSDT", "LONG", entryTime.UnixMilli())
	if got != 12 {
		t.Fatalf("expected cycle 12, got %d", got)
	}
}

func TestBackfillEntryDecisionCycleUpdatesExistingCycle(t *testing.T) {
	s := newTestPositionStore(t)
	pos := &TraderPosition{TraderID: "trader-1", ExchangeID: "exchange-1", Symbol: "BTCUSDT", Side: "LONG", Quantity: 0.001, EntryPrice: 77000, EntryTime: 1, Status: "OPEN"}
	if err := s.CreateOpenPosition(pos); err != nil {
		t.Fatalf("create position: %v", err)
	}
	if err := s.BackfillEntryDecisionCycle(pos.ID, 7841); err != nil {
		t.Fatalf("backfill entry cycle: %v", err)
	}
	got, err := s.GetOpenPositionBySymbol("trader-1", "BTCUSDT", "LONG")
	if err != nil {
		t.Fatalf("get position: %v", err)
	}
	if got.EntryDecisionCycle != 7841 {
		t.Fatalf("expected cycle 7841, got %d", got.EntryDecisionCycle)
	}
	if err := s.BackfillEntryDecisionCycle(pos.ID, 9999); err != nil {
		t.Fatalf("second backfill entry cycle: %v", err)
	}
	got, err = s.GetOpenPositionBySymbol("trader-1", "BTCUSDT", "LONG")
	if err != nil {
		t.Fatalf("get position: %v", err)
	}
	if got.EntryDecisionCycle != 9999 {
		t.Fatalf("expected existing cycle updated, got %d", got.EntryDecisionCycle)
	}
}

func TestFindEntryDecisionCycleForPositionIgnoresFailedRecords(t *testing.T) {
	s := newTestPositionStore(t)
	traderID := "trader-1"
	for _, record := range []DecisionRecordDB{
		{TraderID: traderID, CycleNumber: 20, Timestamp: time.UnixMilli(1000).UTC(), CreatedAt: time.UnixMilli(1000).UTC(), Success: false, Decisions: `[{"symbol":"BTCUSDT","action":"open_long"}]`, ErrorMessage: "rejected"},
		{TraderID: traderID, CycleNumber: 19, Timestamp: time.UnixMilli(900).UTC(), CreatedAt: time.UnixMilli(900).UTC(), Success: true, Decisions: `[{"symbol":"BTCUSDT","action":"open_long"}]`},
	} {
		if err := s.db.Create(&record).Error; err != nil {
			t.Fatalf("create decision record: %v", err)
		}
	}
	got := s.FindEntryDecisionCycleForPosition(traderID, "BTCUSDT", "LONG", time.UnixMilli(2000).UnixMilli())
	if got != 19 {
		t.Fatalf("expected latest successful cycle, got %d", got)
	}
}

func TestBackfillEntryDecisionCycleUpdatesIncorrectExistingCycle(t *testing.T) {
	s := newTestPositionStore(t)
	pos := &TraderPosition{TraderID: "trader-1", ExchangeID: "exchange-1", Symbol: "DOGEUSDT", Side: "SHORT", Quantity: 630, EntryPrice: 0.10342, EntryTime: 1, EntryDecisionCycle: 7836, Status: "OPEN"}
	if err := s.CreateOpenPosition(pos); err != nil {
		t.Fatalf("create position: %v", err)
	}
	if err := s.BackfillEntryDecisionCycle(pos.ID, 7965); err != nil {
		t.Fatalf("backfill entry cycle: %v", err)
	}
	got, err := s.GetOpenPositionBySymbol("trader-1", "DOGEUSDT", "SHORT")
	if err != nil {
		t.Fatalf("get position: %v", err)
	}
	if got.EntryDecisionCycle != 7965 {
		t.Fatalf("expected cycle corrected to 7965, got %d", got.EntryDecisionCycle)
	}
}

func TestFindEntryDecisionCycleForPositionFallsForwardForLateSync(t *testing.T) {
	s := newTestPositionStore(t)
	traderID := "trader-1"
	entryTime := time.UnixMilli(2000).UTC()
	if err := s.db.Create(&DecisionRecordDB{TraderID: traderID, CycleNumber: 41, Timestamp: time.UnixMilli(3000).UTC(), CreatedAt: time.UnixMilli(3000).UTC(), Success: true, Decisions: `[{"symbol":"DOGEUSDT","action":"open_short"}]`}).Error; err != nil {
		t.Fatalf("create decision record: %v", err)
	}
	got := s.FindEntryDecisionCycleForPosition(traderID, "DOGEUSDT", "SHORT", entryTime.UnixMilli())
	if got != 41 {
		t.Fatalf("expected nearest forward cycle 41, got %d", got)
	}
}

func TestFindEntryDecisionCycleForPositionHandlesPrettyJSON(t *testing.T) {
	s := newTestPositionStore(t)
	decisions := `[
  {
    "symbol": "BTCUSDT",
    "action": "open_long"
  }
]`
	if err := s.db.Create(&DecisionRecordDB{TraderID: "trader-1", CycleNumber: 30, Timestamp: time.UnixMilli(1000).UTC(), CreatedAt: time.UnixMilli(1000).UTC(), Success: true, Decisions: decisions}).Error; err != nil {
		t.Fatalf("create decision record: %v", err)
	}
	got := s.FindEntryDecisionCycleForPosition("trader-1", "BTCUSDT", "LONG", time.UnixMilli(2000).UnixMilli())
	if got != 30 {
		t.Fatalf("expected cycle 30, got %d; decisions=%s", got, strings.ReplaceAll(decisions, "\n", " "))
	}
}
