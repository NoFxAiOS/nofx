package trader

import (
	"context"
	"fmt"
	"log"
	"nofx/config"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// LoadTestConfig 负载测试配置
type LoadTestConfig struct {
	ConcurrentRequests int           // 并发请求数
	TotalRequests      int           // 总请求数
	RequestInterval    time.Duration // 请求间隔
	MaxRetries         int           // 最大重试次数
	TestDuration       time.Duration // 测试持续时间
}

// LoadTestResult 负载测试结果
type LoadTestResult struct {
	TotalRequests      int64         // 总请求数
	SuccessfulRequests int64         // 成功请求数
	FailedRequests     int64         // 失败请求数
	RetriedRequests    int64         // 重试请求数
	TotalTime          time.Duration // 总耗时
	AverageLatency     time.Duration // 平均延迟
	P95Latency         time.Duration // 95%分位延迟
	P99Latency         time.Duration // 99%分位延迟
	MaxLatency         time.Duration // 最大延迟
	MinLatency         time.Duration // 最小延迟
	Errors             []error       // 错误列表
	mu                 sync.Mutex    // 保护Errors字段
}

// CreditConsumerLoadTest 积分消费者负载测试
type CreditConsumerLoadTest struct {
	consumer  *TradeCreditConsumer
	db        *config.Database
	config    LoadTestConfig
	result    *LoadTestResult
	latencies []time.Duration // 延迟记录
}

// NewCreditConsumerLoadTest 创建负载测试
func NewCreditConsumerLoadTest(db *config.Database, config LoadTestConfig) *CreditConsumerLoadTest {
	return &CreditConsumerLoadTest{
		consumer:  NewTradeCreditConsumer(db),
		db:        db,
		config:    config,
		result:    &LoadTestResult{},
		latencies: make([]time.Duration, 0, config.TotalRequests),
	}
}

// RunLoadTest 运行负载测试
func (lt *CreditConsumerLoadTest) RunLoadTest(ctx context.Context) *LoadTestResult {
	log.Printf("🚀 开始积分消费者负载测试: 并发=%d, 总请求=%d, 间隔=%v",
		lt.config.ConcurrentRequests, lt.config.TotalRequests, lt.config.RequestInterval)

	startTime := time.Now()

	// 创建信号量控制并发
	semaphore := make(chan struct{}, lt.config.ConcurrentRequests)

	// 创建等待组
	var wg sync.WaitGroup

	// 创建速率限制器
	rateLimiter := time.NewTicker(lt.config.RequestInterval)
	defer rateLimiter.Stop()

	// 启动结果收集器
	resultChan := make(chan time.Duration, lt.config.TotalRequests)
	errorChan := make(chan error, lt.config.TotalRequests)

	// 启动统计协程
	go lt.collectResults(resultChan, errorChan)

	// 发送请求
	for i := 0; i < lt.config.TotalRequests; i++ {
		select {
		case <-ctx.Done():
			log.Printf("⏹ 测试被中断: %v", ctx.Err())
			goto done
		case <-rateLimiter.C:
			wg.Add(1)
			go lt.sendRequest(i, semaphore, &wg, resultChan, errorChan)
		}
	}

	// 等待所有请求完成
	wg.Wait()

done:
	close(resultChan)
	close(errorChan)

	lt.result.TotalTime = time.Since(startTime)
	lt.calculatePercentiles()

	log.Printf("✅ 负载测试完成: 总请求=%d, 成功=%d, 失败=%d, 耗时=%v",
		lt.result.TotalRequests, lt.result.SuccessfulRequests, lt.result.FailedRequests, lt.result.TotalTime)

	return lt.result
}

// sendRequest 发送单个请求
func (lt *CreditConsumerLoadTest) sendRequest(requestID int, semaphore chan struct{}, wg *sync.WaitGroup, resultChan chan<- time.Duration, errorChan chan<- error) {
	defer wg.Done()

	// 获取信号量
	semaphore <- struct{}{}
	defer func() { <-semaphore }()

	// 生成测试数据
	userID := fmt.Sprintf("test_user_%d", requestID%100) // 使用100个不同用户
	tradeID := fmt.Sprintf("test_trade_%d_%d", time.Now().Unix(), requestID)

	startTime := time.Now()

	// 执行预留积分操作
	reservation, err := lt.consumer.ReserveCredit(userID, tradeID)

	latency := time.Since(startTime)
	atomic.AddInt64(&lt.result.TotalRequests, 1)

	if err != nil {
		atomic.AddInt64(&lt.result.FailedRequests, 1)
		errorChan <- fmt.Errorf("请求 %d 失败: %w", requestID, err)
		return
	}

	// 模拟交易执行（90%成功率）
	if requestID%10 != 0 { // 90% 成功
		// 确认积分消耗
		err = reservation.Confirm("BTCUSDT", "buy", "test_trader")
		if err != nil {
			atomic.AddInt64(&lt.result.FailedRequests, 1)
			errorChan <- fmt.Errorf("请求 %d 确认失败: %w", requestID, err)
			return
		}
	} else {
		// 释放预留（模拟交易失败）
		err = reservation.Release()
		if err != nil {
			log.Printf("⚠️ 请求 %d 释放失败: %v", requestID, err)
		}
	}

	atomic.AddInt64(&lt.result.SuccessfulRequests, 1)
	resultChan <- latency
}

// collectResults 收集测试结果
func (lt *CreditConsumerLoadTest) collectResults(resultChan <-chan time.Duration, errorChan <-chan error) {
	for latency := range resultChan {
		lt.latencies = append(lt.latencies, latency)
	}

	for err := range errorChan {
		lt.result.mu.Lock()
		lt.result.Errors = append(lt.result.Errors, err)
		lt.result.mu.Unlock()
	}
}

// calculatePercentiles 计算延迟分位数
func (lt *CreditConsumerLoadTest) calculatePercentiles() {
	if len(lt.latencies) == 0 {
		return
	}

	// 排序延迟
	sortedLatencies := make([]time.Duration, len(lt.latencies))
	copy(sortedLatencies, lt.latencies)

	// 简单排序（对于大量数据可以使用更高效的算法）
	for i := 0; i < len(sortedLatencies)-1; i++ {
		for j := 0; j < len(sortedLatencies)-i-1; j++ {
			if sortedLatencies[j] > sortedLatencies[j+1] {
				sortedLatencies[j], sortedLatencies[j+1] = sortedLatencies[j+1], sortedLatencies[j]
			}
		}
	}

	// 计算统计值
	n := len(sortedLatencies)
	lt.result.MinLatency = sortedLatencies[0]
	lt.result.MaxLatency = sortedLatencies[n-1]

	// 计算平均延迟
	totalLatency := time.Duration(0)
	for _, lat := range sortedLatencies {
		totalLatency += lat
	}
	lt.result.AverageLatency = totalLatency / time.Duration(n)

	// 计算分位数
	p95Index := int(float64(n) * 0.95)
	p99Index := int(float64(n) * 0.99)

	if p95Index < n {
		lt.result.P95Latency = sortedLatencies[p95Index]
	}
	if p99Index < n {
		lt.result.P99Latency = sortedLatencies[p99Index]
	}
}

// PrintReport 打印测试报告
func (lt *CreditConsumerLoadTest) PrintReport() {
	log.Println("\n=== 📊 积分消费者负载测试报告 ===")
	log.Printf("总请求数: %d", lt.result.TotalRequests)
	log.Printf("成功请求数: %d (%.2f%%)", lt.result.SuccessfulRequests,
		float64(lt.result.SuccessfulRequests)/float64(lt.result.TotalRequests)*100)
	log.Printf("失败请求数: %d (%.2f%%)", lt.result.FailedRequests,
		float64(lt.result.FailedRequests)/float64(lt.result.TotalRequests)*100)
	log.Printf("重试请求数: %d", lt.result.RetriedRequests)
	log.Printf("总耗时: %v", lt.result.TotalTime)
	log.Printf("平均延迟: %v", lt.result.AverageLatency)
	log.Printf("最小延迟: %v", lt.result.MinLatency)
	log.Printf("最大延迟: %v", lt.result.MaxLatency)
	log.Printf("P95延迟: %v", lt.result.P95Latency)
	log.Printf("P99延迟: %v", lt.result.P99Latency)
	log.Printf("QPS: %.2f", float64(lt.result.TotalRequests)/lt.result.TotalTime.Seconds())

	if len(lt.result.Errors) > 0 {
		log.Printf("\n错误统计:")
		errorCount := make(map[string]int)
		for _, err := range lt.result.Errors {
			errorMsg := err.Error()
			if len(errorMsg) > 100 {
				errorMsg = errorMsg[:100] + "..."
			}
			errorCount[errorMsg]++
		}

		for errorMsg, count := range errorCount {
			log.Printf("  %s: %d次", errorMsg, count)
		}
	}

	log.Println("\n=== 测试配置 ===")
	log.Printf("并发数: %d", lt.config.ConcurrentRequests)
	log.Printf("总请求数: %d", lt.config.TotalRequests)
	log.Printf("请求间隔: %v", lt.config.RequestInterval)
	log.Printf("最大重试次数: %d", lt.config.MaxRetries)
}

// TestCreditConsumerLoad100 测试100并发请求
func TestCreditConsumerLoad100(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过负载测试")
	}

	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("跳过负载测试：未设置 DATABASE_URL 环境变量")
	}

	db, err := config.NewDatabase("")
	if err != nil {
		t.Fatalf("创建数据库失败: %v", err)
	}
	defer db.GetDB().Close()

	loadTest := NewCreditConsumerLoadTest(db, LoadTestConfig{
		ConcurrentRequests: 100,
		TotalRequests:      1000,
		RequestInterval:    10 * time.Millisecond,
		MaxRetries:         3,
	})

	ctx := context.Background()
	result := loadTest.RunLoadTest(ctx)

	// 验证结果
	if float64(result.SuccessfulRequests) < float64(loadTest.config.TotalRequests)*0.9 { // 90%成功率
		t.Errorf("成功率过低: %d/%d (%.2f%%)",
			result.SuccessfulRequests, result.TotalRequests,
			float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100)
	}

	if result.AverageLatency > 500*time.Millisecond {
		t.Errorf("平均延迟过高: %v", result.AverageLatency)
	}

	if result.P99Latency > 1*time.Second {
		t.Errorf("P99延迟过高: %v", result.P99Latency)
	}

	loadTest.PrintReport()
}

