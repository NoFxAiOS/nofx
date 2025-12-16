package reflection

import (
	"nofx/config"
	"time"
)

// AIClient defines the interface for AI providers.
type AIClient interface {
	GenerateCompletion(prompt string) (string, error)
}

// ExecutorDB defines database operations required by ReflectionExecutor.
type ExecutorDB interface {
	GetTraderByID(traderID string) (*config.TraderRecord, error)
	UpdateReflectionAppliedStatus(reflectionID string, isApplied bool) error
	SaveParameterChange(change *config.ParameterChangeRecord) error
}

// Optimizer defines operations for parameter optimization.
type Optimizer interface {
	AdjustLeverage(traderID, leverageType string, newValue int) error
	UpdatePrompt(traderID, newPrompt string, overrideBase bool) error
	StopTrading(traderID string) error
}

// LearningReflection represents a generated insight and improvement suggestion.
type LearningReflection struct {
	ID                  string
	TraderID            string
	ReflectionType      string // 'strategy', 'risk', 'timing', 'pattern'
	Severity            string // 'critical', 'high', 'medium', 'low'
	ProblemTitle        string
	ProblemDescription  string
	RootCause           string
	RecommendedAction   string
	Priority            int     // 1-10
	ExpectedImprovement float64 // Percentage
	IsApplied           bool
	CreatedAt           time.Time
}
