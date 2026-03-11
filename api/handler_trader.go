package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"nofx/logger"
	"nofx/store"
	"nofx/trader"
	"nofx/trader/aster"
	"nofx/trader/binance"
	"nofx/trader/bitget"
	"nofx/trader/bybit"
	"nofx/trader/gate"
	hyperliquidtrader "nofx/trader/hyperliquid"
	"nofx/trader/kucoin"
	"nofx/trader/lighter"
	"nofx/trader/okx"

	"github.com/gin-gonic/gin"
)

// AI trader management related structures
type CreateTraderRequest struct {
	Name                string  `json:"name" binding:"required"`
	AIModelID           string  `json:"ai_model_id" binding:"required"`
	ExchangeID          string  `json:"exchange_id" binding:"required"`
	StrategyID          string  `json:"strategy_id"` // Strategy ID (new version)
	InitialBalance      float64 `json:"initial_balance"`
	ScanIntervalMinutes int     `json:"scan_interval_minutes"`
	IsCrossMargin       *bool   `json:"is_cross_margin"`     // Pointer type, nil means use default value true
	ShowInCompetition   *bool   `json:"show_in_competition"` // Pointer type, nil means use default value true
	// The following fields are kept for backward compatibility, new version uses strategy config
	BTCETHLeverage       int    `json:"btc_eth_leverage"`
	AltcoinLeverage      int    `json:"altcoin_leverage"`
	TradingSymbols       string `json:"trading_symbols"`
	CustomPrompt         string `json:"custom_prompt"`
	OverrideBasePrompt   bool   `json:"override_base_prompt"`
	SystemPromptTemplate string `json:"system_prompt_template"` // System prompt template name
	UseAI500             bool   `json:"use_ai500"`
	UseOITop             bool   `json:"use_oi_top"`
}

// UpdateTraderRequest Update trader request
type UpdateTraderRequest struct {
	Name                string  `json:"name" binding:"required"`
	AIModelID           string  `json:"ai_model_id" binding:"required"`
	ExchangeID          string  `json:"exchange_id" binding:"required"`
	StrategyID          string  `json:"strategy_id"` // Strategy ID (new version)
	InitialBalance      float64 `json:"initial_balance"`
	ScanIntervalMinutes int     `json:"scan_interval_minutes"`
	IsCrossMargin       *bool   `json:"is_cross_margin"`
	ShowInCompetition   *bool   `json:"show_in_competition"`
	// The following fields are kept for backward compatibility, new version uses strategy config
	BTCETHLeverage       int    `json:"btc_eth_leverage"`
	AltcoinLeverage      int    `json:"altcoin_leverage"`
	TradingSymbols       string `json:"trading_symbols"`
	CustomPrompt         string `json:"custom_prompt"`
	OverrideBasePrompt   bool   `json:"override_base_prompt"`
	SystemPromptTemplate string `json:"system_prompt_template"`
}

