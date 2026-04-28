package store

import (
	"path/filepath"
	"testing"
)

func TestDynamicProtectionStateRoundTrip(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "dynamic-protection.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	record := DynamicProtectionRecord{
		TraderID:            "trader-1",
		ExchangeID:          "exchange-1",
		Symbol:              "BTCUSDT",
		Side:                "short",
		PositionFingerprint: "78867.80000000|0.00020000",
		ProtectionType:      "native_trailing",
		RuleFingerprint:     "rule-a",
		CloseRatioPct:       50,
		Status:              "armed",
		ExchangeOrderID:     "algo-1",
	}
	if err := s.SaveDynamicProtectionRecord(record); err != nil {
		t.Fatalf("save dynamic protection record: %v", err)
	}
	state, err := s.LoadDynamicProtectionState()
	if err != nil {
		t.Fatalf("load dynamic protection state: %v", err)
	}
	key := BuildDynamicProtectionKey(record.TraderID, record.ExchangeID, record.Symbol, record.Side, record.PositionFingerprint, record.ProtectionType, record.RuleFingerprint, record.CloseRatioPct)
	got, ok := state.Records[key]
	if !ok {
		t.Fatalf("expected record at key %q, got %+v", key, state.Records)
	}
	if got.Status != "armed" || got.ExchangeOrderID != "algo-1" {
		t.Fatalf("unexpected loaded record: %+v", got)
	}
}

func TestDeleteDynamicProtectionRecordsForInactive(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "dynamic-protection-cleanup.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	records := []DynamicProtectionRecord{
		{TraderID: "trader-1", ExchangeID: "exchange-1", Symbol: "BTCUSDT", Side: "short", PositionFingerprint: "a", ProtectionType: "native_trailing", RuleFingerprint: "r1", CloseRatioPct: 50, Status: "armed"},
		{TraderID: "trader-1", ExchangeID: "exchange-1", Symbol: "ETHUSDT", Side: "long", PositionFingerprint: "b", ProtectionType: "native_trailing", RuleFingerprint: "r2", CloseRatioPct: 100, Status: "armed"},
	}
	for _, record := range records {
		if err := s.SaveDynamicProtectionRecord(record); err != nil {
			t.Fatalf("save dynamic protection record: %v", err)
		}
	}
	if err := s.DeleteDynamicProtectionRecordsForInactive(map[string]struct{}{"ETHUSDT_long": {}}); err != nil {
		t.Fatalf("delete inactive dynamic protection records: %v", err)
	}
	state, err := s.LoadDynamicProtectionState()
	if err != nil {
		t.Fatalf("load dynamic protection state: %v", err)
	}
	if len(state.Records) != 1 {
		t.Fatalf("expected one active record remaining, got %+v", state.Records)
	}
	for _, record := range state.Records {
		if record.Symbol != "ETHUSDT" || record.Side != "long" {
			t.Fatalf("expected only ETHUSDT long record remaining, got %+v", record)
		}
	}
}
