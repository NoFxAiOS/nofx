package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// BacktestStore 回测数据存储
type BacktestStore struct {
	db *sql.DB
}

// RunState 回测状态
type RunState string

const (
	RunStateCreated   RunState = "created"
	RunStateRunning   RunState = "running"
	RunStatePaused    RunState = "paused"
	RunStateCompleted RunState = "completed"
	RunStateFailed    RunState = "failed"
)

// RunMetadata 回测元数据
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

// RunSummary 回测摘要
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

// EquityPoint 权益点
type EquityPoint struct {
	Timestamp   int64   `json:"timestamp"`
	Equity      float64 `json:"equity"`
	Available   float64 `json:"available"`
	PnL         float64 `json:"pnl"`
	PnLPct      float64 `json:"pnl_pct"`
	DrawdownPct float64 `json:"drawdown_pct"`
	Cycle       int     `json:"cycle"`
}

// TradeEvent 交易事件
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

// RunIndexEntry 回测索引条目
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

// initTables 初始化回测相关表
func (s *BacktestStore) initTables() error {
	queries := []string{
		// 回测运行主表
		`CREATE TABLE IF NOT EXISTS backtest_runs (
			run_id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			config_json TEXT NOT NULL DEFAULT '',
			state TEXT NOT NULL DEFAULT 'created',
			label TEXT DEFAULT '',
			symbol_count INTEGER DEFAULT 0,
			decision_tf TEXT DEFAULT '',
			processed_bars INTEGER DEFAULT 0,
			progress_pct REAL DEFAULT 0,
			equity_last REAL DEFAULT 0,
			max_drawdown_pct REAL DEFAULT 0,
			liquidated BOOLEAN DEFAULT 0,
			liquidation_note TEXT DEFAULT '',
			prompt_template TEXT DEFAULT '',
			custom_prompt TEXT DEFAULT '',
			override_prompt BOOLEAN DEFAULT 0,
			ai_provider TEXT DEFAULT '',
			ai_model TEXT DEFAULT '',
			last_error TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// 回测检查点
		`CREATE TABLE IF NOT EXISTS backtest_checkpoints (
			run_id TEXT PRIMARY KEY,
			payload BLOB NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (run_id) REFERENCES backtest_runs(run_id) ON DELETE CASCADE
		)`,

		// 回测权益曲线
		`CREATE TABLE IF NOT EXISTS backtest_equity (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			ts INTEGER NOT NULL,
			equity REAL NOT NULL,
			available REAL NOT NULL,
			pnl REAL NOT NULL,
			pnl_pct REAL NOT NULL,
			dd_pct REAL NOT NULL,
			cycle INTEGER NOT NULL,
			FOREIGN KEY (run_id) REFERENCES backtest_runs(run_id) ON DELETE CASCADE
		)`,

		// 回测交易记录
		`CREATE TABLE IF NOT EXISTS backtest_trades (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			ts INTEGER NOT NULL,
			symbol TEXT NOT NULL,
			action TEXT NOT NULL,
			side TEXT DEFAULT '',
			qty REAL DEFAULT 0,
			price REAL DEFAULT 0,
			fee REAL DEFAULT 0,
			slippage REAL DEFAULT 0,
			order_value REAL DEFAULT 0,
			realized_pnl REAL DEFAULT 0,
			leverage INTEGER DEFAULT 0,
			cycle INTEGER DEFAULT 0,
			position_after REAL DEFAULT 0,
			liquidation BOOLEAN DEFAULT 0,
			note TEXT DEFAULT '',
			FOREIGN KEY (run_id) REFERENCES backtest_runs(run_id) ON DELETE CASCADE
		)`,

		// 回测指标
		`CREATE TABLE IF NOT EXISTS backtest_metrics (
			run_id TEXT PRIMARY KEY,
			payload BLOB NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (run_id) REFERENCES backtest_runs(run_id) ON DELETE CASCADE
		)`,

		// 回测决策日志
		`CREATE TABLE IF NOT EXISTS backtest_decisions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			cycle INTEGER NOT NULL,
			payload BLOB NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (run_id) REFERENCES backtest_runs(run_id) ON DELETE CASCADE
		)`,

		// 索引
		`CREATE INDEX IF NOT EXISTS idx_backtest_runs_state ON backtest_runs(state, updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_backtest_equity_run_ts ON backtest_equity(run_id, ts)`,
		`CREATE INDEX IF NOT EXISTS idx_backtest_trades_run_ts ON backtest_trades(run_id, ts)`,
		`CREATE INDEX IF NOT EXISTS idx_backtest_decisions_run_cycle ON backtest_decisions(run_id, cycle)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("执行SQL失败: %w", err)
		}
	}

	// 添加可能缺失的列（向后兼容）
	s.addColumnIfNotExists("backtest_runs", "label", "TEXT DEFAULT ''")
	s.addColumnIfNotExists("backtest_runs", "last_error", "TEXT DEFAULT ''")
	s.addColumnIfNotExists("backtest_trades", "leverage", "INTEGER DEFAULT 0")

	return nil
}

