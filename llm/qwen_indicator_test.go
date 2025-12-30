package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"nofx/market"
	"nofx/provider/coinank"
	"nofx/provider/coinank/coinank_api"
	"nofx/provider/coinank/coinank_enum"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// IndicatorResult AI 计算的指标结果
type IndicatorResult struct {
	EMA12   float64 `json:"ema12"`
	EMA26   float64 `json:"ema26"`
	MACD    float64 `json:"macd"`
	RSI14   float64 `json:"rsi14"`
	BOLLUp  float64 `json:"boll_upper"`
	BOLLMid float64 `json:"boll_middle"`
	BOLLLow float64 `json:"boll_lower"`
	ATR14   float64 `json:"atr14"`
	SMA20   float64 `json:"sma20"`
}

// 本地计算指标（使用 market 包的函数）
func calculateLocalIndicators(klines []market.Kline) IndicatorResult {
	result := IndicatorResult{}

	if len(klines) >= 12 {
		result.EMA12 = market.ExportCalculateEMA(klines, 12)
	}
	if len(klines) >= 26 {
		result.EMA26 = market.ExportCalculateEMA(klines, 26)
		result.MACD = market.ExportCalculateMACD(klines)
	}
	if len(klines) > 14 {
		result.RSI14 = market.ExportCalculateRSI(klines, 14)
	}
	if len(klines) >= 20 {
		result.BOLLUp, result.BOLLMid, result.BOLLLow = market.ExportCalculateBOLL(klines, 20, 2.0)
		// SMA20 就是 BOLL 中轨
		result.SMA20 = result.BOLLMid
	}
	if len(klines) > 14 {
		result.ATR14 = market.ExportCalculateATR(klines, 14)
	}

	return result
}

// 格式化 K 线数据为文本，发给 AI
func formatKlinesForAI(klines []market.Kline) string {
	var sb strings.Builder
	sb.WriteString("以下是K线数据（从旧到新排列）：\n")
	sb.WriteString("序号 | 时间 | 开盘价 | 最高价 | 最低价 | 收盘价 | 成交量\n")
	sb.WriteString("-----|------|--------|--------|--------|--------|--------\n")

	for i, k := range klines {
		t := time.UnixMilli(k.OpenTime)
		sb.WriteString(fmt.Sprintf("%d | %s | %.2f | %.2f | %.2f | %.2f | %.2f\n",
			i+1, t.Format("01-02 15:04"), k.Open, k.High, k.Low, k.Close, k.Volume))
	}

	return sb.String()
}

// 构建 AI 计算指标的 prompt
func buildIndicatorPrompt(klines []market.Kline) string {
	klinesText := formatKlinesForAI(klines)

	prompt := fmt.Sprintf(`%s

请根据以上 %d 根K线数据，计算以下技术指标（使用标准算法）：

1. EMA12（12周期指数移动平均线）
2. EMA26（26周期指数移动平均线）
3. MACD（EMA12 - EMA26）
4. RSI14（14周期相对强弱指标，使用Wilder平滑法）
5. BOLL布林带（20周期，2倍标准差）：上轨、中轨、下轨
6. ATR14（14周期平均真实波幅，使用Wilder平滑法）
7. SMA20（20周期简单移动平均线）

请严格按照以下 JSON 格式返回结果，不要添加任何其他文字：
{
  "ema12": 数值,
  "ema26": 数值,
  "macd": 数值,
  "rsi14": 数值,
  "boll_upper": 数值,
  "boll_middle": 数值,
  "boll_lower": 数值,
  "atr14": 数值,
  "sma20": 数值
}

注意：
- 所有数值保留2位小数
- EMA计算使用SMA作为初始值，乘数为 2/(period+1)
- RSI使用Wilder平滑法
- 只返回JSON，不要解释过程`, klinesText, len(klines))

	return prompt
}

