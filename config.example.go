package main

import (
	"nofx/trader"
	"time"
)

// 配置示例 - 复制到 main.go 中使用

func exampleConfig() trader.AutoTraderConfig {
	return trader.AutoTraderConfig{
		// ========== API密钥配置 ==========
		// 在币安官网申请: https://www.binance.com/zh-CN/my/settings/api-management
		BinanceAPIKey:    "YOUR_BINANCE_API_KEY",    // 必填
		BinanceSecretKey: "YOUR_BINANCE_SECRET_KEY", // 必填

		// 币种池API（可选，不填则使用默认池）
		CoinPoolAPIURL: "", // 留空，将从config.json读取

		// ========== AI配置 ==========
		// 选择一个AI服务商（二选一）
		UseQwen: true, // true=使用阿里云Qwen, false=使用DeepSeek

		// DeepSeek配置
		// 申请地址: https://platform.deepseek.com/
		DeepSeekKey: "sk-your-deepseek-api-key",

		// 阿里云Qwen配置
		// 申请地址: https://dashscope.aliyun.com/
		QwenKey: "sk-your-qwen-api-key",

		// ========== 交易周期 ==========
		// AI决策频率（建议3-5分钟）
		ScanInterval: 3 * time.Minute,

		// ========== 风险控制 ==========
		// 注意：这些仅作为提示，AI可以自主决定实际参数
		MaxDailyLoss:    5.0,              // 最大日亏损5%（触发后暂停）
		MaxDrawdown:     10.0,             // 最大回撤10%（触发后暂停）
		StopTradingTime: 30 * time.Minute, // 触发风控后暂停30分钟
	}
}

// ========== AI决策原则（内置在系统中） ==========
//
// 以下参数由AI根据市场情况自主决定，无需配置：
//
// 1. 杠杆倍数: 1-20倍
//    - AI会根据波动率和信心度选择
//    - 高波动 → 低杠杆
//    - 低波动 → 高杠杆
//
// 2. 仓位大小: USD金额
//    - AI会根据账户净值和风险评估决定
//    - 建议单笔风险2-5%
//
// 3. 止损止盈价格:
//    - AI会根据技术指标动态设置
//    - 风险回报比建议 ≥ 1:2
//
// 4. 开仓时机:
//    - 趋势明确、技术指标一致
//    - RSI、MACD、EMA多重确认
//
// 5. 平仓时机:
//    - 到达止损/止盈
//    - 趋势反转信号
//    - 大额亏损保护
//
// ========== 快速开始 ==========
//
// 1. 配置API密钥（main.go中）
// 2. 选择AI服务商（DeepSeek或Qwen）
// 3. 编译: go build -o nofx-auto
// 4. 运行: ./nofx-auto
// 5. 观察AI的思维链分析和决策
//
// ========== 风险提示 ==========
//
// ⚠️  建议先用小额资金（100-1000 USDT）测试
// ⚠️  密切监控系统运行，特别是初期
// ⚠️  设置币安API的IP白名单
// ⚠️  定期检查持仓和盈亏
// ⚠️  加密货币交易有风险，投资需谨慎
