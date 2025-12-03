package decision

import (
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"testing"
	"time"
)

// TestKellyStopManagerEnhancedBasic 测试增强版Kelly管理器基本功能
func TestKellyStopManagerEnhancedBasic(t *testing.T) {
	// 创建临时数据文件
	tempFile := "/tmp/test_kelly_enhanced.json"
	defer os.Remove(tempFile)

	// 创建增强版Kelly管理器
	ksm := NewKellyStopManagerEnhanced(tempFile)

	// 测试基本配置
	config := ksm.GetConfig()
	if config.KellyRatioAdjustment != 0.5 {
		t.Errorf("期望Kelly调整系数为0.5，实际为%f", config.KellyRatioAdjustment)
	}
	if config.MaxTakeProfitMultiplier != 3.0 {
		t.Errorf("期望最大止盈倍数为3.0，实际为%f", config.MaxTakeProfitMultiplier)
	}

	// 测试更新历史统计
	symbol := "BTCUSDT"

	// 添加一些交易记录
	ksm.UpdateHistoricalStatsEnhanced(symbol, true, 0.15, 3600)  // 盈利15%，持仓1小时
	ksm.UpdateHistoricalStatsEnhanced(symbol, false, -0.08, 1800) // 亏损8%，持仓30分钟
	ksm.UpdateHistoricalStatsEnhanced(symbol, true, 0.22, 7200)  // 盈利22%，持仓2小时
	ksm.UpdateHistoricalStatsEnhanced(symbol, true, 0.18, 5400)  // 盈利18%，持仓1.5小时
	ksm.UpdateHistoricalStatsEnhanced(symbol, false, -0.12, 2700) // 亏损12%，持仓45分钟

	// 验证统计数据
	stats := ksm.GetHistoricalStats(symbol)
	if stats == nil {
		t.Fatal("无法获取统计数据")
	}

	if stats.TotalTrades != 5 {
		t.Errorf("期望总交易数为5，实际为%d", stats.TotalTrades)
	}

	if stats.ProfitableTrades != 3 {
		t.Errorf("期望盈利交易数为3，实际为%d", stats.ProfitableTrades)
	}

	expectedWinRate := 3.0 / 5.0
	if math.Abs(stats.WinRate-expectedWinRate) > 0.01 {
		t.Errorf("期望胜率为%.2f，实际为%.2f", expectedWinRate, stats.WinRate)
	}

	if len(stats.TradeHistory) != 5 {
		t.Errorf("期望交易历史记录数为5，实际为%d", len(stats.TradeHistory))
	}

	// 验证加权胜率计算
	if stats.WeightedWinRate <= 0 || stats.WeightedWinRate > 1 {
		t.Errorf("加权胜率应该在0-1之间，实际为%.4f", stats.WeightedWinRate)
	}

	// 验证波动率计算
	if stats.Volatility <= 0 {
		t.Errorf("期望波动率大于0，实际为%.4f", stats.Volatility)
	}

	t.Logf("✅ [%s] 基本统计验证通过: 胜率=%.2f%%, 加权胜率=%.2f%%, 波动率=%.2f%%",
		symbol, stats.WinRate*100, stats.WeightedWinRate*100, stats.Volatility*100)
}

// TestPositionPeakTracking 测试持仓峰值追踪功能
func TestPositionPeakTracking(t *testing.T) {
	tempFile := "/tmp/test_peak_tracking.json"
	defer os.Remove(tempFile)

	ksm := NewKellyStopManagerEnhanced(tempFile)
	symbol := "ETHUSDT"

	// 测试峰值更新
	ksm.UpdatePositionPeak(symbol, 0.05)  // 盈利5%
	peak1 := ksm.GetPositionPeak(symbol)
	if peak1 != 0.05 {
		t.Errorf("期望峰值为0.05，实际为%f", peak1)
	}

	// 更新到更高峰值
	ksm.UpdatePositionPeak(symbol, 0.12) // 盈利12%
	peak2 := ksm.GetPositionPeak(symbol)
	if peak2 != 0.12 {
		t.Errorf("期望峰值为0.12，实际为%f", peak2)
	}

	// 更新到较低值，峰值应该保持不变
	ksm.UpdatePositionPeak(symbol, 0.08) // 盈利8%
	peak3 := ksm.GetPositionPeak(symbol)
	if peak3 != 0.12 {
		t.Errorf("期望峰值保持0.12，实际为%f", peak3)
	}

	// 测试清除峰值
	ksm.ClearPositionPeak(symbol)
	peak4 := ksm.GetPositionPeak(symbol)
	if peak4 != 0 {
		t.Errorf("期望清除后峰值为0，实际为%f", peak4)
	}

	t.Log("✅ 持仓峰值追踪功能验证通过")
}

