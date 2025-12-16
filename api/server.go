package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"nofx/api/credits"
	"nofx/api/handlers"
	"nofx/auth"
	"nofx/config"
	"nofx/decision"
	"nofx/email"
	"nofx/manager"
	"nofx/middleware"
	creditsService "nofx/service/credits"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Server HTTP API服务器
type Server struct {
	router          *gin.Engine
	traderManager   *manager.TraderManager
	database        *config.Database
	emailClient     *email.ResendClient
	creditService   creditsService.Service
	creditHandler   *credits.Handler
	learningHandler *handlers.LearningHandler
	port            int
}

// NewServer 创建API服务器
func NewServer(traderManager *manager.TraderManager, database *config.Database, port int) *Server {
	// 设置为Release模式（减少日志输出）
	gin.SetMode(gin.ReleaseMode)

	// 使用gin.New()而不是gin.Default()，以便我们可以自定义中间件顺序
	router := gin.New()

	// 添加Logger中间件
	router.Use(gin.Logger())

	// 启用CORS（必须在Recovery之前，确保即使panic也能设置CORS头）
	router.Use(corsMiddleware())

	// 添加安全头中间件
	router.Use(middleware.SecurityHeadersMiddleware())

	// 添加频率限制中间件（基础限制）
	router.Use(middleware.RateLimitByIP(60, time.Minute))

	// 添加自定义Recovery中间件，确保panic时也返回带CORS头的响应
	router.Use(corsRecoveryMiddleware())

	// 创建积分服务
	creditService := creditsService.NewCreditService(database)
	creditHandler := credits.NewHandler(creditService)
	learningHandler := handlers.NewLearningHandler(database)

	s := &Server{
		router:          router,
		traderManager:   traderManager,
		database:        database,
		emailClient:     email.NewResendClient(),
		creditService:   creditService,
		creditHandler:   creditHandler,
		learningHandler: learningHandler,
		port:            port,
	}
	// 设置路由
	s.setupRoutes()

	return s
}

// corsRecoveryMiddleware 自定义Recovery中间件，确保panic时也返回带CORS头的响应
func corsRecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 记录panic日志
				log.Printf("❌ Panic recovered: %v", err)

				// 确保CORS头已设置（如果还没设置的话）
				if c.Writer.Header().Get("Access-Control-Allow-Origin") == "" {
					origin := c.Request.Header.Get("Origin")
					if origin != "" {
						c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
						c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
					}
				}

				// 返回500错误
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}

// corsMiddleware CORS中间件
func corsMiddleware() gin.HandlerFunc {
	// 从环境变量获取允许的域名列表，默认为开发环境和Vercel域名
	allowedOrigins := []string{
		// 开发环境
		"http://localhost:3000",
		"http://localhost:5173",
		"http://127.0.0.1:3000",
		"http://127.0.0.1:5173",

		// Vercel部署域名 - 主要实例
		"https://web-3c7a7psvt-gyc567s-projects.vercel.app",
		"https://web-pink-omega-40.vercel.app",
		"https://web-gyc567s-projects.vercel.app",
		"https://web-7jc87z3u4-gyc567s-projects.vercel.app",
		"https://web-gyc567-gyc567s-projects.vercel.app",
		// 新部署实例 - 2025-11-26
		"https://agentrade-nd2sevhec-gyc567s-projects.vercel.app",

		// Vercel部署域名 - 历史实例
		"https://web-fej4rs4y2-gyc567s-projects.vercel.app",
		"https://web-fco5upt1e-gyc567s-projects.vercel.app",
		"https://web-2ybunmaej-gyc567s-projects.vercel.app",
		"https://web-ge79k4nzy-gyc567s-projects.vercel.app",
		// 生产前端域名（含www和不含www）
		"https://www.agentrade.xyz",
		"https://agentrade.xyz",

		// Replit部署域名
		"https://nofx-gyc567.replit.app",
	}

	// 如果设置了环境变量，使用环境变量中的值
	if envOrigins := os.Getenv("ALLOWED_ORIGINS"); envOrigins != "" {
		allowedOrigins = strings.Split(envOrigins, ",")
		for i := range allowedOrigins {
			allowedOrigins[i] = strings.TrimSpace(allowedOrigins[i])
		}
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// 检查origin是否在白名单中
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}

		// 对白名单域名设置CORS头（必须在任何响应之前设置）
		if allowed {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// 始终设置这些头，确保预检请求和错误响应都包含CORS头
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		c.Writer.Header().Set("Access-Control-Allow-Headers",
			"Content-Type, Authorization, Cache-Control, X-Requested-With, X-Requested-By, If-Modified-Since, Pragma, Origin, Accept")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		// 处理预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	// Root health check for Replit deployment
	s.router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "Monnaire Trading Agent OS",
		})
	})

	// Serve static files from web/dist for production
	s.router.Static("/assets", "./web/dist/assets")
	s.router.StaticFile("/index.html", "./web/dist/index.html")

	// Catch-all route for SPA routing - serve index.html for non-API routes
	s.router.NoRoute(func(c *gin.Context) {
		// If the request is for an API route that doesn't exist, return 404 JSON
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "API endpoint not found"})
			return
		}
		// Otherwise serve the frontend index.html for SPA routing
		c.File("./web/dist/index.html")
	})

	// API路由组
	api := s.router.Group("/api")
	{
		// 健康检查
		api.Any("/health", s.handleHealth)
		api.GET("/health/email", s.handleEmailHealthCheck)

		// 认证相关路由（无需认证）
		api.POST("/register", s.handleRegister)
		api.POST("/login", s.handleLogin)

		// 密码重置路由（无需认证）
		api.POST("/request-password-reset", s.handleRequestPasswordReset)
		api.POST("/reset-password", s.handleResetPassword)

		// 系统支持的模型和交易所（无需认证）
		api.GET("/supported-models", s.handleGetSupportedModels)
		api.GET("/supported-exchanges", s.handleGetSupportedExchanges)

		// 系统配置（无需认证）
		api.GET("/config", s.handleGetSystemConfig)

		// 系统提示词模板管理（无需认证）
		api.GET("/prompt-templates", s.handleGetPromptTemplates)
		api.GET("/prompt-templates/:name", s.handleGetPromptTemplate)

		// 积分系统 - 公开接口（无需认证，但有频率限制）
		creditPublic := api.Group("/")
		creditPublic.Use(middleware.RateLimitByIP(60, time.Minute)) // 每分钟最多60次查询
		{
			creditPublic.GET("/credit-packages", s.creditHandler.HandleGetCreditPackages)
			creditPublic.GET("/credit-packages/:id", s.creditHandler.HandleGetCreditPackage)
		}

		// 公开的竞赛数据（无需认证）
		api.GET("/traders", s.handlePublicTraderList)
		api.GET("/competition", s.handlePublicCompetition)
		api.GET("/top-traders", s.handleTopTraders)
		api.GET("/equity-history", s.handleEquityHistory)
		api.POST("/equity-history-batch", s.handleEquityHistoryBatch)
		api.GET("/traders/:id/public-config", s.handleGetPublicTraderConfig)

		// 需要认证的路由
		protected := api.Group("/", s.authMiddleware())
		{
			// AI交易员管理
			protected.GET("/my-traders", s.handleTraderList)
			protected.GET("/traders/:id/config", s.handleGetTraderConfig)
			protected.POST("/traders", s.handleCreateTrader)
			protected.PUT("/traders/:id", s.handleUpdateTrader)
			protected.DELETE("/traders/:id", s.handleDeleteTrader)
			protected.POST("/traders/:id/start", s.handleStartTrader)
			protected.POST("/traders/:id/stop", s.handleStopTrader)
			protected.PUT("/traders/:id/prompt", s.handleUpdateTraderPrompt)

			// AI学习与反思 (Phase 1)
			protected.GET("/traders/:id/analysis", s.learningHandler.HandleGetAnalysis)
			protected.GET("/traders/:id/reflections", s.learningHandler.HandleGetReflections)

			// AI模型配置
			protected.GET("/models", s.handleGetModelConfigs)
			protected.PUT("/models", s.handleUpdateModelConfigs)

			// 交易所配置
			protected.GET("/exchanges", s.handleGetExchangeConfigs)
			protected.PUT("/exchanges", s.handleUpdateExchangeConfigs)

			// 用户信号源配置
			protected.GET("/user/signal-sources", s.handleGetUserSignalSource)
			protected.POST("/user/signal-sources", s.handleSaveUserSignalSource)

			// 指定trader的数据（使用query参数 ?trader_id=xxx）
			protected.GET("/status", s.handleStatus)
			protected.GET("/account", s.handleAccount)
			protected.GET("/positions", s.handlePositions)
			protected.GET("/decisions", s.handleDecisions)
			protected.GET("/decisions/latest", s.handleLatestDecisions)
			protected.GET("/statistics", s.handleStatistics)
			protected.GET("/performance", s.handlePerformance)

			// 用户管理
			protected.GET("/users", s.handleGetUsers)
			protected.GET("/user/me", s.handleGetMe)

			// 积分系统 - 用户接口（需要认证，有用户级别的频率限制）
			creditUser := protected.Group("/user/")
			creditUser.Use(middleware.RateLimitByUser(10, time.Minute)) // 每分钟最多10次积分操作
			{
				creditUser.GET("/credits", s.creditHandler.HandleGetUserCredits)
				creditUser.GET("/credits/transactions", s.creditHandler.HandleGetUserTransactions)
				creditUser.GET("/credits/summary", s.creditHandler.HandleGetUserCreditSummary)
			}
		}

		// 管理员接口（需要认证和管理员权限）
		admin := api.Group("/admin/")
		admin.Use(s.authMiddleware())
		admin.Use(s.adminMiddleware())
		{
			// 积分套餐管理（管理员级别频率限制）
			creditAdmin := admin.Group("/")
			creditAdmin.Use(middleware.RateLimitAdmin(30, time.Minute)) // 管理员每分钟最多30次操作
			{
				creditAdmin.POST("/credit-packages", s.creditHandler.HandleCreateCreditPackage)
				creditAdmin.PUT("/credit-packages/:id", s.creditHandler.HandleUpdateCreditPackage)
				creditAdmin.DELETE("/credit-packages/:id", s.creditHandler.HandleDeleteCreditPackage)

				// 用户积分管理
				creditAdmin.POST("/users/:id/credits/adjust", s.creditHandler.HandleAdjustUserCredits)
				creditAdmin.GET("/users/:id/credits", s.creditHandler.HandleGetUserCreditsByAdmin)
				creditAdmin.GET("/users/:id/credits/transactions", s.creditHandler.HandleGetUserTransactionsByAdmin)
			}
		}
	}
}

