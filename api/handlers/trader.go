package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"nofx/config"
	"nofx/manager"
)

// getTraderFromQuery 从query参数获取trader
func (h *BaseHandler) getTraderFromQuery(c *gin.Context) (*manager.TraderManager, string, error) {
	userID := c.GetString("user_id")
	traderID := c.Query("trader_id")

	// 确保用户的交易员已加载到内存中
	err := h.TraderManager.LoadUserTraders(h.Database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, err)
	}

	if traderID == "" {
		// 如果没有指定trader_id，返回该用户的第一个trader
		ids := h.TraderManager.GetTraderIDs()
		if len(ids) == 0 {
			return nil, "", fmt.Errorf("没有可用的trader")
		}

		// 获取用户的交易员列表，优先返回用户自己的交易员
		userTraders, err := h.Database.GetTraders(userID)
		if err == nil && len(userTraders) > 0 {
			traderID = userTraders[0].ID
		} else {
			traderID = ids[0]
		}
	}

	return h.TraderManager, traderID, nil
}

// CreateTraderRequest 创建交易员请求
type CreateTraderRequest struct {
	Name                 string  `json:"name" binding:"required"`
	AIModelID            string  `json:"ai_model_id" binding:"required"`
	ExchangeID           string  `json:"exchange_id" binding:"required"`
	InitialBalance       float64 `json:"initial_balance"`
	ScanIntervalMinutes  int     `json:"scan_interval_minutes"`
	BTCETHLeverage       int     `json:"btc_eth_leverage"`
	AltcoinLeverage      int     `json:"altcoin_leverage"`
	TradingSymbols       string  `json:"trading_symbols"`
	CustomPrompt         string  `json:"custom_prompt"`
	OverrideBasePrompt   bool    `json:"override_base_prompt"`
	SystemPromptTemplate string  `json:"system_prompt_template"` // 系统提示词模板名称
	IsCrossMargin        *bool   `json:"is_cross_margin"`        // 指针类型，nil表示使用默认值true
	UseCoinPool          bool    `json:"use_coin_pool"`
	UseOITop             bool    `json:"use_oi_top"`
}

