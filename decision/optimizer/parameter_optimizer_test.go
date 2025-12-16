package optimizer

import (
	"fmt"
	"nofx/config"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockConfigDatabase implements ConfigDB interface for testing.
type MockConfigDatabase struct {
	Traders         map[string]*config.TraderRecord
	UpdatedTrader   *config.TraderRecord
	UpdatedPrompt   string
	UpdatedOverride bool
	UpdatedStatus   bool
}

func (m *MockConfigDatabase) GetTraderByID(traderID string) (*config.TraderRecord, error) {
	if t, ok := m.Traders[traderID]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("trader not found")
}

func (m *MockConfigDatabase) UpdateTrader(trader *config.TraderRecord) error {
	m.UpdatedTrader = trader
	m.Traders[trader.ID] = trader
	return nil
}

func (m *MockConfigDatabase) UpdateTraderCustomPrompt(userID, traderID, prompt string, override bool) error {
	m.UpdatedPrompt = prompt
	m.UpdatedOverride = override
	// Also update the mock trader for consistency
	if t, ok := m.Traders[traderID]; ok {
		t.CustomPrompt = prompt
		t.OverrideBasePrompt = override
	}
	return nil
}

func (m *MockConfigDatabase) UpdateTraderStatus(traderID string, isRunning bool) error {
	m.UpdatedStatus = isRunning
	if t, ok := m.Traders[traderID]; ok {
		t.IsRunning = isRunning
	}
	return nil
}

// MockAutoTrader implements TraderController interface for testing.
type MockAutoTrader struct {
	NameVal               string
	LeverageBTCETH        int
	LeverageAltcoin       int
	CustomPromptVal       string
	OverrideBasePromptVal bool
	Stopped               bool
}

func (m *MockAutoTrader) SetLeverage(btcEth, altcoin int) {
	m.LeverageBTCETH = btcEth
	m.LeverageAltcoin = altcoin
}
func (m *MockAutoTrader) SetCustomPrompt(prompt string)       { m.CustomPromptVal = prompt }
func (m *MockAutoTrader) SetOverrideBasePrompt(override bool) { m.OverrideBasePromptVal = override }
func (m *MockAutoTrader) Stop()                               { m.Stopped = true }

// MockTraderManager implements TraderManager interface for testing.
type MockTraderManager struct {
	Traders map[string]*MockAutoTrader
}

func (m *MockTraderManager) GetTraderController(id string) (TraderController, error) {
	if t, ok := m.Traders[id]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("trader not found in manager")
}

func TestParameterOptimizer_AdjustLeverage(t *testing.T) {
	traderID := "test_trader"
	mockTrader := &config.TraderRecord{
		ID:              traderID,
		BTCETHLeverage:  10,
		AltcoinLeverage: 20,
	}
	mockDB := &MockConfigDatabase{
		Traders: map[string]*config.TraderRecord{traderID: mockTrader},
	}
	mockAutoTrader := &MockAutoTrader{NameVal: traderID, LeverageBTCETH: 10, LeverageAltcoin: 20}
	mockTM := &MockTraderManager{
		Traders: map[string]*MockAutoTrader{traderID: mockAutoTrader},
	}

	optimizer := NewParameterOptimizer(mockDB, mockTM)

	// Test BTCETH leverage adjustment
	err := optimizer.AdjustLeverage(traderID, "BTCETHLeverage", 15)
	assert.NoError(t, err)
	assert.Equal(t, 15, mockDB.UpdatedTrader.BTCETHLeverage)
	assert.Equal(t, 15, mockAutoTrader.LeverageBTCETH)

	// Test Altcoin leverage adjustment
	err = optimizer.AdjustLeverage(traderID, "AltcoinLeverage", 10)
	assert.NoError(t, err)
	assert.Equal(t, 10, mockDB.UpdatedTrader.AltcoinLeverage)
	assert.Equal(t, 10, mockAutoTrader.LeverageAltcoin)

	// Test invalid leverage type
	err = optimizer.AdjustLeverage(traderID, "InvalidLeverage", 5)
	assert.Error(t, err)
}

func TestParameterOptimizer_UpdatePrompt(t *testing.T) {
	traderID := "test_trader"
	mockTrader := &config.TraderRecord{
		ID:           traderID,
		CustomPrompt: "old prompt",
	}
	mockDB := &MockConfigDatabase{
		Traders: map[string]*config.TraderRecord{traderID: mockTrader},
	}
	mockAutoTrader := &MockAutoTrader{NameVal: traderID, CustomPromptVal: "old prompt"}
	mockTM := &MockTraderManager{
		Traders: map[string]*MockAutoTrader{traderID: mockAutoTrader},
	}

	optimizer := NewParameterOptimizer(mockDB, mockTM)

	newPrompt := "new prompt"
	err := optimizer.UpdatePrompt(traderID, newPrompt, true)
	assert.NoError(t, err)
	assert.Equal(t, newPrompt, mockDB.UpdatedPrompt)
	assert.True(t, mockDB.UpdatedOverride)
	assert.Equal(t, newPrompt, mockAutoTrader.CustomPromptVal)
	assert.True(t, mockAutoTrader.OverrideBasePromptVal)
}

func TestParameterOptimizer_StopTrading(t *testing.T) {
	traderID := "test_trader"
	mockTrader := &config.TraderRecord{ID: traderID, IsRunning: true}
	mockDB := &MockConfigDatabase{
		Traders: map[string]*config.TraderRecord{traderID: mockTrader},
	}
	mockAutoTrader := &MockAutoTrader{NameVal: traderID, Stopped: false}
	mockTM := &MockTraderManager{
		Traders: map[string]*MockAutoTrader{traderID: mockAutoTrader},
	}

	optimizer := NewParameterOptimizer(mockDB, mockTM)

	err := optimizer.StopTrading(traderID)
	assert.NoError(t, err)
	assert.False(t, mockDB.UpdatedStatus)  // Check DB updated (to false)
	assert.True(t, mockAutoTrader.Stopped) // Check in-memory stopped
}
