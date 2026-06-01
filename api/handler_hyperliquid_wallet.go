package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultHyperliquidBuilderAddress = "0x891dc6f05ad47a3c1a05da55e7a7517971faaf0d"
	defaultHyperliquidBuilderMaxFee  = "0.1%"
	hyperliquidExchangeURL           = "https://api.hyperliquid.xyz/exchange"
	hyperliquidInfoURL               = "https://api.hyperliquid.xyz/info"
)

type hyperliquidSubmitRequest struct {
	Action    map[string]any `json:"action" binding:"required"`
	Nonce     int64          `json:"nonce" binding:"required"`
	Signature struct {
		R string `json:"r" binding:"required"`
		S string `json:"s" binding:"required"`
		V int    `json:"v"`
	} `json:"signature" binding:"required"`
}

type hyperliquidConfigResponse struct {
	BuilderAddress string `json:"builderAddress"`
	BuilderMaxFee  string `json:"builderMaxFee"`
	Chain          string `json:"chain"`
	SignatureChain string `json:"signatureChainId"`
}

type hyperliquidAccountSummary struct {
	Address         string  `json:"address"`
	AccountValue    float64 `json:"accountValue"`
	Withdrawable    float64 `json:"withdrawable"`
	TotalMarginUsed float64 `json:"totalMarginUsed"`
	UnrealizedPnl   float64 `json:"unrealizedPnl"`
	OpenPositions   int     `json:"openPositions"`
	UpdatedAt       int64   `json:"updatedAt"`
}

type hyperliquidClearinghouseState struct {
	MarginSummary struct {
		AccountValue    string `json:"accountValue"`
		TotalMarginUsed string `json:"totalMarginUsed"`
	} `json:"marginSummary"`
	CrossMarginSummary struct {
		AccountValue    string `json:"accountValue"`
		TotalMarginUsed string `json:"totalMarginUsed"`
	} `json:"crossMarginSummary"`
	Withdrawable   string `json:"withdrawable"`
	AssetPositions []struct {
		Position struct {
			Szi           string `json:"szi"`
			UnrealizedPnl string `json:"unrealizedPnl"`
		} `json:"position"`
	} `json:"assetPositions"`
}

func hyperliquidBuilderAddress() string {
	return defaultHyperliquidBuilderAddress
}

func hyperliquidBuilderMaxFee() string {
	return defaultHyperliquidBuilderMaxFee
}

func (s *Server) handleHyperliquidConnectConfig(c *gin.Context) {
	c.JSON(http.StatusOK, hyperliquidConfigResponse{
		BuilderAddress: hyperliquidBuilderAddress(),
		BuilderMaxFee:  hyperliquidBuilderMaxFee(),
		Chain:          "Mainnet",
		SignatureChain: "0x66eee",
	})
}

func (s *Server) handleHyperliquidAccount(c *gin.Context) {
	address := strings.ToLower(strings.TrimSpace(c.Query("address")))
	if !isEVMAddress(address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Hyperliquid wallet address"})
		return
	}

	requestBody := map[string]any{
		"type": "clearinghouseState",
		"user": address,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode Hyperliquid balance request"})
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, hyperliquidInfoURL, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create Hyperliquid balance request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach Hyperliquid", "detail": err.Error()})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Hyperliquid rejected the balance request", "status": resp.StatusCode})
		return
	}

	var state hyperliquidClearinghouseState
	if err := json.Unmarshal(respBody, &state); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to parse Hyperliquid balance response"})
		return
	}

	accountValue := parseFloatOrZero(state.MarginSummary.AccountValue)
	if accountValue == 0 {
		accountValue = parseFloatOrZero(state.CrossMarginSummary.AccountValue)
	}
	marginUsed := parseFloatOrZero(state.MarginSummary.TotalMarginUsed)
	if marginUsed == 0 {
		marginUsed = parseFloatOrZero(state.CrossMarginSummary.TotalMarginUsed)
	}

	var unrealizedPnl float64
	openPositions := 0
	for _, position := range state.AssetPositions {
		size := parseFloatOrZero(position.Position.Szi)
		if size != 0 {
			openPositions++
		}
		unrealizedPnl += parseFloatOrZero(position.Position.UnrealizedPnl)
	}

	c.JSON(http.StatusOK, hyperliquidAccountSummary{
		Address:         address,
		AccountValue:    accountValue,
		Withdrawable:    parseFloatOrZero(state.Withdrawable),
		TotalMarginUsed: marginUsed,
		UnrealizedPnl:   unrealizedPnl,
		OpenPositions:   openPositions,
		UpdatedAt:       time.Now().UnixMilli(),
	})
}

