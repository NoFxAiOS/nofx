package kernel

import (
	"strings"
	"testing"

	"nofx/store"
)

func TestBuildSystemPromptUsesVergexClaw402Prompt(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("zh")
	cfg.CoinSource.SourceType = "vergex_signal"
	cfg.CoinSource.VergexLimit = 5
	cfg.PromptSections.RoleDefinition = "# You are a professional Hyperliquid USDC multi-asset trading AI"
	cfg.CustomPrompt = "Long only, no shorts."

	engine := NewStrategyEngine(&cfg)
	prompt := engine.BuildSystemPrompt(30, "balanced")

	if !strings.Contains(prompt, "NOFX Claw402 auto-trader") {
		t.Fatalf("prompt did not use the Claw402/Vergex TradeFi role:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Claw402.ai Direction Board") || !strings.Contains(prompt, "Current Direction and Direction History") || !strings.Contains(prompt, "Cost/Liquidation Heatmap") {
		t.Fatalf("prompt is missing Claw402/Vergex detail data guidance:\n%s", prompt)
	}
	if !strings.Contains(prompt, "open_short") {
		t.Fatalf("prompt should explicitly allow short entries:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Direction is determined only by the current Claw402 ranking") {
		t.Fatalf("prompt should make the current Claw402 direction authoritative:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Existing long + bullish ranking: always `hold`") ||
		!strings.Contains(prompt, "Existing short + bearish ranking: always `hold`") ||
		!strings.Contains(prompt, "Close an existing position only when its ranking direction changes") {
		t.Fatalf("prompt is missing the strict signal hold/exit state machine:\n%s", prompt)
	}
	if !strings.Contains(prompt, "every open position must use exactly 10x") {
		t.Fatalf("prompt should force 10x leverage for Claw402 opens:\n%s", prompt)
	}
	if !strings.Contains(prompt, "use the full max notional per position") {
		t.Fatalf("prompt should force full-size Claw402 opens:\n%s", prompt)
	}
	if !strings.Contains(prompt, "`stop_loss` must be a positive protective price") ||
		!strings.Contains(prompt, "`take_profit` must be exactly 0") {
		t.Fatalf("prompt must explain signal-managed exit fields:\n%s", prompt)
	}
	if containsCJK(prompt) {
		t.Fatalf("system prompt must be English-only, got CJK text:\n%s", prompt)
	}
	legacyPhrases := []string{
		"Hyperliquid USDC multi-asset trading AI",
		"Long only",
		"Altcoin",
		"BTC/ETH",
		"LONG-ONLY",
		"Do not short",
		"MUST open a long",
		"Ranking alone is not an entry reason",
		"Open only when Signal Lab",
		"Claw402.ai Signal Lab",
	}
	for _, phrase := range legacyPhrases {
		if strings.Contains(prompt, phrase) {
			t.Fatalf("prompt still contains legacy phrase %q:\n%s", phrase, prompt)
		}
	}
}

func TestBuildSystemPromptFallsBackToEnglishWhenConfiguredLanguageIsChinese(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("zh")
	cfg.CoinSource.SourceType = "static"
	cfg.CoinSource.StaticCoins = []string{"BTCUSDT", "ETHUSDT"}
	cfg.CoinSource.VergexLimit = 0
	cfg.CoinSource.VergexMarketType = ""
	cfg.CoinSource.VergexChain = ""
	cfg.PromptSections.RoleDefinition = "# You are a Chinese system prompt"
	cfg.PromptSections.TradingFrequency = "# High-frequency trading\nTrade every minute."
	cfg.PromptSections.EntryStandards = "# Entry\nOpen positions freely."
	cfg.PromptSections.DecisionProcess = "# Decision\nOutput directly."
	cfg.CustomPrompt = "Chinese preference should not enter the system prompt."

	engine := NewStrategyEngine(&cfg)
	prompt := engine.BuildSystemPrompt(30, "balanced")

	required := []string{
		"Data Dictionary & Trading Rules",
		"You are a professional Hyperliquid USDC multi-asset trading AI",
		"Trading Frequency Awareness",
		"Entry Standards",
		"Decision Process",
	}
	for _, phrase := range required {
		if !strings.Contains(prompt, phrase) {
			t.Fatalf("English fallback prompt missing %q:\n%s", phrase, prompt)
		}
	}
	if containsCJK(prompt) {
		t.Fatalf("system prompt must be English-only, got CJK text:\n%s", prompt)
	}
}

func TestBuildSystemPromptDoesNotForceLongOnlyForSingleXYZ(t *testing.T) {
	prompt := buildXYZStockCustomPrompt("XYZ:INTC")

	required := []string{
		"DIRECTIONAL, SIGNAL-DRIVEN",
		"You may open long or short",
		"open_short",
	}
	for _, phrase := range required {
		if !strings.Contains(prompt, phrase) {
			t.Fatalf("single XYZ prompt missing %q:\n%s", phrase, prompt)
		}
	}

	forbidden := []string{
		"LONG-ONLY",
		"Do not short",
		"MUST open a long",
		"Probing > waiting",
	}
	for _, phrase := range forbidden {
		if strings.Contains(prompt, phrase) {
			t.Fatalf("single XYZ prompt still contains forced-long phrase %q:\n%s", phrase, prompt)
		}
	}
}

func containsCJK(text string) bool {
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
	}
	return false
}

func TestLegacyVergexFieldsDoNotSelectSignalManagedPrompt(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("en")
	cfg.CoinSource.SourceType = "claw402"
	cfg.CoinSource.VergexLimit = 5
	cfg.CoinSource.VergexMarketType = "all"
	if NewStrategyEngine(&cfg).usesVergexSignalPrompt() {
		t.Fatal("legacy Vergex fields must not select the signal-managed prompt")
	}
}
