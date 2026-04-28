package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"nofx/store"
)

var (
	firstIntegerPattern = regexp.MustCompile(`\d+`)
	firstFloatPattern   = regexp.MustCompile(`\d+(?:\.\d+)?`)
	timeframeTokenRE    = regexp.MustCompile(`(?i)\b\d{1,2}[mhdw]\b`)
	coinSymbolTokenRE   = regexp.MustCompile(`(?i)^(?:xyz:)?[a-z0-9._-]{2,20}(?:usdt|usd|-usdc)?$`)
	quotedContentRE     = regexp.MustCompile(`[“"]([^“”"]{1,200})[”"]`)
)

const (
	strategyPendingUpdateConfigField = "_pending_strategy_update_config"
	strategyPendingUpdateWarnings    = "_pending_strategy_update_warnings"
	strategyPendingUpdateZhMsg       = "_pending_strategy_update_zh_msg"
	strategyPendingUpdateEnMsg       = "_pending_strategy_update_en_msg"
)

func generatedDraftRequiresConfirmation(session skillSession) bool {
	return fieldValue(session, "_requires_generated_confirmation") == "true"
}

func clearGeneratedDraftConfirmation(session *skillSession, keys ...string) {
	if session == nil || session.Fields == nil {
		return
	}
	delete(session.Fields, "_requires_generated_confirmation")
	for _, key := range keys {
		if strings.TrimSpace(key) != "" {
			delete(session.Fields, key)
		}
	}
}

func detectCatalogField(text string, catalog []entityFieldMeta) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return ""
	}
	if strings.Contains(lower, "api key index") || strings.Contains(lower, "lighter api key index") {
		for _, meta := range catalog {
			if meta.Key == "lighter_api_key_index" {
				return meta.Key
			}
		}
	}
	bestKey := ""
	bestLen := -1
	for _, meta := range catalog {
		for _, keyword := range meta.Keywords {
			normalized := strings.ToLower(strings.TrimSpace(keyword))
			if normalized == "" {
				continue
			}
			if entityFieldExplicitlyMentioned(lower, []string{normalized}) && len([]rune(normalized)) > bestLen {
				bestKey = meta.Key
				bestLen = len([]rune(normalized))
			}
		}
	}
	return bestKey
}

func displayCatalogFieldName(field, lang string) string {
	switch field {
	case "name":
		if lang == "zh" {
			return "名称"
		}
		return "name"
	case "ai_model_id":
		if lang == "zh" {
			return "模型"
		}
		return "model"
	case "exchange_id":
		if lang == "zh" {
			return "交易所"
		}
		return "exchange"
	case "strategy_id":
		if lang == "zh" {
			return "策略"
		}
		return "strategy"
	case "initial_balance":
		if lang == "zh" {
			return "初始资金"
		}
		return "initial balance"
	case "scan_interval_minutes":
		if lang == "zh" {
			return "扫描间隔"
		}
		return "scan interval"
	case "is_cross_margin":
		if lang == "zh" {
			return "全仓模式"
		}
		return "cross margin"
	case "show_in_competition":
		if lang == "zh" {
			return "竞技场显示"
		}
		return "show in competition"
	case "enabled":
		if lang == "zh" {
			return "启用状态"
		}
		return "enabled state"
	case "api_key":
		return "API Key"
	case "custom_api_url":
		if lang == "zh" {
			return "接口地址"
		}
		return "API URL"
	case "custom_model_name":
		if lang == "zh" {
			return "模型名称"
		}
		return "model name"
	case "account_name":
		if lang == "zh" {
			return "账户名"
		}
		return "account name"
	case "exchange_type":
		if lang == "zh" {
			return "交易所类型"
		}
		return "exchange type"
	case "secret_key":
		return "Secret"
	case "passphrase":
		return "Passphrase"
	case "testnet":
		if lang == "zh" {
			return "测试网"
		}
		return "testnet"
	case "hyperliquid_wallet_addr":
		if lang == "zh" {
			return "Hyperliquid 钱包地址"
		}
		return "Hyperliquid wallet address"
	case "hyperliquid_unified_account":
		if lang == "zh" {
			return "Hyperliquid Unified Account"
		}
		return "Hyperliquid unified account"
	case "aster_user":
		if lang == "zh" {
			return "Aster User"
		}
		return "Aster user"
	case "aster_signer":
		if lang == "zh" {
			return "Aster Signer"
		}
		return "Aster signer"
	case "aster_private_key":
		if lang == "zh" {
			return "Aster 私钥"
		}
		return "Aster private key"
	case "lighter_wallet_addr":
		if lang == "zh" {
			return "Lighter 钱包地址"
		}
		return "Lighter wallet address"
	case "lighter_private_key":
		if lang == "zh" {
			return "Lighter 私钥"
		}
		return "Lighter private key"
	case "lighter_api_key_private_key":
		if lang == "zh" {
			return "Lighter API Key 私钥"
		}
		return "Lighter API key private key"
	case "lighter_api_key_index":
		if lang == "zh" {
			return "Lighter API Key Index"
		}
		return "Lighter API key index"
	default:
		if lang == "zh" {
			return field
		}
		return field
	}
}

func detectCatalogDomainFromText(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch {
	case containsAny(lower, []string{"策略", "strategy"}):
		return "strategy_management"
	case containsAny(lower, []string{"交易所", "exchange"}):
		return "exchange_management"
	case containsAny(lower, []string{"模型", "model"}):
		return "model_management"
	default:
		return ""
	}
}

func (a *Agent) executeAtomicSkillWithSession(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if answer, ok := a.dispatchBridgedSkillSession(storeUserID, userID, lang, text, session); ok {
		return answer
	}
	return ""
}

func parseLooseTextValue(text string) string {
	return ""
}

func entityFieldExplicitlyMentioned(text string, keywords []string) bool {
	if len(keywords) == 0 {
		return false
	}
	return containsAny(strings.ToLower(text), keywords)
}

type traderUpdateArgs struct {
	AIModelID           string
	ExchangeID          string
	StrategyID          string
	ScanIntervalMinutes *int
	IsCrossMargin       *bool
	ShowInCompetition   *bool
}

func (a traderUpdateArgs) hasAny() bool {
	return a.AIModelID != "" || a.ExchangeID != "" || a.StrategyID != "" ||
		a.ScanIntervalMinutes != nil || a.IsCrossMargin != nil || a.ShowInCompetition != nil
}

func parseStandaloneTraderUpdateArgs(text string) traderUpdateArgs {
	return traderUpdateArgs{}
}

func mergeTraderUpdateArgs(base, patch traderUpdateArgs) traderUpdateArgs {
	if patch.AIModelID != "" {
		base.AIModelID = patch.AIModelID
	}
	if patch.ExchangeID != "" {
		base.ExchangeID = patch.ExchangeID
	}
	if patch.StrategyID != "" {
		base.StrategyID = patch.StrategyID
	}
	if patch.ScanIntervalMinutes != nil {
		base.ScanIntervalMinutes = patch.ScanIntervalMinutes
	}
	if patch.IsCrossMargin != nil {
		base.IsCrossMargin = patch.IsCrossMargin
	}
	if patch.ShowInCompetition != nil {
		base.ShowInCompetition = patch.ShowInCompetition
	}
	return base
}

func applyTraderUpdateArgsToSession(session *skillSession, args traderUpdateArgs) {
	if args.AIModelID != "" {
		setField(session, "ai_model_id", args.AIModelID)
	}
	if args.ExchangeID != "" {
		setField(session, "exchange_id", args.ExchangeID)
	}
	if args.StrategyID != "" {
		setField(session, "strategy_id", args.StrategyID)
	}
	if args.ScanIntervalMinutes != nil {
		setField(session, "scan_interval_minutes", strconv.Itoa(*args.ScanIntervalMinutes))
	}
	if args.IsCrossMargin != nil {
		setField(session, "is_cross_margin", strconv.FormatBool(*args.IsCrossMargin))
	}
	if args.ShowInCompetition != nil {
		setField(session, "show_in_competition", strconv.FormatBool(*args.ShowInCompetition))
	}
}

func buildTraderUpdateArgsFromSession(session skillSession) traderUpdateArgs {
	var args traderUpdateArgs
	args.AIModelID = fieldValue(session, "ai_model_id")
	args.ExchangeID = fieldValue(session, "exchange_id")
	args.StrategyID = fieldValue(session, "strategy_id")
	if value := fieldValue(session, "scan_interval_minutes"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			args.ScanIntervalMinutes = &parsed
		}
	}
	if value := fieldValue(session, "is_cross_margin"); value != "" {
		parsed := value == "true"
		args.IsCrossMargin = &parsed
	}
	if value := fieldValue(session, "show_in_competition"); value != "" {
		parsed := value == "true"
		args.ShowInCompetition = &parsed
	}
	return args
}

type modelUpdatePatch struct {
	Enabled         *bool
	APIKey          string
	CustomAPIURL    string
	CustomModelName string
}

func (p modelUpdatePatch) hasAny() bool {
	return p.Enabled != nil || p.APIKey != "" || p.CustomAPIURL != "" || p.CustomModelName != ""
}

func applyModelUpdatePatchToSession(session *skillSession, patch modelUpdatePatch) {
	if patch.CustomAPIURL != "" {
		setField(session, "custom_api_url", patch.CustomAPIURL)
	}
	if patch.Enabled != nil {
		setField(session, "enabled", strconv.FormatBool(*patch.Enabled))
	}
	if patch.APIKey != "" {
		setField(session, "api_key", patch.APIKey)
	}
	if patch.CustomModelName != "" {
		setField(session, "custom_model_name", patch.CustomModelName)
	}
}

func mergeModelUpdatePatch(base, patch modelUpdatePatch) modelUpdatePatch {
	if patch.Enabled != nil {
		base.Enabled = patch.Enabled
	}
	if patch.APIKey != "" {
		base.APIKey = patch.APIKey
	}
	if patch.CustomAPIURL != "" {
		base.CustomAPIURL = patch.CustomAPIURL
	}
	if patch.CustomModelName != "" {
		base.CustomModelName = patch.CustomModelName
	}
	return base
}

func buildModelUpdatePatchFromSession(session skillSession) modelUpdatePatch {
	var patch modelUpdatePatch
	if value := fieldValue(session, "enabled"); value != "" {
		parsed := value == "true"
		patch.Enabled = &parsed
	}
	patch.APIKey = fieldValue(session, "api_key")
	patch.CustomAPIURL = fieldValue(session, "custom_api_url")
	patch.CustomModelName = fieldValue(session, "custom_model_name")
	return patch
}

type exchangeUpdatePatch struct {
	AccountName             string
	Enabled                 *bool
	APIKey                  string
	SecretKey               string
	Passphrase              string
	Testnet                 *bool
	HyperliquidWalletAddr   string
	AsterUser               string
	AsterSigner             string
	AsterPrivateKey         string
	LighterWalletAddr       string
	LighterAPIKeyPrivateKey string
	LighterAPIKeyIndex      *int
}

func (p exchangeUpdatePatch) hasAny() bool {
	return p.AccountName != "" || p.Enabled != nil || p.APIKey != "" || p.SecretKey != "" ||
		p.Passphrase != "" || p.Testnet != nil || p.HyperliquidWalletAddr != "" || p.AsterUser != "" ||
		p.AsterSigner != "" || p.AsterPrivateKey != "" || p.LighterWalletAddr != "" ||
		p.LighterAPIKeyPrivateKey != "" || p.LighterAPIKeyIndex != nil
}

func applyExchangeUpdatePatchToSession(session *skillSession, patch exchangeUpdatePatch) {
	if patch.AccountName != "" {
		setField(session, "account_name", patch.AccountName)
	}
	if patch.Enabled != nil {
		setField(session, "enabled", strconv.FormatBool(*patch.Enabled))
	}
	if patch.APIKey != "" {
		setField(session, "api_key", patch.APIKey)
	}
	if patch.SecretKey != "" {
		setField(session, "secret_key", patch.SecretKey)
	}
	if patch.Passphrase != "" {
		setField(session, "passphrase", patch.Passphrase)
	}
	if patch.Testnet != nil {
		setField(session, "testnet", strconv.FormatBool(*patch.Testnet))
	}
	if patch.HyperliquidWalletAddr != "" {
		setField(session, "hyperliquid_wallet_addr", patch.HyperliquidWalletAddr)
	}
	if patch.AsterUser != "" {
		setField(session, "aster_user", patch.AsterUser)
	}
	if patch.AsterSigner != "" {
		setField(session, "aster_signer", patch.AsterSigner)
	}
	if patch.AsterPrivateKey != "" {
		setField(session, "aster_private_key", patch.AsterPrivateKey)
	}
	if patch.LighterWalletAddr != "" {
		setField(session, "lighter_wallet_addr", patch.LighterWalletAddr)
	}
	if patch.LighterAPIKeyPrivateKey != "" {
		setField(session, "lighter_api_key_private_key", patch.LighterAPIKeyPrivateKey)
	}
	if patch.LighterAPIKeyIndex != nil {
		setField(session, "lighter_api_key_index", strconv.Itoa(*patch.LighterAPIKeyIndex))
	}
}

