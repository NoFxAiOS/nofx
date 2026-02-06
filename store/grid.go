package store

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ==================== Grid Store Models ====================
// These models mirror the grid package types but are defined here
// to avoid import cycles between store and grid packages.

// GridConfigModel GORM model for grid_configs table
type GridConfigModel struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	UserID    string    `json:"user_id" gorm:"index"`
	TraderID  string    `json:"trader_id" gorm:"index"`
	Symbol    string    `json:"symbol" gorm:"not null"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	GridCount       int     `json:"grid_count" gorm:"default:10"`
	TotalInvestment float64 `json:"total_investment" gorm:"not null"`
	Leverage        int     `json:"leverage" gorm:"default:5"`
	UpperPrice      float64 `json:"upper_price"`
	LowerPrice      float64 `json:"lower_price"`
	UseATRBounds    bool    `json:"use_atr_bounds" gorm:"default:true"`
	ATRMultiplier   float64 `json:"atr_multiplier" gorm:"default:2.0"`
	Distribution    string  `json:"distribution" gorm:"default:gaussian"`

	MaxDrawdownPct     float64 `json:"max_drawdown_pct" gorm:"default:15.0"`
	StopLossPct        float64 `json:"stop_loss_pct" gorm:"default:5.0"`
	DailyLossLimitPct  float64 `json:"daily_loss_limit_pct" gorm:"default:10"`
	MaxPositionSizePct float64 `json:"max_position_size_pct" gorm:"default:30"`

	RegimeCheckInterval  int  `json:"regime_check_interval" gorm:"default:30"`
	AutoPauseOnTrend     bool `json:"auto_pause_on_trend" gorm:"default:true"`
	MinRangingScore      int  `json:"min_ranging_score" gorm:"default:60"`
	TrendResumeThreshold int  `json:"trend_resume_threshold" gorm:"default:70"`

	// Box indicator periods (1h candles)
	ShortBoxPeriod int `json:"short_box_period" gorm:"default:72"`  // 3 days
	MidBoxPeriod   int `json:"mid_box_period" gorm:"default:240"`   // 10 days
	LongBoxPeriod  int `json:"long_box_period" gorm:"default:500"`  // 21 days

	// Effective leverage limits by regime level
	NarrowRegimeLeverage   int `json:"narrow_regime_leverage" gorm:"default:2"`
	StandardRegimeLeverage int `json:"standard_regime_leverage" gorm:"default:4"`
	WideRegimeLeverage     int `json:"wide_regime_leverage" gorm:"default:3"`
	VolatileRegimeLeverage int `json:"volatile_regime_leverage" gorm:"default:2"`

	// Position limits by regime level (percentage of total investment)
	NarrowRegimePositionPct   float64 `json:"narrow_regime_position_pct" gorm:"default:40"`
	StandardRegimePositionPct float64 `json:"standard_regime_position_pct" gorm:"default:70"`
	WideRegimePositionPct     float64 `json:"wide_regime_position_pct" gorm:"default:60"`
	VolatileRegimePositionPct float64 `json:"volatile_regime_position_pct" gorm:"default:40"`

	OrderRefreshSec  int     `json:"order_refresh_sec" gorm:"default:300"`
	UseMakerOnly     bool    `json:"use_maker_only" gorm:"default:true"`
	SlippageTolerPct float64 `json:"slippage_toler_pct" gorm:"default:0.1"`

	AIProvider string `json:"ai_provider" gorm:"default:deepseek"`
	AIModel    string `json:"ai_model" gorm:"default:deepseek-chat"`
	IsActive   bool   `json:"is_active" gorm:"default:false"`

	// Direction adjustment settings
	EnableDirectionAdjust bool    `json:"enable_direction_adjust" gorm:"default:false"`
	DirectionBiasRatio    float64 `json:"direction_bias_ratio" gorm:"default:0.7"`
}

func (GridConfigModel) TableName() string {
	return "grid_configs"
}

// GridInstanceModel GORM model for grid_instances table
type GridInstanceModel struct {
	ID        string     `json:"id" gorm:"primaryKey"`
	ConfigID  string     `json:"config_id" gorm:"index;not null"`
	Symbol    string     `json:"symbol" gorm:"not null"`
	State     string     `json:"state" gorm:"not null"`
	StartedAt time.Time  `json:"started_at"`
	StoppedAt *time.Time `json:"stopped_at,omitempty"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"autoUpdateTime"`

	CurrentUpperPrice   float64 `json:"current_upper_price"`
	CurrentLowerPrice   float64 `json:"current_lower_price"`
	CurrentGridSpacing  float64 `json:"current_grid_spacing"`
	ActiveLevelCount    int     `json:"active_level_count"`
	CurrentRegime       string  `json:"current_regime"`
	RegimeScore         int     `json:"regime_score"`
	LastRegimeCheck     time.Time `json:"last_regime_check"`
	ConsecutiveTrending int     `json:"consecutive_trending"`

	// Current regime level (narrow/standard/wide/volatile/trending)
	CurrentRegimeLevel string `json:"current_regime_level" gorm:"default:standard"`

	// Box state
	ShortBoxUpper float64 `json:"short_box_upper"`
	ShortBoxLower float64 `json:"short_box_lower"`
	MidBoxUpper   float64 `json:"mid_box_upper"`
	MidBoxLower   float64 `json:"mid_box_lower"`
	LongBoxUpper  float64 `json:"long_box_upper"`
	LongBoxLower  float64 `json:"long_box_lower"`

	// Breakout state
	BreakoutLevel        string    `json:"breakout_level" gorm:"default:none"` // none/short/mid/long
	BreakoutDirection    string    `json:"breakout_direction"`                 // up/down
	BreakoutConfirmCount int       `json:"breakout_confirm_count" gorm:"default:0"`
	BreakoutStartTime    time.Time `json:"breakout_start_time"`

	// Position adjustment due to breakout
	PositionReductionPct float64 `json:"position_reduction_pct" gorm:"default:0"` // 0 = normal, 50 = reduced

	// Grid direction adjustment state
	CurrentDirection       string    `json:"current_direction" gorm:"default:neutral"`
	DirectionChangedAt     time.Time `json:"direction_changed_at"`
	DirectionChangeCount   int       `json:"direction_change_count" gorm:"default:0"`

	TotalProfit     float64   `json:"total_profit" gorm:"default:0"`
	TotalFees       float64   `json:"total_fees" gorm:"default:0"`
	TotalTrades     int       `json:"total_trades" gorm:"default:0"`
	WinningTrades   int       `json:"winning_trades" gorm:"default:0"`
	MaxDrawdown     float64   `json:"max_drawdown" gorm:"default:0"`
	CurrentDrawdown float64   `json:"current_drawdown" gorm:"default:0"`
	PeakEquity      float64   `json:"peak_equity" gorm:"default:0"`
	DailyProfit     float64   `json:"daily_profit" gorm:"default:0"`
	DailyLoss       float64   `json:"daily_loss" gorm:"default:0"`
	LastDailyReset  time.Time `json:"last_daily_reset"`
}

func (GridInstanceModel) TableName() string {
	return "grid_instances"
}

// GridLevelModel GORM model for grid_levels table
type GridLevelModel struct {
	ID               string     `json:"id" gorm:"primaryKey"`
	InstanceID       string     `json:"instance_id" gorm:"index;not null"`
	LevelIndex       int        `json:"level_index" gorm:"not null"`
	Price            float64    `json:"price" gorm:"not null"`
	State            string     `json:"state" gorm:"not null"`
	Side             string     `json:"side"`
	OrderID          string     `json:"order_id,omitempty"`
	OrderPrice       float64    `json:"order_price,omitempty"`
	OrderQuantity    float64    `json:"order_quantity,omitempty"`
	OrderCreatedAt   *time.Time `json:"order_created_at,omitempty"`
	PositionSize     float64    `json:"position_size,omitempty"`
	PositionEntry    float64    `json:"position_entry,omitempty"`
	PositionOpenAt   *time.Time `json:"position_open_at,omitempty"`
	AllocationWeight float64    `json:"allocation_weight"`
	AllocatedUSD     float64    `json:"allocated_usd"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (GridLevelModel) TableName() string {
	return "grid_levels"
}

