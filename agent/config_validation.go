package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"nofx/security"
	"nofx/store"
)

type ConfigValidationResult struct {
	Warnings []string
}

type ConfigValidator interface {
	Validate() error
}

var (
	openAIAPIKeyPattern    = regexp.MustCompile(`^sk-[A-Za-z0-9\-_]{4,}$`)
	genericAPIKeyPattern   = regexp.MustCompile(`^[A-Za-z0-9_\-]{8,}$`)
	hexCredentialPattern   = regexp.MustCompile(`^(0x)?[A-Fa-f0-9]{16,}$`)
	supportedModelProvider = map[string]struct{}{
		"openai": {}, "deepseek": {}, "claude": {}, "gemini": {}, "qwen": {}, "kimi": {}, "grok": {}, "minimax": {}, "claw402": {}, "blockrun-base": {}, "blockrun-sol": {},
	}
)

const (
	manualTraderScanIntervalMin = 3
	manualTraderScanIntervalMax = 60
	manualTraderInitialBalance  = 100.0
	manualLighterAPIKeyIndexMin = 0
	manualLighterAPIKeyIndexMax = 255
)

type modelConfigValidator struct {
	provider        string
	enabled         bool
	apiKey          string
	customAPIURL    string
	customModelName string
	modelID         string
}

func (v modelConfigValidator) Validate() error {
	provider := strings.ToLower(strings.TrimSpace(v.provider))
	if provider == "" {
		return fmt.Errorf("provider is required")
	}
	if _, ok := supportedModelProvider[provider]; !ok {
		return fmt.Errorf("unsupported provider: %s", provider)
	}
	if trimmed := strings.TrimSpace(v.customAPIURL); trimmed != "" {
		if err := security.ValidateURL(strings.TrimSuffix(trimmed, "#")); err != nil {
			return fmt.Errorf("invalid custom_api_url: %w", err)
		}
	}
	if v.enabled && !modelConfigUsable(provider, v.modelID, strings.TrimSpace(v.apiKey), strings.TrimSpace(v.customAPIURL), strings.TrimSpace(v.customModelName)) {
		return fmt.Errorf("cannot enable model config before a usable API key, URL, and model are configured")
	}
	if provider == "openai" && strings.TrimSpace(v.apiKey) != "" && !openAIAPIKeyPattern.MatchString(strings.TrimSpace(v.apiKey)) {
		return fmt.Errorf("OpenAI API Key format looks invalid")
	}
	return nil
}

type exchangeConfigValidator struct {
	exchangeType            string
	enabled                 bool
	apiKey                  string
	secretKey               string
	passphrase              string
	hyperliquidWalletAddr   string
	asterUser               string
	asterSigner             string
	asterPrivateKey         string
	lighterWalletAddr       string
	lighterPrivateKey       string
	lighterAPIKeyPrivateKey string
}

func (v exchangeConfigValidator) Validate() error {
	exchangeType := strings.ToLower(strings.TrimSpace(v.exchangeType))
	if exchangeType == "" {
		return fmt.Errorf("exchange_type is required")
	}
	if trimmed := strings.TrimSpace(v.apiKey); trimmed != "" && !genericAPIKeyPattern.MatchString(trimmed) {
		return fmt.Errorf("API Key format looks invalid")
	}
	if trimmed := strings.TrimSpace(v.secretKey); trimmed != "" && !genericAPIKeyPattern.MatchString(trimmed) && !hexCredentialPattern.MatchString(trimmed) {
		return fmt.Errorf("Secret format looks invalid")
	}
	if exchangeType == "okx" && v.enabled && strings.TrimSpace(v.passphrase) == "" {
		return fmt.Errorf("OKX requires passphrase before enabling this exchange config")
	}
	if exchangeType == "hyperliquid" && v.enabled && strings.TrimSpace(v.hyperliquidWalletAddr) == "" {
		return fmt.Errorf("Hyperliquid requires wallet address before enabling this exchange config")
	}
	if exchangeType == "aster" && v.enabled {
		if strings.TrimSpace(v.asterUser) == "" || strings.TrimSpace(v.asterSigner) == "" || strings.TrimSpace(v.asterPrivateKey) == "" {
			return fmt.Errorf("Aster requires user, signer, and private key before enabling this exchange config")
		}
	}
	if exchangeType == "lighter" && v.enabled {
		if strings.TrimSpace(v.lighterWalletAddr) == "" || strings.TrimSpace(v.lighterAPIKeyPrivateKey) == "" {
			return fmt.Errorf("Lighter requires wallet address and API key private key before enabling this exchange config")
		}
	}
	return nil
}

