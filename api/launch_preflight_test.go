package api

import (
	"errors"
	"testing"

	"nofx/crypto"
	"nofx/store"
)

// Well-known throwaway development key (hardhat account #1) — never funded.
const testClaw402Key = "0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"

func withAIWalletBalance(t *testing.T, balance float64, err error) {
	t.Helper()
	original := queryAIWalletBalance
	queryAIWalletBalance = func(string) (float64, error) {
		return balance, err
	}
	t.Cleanup(func() {
		queryAIWalletBalance = original
	})
}

func findCheck(t *testing.T, checks []LaunchCheck, id string) LaunchCheck {
	t.Helper()
	for _, check := range checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("check %q not found in %+v", id, checks)
	return LaunchCheck{}
}

func TestCheckLaunchAIModel(t *testing.T) {
	if got := checkLaunchAIModel(nil); got.Status != launchCheckStatusFailed || got.Code != "MODEL_NOT_FOUND" {
		t.Fatalf("nil model: expected failed/MODEL_NOT_FOUND, got %+v", got)
	}

	disabled := &store.AIModel{Name: "Claw402", Provider: "claw402", Enabled: false}
	if got := checkLaunchAIModel(disabled); got.Code != "MODEL_DISABLED" {
		t.Fatalf("disabled model: expected MODEL_DISABLED, got %+v", got)
	}

	noKey := &store.AIModel{Name: "Claw402", Provider: "claw402", Enabled: true}
	if got := checkLaunchAIModel(noKey); got.Code != "MODEL_MISSING_CREDENTIALS" {
		t.Fatalf("missing key: expected MODEL_MISSING_CREDENTIALS, got %+v", got)
	}

	ready := &store.AIModel{Name: "Claw402", Provider: "claw402", Enabled: true, APIKey: crypto.EncryptedString(testClaw402Key)}
	if got := checkLaunchAIModel(ready); got.Status != launchCheckStatusOK {
		t.Fatalf("ready model: expected ok, got %+v", got)
	}
}

func TestCheckLaunchAIWalletSkipsNonClaw402(t *testing.T) {
	model := &store.AIModel{Name: "DeepSeek", Provider: "deepseek", Enabled: true, APIKey: crypto.EncryptedString("sk-test")}
	checks := checkLaunchAIWallet(model)
	if got := findCheck(t, checks, launchCheckAIWallet); got.Status != launchCheckStatusSkipped {
		t.Fatalf("non-claw402 wallet check should be skipped, got %+v", got)
	}
	if got := findCheck(t, checks, launchCheckAIWalletFunds); got.Status != launchCheckStatusSkipped {
		t.Fatalf("non-claw402 funds check should be skipped, got %+v", got)
	}
}

func TestCheckLaunchAIWalletInvalidKey(t *testing.T) {
	model := &store.AIModel{Name: "Claw402", Provider: "claw402", Enabled: true, APIKey: crypto.EncryptedString("not-a-key")}
	checks := checkLaunchAIWallet(model)
	got := findCheck(t, checks, launchCheckAIWallet)
	if got.Status != launchCheckStatusFailed || got.Code != "AI_WALLET_INVALID_KEY" {
		t.Fatalf("invalid key: expected failed/AI_WALLET_INVALID_KEY, got %+v", got)
	}
	if funds := findCheck(t, checks, launchCheckAIWalletFunds); funds.Status != launchCheckStatusSkipped {
		t.Fatalf("funds check should be skipped when the key is invalid, got %+v", funds)
	}
}

func TestCheckLaunchAIWalletInsufficientFunds(t *testing.T) {
	withAIWalletBalance(t, 0.25, nil)

	model := &store.AIModel{Name: "Claw402", Provider: "claw402", Enabled: true, APIKey: crypto.EncryptedString(testClaw402Key)}
	checks := checkLaunchAIWallet(model)

	wallet := findCheck(t, checks, launchCheckAIWallet)
	if wallet.Status != launchCheckStatusOK || wallet.Address == "" {
		t.Fatalf("wallet check should pass with derived address, got %+v", wallet)
	}

	funds := findCheck(t, checks, launchCheckAIWalletFunds)
	if funds.Status != launchCheckStatusFailed || funds.Code != "AI_WALLET_INSUFFICIENT_FUNDS" {
		t.Fatalf("expected failed/AI_WALLET_INSUFFICIENT_FUNDS, got %+v", funds)
	}
	if funds.Actual == nil || *funds.Actual != 0.25 {
		t.Fatalf("expected actual balance 0.25, got %+v", funds.Actual)
	}
	if funds.Required != MinAIFeeUSDC {
		t.Fatalf("expected required %v, got %v", MinAIFeeUSDC, funds.Required)
	}
	if funds.Address == "" {
		t.Fatalf("funds check should carry the deposit address")
	}
}

