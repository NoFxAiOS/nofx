#!/bin/bash

# 生产环境 SQLite -> PostgreSQL 数据迁移脚本
# 真实数据迁移工具 - 支持完整数据导出和迁移

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║           🚀 生产环境数据迁移工具 SQLite → PostgreSQL        ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo

# 检查必要文件
SQLITE_DB="config.db"
if [ ! -f "$SQLITE_DB" ]; then
    echo -e "${RED}❌ 错误：找不到 SQLite 数据库文件 $SQLITE_DB${NC}"
    exit 1
fi

# 检测Docker Compose命令
DOCKER_COMPOSE_CMD=""
if command -v "docker-compose" &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker-compose"
elif command -v "docker" &> /dev/null && docker compose version &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker compose"
else
    echo -e "${RED}❌ 错误：找不到 docker-compose 或 docker compose 命令${NC}"
    exit 1
fi

echo -e "${CYAN}📋 使用命令: ${DOCKER_COMPOSE_CMD}${NC}"

# 分析当前SQLite数据
echo -e "\n${BLUE}📊 分析当前SQLite数据库...${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# 获取数据统计
USER_COUNT=$(sqlite3 $SQLITE_DB "SELECT COUNT(*) FROM users;" 2>/dev/null || echo "0")
AI_MODEL_COUNT=$(sqlite3 $SQLITE_DB "SELECT COUNT(*) FROM ai_models;" 2>/dev/null || echo "0")
EXCHANGE_COUNT=$(sqlite3 $SQLITE_DB "SELECT COUNT(*) FROM exchanges;" 2>/dev/null || echo "0")
TRADER_COUNT=$(sqlite3 $SQLITE_DB "SELECT COUNT(*) FROM traders;" 2>/dev/null || echo "0")
CONFIG_COUNT=$(sqlite3 $SQLITE_DB "SELECT COUNT(*) FROM system_config;" 2>/dev/null || echo "0")
BETA_CODE_COUNT=$(sqlite3 $SQLITE_DB "SELECT COUNT(*) FROM beta_codes;" 2>/dev/null || echo "0")

echo "📈 数据库表统计:"
echo "   👥 用户 (users): $USER_COUNT"
echo "   🤖 AI模型 (ai_models): $AI_MODEL_COUNT"  
echo "   🏢 交易所 (exchanges): $EXCHANGE_COUNT"
echo "   🔧 交易员 (traders): $TRADER_COUNT"
echo "   ⚙️  系统配置 (system_config): $CONFIG_COUNT"
echo "   🎟️  内测码 (beta_codes): $BETA_CODE_COUNT"

# 检查是否有exchange_secrets表
if sqlite3 $SQLITE_DB "SELECT name FROM sqlite_master WHERE type='table' AND name='exchange_secrets';" | grep -q exchange_secrets; then
    SECRET_COUNT=$(sqlite3 $SQLITE_DB "SELECT COUNT(*) FROM exchange_secrets;" 2>/dev/null || echo "0")
    echo "   🔐 交易所密钥 (exchange_secrets): $SECRET_COUNT"
fi

# 检查是否有user_signal_sources表
if sqlite3 $SQLITE_DB "SELECT name FROM sqlite_master WHERE type='table' AND name='user_signal_sources';" | grep -q user_signal_sources; then
    SIGNAL_COUNT=$(sqlite3 $SQLITE_DB "SELECT COUNT(*) FROM user_signal_sources;" 2>/dev/null || echo "0")
    echo "   📡 用户信号源 (user_signal_sources): $SIGNAL_COUNT"
fi

echo

# 生成迁移时间戳
TIMESTAMP=$(date '+%Y-%m-%d_%H-%M-%S')
MIGRATION_FILE="migrate_production_${TIMESTAMP}.sql"

echo -e "${BLUE}📤 生成数据迁移脚本: $MIGRATION_FILE${NC}"

# 生成SQL迁移脚本
cat > $MIGRATION_FILE << EOL
-- 生产环境数据迁移脚本
-- 从SQLite自动导出生成
-- 执行时间: $(date)

-- 设置时区
SET timezone = 'Asia/Shanghai';

EOL