func mergeExchangeUpdatePatch(base, patch exchangeUpdatePatch) exchangeUpdatePatch {
	if patch.AccountName != "" {
		base.AccountName = patch.AccountName
	}
	if patch.Enabled != nil {
		base.Enabled = patch.Enabled
	}
	if patch.APIKey != "" {
		base.APIKey = patch.APIKey
	}
	if patch.SecretKey != "" {
		base.SecretKey = patch.SecretKey
	}
	if patch.Passphrase != "" {
		base.Passphrase = patch.Passphrase
	}
	if patch.Testnet != nil {
		base.Testnet = patch.Testnet
	}
	if patch.HyperliquidWalletAddr != "" {
		base.HyperliquidWalletAddr = patch.HyperliquidWalletAddr
	}
	if patch.AsterUser != "" {
		base.AsterUser = patch.AsterUser
	}
	if patch.AsterSigner != "" {
		base.AsterSigner = patch.AsterSigner
	}
	if patch.AsterPrivateKey != "" {
		base.AsterPrivateKey = patch.AsterPrivateKey
	}
	if patch.LighterWalletAddr != "" {
		base.LighterWalletAddr = patch.LighterWalletAddr
	}
	if patch.LighterAPIKeyPrivateKey != "" {
		base.LighterAPIKeyPrivateKey = patch.LighterAPIKeyPrivateKey
	}
	if patch.LighterAPIKeyIndex != nil {
		base.LighterAPIKeyIndex = patch.LighterAPIKeyIndex
	}
	return base
}

func buildExchangeUpdatePatchFromSession(session skillSession) exchangeUpdatePatch {
	var patch exchangeUpdatePatch
	patch.AccountName = fieldValue(session, "account_name")
	if value := fieldValue(session, "enabled"); value != "" {
		parsed := value == "true"
		patch.Enabled = &parsed
	}
	patch.APIKey = fieldValue(session, "api_key")
	patch.SecretKey = fieldValue(session, "secret_key")
	patch.Passphrase = fieldValue(session, "passphrase")
	if value := fieldValue(session, "testnet"); value != "" {
		parsed := value == "true"
		patch.Testnet = &parsed
	}
	patch.HyperliquidWalletAddr = fieldValue(session, "hyperliquid_wallet_addr")
	patch.AsterUser = fieldValue(session, "aster_user")
	patch.AsterSigner = fieldValue(session, "aster_signer")
	patch.AsterPrivateKey = fieldValue(session, "aster_private_key")
	patch.LighterWalletAddr = fieldValue(session, "lighter_wallet_addr")
	patch.LighterAPIKeyPrivateKey = fieldValue(session, "lighter_api_key_private_key")
	if value := fieldValue(session, "lighter_api_key_index"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			patch.LighterAPIKeyIndex = &parsed
		}
	}
	return patch
}

func detectStrategyConfigField(text string) string {
	return ""
}

func strategyConfigFieldDisplayName(field, lang string) string {
	switch field {
	case "name":
		if lang == "zh" {
			return "名称"
		}
		return "name"
	case "strategy_type":
		if lang == "zh" {
			return "策略类型"
		}
		return "strategy type"
	case "symbol":
		if lang == "zh" {
			return "交易对"
		}
		return "symbol"
	case "grid_count":
		if lang == "zh" {
			return "网格数量"
		}
		return "grid count"
	case "total_investment":
		if lang == "zh" {
			return "总投资"
		}
		return "total investment"
	case "upper_price":
		if lang == "zh" {
			return "上沿价格"
		}
		return "upper price"
	case "lower_price":
		if lang == "zh" {
			return "下沿价格"
		}
		return "lower price"
	case "use_atr_bounds":
		if lang == "zh" {
			return "ATR 自动边界"
		}
		return "use ATR bounds"
	case "atr_multiplier":
		if lang == "zh" {
			return "ATR 倍数"
		}
		return "ATR multiplier"
	case "distribution":
		if lang == "zh" {
			return "分布方式"
		}
		return "distribution"
	case "enable_direction_adjust":
		if lang == "zh" {
			return "方向自适应"
		}
		return "enable direction adjust"
	case "direction_bias_ratio":
		if lang == "zh" {
			return "方向偏置比例"
		}
		return "direction bias ratio"
	case "max_drawdown_pct":
		if lang == "zh" {
			return "最大回撤"
		}
		return "max drawdown pct"
	case "stop_loss_pct":
		if lang == "zh" {
			return "止损比例"
		}
		return "stop loss pct"
	case "daily_loss_limit_pct":
		if lang == "zh" {
			return "日亏损限制"
		}
		return "daily loss limit pct"
	case "use_maker_only":
		if lang == "zh" {
			return "仅 Maker"
		}
		return "use maker only"
	case "description":
		if lang == "zh" {
			return "描述"
		}
		return "description"
	case "is_public":
		if lang == "zh" {
			return "发布到市场"
		}
		return "publish to market"
	case "config_visible":
		if lang == "zh" {
			return "配置可见"
		}
		return "config visible"
	case "max_positions":
		if lang == "zh" {
			return "最大持仓"
		}
		return "max positions"
	case "min_confidence":
		if lang == "zh" {
			return "最小置信度"
		}
		return "min confidence"
	case "min_risk_reward_ratio":
		if lang == "zh" {
			return "最小盈亏比"
		}
		return "min risk reward ratio"
	case "leverage":
		if lang == "zh" {
			return "杠杆"
		}
		return "leverage"
	case "btceth_max_leverage":
		if lang == "zh" {
			return "BTC/ETH 最大杠杆"
		}
		return "BTC/ETH max leverage"
	case "altcoin_max_leverage":
		if lang == "zh" {
			return "山寨币最大杠杆"
		}
		return "altcoin max leverage"
	case "btceth_max_position_value_ratio":
		if lang == "zh" {
			return "BTC/ETH 最大仓位价值倍数"
		}
		return "BTC/ETH max position value ratio"
	case "altcoin_max_position_value_ratio":
		if lang == "zh" {
			return "山寨币最大仓位价值倍数"
		}
		return "altcoin max position value ratio"
	case "max_margin_usage":
		if lang == "zh" {
			return "最大保证金使用率"
		}
		return "max margin usage"
	case "min_position_size":
		if lang == "zh" {
			return "最小开仓金额"
		}
		return "min position size"
	case "enable_ema":
		if lang == "zh" {
			return "EMA"
		}
		return "EMA"
	case "enable_macd":
		if lang == "zh" {
			return "MACD"
		}
		return "MACD"
	case "enable_rsi":
		if lang == "zh" {
			return "RSI"
		}
		return "RSI"
	case "enable_atr":
		if lang == "zh" {
			return "ATR"
		}
		return "ATR"
	case "enable_boll":
		if lang == "zh" {
			return "Bollinger"
		}
		return "Bollinger"
	case "enable_all_core_indicators":
		if lang == "zh" {
			return "全部核心指标"
		}
		return "all core indicators"
	case "primary_timeframe":
		if lang == "zh" {
			return "主周期"
		}
		return "primary timeframe"
	case "selected_timeframes":
		if lang == "zh" {
			return "多周期时间框架"
		}
		return "selected timeframes"
	case "source_type":
		if lang == "zh" {
			return "来源类型"
		}
		return "source type"
	case "static_coins":
		if lang == "zh" {
			return "静态币种"
		}
		return "static coins"
	case "excluded_coins":
		if lang == "zh" {
			return "排除币种"
		}
		return "excluded coins"
	case "use_ai500":
		if lang == "zh" {
			return "AI500"
		}
		return "use AI500"
	case "ai500_limit":
		if lang == "zh" {
			return "AI500 数量"
		}
		return "AI500 limit"
	case "use_oi_top":
		if lang == "zh" {
			return "OI Top"
		}
		return "use OI Top"
	case "oi_top_limit":
		if lang == "zh" {
			return "OI Top 数量"
		}
		return "OI Top limit"
	case "use_oi_low":
		if lang == "zh" {
			return "OI Low"
		}
		return "use OI Low"
	case "oi_low_limit":
		if lang == "zh" {
			return "OI Low 数量"
		}
		return "OI Low limit"
	case "primary_count":
		if lang == "zh" {
			return "K线数量"
		}
		return "kline count"
	case "ema_periods":
		return "EMA periods"
	case "rsi_periods":
		return "RSI periods"
	case "atr_periods":
		return "ATR periods"
	case "boll_periods":
		return "BOLL periods"
	case "enable_volume":
		if lang == "zh" {
			return "成交量"
		}
		return "volume"
	case "enable_oi":
		if lang == "zh" {
			return "持仓量"
		}
		return "OI"
	case "enable_funding_rate":
		if lang == "zh" {
			return "资金费率"
		}
		return "funding rate"
	case "nofxos_api_key":
		return "NofxOS API key"
	case "enable_quant_data":
		if lang == "zh" {
			return "量化数据"
		}
		return "quant data"
	case "enable_quant_oi":
		return "quant OI"
	case "enable_quant_netflow":
		return "quant netflow"
	case "enable_oi_ranking":
		return "OI ranking"
	case "oi_ranking_duration":
		return "OI ranking duration"
	case "oi_ranking_limit":
		return "OI ranking limit"
	case "enable_netflow_ranking":
		return "netflow ranking"
	case "netflow_ranking_duration":
		return "netflow ranking duration"
	case "netflow_ranking_limit":
		return "netflow ranking limit"
	case "enable_price_ranking":
		return "price ranking"
	case "price_ranking_duration":
		return "price ranking duration"
	case "price_ranking_limit":
		return "price ranking limit"
	case "role_definition":
		if lang == "zh" {
			return "角色定义"
		}
		return "role definition"
	case "trading_frequency":
		if lang == "zh" {
			return "交易频率"
		}
		return "trading frequency"
	case "entry_standards":
		if lang == "zh" {
			return "开仓标准"
		}
		return "entry standards"
	case "decision_process":
		if lang == "zh" {
			return "决策流程"
		}
		return "decision process"
	case "custom_prompt":
		if lang == "zh" {
			return "自定义 Prompt"
		}
		return "custom prompt"
	default:
		return field
	}
}

func extractStrategyConfigValue(text, field string) (string, bool) {
	return "", false
}

type strategyConfigPatch struct {
	Field string
	Value string
}

func detectStrategyConfigPatches(text string) []strategyConfigPatch {
	return nil
}

