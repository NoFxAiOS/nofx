-- ============================================================
-- Monnaire Trading Agent OS - 数据库迁移脚本
-- 版本: 2025-11-17
-- 兼容性: SQLite3 -> Neon.tech (PostgreSQL)
-- ============================================================

-- 重要说明：
-- 本脚本适用于从SQLite迁移到PostgreSQL (Neon.tech)
-- 执行前请备份现有数据
-- ============================================================

-- ============================================================
-- 第1部分：创建表结构
-- ============================================================

-- 1.1 创建用户表
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    otp_secret TEXT,
    otp_verified BOOLEAN DEFAULT FALSE,
    locked_until TIMESTAMPTZ,
    failed_attempts INTEGER DEFAULT 0,
    last_failed_at TIMESTAMPTZ,
    is_active BOOLEAN DEFAULT TRUE,
    is_admin BOOLEAN DEFAULT FALSE,
    beta_code TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 1.2 创建AI模型配置表
CREATE TABLE IF NOT EXISTS ai_models (
    id TEXT NOT NULL,
    user_id TEXT NOT NULL DEFAULT 'default',
    name TEXT NOT NULL,
    provider TEXT NOT NULL,
    enabled BOOLEAN DEFAULT FALSE,
    api_key TEXT DEFAULT '',
    custom_api_url TEXT DEFAULT '',
    custom_model_name TEXT DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (id, user_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 1.3 创建交易所配置表 (使用复合主键支持多用户)
CREATE TABLE IF NOT EXISTS exchanges (
    id TEXT NOT NULL,
    user_id TEXT NOT NULL DEFAULT 'default',
    name TEXT NOT NULL,
    type TEXT NOT NULL, -- 'cex' or 'dex'
    enabled BOOLEAN DEFAULT FALSE,
    api_key TEXT DEFAULT '',
    secret_key TEXT DEFAULT '',
    testnet BOOLEAN DEFAULT FALSE,
    hyperliquid_wallet_addr TEXT DEFAULT '',
    aster_user TEXT DEFAULT '',
    aster_signer TEXT DEFAULT '',
    aster_private_key TEXT DEFAULT '',
    okx_passphrase TEXT DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (id, user_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 1.4 创建交易员配置表
CREATE TABLE IF NOT EXISTS traders (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL DEFAULT 'default',
    name TEXT NOT NULL,
    ai_model_id TEXT NOT NULL,
    exchange_id TEXT NOT NULL,
    initial_balance REAL NOT NULL,
    scan_interval_minutes INTEGER DEFAULT 3,
    is_running BOOLEAN DEFAULT FALSE,
    btc_eth_leverage INTEGER DEFAULT 5,
    altcoin_leverage INTEGER DEFAULT 5,
    trading_symbols TEXT DEFAULT '',
    use_coin_pool BOOLEAN DEFAULT FALSE,
    use_oi_top BOOLEAN DEFAULT FALSE,
    custom_prompt TEXT DEFAULT '',
    override_base_prompt BOOLEAN DEFAULT FALSE,
    system_prompt_template TEXT DEFAULT 'default',
    is_cross_margin BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (ai_model_id) REFERENCES ai_models(id),
    FOREIGN KEY (exchange_id) REFERENCES exchanges(id)
);

-- 1.5 创建用户信号源配置表
CREATE TABLE IF NOT EXISTS user_signal_sources (
    id SERIAL PRIMARY KEY,
    user_id TEXT NOT NULL,
    coin_pool_url TEXT DEFAULT '',
    oi_top_url TEXT DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(user_id)
);

-- 1.6 创建密码重置令牌表
CREATE TABLE IF NOT EXISTS password_resets (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 1.7 创建登录尝试记录表
CREATE TABLE IF NOT EXISTS login_attempts (
    id TEXT PRIMARY KEY,
    user_id TEXT,
    email TEXT NOT NULL,
    ip_address TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    timestamp TIMESTAMPTZ DEFAULT NOW(),
    user_agent TEXT,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

-- 1.8 创建审计日志表
CREATE TABLE IF NOT EXISTS audit_logs (
    id TEXT PRIMARY KEY,
    user_id TEXT,
    action TEXT NOT NULL,
    ip_address TEXT NOT NULL,
    user_agent TEXT,
    success BOOLEAN NOT NULL,
    details TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

-- 1.9 创建系统配置表
CREATE TABLE IF NOT EXISTS system_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 1.10 创建用户新闻源配置表
CREATE TABLE IF NOT EXISTS user_news_config (
    id SERIAL PRIMARY KEY,
    user_id TEXT NOT NULL UNIQUE,
    enabled BOOLEAN DEFAULT TRUE,
    news_sources TEXT DEFAULT 'mlion',  -- 逗号分隔的新闻源列表
    auto_fetch_interval_minutes INTEGER DEFAULT 5,
    max_articles_per_fetch INTEGER DEFAULT 10,
    sentiment_threshold REAL DEFAULT 0.0,  -- 情绪阈值用于过滤
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 内测码表
CREATE TABLE IF NOT EXISTS beta_codes (
    code TEXT PRIMARY KEY,
    used BOOLEAN DEFAULT FALSE,
    used_by TEXT DEFAULT '',
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================================
-- 第2部分：创建触发器 (PostgreSQL版本)
-- ============================================================

-- 删除旧触发器 (如果存在)
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP TRIGGER IF EXISTS update_ai_models_updated_at ON ai_models;
DROP TRIGGER IF EXISTS update_exchanges_updated_at ON exchanges;
DROP TRIGGER IF EXISTS update_traders_updated_at ON traders;
DROP TRIGGER IF EXISTS update_user_signal_sources_updated_at ON user_signal_sources;
DROP TRIGGER IF EXISTS update_user_news_config_updated_at ON user_news_config;
DROP TRIGGER IF EXISTS update_system_config_updated_at ON system_config;
DROP TRIGGER IF EXISTS update_beta_codes_updated_at ON beta_codes;

-- 创建触发器函数
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 应用触发器到各个表
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_ai_models_updated_at
    BEFORE UPDATE ON ai_models
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_exchanges_updated_at
    BEFORE UPDATE ON exchanges
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_traders_updated_at
    BEFORE UPDATE ON traders
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_signal_sources_updated_at
    BEFORE UPDATE ON user_signal_sources
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_news_config_updated_at
    BEFORE UPDATE ON user_news_config
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_system_config_updated_at
    BEFORE UPDATE ON system_config
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================
-- 第3部分：插入默认AI模型配置
-- ============================================================

INSERT INTO ai_models (id, user_id, name, provider, enabled)
VALUES
    ('deepseek', 'default', 'DeepSeek', 'deepseek', FALSE),
    ('qwen', 'default', 'Qwen', 'qwen', FALSE)
ON CONFLICT (id, user_id) DO UPDATE SET
    name = EXCLUDED.name,
    provider = EXCLUDED.provider;

-- ============================================================
-- 第4部分：插入默认交易所配置
-- ============================================================

INSERT INTO exchanges (id, user_id, name, type, enabled)
VALUES
    ('binance', 'default', 'Binance Futures', 'cex', FALSE),
    ('hyperliquid', 'default', 'Hyperliquid', 'dex', FALSE),
    ('aster', 'default', 'Aster DEX', 'dex', FALSE),
    ('okx', 'default', 'OKX Futures', 'cex', FALSE)
ON CONFLICT (id, user_id) DO UPDATE SET
    name = EXCLUDED.name,
    type = EXCLUDED.type;

-- ============================================================
-- 第5部分：插入默认系统配置
-- ============================================================

INSERT INTO system_config (key, value) VALUES
    ('admin_mode', 'true'),
    ('beta_mode', 'false'),
    ('api_server_port', '8080'),
    ('use_default_coins', 'true'),
    ('default_coins', '["BTCUSDT","ETHUSDT","SOLUSDT","BNBUSDT","XRPUSDT","DOGEUSDT","ADAUSDT","HYPEUSDT"]'),
    ('max_daily_loss', '10.0'),
    ('max_drawdown', '20.0'),
    ('stop_trading_minutes', '60'),
    ('btc_eth_leverage', '5'),
    ('altcoin_leverage', '5'),
    ('jwt_secret', '')
ON CONFLICT (key) DO UPDATE SET
    value = EXCLUDED.value;

-- ============================================================
-- 第6部分：创建索引 (优化性能)
-- ============================================================

-- 用户表索引
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_is_active ON users(is_active);

-- AI模型索引
CREATE INDEX IF NOT EXISTS idx_ai_models_user_id ON ai_models(user_id);
CREATE INDEX IF NOT EXISTS idx_ai_models_enabled ON ai_models(enabled);

-- 交易所索引
CREATE INDEX IF NOT EXISTS idx_exchanges_user_id ON exchanges(user_id);
CREATE INDEX IF NOT EXISTS idx_exchanges_enabled ON exchanges(enabled);

-- 交易员索引
CREATE INDEX IF NOT EXISTS idx_traders_user_id ON traders(user_id);
CREATE INDEX IF NOT EXISTS idx_traders_is_running ON traders(is_running);
CREATE INDEX IF NOT EXISTS idx_traders_exchange_id ON traders(exchange_id);

-- 登录尝试索引
CREATE INDEX IF NOT EXISTS idx_login_attempts_email ON login_attempts(email);
CREATE INDEX IF NOT EXISTS idx_login_attempts_timestamp ON login_attempts(timestamp);

-- 审计日志索引
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);

-- 用户新闻源配置索引
CREATE INDEX IF NOT EXISTS idx_user_news_config_user_id ON user_news_config(user_id);
CREATE INDEX IF NOT EXISTS idx_user_news_config_enabled ON user_news_config(enabled);

-- ============================================================
-- 第7部分：验证数据完整性
-- ============================================================

-- 验证AI模型数量
DO $$
DECLARE
    model_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO model_count FROM ai_models WHERE user_id = 'default';
    IF model_count < 2 THEN
        RAISE EXCEPTION '警告：AI模型数量不足，当前为%，至少需要2个', model_count;
    END IF;
    RAISE NOTICE '验证通过：AI模型数量为%', model_count;
END $$;

-- 验证交易所数量
DO $$
DECLARE
    exchange_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO exchange_count FROM exchanges WHERE user_id = 'default';
    IF exchange_count < 4 THEN
        RAISE EXCEPTION '警告：交易所数量不足，当前为%，至少需要4个', exchange_count;
    END IF;
    RAISE NOTICE '验证通过：交易所数量为%', exchange_count;
END $$;

-- 验证系统配置
DO $$
DECLARE
    config_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO config_count FROM system_config;
    IF config_count < 10 THEN
        RAISE EXCEPTION '警告：系统配置不足，当前为%，至少需要10个', config_count;
    END IF;
    RAISE NOTICE '验证通过：系统配置数量为%', config_count;
END $$;

-- ============================================================
-- 第8部分：显示创建结果
-- ============================================================

-- 显示所有表
SELECT '创建的表:' as info;
SELECT table_name
FROM information_schema.tables
WHERE table_schema = 'public'
ORDER BY table_name;

-- 显示AI模型
SELECT '默认AI模型:' as info;
SELECT id, name, provider FROM ai_models WHERE user_id = 'default' ORDER BY id;

-- 显示交易所
SELECT '默认交易所:' as info;
SELECT id, name, type FROM exchanges WHERE user_id = 'default' ORDER BY id;

-- 显示系统配置
SELECT '系统配置项数:' as info;
SELECT COUNT(*) as config_count FROM system_config;

-- ============================================================
-- 脚本执行完成
-- ============================================================
