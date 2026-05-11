package api

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"nofx/logger"
	"nofx/mcp/payment"
	"nofx/wallet"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
)

type beginnerOnboardingResponse struct {
	Address           string `json:"address"`
	PrivateKey        string `json:"private_key"`
	Chain             string `json:"chain"`
	Asset             string `json:"asset"`
	Provider          string `json:"provider"`
	DefaultModel      string `json:"default_model"`
	ConfiguredModelID string `json:"configured_model_id"`
	BalanceUSDC       string `json:"balance_usdc"`
	EnvSaved          bool   `json:"env_saved"`
	EnvPath           string `json:"env_path,omitempty"`
	ReusedExisting    bool   `json:"reused_existing"`
	EnvWarning        string `json:"env_warning,omitempty"`
}

type currentBeginnerWalletResponse struct {
	Found         bool   `json:"found"`
	Address       string `json:"address,omitempty"`
	BalanceUSDC   string `json:"balance_usdc,omitempty"`
	Source        string `json:"source,omitempty"`
	Claw402Status string `json:"claw402_status"`
}

func (s *Server) handleBeginnerOnboarding(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}

	privateKey, address, configuredModelID, reusedExisting, err := s.resolveBeginnerWallet(userID)
	if err != nil {
		logger.Errorf("Failed to resolve beginner wallet for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare beginner wallet"})
		return
	}

	if !reusedExisting {
		if err := s.store.AIModel().Update(userID, "claw402", true, privateKey, "", payment.DefaultClaw402Model); err != nil {
			logger.Errorf("Failed to save beginner claw402 config for user %s: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save beginner model configuration"})
			return
		}

		configuredModelID, err = s.findConfiguredClaw402ModelID(userID)
		if err != nil {
			logger.Warnf("Could not resolve configured claw402 model id for user %s: %v", userID, err)
		}
	}

	os.Setenv("CLAW402_WALLET_KEY", privateKey)
	os.Setenv("CLAW402_WALLET_ADDRESS", address)
	os.Setenv("CLAW402_DEFAULT_MODEL", payment.DefaultClaw402Model)

	envSaved, envPath, envErr := persistBeginnerWalletEnv(privateKey, address)
	resp := beginnerOnboardingResponse{
		Address:           address,
		PrivateKey:        privateKey,
		Chain:             "base",
		Asset:             "USDC",
		Provider:          "claw402",
		DefaultModel:      payment.DefaultClaw402Model,
		ConfiguredModelID: configuredModelID,
		BalanceUSDC:       wallet.QueryUSDCBalanceStr(address),
		EnvSaved:          envSaved,
		EnvPath:           envPath,
		ReusedExisting:    reusedExisting,
	}
	if envErr != nil {
		resp.EnvWarning = envErr.Error()
		logger.Warnf("Beginner wallet env persistence warning for user %s: %v", userID, envErr)
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleCurrentBeginnerWallet(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	claw402Status := checkClaw402Health()

	models, err := s.store.AIModel().List(userID)
	if err != nil {
		logger.Errorf("Failed to load current beginner wallet for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load current wallet"})
		return
	}

	for _, model := range models {
		if model == nil || model.Provider != "claw402" {
			continue
		}

		privateKey := strings.TrimSpace(model.APIKey.String())
		if privateKey == "" {
			continue
		}

		address, addrErr := walletAddressFromPrivateKey(privateKey)
		if addrErr != nil {
			logger.Warnf("Failed to derive current beginner wallet for user %s: %v", userID, addrErr)
			continue
		}

		c.JSON(http.StatusOK, currentBeginnerWalletResponse{
			Found:         true,
			Address:       address,
			BalanceUSDC:   wallet.QueryUSDCBalanceStr(address),
			Source:        "model",
			Claw402Status: claw402Status,
		})
		return
	}

	address := strings.TrimSpace(os.Getenv("CLAW402_WALLET_ADDRESS"))
	if address != "" {
		c.JSON(http.StatusOK, currentBeginnerWalletResponse{
			Found:         true,
			Address:       address,
			BalanceUSDC:   wallet.QueryUSDCBalanceStr(address),
			Source:        "env",
			Claw402Status: claw402Status,
		})
		return
	}

	c.JSON(http.StatusOK, currentBeginnerWalletResponse{
		Found:         false,
		Claw402Status: claw402Status,
	})
}

