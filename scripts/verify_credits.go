package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL环境变量未设置")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("数据库连接测试失败: %v", err)
	}

	userID := "68003b68-2f1d-4618-8124-e93e4a86200a"

	fmt.Println("=== 用户积分验证查询 ===\n")

	// 1. 查询用户积分
	fmt.Println("1. 用户积分信息:")
	var available, total, used int
	err = db.QueryRow(`
		SELECT available_credits, total_credits, used_credits
		FROM user_credits WHERE user_id = $1
	`, userID).Scan(&available, &total, &used)
	if err != nil {
		log.Fatalf("查询用户积分失败: %v", err)
	}
	fmt.Printf("   用户ID: %s\n", userID)
	fmt.Printf("   可用积分: %d\n", available)
	fmt.Printf("   总积分: %d\n", total)
	fmt.Printf("   已使用: %d\n", used)

	// 2. 查询积分流水
	fmt.Println("\n2. 积分流水记录:")
	rows, err := db.Query(`
		SELECT id, type, amount, balance_before, balance_after, category, description, created_at
		FROM credit_transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		log.Fatalf("查询积分流水失败: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, txnType, category, description string
		var amount, balanceBefore, balanceAfter int
		var createdAt sql.NullTime
		rows.Scan(&id, &txnType, &amount, &balanceBefore, &balanceAfter, &category, &description, &createdAt)
		fmt.Printf("   [%s] %s %+d (余额: %d → %d)\n", createdAt.Time.Format("2006-01-02 15:04:05"), txnType, amount, balanceBefore, balanceAfter)
		fmt.Printf("      描述: %s\n", description)
	}

	// 3. 查询审计日志
	fmt.Println("\n3. 审计日志:")
	auditRows, err := db.Query(`
		SELECT id, action, success, details, created_at
		FROM audit_logs
		WHERE user_id = $1 OR details LIKE '%' || $1 || '%'
		ORDER BY created_at DESC
		LIMIT 5
	`, userID)
	if err != nil {
		log.Printf("查询审计日志失败: %v", err)
	} else {
		defer auditRows.Close()
		for auditRows.Next() {
			var id, action, details string
			var success bool
			var createdAt sql.NullTime
			auditRows.Scan(&id, &action, &success, &details, &createdAt)
			fmt.Printf("   [%s] %s\n", createdAt.Time.Format("2006-01-02 15:04:05"), action)
			fmt.Printf("      %s\n", details)
		}
	}

	fmt.Println("\n=== 验证完成 ===")
}
