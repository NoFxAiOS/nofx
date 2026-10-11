package store

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// adaptivePriceRound rounds a price based on its magnitude to preserve meaningful precision.
// For small prices (like meme coins), it preserves more decimal places.
// It detects the number of decimal places needed from the reference price(s).
func adaptivePriceRound(price float64, referencePrices ...float64) float64 {
	if price == 0 {
		return 0
	}

	// Find the minimum magnitude among all prices (including the price itself)
	minMagnitude := math.Abs(price)
	for _, ref := range referencePrices {
		if ref > 0 && ref < minMagnitude {
			minMagnitude = ref
		}
	}

	// Determine decimal places needed based on price magnitude
	// For price 0.000000541, we need ~15 decimal places
	// For price 0.0001, we need ~8 decimal places
	// For price 1.0, we need ~4 decimal places
	var multiplier float64
	switch {
	case minMagnitude < 0.000001: // Ultra small (meme coins like CHEEMS, SHIB)
		multiplier = 1e15 // 15 decimal places
	case minMagnitude < 0.0001: // Very small (PEPE, FLOKI)
		multiplier = 1e12 // 12 decimal places
	case minMagnitude < 0.01: // Small
		multiplier = 1e10 // 10 decimal places
	case minMagnitude < 1: // Medium
		multiplier = 1e8 // 8 decimal places
	default: // Large
		multiplier = 1e6 // 6 decimal places
	}

	return math.Round(price*multiplier) / multiplier
}

// getPriceDecimalPlaces returns the number of decimal places in a price string
func getPriceDecimalPlaces(price float64) int {
	if price == 0 {
		return 0
	}
	s := strconv.FormatFloat(price, 'f', -1, 64)
	idx := strings.Index(s, ".")
	if idx == -1 {
		return 0
	}
	return len(s) - idx - 1
}

// formatDuration formats a duration
func formatDuration(d time.Duration) string {
	return formatDurationMs(d.Milliseconds())
}

// formatDurationMs formats a duration in milliseconds
func formatDurationMs(ms int64) string {
	seconds := ms / 1000
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24

	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	if hours < 24 {
		remainingMins := minutes % 60
		if remainingMins == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, remainingMins)
	}
	remainingHours := hours % 24
	if remainingHours == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd%dh", days, remainingHours)
}

