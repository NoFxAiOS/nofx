#!/bin/bash

# PostgreSQL数据查看工具
echo "🔍 PostgreSQL 数据查看工具"
echo "=========================="

# 检测Docker Compose命令
DOCKER_COMPOSE_CMD=""
if command -v "docker-compose" &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker-compose"
elif command -v "docker" &> /dev/null && docker compose version &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker compose"
else
    echo "❌ 错误：找不到 docker-compose 或 docker compose 命令"
    exit 1
fi

echo "📊 数据库概览:"
$DOCKER_COMPOSE_CMD exec postgres psql -U nofx -d nofx --pset pager=off -c "
SELECT relname as \"表名\", n_live_tup as \"记录数\" 
FROM pg_stat_user_tables 
WHERE n_live_tup > 0 
ORDER BY relname;
"

echo -e "\n🤖 AI模型配置:"
$DOCKER_COMPOSE_CMD exec postgres psql -U nofx -d nofx --pset pager=off -c "
SELECT id, name, provider, enabled, 
       CASE WHEN api_key != '' THEN '已配置' ELSE '未配置' END as api_key_status
FROM ai_models ORDER BY id;
"

echo -e "\n🏢 交易所配置:"
$DOCKER_COMPOSE_CMD exec postgres psql -U nofx -d nofx --pset pager=off -c "
SELECT id, name, type, enabled,
       CASE WHEN api_key != '' THEN '已配置' ELSE '未配置' END as api_key_status
FROM exchanges ORDER BY id;
"

echo -e "\n⚙️ 关键系统配置:"
$DOCKER_COMPOSE_CMD exec postgres psql -U nofx -d nofx --pset pager=off -c "
SELECT key, 
       CASE 
         WHEN LENGTH(value) > 50 THEN LEFT(value, 50) || '...'
         ELSE value 
       END as value
FROM system_config 
WHERE key IN ('admin_mode', 'beta_mode', 'api_server_port', 'default_coins', 'jwt_secret')
ORDER BY key;
"

echo -e "\n🎟️ 内测码统计:"
$DOCKER_COMPOSE_CMD exec postgres psql -U nofx -d nofx --pset pager=off -c "
SELECT 
    CASE WHEN used THEN '已使用' ELSE '未使用' END as status,
    COUNT(*) as count 
FROM beta_codes 
GROUP BY used 
ORDER BY used;
"

echo -e "\n📝 未使用的内测码:"
$DOCKER_COMPOSE_CMD exec postgres psql -U nofx -d nofx --pset pager=off -c "
SELECT code 
FROM beta_codes 
WHERE used = false 
ORDER BY created_at DESC;
"

echo -e "\n👥 用户信息:"
$DOCKER_COMPOSE_CMD exec postgres psql -U nofx -d nofx --pset pager=off -c "
SELECT id, email, otp_verified, created_at FROM users ORDER BY created_at;
"
