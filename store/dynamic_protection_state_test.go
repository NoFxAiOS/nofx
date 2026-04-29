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

func TestDynamicProtectionStateKeepsNativeTrailingTiersArmed(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "dynamic-protection-native-tiers.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	firstRecord := DynamicProtectionRecord{
		TraderID:            "trader-1",
		ExchangeID:          "exchange-1",
		Symbol:              "DOGEUSDT",
		Side:                "long",
		PositionFingerprint: "0.09930000|650.00000000",
		ProtectionType:      "native_partial_trailing",
		RuleFingerprint:     "rule-1",
		CloseRatioPct:       40,
		Status:              "armed",
		ExchangeOrderID:     "algo-1",
		UpdatedAt:           1000,
	}
	secondRecord := DynamicProtectionRecord{
		TraderID:            "trader-1",
		ExchangeID:          "exchange-1",
		Symbol:              "DOGEUSDT",
		Side:                "long",
		PositionFingerprint: "0.09930000|650.00000000",
		ProtectionType:      "native_partial_trailing",
		RuleFingerprint:     "rule-2",
		CloseRatioPct:       60,
		Status:              "armed",
		ExchangeOrderID:     "algo-2",
		UpdatedAt:           2000,
	}
	if err := s.SaveDynamicProtectionRecord(firstRecord); err != nil {
		t.Fatalf("save first record: %v", err)
	}
	if err := s.SaveDynamicProtectionRecord(secondRecord); err != nil {
		t.Fatalf("save second record: %v", err)
	}
	state, err := s.LoadDynamicProtectionState()
	if err != nil {
		t.Fatalf("load dynamic protection state: %v", err)
	}
	firstKey := BuildDynamicProtectionKey(firstRecord.TraderID, firstRecord.ExchangeID, firstRecord.Symbol, firstRecord.Side, firstRecord.PositionFingerprint, firstRecord.ProtectionType, firstRecord.RuleFingerprint, firstRecord.CloseRatioPct)
	secondKey := BuildDynamicProtectionKey(secondRecord.TraderID, secondRecord.ExchangeID, secondRecord.Symbol, secondRecord.Side, secondRecord.PositionFingerprint, secondRecord.ProtectionType, secondRecord.RuleFingerprint, secondRecord.CloseRatioPct)
	if got := state.Records[firstKey].Status; got != "armed" {
		t.Fatalf("expected first native tier still armed, got %q", got)
	}
	if got := state.Records[secondKey].Status; got != "armed" {
		t.Fatalf("expected second native tier armed, got %q", got)
	}
}

func TestDynamicProtectionStateKeepsBreakEvenSeparateFromNativeOwner(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "dynamic-protection-be-native.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	nativeRecord := DynamicProtectionRecord{TraderID: "trader-1", ExchangeID: "exchange-1", Symbol: "DOGEUSDT", Side: "long", PositionFingerprint: "pos", ProtectionType: "native_partial_trailing", RuleFingerprint: "native", CloseRatioPct: 80, Status: "armed", UpdatedAt: 1000}
	breakEvenRecord := DynamicProtectionRecord{TraderID: "trader-1", ExchangeID: "exchange-1", Symbol: "DOGEUSDT", Side: "long", PositionFingerprint: "pos", ProtectionType: "break_even_stop", RuleFingerprint: "be", Status: "armed", UpdatedAt: 2000}
	if err := s.SaveDynamicProtectionRecord(nativeRecord); err != nil {
		t.Fatalf("save native record: %v", err)
	}
	if err := s.SaveDynamicProtectionRecord(breakEvenRecord); err != nil {
		t.Fatalf("save break-even record: %v", err)
	}
	state, err := s.LoadDynamicProtectionState()
	if err != nil {
		t.Fatalf("load dynamic protection state: %v", err)
	}
	nativeKey := BuildDynamicProtectionKey(nativeRecord.TraderID, nativeRecord.ExchangeID, nativeRecord.Symbol, nativeRecord.Side, nativeRecord.PositionFingerprint, nativeRecord.ProtectionType, nativeRecord.RuleFingerprint, nativeRecord.CloseRatioPct)
	breakEvenKey := BuildDynamicProtectionKey(breakEvenRecord.TraderID, breakEvenRecord.ExchangeID, breakEvenRecord.Symbol, breakEvenRecord.Side, breakEvenRecord.PositionFingerprint, breakEvenRecord.ProtectionType, breakEvenRecord.RuleFingerprint, breakEvenRecord.CloseRatioPct)
	if got := state.Records[nativeKey].Status; got != "armed" {
		t.Fatalf("expected native owner still armed, got %q", got)
	}
	if got := state.Records[breakEvenKey].Status; got != "armed" {
		t.Fatalf("expected break-even owner armed, got %q", got)
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
