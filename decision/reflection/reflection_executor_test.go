package reflection

import (
	"nofx/config"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockParameterOptimizer for testing ReflectionExecutor
type MockParameterOptimizer struct {
	AdjustLeverageCalled bool
	LeverageType         string
	NewLeverageValue     int
	UpdatePromptCalled   bool
	NewPrompt            string
	StopTradingCalled    bool
	Err                  error
}

func (m *MockParameterOptimizer) AdjustLeverage(traderID, leverageType string, newValue int) error {
	m.AdjustLeverageCalled = true
	m.LeverageType = leverageType
	m.NewLeverageValue = newValue
	return m.Err
}

func (m *MockParameterOptimizer) UpdatePrompt(traderID, newPrompt string, overrideBase bool) error {
	m.UpdatePromptCalled = true
	m.NewPrompt = newPrompt
	return m.Err
}

func (m *MockParameterOptimizer) StopTrading(traderID string) error {
	m.StopTradingCalled = true
	return m.Err
}

// MockConfigDatabase for testing ReflectionExecutor
type MockConfigDatabase struct {
	SaveParameterChangeCalled    bool
	UpdateReflectionStatusCalled bool
	ReflectionID                 string
	IsApplied                    bool
}

func (m *MockConfigDatabase) SaveParameterChange(change *config.ParameterChangeRecord) error {
	m.SaveParameterChangeCalled = true
	return nil
}

func (m *MockConfigDatabase) UpdateReflectionAppliedStatus(reflectionID string, isApplied bool) error {
	m.UpdateReflectionStatusCalled = true
	m.ReflectionID = reflectionID
	m.IsApplied = isApplied
	return nil
}

func (m *MockConfigDatabase) GetTraderByID(traderID string) (*config.TraderRecord, error) {
	return nil, nil
}

func TestReflectionExecutor_ApplyReflection(t *testing.T) {
	traderID := "test_trader"
	reflectionID := "reflection_1"

	tests := []struct {
		name              string
		reflection        LearningReflection
		expectedOptimizer func(mo *MockParameterOptimizer)
		expectedDB        func(mdb *MockConfigDatabase)
		expectError       bool
	}{
		{
			name: "Adjust Leverage - BTCETH",
			reflection: LearningReflection{
				ID:                reflectionID,
				TraderID:          traderID,
				RecommendedAction: "将BTC杠杆降低至15倍",
			},
			expectedOptimizer: func(mo *MockParameterOptimizer) {
				assert.True(t, mo.AdjustLeverageCalled)
				assert.Equal(t, "BTCETHLeverage", mo.LeverageType)
				assert.Equal(t, 15, mo.NewLeverageValue)
			},
			expectedDB: func(mdb *MockConfigDatabase) {
				assert.True(t, mdb.SaveParameterChangeCalled)
				assert.True(t, mdb.UpdateReflectionStatusCalled)
				assert.Equal(t, reflectionID, mdb.ReflectionID)
				assert.True(t, mdb.IsApplied)
			},
			expectError: false,
		},
		{
			name: "Stop Trading",
			reflection: LearningReflection{
				ID:                reflectionID,
				TraderID:          traderID,
				RecommendedAction: "停止交易",
			},
			expectedOptimizer: func(mo *MockParameterOptimizer) {
				assert.True(t, mo.StopTradingCalled)
			},
			expectedDB: func(mdb *MockConfigDatabase) {
				assert.True(t, mdb.SaveParameterChangeCalled)
				assert.True(t, mdb.UpdateReflectionStatusCalled)
			},
			expectError: false,
		},
		{
			name: "Unrecognized Action",
			reflection: LearningReflection{
				ID:                reflectionID,
				TraderID:          traderID,
				RecommendedAction: "Do Something Crazy",
			},
			expectedOptimizer: func(mo *MockParameterOptimizer) {
				assert.False(t, mo.AdjustLeverageCalled)
				assert.False(t, mo.StopTradingCalled)
			},
			expectedDB: func(mdb *MockConfigDatabase) {
				assert.False(t, mdb.SaveParameterChangeCalled)
				assert.False(t, mdb.UpdateReflectionStatusCalled) // Should not update if action failed
			},
			expectError: true,
		},
		{
			name: "Already Applied",
			reflection: LearningReflection{
				ID:                reflectionID,
				TraderID:          traderID,
				RecommendedAction: "停止交易",
				IsApplied:         true,
			},
			expectedOptimizer: func(mo *MockParameterOptimizer) {
				assert.False(t, mo.StopTradingCalled) // Should not call optimizer
			},
			expectedDB: func(mdb *MockConfigDatabase) {
				assert.False(t, mdb.SaveParameterChangeCalled)
				assert.False(t, mdb.UpdateReflectionStatusCalled)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOptimizer := &MockParameterOptimizer{}
			mockDB := &MockConfigDatabase{}
			executor := NewReflectionExecutor(mockDB, mockOptimizer)

			err := executor.ApplyReflection(tt.reflection)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			tt.expectedOptimizer(mockOptimizer)
			tt.expectedDB(mockDB)
		})
	}
}