# 导出用户数据
if [ "$USER_COUNT" -gt 0 ]; then
    echo -e "${CYAN}👥 导出用户数据...${NC}"
    echo "-- 用户数据 ($USER_COUNT 条记录)" >> $MIGRATION_FILE
    echo "INSERT INTO users (id, email, password_hash, otp_secret, otp_verified, created_at, updated_at) VALUES" >> $MIGRATION_FILE
    
    sqlite3 $SQLITE_DB "SELECT '(' || quote(id) || ', ' || quote(COALESCE(email, '')) || ', ' || quote(COALESCE(password_hash, '')) || ', ' || quote(COALESCE(otp_secret, '')) || ', ' || 
    CASE WHEN otp_verified = 1 THEN 'true' ELSE 'false' END || ', ' || quote(created_at) || ', ' || quote(updated_at) || '),'
    FROM users;" | sed '$ s/,$//' >> $MIGRATION_FILE
    
    echo "ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email, password_hash = EXCLUDED.password_hash, otp_secret = EXCLUDED.otp_secret, otp_verified = EXCLUDED.otp_verified, updated_at = EXCLUDED.updated_at;" >> $MIGRATION_FILE
    echo "" >> $MIGRATION_FILE
fi

# 导出AI模型数据
if [ "$AI_MODEL_COUNT" -gt 0 ]; then
    echo -e "${CYAN}🤖 导出AI模型数据...${NC}"
    echo "-- AI模型数据 ($AI_MODEL_COUNT 条记录)" >> $MIGRATION_FILE
    echo "INSERT INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name, created_at, updated_at) VALUES" >> $MIGRATION_FILE
    
    sqlite3 $SQLITE_DB "SELECT '(' || quote(id) || ', ' || quote(user_id) || ', ' || 
    quote(name) || ', ' || quote(provider) || ', ' || 
    CASE WHEN enabled = 1 THEN 'true' ELSE 'false' END || ', ' || quote(api_key) || ', ' || quote(COALESCE(custom_api_url, '')) || ', ' || 
    quote(COALESCE(custom_model_name, '')) || ', ' || quote(created_at) || ', ' || quote(updated_at) || '),'
    FROM ai_models WHERE user_id IS NOT NULL AND user_id != '' AND user_id != 'default' 
    AND user_id IN (SELECT id FROM users);" | sed '$ s/,$//' >> $MIGRATION_FILE
    
    echo "ON CONFLICT (id) DO UPDATE SET user_id = EXCLUDED.user_id, name = EXCLUDED.name, provider = EXCLUDED.provider, enabled = EXCLUDED.enabled, api_key = EXCLUDED.api_key, custom_api_url = EXCLUDED.custom_api_url, custom_model_name = EXCLUDED.custom_model_name, updated_at = EXCLUDED.updated_at;" >> $MIGRATION_FILE
    echo "" >> $MIGRATION_FILE
fi

# 导出交易所数据
if [ "$EXCHANGE_COUNT" -gt 0 ]; then
    echo -e "${CYAN}🏢 导出交易所数据...${NC}"
    echo "-- 交易所数据 ($EXCHANGE_COUNT 条记录)" >> $MIGRATION_FILE
    echo "INSERT INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet, hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, created_at, updated_at) VALUES" >> $MIGRATION_FILE
    
    sqlite3 $SQLITE_DB "SELECT '(' || quote(id) || ', ' || quote(user_id) || ', ' || 
    quote(name) || ', ' || quote(type) || ', ' || 
    CASE WHEN enabled = 1 THEN 'true' ELSE 'false' END || ', ' || quote(COALESCE(api_key, '')) || ', ' || quote(COALESCE(secret_key, '')) || ', ' || 
    CASE WHEN testnet = 1 THEN 'true' ELSE 'false' END || ', ' || quote(COALESCE(hyperliquid_wallet_addr, '')) || ', ' || 
    quote(COALESCE(aster_user, '')) || ', ' || quote(COALESCE(aster_signer, '')) || ', ' || quote(COALESCE(aster_private_key, '')) || ', ' || 
    quote(created_at) || ', ' || quote(updated_at) || '),'
    FROM exchanges WHERE user_id IS NOT NULL AND user_id != '' AND user_id != 'default'
    AND user_id IN (SELECT id FROM users);" | sed '$ s/,$//' >> $MIGRATION_FILE
    
    echo "ON CONFLICT (id, user_id) DO UPDATE SET name = EXCLUDED.name, type = EXCLUDED.type, enabled = EXCLUDED.enabled, api_key = EXCLUDED.api_key, secret_key = EXCLUDED.secret_key, testnet = EXCLUDED.testnet, hyperliquid_wallet_addr = EXCLUDED.hyperliquid_wallet_addr, aster_user = EXCLUDED.aster_user, aster_signer = EXCLUDED.aster_signer, aster_private_key = EXCLUDED.aster_private_key, updated_at = EXCLUDED.updated_at;" >> $MIGRATION_FILE
    echo "" >> $MIGRATION_FILE
