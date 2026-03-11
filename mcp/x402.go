package mcp

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
)

const (
	// x402MaxPaymentRetries is the number of retries for 5xx errors on the
	// payment-signed request. The same payment signature is reused (no double-charge).
	x402MaxPaymentRetries = 3

	// x402RetryBaseWait is the base wait between payment retry attempts.
	x402RetryBaseWait = 3 * time.Second
)

// ── Shared x402 types ────────────────────────────────────────────────────────

// x402v2PaymentRequired is the structure of the Payment-Required header (x402 v2).
type x402v2PaymentRequired struct {
	X402Version int              `json:"x402Version"`
	Accepts     []x402AcceptOption `json:"accepts"`
	Resource    *x402Resource    `json:"resource"`
}

// x402AcceptOption is a payment option from the x402 v2 header.
type x402AcceptOption struct {
	Scheme            string            `json:"scheme"`
	Network           string            `json:"network"`
	Amount            string            `json:"amount"`
	Asset             string            `json:"asset"`
	PayTo             string            `json:"payTo"`
	MaxTimeoutSeconds int               `json:"maxTimeoutSeconds"`
	Extra             map[string]string `json:"extra"`
}

// x402Resource describes the resource being paid for.
type x402Resource struct {
	URL         string `json:"url"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// x402SignFunc is a callback that signs an x402 payment header and returns the
// base64-encoded payment signature.
type x402SignFunc func(paymentHeaderB64 string) (string, error)

// ── Shared x402 helpers ──────────────────────────────────────────────────────

// x402DecodeHeader decodes a base64-encoded x402 Payment-Required header,
// trying RawStdEncoding first then StdEncoding as fallback.
func x402DecodeHeader(b64 string) ([]byte, error) {
	decoded, err := base64.RawStdEncoding.DecodeString(b64)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("failed to base64-decode payment header: %w", err)
		}
	}
	return decoded, nil
}

// signBasePaymentHeader decodes a base64 x402 header, parses it, and signs with
// EIP-712 (USDC TransferWithAuthorization). Shared by BlockRunBase and Claw402.
func signBasePaymentHeader(privateKey *ecdsa.PrivateKey, paymentHeaderB64 string, providerName string) (string, error) {
	if privateKey == nil {
		return "", fmt.Errorf("no private key set for %s wallet", providerName)
	}

	decoded, err := x402DecodeHeader(paymentHeaderB64)
	if err != nil {
		return "", err
	}

	var req x402v2PaymentRequired
	if err := json.Unmarshal(decoded, &req); err != nil {
		return "", fmt.Errorf("failed to parse x402 v2 payment header: %w", err)
	}
	if len(req.Accepts) == 0 {
		return "", fmt.Errorf("no payment options in x402 response")
	}

	senderAddr := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	return signX402Payment(privateKey, senderAddr, req.Accepts[0], req.Resource)
}

// doX402Request executes an HTTP request and handles the x402 v2 payment flow.
// On a 402 response it reads the Payment-Required (or X-Payment-Required) header,
// signs via signFn, retries with Payment-Signature, and logs the Payment-Response
// header (tx hash) on success.
func doX402Request(
	httpClient *http.Client,
	buildReqFn func() (*http.Request, error),
	signFn x402SignFunc,
	providerTag string,
	logger Logger,
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
		// Reuses the same payment signature — no double-charge.
		var lastBody []byte
		var lastStatus int
		for attempt := 1; attempt <= x402MaxPaymentRetries; attempt++ {
			req2, err := buildReqFn()
			if err != nil {
				return nil, fmt.Errorf("failed to build retry request: %w", err)
			}
			req2.Header.Set("X-Payment", paymentSig)
			req2.Header.Set("Payment-Signature", paymentSig)

			resp2, err := httpClient.Do(req2)
			if err != nil {
				if attempt < x402MaxPaymentRetries {
					wait := x402RetryBaseWait * time.Duration(attempt)
					logger.Warnf("⚠️  [%s] Payment request failed: %v, retrying in %v (%d/%d)...",
						providerTag, err, wait, attempt+1, x402MaxPaymentRetries)
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

			// Retry on 5xx server errors (502, 503, 520, etc.)
			if resp2.StatusCode >= 500 && attempt < x402MaxPaymentRetries {
				wait := x402RetryBaseWait * time.Duration(attempt)
				logger.Warnf("⚠️  [%s] Server error (status %d), retrying in %v (%d/%d)...",
					providerTag, resp2.StatusCode, wait, attempt+1, x402MaxPaymentRetries)
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

// x402BuildRequest creates a POST request with Content-Type but no auth header.
func x402BuildRequest(url string, jsonData []byte) (*http.Request, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("fail to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// x402SetAuthHeader is a no-op — x402 providers authenticate via payment signing.
func x402SetAuthHeader(_ http.Header) {}

// x402Call handles the x402 payment flow for the simple CallWithMessages path.
func x402Call(c *Client, signFn x402SignFunc, tag string, systemPrompt, userPrompt string) (string, error) {
	c.logger.Infof("📡 [%s] Request AI Server: %s", tag, c.BaseURL)

	requestBody := c.hooks.buildMCPRequestBody(systemPrompt, userPrompt)
	jsonData, err := c.hooks.marshalRequestBody(requestBody)
	if err != nil {
		return "", err
	}

	body, err := doX402Request(c.httpClient, func() (*http.Request, error) {
		return c.hooks.buildRequest(c.hooks.buildUrl(), jsonData)
	}, signFn, tag, c.logger)
	if err != nil {
		return "", err
	}
	return c.hooks.parseMCPResponse(body)
}

// x402CallFull handles the x402 payment flow for the advanced Request path.
func x402CallFull(c *Client, signFn x402SignFunc, tag string, req *Request) (*LLMResponse, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("AI API key not set, please call SetAPIKey first")
	}
	if req.Model == "" {
		req.Model = c.Model
	}

	c.logger.Infof("📡 [%s] Request AI (full): %s", tag, c.BaseURL)

	requestBody := c.hooks.buildRequestBodyFromRequest(req)
	jsonData, err := c.hooks.marshalRequestBody(requestBody)
	if err != nil {
		return nil, err
	}

	body, err := doX402Request(c.httpClient, func() (*http.Request, error) {
		return c.hooks.buildRequest(c.hooks.buildUrl(), jsonData)
	}, signFn, tag, c.logger)
	if err != nil {
		return nil, err
	}
	return c.hooks.parseMCPResponseFull(body)
}
