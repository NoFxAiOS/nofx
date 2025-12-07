package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// DecisionStore 决策日志存储
type DecisionStore struct {
	db *sql.DB
}

// DecisionRecord 决策记录
type DecisionRecord struct {
	ID                  int64              `json:"id"`
	TraderID            string             `json:"trader_id"`
	CycleNumber         int                `json:"cycle_number"`
	Timestamp           time.Time          `json:"timestamp"`
	SystemPrompt        string             `json:"system_prompt"`
	InputPrompt         string             `json:"input_prompt"`
	CoTTrace            string             `json:"cot_trace"`
	DecisionJSON        string             `json:"decision_json"`
	CandidateCoins      []string           `json:"candidate_coins"`
	ExecutionLog        []string           `json:"execution_log"`
	Success             bool               `json:"success"`
	ErrorMessage        string             `json:"error_message"`
	AIRequestDurationMs int64              `json:"ai_request_duration_ms"`
	AccountState        AccountSnapshot    `json:"account_state"`
	Positions           []PositionSnapshot `json:"positions"`
	Decisions           []DecisionAction   `json:"decisions"`
}

// AccountSnapshot 账户状态快照
type AccountSnapshot struct {
	TotalBalance          float64 `json:"total_balance"`
	AvailableBalance      float64 `json:"available_balance"`
	TotalUnrealizedProfit float64 `json:"total_unrealized_profit"`
	PositionCount         int     `json:"position_count"`
	MarginUsedPct         float64 `json:"margin_used_pct"`
	InitialBalance        float64 `json:"initial_balance"`
}

// PositionSnapshot 持仓快照
type PositionSnapshot struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"`
	PositionAmt      float64 `json:"position_amt"`
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	UnrealizedProfit float64 `json:"unrealized_profit"`
	Leverage         float64 `json:"leverage"`
	LiquidationPrice float64 `json:"liquidation_price"`
}

// DecisionAction 决策动作
type DecisionAction struct {
	Action    string    `json:"action"`
	Symbol    string    `json:"symbol"`
	Quantity  float64   `json:"quantity"`
	Leverage  int       `json:"leverage"`
	Price     float64   `json:"price"`
	OrderID   int64     `json:"order_id"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	Error     string    `json:"error"`
}

// Statistics 统计信息
type Statistics struct {
	TotalCycles         int `json:"total_cycles"`
	SuccessfulCycles    int `json:"successful_cycles"`
	FailedCycles        int `json:"failed_cycles"`
	TotalOpenPositions  int `json:"total_open_positions"`
	TotalClosePositions int `json:"total_close_positions"`
}

// initTables 初始化 AI 决策日志表
// 注意：账户净值曲线数据已迁移到 trader_equity_snapshots 表（由 EquityStore 管理）
func (s *DecisionStore) initTables() error {
	queries := []string{
		// AI 决策日志表（记录 AI 的输入输出、思维链等）
		`CREATE TABLE IF NOT EXISTS decision_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trader_id TEXT NOT NULL,
			cycle_number INTEGER NOT NULL,
			timestamp DATETIME NOT NULL,
			system_prompt TEXT DEFAULT '',
			input_prompt TEXT DEFAULT '',
			cot_trace TEXT DEFAULT '',
			decision_json TEXT DEFAULT '',
			candidate_coins TEXT DEFAULT '',
			execution_log TEXT DEFAULT '',
			success BOOLEAN DEFAULT 0,
			error_message TEXT DEFAULT '',
			ai_request_duration_ms INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// 索引
		`CREATE INDEX IF NOT EXISTS idx_decision_records_trader_time ON decision_records(trader_id, timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_decision_records_timestamp ON decision_records(timestamp DESC)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("执行SQL失败: %w", err)
		}
	}

	return nil
}

// LogDecision 记录决策（仅保存 AI 决策日志，净值曲线已迁移到 equity 表）
func (s *DecisionStore) LogDecision(record *DecisionRecord) error {
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	} else {
		record.Timestamp = record.Timestamp.UTC()
	}

	// 序列化候选币种和执行日志为 JSON
	candidateCoinsJSON, _ := json.Marshal(record.CandidateCoins)
	executionLogJSON, _ := json.Marshal(record.ExecutionLog)

	// 插入决策记录主表（仅保存 AI 决策相关内容）
	result, err := s.db.Exec(`
		INSERT INTO decision_records (
			trader_id, cycle_number, timestamp, system_prompt, input_prompt,
			cot_trace, decision_json, candidate_coins, execution_log,
			success, error_message, ai_request_duration_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		record.TraderID, record.CycleNumber, record.Timestamp.Format(time.RFC3339),
		record.SystemPrompt, record.InputPrompt, record.CoTTrace, record.DecisionJSON,
		string(candidateCoinsJSON), string(executionLogJSON),
		record.Success, record.ErrorMessage, record.AIRequestDurationMs,
	)
	if err != nil {
		return fmt.Errorf("插入决策记录失败: %w", err)
	}

	decisionID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("获取决策ID失败: %w", err)
	}
	record.ID = decisionID

	return nil
}

