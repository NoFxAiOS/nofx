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
	ID                  int64     `gorm:"primaryKey;autoIncrement"`
	TraderID            string    `gorm:"column:trader_id;not null;index:idx_decision_records_trader_time"`
	CycleNumber         int       `gorm:"column:cycle_number;not null"`
	Timestamp           time.Time `gorm:"not null;index:idx_decision_records_trader_time,sort:desc;index:idx_decision_records_timestamp,sort:desc"`
	SystemPrompt        string    `gorm:"column:system_prompt;default:''"`
	InputPrompt         string    `gorm:"column:input_prompt;default:''"`
	CoTTrace            string    `gorm:"column:cot_trace;default:''"`
	DecisionJSON        string    `gorm:"column:decision_json;default:''"`
	RawResponse         string    `gorm:"column:raw_response;default:''"`
	CandidateCoins      string    `gorm:"column:candidate_coins;default:''"`
	ExecutionLog        string    `gorm:"column:execution_log;default:''"`
	Decisions           string    `gorm:"column:decisions;default:'[]'"`
	ProtectionSnapshot  string    `gorm:"column:protection_snapshot;default:''"`
	ReviewContext       string    `gorm:"column:review_context;default:''"`
	AllowAIClose        bool      `gorm:"column:allow_ai_close;default:true"`
	AIDecisionMode      string    `gorm:"column:ai_decision_mode;default:'balanced'"`
	Success             bool      `gorm:"default:false"`
	ErrorMessage        string    `gorm:"column:error_message;default:''"`
	AIRequestDurationMs int64     `gorm:"column:ai_request_duration_ms;default:0"`
	CreatedAt           time.Time `json:"created_at"`
}

func (DecisionRecordDB) TableName() string { return "decision_records" }

// DecisionRecord decision record (external API struct)
type DecisionRecord struct {
	ID                  int64                  `json:"id"`
	TraderID            string                 `json:"trader_id"`
	CycleNumber         int                    `json:"cycle_number"`
	Timestamp           time.Time              `json:"timestamp"`
	SystemPrompt        string                 `json:"system_prompt"`
	InputPrompt         string                 `json:"input_prompt"`
	CoTTrace            string                 `json:"cot_trace"`
	DecisionJSON        string                 `json:"decision_json"`
	RawResponse         string                 `json:"raw_response"` // Raw AI response for debugging
	CandidateCoins      []string               `json:"candidate_coins"`
	ExecutionLog        []string               `json:"execution_log"`
	Success             bool                   `json:"success"`
	ErrorMessage        string                 `json:"error_message"`
	AIRequestDurationMs int64                  `json:"ai_request_duration_ms"`
	AccountState        AccountSnapshot        `json:"account_state"`
	Positions           []PositionSnapshot     `json:"positions"`
	Decisions           []DecisionAction       `json:"decisions"`
	ProtectionSnapshot  *ProtectionSnapshot    `json:"protection_snapshot,omitempty"`
	ReviewContext       map[string]interface{} `json:"review_context,omitempty"`
	AllowAIClose        bool                   `json:"allow_ai_close"`
	AIDecisionMode      string                 `json:"ai_decision_mode"`
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
	Action        string                       `json:"action"`
	Symbol        string                       `json:"symbol"`
	Quantity      float64                      `json:"quantity"`
	Leverage      int                          `json:"leverage"`
	Price         float64                      `json:"price"`
	StopLoss      float64                      `json:"stop_loss,omitempty"`   // Stop loss price
	TakeProfit    float64                      `json:"take_profit,omitempty"` // Take profit price
	Confidence    int                          `json:"confidence,omitempty"`  // AI confidence (0-100)
	Reasoning     string                       `json:"reasoning,omitempty"`   // Brief reasoning
	ReviewContext *DecisionActionReviewContext `json:"review_context,omitempty"`
	OrderID       int64                        `json:"order_id"`
	Timestamp     time.Time                    `json:"timestamp"`
	Success       bool                         `json:"success"`
	Error         string                       `json:"error"`
}

// DecisionActionReviewContext captures compact, structured rationale for a single action.
type DecisionActionReviewContext struct {
	PrimaryTimeframe     string                              `json:"primary_timeframe,omitempty"`
	MinRiskReward        float64                             `json:"min_risk_reward,omitempty"`
	RiskReward           *DecisionActionRiskRewardSummary    `json:"risk_reward,omitempty"`
	KeyLevels            *DecisionActionKeyLevels            `json:"key_levels,omitempty"`
	Anchors              []DecisionActionReasonAnchor        `json:"anchors,omitempty"`
	Protection           *DecisionActionProtectionAlignment  `json:"protection,omitempty"`
	ExecutionConstraints *DecisionActionExecutionConstraints `json:"execution_constraints,omitempty"`
}

