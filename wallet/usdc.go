// Package wallet provides shared wallet utilities (USDC balance queries, etc.)
package wallet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"
)

const (
	BaseRPCURL       = "https://mainnet.base.org"
	USDCContractBase = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
	USDCDecimals     = 6
)

// QueryUSDCBalance queries USDC balance on Base chain and returns as float64
func QueryUSDCBalance(address string) (float64, error) {
	balanceStr := QueryUSDCBalanceStr(address)
	var balance float64
	_, err := fmt.Sscanf(balanceStr, "%f", &balance)
	if err != nil {
		return 0, fmt.Errorf("failed to parse balance: %w", err)
	}
	return balance, nil
}

// QueryUSDCBalanceStr queries USDC balance on Base chain and returns as formatted string
func QueryUSDCBalanceStr(address string) string {
	// Build balanceOf(address) call data
	// Function selector: 0x70a08231
	addrNoPre := strings.TrimPrefix(strings.ToLower(address), "0x")
	data := "0x70a08231" + fmt.Sprintf("%064s", addrNoPre)

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_call",
		"params": []interface{}{
			map[string]string{
				"to":   USDCContractBase,
				"data": data,
			},
			"latest",
		},
		"id": 1,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "0.00"
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(BaseRPCURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return "0.00"
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "0.00"
	}

	var rpcResp struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return "0.00"
	}

	// Parse hex result
	hexStr := strings.TrimPrefix(rpcResp.Result, "0x")
	if hexStr == "" || hexStr == "0" {
		return "0.00"
	}

	balance := new(big.Int)
	balance.SetString(hexStr, 16)

	// Convert to float with 6 decimals
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(USDCDecimals), nil)
	whole := new(big.Int).Div(balance, divisor)
	remainder := new(big.Int).Mod(balance, divisor)

	return fmt.Sprintf("%d.%06d", whole, remainder)
}