func applyStrategyConfigPatch(cfg *store.StrategyConfig, field, value string) error {
	ensureGridConfig := func() *store.GridStrategyConfig {
		if cfg.GridConfig == nil {
			defaults := store.GetDefaultStrategyConfig(cfg.Language)
			if defaults.GridConfig != nil {
				copy := *defaults.GridConfig
				cfg.GridConfig = &copy
			} else {
				cfg.GridConfig = &store.GridStrategyConfig{}
			}
		}
		return cfg.GridConfig
	}

	switch field {
	case "strategy_type":
		cfg.StrategyType = value
	case "symbol":
		ensureGridConfig().Symbol = value
	case "grid_count":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("网格数量需要是整数")
		}
		ensureGridConfig().GridCount = parsed
	case "total_investment":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("总投资需要是数字")
		}
		ensureGridConfig().TotalInvestment = parsed
	case "upper_price":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("上沿价格需要是数字")
		}
		ensureGridConfig().UpperPrice = parsed
	case "lower_price":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("下沿价格需要是数字")
		}
		ensureGridConfig().LowerPrice = parsed
	case "use_atr_bounds":
		ensureGridConfig().UseATRBounds = value == "true"
	case "atr_multiplier":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("ATR 倍数需要是数字")
		}
		ensureGridConfig().ATRMultiplier = parsed
	case "distribution":
		ensureGridConfig().Distribution = value
	case "enable_direction_adjust":
		ensureGridConfig().EnableDirectionAdjust = value == "true"
	case "direction_bias_ratio":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("方向偏置比例需要是数字")
		}
		ensureGridConfig().DirectionBiasRatio = parsed
	case "max_drawdown_pct":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("最大回撤需要是数字")
		}
		ensureGridConfig().MaxDrawdownPct = parsed
	case "stop_loss_pct":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("止损比例需要是数字")
		}
		ensureGridConfig().StopLossPct = parsed
	case "daily_loss_limit_pct":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("日亏损限制需要是数字")
		}
		ensureGridConfig().DailyLossLimitPct = parsed
	case "use_maker_only":
		ensureGridConfig().UseMakerOnly = value == "true"
	case "description", "is_public", "config_visible":
		return nil
	case "max_positions":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("最大持仓需要是整数")
		}
		cfg.RiskControl.MaxPositions = parsed
	case "source_type":
		cfg.CoinSource.SourceType = value
	case "static_coins":
		cfg.CoinSource.StaticCoins = cleanStringList(strings.Split(value, ","))
	case "excluded_coins":
		cfg.CoinSource.ExcludedCoins = cleanStringList(strings.Split(value, ","))
	case "use_ai500":
		cfg.CoinSource.UseAI500 = value == "true"
	case "ai500_limit":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("AI500 数量需要是整数")
		}
		cfg.CoinSource.AI500Limit = parsed
	case "use_oi_top":
		cfg.CoinSource.UseOITop = value == "true"
	case "oi_top_limit":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("OI Top 数量需要是整数")
		}
		cfg.CoinSource.OITopLimit = parsed
	case "use_oi_low":
		cfg.CoinSource.UseOILow = value == "true"
	case "oi_low_limit":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("OI Low 数量需要是整数")
		}
		cfg.CoinSource.OILowLimit = parsed
	case "min_confidence":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("最小置信度需要是整数")
		}
		cfg.RiskControl.MinConfidence = parsed
	case "min_risk_reward_ratio":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("最小盈亏比需要是数字")
		}
		cfg.RiskControl.MinRiskRewardRatio = parsed
	case "leverage":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("杠杆需要是整数")
		}
		cfg.RiskControl.BTCETHMaxLeverage = parsed
		cfg.RiskControl.AltcoinMaxLeverage = parsed
	case "btceth_max_leverage":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("BTC/ETH 最大杠杆需要是整数")
		}
		cfg.RiskControl.BTCETHMaxLeverage = parsed
	case "altcoin_max_leverage":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("山寨币最大杠杆需要是整数")
		}
		cfg.RiskControl.AltcoinMaxLeverage = parsed
	case "btceth_max_position_value_ratio":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("BTC/ETH 仓位价值倍数需要是数字")
		}
		cfg.RiskControl.BTCETHMaxPositionValueRatio = parsed
	case "altcoin_max_position_value_ratio":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("山寨币仓位价值倍数需要是数字")
		}
		cfg.RiskControl.AltcoinMaxPositionValueRatio = parsed
	case "max_margin_usage":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("最大保证金使用率需要是数字")
		}
		cfg.RiskControl.MaxMarginUsage = parsed
	case "min_position_size":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("最小开仓金额需要是数字")
		}
		cfg.RiskControl.MinPositionSize = parsed
	case "primary_timeframe":
		cfg.Indicators.Klines.PrimaryTimeframe = value
	case "primary_count":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("K线数量需要是整数")
		}
		cfg.Indicators.Klines.PrimaryCount = parsed
	case "selected_timeframes":
		tfs := strings.Split(value, ",")
		cfg.Indicators.Klines.SelectedTimeframes = tfs
		cfg.Indicators.Klines.EnableMultiTimeframe = len(tfs) > 1
	case "ema_periods":
		cfg.Indicators.EMAPeriods = parseCSVIntegers(value)
	case "rsi_periods":
		cfg.Indicators.RSIPeriods = parseCSVIntegers(value)
	case "atr_periods":
		cfg.Indicators.ATRPeriods = parseCSVIntegers(value)
	case "boll_periods":
		cfg.Indicators.BOLLPeriods = parseCSVIntegers(value)
	case "enable_ema":
		cfg.Indicators.EnableEMA = value == "true"
	case "enable_macd":
		cfg.Indicators.EnableMACD = value == "true"
	case "enable_rsi":
		cfg.Indicators.EnableRSI = value == "true"
	case "enable_atr":
		cfg.Indicators.EnableATR = value == "true"
	case "enable_boll":
		cfg.Indicators.EnableBOLL = value == "true"
	case "enable_volume":
		cfg.Indicators.EnableVolume = value == "true"
	case "enable_oi":
		cfg.Indicators.EnableOI = value == "true"
	case "enable_funding_rate":
		cfg.Indicators.EnableFundingRate = value == "true"
	case "nofxos_api_key":
		cfg.Indicators.NofxOSAPIKey = value
	case "enable_quant_data":
		cfg.Indicators.EnableQuantData = value == "true"
	case "enable_quant_oi":
		cfg.Indicators.EnableQuantOI = value == "true"
	case "enable_quant_netflow":
		cfg.Indicators.EnableQuantNetflow = value == "true"
	case "enable_oi_ranking":
		cfg.Indicators.EnableOIRanking = value == "true"
	case "oi_ranking_duration":
		cfg.Indicators.OIRankingDuration = value
	case "oi_ranking_limit":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("OI 排行数量需要是整数")
		}
		cfg.Indicators.OIRankingLimit = parsed
	case "enable_netflow_ranking":
		cfg.Indicators.EnableNetFlowRanking = value == "true"
	case "netflow_ranking_duration":
		cfg.Indicators.NetFlowRankingDuration = value
	case "netflow_ranking_limit":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("资金流排行数量需要是整数")
		}
		cfg.Indicators.NetFlowRankingLimit = parsed
	case "enable_price_ranking":
		cfg.Indicators.EnablePriceRanking = value == "true"
	case "price_ranking_duration":
		cfg.Indicators.PriceRankingDuration = value
	case "price_ranking_limit":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("涨跌幅排行数量需要是整数")
		}
		cfg.Indicators.PriceRankingLimit = parsed
	case "role_definition":
		cfg.PromptSections.RoleDefinition = value
	case "trading_frequency":
		cfg.PromptSections.TradingFrequency = value
	case "entry_standards":
		cfg.PromptSections.EntryStandards = value
	case "decision_process":
		cfg.PromptSections.DecisionProcess = value
	case "custom_prompt":
		cfg.CustomPrompt = value
	default:
		return fmt.Errorf("unsupported strategy config field: %s", field)
	}
	return nil
}

func parseSourceTypeValue(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch {
	case containsAny(lower, []string{"静态", "固定", "static"}):
		return "static"
	case containsAny(lower, []string{"ai500"}):
		return "ai500"
	case containsAny(lower, []string{"oi top"}):
		return "oi_top"
	case containsAny(lower, []string{"oi low"}):
		return "oi_low"
	default:
		return ""
	}
}

