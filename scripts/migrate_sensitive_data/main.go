package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"nofx/crypto"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	privateKeyPath := flag.String("key", "keys/rsa_private.key", "RSA 私钥路径")
	dryRun := flag.Bool("dry-run", false, "仅检查需要迁移的数据，不写入数据库")
	flag.Parse()

	if err := run(*privateKeyPath, *dryRun); err != nil {
		log.Fatalf("迁移失败: %v", err)
	}
}

func run(privateKeyPath string, dryRun bool) error {
	log.SetFlags(0)

	cryptoService, err := crypto.NewCryptoService(privateKeyPath)
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
