package reflection

import (
	"fmt"
	"log"
	"nofx/config"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ReflectionExecutor applies recommended actions from LearningReflections.
type ReflectionExecutor struct {
	db        ExecutorDB
	optimizer Optimizer
}

// NewReflectionExecutor creates a new ReflectionExecutor.
func NewReflectionExecutor(db ExecutorDB, optimizer Optimizer) *ReflectionExecutor {
	return &ReflectionExecutor{
		db:        db,
		optimizer: optimizer,
	}
}

// ApplyReflection executes the recommended action of a given reflection.
func (re *ReflectionExecutor) ApplyReflection(reflection LearningReflection) error {
	// Only apply if not already applied
	if reflection.IsApplied {
		log.Printf("ℹ️ Reflection %s for trader %s already applied. Skipping.", reflection.ID, reflection.TraderID)
		return nil
	}

	// Parse recommended action
	action := strings.ToLower(reflection.RecommendedAction)
	traderID := reflection.TraderID
	var err error
	var changeRecorded bool

	// Example parsing of RecommendedAction. This part might need to be more robust.
	// For KISS, a few common patterns are handled.
	if strings.Contains(action, "leverage") || strings.Contains(action, "杠杆") {
		// Example: "将BTC杠杆降低至15倍"
		// Regex to extract number before "倍" or "x"
		reg := regexp.MustCompile(`(\d+)(倍|x)`)
		matches := reg.FindStringSubmatch(action)
		
		if len(matches) > 1 {
			if val, convErr := strconv.Atoi(matches[1]); convErr == nil {
				if strings.Contains(action, "btc") || strings.Contains(action, "eth") {
					err = re.optimizer.AdjustLeverage(traderID, "BTCETHLeverage", val)
					// Record change
					re.recordParameterChange(traderID, reflection.ID, "BTCETHLeverage", fmt.Sprintf("%d", val), reflection.RecommendedAction)
					changeRecorded = true
				} else if strings.Contains(action, "altcoin") || strings.Contains(action, "山寨") {
					err = re.optimizer.AdjustLeverage(traderID, "AltcoinLeverage", val)
					re.recordParameterChange(traderID, reflection.ID, "AltcoinLeverage", fmt.Sprintf("%d", val), reflection.RecommendedAction)
					changeRecorded = true
				}
			}
		}
	} else if strings.Contains(action, "stop trading") || strings.Contains(action, "停止交易") {
		err = re.optimizer.StopTrading(traderID)
		re.recordParameterChange(traderID, reflection.ID, "IsRunning", "false", reflection.RecommendedAction)
		changeRecorded = true
	} else if strings.Contains(action, "prompt") || strings.Contains(action, "提示词") {
		// This needs Phase 2.5: Prompt Evolution
		// For now, it might involve updating CustomPrompt in the future.
		// For simplicity, we assume RecommendedAction contains the full new prompt if it's a prompt update
		// And we assume it always overrides for now
		newPrompt := reflection.RecommendedAction // Placeholder
		err = re.optimizer.UpdatePrompt(traderID, newPrompt, true)
		re.recordParameterChange(traderID, reflection.ID, "CustomPrompt", newPrompt, reflection.RecommendedAction)
		changeRecorded = true
	} else {
		log.Printf("⚠️  Reflection %s: Unrecognized action '%s'. Manual intervention required.", reflection.ID, reflection.RecommendedAction)
		return fmt.Errorf("unrecognized action: %s", reflection.RecommendedAction)
	}

	if err != nil {
		return fmt.Errorf("failed to execute action '%s' for trader %s: %w", reflection.RecommendedAction, traderID, err)
	}

	// Mark reflection as applied in DB (only if action taken)
	if changeRecorded {
		err = re.db.UpdateReflectionAppliedStatus(reflection.ID, true)
		if err != nil {
			log.Printf("❌ Failed to update applied status for reflection %s: %v", reflection.ID, err)
		}
	}

	return nil
}

// recordParameterChange saves a record of the parameter change to the database.
func (re *ReflectionExecutor) recordParameterChange(traderID, reflectionID, paramName, newValue, reason string) {
	// Get old value for context
	oldValue := "N/A"
	if trader, err := re.db.GetTraderByID(traderID); err == nil && trader != nil {
		// This is very specific to "leverage" - a more generic approach would be needed for other params
		if paramName == "BTCETHLeverage" {
			oldValue = strconv.Itoa(trader.BTCETHLeverage)
		} else if paramName == "AltcoinLeverage" {
			oldValue = strconv.Itoa(trader.AltcoinLeverage)
		} else if paramName == "IsRunning" {
			oldValue = strconv.FormatBool(trader.IsRunning)
		} else if paramName == "CustomPrompt" {
			oldValue = trader.CustomPrompt
		}
	}

	change := &config.ParameterChangeRecord{
		ID:           uuid.New().String(),
		TraderID:     traderID,
		ReflectionID: reflectionID,
		ParameterName: paramName,
		OldValue:     oldValue,
		NewValue:     newValue,
		ChangeReason: reason,
		CreatedAt:    time.Now(),
	}

	if err := re.db.SaveParameterChange(change); err != nil {
		log.Printf("❌ Failed to save parameter change history for reflection %s: %v", reflectionID, err)
	} else {
		log.Printf("✓ Parameter change recorded: %s for trader %s. Old: %s, New: %s", paramName, traderID, oldValue, newValue)
	}
}
