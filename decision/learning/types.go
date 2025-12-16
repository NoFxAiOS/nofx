package learning

import (
	"nofx/config"
	"nofx/decision/analysis"
	"nofx/decision/reflection"
	"time"
)

// ConfigDB defines database operations required by ParameterOptimizer.
type ConfigDB interface {
	GetTraderByID(traderID string) (*config.TraderRecord, error)
	UpdateTrader(trader *config.TraderRecord) error
	UpdateTraderCustomPrompt(userID, traderID, prompt string, override bool) error
	UpdateTraderStatus(traderID string, isRunning bool) error
	SaveReflection(reflection *config.ReflectionRecord) error
	GetActiveTraders() ([]*config.TraderRecord, error)
}

// Analyzer interface
type Analyzer interface {
	AnalyzeTradesForPeriod(traderID string, start, end time.Time) (*analysis.TradeAnalysisResult, error)
}

// Detector interface
type Detector interface {
	DetectFailurePatterns(stats *analysis.TradeAnalysisResult) []analysis.FailurePattern
}

// Generator interface
type Generator interface {
	GenerateReflections(traderID string, stats *analysis.TradeAnalysisResult, patterns []analysis.FailurePattern) ([]reflection.LearningReflection, error)
}

// Executor interface
type Executor interface {
	ApplyReflection(reflection reflection.LearningReflection) error
}

// TraderController defines operations on a running trader instance.
type TraderController interface {
	SetLeverage(btcEth, altcoin int)
	SetCustomPrompt(prompt string)
	SetOverrideBasePrompt(override bool)
	Stop()
}

// TraderManager defines access to running traders.
type TraderManager interface {
	GetTraderController(id string) (TraderController, error)
}