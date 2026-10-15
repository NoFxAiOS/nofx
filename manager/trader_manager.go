package manager

import (
	"context"
	"errors"
	"fmt"
	"nofx/logger"
	"nofx/store"
	"nofx/trader"
	"runtime/debug"
	"sort"
	"sync"
	"time"
)

// CompetitionCache competition data cache
type CompetitionCache struct {
	data      map[string]interface{}
	timestamp time.Time
	mu        sync.RWMutex
}

// TraderManager manages multiple trader instances
type TraderManager struct {
	traders              map[string]*trader.AutoTrader // key: trader ID
	loadErrors           map[string]error              // key: trader ID, stores last load error
	competitionCache     *CompetitionCache
	mu                   sync.RWMutex
	loadMu               sync.Mutex
	competitionRefreshMu sync.Mutex
	store                *store.Store // set via SetStore; used by EnsureTraderStarted
	// startFn starts an AutoTrader; nil means (*trader.AutoTrader).Start.
	// Test seam only — production code never sets it. review 2026-10-07 B1-5
	startFn func(*trader.AutoTrader) error
	// accountInfoFn fetches one trader's account info for the competition
	// fan-out; nil means (*trader.AutoTrader).GetAccountInfo. Test seam only —
	// production code never sets it. review 2026-10-07 B2-2
	accountInfoFn func(*trader.AutoTrader) (map[string]interface{}, error)
}

// SetStore wires the store (needed by EnsureTraderStarted to persist state).
func (tm *TraderManager) SetStore(st *store.Store) {
	tm.store = st
}

// NewTraderManager creates a trader manager
func NewTraderManager() *TraderManager {
	return &TraderManager{
		traders:    make(map[string]*trader.AutoTrader),
		loadErrors: make(map[string]error),
		competitionCache: &CompetitionCache{
			data: make(map[string]interface{}),
		},
	}
}

// GetLoadError returns the last load error for a trader
func (tm *TraderManager) GetLoadError(traderID string) error {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.loadErrors[traderID]
}

// GetTrader retrieves a trader by ID
func (tm *TraderManager) GetTrader(id string) (*trader.AutoTrader, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	t, exists := tm.traders[id]
	if !exists {
		return nil, fmt.Errorf("trader ID '%s' does not exist", id)
	}
	return t, nil
}

// StartTrader resolves id and starts the instance currently registered in
// the map, atomically with respect to reloads.
//
// review 2026-10-07 B1-5 (P0-1): handleStartTrader used to fetch the instance
// with GetTrader, run ~40 lines of unlocked validation, and only then call
// Start on the held pointer. A background reload (RemoveTraderAndThen +
// LoadUserTradersFromStore) could evict and replace the instance in between,
// so the started copy lived outside the manager map — an orphan loop placing
// live orders with no owner. Lookup + start now share one loadMu critical
// section, the same lock RemoveTraderAndThen and LoadUserTradersFromStore
// hold while evicting or re-adding instances (lock order loadMu -> tm.mu,
// unchanged), so the started instance is by construction the one registered
// under id: an evicted or replaced pointer can never reach Start from here.
// ErrAlreadyRunning is returned unchanged so existing caller tolerance keeps
// working.
func (tm *TraderManager) StartTrader(id string) (traderName string, err error) {
	tm.loadMu.Lock()
	defer tm.loadMu.Unlock()

	start := tm.startFn
	if start == nil {
		start = (*trader.AutoTrader).Start
	}

	tm.mu.Lock()
	t, exists := tm.traders[id]
	tm.mu.Unlock()
	if !exists || t == nil {
		return "", fmt.Errorf("trader ID '%s' does not exist", id)
	}
	if err := start(t); err != nil {
		return t.GetName(), err
	}
	return t.GetName(), nil
}

// GetAllTraders retrieves all traders
func (tm *TraderManager) GetAllTraders() map[string]*trader.AutoTrader {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make(map[string]*trader.AutoTrader)
	for id, t := range tm.traders {
		result[id] = t
	}
	return result
}

