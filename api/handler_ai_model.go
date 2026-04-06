package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"nofx/config"
	"nofx/crypto"
	"nofx/logger"
	"nofx/mcp"
	"nofx/security"
	"nofx/wallet"

	"github.com/gin-gonic/gin"
)

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
	WalletAddress   string `json:"walletAddress,omitempty"`
	BalanceUSDC     string `json:"balanceUsdc,omitempty"`
}

type UpdateModelConfigRequest struct {
	Models map[string]struct {
		Enabled         bool   `json:"enabled"`
		APIKey          string `json:"api_key"`
		CustomAPIURL    string `json:"custom_api_url"`
		CustomModelName string `json:"custom_model_name"`
	} `json:"models"`
}

// handleGetModelConfigs Get AI model configurations
func (s *Server) handleGetModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	logger.Infof("🔍 Querying AI model configs for user %s", userID)
	models, err := s.store.AIModel().List(userID)
	if err != nil {
		logger.Infof("❌ Failed to get AI model configs: %v", err)
		SafeInternalError(c, "Failed to get AI model configs", err)
		return
	}

	// If no models in database, return default models
	if len(models) == 0 {
		logger.Infof("⚠️ No AI models in database, returning defaults")
		defaultModels := []SafeModelConfig{
			{ID: "deepseek", Name: "DeepSeek AI", Provider: "deepseek", Enabled: false},
			{ID: "qwen", Name: "Qwen AI", Provider: "qwen", Enabled: false},
			{ID: "openai", Name: "OpenAI", Provider: "openai", Enabled: false},
			{ID: "claude", Name: "Claude AI", Provider: "claude", Enabled: false},
			{ID: "gemini", Name: "Gemini AI", Provider: "gemini", Enabled: false},
			{ID: "grok", Name: "Grok AI", Provider: "grok", Enabled: false},
			{ID: "kimi", Name: "Kimi AI", Provider: "kimi", Enabled: false},
			{ID: "minimax", Name: "MiniMax AI", Provider: "minimax", Enabled: false},
			{ID: "ollama", Name: "Ollama AI", Provider: "ollama", Enabled: false},
		}
		c.JSON(http.StatusOK, defaultModels)
		return
	}

	logger.Infof("✅ Found %d AI model configs", len(models))

	// Convert to safe response structure, remove sensitive information
	safeModels := make([]SafeModelConfig, len(models))
	for i, model := range models {
		safeModel := SafeModelConfig{
			ID:              model.ID,
			Name:            model.Name,
			Provider:        model.Provider,
			Enabled:         model.Enabled,
			CustomAPIURL:    model.CustomAPIURL,
			CustomModelName: model.CustomModelName,
		}

		if model.Provider == "claw402" {
			if privateKey := strings.TrimSpace(model.APIKey.String()); privateKey != "" {
				if walletAddress, addrErr := walletAddressFromPrivateKey(privateKey); addrErr == nil {
					safeModel.WalletAddress = walletAddress
					safeModel.BalanceUSDC = wallet.QueryUSDCBalanceStr(walletAddress)
				} else {
					logger.Warnf("⚠️ Failed to derive claw402 wallet address for model %s: %v", model.ID, addrErr)
				}
			}
		}

		safeModels[i] = safeModel
	}

	c.JSON(http.StatusOK, safeModels)
}

