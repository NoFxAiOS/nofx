package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"nofx/auth"
	"nofx/backtest"
	"nofx/config"
	"nofx/crypto"
	"nofx/decision"
	"nofx/logger"
	"nofx/manager"
	"nofx/store"
	"nofx/trader"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Server HTTP API server
type Server struct {
	router          *gin.Engine
	traderManager   *manager.TraderManager
	store           *store.Store
	cryptoHandler   *CryptoHandler
	backtestManager *backtest.Manager
	httpServer      *http.Server
	port            int
}

// NewServer Creates API server
func NewServer(traderManager *manager.TraderManager, st *store.Store, cryptoService *crypto.CryptoService, backtestManager *backtest.Manager, port int) *Server {
	// Set to Release mode (reduce log output)
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	// Enable CORS
	router.Use(corsMiddleware())

	// Create crypto handler
	cryptoHandler := NewCryptoHandler(cryptoService)

	s := &Server{
		router:          router,
		traderManager:   traderManager,
		store:           st,
		cryptoHandler:   cryptoHandler,
		backtestManager: backtestManager,
		port:            port,
	}

	// Setup routes
	s.setupRoutes()

	return s
}

// corsMiddleware CORS middleware
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// setupRoutes Setup routes
func (s *Server) setupRoutes() {
	// API route group
	api := s.router.Group("/api")
	{
		// Health check
		api.Any("/health", s.handleHealth)

		// Admin login (used in admin mode, public)

		// System supported models and exchanges (no authentication required)
		api.GET("/supported-models", s.handleGetSupportedModels)
		api.GET("/supported-exchanges", s.handleGetSupportedExchanges)

		// System config (no authentication required, for frontend to determine admin mode/registration status)
		api.GET("/config", s.handleGetSystemConfig)

		// Crypto related endpoints (no authentication required)
		api.GET("/crypto/public-key", s.cryptoHandler.HandleGetPublicKey)
		api.POST("/crypto/decrypt", s.cryptoHandler.HandleDecryptSensitiveData)

		// System prompt template management (no authentication required)
		api.GET("/prompt-templates", s.handleGetPromptTemplates)
		api.GET("/prompt-templates/:name", s.handleGetPromptTemplate)

		// Public competition data (no authentication required)
		api.GET("/traders", s.handlePublicTraderList)
		api.GET("/competition", s.handlePublicCompetition)
		api.GET("/top-traders", s.handleTopTraders)
		api.GET("/equity-history", s.handleEquityHistory)
		api.POST("/equity-history-batch", s.handleEquityHistoryBatch)
		api.GET("/traders/:id/public-config", s.handleGetPublicTraderConfig)

		// Authentication related routes (no authentication required)
		api.POST("/register", s.handleRegister)
		api.POST("/login", s.handleLogin)
		api.POST("/verify-otp", s.handleVerifyOTP)
		api.POST("/complete-registration", s.handleCompleteRegistration)

		// Routes requiring authentication
		protected := api.Group("/", s.authMiddleware())
		{
			// Logout (add to blacklist)
			protected.POST("/logout", s.handleLogout)

			// Server IP query (requires authentication, for whitelist configuration)
			protected.GET("/server-ip", s.handleGetServerIP)

			// AI trader management
			protected.GET("/my-traders", s.handleTraderList)
			protected.GET("/traders/:id/config", s.handleGetTraderConfig)
			protected.POST("/traders", s.handleCreateTrader)
			protected.PUT("/traders/:id", s.handleUpdateTrader)
			protected.DELETE("/traders/:id", s.handleDeleteTrader)
			protected.POST("/traders/:id/start", s.handleStartTrader)
			protected.POST("/traders/:id/stop", s.handleStopTrader)
			protected.PUT("/traders/:id/prompt", s.handleUpdateTraderPrompt)
			protected.POST("/traders/:id/sync-balance", s.handleSyncBalance)
			protected.POST("/traders/:id/close-position", s.handleClosePosition)

			// AI model configuration
			protected.GET("/models", s.handleGetModelConfigs)
			protected.PUT("/models", s.handleUpdateModelConfigs)

			// Exchange configuration
			protected.GET("/exchanges", s.handleGetExchangeConfigs)
			protected.PUT("/exchanges", s.handleUpdateExchangeConfigs)

			// Strategy management
			protected.GET("/strategies", s.handleGetStrategies)
			protected.GET("/strategies/active", s.handleGetActiveStrategy)
			protected.GET("/strategies/default-config", s.handleGetDefaultStrategyConfig)
			protected.GET("/strategies/templates", s.handleGetPromptTemplates)
			protected.POST("/strategies/preview-prompt", s.handlePreviewPrompt)
			protected.POST("/strategies/test-run", s.handleStrategyTestRun)
			protected.GET("/strategies/:id", s.handleGetStrategy)
			protected.POST("/strategies", s.handleCreateStrategy)
			protected.PUT("/strategies/:id", s.handleUpdateStrategy)
			protected.DELETE("/strategies/:id", s.handleDeleteStrategy)
			protected.POST("/strategies/:id/activate", s.handleActivateStrategy)
			protected.POST("/strategies/:id/duplicate", s.handleDuplicateStrategy)

			// Data for specified trader (using query parameter ?trader_id=xxx)
			protected.GET("/status", s.handleStatus)
			protected.GET("/account", s.handleAccount)
			protected.GET("/positions", s.handlePositions)
			protected.GET("/decisions", s.handleDecisions)
			protected.GET("/decisions/latest", s.handleLatestDecisions)
			protected.GET("/statistics", s.handleStatistics)
		}
	}
}

// handleHealth Health check
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   c.Request.Context().Value("time"),
	})
}

// handleGetSystemConfig Get system configuration (configuration that client needs to know)
func (s *Server) handleGetSystemConfig(c *gin.Context) {
	cfg := config.Get()

	c.JSON(http.StatusOK, gin.H{
		"registration_enabled": cfg.RegistrationEnabled,
		"btc_eth_leverage":     10, // Default value
		"altcoin_leverage":     5,  // Default value
	})
}

