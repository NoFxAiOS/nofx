package backtest

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
)

// TestHyperliquidTimeframes 测试Hyperliquid交易所不同时间颗粒度的数据获取能力
// 运行方式: go test -v -run TestHyperliquidTimeframes -timeout 5m
func TestHyperliquidTimeframes(t *testing.T) {
	// 初始化Hyperliquid交易所
	exchange := ccxt.NewHyperliquid(nil)
	symbol := "BTC/USDC:USDC" // Hyperliquid使用的交易对格式
	
	// 定义要测试的时间颗粒度
	testCases := []struct {
		timeframe   string
		description string
		testLimit   int64
		testDaysAgo int // 测试能否获取N天前的数据
	}{
		{"1m", "1分钟", 500, 30},
		{"5m", "5分钟", 500, 60},
		{"15m", "15分钟", 500, 90},
		{"1h", "1小时", 500, 180},
		{"4h", "4小时", 500, 365},
		{"1d", "1天", 500, 730},
	}
	
	results := make(map[string]interface{})
	
	fmt.Printf("\n=== Hyperliquid 数据获取能力测试 ===\n")
	fmt.Printf("交易所: Hyperliquid\n")
	fmt.Printf("交易对: %s\n", symbol)
	fmt.Printf("测试时间: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	
	for _, tc := range testCases {
		t.Run(tc.timeframe, func(t *testing.T) {
			result := make(map[string]interface{})
			
			fmt.Printf("\n--- 测试 %s (%s) ---\n", tc.timeframe, tc.description)
			
			// 1. 测试获取最新数据
			fmt.Printf("  正在获取最新 %d 条数据...\n", tc.testLimit)
			ohlcv, err := exchange.FetchOHLCV(
				symbol,
				ccxt.WithFetchOHLCVTimeframe(tc.timeframe),
				ccxt.WithFetchOHLCVLimit(tc.testLimit),
			)
			
			if err != nil {
				t.Logf("获取最新数据失败: %v", err)
				result["error"] = err.Error()
				result["available"] = false
				results[tc.timeframe] = result
				fmt.Printf("  ❌ 获取失败: %v\n", err)
				return
			}
			
			if len(ohlcv) == 0 {
				t.Log("未获取到数据")
				result["count"] = 0
				result["available"] = false
				results[tc.timeframe] = result
				fmt.Printf("  ⚠️  未获取到数据\n")
				return
			}
			
			// 分析最新数据
			firstTime := time.Unix(int64(ohlcv[0].Timestamp)/1000, 0)
			lastTime := time.Unix(int64(ohlcv[len(ohlcv)-1].Timestamp)/1000, 0)
			duration := lastTime.Sub(firstTime)
			durationDays := duration.Hours() / 24
			
			result["available"] = true
			result["max_count"] = len(ohlcv)
			result["first_time"] = firstTime.Format("2006-01-02 15:04:05")
			result["last_time"] = lastTime.Format("2006-01-02 15:04:05")
			result["duration_days"] = fmt.Sprintf("%.2f", durationDays)
			result["duration_hours"] = fmt.Sprintf("%.2f", duration.Hours())
			
			fmt.Printf("  ✅ 成功获取 %d 条数据\n", len(ohlcv))
			fmt.Printf("     时间范围: %s 至 %s\n", 
				firstTime.Format("2006-01-02 15:04"), 
				lastTime.Format("2006-01-02 15:04"))
			fmt.Printf("     跨度: %.2f 天 (%.2f 小时)\n", durationDays, duration.Hours())
			
			// 显示前2条数据样本
			fmt.Printf("     前2条样本:\n")
			for i := 0; i < 2 && i < len(ohlcv); i++ {
				k := ohlcv[i]
				ts := time.Unix(int64(k.Timestamp)/1000, 0)
				fmt.Printf("       [%d] %s | O:%.2f H:%.2f L:%.2f C:%.2f V:%.2f\n",
					i, ts.Format("01-02 15:04"), k.Open, k.High, k.Low, k.Close, k.Volume)
			}
			
			// 2. 测试历史数据可访问性
			time.Sleep(200 * time.Millisecond) // 避免API限流
			
			fmt.Printf("  正在测试 %d 天前的历史数据...\n", tc.testDaysAgo)
			daysAgo := tc.testDaysAgo
			since := time.Now().AddDate(0, 0, -daysAgo).Unix() * 1000
			
			histOhlcv, err := exchange.FetchOHLCV(
				symbol,
				ccxt.WithFetchOHLCVTimeframe(tc.timeframe),
				ccxt.WithFetchOHLCVSince(since),
				ccxt.WithFetchOHLCVLimit(100),
			)
			
			if err == nil && len(histOhlcv) > 0 {
				histTime := time.Unix(int64(histOhlcv[0].Timestamp)/1000, 0)
				actualDaysAgo := time.Since(histTime).Hours() / 24
				result["historical_available"] = true
				result["historical_requested_days"] = daysAgo
				result["historical_actual_days"] = fmt.Sprintf("%.1f", actualDaysAgo)
				result["historical_start_time"] = histTime.Format("2006-01-02 15:04:05")
				
				fmt.Printf("  ✅ %d天前历史数据可获取\n", daysAgo)
				fmt.Printf("     实际获取到: %s (距今 %.0f 天)\n", 
					histTime.Format("2006-01-02 15:04"), actualDaysAgo)
			} else {
				result["historical_available"] = false
				result["historical_requested_days"] = daysAgo
				if err != nil {
					result["historical_error"] = err.Error()
					fmt.Printf("  ❌ %d天前历史数据不可获取: %v\n", daysAgo, err)
				} else {
					fmt.Printf("  ⚠️  %d天前历史数据不可获取 (无数据返回)\n", daysAgo)
				}
			}
			
			// 3. 二分查找最早可获取的数据点
			time.Sleep(200 * time.Millisecond)
			
			fmt.Printf("  正在查找最早可获取的数据点...\n")
			earliestTime := findEarliestHyperliquidData(exchange, symbol, tc.timeframe)
			if earliestTime > 0 {
				earliest := time.Unix(earliestTime/1000, 0)
				daysAgo := time.Since(earliest).Hours() / 24
				result["earliest_available"] = earliest.Format("2006-01-02 15:04:05")
				result["earliest_days_ago"] = fmt.Sprintf("%.0f", daysAgo)
				fmt.Printf("  ✅ 最早可获取: %s (距今 %.0f 天)\n", 
					earliest.Format("2006-01-02 15:04"), daysAgo)
			} else {
				result["earliest_available"] = "unknown"
				fmt.Printf("  ⚠️  未能确定最早可获取时间\n")
			}
			
			results[tc.timeframe] = result
		})
		
		// 每个测试之间稍作延迟,避免API限流
		time.Sleep(300 * time.Millisecond)
	}
	
	// 输出完整的JSON结果
	fmt.Printf("\n\n=== JSON 详细结果 ===\n")
	jsonData, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(jsonData))
	
	// 输出表格格式的汇总
	printHyperliquidSummary(testCases, results)
}