// TestTimeDecayWeighting 测试时间衰减权重
func TestTimeDecayWeighting(t *testing.T) {
	tempFile := "/tmp/test_time_decay.json"
	defer os.Remove(tempFile)

	ksm := NewKellyStopManagerEnhanced(tempFile)

	// 添加历史交易（模拟不同时间的交易）
	now := time.Now().Unix()

	// 30天前的交易
	oldTrade := TradeRecord{
		Timestamp: now - 30*24*3600,
		ProfitPct: 0.10,
		IsWin:     true,
		Weight:    1.0,
	}

	// 7天前的交易
	midTrade := TradeRecord{
		Timestamp: now - 7*24*3600,
		ProfitPct: -0.05,
		IsWin:     false,
		Weight:    1.0,
	}

	// 今天的交易
	newTrade := TradeRecord{
		Timestamp: now,
		ProfitPct: 0.15,
		IsWin:     true,
		Weight:    1.0,
	}

	// 计算时间权重
	oldWeight := ksm.CalculateTimeWeight(oldTrade.Timestamp)
	midWeight := ksm.CalculateTimeWeight(midTrade.Timestamp)
	newWeight := ksm.CalculateTimeWeight(newTrade.Timestamp)

	// 验证时间衰减：新交易权重 > 中期交易权重 > 旧交易权重
	if newWeight <= midWeight {
		t.Errorf("新交易权重(%f)应该大于中期交易权重(%f)", newWeight, midWeight)
	}
	if midWeight <= oldWeight {
		t.Errorf("中期交易权重(%f)应该大于旧交易权重(%f)", midWeight, oldWeight)
	}

	// 验证权重在合理范围内
	if newWeight > 1.0 || newWeight <= 0 {
		t.Errorf("新交易权重应该在(0,1]范围内，实际为%f", newWeight)
	}

	if oldWeight < 0.01 { // 最小权重为0.01
		t.Errorf("旧交易权重应该大于等于0.01，实际为%f", oldWeight)
	}

	t.Logf("✅ 时间衰减权重验证通过: 新交易=%.4f, 中期交易=%.4f, 旧交易=%.4f",
		newWeight, midWeight, oldWeight)
}

