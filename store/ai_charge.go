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
	"gpt-6":             0.24,
	"gpt-5.6":           0.06,
	"gpt-5.6-terra":     0.03,
	"gpt-5.6-luna":      0.012,
	"claude-fable":      0.24,
	"claude-opus":       0.12,
	"glm-5":             0.003,
}

// GetModelPrice returns the price per call for a given model
func GetModelPrice(model string) float64 {
	if price, ok := modelPrices[model]; ok {
		return price
	}
	return 0.01 // default fallback
}

// modelTokenPrices maps model → USD per 1M input/output tokens, mirroring the
// claw402 gateway pricing config (providers/*.yaml). Used to derive the actual
// upto-settled cost from streamed token usage, where the gateway cannot
// deliver the settlement header (SSE headers are flushed before usage is known).
type modelTokenPrice struct {
	In, Out        float64
	LongContextAt  int
	LongContextIn  float64
	LongContextOut float64
}

var modelTokenPrices = map[string]modelTokenPrice{
	"gpt-6":             {In: 12.5, Out: 50, LongContextAt: 272000, LongContextIn: 25, LongContextOut: 75},
	"gpt-5.6":           {In: 5, Out: 30},
	"gpt-5.6-terra":     {In: 2.5, Out: 15},
	"gpt-5.6-luna":      {In: 1, Out: 6},
	"claude-fable":      {In: 10, Out: 50},
	"claude-opus":       {In: 5, Out: 25},
	"deepseek-v4-flash": {In: 0.14, Out: 0.28},
	"deepseek-v4-pro":   {In: 1.74, Out: 3.48},
	"deepseek":          {In: 0.27, Out: 1.1},
	"deepseek-reasoner": {In: 0.55, Out: 2.19},
	"glm-5":             {In: 0.6, Out: 2},
}

// Gateway upto settlement formula constants (see claw402 token_estimate
// pricing: token_safety_margin 0.15, token_min_price 0.0001).
const (
	uptoSafetyMargin = 1.15
	uptoMinPriceUSD  = 0.0001
)

// ComputeUsageCost derives the upto-settled cost of a call from token usage,
// using the same formula as the claw402 gateway. ok is false for models
// without a token price entry.
func ComputeUsageCost(model string, promptTokens, completionTokens int) (float64, bool) {
	p, ok := modelTokenPrices[model]
	if !ok {
		return 0, false
	}
	inputPrice, outputPrice := p.In, p.Out
	if p.LongContextAt > 0 && promptTokens > p.LongContextAt {
		inputPrice, outputPrice = p.LongContextIn, p.LongContextOut
	}
	cost := (float64(promptTokens)*inputPrice + float64(completionTokens)*outputPrice) / 1e6 * uptoSafetyMargin
	if cost < uptoMinPriceUSD {
		cost = uptoMinPriceUSD
	}
	return cost, true
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
	return s.RecordWithCost(traderID, model, provider, GetModelPrice(model))
}

// RecordWithCost records a charge with an explicit cost — e.g. the actual
// settled amount reported by the payment gateway (upto scheme) — instead of
// the flat per-call estimate from modelPrices.
func (s *AIChargeStore) RecordWithCost(traderID, model, provider string, costUSD float64) error {
	charge := &AICharge{
		TraderID: traderID,
		Model:    model,
		Provider: provider,
		CostUSD:  costUSD,
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
		scanIntervalMinutes = 15
	}
	callsPerDay := float64(24*60) / float64(scanIntervalMinutes)
	pricePerCall := GetModelPrice(modelName)
	dailyCost = callsPerDay * pricePerCall
	if dailyCost > 0 && usdcBalance > 0 {
		runwayDays = usdcBalance / dailyCost
	}
	return dailyCost, runwayDays
}
