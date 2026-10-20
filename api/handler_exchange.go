package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"nofx/config"
	"nofx/crypto"
	"nofx/logger"
	"nofx/store"

	"github.com/gin-gonic/gin"
)

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
	PaperConfig           *store.PaperConfig `json:"paper_config,omitempty"`
	ID                    string             `json:"id"`            // UUID
	ExchangeType          string             `json:"exchange_type"` // "binance", "bybit", "okx", "hyperliquid", "aster", "lighter"
	AccountName           string             `json:"account_name"`  // User-defined account name
	Name                  string             `json:"name"`          // Display name
	Type                  string             `json:"type"`          // "cex" or "dex"
	Enabled               bool               `json:"enabled"`
	Testnet               bool               `json:"testnet,omitempty"`
	HyperliquidWalletAddr string             `json:"hyperliquidWalletAddr"` // Hyperliquid wallet address (not sensitive)
	AsterUser             string             `json:"asterUser"`             // Aster username (not sensitive)
	AsterSigner           string             `json:"asterSigner"`           // Aster signer (not sensitive)
	LighterWalletAddr     string             `json:"lighterWalletAddr"`     // LIGHTER wallet address (not sensitive)
}

// ExchangeConfigPayload is the per-exchange body for update requests.
type ExchangeConfigPayload struct {
	PaperConfig             *store.PaperConfig `json:"paper_config"`
	AccountName             string             `json:"account_name"`
	Enabled                 bool               `json:"enabled"`
	APIKey                  string             `json:"api_key"`
	SecretKey               string             `json:"secret_key"`
	Passphrase              string             `json:"passphrase"` // OKX specific
	Testnet                 bool               `json:"testnet"`
	HyperliquidWalletAddr   string             `json:"hyperliquid_wallet_addr"`
	HyperliquidUnifiedAcct  bool               `json:"hyperliquid_unified_account"` // Unified Account mode
	AsterUser               string             `json:"aster_user"`
	AsterSigner             string             `json:"aster_signer"`
	AsterPrivateKey         string             `json:"aster_private_key"`
	LighterWalletAddr       string             `json:"lighter_wallet_addr"`
	LighterPrivateKey       string             `json:"lighter_private_key"`
	LighterAPIKeyPrivateKey string             `json:"lighter_api_key_private_key"`
	LighterAPIKeyIndex      int                `json:"lighter_api_key_index"`
}

type UpdateExchangeConfigRequest struct {
	Exchanges map[string]ExchangeConfigPayload `json:"exchanges"`
}

// CreateExchangeRequest request structure for creating a new exchange account
type CreateExchangeRequest struct {
	PaperConfig             *store.PaperConfig `json:"paper_config"`
	ExchangeType            string             `json:"exchange_type" binding:"required"` // "binance", "bybit", "okx", "hyperliquid", "aster", "lighter"
	AccountName             string             `json:"account_name"`                     // User-defined account name
	Enabled                 bool               `json:"enabled"`
	APIKey                  string             `json:"api_key"`
	SecretKey               string             `json:"secret_key"`
	Passphrase              string             `json:"passphrase"`
	Testnet                 bool               `json:"testnet"`
	HyperliquidWalletAddr   string             `json:"hyperliquid_wallet_addr"`
	HyperliquidUnifiedAcct  bool               `json:"hyperliquid_unified_account"` // Unified Account mode: Spot as Perp collateral
	AsterUser               string             `json:"aster_user"`
	AsterSigner             string             `json:"aster_signer"`
	AsterPrivateKey         string             `json:"aster_private_key"`
	LighterWalletAddr       string             `json:"lighter_wallet_addr"`
	LighterPrivateKey       string             `json:"lighter_private_key"`
	LighterAPIKeyPrivateKey string             `json:"lighter_api_key_private_key"`
	LighterAPIKeyIndex      int                `json:"lighter_api_key_index"`
}

