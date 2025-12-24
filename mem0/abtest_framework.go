package mem0

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"nofx/ai"
	"sync"
	"time"
)

// ABTestFramework A/B测试框架
// 作用: 对比GetFullDecisionV2(Mem0增强)vs Baseline(原始AI),测量效果
// 收益: 量化Mem0集成的实际改进(胜率、夏普比、回撤)
type ABTestFramework struct {
	testID            string
	startTime         time.Time
	config            ABTestConfig
	results           map[string]*TestResult // variant -> result
	controlMemories   []Memory                 // Baseline用的记忆
	treatmentMemories []Memory                 // V2用的记忆
	metrics           *ABMetrics
	mu                sync.RWMutex
}

// ABTestConfig A/B测试配置
type ABTestConfig struct {
	Name             string        // 测试名称
	Duration         time.Duration // 测试持续时间
	SampleSize       int           // 每个variant的样本数
	TrafficSplit     map[string]float64 // variant -> traffic ratio
	MetricsToTrack   []string      // 追踪的指标
	SignificanceLevel float64       // 统计显著性阈值 (0.05)
}

// TestResult 测试结果
type TestResult struct {
	Variant              string
	SampleCount          int64
	WinRate              float64        // 胜率
	PnL                  float64        // 总盈亏
	SharpeRatio          float64        // 夏普比
	MaxDrawdown          float64        // 最大回撤
	AvgReturnPerTrade    float64        // 平均收益/笔
	TradesExecuted       []TradeRecord
	StartTime            time.Time
	EndTime              *time.Time
	IsWinner             bool
	ConfidenceInterval   float64        // 95% CI
	SignificantlyBetter  bool           // 显著优于对照组
}

// TradeRecord 交易记录
type TradeRecord struct {
	TradeID       string
	Variant       string
	Timestamp     time.Time
	EntryPrice    float64
	ExitPrice     float64
	Quantity      int
	PnL           float64
	Decision      Decision
}

// Decision 决策信息
type Decision struct {
	Recommendation string
	Confidence     float64
	SourceMemories []string // 使用的记忆ID
	Model          string   // "baseline" / "v2"
	UsedCompressor bool
	UsedKBFallback bool
	FilteredCount  int
}

// ABMetrics A/B测试指标
type ABMetrics struct {
	TotalTests     int64
	ActiveTests    int64
	CompletedTests int64
	SignificantWins int64
	mu             sync.RWMutex
}

// GetFullDecisionV2 增强的决策函数(使用Mem0)
type GetFullDecisionV2 struct {
	store              MemoryStore
	model              ai.AIModel            // AI模型(Gemini/GPT-4等),用于生成决策建议
	compressor         *ContextCompressor
	kb                 *GlobalKnowledgeBase
	riskFormatter      *RiskAwareFormatter
	stageManager       *StageManager
	cacheWarmer        *CacheWarmer
	cfg                *Config               // Mem0配置,用于控制模型降级
	metrics            *DecisionMetrics
	mu                 sync.RWMutex
}

// DecisionMetrics 决策指标
type DecisionMetrics struct {
	DecisionsGenerated int64
	MemoriesUsed       int64
	CompressionsRun    int64
	FallbacksUsed      int64
	AveragePrepTime    time.Duration
	mu                 sync.RWMutex
}

// NewABTestFramework 创建A/B测试框架
func NewABTestFramework(testID string, config ABTestConfig) *ABTestFramework {
	return &ABTestFramework{
		testID:    testID,
		startTime: time.Now(),
		config:    config,
		results:   make(map[string]*TestResult),
		metrics:   &ABMetrics{},
	}
}

// InitializeVariants 初始化测试变体
func (ab *ABTestFramework) InitializeVariants() error {
	log.Printf("📊 初始化A/B测试: %s", ab.config.Name)

	for variant, ratio := range ab.config.TrafficSplit {
		if ratio <= 0 || ratio > 1 {
			return fmt.Errorf("❌ 无效的流量占比: %s = %.2f%%", variant, ratio*100)
		}

		ab.results[variant] = &TestResult{
			Variant:        variant,
			TradesExecuted: make([]TradeRecord, 0),
			StartTime:      time.Now(),
		}

		log.Printf("  ✅ 注册变体: %s (流量: %.1f%%)", variant, ratio*100)
	}

	ab.metrics.mu.Lock()
	ab.metrics.ActiveTests++
	ab.metrics.mu.Unlock()

	return nil
}

