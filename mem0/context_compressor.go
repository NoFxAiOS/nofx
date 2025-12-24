package mem0

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// ContextCompressor P0修复#2: Token预算管理
// 作用: 把Mem0返回的3400 tokens压缩到700 tokens
// 防止: Token预算超限,AI模型上下文爆炸
// 策略: 按相关性排序 → 累积token → 去重 → 去冗余
type ContextCompressor struct {
	maxTokens     int
	tokenPerChar  float64 // 平均每个字符的token数
	maxMemories   int     // 最多保留的记忆数
	deduplicator  *Deduplicator
	metrics       *CompressionMetrics
	mu            sync.RWMutex
}

// CompressionMetrics 压缩指标
type CompressionMetrics struct {
	CompressionRuns  int64
	AvgInputTokens   float64
	AvgOutputTokens  float64
	AvgCompressionRatio float64
	TotalRemoved     int64
	LastCompressionAt *time.Time
	mu               sync.RWMutex
}

// Deduplicator 去重器 (LRU缓存)
type Deduplicator struct {
	seenContent map[string]bool // 已见过的内容Hash
	addedOrder  []string        // 添加顺序 (用于LRU淘汰)
	similarity  float64          // 相似度阈值(0-1)
	maxSize     int              // ✅ 修复: 最大容量限制 (防止内存泄漏)
	mu          sync.RWMutex
}

// CompressionResult 压缩结果
type CompressionResult struct {
	Memories       []Memory       // 保留的记忆
	InputTokens    int            // 输入token数
	OutputTokens   int            // 输出token数
	CompressRatio  float64        // 压缩率 (output/input)
	RemovedCount   int            // 移除的记忆数
	DeduplicatedCount int         // 去重的记忆数
	Timestamp      time.Time
}

// NewContextCompressor 创建上下文压缩器
func NewContextCompressor(maxTokens int) *ContextCompressor {
	return &ContextCompressor{
		maxTokens:    maxTokens,
		tokenPerChar: 0.25,  // 平均4个字符 = 1个token
		maxMemories:  20,    // 最多保留20条记忆
		deduplicator: &Deduplicator{
			seenContent: make(map[string]bool),
			addedOrder:  make([]string, 0),
			similarity:  0.85,  // 85%相似视为重复
			maxSize:     5000,  // ✅ 修复: LRU最大容量 (防止无限增长)
		},
		metrics: &CompressionMetrics{},
	}
}

// Compress 压缩Mem0返回的结果
func (cc *ContextCompressor) Compress(memories []Memory) CompressionResult {
	startTime := time.Now()

	result := CompressionResult{
		Memories:  make([]Memory, 0),
		Timestamp: startTime,
	}

	if len(memories) == 0 {
		cc.recordMetrics(result)
		return result
	}

	// Step 1: 计算输入token数
	for _, m := range memories {
		result.InputTokens += cc.estimateTokens(m.Content)
		if m.Metadata != nil {
			for _, v := range m.Metadata {
				result.InputTokens += cc.estimateTokens(fmt.Sprintf("%v", v))
			}
		}
	}

	log.Printf("📥 输入: %d条记忆, %d tokens", len(memories), result.InputTokens)

	// Step 2: 按相关性排序
	sorted := cc.sortByRelevance(memories)

	// Step 3: 去重 + 累积token直到达到limit
	currentTokens := 0
	deduped := 0
	kept := 0

	for _, m := range sorted {
		// 检查去重
		if cc.deduplicator.IsDuplicate(m.Content) {
			deduped++
			result.DeduplicatedCount++
			log.Printf("  ⚠️ 去重: %s (相似度高)", idPrefix(m.ID))
			continue
		}

		memTokens := cc.estimateTokens(m.Content)

		// 检查是否超过token限制
		if currentTokens+memTokens > cc.maxTokens || kept >= cc.maxMemories {
			result.RemovedCount++
			log.Printf("  ❌ 移除: %s (超token限制或数量限制)", idPrefix(m.ID))
			continue
		}

		// 保留此记忆
		result.Memories = append(result.Memories, m)
		currentTokens += memTokens
		kept++
		result.OutputTokens += memTokens

		cc.deduplicator.Add(m.Content)
		log.Printf("  ✅ 保留: %s (Q=%.2f, %d tokens)", idPrefix(m.ID), m.QualityScore, memTokens)
	}

	// Step 4: 计算压缩率
	if result.InputTokens > 0 {
		result.CompressRatio = float64(result.OutputTokens) / float64(result.InputTokens)
	}

	duration := time.Since(startTime)
	log.Printf("✅ 压缩完成: %d/%d保留, %d去重, %d移除 (耗时: %.0fms)",
		kept, len(memories), deduped, result.RemovedCount, duration.Seconds()*1000)
	log.Printf("   Input: %d tokens → Output: %d tokens (比率: %.1f%%)",
		result.InputTokens, result.OutputTokens, result.CompressRatio*100)

	cc.recordMetrics(result)
	return result
}