// handleCreateTrader Create new AI trader
func (s *Server) handleCreateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	var req CreateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Validate leverage values
	if req.BTCETHLeverage < 0 || req.BTCETHLeverage > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "BTC/ETH leverage must be between 1-50x"})
		return
	}
	if req.AltcoinLeverage < 0 || req.AltcoinLeverage > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Altcoin leverage must be between 1-20x"})
		return
	}

	// Validate trading symbol format
	if req.TradingSymbols != "" {
		symbols := strings.Split(req.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" && !strings.HasSuffix(strings.ToUpper(symbol), "USDT") {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid symbol format: %s, must end with USDT", symbol)})
				return
			}
		}
	}

	// Generate trader ID (use short UUID prefix for readability)
	exchangeIDShort := req.ExchangeID
	if len(exchangeIDShort) > 8 {
		exchangeIDShort = exchangeIDShort[:8]
	}
	traderID := fmt.Sprintf("%s_%s_%d", exchangeIDShort, req.AIModelID, time.Now().Unix())

	// Set default values
	isCrossMargin := true // Default to cross margin mode
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	showInCompetition := true // Default to show in competition
	if req.ShowInCompetition != nil {
		showInCompetition = *req.ShowInCompetition
	}

	// Set leverage default values
	btcEthLeverage := 10 // Default value
	altcoinLeverage := 5 // Default value
	if req.BTCETHLeverage > 0 {
		btcEthLeverage = req.BTCETHLeverage
	}
	if req.AltcoinLeverage > 0 {
		altcoinLeverage = req.AltcoinLeverage
	}

	// Set system prompt template default value
	systemPromptTemplate := "default"
	if req.SystemPromptTemplate != "" {
		systemPromptTemplate = req.SystemPromptTemplate
	}

	// Set scan interval default value
	scanIntervalMinutes := req.ScanIntervalMinutes
	if scanIntervalMinutes < 3 {
		scanIntervalMinutes = 3 // Default 3 minutes, not allowed to be less than 3
	}

	// Query exchange actual balance, override user input
	actualBalance := req.InitialBalance // Default to use user input
	exchanges, err := s.store.Exchange().List(userID)
	if err != nil {
		logger.Infof("⚠️ Failed to get exchange config, using user input for initial balance: %v", err)
	}

	// Find matching exchange configuration
	var exchangeCfg *store.Exchange
	for _, ex := range exchanges {
		if ex.ID == req.ExchangeID {
			exchangeCfg = ex
			break
		}
	}

	if exchangeCfg == nil {
		logger.Infof("⚠️ Exchange %s configuration not found, using user input for initial balance", req.ExchangeID)
	} else if !exchangeCfg.Enabled {
		logger.Infof("⚠️ Exchange %s not enabled, using user input for initial balance", req.ExchangeID)
	} else {
		// Create temporary trader based on exchange type to query balance
		var tempTrader trader.Trader
		var createErr error

		// Use ExchangeType (e.g., "binance") instead of ID (UUID)
		// Convert EncryptedString fields to string
		switch exchangeCfg.ExchangeType {
		case "binance":
			tempTrader = binance.NewFuturesTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), userID)
		case "hyperliquid":
			tempTrader, createErr = hyperliquidtrader.NewHyperliquidTrader(
				string(exchangeCfg.APIKey), // private key
				exchangeCfg.HyperliquidWalletAddr,
				exchangeCfg.Testnet,
				exchangeCfg.HyperliquidUnifiedAcct,
			)
		case "aster":
			tempTrader, createErr = aster.NewAsterTrader(
				exchangeCfg.AsterUser,
				exchangeCfg.AsterSigner,
				string(exchangeCfg.AsterPrivateKey),
			)
		case "bybit":
			tempTrader = bybit.NewBybitTrader(
				string(exchangeCfg.APIKey),
				string(exchangeCfg.SecretKey),
			)
		case "okx":
			tempTrader = okx.NewOKXTrader(
				string(exchangeCfg.APIKey),
				string(exchangeCfg.SecretKey),
				string(exchangeCfg.Passphrase),
			)
		case "bitget":
			tempTrader = bitget.NewBitgetTrader(
				string(exchangeCfg.APIKey),
				string(exchangeCfg.SecretKey),
				string(exchangeCfg.Passphrase),
			)
		case "gate":
			tempTrader = gate.NewGateTrader(
				string(exchangeCfg.APIKey),
				string(exchangeCfg.SecretKey),
			)
		case "kucoin":
			tempTrader = kucoin.NewKuCoinTrader(
				string(exchangeCfg.APIKey),
				string(exchangeCfg.SecretKey),
				string(exchangeCfg.Passphrase),
			)
		case "lighter":
			if exchangeCfg.LighterWalletAddr != "" && string(exchangeCfg.LighterAPIKeyPrivateKey) != "" {
				// Lighter only supports mainnet
				tempTrader, createErr = lighter.NewLighterTraderV2(
					exchangeCfg.LighterWalletAddr,
					string(exchangeCfg.LighterAPIKeyPrivateKey),
					exchangeCfg.LighterAPIKeyIndex,
					false, // Always use mainnet for Lighter
				)
			} else {
				createErr = fmt.Errorf("Lighter requires wallet address and API Key private key")
			}
		default:
			logger.Infof("⚠️ Unsupported exchange type: %s, using user input for initial balance", exchangeCfg.ExchangeType)
		}

		if createErr != nil {
			logger.Infof("⚠️ Failed to create temporary trader, using user input for initial balance: %v", createErr)
		} else if tempTrader != nil {
			// Query actual balance
			balanceInfo, balanceErr := tempTrader.GetBalance()
			if balanceErr != nil {
				logger.Infof("⚠️ Failed to query exchange balance, using user input for initial balance: %v", balanceErr)
			} else {
				// Extract total equity (account total value = wallet balance + unrealized PnL)
				// Priority: total_equity > totalWalletBalance > wallet_balance > totalEq > balance
				// Note: Must use total_equity (not availableBalance) for accurate P&L calculation
				balanceKeys := []string{"total_equity", "totalWalletBalance", "wallet_balance", "totalEq", "balance"}
				for _, key := range balanceKeys {
					if balance, ok := balanceInfo[key].(float64); ok && balance > 0 {
						actualBalance = balance
						logger.Infof("✓ Queried exchange total equity (%s): %.2f USDT (user input: %.2f USDT)", key, actualBalance, req.InitialBalance)
						break
					}
				}
				if actualBalance <= 0 {
					logger.Infof("⚠️ Unable to extract total equity from balance info, balanceInfo=%v, using user input for initial balance", balanceInfo)
				}
			}
		}
	}

	// Create trader configuration (database entity)
	logger.Infof("🔧 DEBUG: Starting to create trader config, ID=%s, Name=%s, AIModel=%s, Exchange=%s, StrategyID=%s", traderID, req.Name, req.AIModelID, req.ExchangeID, req.StrategyID)
	traderRecord := &store.Trader{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		StrategyID:           req.StrategyID, // Associated strategy ID (new version)
		InitialBalance:       actualBalance,  // Use actual queried balance
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       req.TradingSymbols,
		UseAI500:             req.UseAI500,
		UseOITop:             req.UseOITop,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: systemPromptTemplate,
		IsCrossMargin:        isCrossMargin,
		ShowInCompetition:    showInCompetition,
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            false,
	}

	// Save to database
	logger.Infof("🔧 DEBUG: Preparing to call CreateTrader")
	err = s.store.Trader().Create(traderRecord)
	if err != nil {
		logger.Infof("❌ Failed to create trader: %v", err)
		SafeInternalError(c, "Failed to create trader", err)
		return
	}
	logger.Infof("🔧 DEBUG: CreateTrader succeeded")

	// Immediately load new trader into TraderManager
	logger.Infof("🔧 DEBUG: Preparing to call LoadUserTraders")
	err = s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to load user traders into memory: %v", err)
		// Don't return error here since trader was successfully created in database
	}
	logger.Infof("🔧 DEBUG: LoadUserTraders completed")

	logger.Infof("✓ Trader created successfully: %s (model: %s, exchange: %s)", req.Name, req.AIModelID, req.ExchangeID)

	c.JSON(http.StatusCreated, gin.H{
		"trader_id":   traderID,
		"trader_name": req.Name,
		"ai_model":    req.AIModelID,
		"is_running":  false,
	})
}

