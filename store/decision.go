package store

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// DecisionStore decision log storage
type DecisionStore struct {
	db *gorm.DB
}

// DecisionRecordDB internal GORM model for decision_records table
type DecisionRecordDB struct {
	ID             int64     `gorm:"primaryKey;autoIncrement"`
	TraderID       string    `gorm:"column:trader_id;not null;index:idx_decision_records_trader_time"`
	CycleNumber    int       `gorm:"column:cycle_number;not null"`
	Timestamp      time.Time `gorm:"not null;index:idx_decision_records_trader_time,sort:desc;index:idx_decision_records_timestamp,sort:desc"`
	SystemPrompt   string    `gorm:"column:system_prompt;default:''"`
	InputPrompt    string    `gorm:"column:input_prompt;default:''"`
	CoTTrace       string    `gorm:"column:cot_trace;default:''"`
	DecisionJSON   string    `gorm:"column:decision_json;default:''"`
	RawResponse    string    `gorm:"column:raw_response;default:''"`
	CandidateCoins string    `gorm:"column:candidate_coins;default:''"`
	// 2026-10-10 per-candidate block reasons (JSON []CandidateVerdict).
	CandidateVerdicts   string    `gorm:"column:candidate_verdicts;type:text;default:''"`
	ExecutionLog        string    `gorm:"column:execution_log;default:''"`
	Decisions           string    `gorm:"column:decisions;default:'[]'"`
	Success             bool      `gorm:"default:false"`
	ErrorMessage        string    `gorm:"column:error_message;default:''"`
	AIRequestDurationMs int64     `gorm:"column:ai_request_duration_ms;default:0"`
	ConfigHash          string    `gorm:"column:config_hash;default:''"` // risk_control hash the cycle ran under (config-drift forensics)
	CreatedAt           time.Time `json:"created_at"`
}

func (DecisionRecordDB) TableName() string { return "decision_records" }

// DecisionRecord decision record (external API struct)
// CandidateVerdict is one candidate's outcome for a cycle (2026-10-10
// per-candidate block reasons). Status: "evaluated" | "blocked" | "filtered" |
// "held" (an open position, evaluated in the positions section).
type CandidateVerdict struct {
	Symbol      string   `json:"symbol"`
	Status      string   `json:"status"`
	LongFailed  []string `json:"long_failed,omitempty"`
	ShortFailed []string `json:"short_failed,omitempty"`
	Reason      string   `json:"reason,omitempty"`
}

type DecisionRecord struct {
	ID                  int64              `json:"id"`
	TraderID            string             `json:"trader_id"`
	CycleNumber         int                `json:"cycle_number"`
	Timestamp           time.Time          `json:"timestamp"`
	SystemPrompt        string             `json:"system_prompt"`
	InputPrompt         string             `json:"input_prompt"`
	CoTTrace            string             `json:"cot_trace"`
	DecisionJSON        string             `json:"decision_json"`
	RawResponse         string             `json:"raw_response"` // Raw AI response for debugging
	CandidateCoins      []string           `json:"candidate_coins"`
	CandidateVerdicts   []CandidateVerdict `json:"candidate_verdicts,omitempty"` // 2026-10-10 per-candidate block reasons
	ExecutionLog        []string           `json:"execution_log"`
	Success             bool               `json:"success"`
	ErrorMessage        string             `json:"error_message"`
	AIRequestDurationMs int64              `json:"ai_request_duration_ms"`
	ConfigHash          string             `json:"config_hash"`
	AccountState        AccountSnapshot    `json:"account_state"`
	Positions           []PositionSnapshot `json:"positions"`
	Decisions           []DecisionAction   `json:"decisions"`
}

// AccountSnapshot account state snapshot
type AccountSnapshot struct {
	TotalBalance          float64 `json:"total_balance"`
	AvailableBalance      float64 `json:"available_balance"`
	TotalUnrealizedProfit float64 `json:"total_unrealized_profit"`
	PositionCount         int     `json:"position_count"`
	MarginUsedPct         float64 `json:"margin_used_pct"`
	InitialBalance        float64 `json:"initial_balance"`
}

// PositionSnapshot position snapshot
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