func (s *Server) handleHyperliquidSubmitExchange(c *gin.Context) {
	var req hyperliquidSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Hyperliquid submit payload"})
		return
	}

	if err := validateSubmittedNonce(req.Action, req.Nonce); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	actionType, _ := req.Action["type"].(string)
	switch actionType {
	case "approveAgent":
		if err := validateApproveAgentAction(req.Action); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	case "approveBuilderFee":
		if err := validateApproveBuilderFeeAction(req.Action); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported Hyperliquid action"})
		return
	}

	payload := map[string]any{
		"action":    req.Action,
		"nonce":     req.Nonce,
		"signature": req.Signature,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode Hyperliquid payload"})
		return
	}

	client := &http.Client{Timeout: 20 * time.Second}
	hlReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, hyperliquidExchangeURL, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create Hyperliquid request"})
		return
	}
	hlReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(hlReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach Hyperliquid", "detail": err.Error()})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var decoded any
	if len(respBody) > 0 {
		_ = json.Unmarshal(respBody, &decoded)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Hyperliquid rejected the action", "status": resp.StatusCode, "response": decoded})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "response": decoded})
}

func validateApproveAgentAction(action map[string]any) error {
	if strings.TrimSpace(fmt.Sprint(action["agentAddress"])) == "" {
		return fmt.Errorf("missing agentAddress")
	}
	if strings.TrimSpace(fmt.Sprint(action["agentName"])) == "" {
		return fmt.Errorf("missing agentName")
	}
	return validateCommonHyperliquidSignedAction(action)
}

func validateApproveBuilderFeeAction(action map[string]any) error {
	builder := strings.ToLower(strings.TrimSpace(fmt.Sprint(action["builder"])))
	if builder != hyperliquidBuilderAddress() {
		return fmt.Errorf("builder address mismatch")
	}
	if strings.TrimSpace(fmt.Sprint(action["maxFeeRate"])) != hyperliquidBuilderMaxFee() {
		return fmt.Errorf("builder max fee mismatch")
	}
	return validateCommonHyperliquidSignedAction(action)
}

func validateCommonHyperliquidSignedAction(action map[string]any) error {
	if strings.TrimSpace(fmt.Sprint(action["signatureChainId"])) != "0x66eee" {
		return fmt.Errorf("invalid signatureChainId")
	}
	if strings.TrimSpace(fmt.Sprint(action["hyperliquidChain"])) != "Mainnet" {
		return fmt.Errorf("invalid hyperliquidChain")
	}
	if _, err := actionNonce(action); err != nil {
		return err
	}
	return nil
}

func validateSubmittedNonce(action map[string]any, submitted int64) error {
	actionValue, err := actionNonce(action)
	if err != nil {
		return err
	}
	if actionValue != submitted {
		return fmt.Errorf("nonce mismatch")
	}
	return nil
}

func isEVMAddress(address string) bool {
	if len(address) != 42 || !strings.HasPrefix(address, "0x") {
		return false
	}
	for _, char := range address[2:] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func parseFloatOrZero(value string) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return parsed
}

func actionNonce(action map[string]any) (int64, error) {
	raw, ok := action["nonce"]
	if !ok {
		return 0, fmt.Errorf("missing nonce")
	}
	switch value := raw.(type) {
	case float64:
		return int64(value), nil
	case int64:
		return value, nil
	case json.Number:
		return value.Int64()
	case string:
		return strconv.ParseInt(value, 10, 64)
	default:
		return 0, fmt.Errorf("invalid nonce")
	}
}