type traderBindingValidator struct {
	store       *store.Store
	storeUserID string
	aiModelID   string
	exchangeID  string
	strategyID  string
}

func (v traderBindingValidator) Validate() error {
	if v.store == nil {
		return fmt.Errorf("store unavailable")
	}
	if strings.TrimSpace(v.aiModelID) == "" {
		return fmt.Errorf("ai_model_id is required")
	}
	if strings.TrimSpace(v.exchangeID) == "" {
		return fmt.Errorf("exchange_id is required")
	}
	model, err := v.store.AIModel().Get(v.storeUserID, strings.TrimSpace(v.aiModelID))
	if err != nil {
		return fmt.Errorf("invalid ai_model_id: %w", err)
	}
	if !model.Enabled {
		return fmt.Errorf("ai model is disabled")
	}
	if !modelConfigUsable(model.Provider, model.ID, strings.TrimSpace(string(model.APIKey)), strings.TrimSpace(model.CustomAPIURL), strings.TrimSpace(model.CustomModelName)) {
		return fmt.Errorf("ai model config is incomplete")
	}
	exchange, err := v.store.Exchange().GetByID(v.storeUserID, strings.TrimSpace(v.exchangeID))
	if err != nil {
		return fmt.Errorf("invalid exchange_id: %w", err)
	}
	if !exchange.Enabled {
		return fmt.Errorf("exchange is disabled")
	}
	if err := (exchangeConfigValidator{
		exchangeType:            exchange.ExchangeType,
		enabled:                 exchange.Enabled,
		apiKey:                  strings.TrimSpace(string(exchange.APIKey)),
		secretKey:               strings.TrimSpace(string(exchange.SecretKey)),
		passphrase:              strings.TrimSpace(string(exchange.Passphrase)),
		hyperliquidWalletAddr:   exchange.HyperliquidWalletAddr,
		asterUser:               exchange.AsterUser,
		asterSigner:             exchange.AsterSigner,
		asterPrivateKey:         strings.TrimSpace(string(exchange.AsterPrivateKey)),
		lighterWalletAddr:       exchange.LighterWalletAddr,
		lighterPrivateKey:       strings.TrimSpace(string(exchange.LighterPrivateKey)),
		lighterAPIKeyPrivateKey: strings.TrimSpace(string(exchange.LighterAPIKeyPrivateKey)),
	}).Validate(); err != nil {
		return fmt.Errorf("exchange config is incomplete: %w", err)
	}
	if trimmed := strings.TrimSpace(v.strategyID); trimmed != "" {
		if _, err := v.store.Strategy().Get(v.storeUserID, trimmed); err != nil {
			return fmt.Errorf("invalid strategy_id: %w", err)
		}
	}
	return nil
}

func (a *Agent) validateModelDraft(storeUserID, modelID, provider string, enabled bool, apiKey, customAPIURL, customModelName string) error {
	if a == nil || a.store == nil {
		return fmt.Errorf("store unavailable")
	}
	if strings.TrimSpace(provider) == "" && strings.TrimSpace(modelID) != "" {
		model, err := a.store.AIModel().Get(storeUserID, strings.TrimSpace(modelID))
		if err != nil {
			return err
		}
		provider = model.Provider
		if strings.TrimSpace(apiKey) == "" {
			apiKey = strings.TrimSpace(string(model.APIKey))
		}
		if strings.TrimSpace(customAPIURL) == "" {
			customAPIURL = strings.TrimSpace(model.CustomAPIURL)
		}
		if strings.TrimSpace(customModelName) == "" {
			customModelName = strings.TrimSpace(model.CustomModelName)
		}
	}
	return (modelConfigValidator{
		provider:        provider,
		enabled:         enabled,
		apiKey:          apiKey,
		customAPIURL:    customAPIURL,
		customModelName: customModelName,
		modelID:         modelID,
	}).Validate()
}