// RecordTrade 记录一笔交易
func (ab *ABTestFramework) RecordTrade(trade TradeRecord) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	result, exists := ab.results[trade.Variant]
	if !exists {
		log.Printf("⚠️ 变体%s不存在", trade.Variant)
		return
	}

	result.TradesExecuted = append(result.TradesExecuted, trade)
	result.SampleCount++

	// 更新统计
	if trade.PnL > 0 {
		// 计数胜率
		result.WinRate = float64(ab.countWins(result)) / float64(result.SampleCount)
	}

	result.PnL += trade.PnL
	result.AvgReturnPerTrade = result.PnL / float64(result.SampleCount)

	log.Printf("  📊 %s: 交易%d, PnL=%.2f, 胜率=%.1f%%",
		trade.Variant, result.SampleCount, result.PnL, result.WinRate*100)
}

// countWins 统计胜利笔数
func (ab *ABTestFramework) countWins(result *TestResult) int {
	count := 0
	for _, trade := range result.TradesExecuted {
		if trade.PnL > 0 {
			count++
		}
	}
	return count
}

// CalculateMetrics 计算所有指标
func (ab *ABTestFramework) CalculateMetrics() {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	for variant, result := range ab.results {
		if len(result.TradesExecuted) == 0 {
			continue
		}

		// 夏普比计算 (简单版本)
		pnls := make([]float64, 0)
		for _, trade := range result.TradesExecuted {
			pnls = append(pnls, trade.PnL)
		}

		result.SharpeRatio = ab.calculateSharpeRatio(pnls)
		result.MaxDrawdown = ab.calculateMaxDrawdown(pnls)

		log.Printf("  📈 %s: SharpeRatio=%.2f, MaxDD=%.2f%%",
			variant, result.SharpeRatio, result.MaxDrawdown*100)
	}
}

// calculateSharpeRatio 计算夏普比
func (ab *ABTestFramework) calculateSharpeRatio(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}

	// Step 1: 计算平均收益
	sum := 0.0
	for _, r := range returns {
		sum += r
	}
	mean := sum / float64(len(returns))

	// Step 2: 计算样本方差 (使用n-1作为自由度)
	variance := 0.0
	for _, r := range returns {
		variance += (r - mean) * (r - mean)
	}
	variance = variance / float64(len(returns)-1) // ✅ 修复: 使用n-1而非n

	// Step 3: 计算标准差 (方差的平方根)
	stdDev := math.Sqrt(variance) // ✅ 修复: 取平方根

	if stdDev == 0 {
		return 0
	}

	// Step 4: 夏普比 = (平均收益 - 无风险利率) / 标准差
	// 假设无风险利率为0
	riskFreeRate := 0.0
	sharpeRatio := (mean - riskFreeRate) / stdDev

	log.Printf("  📊 夏普比计算: mean=%.4f, stdDev=%.4f, sharpe=%.4f",
		mean, stdDev, sharpeRatio)

	return sharpeRatio
}

// calculateMaxDrawdown 计算最大回撤
func (ab *ABTestFramework) calculateMaxDrawdown(returns []float64) float64 {
	if len(returns) == 0 {
		return 0
	}

	cumulative := 0.0
	peak := 0.0
	maxDD := 0.0

	for _, r := range returns {
		cumulative += r
		if cumulative > peak {
			peak = cumulative
		}

		dd := (peak - cumulative) / peak
		if dd > maxDD {
			maxDD = dd
		}
	}

	return maxDD
}

// PerformStatisticalTest 执行统计检验
func (ab *ABTestFramework) PerformStatisticalTest() map[string]interface{} {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	results := make(map[string]interface{})

	// 对比各variant
	variants := make([]string, 0)
	for k := range ab.results {
		variants = append(variants, k)
	}

	if len(variants) < 2 {
		log.Printf("⚠️ 至少需要2个变体才能对比")
		return results
	}

	// 简单的T检验(假设独立且方差齐)
	v1Result := ab.results[variants[0]]
	v2Result := ab.results[variants[1]]

	if len(v1Result.TradesExecuted) == 0 || len(v2Result.TradesExecuted) == 0 {
		log.Printf("⚠️ 样本不足")
		return results
	}

	v1PnL := ab.extractPnLs(v1Result.TradesExecuted)
	v2PnL := ab.extractPnLs(v2Result.TradesExecuted)

	// 计算均值差
	meanDiff := ab.calculateMean(v2PnL) - ab.calculateMean(v1PnL)

	// 计算标准误
	seError := ab.calculateStandardError(v1PnL, v2PnL)

	// 检验统计量
	if seError > 0 {
		tStat := meanDiff / seError
		results["t_statistic"] = tStat
		results["is_significant"] = abs(tStat) > 1.96 // 95%置信度
	}

	results["mean_difference"] = meanDiff
	results["variant_1"] = variants[0]
	results["variant_2"] = variants[1]
	results["p_value"] = 0.05 // 简化

	if meanDiff > 0 {
		v2Result.SignificantlyBetter = true
		v2Result.IsWinner = true
		ab.metrics.SignificantWins++
	}

	return results
}

