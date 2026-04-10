package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// QuantModel represents a small custom quantitative model for strategy
// These models can be used alongside or instead of AI prompts for backtesting
// and live trading, reducing dependency on external AI providers.
type QuantModel struct {
	ID          string `gorm:"primaryKey" json:"id"`
	UserID      string `gorm:"column:user_id;not null;index" json:"user_id"`
	Name        string `gorm:"not null" json:"name"`
	Description string `json:"description"`
	
	// Model metadata
	ModelType   string `json:"model_type"`     // e.g., "indicator_based", "ml_classifier", "rule_based"
	Version     string `gorm:"default:'1.0'" json:"version"`
	IsPublic    bool   `gorm:"default:false" json:"is_public"`
	IsActive    bool   `gorm:"default:true" json:"is_active"`
	
	// Model configuration as JSON
	Config string `gorm:"type:text" json:"config"`
	
	// Backtest statistics (for model ranking)
	BacktestCount   int     `gorm:"default:0" json:"backtest_count"`
	WinRate         float64 `json:"win_rate"`
	AvgProfitPct    float64 `json:"avg_profit_pct"`
	MaxDrawdownPct  float64 `json:"max_drawdown_pct"`
	SharpeRatio     float64 `json:"sharpe_ratio"`
	
	// Usage tracking
	UsageCount  int       `gorm:"default:0" json:"usage_count"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName returns the table name for QuantModel
func (QuantModel) TableName() string {
	return "quant_models"
}

// QuantModelConfig represents the configuration of a quantitative model
// This is stored as JSON in the Config field of QuantModel
type QuantModelConfig struct {
	// Model type-specific settings
	Type string `json:"type"` // "indicator_based", "ml_classifier", "rule_based", "ensemble"
	
	// For indicator-based models
	Indicators []ModelIndicator `json:"indicators,omitempty"`
	
	// For rule-based models
	Rules []ModelRule `json:"rules,omitempty"`
	
	// For ML classifier models
	MLConfig *MLModelConfig `json:"ml_config,omitempty"`
	
	// Ensemble configuration
	Ensemble *EnsembleConfig `json:"ensemble,omitempty"`
	
	// Common parameters
	Parameters ModelParameters `json:"parameters"`
	
	// Signal generation settings
	SignalConfig SignalGenerationConfig `json:"signal_config"`
}

// ModelIndicator defines a single indicator with parameters
// Used in indicator-based models
type ModelIndicator struct {
	Name      string                 `json:"name"`      // e.g., "RSI", "MACD", "EMA", "ATR", "BOLL"
	Period    int                    `json:"period"`    // e.g., 14 for RSI
	Timeframe string                 `json:"timeframe"` // e.g., "1h", "4h", "1d"
	Params    map[string]interface{} `json:"params,omitempty"` // Additional parameters
	Weight    float64                `json:"weight"`  // Weight in multi-indicator models
}

// ModelRule defines a single trading rule
// Used in rule-based models
type ModelRule struct {
	Name        string      `json:"name"`
	Condition   string      `json:"condition"`   // e.g., "RSI_14 < 30 AND Close > EMA_20"
	Action      string      `json:"action"`      // "buy", "sell", "hold"
	Confidence  int         `json:"confidence"`  // 0-100
	Priority    int         `json:"priority"`    // Higher = evaluated first
	StopLossPct *float64   `json:"stop_loss_pct,omitempty"`
	TakeProfitPct *float64 `json:"take_profit_pct,omitempty"`
}

// MLModelConfig for machine learning classifier models
type MLModelConfig struct {
	Algorithm     string            `json:"algorithm"`      // e.g., "random_forest", "xgboost", "neural_net"
	Features      []string          `json:"features"`       // Feature names
	ClassLabels   []string          `json:"class_labels"`   // e.g., ["buy", "sell", "hold"]
	ModelWeights  map[string]float64 `json:"model_weights,omitempty"`
	Thresholds    map[string]float64 `json:"thresholds,omitempty"`   // Decision thresholds
	TrainedAt     *time.Time        `json:"trained_at,omitempty"`
	TrainingData  *TrainingDataInfo `json:"training_data,omitempty"`
}

// TrainingDataInfo stores metadata about training data
type TrainingDataInfo struct {
	StartDate string   `json:"start_date"`
	EndDate   string   `json:"end_date"`
	Symbols   []string `json:"symbols"`
	Timeframes []string `json:"timeframes"`
}

// EnsembleConfig for combining multiple models
type EnsembleConfig struct {
	Method      string            `json:"method"`       // "weighted_vote", "stacking", "average"
	ModelIDs    []string          `json:"model_ids"`    // IDs of sub-models
	Weights     map[string]float64 `json:"weights"`     // Weights for each sub-model
	VotingThreshold float64       `json:"voting_threshold"` // Min consensus for action
}

// ModelParameters contains tunable parameters for the model
type ModelParameters struct {
	LookbackPeriods      int     `json:"lookback_periods"`       // Bars to look back
	EntryThreshold       float64 `json:"entry_threshold"`        // Signal threshold for entry
	ExitThreshold        float64 `json:"exit_threshold"`         // Signal threshold for exit
	MaxPositionHoldTime  int     `json:"max_position_hold_time"` // Max bars to hold
	MinPositionHoldTime  int     `json:"min_position_hold_time"` // Min bars before exit
	MaxDailyTrades       int     `json:"max_daily_trades"`       // Daily trade limit
}

// SignalGenerationConfig controls how signals are generated
type SignalGenerationConfig struct {
	SignalType       string  `json:"signal_type"`       // "discrete", "continuous", "probabilistic"
	MinConfidence    int     `json:"min_confidence"`    // Minimum confidence threshold
	RequireConfirmation bool `json:"require_confirmation"` // Wait for confirmation candle
	ConfirmationDelay   int  `json:"confirmation_delay"`    // Candles to wait for confirmation
}

// QuantModelStore handles CRUD operations for QuantModel
type QuantModelStore struct {
	db *gorm.DB
}

// NewQuantModelStore creates a new QuantModelStore
func NewQuantModelStore(db *gorm.DB) *QuantModelStore {
	return &QuantModelStore{db: db}
}

// InitTables creates the quant_models table
func (s *QuantModelStore) InitTables() error {
	return s.db.AutoMigrate(&QuantModel{})
}

// Create creates a new quant model
func (s *QuantModelStore) Create(model *QuantModel) error {
	if model.ID == "" {
		model.ID = uuid.New().String()
	}
	model.CreatedAt = time.Now().UTC()
	model.UpdatedAt = time.Now().UTC()
	return s.db.Create(model).Error
}

// Update updates an existing quant model
func (s *QuantModelStore) Update(model *QuantModel) error {
	model.UpdatedAt = time.Now().UTC()
	return s.db.Model(&QuantModel{}).
		Where("id = ? AND user_id = ?", model.ID, model.UserID).
		Updates(map[string]interface{}{
			"name":         model.Name,
			"description":  model.Description,
			"model_type":   model.ModelType,
			"version":      model.Version,
			"is_public":    model.IsPublic,
			"is_active":    model.IsActive,
			"config":       model.Config,
			"updated_at":   model.UpdatedAt,
		}).Error
}

// Delete deletes a quant model
func (s *QuantModelStore) Delete(userID, modelID string) error {
	// Check if model is being used by any strategy
	var count int64
	if err := s.db.Model(&Strategy{}).
		Where("config LIKE ?", fmt.Sprintf("%%\"quant_model_id\":\"%s\"%%", modelID)).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("cannot delete model: used by %d strategy(s)", count)
	}

	return s.db.Where("id = ? AND user_id = ?", modelID, userID).Delete(&QuantModel{}).Error
}

// Get retrieves a single quant model by ID
func (s *QuantModelStore) Get(userID, modelID string) (*QuantModel, error) {
	var model QuantModel
	err := s.db.Where("id = ? AND (user_id = ? OR is_public = ?)", modelID, userID, true).
		First(&model).Error
	if err != nil {
		return nil, err
	}
	return &model, nil
}

// List retrieves all quant models for a user
func (s *QuantModelStore) List(userID string) ([]*QuantModel, error) {
	var models []*QuantModel
	err := s.db.Where("user_id = ? OR is_public = ?", userID, true).
		Order("created_at DESC").
		Find(&models).Error
	return models, err
}

// ListPublic retrieves all public quant models
func (s *QuantModelStore) ListPublic() ([]*QuantModel, error) {
	var models []*QuantModel
	err := s.db.Where("is_public = ? AND is_active = ?", true, true).
		Order("usage_count DESC, win_rate DESC").
		Find(&models).Error
	return models, err
}

// IncrementUsage increments the usage counter for a model
func (s *QuantModelStore) IncrementUsage(modelID string) error {
	now := time.Now().UTC()
	return s.db.Model(&QuantModel{}).
		Where("id = ?", modelID).
		Updates(map[string]interface{}{
			"usage_count":  gorm.Expr("usage_count + 1"),
			"last_used_at": now,
		}).Error
}

// UpdateBacktestStats updates backtest statistics for a model
func (s *QuantModelStore) UpdateBacktestStats(modelID string, stats BacktestStats) error {
	return s.db.Model(&QuantModel{}).
		Where("id = ?", modelID).
		Updates(map[string]interface{}{
			"backtest_count":   gorm.Expr("backtest_count + 1"),
			"win_rate":         stats.WinRate,
			"avg_profit_pct":   stats.AvgProfitPct,
			"max_drawdown_pct": stats.MaxDrawdownPct,
			"sharpe_ratio":     stats.SharpeRatio,
		}).Error
}

// BacktestStats represents the results of a backtest run
type BacktestStats struct {
	WinRate        float64
	AvgProfitPct   float64
	MaxDrawdownPct float64
	SharpeRatio    float64
}

// ParseConfig parses the JSON config into QuantModelConfig
func (m *QuantModel) ParseConfig() (*QuantModelConfig, error) {
	var config QuantModelConfig
	if err := json.Unmarshal([]byte(m.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to parse model config: %w", err)
	}
	return &config, nil
}

// SetConfig serializes QuantModelConfig to JSON
func (m *QuantModel) SetConfig(config *QuantModelConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to serialize model config: %w", err)
	}
	m.Config = string(data)
	return nil
}

// ToExportFormat creates an exportable format of the model
func (m *QuantModel) ToExportFormat() (map[string]interface{}, error) {
	config, err := m.ParseConfig()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"version":     "1.0",
		"exported_at": time.Now().UTC().Format(time.RFC3339),
		"model": map[string]interface{}{
			"id":          m.ID,
			"name":        m.Name,
			"description": m.Description,
			"model_type":  m.ModelType,
			"version":     m.Version,
			"config":      config,
		},
	}, nil
}

// ImportFromExport creates a QuantModel from export format
func ImportFromExport(exportData map[string]interface{}, userID string) (*QuantModel, error) {
	modelData, ok := exportData["model"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid export format: missing 'model' field")
	}

	configData, err := json.Marshal(modelData["config"])
	if err != nil {
		return nil, fmt.Errorf("failed to serialize config: %w", err)
	}

	model := &QuantModel{
		ID:          uuid.New().String(), // Generate new ID
		UserID:      userID,
		Name:        getString(modelData, "name", "Imported Model"),
		Description: getString(modelData, "description", ""),
		ModelType:   getString(modelData, "model_type", "indicator_based"),
		Version:     getString(modelData, "version", "1.0"),
		Config:      string(configData),
		IsPublic:    false, // Imported models are private by default
		IsActive:    true,
	}

	return model, nil
}

// Helper function to safely get string from map
func getString(m map[string]interface{}, key, defaultValue string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

// StrategyQuantModelLink links a quant model to a strategy
type StrategyQuantModelLink struct {
	ModelID        string                 `json:"model_id"`
	ModelName      string                 `json:"model_name"`
	Mode           string                 `json:"mode"` // "primary", "secondary", "ensemble"
	Weight         float64                `json:"weight"` // For ensemble mode
	OverrideParams map[string]interface{} `json:"override_params,omitempty"`
}

// StrategyConfig extended with quant model integration
// This is part of StrategyConfig in store/strategy.go
type QuantModelIntegration struct {
	Enabled           bool                     `json:"enabled"`
	PrimaryModelID    string                   `json:"primary_model_id,omitempty"`
	SecondaryModels   []StrategyQuantModelLink `json:"secondary_models,omitempty"`
	FallbackToAI      bool                     `json:"fallback_to_ai"`      // Use AI if model fails
	ModelConfidenceThreshold float64          `json:"model_confidence_threshold"` // Min confidence from model
	BacktestBeforeLive bool                   `json:"backtest_before_live"`
}

// GetDefaultQuantModelConfig returns a default indicator-based model configuration
func GetDefaultQuantModelConfig() *QuantModelConfig {
	return &QuantModelConfig{
		Type: "indicator_based",
		Indicators: []ModelIndicator{
			{
				Name:      "RSI",
				Period:    14,
				Timeframe: "1h",
				Weight:    0.4,
			},
			{
				Name:      "EMA",
				Period:    20,
				Timeframe: "1h",
				Params:    map[string]interface{}{"second_period": 50},
				Weight:    0.3,
			},
			{
				Name:      "MACD",
				Period:    12,
				Timeframe: "1h",
				Params:    map[string]interface{}{"fast": 12, "slow": 26, "signal": 9},
				Weight:    0.3,
			},
		},
		Parameters: ModelParameters{
			LookbackPeriods:      100,
			EntryThreshold:       70,  // RSI below 30 (inverse for buy)
			ExitThreshold:        30,  // RSI above 70 (inverse for sell)
			MaxPositionHoldTime:  48,  // 48 hours
			MinPositionHoldTime:  4,   // 4 hours
			MaxDailyTrades:       3,
		},
		SignalConfig: SignalGenerationConfig{
			SignalType:            "discrete",
			MinConfidence:         65,
			RequireConfirmation:   true,
			ConfirmationDelay:     1,
		},
	}
}

// GetExampleRuleBasedConfig returns an example rule-based model configuration
func GetExampleRuleBasedConfig() *QuantModelConfig {
	stopLoss := 2.0
	takeProfit := 4.0

	return &QuantModelConfig{
		Type: "rule_based",
		Rules: []ModelRule{
			{
				Name:          "RSI_Oversold_EMA_Support",
				Condition:     "RSI_14 < 30 AND Close > EMA_20 AND Volume > SMA_Volume_20 * 1.2",
				Action:        "buy",
				Confidence:    80,
				Priority:      1,
				StopLossPct:   &stopLoss,
				TakeProfitPct: &takeProfit,
			},
			{
				Name:          "RSI_Overbought_EMA_Resistance",
				Condition:     "RSI_14 > 70 AND Close < EMA_20 AND Volume > SMA_Volume_20 * 1.2",
				Action:        "sell",
				Confidence:    75,
				Priority:      2,
			},
			{
				Name:       "ATR_Volatility_Breakout",
				Condition:  "ATR_14 > ATR_14_SMA * 1.5 AND Close > Upper_Bollinger_20",
				Action:     "buy",
				Confidence: 70,
				Priority:   3,
			},
		},
		Parameters: ModelParameters{
			LookbackPeriods:      50,
			EntryThreshold:       0,
			ExitThreshold:        0,
			MaxPositionHoldTime:  24,
			MinPositionHoldTime:  2,
			MaxDailyTrades:       5,
		},
		SignalConfig: SignalGenerationConfig{
			SignalType:            "discrete",
			MinConfidence:         70,
			RequireConfirmation:   false,
			ConfirmationDelay:     0,
		},
	}
}