// DecisionAction decision action
type DecisionAction struct {
	Action     string  `json:"action"`
	Symbol     string  `json:"symbol"`
	Quantity   float64 `json:"quantity"`
	Leverage   int     `json:"leverage"`
	Price      float64 `json:"price"`
	StopLoss   float64 `json:"stop_loss,omitempty"`   // Stop loss price
	TakeProfit float64 `json:"take_profit,omitempty"` // Take profit price
	Confidence int     `json:"confidence,omitempty"`  // AI confidence (0-100)
	Reasoning  string  `json:"reasoning,omitempty"`   // Brief reasoning
	OrderID    int64   `json:"order_id"`
	// EntryOrderID: exchange id (string) of the order an OPEN action placed —
	// limit or market. OrderID stays 0 for limit entries, so the quality
	// buckets join assessment→position on this field (review 2026-10-08 H).
	EntryOrderID string    `json:"entry_order_id,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
	Success      bool      `json:"success"`
	Error        string    `json:"error"`
	// EntryPath tags the entry context for path-attribution stats (user
	// 09-13: rally-window limit shorts vs bb_ride market shorts must be
	// separately measurable): e.g. "15m:down", "15m:rally", "bb_ride".
	EntryPath string `json:"entry_path,omitempty"`
}

// Statistics statistics information
type Statistics struct {
	TotalCycles         int `json:"total_cycles"`
	SuccessfulCycles    int `json:"successful_cycles"`
	FailedCycles        int `json:"failed_cycles"`
	TotalOpenPositions  int `json:"total_open_positions"`
	TotalClosePositions int `json:"total_close_positions"`
}

// NewDecisionStore creates a new DecisionStore
func NewDecisionStore(db *gorm.DB) *DecisionStore {
	return &DecisionStore{db: db}
}

// initTables initializes AI decision log tables
func (s *DecisionStore) initTables() error {
	// For PostgreSQL with existing table, skip AutoMigrate
	if s.db.Dialector.Name() == "postgres" {
		var tableExists int64
		if err := s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'decision_records'`).Scan(&tableExists).Error; err != nil {
			return err
		}
		if tableExists > 0 {
			return ensureColumns(s.db, &DecisionRecordDB{})
		}
	}
	return s.db.AutoMigrate(&DecisionRecordDB{})
}

// toRecord converts DB model to API struct
func (db *DecisionRecordDB) toRecord() *DecisionRecord {
	record := &DecisionRecord{
		ID:                  db.ID,
		TraderID:            db.TraderID,
		CycleNumber:         db.CycleNumber,
		Timestamp:           db.Timestamp,
		SystemPrompt:        db.SystemPrompt,
		InputPrompt:         db.InputPrompt,
		CoTTrace:            db.CoTTrace,
		DecisionJSON:        db.DecisionJSON,
		RawResponse:         db.RawResponse,
		Success:             db.Success,
		ErrorMessage:        db.ErrorMessage,
		AIRequestDurationMs: db.AIRequestDurationMs,
		ConfigHash:          db.ConfigHash,
	}
	json.Unmarshal([]byte(db.CandidateCoins), &record.CandidateCoins)
	if db.CandidateVerdicts != "" {
		json.Unmarshal([]byte(db.CandidateVerdicts), &record.CandidateVerdicts)
	}
	json.Unmarshal([]byte(db.ExecutionLog), &record.ExecutionLog)
	json.Unmarshal([]byte(db.Decisions), &record.Decisions)
	return record
}

// LogDecision logs decision
func (s *DecisionStore) LogDecision(record *DecisionRecord) error {
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	} else {
		record.Timestamp = record.Timestamp.UTC()
	}

	// Serialize arrays to JSON
	candidateCoinsJSON, _ := json.Marshal(record.CandidateCoins)
	candidateVerdictsJSON := ""
	if len(record.CandidateVerdicts) > 0 {
		b, _ := json.Marshal(record.CandidateVerdicts)
		candidateVerdictsJSON = string(b)
	}
	executionLogJSON, _ := json.Marshal(record.ExecutionLog)
	decisionsJSON, _ := json.Marshal(record.Decisions)

	dbRecord := &DecisionRecordDB{
		TraderID:            record.TraderID,
		CycleNumber:         record.CycleNumber,
		Timestamp:           record.Timestamp,
		SystemPrompt:        record.SystemPrompt,
		InputPrompt:         record.InputPrompt,
		CoTTrace:            record.CoTTrace,
		DecisionJSON:        record.DecisionJSON,
		RawResponse:         record.RawResponse,
		CandidateCoins:      string(candidateCoinsJSON),
		CandidateVerdicts:   candidateVerdictsJSON,
		ExecutionLog:        string(executionLogJSON),
		Decisions:           string(decisionsJSON),
		Success:             record.Success,
		ErrorMessage:        record.ErrorMessage,
		AIRequestDurationMs: record.AIRequestDurationMs,
		ConfigHash:          record.ConfigHash,
	}

	if err := s.db.Create(dbRecord).Error; err != nil {
		return fmt.Errorf("failed to insert decision record: %w", err)
	}
	record.ID = dbRecord.ID
	return nil
}

// GetLatestRecords gets the latest N records for specified trader (sorted by time in ascending order: old to new)
func (s *DecisionStore) GetLatestRecords(traderID string, n int) ([]*DecisionRecord, error) {
	var dbRecords []*DecisionRecordDB
	err := s.db.Where("trader_id = ?", traderID).
		Order("timestamp DESC").
		Limit(n).
		Find(&dbRecords).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query decision records: %w", err)
	}

	records := make([]*DecisionRecord, len(dbRecords))
	for i, db := range dbRecords {
		records[i] = db.toRecord()
	}

	// Reverse array to sort time from old to new
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// QualityRecord is the lightweight projection for the system-quality page —
// decision_records rows carry the full prompts, so aggregation must never
// load whole rows.
type QualityRecord struct {
	Success             bool
	ErrorMessage        string
	AIRequestDurationMs int64
	Timestamp           time.Time
}

