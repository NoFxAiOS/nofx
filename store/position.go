package store

import (
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
)

// TraderStats trading statistics metrics
type TraderStats struct {
	TotalTrades    int     `json:"total_trades"`
	WinTrades      int     `json:"win_trades"`
	LossTrades     int     `json:"loss_trades"`
	WinRate        float64 `json:"win_rate"`
	ProfitFactor   float64 `json:"profit_factor"`
	SharpeRatio    float64 `json:"sharpe_ratio"`
	TotalPnL       float64 `json:"total_pnl"`
	TotalFee       float64 `json:"total_fee"`
	AvgWin         float64 `json:"avg_win"`
	AvgLoss        float64 `json:"avg_loss"`
	MaxDrawdownPct float64 `json:"max_drawdown_pct"`
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
	CreatedAt          int64   `gorm:"column:created_at" json:"created_at"`   // Unix milliseconds UTC
	UpdatedAt          int64   `gorm:"column:updated_at" json:"updated_at"`   // Unix milliseconds UTC
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
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'trader_positions'`).Scan(&tableExists)
		if tableExists > 0 {
			// Migrate timestamp columns to bigint (Unix milliseconds UTC)
			// Check if column is still timestamp type before migrating
			timestampColumns := []string{"entry_time", "exit_time", "created_at", "updated_at"}
			for _, col := range timestampColumns {
				var dataType string
				s.db.Raw(`SELECT data_type FROM information_schema.columns WHERE table_name = 'trader_positions' AND column_name = ?`, col).Scan(&dataType)
				if dataType == "timestamp with time zone" || dataType == "timestamp without time zone" {
					// Convert timestamp to Unix milliseconds (bigint)
					s.db.Exec(fmt.Sprintf(`ALTER TABLE trader_positions ALTER COLUMN %s TYPE BIGINT USING EXTRACT(EPOCH FROM %s) * 1000`, col, col))
				}
			}

			// Just ensure index exists
			s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_positions_exchange_pos_unique ON trader_positions(exchange_id, exchange_position_id) WHERE exchange_position_id != ''`)
			return nil
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
		"exit_price":   exitPrice,
		"exit_order_id": exitOrderID,
		"exit_time":    nowMs,
		"realized_pnl": realizedPnL,
		"fee":          fee,
		"status":       "CLOSED",
		"close_reason": closeReason,
		"updated_at":   nowMs,
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
	// Use 8 decimal places for price precision (crypto standard)
	newEntryPrice = math.Round(newEntryPrice*100000000) / 100000000
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
		// Use 8 decimal places for price precision (crypto standard)
		newExitPrice = math.Round(newExitPrice*100000000) / 100000000
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

// ClosePositionFully marks position as fully closed
// exitTimeMs is Unix milliseconds UTC
func (s *PositionStore) ClosePositionFully(id int64, exitPrice float64, exitOrderID string, exitTimeMs int64, totalRealizedPnL float64, totalFee float64, closeReason string) error {
	var pos TraderPosition
	if err := s.db.First(&pos, id).Error; err != nil {
		return fmt.Errorf("failed to get position: %w", err)
	}

	quantity := pos.Quantity
	if pos.EntryQuantity > 0 {
		quantity = pos.EntryQuantity
	}

	return s.db.Model(&TraderPosition{}).Where("id = ?", id).Updates(map[string]interface{}{
		"quantity":       quantity,
		"exit_price":     exitPrice,
		"exit_order_id":  exitOrderID,
		"exit_time":      exitTimeMs,
		"realized_pnl":   totalRealizedPnL,
		"fee":            totalFee,
		"status":         "CLOSED",
		"close_reason":   closeReason,
		"updated_at":     time.Now().UTC().UnixMilli(),
	}).Error
}

