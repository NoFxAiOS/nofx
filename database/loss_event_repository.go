package database

import (
	"fmt"
	"strings"
	"time"
)

// LossEvent 亏损事件记录
type LossEvent struct {
	ID                int
	UserID            int
	TraderID          string
	EventType         string // "consecutive_loss", "daily_loss", "weekly_loss", "max_drawdown"
	EventReason       string
	ConsecutiveLosses int
	DailyLossPct      float64
	WeeklyLossPct     float64
	MaxDrawdownPct    float64
	AccountEquity     float64
	BreachTriggered   bool
	RecoveryAttempt   int
	CreatedAt         time.Time
}

// SaveLossEvent 保存亏损事件
func (db *DatabaseImpl) SaveLossEvent(event *LossEvent) error {
	query := `INSERT INTO loss_events (
		user_id, trader_id, event_type, event_reason, consecutive_losses,
		daily_loss_pct, weekly_loss_pct, max_drawdown_pct, account_equity,
		breach_triggered, recovery_attempt
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	if !db.usingNeon {
		query = strings.ReplaceAll(query, "$1", "?")
		query = strings.ReplaceAll(query, "$2", "?")
		query = strings.ReplaceAll(query, "$3", "?")
		query = strings.ReplaceAll(query, "$4", "?")
		query = strings.ReplaceAll(query, "$5", "?")
		query = strings.ReplaceAll(query, "$6", "?")
		query = strings.ReplaceAll(query, "$7", "?")
		query = strings.ReplaceAll(query, "$8", "?")
		query = strings.ReplaceAll(query, "$9", "?")
		query = strings.ReplaceAll(query, "$10", "?")
		query = strings.ReplaceAll(query, "$11", "?")
	}

	_, err := db.currentDB.Exec(query,
		event.UserID, event.TraderID, event.EventType, event.EventReason,
		event.ConsecutiveLosses, event.DailyLossPct, event.WeeklyLossPct,
		event.MaxDrawdownPct, event.AccountEquity, event.BreachTriggered,
		event.RecoveryAttempt)

	return err
}

// GetRecentLossEvents 获取最近的亏损事件
func (db *DatabaseImpl) GetRecentLossEvents(traderID string, limit int) ([]*LossEvent, error) {
	query := fmt.Sprintf(`SELECT id, user_id, trader_id, event_type, event_reason, consecutive_losses,
		daily_loss_pct, weekly_loss_pct, max_drawdown_pct, account_equity,
		breach_triggered, recovery_attempt, created_at
	FROM loss_events
	WHERE trader_id = $1
	ORDER BY created_at DESC
	LIMIT %d`, limit)

	if !db.usingNeon {
		query = strings.ReplaceAll(query, "$1", "?")
	}

	rows, err := db.currentDB.Query(query, traderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*LossEvent
	for rows.Next() {
		event := &LossEvent{}
		if err := rows.Scan(
			&event.ID, &event.UserID, &event.TraderID, &event.EventType, &event.EventReason,
			&event.ConsecutiveLosses, &event.DailyLossPct, &event.WeeklyLossPct,
			&event.MaxDrawdownPct, &event.AccountEquity, &event.BreachTriggered,
			&event.RecoveryAttempt, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

// GetLossEventsByType 按类型获取亏损事件
func (db *DatabaseImpl) GetLossEventsByType(traderID, eventType string, limit int) ([]*LossEvent, error) {
	query := fmt.Sprintf(`SELECT id, user_id, trader_id, event_type, event_reason, consecutive_losses,
		daily_loss_pct, weekly_loss_pct, max_drawdown_pct, account_equity,
		breach_triggered, recovery_attempt, created_at
	FROM loss_events
	WHERE trader_id = $1 AND event_type = $2
	ORDER BY created_at DESC
	LIMIT %d`, limit)

	if !db.usingNeon {
		query = strings.ReplaceAll(query, "$1", "?")
		query = strings.ReplaceAll(query, "$2", "?")
	}

	rows, err := db.currentDB.Query(query, traderID, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*LossEvent
	for rows.Next() {
		event := &LossEvent{}
		if err := rows.Scan(
			&event.ID, &event.UserID, &event.TraderID, &event.EventType, &event.EventReason,
			&event.ConsecutiveLosses, &event.DailyLossPct, &event.WeeklyLossPct,
			&event.MaxDrawdownPct, &event.AccountEquity, &event.BreachTriggered,
			&event.RecoveryAttempt, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

// GetLossEventsByDateRange 获取日期范围内的亏损事件
func (db *DatabaseImpl) GetLossEventsByDateRange(traderID string, startTime, endTime time.Time) ([]*LossEvent, error) {
	query := `SELECT id, user_id, trader_id, event_type, event_reason, consecutive_losses,
		daily_loss_pct, weekly_loss_pct, max_drawdown_pct, account_equity,
		breach_triggered, recovery_attempt, created_at
	FROM loss_events
	WHERE trader_id = $1 AND created_at >= $2 AND created_at <= $3
	ORDER BY created_at DESC`

	if !db.usingNeon {
		query = strings.ReplaceAll(query, "$1", "?")
		query = strings.ReplaceAll(query, "$2", "?")
		query = strings.ReplaceAll(query, "$3", "?")
	}

	rows, err := db.currentDB.Query(query, traderID, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*LossEvent
	for rows.Next() {
		event := &LossEvent{}
		if err := rows.Scan(
			&event.ID, &event.UserID, &event.TraderID, &event.EventType, &event.EventReason,
			&event.ConsecutiveLosses, &event.DailyLossPct, &event.WeeklyLossPct,
			&event.MaxDrawdownPct, &event.AccountEquity, &event.BreachTriggered,
			&event.RecoveryAttempt, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

// GetConsecutiveLossStats 获取连续亏损的统计信息
func (db *DatabaseImpl) GetConsecutiveLossStats(traderID string) (map[string]interface{}, error) {
	query := `SELECT COUNT(*) as total, MAX(consecutive_losses) as max_losses,
		AVG(consecutive_losses) as avg_losses, SUM(CASE WHEN breach_triggered THEN 1 ELSE 0 END) as breach_count
	FROM loss_events
	WHERE trader_id = $1 AND event_type = 'consecutive_loss'`

	if !db.usingNeon {
		query = strings.ReplaceAll(query, "$1", "?")
	}

	row := db.currentDB.QueryRow(query, traderID)

	var total, maxLosses, breachCount int
	var avgLosses *float64

	if err := row.Scan(&total, &maxLosses, &avgLosses, &breachCount); err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_events":  total,
		"max_losses":    maxLosses,
		"avg_losses":    avgLosses,
		"breach_count":  breachCount,
	}

	return stats, nil
}

// GetBreachFrequency 获取断路器触发频率统计
func (db *DatabaseImpl) GetBreachFrequency(traderID string, days int) (map[string]int, error) {
	query := fmt.Sprintf(`SELECT event_type, COUNT(*) as count
	FROM loss_events
	WHERE trader_id = $1 AND breach_triggered = true AND created_at >= NOW() - INTERVAL '%d days'
	GROUP BY event_type`, days)

	if !db.usingNeon {
		// SQLite特殊处理
		query = fmt.Sprintf(`SELECT event_type, COUNT(*) as count
		FROM loss_events
		WHERE trader_id = ? AND breach_triggered = 1 AND created_at >= datetime('now', '-%d days')
		GROUP BY event_type`, days)
	}

	rows, err := db.currentDB.Query(query, traderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	frequency := make(map[string]int)
	for rows.Next() {
		var eventType string
		var count int
		if err := rows.Scan(&eventType, &count); err != nil {
			return nil, err
		}
		frequency[eventType] = count
	}

	return frequency, rows.Err()
}
