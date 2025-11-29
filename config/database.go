package config

import (
        "crypto/rand"
        "database/sql"
        "encoding/base32"
        "encoding/json"
        "fmt"
        "log"
        "nofx/market"
        "os"
        "slices"
        "strings"
        "time"

        "github.com/google/uuid"
        _ "github.com/lib/pq"
)

// Database 配置数据库
type Database struct {
        db *sql.DB
}

// NewDatabase 创建配置数据库（仅支持PostgreSQL）
func NewDatabase(dbPath string) (*Database, error) {
        databaseURL := os.Getenv("DATABASE_URL")
        if databaseURL == "" {
                return nil, fmt.Errorf("DATABASE_URL环境变量未设置")
        }

        log.Println("🔄 连接PostgreSQL数据库...")
        db, err := sql.Open("postgres", databaseURL)
        if err != nil {
                return nil, fmt.Errorf("连接数据库失败: %w", err)
        }

        // 配置连接池 - 针对Neon PostgreSQL serverless优化
        // 这些设置有助于防止冷启动问题
        db.SetMaxOpenConns(10)                  // 最大打开连接数
        db.SetMaxIdleConns(5)                   // 最大空闲连接数
        db.SetConnMaxIdleTime(30 * time.Second) // 空闲连接最大存活时间
        db.SetConnMaxLifetime(5 * time.Minute)  // 连接最大生命周期
        log.Println("📋 数据库连接池配置: MaxOpen=10, MaxIdle=5, IdleTime=30s, Lifetime=5m")

        if pingErr := db.Ping(); pingErr != nil {
                db.Close()
                return nil, fmt.Errorf("数据库连接测试失败: %w", pingErr)
        }

        log.Println("✅ 成功连接PostgreSQL数据库!")

        database := &Database{db: db}
        log.Println("🔄 开始创建表...")
        if err := database.createTables(); err != nil {
                return nil, fmt.Errorf("创建表失败: %w", err)
        }
        log.Println("✅ 表创建成功!")

        log.Println("🔄 开始修改表结构...")
        if err := database.alterTables(); err != nil {
                log.Printf("⚠️ 数据库迁移警告: %v", err)
        }
        log.Println("✅ 表结构修改完成!")

        log.Println("🔄 开始初始化默认数据...")
        if err := database.initDefaultData(); err != nil {
                return nil, fmt.Errorf("初始化默认数据失败: %w", err)
        }
        log.Println("✅ 默认数据初始化完成!")

        return database, nil
}

// isTransientError 检查是否是可重试的临时错误
func isTransientError(err error) bool {
        if err == nil {
                return false
        }
        errStr := err.Error()
        // Neon PostgreSQL冷启动和连接断开的常见错误
        // 参考: https://neon.com/docs/connect/connection-errors
        transientErrors := []string{
                "connection not available",
                "connection reset",
                "connection refused",
                "broken pipe",
                "EOF",
                "timeout",
                "no connection",
                "connection is closed",
                "unexpected EOF",
                "driver: bad connection",
                // Neon 特有的冷启动错误
                "terminating connection due to administrator command",
                "can't reach database server",
                "the database system is starting up",
                "connection timed out",
                "network is unreachable",
                "i/o timeout",
                "connection reset by peer",
                "no such host",
        }
        for _, te := range transientErrors {
                if strings.Contains(strings.ToLower(errStr), strings.ToLower(te)) {
                        return true
                }
        }
        return false
}

// withRetry 执行带重试的数据库操作
// 最多重试3次，使用指数退避策略（100ms, 200ms, 400ms）
func withRetry[T any](operation func() (T, error)) (T, error) {
        var result T
        var lastErr error
        maxRetries := 3
        baseDelay := 100 * time.Millisecond

        for attempt := 0; attempt < maxRetries; attempt++ {
                result, lastErr = operation()
                if lastErr == nil {
                        return result, nil
                }

                if !isTransientError(lastErr) {
                        // 非临时性错误，不重试
                        return result, lastErr
                }

                if attempt < maxRetries-1 {
                        delay := baseDelay * time.Duration(1<<attempt) // 指数退避: 100ms, 200ms, 400ms
                        log.Printf("⚠️ 数据库操作失败 (尝试 %d/%d): %v, %v后重试...", attempt+1, maxRetries, lastErr, delay)
                        time.Sleep(delay)
                }
        }

        return result, fmt.Errorf("数据库操作重试%d次后仍失败: %w", maxRetries, lastErr)
}

// Ping 检测数据库连接（带重试）
func (d *Database) Ping() error {
        _, err := withRetry(func() (bool, error) {
                return true, d.db.Ping()
        })
        return err
}

// StartKeepAlive 启动后台连接保活协程
// 每5分钟执行一次ping，防止Neon PostgreSQL连接被断开
func (d *Database) StartKeepAlive() {
        go func() {
                ticker := time.NewTicker(5 * time.Minute)
                defer ticker.Stop()
                for range ticker.C {
                        if err := d.db.Ping(); err != nil {
                                log.Printf("⚠️ 数据库保活ping失败: %v", err)
                        }
                }
        }()
        log.Println("🔄 数据库连接保活协程已启动 (每5分钟ping一次)")
}

// convertPlaceholders 将?占位符转换为PostgreSQL的$1, $2格式
func (d *Database) convertPlaceholders(query string) string {
        result := query
        index := 1
        for strings.Contains(result, "?") {
                result = strings.Replace(result, "?", fmt.Sprintf("$%d", index), 1)
                index++
        }
        return result
}

// query 执行查询并自动转换占位符
func (d *Database) query(query string, args ...interface{}) (*sql.Rows, error) {
        return d.db.Query(d.convertPlaceholders(query), args...)
}

// queryRow 执行单行查询并自动转换占位符
func (d *Database) queryRow(query string, args ...interface{}) *sql.Row {
        return d.db.QueryRow(d.convertPlaceholders(query), args...)
}

// exec 执行语句并自动转换占位符
func (d *Database) exec(query string, args ...interface{}) (sql.Result, error) {
        return d.db.Exec(d.convertPlaceholders(query), args...)
}

// createTables 创建数据库表
func (d *Database) createTables() error {
        return d.createTablesPostgres()
}

