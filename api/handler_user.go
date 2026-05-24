package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"nofx/auth"
	"nofx/logger"
	"nofx/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

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

// handleRegister Handle user registration request.
// handleRegister allows registration only when no users exist yet (first-time setup).
// This is a single-user system; subsequent registrations are permanently closed.
func (s *Server) handleRegister(c *gin.Context) {
	userCount, err := s.store.User().Count()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check user count"})
		return
	}

	if userCount > 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "System already initialized"})
		return
	}

	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
		Lang     string `json:"lang"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	lang := req.Lang
	if lang != "zh" && lang != "id" {
		lang = "en"
	}

	// Check if email already exists
	_, err = s.store.User().GetByEmail(req.Email)
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

	// Create user
	userID := uuid.New().String()
	user := &store.User{
		ID:           userID,
		Email:        req.Email,
		PasswordHash: passwordHash,
	}

	err = s.store.User().Create(user)
	if err != nil {
		SafeInternalError(c, "Failed to create user", err)
		return
	}

	// Adopt orphan records from previous account (e.g. after account reset)
	// This preserves wallet keys and exchange configs so funds are not lost.
	s.adoptOrphanRecords(userID)

	// Generate JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Initialize default model and exchange configs for user
	err = s.initUserDefaultConfigs(user.ID, lang)
	if err != nil {
		logger.Infof("Failed to initialize user default configs: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
		"email":   user.Email,
		"message": "Registration successful",
	})
}

// handleLogin Handle user login request
func (s *Server) handleLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
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

	// Issue token directly after password verification.
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

// handleChangePassword changes the password for the currently authenticated user.
func (s *Server) handleChangePassword(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "new_password is required (min 8 chars)")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		SafeInternalError(c, "Password processing failed", err)
		return
	}
	if err := s.store.User().UpdatePassword(userID, hash); err != nil {
		SafeInternalError(c, "Failed to update password", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password updated"})
}

// handleResetPassword Reset password via email and new password
func (s *Server) handleResetPassword(c *gin.Context) {
	var req struct {
		Email       string `json:"email" binding:"required,email"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Query user
	user, err := s.store.User().GetByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email does not exist"})
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

// handleResetAccount clears user authentication data so the system returns to
// uninitialized state for re-registration. Wallet keys (ai_models) are preserved
// so funds are not lost — they will be adopted by the new account during onboarding.
func (s *Server) handleResetAccount(c *gin.Context) {
	err := s.store.Transaction(func(tx *gorm.DB) error {
		// Delete traders and strategies (config, not funds)
		tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&store.Trader{})
		tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&store.Strategy{})
		// Delete users — ai_models and exchanges are intentionally kept
		// so wallet private keys and exchange configs survive re-registration
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&store.User{}).Error; err != nil {
			return fmt.Errorf("failed to delete users: %w", err)
		}
		return nil
	})
	if err != nil {
		SafeInternalError(c, "Failed to reset account", err)
		return
	}

	logger.Infof("✓ User accounts cleared (wallets preserved) — system reset to uninitialized")
	c.JSON(http.StatusOK, gin.H{"message": "Account reset successful, you can now register a new account"})
}

// adoptOrphanRecords re-assigns ai_models and exchanges whose user_id no longer
// exists in the users table. This happens after account reset so the new user
// inherits the previous wallet keys and exchange configurations.
func (s *Server) adoptOrphanRecords(newUserID string) {
	db := s.store.GormDB()
	result := db.Model(&store.AIModel{}).
		Where("user_id NOT IN (SELECT id FROM users)").
		Update("user_id", newUserID)
	if result.RowsAffected > 0 {
		logger.Infof("✓ Adopted %d orphan ai_model(s) for new user %s", result.RowsAffected, newUserID)
	}

	result = db.Model(&store.Exchange{}).
		Where("user_id NOT IN (SELECT id FROM users)").
		Update("user_id", newUserID)
	if result.RowsAffected > 0 {
		logger.Infof("✓ Adopted %d orphan exchange(s) for new user %s", result.RowsAffected, newUserID)
	}
}

// initUserDefaultConfigs Initialize default configs for new user
func (s *Server) initUserDefaultConfigs(userID string, lang string) error {
	if err := s.createDefaultStrategies(userID, lang); err != nil {
		logger.Warnf("Failed to create default strategies for user %s: %v", userID, err)
		// Non-fatal: user can create strategies manually
	}
	logger.Infof("✓ User %s registration completed with default strategies", userID)
	return nil
}