func extractSymbolList(text string, labels []string) []string {
	segment := extractLongSegmentAfterKeywords(text, labels)
	if segment == "" {
		return nil
	}
	parts := strings.FieldsFunc(segment, func(r rune) bool {
		return r == ',' || r == '，' || r == '、' || r == ' ' || r == '\n' || r == '\t'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if !looksLikeCoinSymbol(part) {
			continue
		}
		part = normalizeCoinSymbol(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return cleanStringList(out)
}

func looksLikeCoinSymbol(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	value = strings.Trim(value, `"'“”‘’()[]{}<>`)
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return coinSymbolTokenRE.MatchString(value)
}

func normalizeCoinSymbol(symbol string) string {
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	if symbol == "" {
		return ""
	}
	if strings.HasPrefix(symbol, "XYZ:") {
		return symbol
	}
	if strings.HasSuffix(symbol, "USDT") || strings.HasSuffix(symbol, "USD") || strings.HasSuffix(symbol, "-USDC") {
		return symbol
	}
	return symbol + "USDT"
}

func extractIntegerList(text string) []string {
	matches := firstIntegerPattern.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}
	return matches
}

func parseCSVIntegers(value string) []int {
	parts := strings.Split(value, ",")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out
}

func extractDurationValue(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch {
	case strings.Contains(lower, "1h,4h,24h"):
		return "1h,4h,24h"
	case strings.Contains(lower, "24h"):
		return "24h"
	case strings.Contains(lower, "4h"):
		return "4h"
	case strings.Contains(lower, "1h"):
		return "1h"
	default:
		return ""
	}
}

func parseStrategyTypeValue(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch {
	case containsAny(lower, []string{"grid", "网格"}):
		return "grid_trading"
	case containsAny(lower, []string{"ai trading", "ai策略", "普通策略"}):
		return "ai_trading"
	default:
		return ""
	}
}

func extractLongSegmentAfterKeywords(text string, keywords []string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	for _, keyword := range keywords {
		idx := strings.Index(lower, strings.ToLower(keyword))
		if idx < 0 {
			continue
		}
		segment := strings.TrimSpace(trimmed[idx+len(keyword):])
		segment = strings.TrimLeft(segment, "“”\"'：: ")
		for _, prefix := range []string{"改成", "改为", "设为", "设置为", "变成"} {
			segment = strings.TrimSpace(strings.TrimPrefix(segment, prefix))
		}
		for _, marker := range []string{"排除币", "excluded coins", "exclude coins", "ai500", "oi top", "oi low", "并且", "然后"} {
			if cut := strings.Index(strings.ToLower(segment), marker); cut > 0 {
				segment = strings.TrimSpace(segment[:cut])
				break
			}
		}
		segment = strings.Trim(segment, "“”\"'：: ")
		if segment != "" {
			return segment
		}
	}
	return ""
}

func extractDelimitedSegmentAfterKeywords(text string, keywords []string) string {
	segment := extractLongSegmentAfterKeywords(text, keywords)
	if segment == "" {
		return ""
	}
	for _, marker := range []string{"，", ",", "。", ".", ";", "；", "\n", "\t", "并且", "然后"} {
		if cut := strings.Index(segment, marker); cut > 0 {
			segment = strings.TrimSpace(segment[:cut])
			break
		}
	}
	return strings.Trim(segment, "“”\"'：: ")
}

func extractModelNameValue(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	if !containsAny(lower, []string{"模型名", "模型名称", "model name"}) {
		return ""
	}
	if value := extractDelimitedSegmentAfterKeywords(text, []string{"model name", "模型名称", "模型名"}); value != "" {
		return value
	}
	if containsAny(lower, []string{"改成", "改为"}) {
		if value := extractDelimitedSegmentAfterKeywords(text, []string{"改成", "改为"}); value != "" {
			return value
		}
	}
	if value := extractQuotedContent(text); value != "" {
		return value
	}
	return ""
}

func sanitizeExtractedURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	for _, marker := range []string{"，", ",", "。", ";", "；", "并且", "然后"} {
		if cut := strings.Index(raw, marker); cut > 0 {
			raw = strings.TrimSpace(raw[:cut])
			break
		}
	}
	return raw
}

func strategyFieldKeywords(field string) []string {
	switch field {
	case "source_type":
		return []string{"来源类型", "source type", "选币来源", "静态来源", "ai500来源", "oi top来源", "oi low来源"}
	case "strategy_type":
		return []string{"策略类型", "strategy type", "网格策略", "grid strategy", "ai策略"}
	case "symbol":
		return []string{"交易对", "symbol", "币对"}
	case "grid_count":
		return []string{"网格数量", "grid count", "grid levels"}
	case "total_investment":
		return []string{"总投入", "总投资", "total investment"}
	case "upper_price":
		return []string{"上沿价格", "上限价格", "upper price"}
	case "lower_price":
		return []string{"下沿价格", "下限价格", "lower price"}
	case "use_atr_bounds":
		return []string{"atr自动边界", "atr边界", "use atr bounds"}
	case "atr_multiplier":
		return []string{"atr倍数", "atr multiplier"}
	case "distribution":
		return []string{"分布方式", "distribution", "均匀分布", "高斯分布", "金字塔分布"}
	case "enable_direction_adjust":
		return []string{"方向调整", "direction adjust"}
	case "direction_bias_ratio":
		return []string{"方向偏置", "bias ratio", "direction bias"}
	case "max_drawdown_pct":
		return []string{"最大回撤", "max drawdown"}
	case "stop_loss_pct":
		return []string{"止损比例", "stop loss"}
	case "daily_loss_limit_pct":
		return []string{"日亏损限制", "daily loss limit"}
	case "use_maker_only":
		return []string{"maker only", "只挂maker", "仅maker"}
	case "description":
		return []string{"描述", "description"}
	case "is_public":
		return []string{"发布到市场", "公开", "publish"}
	case "config_visible":
		return []string{"配置可见", "显示配置", "config visible"}
	case "nofxos_api_key":
		return []string{"nofxos api key", "nofxos key", "api key"}
	case "role_definition":
		return []string{"角色定义", "role definition"}
	case "trading_frequency":
		return []string{"交易频率", "trading frequency"}
	case "entry_standards":
		return []string{"开仓标准", "入场标准", "entry standards"}
	case "decision_process":
		return []string{"决策流程", "decision process"}
	case "custom_prompt":
		return []string{"自定义prompt", "custom prompt", "提示词"}
	case "ema_periods":
		return []string{"ema周期", "ema periods"}
	case "rsi_periods":
		return []string{"rsi周期", "rsi periods"}
	case "atr_periods":
		return []string{"atr周期", "atr periods"}
	case "boll_periods":
		return []string{"boll周期", "布林周期", "boll periods"}
	case "oi_ranking_duration":
		return []string{"oi ranking duration", "oi排行周期"}
	case "netflow_ranking_duration":
		return []string{"netflow ranking duration", "资金流排行周期"}
	case "price_ranking_duration":
		return []string{"price ranking duration", "涨跌幅排行周期"}
	case "oi_ranking_limit":
		return []string{"oi ranking limit", "oi排行数量"}
	case "netflow_ranking_limit":
		return []string{"netflow ranking limit", "资金流排行数量"}
	case "price_ranking_limit":
		return []string{"price ranking limit", "涨跌幅排行数量"}
	case "btceth_max_position_value_ratio":
		return []string{"btc/eth仓位价值倍数", "btc eth position value", "主流币仓位价值倍数"}
	case "altcoin_max_position_value_ratio":
		return []string{"山寨币仓位价值倍数", "altcoin position value"}
	case "max_margin_usage":
		return []string{"最大保证金使用率", "max margin usage"}
	default:
		return nil
	}
}

func matchesStrategyFieldKeywords(text, field string) bool {
	keywords := strategyFieldKeywords(field)
	if len(keywords) == 0 {
		return true
	}
	return containsAny(strings.ToLower(text), keywords)
}

func strategyFieldExplicitlyMentioned(text, field string) bool {
	keywords := strategyFieldKeywords(field)
	if len(keywords) == 0 {
		switch field {
		case "max_positions":
			keywords = []string{"最大持仓", "最多持仓", "max positions"}
		case "symbol":
			keywords = []string{"交易对", "symbol", "币对"}
		case "grid_count":
			keywords = []string{"网格数量", "grid count", "grid levels"}
		case "total_investment":
			keywords = []string{"总投入", "总投资", "total investment"}
		case "upper_price":
			keywords = []string{"上沿价格", "上限价格", "upper price"}
		case "lower_price":
			keywords = []string{"下沿价格", "下限价格", "lower price"}
		case "use_atr_bounds":
			keywords = []string{"atr自动边界", "atr边界", "use atr bounds"}
		case "atr_multiplier":
			keywords = []string{"atr倍数", "atr multiplier"}
		case "distribution":
			keywords = []string{"分布方式", "distribution", "均匀分布", "高斯分布", "金字塔分布"}
		case "enable_direction_adjust":
			keywords = []string{"方向调整", "direction adjust"}
		case "direction_bias_ratio":
			keywords = []string{"方向偏置", "bias ratio", "direction bias"}
		case "max_drawdown_pct":
			keywords = []string{"最大回撤", "max drawdown"}
		case "stop_loss_pct":
			keywords = []string{"止损比例", "stop loss"}
		case "daily_loss_limit_pct":
			keywords = []string{"日亏损限制", "daily loss limit"}
		case "use_maker_only":
			keywords = []string{"maker only", "只挂maker", "仅maker"}
		case "min_confidence":
			keywords = []string{"最低置信度", "最小置信度", "min confidence"}
		case "min_risk_reward_ratio":
			keywords = []string{"最小盈亏比", "风险回报比", "risk reward", "risk/reward"}
		case "leverage":
			keywords = []string{"杠杆", "leverage"}
		case "btceth_max_leverage":
			keywords = []string{"btc/eth杠杆", "btc eth杠杆", "btc/eth leverage", "btc eth leverage", "主流币杠杆"}
		case "altcoin_max_leverage":
			keywords = []string{"山寨币杠杆", "altcoin leverage", "alts leverage"}
		case "btceth_max_position_value_ratio":
			keywords = []string{"btc/eth仓位价值倍数", "btc eth position value", "主流币仓位价值倍数"}
		case "altcoin_max_position_value_ratio":
			keywords = []string{"山寨币仓位价值倍数", "altcoin position value"}
		case "max_margin_usage":
			keywords = []string{"最大保证金使用率", "max margin usage"}
		case "primary_timeframe":
			keywords = []string{"主周期", "主时间周期", "primary timeframe"}
		case "primary_count":
			keywords = []string{"k线数量", "k线根数", "primary count", "kline count"}
		case "selected_timeframes":
			keywords = []string{"多周期", "时间框架", "timeframes", "selected timeframes"}
		case "enable_ema":
			keywords = []string{"ema"}
		case "enable_macd":
			keywords = []string{"macd"}
		case "enable_rsi":
			keywords = []string{"rsi"}
		case "enable_atr":
			keywords = []string{"atr"}
		case "enable_boll":
			keywords = []string{"boll", "bollinger", "布林"}
		case "enable_volume":
			keywords = []string{"成交量", "volume"}
		case "enable_oi":
			keywords = []string{"持仓量", "open interest", "oi"}
		case "enable_funding_rate":
			keywords = []string{"资金费率", "funding rate"}
		case "source_type":
			keywords = []string{"来源类型", "source type", "选币来源"}
		case "static_coins":
			keywords = []string{"静态币", "固定币", "static coins", "static symbols"}
		case "excluded_coins":
			keywords = []string{"排除币", "排除币种", "excluded coins", "exclude coins"}
		case "use_ai500":
			keywords = []string{"ai500"}
		case "ai500_limit":
			keywords = []string{"ai500 limit", "ai500数量", "ai500上限"}
		case "use_oi_top":
			keywords = []string{"oi top", "持仓量增长", "持仓量排行上涨"}
		case "oi_top_limit":
			keywords = []string{"oi top limit", "oi top数量", "oi top上限"}
		case "use_oi_low":
			keywords = []string{"oi low", "持仓量下降", "持仓量排行下跌"}
		case "oi_low_limit":
			keywords = []string{"oi low limit", "oi low数量", "oi low上限"}
		case "enable_all_core_indicators":
			keywords = []string{"核心指标"}
		}
	}
	if len(keywords) == 0 {
		return false
	}
	return containsAny(strings.ToLower(text), keywords)
}

func (a *Agent) executeTraderManagementAction(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if session.Action == "query_strategy_binding" || session.Action == "query_exchange_binding" || session.Action == "query_model_binding" {
		if detail, ok := a.describeTrader(storeUserID, lang, session.TargetRef); ok {
			return detail
		}
		return formatReadFastPathResponse(lang, "list_traders", a.toolListTraders(storeUserID))
	}
	switch session.Action {
	case "query", "query_list":
		return formatReadFastPathResponse(lang, "list_traders", a.toolListTraders(storeUserID))
	case "query_detail":
		if detail, ok := a.describeTrader(storeUserID, lang, session.TargetRef); ok {
			return detail
		}
		return formatReadFastPathResponse(lang, "list_traders", a.toolListTraders(storeUserID))
	case "start", "stop", "delete":
		if fieldValue(session, skillDAGStepField) == "" {
			setSkillDAGStep(&session, "await_confirmation")
		}
		if session.Action == "delete" && fieldValue(session, "bulk_scope") == "all" {
			return a.executeBulkTraderDelete(storeUserID, userID, lang, text, session)
		}
		if msg, waiting := a.beginConfirmationIfNeeded(userID, lang, &session, defaultIfEmpty(session.TargetRef.Name, session.TargetRef.ID)); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		if msg, waiting := awaitingConfirmationButNotApproved(lang, session, text); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		var resp string
		switch session.Action {
		case "start":
			setSkillDAGStep(&session, "execute_start")
			resp = a.toolStartTrader(storeUserID, session.TargetRef.ID)
		case "stop":
			setSkillDAGStep(&session, "execute_stop")
			resp = a.toolStopTrader(storeUserID, session.TargetRef.ID)
		case "delete":
			setSkillDAGStep(&session, "execute_delete")
			resp = a.toolDeleteTrader(storeUserID, session.TargetRef.ID)
		}
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "执行失败：" + errMsg
			}
			return "Action failed: " + errMsg
		}
		if lang == "zh" {
			return fmt.Sprintf("已完成交易员操作：%s。", session.Action)
		}
		return fmt.Sprintf("Completed trader action: %s.", session.Action)
	case "update", "update_bindings", "configure_strategy", "configure_exchange", "configure_model":
		if session.Action == "update_bindings" || session.Action == "configure_strategy" || session.Action == "configure_exchange" || session.Action == "configure_model" {
			if fieldValue(session, skillDAGStepField) == "" {
				setSkillDAGStep(&session, "collect_bindings")
			}
			args := manageTraderArgs{
				Action:     "update",
				TraderID:   session.TargetRef.ID,
				AIModelID:  fieldValue(session, "ai_model_id"),
				ExchangeID: fieldValue(session, "exchange_id"),
				StrategyID: fieldValue(session, "strategy_id"),
			}
			if args.AIModelID != "" {
				setField(&session, "ai_model_id", args.AIModelID)
			}
			if args.ExchangeID != "" {
				setField(&session, "exchange_id", args.ExchangeID)
			}
			if args.StrategyID != "" {
				setField(&session, "strategy_id", args.StrategyID)
			}
			selectedField := fieldValue(session, "update_field")
			if selectedField == "" {
				switch session.Action {
				case "configure_strategy":
					selectedField = "strategy_id"
				case "configure_exchange":
					selectedField = "exchange_id"
				case "configure_model":
					selectedField = "ai_model_id"
				default:
					if args.AIModelID == "" && args.ExchangeID == "" && args.StrategyID == "" {
						selectedField = detectCatalogField(text, traderFieldCatalog)
					}
				}
				if selectedField == "name" || selectedField == "scan_interval_minutes" || selectedField == "is_cross_margin" || selectedField == "show_in_competition" {
					selectedField = ""
				}
				if selectedField != "" {
					setField(&session, "update_field", selectedField)
				}
			}
			if args.AIModelID == "" && args.ExchangeID == "" && args.StrategyID == "" {
				if fieldValue(session, "inline_sub_intent") == "create_sub_resource" {
					delete(session.Fields, "inline_sub_intent")
					a.saveSkillSession(userID, session)
					task := a.buildSuspendedTask(userID, lang)
					if task.Kind != "" && task.SkillSession != nil {
						task.ResumeOnSuccess = true
						var childSkill, childResumeTrigger string
						switch session.Action {
						case "configure_strategy":
							childSkill = "strategy_management"
							childResumeTrigger = "strategy_management"
						case "configure_exchange":
							childSkill = "exchange_management"
							childResumeTrigger = "exchange_management"
						case "configure_model":
							childSkill = "model_management"
							childResumeTrigger = "model_management"
						case "create":
							// infer child skill from which binding slot is missing
							slots := session.Slots
							if slots == nil || slots.StrategyID == "" {
								childSkill = "strategy_management"
								childResumeTrigger = "strategy_management"
							} else if slots.ExchangeID == "" {
								childSkill = "exchange_management"
								childResumeTrigger = "exchange_management"
							} else if slots.ModelID == "" {
								childSkill = "model_management"
								childResumeTrigger = "model_management"
							}
						}
						if childSkill != "" {
							task.ResumeTriggers = []string{childResumeTrigger}
							a.SnapshotManager(userID).Save(task)
							a.clearSkillSession(userID)
							child := skillSession{Name: childSkill, Action: "create", Phase: "collecting"}
							var answer string
							var handled bool
							switch childSkill {
							case "strategy_management":
								answer, handled = a.handleStrategyManagementSkill(storeUserID, userID, lang, text, child)
							case "exchange_management":
								answer, handled = a.handleExchangeManagementSkill(storeUserID, userID, lang, text, child)
							case "model_management":
								answer, handled = a.handleModelManagementSkill(storeUserID, userID, lang, text, child)
							}
							if !handled {
								answer = ""
							}
							return a.maybeResumeParentTaskAfterSuccessfulSkill(storeUserID, userID, lang, childSkill, "create", answer)
						}
					}
				}
				if fieldValue(session, "inline_sub_intent") == "edit_sub_resource" {
					delete(session.Fields, "inline_sub_intent")
					a.saveSkillSession(userID, session)
					task := a.buildSuspendedTask(userID, lang)
					if task.Kind != "" && task.SkillSession != nil {
						task.ResumeOnSuccess = true
						var childSkill string
						switch session.Action {
						case "configure_strategy":
							childSkill = "strategy_management"
						case "configure_exchange":
							childSkill = "exchange_management"
						case "configure_model":
							childSkill = "model_management"
						case "create", "update_bindings":
							childSkill = detectCatalogDomainFromText(text)
						}
						if childSkill != "" {
							task.ResumeTriggers = []string{childSkill}
							a.SnapshotManager(userID).Save(task)
							a.clearSkillSession(userID)
							child := skillSession{Name: childSkill, Action: "update", Phase: "collecting"}
							var answer string
							var handled bool
							switch childSkill {
							case "strategy_management":
								answer, handled = a.handleStrategyManagementSkill(storeUserID, userID, lang, text, child)
							case "exchange_management":
								answer, handled = a.handleExchangeManagementSkill(storeUserID, userID, lang, text, child)
							case "model_management":
								answer, handled = a.handleModelManagementSkill(storeUserID, userID, lang, text, child)
							}
							if !handled {
								answer = ""
							}
							return a.maybeResumeParentTaskAfterSuccessfulSkill(storeUserID, userID, lang, childSkill, "update", answer)
						}
					}
				}
				setSkillDAGStep(&session, "collect_bindings")
				a.saveSkillSession(userID, session)
				if lang == "zh" {
					if selectedField != "" {
						return fmt.Sprintf("还差一步：请告诉我你想换成哪个%s。", displayCatalogFieldName(selectedField, lang))
					}
					switch session.Action {
					case "configure_strategy":
						return "好，我来帮你换策略。直接告诉我想用哪个策略就行。"
					case "configure_exchange":
						return "好，我来帮你换交易所。直接告诉我想用哪个交易所就行。"
					case "configure_model":
						return "好，我来帮你换模型。直接告诉我想用哪个模型就行。"
					default:
						return "好，我来帮你调整交易员绑定。你直接告诉我想换成哪个模型、交易所或策略就行。"
					}
				}
				if selectedField != "" {
					return fmt.Sprintf("One more thing: tell me which %s you want to use.", displayCatalogFieldName(selectedField, lang))
				}
				switch session.Action {
				case "configure_strategy":
					return "Sure. Tell me which strategy you want to use."
				case "configure_exchange":
					return "Sure. Tell me which exchange you want to use."
				case "configure_model":
					return "Sure. Tell me which model you want to use."
				default:
					return "Sure. Tell me which model, exchange, or strategy you want to switch to."
				}
			}
			setSkillDAGStep(&session, "execute_update")
			resp := a.toolUpdateTrader(storeUserID, args)
			a.clearSkillSession(userID)
			if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
				if lang == "zh" {
					return "这次没改成功：" + errMsg
				}
				return "That change did not go through: " + errMsg
			}
			a.rememberReferencesFromToolResult(userID, "manage_trader", resp)
			if lang == "zh" {
				switch session.Action {
				case "configure_strategy":
					return "已更新交易员策略。"
				case "configure_exchange":
					return "已更新交易员交易所。"
				case "configure_model":
					return "已更新交易员模型。"
				default:
					return "已更新交易员绑定。"
				}
			}
			switch session.Action {
			case "configure_strategy":
				return "Updated the trader strategy."
			case "configure_exchange":
				return "Updated the trader exchange."
			case "configure_model":
				return "Updated the trader model."
			default:
				return "Updated trader bindings."
			}
		}
		if fieldValue(session, skillDAGStepField) == "" {
			setSkillDAGStep(&session, "collect_name")
		}
		parsedArgs := buildTraderUpdateArgsFromSession(session)
		selectedField := fieldValue(session, "update_field")
		if selectedField == "" {
			if !parsedArgs.hasAny() {
				selectedField = detectCatalogField(text, traderFieldCatalog)
			}
			if selectedField != "" {
				setField(&session, "update_field", selectedField)
			}
		}
		applyTraderUpdateArgsToSession(&session, parsedArgs)
		parsedArgs = mergeTraderUpdateArgs(buildTraderUpdateArgsFromSession(session), parsedArgs)
		if parsedArgs.hasAny() {
			normalizedArgs, warnings := normalizeTraderArgsToManualLimits(lang, parsedArgs)
			applyTraderUpdateArgsToSession(&session, normalizedArgs)
			args := manageTraderArgs{
				Action:              "update",
				TraderID:            session.TargetRef.ID,
				AIModelID:           normalizedArgs.AIModelID,
				ExchangeID:          normalizedArgs.ExchangeID,
				StrategyID:          normalizedArgs.StrategyID,
				ScanIntervalMinutes: normalizedArgs.ScanIntervalMinutes,
				IsCrossMargin:       normalizedArgs.IsCrossMargin,
				ShowInCompetition:   normalizedArgs.ShowInCompetition,
			}
			setSkillDAGStep(&session, "execute_update")
			resp := a.toolUpdateTrader(storeUserID, args)
			a.clearSkillSession(userID)
			if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
				if lang == "zh" {
					return "这次没改成功：" + errMsg
				}
				return "That change did not go through: " + errMsg
			}
			if lang == "zh" {
				reply := "已更新交易员配置。"
				if len(warnings) > 0 {
					reply += "\n\n已按手动面板范围自动调整：\n- " + strings.Join(warnings, "\n- ")
				}
				return reply
			}
			reply := "Updated trader config."
			if len(warnings) > 0 {
				reply += "\n\nAdjusted to stay within the manual editor limits:\n- " + strings.Join(warnings, "\n- ")
			}
			return reply
		}
		if selectedField != "" {
			setSkillDAGStep(&session, "collect_field_value")
		} else {
			setSkillDAGStep(&session, "collect_name")
		}
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			if selectedField != "" {
				if selectedField == "ai_model_id" || selectedField == "exchange_id" || selectedField == "strategy_id" {
					return fmt.Sprintf("还差一步：请告诉我你想换成哪个%s。", displayCatalogFieldName(selectedField, lang))
				}
				return fmt.Sprintf("还差一步：请告诉我新的%s。", displayCatalogFieldName(selectedField, lang))
			}
			return "你可以直接告诉我想改哪一项，比如绑定的模型、交易所、策略，或者扫描间隔、保证金模式、是否展示到竞技场。若你要改策略参数、模型配置或交易所凭证，我会切到对应配置流程。"
		}
		if selectedField != "" {
			if selectedField == "ai_model_id" || selectedField == "exchange_id" || selectedField == "strategy_id" {
				return fmt.Sprintf("One more thing: tell me which %s you want to use.", displayCatalogFieldName(selectedField, lang))
			}
			return fmt.Sprintf("One more thing: tell me the new %s.", displayCatalogFieldName(selectedField, lang))
		}
		return "Tell me what you want to change first, for example the linked model, exchange, strategy, scan interval, margin mode, or competition visibility. If you want to edit the internals of a strategy, model, or exchange, I'll switch to the right config flow."
	default:
		return ""
	}
}