// handleUpdateTrader Update trader configuration
func (s *Server) handleUpdateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	var req UpdateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Check if trader exists and belongs to current user
	traders, err := s.store.Trader().List(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trader list"})
		return
	}

	var existingTrader *store.Trader
	for _, t := range traders {
		if t.ID == traderID {
			existingTrader = t
			break
		}
	}

	if existingTrader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	// Set default values
	isCrossMargin := existingTrader.IsCrossMargin // Keep original value
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	showInCompetition := existingTrader.ShowInCompetition // Keep original value
	if req.ShowInCompetition != nil {
		showInCompetition = *req.ShowInCompetition
	}

	// Set leverage default values
	btcEthLeverage := req.BTCETHLeverage
	altcoinLeverage := req.AltcoinLeverage
	if btcEthLeverage <= 0 {
		btcEthLeverage = existingTrader.BTCETHLeverage // Keep original value
	}
	if altcoinLeverage <= 0 {
		altcoinLeverage = existingTrader.AltcoinLeverage // Keep original value
	}

	// Set scan interval, allow updates
	scanIntervalMinutes := req.ScanIntervalMinutes
	logger.Infof("📊 Update trader scan_interval: req=%d, existing=%d", req.ScanIntervalMinutes, existingTrader.ScanIntervalMinutes)
	if scanIntervalMinutes <= 0 {
		scanIntervalMinutes = existingTrader.ScanIntervalMinutes // Keep original value
	} else if scanIntervalMinutes < 3 {
		scanIntervalMinutes = 3
	}
	logger.Infof("📊 Final scan_interval_minutes: %d", scanIntervalMinutes)

	// Set system prompt template
	systemPromptTemplate := req.SystemPromptTemplate
	if systemPromptTemplate == "" {
		systemPromptTemplate = existingTrader.SystemPromptTemplate // Keep original value
	}

	// Handle strategy ID (if not provided, keep original value)
	strategyID := req.StrategyID
	if strategyID == "" {
		strategyID = existingTrader.StrategyID
	}

	// Update trader configuration
	traderRecord := &store.Trader{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		StrategyID:           strategyID, // Associated strategy ID
		InitialBalance:       req.InitialBalance,
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       req.TradingSymbols,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: systemPromptTemplate,
		IsCrossMargin:        isCrossMargin,
		ShowInCompetition:    showInCompetition,
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            existingTrader.IsRunning, // Keep original value
	}

	// Check if trader was running before update (we'll restart it after)
	wasRunning := false
	if existingMemTrader, memErr := s.traderManager.GetTrader(traderID); memErr == nil {
		status := existingMemTrader.GetStatus()
		if running, ok := status["is_running"].(bool); ok && running {
			wasRunning = true
			logger.Infof("🔄 Trader %s was running, will restart with new config after update", traderID)
		}
	}

	// Update database
	logger.Infof("🔄 Updating trader: ID=%s, Name=%s, AIModelID=%s, StrategyID=%s, ScanInterval=%d min",
		traderRecord.ID, traderRecord.Name, traderRecord.AIModelID, traderRecord.StrategyID, scanIntervalMinutes)
	err = s.store.Trader().Update(traderRecord)
	if err != nil {
		SafeInternalError(c, "Failed to update trader", err)
		return
	}

	// Remove old trader from memory first (this also stops if running)
	s.traderManager.RemoveTrader(traderID)

	// Reload traders into memory with fresh config
	err = s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to reload user traders into memory: %v", err)
	}

	// If trader was running before, restart it with new config
	if wasRunning {
		if reloadedTrader, getErr := s.traderManager.GetTrader(traderID); getErr == nil {
			go func() {
				logger.Infof("▶️ Restarting trader %s with new config...", traderID)
				if runErr := reloadedTrader.Run(); runErr != nil {
					logger.Infof("❌ Trader %s runtime error: %v", traderID, runErr)
				}
			}()
		}
	}

	logger.Infof("✓ Trader updated successfully: %s (model: %s, exchange: %s, strategy: %s)", req.Name, req.AIModelID, req.ExchangeID, strategyID)

	c.JSON(http.StatusOK, gin.H{
		"trader_id":   traderID,
		"trader_name": req.Name,
		"ai_model":    req.AIModelID,
		"message":     "Trader updated successfully",
	})
}

