package store

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PaperLedger is an account snapshot. State contains positions, pending orders,
// execution receipts and settings; Version is an optimistic concurrency token.
// The snapshot and Binance-shaped accounting rows always commit together.
type PaperLedger struct {
	ExchangeID     string `gorm:"primaryKey"`
	TraderID       string
	ExchangeType   string
	InitialBalance float64
	WalletBalance  float64
	RealizedPnL    float64
	LastAdvanceAt  int64 // UTC Unix milliseconds
	Version        int64
	State          string `gorm:"type:text;not null"`
}

func (PaperLedger) TableName() string { return "paper_ledgers" }

var ErrPaperLedgerConflict = errors.New("paper ledger changed; retry operation")

type PaperLedgerStore struct{ db *gorm.DB }

func (s *Store) PaperLedger() *PaperLedgerStore { return &PaperLedgerStore{db: s.gdb} }

func (s *PaperLedgerStore) InitTables() error { return s.db.AutoMigrate(&PaperLedger{}) }

func (s *PaperLedgerStore) Ensure(initial *PaperLedger) (*PaperLedger, error) {
	if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(initial).Error; err != nil {
		return nil, err
	}
	return s.Get(initial.ExchangeID)
}

func (s *PaperLedgerStore) Get(exchangeID string) (*PaperLedger, error) {
	var row PaperLedger
	if err := s.db.Where("exchange_id = ?", exchangeID).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// PaperLedgerTrade carries the same records consumed by OrderStore.ApplyTrade.
type PaperLedgerTrade struct {
	Order             *TraderOrder
	Fill              *TraderFill
	CloseReason       string
	RemainingQuantity float64
}

// Commit checks the account version BEFORE applying fills, inside the same
// transaction. A stale second executor can never overwrite a newer balance.
// Before StartOrderSync supplies an owner, rows use paper:<exchangeID>; binding
// moves only that account's provisional rows to the real trader, atomically.
func (s *PaperLedgerStore) Commit(next *PaperLedger, previous *PaperLedger, trades []PaperLedgerTrade, leverageChanges map[string]int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&PaperLedger{}).Where("exchange_id = ? AND version = ?", next.ExchangeID, previous.Version).
			Select("trader_id", "exchange_type", "initial_balance", "wallet_balance", "realized_pnl", "last_advance_at", "version", "state").Updates(next)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrPaperLedgerConflict
		}
		if previous.TraderID != next.TraderID || previous.ExchangeType != next.ExchangeType {
			owner := previous.TraderID
			if owner == "" {
				owner = "paper:" + next.ExchangeID
			}
			for _, model := range []interface{}{&TraderOrder{}, &TraderFill{}, &TraderPosition{}} {
				if err := tx.Model(model).Where("exchange_id = ? AND trader_id = ?", next.ExchangeID, owner).
					Updates(map[string]interface{}{"trader_id": next.TraderID, "exchange_type": next.ExchangeType}).Error; err != nil {
					return err
				}
			}
		}
		for _, trade := range trades {
			pos, err := NewPositionStore(tx).GetOpenPositionBySymbol(trade.Order.TraderID, trade.Order.Symbol, trade.Order.PositionSide)
			if err != nil {
				return err
			}
			if pos != nil && pos.ExchangeID != next.ExchangeID {
				return fmt.Errorf("paper position belongs to another exchange")
			}
			applied, err := NewOrderStore(tx).ApplyTrade(trade.Order, trade.Fill)
			if err != nil {
				return err
			}
			if !applied {
				continue
			}
			if pos == nil {
				pos, err = NewPositionStore(tx).GetOpenPositionBySymbol(trade.Order.TraderID, trade.Order.Symbol, trade.Order.PositionSide)
				if err != nil {
					return err
				}
			}
			if pos != nil {
				updates := map[string]interface{}{"leverage": trade.Order.Leverage}
				if trade.CloseReason != "" && trade.RemainingQuantity == 0 {
					updates["close_reason"] = trade.CloseReason
				}
				// The shared builder treats <1e-4 remainder as flat. Futures lot
				// steps can be smaller: retain an exact, genuinely partial close.
				if trade.RemainingQuantity > 0 {
					updates["quantity"], updates["status"] = trade.RemainingQuantity, "OPEN"
					updates["exit_time"], updates["exit_order_id"], updates["close_reason"] = 0, "", ""
				}
				if err := tx.Model(&TraderPosition{}).Where("id = ?", pos.ID).Updates(updates).Error; err != nil {
					return err
				}
			}
		}
		owner := next.TraderID
		if owner == "" {
			owner = "paper:" + next.ExchangeID
		}
		for symbol, leverage := range leverageChanges {
			if err := tx.Model(&TraderPosition{}).
				Where("exchange_id = ? AND trader_id = ? AND symbol = ? AND status = ?", next.ExchangeID, owner, symbol, "OPEN").
				Update("leverage", leverage).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