// handleHealth 健康检查
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   c.Request.Context().Value("time"),
	})
}

// handleEmailHealthCheck 邮件服务健康检查
func (s *Server) handleEmailHealthCheck(c *gin.Context) {
	// 检查API Key配置
	hasAPIKey := s.emailClient.HasAPIKey()
	fromEmail := s.emailClient.GetFromEmail()

	if !hasAPIKey {
		log.Printf("🔴 [EMAIL_HEALTH_CHECK] API Key未配置")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":     "unhealthy",
			"service":    "email",
			"provider":   "resend",
			"reason":     "RESEND_API_KEY未配置",
			"timestamp":  time.Now(),
			"from_email": fromEmail,
		})
		return
	}

	if fromEmail == "" {
		log.Printf("🔴 [EMAIL_HEALTH_CHECK] 发件人邮箱未配置")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":    "unhealthy",
			"service":   "email",
			"provider":  "resend",
			"reason":    "发件人邮箱未配置",
			"timestamp": time.Now(),
		})
		return
	}

	log.Printf("✅ [EMAIL_HEALTH_CHECK] 邮件服务正常")
	c.JSON(http.StatusOK, gin.H{
		"status":     "healthy",
		"service":    "email",
		"provider":   "resend",
		"from_email": fromEmail,
		"timestamp":  time.Now(),
	})
}

// handleGetMe 获取当前登录用户信息（用于刷新前端状态）
func (s *Server) handleGetMe(c *gin.Context) {
	userID := c.GetString("user_id")
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          user.ID,
		"email":       user.Email,
		"invite_code": user.InviteCode,
		"is_admin":    user.IsAdmin,
		"created_at":  user.CreatedAt,
	})
}

// handleGetSystemConfig 获取系统配置（客户端需要知道的配置）
func (s *Server) handleGetSystemConfig(c *gin.Context) {
	// 获取默认币种
	defaultCoinsStr, _ := s.database.GetSystemConfig("default_coins")
	var defaultCoins []string
	if defaultCoinsStr != "" {
		json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins)
	}
	if len(defaultCoins) == 0 {
		// 使用硬编码的默认币种
		defaultCoins = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"}
	}

	// 获取杠杆配置
	btcEthLeverageStr, _ := s.database.GetSystemConfig("btc_eth_leverage")
	altcoinLeverageStr, _ := s.database.GetSystemConfig("altcoin_leverage")

	btcEthLeverage := 5
	if val, err := strconv.Atoi(btcEthLeverageStr); err == nil && val > 0 {
		btcEthLeverage = val
	}

	altcoinLeverage := 5
	if val, err := strconv.Atoi(altcoinLeverageStr); err == nil && val > 0 {
		altcoinLeverage = val
	}

	// 获取内测模式配置
	betaModeStr, _ := s.database.GetSystemConfig("beta_mode")
	betaMode := betaModeStr == "true"

	c.JSON(http.StatusOK, gin.H{
		"beta_mode":        betaMode,
		"default_coins":    defaultCoins,
		"btc_eth_leverage": btcEthLeverage,
		"altcoin_leverage": altcoinLeverage,
	})
}

// getTraderFromQuery 从query参数获取trader
func (s *Server) getTraderFromQuery(c *gin.Context) (*manager.TraderManager, string, error) {
	userID := c.GetString("user_id")
	traderID := c.Query("trader_id")

	// 确保用户的交易员已加载到内存中
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, err)
	}

	if traderID == "" {
		// 如果没有指定trader_id，返回该用户的第一个trader
		ids := s.traderManager.GetTraderIDs()
		if len(ids) == 0 {
			return nil, "", fmt.Errorf("没有可用的trader")
		}

		// 获取用户的交易员列表，优先返回用户自己的交易员
		userTraders, err := s.database.GetTraders(userID)
		if err == nil && len(userTraders) > 0 {
			traderID = userTraders[0].ID
		} else {
			traderID = ids[0]
		}
	}

	return s.traderManager, traderID, nil
}

// AI交易员管理相关结构体
type CreateTraderRequest struct {
	Name                 string  `json:"name" binding:"required"`
	AIModelID            string  `json:"ai_model_id" binding:"required"`
	ExchangeID           string  `json:"exchange_id" binding:"required"`
	InitialBalance       float64 `json:"initial_balance"`
	ScanIntervalMinutes  int     `json:"scan_interval_minutes"`
	BTCETHLeverage       int     `json:"btc_eth_leverage"`
	AltcoinLeverage      int     `json:"altcoin_leverage"`
	TradingSymbols       string  `json:"trading_symbols"`
	CustomPrompt         string  `json:"custom_prompt"`
	OverrideBasePrompt   bool    `json:"override_base_prompt"`
	SystemPromptTemplate string  `json:"system_prompt_template"` // 系统提示词模板名称
	IsCrossMargin        *bool   `json:"is_cross_margin"`        // 指针类型，nil表示使用默认值true
	UseCoinPool          bool    `json:"use_coin_pool"`
	UseOITop             bool    `json:"use_oi_top"`
}

type ModelConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Enabled      bool   `json:"enabled"`
	APIKey       string `json:"apiKey,omitempty"`
	CustomAPIURL string `json:"customApiUrl,omitempty"`
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
		Enabled               bool   `json:"enabled"`
		APIKey                string `json:"api_key"`
		SecretKey             string `json:"secret_key"`
		Testnet               bool   `json:"testnet"`
		HyperliquidWalletAddr string `json:"hyperliquid_wallet_addr"`
		AsterUser             string `json:"aster_user"`
		AsterSigner           string `json:"aster_signer"`
		AsterPrivateKey       string `json:"aster_private_key"`
		OKXPassphrase         string `json:"okx_passphrase"`
	} `json:"exchanges"`
}

// handleCreateTrader 创建新的AI交易员
func (s *Server) handleCreateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	var req CreateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 校验杠杆值
	if req.BTCETHLeverage < 0 || req.BTCETHLeverage > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "BTC/ETH杠杆必须在1-50倍之间"})
		return
	}
	if req.AltcoinLeverage < 0 || req.AltcoinLeverage > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "山寨币杠杆必须在1-20倍之间"})
		return
	}

	// 校验交易币种格式
	if req.TradingSymbols != "" {
		symbols := strings.Split(req.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" && !strings.HasSuffix(strings.ToUpper(symbol), "USDT") {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的币种格式: %s，必须以USDT结尾", symbol)})
				return
			}
		}
	}

	// 生成交易员ID
	traderID := fmt.Sprintf("%s_%s_%d", req.ExchangeID, req.AIModelID, time.Now().Unix())

	// 设置默认值
	isCrossMargin := true // 默认为全仓模式
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	// 设置杠杆默认值（从系统配置获取）
	btcEthLeverage := 5
	altcoinLeverage := 5
	if req.BTCETHLeverage > 0 {
		btcEthLeverage = req.BTCETHLeverage
	} else {
		// 从系统配置获取默认值
		if btcEthLeverageStr, _ := s.database.GetSystemConfig("btc_eth_leverage"); btcEthLeverageStr != "" {
			if val, err := strconv.Atoi(btcEthLeverageStr); err == nil && val > 0 {
				btcEthLeverage = val
			}
		}
	}
	if req.AltcoinLeverage > 0 {
		altcoinLeverage = req.AltcoinLeverage
	} else {
		// 从系统配置获取默认值
		if altcoinLeverageStr, _ := s.database.GetSystemConfig("altcoin_leverage"); altcoinLeverageStr != "" {
			if val, err := strconv.Atoi(altcoinLeverageStr); err == nil && val > 0 {
				altcoinLeverage = val
			}
		}
	}

	// 设置系统提示词模板默认值
	systemPromptTemplate := "default"
	if req.SystemPromptTemplate != "" {
		systemPromptTemplate = req.SystemPromptTemplate
	}

	// 设置扫描间隔默认值
	scanIntervalMinutes := req.ScanIntervalMinutes
	if scanIntervalMinutes <= 0 {
		scanIntervalMinutes = 3 // 默认3分钟
	}

	// 创建交易员配置（数据库实体）
	trader := &config.TraderRecord{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		InitialBalance:       req.InitialBalance,
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

	// 保存到数据库
	err := s.database.CreateTrader(trader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建交易员失败: %v", err)})
		return
	}

	// 立即将新交易员加载到TraderManager中
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户交易员到内存失败: %v", err)
		// 这里不返回错误，因为交易员已经成功创建到数据库
	}

	log.Printf("✓ 创建交易员成功: %s (模型: %s, 交易所: %s)", req.Name, req.AIModelID, req.ExchangeID)

	c.JSON(http.StatusCreated, gin.H{
		"trader_id":   traderID,
		"trader_name": req.Name,
		"ai_model":    req.AIModelID,
		"is_running":  false,
	})
}

// UpdateTraderRequest 更新交易员请求
type UpdateTraderRequest struct {
	Name                string  `json:"name" binding:"required"`
	AIModelID           string  `json:"ai_model_id" binding:"required"`
	ExchangeID          string  `json:"exchange_id" binding:"required"`
	InitialBalance      float64 `json:"initial_balance"`
	ScanIntervalMinutes int     `json:"scan_interval_minutes"`
	BTCETHLeverage      int     `json:"btc_eth_leverage"`
	AltcoinLeverage     int     `json:"altcoin_leverage"`
	TradingSymbols      string  `json:"trading_symbols"`
	CustomPrompt        string  `json:"custom_prompt"`
	OverrideBasePrompt  bool    `json:"override_base_prompt"`
	IsCrossMargin       *bool   `json:"is_cross_margin"`
}

// handleUpdateTrader 更新交易员配置
func (s *Server) handleUpdateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	var req UpdateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查交易员是否存在且属于当前用户
	traders, err := s.database.GetTraders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取交易员列表失败"})
		return
	}

	var existingTrader *config.TraderRecord
	for _, trader := range traders {
		if trader.ID == traderID {
			existingTrader = trader
			break
		}
	}

	if existingTrader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 设置默认值
	isCrossMargin := existingTrader.IsCrossMargin // 保持原值
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	// 设置杠杆默认值
	btcEthLeverage := req.BTCETHLeverage
	altcoinLeverage := req.AltcoinLeverage
	if btcEthLeverage <= 0 {
		btcEthLeverage = existingTrader.BTCETHLeverage // 保持原值
	}
	if altcoinLeverage <= 0 {
		altcoinLeverage = existingTrader.AltcoinLeverage // 保持原值
	}

	// 设置扫描间隔，允许更新
	scanIntervalMinutes := req.ScanIntervalMinutes
	if scanIntervalMinutes <= 0 {
		scanIntervalMinutes = existingTrader.ScanIntervalMinutes // 保持原值
	}

	// 更新交易员配置
	trader := &config.TraderRecord{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		InitialBalance:       req.InitialBalance,
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       req.TradingSymbols,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: existingTrader.SystemPromptTemplate, // 保持原值
		IsCrossMargin:        isCrossMargin,
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            existingTrader.IsRunning, // 保持原值
	}

	// 更新数据库
	err = s.database.UpdateTrader(trader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新交易员失败: %v", err)})
		return
	}

	// 重新加载交易员到内存
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 重新加载用户交易员到内存失败: %v", err)
	}

	log.Printf("✓ 更新交易员成功: %s (模型: %s, 交易所: %s)", req.Name, req.AIModelID, req.ExchangeID)

	c.JSON(http.StatusOK, gin.H{
		"trader_id":   traderID,
		"trader_name": req.Name,
		"ai_model":    req.AIModelID,
		"message":     "交易员更新成功",
	})
}

// handleDeleteTrader 删除交易员
func (s *Server) handleDeleteTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 从数据库删除
	err := s.database.DeleteTrader(userID, traderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("删除交易员失败: %v", err)})
		return
	}

	// 如果交易员正在运行，先停止它
	if trader, err := s.traderManager.GetTrader(traderID); err == nil {
		status := trader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); ok && isRunning {
			trader.Stop()
			log.Printf("⏹  已停止运行中的交易员: %s", traderID)
		}
	}

	log.Printf("✓ 交易员已删除: %s", traderID)
	c.JSON(http.StatusOK, gin.H{"message": "交易员已删除"})
}