// handleDeleteTrader Delete trader
func (s *Server) handleDeleteTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// Delete from database
	err := s.store.Trader().Delete(userID, traderID)
	if err != nil {
		SafeInternalError(c, "Failed to delete trader", err)
		return
	}

	// If trader is running, stop it first
	if trader, err := s.traderManager.GetTrader(traderID); err == nil {
		status := trader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); ok && isRunning {
			trader.Stop()
			logger.Infof("⏹  Stopped running trader: %s", traderID)
		}
	}

	// Remove trader from memory
	s.traderManager.RemoveTrader(traderID)

	logger.Infof("✓ Trader deleted: %s", traderID)
	c.JSON(http.StatusOK, gin.H{"message": "Trader deleted"})
}

// handleStartTrader Start trader
func (s *Server) handleStartTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// Verify trader belongs to current user
	_, err := s.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist or no access permission"})
		return
	}

	// Check if trader exists in memory and if it's running
	existingTrader, _ := s.traderManager.GetTrader(traderID)
	if existingTrader != nil {
		status := existingTrader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); ok && isRunning {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Trader is already running"})
			return
		}
		// Trader exists but is stopped - remove from memory to reload fresh config
		logger.Infof("🔄 Removing stopped trader %s from memory to reload config...", traderID)
		s.traderManager.RemoveTrader(traderID)
	}

	// Load trader from database (always reload to get latest config)
	logger.Infof("🔄 Loading trader %s from database...", traderID)
	if loadErr := s.traderManager.LoadUserTradersFromStore(s.store, userID); loadErr != nil {
		logger.Infof("❌ Failed to load user traders: %v", loadErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load trader: " + loadErr.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		// Check detailed reason
		fullCfg, _ := s.store.Trader().GetFullConfig(userID, traderID)
		if fullCfg != nil && fullCfg.Trader != nil {
			// Check strategy
			if fullCfg.Strategy == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Trader has no strategy configured, please create a strategy in Strategy Studio and associate it with the trader"})
				return
			}
			// Check AI model
			if fullCfg.AIModel == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Trader's AI model does not exist, please check AI model configuration"})
				return
			}
			if !fullCfg.AIModel.Enabled {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Trader's AI model is not enabled, please enable the AI model first"})
				return
			}
			// Check exchange
			if fullCfg.Exchange == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Trader's exchange does not exist, please check exchange configuration"})
				return
			}
			if !fullCfg.Exchange.Enabled {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Trader's exchange is not enabled, please enable the exchange first"})
				return
			}
		}
		// Check if there's a specific load error
		if loadErr := s.traderManager.GetLoadError(traderID); loadErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load trader: " + loadErr.Error()})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Failed to load trader, please check AI model, exchange and strategy configuration"})
		return
	}

	// Start trader
	go func() {
		logger.Infof("▶️  Starting trader %s (%s)", traderID, trader.GetName())
		if err := trader.Run(); err != nil {
			logger.Infof("❌ Trader %s runtime error: %v", trader.GetName(), err)
		}
	}()

	// Update running status in database
	err = s.store.Trader().UpdateStatus(userID, traderID, true)
	if err != nil {
		logger.Infof("⚠️  Failed to update trader status: %v", err)
	}

	logger.Infof("✓ Trader %s started", trader.GetName())
	c.JSON(http.StatusOK, gin.H{"message": "Trader started"})
}

// handleStopTrader Stop trader
func (s *Server) handleStopTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// Verify trader belongs to current user
	_, err := s.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist or no access permission"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	// Check if trader is running
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && !isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trader is already stopped"})
		return
	}

	// Stop trader
	trader.Stop()

	// Update running status in database
	err = s.store.Trader().UpdateStatus(userID, traderID, false)
	if err != nil {
		logger.Infof("⚠️  Failed to update trader status: %v", err)
	}

	logger.Infof("⏹  Trader %s stopped", trader.GetName())
	c.JSON(http.StatusOK, gin.H{"message": "Trader stopped"})
}