// GetLatestRecords 获取指定交易员最近N条记录（按时间正序：从旧到新）
func (s *DecisionStore) GetLatestRecords(traderID string, n int) ([]*DecisionRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, trader_id, cycle_number, timestamp, system_prompt, input_prompt,
			   cot_trace, decision_json, candidate_coins, execution_log,
			   success, error_message, ai_request_duration_ms
		FROM decision_records
		WHERE trader_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, traderID, n)
	if err != nil {
		return nil, fmt.Errorf("查询决策记录失败: %w", err)
	}
	defer rows.Close()

	var records []*DecisionRecord
	for rows.Next() {
		record, err := s.scanDecisionRecord(rows)
		if err != nil {
			continue
		}
		records = append(records, record)
	}

	// 填充关联数据
	for _, record := range records {
		s.fillRecordDetails(record)
	}

	// 反转数组，让时间从旧到新排列
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// GetAllLatestRecords 获取所有交易员最近N条记录
func (s *DecisionStore) GetAllLatestRecords(n int) ([]*DecisionRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, trader_id, cycle_number, timestamp, system_prompt, input_prompt,
			   cot_trace, decision_json, candidate_coins, execution_log,
			   success, error_message, ai_request_duration_ms
		FROM decision_records
		ORDER BY timestamp DESC
		LIMIT ?
	`, n)
	if err != nil {
		return nil, fmt.Errorf("查询决策记录失败: %w", err)
	}
	defer rows.Close()

	var records []*DecisionRecord
	for rows.Next() {
		record, err := s.scanDecisionRecord(rows)
		if err != nil {
			continue
		}
		records = append(records, record)
	}

	// 反转数组
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// GetRecordsByDate 获取指定交易员指定日期的所有记录
func (s *DecisionStore) GetRecordsByDate(traderID string, date time.Time) ([]*DecisionRecord, error) {
	dateStr := date.Format("2006-01-02")

	rows, err := s.db.Query(`
		SELECT id, trader_id, cycle_number, timestamp, system_prompt, input_prompt,
			   cot_trace, decision_json, candidate_coins, execution_log,
			   success, error_message, ai_request_duration_ms
		FROM decision_records
		WHERE trader_id = ? AND DATE(timestamp) = ?
		ORDER BY timestamp ASC
	`, traderID, dateStr)
	if err != nil {
		return nil, fmt.Errorf("查询决策记录失败: %w", err)
	}
	defer rows.Close()

	var records []*DecisionRecord
	for rows.Next() {
		record, err := s.scanDecisionRecord(rows)
		if err != nil {
			continue
		}
		records = append(records, record)
	}

	return records, nil
}

// CleanOldRecords 清理N天前的旧记录
func (s *DecisionStore) CleanOldRecords(traderID string, days int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)

	result, err := s.db.Exec(`
		DELETE FROM decision_records
		WHERE trader_id = ? AND timestamp < ?
	`, traderID, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("清理旧记录失败: %w", err)
	}

	return result.RowsAffected()
}

// GetStatistics 获取指定交易员的统计信息
func (s *DecisionStore) GetStatistics(traderID string) (*Statistics, error) {
	stats := &Statistics{}

	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM decision_records WHERE trader_id = ?
	`, traderID).Scan(&stats.TotalCycles)
	if err != nil {
		return nil, fmt.Errorf("查询总周期数失败: %w", err)
	}

	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM decision_records WHERE trader_id = ? AND success = 1
	`, traderID).Scan(&stats.SuccessfulCycles)
	if err != nil {
		return nil, fmt.Errorf("查询成功周期数失败: %w", err)
	}
	stats.FailedCycles = stats.TotalCycles - stats.SuccessfulCycles

	// 从 trader_orders 表统计开仓次数
	s.db.QueryRow(`
		SELECT COUNT(*) FROM trader_orders
		WHERE trader_id = ? AND status = 'FILLED' AND action IN ('open_long', 'open_short')
	`, traderID).Scan(&stats.TotalOpenPositions)

	// 从 trader_orders 表统计平仓次数
	s.db.QueryRow(`
		SELECT COUNT(*) FROM trader_orders
		WHERE trader_id = ? AND status = 'FILLED' AND action IN ('close_long', 'close_short', 'auto_close_long', 'auto_close_short')
	`, traderID).Scan(&stats.TotalClosePositions)

	return stats, nil
}

// GetAllStatistics 获取所有交易员的统计信息
func (s *DecisionStore) GetAllStatistics() (*Statistics, error) {
	stats := &Statistics{}

	s.db.QueryRow(`SELECT COUNT(*) FROM decision_records`).Scan(&stats.TotalCycles)
	s.db.QueryRow(`SELECT COUNT(*) FROM decision_records WHERE success = 1`).Scan(&stats.SuccessfulCycles)
	stats.FailedCycles = stats.TotalCycles - stats.SuccessfulCycles

	// 从 trader_orders 表统计
	s.db.QueryRow(`
		SELECT COUNT(*) FROM trader_orders
		WHERE status = 'FILLED' AND action IN ('open_long', 'open_short')
	`).Scan(&stats.TotalOpenPositions)

	s.db.QueryRow(`
		SELECT COUNT(*) FROM trader_orders
		WHERE status = 'FILLED' AND action IN ('close_long', 'close_short', 'auto_close_long', 'auto_close_short')
	`).Scan(&stats.TotalClosePositions)

	return stats, nil
}

// GetLastCycleNumber 获取指定交易员的最后周期编号
func (s *DecisionStore) GetLastCycleNumber(traderID string) (int, error) {
	var cycleNumber int
	err := s.db.QueryRow(`
		SELECT COALESCE(MAX(cycle_number), 0) FROM decision_records WHERE trader_id = ?
	`, traderID).Scan(&cycleNumber)
	if err != nil {
		return 0, err
	}
	return cycleNumber, nil
}

// scanDecisionRecord 从行中扫描决策记录
func (s *DecisionStore) scanDecisionRecord(rows *sql.Rows) (*DecisionRecord, error) {
	var record DecisionRecord
	var timestampStr string
	var candidateCoinsJSON, executionLogJSON string

	err := rows.Scan(
		&record.ID, &record.TraderID, &record.CycleNumber, &timestampStr,
		&record.SystemPrompt, &record.InputPrompt, &record.CoTTrace,
		&record.DecisionJSON, &candidateCoinsJSON, &executionLogJSON,
		&record.Success, &record.ErrorMessage, &record.AIRequestDurationMs,
	)
	if err != nil {
		return nil, err
	}

	record.Timestamp, _ = time.Parse(time.RFC3339, timestampStr)
	json.Unmarshal([]byte(candidateCoinsJSON), &record.CandidateCoins)
	json.Unmarshal([]byte(executionLogJSON), &record.ExecutionLog)

	return &record, nil
}

// fillRecordDetails 填充决策记录的关联数据（旧的关联表已删除，此函数保留用于兼容性）
// 注意：账户快照、持仓快照、决策动作等数据已不再存储在 decision 相关表中
// - 净值数据请使用 EquityStore.GetLatest()
// - 订单数据请使用 OrderStore
func (s *DecisionStore) fillRecordDetails(record *DecisionRecord) {
	// 旧的关联表已删除，不再需要填充
	// AccountState, Positions, Decisions 字段将保持为零值
}