// handleGetExchangeConfigs Get exchange configurations
func (s *Server) handleGetExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	logger.Infof("🔍 Querying exchange configs for user %s", userID)
	exchanges, err := s.store.Exchange().List(userID)
	if err != nil {
		SafeInternalError(c, "Failed to get exchange configs", err)
		return
	}

	// If no exchanges in database, return empty array (user needs to create accounts)
	if len(exchanges) == 0 {
		logger.Infof("⚠️ No exchanges in database for user %s", userID)
		c.JSON(http.StatusOK, []SafeExchangeConfig{})
		return
	}

	logger.Infof("✅ Found %d exchange configs", len(exchanges))

	// Convert to safe response structure, remove sensitive information
	safeExchanges := make([]SafeExchangeConfig, len(exchanges))
	for i, exchange := range exchanges {
		safeExchanges[i] = SafeExchangeConfig{
			ID:                    exchange.ID,
			ExchangeType:          exchange.ExchangeType,
			AccountName:           exchange.AccountName,
			Name:                  exchange.Name,
			Type:                  exchange.Type,
			Enabled:               exchange.Enabled,
			Testnet:               exchange.Testnet,
			HyperliquidWalletAddr: exchange.HyperliquidWalletAddr,
			AsterUser:             exchange.AsterUser,
			AsterSigner:           exchange.AsterSigner,
			LighterWalletAddr:     exchange.LighterWalletAddr,
		}
		if exchange.ExchangeType == "paper" {
			cfg, err := exchange.ParsePaperConfig()
			if err != nil {
				SafeInternalError(c, "读取模拟盘配置失败", err)
				return
			}
			safeExchanges[i].PaperConfig = &cfg
		}
	}

	c.JSON(http.StatusOK, safeExchanges)
}

