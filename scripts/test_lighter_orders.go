//go:build ignore

// Test script to verify Lighter API authentication
// Run: go run scripts/test_lighter_orders.go
package main

import (
	"encoding/json"
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
	// Configuration - update these values
	walletAddr := os.Getenv("LIGHTER_WALLET")
	apiKeyPrivateKey := os.Getenv("LIGHTER_API_KEY")

	if walletAddr == "" || apiKeyPrivateKey == "" {
		fmt.Println("Usage: LIGHTER_WALLET=0x... LIGHTER_API_KEY=... go run scripts/test_lighter_orders.go")
		fmt.Println("Environment variables required:")
		fmt.Println("  LIGHTER_WALLET    - Ethereum wallet address")
		fmt.Println("  LIGHTER_API_KEY   - API key private key (40 bytes hex)")
		os.Exit(1)
	}

	fmt.Println("=== Lighter API Test ===")
	fmt.Printf("Wallet: %s\n\n", walletAddr)

	baseURL := "https://mainnet.zklighter.elliot.ai"
	chainID := uint32(304)
	client := &http.Client{Timeout: 30 * time.Second}

	// Step 1: Get account info (no auth required)
	fmt.Println("1. Getting account info...")
	accountIndex, err := getAccountIndex(client, baseURL, walletAddr)
	if err != nil {
		fmt.Printf("   FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   OK: account_index = %d\n\n", accountIndex)

	// Step 2: Create TxClient and generate auth token
	fmt.Println("2. Creating TxClient and generating auth token...")
	httpClient := lighterHTTP.NewClient(baseURL)
	txClient, err := lighterClient.NewTxClient(httpClient, apiKeyPrivateKey, accountIndex, 0, chainID)
	if err != nil {
		fmt.Printf("   FAILED: %v\n", err)
		os.Exit(1)
	}

	authToken, err := txClient.GetAuthToken(time.Now().Add(1 * time.Hour))
	if err != nil {
		fmt.Printf("   FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   OK: auth token generated\n\n")

	// Step 3: Test GetActiveOrders with auth query parameter (NEW method)
	fmt.Println("3. Testing GetActiveOrders with auth query parameter (FIXED)...")
	encodedAuth := url.QueryEscape(authToken)
	endpoint := fmt.Sprintf("%s/api/v1/accountActiveOrders?account_index=%d&market_id=0&auth=%s",
		baseURL, accountIndex, encodedAuth)

	resp, err := client.Get(endpoint)
	if err != nil {
		fmt.Printf("   FAILED: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if code, ok := result["code"].(float64); ok && code == 200 {
		orders := result["orders"].([]interface{})
		fmt.Printf("   OK: Retrieved %d orders\n", len(orders))
		if len(orders) > 0 {
			fmt.Println("   Sample orders:")
			for i, o := range orders {
				if i >= 3 {
					fmt.Printf("   ... and %d more\n", len(orders)-3)
					break
				}
				order := o.(map[string]interface{})
				fmt.Printf("   - ID: %v, Price: %v, Side: %v\n",
					order["order_id"], order["price"], order["is_ask"])
			}
		}
	} else {
		fmt.Printf("   FAILED: %s\n", string(body))
		fmt.Println("\n   Possible causes:")
		fmt.Println("   - API key not registered on-chain")
		fmt.Println("   - API key private key incorrect")
		fmt.Println("   - Account index mismatch")
		os.Exit(1)
	}

	// Step 4: Test GetActiveOrders with Authorization header (OLD method - for comparison)
	fmt.Println("\n4. Testing GetActiveOrders with Authorization header (OLD method)...")
	endpoint2 := fmt.Sprintf("%s/api/v1/accountActiveOrders?account_index=%d&market_id=0",
		baseURL, accountIndex)

	req, _ := http.NewRequest("GET", endpoint2, nil)
	req.Header.Set("Authorization", authToken)
	req.Header.Set("Content-Type", "application/json")

	resp2, err := client.Do(req)
	if err != nil {
		fmt.Printf("   FAILED: %v\n", err)
	} else {
		defer resp2.Body.Close()
		body2, _ := io.ReadAll(resp2.Body)
		var result2 map[string]interface{}
		json.Unmarshal(body2, &result2)

		if code, ok := result2["code"].(float64); ok && code == 200 {
			orders := result2["orders"].([]interface{})
			fmt.Printf("   OK: Retrieved %d orders (both methods work!)\n", len(orders))
		} else {
			fmt.Printf("   FAILED: %s\n", string(body2))
			fmt.Println("   ^ This is expected - Authorization header doesn't work consistently")
		}
	}

	fmt.Println("\n=== TEST COMPLETE ===")
	fmt.Println("If test 3 passed, the fix is working correctly.")
}

func getAccountIndex(client *http.Client, baseURL, walletAddr string) (int64, error) {
	endpoint := fmt.Sprintf("%s/api/v1/account?by=l1_address&value=%s", baseURL, walletAddr)
	resp, err := client.Get(endpoint)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code     int `json:"code"`
		Accounts []struct {
			AccountIndex int64 `json:"account_index"`
		} `json:"accounts"`
		SubAccounts []struct {
			AccountIndex int64 `json:"account_index"`
		} `json:"sub_accounts"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("failed to parse: %w", err)
	}

	if len(result.Accounts) > 0 {
		return result.Accounts[0].AccountIndex, nil
	}
	if len(result.SubAccounts) > 0 {
		return result.SubAccounts[0].AccountIndex, nil
	}

	return 0, fmt.Errorf("no account found")
}