// handleUpdateModelConfigs Update AI model configurations (supports both encrypted and plain text based on config)
func (s *Server) handleUpdateModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	cfg := config.Get()

	// Read raw request body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var req UpdateModelConfigRequest

	// Check if transport encryption is enabled
	if !cfg.TransportEncryption {
		// Transport encryption disabled, accept plain JSON
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			logger.Infof("❌ Failed to parse plain JSON request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
		logger.Infof("📝 Received plain text model config (UserID: %s)", userID)
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
		decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
		if err != nil {
			logger.Infof("❌ Failed to decrypt model config (UserID: %s): %v", userID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decrypt data"})
			return
		}

		// Parse decrypted data
		if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
			logger.Infof("❌ Failed to parse decrypted data: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
			return
		}
		logger.Infof("🔓 Decrypted model config data (UserID: %s)", userID)
	}

	// Update each model's configuration and track traders that need reload
	tradersToReload := make(map[string]bool)
	for modelID, modelData := range req.Models {
		// SSRF protection: validate custom_api_url before storing
		if modelData.CustomAPIURL != "" {
			cleanURL := strings.TrimSuffix(modelData.CustomAPIURL, "#")
			if err := security.ValidateURL(cleanURL); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid custom_api_url for model %s: %s", modelID, err.Error())})
				return
			}
		}

		// Find traders using this AI model BEFORE updating
		traders, _ := s.store.Trader().ListByAIModelID(userID, modelID)
		for _, t := range traders {
			tradersToReload[t.ID] = true
		}

		err := s.store.AIModel().Update(userID, modelID, modelData.Enabled, modelData.APIKey, modelData.CustomAPIURL, modelData.CustomModelName)
		if err != nil {
			SafeInternalError(c, fmt.Sprintf("Update model %s", modelID), err)
			return
		}
	}

	// Remove affected traders from memory BEFORE reloading to pick up new config
	for traderID := range tradersToReload {
		logger.Infof("🔄 Removing trader %s from memory to reload with new AI model config", traderID)
		s.traderManager.RemoveTrader(traderID)
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

// handleGetSupportedModels Get list of AI models supported by the system
func (s *Server) handleGetSupportedModels(c *gin.Context) {
	// Return static list of supported AI models with default versions
	supportedModels := []map[string]interface{}{
		{"id": "deepseek", "name": "DeepSeek", "provider": "deepseek", "defaultModel": "deepseek-chat"},
		{"id": "qwen", "name": "Qwen", "provider": "qwen", "defaultModel": "qwen3-max"},
		{"id": "openai", "name": "OpenAI", "provider": "openai", "defaultModel": "gpt-5.1"},
		{"id": "claude", "name": "Claude", "provider": "claude", "defaultModel": "claude-opus-4-6"},
		{"id": "gemini", "name": "Google Gemini", "provider": "gemini", "defaultModel": "gemini-3-pro-preview"},
		{"id": "grok", "name": "Grok (xAI)", "provider": "grok", "defaultModel": "grok-3-latest"},
		{"id": "kimi", "name": "Kimi (Moonshot)", "provider": "kimi", "defaultModel": "moonshot-v1-auto"},
		{"id": "minimax", "name": "MiniMax", "provider": "minimax", "defaultModel": "MiniMax-M2.7"},
		{"id": "ollama", "name": "Ollama (Local)", "provider": "ollama", "defaultModel": "llama3.1"},
		{"id": "claw402", "name": "Claw402 (Base USDC)", "provider": "claw402", "defaultModel": "glm-5"},
	}

	c.JSON(http.StatusOK, supportedModels)
}

// TestModelRequest request body for testing an AI model connection
type TestModelRequest struct {
	Provider        string `json:"provider"`
	APIKey          string `json:"api_key"`
	CustomAPIURL    string `json:"custom_api_url"`
	CustomModelName string `json:"custom_model_name"`
}

// TestModelResponse response for test model endpoint
type TestModelResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	LatencyMs int64  `json:"latency_ms"`
}

// handleTestModel Test AI model connection with provided credentials
func (s *Server) handleTestModel(c *gin.Context) {
	cfg := config.Get()

	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var req TestModelRequest

	if !cfg.TransportEncryption {
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
	} else {
		var encryptedPayload crypto.EncryptedPayload
		if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil || encryptedPayload.WrappedKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Encrypted transmission required"})
			return
		}
		decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decrypt data"})
			return
		}
		if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
			return
		}
	}

	if req.Provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider is required"})
		return
	}
	if req.APIKey == "" && req.Provider != mcp.ProviderOllama {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API key is required"})
		return
	}

	if req.CustomAPIURL != "" {
		cleanURL := strings.TrimSuffix(req.CustomAPIURL, "#")
		if err := security.ValidateURL(cleanURL); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid custom API URL: %s", err.Error())})
			return
		}
	}

	client := mcp.NewAIClientByProvider(
		req.Provider,
		mcp.WithTimeout(15*time.Second),
		mcp.WithMaxRetries(1),
		mcp.WithMaxTokens(10),
	)
	if client == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported provider: %s", req.Provider)})
		return
	}

	client.SetAPIKey(req.APIKey, req.CustomAPIURL, req.CustomModelName)

	start := time.Now()
	_, err = client.CallWithMessages("", "Say hi")
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		errMsg := err.Error()
		userMsg := "Connection failed"
		switch {
		case strings.Contains(errMsg, "401") || strings.Contains(errMsg, "403") || strings.Contains(errMsg, "Unauthorized") || strings.Contains(errMsg, "authentication"):
			userMsg = "Invalid API key"
		case strings.Contains(errMsg, "404"):
			userMsg = "Model not found"
		case strings.Contains(errMsg, "429"):
			userMsg = "Rate limited"
		case strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "Timeout"):
			userMsg = "Request timed out"
		case strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "no such host"):
			userMsg = "Cannot reach API endpoint"
		}
		logger.Infof("❌ Model test failed for %s: %v", req.Provider, err)
		c.JSON(http.StatusOK, TestModelResponse{Success: false, Message: userMsg, LatencyMs: latencyMs})
		return
	}

	c.JSON(http.StatusOK, TestModelResponse{Success: true, Message: "Connection successful", LatencyMs: latencyMs})
}
