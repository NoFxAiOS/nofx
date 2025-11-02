package main

import (
	"fmt"
	"log"
	"nofx/api"
	"nofx/config"
	"nofx/i18n"
	"nofx/manager"
	"nofx/pool"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// t is a shorthand for i18n.T
var t = i18n.T

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║    🏆 AI模型交易竞赛系统 - Qwen vs DeepSeek               ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 加载配置文件
	configFile := "config.json"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	log.Printf(t("loading_config"), configFile)
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		log.Fatalf(t("config_load_failed"), err)
	}

	log.Printf(t("config_loaded"), len(cfg.Traders))
	fmt.Println()

	// 设置默认主流币种列表
	pool.SetDefaultCoins(cfg.DefaultCoins)

	// 设置是否使用默认主流币种
	pool.SetUseDefaultCoins(cfg.UseDefaultCoins)
	if cfg.UseDefaultCoins {
		log.Printf(t("default_coins_enabled"), len(cfg.DefaultCoins), cfg.DefaultCoins)
	}

	// 设置币种池API URL
	if cfg.CoinPoolAPIURL != "" {
		pool.SetCoinPoolAPI(cfg.CoinPoolAPIURL)
		log.Println(t("ai500_configured"))
	}
	if cfg.OITopAPIURL != "" {
		pool.SetOITopAPI(cfg.OITopAPIURL)
		log.Println(t("oi_top_configured"))
	}

	// 创建TraderManager
	traderManager := manager.NewTraderManager()

	// 添加所有启用的trader
	enabledCount := 0
	for i, traderCfg := range cfg.Traders {
		// 跳过未启用的trader
		if !traderCfg.Enabled {
			log.Printf(t("skip_disabled_trader"), i+1, len(cfg.Traders), traderCfg.Name)
			continue
		}

		enabledCount++
		log.Printf(t("initializing_trader"),
			i+1, len(cfg.Traders), traderCfg.Name, strings.ToUpper(traderCfg.AIModel))

		err := traderManager.AddTrader(
			traderCfg,
			cfg.CoinPoolAPIURL,
			cfg.MaxDailyLoss,
			cfg.MaxDrawdown,
			cfg.StopTradingMinutes,
			cfg.Leverage, // 传递杠杆配置
		)
		if err != nil {
			log.Fatalf(t("trader_init_failed"), err)
		}
	}

	// 检查是否至少有一个启用的trader
	if enabledCount == 0 {
		log.Fatalf(t("no_enabled_traders"))
	}

	fmt.Println()
	fmt.Println(t("competition_participants"))
	for _, traderCfg := range cfg.Traders {
		// 只显示启用的trader
		if !traderCfg.Enabled {
			continue
		}
		fmt.Printf(t("initial_capital"),
			traderCfg.Name, strings.ToUpper(traderCfg.AIModel), traderCfg.InitialBalance)
	}

	fmt.Println()
	fmt.Println(t("ai_full_control_mode"))
	fmt.Printf(t("ai_leverage_info")+"\n",
		cfg.Leverage.AltcoinLeverage, cfg.Leverage.BTCETHLeverage)
	fmt.Println(t("ai_position_size"))
	fmt.Println(t("ai_stop_loss"))
	fmt.Println(t("ai_analysis"))
	fmt.Println()
	fmt.Println(t("risk_warning"))
	fmt.Println()
	fmt.Println(t("press_ctrl_c"))
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	// 创建并启动API服务器
	apiServer := api.NewServer(traderManager, cfg.APIServerPort)
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Printf(t("api_server_error"), err)
		}
	}()

	// 设置优雅退出
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 启动所有trader
	traderManager.StartAll()

	// 等待退出信号
	<-sigChan
	fmt.Println()
	fmt.Println()
	log.Println(t("shutdown_signal"))
	traderManager.StopAll()

	fmt.Println()
	fmt.Println(t("thank_you"))
}