// TestCreditConsumerLoad500 测试500并发请求
func TestCreditConsumerLoad500(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过负载测试")
	}

	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("跳过负载测试：未设置 DATABASE_URL 环境变量")
	}

	db, err := config.NewDatabase("")
	if err != nil {
		t.Fatalf("创建数据库失败: %v", err)
	}
	defer db.GetDB().Close()

	loadTest := NewCreditConsumerLoadTest(db, LoadTestConfig{
		ConcurrentRequests: 500,
		TotalRequests:      2500,
		RequestInterval:    5 * time.Millisecond,
		MaxRetries:         3,
	})

	ctx := context.Background()
	result := loadTest.RunLoadTest(ctx)

	// 验证结果
	if float64(result.SuccessfulRequests) < float64(loadTest.config.TotalRequests)*0.85 { // 85%成功率
		t.Errorf("成功率过低: %d/%d (%.2f%%)",
			result.SuccessfulRequests, result.TotalRequests,
			float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100)
	}

	if result.AverageLatency > 1*time.Second {
		t.Errorf("平均延迟过高: %v", result.AverageLatency)
	}

	if result.P99Latency > 2*time.Second {
		t.Errorf("P99延迟过高: %v", result.P99Latency)
	}

	loadTest.PrintReport()
}

// TestCreditConsumerLoad1000 测试1000并发请求（极限测试）
func TestCreditConsumerLoad1000(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过负载测试")
	}

	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("跳过负载测试：未设置 DATABASE_URL 环境变量")
	}

	db, err := config.NewDatabase("")
	if err != nil {
		t.Fatalf("创建数据库失败: %v", err)
	}
	defer db.GetDB().Close()

	loadTest := NewCreditConsumerLoadTest(db, LoadTestConfig{
		ConcurrentRequests: 1000,
		TotalRequests:      5000,
		RequestInterval:    2 * time.Millisecond,
		MaxRetries:         3,
	})

	ctx := context.Background()
	result := loadTest.RunLoadTest(ctx)

	// 验证结果
	if float64(result.SuccessfulRequests) < float64(loadTest.config.TotalRequests)*0.8 { // 80%成功率
		t.Errorf("成功率过低: %d/%d (%.2f%%)",
			result.SuccessfulRequests, result.TotalRequests,
			float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100)
	}

	// 对于1000并发，放宽延迟要求
	if result.AverageLatency > 2*time.Second {
		t.Errorf("平均延迟过高: %v", result.AverageLatency)
	}

	if result.P99Latency > 5*time.Second {
		t.Errorf("P99延迟过高: %v", result.P99Latency)
	}

	loadTest.PrintReport()
}

// BenchmarkCreditConsumer 基准测试
func BenchmarkCreditConsumer(b *testing.B) {
	db, err := config.NewDatabase("")
	if err != nil {
		b.Fatalf("创建数据库失败: %v", err)
	}
	defer db.GetDB().Close()

	consumer := NewTradeCreditConsumer(db)
	userID := "benchmark_user"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			tradeID := fmt.Sprintf("benchmark_trade_%d_%d", time.Now().UnixNano(), i)
			reservation, err := consumer.ReserveCredit(userID, tradeID)
			if err != nil {
				b.Errorf("预留积分失败: %v", err)
				continue
			}

			err = reservation.Confirm("BTCUSDT", "buy", "benchmark_trader")
			if err != nil {
				b.Errorf("确认积分失败: %v", err)
			}
			i++
		}
	})

	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ops/s")
}