func (s *BacktestStore) addColumnIfNotExists(table, column, definition string) {
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			continue
		}
		if name == column {
			return // 列已存在
		}
	}

	s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
}

// SaveCheckpoint 保存检查点
func (s *BacktestStore) SaveCheckpoint(runID string, payload []byte) error {
	_, err := s.db.Exec(`
		INSERT INTO backtest_checkpoints (run_id, payload, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(run_id) DO UPDATE SET payload=excluded.payload, updated_at=CURRENT_TIMESTAMP
	`, runID, payload)
	return err
}

// LoadCheckpoint 加载检查点
func (s *BacktestStore) LoadCheckpoint(runID string) ([]byte, error) {
	var payload []byte
	err := s.db.QueryRow(`SELECT payload FROM backtest_checkpoints WHERE run_id = ?`, runID).Scan(&payload)
	return payload, err
}

// SaveRunMetadata 保存运行元数据
func (s *BacktestStore) SaveRunMetadata(meta *RunMetadata) error {
	created := meta.CreatedAt.UTC().Format(time.RFC3339)
	updated := meta.UpdatedAt.UTC().Format(time.RFC3339)
	userID := meta.UserID

	if _, err := s.db.Exec(`
		INSERT INTO backtest_runs (run_id, user_id, label, last_error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id) DO NOTHING
	`, meta.RunID, userID, meta.Label, meta.LastError, created, updated); err != nil {
		return err
	}

	_, err := s.db.Exec(`
		UPDATE backtest_runs
		SET user_id = ?, state = ?, symbol_count = ?, decision_tf = ?, processed_bars = ?,
		    progress_pct = ?, equity_last = ?, max_drawdown_pct = ?, liquidated = ?,
		    liquidation_note = ?, label = ?, last_error = ?, updated_at = ?
		WHERE run_id = ?
	`, userID, string(meta.State), meta.Summary.SymbolCount, meta.Summary.DecisionTF,
		meta.Summary.ProcessedBars, meta.Summary.ProgressPct, meta.Summary.EquityLast,
		meta.Summary.MaxDrawdownPct, meta.Summary.Liquidated, meta.Summary.LiquidationNote,
		meta.Label, meta.LastError, updated, meta.RunID)
	return err
}

// LoadRunMetadata 加载运行元数据
func (s *BacktestStore) LoadRunMetadata(runID string) (*RunMetadata, error) {
	var (
		userID          string
		state           string
		label           string
		lastErr         string
		symbolCount     int
		decisionTF      string
		processedBars   int
		progressPct     float64
		equityLast      float64
		maxDD           float64
		liquidated      bool
		liquidationNote string
		createdISO      string
		updatedISO      string
	)

	err := s.db.QueryRow(`
		SELECT user_id, state, label, last_error, symbol_count, decision_tf, processed_bars,
		       progress_pct, equity_last, max_drawdown_pct, liquidated, liquidation_note,
		       created_at, updated_at
		FROM backtest_runs WHERE run_id = ?
	`, runID).Scan(&userID, &state, &label, &lastErr, &symbolCount, &decisionTF,
		&processedBars, &progressPct, &equityLast, &maxDD, &liquidated, &liquidationNote,
		&createdISO, &updatedISO)
	if err != nil {
		return nil, err
	}

	meta := &RunMetadata{
		RunID:     runID,
		UserID:    userID,
		Version:   1,
		State:     RunState(state),
		Label:     label,
		LastError: lastErr,
		Summary: RunSummary{
			SymbolCount:     symbolCount,
			DecisionTF:      decisionTF,
			ProcessedBars:   processedBars,
			ProgressPct:     progressPct,
			EquityLast:      equityLast,
			MaxDrawdownPct:  maxDD,
			Liquidated:      liquidated,
			LiquidationNote: liquidationNote,
		},
	}

	meta.CreatedAt, _ = time.Parse(time.RFC3339, createdISO)
	meta.UpdatedAt, _ = time.Parse(time.RFC3339, updatedISO)

	return meta, nil
}