// handleUpdateTraderPrompt Update trader custom prompt
func (s *Server) handleUpdateTraderPrompt(c *gin.Context) {
	traderID := c.Param("id")
	userID := c.GetString("user_id")

	var req struct {
		CustomPrompt       string `json:"custom_prompt"`
		OverrideBasePrompt bool   `json:"override_base_prompt"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Update database
	err := s.store.Trader().UpdateCustomPrompt(userID, traderID, req.CustomPrompt, req.OverrideBasePrompt)
	if err != nil {
		SafeInternalError(c, "Failed to update custom prompt", err)
		return
	}

	// If trader is in memory, update its custom prompt and override settings
	trader, err := s.traderManager.GetTrader(traderID)
	if err == nil {
		trader.SetCustomPrompt(req.CustomPrompt)
		trader.SetOverrideBasePrompt(req.OverrideBasePrompt)
		logger.Infof("✓ Updated trader %s custom prompt (override base=%v)", trader.GetName(), req.OverrideBasePrompt)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Custom prompt updated"})
}

// handleToggleCompetition Toggle trader competition visibility
func (s *Server) handleToggleCompetition(c *gin.Context) {
	traderID := c.Param("id")
	userID := c.GetString("user_id")

	var req struct {
		ShowInCompetition bool `json:"show_in_competition"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Update database
	err := s.store.Trader().UpdateShowInCompetition(userID, traderID, req.ShowInCompetition)
	if err != nil {
		SafeInternalError(c, "Update competition visibility", err)
		return
	}

	// Update in-memory trader if it exists
	if trader, err := s.traderManager.GetTrader(traderID); err == nil {
		trader.SetShowInCompetition(req.ShowInCompetition)
	}

	status := "shown"
	if !req.ShowInCompetition {
		status = "hidden"
	}
	logger.Infof("✓ Trader %s competition visibility updated: %s", traderID, status)
	c.JSON(http.StatusOK, gin.H{
		"message":             "Competition visibility updated",
		"show_in_competition": req.ShowInCompetition,
	})
}

// handleGetGridRiskInfo returns current risk information for a grid trader
func (s *Server) handleGetGridRiskInfo(c *gin.Context) {
	traderID := c.Param("id")

	autoTrader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trader not found"})
		return
	}

	riskInfo := autoTrader.GetGridRiskInfo()
	c.JSON(http.StatusOK, riskInfo)
}

// handleSyncBalance Sync exchange balance to initial_balance (Option B: Manual Sync + Option C: Smart Detection)
func (s *Server) handleSyncBalance(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	logger.Infof("🔄 User %s requested balance sync for trader %s", userID, traderID)

	// Get trader configuration from database (including exchange info)
	fullConfig, err := s.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	traderConfig := fullConfig.Trader
	exchangeCfg := fullConfig.Exchange

	if exchangeCfg == nil || !exchangeCfg.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Exchange not configured or not enabled"})
		return
	}

	// Create temporary trader to query balance
	var tempTrader trader.Trader
	var createErr error

	// Use ExchangeType (e.g., "binance") instead of ExchangeID (which is now UUID)
	// Convert EncryptedString fields to string
	switch exchangeCfg.ExchangeType {
	case "binance":
		tempTrader = binance.NewFuturesTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), userID)
	case "hyperliquid":
		tempTrader, createErr = hyperliquidtrader.NewHyperliquidTrader(
			string(exchangeCfg.APIKey),
			exchangeCfg.HyperliquidWalletAddr,
			exchangeCfg.Testnet,
			exchangeCfg.HyperliquidUnifiedAcct,
		)
	case "aster":
		tempTrader, createErr = aster.NewAsterTrader(
			exchangeCfg.AsterUser,
			exchangeCfg.AsterSigner,
			string(exchangeCfg.AsterPrivateKey),
		)
	case "bybit":
		tempTrader = bybit.NewBybitTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
		)
	case "okx":
		tempTrader = okx.NewOKXTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
			string(exchangeCfg.Passphrase),
		)
	case "bitget":
		tempTrader = bitget.NewBitgetTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
			string(exchangeCfg.Passphrase),
		)
	case "gate":
		tempTrader = gate.NewGateTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
		)
	case "kucoin":
		tempTrader = kucoin.NewKuCoinTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
			string(exchangeCfg.Passphrase),
		)
	case "lighter":
		if exchangeCfg.LighterWalletAddr != "" && string(exchangeCfg.LighterAPIKeyPrivateKey) != "" {
			// Lighter only supports mainnet
			tempTrader, createErr = lighter.NewLighterTraderV2(
				exchangeCfg.LighterWalletAddr,
				string(exchangeCfg.LighterAPIKeyPrivateKey),
				exchangeCfg.LighterAPIKeyIndex,
				false, // Always use mainnet for Lighter
			)
		} else {
			createErr = fmt.Errorf("Lighter requires wallet address and API Key private key")
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported exchange type"})
		return
	}

	if createErr != nil {
		logger.Infof("⚠️ Failed to create temporary trader: %v", createErr)
		SafeInternalError(c, "Failed to connect to exchange", createErr)
		return
	}

	// Query actual balance
	balanceInfo, balanceErr := tempTrader.GetBalance()
	if balanceErr != nil {
		logger.Infof("⚠️ Failed to query exchange balance: %v", balanceErr)
		SafeInternalError(c, "Failed to query balance", balanceErr)
		return
	}

	// Extract total equity (for P&L calculation, we need total account value, not available balance)
	var actualBalance float64
	// Priority: total_equity > totalWalletBalance > wallet_balance > totalEq > balance
	balanceKeys := []string{"total_equity", "totalWalletBalance", "wallet_balance", "totalEq", "balance"}
	for _, key := range balanceKeys {
		if balance, ok := balanceInfo[key].(float64); ok && balance > 0 {
			actualBalance = balance
			break
		}
	}
	if actualBalance <= 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to get total equity"})
		return
	}

	oldBalance := traderConfig.InitialBalance

	// ✅ Option C: Smart balance change detection
	changePercent := ((actualBalance - oldBalance) / oldBalance) * 100
	changeType := "increase"
	if changePercent < 0 {
		changeType = "decrease"
	}

	logger.Infof("✓ Queried actual exchange balance: %.2f USDT (current config: %.2f USDT, change: %.2f%%)",
		actualBalance, oldBalance, changePercent)

	// Update initial_balance in database
	err = s.store.Trader().UpdateInitialBalance(userID, traderID, actualBalance)
	if err != nil {
		logger.Infof("❌ Failed to update initial_balance: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update balance"})
		return
	}

	// Reload traders into memory
	err = s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to reload user traders into memory: %v", err)
	}

	logger.Infof("✅ Synced balance: %.2f → %.2f USDT (%s %.2f%%)", oldBalance, actualBalance, changeType, changePercent)

	c.JSON(http.StatusOK, gin.H{
		"message":        "Balance synced successfully",
		"old_balance":    oldBalance,
		"new_balance":    actualBalance,
		"change_percent": changePercent,
		"change_type":    changeType,
	})
}

