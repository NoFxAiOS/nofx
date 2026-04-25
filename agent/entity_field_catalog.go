package agent

type entityFieldMeta struct {
	Key            string
	Keywords       []string
	ValueType      string
	ManualEditable bool
	AgentUpdatable bool
}

var traderFieldCatalog = []entityFieldMeta{
	{Key: "name", Keywords: []string{"改名", "重命名", "rename", "名字"}, ValueType: "name", ManualEditable: true, AgentUpdatable: true},
	{Key: "ai_model_id", Keywords: []string{"换模型", "切换模型", "模型"}, ValueType: "entity_ref", ManualEditable: true, AgentUpdatable: true},
	{Key: "exchange_id", Keywords: []string{"换交易所", "切换交易所", "交易所"}, ValueType: "entity_ref", ManualEditable: true, AgentUpdatable: true},
	{Key: "strategy_id", Keywords: []string{"换策略", "切换策略", "策略"}, ValueType: "entity_ref", ManualEditable: true, AgentUpdatable: true},
	{Key: "scan_interval_minutes", Keywords: []string{"扫描间隔", "扫描频率", "scan interval", "scan frequency"}, ValueType: "int", ManualEditable: true, AgentUpdatable: true},
	{Key: "is_cross_margin", Keywords: []string{"全仓", "cross margin", "is_cross_margin"}, ValueType: "flag", ManualEditable: true, AgentUpdatable: true},
	{Key: "show_in_competition", Keywords: []string{"竞技场显示", "显示在竞技场", "show in competition", "competition"}, ValueType: "flag", ManualEditable: true, AgentUpdatable: true},
	{Key: "btc_eth_leverage", Keywords: []string{"btc/eth杠杆", "主流币杠杆", "btc eth leverage"}, ValueType: "int", ManualEditable: true, AgentUpdatable: true},
	{Key: "altcoin_leverage", Keywords: []string{"山寨币杠杆", "altcoin leverage", "alts leverage"}, ValueType: "int", ManualEditable: true, AgentUpdatable: true},
	{Key: "trading_symbols", Keywords: []string{"交易对", "symbols", "币种"}, ValueType: "text", ManualEditable: true, AgentUpdatable: true},
	{Key: "custom_prompt", Keywords: []string{"自定义prompt", "custom prompt", "提示词"}, ValueType: "text", ManualEditable: true, AgentUpdatable: true},
	{Key: "override_base_prompt", Keywords: []string{"覆盖基础提示词", "override base prompt"}, ValueType: "flag", ManualEditable: true, AgentUpdatable: true},
	{Key: "system_prompt_template", Keywords: []string{"提示词模板", "system prompt template", "prompt template"}, ValueType: "text", ManualEditable: true, AgentUpdatable: true},
	{Key: "use_ai500", Keywords: []string{"ai500"}, ValueType: "flag", ManualEditable: true, AgentUpdatable: true},
	{Key: "use_oi_top", Keywords: []string{"oi top", "持仓量增长"}, ValueType: "flag", ManualEditable: true, AgentUpdatable: true},
}

var modelFieldCatalog = []entityFieldMeta{
	{Key: "provider", Keywords: []string{"provider", "模型提供商", "模型厂商", "vendor"}, ValueType: "enum", ManualEditable: true, AgentUpdatable: true},
	{Key: "name", Keywords: []string{"名称", "名字", "name"}, ValueType: "name", ManualEditable: true, AgentUpdatable: true},
	{Key: "enabled", Keywords: []string{"启用", "禁用", "enable", "disable"}, ValueType: "enabled", AgentUpdatable: true},
	{Key: "api_key", Keywords: []string{"api key", "apikey", "api_key"}, ValueType: "credential", ManualEditable: true, AgentUpdatable: true},
	{Key: "custom_api_url", Keywords: []string{"url", "endpoint", "地址", "接口"}, ValueType: "url", ManualEditable: true, AgentUpdatable: true},
	{Key: "custom_model_name", Keywords: []string{"model name", "模型名称", "模型名"}, ValueType: "model_name", ManualEditable: true, AgentUpdatable: true},
}

