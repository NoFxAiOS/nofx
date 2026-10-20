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
	// A trim in progress: persisted BEFORE the sell so a partial fill or a crash
	// resumes towards the same target remaining quantity instead of re-applying
	// the ratio to what is left. Both zero = no plan.
	TrimTargetScale float64 `gorm:"column:trim_target_scale;default:0" json:"trim_target_scale"`
	TrimTargetQty   float64 `gorm:"column:trim_target_qty;default:0" json:"trim_target_qty"`
}

// StockRegimeTrimPlan is the persisted target of an unfinished regime trim.
type StockRegimeTrimPlan struct {
	TargetScale float64
	TargetQty   float64
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

// Set stores (or replaces) the applied scale of one symbol and clears any trim plan.
func (s *StockRegimeScaleStore) Set(traderID, symbol string, scale float64) error {
	row := &StockRegimeScale{TraderID: traderID, Symbol: symbol, AppliedScale: scale, UpdatedAt: time.Now().UTC().UnixMilli()}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "trader_id"}, {Name: "symbol"}},
		DoUpdates: clause.AssignmentColumns([]string{"applied_scale", "updated_at", "trim_target_scale", "trim_target_qty"}),
	}).Create(row).Error
}

// ListPlans returns the unfinished trim plans of a trader keyed by symbol.
func (s *StockRegimeScaleStore) ListPlans(traderID string) (map[string]StockRegimeTrimPlan, error) {
	var rows []StockRegimeScale
	if err := s.db.Where("trader_id = ? AND trim_target_scale > 0 AND trim_target_qty > 0", traderID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]StockRegimeTrimPlan, len(rows))
	for _, r := range rows {
		out[r.Symbol] = StockRegimeTrimPlan{TargetScale: r.TrimTargetScale, TargetQty: r.TrimTargetQty}
	}
	return out, nil
}

// SetPlan persists the target of a trim before it is executed. The applied
// scale is preserved (a missing row means 1 and is created as such).
func (s *StockRegimeScaleStore) SetPlan(traderID, symbol string, targetScale, targetQty float64) error {
	row := &StockRegimeScale{TraderID: traderID, Symbol: symbol, AppliedScale: 1, UpdatedAt: time.Now().UTC().UnixMilli(),
		TrimTargetScale: targetScale, TrimTargetQty: targetQty}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "trader_id"}, {Name: "symbol"}},
		DoUpdates: clause.AssignmentColumns([]string{"updated_at", "trim_target_scale", "trim_target_qty"}),
	}).Create(row).Error
}

// ClearPlan drops the trim plan, keeping the applied scale.
func (s *StockRegimeScaleStore) ClearPlan(traderID, symbol string) error {
	return s.db.Model(&StockRegimeScale{}).Where("trader_id = ? AND symbol = ?", traderID, symbol).
		Updates(map[string]interface{}{"trim_target_scale": 0, "trim_target_qty": 0}).Error
}

// Delete removes the applied scale of one symbol (no error when absent).
func (s *StockRegimeScaleStore) Delete(traderID, symbol string) error {
	return s.db.Where("trader_id = ? AND symbol = ?", traderID, symbol).Delete(&StockRegimeScale{}).Error
}