// GetTraderIDs retrieves all trader IDs
func (tm *TraderManager) GetTraderIDs() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	ids := make([]string, 0, len(tm.traders))
	for id := range tm.traders {
		ids = append(ids, id)
	}
	return ids
}

// StartAll starts all traders
func (tm *TraderManager) StartAll() {
	for _, t := range tm.GetAllTraders() {
		if err := t.Start(); err != nil && !errors.Is(err, trader.ErrAlreadyRunning) {
			logger.Errorf("Start trader %s: %v", t.GetID(), err)
		}
	}
}

// StopAll stops all traders
func (tm *TraderManager) StopAll() {
	traders := tm.GetAllTraders()
	for _, t := range traders {
		t.SignalStop()
	}
	var wg sync.WaitGroup
	for _, t := range traders {
		wg.Add(1)
		// Process shutdown: traders come back on the next start, so resting
		// us_stock entries are kept and reconciled then.
		go func(t *trader.AutoTrader) { defer wg.Done(); t.StopForRestart() }(t)
	}
	wg.Wait()
}

func (tm *TraderManager) AutoStartRunningTraders(st *store.Store) {
	// Get all trader configurations (single query)
	traderList, err := st.Trader().ListAll()
	if err != nil {
		logger.Infof("⚠️ Failed to get trader list: %v", err)
		return
	}

	// Build set of running trader IDs
	runningTraderIDs := make(map[string]uint64)
	for _, traderCfg := range traderList {
		if traderCfg.IsRunning {
			runningTraderIDs[traderCfg.ID] = traderCfg.RunVersion
		}
	}

	if len(runningTraderIDs) == 0 {
		logger.Info("📋 No traders to auto-restore")
		return
	}

	startedCount := 0
	for id, t := range tm.GetAllTraders() {
		if version, desired := runningTraderIDs[id]; desired {
			if err := t.StartIfDesired(version); err != nil {
				if !errors.Is(err, trader.ErrAlreadyRunning) {
					logger.Errorf("Auto-restore %s: %v", id, err)
				}
				continue
			}
			startedCount++
		}
	}

	if startedCount > 0 {
		logger.Infof("✓ Auto-restored %d traders", startedCount)
	}
}

// GetComparisonData retrieves comparison data
func (tm *TraderManager) GetComparisonData() (map[string]interface{}, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	comparison := make(map[string]interface{})
	traders := make([]map[string]interface{}, 0, len(tm.traders))

	for _, t := range tm.traders {
		account, err := t.GetAccountInfo()
		if err != nil {
			continue
		}

		status := t.GetStatus()

		traders = append(traders, map[string]interface{}{
			"trader_id":       t.GetID(),
			"trader_name":     t.GetName(),
			"ai_model":        t.GetAIModel(),
			"exchange":        t.GetExchange(),
			"total_equity":    account["total_equity"],
			"total_pnl":       account["total_pnl"],
			"total_pnl_pct":   account["total_pnl_pct"],
			"position_count":  account["position_count"],
			"margin_used_pct": account["margin_used_pct"],
			"call_count":      status["call_count"],
			"is_running":      status["is_running"],
		})
	}

	comparison["traders"] = traders
	comparison["count"] = len(traders)

	return comparison, nil
}

