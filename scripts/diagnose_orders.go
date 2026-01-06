package main

import (
	"flag"
	"fmt"
	"log"
	"nofx/store"
	"os"
	"path/filepath"
	"time"
)

func main() {
	var dbPath string
	var traderID string

	flag.StringVar(&dbPath, "db", "./data/data.db", "数据库文件路径")
	flag.StringVar(&traderID, "trader", "", "Trader ID（可选）")
	flag.Parse()

	// 确保数据库文件存在
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		log.Fatalf("❌ 无效的数据库路径: %v", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		log.Fatalf("❌ 数据库文件不存在: %s", absPath)
	}

	fmt.Printf("📂 数据库路径: %s\n", absPath)

	// 打开数据库
	s, err := store.New(absPath)
	if err != nil {
		log.Fatalf("❌ 无法打开数据库: %v", err)
	}
	defer s.Close()

	orderStore := s.Order()

	// 如果指定了 traderID，获取该 trader 的订单
	if traderID == "" {
		fmt.Println("\n⚠️  未指定 trader_id，使用: --trader <trader_id>")
		fmt.Println("   获取所有 trader 的统计信息...\n")
	}

	// 获取订单列表
	orders, err := orderStore.GetTraderOrders(traderID, 100)
	if err != nil {
		log.Fatalf("❌ 获取订单失败: %v", err)
	}

	fmt.Printf("\n📋 找到 %d 条订单记录\n\n", len(orders))

	if len(orders) == 0 {
		fmt.Println("⚠️  没有订单数据！可能的原因：")
		fmt.Println("   1. Trader 还没有执行过交易")
		fmt.Println("   2. CreateOrder 插入失败（重复键冲突）")
		fmt.Println("   3. 指定的 trader_id 不存在")
		return
	}

	// 统计数据
	var (
		totalOrders       = len(orders)
		filledOrders      = 0
		withFilledAt      = 0
		withAvgFillPrice  = 0
		withOrderAction   = 0
		missingFilledAt   = 0
		missingAvgPrice   = 0
		missingOrderAction = 0
	)

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("%-15s %-10s %-10s %-15s %-10s %-15s\n", "订单ID", "状态", "动作", "平均成交价", "成交时间", "问题")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for _, order := range orders {
		issues := []string{}

		if order.Status == "FILLED" {
			filledOrders++

			// 检查 filled_at
			if order.FilledAt > 0 {
				withFilledAt++
			} else {
				missingFilledAt++
				issues = append(issues, "❌ 缺少成交时间")
			}

			// 检查 avg_fill_price
			if order.AvgFillPrice > 0 {
				withAvgFillPrice++
			} else {
				missingAvgPrice++
				issues = append(issues, "❌ 成交价为0")
			}
		}

		// 检查 order_action
		if order.OrderAction != "" {
			withOrderAction++
		} else {
			missingOrderAction++
			issues = append(issues, "⚠️  缺少订单动作")
		}

		issueStr := "✅ 正常"
		if len(issues) > 0 {
			issueStr = ""
			for i, issue := range issues {
				if i > 0 {
					issueStr += ", "
				}
				issueStr += issue
			}
		}

		filledAtStr := "N/A"
		if order.FilledAt > 0 {
			filledAtStr = time.UnixMilli(order.FilledAt).Format("01-02 15:04")
		}

		fmt.Printf("%-15s %-10s %-10s %-15.2f %-10s %s\n",
			order.ExchangeOrderID[:min(15, len(order.ExchangeOrderID))],
			order.Status,
			order.OrderAction,
			order.AvgFillPrice,
			filledAtStr,
			issueStr,
		)
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// 统计摘要
	fmt.Printf("\n📊 统计摘要:\n")
	fmt.Printf("   总订单数:        %d\n", totalOrders)
	fmt.Printf("   已成交订单:      %d\n", filledOrders)
	fmt.Printf("   有成交时间:      %d / %d (%.1f%%)\n", withFilledAt, filledOrders, float64(withFilledAt)/float64(max(filledOrders, 1))*100)
	fmt.Printf("   有成交价格:      %d / %d (%.1f%%)\n", withAvgFillPrice, filledOrders, float64(withAvgFillPrice)/float64(max(filledOrders, 1))*100)
	fmt.Printf("   有订单动作:      %d / %d (%.1f%%)\n", withOrderAction, totalOrders, float64(withOrderAction)/float64(max(totalOrders, 1))*100)

	fmt.Printf("\n⚠️  问题订单:\n")
	if missingFilledAt > 0 {
		fmt.Printf("   ❌ %d 条订单缺少成交时间 (filled_at)\n", missingFilledAt)
	}
	if missingAvgPrice > 0 {
		fmt.Printf("   ❌ %d 条订单成交价为 0 (avg_fill_price)\n", missingAvgPrice)
	}
	if missingOrderAction > 0 {
		fmt.Printf("   ⚠️  %d 条订单缺少订单动作 (order_action)\n", missingOrderAction)
	}

	if missingFilledAt > 0 || missingAvgPrice > 0 {
		fmt.Println("\n💡 这些订单无法在图表上显示，因为：")
		fmt.Println("   - 缺少成交时间 → 前端无法定位到K线时间轴")
		fmt.Println("   - 成交价为 0 → 前端会过滤掉 (line 164: if (!orderPrice || orderPrice === 0) return)")
		fmt.Println("\n🔧 可能的原因：")
		fmt.Println("   1. UpdateOrderStatus 没有被正确调用")
		fmt.Println("   2. GetOrderStatus 返回的数据缺少 avgPrice 字段")
		fmt.Println("   3. Lighter 交易所的订单状态查询有问题")
	}

	if missingFilledAt == 0 && missingAvgPrice == 0 && missingOrderAction == 0 {
		fmt.Println("\n✅ 所有订单数据完整！")
		fmt.Println("   如果图表仍然没有显示 B/S 标记，检查：")
		fmt.Println("   1. 前端是否正确调用了 /api/orders API")
		fmt.Println("   2. 浏览器控制台是否有错误")
		fmt.Println("   3. 订单时间是否在图表的时间范围内")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