// createTablesPostgres PostgreSQL版本的表创建
func (d *Database) createTablesPostgres() error {
        queries := []string{
                // 用户表 (必须先创建，因为其他表依赖它)
                `CREATE TABLE IF NOT EXISTS users (
                        id TEXT PRIMARY KEY,
                        email TEXT UNIQUE NOT NULL,
                        password_hash TEXT NOT NULL,
                        otp_secret TEXT,
                        otp_verified BOOLEAN DEFAULT false,
                        locked_until TIMESTAMP,
                        failed_attempts INTEGER DEFAULT 0,
                        last_failed_at TIMESTAMP,
                        is_active BOOLEAN DEFAULT true,
                        is_admin BOOLEAN DEFAULT false,
                        beta_code TEXT,
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )`,

                // AI模型配置表
                `CREATE TABLE IF NOT EXISTS ai_models (
                        id TEXT PRIMARY KEY,
                        user_id TEXT NOT NULL DEFAULT 'default',
                        name TEXT NOT NULL,
                        provider TEXT NOT NULL,
                        enabled BOOLEAN DEFAULT false,
                        api_key TEXT DEFAULT '',
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )`,

                // 交易所配置表
                `CREATE TABLE IF NOT EXISTS exchanges (
                        id TEXT PRIMARY KEY,
                        user_id TEXT NOT NULL DEFAULT 'default',
                        name TEXT NOT NULL,
                        type TEXT NOT NULL,
                        enabled BOOLEAN DEFAULT false,
                        api_key TEXT DEFAULT '',
                        secret_key TEXT DEFAULT '',
                        testnet BOOLEAN DEFAULT false,
                        hyperliquid_wallet_addr TEXT DEFAULT '',
                        aster_user TEXT DEFAULT '',
                        aster_signer TEXT DEFAULT '',
                        aster_private_key TEXT DEFAULT '',
                        passphrase TEXT DEFAULT '',
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )`,

                // 用户信号源配置表
                `CREATE TABLE IF NOT EXISTS user_signal_sources (
                        id SERIAL PRIMARY KEY,
                        user_id TEXT NOT NULL UNIQUE,
                        coin_pool_url TEXT DEFAULT '',
                        oi_top_url TEXT DEFAULT '',
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )`,

                // 交易员配置表
                `CREATE TABLE IF NOT EXISTS traders (
                        id TEXT PRIMARY KEY,
                        user_id TEXT NOT NULL DEFAULT 'default',
                        name TEXT NOT NULL,
                        ai_model_id TEXT NOT NULL,
                        exchange_id TEXT NOT NULL,
                        initial_balance REAL NOT NULL,
                        scan_interval_minutes INTEGER DEFAULT 3,
                        is_running BOOLEAN DEFAULT false,
                        btc_eth_leverage INTEGER DEFAULT 5,
                        altcoin_leverage INTEGER DEFAULT 5,
                        trading_symbols TEXT DEFAULT '',
                        use_coin_pool BOOLEAN DEFAULT false,
                        use_oi_top BOOLEAN DEFAULT false,
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )`,

                // 密码重置令牌表
                `CREATE TABLE IF NOT EXISTS password_resets (
                        id TEXT PRIMARY KEY,
                        user_id TEXT NOT NULL,
                        token_hash TEXT NOT NULL,
                        expires_at TIMESTAMP NOT NULL,
                        used_at TIMESTAMP,
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )`,

                // 登录尝试记录表
                `CREATE TABLE IF NOT EXISTS login_attempts (
                        id TEXT PRIMARY KEY,
                        user_id TEXT,
                        email TEXT NOT NULL,
                        ip_address TEXT NOT NULL,
                        success BOOLEAN NOT NULL,
                        timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                        user_agent TEXT
                )`,

                // 审计日志表
                `CREATE TABLE IF NOT EXISTS audit_logs (
                        id TEXT PRIMARY KEY,
                        user_id TEXT,
                        action TEXT NOT NULL,
                        ip_address TEXT NOT NULL,
                        user_agent TEXT,
                        success BOOLEAN NOT NULL,
                        details TEXT,
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )`,

                // 系统配置表
                `CREATE TABLE IF NOT EXISTS system_config (
                        key TEXT PRIMARY KEY,
                        value TEXT NOT NULL,
                        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )`,

                // 内测码表
                `CREATE TABLE IF NOT EXISTS beta_codes (
                        code TEXT PRIMARY KEY,
                        used BOOLEAN DEFAULT false,
                        used_by TEXT DEFAULT '',
                        used_at TIMESTAMP DEFAULT NULL,
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )`,
        }

        for _, query := range queries {
                if _, err := d.exec(query); err != nil {
                        return fmt.Errorf("执行SQL失败: %w, SQL: %s", err, query[:min(100, len(query))])
                }
        }

        log.Println("✅ PostgreSQL表创建成功")
        return nil
}


// 执行数据库迁移查询
func (d *Database) executeQueries(queries []string) error {
        for _, query := range queries {
                if _, err := d.exec(query); err != nil {
                        return fmt.Errorf("执行SQL失败 [%s]: %w", query, err)
                }
        }
        return nil
}

// 为现有数据库添加新字段（向后兼容）
func (d *Database) alterTables() error {
        alterQueries := []string{
                // 添加users表缺失的字段
                `ALTER TABLE users ADD COLUMN locked_until TIMESTAMP`,
                `ALTER TABLE users ADD COLUMN failed_attempts INTEGER DEFAULT 0`,
                `ALTER TABLE users ADD COLUMN last_failed_at TIMESTAMP`,
                `ALTER TABLE users ADD COLUMN is_active BOOLEAN DEFAULT 1`,
                `ALTER TABLE users ADD COLUMN is_admin BOOLEAN DEFAULT 0`,
                `ALTER TABLE users ADD COLUMN beta_code TEXT`,
                // 添加exchanges表字段
                `ALTER TABLE exchanges ADD COLUMN hyperliquid_wallet_addr TEXT DEFAULT ''`,
                `ALTER TABLE exchanges ADD COLUMN aster_user TEXT DEFAULT ''`,
                `ALTER TABLE exchanges ADD COLUMN aster_signer TEXT DEFAULT ''`,
                `ALTER TABLE exchanges ADD COLUMN aster_private_key TEXT DEFAULT ''`,
                `ALTER TABLE exchanges ADD COLUMN okx_passphrase TEXT DEFAULT ''`,
                // 添加traders表字段
                `ALTER TABLE traders ADD COLUMN custom_prompt TEXT DEFAULT ''`,
                `ALTER TABLE traders ADD COLUMN override_base_prompt BOOLEAN DEFAULT 0`,
                `ALTER TABLE traders ADD COLUMN is_cross_margin BOOLEAN DEFAULT 1`,             // 默认为全仓模式
                `ALTER TABLE traders ADD COLUMN use_default_coins BOOLEAN DEFAULT 1`,           // 默认使用默认币种
                `ALTER TABLE traders ADD COLUMN custom_coins TEXT DEFAULT ''`,                  // 自定义币种列表（JSON格式）
                `ALTER TABLE traders ADD COLUMN btc_eth_leverage INTEGER DEFAULT 5`,            // BTC/ETH杠杆倍数
                `ALTER TABLE traders ADD COLUMN altcoin_leverage INTEGER DEFAULT 5`,            // 山寨币杠杆倍数
                `ALTER TABLE traders ADD COLUMN trading_symbols TEXT DEFAULT ''`,               // 交易币种，逗号分隔
                `ALTER TABLE traders ADD COLUMN use_coin_pool BOOLEAN DEFAULT 0`,               // 是否使用COIN POOL信号源
                `ALTER TABLE traders ADD COLUMN use_oi_top BOOLEAN DEFAULT 0`,                  // 是否使用OI TOP信号源
                `ALTER TABLE traders ADD COLUMN system_prompt_template TEXT DEFAULT 'default'`, // 系统提示词模板名称
                // 添加ai_models表字段
                `ALTER TABLE ai_models ADD COLUMN custom_api_url TEXT DEFAULT ''`,              // 自定义API地址
                `ALTER TABLE ai_models ADD COLUMN custom_model_name TEXT DEFAULT ''`,           // 自定义模型名称
        }

        for _, query := range alterQueries {
                // 忽略已存在字段的错误
                d.exec(query)
        }

        // 检查是否需要迁移exchanges表的主键结构
        err := d.migrateExchangesTable()
        if err != nil {
                log.Printf("⚠️ 迁移exchanges表失败: %v", err)
        }

        return nil
}

