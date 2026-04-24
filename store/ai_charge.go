package store

import (
	"time"

	"gorm.io/gorm"
)

// AICharge represents a single AI call charge record
type AICharge struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TraderID  string    `gorm:"column:trader_id;not null;index:idx_ai_charges_trader" json:"trader_id"`
	Model     string    `gorm:"column:model;not null" json:"model"`
	Provider  string    `gorm:"column:provider;not null" json:"provider"`
	CostUSD   float64   `gorm:"column:cost_usd;not null" json:"cost_usd"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (AICharge) TableName() string { return "ai_charges" }

// modelPrices maps model ID to approximate cost per call in USD
var modelPrices = map[string]float64{
	"deepseek":          0.003,
	"deepseek-reasoner": 0.005,
	"deepseek-v4-flash": 0.003,
	"deepseek-v4-pro":   0.01,
	"gpt-5.4":           0.05,
	"gpt-5.4-pro":       0.50,
	"gpt-5.3":           0.01,
	"gpt-5-mini":        0.005,
	"claude-opus":       0.12,
	"qwen-max":          0.01,
	"qwen-plus":         0.005,
	"qwen-turbo":        0.002,
	"qwen-flash":        0.002,
	"grok-4.1":          0.06,
	"gemini-3.1-pro":    0.03,
	"kimi-k2.5":         0.008,
}

// GetModelPrice returns the price per call for a given model
func GetModelPrice(model string) float64 {
	if price, ok := modelPrices[model]; ok {
		return price
	}
	return 0.01 // default fallback
}

// AIChargeStore handles AI charge records
type AIChargeStore struct {
	db *gorm.DB
}

// NewAIChargeStore creates a new AIChargeStore
func NewAIChargeStore(db *gorm.DB) *AIChargeStore {
	return &AIChargeStore{db: db}
}

func (s *AIChargeStore) initTables() error {
	return s.db.AutoMigrate(&AICharge{})
}

// Record records a new AI charge
func (s *AIChargeStore) Record(traderID, model, provider string) error {
	cost := GetModelPrice(model)
	charge := &AICharge{
		TraderID: traderID,
		Model:    model,
		Provider: provider,
		CostUSD:  cost,
	}
	return s.db.Create(charge).Error
}

// GetCharges returns charges for a trader within a period, plus total cost
func (s *AIChargeStore) GetCharges(traderID string, period string) ([]AICharge, float64, error) {
	var charges []AICharge
	query := s.db.Where("trader_id = ?", traderID)
	query = applyPeriodFilter(query, period)
	if err := query.Order("created_at DESC").Find(&charges).Error; err != nil {
		return nil, 0, err
	}

	var total float64
	for _, c := range charges {
		total += c.CostUSD
	}
	return charges, total, nil
}

// GetDailyCost returns total cost across all traders for a period
func (s *AIChargeStore) GetDailyCost(period string) float64 {
	var total float64
	query := s.db.Model(&AICharge{}).Select("COALESCE(SUM(cost_usd), 0)")
	query = applyPeriodFilter(query, period)
	query.Scan(&total)
	return total
}

// GetSummary returns summary stats for a period
func (s *AIChargeStore) GetSummary(period string) (total float64, count int64, byModel map[string]float64) {
	byModel = make(map[string]float64)

	query := s.db.Model(&AICharge{})
	query = applyPeriodFilter(query, period)
	query.Count(&count)

	query2 := s.db.Model(&AICharge{}).Select("COALESCE(SUM(cost_usd), 0)")
	query2 = applyPeriodFilter(query2, period)
	query2.Scan(&total)

	// By model breakdown
	type modelCost struct {
		Model string  `gorm:"column:model"`
		Total float64 `gorm:"column:total"`
	}
	var results []modelCost
	query3 := s.db.Model(&AICharge{}).Select("model, SUM(cost_usd) as total").Group("model")
	query3 = applyPeriodFilter(query3, period)
	query3.Find(&results)
	for _, r := range results {
		byModel[r.Model] = r.Total
	}

	return total, count, byModel
}

func applyPeriodFilter(query *gorm.DB, period string) *gorm.DB {
	now := time.Now()
	switch period {
	case "today":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return query.Where("created_at >= ?", start)
	case "week":
		return query.Where("created_at >= ?", now.AddDate(0, 0, -7))
	case "month":
		return query.Where("created_at >= ?", now.AddDate(0, -1, 0))
	case "all":
		return query
	default:
		// Try parse as date
		if t, err := time.Parse("2006-01-02", period); err == nil {
			end := t.AddDate(0, 0, 1)
			return query.Where("created_at >= ? AND created_at < ?", t, end)
		}
		// Default to today
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return query.Where("created_at >= ?", start)
	}
}

// IsClaw402Config checks if a trader config uses claw402 payment provider
func IsClaw402Config(aiModel string) bool {
	return aiModel == "claw402"
}

// EstimateRunway estimates how many days the given USDC balance will last
func EstimateRunway(usdcBalance float64, modelName string, scanIntervalMinutes int) (dailyCost float64, runwayDays float64) {
	if scanIntervalMinutes <= 0 {
		scanIntervalMinutes = 3
	}
	callsPerDay := float64(24*60) / float64(scanIntervalMinutes)
	pricePerCall := GetModelPrice(modelName)
	dailyCost = callsPerDay * pricePerCall
	if dailyCost > 0 && usdcBalance > 0 {
		runwayDays = usdcBalance / dailyCost
	}
	return dailyCost, runwayDays
}