// handleStartTrader 启动交易员
func (s *Server) handleStartTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 校验交易员是否属于当前用户 - 使用统一的查询逻辑
	traders, err := s.database.GetTraders(userID)
	if err != nil {
		log.Printf("❌ 获取用户 %s 的交易员列表失败: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取交易员列表失败"})
		return
	}

	// 检查指定trader是否存在于用户列表中
	var userTrader *config.TraderRecord
	for _, trader := range traders {
		if trader.ID == traderID {
			userTrader = trader
			break
		}
	}

	if userTrader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在或无访问权限"})
		return
	}

	// 尝试从traderManager获取trader实例
	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		// 如果trader不在内存中，尝试从数据库加载该用户的trader
		log.Printf("🔄 Trader %s 不在内存中，尝试加载...", traderID)
		if loadErr := s.traderManager.LoadUserTraders(s.database, userID); loadErr != nil {
			log.Printf("❌ 加载trader失败: %v", loadErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "加载trader失败"})
			return
		}

		// 再次尝试获取
		trader, err = s.traderManager.GetTrader(traderID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
			return
		}
		log.Printf("✅ Trader %s 已加载到内存", traderID)
	}

	// 检查交易员是否已经在运行
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员已在运行中"})
		return
	}

	// 启动交易员
	go func() {
		log.Printf("▶️  启动交易员 %s (%s)", traderID, trader.GetName())
		if err := trader.Run(); err != nil {
			log.Printf("❌ 交易员 %s 运行错误: %v", trader.GetName(), err)
		}
	}()

	// 更新数据库中的运行状态
	err = s.database.UpdateTraderStatus(traderID, true)
	if err != nil {
		log.Printf("⚠️  更新交易员状态失败: %v", err)
	}

	log.Printf("✓ 交易员 %s 已启动", trader.GetName())
	c.JSON(http.StatusOK, gin.H{"message": "交易员已启动"})
}

// handleStopTrader 停止交易员
func (s *Server) handleStopTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 校验交易员是否属于当前用户 - 使用统一的查询逻辑
	traders, err := s.database.GetTraders(userID)
	if err != nil {
		log.Printf("❌ 获取用户 %s 的交易员列表失败: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取交易员列表失败"})
		return
	}

	// 检查指定trader是否存在于用户列表中
	var userTrader *config.TraderRecord
	for _, trader := range traders {
		if trader.ID == traderID {
			userTrader = trader
			break
		}
	}

	if userTrader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在或无访问权限"})
		return
	}

	// 尝试从traderManager获取trader实例
	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		// 如果trader不在内存中，尝试从数据库加载该用户的trader
		log.Printf("🔄 Trader %s 不在内存中，尝试加载...", traderID)
		if loadErr := s.traderManager.LoadUserTraders(s.database, userID); loadErr != nil {
			log.Printf("❌ 加载trader失败: %v", loadErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "加载trader失败"})
			return
		}

		// 再次尝试获取
		trader, err = s.traderManager.GetTrader(traderID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
			return
		}
		log.Printf("✅ Trader %s 已加载到内存", traderID)
	}

	// 检查交易员是否正在运行
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && !isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员已停止"})
		return
	}

	// 停止交易员
	trader.Stop()

	// 更新数据库中的运行状态
	err = s.database.UpdateTraderStatus(traderID, false)
	if err != nil {
		log.Printf("⚠️  更新交易员状态失败: %v", err)
	}

	log.Printf("⏹  交易员 %s 已停止", trader.GetName())
	c.JSON(http.StatusOK, gin.H{"message": "交易员已停止"})
}

// handleUpdateTraderPrompt 更新交易员自定义Prompt
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

	// 更新数据库
	err := s.database.UpdateTraderCustomPrompt(userID, traderID, req.CustomPrompt, req.OverrideBasePrompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新自定义prompt失败: %v", err)})
		return
	}

	// 如果trader在内存中，更新其custom prompt和override设置
	trader, err := s.traderManager.GetTrader(traderID)
	if err == nil {
		trader.SetCustomPrompt(req.CustomPrompt)
		trader.SetOverrideBasePrompt(req.OverrideBasePrompt)
		log.Printf("✓ 已更新交易员 %s 的自定义prompt (覆盖基础=%v)", trader.GetName(), req.OverrideBasePrompt)
	}

	c.JSON(http.StatusOK, gin.H{"message": "自定义prompt已更新"})
}

// handleGetModelConfigs 获取AI模型配置
func (s *Server) handleGetModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	log.Printf("🔍 查询用户 %s 的AI模型配置", userID)
	models, err := s.database.GetAIModels(userID)
	if err != nil {
		log.Printf("❌ 获取AI模型配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取AI模型配置失败: %v", err)})
		return
	}
	log.Printf("✅ 找到 %d 个AI模型配置", len(models))

	c.JSON(http.StatusOK, models)
}

// handleUpdateModelConfigs 更新AI模型配置
func (s *Server) handleUpdateModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	var req UpdateModelConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新每个模型的配置
	for modelID, modelData := range req.Models {
		err := s.database.UpdateAIModel(userID, modelID, modelData.Enabled, modelData.APIKey, modelData.CustomAPIURL, modelData.CustomModelName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新模型 %s 失败: %v", modelID, err)})
			return
		}
	}

	// 重新加载该用户的所有交易员，使新配置立即生效
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 重新加载用户交易员到内存失败: %v", err)
		// 这里不返回错误，因为模型配置已经成功更新到数据库
	}

	log.Printf("✓ AI模型配置已更新: %+v", req.Models)
	c.JSON(http.StatusOK, gin.H{"message": "模型配置已更新"})
}

// handleGetExchangeConfigs 获取交易所配置
func (s *Server) handleGetExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	log.Printf("🔍 查询用户 %s 的交易所配置", userID)
	exchanges, err := s.database.GetExchanges(userID)
	if err != nil {
		log.Printf("❌ 获取交易所配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取交易所配置失败: %v", err)})
		return
	}
	log.Printf("✅ 找到 %d 个交易所配置", len(exchanges))

	c.JSON(http.StatusOK, exchanges)
}

// handleUpdateExchangeConfigs 更新交易所配置
func (s *Server) handleUpdateExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	var req UpdateExchangeConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新每个交易所的配置
	for exchangeID, exchangeData := range req.Exchanges {
		err := s.database.UpdateExchange(userID, exchangeID, exchangeData.Enabled, exchangeData.APIKey, exchangeData.SecretKey, exchangeData.Testnet, exchangeData.HyperliquidWalletAddr, exchangeData.AsterUser, exchangeData.AsterSigner, exchangeData.AsterPrivateKey, exchangeData.OKXPassphrase)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新交易所 %s 失败: %v", exchangeID, err)})
			return
		}
	}

	// 重新加载该用户的所有交易员，使新配置立即生效
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 重新加载用户交易员到内存失败: %v", err)
		// 这里不返回错误，因为交易所配置已经成功更新到数据库
	}

	log.Printf("✓ 交易所配置已更新: %+v", req.Exchanges)
	c.JSON(http.StatusOK, gin.H{"message": "交易所配置已更新"})
}