// HandleCreateTrader 创建新的AI交易员
func (h *BaseHandler) HandleCreateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	var req CreateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 校验杠杆值
	if req.BTCETHLeverage < 0 || req.BTCETHLeverage > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "BTC/ETH杠杆必须在1-50倍之间"})
		return
	}
	if req.AltcoinLeverage < 0 || req.AltcoinLeverage > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "山寨币杠杆必须在1-20倍之间"})
		return
	}

	// 校验交易币种格式
	if req.TradingSymbols != "" {
		symbols := strings.Split(req.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" && !strings.HasSuffix(strings.ToUpper(symbol), "USDT") {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的币种格式: %s，必须以USDT结尾", symbol)})
				return
			}
		}
	}

	// 生成交易员ID
	traderID := fmt.Sprintf("%s_%s_%d", req.ExchangeID, req.AIModelID, time.Now().Unix())

	// 设置默认值
	isCrossMargin := true // 默认为全仓模式
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	// 设置杠杆默认值（从系统配置获取）
	btcEthLeverage := 5
	altcoinLeverage := 5
	if req.BTCETHLeverage > 0 {
		btcEthLeverage = req.BTCETHLeverage
	} else {
		// 从系统配置获取默认值
		if btcEthLeverageStr, _ := h.Database.GetSystemConfig("btc_eth_leverage"); btcEthLeverageStr != "" {
			if val, err := strconv.Atoi(btcEthLeverageStr); err == nil && val > 0 {
				btcEthLeverage = val
			}
		}
	}
	if req.AltcoinLeverage > 0 {
		altcoinLeverage = req.AltcoinLeverage
	} else {
		// 从系统配置获取默认值
		if altcoinLeverageStr, _ := h.Database.GetSystemConfig("altcoin_leverage"); altcoinLeverageStr != "" {
			if val, err := strconv.Atoi(altcoinLeverageStr); err == nil && val > 0 {
				altcoinLeverage = val
			}
		}
	}

	// 设置系统提示词模板默认值
	systemPromptTemplate := "default"
	if req.SystemPromptTemplate != "" {
		systemPromptTemplate = req.SystemPromptTemplate
	}

	// 设置扫描间隔默认值
	scanIntervalMinutes := req.ScanIntervalMinutes

	// P0修复: TopTrader特殊处理 - 强制设为30分钟
	if req.Name == "TopTrader" || strings.Contains(req.Name, "TopTrader") {
		if scanIntervalMinutes != 30 {
			log.Printf("⚠️ [P0] TopTrader 扫描间隔被设为 %d 分钟，调整为标准 30 分钟", scanIntervalMinutes)
			scanIntervalMinutes = 30
		}
	} else if scanIntervalMinutes <= 0 {
		scanIntervalMinutes = 3 // 其他Trader的默认值3分钟
	}

	log.Printf("✅ [P0] %s 扫描间隔设置为: %d 分钟", req.Name, scanIntervalMinutes)

	// 创建交易员配置（数据库实体）
	trader := &config.TraderRecord{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		InitialBalance:       req.InitialBalance,
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       req.TradingSymbols,
		UseCoinPool:          req.UseCoinPool,
		UseOITop:             req.UseOITop,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: systemPromptTemplate,
		IsCrossMargin:        isCrossMargin,
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            false,
	}

	// 保存到数据库
	err := h.Database.CreateTrader(trader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建交易员失败: %v", err)})
		return
	}

	// 立即将新交易员加载到TraderManager中
	err = h.TraderManager.LoadUserTraders(h.Database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户交易员到内存失败: %v", err)
		// 这里不返回错误，因为交易员已经成功创建到数据库
	}

	// 🔧 修复：验证trader确实被加载到内存
	_, err = h.TraderManager.GetTrader(traderID)
	if err != nil {
		// Trader创建成功但加载失败 - 这通常意味着AI模型或交易所配置缺失
		log.Printf("❌ 交易员已创建但加载失败: %s - %v", traderID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("交易员已创建但加载到内存失败。请检查AI模型 '%s' 和交易所 '%s' 的配置是否存在且已启用。详细错误: %v",
				req.AIModelID, req.ExchangeID, err),
		})
		return
	}

	log.Printf("✓ 创建交易员成功: %s (模型: %s, 交易所: %s)", req.Name, req.AIModelID, req.ExchangeID)

	c.JSON(http.StatusCreated, gin.H{
		"trader_id":   traderID,
		"trader_name": req.Name,
		"ai_model":    req.AIModelID,
		"is_running":  false,
	})
}

// UpdateTraderRequest 更新交易员请求
type UpdateTraderRequest struct {
	Name                string  `json:"name" binding:"required"`
	AIModelID           string  `json:"ai_model_id" binding:"required"`
	ExchangeID          string  `json:"exchange_id" binding:"required"`
	InitialBalance      float64 `json:"initial_balance"`
	ScanIntervalMinutes int     `json:"scan_interval_minutes"`
	BTCETHLeverage      int     `json:"btc_eth_leverage"`
	AltcoinLeverage     int     `json:"altcoin_leverage"`
	TradingSymbols      string  `json:"trading_symbols"`
	CustomPrompt        string  `json:"custom_prompt"`
	OverrideBasePrompt  bool    `json:"override_base_prompt"`
	IsCrossMargin       *bool   `json:"is_cross_margin"`
}

