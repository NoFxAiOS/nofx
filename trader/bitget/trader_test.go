package bitget

import (
	"fmt"
	"os"
	"testing"
)

// 测试 Bitget 下单（带止盈止损）
// 运行: go test -v -run TestOpenLongWithTPSL ./trader/bitget/
func TestOpenLongWithTPSL(t *testing.T) {
	apiKey := os.Getenv("BITGET_API_KEY")
	secretKey := os.Getenv("BITGET_SECRET_KEY")
	passphrase := os.Getenv("BITGET_PASSPHRASE")

	if apiKey == "" || secretKey == "" || passphrase == "" {
		t.Skip("跳过测试: 需要设置环境变量 BITGET_API_KEY, BITGET_SECRET_KEY, BITGET_PASSPHRASE")
	}

	trader := NewBitgetTrader(apiKey, secretKey, passphrase)

	// 测试参数 - 使用最小仓位
	symbol := "BTCUSDT"
	quantity := 0.001  // 最小数量
	leverage := 5
	stopLoss := 70000.0   // 止损价格（根据当前价格调整）
	takeProfit := 110000.0 // 止盈价格

	fmt.Printf("📊 测试 Bitget OpenLongWithTPSL\n")
	fmt.Printf("   Symbol: %s\n", symbol)
	fmt.Printf("   Quantity: %.4f\n", quantity)
	fmt.Printf("   Leverage: %dx\n", leverage)
	fmt.Printf("   StopLoss: %.2f\n", stopLoss)
	fmt.Printf("   TakeProfit: %.2f\n", takeProfit)

	// 先获取当前价格
	price, err := trader.GetMarketPrice(symbol)
	if err != nil {
		t.Fatalf("获取价格失败: %v", err)
	}
	fmt.Printf("   当前价格: %.2f\n", price)

	// 调整止盈止损
	stopLoss = price * 0.95    // 5% 止损
	takeProfit = price * 1.05  // 5% 止盈
	fmt.Printf("   调整后 StopLoss: %.2f (%.1f%%)\n", stopLoss, (1-stopLoss/price)*100)
	fmt.Printf("   调整后 TakeProfit: %.2f (+%.1f%%)\n", takeProfit, (takeProfit/price-1)*100)

	// 执行下单
	fmt.Println("\n🚀 开始下单...")
	result, err := trader.OpenLongWithTPSL(symbol, quantity, leverage, stopLoss, takeProfit)
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}

	fmt.Printf("✅ 下单成功!\n")
	fmt.Printf("   OrderId: %v\n", result["orderId"])
	fmt.Printf("   Symbol: %v\n", result["symbol"])
	fmt.Printf("   Status: %v\n", result["status"])

	// 查看持仓确认
	fmt.Println("\n📋 查看持仓...")
	positions, err := trader.GetPositions()
	if err != nil {
		t.Logf("⚠️ 获取持仓失败: %v", err)
	} else {
		for _, pos := range positions {
			if pos["symbol"] == symbol || pos["symbol"] == "BTCUSDT" {
				fmt.Printf("   持仓: %v\n", pos)
			}
		}
	}
}

// 测试获取账户余额
func TestGetBalance(t *testing.T) {
	apiKey := os.Getenv("BITGET_API_KEY")
	secretKey := os.Getenv("BITGET_SECRET_KEY")
	passphrase := os.Getenv("BITGET_PASSPHRASE")

	if apiKey == "" || secretKey == "" || passphrase == "" {
		t.Skip("跳过测试: 需要设置环境变量")
	}

	trader := NewBitgetTrader(apiKey, secretKey, passphrase)

	balance, err := trader.GetBalance()
	if err != nil {
		t.Fatalf("获取余额失败: %v", err)
	}

	fmt.Printf("💰 账户余额:\n")
	for k, v := range balance {
		fmt.Printf("   %s: %v\n", k, v)
	}
}

// 测试获取持仓
func TestGetPositions(t *testing.T) {
	apiKey := os.Getenv("BITGET_API_KEY")
	secretKey := os.Getenv("BITGET_SECRET_KEY")
	passphrase := os.Getenv("BITGET_PASSPHRASE")

	if apiKey == "" || secretKey == "" || passphrase == "" {
		t.Skip("跳过测试: 需要设置环境变量")
	}

	trader := NewBitgetTrader(apiKey, secretKey, passphrase)

	positions, err := trader.GetPositions()
	if err != nil {
		t.Fatalf("获取持仓失败: %v", err)
	}

	fmt.Printf("📋 当前持仓 (%d):\n", len(positions))
	for i, pos := range positions {
		fmt.Printf("   [%d] %v\n", i+1, pos)
	}
}