func (s *Server) resolveBeginnerWallet(userID string) (privateKey string, address string, configuredModelID string, reused bool, err error) {
	// 1. Check if current user already has a claw402 wallet
	models, err := s.store.AIModel().List(userID)
	if err != nil {
		return "", "", "", false, err
	}

	for _, model := range models {
		if model == nil || model.Provider != "claw402" {
			continue
		}
		existingKey := strings.TrimSpace(model.APIKey.String())
		if existingKey == "" {
			continue
		}

		addr, addrErr := walletAddressFromPrivateKey(existingKey)
		if addrErr != nil {
			logger.Warnf("Existing claw402 key for user %s is invalid, regenerating: %v", userID, addrErr)
			break
		}

		return existingKey, addr, model.ID, true, nil
	}

	// 2. Check for orphan claw402 wallet from a previous account (e.g. after account reset).
	//    Adopt it to preserve funds.
	orphan, orphanErr := s.store.AIModel().FindOrphanClaw402()
	if orphanErr == nil && orphan != nil {
		existingKey := strings.TrimSpace(orphan.APIKey.String())
		if existingKey != "" {
			addr, addrErr := walletAddressFromPrivateKey(existingKey)
			if addrErr == nil {
				if adoptErr := s.store.AIModel().AdoptModel(orphan.ID, userID); adoptErr != nil {
					logger.Warnf("Failed to adopt orphan claw402 wallet for user %s: %v", userID, adoptErr)
				} else {
					logger.Infof("✓ Adopted orphan claw402 wallet %s for new user %s (address: %s)", orphan.ID, userID, addr)
					return existingKey, addr, orphan.ID, true, nil
				}
			}
		}
	}

	// 3. No existing wallet found — generate a new one
	privateKeyObj, genErr := gethcrypto.GenerateKey()
	if genErr != nil {
		return "", "", "", false, genErr
	}

	addr := gethcrypto.PubkeyToAddress(privateKeyObj.PublicKey)
	keyHex := "0x" + hex.EncodeToString(gethcrypto.FromECDSA(privateKeyObj))
	return keyHex, addr.Hex(), "", false, nil
}

func (s *Server) findConfiguredClaw402ModelID(userID string) (string, error) {
	models, err := s.store.AIModel().List(userID)
	if err != nil {
		return "", err
	}

	for _, model := range models {
		if model != nil && model.Provider == "claw402" {
			return model.ID, nil
		}
	}

	return "", fmt.Errorf("claw402 model not found")
}

func walletAddressFromPrivateKey(privateKey string) (string, error) {
	key := strings.TrimSpace(privateKey)
	if !strings.HasPrefix(key, "0x") {
		return "", fmt.Errorf("private key must start with 0x")
	}
	if len(key) != 66 {
		return "", fmt.Errorf("private key must be 66 characters")
	}

	privateKeyObj, err := gethcrypto.HexToECDSA(strings.TrimPrefix(key, "0x"))
	if err != nil {
		return "", err
	}

	return gethcrypto.PubkeyToAddress(privateKeyObj.PublicKey).Hex(), nil
}

func persistBeginnerWalletEnv(privateKey string, address string) (bool, string, error) {
	paths := uniqueEnvPaths([]string{
		".env",
		filepath.Join(".", ".env"),
		"/app/.env",
	})

	var lastErr error
	for _, path := range paths {
		if path == "" {
			continue
		}

		if err := upsertEnvFile(path, map[string]string{
			"CLAW402_WALLET_KEY":     privateKey,
			"CLAW402_WALLET_ADDRESS": address,
			"CLAW402_DEFAULT_MODEL":  payment.DefaultClaw402Model,
		}); err != nil {
			lastErr = err
			continue
		}

		return true, path, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no writable .env path found")
	}
	return false, "", lastErr
}

func uniqueEnvPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	return result
}

func upsertEnvFile(path string, values map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	existingLines := make([]string, 0)
	if file, err := os.Open(path); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			existingLines = append(existingLines, scanner.Text())
		}
		file.Close()
		if err := scanner.Err(); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	remaining := make(map[string]string, len(values))
	for key, value := range values {
		remaining[key] = value
	}

	updatedLines := make([]string, 0, len(existingLines)+len(values))
	for _, line := range existingLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.Contains(line, "=") {
			updatedLines = append(updatedLines, line)
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		value, ok := remaining[key]
		if !ok {
			updatedLines = append(updatedLines, line)
			continue
		}

		updatedLines = append(updatedLines, fmt.Sprintf("%s=%s", key, value))
		delete(remaining, key)
	}

	for key, value := range remaining {
		updatedLines = append(updatedLines, fmt.Sprintf("%s=%s", key, value))
	}

	content := strings.Join(updatedLines, "\n")
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return err
	}

	return nil
}
