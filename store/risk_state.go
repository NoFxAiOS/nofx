package store

import (
	"time"

	"gorm.io/gorm"
)

// ============================================================================
// Account risk state (F8, 2026-10-01 review): the daily-loss halt's day-start
// equity anchor used to live only on the AutoTrader instance — a strategy
// save (which destroys and reloads traders) or a process restart re-anchored
// at the CURRENT equity and silently cleared the halt mid-loss. This table
// makes the anchor durable per (account, UTC day) with a FIRST-ANCHOR-WINS
// contract: once a day has a baseline, later anchors for the same day are
// no-ops. In-process reloads are additionally covered by a package registry
// in trader/ for instances without a store (unit tests).
// ============================================================================

type RiskBaseline struct {
	ID             int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	AccountKey     string  `gorm:"column:account_key;not null;index:idx_risk_baseline_key,unique" json:"account_key"`
	Day            string  `gorm:"column:day;not null;index:idx_risk_baseline_key,unique" json:"day"` // UTC YYYY-MM-DD
	DayStartEquity float64 `gorm:"column:day_start_equity;not null" json:"day_start_equity"`
	// HaltedAt latches the daily-loss halt for this (account, UTC day): once
	// set, opens stay halted for the rest of the day even if equity recovers
	// above the threshold (2026-10-16 review A). NULL = not triggered.
	HaltedAt  *time.Time `gorm:"column:halted_at" json:"halted_at,omitempty"`
	CreatedAt time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (RiskBaseline) TableName() string { return "risk_baselines" }

type RiskStateStore struct {
	db *gorm.DB
}

func NewRiskStateStore(db *gorm.DB) *RiskStateStore { return &RiskStateStore{db: db} }

func (s *RiskStateStore) initTables() error { return s.db.AutoMigrate(&RiskBaseline{}) }

// DayBaseline returns the persisted day-start equity for (account, day).
func (s *RiskStateStore) DayBaseline(accountKey, day string) (float64, bool, error) {
	var row RiskBaseline
	err := s.db.Where("account_key = ? AND day = ?", accountKey, day).First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, false, nil
		}
		return 0, false, err
	}
	return row.DayStartEquity, true, nil
}

// AnchorDayBaseline persists the day-start equity FIRST-WINS: an existing row
// for (account, day) is never overwritten. Returns the effective baseline.
func (s *RiskStateStore) AnchorDayBaseline(accountKey, day string, equity float64) (float64, error) {
	var row RiskBaseline
	err := s.db.Where("account_key = ? AND day = ?", accountKey, day).First(&row).Error
	if err == nil {
		return row.DayStartEquity, nil // first anchor wins
	}
	if err != gorm.ErrRecordNotFound {
		return 0, err
	}
	row = RiskBaseline{AccountKey: accountKey, Day: day, DayStartEquity: equity}
	if err := s.db.Create(&row).Error; err != nil {
		// Lost a create race — the winner's baseline governs.
		var winner RiskBaseline
		if err2 := s.db.Where("account_key = ? AND day = ?", accountKey, day).First(&winner).Error; err2 == nil {
			return winner.DayStartEquity, nil
		}
		return 0, err
	}
	return equity, nil
}

// LatchDayHalt records that the daily-loss halt fired for (account, day). First
// latch wins (the original trigger time is kept); a missing baseline row is an
// error — the halt is only ever evaluated against an anchored day.
func (s *RiskStateStore) LatchDayHalt(accountKey, day string, at time.Time) error {
	res := s.db.Model(&RiskBaseline{}).
		Where("account_key = ? AND day = ? AND halted_at IS NULL", accountKey, day).
		Update("halted_at", at.UTC())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var row RiskBaseline
		if err := s.db.Where("account_key = ? AND day = ?", accountKey, day).First(&row).Error; err != nil {
			return err // no baseline row (or read failure): the latch was NOT stored
		}
		// Row exists with halted_at already set: first latch wins, nothing to do.
	}
	return nil
}

// DayHalt returns the trigger time of the latched daily-loss halt for
// (account, day), found=false when it has not fired.
func (s *RiskStateStore) DayHalt(accountKey, day string) (time.Time, bool, error) {
	var row RiskBaseline
	err := s.db.Where("account_key = ? AND day = ?", accountKey, day).First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	if row.HaltedAt == nil {
		return time.Time{}, false, nil
	}
	return *row.HaltedAt, true, nil
}

// Peak is monotonic across reloads and processes; SQL guards concurrent updates.
func (s *RiskStateStore) UpdatePeak(accountKey string, equity float64) (float64, error) {
	if _, err := s.AnchorDayBaseline(accountKey, "__peak__", equity); err != nil {
		return 0, err
	}
	if err := s.db.Model(&RiskBaseline{}).Where("account_key = ? AND day = ? AND day_start_equity < ?", accountKey, "__peak__", equity).Update("day_start_equity", equity).Error; err != nil {
		return 0, err
	}
	peak, _, err := s.DayBaseline(accountKey, "__peak__")
	return peak, err
}
