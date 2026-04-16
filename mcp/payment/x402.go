package payment

import (
	"bytes"
	"context"
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

	"nofx/mcp"
)

const (
	// X402MaxPaymentRetries is the number of retries for 5xx/expired-402 errors
	// on the payment-signed request. Payment is re-signed on 402 (no double-charge).
	X402MaxPaymentRetries = 5

	// X402RetryBaseWait is the base wait between payment retry attempts.
	X402RetryBaseWait = 3 * time.Second

	// X402Timeout is the HTTP timeout for x402 payment providers.
	// AI inference (especially DeepSeek) can take several minutes; the default
	// 120s causes premature timeouts that trigger duplicate payments.
	X402Timeout = 5 * time.Minute
)

// ── Shared x402 types ────────────────────────────────────────────────────────

// X402v2PaymentRequired is the structure of the Payment-Required header (x402 v2).
type X402v2PaymentRequired struct {
	X402Version int              `json:"x402Version"`
	Accepts     []X402AcceptOption `json:"accepts"`
	Resource    *X402Resource    `json:"resource"`
}

// X402AcceptOption is a payment option from the x402 v2 header.
type X402AcceptOption struct {
	Scheme            string            `json:"scheme"`
	Network           string            `json:"network"`
	Amount            string            `json:"amount"`
	Asset             string            `json:"asset"`
	PayTo             string            `json:"payTo"`
	MaxTimeoutSeconds int               `json:"maxTimeoutSeconds"`
	Extra             map[string]string `json:"extra"`
}

