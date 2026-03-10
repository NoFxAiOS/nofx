package mcp

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/sha3"
)

const (
	ProviderBlockRunBase   = "blockrun-base"
	DefaultBlockRunBaseURL = "https://blockrun.ai"
	DefaultBlockRunModel = "gpt-5.4"
	BlockRunChatEndpoint   = "/api/v1/chat/completions"
	BaseUSDCContract       = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
	BaseChainID      int64 = 8453
	BaseNetwork            = "eip155:8453"
)

// EIP-712 type hashes for USDC TransferWithAuthorization (ERC-3009)
var (
	eip712DomainTypeHash    = keccak256String("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)")
	transferWithAuthTypeHash = keccak256String("TransferWithAuthorization(address from,address to,uint256 value,uint256 validAfter,uint256 validBefore,bytes32 nonce)")
)

func keccak256String(s string) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte(s))
	return h.Sum(nil)
}

func keccak256Bytes(data ...[]byte) []byte {
	h := sha3.NewLegacyKeccak256()
	for _, b := range data {
		h.Write(b)
	}
	return h.Sum(nil)
}

// BlockRunBaseClient implements AIClient using BlockRun's API with x402 v2 EIP-712 payment signing.
type BlockRunBaseClient struct {
	*Client
	privateKey *ecdsa.PrivateKey
}

// NewBlockRunBaseClient creates a BlockRun Base wallet client (backward compatible).
func NewBlockRunBaseClient() AIClient {
	return NewBlockRunBaseClientWithOptions()
}

// NewBlockRunBaseClientWithOptions creates a BlockRun Base wallet client.
func NewBlockRunBaseClientWithOptions(opts ...ClientOption) AIClient {
	baseOpts := []ClientOption{
		WithProvider(ProviderBlockRunBase),
		WithModel(DefaultBlockRunModel),
		WithBaseURL(DefaultBlockRunBaseURL),
	}
	allOpts := append(baseOpts, opts...)
	baseClient := NewClient(allOpts...).(*Client)
	baseClient.UseFullURL = true
	baseClient.BaseURL = DefaultBlockRunBaseURL + BlockRunChatEndpoint

	c := &BlockRunBaseClient{Client: baseClient}
	baseClient.hooks = c
	return c
}

// SetAPIKey stores the EVM private key (hex, with or without 0x prefix).
// customModel selects the AI model to use (e.g. "claude-sonnet-4.6"); empty means default.
func (c *BlockRunBaseClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	hexKey := strings.TrimPrefix(apiKey, "0x")
	privKey, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		c.logger.Warnf("⚠️  [MCP] BlockRun Base: invalid private key: %v", err)
	} else {
		c.privateKey = privKey
		c.APIKey = apiKey
		addr := crypto.PubkeyToAddress(privKey.PublicKey).Hex()
		c.logger.Infof("🔧 [MCP] BlockRun Base wallet: %s", addr)
	}
	if customModel != "" {
		c.Model = customModel
		c.logger.Infof("🔧 [MCP] BlockRun Base model: %s", customModel)
	} else {
		c.logger.Infof("🔧 [MCP] BlockRun Base model: %s", DefaultBlockRunModel)
	}
}

func (c *BlockRunBaseClient) setAuthHeader(reqHeaders http.Header) {
	// No Bearer token — payment is via x402 signing
}

// call overrides the base call to handle HTTP 402 x402 v2 payment flow.
func (c *BlockRunBaseClient) call(systemPrompt, userPrompt string) (string, error) {
	c.logger.Infof("📡 [BlockRun Base] Request AI Server: %s", c.BaseURL)

	requestBody := c.hooks.buildMCPRequestBody(systemPrompt, userPrompt)
	jsonData, err := c.hooks.marshalRequestBody(requestBody)
	if err != nil {
		return "", err
	}

	url := c.hooks.buildUrl()
	req, err := c.hooks.buildRequest(url, jsonData)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Handle x402 v2 Payment Required
	if resp.StatusCode == http.StatusPaymentRequired {
		paymentHeader := resp.Header.Get("X-Payment-Required")
		if paymentHeader == "" {
			return "", fmt.Errorf("received 402 but no X-Payment-Required header")
		}

		paymentSig, err := c.signPayment(paymentHeader)
		if err != nil {
			return "", fmt.Errorf("failed to sign x402 payment: %w", err)
		}

		req2, err := c.hooks.buildRequest(url, jsonData)
		if err != nil {
			return "", fmt.Errorf("failed to build retry request: %w", err)
		}
		req2.Header.Set("X-Payment", paymentSig)

		resp2, err := c.httpClient.Do(req2)
		if err != nil {
			return "", fmt.Errorf("failed to send payment retry: %w", err)
		}
		defer resp2.Body.Close()

		body2, err := io.ReadAll(resp2.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read payment retry response: %w", err)
		}
		if resp2.StatusCode != http.StatusOK {
			return "", fmt.Errorf("BlockRun payment retry failed (status %d): %s", resp2.StatusCode, string(body2))
		}
		return c.hooks.parseMCPResponse(body2)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("BlockRun API error (status %d): %s", resp.StatusCode, string(body))
	}
	return c.hooks.parseMCPResponse(body)
}

