package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

const (
	ProviderBlockRunSol   = "blockrun-sol"
	DefaultBlockRunSolURL = "https://sol.blockrun.ai"
	SolanaUSDCMint        = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	SolanaNetwork         = "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp"
	SolanaMainnetRPC      = "https://api.mainnet-beta.solana.com"

	// Compute budget defaults (match @x402/svm)
	computeUnitLimit = uint32(8000)
	computeUnitPrice = uint64(1)
)

// BlockRunSolClient implements AIClient using BlockRun's Solana x402 v2 payment protocol.
type BlockRunSolClient struct {
	*Client
	keypair solana.PrivateKey
}

// NewBlockRunSolClient creates a BlockRun Solana wallet client (backward compatible).
func NewBlockRunSolClient() AIClient {
	return NewBlockRunSolClientWithOptions()
}

// NewBlockRunSolClientWithOptions creates a BlockRun Solana wallet client.
func NewBlockRunSolClientWithOptions(opts ...ClientOption) AIClient {
	baseOpts := []ClientOption{
		WithProvider(ProviderBlockRunSol),
		WithModel(DefaultBlockRunModel),
		WithBaseURL(DefaultBlockRunSolURL),
	}
	allOpts := append(baseOpts, opts...)
	baseClient := NewClient(allOpts...).(*Client)
	baseClient.UseFullURL = true
	baseClient.BaseURL = DefaultBlockRunSolURL + BlockRunChatEndpoint

	c := &BlockRunSolClient{Client: baseClient}
	baseClient.hooks = c
	return c
}

// SetAPIKey stores the Solana wallet private key (base58-encoded 64-byte keypair).
// customModel selects the AI model; empty means default.
func (c *BlockRunSolClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	kp, err := solana.PrivateKeyFromBase58(strings.TrimSpace(apiKey))
	if err != nil {
		c.logger.Warnf("⚠️  [MCP] BlockRun Sol: failed to parse private key: %v", err)
		return
	}
	c.keypair = kp
	c.APIKey = apiKey
	c.logger.Infof("🔧 [MCP] BlockRun Sol wallet: %s", kp.PublicKey().String())

	if customModel != "" {
		c.Model = customModel
		c.logger.Infof("🔧 [MCP] BlockRun Sol model: %s", customModel)
	} else {
		c.logger.Infof("🔧 [MCP] BlockRun Sol model: %s", DefaultBlockRunModel)
	}
}

func (c *BlockRunSolClient) setAuthHeader(reqHeaders http.Header) {
	// No Bearer token — payment is via x402 signing
}

// call overrides the base call to handle HTTP 402 x402 v2 Solana payment flow.
func (c *BlockRunSolClient) call(systemPrompt, userPrompt string) (string, error) {
	c.logger.Infof("📡 [BlockRun Sol] Request AI Server: %s", c.BaseURL)

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

		paymentSig, err := c.signSolanaPayment(paymentHeader)
		if err != nil {
			return "", fmt.Errorf("failed to sign Solana x402 payment: %w", err)
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
			return "", fmt.Errorf("BlockRun Sol payment retry failed (status %d): %s", resp2.StatusCode, string(body2))
		}
		return c.hooks.parseMCPResponse(body2)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("BlockRun Sol API error (status %d): %s", resp.StatusCode, string(body))
	}
	return c.hooks.parseMCPResponse(body)
}

// solanaPaymentOption is an entry in the accepts[] array of the x402 v2 response.
type solanaPaymentOption struct {
	Scheme            string            `json:"scheme"`
	Network           string            `json:"network"`
	Amount            string            `json:"amount"`
	Asset             string            `json:"asset"`
	PayTo             string            `json:"payTo"`
	MaxTimeoutSeconds int               `json:"maxTimeoutSeconds"`
	Extra             map[string]string `json:"extra"`
}

// x402v2SolanaRequired is the parsed X-Payment-Required header for Solana.
type x402v2SolanaRequired struct {
	X402Version int                   `json:"x402Version"`
	Accepts     []solanaPaymentOption `json:"accepts"`
	Resource    *struct {
		URL         string `json:"url"`
		Description string `json:"description"`
		MimeType    string `json:"mimeType"`
	} `json:"resource"`
}