// X402Resource describes the resource being paid for.
type X402Resource struct {
	URL         string `json:"url"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// X402SignFunc is a callback that signs an x402 payment header and returns the
// base64-encoded payment signature.
type X402SignFunc func(paymentHeaderB64 string) (string, error)

// ── Shared x402 helpers ──────────────────────────────────────────────────────

// X402DecodeHeader decodes a base64-encoded x402 Payment-Required header,
// trying RawStdEncoding first then StdEncoding as fallback.
func X402DecodeHeader(b64 string) ([]byte, error) {
	decoded, err := base64.RawStdEncoding.DecodeString(b64)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("failed to base64-decode payment header: %w", err)
		}
	}
	return decoded, nil
}

// MakeClaw402SignFunc creates an X402SignFunc from a private key for claw402 payments.
func MakeClaw402SignFunc(privateKey *ecdsa.PrivateKey) X402SignFunc {
	return func(paymentHeaderB64 string) (string, error) {
		return SignBasePaymentHeader(privateKey, paymentHeaderB64, "Claw402")
	}
}

// SignBasePaymentHeader decodes a base64 x402 header, parses it, and signs with
// EIP-712 (USDC TransferWithAuthorization).
func SignBasePaymentHeader(privateKey *ecdsa.PrivateKey, paymentHeaderB64 string, providerName string) (string, error) {
	if privateKey == nil {
		return "", fmt.Errorf("no private key set for %s wallet", providerName)
	}

	decoded, err := X402DecodeHeader(paymentHeaderB64)
	if err != nil {
		return "", err
	}

	var req X402v2PaymentRequired
	if err := json.Unmarshal(decoded, &req); err != nil {
		return "", fmt.Errorf("failed to parse x402 v2 payment header: %w", err)
	}
	if len(req.Accepts) == 0 {
		return "", fmt.Errorf("no payment options in x402 response")
	}

	senderAddr := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	return SignX402Payment(privateKey, senderAddr, req.Accepts[0], req.Resource)
}

// DoX402Request executes an HTTP request and handles the x402 v2 payment flow.
func DoX402Request(
	httpClient *http.Client,
	buildReqFn func() (*http.Request, error),
	signFn X402SignFunc,
	providerTag string,
	logger mcp.Logger,
) ([]byte, error) {
	req, err := buildReqFn()
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusPaymentRequired {
		paymentHeader := resp.Header.Get("Payment-Required")
		if paymentHeader == "" {
			paymentHeader = resp.Header.Get("X-Payment-Required")
		}
		if paymentHeader == "" {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("received 402 but no Payment-Required header found. Body: %s", string(body))
		}

		// Drain 402 body to allow HTTP connection reuse.
		_, _ = io.Copy(io.Discard, resp.Body)

		paymentSig, err := signFn(paymentHeader)
		if err != nil {
			return nil, fmt.Errorf("failed to sign x402 payment: %w", err)
		}

		// Retry loop for 5xx / expired-402 errors on the payment-signed request.
		var lastBody []byte
		var lastStatus int
		for attempt := 1; attempt <= X402MaxPaymentRetries; attempt++ {
			req2, err := buildReqFn()
			if err != nil {
				return nil, fmt.Errorf("failed to build retry request: %w", err)
			}
			req2.Header.Set("X-Payment", paymentSig)
			req2.Header.Set("Payment-Signature", paymentSig)

			resp2, err := httpClient.Do(req2)
			if err != nil {
				if attempt < X402MaxPaymentRetries {
					wait := X402RetryBaseWait * time.Duration(attempt)
					logger.Warnf("⚠️  [%s] Payment request failed: %v, retrying in %v (%d/%d)...",
						providerTag, err, wait, attempt+1, X402MaxPaymentRetries)
					time.Sleep(wait)
					continue
				}
				return nil, fmt.Errorf("failed to send payment retry: %w", err)
			}

			body2, readErr := io.ReadAll(resp2.Body)
			resp2.Body.Close()
			if readErr != nil {
				return nil, fmt.Errorf("failed to read payment retry response: %w", readErr)
			}

			if resp2.StatusCode == http.StatusOK {
				if txHash := resp2.Header.Get("Payment-Response"); txHash != "" {
					logger.Infof("💰 [%s] Payment tx: %s", providerTag, txHash)
				}
				if attempt > 1 {
					logger.Infof("✅ [%s] Payment retry succeeded on attempt %d", providerTag, attempt)
				}
				return body2, nil
			}

			lastBody = body2
			lastStatus = resp2.StatusCode

			retryable := resp2.StatusCode >= 500 || resp2.StatusCode == http.StatusPaymentRequired

			if retryable && attempt < X402MaxPaymentRetries {
				wait := X402RetryBaseWait * time.Duration(attempt)

				// If we got 402 again, the payment signature expired — re-sign.
				if resp2.StatusCode == http.StatusPaymentRequired {
					newHeader := resp2.Header.Get("Payment-Required")
					if newHeader == "" {
						newHeader = resp2.Header.Get("X-Payment-Required")
					}
					if newHeader != "" {
						newSig, signErr := signFn(newHeader)
						if signErr == nil {
							paymentSig = newSig
							logger.Warnf("⚠️  [%s] Payment expired (402), re-signed and retrying in %v (%d/%d)...",
								providerTag, wait, attempt+1, X402MaxPaymentRetries)
						} else {
							logger.Warnf("⚠️  [%s] Payment expired (402), re-sign failed: %v, retrying in %v (%d/%d)...",
								providerTag, signErr, wait, attempt+1, X402MaxPaymentRetries)
						}
					} else {
						logger.Warnf("⚠️  [%s] Got 402 but no new Payment-Required header, retrying in %v (%d/%d)...",
							providerTag, wait, attempt+1, X402MaxPaymentRetries)
					}
				} else {
					logger.Warnf("⚠️  [%s] Server error (status %d), retrying in %v (%d/%d)...",
						providerTag, resp2.StatusCode, wait, attempt+1, X402MaxPaymentRetries)
				}

				time.Sleep(wait)
				continue
			}

			// Non-retryable error or final attempt — fail
			break
		}

		return nil, fmt.Errorf("%s payment retry failed (status %d): %s", providerTag, lastStatus, string(lastBody))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s API error (status %d): %s", providerTag, resp.StatusCode, string(body))
	}
	return body, nil
}

// DoX402RequestStream executes an HTTP request with x402 v2 payment flow and
// returns the open *http.Response for streaming. The caller is responsible for
// reading and closing the response body.
// The provided ctx is attached to the final successful HTTP request so that
// cancelling ctx will immediately close the underlying connection and unblock
// any pending body reads.
func DoX402RequestStream(
	ctx context.Context,
	httpClient *http.Client,
	buildReqFn func() (*http.Request, error),
	signFn X402SignFunc,
	providerTag string,
	logger mcp.Logger,
) (*http.Response, error) {
	// Initial request — use background context (no idle timeout yet).
	req, err := buildReqFn()
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Non-402 initial response
	if resp.StatusCode != http.StatusPaymentRequired {
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("%s API error (status %d): %s", providerTag, resp.StatusCode, string(body))
	}

	// 402 — extract payment header and sign
	paymentHeader := resp.Header.Get("Payment-Required")
	if paymentHeader == "" {
		paymentHeader = resp.Header.Get("X-Payment-Required")
	}
	if paymentHeader == "" {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("received 402 but no Payment-Required header found. Body: %s", string(body))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	paymentSig, err := signFn(paymentHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to sign x402 payment: %w", err)
	}

	// Retry loop for the payment-signed request.
	// Attach ctx to these requests so the caller can cancel body reads.
	var lastStatus int
	var lastBody []byte
	for attempt := 1; attempt <= X402MaxPaymentRetries; attempt++ {
		req2, err := buildReqFn()
		if err != nil {
			return nil, fmt.Errorf("failed to build retry request: %w", err)
		}
		req2 = req2.WithContext(ctx)
		req2.Header.Set("X-Payment", paymentSig)
		req2.Header.Set("Payment-Signature", paymentSig)

		resp2, err := httpClient.Do(req2)
		if err != nil {
			if attempt < X402MaxPaymentRetries {
				wait := X402RetryBaseWait * time.Duration(attempt)
				logger.Warnf("⚠️  [%s] Payment request failed: %v, retrying in %v (%d/%d)...",
					providerTag, err, wait, attempt+1, X402MaxPaymentRetries)
				time.Sleep(wait)
				continue
			}
			return nil, fmt.Errorf("failed to send payment retry: %w", err)
		}

		if resp2.StatusCode == http.StatusOK {
			if txHash := resp2.Header.Get("Payment-Response"); txHash != "" {
				logger.Infof("💰 [%s] Payment tx: %s", providerTag, txHash)
			}
			if attempt > 1 {
				logger.Infof("✅ [%s] Payment retry succeeded on attempt %d", providerTag, attempt)
			}
			return resp2, nil // caller reads and closes body
		}

		// Non-200: read body for error handling / re-sign
		body2, readErr := io.ReadAll(resp2.Body)
		resp2.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("failed to read payment retry response: %w", readErr)
		}

		lastBody = body2
		lastStatus = resp2.StatusCode

		retryable := resp2.StatusCode >= 500 || resp2.StatusCode == http.StatusPaymentRequired

		if retryable && attempt < X402MaxPaymentRetries {
			wait := X402RetryBaseWait * time.Duration(attempt)

			if resp2.StatusCode == http.StatusPaymentRequired {
				newHeader := resp2.Header.Get("Payment-Required")
				if newHeader == "" {
					newHeader = resp2.Header.Get("X-Payment-Required")
				}
				if newHeader != "" {
					newSig, signErr := signFn(newHeader)
					if signErr == nil {
						paymentSig = newSig
						logger.Warnf("⚠️  [%s] Payment expired (402), re-signed and retrying in %v (%d/%d)...",
							providerTag, wait, attempt+1, X402MaxPaymentRetries)
					} else {
						logger.Warnf("⚠️  [%s] Payment expired (402), re-sign failed: %v, retrying in %v (%d/%d)...",
							providerTag, signErr, wait, attempt+1, X402MaxPaymentRetries)
					}
				} else {
					logger.Warnf("⚠️  [%s] Got 402 but no new Payment-Required header, retrying in %v (%d/%d)...",
						providerTag, wait, attempt+1, X402MaxPaymentRetries)
				}
			} else {
				logger.Warnf("⚠️  [%s] Server error (status %d), retrying in %v (%d/%d)...",
					providerTag, resp2.StatusCode, wait, attempt+1, X402MaxPaymentRetries)
			}

			time.Sleep(wait)
			continue
		}

		break
	}

	return nil, fmt.Errorf("%s payment retry failed (status %d): %s", providerTag, lastStatus, string(lastBody))
}

// x402StreamIdleTimeout is the idle timeout for SSE streaming through x402.
// If no SSE line arrives for this duration, the stream is considered stalled.
const x402StreamIdleTimeout = 90 * time.Second

// X402CallStream handles the x402 payment flow with streaming for the simple Call path.
// It adds "stream": true to the request body and uses ParseSSEStream to read chunks.
//
// Robustness: uses TeeReader so the raw body is captured while parsing SSE.
// If SSE parsing yields no text (e.g. server returned plain JSON despite stream:true),
// falls back to ParseMCPResponse on the buffered body.
func X402CallStream(c *mcp.Client, signFn X402SignFunc, tag string, systemPrompt, userPrompt string, onChunk func(string)) (string, error) {
	c.Log.Infof("📡 [%s] Request AI Server (stream): %s", tag, c.BaseURL)

	requestBody := c.Hooks.BuildMCPRequestBody(systemPrompt, userPrompt)
	requestBody["stream"] = true
	jsonData, err := c.Hooks.MarshalRequestBody(requestBody)
	if err != nil {
		return "", err
	}

	// Idle-timeout context: cancel() closes the underlying TCP connection,
	// which immediately unblocks any pending resp.Body.Read().
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resp, err := DoX402RequestStream(ctx, c.HTTPClient, func() (*http.Request, error) {
		return c.Hooks.BuildRequest(c.Hooks.BuildUrl(), jsonData)
	}, signFn, tag, c.Log)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	c.Log.Infof("📡 [%s] Response Content-Type: %s", tag, ct)

	// Start idle-timeout watchdog AFTER the 402 dance is done.
	resetCh := make(chan struct{}, 1)
	go func() {
		t := time.NewTimer(x402StreamIdleTimeout)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c.Log.Warnf("⚠️  [%s] SSE idle timeout (%v), cancelling stream", tag, x402StreamIdleTimeout)
				cancel() // closes the TCP connection → body.Read() returns error
				return
			case <-resetCh:
				if !t.Stop() {
					select {
					case <-t.C:
					default:
					}
				}
				t.Reset(x402StreamIdleTimeout)
			}
		}
	}()

	onLine := func() {
		select {
		case resetCh <- struct{}{}:
		default:
		}
	}

	// TeeReader: body is streamed through SSE parser AND captured in bodyBuf.
	// If SSE yields nothing (server returned JSON), we can still parse bodyBuf.
	var bodyBuf bytes.Buffer
	tee := io.TeeReader(resp.Body, &bodyBuf)

	text, usage, sseErr := mcp.ParseSSEStream(tee, onChunk, onLine)
	mcp.ReportStreamUsage(usage, c.Provider, c.Model)

	if text != "" {
		c.Log.Infof("📡 [%s] SSE stream complete, got %d chars", tag, len(text))
		return text, nil
	}

	// SSE yielded nothing — try JSON fallback on the buffered body.
	if bodyBuf.Len() > 0 {
		c.Log.Infof("📡 [%s] SSE empty, trying JSON fallback on %d bytes", tag, bodyBuf.Len())
		jsonText, jsonErr := c.Hooks.ParseMCPResponse(bodyBuf.Bytes())
		if jsonErr == nil && jsonText != "" {
			return jsonText, nil
		}
		c.Log.Warnf("⚠️  [%s] JSON fallback also failed: %v", tag, jsonErr)
	}

	if sseErr != nil {
		return "", fmt.Errorf("[%s] stream failed: %w", tag, sseErr)
	}
	return "", fmt.Errorf("[%s] no content received (SSE empty, body %d bytes)", tag, bodyBuf.Len())
}

// X402BuildRequest creates a POST request with Content-Type but no auth header.
func X402BuildRequest(url string, jsonData []byte) (*http.Request, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("fail to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-ID", "nofx")
	return req, nil
}

// X402SetAuthHeader is a no-op — x402 providers authenticate via payment signing.
func X402SetAuthHeader(_ http.Header) {}

// X402Call handles the x402 payment flow for the simple CallWithMessages path.
func X402Call(c *mcp.Client, signFn X402SignFunc, tag string, systemPrompt, userPrompt string) (string, error) {
	c.Log.Infof("📡 [%s] Request AI Server: %s", tag, c.BaseURL)

	requestBody := c.Hooks.BuildMCPRequestBody(systemPrompt, userPrompt)
	jsonData, err := c.Hooks.MarshalRequestBody(requestBody)
	if err != nil {
		return "", err
	}

	body, err := DoX402Request(c.HTTPClient, func() (*http.Request, error) {
		return c.Hooks.BuildRequest(c.Hooks.BuildUrl(), jsonData)
	}, signFn, tag, c.Log)
	if err != nil {
		return "", err
	}
	return c.Hooks.ParseMCPResponse(body)
}

// X402CallFull handles the x402 payment flow for the advanced Request path.
func X402CallFull(c *mcp.Client, signFn X402SignFunc, tag string, req *mcp.Request) (*mcp.LLMResponse, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("AI API key not set, please call SetAPIKey first")
	}
	if req.Model == "" {
		req.Model = c.Model
	}

	c.Log.Infof("📡 [%s] Request AI (full): %s", tag, c.BaseURL)

	requestBody := c.Hooks.BuildRequestBodyFromRequest(req)
	jsonData, err := c.Hooks.MarshalRequestBody(requestBody)
	if err != nil {
		return nil, err
	}

	body, err := DoX402Request(c.HTTPClient, func() (*http.Request, error) {
		return c.Hooks.BuildRequest(c.Hooks.BuildUrl(), jsonData)
	}, signFn, tag, c.Log)
	if err != nil {
		return nil, err
	}
	return c.Hooks.ParseMCPResponseFull(body)
}

// ── Shared EIP-712 constants & helpers (Base chain, USDC) ────────────────────

const (
	BaseUSDCContract       = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
	BaseChainID      int64 = 8453
	BaseNetwork            = "eip155:8453"
)

// EIP-712 type hashes for USDC TransferWithAuthorization (ERC-3009)
var (
	eip712DomainTypeHash     = keccak256String("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)")
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

// SignX402Payment is the shared EIP-712 signing logic for x402 v2 on Base USDC.
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
