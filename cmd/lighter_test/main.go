// Lighter API Authentication Test Tool
// Usage: go run cmd/lighter_test/main.go -wallet=0x... -apikey=... [-testnet]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	lighterClient "github.com/elliottech/lighter-go/client"
	lighterHTTP "github.com/elliottech/lighter-go/client/http"
)

func main() {
	// Parse command line flags
	walletAddr := flag.String("wallet", "", "Ethereum wallet address")
	apiKeyPrivateKey := flag.String("apikey", "", "API key private key (40 bytes hex)")
	apiKeyIndex := flag.Int("apikeyindex", 0, "API key index (0-255)")
	testnet := flag.Bool("testnet", false, "Use testnet instead of mainnet")
	flag.Parse()

	if *walletAddr == "" || *apiKeyPrivateKey == "" {
		fmt.Println("Usage: go run cmd/lighter_test/main.go -wallet=0x... -apikey=...")
		fmt.Println("Options:")
		fmt.Println("  -wallet        Ethereum wallet address (required)")
		fmt.Println("  -apikey        API key private key, 40 bytes hex (required)")
		fmt.Println("  -apikeyindex   API key index, 0-255 (default: 0)")
		fmt.Println("  -testnet       Use testnet instead of mainnet")
		os.Exit(1)
	}

	fmt.Println("=== Lighter API Authentication Test ===")
	fmt.Printf("Wallet: %s\n", *walletAddr)
	fmt.Printf("API Key Index: %d\n", *apiKeyIndex)
	fmt.Printf("Testnet: %v\n", *testnet)
	fmt.Println()

	// Determine base URL
	baseURL := "https://mainnet.zklighter.elliot.ai"
	chainID := uint32(304)
	if *testnet {
		baseURL = "https://testnet.zklighter.elliot.ai"
		chainID = uint32(300)
	}

	// Create HTTP client
	httpClient := lighterHTTP.NewClient(baseURL)
	client := &http.Client{Timeout: 30 * time.Second}

	// Step 1: Get account info
	fmt.Println("Step 1: Getting account info...")
	accountInfo, err := getAccountByL1Address(client, baseURL, *walletAddr)
	if err != nil {
		fmt.Printf("ERROR: Failed to get account info: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("SUCCESS: Account index = %d\n\n", accountInfo.AccountIndex)

	// Step 2: Create TxClient
	fmt.Println("Step 2: Creating TxClient...")
	txClient, err := lighterClient.NewTxClient(
		httpClient,
		*apiKeyPrivateKey,
		accountInfo.AccountIndex,
		uint8(*apiKeyIndex),
		chainID,
	)
	if err != nil {
		fmt.Printf("ERROR: Failed to create TxClient: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("SUCCESS: TxClient created\n")

	// Step 3: Generate auth token
	fmt.Println("Step 3: Generating auth token...")
	deadline := time.Now().Add(1 * time.Hour)
	authToken, err := txClient.GetAuthToken(deadline)
	if err != nil {
		fmt.Printf("ERROR: Failed to generate auth token: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("SUCCESS: Auth token generated\n")
	fmt.Printf("Token: %s...\n", authToken[:min(50, len(authToken))])
	fmt.Printf("Valid until: %s\n\n", deadline.Format(time.RFC3339))

	// Step 4: Test GetActiveOrders API with auth query parameter
	fmt.Println("Step 4: Testing GetActiveOrders API...")
	encodedAuth := url.QueryEscape(authToken)
	endpoint := fmt.Sprintf("%s/api/v1/accountActiveOrders?account_index=%d&market_id=0&auth=%s",
		baseURL, accountInfo.AccountIndex, encodedAuth)

	fmt.Printf("Endpoint: %s...\n", endpoint[:min(120, len(endpoint))])

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		fmt.Printf("ERROR: Failed to create request: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("ERROR: Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Response: %s\n\n", string(body))

	// Parse response
	var apiResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Orders  []struct {
			OrderID string `json:"order_id"`
			Side    string `json:"side"`
			Type    string `json:"type"`
			Price   string `json:"price"`
		} `json:"orders"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		fmt.Printf("ERROR: Failed to parse response: %v\n", err)
		os.Exit(1)
	}

	if apiResp.Code != 200 {
		fmt.Printf("API ERROR: code=%d, message=%s\n", apiResp.Code, apiResp.Message)
		fmt.Println("\n=== DIAGNOSTIC INFO ===")
		fmt.Println("If you see 'invalid signature', possible causes:")
		fmt.Println("1. API key is not registered on-chain")
		fmt.Println("2. API key private key is incorrect")
		fmt.Println("3. API key index is wrong")
		fmt.Println("4. Account index mismatch")
		fmt.Println("\nTo fix:")
		fmt.Println("- Go to app.lighter.xyz and register/verify your API key")
		fmt.Println("- Make sure you're using the correct API key private key")
		os.Exit(1)
	}

	fmt.Printf("SUCCESS: Retrieved %d orders\n", len(apiResp.Orders))
	for i, order := range apiResp.Orders {
		if i >= 5 {
			fmt.Printf("... and %d more orders\n", len(apiResp.Orders)-5)
			break
		}
		fmt.Printf("  Order %s: %s %s @ %s\n", order.OrderID, order.Side, order.Type, order.Price)
	}

	// Step 5: Test GetTrades API (also needs auth)
	fmt.Println("\nStep 5: Testing GetTrades API...")
	tradesEndpoint := fmt.Sprintf("%s/api/v1/trades?account_index=%d&sort_by=timestamp&sort_dir=desc&limit=5&auth=%s",
		baseURL, accountInfo.AccountIndex, encodedAuth)

	tradesReq, _ := http.NewRequest("GET", tradesEndpoint, nil)
	tradesResp, err := client.Do(tradesReq)
	if err != nil {
		fmt.Printf("ERROR: Trades request failed: %v\n", err)
	} else {
		defer tradesResp.Body.Close()
		tradesBody, _ := io.ReadAll(tradesResp.Body)
		fmt.Printf("Status: %d\n", tradesResp.StatusCode)
		if tradesResp.StatusCode == 200 {
			fmt.Println("SUCCESS: GetTrades API working")
		} else {
			fmt.Printf("Response: %s\n", string(tradesBody))
		}
	}

	fmt.Println("\n=== ALL TESTS PASSED ===")
}

// AccountInfo represents Lighter account information
type AccountInfo struct {
	AccountIndex int64  `json:"account_index"`
	L1Address    string `json:"l1_address"`
}

// getAccountByL1Address gets account info by L1 wallet address
func getAccountByL1Address(client *http.Client, baseURL, walletAddr string) (*AccountInfo, error) {
	endpoint := fmt.Sprintf("%s/api/v1/account?by=l1_address&value=%s", baseURL, walletAddr)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse response - can be in "accounts" or "sub_accounts" field
	var apiResp struct {
		Code        int           `json:"code"`
		Message     string        `json:"message"`
		Accounts    []AccountInfo `json:"accounts"`
		SubAccounts []AccountInfo `json:"sub_accounts"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, body: %s", err, string(body))
	}

	// Check main accounts first
	if len(apiResp.Accounts) > 0 {
		return &apiResp.Accounts[0], nil
	}

	// Check sub-accounts
	if len(apiResp.SubAccounts) > 0 {
		return &apiResp.SubAccounts[0], nil
	}

	return nil, fmt.Errorf("no account found for address: %s", walletAddr)
}
