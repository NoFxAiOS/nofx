package agent

import "strings"

func buildSkillDomainPrimer(lang, skillName string) string {
	skillName = strings.TrimSpace(skillName)
	if skillName == "" {
		return ""
	}
	switch skillName {
	case "model_management":
		fields := []string{
			fieldKnowledgeDisplayName("provider", lang),
			displayCatalogFieldName("name", lang),
			displayCatalogFieldName("api_key", lang),
			displayCatalogFieldName("custom_api_url", lang),
			displayCatalogFieldName("custom_model_name", lang),
			displayCatalogFieldName("enabled", lang),
		}
		if lang == "zh" {
			return strings.Join([]string{
				"### 模型配置领域约束",
				"- 当前领域是 AI 模型配置，不是交易所配置。",
				"- provider 指模型厂商，不是交易所类型。",
				"- 关键字段：" + strings.Join(fields, "、"),
				"- 候选 provider：" + modelProviderSummaryList(lang),
				"- 推荐 provider：claw402。claw402 是 NOFXi 官方推荐方案，按次付费，使用 Base 链 EVM 钱包 + USDC 支付。",
				"- 如果用户不确定选哪个 provider，可以优先推荐 claw402 并说明其优势，但绝不能替用户自动选中 claw402；必须先展示完整 provider 选项并让用户自己选择。",
				"- 如果 provider 还没选定，下一步必须先让用户从完整 provider 列表里选一个，不能先收集 API Key、钱包私钥或其他凭证。",
				"- 普通 provider（openai/deepseek/claude 等）通常要填 API Key；custom_model_name 和 custom_api_url 可以留空走默认值。",
				"- claw402 需要钱包私钥，custom_model_name 留空时默认 deepseek。",
				"- blockrun-base / blockrun-sol 走钱包私钥模式，不需要 custom_api_url，custom_model_name 默认 auto。",
			}, "\n")
		}
		return strings.Join([]string{
			"### Model Config Domain Guard",
			"- The current domain is AI model configuration, not exchange configuration.",
			"- provider means the model vendor, not an exchange venue.",
			"- Key fields: " + strings.Join(fields, ", "),
			"- Supported providers: " + modelProviderSummaryList(lang),
			"- Recommended provider: claw402. claw402 is the NOFXi recommended pay-per-use option that uses a Base chain wallet + USDC.",
			"- If the user is unsure which provider to pick, you may recommend claw402 and explain its advantages, but you must not auto-select claw402 for them. Show the full provider options first and let the user choose.",
			"- If provider is still missing, the next step must be to ask the user to choose one from the full provider list. Do not ask for an API key, wallet private key, or other credentials before the provider is chosen.",
			"- Standard providers (openai/deepseek/claude etc.) usually require an API key; `custom_model_name` and `custom_api_url` can be omitted to use defaults.",
			"- claw402 uses a wallet private key and defaults to `deepseek` if `custom_model_name` is omitted.",
			"- blockrun-base / blockrun-sol use wallet private keys, do not need `custom_api_url`, and default to `auto`.",
		}, "\n")
	case "exchange_management":
		fields := []string{
			slotDisplayName("exchange_type", lang),
			displayCatalogFieldName("account_name", lang),
			displayCatalogFieldName("api_key", lang),
			displayCatalogFieldName("secret_key", lang),
			displayCatalogFieldName("passphrase", lang),
			displayCatalogFieldName("enabled", lang),
		}
		if lang == "zh" {
			return strings.Join([]string{
				"### 交易所配置领域约束",
				"- 当前领域是交易所账户配置，不是 AI 模型配置。",
				"- exchange_type 指交易所类型，provider 这个词不应用来代指交易所。",
				"- 关键字段：" + strings.Join(fields, "、"),
				"- 支持的交易所类型：" + strings.Join(enumOptionValues("exchange_management", "exchange_type"), "、"),
			}, "\n")
		}
		return strings.Join([]string{
			"### Exchange Config Domain Guard",
			"- The current domain is exchange account configuration, not AI model configuration.",
			"- exchange_type means the trading venue. Do not use provider to mean an exchange.",
			"- Key fields: " + strings.Join(fields, ", "),
			"- Supported exchange types: " + strings.Join(enumOptionValues("exchange_management", "exchange_type"), ", "),
		}, "\n")
	case "trader_management":
		fields := []string{
			slotDisplayName("name", lang),
			slotDisplayName("exchange", lang),
			slotDisplayName("model", lang),
			slotDisplayName("strategy", lang),
			displayCatalogFieldName("scan_interval_minutes", lang),
		}
		if lang == "zh" {
			return strings.Join([]string{
				"### 交易员配置领域约束",
				"- 交易员是装配层，负责创建、换绑策略/交易所/模型，以及启动、停止、删除、查询。",
				"- 编辑交易员时，默认只处理绑定关系；不要顺手改策略、模型、交易所内部配置。",
				"- 交易员初始余额由系统在创建时自动读取绑定交易所账户净值，不接受手动设置、充值或人为改余额。",
				"- 若用户要改策略参数、模型配置或交易所凭证，应切到对应 management skill。",
				"- 创建交易员时最关键的是：名称、交易所、模型、策略。",
				"- 关键字段：" + strings.Join(fields, "、"),
			}, "\n")
		}
		return strings.Join([]string{
			"### Trader Config Domain Guard",
			"- Traders are the assembly layer: create, rebind strategy/exchange/model, and control lifecycle.",
			"- When editing a trader, default to changing bindings only; do not silently edit the internals of the strategy, model, or exchange.",
			"- Trader initial balance is auto-read from the bound exchange account equity at creation time; do not ask the user to set, top up, or manually edit trader balance.",
			"- If the user wants to change strategy parameters, model config, or exchange credentials, switch to the corresponding management skill.",
			"- The key create fields are name, exchange, model, and strategy.",
			"- Key fields: " + strings.Join(fields, ", "),
		}, "\n")
	case "strategy_management":
		fields := []string{
			slotDisplayName("name", lang),
			displayCatalogFieldName("strategy_type", lang),
		}
		if lang == "zh" {
			return strings.Join([]string{
				"### 策略配置领域约束",
				"- 本领域只处理策略模板。",
				"- strategy_type 选项：ai_trading、grid_trading。",
				"- 用户提到 AI500、OI Top、OI Low、静态币种/固定币种这类选币来源时，属于 ai_trading。",
				"- 策略类型确定后，只能使用当前类型的产品编辑页模板。",
				"- 策略类型未确定时，只判断类型，不要展示或混合任一分支的具体配置字段。",
				"- 关键字段：" + strings.Join(fields, "、"),
			}, "\n")
		}
		return strings.Join([]string{
			"### Strategy Config Domain Guard",
			"- This domain only handles strategy templates.",
			"- strategy_type options: ai_trading, grid_trading.",
			"- AI500, OI Top, OI Low, and static coin-source requests imply ai_trading.",
			"- Once strategy_type is known, use only that product editor template.",
			"- Before strategy_type is known, only determine the type; do not show or mix concrete fields from either branch.",
			"- Key fields: " + strings.Join(fields, ", "),
		}, "\n")
	default:
		return ""
	}
}

