-- PostgreSQL初始化脚本
-- AI交易系统数据库迁移

-- 用户表
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    otp_secret TEXT,
    otp_verified BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- AI模型配置表
CREATE TABLE IF NOT EXISTS ai_models (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL DEFAULT 'default',
    name TEXT NOT NULL,
    provider TEXT NOT NULL,
    enabled BOOLEAN DEFAULT FALSE,
    api_key TEXT DEFAULT '',
    custom_api_url TEXT DEFAULT '',
    custom_model_name TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 交易所配置表
CREATE TABLE IF NOT EXISTS exchanges (
    id TEXT NOT NULL,
    user_id TEXT NOT NULL DEFAULT 'default',
    name TEXT NOT NULL,
    type TEXT NOT NULL, -- 'cex' or 'dex'
    enabled BOOLEAN DEFAULT FALSE,
    api_key TEXT DEFAULT '',
    secret_key TEXT DEFAULT '',
    testnet BOOLEAN DEFAULT FALSE,
    -- Hyperliquid 特定字段
    hyperliquid_wallet_addr TEXT DEFAULT '',
    -- Aster 特定字段
    aster_user TEXT DEFAULT '',
    aster_signer TEXT DEFAULT '',
    aster_private_key TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, user_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 用户信号源配置表
CREATE TABLE IF NOT EXISTS user_signal_sources (
    id SERIAL PRIMARY KEY,
    user_id TEXT NOT NULL,
    coin_pool_url TEXT DEFAULT '',
    oi_top_url TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(user_id)
);

-- 交易员配置表
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
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (ai_model_id) REFERENCES ai_models(id),
    FOREIGN KEY (exchange_id, user_id) REFERENCES exchanges(id, user_id)
);

-- 系统配置表
CREATE TABLE IF NOT EXISTS system_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 内测码表
CREATE TABLE IF NOT EXISTS beta_codes (
    code TEXT PRIMARY KEY,
    used BOOLEAN DEFAULT FALSE,
    used_by TEXT DEFAULT '',
    used_at TIMESTAMP DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 自动更新 updated_at 函数
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 创建触发器：自动更新 updated_at
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_ai_models_updated_at BEFORE UPDATE ON ai_models
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_exchanges_updated_at BEFORE UPDATE ON exchanges
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_traders_updated_at BEFORE UPDATE ON traders
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_signal_sources_updated_at BEFORE UPDATE ON user_signal_sources
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_system_config_updated_at BEFORE UPDATE ON system_config
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 插入默认数据

-- 初始化AI模型（使用default用户）
INSERT INTO ai_models (id, user_id, name, provider, enabled) VALUES
('deepseek', 'default', 'DeepSeek', 'deepseek', FALSE),
('qwen', 'default', 'Qwen', 'qwen', FALSE)
ON CONFLICT (id) DO NOTHING;

-- 初始化交易所（使用default用户）
INSERT INTO exchanges (id, user_id, name, type, enabled) VALUES
('binance', 'default', 'Binance Futures', 'binance', FALSE),
('hyperliquid', 'default', 'Hyperliquid', 'hyperliquid', FALSE),
('aster', 'default', 'Aster DEX', 'aster', FALSE)
ON CONFLICT (id, user_id) DO NOTHING;

-- 初始化系统配置
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
ON CONFLICT (key) DO NOTHING;

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_ai_models_user_id ON ai_models(user_id);
CREATE INDEX IF NOT EXISTS idx_exchanges_user_id ON exchanges(user_id);
CREATE INDEX IF NOT EXISTS idx_traders_user_id ON traders(user_id);
CREATE INDEX IF NOT EXISTS idx_traders_running ON traders(is_running);
CREATE INDEX IF NOT EXISTS idx_beta_codes_used ON beta_codes(used);