// TestDataPersistence 测试数据持久化功能
func TestDataPersistence(t *testing.T) {
	tempFile := "/tmp/test_persistence.json"
	defer os.Remove(tempFile)

	symbol := "BTCUSDT"

	// 第一步：创建管理器并添加数据
	ksm1 := NewKellyStopManagerEnhanced(tempFile)
	ksm1.UpdateHistoricalStatsEnhanced(symbol, true, 0.20, 3600)
	ksm1.UpdateHistoricalStatsEnhanced(symbol, false, -0.10, 1800)

	// 强制保存
	if err := ksm1.SaveStatsToFile(tempFile); err != nil {
		t.Fatalf("保存数据失败: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		t.Fatal("数据文件未创建")
	}

	// 第二步：创建新的管理器并加载数据
	ksm2 := NewKellyStopManagerEnhanced(tempFile)
	stats := ksm2.GetHistoricalStats(symbol)

	if stats == nil {
		t.Fatal("无法从文件加载统计数据")
	}

	if stats.TotalTrades != 2 {
		t.Errorf("期望总交易数为2，实际为%d", stats.TotalTrades)
	}

	if stats.ProfitableTrades != 1 {
		t.Errorf("期望盈利交易数为1，实际为%d", stats.ProfitableTrades)
	}

	// 验证加权胜率
	if math.Abs(stats.WeightedWinRate-0.5) > 0.1 {
		t.Errorf("期望加权胜率约为0.5，实际为%.4f", stats.WeightedWinRate)
	}

	t.Log("✅ 数据持久化功能验证通过")
}

// TestEnhancedTakeProfitCalculation 测试增强版止盈计算
func TestEnhancedTakeProfitCalculation(t *testing.T) {
	tempFile := "/tmp/test_enhanced_tp.json"
	defer os.Remove(tempFile)

	ksm := NewKellyStopManagerEnhanced(tempFile)
	symbol := "SOLUSDT"

	// 添加足够的交易历史
	for i := 0; i < 10; i++ {
		profit := 0.05 + float64(i)*0.02
		ksm.UpdateHistoricalStatsEnhanced(symbol, true, profit, 3600)
	}

	// 添加一些亏损交易
	ksm.UpdateHistoricalStatsEnhanced(symbol, false, -0.08, 1800)
	ksm.UpdateHistoricalStatsEnhanced(symbol, false, -0.06, 2400)

	// 测试止盈计算
	entryPrice := 100.0
	currentPrice := 110.0 // 当前盈利10%

	// 更新持仓峰值
	ksm.UpdatePositionPeak(symbol, 0.10)

	// 计算增强版止盈
	tpPrice, err := ksm.CalculateOptimalTakeProfitEnhanced(symbol, entryPrice, currentPrice, "long")
	if err != nil {
		t.Fatalf("计算止盈失败: %v", err)
	}

	if tpPrice <= currentPrice {
		t.Errorf("止盈价格(%f)应该大于当前价格(%f)", tpPrice, currentPrice)
	}

	// 验证止盈在合理范围内
	profitPct := (tpPrice - entryPrice) / entryPrice
	if profitPct < 0.10 || profitPct > 0.50 { // 10%-50%范围
		t.Errorf("止盈百分比(%f)不在合理范围内", profitPct*100)
	}

	t.Logf("✅ 增强版止盈计算验证通过: 入场价=%.2f, 当前价=%.2f, 止盈价=%.2f (盈利%.2f%%)",
		entryPrice, currentPrice, tpPrice, profitPct*100)
}

// TestEnhancedStopLossCalculation 测试增强版止损计算
func TestEnhancedStopLossCalculation(t *testing.T) {
	tempFile := "/tmp/test_enhanced_sl.json"
	defer os.Remove(tempFile)

	ksm := NewKellyStopManagerEnhanced(tempFile)
	symbol := "ADAUSDT"

	// 添加交易历史
	ksm.UpdateHistoricalStatsEnhanced(symbol, true, 0.12, 3600)
	ksm.UpdateHistoricalStatsEnhanced(symbol, false, -0.09, 1800)
	ksm.UpdateHistoricalStatsEnhanced(symbol, true, 0.18, 5400)

	// 测试不同盈利阶段的止损计算
	testCases := []struct {
		name              string
		entryPrice        float64
		currentPrice      float64
		expectedProtection float64 // 期望的保护比例范围
	}{
		{
			name:              "盈利初期(3%)",
			entryPrice:        100.0,
			currentPrice:      103.0,
			expectedProtection: 0.9, // 应该保护大部分利润
		},
		{
			name:              "盈利中期(10%)",
			entryPrice:        100.0,
			currentPrice:      110.0,
			expectedProtection: 0.8, // 保护80%左右
		},
		{
			name:              "盈利后期(20%)",
			entryPrice:        100.0,
			currentPrice:      120.0,
			expectedProtection: 0.85, // 保护85%左右
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 设置峰值
			currentProfit := (tc.currentPrice - tc.entryPrice) / tc.entryPrice
			ksm.UpdatePositionPeak(symbol, currentProfit)

			slPrice, err := ksm.CalculateDynamicStopLossEnhanced(symbol, tc.entryPrice, tc.currentPrice, currentProfit)
			if err != nil {
				t.Fatalf("计算止损失败: %v", err)
			}

			if slPrice >= tc.currentPrice {
				t.Errorf("止损价格(%f)应该小于当前价格(%f)", slPrice, tc.currentPrice)
			}

			if slPrice <= tc.entryPrice {
				t.Errorf("止损价格(%f)应该大于入场价格(%f)", slPrice, tc.entryPrice)
			}

			// 验证保护比例
			protectedProfit := (slPrice - tc.entryPrice) / tc.entryPrice
			protectionRatio := protectedProfit / currentProfit

			if protectionRatio < 0.5 || protectionRatio > 1.0 {
				t.Errorf("保护比例(%f)不在合理范围内", protectionRatio)
			}

			t.Logf("✅ %s: 入场价=%.2f, 当前价=%.2f, 止损价=%.2f, 保护比例=%.2f",
				tc.name, tc.entryPrice, tc.currentPrice, slPrice, protectionRatio)
		})
	}
}