// handleGetServerIP Get server IP address (for whitelist configuration)
func (s *Server) handleGetServerIP(c *gin.Context) {
	// Try to get public IP via third-party API
	publicIP := getPublicIPFromAPI()

	// If third-party API fails, get first public IP from network interface
	if publicIP == "" {
		publicIP = getPublicIPFromInterface()
	}

	// If still cannot get it, return error
	if publicIP == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to get public IP address"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"public_ip": publicIP,
		"message":   "Please add this IP address to the whitelist",
	})
}

// getPublicIPFromAPI Get public IP via third-party API
func getPublicIPFromAPI() string {
	// Try multiple public IP query services
	services := []string{
		"https://api.ipify.org?format=text",
		"https://icanhazip.com",
		"https://ifconfig.me",
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for _, service := range services {
		resp, err := client.Get(service)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body := make([]byte, 128)
			n, err := resp.Body.Read(body)
			if err != nil && err.Error() != "EOF" {
				continue
			}

			ip := strings.TrimSpace(string(body[:n]))
			// Verify if it's a valid IP address
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	return ""
}

// getPublicIPFromInterface Get first public IP from network interface
func getPublicIPFromInterface() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		// Skip disabled interfaces and loopback interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// Only consider IPv4 addresses
			if ip.To4() != nil {
				ipStr := ip.String()
				// Exclude private IP address ranges
				if !isPrivateIP(ip) {
					return ipStr
				}
			}
		}
	}

	return ""
}

// isPrivateIP Determine if it's a private IP address
func isPrivateIP(ip net.IP) bool {
	// Private IP address ranges:
	// 10.0.0.0/8
	// 172.16.0.0/12
	// 192.168.0.0/16
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	for _, cidr := range privateRanges {
		_, subnet, _ := net.ParseCIDR(cidr)
		if subnet.Contains(ip) {
			return true
		}
	}

	return false
}

// getTraderFromQuery Get trader from query parameter
func (s *Server) getTraderFromQuery(c *gin.Context) (*manager.TraderManager, string, error) {
	userID := c.GetString("user_id")
	traderID := c.Query("trader_id")

	// Ensure user's traders are loaded into memory
	err := s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to load traders for user %s: %v", userID, err)
	}

	if traderID == "" {
		// If no trader_id specified, return first trader for this user
		ids := s.traderManager.GetTraderIDs()
		if len(ids) == 0 {
			return nil, "", fmt.Errorf("No available traders")
		}

		// Get user's trader list, prioritize returning user's own traders
		userTraders, err := s.store.Trader().List(userID)
		if err == nil && len(userTraders) > 0 {
			traderID = userTraders[0].ID
		} else {
			traderID = ids[0]
		}
	}

	return s.traderManager, traderID, nil
}

// AI trader management related structures
type CreateTraderRequest struct {
	Name                string  `json:"name" binding:"required"`
	AIModelID           string  `json:"ai_model_id" binding:"required"`
	ExchangeID          string  `json:"exchange_id" binding:"required"`
	StrategyID          string  `json:"strategy_id"` // Strategy ID (new version)
	InitialBalance      float64 `json:"initial_balance"`
	ScanIntervalMinutes int     `json:"scan_interval_minutes"`
	IsCrossMargin       *bool   `json:"is_cross_margin"` // Pointer type, nil means use default value true
	// The following fields are kept for backward compatibility, new version uses strategy config
	BTCETHLeverage       int    `json:"btc_eth_leverage"`
	AltcoinLeverage      int    `json:"altcoin_leverage"`
	TradingSymbols       string `json:"trading_symbols"`
	CustomPrompt         string `json:"custom_prompt"`
	OverrideBasePrompt   bool   `json:"override_base_prompt"`
	SystemPromptTemplate string `json:"system_prompt_template"` // System prompt template name
	UseCoinPool          bool   `json:"use_coin_pool"`
	UseOITop             bool   `json:"use_oi_top"`
}

type ModelConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Enabled      bool   `json:"enabled"`
	APIKey       string `json:"apiKey,omitempty"`
	CustomAPIURL string `json:"customApiUrl,omitempty"`
}

// SafeModelConfig Safe model configuration structure (does not contain sensitive information)
type SafeModelConfig struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Provider        string `json:"provider"`
	Enabled         bool   `json:"enabled"`
	CustomAPIURL    string `json:"customApiUrl"`    // Custom API URL (usually not sensitive)
	CustomModelName string `json:"customModelName"` // Custom model name (not sensitive)
}

type ExchangeConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"` // "cex" or "dex"
	Enabled   bool   `json:"enabled"`
	APIKey    string `json:"apiKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
	Testnet   bool   `json:"testnet,omitempty"`
}

// SafeExchangeConfig Safe exchange configuration structure (does not contain sensitive information)
type SafeExchangeConfig struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Type                  string `json:"type"` // "cex" or "dex"
	Enabled               bool   `json:"enabled"`
	Testnet               bool   `json:"testnet,omitempty"`
	HyperliquidWalletAddr string `json:"hyperliquidWalletAddr"` // Hyperliquid wallet address (not sensitive)
	AsterUser             string `json:"asterUser"`             // Aster username (not sensitive)
	AsterSigner           string `json:"asterSigner"`           // Aster signer (not sensitive)
}

type UpdateModelConfigRequest struct {
	Models map[string]struct {
		Enabled         bool   `json:"enabled"`
		APIKey          string `json:"api_key"`
		CustomAPIURL    string `json:"custom_api_url"`
		CustomModelName string `json:"custom_model_name"`
	} `json:"models"`
}

type UpdateExchangeConfigRequest struct {
	Exchanges map[string]struct {
		Enabled                 bool   `json:"enabled"`
		APIKey                  string `json:"api_key"`
		SecretKey               string `json:"secret_key"`
		Passphrase              string `json:"passphrase"` // OKX specific
		Testnet                 bool   `json:"testnet"`
		HyperliquidWalletAddr   string `json:"hyperliquid_wallet_addr"`
		AsterUser               string `json:"aster_user"`
		AsterSigner             string `json:"aster_signer"`
		AsterPrivateKey         string `json:"aster_private_key"`
		LighterWalletAddr       string `json:"lighter_wallet_addr"`
		LighterPrivateKey       string `json:"lighter_private_key"`
		LighterAPIKeyPrivateKey string `json:"lighter_api_key_private_key"`
	} `json:"exchanges"`
}

// handleCreateTrader Create new AI trader
func (s *Server) handleCreateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	var req CreateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	// Generate trader ID
	traderID := fmt.Sprintf("%s_%s_%d", req.ExchangeID, req.AIModelID, time.Now().Unix())

	// Set default values
	isCrossMargin := true // Default to cross margin mode
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
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

		switch req.ExchangeID {
		case "binance":
			tempTrader = trader.NewFuturesTrader(exchangeCfg.APIKey, exchangeCfg.SecretKey, userID)
		case "hyperliquid":
			tempTrader, createErr = trader.NewHyperliquidTrader(
				exchangeCfg.APIKey, // private key
				exchangeCfg.HyperliquidWalletAddr,
				exchangeCfg.Testnet,
			)
		case "aster":
			tempTrader, createErr = trader.NewAsterTrader(
				exchangeCfg.AsterUser,
				exchangeCfg.AsterSigner,
				exchangeCfg.AsterPrivateKey,
			)
		case "bybit":
			tempTrader = trader.NewBybitTrader(
				exchangeCfg.APIKey,
				exchangeCfg.SecretKey,
			)
		default:
			logger.Infof("⚠️ Unsupported exchange type: %s, using user input for initial balance", req.ExchangeID)
		}

		if createErr != nil {
			logger.Infof("⚠️ Failed to create temporary trader, using user input for initial balance: %v", createErr)
		} else if tempTrader != nil {
			// Query actual balance
			balanceInfo, balanceErr := tempTrader.GetBalance()
			if balanceErr != nil {
				logger.Infof("⚠️ Failed to query exchange balance, using user input for initial balance: %v", balanceErr)
			} else {
				// Extract available balance - supports multiple field name formats
				if availableBalance, ok := balanceInfo["availableBalance"].(float64); ok && availableBalance > 0 {
					// Binance format: availableBalance (camelCase)
					actualBalance = availableBalance
					logger.Infof("✓ Queried exchange actual balance: %.2f USDT (user input: %.2f USDT)", actualBalance, req.InitialBalance)
				} else if availableBalance, ok := balanceInfo["available_balance"].(float64); ok && availableBalance > 0 {
					// Other format: available_balance (snake_case)
					actualBalance = availableBalance
					logger.Infof("✓ Queried exchange actual balance: %.2f USDT (user input: %.2f USDT)", actualBalance, req.InitialBalance)
				} else if totalBalance, ok := balanceInfo["totalWalletBalance"].(float64); ok && totalBalance > 0 {
					// Binance format: totalWalletBalance (camelCase)
					actualBalance = totalBalance
					logger.Infof("✓ Queried exchange total balance: %.2f USDT (user input: %.2f USDT)", actualBalance, req.InitialBalance)
				} else if totalBalance, ok := balanceInfo["balance"].(float64); ok && totalBalance > 0 {
					// Other format: balance
					actualBalance = totalBalance
					logger.Infof("✓ Queried exchange actual balance: %.2f USDT (user input: %.2f USDT)", actualBalance, req.InitialBalance)
				} else {
					logger.Infof("⚠️ Unable to extract available balance from balance info, balanceInfo=%v, using user input for initial balance", balanceInfo)
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
		UseCoinPool:          req.UseCoinPool,
		UseOITop:             req.UseOITop,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: systemPromptTemplate,
		IsCrossMargin:        isCrossMargin,
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            false,
	}

	// Save to database
	logger.Infof("🔧 DEBUG: Preparing to call CreateTrader")
	err = s.store.Trader().Create(traderRecord)
	if err != nil {
		logger.Infof("❌ Failed to create trader: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create trader: %v", err)})
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

// UpdateTraderRequest Update trader request
type UpdateTraderRequest struct {
	Name                string  `json:"name" binding:"required"`
	AIModelID           string  `json:"ai_model_id" binding:"required"`
	ExchangeID          string  `json:"exchange_id" binding:"required"`
	StrategyID          string  `json:"strategy_id"` // Strategy ID (new version)
	InitialBalance      float64 `json:"initial_balance"`
	ScanIntervalMinutes int     `json:"scan_interval_minutes"`
	IsCrossMargin       *bool   `json:"is_cross_margin"`
	// The following fields are kept for backward compatibility, new version uses strategy config
	BTCETHLeverage       int    `json:"btc_eth_leverage"`
	AltcoinLeverage      int    `json:"altcoin_leverage"`
	TradingSymbols       string `json:"trading_symbols"`
	CustomPrompt         string `json:"custom_prompt"`
	OverrideBasePrompt   bool   `json:"override_base_prompt"`
	SystemPromptTemplate string `json:"system_prompt_template"`
}

// handleUpdateTrader Update trader configuration
func (s *Server) handleUpdateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	var req UpdateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
	if scanIntervalMinutes <= 0 {
		scanIntervalMinutes = existingTrader.ScanIntervalMinutes // Keep original value
	} else if scanIntervalMinutes < 3 {
		scanIntervalMinutes = 3
	}

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
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            existingTrader.IsRunning, // Keep original value
	}

	// Update database
	err = s.store.Trader().Update(traderRecord)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update trader: %v", err)})
		return
	}

	// Reload traders into memory
	err = s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to reload user traders into memory: %v", err)
	}

	logger.Infof("✓ Trader updated successfully: %s (model: %s, exchange: %s)", req.Name, req.AIModelID, req.ExchangeID)

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete trader: %v", err)})
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

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		// Trader not in memory, try loading from database
		logger.Infof("🔄 Trader %s not in memory, trying to load...", traderID)
		if loadErr := s.traderManager.LoadUserTradersFromStore(s.store, userID); loadErr != nil {
			logger.Infof("❌ Failed to load user traders: %v", loadErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load trader: " + loadErr.Error()})
			return
		}
		// Try to get trader again
		trader, err = s.traderManager.GetTrader(traderID)
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
			c.JSON(http.StatusNotFound, gin.H{"error": "Failed to load trader, please check AI model, exchange and strategy configuration"})
			return
		}
	}

	// Check if trader is already running
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trader is already running"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update database
	err := s.store.Trader().UpdateCustomPrompt(userID, traderID, req.CustomPrompt, req.OverrideBasePrompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update custom prompt: %v", err)})
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

	switch traderConfig.ExchangeID {
	case "binance":
		tempTrader = trader.NewFuturesTrader(exchangeCfg.APIKey, exchangeCfg.SecretKey, userID)
	case "hyperliquid":
		tempTrader, createErr = trader.NewHyperliquidTrader(
			exchangeCfg.APIKey,
			exchangeCfg.HyperliquidWalletAddr,
			exchangeCfg.Testnet,
		)
	case "aster":
		tempTrader, createErr = trader.NewAsterTrader(
			exchangeCfg.AsterUser,
			exchangeCfg.AsterSigner,
			exchangeCfg.AsterPrivateKey,
		)
	case "bybit":
		tempTrader = trader.NewBybitTrader(
			exchangeCfg.APIKey,
			exchangeCfg.SecretKey,
		)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported exchange type"})
		return
	}

	if createErr != nil {
		logger.Infof("⚠️ Failed to create temporary trader: %v", createErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to connect to exchange: %v", createErr)})
		return
	}

	// Query actual balance
	balanceInfo, balanceErr := tempTrader.GetBalance()
	if balanceErr != nil {
		logger.Infof("⚠️ Failed to query exchange balance: %v", balanceErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to query balance: %v", balanceErr)})
		return
	}

	// Extract available balance
	var actualBalance float64
	if availableBalance, ok := balanceInfo["available_balance"].(float64); ok && availableBalance > 0 {
		actualBalance = availableBalance
	} else if availableBalance, ok := balanceInfo["availableBalance"].(float64); ok && availableBalance > 0 {
		actualBalance = availableBalance
	} else if totalBalance, ok := balanceInfo["balance"].(float64); ok && totalBalance > 0 {
		actualBalance = totalBalance
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to get available balance"})
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

	traderConfig := fullConfig.Trader
	exchangeCfg := fullConfig.Exchange

	if exchangeCfg == nil || !exchangeCfg.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Exchange not configured or not enabled"})
		return
	}

	// Create temporary trader to execute close position
	var tempTrader trader.Trader
	var createErr error

	switch traderConfig.ExchangeID {
	case "binance":
		tempTrader = trader.NewFuturesTrader(exchangeCfg.APIKey, exchangeCfg.SecretKey, userID)
	case "hyperliquid":
		tempTrader, createErr = trader.NewHyperliquidTrader(
			exchangeCfg.APIKey,
			exchangeCfg.HyperliquidWalletAddr,
			exchangeCfg.Testnet,
		)
	case "aster":
		tempTrader, createErr = trader.NewAsterTrader(
			exchangeCfg.AsterUser,
			exchangeCfg.AsterSigner,
			exchangeCfg.AsterPrivateKey,
		)
	case "bybit":
		tempTrader = trader.NewBybitTrader(
			exchangeCfg.APIKey,
			exchangeCfg.SecretKey,
		)
	case "okx":
		tempTrader = trader.NewOKXTrader(
			exchangeCfg.APIKey,
			exchangeCfg.SecretKey,
			exchangeCfg.Passphrase,
		)
	case "lighter":
		if exchangeCfg.LighterAPIKeyPrivateKey != "" {
			tempTrader, createErr = trader.NewLighterTraderV2(
				exchangeCfg.LighterPrivateKey,
				exchangeCfg.LighterWalletAddr,
				exchangeCfg.LighterAPIKeyPrivateKey,
				exchangeCfg.Testnet,
			)
		} else {
			tempTrader, createErr = trader.NewLighterTrader(
				exchangeCfg.LighterPrivateKey,
				exchangeCfg.LighterWalletAddr,
				exchangeCfg.Testnet,
			)
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported exchange type"})
		return
	}

	if createErr != nil {
		logger.Infof("⚠️ Failed to create temporary trader: %v", createErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to connect to exchange: %v", createErr)})
		return
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to close position: %v", closeErr)})
		return
	}

	logger.Infof("✅ Position closed successfully: symbol=%s, side=%s, result=%v", req.Symbol, req.Side, result)
	c.JSON(http.StatusOK, gin.H{
		"message": "Position closed successfully",
		"symbol":  req.Symbol,
		"side":    req.Side,
		"result":  result,
	})
}

// handleGetModelConfigs Get AI model configurations
func (s *Server) handleGetModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	logger.Infof("🔍 Querying AI model configs for user %s", userID)
	models, err := s.store.AIModel().List(userID)
	if err != nil {
		logger.Infof("❌ Failed to get AI model configs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get AI model configs: %v", err)})
		return
	}
	logger.Infof("✅ Found %d AI model configs", len(models))

	// Convert to safe response structure, remove sensitive information
	safeModels := make([]SafeModelConfig, len(models))
	for i, model := range models {
		safeModels[i] = SafeModelConfig{
			ID:              model.ID,
			Name:            model.Name,
			Provider:        model.Provider,
			Enabled:         model.Enabled,
			CustomAPIURL:    model.CustomAPIURL,
			CustomModelName: model.CustomModelName,
		}
	}

	c.JSON(http.StatusOK, safeModels)
}

// handleUpdateModelConfigs Update AI model configurations (encrypted data only)
func (s *Server) handleUpdateModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")

	// Read raw request body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Parse encrypted payload
	var encryptedPayload crypto.EncryptedPayload
	if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil {
		logger.Infof("❌ Failed to parse encrypted payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format, encrypted transmission required"})
		return
	}

	// Verify encrypted data
	if encryptedPayload.WrappedKey == "" {
		logger.Infof("❌ Detected unencrypted request (UserID: %s)", userID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "This endpoint only supports encrypted transmission, please use encrypted client",
			"code":    "ENCRYPTION_REQUIRED",
			"message": "Encrypted transmission is required for security reasons",
		})
		return
	}

	// Decrypt data
	decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
	if err != nil {
		logger.Infof("❌ Failed to decrypt model config (UserID: %s): %v", userID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decrypt data"})
		return
	}

	// Parse decrypted data
	var req UpdateModelConfigRequest
	if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
		logger.Infof("❌ Failed to parse decrypted data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
		return
	}
	logger.Infof("🔓 Decrypted model config data (UserID: %s)", userID)

	// Update each model's configuration
	for modelID, modelData := range req.Models {
		err := s.store.AIModel().Update(userID, modelID, modelData.Enabled, modelData.APIKey, modelData.CustomAPIURL, modelData.CustomModelName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update model %s: %v", modelID, err)})
			return
		}
	}

	// Reload all traders for this user to make new config take effect immediately
	err = s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to reload user traders into memory: %v", err)
		// Don't return error here since model config was successfully updated to database
	}

	logger.Infof("✓ AI model config updated: %+v", req.Models)
	c.JSON(http.StatusOK, gin.H{"message": "Model configuration updated"})
}

// handleGetExchangeConfigs Get exchange configurations
func (s *Server) handleGetExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	logger.Infof("🔍 Querying exchange configs for user %s", userID)
	exchanges, err := s.store.Exchange().List(userID)
	if err != nil {
		logger.Infof("❌ Failed to get exchange configs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get exchange configs: %v", err)})
		return
	}
	logger.Infof("✅ Found %d exchange configs", len(exchanges))

	// Convert to safe response structure, remove sensitive information
	safeExchanges := make([]SafeExchangeConfig, len(exchanges))
	for i, exchange := range exchanges {
		safeExchanges[i] = SafeExchangeConfig{
			ID:                    exchange.ID,
			Name:                  exchange.Name,
			Type:                  exchange.Type,
			Enabled:               exchange.Enabled,
			Testnet:               exchange.Testnet,
			HyperliquidWalletAddr: exchange.HyperliquidWalletAddr,
			AsterUser:             exchange.AsterUser,
			AsterSigner:           exchange.AsterSigner,
		}
	}

	c.JSON(http.StatusOK, safeExchanges)
}

// handleUpdateExchangeConfigs Update exchange configurations (encrypted data only)
func (s *Server) handleUpdateExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")

	// Read raw request body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Parse encrypted payload
	var encryptedPayload crypto.EncryptedPayload
	if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil {
		logger.Infof("❌ Failed to parse encrypted payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format, encrypted transmission required"})
		return
	}

	// Verify encrypted data
	if encryptedPayload.WrappedKey == "" {
		logger.Infof("❌ Detected unencrypted request (UserID: %s)", userID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "This endpoint only supports encrypted transmission, please use encrypted client",
			"code":    "ENCRYPTION_REQUIRED",
			"message": "Encrypted transmission is required for security reasons",
		})
		return
	}

	// Decrypt data
	decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
	if err != nil {
		logger.Infof("❌ Failed to decrypt exchange config (UserID: %s): %v", userID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decrypt data"})
		return
	}

	// Parse decrypted data
	var req UpdateExchangeConfigRequest
	if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
		logger.Infof("❌ Failed to parse decrypted data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
		return
	}
	logger.Infof("🔓 Decrypted exchange config data (UserID: %s)", userID)

	// Update each exchange's configuration
	for exchangeID, exchangeData := range req.Exchanges {
		err := s.store.Exchange().Update(userID, exchangeID, exchangeData.Enabled, exchangeData.APIKey, exchangeData.SecretKey, exchangeData.Passphrase, exchangeData.Testnet, exchangeData.HyperliquidWalletAddr, exchangeData.AsterUser, exchangeData.AsterSigner, exchangeData.AsterPrivateKey, exchangeData.LighterWalletAddr, exchangeData.LighterPrivateKey, exchangeData.LighterAPIKeyPrivateKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update exchange %s: %v", exchangeID, err)})
			return
		}
	}

	// Reload all traders for this user to make new config take effect immediately
	err = s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to reload user traders into memory: %v", err)
		// Don't return error here since exchange config was successfully updated to database
	}

	logger.Infof("✓ Exchange config updated: %+v", req.Exchanges)
	c.JSON(http.StatusOK, gin.H{"message": "Exchange configuration updated"})
}

// handleTraderList Trader list
func (s *Server) handleTraderList(c *gin.Context) {
	userID := c.GetString("user_id")
	traders, err := s.store.Trader().List(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get trader list: %v", err)})
		return
	}

	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		// Get real-time running status
		isRunning := trader.IsRunning
		if at, err := s.traderManager.GetTrader(trader.ID); err == nil {
			status := at.GetStatus()
			if running, ok := status["is_running"].(bool); ok {
				isRunning = running
			}
		}

		// Return complete AIModelID (e.g. "admin_deepseek"), don't truncate
		// Frontend needs complete ID to verify model exists (consistent with handleGetTraderConfig)
		result = append(result, map[string]interface{}{
			"trader_id":       trader.ID,
			"trader_name":     trader.Name,
			"ai_model":        trader.AIModelID, // Use complete ID
			"exchange_id":     trader.ExchangeID,
			"is_running":      isRunning,
			"initial_balance": trader.InitialBalance,
		})
	}

	c.JSON(http.StatusOK, result)
}

// handleGetTraderConfig Get trader detailed configuration
func (s *Server) handleGetTraderConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trader ID cannot be empty"})
		return
	}

	fullCfg, err := s.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Failed to get trader config: %v", err)})
		return
	}
	traderConfig := fullCfg.Trader

	// Get real-time running status
	isRunning := traderConfig.IsRunning
	if at, err := s.traderManager.GetTrader(traderID); err == nil {
		status := at.GetStatus()
		if running, ok := status["is_running"].(bool); ok {
			isRunning = running
		}
	}

	// Return complete model ID without conversion, consistent with frontend model list
	aiModelID := traderConfig.AIModelID

	result := map[string]interface{}{
		"trader_id":             traderConfig.ID,
		"trader_name":           traderConfig.Name,
		"ai_model":              aiModelID,
		"exchange_id":           traderConfig.ExchangeID,
		"initial_balance":       traderConfig.InitialBalance,
		"scan_interval_minutes": traderConfig.ScanIntervalMinutes,
		"btc_eth_leverage":      traderConfig.BTCETHLeverage,
		"altcoin_leverage":      traderConfig.AltcoinLeverage,
		"trading_symbols":       traderConfig.TradingSymbols,
		"custom_prompt":         traderConfig.CustomPrompt,
		"override_base_prompt":  traderConfig.OverrideBasePrompt,
		"is_cross_margin":       traderConfig.IsCrossMargin,
		"use_coin_pool":         traderConfig.UseCoinPool,
		"use_oi_top":            traderConfig.UseOITop,
		"is_running":            isRunning,
	}

	c.JSON(http.StatusOK, result)
}

// handleStatus System status
func (s *Server) handleStatus(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	status := trader.GetStatus()
	c.JSON(http.StatusOK, status)
}

// handleAccount Account information
func (s *Server) handleAccount(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	logger.Infof("📊 Received account info request [%s]", trader.GetName())
	account, err := trader.GetAccountInfo()
	if err != nil {
		logger.Infof("❌ Failed to get account info [%s]: %v", trader.GetName(), err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get account info: %v", err),
		})
		return
	}

	logger.Infof("✓ Returning account info [%s]: equity=%.2f, available=%.2f, pnl=%.2f (%.2f%%)",
		trader.GetName(),
		account["total_equity"],
		account["available_balance"],
		account["total_pnl"],
		account["total_pnl_pct"])
	c.JSON(http.StatusOK, account)
}

// handlePositions Position list
func (s *Server) handlePositions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	positions, err := trader.GetPositions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get position list: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, positions)
}

// handleDecisions Decision log list
func (s *Server) handleDecisions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Get all historical decision records (unlimited)
	records, err := trader.GetStore().Decision().GetLatestRecords(trader.GetID(), 10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get decision log: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, records)
}

// handleLatestDecisions Latest decision logs (most recent 5, newest first)
func (s *Server) handleLatestDecisions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	records, err := trader.GetStore().Decision().GetLatestRecords(trader.GetID(), 5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get decision log: %v", err),
		})
		return
	}

	// Reverse array to put newest first (for list display)
	// GetLatestRecords returns oldest to newest (for charts), here we need newest to oldest
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	c.JSON(http.StatusOK, records)
}

// handleStatistics Statistics information
func (s *Server) handleStatistics(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	stats, err := trader.GetStore().Decision().GetStatistics(trader.GetID())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get statistics: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// handleCompetition Competition overview (compare all traders)
func (s *Server) handleCompetition(c *gin.Context) {
	userID := c.GetString("user_id")

	// Ensure user's traders are loaded into memory
	err := s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to load traders for user %s: %v", userID, err)
	}

	competition, err := s.traderManager.GetCompetitionData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get competition data: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, competition)
}

// handleEquityHistory Return rate historical data
// Query directly from database, not dependent on trader in memory (so historical data can be retrieved after restart)
func (s *Server) handleEquityHistory(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get equity historical data from new equity table
	// Every 3 minutes per cycle: 10000 records = about 20 days of data
	snapshots, err := s.store.Equity().GetLatest(traderID, 10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get historical data: %v", err),
		})
		return
	}

	if len(snapshots) == 0 {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	// Build return rate historical data points
	type EquityPoint struct {
		Timestamp        string  `json:"timestamp"`
		TotalEquity      float64 `json:"total_equity"`      // Account equity (wallet + unrealized)
		AvailableBalance float64 `json:"available_balance"` // Available balance
		TotalPnL         float64 `json:"total_pnl"`         // Total PnL (unrealized PnL)
		TotalPnLPct      float64 `json:"total_pnl_pct"`     // Total PnL percentage
		PositionCount    int     `json:"position_count"`    // Position count
		MarginUsedPct    float64 `json:"margin_used_pct"`   // Margin used percentage
	}

	// Use the balance of the first record as initial balance to calculate return rate
	initialBalance := snapshots[0].Balance
	if initialBalance == 0 {
		initialBalance = 1 // Avoid division by zero
	}

	var history []EquityPoint
	for _, snap := range snapshots {
		// Calculate PnL percentage
		totalPnLPct := 0.0
		if initialBalance > 0 {
			totalPnLPct = (snap.UnrealizedPnL / initialBalance) * 100
		}

		history = append(history, EquityPoint{
			Timestamp:        snap.Timestamp.Format("2006-01-02 15:04:05"),
			TotalEquity:      snap.TotalEquity,
			AvailableBalance: snap.Balance,
			TotalPnL:         snap.UnrealizedPnL,
			TotalPnLPct:      totalPnLPct,
			PositionCount:    snap.PositionCount,
			MarginUsedPct:    snap.MarginUsedPct,
		})
	}

	c.JSON(http.StatusOK, history)
}

// authMiddleware JWT authentication middleware
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
			c.Abort()
			return
		}

		// Check Bearer token format
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization format"})
			c.Abort()
			return
		}

		tokenString := tokenParts[1]

		// Blacklist check
		if auth.IsTokenBlacklisted(tokenString) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token expired, please login again"})
			c.Abort()
			return
		}

		// Validate JWT token
		claims, err := auth.ValidateJWT(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token: " + err.Error()})
			c.Abort()
			return
		}

		// Store user information in context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Next()
	}
}

// handleLogout Add current token to blacklist
func (s *Server) handleLogout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
		return
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization format"})
		return
	}
	tokenString := parts[1]
	claims, err := auth.ValidateJWT(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}
	var exp time.Time
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	} else {
		exp = time.Now().Add(24 * time.Hour)
	}
	auth.BlacklistToken(tokenString, exp)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}

// handleRegister Handle user registration request
func (s *Server) handleRegister(c *gin.Context) {
	// Check if registration is allowed
	if !config.Get().RegistrationEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "Registration is disabled"})
		return
	}

	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if email already exists
	_, err := s.store.User().GetByEmail(req.Email)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	// Generate password hash
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Password processing failed"})
		return
	}

	// Generate OTP secret
	otpSecret, err := auth.GenerateOTPSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OTP secret generation failed"})
		return
	}

	// Create user (unverified OTP status)
	userID := uuid.New().String()
	user := &store.User{
		ID:           userID,
		Email:        req.Email,
		PasswordHash: passwordHash,
		OTPSecret:    otpSecret,
		OTPVerified:  false,
	}

	err = s.store.User().Create(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user: " + err.Error()})
		return
	}

	// Return OTP setup information
	qrCodeURL := auth.GetOTPQRCodeURL(otpSecret, req.Email)
	c.JSON(http.StatusOK, gin.H{
		"user_id":     userID,
		"email":       req.Email,
		"otp_secret":  otpSecret,
		"qr_code_url": qrCodeURL,
		"message":     "Please scan the QR code with Google Authenticator and verify OTP",
	})
}

// handleCompleteRegistration Complete registration (verify OTP)
func (s *Server) handleCompleteRegistration(c *gin.Context) {
	var req struct {
		UserID  string `json:"user_id" binding:"required"`
		OTPCode string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user information
	user, err := s.store.User().GetByID(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
		return
	}

	// Verify OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OTP code error"})
		return
	}

	// Update user OTP verified status
	err = s.store.User().UpdateOTPVerified(req.UserID, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user status"})
		return
	}

	// Generate JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Initialize default model and exchange configs for user
	err = s.initUserDefaultConfigs(user.ID)
	if err != nil {
		logger.Infof("Failed to initialize user default configs: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
		"email":   user.Email,
		"message": "Registration completed",
	})
}

// handleLogin Handle user login request
func (s *Server) handleLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user information
	user, err := s.store.User().GetByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email or password incorrect"})
		return
	}

	// Verify password
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email or password incorrect"})
		return
	}

	// Check if OTP is verified
	if !user.OTPVerified {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":              "Account has not completed OTP setup",
			"user_id":            user.ID,
			"requires_otp_setup": true,
		})
		return
	}

	// Return status requiring OTP verification
	c.JSON(http.StatusOK, gin.H{
		"user_id":      user.ID,
		"email":        user.Email,
		"message":      "Please enter Google Authenticator code",
		"requires_otp": true,
	})
}

// handleVerifyOTP Verify OTP and complete login
func (s *Server) handleVerifyOTP(c *gin.Context) {
	var req struct {
		UserID  string `json:"user_id" binding:"required"`
		OTPCode string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user information
	user, err := s.store.User().GetByID(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
		return
	}

	// Verify OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification code error"})
		return
	}

	// Generate JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
		"email":   user.Email,
		"message": "Login successful",
	})
}

// handleResetPassword Reset password (via email + OTP verification)
func (s *Server) handleResetPassword(c *gin.Context) {
	var req struct {
		Email       string `json:"email" binding:"required,email"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
		OTPCode     string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Query user
	user, err := s.store.User().GetByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email does not exist"})
		return
	}

	// Verify OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Google Authenticator code error"})
		return
	}

	// Generate new password hash
	newPasswordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Password processing failed"})
		return
	}

	// Update password
	err = s.store.User().UpdatePassword(user.ID, newPasswordHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Password update failed"})
		return
	}

	logger.Infof("✓ User %s password has been reset", user.Email)
	c.JSON(http.StatusOK, gin.H{"message": "Password reset successful, please login with new password"})
}

