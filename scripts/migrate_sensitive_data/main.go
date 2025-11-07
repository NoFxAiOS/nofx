package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"nofx/crypto"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	privateKeyPath := flag.String("key", "keys/rsa_private.key", "RSA 私钥路径")
	dryRun := flag.Bool("dry-run", false, "仅检查需要迁移的数据，不写入数据库")
	flag.Parse()

	// 尝试加载 .env 文件（从项目根目录运行时）
	envPaths := []string{
		".env",          // 项目根目录
	}
	envLoaded := false
	for _, envPath := range envPaths {
		if err := loadEnvFile(envPath); err == nil {
			log.Printf("成功加载 .env 文件: %s", envPath)
			envLoaded = true
			break
		}
	}
	if !envLoaded {
		log.Printf("警告: 未找到 .env 文件，请确保在项目根目录存在 .env 文件")
		log.Printf("尝试的路径: %v", envPaths)
	}

	// 确保环境变量已设置
	if os.Getenv("DATA_ENCRYPTION_KEY") == "" {
		log.Fatalf("迁移失败: DATA_ENCRYPTION_KEY 环境变量未设置")
	}

	if err := run(*privateKeyPath, *dryRun); err != nil {
		log.Fatalf("迁移失败: %v", err)
	}
}

func run(privateKeyPath string, dryRun bool) error {
	log.SetFlags(0)
	
	// 尝试多个可能的私钥路径（从项目根目录运行时）
	keyPaths := []string{
		privateKeyPath,        // 用户指定的路径
		"keys/rsa_private.key", // 项目根目录的 keys 文件夹
	}
	
	var finalKeyPath string
	for _, path := range keyPaths {
		if _, err := os.Stat(path); err == nil {
			finalKeyPath = path
			log.Printf("找到私钥文件: %s", path)
			break
		}
	}
	
	if finalKeyPath == "" {
		finalKeyPath = privateKeyPath // 使用默认路径，让 crypto 服务生成新密钥
		log.Printf("警告: 私钥文件不存在，将使用路径: %s, 系统将尝试生成新密钥", finalKeyPath)
	}

	cryptoService, err := crypto.NewCryptoService(finalKeyPath)
	if err != nil {
		return fmt.Errorf("初始化加密服务失败: %w", err)
	}

	db, err := openPostgres()
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	defer db.Close()

	log.Printf("开始迁移 AI 模型密钥 (dry-run=%v)", dryRun)
	if err := migrateAIModels(db, cryptoService, dryRun); err != nil {
		return fmt.Errorf("迁移 AI 模型失败: %w", err)
	}

	log.Printf("开始迁移交易所密钥 (dry-run=%v)", dryRun)
	if err := migrateExchanges(db, cryptoService, dryRun); err != nil {
		return fmt.Errorf("迁移交易所失败: %w", err)
	}

	log.Printf("✓ 敏感数据迁移完成")
	return nil
}

