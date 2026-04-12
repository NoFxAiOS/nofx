package store

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// PositionCloseEvent records each partial/full close event for a trader position.
// It is the child event stream behind the aggregated trader_positions history row.
type PositionCloseEvent struct {
	ID               int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	PositionID       int64   `gorm:"column:position_id;not null;index:idx_close_events_position_time" json:"position_id"`
	TraderID         string  `gorm:"column:trader_id;not null;index:idx_close_events_trader_time" json:"trader_id"`
	ExchangeID       string  `gorm:"column:exchange_id;not null;default:''" json:"exchange_id"`
	Symbol           string  `gorm:"column:symbol;not null;index:idx_close_events_symbol" json:"symbol"`
	Side             string  `gorm:"column:side;not null" json:"side"`
	CloseReason      string  `gorm:"column:close_reason;default:''" json:"close_reason"`
	ExecutionSource  string  `gorm:"column:execution_source;default:''" json:"execution_source"`
	ExecutionType    string  `gorm:"column:execution_type;default:''" json:"execution_type"`
	DecisionCycle    int     `gorm:"column:decision_cycle;default:0" json:"decision_cycle"`
	ExchangeOrderID  string  `gorm:"column:exchange_order_id;default:''" json:"exchange_order_id"`
	CloseQuantity    float64 `gorm:"column:close_quantity;default:0" json:"close_quantity"`
	CloseRatioPct    float64 `gorm:"column:close_ratio_pct;default:0" json:"close_ratio_pct"`
	ExecutionPrice   float64 `gorm:"column:execution_price;default:0" json:"execution_price"`
	CloseValueUSDT   float64 `gorm:"column:close_value_usdt;default:0" json:"close_value_usdt"`
	RealizedPnLDelta float64 `gorm:"column:realized_pnl_delta;default:0" json:"realized_pnl_delta"`
	FeeDelta         float64 `gorm:"column:fee_delta;default:0" json:"fee_delta"`
	EventTime        int64   `gorm:"column:event_time;not null;index:idx_close_events_position_time,sort:desc;index:idx_close_events_trader_time,sort:desc" json:"event_time"`
	CreatedAt        int64   `gorm:"column:created_at" json:"created_at"`
}

func (PositionCloseEvent) TableName() string { return "position_close_events" }

type PositionCloseEventStore struct {
	db *gorm.DB
}

func NewPositionCloseEventStore(db *gorm.DB) *PositionCloseEventStore {
	return &PositionCloseEventStore{db: db}
}

func (s *PositionCloseEventStore) InitTables() error {
	if s.db.Dialector.Name() == "postgres" {
		var tableExists int64
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'position_close_events'`).Scan(&tableExists)
		if tableExists > 0 {
			s.db.Exec(`ALTER TABLE position_close_events ADD COLUMN IF NOT EXISTS decision_cycle INTEGER DEFAULT 0`)
			return nil
		}
	}
	if err := s.db.AutoMigrate(&PositionCloseEvent{}); err != nil {
		return fmt.Errorf("failed to migrate position_close_events table: %w", err)
	}
	return nil
}

func (s *PositionCloseEventStore) Create(event *PositionCloseEvent) error {
	if event == nil {
		return nil
	}
	if event.EventTime == 0 {
		event.EventTime = time.Now().UTC().UnixMilli()
	}
	if event.CreatedAt == 0 {
		event.CreatedAt = time.Now().UTC().UnixMilli()
	}
	return s.db.Create(event).Error
}

func (s *PositionCloseEventStore) ListByPositionID(positionID int64) ([]*PositionCloseEvent, error) {
	var events []*PositionCloseEvent
	err := s.db.Where("position_id = ?", positionID).Order("event_time ASC, id ASC").Find(&events).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query close events: %w", err)
	}
	return events, nil
}

func (s *PositionCloseEventStore) UpdateReasonByOrderID(traderID, exchangeOrderID, closeReason, executionSource string) error {
	if exchangeOrderID == "" {
		return nil
	}
	updates := map[string]interface{}{}
	if closeReason != "" {
		updates["close_reason"] = closeReason
	}
	if executionSource != "" {
		updates["execution_source"] = executionSource
	}
	if len(updates) == 0 {
		return nil
	}
	return s.db.Model(&PositionCloseEvent{}).
		Where("trader_id = ? AND exchange_order_id = ?", traderID, exchangeOrderID).
		Updates(updates).Error
}
