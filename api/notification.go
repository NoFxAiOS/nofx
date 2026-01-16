package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"nofx/crypto"
	"nofx/logger"
	"nofx/notify"
	"nofx/store"

	"github.com/gin-gonic/gin"
)

// NotificationConfig request/response struct
type NotificationConfigRequest struct {
	WxPusherToken string `json:"wx_pusher_token"` // WxPusher app token
	WxPusherUIDs  string `json:"wx_pusher_uids"`  // JSON array of UIDs
	IsEnabled     bool   `json:"is_enabled"`
	EnableDecision *bool `json:"enable_decision"`
	EnableTradeOpen *bool `json:"enable_trade_open"`
	EnableTradeClose *bool `json:"enable_trade_close"`
}

// HandleGetNotificationConfig gets the notification config for a trader
func (s *Server) HandleGetNotificationConfig(c *gin.Context) {
	userID := c.GetString("user_id")

	traderID := c.Query("trader_id")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trader_id is required"})
		return
	}

	config, err := s.notificationStore.GetByTraderID(userID, traderID)
	if err != nil {
		logger.Warnf("Failed to get notification config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get notification config"})
		return
	}

	if config == nil {
		config = &store.NotificationConfig{
			ID:        fmt.Sprintf("%s_%s", userID, traderID),
			UserID:    userID,
			TraderID:  traderID,
			IsEnabled: false,
			EnableDecision: true,
			EnableTradeOpen: true,
			EnableTradeClose: true,
		}
	}

	// Backfill defaults for older rows
	if config.IsEnabled {
		if !config.EnableDecision && !config.EnableTradeOpen && !config.EnableTradeClose {
			config.EnableDecision = true
			config.EnableTradeOpen = true
			config.EnableTradeClose = true
		}
	}

	// Hide the token for security (show *** if token exists)
	responseConfig := *config
	if config.WxPusherToken != "" {
		responseConfig.WxPusherToken = "***"
	}

	c.JSON(http.StatusOK, responseConfig)
}

// HandleUpdateNotificationConfig updates the notification config for a trader
func (s *Server) HandleUpdateNotificationConfig(c *gin.Context) {
	userID := c.GetString("user_id")

	traderID := c.Query("trader_id")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trader_id is required"})
		return
	}

	var req NotificationConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate UIDs are valid JSON if provided
	if req.WxPusherUIDs != "" && req.IsEnabled {
		var uids []string
		if err := json.Unmarshal([]byte(req.WxPusherUIDs), &uids); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid wx_pusher_uids format, must be JSON array"})
			return
		}
		if len(uids) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "wx_pusher_uids cannot be empty when enabled"})
			return
		}
	}

	// Validate token when enabled
	if req.IsEnabled && req.WxPusherToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "wx_pusher_token is required when notifications are enabled"})
		return
	}

	// Get or create config
	config, err := s.notificationStore.GetByTraderID(userID, traderID)
	if err != nil {
		logger.Warnf("Failed to get notification config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get notification config"})
		return
	}

	if config == nil {
		config = &store.NotificationConfig{
			ID:       fmt.Sprintf("%s_%s", userID, traderID),
			UserID:   userID,
			TraderID: traderID,
			EnableDecision: true,
			EnableTradeOpen: true,
			EnableTradeClose: true,
		}
	} else if config.ID == "" {
		// Fix legacy records without ID
		config.ID = fmt.Sprintf("%s_%s", userID, traderID)
	}

	// Backfill defaults for legacy rows when enabled but flags are all false
	if config.IsEnabled && !config.EnableDecision && !config.EnableTradeOpen && !config.EnableTradeClose {
		config.EnableDecision = true
		config.EnableTradeOpen = true
		config.EnableTradeClose = true
	}

	// Update fields
	config.WxPusherUIDs = req.WxPusherUIDs
	config.IsEnabled = req.IsEnabled
	if req.EnableDecision != nil {
		config.EnableDecision = *req.EnableDecision
	}
	if req.EnableTradeOpen != nil {
		config.EnableTradeOpen = *req.EnableTradeOpen
	}
	if req.EnableTradeClose != nil {
		config.EnableTradeClose = *req.EnableTradeClose
	}
	
	// Update token if provided (will be encrypted automatically)
	if req.WxPusherToken != "" {
		config.WxPusherToken = crypto.EncryptedString(req.WxPusherToken)
	}

	if err := s.notificationStore.CreateOrUpdate(config); err != nil {
		logger.Warnf("Failed to update notification config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update notification config"})
		return
	}

	// Don't return the token in response for security
	responseConfig := *config
	responseConfig.WxPusherToken = "***"

	c.JSON(http.StatusOK, gin.H{
		"message": "Notification config updated",
		"data":    responseConfig,
	})
}

// HandleTestNotification sends a test notification for a specific scenario
func (s *Server) HandleTestNotification(c *gin.Context) {
	userID := c.GetString("user_id")

	traderID := c.Query("trader_id")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trader_id is required"})
		return
	}

	scenario := c.DefaultQuery("type", "decision") // decision|trade_open|trade_close

	// Get notification config
	config, err := s.notificationStore.GetByTraderID(userID, traderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get notification config"})
		return
	}

	if config == nil || !config.IsEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Notifications not enabled for this trader"})
		return
	}

	// Backfill defaults for legacy rows
	if !config.EnableDecision && !config.EnableTradeOpen && !config.EnableTradeClose {
		config.EnableDecision = true
		config.EnableTradeOpen = true
		config.EnableTradeClose = true
	}

	// Scenario toggle checks
	switch scenario {
	case "decision":
		if !config.EnableDecision {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Decision notifications disabled"})
			return
		}
	case "trade_open":
		if !config.EnableTradeOpen {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Trade open notifications disabled"})
			return
		}
	case "trade_close":
		if !config.EnableTradeClose {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Trade close notifications disabled"})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid type, must be decision|trade_open|trade_close"})
		return
	}

	// Parse UIDs
	var uids []string
	if config.WxPusherUIDs != "" {
		if err := json.Unmarshal([]byte(config.WxPusherUIDs), &uids); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid UIDs configuration"})
			return
		}
	}

	if len(uids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No UIDs configured"})
		return
	}

	// Dispatch scenario-specific test using existing manager logic
	switch scenario {
	case "decision":
		sample := map[string]interface{}{
			"symbol": "BTCUSDT",
			"action": "BUY",
			"confidence": 0.82,
			"reason": "Sample decision test",
		}
		err = s.notificationManager.NotifyDecision(traderID, userID, sample, notify.WithSummary("Test: AI Decision"))
	case "trade_open":
		err = s.notificationManager.NotifyTradeOpened(traderID, userID, "BTCUSDT", "long", 0.01, 50000)
	case "trade_close":
		err = s.notificationManager.NotifyTradeClosed(traderID, userID, "BTCUSDT", "long", 0.01, 50500, 5.0)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to send notification: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Test %s notification sent successfully", scenario),
	})
}

// HandleDisableNotifications disables all notifications for a trader
func (s *Server) HandleDisableNotifications(c *gin.Context) {
	userID := c.GetString("user_id")

	traderID := c.Query("trader_id")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trader_id is required"})
		return
	}

	if err := s.notificationStore.Delete(userID, traderID); err != nil {
		logger.Warnf("Failed to delete notification config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete notification config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Notifications disabled",
	})
}
