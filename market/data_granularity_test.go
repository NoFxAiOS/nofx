package market

import (
	"testing"
)

// TestDataPointsConfiguration 测试数据点配置是否正确应用
func TestDataPointsConfiguration(t *testing.T) {
	// 模拟配置
	config := &IndicatorConfig{
		Indicators: []string{"ema", "macd", "rsi"},
		Timeframes: []string{"3m", "4h"},
		DataPoints: map[string]int{
			"3m": 20, // 自定义20个数据点
			"4h": 10, // 自定义10个数据点
		},
	}

	// 生成测试K线数据
	klines3m := generateTestKlines(100)
	klines4h := generateTestKlines(50)

	// 测试3分钟数据点配置
	t.Run("3分钟数据点配置", func(t *testing.T) {
		dataPoints := config.DataPoints["3m"]
		if dataPoints == 0 {
			dataPoints = 40 // 默认值
		}

		intradayData := calculateIntradaySeries(klines3m, dataPoints)

		// 验证返回的数据点数量
		if len(intradayData.MidPrices) != dataPoints {
			t.Errorf("预期 %d 个数据点，实际得到 %d 个", dataPoints, len(intradayData.MidPrices))
		}
		if len(intradayData.EMA20Values) != dataPoints {
			t.Errorf("预期 EMA20 有 %d 个数据点，实际得到 %d 个", dataPoints, len(intradayData.EMA20Values))
		}
		if len(intradayData.MACDValues) != dataPoints {
			t.Errorf("预期 MACD 有 %d 个数据点，实际得到 %d 个", dataPoints, len(intradayData.MACDValues))
		}
		if len(intradayData.RSI7Values) != dataPoints {
			t.Errorf("预期 RSI7 有 %d 个数据点，实际得到 %d 个", dataPoints, len(intradayData.RSI7Values))
		}
		if len(intradayData.Volume) != dataPoints {
			t.Errorf("预期 Volume 有 %d 个数据点，实际得到 %d 个", dataPoints, len(intradayData.Volume))
		}

		t.Logf("✅ 3分钟数据成功返回 %d 个数据点", len(intradayData.MidPrices))
	})

	// 测试4小时数据点配置
	t.Run("4小时数据点配置", func(t *testing.T) {
		dataPoints := config.DataPoints["4h"]
		if dataPoints == 0 {
			dataPoints = 25 // 默认值
		}

		longerTermData := calculateLongerTermData(klines4h, dataPoints)

		// 验证返回的数据点数量
		if len(longerTermData.MACDValues) != dataPoints {
			t.Errorf("预期 MACD 有 %d 个数据点，实际得到 %d 个", dataPoints, len(longerTermData.MACDValues))
		}
		if len(longerTermData.RSI14Values) != dataPoints {
			t.Errorf("预期 RSI14 有 %d 个数据点，实际得到 %d 个", dataPoints, len(longerTermData.RSI14Values))
		}

		t.Logf("✅ 4小时数据成功返回 %d 个数据点", len(longerTermData.MACDValues))
	})

	// 测试默认值
	t.Run("默认数据点配置", func(t *testing.T) {
		defaultConfig := &IndicatorConfig{
			Indicators: []string{"ema", "macd"},
			Timeframes: []string{"3m", "4h"},
			DataPoints: map[string]int{}, // 空配置，应使用默认值
		}

		// 3分钟应返回40个点（默认值）
		dataPoints3m := defaultConfig.DataPoints["3m"]
		if dataPoints3m == 0 {
			dataPoints3m = 40
		}
		intradayData := calculateIntradaySeries(klines3m, dataPoints3m)
		if len(intradayData.MidPrices) != 40 {
			t.Errorf("预期默认返回 40 个数据点，实际得到 %d 个", len(intradayData.MidPrices))
		}

		// 4小时应返回25个点（默认值）
		dataPoints4h := defaultConfig.DataPoints["4h"]
		if dataPoints4h == 0 {
			dataPoints4h = 25
		}
		longerTermData := calculateLongerTermData(klines4h, dataPoints4h)
		if len(longerTermData.MACDValues) != 25 {
			t.Errorf("预期默认返回 25 个数据点，实际得到 %d 个", len(longerTermData.MACDValues))
		}

		t.Logf("✅ 默认配置正确: 3m=%d, 4h=%d", len(intradayData.MidPrices), len(longerTermData.MACDValues))
	})

	// 测试极限值
	t.Run("极限数据点配置", func(t *testing.T) {
		extremeConfig := &IndicatorConfig{
			Indicators: []string{"ema"},
			Timeframes: []string{"3m"},
			DataPoints: map[string]int{
				"3m": 100, // 非常多的数据点
			},
		}

		dataPoints := extremeConfig.DataPoints["3m"]
		intradayData := calculateIntradaySeries(klines3m, dataPoints)

		// 验证数据点数量（应该等于请求的数量）
		if len(intradayData.MidPrices) != 100 {
			t.Errorf("预期 100 个数据点，实际得到 %d 个", len(intradayData.MidPrices))
		}

		t.Logf("✅ 极限配置正确处理: 请求100个点，实际返回%d个点", len(intradayData.MidPrices))
	})
}

// TestGet_WithCustomDataPoints 测试Get函数是否正确应用配置
func TestGet_WithCustomDataPoints(t *testing.T) {
	// 注意: 这个测试需要真实的WebSocket连接，所以跳过
	// 这里只是演示如何测试Get函数的配置应用
	t.Skip("需要真实的WebSocket连接，跳过")

	config := &IndicatorConfig{
		Indicators: []string{"ema", "macd", "rsi"},
		Timeframes: []string{"3m", "4h"},
		DataPoints: map[string]int{
			"3m": 30,
			"4h": 15,
		},
	}

	data, err := Get("BTCUSDT", config)
	if err != nil {
		t.Fatalf("获取市场数据失败: %v", err)
	}

	// 验证数据点数量
	if data.IntradaySeries != nil {
		if len(data.IntradaySeries.MidPrices) != 30 {
			t.Errorf("预期3分钟数据有30个点，实际得到 %d 个", len(data.IntradaySeries.MidPrices))
		}
	}

	if data.LongerTermContext != nil {
		if len(data.LongerTermContext.MACDValues) != 15 {
			t.Errorf("预期4小时数据有15个点，实际得到 %d 个", len(data.LongerTermContext.MACDValues))
		}
	}
}
