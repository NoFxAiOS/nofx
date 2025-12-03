package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	// 从环境变量获取数据库连接
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL环境变量未设置")
	}

	fmt.Println("🔄 连接数据库...")
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()

	// 测试连接
	if err := db.Ping(); err != nil {
		log.Fatalf("数据库连接测试失败: %v", err)
	}
	fmt.Println("✅ 数据库连接成功!")

	// 目标用户和积分
	userID := "68003b68-2f1d-4618-8124-e93e4a86200a"
	targetCredits := 100000
	adminID := "script_admin"
	reason := "通过脚本更新用户积分"

	fmt.Printf("\n📋 任务详情:\n")
	fmt.Printf("   用户ID: %s\n", userID)
	fmt.Printf("   目标积分: %d\n", targetCredits)
	fmt.Printf("   操作者: %s\n", adminID)

	// 开始事务
	fmt.Println("\n🔄 开始更新积分...")
	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("开始事务失败: %v", err)
	}
	defer tx.Rollback()

	// 查询或创建用户积分账户
	var availableCredits, totalCredits, usedCredits int
	var userCreditsID string
	var createdAt, updatedAt time.Time

	err = tx.QueryRow(`
		SELECT id, available_credits, total_credits, used_credits, created_at, updated_at
		FROM user_credits
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(&userCreditsID, &availableCredits, &totalCredits, &usedCredits, &createdAt, &updatedAt)

	var isNewAccount bool
	if err != nil {
		if err == sql.ErrNoRows {
			// 用户没有积分账户，创建新的
			isNewAccount = true
			availableCredits = 0
			totalCredits = 0
			usedCredits = 0
			createdAt = time.Now()
			updatedAt = time.Now()
			fmt.Println("ℹ️  用户积分账户不存在，将创建新账户")
		} else {
			log.Fatalf("查询用户积分记录失败: %v", err)
		}
	}

	// 计算调整量
	adjustment := targetCredits - availableCredits
	fmt.Printf("   当前积分: %d\n", availableCredits)
	fmt.Printf("   需要调整: %+d\n", adjustment)

	// 计算新的积分值
	var newAvailableCredits, newTotalCredits, newUsedCredits int
	var txnType, category string

	if adjustment > 0 {
		// 增加积分
		newAvailableCredits = availableCredits + adjustment
		newTotalCredits = totalCredits + adjustment
		newUsedCredits = usedCredits
		txnType = "credit"
		category = "admin"
	} else if adjustment < 0 {
		// 扣减积分，检查余额
		if availableCredits < -adjustment {
			log.Fatalf("积分不足: 当前可用积分 %d，需要扣减 %d", availableCredits, -adjustment)
		}
		newAvailableCredits = availableCredits + adjustment
		newTotalCredits = totalCredits
		newUsedCredits = usedCredits - adjustment // 实际使用的积分增加
		txnType = "debit"
		category = "admin"
	} else {
		fmt.Println("✅ 用户积分已经是目标值，无需调整")
		return
	}

	description := fmt.Sprintf("管理员 %s %s 用户 %s 积分: %s (原因: %s)",
		adminID, map[string]string{"credit": "增加", "debit": "扣减"}[txnType], userID, reason)

	// 更新或创建积分账户
	if isNewAccount {
		_, err = tx.Exec(`
			INSERT INTO user_credits
			(id, user_id, available_credits, total_credits, used_credits, created_at, updated_at)
			VALUES (gen_random_uuid()::text, $1, $2, $3, $4, $5, $6)
		`, userID, newAvailableCredits, newTotalCredits, newUsedCredits, createdAt, updatedAt)
		if err != nil {
			log.Fatalf("创建用户积分账户失败: %v", err)
		}
		fmt.Println("✅ 创建新积分账户成功")
	} else {
		_, err = tx.Exec(`
			UPDATE user_credits
			SET available_credits = $1, total_credits = $2, used_credits = $3, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = $4
		`, newAvailableCredits, newTotalCredits, newUsedCredits, userID)
		if err != nil {
			log.Fatalf("更新用户积分失败: %v", err)
		}
		fmt.Println("✅ 更新积分账户成功")
	}

	// 记录积分流水
	_, err = tx.Exec(`
		INSERT INTO credit_transactions
		(id, user_id, type, amount, balance_before, balance_after, category, description, reference_id, created_at)
		VALUES (gen_random_uuid()::text, $1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP)
	`, userID, txnType, adjustment, availableCredits, newAvailableCredits,
		category, description, adminID)
	if err != nil {
		log.Fatalf("记录积分流水失败: %v", err)
	}
	fmt.Println("✅ 记录积分流水成功")

	// 提交事务
	if err := tx.Commit(); err != nil {
		log.Fatalf("提交事务失败: %v", err)
	}

	fmt.Println("\n✅ 积分更新完成!")
	fmt.Printf("   调整: %+d (之前: %d, 之后: %d)\n", adjustment, availableCredits, newAvailableCredits)

	// 验证更新结果
	fmt.Println("\n🔍 验证更新结果...")
	var verifyCredits int
	err = db.QueryRow(`SELECT available_credits FROM user_credits WHERE user_id = $1`, userID).Scan(&verifyCredits)
	if err != nil {
		log.Fatalf("验证失败: %v", err)
	}

	if verifyCredits == targetCredits {
		fmt.Printf("✅ 验证成功! 用户当前积分为: %d\n", verifyCredits)
	} else {
		log.Fatalf("验证失败! 期望 %d，实际 %d", targetCredits, verifyCredits)
	}

	// 显示最新流水
	fmt.Println("\n📊 最新积分流水:")
	rows, err := db.Query(`
		SELECT created_at, type, amount, balance_before, balance_after, category, description
		FROM credit_transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 5
	`, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var createdAt time.Time
			var txnType, category, description string
			var amount, balanceBefore, balanceAfter int
			rows.Scan(&createdAt, &txnType, &amount, &balanceBefore, &balanceAfter, &category, &description)
			fmt.Printf("   [%s] %s %+d (余额: %d → %d)\n",
				createdAt.Format("15:04:05"), txnType, amount, balanceBefore, balanceAfter)
		}
	}

	fmt.Println("\n🎉 所有操作完成!")
}