// GridEventModel GORM model for grid_events table
type GridEventModel struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	InstanceID  string    `json:"instance_id" gorm:"index;not null"`
	LevelID     string    `json:"level_id,omitempty" gorm:"index"`
	EventType   string    `json:"event_type" gorm:"not null"`
	EventTime   time.Time `json:"event_time" gorm:"autoCreateTime"`
	Price       float64   `json:"price,omitempty"`
	Quantity    float64   `json:"quantity,omitempty"`
	Side        string    `json:"side,omitempty"`
	PnL         float64   `json:"pnl,omitempty"`
	Fee         float64   `json:"fee,omitempty"`
	Message     string    `json:"message,omitempty"`
	OldRegime   string    `json:"old_regime,omitempty"`
	NewRegime   string    `json:"new_regime,omitempty"`
	TriggerType string    `json:"trigger_type,omitempty"`
	RawData     string    `json:"raw_data,omitempty" gorm:"type:text"`
}

func (GridEventModel) TableName() string {
	return "grid_events"
}

// GridRegimeAssessmentModel GORM model for grid_regime_assessments table
type GridRegimeAssessmentModel struct {
	ID              string    `json:"id" gorm:"primaryKey"`
	InstanceID      string    `json:"instance_id" gorm:"index;not null"`
	AssessedAt      time.Time `json:"assessed_at" gorm:"autoCreateTime"`
	Regime          string    `json:"regime" gorm:"not null"`
	Score           int       `json:"score" gorm:"not null"`
	Confidence      float64   `json:"confidence"`
	BollingerSignal int       `json:"bollinger_signal"`
	EMASignal       int       `json:"ema_signal"`
	MACDSignal      int       `json:"macd_signal"`
	VolumeSignal    int       `json:"volume_signal"`
	OISignal        int       `json:"oi_signal"`
	FundingSignal   int       `json:"funding_signal"`
	CandleSignal    int       `json:"candle_signal"`
	ATR14           float64   `json:"atr14"`
	BollingerWidth  float64   `json:"bollinger_width"`
	EMADistance     float64   `json:"ema_distance"`
	CurrentPrice    float64   `json:"current_price"`
	AIReasoning     string    `json:"ai_reasoning" gorm:"type:text"`
}