// HandleUpdateTrader 更新交易员配置
func (h *BaseHandler) HandleUpdateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	var req UpdateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查交易员是否存在且属于当前用户
	traders, err := h.Database.GetTraders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取交易员列表失败"})
		return
	}

	var existingTrader *config.TraderRecord
	for _, trader := range traders {
		if trader.ID == traderID {
			existingTrader = trader
			break
		}
	}

	if existingTrader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 设置默认值
	isCrossMargin := existingTrader.IsCrossMargin // 保持原值
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	// 设置杠杆默认值
	btcEthLeverage := req.BTCETHLeverage
	altcoinLeverage := req.AltcoinLeverage
	if btcEthLeverage <= 0 {
		btcEthLeverage = existingTrader.BTCETHLeverage // 保持原值
	}
	if altcoinLeverage <= 0 {
		altcoinLeverage = existingTrader.AltcoinLeverage // 保持原值
	}

	// 设置扫描间隔，允许更新
	scanIntervalMinutes := req.ScanIntervalMinutes
	if scanIntervalMinutes <= 0 {
		scanIntervalMinutes = existingTrader.ScanIntervalMinutes // 保持原值
	}

	// P0修复: TopTrader特殊处理 - 强制设为30分钟
	if existingTrader.Name == "TopTrader" || strings.Contains(existingTrader.Name, "TopTrader") {
		if scanIntervalMinutes != 30 {
			log.Printf("⚠️ [P0] TopTrader 扫描间隔被设为 %d 分钟，调整为标准 30 分钟", scanIntervalMinutes)
			scanIntervalMinutes = 30
		}
	}

	log.Printf("✅ [P0] %s 扫描间隔设置为: %d 分钟", existingTrader.Name, scanIntervalMinutes)

	// 更新交易员配置
	trader := &config.TraderRecord{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		InitialBalance:       req.InitialBalance,
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       req.TradingSymbols,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: existingTrader.SystemPromptTemplate, // 保持原值
		IsCrossMargin:        isCrossMargin,
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            existingTrader.IsRunning, // 保持原值
	}

	// 更新数据库
	err = h.Database.UpdateTrader(trader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新交易员失败: %v", err)})
		return
	}

	// 重新加载交易员到内存
	err = h.TraderManager.LoadUserTraders(h.Database, userID)
	if err != nil {
		log.Printf("⚠️ 重新加载用户交易员到内存失败: %v", err)
	}

	log.Printf("✓ 更新交易员成功: %s (模型: %s, 交易所: %s)", req.Name, req.AIModelID, req.ExchangeID)

	c.JSON(http.StatusOK, gin.H{
		"trader_id":   traderID,
		"trader_name": req.Name,
		"ai_model":    req.AIModelID,
		"message":     "交易员更新成功",
	})
}

// HandleDeleteTrader 删除交易员
func (h *BaseHandler) HandleDeleteTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 从数据库删除
	err := h.Database.DeleteTrader(userID, traderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("删除交易员失败: %v", err)})
		return
	}

	// 如果交易员正在运行，先停止它
	if trader, err := h.TraderManager.GetTrader(traderID); err == nil {
		status := trader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); ok && isRunning {
			trader.Stop()
			log.Printf("⏹  已停止运行中的交易员: %s", traderID)
		}
	}

	log.Printf("✓ 交易员已删除: %s", traderID)
	c.JSON(http.StatusOK, gin.H{"message": "交易员已删除"})
}

// HandleStartTrader 启动交易员
func (h *BaseHandler) HandleStartTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 校验交易员是否属于当前用户 - 使用统一的查询逻辑
	traders, err := h.Database.GetTraders(userID)
	if err != nil {
		log.Printf("❌ 获取用户 %s 的交易员列表失败: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取交易员列表失败"})
		return
	}

	// 检查指定trader是否存在于用户列表中
	var userTrader *config.TraderRecord
	for _, trader := range traders {
		if trader.ID == traderID {
			userTrader = trader
			break
		}
	}

	if userTrader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在或无访问权限"})
		return
	}

	// 尝试从traderManager获取trader实例
	trader, err := h.TraderManager.GetTrader(traderID)
	if err != nil {
		// 如果trader不在内存中，尝试从数据库加载该用户的trader
		log.Printf("🔄 Trader %s 不在内存中，尝试加载...", traderID)
		if loadErr := h.TraderManager.LoadUserTraders(h.Database, userID); loadErr != nil {
			log.Printf("❌ 加载trader失败: %v", loadErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "加载trader失败"})
			return
		}

		// 再次尝试获取
		trader, err = h.TraderManager.GetTrader(traderID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
			return
		}
		log.Printf("✅ Trader %s 已加载到内存", traderID)
	}

	// 检查交易员是否已经在运行
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员已在运行中"})
		return
	}

	// 启动交易员
	go func() {
		log.Printf("▶️  启动交易员 %s (%s)", traderID, trader.GetName())
		if err := trader.Run(); err != nil {
			log.Printf("❌ 交易员 %s 运行错误: %v", trader.GetName(), err)
		}
	}()

	// 更新数据库中的运行状态
	err = h.Database.UpdateTraderStatus(traderID, true)
	if err != nil {
		log.Printf("⚠️  更新交易员状态失败: %v", err)
	}

	log.Printf("✓ 交易员 %s 已启动", trader.GetName())
	c.JSON(http.StatusOK, gin.H{"message": "交易员已启动"})
}