func TestCheckLaunchAIWalletRPCOutageIsWarningNotFailure(t *testing.T) {
	withAIWalletBalance(t, 0, errors.New("rpc unreachable"))

	model := &store.AIModel{Name: "Claw402", Provider: "claw402", Enabled: true, APIKey: crypto.EncryptedString(testClaw402Key)}
	checks := checkLaunchAIWallet(model)

	funds := findCheck(t, checks, launchCheckAIWalletFunds)
	if funds.Status != launchCheckStatusWarning || funds.Code != "AI_WALLET_BALANCE_UNKNOWN" {
		t.Fatalf("RPC outage must degrade to warning, got %+v", funds)
	}
}

func TestCheckLaunchAIWalletFunded(t *testing.T) {
	withAIWalletBalance(t, 25.5, nil)

	model := &store.AIModel{Name: "Claw402", Provider: "claw402", Enabled: true, APIKey: crypto.EncryptedString(testClaw402Key)}
	checks := checkLaunchAIWallet(model)

	funds := findCheck(t, checks, launchCheckAIWalletFunds)
	if funds.Status != launchCheckStatusOK {
		t.Fatalf("funded wallet should pass, got %+v", funds)
	}
}

func TestDescribeExchangeConfigIssue(t *testing.T) {
	if _, code := describeExchangeConfigIssue(nil); code != "EXCHANGE_NOT_FOUND" {
		t.Fatalf("nil exchange: expected EXCHANGE_NOT_FOUND, got %s", code)
	}

	disabled := &store.Exchange{ID: "ex", ExchangeType: "hyperliquid", Enabled: false}
	if _, code := describeExchangeConfigIssue(disabled); code != "EXCHANGE_DISABLED" {
		t.Fatalf("disabled: expected EXCHANGE_DISABLED, got %s", code)
	}

	missing := &store.Exchange{ID: "ex", ExchangeType: "hyperliquid", Enabled: true}
	if _, code := describeExchangeConfigIssue(missing); code != "EXCHANGE_MISSING_FIELDS" {
		t.Fatalf("missing fields: expected EXCHANGE_MISSING_FIELDS, got %s", code)
	}

	unapproved := &store.Exchange{
		ID:                    "ex",
		ExchangeType:          "hyperliquid",
		Enabled:               true,
		APIKey:                crypto.EncryptedString(testClaw402Key),
		HyperliquidWalletAddr: "0x1111111111111111111111111111111111111111",
	}
	if _, code := describeExchangeConfigIssue(unapproved); code != "HYPERLIQUID_BUILDER_NOT_APPROVED" {
		t.Fatalf("builder unapproved: expected HYPERLIQUID_BUILDER_NOT_APPROVED, got %s", code)
	}

	unapproved.HyperliquidBuilderApproved = true
	if _, code := describeExchangeConfigIssue(unapproved); code != "" {
		t.Fatalf("ready exchange: expected no issue, got %s", code)
	}
}

func readyHyperliquidExchange() *store.Exchange {
	return &store.Exchange{
		ID:                         "ex-hl",
		ExchangeType:               "hyperliquid",
		Enabled:                    true,
		APIKey:                     crypto.EncryptedString(testClaw402Key),
		HyperliquidWalletAddr:      "0x1111111111111111111111111111111111111111",
		HyperliquidBuilderApproved: true,
	}
}

func preflightTestServer(t *testing.T, userID string, states map[string]ExchangeAccountState) *Server {
	t.Helper()
	server := &Server{exchangeAccountStateCache: NewExchangeAccountStateCache()}
	if states != nil {
		server.exchangeAccountStateCache.Set(userID, states)
	}
	return server
}

func TestCheckLaunchExchangeInsufficientFundsBlocks(t *testing.T) {
	exchange := readyHyperliquidExchange()
	server := preflightTestServer(t, "user-1", map[string]ExchangeAccountState{
		exchange.ID: {ExchangeID: exchange.ID, Status: exchangeAccountStatusOK, AvailableBalance: 5.5, TotalEquity: 5.5},
	})

	checks := server.checkLaunchExchange("user-1", exchange)
	funds := findCheck(t, checks, launchCheckExchangeFunds)
	if funds.Status != launchCheckStatusFailed || funds.Code != "EXCHANGE_INSUFFICIENT_FUNDS" {
		t.Fatalf("expected failed/EXCHANGE_INSUFFICIENT_FUNDS, got %+v", funds)
	}
	if funds.Actual == nil || *funds.Actual != 5.5 {
		t.Fatalf("expected actual 5.5, got %+v", funds.Actual)
	}
}