// 从 AI 响应中提取 JSON
func extractJSONFromResponse(text string) (IndicatorResult, error) {
	var result IndicatorResult

	// 尝试直接解析
	if err := json.Unmarshal([]byte(text), &result); err == nil {
		return result, nil
	}

	// 提取 JSON 部分
	re := regexp.MustCompile(`\{[^{}]*"ema12"[^{}]*\}`)
	match := re.FindString(text)
	if match == "" {
		// 尝试更宽松的匹配
		start := strings.Index(text, "{")
		end := strings.LastIndex(text, "}")
		if start != -1 && end != -1 && end > start {
			match = text[start : end+1]
		}
	}

	if match == "" {
		return result, fmt.Errorf("no JSON found in response: %s", text[:min(200, len(text))])
	}

	if err := json.Unmarshal([]byte(match), &result); err != nil {
		return result, fmt.Errorf("parse JSON failed: %w, json: %s", err, match)
	}

	return result, nil
}

// 比较两个指标结果，返回误差百分比
func compareIndicators(local, ai IndicatorResult) map[string]float64 {
	errors := make(map[string]float64)

	calcError := func(name string, localVal, aiVal float64) {
		if localVal == 0 {
			if aiVal == 0 {
				errors[name] = 0
			} else {
				errors[name] = 100 // 本地为0但AI不为0
			}
			return
		}
		errors[name] = math.Abs(localVal-aiVal) / math.Abs(localVal) * 100
	}

	calcError("EMA12", local.EMA12, ai.EMA12)
	calcError("EMA26", local.EMA26, ai.EMA26)
	calcError("MACD", local.MACD, ai.MACD)
	calcError("RSI14", local.RSI14, ai.RSI14)
	calcError("BOLL_UP", local.BOLLUp, ai.BOLLUp)
	calcError("BOLL_MID", local.BOLLMid, ai.BOLLMid)
	calcError("BOLL_LOW", local.BOLLLow, ai.BOLLLow)
	calcError("ATR14", local.ATR14, ai.ATR14)
	calcError("SMA20", local.SMA20, ai.SMA20)

	return errors
}

// 生成测试用 K 线数据
func generateTestKlines(count int, basePrice float64) []market.Kline {
	klines := make([]market.Kline, count)
	price := basePrice
	now := time.Now()

	for i := 0; i < count; i++ {
		// 模拟价格波动
		change := (float64(i%7) - 3) * 0.5 // -1.5 到 +1.5 的波动
		price = price + change

		open := price
		high := price + math.Abs(change)*0.5 + 0.5
		low := price - math.Abs(change)*0.5 - 0.3
		close := price + (change * 0.3)

		klines[i] = market.Kline{
			OpenTime:  now.Add(time.Duration(-count+i) * time.Hour).UnixMilli(),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    1000 + float64(i*100),
			CloseTime: now.Add(time.Duration(-count+i+1) * time.Hour).UnixMilli(),
		}
	}

	return klines
}