// initUserDefaultConfigs Initialize default model and exchange configs for new user
func (s *Server) initUserDefaultConfigs(userID string) error {
	// Commented out auto-creation of default configs, let users add manually
	// This way new users won't have config items automatically after registration
	logger.Infof("User %s registration completed, waiting for manual AI model and exchange configuration", userID)
	return nil
}

// handleGetSupportedModels Get list of AI models supported by the system
func (s *Server) handleGetSupportedModels(c *gin.Context) {
	// Return system-supported AI models (get from default user)
	models, err := s.store.AIModel().List("default")
	if err != nil {
		logger.Infof("❌ Failed to get supported AI models: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get supported AI models"})
		return
	}

	c.JSON(http.StatusOK, models)
}

// handleGetSupportedExchanges Get list of exchanges supported by the system
func (s *Server) handleGetSupportedExchanges(c *gin.Context) {
	// Return system-supported exchanges (get from default user)
	exchanges, err := s.store.Exchange().List("default")
	if err != nil {
		logger.Infof("❌ Failed to get supported exchanges: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get supported exchanges"})
		return
	}

	// Convert to safe response structure, remove sensitive information
	safeExchanges := make([]SafeExchangeConfig, len(exchanges))
	for i, exchange := range exchanges {
		safeExchanges[i] = SafeExchangeConfig{
			ID:                    exchange.ID,
			Name:                  exchange.Name,
			Type:                  exchange.Type,
			Enabled:               exchange.Enabled,
			Testnet:               exchange.Testnet,
			HyperliquidWalletAddr: "", // Default config does not include wallet address
			AsterUser:             "", // Default config does not include user info
			AsterSigner:           "",
		}
	}

	c.JSON(http.StatusOK, safeExchanges)
}