// initDefaultData 初始化默认数据
func (d *Database) initDefaultData() error {
        // 确保default系统用户存在（必须在初始化默认数据之前）
        if err := d.EnsureDefaultUser(); err != nil {
                return fmt.Errorf("创建default用户失败: %w", err)
        }

        // 确保admin用户存在（如果启用admin模式）
        if err := d.EnsureAdminUser(); err != nil {
                return fmt.Errorf("创建admin用户失败: %w", err)
        }

        // 初始化AI模型（使用default用户）
        aiModels := []struct {
                id, name, provider string
        }{
                {"deepseek", "DeepSeek", "deepseek"},
                {"qwen", "Qwen", "qwen"},
        }

        for _, model := range aiModels {
                _, err := d.exec(`
                        INSERT INTO ai_models (id, user_id, name, provider, enabled)
                        VALUES ($1, 'default', $2, $3, false)
                        ON CONFLICT (id) DO NOTHING
                `, model.id, model.name, model.provider)
                if err != nil {
                        return fmt.Errorf("初始化AI模型失败: %w", err)
                }
        }

        // 初始化交易所（使用default用户）
        exchanges := []struct {
                id, name, typ string
        }{
                {"binance", "Binance Futures", "cex"},
                {"hyperliquid", "Hyperliquid", "dex"},
                {"aster", "Aster DEX", "dex"},
                {"okx", "OKX Futures", "cex"},
        }

        for _, exchange := range exchanges {
                _, err := d.exec(`
                        INSERT INTO exchanges (id, user_id, name, type, enabled)
                        VALUES ($1, 'default', $2, $3, false)
                        ON CONFLICT (id, user_id) DO NOTHING
                `, exchange.id, exchange.name, exchange.typ)
                if err != nil {
                        return fmt.Errorf("初始化交易所失败: %w", err)
                }
        }

        // 初始化系统配置 - 创建所有字段，设置默认值，后续由config.json同步更新
        systemConfigs := map[string]string{
                "admin_mode":            "true",
                "beta_mode":             "false",
                "api_server_port":       "8080",
                "use_default_coins":     "true",
                "default_coins":         `["BTCUSDT","ETHUSDT","SOLUSDT","BNBUSDT","XRPUSDT","DOGEUSDT","ADAUSDT","HYPEUSDT"]`,
                "max_daily_loss":        "10.0",
                "max_drawdown":          "20.0",
                "stop_trading_minutes":  "60",
                "btc_eth_leverage":      "5",
                "altcoin_leverage":      "5",
                "jwt_secret":            "",
        }

        for key, value := range systemConfigs {
                _, err := d.exec(`
                        INSERT INTO system_config (key, value)
                        VALUES ($1, $2)
                        ON CONFLICT (key) DO NOTHING
                `, key, value)
                if err != nil {
                        return fmt.Errorf("初始化系统配置失败: %w", err)
                }
        }

        return nil
}

// migrateExchangesTable 迁移exchanges表支持多用户
func (d *Database) migrateExchangesTable() error {
        // PostgreSQL不需要这个迁移，已经在createTablesPostgres中创建了正确的表结构
        return nil
}

// User 用户配置
type User struct {
        ID             string     `json:"id"`
        Email          string     `json:"email"`
        PasswordHash   string     `json:"-"` // 不返回到前端
        OTPSecret      string     `json:"-"` // 不返回到前端
        OTPVerified    bool       `json:"otp_verified"`
        LockedUntil    *time.Time `json:"-"` // 账户锁定到期时间，不返回前端
        FailedAttempts int        `json:"-"` // 失败尝试次数，不返回前端
        LastFailedAt   *time.Time `json:"-"` // 最后失败时间，不返回前端
        IsActive       bool       `json:"is_active"`
        IsAdmin        bool       `json:"is_admin"`
        BetaCode       string     `json:"-"` // 内测码，不返回前端
        CreatedAt      time.Time  `json:"created_at"`
        UpdatedAt      time.Time  `json:"updated_at"`
}

// AIModelConfig AI模型配置
type AIModelConfig struct {
        ID              string    `json:"id"`
        UserID          string    `json:"user_id"`
        Name            string    `json:"name"`
        Provider        string    `json:"provider"`
        Enabled         bool      `json:"enabled"`
        APIKey          string    `json:"apiKey"`
        CustomAPIURL    string    `json:"customApiUrl"`
        CustomModelName string    `json:"customModelName"`
        CreatedAt       time.Time `json:"created_at"`
        UpdatedAt       time.Time `json:"updated_at"`
}

// ExchangeConfig 交易所配置
type ExchangeConfig struct {
        ID        string `json:"id"`
        UserID    string `json:"user_id"`
        Name      string `json:"name"`
        Type      string `json:"type"`
        Enabled   bool   `json:"enabled"`
        APIKey    string `json:"apiKey"`
        SecretKey string `json:"secretKey"`
        Testnet   bool   `json:"testnet"`
        // Hyperliquid 特定字段
        HyperliquidWalletAddr string `json:"hyperliquidWalletAddr"`
        // Aster 特定字段
        AsterUser       string    `json:"asterUser"`
        AsterSigner     string    `json:"asterSigner"`
        AsterPrivateKey string    `json:"asterPrivateKey"`
        // OKX 特定字段
        OKXPassphrase   string    `json:"okxPassphrase"`
        CreatedAt       time.Time `json:"created_at"`
        UpdatedAt       time.Time `json:"updated_at"`
}

// TraderRecord 交易员配置（数据库实体）
type TraderRecord struct {
        ID                   string    `json:"id"`
        UserID               string    `json:"user_id"`
        Name                 string    `json:"name"`
        AIModelID            string    `json:"ai_model_id"`
        ExchangeID           string    `json:"exchange_id"`
        InitialBalance       float64   `json:"initial_balance"`
        ScanIntervalMinutes  int       `json:"scan_interval_minutes"`
        IsRunning            bool      `json:"is_running"`
        BTCETHLeverage       int       `json:"btc_eth_leverage"`       // BTC/ETH杠杆倍数
        AltcoinLeverage      int       `json:"altcoin_leverage"`       // 山寨币杠杆倍数
        TradingSymbols       string    `json:"trading_symbols"`        // 交易币种，逗号分隔
        UseCoinPool          bool      `json:"use_coin_pool"`          // 是否使用COIN POOL信号源
        UseOITop             bool      `json:"use_oi_top"`             // 是否使用OI TOP信号源
        CustomPrompt         string    `json:"custom_prompt"`          // 自定义交易策略prompt
        OverrideBasePrompt   bool      `json:"override_base_prompt"`   // 是否覆盖基础prompt
        SystemPromptTemplate string    `json:"system_prompt_template"` // 系统提示词模板名称
        IsCrossMargin        bool      `json:"is_cross_margin"`        // 是否为全仓模式（true=全仓，false=逐仓）
        CreatedAt            time.Time `json:"created_at"`
        UpdatedAt            time.Time `json:"updated_at"`
}

// UserSignalSource 用户信号源配置
type UserSignalSource struct {
        ID          int       `json:"id"`
        UserID      string    `json:"user_id"`
        CoinPoolURL string    `json:"coin_pool_url"`
        OITopURL    string    `json:"oi_top_url"`
        CreatedAt   time.Time `json:"created_at"`
        UpdatedAt   time.Time `json:"updated_at"`
}

// GenerateOTPSecret 生成OTP密钥
func GenerateOTPSecret() (string, error) {
        secret := make([]byte, 20)
        _, err := rand.Read(secret)
        if err != nil {
                return "", err
        }
        return base32.StdEncoding.EncodeToString(secret), nil
}

// CreateUser 创建用户
func (d *Database) CreateUser(user *User) error {
        // 处理可空时间字段
        var lockedUntil, lastFailedAt sql.NullTime
        if user.LockedUntil != nil {
                lockedUntil = sql.NullTime{Time: *user.LockedUntil, Valid: true}
        }
        if user.LastFailedAt != nil {
                lastFailedAt = sql.NullTime{Time: *user.LastFailedAt, Valid: true}
        }

        _, err := d.exec(`
                INSERT INTO users (id, email, password_hash, otp_secret, otp_verified,
                                   locked_until, failed_attempts, last_failed_at,
                                   is_active, is_admin, beta_code, created_at, updated_at)
                VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
        `, user.ID, user.Email, user.PasswordHash, user.OTPSecret, user.OTPVerified,
                lockedUntil, user.FailedAttempts, lastFailedAt,
                user.IsActive, user.IsAdmin, user.BetaCode, user.CreatedAt, user.UpdatedAt)
        return err
}