// handleClosePosition One-click close position
func (s *Server) handleClosePosition(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	var req struct {
		Symbol string `json:"symbol" binding:"required"`
		Side   string `json:"side" binding:"required"` // "LONG" or "SHORT"
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter error: symbol and side are required"})
		return
	}

	logger.Infof("🔻 User %s requested position close: trader=%s, symbol=%s, side=%s", userID, traderID, req.Symbol, req.Side)

	// Get trader configuration from database (including exchange info)
	fullConfig, err := s.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	exchangeCfg := fullConfig.Exchange

	if exchangeCfg == nil || !exchangeCfg.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Exchange not configured or not enabled"})
		return
	}

	// Create temporary trader to execute close position
	var tempTrader trader.Trader
	var createErr error

	// Use ExchangeType (e.g., "binance") instead of ExchangeID (which is now UUID)
	// Convert EncryptedString fields to string
	switch exchangeCfg.ExchangeType {
	case "binance":
		tempTrader = binance.NewFuturesTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), userID)
	case "hyperliquid":
		tempTrader, createErr = hyperliquidtrader.NewHyperliquidTrader(
			string(exchangeCfg.APIKey),
			exchangeCfg.HyperliquidWalletAddr,
			exchangeCfg.Testnet,
			exchangeCfg.HyperliquidUnifiedAcct,
		)
	case "aster":
		tempTrader, createErr = aster.NewAsterTrader(
			exchangeCfg.AsterUser,
			exchangeCfg.AsterSigner,
			string(exchangeCfg.AsterPrivateKey),
		)
	case "bybit":
		tempTrader = bybit.NewBybitTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
		)
	case "okx":
		tempTrader = okx.NewOKXTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
			string(exchangeCfg.Passphrase),
		)
	case "bitget":
		tempTrader = bitget.NewBitgetTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
			string(exchangeCfg.Passphrase),
		)
	case "gate":
		tempTrader = gate.NewGateTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
		)
	case "kucoin":
		tempTrader = kucoin.NewKuCoinTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
			string(exchangeCfg.Passphrase),
		)
	case "lighter":
		if exchangeCfg.LighterWalletAddr != "" && string(exchangeCfg.LighterAPIKeyPrivateKey) != "" {
			// Lighter only supports mainnet
			tempTrader, createErr = lighter.NewLighterTraderV2(
				exchangeCfg.LighterWalletAddr,
				string(exchangeCfg.LighterAPIKeyPrivateKey),
				exchangeCfg.LighterAPIKeyIndex,
				false, // Always use mainnet for Lighter
			)
		} else {
			createErr = fmt.Errorf("Lighter requires wallet address and API Key private key")
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported exchange type"})
		return
	}

	if createErr != nil {
		logger.Infof("⚠️ Failed to create temporary trader: %v", createErr)
		SafeInternalError(c, "Failed to connect to exchange", createErr)
		return
	}

	// Get current position info BEFORE closing (to get quantity and price)
	positions, err := tempTrader.GetPositions()
	if err != nil {
		logger.Infof("⚠️ Failed to get positions: %v", err)
	}

	var posQty float64
	var entryPrice float64
	for _, pos := range positions {
		if pos["symbol"] == req.Symbol && pos["side"] == strings.ToLower(req.Side) {
			if amt, ok := pos["positionAmt"].(float64); ok {
				posQty = amt
				if posQty < 0 {
					posQty = -posQty // Make positive
				}
			}
			if price, ok := pos["entryPrice"].(float64); ok {
				entryPrice = price
			}
			break
		}
	}

	// Execute close position operation
	var result map[string]interface{}
	var closeErr error

	if req.Side == "LONG" {
		result, closeErr = tempTrader.CloseLong(req.Symbol, 0) // 0 means close all
	} else if req.Side == "SHORT" {
		result, closeErr = tempTrader.CloseShort(req.Symbol, 0) // 0 means close all
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "side must be LONG or SHORT"})
		return
	}

	if closeErr != nil {
		logger.Infof("❌ Close position failed: symbol=%s, side=%s, error=%v", req.Symbol, req.Side, closeErr)
		SafeInternalError(c, "Close position", closeErr)
		return
	}

	logger.Infof("✅ Position closed successfully: symbol=%s, side=%s, qty=%.6f, result=%v", req.Symbol, req.Side, posQty, result)

	// Record order to database (for chart markers and history)
	s.recordClosePositionOrder(traderID, exchangeCfg.ID, exchangeCfg.ExchangeType, req.Symbol, req.Side, posQty, entryPrice, result)

	c.JSON(http.StatusOK, gin.H{
		"message": "Position closed successfully",
		"symbol":  req.Symbol,
		"side":    req.Side,
		"result":  result,
	})
}

