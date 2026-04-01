package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"nofx/logger"
	"nofx/store"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AI trader management related structures
type CreateTraderRequest struct {
	Name                string  `json:"name" binding:"required"`
	AIModelID           string  `json:"ai_model_id" binding:"required"`
	ExchangeID          string  `json:"exchange_id" binding:"required"`
	StrategyID          string  `json:"strategy_id"` // Strategy ID (new version)
	InitialBalance      float64 `json:"initial_balance"`
	ScanIntervalMinutes int     `json:"scan_interval_minutes"`
	IsCrossMargin       *bool   `json:"is_cross_margin"`     // Pointer type, nil means use default value true
	ShowInCompetition   *bool   `json:"show_in_competition"` // Pointer type, nil means use default value true
	// The following fields are kept for backward compatibility, new version uses strategy config
	BTCETHLeverage       int    `json:"btc_eth_leverage"`
	AltcoinLeverage      int    `json:"altcoin_leverage"`
	TradingSymbols       string `json:"trading_symbols"`
	CustomPrompt         string `json:"custom_prompt"`
	OverrideBasePrompt   bool   `json:"override_base_prompt"`
	SystemPromptTemplate string `json:"system_prompt_template"` // System prompt template name
	UseAI500             bool   `json:"use_ai500"`
	UseOITop             bool   `json:"use_oi_top"`
}

// UpdateTraderRequest Update trader request
type UpdateTraderRequest struct {
	Name                string  `json:"name" binding:"required"`
	AIModelID           string  `json:"ai_model_id" binding:"required"`
	ExchangeID          string  `json:"exchange_id" binding:"required"`
	StrategyID          string  `json:"strategy_id"` // Strategy ID (new version)
	InitialBalance      float64 `json:"initial_balance"`
	ScanIntervalMinutes int     `json:"scan_interval_minutes"`
	IsCrossMargin       *bool   `json:"is_cross_margin"`
	ShowInCompetition   *bool   `json:"show_in_competition"`
	// The following fields are kept for backward compatibility, new version uses strategy config
	BTCETHLeverage       int    `json:"btc_eth_leverage"`
	AltcoinLeverage      int    `json:"altcoin_leverage"`
	TradingSymbols       string `json:"trading_symbols"`
	CustomPrompt         string `json:"custom_prompt"`
	OverrideBasePrompt   bool   `json:"override_base_prompt"`
	SystemPromptTemplate string `json:"system_prompt_template"`
}

func formatTraderCreationError(reason, nextStep string) string {
	if nextStep == "" {
		return fmt.Sprintf("这次未能创建机器人：%s。", reason)
	}
	return fmt.Sprintf("这次未能创建机器人：%s。%s。", reason, nextStep)
}

func traderCreationRequestError(reason string) string {
	return formatTraderCreationError(reason, "请检查你刚刚填写的内容后，再重新提交")
}

func exchangeDisplayName(exchange *store.Exchange) string {
	if exchange == nil {
		return "所选交易所账户"
	}
	if exchange.AccountName != "" {
		return fmt.Sprintf("%s（%s）", exchange.Name, exchange.AccountName)
	}
	if exchange.Name != "" {
		return exchange.Name
	}
	return "所选交易所账户"
}

func missingExchangeFields(exchange *store.Exchange) []string {
	if exchange == nil {
		return nil
	}

	var missing []string
	switch exchange.ExchangeType {
	case "binance", "bybit", "gate", "indodax":
		if exchange.APIKey == "" {
			missing = append(missing, "API Key")
		}
		if exchange.SecretKey == "" {
			missing = append(missing, "Secret Key")
		}
	case "okx", "bitget", "kucoin":
		if exchange.APIKey == "" {
			missing = append(missing, "API Key")
		}
		if exchange.SecretKey == "" {
			missing = append(missing, "Secret Key")
		}
		if exchange.Passphrase == "" {
			missing = append(missing, "Passphrase")
		}
	case "hyperliquid":
		if exchange.APIKey == "" {
			missing = append(missing, "私钥")
		}
		if strings.TrimSpace(exchange.HyperliquidWalletAddr) == "" {
			missing = append(missing, "钱包地址")
		}
	case "aster":
		if strings.TrimSpace(exchange.AsterUser) == "" {
			missing = append(missing, "Aster User")
		}
		if strings.TrimSpace(exchange.AsterSigner) == "" {
			missing = append(missing, "Aster Signer")
		}
		if exchange.AsterPrivateKey == "" {
			missing = append(missing, "Aster Private Key")
		}
	case "lighter":
		if strings.TrimSpace(exchange.LighterWalletAddr) == "" {
			missing = append(missing, "钱包地址")
		}
		if exchange.LighterAPIKeyPrivateKey == "" {
			missing = append(missing, "API Key Private Key")
		}
	}

	return missing
}