// signSolanaPayment parses the X-Payment-Required header and builds a signed x402 v2 Solana payload.
func (c *BlockRunSolClient) signSolanaPayment(paymentHeaderB64 string) (string, error) {
	if c.keypair == nil {
		return "", fmt.Errorf("no private key set for BlockRun Sol wallet")
	}

	// Decode base64 → JSON
	decoded, err := base64.RawStdEncoding.DecodeString(paymentHeaderB64)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(paymentHeaderB64)
		if err != nil {
			return "", fmt.Errorf("failed to base64-decode payment header: %w", err)
		}
	}

	var req x402v2SolanaRequired
	if err := json.Unmarshal(decoded, &req); err != nil {
		return "", fmt.Errorf("failed to parse x402 v2 Solana header: %w", err)
	}

	// Find the Solana option
	var opt *solanaPaymentOption
	for i := range req.Accepts {
		if strings.HasPrefix(req.Accepts[i].Network, "solana:") {
			opt = &req.Accepts[i]
			break
		}
	}
	if opt == nil {
		return "", fmt.Errorf("no Solana payment option in x402 response")
	}

	recipient := opt.PayTo
	amount := opt.Amount
	feePayer := ""
	if opt.Extra != nil {
		feePayer = opt.Extra["feePayer"]
	}
	if feePayer == "" {
		return "", fmt.Errorf("feePayer missing from Solana x402 extra")
	}

	maxTimeout := opt.MaxTimeoutSeconds
	if maxTimeout == 0 {
		maxTimeout = 300
	}

	resourceURL := DefaultBlockRunSolURL + BlockRunChatEndpoint
	resourceDesc := ""
	resourceMime := "application/json"
	if req.Resource != nil {
		resourceURL = req.Resource.URL
		resourceDesc = req.Resource.Description
		resourceMime = req.Resource.MimeType
	}

	// Build the SPL TransferChecked transaction
	txB64, err := c.buildSolanaTransferTx(recipient, feePayer, amount)
	if err != nil {
		return "", fmt.Errorf("failed to build Solana transfer tx: %w", err)
	}

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
			"network":           SolanaNetwork,
			"amount":            amount,
			"asset":             SolanaUSDCMint,
			"payTo":             recipient,
			"maxTimeoutSeconds": maxTimeout,
			"extra":             opt.Extra,
		},
		"payload": map[string]string{
			"transaction": txB64,
		},
		"extensions": map[string]interface{}{},
	}

	resultJSON, err := json.Marshal(paymentData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Solana payment: %w", err)
	}

	return base64.StdEncoding.EncodeToString(resultJSON), nil
}

// buildSolanaTransferTx builds a partial-signed VersionedTransaction for SPL USDC TransferChecked.
// The fee payer (CDP facilitator) slot is left with a zero signature; only the user signs.
func (c *BlockRunSolClient) buildSolanaTransferTx(recipient, feePayer, amountStr string) (string, error) {
	ownerPubkey := c.keypair.PublicKey()

	// Parse recipient and feePayer
	recipientPK, err := solana.PublicKeyFromBase58(recipient)
	if err != nil {
		return "", fmt.Errorf("invalid recipient address: %w", err)
	}
	feePayerPK, err := solana.PublicKeyFromBase58(feePayer)
	if err != nil {
		return "", fmt.Errorf("invalid feePayer address: %w", err)
	}
	mintPK := solana.MustPublicKeyFromBase58(SolanaUSDCMint)

	// Parse amount
	var amountU64 uint64
	if _, err := fmt.Sscanf(amountStr, "%d", &amountU64); err != nil {
		return "", fmt.Errorf("invalid amount %q: %w", amountStr, err)
	}

	// Derive ATAs
	sourceATA, _, err := solana.FindAssociatedTokenAddress(ownerPubkey, mintPK)
	if err != nil {
		return "", fmt.Errorf("failed to derive source ATA: %w", err)
	}
	destATA, _, err := solana.FindAssociatedTokenAddress(recipientPK, mintPK)
	if err != nil {
		return "", fmt.Errorf("failed to derive dest ATA: %w", err)
	}

	// Fetch latest blockhash from Solana mainnet
	rpcClient := rpc.New(SolanaMainnetRPC)
	bhResp, err := rpcClient.GetLatestBlockhash(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("failed to fetch blockhash: %w", err)
	}
	recentBlockhash := bhResp.Value.Blockhash

	// Build instructions: ComputeBudgetSetLimit, ComputeBudgetSetPrice, TransferChecked
	setLimitIx, err := computebudget.NewSetComputeUnitLimitInstruction(computeUnitLimit).ValidateAndBuild()
	if err != nil {
		return "", fmt.Errorf("failed to build SetComputeUnitLimit: %w", err)
	}
	setPriceIx, err := computebudget.NewSetComputeUnitPriceInstruction(computeUnitPrice).ValidateAndBuild()
	if err != nil {
		return "", fmt.Errorf("failed to build SetComputeUnitPrice: %w", err)
	}
	transferIx, err := token.NewTransferCheckedInstruction(
		amountU64,
		6, // USDC decimals
		sourceATA,
		mintPK,
		destATA,
		ownerPubkey,
		[]solana.PublicKey{},
	).ValidateAndBuild()
	if err != nil {
		return "", fmt.Errorf("failed to build TransferChecked: %w", err)
	}

	// Build transaction with feePayer as payer (matches Python SDK)
	tx, err := solana.NewTransaction(
		[]solana.Instruction{setLimitIx, setPriceIx, transferIx},
		recentBlockhash,
		solana.TransactionPayer(feePayerPK),
	)
	if err != nil {
		return "", fmt.Errorf("failed to build transaction: %w", err)
	}

	// Partial sign: user signs; fee_payer (CDP) co-signs on server side
	// The transaction has 2 signers: [feePayer (index 0), owner (index 1)]
	// We sign only our index (owner).
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(ownerPubkey) {
			return &c.keypair
		}
		return nil // feePayer will be signed by BlockRun CDP
	})
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Serialize transaction
	txBytes, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to serialize transaction: %w", err)
	}

	return base64.StdEncoding.EncodeToString(txBytes), nil
}

// buildUrl returns the full BlockRun Solana endpoint URL.
func (c *BlockRunSolClient) buildUrl() string {
	return DefaultBlockRunSolURL + BlockRunChatEndpoint
}

// buildRequest creates the HTTP request without an Authorization header.
func (c *BlockRunSolClient) buildRequest(url string, jsonData []byte) (*http.Request, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("fail to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}
