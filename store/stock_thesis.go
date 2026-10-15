package store

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StockThesis is the entry thesis and invalidation condition of an open
// us_stock position, carried forward into every later decision prompt. One row
// per (trader, symbol). A stop hit in the protection tick may leave a stale
// row behind; that is harmless because the next open overwrites it.
type StockThesis struct {
	TraderID     string `gorm:"column:trader_id;primaryKey" json:"trader_id"`
	Symbol       string `gorm:"column:symbol;primaryKey" json:"symbol"`
	Thesis       string `gorm:"column:thesis;default:''" json:"thesis"`
	Invalidation string `gorm:"column:invalidation;default:''" json:"invalidation"`
	UpdatedAt    int64  `gorm:"column:updated_at" json:"updated_at"` // Unix ms UTC
}

func (StockThesis) TableName() string { return "stock_theses" }

// StockThesisStore persists the per-position thesis and invalidation.
type StockThesisStore struct {
	db *gorm.DB
}

func NewStockThesisStore(db *gorm.DB) *StockThesisStore { return &StockThesisStore{db: db} }

func (s *StockThesisStore) initTables() error {
	return s.db.AutoMigrate(&StockThesis{})
}

// Upsert stores (or replaces) the thesis of one symbol.
func (s *StockThesisStore) Upsert(traderID, symbol, thesis, invalidation string, now time.Time) error {
	row := &StockThesis{TraderID: traderID, Symbol: symbol, Thesis: thesis, Invalidation: invalidation, UpdatedAt: now.UTC().UnixMilli()}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "trader_id"}, {Name: "symbol"}},
		DoUpdates: clause.AssignmentColumns([]string{"thesis", "invalidation", "updated_at"}),
	}).Create(row).Error
}

// List returns every stored thesis of a trader keyed by symbol.
func (s *StockThesisStore) List(traderID string) (map[string]StockThesis, error) {
	var rows []StockThesis
	if err := s.db.Where("trader_id = ?", traderID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]StockThesis, len(rows))
	for _, r := range rows {
		out[r.Symbol] = r
	}
	return out, nil
}

// Delete removes the thesis of one symbol (no error when absent).
func (s *StockThesisStore) Delete(traderID, symbol string) error {
	return s.db.Where("trader_id = ? AND symbol = ?", traderID, symbol).Delete(&StockThesis{}).Error
}