// findEarliestHyperliquidData 使用二分法查找Hyperliquid最早可获取的数据时间点
func findEarliestHyperliquidData(exchange *ccxt.Hyperliquid, symbol, timeframe string) int64 {
	now := time.Now().Unix() * 1000
	
	// 从3年前开始测试
	left := time.Now().AddDate(-3, 0, 0).Unix() * 1000
	right := now
	
	var earliest int64 = 0
	
	// 二分查找,最多尝试12次
	for i := 0; i < 12 && left < right; i++ {
		mid := (left + right) / 2
		
		ohlcv, err := exchange.FetchOHLCV(
			symbol,
			ccxt.WithFetchOHLCVTimeframe(timeframe),
			ccxt.WithFetchOHLCVSince(mid),
			ccxt.WithFetchOHLCVLimit(5),
		)
		
		if err != nil || len(ohlcv) == 0 {
			// 这个时间点无数据,向右移动
			left = mid + 86400000 // 加1天
		} else {
			// 有数据,记录并继续向左查找
			earliest = int64(ohlcv[0].Timestamp)
			right = mid - 86400000 // 减1天
		}
		
		time.Sleep(150 * time.Millisecond) // 避免API限流
	}
	
	return earliest
}

func printHyperliquidSummary(testCases []struct {
	timeframe   string
	description string
	testLimit   int64
	testDaysAgo int
}, results map[string]interface{}) {
	fmt.Printf("\n\n=== Hyperliquid 测试结果汇总 ===\n\n")
	
	fmt.Printf("%-8s | %-6s | %-12s | %-12s | %-20s | %-12s\n",
		"颗粒度", "可用", "最大条数", "跨度(天)", "最早可获取", "距今(天)")
	fmt.Println("---------|--------|--------------|--------------|----------------------|-------------")
	
	for _, tc := range testCases {
		result, ok := results[tc.timeframe].(map[string]interface{})
		if !ok {
			continue
		}
		
		available := "否"
		if avail, ok := result["available"].(bool); ok && avail {
			available = "是"
		}
		
		maxCount := getResultValue(result, "max_count", "-")
		durationDays := getResultValue(result, "duration_days", "-")
		earliest := getResultValue(result, "earliest_available", "未知")
		earliestDays := getResultValue(result, "earliest_days_ago", "-")
		
		// 截断最早时间显示
		if len(earliest) > 16 {
			earliest = earliest[:16]
		}
		
		fmt.Printf("%-8s | %-6s | %-12s | %-12s | %-20s | %-12s\n",
			tc.timeframe, available, maxCount, durationDays, earliest, earliestDays)
	}
	
	// 输出关键发现
	fmt.Printf("\n=== 关键发现 ===\n")
	for _, tc := range testCases {
		result, ok := results[tc.timeframe].(map[string]interface{})
		if !ok {
			continue
		}
		
		if avail, ok := result["available"].(bool); ok && avail {
			count := getResultValue(result, "max_count", "?")
			duration := getResultValue(result, "duration_days", "?")
			earliest := getResultValue(result, "earliest_days_ago", "?")
			
			fmt.Printf("\n%s (%s):\n", tc.timeframe, tc.description)
			fmt.Printf("  - 单次最多获取: %s 条\n", count)
			fmt.Printf("  - 单次覆盖时长: %s 天\n", duration)
			fmt.Printf("  - 最早数据距今: %s 天\n", earliest)
			
			// 计算需要多少次请求才能获取所有历史数据
			if daysFloat, ok := parseFloat(earliest); ok {
				if durationFloat, ok := parseFloat(duration); ok && durationFloat > 0 {
					requests := int(daysFloat / durationFloat)
					fmt.Printf("  - 获取全部历史数据约需: %d 次请求\n", requests+1)
				}
			}
		}
	}
}

func getResultValue(m map[string]interface{}, key, defaultVal string) string {
	if val, ok := m[key]; ok {
		return fmt.Sprintf("%v", val)
	}
	return defaultVal
}

func parseFloat(s string) (float64, bool) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err == nil
}
