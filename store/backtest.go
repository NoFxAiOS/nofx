package store

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// BacktestStore backtest data storage
type BacktestStore struct {
	db *gorm.DB
}

// NewBacktestStore creates a new backtest store
func NewBacktestStore(db *gorm.DB) *BacktestStore {
	return &BacktestStore{db: db}
}

// isPostgres checks if the database is PostgreSQL
func (s *BacktestStore) isPostgres() bool {
	return s.db.Dialector.Name() == "postgres"
}

// RunState backtest state
type RunState string

const (
	RunStateCreated   RunState = "created"
	RunStateRunning   RunState = "running"
	RunStatePaused    RunState = "paused"
	RunStateCompleted RunState = "completed"
	RunStateFailed    RunState = "failed"
)

// RunMetadata backtest metadata
type RunMetadata struct {
	RunID     string     `json:"run_id"`
	UserID    string     `json:"user_id"`
	Version   int        `json:"version"`
	State     RunState   `json:"state"`
	Label     string     `json:"label"`
	LastError string     `json:"last_error"`
	Summary   RunSummary `json:"summary"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// RunSummary backtest summary
type RunSummary struct {
	SymbolCount     int     `json:"symbol_count"`
	DecisionTF      string  `json:"decision_tf"`
	ProcessedBars   int     `json:"processed_bars"`
	ProgressPct     float64 `json:"progress_pct"`
	EquityLast      float64 `json:"equity_last"`
	MaxDrawdownPct  float64 `json:"max_drawdown_pct"`
	Liquidated      bool    `json:"liquidated"`
	LiquidationNote string  `json:"liquidation_note"`
}

// EquityPoint equity point
type EquityPoint struct {
	Timestamp   int64   `json:"timestamp"`
	Equity      float64 `json:"equity"`
	Available   float64 `json:"available"`
	PnL         float64 `json:"pnl"`
	PnLPct      float64 `json:"pnl_pct"`
	DrawdownPct float64 `json:"drawdown_pct"`
	Cycle       int     `json:"cycle"`
}

// TradeEvent trade event
type TradeEvent struct {
	Timestamp       int64   `json:"timestamp"`
	Symbol          string  `json:"symbol"`
	Action          string  `json:"action"`
	Side            string  `json:"side"`
	Quantity        float64 `json:"quantity"`
	Price           float64 `json:"price"`
	Fee             float64 `json:"fee"`
	Slippage        float64 `json:"slippage"`
	OrderValue      float64 `json:"order_value"`
	RealizedPnL     float64 `json:"realized_pnl"`
	Leverage        int     `json:"leverage"`
	Cycle           int     `json:"cycle"`
	PositionAfter   float64 `json:"position_after"`
	LiquidationFlag bool    `json:"liquidation_flag"`
	Note            string  `json:"note"`
}

// RunIndexEntry backtest index entry
type RunIndexEntry struct {
	RunID          string   `json:"run_id"`
	State          string   `json:"state"`
	Symbols        []string `json:"symbols"`
	DecisionTF     string   `json:"decision_tf"`
	EquityLast     float64  `json:"equity_last"`
	MaxDrawdownPct float64  `json:"max_drawdown_pct"`
	StartTS        int64    `json:"start_ts"`
	EndTS          int64    `json:"end_ts"`
	CreatedAtISO   string   `json:"created_at"`
	UpdatedAtISO   string   `json:"updated_at"`
}

// BacktestRun GORM model for backtest_runs table
type BacktestRun struct {
	RunID           string    `gorm:"column:run_id;primaryKey"`
	UserID          string    `gorm:"column:user_id;not null;default:''"`
	ConfigJSON      []byte    `gorm:"column:config_json"`
	State           string    `gorm:"column:state;not null;default:created"`
	Label           string    `gorm:"column:label;default:''"`
	SymbolCount     int       `gorm:"column:symbol_count;default:0"`
	DecisionTF      string    `gorm:"column:decision_tf;default:''"`
	ProcessedBars   int       `gorm:"column:processed_bars;default:0"`
	ProgressPct     float64   `gorm:"column:progress_pct;default:0"`
	EquityLast      float64   `gorm:"column:equity_last;default:0"`
	MaxDrawdownPct  float64   `gorm:"column:max_drawdown_pct;default:0"`
	Liquidated      bool      `gorm:"column:liquidated;default:false"`
	LiquidationNote string    `gorm:"column:liquidation_note;default:''"`
	PromptTemplate  string    `gorm:"column:prompt_template;default:''"`
	CustomPrompt    string    `gorm:"column:custom_prompt;default:''"`
	OverridePrompt  bool      `gorm:"column:override_prompt;default:false"`
	AIProvider      string    `gorm:"column:ai_provider;default:''"`
	AIModel         string    `gorm:"column:ai_model;default:''"`
	LastError       string    `gorm:"column:last_error;default:''"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (BacktestRun) TableName() string {
	return "backtest_runs"
}

// BacktestCheckpoint GORM model
type BacktestCheckpoint struct {
	RunID     string    `gorm:"column:run_id;primaryKey"`
	Payload   []byte    `gorm:"column:payload;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (BacktestCheckpoint) TableName() string {
	return "backtest_checkpoints"
}

// BacktestEquity GORM model
type BacktestEquity struct {
	ID        int64   `gorm:"primaryKey;autoIncrement"`
	RunID     string  `gorm:"column:run_id;not null;index:idx_backtest_equity_run_ts"`
	TS        int64   `gorm:"column:ts;type:bigint;not null;index:idx_backtest_equity_run_ts"`
	Equity    float64 `gorm:"column:equity;not null"`
	Available float64 `gorm:"column:available;not null"`
	PnL       float64 `gorm:"column:pnl;not null"`
	PnLPct    float64 `gorm:"column:pnl_pct;not null"`
	DDPct     float64 `gorm:"column:dd_pct;not null"`
	Cycle     int     `gorm:"column:cycle;not null"`
}

func (BacktestEquity) TableName() string {
	return "backtest_equity"
}

// BacktestTrade GORM model
type BacktestTrade struct {
	ID            int64   `gorm:"primaryKey;autoIncrement"`
	RunID         string  `gorm:"column:run_id;not null;index:idx_backtest_trades_run_ts"`
	TS            int64   `gorm:"column:ts;type:bigint;not null;index:idx_backtest_trades_run_ts"`
	Symbol        string  `gorm:"column:symbol;not null"`
	Action        string  `gorm:"column:action;not null"`
	Side          string  `gorm:"column:side;default:''"`
	Qty           float64 `gorm:"column:qty;default:0"`
	Price         float64 `gorm:"column:price;default:0"`
	Fee           float64 `gorm:"column:fee;default:0"`
	Slippage      float64 `gorm:"column:slippage;default:0"`
	OrderValue    float64 `gorm:"column:order_value;default:0"`
	RealizedPnL   float64 `gorm:"column:realized_pnl;default:0"`
	Leverage      int     `gorm:"column:leverage;default:0"`
	Cycle         int     `gorm:"column:cycle;default:0"`
	PositionAfter float64 `gorm:"column:position_after;default:0"`
	Liquidation   bool    `gorm:"column:liquidation;default:false"`
	Note          string  `gorm:"column:note;default:''"`
}

func (BacktestTrade) TableName() string {
	return "backtest_trades"
}

// BacktestMetrics GORM model
type BacktestMetrics struct {
	RunID     string    `gorm:"column:run_id;primaryKey"`
	Payload   []byte    `gorm:"column:payload;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (BacktestMetrics) TableName() string {
	return "backtest_metrics"
}

// BacktestDecision GORM model
type BacktestDecision struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	RunID     string    `gorm:"column:run_id;not null;index:idx_backtest_decisions_run_cycle"`
	Cycle     int       `gorm:"column:cycle;not null;index:idx_backtest_decisions_run_cycle"`
	Payload   []byte    `gorm:"column:payload;not null"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (BacktestDecision) TableName() string {
	return "backtest_decisions"
}

// initTables initializes backtest related tables
func (s *BacktestStore) initTables() error {
	// For PostgreSQL with existing tables, skip AutoMigrate to avoid type conflicts
	if s.db.Dialector.Name() == "postgres" {
		var tableExists int64
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'backtest_runs'`).Scan(&tableExists)

		if tableExists > 0 {
			// Tables exist - fix column types and ensure indexes exist
			// Fix ts column type from INTEGER to BIGINT (timestamps in milliseconds exceed int4 max)
			s.db.Exec(`ALTER TABLE backtest_equity ALTER COLUMN ts TYPE BIGINT`)
			s.db.Exec(`ALTER TABLE backtest_trades ALTER COLUMN ts TYPE BIGINT`)
			s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_backtest_equity_run_ts ON backtest_equity(run_id, ts)`)
			s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_backtest_trades_run_ts ON backtest_trades(run_id, ts)`)
			s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_backtest_decisions_run_cycle ON backtest_decisions(run_id, cycle)`)
			return nil
		}
	}

	// AutoMigrate all backtest tables
	if err := s.db.AutoMigrate(
		&BacktestRun{},
		&BacktestCheckpoint{},
		&BacktestEquity{},
		&BacktestTrade{},
		&BacktestMetrics{},
		&BacktestDecision{},
	); err != nil {
		return fmt.Errorf("failed to migrate backtest tables: %w", err)
	}

	return nil
}

// SaveCheckpoint saves checkpoint
func (s *BacktestStore) SaveCheckpoint(runID string, payload []byte) error {
	checkpoint := BacktestCheckpoint{
		RunID:   runID,
		Payload: payload,
	}
	return s.db.Save(&checkpoint).Error
}

// LoadCheckpoint loads checkpoint
func (s *BacktestStore) LoadCheckpoint(runID string) ([]byte, error) {
	var checkpoint BacktestCheckpoint
	err := s.db.Where("run_id = ?", runID).First(&checkpoint).Error
	if err != nil {
		return nil, err
	}
	return checkpoint.Payload, nil
}

// SaveRunMetadata saves run metadata
func (s *BacktestStore) SaveRunMetadata(meta *RunMetadata) error {
	run := BacktestRun{
		RunID:           meta.RunID,
		UserID:          meta.UserID,
		State:           string(meta.State),
		Label:           meta.Label,
		LastError:       meta.LastError,
		SymbolCount:     meta.Summary.SymbolCount,
		DecisionTF:      meta.Summary.DecisionTF,
		ProcessedBars:   meta.Summary.ProcessedBars,
		ProgressPct:     meta.Summary.ProgressPct,
		EquityLast:      meta.Summary.EquityLast,
		MaxDrawdownPct:  meta.Summary.MaxDrawdownPct,
		Liquidated:      meta.Summary.Liquidated,
		LiquidationNote: meta.Summary.LiquidationNote,
		CreatedAt:       meta.CreatedAt,
		UpdatedAt:       meta.UpdatedAt,
	}
	return s.db.Save(&run).Error
}

// LoadRunMetadata loads run metadata
func (s *BacktestStore) LoadRunMetadata(runID string) (*RunMetadata, error) {
	var run BacktestRun
	err := s.db.Where("run_id = ?", runID).First(&run).Error
	if err != nil {
		return nil, err
	}

	return &RunMetadata{
		RunID:     run.RunID,
		UserID:    run.UserID,
		Version:   1,
		State:     RunState(run.State),
		Label:     run.Label,
		LastError: run.LastError,
		Summary: RunSummary{
			SymbolCount:     run.SymbolCount,
			DecisionTF:      run.DecisionTF,
			ProcessedBars:   run.ProcessedBars,
			ProgressPct:     run.ProgressPct,
			EquityLast:      run.EquityLast,
			MaxDrawdownPct:  run.MaxDrawdownPct,
			Liquidated:      run.Liquidated,
			LiquidationNote: run.LiquidationNote,
		},
		CreatedAt: run.CreatedAt,
		UpdatedAt: run.UpdatedAt,
	}, nil
}

// ListRunIDs lists all run IDs
func (s *BacktestStore) ListRunIDs() ([]string, error) {
	var runs []BacktestRun
	err := s.db.Order("updated_at DESC").Find(&runs).Error
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(runs))
	for i, run := range runs {
		ids[i] = run.RunID
	}
	return ids, nil
}

// AppendEquityPoint appends equity point
func (s *BacktestStore) AppendEquityPoint(runID string, point EquityPoint) error {
	eq := BacktestEquity{
		RunID:     runID,
		TS:        point.Timestamp,
		Equity:    point.Equity,
		Available: point.Available,
		PnL:       point.PnL,
		PnLPct:    point.PnLPct,
		DDPct:     point.DrawdownPct,
		Cycle:     point.Cycle,
	}
	return s.db.Create(&eq).Error
}

// LoadEquityPoints loads equity points
func (s *BacktestStore) LoadEquityPoints(runID string) ([]EquityPoint, error) {
	var eqs []BacktestEquity
	err := s.db.Where("run_id = ?", runID).Order("ts ASC").Find(&eqs).Error
	if err != nil {
		return nil, err
	}

	points := make([]EquityPoint, len(eqs))
	for i, eq := range eqs {
		points[i] = EquityPoint{
			Timestamp:   eq.TS,
			Equity:      eq.Equity,
			Available:   eq.Available,
			PnL:         eq.PnL,
			PnLPct:      eq.PnLPct,
			DrawdownPct: eq.DDPct,
			Cycle:       eq.Cycle,
		}
	}
	return points, nil
}

// AppendTradeEvent appends trade event
func (s *BacktestStore) AppendTradeEvent(runID string, event TradeEvent) error {
	trade := BacktestTrade{
		RunID:         runID,
		TS:            event.Timestamp,
		Symbol:        event.Symbol,
		Action:        event.Action,
		Side:          event.Side,
		Qty:           event.Quantity,
		Price:         event.Price,
		Fee:           event.Fee,
		Slippage:      event.Slippage,
		OrderValue:    event.OrderValue,
		RealizedPnL:   event.RealizedPnL,
		Leverage:      event.Leverage,
		Cycle:         event.Cycle,
		PositionAfter: event.PositionAfter,
		Liquidation:   event.LiquidationFlag,
		Note:          event.Note,
	}
	return s.db.Create(&trade).Error
}

// LoadTradeEvents loads trade events
func (s *BacktestStore) LoadTradeEvents(runID string) ([]TradeEvent, error) {
	var trades []BacktestTrade
	err := s.db.Where("run_id = ?", runID).Order("ts ASC").Find(&trades).Error
	if err != nil {
		return nil, err
	}

	events := make([]TradeEvent, len(trades))
	for i, trade := range trades {
		events[i] = TradeEvent{
			Timestamp:       trade.TS,
			Symbol:          trade.Symbol,
			Action:          trade.Action,
			Side:            trade.Side,
			Quantity:        trade.Qty,
			Price:           trade.Price,
			Fee:             trade.Fee,
			Slippage:        trade.Slippage,
			OrderValue:      trade.OrderValue,
			RealizedPnL:     trade.RealizedPnL,
			Leverage:        trade.Leverage,
			Cycle:           trade.Cycle,
			PositionAfter:   trade.PositionAfter,
			LiquidationFlag: trade.Liquidation,
			Note:            trade.Note,
		}
	}
	return events, nil
}

// SaveMetrics saves metrics
func (s *BacktestStore) SaveMetrics(runID string, payload []byte) error {
	metrics := BacktestMetrics{
		RunID:   runID,
		Payload: payload,
	}
	return s.db.Save(&metrics).Error
}

// LoadMetrics loads metrics
func (s *BacktestStore) LoadMetrics(runID string) ([]byte, error) {
	var metrics BacktestMetrics
	err := s.db.Where("run_id = ?", runID).First(&metrics).Error
	if err != nil {
		return nil, err
	}
	return metrics.Payload, nil
}

// SaveDecisionRecord saves decision record
func (s *BacktestStore) SaveDecisionRecord(runID string, cycle int, payload []byte) error {
	decision := BacktestDecision{
		RunID:   runID,
		Cycle:   cycle,
		Payload: payload,
	}
	return s.db.Create(&decision).Error
}

// LoadDecisionRecords loads decision records
func (s *BacktestStore) LoadDecisionRecords(runID string, limit, offset int) ([]json.RawMessage, error) {
	var decisions []BacktestDecision
	err := s.db.Where("run_id = ?", runID).
		Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&decisions).Error
	if err != nil {
		return nil, err
	}

	records := make([]json.RawMessage, len(decisions))
	for i, d := range decisions {
		records[i] = json.RawMessage(d.Payload)
	}
	return records, nil
}

// LoadLatestDecision loads latest decision
func (s *BacktestStore) LoadLatestDecision(runID string, cycle int) ([]byte, error) {
	var decision BacktestDecision
	query := s.db.Where("run_id = ?", runID)
	if cycle > 0 {
		query = query.Where("cycle = ?", cycle)
	}
	err := query.Order("created_at DESC").First(&decision).Error
	if err != nil {
		return nil, err
	}
	return decision.Payload, nil
}

// UpdateProgress updates progress
func (s *BacktestStore) UpdateProgress(runID string, progressPct, equity float64, barIndex int, liquidated bool) error {
	return s.db.Model(&BacktestRun{}).Where("run_id = ?", runID).Updates(map[string]interface{}{
		"progress_pct":   progressPct,
		"equity_last":    equity,
		"processed_bars": barIndex,
		"liquidated":     liquidated,
	}).Error
}

// ListIndexEntries lists index entries
func (s *BacktestStore) ListIndexEntries() ([]RunIndexEntry, error) {
	var runs []BacktestRun
	err := s.db.Order("updated_at DESC").Find(&runs).Error
	if err != nil {
		return nil, err
	}

	entries := make([]RunIndexEntry, len(runs))
	for i, run := range runs {
		entry := RunIndexEntry{
			RunID:          run.RunID,
			State:          run.State,
			DecisionTF:     run.DecisionTF,
			EquityLast:     run.EquityLast,
			MaxDrawdownPct: run.MaxDrawdownPct,
			CreatedAtISO:   run.CreatedAt.Format(time.RFC3339),
			UpdatedAtISO:   run.UpdatedAt.Format(time.RFC3339),
			Symbols:        make([]string, 0, run.SymbolCount),
		}

		if len(run.ConfigJSON) > 0 {
			var cfg struct {
				Symbols []string `json:"symbols"`
				StartTS int64    `json:"start_ts"`
				EndTS   int64    `json:"end_ts"`
			}
			if json.Unmarshal(run.ConfigJSON, &cfg) == nil {
				entry.Symbols = cfg.Symbols
				entry.StartTS = cfg.StartTS
				entry.EndTS = cfg.EndTS
			}
		}

		entries[i] = entry
	}
	return entries, nil
}

// DeleteRun deletes run
func (s *BacktestStore) DeleteRun(runID string) error {
	// Delete related records first (cascade may not work in all cases)
	s.db.Where("run_id = ?", runID).Delete(&BacktestCheckpoint{})
	s.db.Where("run_id = ?", runID).Delete(&BacktestEquity{})
	s.db.Where("run_id = ?", runID).Delete(&BacktestTrade{})
	s.db.Where("run_id = ?", runID).Delete(&BacktestMetrics{})
	s.db.Where("run_id = ?", runID).Delete(&BacktestDecision{})

	return s.db.Where("run_id = ?", runID).Delete(&BacktestRun{}).Error
}

// SaveConfig saves config
func (s *BacktestStore) SaveConfig(runID, userID, template, customPrompt, provider, model string, override bool, configJSON []byte) error {
	if userID == "" {
		userID = "default"
	}

	run := BacktestRun{
		RunID:          runID,
		UserID:         userID,
		ConfigJSON:     configJSON,
		PromptTemplate: template,
		CustomPrompt:   customPrompt,
		OverridePrompt: override,
		AIProvider:     provider,
		AIModel:        model,
	}
	return s.db.Save(&run).Error
}

// LoadConfig loads config
func (s *BacktestStore) LoadConfig(runID string) ([]byte, error) {
	var run BacktestRun
	err := s.db.Where("run_id = ?", runID).First(&run).Error
	if err != nil {
		return nil, err
	}
	return run.ConfigJSON, nil
}
