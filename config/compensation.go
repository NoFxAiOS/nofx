package config

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// CompensationStatus 补偿任务状态
type CompensationStatus string

const (
	CompensationStatusPending   CompensationStatus = "pending"   // 待处理
	CompensationStatusCompleted CompensationStatus = "completed" // 已完成
	CompensationStatusFailed    CompensationStatus = "failed"    // 失败
)

// CompensationTask 补偿任务
type CompensationTask struct {
	ID            string             `json:"id"`
	TradeID       string             `json:"trade_id"`
	UserID        string             `json:"user_id"`
	Symbol        string             `json:"symbol"`
	Action        string             `json:"action"`
	TraderID      string             `json:"trader_id"`
	RetryCount    int                `json:"retry_count"`
	MaxRetries    int                `json:"max_retries"`
	Status        CompensationStatus `json:"status"`
	ErrorMessage  string             `json:"error_message"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
	LastAttemptAt *time.Time         `json:"last_attempt_at,omitempty"`
}

// CreateCompensationTask 创建补偿任务
func (d *Database) CreateCompensationTask(task *CompensationTask) error {
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	task.Status = CompensationStatusPending

	_, err := d.exec(`
		INSERT INTO credit_compensation_tasks
		(id, trade_id, user_id, symbol, action, trader_id, retry_count, max_retries, status, error_message, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, task.ID, task.TradeID, task.UserID, task.Symbol, task.Action, task.TraderID,
		task.RetryCount, task.MaxRetries, task.Status, task.ErrorMessage, task.CreatedAt, task.UpdatedAt)

	if err != nil {
		return fmt.Errorf("创建补偿任务失败: %w", err)
	}

	log.Printf("✅ 创建补偿任务: %s (tradeID: %s, userID: %s)", task.ID, task.TradeID, task.UserID)
	return nil
}

// GetPendingCompensationTasks 获取待处理的补偿任务
func (d *Database) GetPendingCompensationTasks() ([]*CompensationTask, error) {
	rows, err := d.query(`
		SELECT id, trade_id, user_id, symbol, action, trader_id, retry_count, max_retries,
		       status, error_message, created_at, updated_at, last_attempt_at
		FROM credit_compensation_tasks
		WHERE status = $1
		ORDER BY created_at ASC
	`, CompensationStatusPending)
	if err != nil {
		return nil, fmt.Errorf("获取待处理补偿任务失败: %w", err)
	}
	defer rows.Close()

	var tasks []*CompensationTask
	for rows.Next() {
		var task CompensationTask
		var lastAttemptAt sql.NullTime

		err := rows.Scan(
			&task.ID, &task.TradeID, &task.UserID, &task.Symbol, &task.Action, &task.TraderID,
			&task.RetryCount, &task.MaxRetries, &task.Status, &task.ErrorMessage,
			&task.CreatedAt, &task.UpdatedAt, &lastAttemptAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描补偿任务失败: %w", err)
		}

		if lastAttemptAt.Valid {
			task.LastAttemptAt = &lastAttemptAt.Time
		}

		tasks = append(tasks, &task)
	}

	return tasks, nil
}

// MarkCompensationComplete 标记补偿任务完成
func (d *Database) MarkCompensationComplete(taskID string) error {
	_, err := d.exec(`
		UPDATE credit_compensation_tasks
		SET status = $1, updated_at = $2
		WHERE id = $3
	`, CompensationStatusCompleted, time.Now(), taskID)

	if err != nil {
		return fmt.Errorf("标记补偿任务完成失败: %w", err)
	}

	log.Printf("✅ 补偿任务完成: %s", taskID)
	return nil
}

// IncrementCompensationRetry 增加补偿任务重试次数
func (d *Database) IncrementCompensationRetry(taskID string, errorMessage string) error {
	now := time.Now()
	_, err := d.exec(`
		UPDATE credit_compensation_tasks
		SET retry_count = retry_count + 1,
		    last_attempt_at = $1,
		    error_message = $2,
		    updated_at = $3
		WHERE id = $4
	`, now, errorMessage, now, taskID)

	if err != nil {
		return fmt.Errorf("增加补偿任务重试次数失败: %w", err)
	}

	return nil
}

// DeleteCompensationTask 删除补偿任务
func (d *Database) DeleteCompensationTask(taskID string) error {
	_, err := d.exec(`
		DELETE FROM credit_compensation_tasks
		WHERE id = $1
	`, taskID)

	if err != nil {
		return fmt.Errorf("删除补偿任务失败: %w", err)
	}

	return nil
}

// CheckTransactionExistsForCompensation 检查交易流水是否存在（用于补偿）
func (d *Database) CheckTransactionExistsForCompensation(tradeID string) (bool, error) {
	var exists bool
	err := d.queryRow(`
		SELECT EXISTS(SELECT 1 FROM credit_transactions WHERE reference_id = $1)
	`, tradeID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("检查交易流水失败: %w", err)
	}
	return exists, nil
}