func mapStringPairs(kv ...string) map[string]string {
	if len(kv) == 0 {
		return nil
	}

	params := make(map[string]string, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		params[kv[i]] = kv[i+1]
	}
	return params
}

func validateExchangeForTraderCreation(exchange *store.Exchange) (string, string, map[string]string) {
	if exchange == nil {
		return formatTraderCreationError("还没有找到你选择的交易所账户", "请前往「设置 > 交易所配置」先添加一个可用账户，再回来创建机器人"),
			"trader.create.exchange_not_found", nil
	}
	if !exchange.Enabled {
		return formatTraderCreationError(
			fmt.Sprintf("交易所账户「%s」目前处于未启用状态", exchangeDisplayName(exchange)),
			"请前往「设置 > 交易所配置」启用该账户后，再重新创建机器人",
		), "trader.create.exchange_disabled", mapStringPairs("exchange_name", exchangeDisplayName(exchange))
	}

	missing := missingExchangeFields(exchange)
	if len(missing) > 0 {
		return formatTraderCreationError(
			fmt.Sprintf("交易所账户「%s」的配置还不完整，缺少 %s", exchangeDisplayName(exchange), strings.Join(missing, "、")),
			"请前往「设置 > 交易所配置」补全该账户的必填信息后，再重新创建机器人",
		), "trader.create.exchange_missing_fields", mapStringPairs(
			"exchange_name", exchangeDisplayName(exchange),
			"missing_fields", strings.Join(missing, ", "),
		)
	}

	switch exchange.ExchangeType {
	case "binance", "bybit", "okx", "bitget", "gate", "kucoin", "hyperliquid", "aster", "lighter", "indodax":
		return "", "", nil
	default:
		return formatTraderCreationError(
			fmt.Sprintf("交易所账户「%s」使用了当前版本暂不支持的类型 %s", exchangeDisplayName(exchange), exchange.ExchangeType),
			"请改用当前版本支持的交易所账户后，再重新创建机器人",
		), "trader.create.exchange_unsupported", mapStringPairs(
			"exchange_name", exchangeDisplayName(exchange),
			"exchange_type", exchange.ExchangeType,
		)
	}
}

func classifyTraderSetupReason(reason string) (string, string) {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return "", ""
	}

	lower := strings.ToLower(trimmed)

	switch {
	case strings.Contains(lower, "failed to parse strategy config"),
		strings.Contains(lower, "failed to parse strategy configuration"):
		return "trader.reason.strategy_config_invalid", "当前策略配置内容已损坏，系统暂时无法解析"
	case strings.Contains(lower, "has no strategy configured"):
		return "trader.reason.strategy_missing", "当前机器人缺少有效的交易策略配置"
	case strings.Contains(lower, "failed to parse private key"),
		(strings.Contains(lower, "invalid hex character") && strings.Contains(lower, "private key")):
		return "trader.reason.private_key_invalid", "私钥格式不正确，系统无法识别"
	case strings.Contains(lower, "failed to initialize hyperliquid trader"):
		return "trader.reason.hyperliquid_init_failed", "Hyperliquid 账户初始化失败，请确认私钥、主钱包地址和 Agent Wallet 配置是否正确"
	case strings.Contains(lower, "failed to initialize aster trader"):
		return "trader.reason.aster_init_failed", "Aster 账户初始化失败，请确认 Aster User、Signer 和私钥是否正确"
	case strings.Contains(lower, "failed to get meta information"):
		return "trader.reason.exchange_meta_unavailable", "系统暂时无法从交易所读取账户元信息"
	case strings.Contains(lower, "security check failed") && strings.Contains(lower, "agent wallet balance too high"):
		return "trader.reason.hyperliquid_agent_balance_too_high", "Hyperliquid Agent Wallet 余额过高，不符合当前安全要求"
	case strings.Contains(lower, "failed to initialize account"):
		return "trader.reason.exchange_account_init_failed", "交易所账户初始化失败，请确认钱包地址和 API Key 是否匹配"
	case strings.Contains(lower, "unsupported trading platform"):
		return "trader.reason.exchange_unsupported", "当前交易所类型暂不支持机器人初始化"
	case strings.Contains(lower, "initial balance not set and unable to fetch balance from exchange"):
		return "trader.reason.exchange_balance_unavailable", "系统暂时无法从交易所读取账户余额"
	case strings.Contains(lower, "timeout"), strings.Contains(lower, "no such host"), strings.Contains(lower, "connection refused"):
		return "trader.reason.exchange_service_unreachable", "系统暂时无法连接交易所服务"
	default:
		return "trader.reason.unknown", trimmed
	}
}

