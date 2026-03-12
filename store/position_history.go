package store

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

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
