package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"nofx/decision"
	"nofx/market"
	"nofx/mcp"
	"nofx/store"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// handleGetStrategies 获取策略列表
func (s *Server) handleGetStrategies(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	strategies, err := s.store.Strategy().List(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取策略列表失败: " + err.Error()})
		return
	}

	// 转换为前端格式
	result := make([]gin.H, 0, len(strategies))
	for _, st := range strategies {
		var config store.StrategyConfig
		json.Unmarshal([]byte(st.Config), &config)

		result = append(result, gin.H{
			"id":          st.ID,
			"name":        st.Name,
			"description": st.Description,
			"is_active":   st.IsActive,
			"is_default":  st.IsDefault,
			"config":      config,
			"created_at":  st.CreatedAt,
			"updated_at":  st.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"strategies": result,
	})
}

// handleGetStrategy 获取单个策略
func (s *Server) handleGetStrategy(c *gin.Context) {
	userID := c.GetString("user_id")
	strategyID := c.Param("id")

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	strategy, err := s.store.Strategy().Get(userID, strategyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "策略不存在"})
		return
	}

	var config store.StrategyConfig
	json.Unmarshal([]byte(strategy.Config), &config)

	c.JSON(http.StatusOK, gin.H{
		"id":          strategy.ID,
		"name":        strategy.Name,
		"description": strategy.Description,
		"is_active":   strategy.IsActive,
		"is_default":  strategy.IsDefault,
		"config":      config,
		"created_at":  strategy.CreatedAt,
		"updated_at":  strategy.UpdatedAt,
	})
}

// handleCreateStrategy 创建策略
func (s *Server) handleCreateStrategy(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	var req struct {
		Name        string               `json:"name" binding:"required"`
		Description string               `json:"description"`
		Config      store.StrategyConfig `json:"config" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}

	// 序列化配置
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "序列化配置失败"})
		return
	}

	strategy := &store.Strategy{
		ID:          uuid.New().String(),
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		IsActive:    false,
		IsDefault:   false,
		Config:      string(configJSON),
	}

	if err := s.store.Strategy().Create(strategy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建策略失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      strategy.ID,
		"message": "策略创建成功",
	})
}

// handleUpdateStrategy 更新策略
func (s *Server) handleUpdateStrategy(c *gin.Context) {
	userID := c.GetString("user_id")
	strategyID := c.Param("id")

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	// 检查是否是系统默认策略
	existing, err := s.store.Strategy().Get(userID, strategyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "策略不存在"})
		return
	}
	if existing.IsDefault {
		c.JSON(http.StatusForbidden, gin.H{"error": "不能修改系统默认策略"})
		return
	}

	var req struct {
		Name        string               `json:"name"`
		Description string               `json:"description"`
		Config      store.StrategyConfig `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}

	// 序列化配置
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "序列化配置失败"})
		return
	}

	strategy := &store.Strategy{
		ID:          strategyID,
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		Config:      string(configJSON),
	}

	if err := s.store.Strategy().Update(strategy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新策略失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "策略更新成功"})
}

// handleDeleteStrategy 删除策略
func (s *Server) handleDeleteStrategy(c *gin.Context) {
	userID := c.GetString("user_id")
	strategyID := c.Param("id")

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	if err := s.store.Strategy().Delete(userID, strategyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除策略失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "策略删除成功"})
}

// handleActivateStrategy 激活策略
func (s *Server) handleActivateStrategy(c *gin.Context) {
	userID := c.GetString("user_id")
	strategyID := c.Param("id")

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	if err := s.store.Strategy().SetActive(userID, strategyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "激活策略失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "策略激活成功"})
}

// handleDuplicateStrategy 复制策略
func (s *Server) handleDuplicateStrategy(c *gin.Context) {
	userID := c.GetString("user_id")
	sourceID := c.Param("id")

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}

	newID := uuid.New().String()
	if err := s.store.Strategy().Duplicate(userID, sourceID, newID, req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "复制策略失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      newID,
		"message": "策略复制成功",
	})
}

