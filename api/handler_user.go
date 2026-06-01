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

	// NOTE: Orphan record adoption was removed for security reasons. Previously,
	// after a reset-account call, any new user would inherit the prior owner's
	// wallet keys and exchange API credentials — a catastrophic IDOR/takeover
	// path. Operators who need to migrate credentials across users must do so
	// explicitly via export/import, never via implicit adoption on registration.

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

// resetPasswordConfirmPhrase is the friction step for /api/reset-password.
// Same security rationale as resetAccountConfirmPhrase — not a cryptographic
// check, just a guard against accidental and drive-by triggers.
const resetPasswordConfirmPhrase = "I_UNDERSTAND_THIS_RESETS_MY_PASSWORD"

// handleResetPassword resets the password for the given email.
//
// SECURITY NOTE: This endpoint is intentionally callable without a JWT — it
// IS the recovery path for "forgot password" in the single-user self-hosted
// threat model this project targets. A logged-in user changes password via
// PUT /api/user/password; this endpoint exists for users who can no longer
// log in. Mitigations:
//
//  1. Requires the confirm phrase (blocks accidental and drive-by triggers).
//  2. New password must be ≥ 8 chars.
//  3. Authenticated session change is preferred (PUT /api/user/password).
//
// Operators exposing the API to the public internet should put a reverse-proxy
// auth layer in front of /api/reset-password OR set up out-of-band recovery
// (email link, OTP) instead of relying on this endpoint.
func (s *Server) handleResetPassword(c *gin.Context) {
	var req struct {
		Email       string `json:"email" binding:"required,email"`
		NewPassword string `json:"new_password" binding:"required,min=8"`
		Confirm     string `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "email, new_password (min 8 chars), and confirm are required")
		return
	}
	if req.Confirm != resetPasswordConfirmPhrase {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Confirmation phrase required",
			"hint":  `Body must include {"confirm":"` + resetPasswordConfirmPhrase + `"}`,
		})
		return
	}

	user, err := s.store.User().GetByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email does not exist"})
		return
	}

	newPasswordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		SafeInternalError(c, "Password processing failed", err)
		return
	}
	if err := s.store.User().UpdatePassword(user.ID, newPasswordHash); err != nil {
		SafeInternalError(c, "Password update failed", err)
		return
	}

	logger.Infof("✓ User %s password reset via reset endpoint", user.Email)
	c.JSON(http.StatusOK, gin.H{"message": "Password reset successful, please login with new password"})
}

// resetAccountConfirmPhrase must appear in the request body for /api/reset-account.
// This is the single intentional friction step that prevents accidental wipes
// from drive-by scripts and crawlers. It is NOT a cryptographic check — anyone
// who reads this source can send the phrase. The real safety comes from:
//
//  1. Wallet keys are NO LONGER auto-adopted by the next registrant
//     (adoptOrphanRecords was removed). The historical takeover path was:
//     reset → register → inherit prior wallet → drain. That path is closed.
//  2. The destructive action is loud (logged at Warn level).
//
// Operators who expose the API to the public internet and want stronger
// gating can wrap this route with a reverse-proxy auth header check.
const resetAccountConfirmPhrase = "I_UNDERSTAND_THIS_DELETES_EVERYTHING"

// handleResetAccount wipes all users + traders + strategies + AI models +
// exchanges, returning the system to uninitialized state.
//
// SECURITY NOTE: For the single-user, self-hosted threat model this project
// targets, this endpoint is intentionally callable without a JWT — the
// frontend "forgot account" button must still work after the user forgets
// their password. The confirm phrase blocks accidental and drive-by triggers;
// the removal of orphan adoption blocks the post-reset takeover. A determined
// attacker on a public-facing deployment can still grief by wiping local
// state, but they cannot steal funds (everything is deleted, not transferred).
func (s *Server) handleResetAccount(c *gin.Context) {
	var req struct {
		Confirm string `json:"confirm"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Confirm != resetAccountConfirmPhrase {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Confirmation phrase required",
			"hint":  `Body must include {"confirm":"` + resetAccountConfirmPhrase + `"}`,
		})
		return
	}

	err := s.store.Transaction(func(tx *gorm.DB) error {
		// Wipe ALL records — including wallet keys and exchange credentials.
		// Preserving them across user identities is what enabled the takeover.
		tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&store.Trader{})
		tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&store.Strategy{})
		tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&store.AIModel{})
		tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&store.Exchange{})
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&store.User{}).Error; err != nil {
			return fmt.Errorf("failed to delete users: %w", err)
		}
		return nil
	})
	if err != nil {
		SafeInternalError(c, "Failed to reset account", err)
		return
	}

	logger.Warnf("⚠ Account reset performed — all users, traders, strategies, ai_models, exchanges wiped")
	c.JSON(http.StatusOK, gin.H{
		"message": "System wiped. All wallet keys and exchange credentials were deleted. Register a fresh account and re-import everything.",
	})
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