func (a *Agent) executeBulkTraderDelete(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if a == nil || a.store == nil {
		if lang == "zh" {
			return "我这边暂时无法读取交易员列表。"
		}
		return "I cannot load the trader list right now."
	}
	traders, err := a.store.Trader().List(storeUserID)
	if err != nil {
		if lang == "zh" {
			return "我这边暂时没读到交易员列表：" + err.Error()
		}
		return "I could not load the trader list just now: " + err.Error()
	}
	if len(traders) == 0 {
		a.clearSkillSession(userID)
		if lang == "zh" {
			return "当前没有可删除的交易员。"
		}
		return "There are no traders to delete."
	}

	deletable := make([]*store.Trader, 0, len(traders))
	runningNames := make([]string, 0)
	for _, trader := range traders {
		if trader == nil {
			continue
		}
		isRunning := trader.IsRunning
		if a.traderManager != nil {
			if memTrader, err := a.traderManager.GetTrader(trader.ID); err == nil {
				if running, ok := memTrader.GetStatus()["is_running"].(bool); ok {
					isRunning = running
				}
			}
		}
		if isRunning {
			runningNames = append(runningNames, defaultIfEmpty(trader.Name, trader.ID))
			continue
		}
		deletable = append(deletable, trader)
	}

	if len(deletable) == 0 {
		a.clearSkillSession(userID)
		if lang == "zh" {
			return "当前所有交易员都还在运行中，删除前需要先停止：" + strings.Join(runningNames, "、")
		}
		return "All traders are still running. Stop them before deleting: " + strings.Join(runningNames, ", ")
	}

	targetLabel := fmt.Sprintf("全部已停止交易员（共 %d 个）", len(deletable))
	if msg, waiting := a.beginConfirmationIfNeeded(userID, lang, &session, targetLabel); waiting {
		a.saveSkillSession(userID, session)
		return msg
	}
	if msg, waiting := awaitingConfirmationButNotApproved(lang, session, text); waiting {
		a.saveSkillSession(userID, session)
		return msg
	}

	setSkillDAGStep(&session, "execute_delete")
	deletedNames := make([]string, 0, len(deletable))
	failedNames := make([]string, 0)
	for _, trader := range deletable {
		resp := a.toolDeleteTrader(storeUserID, trader.ID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			failedNames = append(failedNames, fmt.Sprintf("%s（%s）", defaultIfEmpty(trader.Name, trader.ID), errMsg))
			continue
		}
		deletedNames = append(deletedNames, defaultIfEmpty(trader.Name, trader.ID))
	}
	a.clearSkillSession(userID)

	if lang == "zh" {
		parts := []string{fmt.Sprintf("批量删除交易员已完成：成功删除 %d 个。", len(deletedNames))}
		if len(runningNames) > 0 {
			parts = append(parts, "这些交易员仍在运行，已跳过，删除前需要先停止："+strings.Join(runningNames, "、"))
		}
		if len(failedNames) > 0 {
			parts = append(parts, "这些没删成功："+strings.Join(failedNames, "；"))
		}
		if len(deletedNames) > 0 {
			parts = append(parts, "已删除："+strings.Join(deletedNames, "、"))
		}
		return strings.Join(parts, "\n")
	}

	parts := []string{fmt.Sprintf("Bulk trader deletion finished: deleted %d trader(s).", len(deletedNames))}
	if len(runningNames) > 0 {
		parts = append(parts, "Skipped running traders; stop them before deleting: "+strings.Join(runningNames, ", "))
	}
	if len(failedNames) > 0 {
		parts = append(parts, "These did not delete successfully: "+strings.Join(failedNames, "; "))
	}
	if len(deletedNames) > 0 {
		parts = append(parts, "Deleted: "+strings.Join(deletedNames, ", "))
	}
	return strings.Join(parts, "\n")
}