// TestQwenIndicatorCalculation 测试 AI 计算技术指标
func TestQwenIndicatorCalculation(t *testing.T) {
	agent := NewQwenAgent(QwenAppID, QwenAPIKey)
	ctx := context.Background()

	// 生成 30 根测试 K 线
	klines := generateTestKlines(30, 95000)

	t.Log("===== K线数据 (最后5根) =====")
	for i := len(klines) - 5; i < len(klines); i++ {
		k := klines[i]
		t.Logf("  [%d] O:%.2f H:%.2f L:%.2f C:%.2f", i+1, k.Open, k.High, k.Low, k.Close)
	}

	// 本地计算
	t.Log("\n===== 本地计算结果 =====")
	localResult := calculateLocalIndicators(klines)
	t.Logf("  EMA12:    %.2f", localResult.EMA12)
	t.Logf("  EMA26:    %.2f", localResult.EMA26)
	t.Logf("  MACD:     %.2f", localResult.MACD)
	t.Logf("  RSI14:    %.2f", localResult.RSI14)
	t.Logf("  BOLL上轨: %.2f", localResult.BOLLUp)
	t.Logf("  BOLL中轨: %.2f", localResult.BOLLMid)
	t.Logf("  BOLL下轨: %.2f", localResult.BOLLLow)
	t.Logf("  ATR14:    %.2f", localResult.ATR14)
	t.Logf("  SMA20:    %.2f", localResult.SMA20)

	// AI 计算
	t.Log("\n===== 调用 AI 计算 =====")
	prompt := buildIndicatorPrompt(klines)
	t.Logf("Prompt 长度: %d 字符", len(prompt))

	start := time.Now()
	resp, err := agent.Chat(ctx, prompt)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("AI 调用失败: %v", err)
	}

	t.Logf("AI 响应耗时: %v", elapsed)
	t.Logf("AI 原始响应:\n%s", resp.Output.Text)

	// 解析 AI 结果
	aiResult, err := extractJSONFromResponse(resp.Output.Text)
	if err != nil {
		t.Fatalf("解析 AI 结果失败: %v", err)
	}

	t.Log("\n===== AI 计算结果 =====")
	t.Logf("  EMA12:    %.2f", aiResult.EMA12)
	t.Logf("  EMA26:    %.2f", aiResult.EMA26)
	t.Logf("  MACD:     %.2f", aiResult.MACD)
	t.Logf("  RSI14:    %.2f", aiResult.RSI14)
	t.Logf("  BOLL上轨: %.2f", aiResult.BOLLUp)
	t.Logf("  BOLL中轨: %.2f", aiResult.BOLLMid)
	t.Logf("  BOLL下轨: %.2f", aiResult.BOLLLow)
	t.Logf("  ATR14:    %.2f", aiResult.ATR14)
	t.Logf("  SMA20:    %.2f", aiResult.SMA20)

	// 对比结果
	t.Log("\n===== 误差对比 (%) =====")
	errors := compareIndicators(localResult, aiResult)

	totalError := 0.0
	for name, errPct := range errors {
		status := "✓"
		if errPct > 5 {
			status = "⚠"
		}
		if errPct > 10 {
			status = "✗"
		}
		t.Logf("  %s %s: %.2f%%", status, name, errPct)
		totalError += errPct
	}

	avgError := totalError / float64(len(errors))
	t.Logf("\n  平均误差: %.2f%%", avgError)

	if avgError > 10 {
		t.Logf("警告: AI 计算误差较大，可能算法理解有差异")
	} else if avgError < 5 {
		t.Log("AI 计算精度良好！")
	}
}

// TestQwenIndicatorWithRealKlines 使用真实 K 线测试
func TestQwenIndicatorWithRealKlines(t *testing.T) {
	// 尝试获取真实 K 线数据
	client := market.NewAPIClient()
	klines, err := client.GetKlines("BTC", "1h", 30)
	if err != nil {
		t.Skipf("获取真实 K 线失败，跳过测试: %v", err)
		return
	}

	if len(klines) < 26 {
		t.Skipf("K 线数量不足: %d", len(klines))
		return
	}

	agent := NewQwenAgent(QwenAppID, QwenAPIKey)
	ctx := context.Background()

	t.Logf("获取到 %d 根 BTC 1h K线", len(klines))
	t.Log("最新价格:", klines[len(klines)-1].Close)

	// 本地计算
	localResult := calculateLocalIndicators(klines)
	t.Log("\n===== 本地计算 =====")
	t.Logf("  EMA12: %.2f, EMA26: %.2f, MACD: %.2f", localResult.EMA12, localResult.EMA26, localResult.MACD)
	t.Logf("  RSI14: %.2f", localResult.RSI14)
	t.Logf("  BOLL: %.2f / %.2f / %.2f", localResult.BOLLUp, localResult.BOLLMid, localResult.BOLLLow)

	// AI 计算
	prompt := buildIndicatorPrompt(klines)
	resp, err := agent.Chat(ctx, prompt)
	if err != nil {
		t.Fatalf("AI 调用失败: %v", err)
	}

	t.Log("\n===== AI 响应 =====")
	t.Log(resp.Output.Text)

	aiResult, err := extractJSONFromResponse(resp.Output.Text)
	if err != nil {
		t.Logf("解析失败: %v", err)
		return
	}

	// 对比
	errors := compareIndicators(localResult, aiResult)
	t.Log("\n===== 误差 =====")
	for name, errPct := range errors {
		t.Logf("  %s: %.2f%%", name, errPct)
	}
}