func buildSkillDomainPrimerForSession(lang string, session skillSession) string {
	if session.Name != "strategy_management" {
		return buildSkillDomainPrimer(lang, session.Name)
	}
	strategyType := explicitStrategyCreateType(session)
	if strategyType == "" {
		return buildSkillDomainPrimer(lang, session.Name)
	}
	if lang == "zh" {
		switch strategyType {
		case "ai_trading":
			return strings.Join([]string{
				"### AI 策略模板",
				"- 只使用 ai_trading 模板：strategy_type + ai_config + publish_config。",
				"- config_patch 必须使用产品 schema 原值，不要使用展示文案：strategy_type=ai_trading；source_type 只能是 static、ai500、oi_top、oi_low；没有 mixed/混合模式。",
				"- 时间周期必须输出为产品枚举字符串，例如 1m、3m、5m、15m、1h；selected_timeframes 必须是字符串数组，例如 [\"1m\",\"5m\",\"15m\"]，不要输出 JSON 字符串。",
				"- AI500/OI Top/OI Low 选币数量范围 1～10；static_coins 最多 10 个；selected_timeframes 最多 4 个；primary_count 10～30。",
				"- BTC/ETH 最大杠杆 1～20；山寨币最大杠杆 1～20；min_confidence 50～100；min_risk_reward_ratio 1～10。",
				"- AI 策略创建方案不要展示或询问非 AI 模板字段：投入金额、每笔固定投入、止损、日亏损限制、最大回撤、网格字段。",
			}, "\n")
		case "grid_trading":
			return strings.Join([]string{
				"### 网格策略模板",
				"- 只使用 grid_trading 模板：strategy_type + grid_config + publish_config；config_patch 必须使用产品 schema 原值，strategy_type=grid_trading。",
				"- 交易对选项：BTCUSDT、ETHUSDT、SOLUSDT、BNBUSDT、XRPUSDT、DOGEUSDT。",
				"- grid_count 5～50；total_investment 最小 100；leverage 1～5；atr_multiplier 1～5。",
				"- total_investment 是用户实际投入/保证金预算，不是杠杆后的名义仓位；最大名义仓位约等于 total_investment × leverage。用户说“投入/总投入/本金/保证金”时默认映射到 total_investment。",
				"- max_drawdown_pct 5～50；stop_loss_pct 1～20；daily_loss_limit_pct 1～30；direction_bias_ratio 0.55～0.90。",
				"- 没有实时行情工具结果时，不要猜当前价格或手动价格上下界；推荐 use_atr_bounds=true 的 ATR 自动边界。",
				"- 如果用户让你选择/推荐剩余网格参数，价格区间默认写入 use_atr_bounds=true；不要反问用户手动价格区间，也不要编造“当前 BTC/ETH 在某价附近”。",
			}, "\n")
		}
	}
	switch strategyType {
	case "ai_trading":
		return strings.Join([]string{
			"### AI Strategy Template",
			"- Use only ai_trading: strategy_type + ai_config + publish_config.",
			"- config_patch must use product schema raw values, not display labels: strategy_type=ai_trading; source_type is only static, ai500, oi_top, or oi_low; no mixed mode.",
			"- Timeframes must be product enum strings such as 1m, 3m, 5m, 15m, 1h; selected_timeframes must be a JSON string array such as [\"1m\",\"5m\",\"15m\"], not a JSON-encoded string.",
			"- AI500/OI source counts 1-10; static_coins at most 10; selected_timeframes at most 4; primary_count 10-30.",
			"- BTC/ETH leverage 1-20; altcoin leverage 1-20; min_confidence 50-100; min_risk_reward_ratio 1-10.",
			"- Do not show or ask for non-AI-template fields in AI strategy drafts: investment amount, fixed per-trade amount, stop loss, daily loss limit, max drawdown, or grid fields.",
		}, "\n")
	case "grid_trading":
		return strings.Join([]string{
			"### Grid Strategy Template",
			"- Use only grid_trading: strategy_type + grid_config + publish_config; config_patch must use product schema raw values with strategy_type=grid_trading.",
			"- Symbol options: BTCUSDT, ETHUSDT, SOLUSDT, BNBUSDT, XRPUSDT, DOGEUSDT.",
			"- grid_count 5-50; total_investment >=100; leverage 1-5; atr_multiplier 1-5.",
			"- total_investment is the user's actual capital/margin budget, not leveraged notional exposure; maximum notional exposure is approximately total_investment * leverage. When the user says investment, capital, amount to put in, or margin, map it to total_investment by default.",
			"- max_drawdown_pct 5-50; stop_loss_pct 1-20; daily_loss_limit_pct 1-30; direction_bias_ratio 0.55-0.90.",
			"- Without fresh market data, do not guess the current price or manual upper/lower prices; recommend ATR auto bounds with use_atr_bounds=true.",
			"- If the user asks you to choose/recommend the remaining grid parameters, default the price range to use_atr_bounds=true; do not ask for manual price bounds or invent statements like the current BTC/ETH price is near a value.",
		}, "\n")
	}
	return buildSkillDomainPrimer(lang, session.Name)
}

func buildManagementDomainPrimer(lang string) string {
	if lang == "zh" {
		return strings.Join([]string{
			"### 管理领域路由速记",
			"- 模型/API Key/provider：model_management。",
			"- 交易所账户/API 凭证：exchange_management。",
			"- 交易员创建、启动、停止、绑定策略/模型/交易所：trader_management。",
			"- 策略模板创建、查看、修改、删除、激活、复制：strategy_management。",
			"- 这里只用于路由；具体字段和模板只在进入对应 skill 后注入。",
		}, "\n")
	}
	return strings.Join([]string{
		"### Management Routing Cheat Sheet",
		"- Model/API key/provider: model_management.",
		"- Exchange account/API credentials: exchange_management.",
		"- Trader create/start/stop/bind strategy/model/exchange: trader_management.",
		"- Strategy template create/query/update/delete/activate/duplicate: strategy_management.",
		"- This is only for routing; detailed fields/templates are injected after entering the selected skill.",
	}, "\n")
}
