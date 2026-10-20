package store

import (
	"fmt"
	"math"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StockProtectionOrder retains each program protection leg, including retired
// legs. Status is terminal only after its final cumulative fills are booked.
// The fill watermark and position updates commit together, including on restart.
type StockProtectionOrder struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	TraderID       string `gorm:"uniqueIndex:idx_stock_protection_order"`
	Symbol         string `gorm:"uniqueIndex:idx_stock_protection_order"`
	OrderID        string `gorm:"uniqueIndex:idx_stock_protection_order"`
	OrderListID    string
	ClientOrderID  string
	ListClientID   string
	Role           string // stop_loss / take_profit
	Quantity       float64
	StopPrice      float64
	StopLimitPrice float64
	TakeProfit     float64
	BookedQty      float64
	BookedQuote    float64
	BookedFee      float64
	Status         string
	Retired        bool // final executions reconciled after canceling the set
	UpdatedAt      int64
}

func (StockProtectionOrder) TableName() string { return "stock_protection_orders" }

// SaveProtectionOrders never resets a known order's cumulative fill watermark.
// Old legs remain available for reconciliation until their terminal status is read.
func (s *StockPaperStore) SaveProtectionOrders(rows []StockProtectionOrder) error {
	if len(rows) == 0 {
		return fmt.Errorf("protection response has no order IDs")
	}
	return s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

func (s *StockPaperStore) ListProtectionOrders(traderID, symbol string) ([]StockProtectionOrder, error) {
	var rows []StockProtectionOrder
	q := s.db.Where("trader_id = ?", traderID)
	if symbol != "" {
		q = q.Where("symbol = ?", symbol)
	}
	err := q.Order("id ASC").Find(&rows).Error
	return rows, err
}

// RetireProtectionOrders is called only after every leg has a verified terminal
// status. Keeping the rows preserves the watermark and last protective prices.
func (s *StockPaperStore) RetireProtectionOrders(traderID, symbol string) error {
	return s.db.Model(&StockProtectionOrder{}).Where("trader_id = ? AND symbol = ?", traderID, symbol).Update("retired", true).Error
}

// BookProtectionStatus books only the new cumulative execution delta. Quote is
// cumulative exchange quote proceeds, NOT the order's limit/trigger price.
// CANCELED/EXPIRED can carry fills too (notably a partially filled OCO sibling).
func (s *StockPaperStore) BookProtectionStatus(key StockProtectionOrder, executed, quote, fee float64, status string, now time.Time) (qty, price, pnl float64, closed bool, err error) {
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var leg StockProtectionOrder
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("trader_id = ? AND symbol = ? AND order_id = ?", key.TraderID, key.Symbol, key.OrderID).First(&leg).Error; e != nil {
			return e
		}
		if math.IsNaN(executed) || math.IsInf(executed, 0) || math.IsNaN(quote) || math.IsInf(quote, 0) || math.IsNaN(fee) || math.IsInf(fee, 0) || executed < 0 || quote < 0 || executed < leg.BookedQty-1e-12 || quote < leg.BookedQuote-1e-12 || fee < 0 {
			return fmt.Errorf("invalid/regressing protection execution for %s", leg.OrderID)
		}
		qty = executed - leg.BookedQty
		if qty > 1e-12 {
			value := quote - leg.BookedQuote
			if value <= 0 {
				return fmt.Errorf("protection fill %s has no execution proceeds", leg.OrderID)
			}
			price = value / qty
			positions := NewPositionStore(tx)
			rows, e := positions.StockOwnedRows(leg.TraderID, leg.Symbol)
			if e != nil {
				return e
			}
			owned := 0.0
			for _, r := range rows {
				owned += r.Quantity
			}
			if qty > owned+1e-9 {
				return fmt.Errorf("protection fill %s qty %.9f exceeds recorded ownership %.9f", leg.OrderID, qty, owned)
			}
			remaining := qty
			for _, r := range rows {
				take := math.Min(remaining, r.Quantity)
				if take <= 1e-12 {
					break
				}
				legPnL := (price - r.EntryPrice) * take
				pnl += legPnL
				priorClosed := math.Max(r.EntryQuantity-r.Quantity, 0)
				exit := (r.ExitPrice*priorClosed + price*take) / (priorClosed + take)
				updates := map[string]interface{}{
					"quantity": r.Quantity - take, "exit_price": exit, "exit_order_id": leg.OrderID,
					"realized_pnl": gorm.Expr("realized_pnl + ?", legPnL),
					"fee":          gorm.Expr("fee + ?", math.Max(fee-leg.BookedFee, 0)*take/qty),
					"updated_at":   now.UTC().UnixMilli(),
				}
				if r.Quantity-take <= 1e-9 {
					// Closed rows retain total entry quantity, as ClosePositionFully does.
					updates["quantity"] = math.Max(r.EntryQuantity, r.Quantity)
					updates["status"], updates["close_reason"] = "CLOSED", leg.Role
					updates["exit_time"] = now.UTC().UnixMilli()
				}
				if e := tx.Model(&TraderPosition{}).Where("id = ?", r.ID).Updates(updates).Error; e != nil {
					return e
				}
				remaining -= take
			}
			closed = owned-qty <= 1e-9
			if closed {
				if e := tx.Where("trader_id = ? AND symbol = ?", leg.TraderID, leg.Symbol).Delete(&StockProtectionIntent{}).Error; e != nil {
					return e
				}
			}
		}
		return tx.Model(&leg).Updates(map[string]interface{}{
			"booked_qty": executed, "booked_quote": quote, "booked_fee": fee,
			"status": status, "updated_at": now.UTC().UnixMilli(),
		}).Error
	})
	return
}

