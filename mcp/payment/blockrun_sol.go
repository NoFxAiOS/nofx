package payment

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"

	"nofx/mcp"
)

const (
	DefaultBlockRunSolURL = "https://sol.blockrun.ai"
	SolanaUSDCMint        = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	SolanaNetwork         = "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp"
	SolanaMainnetRPC      = "https://api.mainnet-beta.solana.com"

	// Compute budget defaults (match @x402/svm)
	computeUnitLimit = uint32(8000)
	computeUnitPrice = uint64(1)
)

func init() {
	mcp.RegisterProvider(mcp.ProviderBlockRunSol, func(opts ...mcp.ClientOption) mcp.AIClient {
		return NewBlockRunSolClientWithOptions(opts...)
	})
}

// BlockRunSolClient implements AIClient using BlockRun's Solana x402 v2 payment protocol.
type BlockRunSolClient struct {
	*mcp.Client
	keypair solana.PrivateKey
}

func (c *BlockRunSolClient) BaseClient() *mcp.Client { return c.Client }

// NewBlockRunSolClient creates a BlockRun Solana wallet client (backward compatible).
func NewBlockRunSolClient() mcp.AIClient {
	return NewBlockRunSolClientWithOptions()
}

// NewBlockRunSolClientWithOptions creates a BlockRun Solana wallet client.
func NewBlockRunSolClientWithOptions(opts ...mcp.ClientOption) mcp.AIClient {
	baseOpts := []mcp.ClientOption{
		mcp.WithProvider(mcp.ProviderBlockRunSol),
		mcp.WithModel(DefaultBlockRunModel),
		mcp.WithBaseURL(DefaultBlockRunSolURL),
		mcp.WithTimeout(X402Timeout),
		mcp.WithMaxRetries(1), // disable outer retry — inner x402 loop handles retries; outer retry causes duplicate payments
	}
	allOpts := append(baseOpts, opts...)
	baseClient := mcp.NewClient(allOpts...).(*mcp.Client)
	baseClient.UseFullURL = true
	baseClient.BaseURL = DefaultBlockRunSolURL + BlockRunChatEndpoint

	c := &BlockRunSolClient{Client: baseClient}
	baseClient.Hooks = c
	return c
}

// SetAPIKey stores the Solana wallet private key (base58-encoded 64-byte keypair).
func (c *BlockRunSolClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	kp, err := solana.PrivateKeyFromBase58(strings.TrimSpace(apiKey))
	if err != nil {
		c.Log.Warnf("⚠️  [MCP] BlockRun Sol: failed to parse private key: %v", err)
		return
	}
	c.keypair = kp
	c.APIKey = apiKey
	c.Log.Infof("🔧 [MCP] BlockRun Sol wallet: %s", kp.PublicKey().String())

	if customModel != "" {
		c.Model = customModel
		c.Log.Infof("🔧 [MCP] BlockRun Sol model: %s", customModel)
	} else {
		c.Log.Infof("🔧 [MCP] BlockRun Sol model: %s", DefaultBlockRunModel)
	}
}

func (c *BlockRunSolClient) SetAuthHeader(h http.Header) { X402SetAuthHeader(h) }

func (c *BlockRunSolClient) Call(systemPrompt, userPrompt string) (string, error) {
	return X402Call(c.Client, c.signSolanaPayment, "BlockRun Sol", systemPrompt, userPrompt)
}

func (c *BlockRunSolClient) CallWithRequestFull(req *mcp.Request) (*mcp.LLMResponse, error) {
	return X402CallFull(c.Client, c.signSolanaPayment, "BlockRun Sol", req)
}

// signSolanaPayment parses the Payment-Required header and builds a signed x402 v2 Solana payload.
func (c *BlockRunSolClient) signSolanaPayment(paymentHeaderB64 string) (string, error) {
	if c.keypair == nil {
		return "", fmt.Errorf("no private key set for BlockRun Sol wallet")
	}

	decoded, err := X402DecodeHeader(paymentHeaderB64)
	if err != nil {
		return "", err
	}

	var req X402v2PaymentRequired
	if err := json.Unmarshal(decoded, &req); err != nil {
		return "", fmt.Errorf("failed to parse x402 v2 Solana header: %w", err)
	}

	// Find the Solana option
	var opt *X402AcceptOption
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
func (c *BlockRunSolClient) buildSolanaTransferTx(recipient, feePayer, amountStr string) (string, error) {
	ownerPubkey := c.keypair.PublicKey()

	recipientPK, err := solana.PublicKeyFromBase58(recipient)
	if err != nil {
		return "", fmt.Errorf("invalid recipient address: %w", err)
	}
	feePayerPK, err := solana.PublicKeyFromBase58(feePayer)
	if err != nil {
		return "", fmt.Errorf("invalid feePayer address: %w", err)
	}
	mintPK := solana.MustPublicKeyFromBase58(SolanaUSDCMint)

	var amountU64 uint64
	if _, err := fmt.Sscanf(amountStr, "%d", &amountU64); err != nil {
		return "", fmt.Errorf("invalid amount %q: %w", amountStr, err)
	}

	sourceATA, _, err := solana.FindAssociatedTokenAddress(ownerPubkey, mintPK)
	if err != nil {
		return "", fmt.Errorf("failed to derive source ATA: %w", err)
	}
	destATA, _, err := solana.FindAssociatedTokenAddress(recipientPK, mintPK)
	if err != nil {
		return "", fmt.Errorf("failed to derive dest ATA: %w", err)
	}

	rpcClient := rpc.New(SolanaMainnetRPC)
	bhResp, err := rpcClient.GetLatestBlockhash(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("failed to fetch blockhash: %w", err)
	}
	recentBlockhash := bhResp.Value.Blockhash

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

	tx, err := solana.NewTransaction(
		[]solana.Instruction{setLimitIx, setPriceIx, transferIx},
		recentBlockhash,
		solana.TransactionPayer(feePayerPK),
	)
	if err != nil {
		return "", fmt.Errorf("failed to build transaction: %w", err)
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(ownerPubkey) {
			return &c.keypair
		}
		return nil // feePayer will be signed by BlockRun CDP
	})
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	txBytes, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to serialize transaction: %w", err)
	}

	return base64.StdEncoding.EncodeToString(txBytes), nil
}

// BuildUrl returns the full BlockRun Solana endpoint URL.
func (c *BlockRunSolClient) BuildUrl() string {
	return DefaultBlockRunSolURL + BlockRunChatEndpoint
}

func (c *BlockRunSolClient) BuildRequest(url string, jsonData []byte) (*http.Request, error) {
	return X402BuildRequest(url, jsonData)
}