// TestAutoSaveFunctionality 测试自动保存功能
func TestAutoSaveFunctionality(t *testing.T) {
	tempFile := "/tmp/test_autosave.json"
	defer os.Remove(tempFile)

	// 创建带有短保存间隔的管理器
	ksm := NewKellyStopManagerEnhanced(tempFile)

	// 修改保存间隔为1秒用于测试
	config := ksm.GetConfig()
	config.SaveIntervalSeconds = 1
	ksm.UpdateConfig(config)

	symbol := "DOTUSDT"

	// 添加交易数据
	ksm.UpdateHistoricalStatsEnhanced(symbol, true, 0.15, 3600)

	// 等待超过保存间隔
	time.Sleep(2 * time.Second)

	// 再次更新数据，应该触发自动保存
	ksm.UpdateHistoricalStatsEnhanced(symbol, false, -0.08, 1800)

	// 验证文件已更新
	data, err := ioutil.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("读取保存文件失败: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("保存文件为空")
	}

	// 验证文件包含正确的数据
	if !contains(string(data), symbol) {
		t.Errorf("保存文件中未找到符号%s", symbol)
	}

	t.Log("✅ 自动保存功能验证通过")
}

// TestParameterOptimization 测试参数优化功能
func TestParameterOptimization(t *testing.T) {
	tempFile := "/tmp/test_param_opt.json"
	defer os.Remove(tempFile)

	// 创建模拟的增强版交易器
	ksm := NewKellyStopManagerEnhanced(tempFile)

	// 添加不同胜率的交易数据
	highWinRateSymbol := "HIGH_WIN"
	for i := 0; i < 20; i++ {
		isWin := i < 15 // 75%胜率
		profit := 0.08
		if !isWin {
			profit = -0.05
		}
		ksm.UpdateHistoricalStatsEnhanced(highWinRateSymbol, isWin, profit, 3600)
	}

	lowWinRateSymbol := "LOW_WIN"
	for i := 0; i < 20; i++ {
		isWin := i < 6 // 30%胜率
		profit := 0.12
		if !isWin {
			profit = -0.08
		}
		ksm.UpdateHistoricalStatsEnhanced(lowWinRateSymbol, isWin, profit, 3600)
	}

	// 验证高胜率币种的统计
	highStats := ksm.GetHistoricalStats(highWinRateSymbol)
	if highStats.WeightedWinRate < 0.7 {
		t.Errorf("高胜率币种加权胜率应该>0.7，实际为%.4f", highStats.WeightedWinRate)
	}

	// 验证低胜率币种的统计
	lowStats := ksm.GetHistoricalStats(lowWinRateSymbol)
	if lowStats.WeightedWinRate > 0.4 {
		t.Errorf("低胜率币种加权胜率应该<0.4，实际为%.4f", lowStats.WeightedWinRate)
	}

	t.Logf("✅ 参数优化测试验证通过: 高胜率=%.2f%%, 低胜率=%.2f%%",
		highStats.WeightedWinRate*100, lowStats.WeightedWinRate*100)
}

