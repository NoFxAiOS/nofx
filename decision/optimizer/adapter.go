package optimizer

import (
	"fmt"
	"nofx/manager"
)

// RealTraderManager adapts manager.TraderManager to optimizer.TraderManager interface.
type RealTraderManager struct {
	tm *manager.TraderManager
}

func NewRealTraderManager(tm *manager.TraderManager) *RealTraderManager {
	return &RealTraderManager{tm: tm}
}

func (r *RealTraderManager) GetTraderController(id string) (TraderController, error) {
	t, err := r.tm.GetTrader(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get trader: %w", err)
	}
	return t, nil
}
