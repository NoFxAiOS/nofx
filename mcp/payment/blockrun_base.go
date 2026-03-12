package payment

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/sha3"

	"nofx/mcp"
)

const (
	DefaultBlockRunBaseURL = "https://blockrun.ai"
	DefaultBlockRunModel   = "gpt-5.4"
	BlockRunChatEndpoint   = "/api/v1/chat/completions"
	BaseUSDCContract       = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
	BaseChainID      int64 = 8453
	BaseNetwork            = "eip155:8453"
)

// EIP-712 type hashes for USDC TransferWithAuthorization (ERC-3009)
var (
	eip712DomainTypeHash     = keccak256String("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)")
	transferWithAuthTypeHash = keccak256String("TransferWithAuthorization(address from,address to,uint256 value,uint256 validAfter,uint256 validBefore,bytes32 nonce)")
)

func init() {
	mcp.RegisterProvider(mcp.ProviderBlockRunBase, func(opts ...mcp.ClientOption) mcp.AIClient {
		return NewBlockRunBaseClientWithOptions(opts...)
	})
}

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
	*mcp.Client
	privateKey *ecdsa.PrivateKey
}

func (c *BlockRunBaseClient) BaseClient() *mcp.Client { return c.Client }

// NewBlockRunBaseClient creates a BlockRun Base wallet client (backward compatible).
func NewBlockRunBaseClient() mcp.AIClient {
	return NewBlockRunBaseClientWithOptions()
}

// NewBlockRunBaseClientWithOptions creates a BlockRun Base wallet client.
func NewBlockRunBaseClientWithOptions(opts ...mcp.ClientOption) mcp.AIClient {
	baseOpts := []mcp.ClientOption{
		mcp.WithProvider(mcp.ProviderBlockRunBase),
		mcp.WithModel(DefaultBlockRunModel),
		mcp.WithBaseURL(DefaultBlockRunBaseURL),
		mcp.WithTimeout(X402Timeout),
	}
	allOpts := append(baseOpts, opts...)
	baseClient := mcp.NewClient(allOpts...).(*mcp.Client)
	baseClient.UseFullURL = true
	baseClient.BaseURL = DefaultBlockRunBaseURL + BlockRunChatEndpoint

	c := &BlockRunBaseClient{Client: baseClient}
	baseClient.Hooks = c
	return c
}

// SetAPIKey stores the EVM private key (hex, with or without 0x prefix).
func (c *BlockRunBaseClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	hexKey := strings.TrimPrefix(apiKey, "0x")
	privKey, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		c.Log.Warnf("⚠️  [MCP] BlockRun Base: invalid private key: %v", err)
	} else {
		c.privateKey = privKey
		c.APIKey = apiKey
		addr := crypto.PubkeyToAddress(privKey.PublicKey).Hex()
		c.Log.Infof("🔧 [MCP] BlockRun Base wallet: %s", addr)
	}
	if customModel != "" {
		c.Model = customModel
		c.Log.Infof("🔧 [MCP] BlockRun Base model: %s", customModel)
	} else {
		c.Log.Infof("🔧 [MCP] BlockRun Base model: %s", DefaultBlockRunModel)
	}
}

func (c *BlockRunBaseClient) SetAuthHeader(h http.Header) { X402SetAuthHeader(h) }

func (c *BlockRunBaseClient) Call(systemPrompt, userPrompt string) (string, error) {
	return X402Call(c.Client, c.signPayment, "BlockRun Base", systemPrompt, userPrompt)
}

func (c *BlockRunBaseClient) CallWithRequestFull(req *mcp.Request) (*mcp.LLMResponse, error) {
	return X402CallFull(c.Client, c.signPayment, "BlockRun Base", req)
}

// signPayment parses the Payment-Required header (x402 v2) and returns a signed payment value.
func (c *BlockRunBaseClient) signPayment(paymentHeaderB64 string) (string, error) {
	return SignBasePaymentHeader(c.privateKey, paymentHeaderB64, "BlockRun Base")
}

// SignX402Payment is the shared EIP-712 signing logic for x402 v2 on Base USDC.
// Used by both BlockRunBaseClient and Claw402Client.
func SignX402Payment(privateKey *ecdsa.PrivateKey, senderAddr string, opt X402AcceptOption, resource *X402Resource) (string, error) {
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
	if resource != nil {
		resourceURL = resource.URL
		resourceDesc = resource.Description
		resourceMime = resource.MimeType
	}

	now := time.Now().Unix()
	validAfter := int64(0)
	validBefore := now + int64(maxTimeout)

	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	nonce := "0x" + hex.EncodeToString(nonceBytes)

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

	amountBig, err := parseBigInt(amount)
	if err != nil {
		return "", fmt.Errorf("invalid amount: %w", err)
	}

	structHash, err := buildTransferWithAuthHashDynamic(senderAddr, recipient, amountBig, validAfter, validBefore, nonce)
	if err != nil {
		return "", fmt.Errorf("failed to build struct hash: %w", err)
	}

	digest := make([]byte, 0, 66)
	digest = append(digest, 0x19, 0x01)
	digest = append(digest, domainSeparator...)
	digest = append(digest, structHash...)
	hash := keccak256Bytes(digest)

	sig, err := crypto.Sign(hash, privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign: %w", err)
	}
	if sig[64] < 27 {
		sig[64] += 27
	}

	sigHex := "0x" + hex.EncodeToString(sig)

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
	n := new(big.Int)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		if _, ok := n.SetString(s[2:], 16); ok {
			return n, nil
		}
		return nil, fmt.Errorf("cannot parse hex big.Int from %q", s)
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

// BuildUrl returns the full BlockRun endpoint URL.
func (c *BlockRunBaseClient) BuildUrl() string {
	return DefaultBlockRunBaseURL + BlockRunChatEndpoint
}

func (c *BlockRunBaseClient) BuildRequest(url string, jsonData []byte) (*http.Request, error) {
	return X402BuildRequest(url, jsonData)
}
