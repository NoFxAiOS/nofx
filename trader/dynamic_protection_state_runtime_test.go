package trader

import (
	"path/filepath"
	"testing"

	"nofx/store"
)

func TestLoadDynamicProtectionStateFromStoreRestoresRuntimeMaps(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "dynamic-protection-runtime.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	traderID := "trader-1"
	records := []store.DynamicProtectionRecord{
		{TraderID: traderID, ExchangeID: "exchange-1", Symbol: "BTCUSDT", Side: "short", PositionFingerprint: "100|2", ProtectionType: "native_partial_trailing", RuleFingerprint: "100|2|5|40|50", CloseRatioPct: 50, Status: "armed", ExchangeOrderID: "algo-partial"},
		{TraderID: traderID, ExchangeID: "exchange-1", Symbol: "ETHUSDT", Side: "long", PositionFingerprint: "200|1", ProtectionType: "native_trailing", RuleFingerprint: "200|1|5|40|100", CloseRatioPct: 100, Status: "armed", ExchangeOrderID: "algo-full"},
		{TraderID: "other-trader", ExchangeID: "exchange-1", Symbol: "SOLUSDT", Side: "long", PositionFingerprint: "50|10", ProtectionType: "native_trailing", RuleFingerprint: "50|10|5|40|100", CloseRatioPct: 100, Status: "armed"},
	}
	for _, record := range records {
		if err := st.SaveDynamicProtectionRecord(record); err != nil {
			t.Fatalf("save dynamic protection record: %v", err)
		}
	}

	at := &AutoTrader{id: traderID, store: st}
	at.loadDynamicProtectionStateFromStore()

	if got := at.getProtectionState("BTCUSDT", "short"); got != "native_partial_trailing_armed" {
		t.Fatalf("expected BTC partial trailing state restored, got %q", got)
	}
	if got := at.getProtectionState("ETHUSDT", "long"); got != "native_trailing_armed" {
		t.Fatalf("expected ETH full trailing state restored, got %q", got)
	}
	if got := at.getProtectionState("SOLUSDT", "long"); got != "" {
		t.Fatalf("expected other trader state ignored, got %q", got)
	}
	if got := at.getDrawdownExecutionFingerprint("BTCUSDT", "short"); got != "100|2|5|40|50" {
		t.Fatalf("expected BTC drawdown fingerprint restored, got %q", got)
	}
	if got := at.getDrawdownExecutionFingerprint("ETHUSDT", "long"); got != "200|1|5|40|100" {
		t.Fatalf("expected ETH drawdown fingerprint restored, got %q", got)
	}
}

func TestLoadDynamicProtectionStateFromStoreSkipsInactiveNativeRecords(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "dynamic-protection-runtime-status.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	traderID := "trader-1"
	records := []store.DynamicProtectionRecord{
		{TraderID: traderID, ExchangeID: "exchange-1", Symbol: "BTCUSDT", Side: "long", PositionFingerprint: "100|1", ProtectionType: "native_trailing", RuleFingerprint: "100|1|5|40|100", CloseRatioPct: 100, Status: "canceled", ExchangeOrderID: "old-algo"},
		{TraderID: traderID, ExchangeID: "exchange-1", Symbol: "ETHUSDT", Side: "long", PositionFingerprint: "200|1", ProtectionType: "managed_drawdown", RuleFingerprint: "200|1|5|40|50", CloseRatioPct: 50, Status: "executed"},
	}
	for _, record := range records {
		if err := st.SaveDynamicProtectionRecord(record); err != nil {
			t.Fatalf("save dynamic protection record: %v", err)
		}
	}

	at := &AutoTrader{id: traderID, store: st}
	at.loadDynamicProtectionStateFromStore()

	if got := at.getProtectionState("BTCUSDT", "long"); got != "" {
		t.Fatalf("expected canceled native trailing not restored, got %q", got)
	}
	if got := at.getDrawdownExecutionFingerprint("BTCUSDT", "long"); got != "" {
		t.Fatalf("expected canceled native trailing fingerprint not restored, got %q", got)
	}
	if got := at.getDrawdownExecutionFingerprint("ETHUSDT", "long"); got != "200|1|5|40|50" {
		t.Fatalf("expected executed managed drawdown fingerprint restored, got %q", got)
	}
}

func TestLoadDynamicProtectionStateFromStoreUsesLatestNativeRecordPerPosition(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "dynamic-protection-runtime-latest.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	traderID := "trader-1"
	records := []store.DynamicProtectionRecord{
		{TraderID: traderID, ExchangeID: "exchange-1", Symbol: "DOGEUSDT", Side: "long", PositionFingerprint: "old", ProtectionType: "native_trailing", RuleFingerprint: "old-full", CloseRatioPct: 100, Status: "armed", UpdatedAt: 1000},
		{TraderID: traderID, ExchangeID: "exchange-1", Symbol: "DOGEUSDT", Side: "long", PositionFingerprint: "new", ProtectionType: "native_partial_trailing", RuleFingerprint: "new-partial", CloseRatioPct: 80, Status: "armed", UpdatedAt: 2000},
	}
	for _, record := range records {
		if err := st.SaveDynamicProtectionRecord(record); err != nil {
			t.Fatalf("save dynamic protection record: %v", err)
		}
	}

	at := &AutoTrader{id: traderID, store: st}
	at.loadDynamicProtectionStateFromStore()

	if got := at.getProtectionState("DOGEUSDT", "long"); got != "native_partial_trailing_armed" {
		t.Fatalf("expected latest native partial state restored, got %q", got)
	}
	if got := at.getDrawdownExecutionFingerprint("DOGEUSDT", "long"); got != "new-partial" {
		t.Fatalf("expected latest native fingerprint restored, got %q", got)
	}
}