// TraderPosition position record
// All time fields use int64 millisecond timestamps (UTC) to avoid timezone issues
type TraderPosition struct {
	ID                 int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	TraderID           string  `gorm:"column:trader_id;not null;index:idx_positions_trader" json:"trader_id"`
	ExchangeID         string  `gorm:"column:exchange_id;not null;default:'';index:idx_positions_exchange" json:"exchange_id"`
	ExchangeType       string  `gorm:"column:exchange_type;not null;default:''" json:"exchange_type"`
	ExchangePositionID string  `gorm:"column:exchange_position_id;not null;default:''" json:"exchange_position_id"`
	Symbol             string  `gorm:"column:symbol;not null" json:"symbol"`
	Side               string  `gorm:"column:side;not null" json:"side"`
	EntryQuantity      float64 `gorm:"column:entry_quantity;default:0" json:"entry_quantity"`
	Quantity           float64 `gorm:"column:quantity;not null" json:"quantity"`
	EntryPrice         float64 `gorm:"column:entry_price;not null" json:"entry_price"`
	EntryOrderID       string  `gorm:"column:entry_order_id;default:''" json:"entry_order_id"`
	EntryTime          int64   `gorm:"column:entry_time;not null;index:idx_positions_entry" json:"entry_time"` // Unix milliseconds UTC
	ExitPrice          float64 `gorm:"column:exit_price;default:0" json:"exit_price"`
	ExitOrderID        string  `gorm:"column:exit_order_id;default:''" json:"exit_order_id"`
	ExitTime           int64   `gorm:"column:exit_time;index:idx_positions_exit" json:"exit_time"` // Unix milliseconds UTC, 0 means not set
	RealizedPnL        float64 `gorm:"column:realized_pnl;default:0" json:"realized_pnl"`
	Fee                float64 `gorm:"column:fee;default:0" json:"fee"`
	Leverage           int     `gorm:"column:leverage;default:1" json:"leverage"`
	Status             string  `gorm:"column:status;default:OPEN;index:idx_positions_status" json:"status"`
	CloseReason        string  `gorm:"column:close_reason;default:''" json:"close_reason"`
	Source             string  `gorm:"column:source;default:system" json:"source"`
	// AIManaged: the position belonged to the AI's book (09-28 review P3).
	// Order sync ingests the WHOLE exchange account, so manual trades (and
	// other traders sharing the account) fold into these rows — without the
	// attribution, a manual win resets the AI's loss-streak circuit breaker
	// and a manual loss extends it. Stamped from the ai_managed_positions
	// registry at row creation (AI open paths mark BEFORE placing the order,
	// so the mark always exists by the time OrderSync sees the fill) and
	// re-checked at close. The loss-streak breaker consumes this flag.
	AIManaged bool `gorm:"column:ai_managed;default:false" json:"ai_managed"`
	// InitialStopLoss is the OPENING-risk stop captured at entry (write-once).
	// It anchors the 1R profit lock — later stop moves (AI tighten, trailing,
	// breakeven) rewrite only the live stop and must not pull the R bar along.
	InitialStopLoss float64 `gorm:"column:initial_stop_loss;default:0" json:"initial_stop_loss"`
	// R1TrimDone / TPTrimDone: restart-persistent one-shot markers for the
	// exit ladder (09-28 review P1). The in-memory maps (r1TrimDone /
	// tpTrimDone) die with the process — a restart while a position still
	// sits ≥1R (or ≥ the ROE trim threshold) re-fired the reductions on the
	// remainder with no plan change. Written on successful trim, read as OR
	// alongside the in-memory flags (memory stays the fast path, the row is
	// the authority across restarts).
	R1TrimDone bool `gorm:"column:r1_trim_done;default:false" json:"r1_trim_done"`
	TPTrimDone bool `gorm:"column:tp_trim_done;default:false" json:"tp_trim_done"`
	// ExitMode is the exit template the program executes for this position
	// (user menu directive 09-29): "trend" = split TP + trailing runner,
	// "range" = full TP at the chosen target, "quick" = full TP + program
	// time stop. Chosen by the model at open (Decision.ExitMode), stamped
	// write-once here so the exit ladder survives restarts. ''/'trend' both
	// mean the legacy ladder.
	ExitMode string `gorm:"column:exit_mode;default:''" json:"exit_mode,omitempty"`
	// Exit excursion replay over 1m klines (entry_time → exit_time): the max
	// ADVERSE / max FAVORABLE excursion as % of entry, and the same in R
	// (÷|entry − initial_stop_loss|) when the write-once anchor exists. The
	// price-pct form is the durable one — R under any HYPOTHETICAL stop is
	// pct ÷ that stop's distance% — which is what makes offline exit-ladder
	// ablation possible without re-replaying. 0 = not replayed yet (the
	// per-cycle backfill trickles newest-first); -1 = replay attempted and
	// the window was unreplayable (sentinel, never a physical excursion).
	MAEPct    float64 `gorm:"column:mae_pct;default:0" json:"mae_pct"`
	MFEPct    float64 `gorm:"column:mfe_pct;default:0" json:"mfe_pct"`
	MAER      float64 `gorm:"column:mae_r;default:0" json:"mae_r"`
	MFER      float64 `gorm:"column:mfe_r;default:0" json:"mfe_r"`
	CreatedAt int64   `gorm:"column:created_at" json:"created_at"` // Unix milliseconds UTC
	UpdatedAt int64   `gorm:"column:updated_at" json:"updated_at"` // Unix milliseconds UTC
}

// TableName returns the table name
func (TraderPosition) TableName() string {
	return "trader_positions"
}

// PositionStore position storage
type PositionStore struct {
	db *gorm.DB
}

// NewPositionStore creates position storage instance
func NewPositionStore(db *gorm.DB) *PositionStore {
	return &PositionStore{db: db}
}

// isPostgres checks if the database is PostgreSQL
func (s *PositionStore) isPostgres() bool {
	return s.db.Dialector.Name() == "postgres"
}