func openPostgres() (*sql.DB, error) {
	host := getEnv("POSTGRES_HOST", "localhost")
	// 如果是 Docker 服务名，替换为 localhost
	if host == "postgres" {
		host = "localhost"
	}
	port := getEnv("POSTGRES_PORT", "5432")
	dbname := getEnv("POSTGRES_DB", "nofx")
	user := getEnv("POSTGRES_USER", "nofx")
	password := getEnv("POSTGRES_PASSWORD", "nofx123456")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func migrateAIModels(db *sql.DB, cryptoService *crypto.CryptoService, dryRun bool) error {
	type record struct {
		ID     string
		UserID string
		APIKey string
	}

	rows, err := db.Query(`
		SELECT id, user_id, COALESCE(api_key, '') 
		FROM ai_models 
		WHERE COALESCE(deleted, FALSE) = FALSE
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var records []record
	for rows.Next() {
		var r record
		if err := rows.Scan(&r.ID, &r.UserID, &r.APIKey); err != nil {
			return err
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	var updated int
	for _, r := range records {
		if r.APIKey == "" || cryptoService.IsEncryptedStorageValue(r.APIKey) {
			continue
		}

		encrypted, err := cryptoService.EncryptForStorage(r.APIKey, r.UserID, r.ID, "api_key")
		if err != nil {
			return fmt.Errorf("加密 AI 模型 %s (%s) 失败: %w", r.ID, r.UserID, err)
		}

		updated++
		if dryRun {
			log.Printf("[DRY-RUN] AI 模型 %s (%s) 将被加密", r.ID, r.UserID)
			continue
		}

		if _, err := db.Exec(`
			UPDATE ai_models 
			SET api_key = $1, updated_at = CURRENT_TIMESTAMP 
			WHERE id = $2 AND user_id = $3
		`, encrypted, r.ID, r.UserID); err != nil {
			return fmt.Errorf("更新 AI 模型 %s (%s) 失败: %w", r.ID, r.UserID, err)
		}
	}

	log.Printf("AI 模型处理完成，需更新 %d 条记录", updated)
	return nil
}

func migrateExchanges(db *sql.DB, cryptoService *crypto.CryptoService, dryRun bool) error {
	type record struct {
		ID                string
		UserID            string
		APIKey            string
		SecretKey         string
		HyperliquidWallet string
		AsterUser         string
		AsterSigner       string
		AsterPrivateKey   string
	}

	rows, err := db.Query(`
		SELECT id, user_id,
		       COALESCE(api_key, '') AS api_key,
		       COALESCE(secret_key, '') AS secret_key,
		       COALESCE(hyperliquid_wallet_addr, '') AS hyperliquid_wallet_addr,
		       COALESCE(aster_user, '') AS aster_user,
		       COALESCE(aster_signer, '') AS aster_signer,
		       COALESCE(aster_private_key, '') AS aster_private_key
		FROM exchanges
		WHERE COALESCE(deleted, FALSE) = FALSE
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var records []record
	for rows.Next() {
		var r record
		if err := rows.Scan(
			&r.ID, &r.UserID,
			&r.APIKey, &r.SecretKey,
			&r.HyperliquidWallet,
			&r.AsterUser, &r.AsterSigner, &r.AsterPrivateKey,
		); err != nil {
			return err
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	var updated int
	for _, r := range records {
		newAPIKey := r.APIKey
		newSecretKey := r.SecretKey
		newHyper := r.HyperliquidWallet
		newAsterUser := r.AsterUser
		newAsterSigner := r.AsterSigner
		newAsterPrivate := r.AsterPrivateKey

		changed := false

		if r.APIKey != "" && !cryptoService.IsEncryptedStorageValue(r.APIKey) {
			enc, err := cryptoService.EncryptForStorage(r.APIKey, r.UserID, r.ID, "api_key")
			if err != nil {
				return fmt.Errorf("加密交易所 API Key 失败: %s (%s): %w", r.ID, r.UserID, err)
			}
			newAPIKey = enc
			changed = true
		}
		if r.SecretKey != "" && !cryptoService.IsEncryptedStorageValue(r.SecretKey) {
			enc, err := cryptoService.EncryptForStorage(r.SecretKey, r.UserID, r.ID, "secret_key")
			if err != nil {
				return fmt.Errorf("加密交易所 Secret Key 失败: %s (%s): %w", r.ID, r.UserID, err)
			}
			newSecretKey = enc
			changed = true
		}
		if r.HyperliquidWallet != "" && !cryptoService.IsEncryptedStorageValue(r.HyperliquidWallet) {
			enc, err := cryptoService.EncryptForStorage(r.HyperliquidWallet, r.UserID, r.ID, "hyperliquid_wallet_addr")
			if err != nil {
				return fmt.Errorf("加密 Hyperliquid 地址失败: %s (%s): %w", r.ID, r.UserID, err)
			}
			newHyper = enc
			changed = true
		}
		if r.AsterUser != "" && !cryptoService.IsEncryptedStorageValue(r.AsterUser) {
			enc, err := cryptoService.EncryptForStorage(r.AsterUser, r.UserID, r.ID, "aster_user")
			if err != nil {
				return fmt.Errorf("加密 Aster 用户失败: %s (%s): %w", r.ID, r.UserID, err)
			}
			newAsterUser = enc
			changed = true
		}
		if r.AsterSigner != "" && !cryptoService.IsEncryptedStorageValue(r.AsterSigner) {
			enc, err := cryptoService.EncryptForStorage(r.AsterSigner, r.UserID, r.ID, "aster_signer")
			if err != nil {
				return fmt.Errorf("加密 Aster Signer 失败: %s (%s): %w", r.ID, r.UserID, err)
			}
			newAsterSigner = enc
			changed = true
		}
		if r.AsterPrivateKey != "" && !cryptoService.IsEncryptedStorageValue(r.AsterPrivateKey) {
			enc, err := cryptoService.EncryptForStorage(r.AsterPrivateKey, r.UserID, r.ID, "aster_private_key")
			if err != nil {
				return fmt.Errorf("加密 Aster 私钥失败: %s (%s): %w", r.ID, r.UserID, err)
			}
			newAsterPrivate = enc
			changed = true
		}

		if !changed {
			continue
		}

		updated++
		if dryRun {
			log.Printf("[DRY-RUN] 交易所 %s (%s) 将被加密", r.ID, r.UserID)
			continue
		}

		if _, err := db.Exec(`
			UPDATE exchanges
			SET api_key = $1,
			    secret_key = $2,
			    hyperliquid_wallet_addr = $3,
			    aster_user = $4,
			    aster_signer = $5,
			    aster_private_key = $6,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = $7 AND user_id = $8
		`, newAPIKey, newSecretKey, newHyper, newAsterUser, newAsterSigner, newAsterPrivate, r.ID, r.UserID); err != nil {
			return fmt.Errorf("更新交易所 %s (%s) 失败: %w", r.ID, r.UserID, err)
		}
	}

	log.Printf("交易所处理完成，需更新 %d 条记录", updated)
	return nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func loadEnvFile(filename string) error {
	// 检查文件是否存在
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("文件不存在: %s", filename)
	}

	// 打开文件
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("无法打开文件: %w", err)
	}
	defer file.Close()

	// 逐行读取
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// 跳过空行和注释行
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析 KEY=VALUE 格式
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// 只有当环境变量不存在时才设置
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}