func humanizeTraderSetupReason(reason string) string {
	_, message := classifyTraderSetupReason(reason)
	return message
}

func traderSetupReasonParams(err error, fallback string, kv ...string) map[string]string {
	params := mapStringPairs(kv...)
	rawReason := SanitizeError(err, fallback)
	reasonKey, reasonMessage := classifyTraderSetupReason(rawReason)
	if reasonMessage == "" && fallback != "" {
		reasonMessage = fallback
	}
	if reasonMessage != "" {
		if params == nil {
			params = map[string]string{}
		}
		params["reason"] = reasonMessage
	}
	if reasonKey != "" {
		if params == nil {
			params = map[string]string{}
		}
		params["reason_key"] = reasonKey
	}
	return params
}

func describeTraderLoadError(traderName string, err error) string {
	if err == nil {
		return formatTraderCreationError("机器人配置虽然保存了，但运行实例没有成功初始化", "请检查模型、策略和交易所配置是否完整，然后再试一次")
	}

	reason := humanizeTraderSetupReason(SanitizeError(err, ""))
	if reason == "" {
		return formatTraderCreationError(
			fmt.Sprintf("机器人「%s」在初始化运行实例时没有成功启动", traderName),
			"请检查模型、策略和交易所配置是否完整，然后再试一次",
		)
	}

	return formatTraderCreationError(
		fmt.Sprintf("机器人「%s」在初始化运行实例时没有成功启动，原因是：%s", traderName, reason),
		"请检查模型、策略和交易所配置是否完整，然后再试一次",
	)
}

func describeTraderCreationWarning(traderName string, err error) string {
	if err == nil {
		return fmt.Sprintf("机器人「%s」已经保存，但当前还没有通过启动前校验。请先检查模型、策略和交易所配置，修正后再点击启动。", traderName)
	}

	reason := humanizeTraderSetupReason(SanitizeError(err, ""))
	if reason == "" {
		return fmt.Sprintf("机器人「%s」已经保存，但当前暂时还不能启动。请先检查模型、策略和交易所配置，修正后再点击启动。", traderName)
	}

	return fmt.Sprintf("机器人「%s」已经保存，但当前暂时还不能启动，原因是：%s。请先检查模型、策略和交易所配置，修正后再点击启动。", traderName, reason)
}

func describeTraderStartError(traderName string, err error) string {
	if err == nil {
		return fmt.Sprintf("这次未能启动机器人：机器人「%s」暂时还不能启动。请检查模型、策略和交易所配置后，再重新点击启动。", traderName)
	}

	reason := humanizeTraderSetupReason(SanitizeError(err, ""))
	if reason == "" {
		return fmt.Sprintf("这次未能启动机器人：机器人「%s」暂时还不能启动。请检查模型、策略和交易所配置后，再重新点击启动。", traderName)
	}

	return fmt.Sprintf("这次未能启动机器人：机器人「%s」暂时还不能启动，原因是：%s。请检查模型、策略和交易所配置后，再重新点击启动。", traderName, reason)
}

func formatTraderStartError(reason, nextStep string) string {
	if nextStep == "" {
		return fmt.Sprintf("这次未能启动机器人：%s。", reason)
	}
	return fmt.Sprintf("这次未能启动机器人：%s。%s。", reason, nextStep)
}

