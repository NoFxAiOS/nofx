package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"nofx/store"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var dbPath string
	var force bool

	flag.StringVar(&dbPath, "db", "./data/data.db", "数据库文件路径")
	flag.BoolVar(&force, "force", false, "跳过确认直接删除")
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

	db := s.DB()

	// 统计当前数据
	var orderCount, fillCount int
	db.QueryRow(`SELECT COUNT(*) FROM trader_orders`).Scan(&orderCount)
	db.QueryRow(`SELECT COUNT(*) FROM trader_fills`).Scan(&fillCount)

	fmt.Printf("\n📊 当前数据统计:\n")
	fmt.Printf("   trader_orders: %d 条记录\n", orderCount)
	fmt.Printf("   trader_fills:  %d 条记录\n", fillCount)

	if orderCount == 0 && fillCount == 0 {
		fmt.Println("\n✅ 表已经是空的，无需清空")
		return
	}

	// 确认删除
	if !force {
		fmt.Println("\n⚠️  警告: 此操作将删除所有订单和成交记录，无法恢复！")
		fmt.Print("\n确认删除？请输入 'yes' 继续: ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input != "yes" {
			fmt.Println("\n❌ 操作已取消")
			return
		}
	}

	fmt.Println("\n🗑️  开始清空表...")

	// 清空 trader_fills 表（先删除，因为有外键约束）
	result, err := db.Exec(`DELETE FROM trader_fills`)
	if err != nil {
		log.Fatalf("❌ 清空 trader_fills 失败: %v", err)
	}
	fillsDeleted, _ := result.RowsAffected()
	fmt.Printf("   ✅ 删除了 %d 条成交记录\n", fillsDeleted)

	// 清空 trader_orders 表
	result, err = db.Exec(`DELETE FROM trader_orders`)
	if err != nil {
		log.Fatalf("❌ 清空 trader_orders 失败: %v", err)
	}
	ordersDeleted, _ := result.RowsAffected()
	fmt.Printf("   ✅ 删除了 %d 条订单记录\n", ordersDeleted)

	// 重置自增ID（可选，让ID从1重新开始）
	_, err = db.Exec(`DELETE FROM sqlite_sequence WHERE name IN ('trader_orders', 'trader_fills')`)
	if err == nil {
		fmt.Println("   ✅ 重置了自增ID计数器")
	}

	// 验证清空结果
	db.QueryRow(`SELECT COUNT(*) FROM trader_orders`).Scan(&orderCount)
	db.QueryRow(`SELECT COUNT(*) FROM trader_fills`).Scan(&fillCount)

	fmt.Printf("\n🔍 验证结果:\n")
	fmt.Printf("   trader_orders: %d 条记录\n", orderCount)
	fmt.Printf("   trader_fills:  %d 条记录\n", fillCount)

	if orderCount == 0 && fillCount == 0 {
		fmt.Println("\n✅ 表已成功清空！")
		fmt.Println("\n💡 现在可以重新运行 trader 进行测试")
		fmt.Println("   新的订单将从 ID=1 开始记录")
	} else {
		fmt.Println("\n⚠️  清空未完成，请检查数据库")
	}
}
