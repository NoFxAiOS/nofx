package optimizer

import "nofx/config"

// ConfigDB defines database operations required by ParameterOptimizer.
type ConfigDB interface {
	GetTraderByID(traderID string) (*config.TraderRecord, error)
	UpdateTrader(trader *config.TraderRecord) error
	UpdateTraderCustomPrompt(userID, traderID, prompt string, override bool) error
	UpdateTraderStatus(traderID string, isRunning bool) error
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
