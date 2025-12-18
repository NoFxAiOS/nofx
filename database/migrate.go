package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
)

// MigrateData 从SQLite迁移数据到Neon PostgreSQL
func MigrateData(sqlitePath, neonDSN string) error {
	log.Println("开始从SQLite迁移数据到Neon PostgreSQL...")

	// 连接SQLite
	sqliteDB, err := sql.Open("sqlite3", sqlitePath)
	if err != nil {
		return fmt.Errorf("连接SQLite失败: %w", err)
	}
	defer sqliteDB.Close()

	// 连接Neon PostgreSQL
	pgDB, err := sql.Open("pgx", neonDSN)
	if err != nil {
		return fmt.Errorf("连接Neon失败: %w", err)
	}
	defer pgDB.Close()

	// 创建表结构（确保与原SQLite表结构兼容）
	tables := []string{
		// 用户表
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(128) NOT NULL UNIQUE,
			password_hash VARCHAR(256) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			is_admin BOOLEAN DEFAULT false
		)`,

		// 交易所配置表
		`CREATE TABLE IF NOT EXISTS exchanges (
			id SERIAL PRIMARY KEY,
			user_id INT NOT NULL,
			type VARCHAR(32) NOT NULL,
			api_key VARCHAR(512) NOT NULL,
			secret_key VARCHAR(512) NOT NULL,
			passphrase VARCHAR(256),
			testnet BOOLEAN DEFAULT false,
			enabled BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// AI模型配置表
		`CREATE TABLE IF NOT EXISTS ai_models (
			id SERIAL PRIMARY KEY,
			user_id INT NOT NULL,
			provider VARCHAR(32) NOT NULL,
			api_key VARCHAR(512) NOT NULL,
			custom_api_url VARCHAR(256),
			custom_model_name VARCHAR(128),
			enabled BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// 性能快照表 (每日快照)
		`CREATE TABLE IF NOT EXISTS performance_snapshots (
			id SERIAL PRIMARY KEY,
			user_id INT NOT NULL,
			trader_id VARCHAR(64) NOT NULL,
			snapshot_date DATE NOT NULL,
			total_trades INT,
			winning_trades INT,
			losing_trades INT,
			win_rate DECIMAL(5,2),
			sharpe_ratio DECIMAL(8,4),
			max_drawdown_pct DECIMAL(8,4),
			consecutive_losses INT,
			max_consecutive_loss INT,
			volatility DECIMAL(8,4),
			weighted_win_rate DECIMAL(5,2),
			profit_factor DECIMAL(8,4),
			avg_profit_per_win DECIMAL(12,4),
			avg_loss_per_loss DECIMAL(12,4),
			best_performing_pair VARCHAR(32),
			worst_performing_pair VARCHAR(32),
			best_trading_hour INT,
			total_pnl DECIMAL(15,4),
			total_pnl_pct DECIMAL(8,4),
			equity DECIMAL(15,4),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(trader_id, snapshot_date)
		)`,

		// 币种性能统计表 (逐个币种详细统计)
		`CREATE TABLE IF NOT EXISTS symbol_performance (
			id SERIAL PRIMARY KEY,
			user_id INT NOT NULL,
			trader_id VARCHAR(64) NOT NULL,
			symbol VARCHAR(32) NOT NULL,
			snapshot_date DATE NOT NULL,
			trades_count INT,
			win_rate DECIMAL(5,2),
			avg_profit_pct DECIMAL(8,4),
			avg_loss_pct DECIMAL(8,4),
			best_trade_pct DECIMAL(8,4),
			worst_trade_pct DECIMAL(8,4),
			volatility DECIMAL(8,4),
			max_drawdown_pct DECIMAL(8,4),
			profit_factor DECIMAL(8,4),
			total_profit DECIMAL(15,4),
			total_loss DECIMAL(15,4),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(trader_id, symbol, snapshot_date)
		)`,

		// 亏损事件表 (记录触发断路器的事件)
		`CREATE TABLE IF NOT EXISTS loss_events (
			id SERIAL PRIMARY KEY,
			user_id INT NOT NULL,
			trader_id VARCHAR(64) NOT NULL,
			event_type VARCHAR(32) NOT NULL,
			event_reason VARCHAR(256) NOT NULL,
			consecutive_losses INT,
			daily_loss_pct DECIMAL(8,4),
			weekly_loss_pct DECIMAL(8,4),
			max_drawdown_pct DECIMAL(8,4),
			account_equity DECIMAL(15,4),
			breach_triggered BOOLEAN DEFAULT false,
			recovery_attempt INT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
	}

	for _, tableSQL := range tables {
		if _, err := pgDB.Exec(tableSQL); err != nil {
			return fmt.Errorf("创建表失败: %w", err)
		}
		log.Printf("已创建表: %s", getTableName(tableSQL))
	}

	// 迁移用户数据
	if err := migrateUsers(sqliteDB, pgDB); err != nil {
		return fmt.Errorf("迁移用户数据失败: %w", err)
	}

	// 迁移交易所配置
	if err := migrateExchanges(sqliteDB, pgDB); err != nil {
		return fmt.Errorf("迁移交易所配置失败: %w", err)
	}

	// 迁移AI模型配置
	if err := migrateAIModels(sqliteDB, pgDB); err != nil {
		return fmt.Errorf("迁移AI模型配置失败: %w", err)
	}

	log.Println("数据迁移完成！")
	return nil
}

// 辅助函数
func getTableName(sql string) string {
	// 简单解析表名（实际项目中可以使用更可靠的解析方法）
	return "unknown"
}

// 迁移用户数据
func migrateUsers(sqliteDB, pgDB *sql.DB) error {
	rows, err := sqliteDB.Query("SELECT id, email, password_hash, is_admin FROM users")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id interface{}
		var email string
		var passwordHash string
		var isAdmin bool

		if err := rows.Scan(&id, &email, &passwordHash, &isAdmin); err != nil {
			return err
		}

		// 转换id为合适的类型
		var pgID int
		switch v := id.(type) {
		case int:
			pgID = v
		case int64:
			pgID = int(v)
		case float64:
			pgID = int(v)
		case string:
			// 如果id是字符串类型，跳过
			continue
		default:
			// 其他未知类型，跳过
			continue
		}

		_, err := pgDB.Exec("INSERT INTO users (id, email, password_hash, is_admin) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO NOTHING",
			pgID, email, passwordHash, isAdmin)
		if err != nil {
			return err
		}
	}

	return nil
}