// sortByRelevance 按相关性排序(质量分 + 相似度)
func (cc *ContextCompressor) sortByRelevance(memories []Memory) []Memory {
	sorted := make([]Memory, len(memories))
	copy(sorted, memories)

	sort.Slice(sorted, func(i, j int) bool {
		// 优先级: 质量分 > 相似度 > 新旧
		scoreI := sorted[i].QualityScore*0.5 + sorted[i].Similarity*0.5
		scoreJ := sorted[j].QualityScore*0.5 + sorted[j].Similarity*0.5

		if scoreI != scoreJ {
			return scoreI > scoreJ // 高分优先
		}

		// 同分数,新的优先
		return sorted[i].UpdatedAt.After(sorted[j].UpdatedAt)
	})

	return sorted
}

// estimateTokens 估算字符串的token数(简单线性估计)
func (cc *ContextCompressor) estimateTokens(s string) int {
	if s == "" {
		return 0
	}

	// 中文: 1个字符 = 1.3个token
	// 英文: 4个字符 = 1个token
	chineseCount := 0
	englishCount := 0

	for _, ch := range s {
		if ch >= 0x4E00 && ch <= 0x9FFF {
			chineseCount++
		} else if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			englishCount++
		}
	}

	tokens := int(float64(chineseCount)*1.3 + float64(englishCount)/4.0)
	if tokens == 0 && len(s) > 0 {
		tokens = 1
	}

	return tokens
}

// ===== Deduplicator Methods =====

// IsDuplicate 检查是否重复
func (d *Deduplicator) IsDuplicate(content string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	normalized := strings.TrimSpace(strings.ToLower(content))

	// 精确匹配
	if d.seenContent[normalized] {
		return true
	}

	// 相似度检查(简单实现)
	for seen := range d.seenContent {
		if d.calculateSimilarity(normalized, seen) > d.similarity {
			return true
		}
	}

	return false
}

// Add 添加内容到去重集合 (LRU淘汰)
func (d *Deduplicator) Add(content string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	normalized := strings.TrimSpace(strings.ToLower(content))

	// 如果已存在,不重复添加
	if d.seenContent[normalized] {
		return
	}

	// ✅ 修复: LRU淘汰机制 (防止无限增长)
	// 当达到最大容量时,删除最旧的条目
	if len(d.seenContent) >= d.maxSize {
		if len(d.addedOrder) > 0 {
			oldest := d.addedOrder[0]
			delete(d.seenContent, oldest)
			d.addedOrder = d.addedOrder[1:]

			log.Printf("  🗑️ LRU淘汰: 删除最旧的条目 (%d字符)",
				len(oldest))
		}
	}

	// 添加新内容
	d.seenContent[normalized] = true
	d.addedOrder = append(d.addedOrder, normalized)
}

// calculateSimilarity 计算两个字符串的相似度(Jaccard)
func (d *Deduplicator) calculateSimilarity(s1, s2 string) float64 {
	// 简单的基于单词的Jaccard相似度
	words1 := strings.Fields(s1)
	words2 := strings.Fields(s2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0
	}

	// 转换为Set
	set1 := make(map[string]bool)
	set2 := make(map[string]bool)

	for _, w := range words1 {
		set1[w] = true
	}
	for _, w := range words2 {
		set2[w] = true
	}

	// 计算交集和并集
	intersection := 0
	union := len(set1) + len(set2)

	for w := range set1 {
		if set2[w] {
			intersection++
			union--
		}
	}

	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// Clear 清空去重集合
func (d *Deduplicator) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.seenContent = make(map[string]bool)
}