// TestVolatilityBasedAdjustment 测试基于波动率的调整
func TestVolatilityBasedAdjustment(t *testing.T) {
	tempFile := "/tmp/test_volatility.json"
	defer os.Remove(tempFile)

	ksm := NewKellyStopManagerEnhanced(tempFile)
	symbol := "VOLA_TEST"

	// 添加高波动率的交易历史
	for i := 0; i < 15; i++ {
		profit := 0.25 // 25%盈利
		if i%3 == 0 {   // 每3个交易有一个大亏损
			profit = -0.20
		}
		ksm.UpdateHistoricalStatsEnhanced(symbol, profit > 0, profit, 3600)
	}

	stats := ksm.GetHistoricalStats(symbol)
	if stats.Volatility < 0.15 { // 应该检测到高波动率
		t.Errorf("期望检测到高波动率(>0.15)，实际为%.4f", stats.Volatility)
	}

	// 测试基于波动率的止盈计算
	entryPrice := 100.0
	currentPrice := 110.0

	// 高波动率应该导致更保守的止盈
	tpPrice, err := ksm.CalculateOptimalTakeProfitEnhanced(symbol, entryPrice, currentPrice, "long")
	if err != nil {
		t.Fatalf("计算止盈失败: %v", err)
	}

	profitPct := (tpPrice - entryPrice) / entryPrice
	if profitPct > 0.5 { // 高波动率下不应该设置过高的止盈
		t.Errorf("高波动率下止盈百分比(%f)过高", profitPct*100)
	}

	t.Logf("✅ 波动率调整验证通过: 波动率=%.2f%%, 止盈百分比=%.2f%%",
		stats.Volatility*100, profitPct*100)
}

// TestEdgeCases 测试边界情况
func TestEdgeCases(t *testing.T) {
	tempFile := "/tmp/test_edge_cases.json"
	defer os.Remove(tempFile)

	ksm := NewKellyStopManagerEnhanced(tempFile)

	// 测试1: 无效价格
	_, err := ksm.CalculateOptimalTakeProfitEnhanced("TEST", 0, 100, "long")
	if err == nil {
		t.Error("期望无效价格会返回错误")
	}

	_, err = ksm.CalculateDynamicStopLossEnhanced("TEST", 100, 0, 0.1)
	if err == nil {
		t.Error("期望无效价格会返回错误")
	}

	// 测试2: 负数盈利百分比
	symbol := "NEG_TEST"
	ksm.UpdatePositionPeak(symbol, -0.05) // 负盈利不应该更新峰值
	peak := ksm.GetPositionPeak(symbol)
	if peak != 0 {
		t.Errorf("负盈利不应该更新峰值，期望为0，实际为%f", peak)
	}

	// 测试3: 零交易历史
	emptyStats := ksm.GetHistoricalStats("NON_EXISTENT")
	if emptyStats != nil {
		t.Error("不存在的符号应该返回nil")
	}

	// 测试4: 极端配置值
	config := ksm.GetConfig()
	config.KellyRatioAdjustment = 0.0 // 零凯利系数
	config.MaxTakeProfitMultiplier = 100.0 // 极大止盈倍数
	ksm.UpdateConfig(config)

	// 验证配置更新
	newConfig := ksm.GetConfig()
	if newConfig.KellyRatioAdjustment != 0.0 {
		t.Error("配置更新失败")
	}

	t.Log("✅ 边界情况测试验证通过")
}

// TestPerformanceBenchmark 性能基准测试
func TestPerformanceBenchmark(t *testing.T) {
	tempFile := "/tmp/test_performance.json"
	defer os.Remove(tempFile)

	ksm := NewKellyStopManagerEnhanced(tempFile)

	// 测试大量交易记录的更新性能
	start := time.Now()
	symbol := "PERF_TEST"

	for i := 0; i < 1000; i++ {
		isWin := i%3 == 0 // 33%胜率
		profit := 0.08
		if !isWin {
			profit = -0.05
		}
		ksm.UpdateHistoricalStatsEnhanced(symbol, isWin, profit, 3600)
	}

	elapsed := time.Since(start)

	stats := ksm.GetHistoricalStats(symbol)
	if stats.TotalTrades != 1000 {
		t.Errorf("期望1000笔交易，实际为%d", stats.TotalTrades)
	}

	t.Logf("✅ 性能基准测试: 更新1000笔交易耗时 %v", elapsed)
	if elapsed > 100*time.Millisecond {
		t.Logf("⚠️ 性能警告: 更新1000笔交易耗时较长(%v)，建议优化", elapsed)
	}
}