// x402v2PaymentRequired is the structure of the X-Payment-Required header (x402 v2).
type x402v2PaymentRequired struct {
	X402Version int `json:"x402Version"`
	Accepts     []struct {
		Scheme            string            `json:"scheme"`
		Network           string            `json:"network"`
		Amount            string            `json:"amount"`
		Asset             string            `json:"asset"`
		PayTo             string            `json:"payTo"`
		MaxTimeoutSeconds int               `json:"maxTimeoutSeconds"`
		Extra             map[string]string `json:"extra"`
	} `json:"accepts"`
	Resource *struct {
		URL         string `json:"url"`
		Description string `json:"description"`
		MimeType    string `json:"mimeType"`
	} `json:"resource"`
}

// signPayment parses the X-Payment-Required header (x402 v2) and returns a signed X-Payment value.
func (c *BlockRunBaseClient) signPayment(paymentHeaderB64 string) (string, error) {
	if c.privateKey == nil {
		return "", fmt.Errorf("no private key set for BlockRun Base wallet")
	}

	// Decode base64 → JSON
	decoded, err := base64.RawStdEncoding.DecodeString(paymentHeaderB64)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(paymentHeaderB64)
		if err != nil {
			return "", fmt.Errorf("failed to base64-decode payment header: %w", err)
		}
	}

	var req x402v2PaymentRequired
	if err := json.Unmarshal(decoded, &req); err != nil {
		return "", fmt.Errorf("failed to parse x402 v2 payment header: %w", err)
	}

	if len(req.Accepts) == 0 {
		return "", fmt.Errorf("no payment options in x402 response")
	}

	opt := req.Accepts[0]
	recipient := opt.PayTo
	amount := opt.Amount
	network := opt.Network
	asset := opt.Asset
	extra := opt.Extra
	maxTimeout := opt.MaxTimeoutSeconds
	if maxTimeout == 0 {
		maxTimeout = 300
	}

	resourceURL := ""
	resourceDesc := ""
	resourceMime := "application/json"
	if req.Resource != nil {
		resourceURL = req.Resource.URL
		resourceDesc = req.Resource.Description
		resourceMime = req.Resource.MimeType
	}

	// Timestamps: validAfter = now-600 (clock skew), validBefore = now+maxTimeout
	now := time.Now().Unix()
	validAfter := now - 600
	validBefore := now + int64(maxTimeout)

	// Random nonce (bytes32)
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	nonce := "0x" + hex.EncodeToString(nonceBytes)

	// Sender address
	senderAddr := crypto.PubkeyToAddress(c.privateKey.PublicKey).Hex()

	// Build EIP-712 domain separator
	domainName := "USD Coin"
	domainVersion := "2"
	if extra != nil {
		if v, ok := extra["name"]; ok && v != "" {
			domainName = v
		}
		if v, ok := extra["version"]; ok && v != "" {
			domainVersion = v
		}
	}

	domainSeparator, err := buildDomainSeparatorDynamic(domainName, domainVersion, network, asset)
	if err != nil {
		return "", fmt.Errorf("failed to build domain separator: %w", err)
	}

	// Build struct hash
	amountBig, err := parseBigInt(amount)
	if err != nil {
		return "", fmt.Errorf("invalid amount: %w", err)
	}

	structHash, err := buildTransferWithAuthHashDynamic(senderAddr, recipient, amountBig, validAfter, validBefore, nonce)
	if err != nil {
		return "", fmt.Errorf("failed to build struct hash: %w", err)
	}

	// EIP-712 digest
	digest := make([]byte, 0, 66)
	digest = append(digest, 0x19, 0x01)
	digest = append(digest, domainSeparator...)
	digest = append(digest, structHash...)
	hash := keccak256Bytes(digest)

	// Sign with secp256k1
	sig, err := crypto.Sign(hash, c.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign: %w", err)
	}
	// Adjust V: go-ethereum returns 0/1, EIP-712 expects 27/28
	if sig[64] < 27 {
		sig[64] += 27
	}

	sigHex := "0x" + hex.EncodeToString(sig)

	// Build x402 v2 payment payload
	paymentData := map[string]interface{}{
		"x402Version": 2,
		"resource": map[string]string{
			"url":         resourceURL,
			"description": resourceDesc,
			"mimeType":    resourceMime,
		},
		"accepted": map[string]interface{}{
			"scheme":            "exact",
			"network":           network,
			"amount":            amount,
			"asset":             asset,
			"payTo":             recipient,
			"maxTimeoutSeconds": maxTimeout,
			"extra":             extra,
		},
		"payload": map[string]interface{}{
			"signature": sigHex,
			"authorization": map[string]string{
				"from":        senderAddr,
				"to":          recipient,
				"value":       amount,
				"validAfter":  fmt.Sprintf("%d", validAfter),
				"validBefore": fmt.Sprintf("%d", validBefore),
				"nonce":       nonce,
			},
		},
		"extensions": map[string]interface{}{},
	}

	resultJSON, err := json.Marshal(paymentData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payment result: %w", err)
	}

	return base64.StdEncoding.EncodeToString(resultJSON), nil
}