// recordClosePositionOrder Record close position order to database (Lighter version - direct FILLED status)
func (s *Server) recordClosePositionOrder(traderID, exchangeID, exchangeType, symbol, side string, quantity, exitPrice float64, result map[string]interface{}) {
	// Skip for exchanges with OrderSync - let the background sync handle it to avoid duplicates
	switch exchangeType {
	case "binance", "lighter", "hyperliquid", "bybit", "okx", "bitget", "aster", "gate":
		logger.Infof("  📝 Close order will be synced by OrderSync, skipping immediate record")
		return
	}

	// Check if order was placed (skip if NO_POSITION)
	status, _ := result["status"].(string)
	if status == "NO_POSITION" {
		logger.Infof("  ⚠️ No position to close, skipping order record")
		return
	}

	// Get order ID from result
	var orderID string
	switch v := result["orderId"].(type) {
	case int64:
		orderID = fmt.Sprintf("%d", v)
	case float64:
		orderID = fmt.Sprintf("%.0f", v)
	case string:
		orderID = v
	default:
		orderID = fmt.Sprintf("%v", v)
	}

	if orderID == "" || orderID == "0" {
		logger.Infof("  ⚠️ Order ID is empty, skipping record")
		return
	}

	// Determine order action based on side
	var orderAction string
	if side == "LONG" {
		orderAction = "close_long"
	} else {
		orderAction = "close_short"
	}

	// Use entry price if exit price not available
	if exitPrice == 0 {
		exitPrice = quantity * 100 // Rough estimate if we don't have price
	}

	// Estimate fee (0.04% for Lighter taker)
	fee := exitPrice * quantity * 0.0004

	// Create order record - DIRECTLY as FILLED (Lighter market orders fill immediately)
	orderRecord := &store.TraderOrder{
		TraderID:        traderID,
		ExchangeID:      exchangeID,
		ExchangeType:    exchangeType,
		ExchangeOrderID: orderID,
		Symbol:          symbol,
		PositionSide:    side,
		OrderAction:     orderAction,
		Type:            "MARKET",
		Side:            getSideFromAction(orderAction),
		Quantity:        quantity,
		Price:           0, // Market order
		Status:          "FILLED",
		FilledQuantity:  quantity,
		AvgFillPrice:    exitPrice,
		Commission:      fee,
		FilledAt:        time.Now().UTC().UnixMilli(),
		CreatedAt:       time.Now().UTC().UnixMilli(),
		UpdatedAt:       time.Now().UTC().UnixMilli(),
	}

	if err := s.store.Order().CreateOrder(orderRecord); err != nil {
		logger.Infof("  ⚠️ Failed to record order: %v", err)
		return
	}

	logger.Infof("  ✅ Order recorded as FILLED: %s [%s] %s qty=%.6f price=%.6f", orderID, orderAction, symbol, quantity, exitPrice)

	// Create fill record immediately
	tradeID := fmt.Sprintf("%s-%d", orderID, time.Now().UnixNano())
	fillRecord := &store.TraderFill{
		TraderID:        traderID,
		ExchangeID:      exchangeID,
		ExchangeType:    exchangeType,
		OrderID:         orderRecord.ID,
		ExchangeOrderID: orderID,
		ExchangeTradeID: tradeID,
		Symbol:          symbol,
		Side:            getSideFromAction(orderAction),
		Price:           exitPrice,
		Quantity:        quantity,
		QuoteQuantity:   exitPrice * quantity,
		Commission:      fee,
		CommissionAsset: "USDT",
		RealizedPnL:     0,
		IsMaker:         false,
		CreatedAt:       time.Now().UTC().UnixMilli(),
	}

	if err := s.store.Order().CreateFill(fillRecord); err != nil {
		logger.Infof("  ⚠️ Failed to record fill: %v", err)
	} else {
		logger.Infof("  ✅ Fill record created: price=%.6f qty=%.6f", exitPrice, quantity)
	}
}

