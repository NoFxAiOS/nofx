package market

import (
	"sync"
	"testing"
)

// ==================== WSMonitor 结构测试 ====================

func TestNewWSMonitor_InitializesAllTimeframes(t *testing.T) {
	monitor := NewWSMonitor(100)

	if monitor == nil {
		t.Fatal("NewWSMonitor returned nil")
	}

	// 验证所有11个时间框架都已初始化
	expectedTimeframes := []string{
		"1m", "3m", "5m", "15m", "30m",
		"1h", "2h", "4h", "6h", "12h", "1d",
	}

	monitor.klineMapsMutex.RLock()
	defer monitor.klineMapsMutex.RUnlock()

	if len(monitor.klineDataMaps) != len(expectedTimeframes) {
		t.Errorf("klineDataMaps should have %d entries, got %d",
			len(expectedTimeframes), len(monitor.klineDataMaps))
	}

	for _, tf := range expectedTimeframes {
		if _, exists := monitor.klineDataMaps[tf]; !exists {
			t.Errorf("klineDataMaps should contain '%s' entry", tf)
		}
	}
}

func TestGetKlineDataMap_ValidTimeframes(t *testing.T) {
	monitor := NewWSMonitor(100)

	validTimeframes := []string{
		"1m", "3m", "5m", "15m", "30m",
		"1h", "2h", "4h", "6h", "12h", "1d",
	}

	for _, tf := range validTimeframes {
		t.Run(tf, func(t *testing.T) {
			klineMap := monitor.getKlineDataMap(tf)

			if klineMap == nil {
				t.Errorf("getKlineDataMap(%s) should not return nil", tf)
			}
		})
	}
}

func TestGetKlineDataMap_InvalidTimeframe(t *testing.T) {
	monitor := NewWSMonitor(100)

	invalidTimeframes := []string{
		"2m", "8h", "1w", "invalid", "",
	}

	for _, tf := range invalidTimeframes {
		t.Run(tf, func(t *testing.T) {
			klineMap := monitor.getKlineDataMap(tf)

			if klineMap != nil {
				t.Errorf("getKlineDataMap(%s) should return nil for invalid timeframe", tf)
			}
		})
	}
}