// handleCreateTrader Create new AI trader
func (s *Server) handleCreateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	var req CreateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequestWithDetails(c, traderCreationRequestError("提交的信息不完整，或者格式不正确"), "trader.create.invalid_request", nil)
		return
	}

	// Validate leverage values
	if req.BTCETHLeverage < 0 || req.BTCETHLeverage > 50 {
		SafeBadRequestWithDetails(c, traderCreationRequestError("BTC/ETH 杠杆倍数需要在 1 到 50 倍之间"), "trader.create.invalid_btc_eth_leverage", nil)
		return
	}
	if req.AltcoinLeverage < 0 || req.AltcoinLeverage > 20 {
		SafeBadRequestWithDetails(c, traderCreationRequestError("山寨币杠杆倍数需要在 1 到 20 倍之间"), "trader.create.invalid_altcoin_leverage", nil)
		return
	}

	// Validate trading symbol format
	if req.TradingSymbols != "" {
		symbols := strings.Split(req.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" && !strings.HasSuffix(strings.ToUpper(symbol), "USDT") {
				SafeBadRequestWithDetails(c, traderCreationRequestError(
					fmt.Sprintf("交易对 %s 的格式不正确，目前只支持以 USDT 结尾的合约交易对", symbol),
				), "trader.create.invalid_symbol", mapStringPairs("symbol", symbol))
				return
			}
		}
	}

	model, err := s.store.AIModel().Get(userID, req.AIModelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			SafeBadRequestWithDetails(c, formatTraderCreationError("还没有找到你选择的 AI 模型", "请前往「设置 > 模型配置」先添加并启用一个可用模型，再回来创建机器人"), "trader.create.model_not_found", nil)
			return
		}
		SafeError(c, http.StatusInternalServerError,
			formatTraderCreationError("暂时无法读取你的 AI 模型配置", "请稍后重试；如果问题持续，再检查本地服务是否正常"),
			err,
		)
		return
	}
	if !model.Enabled {
		SafeBadRequestWithDetails(c, formatTraderCreationError(
			fmt.Sprintf("AI 模型「%s」目前还没有启用", model.Name),
			"请前往「设置 > 模型配置」启用它后，再重新创建机器人",
		), "trader.create.model_disabled", mapStringPairs("model_name", model.Name))
		return
	}
	if model.APIKey == "" {
		SafeBadRequestWithDetails(c, formatTraderCreationError(
			fmt.Sprintf("AI 模型「%s」缺少 API Key 或支付凭证", model.Name),
			"请前往「设置 > 模型配置」补全模型凭证后，再重新创建机器人",
		), "trader.create.model_missing_credentials", mapStringPairs("model_name", model.Name))
		return
	}

	if req.StrategyID == "" {
		SafeBadRequestWithDetails(c, formatTraderCreationError("你还没有选择交易策略", "请先选择一个策略，再继续创建机器人"), "trader.create.strategy_required", nil)
		return
	}

	if req.StrategyID != "" {
		_, err = s.store.Strategy().Get(userID, req.StrategyID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				SafeBadRequestWithDetails(c, formatTraderCreationError("你选择的策略不存在，或者已经被删除了", "请重新选择一个可用策略后，再继续创建机器人"), "trader.create.strategy_not_found", nil)
				return
			}
			SafeError(c, http.StatusInternalServerError,
				formatTraderCreationError("暂时无法读取你选择的策略配置", "请稍后重试；如果问题持续，再检查本地服务是否正常"),
				err,
			)
			return
		}
	}

	// Generate trader ID (use short UUID prefix for readability)
	exchangeIDShort := req.ExchangeID
	if len(exchangeIDShort) > 8 {
		exchangeIDShort = exchangeIDShort[:8]
	}
	traderID := fmt.Sprintf("%s_%s_%d", exchangeIDShort, req.AIModelID, time.Now().Unix())

	// Set default values
	isCrossMargin := true // Default to cross margin mode
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	showInCompetition := true // Default to show in competition
	if req.ShowInCompetition != nil {
		showInCompetition = *req.ShowInCompetition
	}

	// Set leverage default values
	btcEthLeverage := 10 // Default value
	altcoinLeverage := 5 // Default value
	if req.BTCETHLeverage > 0 {
		btcEthLeverage = req.BTCETHLeverage
	}
	if req.AltcoinLeverage > 0 {
		altcoinLeverage = req.AltcoinLeverage
	}

	// Set system prompt template default value
	systemPromptTemplate := "default"
	if req.SystemPromptTemplate != "" {
		systemPromptTemplate = req.SystemPromptTemplate
	}

	// Set scan interval default value
	scanIntervalMinutes := req.ScanIntervalMinutes
	if scanIntervalMinutes < 3 {
		scanIntervalMinutes = 3 // Default 3 minutes, not allowed to be less than 3
	}

	// Query exchange actual balance, override user input
	actualBalance := req.InitialBalance // Default to use user input
	exchanges, err := s.store.Exchange().List(userID)
	if err != nil {
		SafeError(c, http.StatusInternalServerError,
			formatTraderCreationError("暂时无法读取你的交易所配置", "请稍后重试；如果问题持续，再检查本地服务是否正常"),
			err,
		)
		return
	}

	// Find matching exchange configuration
	var exchangeCfg *store.Exchange
	for _, ex := range exchanges {
		if ex.ID == req.ExchangeID {
			exchangeCfg = ex
			break
		}
	}

	if exchangeMsg, exchangeErrorKey, exchangeErrorParams := validateExchangeForTraderCreation(exchangeCfg); exchangeMsg != "" {
		SafeBadRequestWithDetails(c, exchangeMsg, exchangeErrorKey, exchangeErrorParams)
		return
	}

	{
		tempTrader, createErr := buildExchangeProbeTrader(exchangeCfg, userID)
		if createErr != nil {
			SafeBadRequestWithDetails(c, formatTraderCreationError(
				fmt.Sprintf("交易所账户「%s」没有通过初始化校验，原因是：%s", exchangeDisplayName(exchangeCfg), humanizeTraderSetupReason(SanitizeError(createErr, "配置校验未通过"))),
				"请前往「设置 > 交易所配置」检查这个账户的密钥、地址和账户信息是否填写正确",
			), "trader.create.exchange_probe_failed", traderSetupReasonParams(createErr, "配置校验未通过",
				"exchange_name", exchangeDisplayName(exchangeCfg),
			))
			return
		} else if tempTrader != nil {
			// Query actual balance
			balanceInfo, balanceErr := tempTrader.GetBalance()
			if balanceErr != nil {
				logger.Infof("⚠️ Failed to query exchange balance, using user input for initial balance: %v", balanceErr)
			} else {
				if extractedBalance, found := extractExchangeTotalEquity(balanceInfo); found {
					actualBalance = extractedBalance
					logger.Infof("✓ Queried exchange total equity: %.2f %s (user input: %.2f)",
						actualBalance, accountAssetForExchange(exchangeCfg.ExchangeType), req.InitialBalance)
				} else {
					logger.Infof("⚠️ Unable to extract total equity from balance info, balanceInfo=%v, using user input for initial balance", balanceInfo)
				}
			}
		}
	}

	// Create trader configuration (database entity)
	logger.Infof("🔧 DEBUG: Starting to create trader config, ID=%s, Name=%s, AIModel=%s, Exchange=%s, StrategyID=%s", traderID, req.Name, req.AIModelID, req.ExchangeID, req.StrategyID)
	traderRecord := &store.Trader{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		StrategyID:           req.StrategyID, // Associated strategy ID (new version)
		InitialBalance:       actualBalance,  // Use actual queried balance
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       req.TradingSymbols,
		UseAI500:             req.UseAI500,
		UseOITop:             req.UseOITop,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: systemPromptTemplate,
		IsCrossMargin:        isCrossMargin,
		ShowInCompetition:    showInCompetition,
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            false,
	}

	// Save to database
	logger.Infof("🔧 DEBUG: Preparing to call CreateTrader")
	err = s.store.Trader().Create(traderRecord)
	if err != nil {
		logger.Infof("❌ Failed to create trader: %v", err)
		publicMsg := SanitizeError(err, formatTraderCreationError("机器人配置没有保存成功", "请检查名称、模型、策略和交易所配置后，再试一次"))
		statusCode := http.StatusBadRequest
		if publicMsg == formatTraderCreationError("机器人配置没有保存成功", "请检查名称、模型、策略和交易所配置后，再试一次") {
			statusCode = http.StatusInternalServerError
		}
		SafeError(c, statusCode, publicMsg, err)
		return
	}
	logger.Infof("🔧 DEBUG: CreateTrader succeeded")

	// Immediately load new trader into TraderManager
	logger.Infof("🔧 DEBUG: Preparing to call LoadUserTraders")
	startupWarning := ""
	err = s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to load user traders into memory: %v", err)
		startupWarning = describeTraderCreationWarning(req.Name, err)
	}
	logger.Infof("🔧 DEBUG: LoadUserTraders completed")

	if startupWarning == "" {
		if loadErr := s.traderManager.GetLoadError(traderID); loadErr != nil {
		logger.Infof("⚠️ Trader %s failed to load after creation: %v", traderID, loadErr)
			startupWarning = describeTraderCreationWarning(req.Name, loadErr)
		}
	}

	if startupWarning == "" {
		if _, getErr := s.traderManager.GetTrader(traderID); getErr != nil {
		logger.Infof("⚠️ Trader %s not found in memory after creation: %v", traderID, getErr)
			startupWarning = describeTraderCreationWarning(req.Name, getErr)
		}
	}

	logger.Infof("✓ Trader created successfully: %s (model: %s, exchange: %s)", req.Name, req.AIModelID, req.ExchangeID)

	c.JSON(http.StatusCreated, gin.H{
		"trader_id":        traderID,
		"trader_name":      req.Name,
		"ai_model":         req.AIModelID,
		"is_running":       false,
		"startup_warning":  startupWarning,
	})
}

