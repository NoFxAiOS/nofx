package agent

import (
	"strings"
	"testing"
)

func TestModelProviderChoicePromptIncludesRecommendationWithoutAutoSelection(t *testing.T) {
	msg := modelProviderChoicePrompt("zh")
	for _, want := range []string{
		"可选模型 provider",
		"claw402",
		"DeepSeek",
		"OpenAI",
		"并列可选",
		"blockrun-base",
		"直接创建 Base 钱包",
		"直接扫码充值/支付",
		"请先告诉我你想用哪个 provider",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected prompt to contain %q, got: %s", want, msg)
		}
	}
	if strings.Contains(msg, "把私钥发给我") {
		t.Fatalf("provider choice prompt should not jump ahead to credential collection: %s", msg)
	}
}

func TestModelProviderCredentialGuidanceForClaw402MentionsConfigPageWalletFlow(t *testing.T) {
	msg := modelProviderCredentialGuidance("zh", "claw402")
	for _, want := range []string{
		"Base 链 EVM 钱包私钥",
		"配置页的模型配置里选择 `claw402`",
		"快速创建钱包",
		"充值入口",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected guidance to contain %q, got: %s", want, msg)
		}
	}
}

func TestModelProviderDetailedGuidanceForClaw402MentionsBeginnerFlow(t *testing.T) {
	msg := modelProviderDetailedGuidance("zh", "claw402")
	for _, want := range []string{
		"优先推荐",
		"按次付费",
		"Base USDC 钱包支付",
		"直接创建 Base 钱包",
		"直接扫码充值/支付",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected detailed guidance to contain %q, got: %s", want, msg)
		}
	}
}