// GetCompetitionData retrieves competition data (all traders across platform)
func (tm *TraderManager) GetCompetitionData() (map[string]interface{}, error) {
	tm.competitionRefreshMu.Lock()
	defer tm.competitionRefreshMu.Unlock()
	// Check if cache is valid (within 30 seconds)
	tm.competitionCache.mu.RLock()
	if time.Since(tm.competitionCache.timestamp) < 30*time.Second && len(tm.competitionCache.data) > 0 {
		// Return cached data
		cachedData := make(map[string]interface{})
		for k, v := range tm.competitionCache.data {
			cachedData[k] = v
		}
		tm.competitionCache.mu.RUnlock()
		return cachedData, nil
	}
	tm.competitionCache.mu.RUnlock()

	tm.mu.RLock()

	// Get all trader list (only those with ShowInCompetition = true)
	allTraders := make([]*trader.AutoTrader, 0, len(tm.traders))
	for id, t := range tm.traders {
		if t.GetShowInCompetition() {
			allTraders = append(allTraders, t)
			logger.Infof("📋 Competition data includes trader: %s (%s)", t.GetName(), id)
		} else {
			logger.Infof("📋 Competition data excludes trader (hidden): %s (%s)", t.GetName(), id)
		}
	}
	tm.mu.RUnlock()

	logger.Infof("🔄 Refreshing competition data, trader count: %d", len(allTraders))

	// Concurrently fetch trader data
	traders := tm.getConcurrentTraderData(allTraders)

	// Sort by profit rate (descending)
	sort.Slice(traders, func(i, j int) bool {
		pnlPctI, okI := traders[i]["total_pnl_pct"].(float64)
		pnlPctJ, okJ := traders[j]["total_pnl_pct"].(float64)
		if !okI {
			pnlPctI = 0
		}
		if !okJ {
			pnlPctJ = 0
		}
		return pnlPctI > pnlPctJ
	})

	// Limit to top 50
	totalCount := len(traders)
	limit := 50
	if len(traders) > limit {
		traders = traders[:limit]
	}

	comparison := make(map[string]interface{})
	comparison["traders"] = traders
	comparison["count"] = len(traders)
	comparison["total_count"] = totalCount // Total number of traders

	// Update cache
	tm.competitionCache.mu.Lock()
	tm.competitionCache.data = comparison
	tm.competitionCache.timestamp = time.Now()
	tm.competitionCache.mu.Unlock()

	return comparison, nil
}

// InvalidateCompetitionCache drops cached public leaderboard data. Visibility
// changes must take effect immediately rather than waiting for the cache TTL.
func (tm *TraderManager) InvalidateCompetitionCache() {
	tm.competitionCache.mu.Lock()
	tm.competitionCache.data = nil
	tm.competitionCache.timestamp = time.Time{}
	tm.competitionCache.mu.Unlock()
}

