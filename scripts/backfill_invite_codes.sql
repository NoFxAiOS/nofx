-- 邀请码数据回填 SQL 脚本
-- 为所有缺少 invite_code 的老用户生成唯一邀请码

-- 步骤 1: 检查缺少邀请码的用户数量
SELECT
    COUNT(*) as missing_invite_codes_count
FROM users
WHERE invite_code IS NULL OR invite_code = '';

-- 步骤 2: 为所有缺少邀请码的用户生成邀请码
-- 使用 MD5 哈希结合 ID 和时间戳来生成唯一的邀请码
UPDATE users
SET invite_code = upper(substr(md5(id || ':' || created_at::text || ':' || random()::text), 1, 20))
WHERE (invite_code IS NULL OR invite_code = '')
  AND email IS NOT NULL;

-- 步骤 3: 验证更新结果
SELECT
    COUNT(*) as remaining_missing_count
FROM users
WHERE invite_code IS NULL OR invite_code = '';

-- 步骤 4: 显示更新后的样本数据
SELECT id, email, invite_code, created_at
FROM users
WHERE invite_code IS NOT NULL
ORDER BY updated_at DESC
LIMIT 10;
