package main

import (
	"encoding/json"
	"fmt"
	"log"
	"nofx/api"
	"nofx/auth"
	"nofx/config"
	"nofx/manager"
	"nofx/pool"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// LeverageConfig 杠杆配置
type LeverageConfig struct {
	BTCETHLeverage  int `json:"btc_eth_leverage"`
	AltcoinLeverage int `json:"altcoin_leverage"`
}

// EconomicCalendarConfig 经济日历配置
type EconomicCalendarConfig struct {
	Enabled               bool   `json:"enabled"`                  // 是否启用经济日历功能
	DBPath                string `json:"db_path"`                  // 数据库文件路径
	ScriptPath            string `json:"script_path"`              // Python脚本路径
	UpdateIntervalSeconds int    `json:"update_interval_seconds"`  // 数据更新间隔(秒)
	HoursAhead            int    `json:"hours_ahead"`              // 查询未来多少小时的事件
	MinImportance         string `json:"min_importance"`           // 最低重要性("高"/"中"/"低")
}

// startEconomicCalendarService 启动经济日历数据采集服务
func startEconomicCalendarService(cfg *EconomicCalendarConfig) *exec.Cmd {
	if cfg == nil || !cfg.Enabled {
		log.Printf("⏭️  经济日历功能未启用")
		return nil
	}

	// 从配置中获取路径
	scriptPath := cfg.ScriptPath
	if scriptPath == "" {
		scriptPath = "world/经济日历/economic_calendar_minimal.py"
	}

	// 获取脚本目录
	calendarDir := filepath.Dir(scriptPath)
	calendarScript := filepath.Base(scriptPath)
	calendarPath := filepath.Join(calendarDir, calendarScript)

	// 检查脚本是否存在
	if _, err := os.Stat(calendarPath); os.IsNotExist(err) {
		log.Printf("⚠️  经济日历脚本不存在: %s (跳过自动启动)", calendarPath)
		return nil
	}

	// 检查Python是否可用
	pythonCmd := "python3"
	if _, err := exec.LookPath(pythonCmd); err != nil {
		pythonCmd = "python" // 尝试python命令
		if _, err := exec.LookPath(pythonCmd); err != nil {
			log.Printf("⚠️  未找到Python环境 (跳过经济日历服务)")
			return nil
		}
	}

	// 启动Python服务
	log.Printf("🚀 启动经济日历数据采集服务...")
	intervalStr := strconv.Itoa(cfg.UpdateIntervalSeconds)
	if intervalStr == "0" {
		intervalStr = "300" // 默认5分钟
	}
	cmd := exec.Command(pythonCmd, calendarScript, "--interval", intervalStr)
	cmd.Dir = calendarDir

	// 设置输出到日志文件
	logFile, err := os.Create(filepath.Join(calendarDir, "calendar.log"))
	if err != nil {
		log.Printf("⚠️  创建日志文件失败: %v", err)
		return nil
	}

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// 启动服务
	if err := cmd.Start(); err != nil {
		log.Printf("⚠️  启动经济日历服务失败: %v", err)
		logFile.Close()
		return nil
	}

	log.Printf("✓ 经济日历服务已启动 (PID: %d, 日志: %s/calendar.log)", cmd.Process.Pid, calendarDir)
	return cmd
}

// ConfigFile 配置文件结构，只包含需要同步到数据库的字段
type ConfigFile struct {
	AdminMode          bool           `json:"admin_mode"`
	APIServerPort      int            `json:"api_server_port"`
	UseDefaultCoins    bool           `json:"use_default_coins"`
	DefaultCoins       []string       `json:"default_coins"`
	CoinPoolAPIURL     string         `json:"coin_pool_api_url"`
	OITopAPIURL        string         `json:"oi_top_api_url"`
	MaxDailyLoss       float64        `json:"max_daily_loss"`
	MaxDrawdown        float64        `json:"max_drawdown"`
	StopTradingMinutes int            `json:"stop_trading_minutes"`
	Leverage           LeverageConfig `json:"leverage"`
	JWTSecret          string         `json:"jwt_secret"`
}

// syncConfigToDatabase 从config.json读取配置并同步到数据库
func syncConfigToDatabase(database *config.Database) error {
	// 检查config.json是否存在
	if _, err := os.Stat("config.json"); os.IsNotExist(err) {
		log.Printf("📄 config.json不存在，跳过同步")
		return nil
	}

	// 读取config.json
	data, err := os.ReadFile("config.json")
	if err != nil {
		return fmt.Errorf("读取config.json失败: %w", err)
	}

	// 解析JSON
	var configFile ConfigFile
	if err := json.Unmarshal(data, &configFile); err != nil {
		return fmt.Errorf("解析config.json失败: %w", err)
	}

	log.Printf("🔄 开始同步config.json到数据库...")

	// 同步各配置项到数据库
	configs := map[string]string{
		"admin_mode":            fmt.Sprintf("%t", configFile.AdminMode),
		"api_server_port":       strconv.Itoa(configFile.APIServerPort),
		"use_default_coins":     fmt.Sprintf("%t", configFile.UseDefaultCoins),
		"coin_pool_api_url":     configFile.CoinPoolAPIURL,
		"oi_top_api_url":        configFile.OITopAPIURL,
		"max_daily_loss":        fmt.Sprintf("%.1f", configFile.MaxDailyLoss),
		"max_drawdown":          fmt.Sprintf("%.1f", configFile.MaxDrawdown),
		"stop_trading_minutes":  strconv.Itoa(configFile.StopTradingMinutes),
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
		if err := database.SetSystemConfig(key, value); err != nil {
			log.Printf("⚠️  更新配置 %s 失败: %v", key, err)
		} else {
			log.Printf("✓ 同步配置: %s = %s", key, value)
		}
	}

	log.Printf("✅ config.json同步完成")
	return nil
}

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║    🤖 AI多模型交易系统 - 支持 DeepSeek & Qwen            ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 初始化数据库配置
	dbPath := "config.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	log.Printf("📋 初始化配置数据库: %s", dbPath)
	database, err := config.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("❌ 初始化数据库失败: %v", err)
	}
	defer database.Close()

	// 同步config.json到数据库
	if err := syncConfigToDatabase(database); err != nil {
		log.Printf("⚠️  同步config.json到数据库失败: %v", err)
	}

	// 获取系统配置
	useDefaultCoinsStr, _ := database.GetSystemConfig("use_default_coins")
	useDefaultCoins := useDefaultCoinsStr == "true"
	apiPortStr, _ := database.GetSystemConfig("api_server_port")
	
	// 获取管理员模式配置
	adminModeStr, _ := database.GetSystemConfig("admin_mode")
	adminMode := adminModeStr != "false" // 默认为true
	
	// 设置JWT密钥
	jwtSecret, _ := database.GetSystemConfig("jwt_secret")
	if jwtSecret == "" {
		jwtSecret = "your-jwt-secret-key-change-in-production-make-it-long-and-random"
		log.Printf("⚠️  使用默认JWT密钥，建议在生产环境中配置")
	}
	auth.SetJWTSecret(jwtSecret)
	
	// 在管理员模式下，确保admin用户存在
	if adminMode {
		err := database.EnsureAdminUser()
		if err != nil {
			log.Printf("⚠️  创建admin用户失败: %v", err)
		} else {
			log.Printf("✓ 管理员模式已启用，无需登录")
		}
		auth.SetAdminMode(true)
	}
	
	log.Printf("✓ 配置数据库初始化成功")
	fmt.Println()

	// 从数据库读取默认主流币种列表
	defaultCoinsJSON, _ := database.GetSystemConfig("default_coins")
	var defaultCoins []string

	if defaultCoinsJSON != "" {
		// 尝试从JSON解析
		if err := json.Unmarshal([]byte(defaultCoinsJSON), &defaultCoins); err != nil {
			log.Printf("⚠️  解析default_coins配置失败: %v，使用硬编码默认值", err)
			defaultCoins = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"}
		} else {
			log.Printf("✓ 从数据库加载默认币种列表（共%d个）: %v", len(defaultCoins), defaultCoins)
		}
	} else {
		// 如果数据库中没有配置，使用硬编码默认值
		defaultCoins = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"}
		log.Printf("⚠️  数据库中未配置default_coins，使用硬编码默认值")
	}

	pool.SetDefaultCoins(defaultCoins)

	// 设置是否使用默认主流币种
	pool.SetUseDefaultCoins(useDefaultCoins)
	if useDefaultCoins {
		log.Printf("✓ 已启用默认主流币种列表")
	}

	// 启动经济日历服务(新增)
	// 从config.json读取经济日历配置
	var economicCalendarCfg *EconomicCalendarConfig
	rawCfg, err := os.ReadFile("config.json")
	if err == nil {
		var tmpCfg struct {
			EconomicCalendar *EconomicCalendarConfig `json:"economic_calendar"`
		}
		if err := json.Unmarshal(rawCfg, &tmpCfg); err == nil {
			economicCalendarCfg = tmpCfg.EconomicCalendar
		}
	}

	calendarCmd := startEconomicCalendarService(economicCalendarCfg)
	if calendarCmd != nil {
		// 确保退出时停止经济日历服务
		defer func() {
			if calendarCmd.Process != nil {
				log.Printf("🛑 停止经济日历服务 (PID: %d)...", calendarCmd.Process.Pid)
				calendarCmd.Process.Kill()
			}
		}()
	}

	// 设置币种池API URL
	coinPoolAPIURL, _ := database.GetSystemConfig("coin_pool_api_url")
	if coinPoolAPIURL != "" {
		pool.SetCoinPoolAPI(coinPoolAPIURL)
		log.Printf("✓ 已配置AI500币种池API")
	}
	
	oiTopAPIURL, _ := database.GetSystemConfig("oi_top_api_url")
	if oiTopAPIURL != "" {
		pool.SetOITopAPI(oiTopAPIURL)
		log.Printf("✓ 已配置OI Top API")
	}

	// 创建TraderManager
	traderManager := manager.NewTraderManager()

	// 从数据库加载所有交易员到内存
	err = traderManager.LoadTradersFromDatabase(database)
	if err != nil {
		log.Fatalf("❌ 加载交易员失败: %v", err)
	}

	// 获取所有用户的交易员配置（用于显示）
	userIDs, err := database.GetAllUsers()
	if err != nil {
		log.Printf("⚠️ 获取用户列表失败: %v", err)
		userIDs = []string{"default"} // 回退到default用户
	}

	var allTraders []*config.TraderRecord
	for _, userID := range userIDs {
		traders, err := database.GetTraders(userID)
		if err != nil {
			log.Printf("⚠️ 获取用户 %s 的交易员失败: %v", userID, err)
			continue
		}
		allTraders = append(allTraders, traders...)
	}

	// 显示加载的交易员信息
	fmt.Println()
	fmt.Println("🤖 数据库中的AI交易员配置:")
	if len(allTraders) == 0 {
		fmt.Println("  • 暂无配置的交易员，请通过Web界面创建")
	} else {
		for _, trader := range allTraders {
			status := "停止"
			if trader.IsRunning {
				status = "运行中"
			}
			fmt.Printf("  • %s (%s + %s) - 用户: %s - 初始资金: %.0f USDT [%s]\n",
				trader.Name, strings.ToUpper(trader.AIModelID), strings.ToUpper(trader.ExchangeID), 
				trader.UserID, trader.InitialBalance, status)
		}
	}

	fmt.Println()
	fmt.Println("🤖 AI全权决策模式:")
	fmt.Printf("  • AI将自主决定每笔交易的杠杆倍数（山寨币最高5倍，BTC/ETH最高5倍）\n")
	fmt.Println("  • AI将自主决定每笔交易的仓位大小")
	fmt.Println("  • AI将自主设置止损和止盈价格")
	fmt.Println("  • AI将基于市场数据、技术指标、账户状态做出全面分析")
	fmt.Println()
	fmt.Println("⚠️  风险提示: AI自动交易有风险，建议小额资金测试！")
	fmt.Println()
	fmt.Println("按 Ctrl+C 停止运行")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	// 获取API服务器端口
    apiPort := 8080 // 默认端口
	if apiPortStr != "" {
		if port, err := strconv.Atoi(apiPortStr); err == nil {
			apiPort = port
		}
	}

	// 创建并启动API服务器
	apiServer := api.NewServer(traderManager, database, apiPort)
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Printf("❌ API服务器错误: %v", err)
		}
	}()

	// 设置优雅退出
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// TODO: 启动数据库中配置为运行状态的交易员
	// traderManager.StartAll()

	// 等待退出信号
	<-sigChan
	fmt.Println()
	fmt.Println()
	log.Println("📛 收到退出信号，正在停止所有trader...")
	traderManager.StopAll()

	fmt.Println()
	fmt.Println("👋 感谢使用AI交易系统！")
}
