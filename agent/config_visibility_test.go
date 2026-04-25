package agent

import (
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"nofx/store"
)

func TestToolManageModelConfigCreateRequiresCredential(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "visibility.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	resp := a.toolManageModelConfig("default", `{"action":"create","provider":"deepseek"}`)
	if !strings.Contains(resp, `"error":"api_key is required for create"`) {
		t.Fatalf("expected missing api_key error, got: %s", resp)
	}
}

func TestToolGetModelConfigsHidesIncompleteRows(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "visibility-list.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	if err := st.AIModel().UpdateWithName("default", "default_openai", "OpenAI", false, "", "", ""); err != nil {
		t.Fatalf("seed incomplete model: %v", err)
	}
	if err := st.AIModel().UpdateWithName("default", "default_deepseek", "DeepSeek", false, "sk-test-12345", "", "deepseek-chat"); err != nil {
		t.Fatalf("seed configured model: %v", err)
	}

	resp := a.toolGetModelConfigs("default")
	if strings.Contains(resp, `"id":"default_openai"`) {
		t.Fatalf("incomplete model should be hidden from tool query: %s", resp)
	}
	if !strings.Contains(resp, `"id":"default_deepseek"`) {
		t.Fatalf("configured model should remain visible: %s", resp)
	}
}

func TestExchangeSkillOptionSummaryMatchesManualPage(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "exchange-options.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	summary := a.exchangeSkillOptionSummary("zh")
	for _, expected := range []string{"Binance", "Bybit", "OKX", "Bitget", "Gate", "KuCoin", "Hyperliquid", "Aster", "Lighter", "Indodax"} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("expected option %q in summary, got: %s", expected, summary)
		}
	}
	for _, hidden := range []string{"Alpaca", "Forex", "Metals"} {
		if strings.Contains(summary, hidden) {
			t.Fatalf("did not expect hidden manual-page option %q in summary: %s", hidden, summary)
		}
	}
}

func TestDescribeExchangeIncludesTypeSpecificVisibleFields(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "exchange-detail.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	hyperID, err := st.Exchange().Create("default", "hyperliquid", "Dex Pro", true, "hyper-api-key", "", "", true, "0xabc", true, "", "", "", "", "", "", 0)
	if err != nil {
		t.Fatalf("seed hyperliquid exchange: %v", err)
	}
	detail, ok := a.describeExchange("default", "zh", &EntityReference{ID: hyperID})
	if !ok {
		t.Fatal("expected describeExchange to resolve hyperliquid config")
	}
	for _, expected := range []string{"交易所配置“Dex Pro”详情", "交易所：hyperliquid", "账户名：Dex Pro", "API Key：true", "Hyperliquid 钱包地址：0xabc"} {
		if !strings.Contains(detail, expected) {
			t.Fatalf("expected hyperliquid detail to contain %q, got: %s", expected, detail)
		}
	}

	lighterID, err := st.Exchange().Create("default", "lighter", "Lighter Main", false, "", "", "", false, "", true, "", "", "", "wallet-1", "", "lighter-secret", 7)
	if err != nil {
		t.Fatalf("seed lighter exchange: %v", err)
	}
	detail, ok = a.describeExchange("default", "zh", &EntityReference{ID: lighterID})
	if !ok {
		t.Fatal("expected describeExchange to resolve lighter config")
	}
	for _, expected := range []string{"交易所：lighter", "Lighter 钱包地址：wallet-1", "Lighter API Key 私钥：true", "Lighter API Key Index：7"} {
		if !strings.Contains(detail, expected) {
			t.Fatalf("expected lighter detail to contain %q, got: %s", expected, detail)
		}
	}
}

func TestSkillVisibleFieldSummaryForExchangeUsesReadableNames(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "exchange-field-summary.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	summary := a.skillVisibleFieldSummary("default", "zh", "exchange_management", "update")
	for _, expected := range []string{"交易所类型", "账户名", "API Key", "Secret", "Passphrase", "Hyperliquid 钱包地址", "Aster User", "Lighter API Key 私钥", "Lighter API Key Index"} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("expected field label %q in summary, got: %s", expected, summary)
		}
	}
	if strings.Contains(summary, "hyperliquid_wallet_addr") || strings.Contains(summary, "lighter_api_key_private_key") {
		t.Fatalf("field summary should use readable labels instead of raw keys: %s", summary)
	}
}