// TestQwenIndicatorMultiTimeframe 测试多个时间周期
func TestQwenIndicatorMultiTimeframe(t *testing.T) {
	agent := NewQwenAgent(QwenAppID, QwenAPIKey)
	ctx := context.Background()

	timeframes := []struct {
		name  string
		count int
		price float64
	}{
		{"5m周期", 30, 95000},
		{"1h周期", 50, 95000},
		{"4h周期", 40, 95000},
	}

	for _, tf := range timeframes {
		t.Run(tf.name, func(t *testing.T) {
			klines := generateTestKlines(tf.count, tf.price)

			localResult := calculateLocalIndicators(klines)

			// 简化的 prompt
			prompt := buildSimpleIndicatorPrompt(klines)

			resp, err := agent.Chat(ctx, prompt)
			if err != nil {
				t.Fatalf("AI 调用失败: %v", err)
			}

			aiResult, err := extractJSONFromResponse(resp.Output.Text)
			if err != nil {
				t.Logf("解析失败: %v", err)
				t.Logf("AI 响应: %s", resp.Output.Text[:min(500, len(resp.Output.Text))])
				return
			}

			errors := compareIndicators(localResult, aiResult)

			// 计算平均误差
			total := 0.0
			for _, e := range errors {
				total += e
			}
			avgErr := total / float64(len(errors))

			t.Logf("本地 MACD: %.2f, AI MACD: %.2f, 误差: %.2f%%", localResult.MACD, aiResult.MACD, errors["MACD"])
			t.Logf("本地 RSI: %.2f, AI RSI: %.2f, 误差: %.2f%%", localResult.RSI14, aiResult.RSI14, errors["RSI14"])
			t.Logf("平均误差: %.2f%%", avgErr)
		})

		time.Sleep(2 * time.Second) // 避免请求过快
	}
}

// 简化的 prompt
func buildSimpleIndicatorPrompt(klines []market.Kline) string {
	// 只提供收盘价序列，减少 token
	var prices []string
	for _, k := range klines {
		prices = append(prices, fmt.Sprintf("%.2f", k.Close))
	}

	return fmt.Sprintf(`收盘价序列（从旧到新）: [%s]

请计算技术指标并返回 JSON：
- ema12: 12周期EMA
- ema26: 26周期EMA
- macd: EMA12-EMA26
- rsi14: 14周期RSI(Wilder平滑)
- boll_upper, boll_middle, boll_lower: 20周期BOLL(2倍标准差)
- atr14: 0 (无高低价数据)
- sma20: 20周期SMA

只返回JSON格式：{"ema12":数值,"ema26":数值,...}`, strings.Join(prices, ","))
}

