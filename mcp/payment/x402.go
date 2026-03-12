package payment

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	"nofx/mcp"
)

const (
	// X402MaxPaymentRetries is the number of retries for 5xx errors on the
	// payment-signed request. The same payment signature is reused (no double-charge).
	X402MaxPaymentRetries = 3

	// X402RetryBaseWait is the base wait between payment retry attempts.
	X402RetryBaseWait = 3 * time.Second
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

// SignBasePaymentHeader decodes a base64 x402 header, parses it, and signs with
// EIP-712 (USDC TransferWithAuthorization). Shared by BlockRunBase and Claw402.
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

		// Retry loop for 5xx errors on the payment-signed request.
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

			// Retry on 5xx server errors
			if resp2.StatusCode >= 500 && attempt < X402MaxPaymentRetries {
				wait := X402RetryBaseWait * time.Duration(attempt)
				logger.Warnf("⚠️  [%s] Server error (status %d), retrying in %v (%d/%d)...",
					providerTag, resp2.StatusCode, wait, attempt+1, X402MaxPaymentRetries)
				time.Sleep(wait)
				continue
			}

			// Non-5xx error or final attempt — fail
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

// X402BuildRequest creates a POST request with Content-Type but no auth header.
func X402BuildRequest(url string, jsonData []byte) (*http.Request, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("fail to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
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