func (a *Agent) validateExchangeDraft(storeUserID, exchangeID, exchangeType string, enabled bool, apiKey, secretKey, passphrase, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey, lighterWalletAddr, lighterAPIKeyPrivateKey string) error {
	if a == nil || a.store == nil {
		return fmt.Errorf("store unavailable")
	}
	if strings.TrimSpace(exchangeType) == "" && strings.TrimSpace(exchangeID) != "" {
		exchange, err := a.store.Exchange().GetByID(storeUserID, strings.TrimSpace(exchangeID))
		if err != nil {
			return err
		}
		exchangeType = exchange.ExchangeType
		if strings.TrimSpace(apiKey) == "" {
			apiKey = strings.TrimSpace(string(exchange.APIKey))
		}
		if strings.TrimSpace(secretKey) == "" {
			secretKey = strings.TrimSpace(string(exchange.SecretKey))
		}
		if strings.TrimSpace(passphrase) == "" {
			passphrase = strings.TrimSpace(string(exchange.Passphrase))
		}
		if strings.TrimSpace(hyperliquidWalletAddr) == "" {
			hyperliquidWalletAddr = strings.TrimSpace(exchange.HyperliquidWalletAddr)
		}
		if strings.TrimSpace(asterUser) == "" {
			asterUser = strings.TrimSpace(exchange.AsterUser)
		}
		if strings.TrimSpace(asterSigner) == "" {
			asterSigner = strings.TrimSpace(exchange.AsterSigner)
		}
		if strings.TrimSpace(asterPrivateKey) == "" {
			asterPrivateKey = strings.TrimSpace(string(exchange.AsterPrivateKey))
		}
		if strings.TrimSpace(lighterWalletAddr) == "" {
			lighterWalletAddr = strings.TrimSpace(exchange.LighterWalletAddr)
		}
		if strings.TrimSpace(lighterAPIKeyPrivateKey) == "" {
			lighterAPIKeyPrivateKey = strings.TrimSpace(string(exchange.LighterAPIKeyPrivateKey))
		}
	}
	return (exchangeConfigValidator{
		exchangeType:            exchangeType,
		enabled:                 enabled,
		apiKey:                  apiKey,
		secretKey:               secretKey,
		passphrase:              passphrase,
		hyperliquidWalletAddr:   hyperliquidWalletAddr,
		asterUser:               asterUser,
		asterSigner:             asterSigner,
		asterPrivateKey:         asterPrivateKey,
		lighterWalletAddr:       lighterWalletAddr,
		lighterAPIKeyPrivateKey: lighterAPIKeyPrivateKey,
	}).Validate()
}

func (a *Agent) validateTraderDraft(storeUserID, aiModelID, exchangeID, strategyID string) error {
	return (traderBindingValidator{
		store:       a.store,
		storeUserID: storeUserID,
		aiModelID:   aiModelID,
		exchangeID:  exchangeID,
		strategyID:  strategyID,
	}).Validate()
}

