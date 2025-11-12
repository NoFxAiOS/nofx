package market

import (
	"testing"
	"time"
)

// ==================== market.Get() 多时间框架测试 ====================

// 注意: 这些测试需要 WSMonitorCli 已初始化
// 在实际运行时可能需要 mock WebSocket 数据

func TestGet_DefaultConfig(t *testing.T) {
	// 跳过需要WebSocket连接的测试
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 使用默认配置(应该是 3m 和 4h)
	data, err := Get("BTCUSDT")

	if err != nil {
		t.Fatalf("Get() with default config failed: %v", err)
	}

	if data == nil {
		t.Fatal("Get() returned nil data")
	}

	if data.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %s; want BTCUSDT", data.Symbol)
	}

	// 应该有 TimeframeData
	if data.TimeframeData == nil {
		t.Error("TimeframeData should not be nil")
	}

	// 向后兼容性检查
	if data.IntradaySeries == nil {
		t.Error("IntradaySeries should not be nil for backward compatibility")
	}

	if data.LongerTermContext == nil {
		t.Error("LongerTermContext should not be nil for backward compatibility")
	}
}

func TestGet_CustomSingleTimeframe(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &IndicatorConfig{
		Timeframes: []string{"1h"},
		DataPoints: map[string]int{"1h": 30},
	}

	data, err := Get("ETHUSDT", cfg)

	if err != nil {
		t.Fatalf("Get() with single timeframe failed: %v", err)
	}

	if data == nil {
		t.Fatal("Get() returned nil data")
	}

	// 应该有 1h 的数据
	if _, exists := data.TimeframeData["1h"]; !exists {
		t.Error("TimeframeData should contain '1h' entry")
	}

	// 验证数据点数量
	if tfData := data.TimeframeData["1h"]; tfData != nil {
		if tfData.DataPoints != 30 {
			t.Errorf("1h DataPoints = %d; want 30", tfData.DataPoints)
		}
	}
}

func TestGet_MultipleTimeframes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &IndicatorConfig{
		Timeframes: []string{"3m", "1h", "4h"},
		DataPoints: map[string]int{
			"3m": 40,
			"1h": 30,
			"4h": 25,
		},
	}

	data, err := Get("BTCUSDT", cfg)

	if err != nil {
		t.Fatalf("Get() with multiple timeframes failed: %v", err)
	}

	if data == nil {
		t.Fatal("Get() returned nil data")
	}

	// 验证所有时间框架都存在
	expectedTimeframes := []string{"3m", "1h", "4h"}
	for _, tf := range expectedTimeframes {
		if _, exists := data.TimeframeData[tf]; !exists {
			t.Errorf("TimeframeData should contain '%s' entry", tf)
		}
	}

	// 验证每个时间框架的数据点
	if tfData := data.TimeframeData["3m"]; tfData != nil {
		if tfData.DataPoints != 40 {
			t.Errorf("3m DataPoints = %d; want 40", tfData.DataPoints)
		}
	}

	if tfData := data.TimeframeData["1h"]; tfData != nil {
		if tfData.DataPoints != 30 {
			t.Errorf("1h DataPoints = %d; want 30", tfData.DataPoints)
		}
	}

	if tfData := data.TimeframeData["4h"]; tfData != nil {
		if tfData.DataPoints != 25 {
			t.Errorf("4h DataPoints = %d; want 25", tfData.DataPoints)
		}
	}
}

func TestGet_InvalidTimeframeFiltered(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &IndicatorConfig{
		Timeframes: []string{"3m", "invalid", "1h"},
		DataPoints: map[string]int{
			"3m": 40,
			"1h": 30,
		},
	}

	data, err := Get("BTCUSDT", cfg)

	if err != nil {
		t.Fatalf("Get() with invalid timeframe should not error: %v", err)
	}

	// 应该只有有效的时间框架
	if _, exists := data.TimeframeData["invalid"]; exists {
		t.Error("TimeframeData should not contain 'invalid' entry")
	}

	// 有效的时间框架应该存在
	if _, exists := data.TimeframeData["3m"]; !exists {
		t.Error("TimeframeData should contain '3m' entry")
	}

	if _, exists := data.TimeframeData["1h"]; !exists {
		t.Error("TimeframeData should contain '1h' entry")
	}
}

