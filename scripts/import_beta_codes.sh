#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

echo "🎟️ 导入 beta_codes.txt 到 PostgreSQL"

if [ ! -f "beta_codes.txt" ]; then
    echo "❌ 找不到 beta_codes.txt 文件"
    exit 1
fi

if command -v "docker-compose" &> /dev/null; then
    DOCKER_CMD="docker-compose"
elif command -v "docker" &> /dev/null && docker compose version &> /dev/null; then
    DOCKER_CMD="docker compose"
else
    echo "❌ 错误：找不到 docker-compose 或 docker compose 命令"
    exit 1
fi

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

POSTGRES_CONTAINER=$($DOCKER_CMD ps -q "$POSTGRES_SERVICE" 2>/dev/null || true)
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

SQL_PAYLOAD=$(python3 - <<'PY'
from pathlib import Path

codes = []
for line in Path('beta_codes.txt').read_text(encoding='utf-8').splitlines():
    code = line.strip()
    if code and not code.startswith('#'):
        codes.append(f"('{code}')")

if codes:
    values = ",\n".join(codes)
    print(f"INSERT INTO beta_codes (code) VALUES\n{values}\nON CONFLICT (code) DO NOTHING;")
PY
)

if [ -z "$SQL_PAYLOAD" ]; then
    echo "⚠️  beta_codes.txt 中没有有效的内测码，已跳过导入"
    exit 0
fi

TOTAL_CODES=$(grep -vc '^\s*$' beta_codes.txt || true)
echo "📊 检测到 $TOTAL_CODES 条内测码记录"

echo "🔄 导入到数据库..."
printf '%s\n' "$SQL_PAYLOAD" | docker exec -i "${PG_ENV_ARGS[@]}" "$POSTGRES_CONTAINER" \
    psql -v ON_ERROR_STOP=1 --pset pager=off -U "$POSTGRES_USER" -d "$POSTGRES_DB"

echo "✅ 导入完成（重复的已跳过）"