// InitTables initializes position tables
func (s *PositionStore) InitTables() error {
	// For PostgreSQL with existing table, skip AutoMigrate
	if s.isPostgres() {
		var tableExists int64
		if err := s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'trader_positions'`).Scan(&tableExists).Error; err != nil {
			return err
		}
		if tableExists > 0 {
			// Migrate timestamp columns to bigint (Unix milliseconds UTC)
			// Check if column is still timestamp type before migrating
			timestampColumns := []string{"entry_time", "exit_time", "created_at", "updated_at"}
			for _, col := range timestampColumns {
				var dataType string
				if err := s.db.Raw(`SELECT data_type FROM information_schema.columns WHERE table_name = 'trader_positions' AND column_name = ?`, col).Scan(&dataType).Error; err != nil {
					return err
				}
				if dataType == "timestamp with time zone" || dataType == "timestamp without time zone" {
					// Convert timestamp to Unix milliseconds (bigint)
					if err := s.db.Exec(fmt.Sprintf(`ALTER TABLE trader_positions ALTER COLUMN %s TYPE BIGINT USING EXTRACT(EPOCH FROM %s) * 1000`, col, col)).Error; err != nil {
						return err
					}
				}
			}

			// Just ensure index exists
			if err := s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_positions_exchange_pos_unique ON trader_positions(exchange_id, exchange_position_id) WHERE exchange_position_id != ''`).Error; err != nil {
				return err
			}
			// New columns on the existing table — AutoMigrate is skipped above.
			if err := s.db.Exec(`ALTER TABLE trader_positions ADD COLUMN IF NOT EXISTS initial_stop_loss DOUBLE PRECISION DEFAULT 0`).Error; err != nil {
				return err
			}
			if err := s.db.Exec(`ALTER TABLE trader_positions ADD COLUMN IF NOT EXISTS mae_pct DOUBLE PRECISION DEFAULT 0`).Error; err != nil {
				return err
			}
			if err := s.db.Exec(`ALTER TABLE trader_positions ADD COLUMN IF NOT EXISTS mfe_pct DOUBLE PRECISION DEFAULT 0`).Error; err != nil {
				return err
			}
			if err := s.db.Exec(`ALTER TABLE trader_positions ADD COLUMN IF NOT EXISTS mae_r DOUBLE PRECISION DEFAULT 0`).Error; err != nil {
				return err
			}
			if err := s.db.Exec(`ALTER TABLE trader_positions ADD COLUMN IF NOT EXISTS mfe_r DOUBLE PRECISION DEFAULT 0`).Error; err != nil {
				return err
			}
			if err := s.db.Exec(`ALTER TABLE trader_positions ADD COLUMN IF NOT EXISTS exit_mode TEXT DEFAULT ''`).Error; err != nil {
				return err
			}
			return ensureColumns(s.db, &TraderPosition{})
		}
	}

	if err := s.db.AutoMigrate(&TraderPosition{}); err != nil {
		return fmt.Errorf("failed to migrate trader_positions table: %w", err)
	}

	// Create unique partial index for exchange position deduplication
	var indexSQL string
	if s.isPostgres() {
		indexSQL = `CREATE UNIQUE INDEX IF NOT EXISTS idx_positions_exchange_pos_unique ON trader_positions(exchange_id, exchange_position_id) WHERE exchange_position_id != ''`
	} else {
		indexSQL = `CREATE UNIQUE INDEX IF NOT EXISTS idx_positions_exchange_pos_unique ON trader_positions(exchange_id, exchange_position_id) WHERE exchange_position_id != ''`
	}
	if err := s.db.Exec(indexSQL).Error; err != nil {
		if !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("failed to create unique index: %w", err)
		}
	}

	return nil
}

// Create creates position record
func (s *PositionStore) Create(pos *TraderPosition) error {
	pos.Status = "OPEN"
	if pos.EntryQuantity == 0 {
		pos.EntryQuantity = pos.Quantity
	}
	return s.db.Create(pos).Error
}

