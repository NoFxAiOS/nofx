#!/bin/bash

# 从SQLite导入default用户和系统默认数据到PostgreSQL
echo "🔧 从SQLite导入default用户和系统默认数据"
echo "========================================"

# 检查SQLite数据库文件
SQLITE_DB="config.db"
if [ ! -f "$SQLITE_DB" ]; then
    echo "❌ 错误：找不到 SQLite 数据库文件 $SQLITE_DB"
    exit 1
fi

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

echo "📋 使用命令: $DOCKER_COMPOSE_CMD"

# 分析SQLite中的default用户数据
echo "📊 分析SQLite中的default用户数据..."
AI_MODEL_COUNT=$(sqlite3 $SQLITE_DB "SELECT COUNT(*) FROM ai_models WHERE user_id = 'default';" 2>/dev/null || echo "0")
EXCHANGE_COUNT=$(sqlite3 $SQLITE_DB "SELECT COUNT(*) FROM exchanges WHERE user_id = 'default';" 2>/dev/null || echo "0")

echo "   🤖 AI模型: $AI_MODEL_COUNT 个"
echo "   🏢 交易所: $EXCHANGE_COUNT 个"

if [ "$AI_MODEL_COUNT" -eq 0 ] && [ "$EXCHANGE_COUNT" -eq 0 ]; then
    echo "⚠️  SQLite中没有default用户的数据，将跳过导入"
    exit 0
fi

# 生成导入脚本
IMPORT_SQL="import_default_data.sql"

cat > $IMPORT_SQL << EOL
-- 从SQLite导入default用户和系统默认数据
-- 生成时间: $(date)

-- 设置时区
SET timezone = 'Asia/Shanghai';

