package optimizer

import (
	"fmt"
	"log"
)

// ParameterOptimizer handles applying changes to trader parameters.
type ParameterOptimizer struct {
	db            ConfigDB
	traderManager TraderManager
}

// NewParameterOptimizer creates a new ParameterOptimizer.
func NewParameterOptimizer(db ConfigDB, tm TraderManager) *ParameterOptimizer {
	return &ParameterOptimizer{
		db:            db,
		traderManager: tm,
	}
}

// AdjustLeverage updates the leverage settings for a specific trader.
func (po *ParameterOptimizer) AdjustLeverage(traderID, leverageType string, newValue int) error {
	// 1. Get current trader config
	traderRecord, err := po.db.GetTraderByID(traderID)
	if err != nil {
		return fmt.Errorf("failed to get trader %s: %w", traderID, err)
	}

	// 2. Apply change
	if leverageType == "BTCETHLeverage" {
		traderRecord.BTCETHLeverage = newValue
	} else if leverageType == "AltcoinLeverage" {
		traderRecord.AltcoinLeverage = newValue
	} else {
		return fmt.Errorf("invalid leverage type: %s", leverageType)
	}

	// 3. Update database
	err = po.db.UpdateTrader(traderRecord)
	if err != nil {
		return fmt.Errorf("failed to update trader %s leverage in DB: %w", traderID, err)
	}

	// 4. Update in-memory trader (if running)
	if trader, err := po.traderManager.GetTraderController(traderID); err == nil {
		trader.SetLeverage(traderRecord.BTCETHLeverage, traderRecord.AltcoinLeverage)
		log.Printf("✓ Trader %s leverage updated to %s=%d in-memory.", traderID, leverageType, newValue)
	} else {
		log.Printf("⚠️ Trader %s not found in-memory, leverage update will apply on next load.", traderID)
	}

	log.Printf("✅ Trader %s: Adjusted %s to %d.", traderID, leverageType, newValue)
	return nil
}

// UpdatePrompt updates the custom prompt for a specific trader.
func (po *ParameterOptimizer) UpdatePrompt(traderID, newPrompt string, overrideBase bool) error {
	// 1. Get current trader config to get UserID
	traderRecord, err := po.db.GetTraderByID(traderID)
	if err != nil {
		return fmt.Errorf("failed to get trader %s: %w", traderID, err)
	}

	// 2. Update database
	err = po.db.UpdateTraderCustomPrompt(traderRecord.UserID, traderID, newPrompt, overrideBase)
	if err != nil {
		return fmt.Errorf("failed to update trader %s prompt in DB: %w", traderID, err)
	}

	// 3. Update in-memory trader (if running)
	if trader, err := po.traderManager.GetTraderController(traderID); err == nil {
		trader.SetCustomPrompt(newPrompt)
		trader.SetOverrideBasePrompt(overrideBase)
		log.Printf("✓ Trader %s prompt updated in-memory.", traderID)
	} else {
		log.Printf("⚠️ Trader %s not found in-memory, prompt update will apply on next load.", traderID)
	}

	log.Printf("✅ Trader %s: Updated custom prompt (override=%t).", traderID, overrideBase)
	return nil
}

// StopTrading stops a specific trader.
func (po *ParameterOptimizer) StopTrading(traderID string) error {
	// 1. Stop in-memory trader
	if trader, err := po.traderManager.GetTraderController(traderID); err == nil {
		trader.Stop()
		log.Printf("✓ Trader %s stopped in-memory.", traderID)
	} else {
		log.Printf("⚠️ Trader %s not found in-memory, cannot stop directly.", traderID)
	}

	// 2. Update database status
	err := po.db.UpdateTraderStatus(traderID, false)
	if err != nil {
		return fmt.Errorf("failed to update trader %s status in DB: %w", traderID, err)
	}

	log.Printf("✅ Trader %s: Stopped trading.", traderID)
	return nil
}
