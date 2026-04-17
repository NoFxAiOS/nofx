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

// QueryUSDCBalance queries USDC balance on Base chain. RPC / decode failures
// are surfaced as errors so callers can distinguish a real zero balance from
// an unreachable RPC.
func QueryUSDCBalance(address string) (float64, error) {
	return queryUSDCBalanceRPC(address)
}

// QueryUSDCBalanceStr is the display-oriented counterpart to QueryUSDCBalance:
// it swallows errors and returns "0.00" so UI handlers always have a string to
// render. Use QueryUSDCBalance when you need to react to failure.
func QueryUSDCBalanceStr(address string) string {
	balance, err := queryUSDCBalanceRPC(address)
	if err != nil {
		return "0.00"
	}
	return fmt.Sprintf("%.6f", balance)
}

func queryUSDCBalanceRPC(address string) (float64, error) {
	// Build balanceOf(address) call data — function selector 0x70a08231.
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
		return 0, fmt.Errorf("marshal rpc payload: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(BaseRPCURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("rpc post: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read rpc response: %w", err)
	}

	var rpcResp struct {
		Result string          `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return 0, fmt.Errorf("decode rpc response: %w", err)
	}
	if len(rpcResp.Error) > 0 && string(rpcResp.Error) != "null" {
		return 0, fmt.Errorf("rpc error: %s", string(rpcResp.Error))
	}

	hexStr := strings.TrimPrefix(rpcResp.Result, "0x")
	if hexStr == "" {
		return 0, nil
	}
	balance, ok := new(big.Int).SetString(hexStr, 16)
	if !ok {
		return 0, fmt.Errorf("invalid hex balance: %q", rpcResp.Result)
	}

	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(USDCDecimals), nil)
	whole := new(big.Int).Quo(balance, divisor)
	remainder := new(big.Int).Mod(balance, divisor)
	// Preserve 6-decimal precision without float drift.
	frac := fmt.Sprintf("%06d", remainder.Int64())
	combined := whole.String() + "." + frac
	var out float64
	if _, err := fmt.Sscanf(combined, "%f", &out); err != nil {
		return 0, fmt.Errorf("parse balance %q: %w", combined, err)
	}
	return out, nil
}