func (a *Agent) executeExchangeManagementAction(storeUserID string, userID int64, lang, text string, session skillSession) string {
	switch session.Action {
	case "query_detail":
		if detail, ok := a.describeExchange(storeUserID, lang, session.TargetRef); ok {
			return detail
		}
		return formatReadFastPathResponse(lang, "get_exchange_configs", a.toolGetExchangeConfigs(storeUserID))
	case "delete":
		if fieldValue(session, skillDAGStepField) == "" {
			setSkillDAGStep(&session, "await_confirmation")
		}
		if msg, waiting := a.beginConfirmationIfNeeded(userID, lang, &session, defaultIfEmpty(session.TargetRef.Name, session.TargetRef.ID)); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		if msg, waiting := awaitingConfirmationButNotApproved(lang, session, text); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		setSkillDAGStep(&session, "execute_delete")
		args, _ := json.Marshal(map[string]any{"action": "delete", "exchange_id": session.TargetRef.ID})
		resp := a.toolManageExchangeConfig(storeUserID, string(args))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "这次没删成功：" + errMsg
			}
			return "That delete did not go through: " + errMsg
		}
		if lang == "zh" {
			return a.maybeResumeParentTaskAfterSuccessfulSkill(storeUserID, userID, lang, "exchange_management", "delete", "已删除交易所配置。")
		}
		return a.maybeResumeParentTaskAfterSuccessfulSkill(storeUserID, userID, lang, "exchange_management", "delete", "Deleted exchange config.")
	case "update", "update_name", "update_status":
		if fieldValue(session, skillDAGStepField) == "" {
			if session.Action == "update_status" {
				setSkillDAGStep(&session, "collect_enabled")
			} else {
				setSkillDAGStep(&session, "collect_account_name")
			}
		}
		patch := buildExchangeUpdatePatchFromSession(session)
		selectedField := fieldValue(session, "update_field")
		if selectedField == "" && session.Action == "update_status" {
			selectedField = "enabled"
			setField(&session, "update_field", selectedField)
		}
		applyExchangeUpdatePatchToSession(&session, patch)
		patch = mergeExchangeUpdatePatch(buildExchangeUpdatePatchFromSession(session), patch)
		patch, warnings := normalizeExchangePatchToManualLimits(lang, patch)
		applyExchangeUpdatePatchToSession(&session, patch)
		payload := map[string]any{"action": "update", "exchange_id": session.TargetRef.ID}
		accountName := defaultIfEmpty(patch.AccountName, fieldValue(session, "account_name"))
		if accountName != "" && session.Action != "update_status" {
			payload["account_name"] = accountName
		}
		enabledRaw := fieldValue(session, "enabled")
		if patch.Enabled != nil {
			enabledRaw = strconv.FormatBool(*patch.Enabled)
		}
		if enabledRaw != "" {
			payload["enabled"] = enabledRaw == "true"
		}
		if value := defaultIfEmpty(patch.APIKey, fieldValue(session, "api_key")); value != "" {
			payload["api_key"] = value
		}
		if value := defaultIfEmpty(patch.SecretKey, fieldValue(session, "secret_key")); value != "" {
			payload["secret_key"] = value
		}
		if value := defaultIfEmpty(patch.Passphrase, fieldValue(session, "passphrase")); value != "" {
			payload["passphrase"] = value
		}
		testnetRaw := fieldValue(session, "testnet")
		if patch.Testnet != nil {
			testnetRaw = strconv.FormatBool(*patch.Testnet)
		}
		if value := testnetRaw; value != "" {
			payload["testnet"] = value == "true"
		}
		if value := defaultIfEmpty(patch.HyperliquidWalletAddr, fieldValue(session, "hyperliquid_wallet_addr")); value != "" {
			payload["hyperliquid_wallet_addr"] = value
		}
		if value := defaultIfEmpty(patch.AsterUser, fieldValue(session, "aster_user")); value != "" {
			payload["aster_user"] = value
		}
		if value := defaultIfEmpty(patch.AsterSigner, fieldValue(session, "aster_signer")); value != "" {
			payload["aster_signer"] = value
		}
		if value := defaultIfEmpty(patch.AsterPrivateKey, fieldValue(session, "aster_private_key")); value != "" {
			payload["aster_private_key"] = value
		}
		if value := defaultIfEmpty(patch.LighterWalletAddr, fieldValue(session, "lighter_wallet_addr")); value != "" {
			payload["lighter_wallet_addr"] = value
		}
		if value := defaultIfEmpty(patch.LighterAPIKeyPrivateKey, fieldValue(session, "lighter_api_key_private_key")); value != "" {
			payload["lighter_api_key_private_key"] = value
		}
		if patch.LighterAPIKeyIndex != nil {
			payload["lighter_api_key_index"] = *patch.LighterAPIKeyIndex
		} else if value := fieldValue(session, "lighter_api_key_index"); value != "" {
			if parsed, err := strconv.Atoi(value); err == nil {
				payload["lighter_api_key_index"] = parsed
			}
		}
		if session.Action == "update_status" {
			delete(payload, "account_name")
		}
		if len(payload) == 2 {
			if session.Action == "update_status" {
				setSkillDAGStep(&session, "collect_enabled")
			} else {
				if selectedField != "" {
					setSkillDAGStep(&session, "collect_field_value")
				} else {
					setSkillDAGStep(&session, "collect_account_name")
				}
			}
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				if selectedField != "" {
					return fmt.Sprintf("还差一步：请告诉我你想把交易所配置里的%s改成什么。", displayCatalogFieldName(selectedField, lang))
				}
				return "你可以直接告诉我想改交易所配置里的哪一项，比如账户名、启用开关、API Key、Passphrase、钱包地址或 testnet。"
			}
			if selectedField != "" {
				return fmt.Sprintf("One more thing: tell me what you want to change the exchange config %s to.", displayCatalogFieldName(selectedField, lang))
			}
			return "Tell me which exchange config field you want to change, for example the account name, enabled switch, API key, passphrase, wallet address, or testnet."
		}
		if err := a.validateExchangeDraft(
			storeUserID,
			session.TargetRef.ID,
			"",
			payload["enabled"] == true,
			asString(payload["api_key"]),
			asString(payload["secret_key"]),
			asString(payload["passphrase"]),
			asString(payload["hyperliquid_wallet_addr"]),
			asString(payload["aster_user"]),
			asString(payload["aster_signer"]),
			asString(payload["aster_private_key"]),
			asString(payload["lighter_wallet_addr"]),
			asString(payload["lighter_api_key_private_key"]),
		); err != nil {
			a.saveSkillSession(userID, session)
			return formatValidationFeedback(lang, "exchange", err)
		}
		setSkillDAGStep(&session, "execute_update")
		raw, _ := json.Marshal(payload)
		resp := a.toolManageExchangeConfig(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "这次没改成功：" + errMsg
			}
			return "That change did not go through: " + errMsg
		}
		a.rememberReferencesFromToolResult(userID, "manage_exchange_config", resp)
		if lang == "zh" {
			reply := "已更新交易所配置。"
			if len(warnings) > 0 {
				reply += "\n\n已按手动面板范围自动调整：\n- " + strings.Join(warnings, "\n- ")
			}
			return a.maybeResumeParentTaskAfterSuccessfulSkill(storeUserID, userID, lang, "exchange_management", "update", reply)
		}
		reply := "Updated exchange config."
		if len(warnings) > 0 {
			reply += "\n\nAdjusted to stay within the manual editor limits:\n- " + strings.Join(warnings, "\n- ")
		}
		return a.maybeResumeParentTaskAfterSuccessfulSkill(storeUserID, userID, lang, "exchange_management", "update", reply)
	default:
		return ""
	}
}

func (a *Agent) executeModelManagementAction(storeUserID string, userID int64, lang, text string, session skillSession) string {
	switch session.Action {
	case "query_detail":
		if detail, ok := a.describeModel(storeUserID, lang, session.TargetRef); ok {
			return detail
		}
		return formatReadFastPathResponse(lang, "get_model_configs", a.toolGetModelConfigs(storeUserID))
	case "delete":
		if fieldValue(session, skillDAGStepField) == "" {
			setSkillDAGStep(&session, "await_confirmation")
		}
		if msg, waiting := a.beginConfirmationIfNeeded(userID, lang, &session, defaultIfEmpty(session.TargetRef.Name, session.TargetRef.ID)); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		if msg, waiting := awaitingConfirmationButNotApproved(lang, session, text); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		setSkillDAGStep(&session, "execute_delete")
		raw, _ := json.Marshal(map[string]any{"action": "delete", "model_id": session.TargetRef.ID})
		resp := a.toolManageModelConfig(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "这次没删成功：" + errMsg
			}
			return "That delete did not go through: " + errMsg
		}
		if lang == "zh" {
			return "已删除模型配置。"
		}
		return "Deleted model config."
	case "update", "update_name", "update_endpoint", "update_status":
		if fieldValue(session, skillDAGStepField) == "" {
			switch session.Action {
			case "update_status":
				setSkillDAGStep(&session, "collect_enabled")
			case "update_endpoint":
				setSkillDAGStep(&session, "collect_custom_api_url")
			default:
				setSkillDAGStep(&session, "collect_custom_model_name")
			}
		}
		payload := map[string]any{"action": "update", "model_id": session.TargetRef.ID}
		patch := buildModelUpdatePatchFromSession(session)
		selectedField := fieldValue(session, "update_field")
		if selectedField == "" {
			switch session.Action {
			case "update_status":
				selectedField = "enabled"
			case "update_endpoint":
				selectedField = "custom_api_url"
			}
			if selectedField != "" {
				setField(&session, "update_field", selectedField)
			}
		}
		applyModelUpdatePatchToSession(&session, patch)
		patch = mergeModelUpdatePatch(buildModelUpdatePatchFromSession(session), patch)
		urlValue := patch.CustomAPIURL
		enabledValue := ""
		if patch.Enabled != nil {
			enabledValue = strconv.FormatBool(*patch.Enabled)
		}
		apiKeyValue := patch.APIKey
		modelNameValue := patch.CustomModelName
		if value := defaultIfEmpty(urlValue, fieldValue(session, "custom_api_url")); value != "" {
			payload["custom_api_url"] = value
		}
		if value := defaultIfEmpty(enabledValue, fieldValue(session, "enabled")); value != "" {
			payload["enabled"] = value == "true"
		}
		if value := defaultIfEmpty(apiKeyValue, fieldValue(session, "api_key")); value != "" {
			payload["api_key"] = value
		}
		if value := defaultIfEmpty(modelNameValue, fieldValue(session, "custom_model_name")); value != "" {
			payload["custom_model_name"] = value
		}
		if session.Action == "update_name" {
			delete(payload, "custom_api_url")
			delete(payload, "enabled")
			delete(payload, "api_key")
		}
		if session.Action == "update_status" {
			delete(payload, "custom_api_url")
			delete(payload, "custom_model_name")
			delete(payload, "api_key")
		}
		if session.Action == "update_endpoint" {
			delete(payload, "custom_model_name")
			delete(payload, "enabled")
			delete(payload, "api_key")
		}
		if len(payload) == 2 {
			switch session.Action {
			case "update_status":
				setSkillDAGStep(&session, "collect_enabled")
			case "update_endpoint":
				setSkillDAGStep(&session, "collect_custom_api_url")
			default:
				if selectedField != "" {
					setSkillDAGStep(&session, "collect_field_value")
				} else {
					setSkillDAGStep(&session, "collect_custom_model_name")
				}
			}
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				if selectedField != "" {
					return fmt.Sprintf("还差一步：请告诉我新的%s。", displayCatalogFieldName(selectedField, lang))
				}
				return "你可以直接告诉我想改哪一项，比如模型名称、接口地址，或者开关状态。"
			}
			if selectedField != "" {
				return fmt.Sprintf("One more thing: tell me the new %s.", displayCatalogFieldName(selectedField, lang))
			}
			return "Tell me what you want to change, for example the model name, endpoint URL, or on or off status."
		}
		if err := a.validateModelDraft(
			storeUserID,
			session.TargetRef.ID,
			"",
			payload["enabled"] == true,
			asString(payload["api_key"]),
			asString(payload["custom_api_url"]),
			asString(payload["custom_model_name"]),
		); err != nil {
			a.saveSkillSession(userID, session)
			return formatValidationFeedback(lang, "model", err)
		}
		setSkillDAGStep(&session, "execute_update")
		raw, _ := json.Marshal(payload)
		resp := a.toolManageModelConfig(storeUserID, string(raw))
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				if strings.Contains(errMsg, "cannot enable model config before API key is configured") {
					return "更新模型配置失败：这个模型还没有配置 API Key，暂时不能启用。你可以直接把 API Key 发给我，我帮你继续配置。"
				}
				return "这次没改成功：" + errMsg
			}
			a.saveSkillSession(userID, session)
			return "That change did not go through: " + errMsg
		}
		a.clearSkillSession(userID)
		a.rememberReferencesFromToolResult(userID, "manage_model_config", resp)
		if lang == "zh" {
			if session.Action == "update_status" {
				return "已更新模型配置启用状态。"
			}
			return "已更新模型配置。"
		}
		return "Updated model config."
	default:
		return ""
	}
}