// handleGetActiveStrategy 获取当前激活的策略
func (s *Server) handleGetActiveStrategy(c *gin.Context) {
	userID := c.GetString("user_id")

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	strategy, err := s.store.Strategy().GetActive(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "没有激活的策略"})
		return
	}

	var config store.StrategyConfig
	json.Unmarshal([]byte(strategy.Config), &config)

	c.JSON(http.StatusOK, gin.H{
		"id":          strategy.ID,
		"name":        strategy.Name,
		"description": strategy.Description,
		"is_active":   strategy.IsActive,
		"is_default":  strategy.IsDefault,
		"config":      config,
		"created_at":  strategy.CreatedAt,
		"updated_at":  strategy.UpdatedAt,
	})
}

// handleGetDefaultStrategyConfig 获取默认策略配置模板
func (s *Server) handleGetDefaultStrategyConfig(c *gin.Context) {
	// 返回默认配置结构，供前端创建新策略时使用
	defaultConfig := store.StrategyConfig{
		CoinSource: store.CoinSourceConfig{
			SourceType:    "coinpool",
			UseCoinPool:   true,
			CoinPoolLimit: 30,
			UseOITop:      true,
			OITopLimit:    20,
			StaticCoins:   []string{},
		},
		Indicators: store.IndicatorConfig{
			Klines: store.KlineConfig{
				PrimaryTimeframe:     "5m",
				PrimaryCount:         30,
				LongerTimeframe:      "4h",
				LongerCount:          10,
				EnableMultiTimeframe: true,
				SelectedTimeframes:   []string{"5m", "15m", "1h", "4h"},
			},
			EnableEMA:         true,
			EnableMACD:        true,
			EnableRSI:         true,
			EnableATR:         true,
			EnableVolume:      true,
			EnableOI:          true,
			EnableFundingRate: true,
			EMAPeriods:        []int{20, 50},
			RSIPeriods:        []int{7, 14},
			ATRPeriods:        []int{14},
		},
		RiskControl: store.RiskControlConfig{
			MaxPositions:       3,
			BTCETHMaxLeverage:  5,
			AltcoinMaxLeverage: 5,
			MinRiskRewardRatio: 3.0,
			MaxMarginUsage:     0.9,
			MaxPositionRatio:   1.5,
			MinPositionSize:    12,
			MinConfidence:      75,
		},
		PromptSections: store.PromptSectionsConfig{
			RoleDefinition: `# 你是专业的加密货币交易AI

你专注于技术分析和风险管理，基于市场数据做出理性的交易决策。
你的目标是在控制风险的前提下，捕捉高概率的交易机会。`,
			TradingFrequency: `# ⏱️ 交易频率认知

- 优秀交易员：每天2-4笔 ≈ 每小时0.1-0.2笔
- 每小时>2笔 = 过度交易
- 单笔持仓时间≥30-60分钟
如果你发现自己每个周期都在交易 → 标准过低；若持仓<30分钟就平仓 → 过于急躁。`,
			EntryStandards: `# 🎯 开仓标准（严格）

只在多重信号共振时开仓：
- 趋势方向明确（EMA排列、价格位置）
- 动量确认（MACD、RSI协同）
- 波动率适中（ATR合理范围）
- 量价配合（成交量支持方向）

避免：单一指标、信号矛盾、横盘震荡、刚平仓即重启。`,
			DecisionProcess: `# 📋 决策流程

1. 检查持仓 → 是否该止盈/止损
2. 扫描候选币 + 多时间框 → 是否存在强信号
3. 评估风险回报比 → 是否满足最小要求
4. 先写思维链，再输出结构化JSON`,
		},
	}

	c.JSON(http.StatusOK, defaultConfig)
}