// 辅助函数
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestRealWorldScenarios 测试真实场景
func TestRealWorldScenarios(t *testing.T) {
	tempFile := "/tmp/test_real_world.json"
	defer os.Remove(tempFile)

	ksm := NewKellyStopManagerEnhanced(tempFile)
	symbol := "BTCUSDT"

	// 场景1: 趋势行情（连续盈利）
	t.Run("趋势行情", func(t *testing.T) {
		for i := 0; i < 8; i++ {
			profit := 0.05 + float64(i)*0.02 // 递增盈利
			ksm.UpdateHistoricalStatsEnhanced(symbol, true, profit, 3600)
		}

		stats := ksm.GetHistoricalStats(symbol)
		if stats.WinRate < 0.99 { // 几乎100%胜率
			t.Errorf("趋势行情中期望高胜率，实际为%.2f%%", stats.WinRate*100)
		}

		// 测试止盈计算
		entryPrice := 50000.0
		currentPrice := 52000.0 // 4%盈利
		tpPrice, _ := ksm.CalculateOptimalTakeProfitEnhanced(symbol, entryPrice, currentPrice, "long")

		profitPct := (tpPrice - entryPrice) / entryPrice
		t.Logf("趋势行情: 入场价=%.0f, 当前价=%.0f, 止盈价=%.0f (盈利%.2f%%), 胜率=%.2f%%",
			entryPrice, currentPrice, tpPrice, profitPct*100, stats.WinRate*100)
	})

	// 清空数据测试下一个场景
	os.Remove(tempFile)
	ksm = NewKellyStopManagerEnhanced(tempFile)

	// 场景2: 震荡行情（胜负交替）
	t.Run("震荡行情", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			isWin := i%2 == 0 // 胜负交替
			profit := 0.08
			if !isWin {
				profit = -0.06
			}
			ksm.UpdateHistoricalStatsEnhanced(symbol, isWin, profit, 3600)
		}

		stats := ksm.GetHistoricalStats(symbol)
		if math.Abs(stats.WinRate-0.5) > 0.1 { // 接近50%胜率
			t.Errorf("震荡行情中期望胜率接近50%%，实际为%.2f%%", stats.WinRate*100)
		}

		t.Logf("震荡行情: 胜率=%.2f%%, 加权胜率=%.2f%%, 波动率=%.2f%%",
			stats.WinRate*100, stats.WeightedWinRate*100, stats.Volatility*100)
	})

	// 场景3: 下跌趋势（连续亏损）
	os.Remove(tempFile)
	ksm = NewKellyStopManagerEnhanced(tempFile)

	t.Run("下跌趋势", func(t *testing.T) {
		for i := 0; i < 6; i++ {
			loss := 0.05 + float64(i)*0.01 // 递增亏损
			ksm.UpdateHistoricalStatsEnhanced(symbol, false, -loss, 3600)
		}

		stats := ksm.GetHistoricalStats(symbol)
		if stats.WinRate > 0.1 { // 低胜率
			t.Errorf("下跌趋势中期望低胜率，实际为%.2f%%", stats.WinRate*100)
		}

		// 测试凯利比例为负时的保守策略
		entryPrice := 50000.0
		currentPrice := 51000.0 // 2%盈利
		tpPrice, _ := ksm.CalculateOptimalTakeProfitEnhanced(symbol, entryPrice, currentPrice, "long")

		profitPct := (tpPrice - entryPrice) / entryPrice
		if profitPct > 0.1 { // 应该使用保守策略
			t.Errorf("下跌趋势中应该使用保守止盈策略，盈利百分比(%f)过高", profitPct*100)
		}

		t.Logf("下跌趋势: 胜率=%.2f%%, 止盈盈利=%.2f%%",
			stats.WinRate*100, profitPct*100)
	})
}

// 创建测试数据目录
func TestMain(m *testing.M) {
	// 确保测试目录存在
	os.MkdirAll("/tmp", 0755)

	// 运行测试
	code := m.Run()

	// 清理测试文件
	cleanupFiles := []string{
		"/tmp/test_kelly_enhanced.json",
		"/tmp/test_peak_tracking.json",
		"/tmp/test_time_decay.json",
		"/tmp/test_persistence.json",
		"/tmp/test_enhanced_tp.json",
		"/tmp/test_enhanced_sl.json",
		"/tmp/test_autosave.json",
		"/tmp/test_param_opt.json",
		"/tmp/test_volatility.json",
		"/tmp/test_edge_cases.json",
		"/tmp/test_performance.json",
		"/tmp/test_real_world.json",
	}

	for _, file := range cleanupFiles {
		os.Remove(file)
	}

	os.Exit(code)
}

