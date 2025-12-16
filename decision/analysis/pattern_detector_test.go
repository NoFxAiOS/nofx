package analysis

import (
	"testing"
)

func TestPatternDetector_DetectFailurePatterns(t *testing.T) {
	detector := NewPatternDetector()

	tests := []struct {
		name     string
		analysis *TradeAnalysisResult
		wantType string
	}{
		{
			name: "High Leverage Risk",
			analysis: &TradeAnalysisResult{
				ProfitFactor:       1.0,
				LoseStreak:         4,
				LosingTrades:       5,
				AverageLossPerLoss: 100,
			},
			wantType: "high_risk_streaks",
		},
		{
			name: "Poor Pair Selection",
			analysis: &TradeAnalysisResult{
				WorstPerformingPair: "JUNKUSDT",
				TradeByPairStats: map[string]*PairStats{
					"JUNKUSDT": {Symbol: "JUNKUSDT", AvgProfit: -5.0, TotalTrades: 10},
				},
			},
			wantType: "poor_pair_selection",
		},
		{
			name: "Poor Timing",
			analysis: &TradeAnalysisResult{
				WinRate:         30.0,
				BestTradingHour: 10,
				TradeByHourStats: map[int]*HourStats{
					10: {Hour: 10, WinRate: 80.0, TotalTrades: 5},
				},
				TotalTrades: 20,
			},
			wantType: "poor_timing",
		},
		{
			name:     "No Patterns",
			analysis: &TradeAnalysisResult{ProfitFactor: 2.0, LoseStreak: 1},
			wantType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := detector.DetectFailurePatterns(tt.analysis)
			
			if tt.wantType == "" {
				if len(patterns) > 0 {
					t.Errorf("Expected no patterns, got %d", len(patterns))
				}
			} else {
				found := false
				for _, p := range patterns {
					if p.PatternType == tt.wantType {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected pattern %s, but not found", tt.wantType)
				}
			}
		})
	}
}
