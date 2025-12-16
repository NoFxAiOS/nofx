package reflection

import (
	"encoding/json"
	"fmt"
	"nofx/decision/analysis"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ReflectionGenerator generates reflections using AI.
type ReflectionGenerator struct {
	aiClient AIClient
}

// NewReflectionGenerator creates a new ReflectionGenerator.
func NewReflectionGenerator(client AIClient) *ReflectionGenerator {
	return &ReflectionGenerator{aiClient: client}
}

// GenerateReflections analyzes the data and produces reflections.
func (rg *ReflectionGenerator) GenerateReflections(traderID string, stats *analysis.TradeAnalysisResult, patterns []analysis.FailurePattern) ([]LearningReflection, error) {
	// 1. Construct Prompt
	prompt := rg.buildPrompt(stats, patterns)

	// 2. Call AI
	response, err := rg.aiClient.GenerateCompletion(prompt)
	if err != nil {
		return nil, fmt.Errorf("AI call failed: %w", err)
	}

	// 3. Parse Response
	return rg.parseResponse(traderID, response)
}

func (rg *ReflectionGenerator) buildPrompt(stats *analysis.TradeAnalysisResult, patterns []analysis.FailurePattern) string {
	var sb strings.Builder
	sb.WriteString("You are a Trading Performance Analyst. Analyze the following trading statistics and failure patterns.\n")
	sb.WriteString("Generate specific, actionable 'Reflections' to improve performance.\n\n")

	// Stats
	sb.WriteString(fmt.Sprintf("STATS:\n"))
	sb.WriteString(fmt.Sprintf("- Win Rate: %.2f%%\n", stats.WinRate))
	sb.WriteString(fmt.Sprintf("- Profit Factor: %.2f\n", stats.ProfitFactor))
	sb.WriteString(fmt.Sprintf("- Total Trades: %d\n", stats.TotalTrades))
	sb.WriteString(fmt.Sprintf("- Best Pair: %s\n", stats.BestPerformingPair))
	sb.WriteString(fmt.Sprintf("- Worst Pair: %s\n", stats.WorstPerformingPair))
	sb.WriteString("\n")

	// Patterns
	sb.WriteString("DETECTED PATTERNS:\n")
	if len(patterns) == 0 {
		sb.WriteString("No specific failure patterns detected.\n")
	} else {
		for _, p := range patterns {
			sb.WriteString(fmt.Sprintf("- %s: %s (Impact: $%.2f)\n", p.PatternType, p.Description, p.ImpactLoss))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("OUTPUT FORMAT: Provide a JSON array of objects with keys: 'reflection_type', 'severity', 'problem_title', 'problem_description', 'root_cause', 'recommended_action', 'priority' (1-10), 'expected_improvement' (float percentage).\n")
	sb.WriteString("Example: [{\"reflection_type\": \"risk\", \"problem_title\": \"High Leverage\", ...}]")

	return sb.String()
}

func (rg *ReflectionGenerator) parseResponse(traderID string, response string) ([]LearningReflection, error) {
	// Clean response (strip markdown code blocks if any)
	cleaned := strings.TrimSpace(response)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")

	var rawReflections []struct {
		ReflectionType      string  `json:"reflection_type"`
		Severity            string  `json:"severity"`
		ProblemTitle        string  `json:"problem_title"`
		ProblemDescription  string  `json:"problem_description"`
		RootCause           string  `json:"root_cause"`
		RecommendedAction   string  `json:"recommended_action"`
		Priority            int     `json:"priority"`
		ExpectedImprovement float64 `json:"expected_improvement"`
	}

	if err := json.Unmarshal([]byte(cleaned), &rawReflections); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	var results []LearningReflection
	for _, r := range rawReflections {
		results = append(results, LearningReflection{
			ID:                  uuid.New().String(),
			TraderID:            traderID,
			ReflectionType:      r.ReflectionType,
			Severity:            r.Severity,
			ProblemTitle:        r.ProblemTitle,
			ProblemDescription:  r.ProblemDescription,
			RootCause:           r.RootCause,
			RecommendedAction:   r.RecommendedAction,
			Priority:            r.Priority,
			ExpectedImprovement: r.ExpectedImprovement,
			CreatedAt:           time.Now(),
		})
	}

	return results, nil
}