// EnsureDefaultUser 确保default系统用户存在（用于存储系统级别配置）
func (d *Database) EnsureDefaultUser() error {
        // 检查default用户是否已存在
        var count int
        err := d.queryRow(`SELECT COUNT(*) FROM users WHERE id = 'default'`).Scan(&count)
        if err != nil {
                return err
        }

        // 如果已存在，直接返回
        if count > 0 {
                return nil
        }

        // 创建default用户（系统级别用户，用于存储系统默认配置）
        now := time.Now()
        defaultUser := &User{
                ID:             "default",
                Email:          "default@system",
                PasswordHash:   "", // 系统用户不需要密码
                OTPSecret:      "",
                OTPVerified:    true,
                IsActive:       true,
                IsAdmin:        false, // 不是管理员，只是系统用户
                FailedAttempts: 0,
                CreatedAt:      now,
                UpdatedAt:      now,
        }

        log.Println("📝 创建default系统用户（用于存储系统级别配置）...")
        return d.CreateUser(defaultUser)
}

// EnsureAdminUser 确保admin用户存在（用于管理员模式）
func (d *Database) EnsureAdminUser() error {
        // 检查admin用户是否已存在
        var count int
        err := d.queryRow(`SELECT COUNT(*) FROM users WHERE id = 'admin'`).Scan(&count)
        if err != nil {
                return err
        }

        // 如果已存在，直接返回
        if count > 0 {
                return nil
        }

        // 创建admin用户（密码为空，因为管理员模式下不需要密码）
        now := time.Now()
        adminUser := &User{
                ID:             "admin",
                Email:          "admin@localhost",
                PasswordHash:   "", // 管理员模式下不使用密码
                OTPSecret:      "",
                OTPVerified:    true,
                IsActive:       true,
                IsAdmin:        true,
                FailedAttempts: 0,
                CreatedAt:      now,
                UpdatedAt:      now,
        }

        return d.CreateUser(adminUser)
}

// GetUserByEmail 通过邮箱获取用户
func (d *Database) GetUserByEmail(email string) (*User, error) {
        var user User
        var lockedUntil, lastFailedAt sql.NullTime
        err := d.queryRow(`
                SELECT id, email, password_hash, otp_secret, otp_verified,
                       locked_until, failed_attempts, last_failed_at,
                       is_active, is_admin, beta_code,
                       created_at, updated_at
                FROM users WHERE email = ?
        `, email).Scan(
                &user.ID, &user.Email, &user.PasswordHash, &user.OTPSecret, &user.OTPVerified,
                &lockedUntil, &user.FailedAttempts, &lastFailedAt,
                &user.IsActive, &user.IsAdmin, &user.BetaCode,
                &user.CreatedAt, &user.UpdatedAt,
        )
        if err != nil {
                return nil, err
        }
        if lockedUntil.Valid {
                user.LockedUntil = &lockedUntil.Time
        }
        if lastFailedAt.Valid {
                user.LastFailedAt = &lastFailedAt.Time
        }
        return &user, nil
}

// GetUserByID 通过ID获取用户
func (d *Database) GetUserByID(userID string) (*User, error) {
        return withRetry(func() (*User, error) {
                var user User
                var lockedUntil, lastFailedAt sql.NullTime
                err := d.queryRow(`
                        SELECT id, email, password_hash, otp_secret, otp_verified,
                               locked_until, failed_attempts, last_failed_at,
                               is_active, is_admin, beta_code,
                               created_at, updated_at
                        FROM users WHERE id = ?
                `, userID).Scan(
                        &user.ID, &user.Email, &user.PasswordHash, &user.OTPSecret, &user.OTPVerified,
                        &lockedUntil, &user.FailedAttempts, &lastFailedAt,
                        &user.IsActive, &user.IsAdmin, &user.BetaCode,
                        &user.CreatedAt, &user.UpdatedAt,
                )
                if err != nil {
                        return nil, err
                }
                if lockedUntil.Valid {
                        user.LockedUntil = &lockedUntil.Time
                }
                if lastFailedAt.Valid {
                        user.LastFailedAt = &lastFailedAt.Time
                }
                return &user, nil
        })
}

// GetUsers 获取用户列表（分页、搜索、排序）
func (d *Database) GetUsers(page, limit int, search, sort, order string) ([]*User, int, error) {
        // 参数验证
        if limit > 100 {
                limit = 100
        }
        if page < 1 {
                page = 1
        }

        // 计算偏移量
        offset := (page - 1) * limit

        // 验证排序字段
        validSortFields := map[string]bool{
                "created_at": true,
                "email":      true,
        }
        if !validSortFields[sort] {
                sort = "created_at"
        }

        // 验证排序方向
        if order != "asc" && order != "desc" {
                order = "desc"
        }

        // 构建SQL查询
        var args []interface{}
        sql := `
                SELECT id, email, is_active, is_admin, otp_verified,
                       created_at, updated_at
                FROM users
        `

        // 添加搜索条件
        if search != "" {
                sql += " WHERE email LIKE ?"
                args = append(args, "%"+search+"%")
        }

        // 添加排序
        sql += fmt.Sprintf(" ORDER BY %s %s", sort, order)

        // 添加分页
        sql += " LIMIT ? OFFSET ?"
        args = append(args, limit, offset)

        // 执行查询
        rows, err := d.query(sql, args...)
        if err != nil {
                return nil, 0, fmt.Errorf("查询用户列表失败: %w", err)
        }
        defer rows.Close()

        // 处理结果
        var users []*User
        for rows.Next() {
                user := &User{}
                err := rows.Scan(
                        &user.ID,
                        &user.Email,
                        &user.IsActive,
                        &user.IsAdmin,
                        &user.OTPVerified,
                        &user.CreatedAt,
                        &user.UpdatedAt,
                )
                if err != nil {
                        return nil, 0, fmt.Errorf("扫描用户数据失败: %w", err)
                }
                users = append(users, user)
        }

        // 获取总数
        total, err := d.GetUserCount(search)
        if err != nil {
                return nil, 0, fmt.Errorf("获取用户总数失败: %w", err)
        }

        return users, total, nil
}

// GetUserCount 获取用户总数
func (d *Database) GetUserCount(search string) (int, error) {
        var count int
        sql := "SELECT COUNT(*) FROM users"

        // 添加搜索条件
        if search != "" {
                sql += " WHERE email LIKE ?"
                row := d.queryRow(sql, "%"+search+"%")
                err := row.Scan(&count)
                if err != nil {
                        return 0, fmt.Errorf("获取用户总数失败: %w", err)
                }
        } else {
                row := d.queryRow(sql)
                err := row.Scan(&count)
                if err != nil {
                        return 0, fmt.Errorf("获取用户总数失败: %w", err)
                }
        }

        return count, nil
}

// GetAllUsers 获取所有用户ID列表
func (d *Database) GetAllUsers() ([]string, error) {
        rows, err := d.query(`SELECT id FROM users ORDER BY id`)
        if err != nil {
                return nil, err
        }
        defer rows.Close()

        var userIDs []string
        for rows.Next() {
                var userID string
                if err := rows.Scan(&userID); err != nil {
                        return nil, err
                }
                userIDs = append(userIDs, userID)
        }
        return userIDs, nil
}

// UpdateUserOTPVerified 更新用户OTP验证状态
func (d *Database) UpdateUserOTPVerified(userID string, verified bool) error {
        _, err := d.exec(`UPDATE users SET otp_verified = ? WHERE id = ?`, verified, userID)
        return err
}