// TestQwenIndicatorAccuracy 精度测试：使用简单数据验证算法
func TestQwenIndicatorAccuracy(t *testing.T) {
	agent := NewQwenAgent(QwenAppID, QwenAPIKey)
	ctx := context.Background()

	// 使用简单递增数据，便于验证
	prices := []float64{
		100, 101, 102, 103, 104, 105, 106, 107, 108, 109, // 1-10
		110, 111, 112, 113, 114, 115, 116, 117, 118, 119, // 11-20
		120, 121, 122, 123, 124, 125, 126, 127, 128, 129, // 21-30
	}

	// 构建 K 线
	klines := make([]market.Kline, len(prices))
	for i, p := range prices {
		klines[i] = market.Kline{
			Open:  p - 0.5,
			High:  p + 1,
			Low:   p - 1,
			Close: p,
		}
	}

	// 本地计算
	localResult := calculateLocalIndicators(klines)

	t.Log("===== 简单递增数据测试 =====")
	t.Logf("价格序列: %v", prices)
	t.Logf("本地计算:")
	t.Logf("  SMA20 = %.4f (理论值: 119.5)", localResult.SMA20)
	t.Logf("  EMA12 = %.4f", localResult.EMA12)
	t.Logf("  RSI14 = %.4f (持续上涨应接近100)", localResult.RSI14)

	// AI 计算
	var priceStrs []string
	for _, p := range prices {
		priceStrs = append(priceStrs, strconv.FormatFloat(p, 'f', 0, 64))
	}

	prompt := fmt.Sprintf(`收盘价序列: [%s]

请计算:
1. SMA20 (20周期简单移动平均)
2. EMA12 (12周期指数移动平均，初始值用SMA，乘数=2/13)
3. RSI14 (14周期RSI，Wilder平滑法)

返回JSON: {"sma20":数值,"ema12":数值,"rsi14":数值}
只返回JSON`, strings.Join(priceStrs, ","))

	resp, err := agent.Chat(ctx, prompt)
	if err != nil {
		t.Fatalf("AI 调用失败: %v", err)
	}

	t.Logf("\nAI 响应: %s", resp.Output.Text)

	// 简单解析
	var aiSimple struct {
		SMA20 float64 `json:"sma20"`
		EMA12 float64 `json:"ema12"`
		RSI14 float64 `json:"rsi14"`
	}

	text := resp.Output.Text
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start != -1 && end > start {
		json.Unmarshal([]byte(text[start:end+1]), &aiSimple)
	}

	t.Logf("\nAI 计算:")
	t.Logf("  SMA20 = %.4f", aiSimple.SMA20)
	t.Logf("  EMA12 = %.4f", aiSimple.EMA12)
	t.Logf("  RSI14 = %.4f", aiSimple.RSI14)

	// 验证 SMA20 (理论值应该是 110+...+129 的平均 = 119.5)
	expectedSMA := 119.5
	if math.Abs(aiSimple.SMA20-expectedSMA) < 0.1 {
		t.Log("\n✓ AI 的 SMA20 计算正确!")
	} else {
		t.Logf("\n✗ AI 的 SMA20 有误差，期望 %.2f", expectedSMA)
	}
}

// coinankKlinesToMarket 将 coinank K线转换为 market.Kline
func coinankKlinesToMarket(klines []coinank.KlineResult) []market.Kline {
	result := make([]market.Kline, len(klines))
	for i, k := range klines {
		result[i] = market.Kline{
			OpenTime:  k.StartTime,
			Open:      k.Open,
			High:      k.High,
			Low:       k.Low,
			Close:     k.Close,
			Volume:    k.Volume,
			CloseTime: k.EndTime,
		}
	}
	return result
}

