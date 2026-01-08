package api

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"log"
	"math/big"
	"net/http"
	"nofx/config"
	"nofx/crypto"
	"strings"

	"github.com/gin-gonic/gin"
)

// CryptoHandler Encryption API handler
type CryptoHandler struct {
	cryptoService *crypto.CryptoService
}

// NewCryptoHandler Creates encryption handler
func NewCryptoHandler(cryptoService *crypto.CryptoService) *CryptoHandler {
	return &CryptoHandler{
		cryptoService: cryptoService,
	}
}

// ==================== Crypto Config Endpoint ====================

// HandleGetCryptoConfig Get crypto configuration
func (h *CryptoHandler) HandleGetCryptoConfig(c *gin.Context) {
	cfg := config.Get()
	c.JSON(http.StatusOK, gin.H{
		"transport_encryption": cfg.TransportEncryption,
	})
}

// ==================== Public Key Endpoint ====================

// HandleGetPublicKey Get server public key
func (h *CryptoHandler) HandleGetPublicKey(c *gin.Context) {
	cfg := config.Get()
	if !cfg.TransportEncryption {
		c.JSON(http.StatusOK, gin.H{
			"public_key":           "",
			"algorithm":            "",
			"transport_encryption": false,
		})
		return
	}

	publicKey := h.cryptoService.GetPublicKeyPEM()
	c.JSON(http.StatusOK, gin.H{
		"public_key":           publicKey,
		"algorithm":            "RSA-OAEP-2048",
		"transport_encryption": true,
	})
}

// ==================== Encrypted Data Decryption Endpoint ====================

// HandleDecryptSensitiveData Decrypt encrypted data sent from client
func (h *CryptoHandler) HandleDecryptSensitiveData(c *gin.Context) {
	var payload crypto.EncryptedPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Decrypt
	decrypted, err := h.cryptoService.DecryptSensitiveData(&payload)
	if err != nil {
		log.Printf("❌ Decryption failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Decryption failed"})
		return
	}

	c.JSON(http.StatusOK, map[string]string{
		"plaintext": decrypted,
	})
}

// ==================== Audit Log Query Endpoint ====================

// Audit log functionality removed, not needed in current simplified implementation

// ==================== Utility Functions ====================

// isValidPrivateKey Validate private key format
func isValidPrivateKey(key string) bool {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return false
	}

	if isValidEVMPrivateKey(trimmed) {
		return true
	}
	if isValidSolanaPrivateKey(trimmed) {
		return true
	}
	// TODO: Add validation for other chains
	return false
}

func isValidEVMPrivateKey(key string) bool {
	// EVM private key: 64 hex characters (optional 0x prefix)
	trimmed := key
	if len(trimmed) == 66 && strings.HasPrefix(trimmed, "0x") {
		trimmed = trimmed[2:]
	}
	return len(trimmed) == 64 && isHexString(trimmed)
}

func isValidSolanaPrivateKey(key string) bool {
	keyBytes, ok := parseSolanaPrivateKeyBytes(key)
	if !ok {
		return false
	}

	switch len(keyBytes) {
	case ed25519.SeedSize:
		return true
	case ed25519.PrivateKeySize:
		seed := keyBytes[:ed25519.SeedSize]
		pub := keyBytes[ed25519.SeedSize:]
		derived := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
		return bytes.Equal(derived, pub)
	default:
		return false
	}
}

func parseSolanaPrivateKeyBytes(key string) ([]byte, bool) {
	trimmed := strings.TrimSpace(key)
	if strings.HasPrefix(trimmed, "[") {
		return parseSolanaJSONKey(trimmed)
	}

	decoded, ok := decodeBase58(trimmed)
	if !ok {
		return nil, false
	}
	if len(decoded) != ed25519.SeedSize && len(decoded) != ed25519.PrivateKeySize {
		return nil, false
	}
	return decoded, true
}

func parseSolanaJSONKey(key string) ([]byte, bool) {
	var values []int
	if err := json.Unmarshal([]byte(key), &values); err != nil {
		return nil, false
	}
	if len(values) != ed25519.SeedSize && len(values) != ed25519.PrivateKeySize {
		return nil, false
	}

	out := make([]byte, len(values))
	for i, v := range values {
		if v < 0 || v > 255 {
			return nil, false
		}
		out[i] = byte(v)
	}
	return out, true
}

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

func decodeBase58(input string) ([]byte, bool) {
	if input == "" {
		return nil, false
	}

	base := big.NewInt(58)
	num := big.NewInt(0)
	for i := 0; i < len(input); i++ {
		idx := strings.IndexByte(base58Alphabet, input[i])
		if idx < 0 {
			return nil, false
		}
		num.Mul(num, base)
		num.Add(num, big.NewInt(int64(idx)))
	}

	decoded := num.Bytes()
	leadingZeros := 0
	for leadingZeros < len(input) && input[leadingZeros] == '1' {
		leadingZeros++
	}
	if leadingZeros > 0 {
		decoded = append(make([]byte, leadingZeros), decoded...)
	}

	return decoded, true
}

func isHexString(s string) bool {
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}