func TestGet_AllTimeframes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	allTimeframes := []string{"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "12h", "1d"}

	cfg := &IndicatorConfig{
		Timeframes: allTimeframes,
		DataPoints: make(map[string]int),
	}

	// 使用默认数据点
	for _, tf := range allTimeframes {
		cfg.DataPoints[tf] = GetDefaultDataPoints(tf)
	}

	// 这个测试可能需要较长时间
	data, err := Get("BTCUSDT", cfg)

	if err != nil {
		t.Fatalf("Get() with all timeframes failed: %v", err)
	}

	if data == nil {
		t.Fatal("Get() returned nil data")
	}

	// 应该有数据(至少部分时间框架)
	if len(data.TimeframeData) == 0 {
		t.Error("TimeframeData should not be empty")
	}

	t.Logf("Successfully fetched %d out of %d timeframes", len(data.TimeframeData), len(allTimeframes))
}

func TestGet_EmptyTimeframesUsesDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &IndicatorConfig{
		Timeframes: []string{}, // 空数组应该使用默认值
		DataPoints: make(map[string]int),
	}

	data, err := Get("BTCUSDT", cfg)

	if err != nil {
		t.Fatalf("Get() with empty timeframes failed: %v", err)
	}

	if data == nil {
		t.Fatal("Get() returned nil data")
	}

	// 应该回退到默认的 3m 和 4h
	if len(data.TimeframeData) == 0 {
		t.Error("TimeframeData should have default timeframes")
	}
}

func TestGet_SymbolNormalization(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 测试小写symbol是否被正确标准化
	testCases := []string{
		"btcusdt",
		"BTCUSDT",
		"BtcUsdt",
	}

	for _, symbol := range testCases {
		t.Run(symbol, func(t *testing.T) {
			data, err := Get(symbol)

			if err != nil {
				t.Fatalf("Get(%s) failed: %v", symbol, err)
			}

			// 所有变体应该被标准化为大写
			if data.Symbol != "BTCUSDT" {
				t.Errorf("Symbol = %s; want BTCUSDT", data.Symbol)
			}
		})
	}
}

func TestGet_CurrentPriceCalculation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	data, err := Get("BTCUSDT")

	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	// 当前价格应该大于0
	if data.CurrentPrice <= 0 {
		t.Errorf("CurrentPrice should be positive, got %f", data.CurrentPrice)
	}

	// EMA20应该有值
	if data.CurrentEMA20 <= 0 {
		t.Errorf("CurrentEMA20 should be positive, got %f", data.CurrentEMA20)
	}

	// RSI7应该在0-100之间
	if data.CurrentRSI7 < 0 || data.CurrentRSI7 > 100 {
		t.Errorf("CurrentRSI7 should be between 0-100, got %f", data.CurrentRSI7)
	}
}

// ==================== 并发安全测试 ====================

func TestGet_ConcurrentCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	concurrency := 5
	done := make(chan bool, concurrency)
	errors := make(chan error, concurrency)

	cfg := &IndicatorConfig{
		Timeframes: []string{"3m", "1h"},
		DataPoints: map[string]int{
			"3m": 40,
			"1h": 30,
		},
	}

	// 并发调用Get函数
	for i := 0; i < concurrency; i++ {
		go func(index int) {
			symbol := "BTCUSDT"
			if index%2 == 0 {
				symbol = "ETHUSDT"
			}

			data, err := Get(symbol, cfg)
			if err != nil {
				errors <- err
				done <- false
				return
			}

			if data == nil {
				errors <- err
				done <- false
				return
			}

			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	successCount := 0
	for i := 0; i < concurrency; i++ {
		select {
		case success := <-done:
			if success {
				successCount++
			}
		case err := <-errors:
			t.Errorf("Concurrent call failed: %v", err)
		case <-time.After(30 * time.Second):
			t.Fatal("Concurrent calls timed out")
		}
	}

	t.Logf("Concurrent test: %d/%d calls succeeded", successCount, concurrency)
}

// ==================== 性能测试 ====================

func BenchmarkGet_SingleTimeframe(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	cfg := &IndicatorConfig{
		Timeframes: []string{"3m"},
		DataPoints: map[string]int{"3m": 40},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Get("BTCUSDT", cfg)
		if err != nil {
			b.Fatalf("Get() failed: %v", err)
		}
	}
}

func BenchmarkGet_ThreeTimeframes(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	cfg := &IndicatorConfig{
		Timeframes: []string{"3m", "1h", "4h"},
		DataPoints: map[string]int{
			"3m": 40,
			"1h": 30,
			"4h": 25,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Get("BTCUSDT", cfg)
		if err != nil {
			b.Fatalf("Get() failed: %v", err)
		}
	}
}

func BenchmarkGet_DefaultConfig(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Get("BTCUSDT")
		if err != nil {
			b.Fatalf("Get() failed: %v", err)
		}
	}
}
