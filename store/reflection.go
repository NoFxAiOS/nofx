package store

import (
	"time"

	"gorm.io/gorm"
)

// Reflection reflection record
type Reflection struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TraderID   string    `gorm:"index" json:"trader_id"`
	PositionID int64     `gorm:"uniqueIndex" json:"position_id"` // Link to TraderPosition
	Content    string    `gorm:"type:text" json:"content"`
	Score      int       `json:"score"` // 1-10
	Tags       string    `json:"tags"`  // JSON array
	CreatedAt  time.Time `json:"created_at"`
}

// ReflectionStore reflection storage
type ReflectionStore struct {
	db *gorm.DB
}

// NewReflectionStore creates a new ReflectionStore
func NewReflectionStore(db *gorm.DB) *ReflectionStore {
	return &ReflectionStore{db: db}
}

// initTables initializes reflection tables
func (s *ReflectionStore) initTables() error {
	// For PostgreSQL with existing table, skip AutoMigrate
	if s.db.Dialector.Name() == "postgres" {
		var tableExists int64
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'reflections'`).Scan(&tableExists)
		if tableExists > 0 {
			return nil
		}
	}
	return s.db.AutoMigrate(&Reflection{})
}

// Create creates a reflection record
func (s *ReflectionStore) Create(reflection *Reflection) error {
	if reflection.CreatedAt.IsZero() {
		reflection.CreatedAt = time.Now().UTC()
	}
	return s.db.Create(reflection).Error
}

// GetByPositionID gets reflection by position ID
func (s *ReflectionStore) GetByPositionID(positionID int64) (*Reflection, error) {
	var reflection Reflection
	err := s.db.Where("position_id = ?", positionID).First(&reflection).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &reflection, nil
}

// GetByTraderID gets reflections by trader ID
func (s *ReflectionStore) GetByTraderID(traderID string, limit int) ([]*Reflection, error) {
	var reflections []*Reflection
	err := s.db.Where("trader_id = ?", traderID).
		Order("created_at DESC").
		Limit(limit).
		Find(&reflections).Error
	if err != nil {
		return nil, err
	}
	return reflections, nil
}

// Update updates a reflection record
func (s *ReflectionStore) Update(reflection *Reflection) error {
	return s.db.Save(reflection).Error
}