// handlePreviewPrompt 预览策略生成的 Prompt
func (s *Server) handlePreviewPrompt(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	var req struct {
		Config          store.StrategyConfig `json:"config" binding:"required"`
		AccountEquity   float64              `json:"account_equity"`
		PromptVariant   string               `json:"prompt_variant"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}

	// 使用默认值
	if req.AccountEquity <= 0 {
		req.AccountEquity = 1000.0 // 默认模拟账户净值
	}
	if req.PromptVariant == "" {
		req.PromptVariant = "balanced"
	}

	// 创建策略引擎来构建 prompt
	engine := decision.NewStrategyEngine(&req.Config)

	// 构建系统 prompt（使用策略引擎内置的方法）
	systemPrompt := engine.BuildSystemPrompt(
		req.AccountEquity,
		req.PromptVariant,
	)

	// 获取可用的 prompt 模板列表
	templateNames := decision.GetAllPromptTemplateNames()

	c.JSON(http.StatusOK, gin.H{
		"system_prompt":       systemPrompt,
		"prompt_variant":      req.PromptVariant,
		"available_templates": templateNames,
		"config_summary": gin.H{
			"coin_source":      req.Config.CoinSource.SourceType,
			"primary_tf":       req.Config.Indicators.Klines.PrimaryTimeframe,
			"btc_eth_leverage": req.Config.RiskControl.BTCETHMaxLeverage,
			"altcoin_leverage": req.Config.RiskControl.AltcoinMaxLeverage,
			"max_positions":    req.Config.RiskControl.MaxPositions,
		},
	})
}

// handleStrategyTestRun AI 测试运行（不执行交易，只返回 AI 分析结果）
func (s *Server) handleStrategyTestRun(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	var req struct {
		Config        store.StrategyConfig `json:"config" binding:"required"`
		PromptVariant string               `json:"prompt_variant"`
		AIModelID     string               `json:"ai_model_id"`
		RunRealAI     bool                 `json:"run_real_ai"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}

	if req.PromptVariant == "" {
		req.PromptVariant = "balanced"
	}

	// 创建策略引擎来构建 prompt
	engine := decision.NewStrategyEngine(&req.Config)

	// 获取候选币种
	candidates, err := engine.GetCandidateCoins()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":       "获取候选币种失败: " + err.Error(),
			"ai_response": "",
		})
		return
	}

	// 获取时间周期配置
	timeframes := req.Config.Indicators.Klines.SelectedTimeframes
	primaryTimeframe := req.Config.Indicators.Klines.PrimaryTimeframe
	klineCount := req.Config.Indicators.Klines.PrimaryCount

	// 如果没有选择时间周期，使用默认值
	if len(timeframes) == 0 {
		// 兼容旧配置：使用主周期和长周期
		if primaryTimeframe != "" {
			timeframes = append(timeframes, primaryTimeframe)
		} else {
			timeframes = append(timeframes, "3m")
		}
		if req.Config.Indicators.Klines.LongerTimeframe != "" {
			timeframes = append(timeframes, req.Config.Indicators.Klines.LongerTimeframe)
		}
	}
	if primaryTimeframe == "" {
		primaryTimeframe = timeframes[0]
	}
	if klineCount <= 0 {
		klineCount = 30
	}

	fmt.Printf("📊 使用时间周期: %v, 主周期: %s, K线数量: %d\n", timeframes, primaryTimeframe, klineCount)

	// 获取真实市场数据（使用多时间周期）
	marketDataMap := make(map[string]*market.Data)
	for _, coin := range candidates {
		data, err := market.GetWithTimeframes(coin.Symbol, timeframes, primaryTimeframe, klineCount)
		if err != nil {
			// 如果获取某个币种数据失败，记录日志但继续
			fmt.Printf("⚠️  获取 %s 市场数据失败: %v\n", coin.Symbol, err)
			continue
		}
		marketDataMap[coin.Symbol] = data
	}

	// 构建真实的上下文（用于生成 User Prompt）
	testContext := &decision.Context{
		CurrentTime:    time.Now().Format("2006-01-02 15:04:05"),
		RuntimeMinutes: 0,
		CallCount:      1,
		Account: decision.AccountInfo{
			TotalEquity:      1000.0,
			AvailableBalance: 1000.0,
			UnrealizedPnL:    0,
			TotalPnL:         0,
			TotalPnLPct:      0,
			MarginUsed:       0,
			MarginUsedPct:    0,
			PositionCount:    0,
		},
		Positions:      []decision.PositionInfo{},
		CandidateCoins: candidates,
		PromptVariant:  req.PromptVariant,
		MarketDataMap:  marketDataMap,
	}

	// 构建 System Prompt
	systemPrompt := engine.BuildSystemPrompt(1000.0, req.PromptVariant)

	// 构建 User Prompt（使用真实市场数据）
	userPrompt := engine.BuildUserPrompt(testContext)

	// 如果请求真实 AI 调用
	if req.RunRealAI && req.AIModelID != "" {
		aiResponse, aiErr := s.runRealAITest(userID, req.AIModelID, systemPrompt, userPrompt)
		if aiErr != nil {
			c.JSON(http.StatusOK, gin.H{
				"system_prompt":   systemPrompt,
				"user_prompt":     userPrompt,
				"candidate_count": len(candidates),
				"candidates":      candidates,
				"prompt_variant":  req.PromptVariant,
				"ai_response":     fmt.Sprintf("❌ AI 调用失败: %s", aiErr.Error()),
				"ai_error":        aiErr.Error(),
				"note":            "AI 调用出错",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"system_prompt":   systemPrompt,
			"user_prompt":     userPrompt,
			"candidate_count": len(candidates),
			"candidates":      candidates,
			"prompt_variant":  req.PromptVariant,
			"ai_response":     aiResponse,
			"note":            "✅ 真实 AI 测试运行成功",
		})
		return
	}

	// 返回结果（不实际调用 AI，只返回构建的 prompt）
	c.JSON(http.StatusOK, gin.H{
		"system_prompt":   systemPrompt,
		"user_prompt":     userPrompt,
		"candidate_count": len(candidates),
		"candidates":      candidates,
		"prompt_variant":  req.PromptVariant,
		"ai_response":     "请选择 AI 模型并点击「运行测试」来执行真实的 AI 分析。",
		"note":            "未选择 AI 模型或未启用真实 AI 调用",
	})
}