// handleUpdateExchangeConfigs Update exchange configurations (supports both encrypted and plain text based on config)
func (s *Server) handleUpdateExchangeConfigs(c *gin.Context) {
	s.traderOpsMu.Lock()
	defer s.traderOpsMu.Unlock()
	userID := c.GetString("user_id")
	cfg := config.Get()

	// Read raw request body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var req UpdateExchangeConfigRequest

	// Check if transport encryption is enabled
	if !cfg.TransportEncryption {
		// Transport encryption disabled, accept plain JSON
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			logger.Infof("❌ Failed to parse plain JSON request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
		logger.Infof("📝 Received plain text exchange config (UserID: %s)", userID)
	} else {
		// Transport encryption enabled, require encrypted payload
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
		decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveDataForUser(&encryptedPayload, userID)
		if err != nil {
			logger.Infof("❌ Failed to decrypt exchange config (UserID: %s): %v", userID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decrypt data"})
			return
		}

		// Parse decrypted data
		if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
			logger.Infof("❌ Failed to parse decrypted data: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
			return
		}
		logger.Infof("🔓 Decrypted exchange config data (UserID: %s)", userID)
	}

	// Validate the entire batch before any write.
	for exchangeID, data := range req.Exchanges {
		ex, err := s.store.Exchange().GetByID(userID, exchangeID)
		if err != nil {
			SafeNotFound(c, "Exchange")
			return
		}
		if err := store.ValidateExchangeEnvironment(ex.ExchangeType, data.Testnet); err != nil {
			SafeBadRequest(c, err.Error())
			return
		}
		if ex.ExchangeType == "paper" {
			_, err := ex.ParsePaperConfig()
			if data.PaperConfig != nil {
				err = data.PaperConfig.Validate()
			}
			if err != nil {
				SafeBadRequest(c, err.Error())
				return
			}
		}
	}
	// Update each exchange's configuration and track traders that need reload
	tradersToReload := make(map[string]bool)
	for exchangeID, exchangeData := range req.Exchanges {
		ex, err := s.store.Exchange().GetByID(userID, exchangeID)
		if err != nil {
			SafeNotFound(c, "Exchange")
			return
		}
		if err := store.ValidateExchangeEnvironment(ex.ExchangeType, exchangeData.Testnet); err != nil {
			SafeBadRequest(c, err.Error())
			return
		}
		// Find traders using this exchange BEFORE updating
		traders, _ := s.store.Trader().ListByExchangeID(userID, exchangeID)
		for _, t := range traders {
			tradersToReload[t.ID] = true
		}

		if ex.ExchangeType == "paper" {
			cfg, parseErr := ex.ParsePaperConfig()
			if exchangeData.PaperConfig != nil {
				cfg, parseErr = *exchangeData.PaperConfig, nil
			}
			if parseErr != nil {
				SafeBadRequest(c, parseErr.Error())
				return
			}
			err = s.store.Exchange().UpdatePaperConfig(userID, exchangeID, exchangeData.AccountName, exchangeData.Enabled, cfg)
		} else {
			err = s.store.Exchange().Update(userID, exchangeID, exchangeData.Enabled, exchangeData.APIKey, exchangeData.SecretKey, exchangeData.Passphrase, exchangeData.Testnet, exchangeData.HyperliquidWalletAddr, exchangeData.HyperliquidUnifiedAcct, exchangeData.AsterUser, exchangeData.AsterSigner, exchangeData.AsterPrivateKey, exchangeData.LighterWalletAddr, exchangeData.LighterPrivateKey, exchangeData.LighterAPIKeyPrivateKey, exchangeData.LighterAPIKeyIndex)
		}
		if err != nil {
			SafeInternalError(c, fmt.Sprintf("Update exchange %s", exchangeID), err)
			return
		}
	}

	s.exchangeAccountStateCache.Invalidate(userID)

	// Remove affected traders from memory BEFORE reloading to pick up new config.
	// 2026-10-01: background stop→reload chain — see handler_ai_model.go for
	// the rationale (Stop() joins the in-flight AI cycle and used to hang the
	// save request for minutes).
	reloadTargets := make([]string, 0, len(tradersToReload))
	for traderID := range tradersToReload {
		reloadTargets = append(reloadTargets, traderID)
	}
	if len(reloadTargets) > 0 {
		targets := reloadTargets
		go func() {
			for _, traderID := range targets {
				logger.Infof("🔄 Removing trader %s from memory to reload with new exchange config (background)", traderID)
				s.removeTraderForReload(userID, traderID)
			}
			if err := s.traderManager.LoadUserTradersFromStore(s.store, userID); err != nil {
				logger.Infof("⚠️ Failed to reload user traders into memory: %v", err)
				// Don't return error here since exchange config was successfully updated to database
			}
		}()
	}

	logger.Infof("✓ Exchange config updated: %+v", SanitizeExchangeConfigForLog(req.Exchanges))
	c.JSON(http.StatusOK, gin.H{"message": "Exchange configuration updated", "reload": "background"})
}

// handleCreateExchange Create a new exchange account
func (s *Server) handleCreateExchange(c *gin.Context) {
	userID := c.GetString("user_id")
	cfg := config.Get()

	// Read raw request body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var req CreateExchangeRequest

	// Check if transport encryption is enabled
	if !cfg.TransportEncryption {
		// Transport encryption disabled, accept plain JSON
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			logger.Infof("❌ Failed to parse plain JSON request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
	} else {
		// Transport encryption enabled, require encrypted payload
		var encryptedPayload crypto.EncryptedPayload
		if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format, encrypted transmission required"})
			return
		}

		if encryptedPayload.WrappedKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "This endpoint only supports encrypted transmission",
				"code":    "ENCRYPTION_REQUIRED",
				"message": "Encrypted transmission is required for security reasons",
			})
			return
		}

		decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveDataForUser(&encryptedPayload, userID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decrypt data"})
			return
		}

		if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
			return
		}
	}

	// Validate exchange type
	validTypes := map[string]bool{
		"paper": true, "binance": true, "binance_stocks": true, "bybit": true, "okx": true, "bitget": true,
		"hyperliquid": true, "aster": true, "lighter": true, "gate": true, "kucoin": true, "indodax": true,
	}
	if !validTypes[req.ExchangeType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid exchange type: %s", req.ExchangeType)})
		return
	}

	// Create new exchange account
	if err := store.ValidateExchangeEnvironment(req.ExchangeType, req.Testnet); err != nil {
		SafeBadRequest(c, err.Error())
		return
	}
	var id string
	if req.ExchangeType == "paper" {
		paperCfg := store.DefaultPaperConfig()
		if req.PaperConfig != nil {
			paperCfg = *req.PaperConfig
		}
		if err := paperCfg.Validate(); err != nil {
			SafeBadRequest(c, err.Error())
			return
		}
		id, err = s.store.Exchange().CreatePaper(userID, req.AccountName, req.Enabled, paperCfg)
	} else {
		id, err = s.store.Exchange().Create(
			userID, req.ExchangeType, req.AccountName, req.Enabled,
			req.APIKey, req.SecretKey, req.Passphrase, req.Testnet,
			req.HyperliquidWalletAddr, req.HyperliquidUnifiedAcct,
			req.AsterUser, req.AsterSigner, req.AsterPrivateKey,
			req.LighterWalletAddr, req.LighterPrivateKey, req.LighterAPIKeyPrivateKey, req.LighterAPIKeyIndex,
		)
	}
	if err != nil {
		logger.Infof("❌ Failed to create exchange account: %v", err)
		SafeInternalError(c, "Failed to create exchange account", err)
		return
	}

	s.exchangeAccountStateCache.Invalidate(userID)

	logger.Infof("✓ Created exchange account: type=%s, name=%s, id=%s", req.ExchangeType, req.AccountName, id)
	c.JSON(http.StatusOK, gin.H{
		"message": "Exchange account created",
		"id":      id,
	})
}

