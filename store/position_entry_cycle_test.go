package store

import (
	"strings"
	"testing"
	"time"
)

func TestFindEntryDecisionCycleForPositionMatchesSymbolAndSideBeforeEntry(t *testing.T) {
	s := newTestPositionStore(t)
	traderID := "trader-1"
	now := time.Now().UTC()
	entryTime := now
	records := []DecisionRecordDB{
		{TraderID: traderID, CycleNumber: 10, Timestamp: now.Add(-60 * time.Second), CreatedAt: now.Add(-60 * time.Second), Success: true, Decisions: `[{"symbol":"ETHUSDT","action":"open_long"}]`},
		{TraderID: traderID, CycleNumber: 11, Timestamp: now.Add(-30 * time.Second), CreatedAt: now.Add(-30 * time.Second), Success: true, Decisions: `[{"symbol":"BTCUSDT","action":"open_short"}]`},
		{TraderID: traderID, CycleNumber: 12, Timestamp: now.Add(-10 * time.Second), CreatedAt: now.Add(-10 * time.Second), Success: true, Decisions: `[{"symbol":"BTCUSDT","action":"open_long"}]`},
		// Cycle 13: a DIFFERENT decision 3 minutes after entry — must NOT be matched
		{TraderID: traderID, CycleNumber: 13, Timestamp: now.Add(3 * time.Minute), CreatedAt: now.Add(3 * time.Minute), Success: true, Decisions: `[{"symbol":"BTCUSDT","action":"open_long"}]`},
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

func TestFindEntryDecisionCycleForPositionGracePeriodMatchesPostEntryDecision(t *testing.T) {
	s := newTestPositionStore(t)
	traderID := "trader-1"
	now := time.Now().UTC()
	entryTime := now
	records := []DecisionRecordDB{
		// Stale cycle from a previous day — should NOT be matched
		{TraderID: traderID, CycleNumber: 100, Timestamp: now.Add(-24 * time.Hour), CreatedAt: now.Add(-24 * time.Hour), Success: true, Decisions: `[{"symbol":"ETHUSDT","action":"open_short"}]`},
		// Correct cycle: decision recorded 15s after execution (typical AI response delay)
		{TraderID: traderID, CycleNumber: 659, Timestamp: now.Add(15 * time.Second), CreatedAt: now.Add(15 * time.Second), Success: true, Decisions: `[{"symbol":"ETHUSDT","action":"open_short"}]`},
	}
	for _, record := range records {
		if err := s.db.Create(&record).Error; err != nil {
			t.Fatalf("create decision record: %v", err)
		}
	}

	got := s.FindEntryDecisionCycleForPosition(traderID, "ETHUSDT", "SHORT", entryTime.UnixMilli())
	if got != 659 {
		t.Fatalf("expected cycle 659 (15s post-entry grace), got %d", got)
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

func TestFindEntryDecisionCycleForPositionRejectsCrossObjectMatch(t *testing.T) {
	s := newTestPositionStore(t)
	// Cycle 10: has XAGUSDT hold + ZECUSDT open_long — should NOT match XAGUSDT open_long
	decoyDecisions := `[{"action":"open_long","symbol":"ZECUSDT"},{"action":"hold","symbol":"XAGUSDT"}]`
	if err := s.db.Create(&DecisionRecordDB{TraderID: "trader-1", CycleNumber: 10, Timestamp: time.UnixMilli(500).UTC(), CreatedAt: time.UnixMilli(500).UTC(), Success: true, Decisions: decoyDecisions}).Error; err != nil {
		t.Fatalf("create decoy record: %v", err)
	}
	// Cycle 20: has the actual XAGUSDT open_long
	realDecisions := `[{"action":"open_long","symbol":"XAGUSDT"}]`
	if err := s.db.Create(&DecisionRecordDB{TraderID: "trader-1", CycleNumber: 20, Timestamp: time.UnixMilli(1500).UTC(), CreatedAt: time.UnixMilli(1500).UTC(), Success: true, Decisions: realDecisions}).Error; err != nil {
		t.Fatalf("create real record: %v", err)
	}
	got := s.FindEntryDecisionCycleForPosition("trader-1", "XAGUSDT", "LONG", time.UnixMilli(2000).UnixMilli())
	if got != 20 {
		t.Fatalf("expected cycle 20 (actual open_long), got %d — cross-object false match", got)
	}
}