// DecisionActionExecutionConstraints stores compact execution-relevant venue constraints.
type DecisionActionExecutionConstraints struct {
	TickSize             float64 `json:"tick_size,omitempty"`
	PricePrecision       int     `json:"price_precision,omitempty"`
	QtyStepSize          float64 `json:"qty_step_size,omitempty"`
	QtyPrecision         int     `json:"qty_precision,omitempty"`
	MinQty               float64 `json:"min_qty,omitempty"`
	MinNotional          float64 `json:"min_notional,omitempty"`
	ContractValue        float64 `json:"contract_value,omitempty"`
	MarkPrice            float64 `json:"mark_price,omitempty"`
	LastPrice            float64 `json:"last_price,omitempty"`
	BestBid              float64 `json:"best_bid,omitempty"`
	BestAsk              float64 `json:"best_ask,omitempty"`
	SpreadBps            float64 `json:"spread_bps,omitempty"`
	TakerFeeRate         float64 `json:"taker_fee_rate,omitempty"`
	MakerFeeRate         float64 `json:"maker_fee_rate,omitempty"`
	EstimatedSlippageBps float64 `json:"estimated_slippage_bps,omitempty"`
}

// DecisionActionRiskRewardSummary stores gross/net RR and pass/fail metadata.
type DecisionActionRiskRewardSummary struct {
	Entry            float64 `json:"entry,omitempty"`
	Invalidation     float64 `json:"invalidation,omitempty"`
	FirstTarget      float64 `json:"first_target,omitempty"`
	GrossEstimatedRR float64 `json:"gross_estimated_rr,omitempty"`
	NetEstimatedRR   float64 `json:"net_estimated_rr,omitempty"`
	Passed           bool    `json:"passed"`
}

// DecisionActionKeyLevels stores compact key support/resistance levels for audit UI.
type DecisionActionKeyLevels struct {
	Support    []float64 `json:"support,omitempty"`
	Resistance []float64 `json:"resistance,omitempty"`
}

// DecisionActionReasonAnchor stores a compact rationale anchor.
type DecisionActionReasonAnchor struct {
	Type      string  `json:"type,omitempty"`
	Timeframe string  `json:"timeframe,omitempty"`
	Price     float64 `json:"price,omitempty"`
	Reason    string  `json:"reason,omitempty"`
}

// DecisionActionProtectionAlignment stores compact protection alignment audit notes.
type DecisionActionProtectionAlignment struct {
	StopBeyondInvalidation bool     `json:"stop_beyond_invalidation,omitempty"`
	TargetAligned          bool     `json:"target_aligned,omitempty"`
	BreakEvenBeforeTarget  bool     `json:"break_even_before_target,omitempty"`
	FallbackWithinEnvelope bool     `json:"fallback_within_envelope,omitempty"`
	PolicyStatus           string   `json:"policy_status,omitempty"`
	PolicyOverride         bool     `json:"policy_override,omitempty"`
	PolicyRejected         bool     `json:"policy_rejected,omitempty"`
	PolicyReasons          []string `json:"policy_reasons,omitempty"`
	Notes                  []string `json:"notes,omitempty"`
}

// ProtectionSnapshot captures the active protection configuration at decision time
type ProtectionSnapshot struct {
	FullTPSL   *ProtectionSnapshotFullTPSL  `json:"full_tp_sl,omitempty"`
	LadderTPSL *ProtectionSnapshotLadder    `json:"ladder_tp_sl,omitempty"`
	Drawdown   []ProtectionSnapshotDrawdown `json:"drawdown,omitempty"`
	BreakEven  *ProtectionSnapshotBreakEven `json:"break_even,omitempty"`
}

type ProtectionSnapshotValueSource struct {
	Mode  string  `json:"mode,omitempty"`
	Value float64 `json:"value,omitempty"`
}

// ProtectionSnapshotFullTPSL full take-profit / stop-loss snapshot
type ProtectionSnapshotFullTPSL struct {
	Enabled         bool                          `json:"enabled"`
	Mode            string                        `json:"mode"`
	TakeProfit      ProtectionSnapshotValueSource `json:"take_profit,omitempty"`
	StopLoss        ProtectionSnapshotValueSource `json:"stop_loss,omitempty"`
	FallbackMaxLoss ProtectionSnapshotValueSource `json:"fallback_max_loss,omitempty"`
}

// ProtectionSnapshotLadder ladder TP/SL snapshot with concrete rules
type ProtectionSnapshotLadder struct {
	Enabled           bool                           `json:"enabled"`
	Mode              string                         `json:"mode"`
	TakeProfitEnabled bool                           `json:"take_profit_enabled"`
	StopLossEnabled   bool                           `json:"stop_loss_enabled"`
	TakeProfitPrice   ProtectionSnapshotValueSource  `json:"take_profit_price,omitempty"`
	TakeProfitSize    ProtectionSnapshotValueSource  `json:"take_profit_size,omitempty"`
	StopLossPrice     ProtectionSnapshotValueSource  `json:"stop_loss_price,omitempty"`
	StopLossSize      ProtectionSnapshotValueSource  `json:"stop_loss_size,omitempty"`
	FallbackMaxLoss   ProtectionSnapshotValueSource  `json:"fallback_max_loss,omitempty"`
	Rules             []ProtectionSnapshotLadderRule `json:"rules,omitempty"`
}