func (a *Agent) executeStrategyManagementAction(storeUserID string, userID int64, lang, text string, session skillSession) string {
	switch session.Action {
	case "query", "query_list":
		return formatReadFastPathResponse(lang, "get_strategies", a.toolGetStrategies(storeUserID))
	case "query_detail":
		if detail, ok := a.describeStrategy(storeUserID, lang, session.TargetRef); ok {
			return detail
		}
		return formatReadFastPathResponse(lang, "get_strategies", a.toolGetStrategies(storeUserID))
	case "activate":
		raw, _ := json.Marshal(map[string]any{"action": "activate", "strategy_id": session.TargetRef.ID})
		resp := a.toolManageStrategy(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "这次没激活成功：" + errMsg
			}
			return "That activation did not go through: " + errMsg
		}
		if lang == "zh" {
			return "已激活策略。"
		}
		return "Activated strategy."
	case "duplicate":
		if fieldValue(session, skillDAGStepField) == "" {
			setSkillDAGStep(&session, "collect_name")
		}
		newName := fieldValue(session, "name")
		if newName != "" {
			setField(&session, "name", newName)
		}
		if newName == "" {
			setSkillDAGStep(&session, "collect_name")
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "还差一步：请给这个新策略起个名字。"
			}
			return "One more thing: give the new strategy a name."
		}
		setSkillDAGStep(&session, "execute_duplicate")
		raw, _ := json.Marshal(map[string]any{"action": "duplicate", "strategy_id": session.TargetRef.ID, "name": newName})
		resp := a.toolManageStrategy(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "这次没复制成功：" + errMsg
			}
			return "That copy did not go through: " + errMsg
		}
		if lang == "zh" {
			return fmt.Sprintf("已复制策略，新名称为“%s”。", newName)
		}
		return fmt.Sprintf("Duplicated strategy as %q.", newName)
	case "delete":
		if fieldValue(session, skillDAGStepField) == "" {
			setSkillDAGStep(&session, "await_confirmation")
		}
		if fieldValue(session, "bulk_scope") == "all" {
			strategies, err := a.store.Strategy().List(storeUserID)
			if err != nil {
				if lang == "zh" {
					return "我这边暂时没读到策略列表：" + err.Error()
				}
				return "I could not load the strategy list just now: " + err.Error()
			}

			deletable := make([]*store.Strategy, 0, len(strategies))
			skippedDefault := 0
			for _, strategy := range strategies {
				if strategy == nil {
					continue
				}
				if strategy.IsDefault {
					skippedDefault++
					continue
				}
				deletable = append(deletable, strategy)
			}
			if len(deletable) == 0 {
				a.clearSkillSession(userID)
				if lang == "zh" {
					return "当前没有可删除的自定义策略。"
				}
				return "There are no user-created strategies to delete."
			}

			targetLabel := fmt.Sprintf("全部自定义策略（共 %d 个）", len(deletable))
			if msg, waiting := a.beginConfirmationIfNeeded(userID, lang, &session, targetLabel); waiting {
				a.saveSkillSession(userID, session)
				return msg
			}
			if msg, waiting := awaitingConfirmationButNotApproved(lang, session, text); waiting {
				a.saveSkillSession(userID, session)
				return msg
			}

			setSkillDAGStep(&session, "execute_delete")
			deletedNames := make([]string, 0, len(deletable))
			failedNames := make([]string, 0)
			for _, strategy := range deletable {
				raw, _ := json.Marshal(map[string]any{"action": "delete", "strategy_id": strategy.ID})
				resp := a.toolManageStrategy(storeUserID, string(raw))
				if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
					failedNames = append(failedNames, fmt.Sprintf("%s（%s）", strategy.Name, errMsg))
					continue
				}
				deletedNames = append(deletedNames, strategy.Name)
			}
			a.clearSkillSession(userID)

			if lang == "zh" {
				parts := []string{fmt.Sprintf("批量删除策略已完成：成功删除 %d 个。", len(deletedNames))}
				if skippedDefault > 0 {
					parts = append(parts, fmt.Sprintf("已跳过系统默认策略 %d 个。", skippedDefault))
				}
				if len(failedNames) > 0 {
					parts = append(parts, "这些没删成功："+strings.Join(failedNames, "；"))
				}
				if len(deletedNames) > 0 {
					parts = append(parts, "已删除："+strings.Join(deletedNames, "、"))
				}
				return strings.Join(parts, "\n")
			}

			parts := []string{fmt.Sprintf("Bulk strategy deletion finished: deleted %d strategy(s).", len(deletedNames))}
			if skippedDefault > 0 {
				parts = append(parts, fmt.Sprintf("Skipped %d default strategy(ies).", skippedDefault))
			}
			if len(failedNames) > 0 {
				parts = append(parts, "These did not delete successfully: "+strings.Join(failedNames, "; "))
			}
			if len(deletedNames) > 0 {
				parts = append(parts, "Deleted: "+strings.Join(deletedNames, ", "))
			}
			return strings.Join(parts, "\n")
		}
		if msg, waiting := a.beginConfirmationIfNeeded(userID, lang, &session, defaultIfEmpty(session.TargetRef.Name, session.TargetRef.ID)); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		if msg, waiting := awaitingConfirmationButNotApproved(lang, session, text); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		setSkillDAGStep(&session, "execute_delete")
		raw, _ := json.Marshal(map[string]any{"action": "delete", "strategy_id": session.TargetRef.ID})
		resp := a.toolManageStrategy(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "这次没删成功：" + errMsg
			}
			return "That delete did not go through: " + errMsg
		}
		if lang == "zh" {
			return "已删除策略。"
		}
		return "Deleted strategy."
	case "update_name", "update_config", "update_prompt":
		if session.Action == "update_prompt" {
			return a.executeStrategyPromptUpdate(storeUserID, userID, lang, text, session)
		}
		if session.Action == "update_config" ||
			fieldValue(session, strategyPendingUpdateConfigField) != "" ||
			fieldValue(session, "config_field") != "" ||
			fieldValue(session, "config_value") != "" {
			return a.executeStrategyConfigUpdate(storeUserID, userID, lang, text, session)
		}
		if fieldValue(session, skillDAGStepField) == "" {
			setSkillDAGStep(&session, "collect_name")
		}
		newName := fieldValue(session, "name")
		if newName != "" {
			setField(&session, "name", newName)
		}
		if newName == "" {
			setSkillDAGStep(&session, "collect_name")
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "目前这里先支持改策略名称。你直接把新名字发给我就行。"
			}
			return "For now, this step supports renaming the strategy. Just send me the new name."
		}
		setSkillDAGStep(&session, "execute_update")
		raw, _ := json.Marshal(map[string]any{"action": "update", "strategy_id": session.TargetRef.ID, "name": newName})
		resp := a.toolManageStrategy(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "这次没改成功：" + errMsg
			}
			return "That change did not go through: " + errMsg
		}
		if lang == "zh" {
			return fmt.Sprintf("已将策略改名为“%s”。", newName)
		}
		return fmt.Sprintf("Renamed strategy to %q.", newName)
	case "update":
		a.clearSkillSession(userID)
		if lang == "zh" {
			return "我需要先明确你要改策略的哪一部分：名称、提示词，还是策略参数。"
		}
		return "I need to know which part of the strategy to update: name, prompt, or config."
	default:
		return ""
	}
}

func (a *Agent) executeStrategyPromptUpdate(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if fieldValue(session, skillDAGStepField) == "" {
		setSkillDAGStep(&session, "collect_prompt")
	}
	strategy, cfg, err := a.loadStrategyConfigForUpdate(storeUserID, session.TargetRef.ID)
	if err != nil {
		if lang == "zh" {
			return "我这边暂时没读到这份策略：" + err.Error()
		}
		return "I could not load that strategy just now: " + err.Error()
	}

	prompt := fieldValue(session, "prompt")
	if prompt == "" {
		prompt = fieldValue(session, "custom_prompt")
		if prompt != "" {
			setField(&session, "prompt", prompt)
		}
	}
	if generatedDraftRequiresConfirmation(session) {
		switch {
		case createConfirmationReply(text):
			clearGeneratedDraftConfirmation(&session)
		case isNoReply(text):
			clearGeneratedDraftConfirmation(&session, "prompt", "custom_prompt")
			setSkillDAGStep(&session, "collect_prompt")
			session.Phase = "collecting"
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "好，我先不用这版草稿。你可以告诉我想保留的风格，或者直接让我重新设计一版 prompt。"
			}
			return "Okay, I won't use that draft. Tell me the style you want to keep, or ask me to draft another prompt."
		}
	}
	if prompt == "" {
		prompt = extractQuotedContent(text)
		if prompt != "" {
			setField(&session, "prompt", prompt)
		}
	}
	if prompt == "" {
		setSkillDAGStep(&session, "collect_prompt")
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "还差一步：请把新的提示词内容发给我，直接发正文就行。"
		}
		return "One more thing: send me the new prompt text."
	}

	cfg.CustomPrompt = prompt
	setSkillDAGStep(&session, "execute_update")
	return a.persistStrategyConfigUpdate(storeUserID, userID, lang, strategy, cfg, "已更新策略 prompt。", "Updated strategy prompt.")
}