func formatValidationFeedback(lang, domain string, err error) string {
	if err == nil {
		return ""
	}
	raw := strings.TrimSpace(err.Error())
	lower := strings.ToLower(raw)
	if lang == "zh" {
		switch {
		case strings.Contains(lower, "openai api key format looks invalid"):
			return "这份配置还有问题：API Key 格式不对。OpenAI 的 API Key 通常以 `sk-` 开头，请直接发完整 Key，我继续帮你补进当前草稿。"
		case strings.Contains(lower, "api key format looks invalid"):
			return "这份配置还有问题：API Key 格式不对。请直接发完整的 API Key，不要附带多余说明文字。"
		case strings.Contains(lower, "secret format looks invalid"):
			return "这份配置还有问题：Secret 格式不对。请直接发完整的 Secret 值，不要和 API Key 填反。"
		case strings.Contains(lower, "okx requires passphrase"):
			return "这份配置还有问题：OKX 账户缺少 Passphrase，启用前需要补齐。你直接把 Passphrase 发我就行。"
		case strings.Contains(lower, "hyperliquid requires wallet address"):
			return "这份配置还有问题：Hyperliquid 账户缺少钱包地址，启用前需要补齐。"
		case strings.Contains(lower, "aster requires user, signer, and private key"):
			return "这份配置还有问题：Aster 账户还缺 user、signer 和 private key，启用前需要补齐。"
		case strings.Contains(lower, "lighter requires wallet address and api key private key"):
			return "这份配置还有问题：Lighter 账户还缺钱包地址和 API key private key，启用前需要补齐。"
		case strings.Contains(lower, "cannot enable model config before a usable api key, url, and model are configured"):
			return "这份配置还有问题：要先把 API Key、接口地址和模型名称配完整，才能启用。你可以继续把缺的字段发给我。"
		case strings.Contains(lower, "unsupported provider"):
			return "这份配置还有问题：provider 不在支持范围内。请从 OpenAI、DeepSeek、Claude、Gemini、Qwen、Kimi、Grok、Minimax 里选一个。"
		case strings.Contains(lower, "invalid custom_api_url"):
			return "这份配置还有问题：接口地址格式不对。请给我完整的 URL，或直接说使用默认地址。"
		case strings.Contains(lower, "ai model is disabled"):
			return "这份配置还有问题：绑定的模型当前是禁用状态。请换一个已启用模型，或先启用这个模型。"
		case strings.Contains(lower, "exchange is disabled"):
			return "这份配置还有问题：绑定的交易所当前已禁用。请换一个已启用交易所，或先启用这个交易所。"
		case strings.Contains(lower, "ai model config is incomplete"):
			return "这份配置还有问题：绑定的模型配置还没补完整，暂时不能使用。"
		case strings.Contains(lower, "invalid ai_model_id"):
			return "这份配置还有问题：模型引用无效。请明确告诉我你要绑定哪个模型。"
		case strings.Contains(lower, "invalid exchange_id"):
			return "这份配置还有问题：交易所引用无效。请明确告诉我你要绑定哪个交易所。"
		case strings.Contains(lower, "invalid strategy_id"):
			return "这份配置还有问题：策略引用无效。请明确告诉我你要绑定哪个策略。"
		case strings.Contains(lower, "provider is required"):
			return "这份配置还缺 provider。请先告诉我你要用哪个模型提供商。"
		case strings.Contains(lower, "exchange_type is required"):
			return "这份配置还缺交易所类型。请先告诉我你要接哪个交易所。"
		}
		switch domain {
		case "model":
			return "这份模型草稿还有问题：" + raw
		case "exchange":
			return "这份交易所草稿还有问题：" + raw
		case "trader":
			return "这份交易员草稿还有问题：" + raw
		case "strategy":
			return "这份策略草稿还有问题：" + raw
		default:
			return "这份配置还有问题：" + raw
		}
	}

	switch {
	case strings.Contains(lower, "openai api key format looks invalid"):
		return "This draft still has an issue: the API key format looks wrong. OpenAI keys usually start with `sk-`. Send the full key and I'll keep filling the draft."
	case strings.Contains(lower, "api key format looks invalid"):
		return "This draft still has an issue: the API key format looks wrong. Send the full API key directly."
	case strings.Contains(lower, "secret format looks invalid"):
		return "This draft still has an issue: the secret format looks wrong. Send the full secret value directly."
	case strings.Contains(lower, "okx requires passphrase"):
		return "This draft still has an issue: an OKX config needs a passphrase before it can be enabled. Send the passphrase and I'll keep going."
	case strings.Contains(lower, "cannot enable model config before a usable api key, url, and model are configured"):
		return "This draft still has an issue: the API key, endpoint URL, and model name must be completed before the config can be enabled."
	}
	switch domain {
	case "model":
		return "This model draft still has an issue: " + raw
	case "exchange":
		return "This exchange draft still has an issue: " + raw
	case "trader":
		return "This trader draft still has an issue: " + raw
	case "strategy":
		return "This strategy draft still has an issue: " + raw
	default:
		return "This draft still has an issue: " + raw
	}
}