func TestCheckLaunchExchangeDeployedMarginPasses(t *testing.T) {
	// A running bot with capital locked in positions: low available balance
	// but healthy equity. Restart must not be blocked.
	exchange := readyHyperliquidExchange()
	server := preflightTestServer(t, "user-1", map[string]ExchangeAccountState{
		exchange.ID: {ExchangeID: exchange.ID, Status: exchangeAccountStatusOK, AvailableBalance: 3, TotalEquity: 100},
	})

	checks := server.checkLaunchExchange("user-1", exchange)
	funds := findCheck(t, checks, launchCheckExchangeFunds)
	if funds.Status != launchCheckStatusOK {
		t.Fatalf("deployed margin with healthy equity should pass, got %+v", funds)
	}
	if funds.Actual == nil || *funds.Actual != 100 {
		t.Fatalf("expected actual 100 (equity), got %+v", funds.Actual)
	}
}

func TestCheckLaunchExchangeTestnetLowFundsIsWarning(t *testing.T) {
	exchange := readyHyperliquidExchange()
	exchange.Testnet = true
	server := preflightTestServer(t, "user-1", map[string]ExchangeAccountState{
		exchange.ID: {ExchangeID: exchange.ID, Status: exchangeAccountStatusOK, AvailableBalance: 0},
	})

	checks := server.checkLaunchExchange("user-1", exchange)
	funds := findCheck(t, checks, launchCheckExchangeFunds)
	if funds.Status != launchCheckStatusWarning {
		t.Fatalf("testnet low funds should warn, not block, got %+v", funds)
	}
}

func TestCheckLaunchExchangeInvalidCredentials(t *testing.T) {
	exchange := readyHyperliquidExchange()
	server := preflightTestServer(t, "user-1", map[string]ExchangeAccountState{
		exchange.ID: {
			ExchangeID:   exchange.ID,
			Status:       exchangeAccountStatusInvalidCredentials,
			ErrorCode:    "INVALID_CREDENTIALS",
			ErrorMessage: "Exchange credentials are invalid",
		},
	})

	checks := server.checkLaunchExchange("user-1", exchange)
	account := findCheck(t, checks, launchCheckExchangeAccount)
	if account.Status != launchCheckStatusFailed || account.Code != "INVALID_CREDENTIALS" {
		t.Fatalf("expected failed/INVALID_CREDENTIALS, got %+v", account)
	}
	if funds := findCheck(t, checks, launchCheckExchangeFunds); funds.Status != launchCheckStatusSkipped {
		t.Fatalf("funds check should be skipped when account probe fails, got %+v", funds)
	}
}

func TestRunLaunchPreflightAggregatesReadiness(t *testing.T) {
	withAIWalletBalance(t, 10, nil)

	exchange := readyHyperliquidExchange()
	server := preflightTestServer(t, "user-1", map[string]ExchangeAccountState{
		exchange.ID: {ExchangeID: exchange.ID, Status: exchangeAccountStatusOK, AvailableBalance: 100, TotalEquity: 100},
	})
	model := &store.AIModel{Name: "Claw402", Provider: "claw402", Enabled: true, APIKey: crypto.EncryptedString(testClaw402Key)}
	strategy := &store.Strategy{ID: "strat-1", Name: "Autopilot"}

	result := server.runLaunchPreflight("user-1", model, exchange, strategy, true)
	if !result.Ready {
		t.Fatalf("expected ready, got %+v", result)
	}
	if result.MinAIFeeUSDC != MinAIFeeUSDC || result.MinTradingUSDC != MinTradingUSDC {
		t.Fatalf("minimums must be exposed in the response, got %+v", result)
	}

	// Break one prerequisite → not ready, and Summary explains it.
	withAIWalletBalance(t, 0, nil)
	result = server.runLaunchPreflight("user-1", model, exchange, strategy, true)
	if result.Ready {
		t.Fatalf("expected not ready with empty AI wallet, got %+v", result)
	}
	if result.Summary() == "" {
		t.Fatalf("summary should describe the failing check")
	}
}

func TestRunLaunchPreflightWarningsDoNotBlock(t *testing.T) {
	withAIWalletBalance(t, 0, errors.New("rpc down"))

	exchange := readyHyperliquidExchange()
	server := preflightTestServer(t, "user-1", map[string]ExchangeAccountState{
		exchange.ID: {ExchangeID: exchange.ID, Status: exchangeAccountStatusOK, AvailableBalance: 100},
	})
	model := &store.AIModel{Name: "Claw402", Provider: "claw402", Enabled: true, APIKey: crypto.EncryptedString(testClaw402Key)}

	result := server.runLaunchPreflight("user-1", model, exchange, nil, false)
	if !result.Ready {
		t.Fatalf("warnings must not block launch, got %+v", result)
	}
}