// BenchmarkKellyCalculation 基准测试Kelly计算性能
func BenchmarkKellyCalculation(b *testing.B) {
	tempFile := "/tmp/bench_kelly.json"
	defer os.Remove(tempFile)

	ksm := NewKellyStopManagerEnhanced(tempFile)
	symbol := "BENCHMARK"

	// 预填充数据
	for i := 0; i < 100; i++ {
		ksm.UpdateHistoricalStatsEnhanced(symbol, true, 0.10, 3600)
	}

	entryPrice := 100.0
	currentPrice := 110.0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ksm.CalculateOptimalTakeProfitEnhanced(symbol, entryPrice, currentPrice, "long")
	}
}

// BenchmarkStopLossCalculation 基准测试止损计算性能
func BenchmarkStopLossCalculation(b *testing.B) {
	tempFile := "/tmp/bench_sl.json"
	defer os.Remove(tempFile)

	ksm := NewKellyStopManagerEnhanced(tempFile)
	symbol := "BENCHMARK"

	// 预填充数据
	for i := 0; i < 100; i++ {
		ksm.UpdateHistoricalStatsEnhanced(symbol, true, 0.10, 3600)
	}

	entryPrice := 100.0
	currentPrice := 115.0
	maxProfit := 0.15

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ksm.CalculateDynamicStopLossEnhanced(symbol, entryPrice, currentPrice, maxProfit)
	}
}

// BenchmarkDataPersistence 基准测试数据持久化性能
func BenchmarkDataPersistence(b *testing.B) {
	tempFile := "/tmp/bench_persist.json"
	defer os.Remove(tempFile)

	ksm := NewKellyStopManagerEnhanced(tempFile)
	symbol := "BENCHMARK"

	// 预填充大量数据
	for i := 0; i < 1000; i++ {
		ksm.UpdateHistoricalStatsEnhanced(symbol, true, 0.10, 3600)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ksm.SaveStatsToFile(tempFile)
	}
}

// 示例：如何使用增强版Kelly管理器
func ExampleKellyStopManagerEnhanced() {
	// 创建带有数据持久化的增强版Kelly管理器
	ksm := NewKellyStopManagerEnhanced("data/kelly_stats.json")

	// 更新交易结果（平仓时调用）
	symbol := "BTCUSDT"
	isWin := true
	profitPct := 0.15  // 15%盈利
	holdingTime := int64(3600) // 1小时持仓

	ksm.UpdateHistoricalStatsEnhanced(symbol, isWin, profitPct, holdingTime)

	// 在持仓期间更新峰值盈利（每个交易周期调用）
	currentProfitPct := 0.08 // 当前盈利8%
	ksm.UpdatePositionPeak(symbol, currentProfitPct)

	// 计算动态止盈止损（每个交易周期调用）
	entryPrice := 50000.0
	currentPrice := 54000.0 // 8%盈利
	positionSide := "long"

	takeProfitPrice, _ := ksm.CalculateOptimalTakeProfitEnhanced(symbol, entryPrice, currentPrice, positionSide)
	stopLossPrice, _ := ksm.CalculateDynamicStopLossEnhanced(symbol, entryPrice, currentPrice, currentProfitPct)

	fmt.Printf("建议止盈价格: %.2f\n", takeProfitPrice)
	fmt.Printf("建议止损价格: %.2f\n", stopLossPrice)

	// 获取统计数据
	stats := ksm.GetHistoricalStats(symbol)
	if stats != nil {
		fmt.Printf("总交易数: %d\n", stats.TotalTrades)
		fmt.Printf("加权胜率: %.2f%%\n", stats.WeightedWinRate*100)
		fmt.Printf("波动率: %.2f%%\n", stats.Volatility*100)
	}

	// 程序退出时保存数据
	ksm.Shutdown()
}