// GetAIModels 获取用户的AI模型配置
func (d *Database) GetAIModels(userID string) ([]*AIModelConfig, error) {
        return withRetry(func() ([]*AIModelConfig, error) {
                rows, err := d.query(`
                        SELECT id, user_id, name, provider, enabled, api_key,
                               COALESCE(custom_api_url, '') as custom_api_url,
                               COALESCE(custom_model_name, '') as custom_model_name,
                               created_at, updated_at
                        FROM ai_models WHERE user_id = ? ORDER BY id
                `, userID)
                if err != nil {
                        return nil, err
                }
                defer rows.Close()

                // 初始化为空切片而不是nil，确保JSON序列化为[]而不是null
                models := make([]*AIModelConfig, 0)
                for rows.Next() {
                        var model AIModelConfig
                        err := rows.Scan(
                                &model.ID, &model.UserID, &model.Name, &model.Provider,
                                &model.Enabled, &model.APIKey, &model.CustomAPIURL, &model.CustomModelName,
                                &model.CreatedAt, &model.UpdatedAt,
                        )
                        if err != nil {
                                return nil, err
                        }
                        models = append(models, &model)
                }

                return models, nil
        })
}

// UpdateAIModel 更新AI模型配置，如果不存在则创建用户特定配置
func (d *Database) UpdateAIModel(userID, id string, enabled bool, apiKey, customAPIURL, customModelName string) error {
        // 先尝试精确匹配 ID（新版逻辑，支持多个相同 provider 的模型）
        var existingID string
        err := d.queryRow(`
                SELECT id FROM ai_models WHERE user_id = $1 AND id = $2 LIMIT 1
        `, userID, id).Scan(&existingID)

        if err == nil {
                // 找到了现有配置（精确匹配 ID），更新它
                _, err = d.exec(`
                        UPDATE ai_models SET enabled = $1, api_key = $2, custom_api_url = $3, custom_model_name = $4, updated_at = CURRENT_TIMESTAMP
                        WHERE id = $5 AND user_id = $6
                `, enabled, apiKey, customAPIURL, customModelName, existingID, userID)
                return err
        }

        // ID 不存在，尝试兼容旧逻辑：将 id 作为 provider 查找
        provider := id
        err = d.queryRow(`
                SELECT id FROM ai_models WHERE user_id = $1 AND provider = $2 LIMIT 1
        `, userID, provider).Scan(&existingID)

        if err == nil {
                // 找到了现有配置（通过 provider 匹配，兼容旧版），更新它
                log.Printf("⚠️  使用旧版 provider 匹配更新模型: %s -> %s", provider, existingID)
                _, err = d.exec(`
                        UPDATE ai_models SET enabled = $1, api_key = $2, custom_api_url = $3, custom_model_name = $4, updated_at = CURRENT_TIMESTAMP
                        WHERE id = $5 AND user_id = $6
                `, enabled, apiKey, customAPIURL, customModelName, existingID, userID)
                return err
        }

        // 没有找到任何现有配置，创建新的
        // 推断 provider（从 id 中提取，或者直接使用 id）
        if provider == id && (provider == "deepseek" || provider == "qwen") {
                // id 本身就是 provider
                provider = id
        } else {
                // 从 id 中提取 provider（假设格式是 userID_provider 或 timestamp_userID_provider）
                parts := strings.Split(id, "_")
                if len(parts) >= 2 {
                        provider = parts[len(parts)-1] // 取最后一部分作为 provider
                } else {
                        provider = id
                }
        }

        // 获取模型的基本信息
        var name string
        err = d.queryRow(`
                SELECT name FROM ai_models WHERE provider = $1 LIMIT 1
        `, provider).Scan(&name)
        if err != nil {
                // 如果找不到基本信息，使用默认值
                if provider == "deepseek" {
                        name = "DeepSeek AI"
                } else if provider == "qwen" {
                        name = "Qwen AI"
                } else {
                        name = provider + " AI"
                }
        }

        // 如果传入的 ID 已经是完整格式（如 "admin_deepseek_custom1"），直接使用
        // 否则生成新的 ID
        newModelID := id
        if id == provider {
                // id 就是 provider，生成新的用户特定 ID
                newModelID = fmt.Sprintf("%s_%s", userID, provider)
        }

        log.Printf("✓ 创建新的 AI 模型配置: ID=%s, Provider=%s, Name=%s", newModelID, provider, name)
        _, err = d.exec(`
                INSERT INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        `, newModelID, userID, name, provider, enabled, apiKey, customAPIURL, customModelName)

        return err
}

// GetExchanges 获取用户的交易所配置
func (d *Database) GetExchanges(userID string) ([]*ExchangeConfig, error) {
        return withRetry(func() ([]*ExchangeConfig, error) {
                rows, err := d.query(`
                        SELECT id, user_id, name, type, enabled, api_key, secret_key, testnet,
                               COALESCE(hyperliquid_wallet_addr, '') as hyperliquid_wallet_addr,
                               COALESCE(aster_user, '') as aster_user,
                               COALESCE(aster_signer, '') as aster_signer,
                               COALESCE(aster_private_key, '') as aster_private_key,
                               COALESCE(okx_passphrase, '') as okx_passphrase,
                               created_at, updated_at
                        FROM exchanges WHERE user_id = ? ORDER BY id
                `, userID)
                if err != nil {
                        return nil, err
                }
                defer rows.Close()

                // 初始化为空切片而不是nil，确保JSON序列化为[]而不是null
                exchanges := make([]*ExchangeConfig, 0)
                for rows.Next() {
                        var exchange ExchangeConfig
                        err := rows.Scan(
                                &exchange.ID, &exchange.UserID, &exchange.Name, &exchange.Type,
                                &exchange.Enabled, &exchange.APIKey, &exchange.SecretKey, &exchange.Testnet,
                                &exchange.HyperliquidWalletAddr, &exchange.AsterUser,
                                &exchange.AsterSigner, &exchange.AsterPrivateKey,
                                &exchange.OKXPassphrase,
                                &exchange.CreatedAt, &exchange.UpdatedAt,
                        )
                        if err != nil {
                                return nil, err
                        }
                        exchanges = append(exchanges, &exchange)
                }

                return exchanges, nil
        })
}