func (GridRegimeAssessmentModel) TableName() string {
	return "grid_regime_assessments"
}

// ==================== Grid Store ====================

// GridStore provides database operations for grid trading
type GridStore struct {
	db *gorm.DB
}

// NewGridStore creates a new grid store
func NewGridStore(db *gorm.DB) *GridStore {
	return &GridStore{db: db}
}

// InitTables initializes grid-related tables
func (s *GridStore) InitTables() error {
	// For PostgreSQL with existing tables, skip AutoMigrate to avoid type conflicts
	if s.db.Dialector.Name() == "postgres" {
		var tableExists int64
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'grid_configs'`).Scan(&tableExists)

		if tableExists > 0 {
			// Tables exist, just ensure indexes
			s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_grid_configs_user_id ON grid_configs(user_id)`)
			s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_grid_configs_trader_id ON grid_configs(trader_id)`)
			s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_grid_instances_config_id ON grid_instances(config_id)`)
			s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_grid_levels_instance_id ON grid_levels(instance_id)`)
			s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_grid_events_instance_id ON grid_events(instance_id)`)
			s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_grid_events_level_id ON grid_events(level_id)`)
			s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_grid_regime_assessments_instance_id ON grid_regime_assessments(instance_id)`)
			return nil
		}
	}

	// AutoMigrate all grid tables
	if err := s.db.AutoMigrate(
		&GridConfigModel{},
		&GridInstanceModel{},
		&GridLevelModel{},
		&GridEventModel{},
		&GridRegimeAssessmentModel{},
	); err != nil {
		return fmt.Errorf("failed to migrate grid tables: %w", err)
	}

	return nil
}

// ==================== Config Operations ====================

// SaveGridConfig saves or updates a grid configuration
func (s *GridStore) SaveGridConfig(config *GridConfigModel) error {
	config.UpdatedAt = time.Now()
	if config.CreatedAt.IsZero() {
		config.CreatedAt = time.Now()
	}
	return s.db.Save(config).Error
}

// LoadGridConfig loads a grid configuration by ID
func (s *GridStore) LoadGridConfig(id string) (*GridConfigModel, error) {
	var config GridConfigModel
	err := s.db.Where("id = ?", id).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// LoadGridConfigByTrader loads a grid configuration by trader ID
func (s *GridStore) LoadGridConfigByTrader(traderID string) (*GridConfigModel, error) {
	var config GridConfigModel
	err := s.db.Where("trader_id = ? AND is_active = true", traderID).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// ListGridConfigs lists all grid configurations for a user
func (s *GridStore) ListGridConfigs(userID string) ([]GridConfigModel, error) {
	var configs []GridConfigModel
	err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&configs).Error
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// DeleteGridConfig deletes a grid configuration and all related data
func (s *GridStore) DeleteGridConfig(id string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Get all instances for this config
		var instances []GridInstanceModel
		if err := tx.Where("config_id = ?", id).Find(&instances).Error; err != nil {
			return err
		}

		// Delete related data for each instance
		for _, instance := range instances {
			if err := tx.Where("instance_id = ?", instance.ID).Delete(&GridLevelModel{}).Error; err != nil {
				return err
			}
			if err := tx.Where("instance_id = ?", instance.ID).Delete(&GridEventModel{}).Error; err != nil {
				return err
			}
			if err := tx.Where("instance_id = ?", instance.ID).Delete(&GridRegimeAssessmentModel{}).Error; err != nil {
				return err
			}
		}

		// Delete instances
		if err := tx.Where("config_id = ?", id).Delete(&GridInstanceModel{}).Error; err != nil {
			return err
		}

		// Delete config
		return tx.Where("id = ?", id).Delete(&GridConfigModel{}).Error
	})
}