// handleUpdateTrader Update trader configuration
func (s *Server) handleUpdateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	var req UpdateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Check if trader exists and belongs to current user
	traders, err := s.store.Trader().List(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trader list"})
		return
	}

	var existingTrader *store.Trader
	for _, t := range traders {
		if t.ID == traderID {
			existingTrader = t
			break
		}
	}

	if existingTrader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	// Set default values
	isCrossMargin := existingTrader.IsCrossMargin // Keep original value
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	showInCompetition := existingTrader.ShowInCompetition // Keep original value
	if req.ShowInCompetition != nil {
		showInCompetition = *req.ShowInCompetition
	}

	// Set leverage default values
	btcEthLeverage := req.BTCETHLeverage
	altcoinLeverage := req.AltcoinLeverage
	if btcEthLeverage <= 0 {
		btcEthLeverage = existingTrader.BTCETHLeverage // Keep original value
	}
	if altcoinLeverage <= 0 {
		altcoinLeverage = existingTrader.AltcoinLeverage // Keep original value
	}

	// Set scan interval, allow updates
	scanIntervalMinutes := req.ScanIntervalMinutes
	logger.Infof("📊 Update trader scan_interval: req=%d, existing=%d", req.ScanIntervalMinutes, existingTrader.ScanIntervalMinutes)
	if scanIntervalMinutes <= 0 {
		scanIntervalMinutes = existingTrader.ScanIntervalMinutes // Keep original value
	} else if scanIntervalMinutes < 3 {
		scanIntervalMinutes = 3
	}
	logger.Infof("📊 Final scan_interval_minutes: %d", scanIntervalMinutes)

	// Set system prompt template
	systemPromptTemplate := req.SystemPromptTemplate
	if systemPromptTemplate == "" {
		systemPromptTemplate = existingTrader.SystemPromptTemplate // Keep original value
	}

	// Handle strategy ID (if not provided, keep original value)
	strategyID := req.StrategyID
	if strategyID == "" {
		strategyID = existingTrader.StrategyID
	}

	exchangeChanged := req.ExchangeID != "" && req.ExchangeID != existingTrader.ExchangeID
	resetInitialBalance := exchangeChanged && req.InitialBalance <= 0

	initialBalance := existingTrader.InitialBalance
	if req.InitialBalance > 0 {
		initialBalance = req.InitialBalance
	}
	if resetInitialBalance {
		initialBalance = 0
	}

	// Update trader configuration
	traderRecord := &store.Trader{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		StrategyID:           strategyID, // Associated strategy ID
		InitialBalance:       initialBalance,
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       req.TradingSymbols,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: systemPromptTemplate,
		IsCrossMargin:        isCrossMargin,
		ShowInCompetition:    showInCompetition,
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            existingTrader.IsRunning, // Keep original value
	}

	// Check if trader was running before update (we'll restart it after)
	wasRunning := false
	if existingMemTrader, memErr := s.traderManager.GetTrader(traderID); memErr == nil {
		status := existingMemTrader.GetStatus()
		if running, ok := status["is_running"].(bool); ok && running {
			wasRunning = true
			logger.Infof("🔄 Trader %s was running, will restart with new config after update", traderID)
		}
	}

	// Update database
	logger.Infof("🔄 Updating trader: ID=%s, Name=%s, AIModelID=%s, StrategyID=%s, ScanInterval=%d min",
		traderRecord.ID, traderRecord.Name, traderRecord.AIModelID, traderRecord.StrategyID, scanIntervalMinutes)
	err = s.store.Trader().Update(traderRecord)
	if err != nil {
		SafeInternalError(c, "Failed to update trader", err)
		return
	}

	if resetInitialBalance {
		logger.Infof("🔄 Exchange changed for trader %s, resetting stale initial_balance to 0", traderID)
		if err := s.store.Trader().UpdateInitialBalance(userID, traderID, 0); err != nil {
			SafeInternalError(c, "Failed to reset trader initial balance", err)
			return
		}
	}

	// Remove old trader from memory first (this also stops if running)
	s.traderManager.RemoveTrader(traderID)

	// Reload traders into memory with fresh config
	err = s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to reload user traders into memory: %v", err)
	}

	// If trader was running before, restart it with new config
	if wasRunning {
		if reloadedTrader, getErr := s.traderManager.GetTrader(traderID); getErr == nil {
			go func() {
				logger.Infof("▶️ Restarting trader %s with new config...", traderID)
				if runErr := reloadedTrader.Run(); runErr != nil {
					logger.Infof("❌ Trader %s runtime error: %v", traderID, runErr)
				}
			}()
		}
	}

	logger.Infof("✓ Trader updated successfully: %s (model: %s, exchange: %s, strategy: %s)", req.Name, req.AIModelID, req.ExchangeID, strategyID)

	c.JSON(http.StatusOK, gin.H{
		"trader_id":   traderID,
		"trader_name": req.Name,
		"ai_model":    req.AIModelID,
		"message":     "Trader updated successfully",
	})
}

