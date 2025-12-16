package analysis

import (
	"math/rand"
	"nofx/database"
	"testing"
	"time"
)

func BenchmarkTradeAnalyzer_Analyze(b *testing.B) {
	// Setup large dataset
	trades := make([]database.TradeRecord, 10000)
	now := time.Now()
	for i := 0; i < 10000; i++ {
		trades[i] = database.TradeRecord{
			Symbol:             "BTCUSDT",
			ProfitPct:          (rand.Float64() - 0.5) * 20, // -10% to +10%
			HoldingTimeSeconds: int64(rand.Intn(3600)),
			CreatedAt:          now.Add(time.Duration(-i) * time.Minute),
		}
	}

	analyzer := NewTradeAnalyzer(&MockTradeProvider{}) // Provider not used in Analyze()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.Analyze(trades)
	}
}
