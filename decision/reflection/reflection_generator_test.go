package reflection

import (
	"nofx/decision/analysis"
	"testing"
)

type MockAIClient struct {
	Response string
	Err      error
}

func (m *MockAIClient) GenerateCompletion(prompt string) (string, error) {
	return m.Response, m.Err
}

func TestReflectionGenerator_GenerateReflections(t *testing.T) {
	mockResponse := `
	[
		{
			"reflection_type": "risk",
			"severity": "high",
			"problem_title": "High Leverage",
			"problem_description": "Excessive leverage detected.",
			"root_cause": "Aggressive settings",
			"recommended_action": "Reduce leverage to 5x",
			"priority": 9,
			"expected_improvement": 20.0
		}
	]`

	client := &MockAIClient{Response: mockResponse}
	generator := NewReflectionGenerator(client)

	stats := &analysis.TradeAnalysisResult{
		WinRate: 40.0,
		ProfitFactor: 0.8,
	}
	patterns := []analysis.FailurePattern{
		{PatternType: "high_risk", Description: "High leverage usage", ImpactLoss: 100},
	}

	reflections, err := generator.GenerateReflections("trader_1", stats, patterns)
	if err != nil {
		t.Fatalf("GenerateReflections failed: %v", err)
	}

	if len(reflections) != 1 {
		t.Errorf("Expected 1 reflection, got %d", len(reflections))
	}
	if reflections[0].ProblemTitle != "High Leverage" {
		t.Errorf("Unexpected title: %s", reflections[0].ProblemTitle)
	}
	if reflections[0].TraderID != "trader_1" {
		t.Errorf("Unexpected TraderID: %s", reflections[0].TraderID)
	}
}