// HandleStopTrader 停止交易员
func (h *BaseHandler) HandleStopTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 校验交易员是否属于当前用户 - 使用统一的查询逻辑
	traders, err := h.Database.GetTraders(userID)
	if err != nil {
		log.Printf("❌ 获取用户 %s 的交易员列表失败: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取交易员列表失败"})
		return
	}

	// 检查指定trader是否存在于用户列表中
	var userTrader *config.TraderRecord
	for _, trader := range traders {
		if trader.ID == traderID {
			userTrader = trader
			break
		}
	}

	if userTrader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在或无访问权限"})
		return
	}

	// 尝试从traderManager获取trader实例
	trader, err := h.TraderManager.GetTrader(traderID)
	if err != nil {
		// 如果trader不在内存中，尝试从数据库加载该用户的trader
		log.Printf("🔄 Trader %s 不在内存中，尝试加载...", traderID)
		if loadErr := h.TraderManager.LoadUserTraders(h.Database, userID); loadErr != nil {
			log.Printf("❌ 加载trader失败: %v", loadErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "加载trader失败"})
			return
		}

		// 再次尝试获取
		trader, err = h.TraderManager.GetTrader(traderID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
			return
		}
		log.Printf("✅ Trader %s 已加载到内存", traderID)
	}

	// 检查交易员是否正在运行
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && !isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员已停止"})
		return
	}

	// 停止交易员
	trader.Stop()

	// 更新数据库中的运行状态
	err = h.Database.UpdateTraderStatus(traderID, false)
	if err != nil {
		log.Printf("⚠️  更新交易员状态失败: %v", err)
	}

	log.Printf("⏹  交易员 %s 已停止", trader.GetName())
	c.JSON(http.StatusOK, gin.H{"message": "交易员已停止"})
}