func TestGetKlineDataMap_ThreadSafety(t *testing.T) {
	monitor := NewWSMonitor(100)

	concurrency := 10
	iterations := 100
	var wg sync.WaitGroup

	// 并发读取测试
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			timeframes := []string{"3m", "1h", "4h"}
			for j := 0; j < iterations; j++ {
				tf := timeframes[j%len(timeframes)]
				klineMap := monitor.getKlineDataMap(tf)

				if klineMap == nil {
					t.Errorf("getKlineDataMap(%s) returned nil in concurrent test", tf)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestWSMonitor_StoreAndRetrieveKlines(t *testing.T) {
	monitor := NewWSMonitor(100)

	// 生成测试数据
	testKlines := generateTestKlines(50)
	symbol := "BTCUSDT"
	timeframe := "3m"

	// 获取对应的sync.Map
	klineMap := monitor.getKlineDataMap(timeframe)
	if klineMap == nil {
		t.Fatal("getKlineDataMap returned nil")
	}

	// 存储数据
	klineMap.Store(symbol, testKlines)

	// 读取数据
	value, exists := klineMap.Load(symbol)
	if !exists {
		t.Fatal("Failed to load stored klines")
	}

	retrievedKlines, ok := value.([]Kline)
	if !ok {
		t.Fatal("Failed to convert loaded value to []Kline")
	}

	if len(retrievedKlines) != len(testKlines) {
		t.Errorf("Retrieved klines length = %d; want %d",
			len(retrievedKlines), len(testKlines))
	}
}

func TestWSMonitor_MultipleSymbolsMultipleTimeframes(t *testing.T) {
	monitor := NewWSMonitor(100)

	symbols := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT"}
	timeframes := []string{"3m", "1h", "4h"}

	// 为每个symbol和timeframe组合存储数据
	for _, symbol := range symbols {
		for _, tf := range timeframes {
			klineMap := monitor.getKlineDataMap(tf)
			if klineMap == nil {
				t.Fatalf("getKlineDataMap(%s) returned nil", tf)
			}

			testKlines := generateTestKlines(30)
			klineMap.Store(symbol, testKlines)
		}
	}

	// 验证所有数据都能正确读取
	for _, symbol := range symbols {
		for _, tf := range timeframes {
			klineMap := monitor.getKlineDataMap(tf)
			value, exists := klineMap.Load(symbol)

			if !exists {
				t.Errorf("Failed to load klines for %s:%s", symbol, tf)
				continue
			}

			klines, ok := value.([]Kline)
			if !ok {
				t.Errorf("Failed to convert value to []Kline for %s:%s", symbol, tf)
				continue
			}

			if len(klines) != 30 {
				t.Errorf("Klines for %s:%s has %d entries; want 30",
					symbol, tf, len(klines))
			}
		}
	}
}

// ==================== GetCurrentKlines 测试 ====================

func TestGetCurrentKlines_ValidTimeframe(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 这个测试需要WebSocket连接,在实际环境中运行
	monitor := NewWSMonitor(100)

	// 预先存储测试数据
	testKlines := generateTestKlines(100)
	klineMap := monitor.getKlineDataMap("3m")
	if klineMap != nil {
		klineMap.Store("BTCUSDT", testKlines)
	}

	klines, err := monitor.GetCurrentKlines("BTCUSDT", "3m")

	if err != nil {
		t.Fatalf("GetCurrentKlines failed: %v", err)
	}

	if len(klines) == 0 {
		t.Error("GetCurrentKlines returned empty klines")
	}
}

func TestGetCurrentKlines_InvalidTimeframe(t *testing.T) {
	monitor := NewWSMonitor(100)

	_, err := monitor.GetCurrentKlines("BTCUSDT", "invalid")

	if err == nil {
		t.Error("GetCurrentKlines with invalid timeframe should return error")
	}
}

func TestGetCurrentKlines_DeepCopy(t *testing.T) {
	monitor := NewWSMonitor(100)

	// 存储测试数据
	originalKlines := generateTestKlines(50)
	klineMap := monitor.getKlineDataMap("3m")
	if klineMap == nil {
		t.Fatal("getKlineDataMap returned nil")
	}
	klineMap.Store("TESTUSDT", originalKlines)

	// 获取数据(应该是深拷贝)
	klines1, err1 := monitor.GetCurrentKlines("TESTUSDT", "3m")
	if err1 != nil {
		t.Fatalf("GetCurrentKlines failed: %v", err1)
	}

	klines2, err2 := monitor.GetCurrentKlines("TESTUSDT", "3m")
	if err2 != nil {
		t.Fatalf("GetCurrentKlines failed: %v", err2)
	}

	// 修改第一个返回值
	if len(klines1) > 0 {
		klines1[0].Close = 999999.0
	}

	// 第二个返回值应该不受影响(证明是深拷贝)
	if len(klines2) > 0 && klines2[0].Close == 999999.0 {
		t.Error("GetCurrentKlines should return a deep copy, but modification affected other result")
	}

	// 原始数据也不应该受影响
	value, _ := klineMap.Load("TESTUSDT")
	storedKlines := value.([]Kline)
	if storedKlines[0].Close == 999999.0 {
		t.Error("GetCurrentKlines should return a deep copy, but modification affected stored data")
	}
}

func TestGetCurrentKlines_ConcurrentAccess(t *testing.T) {
	monitor := NewWSMonitor(100)

	// 预存数据
	testKlines := generateTestKlines(100)
	for _, tf := range []string{"3m", "1h", "4h"} {
		klineMap := monitor.getKlineDataMap(tf)
		if klineMap != nil {
			klineMap.Store("BTCUSDT", testKlines)
		}
	}

	concurrency := 20
	iterations := 50
	var wg sync.WaitGroup
	errors := make(chan error, concurrency*iterations)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			timeframes := []string{"3m", "1h", "4h"}
			for j := 0; j < iterations; j++ {
				tf := timeframes[j%len(timeframes)]
				_, err := monitor.GetCurrentKlines("BTCUSDT", tf)
				if err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent GetCurrentKlines error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Errorf("Got %d errors in concurrent access test", errorCount)
	}
}

// ==================== Benchmark 测试 ====================

func BenchmarkGetKlineDataMap(b *testing.B) {
	monitor := NewWSMonitor(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.getKlineDataMap("3m")
	}
}

func BenchmarkGetCurrentKlines(b *testing.B) {
	monitor := NewWSMonitor(100)

	// 预存数据
	testKlines := generateTestKlines(100)
	klineMap := monitor.getKlineDataMap("3m")
	if klineMap != nil {
		klineMap.Store("BTCUSDT", testKlines)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.GetCurrentKlines("BTCUSDT", "3m")
	}
}

func BenchmarkGetCurrentKlines_Concurrent(b *testing.B) {
	monitor := NewWSMonitor(100)

	// 预存数据
	testKlines := generateTestKlines(100)
	klineMap := monitor.getKlineDataMap("3m")
	if klineMap != nil {
		klineMap.Store("BTCUSDT", testKlines)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			monitor.GetCurrentKlines("BTCUSDT", "3m")
		}
	})
}

func BenchmarkWSMonitor_Store(b *testing.B) {
	monitor := NewWSMonitor(100)
	testKlines := generateTestKlines(100)
	klineMap := monitor.getKlineDataMap("3m")

	if klineMap == nil {
		b.Fatal("getKlineDataMap returned nil")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		klineMap.Store("BTCUSDT", testKlines)
	}
}

func BenchmarkWSMonitor_Load(b *testing.B) {
	monitor := NewWSMonitor(100)
	testKlines := generateTestKlines(100)
	klineMap := monitor.getKlineDataMap("3m")

	if klineMap == nil {
		b.Fatal("getKlineDataMap returned nil")
	}

	klineMap.Store("BTCUSDT", testKlines)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		klineMap.Load("BTCUSDT")
	}
}
