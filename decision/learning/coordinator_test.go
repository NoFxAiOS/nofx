package learning

import (
	"errors"
	"nofx/config"
	"nofx/decision/analysis"
	"nofx/decision/reflection"
	"testing"
	"time"
)

// MockAnalyzer
type MockAnalyzer struct {
	Err error
}

func (m *MockAnalyzer) AnalyzeTradesForPeriod(id string, start, end time.Time) (*analysis.TradeAnalysisResult, error) {
	return &analysis.TradeAnalysisResult{}, m.Err
}

// MockDetector
type MockDetector struct{}

func (m *MockDetector) DetectFailurePatterns(stats *analysis.TradeAnalysisResult) []analysis.FailurePattern {
	return []analysis.FailurePattern{}
}

// MockGenerator
type MockGenerator struct {
	Err error
}

func (m *MockGenerator) GenerateReflections(id string, stats *analysis.TradeAnalysisResult, patterns []analysis.FailurePattern) ([]reflection.LearningReflection, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return []reflection.LearningReflection{{ID: "1", Priority: 9}}, nil
}

// MockExecutor
type MockExecutor struct{}

func (m *MockExecutor) ApplyReflection(r reflection.LearningReflection) error { return nil }

// MockConfigDB
type MockConfigDB struct {
	SaveErr error
}

func (m *MockConfigDB) SaveReflection(r *config.ReflectionRecord) error { return m.SaveErr }
// Stub other methods
func (m *MockConfigDB) GetTraderByID(id string) (*config.TraderRecord, error) { return nil, nil }
func (m *MockConfigDB) UpdateTrader(t *config.TraderRecord) error { return nil }
func (m *MockConfigDB) UpdateTraderCustomPrompt(uid, tid, p string, o bool) error { return nil }
func (m *MockConfigDB) UpdateTraderStatus(tid string, r bool) error { return nil }
func (m *MockConfigDB) GetActiveTraders() ([]*config.TraderRecord, error) { return nil, nil }

func TestRunLearningCycle_Robustness(t *testing.T) {
	// Scenario 1: Analyzer Failure
	coord := &LearningCoordinator{
		analyzer: &MockAnalyzer{Err: errors.New("db error")},
	}
	if err := coord.RunLearningCycle("t1"); err == nil {
		t.Error("Expected error from analyzer failure")
	}

	// Scenario 2: Generator Failure (AI Down)
	coord = &LearningCoordinator{
		analyzer:  &MockAnalyzer{},
		detector:  &MockDetector{},
		generator: &MockGenerator{Err: errors.New("ai down")},
	}
	if err := coord.RunLearningCycle("t1"); err == nil {
		t.Error("Expected error from generator failure")
	}

	// Scenario 3: Save DB Failure (Should NOT panic, log error and continue)
    // Wait, RunLearningCycle logic:
    /*
    for _, r := range reflections {
        if err := lc.db.SaveReflection(&r); err != nil {
            log.Printf...
            continue
        }
    }
    */
    // It continues. It returns nil at end.
    
	coord = &LearningCoordinator{
		analyzer:  &MockAnalyzer{},
		detector:  &MockDetector{},
		generator: &MockGenerator{},
		executor:  &MockExecutor{},
		db:        &MockConfigDB{SaveErr: errors.New("save fail")},
	}
	if err := coord.RunLearningCycle("t1"); err != nil {
		t.Errorf("Expected success (graceful degradation) on save fail, got: %v", err)
	}
}