// ClosePosition closes position
func (s *PositionStore) ClosePosition(id int64, exitPrice float64, exitOrderID string, realizedPnL float64, fee float64, closeReason string) error {
	nowMs := time.Now().UTC().UnixMilli()
	return s.db.Model(&TraderPosition{}).Where("id = ?", id).Updates(map[string]interface{}{
		"exit_price":    exitPrice,
		"exit_order_id": exitOrderID,
		"exit_time":     nowMs,
		"realized_pnl":  realizedPnL,
		"fee":           fee,
		"status":        "CLOSED",
		"close_reason":  closeReason,
		"updated_at":    nowMs,
	}).Error
}

// UpdatePositionQuantityAndPrice updates position quantity and recalculates entry price
func (s *PositionStore) UpdatePositionQuantityAndPrice(id int64, addQty float64, addPrice float64, addFee float64) error {
	var pos TraderPosition
	if err := s.db.First(&pos, id).Error; err != nil {
		return fmt.Errorf("failed to get current position: %w", err)
	}

	currentEntryQty := pos.EntryQuantity
	if currentEntryQty == 0 {
		currentEntryQty = pos.Quantity
	}

	newQty := math.Round((pos.Quantity+addQty)*10000) / 10000
	newEntryQty := math.Round((currentEntryQty+addQty)*10000) / 10000
	newEntryPrice := (pos.EntryPrice*pos.Quantity + addPrice*addQty) / newQty
	// Use adaptive precision based on price magnitude (for meme coins with very small prices)
	newEntryPrice = adaptivePriceRound(newEntryPrice, pos.EntryPrice, addPrice)
	newFee := pos.Fee + addFee
	nowMs := time.Now().UTC().UnixMilli()

	return s.db.Model(&TraderPosition{}).Where("id = ?", id).Updates(map[string]interface{}{
		"quantity":       newQty,
		"entry_quantity": newEntryQty,
		"entry_price":    newEntryPrice,
		"fee":            newFee,
		"updated_at":     nowMs,
	}).Error
}

// ReducePositionQuantity reduces position quantity for partial close
// If quantity reaches 0 (or near 0), automatically closes the position
func (s *PositionStore) ReducePositionQuantity(id int64, reduceQty float64, exitPrice float64, addFee float64, addPnL float64) error {
	var pos TraderPosition
	if err := s.db.First(&pos, id).Error; err != nil {
		return fmt.Errorf("failed to get current position: %w", err)
	}

	newQty := math.Round((pos.Quantity-reduceQty)*10000) / 10000
	newFee := pos.Fee + addFee
	newPnL := pos.RealizedPnL + addPnL

	closedQty := pos.EntryQuantity - pos.Quantity
	newClosedQty := closedQty + reduceQty

	var newExitPrice float64
	if newClosedQty > 0 {
		newExitPrice = (pos.ExitPrice*closedQty + exitPrice*reduceQty) / newClosedQty
		// Use adaptive precision based on price magnitude (for meme coins with very small prices)
		newExitPrice = adaptivePriceRound(newExitPrice, pos.ExitPrice, exitPrice, pos.EntryPrice)
	}

	nowMs := time.Now().UTC().UnixMilli()

	// Check if position should be fully closed (quantity reduced to ~0)
	const QUANTITY_TOLERANCE = 0.0001
	if newQty <= QUANTITY_TOLERANCE {
		// Auto-close: set status to CLOSED
		return s.db.Model(&TraderPosition{}).Where("id = ?", id).Updates(map[string]interface{}{
			"quantity":     0,
			"fee":          newFee,
			"exit_price":   newExitPrice,
			"realized_pnl": newPnL,
			"status":       "CLOSED",
			"exit_time":    nowMs,
			"close_reason": "sync",
			"updated_at":   nowMs,
		}).Error
	}

	return s.db.Model(&TraderPosition{}).Where("id = ?", id).Updates(map[string]interface{}{
		"quantity":     newQty,
		"fee":          newFee,
		"exit_price":   newExitPrice,
		"realized_pnl": newPnL,
		"updated_at":   nowMs,
	}).Error
}