// 迁移交易所配置
func migrateExchanges(sqliteDB, pgDB *sql.DB) error {
	rows, err := sqliteDB.Query("SELECT id, user_id, type, api_key, secret_key, passphrase, testnet, enabled FROM exchanges")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var userID int
		var exchangeType string
		var apiKey string
		var secretKey string
		var passphrase sql.NullString
		var useTestnet bool
		var enabled bool

		if err := rows.Scan(&id, &userID, &exchangeType, &apiKey, &secretKey, &passphrase, &useTestnet, &enabled); err != nil {
			return err
		}

		_, err := pgDB.Exec("INSERT INTO exchanges (id, user_id, type, api_key, secret_key, passphrase, testnet, enabled) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT (id) DO NOTHING",
			id, userID, exchangeType, apiKey, secretKey, passphrase.String, useTestnet, enabled)
		if err != nil {
			return err
		}
	}

	return nil
}

// 迁移AI模型配置
func migrateAIModels(sqliteDB, pgDB *sql.DB) error {
	rows, err := sqliteDB.Query("SELECT id, user_id, provider, api_key, custom_api_url, custom_model_name, enabled FROM ai_models")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var userID int
		var modelType string
		var apiKey string
		var customAPIURL sql.NullString
		var customModelName sql.NullString
		var enabled bool

		if err := rows.Scan(&id, &userID, &modelType, &apiKey, &customAPIURL, &customModelName, &enabled); err != nil {
			return err
		}

		_, err := pgDB.Exec("INSERT INTO ai_models (id, user_id, provider, api_key, custom_api_url, custom_model_name, enabled) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (id) DO NOTHING",
			id, userID, modelType, apiKey, customAPIURL.String, customModelName.String, enabled)
		if err != nil {
			return err
		}
	}

	return nil
}