// pollAndUpdateOrderStatus Poll order status and update with fill data
func (s *Server) pollAndUpdateOrderStatus(orderRecordID int64, traderID, exchangeID, exchangeType, orderID, symbol, orderAction string, tempTrader trader.Trader) {
	var actualPrice float64
	var actualQty float64
	var fee float64

	// Wait a bit for order to be filled
	time.Sleep(500 * time.Millisecond)

	// For Lighter, use GetTrades instead of GetOrderStatus (market orders are filled immediately)
	if exchangeType == "lighter" {
		s.pollLighterTradeHistory(orderRecordID, traderID, exchangeID, exchangeType, orderID, symbol, orderAction, tempTrader)
		return
	}

	// For other exchanges, poll GetOrderStatus
	for i := 0; i < 5; i++ {
		status, err := tempTrader.GetOrderStatus(symbol, orderID)
		if err != nil {
			logger.Infof("  ⚠️ GetOrderStatus failed (attempt %d/5): %v", i+1, err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if err == nil {
			statusStr, _ := status["status"].(string)
			if statusStr == "FILLED" {
				// Get actual fill price
				if avgPrice, ok := status["avgPrice"].(float64); ok && avgPrice > 0 {
					actualPrice = avgPrice
				}
				// Get actual executed quantity
				if execQty, ok := status["executedQty"].(float64); ok && execQty > 0 {
					actualQty = execQty
				}
				// Get commission/fee
				if commission, ok := status["commission"].(float64); ok {
					fee = commission
				}

				logger.Infof("  ✅ Order filled: avgPrice=%.6f, qty=%.6f, fee=%.6f", actualPrice, actualQty, fee)

				// Update order status to FILLED
				if err := s.store.Order().UpdateOrderStatus(orderRecordID, "FILLED", actualQty, actualPrice, fee); err != nil {
					logger.Infof("  ⚠️ Failed to update order status: %v", err)
					return
				}

				// Record fill details
				tradeID := fmt.Sprintf("%s-%d", orderID, time.Now().UnixNano())
				fillRecord := &store.TraderFill{
					TraderID:        traderID,
					ExchangeID:      exchangeID,
					ExchangeType:    exchangeType,
					OrderID:         orderRecordID,
					ExchangeOrderID: orderID,
					ExchangeTradeID: tradeID,
					Symbol:          symbol,
					Side:            getSideFromAction(orderAction),
					Price:           actualPrice,
					Quantity:        actualQty,
					QuoteQuantity:   actualPrice * actualQty,
					Commission:      fee,
					CommissionAsset: "USDT",
					RealizedPnL:     0,
					IsMaker:         false,
					CreatedAt:       time.Now().UTC().UnixMilli(),
				}

				if err := s.store.Order().CreateFill(fillRecord); err != nil {
					logger.Infof("  ⚠️ Failed to record fill: %v", err)
				} else {
					logger.Infof("  📝 Fill recorded: price=%.6f, qty=%.6f", actualPrice, actualQty)
				}

				return
			} else if statusStr == "CANCELED" || statusStr == "EXPIRED" || statusStr == "REJECTED" {
				logger.Infof("  ⚠️ Order %s, updating status", statusStr)
				s.store.Order().UpdateOrderStatus(orderRecordID, statusStr, 0, 0, 0)
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	logger.Infof("  ⚠️ Failed to confirm order fill after polling, order may still be pending")
}

// pollLighterTradeHistory No longer used - Lighter orders are marked as FILLED immediately
// Keeping this function stub for compatibility with other exchanges
func (s *Server) pollLighterTradeHistory(orderRecordID int64, traderID, exchangeID, exchangeType, orderID, symbol, orderAction string, tempTrader trader.Trader) {
	// For Lighter, orders are now recorded as FILLED immediately in recordClosePositionOrder
	// This function is no longer called for Lighter exchange
	logger.Infof("  ℹ️ pollLighterTradeHistory called but not needed (order already marked FILLED)")
}

// getSideFromAction Get order side (BUY/SELL) from order action
func getSideFromAction(action string) string {
	switch action {
	case "open_long", "close_short":
		return "BUY"
	case "open_short", "close_long":
		return "SELL"
	default:
		return "BUY"
	}
}