// Start Start server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	logger.Infof("🌐 API server starting at http://localhost%s", addr)
	logger.Infof("📊 API Documentation:")
	logger.Infof("  • GET  /api/health           - Health check")
	logger.Infof("  • GET  /api/traders          - Public AI trader leaderboard top 50 (no auth required)")
	logger.Infof("  • GET  /api/competition      - Public competition data (no auth required)")
	logger.Infof("  • GET  /api/top-traders      - Top 5 trader data (no auth required, for performance comparison)")
	logger.Infof("  • GET  /api/equity-history?trader_id=xxx - Public return rate historical data (no auth required, for competition)")
	logger.Infof("  • GET  /api/equity-history-batch?trader_ids=a,b,c - Batch get historical data (no auth required, performance comparison optimization)")
	logger.Infof("  • GET  /api/traders/:id/public-config - Public trader config (no auth required, no sensitive info)")
	logger.Infof("  • POST /api/traders          - Create new AI trader")
	logger.Infof("  • DELETE /api/traders/:id    - Delete AI trader")
	logger.Infof("  • POST /api/traders/:id/start - Start AI trader")
	logger.Infof("  • POST /api/traders/:id/stop  - Stop AI trader")
	logger.Infof("  • GET  /api/models           - Get AI model config")
	logger.Infof("  • PUT  /api/models           - Update AI model config")
	logger.Infof("  • GET  /api/exchanges        - Get exchange config")
	logger.Infof("  • PUT  /api/exchanges        - Update exchange config")
	logger.Infof("  • GET  /api/status?trader_id=xxx     - Specified trader's system status")
	logger.Infof("  • GET  /api/account?trader_id=xxx    - Specified trader's account info")
	logger.Infof("  • GET  /api/positions?trader_id=xxx  - Specified trader's position list")
	logger.Infof("  • GET  /api/decisions?trader_id=xxx  - Specified trader's decision log")
	logger.Infof("  • GET  /api/decisions/latest?trader_id=xxx - Specified trader's latest decisions")
	logger.Infof("  • GET  /api/statistics?trader_id=xxx - Specified trader's statistics")
	logger.Infof("  • GET  /api/performance?trader_id=xxx - Specified trader's AI learning performance analysis")
	logger.Info()

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	return s.httpServer.ListenAndServe()
}