func (s *Server) createDefaultStrategies(userID string, lang string) error {
	type strategyI18n struct {
		name, description string
	}
	type strategyLocale struct {
		trend, megaCap, breakout strategyI18n
	}
	locales := map[string]strategyLocale{
		"zh": {
			trend:    strategyI18n{"美股趋势策略", "开箱即用的 Hyperliquid 美股 USDC 策略。只扫描流动性更好的美股合约，低杠杆、低频率，适合直接创建 Agent 后运行。"},
			megaCap:  strategyI18n{"美股大盘稳健策略", "开箱即用的 Hyperliquid 美股 USDC 策略。固定关注 AAPL、MSFT、GOOGL、AMZN、META 等大盘股，强调趋势确认和回撤控制。"},
			breakout: strategyI18n{"美股突破策略", "开箱即用的 Hyperliquid 美股 USDC 策略。扫描 24h 强势美股，等待突破确认后再开仓，避免频繁追涨。"},
		},
		"en": {
			trend:    strategyI18n{"US Stock Trend Strategy", "Ready-to-run Hyperliquid USDC equity strategy. Scans liquid US stock perps with low leverage and low trade frequency, suitable for one-click Agent deployment."},
			megaCap:  strategyI18n{"US Mega-Cap Steady Strategy", "Ready-to-run Hyperliquid USDC equity strategy. Fixed universe: AAPL, MSFT, GOOGL, AMZN and META, with trend confirmation and drawdown control."},
			breakout: strategyI18n{"US Stock Breakout Strategy", "Ready-to-run Hyperliquid USDC equity strategy. Scans 24h strong US stocks and waits for breakout confirmation before entering, avoiding impulsive chasing."},
		},
		"id": {
			trend:    strategyI18n{"Strategi Tren Saham AS", "Strategi saham AS USDC Hyperliquid siap jalan. Memindai perp saham AS likuid dengan leverage rendah dan frekuensi rendah."},
			megaCap:  strategyI18n{"Strategi Stabil Mega-Cap AS", "Strategi saham AS USDC Hyperliquid siap jalan. Universe tetap: AAPL, MSFT, GOOGL, AMZN, META, dengan konfirmasi tren."},
			breakout: strategyI18n{"Strategi Breakout Saham AS", "Strategi saham AS USDC Hyperliquid siap jalan. Memindai saham AS kuat 24 jam dan menunggu konfirmasi breakout."},
		},
	}
	locale, ok := locales[lang]
	if !ok {
		locale = locales["en"]
	}

	type strategyDef struct {
		name        string
		description string
		isActive    bool
		applyConfig func(*store.StrategyConfig)
	}

	setStockRank := func(c *store.StrategyConfig, direction string, limit int) {
		c.CoinSource.SourceType = "hyper_rank"
		c.CoinSource.StaticCoins = nil
		c.CoinSource.UseAI500 = false
		c.CoinSource.UseOITop = false
		c.CoinSource.UseOILow = false
		c.CoinSource.UseHyperAll = false
		c.CoinSource.UseHyperMain = false
		c.CoinSource.HyperRankCategory = "stock"
		c.CoinSource.HyperRankDirection = direction
		c.CoinSource.HyperRankLimit = limit
	}
	setStaticStocks := func(c *store.StrategyConfig, symbols []string) {
		c.CoinSource.SourceType = "static"
		c.CoinSource.StaticCoins = symbols
		c.CoinSource.UseAI500 = false
		c.CoinSource.UseOITop = false
		c.CoinSource.UseOILow = false
		c.CoinSource.UseHyperAll = false
		c.CoinSource.UseHyperMain = false
	}
	setStableRisk := func(c *store.StrategyConfig) {
		c.RiskControl.MaxPositions = 2
		c.RiskControl.BTCETHMaxLeverage = 3
		c.RiskControl.AltcoinMaxLeverage = 3
		c.RiskControl.BTCETHMaxPositionValueRatio = 2.0
		c.RiskControl.AltcoinMaxPositionValueRatio = 0.6
		c.RiskControl.MaxMarginUsage = 0.45
		c.RiskControl.MinConfidence = 78
		c.RiskControl.MinRiskRewardRatio = 3.0
		c.Indicators.Klines.PrimaryTimeframe = "15m"
		c.Indicators.Klines.LongerTimeframe = "4h"
		c.Indicators.Klines.SelectedTimeframes = []string{"15m", "1h", "4h"}
		c.Indicators.EnableEMA = true
		c.Indicators.EnableMACD = true
		c.Indicators.EnableRSI = true
		c.Indicators.EnableATR = true
		c.Indicators.EnableVolume = true
	}

	definitions := []strategyDef{
		{
			name:        locale.trend.name,
			description: locale.trend.description,
			isActive:    true,
			applyConfig: func(c *store.StrategyConfig) {
				setStockRank(c, "volume", 5)
				setStableRisk(c)
			},
		},
		{
			name:        locale.megaCap.name,
			description: locale.megaCap.description,
			isActive:    false,
			applyConfig: func(c *store.StrategyConfig) {
				setStaticStocks(c, []string{"AAPL-USDC", "MSFT-USDC", "GOOGL-USDC", "AMZN-USDC", "META-USDC"})
				setStableRisk(c)
				c.RiskControl.MaxPositions = 2
				c.RiskControl.MinConfidence = 80
			},
		},
		{
			name:        locale.breakout.name,
			description: locale.breakout.description,
			isActive:    false,
			applyConfig: func(c *store.StrategyConfig) {
				setStockRank(c, "gainers", 5)
				setStableRisk(c)
				c.RiskControl.MinConfidence = 82
				c.RiskControl.MinRiskRewardRatio = 3.5
			},
		},
	}

	// GetDefaultStrategyConfig only supports zh/en; map id -> en
	configLang := lang
	if lang == "id" {
		configLang = "en"
	}

	// Pre-build all strategy objects before opening the transaction
	var strategies []*store.Strategy
	for _, def := range definitions {
		config := store.GetDefaultStrategyConfig(configLang)
		def.applyConfig(&config)
		config.ClampLimits()

		strategy := &store.Strategy{
			ID:          uuid.New().String(),
			UserID:      userID,
			Name:        def.name,
			Description: def.description,
			IsActive:    def.isActive,
			IsDefault:   false,
		}
		if err := strategy.SetConfig(&config); err != nil {
			return fmt.Errorf("failed to set config for strategy %q: %w", def.name, err)
		}
		strategies = append(strategies, strategy)
	}

	legacyDefaultNames := []string{
		"均衡策略", "稳健策略", "积极策略",
		"Balanced Strategy", "Conservative Strategy", "Aggressive Strategy",
		"Strategi Seimbang", "Strategi Konservatif", "Strategi Agresif",
	}

	return s.store.Transaction(func(tx *gorm.DB) error {
		// Remove obsolete built-in risk-profile presets for this user. If a trader still
		// references one of them, keep it to avoid breaking an existing running setup.
		deleteResult := tx.Where("user_id = ? AND name IN ? AND id NOT IN (SELECT strategy_id FROM traders WHERE user_id = ? AND strategy_id IS NOT NULL)", userID, legacyDefaultNames, userID).
			Delete(&store.Strategy{})
		if deleteResult.Error != nil {
			return fmt.Errorf("failed to remove legacy default strategies: %w", deleteResult.Error)
		}
		if deleteResult.RowsAffected > 0 {
			logger.Infof("  ✓ Removed %d legacy default strategy preset(s)", deleteResult.RowsAffected)
		}

		var activeCount int64
		if err := tx.Model(&store.Strategy{}).Where("user_id = ? AND is_active = ?", userID, true).Count(&activeCount).Error; err != nil {
			return fmt.Errorf("failed to count active strategies: %w", err)
		}

		for _, strategy := range strategies {
			var existing int64
			if err := tx.Model(&store.Strategy{}).Where("user_id = ? AND name = ?", userID, strategy.Name).Count(&existing).Error; err != nil {
				return fmt.Errorf("failed to check strategy %q: %w", strategy.Name, err)
			}
			if existing > 0 {
				continue
			}
			if activeCount > 0 {
				strategy.IsActive = false
			}
			if err := tx.Create(strategy).Error; err != nil {
				return fmt.Errorf("failed to create strategy %q: %w", strategy.Name, err)
			}
			if strategy.IsActive {
				activeCount++
			}
			logger.Infof("  ✓ Created default strategy: %s (active=%v)", strategy.Name, strategy.IsActive)
		}
		return nil
	})
}