// runRealAITest 执行真实的 AI 测试调用
func (s *Server) runRealAITest(userID, modelID, systemPrompt, userPrompt string) (string, error) {
	// 获取 AI 模型配置
	model, err := s.store.AIModel().Get(userID, modelID)
	if err != nil {
		return "", fmt.Errorf("获取 AI 模型失败: %w", err)
	}

	if !model.Enabled {
		return "", fmt.Errorf("AI 模型 %s 尚未启用", model.Name)
	}

	if model.APIKey == "" {
		return "", fmt.Errorf("AI 模型 %s 缺少 API Key", model.Name)
	}

	// 创建 AI 客户端
	var aiClient mcp.AIClient
	provider := model.Provider

	switch provider {
	case "qwen":
		aiClient = mcp.NewQwenClient()
		aiClient.SetAPIKey(model.APIKey, model.CustomAPIURL, model.CustomModelName)
	case "deepseek":
		aiClient = mcp.NewDeepSeekClient()
		aiClient.SetAPIKey(model.APIKey, model.CustomAPIURL, model.CustomModelName)
	default:
		// 使用通用客户端
		aiClient = mcp.NewClient()
		aiClient.SetAPIKey(model.APIKey, model.CustomAPIURL, model.CustomModelName)
	}

	// 调用 AI API
	response, err := aiClient.CallWithMessages(systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("AI API 调用失败: %w", err)
	}

	return response, nil
}