// Shutdown Gracefully shutdown server
func (s *Server) Shutdown() error {
	if s.httpServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// handleGetPromptTemplates Get all system prompt template list
func (s *Server) handleGetPromptTemplates(c *gin.Context) {
	// Import decision package
	templates := decision.GetAllPromptTemplates()

	// Convert to response format
	response := make([]map[string]interface{}, 0, len(templates))
	for _, tmpl := range templates {
		response = append(response, map[string]interface{}{
			"name": tmpl.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": response,
	})
}

// handleGetPromptTemplate Get prompt template content by specified name
func (s *Server) handleGetPromptTemplate(c *gin.Context) {
	templateName := c.Param("name")

	template, err := decision.GetPromptTemplate(templateName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Template does not exist: %s", templateName)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":    template.Name,
		"content": template.Content,
	})
}

// handlePublicTraderList Get public trader list (no authentication required)
func (s *Server) handlePublicTraderList(c *gin.Context) {
	// Get trader information from all users
	competition, err := s.traderManager.GetCompetitionData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get trader list: %v", err),
		})
		return
	}

	// Get traders array
	tradersData, exists := competition["traders"]
	if !exists {
		c.JSON(http.StatusOK, []map[string]interface{}{})
		return
	}

	traders, ok := tradersData.([]map[string]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Trader data format error",
		})
		return
	}

	// Return trader basic information, filter sensitive information
	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		result = append(result, map[string]interface{}{
			"trader_id":       trader["trader_id"],
			"trader_name":     trader["trader_name"],
			"ai_model":        trader["ai_model"],
			"exchange":        trader["exchange"],
			"is_running":      trader["is_running"],
			"total_equity":    trader["total_equity"],
			"total_pnl":       trader["total_pnl"],
			"total_pnl_pct":   trader["total_pnl_pct"],
			"position_count":  trader["position_count"],
			"margin_used_pct": trader["margin_used_pct"],
		})
	}

	c.JSON(http.StatusOK, result)
}