// UpdateExchange 更新交易所配置，如果不存在则创建用户特定配置
func (d *Database) UpdateExchange(userID, id string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey, okxPassphrase string) error {
        log.Printf("🔧 UpdateExchange: userID=%s, id=%s, enabled=%v", userID, id, enabled)

        // 首先尝试更新现有的用户配置
        result, err := d.exec(`
                UPDATE exchanges SET enabled = $1, api_key = $2, secret_key = $3, testnet = $4,
                       hyperliquid_wallet_addr = $5, aster_user = $6, aster_signer = $7, aster_private_key = $8, okx_passphrase = $9, updated_at = CURRENT_TIMESTAMP
                WHERE id = $10 AND user_id = $11
        `, enabled, apiKey, secretKey, testnet, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey, okxPassphrase, id, userID)
        if err != nil {
                log.Printf("❌ UpdateExchange: 更新失败: %v", err)
                return err
        }

        // 检查是否有行被更新
        rowsAffected, err := result.RowsAffected()
        if err != nil {
                log.Printf("❌ UpdateExchange: 获取影响行数失败: %v", err)
                return err
        }

        log.Printf("📊 UpdateExchange: 影响行数 = %d", rowsAffected)

        // 如果没有行被更新，说明用户没有这个交易所的配置，需要创建
        if rowsAffected == 0 {
                log.Printf("💡 UpdateExchange: 没有现有记录，创建新记录")

                // 根据交易所ID确定基本信息
                var name, typ string
                if id == "binance" {
                        name = "Binance Futures"
                        typ = "cex"
                } else if id == "hyperliquid" {
                        name = "Hyperliquid"
                        typ = "dex"
                } else if id == "aster" {
                        name = "Aster DEX"
                        typ = "dex"
                } else if id == "okx" {
                        name = "OKX Futures"
                        typ = "cex"
                } else {
                        name = id + " Exchange"
                        typ = "cex"
                }

                log.Printf("🆕 UpdateExchange: 创建新记录 ID=%s, name=%s, type=%s", id, name, typ)

                // 创建用户特定的配置，使用ON CONFLICT处理冲突
                _, err = d.exec(`
                        INSERT INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet,
                                               hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, okx_passphrase)
                        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
                        ON CONFLICT (id, user_id) DO UPDATE SET
                                enabled = EXCLUDED.enabled,
                                api_key = EXCLUDED.api_key,
                                secret_key = EXCLUDED.secret_key,
                                testnet = EXCLUDED.testnet,
                                hyperliquid_wallet_addr = EXCLUDED.hyperliquid_wallet_addr,
                                aster_user = EXCLUDED.aster_user,
                                aster_signer = EXCLUDED.aster_signer,
                                aster_private_key = EXCLUDED.aster_private_key,
                                okx_passphrase = EXCLUDED.okx_passphrase,
                                updated_at = CURRENT_TIMESTAMP
                `, id, userID, name, typ, enabled, apiKey, secretKey, testnet, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey, okxPassphrase)

                if err != nil {
                        log.Printf("❌ UpdateExchange: 创建记录失败: %v", err)
                } else {
                        log.Printf("✅ UpdateExchange: 创建记录成功")
                }
                return err
        }

        log.Printf("✅ UpdateExchange: 更新现有记录成功")
        return nil
}

// CreateAIModel 创建AI模型配置
func (d *Database) CreateAIModel(userID, id, name, provider string, enabled bool, apiKey, customAPIURL string) error {
        _, err := d.exec(`
                INSERT INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url)
                VALUES ($1, $2, $3, $4, $5, $6, $7)
                ON CONFLICT (id) DO NOTHING
        `, id, userID, name, provider, enabled, apiKey, customAPIURL)
        return err
}

// CreateExchange 创建交易所配置
func (d *Database) CreateExchange(userID, id, name, typ string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) error {
        _, err := d.exec(`
                INSERT INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet, hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key)
                VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
                ON CONFLICT (id) DO NOTHING
        `, id, userID, name, typ, enabled, apiKey, secretKey, testnet, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey)
        return err
}

// CreateTrader 创建交易员
func (d *Database) CreateTrader(trader *TraderRecord) error {
        _, err := d.exec(`
                INSERT INTO traders (id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running, btc_eth_leverage, altcoin_leverage, trading_symbols, use_coin_pool, use_oi_top, custom_prompt, override_base_prompt, system_prompt_template, is_cross_margin)
                VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
        `, trader.ID, trader.UserID, trader.Name, trader.AIModelID, trader.ExchangeID, trader.InitialBalance, trader.ScanIntervalMinutes, trader.IsRunning, trader.BTCETHLeverage, trader.AltcoinLeverage, trader.TradingSymbols, trader.UseCoinPool, trader.UseOITop, trader.CustomPrompt, trader.OverrideBasePrompt, trader.SystemPromptTemplate, trader.IsCrossMargin)
        return err
}

// GetTraders 获取用户的交易员
func (d *Database) GetTraders(userID string) ([]*TraderRecord, error) {
        return withRetry(func() ([]*TraderRecord, error) {
                rows, err := d.query(`
                        SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
                               COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
                               COALESCE(trading_symbols, '') as trading_symbols,
                               COALESCE(use_coin_pool, false) as use_coin_pool, COALESCE(use_oi_top, false) as use_oi_top,
                               COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, false) as override_base_prompt,
                               COALESCE(system_prompt_template, 'default') as system_prompt_template,
                               COALESCE(is_cross_margin, true) as is_cross_margin, created_at, updated_at
                        FROM traders WHERE user_id = ? ORDER BY created_at DESC
                `, userID)
                if err != nil {
                        return nil, err
                }
                defer rows.Close()

                var traders []*TraderRecord
                for rows.Next() {
                        var trader TraderRecord
                        err := rows.Scan(
                                &trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
                                &trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
                                &trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
                                &trader.UseCoinPool, &trader.UseOITop,
                                &trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
                                &trader.IsCrossMargin,
                                &trader.CreatedAt, &trader.UpdatedAt,
                        )
                        if err != nil {
                                return nil, err
                        }
                        traders = append(traders, &trader)
                }

                return traders, nil
        })
}

// UpdateTraderStatus 更新交易员状态
func (d *Database) UpdateTraderStatus(userID, id string, isRunning bool) error {
        _, err := d.exec(`UPDATE traders SET is_running = ? WHERE id = ? AND user_id = ?`, isRunning, id, userID)
        return err
}

// UpdateTrader 更新交易员配置
func (d *Database) UpdateTrader(trader *TraderRecord) error {
        _, err := d.exec(`
                UPDATE traders SET
                        name = ?, ai_model_id = ?, exchange_id = ?, initial_balance = ?,
                        scan_interval_minutes = ?, btc_eth_leverage = ?, altcoin_leverage = ?,
                        trading_symbols = ?, custom_prompt = ?, override_base_prompt = ?,
                        system_prompt_template = ?, is_cross_margin = ?, updated_at = CURRENT_TIMESTAMP
                WHERE id = ? AND user_id = ?
        `, trader.Name, trader.AIModelID, trader.ExchangeID, trader.InitialBalance,
                trader.ScanIntervalMinutes, trader.BTCETHLeverage, trader.AltcoinLeverage,
                trader.TradingSymbols, trader.CustomPrompt, trader.OverrideBasePrompt,
                trader.SystemPromptTemplate, trader.IsCrossMargin, trader.ID, trader.UserID)
        return err
}

// UpdateTraderCustomPrompt 更新交易员自定义Prompt
func (d *Database) UpdateTraderCustomPrompt(userID, id string, customPrompt string, overrideBase bool) error {
        _, err := d.exec(`UPDATE traders SET custom_prompt = ?, override_base_prompt = ? WHERE id = ? AND user_id = ?`, customPrompt, overrideBase, id, userID)
        return err
}

// DeleteTrader 删除交易员
func (d *Database) DeleteTrader(userID, id string) error {
        _, err := d.exec(`DELETE FROM traders WHERE id = ? AND user_id = ?`, id, userID)
        return err
}

// traderConfigResult 用于 withRetry 的返回结构
type traderConfigResult struct {
        trader   *TraderRecord
        aiModel  *AIModelConfig
        exchange *ExchangeConfig
}

