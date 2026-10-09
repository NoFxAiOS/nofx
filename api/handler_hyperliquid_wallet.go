package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultHyperliquidBuilderAddress = "0x891dc6f05ad47a3c1a05da55e7a7517971faaf0d"
	// 0.05% (5 bps) — matches BuilderInfo.Fee=50 charged at order placement.
	// New wallet approvals sign this exact value; existing approvals at the
	// prior 0.1% cap remain valid because 0.05% is within their approved max.
	defaultHyperliquidBuilderMaxFee = "0.05%"
	hyperliquidExchangeURL          = "https://api.hyperliquid.xyz/exchange"
	hyperliquidInfoURL              = "https://api.hyperliquid.xyz/info"
	// nofxHyperliquidAgentName must match AGENT_NAME used by the frontend
	// approveAgent flow so we can locate the NOFX-managed agent on-chain.
	nofxHyperliquidAgentName = "NOFX Agent"
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
	// Spot USDC ("main wallet") balance, so the UI can offer spot->perp funding.
	SpotUSDC          float64 `json:"spotUsdc"`
	SpotUSDCAvailable float64 `json:"spotUsdcAvailable"`
	UpdatedAt         int64   `json:"updatedAt"`
}

type hyperliquidSpotState struct {
	Balances []struct {
		Coin  string `json:"coin"`
		Total string `json:"total"`
		Hold  string `json:"hold"`
	} `json:"balances"`
}

type hyperliquidAgentInfo struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	ValidUntil int64  `json:"validUntil"` // unix milliseconds
}