// handlePublicCompetition Get public competition data (no authentication required)
func (s *Server) handlePublicCompetition(c *gin.Context) {
	competition, err := s.traderManager.GetCompetitionData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get competition data: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, competition)
}

// handleTopTraders Get top 5 trader data (no authentication required, for performance comparison)
func (s *Server) handleTopTraders(c *gin.Context) {
	topTraders, err := s.traderManager.GetTopTradersData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get top 10 trader data: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, topTraders)
}

// handleEquityHistoryBatch Batch get return rate historical data for multiple traders (no authentication required, for performance comparison)
func (s *Server) handleEquityHistoryBatch(c *gin.Context) {
	var requestBody struct {
		TraderIDs []string `json:"trader_ids"`
	}

	// Try to parse POST request JSON body
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		// If JSON parse fails, try to get from query parameters (compatible with GET request)
		traderIDsParam := c.Query("trader_ids")
		if traderIDsParam == "" {
			// If no trader_ids specified, return historical data for top 5
			topTraders, err := s.traderManager.GetTopTradersData()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("Failed to get top 5 traders: %v", err),
				})
				return
			}

			traders, ok := topTraders["traders"].([]map[string]interface{})
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Trader data format error"})
				return
			}

			// Extract trader IDs
			traderIDs := make([]string, 0, len(traders))
			for _, trader := range traders {
				if traderID, ok := trader["trader_id"].(string); ok {
					traderIDs = append(traderIDs, traderID)
				}
			}

			result := s.getEquityHistoryForTraders(traderIDs)
			c.JSON(http.StatusOK, result)
			return
		}

		// Parse comma-separated trader IDs
		requestBody.TraderIDs = strings.Split(traderIDsParam, ",")
		for i := range requestBody.TraderIDs {
			requestBody.TraderIDs[i] = strings.TrimSpace(requestBody.TraderIDs[i])
		}
	}

	// Limit to maximum 20 traders to prevent oversized requests
	if len(requestBody.TraderIDs) > 20 {
		requestBody.TraderIDs = requestBody.TraderIDs[:20]
	}

	result := s.getEquityHistoryForTraders(requestBody.TraderIDs)
	c.JSON(http.StatusOK, result)
}

