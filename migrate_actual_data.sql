-- 实际数据迁移脚本 - 从SQLite迁移到PostgreSQL
-- 执行方式: psql -h localhost -p 5433 -U nofx -d nofx -f migrate_actual_data.sql

-- 首先插入default用户（满足外键约束）
INSERT INTO users (id, email, password_hash, otp_secret, otp_verified, created_at, updated_at) VALUES
('default', 'default@localhost', '', '', true, '2025-11-03 09:09:52', '2025-11-03 09:09:52')
ON CONFLICT (id) DO NOTHING;

-- 插入AI模型数据（转换布尔值：0->false, 1->true）
INSERT INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name, created_at, updated_at) VALUES
('deepseek', 'default', 'DeepSeek', 'deepseek', false, '', '', '', '2025-11-03 09:09:52', '2025-11-03 09:09:52'),
('qwen', 'default', 'Qwen', 'qwen', false, '', '', '', '2025-11-03 09:09:52', '2025-11-03 09:09:52')
ON CONFLICT (id) DO NOTHING;

-- 插入交易所数据（转换布尔值）
INSERT INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet, hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, created_at, updated_at) VALUES
('binance', 'default', 'Binance Futures', 'binance', false, '', '', false, '', '', '', '', '2025-11-03 09:09:52', '2025-11-03 09:09:52'),
('hyperliquid', 'default', 'Hyperliquid', 'hyperliquid', false, '', '', false, '', '', '', '', '2025-11-03 09:09:52', '2025-11-03 09:09:52'),
('aster', 'default', 'Aster DEX', 'aster', false, '', '', false, '', '', '', '', '2025-11-03 09:09:52', '2025-11-03 09:09:52')
ON CONFLICT (id, user_id) DO NOTHING;