fi

# 导出交易员数据
if [ "$TRADER_COUNT" -gt 0 ]; then
    echo -e "${CYAN}🔧 导出交易员数据...${NC}"
    echo "-- 交易员数据 ($TRADER_COUNT 条记录)" >> $MIGRATION_FILE
    echo "INSERT INTO traders (id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running, btc_eth_leverage, altcoin_leverage, trading_symbols, use_coin_pool, use_oi_top, custom_prompt, override_base_prompt, system_prompt_template, is_cross_margin, created_at, updated_at) VALUES" >> $MIGRATION_FILE
    
    sqlite3 $SQLITE_DB "SELECT '(' || quote(id) || ', ' || quote(user_id) || ', ' || 
    quote(name) || ', ' || quote(ai_model_id) || ', ' || 
    quote(exchange_id) || ', ' || initial_balance || ', ' || scan_interval_minutes || ', ' || 
    CASE WHEN is_running = 1 THEN 'true' ELSE 'false' END || ', ' || btc_eth_leverage || ', ' || altcoin_leverage || ', ' || 
    quote(COALESCE(trading_symbols, '')) || ', ' || 
    CASE WHEN use_coin_pool = 1 THEN 'true' ELSE 'false' END || ', ' || CASE WHEN use_oi_top = 1 THEN 'true' ELSE 'false' END || ', ' || 
    quote(COALESCE(custom_prompt, '')) || ', ' || CASE WHEN override_base_prompt = 1 THEN 'true' ELSE 'false' END || ', ' || 
    quote(COALESCE(system_prompt_template, 'default')) || ', ' || CASE WHEN is_cross_margin = 1 THEN 'true' ELSE 'false' END || ', ' || 
    quote(created_at) || ', ' || quote(updated_at) || '),'
    FROM traders WHERE user_id IS NOT NULL AND user_id != '' AND user_id != 'default'
    AND user_id IN (SELECT id FROM users);" | sed '$ s/,$//' >> $MIGRATION_FILE
    
    echo "ON CONFLICT (id) DO UPDATE SET user_id = EXCLUDED.user_id, name = EXCLUDED.name, ai_model_id = EXCLUDED.ai_model_id, exchange_id = EXCLUDED.exchange_id, initial_balance = EXCLUDED.initial_balance, scan_interval_minutes = EXCLUDED.scan_interval_minutes, is_running = EXCLUDED.is_running, btc_eth_leverage = EXCLUDED.btc_eth_leverage, altcoin_leverage = EXCLUDED.altcoin_leverage, trading_symbols = EXCLUDED.trading_symbols, use_coin_pool = EXCLUDED.use_coin_pool, use_oi_top = EXCLUDED.use_oi_top, custom_prompt = EXCLUDED.custom_prompt, override_base_prompt = EXCLUDED.override_base_prompt, system_prompt_template = EXCLUDED.system_prompt_template, is_cross_margin = EXCLUDED.is_cross_margin, updated_at = EXCLUDED.updated_at;" >> $MIGRATION_FILE
    echo "" >> $MIGRATION_FILE
fi

# 导出系统配置
if [ "$CONFIG_COUNT" -gt 0 ]; then
    echo -e "${CYAN}⚙️  导出系统配置...${NC}"
    echo "-- 系统配置数据 ($CONFIG_COUNT 条记录)" >> $MIGRATION_FILE
    echo "INSERT INTO system_config (key, value, updated_at) VALUES" >> $MIGRATION_FILE
    
    sqlite3 $SQLITE_DB "SELECT '(' || quote(key) || ', ' || quote(value) || ', ' || quote(updated_at) || '),'
    FROM system_config;" | sed '$ s/,$//' >> $MIGRATION_FILE
    
    echo "ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at;" >> $MIGRATION_FILE
    echo "" >> $MIGRATION_FILE
fi

