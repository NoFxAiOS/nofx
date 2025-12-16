package main

import (
	"log"
	"nofx/config"
	"os"
)

// SQL content for the migration
const migrationSQL = `
INSERT INTO system_config (key, value) VALUES ('mlion_api_key', 'c559b9a8-80c2-4c17-8c31-bb7659b12b52') ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
INSERT INTO system_config (key, value) VALUES ('mlion_target_topic_id', '17758') ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
INSERT INTO system_config (key, value) VALUES ('mlion_news_enabled', 'true') ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
`

func main() {
	log.Println("🚀 Starting Mlion Config Migration...")

	// 1. Initialize Database
	// It relies on DATABASE_URL environment variable being set.
	// We pass a dummy path "config.db" as it's required by the signature but ignored for Postgres connections.
	db, err := config.NewDatabase("config.db")
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v. Please check DATABASE_URL env var.", err)
	}
	defer db.GetDB().Close()

	// 2. Execute SQL
	log.Println("🔄 Executing migration SQL...")
	_, err = db.GetDB().Exec(migrationSQL)
	if err != nil {
		log.Fatalf("❌ Migration failed: %v", err)
	}

	// 3. Verify
	val, err := db.GetSystemConfig("mlion_news_enabled")
	if err != nil {
		log.Printf("⚠️  Verification check failed: %v", err)
	} else if val == "true" {
		log.Println("✅ Migration applied successfully! Mlion news is enabled.")
	} else {
		log.Printf("⚠️  Migration ran but verification returned '%s' instead of 'true'.", val)
	}

	// Explicit exit code for success
	os.Exit(0)
}