// getEquityHistoryForTraders Get historical data for multiple traders
// Query directly from database, not dependent on trader in memory (so historical data can be retrieved after restart)
func (s *Server) getEquityHistoryForTraders(traderIDs []string) map[string]interface{} {
	result := make(map[string]interface{})
	histories := make(map[string]interface{})
	errors := make(map[string]string)

	for _, traderID := range traderIDs {
		if traderID == "" {
			continue
		}

		// Get equity historical data from new equity table
		snapshots, err := s.store.Equity().GetLatest(traderID, 500)
		if err != nil {
			errors[traderID] = fmt.Sprintf("Failed to get historical data: %v", err)
			continue
		}

		if len(snapshots) == 0 {
			// No historical records, return empty array
			histories[traderID] = []map[string]interface{}{}
			continue
		}

		// Build return rate historical data
		history := make([]map[string]interface{}, 0, len(snapshots))
		for _, snap := range snapshots {
			history = append(history, map[string]interface{}{
				"timestamp":    snap.Timestamp,
				"total_equity": snap.TotalEquity,
				"total_pnl":    snap.UnrealizedPnL,
				"balance":      snap.Balance,
			})
		}

		histories[traderID] = history
	}

	result["histories"] = histories
	result["count"] = len(histories)
	if len(errors) > 0 {
		result["errors"] = errors
	}

	return result
}

// handleGetPublicTraderConfig Get public trader configuration information (no authentication required, does not include sensitive information)
func (s *Server) handleGetPublicTraderConfig(c *gin.Context) {
	traderID := c.Param("id")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trader ID cannot be empty"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	// Get trader status information
	status := trader.GetStatus()

	// Only return public configuration information, not including sensitive data like API keys
	result := map[string]interface{}{
		"trader_id":   trader.GetID(),
		"trader_name": trader.GetName(),
		"ai_model":    trader.GetAIModel(),
		"exchange":    trader.GetExchange(),
		"is_running":  status["is_running"],
		"ai_provider": status["ai_provider"],
		"start_time":  status["start_time"],
	}

	c.JSON(http.StatusOK, result)
}
