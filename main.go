package main

import (
	"nofx/api"
	"nofx/auth"
	"nofx/backtest"
	"nofx/config"
	"nofx/crypto"
	"nofx/logger"
	"nofx/manager"
	"nofx/market"
	"nofx/mcp"
	"nofx/store"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	// 加载 .env 环境变量
	_ = godotenv.Load()

	// 初始化日志
	logger.Init(nil)

	logger.Info("╔════════════════════════════════════════════════════════════╗")
	logger.Info("║    🤖 AI多模型交易系统 - 支持 DeepSeek & Qwen            ║")
	logger.Info("╚════════════════════════════════════════════════════════════╝")

	// 初始化全局配置（从 .env 加载）
	config.Init()
	cfg := config.Get()
	logger.Info("✅ 配置加载完成")

	// 初始化数据库
	dbPath := "data.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	logger.Infof("📋 初始化数据库: %s", dbPath)
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

	// 设置 JWT 密钥
	auth.SetJWTSecret(cfg.JWTSecret)
	logger.Info("🔑 JWT 密钥已设置")

	// 创建 TraderManager 与 BacktestManager
	traderManager := manager.NewTraderManager()
	mcpClient := newSharedMCPClient()
	backtestManager := backtest.NewManager(mcpClient)
	if err := backtestManager.RestoreRuns(); err != nil {
		logger.Warnf("⚠️ 恢复历史回测失败: %v", err)
	}

	// 从数据库加载所有交易员到内存
	if err := traderManager.LoadTradersFromStore(st); err != nil {
		logger.Fatalf("❌ 加载交易员失败: %v", err)
	}

	// 显示加载的交易员信息
	traders, err := st.Trader().List("default")
	if err != nil {
		logger.Fatalf("❌ 获取交易员列表失败: %v", err)
	}

	logger.Info("🤖 数据库中的AI交易员配置:")
	if len(traders) == 0 {
		logger.Info("  (无交易员配置，请通过Web管理界面创建)")
	} else {
		for _, t := range traders {
			status := "❌ 已停止"
			if t.IsRunning {
				status = "✅ 运行中"
			}
			logger.Infof("  • %s [%s] %s - AI模型: %s, 交易所: %s",
				t.Name, t.ID[:8], status, t.AIModelID, t.ExchangeID)
		}
	}

	// 启动 WebSocket 行情监控（获取所有 USDT 永续合约的行情数据）
	go market.NewWSMonitor(150).Start(nil)
	logger.Info("📊 WebSocket 行情监控已启动")

	// 启动API服务器
	server := api.NewServer(traderManager, st, cryptoService, backtestManager, cfg.APIServerPort)
	go func() {
		if err := server.Start(); err != nil {
			logger.Fatalf("❌ API服务器启动失败: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("✅ 系统启动完成，等待交易指令...")
	logger.Info("📌 提示: 使用 Ctrl+C 停止系统")

	<-quit
	logger.Info("📴 收到停止信号，正在关闭系统...")

	// 停止所有交易员
	traderManager.StopAll()
	logger.Info("✅ 系统已安全关闭")
}

// newSharedMCPClient 创建共享的 MCP AI 客户端（用于回测）
func newSharedMCPClient() mcp.AIClient {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		logger.Warn("⚠️ DEEPSEEK_API_KEY 未设置，AI 功能将不可用")
		return nil
	}
	return mcp.NewDeepSeekClient()
}