// ProtectionSnapshotLadderRule a single ladder rule with concrete values
type ProtectionSnapshotLadderRule struct {
	TakeProfitPct           float64 `json:"take_profit_pct,omitempty"`
	TakeProfitCloseRatioPct float64 `json:"take_profit_close_ratio_pct,omitempty"`
	StopLossPct             float64 `json:"stop_loss_pct,omitempty"`
	StopLossCloseRatioPct   float64 `json:"stop_loss_close_ratio_pct,omitempty"`
}

// ProtectionSnapshotDrawdown drawdown take-profit rule snapshot
type ProtectionSnapshotDrawdown struct {
	Mode           string  `json:"mode,omitempty"`
	Source         string  `json:"source,omitempty"`
	MinProfitPct   float64 `json:"min_profit_pct"`
	MaxDrawdownPct float64 `json:"max_drawdown_pct"`
	CloseRatioPct  float64 `json:"close_ratio_pct"`
	PollIntervalS  int     `json:"poll_interval_s"`
}

// ProtectionSnapshotBreakEven break-even stop snapshot
type ProtectionSnapshotBreakEven struct {
	Enabled      bool    `json:"enabled"`
	Source       string  `json:"source,omitempty"`
	TriggerMode  string  `json:"trigger_mode"`
	TriggerValue float64 `json:"trigger_value"`
	OffsetPct    float64 `json:"offset_pct"`
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
	// For PostgreSQL with existing table, add missing columns instead of full AutoMigrate
	if s.db.Dialector.Name() == "postgres" {
		var tableExists int64
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'decision_records'`).Scan(&tableExists)
		if tableExists > 0 {
			// Add protection_snapshot column if missing (safe: ADD COLUMN IF NOT EXISTS)
			s.db.Exec(`ALTER TABLE decision_records ADD COLUMN IF NOT EXISTS protection_snapshot TEXT DEFAULT ''`)
			s.db.Exec(`ALTER TABLE decision_records ADD COLUMN IF NOT EXISTS review_context TEXT DEFAULT ''`)
			s.db.Exec(`ALTER TABLE decision_records ADD COLUMN IF NOT EXISTS allow_ai_close BOOLEAN DEFAULT true`)
			s.db.Exec(`ALTER TABLE decision_records ADD COLUMN IF NOT EXISTS ai_decision_mode TEXT DEFAULT 'balanced'`)
			return nil
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
		AllowAIClose:        db.AllowAIClose,
		AIDecisionMode:      db.AIDecisionMode,
	}
	json.Unmarshal([]byte(db.CandidateCoins), &record.CandidateCoins)
	json.Unmarshal([]byte(db.ExecutionLog), &record.ExecutionLog)
	json.Unmarshal([]byte(db.Decisions), &record.Decisions)
	if db.ProtectionSnapshot != "" {
		var ps ProtectionSnapshot
		if err := json.Unmarshal([]byte(db.ProtectionSnapshot), &ps); err == nil {
			record.ProtectionSnapshot = &ps
		}
	}
	if db.ReviewContext != "" {
		var rc map[string]interface{}
		if err := json.Unmarshal([]byte(db.ReviewContext), &rc); err == nil {
			record.ReviewContext = rc
		}
	}
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
	executionLogJSON, _ := json.Marshal(record.ExecutionLog)
	decisionsJSON, _ := json.Marshal(record.Decisions)
	protectionSnapshotJSON := ""
	if record.ProtectionSnapshot != nil {
		if ps, err := json.Marshal(record.ProtectionSnapshot); err == nil {
			protectionSnapshotJSON = string(ps)
		}
	}
	reviewContextJSON := ""
	if record.ReviewContext != nil {
		if rc, err := json.Marshal(record.ReviewContext); err == nil {
			reviewContextJSON = string(rc)
		}
	}

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
		ExecutionLog:        string(executionLogJSON),
		Decisions:           string(decisionsJSON),
		ProtectionSnapshot:  protectionSnapshotJSON,
		ReviewContext:       reviewContextJSON,
		AllowAIClose:        record.AllowAIClose,
		AIDecisionMode:      record.AIDecisionMode,
		Success:             record.Success,
		ErrorMessage:        record.ErrorMessage,
		AIRequestDurationMs: record.AIRequestDurationMs,
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

func (s *DecisionStore) GetRecordByCycle(traderID string, cycleNumber int) (*DecisionRecord, error) {
	if traderID == "" || cycleNumber <= 0 {
		return nil, nil
	}
	var dbRecord DecisionRecordDB
	err := s.db.Where("trader_id = ? AND cycle_number = ?", traderID, cycleNumber).First(&dbRecord).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query decision record by cycle: %w", err)
	}
	return dbRecord.toRecord(), nil
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