// getConcurrentTraderData concurrently fetches data for multiple traders
func (tm *TraderManager) getConcurrentTraderData(traders []*trader.AutoTrader) []map[string]interface{} {
	type traderResult struct {
		index int
		data  map[string]interface{}
	}

	// Create result channel
	resultChan := make(chan traderResult, len(traders))

	// review 2026-10-07 B2-2 (P1-2): channel sizing makes every send in the
	// fan-out non-blocking, so a goroutine abandoned by the 10s timeout can
	// never leak on its final send: resultChan is buffered to len(traders)
	// and the collector receives exactly that many values, while
	// accountChan/errorChan are buffered to 1 so the late result always
	// lands even after the outer select has moved on.
	getInfo := tm.accountInfoFn
	if getInfo == nil {
		getInfo = (*trader.AutoTrader).GetAccountInfo
	}

	// Concurrently fetch data for each trader
	for i, t := range traders {
		go func(index int, trader *trader.AutoTrader) {
			// Set timeout to 10 seconds for single trader (increased from 3s for DEX reliability)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Use channel for timeout control
			accountChan := make(chan map[string]interface{}, 1)
			errorChan := make(chan error, 1)

			go func() {
				// review 2026-10-07 B2-2 (P1-2): this goroutine used to call
				// GetAccountInfo bare — a single panic (nil map write in an
				// exchange client, index bug, ...) crashed the whole live
				// process. Recover it into this trader's error result;
				// every other trader's fan-out is unaffected. The errorChan
				// send after recovery cannot block (buffered above).
				defer func() {
					if r := recover(); r != nil {
						logger.Errorf("🚨 Panic fetching account info for trader %s (%s/%s): %v\n%s",
							trader.GetName(), trader.GetID(), trader.GetExchange(), r, debug.Stack())
						errorChan <- fmt.Errorf("account info panic: %v", r)
					}
				}()
				account, err := getInfo(trader)
				if err != nil {
					errorChan <- err
				} else {
					accountChan <- account
				}
			}()

			status := trader.GetStatus()
			var traderData map[string]interface{}

			select {
			case account := <-accountChan:
				// Successfully got account info
				traderData = map[string]interface{}{
					"trader_id":              trader.GetID(),
					"trader_name":            trader.GetName(),
					"ai_model":               trader.GetAIModel(),
					"exchange":               trader.GetExchange(),
					"total_equity":           account["total_equity"],
					"total_pnl":              account["total_pnl"],
					"total_pnl_pct":          account["total_pnl_pct"],
					"position_count":         account["position_count"],
					"margin_used_pct":        account["margin_used_pct"],
					"is_running":             status["is_running"],
					"system_prompt_template": trader.GetSystemPromptTemplate(),
				}
			case err := <-errorChan:
				// Failed to get account info
				logger.Infof("⚠️ Failed to get account info for trader %s (%s/%s): %v", trader.GetName(), trader.GetID(), trader.GetExchange(), err)
				traderData = map[string]interface{}{
					"trader_id":              trader.GetID(),
					"trader_name":            trader.GetName(),
					"ai_model":               trader.GetAIModel(),
					"exchange":               trader.GetExchange(),
					"total_equity":           0.0,
					"total_pnl":              0.0,
					"total_pnl_pct":          0.0,
					"position_count":         0,
					"margin_used_pct":        0.0,
					"is_running":             status["is_running"],
					"system_prompt_template": trader.GetSystemPromptTemplate(),
					"error":                  "Failed to get account data",
				}
			case <-ctx.Done():
				// Timeout
				logger.Infof("⏰ Timeout (10s) getting account info for trader %s (%s/%s)", trader.GetName(), trader.GetID(), trader.GetExchange())
				traderData = map[string]interface{}{
					"trader_id":              trader.GetID(),
					"trader_name":            trader.GetName(),
					"ai_model":               trader.GetAIModel(),
					"exchange":               trader.GetExchange(),
					"total_equity":           0.0,
					"total_pnl":              0.0,
					"total_pnl_pct":          0.0,
					"position_count":         0,
					"margin_used_pct":        0.0,
					"is_running":             status["is_running"],
					"system_prompt_template": trader.GetSystemPromptTemplate(),
					"error":                  "Request timeout",
				}
			}

			resultChan <- traderResult{index: index, data: traderData}
		}(i, t)
	}

	// Collect all results
	results := make([]map[string]interface{}, len(traders))
	for i := 0; i < len(traders); i++ {
		result := <-resultChan
		results[result.index] = result.data
	}

	return results
}

