-- ============================================================
-- nofx LIGHTER API Key 支持迁移
-- Version: 1.0.0
-- Date: 2025-01-20
-- Description: 添加 lighter_api_key_private_key 字段支持 LIGHTER V2
-- ============================================================

-- 开启 WAL 模式检查 (确保已启用)
PRAGMA journal_mode;
-- 预期输出: wal

-- 开始事务
BEGIN TRANSACTION;

-- ============================================================
-- Part 1: 添加 LIGHTER API Key 字段
-- ============================================================

-- 检查列是否已存在（SQLite 3.35.0+ 支持 IF NOT EXISTS）
-- 如果您的 SQLite 版本较旧，请手动检查后再执行
ALTER TABLE exchanges
ADD COLUMN lighter_api_key_private_key TEXT DEFAULT '';

-- ============================================================
-- Part 2: 更新现有 LIGHTER 配置的注释
-- ============================================================

-- 对于已有的 LIGHTER 配置，lighter_api_key_private_key 默认为空字符串
-- 用户需要手动配置才能使用 V2 功能

-- 提交事务
COMMIT;

-- ============================================================
-- Part 3: 验证迁移
-- ============================================================

-- 查看 exchanges 表结构
PRAGMA table_info(exchanges);

-- 验证新增字段
SELECT
    name,
    type,
    dflt_value
FROM pragma_table_info('exchanges')
WHERE name = 'lighter_api_key_private_key';

-- 统计现有 LIGHTER 配置
SELECT
    COUNT(*) as lighter_count,
    SUM(CASE WHEN lighter_api_key_private_key != '' THEN 1 ELSE 0 END) as with_api_key,
    SUM(CASE WHEN lighter_api_key_private_key = '' THEN 1 ELSE 0 END) as without_api_key
FROM exchanges
WHERE type = 'lighter';

-- 输出摘要
SELECT '✅ LIGHTER API Key field added successfully' AS status;
SELECT 'ℹ️  现有 LIGHTER 配置默认使用 V1 (需手动配置 API Key 以使用 V2)' AS note;