// GetQualityRecordsSince returns the lightweight projection of all records
// for a trader since the given time, oldest first.
func (s *DecisionStore) GetQualityRecordsSince(traderID string, since time.Time) ([]QualityRecord, error) {
	var rows []QualityRecord
	if err := s.qualityQuery(traderID, since).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query quality records: %w", err)
	}
	return rows, nil
}

func (s *DecisionStore) qualityQuery(traderID string, since time.Time) *gorm.DB {
	return s.db.Table("decision_records").
		Select("success, error_message, ai_request_duration_ms, timestamp").
		Where("trader_id = ? AND timestamp >= ?", traderID, since).
		Order("timestamp ASC")
}

// GetAllLatestRecords gets the latest N records for all traders
func (s *DecisionStore) GetAllLatestRecords(n int) ([]*DecisionRecord, error) {
	var dbRecords []*DecisionRecordDB
	err := s.db.Order("timestamp DESC").Limit(n).Find(&dbRecords).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query decision records: %w", err)
	}

	records := make([]*DecisionRecord, len(dbRecords))
	for i, db := range dbRecords {
		records[i] = db.toRecord()
	}

	// Reverse array
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// GetRecordsByDate gets all records for a specified trader on a specified date
func (s *DecisionStore) GetRecordsByDate(traderID string, date time.Time) ([]*DecisionRecord, error) {
	dateStr := date.Format("2006-01-02")

	var dbRecords []*DecisionRecordDB
	err := s.db.Where("trader_id = ? AND DATE(timestamp) = ?", traderID, dateStr).
		Order("timestamp ASC").
		Find(&dbRecords).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query decision records: %w", err)
	}

	records := make([]*DecisionRecord, len(dbRecords))
	for i, db := range dbRecords {
		records[i] = db.toRecord()
	}

	return records, nil
}

// CleanOldRecords cleans old records from N days ago
func (s *DecisionStore) CleanOldRecords(traderID string, days int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -days)

	result := s.db.Where("trader_id = ? AND timestamp < ?", traderID, cutoffTime).
		Delete(&DecisionRecordDB{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to clean old records: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// GetStatistics gets statistics information for specified trader
func (s *DecisionStore) GetStatistics(traderID string) (*Statistics, error) {
	stats := &Statistics{}

	var totalCount, successCount int64
	s.db.Model(&DecisionRecordDB{}).Where("trader_id = ?", traderID).Count(&totalCount)
	s.db.Model(&DecisionRecordDB{}).Where("trader_id = ? AND success = ?", traderID, true).Count(&successCount)

	stats.TotalCycles = int(totalCount)
	stats.SuccessfulCycles = int(successCount)
	stats.FailedCycles = stats.TotalCycles - stats.SuccessfulCycles

	// Count from trader_positions table using raw query for cross-table
	s.db.Raw("SELECT COUNT(*) FROM trader_positions WHERE trader_id = ?", traderID).Scan(&stats.TotalOpenPositions)
	s.db.Raw("SELECT COUNT(*) FROM trader_positions WHERE trader_id = ? AND status = 'CLOSED'", traderID).Scan(&stats.TotalClosePositions)

	return stats, nil
}

// GetAllStatistics gets statistics information for all traders
func (s *DecisionStore) GetAllStatistics() (*Statistics, error) {
	stats := &Statistics{}

	var totalCount, successCount int64
	s.db.Model(&DecisionRecordDB{}).Count(&totalCount)
	s.db.Model(&DecisionRecordDB{}).Where("success = ?", true).Count(&successCount)

	stats.TotalCycles = int(totalCount)
	stats.SuccessfulCycles = int(successCount)
	stats.FailedCycles = stats.TotalCycles - stats.SuccessfulCycles

	// Count from trader_positions table
	s.db.Raw("SELECT COUNT(*) FROM trader_positions").Scan(&stats.TotalOpenPositions)
	s.db.Raw("SELECT COUNT(*) FROM trader_positions WHERE status = 'CLOSED'").Scan(&stats.TotalClosePositions)

	return stats, nil
}

// GetLastCycleNumber gets the last cycle number for specified trader
func (s *DecisionStore) GetLastCycleNumber(traderID string) (int, error) {
	var cycleNumber *int
	err := s.db.Model(&DecisionRecordDB{}).
		Where("trader_id = ?", traderID).
		Select("MAX(cycle_number)").
		Scan(&cycleNumber).Error
	if err != nil {
		return 0, err
	}
	if cycleNumber == nil {
		return 0, nil
	}
	return *cycleNumber, nil
}

// CandidatePoolRecord contains only the columns needed by shadow overlap research.
type CandidatePoolRecord struct {
	Timestamp      time.Time
	CandidateCoins string
}

// GetCandidatePoolsBetween loads every trader's candidate arrays in one query.
func (s *DecisionStore) GetCandidatePoolsBetween(start, end time.Time) ([]CandidatePoolRecord, error) {
	var rows []CandidatePoolRecord
	err := s.db.Model(&DecisionRecordDB{}).Select("timestamp, candidate_coins").Where("timestamp >= ? AND timestamp <= ?", start.UTC(), end.UTC()).Order("timestamp ASC").Find(&rows).Error
	return rows, err
}