// buildDomainSeparatorDynamic builds the EIP-712 domain separator using runtime values.
func buildDomainSeparatorDynamic(name, version, network, asset string) ([]byte, error) {
	// Extract chain ID from network string like "eip155:8453"
	chainID := new(big.Int).SetInt64(BaseChainID)
	if strings.HasPrefix(network, "eip155:") {
		parts := strings.SplitN(network, ":", 2)
		if len(parts) == 2 {
			if n, ok := new(big.Int).SetString(parts[1], 10); ok {
				chainID = n
			}
		}
	}

	contractAddr, err := hex.DecodeString(strings.TrimPrefix(asset, "0x"))
	if err != nil {
		return nil, fmt.Errorf("invalid contract address: %w", err)
	}

	nameHash := keccak256String(name)
	versionHash := keccak256String(version)

	encoded := make([]byte, 0, 5*32)
	encoded = append(encoded, leftPad32(eip712DomainTypeHash)...)
	encoded = append(encoded, leftPad32(nameHash)...)
	encoded = append(encoded, leftPad32(versionHash)...)
	encoded = append(encoded, leftPad32(chainID.Bytes())...)
	addrPadded := make([]byte, 32)
	copy(addrPadded[32-len(contractAddr):], contractAddr)
	encoded = append(encoded, addrPadded...)

	return keccak256Bytes(encoded), nil
}

// buildTransferWithAuthHashDynamic builds the struct hash for TransferWithAuthorization.
func buildTransferWithAuthHashDynamic(from, to string, value *big.Int, validAfter, validBefore int64, nonce string) ([]byte, error) {
	fromBytes, err := hexToAddress(from)
	if err != nil {
		return nil, fmt.Errorf("invalid from address: %w", err)
	}
	toBytes, err := hexToAddress(to)
	if err != nil {
		return nil, fmt.Errorf("invalid to address: %w", err)
	}
	nonceBytes, err := hexToBytes32(nonce)
	if err != nil {
		return nil, fmt.Errorf("invalid nonce: %w", err)
	}

	validAfterBig := new(big.Int).SetInt64(validAfter)
	validBeforeBig := new(big.Int).SetInt64(validBefore)

	encoded := make([]byte, 0, 7*32)
	encoded = append(encoded, leftPad32(transferWithAuthTypeHash)...)
	encoded = append(encoded, leftPad32(fromBytes)...)
	encoded = append(encoded, leftPad32(toBytes)...)
	encoded = append(encoded, leftPad32(value.Bytes())...)
	encoded = append(encoded, leftPad32(validAfterBig.Bytes())...)
	encoded = append(encoded, leftPad32(validBeforeBig.Bytes())...)
	encoded = append(encoded, leftPad32(nonceBytes)...)

	return keccak256Bytes(encoded), nil
}

func hexToAddress(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(b) != 20 {
		return nil, fmt.Errorf("address must be 20 bytes, got %d", len(b))
	}
	return b, nil
}

func hexToBytes32(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(b) > 32 {
		return nil, fmt.Errorf("nonce too long: %d bytes", len(b))
	}
	return b, nil
}

func parseBigInt(s string) (*big.Int, error) {
	s = strings.TrimPrefix(s, "0x")
	n := new(big.Int)
	if _, ok := n.SetString(s, 16); ok {
		return n, nil
	}
	if _, ok := n.SetString(s, 10); ok {
		return n, nil
	}
	return nil, fmt.Errorf("cannot parse big.Int from %q", s)
}

// leftPad32 pads a byte slice to 32 bytes on the left (ABI encoding).
func leftPad32(b []byte) []byte {
	if len(b) >= 32 {
		return b[:32]
	}
	padded := make([]byte, 32)
	copy(padded[32-len(b):], b)
	return padded
}

// buildUrl returns the full BlockRun endpoint URL.
func (c *BlockRunBaseClient) buildUrl() string {
	return DefaultBlockRunBaseURL + BlockRunChatEndpoint
}

// buildRequest creates the HTTP request without an Authorization header.
func (c *BlockRunBaseClient) buildRequest(url string, jsonData []byte) (*http.Request, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("fail to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}
