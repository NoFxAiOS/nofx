-- ============================================================
-- Web3钱包支持 - 回滚脚本
-- ============================================================

-- 删除触发器
DROP TRIGGER IF EXISTS update_web3_wallets_updated_at ON web3_wallets;

-- 删除表（按依赖顺序）
DROP TABLE IF EXISTS user_wallets CASCADE;
DROP TABLE IF EXISTS web3_wallets CASCADE;
DROP TABLE IF EXISTS web3_wallet_nonces CASCADE;

-- 删除函数
DROP FUNCTION IF EXISTS cleanup_expired_nonces() CASCADE;

-- 删除系统配置
DELETE FROM system_config WHERE key IN (
    'web3.supported_wallet_types',
    'web3.max_wallets_per_user',
    'web3.nonce_expiry_minutes',
    'web3.rate_limit_per_ip',
    'web3.rate_limit_window_minutes'
);

-- 验证回滚
DO $$
DECLARE
    table_exists INTEGER;
BEGIN
    SELECT COUNT(*) INTO table_exists
    FROM information_schema.tables
    WHERE table_name IN ('web3_wallets', 'user_wallets', 'web3_wallet_nonces');

    IF table_exists > 0 THEN
        RAISE EXCEPTION '回滚失败：仍有%个表存在', table_exists;
    END IF;

    RAISE NOTICE '✅ Web3钱包表回滚完成';
END $$;
