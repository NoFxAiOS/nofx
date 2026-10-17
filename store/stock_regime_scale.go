package store

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StockRegimeScale is the regime scale last applied to one us_stock position
// (set when the position was opened/added or last trimmed). The live trim
// compares the current scale with it; a missing row means 1.0. One row per
// (trader, symbol).
type StockRegimeScale struct {
	TraderID     string  `gorm:"column:trader_id;primaryKey" json:"trader_id"`
	Symbol       string  `gorm:"column:symbol;primaryKey" json:"symbol"`
	AppliedScale float64 `gorm:"column:applied_scale" json:"applied_scale"`
	UpdatedAt    int64   `gorm:"column:updated_at" json:"updated_at"` // Unix ms UTC
}

func (StockRegimeScale) TableName() string { return "stock_regime_scales" }

// StockRegimeScaleStore persists the per-position applied regime scale.
type StockRegimeScaleStore struct {
	db *gorm.DB
}

func NewStockRegimeScaleStore(db *gorm.DB) *StockRegimeScaleStore {
	return &StockRegimeScaleStore{db: db}
}

func (s *StockRegimeScaleStore) initTables() error {
	return s.db.AutoMigrate(&StockRegimeScale{})
}

// List returns every applied scale of a trader keyed by symbol.
func (s *StockRegimeScaleStore) List(traderID string) (map[string]float64, error) {
	var rows []StockRegimeScale
	if err := s.db.Where("trader_id = ?", traderID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(rows))
	for _, r := range rows {
		out[r.Symbol] = r.AppliedScale
	}
	return out, nil
}

// Set stores (or replaces) the applied scale of one symbol.
func (s *StockRegimeScaleStore) Set(traderID, symbol string, scale float64) error {
	row := &StockRegimeScale{TraderID: traderID, Symbol: symbol, AppliedScale: scale, UpdatedAt: time.Now().UTC().UnixMilli()}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "trader_id"}, {Name: "symbol"}},
		DoUpdates: clause.AssignmentColumns([]string{"applied_scale", "updated_at"}),
	}).Create(row).Error
}

// Delete removes the applied scale of one symbol (no error when absent).
func (s *StockRegimeScaleStore) Delete(traderID, symbol string) error {
	return s.db.Where("trader_id = ? AND symbol = ?", traderID, symbol).Delete(&StockRegimeScale{}).Error
}