// ==================== Instance Operations ====================

// SaveGridInstance saves or updates a grid instance
func (s *GridStore) SaveGridInstance(instance *GridInstanceModel) error {
	instance.UpdatedAt = time.Now()
	return s.db.Save(instance).Error
}

// LoadGridInstance loads a grid instance by config ID
func (s *GridStore) LoadGridInstance(configID string) (*GridInstanceModel, error) {
	var instance GridInstanceModel
	err := s.db.Where("config_id = ?", configID).
		Order("started_at DESC").
		First(&instance).Error
	if err != nil {
		return nil, err
	}
	return &instance, nil
}

// LoadGridInstanceByID loads a grid instance by ID
func (s *GridStore) LoadGridInstanceByID(id string) (*GridInstanceModel, error) {
	var instance GridInstanceModel
	err := s.db.Where("id = ?", id).First(&instance).Error
	if err != nil {
		return nil, err
	}
	return &instance, nil
}

// ListGridInstances lists all instances for a config
func (s *GridStore) ListGridInstances(configID string) ([]GridInstanceModel, error) {
	var instances []GridInstanceModel
	err := s.db.Where("config_id = ?", configID).
		Order("started_at DESC").
		Find(&instances).Error
	if err != nil {
		return nil, err
	}
	return instances, nil
}

// ==================== Level Operations ====================

// SaveGridLevel saves or updates a grid level
func (s *GridStore) SaveGridLevel(level *GridLevelModel) error {
	level.UpdatedAt = time.Now()
	return s.db.Save(level).Error
}

// SaveGridLevels saves multiple grid levels
func (s *GridStore) SaveGridLevels(levels []GridLevelModel) error {
	if len(levels) == 0 {
		return nil
	}
	now := time.Now()
	for i := range levels {
		levels[i].UpdatedAt = now
	}
	return s.db.Save(&levels).Error
}

// LoadGridLevels loads all levels for an instance
func (s *GridStore) LoadGridLevels(instanceID string) ([]GridLevelModel, error) {
	var levels []GridLevelModel
	err := s.db.Where("instance_id = ?", instanceID).
		Order("level_index ASC").
		Find(&levels).Error
	if err != nil {
		return nil, err
	}
	return levels, nil
}

// DeleteGridLevels deletes all levels for an instance
func (s *GridStore) DeleteGridLevels(instanceID string) error {
	return s.db.Where("instance_id = ?", instanceID).Delete(&GridLevelModel{}).Error
}

// ==================== Event Operations ====================

// SaveGridEvent saves a grid event
func (s *GridStore) SaveGridEvent(event *GridEventModel) error {
	if event.EventTime.IsZero() {
		event.EventTime = time.Now()
	}
	return s.db.Create(event).Error
}

