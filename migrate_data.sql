-- PostgreSQL数据迁移脚本
-- 从SQLite导出的数据转换为PostgreSQL格式

-- 注意：这个脚本需要根据实际的SQLite导出数据进行调整
-- 主要差异：
-- 1. SQLite的AUTOINCREMENT -> PostgreSQL的SERIAL
-- 2. 布尔值：SQLite的0/1 -> PostgreSQL的false/true
-- 3. 日期时间格式可能需要调整
-- 4. 主键冲突处理：使用ON CONFLICT

-- 如果有实际数据，请在这里添加INSERT语句
-- 例如：

-- 插入用户数据（如果有）
-- INSERT INTO users (id, email, password_hash, otp_secret, otp_verified, created_at, updated_at) 
-- VALUES (...) ON CONFLICT (id) DO NOTHING;

-- 插入AI模型配置（如果有自定义）
-- INSERT INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name, created_at, updated_at)
-- VALUES (...) ON CONFLICT (id) DO NOTHING;

-- 插入交易所配置（如果有自定义）
-- INSERT INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet, hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, created_at, updated_at)
-- VALUES (...) ON CONFLICT (id, user_id) DO NOTHING;

-- 插入交易员配置（如果有）
-- INSERT INTO traders (id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running, btc_eth_leverage, altcoin_leverage, trading_symbols, use_coin_pool, use_oi_top, custom_prompt, override_base_prompt, system_prompt_template, is_cross_margin, created_at, updated_at)
-- VALUES (...) ON CONFLICT (id) DO NOTHING;

-- 插入系统配置（如果有自定义）
-- INSERT INTO system_config (key, value, updated_at)
-- VALUES (...) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

-- 插入内测码（如果有）
-- INSERT INTO beta_codes (code, used, used_by, used_at, created_at)
-- VALUES (...) ON CONFLICT (code) DO NOTHING;

-- 数据迁移完成后的验证查询
-- SELECT 'users' as table_name, COUNT(*) as count FROM users
-- UNION ALL
-- SELECT 'ai_models', COUNT(*) FROM ai_models
-- UNION ALL
-- SELECT 'exchanges', COUNT(*) FROM exchanges
-- UNION ALL
-- SELECT 'traders', COUNT(*) FROM traders
-- UNION ALL
-- SELECT 'system_config', COUNT(*) FROM system_config
-- UNION ALL
-- SELECT 'beta_codes', COUNT(*) FROM beta_codes;