// handleDeleteExchange Delete an exchange account
func (s *Server) handleDeleteExchange(c *gin.Context) {
	s.traderOpsMu.Lock()
	defer s.traderOpsMu.Unlock()
	userID := c.GetString("user_id")
	exchangeID := c.Param("id")

	if exchangeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Exchange ID is required"})
		return
	}

	// Check if any traders are using this exchange
	traders, err := s.store.Trader().List(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check traders"})
		return
	}

	for _, trader := range traders {
		if trader.ExchangeID == exchangeID {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":       "Cannot delete exchange account that is in use by traders",
				"trader_id":   trader.ID,
				"trader_name": trader.Name,
			})
			return
		}
	}

	// Delete exchange account
	err = s.store.Exchange().Delete(userID, exchangeID)
	if err != nil {
		logger.Infof("❌ Failed to delete exchange account: %v", err)
		SafeInternalError(c, "Failed to delete exchange account", err)
		return
	}

	s.exchangeAccountStateCache.Invalidate(userID)

	logger.Infof("✓ Deleted exchange account: id=%s", exchangeID)
	c.JSON(http.StatusOK, gin.H{"message": "Exchange account deleted"})
}

// handleGetSupportedExchanges Get list of exchanges supported by the system
func (s *Server) handleGetSupportedExchanges(c *gin.Context) {
	// Return static list of supported exchange types
	// Note: ID is empty for supported exchanges (they are templates, not actual accounts)
	supportedExchanges := []SafeExchangeConfig{
		{ExchangeType: "paper", Name: "Paper Futures (模拟盘)", Type: "paper"},
		{ExchangeType: "binance", Name: "Binance Futures", Type: "cex"},
		{ExchangeType: "binance_stocks", Name: "Binance Stocks (US Equity)", Type: "stock"},
		{ExchangeType: "bybit", Name: "Bybit Futures", Type: "cex"},
		{ExchangeType: "bitget", Name: "Bitget Futures", Type: "cex"},
		{ExchangeType: "indodax", Name: "Indodax Spot", Type: "cex"},
		{ExchangeType: "okx", Name: "OKX Futures", Type: "cex"},
		{ExchangeType: "gate", Name: "Gate.io Futures", Type: "cex"},
		{ExchangeType: "kucoin", Name: "KuCoin Futures", Type: "cex"},
		{ExchangeType: "hyperliquid", Name: "Hyperliquid", Type: "dex"},
		{ExchangeType: "aster", Name: "Aster DEX", Type: "dex"},
		{ExchangeType: "lighter", Name: "LIGHTER DEX", Type: "dex"},
	}

	c.JSON(http.StatusOK, supportedExchanges)
}