// ListRunIDs 列出所有运行ID
func (s *BacktestStore) ListRunIDs() ([]string, error) {
	rows, err := s.db.Query(`SELECT run_id FROM backtest_runs ORDER BY datetime(updated_at) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var runID string
		if err := rows.Scan(&runID); err != nil {
			return nil, err
		}
		ids = append(ids, runID)
	}
	return ids, rows.Err()
}

// AppendEquityPoint 添加权益点
func (s *BacktestStore) AppendEquityPoint(runID string, point EquityPoint) error {
	_, err := s.db.Exec(`
		INSERT INTO backtest_equity (run_id, ts, equity, available, pnl, pnl_pct, dd_pct, cycle)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, runID, point.Timestamp, point.Equity, point.Available, point.PnL,
		point.PnLPct, point.DrawdownPct, point.Cycle)
	return err
}

// LoadEquityPoints 加载权益点
func (s *BacktestStore) LoadEquityPoints(runID string) ([]EquityPoint, error) {
	rows, err := s.db.Query(`
		SELECT ts, equity, available, pnl, pnl_pct, dd_pct, cycle
		FROM backtest_equity WHERE run_id = ? ORDER BY ts ASC
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]EquityPoint, 0)
	for rows.Next() {
		var point EquityPoint
		if err := rows.Scan(&point.Timestamp, &point.Equity, &point.Available,
			&point.PnL, &point.PnLPct, &point.DrawdownPct, &point.Cycle); err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, rows.Err()
}

// AppendTradeEvent 添加交易事件
func (s *BacktestStore) AppendTradeEvent(runID string, event TradeEvent) error {
	_, err := s.db.Exec(`
		INSERT INTO backtest_trades (run_id, ts, symbol, action, side, qty, price, fee,
		                             slippage, order_value, realized_pnl, leverage, cycle,
		                             position_after, liquidation, note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, runID, event.Timestamp, event.Symbol, event.Action, event.Side, event.Quantity,
		event.Price, event.Fee, event.Slippage, event.OrderValue, event.RealizedPnL,
		event.Leverage, event.Cycle, event.PositionAfter, event.LiquidationFlag, event.Note)
	return err
}

// LoadTradeEvents 加载交易事件
func (s *BacktestStore) LoadTradeEvents(runID string) ([]TradeEvent, error) {
	rows, err := s.db.Query(`
		SELECT ts, symbol, action, side, qty, price, fee, slippage, order_value,
		       realized_pnl, leverage, cycle, position_after, liquidation, note
		FROM backtest_trades WHERE run_id = ? ORDER BY ts ASC
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]TradeEvent, 0)
	for rows.Next() {
		var event TradeEvent
		if err := rows.Scan(&event.Timestamp, &event.Symbol, &event.Action, &event.Side,
			&event.Quantity, &event.Price, &event.Fee, &event.Slippage, &event.OrderValue,
			&event.RealizedPnL, &event.Leverage, &event.Cycle, &event.PositionAfter,
			&event.LiquidationFlag, &event.Note); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// SaveMetrics 保存指标
func (s *BacktestStore) SaveMetrics(runID string, payload []byte) error {
	_, err := s.db.Exec(`
		INSERT INTO backtest_metrics (run_id, payload, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(run_id) DO UPDATE SET payload=excluded.payload, updated_at=CURRENT_TIMESTAMP
	`, runID, payload)
	return err
}

// LoadMetrics 加载指标
func (s *BacktestStore) LoadMetrics(runID string) ([]byte, error) {
	var payload []byte
	err := s.db.QueryRow(`SELECT payload FROM backtest_metrics WHERE run_id = ?`, runID).Scan(&payload)
	return payload, err
}

// SaveDecisionRecord 保存决策记录
func (s *BacktestStore) SaveDecisionRecord(runID string, cycle int, payload []byte) error {
	_, err := s.db.Exec(`
		INSERT INTO backtest_decisions (run_id, cycle, payload)
		VALUES (?, ?, ?)
	`, runID, cycle, payload)
	return err
}

// LoadDecisionRecords 加载决策记录
func (s *BacktestStore) LoadDecisionRecords(runID string, limit, offset int) ([]json.RawMessage, error) {
	rows, err := s.db.Query(`
		SELECT payload FROM backtest_decisions
		WHERE run_id = ?
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, runID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]json.RawMessage, 0, limit)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		records = append(records, json.RawMessage(payload))
	}
	return records, rows.Err()
}

