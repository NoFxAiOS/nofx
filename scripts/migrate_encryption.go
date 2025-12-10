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
	log.Println("🔄 Starting database migration to encrypted format...")

	// 1. Check database file
	dbPath := "data/data.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Fatalf("❌ Database file does not exist: %s", dbPath)
	}

	// 2. Backup database
	backupPath := fmt.Sprintf("%s.pre_encryption_backup", dbPath)
	log.Printf("📦 Backing up database to: %s", backupPath)

	input, err := os.ReadFile(dbPath)
	if err != nil {
		log.Fatalf("❌ Failed to read database: %v", err)
	}

	if err := os.WriteFile(backupPath, input, 0600); err != nil {
		log.Fatalf("❌ Backup failed: %v", err)
	}

	// 3. Open database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("❌ Failed to open database: %v", err)
	}
	defer db.Close()

	// 4. Initialize CryptoService (load key from environment variables)
	cs, err := crypto.NewCryptoService()
	if err != nil {
		log.Fatalf("❌ Failed to initialize encryption service: %v", err)
	}

	// 5. Migrate exchange configurations
	if err := migrateExchanges(db, cs); err != nil {
		log.Fatalf("❌ Failed to migrate exchange configurations: %v", err)
	}

	// 6. Migrate AI model configurations
	if err := migrateAIModels(db, cs); err != nil {
		log.Fatalf("❌ Failed to migrate AI model configurations: %v", err)
	}

	log.Println("✅ Data migration completed!")
	log.Printf("📝 Original data backed up at: %s", backupPath)
	log.Println("⚠️  Please verify system functionality before manually deleting backup file")
}

// migrateExchanges migrates exchange configurations
func migrateExchanges(db *sql.DB, cs *crypto.CryptoService) error {
	log.Println("🔄 Migrating exchange configurations...")

	// Query all unencrypted records (encrypted data starts with ENC:v1:)
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

		// Encrypt each field
		encAPIKey, err := cs.EncryptForStorage(apiKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt API Key: %w", err)
		}

		encSecretKey, err := cs.EncryptForStorage(secretKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt Secret Key: %w", err)
		}

		encHLPrivateKey := ""
		if hlPrivateKey != "" {
			encHLPrivateKey, err = cs.EncryptForStorage(hlPrivateKey)
			if err != nil {
				return fmt.Errorf("failed to encrypt Hyperliquid Private Key: %w", err)
			}
		}

		encAsterPrivateKey := ""
		if asterPrivateKey != "" {
			encAsterPrivateKey, err = cs.EncryptForStorage(asterPrivateKey)
			if err != nil {
				return fmt.Errorf("failed to encrypt Aster Private Key: %w", err)
			}
		}

		// Update database
		_, err = tx.Exec(`
			UPDATE exchanges
			SET api_key = ?, secret_key = ?,
			    hyperliquid_private_key = ?, aster_private_key = ?
			WHERE user_id = ? AND id = ?
		`, encAPIKey, encSecretKey, encHLPrivateKey, encAsterPrivateKey, userID, exchangeID)

		if err != nil {
			return fmt.Errorf("failed to update database: %w", err)
		}

		log.Printf("  ✓ Encrypted: [%s] %s", userID, exchangeID)
		count++
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("✅ Migrated %d exchange configurations", count)
	return nil
}

// migrateAIModels migrates AI model configurations
func migrateAIModels(db *sql.DB, cs *crypto.CryptoService) error {
	log.Println("🔄 Migrating AI model configurations...")

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
			return fmt.Errorf("failed to encrypt API Key: %w", err)
		}

		_, err = tx.Exec(`
			UPDATE ai_models SET api_key = ? WHERE user_id = ? AND id = ?
		`, encAPIKey, userID, modelID)

		if err != nil {
			return fmt.Errorf("failed to update database: %w", err)
		}

		log.Printf("  ✓ Encrypted: [%s] %s", userID, modelID)
		count++
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("✅ Migrated %d AI model configurations", count)
	return nil
}
