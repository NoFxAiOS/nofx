package market

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 确保 WSMonitorCli 在测试开始前初始化
var initOnce sync.Once

func ensureWSMonitorInitialized() {
	initOnce.Do(func() {
		if WSMonitorCli == nil {
			WSMonitorCli = NewWSMonitor(100) // 使用默认批处理大小
		}
	})
}

// BenchmarkGet1Timeframe 测试单时间框架性能
func BenchmarkGet1Timeframe(b *testing.B) {
	ensureWSMonitorInitialized()
	ensureWSMonitorInitialized()

	symbol := "BTCUSDT"
	config := &IndicatorConfig{
		Timeframes: []string{"3m"},
		Indicators: []string{"ema", "macd"},
		DataPoints: map[string]int{"3m": 40},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Get(symbol, config)
		if err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}

// BenchmarkGet3Timeframes 测试 3 个时间框架性能
func BenchmarkGet3Timeframes(b *testing.B) {
	ensureWSMonitorInitialized()
	ensureWSMonitorInitialized()

	symbol := "BTCUSDT"
	config := &IndicatorConfig{
		Timeframes: []string{"3m", "1h", "4h"},
		Indicators: []string{"ema", "macd", "rsi"},
		DataPoints: map[string]int{
			"3m": 40,
			"1h": 30,
			"4h": 25,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Get(symbol, config)
		if err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}

// BenchmarkGet5Timeframes 测试 5 个时间框架性能
func BenchmarkGet5Timeframes(b *testing.B) {
	ensureWSMonitorInitialized()
	symbol := "BTCUSDT"
	config := &IndicatorConfig{
		Timeframes: []string{"1m", "5m", "15m", "1h", "4h"},
		Indicators: []string{"ema", "macd", "rsi", "atr"},
		DataPoints: map[string]int{
			"1m":  50,
			"5m":  45,
			"15m": 40,
			"1h":  30,
			"4h":  25,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Get(symbol, config)
		if err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}

// BenchmarkGet11Timeframes 测试所有 11 个时间框架性能
func BenchmarkGet11Timeframes(b *testing.B) {
	ensureWSMonitorInitialized()
	symbol := "BTCUSDT"
	config := &IndicatorConfig{
		Timeframes: []string{
			"1m", "3m", "5m", "15m", "30m",
			"1h", "2h", "4h", "6h", "12h", "1d",
		},
		Indicators: []string{"ema", "macd", "rsi", "atr", "volume"},
		DataPoints: map[string]int{
			"1m": 50, "3m": 48, "5m": 45, "15m": 42, "30m": 40,
			"1h": 35, "2h": 32, "4h": 30, "6h": 28, "12h": 25, "1d": 20,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Get(symbol, config)
		if err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}

// BenchmarkFormat 测试 AI 提示词生成性能
func BenchmarkFormat(b *testing.B) {
	ensureWSMonitorInitialized()
	symbol := "BTCUSDT"
	config := &IndicatorConfig{
		Timeframes: []string{"3m", "1h", "4h"},
		Indicators: []string{"ema", "macd", "rsi"},
		DataPoints: map[string]int{
			"3m": 40,
			"1h": 30,
			"4h": 25,
		},
	}

	// 先获取数据
	data, err := Get(symbol, config)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Format(data)
	}
}

// TestPerformance1Timeframe 验证单时间框架性能
func TestPerformance1Timeframe(t *testing.T) {
	ensureWSMonitorInitialized()
	ensureWSMonitorInitialized()
	symbol := "BTCUSDT"
	config := &IndicatorConfig{
		Timeframes: []string{"3m"},
		Indicators: []string{"ema", "macd"},
		DataPoints: map[string]int{"3m": 40},
	}

	start := time.Now()
	_, err := Get(symbol, config)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Less(t, elapsed, 1*time.Second, "Single timeframe should complete within 1s")

	t.Logf("Single timeframe performance: %v", elapsed)
}

// TestPerformance3Timeframes 验证 3 个时间框架性能
func TestPerformance3Timeframes(t *testing.T) {
	ensureWSMonitorInitialized()
	ensureWSMonitorInitialized()
	symbol := "BTCUSDT"
	config := &IndicatorConfig{
		Timeframes: []string{"3m", "1h", "4h"},
		Indicators: []string{"ema", "macd", "rsi"},
		DataPoints: map[string]int{
			"3m": 40,
			"1h": 30,
			"4h": 25,
		},
	}

	start := time.Now()
	_, err := Get(symbol, config)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Less(t, elapsed, 1500*time.Millisecond, "3 timeframes should complete within 1.5s")

	t.Logf("3 timeframes performance: %v", elapsed)
}

// TestPerformance5Timeframes 验证 5 个时间框架性能
func TestPerformance5Timeframes(t *testing.T) {
	ensureWSMonitorInitialized()
	ensureWSMonitorInitialized()
	symbol := "BTCUSDT"
	config := &IndicatorConfig{
		Timeframes: []string{"1m", "5m", "15m", "1h", "4h"},
		Indicators: []string{"ema", "macd", "rsi", "atr"},
		DataPoints: map[string]int{
			"1m":  50,
			"5m":  45,
			"15m": 40,
			"1h":  30,
			"4h":  25,
		},
	}

	start := time.Now()
	_, err := Get(symbol, config)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Less(t, elapsed, 2*time.Second, "5 timeframes should complete within 2s")

	t.Logf("5 timeframes performance: %v", elapsed)
}

// TestPerformance11Timeframes 验证所有 11 个时间框架性能
func TestPerformance11Timeframes(t *testing.T) {
	ensureWSMonitorInitialized()
	ensureWSMonitorInitialized()
	symbol := "BTCUSDT"
	config := &IndicatorConfig{
		Timeframes: []string{
			"1m", "3m", "5m", "15m", "30m",
			"1h", "2h", "4h", "6h", "12h", "1d",
		},
		Indicators: []string{"ema", "macd", "rsi", "atr", "volume"},
		DataPoints: map[string]int{
			"1m": 50, "3m": 48, "5m": 45, "15m": 42, "30m": 40,
			"1h": 35, "2h": 32, "4h": 30, "6h": 28, "12h": 25, "1d": 20,
		},
	}

	start := time.Now()
	_, err := Get(symbol, config)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Less(t, elapsed, 3*time.Second, "11 timeframes should complete within 3s")

	t.Logf("11 timeframes performance: %v", elapsed)
}

// TestConcurrentTraders 测试并发交易员
func TestConcurrentTraders(t *testing.T) {
	ensureWSMonitorInitialized()
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	symbol := "BTCUSDT"
	config := &IndicatorConfig{
		Timeframes: []string{"3m", "1h", "4h"},
		Indicators: []string{"ema", "macd", "rsi"},
		DataPoints: map[string]int{
			"3m": 40,
			"1h": 30,
			"4h": 25,
		},
	}

	numTraders := 5
	start := time.Now()

	// 并发运行多个交易员
	errors := make(chan error, numTraders)
	for i := 0; i < numTraders; i++ {
		go func(id int) {
			_, err := Get(symbol, config)
			errors <- err
		}(i)
	}

	// 收集结果
	for i := 0; i < numTraders; i++ {
		err := <-errors
		require.NoError(t, err, "Trader %d failed", i)
	}

	elapsed := time.Since(start)
	assert.Less(t, elapsed, 5*time.Second, "5 concurrent traders should complete within 5s")

	t.Logf("5 concurrent traders performance: %v", elapsed)
}

// TestMemoryUsage 测试内存使用
func TestMemoryUsage(t *testing.T) {
	ensureWSMonitorInitialized()
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	symbol := "BTCUSDT"
	config := &IndicatorConfig{
		Timeframes: []string{
			"1m", "3m", "5m", "15m", "30m",
			"1h", "2h", "4h", "6h", "12h", "1d",
		},
		Indicators: []string{"ema", "macd", "rsi", "atr", "volume"},
		DataPoints: map[string]int{
			"1m": 50, "3m": 48, "5m": 45, "15m": 42, "30m": 40,
			"1h": 35, "2h": 32, "4h": 30, "6h": 28, "12h": 25, "1d": 20,
		},
	}

	// 多次获取数据
	iterations := 10
	for i := 0; i < iterations; i++ {
		_, err := Get(symbol, config)
		require.NoError(t, err)
	}

	// 注意: 实际内存使用需要通过外部工具测量
	t.Log("Memory usage test completed - use -benchmem to see detailed memory stats")
}

// TestFormatPerformance 测试 AI 提示词生成性能
func TestFormatPerformance(t *testing.T) {
	ensureWSMonitorInitialized()
	symbol := "BTCUSDT"
	config := &IndicatorConfig{
		Timeframes: []string{"3m", "1h", "4h"},
		Indicators: []string{"ema", "macd", "rsi"},
		DataPoints: map[string]int{
			"3m": 40,
			"1h": 30,
			"4h": 25,
		},
	}

	// 获取数据
	data, err := Get(symbol, config)
	require.NoError(t, err)

	// 测试格式化性能
	start := time.Now()
	iterations := 100
	for i := 0; i < iterations; i++ {
		_ = Format(data)
	}
	elapsed := time.Since(start)

	avgTime := elapsed / time.Duration(iterations)
	assert.Less(t, avgTime, 10*time.Millisecond, "Format should complete within 10ms on average")

	t.Logf("Format performance: %v per iteration (average over %d iterations)", avgTime, iterations)
}

// TestConcurrentGetAndFormat 测试并发数据获取和格式化
func TestConcurrentGetAndFormat(t *testing.T) {
	ensureWSMonitorInitialized()
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	symbol := "BTCUSDT"
	config := &IndicatorConfig{
		Timeframes: []string{"3m", "1h", "4h"},
		Indicators: []string{"ema", "macd", "rsi"},
		DataPoints: map[string]int{
			"3m": 40,
			"1h": 30,
			"4h": 25,
		},
	}

	numWorkers := 10
	start := time.Now()

	// 并发获取和格式化
	errors := make(chan error, numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func(id int) {
			data, err := Get(symbol, config)
			if err != nil {
				errors <- err
				return
			}
			_ = Format(data)
			errors <- nil
		}(i)
	}

	// 收集结果
	for i := 0; i < numWorkers; i++ {
		err := <-errors
		require.NoError(t, err, "Worker %d failed", i)
	}

	elapsed := time.Since(start)
	assert.Less(t, elapsed, 10*time.Second, "10 concurrent workers should complete within 10s")

	t.Logf("10 concurrent Get+Format: %v", elapsed)
}

// TestDataConsistencyUnderLoad 测试高负载下数据一致性
func TestDataConsistencyUnderLoad(t *testing.T) {
	ensureWSMonitorInitialized()
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	symbol := "BTCUSDT"
	config := &IndicatorConfig{
		Timeframes: []string{"3m", "1h"},
		Indicators: []string{"ema", "macd"},
		DataPoints: map[string]int{
			"3m": 40,
			"1h": 30,
		},
	}

	// 并发获取数据
	numRequests := 20
	results := make(chan *Data, numRequests)
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			data, err := Get(symbol, config)
			if err != nil {
				errors <- err
				return
			}
			results <- data
			errors <- nil
		}()
	}

	// 收集结果
	var firstData *Data
	for i := 0; i < numRequests; i++ {
		err := <-errors
		require.NoError(t, err)

		if i == 0 {
			firstData = <-results
		} else {
			data := <-results
			// 验证所有请求返回相同的时间框架数量
			assert.Equal(t, len(firstData.TimeframeData), len(data.TimeframeData),
				"All requests should return same number of timeframes")
		}
	}

	t.Logf("Data consistency verified under %d concurrent requests", numRequests)
}
