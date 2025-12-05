package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"nofx/crypto"

	_ "modernc.org/sqlite"
)

func main() {
	log.Println("🔄 开始迁移数据库到加密格式...")

	// 1. 检查数据库文件
	dbPath := "data.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Fatalf("❌ 数据库文件不存在: %s", dbPath)
	}

	// 2. 备份数据库
	backupPath := fmt.Sprintf("%s.pre_encryption_backup", dbPath)
	log.Printf("📦 备份数据库到: %s", backupPath)

	input, err := os.ReadFile(dbPath)
	if err != nil {
		log.Fatalf("❌ 读取数据库失败: %v", err)
	}

	if err := os.WriteFile(backupPath, input, 0600); err != nil {
		log.Fatalf("❌ 备份失败: %v", err)
	}

	// 3. 打开数据库
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("❌ 打开数据库失败: %v", err)
	}
	defer db.Close()

	// 4. 初始化 CryptoService（从环境变量加载密钥）
	cs, err := crypto.NewCryptoService()
	if err != nil {
		log.Fatalf("❌ 初始化加密服务失败: %v", err)
	}

	// 5. 迁移交易所配置
	if err := migrateExchanges(db, cs); err != nil {
		log.Fatalf("❌ 迁移交易所配置失败: %v", err)
	}

	// 6. 迁移 AI 模型配置
	if err := migrateAIModels(db, cs); err != nil {
		log.Fatalf("❌ 迁移 AI 模型配置失败: %v", err)
	}

	log.Println("✅ 数据迁移完成！")
	log.Printf("📝 原始数据备份位于: %s", backupPath)
	log.Println("⚠️  请验证系统功能正常后，手动删除备份文件")
}

// migrateExchanges 迁移交易所配置
func migrateExchanges(db *sql.DB, cs *crypto.CryptoService) error {
	log.Println("🔄 迁移交易所配置...")

	// 查询所有未加密的记录（加密数据以 ENC:v1: 开头）
	rows, err := db.Query(`
		SELECT user_id, id, api_key, secret_key,
		       COALESCE(hyperliquid_private_key, ''),
		       COALESCE(aster_private_key, '')
		FROM exchanges
		WHERE (api_key != '' AND api_key NOT LIKE 'ENC:v1:%')
		   OR (secret_key != '' AND secret_key NOT LIKE 'ENC:v1:%')
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	count := 0
	for rows.Next() {
		var userID, exchangeID, apiKey, secretKey, hlPrivateKey, asterPrivateKey string
		if err := rows.Scan(&userID, &exchangeID, &apiKey, &secretKey, &hlPrivateKey, &asterPrivateKey); err != nil {
			return err
		}

		// 加密每个字段
		encAPIKey, err := cs.EncryptForStorage(apiKey)
		if err != nil {
			return fmt.Errorf("加密 API Key 失败: %w", err)
		}

		encSecretKey, err := cs.EncryptForStorage(secretKey)
		if err != nil {
			return fmt.Errorf("加密 Secret Key 失败: %w", err)
		}

		encHLPrivateKey := ""
		if hlPrivateKey != "" {
			encHLPrivateKey, err = cs.EncryptForStorage(hlPrivateKey)
			if err != nil {
				return fmt.Errorf("加密 Hyperliquid Private Key 失败: %w", err)
			}
		}

		encAsterPrivateKey := ""
		if asterPrivateKey != "" {
			encAsterPrivateKey, err = cs.EncryptForStorage(asterPrivateKey)
			if err != nil {
				return fmt.Errorf("加密 Aster Private Key 失败: %w", err)
			}
		}

		// 更新数据库
		_, err = tx.Exec(`
			UPDATE exchanges
			SET api_key = ?, secret_key = ?,
			    hyperliquid_private_key = ?, aster_private_key = ?
			WHERE user_id = ? AND id = ?
		`, encAPIKey, encSecretKey, encHLPrivateKey, encAsterPrivateKey, userID, exchangeID)

		if err != nil {
			return fmt.Errorf("更新数据库失败: %w", err)
		}

		log.Printf("  ✓ 已加密: [%s] %s", userID, exchangeID)
		count++
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("✅ 已迁移 %d 个交易所配置", count)
	return nil
}

// migrateAIModels 迁移 AI 模型配置
func migrateAIModels(db *sql.DB, cs *crypto.CryptoService) error {
	log.Println("🔄 迁移 AI 模型配置...")

	rows, err := db.Query(`
		SELECT user_id, id, api_key
		FROM ai_models
		WHERE api_key != '' AND api_key NOT LIKE 'ENC:v1:%'
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	count := 0
	for rows.Next() {
		var userID, modelID, apiKey string
		if err := rows.Scan(&userID, &modelID, &apiKey); err != nil {
			return err
		}

		encAPIKey, err := cs.EncryptForStorage(apiKey)
		if err != nil {
			return fmt.Errorf("加密 API Key 失败: %w", err)
		}

		_, err = tx.Exec(`
			UPDATE ai_models SET api_key = ? WHERE user_id = ? AND id = ?
		`, encAPIKey, userID, modelID)

		if err != nil {
			return fmt.Errorf("更新数据库失败: %w", err)
		}

		log.Printf("  ✓ 已加密: [%s] %s", userID, modelID)
		count++
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("✅ 已迁移 %d 个 AI 模型配置", count)
	return nil
}