-- 插入系统配置数据
INSERT INTO system_config (key, value, updated_at) VALUES
('coin_pool_api_url', '', '2025-11-03 09:09:52'),
('btc_eth_leverage', '5', '2025-11-03 09:09:52'),
('api_server_port', '8080', '2025-11-03 09:09:52'),
('oi_top_api_url', '', '2025-11-03 09:09:52'),
('stop_trading_minutes', '60', '2025-11-03 09:09:52'),
('default_coins', '["BTCUSDT","ETHUSDT","SOLUSDT","BNBUSDT","XRPUSDT","DOGEUSDT","ADAUSDT","HYPEUSDT"]', '2025-11-03 09:09:52'),
('altcoin_leverage', '5', '2025-11-03 09:09:52'),
('beta_mode', 'true', '2025-11-03 09:09:52'),
('use_default_coins', 'true', '2025-11-03 09:09:52'),
('max_daily_loss', '10.0', '2025-11-03 09:09:52'),
('jwt_secret', 'Qk0kAa+d0iIEzXVHXbNbm+UaN3RNabmWtH8rDWZ5OPf+4GX8pBflAHodfpbipVMyrw1fsDanHsNBjhgbDeK9Jg==', '2025-11-03 09:09:52'),
('admin_mode', 'false', '2025-11-03 09:09:52'),
('max_drawdown', '20.0', '2025-11-03 09:09:52'),
('encryption_public_key', '-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAxDsGHRSFXqR2YFoWMNWC
8s0FlVE2KglHjLnm1f+i5yPfuTYkTUbVDu6RZuqLJdvhX+UO0x1XnwFIhZqmEfro
8Myr5+RnItl7QGqWWcKry4ZlPHroMwIK50WJt316KUKVUv7wUMMLoUUq7yctI8V/
thRX+ZRaErJJU9DWkSqjYOVdc+KwsZnN9WifoYhp6veTKmJ1kJOd6AVtF+KJ/z0R
hFarXjaQ89vf/oUgKahS/BUH7P6jpP+L+7z8G650oygp3Pn66eq+ttcUdc20WiBj
K5eDBUJUUeNmdesqZXBafhJBhsQyilC0+LgI+3laSkGh3odMdY5Mf9lnke9mfX8E
RQIDAQAB
-----END PUBLIC KEY-----', '2025-11-03 09:09:52'),
('encryption_public_key_version', 'mock-v1', '2025-11-03 09:09:52')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at;

-- 插入内测码数据（转换布尔值：0->false, 1->true）
INSERT INTO beta_codes (code, used, used_by, used_at, created_at) VALUES
('2aw4wm', false, '', NULL, '2025-11-03 09:09:52'),
('34cvds', false, '', NULL, '2025-11-03 09:09:52'),
('3f39nc', false, '', NULL, '2025-11-03 09:09:52'),
('3qmg67', false, '', NULL, '2025-11-03 09:09:52'),
('5rjp6k', false, '', NULL, '2025-11-03 09:09:52'),
('65a3e6', false, '', NULL, '2025-11-03 09:09:52'),
('6hzgpr', false, '', NULL, '2025-11-03 09:09:52'),
('6wruwb', false, '', NULL, '2025-11-03 09:09:52'),
('8bdf7a', false, '', NULL, '2025-11-03 09:09:52'),
('8jxnp5', false, '', NULL, '2025-11-03 09:09:52'),
('8xp3xq', false, '', NULL, '2025-11-03 09:09:52'),
('9r5uev', false, '', NULL, '2025-11-03 09:09:52'),
('adbn7p', false, '', NULL, '2025-11-03 09:09:52'),
('azm8y4', false, '', NULL, '2025-11-03 09:09:52'),
('b6tfqu', false, '', NULL, '2025-11-03 09:09:52'),
('bs32f9', false, '', NULL, '2025-11-03 09:09:52'),
('ctz8gn', false, '', NULL, '2025-11-03 09:09:52'),
('d8rmq8', false, '', NULL, '2025-11-03 09:09:52'),
('dmf2yt', false, '', NULL, '2025-11-03 09:09:52'),
('dz7e8d', false, '', NULL, '2025-11-03 09:09:52'),
('e9ptrm', false, '', NULL, '2025-11-03 09:09:52'),
('f25m8s', false, '', NULL, '2025-11-03 09:09:52'),
('feuzgb', false, '', NULL, '2025-11-03 09:09:52'),
('fnd7z7', false, '', NULL, '2025-11-03 09:09:52'),
('h43s95', false, '', NULL, '2025-11-03 09:09:52'),
('hgs7gq', false, '', NULL, '2025-11-03 09:09:52'),
('huhkra', false, '', NULL, '2025-11-03 09:09:52'),
('mhqch4', false, '', NULL, '2025-11-03 09:09:52'),
('mqwkau', false, '', NULL, '2025-11-03 09:09:52'),
('mwfssp', false, '', NULL, '2025-11-03 09:09:52'),
('na7629', false, '', NULL, '2025-11-03 09:09:52'),
('pb5c2n', false, '', NULL, '2025-11-03 09:09:52'),
('q5k6jt', false, '', NULL, '2025-11-03 09:09:52'),
('qrurb8', false, '', NULL, '2025-11-03 09:09:52'),
('rssybm', false, '', NULL, '2025-11-03 09:09:52'),
('s7hbk7', false, '', NULL, '2025-11-03 09:09:52'),
('sj8rus', false, '', NULL, '2025-11-03 09:09:52'),
('sxy53c', false, '', NULL, '2025-11-03 09:09:52'),
('t8fjmk', false, '', NULL, '2025-11-03 09:09:52'),
('udmqcb', false, '', NULL, '2025-11-03 09:09:52'),
('um6xu6', false, '', NULL, '2025-11-03 09:09:52'),
('uzwb4r', false, '', NULL, '2025-11-03 09:09:52'),
('w2uh55', false, '', NULL, '2025-11-03 09:09:52'),
('wejxcq', false, '', NULL, '2025-11-03 09:09:52'),
('wtaama', false, '', NULL, '2025-11-03 09:09:52'),
('x82qvu', false, '', NULL, '2025-11-03 09:09:52'),
('ygg4d4', false, '', NULL, '2025-11-03 09:09:52'),
('yv8hnn', false, '', NULL, '2025-11-03 09:09:52'),
('z9ywv8', false, '', NULL, '2025-11-03 09:09:52'),
('znpa5t', false, '', NULL, '2025-11-03 09:09:52')
ON CONFLICT (code) DO NOTHING;

-- 数据迁移验证查询
SELECT 'Migration Summary:' as status;
SELECT 'ai_models' as table_name, COUNT(*) as count FROM ai_models
UNION ALL
SELECT 'exchanges', COUNT(*) FROM exchanges
UNION ALL
SELECT 'system_config', COUNT(*) FROM system_config
UNION ALL
SELECT 'beta_codes', COUNT(*) FROM beta_codes;

-- 显示当前配置
SELECT 'Current System Config:' as status;
SELECT key, value FROM system_config ORDER BY key;