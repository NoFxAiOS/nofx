#!/bin/bash

# 邀请码数据回填脚本 - 直接更新数据库
# 不需要修改代码，只需要修复数据

set -e

echo "================================================================"
echo "邀请码数据回填脚本"
echo "================================================================"
echo ""

# 连接数据库
psql $DATABASE_URL << 'SQL'

-- 检查有多少用户缺少邀请码
SELECT '📊 缺少邀请码的用户数:' as check;
SELECT COUNT(*) FROM users WHERE invite_code IS NULL OR invite_code = '';

-- 为老用户生成邀请码
-- 使用 substring(md5(id || created_at), 1, 20) 为每个用户生成唯一的邀请码
UPDATE users
SET invite_code = upper(substring(md5(id || created_at::text || random()::text), 1, 20))
WHERE invite_code IS NULL OR invite_code = '';

-- 验证更新结果
SELECT '✅ 更新后缺少邀请码的用户数:' as check;
SELECT COUNT(*) FROM users WHERE invite_code IS NULL OR invite_code = '';

-- 显示样本
SELECT '✅ 更新后的邀请码样本:' as check;
SELECT id, email, invite_code FROM users WHERE invite_code IS NOT NULL LIMIT 5;

SQL

echo ""
echo "================================================================"
echo "✅ 邀请码回填完成！"
echo "================================================================"