// handleGetUserSignalSource 获取用户信号源配置
func (s *Server) handleGetUserSignalSource(c *gin.Context) {
	userID := c.GetString("user_id")
	source, err := s.database.GetUserSignalSource(userID)
	if err != nil {
		// 如果配置不存在，返回空配置而不是404错误
		c.JSON(http.StatusOK, gin.H{
			"coin_pool_url": "",
			"oi_top_url":    "",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"coin_pool_url": source.CoinPoolURL,
		"oi_top_url":    source.OITopURL,
	})
}

// handleSaveUserSignalSource 保存用户信号源配置
func (s *Server) handleSaveUserSignalSource(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		CoinPoolURL string `json:"coin_pool_url"`
		OITopURL    string `json:"oi_top_url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := s.database.CreateUserSignalSource(userID, req.CoinPoolURL, req.OITopURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存用户信号源配置失败: %v", err)})
		return
	}

	log.Printf("✓ 用户信号源配置已保存: user=%s, coin_pool=%s, oi_top=%s", userID, req.CoinPoolURL, req.OITopURL)
	c.JSON(http.StatusOK, gin.H{"message": "用户信号源配置已保存"})
}

// handleTraderList trader列表
func (s *Server) handleTraderList(c *gin.Context) {
	userID := c.GetString("user_id")
	traders, err := s.database.GetTraders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取交易员列表失败: %v", err)})
		return
	}

	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		// 获取实时运行状态
		isRunning := trader.IsRunning
		if at, err := s.traderManager.GetTrader(trader.ID); err == nil {
			status := at.GetStatus()
			if running, ok := status["is_running"].(bool); ok {
				isRunning = running
			}
		}

		// AIModelID 应该已经是 provider（如 "deepseek"），直接使用
		// 如果是旧数据格式（如 "admin_deepseek"），提取 provider 部分
		aiModelID := trader.AIModelID
		// 兼容旧数据：如果包含下划线，提取最后一部分作为 provider
		if strings.Contains(aiModelID, "_") {
			parts := strings.Split(aiModelID, "_")
			aiModelID = parts[len(parts)-1]
		}

		result = append(result, map[string]interface{}{
			"trader_id":       trader.ID,
			"trader_name":     trader.Name,
			"ai_model":        aiModelID,
			"exchange_id":     trader.ExchangeID,
			"is_running":      isRunning,
			"initial_balance": trader.InitialBalance,
		})
	}

	c.JSON(http.StatusOK, result)
}

// handleGetTraderConfig 获取交易员详细配置
func (s *Server) handleGetTraderConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员ID不能为空"})
		return
	}

	traderConfig, _, _, err := s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("获取交易员配置失败: %v", err)})
		return
	}

	// 获取实时运行状态
	isRunning := traderConfig.IsRunning
	if at, err := s.traderManager.GetTrader(traderID); err == nil {
		status := at.GetStatus()
		if running, ok := status["is_running"].(bool); ok {
			isRunning = running
		}
	}

	// 返回完整的模型ID，不做转换，保持与前端模型列表一致
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

// handleStatus 系统状态
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

// handleAccount 账户信息
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

	log.Printf("📊 收到账户信息请求 [%s]", trader.GetName())
	account, err := trader.GetAccountInfo()
	if err != nil {
		log.Printf("❌ 获取账户信息失败 [%s]: %v", trader.GetName(), err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取账户信息失败: %v", err),
		})
		return
	}

	log.Printf("✓ 返回账户信息 [%s]: 净值=%.2f, 可用=%.2f, 盈亏=%.2f (%.2f%%)",
		trader.GetName(),
		account["total_equity"],
		account["available_balance"],
		account["total_pnl"],
		account["total_pnl_pct"])
	c.JSON(http.StatusOK, account)
}

// handlePositions 持仓列表
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
			"error": fmt.Sprintf("获取持仓列表失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, positions)
}

// handleDecisions 决策日志列表
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

	// 获取所有历史决策记录（无限制）
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取决策日志失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, records)
}

// handleLatestDecisions 最新决策日志（最近5条，最新的在前）
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

	records, err := trader.GetDecisionLogger().GetLatestRecords(5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取决策日志失败: %v", err),
		})
		return
	}

	// 反转数组，让最新的在前面（用于列表显示）
	// GetLatestRecords返回的是从旧到新（用于图表），这里需要从新到旧
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	c.JSON(http.StatusOK, records)
}

// handleStatistics 统计信息
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

	stats, err := trader.GetDecisionLogger().GetStatistics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取统计信息失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// handleCompetition 竞赛总览（对比所有trader）
func (s *Server) handleCompetition(c *gin.Context) {
	userID := c.GetString("user_id")

	// 确保用户的交易员已加载到内存中
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, err)
	}

	competition, err := s.traderManager.GetCompetitionData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取竞赛数据失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, competition)
}

// handleEquityHistory 收益率历史数据
func (s *Server) handleEquityHistory(c *gin.Context) {
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

	// 获取尽可能多的历史数据（几天的数据）
	// 每3分钟一个周期：10000条 = 约20天的数据
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取历史数据失败: %v", err),
		})
		return
	}

	// 构建收益率历史数据点
	type EquityPoint struct {
		Timestamp        string  `json:"timestamp"`
		TotalEquity      float64 `json:"total_equity"`      // 账户净值（wallet + unrealized）
		AvailableBalance float64 `json:"available_balance"` // 可用余额
		TotalPnL         float64 `json:"total_pnl"`         // 总盈亏（相对初始余额）
		TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
		PositionCount    int     `json:"position_count"`    // 持仓数量
		MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
		CycleNumber      int     `json:"cycle_number"`
	}

	// 从AutoTrader获取初始余额（用于计算盈亏百分比）
	initialBalance := 0.0
	if status := trader.GetStatus(); status != nil {
		if ib, ok := status["initial_balance"].(float64); ok && ib > 0 {
			initialBalance = ib
		}
	}

	// 如果无法从status获取，且有历史记录，则从第一条记录获取
	if initialBalance == 0 && len(records) > 0 {
		// 第一条记录的equity作为初始余额
		initialBalance = records[0].AccountState.TotalBalance
	}

	// 如果还是无法获取，返回错误
	if initialBalance == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "无法获取初始余额",
		})
		return
	}

	var history []EquityPoint
	for _, record := range records {
		// TotalBalance字段实际存储的是TotalEquity
		totalEquity := record.AccountState.TotalBalance
		// TotalUnrealizedProfit字段实际存储的是TotalPnL（相对初始余额）
		totalPnL := record.AccountState.TotalUnrealizedProfit

		// 计算盈亏百分比
		totalPnLPct := 0.0
		if initialBalance > 0 {
			totalPnLPct = (totalPnL / initialBalance) * 100
		}

		history = append(history, EquityPoint{
			Timestamp:        record.Timestamp.Format("2006-01-02 15:04:05"),
			TotalEquity:      totalEquity,
			AvailableBalance: record.AccountState.AvailableBalance,
			TotalPnL:         totalPnL,
			TotalPnLPct:      totalPnLPct,
			PositionCount:    record.AccountState.PositionCount,
			MarginUsedPct:    record.AccountState.MarginUsedPct,
			CycleNumber:      record.CycleNumber,
		})
	}

	c.JSON(http.StatusOK, history)
}

// handlePerformance AI历史表现分析（用于展示AI学习和反思）
func (s *Server) handlePerformance(c *gin.Context) {
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

	// 分析最近100个周期的交易表现（避免长期持仓的交易记录丢失）
	// 假设每3分钟一个周期，100个周期 = 5小时，足够覆盖大部分交易
	performance, err := trader.GetDecisionLogger().AnalyzePerformance(100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("分析历史表现失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, performance)
}

// authMiddleware JWT认证中间件
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// OPTIONS预检请求不需要认证，直接放行
		// CORS预检请求由corsMiddleware处理
		if c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// 检查是否开启admin模式
		adminModeStr, _ := s.database.GetSystemConfig("admin_mode")
		isAdminMode := adminModeStr == "true"

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// 如果是admin模式，使用admin用户
			if isAdminMode {
				user, err := s.database.GetUserByID("admin")
				if err != nil {
					log.Printf("获取admin用户失败: %v", err)
					c.JSON(http.StatusUnauthorized, gin.H{"error": "admin用户不存在"})
					c.Abort()
					return
				}
				c.Set("user", user)
				c.Set("user_id", "admin")
				c.Next()
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少Authorization头"})
			c.Abort()
			return
		}

		// 检查Bearer token格式
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的Authorization格式"})
			c.Abort()
			return
		}

		// 验证JWT token
		claims, err := auth.ValidateJWT(tokenParts[1])
		if err != nil {
			// JWT验证失败时，记录详细错误信息
			log.Printf("⚠️ JWT验证失败: %v (token前20字符: %s...)", err, tokenParts[1][:min(20, len(tokenParts[1]))])

			// 即使在admin模式下，如果用户提供了token但验证失败，也应该返回错误
			// 只有在完全没有token时才回退到admin用户（见上面的authHeader==""分支）
			// 这样可以确保用户的请求使用正确的用户身份
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的token: " + err.Error()})
			c.Abort()
			return
		}

		// 获取完整的用户信息
		user, err := s.database.GetUserByID(claims.UserID)
		if err != nil {
			log.Printf("获取用户信息失败: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "无效的用户",
			})
			c.Abort()
			return
		}

		// 将完整的用户对象存储到上下文中
		c.Set("user", user)
		// 为了向后兼容，同时保留user_id
		c.Set("user_id", claims.UserID)
		c.Next()
	}
}

// handleRegister 处理用户注册请求
func (s *Server) handleRegister(c *gin.Context) {
	var req struct {
		Email      string `json:"email" binding:"required,email"`
		Password   string `json:"password" binding:"required,min=8"`
		BetaCode   string `json:"beta_code"`
		InviteCode string `json:"invite_code"`
	}

	// 验证请求数据
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求数据格式错误",
			"details": "请确保邮箱格式正确，密码长度不少于8位",
		})
		return
	}

	// 验证密码强度
	if len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "密码强度不够",
			"details": "密码必须至少包含8个字符",
		})
		return
	}

	// 检查是否开启了内测模式
	betaModeStr, _ := s.database.GetSystemConfig("beta_mode")
	if betaModeStr == "true" {
		// 内测模式下必须提供有效的内测码
		if req.BetaCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "内测码不能为空",
				"details": "当前为内测期间，注册需要提供有效的内测码",
			})
			return
		}

		// 验证内测码
		isValid, err := s.database.ValidateBetaCode(req.BetaCode)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "内测码验证失败",
				"details": "服务器内部错误，请稍后重试",
			})
			return
		}
		if !isValid {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "内测码无效",
				"details": "内测码无效或已被使用，请检查后重试",
			})
			return
		}
	}

	// 验证邀请码 (如果提供)
	var inviter *config.User
	if req.InviteCode != "" {
		var err error
		inviter, err = s.database.GetUserByInviteCode(req.InviteCode)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   "邀请码无效",
					"details": "请检查邀请码是否正确",
				})
				return
			}
			log.Printf("验证邀请码失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "系统错误",
				"details": "验证邀请码时发生错误",
			})
			return
		}
	}

	// 检查邮箱是否已存在
	_, err := s.database.GetUserByEmail(req.Email)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   "邮箱已被注册",
			"details": "该邮箱地址已经注册，请使用其他邮箱或尝试登录",
		})
		return
	}
	if err != sql.ErrNoRows {
		// 数据库查询失败，不是用户不存在的错误
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "系统错误",
			"details": "服务器内部错误，请稍后重试",
		})
		return
	}

	// 生成密码哈希
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "密码处理失败",
			"details": "服务器内部错误，请稍后重试",
		})
		return
	}

	// 创建用户（直接激活，无需OTP验证）
	userID := uuid.New().String()
	now := time.Now()
	user := &config.User{
		ID:             userID,
		Email:          req.Email,
		PasswordHash:   passwordHash,
		OTPSecret:      "",           // 移除OTP密钥
		OTPVerified:    true,         // 直接标记为已验证
		IsActive:       true,         // 账户激活状态
		IsAdmin:        false,        // 非管理员
		BetaCode:       req.BetaCode, // 关联内测码
		FailedAttempts: 0,            // 失败尝试次数
		CreatedAt:      now,          // 创建时间
		UpdatedAt:      now,          // 更新时间
	}

	// 设置邀请关系
	if inviter != nil {
		user.InvitedByUserID = inviter.ID
		user.InvitationLevel = inviter.InvitationLevel + 1
	}

	// 使用支持事务和邀请奖励的创建方法
	err = s.database.CreateUserWithInvitation(user)
	if err != nil {
		log.Printf("创建用户失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "创建用户失败",
			"details": "服务器内部错误，请稍后重试",
		})
		return
	}

	// 如果是内测模式，标记内测码为已使用
	betaModeStr2, _ := s.database.GetSystemConfig("beta_mode")
	if betaModeStr2 == "true" && req.BetaCode != "" {
		err := s.database.UseBetaCode(req.BetaCode, req.Email)
		if err != nil {
			log.Printf("⚠️ 标记内测码为已使用失败: %v", err)
			// 这里不返回错误，因为用户已经创建成功
		} else {
			log.Printf("✓ 内测码 %s 已被用户 %s 使用", req.BetaCode, req.Email)
		}
	}

	// 生成JWT令牌
	token, err := auth.GenerateJWT(userID, req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "令牌生成失败",
			"details": "服务器内部错误，请稍后重试",
		})
		return
	}

	// 返回成功信息
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "注册成功，欢迎加入Monnaire Trading Agent OS！",
		"token":   token,
		"user": gin.H{
			"id":          userID,
			"email":       req.Email,
			"invite_code": user.InviteCode,
		},
	})
}

// handleCompleteRegistration 完成注册（验证OTP）
func (s *Server) handleCompleteRegistration(c *gin.Context) {
	var req struct {
		UserID  string `json:"user_id" binding:"required"`
		OTPCode string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户信息
	user, err := s.database.GetUserByID(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 验证OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OTP验证码错误"})
		return
	}

	// 更新用户OTP验证状态
	err = s.database.UpdateUserOTPVerified(req.UserID, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户状态失败"})
		return
	}

	// 生成JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
		return
	}

	// 初始化用户的默认模型和交易所配置
	err = s.initUserDefaultConfigs(user.ID)
	if err != nil {
		log.Printf("初始化用户默认配置失败: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
		"email":   user.Email,
		"message": "注册完成",
	})
}

// handleLogin 处理用户登录请求
func (s *Server) handleLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户信息
	user, err := s.database.GetUserByEmail(req.Email)
	if err != nil {
		log.Printf("🔴 [LOGIN_FAILED] 用户不存在或查询错误: email=%s, error=%v", req.Email, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}

	log.Printf("✓ [LOGIN_CHECK] 用户存在: email=%s, passwordHashExists=%t", user.Email, user.PasswordHash != "")
	log.Printf("   用户数据: ID=%s, Email=%s, PasswordHashLen=%d", user.ID, user.Email, len(user.PasswordHash))

	// 验证密码
	passwordMatch := auth.CheckPassword(req.Password, user.PasswordHash)
	log.Printf("🔍 [LOGIN_DEBUG] 密码验证详情: email=%s, passwordLen=%d, hashLen=%d, match=%t",
		req.Email, len(req.Password), len(user.PasswordHash), passwordMatch)

	if !passwordMatch {
		log.Printf("🔴 [LOGIN_FAILED] 密码验证失败: email=%s", user.Email)
		log.Printf("   密码长度: %d, 哈希长度: %d", len(req.Password), len(user.PasswordHash))
		log.Printf("   提示: 检查密码是否被正确编码或哈希是否正确存储")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}

	log.Printf("✅ [LOGIN_PASSWORD_OK] 密码验证成功: email=%s", user.Email)

	// 检查是否开启内测模式
	betaModeStr, _ := s.database.GetSystemConfig("beta_mode")
	log.Printf("✓ [LOGIN_BETA_CHECK] 内测模式: %s", betaModeStr)
	if betaModeStr == "true" {
		// 内测模式下，验证用户是否有有效的内测码
		userBetaCode, err := s.database.GetUserBetaCode(user.ID)
		if err != nil {
			log.Printf("🔴 [LOGIN_FAILED] 获取用户内测码失败: email=%s, error=%v", user.Email, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "验证失败，请稍后重试"})
			return
		}

		if userBetaCode == "" {
			log.Printf("🔴 [LOGIN_FAILED] 用户无内测码: email=%s", user.Email)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "内测码无效，请联系管理员"})
			return
		}

		// 验证内测码是否仍然有效
		isValid, err := s.database.ValidateBetaCode(userBetaCode)
		if err != nil {
			log.Printf("⚠️ 验证内测码失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "验证失败，请稍后重试"})
			return
		}

		if !isValid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "内测码无效，请联系管理员"})
			return
		}

		log.Printf("✓ 用户 %s 登录成功（内测码: %s）", user.Email, userBetaCode)
	}

	// 生成JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
		return
	}

	// 返回成功信息
	c.JSON(http.StatusOK, gin.H{
		"token":       token,
		"user_id":     user.ID,
		"email":       user.Email,
		"invite_code": user.InviteCode,
		"message":     "登录成功",
	})
}

// handleVerifyOTP 验证OTP并完成登录
func (s *Server) handleVerifyOTP(c *gin.Context) {
	var req struct {
		UserID  string `json:"user_id" binding:"required"`
		OTPCode string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户信息
	user, err := s.database.GetUserByID(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 验证OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误"})
		return
	}

	// 生成JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
		"email":   user.Email,
		"message": "登录成功",
	})
}

// initUserDefaultConfigs 为新用户初始化默认的模型和交易所配置
func (s *Server) initUserDefaultConfigs(userID string) error {
	// 注释掉自动创建默认配置，让用户手动添加
	// 这样新用户注册后不会自动有配置项
	log.Printf("用户 %s 注册完成，等待手动配置AI模型和交易所", userID)
	return nil
}

// handleGetSupportedModels 获取系统支持的AI模型列表
func (s *Server) handleGetSupportedModels(c *gin.Context) {
	// 返回系统支持的AI模型（从default用户获取）
	models, err := s.database.GetAIModels("default")
	if err != nil {
		log.Printf("❌ 获取支持的AI模型失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取支持的AI模型失败"})
		return
	}

	c.JSON(http.StatusOK, models)
}

// handleGetSupportedExchanges 获取系统支持的交易所列表
func (s *Server) handleGetSupportedExchanges(c *gin.Context) {
	// 返回系统支持的交易所（从default用户获取）
	exchanges, err := s.database.GetExchanges("default")
	if err != nil {
		log.Printf("❌ 获取支持的交易所失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取支持的交易所失败"})
		return
	}

	c.JSON(http.StatusOK, exchanges)
}

// Start 启动服务器
func (s *Server) Start() error {
	// 绑定到 0.0.0.0 确保可以从外部访问
	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	log.Printf("🌐 API服务器启动在 http://0.0.0.0:%d", s.port)
	log.Printf("📊 API文档:")
	log.Printf("  • GET  /api/health           - 健康检查")
	log.Printf("  • GET  /api/traders          - 公开的AI交易员排行榜前50名（无需认证）")
	log.Printf("  • GET  /api/competition      - 公开的竞赛数据（无需认证）")
	log.Printf("  • GET  /api/top-traders      - 前5名交易员数据（无需认证，表现对比用）")
	log.Printf("  • GET  /api/equity-history?trader_id=xxx - 公开的收益率历史数据（无需认证，竞赛用）")
	log.Printf("  • GET  /api/equity-history-batch?trader_ids=a,b,c - 批量获取历史数据（无需认证，表现对比优化）")
	log.Printf("  • GET  /api/traders/:id/public-config - 公开的交易员配置（无需认证，不含敏感信息）")
	log.Printf("  • POST /api/traders          - 创建新的AI交易员")
	log.Printf("  • DELETE /api/traders/:id    - 删除AI交易员")
	log.Printf("  • POST /api/traders/:id/start - 启动AI交易员")
	log.Printf("  • POST /api/traders/:id/stop  - 停止AI交易员")
	log.Printf("  • GET  /api/models           - 获取AI模型配置")
	log.Printf("  • PUT  /api/models           - 更新AI模型配置")
	log.Printf("  • GET  /api/exchanges        - 获取交易所配置")
	log.Printf("  • PUT  /api/exchanges        - 更新交易所配置")
	log.Printf("  • GET  /api/status?trader_id=xxx     - 指定trader的系统状态")
	log.Printf("  • GET  /api/account?trader_id=xxx    - 指定trader的账户信息")
	log.Printf("  • GET  /api/positions?trader_id=xxx  - 指定trader的持仓列表")
	log.Printf("  • GET  /api/decisions?trader_id=xxx  - 指定trader的决策日志")
	log.Printf("  • GET  /api/decisions/latest?trader_id=xxx - 指定trader的最新决策")
	log.Printf("  • GET  /api/statistics?trader_id=xxx - 指定trader的统计信息")
	log.Printf("  • GET  /api/performance?trader_id=xxx - 指定trader的AI学习表现分析")
	log.Println()
	log.Printf("✅ API服务器就绪，等待请求...")

	return s.router.Run(addr)
}

// handleGetPromptTemplates 获取所有系统提示词模板列表
func (s *Server) handleGetPromptTemplates(c *gin.Context) {
	// 导入 decision 包
	templates := decision.GetAllPromptTemplates()

	// 转换为响应格式
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

// handleGetPromptTemplate 获取指定名称的提示词模板内容
func (s *Server) handleGetPromptTemplate(c *gin.Context) {
	templateName := c.Param("name")

	template, err := decision.GetPromptTemplate(templateName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("模板不存在: %s", templateName)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":    template.Name,
		"content": template.Content,
	})
}

// handlePublicTraderList 获取公开的交易员列表（无需认证）
func (s *Server) handlePublicTraderList(c *gin.Context) {
	// 从所有用户获取交易员信息
	competition, err := s.traderManager.GetCompetitionData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取交易员列表失败: %v", err),
		})
		return
	}

	// 获取traders数组
	tradersData, exists := competition["traders"]
	if !exists {
		c.JSON(http.StatusOK, []map[string]interface{}{})
		return
	}

	traders, ok := tradersData.([]map[string]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "交易员数据格式错误",
		})
		return
	}

	// 返回交易员基本信息，过滤敏感信息
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

// handlePublicCompetition 获取公开的竞赛数据（无需认证）
func (s *Server) handlePublicCompetition(c *gin.Context) {
	competition, err := s.traderManager.GetCompetitionData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取竞赛数据失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, competition)
}

// handleTopTraders 获取前5名交易员数据（无需认证，用于表现对比）
func (s *Server) handleTopTraders(c *gin.Context) {
	topTraders, err := s.traderManager.GetTopTradersData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取前10名交易员数据失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, topTraders)
}

// handleEquityHistoryBatch 批量获取多个交易员的收益率历史数据（无需认证，用于表现对比）
func (s *Server) handleEquityHistoryBatch(c *gin.Context) {
	var requestBody struct {
		TraderIDs []string `json:"trader_ids"`
	}

	// 尝试解析POST请求的JSON body
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		// 如果JSON解析失败，尝试从query参数获取（兼容GET请求）
		traderIDsParam := c.Query("trader_ids")
		if traderIDsParam == "" {
			// 如果没有指定trader_ids，则返回前5名的历史数据
			topTraders, err := s.traderManager.GetTopTradersData()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("获取前5名交易员失败: %v", err),
				})
				return
			}

			traders, ok := topTraders["traders"].([]map[string]interface{})
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "交易员数据格式错误"})
				return
			}

			// 提取trader IDs
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

		// 解析逗号分隔的trader IDs
		requestBody.TraderIDs = strings.Split(traderIDsParam, ",")
		for i := range requestBody.TraderIDs {
			requestBody.TraderIDs[i] = strings.TrimSpace(requestBody.TraderIDs[i])
		}
	}

	// 限制最多20个交易员，防止请求过大
	if len(requestBody.TraderIDs) > 20 {
		requestBody.TraderIDs = requestBody.TraderIDs[:20]
	}

	result := s.getEquityHistoryForTraders(requestBody.TraderIDs)
	c.JSON(http.StatusOK, result)
}

// getEquityHistoryForTraders 获取多个交易员的历史数据
func (s *Server) getEquityHistoryForTraders(traderIDs []string) map[string]interface{} {
	result := make(map[string]interface{})
	histories := make(map[string]interface{})
	errors := make(map[string]string)

	for _, traderID := range traderIDs {
		if traderID == "" {
			continue
		}

		trader, err := s.traderManager.GetTrader(traderID)
		if err != nil {
			errors[traderID] = "交易员不存在"
			continue
		}

		// 获取历史数据（用于对比展示，限制数据量）
		records, err := trader.GetDecisionLogger().GetLatestRecords(500)
		if err != nil {
			errors[traderID] = fmt.Sprintf("获取历史数据失败: %v", err)
			continue
		}

		// 构建收益率历史数据
		history := make([]map[string]interface{}, 0, len(records))
		for _, record := range records {
			// 计算总权益（余额+未实现盈亏）
			totalEquity := record.AccountState.TotalBalance + record.AccountState.TotalUnrealizedProfit

			history = append(history, map[string]interface{}{
				"timestamp":    record.Timestamp,
				"total_equity": totalEquity,
				"total_pnl":    record.AccountState.TotalUnrealizedProfit,
				"balance":      record.AccountState.TotalBalance,
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

// handleGetPublicTraderConfig 获取公开的交易员配置信息（无需认证，不包含敏感信息）
func (s *Server) handleGetPublicTraderConfig(c *gin.Context) {
	traderID := c.Param("id")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员ID不能为空"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 获取交易员的状态信息
	status := trader.GetStatus()

	// 只返回公开的配置信息，不包含API密钥等敏感数据
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

// handleRequestPasswordReset 处理密码重置请求
func (s *Server) handleRequestPasswordReset(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查用户是否存在
	user, err := s.database.GetUserByEmail(req.Email)
	if err != nil {
		// 即使用户不存在，也返回成功，防止邮箱枚举攻击
		c.JSON(http.StatusOK, gin.H{
			"message": "如果该邮箱已注册，您将收到密码重置邮件",
		})
		return
	}

	// 检查IP频率限制
	ipAddress := auth.ExtractIPFromRequest(map[string]string{
		"X-Forwarded-For": c.GetHeader("X-Forwarded-For"),
		"X-Real-IP":       c.GetHeader("X-Real-IP"),
	})

	failedAttempts, err := s.database.GetLoginAttemptsByIP(ipAddress)
	if err != nil {
		log.Printf("获取IP登录尝试次数失败: %v", err)
	}

	// 检查邮箱频率限制
	emailAttempts, err := s.database.GetLoginAttemptsByEmail(req.Email)
	if err != nil {
		log.Printf("获取邮箱登录尝试次数失败: %v", err)
	}

	// 频率限制：每IP每小时最多3次，每邮箱每小时最多3次
	if failedAttempts >= 3 || emailAttempts >= 3 {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": "请求过于频繁，请稍后再试",
		})
		return
	}

	// 生成密码重置令牌
	token, err := auth.GeneratePasswordResetToken()
	if err != nil {
		log.Printf("生成密码重置令牌失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成重置令牌失败"})
		return
	}

	tokenHash := auth.HashPasswordResetToken(token)
	expiresAt := time.Now().Add(1 * time.Hour)

	// 存储令牌
	err = s.database.CreatePasswordResetToken(user.ID, token, tokenHash, expiresAt)
	if err != nil {
		log.Printf("存储密码重置令牌失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建重置令牌失败"})
		return
	}

	// 获取前端URL（从环境变量或使用默认值）
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "https://web-pink-omega-40.vercel.app" // 默认前端URL
	}

	// 发送密码重置邮件（带重试）
	err = s.emailClient.SendPasswordResetEmailWithRetry(req.Email, token, frontendURL)
	if err != nil {
		// ✅ 改进: 详细的诊断日志，便于快速定位问题
		log.Printf("🔴 [PASSWORD_RESET_FAILED] 邮件发送失败（已重试）")
		log.Printf("   收件人: %s", req.Email)
		log.Printf("   错误信息: %v", err)
		log.Printf("   诊断检查清单:")
		log.Printf("     □ API Key配置: %s", func() string {
			if os.Getenv("RESEND_API_KEY") != "" {
				return "✅ 已配置"
			}
			return "❌ 未配置"
		}())
		log.Printf("     □ 发件人邮箱: %s", func() string {
			fromEmail := os.Getenv("RESEND_FROM_EMAIL")
			if fromEmail == "" {
				return "❌ 未配置，使用默认值"
			}
			return "✅ " + fromEmail
		}())
		log.Printf("     □ 前端URL: %s", frontendURL)
		log.Printf("   故障排查提示:")
		log.Printf("     1. 检查环境变量: echo $RESEND_API_KEY")
		log.Printf("     2. 检查发件人在Resend中是否被验证")
		log.Printf("     3. 检查API配额是否已用尽")
		log.Printf("     4. 检查网络连接是否正常")
		// 即使邮件发送失败，也返回成功消息（防止邮箱枚举）
		// 但记录错误日志供管理员查看
	} else {
		log.Printf("✅ [PASSWORD_RESET_SUCCESS] 邮件已发送（重试成功）- 收件人: %s", req.Email)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "如果该邮箱已注册，您将收到密码重置邮件",
	})
}

// handleResetPassword 处理密码重置确认
func (s *Server) handleResetPassword(c *gin.Context) {
	var req struct {
		Token    string `json:"token" binding:"required"`
		Password string `json:"password" binding:"required,min=8"`
		OTPCode  string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证令牌
	tokenHash := auth.HashPasswordResetToken(req.Token)
	userID, err := s.database.ValidatePasswordResetToken(tokenHash)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "重置链接无效或已过期"})
		return
	}

	// 获取用户信息
	user, err := s.database.GetUserByID(*userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 验证OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误"})
		return
	}

	// 生成新密码哈希
	newPasswordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新密码
	err = s.database.UpdateUserPassword(user.ID, newPasswordHash)
	if err != nil {
		log.Printf("更新用户密码失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新密码失败"})
		return
	}

	// 标记令牌为已使用
	err = s.database.MarkPasswordResetTokenAsUsed(tokenHash)
	if err != nil {
		log.Printf("标记令牌为已使用失败: %v", err)
	}

	// 使用户的所有其他令牌失效
	err = s.database.InvalidateAllPasswordResetTokens(user.ID)
	if err != nil {
		log.Printf("使其他令牌失效失败: %v", err)
	}

	// 重置失败尝试次数
	err = s.database.ResetUserFailedAttempts(user.ID)
	if err != nil {
		log.Printf("重置用户失败尝试次数失败: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "密码重置成功，请使用新密码登录",
	})
}

// handleGetUsers 处理获取用户列表请求
func (s *Server) handleGetUsers(c *gin.Context) {
	// 解析参数
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil || limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	search := c.Query("search")
	sort := c.DefaultQuery("sort", "created_at")
	order := c.DefaultQuery("order", "desc")

	// 验证排序字段
	validSortFields := []string{"created_at", "email"}
	sortValid := false
	for _, field := range validSortFields {
		if sort == field {
			sortValid = true
			break
		}
	}
	if !sortValid {
		sort = "created_at"
	}

	// 验证排序方向
	if order != "asc" && order != "desc" {
		order = "desc"
	}

	// 权限检查
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "未认证的访问",
		})
		return
	}

	currentUser := user.(*config.User)
	if !currentUser.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "权限不足，需要管理员权限",
		})
		return
	}

	// 调用数据库方法
	users, total, err := s.database.GetUsers(page, limit, search, sort, order)
	if err != nil {
		log.Printf("获取用户列表失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "获取用户列表失败",
		})
		return
	}

	// 计算分页信息
	totalPages := (total + limit - 1) / limit // 向上取整
	hasNext := page < totalPages
	hasPrev := page > 1

	// 构建响应
	response := gin.H{
		"users": users,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
			"has_next":    hasNext,
			"has_prev":    hasPrev,
		},
	}

	// 记录访问日志
	log.Printf("管理员 %s 查询用户列表 (page=%d, limit=%d, search=%s, sort=%s, order=%s)",
		currentUser.Email, page, limit, search, sort, order)

	// 返回响应
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
		"message": "获取用户列表成功",
	})
}

// adminMiddleware 管理员权限中间件（积分系统专用）
func (s *Server) adminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := ""
		if uid, exists := c.Get("userID"); exists {
			userID = uid.(string)
		}

		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "用户未认证",
			})
			c.Abort()
			return
		}

		// TODO: 实现管理员权限检查
		// 这里简化处理，实际应该查询数据库验证管理员身份
		// 暂时允许所有认证用户（测试用）
		c.Next()
	}
}