// HandleUpdateTraderPrompt 更新交易员自定义Prompt
func (h *BaseHandler) HandleUpdateTraderPrompt(c *gin.Context) {
	traderID := c.Param("id")
	userID := c.GetString("user_id")

	var req struct {
		CustomPrompt       string `json:"custom_prompt"`
		OverrideBasePrompt bool   `json:"override_base_prompt"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新数据库
	err := h.Database.UpdateTraderCustomPrompt(userID, traderID, req.CustomPrompt, req.OverrideBasePrompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新自定义prompt失败: %v", err)})
		return
	}

	// 如果trader在内存中，更新其custom prompt和override设置
	trader, err := h.TraderManager.GetTrader(traderID)
	if err == nil {
		trader.SetCustomPrompt(req.CustomPrompt)
		trader.SetOverrideBasePrompt(req.OverrideBasePrompt)
		log.Printf("✓ 已更新交易员 %s 的自定义prompt (覆盖基础=%v)", trader.GetName(), req.OverrideBasePrompt)
	}

	c.JSON(http.StatusOK, gin.H{"message": "自定义prompt已更新"})
}

// HandleTraderList trader列表
func (h *BaseHandler) HandleTraderList(c *gin.Context) {
	userID := c.GetString("user_id")
	traders, err := h.Database.GetTraders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取交易员列表失败: %v", err)})
		return
	}

	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		// 获取实时运行状态
		isRunning := trader.IsRunning
		if at, err := h.TraderManager.GetTrader(trader.ID); err == nil {
			status := at.GetStatus()
			if running, ok := status["is_running"].(bool); ok {
				isRunning = running
			}
		}

		// AIModelID 应该已经是 provider（如 "deepseek"），直接使用
		// 如果是旧数据格式（如 "admin_deepseek"），提取 provider 部分
		aiModelID := trader.AIModelID
		// 兼容旧数据：如果包含下划线，提取最后一部分作为 provider
		if strings.Contains(aiModelID, "_") {
			parts := strings.Split(aiModelID, "_")
			aiModelID = parts[len(parts)-1]
		}

		result = append(result, map[string]interface{}{
			"trader_id":       trader.ID,
			"trader_name":     trader.Name,
			"ai_model":        aiModelID,
			"exchange_id":     trader.ExchangeID,
			"is_running":      isRunning,
			"initial_balance": trader.InitialBalance,
		})
	}

	c.JSON(http.StatusOK, result)
}

// HandleGetTraderConfig 获取交易员详细配置
func (h *BaseHandler) HandleGetTraderConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员ID不能为空"})
		return
	}

	traderConfig, _, _, err := h.Database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("获取交易员配置失败: %v", err)})
		return
	}

	// 获取实时运行状态
	isRunning := traderConfig.IsRunning
	if at, err := h.TraderManager.GetTrader(traderID); err == nil {
		status := at.GetStatus()
		if running, ok := status["is_running"].(bool); ok {
			isRunning = running
		}
	}

	// 返回完整的模型ID，不做转换，保持与前端模型列表一致
	aiModelID := traderConfig.AIModelID

	result := map[string]interface{}{
		"trader_id":             traderConfig.ID,
		"trader_name":           traderConfig.Name,
		"ai_model":              aiModelID,
		"exchange_id":           traderConfig.ExchangeID,
		"initial_balance":       traderConfig.InitialBalance,
		"scan_interval_minutes": traderConfig.ScanIntervalMinutes,
		"btc_eth_leverage":      traderConfig.BTCETHLeverage,
		"altcoin_leverage":      traderConfig.AltcoinLeverage,
		"trading_symbols":       traderConfig.TradingSymbols,
		"custom_prompt":         traderConfig.CustomPrompt,
		"override_base_prompt":  traderConfig.OverrideBasePrompt,
		"is_cross_margin":       traderConfig.IsCrossMargin,
		"use_coin_pool":         traderConfig.UseCoinPool,
		"use_oi_top":            traderConfig.UseOITop,
		"is_running":            isRunning,
	}

	c.JSON(http.StatusOK, result)
}

// HandleStatus 系统状态
func (h *BaseHandler) HandleStatus(c *gin.Context) {
	_, traderID, err := h.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := h.TraderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	status := trader.GetStatus()
	c.JSON(http.StatusOK, status)
}

// HandleAccount 账户信息
func (h *BaseHandler) HandleAccount(c *gin.Context) {
	_, traderID, err := h.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := h.TraderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("📊 收到账户信息请求 [%s]", trader.GetName())
	account, err := trader.GetAccountInfo()
	if err != nil {
		log.Printf("❌ 获取账户信息失败 [%s]: %v", trader.GetName(), err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取账户信息失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, account)
}

// HandlePositions 持仓列表
func (h *BaseHandler) HandlePositions(c *gin.Context) {
	_, traderID, err := h.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := h.TraderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	positions, err := trader.GetPositions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取持仓列表失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, positions)
}

// HandleDecisions 决策日志列表
func (h *BaseHandler) HandleDecisions(c *gin.Context) {
	_, traderID, err := h.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := h.TraderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 获取所有历史决策记录（无限制）
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取决策日志失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, records)
}

// HandleLatestDecisions 最新决策日志（最近5条，最新的在前）
func (h *BaseHandler) HandleLatestDecisions(c *gin.Context) {
	_, traderID, err := h.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := h.TraderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	records, err := trader.GetDecisionLogger().GetLatestRecords(5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取决策日志失败: %v", err),
		})
		return
	}

	// 反转数组，让最新的在前面（用于列表显示）
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	c.JSON(http.StatusOK, records)
}

// HandleStatistics 统计信息
func (h *BaseHandler) HandleStatistics(c *gin.Context) {
	_, traderID, err := h.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := h.TraderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	stats, err := trader.GetDecisionLogger().GetStatistics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取统计信息失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// HandlePerformance AI历史表现分析（用于展示AI学习和反思）
func (h *BaseHandler) HandlePerformance(c *gin.Context) {
	_, traderID, err := h.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := h.TraderManager.GetTrader(traderID)

	// 🔧 修复：如果trader不在内存中，尝试重新加载（可能刚创建）
	if err != nil {
		log.Printf("⏳ Trader在内存中未找到 %s，尝试重新加载...", traderID)
		userID := c.GetString("user_id")

		// 尝试重新加载用户的traders
		err = h.TraderManager.LoadUserTraders(h.Database, userID)
		if err != nil {
			log.Printf("⚠️ 重新加载traders失败: %v", err)
			// 继续尝试获取
		}

		// 再试一次
		trader, err = h.TraderManager.GetTrader(traderID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": fmt.Sprintf("交易员不存在或配置缺失: %v。请检查AI模型和交易所的配置。", err),
			})
			return
		}
		log.Printf("✓ Trader重新加载成功: %s", traderID)
	}

	// 分析最近100个周期的交易表现（避免长期持仓的交易记录丢失）
	performance, err := trader.GetDecisionLogger().AnalyzePerformance(100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("分析历史表现失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, performance)
}