// DeleteAllOpenPositions deletes all OPEN positions for a trader
func (s *PositionStore) DeleteAllOpenPositions(traderID string) error {
	return s.db.Where("trader_id = ? AND status = ?", traderID, "OPEN").Delete(&TraderPosition{}).Error
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

// GetPositionStats gets position statistics
func (s *PositionStore) GetPositionStats(traderID string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	type result struct {
		Total    int
		Wins     int
		TotalPnL float64
		TotalFee float64
	}
	var r result

	err := s.db.Model(&TraderPosition{}).
		Select("COUNT(*) as total, SUM(CASE WHEN realized_pnl > 0 THEN 1 ELSE 0 END) as wins, COALESCE(SUM(realized_pnl), 0) as total_pnl, COALESCE(SUM(fee), 0) as total_fee").
		Where("trader_id = ? AND status = ?", traderID, "CLOSED").
		Scan(&r).Error
	if err != nil {
		return nil, err
	}

	stats["total_trades"] = r.Total
	stats["win_trades"] = r.Wins
	stats["total_pnl"] = r.TotalPnL
	stats["total_fee"] = r.TotalFee
	if r.Total > 0 {
		stats["win_rate"] = float64(r.Wins) / float64(r.Total) * 100
	} else {
		stats["win_rate"] = 0.0
	}

	return stats, nil
}

// GetFullStats gets complete trading statistics
func (s *PositionStore) GetFullStats(traderID string) (*TraderStats, error) {
	stats := &TraderStats{}

	var count int64
	if err := s.db.Model(&TraderPosition{}).Where("trader_id = ? AND status = ?", traderID, "CLOSED").Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return stats, nil
	}

	var positions []TraderPosition
	err := s.db.Where("trader_id = ? AND status = ?", traderID, "CLOSED").
		Order("exit_time ASC").
		Find(&positions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query position statistics: %w", err)
	}

	var pnls []float64
	var totalWin, totalLoss float64

	for _, pos := range positions {
		stats.TotalTrades++
		stats.TotalPnL += pos.RealizedPnL
		stats.TotalFee += pos.Fee
		pnls = append(pnls, pos.RealizedPnL)

		if pos.RealizedPnL > 0 {
			stats.WinTrades++
			totalWin += pos.RealizedPnL
		} else if pos.RealizedPnL < 0 {
			stats.LossTrades++
			totalLoss += -pos.RealizedPnL
		}
	}

	if stats.TotalTrades > 0 {
		stats.WinRate = float64(stats.WinTrades) / float64(stats.TotalTrades) * 100
	}
	if totalLoss > 0 {
		stats.ProfitFactor = totalWin / totalLoss
	}
	if stats.WinTrades > 0 {
		stats.AvgWin = totalWin / float64(stats.WinTrades)
	}
	if stats.LossTrades > 0 {
		stats.AvgLoss = totalLoss / float64(stats.LossTrades)
	}
	if len(pnls) > 1 {
		stats.SharpeRatio = calculateSharpeRatioFromPnls(pnls)
	}
	if len(pnls) > 0 {
		stats.MaxDrawdownPct = calculateMaxDrawdownFromPnls(pnls)
	}

	return stats, nil
}

