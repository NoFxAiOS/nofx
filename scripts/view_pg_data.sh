#!/bin/bash

set -euo pipefail

# 保证从仓库根目录运行
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

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

# 加载数据库配置
ENV_FILE=".env"
if [ -f "$ENV_FILE" ]; then
    echo "📁 加载 .env 配置..."
    set -a
    # shellcheck disable=SC1090
    source "$ENV_FILE"
    set +a
else
    echo "⚠️  未找到 .env 文件，使用默认数据库配置"
fi

POSTGRES_HOST=${POSTGRES_HOST:-postgres}
POSTGRES_PORT=${POSTGRES_PORT:-5432}
POSTGRES_DB=${POSTGRES_DB:-nofx}
POSTGRES_USER=${POSTGRES_USER:-nofx}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-}
POSTGRES_SERVICE=${POSTGRES_SERVICE:-postgres}
POSTGRES_CONTAINER_NAME=${POSTGRES_CONTAINER_NAME:-nofx-postgres}

# 获取 PostgreSQL 容器 ID
POSTGRES_CONTAINER=$($DOCKER_COMPOSE_CMD ps -q "$POSTGRES_SERVICE" 2>/dev/null || true)
if [ -z "$POSTGRES_CONTAINER" ]; then
    POSTGRES_CONTAINER=$(docker ps -q --filter "name=$POSTGRES_CONTAINER_NAME" | head -n 1)
fi

if [ -z "$POSTGRES_CONTAINER" ]; then
    echo "❌ 找不到 PostgreSQL 容器 (${POSTGRES_SERVICE}/${POSTGRES_CONTAINER_NAME})"
    echo "💡 请确认数据库服务已启动"
    exit 1
fi

PG_ENV_ARGS=()
if [ -n "$POSTGRES_PASSWORD" ]; then
    PG_ENV_ARGS=(--env "PGPASSWORD=$POSTGRES_PASSWORD")
fi

run_psql() {
    local sql="$1"
    docker exec -i "${PG_ENV_ARGS[@]}" "$POSTGRES_CONTAINER" \
        psql -v ON_ERROR_STOP=1 --pset pager=off -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "$sql"
}

echo "📋 数据库容器: $POSTGRES_CONTAINER"
echo "📋 连接参数: $POSTGRES_HOST:${POSTGRES_PORT}/$POSTGRES_DB (user: $POSTGRES_USER)"

echo "📊 数据库概览:"
run_psql "SELECT relname AS \"表名\", n_live_tup AS \"记录数\" FROM pg_stat_user_tables WHERE n_live_tup > 0 ORDER BY relname;"

echo -e "\n🤖 AI模型配置:"
run_psql "SELECT id, name, provider, enabled, CASE WHEN api_key != '' THEN '已配置' ELSE '未配置' END AS api_key_status FROM ai_models ORDER BY id;"

echo -e "\n🏢 交易所配置:"
run_psql "SELECT id, name, type, enabled, CASE WHEN api_key != '' THEN '已配置' ELSE '未配置' END AS api_key_status FROM exchanges ORDER BY id;"

echo -e "\n⚙️ 关键系统配置:"
run_psql "SELECT key, CASE WHEN LENGTH(value) > 50 THEN LEFT(value, 50) || '...' ELSE value END AS value FROM system_config WHERE key IN ('beta_mode', 'api_server_port', 'default_coins', 'jwt_secret') ORDER BY key;"

echo -e "\n🎟️ 内测码统计:"
run_psql "SELECT CASE WHEN used THEN '已使用' ELSE '未使用' END AS status, COUNT(*) AS count FROM beta_codes GROUP BY used ORDER BY used;"

echo -e "\n👥 用户信息:"
run_psql "SELECT id, email, otp_verified, created_at FROM users ORDER BY created_at;"
