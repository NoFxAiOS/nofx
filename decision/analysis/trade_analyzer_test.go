package analysis

import (
	"nofx/database"
	"testing"
	"time"
)

type MockTradeProvider struct {
	Trades []database.TradeRecord
}

func (m *MockTradeProvider) GetTradesInPeriod(traderID string, start, end time.Time) ([]database.TradeRecord, error) {
	return m.Trades, nil
}

func TestTradeAnalyzer_Analyze(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		trades   []database.TradeRecord
		wantWin  float64
		wantPF   float64
		wantPair string
	}{
		{
			name: "Basic Win/Loss",
			trades: []database.TradeRecord{
				{Symbol: "BTCUSDT", ProfitPct: 10.0, CreatedAt: now},
				{Symbol: "BTCUSDT", ProfitPct: -5.0, CreatedAt: now},
			},
			wantWin:  50.0,
			wantPF:   2.0, // 10 / 5
			wantPair: "BTCUSDT",
		},
		{
			name: "All Wins",
			trades: []database.TradeRecord{
				{Symbol: "ETHUSDT", ProfitPct: 10.0, CreatedAt: now},
				{Symbol: "ETHUSDT", ProfitPct: 10.0, CreatedAt: now},
			},
			wantWin:  100.0,
			wantPF:   999.0, // Infinite
			wantPair: "ETHUSDT",
		},
		{
			name: "All Losses",
			trades: []database.TradeRecord{
				{Symbol: "SOLUSDT", ProfitPct: -10.0, CreatedAt: now},
			},
			wantWin:  0.0,
			wantPF:   0.0,
			wantPair: "SOLUSDT",
		},
		{
			name: "Mixed Pairs",
			trades: []database.TradeRecord{
				{Symbol: "BTCUSDT", ProfitPct: 10.0, CreatedAt: now},
				{Symbol: "ETHUSDT", ProfitPct: -20.0, CreatedAt: now}, // Worse
			},
			wantWin:  50.0,
			wantPF:   0.5,
			wantPair: "BTCUSDT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewTradeAnalyzer(&MockTradeProvider{})
			result := analyzer.Analyze(tt.trades)

			if result.WinRate != tt.wantWin {
				t.Errorf("WinRate = %v, want %v", result.WinRate, tt.wantWin)
			}
			if result.ProfitFactor != tt.wantPF {
				t.Errorf("ProfitFactor = %v, want %v", result.ProfitFactor, tt.wantPF)
			}
			if result.BestPerformingPair != tt.wantPair {
				t.Errorf("BestPerformingPair = %v, want %v", result.BestPerformingPair, tt.wantPair)
			}
		})
	}
}