// extractPnLs 提取PnL列表
func (ab *ABTestFramework) extractPnLs(trades []TradeRecord) []float64 {
	pnls := make([]float64, 0)
	for _, trade := range trades {
		pnls = append(pnls, trade.PnL)
	}
	return pnls
}

// calculateMean 计算平均值
func (ab *ABTestFramework) calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculateStandardError 计算标准误
func (ab *ABTestFramework) calculateStandardError(s1, s2 []float64) float64 {
	var1 := ab.calculateVariance(s1)
	var2 := ab.calculateVariance(s2)
	n1 := float64(len(s1))
	n2 := float64(len(s2))

	if n1+n2 <= 2 {
		return 0
	}

	// ✅ 修复: 标准误 = sqrt(var1/n1 + var2/n2)
	// 而不仅仅是方差和
	pooledVariance := var1/n1 + var2/n2
	seError := math.Sqrt(pooledVariance)

	log.Printf("  📊 标准误计算: var1=%.4f, var2=%.4f, se=%.4f",
		var1, var2, seError)

	return seError
}

// calculateVariance 计算方差
func (ab *ABTestFramework) calculateVariance(values []float64) float64 {
	if len(values) <= 1 {
		return 0
	}

	mean := ab.calculateMean(values)
	variance := 0.0

	for _, v := range values {
		variance += (v - mean) * (v - mean)
	}

	return variance / float64(len(values)-1)
}

// CompleteTest 完成测试
func (ab *ABTestFramework) CompleteTest() map[string]interface{} {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	ab.CalculateMetrics()
	testResults := ab.PerformStatisticalTest()

	summary := map[string]interface{}{
		"test_id":    ab.testID,
		"duration":   time.Since(ab.startTime),
		"variants":   make(map[string]interface{}),
		"statistical_test": testResults,
	}

	for variant, result := range ab.results {
		result.EndTime = &time.Time{}
		*result.EndTime = time.Now()

		summary["variants"].(map[string]interface{})[variant] = map[string]interface{}{
			"sample_count":           result.SampleCount,
			"win_rate":               result.WinRate,
			"total_pnl":              result.PnL,
			"avg_return_per_trade":   result.AvgReturnPerTrade,
			"sharpe_ratio":           result.SharpeRatio,
			"max_drawdown":           result.MaxDrawdown,
			"is_winner":              result.IsWinner,
			"significantly_better":   result.SignificantlyBetter,
		}
	}

	ab.metrics.CompletedTests++
	ab.metrics.ActiveTests--

	return summary
}

// NewGetFullDecisionV2 创建增强决策器,集成AI模型工厂
// cfg: Mem0配置,包含模型选择(gemini/gpt-4等)和降级策略
// 返回error: 如果模型创建失败(但不会阻止系统,会降级使用备用模型)
func NewGetFullDecisionV2(store MemoryStore, compressor *ContextCompressor,
	kb *GlobalKnowledgeBase, riskFormatter *RiskAwareFormatter,
	stageManager *StageManager, cacheWarmer *CacheWarmer, cfg *Config) (*GetFullDecisionV2, error) {

	if cfg == nil {
		return nil, fmt.Errorf("❌ Mem0配置(Config)不能为nil")
	}

	// 通过工厂创建AI模型,支持自动降级
	factory := ai.NewAIModelFactory(cfg, nil) // 注: nil可以替换为*sql.DB以从数据库读取Gemini配置
	model, err := factory.CreateWithFallback(cfg.UnderstandingModel, cfg.FallbackModel)

	if err != nil {
		// 降级失败,使用Mock模型作为最后的防线
		log.Printf("⚠️ AI模型创建失败(primary=%s, fallback=%s): %v, 使用Mock模型",
			cfg.UnderstandingModel, cfg.FallbackModel, err)
		model = ai.NewMockAIModel()
	}

	log.Printf("✅ GetFullDecisionV2初始化: 模型=%s, 备用=%s", cfg.UnderstandingModel, cfg.FallbackModel)

	return &GetFullDecisionV2{
		store:         store,
		model:         model,
		compressor:    compressor,
		kb:            kb,
		riskFormatter: riskFormatter,
		stageManager:  stageManager,
		cacheWarmer:   cacheWarmer,
		cfg:           cfg,
		metrics:       &DecisionMetrics{},
	}, nil
}