// ===== Metrics Methods =====

func (cc *ContextCompressor) recordMetrics(result CompressionResult) {
	cc.metrics.mu.Lock()
	defer cc.metrics.mu.Unlock()

	n := cc.metrics.CompressionRuns + 1

	// 更新平均值
	oldAvgInput := cc.metrics.AvgInputTokens * float64(cc.metrics.CompressionRuns)
	cc.metrics.AvgInputTokens = (oldAvgInput + float64(result.InputTokens)) / float64(n)

	oldAvgOutput := cc.metrics.AvgOutputTokens * float64(cc.metrics.CompressionRuns)
	cc.metrics.AvgOutputTokens = (oldAvgOutput + float64(result.OutputTokens)) / float64(n)

	oldAvgRatio := cc.metrics.AvgCompressionRatio * float64(cc.metrics.CompressionRuns)
	cc.metrics.AvgCompressionRatio = (oldAvgRatio + result.CompressRatio) / float64(n)

	cc.metrics.CompressionRuns = n
	cc.metrics.TotalRemoved += int64(result.RemovedCount)
	now := time.Now()
	cc.metrics.LastCompressionAt = &now
}

// GetMetrics 获取压缩指标
func (cc *ContextCompressor) GetMetrics() CompressionMetrics {
	cc.metrics.mu.RLock()
	defer cc.metrics.mu.RUnlock()

	return *cc.metrics
}

// idPrefix 安全地获取ID前缀(最多8个字符)
func idPrefix(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// PrintStats 打印统计信息
func (cc *ContextCompressor) PrintStats() {
	metrics := cc.GetMetrics()

	log.Println("\n📦 上下文压缩统计:")
	log.Println(strings.Repeat("═", 60))
	log.Printf("  压缩次数: %d\n", metrics.CompressionRuns)

	if metrics.CompressionRuns > 0 {
		log.Printf("  平均输入: %.0f tokens\n", metrics.AvgInputTokens)
		log.Printf("  平均输出: %.0f tokens\n", metrics.AvgOutputTokens)
		log.Printf("  平均压缩率: %.1f%%\n", metrics.AvgCompressionRatio*100)
		log.Printf("  总移除: %d条\n", metrics.TotalRemoved)
	}

	if metrics.LastCompressionAt != nil {
		log.Printf("  最后压缩: %s\n", metrics.LastCompressionAt.Format("2006-01-02 15:04:05"))
	}

	log.Println(strings.Repeat("═", 60))
}

// ContextCompressionExample 使用示例
func ContextCompressionExample() {
	// 创建压缩器(目标700 tokens)
	compressor := NewContextCompressor(700)

	// 模拟Mem0返回的原始结果(3400+ tokens)
	memories := []Memory{
		{
			ID:             "m1",
			Content:        "这是一个高质量的相似交易记录,包含详细的执行过程和结果分析...",
			Type:           "decision",
			QualityScore:   0.95,
			Similarity:     0.92,
			Status:         "evaluated",
		},
		{
			ID:             "m2",
			Content:        "另一个相似的交易,但质量稍低...",
			Type:           "decision",
			QualityScore:   0.72,
			Similarity:     0.88,
			Status:         "evaluated",
		},
		// ... 更多记忆
	}

	// 压缩
	result := compressor.Compress(memories)

	// 使用压缩后的结果
	fmt.Printf("✅ 压缩完成:\n")
	fmt.Printf("   输入: %d tokens → 输出: %d tokens (比率: %.1f%%)\n",
		result.InputTokens, result.OutputTokens, result.CompressRatio*100)
	fmt.Printf("   保留: %d条, 移除: %d条, 去重: %d条\n",
		len(result.Memories), result.RemovedCount, result.DeduplicatedCount)
}