// TestQwenETHMultiTimeframe 使用 Coinank 免费 API 获取真实 ETH 数据测试多周期指标
func TestQwenETHMultiTimeframe(t *testing.T) {
	ctx := context.Background()
	agent := NewQwenAgent(QwenAppID, QwenAPIKey)

	// 测试多个时间周期
	timeframes := []struct {
		name     string
		interval coinank_enum.Interval
		size     int
	}{
		{"5分钟", coinank_enum.Minute5, 50},
		{"1小时", coinank_enum.Hour1, 50},
		{"4小时", coinank_enum.Hour4, 50},
		{"日线", coinank_enum.Day1, 30},
	}

	now := time.Now()

	for _, tf := range timeframes {
		t.Run(tf.name, func(t *testing.T) {
			// 使用 coinank 免费 API 获取 ETH K线数据
			coinankKlines, err := coinank_api.Kline(ctx, "ETHUSDT", coinank_enum.Binance,
				now.UnixMilli(), coinank_enum.To, tf.size, tf.interval)
			if err != nil {
				t.Fatalf("获取 %s K线失败: %v", tf.name, err)
			}

			if len(coinankKlines) < 26 {
				t.Skipf("K线数量不足: %d", len(coinankKlines))
				return
			}

			// 转换为 market.Kline
			klines := coinankKlinesToMarket(coinankKlines)

			t.Logf("获取到 %d 根 ETH %s K线", len(klines), tf.name)
			t.Logf("最新收盘价: %.2f, 时间: %s",
				klines[len(klines)-1].Close,
				time.UnixMilli(klines[len(klines)-1].CloseTime).Format("2006-01-02 15:04"))

			// 本地计算
			localResult := calculateLocalIndicators(klines)
			t.Log("\n===== 本地计算 =====")
			t.Logf("  EMA12: %.2f, EMA26: %.2f, MACD: %.4f",
				localResult.EMA12, localResult.EMA26, localResult.MACD)
			t.Logf("  RSI14: %.2f", localResult.RSI14)
			t.Logf("  BOLL: %.2f / %.2f / %.2f",
				localResult.BOLLUp, localResult.BOLLMid, localResult.BOLLLow)
			t.Logf("  ATR14: %.4f", localResult.ATR14)

			// AI 计算 - 使用简化 prompt（只发收盘价）
			prompt := buildSimpleIndicatorPrompt(klines)
			t.Logf("\nPrompt 长度: %d 字符", len(prompt))

			start := time.Now()
			resp, err := agent.Chat(ctx, prompt)
			elapsed := time.Since(start)

			if err != nil {
				t.Fatalf("AI 调用失败: %v", err)
			}

			t.Logf("AI 响应耗时: %v", elapsed)

			// 解析 AI 结果
			aiResult, err := extractJSONFromResponse(resp.Output.Text)
			if err != nil {
				t.Logf("AI 原始响应:\n%s", resp.Output.Text[:min(500, len(resp.Output.Text))])
				t.Fatalf("解析失败: %v", err)
			}

			t.Log("\n===== AI 计算 =====")
			t.Logf("  EMA12: %.2f, EMA26: %.2f, MACD: %.4f",
				aiResult.EMA12, aiResult.EMA26, aiResult.MACD)
			t.Logf("  RSI14: %.2f", aiResult.RSI14)
			t.Logf("  BOLL: %.2f / %.2f / %.2f",
				aiResult.BOLLUp, aiResult.BOLLMid, aiResult.BOLLLow)

			// 对比误差
			t.Log("\n===== 误差对比 =====")
			errors := compareIndicators(localResult, aiResult)
			totalErr := 0.0
			for name, errPct := range errors {
				status := "✓"
				if errPct > 1 {
					status = "⚠"
				}
				if errPct > 5 {
					status = "✗"
				}
				t.Logf("  %s %-10s: %.2f%%", status, name, errPct)
				totalErr += errPct
			}

			avgErr := totalErr / float64(len(errors))
			t.Logf("\n  平均误差: %.2f%%", avgErr)

			if avgErr < 1 {
				t.Log("  ✓ AI 计算精度优秀!")
			} else if avgErr < 5 {
				t.Log("  ⚠ AI 计算精度良好")
			} else {
				t.Log("  ✗ AI 计算误差较大")
			}

			// 等待避免请求过快
			time.Sleep(2 * time.Second)
		})
	}
}