// LoadLatestDecision 加载最新决策
func (s *BacktestStore) LoadLatestDecision(runID string, cycle int) ([]byte, error) {
	var query string
	var args []interface{}

	if cycle > 0 {
		query = `SELECT payload FROM backtest_decisions WHERE run_id = ? AND cycle = ? ORDER BY datetime(created_at) DESC LIMIT 1`
		args = []interface{}{runID, cycle}
	} else {
		query = `SELECT payload FROM backtest_decisions WHERE run_id = ? ORDER BY datetime(created_at) DESC LIMIT 1`
		args = []interface{}{runID}
	}

	var payload []byte
	err := s.db.QueryRow(query, args...).Scan(&payload)
	return payload, err
}

// UpdateProgress 更新进度
func (s *BacktestStore) UpdateProgress(runID string, progressPct, equity float64, barIndex int, liquidated bool) error {
	_, err := s.db.Exec(`
		UPDATE backtest_runs
		SET progress_pct = ?, equity_last = ?, processed_bars = ?, liquidated = ?, updated_at = CURRENT_TIMESTAMP
		WHERE run_id = ?
	`, progressPct, equity, barIndex, liquidated, runID)
	return err
}

// ListIndexEntries 列出索引条目
func (s *BacktestStore) ListIndexEntries() ([]RunIndexEntry, error) {
	rows, err := s.db.Query(`
		SELECT run_id, state, symbol_count, decision_tf, equity_last, max_drawdown_pct,
		       created_at, updated_at, config_json
		FROM backtest_runs
		ORDER BY datetime(updated_at) DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []RunIndexEntry
	for rows.Next() {
		var entry RunIndexEntry
		var symbolCnt int
		var cfgJSON []byte
		var createdISO, updatedISO string

		if err := rows.Scan(&entry.RunID, &entry.State, &symbolCnt, &entry.DecisionTF,
			&entry.EquityLast, &entry.MaxDrawdownPct, &createdISO, &updatedISO, &cfgJSON); err != nil {
			return nil, err
		}

		entry.CreatedAtISO = createdISO
		entry.UpdatedAtISO = updatedISO
		entry.Symbols = make([]string, 0, symbolCnt)

		// 尝试从配置中提取更多信息
		if len(cfgJSON) > 0 {
			var cfg struct {
				Symbols []string `json:"symbols"`
				StartTS int64    `json:"start_ts"`
				EndTS   int64    `json:"end_ts"`
			}
			if json.Unmarshal(cfgJSON, &cfg) == nil {
				entry.Symbols = cfg.Symbols
				entry.StartTS = cfg.StartTS
				entry.EndTS = cfg.EndTS
			}
		}

		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// DeleteRun 删除运行
func (s *BacktestStore) DeleteRun(runID string) error {
	_, err := s.db.Exec(`DELETE FROM backtest_runs WHERE run_id = ?`, runID)
	return err
}

// SaveConfig 保存配置
func (s *BacktestStore) SaveConfig(runID, userID, template, customPrompt, provider, model string, override bool, configJSON []byte) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if userID == "" {
		userID = "default"
	}

	_, err := s.db.Exec(`
		INSERT INTO backtest_runs (run_id, user_id, config_json, prompt_template, custom_prompt,
		                           override_prompt, ai_provider, ai_model, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id) DO NOTHING
	`, runID, userID, configJSON, template, customPrompt, override, provider, model, now, now)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		UPDATE backtest_runs
		SET user_id = ?, config_json = ?, prompt_template = ?, custom_prompt = ?,
		    override_prompt = ?, ai_provider = ?, ai_model = ?, updated_at = CURRENT_TIMESTAMP
		WHERE run_id = ?
	`, userID, configJSON, template, customPrompt, override, provider, model, runID)
	return err
}

// LoadConfig 加载配置
func (s *BacktestStore) LoadConfig(runID string) ([]byte, error) {
	var payload []byte
	err := s.db.QueryRow(`SELECT config_json FROM backtest_runs WHERE run_id = ?`, runID).Scan(&payload)
	return payload, err
}