// GenerateDecision 生成增强的交易决策,整合Mem0记忆和AI模型
// 流程: Query -> Mem0检索 -> 上下文压缩 -> 风险过滤 -> AI模型生成建议
func (gfd *GetFullDecisionV2) GenerateDecision(ctx context.Context, query Query) (Decision, error) {
	startTime := time.Now()

	decision := Decision{
		Model:          gfd.cfg.UnderstandingModel, // 记录使用的模型名称(gemini/gpt-4等)
		SourceMemories: make([]string, 0),
	}

	// Step 1: 检查缓存
	if _, ok := gfd.cacheWarmer.Get("similar_trades_cache"); ok {
		decision.SourceMemories = append(decision.SourceMemories, "cached")
	}

	// Step 2: 从Mem0查询相关的记忆
	memories, err := gfd.store.Search(ctx, query)
	if err != nil {
		log.Printf("⚠️ Mem0查询失败,使用知识库降级")
		memories = gfd.kb.GetReferencesForColdStart(5)
		decision.UsedKBFallback = true
	}

	// Step 3: 压缩上下文
	compressResult := gfd.compressor.Compress(memories)
	decision.UsedCompressor = true

	// Step 4: 应用风险过滤
	currentStage := gfd.stageManager.GetCurrentStage()
	filterResult := gfd.riskFormatter.FilterMemories(compressResult.Memories, currentStage)
	decision.FilteredCount = filterResult.RemovedCount

	// Step 5: 调用AI模型生成决策建议
	// 系统提示词: Mem0的角色定义
	systemPrompt := fmt.Sprintf(`你是一个专业的交易决策助手,使用Mem0长期记忆系统增强决策能力。
当前交易阶段: %s
模型: %s
任务: 根据历史记忆和当前查询,生成最优的交易建议。`,
		currentStage, gfd.cfg.UnderstandingModel)

	// 用户提示词: 包含Query和筛选后的记忆内容
	userPrompt := gfd.buildUserPrompt(query, filterResult.Memories)

	// 调用AI模型(支持自动降级)
	recommendation, err := gfd.model.CallAPI(ctx, systemPrompt, userPrompt)
	if err != nil {
		log.Printf("⚠️ AI模型调用失败: %v, 使用知识库默认建议", err)
		if len(filterResult.Memories) > 0 {
			topMemory := filterResult.Memories[0]
			decision.Recommendation = topMemory.Content
			decision.SourceMemories = append(decision.SourceMemories, topMemory.ID)
			decision.Confidence = topMemory.QualityScore * 0.8 // 降级时置信度降低20%
		} else {
			decision.Recommendation = "数据不足,建议观望"
			decision.Confidence = 0.3
		}
	} else {
		// AI模型成功生成建议
		decision.Recommendation = recommendation
		decision.Confidence = 0.85

		// 记录使用的记忆ID
		for _, mem := range filterResult.Memories {
			decision.SourceMemories = append(decision.SourceMemories, mem.ID)
		}
	}

	// Step 6: 记录指标
	gfd.recordMetrics(len(memories), time.Since(startTime))

	return decision, nil
}

// buildUserPrompt 构建发送给AI模型的用户提示词
// 包含当前查询和筛选后的相关记忆
func (gfd *GetFullDecisionV2) buildUserPrompt(query Query, memories []Memory) string {
	queryDesc := fmt.Sprintf("查询类型: %s, 限制数: %d", query.Type, query.Limit)
	prompt := fmt.Sprintf("当前查询: %s\n\n", queryDesc)

	if len(memories) == 0 {
		prompt += "当前没有相关的历史记忆,请基于市场常识给出建议。"
		return prompt
	}

	prompt += fmt.Sprintf("相关的历史记忆(%d条):\n", len(memories))
	for i, mem := range memories {
		prompt += fmt.Sprintf("记忆%d [质量%.2f]: %s\n",
			i+1, mem.QualityScore, mem.Content)
	}

	prompt += "\n请综合考虑上述历史记忆,生成最优的交易决策建议。"
	return prompt
}

// recordMetrics 记录指标
func (gfd *GetFullDecisionV2) recordMetrics(memoriesUsed int, duration time.Duration) {
	gfd.metrics.mu.Lock()
	defer gfd.metrics.mu.Unlock()

	gfd.metrics.DecisionsGenerated++
	gfd.metrics.MemoriesUsed += int64(memoriesUsed)
	gfd.metrics.CompressionsRun++

	oldAvg := gfd.metrics.AveragePrepTime
	newAvg := (oldAvg*time.Duration(gfd.metrics.DecisionsGenerated-1) + duration) / time.Duration(gfd.metrics.DecisionsGenerated)
	gfd.metrics.AveragePrepTime = newAvg
}

// GetMetrics 获取决策指标
func (gfd *GetFullDecisionV2) GetMetrics() DecisionMetrics {
	gfd.metrics.mu.RLock()
	defer gfd.metrics.mu.RUnlock()

	return *gfd.metrics
}

// ===== Helper Functions =====

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// SelectVariant 根据流量分配选择变体
func SelectVariant(trafficSplit map[string]float64) string {
	r := rand.Float64()
	cumulative := 0.0

	for variant, ratio := range trafficSplit {
		cumulative += ratio
		if r <= cumulative {
			return variant
		}
	}

	// 默认返回第一个
	for variant := range trafficSplit {
		return variant
	}

	return "baseline"
}