// GetTopTradersData retrieves top 5 traders data (for performance comparison)
func (tm *TraderManager) GetTopTradersData() (map[string]interface{}, error) {
	// Reuse competition data cache, as top 5 is filtered from all data
	competitionData, err := tm.GetCompetitionData()
	if err != nil {
		return nil, err
	}

	// Extract top 5 from competition data
	allTraders, ok := competitionData["traders"].([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid competition data format")
	}

	// Limit to top 5
	limit := 5
	topTraders := allTraders
	if len(allTraders) > limit {
		topTraders = allTraders[:limit]
	}

	result := map[string]interface{}{
		"traders": topTraders,
		"count":   len(topTraders),
	}

	return result, nil
}

// RemoveTrader removes a trader from memory (does not affect database)
// Used to force reload when updating trader configuration
// If the trader is running, it will be stopped first
// IsTraderRunning reports whether the in-memory trader loop is alive.
func (tm *TraderManager) IsTraderRunning(traderID string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	t, exists := tm.traders[traderID]
	if !exists {
		return false
	}
	status := t.GetStatus()
	running, ok := status["is_running"].(bool)
	return ok && running
}

// EnsureTraderStarted restarts a trader whose in-memory loop died (e.g. lost
// in a reload race). Persists is_running=true like the manual start flow.
func (tm *TraderManager) EnsureTraderStarted(userID, traderID string) error {
	if tm.store != nil {
		if _, err := tm.store.Trader().GetForUser(userID, traderID); err != nil {
			return err
		}
	}
	t, err := tm.GetTrader(traderID)
	if err != nil {
		return err
	}
	if err := t.Start(); err != nil && !errors.Is(err, trader.ErrAlreadyRunning) {
		return err
	}
	return nil
}

func (tm *TraderManager) RemoveTrader(traderID string) {
	_ = tm.RemoveTraderAndThen(traderID, nil)
}

// Keep reloads out until both the running client and its database state are removed.
func (tm *TraderManager) RemoveTraderAndThen(traderID string, after func() error) error {
	return tm.removeTrader(traderID, false, after)
}

// RemoveTraderForReload removes a trader that is reloaded right away (config
// save): its us_stock resting entries stay on the exchange for the new instance.
func (tm *TraderManager) RemoveTraderForReload(traderID string) error {
	return tm.removeTrader(traderID, true, nil)
}

func (tm *TraderManager) removeTrader(traderID string, forRestart bool, after func() error) error {
	tm.loadMu.Lock()
	defer tm.loadMu.Unlock()
	tm.mu.Lock()
	t := tm.traders[traderID]
	delete(tm.traders, traderID)
	tm.mu.Unlock()
	if t != nil {
		if forRestart {
			t.StopForRestart()
		} else {
			t.Stop()
		}
	}
	if after != nil {
		return after()
	}
	return nil
}

// LoadUserTradersFromStore loads traders from store for a specific user to memory
func (tm *TraderManager) LoadUserTradersFromStore(st *store.Store, userID string) error {
	tm.loadMu.Lock()
	defer tm.loadMu.Unlock()

	// Get all traders for the specified user
	traders, err := st.Trader().List(userID)
	if err != nil {
		return fmt.Errorf("failed to get trader list for user %s: %w", userID, err)
	}

	logger.Infof("📋 Loading trader configurations for user %s: %d traders", userID, len(traders))

	// Get AI model and exchange lists (query only once outside loop)
	aiModels, err := st.AIModel().List(userID)
	if err != nil {
		logger.Infof("⚠️ Failed to get AI model config for user %s: %v", userID, err)
		return fmt.Errorf("failed to get AI model config: %w", err)
	}

	exchanges, err := st.Exchange().List(userID)
	if err != nil {
		logger.Infof("⚠️ Failed to get exchange config for user %s: %v", userID, err)
		return fmt.Errorf("failed to get exchange config: %w", err)
	}

	// Load configuration for each trader
	for _, traderCfg := range traders {
		// Check if this trader is already loaded
		if _, err := tm.GetTrader(traderCfg.ID); err == nil {
			// Trader already loaded - this is normal, no need to log
			continue
		}

		// Find AI model config from already queried list
		var aiModelCfg *store.AIModel
		for _, model := range aiModels {
			if model.ID == traderCfg.AIModelID {
				aiModelCfg = model
				break
			}
		}
		if aiModelCfg == nil {
			for _, model := range aiModels {
				if model.Provider == traderCfg.AIModelID {
					aiModelCfg = model
					break
				}
			}
		}

		if aiModelCfg == nil {
			logger.Infof("⚠️ AI model %s for trader %s does not exist, skipping", traderCfg.AIModelID, traderCfg.Name)
			continue
		}

		if !aiModelCfg.Enabled {
			logger.Infof("⚠️ AI model %s for trader %s is not enabled, skipping", traderCfg.AIModelID, traderCfg.Name)
			continue
		}

		// Find exchange config from already queried list
		var exchangeCfg *store.Exchange
		for _, exchange := range exchanges {
			if exchange.ID == traderCfg.ExchangeID {
				exchangeCfg = exchange
				break
			}
		}

		if exchangeCfg == nil {
			logger.Infof("⚠️ Exchange %s for trader %s does not exist, skipping", traderCfg.ExchangeID, traderCfg.Name)
			continue
		}

		if !exchangeCfg.Enabled {
			logger.Infof("⚠️ Exchange %s for trader %s is not enabled, skipping", traderCfg.ExchangeID, traderCfg.Name)
			continue
		}

		// Use existing method to load trader
		logger.Infof("📦 Loading trader %s (AI Model: %s, Exchange: %s/%s, Strategy ID: %s)", traderCfg.Name, aiModelCfg.Provider, exchangeCfg.ExchangeType, exchangeCfg.AccountName, traderCfg.StrategyID)
		err = tm.addTraderFromStore(traderCfg, aiModelCfg, exchangeCfg, st)
		if err != nil {
			logger.Infof("❌ Failed to load trader %s: %v", traderCfg.Name, err)
			// Save error for later retrieval
			tm.mu.Lock()
			tm.loadErrors[traderCfg.ID] = err
			tm.mu.Unlock()
		} else {
			// Clear any previous error on success
			tm.mu.Lock()
			delete(tm.loadErrors, traderCfg.ID)
			tm.mu.Unlock()
		}
	}

	return nil
}

// LoadTradersFromStore loads all traders from store to memory (new API)
func (tm *TraderManager) LoadTradersFromStore(st *store.Store) error {
	tm.loadMu.Lock()
	defer tm.loadMu.Unlock()

	// Get all users
	userIDs, err := st.User().GetAllIDs()
	if err != nil {
		return fmt.Errorf("failed to get user list: %w", err)
	}

	logger.Infof("📋 Found %d users, loading all trader configurations...", len(userIDs))

	var allTraders []*store.Trader
	for _, userID := range userIDs {
		// Get traders for each user
		traders, err := st.Trader().List(userID)
		if err != nil {
			logger.Infof("⚠️ Failed to get traders for user %s: %v", userID, err)
			continue
		}
		logger.Infof("📋 User %s: %d traders", userID, len(traders))
		allTraders = append(allTraders, traders...)
	}

	logger.Infof("📋 Total loaded trader configurations: %d", len(allTraders))

	// Get AI model and exchange configs for each trader
	for _, traderCfg := range allTraders {
		// Get AI model config
		aiModels, err := st.AIModel().List(traderCfg.UserID)
		if err != nil {
			logger.Infof("⚠️  Failed to get AI model config: %v", err)
			continue
		}

		var aiModelCfg *store.AIModel
		// Prioritize exact match on model.ID
		for _, model := range aiModels {
			if model.ID == traderCfg.AIModelID {
				aiModelCfg = model
				break
			}
		}
		// If no exact match, try matching provider (for backward compatibility)
		if aiModelCfg == nil {
			for _, model := range aiModels {
				if model.Provider == traderCfg.AIModelID {
					aiModelCfg = model
					logger.Infof("⚠️  Trader %s using legacy provider match: %s -> %s", traderCfg.Name, traderCfg.AIModelID, model.ID)
					break
				}
			}
		}

		if aiModelCfg == nil {
			logger.Infof("⚠️  AI model %s for trader %s does not exist, skipping", traderCfg.AIModelID, traderCfg.Name)
			continue
		}

		if !aiModelCfg.Enabled {
			logger.Infof("⚠️  AI model %s for trader %s is not enabled, skipping", traderCfg.AIModelID, traderCfg.Name)
			continue
		}

		// Get exchange config
		exchanges, err := st.Exchange().List(traderCfg.UserID)
		if err != nil {
			logger.Infof("⚠️  Failed to get exchange config: %v", err)
			continue
		}

		var exchangeCfg *store.Exchange
		for _, exchange := range exchanges {
			if exchange.ID == traderCfg.ExchangeID {
				exchangeCfg = exchange
				break
			}
		}

		if exchangeCfg == nil {
			logger.Infof("⚠️  Exchange %s for trader %s does not exist, skipping", traderCfg.ExchangeID, traderCfg.Name)
			continue
		}

		if !exchangeCfg.Enabled {
			logger.Infof("⚠️  Exchange %s for trader %s is not enabled, skipping", traderCfg.ExchangeID, traderCfg.Name)
			continue
		}

		// Add to TraderManager (ai500APIURL/oiTopAPIURL already obtained from strategy config)
		err = tm.addTraderFromStore(traderCfg, aiModelCfg, exchangeCfg, st)
		if err != nil {
			logger.Infof("❌ Failed to add trader %s: %v", traderCfg.Name, err)
			continue
		}
	}

	logger.Infof("✓ Successfully loaded %d traders to memory", len(tm.GetAllTraders()))
	return nil
}

// addTraderFromStore internal method: adds trader from store configuration
func (tm *TraderManager) addTraderFromStore(traderCfg *store.Trader, aiModelCfg *store.AIModel, exchangeCfg *store.Exchange, st *store.Store) error {
	if _, err := tm.GetTrader(traderCfg.ID); err == nil {
		return fmt.Errorf("trader ID '%s' already exists", traderCfg.ID)
	}

	// Load strategy config (must have strategy)
	var strategyConfig *store.StrategyConfig
	if traderCfg.StrategyID != "" {
		strategy, err := st.Strategy().Get(traderCfg.UserID, traderCfg.StrategyID)
		if err != nil {
			return fmt.Errorf("failed to load strategy %s for trader %s: %w", traderCfg.StrategyID, traderCfg.Name, err)
		}
		// Parse JSON config
		strategyConfig, err = strategy.ParseConfig()
		if err != nil {
			return fmt.Errorf("failed to parse strategy config for trader %s: %w", traderCfg.Name, err)
		}
		logger.Infof("✓ Trader %s loaded strategy config: %s", traderCfg.Name, strategy.Name)
	} else {
		return fmt.Errorf("trader %s has no strategy configured", traderCfg.Name)
	}

	// Build AutoTraderConfig (ai500APIURL/oiTopAPIURL obtained from strategy config, used in StrategyEngine)
	traderConfig := trader.AutoTraderConfig{
		ID:                    traderCfg.ID,
		Name:                  traderCfg.Name,
		AIModel:               aiModelCfg.Provider,
		Exchange:              exchangeCfg.ExchangeType, // Exchange type: binance/bybit/okx/etc
		ExchangeID:            exchangeCfg.ID,           // Exchange account UUID (for multi-account)
		BinanceAPIKey:         "",
		BinanceSecretKey:      "",
		HyperliquidPrivateKey: "",
		HyperliquidTestnet:    exchangeCfg.Testnet,
		UseQwen:               aiModelCfg.Provider == "qwen",
		DeepSeekKey:           "",
		QwenKey:               "",
		CustomAPIURL:          aiModelCfg.CustomAPIURL,
		CustomModelName:       aiModelCfg.CustomModelName,
		ScanInterval:          time.Duration(traderCfg.ScanIntervalMinutes) * time.Minute,
		InitialBalance:        traderCfg.InitialBalance,
		IsCrossMargin:         traderCfg.IsCrossMargin,
		ShowInCompetition:     traderCfg.ShowInCompetition,
		StrategyConfig:        strategyConfig,
		StrategyID:            traderCfg.StrategyID,
	}

	logger.Infof("📊 Loading trader %s: ScanIntervalMinutes=%d (from DB), ScanInterval=%v",
		traderCfg.Name, traderCfg.ScanIntervalMinutes, traderConfig.ScanInterval)

	// Set API keys based on exchange type (convert EncryptedString to string)
	switch exchangeCfg.ExchangeType {
	case "binance":
		traderConfig.BinanceAPIKey = string(exchangeCfg.APIKey)
		traderConfig.BinanceSecretKey = string(exchangeCfg.SecretKey)
	case "binance_stocks":
		traderConfig.BinanceStocksAPIKey = string(exchangeCfg.APIKey)
		traderConfig.BinanceStocksSecretKey = string(exchangeCfg.SecretKey)
	case "bybit":
		traderConfig.BybitAPIKey = string(exchangeCfg.APIKey)
		traderConfig.BybitSecretKey = string(exchangeCfg.SecretKey)
	case "okx":
		traderConfig.OKXAPIKey = string(exchangeCfg.APIKey)
		traderConfig.OKXSecretKey = string(exchangeCfg.SecretKey)
		traderConfig.OKXPassphrase = string(exchangeCfg.Passphrase)
	case "bitget":
		traderConfig.BitgetAPIKey = string(exchangeCfg.APIKey)
		traderConfig.BitgetSecretKey = string(exchangeCfg.SecretKey)
		traderConfig.BitgetPassphrase = string(exchangeCfg.Passphrase)
	case "gate":
		traderConfig.GateAPIKey = string(exchangeCfg.APIKey)
		traderConfig.GateSecretKey = string(exchangeCfg.SecretKey)
	case "kucoin":
		traderConfig.KuCoinAPIKey = string(exchangeCfg.APIKey)
		traderConfig.KuCoinSecretKey = string(exchangeCfg.SecretKey)
		traderConfig.KuCoinPassphrase = string(exchangeCfg.Passphrase)
	case "hyperliquid":
		traderConfig.HyperliquidPrivateKey = string(exchangeCfg.APIKey)
		traderConfig.HyperliquidWalletAddr = exchangeCfg.HyperliquidWalletAddr
		traderConfig.HyperliquidUnifiedAcct = exchangeCfg.HyperliquidUnifiedAcct
	case "aster":
		traderConfig.AsterUser = exchangeCfg.AsterUser
		traderConfig.AsterSigner = exchangeCfg.AsterSigner
		traderConfig.AsterPrivateKey = string(exchangeCfg.AsterPrivateKey)
	case "lighter":
		traderConfig.LighterPrivateKey = string(exchangeCfg.LighterPrivateKey)
		traderConfig.LighterWalletAddr = exchangeCfg.LighterWalletAddr
		traderConfig.LighterAPIKeyPrivateKey = string(exchangeCfg.LighterAPIKeyPrivateKey)
		traderConfig.LighterAPIKeyIndex = exchangeCfg.LighterAPIKeyIndex
		traderConfig.LighterTestnet = exchangeCfg.Testnet
	case "indodax":
		traderConfig.IndodaxAPIKey = string(exchangeCfg.APIKey)
		traderConfig.IndodaxSecretKey = string(exchangeCfg.SecretKey)
	}

	// Set API keys based on AI model (convert EncryptedString to string)
	switch aiModelCfg.Provider {
	case "qwen":
		traderConfig.QwenKey = string(aiModelCfg.APIKey)
	case "deepseek":
		traderConfig.DeepSeekKey = string(aiModelCfg.APIKey)
	default:
		// For other providers (grok, openai, claude, gemini, kimi, etc.), use CustomAPIKey
		traderConfig.CustomAPIKey = string(aiModelCfg.APIKey)
	}

	// Create trader instance
	at, err := trader.NewAutoTrader(traderConfig, st, traderCfg.UserID)
	if err != nil {
		return fmt.Errorf("failed to create trader: %w", err)
	}

	// Set custom prompt (if exists)
	if traderCfg.CustomPrompt != "" {
		at.SetCustomPrompt(traderCfg.CustomPrompt)
		at.SetOverrideBasePrompt(traderCfg.OverrideBasePrompt)
		if traderCfg.OverrideBasePrompt {
			logger.Infof("✓ Set custom trading strategy prompt (overriding base prompt)")
		} else {
			logger.Infof("✓ Set custom trading strategy prompt (supplementing base prompt)")
		}
	}

	tm.mu.Lock()
	tm.traders[traderCfg.ID] = at
	tm.mu.Unlock()
	logger.Infof("✓ Trader '%s' (%s + %s/%s) loaded to memory", traderCfg.Name, aiModelCfg.Provider, exchangeCfg.ExchangeType, exchangeCfg.AccountName)

	if traderCfg.IsRunning {
		if err := at.StartIfDesired(traderCfg.RunVersion); err != nil {
			return err
		}
	}
	return nil
}