// LoadRecentGridEvents loads recent events for an instance
func (s *GridStore) LoadRecentGridEvents(instanceID string, limit int) ([]GridEventModel, error) {
	var events []GridEventModel
	query := s.db.Where("instance_id = ?", instanceID).
		Order("event_time DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}

// LoadGridEventsByType loads events of a specific type
func (s *GridStore) LoadGridEventsByType(instanceID, eventType string, limit int) ([]GridEventModel, error) {
	var events []GridEventModel
	query := s.db.Where("instance_id = ? AND event_type = ?", instanceID, eventType).
		Order("event_time DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}

// CountGridEvents counts events for an instance
func (s *GridStore) CountGridEvents(instanceID string) (int64, error) {
	var count int64
	err := s.db.Model(&GridEventModel{}).
		Where("instance_id = ?", instanceID).
		Count(&count).Error
	return count, err
}

// ==================== Regime Assessment Operations ====================

// SaveGridRegimeAssessment saves a regime assessment
func (s *GridStore) SaveGridRegimeAssessment(assessment *GridRegimeAssessmentModel) error {
	if assessment.AssessedAt.IsZero() {
		assessment.AssessedAt = time.Now()
	}
	return s.db.Create(assessment).Error
}

// LoadLatestGridRegime loads the latest regime assessment
func (s *GridStore) LoadLatestGridRegime(instanceID string) (*GridRegimeAssessmentModel, error) {
	var assessment GridRegimeAssessmentModel
	err := s.db.Where("instance_id = ?", instanceID).
		Order("assessed_at DESC").
		First(&assessment).Error
	if err != nil {
		return nil, err
	}
	return &assessment, nil
}

// LoadGridRegimeHistory loads regime assessment history
func (s *GridStore) LoadGridRegimeHistory(instanceID string, limit int) ([]GridRegimeAssessmentModel, error) {
	var assessments []GridRegimeAssessmentModel
	query := s.db.Where("instance_id = ?", instanceID).
		Order("assessed_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&assessments).Error
	if err != nil {
		return nil, err
	}
	return assessments, nil
}

// ==================== Statistics Operations ====================

// GetGridInstanceStatistics returns statistics for an instance
func (s *GridStore) GetGridInstanceStatistics(instanceID string) (map[string]interface{}, error) {
	var instance GridInstanceModel
	if err := s.db.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		return nil, err
	}

	// Count events by type
	var eventCounts []struct {
		EventType string
		Count     int64
	}
	s.db.Model(&GridEventModel{}).
		Select("event_type, count(*) as count").
		Where("instance_id = ?", instanceID).
		Group("event_type").
		Find(&eventCounts)

	eventCountMap := make(map[string]int64)
	for _, ec := range eventCounts {
		eventCountMap[ec.EventType] = ec.Count
	}

	// Get latest regime
	var latestRegime GridRegimeAssessmentModel
	s.db.Where("instance_id = ?", instanceID).
		Order("assessed_at DESC").
		First(&latestRegime)

	winRate := 0.0
	if instance.TotalTrades > 0 {
		winRate = float64(instance.WinningTrades) / float64(instance.TotalTrades) * 100
	}

	return map[string]interface{}{
		"instance_id":         instance.ID,
		"state":               instance.State,
		"started_at":          instance.StartedAt,
		"stopped_at":          instance.StoppedAt,
		"total_profit":        instance.TotalProfit,
		"total_fees":          instance.TotalFees,
		"total_trades":        instance.TotalTrades,
		"winning_trades":      instance.WinningTrades,
		"win_rate":            winRate,
		"max_drawdown":        instance.MaxDrawdown,
		"current_drawdown":    instance.CurrentDrawdown,
		"peak_equity":         instance.PeakEquity,
		"active_level_count":  instance.ActiveLevelCount,
		"current_regime":      instance.CurrentRegime,
		"regime_score":        instance.RegimeScore,
		"event_counts":        eventCountMap,
		"latest_regime_score": latestRegime.Score,
	}, nil
}

// GetGridPerformanceMetrics returns performance metrics for a time period
func (s *GridStore) GetGridPerformanceMetrics(instanceID string, from, to time.Time) (map[string]interface{}, error) {
	// Count trades in period
	var tradeCounts struct {
		TotalFills int64
		BuyFills   int64
		SellFills  int64
	}
	s.db.Model(&GridEventModel{}).
		Select("count(*) as total_fills, "+
			"sum(case when side = 'buy' then 1 else 0 end) as buy_fills, "+
			"sum(case when side = 'sell' then 1 else 0 end) as sell_fills").
		Where("instance_id = ? AND event_type = 'order_filled' AND event_time BETWEEN ? AND ?",
			instanceID, from, to).
		Scan(&tradeCounts)

	// Sum profit/loss
	var pnlSum struct {
		TotalPnL float64
		TotalFee float64
	}
	s.db.Model(&GridEventModel{}).
		Select("coalesce(sum(pnl), 0) as total_pnl, coalesce(sum(fee), 0) as total_fee").
		Where("instance_id = ? AND event_time BETWEEN ? AND ?", instanceID, from, to).
		Scan(&pnlSum)

	// Count regime changes
	var regimeChanges int64
	s.db.Model(&GridEventModel{}).
		Where("instance_id = ? AND event_type = 'regime_change' AND event_time BETWEEN ? AND ?",
			instanceID, from, to).
		Count(&regimeChanges)

	return map[string]interface{}{
		"period_start":   from,
		"period_end":     to,
		"total_fills":    tradeCounts.TotalFills,
		"buy_fills":      tradeCounts.BuyFills,
		"sell_fills":     tradeCounts.SellFills,
		"total_pnl":      pnlSum.TotalPnL,
		"total_fees":     pnlSum.TotalFee,
		"net_pnl":        pnlSum.TotalPnL - pnlSum.TotalFee,
		"regime_changes": regimeChanges,
	}, nil
}
