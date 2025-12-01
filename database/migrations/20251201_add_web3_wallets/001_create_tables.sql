-- ============================================================
-- Web3钱包支持 - 数据库迁移
-- 版本: 2025-12-01
-- 修复版本: 关键安全漏洞已修复
-- ============================================================

-- 创建web3_wallets表
CREATE TABLE IF NOT EXISTS web3_wallets (
    id TEXT PRIMARY KEY,
    wallet_addr TEXT UNIQUE NOT NULL,
    chain_id INTEGER NOT NULL DEFAULT 1,
    wallet_type TEXT NOT NULL,
    label TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT chk_wallet_addr CHECK (wallet_addr ~ '^0x[a-fA-F0-9]{40}$'),
    CONSTRAINT chk_chain_id CHECK (chain_id > 0),
    CONSTRAINT chk_wallet_type CHECK (wallet_type IN ('metamask', 'tp', 'other'))
);

-- 创建user_wallets关联表
CREATE TABLE IF NOT EXISTS user_wallets (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    wallet_addr TEXT NOT NULL,
    is_primary BOOLEAN DEFAULT FALSE,
    bound_at TIMESTAMPTZ DEFAULT NOW(),
    last_used_at TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (wallet_addr) REFERENCES web3_wallets(wallet_addr) ON DELETE CASCADE,
    UNIQUE(user_id, wallet_addr),
    CONSTRAINT chk_is_primary CHECK (
        CASE
            WHEN is_primary = TRUE THEN
                NOT EXISTS (
                    SELECT 1 FROM user_wallets uw2
                    WHERE uw2.user_id = user_wallets.user_id
                    AND uw2.is_primary = TRUE
                    AND uw2.wallet_addr != user_wallets.wallet_addr
                )
            ELSE TRUE
        END
    )
);

-- ============ 修复CVE-WS-002: 添加nonce存储表 ============
CREATE TABLE IF NOT EXISTS web3_wallet_nonces (
    id TEXT PRIMARY KEY,
    address TEXT NOT NULL,
    nonce TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT chk_nonce_address CHECK (address ~ '^0x[a-fA-F0-9]{40}$')
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_web3_wallets_addr ON web3_wallets(wallet_addr);
CREATE INDEX IF NOT EXISTS idx_web3_wallets_type ON web3_wallets(wallet_type);
CREATE INDEX IF NOT EXISTS idx_user_wallets_user_id ON user_wallets(user_id);
CREATE INDEX IF NOT EXISTS idx_user_wallets_primary ON user_wallets(user_id, is_primary);

-- ============ 添加nonce相关索引 ============
CREATE INDEX IF NOT EXISTS idx_nonces_address ON web3_wallet_nonces(address);
CREATE INDEX IF NOT EXISTS idx_nonces_expires ON web3_wallet_nonces(expires_at) WHERE NOT used;
CREATE INDEX IF NOT EXISTS idx_nonces_used ON web3_wallet_nonces(used, expires_at);

-- 创建触发器
DROP TRIGGER IF EXISTS update_web3_wallets_updated_at ON web3_wallets;

CREATE TRIGGER update_web3_wallets_updated_at
    BEFORE UPDATE ON web3_wallets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 清理过期nonce的定期任务函数
CREATE OR REPLACE FUNCTION cleanup_expired_nonces()
RETURNS void AS $$
BEGIN
    DELETE FROM web3_wallet_nonces
    WHERE expires_at < NOW() - INTERVAL '1 hour';
END;
$$ LANGUAGE plpgsql;

-- 插入默认系统配置
INSERT INTO system_config (key, value) VALUES
    ('web3.supported_wallet_types', '["metamask", "tp", "other"]'),
    ('web3.max_wallets_per_user', '10'),
    ('web3.nonce_expiry_minutes', '10'),
    ('web3.rate_limit_per_ip', '10'),
    ('web3.rate_limit_window_minutes', '10')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

-- 验证迁移
DO $$
DECLARE
    table_count INTEGER;
BEGIN
    -- 验证表是否存在
    SELECT COUNT(*) INTO table_count
    FROM information_schema.tables
    WHERE table_name IN ('web3_wallets', 'user_wallets', 'web3_wallet_nonces');

    IF table_count != 3 THEN
        RAISE EXCEPTION '迁移失败：预期3个表，实际创建了%个表', table_count;
    END IF;

    -- 验证nonce表结构
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'web3_wallet_nonces'
        AND column_name = 'used'
    ) THEN
        RAISE EXCEPTION '迁移失败：nonce表缺少used列';
    END IF;

    RAISE NOTICE '✅ Web3钱包表创建成功，包括nonce存储表';
END $$;