// UpdatePositionExchangeInfo updates exchange_id and exchange_type
func (s *PositionStore) UpdatePositionExchangeInfo(id int64, exchangeID, exchangeType string) error {
	nowMs := time.Now().UTC().UnixMilli()
	return s.db.Model(&TraderPosition{}).Where("id = ?", id).Updates(map[string]interface{}{
		"exchange_id":   exchangeID,
		"exchange_type": exchangeType,
		"updated_at":    nowMs,
	}).Error
}

// UpdatePositionLeverage backfills the real exchange leverage onto an OPEN
// row. The sync path (PositionBuilder) has always hardcoded 1 — binance
// orders ride OrderSync, whose fills carry no leverage — so every row landed
// as 1x while the real position ran 3-5x, silently inflating the journal's
// margin/ROI calibers and poisoning every statistic derived from it
// (QUANT_REVIEW_2026-09-22 D3: 185/185 live rows were leverage=1).
func (s *PositionStore) UpdatePositionLeverage(id int64, leverage int) error {
	if leverage <= 0 {
		return fmt.Errorf("refusing to backfill leverage %d on position #%d", leverage, id)
	}
	nowMs := time.Now().UTC().UnixMilli()
	return s.db.Model(&TraderPosition{}).Where("id = ?", id).Updates(map[string]interface{}{
		"leverage":   leverage,
		"updated_at": nowMs,
	}).Error
}

// UpdateCloseReason re-labels a closed row's exit path. The fill sync stamps
// every close 'sync' (SL/TP triggers, AI closes and manual closes all land in
// the same fills); the trader's per-cycle classifier narrows it afterwards
// using state only it holds (recorded SL/TP, close intents).
func (s *PositionStore) UpdateCloseReason(id int64, reason string) error {
	if reason == "" {
		return nil
	}
	nowMs := time.Now().UTC().UnixMilli()
	return s.db.Model(&TraderPosition{}).Where("id = ?", id).Updates(map[string]interface{}{
		"close_reason": reason,
		"updated_at":   nowMs,
	}).Error
}

// UpdateExitExcursions persists the MAE/MFE replay outcome for one closed row.
func (s *PositionStore) UpdateExitExcursions(id int64, maePct, mfePct, maeR, mfeR float64) error {
	nowMs := time.Now().UTC().UnixMilli()
	return s.db.Model(&TraderPosition{}).Where("id = ?", id).Updates(map[string]interface{}{
		"mae_pct":    maePct,
		"mfe_pct":    mfePct,
		"mae_r":      maeR,
		"mfe_r":      mfeR,
		"updated_at": nowMs,
	}).Error
}