// RecentTrade recent trade record
type RecentTrade struct {
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`
	EntryPrice   float64 `json:"entry_price"`
	ExitPrice    float64 `json:"exit_price"`
	RealizedPnL  float64 `json:"realized_pnl"`
	PnLPct       float64 `json:"pnl_pct"`
	EntryTime    int64   `json:"entry_time"`
	ExitTime     int64   `json:"exit_time"`
	HoldDuration string  `json:"hold_duration"`
}

// GetRecentTrades gets recent closed trades
func (s *PositionStore) GetRecentTrades(traderID string, limit int) ([]RecentTrade, error) {
	var positions []TraderPosition
	err := s.db.Where("trader_id = ? AND status = ?", traderID, "CLOSED").
		Order("exit_time DESC").
		Limit(limit).
		Find(&positions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query recent trades: %w", err)
	}

	var trades []RecentTrade
	for _, pos := range positions {
		t := RecentTrade{
			Symbol:      pos.Symbol,
			Side:        strings.ToLower(pos.Side),
			EntryPrice:  pos.EntryPrice,
			ExitPrice:   pos.ExitPrice,
			RealizedPnL: pos.RealizedPnL,
			EntryTime:   pos.EntryTime / 1000, // Convert ms to seconds for API compatibility
		}

		if pos.ExitTime > 0 {
			t.ExitTime = pos.ExitTime / 1000 // Convert ms to seconds
			durationMs := pos.ExitTime - pos.EntryTime
			t.HoldDuration = formatDurationMs(durationMs)
		}

		if pos.EntryPrice > 0 {
			if t.Side == "long" {
				t.PnLPct = (pos.ExitPrice - pos.EntryPrice) / pos.EntryPrice * 100 * float64(pos.Leverage)
			} else {
				t.PnLPct = (pos.EntryPrice - pos.ExitPrice) / pos.EntryPrice * 100 * float64(pos.Leverage)
			}
		}

		trades = append(trades, t)
	}

	return trades, nil
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

// calculateSharpeRatioFromPnls calculates Sharpe ratio
func calculateSharpeRatioFromPnls(pnls []float64) float64 {
	if len(pnls) < 2 {
		return 0
	}

	var sum float64
	for _, pnl := range pnls {
		sum += pnl
	}
	mean := sum / float64(len(pnls))

	var variance float64
	for _, pnl := range pnls {
		variance += (pnl - mean) * (pnl - mean)
	}
	stdDev := math.Sqrt(variance / float64(len(pnls)-1))

	if stdDev == 0 {
		return 0
	}

	return mean / stdDev
}

// calculateMaxDrawdownFromPnls calculates maximum drawdown
func calculateMaxDrawdownFromPnls(pnls []float64) float64 {
	if len(pnls) == 0 {
		return 0
	}

	const startingEquity = 10000.0
	equity := startingEquity
	peak := startingEquity
	var maxDD float64

	for _, pnl := range pnls {
		equity += pnl
		if equity > peak {
			peak = equity
		}
		if peak > 0 {
			dd := (peak - equity) / peak * 100
			if dd > maxDD {
				maxDD = dd
			}
		}
	}

	return maxDD
}

// SymbolStats per-symbol trading statistics
type SymbolStats struct {
	Symbol      string  `json:"symbol"`
	TotalTrades int     `json:"total_trades"`
	WinTrades   int     `json:"win_trades"`
	WinRate     float64 `json:"win_rate"`
	TotalPnL    float64 `json:"total_pnl"`
	AvgPnL      float64 `json:"avg_pnl"`
	AvgHoldMins float64 `json:"avg_hold_mins"`
}

// GetSymbolStats gets per-symbol trading statistics
func (s *PositionStore) GetSymbolStats(traderID string, limit int) ([]SymbolStats, error) {
	var positions []TraderPosition
	err := s.db.Where("trader_id = ? AND status = ?", traderID, "CLOSED").Find(&positions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query symbol stats: %w", err)
	}

	// Group by symbol
	symbolMap := make(map[string]*SymbolStats)
	symbolHoldMins := make(map[string][]float64)

	for _, pos := range positions {
		if _, ok := symbolMap[pos.Symbol]; !ok {
			symbolMap[pos.Symbol] = &SymbolStats{Symbol: pos.Symbol}
			symbolHoldMins[pos.Symbol] = []float64{}
		}
		s := symbolMap[pos.Symbol]
		s.TotalTrades++
		s.TotalPnL += pos.RealizedPnL
		if pos.RealizedPnL > 0 {
			s.WinTrades++
		}

		if pos.ExitTime > 0 {
			holdMins := float64(pos.ExitTime-pos.EntryTime) / 60000.0 // ms to minutes
			symbolHoldMins[pos.Symbol] = append(symbolHoldMins[pos.Symbol], holdMins)
		}
	}

	var stats []SymbolStats
	for symbol, s := range symbolMap {
		if s.TotalTrades > 0 {
			s.WinRate = float64(s.WinTrades) / float64(s.TotalTrades) * 100
			s.AvgPnL = s.TotalPnL / float64(s.TotalTrades)
		}
		if len(symbolHoldMins[symbol]) > 0 {
			var totalMins float64
			for _, m := range symbolHoldMins[symbol] {
				totalMins += m
			}
			s.AvgHoldMins = totalMins / float64(len(symbolHoldMins[symbol]))
		}
		stats = append(stats, *s)
	}

	// Sort by TotalPnL descending and limit
	for i := 0; i < len(stats)-1; i++ {
		for j := i + 1; j < len(stats); j++ {
			if stats[j].TotalPnL > stats[i].TotalPnL {
				stats[i], stats[j] = stats[j], stats[i]
			}
		}
	}

	if limit > 0 && len(stats) > limit {
		stats = stats[:limit]
	}

	return stats, nil
}

// HoldingTimeStats holding duration analysis
type HoldingTimeStats struct {
	Range      string  `json:"range"`
	TradeCount int     `json:"trade_count"`
	WinRate    float64 `json:"win_rate"`
	AvgPnL     float64 `json:"avg_pnl"`
}

// GetHoldingTimeStats analyzes performance by holding duration
func (s *PositionStore) GetHoldingTimeStats(traderID string) ([]HoldingTimeStats, error) {
	var positions []TraderPosition
	err := s.db.Where("trader_id = ? AND status = ? AND exit_time > 0", traderID, "CLOSED").Find(&positions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query holding time stats: %w", err)
	}

	rangeStats := map[string]*struct {
		count   int
		wins    int
		totalPnL float64
	}{
		"<1h":   {},
		"1-4h":  {},
		"4-24h": {},
		">24h":  {},
	}

	for _, pos := range positions {
		if pos.ExitTime == 0 {
			continue
		}
		holdHours := float64(pos.ExitTime-pos.EntryTime) / 3600000.0 // ms to hours

		var rangeKey string
		switch {
		case holdHours < 1:
			rangeKey = "<1h"
		case holdHours < 4:
			rangeKey = "1-4h"
		case holdHours < 24:
			rangeKey = "4-24h"
		default:
			rangeKey = ">24h"
		}

		r := rangeStats[rangeKey]
		r.count++
		r.totalPnL += pos.RealizedPnL
		if pos.RealizedPnL > 0 {
			r.wins++
		}
	}

	var stats []HoldingTimeStats
	for _, rangeKey := range []string{"<1h", "1-4h", "4-24h", ">24h"} {
		r := rangeStats[rangeKey]
		if r.count > 0 {
			stats = append(stats, HoldingTimeStats{
				Range:      rangeKey,
				TradeCount: r.count,
				WinRate:    float64(r.wins) / float64(r.count) * 100,
				AvgPnL:     r.totalPnL / float64(r.count),
			})
		}
	}

	return stats, nil
}

// DirectionStats long/short performance comparison
type DirectionStats struct {
	Side       string  `json:"side"`
	TradeCount int     `json:"trade_count"`
	WinRate    float64 `json:"win_rate"`
	TotalPnL   float64 `json:"total_pnl"`
	AvgPnL     float64 `json:"avg_pnl"`
}

// GetDirectionStats analyzes long vs short performance
func (s *PositionStore) GetDirectionStats(traderID string) ([]DirectionStats, error) {
	var positions []TraderPosition
	err := s.db.Where("trader_id = ? AND status = ?", traderID, "CLOSED").Find(&positions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query direction stats: %w", err)
	}

	sideStats := make(map[string]*DirectionStats)
	for _, pos := range positions {
		if _, ok := sideStats[pos.Side]; !ok {
			sideStats[pos.Side] = &DirectionStats{Side: pos.Side}
		}
		s := sideStats[pos.Side]
		s.TradeCount++
		s.TotalPnL += pos.RealizedPnL
		if pos.RealizedPnL > 0 {
			s.WinRate++
		}
	}

	var stats []DirectionStats
	for _, s := range sideStats {
		if s.TradeCount > 0 {
			s.AvgPnL = s.TotalPnL / float64(s.TradeCount)
			s.WinRate = s.WinRate / float64(s.TradeCount) * 100
		}
		stats = append(stats, *s)
	}

	return stats, nil
}

// HistorySummary comprehensive trading history for AI context
type HistorySummary struct {
	TotalTrades    int     `json:"total_trades"`
	WinRate        float64 `json:"win_rate"`
	TotalPnL       float64 `json:"total_pnl"`
	AvgTradeReturn float64 `json:"avg_trade_return"`

	BestSymbols  []SymbolStats `json:"best_symbols"`
	WorstSymbols []SymbolStats `json:"worst_symbols"`

	LongWinRate  float64 `json:"long_win_rate"`
	ShortWinRate float64 `json:"short_win_rate"`
	LongPnL      float64 `json:"long_pnl"`
	ShortPnL     float64 `json:"short_pnl"`

	AvgHoldingMins float64 `json:"avg_holding_mins"`
	BestHoldRange  string  `json:"best_hold_range"`

	RecentWinRate float64 `json:"recent_win_rate"`
	RecentPnL     float64 `json:"recent_pnl"`

	CurrentStreak int `json:"current_streak"`
	MaxWinStreak  int `json:"max_win_streak"`
	MaxLoseStreak int `json:"max_lose_streak"`
}

// GetHistorySummary generates comprehensive AI context summary
func (s *PositionStore) GetHistorySummary(traderID string) (*HistorySummary, error) {
	summary := &HistorySummary{}

	fullStats, err := s.GetFullStats(traderID)
	if err != nil {
		return nil, err
	}
	summary.TotalTrades = fullStats.TotalTrades
	summary.WinRate = fullStats.WinRate
	summary.TotalPnL = fullStats.TotalPnL
	if fullStats.TotalTrades > 0 {
		summary.AvgTradeReturn = fullStats.TotalPnL / float64(fullStats.TotalTrades)
	}

	symbolStats, _ := s.GetSymbolStats(traderID, 20)
	if len(symbolStats) > 0 {
		for i := 0; i < len(symbolStats) && i < 3; i++ {
			if symbolStats[i].TotalPnL > 0 {
				summary.BestSymbols = append(summary.BestSymbols, symbolStats[i])
			}
		}
		for i := len(symbolStats) - 1; i >= 0 && len(summary.WorstSymbols) < 3; i-- {
			if symbolStats[i].TotalPnL < 0 {
				summary.WorstSymbols = append(summary.WorstSymbols, symbolStats[i])
			}
		}
	}

	dirStats, _ := s.GetDirectionStats(traderID)
	for _, d := range dirStats {
		if d.Side == "LONG" {
			summary.LongWinRate = d.WinRate
			summary.LongPnL = d.TotalPnL
		} else if d.Side == "SHORT" {
			summary.ShortWinRate = d.WinRate
			summary.ShortPnL = d.TotalPnL
		}
	}

	holdStats, _ := s.GetHoldingTimeStats(traderID)
	var bestHoldWinRate float64
	for _, h := range holdStats {
		if h.WinRate > bestHoldWinRate && h.TradeCount >= 3 {
			bestHoldWinRate = h.WinRate
			summary.BestHoldRange = h.Range
		}
	}

	// Calculate average holding time
	var positions []TraderPosition
	s.db.Where("trader_id = ? AND status = ? AND exit_time > 0", traderID, "CLOSED").Find(&positions)
	if len(positions) > 0 {
		var totalMins float64
		for _, pos := range positions {
			if pos.ExitTime > 0 {
				totalMins += float64(pos.ExitTime-pos.EntryTime) / 60000.0 // ms to minutes
			}
		}
		summary.AvgHoldingMins = totalMins / float64(len(positions))
	}

	// Recent 20 trades
	var recent []TraderPosition
	s.db.Where("trader_id = ? AND status = ?", traderID, "CLOSED").
		Order("exit_time DESC").Limit(20).Find(&recent)
	for _, pos := range recent {
		summary.RecentPnL += pos.RealizedPnL
		if pos.RealizedPnL > 0 {
			summary.RecentWinRate++
		}
	}
	if len(recent) > 0 {
		summary.RecentWinRate = summary.RecentWinRate / float64(len(recent)) * 100
	}

	// Calculate streaks
	s.calculateStreaks(traderID, summary)

	return summary, nil
}

// calculateStreaks calculates win/loss streaks
func (s *PositionStore) calculateStreaks(traderID string, summary *HistorySummary) {
	var positions []TraderPosition
	err := s.db.Where("trader_id = ? AND status = ?", traderID, "CLOSED").
		Order("exit_time DESC").
		Find(&positions).Error
	if err != nil || len(positions) == 0 {
		return
	}

	var currentStreak, maxWin, maxLose int
	var prevWin *bool
	isFirst := true

	for _, pos := range positions {
		isWin := pos.RealizedPnL > 0

		if isFirst {
			if isWin {
				currentStreak = 1
			} else {
				currentStreak = -1
			}
			isFirst = false
		}

		if prevWin == nil {
			prevWin = &isWin
		} else if *prevWin == isWin {
			if isWin {
				currentStreak++
				if currentStreak > maxWin {
					maxWin = currentStreak
				}
			} else {
				currentStreak--
				if -currentStreak > maxLose {
					maxLose = -currentStreak
				}
			}
		} else {
			if isWin {
				currentStreak = 1
			} else {
				currentStreak = -1
			}
			*prevWin = isWin
		}
	}

	summary.CurrentStreak = currentStreak
	summary.MaxWinStreak = maxWin
	summary.MaxLoseStreak = maxLose
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

// ClosedPnLRecord represents a closed position record from exchange
// All time fields use int64 millisecond timestamps (UTC)
type ClosedPnLRecord struct {
	Symbol      string
	Side        string
	EntryPrice  float64
	ExitPrice   float64
	Quantity    float64
	RealizedPnL float64
	Fee         float64
	Leverage    int
	EntryTime   int64 // Unix milliseconds UTC
	ExitTime    int64 // Unix milliseconds UTC
	OrderID     string
	CloseType   string
	ExchangeID  string
}

// CreateFromClosedPnL creates a closed position record from exchange data
func (s *PositionStore) CreateFromClosedPnL(traderID, exchangeID, exchangeType string, record *ClosedPnLRecord) (bool, error) {
	if record.Symbol == "" {
		return false, nil
	}

	side := strings.ToUpper(record.Side)
	if side == "LONG" || side == "BUY" {
		side = "LONG"
	} else if side == "SHORT" || side == "SELL" {
		side = "SHORT"
	} else {
		return false, nil
	}

	if record.Quantity <= 0 || record.ExitPrice <= 0 || record.EntryPrice <= 0 {
		return false, nil
	}

	exchangePositionID := record.ExchangeID
	if exchangePositionID == "" {
		exchangePositionID = fmt.Sprintf("%s_%s_%d_%.8f", record.Symbol, side, record.ExitTime, record.RealizedPnL)
	}

	exists, err := s.ExistsWithExchangePositionID(exchangeID, exchangePositionID)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	exitTimeMs := record.ExitTime
	entryTimeMs := record.EntryTime

	// Validate timestamps (must be after year 2000 = ~946684800000 ms)
	minValidTime := int64(946684800000) // 2000-01-01 UTC in milliseconds
	if exitTimeMs < minValidTime {
		return false, nil
	}
	if entryTimeMs < minValidTime {
		entryTimeMs = exitTimeMs
	}
	if entryTimeMs > exitTimeMs {
		entryTimeMs = exitTimeMs
	}

	nowMs := time.Now().UTC().UnixMilli()
	pos := &TraderPosition{
		TraderID:           traderID,
		ExchangeID:         exchangeID,
		ExchangeType:       exchangeType,
		ExchangePositionID: exchangePositionID,
		Symbol:             record.Symbol,
		Side:               side,
		Quantity:           record.Quantity,
		EntryQuantity:      record.Quantity,
		EntryPrice:         record.EntryPrice,
		EntryTime:          entryTimeMs,
		ExitPrice:          record.ExitPrice,
		ExitOrderID:        record.OrderID,
		ExitTime:           exitTimeMs,
		RealizedPnL:        record.RealizedPnL,
		Fee:                record.Fee,
		Leverage:           record.Leverage,
		Status:             "CLOSED",
		CloseReason:        record.CloseType,
		Source:             "sync",
		CreatedAt:          nowMs,
		UpdatedAt:          nowMs,
	}

	err = s.db.Create(pos).Error
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return false, nil
		}
		return false, fmt.Errorf("failed to create position from closed PnL: %w", err)
	}

	return true, nil
}

// GetLastClosedPositionTime gets the most recent exit time (Unix ms)
func (s *PositionStore) GetLastClosedPositionTime(traderID string) (int64, error) {
	var pos TraderPosition
	err := s.db.Where("trader_id = ? AND status = ? AND exit_time > 0", traderID, "CLOSED").
		Order("exit_time DESC").
		First(&pos).Error

	if err == gorm.ErrRecordNotFound || pos.ExitTime == 0 {
		return time.Now().UTC().Add(-30 * 24 * time.Hour).UnixMilli(), nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get last closed position time: %w", err)
	}

	return pos.ExitTime, nil
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

// SyncClosedPositions syncs closed positions from exchange
func (s *PositionStore) SyncClosedPositions(traderID, exchangeID, exchangeType string, records []ClosedPnLRecord) (int, int, error) {
	created, skipped := 0, 0
	for _, record := range records {
		rec := record
		wasCreated, err := s.CreateFromClosedPnL(traderID, exchangeID, exchangeType, &rec)
		if err != nil {
			return created, skipped, fmt.Errorf("failed to sync position: %w", err)
		}
		if wasCreated {
			created++
		} else {
			skipped++
		}
	}
	return created, skipped, nil
}