# 导出内测码数据
if [ "$BETA_CODE_COUNT" -gt 0 ]; then
    echo -e "${CYAN}🎟️  导出内测码数据...${NC}"
    echo "-- 内测码数据 ($BETA_CODE_COUNT 条记录)" >> $MIGRATION_FILE
    echo "INSERT INTO beta_codes (code, used, used_by, used_at, created_at) VALUES" >> $MIGRATION_FILE
    
    sqlite3 $SQLITE_DB "SELECT '(' || quote(code) || ', ' || CASE WHEN used = 1 THEN 'true' ELSE 'false' END || ', ' || 
    quote(COALESCE(used_by, '')) || ', ' || CASE WHEN used_at IS NULL THEN 'NULL' ELSE quote(used_at) END || ', ' || 
    quote(created_at) || '),'
    FROM beta_codes;" | sed '$ s/,$//' >> $MIGRATION_FILE
    
    echo "ON CONFLICT (code) DO UPDATE SET used = EXCLUDED.used, used_by = EXCLUDED.used_by, used_at = EXCLUDED.used_at;" >> $MIGRATION_FILE
    echo "" >> $MIGRATION_FILE
fi

# 导出用户信号源（如果存在）
if sqlite3 $SQLITE_DB "SELECT name FROM sqlite_master WHERE type='table' AND name='user_signal_sources';" | grep -q user_signal_sources; then
    SIGNAL_COUNT=$(sqlite3 $SQLITE_DB "SELECT COUNT(*) FROM user_signal_sources;" 2>/dev/null || echo "0")
    if [ "$SIGNAL_COUNT" -gt 0 ]; then
        echo -e "${CYAN}📡 导出用户信号源数据...${NC}"
        echo "-- 用户信号源数据 ($SIGNAL_COUNT 条记录)" >> $MIGRATION_FILE
        echo "INSERT INTO user_signal_sources (user_id, coin_pool_url, oi_top_url, created_at, updated_at) VALUES" >> $MIGRATION_FILE
        
        sqlite3 $SQLITE_DB "SELECT '(' || quote(user_id) || ', ' || 
        quote(COALESCE(coin_pool_url, '')) || ', ' || 
        quote(COALESCE(oi_top_url, '')) || ', ' || quote(created_at) || ', ' || quote(updated_at) || '),'
        FROM user_signal_sources WHERE user_id IS NOT NULL AND user_id != '' AND user_id != 'default'
        AND user_id IN (SELECT id FROM users);" | sed '$ s/,$//' >> $MIGRATION_FILE
        
        echo "ON CONFLICT (user_id) DO UPDATE SET coin_pool_url = EXCLUDED.coin_pool_url, oi_top_url = EXCLUDED.oi_top_url, updated_at = EXCLUDED.updated_at;" >> $MIGRATION_FILE
        echo "" >> $MIGRATION_FILE
    fi
fi

# 添加迁移验证查询
cat >> $MIGRATION_FILE << 'EOL'
-- 迁移验证查询
SELECT '=== 数据迁移完成验证 ===' as status;
SELECT 'users' as table_name, COUNT(*) as record_count FROM users
UNION ALL SELECT 'ai_models', COUNT(*) FROM ai_models
UNION ALL SELECT 'exchanges', COUNT(*) FROM exchanges  
UNION ALL SELECT 'traders', COUNT(*) FROM traders
UNION ALL SELECT 'system_config', COUNT(*) FROM system_config
UNION ALL SELECT 'beta_codes', COUNT(*) FROM beta_codes
UNION ALL SELECT 'user_signal_sources', COUNT(*) FROM user_signal_sources
ORDER BY table_name;

-- 显示关键配置
SELECT '=== 关键系统配置 ===' as info;
SELECT key, 
       CASE WHEN LENGTH(value) > 50 THEN LEFT(value, 50) || '...' ELSE value END as value
FROM system_config 
WHERE key IN ('admin_mode', 'beta_mode', 'api_server_port', 'default_coins', 'jwt_secret')
ORDER BY key;
EOL

echo -e "${GREEN}✅ 迁移脚本生成完成: $MIGRATION_FILE${NC}"

# 确认是否执行迁移
echo
echo -e "${YELLOW}⚠️  准备执行数据迁移，这将：${NC}"
echo "   1. 停止现有服务"
echo "   2. 启动PostgreSQL和Redis"  
echo "   3. 执行数据迁移"
echo "   4. 验证迁移结果"
echo
read -p "确认执行迁移? (y/N): " confirm