-- 1. 创建default用户（如果不存在）
INSERT INTO users (id, email, password_hash, otp_secret, otp_verified, created_at, updated_at)
VALUES ('default', 'default@localhost', '', '', true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;

EOL

# 导出AI模型数据
if [ "$AI_MODEL_COUNT" -gt 0 ]; then
    echo "🤖 导出AI模型数据..."
    echo "-- AI模型数据 ($AI_MODEL_COUNT 条记录)" >> $IMPORT_SQL
    echo "INSERT INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name, created_at, updated_at) VALUES" >> $IMPORT_SQL
    
    sqlite3 $SQLITE_DB "SELECT '(' || quote(id) || ', ' || quote(user_id) || ', ' || quote(name) || ', ' || quote(provider) || ', ' || 
    CASE WHEN enabled = 1 THEN 'true' ELSE 'false' END || ', ' || quote(COALESCE(api_key, '')) || ', ' || quote(COALESCE(custom_api_url, '')) || ', ' || 
    quote(COALESCE(custom_model_name, '')) || ', ' || quote(created_at) || ', ' || quote(updated_at) || '),'
    FROM ai_models WHERE user_id = 'default';" | sed '$ s/,$//' >> $IMPORT_SQL
    
    echo "ON CONFLICT (id) DO UPDATE SET user_id = EXCLUDED.user_id, name = EXCLUDED.name, provider = EXCLUDED.provider, enabled = EXCLUDED.enabled, api_key = EXCLUDED.api_key, custom_api_url = EXCLUDED.custom_api_url, custom_model_name = EXCLUDED.custom_model_name, updated_at = EXCLUDED.updated_at;" >> $IMPORT_SQL
    echo "" >> $IMPORT_SQL
fi

# 导出交易所数据
if [ "$EXCHANGE_COUNT" -gt 0 ]; then
    echo "🏢 导出交易所数据..."
    echo "-- 交易所数据 ($EXCHANGE_COUNT 条记录)" >> $IMPORT_SQL
    echo "INSERT INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet, hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, created_at, updated_at) VALUES" >> $IMPORT_SQL
    
    sqlite3 $SQLITE_DB "SELECT '(' || quote(id) || ', ' || quote(user_id) || ', ' || quote(name) || ', ' || quote(type) || ', ' || 
    CASE WHEN enabled = 1 THEN 'true' ELSE 'false' END || ', ' || quote(COALESCE(api_key, '')) || ', ' || quote(COALESCE(secret_key, '')) || ', ' || 
    CASE WHEN testnet = 1 THEN 'true' ELSE 'false' END || ', ' || quote(COALESCE(hyperliquid_wallet_addr, '')) || ', ' || 
    quote(COALESCE(aster_user, '')) || ', ' || quote(COALESCE(aster_signer, '')) || ', ' || quote(COALESCE(aster_private_key, '')) || ', ' || 
    quote(created_at) || ', ' || quote(updated_at) || '),'
    FROM exchanges WHERE user_id = 'default';" | sed '$ s/,$//' >> $IMPORT_SQL
    
    echo "ON CONFLICT (id, user_id) DO UPDATE SET name = EXCLUDED.name, type = EXCLUDED.type, enabled = EXCLUDED.enabled, api_key = EXCLUDED.api_key, secret_key = EXCLUDED.secret_key, testnet = EXCLUDED.testnet, hyperliquid_wallet_addr = EXCLUDED.hyperliquid_wallet_addr, aster_user = EXCLUDED.aster_user, aster_signer = EXCLUDED.aster_signer, aster_private_key = EXCLUDED.aster_private_key, updated_at = EXCLUDED.updated_at;" >> $IMPORT_SQL
    echo "" >> $IMPORT_SQL
fi

# 添加验证查询
cat >> $IMPORT_SQL << 'EOL'
-- 验证导入结果
SELECT '=== 导入完成验证 ===' as status;
SELECT 'default用户' as item, COUNT(*) as count FROM users WHERE id = 'default'
UNION ALL SELECT 'AI模型', COUNT(*) FROM ai_models WHERE user_id = 'default'
UNION ALL SELECT '交易所', COUNT(*) FROM exchanges WHERE user_id = 'default';
EOL

echo "✅ 生成导入脚本: $IMPORT_SQL"

# 确认执行
echo
echo "⚠️  准备导入default用户和系统默认数据，包括："
echo "   1. default用户账户"
echo "   2. $AI_MODEL_COUNT 个AI模型"
echo "   3. $EXCHANGE_COUNT 个交易所"
echo
read -p "确认执行导入? (y/N): " confirm

if [[ $confirm != [yY] ]]; then
    echo "ℹ️  已取消导入"
    echo "手动执行命令: $DOCKER_COMPOSE_CMD exec postgres psql -U nofx -d nofx -f /tmp/$IMPORT_SQL"
    exit 0
fi

# 检查PostgreSQL连接
echo "🔌 检查数据库连接..."
if ! $DOCKER_COMPOSE_CMD exec postgres pg_isready -U nofx > /dev/null 2>&1; then
    echo "❌ PostgreSQL连接失败，请确保服务正在运行"
    exit 1
fi

# 复制SQL脚本到容器
echo "📦 复制导入脚本到容器..."
POSTGRES_CONTAINER=$($DOCKER_COMPOSE_CMD ps -q postgres)
if [ -z "$POSTGRES_CONTAINER" ]; then
    echo "❌ 找不到PostgreSQL容器"
    exit 1
fi

docker cp $IMPORT_SQL ${POSTGRES_CONTAINER}:/tmp/$IMPORT_SQL

# 执行导入
echo "🔄 执行数据导入..."
if $DOCKER_COMPOSE_CMD exec postgres psql -U nofx -d nofx --pset pager=off -f /tmp/$IMPORT_SQL; then
    echo
    echo "✅ default用户和系统默认数据导入成功！"
    echo
    echo "📋 现在可以访问以下接口："
    echo "   - GET /api/supported-models    ($AI_MODEL_COUNT 个AI模型)"
    echo "   - GET /api/supported-exchanges ($EXCHANGE_COUNT 个交易所)"
    echo
    echo "🧹 清理导入文件..."
    rm -f $IMPORT_SQL
    $DOCKER_COMPOSE_CMD exec postgres rm -f /tmp/$IMPORT_SQL
else
    echo "❌ 数据导入失败"
    exit 1
fi

echo "🎉 导入完成！"
