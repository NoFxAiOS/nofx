package paper

import (
	"testing"
	"time"
)

func TestPaperTraderOpenAndProtect(t *testing.T) {
	pt := NewTrader()
	pt.SetPrice("BTCUSDT", 100)

	if _, err := pt.OpenLong("BTCUSDT", 1, 5); err != nil {
		t.Fatalf("expected open long success, got %v", err)
	}
	if err := pt.SetStopLoss("BTCUSDT", "LONG", 1, 98); err != nil {
		t.Fatalf("expected stop-loss success, got %v", err)
	}
	if err := pt.SetTakeProfit("BTCUSDT", "LONG", 1, 105); err != nil {
		t.Fatalf("expected take-profit success, got %v", err)
	}

	positions, err := pt.GetPositions()
	if err != nil {
		t.Fatalf("expected positions, got %v", err)
	}
	if len(positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(positions))
	}

	orders, err := pt.GetOpenOrders("BTCUSDT")
	if err != nil {
		t.Fatalf("expected orders, got %v", err)
	}
	if len(orders) != 2 {
		t.Fatalf("expected 2 protection orders, got %d", len(orders))
	}
}

func TestPaperTraderClosePosition(t *testing.T) {
	pt := NewTrader()
	pt.SetPrice("ETHUSDT", 200)

	if _, err := pt.OpenShort("ETHUSDT", 2, 3); err != nil {
		t.Fatalf("expected open short success, got %v", err)
	}
	if err := pt.SetStopLoss("ETHUSDT", "SHORT", 2, 205); err != nil {
		t.Fatalf("expected stop-loss success, got %v", err)
	}
	if err := pt.SetTakeProfit("ETHUSDT", "SHORT", 2, 190); err != nil {
		t.Fatalf("expected take-profit success, got %v", err)
	}
	pt.SetPrice("ETHUSDT", 190)
	if _, err := pt.CloseShort("ETHUSDT", 0); err != nil {
		t.Fatalf("expected close short success, got %v", err)
	}

	positions, _ := pt.GetPositions()
	if len(positions) != 0 {
		t.Fatalf("expected no positions after close, got %d", len(positions))
	}
	orders, err := pt.GetOpenOrders("ETHUSDT")
	if err != nil {
		t.Fatalf("expected open orders query success, got %v", err)
	}
	if len(orders) != 0 {
		t.Fatalf("expected no open protection orders after close, got %d", len(orders))
	}
	closed, err := pt.GetClosedPnL(time.Time{}, 10)
	if err != nil {
		t.Fatalf("expected closed pnl list, got %v", err)
	}
	if len(closed) != 1 {
		t.Fatalf("expected 1 closed pnl record, got %d", len(closed))
	}
}
