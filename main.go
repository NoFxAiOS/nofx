package main

import (
	"encoding/json"
	"fmt"
	"nofx/api"
	"nofx/auth"
	"nofx/backtest"
	"nofx/config"
	"nofx/crypto"
	"nofx/logger"
	"nofx/manager"
	"nofx/market"
	"nofx/mcp"
	"nofx/pool"
	"nofx/store"
	"nofx/trader"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

// ConfigFile 配置文件结构，只包含需要同步到数据库的字段
// TODO 现在与config.Config相同，未来会被替换， 现在为了兼容性不得不保留当前文件
type ConfigFile struct {
	BetaMode           bool                  `json:"beta_mode"`
	APIServerPort      int                   `json:"api_server_port"`
	UseDefaultCoins    bool                  `json:"use_default_coins"`
	DefaultCoins       []string              `json:"default_coins"`
	CoinPoolAPIURL     string                `json:"coin_pool_api_url"`
	OITopAPIURL        string                `json:"oi_top_api_url"`
	MaxDailyLoss       float64               `json:"max_daily_loss"`
	MaxDrawdown        float64               `json:"max_drawdown"`
	StopTradingMinutes int                   `json:"stop_trading_minutes"`
	Leverage           config.LeverageConfig `json:"leverage"`
	JWTSecret          string                `json:"jwt_secret"`
	DataKLineTime      string                `json:"data_k_line_time"`
	Log                *config.LogConfig     `json:"log"` // 日志配置
}

// loadConfigFile 读取并解析config.json文件
func loadConfigFile() (*ConfigFile, error) {
	// 检查config.json是否存在
	if _, err := os.Stat("config.json"); os.IsNotExist(err) {
		logger.Info("📄 config.json不存在，使用默认配置")
		return &ConfigFile{}, nil
	}

	// 读取config.json
	data, err := os.ReadFile("config.json")
	if err != nil {
		return nil, fmt.Errorf("读取config.json失败: %w", err)
	}

	// 解析JSON
	var configFile ConfigFile
	if err := json.Unmarshal(data, &configFile); err != nil {
		return nil, fmt.Errorf("解析config.json失败: %w", err)
	}

	return &configFile, nil
}

// syncConfigToDatabase 将配置同步到数据库
func syncConfigToDatabase(st *store.Store, configFile *ConfigFile) error {
	if configFile == nil {
		return nil
	}

	logger.Info("🔄 开始同步config.json到数据库...")

	// 同步各配置项到数据库
	configs := map[string]string{
		"beta_mode":            fmt.Sprintf("%t", configFile.BetaMode),
		"api_server_port":      strconv.Itoa(configFile.APIServerPort),
		"use_default_coins":    fmt.Sprintf("%t", configFile.UseDefaultCoins),
		"coin_pool_api_url":    configFile.CoinPoolAPIURL,
		"oi_top_api_url":       configFile.OITopAPIURL,
		"max_daily_loss":       fmt.Sprintf("%.1f", configFile.MaxDailyLoss),
		"max_drawdown":         fmt.Sprintf("%.1f", configFile.MaxDrawdown),
		"stop_trading_minutes": strconv.Itoa(configFile.StopTradingMinutes),
	}

	// 同步default_coins（转换为JSON字符串存储）
	if len(configFile.DefaultCoins) > 0 {
		defaultCoinsJSON, err := json.Marshal(configFile.DefaultCoins)
		if err == nil {
			configs["default_coins"] = string(defaultCoinsJSON)
		}
	}

	// 同步杠杆配置
	if configFile.Leverage.BTCETHLeverage > 0 {
		configs["btc_eth_leverage"] = strconv.Itoa(configFile.Leverage.BTCETHLeverage)
	}
	if configFile.Leverage.AltcoinLeverage > 0 {
		configs["altcoin_leverage"] = strconv.Itoa(configFile.Leverage.AltcoinLeverage)
	}

	// 如果JWT密钥不为空，也同步
	if configFile.JWTSecret != "" {
		configs["jwt_secret"] = configFile.JWTSecret
	}

	// 更新数据库配置
	for key, value := range configs {
		if err := st.SystemConfig().Set(key, value); err != nil {
			logger.Warnf("⚠️  更新配置 %s 失败: %v", key, err)
		} else {
			logger.Infof("✓ 同步配置: %s = %s", key, value)
		}
	}

	logger.Info("✅ config.json同步完成")
	return nil
}

// loadBetaCodesToDatabase 加载内测码文件到数据库
func loadBetaCodesToDatabase(st *store.Store) error {
	betaCodeFile := "beta_codes.txt"

	// 检查内测码文件是否存在
	if _, err := os.Stat(betaCodeFile); os.IsNotExist(err) {
		logger.Infof("📄 内测码文件 %s 不存在，跳过加载", betaCodeFile)
		return nil
	}

	// 获取文件信息
	fileInfo, err := os.Stat(betaCodeFile)
	if err != nil {
		return fmt.Errorf("获取内测码文件信息失败: %w", err)
	}

	logger.Infof("🔄 发现内测码文件 %s (%.1f KB)，开始加载...", betaCodeFile, float64(fileInfo.Size())/1024)

	// 加载内测码到数据库
	err = st.BetaCode().LoadFromFile(betaCodeFile)
	if err != nil {
		return fmt.Errorf("加载内测码失败: %w", err)
	}

	// 显示统计信息
	total, used, err := st.BetaCode().GetStats()
	if err != nil {
		logger.Warnf("⚠️  获取内测码统计失败: %v", err)
	} else {
		logger.Infof("✅ 内测码加载完成: 总计 %d 个，已使用 %d 个，剩余 %d 个", total, used, total-used)
	}

	return nil
}

func main() {
	// Load environment variables from .env file if present (for local/dev runs)
	// In Docker Compose, variables are injected by the runtime and this is harmless.
	_ = godotenv.Load()

	// 初始化日志
	logger.Init(nil)

	logger.Info("╔════════════════════════════════════════════════════════════╗")
	logger.Info("║    🤖 AI多模型交易系统 - 支持 DeepSeek & Qwen            ║")
	logger.Info("╚════════════════════════════════════════════════════════════╝")

	// 初始化数据库配置
	dbPath := "data.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	// 读取配置文件
	configFile, err := loadConfigFile()
	if err != nil {
		logger.Fatalf("❌ 读取config.json失败: %v", err)
	}

	logger.Infof("📋 初始化配置数据库: %s", dbPath)
	st, err := store.New(dbPath)
	if err != nil {
		logger.Fatalf("❌ 初始化数据库失败: %v", err)
	}
	defer st.Close()
	backtest.UseDatabase(st.DB())

	// 初始化加密服务
	logger.Info("🔐 初始化加密服务...")
	cryptoService, err := crypto.NewCryptoService()
	if err != nil {
		logger.Fatalf("❌ 初始化加密服务失败: %v", err)
	}
	// 创建加密/解密包装函数
	encryptFunc := func(plaintext string) string {
		if plaintext == "" {
			return plaintext
		}
		encrypted, err := cryptoService.EncryptForStorage(plaintext)
		if err != nil {
			logger.Warnf("⚠️ 加密失败: %v", err)
			return plaintext
		}
		return encrypted
	}
	decryptFunc := func(encrypted string) string {
		if encrypted == "" {
			return encrypted
		}
		if !cryptoService.IsEncryptedStorageValue(encrypted) {
			return encrypted
		}
		decrypted, err := cryptoService.DecryptFromStorage(encrypted)
		if err != nil {
			logger.Warnf("⚠️ 解密失败: %v", err)
			return encrypted
		}
		return decrypted
	}
	st.SetCryptoFuncs(encryptFunc, decryptFunc)
	logger.Info("✅ 加密服务初始化成功")

	// 同步config.json到数据库
	if err := syncConfigToDatabase(st, configFile); err != nil {
		logger.Warnf("⚠️  同步config.json到数据库失败: %v", err)
	}

	// 加载内测码到数据库
	if err := loadBetaCodesToDatabase(st); err != nil {
		logger.Warnf("⚠️  加载内测码到数据库失败: %v", err)
	}

	// 获取系统配置
	useDefaultCoinsStr, _ := st.SystemConfig().Get("use_default_coins")
	useDefaultCoins := useDefaultCoinsStr == "true"
	apiPortStr, _ := st.SystemConfig().Get("api_server_port")

	// 设置JWT密钥（优先使用环境变量）
	jwtSecret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if jwtSecret == "" {
		// 回退到数据库配置
		jwtSecret, _ = st.SystemConfig().Get("jwt_secret")
		if jwtSecret == "" {
			jwtSecret = "your-jwt-secret-key-change-in-production-make-it-long-and-random"
			logger.Warn("⚠️  使用默认JWT密钥，建议使用加密设置脚本生成安全密钥")
		} else {
			logger.Info("🔑 使用数据库中JWT密钥")
		}
	} else {
		logger.Info("🔑 使用环境变量JWT密钥")
	}
	auth.SetJWTSecret(jwtSecret)

	// 管理员模式下需要管理员密码，缺失则退出

	logger.Info("✓ 配置数据库初始化成功")

	// 从数据库读取默认主流币种列表
	defaultCoinsJSON, _ := st.SystemConfig().Get("default_coins")
	var defaultCoins []string

	if defaultCoinsJSON != "" {
		// 尝试从JSON解析
		if err := json.Unmarshal([]byte(defaultCoinsJSON), &defaultCoins); err != nil {
			logger.Warnf("⚠️  解析default_coins配置失败: %v，使用硬编码默认值", err)
			defaultCoins = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"}
		} else {
			logger.Infof("✓ 从数据库加载默认币种列表（共%d个）: %v", len(defaultCoins), defaultCoins)
		}
	} else {
		// 如果数据库中没有配置，使用硬编码默认值
		defaultCoins = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"}
		logger.Warn("⚠️  数据库中未配置default_coins，使用硬编码默认值")
	}

	pool.SetDefaultCoins(defaultCoins)
	// 设置是否使用默认主流币种
	pool.SetUseDefaultCoins(useDefaultCoins)
	if useDefaultCoins {
		logger.Info("✓ 已启用默认主流币种列表")
	}

	// 设置币种池API URL
	coinPoolAPIURL, _ := st.SystemConfig().Get("coin_pool_api_url")
	if coinPoolAPIURL != "" {
		pool.SetCoinPoolAPI(coinPoolAPIURL)
		logger.Info("✓ 已配置AI500币种池API")
	}

	oiTopAPIURL, _ := st.SystemConfig().Get("oi_top_api_url")
	if oiTopAPIURL != "" {
		pool.SetOITopAPI(oiTopAPIURL)
		logger.Info("✓ 已配置OI Top API")
	}

	// 创建TraderManager 与 BacktestManager
	cfgForAI, cfgErr := config.LoadConfig("config.json")
	if cfgErr != nil {
		logger.Warnf("⚠️  加载config.json用于AI客户端失败: %v", cfgErr)
	}

	traderManager := manager.NewTraderManager()
	mcpClient := newSharedMCPClient(cfgForAI)
	backtestManager := backtest.NewManager(mcpClient)
	if err := backtestManager.RestoreRuns(); err != nil {
		logger.Warnf("⚠️  恢复历史回测失败: %v", err)
	}

	// 从数据库加载所有交易员到内存
	err = traderManager.LoadTradersFromStore(st)
	if err != nil {
		logger.Fatalf("❌ 加载交易员失败: %v", err)
	}

	// 获取数据库中的所有交易员配置（用于显示，使用default用户）
	traders, err := st.Trader().List("default")
	if err != nil {
		logger.Fatalf("❌ 获取交易员列表失败: %v", err)
	}

	// 显示加载的交易员信息
	logger.Info("🤖 数据库中的AI交易员配置:")
	if len(traders) == 0 {
		logger.Info("  • 暂无配置的交易员，请通过Web界面创建")
	} else {
		for _, trader := range traders {
			status := "停止"
			if trader.IsRunning {
				status = "运行中"
			}
			logger.Infof("  • %s (%s + %s) - 初始资金: %.0f USDT [%s]",
				trader.Name, strings.ToUpper(trader.AIModelID), strings.ToUpper(trader.ExchangeID),
				trader.InitialBalance, status)
		}
	}

	logger.Info("🤖 AI全权决策模式:")
	logger.Info("  • AI将自主决定每笔交易的杠杆倍数（山寨币最高5倍，BTC/ETH最高5倍）")
	logger.Info("  • AI将自主决定每笔交易的仓位大小")
	logger.Info("  • AI将自主设置止损和止盈价格")
	logger.Info("  • AI将基于市场数据、技术指标、账户状态做出全面分析")
	logger.Warn("⚠️  风险提示: AI自动交易有风险，建议小额资金测试！")
	logger.Info("按 Ctrl+C 停止运行")
	logger.Info(strings.Repeat("=", 60))

	// 自动恢复之前运行中的交易员
	traderManager.AutoStartRunningTraders(st)

	// 获取API服务器端口（优先级：环境变量 > 数据库配置 > 默认值）
	apiPort := 8080 // 默认端口

	// 1. 优先从环境变量 NOFX_BACKEND_PORT 读取
	if envPort := strings.TrimSpace(os.Getenv("NOFX_BACKEND_PORT")); envPort != "" {
		if port, err := strconv.Atoi(envPort); err == nil && port > 0 {
			apiPort = port
			logger.Infof("🔌 使用环境变量端口: %d (NOFX_BACKEND_PORT)", apiPort)
		} else {
			logger.Warnf("⚠️  环境变量 NOFX_BACKEND_PORT 无效: %s", envPort)
		}
	} else if apiPortStr != "" {
		// 2. 从数据库配置读取（config.json 同步过来的）
		if port, err := strconv.Atoi(apiPortStr); err == nil && port > 0 {
			apiPort = port
			logger.Infof("🔌 使用数据库配置端口: %d (api_server_port)", apiPort)
		}
	} else {
		logger.Infof("🔌 使用默认端口: %d", apiPort)
	}

	// 启动订单同步管理器
	orderSyncManager := trader.NewOrderSyncManager(st, 10*time.Second)
	orderSyncManager.Start()

	// 启动仓位同步管理器（检测手动平仓等变化）
	positionSyncManager := trader.NewPositionSyncManager(st, 10*time.Second)
	positionSyncManager.Start()

	// 创建并启动API服务器
	apiServer := api.NewServer(traderManager, st, cryptoService, backtestManager, apiPort)
	go func() {
		if err := apiServer.Start(); err != nil {
			logger.Errorf("❌ API服务器错误: %v", err)
		}
	}()

	// 启动流行情数据 - 默认使用所有交易员设置的币种 如果没有设置币种 则优先使用系统默认
	go market.NewWSMonitor(150).Start(st.Trader().GetCustomCoins())
	//go market.NewWSMonitor(150).Start([]string{}) //这里是一个使用方式 传入空的话 则使用market市场的所有币种
	// 设置优雅退出
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// TODO: 启动数据库中配置为运行状态的交易员
	// traderManager.StartAll()

	// 等待退出信号
	<-sigChan
	logger.Info("📛 收到退出信号，正在优雅关闭...")

	// 步骤 1: 停止所有交易员
	logger.Info("⏸️  停止所有交易员...")
	traderManager.StopAll()
	logger.Info("✅ 所有交易员已停止")

	// 步骤 2: 停止订单同步管理器和仓位同步管理器
	logger.Info("📦 停止订单同步管理器...")
	orderSyncManager.Stop()
	logger.Info("📊 停止仓位同步管理器...")
	positionSyncManager.Stop()

	// 步骤 3: 关闭 API 服务器
	logger.Info("🛑 停止 API 服务器...")
	if err := apiServer.Shutdown(); err != nil {
		logger.Warnf("⚠️  关闭 API 服务器时出错: %v", err)
	} else {
		logger.Info("✅ API 服务器已安全关闭")
	}

	// 步骤 4: 关闭数据库连接 (确保所有写入完成)
	logger.Info("💾 关闭数据库连接...")
	if err := st.Close(); err != nil {
		logger.Errorf("❌ 关闭数据库失败: %v", err)
	} else {
		logger.Info("✅ 数据库已安全关闭，所有数据已持久化")
	}

	logger.Info("👋 感谢使用AI交易系统！")
}

func newSharedMCPClient(cfg *config.Config) mcp.AIClient {
	return mcp.NewClient()
}