// TestQwenETHIndicatorComparison ETH 指标对比：使用 Coinank 免费 API + Qwen 标准 API
func TestQwenETHIndicatorComparison(t *testing.T) {
	ctx := context.Background()
	agent := NewQwenAgent(QwenAppID, QwenAPIKey)

	// 使用 coinank 免费 API 获取 ETH 1小时 K线
	now := time.Now()
	coinankKlines, err := coinank_api.Kline(ctx, "ETHUSDT", coinank_enum.Binance,
		now.UnixMilli(), coinank_enum.To, 30, coinank_enum.Hour1)
	if err != nil {
		t.Fatalf("获取 K线失败: %v", err)
	}

	// 转换为 market.Kline
	klines := coinankKlinesToMarket(coinankKlines)

	t.Logf("获取到 %d 根 ETH 1h K线", len(klines))

	// 只用收盘价，简化 prompt
	var prices []string
	for _, k := range klines {
		prices = append(prices, fmt.Sprintf("%.2f", k.Close))
	}

	// 本地计算
	localResult := calculateLocalIndicators(klines)

	t.Log("\n===== 本地计算结果 =====")
	t.Logf("SMA20: %.2f", localResult.SMA20)
	t.Logf("EMA12: %.2f", localResult.EMA12)
	t.Logf("EMA26: %.2f", localResult.EMA26)
	t.Logf("MACD:  %.4f", localResult.MACD)
	t.Logf("RSI14: %.2f", localResult.RSI14)

	// 简化的 AI prompt
	prompt := fmt.Sprintf(`ETH 最近30根1小时K线收盘价（从旧到新）:
[%s]

请计算以下指标并返回纯 JSON:
1. sma20: 最后20个价格的简单移动平均
2. ema12: 12周期EMA（初始值用前12个价格的SMA，乘数=2/13）
3. ema26: 26周期EMA（初始值用前26个价格的SMA，乘数=2/27）
4. macd: EMA12 - EMA26
5. rsi14: 14周期RSI（Wilder平滑法）

只返回JSON格式: {"sma20":数值,"ema12":数值,"ema26":数值,"macd":数值,"rsi14":数值}
不要任何解释文字`, strings.Join(prices, ", "))

	t.Logf("\n发送 Prompt (%d 字符)", len(prompt))

	// 使用标准 API
	resp, err := agent.ChatWithModel(ctx, "qwen-max", prompt)
	if err != nil {
		t.Fatalf("AI 调用失败: %v", err)
	}

	aiText := resp.GetContent()
	t.Logf("\nAI 响应:\n%s", aiText)

	// 解析
	var aiResult struct {
		SMA20 float64 `json:"sma20"`
		EMA12 float64 `json:"ema12"`
		EMA26 float64 `json:"ema26"`
		MACD  float64 `json:"macd"`
		RSI14 float64 `json:"rsi14"`
	}

	start := strings.Index(aiText, "{")
	end := strings.LastIndex(aiText, "}")
	if start != -1 && end > start {
		if err := json.Unmarshal([]byte(aiText[start:end+1]), &aiResult); err != nil {
			t.Logf("JSON 解析失败: %v", err)
		}
	}

	t.Log("\n===== AI 计算结果 =====")
	t.Logf("SMA20: %.2f", aiResult.SMA20)
	t.Logf("EMA12: %.2f", aiResult.EMA12)
	t.Logf("EMA26: %.2f", aiResult.EMA26)
	t.Logf("MACD:  %.4f", aiResult.MACD)
	t.Logf("RSI14: %.2f", aiResult.RSI14)

	// 计算误差
	t.Log("\n===== 误差 =====")
	calcErr := func(name string, local, ai float64) {
		if local == 0 {
			t.Logf("  %s: 本地=0, AI=%.2f", name, ai)
			return
		}
		errPct := math.Abs(local-ai) / math.Abs(local) * 100
		status := "✓"
		if errPct > 1 {
			status = "⚠"
		}
		if errPct > 5 {
			status = "✗"
		}
		t.Logf("  %s %s: 本地=%.2f, AI=%.2f, 误差=%.2f%%", status, name, local, ai, errPct)
	}

	calcErr("SMA20", localResult.SMA20, aiResult.SMA20)
	calcErr("EMA12", localResult.EMA12, aiResult.EMA12)
	calcErr("EMA26", localResult.EMA26, aiResult.EMA26)
	calcErr("MACD", localResult.MACD, aiResult.MACD)
	calcErr("RSI14", localResult.RSI14, aiResult.RSI14)
}
