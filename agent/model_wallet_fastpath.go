package agent

import (
	"fmt"
	"strconv"
	"strings"
)

func isModelWalletBalanceQuestion(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	// Direct wallet address questions: "我的钱包地址", "wallet address", etc.
	if containsAny(lower, []string{"钱包", "wallet"}) && containsAny(lower, []string{"地址", "address"}) {
		return true
	}
	// Balance questions with wallet context
	if containsAny(lower, []string{"余额", "balance", "usdc"}) &&
		containsAny(lower, []string{"钱包", "wallet", "主钱包", "base", "claw402"}) {
		return true
	}
	return false
}

func (a *Agent) handleModelWalletBalanceQuestion(storeUserID, lang, text string) (string, bool) {
	if !isModelWalletBalanceQuestion(text) || a == nil || a.store == nil {
		return "", false
	}
	models, err := a.store.AIModel().List(storeUserID)
	if err != nil {
		if lang == "zh" {
			return "我现在读取模型配置失败，暂时查不到 claw402 钱包余额。", true
		}
		return "I could not read model configs, so I cannot check the claw402 wallet balance right now.", true
	}

	var matches []safeModelToolConfig
	for _, model := range models {
		if model == nil || strings.ToLower(strings.TrimSpace(model.Provider)) != "claw402" {
			continue
		}
		matches = append(matches, safeModelForTool(model))
	}
	if len(matches) == 0 {
		if lang == "zh" {
			return "当前没有找到 claw402 模型钱包配置。", true
		}
		return "No claw402 model wallet config was found.", true
	}

	if lang == "zh" {
		lines := []string{"当前 claw402 模型钱包余额："}
		for _, model := range matches {
			name := defaultIfEmpty(model.Name, model.ID)
			lines = append(lines, fmt.Sprintf("- %s：%s USDC", name, defaultIfEmpty(model.BalanceUSDC, "暂时无法读取")))
			if strings.TrimSpace(model.WalletAddress) != "" {
				lines = append(lines, fmt.Sprintf("  钱包地址：%s", model.WalletAddress))
			}
			if balanceIsZero(model.BalanceUSDC) {
				if model.Enabled {
					lines = append(lines, "  这个模型配置已启用，但钱包余额为 0 USDC；这不是“未启用”，而是需要先充值 Base USDC 后才能稳定调用。")
				} else {
					lines = append(lines, "  钱包余额为 0 USDC；启用并充值 Base USDC 后才能稳定调用。")
				}
			}
		}
		lines = append(lines, "注意：这是 claw402/Base 模型支付钱包余额，不是 OKX/Binance 等交易所账户余额。")
		return strings.Join(lines, "\n"), true
	}

	lines := []string{"Current claw402 model wallet balance:"}
	for _, model := range matches {
		name := defaultIfEmpty(model.Name, model.ID)
		lines = append(lines, fmt.Sprintf("- %s: %s USDC", name, defaultIfEmpty(model.BalanceUSDC, "unavailable")))
		if strings.TrimSpace(model.WalletAddress) != "" {
			lines = append(lines, fmt.Sprintf("  Wallet address: %s", model.WalletAddress))
		}
		if balanceIsZero(model.BalanceUSDC) {
			lines = append(lines, "  This model config may be enabled, but the wallet balance is 0 USDC; recharge Base USDC before relying on it.")
		}
	}
	lines = append(lines, "Note: this is the claw402/Base model payment wallet balance, not an exchange account balance.")
	return strings.Join(lines, "\n"), true
}

func balanceIsZero(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	parsed, err := strconv.ParseFloat(trimmed, 64)
	return err == nil && parsed <= 0
}
