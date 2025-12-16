package service

import (
	"context"
	"fmt"
	"log"
	"nofx/config"
	"nofx/service/credits"
	"time"
)

// CompensationService 补偿服务
type CompensationService struct {
	db             *config.Database
	creditsService credits.Service
	retryInterval  time.Duration
	stopChan       chan bool
}

// NewCompensationService 创建补偿服务
func NewCompensationService(db *config.Database, creditsService credits.Service) *CompensationService {
	return &CompensationService{
		db:             db,
		creditsService: creditsService,
		retryInterval:  5 * time.Second, // 默认5秒重试间隔
		stopChan:       make(chan bool),
	}
}

// Start 启动补偿服务
func (cs *CompensationService) Start() {
	log.Println("🚀 启动补偿服务...")
	go cs.processLoop()
}

// Stop 停止补偿服务
func (cs *CompensationService) Stop() {
	log.Println("⏹ 停止补偿服务...")
	cs.stopChan <- true
}

// processLoop 处理补偿任务的循环
func (cs *CompensationService) processLoop() {
	ticker := time.NewTicker(cs.retryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cs.ProcessPendingTasks()
		case <-cs.stopChan:
			log.Println("✅ 补偿服务已停止")
			return
		}
	}
}

// ProcessPendingTasks 处理待补偿任务
func (cs *CompensationService) ProcessPendingTasks() error {
	tasks, err := cs.db.GetPendingCompensationTasks()
	if err != nil {
		return fmt.Errorf("获取待处理补偿任务失败: %w", err)
	}

	if len(tasks) == 0 {
		return nil
	}

	log.Printf("🔄 发现 %d 个待补偿任务", len(tasks))

	for _, task := range tasks {
		err := cs.processCompensation(task)
		if err != nil {
			log.Printf("⚠️ 补偿任务 %s 处理失败: %v", task.ID, err)
			continue
		}

		log.Printf("✅ 补偿任务 %s 处理成功", task.ID)
	}

	return nil
}

// processCompensation 处理单个补偿任务
func (cs *CompensationService) processCompensation(task *config.CompensationTask) error {
	// 检查是否已处理（幂等性）
	exists, err := cs.db.CheckTransactionExistsForCompensation(task.TradeID)
	if err != nil {
		return fmt.Errorf("检查交易流水失败: %w", err)
	}

	if exists {
		// 交易流水已存在，说明积分已经扣减完成，标记任务完成
		err := cs.db.MarkCompensationComplete(task.ID)
		if err != nil {
			return fmt.Errorf("标记补偿任务完成失败: %w", err)
		}
		log.Printf("✅ 补偿任务 %s 已完成（积分已扣减）", task.ID)
		return nil
	}

	// 尝试补偿扣减积分
	ctx := context.Background()
	err = cs.creditsService.DeductCredits(ctx, task.UserID, 1, "trade",
		fmt.Sprintf("补偿扣减: %s %s by %s", task.Symbol, task.Action, task.TraderID),
		task.TradeID)

	if err != nil {
		// 补偿失败，增加重试次数
		task.RetryCount++
		if task.RetryCount >= task.MaxRetries {
			log.Printf("❌ 补偿任务 %s 达到最大重试次数 (%d)，停止补偿", task.ID, task.MaxRetries)
			// 可以考虑设置为 "failed" 状态
		} else {
			errMsg := fmt.Sprintf("补偿失败: %v", err)
			err := cs.db.IncrementCompensationRetry(task.ID, errMsg)
			if err != nil {
				log.Printf("⚠️ 增加重试次数失败: %v", err)
			}
		}
		return fmt.Errorf("补偿扣减失败: %w", err)
	}

	// 补偿成功，标记任务完成
	err = cs.db.MarkCompensationComplete(task.ID)
	if err != nil {
		return fmt.Errorf("标记补偿任务完成失败: %w", err)
	}

	log.Printf("✅ 补偿任务 %s 成功，积分已扣减", task.ID)
	return nil
}

// CreateCompensationTask 创建补偿任务
func (cs *CompensationService) CreateCompensationTask(tradeID, userID, symbol, action, traderID string) error {
	task := &config.CompensationTask{
		ID:         config.GenerateUUID(),
		TradeID:    tradeID,
		UserID:     userID,
		Symbol:     symbol,
		Action:     action,
		TraderID:   traderID,
		RetryCount: 0,
		MaxRetries: 3, // 默认最大重试3次
	}

	err := cs.db.CreateCompensationTask(task)
	if err != nil {
		return fmt.Errorf("创建补偿任务失败: %w", err)
	}

	log.Printf("📝 创建补偿任务: %s (tradeID: %s)", task.ID, tradeID)
	return nil
}