// GetTraderConfig 获取交易员完整配置（包含AI模型和交易所信息）
func (d *Database) GetTraderConfig(userID, traderID string) (*TraderRecord, *AIModelConfig, *ExchangeConfig, error) {
        result, err := withRetry(func() (*traderConfigResult, error) {
                var trader TraderRecord
                var aiModel AIModelConfig
                var exchange ExchangeConfig

                err := d.queryRow(`
                        SELECT 
                                t.id, t.user_id, t.name, t.ai_model_id, t.exchange_id, t.initial_balance, t.scan_interval_minutes, t.is_running, t.created_at, t.updated_at,
                                a.id, a.user_id, a.name, a.provider, a.enabled, a.api_key, a.created_at, a.updated_at,
                                e.id, e.user_id, e.name, e.type, e.enabled, e.api_key, e.secret_key, e.testnet,
                                COALESCE(e.hyperliquid_wallet_addr, '') as hyperliquid_wallet_addr,
                                COALESCE(e.aster_user, '') as aster_user,
                                COALESCE(e.aster_signer, '') as aster_signer,
                                COALESCE(e.aster_private_key, '') as aster_private_key,
                                e.created_at, e.updated_at
                        FROM traders t
                        JOIN ai_models a ON t.ai_model_id = a.id AND t.user_id = a.user_id
                        JOIN exchanges e ON t.exchange_id = e.id AND t.user_id = e.user_id
                        WHERE t.id = ? AND t.user_id = ?
                `, traderID, userID).Scan(
                        &trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
                        &trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
                        &trader.CreatedAt, &trader.UpdatedAt,
                        &aiModel.ID, &aiModel.UserID, &aiModel.Name, &aiModel.Provider, &aiModel.Enabled, &aiModel.APIKey,
                        &aiModel.CreatedAt, &aiModel.UpdatedAt,
                        &exchange.ID, &exchange.UserID, &exchange.Name, &exchange.Type, &exchange.Enabled,
                        &exchange.APIKey, &exchange.SecretKey, &exchange.Testnet,
                        &exchange.HyperliquidWalletAddr, &exchange.AsterUser, &exchange.AsterSigner, &exchange.AsterPrivateKey,
                        &exchange.CreatedAt, &exchange.UpdatedAt,
                )

                if err != nil {
                        return nil, err
                }

                return &traderConfigResult{
                        trader:   &trader,
                        aiModel:  &aiModel,
                        exchange: &exchange,
                }, nil
        })

        if err != nil {
                return nil, nil, nil, err
        }

        return result.trader, result.aiModel, result.exchange, nil
}

// GetSystemConfig 获取系统配置
func (d *Database) GetSystemConfig(key string) (string, error) {
        return withRetry(func() (string, error) {
                var value string
                err := d.queryRow(`SELECT value FROM system_config WHERE key = $1`, key).Scan(&value)
                if err != nil {
                        if err == sql.ErrNoRows {
                                // 如果 key 不存在，返回空字符串和 nil 错误
                                return "", nil
                        }
                        return "", err
                }
                return value, nil
        })
}

// SetSystemConfig 设置系统配置
func (d *Database) SetSystemConfig(key, value string) error {
        _, err := d.exec(`
                INSERT INTO system_config (key, value) VALUES ($1, $2)
                ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP
        `, key, value)
        return err
}

// CreateUserSignalSource 创建用户信号源配置
func (d *Database) CreateUserSignalSource(userID, coinPoolURL, oiTopURL string) error {
        _, err := d.exec(`
                INSERT INTO user_signal_sources (user_id, coin_pool_url, oi_top_url, updated_at)
                VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
                ON CONFLICT (user_id) DO UPDATE SET coin_pool_url = EXCLUDED.coin_pool_url, oi_top_url = EXCLUDED.oi_top_url, updated_at = CURRENT_TIMESTAMP
        `, userID, coinPoolURL, oiTopURL)
        return err
}

// GetUserSignalSource 获取用户信号源配置
func (d *Database) GetUserSignalSource(userID string) (*UserSignalSource, error) {
        var source UserSignalSource
        err := d.queryRow(`
                SELECT id, user_id, coin_pool_url, oi_top_url, created_at, updated_at
                FROM user_signal_sources WHERE user_id = $1
        `, userID).Scan(
                &source.ID, &source.UserID, &source.CoinPoolURL, &source.OITopURL,
                &source.CreatedAt, &source.UpdatedAt,
        )
        if err != nil {
                return nil, err
        }
        return &source, nil
}

// UpdateUserSignalSource 更新用户信号源配置
func (d *Database) UpdateUserSignalSource(userID, coinPoolURL, oiTopURL string) error {
        _, err := d.exec(`
                UPDATE user_signal_sources SET coin_pool_url = ?, oi_top_url = ?, updated_at = CURRENT_TIMESTAMP
                WHERE user_id = ?
        `, coinPoolURL, oiTopURL, userID)
        return err
}

// GetCustomCoins 获取所有交易员自定义币种 / Get all trader-customized currencies
func (d *Database) GetCustomCoins() []string {
        var symbol string
        var symbols []string
        _ = d.queryRow(`
                SELECT GROUP_CONCAT(custom_coins , ',') as symbol
                FROM main.traders where custom_coins != ''
        `).Scan(&symbol)
        // 检测用户是否未配置币种 - 兼容性
        if symbol == "" {
                symbolJSON, _ := d.GetSystemConfig("default_coins")
                if err := json.Unmarshal([]byte(symbolJSON), &symbols); err != nil {
                        log.Printf("⚠️  解析default_coins配置失败: %v，使用硬编码默认值", err)
                        symbols = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"}
                }
        }
        // filter Symbol
        for _, s := range strings.Split(symbol, ",") {
                if s == "" {
                        continue
                }
                coin := market.Normalize(s)
                if !slices.Contains(symbols, coin) {
                        symbols = append(symbols, coin)
                }
        }
        return symbols
}

// Close 关闭数据库连接
func (d *Database) Close() error {
        return d.db.Close()
}

// LoadBetaCodesFromFile 从文件加载内测码到数据库
func (d *Database) LoadBetaCodesFromFile(filePath string) error {
        // 读取文件内容
        content, err := os.ReadFile(filePath)
        if err != nil {
                return fmt.Errorf("读取内测码文件失败: %w", err)
        }

        // 按行分割内测码
        lines := strings.Split(string(content), "\n")
        var codes []string
        for _, line := range lines {
                code := strings.TrimSpace(line)
                if code != "" && !strings.HasPrefix(code, "#") {
                        codes = append(codes, code)
                }
        }

        // 批量插入内测码
        tx, err := d.db.Begin()
        if err != nil {
                return fmt.Errorf("开始事务失败: %w", err)
        }
        defer tx.Rollback()

        stmt, err := tx.Prepare(`INSERT INTO beta_codes (code) VALUES ($1) ON CONFLICT (code) DO NOTHING`)
        if err != nil {
                return fmt.Errorf("准备语句失败: %w", err)
        }
        defer stmt.Close()

        insertedCount := 0
        for _, code := range codes {
                result, err := stmt.Exec(code)
                if err != nil {
                        log.Printf("插入内测码 %s 失败: %v", code, err)
                        continue
                }
                
                if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
                        insertedCount++
                }
        }

        if err := tx.Commit(); err != nil {
                return fmt.Errorf("提交事务失败: %w", err)
        }

        log.Printf("✅ 成功加载 %d 个内测码到数据库 (总计 %d 个)", insertedCount, len(codes))
        return nil
}

// ValidateBetaCode 验证内测码是否有效且未使用
func (d *Database) ValidateBetaCode(code string) (bool, error) {
        var used bool
        err := d.queryRow(`SELECT used FROM beta_codes WHERE code = $1`, code).Scan(&used)
        if err != nil {
                if err == sql.ErrNoRows {
                        return false, nil // 内测码不存在
                }
                return false, err
        }
        return !used, nil // 内测码存在且未使用
}

// UseBetaCode 使用内测码（标记为已使用）
func (d *Database) UseBetaCode(code, userEmail string) error {
        result, err := d.exec(`
                UPDATE beta_codes SET used = 1, used_by = ?, used_at = CURRENT_TIMESTAMP 
                WHERE code = ? AND used = 0
        `, userEmail, code)
        if err != nil {
                return err
        }

        rowsAffected, err := result.RowsAffected()
        if err != nil {
                return err
        }

        if rowsAffected == 0 {
                return fmt.Errorf("内测码无效或已被使用")
        }

        return nil
}