// GetRecentSyncClosed returns this trader's rows closed within the window that
// still carry the raw 'sync' stamp — the classifier's work queue.
func (s *PositionStore) GetRecentSyncClosed(traderID string, sinceMs int64, limit int) ([]TraderPosition, error) {
	var rows []TraderPosition
	err := s.db.Where(
		"trader_id = ? AND status = ? AND close_reason = ? AND exit_time >= ?",
		traderID, "CLOSED", "sync", sinceMs,
	).Order("exit_time DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// GetClosedMissingExcursions returns closed rows (newest first) that have no
// excursion replay yet — the MAE/MFE backfill's work queue. Rows where the
// replay already ran but both excursions are genuinely 0 re-qualify; the
// replay is idempotent, the cost is an occasional duplicate API call.
func (s *PositionStore) GetClosedMissingExcursions(traderID string, limit int) ([]TraderPosition, error) {
	var rows []TraderPosition
	err := s.db.Where(
		"trader_id = ? AND status = ? AND exit_time > 0 AND exit_time > entry_time AND mae_pct = 0",
		traderID, "CLOSED",
	).Order("exit_time DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// ClosePositionFully marks position as fully closed
// exitTimeMs is Unix milliseconds UTC.
//
// addPnL/addFee are the DELTA of the closing leg only — they are added to the
// row's stored accumulation in SQL. 2026-10-07 review N1: callers used to pass
// the row's running total (stored + this leg) while this function also added
// the stored value, so every close doubled the entry fee and every earlier
// partial-close PnL. Pass 0/0 when the row's accumulation is already final.
func (s *PositionStore) ClosePositionFully(id int64, exitPrice float64, exitOrderID string, exitTimeMs int64, addPnL float64, addFee float64, closeReason string) error {
	var pos TraderPosition
	if err := s.db.First(&pos, id).Error; err != nil {
		return fmt.Errorf("failed to get position: %w", err)
	}

	quantity := pos.Quantity
	if pos.EntryQuantity > 0 {
		quantity = pos.EntryQuantity
	}

	// 2026-10-03 review P1: accumulate PnL/fee via SQL — the read-then-
	// absolute-write let a concurrent close leg (sync × orphan reconcile
	// landing between the First and the Updates) overwrite the base and lose
	// one leg's PnL/fee entirely.
	return s.db.Model(&TraderPosition{}).Where("id = ?", id).Updates(map[string]interface{}{
		"quantity":      quantity,
		"exit_price":    exitPrice,
		"exit_order_id": exitOrderID,
		"exit_time":     exitTimeMs,
		"realized_pnl":  gorm.Expr("realized_pnl + ?", addPnL),
		"fee":           gorm.Expr("fee + ?", addFee),
		"status":        "CLOSED",
		"close_reason":  closeReason,
		"updated_at":    time.Now().UTC().UnixMilli(),
	}).Error
}

// DeleteAllOpenPositions deletes all OPEN positions for a trader
func (s *PositionStore) DeleteAllOpenPositions(traderID string) error {
	return s.db.Where("trader_id = ? AND status = ?", traderID, "OPEN").Delete(&TraderPosition{}).Error
}

// GetOpenSymbolsByExchange discovers unsynced closes even when the exchange is flat.
// review 2026-10-07 B2-C: include the account's durable OPEN rows in sync discovery.
func (s *PositionStore) GetOpenSymbolsByExchange(exchangeID string) ([]string, error) {
	var symbols []string
	err := s.db.Model(&TraderPosition{}).Where("exchange_id = ? AND status = ?", exchangeID, "OPEN").Distinct().Pluck("symbol", &symbols).Error
	return symbols, err
}

// GetOpenPositions gets all open positions
func (s *PositionStore) GetOpenPositions(traderID string) ([]*TraderPosition, error) {
	var positions []*TraderPosition
	err := s.db.Where("trader_id = ? AND status = ?", traderID, "OPEN").
		Order("entry_time DESC").
		Find(&positions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query open positions: %w", err)
	}

	// Fix EntryQuantity if it's 0
	for _, pos := range positions {
		if pos.EntryQuantity == 0 {
			pos.EntryQuantity = pos.Quantity
		}
	}
	return positions, nil
}

// GetOpenPositionBySymbol gets open position for specified symbol and direction
func (s *PositionStore) GetOpenPositionBySymbol(traderID, symbol, side string) (*TraderPosition, error) {
	var pos TraderPosition
	err := s.db.Where("trader_id = ? AND symbol = ? AND side = ? AND status = ?", traderID, symbol, side, "OPEN").
		Order("entry_time DESC").
		First(&pos).Error

	if err == nil {
		if pos.EntryQuantity == 0 {
			pos.EntryQuantity = pos.Quantity
		}
		return &pos, nil
	}

	if err == gorm.ErrRecordNotFound {
		// Try without USDT suffix for backward compatibility
		if strings.HasSuffix(symbol, "USDT") {
			baseSymbol := strings.TrimSuffix(symbol, "USDT")
			err = s.db.Where("trader_id = ? AND symbol = ? AND side = ? AND status = ?", traderID, baseSymbol, side, "OPEN").
				Order("entry_time DESC").
				First(&pos).Error
			if err == nil {
				if pos.EntryQuantity == 0 {
					pos.EntryQuantity = pos.Quantity
				}
				return &pos, nil
			}
		}
		return nil, nil
	}
	return nil, err
}

// GetPositionForFill returns the trader's row on (symbol, side) that a fill at
// fillTimeMs belongs to: the most recently opened row that existed at the fill,
// i.e. opened at or before it and still OPEN or closed at or after it (a closing
// fill stamps exit_time with its own time). nil when there is none. Like
// GetOpenPositionBySymbol it falls back to the base symbol for legacy rows.
func (s *PositionStore) GetPositionForFill(traderID, symbol, side string, fillTimeMs int64) (*TraderPosition, error) {
	symbols := []string{symbol}
	if base := strings.TrimSuffix(symbol, "USDT"); base != symbol {
		symbols = append(symbols, base)
	}
	for _, sym := range symbols {
		var pos TraderPosition
		err := s.db.Where("trader_id = ? AND symbol = ? AND side = ? AND entry_time <= ? AND (status = ? OR exit_time >= ?)",
			traderID, sym, side, fillTimeMs, "OPEN", fillTimeMs).
			Order("entry_time DESC").
			First(&pos).Error
		if err == gorm.ErrRecordNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		if pos.EntryQuantity == 0 {
			pos.EntryQuantity = pos.Quantity
		}
		return &pos, nil
	}
	return nil, nil
}

// GetClosedPositions gets closed positions
func (s *PositionStore) GetClosedPositions(traderID string, limit int) ([]*TraderPosition, error) {
	var positions []*TraderPosition
	err := s.db.Where("trader_id = ? AND status = ?", traderID, "CLOSED").
		Order("exit_time DESC").
		Limit(limit).
		Find(&positions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query closed positions: %w", err)
	}

	for _, pos := range positions {
		if pos.EntryQuantity == 0 {
			pos.EntryQuantity = pos.Quantity
		}
	}
	return positions, nil
}

// GetAllOpenPositions gets all traders' open positions
func (s *PositionStore) GetAllOpenPositions() ([]*TraderPosition, error) {
	var positions []*TraderPosition
	err := s.db.Where("status = ?", "OPEN").
		Order("trader_id, entry_time DESC").
		Find(&positions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query all open positions: %w", err)
	}

	for _, pos := range positions {
		if pos.EntryQuantity == 0 {
			pos.EntryQuantity = pos.Quantity
		}
	}
	return positions, nil
}

// ExistsWithExchangePositionID checks if a position exists
func (s *PositionStore) ExistsWithExchangePositionID(exchangeID, exchangePositionID string) (bool, error) {
	if exchangePositionID == "" {
		return false, nil
	}

	var count int64
	err := s.db.Model(&TraderPosition{}).
		Where("exchange_id = ? AND exchange_position_id = ?", exchangeID, exchangePositionID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check position existence: %w", err)
	}
	return count > 0, nil
}

// GetOpenPositionByExchangePositionID gets an OPEN position by exchange_position_id
func (s *PositionStore) GetOpenPositionByExchangePositionID(exchangeID, exchangePositionID string) (*TraderPosition, error) {
	if exchangePositionID == "" {
		return nil, nil
	}

	var pos TraderPosition
	err := s.db.Where("exchange_id = ? AND exchange_position_id = ? AND status = ?", exchangeID, exchangePositionID, "OPEN").
		First(&pos).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	if pos.EntryQuantity == 0 {
		pos.EntryQuantity = pos.Quantity
	}
	return &pos, nil
}

// CreateOpenPosition creates an open position
func (s *PositionStore) CreateOpenPosition(pos *TraderPosition) error {
	if pos.ExchangePositionID != "" && pos.ExchangeID != "" {
		existingPos, err := s.GetOpenPositionByExchangePositionID(pos.ExchangeID, pos.ExchangePositionID)
		if err != nil {
			return err
		}
		if existingPos != nil {
			return s.UpdatePositionQuantityAndPrice(existingPos.ID, pos.Quantity, pos.EntryPrice, pos.Fee)
		}
		exists, err := s.ExistsWithExchangePositionID(pos.ExchangeID, pos.ExchangePositionID)
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
	}

	if pos.Status == "" {
		pos.Status = "OPEN"
	}
	if pos.Source == "" {
		pos.Source = "system"
	}
	if pos.EntryQuantity == 0 {
		pos.EntryQuantity = pos.Quantity
	}

	err := s.db.Create(pos).Error
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			existingPos, findErr := s.GetOpenPositionByExchangePositionID(pos.ExchangeID, pos.ExchangePositionID)
			if findErr != nil {
				return findErr
			}
			if existingPos != nil {
				return s.UpdatePositionQuantityAndPrice(existingPos.ID, pos.Quantity, pos.EntryPrice, pos.Fee)
			}
			return nil
		}
		return fmt.Errorf("failed to create open position: %w", err)
	}

	return nil
}

// SetInitialStopLossIfEmpty records the opening-risk stop on the trader's OPEN
// row for (symbol, side), first write only — a fixed anchor the 1R profit lock
// can measure against across stop adjustments and restarts. side accepts
// either case. Returns whether a row was stamped.
func (s *PositionStore) SetInitialStopLossIfEmpty(traderID, symbol, side string, sl float64) (bool, error) {
	if sl <= 0 {
		return false, nil
	}
	res := s.db.Model(&TraderPosition{}).
		Where("trader_id = ? AND symbol = ? AND UPPER(side) = ? AND status = ? AND initial_stop_loss = 0",
			traderID, symbol, strings.ToUpper(side), "OPEN").
		Update("initial_stop_loss", sl)
	return res.RowsAffected > 0, res.Error
}

// MarkR1TrimDone persists the 1R profit-lock trim marker on the trader's OPEN
// row for (symbol, side) — the in-memory r1TrimDone map dies with the process,
// and without this a restart while the position still sits ≥1R re-fired the
// 50% reduction on the remainder (09-28 review P1). Idempotent.
func (s *PositionStore) MarkR1TrimDone(traderID, symbol, side string) error {
	return s.db.Model(&TraderPosition{}).
		Where("trader_id = ? AND symbol = ? AND UPPER(side) = ? AND status = ?",
			traderID, symbol, strings.ToUpper(side), "OPEN").
		Update("r1_trim_done", true).Error
}

// MarkTPTrimDone persists the ROE-ladder 1/3 trim marker (same restart
// re-fire class as MarkR1TrimDone — tpTrimDone is memory-only).
func (s *PositionStore) MarkTPTrimDone(traderID, symbol, side string) error {
	return s.db.Model(&TraderPosition{}).
		Where("trader_id = ? AND symbol = ? AND UPPER(side) = ? AND status = ?",
			traderID, symbol, strings.ToUpper(side), "OPEN").
		Update("tp_trim_done", true).Error
}

// SetExitModeIfEmpty stamps the position's exit template on the trader's OPEN
// row (write-once at open) — the exit ladder and the quick-mode time stop read
// it back after restarts. Empty mode is a no-op ('trend' is the default).
func (s *PositionStore) SetExitModeIfEmpty(traderID, symbol, side, mode string) error {
	if mode == "" {
		return nil
	}
	return s.db.Model(&TraderPosition{}).
		Where("trader_id = ? AND symbol = ? AND UPPER(side) = ? AND status = ? AND (exit_mode = '' OR exit_mode IS NULL)",
			traderID, symbol, strings.ToUpper(side), "OPEN").
		Update("exit_mode", mode).Error
}

// SetAIManaged stamps the AI-book attribution on one row by id (close-time
// re-check in the position builder: the registry mark can still exist when
// OrderSync closes a row the open-time stamp missed, e.g. the one-time
// seedAIManagedOnce migration's pre-existing positions).
func (s *PositionStore) SetAIManaged(id int64) error {
	return s.db.Model(&TraderPosition{}).Where("id = ?", id).Update("ai_managed", true).Error
}

// ClosePositionWithAccurateData closes a position with accurate data from exchange
// exitTimeMs is Unix milliseconds UTC
func (s *PositionStore) ClosePositionWithAccurateData(id int64, exitPrice float64, exitOrderID string, exitTimeMs int64, realizedPnL float64, fee float64, closeReason string) error {
	return s.db.Model(&TraderPosition{}).Where("id = ?", id).Updates(map[string]interface{}{
		"exit_price":    exitPrice,
		"exit_order_id": exitOrderID,
		"exit_time":     exitTimeMs,
		"realized_pnl":  realizedPnL,
		"fee":           fee,
		"status":        "CLOSED",
		"close_reason":  closeReason,
		"updated_at":    time.Now().UTC().UnixMilli(),
	}).Error
}