if [[ $confirm != [yY] ]]; then
    echo -e "${BLUE}ℹ️  迁移脚本已生成，可稍后手动执行${NC}"
    echo "手动执行命令: $DOCKER_COMPOSE_CMD exec postgres psql -U nofx -d nofx -f /tmp/$MIGRATION_FILE"
    exit 0
fi

# 执行迁移
echo -e "\n${BLUE}🚀 开始执行数据迁移...${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# 停止现有服务
echo -e "${YELLOW}🛑 停止现有服务...${NC}"
$DOCKER_COMPOSE_CMD down 2>/dev/null || true

# 启动PostgreSQL和Redis
echo -e "${YELLOW}🚀 启动PostgreSQL和Redis服务...${NC}"
$DOCKER_COMPOSE_CMD up postgres redis -d

# 等待服务启动
echo -e "${YELLOW}⏳ 等待服务启动...${NC}"
sleep 15

# 检查PostgreSQL连接
echo -e "${BLUE}🔌 测试数据库连接...${NC}"
max_retries=15
retry_count=0

while [ $retry_count -lt $max_retries ]; do
    if $DOCKER_COMPOSE_CMD exec postgres pg_isready -U nofx > /dev/null 2>&1; then
        echo -e "${GREEN}✅ PostgreSQL连接正常${NC}"
        break
    else
        retry_count=$((retry_count + 1))
        echo -e "${YELLOW}⏳ 等待PostgreSQL启动... (${retry_count}/${max_retries})${NC}"
        sleep 3
    fi
done

if [ $retry_count -eq $max_retries ]; then
    echo -e "${RED}❌ 无法连接到PostgreSQL，请检查服务状态${NC}"
    $DOCKER_COMPOSE_CMD logs postgres
    exit 1
fi

# 复制迁移脚本到容器
echo -e "${BLUE}📦 复制迁移脚本到容器...${NC}"
POSTGRES_CONTAINER=$($DOCKER_COMPOSE_CMD ps -q postgres)
if [ -z "$POSTGRES_CONTAINER" ]; then
    echo -e "${RED}❌ 找不到PostgreSQL容器${NC}"
    exit 1
fi

docker cp $MIGRATION_FILE ${POSTGRES_CONTAINER}:/tmp/$MIGRATION_FILE

# 验证文件复制成功
if ! $DOCKER_COMPOSE_CMD exec postgres test -f /tmp/$MIGRATION_FILE; then
    echo -e "${RED}❌ 迁移脚本复制失败${NC}"
    exit 1
fi

# 执行数据迁移
echo -e "${BLUE}🔄 执行数据迁移...${NC}"
if $DOCKER_COMPOSE_CMD exec postgres psql -U nofx -d nofx --pset pager=off -f /tmp/$MIGRATION_FILE; then
    echo -e "${GREEN}✅ 数据迁移成功！${NC}"
else
    echo -e "${RED}❌ 数据迁移失败${NC}"
    echo "查看错误日志: $DOCKER_COMPOSE_CMD exec postgres psql -U nofx -d nofx -c \"SELECT version();\""
    exit 1
fi

echo
echo -e "${GREEN}🎉 生产环境数据迁移完成！${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${BLUE}📋 后续步骤：${NC}"
echo -e "1. 备份原SQLite: ${YELLOW}mv config.db config.db.backup.$(date +%Y%m%d)${NC}"
echo -e "2. 启动完整应用: ${YELLOW}$DOCKER_COMPOSE_CMD up${NC}"
echo -e "3. 验证功能: 访问 ${YELLOW}http://localhost:3000${NC}"
echo -e "4. 删除迁移文件: ${YELLOW}rm $MIGRATION_FILE${NC}"
echo
echo -e "${BLUE}🔧 如需回滚:${NC}"
echo -e "1. 停止服务: ${YELLOW}$DOCKER_COMPOSE_CMD down${NC}"
echo -e "2. 恢复SQLite: ${YELLOW}mv config.db.backup.$(date +%Y%m%d) config.db${NC}"
echo -e "3. 删除环境变量或编辑 .env 文件注释掉 POSTGRES_HOST"
echo -e "4. 重启: ${YELLOW}$DOCKER_COMPOSE_CMD up${NC}"
echo
echo -e "${GREEN}🚀 PostgreSQL生产环境迁移成功！${NC}"