// GetBetaCodeStats 获取内测码统计信息
func (d *Database) GetBetaCodeStats() (total, used int, err error) {
        err = d.queryRow(`SELECT COUNT(*) FROM beta_codes`).Scan(&total)
        if err != nil {
                return 0, 0, err
        }

        err = d.queryRow(`SELECT COUNT(*) FROM beta_codes WHERE used = 1`).Scan(&used)
        if err != nil {
                return 0, 0, err
        }

        return total, used, nil
}

// UpdateUserPassword 更新用户密码
func (d *Database) UpdateUserPassword(userID, newPasswordHash string) error {
        _, err := d.exec(`
                UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP
                WHERE id = ?
        `, newPasswordHash, userID)
        return err
}

// UpdateUserLockoutStatus 更新用户锁定状态
func (d *Database) UpdateUserLockoutStatus(userID string, failedAttempts int, lockedUntil *time.Time) error {
        _, err := d.exec(`
                UPDATE users SET failed_attempts = ?, locked_until = ?, last_failed_at = CURRENT_TIMESTAMP
                WHERE id = ?
        `, failedAttempts, lockedUntil, userID)
        return err
}

// ResetUserFailedAttempts 重置用户失败尝试次数
func (d *Database) ResetUserFailedAttempts(userID string) error {
        _, err := d.exec(`
                UPDATE users SET failed_attempts = 0, locked_until = NULL, last_failed_at = NULL
                WHERE id = ?
        `, userID)
        return err
}

// RecordLoginAttempt 记录登录尝试
func (d *Database) RecordLoginAttempt(userID *string, email, ipAddress, userAgent string, success bool) error {
        _, err := d.exec(`
                INSERT INTO login_attempts (id, user_id, email, ip_address, success, user_agent)
                VALUES (?, ?, ?, ?, ?, ?)
        `, GenerateUUID(), userID, email, ipAddress, success, userAgent)
        return err
}

// GetLoginAttemptsByIP 获取IP在过去15分钟内的失败尝试次数
func (d *Database) GetLoginAttemptsByIP(ipAddress string) (int, error) {
        var count int
        err := d.queryRow(`
                SELECT COUNT(*) FROM login_attempts
                WHERE ip_address = $1 AND success = 0 AND timestamp > CURRENT_TIMESTAMP - INTERVAL '15 minutes'
        `, ipAddress).Scan(&count)
        return count, err
}

// GetLoginAttemptsByEmail 获取邮箱在过去15分钟内的失败尝试次数
func (d *Database) GetLoginAttemptsByEmail(email string) (int, error) {
        var count int
        err := d.queryRow(`
                SELECT COUNT(*) FROM login_attempts
                WHERE email = $1 AND success = 0 AND timestamp > CURRENT_TIMESTAMP - INTERVAL '15 minutes'
        `, email).Scan(&count)
        return count, err
}

// CreatePasswordResetToken 创建密码重置令牌
func (d *Database) CreatePasswordResetToken(userID, token, tokenHash string, expiresAt time.Time) error {
        _, err := d.exec(`
                INSERT INTO password_resets (id, user_id, token_hash, expires_at)
                VALUES (?, ?, ?, ?)
        `, token, userID, tokenHash, expiresAt)
        return err
}

// ValidatePasswordResetToken 验证密码重置令牌
func (d *Database) ValidatePasswordResetToken(tokenHash string) (*string, error) {
        var userID string
        var expiresAt time.Time
        err := d.queryRow(`
                SELECT user_id, expires_at FROM password_resets
                WHERE token_hash = ? AND used_at IS NULL AND expires_at > CURRENT_TIMESTAMP
        `, tokenHash).Scan(&userID, &expiresAt)
        if err != nil {
                return nil, err
        }
        return &userID, nil
}

// MarkPasswordResetTokenAsUsed 标记密码重置令牌为已使用
func (d *Database) MarkPasswordResetTokenAsUsed(tokenHash string) error {
        _, err := d.exec(`
                UPDATE password_resets SET used_at = CURRENT_TIMESTAMP
                WHERE token_hash = ?
        `, tokenHash)
        return err
}

// InvalidateAllPasswordResetTokens 使用户的所有密码重置令牌失效
func (d *Database) InvalidateAllPasswordResetTokens(userID string) error {
        _, err := d.exec(`
                UPDATE password_resets SET used_at = CURRENT_TIMESTAMP
                WHERE user_id = ? AND used_at IS NULL
        `, userID)
        return err
}

// CreateAuditLog 创建审计日志
func (d *Database) CreateAuditLog(userID *string, action, ipAddress, userAgent string, success bool, details string) error {
        _, err := d.exec(`
                INSERT INTO audit_logs (id, user_id, action, ip_address, user_agent, success, details)
                VALUES (?, ?, ?, ?, ?, ?, ?)
        `, GenerateUUID(), userID, action, ipAddress, userAgent, success, details)
        return err
}

// GetAuditLogs 获取用户的审计日志
func (d *Database) GetAuditLogs(userID string, limit int) ([]map[string]interface{}, error) {
        rows, err := d.query(`
                SELECT action, ip_address, success, details, created_at
                FROM audit_logs
                WHERE user_id = ?
                ORDER BY created_at DESC
                LIMIT ?
        `, userID, limit)
        if err != nil {
                return nil, err
        }
        defer rows.Close()

        var logs []map[string]interface{}
        for rows.Next() {
                var action, ipAddress, details, createdAt string
                var success bool
                if err := rows.Scan(&action, &ipAddress, &success, &details, &createdAt); err != nil {
                        return nil, err
                }
                log := map[string]interface{}{
                        "action":      action,
                        "ip_address":  ipAddress,
                        "success":     success,
                        "details":     details,
                        "created_at":  createdAt,
                }
                logs = append(logs, log)
        }

        return logs, nil
}

// GenerateUUID 生成UUID
func GenerateUUID() string {
        return strings.Replace(uuid.New().String(), "-", "", -1)
}

// MigrateUserBetaCodes 回填用户的 beta_code 字段
// 从 beta_codes 表的 used_by 字段获取用户邮箱，然后更新到用户表的 beta_code 字段
func (d *Database) MigrateUserBetaCodes() (int, error) {
        // 查询已使用的内测码及其用户邮箱
        rows, err := d.query(`
                SELECT DISTINCT bc.code, bc.used_by
                FROM beta_codes bc
                WHERE bc.used = 1 AND bc.used_by IS NOT NULL AND bc.used_by != ''
        `)
        if err != nil {
                return 0, fmt.Errorf("查询内测码失败: %w", err)
        }
        defer rows.Close()

        updatedCount := 0
        for rows.Next() {
                var code, usedBy string
                if err := rows.Scan(&code, &usedBy); err != nil {
                        log.Printf("⚠️ 扫描内测码记录失败: %v", err)
                        continue
                }

                // 更新用户表的 beta_code 字段
                result, err := d.exec(`
                        UPDATE users
                        SET beta_code = ?
                        WHERE email = ? AND beta_code IS NULL
                `, code, usedBy)
                if err != nil {
                        log.Printf("⚠️ 更新用户 %s 的 beta_code 失败: %v", usedBy, err)
                        continue
                }

                rowsAffected, _ := result.RowsAffected()
                if rowsAffected > 0 {
                        updatedCount++
                        log.Printf("✅ 已为用户 %s 关联内测码 %s", usedBy, code)
                }
        }

        return updatedCount, nil
}

// GetUserBetaCode 获取用户关联的内测码
func (d *Database) GetUserBetaCode(userID string) (string, error) {
        var betaCode sql.NullString
        err := d.queryRow(`
                SELECT beta_code FROM users WHERE id = $1
        `, userID).Scan(&betaCode)
        if err != nil {
                return "", err
        }
        if !betaCode.Valid {
                return "", nil
        }
        return betaCode.String, nil
}
