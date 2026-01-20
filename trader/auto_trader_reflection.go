package trader

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"nofx/store"
	"strings"
	"time"
)

// ReflectionResult stores the parsed result from AI
type ReflectionResult struct {
	Content string   `json:"content"`
	Score   int      `json:"score"`
	Tags    []string `json:"tags"`
}

// GenerateReflection generates a reflection for a specific closed position
func (at *AutoTrader) GenerateReflection(positionID int64) (*store.Reflection, error) {
	if at.store == nil {
		return nil, fmt.Errorf("store is not initialized")
	}

	// 1. Fetch position details
	var position store.TraderPosition
	if err := at.store.GormDB().First(&position, positionID).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch position: %w", err)
	}

	if position.Status != "CLOSED" {
		return nil, fmt.Errorf("position is not closed")
	}

	// 2. Fetch associated decision logs (closest to entry time)
	// EntryTime is in ms
	entryTime := time.UnixMilli(position.EntryTime)
	// Look for decisions 5 minutes before to 1 minute after entry
	startTime := entryTime.Add(-5 * time.Minute)
	endTime := entryTime.Add(1 * time.Minute)

	// Convert to database records search
	var decisionRecords []*store.DecisionRecordDB
	err := at.store.GormDB().Where("trader_id = ? AND timestamp BETWEEN ? AND ?", at.id, startTime, endTime).
		Order("timestamp DESC").
		Limit(1).
		Find(&decisionRecords).Error

	var decisionContext string
	if err == nil && len(decisionRecords) > 0 {
		rec := decisionRecords[0]
		decisionContext = fmt.Sprintf(`
Original AI Decision Context:
System Prompt: %s
User Prompt: %s
CoT Trace: %s
Reasoning: %s
`, rec.SystemPrompt, rec.InputPrompt, rec.CoTTrace, rec.DecisionJSON)
	} else {
		decisionContext = "Original AI decision context not found."
	}

	// 3. Construct prompt
	duration := time.Duration(position.ExitTime - position.EntryTime) * time.Millisecond
	pnlPct := 0.0
	if position.EntryPrice > 0 {
		if position.Side == "LONG" {
			pnlPct = (position.ExitPrice - position.EntryPrice) / position.EntryPrice * 100 * float64(position.Leverage)
		} else {
			pnlPct = (position.EntryPrice - position.ExitPrice) / position.EntryPrice * 100 * float64(position.Leverage)
		}
	}

	prompt := fmt.Sprintf(`You are an expert trading coach. Analyze the following trade and provide a reflection.

Trade Details:
Symbol: %s
Side: %s
Entry Price: %.4f
Exit Price: %.4f
Quantity: %.4f
PnL: %.2f (%.2f%%)
Entry Time: %s
Exit Time: %s
Duration: %s
Exit Reason: %s

%s

Task:
1. Analyze the entry logic. Was it sound based on the available information?
2. Analyze the exit. Was it premature, late, or optimal?
3. Evaluate the outcome. Was it luck or skill?
4. Provide constructive feedback for future trades.
5. Rate the trade from 1-10.
6. Provide tags (e.g., "FOMO", "Good Entry", "Stop Hunted", "Trend Following", "Counter Trend").

Output Format: JSON only
{
    "content": "Detailed analysis...",
    "score": 8,
    "tags": ["Good Entry", "Premature Exit"]
}
`,
		position.Symbol,
		position.Side,
		position.EntryPrice,
		position.ExitPrice,
		position.Quantity,
		position.RealizedPnL,
		pnlPct,
		entryTime.Format(time.RFC3339),
		time.UnixMilli(position.ExitTime).Format(time.RFC3339),
		duration.String(),
		position.CloseReason,
		decisionContext,
	)

	// 4. Call AI
	logger.Infof("🤔 Generating reflection for position %d (%s %s)", positionID, position.Symbol, position.Side)
	response, err := at.mcpClient.CallWithMessages("You are a helpful trading assistant. Return JSON only.", prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to call AI: %w", err)
	}

	// 5. Parse response
	// Clean markdown code blocks if present
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var result ReflectionResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		logger.Errorf("Failed to parse AI response: %s", response)
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	tagsJSON, _ := json.Marshal(result.Tags)

	// 6. Save reflection
	reflection := &store.Reflection{
		TraderID:   at.id,
		PositionID: positionID,
		Content:    result.Content,
		Score:      result.Score,
		Tags:       string(tagsJSON),
		CreatedAt:  time.Now().UTC(),
	}

	if err := at.store.Reflection().Create(reflection); err != nil {
		return nil, fmt.Errorf("failed to save reflection: %w", err)
	}

	return reflection, nil
}
