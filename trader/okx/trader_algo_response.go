package okx

import (
	"encoding/json"
	"fmt"
)

func parseOKXAlgoOrderResponse(resp []byte, action string) (string, error) {
	var orders []struct {
		AlgoId string `json:"algoId"`
		SCode  string `json:"sCode"`
		SMsg   string `json:"sMsg"`
	}
	if err := json.Unmarshal(resp, &orders); err != nil {
		return "", fmt.Errorf("failed to parse OKX %s response: %w", action, err)
	}
	if len(orders) == 0 {
		return "", fmt.Errorf("OKX %s response missing order result", action)
	}
	if orders[0].SCode != "0" {
		return "", fmt.Errorf("OKX %s rejected: code=%s msg=%s", action, orders[0].SCode, orders[0].SMsg)
	}
	return orders[0].AlgoId, nil
}
