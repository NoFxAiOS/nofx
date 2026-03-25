package nofxos

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"net/http"
	"nofx/mcp"
	"nofx/mcp/payment"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

// Claw402DataClient wraps nofxos API calls through claw402's x402 payment gateway.
// Instead of calling nofxos.ai directly, it calls claw402.ai/api/v1/nofx/...
// and pays with USDC for each request.
type Claw402DataClient struct {
	claw402URL string
	privateKey *ecdsa.PrivateKey
	httpClient *http.Client
	logger     mcp.Logger
}

// NewClaw402DataClient creates a client that routes nofxos requests through claw402.
// privateKeyHex is the wallet private key (0x-prefixed hex string).
func NewClaw402DataClient(claw402URL, privateKeyHex string, logger mcp.Logger) (*Claw402DataClient, error) {
	if claw402URL == "" {
		claw402URL = "https://claw402.ai"
	}
	claw402URL = strings.TrimRight(claw402URL, "/")

	if privateKeyHex == "" {
		privateKeyHex = os.Getenv("CLAW402_WALLET_KEY")
	}
	if privateKeyHex == "" {
		return nil, fmt.Errorf("claw402 wallet private key not set")
	}

	hexKey := strings.TrimPrefix(privateKeyHex, "0x")
	pk, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid claw402 private key: %w", err)
	}

	return &Claw402DataClient{
		claw402URL: claw402URL,
		privateKey: pk,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}, nil
}

// endpoint mapping: nofxos path → claw402 path
var endpointMap = map[string]string{
	"/api/ai500/list":  "/api/v1/nofx/ai500/list",
	"/api/ai500/stats": "/api/v1/nofx/ai500/stats",
}

// mapEndpoint converts a nofxos endpoint to a claw402 endpoint.
// For endpoints not in the static map, applies the general pattern:
// /api/xxx → /api/v1/nofx/xxx
func mapEndpoint(nofxosPath string) string {
	if mapped, ok := endpointMap[nofxosPath]; ok {
		return mapped
	}
	// General pattern: /api/xxx → /api/v1/nofx/xxx
	if strings.HasPrefix(nofxosPath, "/api/") {
		return "/api/v1/nofx/" + strings.TrimPrefix(nofxosPath, "/api/")
	}
	return nofxosPath
}

// DoRequest makes a GET request through claw402 with x402 payment.
func (c *Claw402DataClient) DoRequest(endpoint string) ([]byte, error) {
	claw402Path := mapEndpoint(endpoint)
	// Strip auth= query params (claw402 uses x402 payment, not auth keys)
	if idx := strings.Index(claw402Path, "?auth="); idx != -1 {
		claw402Path = claw402Path[:idx]
	}
	if idx := strings.Index(claw402Path, "&auth="); idx != -1 {
		claw402Path = claw402Path[:idx]
	}

	fullURL := c.claw402URL + claw402Path

	buildReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, fullURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Client-ID", "nofx")
		return req, nil
	}

	signFn := payment.MakeClaw402SignFunc(c.privateKey)

	body, err := payment.DoX402Request(
		c.httpClient,
		buildReq,
		signFn,
		"claw402-data",
		c.logger,
	)
	if err != nil {
		return nil, fmt.Errorf("claw402 data request failed (%s): %w", claw402Path, err)
	}

	return body, nil
}