type hyperliquidAgentResponse struct {
	// Agent is the NOFX-managed agent ("NOFX Agent"), nil when none is approved.
	Agent *hyperliquidAgentInfo `json:"agent"`
	// Agents lists every approved agent for the wallet (for visibility/cleanup).
	Agents []hyperliquidAgentInfo `json:"agents"`
	// BuilderApproved is live on-chain state for the configured NOFX builder.
	BuilderApproved bool `json:"builderApproved"`
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

// agentValidUntilSuffix matches the " valid_until <ms>" suffix Hyperliquid uses
// to encode an agent's expiry inside the agent name. Hyperliquid normally strips
// it from the stored name, but we strip defensively before matching the slot.
var agentValidUntilSuffix = regexp.MustCompile(` valid_until \d+$`)

// hexChainIDPattern matches an EVM chain id in hex form (e.g. "0xa4b1").
var hexChainIDPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{1,16}$`)

func baseAgentName(name string) string {
	return strings.TrimSpace(agentValidUntilSuffix.ReplaceAllString(name, ""))
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

	var state hyperliquidClearinghouseState
	if err := postHyperliquidInfo(c, map[string]any{"type": "clearinghouseState", "user": address}, &state); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to query Hyperliquid balance", "detail": err.Error()})
		return
	}

	// Spot ("main wallet") balance is best-effort: a wallet with no spot assets
	// must not break the perp summary.
	var spotUSDC, spotAvailable float64
	var spotState hyperliquidSpotState
	if err := postHyperliquidInfo(c, map[string]any{"type": "spotClearinghouseState", "user": address}, &spotState); err == nil {
		for _, balance := range spotState.Balances {
			if strings.EqualFold(balance.Coin, "USDC") {
				spotUSDC = parseFloatOrZero(balance.Total)
				spotAvailable = spotUSDC - parseFloatOrZero(balance.Hold)
				if spotAvailable < 0 {
					spotAvailable = 0
				}
			}
		}
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
		Address:           address,
		AccountValue:      accountValue,
		Withdrawable:      parseFloatOrZero(state.Withdrawable),
		TotalMarginUsed:   marginUsed,
		UnrealizedPnl:     unrealizedPnl,
		OpenPositions:     openPositions,
		SpotUSDC:          spotUSDC,
		SpotUSDCAvailable: spotAvailable,
		UpdatedAt:         time.Now().UnixMilli(),
	})
}

// postHyperliquidInfo posts a query to the Hyperliquid info endpoint and
// decodes the JSON response into out.
func postHyperliquidInfo(c *gin.Context, requestBody map[string]any, out any) error {
	body, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, hyperliquidInfoURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("reach Hyperliquid: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Hyperliquid returned status %d", resp.StatusCode)
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	return nil
}

// handleHyperliquidAgent reports the on-chain approved agents for a wallet,
// including the NOFX agent's validUntil so the UI can show the expiry date and
// warn before the 180-day authorization lapses.
func (s *Server) handleHyperliquidAgent(c *gin.Context) {
	address := strings.ToLower(strings.TrimSpace(c.Query("address")))
	if !isEVMAddress(address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Hyperliquid wallet address"})
		return
	}

	body, err := json.Marshal(map[string]any{"type": "extraAgents", "user": address})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode Hyperliquid agent request"})
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, hyperliquidInfoURL, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create Hyperliquid agent request"})
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
		c.JSON(http.StatusBadGateway, gin.H{"error": "Hyperliquid rejected the agent request", "status": resp.StatusCode})
		return
	}

	// extraAgents returns null when no agents are approved.
	agents := []hyperliquidAgentInfo{}
	if len(respBody) > 0 && string(bytes.TrimSpace(respBody)) != "null" {
		if err := json.Unmarshal(respBody, &agents); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to parse Hyperliquid agent response"})
			return
		}
	}

	out := hyperliquidAgentResponse{Agents: agents}
	var maxBuilderFee json.RawMessage
	if err := postHyperliquidInfo(c, map[string]any{
		"type": "maxBuilderFee", "user": address, "builder": hyperliquidBuilderAddress(),
	}, &maxBuilderFee); err == nil {
		fee, parseErr := strconv.ParseFloat(strings.Trim(string(maxBuilderFee), "\""), 64)
		out.BuilderApproved = parseErr == nil && fee >= 50
	}
	for i := range agents {
		if strings.EqualFold(baseAgentName(agents[i].Name), nofxHyperliquidAgentName) {
			agent := agents[i]
			out.Agent = &agent
			break
		}
	}

	c.JSON(http.StatusOK, out)
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
	case "usdClassTransfer":
		if err := validateUsdClassTransferAction(req.Action); err != nil {
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

	// Hyperliquid returns HTTP 200 even for logical failures, signalling them via
	// {"status":"err","response":"<message>"}. Without this check a rejected
	// approval (e.g. valid_until past the cap, or an unchanged agent) is reported
	// to the user as success while nothing changes on-chain.
	var hlResp struct {
		Status   string          `json:"status"`
		Response json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal(respBody, &hlResp); err == nil && strings.EqualFold(hlResp.Status, "err") {
		msg := strings.TrimSpace(strings.Trim(string(hlResp.Response), `"`))
		if msg == "" {
			msg = "Hyperliquid rejected the action"
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": msg, "response": decoded})
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

// validateUsdClassTransferAction guards the spot<->perp transfer relay. The
// signature is produced by the user's own wallet, so the account moved is
// always the signer's; validation only keeps malformed or SDK-style
// subaccount-suffixed amounts from reaching Hyperliquid through NOFX.
func validateUsdClassTransferAction(action map[string]any) error {
	rawAmount, ok := action["amount"].(string)
	if !ok {
		return fmt.Errorf("missing amount")
	}
	amount := strings.TrimSpace(rawAmount)
	if strings.Contains(amount, " ") {
		return fmt.Errorf("invalid amount")
	}
	parsed, err := strconv.ParseFloat(amount, 64)
	if err != nil || parsed <= 0 {
		return fmt.Errorf("amount must be a positive number")
	}
	if _, ok := action["toPerp"].(bool); !ok {
		return fmt.Errorf("missing or invalid toPerp")
	}
	return validateCommonHyperliquidSignedAction(action)
}

func validateCommonHyperliquidSignedAction(action map[string]any) error {
	// signatureChainId is the chain the user's wallet was on when signing —
	// Hyperliquid accepts any value as long as it matches the EIP-712 domain
	// of the signature (docs example: "0xa4b1"); the network is selected by
	// hyperliquidChain, not by this field. Strict EIP-712 wallets (Rabby)
	// require the signing domain to equal the wallet's active chain, so this
	// value cannot be pinned server-side — validate the format only.
	if !hexChainIDPattern.MatchString(strings.TrimSpace(fmt.Sprint(action["signatureChainId"]))) {
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
