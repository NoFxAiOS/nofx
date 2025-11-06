#!/bin/bash

set -euo pipefail

echo "🔧 同步默认用户与基础配置"
echo "==============================="

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

# 检测 Docker Compose 命令
if command -v docker-compose &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker-compose"
elif docker compose version &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker compose"
else
    echo "❌ 无法找到 docker-compose 或 docker compose 命令"
    exit 1
fi

echo "📋 使用命令: $DOCKER_COMPOSE_CMD"

# 加载 .env 配置
ENV_FILE=".env"
if [ -f "$ENV_FILE" ]; then
    echo "📁 加载 .env ..."
    set -a
    # shellcheck disable=SC1090
    source "$ENV_FILE"
    set +a
else
    echo "⚠️  未找到 .env，使用默认数据库配置"
fi

POSTGRES_HOST=${POSTGRES_HOST:-postgres}
POSTGRES_PORT=${POSTGRES_PORT:-5432}
POSTGRES_DB=${POSTGRES_DB:-nofx}
POSTGRES_USER=${POSTGRES_USER:-nofx}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-}
POSTGRES_SERVICE=${POSTGRES_SERVICE:-postgres}
POSTGRES_CONTAINER_NAME=${POSTGRES_CONTAINER_NAME:-nofx-postgres}

# 查找 PostgreSQL 容器
POSTGRES_CONTAINER=$($DOCKER_COMPOSE_CMD ps -q "$POSTGRES_SERVICE" 2>/dev/null || true)
if [ -z "$POSTGRES_CONTAINER" ]; then
    POSTGRES_CONTAINER=$(docker ps -q --filter "name=$POSTGRES_CONTAINER_NAME" | head -n 1)
fi

if [ -z "$POSTGRES_CONTAINER" ]; then
    echo "❌ 未找到 PostgreSQL 容器 (${POSTGRES_SERVICE}/${POSTGRES_CONTAINER_NAME})"
    echo "💡 请先启动数据库容器: $DOCKER_COMPOSE_CMD up -d postgres"
    exit 1
fi

PG_ENV_ARGS=()
if [ -n "$POSTGRES_PASSWORD" ]; then
    PG_ENV_ARGS=(-e "PGPASSWORD=$POSTGRES_PASSWORD")
fi

echo "🔌 检查数据库连接..."
if ! docker exec "${PG_ENV_ARGS[@]}" "$POSTGRES_CONTAINER" pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" > /dev/null 2>&1; then
    echo "❌ 无法连接到 PostgreSQL，请确认容器和凭据"
    exit 1
fi

echo
read -p "确认写入默认账号和基础配置? (y/N): " confirm
if [[ $confirm != [yY] ]]; then
    echo "ℹ️  已取消操作"
    exit 0
fi

echo "🚀 执行初始化 SQL..."
if docker exec -i "${PG_ENV_ARGS[@]}" "$POSTGRES_CONTAINER" \
    psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<'SQL'
-- 确保 traders 表存在 custom_coins 字段
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'traders' AND column_name = 'custom_coins'
    ) THEN
        ALTER TABLE traders ADD COLUMN custom_coins TEXT DEFAULT '';
    END IF;
END
$$;

-- 创建 default 用户
INSERT INTO users (id, email, password_hash, otp_secret, otp_verified, created_at, updated_at)
VALUES ('default', 'default@localhost', '', '', true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO UPDATE
    SET email = EXCLUDED.email,
        updated_at = CURRENT_TIMESTAMP;

-- 默认 AI 模型配置
INSERT INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name, created_at, updated_at) VALUES
('deepseek', 'default', 'DeepSeek', 'deepseek', false, '', '', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('qwen', 'default', 'Qwen', 'qwen', false, '', '', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO UPDATE
    SET user_id = EXCLUDED.user_id,
        name = EXCLUDED.name,
        provider = EXCLUDED.provider,
        enabled = EXCLUDED.enabled,
        api_key = EXCLUDED.api_key,
        custom_api_url = EXCLUDED.custom_api_url,
        custom_model_name = EXCLUDED.custom_model_name,
        updated_at = CURRENT_TIMESTAMP;

-- 默认交易所配置
INSERT INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet,
                       hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key,
                       created_at, updated_at) VALUES
('binance', 'default', 'Binance Futures', 'binance', false, '', '', false, '', '', '', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('hyperliquid', 'default', 'Hyperliquid', 'hyperliquid', false, '', '', false, '', '', '', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('aster', 'default', 'Aster DEX', 'aster', false, '', '', false, '', '', '', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id, user_id) DO UPDATE
    SET name = EXCLUDED.name,
        type = EXCLUDED.type,
        enabled = EXCLUDED.enabled,
        api_key = EXCLUDED.api_key,
        secret_key = EXCLUDED.secret_key,
        testnet = EXCLUDED.testnet,
        hyperliquid_wallet_addr = EXCLUDED.hyperliquid_wallet_addr,
        aster_user = EXCLUDED.aster_user,
        aster_signer = EXCLUDED.aster_signer,
        aster_private_key = EXCLUDED.aster_private_key,
        updated_at = CURRENT_TIMESTAMP;

-- 默认系统配置（不存在时写入）
INSERT INTO system_config (key, value) VALUES
('beta_mode', 'false'),
('api_server_port', '8080'),
('use_default_coins', 'true'),
('default_coins', '["BTCUSDT","ETHUSDT","SOLUSDT","BNBUSDT","XRPUSDT","DOGEUSDT","ADAUSDT","HYPEUSDT"]'),
('max_daily_loss', '10.0'),
('max_drawdown', '20.0'),
('stop_trading_minutes', '60'),
('btc_eth_leverage', '5'),
('altcoin_leverage', '5'),
('jwt_secret', '')
ON CONFLICT (key) DO NOTHING;

-- 输出校验信息
SELECT 'default_user' AS item, COUNT(*) AS count FROM users WHERE id = 'default'
UNION ALL
SELECT 'default_ai_models', COUNT(*) FROM ai_models WHERE user_id = 'default'
UNION ALL
SELECT 'default_exchanges', COUNT(*) FROM exchanges WHERE user_id = 'default';
SQL
then
    echo
    echo "✅ 默认数据写入完成"
else
    echo
    echo "❌ 数据写入失败"
    exit 1
fi

echo "🎉 操作完成"