var exchangeFieldCatalog = []entityFieldMeta{
	{Key: "exchange_type", Keywords: []string{"交易所类型", "交易所", "exchange type", "exchange"}, ValueType: "enum", ManualEditable: true, AgentUpdatable: true},
	{Key: "account_name", Keywords: []string{"账户名", "account name"}, ValueType: "account_name", ManualEditable: true, AgentUpdatable: true},
	{Key: "enabled", Keywords: []string{"启用", "禁用", "enable", "disable"}, ValueType: "enabled", AgentUpdatable: true},
	{Key: "api_key", Keywords: []string{"api key", "apikey", "api_key"}, ValueType: "credential", ManualEditable: true, AgentUpdatable: true},
	{Key: "secret_key", Keywords: []string{"secret key", "secret", "secret_key"}, ValueType: "credential", ManualEditable: true, AgentUpdatable: true},
	{Key: "passphrase", Keywords: []string{"passphrase", "密码短语"}, ValueType: "credential", ManualEditable: true, AgentUpdatable: true},
	{Key: "testnet", Keywords: []string{"testnet", "测试网"}, ValueType: "flag", ManualEditable: true, AgentUpdatable: true},
	{Key: "hyperliquid_wallet_addr", Keywords: []string{"hyperliquid wallet", "hyperliquid钱包", "主钱包地址", "wallet address"}, ValueType: "credential", ManualEditable: true, AgentUpdatable: true},
	{Key: "aster_user", Keywords: []string{"aster user", "aster用户", "用户地址", "user"}, ValueType: "credential", ManualEditable: true, AgentUpdatable: true},
	{Key: "aster_signer", Keywords: []string{"aster signer", "signer"}, ValueType: "credential", ManualEditable: true, AgentUpdatable: true},
	{Key: "aster_private_key", Keywords: []string{"aster private key", "aster私钥", "private key"}, ValueType: "credential", ManualEditable: true, AgentUpdatable: true},
	{Key: "lighter_wallet_addr", Keywords: []string{"lighter wallet", "lighter钱包", "wallet address"}, ValueType: "credential", ManualEditable: true, AgentUpdatable: true},
	{Key: "lighter_api_key_private_key", Keywords: []string{"lighter api key private key", "lighter api key", "api key private key"}, ValueType: "credential", ManualEditable: true, AgentUpdatable: true},
	{Key: "lighter_api_key_index", Keywords: []string{"lighter api key index", "lighter索引", "api key index"}, ValueType: "int", ManualEditable: true, AgentUpdatable: true},
}

func fieldKeysByCapability(catalog []entityFieldMeta, include func(entityFieldMeta) bool) []string {
	keys := make([]string, 0, len(catalog))
	for _, field := range catalog {
		if include(field) {
			keys = append(keys, field.Key)
		}
	}
	return keys
}

func keywordsForField(catalog []entityFieldMeta, field string) []string {
	for _, item := range catalog {
		if item.Key == field {
			return item.Keywords
		}
	}
	return nil
}

func manualTraderEditableFieldKeys() []string {
	return fieldKeysByCapability(traderFieldCatalog, func(field entityFieldMeta) bool {
		return field.ManualEditable
	})
}

func agentTraderUpdatableFieldKeys() []string {
	return fieldKeysByCapability(traderFieldCatalog, func(field entityFieldMeta) bool {
		return field.AgentUpdatable
	})
}

func manualModelEditableFieldKeys() []string {
	return fieldKeysByCapability(modelFieldCatalog, func(field entityFieldMeta) bool {
		return field.ManualEditable
	})
}

func agentModelUpdatableFieldKeys() []string {
	return fieldKeysByCapability(modelFieldCatalog, func(field entityFieldMeta) bool {
		return field.AgentUpdatable
	})
}

func manualExchangeEditableFieldKeys() []string {
	return fieldKeysByCapability(exchangeFieldCatalog, func(field entityFieldMeta) bool {
		return field.ManualEditable
	})
}

func agentExchangeUpdatableFieldKeys() []string {
	return fieldKeysByCapability(exchangeFieldCatalog, func(field entityFieldMeta) bool {
		return field.AgentUpdatable
	})
}

func traderFieldKeywords(field string) []string {
	return keywordsForField(traderFieldCatalog, field)
}

func modelFieldKeywords(field string) []string {
	return keywordsForField(modelFieldCatalog, field)
}

func exchangeFieldKeywords(field string) []string {
	return keywordsForField(exchangeFieldCatalog, field)
}