// StockProtectionIntent marks a program position whose FIRST protection has not
// been confirmed yet (a buy fill was booked, the stop/OCO is still to be placed).
// Unlike a legacy position of unknown provenance, such a position is known to be
// the program's own and was never covered by a protection that could have sold
// it, so reconciliation may retry the protection (and, after Attempts failures,
// exit) instead of quarantining it forever. Cleared once protection is placed or
// the position is closed.
type StockProtectionIntent struct {
	TraderID  string `gorm:"column:trader_id;primaryKey"`
	Symbol    string `gorm:"column:symbol;primaryKey"`
	Attempts  int    `gorm:"column:attempts;default:0"`
	CreatedAt int64  `gorm:"column:created_at"`
}

func (StockProtectionIntent) TableName() string { return "stock_protection_intents" }

// ListProtectionIntents returns the trader's symbols with an unconfirmed first protection.
func (s *StockPaperStore) ListProtectionIntents(traderID string) (map[string]bool, error) {
	var rows []StockProtectionIntent
	if err := s.db.Where("trader_id = ?", traderID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(rows))
	for _, r := range rows {
		out[r.Symbol] = true
	}
	return out, nil
}

// BumpProtectionIntent counts one failed protection attempt and returns the new
// total; 0 when the symbol has no intent.
func (s *StockPaperStore) BumpProtectionIntent(traderID, symbol string) (int, error) {
	res := s.db.Model(&StockProtectionIntent{}).Where("trader_id = ? AND symbol = ?", traderID, symbol).
		Update("attempts", gorm.Expr("attempts + 1"))
	if res.Error != nil || res.RowsAffected == 0 {
		return 0, res.Error
	}
	var row StockProtectionIntent
	if err := s.db.Where("trader_id = ? AND symbol = ?", traderID, symbol).First(&row).Error; err != nil {
		return 0, err
	}
	return row.Attempts, nil
}

// ClearProtectionIntent forgets the intent (protection placed / position closed).
func (s *StockPaperStore) ClearProtectionIntent(traderID, symbol string) error {
	return s.db.Where("trader_id = ? AND symbol = ?", traderID, symbol).Delete(&StockProtectionIntent{}).Error
}

// UpdateProtectionClientID persists a client ID that legitimately changed on
// the exchange (a cancel request's newClientOrderId replaces the order's ID).
func (s *StockPaperStore) UpdateProtectionClientID(traderID, symbol, orderID, clientID string) error {
	if clientID == "" {
		return nil
	}
	return s.db.Model(&StockProtectionOrder{}).Where("trader_id = ? AND symbol = ? AND order_id = ?", traderID, symbol, orderID).
		Update("client_order_id", clientID).Error
}
