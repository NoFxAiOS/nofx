package backtest

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"nofx/store"
)

func saveCheckpointDB(runID string, ckpt *Checkpoint) error {
	data, err := json.Marshal(ckpt)
	if err != nil {
		return err
	}
	_, err = persistenceDB.Exec(`
		INSERT INTO backtest_checkpoints (run_id, payload, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(run_id) DO UPDATE SET payload=excluded.payload, updated_at=CURRENT_TIMESTAMP
	`, runID, data)
	return err
}

func loadCheckpointDB(runID string) (*Checkpoint, error) {
	var payload []byte
	err := persistenceDB.QueryRow(`SELECT payload FROM backtest_checkpoints WHERE run_id = ?`, runID).Scan(&payload)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}
	var ckpt Checkpoint
	if err := json.Unmarshal(payload, &ckpt); err != nil {
		return nil, err
	}
	return &ckpt, nil
}

func saveConfigDB(runID string, cfg *BacktestConfig) error {
	persist := *cfg
	persist.AICfg.APIKey = ""
	data, err := json.Marshal(&persist)
	if err != nil {
		return err
	}
	template := cfg.PromptTemplate
	if template == "" {
		template = "default"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	userID := cfg.UserID
	if userID == "" {
		userID = "default"
	}
	_, err = persistenceDB.Exec(`
		INSERT INTO backtest_runs (run_id, user_id, config_json, prompt_template, custom_prompt, override_prompt, ai_provider, ai_model, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id) DO NOTHING
	`, runID, userID, data, template, cfg.CustomPrompt, cfg.OverrideBasePrompt, cfg.AICfg.Provider, cfg.AICfg.Model, now, now)
	if err != nil {
		return err
	}
	_, err = persistenceDB.Exec(`
		UPDATE backtest_runs
		SET user_id = ?, config_json = ?, prompt_template = ?, custom_prompt = ?, override_prompt = ?, ai_provider = ?, ai_model = ?, updated_at = CURRENT_TIMESTAMP
		WHERE run_id = ?
	`, userID, data, template, cfg.CustomPrompt, cfg.OverrideBasePrompt, cfg.AICfg.Provider, cfg.AICfg.Model, runID)
	return err
}

func loadConfigDB(runID string) (*BacktestConfig, error) {
	var payload []byte
	err := persistenceDB.QueryRow(`SELECT config_json FROM backtest_runs WHERE run_id = ?`, runID).Scan(&payload)
	if err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("config missing for %s", runID)
	}
	var cfg BacktestConfig
	if err := json.Unmarshal(payload, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveRunMetadataDB(meta *RunMetadata) error {
	created := meta.CreatedAt.UTC().Format(time.RFC3339)
	updated := meta.UpdatedAt.UTC().Format(time.RFC3339)
	userID := meta.UserID
	if userID == "" {
		userID = "default"
	}
	if _, err := persistenceDB.Exec(`
		INSERT INTO backtest_runs (run_id, user_id, label, last_error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id) DO NOTHING
	`, meta.RunID, userID, meta.Label, meta.LastError, created, updated); err != nil {
		return err
	}
	_, err := persistenceDB.Exec(`
		UPDATE backtest_runs
		SET user_id = ?, state = ?, symbol_count = ?, decision_tf = ?, processed_bars = ?, progress_pct = ?, equity_last = ?, max_drawdown_pct = ?, liquidated = ?, liquidation_note = ?, label = ?, last_error = ?, updated_at = ?
		WHERE run_id = ?
	`, userID, string(meta.State), meta.Summary.SymbolCount, meta.Summary.DecisionTF, meta.Summary.ProcessedBars, meta.Summary.ProgressPct, meta.Summary.EquityLast, meta.Summary.MaxDrawdownPct, meta.Summary.Liquidated, meta.Summary.LiquidationNote, meta.Label, meta.LastError, updated, meta.RunID)
	return err
}

func loadRunMetadataDB(runID string) (*RunMetadata, error) {
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
	err := persistenceDB.QueryRow(`
		SELECT user_id, state, label, last_error, symbol_count, decision_tf, processed_bars, progress_pct, equity_last, max_drawdown_pct, liquidated, liquidation_note, created_at, updated_at
		FROM backtest_runs WHERE run_id = ?
	`, runID).Scan(&userID, &state, &label, &lastErr, &symbolCount, &decisionTF, &processedBars, &progressPct, &equityLast, &maxDD, &liquidated, &liquidationNote, &createdISO, &updatedISO)
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
	if meta.UserID == "" {
		meta.UserID = "default"
	}
	if t, err := time.Parse(time.RFC3339, createdISO); err == nil {
		meta.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, updatedISO); err == nil {
		meta.UpdatedAt = t
	}
	return meta, nil
}

func loadRunIDsDB() ([]string, error) {
	rows, err := persistenceDB.Query(`SELECT run_id FROM backtest_runs ORDER BY datetime(updated_at) DESC`)
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

func appendEquityPointDB(runID string, point EquityPoint) error {
	_, err := persistenceDB.Exec(`
		INSERT INTO backtest_equity (run_id, ts, equity, available, pnl, pnl_pct, dd_pct, cycle)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, runID, point.Timestamp, point.Equity, point.Available, point.PnL, point.PnLPct, point.DrawdownPct, point.Cycle)
	return err
}

func loadEquityPointsDB(runID string) ([]EquityPoint, error) {
	rows, err := persistenceDB.Query(`
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
		if err := rows.Scan(&point.Timestamp, &point.Equity, &point.Available, &point.PnL, &point.PnLPct, &point.DrawdownPct, &point.Cycle); err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, rows.Err()
}

func appendTradeEventDB(runID string, event TradeEvent) error {
	_, err := persistenceDB.Exec(`
		INSERT INTO backtest_trades (run_id, ts, symbol, action, side, qty, price, fee, slippage, order_value, realized_pnl, leverage, cycle, position_after, liquidation, note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, runID, event.Timestamp, event.Symbol, event.Action, event.Side, event.Quantity, event.Price, event.Fee, event.Slippage, event.OrderValue, event.RealizedPnL, event.Leverage, event.Cycle, event.PositionAfter, event.LiquidationFlag, event.Note)
	return err
}

func loadTradeEventsDB(runID string) ([]TradeEvent, error) {
	rows, err := persistenceDB.Query(`
		SELECT ts, symbol, action, side, qty, price, fee, slippage, order_value, realized_pnl, leverage, cycle, position_after, liquidation, note
		FROM backtest_trades WHERE run_id = ? ORDER BY ts ASC
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]TradeEvent, 0)
	for rows.Next() {
		var event TradeEvent
		if err := rows.Scan(&event.Timestamp, &event.Symbol, &event.Action, &event.Side, &event.Quantity, &event.Price, &event.Fee, &event.Slippage, &event.OrderValue, &event.RealizedPnL, &event.Leverage, &event.Cycle, &event.PositionAfter, &event.LiquidationFlag, &event.Note); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func saveMetricsDB(runID string, metrics *Metrics) error {
	data, err := json.Marshal(metrics)
	if err != nil {
		return err
	}
	_, err = persistenceDB.Exec(`
		INSERT INTO backtest_metrics (run_id, payload, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(run_id) DO UPDATE SET payload=excluded.payload, updated_at=CURRENT_TIMESTAMP
	`, runID, data)
	return err
}

func loadMetricsDB(runID string) (*Metrics, error) {
	var payload []byte
	err := persistenceDB.QueryRow(`SELECT payload FROM backtest_metrics WHERE run_id = ?`, runID).Scan(&payload)
	if err != nil {
		return nil, err
	}
	var metrics Metrics
	if err := json.Unmarshal(payload, &metrics); err != nil {
		return nil, err
	}
	return &metrics, nil
}

func saveProgressDB(runID string, payload progressPayload) error {
	_, err := persistenceDB.Exec(`
		UPDATE backtest_runs
		SET progress_pct = ?, equity_last = ?, processed_bars = ?, liquidated = ?, updated_at = ?
		WHERE run_id = ?
	`, payload.ProgressPct, payload.Equity, payload.BarIndex, payload.Liquidated, payload.UpdatedAtISO, runID)
	return err
}

func loadDecisionTraceDB(runID string, cycle int) (*store.DecisionRecord, error) {
	query := `SELECT payload FROM backtest_decisions WHERE run_id = ?`
	var rows *sql.Rows
	var err error
	if cycle > 0 {
		rows, err = persistenceDB.Query(query+` AND cycle = ? ORDER BY datetime(created_at) DESC LIMIT 1`, runID, cycle)
	} else {
		rows, err = persistenceDB.Query(query+` ORDER BY datetime(created_at) DESC LIMIT 1`, runID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, fmt.Errorf("decision trace not found for %s", runID)
	}
	var payload []byte
	if err := rows.Scan(&payload); err != nil {
		return nil, err
	}
	var record store.DecisionRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func saveDecisionRecordDB(runID string, record *store.DecisionRecord) error {
	if record == nil {
		return nil
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = persistenceDB.Exec(`
		INSERT INTO backtest_decisions (run_id, cycle, payload)
		VALUES (?, ?, ?)
	`, runID, record.CycleNumber, data)
	return err
}

func loadDecisionRecordsDB(runID string, limit, offset int) ([]*store.DecisionRecord, error) {
	rows, err := persistenceDB.Query(`
		SELECT payload FROM backtest_decisions
		WHERE run_id = ?
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, runID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]*store.DecisionRecord, 0, limit)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var record store.DecisionRecord
		if err := json.Unmarshal(payload, &record); err != nil {
			return nil, err
		}
		records = append(records, &record)
	}
	return records, rows.Err()
}

func createRunExportDB(runID string) (string, error) {
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("%s-*.zip", runID))
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	zipWriter := zip.NewWriter(tmpFile)
	defer zipWriter.Close()

	if meta, err := loadRunMetadataDB(runID); err == nil {
		if err := writeJSONToZip(zipWriter, "run.json", meta); err != nil {
			return "", err
		}
	}
	if cfg, err := loadConfigDB(runID); err == nil {
		if err := writeJSONToZip(zipWriter, "config.json", cfg); err != nil {
			return "", err
		}
	}
	if ckpt, err := loadCheckpointDB(runID); err == nil {
		if err := writeJSONToZip(zipWriter, "checkpoint.json", ckpt); err != nil {
			return "", err
		}
	}
	if metrics, err := loadMetricsDB(runID); err == nil {
		if err := writeJSONToZip(zipWriter, "metrics.json", metrics); err != nil {
			return "", err
		}
	}
	if points, err := loadEquityPointsDB(runID); err == nil && len(points) > 0 {
		if err := writeJSONLinesToZip(zipWriter, "equity.jsonl", points); err != nil {
			return "", err
		}
	}
	if trades, err := loadTradeEventsDB(runID); err == nil && len(trades) > 0 {
		if err := writeJSONLinesToZip(zipWriter, "trades.jsonl", trades); err != nil {
			return "", err
		}
	}
	if err := writeDecisionLogsToZip(zipWriter, runID); err != nil {
		return "", err
	}

	if err := zipWriter.Close(); err != nil {
		return "", err
	}
	if err := tmpFile.Sync(); err != nil {
		return "", err
	}
	return tmpFile.Name(), nil
}

func writeJSONToZip(z *zip.Writer, name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	w, err := z.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func writeJSONLinesToZip[T any](z *zip.Writer, name string, items []T) error {
	w, err := z.Create(name)
	if err != nil {
		return err
	}
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return err
		}
	}
	return nil
}

func writeDecisionLogsToZip(z *zip.Writer, runID string) error {
	rows, err := persistenceDB.Query(`
		SELECT id, cycle, payload FROM backtest_decisions
		WHERE run_id = ? ORDER BY id ASC
	`, runID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id      int64
			cycle   int
			payload []byte
		)
		if err := rows.Scan(&id, &cycle, &payload); err != nil {
			return err
		}
		name := fmt.Sprintf("decision_logs/decision_%d_cycle%d.json", id, cycle)
		w, err := z.Create(name)
		if err != nil {
			return err
		}
		if _, err := w.Write(payload); err != nil {
			return err
		}
	}
	return rows.Err()
}

func listIndexEntriesDB() ([]RunIndexEntry, error) {
	rows, err := persistenceDB.Query(`
		SELECT run_id, state, symbol_count, decision_tf, equity_last, max_drawdown_pct, created_at, updated_at, config_json
		FROM backtest_runs
		ORDER BY datetime(updated_at) DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []RunIndexEntry
	for rows.Next() {
		var (
			entry      RunIndexEntry
			createdISO string
			updatedISO string
			cfgJSON    []byte
			symbolCnt  int
		)
		if err := rows.Scan(&entry.RunID, &entry.State, &symbolCnt, &entry.DecisionTF, &entry.EquityLast, &entry.MaxDrawdownPct, &createdISO, &updatedISO, &cfgJSON); err != nil {
			return nil, err
		}
		entry.CreatedAtISO = createdISO
		entry.UpdatedAtISO = updatedISO
		entry.Symbols = make([]string, 0, symbolCnt)
		var cfg BacktestConfig
		if len(cfgJSON) > 0 && json.Unmarshal(cfgJSON, &cfg) == nil {
			entry.Symbols = append([]string(nil), cfg.Symbols...)
			entry.StartTS = cfg.StartTS
			entry.EndTS = cfg.EndTS
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func deleteRunDB(runID string) error {
	_, err := persistenceDB.Exec(`DELETE FROM backtest_runs WHERE run_id = ?`, runID)
	return err
}
