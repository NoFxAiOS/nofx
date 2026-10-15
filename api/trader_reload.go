package api

import "nofx/logger"

// Running intent remains in the database while the old client drains. The
// loader checks its current version, so a stop during reload cannot be undone.
func (s *Server) removeTraderForReload(userID, traderID string) {
	if _, err := s.store.Trader().GetForUser(userID, traderID); err != nil {
		return
	}
	cfg, err := s.store.Trader().GetFullConfig(userID, traderID)
	disabled := err != nil || cfg.AIModel == nil || !cfg.AIModel.Enabled || cfg.Exchange == nil || !cfg.Exchange.Enabled
	if disabled {
		if err := s.store.Trader().UpdateStatus(userID, traderID, false); err != nil {
			logger.Errorf("disable reload: %v", err)
			return
		}
		// The trader will not come back: a plain stop cancels resting entries.
		if err := s.traderManager.RemoveTraderAndThen(traderID, nil); err != nil {
			logger.Errorf("trader reload: %v", err)
		}
		return
	}
	// Reloaded right away: keep us_stock resting entries for the new instance.
	if err := s.traderManager.RemoveTraderForReload(traderID); err != nil {
		logger.Errorf("trader reload: %v", err)
	}
}
