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
		balanced, conservative, aggressive strategyI18n
	}
	locales := map[string]strategyLocale{
		"zh": {
			balanced:     strategyI18n{"均衡策略", "系统默认策略。均衡风险收益，适合大多数市场环境。5倍杠杆，最多3个仓位。"},
			conservative: strategyI18n{"稳健策略", "系统默认策略。低杠杆保守操作，优先保护本金。3倍杠杆，专注主流资产。"},
			aggressive:   strategyI18n{"积极策略", "系统默认策略。高杠杆主动交易，更广泛的币种选择，适合经验丰富的交易者。10倍杠杆，最多5个仓位。"},
		},
		"en": {
			balanced:     strategyI18n{"Balanced Strategy", "System default strategy. Balanced risk-reward, suitable for most market conditions. 5x leverage, up to 3 positions."},
			conservative: strategyI18n{"Conservative Strategy", "System default strategy. Low-leverage conservative trading, capital preservation first. 3x leverage, focused on major assets."},
			aggressive:   strategyI18n{"Aggressive Strategy", "System default strategy. High-leverage active trading, wider asset selection, for experienced traders. 10x leverage, up to 5 positions."},
		},
		"id": {
			balanced:     strategyI18n{"Strategi Seimbang", "Strategi default sistem. Risiko-reward seimbang, cocok untuk sebagian besar kondisi pasar. Leverage 5x, hingga 3 posisi."},
			conservative: strategyI18n{"Strategi Konservatif", "Strategi default sistem. Trading konservatif leverage rendah, utamakan perlindungan modal. Leverage 3x, fokus aset utama."},
			aggressive:   strategyI18n{"Strategi Agresif", "Strategi default sistem. Trading aktif leverage tinggi, pilihan aset lebih luas, untuk trader berpengalaman. Leverage 10x, hingga 5 posisi."},
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

	definitions := []strategyDef{
		{
			name:        locale.balanced.name,
			description: locale.balanced.description,
			isActive:    true,
			applyConfig: func(c *store.StrategyConfig) {
				// Uses default config as-is
			},
		},
		{
			name:        locale.conservative.name,
			description: locale.conservative.description,
			isActive:    false,
			applyConfig: func(c *store.StrategyConfig) {
				c.RiskControl.BTCETHMaxLeverage = 3
				c.RiskControl.AltcoinMaxLeverage = 3
				c.RiskControl.BTCETHMaxPositionValueRatio = 3.0
				c.RiskControl.AltcoinMaxPositionValueRatio = 0.5
				c.RiskControl.MinConfidence = 80
				c.RiskControl.MinRiskRewardRatio = 4.0
				c.Indicators.Klines.SelectedTimeframes = []string{"15m", "1h", "4h"}
				c.Indicators.Klines.PrimaryTimeframe = "15m"
			},
		},
		{
			name:        locale.aggressive.name,
			description: locale.aggressive.description,
			isActive:    false,
			applyConfig: func(c *store.StrategyConfig) {
				c.RiskControl.BTCETHMaxLeverage = 10
				c.RiskControl.AltcoinMaxLeverage = 7
				c.RiskControl.MaxPositions = 5
				c.RiskControl.AltcoinMaxPositionValueRatio = 2.0
				c.RiskControl.MinConfidence = 70
				c.CoinSource.AI500Limit = 5
				c.CoinSource.UseOITop = true
				c.CoinSource.OITopLimit = 5
				c.Indicators.Klines.SelectedTimeframes = []string{"3m", "15m", "1h"}
				c.Indicators.Klines.PrimaryTimeframe = "3m"
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

	return s.store.Transaction(func(tx *gorm.DB) error {
		for _, strategy := range strategies {
			if err := tx.Create(strategy).Error; err != nil {
				return fmt.Errorf("failed to create strategy %q: %w", strategy.Name, err)
			}
			logger.Infof("  ✓ Created default strategy: %s (active=%v)", strategy.Name, strategy.IsActive)
		}
		return nil
	})
}
