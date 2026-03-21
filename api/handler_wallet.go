package api

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"nofx/wallet"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
)

type walletValidateRequest struct {
	PrivateKey string `json:"private_key"`
}

type walletValidateResponse struct {
	Valid        bool   `json:"valid"`
	Address      string `json:"address,omitempty"`
	BalanceUSDC  string `json:"balance_usdc,omitempty"`
	Claw402Status string `json:"claw402_status"` // "ok", "unreachable", "error"
	Error        string `json:"error,omitempty"`
}



func (s *Server) handleWalletValidate(c *gin.Context) {
	var req walletValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, walletValidateResponse{
			Valid: false,
			Error: "invalid request body",
		})
		return
	}

	pk := req.PrivateKey

	// Validate format
	if !strings.HasPrefix(pk, "0x") {
		c.JSON(http.StatusOK, walletValidateResponse{
			Valid: false,
			Error: "missing 0x prefix",
		})
		return
	}

	if len(pk) != 66 {
		c.JSON(http.StatusOK, walletValidateResponse{
			Valid: false,
			Error: fmt.Sprintf("should be 66 characters, got %d", len(pk)),
		})
		return
	}

	hexPart := pk[2:]
	if _, err := hex.DecodeString(hexPart); err != nil {
		c.JSON(http.StatusOK, walletValidateResponse{
			Valid: false,
			Error: "contains invalid hex characters",
		})
		return
	}

	// Derive address
	privateKey, err := crypto.HexToECDSA(hexPart)
	if err != nil {
		c.JSON(http.StatusOK, walletValidateResponse{
			Valid: false,
			Error: "invalid private key",
		})
		return
	}

	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	addrHex := address.Hex()

	// Query USDC balance (async-ish, but sequential for simplicity)
	balanceStr := wallet.QueryUSDCBalanceStr(addrHex)

	// Check claw402 health
	claw402Status := checkClaw402Health()

	c.JSON(http.StatusOK, walletValidateResponse{
		Valid:        true,
		Address:      addrHex,
		BalanceUSDC:  balanceStr,
		Claw402Status: claw402Status,
	})
}



type walletGenerateResponse struct {
	Address    string `json:"address"`
	PrivateKey string `json:"private_key"`
}

func (s *Server) handleWalletGenerate(c *gin.Context) {
	// Generate new EVM wallet
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate wallet"})
		return
	}

	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	privKeyHex := "0x" + hex.EncodeToString(crypto.FromECDSA(privateKey))

	c.JSON(http.StatusOK, walletGenerateResponse{
		Address:    address.Hex(),
		PrivateKey: privKeyHex,
	})
}

func checkClaw402Health() string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://claw402.ai/health")
	if err != nil {
		return "unreachable"
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "ok"
	}
	return "error"
}