func normalizeTraderArgsToManualLimits(lang string, args traderUpdateArgs) (traderUpdateArgs, []string) {
	warnings := make([]string, 0, 2)
	if args.ScanIntervalMinutes != nil {
		requested := *args.ScanIntervalMinutes
		normalized := requested
		if normalized < manualTraderScanIntervalMin {
			normalized = manualTraderScanIntervalMin
		}
		if normalized > manualTraderScanIntervalMax {
			normalized = manualTraderScanIntervalMax
		}
		if normalized != requested {
			args.ScanIntervalMinutes = &normalized
			if lang == "zh" {
				warnings = append(warnings, fmt.Sprintf("扫描间隔手动可配置范围是 %d 到 %d 分钟，已从 %d 调整为 %d", manualTraderScanIntervalMin, manualTraderScanIntervalMax, requested, normalized))
			} else {
				warnings = append(warnings, fmt.Sprintf("scan interval is limited to %d-%d minutes in the manual config, adjusted from %d to %d", manualTraderScanIntervalMin, manualTraderScanIntervalMax, requested, normalized))
			}
		}
	}
	if args.InitialBalance != nil {
		requested := *args.InitialBalance
		normalized := requested
		if normalized < manualTraderInitialBalance {
			normalized = manualTraderInitialBalance
		}
		if normalized != requested {
			args.InitialBalance = &normalized
			if lang == "zh" {
				warnings = append(warnings, fmt.Sprintf("初始资金手动面板最低是 %.2f，已从 %.2f 调整为 %.2f", manualTraderInitialBalance, requested, normalized))
			} else {
				warnings = append(warnings, fmt.Sprintf("initial balance has a manual minimum of %.2f, adjusted from %.2f to %.2f", manualTraderInitialBalance, requested, normalized))
			}
		}
	}
	return args, warnings
}

func formatRiskControlAcceptancePrompt(lang string, warnings []string, confirmLabel string) string {
	if len(warnings) == 0 {
		return ""
	}
	if lang == "zh" {
		lines := []string{
			"这些配置超出了手动面板允许的范围，我已经先按风控范围收敛：",
		}
		for _, warning := range warnings {
			lines = append(lines, "- "+warning)
		}
		lines = append(lines, fmt.Sprintf("如果接受当前范围，回复“%s”；也可以继续告诉我你想怎么改。", confirmLabel))
		return strings.Join(lines, "\n")
	}
	lines := []string{
		"Some values were outside the manual editor limits, so I normalized them first:",
	}
	for _, warning := range warnings {
		lines = append(lines, "- "+warning)
	}
	lines = append(lines, fmt.Sprintf("Reply %q to accept these safe values, or keep refining the draft.", confirmLabel))
	return strings.Join(lines, "\n")
}

func marshalStringList(values []string) string {
	if len(values) == 0 {
		return ""
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(raw)
}

func unmarshalStringList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return values
}

func normalizeExchangePatchToManualLimits(lang string, patch exchangeUpdatePatch) (exchangeUpdatePatch, []string) {
	warnings := make([]string, 0, 1)
	if patch.LighterAPIKeyIndex != nil {
		requested := *patch.LighterAPIKeyIndex
		normalized := requested
		if normalized < manualLighterAPIKeyIndexMin {
			normalized = manualLighterAPIKeyIndexMin
		}
		if normalized > manualLighterAPIKeyIndexMax {
			normalized = manualLighterAPIKeyIndexMax
		}
		if normalized != requested {
			patch.LighterAPIKeyIndex = &normalized
			if lang == "zh" {
				warnings = append(warnings, fmt.Sprintf("Lighter API Key Index 手动面板范围是 %d 到 %d，已从 %d 调整为 %d", manualLighterAPIKeyIndexMin, manualLighterAPIKeyIndexMax, requested, normalized))
			} else {
				warnings = append(warnings, fmt.Sprintf("lighter API key index is limited to %d-%d in the manual editor, adjusted from %d to %d", manualLighterAPIKeyIndexMin, manualLighterAPIKeyIndexMax, requested, normalized))
			}
		}
	}
	return patch, warnings
}