func (a *Agent) executeStrategyConfigUpdate(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if rawPending := fieldValue(session, strategyPendingUpdateConfigField); rawPending != "" {
		if createConfirmationReply(text) {
			var pendingCfg store.StrategyConfig
			if err := json.Unmarshal([]byte(rawPending), &pendingCfg); err != nil {
				if session.Fields != nil {
					delete(session.Fields, strategyPendingUpdateConfigField)
					delete(session.Fields, strategyPendingUpdateWarnings)
					delete(session.Fields, strategyPendingUpdateZhMsg)
					delete(session.Fields, strategyPendingUpdateEnMsg)
				}
				session.Phase = "collecting"
				a.saveSkillSession(userID, session)
				if lang == "zh" {
					return "我这边暂时没读到刚才那版草稿。你再告诉我想改哪一项，我马上继续。"
				}
				return "I could not read that draft just now. Tell me what you want to change and I will continue."
			}
			zhMsg := defaultIfEmpty(fieldValue(session, strategyPendingUpdateZhMsg), "已更新策略参数。")
			enMsg := defaultIfEmpty(fieldValue(session, strategyPendingUpdateEnMsg), "Updated strategy config.")
			return a.persistPendingStrategyConfigUpdate(storeUserID, userID, lang, session, pendingCfg, zhMsg, enMsg)
		}
		if session.Fields != nil {
			delete(session.Fields, strategyPendingUpdateConfigField)
			delete(session.Fields, strategyPendingUpdateWarnings)
			delete(session.Fields, strategyPendingUpdateZhMsg)
			delete(session.Fields, strategyPendingUpdateEnMsg)
		}
		session.Phase = "collecting"
	}

	if _, ok := getSkillDAG("strategy_management", "update_config"); ok {
		if fieldValue(session, skillDAGStepField) == "" {
			setSkillDAGStep(&session, "resolve_config_field")
		}
	}

	currentStep, _ := currentSkillDAGStep(session)
	strategy, cfg, err := a.loadStrategyConfigForUpdate(storeUserID, session.TargetRef.ID)
	if err != nil {
		if lang == "zh" {
			return "我这边暂时没读到这份策略：" + err.Error()
		}
		return "I could not load that strategy just now: " + err.Error()
	}

	if patchRaw := strings.TrimSpace(fieldValue(session, strategyCreateConfigPatchField)); patchRaw != "" {
		var patch map[string]any
		if err := json.Unmarshal([]byte(patchRaw), &patch); err != nil {
			setSkillDAGStep(&session, "resolve_config_field")
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "策略配置 patch 不是合法 JSON：" + err.Error()
			}
			return "The strategy config patch is not valid JSON: " + err.Error()
		}
		merged, err := store.MergeStrategyConfig(cfg, patch)
		if err != nil {
			setSkillDAGStep(&session, "resolve_config_field")
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "策略配置 patch 无法应用：" + err.Error()
			}
			return "The strategy config patch could not be applied: " + err.Error()
		}
		beforeClamp := merged
		merged.ClampLimits()
		msgZH := "已更新策略配置。"
		msgEN := "Updated strategy config."
		setSkillDAGStep(&session, "apply_field_update")
		if warnings := store.StrategyClampWarnings(beforeClamp, merged, lang); len(warnings) > 0 {
			return a.deferStrategyRiskControlledUpdate(userID, lang, &session, merged, warnings, msgZH, msgEN)
		}
		setSkillDAGStep(&session, "execute_update")
		raw, _ := json.Marshal(map[string]any{
			"action":               "update",
			"strategy_id":          strategy.ID,
			"config":               patch,
			"allow_clamped_update": true,
		})
		resp := a.toolManageStrategy(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "这次没改成功：" + errMsg
			}
			return "That change did not go through: " + errMsg
		}
		a.rememberReferencesFromToolResult(userID, "manage_strategy", resp)
		if lang == "zh" {
			return msgZH
		}
		return msgEN
	}

	if generatedDraftRequiresConfirmation(session) && fieldValue(session, "config_field") == "" && fieldValue(session, "config_value") == "" {
		if generated := fieldValue(session, "custom_prompt"); generated != "" {
			setField(&session, "config_field", "custom_prompt")
			setField(&session, "config_value", generated)
		}
	}
	if generatedDraftRequiresConfirmation(session) {
		switch {
		case createConfirmationReply(text):
			clearGeneratedDraftConfirmation(&session)
		case isNoReply(text):
			clearGeneratedDraftConfirmation(&session, "config_field", "config_value", "custom_prompt")
			setSkillDAGStep(&session, "resolve_config_field")
			session.Phase = "collecting"
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "好，我先不用这版草稿。你可以直接告诉我要改哪个配置，或者继续让我重新设计一版。"
			}
			return "Okay, I won't use that draft. Tell me which config to change, or ask me to draft another version."
		}
	}

	if fieldValue(session, "config_field") == "" && fieldValue(session, "config_value") == "" {
		if strings.Contains(strings.ToLower(text), "min position size") || strings.Contains(strings.ToLower(text), "最小开仓金额") {
			a.clearSkillSession(userID)
			return strategyLockedFieldError(lang, "min_position_size")
		}
		patches := detectStrategyConfigPatches(text)
		if len(patches) > 1 {
			changed := make([]string, 0, len(patches))
			for _, patch := range patches {
				if patch.Field == "min_position_size" {
					a.clearSkillSession(userID)
					return strategyLockedFieldError(lang, "min_position_size")
				}
				if err := applyStrategyConfigPatch(&cfg, patch.Field, patch.Value); err != nil {
					a.saveSkillSession(userID, session)
					if lang == "zh" {
						return "这次没改成功：" + err.Error()
					}
					return "That change did not go through: " + err.Error()
				}
				switch patch.Field {
				case "description":
					strategy.Description = patch.Value
				case "is_public":
					strategy.IsPublic = patch.Value == "true"
				case "config_visible":
					strategy.ConfigVisible = patch.Value == "true"
				}
				changed = append(changed, strategyConfigFieldDisplayName(patch.Field, lang))
			}
			beforeClamp := cfg
			cfg.ClampLimits()
			setSkillDAGStep(&session, "apply_field_update")
			msgZH := "已更新策略参数：" + strings.Join(changed, "、") + "。"
			msgEN := "Updated strategy config fields: " + strings.Join(changed, ", ") + "."
			if warnings := store.StrategyClampWarnings(beforeClamp, cfg, lang); len(warnings) > 0 {
				return a.deferStrategyRiskControlledUpdate(userID, lang, &session, cfg, warnings, msgZH, msgEN)
			}
			setSkillDAGStep(&session, "execute_update")
			return a.persistStrategyConfigUpdate(storeUserID, userID, lang, strategy, cfg, msgZH, msgEN)
		}
	}

	field := fieldValue(session, "config_field")
	if field == "" {
		field = detectStrategyConfigField(text)
		if field != "" {
			if field == "min_position_size" {
				a.clearSkillSession(userID)
				return strategyLockedFieldError(lang, field)
			}
			setField(&session, "config_field", field)
			if currentStep.ID == "resolve_config_field" {
				advanceSkillDAGStep(&session, currentStep.ID)
				currentStep, _ = currentSkillDAGStep(session)
			}
		}
	}
	if field == "" {
		setSkillDAGStep(&session, "resolve_config_field")
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "你可以直接告诉我想改哪一项，比如币种来源、杠杆、时间周期、技术指标，或者提示词。"
		}
		return "Tell me what you want to change, for example coin source, leverage, timeframes, indicators, or the prompt."
	}

	if fieldValue(session, "config_value") == "" {
		if value, ok := extractStrategyConfigValue(text, field); ok {
			setField(&session, "config_value", value)
			if currentStep.ID == "resolve_config_value" {
				advanceSkillDAGStep(&session, currentStep.ID)
				currentStep, _ = currentSkillDAGStep(session)
			}
		}
	}
	value := fieldValue(session, "config_value")
	if value == "" {
		setSkillDAGStep(&session, "resolve_config_value")
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return fmt.Sprintf("还差一步：请告诉我新的%s。", strategyConfigFieldDisplayName(field, lang))
		}
		return fmt.Sprintf("One more thing: tell me the new %s.", strategyConfigFieldDisplayName(field, lang))
	}

	if err := applyStrategyConfigPatch(&cfg, field, value); err != nil {
		setSkillDAGStep(&session, "resolve_config_value")
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return err.Error()
		}
		return err.Error()
	}
	switch field {
	case "description":
		strategy.Description = value
	case "is_public":
		strategy.IsPublic = value == "true"
	case "config_visible":
		strategy.ConfigVisible = value == "true"
	}

	beforeClamp := cfg
	cfg.ClampLimits()
	changed := []string{field}
	displayChanged := make([]string, 0, len(changed))
	for _, item := range changed {
		displayChanged = append(displayChanged, strategyConfigFieldDisplayName(item, lang))
	}
	msgZH := "已更新策略参数：" + strings.Join(displayChanged, "、") + "。"
	msgEN := "Updated strategy config fields: " + strings.Join(displayChanged, ", ") + "."
	setSkillDAGStep(&session, "apply_field_update")
	if warnings := store.StrategyClampWarnings(beforeClamp, cfg, lang); len(warnings) > 0 {
		return a.deferStrategyRiskControlledUpdate(userID, lang, &session, cfg, warnings, msgZH, msgEN)
	}
	setSkillDAGStep(&session, "execute_update")
	return a.persistStrategyConfigUpdate(storeUserID, userID, lang, strategy, cfg, msgZH, msgEN)
}

func (a *Agent) loadStrategyConfigForUpdate(storeUserID, strategyID string) (*store.Strategy, store.StrategyConfig, error) {
	strategy, err := a.store.Strategy().Get(storeUserID, strategyID)
	if err != nil {
		return nil, store.StrategyConfig{}, err
	}
	cfg := store.GetDefaultStrategyConfig("zh")
	if strings.TrimSpace(strategy.Config) != "" {
		_ = json.Unmarshal([]byte(strategy.Config), &cfg)
	}
	return strategy, cfg, nil
}

func (a *Agent) deferStrategyRiskControlledUpdate(userID int64, lang string, session *skillSession, cfg store.StrategyConfig, warnings []string, zhMsg, enMsg string) string {
	rawConfig, _ := json.Marshal(cfg)
	setField(session, strategyPendingUpdateConfigField, string(rawConfig))
	setField(session, strategyPendingUpdateWarnings, marshalStringList(warnings))
	setField(session, strategyPendingUpdateZhMsg, zhMsg)
	setField(session, strategyPendingUpdateEnMsg, enMsg)
	session.Phase = "await_confirmation"
	setSkillDAGStep(session, "await_confirmation")
	a.saveSkillSession(userID, *session)
	task := SuspendedTask{
		Kind: "skill_session",
		SkillSession: func() *skillSession {
			copy := normalizeSkillSession(*session)
			return &copy
		}(),
		ResumeHint: buildSkillResumeHint(lang, *session),
	}
	a.SnapshotManager(userID).Save(task)
	return formatRiskControlAcceptancePrompt(lang, warnings, "确认应用")
}

func (a *Agent) persistPendingStrategyConfigUpdate(storeUserID string, userID int64, lang string, session skillSession, cfg store.StrategyConfig, zhMsg, enMsg string) string {
	if session.Fields != nil {
		delete(session.Fields, strategyPendingUpdateConfigField)
		delete(session.Fields, strategyPendingUpdateWarnings)
		delete(session.Fields, strategyPendingUpdateZhMsg)
		delete(session.Fields, strategyPendingUpdateEnMsg)
	}
	strategy, _, err := a.loadStrategyConfigForUpdate(storeUserID, session.TargetRef.ID)
	if err != nil {
		if lang == "zh" {
			return "我这边暂时没读到这份策略：" + err.Error()
		}
		return "I could not load that strategy just now: " + err.Error()
	}
	return a.persistStrategyConfigUpdate(storeUserID, userID, lang, strategy, cfg, zhMsg, enMsg)
}

func (a *Agent) persistStrategyConfigUpdate(storeUserID string, userID int64, lang string, strategy *store.Strategy, cfg store.StrategyConfig, zhMsg, enMsg string) string {
	rawConfig, err := json.Marshal(cfg)
	if err != nil {
		if lang == "zh" {
			return "我这边整理这份策略配置时出了点问题：" + err.Error()
		}
		return "I ran into a problem while preparing that strategy config: " + err.Error()
	}
	raw, _ := json.Marshal(map[string]any{
		"action":         "update",
		"strategy_id":    strategy.ID,
		"name":           strategy.Name,
		"description":    strategy.Description,
		"is_public":      strategy.IsPublic,
		"config_visible": strategy.ConfigVisible,
		"config":         json.RawMessage(rawConfig),
	})
	resp := a.toolManageStrategy(storeUserID, string(raw))
	a.clearSkillSession(userID)
	if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
		if lang == "zh" {
			return "这次没改成功：" + errMsg
		}
		return "That change did not go through: " + errMsg
	}
	if warnings := parseToolWarnings(resp); len(warnings) > 0 {
		if lang == "zh" {
			zhMsg += "\n\n已按安全范围自动调整：\n- " + strings.Join(warnings, "\n- ")
		} else {
			enMsg += "\n\nAdjusted to stay within safe limits:\n- " + strings.Join(warnings, "\n- ")
		}
	}
	if lang == "zh" {
		return zhMsg
	}
	return enMsg
}

func parseToolWarnings(raw string) []string {
	var payload struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return payload.Warnings
}

func extractQuotedContent(text string) string {
	if matches := quotedContentRE.FindStringSubmatch(text); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func extractLabeledInt(text string, labels []string) (int, bool) {
	lower := strings.ToLower(text)
	for _, label := range labels {
		idx := strings.Index(lower, strings.ToLower(label))
		if idx < 0 {
			continue
		}
		segment := text[idx:]
		if match := firstIntegerPattern.FindString(segment); match != "" {
			if value, err := strconv.Atoi(match); err == nil {
				return value, true
			}
		}
	}
	return 0, false
}

func extractTimeframeAfterKeywords(text string, labels []string) string {
	lower := strings.ToLower(text)
	for _, label := range labels {
		idx := strings.Index(lower, strings.ToLower(label))
		if idx < 0 {
			continue
		}
		segment := text[idx:]
		if match := timeframeTokenRE.FindString(segment); match != "" {
			return strings.ToLower(match)
		}
	}
	return ""
}

func extractTimeframes(text string) []string {
	matches := timeframeTokenRE.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		tf := strings.ToLower(strings.TrimSpace(match))
		if tf == "" {
			continue
		}
		if _, ok := seen[tf]; ok {
			continue
		}
		seen[tf] = struct{}{}
		out = append(out, tf)
	}
	return out
}

func (a *Agent) handleTraderDiagnosisSkill(storeUserID, lang, text string) string {
	raw := a.toolListTraders(storeUserID)
	list := formatReadFastPathResponse(lang, "list_traders", raw)
	if lang == "zh" {
		reply := "现象：这是交易员运行诊断问题。\n优先排查：\n1. 交易员是否已创建并处于运行状态。\n2. 绑定的模型、交易所、策略是否齐全。\n3. 是“没有启动”、还是“启动了但 AI 没有下单”、还是“下单失败”。\n当前交易员概览：\n" + list
		if excerpt := backendLogDiagnosisExcerpt(lang, text, "trader"); excerpt != "" {
			reply += "\n" + excerpt
		}
		return reply
	}
	reply := "This looks like a trader diagnosis issue.\nCheck whether the trader exists, is running, and has model/exchange/strategy bindings.\nCurrent trader overview:\n" + list
	if excerpt := backendLogDiagnosisExcerpt(lang, text, "trader"); excerpt != "" {
		reply += "\n" + excerpt
	}
	return reply
}

func (a *Agent) handleStrategyDiagnosisSkill(storeUserID, lang, text string) string {
	raw := a.toolGetStrategies(storeUserID)
	list := formatReadFastPathResponse(lang, "get_strategies", raw)
	if lang == "zh" {
		reply := "现象：这是策略或提示词生效问题。\n优先排查：\n1. 你改的是策略模板，还是 trader 上的 custom prompt。\n2. 策略是否真的保存成功。\n3. 运行结果不符合预期，是配置问题还是市场条件问题。\n当前策略概览：\n" + list
		if excerpt := backendLogDiagnosisExcerpt(lang, text, "strategy"); excerpt != "" {
			reply += "\n" + excerpt
		}
		return reply
	}
	reply := "This looks like a strategy or prompt diagnosis issue.\nCheck whether you changed the strategy template or a trader-specific prompt override.\nCurrent strategy overview:\n" + list
	if excerpt := backendLogDiagnosisExcerpt(lang, text, "strategy"); excerpt != "" {
		reply += "\n" + excerpt
	}
	return reply
}