// handleDeleteTrader Delete trader
func (s *Server) handleDeleteTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// Delete from database
	err := s.store.Trader().Delete(userID, traderID)
	if err != nil {
		SafeInternalError(c, "Failed to delete trader", err)
		return
	}

	// If trader is running, stop it first
	if trader, err := s.traderManager.GetTrader(traderID); err == nil {
		status := trader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); ok && isRunning {
			trader.Stop()
			logger.Infof("⏹  Stopped running trader: %s", traderID)
		}
	}

	// Remove trader from memory
	s.traderManager.RemoveTrader(traderID)

	logger.Infof("✓ Trader deleted: %s", traderID)
	c.JSON(http.StatusOK, gin.H{"message": "Trader deleted"})
}

// handleStartTrader Start trader
func (s *Server) handleStartTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// Verify trader belongs to current user
	fullCfg, err := s.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist or no access permission"})
		return
	}
	traderName := traderID
	if fullCfg != nil && fullCfg.Trader != nil && fullCfg.Trader.Name != "" {
		traderName = fullCfg.Trader.Name
	}

	// Check if trader exists in memory and if it's running
	existingTrader, _ := s.traderManager.GetTrader(traderID)
	if existingTrader != nil {
		status := existingTrader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); ok && isRunning {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Trader is already running"})
			return
		}
		// Trader exists but is stopped - remove from memory to reload fresh config
		logger.Infof("🔄 Removing stopped trader %s from memory to reload config...", traderID)
		s.traderManager.RemoveTrader(traderID)
	}

	// Load trader from database (always reload to get latest config)
	logger.Infof("🔄 Loading trader %s from database...", traderID)
	if loadErr := s.traderManager.LoadUserTradersFromStore(s.store, userID); loadErr != nil {
		logger.Infof("❌ Failed to load user traders: %v", loadErr)
		SafeErrorWithDetails(c, http.StatusInternalServerError, describeTraderStartError(traderName, loadErr), "trader.start.load_failed", traderSetupReasonParams(loadErr, "", "trader_name", traderName), loadErr)
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		if fullCfg != nil && fullCfg.Trader != nil {
			// Check strategy
			if fullCfg.Strategy == nil {
				SafeBadRequestWithDetails(c, describeTraderStartError(traderName, fmt.Errorf("trader has no strategy configured")), "trader.start.strategy_missing", mapStringPairs("trader_name", traderName))
				return
			}
			// Check AI model
			if fullCfg.AIModel == nil {
				SafeBadRequestWithDetails(c, formatTraderStartError("这个机器人关联的 AI 模型不存在", "请前往「设置 > 模型配置」检查后，再重新点击启动"), "trader.start.model_not_found", mapStringPairs("trader_name", traderName))
				return
			}
			if !fullCfg.AIModel.Enabled {
				SafeBadRequestWithDetails(c, formatTraderStartError(
					fmt.Sprintf("机器人「%s」关联的 AI 模型「%s」目前还没有启用", traderName, fullCfg.AIModel.Name),
					"请前往「设置 > 模型配置」启用它后，再重新点击启动",
				), "trader.start.model_disabled", mapStringPairs("trader_name", traderName, "model_name", fullCfg.AIModel.Name))
				return
			}
			// Check exchange
			if fullCfg.Exchange == nil {
				SafeBadRequestWithDetails(c, formatTraderStartError("这个机器人关联的交易所账户不存在", "请前往「设置 > 交易所配置」检查后，再重新点击启动"), "trader.start.exchange_not_found", mapStringPairs("trader_name", traderName))
				return
			}
			if !fullCfg.Exchange.Enabled {
				SafeBadRequestWithDetails(c, formatTraderStartError(
					fmt.Sprintf("机器人「%s」关联的交易所账户「%s」目前还没有启用", traderName, exchangeDisplayName(fullCfg.Exchange)),
					"请前往「设置 > 交易所配置」启用它后，再重新点击启动",
				), "trader.start.exchange_disabled", mapStringPairs("trader_name", traderName, "exchange_name", exchangeDisplayName(fullCfg.Exchange)))
				return
			}
		}
		// Check if there's a specific load error
		if loadErr := s.traderManager.GetLoadError(traderID); loadErr != nil {
			SafeBadRequestWithDetails(c, describeTraderStartError(traderName, loadErr), "trader.start.load_failed", traderSetupReasonParams(loadErr, "", "trader_name", traderName))
			return
		}
		SafeBadRequestWithDetails(c, describeTraderStartError(traderName, err), "trader.start.setup_invalid", traderSetupReasonParams(err, "", "trader_name", traderName))
		return
	}

	// Start trader
	go func() {
		logger.Infof("▶️  Starting trader %s (%s)", traderID, trader.GetName())
		if err := trader.Run(); err != nil {
			logger.Infof("❌ Trader %s runtime error: %v", trader.GetName(), err)
		}
	}()

	// Update running status in database
	err = s.store.Trader().UpdateStatus(userID, traderID, true)
	if err != nil {
		logger.Infof("⚠️  Failed to update trader status: %v", err)
	}

	logger.Infof("✓ Trader %s started", trader.GetName())
	c.JSON(http.StatusOK, gin.H{"message": "Trader started"})
}

// handleStopTrader Stop trader
func (s *Server) handleStopTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// Verify trader belongs to current user
	_, err := s.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist or no access permission"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	// Check if trader is running
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && !isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trader is already stopped"})
		return
	}

	// Stop trader
	trader.Stop()

	// Update running status in database
	err = s.store.Trader().UpdateStatus(userID, traderID, false)
	if err != nil {
		logger.Infof("⚠️  Failed to update trader status: %v", err)
	}

	logger.Infof("⏹  Trader %s stopped", trader.GetName())
	c.JSON(http.StatusOK, gin.H{"message": "Trader stopped"})
}
